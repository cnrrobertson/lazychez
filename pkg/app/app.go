package app

import (
	"fmt"
	"os"
	"os/exec"

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

		pendingEdit, err := g.Run()
		if err != nil {
			return err
		}
		if pendingEdit == "" {
			// Normal quit — nothing pending.
			return nil
		}

		// Suspend TUI: run `chezmoi edit <target>` with full terminal access.
		cmd := exec.Command("chezmoi", "edit", pendingEdit)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if runErr := cmd.Run(); runErr != nil {
			fmt.Fprintf(os.Stderr, "chezmoi edit: %v\n", runErr)
		}
		// Loop: restart the TUI after the editor exits.
	}
}
