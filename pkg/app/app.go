package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/muesli/termenv"
	"lazychez/pkg/chezmoi"
	"lazychez/pkg/gui"
)

// Run is the main entry point. It verifies chezmoi is available, detects
// terminal theme, and starts the TUI event loop.
//
// The loop restarts when the user presses 'e' to edit a file: the TUI exits,
// chezmoi edit runs with full terminal access, then the TUI relaunches.
func Run() error {
	if _, err := exec.LookPath("chezmoi"); err != nil {
		return fmt.Errorf("chezmoi not found — install from https://www.chezmoi.io")
	}

	// Detect terminal background colour HERE — before gocui takes over stdin.
	// termenv.HasDarkBackground() sends an OSC 11 query and reads the response;
	// if called after gocui.NewGui() the response bytes leak into gocui's event
	// stream and the 5-second OSCTimeout causes a noticeable startup delay.
	glamourStyle := "dark"
	if !termenv.HasDarkBackground() {
		glamourStyle = "light"
	}

	cz := chezmoi.NewClient()

	for {
		g := gui.NewGui(cz, glamourStyle)
		cz.SetCmdLog(g.CmdLogger())

		result, err := g.Run()
		if err != nil {
			return err
		}

		// Normal quit — nothing pending.
		if result.PendingEdit == "" && result.PendingApply == "" && !result.ApplyAll {
			return nil
		}

		var cmd *exec.Cmd
		switch {
		case result.PendingEdit != "":
			// Suspend TUI: run `chezmoi edit <target>` with full terminal access.
			// Resolve to absolute path — exec.Command does not invoke a shell.
			home, _ := os.UserHomeDir()
			cmd = exec.Command("chezmoi", "edit", filepath.Join(home, result.PendingEdit))
		case result.ApplyAll:
			// Suspend TUI: run `chezmoi apply` with full terminal access so that
			// chezmoi's interactive overwrite prompts reach the user.
			cmd = exec.Command("chezmoi", "apply")
		case result.PendingApply != "":
			// Suspend TUI: run `chezmoi apply <target>` with full terminal access.
			// Resolve the home-relative path to an absolute path; exec.Command
			// does not invoke a shell so "~/" would not be expanded.
			home, _ := os.UserHomeDir()
			absTarget := filepath.Join(home, result.PendingApply)
			cmd = exec.Command("chezmoi", "apply", absTarget)
		}

		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if runErr := cmd.Run(); runErr != nil {
			fmt.Fprintf(os.Stderr, "chezmoi: %v\n", runErr)
		}
		// Loop: restart the TUI after the command exits.
	}
}
