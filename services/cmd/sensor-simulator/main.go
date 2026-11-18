package main

import (
	"cowhealth/internal/platform"
	"cowhealth/internal/simulator"
	"net/http"
)

func main() {
	ctx := platform.Context()
	db := platform.DB(ctx)
	defer db.Close()
	go simulator.Run(ctx, db)
	m := http.NewServeMux()
	platform.Health(m, db)
	platform.Serve(ctx, m)
}
