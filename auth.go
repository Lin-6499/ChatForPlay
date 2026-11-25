package main

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9_]{3,32}$`)
var idPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{16,80}$`)

func randomToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func tokenHash(t string) string {
	s := sha256.Sum256([]byte(t))
	return hex.EncodeToString(s[:])
}

type credentials struct {
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Password string `json:"password"`
	Token    string `json:"token"`
}

func validateCredentials(c credentials) string {
	if !usernamePattern.MatchString(c.Username) {
		return "账号需为 3–32 位字母、数字或下划线"
	}
	if utf8.RuneCountInString(strings.TrimSpace(c.Nickname)) < 1 || utf8.RuneCountInString(c.Nickname) > 30 {
		return "昵称需为 1–30 个字符"
	}
	if len(c.Password) < 10 || len(c.Password) > 72 {
		return "密码需为 10–72 字节"
	}
	return ""
}

func (a *app) status(w http.ResponseWriter, r *http.Request) {
	var n int
	if err := a.store.QueryRow("SELECT COUNT(*) FROM users").Scan(&n); err != nil {
		fail(w, 500, "读取状态失败")
		return
	}
	writeJSON(w, 200, map[string]any{"name": a.cfg.Name, "initialized": n > 0,
		"max_upload": a.cfg.MaxUpload, "registration": "invite", "version": serverVersion})
}

func (a *app) setup(w http.ResponseWriter, r *http.Request) {
	if !a.limiter.allow("setup:"+clientIP(r), 6, time.Minute) {
		fail(w, 429, "操作过于频繁，请稍后重试")
		return
	}
	var c credentials
	if !decode(w, r, &c) {
		return
	}
	a.setupMu.Lock()
	defer a.setupMu.Unlock()
	if a.setupToken == "" {
		fail(w, 409, "服务器已经完成初始化")
		return
	}
	if subtle.ConstantTimeCompare([]byte(c.Token), []byte(a.setupToken)) != 1 {
		fail(w, 403, "初始化令牌无效，请使用服务端提供的首次初始化地址")
		return
	}
	if msg := validateCredentials(c); msg != "" {
		fail(w, 400, msg)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(c.Password), bcrypt.DefaultCost)
	if err != nil {
		fail(w, 500, "创建账号失败")
		return
	}
	tx, err := a.store.Begin()
	if err != nil {
		fail(w, 500, "数据库暂时不可用")
		return
	}
	defer tx.Rollback()
	now := time.Now().UnixMilli()
	res, err := tx.Exec("INSERT INTO users(username,nickname,password,admin,created_at) VALUES(?,?,?,?,?)",
		c.Username, strings.TrimSpace(c.Nickname), string(hash), 1, now)
	if err != nil {
		fail(w, 409, "无法创建管理员账号")
		return
	}
	uid, _ := res.LastInsertId()
	res, err = tx.Exec("INSERT INTO conversations(kind,name,owner_id,created_at) VALUES('lobby','公共大厅',?,?)", uid, now)
	if err != nil {
		fail(w, 500, "创建大厅失败")
		return
	}
	cid, _ := res.LastInsertId()
	if _, err = tx.Exec("INSERT INTO members(conversation_id,user_id) VALUES(?,?)", cid, uid); err != nil {
		fail(w, 500, "创建大厅失败")
		return
	}
	if err = tx.Commit(); err != nil {
		fail(w, 500, "保存账号失败")
		return
	}
	a.setupToken = ""
	_ = os.Remove(filepath.Join(a.cfg.DataDir, "setup-token"))
	_ = os.Remove(filepath.Join(a.cfg.DataDir, "首次初始化.txt"))
	if !a.newSession(w, uid) {
		return
	}
	writeJSON(w, 201, map[string]any{"ok": true})
}

func (a *app) register(w http.ResponseWriter, r *http.Request) {
	if !a.limiter.allow("register:"+clientIP(r), 10, time.Minute) {
		fail(w, 429, "注册过于频繁，请稍后重试")
		return
	}
	var c credentials
	if !decode(w, r, &c) {
		return
	}
	if msg := validateCredentials(c); msg != "" {
		fail(w, 400, msg)
		return
	}
	var remaining int
	if err := a.store.QueryRow("SELECT remaining FROM invites WHERE token=? AND expires_at>? AND remaining>0",
		tokenHash(c.Token), time.Now().UnixMilli()).Scan(&remaining); err != nil {
		fail(w, 403, "邀请码无效、已用完或已过期")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(c.Password), bcrypt.DefaultCost)
	if err != nil {
		fail(w, 500, "创建账号失败")
		return
	}
	tx, err := a.store.Begin()
	if err != nil {
		fail(w, 500, "数据库暂时不可用")
		return
	}
	defer tx.Rollback()
	res, err := tx.Exec("UPDATE invites SET remaining=remaining-1 WHERE token=? AND expires_at>? AND remaining>0",
		tokenHash(c.Token), time.Now().UnixMilli())
	if err != nil {
		fail(w, 500, "邀请码处理失败")
		return
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		fail(w, 403, "邀请码无效或已过期")
		return
	}
	res, err = tx.Exec("INSERT INTO users(username,nickname,password,created_at) VALUES(?,?,?,?)",
		c.Username, strings.TrimSpace(c.Nickname), string(hash), time.Now().UnixMilli())
	if err != nil {
		fail(w, 409, "账号已存在")
		return
	}
	uid, _ := res.LastInsertId()
	_, err = tx.Exec("INSERT INTO members(conversation_id,user_id) SELECT id,? FROM conversations WHERE kind='lobby'", uid)
	if err != nil || tx.Commit() != nil {
		fail(w, 500, "保存账号失败")
		return
	}
	if !a.newSession(w, uid) {
		return
	}
	a.hub.broadcastAll(map[string]any{"type": "users.changed"})
	writeJSON(w, 201, map[string]any{"ok": true})
}

func (a *app) login(w http.ResponseWriter, r *http.Request) {
	var c struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decode(w, r, &c) {
		return
	}
	if !a.limiter.allow("login-ip:"+clientIP(r), 60, time.Minute) ||
		!a.limiter.allow("login-user:"+strings.ToLower(c.Username), 10, time.Minute) {
		fail(w, 429, "登录尝试过多，请一分钟后重试")
		return
	}
	var uid int64
	var hash string
	err := a.store.QueryRow("SELECT id,password FROM users WHERE username=?", c.Username).Scan(&uid, &hash)
	if err != nil {
		// Run the same expensive operation for unknown users.
		_ = bcrypt.CompareHashAndPassword([]byte("$2a$10$7EqJtq98hPqEX7fNZaFWoO5Lv4JQF99a5dVxNn6TvHHKgB5LrOzee"), []byte(c.Password))
		fail(w, 401, "账号或密码不正确")
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(c.Password)) != nil {
		fail(w, 401, "账号或密码不正确")
		return
	}
	if a.newSession(w, uid) {
		writeJSON(w, 200, map[string]any{"ok": true})
	}
}

func (a *app) newSession(w http.ResponseWriter, uid int64) bool {
	token := randomToken(32)
	expires := time.Now().Add(30 * 24 * time.Hour)
	if _, err := a.store.Exec("INSERT INTO sessions(token,user_id,expires_at) VALUES(?,?,?)", tokenHash(token), uid, expires.UnixMilli()); err != nil {
		log.Printf("创建会话失败：%v", err)
		fail(w, 500, "创建登录会话失败，请重新登录")
		return false
	}
	http.SetCookie(w, &http.Cookie{Name: "linchat_session", Value: token, Path: "/", HttpOnly: true,
		Secure: a.cfg.SecureCookie, SameSite: http.SameSiteLaxMode, MaxAge: 30 * 86400, Expires: expires})
	return true
}

type authHandler func(http.ResponseWriter, *http.Request, user)

func (a *app) auth(next authHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, _, err := a.currentUser(r)
		if err != nil {
			fail(w, 401, "请先登录")
			return
		}
		next(w, r, u)
	})
}

func (a *app) currentUser(r *http.Request) (user, int64, error) {
	var u user
	var expiry int64
	c, err := r.Cookie("linchat_session")
	if err != nil {
		return u, 0, err
	}
	err = a.store.QueryRow(`SELECT u.id,u.username,u.nickname,u.admin,s.expires_at FROM users u
		JOIN sessions s ON s.user_id=u.id WHERE s.token=? AND s.expires_at>?`, tokenHash(c.Value), time.Now().UnixMilli()).
		Scan(&u.ID, &u.Username, &u.Nickname, &u.Admin, &expiry)
	return u, expiry, err
}

func (a *app) logout(w http.ResponseWriter, r *http.Request, u user) {
	c, _ := r.Cookie("linchat_session")
	if _, err := a.store.Exec("DELETE FROM sessions WHERE token=?", tokenHash(c.Value)); err != nil {
		fail(w, 500, "退出失败")
		return
	}
	a.hub.closeSession(tokenHash(c.Value))
	http.SetCookie(w, &http.Cookie{Name: "linchat_session", Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: a.cfg.SecureCookie, SameSite: http.SameSiteLaxMode})
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (a *app) me(w http.ResponseWriter, r *http.Request, u user) {
	writeJSON(w, 200, u)
}

func (a *app) updateProfile(w http.ResponseWriter, r *http.Request, u user) {
	var in struct {
		Nickname    string `json:"nickname"`
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.Nickname = strings.TrimSpace(in.Nickname)
	if len(in.Nickname) == 0 || utf8.RuneCountInString(in.Nickname) > 30 {
		fail(w, 400, "昵称需为 1–30 个字符")
		return
	}
	if in.NewPassword != "" {
		if !a.limiter.allow("password:"+u.Username, 5, time.Minute) {
			fail(w, 429, "操作过于频繁，请稍后重试")
			return
		}
		var old string
		if a.store.QueryRow("SELECT password FROM users WHERE id=?", u.ID).Scan(&old) != nil ||
			bcrypt.CompareHashAndPassword([]byte(old), []byte(in.OldPassword)) != nil {
			fail(w, 403, "当前密码不正确")
			return
		}
		if len(in.NewPassword) < 10 || len(in.NewPassword) > 72 {
			fail(w, 400, "新密码需为 10–72 字节")
			return
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(in.NewPassword), bcrypt.DefaultCost)
		if err != nil {
			fail(w, 500, "密码更新失败")
			return
		}
		tx, err := a.store.Begin()
		if err != nil {
			fail(w, 500, "保存失败")
			return
		}
		defer tx.Rollback()
		if _, err = tx.Exec("UPDATE users SET password=?,nickname=? WHERE id=?", string(hash), in.Nickname, u.ID); err != nil {
			fail(w, 500, "保存失败")
			return
		}
		if _, err = tx.Exec("DELETE FROM sessions WHERE user_id=?", u.ID); err != nil || tx.Commit() != nil {
			fail(w, 500, "保存失败")
			return
		}
		a.hub.closeUser(u.ID)
		if !a.newSession(w, u.ID) {
			return
		}
	} else if _, err := a.store.Exec("UPDATE users SET nickname=? WHERE id=?", in.Nickname, u.ID); err != nil {
		fail(w, 500, "保存失败")
		return
	}
	a.hub.broadcastAll(map[string]any{"type": "users.changed"})
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (a *app) users(w http.ResponseWriter, r *http.Request, u user) {
	rows, err := a.store.Query("SELECT id,username,nickname,admin FROM users ORDER BY nickname")
	if err != nil {
		fail(w, 500, "读取成员失败")
		return
	}
	defer rows.Close()
	out := []user{}
	for rows.Next() {
		var item user
		if err := rows.Scan(&item.ID, &item.Username, &item.Nickname, &item.Admin); err != nil {
			fail(w, 500, "读取成员失败")
			return
		}
		item.Online = a.hub.online(item.ID)
		out = append(out, item)
	}
	writeJSON(w, 200, out)
}

func (a *app) createInvite(w http.ResponseWriter, r *http.Request, u user) {
	if !u.Admin {
		fail(w, 403, "只有管理员可以生成邀请")
		return
	}
	var in struct {
		Uses int `json:"uses"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Uses < 1 || in.Uses > 100 {
		fail(w, 400, "每个邀请支持 1–100 人")
		return
	}
	token := randomToken(24)
	expires := time.Now().Add(7 * 24 * time.Hour).UnixMilli()
	if _, err := a.store.Exec("INSERT INTO invites(token,created_by,remaining,expires_at) VALUES(?,?,?,?)",
		tokenHash(token), u.ID, in.Uses, expires); err != nil {
		fail(w, 500, "生成邀请失败")
		return
	}
	writeJSON(w, 201, map[string]any{"token": token, "uses": in.Uses, "expires_at": expires})
}

type rateEntry struct {
	N     int
	Until time.Time
}
type rateLimiter struct {
	sync.Mutex
	entries map[string]rateEntry
}

func newLimiter() *rateLimiter { return &rateLimiter{entries: make(map[string]rateEntry)} }

func (l *rateLimiter) allow(key string, max int, window time.Duration) bool {
	l.Lock()
	defer l.Unlock()
	now := time.Now()
	if len(l.entries) > 10000 {
		for k, v := range l.entries {
			if now.After(v.Until) {
				delete(l.entries, k)
			}
		}
		if len(l.entries) > 10000 {
			return false
		}
	}
	e := l.entries[key]
	if now.After(e.Until) {
		e = rateEntry{Until: now.Add(window)}
	}
	if e.N >= max {
		return false
	}
	e.N++
	l.entries[key] = e
	return true
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
