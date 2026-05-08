package cmd

import (
	"fmt"
	"os/exec"
	"strings"

	mcpserver "github.com/tolgazorlu/btrack/internal/mcp"
	"github.com/tolgazorlu/btrack/internal/ui"
)

// handleMCPSlash dispatches /mcp inside the interactive console. The default
// (`/mcp` with no subcommand) ensures the background HTTP server is running
// and prints its URL plus copy-paste instructions for every supported AI
// client. Subcommands target specific clients or stop the server.
//
//	/mcp                         start (if needed) + show register options
//	/mcp :9000                   start on a custom port
//	/mcp stop                    stop the background server
//	/mcp status                  show url without (re)starting
//	/mcp claude                  run `claude mcp add ...`
//	/mcp cursor / gemini / chatgpt   print the JSON snippet to paste
func handleMCPSlash(input string) {
	rest := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(input), "/mcp"))
	parts := strings.Fields(rest)
	sub := ""
	if len(parts) > 0 {
		sub = strings.ToLower(parts[0])
	}

	switch sub {
	case "stop":
		stopMCP()
		return
	case "status":
		printMCPStatus()
		return
	case "claude":
		registerWithClaude()
		return
	case "cursor":
		printJSONSnippet("paste into ~/.cursor/mcp.json")
		return
	case "gemini":
		printJSONSnippet("paste into ~/.gemini/settings.json")
		return
	case "chatgpt", "openai":
		printChatGPTHint()
		return
	}

	// Accept addr overrides: empty (default), ":port", or "host:port".
	// Anything else is a typo — show a hint rather than trying to bind.
	if sub != "" && !strings.HasPrefix(sub, ":") && !strings.Contains(sub, ":") {
		ui.Blank()
		ui.Hint("unknown /mcp option: " + sub)
		ui.Hint("try: /mcp  /mcp stop  /mcp status  /mcp claude  /mcp cursor  /mcp gemini")
		ui.Blank()
		return
	}
	addr := "127.0.0.1:8765"
	if sub != "" {
		addr = sub
	}
	startMCPAndPrint(addr)
}

func startMCPAndPrint(addr string) {
	status, err := mcpserver.StartHTTPBackground(addr)
	if err != nil {
		fmt.Fprintln(ui.Out, ui.Indent+ui.StyleError.Render(" error ")+" "+err.Error())
		return
	}
	ui.Blank()
	ui.Sign(
		ui.StyleSuccess.Render("●"),
		"btrack mcp http  "+ui.StyleHighlight.Render(status.URL)+"  "+ui.StyleDimmed.Render(fmt.Sprintf("(pid %d)", status.PID)),
	)
	ui.Blank()
	ui.Section("register your AI client")
	ui.Cmd("/mcp claude", "run `claude mcp add` automatically")
	ui.Cmd("/mcp cursor", "print snippet for ~/.cursor/mcp.json")
	ui.Cmd("/mcp gemini", "print snippet for ~/.gemini/settings.json")
	ui.Cmd("/mcp chatgpt", "show ChatGPT Desktop connector instructions")
	ui.Blank()
	ui.Hint("/mcp stop  to stop the server  ·  /mcp status  to recall the url")
	ui.Blank()
}

func stopMCP() {
	if err := mcpserver.StopHTTPBackground(); err != nil {
		ui.Sign(ui.StyleDimmed.Render("○"), ui.StyleDimmed.Render("mcp http: "+err.Error()))
		return
	}
	ui.Sign(ui.StyleDimmed.Render(ui.Sym.Stop), ui.StyleDimmed.Render("btrack mcp http stopped"))
}

func printMCPStatus() {
	status := mcpserver.CurrentHTTPStatus()
	ui.Blank()
	if !status.Running {
		ui.Sign(ui.StyleDimmed.Render("○"), ui.StyleDimmed.Render("btrack mcp http: not running — type /mcp to start"))
		ui.Blank()
		return
	}
	ui.Sign(
		ui.StyleSuccess.Render("●"),
		"btrack mcp http  "+ui.StyleHighlight.Render(status.URL)+"  "+ui.StyleDimmed.Render(fmt.Sprintf("(pid %d)", status.PID)),
	)
	ui.Blank()
}

// registerWithClaude shells out to `claude mcp add ...`. If the claude CLI
// isn't on PATH (or the command fails), we fall back to printing the exact
// command for the user to copy.
func registerWithClaude() {
	status := mcpserver.CurrentHTTPStatus()
	if !status.Running {
		ui.Hint("server not running — type /mcp to start it first")
		return
	}
	args := []string{"mcp", "add", "--transport", "http", "btrack", status.URL}

	ui.Blank()
	if _, err := exec.LookPath("claude"); err != nil {
		ui.Sign(ui.StyleDimmed.Render("○"), "claude CLI not found — copy & run:")
		fmt.Fprintln(ui.Out, ui.Indent+"  claude "+strings.Join(args, " "))
		ui.Blank()
		return
	}

	out, err := exec.Command("claude", args...).CombinedOutput()
	trimmed := strings.TrimSpace(string(out))
	if err != nil {
		ui.Sign(ui.StyleError.Render("✗"), "claude mcp add failed — copy & run:")
		fmt.Fprintln(ui.Out, ui.Indent+"  claude "+strings.Join(args, " "))
		if trimmed != "" {
			fmt.Fprintln(ui.Out, ui.Indent+"  "+trimmed)
		}
		ui.Blank()
		return
	}
	ui.Sign(ui.StyleSuccess.Render("✓"), "registered with claude code")
	if trimmed != "" {
		fmt.Fprintln(ui.Out, ui.Indent+trimmed)
	}
	ui.Blank()
}

func printJSONSnippet(label string) {
	status := mcpserver.CurrentHTTPStatus()
	if !status.Running {
		ui.Hint("server not running — type /mcp to start it first")
		return
	}
	snippet := fmt.Sprintf(`{
  "mcpServers": {
    "btrack": {
      "url": "%s"
    }
  }
}`, status.URL)
	ui.Blank()
	ui.Section(label)
	for _, line := range strings.Split(snippet, "\n") {
		fmt.Fprintln(ui.Out, ui.Indent+line)
	}
	ui.Blank()
}

func printChatGPTHint() {
	status := mcpserver.CurrentHTTPStatus()
	if !status.Running {
		ui.Hint("server not running — type /mcp to start it first")
		return
	}
	ui.Blank()
	ui.Section("chatgpt desktop connector")
	fmt.Fprintln(ui.Out, ui.Indent+"Settings → Connectors → Custom connector")
	fmt.Fprintln(ui.Out, ui.Indent+"  Name: btrack")
	fmt.Fprintln(ui.Out, ui.Indent+"  URL:  "+ui.StyleHighlight.Render(status.URL))
	ui.Blank()
}
