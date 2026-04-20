package gui

import (
	"errors"
	"fmt"
	"time"

	"github.com/jesseduffield/gocui"
	"lazychez/pkg/chezmoi"
	"lazychez/pkg/filetree"
)

// leftPanels is the ordered list of focusable left-column panels.
var leftPanels = []string{"changed", "managed", "scripts"}

// Gui holds all runtime state for the lazychez TUI.
type Gui struct {
	g  *gocui.Gui
	cz *chezmoi.Client

	// --- data loaded from chezmoi ---
	changedFiles []*chezmoi.StatusFile
	managedFiles []*chezmoi.ManagedFile
	scripts      []*chezmoi.Script

	// --- tree state for "changed" panel ---
	changedTree      *filetree.TreeNode
	changedFlat      []filetree.FlatNode
	changedCollapsed map[string]bool

	// --- tree state for "managed" panel ---
	managedTree      *filetree.TreeNode
	managedFlat      []filetree.FlatNode
	managedCollapsed map[string]bool

	// --- selection indices (into flat lists for changed/managed; raw for scripts) ---
	changedIdx int
	managedIdx int
	scriptIdx  int

	// --- panel focus ---
	currentPanel    string // one of leftPanels
	currentPanelIdx int    // index into leftPanels

	// --- help overlay ---
	bindings    []*Binding
	helpVisible bool
	helpFilter  string

	// --- search ---
	searchVisible bool   // search bar is open
	searchPanel   string // panel being searched ("changed"|"managed"|"scripts")
	searchQuery   string // current typed query

	// --- async preview guard ---
	// previewGen is incremented on each preview request. The goroutine captures
	// a snapshot; if the snapshot no longer matches when it finishes, the result
	// is discarded (user navigated away). Same pattern as lazygithub.
	previewGen   int
	previewTimer *time.Timer // debounce timer; reset on every navigation event

	// glamourStyle is "dark" or "light", detected once before gocui starts.
	// Reserved for future syntax highlighting.
	glamourStyle string

}

// NewGui creates a new Gui bound to the given chezmoi client.
// glamourStyle should be "dark" or "light", detected by the caller before
// gocui initializes the terminal (to avoid TTY race with termenv).
func NewGui(cz *chezmoi.Client, glamourStyle string) *Gui {
	return &Gui{
		cz:               cz,
		currentPanel:     "changed",
		currentPanelIdx:  0,
		glamourStyle:     glamourStyle,
		changedCollapsed: make(map[string]bool),
		managedCollapsed: make(map[string]bool),
	}
}

// Run initialises gocui, wires up layout and keybindings, kicks off the
// initial data load, and starts the event loop. Returns a non-nil error only
// for unexpected gocui failures (not ErrQuit from a normal quit).
func (gui *Gui) Run() error {
	g, err := gocui.NewGui(gocui.NewGuiOpts{
		OutputMode:      gocui.OutputTrue,
		SupportOverlaps: false,
	})
	if err != nil {
		return fmt.Errorf("initialising terminal UI: %w", err)
	}
	defer g.Close()

	gui.g = g
	g.Highlight = true
	g.SelFgColor = gocui.ColorGreen

	// gocui defaults: SearchEscapeKey=Esc, NextSearchMatchKey='n', PrevSearchMatchKey='N'.
	// Hook OnSearchEscape so our searchQuery field stays in sync when the user
	// presses Esc to clear an active search on a panel.
	g.OnSearchEscape = func() error {
		gui.searchQuery = ""
		return nil
	}

	g.SetManagerFunc(gui.layout)

	if err := gui.setKeybindings(); err != nil {
		return fmt.Errorf("setting keybindings: %w", err)
	}

	// Load chezmoi data in the background — the UI is usable immediately.
	go gui.initialLoad()

	if err := g.MainLoop(); err != nil && !errors.Is(err, gocui.ErrQuit) {
		return err
	}
	return nil
}
