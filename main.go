package main

import (
	"bufio"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
)

//go:embed all:web/dist
var assets embed.FS

const serverVersion = "1.2.9"

type config struct {
	Addr, DataDir, PublicURL, Name string
	SecureCookie                   bool
	MaxUpload, FileQuota           int64
}

type app struct {
	cfg         config
	store       *store
	hub         *hub
	setupToken  string
	setupMu     sync.Mutex
	uploadMu    sync.Mutex
	limiter     *rateLimiter
	gameWorkers sync.Map
	gameMu      sync.Mutex
	gameCtx     context.Context
	gameCancel  context.CancelFunc
	gameWG      sync.WaitGroup
	gameKey     []byte
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	addr := flag.String("listen", env("LINCHAT_LISTEN", "127.0.0.1:8787"), "监听地址")
	data := flag.String("data", env("LINCHAT_DATA", "./data"), "数据目录")
	public := flag.String("url", env("LINCHAT_URL", ""), "公开访问地址，例如 https://chat.linblog.site")
	name := flag.String("name", env("LINCHAT_NAME", "LinChat"), "服务器名称")
	mode := flag.String("mode", "standard", "standard 或 radmin")
	open := flag.Bool("open", false, "启动后打开浏览器")
	managed := flag.Bool("managed", false, "由桌面程序通过标准输入管理生命周期")
	flag.Parse()
	if *mode == "radmin" {
		ip, err := radminIP()
		if err != nil {
			log.Fatal(err)
		}
		*addr = net.JoinHostPort(ip, "8787")
		*public = "http://" + *addr
	}
	if *public == "" {
		*public = "http://" + *addr
	}
	u, err := url.Parse(*public)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		log.Fatal("访问地址格式无效，请使用 http:// 或 https:// 地址")
	}
	c := config{Addr: *addr, DataDir: *data, PublicURL: strings.TrimRight(*public, "/"), Name: *name,
		SecureCookie: u.Scheme == "https", MaxUpload: 25 << 20, FileQuota: 5 << 30}
	a, err := newApp(c)
	if err != nil {
		log.Fatal(err)
	}
	defer a.store.Close()
	defer a.closeGames()
	server := &http.Server{Addr: c.Addr, Handler: a.handler(), ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout: 2 * time.Minute, WriteTimeout: 2 * time.Minute, IdleTimeout: 90 * time.Second}
	listener, err := net.Listen("tcp", c.Addr)
	if err != nil {
		log.Fatal(err)
	}
	launchURL := c.PublicURL
	if a.setupToken != "" {
		launchURL += "/#setup=" + a.setupToken
		log.Printf("首次初始化地址（仅交给管理员）：%s", launchURL)
		_ = os.WriteFile(filepath.Join(c.DataDir, "首次初始化.txt"), []byte(launchURL+"\n"), 0600)
	}
	log.Printf("LinChat 已启动：%s | 数据目录：%s", c.PublicURL, c.DataDir)
	if *open {
		openBrowser(launchURL)
	}
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("HTTP 服务异常：%v", err)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	if *managed {
		go func() {
			scanner := bufio.NewScanner(os.Stdin)
			for scanner.Scan() {
				if scanner.Text() == "shutdown" {
					break
				}
			}
			stop <- os.Interrupt
		}()
	}
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	a.hub.close()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("关闭连接：%v", err)
	}
}

func newApp(c config) (*app, error) {
	if err := os.MkdirAll(filepath.Join(c.DataDir, "files"), 0700); err != nil {
		return nil, err
	}
	s, err := openStore(filepath.Join(c.DataDir, "linchat.db"))
	if err != nil {
		return nil, err
	}
	a := &app{cfg: c, store: s, hub: newHub(), limiter: newLimiter()}
	if err = a.initGames(); err != nil {
		s.Close()
		return nil, err
	}
	var count int
	if err = s.QueryRow("SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		s.Close()
		return nil, err
	}
	if count == 0 {
		p := filepath.Join(c.DataDir, "setup-token")
		b, e := os.ReadFile(p)
		if e != nil {
			if !os.IsNotExist(e) {
				s.Close()
				return nil, e
			}
			a.setupToken = randomToken(24)
			if err = os.WriteFile(p, []byte(a.setupToken), 0600); err != nil {
				s.Close()
				return nil, err
			}
		} else {
			a.setupToken = strings.TrimSpace(string(b))
			if len(a.setupToken) < 24 {
				s.Close()
				return nil, errors.New("初始化令牌无效")
			}
		}
	}
	a.resumeGames()
	return a, nil
}

func (a *app) handler() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("GET /api/status", a.status)
	m.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		if a.store.PingContext(r.Context()) != nil {
			fail(w, 503, "数据库暂时不可用")
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true, "version": serverVersion})
	})
	m.HandleFunc("POST /api/setup", a.setup)
	m.HandleFunc("POST /api/auth/login", a.login)
	m.HandleFunc("POST /api/auth/register", a.register)
	m.Handle("POST /api/auth/logout", a.auth(a.logout))
	m.Handle("GET /api/me", a.auth(a.me))
	m.Handle("PATCH /api/me", a.auth(a.updateProfile))
	m.Handle("GET /api/users", a.auth(a.users))
	m.Handle("POST /api/invites", a.auth(a.createInvite))
	m.Handle("GET /api/conversations", a.auth(a.conversations))
	m.Handle("POST /api/conversations", a.auth(a.createConversation))
	m.Handle("GET /api/conversations/{id}", a.auth(a.conversationDetail))
	m.Handle("POST /api/conversations/{id}/members", a.auth(a.addMembers))
	m.Handle("GET /api/conversations/{id}/messages", a.auth(a.messages))
	m.Handle("POST /api/conversations/{id}/messages", a.auth(a.sendMessage))
	m.Handle("POST /api/conversations/{id}/read", a.auth(a.markRead))
	m.Handle("POST /api/conversations/{id}/files", a.auth(a.uploadFile))
	m.Handle("GET /api/files/{id}", a.auth(a.downloadFile))
	m.Handle("GET /api/ws", a.auth(a.websocket))
	a.registerGames(m)
	m.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) { fail(w, 404, "接口不存在") })
	staticFS, _ := fs.Sub(assets, "web/dist")
	static := http.FileServer(http.FS(staticFS))
	m.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			fail(w, http.StatusMethodNotAllowed, "不支持此请求方法")
			return
		}
		if strings.HasPrefix(r.URL.Path, "/assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		static.ServeHTTP(w, r)
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; connect-src 'self'; img-src 'self' blob: data:; style-src 'self' 'unsafe-inline'; script-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		if r.Method != "GET" && r.Method != "HEAD" && r.Method != "OPTIONS" {
			if r.Header.Get("X-LinChat") != "1" || !sameOrigin(r) {
				fail(w, 403, "请求来源无效")
				return
			}
		}
		defer func() {
			if v := recover(); v != nil {
				log.Printf("处理请求异常：%v", v)
				fail(w, 500, "服务器处理失败")
			}
		}()
		m.ServeHTTP(w, r)
	})
}

func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && strings.EqualFold(u.Host, r.Host)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, code int, message string) {
	writeJSON(w, code, map[string]string{"error": message})
}

func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		fail(w, 415, "请使用 JSON 请求")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(dst); err != nil {
		fail(w, 400, "请求内容格式无效")
		return false
	}
	return true
}

func radminIP() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, i := range ifaces {
		if strings.Contains(strings.ToLower(i.Name), "radmin") && i.Flags&net.FlagUp != 0 {
			addrs, _ := i.Addrs()
			for _, a := range addrs {
				ip, _, e := net.ParseCIDR(a.String())
				if e == nil && ip.To4() != nil {
					return ip.String(), nil
				}
			}
		}
	}
	return "", fmt.Errorf("未发现已启用的 Radmin VPN IPv4 网卡，请先安装并连接 Radmin VPN")
}

func openBrowser(u string) {
	switch runtime.GOOS {
	case "windows":
		_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", u).Start()
	case "darwin":
		_ = exec.Command("open", u).Start()
	default:
		_ = exec.Command("xdg-open", u).Start()
	}
}
