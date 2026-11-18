package main

import (
	"cowhealth/internal/platform"
	"os"
	"path/filepath"
	"sort"
)

func main() {
	ctx := platform.Context()
	db := platform.DB(ctx)
	defer db.Close()
	conn, e := db.Acquire(ctx)
	if e != nil {
		panic(e)
	}
	defer conn.Release()
	if _, e = conn.Exec(ctx, "SELECT pg_advisory_lock(127001)"); e != nil {
		panic(e)
	}
	defer conn.Exec(ctx, "SELECT pg_advisory_unlock(127001)")
	if _, e = conn.Exec(ctx, "CREATE TABLE IF NOT EXISTS schema_migrations(name text PRIMARY KEY,applied_at timestamptz NOT NULL DEFAULT now())"); e != nil {
		panic(e)
	}
	files, e := filepath.Glob(platform.Env("MIGRATIONS_DIR", "../infrastructure/migrations") + "/*.up.sql")
	if e != nil || len(files) == 0 {
		panic("migrations missing")
	}
	sort.Strings(files)
	for _, f := range files {
		var exists bool
		if e = conn.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE name=$1)", filepath.Base(f)).Scan(&exists); e != nil {
			panic(e)
		}
		if exists {
			continue
		}
		b, e := os.ReadFile(f)
		if e != nil {
			panic(e)
		}
		tx, e := conn.Begin(ctx)
		if e != nil {
			panic(e)
		}
		if _, e = tx.Exec(ctx, string(b)); e != nil {
			panic(e)
		}
		if _, e = tx.Exec(ctx, "INSERT INTO schema_migrations(name) VALUES($1)", filepath.Base(f)); e != nil {
			panic(e)
		}
		if e = tx.Commit(ctx); e != nil {
			panic(e)
		}
	}
}
