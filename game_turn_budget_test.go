package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestToolBudgetsAreBoundedForJointActions(t *testing.T) {
	for _, tc := range []struct{ n, want int }{{0, 8}, {1, 8}, {3, 10}, {4, 12}, {12, 28}, {999, 28}} {
		if got := gameToolBudget(tc.n); got != tc.want {
			t.Fatalf("%d: %d != %d", tc.n, got, tc.want)
		}
	}
}
func TestHeartbeatCannotReviveExpiredOrExtendAbsoluteDeadline(t *testing.T) {
	f := testApp(t)
	bob := f.member(t, "bob")
	c := setTestGame(t, f, createTestGame(t, f, 2), "external", "awaiting_action")
	agent, _ := agentForGame(t, f, c, false)
	other, _ := agentForGame(t, f, c, false)
	f.request(t, bob, "POST", gameURL(c, "actions"), map[string]any{"client_id": randomToken(18), "content": "调查"}, 200, nil)
	var claim struct {
		Turn gameTurn `json:"turn"`
	}
	f.request(t, agent, "POST", gameURL(c, "turns/claim"), map[string]any{}, 200, &claim)
	url := gameURL(c, fmt.Sprintf("turns/%d/heartbeat", claim.Turn.ID))
	input := map[string]any{"lease": claim.Turn.Lease}
	f.request(t, other, "POST", url, input, 409, nil)
	before := claim.Turn.DeadlineAt
	f.app.store.Exec("UPDATE campaign_turns SET started_at=? WHERE id=?", time.Now().Add(-170*time.Second).UnixMilli(), claim.Turn.ID)
	var out struct {
		DeadlineAt int64 `json:"deadline_at"`
	}
	f.request(t, agent, "POST", url, input, 200, &out)
	if before != out.DeadlineAt {
		t.Fatal("heartbeat extended absolute deadline")
	}
	f.app.store.Exec("UPDATE campaign_turns SET started_at=0 WHERE id=?", claim.Turn.ID)
	f.request(t, agent, "POST", url, input, 409, nil)
	f.app.store.Exec("UPDATE campaign_turns SET started_at=? WHERE id=?", time.Now().UnixMilli(), claim.Turn.ID)
	f.app.store.Exec("UPDATE campaign_turn_runtime SET deadline_at=0 WHERE turn_id=?", claim.Turn.ID)
	f.request(t, agent, "POST", url, input, 409, nil)
	f.request(t, f.admin, "POST", gameURL(c, fmt.Sprintf("turns/%d/retry", claim.Turn.ID)), map[string]any{"client_id": randomToken(18), "version": stateGame(t, f, c).Version}, 200, nil)
	f.request(t, other, "POST", gameURL(c, "turns/claim"), map[string]any{}, 409, nil)
	f.request(t, agent, "POST", gameURL(c, "turns/claim"), map[string]any{}, 200, &claim)
	if !claim.Turn.Resuming || claim.Turn.DeadlineAt <= time.Now().UnixMilli() {
		t.Fatal("new attempt did not safely resume")
	}
}
func TestFourPlayerBuiltinResumesNineToolWorkflowWithoutDoubleCost(t *testing.T) {
	f := testApp(t)
	clients := []*http.Client{f.admin, f.member(t, "bob"), f.member(t, "carol"), f.member(t, "dave")}
	c := createTestGame(t, f, 2, 3, 4)
	for i, client := range clients {
		f.request(t, client, "POST", gameURL(c, "characters"), map[string]any{"client_id": randomToken(18), "version": stateGame(t, f, c).Version, "character": testCharacter(int64(i + 1))}, 200, nil)
	}
	var failed atomic.Bool
	var calls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Messages []gameModelMessage `json:"messages"`
		}
		if json.NewDecoder(r.Body).Decode(&input) != nil {
			w.WriteHeader(400)
			return
		}
		calls.Add(1)
		step := 0
		for _, m := range input.Messages {
			if m.Role == "assistant" {
				step++
			}
		}
		if step == 4 && !failed.Swap(true) {
			w.WriteHeader(503)
			return
		}
		var source struct {
			Context struct {
				Actions    []flowAction `json:"turn_actions"`
				Characters []character  `json:"characters"`
			} `json:"context"`
		}
		for _, m := range input.Messages {
			if m.Role == "user" {
				json.Unmarshal([]byte(m.Content.(string)), &source)
			}
		}
		call := gameToolCall{ID: fmt.Sprintf("step-%d", step), Type: "function"}
		if step < 8 && step%2 == 0 {
			action := source.Context.Actions[step/2]
			var ch character
			for _, candidate := range source.Context.Characters {
				if candidate.UserID == action.UserID {
					ch = candidate
				}
			}
			in := checkPrepareInput{EventID: action.EventID, CharacterID: ch.ID, Kind: "ability", Ability: "str", Reason: "联合调查", DC: 10, Cost: checkEffect{Description: "行动消耗", HPDelta: -1}, Outcomes: map[string]checkEffect{}}
			for _, grade := range checkGrades {
				in.Outcomes[grade] = checkEffect{Description: "已声明后果"}
			}
			// Model schema excludes lease/version/client_id: send only tool fields.
			args := map[string]any{"event_id": in.EventID, "character_id": in.CharacterID, "kind": in.Kind, "ability": in.Ability, "reason": in.Reason, "dc": in.DC, "cost": in.Cost, "outcomes": in.Outcomes}
			call.Function.Name = "prepare_check"
			call.Function.Arguments = gameJSON(args)
		} else if step < 8 {
			var check abilityCheck
			json.Unmarshal([]byte(input.Messages[len(input.Messages)-1].Content.(string)), &check)
			call.Function.Name = "roll_check"
			call.Function.Arguments = gameJSON(map[string]any{"check_id": check.ID})
		} else {
			call.Function.Name = "finish_turn"
			call.Function.Arguments = gameJSON(gameOutcome{Narration: "四人的调查均已结算。", Phase: "awaiting_action", Changes: []gameCharacterChange{}})
		}
		writeJSON(w, 200, map[string]any{"choices": []any{map[string]any{"message": gameModelMessage{Role: "assistant", ToolCalls: []gameToolCall{call}}}}})
	}))
	defer provider.Close()
	f.request(t, f.admin, "PUT", "/api/v1/entertainment/ai", map[string]any{"base_url": provider.URL + "/v1", "model": "joint-checkpoint-test", "api_key": "fixture-only", "enabled": true}, 200, nil)
	c = setTestGame(t, f, c, "builtin", "awaiting_action")
	f.request(t, f.admin, "PUT", gameURL(c, "flow"), map[string]any{"client_id": randomToken(18), "version": c.Version, "mode": "collect", "participants": []int64{1, 2, 3, 4}}, 200, nil)
	var action struct {
		TurnID int64 `json:"turn_id"`
	}
	for _, client := range clients {
		f.request(t, client, "POST", gameURL(c, "actions"), map[string]any{"client_id": randomToken(18), "content": "我调查自己的方向"}, 200, &action)
	}
	waitGame(t, f, c, "failed")
	f.request(t, f.admin, "POST", gameURL(c, fmt.Sprintf("turns/%d/retry", action.TurnID)), map[string]any{"client_id": randomToken(18), "version": stateGame(t, f, c).Version}, 200, nil)
	waitGame(t, f, c, "done")
	var checks int
	f.app.store.QueryRow("SELECT count(*) FROM campaign_checks WHERE campaign_id=? AND status='resolved'", c.ID).Scan(&checks)
	if checks != 4 || calls.Load() != 10 {
		t.Fatalf("checks=%d calls=%d; expected four unique checks and replay-free model recovery", checks, calls.Load())
	}
	rows, err := f.app.store.Query("SELECT hp FROM characters WHERE campaign_id=?", c.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var hp int
		rows.Scan(&hp)
		if hp != testCharacter(1).HP-1 {
			t.Fatal("resource deducted twice", hp)
		}
	}
}
