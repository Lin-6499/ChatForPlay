package main

import (
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type gameTurn struct {
	ID          int64  `json:"id"`
	EventID     int64  `json:"event_id"`
	Kind        string `json:"kind"`
	Status      string `json:"status"`
	BaseVersion int64  `json:"base_version"`
	StartedAt   int64  `json:"started_at"`
	Error       string `json:"error"`
	Lease       string `json:"lease,omitempty"`
}

func (a *app) gameTurns(r *http.Request, g gameActor) (any, error) {
	c, err := a.gameAccess(pathID(r, "id"), g)
	if err != nil {
		return nil, err
	}
	rows, err := a.store.Query("SELECT id,event_id,kind,status,base_version,started_at,error FROM campaign_turns WHERE campaign_id=? ORDER BY id DESC LIMIT 40", c.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []gameTurn{}
	for rows.Next() {
		var t gameTurn
		if err = rows.Scan(&t.ID, &t.EventID, &t.Kind, &t.Status, &t.BaseVersion, &t.StartedAt, &t.Error); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
func checkGameLease(tx *sql.Tx, c campaign, g gameActor, turnID int64, lease string) error {
	var claimant, status, stored string
	var version, started int64
	err := tx.QueryRow("SELECT claimant,status,lease,base_version,started_at FROM campaign_turns WHERE id=? AND campaign_id=?", turnID, c.ID).Scan(&claimant, &status, &stored, &version, &started)
	if err != nil || c.ActiveTurn != turnID || c.Phase == "paused" || status != "running" || stored == "" || stored != lease || claimant != g.key() {
		return gameFail(409, "当前回合未领取、已失效或属于另一个主持人")
	}
	if time.Now().UnixMilli()-started > 180000 {
		return gameFail(409, "主持回合超时，请房主重试")
	}
	if c.Version != version {
		return gameFail(409, "战役状态已变更，主持结果未发布")
	}
	return nil
}
func (a *app) claimGame(cid int64, g gameActor) (*gameTurn, error) {
	c, err := a.gameAccess(cid, g)
	if err != nil {
		return nil, err
	}
	if (g.actorKey == "builtin" && c.Mode != "builtin") || (g.actorKey != "builtin" && (g.AgentID == 0 || c.Mode != "external")) {
		return nil, gameFail(403, "主持模式与接入方式不匹配")
	}
	tx, err := a.store.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	c, err = scanCampaign(tx.QueryRow(campaignSelect+" WHERE id=?", cid))
	if err != nil {
		return nil, err
	}
	if c.Phase == "paused" {
		return nil, gameFail(409, "战役已暂停")
	}
	var pendingChecks int
	if err := tx.QueryRow("SELECT COUNT(*) FROM campaign_checks WHERE campaign_id=? AND (status='awaiting_choice' OR (status='prepared' AND turn_id=0))", cid).Scan(&pendingChecks); err != nil {
		return nil, err
	}
	if pendingChecks > 0 && c.ActiveTurn == 0 {
		return nil, gameFail(409, "请先完成已锁定检定或等待玩家选择代价")
	}
	if (g.actorKey == "builtin" && c.Mode != "builtin") || (g.actorKey != "builtin" && c.Mode != "external") {
		return nil, gameFail(403, "主持模式已改变")
	}
	var t gameTurn
	var claimant string
	err = tx.QueryRow(`SELECT id,event_id,kind,status,base_version,started_at,error,lease,claimant FROM campaign_turns
		WHERE campaign_id=? AND status<>'done' ORDER BY id LIMIT 1`, cid).Scan(&t.ID, &t.EventID, &t.Kind, &t.Status, &t.BaseVersion, &t.StartedAt, &t.Error, &t.Lease, &claimant)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if t.Status == "running" {
		if time.Now().UnixMilli()-t.StartedAt > 180000 {
			if _, err = tx.Exec("UPDATE campaign_turns SET status='failed',lease='',error='主持回合超时，请房主重试' WHERE id=?", t.ID); err != nil {
				return nil, err
			}
			if _, err = tx.Exec("UPDATE campaigns SET active_turn=0,phase='awaiting_action' WHERE id=?", cid); err != nil {
				return nil, err
			}
			if err = tx.Commit(); err != nil {
				return nil, err
			}
			a.gameChanged(c)
			return nil, gameFail(409, "上个主持回合超时，请房主重试或跳过")
		}
		if claimant == g.key() {
			return &t, nil
		}
		return nil, gameFail(409, "其他主持人正在处理此战役")
	}
	if t.Status == "failed" {
		return nil, gameFail(409, "有失败的行动待房主重试或跳过")
	}
	t.Status = "running"
	t.BaseVersion = c.Version
	t.StartedAt = time.Now().UnixMilli()
	t.Lease = randomToken(24)
	_, err = tx.Exec("UPDATE campaign_turns SET status='running',claimant=?,lease=?,base_version=?,started_at=?,error='' WHERE id=?", g.key(), t.Lease, t.BaseVersion, t.StartedAt, t.ID)
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec("UPDATE campaigns SET phase='narrating',active_turn=? WHERE id=?", t.ID, cid); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	a.gameChanged(c)
	return &t, nil
}
func (a *app) gameClaimHTTP(r *http.Request, g gameActor) (any, error) {
	t, err := a.claimGame(pathID(r, "id"), g)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return map[string]any{"turn": nil}, nil
	}
	c, err := a.gameAccess(pathID(r, "id"), g)
	if err != nil {
		return nil, err
	}
	context, err := a.gamePublicContext(c)
	return map[string]any{"turn": t, "context": context}, err
}

type gameCharacterChange struct {
	ID         int64   `json:"id"`
	HP         *int    `json:"hp,omitempty"`
	Conditions *string `json:"conditions,omitempty"`
	Inventory  *string `json:"inventory,omitempty"`
	Resources  *string `json:"resources,omitempty"`
}
type gameOutcome struct {
	Narration string                `json:"narration"`
	Scene     *string               `json:"scene,omitempty"`
	Phase     string                `json:"phase"`
	Summary   string                `json:"summary,omitempty"`
	Changes   []gameCharacterChange `json:"changes"`
}
type gameCompleteInput struct {
	gameOp
	Lease   string      `json:"lease"`
	Outcome gameOutcome `json:"outcome"`
}

func (a *app) completeGame(cid, turnID int64, g gameActor, in gameCompleteInput) (any, error) {
	fingerprint := in
	fingerprint.Lease = ""
	return a.gameMutation(cid, g, in.gameOp, fmt.Sprintf("complete:%d", turnID), fingerprint, func(tx *sql.Tx, c campaign) (any, error) {
		if err := checkGameLease(tx, c, g, turnID, in.Lease); err != nil {
			return nil, err
		}
		if c.Version != in.Version {
			return nil, gameFail(409, "状态版本已变化")
		}
		o := in.Outcome
		var unrolled, awaiting int
		if err := tx.QueryRow("SELECT COUNT(*) FROM campaign_checks WHERE campaign_id=? AND turn_id=? AND status='prepared'", cid, turnID).Scan(&unrolled); err != nil {
			return nil, err
		}
		if unrolled > 0 {
			return nil, gameFail(409, "请先投掷本回合已锁定的检定")
		}
		if err := tx.QueryRow("SELECT COUNT(*) FROM campaign_checks WHERE campaign_id=? AND status='awaiting_choice'", cid).Scan(&awaiting); err != nil {
			return nil, err
		}
		if awaiting > 0 {
			o.Phase = "awaiting_roll"
		}
		if !textOK(o.Narration, 1, 8000) || !textOK(o.Summary, 0, 8000) || len(o.Changes) > 50 {
			return nil, gameFail(400, "主持结果为空或过长")
		}
		if o.Phase != "awaiting_action" && o.Phase != "awaiting_roll" {
			return nil, gameFail(400, "主持后应等待玩家行动或掷骰")
		}
		if o.Scene != nil && !textOK(*o.Scene, 0, 8000) {
			return nil, gameFail(400, "场景内容过长")
		}
		seen := map[int64]bool{}
		for _, change := range o.Changes {
			if err := pendingCharacterCheck(tx, cid, change.ID); err != nil {
				return nil, err
			}
			var applied int
			if err := tx.QueryRow("SELECT COUNT(*) FROM campaign_checks WHERE campaign_id=? AND turn_id=? AND character_id=? AND status='resolved'", cid, turnID, change.ID).Scan(&applied); err != nil {
				return nil, err
			}
			if applied > 0 {
				return nil, gameFail(409, "该角色的检定后果已由软件应用，不得重复修改资源")
			}
			if seen[change.ID] {
				return nil, gameFail(400, "同一角色只能更新一次")
			}
			seen[change.ID] = true
			var hp, max int
			var conditions, inventory, resources string
			if tx.QueryRow("SELECT hp,max_hp,conditions,inventory,resources FROM characters WHERE campaign_id=? AND id=?", cid, change.ID).Scan(&hp, &max, &conditions, &inventory, &resources) != nil {
				return nil, gameFail(400, "角色不属于此战役")
			}
			if change.HP != nil {
				hp = *change.HP
			}
			if hp < 0 || hp > max {
				return nil, gameFail(400, "角色生命值超出范围")
			}
			for _, v := range []*string{change.Conditions, change.Inventory, change.Resources} {
				if v != nil && !textOK(*v, 0, 6000) {
					return nil, gameFail(400, "角色状态内容过长")
				}
			}
			if change.Conditions != nil {
				conditions = *change.Conditions
			}
			if change.Inventory != nil {
				inventory = *change.Inventory
			}
			if change.Resources != nil {
				resources = *change.Resources
			}
			if _, err := tx.Exec("UPDATE characters SET hp=?,conditions=?,inventory=?,resources=?,version=version+1 WHERE id=?", hp, conditions, inventory, resources, change.ID); err != nil {
				return nil, err
			}
		}
		var kind string
		if err := tx.QueryRow("SELECT kind FROM campaign_turns WHERE id=?", turnID).Scan(&kind); err != nil {
			return nil, err
		}
		eventKind := "narration"
		content := o.Narration
		if kind == "summary" {
			if !textOK(o.Summary, 1, 8000) {
				return nil, gameFail(400, "结束时必须生成战役摘要")
			}
			c.Summary = o.Summary
			o.Phase = "paused"
			eventKind = "summary"
			content = o.Summary
		} else if o.Summary != "" {
			return nil, gameFail(400, "摘要只能通过结束本次流程生成")
		}
		if o.Scene != nil {
			c.Scene = *o.Scene
		}
		id, err := gameEvent(tx, c, c.BotID, eventKind, "public", 0, content, map[string]any{"turn_id": turnID, "changes": o.Changes})
		if err != nil {
			return nil, err
		}
		_, err = tx.Exec("UPDATE campaigns SET scene=?,summary=?,phase=?,active_turn=0,version=version+1,updated_at=? WHERE id=?", c.Scene, c.Summary, o.Phase, time.Now().UnixMilli(), cid)
		if err != nil {
			return nil, err
		}
		result := map[string]any{"event_id": id, "turn_id": turnID, "version": c.Version + 1}
		_, err = tx.Exec("UPDATE campaign_turns SET status='done',lease='',result=? WHERE id=?", gameJSON(result), turnID)
		return result, err
	})
}
func (a *app) gameCompleteHTTP(r *http.Request, g gameActor) (any, error) {
	var in gameCompleteInput
	if err := gameBody(r, &in); err != nil {
		return nil, err
	}
	return a.completeGame(pathID(r, "id"), pathID(r, "turn"), g, in)
}
func (a *app) gameFailHTTP(r *http.Request, g gameActor) (any, error) {
	var in struct {
		gameOp
		Lease string `json:"lease"`
	}
	if err := gameBody(r, &in); err != nil {
		return nil, err
	}
	id := pathID(r, "turn")
	return a.gameMutation(pathID(r, "id"), g, in.gameOp, fmt.Sprintf("fail:%d", id), in, func(tx *sql.Tx, c campaign) (any, error) {
		if err := checkGameLease(tx, c, g, id, in.Lease); err != nil {
			return nil, err
		}
		if _, err := tx.Exec("UPDATE campaign_turns SET status='failed',lease='',error='外部主持未完成，请房主检查后重试；掷骰结果已保留' WHERE id=?", id); err != nil {
			return nil, err
		}
		_, err := tx.Exec("UPDATE campaigns SET active_turn=0,phase='awaiting_action' WHERE id=?", c.ID)
		return map[string]any{"ok": true}, err
	})
}
func (a *app) failGameTurn(cid int64, t gameTurn, message string) {
	tx, err := a.store.Begin()
	if err != nil {
		return
	}
	defer tx.Rollback()
	res, err := tx.Exec("UPDATE campaign_turns SET status='failed',error=?,lease='' WHERE id=? AND campaign_id=? AND status='running' AND lease=?", message, t.ID, cid, t.Lease)
	if err != nil {
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return
	}
	if _, err = tx.Exec("UPDATE campaigns SET active_turn=0,phase=CASE WHEN phase='paused' THEN phase ELSE 'awaiting_action' END WHERE id=? AND active_turn=?", cid, t.ID); err != nil {
		return
	}
	if tx.Commit() == nil {
		if c, e := scanCampaign(a.store.QueryRow(campaignSelect+" WHERE id=?", cid)); e == nil {
			a.gameChanged(c)
		}
	}
}
func (a *app) gameRetry(r *http.Request, g gameActor) (any, error) {
	var in struct {
		gameOp
		Skip bool `json:"skip"`
	}
	if err := gameBody(r, &in); err != nil {
		return nil, err
	}
	turnID := pathID(r, "turn")
	out, err := a.gameMutation(pathID(r, "id"), g, in.gameOp, fmt.Sprintf("retry:%d", turnID), in, func(tx *sql.Tx, c campaign) (any, error) {
		if !g.owner(c) {
			return nil, gameFail(403, "只有房主能重试或跳过")
		}
		var status string
		var started int64
		if tx.QueryRow("SELECT status,started_at FROM campaign_turns WHERE campaign_id=? AND id=?", c.ID, turnID).Scan(&status, &started) != nil {
			return nil, gameFail(404, "回合不存在")
		}
		if status != "failed" && (status != "running" || time.Now().UnixMilli()-started <= 180000) {
			return nil, gameFail(409, "只能处理失败或已超时的回合")
		}
		if c.Version != in.Version {
			return nil, gameFail(409, "请刷新战役状态")
		}
		next := "queued"
		if in.Skip {
			next = "done"
		}
		if _, err := tx.Exec("UPDATE campaign_turns SET status=?,error='',lease='',claimant='' WHERE id=?", next, turnID); err != nil {
			return nil, err
		}
		if c.ActiveTurn == turnID {
			if _, err := tx.Exec("UPDATE campaigns SET active_turn=0,phase=CASE WHEN phase='paused' THEN phase ELSE 'awaiting_action' END WHERE id=?", c.ID); err != nil {
				return nil, err
			}
		}
		return map[string]any{"status": next}, nil
	})
	if err == nil {
		a.kickGame(pathID(r, "id"))
	}
	return out, err
}
func (a *app) gameHost(r *http.Request, g gameActor) (any, error) {
	var in struct {
		gameOp
		Kind string `json:"kind"`
	}
	if err := gameBody(r, &in); err != nil {
		return nil, err
	}
	out, err := a.gameMutation(pathID(r, "id"), g, in.gameOp, "host", in, func(tx *sql.Tx, c campaign) (any, error) {
		if !g.owner(c) {
			return nil, gameFail(403, "只有房主能主动要求主持或结束本次")
		}
		if c.Phase == "paused" {
			return nil, gameFail(409, "请先继续战役")
		}
		if c.Mode == "manual" {
			return nil, gameFail(409, "当前为手动主持，请在主持栏撰写叙述或切换 AI 模式")
		}
		if in.Kind != "continue" && in.Kind != "summary" {
			return nil, gameFail(400, "主持请求类型无效")
		}
		content := "请根据当前公开场景与玩家已提交的行动继续主持。"
		kind := "action"
		if in.Kind == "summary" {
			kind = "summary"
			content = "结束本次跑团，归纳已经发生的事实、资源变化和未完成线索，生成供下次使用的战役摘要。"
		}
		id, err := gameEvent(tx, c, g.ID, "request", "public", 0, content, map[string]any{})
		if err != nil {
			return nil, err
		}
		turn, err := queueGameTurn(tx, c, id, kind)
		return map[string]any{"event_id": id, "turn_id": turn}, err
	})
	if err == nil {
		a.kickGame(pathID(r, "id"))
	}
	return out, err
}

// Group @mentions use the same durable event and queue as the campaign composer.
func (a *app) gameMention(tx *sql.Tx, m message) (int64, error) {
	c, err := scanCampaign(tx.QueryRow(campaignSelect+" WHERE conversation_id=?", m.ConversationID))
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if m.SenderID == c.BotID || c.Phase == "paused" {
		return 0, nil
	}
	mentioned := false
	for _, mt := range m.Mentions {
		if mt.UserID == c.BotID {
			mentioned = true
		}
	}
	if !mentioned {
		return 0, nil
	}
	id, err := gameEvent(tx, c, m.SenderID, "action", "public", 0, m.Content, map[string]any{"message_id": m.ID})
	if err != nil {
		return 0, err
	}
	if c.AutoRespond && c.Mode != "manual" {
		_, err = queueGameTurn(tx, c, id, "action")
	}
	return c.ID, err
}

// Keep JSON tool decoding strict too; callers cannot smuggle extra state fields.
func gameToolArgs(raw string, out any) error {
	req, _ := http.NewRequest("POST", "http://localhost", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Body = io.NopCloser(strings.NewReader(raw))
	return gameBody(req, out)
}
