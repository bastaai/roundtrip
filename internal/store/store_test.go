package store

import (
	"errors"
	"fmt"
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"roundtrip/internal/model"
)

func open(t *testing.T) (*Store, string, string) {
	t.Helper()
	dir, local := t.TempDir(), t.TempDir()
	s, err := Open(dir, local)
	if err != nil {
		t.Fatal(err)
	}
	return s, dir, local
}

func req(url string) *model.Request {
	return &model.Request{Method: "GET", URL: url, Headers: []model.KV{{Key: "Accept", Value: "*/*", Enabled: true}}}
}

// tree lists every file under dir, relative and slash-separated.
func tree(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			rel, _ := filepath.Rel(dir, path)
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(out)
	return out
}

func TestEnvironmentsAreLocalOnly(t *testing.T) {
	s, dir, local := open(t)
	env := &model.Environment{ID: "e1", Name: "Staging", Variables: []model.KV{
		{Key: "baseUrl", Value: "https://staging", Enabled: true},
		{Key: "token", Value: "hunter2", Enabled: true, Secret: true},
	}}
	if err := s.SaveEnvironment(env); err != nil {
		t.Fatal(err)
	}
	s.SaveEnvironment(&model.Environment{ID: "e2", Name: "dev"})
	if got := tree(t, dir); len(got) != 0 {
		t.Fatalf("environment data was written to the workspace: %q", got)
	}
	if got := tree(t, local); !reflect.DeepEqual(got, []string{"environments.json"}) {
		t.Fatalf("local files: %q", got)
	}

	got, err := s.Environment("e1")
	if err != nil || !reflect.DeepEqual(got, env) {
		t.Fatalf("got %#v %v", got, err)
	}
	// Sorted by name, and never nil variables.
	envs, _ := s.Environments()
	if len(envs) != 2 || envs[0].ID != "e2" || envs[0].Variables == nil {
		t.Fatalf("got %#v", envs)
	}

	// A teammate with the same workspace has none of it.
	other, _ := Open(dir, t.TempDir())
	if envs, err := other.Environments(); err != nil || envs == nil || len(envs) != 0 {
		t.Fatalf("got %#v %v", envs, err)
	}

	env.Name = "Stage"
	s.SaveEnvironment(env)
	if envs, _ = s.Environments(); len(envs) != 2 || envs[1].Name != "Stage" {
		t.Fatalf("rename: %#v", envs)
	}

	s.DeleteEnvironment("e1")
	data, _ := os.ReadFile(filepath.Join(local, "environments.json"))
	if e, _ := s.Environment("e1"); e != nil || strings.Contains(string(data), "hunter2") {
		t.Fatalf("delete: %#v %s", e, data)
	}
	if got := tree(t, dir); len(got) != 0 {
		t.Fatalf("environment data was written to the workspace: %q", got)
	}
}

func TestOpenReadOnlyCreatesNothing(t *testing.T) {
	base := t.TempDir()
	if _, err := OpenReadOnly(filepath.Join(base, "missing"), filepath.Join(base, "local")); err == nil {
		t.Fatal("want error for a missing workspace")
	}
	s, err := OpenReadOnly(base, filepath.Join(base, "local"))
	if err != nil {
		t.Fatal(err)
	}
	cols, err1 := s.Collections()
	envs, err2 := s.Environments()
	if err1 != nil || err2 != nil || len(cols) != 0 || len(envs) != 0 {
		t.Fatalf("got %v %v %v %v", cols, envs, err1, err2)
	}
	if got := tree(t, base); len(got) != 0 {
		t.Fatalf("files created: %q", got)
	}
	if _, err := os.Stat(filepath.Join(base, "local")); err == nil {
		t.Fatal("local directory created")
	}
}

// Whatever a user types must come back byte for byte, however YAML would
// read it if it were written unquoted.
func TestStringsRoundTripExactly(t *testing.T) {
	s, dir, _ := open(t)
	values := []string{
		"yes", "no", "on", "off", "y", "n", "true", "False", "null", "Null", "~", "",
		"0123", "0x1F", "0o17", "1e3", "1_000", "12", "-1", "3.14", ".5", "+1", ".inf", ".NaN", "2026-10-14", "12:30:00",
		" leading", "trailing ", "  ", "\t", "\ttab first", "tab\tinside",
		"{{var}}", "{{ var }}/x", "{a: 1}", "[1, 2]", "a: b", "a #b", "#hash", "- dash", "? q", "! bang", "!!str x", "&a", "*a", "|", ">", "%x", "@x", "`x`", "'", "\"", "it's", `back\slash`, "'quoted'", `"quoted"`, "a,b", "key:", ": v", "---", "...", "--- a",
		"line1\nline2", "line1\nline2\n", "line1\nline2\n\n", "line1\nline2\n\n\n", "\n", "\n\n", "\nleading break", "\n\nline",
		"  indented first\nsecond", "\tindented first\nsecond", "first\n  indented later\n", "{\n\t\"tabs\": 1\n}",
		"trailing space \nnext", "next\ntrailing space ", "only spaces line\n   \nend", "tab at end\t\nnext",
		"crlf\r\nbody\r\n", "{\r\n  \"a\": 1\r\n}", "lone\rcr", "mixed\r\nand\nlf",
		"# comment\nline", "--- \ndoc", "---\ndoc", "...\nend", "key: value\nother: {{x}}", "- a\n- b",
		"unicode é ✓ 日本語", "emoji \U0001F600\nline", "nbsp\u00a0x", "line sep\u2028x", "next line\u0085x", "bom\ufeffx", "nul\x00x", "bell\x07", "del\x7f", "esc\x1b[0m",
		strings.Repeat("long words with spaces ", 40), strings.Repeat("x", 500), strings.Repeat("a long line with spaces in it ", 10) + "\n" + strings.Repeat("and another ", 30),
	}
	c := &model.Collection{ID: "c1", Name: "C"}
	for i, v := range values {
		c.Items = append(c.Items, &model.Item{ID: fmt.Sprint(i), Name: fmt.Sprintf("r%03d", i), Request: &model.Request{
			Method:  v,
			URL:     v,
			Headers: []model.KV{{Key: v, Value: v, Enabled: true}},
			Auth:    model.Auth{Type: "bearer", Token: v},
			Body:    model.Body{Mode: "graphql", Raw: v, Variables: v, ContentType: v},
			Tests:   []model.Assertion{{Target: "body", Op: "eq", Value: v}},
		}})
	}
	// Names go through the same encoder (the file name is derived separately).
	c.Items = append(c.Items, &model.Item{ID: "n1", Name: "0123", Request: &model.Request{}}, &model.Item{ID: "n2", Name: "yes", Request: &model.Request{}})
	if err := s.SaveCollection(c); err != nil {
		t.Fatal(err)
	}
	got, err := s.Collections()
	if err != nil {
		t.Fatal(err)
	}
	if len(got[0].Items) != len(c.Items) {
		t.Fatalf("got %d items, want %d", len(got[0].Items), len(c.Items))
	}
	byID := map[string]*model.Item{}
	for _, it := range got[0].Items {
		byID[it.ID] = it
	}
	for _, want := range c.Items {
		it := byID[want.ID]
		if it == nil || !reflect.DeepEqual(it, want) {
			file, _ := os.ReadFile(filepath.Join(dir, "collections", "C.yaml"))
			t.Errorf("%q did not round trip: %#v\n%s", want.Request.URL, it, file)
		}
	}

	// Saving what was read changes nothing on disk.
	before := map[string]string{}
	for _, f := range tree(t, dir) {
		data, _ := os.ReadFile(filepath.Join(dir, f))
		before[f] = string(data)
	}
	if err := s.SaveCollection(got[0]); err != nil {
		t.Fatal(err)
	}
	for _, f := range tree(t, dir) {
		if data, _ := os.ReadFile(filepath.Join(dir, f)); string(data) != before[f] {
			t.Errorf("%s changed on re-save", f)
		}
	}
}

// Random mixes of the characters YAML cares about, through every position a
// string can be written in (plain value, block scalar, inside a list).
func TestRandomStringsRoundTrip(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	alphabet := []string{" ", " ", "\t", "\n", "\n", "\r", "\r\n", "a", "b", ":", "#", "-", "{", "}", "\"", "'", "|", ">", "\\", "\u2028", "\u0085", "\ufeff", "é", "%", "!", "&", "*", "?", ",", "[", "0", "~", "\x00", "---", "..."}
	path := filepath.Join(t.TempDir(), "f.yaml")
	for range 4000 {
		s := ""
		for range rng.Intn(12) {
			s += alphabet[rng.Intn(len(alphabet))]
		}
		in := requestFile{ID: s, Name: s}
		in.Body.Raw = s
		in.Tests = []model.Assertion{{Target: s, Op: s, Value: s}}
		os.Remove(path)
		if err := writeYAML(path, in); err != nil {
			t.Fatalf("%q: %v", s, err)
		}
		out := requestFile{}
		if err := readYAML(path, &out); err != nil || !reflect.DeepEqual(out, in) {
			file, _ := os.ReadFile(path)
			t.Fatalf("%q did not round trip: %v\n%s", s, err, file)
		}
	}
}

func TestHistoryKeepsResponses(t *testing.T) {
	s, _, local := open(t)
	s.AddHistory(&model.HistoryEntry{ID: "a1"}, &model.Response{Status: 200, Body: `{"ok":true}`})
	s.AddHistory(&model.HistoryEntry{ID: "b2"}, &model.Response{Status: 200, Body: strings.Repeat("x", keptBodyLimit+1), Size: keptBodyLimit + 1})

	if res, err := s.HistoryResponse("a1"); err != nil || res.Body != `{"ok":true}` || res.BodyNotKept {
		t.Fatalf("small response: %+v %v", res, err)
	}
	if res, err := s.HistoryResponse("b2"); err != nil || res.Body != "" || !res.BodyNotKept || res.Size != keptBodyLimit+1 {
		t.Fatalf("large response: %+v %v", res, err)
	}
	for _, id := range []string{"c3", "../history", ""} {
		if _, err := s.HistoryResponse(id); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("%q: %v, want not exist", id, err)
		}
	}

	// Entries that fall off the end take their response with them.
	for i := range historyLimit {
		s.AddHistory(&model.HistoryEntry{ID: fmt.Sprintf("%x", 0x100+i)}, &model.Response{})
	}
	if _, err := s.HistoryResponse("a1"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("pruned response still there: %v", err)
	}
	if files, _ := os.ReadDir(filepath.Join(local, "responses")); len(files) != historyLimit {
		t.Fatalf("%d response files, want %d", len(files), historyLimit)
	}
	s.ClearHistory()
	if _, err := os.Stat(filepath.Join(local, "responses")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("responses not cleared")
	}
}

func TestHistoryIsLocalNewestFirstAndCapped(t *testing.T) {
	s, dir, _ := open(t)
	for i := range historyLimit + 5 {
		s.AddHistory(&model.HistoryEntry{ID: fmt.Sprint(i)}, nil)
	}
	h, _ := s.History()
	if len(h) != historyLimit || h[0].ID != fmt.Sprint(historyLimit+4) {
		t.Fatalf("len=%d first=%s", len(h), h[0].ID)
	}
	if got := tree(t, dir); len(got) != 0 {
		t.Fatalf("history leaked into workspace: %q", got)
	}
	s.ClearHistory()
	if h, _ := s.History(); len(h) != 0 {
		t.Fatal("not cleared")
	}
}

func TestSanitize(t *testing.T) {
	cases := map[string]string{
		"Get user":    "Get user",
		"a/b\\c":      "a-b-c",
		"  .hidden. ": "hidden",
		"_meta":       "meta",
		"":            "untitled",
		"???":         "---",
		"CON":         "CON-",
		"nul":         "nul-",
	}
	for in, want := range cases {
		if got := sanitize(in); got != want {
			t.Errorf("sanitize(%q) = %q, want %q", in, got, want)
		}
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func write(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCollectionIsOneFile(t *testing.T) {
	s, dir, _ := open(t)
	if got, err := s.Collections(); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("want empty non-nil slice, got %#v %v", got, err)
	}

	c := &model.Collection{ID: "c1", Name: "Users API", Items: []*model.Item{
		{ID: "r1", Name: "List users", Request: req("https://x/users")},
		{ID: "f1", Name: "Admin", Items: []*model.Item{
			{ID: "r2", Name: "Delete user", Request: req("https://x/users/1")},
			{ID: "f2", Name: "Deeper", Items: []*model.Item{}},
		}},
		{ID: "r3", Name: "list users", Request: req("https://x/users?page=2")},
		{ID: "r4", Name: "a/b: c?", Request: req("https://x")},
	}}
	if err := s.SaveCollection(c); err != nil {
		t.Fatal(err)
	}
	if got := tree(t, dir); !reflect.DeepEqual(got, []string{"collections/Users API.yaml"}) {
		t.Fatalf("layout: %q", got)
	}
	want := `id: c1
name: Users API
folders:
  - id: f1
    name: Admin
    folders:
      - id: f2
        name: Deeper
    requests:
      - id: r2
        name: Delete user
        method: GET
        url: https://x/users/1
        headers:
          - key: Accept
            value: '*/*'
            enabled: true
requests:
  - id: r4
    name: 'a/b: c?'
    method: GET
    url: https://x
    headers:
      - key: Accept
        value: '*/*'
        enabled: true
  - id: r1
    name: List users
    method: GET
    url: https://x/users
    headers:
      - key: Accept
        value: '*/*'
        enabled: true
  - id: r3
    name: list users
    method: GET
    url: https://x/users?page=2
    headers:
      - key: Accept
        value: '*/*'
        enabled: true
`
	if got := read(t, filepath.Join(dir, "collections", "Users API.yaml")); got != want {
		t.Fatalf("file:\n%s\nwant:\n%s", got, want)
	}

	got, err := s.Collections()
	if err != nil {
		t.Fatal(err)
	}
	// Folders first, then requests, by name. A folder with no items reads
	// back as an empty list.
	c.Items[1].Items[1].Items = []*model.Item{}
	c.Items = []*model.Item{c.Items[1], c.Items[3], c.Items[0], c.Items[2]}
	c.Items[0].Items = []*model.Item{c.Items[0].Items[1], c.Items[0].Items[0]}
	if !reflect.DeepEqual(got, []*model.Collection{c}) {
		t.Fatalf("round trip mismatch: %#v", got[0])
	}
}

func TestSaveIsStable(t *testing.T) {
	s, dir, _ := open(t)
	c := &model.Collection{ID: "c1", Name: "C", Items: []*model.Item{
		{ID: "r1", Name: "B", Request: req("b")},
		{ID: "r2", Name: "a", Request: req("a")},
		{ID: "r3", Name: "A", Request: req("a2")},
		{ID: "r4", Name: "A", Request: req("a3")},
		{ID: "f1", Name: "F", Items: []*model.Item{{ID: "r5", Name: "x", Request: req("x")}, {ID: "r6", Name: "X", Request: req("y")}}},
	}}
	if err := s.SaveCollection(c); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "collections", "C.yaml")
	first := read(t, path)
	old := time.Now().Add(-time.Hour).Truncate(time.Second)
	os.Chtimes(path, old, old)

	// The same collection in any order writes the same bytes, so nothing is
	// written at all.
	slices.Reverse(c.Items)
	slices.Reverse(c.Items[0].Items)
	if err := s.SaveCollection(c); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Collections()
	if err := s.SaveCollection(got[0]); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(path); !info.ModTime().Equal(old) || read(t, path) != first {
		t.Fatalf("unchanged collection was rewritten:\n%s", read(t, path))
	}
}

func TestRenameCollisionAndDelete(t *testing.T) {
	s, dir, _ := open(t)
	one := &model.Collection{ID: "c1", Name: "One", Items: []*model.Item{{ID: "r1", Name: "R", Request: req("a")}}}
	s.SaveCollection(one)
	one.Name = "Two"
	if err := s.SaveCollection(one); err != nil {
		t.Fatal(err)
	}
	if got := tree(t, dir); !reflect.DeepEqual(got, []string{"collections/Two.yaml"}) {
		t.Fatalf("rename: %q", got)
	}

	// Another collection whose name makes the same file name gets its own.
	two := &model.Collection{ID: "c2", Name: "two"}
	if err := s.SaveCollection(two); err != nil {
		t.Fatal(err)
	}
	if got := tree(t, dir); !reflect.DeepEqual(got, []string{"collections/Two.yaml", "collections/two 2.yaml"}) {
		t.Fatalf("collision: %q", got)
	}
	if !strings.Contains(read(t, filepath.Join(dir, "collections", "Two.yaml")), "id: c1\n") {
		t.Fatal("collision overwrote the first collection")
	}
	two.Name = "Three"
	s.SaveCollection(two)
	// Only the case changes.
	one.Name = "TWO"
	s.SaveCollection(one)
	if got := tree(t, dir); !reflect.DeepEqual(got, []string{"collections/TWO.yaml", "collections/Three.yaml"}) {
		t.Fatalf("renames: %q", got)
	}
	cs, err := s.Collections()
	if err != nil || len(cs) != 2 || cs[0].Name != "Three" || cs[1].Name != "TWO" || len(cs[1].Items) != 1 {
		t.Fatalf("collections: %#v %v", cs, err)
	}

	if err := s.DeleteCollection("c1"); err != nil {
		t.Fatal(err)
	}
	if got := tree(t, dir); !reflect.DeepEqual(got, []string{"collections/Three.yaml"}) {
		t.Fatalf("delete: %q", got)
	}
	if err := s.DeleteCollection("missing"); err != nil {
		t.Fatal(err)
	}
}

// The layout before collections were single files: a directory per
// collection and folder, a file per request.
var oldLayout = map[string]string{
	"collections/Users API/_collection.yaml":         "id: c1\nname: Users API\nproto:\n  roots: [protos]\n",
	"collections/Users API/List users.yaml":          "id: r1\nname: List users\nmethod: GET\nurl: https://x/users\n",
	"collections/Users API/README.md":                "notes",
	"collections/Users API/.keep":                    "",
	"collections/Users API/Admin/_folder.yaml":       "id: f1\nname: Admin\n",
	"collections/Users API/Admin/Delete user.yaml":   "id: r2\nname: Delete user\nmethod: DELETE\nurl: https://x/users/1\nbody:\n  mode: raw\n  raw: |\n    {\n      \"a\": 1\n    }\n",
	"collections/Users API/Admin/Empty/_folder.yaml": "id: f2\nname: Empty\n",
	"collections/Users API/Admin/Empty/legacy.json":  "{}",
	"collections/Users API/Other/_folder.yaml":       "id: f3\nname: Other\n",
}

func TestOldLayoutIsReadAndMigratedOnSave(t *testing.T) {
	s, dir, _ := open(t)
	write(t, dir, oldLayout)
	before, err := s.Collections()
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != 1 || before[0].ID != "c1" || before[0].Proto == nil || len(before[0].Items) != 3 || before[0].Items[0].Items[1].Request.Body.Raw != "{\n  \"a\": 1\n}\n" {
		t.Fatalf("old layout: %#v", before)
	}

	if err := s.SaveCollection(before[0]); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"collections/Users API.yaml",
		"collections/Users API/.keep",
		"collections/Users API/Admin/Empty/legacy.json",
		"collections/Users API/README.md",
	}
	if got := tree(t, dir); !reflect.DeepEqual(got, want) {
		t.Fatalf("after save:\n got %q\nwant %q", got, want)
	}
	if _, err := os.Stat(filepath.Join(dir, "collections", "Users API", "Other")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("empty folder directory left behind")
	}
	if file := read(t, filepath.Join(dir, "collections", "Users API.yaml")); !strings.Contains(file, "          raw: |\n            {\n              \"a\": 1\n            }\n") {
		t.Fatalf("body is not a block scalar:\n%s", file)
	}
	after, err := s.Collections()
	if err != nil || !reflect.DeepEqual(after, before) {
		t.Fatalf("after migration: %#v %v", after, err)
	}
}

func TestSingleFileWinsOverOldLayout(t *testing.T) {
	s, dir, _ := open(t)
	write(t, dir, oldLayout)
	write(t, dir, map[string]string{"collections/Users.yaml": "id: c1\nname: Users\nrequests:\n  - id: r9\n    name: New\n    method: GET\n    url: https://x\n"})
	cs, err := s.Collections()
	if err != nil || len(cs) != 1 || cs[0].Name != "Users" || len(cs[0].Items) != 1 || cs[0].Items[0].ID != "r9" {
		t.Fatalf("got %#v %v", cs, err)
	}
	if err := s.DeleteCollection("c1"); err != nil {
		t.Fatal(err)
	}
	want := []string{"collections/Users API/.keep", "collections/Users API/Admin/Empty/legacy.json", "collections/Users API/README.md"}
	if got := tree(t, dir); !reflect.DeepEqual(got, want) {
		t.Fatalf("after delete: %q", got)
	}
	// What is left behind is not a collection.
	if cs, err := s.Collections(); err != nil || len(cs) != 0 {
		t.Fatalf("leftovers read as a collection: %#v %v", cs, err)
	}
}

func TestHandWrittenCollectionFile(t *testing.T) {
	s, dir, _ := open(t)
	write(t, dir, map[string]string{
		"collections/Manual.yaml": `requests:
  - {name: b, method: GET, url: https://x/b}
  - method: POST
    url: https://x/unnamed
  - name: a
    method: GET
    url: https://x/a
folders:
  - name: Sub
    requests:
      - {name: a, method: GET, url: https://x/sub/a}
      - {name: a, method: GET, url: https://x/sub/a2}
  - requests: []
`,
		"collections/Stem.yaml":   "id: s1\n",
		"collections/notes.txt":   "not a collection",
		"collections/_draft.yaml": "id: d1\n",
	})
	a, err := s.Collections()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := s.Collections()
	if !reflect.DeepEqual(a, b) {
		t.Fatal("ids change between reads")
	}
	if len(a) != 2 || a[0].Name != "Manual" || a[1].Name != "Stem" || a[1].ID != "s1" || a[1].Items == nil {
		t.Fatalf("collections: %#v", a)
	}
	var names []string
	ids := map[string]bool{a[0].ID: true}
	var walk func([]*model.Item, string)
	walk = func(items []*model.Item, prefix string) {
		for _, it := range items {
			names = append(names, prefix+it.Name)
			ids[it.ID] = true
			walk(it.Items, prefix+it.Name+"/")
		}
	}
	walk(a[0].Items, "")
	if want := []string{"Sub", "Sub/a", "Sub/a", "Untitled", "a", "b", "Untitled"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("names: %q, want %q", names, want)
	}
	if len(ids) != 8 || ids[""] {
		t.Fatalf("ids not unique: %v", ids)
	}

	// Saving writes the ids it was given, so they stay.
	if err := s.SaveCollection(a[0]); err != nil {
		t.Fatal(err)
	}
	if file := read(t, filepath.Join(dir, "collections", "Manual.yaml")); !strings.HasPrefix(file, "id: "+a[0].ID+"\nname: Manual\nfolders:\n") {
		t.Fatalf("saved:\n%s", file)
	}
	if c, err := s.Collections(); err != nil || !reflect.DeepEqual(c, a) {
		t.Fatalf("after save: %#v %v", c, err)
	}
}

func TestHandWrittenOldLayoutGetsStableIdentity(t *testing.T) {
	s, dir, _ := open(t)
	write(t, dir, map[string]string{"collections/Manual/sub/note.md": "", "collections/Manual/ping.yaml": "method: GET\nurl: https://x/ping\n"})

	a, err := s.Collections()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := s.Collections()
	if len(a) != 1 || a[0].Name != "Manual" || len(a[0].Items) != 2 {
		t.Fatalf("got %#v", a)
	}
	ping := a[0].Items[1]
	if ping.Name != "ping" || ping.ID == "" || ping.ID != b[0].Items[1].ID || ping.Request.URL != "https://x/ping" {
		t.Fatalf("got %#v", ping)
	}
	if err := s.SaveCollection(a[0]); err != nil {
		t.Fatal(err)
	}
	if c, err := s.Collections(); err != nil || !reflect.DeepEqual(c, a) {
		t.Fatalf("after save: %#v %v", c, err)
	}
	if got := tree(t, dir); !reflect.DeepEqual(got, []string{"collections/Manual.yaml", "collections/Manual/sub/note.md"}) {
		t.Fatalf("after save: %q", got)
	}
}

func TestBrokenFileIsReportedByPath(t *testing.T) {
	for _, name := range []string{"collections/bad.yaml", "collections/C/bad.yaml"} {
		s, dir, _ := open(t)
		write(t, dir, map[string]string{name: "<<<<<<< HEAD\nmethod: GET\n=======\nmethod: POST\n>>>>>>> theirs\n"})
		if _, err := s.Collections(); err == nil || !strings.Contains(err.Error(), "bad.yaml") {
			t.Errorf("%s: want error naming the file, got %v", name, err)
		}
	}
}

func TestRequestsAreReadableYAML(t *testing.T) {
	s, dir, _ := open(t)
	r := &model.Request{
		Method:  "POST",
		URL:     "{{baseUrl}}/login",
		Headers: []model.KV{{Key: "Accept", Value: "*/*", Enabled: true}, {Key: "X-Debug", Value: "{{debug}}"}},
		Auth:    model.Auth{Type: "bearer", Token: "{{token}}"},
		Body:    model.Body{Mode: "raw", ContentType: "application/json", Raw: "{\n  \"user\": \"ada\",\n  \"roles\": [\"admin\"]\n}"},
		Tests:   []model.Assertion{{Target: "status", Op: "eq", Value: "200"}, {Target: "body.token", Op: "exists"}},
		Extract: []model.Extract{{Var: "token", From: "body.token"}},
		Docs:    "Signs in and saves the session token.\nNeeds an active user.",
	}
	gql := &model.Request{Method: "POST", URL: "https://x/graphql", Body: model.Body{Mode: "graphql", Raw: "query Q($id: ID!) {\n  user(id: $id) { name }\n}\n", Variables: "{\n  \"id\": \"1\"\n}"}}
	c := &model.Collection{ID: "c1", Name: "API", Items: []*model.Item{
		{ID: "r0", Name: "GraphQL", Request: gql},
		{ID: "r1", Name: "Login", Request: r},
		{ID: "r2", Name: "Minimal", Request: &model.Request{}},
	}}
	if err := s.SaveCollection(c); err != nil {
		t.Fatal(err)
	}
	want := `id: c1
name: API
requests:
  - id: r0
    name: GraphQL
    method: POST
    url: https://x/graphql
    body:
      mode: graphql
      raw: |
        query Q($id: ID!) {
          user(id: $id) { name }
        }
      variables: |-
        {
          "id": "1"
        }
  - id: r1
    name: Login
    method: POST
    url: '{{baseUrl}}/login'
    headers:
      - key: Accept
        value: '*/*'
        enabled: true
      - key: X-Debug
        value: '{{debug}}'
        enabled: false
    auth:
      type: bearer
      token: '{{token}}'
    body:
      mode: raw
      raw: |-
        {
          "user": "ada",
          "roles": ["admin"]
        }
      contentType: application/json
    tests:
      - target: status
        op: eq
        value: "200"
      - target: body.token
        op: exists
    extract:
      - var: token
        from: body.token
    docs: |-
      Signs in and saves the session token.
      Needs an active user.
  - id: r2
    name: Minimal
    method: ""
    url: ""
`
	if got := read(t, filepath.Join(dir, "collections", "API.yaml")); got != want {
		t.Errorf("file:\n%s\nwant:\n%s", got, want)
	}
	got, err := s.Collections()
	if err != nil || !reflect.DeepEqual(got, []*model.Collection{c}) {
		t.Fatalf("round trip: %v %#v", err, got[0].Items[1].Request)
	}
}

// What the editor fills in by default is not written to the file.
func TestDefaultsAreNotWritten(t *testing.T) {
	s, dir, _ := open(t)
	r := &model.Request{
		Method: "GET",
		URL:    "https://x/y",
		Auth:   model.Auth{Type: "none", In: "header"},
		Body:   model.Body{Mode: "none", ContentType: "application/json"},
	}
	key := &model.Request{Method: "GET", URL: "https://x/y", Auth: model.Auth{Type: "apikey", Key: "X-Key", Value: "{{key}}", In: "header"}}
	c := &model.Collection{ID: "c1", Name: "API", Items: []*model.Item{{ID: "r1", Name: "Plain", Request: r}, {ID: "r2", Name: "Key", Request: key}}}
	if err := s.SaveCollection(c); err != nil {
		t.Fatal(err)
	}
	want := "id: c1\nname: API\nrequests:\n" +
		"  - id: r2\n    name: Key\n    method: GET\n    url: https://x/y\n    auth:\n      type: apikey\n      key: X-Key\n      value: '{{key}}'\n" +
		"  - id: r1\n    name: Plain\n    method: GET\n    url: https://x/y\n"
	if got := read(t, filepath.Join(dir, "collections", "API.yaml")); got != want {
		t.Errorf("file:\n%s\nwant:\n%s", got, want)
	}
}

// Multi-line text is written as a block scalar wherever YAML can represent it
// that way, so a body diffs line by line.
func TestMultiLineBodiesAreBlockScalars(t *testing.T) {
	s, dir, _ := open(t)
	cases := map[string]bool{
		"{\n  \"a\": 1\n}":       true,
		"{\n  \"a\": 1\n}\n":     true,
		"{\n\t\"a\": 1\n}\n\n":   true,
		"  indented\nfirst line": true,
		"a: b\n# not a comment":  true,
		// YAML cannot keep these in a block scalar; they are quoted instead.
		"{\r\n  \"a\": 1\r\n}": false,
		"trailing \nspace":     false,
	}
	for body, block := range cases {
		c := &model.Collection{ID: "c1", Name: "C", Items: []*model.Item{{ID: "f", Name: "F", Items: []*model.Item{{ID: "r", Name: "R", Request: &model.Request{Body: model.Body{Mode: "raw", Raw: body}}}}}}}
		if err := s.SaveCollection(c); err != nil {
			t.Fatal(err)
		}
		file := read(t, filepath.Join(dir, "collections", "C.yaml"))
		if got := strings.Contains(file, "raw: |"); got != block {
			t.Errorf("%q: block scalar = %v, want %v\n%s", body, got, block, file)
		}
		got, err := s.Collections()
		if err != nil || got[0].Items[0].Items[0].Request.Body.Raw != body {
			t.Errorf("%q did not round trip: %v\n%s", body, err, file)
		}
	}
}

func TestHandWrittenYAMLIsRead(t *testing.T) {
	// Unquoted scalars land in string fields as written; CRLF line endings
	// (git autocrlf on Windows) do not leak into values.
	request := "method: POST\r\nurl: http://x/1\r\nbody:\r\n  mode: raw\r\n  raw: |\r\n    {\r\n      \"a\": 1\r\n    }\r\ntests:\r\n  - {target: status, op: eq, value: 200}\r\n  - {target: body.n, op: eq, value: 0123}\r\n  - {target: body.ok, op: eq, value: true}\r\n"
	indented := "    " + strings.ReplaceAll(strings.TrimSuffix(request, "\r\n"), "\r\n", "\r\n    ") + "\r\n"
	for name, files := range map[string]map[string]string{
		"old layout":  {"collections/C/login.yaml": "# login\r\n" + request, "collections/C/legacy.json": `{"method":"GET"}`},
		"single file": {"collections/C.yaml": "# login\r\nrequests:\r\n  - name: login\r\n" + indented},
	} {
		s, dir, _ := open(t)
		write(t, dir, files)
		got, err := s.Collections()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		r := got[0].Items[0].Request
		want := []model.Assertion{{Target: "status", Op: "eq", Value: "200"}, {Target: "body.n", Op: "eq", Value: "0123"}, {Target: "body.ok", Op: "eq", Value: "true"}}
		if got[0].Items[0].Name != "login" || r.Method != "POST" || r.Body.Raw != "{\n  \"a\": 1\n}\n" || !reflect.DeepEqual(r.Tests, want) {
			t.Fatalf("%s: got %#v", name, r)
		}
		// Files this app does not own survive a save.
		if err := s.SaveCollection(got[0]); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(dir, "collections", "C", "legacy.json")); err != nil && files["collections/C/legacy.json"] != "" {
			t.Fatalf("%s: foreign file removed", name)
		}
	}
}

func TestCollectionProtoConfigRoundTrips(t *testing.T) {
	s, dir, _ := open(t)
	file := filepath.Join(dir, "collections", "Users API.yaml")
	c := &model.Collection{ID: "c1", Name: "Users API", Items: []*model.Item{},
		Proto: &model.ProtoConfig{Roots: []string{"protos", "{{protoRoot}}/third_party"}, Files: []string{"api/v1/users.proto"}}}
	if err := s.SaveCollection(c); err != nil {
		t.Fatal(err)
	}
	want := `id: c1
name: Users API
proto:
  roots:
    - protos
    - '{{protoRoot}}/third_party'
  files:
    - api/v1/users.proto
`
	if got := read(t, file); got != want {
		t.Fatalf("file:\n%s", got)
	}
	if got, err := s.Collections(); err != nil || !reflect.DeepEqual(got, []*model.Collection{c}) {
		t.Fatalf("round trip: %#v %v", got[0], err)
	}

	// Roots alone, and a config written by hand.
	os.WriteFile(file, []byte("id: c1\nname: Users API\nproto:\n  roots: [protos]\n"), 0o644)
	got, err := s.Collections()
	if err != nil || !reflect.DeepEqual(got[0].Proto, &model.ProtoConfig{Roots: []string{"protos"}}) {
		t.Fatalf("hand-written: %#v %v", got[0].Proto, err)
	}

	// An empty config is not written and reads back as none.
	for _, empty := range []*model.ProtoConfig{nil, {}, {Roots: []string{}, Files: []string{" "}}} {
		c.Proto = empty
		if err := s.SaveCollection(c); err != nil {
			t.Fatal(err)
		}
		if got := read(t, file); got != "id: c1\nname: Users API\n" {
			t.Fatalf("empty config written:\n%s", got)
		}
		if got, err := s.Collections(); err != nil || got[0].Proto != nil || c.Proto != nil {
			t.Fatalf("empty config read back: %#v %v", got[0].Proto, err)
		}
	}
	os.WriteFile(file, []byte("id: c1\nname: Users API\nproto: {}\n"), 0o644)
	if got, err := s.Collections(); err != nil || got[0].Proto != nil {
		t.Fatalf("hand-written empty config: %#v %v", got[0].Proto, err)
	}
}

// The file types must name their fields the same in YAML as in JSON (which
// is what writeYAML writes), and omit the same empty ones.
func TestFileYAMLTagsMirrorJSONTags(t *testing.T) {
	for _, v := range []any{collectionFile{}, folderFile{}, requestFile{}, meta{}} {
		typ := reflect.TypeOf(v)
		for i := range typ.NumField() {
			f := typ.Field(i)
			if f.Anonymous {
				continue
			}
			if j, y := f.Tag.Get("json"), f.Tag.Get("yaml"); y != j || y == "" {
				t.Errorf("%s.%s: json %q, yaml %q", typ.Name(), f.Name, j, y)
			}
		}
	}
}
