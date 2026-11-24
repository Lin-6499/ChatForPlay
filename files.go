package main

import (
	"database/sql"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

func (a *app) uploadFile(w http.ResponseWriter, r *http.Request, u user) {
	cid, ok := a.conversationID(w, r, u)
	if !ok {
		return
	}
	if !a.limiter.allow("upload:"+u.Username, 20, time.Minute) {
		fail(w, 429, "上传过于频繁，请稍后重试")
		return
	}
	clientID := r.URL.Query().Get("client_id")
	if !idPattern.MatchString(clientID) {
		fail(w, 400, "文件消息编号无效")
		return
	}
	// Serialize uploads so concurrent streams cannot overrun the configured quota.
	a.uploadMu.Lock()
	defer a.uploadMu.Unlock()
	var existing, existingCID int64
	var existingKind string
	err := a.store.QueryRow("SELECT id,conversation_id,kind FROM messages WHERE sender_id=? AND client_id=?",
		u.ID, clientID).Scan(&existing, &existingCID, &existingKind)
	if err == nil {
		if existingCID != cid || existingKind == "text" {
			fail(w, 409, "消息编号已经用于其他内容")
			return
		}
		m, e := a.store.getMessage(existing)
		if e != nil {
			fail(w, 500, "读取文件消息失败")
			return
		}
		writeJSON(w, 200, m)
		return
	}
	if err != sql.ErrNoRows {
		fail(w, 500, "读取文件记录失败")
		return
	}
	var used int64
	if a.store.QueryRow("SELECT COALESCE(SUM(size),0) FROM files").Scan(&used) != nil {
		fail(w, 500, "读取存储空间失败")
		return
	}
	if used >= a.cfg.FileQuota {
		fail(w, 507, "文件存储空间已满，请联系管理员")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, a.cfg.MaxUpload+(1<<20))
	reader, err := r.MultipartReader()
	if err != nil {
		fail(w, 400, "请使用文件上传表单")
		return
	}
	part, err := reader.NextPart()
	if err != nil || part.FormName() != "file" || part.FileName() == "" {
		fail(w, 400, "未选择文件")
		return
	}
	defer part.Close()
	name := filepath.Base(strings.ReplaceAll(part.FileName(), "\\", "/"))
	name = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, name)
	if name == "" || name == "." || utf8.RuneCountInString(name) > 180 {
		fail(w, 400, "文件名无效或过长")
		return
	}
	id := randomToken(24)
	temp, err := os.CreateTemp(filepath.Join(a.cfg.DataDir, "files"), ".upload-")
	if err != nil {
		fail(w, 500, "无法创建文件")
		return
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	limit := a.cfg.MaxUpload
	if a.cfg.FileQuota-used < limit {
		limit = a.cfg.FileQuota - used
	}
	size, err := io.Copy(temp, io.LimitReader(part, limit+1))
	if closeErr := temp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		fail(w, 400, "上传未完成，请重试")
		return
	}
	if size > limit {
		fail(w, 413, "文件超出上传大小或剩余存储空间限制")
		return
	}
	f, err := os.Open(tempName)
	if err != nil {
		fail(w, 500, "文件读取失败")
		return
	}
	header := make([]byte, 512)
	n, _ := f.Read(header)
	_ = f.Close()
	contentType := http.DetectContentType(header[:n])
	kind := "file"
	if isImage(contentType) {
		kind = "image"
	}
	final := filepath.Join(a.cfg.DataDir, "files", id)
	if err = os.Rename(tempName, final); err != nil {
		fail(w, 500, "保存文件失败")
		return
	}
	saved := false
	defer func() {
		if !saved {
			_ = os.Remove(final)
		}
	}()
	tx, err := a.store.Begin()
	if err != nil {
		fail(w, 500, "保存文件记录失败")
		return
	}
	defer tx.Rollback()
	now := time.Now().UnixMilli()
	_, err = tx.Exec("INSERT INTO files(id,conversation_id,uploaded_by,name,size,mime,created_at) VALUES(?,?,?,?,?,?,?)",
		id, cid, u.ID, name, size, contentType, now)
	if err != nil {
		fail(w, 500, "保存文件记录失败")
		return
	}
	res, err := tx.Exec("INSERT INTO messages(conversation_id,sender_id,client_id,kind,content,file_id,created_at) VALUES(?,?,?,?,?,?,?)",
		cid, u.ID, clientID, kind, "", id, now)
	if err != nil {
		fail(w, 500, "保存文件消息失败")
		return
	}
	mid, _ := res.LastInsertId()
	if err = tx.Commit(); err != nil {
		fail(w, 500, "保存文件消息失败")
		return
	}
	saved = true
	m, err := a.store.getMessage(mid)
	if err != nil {
		fail(w, 500, "读取文件消息失败")
		return
	}
	a.publishMessage(m)
	writeJSON(w, 201, m)
}

func isImage(mime string) bool {
	switch mime {
	case "image/png", "image/jpeg", "image/gif", "image/webp":
		return true
	default:
		return false
	}
}

func (a *app) downloadFile(w http.ResponseWriter, r *http.Request, u user) {
	id := r.PathValue("id")
	if !idPattern.MatchString(id) {
		fail(w, 404, "文件不存在")
		return
	}
	var cid int64
	var name, mimeType string
	if a.store.QueryRow("SELECT conversation_id,name,mime FROM files WHERE id=?", id).Scan(&cid, &name, &mimeType) != nil {
		fail(w, 404, "文件不存在")
		return
	}
	if !a.store.isMember(cid, u.ID) {
		fail(w, 403, "你没有权限访问这个文件")
		return
	}
	f, err := os.Open(filepath.Join(a.cfg.DataDir, "files", id))
	if err != nil {
		fail(w, 404, "文件不存在")
		return
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		fail(w, 500, "读取文件失败")
		return
	}
	disposition := "attachment"
	if isImage(mimeType) && r.URL.Query().Get("download") != "1" {
		disposition = "inline"
	}
	w.Header().Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": name}))
	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	http.ServeContent(w, r, name, stat.ModTime(), f)
}
