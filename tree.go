package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type node struct {
	name     string
	path     string // absolute
	rel      string // slash-separated, relative to root
	isDir    bool
	depth    int
	open     bool
	children []*node
	parent   *node
	err      string
}

// loadDir (re)reads a directory, keeping existing child nodes (and so their
// open/closed state), and recursively refreshes open subdirectories.
func loadDir(n *node) {
	entries, err := os.ReadDir(n.path)
	if err != nil {
		n.err = err.Error()
		n.children = nil
		return
	}
	n.err = ""
	old := make(map[string]*node, len(n.children))
	for _, c := range n.children {
		old[c.name] = c
	}
	kids := make([]*node, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if name == ".git" {
			continue
		}
		full := filepath.Join(n.path, name)
		dir := e.IsDir()
		if !dir && e.Type()&os.ModeSymlink != 0 {
			if fi, err := os.Stat(full); err == nil && fi.IsDir() {
				dir = true
			}
		}
		c := old[name]
		if c == nil || c.isDir != dir {
			rel := name
			if n.rel != "" {
				rel = n.rel + "/" + name
			}
			c = &node{name: name, path: full, rel: rel, isDir: dir, depth: n.depth + 1, parent: n}
		}
		kids = append(kids, c)
	}
	sort.Slice(kids, func(i, j int) bool {
		if kids[i].isDir != kids[j].isDir {
			return kids[i].isDir
		}
		return strings.ToLower(kids[i].name) < strings.ToLower(kids[j].name)
	})
	n.children = kids
	for _, c := range kids {
		if c.isDir && c.open {
			loadDir(c)
		}
	}
}

// flatten lists the visible rows (children of open directories).
func flatten(n *node, out []*node) []*node {
	for _, c := range n.children {
		out = append(out, c)
		if c.isDir && c.open {
			out = flatten(c, out)
		}
	}
	return out
}
