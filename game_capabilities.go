package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"
)

type gameExecer interface {
	Exec(string, ...any) (sql.Result, error)
}

func touchGameHost(db gameExecer, cid int64, field string) error {
	switch field {
	case "contact_at", "claim_at", "stream_at", "complete_at", "fail_at":
	default:
		return fmt.Errorf("invalid activity field")
	}
	_, err := db.Exec("INSERT INTO campaign_host_activity(campaign_id,"+field+") VALUES(?,?) ON CONFLICT(campaign_id) DO UPDATE SET "+field+"=excluded."+field, cid, time.Now().UnixMilli())
	return err
}
func (a *app) gameCapabilities(r *http.Request, g gameActor) (any, error) {
	c, err := a.gameAccess(pathID(r, "id"), g)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"server_version": serverVersion, "protocol": map[string]any{"min": 1, "max": 1}, "campaign_id": c.ID,
		"features":   []string{"atomic_complete", "stream_snapshots", "reply_choices", "d20_graded_v1", "lore", "memory_suggestions", "recap", "group_actions", "action_supplements"},
		"check_rule": checkRule, "mode": c.Mode,
		"limits": map[string]any{"lease_seconds": 180, "max_group_participants": 12, "max_supplements": 5, "context_characters": 24000, "max_memory_suggestions": 5},
		"docs": map[string]string{
			"entry":     "https://github.com/Lin-6499/ChatForPlay/blob/main/docs/HOST_INTEGRATION.md",
			"api":       "https://github.com/Lin-6499/ChatForPlay/blob/main/docs/CAMPAIGN_API.md",
			"checks":    "https://github.com/Lin-6499/ChatForPlay/blob/main/docs/ABILITY_CHECKS.md",
			"streaming": "https://github.com/Lin-6499/ChatForPlay/blob/main/docs/HOST_STREAMING.md",
			"memory":    "https://github.com/Lin-6499/ChatForPlay/blob/main/docs/MEMORY_PROGRESS.md",
		},
	}, nil
}
func (a *app) gameHostDiagnostics(r *http.Request, g gameActor) (any, error) {
	c, err := a.gameAccess(pathID(r, "id"), g)
	if err != nil {
		return nil, err
	}
	if !g.owner(c) {
		return nil, gameFail(403, "接入诊断仅供真人房主查看")
	}
	var contact, claim, stream, complete, failed int64
	err = a.store.QueryRow("SELECT contact_at,claim_at,stream_at,complete_at,fail_at FROM campaign_host_activity WHERE campaign_id=?", c.ID).Scan(&contact, &claim, &stream, &complete, &failed)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	settings, err := a.readGameAI()
	if err != nil {
		return nil, err
	}
	return map[string]any{"server_version": serverVersion, "mode": c.Mode, "phase": c.Phase,
		"builtin_ready": settings.Enabled && settings.HasKey, "contact_at": contact, "claim_at": claim, "stream_at": stream, "complete_at": complete, "fail_at": failed,
		"notice": "最近联系不等于机器人当前在线；仅支持本站API能力检查，不能替外部客户端更新工具或文档。"}, nil
}
