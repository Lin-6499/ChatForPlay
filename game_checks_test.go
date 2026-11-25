package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
)

func TestGradedCheckBoundariesAndNaturalPriority(t *testing.T) {
	cases := []struct {
		natural, modifier, dc int
		grade                 string
	}{
		{20, -20, 40, "critical_success"}, {1, 20, 1, "critical_failure"},
		{15, 5, 15, "great_success"}, {14, 5, 15, "success"}, {10, 5, 15, "success"},
		{9, 5, 15, "minor_failure"}, {6, 5, 15, "minor_failure"}, {5, 5, 15, "failure"},
		{16, 4, 20, "success"}, {2, -2, 10, "failure"},
	}
	for _, c := range cases {
		if got := gradedCheck(c.natural, c.natural+c.modifier, c.dc); got != c.grade {
			t.Fatalf("d20 %d + %d / DC %d: %s, want %s", c.natural, c.modifier, c.dc, got, c.grade)
		}
	}
}
func checkFixture(t *testing.T) (*fixture, *http.Client, *http.Client, campaign, character) {
	t.Helper()
	f := testApp(t)
	bob, carol := f.member(t, "bob"), f.member(t, "carol")
	c := createTestGame(t, f, 2, 3)
	ch := testCharacter(2)
	ch.Abilities["str"] = 9
	f.request(t, bob, "POST", gameURL(c, "characters"), map[string]any{"client_id": randomToken(18), "version": c.Version, "character": ch}, 200, &ch)
	c = setTestGame(t, f, c, "manual", "awaiting_action")
	return f, bob, carol, c, ch
}
func checkInput(t *testing.T, f *fixture, bob *http.Client, c campaign, ch character) checkPrepareInput {
	t.Helper()
	var action struct {
		EventID int64 `json:"event_id"`
	}
	f.request(t, bob, "POST", gameURL(c, "actions"), map[string]any{"client_id": randomToken(18), "content": "我尝试打开木门"}, 200, &action)
	in := checkPrepareInput{gameOp: gameOp{ClientID: randomToken(18), Version: stateGame(t, f, c).Version},
		EventID: action.EventID, CharacterID: ch.ID, Kind: "ability", Ability: "str", Reason: "开门", DC: 15,
		Cost: checkEffect{Description: "尝试花费体力", HPDelta: -1}, Outcomes: map[string]checkEffect{}}
	for _, grade := range checkGrades {
		in.Outcomes[grade] = checkEffect{Description: "预定后果：" + grade, HPDelta: -2}
	}
	return in
}
func TestAbilityChecksLockedOwnedDurableAndExactlyOnce(t *testing.T) {
	f, bob, carol, c, ch := checkFixture(t)
	in := checkInput(t, f, bob, c, ch)
	f.request(t, bob, "POST", gameURL(c, "checks"), in, 403, nil)
	bad := in
	bad.Kind = "attack"
	f.request(t, f.admin, "POST", gameURL(c, "checks"), bad, 400, nil)
	var check abilityCheck
	f.request(t, f.admin, "POST", gameURL(c, "checks"), in, 200, &check)
	if check.AbilityModifier != -1 || check.Modifier != -1 || check.UserID != 2 || check.Natural != 0 {
		t.Fatalf("unexpected locked check: %+v", check)
	}
	in.DC++
	f.request(t, f.admin, "POST", gameURL(c, "checks"), in, 409, nil)
	in.ClientID = randomToken(18)
	f.request(t, f.admin, "POST", gameURL(c, "checks"), in, 409, nil)
	f.request(t, bob, "POST", gameURL(c, "characters"), map[string]any{"client_id": randomToken(18), "version": c.Version, "character": ch}, 409, nil)
	route := gameURL(c, fmt.Sprintf("checks/%d/roll", check.ID))
	f.request(t, carol, "POST", route, map[string]any{"client_id": randomToken(18)}, 403, nil)
	f.request(t, bob, "POST", route, map[string]any{"client_id": randomToken(18), "natural": 20}, 400, nil)
	var results [2]abilityCheck
	var wg sync.WaitGroup
	// Different request IDs and different authorized actors must still roll only once.
	for i, client := range []*http.Client{f.admin, bob} {
		wg.Add(1)
		go func(i int, client *http.Client) {
			defer wg.Done()
			f.request(t, client, "POST", route, map[string]any{"client_id": randomToken(18)}, 200, &results[i])
		}(i, client)
	}
	wg.Wait()
	if gameJSON(results[0]) != gameJSON(results[1]) {
		t.Fatal("concurrent requests rerolled or double-applied")
	}
	result := results[0]
	if result.Natural < 1 || result.Natural > 20 || result.Total != result.Natural-1 || result.After.HP != ch.HP-3 {
		t.Fatalf("bad result: %+v", result)
	}
	var hp, version int
	f.app.store.QueryRow("SELECT hp,version FROM characters WHERE id=?", ch.ID).Scan(&hp, &version)
	if hp != 9 || version != 2 {
		t.Fatalf("resources applied more than once: hp %d, version %d", hp, version)
	}
	var counts int
	f.app.store.QueryRow("SELECT COUNT(*) FROM campaign_events WHERE campaign_id=? AND kind='check'", c.ID).Scan(&counts)
	if counts != 2 {
		t.Fatalf("expected prepare and roll events, got %d", counts)
	}
	f.request(t, f.admin, "POST", gameURL(c, fmt.Sprintf("checks/%d/cancel", check.ID)), map[string]any{"client_id": randomToken(18), "version": stateGame(t, f, c).Version}, 409, nil)
	reopened, err := openStore(filepath.Join(f.app.cfg.DataDir, "linchat.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	persisted, err := loadAbilityCheck(reopened.QueryRow("SELECT document FROM campaign_checks WHERE id=?", check.ID))
	if err != nil || gameJSON(persisted) != gameJSON(result) {
		t.Fatal("check did not survive database reopening")
	}
}
func TestAbilityCheckBindsActionActorAndCanCancelUnrolled(t *testing.T) {
	f, bob, carol, c, ch := checkFixture(t)
	in := checkInput(t, f, bob, c, ch)
	var other struct {
		EventID int64 `json:"event_id"`
	}
	f.request(t, carol, "POST", gameURL(c, "actions"), map[string]any{"client_id": randomToken(18), "content": "另一位玩家的行动"}, 200, &other)
	wrong := in
	wrong.EventID = other.EventID
	f.request(t, f.admin, "POST", gameURL(c, "checks"), wrong, 400, nil)
	var check abilityCheck
	f.request(t, f.admin, "POST", gameURL(c, "checks"), in, 200, &check)
	f.request(t, f.admin, "POST", gameURL(c, fmt.Sprintf("checks/%d/cancel", check.ID)), map[string]any{"client_id": randomToken(18), "version": c.Version}, 200, &check)
	if check.Status != "cancelled" || check.Natural != 0 || check.After.HP != ch.HP {
		t.Fatal("cancellation changed resources")
	}
	f.request(t, bob, "POST", gameURL(c, "characters"), map[string]any{"client_id": randomToken(18), "version": c.Version, "character": ch}, 200, nil)
}
func TestCheckChoiceOnlyPlayerAndNoDoubleCharge(t *testing.T) {
	for _, accept := range []bool{true, false} {
		t.Run(fmt.Sprint(accept), func(t *testing.T) {
			f, bob, carol, c, ch := checkFixture(t)
			in := checkInput(t, f, bob, c, ch)
			in.Offer = &checkEffect{Description: "接受擦伤，打开木门", HPDelta: -3}
			var check abilityCheck
			f.request(t, f.admin, "POST", gameURL(c, "checks"), in, 200, &check)
			// Seed an already-rolled minor-failure fixture; random boundary arithmetic
			// is covered separately without injecting a controllable dice endpoint.
			check.Status = "awaiting_choice"
			check.Natural = 12
			check.Total = 11
			check.Grade = "minor_failure"
			check.Margin = -4
			check.After.HP--
			check.CharacterVersion++
			tx, err := f.app.store.Begin()
			if err != nil {
				t.Fatal(err)
			}
			tx.Exec("UPDATE characters SET hp=hp-1,version=version+1 WHERE id=?", ch.ID)
			if err = saveAbilityCheck(tx, check); err != nil {
				t.Fatal(err)
			}
			if err = tx.Commit(); err != nil {
				t.Fatal(err)
			}
			agent, _ := agentForGame(t, f, c, true)
			if accept {
				c = setTestGame(t, f, c, "external", "awaiting_action")
				for i := 0; i < 20; i++ {
					f.request(t, bob, "POST", gameURL(c, "actions"), map[string]any{"client_id": randomToken(18), "content": "排队行动"}, 200, nil)
				}
				f.request(t, agent, "POST", gameURL(c, "turns/claim"), map[string]any{}, 409, nil)
			}
			route := gameURL(c, fmt.Sprintf("checks/%d/choose", check.ID))
			body := map[string]any{"client_id": randomToken(18), "accept": accept}
			for _, client := range []*http.Client{f.admin, carol, agent} {
				f.request(t, client, "POST", route, body, 403, nil)
			}
			f.request(t, bob, "POST", route, map[string]any{"client_id": randomToken(18)}, 400, nil)
			var first, repeat abilityCheck
			f.request(t, bob, "POST", route, body, 200, &first)
			body["client_id"] = randomToken(18)
			f.request(t, bob, "POST", route, body, 200, &repeat)
			want := 9
			if accept {
				want = 8
			}
			if first.After.HP != want || gameJSON(first) != gameJSON(repeat) || first.Accepted == nil || *first.Accepted != accept {
				t.Fatalf("choice result incorrect: %+v", first)
			}
			if accept {
				var queued int
				f.app.store.QueryRow("SELECT COUNT(*) FROM campaign_turns WHERE campaign_id=?", c.ID).Scan(&queued)
				if queued != 21 {
					t.Fatalf("full queue blocked or duplicated choice continuation: %d", queued)
				}
				f.request(t, agent, "POST", gameURL(c, "turns/claim"), map[string]any{}, 200, nil)
			}
			body["client_id"] = randomToken(18)
			body["accept"] = !accept
			f.request(t, bob, "POST", route, body, 409, nil)
		})
	}
}
func TestCheckLeaseRecoveryAndCompletionCannotDoubleApply(t *testing.T) {
	f, bob, _, c, ch := checkFixture(t)
	c = setTestGame(t, f, c, "external", "awaiting_action")
	agent, _ := agentForGame(t, f, c, false)
	in := checkInput(t, f, bob, c, ch)
	var claim struct {
		Turn    gameTurn        `json:"turn"`
		Context json.RawMessage `json:"context"`
	}
	f.request(t, agent, "POST", gameURL(c, "turns/claim"), map[string]any{}, 200, &claim)
	in.TurnID = claim.Turn.ID
	in.Lease = claim.Turn.Lease
	var check abilityCheck
	f.request(t, agent, "POST", gameURL(c, "checks"), in, 200, &check)
	f.request(t, agent, "POST", gameURL(c, fmt.Sprintf("turns/%d/fail", claim.Turn.ID)), map[string]any{"client_id": randomToken(18), "lease": claim.Turn.Lease}, 200, nil)
	f.request(t, f.admin, "POST", gameURL(c, fmt.Sprintf("turns/%d/retry", claim.Turn.ID)), map[string]any{"client_id": randomToken(18), "version": stateGame(t, f, c).Version}, 200, nil)
	previousLease := claim.Turn.Lease
	f.request(t, agent, "POST", gameURL(c, "turns/claim"), map[string]any{}, 200, &claim)
	if claim.Turn.Lease == previousLease {
		t.Fatal("retry did not renew the lease")
	}
	in.Lease = claim.Turn.Lease
	var recovered abilityCheck
	f.request(t, agent, "POST", gameURL(c, "checks"), in, 200, &recovered)
	if recovered.ID != check.ID {
		t.Fatal("recovery created another check")
	}
	complete := gameCompleteInput{gameOp: gameOp{ClientID: randomToken(18), Version: claim.Turn.BaseVersion}, Lease: claim.Turn.Lease, Outcome: gameOutcome{Narration: "继续", Phase: "awaiting_action", Changes: []gameCharacterChange{}}}
	route := gameURL(c, fmt.Sprintf("turns/%d/complete", claim.Turn.ID))
	f.request(t, agent, "POST", route, complete, 409, nil)
	roll := checkRollInput{gameOp: gameOp{ClientID: randomToken(18)}, TurnID: claim.Turn.ID, Lease: "wrong"}
	f.request(t, agent, "POST", gameURL(c, fmt.Sprintf("checks/%d/roll", check.ID)), roll, 409, nil)
	roll.Lease = claim.Turn.Lease
	f.request(t, agent, "POST", gameURL(c, fmt.Sprintf("checks/%d/roll", check.ID)), roll, 200, &check)
	hp := 6
	complete.Outcome.Changes = []gameCharacterChange{{ID: ch.ID, HP: &hp}}
	f.request(t, agent, "POST", route, complete, 409, nil)
	complete.Outcome.Changes = []gameCharacterChange{}
	f.request(t, agent, "POST", route, complete, 200, nil)
	if stateGame(t, f, c).Version != claim.Turn.BaseVersion+1 {
		t.Fatal("check invalidated turn base version")
	}
}

func TestBuiltInHostUsesLockedChecks(t *testing.T) {
	f, bob, _, c, ch := checkFixture(t)
	outcomes := map[string]checkEffect{}
	for _, grade := range checkGrades {
		outcomes[grade] = checkEffect{Description: grade}
	}
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Messages []gameModelMessage `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		name := "prepare_check"
		args := gameJSON(map[string]any{"character_id": ch.ID, "kind": "ability", "ability": "str", "reason": "开门", "dc": 15, "cost": checkEffect{Description: "体力消耗", HPDelta: -1}, "outcomes": outcomes})
		last := input.Messages[len(input.Messages)-1]
		if last.Role == "tool" {
			var check abilityCheck
			if err := json.Unmarshal([]byte(last.Content.(string)), &check); err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
			if check.Status == "prepared" {
				name = "roll_check"
				args = gameJSON(map[string]any{"check_id": check.ID})
			} else {
				name = "finish_turn"
				args = gameJSON(gameOutcome{Narration: "使用软件判定结果继续故事", Phase: "awaiting_action", Changes: []gameCharacterChange{}})
			}
		}
		call := gameToolCall{ID: randomToken(18), Type: "function"}
		call.Function.Name = name
		call.Function.Arguments = args
		writeJSON(w, 200, map[string]any{"choices": []any{map[string]any{"message": gameModelMessage{Role: "assistant", Content: nil, ToolCalls: []gameToolCall{call}}}}})
	}))
	defer provider.Close()
	f.request(t, f.admin, "PUT", "/api/v1/entertainment/ai", map[string]any{"base_url": provider.URL + "/v1", "model": "fixture-checks", "api_key": "fixture-only", "enabled": true}, 200, nil)
	c = setTestGame(t, f, c, "builtin", "awaiting_action")
	f.request(t, bob, "POST", gameURL(c, "actions"), map[string]any{"client_id": randomToken(18), "content": "我尝试开门"}, 200, nil)
	waitGame(t, f, c, "done")
	var list []abilityCheck
	f.request(t, bob, "GET", gameURL(c, "checks"), nil, 200, &list)
	if len(list) != 1 || list[0].Status != "resolved" || list[0].After.HP != ch.HP-1 || list[0].Natural < 1 {
		t.Fatalf("built-in check flow incomplete: %+v", list)
	}
}
