package cmd

import (
	"fmt"

	"github.com/tolgazorlu/btrack/internal/config"
	"github.com/tolgazorlu/btrack/internal/db"
)

// loadSessionJSON opens the store, loads session id with its notes, and
// converts it to the stable SessionJSON shape. Used by the live-timer
// commands (start/stop/switch/resume) to report the session the daemon
// just touched, since the daemon's own SessionDTO is a slimmer shape.
func loadSessionJSON(id int64) (SessionJSON, error) {
	cfg, err := config.Load()
	if err != nil {
		return SessionJSON{}, err
	}
	store, err := db.Open(cfg)
	if err != nil {
		return SessionJSON{}, err
	}
	defer store.Close()

	sess, err := store.GetSessionByID(id)
	if err != nil || sess == nil {
		return SessionJSON{}, fmt.Errorf("session %d not found", id)
	}
	notes, _ := store.GetAllLogs(id)
	return toSessionJSON(sess, notes), nil
}
