package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type gameError struct {
	status  int
	message string
}

func (e *gameError) Error() string           { return e.message }
func gameFail(status int, text string) error { return &gameError{status, text} }
func gameJSON(v any) string                  { b, _ := json.Marshal(v); return string(b) }
func gameBody(r *http.Request, value any) error {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		return gameFail(415, "请使用 JSON 请求")
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 65537))
	if err != nil || len(body) > 65536 {
		return gameFail(400, "请求内容过大")
	}
	d := json.NewDecoder(bytes.NewReader(body))
	d.DisallowUnknownFields()
	if d.Decode(value) != nil {
		return gameFail(400, "请求内容格式无效")
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return gameFail(400, "请求必须是单个 JSON 对象")
	}
	return nil
}
func textOK(s string, min, max int) bool {
	n := utf8.RuneCountInString(strings.TrimSpace(s))
	return n >= min && n <= max
}
func pathID(r *http.Request, name string) int64 {
	id, _ := strconv.ParseInt(r.PathValue(name), 10, 64)
	return id
}

type gameActor struct {
	user
	AgentID  int64
	Secrets  bool
	actorKey string
}

func (g gameActor) key() string {
	if g.actorKey != "" {
		return g.actorKey
	}
	if g.AgentID != 0 {
		return fmt.Sprintf("agent:%d", g.AgentID)
	}
	return fmt.Sprintf("user:%d", g.ID)
}

type campaign struct {
	ID             int64  `json:"id"`
	ConversationID int64  `json:"conversation_id"`
	OwnerID        int64  `json:"owner_id"`
	BotID          int64  `json:"bot_id"`
	Title          string `json:"title"`
	Ruleset        string `json:"ruleset"`
	HouseRules     string `json:"house_rules"`
	Scene          string `json:"scene"`
	Summary        string `json:"summary"`
	Phase          string `json:"phase"`
	Mode           string `json:"mode"`
	AutoRespond    bool   `json:"auto_respond"`
	Version        int64  `json:"version"`
	ActiveTurn     int64  `json:"active_turn"`
	CreatedAt      int64  `json:"created_at"`
	UpdatedAt      int64  `json:"updated_at"`
}

const campaignSelect = `SELECT id,conversation_id,owner_id,bot_id,title,ruleset,house_rules,scene,summary,phase,mode,auto_respond,version,active_turn,created_at,updated_at FROM campaigns`

func scanCampaign(row scanner) (campaign, error) {
	var c campaign
	err := row.Scan(&c.ID, &c.ConversationID, &c.OwnerID, &c.BotID, &c.Title, &c.Ruleset, &c.HouseRules, &c.Scene, &c.Summary, &c.Phase, &c.Mode, &c.AutoRespond, &c.Version, &c.ActiveTurn, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}
func (a *app) gameActor(r *http.Request) (gameActor, error) {
	if raw := r.Header.Get("Authorization"); raw != "" {
		if !strings.HasPrefix(raw, "Bearer ") || pathID(r, "id") <= 0 {
			return gameActor{}, gameFail(401, "智能体令牌无效")
		}
		var g gameActor
		err := a.store.QueryRow(`SELECT t.id,t.secrets,c.owner_id,u.username,u.nickname FROM campaign_tokens t
			JOIN campaigns c ON c.id=t.campaign_id JOIN users u ON u.id=c.owner_id
			WHERE t.campaign_id=? AND t.token_hash=? AND t.revoked=0 AND t.expires_at>?`,
			pathID(r, "id"), tokenHash(strings.TrimPrefix(raw, "Bearer ")), time.Now().UnixMilli()).Scan(&g.AgentID, &g.Secrets, &g.ID, &g.Username, &g.Nickname)
		if err != nil {
			return g, gameFail(401, "令牌无效、已过期或不属于此战役")
		}
		return g, nil
	}
	u, _, err := a.currentUser(r)
	if err != nil {
		return gameActor{}, gameFail(401, "请先登录")
	}
	return gameActor{user: u}, nil
}
func (a *app) gameHTTP(fn func(*http.Request, gameActor) (any, error)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g, err := a.gameActor(r)
		limit, bucket := 100, "game:"
		if strings.HasSuffix(r.URL.Path, "/stream") {
			limit, bucket = 300, "game-stream:"
		}
		if err == nil && r.Method != "GET" && !a.limiter.allow(bucket+g.key(), limit, time.Minute) {
			err = gameFail(429, "操作太快，请稍后重试")
		}
		var out any
		if err == nil {
			out, err = fn(r, g)
		}
		if err != nil {
			var ge *gameError
			if errors.As(err, &ge) {
				fail(w, ge.status, ge.message)
			} else {
				log.Printf("战役操作失败：%v", err)
				fail(w, 500, "战役保存失败，请稍后重试")
			}
			return
		}
		writeJSON(w, 200, out)
	})
}
func (a *app) gameAccess(cid int64, g gameActor) (campaign, error) {
	c, err := scanCampaign(a.store.QueryRow(campaignSelect+" WHERE id=?", cid))
	if err != nil {
		return c, gameFail(404, "战役不存在")
	}
	if !a.store.isMember(c.ConversationID, g.ID) {
		return c, gameFail(403, "你不是该战役的成员")
	}
	return c, nil
}
func (g gameActor) owner(c campaign) bool   { return g.AgentID == 0 && g.ID == c.OwnerID }
func (g gameActor) host(c campaign) bool    { return g.owner(c) || g.AgentID != 0 }
func (g gameActor) secrets(c campaign) bool { return g.owner(c) || (g.AgentID != 0 && g.Secrets) }

type gameOp struct {
	ClientID string `json:"client_id"`
	Version  int64  `json:"version"`
}

// Receipts and mutations commit together; retries never reroll, double-apply or republish.
func (a *app) gameMutation(cid int64, g gameActor, op gameOp, operation string, input any, fn func(*sql.Tx, campaign) (any, error)) (any, error) {
	if !idPattern.MatchString(op.ClientID) {
		return nil, gameFail(400, "需要 16–80 位的唯一 client_id")
	}
	if _, err := a.gameAccess(cid, g); err != nil {
		return nil, err
	}
	tx, err := a.store.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	fingerprint := tokenHash(operation + ":" + gameJSON(input))
	var saved, hash string
	err = tx.QueryRow("SELECT fingerprint,response FROM campaign_operations WHERE campaign_id=? AND actor=? AND client_id=?", cid, g.key(), op.ClientID).Scan(&hash, &saved)
	if err == nil {
		if hash != fingerprint {
			return nil, gameFail(409, "此请求编号已用于其他内容")
		}
		return json.RawMessage(saved), nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	c, err := scanCampaign(tx.QueryRow(campaignSelect+" WHERE id=?", cid))
	if err != nil {
		return nil, err
	}
	var lastMessage int64
	if err = tx.QueryRow("SELECT COALESCE(MAX(id),0) FROM messages WHERE conversation_id=?", c.ConversationID).Scan(&lastMessage); err != nil {
		return nil, err
	}
	out, err := fn(tx, c)
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec("INSERT INTO campaign_operations(campaign_id,actor,client_id,fingerprint,response) VALUES(?,?,?,?,?)", cid, g.key(), op.ClientID, fingerprint, gameJSON(out)); err != nil {
		return nil, err
	}
	rows, err := tx.Query(messageSelect+" WHERE m.conversation_id=? AND m.id>? ORDER BY m.id", c.ConversationID, lastMessage)
	if err != nil {
		return nil, err
	}
	var published []message
	for rows.Next() {
		m, e := scanMessage(rows)
		if e != nil {
			rows.Close()
			return nil, e
		}
		published = append(published, m)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	for _, m := range published {
		a.publishMessage(m)
	}
	a.gameChanged(c)
	return out, nil
}
func (a *app) gameChanged(c campaign) {
	a.hub.broadcast(a.store.memberIDs(c.ConversationID), map[string]any{"type": "campaign.changed", "campaign_id": c.ID, "conversation_id": c.ConversationID})
}
func gameVersion(c campaign, version int64) error {
	if c.Version != version {
		return gameFail(409, "战役已有新进展，请刷新后再提交")
	}
	if c.ActiveTurn != 0 {
		return gameFail(409, "主持人正在处理行动，请等待完成或先暂停战役")
	}
	return nil
}
func bumpGame(tx *sql.Tx, cid int64) error {
	_, err := tx.Exec("UPDATE campaigns SET version=version+1,updated_at=? WHERE id=?", time.Now().UnixMilli(), cid)
	return err
}
func gameEvent(tx *sql.Tx, c campaign, actor int64, kind, visibility string, recipient int64, content string, payload any) (int64, error) {
	res, err := tx.Exec(`INSERT INTO campaign_events(campaign_id,actor_id,kind,visibility,recipient_id,content,payload,created_at) VALUES(?,?,?,?,?,?,?,?)`,
		c.ID, actor, kind, visibility, recipient, content, gameJSON(payload), time.Now().UnixMilli())
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	if visibility == "public" && (kind == "narration" || kind == "summary" || kind == "roll" || kind == "check") {
		_, err = tx.Exec(`INSERT INTO messages(conversation_id,sender_id,client_id,kind,content,created_at) VALUES(?,?,?,'text',?,?)`,
			c.ConversationID, c.BotID, fmt.Sprintf("campaign-event-%d", id), content, time.Now().UnixMilli())
	}
	return id, err
}

func (a *app) registerGames(m *http.ServeMux) {
	routes := map[string]func(*http.Request, gameActor) (any, error){
		"GET /api/v1/campaigns/{id}/reply-prompt":           a.gameReplyPromptHTTP,
		"GET /api/v1/campaigns/{id}/checks":                 a.gameChecks,
		"GET /api/v1/campaigns/{id}/checks/{check}":         a.gameChecks,
		"POST /api/v1/campaigns/{id}/checks":                a.gamePrepareCheck,
		"POST /api/v1/campaigns/{id}/checks/{check}/roll":   a.gameRollCheck,
		"POST /api/v1/campaigns/{id}/checks/{check}/choose": a.gameChooseCheck,
		"POST /api/v1/campaigns/{id}/checks/{check}/cancel": a.gameCancelCheck,
		"GET /api/v1/campaigns":                             a.gameList, "POST /api/v1/campaigns": a.gameCreate,
		"GET /api/v1/campaigns/{id}/state": a.gameState, "PATCH /api/v1/campaigns/{id}/state": a.gamePatchState,
		"GET /api/v1/campaigns/{id}/context": a.gameContextHTTP, "GET /api/v1/campaigns/{id}/rules": a.gameRules,
		"GET /api/v1/campaigns/{id}/characters": a.gameCharactersHTTP, "POST /api/v1/campaigns/{id}/characters": a.gameCharacterSave,
		"GET /api/v1/campaigns/{id}/dm-notes": a.gameNotes, "PUT /api/v1/campaigns/{id}/dm-notes": a.gameSaveNotes,
		"GET /api/v1/campaigns/{id}/events": a.gameEventsHTTP, "POST /api/v1/campaigns/{id}/actions": a.gameAction,
		"POST /api/v1/campaigns/{id}/rolls": a.gameRoll, "POST /api/v1/campaigns/{id}/messages": a.gameMessage,
		"GET /api/v1/campaigns/{id}/story-proposals": a.gameProposals, "POST /api/v1/campaigns/{id}/story-proposals": a.gamePropose,
		"POST /api/v1/campaigns/{id}/story-proposals/{proposal}/decide": a.gameDecide,
		"GET /api/v1/campaigns/{id}/agent-tokens":                       a.gameTokens, "POST /api/v1/campaigns/{id}/agent-tokens": a.gameTokenCreate,
		"DELETE /api/v1/campaigns/{id}/agent-tokens/{token}": a.gameTokenRevoke,
		"GET /api/v1/campaigns/{id}/turns":                   a.gameTurns, "POST /api/v1/campaigns/{id}/turns/claim": a.gameClaimHTTP,
		"POST /api/v1/campaigns/{id}/turns/{turn}/complete": a.gameCompleteHTTP,
		"POST /api/v1/campaigns/{id}/turns/{turn}/stream":   a.gameStreamHTTP,
		"GET /api/v1/campaigns/{id}/lore":                   a.gameLoreList,
		"POST /api/v1/campaigns/{id}/lore":                  a.gameLoreSave,
		"GET /api/v1/campaigns/{id}/lore/{entry}/history":   a.gameLoreHistory,
		"GET /api/v1/campaigns/{id}/stream":                 a.gameStreamRead,
		"POST /api/v1/campaigns/{id}/turns/{turn}/fail":     a.gameFailHTTP,
		"POST /api/v1/campaigns/{id}/turns/{turn}/retry":    a.gameRetry,
		"POST /api/v1/campaigns/{id}/host":                  a.gameHost,
		"GET /api/v1/entertainment/ai":                      a.gameAIGet, "PUT /api/v1/entertainment/ai": a.gameAISet,
		"POST /api/v1/entertainment/ai/test": a.gameAITest,
	}
	for path, fn := range routes {
		m.Handle(path, a.gameHTTP(fn))
	}
}
func (a *app) gameList(r *http.Request, g gameActor) (any, error) {
	rows, err := a.store.Query(campaignSelect+" WHERE conversation_id IN (SELECT conversation_id FROM members WHERE user_id=?) ORDER BY updated_at DESC", g.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []campaign{}
	for rows.Next() {
		c, e := scanCampaign(rows)
		if e != nil {
			return nil, e
		}
		items = append(items, c)
	}
	return items, rows.Err()
}
func (a *app) gameCreate(r *http.Request, g gameActor) (any, error) {
	var in struct {
		Title      string  `json:"title"`
		Ruleset    string  `json:"ruleset"`
		Members    []int64 `json:"members"`
		Scene      string  `json:"scene"`
		HouseRules string  `json:"house_rules"`
	}
	if err := gameBody(r, &in); err != nil {
		return nil, err
	}
	in.Title = strings.TrimSpace(in.Title)
	if !textOK(in.Title, 1, 40) || !textOK(in.Scene, 0, 8000) || !textOK(in.HouseRules, 0, 12000) {
		return nil, gameFail(400, "名称最多 40 字，场景最多 8000 字，规则资料最多 12000 字")
	}
	if in.Ruleset != "5e-2014" && in.Ruleset != "5e-2024" && in.Ruleset != "custom" {
		return nil, gameFail(400, "请选择规则版本")
	}
	if !a.limiter.allow("campaign-create:"+g.key(), 10, time.Hour) {
		return nil, gameFail(429, "创建过于频繁")
	}
	ids := map[int64]bool{g.ID: true}
	for _, id := range in.Members {
		if id > 0 {
			ids[id] = true
		}
	}
	if len(ids) > 99 {
		return nil, gameFail(400, "一个战役最多 99 位真人成员")
	}
	tx, err := a.store.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	now := time.Now().UnixMilli()
	res, err := tx.Exec("INSERT INTO users(username,nickname,password,created_at) VALUES(?,'主持人','!service-account',?)", "dm_"+randomToken(12), now)
	if err != nil {
		return nil, err
	}
	botID, _ := res.LastInsertId()
	if _, err = tx.Exec("INSERT INTO service_accounts(user_id) VALUES(?)", botID); err != nil {
		return nil, err
	}
	res, err = tx.Exec("INSERT INTO conversations(kind,name,owner_id,created_at) VALUES('group',?,?,?)", in.Title, g.ID, now)
	if err != nil {
		return nil, err
	}
	roomID, _ := res.LastInsertId()
	for id := range ids {
		var n int
		if tx.QueryRow("SELECT COUNT(*) FROM users WHERE id=? AND id NOT IN (SELECT user_id FROM service_accounts)", id).Scan(&n) != nil || n != 1 {
			return nil, gameFail(400, "所选成员不存在")
		}
		if _, err = tx.Exec("INSERT INTO members(conversation_id,user_id) VALUES(?,?)", roomID, id); err != nil {
			return nil, err
		}
	}
	if _, err = tx.Exec("INSERT INTO members(conversation_id,user_id) VALUES(?,?)", roomID, botID); err != nil {
		return nil, err
	}
	res, err = tx.Exec(`INSERT INTO campaigns(conversation_id,owner_id,bot_id,title,ruleset,scene,house_rules,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?)`, roomID, g.ID, botID, in.Title, in.Ruleset, in.Scene, in.HouseRules, now, now)
	if err != nil {
		return nil, err
	}
	cid, _ := res.LastInsertId()
	if _, err = tx.Exec("INSERT INTO campaign_notes(campaign_id) VALUES(?)", cid); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	a.hub.broadcast(a.store.memberIDs(roomID), map[string]any{"type": "conversations.changed"})
	c, err := a.gameAccess(cid, g)
	if err == nil {
		a.gameChanged(c)
	}
	return c, err
}
func (a *app) gameState(r *http.Request, g gameActor) (any, error) {
	return a.gameAccess(pathID(r, "id"), g)
}
func (a *app) gameRules(r *http.Request, g gameActor) (any, error) {
	c, err := a.gameAccess(pathID(r, "id"), g)
	if err != nil {
		return nil, err
	}
	return map[string]any{"ruleset": c.Ruleset, "house_rules": c.HouseRules, "guidance": "采用创建时固定的规则版本。只使用房主提供的规则资料；不确定时说明并请房主裁定。玩家决定自己的行动，所有随机结果由服务器掷骰产生。"}, nil
}
func (a *app) gamePatchState(r *http.Request, g gameActor) (any, error) {
	var in struct {
		gameOp
		Scene       *string `json:"scene"`
		Phase       *string `json:"phase"`
		Mode        *string `json:"mode"`
		AutoRespond *bool   `json:"auto_respond"`
	}
	if err := gameBody(r, &in); err != nil {
		return nil, err
	}
	out, err := a.gameMutation(pathID(r, "id"), g, in.gameOp, "state", in, func(tx *sql.Tx, c campaign) (any, error) {
		if !g.host(c) {
			return nil, gameFail(403, "只有房主或获准主持智能体能修改战役")
		}
		pause := in.Phase != nil && *in.Phase == "paused" && g.owner(c)
		if c.Version != in.Version {
			return nil, gameFail(409, "战役已有新进展，请刷新后再提交")
		}
		if c.ActiveTurn != 0 && !pause {
			return nil, gameFail(409, "请等待主持完成或先暂停")
		}
		if in.Scene != nil {
			if !textOK(*in.Scene, 0, 8000) {
				return nil, gameFail(400, "场景最多 8000 字")
			}
			c.Scene = *in.Scene
		}
		if in.Phase != nil {
			if *in.Phase != "awaiting_action" && *in.Phase != "awaiting_roll" && *in.Phase != "paused" {
				return nil, gameFail(400, "战役阶段无效")
			}
			c.Phase = *in.Phase
		}
		if in.Mode != nil {
			if !g.owner(c) {
				return nil, gameFail(403, "只有房主能选择主持方式")
			}
			if *in.Mode != "builtin" && *in.Mode != "external" && *in.Mode != "manual" {
				return nil, gameFail(400, "主持方式无效")
			}
			c.Mode = *in.Mode
		}
		if in.AutoRespond != nil {
			if !g.owner(c) {
				return nil, gameFail(403, "只有房主能配置自动主持")
			}
			c.AutoRespond = *in.AutoRespond
		}
		if pause {
			if _, err := tx.Exec("UPDATE campaign_turns SET status='failed',error='房主已暂停，请确认后重试',lease='' WHERE campaign_id=? AND status='running'", c.ID); err != nil {
				return nil, err
			}
			c.ActiveTurn = 0
		}
		_, err := tx.Exec("UPDATE campaigns SET scene=?,phase=?,mode=?,auto_respond=?,active_turn=?,version=version+1,updated_at=? WHERE id=?", c.Scene, c.Phase, c.Mode, c.AutoRespond, c.ActiveTurn, time.Now().UnixMilli(), c.ID)
		if err != nil {
			return nil, err
		}
		c.Version++
		return c, nil
	})
	if err == nil {
		a.kickGame(pathID(r, "id"))
	}
	return out, err
}
