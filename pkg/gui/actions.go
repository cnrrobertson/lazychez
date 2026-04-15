package gui

import (
	"fmt"

	"github.com/jesseduffield/gocui"
	"lazychez/pkg/filetree"
)

// quit exits the TUI cleanly. Returning gocui.ErrQuit causes gocui to shut down.
func (gui *Gui) quit(_ *gocui.Gui, _ *gocui.View) error {
	return gocui.ErrQuit
}

// refreshAll reloads all chezmoi data.
func (gui *Gui) refreshAll(_ *gocui.Gui, _ *gocui.View) error {
	go gui.initialLoad()
	return nil
}

// ── Apply ─────────────────────────────────────────────────────────────────────

// applyFile suspends the TUI and runs `chezmoi apply <target>` with full
// terminal access, allowing chezmoi's interactive overwrite prompts to reach
// the user. The TUI restarts automatically after the command completes.
func (gui *Gui) applyFile(_ *gocui.Gui, _ *gocui.View) error {
	target := gui.changedFileTarget()
	if target == "" {
		return nil
	}
	gui.pendingApply = target
	return gocui.ErrQuit
}

// applyAll suspends the TUI and runs `chezmoi apply` (no target) with full
// terminal access, allowing chezmoi's interactive overwrite prompts to reach
// the user. The TUI restarts automatically after the command completes.
func (gui *Gui) applyAll(_ *gocui.Gui, _ *gocui.View) error {
	gui.pendingApplyAll = true
	return gocui.ErrQuit
}

// ── Add / Re-add ──────────────────────────────────────────────────────────────

// addFile runs `chezmoi add <target>` for the currently selected file or
// directory in the "changed" panel.
func (gui *Gui) addFile(_ *gocui.Gui, _ *gocui.View) error {
	target := gui.changedTarget()
	if target == "" {
		return nil
	}

	go func() {
		gui.logConsole(fmt.Sprintf("Adding %s...", target))
		if err := gui.cz.Add(target); err != nil {
			gui.logConsole("Error: " + err.Error())
			return
		}
		gui.logConsole(fmt.Sprintf("✓ Added %s", target))
		go gui.reloadChanged()
	}()
	return nil
}

// reAddFile runs `chezmoi re-add <target>` for the currently selected file or
// directory in the "changed" panel.
func (gui *Gui) reAddFile(_ *gocui.Gui, _ *gocui.View) error {
	target := gui.changedTarget()
	if target == "" {
		return nil
	}

	go func() {
		gui.logConsole(fmt.Sprintf("Re-adding %s...", target))
		if err := gui.cz.ReAdd(target); err != nil {
			gui.logConsole("Error: " + err.Error())
			return
		}
		gui.logConsole(fmt.Sprintf("✓ Re-added %s", target))
		go gui.reloadChanged()
	}()
	return nil
}

// ── Tree collapse ─────────────────────────────────────────────────────────────

// toggleCollapse expands or collapses the directory node currently under the
// cursor. No-ops when a file node is selected.
func (gui *Gui) toggleCollapse(g *gocui.Gui, v *gocui.View) error {
	switch v.Name() {
	case "changed":
		if gui.changedIdx >= len(gui.changedFlat) {
			return nil
		}
		node := gui.changedFlat[gui.changedIdx].Node
		if !node.IsDir {
			return nil
		}
		if gui.changedCollapsed[node.Path] {
			delete(gui.changedCollapsed, node.Path)
		} else {
			gui.changedCollapsed[node.Path] = true
		}
		gui.rebuildChangedTree()
		gui.repositionToDir(v, gui.changedFlat, &gui.changedIdx, node.Path)
		return gui.renderChanged(g)

	case "managed":
		if gui.managedIdx >= len(gui.managedFlat) {
			return nil
		}
		node := gui.managedFlat[gui.managedIdx].Node
		if !node.IsDir {
			return nil
		}
		if gui.managedCollapsed[node.Path] {
			delete(gui.managedCollapsed, node.Path)
		} else {
			gui.managedCollapsed[node.Path] = true
		}
		gui.rebuildManagedTree()
		gui.repositionToDir(v, gui.managedFlat, &gui.managedIdx, node.Path)
		return gui.renderManaged(g)
	}
	return nil
}

// ── Edit ──────────────────────────────────────────────────────────────────────

// editFile sets gui.pendingEdit to the current item's target path and quits the
// TUI. app.Run() detects the non-empty pendingEdit, runs `chezmoi edit <path>`
// with full terminal access, then restarts the TUI.
func (gui *Gui) editFile(g *gocui.Gui, v *gocui.View) error {
	target := gui.currentTarget()
	if target == "" {
		return nil
	}
	gui.logConsole(fmt.Sprintf("Opening editor for %s...", target))
	gui.pendingEdit = target
	return gocui.ErrQuit
}

// currentTarget returns the target path for the currently focused panel/item.
// Returns "" when the panel is empty, a directory node is selected, or the
// panel is unexpected.
func (gui *Gui) currentTarget() string {
	switch gui.currentPanel {
	case "changed":
		return gui.changedFileTarget()
	case "managed":
		return gui.managedFileTarget()
	case "scripts":
		if gui.scriptIdx < len(gui.scripts) {
			return gui.scripts[gui.scriptIdx].Path
		}
	}
	return ""
}

// changedFileTarget returns the target path of the selected file in the
// "changed" panel, or "" if a directory node is selected or the panel is empty.
func (gui *Gui) changedFileTarget() string {
	if gui.changedIdx >= len(gui.changedFlat) {
		return ""
	}
	fn := gui.changedFlat[gui.changedIdx]
	if fn.Node.IsDir {
		return ""
	}
	return gui.changedFiles[fn.Node.Index].Path
}

// changedTarget returns the target path of the selected file or directory in
// the "changed" panel. Unlike changedFileTarget, this also returns dir paths
// so that add/re-add can operate recursively on whole directories.
func (gui *Gui) changedTarget() string {
	if gui.changedIdx >= len(gui.changedFlat) {
		return ""
	}
	fn := gui.changedFlat[gui.changedIdx]
	if fn.Node.IsDir {
		return fn.Node.Path
	}
	return gui.changedFiles[fn.Node.Index].Path
}

// managedFileTarget returns the target path of the selected file in the
// "managed" panel, or "" if a directory node is selected or the panel is empty.
func (gui *Gui) managedFileTarget() string {
	if gui.managedIdx >= len(gui.managedFlat) {
		return ""
	}
	fn := gui.managedFlat[gui.managedIdx]
	if fn.Node.IsDir {
		return ""
	}
	return gui.managedFiles[fn.Node.Index].Path
}

// repositionToDir finds dirPath in flat and sets *idx and the view cursor to
// that position. Used after a collapse toggle so the cursor stays on the
// directory that was just folded/unfolded.
func (gui *Gui) repositionToDir(v *gocui.View, flat []filetree.FlatNode, idx *int, dirPath string) {
	for i, fn := range flat {
		if fn.Node.Path == dirPath {
			*idx = i
			_, vh := v.InnerSize()
			v.SetOrigin(0, 0)
			if i < vh {
				v.SetCursor(0, i)
			} else {
				v.SetOrigin(0, i-vh+1)
				v.SetCursor(0, vh-1)
			}
			return
		}
	}
}

// ── Lazygit ───────────────────────────────────────────────────────────────────

// openLazygit suspends the TUI and launches lazygit in the chezmoi source
// directory. The TUI restarts automatically when lazygit exits.
func (gui *Gui) openLazygit(_ *gocui.Gui, _ *gocui.View) error {
	gui.pendingLazygit = true
	return gocui.ErrQuit
}

// ── Help overlay ──────────────────────────────────────────────────────────────

// showHelp opens the keybindings help overlay. Pressing '?' again closes it.
func (gui *Gui) showHelp(g *gocui.Gui, v *gocui.View) error {
	if gui.helpVisible {
		return gui.hideHelp(g, v)
	}
	gui.helpVisible = true
	gui.helpFilter = ""
	// layout() will create the overlay views on the next frame.
	return nil
}

// hideHelp closes the help overlay and restores focus to the previous panel.
func (gui *Gui) hideHelp(g *gocui.Gui, _ *gocui.View) error {
	gui.helpVisible = false
	g.DeleteView("help")
	g.DeleteView("helpsearch")
	_, err := g.SetCurrentView(gui.currentPanel)
	return err
}
