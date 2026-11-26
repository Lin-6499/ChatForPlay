package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestEstateQuotesAndExactRedemption(t *testing.T) {
	for _, tile := range []int{1, 13} {
		g := estateFixtureGame()
		g.Properties[tile] = estateProperty{Owner: 1, Level: 1, Mortgaged: true}
		cost := map[int]int{1: 110, 13: 220}[tile]
		g.Players[0].Cash = cost
		if g.quotes()[tile].RedeemCost != cost {
			t.Fatal("quote is not exact")
		}
		if err := g.action(1, "redeem", tile, 1000); err != nil {
			t.Fatal(err)
		}
		if g.Players[0].Cash != 0 || g.Properties[tile].Mortgaged {
			t.Fatal("exact-balance redemption failed")
		}
	}
	g := estateFixtureGame()
	for _, tile := range []int{1, 3, 5, 8, 16} {
		g.Properties[tile] = estateProperty{Owner: 1, Level: 1}
	}
	g.Properties[1].Level = 2
	quotes := g.quotes()
	if len(quotes[3].MortgageRents) != 3 || len(quotes[8].MortgageRents) != 2 {
		t.Fatal("group/station impact missing")
	}
	if quotes[1].SellRents[0] != (estateRentChange{1, 112, 37}) {
		t.Fatalf("sell quote: %+v", quotes[1])
	}
	if g.Properties[1].Level != 2 || g.Properties[3].Mortgaged {
		t.Fatal("quoting mutated game")
	}
}

// Used only by the isolated headless edge-state test. No production endpoint or
// fixed dice hook is added. The debt scenario is entered through the real rule.
func TestEstateUIFixtures(t *testing.T) {
	dir := os.Getenv("LINCHAT_ESTATE_FIXTURE_DATA")
	if dir == "" {
		t.Skip("only used by scripts/estate-edge-check.cjs")
	}
	dir, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs(filepath.Join("dist", "verification"))
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(dir) != root || !strings.HasPrefix(filepath.Base(dir), "campaign-demo-") {
		t.Fatal("fixture must be an isolated campaign-demo directory")
	}
	db, err := openStore(filepath.Join(dir, "linchat.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var name string
	if db.QueryRow("SELECT username FROM users WHERE id=1").Scan(&name) != nil || name != "demo_owner" {
		t.Fatal("not a fixture database")
	}
	ids := map[string]int64{}
	for _, scenario := range []string{"redeem110", "redeem220", "debt", "crowded"} {
		tx, err := db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		now := time.Now().UnixMilli()
		res, err := tx.Exec("INSERT INTO conversations(kind,name,owner_id,created_at) VALUES('group',?,1,?)", "边界验收 "+scenario, now)
		if err != nil {
			t.Fatal(err)
		}
		cid, _ := res.LastInsertId()
		for _, uid := range []int{1, 2, 3} {
			if _, err = tx.Exec("INSERT INTO members(conversation_id,user_id) VALUES(?,?)", cid, uid); err != nil {
				t.Fatal(err)
			}
		}
		res, err = tx.Exec("INSERT INTO estate_games(conversation_id,document) VALUES(?,'{}')", cid)
		if err != nil {
			t.Fatal(err)
		}
		id, _ := res.LastInsertId()
		g := estateFixtureGame()
		g.ID = id
		g.ConversationID = cid
		g.Title = "规则边界 · " + scenario
		g.OwnerID = 1
		g.Capacity = 3
		g.Players[0].Name = "林同学"
		g.Players[1].Name = "小满"
		g.Players[1].Token = 1
		g.Players = append(g.Players, estatePlayer{ID: 3, Name: "已破产玩家", Token: 2, Bankrupt: true})
		g.begin(now)
		if scenario == "crowded" {
			g.Capacity = 4
			g.Players[2].Bankrupt = false
			g.Players[2].Name = "另一位玩家"
			g.Players[2].Cash = 2000
			g.Players = append(g.Players, estatePlayer{ID: 4, Name: "四号伙伴", Token: 3, Cash: 2000})
			for i := range g.Players {
				g.Players[i].Position = 1
			}
			g.Properties[1] = estateProperty{Owner: 2, Level: 3}
			g.Properties[3] = estateProperty{Owner: 2, Level: 1}
		} else if scenario == "debt" {
			g.Players[0].Cash = 10
			for _, tile := range []int{1, 3, 5, 8, 16} {
				g.Properties[tile] = estateProperty{Owner: 1, Level: 1}
			}
			g.Properties[1].Level = 2
			g.pay(270, 2, now)
		} else {
			tile, cash := 1, 110
			if scenario == "redeem220" {
				tile, cash = 13, 220
			}
			g.Players[0].Cash = cash
			g.Properties[tile] = estateProperty{Owner: 1, Level: 1, Mortgaged: true}
		}
		if err = saveEstate(tx, &g); err != nil {
			t.Fatal(err)
		}
		if err = tx.Commit(); err != nil {
			t.Fatal(err)
		}
		ids[scenario] = id
	}
	b, _ := json.Marshal(ids)
	if err = os.WriteFile(filepath.Join(dir, "estate-ui-fixtures.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
}

func estateFixtureGame() estateGame {
	return estateGame{ID: 1, Ruleset: "estate-v1", Phase: "management", Round: 1, Current: 0, Version: 1, Properties: make([]estateProperty, 24),
		Players: []estatePlayer{{ID: 1, Name: "甲", Cash: 2000}, {ID: 2, Name: "乙", Cash: 2000}},
		Deck:    []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}, Events: []estateEvent{}}
}
func TestEstateEconomyAndDebt(t *testing.T) {
	g := estateFixtureGame()
	now := int64(1000)
	g.Properties[1] = estateProperty{Owner: 1, Level: 1}
	if err := g.action(2, "upgrade", 1, now); err == nil {
		t.Fatal("other player's action accepted")
	}
	if err := g.action(1, "upgrade", 1, now); err != nil {
		t.Fatal(err)
	}
	if g.Players[0].Cash != 1900 || g.rent(1) != 90 {
		t.Fatal("upgrade incorrect")
	}
	if err := g.action(1, "upgrade", 1, now); err == nil {
		t.Fatal("double upgrade")
	}
	g.Properties[3] = estateProperty{Owner: 1, Level: 1}
	g.Properties[5] = estateProperty{Owner: 1, Level: 1}
	if g.rent(1) != 112 {
		t.Fatalf("group rounding: %d", g.rent(1))
	}
	if err := g.action(1, "mortgage", 3, now); err != nil {
		t.Fatal(err)
	}
	if g.rent(1) != 90 || g.rent(3) != 0 {
		t.Fatal("mortgage should disable group bonus and own rent")
	}
	if err := g.action(1, "redeem", 3, now); err != nil {
		t.Fatal(err)
	}
	if g.Players[0].Cash != 1890 {
		t.Fatal("redemption fee")
	}
	g.Current = 1
	g.Players[1].Position = 1
	g.Players[1].Cash = 10
	g.Players[1].Coupon = true
	g.land(now)
	if g.Players[1].Cash != 10 || g.Players[1].Coupon || g.Debt != 0 {
		t.Fatal("coupon")
	}
	g.land(now)
	if g.Phase != "debt" || g.Debt != 112 || g.Players[1].Cash < 0 {
		t.Fatal("debt state")
	}
	g.Properties[7] = estateProperty{Owner: 2, Level: 2}
	g.liquidate(now)
	if g.Players[1].Cash != 123 || g.Players[0].Cash != 2002 || !g.Properties[7].Mortgaged || g.Phase != "end_turn" {
		t.Fatalf("liquidation %+v", g)
	}
	if g.Players[0].RentEarned != 112 {
		t.Fatal("rent ledger")
	}
}
func TestEstateBankruptcyAndScores(t *testing.T) {
	g := estateFixtureGame()
	g.Properties[1] = estateProperty{Owner: 1, Level: 2}
	g.Properties[3] = estateProperty{Owner: 1, Level: 1, Mortgaged: true}
	s := g.scores()
	if s[0].ID != 1 || s[0].Total != 2340 || s[0].Buildings != 50 || s[0].Equity != 90 {
		t.Fatalf("net worth %+v", s)
	}
	g.Current = 0
	g.Players[0].Cash = 0
	g.pay(1000, 2, 1000)
	g.liquidate(61000)
	if !g.Players[0].Bankrupt || g.Phase != "finished" || g.Players[1].Cash != 2150 {
		t.Fatalf("bankruptcy %+v", g.Players)
	}
	for _, pr := range g.Properties {
		if pr.Owner == 1 {
			t.Fatal("bank did not recover land")
		}
	}
	if g.scores()[0].ID != 2 {
		t.Fatal("winner")
	}
	g = estateFixtureGame()
	s = g.scores()
	if s[0].Rank != 1 || s[1].Rank != 1 {
		t.Fatal("tied rank")
	}
}
func TestEstateCardsCheckAndLap(t *testing.T) {
	for card := 0; card < 12; card++ {
		g := estateFixtureGame()
		g.Deck = []int{card}
		g.Players[0].Position = 20
		g.draw(1000)
		if g.LastCard != estateCards[card] {
			t.Fatal("card identity")
		}
		if card == 8 && (g.Players[0].Position != 8 || g.Players[0].Cash != 2000 || g.Phase != "purchase_decision") {
			t.Fatal("station movement")
		}
		if card == 9 && (g.Players[0].Position != 6 || g.Players[0].Cash != 2000) {
			t.Fatal("rest movement")
		}
		if card == 10 && (!g.Players[0].Checked || g.Players[0].Position != 12) {
			t.Fatal("check card")
		}
	}
	g := estateFixtureGame()
	g.Players[0].Position = 22
	g.roll([2]int{1, 1}, 1000)
	if g.Players[0].Position != 0 || g.Players[0].Cash != 2300 || g.Phase != "end_turn" {
		t.Fatal("lap / doubles")
	}
	g.next(3000)
	if g.Current != 1 {
		t.Fatal("doubles granted extra turn")
	}
	g.Current = 0
	g.Players[0].Checked = true
	g.begin(5000)
	if err := g.action(1, "upgrade", 1, 5000); err == nil {
		t.Fatal("checked management allowed")
	}
	if err := g.action(1, "check_release", 0, 6000); err != nil {
		t.Fatal(err)
	}
	if g.Deadline != 50000 || g.Phase != "management" || g.Players[0].Cash != 2200 {
		t.Fatal("check release must retain original deadline")
	}
}
func TestEstateCompleteTwelveRounds(t *testing.T) {
	for n := 2; n <= 4; n++ {
		g := estateFixtureGame()
		for len(g.Players) < n {
			i := len(g.Players) + 1
			g.Players = append(g.Players, estatePlayer{ID: int64(i), Name: fmt.Sprint(i), Cash: 2000})
		}
		g.begin(1000)
		now := int64(1000)
		for step := 0; g.Phase != "finished" && step < 500; step++ {
			now += 65000
			if !g.expire(now) {
				t.Fatalf("stalled phase %s", g.Phase)
			}
			for _, p := range g.Players {
				if p.Cash < 0 {
					t.Fatal("negative cash")
				}
			}
		}
		if g.Phase != "finished" || g.Round != 12 {
			t.Fatalf("%d players did not finish: %s round %d", n, g.Phase, g.Round)
		}
		for _, p := range g.Players {
			if !p.Bankrupt && p.Turns != 12 {
				t.Fatalf("unequal turns %+v", p)
			}
		}
	}
}

type estateTestView struct {
	Game   estateGame    `json:"game"`
	Scores []estateScore `json:"scores"`
}

func createEstateTest(t *testing.T, f *fixture) estateTestView {
	var v estateTestView
	f.request(t, f.admin, "POST", "/api/v1/estate/rooms", map[string]any{"client_id": randomToken(18), "title": "棋盘测试", "capacity": 4}, 200, &v)
	return v
}
func TestEstateHTTPPermissionsReceiptsAndSecrecy(t *testing.T) {
	f := testApp(t)
	member := f.member(t, "bob")
	outsider := f.member(t, "eve")
	v := createEstateTest(t, f)
	base := fmt.Sprintf("/api/v1/estate/rooms/%d/", v.Game.ID)
	match := fmt.Sprintf("/api/v1/estate/matches/%d", v.Game.ID)
	f.request(t, outsider, "GET", match, nil, 403, nil)
	f.request(t, member, "POST", base+"join", map[string]any{"client_id": randomToken(18), "version": v.Game.Version}, 200, &v)
	f.request(t, f.admin, "POST", base+"ready", map[string]any{"client_id": randomToken(18), "version": v.Game.Version, "ready": true}, 200, &v)
	f.request(t, member, "POST", base+"ready", map[string]any{"client_id": randomToken(18), "version": v.Game.Version, "ready": true}, 200, &v)
	f.request(t, member, "POST", base+"start", map[string]any{"client_id": randomToken(18), "version": v.Game.Version}, 403, nil)
	f.request(t, f.admin, "POST", base+"start", map[string]any{"client_id": randomToken(18), "version": v.Game.Version}, 200, &v)
	if len(v.Game.Deck) > 0 || v.Game.DeckCursor != 0 {
		t.Fatal("private deck leaked")
	}
	var raw map[string]any
	f.request(t, f.admin, "GET", match, nil, 200, &raw)
	if strings.Contains(gameJSON(raw), `"deck"`) {
		t.Fatal("deck field leaked")
	}
	current, other := f.admin, member
	if v.Game.Players[v.Game.Current].ID == 2 {
		current, other = member, f.admin
	}
	op := map[string]any{"client_id": randomToken(18), "version": v.Game.Version, "action": "roll"}
	f.request(t, other, "POST", match+"/actions", op, 403, nil)
	var first, again estateTestView
	f.request(t, current, "POST", match+"/actions", op, 200, &first)
	f.request(t, current, "POST", match+"/actions", op, 200, &again)
	if gameJSON(first) != gameJSON(again) {
		t.Fatal("retry changed receipt")
	}
	op["action"] = "buy"
	f.request(t, current, "POST", match+"/actions", op, 409, nil)
	f.request(t, current, "POST", match+"/actions", map[string]any{"client_id": randomToken(18), "version": v.Game.Version, "action": "roll"}, 409, nil)
	f.request(t, outsider, "POST", base+"join", map[string]any{"client_id": randomToken(18), "version": first.Game.Version}, 409, nil)
	// Chat membership exists only for joined players, and doesn't trigger a turn.
	f.request(t, member, "POST", fmt.Sprintf("/api/conversations/%d/messages", v.Game.ConversationID), map[string]any{"client_id": randomToken(18), "content": "一起玩"}, 201, nil)
}
func TestEstateConcurrentCommandsAndPersistentTimeout(t *testing.T) {
	f := testApp(t)
	v := createEstateTest(t, f)
	tx, err := f.app.store.Begin()
	if err != nil {
		t.Fatal(err)
	}
	g, err := loadEstate(tx, v.Game.ID)
	if err != nil {
		t.Fatal(err)
	}
	g.Phase = "management"
	g.Round = 1
	g.Players = append(g.Players, estatePlayer{ID: 2, Name: "fixture", Cash: 2000})
	g.Deck = []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}
	g.Deadline = time.Now().Add(time.Minute).UnixMilli()
	if err = saveEstate(tx, &g); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	route := fmt.Sprintf("/api/v1/estate/matches/%d/actions", g.ID)
	var wg sync.WaitGroup
	statuses := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			body := gameJSON(map[string]any{"client_id": randomToken(18), "version": g.Version, "action": "roll"})
			req, _ := http.NewRequest("POST", f.server.URL+route, strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-LinChat", "1")
			resp, err := f.admin.Do(req)
			if err != nil {
				statuses <- 0
				return
			}
			resp.Body.Close()
			statuses <- resp.StatusCode
		}()
	}
	wg.Wait()
	close(statuses)
	counts := map[int]int{}
	for s := range statuses {
		counts[s]++
	}
	if counts[200] != 1 || counts[409] != 1 {
		t.Fatalf("race result %v", counts)
	}
	// Persist a due decision; closing/reopening the store keeps its deadline and
	// re-running expiration must not apply it twice.
	f.app.closeGames()
	tx, _ = f.app.store.Begin()
	g, _ = loadEstate(tx, g.ID)
	g.Phase = "purchase_decision"
	g.Deadline = 1000
	if err = saveEstate(tx, &g); err != nil {
		t.Fatal(err)
	}
	tx.Commit()
	id := g.ID
	version := g.Version
	f.app.store.Close()
	f.app.store, err = openStore(filepath.Join(f.app.cfg.DataDir, "linchat.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err = f.app.expireEstate(id, 1001); err != nil {
		t.Fatal(err)
	}
	if err = f.app.expireEstate(id, 1001); err != nil {
		t.Fatal(err)
	}
	tx, _ = f.app.store.Begin()
	g, _ = loadEstate(tx, id)
	tx.Rollback()
	if g.Phase != "end_turn" || g.Version != version+1 {
		t.Fatalf("timeout applied twice %+v", g)
	}
}
func TestEstateCreationRetryAndLobbyTransfer(t *testing.T) {
	f := testApp(t)
	member := f.member(t, "bob")
	op := map[string]any{"client_id": randomToken(18), "title": "幂等开房", "capacity": 2}
	var a, b estateTestView
	f.request(t, f.admin, "POST", "/api/v1/estate/rooms", op, 200, &a)
	f.request(t, f.admin, "POST", "/api/v1/estate/rooms", op, 200, &b)
	if a.Game.ID != b.Game.ID {
		t.Fatal("created twice")
	}
	base := fmt.Sprintf("/api/v1/estate/rooms/%d/", a.Game.ID)
	f.request(t, member, "POST", base+"join", map[string]any{"client_id": randomToken(18), "version": a.Game.Version}, 200, &a)
	f.request(t, f.admin, "POST", base+"leave", map[string]any{"client_id": randomToken(18), "version": a.Game.Version}, 200, &a)
	if a.Game.OwnerID != 2 || len(a.Game.Players) != 1 {
		t.Fatal("host transfer")
	}
	var doc string
	f.app.store.QueryRow("SELECT document FROM estate_games WHERE id=?", a.Game.ID).Scan(&doc)
	var stored estateGame
	if json.Unmarshal([]byte(doc), &stored) != nil {
		t.Fatal("invalid store")
	}
	if stored.OwnerID != 2 {
		t.Fatal("not persisted")
	}
}
