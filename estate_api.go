package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func migrateEstate(db *sql.DB) error {
	_, err := db.Exec(`
	CREATE TABLE IF NOT EXISTS estate_games (
	  id INTEGER PRIMARY KEY AUTOINCREMENT,
	  conversation_id INTEGER NOT NULL REFERENCES conversations(id),
	  document TEXT NOT NULL, deadline INTEGER NOT NULL DEFAULT 0
	);
	CREATE INDEX IF NOT EXISTS estate_deadline ON estate_games(deadline);
	CREATE TABLE IF NOT EXISTS estate_operations (
	  user_id INTEGER NOT NULL REFERENCES users(id), client_id TEXT NOT NULL,
	  fingerprint TEXT NOT NULL, response TEXT NOT NULL,
	  PRIMARY KEY(user_id,client_id)
	);`)
	return err
}
func loadEstate(tx *sql.Tx, id int64) (estateGame, error) {
	var g estateGame
	var raw string
	if err := tx.QueryRow("SELECT document FROM estate_games WHERE id=?", id).Scan(&raw); err != nil {
		if err == sql.ErrNoRows {
			return g, gameFail(404, "对局不存在")
		}
		return g, err
	}
	err := json.Unmarshal([]byte(raw), &g)
	return g, err
}
func saveEstate(tx *sql.Tx, g *estateGame) error {
	_, err := tx.Exec("UPDATE estate_games SET document=?,deadline=? WHERE id=?", gameJSON(g), g.Deadline, g.ID)
	return err
}
func estateView(g estateGame) any {
	g.Deck = nil
	g.DeckCursor = 0
	rents := make([]int, 24)
	for i := range rents {
		rents[i] = g.rent(i)
	}
	return map[string]any{"game": g, "tiles": estateMap(), "rents": rents, "quotes": g.quotes(), "scores": g.scores(), "server_time": time.Now().UnixMilli()}
}
func estateFailure(w http.ResponseWriter, err error) {
	var e *gameError
	if errors.As(err, &e) {
		fail(w, e.status, e.message)
	} else {
		log.Printf("地产对局失败：%v", err)
		fail(w, 500, "对局保存失败，请重试")
	}
}
func (a *app) estateNotify(g estateGame) {
	ids := make([]int64, 0, len(g.Players))
	for _, p := range g.Players {
		ids = append(ids, p.ID)
	}
	a.hub.broadcast(ids, map[string]any{"type": "estate.changed", "room_id": g.ID, "version": g.Version})
}
func (a *app) registerEstate(m *http.ServeMux) {
	m.Handle("GET /api/v1/estate/rooms", a.auth(a.estateList))
	m.Handle("POST /api/v1/estate/rooms", a.auth(a.estateWrite))
	for _, action := range []string{"join", "ready", "start", "leave", "token"} {
		m.Handle("POST /api/v1/estate/rooms/{id}/"+action, a.auth(a.estateWrite))
	}
	m.Handle("GET /api/v1/estate/matches/{id}", a.auth(a.estateRead))
	m.Handle("GET /api/v1/estate/matches/{id}/events", a.auth(a.estateRead))
	m.Handle("POST /api/v1/estate/matches/{id}/actions", a.auth(a.estateWrite))
}
func (a *app) estateList(w http.ResponseWriter, r *http.Request, u user) {
	rows, err := a.store.Query("SELECT document FROM estate_games ORDER BY id DESC LIMIT 200")
	if err != nil {
		estateFailure(w, err)
		return
	}
	defer rows.Close()
	out := []any{}
	for rows.Next() {
		var raw string
		var g estateGame
		if err = rows.Scan(&raw); err != nil {
			break
		}
		if err = json.Unmarshal([]byte(raw), &g); err != nil {
			break
		}
		if g.Phase == "closed" || (g.Phase != "lobby" && g.player(u.ID) == nil) {
			continue
		}
		out = append(out, map[string]any{"id": g.ID, "title": g.Title, "capacity": g.Capacity, "count": len(g.Players), "phase": g.Phase, "joined": g.player(u.ID) != nil, "version": g.Version})
	}
	if err == nil {
		err = rows.Err()
	}
	if err != nil {
		estateFailure(w, err)
		return
	}
	writeJSON(w, 200, out)
}
func (a *app) estateRead(w http.ResponseWriter, r *http.Request, u user) {
	tx, err := a.store.BeginTx(r.Context(), nil)
	if err != nil {
		estateFailure(w, err)
		return
	}
	defer tx.Rollback()
	g, err := loadEstate(tx, pathID(r, "id"))
	if err == nil && g.player(u.ID) == nil {
		err = gameFail(403, "请先加入此房间")
	}
	if err != nil {
		estateFailure(w, err)
		return
	}
	if strings.HasSuffix(r.URL.Path, "/events") {
		seq, _ := strconv.Atoi(r.URL.Query().Get("after_seq"))
		if seq < 0 {
			seq = 0
		}
		if seq > len(g.Events) {
			seq = len(g.Events)
		}
		writeJSON(w, 200, g.Events[seq:])
		return
	}
	writeJSON(w, 200, estateView(g))
}

type estateRequest struct {
	gameOp
	Title    string `json:"title,omitempty"`
	Capacity int    `json:"capacity,omitempty"`
	Ready    bool   `json:"ready,omitempty"`
	Token    int    `json:"token,omitempty"`
	Action   string `json:"action,omitempty"`
	Tile     int    `json:"tile,omitempty"`
}

func (a *app) estateWrite(w http.ResponseWriter, r *http.Request, u user) {
	if !a.limiter.allow(fmt.Sprintf("estate:%d", u.ID), 120, time.Minute) {
		fail(w, 429, "操作太快，请稍后重试")
		return
	}
	var in estateRequest
	if err := gameBody(r, &in); err != nil {
		estateFailure(w, err)
		return
	}
	if !idPattern.MatchString(in.ClientID) {
		fail(w, 400, "需要唯一的16–80位client_id")
		return
	}
	g, response, err := a.estateMutate(r, u, in)
	if err != nil {
		estateFailure(w, err)
		return
	}
	if g != nil {
		a.estateNotify(*g)
	}
	writeJSON(w, 200, response)
}
func (a *app) estateMutate(r *http.Request, u user, in estateRequest) (*estateGame, any, error) {
	tx, err := a.store.BeginTx(r.Context(), nil)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback()
	fingerprint := r.URL.Path + "\n" + gameJSON(in)
	var old, receipt string
	err = tx.QueryRow("SELECT fingerprint,response FROM estate_operations WHERE user_id=? AND client_id=?", u.ID, in.ClientID).Scan(&old, &receipt)
	if err == nil {
		if old != fingerprint {
			return nil, nil, gameFail(409, "请求编号已用于其他内容")
		}
		return nil, json.RawMessage(receipt), nil
	}
	if err != sql.ErrNoRows {
		return nil, nil, err
	}
	now := time.Now().UnixMilli()
	id := pathID(r, "id")
	var g estateGame
	route := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
	if id == 0 && route == "rooms" {
		in.Title = strings.TrimSpace(in.Title)
		if !textOK(in.Title, 1, 40) || in.Capacity < 2 || in.Capacity > 4 {
			return nil, nil, gameFail(400, "房间名1–40字，人数2–4人")
		}
		var count int
		if err = tx.QueryRow(`SELECT COUNT(*) FROM estate_games WHERE json_extract(document,'$.owner_id')=? AND json_extract(document,'$.phase') NOT IN ('finished','closed')`, u.ID).Scan(&count); err != nil {
			return nil, nil, err
		}
		if count >= 3 {
			return nil, nil, gameFail(409, "最多保留3个未结束房间，请先结束或离开已有房间")
		}
		res, err := tx.Exec("INSERT INTO conversations(kind,name,owner_id,created_at) VALUES('group',?,?,?)", "地产 · "+in.Title, u.ID, now)
		if err != nil {
			return nil, nil, err
		}
		cid, err := res.LastInsertId()
		if err != nil {
			return nil, nil, err
		}
		if _, err = tx.Exec("INSERT INTO members(conversation_id,user_id) VALUES(?,?)", cid, u.ID); err != nil {
			return nil, nil, err
		}
		res, err = tx.Exec("INSERT INTO estate_games(conversation_id,document) VALUES(?,'{}')", cid)
		if err != nil {
			return nil, nil, err
		}
		id, err = res.LastInsertId()
		if err != nil {
			return nil, nil, err
		}
		g = estateGame{ID: id, ConversationID: cid, OwnerID: u.ID, Title: in.Title, Capacity: in.Capacity, Ruleset: "estate-v1", Phase: "lobby", Properties: make([]estateProperty, 24), Players: []estatePlayer{{ID: u.ID, Name: u.Nickname, Token: 0, Cash: 2000}}, Events: []estateEvent{}}
		g.log(now, "%s创建房间，等待玩家准备。", u.Nickname)
	} else {
		g, err = loadEstate(tx, id)
		if err != nil {
			return nil, nil, err
		}
		p := g.player(u.ID)
		if route != "join" && p == nil {
			return nil, nil, gameFail(403, "你不是此对局玩家")
		}
		if g.Version != in.Version {
			return nil, nil, gameFail(409, "局面已更新，请刷新后重新确认")
		}
		if g.Deadline > 0 && now >= g.Deadline {
			return nil, nil, gameFail(409, "操作已超时，请等待自动处理")
		}
		switch route {
		case "join":
			if g.Phase != "lobby" {
				return nil, nil, gameFail(409, "对局已开始，不能补位")
			}
			if p == nil {
				if len(g.Players) >= g.Capacity {
					return nil, nil, gameFail(409, "房间已满")
				}
				token := 0
				for ; token < 4; token++ {
					used := false
					for _, other := range g.Players {
						if other.Token == token {
							used = true
						}
					}
					if !used {
						break
					}
				}
				g.Players = append(g.Players, estatePlayer{ID: u.ID, Name: u.Nickname, Token: token, Cash: 2000})
				if _, err = tx.Exec("INSERT OR IGNORE INTO members(conversation_id,user_id) VALUES(?,?)", g.ConversationID, u.ID); err != nil {
					return nil, nil, err
				}
				g.log(now, "%s加入房间。", u.Nickname)
			}
		case "ready", "token":
			if g.Phase != "lobby" {
				return nil, nil, gameFail(409, "对局已经开始")
			}
			if route == "ready" {
				p.Ready = in.Ready
			} else {
				if in.Token < 0 || in.Token > 3 {
					return nil, nil, gameFail(400, "请选择有效棋子")
				}
				for _, other := range g.Players {
					if other.ID != u.ID && other.Token == in.Token {
						return nil, nil, gameFail(409, "棋子已被选择")
					}
				}
				p.Token = in.Token
				p.Ready = false
			}
		case "leave":
			if g.Phase != "lobby" {
				return nil, nil, gameFail(409, "已开始的对局退出界面即可，离线按超时规则行动")
			}
			players := []estatePlayer{}
			for _, other := range g.Players {
				if other.ID != u.ID {
					players = append(players, other)
				}
			}
			g.Players = players
			if _, err = tx.Exec("DELETE FROM members WHERE conversation_id=? AND user_id=?", g.ConversationID, u.ID); err != nil {
				return nil, nil, err
			}
			if len(g.Players) == 0 {
				g.Phase = "closed"
			} else if g.OwnerID == u.ID {
				g.OwnerID = g.Players[0].ID
				if _, err = tx.Exec("UPDATE conversations SET owner_id=? WHERE id=?", g.OwnerID, g.ConversationID); err != nil {
					return nil, nil, err
				}
			}
			g.log(now, "%s离开准备房。", u.Nickname)
		case "start":
			if g.OwnerID != u.ID {
				return nil, nil, gameFail(403, "只有房主可以开始")
			}
			if err = g.start(now); err != nil {
				return nil, nil, err
			}
		case "actions":
			if err = g.action(u.ID, in.Action, in.Tile, now); err != nil {
				return nil, nil, err
			}
		default:
			return nil, nil, gameFail(404, "操作不存在")
		}
	}
	g.Version++
	if err = saveEstate(tx, &g); err != nil {
		return nil, nil, err
	}
	view := estateView(g)
	if _, err = tx.Exec("INSERT INTO estate_operations(user_id,client_id,fingerprint,response) VALUES(?,?,?,?)", u.ID, in.ClientID, fingerprint, gameJSON(view)); err != nil {
		return nil, nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, nil, err
	}
	return &g, view, nil
}

// Persistent deadlines survive process restarts. Each expiration is serialized
// with player operations by the database transaction, then broadcast after commit.
func (a *app) expireEstate(id, now int64) error {
	tx, err := a.store.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	g, err := loadEstate(tx, id)
	if err != nil {
		return err
	}
	if !g.expire(now) {
		return nil
	}
	g.Version++
	if err = saveEstate(tx, &g); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	a.estateNotify(g)
	return nil
}
func (a *app) resumeEstate() {
	a.gameWG.Add(1)
	go func() {
		defer a.gameWG.Done()
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-a.gameCtx.Done():
				return
			case now := <-ticker.C:
				rows, err := a.store.Query("SELECT id FROM estate_games WHERE deadline>0 AND deadline<=?", now.UnixMilli())
				if err != nil {
					log.Printf("地产期限查询失败：%v", err)
					continue
				}
				ids := []int64{}
				for rows.Next() {
					var id int64
					if rows.Scan(&id) == nil {
						ids = append(ids, id)
					}
				}
				err = rows.Err()
				rows.Close()
				if err != nil {
					log.Printf("地产期限读取失败：%v", err)
					continue
				}
				for _, id := range ids {
					if err := a.expireEstate(id, now.UnixMilli()); err != nil {
						log.Printf("地产超时处理失败：%v", err)
					}
				}
			}
		}
	}()
}
