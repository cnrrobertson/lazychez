package gui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jesseduffield/gocui"
	"lazychez/pkg/filetree"
)

// quit exits the TUI cleanly. Returning gocui.ErrQuit causes gocui to shut down.
func (gui *Gui) quit(_ *gocui.Gui, _ *gocui.View) error {
	return gocui.ErrQuit
}

// ── Jump to top / bottom ──────────────────────────────────────────────────────

// scrollToTop moves the cursor to the first item in the current panel/tab.
func (gui *Gui) scrollToTop(g *gocui.Gui, v *gocui.View) error {
	pv, panel := gui.resolvePanel(g, v)
	if pv == nil {
		return nil
	}
	idx, _ := gui.panelIdxAndLen(panel)
	if idx == nil {
		return nil
	}
	*idx = 0
	positionCursor(pv, 0)
	return gui.updatePreview(g)
}

// scrollToBottom moves the cursor to the last item in the current panel/tab.
func (gui *Gui) scrollToBottom(g *gocui.Gui, v *gocui.View) error {
	pv, panel := gui.resolvePanel(g, v)
	if pv == nil {
		return nil
	}
	idx, total := gui.panelIdxAndLen(panel)
	if idx == nil || total == 0 {
		return nil
	}
	*idx = total - 1
	positionCursor(pv, *idx)
	return gui.updatePreview(g)
}

// panelIdxAndLen returns a pointer to the active selection index and the
// length of the active list for the given panel, respecting the current tab.
// Returns (nil, 0) for unknown panels.
func (gui *Gui) panelIdxAndLen(panel string) (idx *int, total int) {
	switch panel {
	case "changed":
		if gui.changedTab == 1 {
			return &gui.fsIdx, len(gui.fsFlat)
		}
		return &gui.changedIdx, len(gui.changedFlat)
	case "managed":
		if gui.managedTab == 1 {
			return &gui.tmplIdx, len(gui.tmplFlat)
		}
		return &gui.managedIdx, len(gui.managedFlat)
	case "scripts":
		if gui.scriptsTab == 1 {
			return &gui.srcIdx, len(gui.srcFlat)
		}
		return &gui.scriptIdx, len(gui.scripts)
	}
	return nil, 0
}

// refreshAll reloads all chezmoi data.
func (gui *Gui) refreshAll(_ *gocui.Gui, _ *gocui.View) error {
	go gui.initialLoad()
	return nil
}

// ── Apply ─────────────────────────────────────────────────────────────────────

// applyFile suspends the TUI and runs `chezmoi apply <target>` with full
// terminal access, allowing chezmoi's interactive overwrite prompts to reach
// the user. Resumes the TUI and reloads the changed panel when done.
func (gui *Gui) applyFile(g *gocui.Gui, _ *gocui.View) error {
	target := gui.changedFileTarget()
	if target == "" {
		return nil
	}
	home, _ := os.UserHomeDir()
	cmd := exec.Command("chezmoi", "apply", filepath.Join(home, target))
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr

	if err := g.Suspend(); err != nil {
		return err
	}
	cmd.Run()
	if err := g.Resume(); err != nil {
		return err
	}
	go gui.reloadChanged()
	return nil
}

// applyAll suspends the TUI and runs `chezmoi apply` (no target) with full
// terminal access, allowing chezmoi's interactive overwrite prompts to reach
// the user. Resumes the TUI and reloads all panels when done.
func (gui *Gui) applyAll(g *gocui.Gui, _ *gocui.View) error {
	cmd := exec.Command("chezmoi", "apply")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr

	if err := g.Suspend(); err != nil {
		return err
	}
	cmd.Run()
	if err := g.Resume(); err != nil {
		return err
	}
	go gui.initialLoad()
	return nil
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

// ── Forget ────────────────────────────────────────────────────────────────────

// forgetFile runs `chezmoi forget <target>` for the currently selected file,
// removing it from chezmoi management without deleting the target file.
// Works from both the "managed" and "changed" panels.
func (gui *Gui) forgetFile(_ *gocui.Gui, _ *gocui.View) error {
	target := gui.currentTarget()
	if target == "" || filepath.IsAbs(target) {
		// Empty = nothing selected; absolute = source-dir file (Templates tab),
		// which has no chezmoi target to forget.
		return nil
	}
	go func() {
		gui.logConsole(fmt.Sprintf("Forgetting %s...", target))
		if err := gui.cz.Forget(target); err != nil {
			gui.logConsole("Error: " + err.Error())
			return
		}
		gui.logConsole(fmt.Sprintf("✓ Forgot %s", target))
		go gui.initialLoad()
	}()
	return nil
}

// ── Tree collapse ─────────────────────────────────────────────────────────────

// toggleCollapse expands or collapses the directory node currently under the
// cursor. No-ops when a file node is selected.
func (gui *Gui) toggleCollapse(g *gocui.Gui, v *gocui.View) error {
	switch v.Name() {
	case "changed":
		if gui.changedTab == 1 {
			return gui.toggleFsCollapse(g, v)
		}
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
		if gui.managedTab == 1 {
			return gui.toggleTmplCollapse(g, v)
		}
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

	case "scripts":
		if gui.scriptsTab == 1 {
			return gui.toggleSrcCollapse(g, v)
		}
	}
	return nil
}

// ── Edit ──────────────────────────────────────────────────────────────────────

// editFile suspends the TUI and opens the selected file for editing.
// For target files (home-relative paths), it uses `chezmoi edit` so chezmoi
// can locate the source. For source-dir files (absolute paths — returned by
// currentTarget when the Scripts Data tab is active), it opens $EDITOR directly.
func (gui *Gui) editFile(g *gocui.Gui, _ *gocui.View) error {
	target := gui.currentTarget()
	if target == "" {
		return nil
	}

	var cmd *exec.Cmd
	if filepath.IsAbs(target) {
		// Absolute path = source-dir file; open directly in $EDITOR.
		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "vi"
		}
		cmd = exec.Command(editor, target)
	} else {
		home, _ := os.UserHomeDir()
		cmd = exec.Command("chezmoi", "edit", filepath.Join(home, target))
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr

	if err := g.Suspend(); err != nil {
		return err
	}
	cmd.Run()
	if err := g.Resume(); err != nil {
		return err
	}
	go gui.initialLoad()
	return nil
}

// currentTarget returns the target path for the currently focused panel/item.
// Returns "" when the panel is empty, a directory node is selected, or the
// panel is unexpected.
func (gui *Gui) currentTarget() string {
	switch gui.currentPanel {
	case "changed":
		if gui.changedTab == 1 {
			if gui.fsIdx < len(gui.fsFlat) && !gui.fsFlat[gui.fsIdx].IsDir {
				return gui.fsFlat[gui.fsIdx].Path
			}
			return ""
		}
		return gui.changedFileTarget()
	case "managed":
		if gui.managedTab == 1 {
			// Templates tab: return absolute source path so editFile uses $EDITOR.
			if gui.tmplIdx < len(gui.tmplFlat) {
				fn := gui.tmplFlat[gui.tmplIdx]
				if !fn.Node.IsDir {
					return filepath.Join(gui.srcRoot, gui.tmplPaths[fn.Node.Index])
				}
			}
			return ""
		}
		return gui.managedFileTarget()
	case "scripts":
		if gui.scriptsTab == 1 {
			// Data tab: return absolute source path so editFile uses $EDITOR.
			if gui.srcIdx < len(gui.srcFlat) && !gui.srcFlat[gui.srcIdx].IsDir {
				return filepath.Join(gui.srcRoot, gui.srcFlat[gui.srcIdx].Path)
			}
			return ""
		}
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
// directory. Resumes the TUI and reloads all data when lazygit exits.
func (gui *Gui) openLazygit(g *gocui.Gui, _ *gocui.View) error {
	sourceDir, err := exec.Command("chezmoi", "source-path").Output()
	if err != nil {
		gui.logConsole("Error: could not get chezmoi source path: " + err.Error())
		return nil
	}
	cmd := exec.Command("lazygit", "-p", strings.TrimSpace(string(sourceDir)))
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr

	if err := g.Suspend(); err != nil {
		return err
	}
	cmd.Run()
	if err := g.Resume(); err != nil {
		return err
	}
	go gui.initialLoad()
	return nil
}

// ── Search ────────────────────────────────────────────────────────────────────

// openSearch opens the search bar for the currently focused panel.
// layout() creates the "searchbar" view on the next frame.
func (gui *Gui) openSearch(g *gocui.Gui, _ *gocui.View) error {
	gui.searchPanel = gui.currentPanel
	gui.searchQuery = ""
	gui.searchVisible = true
	return nil
}

// confirmSearch closes the search bar while keeping the highlights active on
// the panel. gocui's built-in n/N/Esc handling takes over once the panel
// regains focus. Syncs the panel index to wherever the search placed the cursor.
func (gui *Gui) confirmSearch(g *gocui.Gui) error {
	if pv, err := g.View(gui.searchPanel); err == nil {
		_, oy := pv.Origin()
		_, cy := pv.Cursor()
		gui.syncPanelIdx(gui.searchPanel, oy+cy)
	}
	gui.searchVisible = false
	g.DeleteView("searchbar")
	_, err := g.SetCurrentView(gui.searchPanel)
	return err
}

// cancelSearch clears all search highlights and closes the bar. Syncs the
// panel index to the current cursor position so the selection matches what's
// visible (search may have moved the cursor before the user cancelled).
func (gui *Gui) cancelSearch(g *gocui.Gui) error {
	if pv, err := g.View(gui.searchPanel); err == nil {
		_, oy := pv.Origin()
		_, cy := pv.Cursor()
		gui.syncPanelIdx(gui.searchPanel, oy+cy)
		pv.ClearSearch()
	}
	gui.searchVisible = false
	gui.searchQuery = ""
	g.DeleteView("searchbar")
	_, err := g.SetCurrentView(gui.searchPanel)
	return err
}

// syncPanelIdx sets the selection index for the given panel to line, clamped
// to the valid range. Used to re-sync after search navigation moves the view
// cursor without going through navigateDown/Up.
func (gui *Gui) syncPanelIdx(panel string, line int) {
	switch panel {
	case "changed":
		if gui.changedTab == 1 {
			if line >= 0 && line < len(gui.fsFlat) {
				gui.fsIdx = line
			}
		} else {
			if line >= 0 && line < len(gui.changedFlat) {
				gui.changedIdx = line
			}
		}
	case "managed":
		if gui.managedTab == 1 {
			if line >= 0 && line < len(gui.tmplFlat) {
				gui.tmplIdx = line
			}
		} else {
			if line >= 0 && line < len(gui.managedFlat) {
				gui.managedIdx = line
			}
		}
	case "scripts":
		if gui.scriptsTab == 1 {
			if line >= 0 && line < len(gui.srcFlat) {
				gui.srcIdx = line
			}
		} else {
			if line >= 0 && line < len(gui.scripts) {
				gui.scriptIdx = line
			}
		}
	}
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

// ── Changed panel tabs ────────────────────────────────────────────────────────

// nextChangedTab switches the Changed panel to the Files tab and builds the
// filesystem flat list on first visit (lazy — no up-front full-home scan).
func (gui *Gui) nextChangedTab(g *gocui.Gui, _ *gocui.View) error {
	if gui.changedTab == 1 {
		return nil
	}
	gui.changedTab = 1
	gui.rebuildFsFlat()
	gui.fsIdx = 0
	if v, err := g.View("changed"); err == nil {
		v.TabIndex = 1
		v.SetOrigin(0, 0)
		v.SetCursor(0, 0)
	}
	return gui.renderChanged(g)
}

// prevChangedTab switches the Changed panel back to the Changes tab and
// restores the cursor to the previously selected item.
func (gui *Gui) prevChangedTab(g *gocui.Gui, _ *gocui.View) error {
	if gui.changedTab == 0 {
		return nil
	}
	gui.changedTab = 0
	if v, err := g.View("changed"); err == nil {
		v.TabIndex = 0
		positionCursor(v, gui.changedIdx)
	}
	return gui.renderChanged(g)
}

// ── Filesystem collapse toggle ────────────────────────────────────────────────

// toggleFsCollapse expands or collapses the directory under the cursor in the
// Files tab. No-op when a file is selected.
func (gui *Gui) toggleFsCollapse(g *gocui.Gui, v *gocui.View) error {
	if gui.fsIdx >= len(gui.fsFlat) {
		return nil
	}
	entry := gui.fsFlat[gui.fsIdx]
	if !entry.IsDir {
		return nil
	}
	if gui.fsExpanded[entry.Path] {
		delete(gui.fsExpanded, entry.Path)
	} else {
		gui.fsExpanded[entry.Path] = true
	}
	gui.rebuildFsFlat()
	// Reposition cursor to the toggled directory.
	for i, fe := range gui.fsFlat {
		if fe.Path == entry.Path {
			gui.fsIdx = i
			if v != nil {
				positionCursor(v, gui.fsIdx)
			}
			break
		}
	}
	return gui.renderChanged(g)
}

// ── Add from filesystem tab ───────────────────────────────────────────────────

// addFsFile runs `chezmoi add` on the currently selected entry in the Files tab.
func (gui *Gui) addFsFile(_ *gocui.Gui, _ *gocui.View) error {
	if gui.fsIdx >= len(gui.fsFlat) {
		return nil
	}
	target := gui.fsFlat[gui.fsIdx].Path
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

// addFileDispatch routes the `+` keybind to the correct add handler depending
// on which tab is active on the Changed panel.
func (gui *Gui) addFileDispatch(g *gocui.Gui, v *gocui.View) error {
	if gui.changedTab == 1 {
		return gui.addFsFile(g, v)
	}
	return gui.addFile(g, v)
}

// ── Managed panel tabs ────────────────────────────────────────────────────────

// nextManagedTab switches the Managed panel to the Templates tab. Fetches the
// chezmoi source directory lazily on first visit and rebuilds the template tree.
func (gui *Gui) nextManagedTab(g *gocui.Gui, _ *gocui.View) error {
	if gui.managedTab == 1 {
		return nil
	}
	if gui.srcRoot == "" {
		srcDir, err := gui.cz.SourceDir()
		if err != nil {
			gui.logConsole("Error: could not get chezmoi source path: " + err.Error())
			return nil
		}
		gui.srcRoot = srcDir
	}
	gui.managedTab = 1
	gui.rebuildTmplTree()
	gui.tmplIdx = 0
	if v, err := g.View("managed"); err == nil {
		v.TabIndex = 1
		v.SetOrigin(0, 0)
		v.SetCursor(0, 0)
	}
	return gui.renderManaged(g)
}

// prevManagedTab switches the Managed panel back to the Managed tab and
// restores the cursor to the previously selected file.
func (gui *Gui) prevManagedTab(g *gocui.Gui, _ *gocui.View) error {
	if gui.managedTab == 0 {
		return nil
	}
	gui.managedTab = 0
	if v, err := g.View("managed"); err == nil {
		v.TabIndex = 0
		positionCursor(v, gui.managedIdx)
	}
	return gui.renderManaged(g)
}

// ── Template tree collapse toggle ─────────────────────────────────────────────

// toggleTmplCollapse expands or collapses the directory under the cursor in the
// Templates tab. No-op when a file is selected.
func (gui *Gui) toggleTmplCollapse(g *gocui.Gui, v *gocui.View) error {
	if gui.tmplIdx >= len(gui.tmplFlat) {
		return nil
	}
	fn := gui.tmplFlat[gui.tmplIdx]
	if !fn.Node.IsDir {
		return nil
	}
	if gui.tmplCollapsed[fn.Node.Path] {
		delete(gui.tmplCollapsed, fn.Node.Path)
	} else {
		gui.tmplCollapsed[fn.Node.Path] = true
	}
	gui.tmplFlat = filetree.Flatten(gui.tmplTree, gui.tmplCollapsed)
	// Reposition cursor to the toggled directory.
	for i, f := range gui.tmplFlat {
		if f.Node.Path == fn.Node.Path {
			gui.tmplIdx = i
			if v != nil {
				positionCursor(v, gui.tmplIdx)
			}
			break
		}
	}
	return gui.renderManaged(g)
}

// ── Scripts panel tabs ────────────────────────────────────────────────────────

// nextScriptsTab switches the Scripts panel to the Data tab. Fetches the
// chezmoi source directory lazily on first visit.
func (gui *Gui) nextScriptsTab(g *gocui.Gui, _ *gocui.View) error {
	if gui.scriptsTab == 1 {
		return nil
	}
	// Fetch source dir lazily — only on first switch.
	if gui.srcRoot == "" {
		srcDir, err := gui.cz.SourceDir()
		if err != nil {
			gui.logConsole("Error: could not get chezmoi source path: " + err.Error())
			return nil
		}
		gui.srcRoot = srcDir
	}
	gui.scriptsTab = 1
	gui.rebuildSrcFlat()
	// Expand all directories on first open — source dir is bounded, unlike ~.
	for {
		prev := len(gui.srcFlat)
		for _, fe := range gui.srcFlat {
			if fe.IsDir {
				gui.srcExpanded[fe.Path] = true
			}
		}
		gui.rebuildSrcFlat()
		if len(gui.srcFlat) == prev {
			break
		}
	}
	gui.srcIdx = 0
	if v, err := g.View("scripts"); err == nil {
		v.TabIndex = 1
		v.SetOrigin(0, 0)
		v.SetCursor(0, 0)
	}
	return gui.renderScripts(g)
}

// prevScriptsTab switches the Scripts panel back to the Scripts tab and
// restores the cursor to the previously selected script.
func (gui *Gui) prevScriptsTab(g *gocui.Gui, _ *gocui.View) error {
	if gui.scriptsTab == 0 {
		return nil
	}
	gui.scriptsTab = 0
	if v, err := g.View("scripts"); err == nil {
		v.TabIndex = 0
		positionCursor(v, gui.scriptIdx)
	}
	return gui.renderScripts(g)
}

// ── Collapse all / expand all ─────────────────────────────────────────────────

// collapseAll folds every directory in the active panel/tab at once.
// For filetree panels (collapsed map, empty = all expanded): populates the map
// with every directory path from the flat list.
// For FsEntry panels (expanded map, empty = all collapsed): clears the map.
// Scripts tab 0 is a flat list with no directories — no-op.
func (gui *Gui) collapseAll(g *gocui.Gui, v *gocui.View) error {
	pv, panel := gui.resolvePanel(g, v)
	switch panel {
	case "changed":
		if gui.changedTab == 1 {
			// Files tab: collapse = clear expanded map.
			clear(gui.fsExpanded)
			gui.rebuildFsFlat()
			gui.fsIdx = 0
			if pv != nil {
				positionCursor(pv, 0)
			}
			return gui.renderChanged(g)
		}
		// Changes tab: mark every directory collapsed.
		for _, fn := range gui.changedFlat {
			if fn.Node.IsDir {
				gui.changedCollapsed[fn.Node.Path] = true
			}
		}
		gui.changedFlat = filetree.Flatten(gui.changedTree, gui.changedCollapsed)
		gui.changedIdx = clampIdx(gui.changedIdx, len(gui.changedFlat))
		if pv != nil {
			positionCursor(pv, gui.changedIdx)
		}
		return gui.renderChanged(g)

	case "managed":
		if gui.managedTab == 1 {
			// Templates tab: mark every directory collapsed.
			for _, fn := range gui.tmplFlat {
				if fn.Node.IsDir {
					gui.tmplCollapsed[fn.Node.Path] = true
				}
			}
			gui.tmplFlat = filetree.Flatten(gui.tmplTree, gui.tmplCollapsed)
			gui.tmplIdx = clampIdx(gui.tmplIdx, len(gui.tmplFlat))
			if pv != nil {
				positionCursor(pv, gui.tmplIdx)
			}
			return gui.renderManaged(g)
		}
		// Managed tab: mark every directory collapsed.
		for _, fn := range gui.managedFlat {
			if fn.Node.IsDir {
				gui.managedCollapsed[fn.Node.Path] = true
			}
		}
		gui.managedFlat = filetree.Flatten(gui.managedTree, gui.managedCollapsed)
		gui.managedIdx = clampIdx(gui.managedIdx, len(gui.managedFlat))
		if pv != nil {
			positionCursor(pv, gui.managedIdx)
		}
		return gui.renderManaged(g)

	case "scripts":
		if gui.scriptsTab == 1 {
			// Data tab: collapse = clear expanded map.
			clear(gui.srcExpanded)
			gui.rebuildSrcFlat()
			gui.srcIdx = 0
			if pv != nil {
				positionCursor(pv, 0)
			}
			return gui.renderScripts(g)
		}
		// Scripts tab 0: flat list, nothing to collapse.
	}
	return nil
}

// expandAll unfolds every directory in the active panel/tab at once.
// For filetree panels: clears the collapsed map and rebuilds.
// For FsEntry panels (home dir / source dir): the home dir can be enormous so
// we only expand the FsEntry panels that are bounded (source dir). The Files
// tab (home dir) is left unchanged to avoid a multi-second scan.
func (gui *Gui) expandAll(g *gocui.Gui, v *gocui.View) error {
	pv, panel := gui.resolvePanel(g, v)
	switch panel {
	case "changed":
		if gui.changedTab == 1 {
			// Files tab: expanding the entire home dir is impractical — no-op.
			return nil
		}
		clear(gui.changedCollapsed)
		gui.changedFlat = filetree.Flatten(gui.changedTree, gui.changedCollapsed)
		gui.changedIdx = clampIdx(gui.changedIdx, len(gui.changedFlat))
		if pv != nil {
			positionCursor(pv, gui.changedIdx)
		}
		return gui.renderChanged(g)

	case "managed":
		if gui.managedTab == 1 {
			clear(gui.tmplCollapsed)
			gui.tmplFlat = filetree.Flatten(gui.tmplTree, gui.tmplCollapsed)
			gui.tmplIdx = clampIdx(gui.tmplIdx, len(gui.tmplFlat))
			if pv != nil {
				positionCursor(pv, gui.tmplIdx)
			}
			return gui.renderManaged(g)
		}
		clear(gui.managedCollapsed)
		gui.managedFlat = filetree.Flatten(gui.managedTree, gui.managedCollapsed)
		gui.managedIdx = clampIdx(gui.managedIdx, len(gui.managedFlat))
		if pv != nil {
			positionCursor(pv, gui.managedIdx)
		}
		return gui.renderManaged(g)

	case "scripts":
		if gui.scriptsTab == 1 {
			// Data tab: source dir is bounded — expand all entries lazily.
			for _, fe := range gui.srcFlat {
				if fe.IsDir {
					gui.srcExpanded[fe.Path] = true
				}
			}
			gui.rebuildSrcFlat()
			// Keep expanding until no new dirs appear (depth > 1 dirs were hidden).
			for {
				prev := len(gui.srcFlat)
				for _, fe := range gui.srcFlat {
					if fe.IsDir {
						gui.srcExpanded[fe.Path] = true
					}
				}
				gui.rebuildSrcFlat()
				if len(gui.srcFlat) == prev {
					break
				}
			}
			gui.srcIdx = clampIdx(gui.srcIdx, len(gui.srcFlat))
			if pv != nil {
				positionCursor(pv, gui.srcIdx)
			}
			return gui.renderScripts(g)
		}
		// Scripts tab 0: flat list, nothing to expand.
	}
	return nil
}

// ── Source dir collapse toggle ────────────────────────────────────────────────

// toggleSrcCollapse expands or collapses the directory under the cursor in the
// Data tab. No-op when a file is selected.
func (gui *Gui) toggleSrcCollapse(g *gocui.Gui, v *gocui.View) error {
	if gui.srcIdx >= len(gui.srcFlat) {
		return nil
	}
	entry := gui.srcFlat[gui.srcIdx]
	if !entry.IsDir {
		return nil
	}
	if gui.srcExpanded[entry.Path] {
		delete(gui.srcExpanded, entry.Path)
	} else {
		gui.srcExpanded[entry.Path] = true
	}
	gui.rebuildSrcFlat()
	// Reposition cursor to the toggled directory.
	for i, fe := range gui.srcFlat {
		if fe.Path == entry.Path {
			gui.srcIdx = i
			if v != nil {
				positionCursor(v, gui.srcIdx)
			}
			break
		}
	}
	return gui.renderScripts(g)
}
