package gui

import (
	"fmt"

	"github.com/jesseduffield/gocui"
	"lazychez/pkg/boxlayout"
)

// layout is the gocui manager function. It is called on every resize event and
// on the first render. It uses boxlayout to compute panel coordinates and
// creates/updates the five gocui views.
func (gui *Gui) layout(g *gocui.Gui) error {
	width, height := g.Size()
	if width == 0 || height == 0 {
		return nil
	}

	dims := boxlayout.ArrangeWindows(&boxlayout.Box{
		Direction: boxlayout.COLUMN,
		Children: []*boxlayout.Box{
			{
				// Left column — three stacked list panels.
				Direction: boxlayout.ROW,
				Weight:    40,
				Children: []*boxlayout.Box{
					{Window: "changed", Weight: 1},
					{Window: "managed", Weight: 1},
					{Window: "scripts", Weight: 1},
				},
			},
			{
				// Right column — preview (tall) + console (short).
				Direction: boxlayout.ROW,
				Weight:    80,
				Children: []*boxlayout.Box{
					{Window: "preview", Weight: 3},
					{Window: "console", Weight: 1},
				},
			},
		},
	}, 0, 0, width, height)

	for name, d := range dims {
		v, err := g.SetView(name, d.X0, d.Y0, d.X1, d.Y1, 0)
		if v == nil {
			return fmt.Errorf("setting view %q: %w", name, err)
		}
		gui.configureView(v, name)
	}

	if _, err := g.SetCurrentView(gui.currentPanel); err != nil {
		// Ignore on the very first frame before views are ready.
		_ = err
	}

	// Search bar — docked at the bottom of the active panel when visible.
	// Uses a framed view (same as helpsearch) so gocui draws proper border
	// characters, preventing the underlying panel content from bleeding through.
	if gui.searchVisible {
		d := dims[gui.searchPanel]
		sv, _ := g.SetView("searchbar", d.X0, d.Y1-3, d.X1, d.Y1-1, 0)
		if sv != nil {
			sv.Title = " / "
			sv.Editable = true
			sv.Editor = &searchEditor{gui: gui}
		}
		_, _ = g.SetViewOnTop("searchbar")
		if sv, err2 := g.View("searchbar"); err2 == nil {
			sv.Clear()
			fmt.Fprint(sv, gui.searchQuery)
		}
		_, _ = g.SetCurrentView("searchbar")
	} else {
		g.DeleteView("searchbar")
	}

	// Help overlay — rendered on top of everything else when visible.
	if gui.helpVisible {
		hw := min(64, width-4)
		hh := min(26, height-4)
		x0, y0 := (width-hw)/2, (height-hh)/2

		// Content area (above search bar).
		hv, _ := g.SetView("help", x0, y0, x0+hw, y0+hh-3, 0)
		if hv != nil {
			hv.Title = " Keybindings  (type · @key filters by key · esc closes) "
			hv.Wrap = false
		}

		// Single-line search bar at the bottom of the modal.
		sv, _ := g.SetView("helpsearch", x0, y0+hh-3, x0+hw, y0+hh-1, 0)
		if sv != nil {
			sv.Title = " filter "
			sv.Editable = true
			sv.Editor = &helpEditor{gui: gui}
		}

		// Always keep focus on the search bar while the overlay is open.
		_, _ = g.SetCurrentView("helpsearch")
		// Re-render content on every layout pass (triggered by each keypress).
		gui.renderHelp(g)
	}

	return nil
}

// configureView sets properties on a view that should persist across redraws.
// Safe to call on every layout pass — gocui is idempotent for these fields.
func (gui *Gui) configureView(v *gocui.View, name string) {
	switch name {
	case "changed":
		v.Tabs = []string{"Changes", "Files"}
		v.TabIndex = gui.changedTab
		if v.IsSearching() {
			v.Title = fmt.Sprintf(" ● Changed  /%s ", gui.searchQuery)
		} else {
			v.Title = " ● Changed "
		}
		v.Highlight = true
		v.HighlightInactive = true
		v.SelBgColor = gocui.ColorBlue
		v.SelFgColor = gocui.ColorWhite
		v.InactiveViewSelBgColor = gocui.NewRGBColor(40, 60, 100)

	case "managed":
		v.Tabs = []string{"Managed", "Templates"}
		v.TabIndex = gui.managedTab
		if v.IsSearching() {
			v.Title = fmt.Sprintf(" ✓ Managed  /%s ", gui.searchQuery)
		} else {
			v.Title = " ✓ Managed "
		}
		v.Highlight = true
		v.HighlightInactive = true
		v.SelBgColor = gocui.ColorBlue
		v.SelFgColor = gocui.ColorWhite
		v.InactiveViewSelBgColor = gocui.NewRGBColor(40, 60, 100)

	case "scripts":
		v.Tabs = []string{"Scripts", "Data"}
		v.TabIndex = gui.scriptsTab
		if v.IsSearching() {
			v.Title = fmt.Sprintf(" ⚙ Scripts  /%s ", gui.searchQuery)
		} else {
			v.Title = " ⚙ Scripts "
		}
		v.Highlight = true
		v.HighlightInactive = true
		v.SelBgColor = gocui.ColorBlue
		v.SelFgColor = gocui.ColorWhite
		v.InactiveViewSelBgColor = gocui.NewRGBColor(40, 60, 100)

	case "preview":
		v.Title = " Preview "
		v.Wrap = true

	case "console":
		v.Title = " Console "
		v.Wrap = true
		v.Autoscroll = true
	}
}
