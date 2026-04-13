package gui

import (
	"errors"
	"fmt"

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

	// --- async preview guard ---
	// previewGen is incremented on each preview request. The goroutine captures
	// a snapshot; if the snapshot no longer matches when it finishes, the result
	// is discarded (user navigated away). Same pattern as lazygithub.
	previewGen int

	// glamourStyle is "dark" or "light", detected once before gocui starts.
	// Reserved for future syntax highlighting.
	glamourStyle string

	// --- pending edit ---
	// Set to a target path when the user presses 'e'. Run() returns this value
	// so app.Run() can suspend, run `chezmoi edit`, and restart the TUI.
	pendingEdit string
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
// initial data load, and starts the event loop.
//
// Returns (pendingEdit, err):
//   - pendingEdit is non-empty when the user pressed 'e' to edit a file;
//     the caller should run `chezmoi edit <pendingEdit>` then restart the TUI.
//   - err is non-nil only for unexpected gocui errors (not ErrQuit).
func (gui *Gui) Run() (string, error) {
	g, err := gocui.NewGui(gocui.NewGuiOpts{
		OutputMode:      gocui.OutputTrue,
		SupportOverlaps: false,
	})
	if err != nil {
		return "", fmt.Errorf("initialising terminal UI: %w", err)
	}
	defer g.Close()

	gui.g = g
	g.Highlight = true
	g.SelFgColor = gocui.ColorGreen

	g.SetManagerFunc(gui.layout)

	if err := gui.setKeybindings(); err != nil {
		return "", fmt.Errorf("setting keybindings: %w", err)
	}

	// Load chezmoi data in the background — the UI is usable immediately.
	go gui.initialLoad()

	if err := g.MainLoop(); err != nil && !errors.Is(err, gocui.ErrQuit) {
		return "", err
	}
	return gui.pendingEdit, nil
}
