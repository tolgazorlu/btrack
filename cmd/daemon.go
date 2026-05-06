package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/tolgazorlu/btrack/internal/config"
	"github.com/tolgazorlu/btrack/internal/daemon"
	"github.com/tolgazorlu/btrack/internal/db"
	"github.com/tolgazorlu/btrack/internal/ui"
)

var daemonCmd = &cobra.Command{
	Use:    "daemon",
	Short:  "Manage the background daemon process",
	Hidden: true,
}

var daemonStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the daemon (auto-started by CLI commands)",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		store, err := db.Open(cfg)
		if err != nil {
			return err
		}

		srv := daemon.NewServer(store)

		quit := make(chan os.Signal, 1)
		signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-quit
			srv.Stop()
			store.Close()
			os.Exit(0)
		}()

		return srv.Start()
	},
}

var daemonStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the running daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		data, err := os.ReadFile(config.PidFile())
		if err != nil {
			return fmt.Errorf("daemon not running (no pid file)")
		}
		pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
		if err != nil {
			return fmt.Errorf("invalid pid file")
		}
		proc, err := os.FindProcess(pid)
		if err != nil {
			return fmt.Errorf("process not found: %w", err)
		}
		if err := proc.Signal(os.Interrupt); err != nil {
			return fmt.Errorf("send signal: %w", err)
		}
		ui.Sign(ui.StyleSuccess.Render(ui.Sym.Stop), fmt.Sprintf("daemon stopped (pid %d)", pid))
		return nil
	},
}

var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check whether the daemon is running",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := daemon.NewClient()
		if client.Ping() {
			ui.Sign(ui.StyleSuccess.Render("●"), "daemon is running")
		} else {
			ui.Sign(ui.StyleDimmed.Render("○"), ui.StyleDimmed.Render("daemon is not running"))
		}
		return nil
	},
}

var daemonKillCmd = &cobra.Command{
	Use:   "kill",
	Short: "Force-kill the daemon (use after updating btrack binary)",
	RunE: func(cmd *cobra.Command, args []string) error {
		data, err := os.ReadFile(config.PidFile())
		if err != nil {
			ui.Sign(ui.StyleDimmed.Render("○"), ui.StyleDimmed.Render("daemon not running"))
			return nil
		}
		pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
		if err != nil {
			return fmt.Errorf("invalid pid file")
		}
		proc, err := os.FindProcess(pid)
		if err != nil {
			return fmt.Errorf("process not found: %w", err)
		}
		_ = proc.Kill()
		_ = os.Remove(config.PidFile())
		_ = os.Remove(config.SocketPath())
		ui.Sign(ui.StyleSuccess.Render(ui.Sym.Stop), fmt.Sprintf("daemon killed (pid %d)", pid))
		return nil
	},
}

var daemonRestartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Kill the daemon so the next command starts a fresh one",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := daemonKillCmd.RunE(cmd, args); err != nil {
			return err
		}
		ui.Hint("next btrack command will start a fresh daemon")
		return nil
	},
}

func init() {
	daemonCmd.AddCommand(daemonStartCmd, daemonStopCmd, daemonStatusCmd, daemonKillCmd, daemonRestartCmd)
	rootCmd.AddCommand(daemonCmd)
}
