// Package gitsync commits the workspace and syncs it with its remote by
// running the system git, so the user's existing SSH keys and credential
// helpers just work.
package gitsync

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Only these directories are ever staged, so a workspace that sits inside a
// larger repository never drags unrelated changes into a commit. Environments
// are per-machine and are not part of the workspace at all.
var managed = []string{"collections"}

type Change struct {
	Path   string `json:"path"`
	Status string `json:"status"` // added, modified, deleted
}

type Status struct {
	Repo     bool     `json:"repo"`
	Remote   string   `json:"remote"`
	Branch   string   `json:"branch"`
	Upstream bool     `json:"upstream"`
	Ahead    int      `json:"ahead"`
	Behind   int      `json:"behind"`
	Changes  []Change `json:"changes"`
}

func Available() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

func git(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-c", "core.quotepath=false"}, args...)...)
	cmd.Dir = dir
	// Fail instead of hanging on a credentials prompt nobody can see.
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", args[0], msg)
	}
	return strings.TrimRight(stdout.String(), "\n"), nil
}

func IsRepo(ctx context.Context, dir string) bool {
	out, err := git(ctx, dir, "rev-parse", "--is-inside-work-tree")
	return err == nil && out == "true"
}

// paths returns the managed directories git should look at: those on disk
// plus those that only exist in the index (so deletions get committed).
func paths(ctx context.Context, dir string) []string {
	var out []string
	for _, p := range managed {
		if _, err := os.Stat(filepath.Join(dir, p)); err == nil {
			out = append(out, p)
		} else if tracked, _ := git(ctx, dir, "ls-files", "--", p); tracked != "" {
			out = append(out, p)
		}
	}
	return out
}

func GetStatus(ctx context.Context, dir string) (Status, error) {
	st := Status{Changes: []Change{}}
	if !Available() || !IsRepo(ctx, dir) {
		return st, nil
	}
	st.Repo = true
	st.Remote, _ = git(ctx, dir, "remote", "get-url", "origin")
	st.Branch, _ = git(ctx, dir, "symbolic-ref", "--short", "HEAD")

	if counts, err := git(ctx, dir, "rev-list", "--left-right", "--count", "@{u}...HEAD"); err == nil {
		st.Upstream = true
		if behind, ahead, ok := strings.Cut(counts, "\t"); ok {
			st.Behind, _ = strconv.Atoi(behind)
			st.Ahead, _ = strconv.Atoi(ahead)
		}
	}

	ps := paths(ctx, dir)
	if len(ps) == 0 {
		return st, nil
	}
	out, err := git(ctx, dir, append([]string{"status", "--porcelain", "--untracked-files=all", "--no-renames", "--"}, ps...)...)
	if err != nil {
		return st, err
	}
	for _, line := range strings.Split(out, "\n") {
		if len(line) < 4 {
			continue
		}
		code, path := line[:2], strings.Trim(line[3:], `"`)
		status := "modified"
		switch {
		case strings.Contains(code, "?"), strings.Contains(code, "A"):
			status = "added"
		case strings.Contains(code, "D"):
			status = "deleted"
		}
		st.Changes = append(st.Changes, Change{Path: path, Status: status})
	}
	return st, nil
}

// Clone rejects anything that isn't a plain repository address, since the
// address ends up on a git command line.
func Clone(ctx context.Context, url, dir string) error {
	url = strings.TrimSpace(url)
	ok := filepath.IsAbs(url)
	for _, prefix := range []string{"https://", "http://", "ssh://", "git@"} {
		ok = ok || strings.HasPrefix(url, prefix)
	}
	if !ok {
		return errors.New("repository must be an https://, ssh:// or git@ address")
	}
	_, err := git(ctx, "", "clone", "--", url, dir)
	return err
}

// Pull rebases local commits onto the remote. Conflicts abort the rebase and
// leave the workspace as it was.
func Pull(ctx context.Context, dir string) error {
	if remote, _ := git(ctx, dir, "remote", "get-url", "origin"); remote == "" {
		return nil
	}
	branch, err := git(ctx, dir, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		return err
	}
	if _, err := git(ctx, dir, "fetch", "origin"); err != nil {
		return err
	}
	if _, err := git(ctx, dir, "rev-parse", "--verify", "--quiet", "refs/remotes/origin/"+branch); err != nil {
		// The remote doesn't have this branch yet (e.g. a brand new repository).
		return nil
	}
	if _, err := git(ctx, dir, "rev-parse", "--verify", "--quiet", "HEAD"); err != nil {
		// No local commits yet: just take what the remote has.
		_, err := git(ctx, dir, "merge", "--ff-only", "origin/"+branch)
		return err
	}
	if _, err := git(ctx, dir, "pull", "--rebase", "--autostash", "origin", branch); err != nil {
		git(ctx, dir, "rebase", "--abort")
		return fmt.Errorf("%w\n\nYour changes conflict with the remote. Resolve them with git in %s, then sync again", err, dir)
	}
	return nil
}

// Sync commits pending changes in the managed directories, pulls, and pushes.
func Sync(ctx context.Context, dir, message string) error {
	st, err := GetStatus(ctx, dir)
	if err != nil {
		return err
	}
	if !st.Repo {
		return errors.New("the workspace is not a git repository")
	}
	if len(st.Changes) > 0 {
		if strings.TrimSpace(message) == "" {
			message = "Update requests"
		}
		ps := paths(ctx, dir)
		if _, err := git(ctx, dir, append([]string{"add", "-A", "--"}, ps...)...); err != nil {
			return err
		}
		if _, err := git(ctx, dir, append([]string{"commit", "-m", message, "--"}, ps...)...); err != nil {
			return err
		}
	}
	if st.Remote == "" {
		return nil
	}
	if err := Pull(ctx, dir); err != nil {
		return err
	}
	if _, err := git(ctx, dir, "rev-parse", "--verify", "--quiet", "HEAD"); err != nil {
		// No commits at all: nothing to push.
		return nil
	}
	_, err = git(ctx, dir, "push", "-u", "origin", "HEAD")
	return err
}
