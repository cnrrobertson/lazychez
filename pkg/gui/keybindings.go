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

		// --- changed panel ---
		{ViewName: "changed", Key: 'j', Mod: gocui.ModNone, Description: "move down", Tag: "navigation", Handler: gui.navigateDown},
		{ViewName: "changed", Key: gocui.KeyArrowDown, Mod: gocui.ModNone, Handler: gui.navigateDown},
		{ViewName: "changed", Key: 'k', Mod: gocui.ModNone, Description: "move up", Tag: "navigation", Handler: gui.navigateUp},
		{ViewName: "changed", Key: gocui.KeyArrowUp, Mod: gocui.ModNone, Handler: gui.navigateUp},
		{ViewName: "changed", Key: 'a', Mod: gocui.ModNone, Description: "apply this file", Handler: gui.applyFile},
		{ViewName: "changed", Key: 'A', Mod: gocui.ModNone, Description: "apply all files", Handler: gui.applyAll},
		{ViewName: "changed", Key: 'e', Mod: gocui.ModNone, Description: "edit source file", Handler: gui.editFile},
		{ViewName: "changed", Key: '+', Mod: gocui.ModNone, Description: "add file to chezmoi", Handler: gui.addFile},
		{ViewName: "changed", Key: 'R', Mod: gocui.ModNone, Description: "re-add (pull target → source)", Handler: gui.reAddFile},
		{ViewName: "changed", Key: gocui.KeyEnter, Mod: gocui.ModNone, Description: "expand/collapse directory", Handler: gui.toggleCollapse},

		// --- managed panel ---
		{ViewName: "managed", Key: 'j', Mod: gocui.ModNone, Description: "move down", Tag: "navigation", Handler: gui.navigateDown},
		{ViewName: "managed", Key: gocui.KeyArrowDown, Mod: gocui.ModNone, Handler: gui.navigateDown},
		{ViewName: "managed", Key: 'k', Mod: gocui.ModNone, Description: "move up", Tag: "navigation", Handler: gui.navigateUp},
		{ViewName: "managed", Key: gocui.KeyArrowUp, Mod: gocui.ModNone, Handler: gui.navigateUp},
		{ViewName: "managed", Key: 'e', Mod: gocui.ModNone, Description: "edit source file", Handler: gui.editFile},
		{ViewName: "managed", Key: gocui.KeyEnter, Mod: gocui.ModNone, Description: "expand/collapse directory", Handler: gui.toggleCollapse},

		// --- scripts panel ---
		{ViewName: "scripts", Key: 'j', Mod: gocui.ModNone, Description: "move down", Tag: "navigation", Handler: gui.navigateDown},
		{ViewName: "scripts", Key: gocui.KeyArrowDown, Mod: gocui.ModNone, Handler: gui.navigateDown},
		{ViewName: "scripts", Key: 'k', Mod: gocui.ModNone, Description: "move up", Tag: "navigation", Handler: gui.navigateUp},
		{ViewName: "scripts", Key: gocui.KeyArrowUp, Mod: gocui.ModNone, Handler: gui.navigateUp},
		{ViewName: "scripts", Key: 'e', Mod: gocui.ModNone, Description: "edit script", Handler: gui.editFile},
	}

	for _, b := range bs {
		if err := gui.registerBinding(b); err != nil {
			return err
		}
	}
	return nil
}

// navigateDown moves the cursor down one line in the focused panel and updates
// the preview pane to reflect the newly selected item.
func (gui *Gui) navigateDown(g *gocui.Gui, v *gocui.View) error {
	switch v.Name() {
	case "changed":
		gui.scrollDown(v, &gui.changedIdx, len(gui.changedFlat))
	case "managed":
		gui.scrollDown(v, &gui.managedIdx, len(gui.managedFlat))
	case "scripts":
		gui.scrollDown(v, &gui.scriptIdx, len(gui.scripts))
	}
	return gui.updatePreview(g)
}

// navigateUp moves the cursor up one line in the focused panel and updates the
// preview pane.
func (gui *Gui) navigateUp(g *gocui.Gui, v *gocui.View) error {
	switch v.Name() {
	case "changed":
		gui.scrollUp(v, &gui.changedIdx)
	case "managed":
		gui.scrollUp(v, &gui.managedIdx)
	case "scripts":
		gui.scrollUp(v, &gui.scriptIdx)
	}
	return gui.updatePreview(g)
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
