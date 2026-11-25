package main

import (
	"fmt"
	"io"
	"strings"
	"testing"
)

func TestStreamLeaseIsolationAndAtomicCompletion(t *testing.T) {
	f := testApp(t)
	bob := f.member(t, "bob")
	outsider := f.member(t, "carol")
	c := setTestGame(t, f, createTestGame(t, f, 2), "external", "awaiting_action")
	agent, _ := agentForGame(t, f, c, false)
	other, _ := agentForGame(t, f, c, false)
	f.request(t, bob, "POST", gameURL(c, "actions"), map[string]any{"client_id": randomToken(18), "content": "我推开门"}, 200, nil)
	var claim struct {
		Turn gameTurn `json:"turn"`
	}
	f.request(t, agent, "POST", gameURL(c, "turns/claim"), map[string]any{}, 200, &claim)
	u := gameURL(c, fmt.Sprintf("turns/%d/stream", claim.Turn.ID))
	in := gameStreamInput{Lease: claim.Turn.Lease, Version: claim.Turn.BaseVersion, Seq: 1, Narration: "门后"}
	f.request(t, other, "POST", u, in, 409, nil)
	f.request(t, bob, "POST", u, in, 409, nil)
	f.request(t, agent, "POST", u, in, 200, nil)
	f.request(t, agent, "POST", u, in, 200, nil)
	var read struct {
		Draft *gameDraft `json:"draft"`
	}
	f.request(t, bob, "GET", gameURL(c, "stream"), nil, 200, &read)
	if read.Draft == nil || read.Draft.Narration != "门后" {
		t.Fatal("missing live draft")
	}
	f.request(t, outsider, "GET", gameURL(c, "stream"), nil, 403, nil)
	in.Narration = "冲突"
	f.request(t, agent, "POST", u, in, 409, nil)
	in.Seq = 2
	in.Narration = "门后亮起灯光"
	f.request(t, agent, "POST", u, in, 200, nil)
	in.Seq = 1
	f.request(t, agent, "POST", u, in, 409, nil)
	var count int
	f.app.store.QueryRow("SELECT count(*) FROM campaign_events WHERE campaign_id=? AND kind='narration'", c.ID).Scan(&count)
	if count != 0 || stateGame(t, f, c).Version != in.Version {
		t.Fatal("draft mutated canonical state")
	}
	complete := gameCompleteInput{gameOp: gameOp{ClientID: randomToken(18), Version: in.Version}, Lease: in.Lease, Outcome: gameOutcome{Narration: "门后亮起灯光。", Phase: "awaiting_action", Changes: []gameCharacterChange{}}}
	f.request(t, agent, "POST", gameURL(c, fmt.Sprintf("turns/%d/complete", claim.Turn.ID)), complete, 200, nil)
	f.request(t, agent, "POST", gameURL(c, fmt.Sprintf("turns/%d/complete", claim.Turn.ID)), complete, 200, nil)
	f.request(t, bob, "GET", gameURL(c, "stream"), nil, 200, &read)
	if read.Draft != nil {
		t.Fatal("completed draft still visible")
	}
	in.Seq = 3
	f.request(t, agent, "POST", u, in, 409, nil)
	f.app.store.QueryRow("SELECT count(*) FROM campaign_events WHERE campaign_id=? AND kind='narration'", c.ID).Scan(&count)
	if count != 1 {
		t.Fatal("duplicate narration")
	}
}

func TestPublicFieldPartialJSON(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{`{"narration":"门后`, "门后"},
		{`{"scene":{"narration":"秘密"},"narration":"公开`, "公开"},
		{`{"narration":"一\n二\uD83D`, "一\n二"},
		{`{"narration":"一\uD83D\uDE00`, "一😀"},
		{`{"changes":[{"narration":"不可显示`, ""},
	} {
		if got := partialPublicText(tc.in, "narration"); got != tc.want {
			t.Fatalf("%s: %q != %q", tc.in, got, tc.want)
		}
	}
}

func TestStreamInvalidation(t *testing.T) {
	for _, cause := range []string{"expired", "failed", "version", "revoked"} {
		t.Run(cause, func(t *testing.T) {
			f := testApp(t)
			bob := f.member(t, "bob")
			c := setTestGame(t, f, createTestGame(t, f, 2), "external", "awaiting_action")
			agent, tokenID := agentForGame(t, f, c, false)
			f.request(t, bob, "POST", gameURL(c, "actions"), map[string]any{"client_id": randomToken(18), "content": "检查"}, 200, nil)
			var claim struct {
				Turn gameTurn `json:"turn"`
			}
			f.request(t, agent, "POST", gameURL(c, "turns/claim"), map[string]any{}, 200, &claim)
			u := gameURL(c, fmt.Sprintf("turns/%d/stream", claim.Turn.ID))
			in := gameStreamInput{Lease: claim.Turn.Lease, Version: claim.Turn.BaseVersion, Seq: 1, Narration: "临时正文"}
			f.request(t, agent, "POST", u, in, 200, nil)
			code := 409
			switch cause {
			case "expired":
				f.app.store.Exec("UPDATE campaign_turns SET started_at=0 WHERE id=?", claim.Turn.ID)
			case "failed":
				f.request(t, agent, "POST", gameURL(c, fmt.Sprintf("turns/%d/fail", claim.Turn.ID)), map[string]any{"client_id": randomToken(18), "lease": in.Lease}, 200, nil)
			case "version":
				f.app.store.Exec("UPDATE campaigns SET version=version+1 WHERE id=?", c.ID)
			case "revoked":
				f.app.store.Exec("UPDATE campaign_tokens SET revoked=1 WHERE id=?", tokenID)
				code = 401
			}
			in.Seq = 2
			f.request(t, agent, "POST", u, in, code, nil)
			{
				var out struct {
					Draft *gameDraft `json:"draft"`
				}
				f.request(t, bob, "GET", gameURL(c, "stream"), nil, 200, &out)
				if out.Draft != nil {
					t.Fatal("invalid draft visible")
				}
			}
		})
	}
}

func TestModelSSEBeforeCompletionAndInterruption(t *testing.T) {
	r, w := io.Pipe()
	first := make(chan string, 1)
	finish := make(chan error, 1)
	go func() {
		_, err := readGameModelStream(r, "narration", func(s string) error { first <- s; return nil })
		finish <- err
	}()
	event := `data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call","function":{"name":"finish_turn","arguments":"{\"narration\":\"门后"}}]}}]}` + "\n\n"
	go func() { _, _ = w.Write([]byte(event)) }()
	if got := <-first; got != "门后" {
		t.Fatal(got)
	}
	w.Close()
	if err := <-finish; err == nil {
		t.Fatal("truncated stream accepted")
	}
	// Content and arguments of other tools are never exposed.
	other := strings.ReplaceAll(event, "finish_turn", "roll_dice") + "data: [DONE]\n\n"
	_, err := readGameModelStream(strings.NewReader(other), "narration", func(s string) error { t.Fatal("leaked tool args"); return nil })
	if err != nil {
		t.Fatal(err)
	}
}
