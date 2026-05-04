package db

import (
	"fmt"

	"github.com/tolgazorlu/btrack/internal/config"
)

// Open returns the configured Store.
func Open(cfg *config.Config) (Store, error) {
	switch cfg.Database.Type {
	case "postgres":
		if cfg.Database.DSN == "" {
			return nil, fmt.Errorf("database.dsn is required for postgres")
		}
		return NewPostgresStore(cfg.Database.DSN)
	default:
		return NewSQLiteStore(config.SQLitePath())
	}
}
