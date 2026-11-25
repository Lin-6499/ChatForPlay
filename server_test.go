package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

type fixture struct {
	app    *app
	server *httptest.Server
	admin  *http.Client
}

func testApp(t *testing.T) *fixture {
	t.Helper()
	c := config{DataDir: t.TempDir(), Name: "TestChat", MaxUpload: 1024, FileQuota: 8192}
	a, err := newApp(c)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.closeGames(); a.hub.close(); a.store.Close() })
	s := httptest.NewServer(a.handler())
	a.cfg.PublicURL = s.URL
	f := &fixture{app: a, server: s, admin: testClient()}
	t.Cleanup(func() { a.hub.close(); s.Close() })
	f.request(t, f.admin, "POST", "/api/setup", credentials{
		Username: "alice", Nickname: "Alice", Password: "alice-password-123", Token: a.setupToken,
	}, 201, nil)
	return f
}

func testClient() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{Jar: jar, Timeout: 5 * time.Second}
}

func (f *fixture) request(t *testing.T, c *http.Client, method, path string, body any, status int, out any) *http.Response {
	t.Helper()
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, f.server.URL+path, reader)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("X-LinChat", "1")
	req.Header.Set("Origin", f.server.URL)
	res, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode != status {
		t.Fatalf("%s %s: got %d, want %d: %s", method, path, res.StatusCode, status, b)
	}
	if out != nil {
		if err = json.Unmarshal(b, out); err != nil {
			t.Fatalf("decode %s: %v", b, err)
		}
	}
	return res
}

func (f *fixture) invite(t *testing.T, uses int) string {
	t.Helper()
	var result struct{ Token string }
	f.request(t, f.admin, "POST", "/api/invites", map[string]int{"uses": uses}, 201, &result)
	return result.Token
}

func (f *fixture) member(t *testing.T, name string) *http.Client {
	t.Helper()
	c := testClient()
	f.request(t, c, "POST", "/api/auth/register", credentials{Username: name, Nickname: name,
		Password: name + "-password-123", Token: f.invite(t, 1)}, 201, nil)
	return c
}

func (f *fixture) direct(t *testing.T, uid int64) int64 {
	t.Helper()
	var result struct{ ID int64 }
	f.request(t, f.admin, "POST", "/api/conversations", map[string]any{"kind": "direct", "name": "", "members": []int64{uid}}, 201, &result)
	return result.ID
}

func (f *fixture) socket(t *testing.T, c *http.Client) *websocket.Conn {
	t.Helper()
	req, _ := http.NewRequest("GET", f.server.URL, nil)
	header := http.Header{"Origin": []string{f.server.URL}}
	for _, cookie := range c.Jar.Cookies(req.URL) {
		req.AddCookie(cookie)
	}
	header.Set("Cookie", req.Header.Get("Cookie"))
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(f.server.URL, "http")+"/api/ws", header)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	readEvent(t, conn, "hello")
	return conn
}

func readEvent(t *testing.T, conn *websocket.Conn, kind string) map[string]json.RawMessage {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	for {
		var event map[string]json.RawMessage
		if err := conn.ReadJSON(&event); err != nil {
			t.Fatal(err)
		}
		if string(event["type"]) == `"`+kind+`"` {
			return event
		}
	}
}

func TestChatMembershipDeliveryAndIdempotency(t *testing.T) {
	f := testApp(t)
	bob := f.member(t, "bob")
	carol := f.member(t, "carol")
	cid := f.direct(t, 2)
	path := fmt.Sprintf("/api/conversations/%d/messages", cid)
	f.request(t, carol, "GET", path, nil, 403, nil)
	f.request(t, carol, "POST", path, map[string]string{"client_id": randomToken(18), "content": "unauthorized"}, 403, nil)
	var carolConversations []conversation
	f.request(t, carol, "GET", "/api/conversations", nil, 200, &carolConversations)
	for _, c := range carolConversations {
		if c.ID == cid {
			t.Fatal("private conversation leaked to non-member")
		}
	}
	bobSocket := f.socket(t, bob)
	carolSocket := f.socket(t, carol)
	input := map[string]string{"client_id": randomToken(18), "content": "跨地区多人聊天"}
	var first, duplicate message
	f.request(t, f.admin, "POST", path, input, 201, &first)
	event := readEvent(t, bobSocket, "message")
	var pushed message
	if err := json.Unmarshal(event["message"], &pushed); err != nil || pushed.ID != first.ID {
		t.Fatalf("unexpected websocket message: %s", event["message"])
	}
	f.request(t, f.admin, "POST", path, input, 200, &duplicate)
	if duplicate.ID != first.ID {
		t.Fatal("retry created a duplicate message")
	}
	input["content"] = "different content"
	f.request(t, f.admin, "POST", path, input, 409, nil)
	var count int
	f.app.store.QueryRow("SELECT COUNT(*) FROM messages WHERE conversation_id=?", cid).Scan(&count)
	if count != 1 {
		t.Fatalf("want exactly one durable message, got %d", count)
	}
	f.request(t, bob, "POST", fmt.Sprintf("/api/conversations/%d/read", cid), map[string]int64{"message_id": first.ID}, 200, nil)
	read := readEvent(t, bobSocket, "read")
	if string(read["last_read"]) != fmt.Sprint(first.ID) {
		t.Fatal("read acknowledgement incorrect")
	}
	// A third authenticated user must not receive private message payloads.
	carolSocket.SetReadDeadline(time.Now().Add(150 * time.Millisecond))
	for {
		var e map[string]any
		if err := carolSocket.ReadJSON(&e); err != nil {
			break
		}
		if e["type"] == "message" || e["type"] == "read" {
			t.Fatal("private event leaked to non-member")
		}
	}
}

func TestInvitationsSessionsAndOriginProtection(t *testing.T) {
	f := testApp(t)
	anon := testClient()
	f.request(t, anon, "GET", "/api/me", nil, 401, nil)
	f.request(t, anon, "POST", "/api/setup", credentials{}, 409, nil)
	token := f.invite(t, 1)
	bob := testClient()
	f.request(t, bob, "POST", "/api/auth/register", credentials{Username: "bob", Nickname: "Bob", Password: "bob-password-123", Token: token}, 201, nil)
	f.request(t, anon, "POST", "/api/auth/register", credentials{Username: "carol", Nickname: "Carol", Password: "carol-password-123", Token: token}, 403, nil)
	f.request(t, bob, "POST", "/api/invites", map[string]int{"uses": 1}, 403, nil)
	var profile user
	f.request(t, bob, "GET", "/api/me", nil, 200, &profile)
	if profile.Admin {
		t.Fatal("invited user became administrator")
	}
	second := testClient()
	response := f.request(t, second, "POST", "/api/auth/login", map[string]string{"username": "bob", "password": "bob-password-123"}, 200, nil)
	cookies := response.Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteLaxMode {
		t.Fatal("session cookie protection missing")
	}
	req, _ := http.NewRequest("POST", f.server.URL+"/api/auth/logout", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://foreign.example")
	req.Header.Set("X-LinChat", "1")
	res, err := bob.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 403 {
		t.Fatal("cross-origin mutation was accepted")
	}
	_, bad, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(f.server.URL, "http")+"/api/ws", http.Header{
		"Origin": []string{"https://foreign.example"}, "Cookie": []string{"linchat_session=" + cookies[0].Value},
	})
	if err == nil || bad.StatusCode != 403 {
		t.Fatal("cross-origin websocket was accepted")
	}
	if bad != nil {
		bad.Body.Close()
	}
	bobSocket := f.socket(t, bob)
	f.request(t, bob, "PATCH", "/api/me", map[string]string{"nickname": "Bob 2", "old_password": "bob-password-123", "new_password": "bob-new-password-123"}, 200, nil)
	f.request(t, second, "GET", "/api/me", nil, 401, nil)
	f.request(t, bob, "GET", "/api/me", nil, 200, nil)
	bobSocket.SetReadDeadline(time.Now().Add(time.Second))
	if _, _, err = bobSocket.ReadMessage(); err == nil {
		t.Fatal("password change did not close old socket")
	}
	f.request(t, bob, "POST", "/api/auth/logout", map[string]any{}, 200, nil)
	f.request(t, bob, "GET", "/api/me", nil, 401, nil)
}

func (f *fixture) file(t *testing.T, c *http.Client, cid int64, clientID string, content []byte, status int) message {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, _ := mw.CreateFormFile("file", "notes.txt")
	part.Write(content)
	mw.Close()
	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/conversations/%d/files?client_id=%s", f.server.URL, cid, clientID), &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("X-LinChat", "1")
	req.Header.Set("Origin", f.server.URL)
	res, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	if res.StatusCode != status {
		t.Fatalf("upload: got %d, expected %d: %s", res.StatusCode, status, data)
	}
	var m message
	json.Unmarshal(data, &m)
	return m
}

func TestFileAuthorizationSizeAndRetry(t *testing.T) {
	f := testApp(t)
	bob := f.member(t, "bob")
	carol := f.member(t, "carol")
	cid := f.direct(t, 2)
	content := []byte("Shared file through cloud or Radmin")
	clientID := randomToken(18)
	m := f.file(t, f.admin, cid, clientID, content, 201)
	duplicate := f.file(t, f.admin, cid, clientID, content, 200)
	if duplicate.ID != m.ID {
		t.Fatal("file upload retry duplicated message")
	}
	url := "/api/files/" + m.FileID
	f.request(t, carol, "GET", url, nil, 403, nil)
	f.request(t, testClient(), "GET", url, nil, 401, nil)
	res, err := bob.Get(f.server.URL + url)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if !bytes.Equal(got, content) || !strings.HasPrefix(res.Header.Get("Content-Disposition"), "attachment") {
		t.Fatal("file download content or disposition incorrect")
	}
	f.file(t, f.admin, cid, randomToken(18), bytes.Repeat([]byte("x"), 1025), 413)
	var count int
	f.app.store.QueryRow("SELECT COUNT(*) FROM files").Scan(&count)
	if count != 1 {
		t.Fatal("rejected upload left a database record")
	}
	entries, err := os.ReadDir(filepath.Join(f.app.cfg.DataDir, "files"))
	if err != nil || len(entries) != 1 {
		t.Fatal("rejected upload left a partial disk file")
	}
}

func TestReconnectPaginationAndPersistence(t *testing.T) {
	f := testApp(t)
	bob := f.member(t, "bob")
	cid := f.direct(t, 2)
	path := fmt.Sprintf("/api/conversations/%d/messages", cid)
	for i := 0; i < 104; i++ {
		f.request(t, f.admin, "POST", path, map[string]string{"client_id": randomToken(18), "content": fmt.Sprintf("Message %03d", i)}, 201, nil)
	}
	type page struct {
		Messages []message
		More     bool
	}
	var first, second page
	f.request(t, bob, "GET", path+"?after=0", nil, 200, &first)
	if len(first.Messages) != 100 || !first.More {
		t.Fatal("first synchronization page incorrect")
	}
	f.request(t, bob, "GET", fmt.Sprintf("%s?after=%d", path, first.Messages[99].ID), nil, 200, &second)
	if len(second.Messages) != 4 || second.More || second.Messages[0].ID <= first.Messages[99].ID {
		t.Fatal("reconnection missed or duplicated history")
	}
	readPath := fmt.Sprintf("/api/conversations/%d/read", cid)
	last := second.Messages[3].ID
	f.request(t, bob, "POST", readPath, map[string]int64{"message_id": last}, 200, nil)
	f.request(t, bob, "POST", readPath, map[string]int64{"message_id": first.Messages[0].ID}, 200, nil)
	var read int64
	f.app.store.QueryRow("SELECT last_read FROM members WHERE conversation_id=? AND user_id=2", cid).Scan(&read)
	if read != last {
		t.Fatal("read cursor moved backwards")
	}
	// Close and reopen the actual SQLite database to verify persisted messages and sessions.
	f.app.store.Close()
	reopened, err := openStore(filepath.Join(f.app.cfg.DataDir, "linchat.db"))
	if err != nil {
		t.Fatal(err)
	}
	f.app.store = reopened
	f.request(t, bob, "GET", path+"?after=0", nil, 200, &first)
	if len(first.Messages) != 100 {
		t.Fatal("messages did not survive database reopen")
	}
	f.request(t, bob, "GET", "/api/me", nil, 200, nil)
}
