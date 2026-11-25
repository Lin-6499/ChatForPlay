package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"regexp"
)

type gameReplyOption struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Content string `json:"content"`
}
type gameReplyPrompt struct {
	TargetUserID int64             `json:"target_user_id"`
	Options      []gameReplyOption `json:"options"`
}
type gameReplyView struct {
	gameReplyPrompt
	EventID  int64  `json:"event_id"`
	Content  string `json:"content"`
	Answered bool   `json:"answered"`
	CanReply bool   `json:"can_reply"`
}

var replyOptionID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,40}$`)

func validateReplyPrompt(tx *sql.Tx, c campaign, prompt *gameReplyPrompt) error {
	if prompt == nil {
		return nil
	}
	if len(prompt.Options) < 1 || len(prompt.Options) > 6 || prompt.TargetUserID < 0 {
		return gameFail(400, "快捷回复需要 1–6 个选项和有效的回复者")
	}
	if prompt.TargetUserID != 0 {
		var count int
		if err := tx.QueryRow("SELECT COUNT(*) FROM members WHERE conversation_id=? AND user_id=? AND user_id<>?",
			c.ConversationID, prompt.TargetUserID, c.BotID).Scan(&count); err != nil {
			return err
		}
		if count != 1 {
			return gameFail(400, "指定回复者不属于此战役")
		}
	}
	seen := map[string]bool{}
	for _, option := range prompt.Options {
		if !replyOptionID.MatchString(option.ID) || seen[option.ID] || !textOK(option.Label, 1, 40) || !textOK(option.Content, 1, 4000) {
			return gameFail(400, "选项 ID 须唯一，按钮文字为 1–40 字，回复内容为 1–4000 字")
		}
		seen[option.ID] = true
	}
	return nil
}

type replyQuerier interface {
	QueryRow(string, ...any) *sql.Row
}

// Any newer public narration/summary replaces the prompt, even when it has no
// choices. Player actions, dice and whispers do not invalidate other players.
func currentReplyPrompt(q replyQuerier, cid int64) (*gameReplyView, error) {
	var id int64
	var content, raw string
	err := q.QueryRow(`SELECT id,content,payload FROM campaign_events WHERE campaign_id=? AND visibility='public'
		AND kind IN ('narration','summary') ORDER BY id DESC LIMIT 1`, cid).Scan(&id, &content, &raw)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var payload struct {
		Prompt *gameReplyPrompt `json:"reply_prompt"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, err
	}
	if payload.Prompt == nil {
		return nil, nil
	}
	return &gameReplyView{gameReplyPrompt: *payload.Prompt, EventID: id, Content: content}, nil
}

func (a *app) gameReplyPromptHTTP(r *http.Request, g gameActor) (any, error) {
	c, err := a.gameAccess(pathID(r, "id"), g)
	if err != nil {
		return nil, err
	}
	prompt, err := currentReplyPrompt(a.store, c.ID)
	if err != nil {
		return nil, err
	}
	if prompt != nil {
		var count int
		if err := a.store.QueryRow("SELECT COUNT(*) FROM campaign_replies WHERE campaign_id=? AND prompt_event_id=? AND user_id=?", c.ID, prompt.EventID, g.ID).Scan(&count); err != nil {
			return nil, err
		}
		prompt.Answered = count > 0
		prompt.CanReply = g.AgentID == 0 && g.actorKey != "builtin" && !prompt.Answered &&
			(prompt.TargetUserID == 0 || prompt.TargetUserID == g.ID) && c.Phase == "awaiting_action" && c.ActiveTurn == 0
	}
	return map[string]any{"prompt": prompt}, nil
}

func resolveGameReply(tx *sql.Tx, c campaign, g gameActor, eventID int64, optionID, content string) (string, error) {
	if eventID == 0 {
		if optionID != "" {
			return "", gameFail(400, "选择快捷回复时必须提供 reply_to_event_id")
		}
		return content, nil
	}
	prompt, err := currentReplyPrompt(tx, c.ID)
	if err != nil {
		return "", err
	}
	if prompt == nil || prompt.EventID != eventID || c.Phase != "awaiting_action" || c.ActiveTurn != 0 {
		return "", gameFail(409, "这组回复已失效或当前不能回复，请刷新战役")
	}
	if prompt.TargetUserID != 0 && prompt.TargetUserID != g.ID {
		return "", gameFail(403, "请等待指定玩家回复")
	}
	var count int
	if err := tx.QueryRow("SELECT COUNT(*) FROM campaign_replies WHERE campaign_id=? AND prompt_event_id=? AND user_id=?", c.ID, eventID, g.ID).Scan(&count); err != nil {
		return "", err
	}
	if count > 0 {
		return "", gameFail(409, "你已回复过这组问题")
	}
	if optionID == "" {
		return content, nil // Free text is always allowed.
	}
	if content != "" {
		return "", gameFail(400, "option_id 与自定义 content 不能同时提交")
	}
	for _, option := range prompt.Options {
		if option.ID == optionID {
			return option.Content, nil
		}
	}
	return "", gameFail(400, "快捷回复选项不存在")
}

func replyPromptSchema() any {
	return map[string]any{"type": "object", "properties": map[string]any{
		"target_user_id": map[string]any{"type": "integer", "minimum": 0, "description": "指定玩家 ID；0 表示每位成员均可回复"},
		"options": map[string]any{"type": "array", "minItems": 1, "maxItems": 6, "items": map[string]any{
			"type": "object", "properties": map[string]any{
				"id":      map[string]any{"type": "string", "pattern": "^[A-Za-z0-9_-]{1,40}$"},
				"label":   map[string]any{"type": "string", "minLength": 1, "maxLength": 40},
				"content": map[string]any{"type": "string", "minLength": 1, "maxLength": 4000},
			}, "required": []string{"id", "label", "content"}, "additionalProperties": false,
		}},
	}, "required": []string{"target_user_id", "options"}, "additionalProperties": false}
}
