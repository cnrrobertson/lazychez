package gui

import "github.com/jesseduffield/gocui"

// setKeybindings registers all key handlers via registerBinding, which stores
// each Binding for the dynamic help overlay and also calls g.SetKeybinding.
func (gui *Gui) setKeybindings() error {
	bs := []*Binding{
		// --- global ---
		{ViewName: "", Key: '?', Mod: gocui.ModNone, Description: "show keybindings help", Tag: "global", Handler: gui.showHelp},
		{ViewName: "", Key: gocui.KeyCtrlC, Mod: gocui.ModNone, Description: "quit", Tag: "global", Handler: gui.quit},
		{ViewName: "", Key: 'q', Mod: gocui.ModNone, Description: "quit", Tag: "global", Handler: gui.quit},
		{ViewName: "", Key: gocui.KeyTab, Mod: gocui.ModNone, Description: "next panel", Tag: "navigation", Handler: gui.nextPanel},
		{ViewName: "", Key: gocui.KeyBacktab, Mod: gocui.ModNone, Description: "previous panel", Tag: "navigation", Handler: gui.prevPanel},
		{ViewName: "", Key: 'r', Mod: gocui.ModNone, Description: "refresh all", Tag: "global", Handler: gui.refreshAll},
		{ViewName: "", Key: 'g', Mod: gocui.ModNone, Description: "open lazygit in chezmoi source dir", Tag: "global", Handler: gui.openLazygit},

		// --- changed panel ---
		{ViewName: "changed", Key: 'j', Mod: gocui.ModNone, Description: "move down", Tag: "navigation", Handler: gui.navigateDown},
		{ViewName: "changed", Key: gocui.KeyArrowDown, Mod: gocui.ModNone, Handler: gui.navigateDown},
		{ViewName: "changed", Key: 'k', Mod: gocui.ModNone, Description: "move up", Tag: "navigation", Handler: gui.navigateUp},
		{ViewName: "changed", Key: gocui.KeyArrowUp, Mod: gocui.ModNone, Handler: gui.navigateUp},
		{ViewName: "changed", Key: ']', Mod: gocui.ModNone, Description: "next tab", Handler: gui.nextChangedTab},
		{ViewName: "changed", Key: '[', Mod: gocui.ModNone, Description: "previous tab", Handler: gui.prevChangedTab},
		{ViewName: "changed", Key: 'a', Mod: gocui.ModNone, Description: "apply this file", Handler: gui.applyFile},
		{ViewName: "changed", Key: 'A', Mod: gocui.ModNone, Description: "apply all files", Handler: gui.applyAll},
		{ViewName: "changed", Key: 'e', Mod: gocui.ModNone, Description: "edit source file", Handler: gui.editFile},
		{ViewName: "changed", Key: '+', Mod: gocui.ModNone, Description: "add file to chezmoi", Handler: gui.addFileDispatch},
		{ViewName: "changed", Key: 'R', Mod: gocui.ModNone, Description: "re-add (pull target → source)", Handler: gui.reAddFile},
		{ViewName: "changed", Key: gocui.KeyEnter, Mod: gocui.ModNone, Description: "expand/collapse directory", Handler: gui.toggleCollapse},
		{ViewName: "changed", Key: 'D', Mod: gocui.ModNone, Description: "forget file (stop tracking)", Handler: gui.forgetFile},

		// --- managed panel ---
		{ViewName: "managed", Key: 'j', Mod: gocui.ModNone, Description: "move down", Tag: "navigation", Handler: gui.navigateDown},
		{ViewName: "managed", Key: gocui.KeyArrowDown, Mod: gocui.ModNone, Handler: gui.navigateDown},
		{ViewName: "managed", Key: 'k', Mod: gocui.ModNone, Description: "move up", Tag: "navigation", Handler: gui.navigateUp},
		{ViewName: "managed", Key: gocui.KeyArrowUp, Mod: gocui.ModNone, Handler: gui.navigateUp},
		{ViewName: "managed", Key: ']', Mod: gocui.ModNone, Description: "next tab", Handler: gui.nextManagedTab},
		{ViewName: "managed", Key: '[', Mod: gocui.ModNone, Description: "previous tab", Handler: gui.prevManagedTab},
		{ViewName: "managed", Key: 'e', Mod: gocui.ModNone, Description: "edit source file / template", Handler: gui.editFile},
		{ViewName: "managed", Key: gocui.KeyEnter, Mod: gocui.ModNone, Description: "expand/collapse directory", Handler: gui.toggleCollapse},
		{ViewName: "managed", Key: 'D', Mod: gocui.ModNone, Description: "forget file (stop tracking)", Handler: gui.forgetFile},

		// --- scripts panel ---
		{ViewName: "scripts", Key: 'j', Mod: gocui.ModNone, Description: "move down", Tag: "navigation", Handler: gui.navigateDown},
		{ViewName: "scripts", Key: gocui.KeyArrowDown, Mod: gocui.ModNone, Handler: gui.navigateDown},
		{ViewName: "scripts", Key: 'k', Mod: gocui.ModNone, Description: "move up", Tag: "navigation", Handler: gui.navigateUp},
		{ViewName: "scripts", Key: gocui.KeyArrowUp, Mod: gocui.ModNone, Handler: gui.navigateUp},
		{ViewName: "scripts", Key: ']', Mod: gocui.ModNone, Description: "next tab", Handler: gui.nextScriptsTab},
		{ViewName: "scripts", Key: '[', Mod: gocui.ModNone, Description: "previous tab", Handler: gui.prevScriptsTab},
		{ViewName: "scripts", Key: 'e', Mod: gocui.ModNone, Description: "edit script / data file", Handler: gui.editFile},
		{ViewName: "scripts", Key: gocui.KeyEnter, Mod: gocui.ModNone, Description: "expand/collapse directory", Handler: gui.toggleCollapse},

		// --- search: open ---
		{ViewName: "changed", Key: '/', Mod: gocui.ModNone, Description: "search", Handler: gui.openSearch},
		{ViewName: "managed", Key: '/', Mod: gocui.ModNone, Description: "search", Handler: gui.openSearch},
		{ViewName: "scripts", Key: '/', Mod: gocui.ModNone, Description: "search", Handler: gui.openSearch},

		// --- search: navigate panel while bar has focus (bypass searchEditor) ---
		{ViewName: "searchbar", Key: 'j', Mod: gocui.ModNone, Handler: gui.navigateDown},
		{ViewName: "searchbar", Key: gocui.KeyArrowDown, Mod: gocui.ModNone, Handler: gui.navigateDown},
		{ViewName: "searchbar", Key: 'k', Mod: gocui.ModNone, Handler: gui.navigateUp},
		{ViewName: "searchbar", Key: gocui.KeyArrowUp, Mod: gocui.ModNone, Handler: gui.navigateUp},
	}

	for _, b := range bs {
		if err := gui.registerBinding(b); err != nil {
			return err
		}
	}
	return nil
}

// navigateDown moves the cursor down one line in the focused panel and updates
// the preview pane to reflect the newly selected item. When called from the
// "searchbar" view, it resolves to the panel behind the bar.
func (gui *Gui) navigateDown(g *gocui.Gui, v *gocui.View) error {
	pv, panel := gui.resolvePanel(g, v)
	if pv == nil {
		return nil
	}
	switch panel {
	case "changed":
		if gui.changedTab == 1 {
			gui.scrollDown(pv, &gui.fsIdx, len(gui.fsFlat))
		} else {
			gui.scrollDown(pv, &gui.changedIdx, len(gui.changedFlat))
		}
	case "managed":
		if gui.managedTab == 1 {
			gui.scrollDown(pv, &gui.tmplIdx, len(gui.tmplFlat))
		} else {
			gui.scrollDown(pv, &gui.managedIdx, len(gui.managedFlat))
		}
	case "scripts":
		if gui.scriptsTab == 1 {
			gui.scrollDown(pv, &gui.srcIdx, len(gui.srcFlat))
		} else {
			gui.scrollDown(pv, &gui.scriptIdx, len(gui.scripts))
		}
	}
	return gui.updatePreview(g)
}

// navigateUp moves the cursor up one line in the focused panel and updates the
// preview pane. When called from "searchbar", it resolves to the panel behind.
func (gui *Gui) navigateUp(g *gocui.Gui, v *gocui.View) error {
	pv, panel := gui.resolvePanel(g, v)
	if pv == nil {
		return nil
	}
	switch panel {
	case "changed":
		if gui.changedTab == 1 {
			gui.scrollUp(pv, &gui.fsIdx)
		} else {
			gui.scrollUp(pv, &gui.changedIdx)
		}
	case "managed":
		if gui.managedTab == 1 {
			gui.scrollUp(pv, &gui.tmplIdx)
		} else {
			gui.scrollUp(pv, &gui.managedIdx)
		}
	case "scripts":
		if gui.scriptsTab == 1 {
			gui.scrollUp(pv, &gui.srcIdx)
		} else {
			gui.scrollUp(pv, &gui.scriptIdx)
		}
	}
	return gui.updatePreview(g)
}

// resolvePanel returns the real panel view and name for navigation purposes.
// When called from "searchbar", it returns the panel behind the bar so that
// j/k while typing navigate the list rather than the (invisible) editor view.
func (gui *Gui) resolvePanel(g *gocui.Gui, v *gocui.View) (*gocui.View, string) {
	if v.Name() == "searchbar" {
		pv, err := g.View(gui.searchPanel)
		if err != nil {
			return nil, ""
		}
		return pv, gui.searchPanel
	}
	return v, v.Name()
}

// nextPanel cycles focus to the next left panel and refreshes the preview.
func (gui *Gui) nextPanel(g *gocui.Gui, v *gocui.View) error {
	gui.currentPanelIdx = (gui.currentPanelIdx + 1) % len(leftPanels)
	gui.currentPanel = leftPanels[gui.currentPanelIdx]
	if _, err := g.SetCurrentView(gui.currentPanel); err != nil {
		return err
	}
	return gui.updatePreview(g)
}

// prevPanel cycles focus to the previous left panel and refreshes the preview.
func (gui *Gui) prevPanel(g *gocui.Gui, v *gocui.View) error {
	gui.currentPanelIdx = (gui.currentPanelIdx - 1 + len(leftPanels)) % len(leftPanels)
	gui.currentPanel = leftPanels[gui.currentPanelIdx]
	if _, err := g.SetCurrentView(gui.currentPanel); err != nil {
		return err
	}
	return gui.updatePreview(g)
}

// --- cursor helpers ---

// scrollDown moves the view cursor down by one line, scrolling origin if needed.
// idx is incremented and clamped to [0, total).
func (gui *Gui) scrollDown(v *gocui.View, idx *int, total int) {
	if total == 0 || *idx >= total-1 {
		return
	}
	*idx++
	cx, cy := v.Cursor()
	ox, oy := v.Origin()
	_, vh := v.InnerSize()
	if cy < vh-1 {
		v.SetCursor(cx, cy+1)
	} else {
		v.SetOrigin(ox, oy+1)
	}
}

// scrollUp moves the view cursor up by one line, scrolling origin if needed.
func (gui *Gui) scrollUp(v *gocui.View, idx *int) {
	if *idx == 0 {
		return
	}
	*idx--
	cx, cy := v.Cursor()
	ox, oy := v.Origin()
	if cy > 0 {
		v.SetCursor(cx, cy-1)
	} else if oy > 0 {
		v.SetOrigin(ox, oy-1)
	}
}
