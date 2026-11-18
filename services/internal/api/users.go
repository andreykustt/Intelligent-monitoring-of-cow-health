package api

import (
	"cowhealth/internal/platform"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"strings"
)

func (s *Server) users(w http.ResponseWriter, r *http.Request) {
	if !admin(w, r) {
		return
	}
	s.paged(w, r, `SELECT jsonb_build_object('id',u.id,'username',u.username,'role',u.role,'active',u.active,'farms',ARRAY(SELECT farm_id FROM user_farms WHERE user_id=u.id)) item,row_number() OVER(ORDER BY username) ordinal FROM users u WHERE EXISTS(SELECT 1 FROM user_farms f WHERE f.user_id=u.id AND f.farm_id=ANY($1)) AND NOT EXISTS(SELECT 1 FROM user_farms f WHERE f.user_id=u.id AND NOT(f.farm_id=ANY($1)))`, who(r).Farms)
}
func (s *Server) saveUser(w http.ResponseWriter, r *http.Request) {
	if !admin(w, r) {
		return
	}
	var b struct {
		Username string   `json:"username"`
		Password string   `json:"password"`
		Role     string   `json:"role"`
		Farms    []string `json:"farms"`
		Active   bool     `json:"active"`
	}
	if !platform.Decode(w, r, &b) {
		return
	}
	if b.Role != "admin" && b.Role != "operator" || len(b.Farms) == 0 {
		platform.Error(w, 422, "VALIDATION_ERROR", "Проверьте роль и фермы")
		return
	}
	for _, f := range b.Farms {
		if !farmAllowed(r, f) {
			platform.Error(w, 403, "FORBIDDEN", "Ферма недоступна")
			return
		}
	}
	ctx := r.Context()
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		s.fail(w, e)
		return
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(127002)"); e != nil {
		s.fail(w, e)
		return
	}
	id := r.PathValue("id")
	if id == "" {
		if len(b.Password) < 12 || len(b.Password) > 72 || strings.TrimSpace(b.Username) == "" {
			platform.Error(w, 422, "VALIDATION_ERROR", "Нужен логин и пароль 12–72 символа")
			return
		}
		h, _ := bcrypt.GenerateFromPassword([]byte(b.Password), 12)
		id = platform.ID()
		_, e = tx.Exec(ctx, "INSERT INTO users VALUES($1,$2,$3,$4,$5)", id, b.Username, string(h), b.Role, b.Active)
	} else {
		var visible bool
		e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users u WHERE id::text=$1 AND EXISTS(SELECT 1 FROM user_farms WHERE user_id=u.id AND farm_id=ANY($2)) AND NOT EXISTS(SELECT 1 FROM user_farms WHERE user_id=u.id AND NOT(farm_id=ANY($2))))`, id, who(r).Farms).Scan(&visible)
		if e != nil || !visible {
			platform.Error(w, 404, "NOT_FOUND", "Пользователь не найден")
			return
		}
		_, e = tx.Exec(ctx, "UPDATE users SET role=$2,active=$3 WHERE id::text=$1", id, b.Role, b.Active)
	}
	if e != nil {
		platform.Error(w, 409, "CONFLICT", "Логин уже занят или данные некорректны")
		return
	}
	var count int
	if e = tx.QueryRow(ctx, "SELECT count(*) FROM users WHERE active AND role='admin'").Scan(&count); e != nil {
		s.fail(w, e)
		return
	}
	if count == 0 {
		platform.Error(w, 409, "CONFLICT", "Нельзя отключить последнего администратора")
		return
	}
	_, e = tx.Exec(ctx, "DELETE FROM user_farms WHERE user_id::text=$1", id)
	for _, f := range b.Farms {
		if e == nil {
			_, e = tx.Exec(ctx, "INSERT INTO user_farms VALUES($1,$2) ON CONFLICT DO NOTHING", id, f)
		}
	}
	if e == nil {
		_, e = tx.Exec(ctx, "DELETE FROM sessions WHERE user_id::text=$1", id)
	}
	if e == nil {
		e = tx.Commit(ctx)
	}
	if e != nil {
		s.fail(w, e)
		return
	}
	platform.JSON(w, 200, map[string]string{"id": id})
}
func (s *Server) password(w http.ResponseWriter, r *http.Request) {
	if !admin(w, r) {
		return
	}
	var b struct {
		Password string `json:"password"`
	}
	if !platform.Decode(w, r, &b) {
		return
	}
	if len(b.Password) < 12 || len(b.Password) > 72 {
		platform.Error(w, 422, "VALIDATION_ERROR", "Пароль должен содержать 12–72 символа")
		return
	}
	h, _ := bcrypt.GenerateFromPassword([]byte(b.Password), 12)
	ctx := r.Context()
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		s.fail(w, e)
		return
	}
	defer tx.Rollback(ctx)
	tag, e := tx.Exec(ctx, `UPDATE users u SET password_hash=$1 WHERE id::text=$2 AND EXISTS(SELECT 1 FROM user_farms WHERE user_id=u.id AND farm_id=ANY($3)) AND NOT EXISTS(SELECT 1 FROM user_farms WHERE user_id=u.id AND NOT(farm_id=ANY($3)))`, string(h), r.PathValue("id"), who(r).Farms)
	if e != nil {
		s.fail(w, e)
		return
	}
	if tag.RowsAffected() == 0 {
		platform.Error(w, 404, "NOT_FOUND", "Пользователь не найден")
		return
	}
	_, e = tx.Exec(ctx, "DELETE FROM sessions WHERE user_id::text=$1", r.PathValue("id"))
	if e == nil {
		e = tx.Commit(ctx)
	}
	if e != nil {
		s.fail(w, e)
		return
	}
	platform.JSON(w, 200, map[string]bool{"ok": true})
}
