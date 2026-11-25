package main

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestCollectionJointTurnIdentitySupplementsAndContext(t *testing.T) {
	f := testApp(t)
	bob := f.member(t, "bob")
	c := createTestGame(t, f, 2)
	var ch character
	f.request(t, bob, "POST", gameURL(c, "characters"), map[string]any{"client_id": randomToken(18), "version": c.Version, "character": testCharacter(2)}, 200, &ch)
	c = setTestGame(t, f, c, "external", "awaiting_action")
	agent, _ := agentForGame(t, f, c, false)
	f.request(t, f.admin, "PUT", gameURL(c, "flow"), map[string]any{"client_id": randomToken(18), "version": c.Version, "mode": "collect", "participants": []int64{1, 2}}, 200, nil)
	var first, second struct {
		EventID int64 `json:"event_id"`
		TurnID  int64 `json:"turn_id"`
	}
	f.request(t, f.admin, "POST", gameURL(c, "actions"), map[string]any{"client_id": randomToken(18), "content": "我观察钟楼"}, 200, &first)
	var empty struct {
		Turn *gameTurn `json:"turn"`
	}
	f.request(t, agent, "POST", gameURL(c, "turns/claim"), map[string]any{}, 200, &empty)
	if empty.Turn != nil {
		t.Fatal("claimed collection before all participants submitted")
	}
	u := gameURL(c, fmt.Sprintf("actions/%d/supplement", first.EventID))
	input := map[string]any{"client_id": randomToken(18), "content": "我同时留意脚印"}
	f.request(t, bob, "POST", u, input, 403, nil)
	f.request(t, f.admin, "POST", u, input, 200, nil)
	f.request(t, f.admin, "POST", u, input, 200, nil)
	f.request(t, f.admin, "POST", gameURL(c, "actions"), map[string]any{"client_id": randomToken(18), "content": "重复行动"}, 409, nil)
	f.request(t, bob, "POST", gameURL(c, "actions"), map[string]any{"client_id": randomToken(18), "content": "我推开门"}, 200, &second)
	if first.TurnID != 0 || second.TurnID == 0 {
		t.Fatal("collection did not create exactly one joint turn")
	}
	var claim struct {
		Turn    gameTurn `json:"turn"`
		Context struct {
			Actions []flowAction    `json:"turn_actions"`
			Events  []campaignEvent `json:"events"`
		} `json:"context"`
	}
	f.request(t, agent, "POST", gameURL(c, "turns/claim"), map[string]any{}, 200, &claim)
	if claim.Turn.Kind != "group" || len(claim.Context.Actions) != 2 || claim.Context.Actions[1].UserID != 2 || len(claim.Context.Actions[0].Supplements) != 1 {
		t.Fatal("group identity or supplement lost", claim)
	}
	f.request(t, f.admin, "POST", u, map[string]any{"client_id": randomToken(18), "content": "领取后补充"}, 409, nil)
	var future struct {
		EventID int64 `json:"event_id"`
	}
	f.request(t, f.admin, "POST", gameURL(c, "actions"), map[string]any{"client_id": randomToken(18), "content": "NEXT-ROUND-NOT-IN-CONTEXT"}, 200, &future)
	var ctx json.RawMessage
	f.request(t, agent, "GET", gameURL(c, "context"), nil, 200, &ctx)
	var generic map[string]any
	json.Unmarshal(ctx, &generic)
	for _, e := range generic["events"].([]any) {
		if e.(map[string]any)["content"] == "NEXT-ROUND-NOT-IN-CONTEXT" {
			t.Fatal("future batch leaked")
		}
	}
	in := checkPrepareInput{gameOp: gameOp{ClientID: randomToken(18), Version: claim.Turn.BaseVersion}, EventID: second.EventID, CharacterID: ch.ID, Kind: "ability", Ability: "str", Reason: "推门", DC: 10, TurnID: claim.Turn.ID, Lease: claim.Turn.Lease, Cost: checkEffect{Description: "无消耗"}, Outcomes: map[string]checkEffect{}}
	for _, grade := range checkGrades {
		in.Outcomes[grade] = checkEffect{Description: "预先声明结果"}
	}
	bad := in
	bad.EventID = future.EventID
	bad.ClientID = randomToken(18)
	f.request(t, agent, "POST", gameURL(c, "checks"), bad, 400, nil)
	var check abilityCheck
	f.request(t, agent, "POST", gameURL(c, "checks"), in, 200, &check)
	if check.UserID != 2 {
		t.Fatal("second action check reassigned to first player")
	}
	f.request(t, agent, "POST", gameURL(c, fmt.Sprintf("checks/%d/roll", check.ID)), map[string]any{"client_id": randomToken(18), "turn_id": claim.Turn.ID, "lease": claim.Turn.Lease}, 200, nil)
	f.request(t, agent, "POST", gameURL(c, fmt.Sprintf("turns/%d/complete", claim.Turn.ID)), gameCompleteInput{gameOp: gameOp{ClientID: randomToken(18), Version: claim.Turn.BaseVersion}, Lease: claim.Turn.Lease, Outcome: gameOutcome{Narration: "两人的行动已一起处理。", Phase: "awaiting_action", Changes: []gameCharacterChange{}}}, 200, nil)
	var count int
	f.app.store.QueryRow("SELECT count(*) FROM campaign_turns WHERE campaign_id=?", c.ID).Scan(&count)
	if count != 1 {
		t.Fatal("joint turn was split or future collection auto claimed")
	}
}

func TestCollectionOwnerReleaseIsExplicitAndIdempotent(t *testing.T) {
	f := testApp(t)
	bob := f.member(t, "bob")
	c := setTestGame(t, f, createTestGame(t, f, 2), "external", "awaiting_action")
	f.request(t, f.admin, "PUT", gameURL(c, "flow"), map[string]any{"client_id": randomToken(18), "version": c.Version, "mode": "collect", "participants": []int64{1, 2}}, 200, nil)
	f.request(t, bob, "POST", gameURL(c, "actions"), map[string]any{"client_id": randomToken(18), "content": "我先调查"}, 200, nil)
	f.request(t, f.admin, "PUT", gameURL(c, "flow"), map[string]any{"client_id": randomToken(18), "version": stateGame(t, f, c).Version, "mode": "individual"}, 409, nil)
	f.request(t, bob, "POST", gameURL(c, "flow/release"), map[string]any{"client_id": randomToken(18), "version": stateGame(t, f, c).Version, "allow_missing": true}, 403, nil)
	input := map[string]any{"client_id": randomToken(18), "version": stateGame(t, f, c).Version, "allow_missing": true}
	f.request(t, f.admin, "POST", gameURL(c, "flow/release"), input, 200, nil)
	f.request(t, f.admin, "POST", gameURL(c, "flow/release"), input, 200, nil)
	var count int
	f.app.store.QueryRow("SELECT count(*) FROM campaign_turns WHERE campaign_id=?", c.ID).Scan(&count)
	if count != 1 {
		t.Fatal("release retry duplicated joint turn")
	}
}
