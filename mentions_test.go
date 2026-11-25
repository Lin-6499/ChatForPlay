package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"testing"
)

func TestMentionsDeliveryReadStateAndPersistence(t *testing.T) {
	f := testApp(t)
	bob := f.member(t, "bob")
	carol := f.member(t, "carol")
	dave := f.member(t, "dave")
	f.request(t, bob, "PATCH", "/api/me", map[string]string{"nickname": "小林"}, 200, nil)
	// Identical nicknames must not change which account is mentioned.
	f.request(t, carol, "PATCH", "/api/me", map[string]string{"nickname": "小林"}, 200, nil)
	var group struct{ ID int64 }
	f.request(t, f.admin, "POST", "/api/conversations",
		map[string]any{"kind": "group", "name": "提及测试", "members": []int64{2, 3}}, 201, &group)
	path := fmt.Sprintf("/api/conversations/%d/messages", group.ID)
	socket := f.socket(t, bob)
	input := map[string]any{
		"client_id": randomToken(18), "content": "  👋 @小林 请查看 @小林  ",
		"mentions": []mention{{UserID: 2, Start: 4, End: 7}, {UserID: 2, Start: 12, End: 15}},
	}
	var sent message
	f.request(t, f.admin, "POST", path, input, 201, &sent)
	want := []mention{{UserID: 2, Start: 2, End: 5}, {UserID: 2, Start: 10, End: 13}}
	if sent.Content != "👋 @小林 请查看 @小林" || !slices.Equal(sent.Mentions, want) {
		t.Fatalf("Unicode offsets or trim normalization incorrect: %#v", sent)
	}
	event := readEvent(t, socket, "message")
	var pushed message
	if err := json.Unmarshal(event["message"], &pushed); err != nil || !slices.Equal(pushed.Mentions, want) {
		t.Fatalf("WebSocket lost mention identities: %s", event["message"])
	}
	assertUnread := func(clientName string, expected int) {
		t.Helper()
		client := bob
		if clientName == "carol" {
			client = carol
		}
		var conversations []conversation
		f.request(t, client, "GET", "/api/conversations", nil, 200, &conversations)
		for _, c := range conversations {
			if c.ID == group.ID {
				if c.UnreadMentions != expected {
					t.Fatalf("%s unread mentions = %d, want %d", clientName, c.UnreadMentions, expected)
				}
				return
			}
		}
		t.Fatal("group missing")
	}
	assertUnread("bob", 1)
	assertUnread("carol", 0)
	f.request(t, dave, "GET", path, nil, 403, nil)
	// A retry after a nickname change returns the immutable original mentions.
	f.request(t, bob, "PATCH", "/api/me", map[string]string{"nickname": "改名后的成员"}, 200, nil)
	var retry message
	f.request(t, f.admin, "POST", path, input, 200, &retry)
	if retry.ID != sent.ID || !slices.Equal(retry.Mentions, want) {
		t.Fatal("retry changed a mention")
	}
	input["mentions"] = []mention{{UserID: 3, Start: 4, End: 7}, {UserID: 3, Start: 12, End: 15}}
	f.request(t, f.admin, "POST", path, input, 409, nil)
	// Persisted mentions must survive a disconnect/server restart.
	f.app.store.Close()
	reopened, err := openStore(filepath.Join(f.app.cfg.DataDir, "linchat.db"))
	if err != nil {
		t.Fatal(err)
	}
	f.app.store = reopened
	var history struct{ Messages []message }
	f.request(t, bob, "GET", path+"?after=0", nil, 200, &history)
	if len(history.Messages) != 1 || !slices.Equal(history.Messages[0].Mentions, want) {
		t.Fatal("mention history did not survive reopening the database")
	}
	assertUnread("bob", 1)
	f.request(t, bob, "POST", fmt.Sprintf("/api/conversations/%d/read", group.ID),
		map[string]int64{"message_id": sent.ID}, 200, nil)
	assertUnread("bob", 0)
}

func TestMentionsValidateMembershipAndSpans(t *testing.T) {
	f := testApp(t)
	bob := f.member(t, "bob")
	f.member(t, "carol")
	var group struct{ ID int64 }
	f.request(t, f.admin, "POST", "/api/conversations",
		map[string]any{"kind": "group", "name": "小组", "members": []int64{2}}, 201, &group)
	path := fmt.Sprintf("/api/conversations/%d/messages", group.ID)
	cases := []struct {
		name, content string
		mentions      []mention
	}{
		{"outside group", "@carol", []mention{{3, 0, 6}}},
		{"nonexistent user", "@bob", []mention{{999, 0, 4}}},
		{"wrong label", "@carol", []mention{{2, 0, 6}}},
		{"negative start", "@bob", []mention{{2, -1, 4}}},
		{"out of bounds", "@bob", []mention{{2, 0, 9}}},
		{"empty span", "@bob", []mention{{2, 0, 0}}},
		{"overlap", "@bob", []mention{{2, 0, 4}, {2, 0, 4}}},
		{"not a mention", "bob", []mention{{2, 0, 3}}},
		{"trimmed span", " @bob ", []mention{{2, 0, 5}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f.request(t, f.admin, "POST", path,
				map[string]any{"client_id": randomToken(18), "content": tc.content, "mentions": tc.mentions}, 400, nil)
		})
	}
	var count int
	f.app.store.QueryRow("SELECT COUNT(*) FROM messages").Scan(&count)
	if count != 0 {
		t.Fatal("rejected mention left a partial message")
	}
	direct := fmt.Sprintf("/api/conversations/%d/messages", f.direct(t, 2))
	f.request(t, f.admin, "POST", direct,
		map[string]any{"client_id": randomToken(18), "content": "@bob", "mentions": []mention{{2, 0, 4}}}, 400, nil)
	var plain message
	f.request(t, f.admin, "POST", path,
		map[string]string{"client_id": randomToken(18), "content": "@bob is ordinary text without selecting a member"}, 201, &plain)
	if len(plain.Mentions) != 0 {
		t.Fatal("plain text unexpectedly notified a member")
	}
	var lobby message
	f.request(t, f.admin, "POST", "/api/conversations/1/messages",
		map[string]any{"client_id": randomToken(18), "content": "@bob @carol",
			"mentions": []mention{{3, 5, 11}, {2, 0, 4}}}, 201, &lobby)
	if len(lobby.Mentions) != 2 || lobby.Mentions[0].UserID != 2 {
		t.Fatal("lobby or multi-person mentions failed")
	}
	f.request(t, bob, "GET", "/api/conversations/1/messages", nil, 200, nil)
}

func TestMentionMigrationPreservesExistingMessages(t *testing.T) {
	f := testApp(t)
	f.member(t, "bob")
	var sent message
	f.request(t, f.admin, "POST", "/api/conversations/1/messages",
		map[string]string{"client_id": randomToken(18), "content": "旧版本保存的消息"}, 201, &sent)
	// Reproduce the v1 schema, then open it with the new additive migration.
	if _, err := f.app.store.Exec("DROP TABLE message_mentions; PRAGMA user_version=1"); err != nil {
		t.Fatal(err)
	}
	f.app.store.Close()
	reopened, err := openStore(filepath.Join(f.app.cfg.DataDir, "linchat.db"))
	if err != nil {
		t.Fatal(err)
	}
	f.app.store = reopened
	old, err := reopened.getMessage(sent.ID)
	if err != nil || old.Content != sent.Content || len(old.Mentions) != 0 {
		t.Fatalf("v1 history lost during migration: %#v, %v", old, err)
	}
	f.request(t, f.admin, "POST", "/api/conversations/1/messages",
		map[string]any{"client_id": randomToken(18), "content": "@bob", "mentions": []mention{{2, 0, 4}}}, 201, nil)
}
