package api

import (
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type fsEntry struct {
	Name string `json:"name"`
	Dir  bool   `json:"dir"`
}

// fsListing is one directory as shown by the UI's folder and file browser.
type fsListing struct {
	Path string `json:"path"`
	// Parent is empty at the top of the file system.
	Parent  string    `json:"parent"`
	Entries []fsEntry `json:"entries"`
	// Shortcuts the browser offers as starting points.
	Home      string `json:"home"`
	Workspace string `json:"workspace"`
}

// listDir backs the folder and file browser: the UI runs in a web view and
// cannot see the file system itself. It lists sub-directories, plus files
// with the given extension when ext is set. Hidden entries are left out.
func (s *server) listDir(w http.ResponseWriter, r *http.Request) {
	home, _ := os.UserHomeDir()
	path := strings.TrimSpace(r.URL.Query().Get("path"))
	switch {
	case path == "":
		path = s.dir()
	case path == "~":
		path = home
	case strings.HasPrefix(path, "~/"), strings.HasPrefix(path, `~\`):
		path = filepath.Join(home, path[2:])
	}
	if !filepath.IsAbs(path) {
		http.Error(w, "enter the full path of a folder", http.StatusBadRequest)
		return
	}
	path = filepath.Clean(path)
	entries, err := os.ReadDir(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ext := strings.ToLower(r.URL.Query().Get("ext"))
	out := fsListing{Path: path, Entries: []fsEntry{}, Home: home, Workspace: s.dir()}
	if parent := filepath.Dir(path); parent != path {
		out.Parent = parent
	}
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		dir := e.IsDir()
		if !dir && e.Type()&os.ModeSymlink != 0 {
			// Follow links so a linked proto directory can be browsed.
			if info, err := os.Stat(filepath.Join(path, name)); err == nil {
				dir = info.IsDir()
			}
		}
		if dir || (ext != "" && strings.HasSuffix(strings.ToLower(name), ext)) {
			out.Entries = append(out.Entries, fsEntry{Name: name, Dir: dir})
		}
	}
	sort.SliceStable(out.Entries, func(i, j int) bool {
		a, b := out.Entries[i], out.Entries[j]
		if a.Dir != b.Dir {
			return a.Dir
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})
	respond(w)(out, nil)
}
