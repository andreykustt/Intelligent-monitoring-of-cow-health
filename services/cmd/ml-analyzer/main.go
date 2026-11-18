package main

import (
	"cowhealth/internal/analyzer"
	"cowhealth/internal/platform"
	"net/http"
)

func main() {
	ctx := platform.Context()
	db := platform.DB(ctx)
	defer db.Close()
	go analyzer.Run(ctx, db)
	m := http.NewServeMux()
	platform.Health(m, db)
	platform.Serve(ctx, m)
}
