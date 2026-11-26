package main

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"sort"
)

// estate-v1 is independent of campaign characters and AI hosting. Money is
// integer-valued; only the server applies these rules.
type estateTile struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	Group   string `json:"group"`
	Price   int    `json:"price"`
	Upgrade int    `json:"upgrade"`
	Rents   [3]int `json:"rents"`
}

func estateMap() []estateTile {
	names := []string{"起点", "晨光巷", "机会", "钟楼街", "城市税", "河畔市场", "休息广场", "星桥路", "北站", "银杏大道", "机会", "百货广场", "检查中心", "创客街", "公共维护费", "数码港", "南站", "未来园", "临时检查", "海风大道", "机会", "灯塔湾", "市集", "云顶酒店"}
	tiles := make([]estateTile, 24)
	for i, n := range names {
		tiles[i] = estateTile{ID: i, Name: n, Kind: "rest"}
	}
	for g, ids := range [][]int{{1, 3, 5}, {7, 9, 11}, {13, 15, 17}, {19, 21, 23}} {
		for _, id := range ids {
			price := (g + 2) * 100
			tiles[id] = estateTile{ID: id, Name: names[id], Kind: "property", Group: string(rune('A' + g)), Price: price, Upgrade: price / 2, Rents: [3]int{price * 15 / 100, price * 45 / 100, price * 90 / 100}}
		}
	}
	for _, id := range []int{8, 16} {
		tiles[id].Kind = "station"
		tiles[id].Price = 300
	}
	for _, id := range []int{2, 10, 20} {
		tiles[id].Kind = "chance"
	}
	tiles[0].Kind = "start"
	tiles[4].Kind = "tax"
	tiles[14].Kind = "tax"
	tiles[12].Kind = "check"
	tiles[18].Kind = "go_check"
	tiles[22].Kind = "market"
	return tiles
}

type estatePlayer struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Token      int    `json:"token"`
	Ready      bool   `json:"ready"`
	Cash       int    `json:"cash"`
	Position   int    `json:"position"`
	Bankrupt   bool   `json:"bankrupt"`
	Checked    bool   `json:"checked"`
	Coupon     bool   `json:"coupon"`
	RentEarned int    `json:"rent_earned"`
	MaxPayment int    `json:"max_payment"`
	Turns      int    `json:"turns"`
}
type estateProperty struct {
	Owner     int64 `json:"owner"`
	Level     int   `json:"level"`
	Mortgaged bool  `json:"mortgaged"`
}

func (p *estatePlayer) spend(amount int) {
	p.Cash -= amount
	p.MaxPayment = max(p.MaxPayment, amount)
}

type estateEvent struct {
	Seq  int    `json:"seq"`
	At   int64  `json:"at"`
	Text string `json:"text"`
}
type estateGame struct {
	ID             int64            `json:"id"`
	ConversationID int64            `json:"conversation_id"`
	OwnerID        int64            `json:"owner_id"`
	Title          string           `json:"title"`
	Capacity       int              `json:"capacity"`
	Ruleset        string           `json:"ruleset"`
	Version        int64            `json:"version"`
	Phase          string           `json:"phase"`
	Round          int              `json:"round"`
	Current        int              `json:"current"`
	Deadline       int64            `json:"deadline"`
	Players        []estatePlayer   `json:"players"`
	Properties     []estateProperty `json:"properties"`
	Dice           [2]int           `json:"dice"`
	LastCard       string           `json:"last_card"`
	Debt           int              `json:"debt"`
	Creditor       int64            `json:"creditor"`
	Upgraded       bool             `json:"upgraded"`
	Events         []estateEvent    `json:"events"`
	// Stored in SQLite only. Public serialization uses an explicit sanitized copy.
	Deck       []int `json:"deck,omitempty"`
	DeckCursor int   `json:"deck_cursor,omitempty"`
}

func estateRandom(n int) int {
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		panic(err)
	}
	return int(v.Int64())
}
func estateShuffle(v []int) {
	for i := len(v) - 1; i > 0; i-- {
		j := estateRandom(i + 1)
		v[i], v[j] = v[j], v[i]
	}
}
func (g *estateGame) player(uid int64) *estatePlayer {
	for i := range g.Players {
		if g.Players[i].ID == uid {
			return &g.Players[i]
		}
	}
	return nil
}
func (g *estateGame) log(now int64, format string, args ...any) {
	g.Events = append(g.Events, estateEvent{Seq: len(g.Events) + 1, At: now, Text: fmt.Sprintf(format, args...)})
}
func (g *estateGame) phase(phase string, now int64, seconds int64) {
	g.Phase = phase
	g.Deadline = 0
	if seconds > 0 {
		g.Deadline = now + seconds*1000
	}
}
func (g *estateGame) start(now int64) error {
	if g.Phase != "lobby" || len(g.Players) < 2 {
		return gameFail(409, "至少两名玩家准备后才能开始")
	}
	for _, p := range g.Players {
		if !p.Ready {
			return gameFail(409, "等待所有玩家准备")
		}
	}
	order := make([]int, len(g.Players))
	for i := range order {
		order[i] = i
	}
	estateShuffle(order)
	old := append([]estatePlayer(nil), g.Players...)
	for i, j := range order {
		g.Players[i] = old[j]
	}
	g.Round = 1
	g.Current = 0
	g.Deck = make([]int, 12)
	for i := range g.Deck {
		g.Deck[i] = i
	}
	estateShuffle(g.Deck)
	g.log(now, "对局开始：12轮，起始现金2000。服务器已随机确定行动顺序。")
	g.begin(now)
	return nil
}
func (g *estateGame) begin(now int64) {
	g.Upgraded = false
	g.Debt = 0
	g.Creditor = 0
	g.LastCard = ""
	p := &g.Players[g.Current]
	p.Turns++
	if p.Checked {
		g.phase("check_decision", now, 45)
	} else {
		g.phase("management", now, 45)
	}
	g.log(now, "第%d轮，轮到%s。", g.Round, p.Name)
}
func (g *estateGame) end(now int64) {
	alive := 0
	for _, p := range g.Players {
		if !p.Bankrupt {
			alive++
		}
	}
	if alive <= 1 {
		g.finish(now)
		return
	}
	g.phase("end_turn", now, 2)
}
func (g *estateGame) finish(now int64) {
	g.phase("finished", now, 0)
	g.log(now, "本局结束，按净资产、现金和未抵押地产数结算。")
}
func (g *estateGame) next(now int64) {
	for step := 0; step < len(g.Players); step++ {
		g.Current++
		if g.Current >= len(g.Players) {
			g.Current = 0
			g.Round++
			if g.Round > 12 {
				g.Round = 12
				g.finish(now)
				return
			}
		}
		if !g.Players[g.Current].Bankrupt {
			g.begin(now)
			return
		}
	}
	g.finish(now)
}
func (g *estateGame) rent(tile int) int {
	pr := g.Properties[tile]
	if pr.Owner == 0 || pr.Mortgaged {
		return 0
	}
	t := estateMap()[tile]
	if t.Kind == "station" {
		n := 0
		for _, i := range []int{8, 16} {
			if g.Properties[i].Owner == pr.Owner && !g.Properties[i].Mortgaged {
				n++
			}
		}
		return n * 50
	}
	rent := t.Rents[pr.Level-1]
	full := true
	for _, other := range estateMap() {
		if other.Group == t.Group && other.Kind == "property" {
			p := g.Properties[other.ID]
			if p.Owner != pr.Owner || p.Mortgaged {
				full = false
			}
		}
	}
	if full {
		rent = rent * 125 / 100
	}
	return rent
}
func (g *estateGame) pay(amount int, creditor int64, now int64) {
	p := &g.Players[g.Current]
	if p.Cash < amount {
		g.Debt = amount
		g.Creditor = creditor
		g.phase("debt", now, 60)
		g.log(now, "%s需支付%d，现金%d，进入偿债阶段。", p.Name, amount, p.Cash)
		return
	}
	p.spend(amount)
	if receiver := g.player(creditor); receiver != nil {
		receiver.Cash += amount
		receiver.RentEarned += amount
	}
	g.log(now, "%s支付%d。", p.Name, amount)
	g.Debt = 0
	g.Creditor = 0
	g.end(now)
}
func (g *estateGame) land(now int64) {
	p := &g.Players[g.Current]
	tile := estateMap()[p.Position]
	g.log(now, "%s抵达%s。", p.Name, tile.Name)
	switch tile.Kind {
	case "property", "station":
		pr := g.Properties[tile.ID]
		if pr.Owner == 0 {
			g.phase("purchase_decision", now, 20)
			return
		}
		if pr.Owner != p.ID && !pr.Mortgaged {
			if p.Coupon {
				p.Coupon = false
				g.log(now, "%s自动使用免租券，免除租金%d。", p.Name, g.rent(tile.ID))
				g.end(now)
				return
			}
			g.log(now, "应向%s支付%s租金%d。", g.player(pr.Owner).Name, tile.Name, g.rent(tile.ID))
			g.pay(g.rent(tile.ID), pr.Owner, now)
			return
		}
	case "tax":
		amount := 100
		if tile.ID == 14 {
			amount = 150
		}
		g.pay(amount, 0, now)
		return
	case "chance":
		g.draw(now)
		return
	case "go_check":
		p.Position = 12
		p.Checked = true
		g.log(now, "%s被送至检查中心，下回合缴费或停业。", p.Name)
	case "market":
		if !p.Coupon {
			g.phase("market_decision", now, 20)
			return
		}
	}
	g.end(now)
}

var estateCards = []string{"街区节庆：获得150", "商业奖励：获得200", "交通补贴：获得80", "社区返利：获得100", "市政维修：支付100", "临时账单：支付150", "设施维护：每个已建等级支付40", "市场回暖：每块未抵押普通地产获得30", "商务出行：前进至最近枢纽，不领起点奖励", "城市漫步：回到休息广场，不领起点奖励", "临时检查：前往检查中心", "租金保障：获得免租券，已有则获80"}

func (g *estateGame) draw(now int64) {
	if g.DeckCursor >= len(g.Deck) {
		estateShuffle(g.Deck)
		g.DeckCursor = 0
	}
	card := g.Deck[g.DeckCursor]
	g.DeckCursor++
	g.LastCard = estateCards[card]
	g.log(now, "机会卡 · %s", g.LastCard)
	p := &g.Players[g.Current]
	switch card {
	case 0, 1, 2, 3:
		p.Cash += []int{150, 200, 80, 100}[card]
	case 4, 5:
		g.pay([]int{100, 150}[card-4], 0, now)
		return
	case 6:
		amount := 0
		for _, pr := range g.Properties {
			if pr.Owner == p.ID {
				amount += max(0, pr.Level-1) * 40
			}
		}
		g.pay(amount, 0, now)
		return
	case 7:
		for i, pr := range g.Properties {
			if pr.Owner == p.ID && !pr.Mortgaged && estateMap()[i].Kind == "property" {
				p.Cash += 30
			}
		}
	case 8:
		if p.Position < 8 || p.Position >= 16 {
			p.Position = 8
		} else {
			p.Position = 16
		}
		g.land(now)
		return
	case 9:
		p.Position = 6
	case 10:
		p.Position = 12
		p.Checked = true
	case 11:
		if p.Coupon {
			p.Cash += 80
		} else {
			p.Coupon = true
		}
	}
	g.end(now)
}
func (g *estateGame) roll(dice [2]int, now int64) {
	p := &g.Players[g.Current]
	g.Dice = dice
	next := p.Position + dice[0] + dice[1]
	if next >= 24 {
		p.Cash += 300
		g.log(now, "%s经过起点，获得300。", p.Name)
	}
	p.Position = next % 24
	g.log(now, "%s掷出%d＋%d。", p.Name, dice[0], dice[1])
	g.land(now)
}
func (g *estateGame) manage(action string, tile int, now int64) error {
	if tile < 0 || tile >= 24 {
		return gameFail(400, "地产编号无效")
	}
	p := &g.Players[g.Current]
	pr := &g.Properties[tile]
	t := estateMap()[tile]
	if pr.Owner != p.ID {
		return gameFail(403, "只能管理自己的地产")
	}
	switch action {
	case "upgrade":
		if g.Phase != "management" || g.Upgraded || t.Kind != "property" || pr.Mortgaged || pr.Level >= 3 || p.Cash < t.Upgrade {
			return gameFail(409, "本回合已升级，或现金、等级、抵押状态不满足")
		}
		p.spend(t.Upgrade)
		pr.Level++
		g.Upgraded = true
		g.log(now, "%s支付%d，将%s升级至%d级。", p.Name, t.Upgrade, t.Name, pr.Level)
	case "sell":
		if pr.Level <= 1 || pr.Mortgaged {
			return gameFail(409, "没有可拆建筑")
		}
		pr.Level--
		p.Cash += t.Upgrade / 2
		g.log(now, "%s拆除%s一级建筑，收回%d。", p.Name, t.Name, t.Upgrade/2)
	case "mortgage":
		if pr.Level != 1 || pr.Mortgaged {
			return gameFail(409, "请先拆除建筑，且不能重复抵押")
		}
		pr.Mortgaged = true
		p.Cash += t.Price / 2
		g.log(now, "%s抵押%s，获得%d。", p.Name, t.Name, t.Price/2)
	case "redeem":
		cost := estateRedeemCost(t.Price)
		if g.Phase != "management" || !pr.Mortgaged || p.Cash < cost {
			return gameFail(409, "当前不能赎回或现金不足")
		}
		pr.Mortgaged = false
		p.spend(cost)
		g.log(now, "%s赎回%s，支付%d。", p.Name, t.Name, cost)
	default:
		return gameFail(400, "未知资产操作")
	}
	if g.Phase == "debt" && p.Cash >= g.Debt {
		g.pay(g.Debt, g.Creditor, now)
	}
	return nil
}
func (g *estateGame) liquidate(now int64) {
	p := &g.Players[g.Current]
	for g.Phase == "debt" {
		best := -1
		for i, pr := range g.Properties {
			if pr.Owner == p.ID && pr.Level > 1 && (best < 0 || pr.Level > g.Properties[best].Level) {
				best = i
			}
		}
		if best >= 0 {
			_ = g.manage("sell", best, now)
			continue
		}
		for i, pr := range g.Properties {
			if pr.Owner == p.ID && !pr.Mortgaged && (best < 0 || estateMap()[i].Price < estateMap()[best].Price) {
				best = i
			}
		}
		if best >= 0 {
			_ = g.manage("mortgage", best, now)
			continue
		}
		if receiver := g.player(g.Creditor); receiver != nil {
			receiver.Cash += p.Cash
			receiver.RentEarned += p.Cash
		}
		if p.Cash > p.MaxPayment {
			p.MaxPayment = p.Cash
		}
		p.Cash = 0
		p.Bankrupt = true
		p.Coupon = false
		for i, pr := range g.Properties {
			if pr.Owner == p.ID {
				g.Properties[i] = estateProperty{}
			}
		}
		g.log(now, "%s破产，土地回收至银行，剩余债务注销。", p.Name)
		g.Debt = 0
		g.Creditor = 0
		g.end(now)
	}
}
func (g *estateGame) action(uid int64, action string, tile int, now int64) error {
	if g.Phase == "lobby" || g.Phase == "finished" || g.Phase == "closed" {
		return gameFail(409, "当前不在对局中")
	}
	if g.Players[g.Current].ID != uid {
		return gameFail(403, "尚未轮到你")
	}
	switch action {
	case "roll":
		if g.Phase != "management" {
			return gameFail(409, "当前不能掷骰")
		}
		g.roll([2]int{estateRandom(6) + 1, estateRandom(6) + 1}, now)
	case "buy", "decline":
		if g.Phase != "purchase_decision" {
			return gameFail(409, "当前没有待购买地产")
		}
		if action == "buy" {
			p := &g.Players[g.Current]
			t := estateMap()[p.Position]
			if p.Cash < t.Price {
				return gameFail(409, "现金不足")
			}
			if g.Properties[t.ID].Owner != 0 {
				return gameFail(409, "地产已被购买")
			}
			p.spend(t.Price)
			g.Properties[t.ID] = estateProperty{Owner: p.ID, Level: 1}
			g.log(now, "%s支付%d购买%s。", p.Name, t.Price, t.Name)
		} else {
			g.log(now, "%s放弃购买。", g.Players[g.Current].Name)
		}
		g.end(now)
	case "market_buy", "market_leave":
		if g.Phase != "market_decision" {
			return gameFail(409, "当前不在市集")
		}
		p := &g.Players[g.Current]
		if action == "market_buy" {
			if p.Cash < 100 || p.Coupon {
				return gameFail(409, "现金不足或已持有免租券")
			}
			p.spend(100)
			p.Coupon = true
			g.log(now, "%s支付100购买免租券。", p.Name)
		}
		g.end(now)
	case "check_release", "check_wait":
		if g.Phase != "check_decision" {
			return gameFail(409, "当前无需处理检查")
		}
		p := &g.Players[g.Current]
		if action == "check_release" {
			if p.Cash < 100 {
				return gameFail(409, "现金不足")
			}
			p.spend(100)
			p.Checked = false
			g.log(now, "%s支付100解除检查。", p.Name)
			g.Phase = "management"
		} else {
			p.Checked = false
			g.log(now, "%s停业一回合，检查解除。", p.Name)
			g.end(now)
		}
	case "upgrade", "sell", "mortgage", "redeem":
		if g.Phase != "management" && g.Phase != "debt" {
			return gameFail(409, "当前不能管理资产")
		}
		return g.manage(action, tile, now)
	default:
		return gameFail(400, "未知操作")
	}
	return nil
}
func (g *estateGame) expire(now int64) bool {
	if g.Deadline == 0 || now < g.Deadline {
		return false
	}
	g.log(now, "操作时间到，执行默认处理。")
	switch g.Phase {
	case "management":
		g.roll([2]int{estateRandom(6) + 1, estateRandom(6) + 1}, now)
	case "purchase_decision", "market_decision":
		g.end(now)
	case "check_decision":
		g.Players[g.Current].Checked = false
		g.end(now)
	case "debt":
		g.liquidate(now)
	case "end_turn":
		g.next(now)
	default:
		return false
	}
	return true
}

type estateScore struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Cash       int    `json:"cash"`
	Land       int    `json:"land"`
	Buildings  int    `json:"buildings"`
	Equity     int    `json:"equity"`
	Total      int    `json:"total"`
	Properties int    `json:"properties"`
	Bankrupt   bool   `json:"bankrupt"`
	Turns      int    `json:"turns"`
	Rank       int    `json:"rank"`
}

type estateRentChange struct {
	Tile   int `json:"tile"`
	Before int `json:"before"`
	After  int `json:"after"`
}
type estateQuote struct {
	RedeemCost     int                `json:"redeem_cost"`
	SellIncome     int                `json:"sell_income"`
	MortgageIncome int                `json:"mortgage_income"`
	SellRents      []estateRentChange `json:"sell_rents"`
	MortgageRents  []estateRentChange `json:"mortgage_rents"`
}

func estateRedeemCost(price int) int { return (price/2*110 + 99) / 100 }

// Quotes use the same integer arithmetic and rent rules as committed actions.
// Mortgaging a tile can lower rent on its siblings too, so return every change.
func (g *estateGame) quotes() []estateQuote {
	quotes := make([]estateQuote, 24)
	for i, t := range estateMap() {
		q := estateQuote{RedeemCost: estateRedeemCost(t.Price), MortgageIncome: t.Price / 2, SellIncome: t.Upgrade / 2, SellRents: []estateRentChange{}, MortgageRents: []estateRentChange{}}
		pr := g.Properties[i]
		if pr.Owner != 0 && !pr.Mortgaged {
			copy := *g
			copy.Properties = append([]estateProperty(nil), g.Properties...)
			if pr.Level > 1 {
				copy.Properties[i].Level--
				for j := range g.Properties {
					before, after := g.rent(j), copy.rent(j)
					if before != after {
						q.SellRents = append(q.SellRents, estateRentChange{j, before, after})
					}
				}
			} else {
				copy.Properties[i].Mortgaged = true
				for j := range g.Properties {
					before, after := g.rent(j), copy.rent(j)
					if before != after {
						q.MortgageRents = append(q.MortgageRents, estateRentChange{j, before, after})
					}
				}
			}
		}
		quotes[i] = q
	}
	return quotes
}

func (g *estateGame) scores() []estateScore {
	scores := []estateScore{}
	for _, p := range g.Players {
		s := estateScore{ID: p.ID, Name: p.Name, Cash: p.Cash, Bankrupt: p.Bankrupt, Turns: p.Turns}
		for i, pr := range g.Properties {
			if pr.Owner == p.ID {
				t := estateMap()[i]
				if pr.Mortgaged {
					s.Equity += max(0, t.Price-estateRedeemCost(t.Price))
				} else {
					s.Land += t.Price
					s.Properties++
					s.Buildings += max(0, pr.Level-1) * (t.Upgrade / 2)
				}
			}
		}
		s.Total = s.Cash + s.Land + s.Buildings + s.Equity
		scores = append(scores, s)
	}
	sort.SliceStable(scores, func(i, j int) bool {
		a, b := scores[i], scores[j]
		if a.Bankrupt != b.Bankrupt {
			return !a.Bankrupt
		}
		if a.Bankrupt {
			return a.Turns > b.Turns
		}
		if a.Total != b.Total {
			return a.Total > b.Total
		}
		if a.Cash != b.Cash {
			return a.Cash > b.Cash
		}
		return a.Properties > b.Properties
	})
	for i := range scores {
		scores[i].Rank = i + 1
		if i > 0 {
			a, b := scores[i-1], scores[i]
			if a.Bankrupt == b.Bankrupt && ((a.Bankrupt && a.Turns == b.Turns) || (!a.Bankrupt && a.Total == b.Total && a.Cash == b.Cash && a.Properties == b.Properties)) {
				scores[i].Rank = a.Rank
			}
		}
	}
	return scores
}
