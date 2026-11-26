package main

import (
	"database/sql"
	"net/http"
	"time"
)

const gameAttemptLimit = 12 * time.Minute

func gameToolBudget(actions int) int {
	return min(28, max(8, 2*actions+4))
}
func (a *app) heartbeatGameTurn(cid, tid int64, g gameActor, lease string) (any, error) {
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
	if err = checkGameLease(tx, c, g, tid, lease); err != nil {
		return nil, err
	}
	var deadline int64
	if err = tx.QueryRow("SELECT deadline_at FROM campaign_turn_runtime WHERE turn_id=?", tid).Scan(&deadline); err != nil {
		return nil, err
	}
	if _, err = tx.Exec("UPDATE campaign_turns SET started_at=? WHERE id=? AND lease=?", time.Now().UnixMilli(), tid, lease); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return map[string]any{"deadline_at": deadline, "lease_seconds": 180}, nil
}
func (a *app) gameHeartbeat(r *http.Request, g gameActor) (any, error) {
	var in struct {
		Lease string `json:"lease"`
	}
	if err := gameBody(r, &in); err != nil {
		return nil, err
	}
	return a.heartbeatGameTurn(pathID(r, "id"), pathID(r, "turn"), g, in.Lease)
}

// Legacy running turns without runtime metadata retain their original short
// lease. New claims get an absolute attempt deadline; heartbeat cannot extend it.
func checkAttemptDeadline(tx *sql.Tx, tid int64) error {
	expired, err := gameAttemptExpired(tx, tid)
	if err != nil {
		return err
	}
	if expired {
		return gameFail(409, "本次主持达到12分钟执行上限，请房主检查后恢复或接管")
	}
	return nil
}
func gameAttemptExpired(tx *sql.Tx, tid int64) (bool, error) {
	var deadline int64
	err := tx.QueryRow("SELECT deadline_at FROM campaign_turn_runtime WHERE turn_id=?", tid).Scan(&deadline)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return time.Now().UnixMilli() >= deadline, nil
}
