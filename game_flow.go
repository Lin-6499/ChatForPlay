package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
)

type campaignFlow struct {
	Mode         string  `json:"mode"`
	Participants []int64 `json:"participants"`
	Round        int64   `json:"round"`
}

func readCampaignFlow(row scanner) (campaignFlow, error) {
	out := campaignFlow{Mode: "individual", Participants: []int64{}, Round: 1}
	var raw string
	err := row.Scan(&out.Mode, &raw, &out.Round)
	if err == sql.ErrNoRows {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	err = json.Unmarshal([]byte(raw), &out.Participants)
	return out, err
}
func collectCampaignAction(tx *sql.Tx, c campaign, uid, eid int64) (int64, bool, error) {
	flow, err := readCampaignFlow(tx.QueryRow("SELECT mode,participants,round FROM campaign_flow WHERE campaign_id=?", c.ID))
	if err != nil {
		return 0, false, err
	}
	if flow.Mode != "collect" {
		id, err := queueGameTurn(tx, c, eid, "action")
		return id, false, err
	}
	allowed := false
	for _, id := range flow.Participants {
		if id == uid {
			allowed = true
		}
	}
	if !allowed {
		return 0, false, gameFail(403, "你未纳入本轮收集名单，请由房主调整名单或使用群聊")
	}
	var exists int
	if err = tx.QueryRow("SELECT count(*) FROM campaign_collected_actions WHERE campaign_id=? AND round=? AND user_id=?", c.ID, flow.Round, uid).Scan(&exists); err != nil {
		return 0, false, err
	}
	if exists > 0 {
		return 0, false, gameFail(409, "本轮已提交行动，可在行动队列中补充，不要重复提交")
	}
	if _, err = tx.Exec("INSERT INTO campaign_collected_actions(campaign_id,round,user_id,event_id) VALUES(?,?,?,?)", c.ID, flow.Round, uid, eid); err != nil {
		return 0, false, err
	}
	var count int
	if err = tx.QueryRow("SELECT count(*) FROM campaign_collected_actions WHERE campaign_id=? AND round=? AND turn_id=0", c.ID, flow.Round).Scan(&count); err != nil {
		return 0, false, err
	}
	if count < len(flow.Participants) {
		return 0, true, nil
	}
	id, err := releaseCampaignCollection(tx, c, flow)
	return id, false, err
}
func releaseCampaignCollection(tx *sql.Tx, c campaign, flow campaignFlow) (int64, error) {
	var first int64
	err := tx.QueryRow("SELECT COALESCE(MIN(event_id),0) FROM campaign_collected_actions WHERE campaign_id=? AND round=? AND turn_id=0", c.ID, flow.Round).Scan(&first)
	if err != nil {
		return 0, err
	}
	if first == 0 {
		return 0, gameFail(409, "本轮尚无行动可处理")
	}
	tid, err := queueGameTurn(tx, c, first, "group")
	if err != nil {
		return 0, err
	}
	if _, err = tx.Exec("UPDATE campaign_collected_actions SET turn_id=? WHERE campaign_id=? AND round=? AND turn_id=0", tid, c.ID, flow.Round); err != nil {
		return 0, err
	}
	_, err = tx.Exec("UPDATE campaign_flow SET round=round+1 WHERE campaign_id=?", c.ID)
	return tid, err
}
func (a *app) gameFlowSave(r *http.Request, g gameActor) (any, error) {
	var in struct {
		gameOp
		Mode         string  `json:"mode"`
		Participants []int64 `json:"participants"`
	}
	if err := gameBody(r, &in); err != nil {
		return nil, err
	}
	if in.Mode != "individual" && in.Mode != "collect" {
		return nil, gameFail(400, "行动节奏无效")
	}
	return a.gameMutation(pathID(r, "id"), g, in.gameOp, "flow", in, func(tx *sql.Tx, c campaign) (any, error) {
		if !g.owner(c) {
			return nil, gameFail(403, "只有房主可设置行动节奏")
		}
		if err := gameVersion(c, in.Version); err != nil {
			return nil, err
		}
		var pending int
		if err := tx.QueryRow(`SELECT (SELECT count(*) FROM campaign_turns WHERE campaign_id=? AND status<>'done')+
		  (SELECT count(*) FROM campaign_collected_actions WHERE campaign_id=? AND turn_id=0)`, c.ID, c.ID).Scan(&pending); err != nil {
			return nil, err
		}
		if c.ActiveTurn != 0 || pending > 0 {
			return nil, gameFail(409, "请先处理待办或已收集的行动，再更改节奏与名单")
		}
		if in.Mode == "collect" && (c.Mode == "manual" || !c.AutoRespond) {
			return nil, gameFail(409, "全员收集需要启用自动排队及内置或外部主持")
		}
		if len(in.Participants) > 12 || in.Mode == "collect" && len(in.Participants) < 1 {
			return nil, gameFail(400, "请指定1–12位本轮参与者")
		}
		seen := map[int64]bool{}
		for _, id := range in.Participants {
			var count int
			if err := tx.QueryRow(`SELECT count(*) FROM members m WHERE conversation_id=? AND user_id=?
			  AND NOT EXISTS(SELECT 1 FROM service_accounts s WHERE s.user_id=m.user_id)`, c.ConversationID, id).Scan(&count); err != nil {
				return nil, err
			}
			if count != 1 || seen[id] {
				return nil, gameFail(400, "参与者必须是本战役真人成员且不可重复")
			}
			seen[id] = true
		}
		if in.Participants == nil {
			in.Participants = []int64{}
		}
		if _, err := tx.Exec(`INSERT INTO campaign_flow(campaign_id,mode,participants) VALUES(?,?,?)
		 ON CONFLICT(campaign_id) DO UPDATE SET mode=excluded.mode,participants=excluded.participants`, c.ID, in.Mode, gameJSON(in.Participants)); err != nil {
			return nil, err
		}
		if _, err := tx.Exec("UPDATE campaigns SET version=version+1 WHERE id=?", c.ID); err != nil {
			return nil, err
		}
		return map[string]any{"mode": in.Mode, "version": c.Version + 1}, nil
	})
}
func (a *app) gameFlowRelease(r *http.Request, g gameActor) (any, error) {
	var in struct {
		gameOp
		AllowMissing bool `json:"allow_missing"`
	}
	if err := gameBody(r, &in); err != nil {
		return nil, err
	}
	out, err := a.gameMutation(pathID(r, "id"), g, in.gameOp, "flow-release", in, func(tx *sql.Tx, c campaign) (any, error) {
		if !g.owner(c) {
			return nil, gameFail(403, "只有房主可结束收集")
		}
		if err := gameVersion(c, in.Version); err != nil {
			return nil, err
		}
		if c.Phase == "paused" {
			return nil, gameFail(409, "请先继续战役")
		}
		flow, err := readCampaignFlow(tx.QueryRow("SELECT mode,participants,round FROM campaign_flow WHERE campaign_id=?", c.ID))
		if err != nil {
			return nil, err
		}
		if flow.Mode != "collect" || !in.AllowMissing {
			return nil, gameFail(400, "需要明确确认本轮未提交者将跳过")
		}
		id, err := releaseCampaignCollection(tx, c, flow)
		return map[string]any{"turn_id": id}, err
	})
	if err == nil {
		a.kickGame(pathID(r, "id"))
	}
	return out, err
}

type flowAction struct {
	EventID     int64            `json:"event_id"`
	UserID      int64            `json:"user_id"`
	Actor       string           `json:"actor"`
	Content     string           `json:"content"`
	Supplements []map[string]any `json:"supplements"`
}

func (a *app) turnActionDetails(cid, tid, eid int64) ([]flowAction, error) {
	rows, err := a.store.Query(`SELECT e.id,e.actor_id,u.nickname,e.content FROM campaign_events e JOIN users u ON u.id=e.actor_id
	  WHERE e.campaign_id=? AND e.kind='action' AND (e.id=? OR e.id IN(SELECT event_id FROM campaign_collected_actions WHERE campaign_id=? AND turn_id=? AND turn_id<>0)) ORDER BY e.id`, cid, eid, cid, tid)
	if err != nil {
		return nil, err
	}
	out := []flowAction{}
	for rows.Next() {
		var item flowAction
		if err = rows.Scan(&item.EventID, &item.UserID, &item.Actor, &item.Content); err != nil {
			rows.Close()
			return nil, err
		}
		item.Supplements = []map[string]any{}
		out = append(out, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	for i := range out {
		supp, err := a.store.Query(`SELECT e.id,e.content FROM campaign_action_supplements s JOIN campaign_events e ON e.id=s.event_id WHERE s.base_event_id=? AND e.campaign_id=? ORDER BY e.id`, out[i].EventID, cid)
		if err != nil {
			return nil, err
		}
		for supp.Next() {
			var id int64
			var text string
			if err = supp.Scan(&id, &text); err != nil {
				supp.Close()
				return nil, err
			}
			out[i].Supplements = append(out[i].Supplements, map[string]any{"event_id": id, "content": text})
		}
		err = supp.Err()
		supp.Close()
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}
func (a *app) gameFlowRead(r *http.Request, g gameActor) (any, error) {
	c, err := a.gameAccess(pathID(r, "id"), g)
	if err != nil {
		return nil, err
	}
	flow, err := readCampaignFlow(a.store.QueryRow("SELECT mode,participants,round FROM campaign_flow WHERE campaign_id=?", c.ID))
	if err != nil {
		return nil, err
	}
	rows, err := a.store.Query(`SELECT id,event_id,kind,status,error,started_at FROM campaign_turns WHERE campaign_id=? AND status<>'done' ORDER BY id LIMIT 20`, c.ID)
	if err != nil {
		return nil, err
	}
	queue := []map[string]any{}
	for rows.Next() {
		var id, event, started int64
		var kind, status, message string
		if err = rows.Scan(&id, &event, &kind, &status, &message, &started); err != nil {
			rows.Close()
			return nil, err
		}
		queue = append(queue, map[string]any{"turn_id": id, "event_id": event, "kind": kind, "status": status, "error": message, "started_at": started})
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	for _, turn := range queue {
		actions, e := a.turnActionDetails(c.ID, turn["turn_id"].(int64), turn["event_id"].(int64))
		if e != nil {
			return nil, e
		}
		turn["actions"] = actions
	}
	collected := []flowAction{}
	waiting := []map[string]any{}
	if flow.Mode == "collect" {
		for _, id := range flow.Participants {
			var eid int64
			var name string
			var member int
			if err = a.store.QueryRow("SELECT nickname FROM users WHERE id=?", id).Scan(&name); err != nil {
				return nil, err
			}
			if err = a.store.QueryRow("SELECT count(*) FROM members WHERE conversation_id=? AND user_id=?", c.ConversationID, id).Scan(&member); err != nil {
				return nil, err
			}
			err = a.store.QueryRow("SELECT event_id FROM campaign_collected_actions WHERE campaign_id=? AND round=? AND user_id=?", c.ID, flow.Round, id).Scan(&eid)
			if err == sql.ErrNoRows {
				waiting = append(waiting, map[string]any{"user_id": id, "actor": name, "left": member == 0})
				continue
			}
			if err != nil {
				return nil, err
			}
			items, e := a.turnActionDetails(c.ID, 0, eid)
			if e != nil {
				return nil, e
			}
			collected = append(collected, items...)
		}
	}
	var choice int
	if err = a.store.QueryRow("SELECT count(*) FROM campaign_checks WHERE campaign_id=? AND status='awaiting_choice'", c.ID).Scan(&choice); err != nil {
		return nil, err
	}
	return map[string]any{"flow": flow, "queue": queue, "collected": collected, "waiting": waiting, "awaiting_choices": choice, "phase": c.Phase}, nil
}
func (a *app) gameSupplement(r *http.Request, g gameActor) (any, error) {
	var in struct {
		gameOp
		Content string `json:"content"`
	}
	if err := gameBody(r, &in); err != nil {
		return nil, err
	}
	base := pathID(r, "event")
	if !textOK(in.Content, 1, 1000) {
		return nil, gameFail(400, "补充内容需1–1000字")
	}
	return a.gameMutation(pathID(r, "id"), g, in.gameOp, fmt.Sprintf("supplement:%d", base), in, func(tx *sql.Tx, c campaign) (any, error) {
		if g.AgentID != 0 {
			return nil, gameFail(403, "主持智能体不能替玩家补充行动")
		}
		var actor int64
		var kind string
		if tx.QueryRow("SELECT actor_id,kind FROM campaign_events WHERE campaign_id=? AND id=?", c.ID, base).Scan(&actor, &kind) != nil || actor != g.ID || kind != "action" {
			return nil, gameFail(403, "只能补充自己的玩家行动")
		}
		if c.Phase == "paused" {
			return nil, gameFail(409, "战役暂停中")
		}
		var pending int
		if err := tx.QueryRow(`SELECT (SELECT count(*) FROM campaign_turns t WHERE t.campaign_id=? AND t.status='queued'
		  AND (t.event_id=? OR t.id IN(SELECT turn_id FROM campaign_collected_actions WHERE event_id=?)))+
		  (SELECT count(*) FROM campaign_collected_actions WHERE campaign_id=? AND event_id=? AND turn_id=0)`, c.ID, base, base, c.ID, base).Scan(&pending); err != nil {
			return nil, err
		}
		if pending == 0 {
			return nil, gameFail(409, "行动已领取或处理，不能补充；请提交新行动")
		}
		var count int
		if err := tx.QueryRow("SELECT count(*) FROM campaign_action_supplements WHERE base_event_id=?", base).Scan(&count); err != nil {
			return nil, err
		}
		if count >= 5 {
			return nil, gameFail(400, "每条行动最多5次补充")
		}
		id, err := gameEvent(tx, c, g.ID, "action_supplement", "public", 0, in.Content, map[string]any{"base_event_id": base})
		if err != nil {
			return nil, err
		}
		if _, err = tx.Exec("INSERT INTO campaign_action_supplements(event_id,base_event_id) VALUES(?,?)", id, base); err != nil {
			return nil, err
		}
		return map[string]any{"event_id": id, "base_event_id": base}, nil
	})
}
