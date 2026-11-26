package main

import (
	"fmt"
	"testing"
)

func TestClaimedInputRemainsFrozenAcrossRetryAndVersionChange(t *testing.T) {
	f := testApp(t)
	bob := f.member(t, "bob")
	c := setTestGame(t, f, createTestGame(t, f, 2), "external", "awaiting_action")
	agent, _ := agentForGame(t, f, c, false)
	var action struct {
		EventID int64 `json:"event_id"`
	}
	f.request(t, bob, "POST", gameURL(c, "actions"), map[string]any{"client_id": randomToken(18), "content": "旧输入"}, 200, &action)
	var claim struct {
		Turn gameTurn `json:"turn"`
	}
	f.request(t, agent, "POST", gameURL(c, "turns/claim"), map[string]any{}, 200, &claim)
	fail := func() {
		f.request(t, agent, "POST", gameURL(c, fmt.Sprintf("turns/%d/fail", claim.Turn.ID)), map[string]any{"client_id": randomToken(18), "lease": claim.Turn.Lease}, 200, nil)
	}
	fail()
	retryURL := gameURL(c, fmt.Sprintf("turns/%d/retry", claim.Turn.ID))
	f.request(t, f.admin, "POST", retryURL, map[string]any{"client_id": randomToken(18), "version": stateGame(t, f, c).Version}, 200, nil)
	f.request(t, bob, "POST", gameURL(c, fmt.Sprintf("actions/%d/supplement", action.EventID)), map[string]any{"client_id": randomToken(18), "content": "重试后插入的新条件"}, 409, nil)
	var flow struct {
		Queue []struct {
			Actions []flowAction `json:"actions"`
		} `json:"queue"`
	}
	f.request(t, bob, "GET", gameURL(c, "flow"), nil, 200, &flow)
	if flow.Queue[0].Actions[0].CanSupplement {
		t.Fatal("retry advertised mutable original input")
	}
	f.request(t, agent, "POST", gameURL(c, "turns/claim"), map[string]any{}, 200, &claim)
	fail()
	f.request(t, f.admin, "PATCH", gameURL(c, "state"), map[string]any{"client_id": randomToken(18), "version": stateGame(t, f, c).Version, "scene": "房主已明确改动后的新场景"}, 200, nil)
	f.request(t, f.admin, "POST", retryURL, map[string]any{"client_id": randomToken(18), "version": stateGame(t, f, c).Version}, 409, nil)
	f.request(t, f.admin, "POST", retryURL, map[string]any{"client_id": randomToken(18), "version": stateGame(t, f, c).Version, "skip": true}, 200, nil)
}
func TestPendingMemoriesAreNotHiddenByProcessedHistoryAndPaginate(t *testing.T) {
	f := testApp(t)
	c := setTestGame(t, f, createTestGame(t, f), "manual", "awaiting_action")
	var event struct {
		EventID int64 `json:"event_id"`
	}
	f.request(t, f.admin, "POST", gameURL(c, "messages"), map[string]any{"client_id": randomToken(18), "version": c.Version, "content": "正式叙述来源", "visibility": "public"}, 200, &event)
	tx, err := f.app.store.Begin()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 165; i++ {
		status := "pending"
		if i > 0 && i <= 120 {
			status = "accepted"
		}
		if _, err = tx.Exec("INSERT INTO campaign_memory_suggestions(campaign_id,turn_id,event_id,document,status,created_at) VALUES(?,?,?,?,?,?)", c.ID, 1, event.EventID, gameJSON(memorySuggestionInput{Kind: "fact", Title: fmt.Sprintf("建议%d", i), Content: "经正式回合提出的建议"}), status, i); err != nil {
			t.Fatal(err)
		}
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	var old []map[string]any
	f.request(t, f.admin, "GET", gameURL(c, "memory-suggestions"), nil, 200, &old)
	found := false
	for _, v := range old {
		if v["status"] == "pending" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("legacy pending suggestion hidden by recent processed entries")
	}
	seen := map[int64]bool{}
	cursor := int64(0)
	for page := 0; page < 3; page++ {
		var result struct {
			Items []struct {
				ID     int64  `json:"id"`
				Status string `json:"status"`
			} `json:"items"`
			Pending int   `json:"total_pending"`
			Next    int64 `json:"next_before"`
			More    bool  `json:"has_more"`
		}
		suffix := "memory-suggestions?paged=1&status=pending"
		if cursor > 0 {
			suffix += fmt.Sprintf("&before=%d", cursor)
		}
		f.request(t, f.admin, "GET", gameURL(c, suffix), nil, 200, &result)
		if result.Pending != 45 {
			t.Fatal("incorrect pending total", result.Pending)
		}
		for _, item := range result.Items {
			if seen[item.ID] || item.Status != "pending" {
				t.Fatal("duplicate or wrong status")
			}
			seen[item.ID] = true
		}
		cursor = result.Next
		if page < 2 && !result.More || page == 2 && result.More {
			t.Fatal("incorrect pagination")
		}
	}
	if len(seen) != 45 {
		t.Fatal("not all pending suggestions reachable")
	}
	f.request(t, f.admin, "GET", gameURL(c, "memory-suggestions?paged=1&status=invalid"), nil, 400, nil)
	f.request(t, f.admin, "GET", gameURL(c, "memory-suggestions?paged=1&before=-1"), nil, 400, nil)
}
