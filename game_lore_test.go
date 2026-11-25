package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestLorePermissionsHistoryAndContext(t *testing.T) {
	f := testApp(t)
	bob := f.member(t, "bob")
	other := f.member(t, "carol")
	c := createTestGame(t, f, 2)
	public, _ := agentForGame(t, f, c, false)
	secret, _ := agentForGame(t, f, c, true)
	entry := gameLore{Kind: "chapter", Title: "灯塔", Content: "公开的灯塔入口", Visibility: "public", Status: "active", InContext: true}
	body := func(e gameLore) map[string]any {
		return map[string]any{"client_id": randomToken(18), "version": stateGame(t, f, c).Version, "entry": e}
	}
	f.request(t, bob, "POST", gameURL(c, "lore"), body(entry), 403, nil)
	f.request(t, public, "POST", gameURL(c, "lore"), body(entry), 403, nil)
	var saved struct {
		Entry   gameLore `json:"entry"`
		Version int64    `json:"version"`
	}
	input := body(entry)
	f.request(t, f.admin, "POST", gameURL(c, "lore"), input, 200, &saved)
	f.request(t, f.admin, "POST", gameURL(c, "lore"), input, 200, nil)
	entry = saved.Entry
	private := gameLore{Kind: "npc", Title: "幕后人物", Content: "SECRET-NPC-TRUTH", Visibility: "dm", Status: "active", InContext: true}
	f.request(t, f.admin, "POST", gameURL(c, "lore"), body(private), 200, &saved)
	private = saved.Entry
	if private.InContext {
		t.Fatal("secret context flag accepted")
	}
	var list []gameLore
	f.request(t, bob, "GET", gameURL(c, "lore"), nil, 200, &list)
	if len(list) != 1 || list[0].ID != entry.ID {
		t.Fatal("player received secret")
	}
	f.request(t, public, "GET", gameURL(c, "lore"), nil, 200, &list)
	if len(list) != 1 {
		t.Fatal("public agent received secret")
	}
	f.request(t, secret, "GET", gameURL(c, "lore"), nil, 200, &list)
	if len(list) != 2 {
		t.Fatal("secret agent missing data")
	}
	f.request(t, other, "GET", gameURL(c, "lore"), nil, 403, nil)
	historyURL := gameURL(c, fmt.Sprintf("lore/%d/history", private.ID))
	f.request(t, bob, "GET", historyURL, nil, 404, nil)
	var context json.RawMessage
	f.request(t, secret, "GET", gameURL(c, "context"), nil, 200, &context)
	if strings.Contains(string(context), "SECRET-NPC-TRUTH") || !strings.Contains(string(context), "公开的灯塔入口") {
		t.Fatal("public context isolation failed", string(context))
	}
	// Reveal a secret as a new public revision; old secret history stays protected.
	private.Visibility = "public"
	private.Content = "已揭露的人物身份"
	f.request(t, f.admin, "POST", gameURL(c, "lore"), body(private), 200, &saved)
	private = saved.Entry
	f.request(t, bob, "GET", historyURL, nil, 200, &list)
	if len(list) != 1 || list[0].Content != "已揭露的人物身份" {
		t.Fatal("secret revision leaked")
	}
	f.request(t, secret, "GET", historyURL, nil, 200, &list)
	if len(list) != 2 {
		t.Fatal("audit history missing")
	}
	old := entry
	entry.Content = "房主确认的新入口"
	f.request(t, f.admin, "POST", gameURL(c, "lore"), body(entry), 200, &saved)
	entry = saved.Entry
	f.request(t, f.admin, "POST", gameURL(c, "lore"), body(old), 409, nil)
	entry.Status = "archived"
	f.request(t, f.admin, "POST", gameURL(c, "lore"), body(entry), 200, &saved)
	f.request(t, public, "GET", gameURL(c, "context"), nil, 200, &context)
	if strings.Contains(string(context), "房主确认的新入口") {
		t.Fatal("archive still in model context")
	}
	var count int
	f.app.store.QueryRow("SELECT count(*) FROM campaign_lore WHERE campaign_id=?", c.ID).Scan(&count)
	if count != 2 {
		t.Fatal("idempotent save duplicated entries")
	}
}

func TestLoreContextBudgetAndRunningTurn(t *testing.T) {
	f := testApp(t)
	bob := f.member(t, "bob")
	c := createTestGame(t, f, 2)
	var saved struct {
		Entry gameLore `json:"entry"`
	}
	for i := 0; i < 3; i++ {
		entry := gameLore{Kind: "fact", Title: fmt.Sprintf("事实%d", i), Content: strings.Repeat("字", 10000), Visibility: "public", Status: "active", InContext: true}
		f.request(t, f.admin, "POST", gameURL(c, "lore"), map[string]any{"client_id": randomToken(18), "version": stateGame(t, f, c).Version, "entry": entry}, 200, &saved)
	}
	var context struct {
		Lore struct {
			Entries []gameLore `json:"entries"`
			Omitted int        `json:"omitted"`
		} `json:"lore"`
	}
	f.request(t, bob, "GET", gameURL(c, "context"), nil, 200, &context)
	if len(context.Lore.Entries) != 2 || context.Lore.Omitted != 1 {
		t.Fatal("context budget not bounded")
	}
	c = setTestGame(t, f, c, "external", "awaiting_action")
	agent, _ := agentForGame(t, f, c, false)
	f.request(t, bob, "POST", gameURL(c, "actions"), map[string]any{"client_id": randomToken(18), "content": "前进"}, 200, nil)
	f.request(t, agent, "POST", gameURL(c, "turns/claim"), map[string]any{}, 200, nil)
	f.request(t, f.admin, "POST", gameURL(c, "lore"), map[string]any{"client_id": randomToken(18), "version": stateGame(t, f, c).Version, "entry": saved.Entry}, 409, nil)
}
