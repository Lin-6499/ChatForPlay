package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestCapabilitiesAndPrivateHostDiagnostics(t *testing.T) {
	f := testApp(t)
	bob := f.member(t, "bob")
	outsider := f.member(t, "outsider")
	c := setTestGame(t, f, createTestGame(t, f, 2), "external", "awaiting_action")
	agent, _ := agentForGame(t, f, c, false)
	var caps struct {
		ServerVersion string                 `json:"server_version"`
		Protocol      struct{ Min, Max int } `json:"protocol"`
		Features      []string               `json:"features"`
	}
	f.request(t, agent, "GET", gameURL(c, "capabilities"), nil, 200, &caps)
	if caps.ServerVersion != serverVersion || caps.Protocol.Min != 1 || len(caps.Features) < 9 {
		t.Fatal("missing capability declarations")
	}
	f.request(t, outsider, "GET", gameURL(c, "capabilities"), nil, 403, nil)
	f.request(t, bob, "GET", gameURL(c, "host-diagnostics"), nil, 403, nil)
	f.request(t, agent, "GET", gameURL(c, "host-diagnostics"), nil, 403, nil)
	f.request(t, bob, "POST", gameURL(c, "actions"), map[string]any{"client_id": randomToken(18), "content": "我调查门轴"}, 200, nil)
	var claim struct {
		Turn gameTurn `json:"turn"`
	}
	f.request(t, agent, "POST", gameURL(c, "turns/claim"), map[string]any{}, 200, &claim)
	f.request(t, agent, "POST", gameURL(c, fmt.Sprintf("turns/%d/stream", claim.Turn.ID)), gameStreamInput{Lease: claim.Turn.Lease, Version: claim.Turn.BaseVersion, Seq: 1, Narration: "门后传来声音"}, 200, nil)
	f.request(t, agent, "POST", gameURL(c, fmt.Sprintf("turns/%d/fail", claim.Turn.ID)), map[string]any{"client_id": randomToken(18), "lease": claim.Turn.Lease}, 200, nil)
	var diagnostics map[string]any
	f.request(t, f.admin, "GET", gameURL(c, "host-diagnostics"), nil, 200, &diagnostics)
	for _, key := range []string{"contact_at", "claim_at", "stream_at", "fail_at"} {
		if diagnostics[key].(float64) <= 0 {
			t.Fatal("missing timestamp", key)
		}
	}
	raw, _ := json.Marshal(diagnostics)
	if strings.Contains(string(raw), claim.Turn.Lease) || strings.Contains(string(raw), "api_key") {
		t.Fatal("credential leaked")
	}
	f.request(t, f.admin, "POST", gameURL(c, fmt.Sprintf("turns/%d/retry", claim.Turn.ID)), map[string]any{"client_id": randomToken(18), "version": stateGame(t, f, c).Version}, 200, nil)
	f.request(t, agent, "POST", gameURL(c, "turns/claim"), map[string]any{}, 200, &claim)
	f.request(t, agent, "POST", gameURL(c, fmt.Sprintf("turns/%d/complete", claim.Turn.ID)), gameCompleteInput{gameOp: gameOp{ClientID: randomToken(18), Version: claim.Turn.BaseVersion}, Lease: claim.Turn.Lease, Outcome: gameOutcome{Narration: "调查已完成。", Phase: "awaiting_action", Changes: []gameCharacterChange{}}}, 200, nil)
	f.request(t, f.admin, "GET", gameURL(c, "host-diagnostics"), nil, 200, &diagnostics)
	if diagnostics["complete_at"].(float64) <= 0 {
		t.Fatal("completion activity missing")
	}
}
