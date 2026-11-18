package main

import (
	"cowhealth/internal/platform"
	"cowhealth/internal/telemetry"
)

func main() { telemetry.Ingest(platform.Context()) }
