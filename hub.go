package main

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type peer struct {
	conn    *websocket.Conn
	userID  int64
	session string
	expires time.Time
	send    chan []byte
	done    chan struct{}
	once    sync.Once
}

func (p *peer) close() {
	p.once.Do(func() {
		close(p.done)
		_ = p.conn.Close()
	})
}

type hub struct {
	sync.RWMutex
	peers map[*peer]bool
}

func newHub() *hub { return &hub{peers: make(map[*peer]bool)} }

func (h *hub) online(uid int64) bool {
	h.RLock()
	defer h.RUnlock()
	for p := range h.peers {
		if p.userID == uid {
			return true
		}
	}
	return false
}

func (h *hub) broadcast(ids []int64, v any) {
	if len(ids) == 0 {
		return
	}
	h.fanout(ids, false, v)
}

func (h *hub) fanout(ids []int64, all bool, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		return
	}
	selected := make(map[int64]bool, len(ids))
	for _, id := range ids {
		selected[id] = true
	}
	h.RLock()
	defer h.RUnlock()
	for p := range h.peers {
		if all || selected[p.userID] {
			select {
			case p.send <- b:
			case <-p.done:
			default:
				p.close()
			}
		}
	}
}

func (h *hub) broadcastAll(v any) { h.fanout(nil, true, v) }

func (h *hub) closeSession(token string) {
	h.RLock()
	defer h.RUnlock()
	for p := range h.peers {
		if p.session == token {
			p.close()
		}
	}
}

func (h *hub) closeUser(id int64) {
	h.RLock()
	defer h.RUnlock()
	for p := range h.peers {
		if p.userID == id {
			p.close()
		}
	}
}

func (h *hub) close() {
	h.RLock()
	defer h.RUnlock()
	for p := range h.peers {
		p.close()
	}
}

func (a *app) websocket(w http.ResponseWriter, r *http.Request, u user) {
	upgrader := websocket.Upgrader{ReadBufferSize: 1024, WriteBufferSize: 4096, CheckOrigin: sameOrigin,
		HandshakeTimeout: 10 * time.Second}
	_, expiry, err := a.currentUser(r)
	if err != nil {
		fail(w, 401, "请重新登录")
		return
	}
	// Keep accidental reconnect storms from growing the connection set without bound.
	a.hub.RLock()
	count := 0
	for p := range a.hub.peers {
		if p.userID == u.ID {
			count++
		}
	}
	a.hub.RUnlock()
	if count >= 10 {
		fail(w, 429, "打开的聊天窗口过多")
		return
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	cookie, _ := r.Cookie("linchat_session")
	p := &peer{conn: conn, userID: u.ID, session: tokenHash(cookie.Value), expires: time.UnixMilli(expiry),
		send: make(chan []byte, 128), done: make(chan struct{})}
	a.hub.Lock()
	a.hub.peers[p] = true
	a.hub.Unlock()
	a.hub.broadcastAll(map[string]any{"type": "presence", "user_id": u.ID, "online": true})
	hello, _ := json.Marshal(map[string]any{"type": "hello", "user_id": u.ID, "server_time": time.Now().UnixMilli()})
	p.send <- hello
	go p.writeLoop()
	conn.SetReadLimit(1024)
	_ = conn.SetReadDeadline(time.Now().Add(65 * time.Second))
	conn.SetPongHandler(func(string) error { return conn.SetReadDeadline(time.Now().Add(65 * time.Second)) })
	defer func() {
		p.close()
		a.hub.Lock()
		delete(a.hub.peers, p)
		a.hub.Unlock()
		a.hub.broadcastAll(map[string]any{"type": "presence", "user_id": u.ID, "online": a.hub.online(u.ID)})
	}()
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
		// Persistent mutations use authenticated HTTP endpoints with explicit acknowledgements.
	}
}

func (p *peer) writeLoop() {
	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()
	defer p.close()
	for {
		select {
		case <-p.done:
			return
		case b := <-p.send:
			_ = p.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := p.conn.WriteMessage(websocket.TextMessage, b); err != nil {
				return
			}
		case <-ticker.C:
			if time.Now().After(p.expires) {
				return
			}
			_ = p.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := p.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
