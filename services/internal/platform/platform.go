package platform

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func Required(k string) string {
	v := os.Getenv(k)
	if v == "" {
		panic(k + " is required")
	}
	return v
}
func ID() string {
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:])
}
func Token() string {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
func Context() context.Context {
	ctx, _ := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	return ctx
}
func DB(ctx context.Context) *pgxpool.Pool {
	p, e := pgxpool.New(ctx, Required("DATABASE_URL"))
	if e != nil {
		panic(e)
	}
	return p
}
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func Error(w http.ResponseWriter, status int, code, msg string) {
	JSON(w, status, map[string]any{"error": map[string]any{"code": code, "message": msg, "details": map[string]any{}}})
}
func Decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		Error(w, 422, "VALIDATION_ERROR", "Некорректный JSON: "+e.Error())
		return false
	}
	if d.Decode(new(any)) != io.EOF {
		Error(w, 422, "VALIDATION_ERROR", "Ожидался один JSON-объект")
		return false
	}
	return true
}
func Serve(ctx context.Context, h http.Handler) {
	s := &http.Server{Addr: Env("LISTEN_ADDR", ":8080"), Handler: h, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		<-ctx.Done()
		c, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = s.Shutdown(c)
	}()
	slog.Info("http listening", "address", s.Addr)
	if e := s.ListenAndServe(); e != nil && e != http.ErrServerClosed {
		slog.Error("http server", "error", e)
		os.Exit(1)
	}
}
func Health(m *http.ServeMux, p *pgxpool.Pool) {
	m.HandleFunc("GET /health/live", func(w http.ResponseWriter, r *http.Request) { JSON(w, 200, map[string]bool{"live": true}) })
	m.HandleFunc("GET /health/ready", func(w http.ResponseWriter, r *http.Request) {
		ctx, c := context.WithTimeout(r.Context(), 2*time.Second)
		defer c()
		if p != nil && p.Ping(ctx) != nil {
			Error(w, 503, "NOT_READY", "Database unavailable")
			return
		}
		JSON(w, 200, map[string]bool{"ready": true})
	})
}
