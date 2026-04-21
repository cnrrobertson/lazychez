package gui

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"lazychez/pkg/filetree"
)

// FsEntry is one visible row in the filesystem explorer tab.
type FsEntry struct {
	Name  string // basename
	Path  string // home-relative (e.g. ".config/nvim")
	IsDir bool
	Depth int
}

// rebuildFsFlat rebuilds gui.fsFlat by walking gui.fsRoot, respecting fsExpanded.
func (gui *Gui) rebuildFsFlat() {
	gui.fsFlat = buildFsEntries(gui.fsRoot, gui.fsRoot, 0, gui.fsExpanded)
}

// buildFsEntries recursively builds a flat list of visible entries.
// Dirs not in expanded are listed but not recursed into (collapsed by default).
func buildFsEntries(root, dir string, depth int, expanded map[string]bool) []FsEntry {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	// Dirs first, then files — both groups sorted alphabetically.
	sort.Slice(entries, func(i, j int) bool {
		di, dj := entries[i].IsDir(), entries[j].IsDir()
		if di != dj {
			return di // dirs before files
		}
		return entries[i].Name() < entries[j].Name()
	})
	var result []FsEntry
	for _, e := range entries {
		rel, err := filepath.Rel(root, filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		fe := FsEntry{Name: e.Name(), Path: rel, IsDir: e.IsDir(), Depth: depth}
		result = append(result, fe)
		if e.IsDir() && expanded[rel] {
			children := buildFsEntries(root, filepath.Join(dir, e.Name()), depth+1, expanded)
			result = append(result, children...)
		}
	}
	return result
}

// rebuildSrcFlat rebuilds gui.srcFlat by walking the chezmoi source directory,
// respecting srcExpanded. Only entries whose name starts with ".chezmoi" are
// shown at depth 0 (the source root), filtering out encoded source files that
// already appear in the Managed and Scripts panels. Children of expanded dirs
// are shown in full.
func (gui *Gui) rebuildSrcFlat() {
	if gui.srcRoot == "" {
		return
	}
	gui.srcFlat = buildSrcEntries(gui.srcRoot, gui.srcRoot, 0, gui.srcExpanded)
}

// buildSrcEntries is like buildFsEntries but applies the chezmoi special-file
// filter at depth 0: only entries whose name starts with ".chezmoi" are listed
// at the source root. Deeper levels are unfiltered so data files inside
// .chezmoidata/ etc. are all visible.
func buildSrcEntries(root, dir string, depth int, expanded map[string]bool) []FsEntry {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	sort.Slice(entries, func(i, j int) bool {
		di, dj := entries[i].IsDir(), entries[j].IsDir()
		if di != dj {
			return di
		}
		return entries[i].Name() < entries[j].Name()
	})
	var result []FsEntry
	for _, e := range entries {
		// At source root: only show chezmoi special files/dirs.
		if depth == 0 && !strings.HasPrefix(e.Name(), ".chezmoi") {
			continue
		}
		rel, err := filepath.Rel(root, filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		fe := FsEntry{Name: e.Name(), Path: rel, IsDir: e.IsDir(), Depth: depth}
		result = append(result, fe)
		if e.IsDir() && expanded[rel] {
			children := buildSrcEntries(root, filepath.Join(dir, e.Name()), depth+1, expanded)
			result = append(result, children...)
		}
	}
	return result
}

// collectTmplPaths walks the chezmoi source directory and returns all
// source-relative paths of .tmpl files. Config templates (.chezmoi*.tmpl) at
// the source root are excluded — they are already visible in Scripts > Data.
func collectTmplPaths(srcRoot string) []string {
	var paths []string
	_ = filepath.WalkDir(srcRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		name := d.Name()
		if !strings.HasSuffix(name, ".tmpl") {
			return nil
		}
		rel, relErr := filepath.Rel(srcRoot, p)
		if relErr != nil {
			return nil
		}
		// Exclude .chezmoi*.tmpl directly at the source root (config templates).
		if !strings.Contains(rel, string(filepath.Separator)) && strings.HasPrefix(name, ".chezmoi") {
			return nil
		}
		// Use forward slashes for filetree.BuildTree compatibility.
		paths = append(paths, filepath.ToSlash(rel))
		return nil
	})
	return paths
}

// rebuildTmplTree rebuilds gui.tmplPaths, gui.tmplTree, and gui.tmplFlat by
// walking the chezmoi source directory for .tmpl files and feeding them into
// the filetree package so the Templates tab gets the same collapsible tree UX
// as the Managed and Changed panels.
func (gui *Gui) rebuildTmplTree() {
	if gui.srcRoot == "" {
		return
	}
	gui.tmplPaths = collectTmplPaths(gui.srcRoot)
	gui.tmplTree = filetree.BuildTree(gui.tmplPaths)
	gui.tmplFlat = filetree.Flatten(gui.tmplTree, gui.tmplCollapsed)
}

// renderFsRow returns the display string for one filesystem entry.
func renderFsRow(fe FsEntry, expanded map[string]bool) string {
	indent := strings.Repeat("  ", fe.Depth)
	if fe.IsDir {
		if expanded[fe.Path] {
			return fmt.Sprintf("%s\x1b[34m▼ %s\x1b[0m", indent, fe.Name)
		}
		return fmt.Sprintf("%s\x1b[34m▶ %s\x1b[0m", indent, fe.Name)
	}
	return fmt.Sprintf("%s  %s", indent, fe.Name)
}
