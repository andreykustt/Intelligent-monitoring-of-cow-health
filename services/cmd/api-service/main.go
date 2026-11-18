package main

import (
	"cowhealth/internal/api"
	"cowhealth/internal/platform"
)

func main() {
	ctx := platform.Context()
	db := platform.DB(ctx)
	defer db.Close()
	go platform.Outbox(ctx, db, "api-service")
	s := api.New(db)
	go s.ConsumeSimulationStates(ctx)
	platform.Serve(ctx, s.Handler())
}
