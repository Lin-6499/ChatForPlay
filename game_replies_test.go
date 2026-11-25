package main

import (
	"fmt"
	"testing"
)

func testReplyPrompt(target int64) *gameReplyPrompt {
	return &gameReplyPrompt{TargetUserID: target, Options: []gameReplyOption{
		{ID: "accept", Label: "接下委托", Content: "我接下委托，准备出发。"},
		{ID: "ask", Label: "询问报酬", Content: "我想先了解这份委托的报酬。"},
	}}
}

func TestReplyChoicesPermissionsIdempotencyAndExpiry(t *testing.T) {
	f := testApp(t)
	bob := f.member(t, "bob")
	outsider := f.member(t, "carol")
	c := setTestGame(t, f, createTestGame(t, f, 2), "manual", "awaiting_action")
	agent, _ := agentForGame(t, f, c, false)
	post := func(prompt *gameReplyPrompt) int64 {
		t.Helper()
		var result struct {
			EventID int64 `json:"event_id"`
		}
		f.request(t, f.admin, "POST", gameURL(c, "messages"), map[string]any{
			"client_id": randomToken(18), "version": stateGame(t, f, c).Version,
			"content": "你准备怎么回应？", "reply_prompt": prompt,
		}, 200, &result)
		return result.EventID
	}
	read := func() *gameReplyView {
		t.Helper()
		var result struct {
			Prompt *gameReplyView `json:"prompt"`
		}
		f.request(t, bob, "GET", gameURL(c, "reply-prompt"), nil, 200, &result)
		return result.Prompt
	}
	event := post(testReplyPrompt(2))
	if p := read(); p == nil || !p.CanReply || p.EventID != event {
		t.Fatal("prompt unavailable", p)
	}
	f.request(t, outsider, "GET", gameURL(c, "reply-prompt"), nil, 403, nil)
	f.request(t, f.admin, "POST", gameURL(c, "actions"), map[string]any{"client_id": randomToken(18), "reply_to_event_id": event, "option_id": "accept"}, 403, nil)
	f.request(t, agent, "POST", gameURL(c, "actions"), map[string]any{"client_id": randomToken(18), "reply_to_event_id": event, "option_id": "accept"}, 403, nil)
	f.request(t, bob, "POST", gameURL(c, "actions"), map[string]any{"client_id": randomToken(18), "reply_to_event_id": event, "option_id": "invented"}, 400, nil)
	f.request(t, bob, "POST", gameURL(c, "actions"), map[string]any{"client_id": randomToken(18), "reply_to_event_id": event, "option_id": "accept", "content": "篡改选项"}, 400, nil)
	f.request(t, bob, "POST", gameURL(c, "actions"), map[string]any{"client_id": randomToken(18), "option_id": "accept"}, 400, nil)
	input := map[string]any{"client_id": randomToken(18), "reply_to_event_id": event, "option_id": "accept"}
	var first, retry map[string]any
	f.request(t, bob, "POST", gameURL(c, "actions"), input, 200, &first)
	f.request(t, bob, "POST", gameURL(c, "actions"), input, 200, &retry)
	if first["event_id"] != retry["event_id"] {
		t.Fatal("retry duplicated action")
	}
	var content string
	f.app.store.QueryRow("SELECT content FROM campaign_events WHERE id=?", first["event_id"]).Scan(&content)
	if content != testReplyPrompt(2).Options[0].Content {
		t.Fatal("option was not resolved server-side")
	}
	if p := read(); p == nil || p.CanReply || !p.Answered {
		t.Fatal("answer state not persisted")
	}
	f.request(t, bob, "POST", gameURL(c, "actions"), map[string]any{"client_id": randomToken(18), "reply_to_event_id": event, "content": "第二次回复"}, 409, nil)

	event = post(testReplyPrompt(0))
	for _, client := range []struct {
		name string
		user bool
	}{{"玩家自由输入", true}, {"房主也自行回复", false}} {
		httpClient := bob
		if !client.user {
			httpClient = f.admin
		}
		f.request(t, httpClient, "POST", gameURL(c, "actions"), map[string]any{"client_id": randomToken(18), "reply_to_event_id": event, "content": client.name}, 200, nil)
	}
	event = post(testReplyPrompt(2))
	post(nil)
	if read() != nil {
		t.Fatal("new narration did not retire old choices")
	}
	f.request(t, bob, "POST", gameURL(c, "actions"), map[string]any{"client_id": randomToken(18), "reply_to_event_id": event, "option_id": "ask"}, 409, nil)
	for _, prompt := range []*gameReplyPrompt{testReplyPrompt(3), {Options: []gameReplyOption{}}, {Options: []gameReplyOption{{ID: "a", Label: "一", Content: "一"}, {ID: "a", Label: "二", Content: "二"}}}} {
		f.request(t, f.admin, "POST", gameURL(c, "messages"), map[string]any{"client_id": randomToken(18), "version": stateGame(t, f, c).Version, "content": "问题", "reply_prompt": prompt}, 400, nil)
	}
	f.request(t, bob, "POST", gameURL(c, "messages"), map[string]any{"client_id": randomToken(18), "version": stateGame(t, f, c).Version, "content": "冒充主持人", "reply_prompt": testReplyPrompt(0)}, 403, nil)
}

func TestExternalHostReplyPromptAtomicCompletion(t *testing.T) {
	f := testApp(t)
	bob := f.member(t, "bob")
	c := setTestGame(t, f, createTestGame(t, f, 2), "external", "awaiting_action")
	agent, _ := agentForGame(t, f, c, false)
	f.request(t, bob, "POST", gameURL(c, "actions"), map[string]any{"client_id": randomToken(18), "content": "我走进大厅。"}, 200, nil)
	var claim struct {
		Turn gameTurn `json:"turn"`
	}
	f.request(t, agent, "POST", gameURL(c, "turns/claim"), map[string]any{}, 200, &claim)
	in := gameCompleteInput{gameOp: gameOp{ClientID: randomToken(18), Version: claim.Turn.BaseVersion}, Lease: claim.Turn.Lease,
		Outcome: gameOutcome{Narration: "要接下委托吗？", Phase: "awaiting_action", ReplyPrompt: testReplyPrompt(2)}}
	url := gameURL(c, fmt.Sprintf("turns/%d/complete", claim.Turn.ID))
	f.request(t, agent, "POST", url, in, 200, nil)
	f.request(t, agent, "POST", url, in, 200, nil)
	var result struct {
		Prompt *gameReplyView `json:"prompt"`
	}
	f.request(t, bob, "GET", gameURL(c, "reply-prompt"), nil, 200, &result)
	if result.Prompt == nil || !result.Prompt.CanReply {
		t.Fatal("completed turn lost its prompt")
	}
	f.request(t, bob, "POST", gameURL(c, "actions"), map[string]any{"client_id": randomToken(18), "reply_to_event_id": result.Prompt.EventID, "content": "我先寻找同伴商量"}, 200, nil)
	f.request(t, agent, "POST", gameURL(c, "turns/claim"), map[string]any{}, 200, &claim)
	var raw string
	f.app.store.QueryRow("SELECT payload FROM campaign_events WHERE id=?", claim.Turn.EventID).Scan(&raw)
	if raw == "{}" {
		t.Fatal("reply linkage missing from next host turn")
	}
}
