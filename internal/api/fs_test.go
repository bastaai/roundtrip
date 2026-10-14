package api

import (
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestListDir(t *testing.T) {
	c := setup(t)
	root := mustEval(t, t.TempDir())
	for _, dir := range []string{"protos/billing", "Zeta", ".git", "alpha"} {
		os.MkdirAll(filepath.Join(root, filepath.FromSlash(dir)), 0o755)
	}
	for _, file := range []string{"users.proto", "README.md", "Orders.PROTO", ".hidden.proto"} {
		os.WriteFile(filepath.Join(root, file), nil, 0o644)
	}
	list := func(query string) (fsListing, int) {
		var out fsListing
		code := c.do("GET", "/api/fs/list?"+query, nil, &out)
		return out, code
	}
	q := func(path, ext string) string {
		return url.Values{"path": {path}, "ext": {ext}}.Encode()
	}

	// Folders only, hidden ones left out, sorted without regard to case.
	got, code := list(q(root, ""))
	want := []fsEntry{{"alpha", true}, {"protos", true}, {"Zeta", true}}
	if code != 200 || got.Path != root || got.Parent != filepath.Dir(root) || !reflect.DeepEqual(got.Entries, want) {
		t.Fatalf("folders: %d %#v", code, got)
	}
	if got.Home == "" || got.Workspace == "" {
		t.Fatalf("shortcuts missing: %#v", got)
	}

	// With an extension, matching files follow the folders.
	got, _ = list(q(root, ".proto"))
	want = append(want, fsEntry{"Orders.PROTO", false}, fsEntry{"users.proto", false})
	if !reflect.DeepEqual(got.Entries, want) {
		t.Fatalf("files: %#v", got.Entries)
	}

	// No path starts in the workspace; an empty folder lists as empty, not null.
	got, code = list("")
	if code != 200 || got.Path != got.Workspace || got.Entries == nil {
		t.Fatalf("default: %d %#v", code, got)
	}

	if got, code = list(q("~", "")); code != 200 || got.Path != got.Home {
		t.Fatalf("home: %d %#v", code, got)
	}
	if code := c.do("GET", "/api/fs/list?"+q("relative/path", ""), nil, nil); code != 400 {
		t.Fatalf("relative path: %d", code)
	}
	if code := c.do("GET", "/api/fs/list?"+q(filepath.Join(root, "missing"), ""), nil, nil); code != 400 {
		t.Fatalf("missing folder: %d", code)
	}
}
