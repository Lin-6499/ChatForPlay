package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestChapterFocusProgressAndRecapIsolation(t *testing.T) {
	f := testApp(t)
	bob := f.member(t, "bob")
	outsider := f.member(t, "outsider")
	c := createTestGame(t, f, 2)
	save := func(l gameLore) gameLore {
		t.Helper()
		var out struct {
			Entry gameLore `json:"entry"`
		}
		f.request(t, f.admin, "POST", gameURL(c, "lore"), map[string]any{"client_id": randomToken(18), "version": stateGame(t, f, c).Version, "entry": l}, 200, &out)
		return out.Entry
	}
	chapter := save(gameLore{Kind: "chapter", Title: "本幕", Content: "公开的钟楼", Visibility: "public", Status: "active"})
	quest := save(gameLore{Kind: "quest", Title: "找钥匙", Content: "尚未找到钥匙", Visibility: "public", Status: "active", ChapterID: chapter.ID, Progress: "in_progress"})
	save(gameLore{Kind: "fact", Title: "另一幕", Content: "不相关", Visibility: "public", Status: "active", InContext: true})
	save(gameLore{Kind: "clue", Title: "SECRET-CLUE", Content: "幕后真相", Visibility: "dm", Status: "active", ChapterID: chapter.ID, Progress: "undiscovered"})
	focus := map[string]any{"client_id": randomToken(18), "version": stateGame(t, f, c).Version, "chapter_id": chapter.ID}
	f.request(t, bob, "POST", gameURL(c, "lore/focus"), focus, 403, nil)
	f.request(t, f.admin, "POST", gameURL(c, "lore/focus"), focus, 200, nil)
	f.request(t, f.admin, "POST", gameURL(c, "lore/focus"), focus, 200, nil)
	var ctx struct {
		Lore struct {
			Entries []gameLore `json:"entries"`
		} `json:"lore"`
	}
	f.request(t, bob, "GET", gameURL(c, "context"), nil, 200, &ctx)
	if len(ctx.Lore.Entries) != 2 {
		t.Fatal("focus did not select only chapter and public child")
	}
	var recap json.RawMessage
	f.request(t, bob, "GET", gameURL(c, "recap"), nil, 200, &recap)
	if strings.Contains(string(recap), "SECRET-CLUE") || !strings.Contains(string(recap), "找钥匙") {
		t.Fatal("recap filtering", string(recap))
	}
	f.request(t, outsider, "GET", gameURL(c, "recap"), nil, 403, nil)
	// Focus creates a new entry revision, so reload before editing.
	var list []gameLore
	f.request(t, f.admin, "GET", gameURL(c, "lore"), nil, 200, &list)
	for _, entry := range list {
		if entry.ID == quest.ID {
			quest = entry
		}
	}
	quest.Progress = "completed"
	save(quest)
	f.request(t, bob, "GET", gameURL(c, "recap"), nil, 200, &recap)
	if strings.Contains(string(recap), "找钥匙") {
		t.Fatal("completed quest shown as unresolved")
	}
	// A public child's chapter cannot be hidden behind a secret parent.
	chapter.Visibility = "dm"
	for _, entry := range list {
		if entry.ID == chapter.ID {
			chapter.Version = entry.Version
		}
	}
	f.request(t, f.admin, "POST", gameURL(c, "lore"), map[string]any{"client_id": randomToken(18), "version": stateGame(t, f, c).Version, "entry": chapter}, 409, nil)
}

func TestMemorySuggestionsAreAtomicOwnerConfirmedAndRetrySafe(t *testing.T) {
	f := testApp(t)
	bob := f.member(t, "bob")
	c := setTestGame(t, f, createTestGame(t, f, 2), "external", "awaiting_action")
	agent, _ := agentForGame(t, f, c, false)
	f.request(t, bob, "POST", gameURL(c, "actions"), map[string]any{"client_id": randomToken(18), "content": "我询问守夜人"}, 200, nil)
	var claim struct {
		Turn gameTurn `json:"turn"`
	}
	f.request(t, agent, "POST", gameURL(c, "turns/claim"), map[string]any{}, 200, &claim)
	complete := gameCompleteInput{gameOp: gameOp{ClientID: randomToken(18), Version: claim.Turn.BaseVersion}, Lease: claim.Turn.Lease, Outcome: gameOutcome{
		Narration: "守夜人交出了一把钥匙。", Phase: "awaiting_action", Changes: []gameCharacterChange{},
		MemorySuggestions: []memorySuggestionInput{{Kind: "fact", Title: "得到钥匙", Content: "守夜人已交出钥匙。"}},
	}}
	u := gameURL(c, fmt.Sprintf("turns/%d/complete", claim.Turn.ID))
	f.request(t, agent, "POST", u, complete, 200, nil)
	f.request(t, agent, "POST", u, complete, 200, nil)
	var count int
	f.app.store.QueryRow("SELECT count(*) FROM campaign_lore WHERE campaign_id=?", c.ID).Scan(&count)
	if count != 0 {
		t.Fatal("suggestion became canon without approval")
	}
	var proposals []struct {
		ID      int64  `json:"id"`
		EventID int64  `json:"event_id"`
		Status  string `json:"status"`
	}
	f.request(t, f.admin, "GET", gameURL(c, "memory-suggestions"), nil, 200, &proposals)
	if len(proposals) != 1 || proposals[0].Status != "pending" {
		t.Fatal("duplicate or missing suggestion")
	}
	f.request(t, bob, "GET", gameURL(c, "memory-suggestions"), nil, 403, nil)
	f.request(t, agent, "GET", gameURL(c, "memory-suggestions"), nil, 403, nil)
	entry := gameLore{Kind: "fact", Title: "房主核对的钥匙", Content: "已核对正式叙述：钥匙由守夜人交给队伍。", Visibility: "public", Status: "active", InContext: true}
	accept := map[string]any{"client_id": randomToken(18), "version": stateGame(t, f, c).Version, "suggestion_id": proposals[0].ID, "entry": entry}
	f.request(t, f.admin, "POST", gameURL(c, "lore"), accept, 200, nil)
	f.request(t, f.admin, "POST", gameURL(c, "lore"), accept, 200, nil)
	f.request(t, f.admin, "POST", gameURL(c, "lore"), map[string]any{"client_id": randomToken(18), "version": stateGame(t, f, c).Version, "suggestion_id": proposals[0].ID, "entry": entry}, 409, nil)
	f.app.store.QueryRow("SELECT count(*) FROM campaign_lore WHERE campaign_id=?", c.ID).Scan(&count)
	if count != 1 {
		t.Fatal("approval retry duplicated memory")
	}
}
