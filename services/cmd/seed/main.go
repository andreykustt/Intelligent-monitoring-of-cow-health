package main

import (
	"cowhealth/internal/platform"
	"fmt"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	ctx := platform.Context()
	db := platform.DB(ctx)
	defer db.Close()
	tx, e := db.Begin(ctx)
	if e != nil {
		panic(e)
	}
	defer tx.Rollback(ctx)
	exec := func(q string, a ...any) {
		if _, e := tx.Exec(ctx, q, a...); e != nil {
			panic(e)
		}
	}
	exec("INSERT INTO farms VALUES('demo','Учебная ферма') ON CONFLICT DO NOTHING")
	for _, role := range []string{"admin", "operator"} {
		key := "ADMIN_PASSWORD"
		if role == "operator" {
			key = "OPERATOR_PASSWORD"
		}
		p := platform.Required(key)
		if len(p) < 12 {
			panic("seed passwords must contain at least 12 characters")
		}
		hash, e := bcrypt.GenerateFromPassword([]byte(p), 12)
		if e != nil {
			panic(e)
		}
		exec("INSERT INTO users(id,username,password_hash,role) VALUES($1,$2,$3,$2) ON CONFLICT(username) DO NOTHING", platform.ID(), role, string(hash))
		exec("INSERT INTO user_farms SELECT id,'demo' FROM users WHERE username=$1 ON CONFLICT DO NOTHING", role)
	}
	for i := 1; i <= 200; i++ {
		id := fmt.Sprintf("COW-%03d", i)
		exec("INSERT INTO cows(id,farm_id,name) VALUES($1,'demo',$2) ON CONFLICT DO NOTHING", id, fmt.Sprintf("Корова %03d", i))
		exec("INSERT INTO sensors SELECT $1||':'||code,$1,code FROM metric_definitions ON CONFLICT DO NOTHING", id)
	}
	if e = tx.Commit(ctx); e != nil {
		panic(e)
	}
}
