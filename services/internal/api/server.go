package api

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"cowhealth/internal/platform"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type identity struct {
	ID       string   `json:"id"`
	Username string   `json:"username"`
	Role     string   `json:"role"`
	CSRF     string   `json:"csrf_token"`
	Farms    []string `json:"farms"`
}
type identityKey struct{}
type Server struct {
	DB         *pgxpool.Pool
	Origin     string
	Secure     bool
	loginMu    sync.Mutex
	loginTimes map[string][]time.Time
}

func New(db *pgxpool.Pool) *Server {
	return &Server{DB: db, Origin: platform.Required("APP_ORIGIN"), Secure: platform.Env("COOKIE_SECURE", "false") == "true", loginTimes: map[string][]time.Time{}}
}
func who(r *http.Request) identity { return r.Context().Value(identityKey{}).(identity) }
func hash(s string) string         { b := sha256.Sum256([]byte(s)); return hex.EncodeToString(b[:]) }
func (s *Server) Handler() http.Handler {
	m := http.NewServeMux()
	platform.Health(m, s.DB)
	m.HandleFunc("POST /api/v1/auth/login", s.login)
	protected := http.NewServeMux()
	protected.HandleFunc("GET /api/v1/auth/me", func(w http.ResponseWriter, r *http.Request) { platform.JSON(w, 200, who(r)) })
	protected.HandleFunc("POST /api/v1/auth/logout", s.logout)
	protected.HandleFunc("GET /api/v1/cows", s.cows)
	protected.HandleFunc("GET /api/v1/cows/{id}", s.cow)
	protected.HandleFunc("POST /api/v1/cows", s.saveCow)
	protected.HandleFunc("PATCH /api/v1/cows/{id}", s.saveCow)
	protected.HandleFunc("GET /api/v1/metrics", s.metrics)
	protected.HandleFunc("GET /api/v1/telemetry/latest", s.latest)
	protected.HandleFunc("GET /api/v1/telemetry/history", s.history)
	protected.HandleFunc("GET /api/v1/cows/{id}/measurements", s.history)
	protected.HandleFunc("GET /api/v1/cows/{id}/predictions", s.predictions)
	protected.HandleFunc("GET /api/v1/cows/{id}/health-summary", s.timeline)
	protected.HandleFunc("GET /api/v1/analytics/overview", s.overview)
	protected.HandleFunc("GET /api/v1/analytics/{metric}", s.history)
	protected.HandleFunc("GET /api/v1/alerts", s.alerts)
	protected.HandleFunc("GET /api/v1/alerts/{id}", s.alert)
	protected.HandleFunc("POST /api/v1/alerts/{id}/{action}", s.alertAction)
	protected.HandleFunc("GET /api/v1/simulations", s.simulations)
	protected.HandleFunc("POST /api/v1/simulations", s.simulationCreate)
	protected.HandleFunc("GET /api/v1/simulations/{id}", s.simulation)
	protected.HandleFunc("POST /api/v1/simulations/{id}/{action}", s.simulationAction)
	protected.HandleFunc("PATCH /api/v1/simulations/{id}/config", s.simulationAction)
	protected.HandleFunc("GET /api/v1/users", s.users)
	protected.HandleFunc("POST /api/v1/users", s.saveUser)
	protected.HandleFunc("PATCH /api/v1/users/{id}", s.saveUser)
	protected.HandleFunc("POST /api/v1/users/{id}/password", s.password)
	protected.HandleFunc("GET /api/v1/stream", s.stream)
	for _, kind := range []string{"milkings", "inspections", "laboratory-results", "body-condition-records", "veterinary-examinations"} {
		protected.HandleFunc("POST /api/v1/"+kind, s.manualCreate)
		protected.HandleFunc("GET /api/v1/"+kind+"/{id}", s.manualGet)
		protected.HandleFunc("PATCH /api/v1/"+kind+"/{id}", s.manualEdit)
		protected.HandleFunc("GET /api/v1/"+kind+"/{id}/revisions", s.revisions)
	}
	m.Handle("/api/v1/", s.auth(protected))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := platform.ID()
		w.Header().Set("X-Request-ID", requestID)
		started := time.Now()
		defer func() {
			slog.Info("http request", "request_id", requestID, "method", r.Method, "path", r.URL.Path, "duration_ms", time.Since(started).Milliseconds())
		}()
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		defer func() {
			if e := recover(); e != nil {
				slog.Error("request panic", "error", e)
				platform.Error(w, 500, "INTERNAL_ERROR", "Внутренняя ошибка")
			}
		}()
		if !strings.HasSuffix(r.URL.Path, "/stream") {
			ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
			defer cancel()
			r = r.WithContext(ctx)
		}
		m.ServeHTTP(w, r)
	})
}
func (s *Server) session(r *http.Request) (identity, error) {
	var u identity
	c, e := r.Cookie("session")
	if e != nil {
		return u, e
	}
	e = s.DB.QueryRow(r.Context(), `SELECT u.id::text,u.username,u.role,se.csrf,ARRAY(SELECT farm_id FROM user_farms WHERE user_id=u.id ORDER BY farm_id) FROM sessions se JOIN users u ON u.id=se.user_id WHERE token_hash=$1 AND expires_at>now() AND u.active`, hash(c.Value)).Scan(&u.ID, &u.Username, &u.Role, &u.CSRF, &u.Farms)
	return u, e
}
func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, e := s.session(r)
		if e != nil {
			platform.Error(w, 401, "UNAUTHORIZED", "Войдите в систему")
			return
		}
		if r.Method != "GET" {
			if r.Header.Get("Origin") != s.Origin || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-CSRF-Token")), []byte(u.CSRF)) != 1 {
				platform.Error(w, 403, "FORBIDDEN", "Проверка CSRF не пройдена")
				return
			}
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), identityKey{}, u)))
	})
}
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Origin") != s.Origin {
		platform.Error(w, 403, "FORBIDDEN", "Недопустимый Origin")
		return
	}
	key := r.Header.Get("X-Real-IP")
	if key == "" {
		key = strings.Split(r.RemoteAddr, ":")[0]
	}
	s.loginMu.Lock()
	now := time.Now()
	old := s.loginTimes[key]
	recent := old[:0]
	for _, t := range old {
		if now.Sub(t) < time.Minute {
			recent = append(recent, t)
		}
	}
	allowed := len(recent) < 10
	if allowed {
		recent = append(recent, now)
	}
	s.loginTimes[key] = recent
	if len(s.loginTimes) > 10000 {
		for k, v := range s.loginTimes {
			if len(v) == 0 || now.Sub(v[len(v)-1]) > time.Minute {
				delete(s.loginTimes, k)
			}
		}
	}
	s.loginMu.Unlock()
	if !allowed {
		platform.Error(w, 429, "RATE_LIMITED", "Слишком много попыток. Повторите через минуту")
		return
	}
	var b struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !platform.Decode(w, r, &b) {
		return
	}
	var id, p string
	err := s.DB.QueryRow(r.Context(), "SELECT id::text,password_hash FROM users WHERE username=$1 AND active", b.Username).Scan(&id, &p)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(p), []byte(b.Password)) != nil {
		platform.Error(w, 401, "UNAUTHORIZED", "Неверный логин или пароль")
		return
	}
	token, csrf := platform.Token(), platform.Token()
	if _, err = s.DB.Exec(r.Context(), "INSERT INTO sessions VALUES($1,$2,$3,now()+interval '12 hours')", hash(token), id, csrf); err != nil {
		s.fail(w, err)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "session", Value: token, Path: "/", HttpOnly: true, Secure: s.Secure, SameSite: http.SameSiteStrictMode, MaxAge: 43200})
	platform.JSON(w, 200, map[string]bool{"ok": true})
}
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	c, _ := r.Cookie("session")
	if _, e := s.DB.Exec(r.Context(), "DELETE FROM sessions WHERE token_hash=$1", hash(c.Value)); e != nil {
		s.fail(w, e)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "session", Path: "/", MaxAge: -1, HttpOnly: true, Secure: s.Secure, SameSite: http.SameSiteStrictMode})
	platform.JSON(w, 200, map[string]bool{"ok": true})
}
func admin(w http.ResponseWriter, r *http.Request) bool {
	if who(r).Role != "admin" {
		platform.Error(w, 403, "FORBIDDEN", "Требуется роль администратора")
		return false
	}
	return true
}
func farmAllowed(r *http.Request, f string) bool {
	for _, v := range who(r).Farms {
		if f == v {
			return true
		}
	}
	return false
}
func (s *Server) cowAllowed(w http.ResponseWriter, r *http.Request, id string) bool {
	var ok bool
	e := s.DB.QueryRow(r.Context(), "SELECT EXISTS(SELECT 1 FROM cows WHERE id=$1 AND farm_id=ANY($2))", id, who(r).Farms).Scan(&ok)
	if e != nil {
		s.fail(w, e)
		return false
	}
	if !ok {
		platform.Error(w, 404, "NOT_FOUND", "Животное не найдено")
	}
	return ok
}
func (s *Server) fail(w http.ResponseWriter, e error) {
	slog.Error("database request", "error", e)
	platform.Error(w, 500, "INTERNAL_ERROR", "Не удалось выполнить операцию")
}
func (s *Server) one(w http.ResponseWriter, r *http.Request, q string, a ...any) {
	var b json.RawMessage
	e := s.DB.QueryRow(r.Context(), q, a...).Scan(&b)
	if errors.Is(e, pgx.ErrNoRows) {
		platform.Error(w, 404, "NOT_FOUND", "Запись не найдена")
		return
	}
	if e != nil {
		s.fail(w, e)
		return
	}
	platform.JSON(w, 200, b)
}
func (s *Server) list(w http.ResponseWriter, r *http.Request, q string, a ...any) {
	rows, e := s.DB.Query(r.Context(), q, a...)
	if e != nil {
		s.fail(w, e)
		return
	}
	defer rows.Close()
	items := []json.RawMessage{}
	for rows.Next() {
		var b json.RawMessage
		if e = rows.Scan(&b); e != nil {
			s.fail(w, e)
			return
		}
		items = append(items, b)
	}
	if rows.Err() != nil {
		s.fail(w, rows.Err())
		return
	}
	platform.JSON(w, 200, items)
}
func page(w http.ResponseWriter, r *http.Request) (int, int, bool) {
	p, n := 1, 25
	var e error
	if v := r.URL.Query().Get("page"); v != "" {
		p, e = strconv.Atoi(v)
		if e != nil {
			p = 0
		}
	}
	if v := r.URL.Query().Get("page_size"); v != "" {
		n, e = strconv.Atoi(v)
		if e != nil {
			n = 0
		}
	}
	if p < 1 || p > 100000 || n < 1 || n > 100 {
		platform.Error(w, 422, "VALIDATION_ERROR", "Некорректная пагинация")
		return 0, 0, false
	}
	return p, n, true
}
func (s *Server) paged(w http.ResponseWriter, r *http.Request, query string, args ...any) {
	p, n, ok := page(w, r)
	if !ok {
		return
	}
	var total int
	var items json.RawMessage
	args = append(args, n, (p-1)*n)
	q := fmt.Sprintf(`WITH data AS (%s), selected AS (SELECT item FROM data ORDER BY ordinal LIMIT $%d OFFSET $%d) SELECT (SELECT count(*) FROM data),COALESCE((SELECT jsonb_agg(item) FROM selected),'[]')`, query, len(args)-1, len(args))
	if e := s.DB.QueryRow(r.Context(), q, args...).Scan(&total, &items); e != nil {
		s.fail(w, e)
		return
	}
	platform.JSON(w, 200, map[string]any{"items": items, "page": p, "page_size": n, "total": total})
}
