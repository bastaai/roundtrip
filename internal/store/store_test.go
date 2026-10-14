package store

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

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
	return &model.Request{Method: "GET", URL: url, Headers: []model.KV{{Key: "Accept", Value: "*/*", Enabled: true}}, Auth: model.Auth{Type: "none"}, Body: model.Body{Mode: "none"}}
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

func TestCollectionLayoutAndRoundTrip(t *testing.T) {
	s, dir, _ := open(t)
	if got, err := s.Collections(); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("want empty non-nil slice, got %#v %v", got, err)
	}

	c := &model.Collection{ID: "c1", Name: "Users API", Items: []*model.Item{
		{ID: "f1", Name: "Admin", Items: []*model.Item{
			{ID: "r2", Name: "Delete user", Request: req("https://x/users/1")},
		}},
		{ID: "r1", Name: "List users", Request: req("https://x/users")},
		{ID: "r3", Name: "list users", Request: req("https://x/users?page=2")},
		{ID: "r4", Name: "a/b: c?", Request: req("https://x")},
	}}
	if err := s.SaveCollection(c); err != nil {
		t.Fatal(err)
	}

	want := []string{
		"collections/Users API/Admin/Delete user.yaml",
		"collections/Users API/Admin/_folder.yaml",
		"collections/Users API/List users.yaml",
		"collections/Users API/_collection.yaml",
		"collections/Users API/a-b- c-.yaml",
		"collections/Users API/list users 2.yaml",
	}
	if got := tree(t, dir); !reflect.DeepEqual(got, want) {
		t.Fatalf("layout:\n got %q\nwant %q", got, want)
	}

	got, err := s.Collections()
	if err != nil {
		t.Fatal(err)
	}
	// Folders first, then requests, by name.
	c.Items = []*model.Item{c.Items[0], c.Items[3], c.Items[1], c.Items[2]}
	if !reflect.DeepEqual(got, []*model.Collection{c}) {
		a, _ := os.ReadFile(filepath.Join(dir, "collections/Users API/List users.yaml"))
		t.Fatalf("round trip mismatch: %#v\n%s", got[0], a)
	}
}

func TestSaveCollectionSyncsRenamesAndDeletes(t *testing.T) {
	s, dir, _ := open(t)
	c := &model.Collection{ID: "c1", Name: "One", Items: []*model.Item{
		{ID: "f1", Name: "Folder", Items: []*model.Item{{ID: "r2", Name: "Inner", Request: req("b")}}},
		{ID: "r1", Name: "Old name", Request: req("a")},
		{ID: "r3", Name: "Gone", Request: req("c")},
	}}
	s.SaveCollection(c)
	readme := filepath.Join(dir, "collections", "One", "README.md")
	os.WriteFile(readme, []byte("docs"), 0o644)

	c.Name = "Two"
	c.Items[0].Name = "Renamed folder"
	c.Items[1].Name = "New name"
	c.Items = c.Items[:2]
	if err := s.SaveCollection(c); err != nil {
		t.Fatal(err)
	}

	want := []string{
		"collections/Two/New name.yaml",
		"collections/Two/README.md",
		"collections/Two/Renamed folder/Inner.yaml",
		"collections/Two/Renamed folder/_folder.yaml",
		"collections/Two/_collection.yaml",
	}
	if got := tree(t, dir); !reflect.DeepEqual(got, want) {
		t.Fatalf("layout:\n got %q\nwant %q", got, want)
	}

	// A second collection with the same name gets its own directory.
	s.SaveCollection(&model.Collection{ID: "c2", Name: "two"})
	cs, _ := s.Collections()
	if len(cs) != 2 {
		t.Fatalf("want 2 collections, got %d: %q", len(cs), tree(t, dir))
	}

	s.DeleteCollection("c1")
	cs, _ = s.Collections()
	if len(cs) != 1 || cs[0].ID != "c2" {
		t.Fatalf("delete: %#v", cs)
	}
}

func TestUnchangedSaveDoesNotTouchFiles(t *testing.T) {
	s, dir, _ := open(t)
	c := &model.Collection{ID: "c1", Name: "C", Items: []*model.Item{{ID: "r1", Name: "R", Request: req("a")}}}
	s.SaveCollection(c)
	path := filepath.Join(dir, "collections", "C", "R.yaml")
	before, _ := os.Stat(path)
	s.SaveCollection(c)
	after, _ := os.Stat(path)
	if !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("identical content was rewritten")
	}
}

func TestHandWrittenFilesGetStableIdentity(t *testing.T) {
	s, dir, _ := open(t)
	os.MkdirAll(filepath.Join(dir, "collections", "Manual", "sub"), 0o755)
	os.WriteFile(filepath.Join(dir, "collections", "Manual", "ping.yaml"), []byte("method: GET\nurl: https://x/ping\n"), 0o644)

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
}

func TestBrokenFileIsReportedByPath(t *testing.T) {
	s, dir, _ := open(t)
	os.MkdirAll(filepath.Join(dir, "collections", "C"), 0o755)
	os.WriteFile(filepath.Join(dir, "collections", "C", "bad.yaml"), []byte("<<<<<<< HEAD\nmethod: GET\n=======\nmethod: POST\n>>>>>>> theirs\n"), 0o644)
	if _, err := s.Collections(); err == nil || !strings.Contains(err.Error(), "bad.yaml") {
		t.Fatalf("want error naming the file, got %v", err)
	}
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

func TestRequestFileIsReadableYAML(t *testing.T) {
	s, dir, _ := open(t)
	r := &model.Request{
		Method:  "POST",
		URL:     "{{baseUrl}}/login",
		Headers: []model.KV{{Key: "Accept", Value: "*/*", Enabled: true}, {Key: "X-Debug", Value: "{{debug}}"}},
		Auth:    model.Auth{Type: "bearer", Token: "{{token}}"},
		Body:    model.Body{Mode: "raw", ContentType: "application/json", Raw: "{\n  \"user\": \"ada\",\n  \"roles\": [\"admin\"]\n}"},
		Tests:   []model.Assertion{{Target: "status", Op: "eq", Value: "200"}, {Target: "body.token", Op: "exists"}},
		Extract: []model.Extract{{Var: "token", From: "body.token"}},
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
	files := map[string]string{
		"_collection.yaml": "id: c1\nname: API\n",
		"Minimal.yaml":     "id: r2\nname: Minimal\nmethod: \"\"\nurl: \"\"\n",
		"Login.yaml": `id: r1
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
`,
		"GraphQL.yaml": `id: r0
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
`,
	}
	for name, want := range files {
		got, _ := os.ReadFile(filepath.Join(dir, "collections", "API", name))
		if string(got) != want {
			t.Errorf("%s:\n%s\nwant:\n%s", name, got, want)
		}
	}
	got, err := s.Collections()
	if err != nil || !reflect.DeepEqual(got, []*model.Collection{c}) {
		t.Fatalf("round trip: %v %#v", err, got[0].Items[1].Request)
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
			file, _ := os.ReadFile(filepath.Join(dir, "collections", "C", want.Name+".yaml"))
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
		c := &model.Collection{ID: "c1", Name: "C", Items: []*model.Item{{ID: "r", Name: "R", Request: &model.Request{Body: model.Body{Mode: "raw", Raw: body}}}}}
		if err := s.SaveCollection(c); err != nil {
			t.Fatal(err)
		}
		file, _ := os.ReadFile(filepath.Join(dir, "collections", "C", "R.yaml"))
		if got := strings.Contains(string(file), "raw: |"); got != block {
			t.Errorf("%q: block scalar = %v, want %v\n%s", body, got, block, file)
		}
		got, err := s.Collections()
		if err != nil || got[0].Items[0].Request.Body.Raw != body {
			t.Errorf("%q did not round trip: %v\n%s", body, err, file)
		}
	}
}

func TestHandWrittenYAMLIsRead(t *testing.T) {
	s, dir, _ := open(t)
	os.MkdirAll(filepath.Join(dir, "collections", "C"), 0o755)
	// Unquoted scalars land in string fields as written; CRLF line endings
	// (git autocrlf on Windows) do not leak into values.
	file := "# login\r\nmethod: POST\r\nurl: http://x/1\r\nbody:\r\n  mode: raw\r\n  raw: |\r\n    {\r\n      \"a\": 1\r\n    }\r\ntests:\r\n  - {target: status, op: eq, value: 200}\r\n  - {target: body.n, op: eq, value: 0123}\r\n  - {target: body.ok, op: eq, value: true}\r\n"
	os.WriteFile(filepath.Join(dir, "collections", "C", "login.yaml"), []byte(file), 0o644)
	os.WriteFile(filepath.Join(dir, "collections", "C", "empty.yaml"), nil, 0o644)
	os.WriteFile(filepath.Join(dir, "collections", "C", "legacy.json"), []byte(`{"method":"GET"}`), 0o644)
	got, err := s.Collections()
	if err != nil {
		t.Fatal(err)
	}
	if len(got[0].Items) != 2 || got[0].Items[0].Name != "empty" {
		t.Fatalf("got %#v", got[0].Items)
	}
	r := got[0].Items[1].Request
	want := []model.Assertion{{Target: "status", Op: "eq", Value: "200"}, {Target: "body.n", Op: "eq", Value: "0123"}, {Target: "body.ok", Op: "eq", Value: "true"}}
	if r.Method != "POST" || r.Body.Raw != "{\n  \"a\": 1\n}\n" || !reflect.DeepEqual(r.Tests, want) {
		t.Fatalf("got %#v", r)
	}
	// Files this app does not own survive a save.
	if err := s.SaveCollection(got[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "collections", "C", "legacy.json")); err != nil {
		t.Fatal("foreign file removed")
	}
}

func TestHistoryIsLocalNewestFirstAndCapped(t *testing.T) {
	s, dir, _ := open(t)
	for i := range historyLimit + 5 {
		s.AddHistory(&model.HistoryEntry{ID: fmt.Sprint(i)})
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

func TestCollectionProtoConfigRoundTrips(t *testing.T) {
	s, dir, _ := open(t)
	file := filepath.Join(dir, "collections", "Users API", "_collection.yaml")
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
	if got, _ := os.ReadFile(file); string(got) != want {
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
		if got, _ := os.ReadFile(file); string(got) != "id: c1\nname: Users API\n" {
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
	// Folders never get one.
	c.Items = []*model.Item{{ID: "f1", Name: "Admin", Items: []*model.Item{}}}
	c.Proto = &model.ProtoConfig{Files: []string{"a.proto"}}
	s.SaveCollection(c)
	if got, _ := os.ReadFile(filepath.Join(dir, "collections", "Users API", "Admin", "_folder.yaml")); string(got) != "id: f1\nname: Admin\n" {
		t.Fatalf("folder:\n%s", got)
	}
}
