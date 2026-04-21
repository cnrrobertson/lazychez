package gui

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	chromaQuick "github.com/alecthomas/chroma/v2/quick"
	"github.com/jesseduffield/gocui"
	"lazychez/pkg/chezmoi"
	"lazychez/pkg/filetree"
)

// ── Panel renderers ───────────────────────────────────────────────────────────

// renderChanged populates the "changed" panel. Dispatches on changedTab:
// tab 0 = Changes (collapsible chezmoi status tree), tab 1 = Files (filesystem explorer).
func (gui *Gui) renderChanged(g *gocui.Gui) error {
	v, err := g.View("changed")
	if err != nil {
		return nil
	}
	v.Clear()
	switch gui.changedTab {
	case 1: // Files tab
		if len(gui.fsFlat) == 0 {
			fmt.Fprintln(v, "  \x1b[90m(empty)\x1b[0m")
			return nil
		}
		for _, fe := range gui.fsFlat {
			fmt.Fprintln(v, renderFsRow(fe, gui.fsExpanded))
		}
	default: // Changes tab
		if len(gui.changedFlat) == 0 {
			fmt.Fprintln(v, "  \x1b[90m(no changed files)\x1b[0m")
			return nil
		}
		for _, fn := range gui.changedFlat {
			var prefix string
			if !fn.Node.IsDir {
				prefix = statusIcon(gui.changedFiles[fn.Node.Index].Status)
			}
			fmt.Fprintln(v, filetree.RenderRow(fn, gui.changedCollapsed, prefix))
		}
	}
	return nil
}

// renderManaged populates the "managed" panel. Dispatches on managedTab:
// tab 0 = Managed (collapsible tree of managed files), tab 1 = Templates (.tmpl tree).
func (gui *Gui) renderManaged(g *gocui.Gui) error {
	v, err := g.View("managed")
	if err != nil {
		return nil
	}
	v.Clear()
	switch gui.managedTab {
	case 1: // Templates tab
		if len(gui.tmplFlat) == 0 {
			fmt.Fprintln(v, "  \x1b[90m(no templates)\x1b[0m")
			return nil
		}
		for _, fn := range gui.tmplFlat {
			fmt.Fprintln(v, filetree.RenderRow(fn, gui.tmplCollapsed, ""))
		}
	default: // Managed tab
		if len(gui.managedFlat) == 0 {
			fmt.Fprintln(v, "  \x1b[90m(no managed files)\x1b[0m")
			return nil
		}
		for _, fn := range gui.managedFlat {
			fmt.Fprintln(v, filetree.RenderRow(fn, gui.managedCollapsed, ""))
		}
	}
	return nil
}

// renderScripts populates the "scripts" panel. Dispatches on scriptsTab:
// tab 0 = Scripts list, tab 1 = Data (chezmoi special files explorer).
func (gui *Gui) renderScripts(g *gocui.Gui) error {
	v, err := g.View("scripts")
	if err != nil {
		return nil
	}
	v.Clear()
	switch gui.scriptsTab {
	case 1: // Data tab
		if len(gui.srcFlat) == 0 {
			fmt.Fprintln(v, "  \x1b[90m(empty)\x1b[0m")
			return nil
		}
		for _, fe := range gui.srcFlat {
			fmt.Fprintln(v, renderFsRow(fe, gui.srcExpanded))
		}
	default: // Scripts tab
		if len(gui.scripts) == 0 {
			fmt.Fprintln(v, "  \x1b[90m(no scripts)\x1b[0m")
			return nil
		}
		for _, s := range gui.scripts {
			fmt.Fprintf(v, "  \x1b[33m⚙\x1b[0m %s\n", truncate(s.Name, 48))
		}
	}
	return nil
}

// ── Preview dispatcher ────────────────────────────────────────────────────────

// updatePreview debounces preview refreshes so that rapid navigation (holding
// j/k) does not spawn a subprocess on every keypress. After the user pauses
// for previewDebounce the actual render is dispatched via g.Update.
const previewDebounce = 150 * time.Millisecond

func (gui *Gui) updatePreview(g *gocui.Gui) error {
	if gui.previewTimer != nil {
		gui.previewTimer.Stop()
	}
	gui.previewTimer = time.AfterFunc(previewDebounce, func() {
		g.Update(func(g *gocui.Gui) error {
			return gui.triggerPreview(g)
		})
	})
	return nil
}

// triggerPreview is the real preview dispatcher, called after the debounce
// delay. It mirrors the old updatePreview body exactly.
func (gui *Gui) triggerPreview(g *gocui.Gui) error {
	switch gui.currentPanel {
	case "changed":
		if gui.changedTab == 1 {
			// Files tab: preview real file content from disk.
			if gui.fsIdx < len(gui.fsFlat) {
				fe := gui.fsFlat[gui.fsIdx]
				if !fe.IsDir {
					gui.renderFsPreview(g, fe, gui.fsRoot)
					return nil
				}
			}
		} else {
			// Changes tab: chezmoi diff preview.
			if gui.changedIdx < len(gui.changedFlat) {
				fn := gui.changedFlat[gui.changedIdx]
				if !fn.Node.IsDir {
					gui.renderDiffPreview(g, gui.changedFiles[fn.Node.Index])
					return nil
				}
			}
		}

	case "managed":
		if gui.managedTab == 1 {
			// Templates tab: preview the raw source .tmpl file from disk.
			if gui.tmplIdx < len(gui.tmplFlat) {
				fn := gui.tmplFlat[gui.tmplIdx]
				if !fn.Node.IsDir {
					relPath := gui.tmplPaths[fn.Node.Index]
					fe := FsEntry{Name: filepath.Base(relPath), Path: relPath}
					gui.renderFsPreview(g, fe, gui.srcRoot)
					return nil
				}
			}
		} else {
			if gui.managedIdx < len(gui.managedFlat) {
				fn := gui.managedFlat[gui.managedIdx]
				if !fn.Node.IsDir {
					gui.renderCatPreview(g, gui.managedFiles[fn.Node.Index].Path, "managed")
					return nil
				}
			}
		}

	case "scripts":
		if gui.scriptsTab == 1 {
			// Data tab: preview source file content directly from disk.
			if gui.srcIdx < len(gui.srcFlat) {
				fe := gui.srcFlat[gui.srcIdx]
				if !fe.IsDir {
					gui.renderFsPreview(g, fe, gui.srcRoot)
					return nil
				}
			}
		} else {
			if gui.scriptIdx < len(gui.scripts) {
				gui.renderCatPreview(g, gui.scripts[gui.scriptIdx].Path, "script")
				return nil
			}
		}
	}

	return gui.clearPreview(g)
}

// renderDiffPreview shows `chezmoi diff <target>` as a colourised unified diff.
// Runs asynchronously so the main loop stays responsive.
func (gui *Gui) renderDiffPreview(g *gocui.Gui, f *chezmoi.StatusFile) {
	gui.previewGen++
	gen := gui.previewGen

	v, err := g.View("preview")
	if err != nil {
		return
	}
	v.Title = fmt.Sprintf(" diff: %s ", f.Path)
	v.Clear()
	fmt.Fprintln(v, "\x1b[90m  loading diff…\x1b[0m")

	go func() {
		diff, err := gui.cz.Diff(f.Path)

		gui.g.Update(func(g *gocui.Gui) error {
			if gui.previewGen != gen {
				return nil // stale — user navigated away
			}
			v, err2 := g.View("preview")
			if err2 != nil {
				return nil
			}
			v.Clear()
			if err != nil {
				fmt.Fprintf(v, "\x1b[31m%v\x1b[0m\n", err)
				return nil
			}
			if strings.TrimSpace(diff) == "" {
				fmt.Fprintln(v, "\x1b[90m  (no diff — file is up to date)\x1b[0m")
				return nil
			}
			fmt.Fprint(v, colorizeDiff(diff))
			return nil
		})
	}()
}

// renderFsPreview shows the raw content of a file in the filesystem explorer tab.
// Reads directly from disk (not via chezmoi cat) so it works for unmanaged files.
// Runs asynchronously; uses previewGen to discard stale results.
func (gui *Gui) renderFsPreview(g *gocui.Gui, fe FsEntry, root string) {
	gui.previewGen++
	gen := gui.previewGen
	fullPath := filepath.Join(root, fe.Path)

	v, err := g.View("preview")
	if err != nil {
		return
	}
	v.Title = fmt.Sprintf(" %s ", fe.Path)
	v.Clear()
	fmt.Fprintln(v, "\x1b[90m  loading…\x1b[0m")

	go func() {
		raw, readErr := os.ReadFile(fullPath)
		var highlighted string
		if readErr == nil && len(bytes.TrimSpace(raw)) > 0 {
			if h, hlErr := syntaxHighlight(fe.Name, string(raw), gui.glamourStyle); hlErr == nil {
				highlighted = h
			} else {
				highlighted = string(raw)
			}
		}
		gui.g.Update(func(g *gocui.Gui) error {
			if gui.previewGen != gen {
				return nil // stale — user navigated away
			}
			v, err2 := g.View("preview")
			if err2 != nil {
				return nil
			}
			v.Clear()
			if readErr != nil {
				fmt.Fprintf(v, "\x1b[31m%v\x1b[0m\n", readErr)
				return nil
			}
			if len(bytes.TrimSpace(raw)) == 0 {
				fmt.Fprintln(v, "\x1b[90m  (empty file)\x1b[0m")
				return nil
			}
			fmt.Fprint(v, highlighted)
			return nil
		})
	}()
}

// renderCatPreview shows `chezmoi cat <target>` — the rendered source content.
// kind is "managed" or "script", used only for the loading indicator message.
// Runs asynchronously.
func (gui *Gui) renderCatPreview(g *gocui.Gui, target, kind string) {
	gui.previewGen++
	gen := gui.previewGen

	v, err := g.View("preview")
	if err != nil {
		return
	}
	v.Title = fmt.Sprintf(" %s ", target)
	v.Clear()
	fmt.Fprintf(v, "\x1b[90m  loading %s…\x1b[0m\n", kind)

	go func() {
		content, err := gui.cz.Cat(target)

		// Run syntax highlighting off the main thread — chroma's regexp2-based
		// lexers can be slow (especially for fish/bash), and g.Update runs on
		// the event loop, so doing it here keeps the TUI responsive.
		var highlighted string
		if err == nil && strings.TrimSpace(content) != "" {
			if h, hlErr := syntaxHighlight(target, content, gui.glamourStyle); hlErr == nil {
				highlighted = h
			} else {
				highlighted = content // fallback: plain text
			}
		}

		gui.g.Update(func(g *gocui.Gui) error {
			if gui.previewGen != gen {
				return nil
			}
			v, err2 := g.View("preview")
			if err2 != nil {
				return nil
			}
			v.Clear()
			if err != nil {
				fmt.Fprintf(v, "\x1b[31m%v\x1b[0m\n", err)
				return nil
			}
			if strings.TrimSpace(content) == "" {
				fmt.Fprintln(v, "\x1b[90m  (empty file)\x1b[0m")
				return nil
			}
			fmt.Fprint(v, highlighted)
			return nil
		})
	}()
}

// clearPreview resets the preview pane to its default empty state.
func (gui *Gui) clearPreview(g *gocui.Gui) error {
	v, err := g.View("preview")
	if err != nil {
		return nil
	}
	v.Title = " Preview "
	v.Clear()
	return nil
}

// ── Help overlay renderer ─────────────────────────────────────────────────────

// renderHelp populates the "help" view with fuzzy-filtered keybinding lines.
// Called from layout() on every event cycle while the overlay is visible.
func (gui *Gui) renderHelp(g *gocui.Gui) error {
	hv, err := g.View("help")
	if err != nil {
		return nil
	}
	hv.Clear()
	for _, line := range gui.helpLines(gui.currentPanel, gui.helpFilter) {
		fmt.Fprintln(hv, line)
	}
	return nil
}

// ── Colour helpers ────────────────────────────────────────────────────────────

// colorizeDiff applies ANSI colours to a unified diff string.
func colorizeDiff(patch string) string {
	var sb strings.Builder
	for _, line := range strings.Split(patch, "\n") {
		switch {
		case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
			sb.WriteString("\x1b[1m" + line + "\x1b[0m\n")
		case strings.HasPrefix(line, "+"):
			sb.WriteString("\x1b[32m" + line + "\x1b[0m\n")
		case strings.HasPrefix(line, "-"):
			sb.WriteString("\x1b[31m" + line + "\x1b[0m\n")
		case strings.HasPrefix(line, "@@"):
			sb.WriteString("\x1b[36m" + line + "\x1b[0m\n")
		default:
			sb.WriteString(line + "\n")
		}
	}
	return sb.String()
}

// statusIcon returns a colourised single-char symbol for a chezmoi status code.
// The two-character status is "XY" where X = source state, Y = target state.
func statusIcon(status string) string {
	if len(status) == 0 {
		return "?"
	}
	// Prefer the source (X) state; fall back to destination (Y) state.
	ch := status[0]
	if ch == ' ' && len(status) > 1 {
		ch = status[1]
	}
	switch ch {
	case 'A':
		return "\x1b[32mA\x1b[0m" // green  — added
	case 'D':
		return "\x1b[31mD\x1b[0m" // red    — deleted
	case 'M':
		return "\x1b[34mM\x1b[0m" // blue   — modified
	case 'R':
		return "\x1b[33mR\x1b[0m" // yellow — run script
	default:
		return "\x1b[90m?\x1b[0m"
	}
}

// syntaxHighlight applies chroma syntax highlighting to content, detecting the
// language from the filename. glamourStyle should be "dark" or "light".
// Returns an error if chroma cannot highlight (caller should fall back to plain text).
func syntaxHighlight(filename, content, glamourStyle string) (string, error) {
	theme := "monokai"
	if glamourStyle == "light" {
		theme = "friendly"
	}
	var buf bytes.Buffer
	if err := chromaQuick.Highlight(&buf, content, filename, "terminal256", theme); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// truncate shortens s to at most n runes, appending "…" if trimmed.
func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n-1]) + "…"
}
