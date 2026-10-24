package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	_ "dm"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type App struct {
	db       *sql.DB
	client   *http.Client
	mu       sync.Mutex
	flows    map[string]Flow
	sessions map[string]Session
}
type Flow struct {
	Bind, Verifier, Nonce, Redirect string
	Exp                             time.Time
}
type Session struct {
	Token, Sub, Name, Display string
	Exp                       time.Time
	Revision                  uint64
}
type Identity struct {
	Sub, Name, Display     string
	Super                  bool
	Role                   string
	Permissions, RoleNames []string
}

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func secret(k string) string {
	if p := os.Getenv(k + "_FILE"); p != "" {
		b, e := os.ReadFile(p)
		if e != nil {
			log.Fatal(k + " 配置不可读")
		}
		return strings.TrimSpace(string(b))
	}
	return os.Getenv(k)
}
func random() string {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
func hashed(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
func jsonOut(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, msg string) {
	jsonOut(w, status, map[string]string{"message": msg})
}
func decode(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	d := json.NewDecoder(r.Body)
	d.UseNumber()
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		return e
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return errors.New("请求须为单个 JSON 对象")
	}
	return nil
}
func cookie(w http.ResponseWriter, name, value string, age int) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: value, Path: "/", HttpOnly: true, Secure: strings.HasPrefix(env("PUBLIC_URL", ""), "https:"), SameSite: http.SameSiteLaxMode, MaxAge: age})
}
func originOK(r *http.Request) bool {
	return r.Header.Get("Origin") == env("PUBLIC_URL", "http://cd.libeasy.cn:8027")
}
func hashedCookie(r *http.Request) string {
	c, e := r.Cookie("revenue_session")
	if e != nil {
		return ""
	}
	return hashed(c.Value)
}
func (a *App) getSession(r *http.Request) (Session, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	key := hashedCookie(r)
	s, ok := a.sessions[key]
	if ok && time.Now().After(s.Exp) {
		delete(a.sessions, key)
		ok = false
	}
	return s, ok
}
func (a *App) authorized(next func(http.ResponseWriter, *http.Request, Identity)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s, ok := a.getSession(r)
		if !ok {
			fail(w, 401, "请通过统一身份认证登录")
			return
		}
		id, e := a.identity(s)
		if e != nil {
			a.mu.Lock()
			delete(a.sessions, hashedCookie(r))
			a.mu.Unlock()
			fail(w, 403, "统一认证不可用或应用授权已撤销，请重新登录")
			return
		}
		id, e = a.localRole(r.Context(), id)
		if e != nil {
			a.invalidateUsers([]string{s.Sub})
			fail(w, 403, "本系统用户已停用或尚未分配有效角色，请联系管理员")
			return
		}
		if r.Method != "GET" && !originOK(r) {
			fail(w, 403, "请求来源校验失败")
			return
		}
		next(w, r, id)
	}
}
func (a *App) health(w http.ResponseWriter, r *http.Request) {
	ctx, c := context.WithTimeout(r.Context(), 3*time.Second)
	defer c()
	if a.db.PingContext(ctx) != nil {
		fail(w, 503, "数据库暂不可用")
		return
	}
	jsonOut(w, 200, map[string]string{"status": "ok", "schema": "REVENUESTATISTICSSYSTEM", "database": "SZGCTRIP"})
}
func main() {
	dsn := secret("DM_DSN")
	if dsn == "" {
		log.Fatal("缺少 DM_DSN")
	}
	db, e := sql.Open("dm", dsn)
	if e != nil {
		log.Fatal("达梦初始化失败")
	}
	defer db.Close()
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(3)
	db.SetConnMaxLifetime(10 * time.Minute)
	ctx, c := context.WithTimeout(context.Background(), 8*time.Second)
	e = db.PingContext(ctx)
	c()
	if e != nil {
		log.Fatal("达梦连接失败")
	}
	a := &App{db: db, client: &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, flows: map[string]Flow{}, sessions: map[string]Session{}}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/auth/login", a.login)
	mux.HandleFunc("/api/auth/callback", a.callback)
	mux.HandleFunc("/api/auth/session", a.session)
	mux.HandleFunc("/api/auth/logout", a.logout)
	mux.HandleFunc("/api/health", a.health)
	mux.HandleFunc("/api/ingest", a.ingest)
	mux.HandleFunc("/api/ingest/guide", a.guideIngest)
	mux.HandleFunc("/api/", a.authorized(a.api))
	static := env("STATIC_DIR", "../frontend/apps/web-antd/dist")
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "HEAD" {
			fail(w, 405, "方法不支持")
			return
		}
		if r.URL.Path == "/oidc-callback" {
			http.ServeFile(w, r, static+"/index.html")
			return
		}
		http.FileServer(http.Dir(static)).ServeHTTP(w, r)
	})
	go func() {
		for range time.Tick(time.Minute) {
			a.mu.Lock()
			for k, f := range a.flows {
				if time.Now().After(f.Exp) {
					delete(a.flows, k)
				}
			}
			for k, s := range a.sessions {
				if time.Now().After(s.Exp) {
					delete(a.sessions, k)
				}
			}
			a.mu.Unlock()
		}
	}()
	srv := &http.Server{Addr: env("LISTEN_ADDR", "127.0.0.1:8127"), Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if env("GUIDE_INGEST_ONLY", "") == "1" && r.URL.Path != "/api/ingest/guide" && r.URL.Path != "/api/health" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("Referrer-Policy", "same-origin")
		mux.ServeHTTP(w, r)
	}), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	log.Printf("营收统计服务监听 %s", srv.Addr)
	log.Fatal(srv.ListenAndServe())
}
