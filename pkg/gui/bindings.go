package gui

import (
	"fmt"
	"strings"

	"github.com/jesseduffield/gocui"
	"github.com/sahilm/fuzzy"
)

// Binding holds a keybinding together with the metadata needed to display it
// in the dynamic help overlay. Mirrors lazygit's types.Binding.
type Binding struct {
	ViewName    string                              // "" = global
	Key         interface{}                         // rune or gocui.Key
	Mod         gocui.Modifier
	Description string                              // "" = hidden from help menu
	Tag         string                              // "global" | "navigation" | "" (panel-local)
	Handler     func(*gocui.Gui, *gocui.View) error
}

// registerBinding stores b in gui.bindings and registers it with gocui.
// Use this instead of calling g.SetKeybinding directly.
func (gui *Gui) registerBinding(b *Binding) error {
	gui.bindings = append(gui.bindings, b)
	return gui.g.SetKeybinding(b.ViewName, b.Key, b.Mod, b.Handler)
}

// ── Fuzzy source ──────────────────────────────────────────────────────────────

// bindingSource implements fuzzy.Source over a []*Binding slice.
type bindingSource struct {
	items       []*Binding
	filterByKey bool // true when the query starts with "@"
}

func (s *bindingSource) String(i int) string {
	b := s.items[i]
	if s.filterByKey {
		return keyName(b.Key)
	}
	return b.Description
}

func (s *bindingSource) Len() int { return len(s.items) }

// ── Help content ──────────────────────────────────────────────────────────────

// helpLines returns the display lines for the help overlay, fuzzy-filtered by
// rawFilter. Mirrors lazygit's applyFilter + options_menu_action sectioning.
//
// If rawFilter starts with "@" the remainder is matched against key names;
// otherwise it is matched against descriptions.
func (gui *Gui) helpLines(currentPanel, rawFilter string) []string {
	filterByKey := strings.HasPrefix(rawFilter, "@")
	pattern := strings.TrimPrefix(rawFilter, "@")

	// Bucket visible bindings into sections.
	var local, global, nav []*Binding
	for _, b := range gui.bindings {
		if b.Description == "" {
			continue
		}
		switch b.Tag {
		case "navigation":
			nav = append(nav, b)
		case "global":
			global = append(global, b)
		default:
			if b.ViewName == currentPanel {
				local = append(local, b)
			}
		}
	}

	// filterSection applies fuzzy.FindFrom.
	filterSection := func(items []*Binding) []*Binding {
		if pattern == "" {
			return items
		}
		src := &bindingSource{items: items, filterByKey: filterByKey}
		matches := fuzzy.FindFrom(pattern, src)
		out := make([]*Binding, len(matches))
		for i, m := range matches {
			out[i] = items[m.Index]
		}
		return out
	}

	local = filterSection(local)
	global = filterSection(global)
	nav = filterSection(nav)

	// Build display lines with ANSI-coloured section headers.
	var result []string
	addSection := func(title string, items []*Binding) {
		if len(items) == 0 {
			return
		}
		if len(result) > 0 {
			result = append(result, "")
		}
		result = append(result, fmt.Sprintf("  \x1b[33m[ %s ]\x1b[0m", title))
		for _, b := range items {
			result = append(result, fmt.Sprintf("  %-12s %s", keyName(b.Key), b.Description))
		}
	}
	addSection(currentPanel, local)
	addSection("global", global)
	addSection("navigation", nav)
	return result
}

// ── Key name ──────────────────────────────────────────────────────────────────

// keyName returns a human-readable label for a key.
func keyName(key interface{}) string {
	switch k := key.(type) {
	case rune:
		return string(k)
	case gocui.Key:
		switch k {
		case gocui.KeyEnter:
			return "enter"
		case gocui.KeyTab:
			return "tab"
		case gocui.KeyBacktab:
			return "shift+tab"
		case gocui.KeyCtrlC:
			return "ctrl+c"
		case gocui.KeySpace:
			return "space"
		case gocui.KeyArrowUp:
			return "↑"
		case gocui.KeyArrowDown:
			return "↓"
		case gocui.KeyEsc:
			return "esc"
		case gocui.KeyBackspace, gocui.KeyBackspace2:
			return "backspace"
		default:
			return fmt.Sprintf("key(%d)", k)
		}
	}
	return "?"
}

// ── Help search editor ────────────────────────────────────────────────────────

// helpEditor implements gocui.Editor for the "helpsearch" input view.
// Each keystroke updates gui.helpFilter; layout() re-renders the help content
// on the next event cycle.
type helpEditor struct{ gui *Gui }

// Edit satisfies gocui.Editor.
func (e *helpEditor) Edit(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) bool {
	switch key {
	case gocui.KeyBackspace, gocui.KeyBackspace2:
		if len(e.gui.helpFilter) > 0 {
			runes := []rune(e.gui.helpFilter)
			e.gui.helpFilter = string(runes[:len(runes)-1])
		}
	case gocui.KeyEsc:
		e.gui.g.Update(func(g *gocui.Gui) error {
			return e.gui.hideHelp(g, nil)
		})
		return true
	default:
		if ch != 0 {
			e.gui.helpFilter += string(ch)
		}
	}
	// Refresh the search input display; layout() will call renderHelp() on the
	// next frame automatically (gocui calls all managers after every event).
	v.Clear()
	fmt.Fprint(v, e.gui.helpFilter)
	return true
}
