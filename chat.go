package main

import (
	"database/sql"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type conversation struct {
	ID             int64  `json:"id"`
	Kind           string `json:"kind"`
	Name           string `json:"name"`
	OwnerID        int64  `json:"owner_id"`
	MemberCount    int    `json:"member_count"`
	LastID         int64  `json:"last_id"`
	LastAt         int64  `json:"last_at"`
	Preview        string `json:"preview"`
	Unread         int    `json:"unread"`
	LastRead       int64  `json:"last_read"`
	UnreadMentions int    `json:"unread_mentions"`
}

func (a *app) conversationID(w http.ResponseWriter, r *http.Request, u user) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		fail(w, 400, "会话编号无效")
		return 0, false
	}
	if !a.store.isMember(id, u.ID) {
		fail(w, 403, "你不是该会话的成员")
		return 0, false
	}
	return id, true
}

func (a *app) conversations(w http.ResponseWriter, r *http.Request, u user) {
	rows, err := a.store.Query(`
	SELECT c.id,c.kind,
	  CASE WHEN c.kind='direct' THEN COALESCE((SELECT u.nickname FROM users u JOIN members mm
	    ON mm.user_id=u.id WHERE mm.conversation_id=c.id AND u.id<>?),c.name) ELSE c.name END,
	  c.owner_id,
	  (SELECT COUNT(*) FROM members mm WHERE mm.conversation_id=c.id),
	  COALESCE((SELECT MAX(id) FROM messages WHERE conversation_id=c.id),0),
	  COALESCE((SELECT created_at FROM messages WHERE conversation_id=c.id ORDER BY id DESC LIMIT 1),c.created_at),
	  COALESCE((SELECT CASE WHEN kind='text' THEN content ELSE '[附件]' END FROM messages
	    WHERE conversation_id=c.id ORDER BY id DESC LIMIT 1),'开始你们的对话'),
	  (SELECT COUNT(*) FROM messages WHERE conversation_id=c.id AND id>mm.last_read AND sender_id<>?),
	  mm.last_read,
	  (SELECT COUNT(*) FROM messages msg WHERE msg.conversation_id=c.id AND msg.id>mm.last_read
	    AND msg.sender_id<>? AND EXISTS(SELECT 1 FROM message_mentions mt WHERE mt.message_id=msg.id AND mt.user_id=?))
	FROM conversations c JOIN members mm ON mm.conversation_id=c.id WHERE mm.user_id=?
	ORDER BY 7 DESC,c.id DESC`, u.ID, u.ID, u.ID, u.ID, u.ID)
	if err != nil {
		fail(w, 500, "读取会话失败")
		return
	}
	defer rows.Close()
	out := []conversation{}
	for rows.Next() {
		var c conversation
		if rows.Scan(&c.ID, &c.Kind, &c.Name, &c.OwnerID, &c.MemberCount, &c.LastID, &c.LastAt, &c.Preview, &c.Unread, &c.LastRead, &c.UnreadMentions) != nil {
			fail(w, 500, "读取会话失败")
			return
		}
		out = append(out, c)
	}
	writeJSON(w, 200, out)
}

func (a *app) createConversation(w http.ResponseWriter, r *http.Request, u user) {
	if !a.limiter.allow("create-conv:"+u.Username, 20, time.Minute) {
		fail(w, 429, "创建会话过于频繁")
		return
	}
	var in struct {
		Kind    string  `json:"kind"`
		Name    string  `json:"name"`
		Members []int64 `json:"members"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Kind != "direct" && in.Kind != "group" {
		fail(w, 400, "会话类型无效")
		return
	}
	ids := []int64{u.ID}
	seen := map[int64]bool{u.ID: true}
	for _, id := range in.Members {
		if id > 0 && !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	if len(ids) < 2 || len(ids) > 100 || (in.Kind == "direct" && len(ids) != 2) {
		fail(w, 400, "请选择成员：私聊 2 人，群聊 2–100 人")
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Kind == "group" && (in.Name == "" || utf8.RuneCountInString(in.Name) > 40) {
		fail(w, 400, "群名称需为 1–40 个字符")
		return
	}
	var key any
	if in.Kind == "direct" {
		key = directKey(ids[0], ids[1])
	}
	tx, err := a.store.Begin()
	if err != nil {
		fail(w, 500, "创建会话失败")
		return
	}
	defer tx.Rollback()
	if key != nil {
		var existing int64
		if tx.QueryRow("SELECT id FROM conversations WHERE direct_key=?", key).Scan(&existing) == nil {
			writeJSON(w, 200, map[string]any{"id": existing})
			return
		}
	}
	res, err := tx.Exec("INSERT INTO conversations(kind,name,owner_id,direct_key,created_at) VALUES(?,?,?,?,?)",
		in.Kind, in.Name, u.ID, key, time.Now().UnixMilli())
	if err != nil {
		fail(w, 500, "创建会话失败")
		return
	}
	cid, _ := res.LastInsertId()
	for _, id := range ids {
		if _, err = tx.Exec("INSERT INTO members(conversation_id,user_id) VALUES(?,?)", cid, id); err != nil {
			fail(w, 400, "部分成员不存在")
			return
		}
	}
	if err = tx.Commit(); err != nil {
		fail(w, 500, "保存会话失败")
		return
	}
	a.hub.broadcast(ids, map[string]any{"type": "conversations.changed"})
	writeJSON(w, 201, map[string]any{"id": cid})
}

func (a *app) conversationDetail(w http.ResponseWriter, r *http.Request, u user) {
	cid, ok := a.conversationID(w, r, u)
	if !ok {
		return
	}
	type member struct {
		user
		LastRead int64 `json:"last_read"`
	}
	rows, err := a.store.Query(`SELECT u.id,u.username,u.nickname,u.admin,m.last_read FROM members m
		JOIN users u ON u.id=m.user_id WHERE m.conversation_id=? ORDER BY u.nickname`, cid)
	if err != nil {
		fail(w, 500, "读取成员失败")
		return
	}
	defer rows.Close()
	members := []member{}
	for rows.Next() {
		var item member
		if rows.Scan(&item.ID, &item.Username, &item.Nickname, &item.Admin, &item.LastRead) != nil {
			fail(w, 500, "读取成员失败")
			return
		}
		item.Online = a.hub.online(item.ID)
		members = append(members, item)
	}
	writeJSON(w, 200, map[string]any{"members": members})
}

func (a *app) addMembers(w http.ResponseWriter, r *http.Request, u user) {
	cid, ok := a.conversationID(w, r, u)
	if !ok {
		return
	}
	var owner int64
	var kind string
	if a.store.QueryRow("SELECT owner_id,kind FROM conversations WHERE id=?", cid).Scan(&owner, &kind) != nil {
		fail(w, 404, "会话不存在")
		return
	}
	if kind != "group" || (owner != u.ID && !u.Admin) {
		fail(w, 403, "只有群主或管理员能添加群成员")
		return
	}
	var in struct {
		Members []int64 `json:"members"`
	}
	if !decode(w, r, &in) {
		return
	}
	if len(in.Members) == 0 || len(in.Members) > 100 {
		fail(w, 400, "请选择要添加的成员")
		return
	}
	tx, err := a.store.Begin()
	if err != nil {
		fail(w, 500, "添加成员失败")
		return
	}
	defer tx.Rollback()
	for _, id := range in.Members {
		if _, err = tx.Exec("INSERT INTO members(conversation_id,user_id) VALUES(?,?) ON CONFLICT DO NOTHING", cid, id); err != nil {
			fail(w, 400, "成员不存在")
			return
		}
	}
	var count int
	if tx.QueryRow("SELECT COUNT(*) FROM members WHERE conversation_id=?", cid).Scan(&count) != nil || count > 100 {
		fail(w, 400, "每个群最多 100 人")
		return
	}
	if err := tx.Commit(); err != nil {
		fail(w, 500, "保存成员失败")
		return
	}
	a.hub.broadcast(a.store.memberIDs(cid), map[string]any{"type": "conversations.changed"})
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (a *app) messages(w http.ResponseWriter, r *http.Request, u user) {
	cid, ok := a.conversationID(w, r, u)
	if !ok {
		return
	}
	query := messageSelect + " WHERE m.conversation_id=?"
	args := []any{cid}
	ascending := false
	for _, key := range []string{"before", "after"} {
		if raw := r.URL.Query().Get(key); raw != "" {
			v, err := strconv.ParseInt(raw, 10, 64)
			if err != nil || v < 0 {
				fail(w, 400, "消息游标无效")
				return
			}
			if key == "before" {
				query += " AND m.id<?"
			} else {
				query += " AND m.id>?"
				ascending = true
			}
			args = append(args, v)
		}
	}
	if ascending {
		query += " ORDER BY m.id ASC LIMIT 101"
	} else {
		query += " ORDER BY m.id DESC LIMIT 101"
	}
	rows, err := a.store.Query(query, args...)
	if err != nil {
		fail(w, 500, "读取消息失败")
		return
	}
	defer rows.Close()
	items := []message{}
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			fail(w, 500, "读取消息失败")
			return
		}
		items = append(items, m)
	}
	more := len(items) > 100
	if more {
		items = items[:100]
	}
	if !ascending {
		for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
			items[i], items[j] = items[j], items[i]
		}
	}
	writeJSON(w, 200, map[string]any{"messages": items, "more": more})
}

func (a *app) sendMessage(w http.ResponseWriter, r *http.Request, u user) {
	cid, ok := a.conversationID(w, r, u)
	if !ok {
		return
	}
	var in struct {
		ClientID string    `json:"client_id"`
		Content  string    `json:"content"`
		Mentions []mention `json:"mentions"`
	}
	if !decode(w, r, &in) {
		return
	}
	leading := utf8.RuneCountInString(in.Content) - utf8.RuneCountInString(strings.TrimLeftFunc(in.Content, unicode.IsSpace))
	in.Content = strings.TrimSpace(in.Content)
	if !idPattern.MatchString(in.ClientID) || in.Content == "" || utf8.RuneCountInString(in.Content) > 4000 {
		fail(w, 400, "消息需为 1–4000 个字符，并携带有效的消息编号")
		return
	}
	if len(in.Mentions) > 100 {
		fail(w, 400, "每条消息最多包含 100 处 @成员")
		return
	}
	for i := range in.Mentions {
		in.Mentions[i].Start -= leading
		in.Mentions[i].End -= leading
	}
	slices.SortFunc(in.Mentions, func(a, b mention) int {
		if a.Start < b.Start {
			return -1
		}
		if a.Start > b.Start {
			return 1
		}
		return 0
	})
	runes := []rune(in.Content)
	end := 0
	for _, m := range in.Mentions {
		if m.UserID <= 0 || m.Start < end || m.End <= m.Start || m.End > len(runes) || runes[m.Start] != '@' {
			fail(w, 400, "@成员标记无效，请重新选择成员")
			return
		}
		end = m.End
	}
	if !a.limiter.allow("message:"+u.Username, 120, time.Minute) {
		fail(w, 429, "发送太快，请稍后重试")
		return
	}
	tx, err := a.store.Begin()
	if err != nil {
		fail(w, 500, "保存消息失败")
		return
	}
	defer tx.Rollback()
	existing, err := scanMessage(tx.QueryRow(messageSelect+" WHERE m.sender_id=? AND m.client_id=?", u.ID, in.ClientID))
	if err == nil {
		_ = tx.Rollback()
		if existing.ConversationID != cid || existing.Content != in.Content || existing.Kind != "text" || !slices.Equal(existing.Mentions, in.Mentions) {
			fail(w, 409, "消息编号已经用于其他内容")
			return
		}
		writeJSON(w, 200, existing)
		return
	}
	if !errors.Is(err, sql.ErrNoRows) {
		fail(w, 500, "读取消息失败")
		return
	}
	if len(in.Mentions) > 0 {
		var kind string
		if tx.QueryRow("SELECT kind FROM conversations WHERE id=?", cid).Scan(&kind) != nil {
			fail(w, 500, "读取会话失败")
			return
		}
		if kind != "group" && kind != "lobby" {
			fail(w, 400, "只有群聊和公共大厅支持 @成员")
			return
		}
		for _, m := range in.Mentions {
			var nickname, username string
			err := tx.QueryRow(`SELECT u.nickname,u.username FROM users u JOIN members mm ON mm.user_id=u.id
				WHERE mm.conversation_id=? AND u.id=?`, cid, m.UserID).Scan(&nickname, &username)
			if errors.Is(err, sql.ErrNoRows) {
				fail(w, 400, "只能 @ 当前会话里的成员")
				return
			}
			if err != nil {
				fail(w, 500, "读取成员失败")
				return
			}
			label := string(runes[m.Start:m.End])
			if label != "@"+nickname && label != "@"+username {
				fail(w, 400, "被提及成员的昵称已变化，请重新选择")
				return
			}
		}
	}
	res, err := tx.Exec("INSERT INTO messages(conversation_id,sender_id,client_id,kind,content,created_at) VALUES(?,?,?,'text',?,?)",
		cid, u.ID, in.ClientID, in.Content, time.Now().UnixMilli())
	if err != nil {
		fail(w, 500, "保存消息失败")
		return
	}
	mid, _ := res.LastInsertId()
	for _, m := range in.Mentions {
		if _, err = tx.Exec("INSERT INTO message_mentions(message_id,user_id,start_offset,end_offset) VALUES(?,?,?,?)",
			mid, m.UserID, m.Start, m.End); err != nil {
			fail(w, 500, "保存 @成员失败")
			return
		}
	}
	if err = tx.Commit(); err != nil {
		fail(w, 500, "保存消息失败")
		return
	}
	m, err := a.store.getMessage(mid)
	if err != nil {
		fail(w, 500, "读取消息失败")
		return
	}
	a.publishMessage(m)
	writeJSON(w, 201, m)
}

func (a *app) publishMessage(m message) {
	a.hub.broadcast(a.store.memberIDs(m.ConversationID), map[string]any{"type": "message", "message": m})
}

func (a *app) markRead(w http.ResponseWriter, r *http.Request, u user) {
	cid, ok := a.conversationID(w, r, u)
	if !ok {
		return
	}
	var in struct {
		MessageID int64 `json:"message_id"`
	}
	if !decode(w, r, &in) {
		return
	}
	var n int
	if a.store.QueryRow("SELECT 1 FROM messages WHERE id=? AND conversation_id=?", in.MessageID, cid).Scan(&n) != nil {
		fail(w, 400, "消息不存在")
		return
	}
	res, err := a.store.Exec("UPDATE members SET last_read=? WHERE conversation_id=? AND user_id=? AND last_read<?",
		in.MessageID, cid, u.ID, in.MessageID)
	if err != nil {
		fail(w, 500, "保存已读状态失败")
		return
	}
	changed, _ := res.RowsAffected()
	if changed > 0 {
		a.hub.broadcast(a.store.memberIDs(cid), map[string]any{"type": "read", "conversation_id": cid,
			"user_id": u.ID, "last_read": in.MessageID})
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}
