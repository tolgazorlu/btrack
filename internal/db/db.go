package db

import (
	"github.com/tolgazorlu/btrack/internal/config"
)

// Open returns the SQLite-backed Store.
func Open(cfg *config.Config) (Store, error) {
	return NewSQLiteStore(config.SQLitePath())
}
