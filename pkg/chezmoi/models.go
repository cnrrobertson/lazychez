package chezmoi

import "path/filepath"

// StatusFile is one entry from `chezmoi status`.
// Status is the two-character XY code: X = source state, Y = destination state.
// Common values:
//
//	"M " — modified in source (chezmoi source differs from target)
//	" M" — target modified relative to chezmoi source
//	"A " — added in source (will be created in target on apply)
//	"D " — deleted in source (will be removed from target on apply)
//	"R " — run script (will execute on next apply)
//	"RM" — run script, modified
type StatusFile struct {
	Path   string // target path as reported by chezmoi (e.g. .bashrc)
	Status string // two-character status code
}

// ManagedFile is one entry from `chezmoi managed --include=files`.
type ManagedFile struct {
	Path string // relative target path (e.g. .bashrc or .config/nvim/init.vim)
	Name string // filepath.Base(Path) for compact display
}

// NewManagedFile constructs a ManagedFile from a raw path returned by chezmoi.
func NewManagedFile(path string) *ManagedFile {
	return &ManagedFile{Path: path, Name: filepath.Base(path)}
}

// Script is one entry from `chezmoi managed --include=scripts`.
type Script struct {
	Path string // path as reported by chezmoi
	Name string // filepath.Base(Path) for display
}

// NewScript constructs a Script from a raw path returned by chezmoi.
func NewScript(path string) *Script {
	return &Script{Path: path, Name: filepath.Base(path)}
}
