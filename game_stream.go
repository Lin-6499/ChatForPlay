package main

import (
	"database/sql"
	"net/http"
	"time"
)

// Snapshots, rather than append-only chunks, make a lost response retry safe.
// Drafts never enter messages/events or mutate the campaign version.
type gameStreamInput struct {
	Lease     string `json:"lease"`
	Version   int64  `json:"version"`
	Seq       int64  `json:"seq"`
	Narration string `json:"narration"`
}
type gameDraft struct {
	TurnID    int64  `json:"turn_id"`
	Seq       int64  `json:"seq"`
	Narration string `json:"narration"`
	ExpiresAt int64  `json:"expires_at"`
}

func (a *app) gameStreamHTTP(r *http.Request, g gameActor) (any, error) {
	var in gameStreamInput
	if err := gameBody(r, &in); err != nil {
		return nil, err
	}
	return a.streamGame(pathID(r, "id"), pathID(r, "turn"), g, in)
}
func (a *app) streamGame(cid, tid int64, g gameActor, in gameStreamInput) (any, error) {
	if in.Seq < 1 || in.Seq > 100000 || !textOK(in.Narration, 0, 12000) {
		return nil, gameFail(400, "流式正文或序号无效")
	}
	if _, err := a.gameAccess(cid, g); err != nil {
		return nil, err
	}
	tx, err := a.store.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	c, err := scanCampaign(tx.QueryRow(campaignSelect+" WHERE id=?", cid))
	if err != nil {
		return nil, err
	}
	if err = checkGameLease(tx, c, g, tid, in.Lease); err != nil {
		return nil, err
	}
	if c.Version != in.Version {
		return nil, gameFail(409, "战役版本已改变")
	}
	var old gameDraft
	var lease string
	err = tx.QueryRow("SELECT turn_id,lease,seq,narration FROM campaign_drafts WHERE campaign_id=?", cid).Scan(&old.TurnID, &lease, &old.Seq, &old.Narration)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	if old.TurnID == tid && lease == in.Lease && in.Seq <= old.Seq {
		if in.Seq == old.Seq && in.Narration == old.Narration {
			return map[string]any{"seq": in.Seq}, nil
		}
		return nil, gameFail(409, "分片序号已过期或内容冲突")
	}
	_, err = tx.Exec(`INSERT INTO campaign_drafts(campaign_id,turn_id,lease,seq,narration) VALUES(?,?,?,?,?)
	 ON CONFLICT(campaign_id) DO UPDATE SET turn_id=excluded.turn_id,lease=excluded.lease,seq=excluded.seq,narration=excluded.narration`, cid, tid, in.Lease, in.Seq, in.Narration)
	if err != nil {
		return nil, err
	}
	if err = touchGameHost(tx, cid, "stream_at"); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	a.hub.broadcast(a.store.memberIDs(c.ConversationID), map[string]any{"type": "campaign.stream", "campaign_id": cid})
	return map[string]any{"seq": in.Seq}, nil
}
func (a *app) gameStreamRead(r *http.Request, g gameActor) (any, error) {
	c, err := a.gameAccess(pathID(r, "id"), g)
	if err != nil {
		return nil, err
	}
	var d gameDraft
	err = a.store.QueryRow(`SELECT d.turn_id,d.seq,d.narration,t.started_at+180000 FROM campaign_drafts d
	 JOIN campaign_turns t ON t.id=d.turn_id JOIN campaigns c ON c.id=d.campaign_id
	 WHERE c.id=? AND c.active_turn=t.id AND c.phase='narrating' AND t.status='running'
	 AND d.lease=t.lease AND t.base_version=c.version AND t.started_at>?
	 AND (t.claimant='builtin' OR EXISTS (SELECT 1 FROM campaign_tokens token
	   WHERE 'agent:' || token.id=t.claimant AND token.campaign_id=c.id AND token.revoked=0 AND token.expires_at>?))`, c.ID, time.Now().UnixMilli()-180000, time.Now().UnixMilli()).Scan(&d.TurnID, &d.Seq, &d.Narration, &d.ExpiresAt)
	if err == sql.ErrNoRows {
		return map[string]any{"draft": nil}, nil
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{"draft": d}, nil
}
