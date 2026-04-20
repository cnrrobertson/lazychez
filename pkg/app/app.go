package app

import (
	"fmt"
	"os/exec"

	"github.com/muesli/termenv"
	"lazychez/pkg/chezmoi"
	"lazychez/pkg/gui"
)

// Run is the main entry point. It verifies chezmoi is available, detects
// terminal theme, and starts the TUI event loop.
//
// External commands (chezmoi edit, chezmoi apply, lazygit) are run directly
// from within keybinding handlers using gocui's Suspend/Resume mechanism,
// so the TUI never fully quits and all navigation state is preserved.
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
	g := gui.NewGui(cz, glamourStyle)
	cz.SetCmdLog(g.CmdLogger())
	return g.Run()
}
