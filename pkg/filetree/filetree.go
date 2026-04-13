// Package filetree builds a collapsible directory tree from a flat list of
// slash-delimited paths and produces a cursor-navigable flat list that skips
// collapsed subtrees. Inspired by lazygit's pkg/gui/filetree package.
package filetree

import (
	"fmt"
	"sort"
	"strings"
)

const (
	expandedArrow  = "▼"
	collapsedArrow = "▶"
)

// TreeNode is one node in the directory tree.
// Directory nodes have IsDir=true, Index=-1, and Children populated.
// File nodes have IsDir=false, Index = position in the original flat slice.
type TreeNode struct {
	Name     string
	Path     string // full slash-joined path from root (used as the collapse map key)
	IsDir    bool
	Index    int // index into original []StatusFile / []ManagedFile; -1 for dirs
	Children []*TreeNode
}

// FlatNode is one entry in the cursor-navigable flat list produced by Flatten.
type FlatNode struct {
	Node  *TreeNode
	Depth int // visual indentation level; 0 = top-level
}

// BuildTree constructs a directory tree from a flat list of slash-delimited
// paths. The returned root is invisible — its Children are the top-level
// entries. Each file node stores its index into the original paths slice.
func BuildTree(paths []string) *TreeNode {
	root := &TreeNode{IsDir: true, Index: -1}
	for i, p := range paths {
		if p == "" {
			continue
		}
		parts := strings.Split(p, "/")
		insertNode(root, parts, i, "")
	}
	sortTree(root)
	compressTree(root)
	return root
}

// insertNode recursively inserts a file (and any missing intermediate
// directory nodes) into the subtree rooted at parent.
func insertNode(parent *TreeNode, parts []string, fileIdx int, parentPath string) {
	name := parts[0]
	fullPath := name
	if parentPath != "" {
		fullPath = parentPath + "/" + name
	}

	if len(parts) == 1 {
		// Leaf: file node.
		parent.Children = append(parent.Children, &TreeNode{
			Name:  name,
			Path:  fullPath,
			IsDir: false,
			Index: fileIdx,
		})
		return
	}

	// Intermediate directory: find existing or create a new one.
	for _, child := range parent.Children {
		if child.IsDir && child.Name == name {
			insertNode(child, parts[1:], fileIdx, fullPath)
			return
		}
	}
	dir := &TreeNode{Name: name, Path: fullPath, IsDir: true, Index: -1}
	parent.Children = append(parent.Children, dir)
	insertNode(dir, parts[1:], fileIdx, fullPath)
}

// sortTree sorts each node's children: directories first (alphabetically),
// then files (alphabetically). Applied recursively.
func sortTree(node *TreeNode) {
	sort.Slice(node.Children, func(i, j int) bool {
		a, b := node.Children[i], node.Children[j]
		if a.IsDir != b.IsDir {
			return a.IsDir // dirs before files
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})
	for _, child := range node.Children {
		if child.IsDir {
			sortTree(child)
		}
	}
}

// Flatten returns a depth-first flat list of all currently visible nodes,
// respecting the collapsed map. The root itself is never included. Children
// of collapsed directories are omitted entirely — this is what lets cursor
// navigation skip over hidden content without any special-casing.
func Flatten(root *TreeNode, collapsed map[string]bool) []FlatNode {
	var result []FlatNode
	for _, child := range root.Children {
		result = append(result, flattenNode(child, 0, collapsed)...)
	}
	return result
}

func flattenNode(node *TreeNode, depth int, collapsed map[string]bool) []FlatNode {
	result := []FlatNode{{Node: node, Depth: depth}}
	if node.IsDir && !collapsed[node.Path] {
		for _, child := range node.Children {
			result = append(result, flattenNode(child, depth+1, collapsed)...)
		}
	}
	return result
}

// compressTree squashes single-child directory chains into one node so that
// a path like a/b/c/ with no siblings renders as a single "a/b/c/" entry
// rather than three nested directories. Applied post-order (children first).
func compressTree(node *TreeNode) {
	// Compress children recursively first (post-order).
	for _, child := range node.Children {
		if child.IsDir {
			compressTree(child)
		}
	}

	// Squash any child directory that has exactly one child which is also a
	// directory. Repeat until the chain breaks.
	for _, child := range node.Children {
		if !child.IsDir {
			continue
		}
		for len(child.Children) == 1 && child.Children[0].IsDir {
			grandchild := child.Children[0]
			child.Name = child.Name + "/" + grandchild.Name
			child.Path = grandchild.Path
			child.Children = grandchild.Children
		}
	}
}

// RenderRow returns one terminal-ready line for fn with indentation and a
// collapse arrow for directories.
//
// statusPrefix is prepended before the file name for leaf nodes; pass "" for
// panels that don't show per-file status (e.g. the managed panel). The prefix
// is ignored for directory nodes.
func RenderRow(fn FlatNode, collapsed map[string]bool, statusPrefix string) string {
	indent := strings.Repeat("  ", fn.Depth)
	if fn.Node.IsDir {
		arrow := expandedArrow
		if collapsed[fn.Node.Path] {
			arrow = collapsedArrow
		}
		return fmt.Sprintf("%s\x1b[34m%s %s/\x1b[0m", indent, arrow, fn.Node.Name)
	}
	if statusPrefix != "" {
		return fmt.Sprintf("%s%s %s", indent, statusPrefix, fn.Node.Name)
	}
	return fmt.Sprintf("%s  %s", indent, fn.Node.Name)
}
