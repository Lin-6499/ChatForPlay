package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type character struct {
	ID           int64          `json:"id"`
	CampaignID   int64          `json:"campaign_id"`
	UserID       int64          `json:"user_id"`
	Name         string         `json:"name"`
	Role         string         `json:"role"`
	Level        int            `json:"level"`
	HP           int            `json:"hp"`
	MaxHP        int            `json:"max_hp"`
	AC           int            `json:"ac"`
	Abilities    map[string]int `json:"abilities"`
	Conditions   string         `json:"conditions"`
	Inventory    string         `json:"inventory"`
	Resources    string         `json:"resources"`
	Background   string         `json:"background"`
	PrivateNotes *string        `json:"private_notes,omitempty"`
	Version      int64          `json:"version"`
}

func (a *app) gameCharacters(c campaign, g gameActor) ([]character, error) {
	privateUser := int64(0)
	if g.AgentID == 0 {
		privateUser = g.ID
	}
	rows, err := a.store.Query(`SELECT id,campaign_id,user_id,name,role,level,hp,max_hp,ac,abilities,conditions,inventory,resources,background,
		CASE WHEN ? OR user_id=? THEN private_notes ELSE NULL END,version FROM characters WHERE campaign_id=? ORDER BY id`, g.secrets(c), privateUser, c.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []character{}
	for rows.Next() {
		var ch character
		var abilities string
		var secret sql.NullString
		if err = rows.Scan(&ch.ID, &ch.CampaignID, &ch.UserID, &ch.Name, &ch.Role, &ch.Level, &ch.HP, &ch.MaxHP, &ch.AC, &abilities, &ch.Conditions, &ch.Inventory, &ch.Resources, &ch.Background, &secret, &ch.Version); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(abilities), &ch.Abilities); err != nil {
			return nil, err
		}
		if secret.Valid {
			ch.PrivateNotes = &secret.String
		}
		out = append(out, ch)
	}
	return out, rows.Err()
}
func (a *app) gameCharactersHTTP(r *http.Request, g gameActor) (any, error) {
	c, err := a.gameAccess(pathID(r, "id"), g)
	if err != nil {
		return nil, err
	}
	return a.gameCharacters(c, g)
}
func (a *app) gameCharacterSave(r *http.Request, g gameActor) (any, error) {
	var in struct {
		gameOp
		Character character `json:"character"`
	}
	if err := gameBody(r, &in); err != nil {
		return nil, err
	}
	ch := in.Character
	if !textOK(ch.Name, 1, 40) || !textOK(ch.Role, 1, 60) || ch.Level < 1 || ch.Level > 20 || ch.MaxHP < 1 || ch.MaxHP > 9999 || ch.HP < 0 || ch.HP > ch.MaxHP || ch.AC < 0 || ch.AC > 99 {
		return nil, gameFail(400, "检查角色名称、职业、等级、生命值和护甲等级")
	}
	for _, v := range []string{ch.Conditions, ch.Inventory, ch.Resources, ch.Background} {
		if !textOK(v, 0, 6000) {
			return nil, gameFail(400, "角色资料每项最多 6000 字")
		}
	}
	if ch.PrivateNotes != nil && !textOK(*ch.PrivateNotes, 0, 6000) {
		return nil, gameFail(400, "私密背景最多 6000 字")
	}
	if len(ch.Abilities) != 6 {
		return nil, gameFail(400, "需要六项属性：str、dex、con、int、wis、cha")
	}
	for _, key := range []string{"str", "dex", "con", "int", "wis", "cha"} {
		if ch.Abilities[key] < 1 || ch.Abilities[key] > 30 {
			return nil, gameFail(400, "属性值范围为 1–30")
		}
	}
	return a.gameMutation(pathID(r, "id"), g, in.gameOp, "character", in, func(tx *sql.Tx, c campaign) (any, error) {
		if g.AgentID != 0 || (!g.owner(c) && ch.UserID != g.ID) {
			return nil, gameFail(403, "只能编辑自己的角色卡")
		}
		if err := gameVersion(c, in.Version); err != nil {
			return nil, err
		}
		var n int
		if tx.QueryRow("SELECT COUNT(*) FROM members WHERE conversation_id=? AND user_id=? AND user_id<>?", c.ConversationID, ch.UserID, c.BotID).Scan(&n) != nil || n != 1 {
			return nil, gameFail(400, "角色必须属于本战役的真人成员")
		}
		secret := ""
		if ch.PrivateNotes != nil {
			secret = *ch.PrivateNotes
		}
		var oldID, oldVersion int64
		var oldSecret, oldBackground string
		err := tx.QueryRow("SELECT id,version,private_notes,background FROM characters WHERE campaign_id=? AND user_id=?", c.ID, ch.UserID).Scan(&oldID, &oldVersion, &oldSecret, &oldBackground)
		if err == nil {
			if ch.Background != oldBackground && !g.owner(c) {
				return nil, gameFail(409, "已有角色背景的修改请提交设定提案，由房主确认")
			}
			if ch.ID != oldID || ch.Version != oldVersion {
				return nil, gameFail(409, "角色卡已更新，请刷新")
			}
			if ch.PrivateNotes == nil {
				secret = oldSecret
			}
			_, err = tx.Exec(`UPDATE characters SET name=?,role=?,level=?,hp=?,max_hp=?,ac=?,abilities=?,conditions=?,inventory=?,resources=?,background=?,private_notes=?,version=version+1 WHERE id=?`,
				ch.Name, ch.Role, ch.Level, ch.HP, ch.MaxHP, ch.AC, gameJSON(ch.Abilities), ch.Conditions, ch.Inventory, ch.Resources, ch.Background, secret, oldID)
			ch.Version = oldVersion + 1
		} else if err == sql.ErrNoRows {
			if ch.ID != 0 {
				return nil, gameFail(409, "角色卡不存在")
			}
			var res sql.Result
			res, err = tx.Exec(`INSERT INTO characters(campaign_id,user_id,name,role,level,hp,max_hp,ac,abilities,conditions,inventory,resources,background,private_notes) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
				c.ID, ch.UserID, ch.Name, ch.Role, ch.Level, ch.HP, ch.MaxHP, ch.AC, gameJSON(ch.Abilities), ch.Conditions, ch.Inventory, ch.Resources, ch.Background, secret)
			if err == nil {
				ch.ID, _ = res.LastInsertId()
				ch.Version = 1
			}
		}
		if err != nil {
			return nil, err
		}
		if err = bumpGame(tx, c.ID); err != nil {
			return nil, err
		}
		ch.CampaignID = c.ID
		ch.PrivateNotes = &secret
		return ch, nil
	})
}
func (a *app) gameNotes(r *http.Request, g gameActor) (any, error) {
	c, err := a.gameAccess(pathID(r, "id"), g)
	if err != nil {
		return nil, err
	}
	if !g.secrets(c) {
		return nil, gameFail(403, "此资料仅向房主和获准读取秘密的主持智能体开放")
	}
	var content string
	var version int64
	err = a.store.QueryRow("SELECT content,version FROM campaign_notes WHERE campaign_id=?", c.ID).Scan(&content, &version)
	return map[string]any{"content": content, "version": version}, err
}
func (a *app) gameSaveNotes(r *http.Request, g gameActor) (any, error) {
	var in struct {
		gameOp
		Content     string `json:"content"`
		NoteVersion int64  `json:"note_version"`
	}
	if err := gameBody(r, &in); err != nil {
		return nil, err
	}
	return a.gameMutation(pathID(r, "id"), g, in.gameOp, "notes", in, func(tx *sql.Tx, c campaign) (any, error) {
		if !g.secrets(c) {
			return nil, gameFail(403, "没有主持笔记权限")
		}
		if !textOK(in.Content, 0, 16000) {
			return nil, gameFail(400, "主持笔记最多 16000 字")
		}
		res, err := tx.Exec("UPDATE campaign_notes SET content=?,version=version+1 WHERE campaign_id=? AND version=?", in.Content, c.ID, in.NoteVersion)
		if err != nil {
			return nil, err
		}
		n, _ := res.RowsAffected()
		if n != 1 {
			return nil, gameFail(409, "笔记已更新，请刷新")
		}
		return map[string]any{"content": in.Content, "version": in.NoteVersion + 1}, nil
	})
}

type campaignEvent struct {
	ID          int64           `json:"id"`
	ActorID     int64           `json:"actor_id"`
	Actor       string          `json:"actor"`
	Kind        string          `json:"kind"`
	Visibility  string          `json:"visibility"`
	RecipientID int64           `json:"recipient_id"`
	Content     string          `json:"content"`
	Payload     json.RawMessage `json:"payload"`
	CreatedAt   int64           `json:"created_at"`
}

func (a *app) gameEvents(c campaign, g gameActor, before int64, publicOnly bool) ([]campaignEvent, error) {
	privateUser := int64(0)
	if g.AgentID == 0 {
		privateUser = g.ID
	}
	if publicOnly {
		privateUser = 0
		g = gameActor{}
	}
	rows, err := a.store.Query(`SELECT e.id,e.actor_id,u.nickname,e.kind,e.visibility,e.recipient_id,e.content,e.payload,e.created_at
		FROM campaign_events e JOIN users u ON u.id=e.actor_id WHERE e.campaign_id=? AND (?=0 OR e.id<?)
		AND (e.visibility='public' OR ? OR (e.visibility='private' AND (e.recipient_id=? OR e.actor_id=?)))
		AND (?=0 OR NOT EXISTS(SELECT 1 FROM campaign_turns t WHERE t.event_id=e.id AND t.id>?))
		ORDER BY e.id DESC LIMIT 80`, c.ID, before, before, g.secrets(c), privateUser, privateUser,
		publicContextTurn(c, publicOnly), publicContextTurn(c, publicOnly))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []campaignEvent{}
	for rows.Next() {
		var e campaignEvent
		var raw string
		if err = rows.Scan(&e.ID, &e.ActorID, &e.Actor, &e.Kind, &e.Visibility, &e.RecipientID, &e.Content, &raw, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.Payload = json.RawMessage(raw)
		out = append(out, e)
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, rows.Err()
}
func publicContextTurn(c campaign, publicOnly bool) int64 {
	if publicOnly {
		return c.ActiveTurn
	}
	return 0
}
func (a *app) gameEventsHTTP(r *http.Request, g gameActor) (any, error) {
	c, err := a.gameAccess(pathID(r, "id"), g)
	if err != nil {
		return nil, err
	}
	before, _ := strconv.ParseInt(r.URL.Query().Get("before"), 10, 64)
	return a.gameEvents(c, g, before, false)
}
func (a *app) gamePublicContext(c campaign) (any, error) {
	chars, err := a.gameCharacters(c, gameActor{})
	if err != nil {
		return nil, err
	}
	events, err := a.gameEvents(c, gameActor{}, 0, true)
	if err != nil {
		return nil, err
	}
	// Public generation deliberately never receives DM notes or private character fields.
	return map[string]any{"state": c, "characters": chars, "events": events, "secret_policy": "这里只含公开资料；秘密通过单独授权接口访问，待公开的线索由房主批准为场景。"}, nil
}
func (a *app) gameContextHTTP(r *http.Request, g gameActor) (any, error) {
	c, err := a.gameAccess(pathID(r, "id"), g)
	if err != nil {
		return nil, err
	}
	return a.gamePublicContext(c)
}
func queueGameTurn(tx *sql.Tx, c campaign, eventID int64, kind string) (int64, error) {
	var count int
	if err := tx.QueryRow("SELECT COUNT(*) FROM campaign_turns WHERE campaign_id=? AND status IN ('queued','running')", c.ID).Scan(&count); err != nil {
		return 0, err
	}
	if count >= 20 {
		return 0, gameFail(429, "待处理行动已满，请等待主持人处理")
	}
	res, err := tx.Exec("INSERT INTO campaign_turns(campaign_id,event_id,kind) VALUES(?,?,?)", c.ID, eventID, kind)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	return id, nil
}
func (a *app) gameAction(r *http.Request, g gameActor) (any, error) {
	var in struct {
		gameOp
		Content string `json:"content"`
	}
	if err := gameBody(r, &in); err != nil {
		return nil, err
	}
	out, err := a.gameMutation(pathID(r, "id"), g, in.gameOp, "action", in, func(tx *sql.Tx, c campaign) (any, error) {
		if g.AgentID != 0 {
			return nil, gameFail(403, "智能体不能替玩家提交行动")
		}
		if c.Phase == "paused" {
			return nil, gameFail(409, "战役已暂停，请房主继续后再提交行动")
		}
		if !textOK(in.Content, 1, 4000) {
			return nil, gameFail(400, "行动内容为 1–4000 字")
		}
		content := strings.TrimSpace(in.Content)
		id, err := gameEvent(tx, c, g.ID, "action", "public", 0, content, map[string]any{})
		if err != nil {
			return nil, err
		}
		var turn int64
		if c.AutoRespond && c.Mode != "manual" {
			turn, err = queueGameTurn(tx, c, id, "action")
		}
		return map[string]any{"event_id": id, "turn_id": turn}, err
	})
	if err == nil {
		a.kickGame(pathID(r, "id"))
	}
	return out, err
}

type gameRollInput struct {
	gameOp
	Formula    string `json:"formula"`
	Reason     string `json:"reason"`
	Visibility string `json:"visibility"`
	TurnID     int64  `json:"turn_id"`
	Lease      string `json:"lease"`
}

func (a *app) gameRollValue(cid int64, g gameActor, in gameRollInput) (any, error) {
	fingerprint := in
	fingerprint.Lease = ""
	return a.gameMutation(cid, g, in.gameOp, "roll", fingerprint, func(tx *sql.Tx, c campaign) (any, error) {
		if in.Visibility == "" {
			in.Visibility = "public"
		}
		if in.Visibility != "public" && in.Visibility != "dm" {
			return nil, gameFail(400, "掷骰可见范围无效")
		}
		if in.Visibility == "dm" && !g.secrets(c) {
			return nil, gameFail(403, "没有秘密掷骰权限")
		}
		if !textOK(in.Reason, 1, 200) {
			return nil, gameFail(400, "请填写 1–200 字的检定目的")
		}
		if c.Phase == "paused" {
			return nil, gameFail(409, "战役已暂停")
		}
		if in.TurnID != 0 {
			if err := checkGameLease(tx, c, g, in.TurnID, in.Lease); err != nil {
				return nil, err
			}
		} else if c.ActiveTurn != 0 {
			return nil, gameFail(409, "请等待当前主持回合结束")
		}
		roll, err := rollDice(in.Formula)
		if err != nil {
			return nil, err
		}
		text := fmt.Sprintf("%s · %s → %d\n骰子 %v；保留 %v；修正 %+d", in.Reason, roll.Formula, roll.Total, roll.Dice, roll.Kept, roll.Modifier)
		id, err := gameEvent(tx, c, g.ID, "roll", in.Visibility, 0, text, map[string]any{"roll": roll, "turn_id": in.TurnID})
		if err != nil {
			return nil, err
		}
		var turn int64
		if in.TurnID == 0 && in.Visibility == "public" && c.AutoRespond && c.Mode != "manual" {
			turn, err = queueGameTurn(tx, c, id, "action")
		}
		return map[string]any{"event_id": id, "turn_id": turn, "roll": roll}, err
	})
}
func (a *app) gameRoll(r *http.Request, g gameActor) (any, error) {
	var in gameRollInput
	if err := gameBody(r, &in); err != nil {
		return nil, err
	}
	out, err := a.gameRollValue(pathID(r, "id"), g, in)
	if err == nil {
		a.kickGame(pathID(r, "id"))
	}
	return out, err
}
func (a *app) gameMessage(r *http.Request, g gameActor) (any, error) {
	var in struct {
		gameOp
		Content     string `json:"content"`
		Visibility  string `json:"visibility"`
		RecipientID int64  `json:"recipient_id"`
		Kind        string `json:"kind"`
	}
	if err := gameBody(r, &in); err != nil {
		return nil, err
	}
	return a.gameMutation(pathID(r, "id"), g, in.gameOp, "message", in, func(tx *sql.Tx, c campaign) (any, error) {
		if !textOK(in.Content, 1, 8000) {
			return nil, gameFail(400, "内容为 1–8000 字")
		}
		if in.Visibility == "" {
			in.Visibility = "public"
		}
		if in.Kind == "" {
			in.Kind = "narration"
		}
		if in.Visibility == "public" {
			if !g.host(c) {
				return nil, gameFail(403, "公开叙述由主持人发送；玩家请提交行动")
			}
			if err := gameVersion(c, in.Version); err != nil {
				return nil, err
			}
			if in.Kind != "narration" && in.Kind != "summary" {
				return nil, gameFail(400, "消息类型无效")
			}
		} else if in.Visibility == "private" {
			if g.AgentID != 0 && !g.Secrets {
				return nil, gameFail(403, "此智能体未获私密资料权限")
			}
			if !g.host(c) && in.RecipientID != c.OwnerID {
				return nil, gameFail(403, "玩家私密消息只能发给房主")
			}
			var n int
			if tx.QueryRow("SELECT COUNT(*) FROM members WHERE conversation_id=? AND user_id=? AND user_id<>?", c.ConversationID, in.RecipientID, c.BotID).Scan(&n) != nil || n != 1 {
				return nil, gameFail(400, "接收者不属于此战役")
			}
			in.Kind = "whisper"
		} else {
			return nil, gameFail(400, "请选择公开或私密消息")
		}
		actor := g.ID
		if in.Visibility == "public" {
			actor = c.BotID
		}
		id, err := gameEvent(tx, c, actor, in.Kind, in.Visibility, in.RecipientID, in.Content, map[string]any{})
		if err != nil {
			return nil, err
		}
		if in.Visibility == "public" {
			if in.Kind == "summary" {
				_, err = tx.Exec("UPDATE campaigns SET summary=?,phase='paused' WHERE id=?", in.Content, c.ID)
			}
			if err == nil {
				err = bumpGame(tx, c.ID)
			}
		}
		return map[string]any{"event_id": id}, err
	})
}
func (a *app) gameProposals(r *http.Request, g gameActor) (any, error) {
	c, err := a.gameAccess(pathID(r, "id"), g)
	if err != nil {
		return nil, err
	}
	// Draft material can contain unrevealed fiction; only its author and host see it.
	author := g.ID
	if g.AgentID != 0 {
		author = 0
	}
	rows, err := a.store.Query("SELECT id,author_id,kind,content,character_id,status,version,created_at FROM campaign_proposals WHERE campaign_id=? AND (? OR author_id=?) ORDER BY id DESC LIMIT 100", c.ID, g.secrets(c), author)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, uid, ch, version, at int64
		var kind, content, status string
		if err = rows.Scan(&id, &uid, &kind, &content, &ch, &status, &version, &at); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"id": id, "author_id": uid, "kind": kind, "content": content, "character_id": ch, "status": status, "version": version, "created_at": at})
	}
	return out, rows.Err()
}
func (a *app) gamePropose(r *http.Request, g gameActor) (any, error) {
	var in struct {
		gameOp
		Kind        string `json:"kind"`
		Content     string `json:"content"`
		CharacterID int64  `json:"character_id"`
	}
	if err := gameBody(r, &in); err != nil {
		return nil, err
	}
	return a.gameMutation(pathID(r, "id"), g, in.gameOp, "proposal", in, func(tx *sql.Tx, c campaign) (any, error) {
		if in.Kind != "scene" && in.Kind != "house_rules" && in.Kind != "background" {
			return nil, gameFail(400, "提案类型无效")
		}
		limit := 8000
		if in.Kind == "house_rules" {
			limit = 12000
		}
		if in.Kind == "background" {
			limit = 6000
		}
		if !textOK(in.Content, 1, limit) {
			return nil, gameFail(400, "提案内容为空或过长")
		}
		if in.Kind == "background" {
			var n int
			if tx.QueryRow("SELECT COUNT(*) FROM characters WHERE id=? AND campaign_id=?", in.CharacterID, c.ID).Scan(&n) != nil || n != 1 {
				return nil, gameFail(400, "角色不存在")
			}
		}
		res, err := tx.Exec("INSERT INTO campaign_proposals(campaign_id,author_id,kind,content,character_id,created_at) VALUES(?,?,?,?,?,?)", c.ID, g.ID, in.Kind, in.Content, in.CharacterID, time.Now().UnixMilli())
		if err != nil {
			return nil, err
		}
		id, _ := res.LastInsertId()
		return map[string]any{"id": id, "status": "pending"}, nil
	})
}
func (a *app) gameDecide(r *http.Request, g gameActor) (any, error) {
	var in struct {
		gameOp
		Accept bool `json:"accept"`
	}
	if err := gameBody(r, &in); err != nil {
		return nil, err
	}
	proposalID := pathID(r, "proposal")
	return a.gameMutation(pathID(r, "id"), g, in.gameOp, fmt.Sprintf("decide:%d", proposalID), in, func(tx *sql.Tx, c campaign) (any, error) {
		if !g.owner(c) {
			return nil, gameFail(403, "提案由房主确认")
		}
		if err := gameVersion(c, in.Version); err != nil {
			return nil, err
		}
		var kind, content, status string
		var charID int64
		if tx.QueryRow("SELECT kind,content,character_id,status FROM campaign_proposals WHERE id=? AND campaign_id=?", proposalID, c.ID).Scan(&kind, &content, &charID, &status) != nil {
			return nil, gameFail(404, "提案不存在")
		}
		if status != "pending" {
			return nil, gameFail(409, "提案已处理")
		}
		status = "rejected"
		if in.Accept {
			status = "accepted"
			var err error
			switch kind {
			case "scene":
				_, err = tx.Exec("UPDATE campaigns SET scene=? WHERE id=?", content, c.ID)
			case "house_rules":
				_, err = tx.Exec("UPDATE campaigns SET house_rules=? WHERE id=?", content, c.ID)
			case "background":
				_, err = tx.Exec("UPDATE characters SET background=?,version=version+1 WHERE id=? AND campaign_id=?", content, charID, c.ID)
			}
			if err != nil {
				return nil, err
			}
			if _, err = gameEvent(tx, c, g.ID, "setting", "public", 0, "房主已采纳设定变更："+content, map[string]any{"proposal_id": proposalID, "kind": kind}); err != nil {
				return nil, err
			}
			if err = bumpGame(tx, c.ID); err != nil {
				return nil, err
			}
		}
		_, err := tx.Exec("UPDATE campaign_proposals SET status=?,version=version+1 WHERE id=?", status, proposalID)
		return map[string]any{"status": status}, err
	})
}
func (a *app) gameTokens(r *http.Request, g gameActor) (any, error) {
	c, err := a.gameAccess(pathID(r, "id"), g)
	if err != nil {
		return nil, err
	}
	if !g.owner(c) {
		return nil, gameFail(403, "只有房主能管理智能体令牌")
	}
	rows, err := a.store.Query("SELECT id,label,secrets,expires_at,revoked FROM campaign_tokens WHERE campaign_id=? ORDER BY id DESC", c.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, expires int64
		var label string
		var secrets, revoked bool
		if err = rows.Scan(&id, &label, &secrets, &expires, &revoked); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"id": id, "label": label, "secrets": secrets, "expires_at": expires, "revoked": revoked})
	}
	return out, rows.Err()
}
func (a *app) gameTokenCreate(r *http.Request, g gameActor) (any, error) {
	var in struct {
		Label   string `json:"label"`
		Secrets bool   `json:"secrets"`
	}
	if err := gameBody(r, &in); err != nil {
		return nil, err
	}
	c, err := a.gameAccess(pathID(r, "id"), g)
	if err != nil {
		return nil, err
	}
	if !g.owner(c) {
		return nil, gameFail(403, "只有房主能签发令牌")
	}
	if !textOK(in.Label, 1, 40) {
		return nil, gameFail(400, "请填写 1–40 字的智能体名称")
	}
	if !a.limiter.allow("game-token:"+g.key(), 10, time.Hour) {
		return nil, gameFail(429, "签发过于频繁")
	}
	token := "lcgame_" + randomToken(32)
	expires := time.Now().Add(30 * 24 * time.Hour).UnixMilli()
	res, err := a.store.Exec("INSERT INTO campaign_tokens(campaign_id,token_hash,label,secrets,expires_at,created_at) VALUES(?,?,?,?,?,?)", c.ID, tokenHash(token), in.Label, in.Secrets, expires, time.Now().UnixMilli())
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return map[string]any{"id": id, "token": token, "expires_at": expires, "base_url": a.cfg.PublicURL + fmt.Sprintf("/api/v1/campaigns/%d", c.ID)}, nil
}
func (a *app) gameTokenRevoke(r *http.Request, g gameActor) (any, error) {
	c, err := a.gameAccess(pathID(r, "id"), g)
	if err != nil {
		return nil, err
	}
	if !g.owner(c) {
		return nil, gameFail(403, "只有房主能撤销令牌")
	}
	_, err = a.store.Exec("UPDATE campaign_tokens SET revoked=1 WHERE campaign_id=? AND id=?", c.ID, pathID(r, "token"))
	return map[string]any{"ok": true}, err
}
