package collector

import "github.com/incendiarybean/go-collect-as-config/internal/collectors"

// Create new collectors based on provided config path
// Provides storage for collectors via SQLite and stores in the provided path
func CreateCollector(configPath string, sqlitePath string) collectors.Collector {
	workers := collectors.New(sqlitePath, configPath)

	return workers
}
