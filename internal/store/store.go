// Package store persists the workspace as plain files meant to live in a git
// repository: one YAML file per collection, holding its folders and requests.
//
//	<workspace>/collections/<collection>.yaml
//
// YAML keeps multi-line bodies and queries as block scalars, so a change to
// one line of a body is a one-line diff. Everything is read from disk on each
// call so a `git pull` shows up without a restart. Environments and history are
// per-machine and live as JSON in the local directory, outside the workspace.
//
// Workspaces from before the single file still read: a directory under
// collections/ with a _collection.yaml, a _folder.yaml per folder directory
// and one YAML file per request. Saving such a collection writes its single
// file and removes those files, leaving anything else in the directory.
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

// collectionFile is the content of collections/<collection>.yaml. Folders
// come before requests, each sorted by name, as the sidebar shows them.
type collectionFile struct {
	ID       string             `json:"id" yaml:"id"`
	Name     string             `json:"name" yaml:"name"`
	Proto    *model.ProtoConfig `json:"proto,omitempty" yaml:"proto,omitempty"`
	Folders  []folderFile       `json:"folders,omitempty" yaml:"folders,omitempty"`
	Requests []requestFile      `json:"requests,omitempty" yaml:"requests,omitempty"`
}

type folderFile struct {
	ID       string        `json:"id" yaml:"id"`
	Name     string        `json:"name" yaml:"name"`
	Folders  []folderFile  `json:"folders,omitempty" yaml:"folders,omitempty"`
	Requests []requestFile `json:"requests,omitempty" yaml:"requests,omitempty"`
}

// meta is _collection.yaml or _folder.yaml in the old layout. Proto is only
// used by collections.
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

// untitled names a hand-written request or folder that has no name.
const untitled = "Untitled"

// stored is a collection on disk: its single file, or a directory in the old
// layout.
type stored struct {
	name string
	dir  bool
	id   string
}

func (s *Store) Collections() ([]*model.Collection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	root := filepath.Join(s.dir, "collections")
	list, err := s.stored(root)
	if err != nil {
		return nil, err
	}
	files := map[string]bool{}
	for _, e := range list {
		if !e.dir {
			files[e.id] = true
		}
	}
	out := []*model.Collection{}
	for _, e := range list {
		path := filepath.Join(root, e.name)
		var c *model.Collection
		switch {
		case !e.dir:
			c, err = s.readCollectionFile(path)
		case !files[e.id]:
			c, err = s.readCollectionDir(path)
		default:
			continue
		}
		if err != nil {
			return nil, err
		}
		out = append(out, c)
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
	list, err := s.stored(root)
	if err != nil {
		return err
	}
	old := ""
	used := map[string]bool{}
	for _, e := range list {
		switch {
		case e.dir:
		case e.id == c.ID:
			old = e.name
		default:
			used[lower(strings.TrimSuffix(e.name, ext))] = true
		}
	}
	name := uniqueName(sanitize(c.Name), used) + ext
	if old != "" && old != name {
		if err := os.Rename(filepath.Join(root, old), filepath.Join(root, name)); err != nil {
			return err
		}
	}
	folders, requests := toFile(c.Items)
	f := collectionFile{ID: c.ID, Name: c.Name, Proto: c.Proto, Folders: folders, Requests: requests}
	if err := writeYAML(filepath.Join(root, name), f); err != nil {
		return err
	}
	return s.removeDirs(root, list, c.ID)
}

func (s *Store) DeleteCollection(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	root := filepath.Join(s.dir, "collections")
	list, err := s.stored(root)
	if err != nil {
		return err
	}
	for _, e := range list {
		if !e.dir && e.id == id {
			if err := os.Remove(filepath.Join(root, e.name)); err != nil {
				return err
			}
		}
	}
	return s.removeDirs(root, list, id)
}

// stored lists the collections under root with their ids. A directory counts
// only while it still holds old-layout files, so one left with just a README
// after its collection moved to a single file is not a collection.
func (s *Store) stored(root string) ([]stored, error) {
	entries, err := os.ReadDir(root)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	var out []stored
	for _, e := range entries {
		name := e.Name()
		path := filepath.Join(root, name)
		switch {
		case strings.HasPrefix(name, "."):
		case e.IsDir():
			if files, _ := legacyFiles(path, collectionMeta); len(files) == 0 {
				continue
			}
			m, err := s.readMeta(path, collectionMeta)
			if err != nil {
				return nil, err
			}
			out = append(out, stored{name: name, dir: true, id: m.ID})
		case strings.HasPrefix(name, "_"), !strings.HasSuffix(name, ext):
		default:
			var head struct {
				ID string `yaml:"id"`
			}
			if err := readYAML(path, &head); err != nil {
				return nil, err
			}
			if head.ID == "" {
				head.ID = s.pathID(path)
			}
			out = append(out, stored{name: name, id: head.ID})
		}
	}
	return out, nil
}

// removeDirs removes the old-layout files of collection id from root.
func (s *Store) removeDirs(root string, list []stored, id string) error {
	for _, e := range list {
		if e.dir && e.id == id {
			if err := removeLegacy(filepath.Join(root, e.name)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Store) readCollectionFile(path string) (*model.Collection, error) {
	f := collectionFile{}
	if err := readYAML(path, &f); err != nil {
		return nil, err
	}
	if f.ID == "" {
		f.ID = s.pathID(path)
	}
	if f.Name == "" {
		f.Name = strings.TrimSuffix(filepath.Base(path), ext)
	}
	if f.Proto.Empty() {
		f.Proto = nil
	}
	rel, err := filepath.Rel(s.dir, path)
	if err != nil {
		rel = path
	}
	items := s.fromFile(filepath.ToSlash(rel), f.Folders, f.Requests, map[string]bool{})
	return &model.Collection{ID: f.ID, Name: f.Name, Proto: f.Proto, Items: items}, nil
}

// fromFile returns folders first, then requests, each sorted by name. An item
// without an id gets one from its file and the names leading to it (key), so
// it keeps it from one read to the next.
func (s *Store) fromFile(key string, folders []folderFile, requests []requestFile, seen map[string]bool) []*model.Item {
	id := func(id, name string) string {
		if id != "" {
			return id
		}
		return hashID(uniqueName(key+"/"+name, seen))
	}
	out := []*model.Item{}
	for _, f := range folders {
		if f.Name == "" {
			f.Name = untitled
		}
		f.ID = id(f.ID, f.Name)
		out = append(out, &model.Item{ID: f.ID, Name: f.Name, Items: s.fromFile(key+"/"+f.Name, f.Folders, f.Requests, seen)})
	}
	sortItems(out)
	reqs := []*model.Item{}
	for _, rf := range requests {
		if rf.Name == "" {
			rf.Name = untitled
		}
		req := rf.Request
		reqs = append(reqs, &model.Item{ID: id(rf.ID, rf.Name), Name: rf.Name, Request: &req})
	}
	sortItems(reqs)
	return append(out, reqs...)
}

// toFile splits items into the folders and requests of a file, sorted so the
// file only changes when the collection does.
func toFile(items []*model.Item) ([]folderFile, []requestFile) {
	var folders, requests []*model.Item
	for _, it := range items {
		if it.Request == nil {
			folders = append(folders, it)
		} else {
			requests = append(requests, it)
		}
	}
	sortItems(folders)
	sortItems(requests)
	var ff []folderFile
	for _, it := range folders {
		sub, reqs := toFile(it.Items)
		ff = append(ff, folderFile{ID: it.ID, Name: it.Name, Folders: sub, Requests: reqs})
	}
	var rf []requestFile
	for _, it := range requests {
		rf = append(rf, requestFile{ID: it.ID, Name: it.Name, Request: withoutDefaults(*it.Request)})
	}
	return ff, rf
}

// sortItems orders by name as the sidebar does, then by exact name and id so
// the order never depends on the order items arrived in.
func sortItems(items []*model.Item) {
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if lower(a.Name) != lower(b.Name) {
			return lower(a.Name) < lower(b.Name)
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.ID < b.ID
	})
}

func (s *Store) readCollectionDir(dir string) (*model.Collection, error) {
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
	return &model.Collection{ID: m.ID, Name: m.Name, Proto: m.Proto, Items: items}, nil
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

// readItems reads an old-layout directory and returns folders first, then
// requests, each sorted by name.
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
	sortItems(folders)
	sortItems(requests)
	return append(folders, requests...), nil
}

// legacyFiles lists the files of an old-layout directory that readItems reads
// (plus its meta file), and the directories it looks in, deepest first.
func legacyFiles(dir, metaFile string) (files, dirs []string) {
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		name := e.Name()
		path := filepath.Join(dir, name)
		switch {
		case name == metaFile && !e.IsDir():
			files = append(files, path)
		case strings.HasPrefix(name, "."), strings.HasPrefix(name, "_"):
		case e.IsDir():
			f, d := legacyFiles(path, folderMeta)
			files, dirs = append(files, f...), append(dirs, d...)
		case strings.HasSuffix(name, ext):
			files = append(files, path)
		}
	}
	return files, append(dirs, dir)
}

// removeLegacy removes the old-layout files under dir, then the directories
// that leaves empty. Anything else (a README, notes) stays where it is.
func removeLegacy(dir string) error {
	files, dirs := legacyFiles(dir, collectionMeta)
	for _, f := range files {
		if err := os.Remove(f); err != nil {
			return err
		}
	}
	for _, d := range dirs {
		if entries, err := os.ReadDir(d); err != nil || len(entries) > 0 {
			continue
		}
		if err := os.Remove(d); err != nil {
			return err
		}
	}
	return nil
}

// withoutDefaults drops what a request does not need to say because it is
// the default anyway: no auth, an API key in a header, an empty body and its
// content type. The editor fills these in, so without this every saved
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
	return hashID(filepath.ToSlash(rel))
}

func hashID(key string) string {
	sum := sha1.Sum([]byte(key))
	return hex.EncodeToString(sum[:8])
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
