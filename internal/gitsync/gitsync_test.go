package gitsync

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

var ctx = context.Background()

func setup(t *testing.T) {
	t.Helper()
	if !Available() {
		t.Skip("git not installed")
	}
	// Isolate from the developer's own git config and identity.
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "gitconfig"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	for _, k := range []string{"GIT_AUTHOR_NAME", "GIT_COMMITTER_NAME"} {
		t.Setenv(k, "Test")
	}
	for _, k := range []string{"GIT_AUTHOR_EMAIL", "GIT_COMMITTER_EMAIL"} {
		t.Setenv(k, "test@example.com")
	}
}

func run(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func write(t *testing.T, dir, rel, content string) {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(rel))
	os.MkdirAll(filepath.Dir(path), 0o755)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// remote creates an empty bare repository to stand in for GitHub.
func remote(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "remote.git")
	run(t, "", "init", "--bare", "-b", "main", dir)
	return dir
}

func clone(t *testing.T, url string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "clone")
	run(t, "", "clone", url, dir)
	return dir
}

func TestStatusOutsideRepo(t *testing.T) {
	setup(t)
	st, err := GetStatus(ctx, t.TempDir())
	if err != nil || st.Repo || st.Changes == nil {
		t.Fatalf("got %#v %v", st, err)
	}
	if err := Sync(ctx, t.TempDir(), "x"); err == nil {
		t.Fatal("sync outside a repository should fail")
	}
}

func TestSyncRoundTripBetweenTwoClones(t *testing.T) {
	setup(t)
	url := remote(t)
	alice, bob := clone(t, url), clone(t, url)

	write(t, alice, "collections/API/Get user.yaml", "method: GET\n")
	write(t, alice, "collections/API/Old.yaml", "method: GET\n")
	write(t, alice, "environments/Dev.yaml", "not ours either")
	write(t, alice, "notes.txt", "not ours")

	st, _ := GetStatus(ctx, alice)
	want := []Change{{"collections/API/Get user.yaml", "added"}, {"collections/API/Old.yaml", "added"}}
	if !st.Repo || st.Remote != url || st.Branch != "main" || !reflect.DeepEqual(st.Changes, want) {
		t.Fatalf("status before sync: %#v", st)
	}

	// First push into an empty remote.
	if err := Sync(ctx, alice, "Add user request"); err != nil {
		t.Fatal(err)
	}
	st, _ = GetStatus(ctx, alice)
	if len(st.Changes) != 0 || !st.Upstream || st.Ahead != 0 {
		t.Fatalf("status after sync: %#v", st)
	}
	if got := run(t, alice, "log", "-1", "--format=%s"); got != "Add user request" {
		t.Fatalf("commit message: %q", got)
	}
	if got := run(t, alice, "status", "--porcelain"); got != "?? environments/\n?? notes.txt" {
		t.Fatalf("files outside the managed directories must be left alone: %q", got)
	}

	// Bob has an unrelated local change; syncing merges both sides.
	write(t, bob, "collections/API/List users.yaml", "method: GET\n")
	if err := Sync(ctx, bob, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(bob, "collections", "API", "Get user.yaml")); err != nil {
		t.Fatal("bob did not receive alice's request")
	}
	if got := run(t, bob, "log", "-1", "--format=%s"); got != "Update requests" {
		t.Fatalf("default commit message: %q", got)
	}

	// Alice deletes a file and edits nothing else; pull brings Bob's file.
	os.Remove(filepath.Join(alice, "collections", "API", "Old.yaml"))
	st, _ = GetStatus(ctx, alice)
	if !reflect.DeepEqual(st.Changes, []Change{{"collections/API/Old.yaml", "deleted"}}) {
		t.Fatalf("deletion: %#v", st.Changes)
	}
	if err := Sync(ctx, alice, "Remove old"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(alice, "collections", "API", "List users.yaml")); err != nil {
		t.Fatal("alice did not receive bob's request")
	}
	if err := Pull(ctx, bob); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(bob, "collections", "API", "Old.yaml")); err == nil {
		t.Fatal("deletion did not reach bob")
	}
}

func TestConflictLeavesWorkspaceIntact(t *testing.T) {
	setup(t)
	url := remote(t)
	alice, bob := clone(t, url), clone(t, url)
	write(t, alice, "collections/API/R.yaml", "base\n")
	if err := Sync(ctx, alice, "base"); err != nil {
		t.Fatal(err)
	}
	if err := Pull(ctx, bob); err != nil {
		t.Fatal(err)
	}

	write(t, alice, "collections/API/R.yaml", "alice\n")
	Sync(ctx, alice, "alice")
	write(t, bob, "collections/API/R.yaml", "bob\n")

	err := Sync(ctx, bob, "bob")
	if err == nil || !strings.Contains(err.Error(), "conflict") {
		t.Fatalf("want conflict error, got %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(bob, "collections", "API", "R.yaml"))
	if string(got) != "bob\n" {
		t.Fatalf("bob's version was lost: %q", got)
	}
	if _, err := os.Stat(filepath.Join(bob, ".git", "rebase-merge")); err == nil {
		t.Fatal("rebase left in progress")
	}
}
