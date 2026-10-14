// Package store persists the workspace as plain files meant to live in a git
// repository: one YAML file per request, one directory per folder.
//
//	<workspace>/collections/<collection>/_collection.yaml   (id, name, proto config)
//	<workspace>/collections/<collection>/<request>.yaml
//	<workspace>/collections/<collection>/<folder>/_folder.yaml
//
// YAML keeps multi-line bodies and queries as block scalars, so a change to
// one line of a body is a one-line diff. Everything is read from disk on each
// call so a `git pull` shows up without a restart. Environments and history are
// per-machine and live as JSON in the local directory, outside the workspace.
package store

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"

	"go.yaml.in/yaml/v3"

	"roundtrip/internal/model"
)

const (
	historyLimit   = 200
	keptBodyLimit  = 1 << 20
	ext            = ".yaml"
	collectionMeta = "_collection" + ext
	folderMeta     = "_folder" + ext
)

type Store struct {
	dir   string
	local string
	mu    sync.Mutex
}

// Proto is only used by collections.
type meta struct {
	ID    string             `json:"id" yaml:"id"`
	Name  string             `json:"name" yaml:"name"`
	Proto *model.ProtoConfig `json:"proto,omitempty" yaml:"proto,omitempty"`
}

type requestFile struct {
	ID            string `json:"id" yaml:"id"`
	Name          string `json:"name" yaml:"name"`
	model.Request `yaml:",inline"`
}

func Open(dir, local string) (*Store, error) {
	for _, d := range []string{dir, local} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			return nil, err
		}
	}
	return &Store{dir: dir, local: local}, nil
}

// OpenReadOnly opens an existing workspace without creating anything, for
// callers that only read (the command line runner).
func OpenReadOnly(dir, local string) (*Store, error) {
	if info, err := os.Stat(dir); err != nil {
		return nil, err
	} else if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", dir)
	}
	return &Store{dir: dir, local: local}, nil
}

// LocalDir is where the per-machine data of a workspace lives under dataDir.
func LocalDir(dataDir, workspace string) string {
	sum := sha1.Sum([]byte(workspace))
	return filepath.Join(dataDir, "local", hex.EncodeToString(sum[:6]))
}

// ---- collections ----

func (s *Store) Collections() ([]*model.Collection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	root := filepath.Join(s.dir, "collections")
	out := []*model.Collection{}
	for _, name := range subdirs(root) {
		dir := filepath.Join(root, name)
		m, err := s.readMeta(dir, collectionMeta)
		if err != nil {
			return nil, err
		}
		items, err := s.readItems(dir)
		if err != nil {
			return nil, err
		}
		if m.Proto.Empty() {
			m.Proto = nil
		}
		out = append(out, &model.Collection{ID: m.ID, Name: m.Name, Proto: m.Proto, Items: items})
	}
	sort.SliceStable(out, func(i, j int) bool { return lower(out[i].Name) < lower(out[j].Name) })
	return out, nil
}

// SaveCollection drops an empty proto config from c so it is neither written
// nor echoed back.
func (s *Store) SaveCollection(c *model.Collection) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c.Proto.Empty() {
		c.Proto = nil
	}
	root := filepath.Join(s.dir, "collections")
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	old := ""
	used := map[string]bool{}
	for _, name := range subdirs(root) {
		m, err := s.readMeta(filepath.Join(root, name), collectionMeta)
		if err != nil {
			return err
		}
		if m.ID == c.ID {
			old = name
		} else {
			used[lower(name)] = true
		}
	}
	name := uniqueName(sanitize(c.Name), used)
	if old != "" && old != name {
		if err := os.Rename(filepath.Join(root, old), filepath.Join(root, name)); err != nil {
			return err
		}
	}
	return writeDir(filepath.Join(root, name), collectionMeta, meta{ID: c.ID, Name: c.Name, Proto: c.Proto}, c.Items)
}

func (s *Store) DeleteCollection(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	root := filepath.Join(s.dir, "collections")
	for _, name := range subdirs(root) {
		m, err := s.readMeta(filepath.Join(root, name), collectionMeta)
		if err != nil {
			return err
		}
		if m.ID == id {
			return os.RemoveAll(filepath.Join(root, name))
		}
	}
	return nil
}

func (s *Store) readMeta(dir, file string) (meta, error) {
	m := meta{}
	if err := readYAML(filepath.Join(dir, file), &m); err != nil {
		return m, err
	}
	if m.ID == "" {
		m.ID = s.pathID(dir)
	}
	if m.Name == "" {
		m.Name = filepath.Base(dir)
	}
	return m, nil
}

// readItems returns folders first, then requests, each sorted by name.
func (s *Store) readItems(dir string) ([]*model.Item, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	folders, requests := []*model.Item{}, []*model.Item{}
	for _, e := range entries {
		name := e.Name()
		path := filepath.Join(dir, name)
		switch {
		case strings.HasPrefix(name, "."), strings.HasPrefix(name, "_"):
		case e.IsDir():
			m, err := s.readMeta(path, folderMeta)
			if err != nil {
				return nil, err
			}
			children, err := s.readItems(path)
			if err != nil {
				return nil, err
			}
			folders = append(folders, &model.Item{ID: m.ID, Name: m.Name, Items: children})
		case strings.HasSuffix(name, ext):
			rf := requestFile{}
			if err := readYAML(path, &rf); err != nil {
				return nil, err
			}
			if rf.ID == "" {
				rf.ID = s.pathID(path)
			}
			if rf.Name == "" {
				rf.Name = strings.TrimSuffix(name, ext)
			}
			req := rf.Request
			requests = append(requests, &model.Item{ID: rf.ID, Name: rf.Name, Request: &req})
		}
	}
	for _, list := range [][]*model.Item{folders, requests} {
		sort.SliceStable(list, func(i, j int) bool { return lower(list[i].Name) < lower(list[j].Name) })
	}
	return append(folders, requests...), nil
}

// withoutDefaults drops what a request file does not need to say because it
// is the default anyway: no auth, an API key in a header, an empty body and
// its content type. The editor fills these in, so without this every saved
// request would carry them.
func withoutDefaults(r model.Request) model.Request {
	if r.Auth.In == "header" {
		r.Auth.In = ""
	}
	if r.Auth == (model.Auth{Type: "none"}) {
		r.Auth = model.Auth{}
	}
	b := r.Body
	if (b.Mode == "none" || b.Mode == "") && b.Raw == "" && len(b.Form) == 0 && b.Variables == "" {
		r.Body = model.Body{}
	}
	return r
}

// writeDir syncs dir to match items. Stale request files and folders are
// removed; files this app doesn't own (READMEs, dotfiles) are left alone.
func writeDir(dir, metaFile string, m meta, items []*model.Item) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := writeYAML(filepath.Join(dir, metaFile), m); err != nil {
		return err
	}
	keep := map[string]bool{lower(metaFile): true}
	usedDirs, usedFiles := map[string]bool{}, map[string]bool{}
	for _, it := range items {
		if it.Request == nil {
			name := uniqueName(sanitize(it.Name), usedDirs)
			keep[lower(name)] = true
			if err := writeDir(filepath.Join(dir, name), folderMeta, meta{ID: it.ID, Name: it.Name}, it.Items); err != nil {
				return err
			}
			continue
		}
		name := uniqueName(sanitize(it.Name), usedFiles) + ext
		keep[lower(name)] = true
		if err := writeYAML(filepath.Join(dir, name), requestFile{ID: it.ID, Name: it.Name, Request: withoutDefaults(*it.Request)}); err != nil {
			return err
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") || keep[lower(name)] {
			continue
		}
		if e.IsDir() || strings.HasSuffix(name, ext) {
			if err := os.RemoveAll(filepath.Join(dir, name)); err != nil {
				return err
			}
		}
	}
	return nil
}

// ---- environments ----

// Environments are per-machine: they hold URLs and credentials that differ
// per person, so they live in the local directory and never in the workspace.

func (s *Store) Environments() ([]*model.Environment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.environments()
}

func (s *Store) Environment(id string) (*model.Environment, error) {
	envs, err := s.Environments()
	for _, e := range envs {
		if e.ID == id {
			return e, nil
		}
	}
	return nil, err
}

func (s *Store) environments() ([]*model.Environment, error) {
	envs := []*model.Environment{}
	if err := readJSON(filepath.Join(s.local, "environments.json"), &envs); err != nil {
		return nil, err
	}
	for _, e := range envs {
		if e.Variables == nil {
			e.Variables = []model.KV{}
		}
	}
	sort.SliceStable(envs, func(i, j int) bool { return lower(envs[i].Name) < lower(envs[j].Name) })
	return envs, nil
}

func (s *Store) SaveEnvironment(e *model.Environment) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	envs, err := s.environments()
	if err != nil {
		return err
	}
	i := slices.IndexFunc(envs, func(o *model.Environment) bool { return o.ID == e.ID })
	if i < 0 {
		envs = append(envs, e)
	} else {
		envs[i] = e
	}
	return writeJSON(filepath.Join(s.local, "environments.json"), envs)
}

func (s *Store) DeleteEnvironment(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	envs, err := s.environments()
	if err != nil {
		return err
	}
	envs = slices.DeleteFunc(envs, func(o *model.Environment) bool { return o.ID == id })
	return writeJSON(filepath.Join(s.local, "environments.json"), envs)
}

// ---- history ----

// History returns entries newest first.
func (s *Store) History() ([]*model.HistoryEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.history()
}

func (s *Store) history() ([]*model.HistoryEntry, error) {
	h := []*model.HistoryEntry{}
	err := readJSON(filepath.Join(s.local, "history.json"), &h)
	return h, err
}

// AddHistory records e and keeps res, so it can be shown again later. A body
// over keptBodyLimit is not kept; the rest of the response is.
func (s *Store) AddHistory(e *model.HistoryEntry, res *model.Response) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	h, err := s.history()
	if err != nil {
		return err
	}
	if res != nil {
		kept := *res
		if len(kept.Body) > keptBodyLimit {
			kept.Body, kept.BodyNotKept = "", true
		}
		if err := os.MkdirAll(filepath.Join(s.local, "responses"), 0o755); err != nil {
			return err
		}
		if err := writeJSON(s.responsePath(e.ID), kept); err != nil {
			return err
		}
	}
	h = append([]*model.HistoryEntry{e}, h...)
	if len(h) > historyLimit {
		for _, old := range h[historyLimit:] {
			os.Remove(s.responsePath(old.ID))
		}
		h = h[:historyLimit]
	}
	return writeJSON(filepath.Join(s.local, "history.json"), h)
}

// HistoryResponse is the response kept for a history entry, or an error
// matching fs.ErrNotExist when there is none.
func (s *Store) HistoryResponse(id string) (*model.Response, error) {
	if !validID(id) {
		return nil, fs.ErrNotExist
	}
	path := s.responsePath(id)
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	res := &model.Response{}
	return res, readJSON(path, res)
}

func (s *Store) ClearHistory() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.RemoveAll(filepath.Join(s.local, "responses")); err != nil {
		return err
	}
	return writeJSON(filepath.Join(s.local, "history.json"), []*model.HistoryEntry{})
}

func (s *Store) responsePath(id string) string {
	return filepath.Join(s.local, "responses", id+".json")
}

// validID accepts the ids model.NewID makes, so an id from a URL cannot point
// outside the responses directory.
func validID(id string) bool {
	if id == "" || len(id) > 64 {
		return false
	}
	for _, c := range id {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// ---- helpers ----

// pathID gives hand-written files without an "id" a stable identity.
func (s *Store) pathID(path string) string {
	rel, err := filepath.Rel(s.dir, path)
	if err != nil {
		rel = path
	}
	sum := sha1.Sum([]byte(filepath.ToSlash(rel)))
	return hex.EncodeToString(sum[:8])
}

func subdirs(root string) []string {
	var out []string
	entries, _ := os.ReadDir(root)
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			out = append(out, e.Name())
		}
	}
	return out
}

// readJSON leaves v untouched when the file does not exist.
func readJSON(path string, v any) error {
	return readFile(path, func(data []byte) error { return json.Unmarshal(data, v) })
}

// readYAML leaves v untouched when the file does not exist or is empty.
func readYAML(path string, v any) error {
	return readFile(path, func(data []byte) error {
		err := yaml.NewDecoder(bytes.NewReader(data)).Decode(v)
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	})
}

func readFile(path string, unmarshal func([]byte) error) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := unmarshal(data); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(path, append(data, '\n'))
}

// writeYAML writes what json.Marshal would (same names, same order, same
// omitted fields) as YAML, with multi-line strings as block scalars.
func writeYAML(path string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	node, err := yamlNode(dec)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(node); err != nil {
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}
	return writeFile(path, buf.Bytes())
}

// yamlNode converts the next JSON value. The encoder quotes strings that
// YAML would otherwise read as something else, and falls back to a quoted
// string when text cannot be a block scalar (trailing spaces, \r).
func yamlNode(dec *json.Decoder) (*yaml.Node, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch t := tok.(type) {
	case json.Delim:
		n := &yaml.Node{Kind: yaml.MappingNode}
		if t == '[' {
			n.Kind = yaml.SequenceNode
		}
		for dec.More() {
			child, err := yamlNode(dec)
			if err != nil {
				return nil, err
			}
			n.Content = append(n.Content, child)
		}
		_, err := dec.Token()
		return n, err
	case string:
		n := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: t}
		// The library writes a block scalar starting with a tab that it
		// cannot read back, so that one stays quoted.
		if strings.HasPrefix(t, "\t") {
			n.Style = yaml.DoubleQuotedStyle
		} else if strings.Contains(t, "\n") {
			n.Style = yaml.LiteralStyle
		}
		return n, nil
	case bool:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: strconv.FormatBool(t)}, nil
	case json.Number:
		return &yaml.Node{Kind: yaml.ScalarNode, Value: t.String()}, nil
	}
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"}, nil
}

// writeFile leaves a file with the same content alone.
func writeFile(path string, data []byte) error {
	if old, err := os.ReadFile(path); err == nil && string(old) == string(data) {
		return nil
	}
	if err := os.WriteFile(path+".tmp", data, 0o644); err != nil {
		return err
	}
	return os.Rename(path+".tmp", path)
}

var reservedName = regexp.MustCompile(`(?i)^(con|prn|aux|nul|com[1-9]|lpt[1-9])$`)

// sanitize turns a display name into a file name valid on macOS and Windows.
func sanitize(name string) string {
	name = strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(`<>:"/\|?*`, r) {
			return '-'
		}
		return r
	}, name)
	if rs := []rune(name); len(rs) > 100 {
		name = string(rs[:100])
	}
	name = strings.Trim(name, " ._")
	if name == "" {
		name = "untitled"
	}
	if reservedName.MatchString(name) {
		name += "-"
	}
	return name
}

// uniqueName avoids collisions on case-insensitive file systems.
func uniqueName(name string, used map[string]bool) string {
	candidate := name
	for n := 2; used[lower(candidate)]; n++ {
		candidate = name + " " + strconv.Itoa(n)
	}
	used[lower(candidate)] = true
	return candidate
}

func lower(s string) string { return strings.ToLower(s) }
