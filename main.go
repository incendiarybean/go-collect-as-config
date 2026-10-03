package main

import (
	"log/slog"
	"os"
	"time"

	"benweare.co.uk/collector/internal/collectors"
	"benweare.co.uk/collector/internal/routes"
	"github.com/lmittmann/tint"
	_ "modernc.org/sqlite"
)

func main() {

	// Create a new coloured logger
	slog.SetDefault(slog.New(
		tint.NewTextHandler(os.Stderr, &tint.Options{
			Level:      slog.LevelDebug,
			TimeFormat: time.RFC3339,
		}),
	))

	// Create collectors using path to datastore
	workers := collectors.New("./store.db", "./config")

	// Run the workers threaded
	go workers.Run()

	// Initialise the routes
	routes.DefaultRoutes(workers.GetDB())
}
