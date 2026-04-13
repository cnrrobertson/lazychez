package gui

import (
	"fmt"
	"sync"

	"github.com/jesseduffield/gocui"
	"lazychez/pkg/filetree"
)

// initialLoad fetches all chezmoi data in parallel and populates the panels.
// Called once in a goroutine at startup, and again when the user presses 'r'.
// All writes to shared GUI state happen inside g.Update callbacks so they are
// serialized by the main loop, eliminating goroutine data races.
func (gui *Gui) initialLoad() {
	gui.logConsole("Loading chezmoi data...")

	var wg sync.WaitGroup
	wg.Add(3)

	// --- changed files (chezmoi status) ---
	go func() {
		defer wg.Done()
		files, err := gui.cz.Status()
		if err != nil {
			gui.logConsole("Error loading status: " + err.Error())
			return
		}
		gui.g.Update(func(g *gocui.Gui) error {
			gui.changedFiles = files
			gui.rebuildChangedTree()
			gui.changedIdx = 0
			if v, err := g.View("changed"); err == nil {
				v.SetOrigin(0, 0)
				v.SetCursor(0, 0)
			}
			gui.logConsole(fmt.Sprintf("Loaded %d changed file(s)", len(files)))
			if err := gui.renderChanged(g); err != nil {
				return err
			}
			if gui.currentPanel == "changed" {
				return gui.updatePreview(g)
			}
			return nil
		})
	}()

	// --- managed files (chezmoi managed --include=files) ---
	go func() {
		defer wg.Done()
		files, err := gui.cz.Managed()
		if err != nil {
			gui.logConsole("Error loading managed files: " + err.Error())
			return
		}
		gui.g.Update(func(g *gocui.Gui) error {
			gui.managedFiles = files
			gui.rebuildManagedTree()
			gui.managedIdx = 0
			if v, err := g.View("managed"); err == nil {
				v.SetOrigin(0, 0)
				v.SetCursor(0, 0)
			}
			gui.logConsole(fmt.Sprintf("Loaded %d managed file(s)", len(files)))
			return gui.renderManaged(g)
		})
	}()

	// --- scripts (chezmoi managed --include=scripts) ---
	go func() {
		defer wg.Done()
		scripts, err := gui.cz.Scripts()
		if err != nil {
			gui.logConsole("Error loading scripts: " + err.Error())
			return
		}
		gui.g.Update(func(g *gocui.Gui) error {
			gui.scripts = scripts
			gui.scriptIdx = 0
			if v, err := g.View("scripts"); err == nil {
				v.SetOrigin(0, 0)
				v.SetCursor(0, 0)
			}
			gui.logConsole(fmt.Sprintf("Loaded %d script(s)", len(scripts)))
			return gui.renderScripts(g)
		})
	}()

	wg.Wait()

	gui.g.Update(func(g *gocui.Gui) error {
		gui.logConsole("Ready. Press q to quit, ? for help.")
		return nil
	})
}

// reloadChanged re-runs `chezmoi status` and refreshes only the "changed"
// panel and preview. Called after a single-file apply so the panel updates
// without reloading the full managed/scripts lists.
func (gui *Gui) reloadChanged() {
	files, err := gui.cz.Status()
	if err != nil {
		gui.logConsole("Error reloading status: " + err.Error())
		return
	}
	gui.g.Update(func(g *gocui.Gui) error {
		gui.changedFiles = files
		gui.rebuildChangedTree()

		// Clamp index in case the applied file was the last one.
		if gui.changedIdx >= len(gui.changedFlat) && gui.changedIdx > 0 {
			gui.changedIdx = max(0, len(gui.changedFlat)-1)
			if v, err := g.View("changed"); err == nil {
				v.SetOrigin(0, 0)
				v.SetCursor(0, gui.changedIdx)
			}
		}

		if err := gui.renderChanged(g); err != nil {
			return err
		}
		if gui.currentPanel == "changed" {
			return gui.updatePreview(g)
		}
		return nil
	})
}

// ── Tree helpers ──────────────────────────────────────────────────────────────

// rebuildChangedTree reconstructs the tree and flat list for the "changed"
// panel from the current changedFiles slice. The collapsed map is preserved so
// directories the user folded survive a data reload.
func (gui *Gui) rebuildChangedTree() {
	paths := make([]string, len(gui.changedFiles))
	for i, f := range gui.changedFiles {
		paths[i] = f.Path
	}
	gui.changedTree = filetree.BuildTree(paths)
	gui.changedFlat = filetree.Flatten(gui.changedTree, gui.changedCollapsed)
}

// rebuildManagedTree reconstructs the tree and flat list for the "managed"
// panel from the current managedFiles slice.
func (gui *Gui) rebuildManagedTree() {
	paths := make([]string, len(gui.managedFiles))
	for i, f := range gui.managedFiles {
		paths[i] = f.Path
	}
	gui.managedTree = filetree.BuildTree(paths)
	gui.managedFlat = filetree.Flatten(gui.managedTree, gui.managedCollapsed)
}

// CmdLogger returns a function that logs a message to the console panel.
// Used to wire the chezmoi client's command log before the event loop starts,
// avoiding a circular dependency between the chezmoi and gui packages.
func (gui *Gui) CmdLogger() func(string) {
	return func(msg string) { gui.logConsole(msg) }
}

// logConsole appends a message to the console panel in a thread-safe way.
// Safe to call from any goroutine.
func (gui *Gui) logConsole(msg string) {
	if gui.g == nil {
		return
	}
	gui.g.Update(func(g *gocui.Gui) error {
		v, err := g.View("console")
		if err != nil {
			return nil
		}
		fmt.Fprintln(v, "> "+msg)
		return nil
	})
}
