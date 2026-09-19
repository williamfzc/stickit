package e2e

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// add serves one note object; the key set is the agent contract.
func TestAddServesFullNoteObject(t *testing.T) {
	env := newEnv(t)
	repo := newGitRepo(t, env)
	writeLines(t, filepath.Join(repo, "file.go"), "one", "two", "three", "four", "five")

	stdout := mustRun(t, repo, env, "add", "file.go:2", "mind the gap #gotcha")

	want := []string{"id", "file", "start_line", "end_line", "status", "drifted",
		"tags", "author", "branch", "commit", "body", "created_at", "updated_at"}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal([]byte(stdout), &keys); err != nil {
		t.Fatalf("decode add output %q: %v", stdout, err)
	}
	if len(keys) != len(want) {
		t.Fatalf("add output keys = %v, want exactly %v", keysOf(keys), want)
	}
	for _, k := range want {
		if _, ok := keys[k]; !ok {
			t.Fatalf("add output is missing key %q, has %v", k, keysOf(keys))
		}
	}

	n := decodeNote(t, stdout, "add")
	if n.ID == "" {
		t.Fatal("add: id must not be empty")
	}
	if n.File != "file.go" {
		t.Fatalf("add: file = %q, want file.go", n.File)
	}
	requireAnchor(t, n, 2, 2, "add")
	requireStatus(t, n, "active", "add")
	if n.Drifted {
		t.Fatal("add: a fresh note must not be flagged drifted")
	}
	if len(n.Tags) != 1 || n.Tags[0] != "gotcha" {
		t.Fatalf("add: tags = %v, want [gotcha]", n.Tags)
	}
	if n.Author != agentName {
		t.Fatalf("add: author = %q, want NOTES_AGENT %q", n.Author, agentName)
	}
	requireBranch(t, n, "main", "add")
	requireCommit(t, n, git(t, repo, env, "rev-parse", "HEAD"), "add")
	if n.Body != "mind the gap #gotcha" {
		t.Fatalf("add: body = %q", n.Body)
	}
	if n.CreatedAt == "" || n.CreatedAt != n.UpdatedAt {
		t.Fatalf("add: created_at %q / updated_at %q must match and be set", n.CreatedAt, n.UpdatedAt)
	}
}

// A target without lines is a file-level note: null lines, null branch
// outside git.
func TestAddFileLevelNoteOutsideGit(t *testing.T) {
	env := newEnv(t)
	dir := newPlainDir(t)
	writeLines(t, filepath.Join(dir, "notes.md"), "prose line")

	n := mustAdd(t, dir, env, "notes.md", "file-wide remark")
	requireNoAnchor(t, n, "file-level add")
	requireStatus(t, n, "active", "file-level add")
	requireBranch(t, n, "", "file-level add")
	requireCommit(t, n, "", "file-level add")
	if n.Author != agentName {
		t.Fatalf("file-level add: author = %q, want %q", n.Author, agentName)
	}
	if n.Tags != nil && len(n.Tags) != 0 {
		t.Fatalf("file-level add: tags = %v, want empty", n.Tags)
	}
}

func TestAddRangeAnchor(t *testing.T) {
	env := newEnv(t)
	dir := newPlainDir(t)
	writeLines(t, filepath.Join(dir, "file.go"), "one", "two", "three", "four")

	n := mustAdd(t, dir, env, "file.go:2-3", "spans two lines")
	requireAnchor(t, n, 2, 3, "range add")
	requireStatus(t, n, "active", "range add")
}

// #hashtags are parsed from the body: lowercased, unique, first-seen order.
func TestAddParsesTags(t *testing.T) {
	env := newEnv(t)
	dir := t.TempDir()
	writeLines(t, filepath.Join(dir, "file.go"), "one")

	n := mustAdd(t, dir, env, "file.go:1", "#Alpha then #alpha again, #Beta_beta and #x9")
	want := []string{"alpha", "beta_beta", "x9"}
	if len(n.Tags) != len(want) {
		t.Fatalf("tags = %v, want %v", n.Tags, want)
	}
	for i := range want {
		if n.Tags[i] != want[i] {
			t.Fatalf("tags = %v, want %v", n.Tags, want)
		}
	}
}

// resolve archives: hidden by default, shown as archived under --all, and
// idempotent.
func TestResolveArchives(t *testing.T) {
	env := newEnv(t)
	dir := newPlainDir(t)
	writeLines(t, filepath.Join(dir, "file.go"), "one", "two")
	n := mustAdd(t, dir, env, "file.go:1", "addressed review comment")

	resolved := decodeNote(t, mustRun(t, dir, env, "resolve", n.ID), "resolve")
	if resolved.ID != n.ID {
		t.Fatalf("resolve: id = %q, want %q", resolved.ID, n.ID)
	}
	requireStatus(t, resolved, "archived", "resolve")

	if notes := mustList(t, dir, env, "file.go"); len(notes) != 0 {
		t.Fatalf("resolved note must be hidden by default, ls = %+v", notes)
	}
	all := singleNote(t, mustList(t, dir, env, "--all"), "ls --all after resolve")
	if all.ID != n.ID {
		t.Fatalf("ls --all = %+v, want the resolved note %s", all, n.ID)
	}
	requireStatus(t, all, "archived", "ls --all after resolve")

	// Resolving again succeeds.
	if _, stderr, code := run(t, dir, env, "resolve", n.ID); code != 0 {
		t.Fatalf("second resolve: exit %d, want 0 (stderr %q)", code, stderr)
	}
}
