package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"time"
	"unicode/utf8"
)

func normalizeLoreProgress(l *gameLore) error {
	switch l.Kind {
	case "quest":
		if l.Progress == "" {
			l.Progress = "not_started"
		}
		if l.Progress != "not_started" && l.Progress != "in_progress" && l.Progress != "completed" && l.Progress != "failed" {
			return gameFail(400, "任务进度无效")
		}
	case "clue":
		if l.Progress == "" {
			l.Progress = "undiscovered"
		}
		if l.Progress != "undiscovered" && l.Progress != "discovered" && l.Progress != "resolved" {
			return gameFail(400, "线索进度无效")
		}
	default:
		l.Progress = ""
	}
	if l.ChapterID < 0 || (l.Kind == "chapter" && l.ChapterID != 0) {
		return gameFail(400, "章节关联无效")
	}
	return nil
}
func validateLoreChapter(tx *sql.Tx, c campaign, l gameLore) error {
	if l.ChapterID == 0 {
		return nil
	}
	var kind, visibility string
	err := tx.QueryRow("SELECT kind,visibility FROM campaign_lore WHERE id=? AND campaign_id=?", l.ChapterID, c.ID).Scan(&kind, &visibility)
	if err != nil || kind != "chapter" || (l.Visibility == "public" && visibility != "public") {
		return gameFail(400, "公开资料只能关联本战役公开章节，秘密资料可关联本战役任一章节")
	}
	return nil
}
func (a *app) gameLoreFocus(r *http.Request, g gameActor) (any, error) {
	var in struct {
		gameOp
		ChapterID int64 `json:"chapter_id"`
	}
	if err := gameBody(r, &in); err != nil {
		return nil, err
	}
	return a.gameMutation(pathID(r, "id"), g, in.gameOp, "lore-focus", in, func(tx *sql.Tx, c campaign) (any, error) {
		if !g.owner(c) {
			return nil, gameFail(403, "只有房主可选择本幕资料")
		}
		if err := gameVersion(c, in.Version); err != nil {
			return nil, err
		}
		if c.ActiveTurn != 0 {
			return nil, gameFail(409, "请等待主持回合完成后再切换本幕资料")
		}
		chapter, err := scanLore(tx.QueryRow(loreSelect+" WHERE id=? AND campaign_id=?", in.ChapterID, c.ID))
		if err != nil || chapter.Kind != "chapter" || chapter.Visibility != "public" || chapter.Status != "active" {
			return nil, gameFail(400, "请选择本战役有效的公开章节")
		}
		rows, err := tx.Query(loreSelect+" WHERE campaign_id=? AND visibility='public'", c.ID)
		if err != nil {
			return nil, err
		}
		entries := []gameLore{}
		for rows.Next() {
			l, e := scanLore(rows)
			if e != nil {
				rows.Close()
				return nil, e
			}
			entries = append(entries, l)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
		budget, count := 0, 0
		for _, l := range entries {
			if l.Status == "active" && (l.ID == chapter.ID || l.ChapterID == chapter.ID) {
				budget += utf8.RuneCountInString(l.Content) + utf8.RuneCountInString(l.Title)
				count++
			}
		}
		if budget > 24000 {
			return nil, gameFail(400, "本幕资料超过24000字，请拆分章节或逐条选择参考资料；尚未更改原选择")
		}
		now := time.Now().UnixMilli()
		for _, l := range entries {
			include := l.Status == "active" && (l.ID == chapter.ID || l.ChapterID == chapter.ID)
			if l.InContext == include {
				continue
			}
			l.InContext = include
			l.Version++
			l.UpdatedAt = now
			if _, err = tx.Exec("UPDATE campaign_lore SET in_context=?,version=?,updated_at=? WHERE id=?", include, l.Version, now, l.ID); err != nil {
				return nil, err
			}
			if _, err = tx.Exec("INSERT INTO campaign_lore_history(entry_id,version,actor_id,document,created_at) VALUES(?,?,?,?,?)", l.ID, l.Version, g.ID, gameJSON(l), now); err != nil {
				return nil, err
			}
		}
		if _, err = tx.Exec("UPDATE campaigns SET version=version+1,updated_at=? WHERE id=?", now, c.ID); err != nil {
			return nil, err
		}
		return map[string]any{"chapter_id": chapter.ID, "included": count, "version": c.Version + 1}, nil
	})
}

type memorySuggestionInput struct {
	Kind    string `json:"kind"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

func memorySuggestionSchema() any {
	return map[string]any{"type": "array", "maxItems": 5, "description": "可选：仅根据本回合正式叙述提出待房主确认的记忆，不会直接改变既定事实；不含推测或秘密。无重要变化时省略。", "items": map[string]any{"type": "object", "properties": map[string]any{
		"kind":    map[string]any{"type": "string", "enum": []string{"fact", "npc", "quest", "clue"}},
		"title":   map[string]any{"type": "string", "maxLength": 100},
		"content": map[string]any{"type": "string", "maxLength": 2000},
	}, "required": []string{"kind", "title", "content"}, "additionalProperties": false}}
}
func saveMemorySuggestions(tx *sql.Tx, c campaign, tid, eid int64, suggestions []memorySuggestionInput) error {
	if len(suggestions) > 5 {
		return gameFail(400, "每回合最多5条记忆建议")
	}
	for _, s := range suggestions {
		if (s.Kind != "fact" && s.Kind != "npc" && s.Kind != "quest" && s.Kind != "clue") || !textOK(s.Title, 1, 100) || !textOK(s.Content, 1, 2000) {
			return gameFail(400, "记忆建议分类、标题或正文无效")
		}
		if _, err := tx.Exec(`INSERT INTO campaign_memory_suggestions(campaign_id,turn_id,event_id,document,created_at) VALUES(?,?,?,?,?)`, c.ID, tid, eid, gameJSON(s), time.Now().UnixMilli()); err != nil {
			return err
		}
	}
	return nil
}
func (a *app) gameMemorySuggestions(r *http.Request, g gameActor) (any, error) {
	c, err := a.gameAccess(pathID(r, "id"), g)
	if err != nil {
		return nil, err
	}
	if !g.owner(c) {
		return nil, gameFail(403, "记忆建议仅供房主确认")
	}
	paged := r.URL.Query().Get("paged") == "1"
	status := r.URL.Query().Get("status")
	if status == "" {
		status = "all"
	}
	if status != "all" && status != "pending" && status != "accepted" && status != "rejected" {
		return nil, gameFail(400, "建议筛选状态无效")
	}
	before := int64(0)
	if raw := r.URL.Query().Get("before"); raw != "" {
		before, err = strconv.ParseInt(raw, 10, 64)
		if err != nil || before < 1 {
			return nil, gameFail(400, "分页游标无效")
		}
	}
	limit := 100
	if paged {
		limit = 20
	}
	query := `SELECT s.id,s.turn_id,s.event_id,s.document,s.status,s.entry_id,e.content FROM campaign_memory_suggestions s
	  JOIN campaign_events e ON e.id=s.event_id AND e.campaign_id=s.campaign_id WHERE s.campaign_id=?`
	args := []any{c.ID}
	if status != "all" {
		query += " AND s.status=?"
		args = append(args, status)
	}
	if before > 0 {
		query += " AND s.id<?"
		args = append(args, before)
	}
	if !paged && status == "all" {
		query += " ORDER BY CASE WHEN s.status='pending' THEN 0 ELSE 1 END,s.id DESC"
	} else {
		query += " ORDER BY s.id DESC"
	}
	query += " LIMIT ?"
	args = append(args, limit+1)
	rows, err := a.store.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, tid, eid, entry int64
		var document, status, source string
		if err := rows.Scan(&id, &tid, &eid, &document, &status, &entry, &source); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"id": id, "turn_id": tid, "event_id": eid, "suggestion": json.RawMessage(document), "status": status, "entry_id": entry, "source_content": source})
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	hasMore := len(out) > limit
	if hasMore {
		out = out[:limit]
	}
	if !paged {
		return out, nil
	}
	var pending int
	if err = a.store.QueryRow("SELECT count(*) FROM campaign_memory_suggestions WHERE campaign_id=? AND status='pending'", c.ID).Scan(&pending); err != nil {
		return nil, err
	}
	var next int64
	if hasMore && len(out) > 0 {
		next = out[len(out)-1]["id"].(int64)
	}
	return map[string]any{"items": out, "total_pending": pending, "has_more": hasMore, "next_before": next, "status": status}, nil
}
func (a *app) gameRejectMemory(r *http.Request, g gameActor) (any, error) {
	var in struct{ gameOp }
	if err := gameBody(r, &in); err != nil {
		return nil, err
	}
	fingerprint := struct {
		ID    int64
		Input any
	}{pathID(r, "suggestion"), in}
	return a.gameMutation(pathID(r, "id"), g, in.gameOp, "memory-reject", fingerprint, func(tx *sql.Tx, c campaign) (any, error) {
		if !g.owner(c) {
			return nil, gameFail(403, "仅房主可处理记忆建议")
		}
		result, err := tx.Exec("UPDATE campaign_memory_suggestions SET status='rejected' WHERE id=? AND campaign_id=? AND status='pending'", pathID(r, "suggestion"), c.ID)
		if err != nil {
			return nil, err
		}
		n, _ := result.RowsAffected()
		if n != 1 {
			return nil, gameFail(409, "建议不存在或已处理")
		}
		return map[string]any{"status": "rejected"}, nil
	})
}
func (a *app) gameRecap(r *http.Request, g gameActor) (any, error) {
	c, err := a.gameAccess(pathID(r, "id"), g)
	if err != nil {
		return nil, err
	}
	entries, err := a.loreEntries(c, false)
	if err != nil {
		return nil, err
	}
	objectives := []gameLore{}
	facts := []gameLore{}
	for _, l := range entries {
		if l.Status != "active" {
			continue
		}
		if l.Kind == "quest" && l.Progress != "completed" && l.Progress != "failed" || l.Kind == "clue" && l.Progress != "resolved" {
			runes := []rune(l.Content)
			l.Content = string(runes[:min(400, len(runes))])
			objectives = append(objectives, l)
		}
		if l.Kind == "fact" && l.InContext && len(facts) < 10 {
			runes := []rune(l.Content)
			l.Content = string(runes[:min(400, len(runes))])
			facts = append(facts, l)
		}
	}
	rows, err := a.store.Query("SELECT id,content,created_at FROM campaign_events WHERE campaign_id=? AND visibility='public' AND kind='narration' ORDER BY id DESC LIMIT 3", c.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	recent := []map[string]any{}
	for rows.Next() {
		var id, at int64
		var text string
		if err := rows.Scan(&id, &text, &at); err != nil {
			return nil, err
		}
		runes := []rune(text)
		recent = append(recent, map[string]any{"event_id": id, "content": string(runes[:min(2000, len(runes))]), "created_at": at, "excerpt": len(runes) > 2000})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return map[string]any{"version": c.Version, "summary": c.Summary, "scene": c.Scene, "phase": c.Phase, "objectives": objectives[:min(20, len(objectives))], "objectives_omitted": max(0, len(objectives)-20), "facts": facts, "recent": recent, "policy": "依据已保存公开资料整理，不调用模型，也不产生新事实。"}, nil
}
