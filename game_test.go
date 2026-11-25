package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func createTestGame(t *testing.T, f *fixture, members ...int64) campaign {
	t.Helper()
	var c campaign
	f.request(t, f.admin, "POST", "/api/v1/campaigns", map[string]any{"title": "雾港来信", "ruleset": "5e-2024", "members": members, "scene": "旅人们在雾港的旧钟楼前相遇。", "house_rules": "先声明意图，再由主持人要求检定。"}, 200, &c)
	return c
}
func gameURL(c campaign, suffix string) string {
	return fmt.Sprintf("/api/v1/campaigns/%d/%s", c.ID, suffix)
}
func stateGame(t *testing.T, f *fixture, c campaign) campaign {
	t.Helper()
	var out campaign
	f.request(t, f.admin, "GET", gameURL(c, "state"), nil, 200, &out)
	return out
}
func setTestGame(t *testing.T, f *fixture, c campaign, mode, phase string) campaign {
	t.Helper()
	c = stateGame(t, f, c)
	var out campaign
	f.request(t, f.admin, "PATCH", gameURL(c, "state"), map[string]any{"client_id": randomToken(18), "version": c.Version, "mode": mode, "phase": phase}, 200, &out)
	return out
}
func testCharacter(uid int64) character {
	secret := "PERSONAL_SECRET"
	return character{UserID: uid, Name: "伊岚", Role: "游侠", Level: 1, HP: 12, MaxHP: 12, AC: 14, Abilities: map[string]int{"str": 12, "dex": 16, "con": 13, "int": 10, "wis": 14, "cha": 10}, Inventory: "长弓、火把 × 3", Resources: "箭 × 20", Background: "一名来自北地的游侠。", PrivateNotes: &secret}
}

type bearerTransport struct{ token string }

func (b bearerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(r)
}
func agentForGame(t *testing.T, f *fixture, c campaign, secrets bool) (*http.Client, int64) {
	t.Helper()
	var token struct {
		ID    int64  `json:"id"`
		Token string `json:"token"`
	}
	f.request(t, f.admin, "POST", gameURL(c, "agent-tokens"), map[string]any{"label": "测试主持", "secrets": secrets}, 200, &token)
	return &http.Client{Transport: bearerTransport{token.Token}, Timeout: 5 * time.Second}, token.ID
}
func TestCampaignSecretsScopesAndDurableDice(t *testing.T) {
	f := testApp(t)
	bob := f.member(t, "bob")
	outsider := f.member(t, "carol")
	c := createTestGame(t, f, 2)
	f.request(t, outsider, "GET", gameURL(c, "state"), nil, 403, nil)
	f.request(t, outsider, "GET", gameURL(c, "characters"), nil, 403, nil)
	f.request(t, outsider, "POST", gameURL(c, "actions"), map[string]any{"client_id": randomToken(18), "content": "闯入"}, 403, nil)
	var saved character
	f.request(t, bob, "POST", gameURL(c, "characters"), map[string]any{"client_id": randomToken(18), "version": c.Version, "character": testCharacter(2)}, 200, &saved)
	if saved.PrivateNotes == nil || *saved.PrivateNotes != "PERSONAL_SECRET" {
		t.Fatal("character owner cannot read own notes")
	}
	f.request(t, f.admin, "PUT", gameURL(c, "dm-notes"), map[string]any{"client_id": randomToken(18), "content": "DM_SECRET: 钟楼守卫是幕后策划者。", "note_version": 1}, 200, nil)
	f.request(t, bob, "GET", gameURL(c, "dm-notes"), nil, 403, nil)
	agent, tokenID := agentForGame(t, f, c, false)
	dm, _ := agentForGame(t, f, c, true)
	f.request(t, agent, "GET", gameURL(c, "dm-notes"), nil, 403, nil)
	var secret map[string]any
	f.request(t, dm, "GET", gameURL(c, "dm-notes"), nil, 200, &secret)
	if !strings.Contains(secret["content"].(string), "DM_SECRET") {
		t.Fatal("authorized DM notes unavailable")
	}
	f.request(t, bob, "POST", gameURL(c, "messages"), map[string]any{"client_id": randomToken(18), "content": "WHISPER_SECRET", "visibility": "private", "recipient_id": 1}, 200, nil)
	for _, client := range []*http.Client{f.admin, bob, agent, dm} {
		var context json.RawMessage
		f.request(t, client, "GET", gameURL(c, "context"), nil, 200, &context)
		for _, marker := range []string{"DM_SECRET", "PERSONAL_SECRET", "WHISPER_SECRET"} {
			if bytes.Contains(context, []byte(marker)) {
				t.Fatalf("public context leaked %s", marker)
			}
		}
	}
	var publicChars []character
	f.request(t, agent, "GET", gameURL(c, "characters"), nil, 200, &publicChars)
	if publicChars[0].PrivateNotes != nil {
		t.Fatal("public agent received private character notes")
	}
	c = setTestGame(t, f, c, "manual", "awaiting_action")
	rollInput := map[string]any{"client_id": randomToken(18), "formula": "2d20kh1+5", "reason": "调查刻痕", "visibility": "public"}
	var first, second struct {
		EventID int64      `json:"event_id"`
		Roll    diceResult `json:"roll"`
	}
	f.request(t, bob, "POST", gameURL(c, "rolls"), rollInput, 200, &first)
	f.request(t, bob, "POST", gameURL(c, "rolls"), rollInput, 200, &second)
	if first.EventID != second.EventID || gameJSON(first.Roll) != gameJSON(second.Roll) {
		t.Fatal("retry rerolled dice")
	}
	want := max(first.Roll.Dice[0], first.Roll.Dice[1]) + 5
	if first.Roll.Total != want {
		t.Fatal("advantage arithmetic incorrect")
	}
	rollInput["formula"] = "1d20"
	f.request(t, bob, "POST", gameURL(c, "rolls"), rollInput, 409, nil)
	f.request(t, bob, "POST", gameURL(c, "rolls"), map[string]any{"client_id": randomToken(18), "formula": "999d99999", "reason": "bad"}, 400, nil)
	f.request(t, bob, "POST", gameURL(c, "rolls"), map[string]any{"client_id": randomToken(18), "formula": "1d20", "reason": "private", "visibility": "dm"}, 403, nil)
	f.request(t, f.admin, "POST", gameURL(c, "rolls"), map[string]any{"client_id": randomToken(18), "formula": "1d20", "reason": "HIDDEN_ROLL", "visibility": "dm"}, 200, nil)
	var log []campaignEvent
	f.request(t, bob, "GET", gameURL(c, "events"), nil, 200, &log)
	if strings.Contains(gameJSON(log), "HIDDEN_ROLL") {
		t.Fatal("secret roll leaked")
	}
	another := createTestGame(t, f)
	f.request(t, agent, "GET", gameURL(another, "state"), nil, 401, nil)
	f.request(t, agent, "GET", "/api/v1/entertainment/ai", nil, 401, nil)
	f.request(t, f.admin, "DELETE", gameURL(c, fmt.Sprintf("agent-tokens/%d", tokenID)), nil, 200, nil)
	f.request(t, agent, "GET", gameURL(c, "state"), nil, 401, nil)
	reopened, err := openStore(filepath.Join(f.app.cfg.DataDir, "linchat.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	var count int
	if err = reopened.QueryRow("SELECT COUNT(*) FROM campaign_events WHERE id=? AND kind='roll'", first.EventID).Scan(&count); err != nil || count != 1 {
		t.Fatal("roll not persisted")
	}
	var schema int
	reopened.QueryRow("PRAGMA user_version").Scan(&schema)
	if schema != 5 {
		t.Fatal("campaign migration missing")
	}
}
func TestCampaignExternalQueueAtomicCompletionAndProposals(t *testing.T) {
	f := testApp(t)
	bob := f.member(t, "bob")
	c := createTestGame(t, f, 2)
	var ch character
	f.request(t, bob, "POST", gameURL(c, "characters"), map[string]any{"client_id": randomToken(18), "version": c.Version, "character": testCharacter(2)}, 200, &ch)
	c = setTestGame(t, f, c, "external", "awaiting_action")
	agent, _ := agentForGame(t, f, c, false)
	other, _ := agentForGame(t, f, c, false)
	var action struct {
		EventID int64 `json:"event_id"`
		TurnID  int64 `json:"turn_id"`
	}
	input := map[string]any{"client_id": randomToken(18), "content": "我调查钟楼的入口。"}
	f.request(t, bob, "POST", gameURL(c, "actions"), input, 200, &action)
	f.request(t, bob, "POST", gameURL(c, "actions"), input, 200, nil)
	f.request(t, f.admin, "POST", gameURL(c, "actions"), map[string]any{"client_id": randomToken(18), "content": "我观察街道。"}, 200, nil)
	var claim struct {
		Turn    gameTurn        `json:"turn"`
		Context json.RawMessage `json:"context"`
	}
	f.request(t, agent, "POST", gameURL(c, "turns/claim"), map[string]any{}, 200, &claim)
	if claim.Turn.ID != action.TurnID {
		t.Fatal("actions not processed FIFO")
	}
	if bytes.Contains(claim.Context, []byte("我观察街道")) {
		t.Fatal("active turn received a later queued player's action")
	}
	f.request(t, other, "POST", gameURL(c, "turns/claim"), map[string]any{}, 409, nil)
	f.request(t, f.admin, "PATCH", gameURL(c, "state"), map[string]any{"client_id": randomToken(18), "version": c.Version, "scene": "并发变更"}, 409, nil)
	roll := map[string]any{"client_id": randomToken(18), "formula": "1d20+2", "reason": "调查", "turn_id": claim.Turn.ID, "lease": claim.Turn.Lease}
	f.request(t, agent, "POST", gameURL(c, "rolls"), roll, 200, nil)
	hp := 9
	complete := gameCompleteInput{gameOp: gameOp{ClientID: randomToken(18), Version: claim.Turn.BaseVersion}, Lease: claim.Turn.Lease, Outcome: gameOutcome{Narration: "你发现了一条被泥沙掩埋的通道。", Phase: "awaiting_action", Changes: []gameCharacterChange{{ID: ch.ID, HP: &hp}}}}
	url := gameURL(c, fmt.Sprintf("turns/%d/complete", claim.Turn.ID))
	// Two simultaneous network retries commit a single narration and one resource change.
	var wg sync.WaitGroup
	results := make(chan string, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req, _ := http.NewRequest("POST", f.server.URL+url, strings.NewReader(gameJSON(complete)))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-LinChat", "1")
			resp, err := agent.Do(req)
			if err != nil {
				results <- err.Error()
				return
			}
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			results <- fmt.Sprintf("%d:%s", resp.StatusCode, body)
		}()
	}
	wg.Wait()
	close(results)
	var previous string
	for result := range results {
		if !strings.HasPrefix(result, "200:") {
			t.Fatal(result)
		}
		if previous != "" && previous != result {
			t.Fatal("retry result differs")
		}
		previous = result
	}
	var count int
	f.app.store.QueryRow("SELECT COUNT(*) FROM campaign_events WHERE campaign_id=? AND kind='narration'", c.ID).Scan(&count)
	if count != 1 {
		t.Fatal("duplicate narration")
	}
	var chars []character
	f.request(t, bob, "GET", gameURL(c, "characters"), nil, 200, &chars)
	if chars[0].HP != 9 || chars[0].Version != 2 {
		t.Fatal("resource update applied more than once")
	}
	f.request(t, agent, "POST", gameURL(c, "turns/claim"), map[string]any{}, 200, &claim)
	c = stateGame(t, f, c)
	f.request(t, f.admin, "PATCH", gameURL(c, "state"), map[string]any{"client_id": randomToken(18), "version": c.Version, "phase": "paused"}, 200, nil)
	complete.ClientID = randomToken(18)
	complete.Lease = claim.Turn.Lease
	complete.Version = claim.Turn.BaseVersion
	f.request(t, agent, "POST", gameURL(c, fmt.Sprintf("turns/%d/complete", claim.Turn.ID)), complete, 409, nil)
	var proposal struct {
		ID int64 `json:"id"`
	}
	f.request(t, agent, "POST", gameURL(c, "story-proposals"), map[string]any{"client_id": randomToken(18), "kind": "scene", "content": "NEW_CANON", "character_id": 0}, 200, &proposal)
	c = stateGame(t, f, c)
	if c.Scene == "NEW_CANON" {
		t.Fatal("draft changed canon")
	}
	decision := map[string]any{"client_id": randomToken(18), "version": c.Version, "accept": true}
	f.request(t, agent, "POST", gameURL(c, fmt.Sprintf("story-proposals/%d/decide", proposal.ID)), decision, 403, nil)
	f.request(t, bob, "POST", gameURL(c, fmt.Sprintf("story-proposals/%d/decide", proposal.ID)), decision, 403, nil)
	f.request(t, f.admin, "POST", gameURL(c, fmt.Sprintf("story-proposals/%d/decide", proposal.ID)), decision, 200, nil)
	if stateGame(t, f, c).Scene != "NEW_CANON" {
		t.Fatal("approved story did not persist")
	}
}
func waitGame(t *testing.T, f *fixture, c campaign, status string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var actual string
		err := f.app.store.QueryRow("SELECT status FROM campaign_turns WHERE campaign_id=? ORDER BY id DESC LIMIT 1", c.ID).Scan(&actual)
		if err == nil && actual == status {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	var statusNow, errorNow string
	f.app.store.QueryRow("SELECT status,error FROM campaign_turns WHERE campaign_id=? ORDER BY id DESC LIMIT 1", c.ID).Scan(&statusNow, &errorNow)
	t.Fatalf("turn: %s: %s", statusNow, errorNow)
}
func TestCampaignBuiltInToolsEncryptionAndGroupMention(t *testing.T) {
	f := testApp(t)
	bob := f.member(t, "bob")
	c := createTestGame(t, f, 2)
	var leaked atomic.Bool
	var calls atomic.Int32
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		raw, _ := io.ReadAll(r.Body)
		if bytes.Contains(raw, []byte("DO_NOT_SEND_TO_MODEL")) {
			leaked.Store(true)
		}
		if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer fake-provider-secret" {
			w.WriteHeader(401)
			return
		}
		var in struct {
			Messages []gameModelMessage `json:"messages"`
		}
		if json.Unmarshal(raw, &in) != nil {
			w.WriteHeader(400)
			return
		}
		name := "roll_dice"
		args := gameJSON(map[string]any{"formula": "1d20+3", "reason": "调查钟楼"})
		if len(in.Messages) > 0 && in.Messages[len(in.Messages)-1].Role == "tool" {
			name = "finish_turn"
			args = gameJSON(gameOutcome{Narration: "服务器的检定已记录。你找到通往钟楼的楼梯。", Phase: "awaiting_action", Changes: []gameCharacterChange{}})
		}
		if bytes.Contains(raw, []byte(`\"kind\":\"summary\"`)) {
			name = "finish_turn"
			args = gameJSON(gameOutcome{Narration: "本次冒险结束。", Summary: "旅人们调查了钟楼，发现楼梯。下次从楼梯入口继续。", Phase: "awaiting_action", Changes: []gameCharacterChange{}})
		}
		call := gameToolCall{ID: "test_tool", Type: "function"}
		call.Function.Name = name
		call.Function.Arguments = args
		writeJSON(w, 200, map[string]any{"choices": []any{map[string]any{"message": gameModelMessage{Role: "assistant", Content: nil, ToolCalls: []gameToolCall{call}}}}})
	}))
	defer model.Close()
	f.request(t, bob, "PUT", "/api/v1/entertainment/ai", map[string]any{}, 403, nil)
	f.request(t, f.admin, "PUT", "/api/v1/entertainment/ai", map[string]any{"base_url": model.URL + "/v1", "model": "fake-tools-model", "api_key": "fake-provider-secret", "enabled": true}, 200, nil)
	var encrypted string
	f.app.store.QueryRow("SELECT key_cipher FROM game_ai_settings").Scan(&encrypted)
	if encrypted == "" || strings.Contains(encrypted, "fake-provider-secret") {
		t.Fatal("API key not encrypted")
	}
	var publicSettings json.RawMessage
	f.request(t, bob, "GET", "/api/v1/entertainment/ai", nil, 200, &publicSettings)
	if bytes.Contains(publicSettings, []byte("fake-provider-secret")) || bytes.Contains(publicSettings, []byte(model.URL)) {
		t.Fatal("provider config leaked")
	}
	f.request(t, f.admin, "PUT", gameURL(c, "dm-notes"), map[string]any{"client_id": randomToken(18), "content": "DO_NOT_SEND_TO_MODEL", "note_version": 1}, 200, nil)
	c = setTestGame(t, f, c, "builtin", "awaiting_action")
	mentionInput := map[string]any{"client_id": randomToken(18), "content": "@主持人 我想调查钟楼。", "mentions": []mention{{UserID: c.BotID, Start: 0, End: 4}}}
	f.request(t, bob, "POST", fmt.Sprintf("/api/conversations/%d/messages", c.ConversationID), mentionInput, 201, nil)
	waitGame(t, f, c, "done")
	f.request(t, bob, "POST", fmt.Sprintf("/api/conversations/%d/messages", c.ConversationID), mentionInput, 200, nil)
	var count int
	f.app.store.QueryRow("SELECT COUNT(*) FROM campaign_turns WHERE campaign_id=?", c.ID).Scan(&count)
	if count != 1 {
		t.Fatal("@ retry queued another action")
	}
	if leaked.Load() {
		t.Fatal("DM notes were included in provider input")
	}
	if calls.Load() != 2 {
		t.Fatalf("expected roll + finish roundtrip, got %d", calls.Load())
	}
	var msgs struct {
		Messages []message `json:"messages"`
	}
	f.request(t, bob, "GET", fmt.Sprintf("/api/conversations/%d/messages", c.ConversationID), nil, 200, &msgs)
	if len(msgs.Messages) != 3 {
		t.Fatalf("expected original, dice and narrator in group, got %d", len(msgs.Messages))
	}
	f.request(t, f.admin, "POST", gameURL(c, "host"), map[string]any{"client_id": randomToken(18), "kind": "summary"}, 200, nil)
	waitGame(t, f, c, "done")
	c = stateGame(t, f, c)
	if c.Phase != "paused" || c.Summary == "" {
		t.Fatal("session summary not saved or paused")
	}
}

func TestCampaignProviderRecoveryReusesRollAndRejectsStaleResult(t *testing.T) {
	f := testApp(t)
	c := createTestGame(t, f)
	var failAfterRoll atomic.Bool
	failAfterRoll.Store(true)
	var rollRequests atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Messages []gameModelMessage `json:"messages"`
		}
		json.NewDecoder(r.Body).Decode(&in)
		call := gameToolCall{ID: "recovery_call", Type: "function"}
		if in.Messages[len(in.Messages)-1].Role == "tool" {
			if failAfterRoll.Load() {
				w.WriteHeader(503)
				return
			}
			call.Function.Name = "finish_turn"
			call.Function.Arguments = gameJSON(gameOutcome{Narration: "使用先前记录的检定，你找到入口。", Phase: "awaiting_action", Changes: []gameCharacterChange{}})
		} else {
			rollRequests.Add(1)
			call.Function.Name = "roll_dice"
			call.Function.Arguments = `{"formula":"1d20+4","reason":"找到入口"}`
		}
		writeJSON(w, 200, map[string]any{"choices": []any{map[string]any{"message": gameModelMessage{Role: "assistant", ToolCalls: []gameToolCall{call}}}}})
	}))
	defer provider.Close()
	f.request(t, f.admin, "PUT", "/api/v1/entertainment/ai", map[string]any{"base_url": provider.URL + "/v1", "model": "recovery-test", "api_key": "test-only-key", "enabled": true}, 200, nil)
	c = setTestGame(t, f, c, "builtin", "awaiting_action")
	var action struct {
		TurnID int64 `json:"turn_id"`
	}
	f.request(t, f.admin, "POST", gameURL(c, "actions"), map[string]any{"client_id": randomToken(18), "content": "我寻找入口。"}, 200, &action)
	waitGame(t, f, c, "failed")
	var before string
	f.app.store.QueryRow("SELECT payload FROM campaign_events WHERE campaign_id=? AND kind='roll'", c.ID).Scan(&before)
	if before == "" {
		t.Fatal("roll was not saved before provider failure")
	}
	// Simulate a lost local tool result after the durable dice receipt committed.
	f.app.store.Exec("UPDATE campaign_turn_steps SET tool_result='' WHERE turn_id=?", action.TurnID)
	failAfterRoll.Store(false)
	c = stateGame(t, f, c)
	f.request(t, f.admin, "POST", gameURL(c, fmt.Sprintf("turns/%d/retry", action.TurnID)), map[string]any{"client_id": randomToken(18), "version": c.Version}, 200, nil)
	waitGame(t, f, c, "done")
	var count int
	f.app.store.QueryRow("SELECT COUNT(*) FROM campaign_events WHERE campaign_id=? AND kind='roll'", c.ID).Scan(&count)
	var after string
	f.app.store.QueryRow("SELECT payload FROM campaign_events WHERE campaign_id=? AND kind='roll'", c.ID).Scan(&after)
	if count != 1 || before != after || rollRequests.Load() != 1 {
		t.Fatal("provider recovery rerolled or changed the prior dice")
	}
	f.request(t, f.admin, "PATCH", gameURL(c, "state"), map[string]any{"client_id": randomToken(18), "version": c.Version, "scene": "stale"}, 409, nil)
}
