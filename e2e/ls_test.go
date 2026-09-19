package e2e

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestLsEmptyBoardIsEmptyArray(t *testing.T) {
	env := newEnv(t)
	dir := newPlainDir(t)
	stdout := mustRun(t, dir, env, "ls")
	if got := strings.TrimSpace(stdout); got != "[]" {
		t.Fatalf("ls on an empty board = %q, want []", got)
	}
}

// An ls path filter names one file exactly — from the repo root and from a
// subdirectory with a cwd-relative argument.
func TestLsExactFileFilterFromSubdir(t *testing.T) {
	env := newEnv(t)
	repo := newGitRepo(t, env)
	writeLines(t, filepath.Join(repo, "src", "alpha.go"), "a1", "a2", "a3", "a4")
	writeLines(t, filepath.Join(repo, "src", "beta.go"), "b1", "b2")
	writeLines(t, filepath.Join(repo, "gamma.go"), "g1", "g2")
	first := mustAdd(t, repo, env, "src/alpha.go:1", "first on alpha")
	second := mustAdd(t, repo, env, "src/alpha.go:3", "second on alpha")
	mustAdd(t, repo, env, "src/beta.go:1", "on beta")
	mustAdd(t, repo, env, "gamma.go:1", "on gamma")

	fromRoot := mustList(t, repo, env, "src/alpha.go")
	if len(fromRoot) != 2 {
		t.Fatalf("ls src/alpha.go from root = %d notes, want 2", len(fromRoot))
	}
	// Same board file key, ordered by line.
	if fromRoot[0].ID != first.ID || fromRoot[1].ID != second.ID {
		t.Fatalf("ls src/alpha.go order = %s, %s; want %s, %s",
			fromRoot[0].ID, fromRoot[1].ID, first.ID, second.ID)
	}
	for _, n := range fromRoot {
		if n.File != "src/alpha.go" {
			t.Fatalf("note file = %q, want src/alpha.go", n.File)
		}
	}

	fromSub := mustList(t, filepath.Join(repo, "src"), env, "alpha.go")
	if len(fromSub) != 2 || fromSub[0].ID != first.ID || fromSub[1].ID != second.ID {
		t.Fatalf("ls alpha.go from src/ = %+v, want the same two notes", fromSub)
	}
}

// A directory argument filters by prefix, with or without the trailing
// slash.
func TestLsDirectoryFilter(t *testing.T) {
	env := newEnv(t)
	repo := newGitRepo(t, env)
	writeLines(t, filepath.Join(repo, "src", "alpha.go"), "a1", "a2")
	writeLines(t, filepath.Join(repo, "src", "beta.go"), "b1", "b2")
	writeLines(t, filepath.Join(repo, "gamma.go"), "g1", "g2")
	mustAdd(t, repo, env, "src/alpha.go:1", "on alpha")
	mustAdd(t, repo, env, "src/beta.go:1", "on beta")
	mustAdd(t, repo, env, "gamma.go:1", "outside src")

	for _, arg := range []string{"src/", "src"} {
		notes := mustList(t, repo, env, arg)
		if len(notes) != 2 {
			t.Fatalf("ls %s = %d notes, want 2: %+v", arg, len(notes), notes)
		}
		for _, n := range notes {
			if !strings.HasPrefix(n.File, "src/") {
				t.Fatalf("ls %s surfaced %q, want only files under src/", arg, n.File)
			}
		}
	}
}

// searchFixture pins three notes: A and B mention kafka, C mentions
// nothing relevant.
func searchFixture(t *testing.T) (dir string, env []string, a, b, c note) {
	t.Helper()
	env = newEnv(t)
	dir = t.TempDir()
	for _, f := range []string{"a.go", "b.go", "c.go"} {
		writeLines(t, filepath.Join(dir, f), "x")
	}
	a = mustAdd(t, dir, env, "a.go:1", "connects to kafka before startup")
	b = mustAdd(t, dir, env, "b.go:1", "kafka partitions must be bumped first")
	c = mustAdd(t, dir, env, "c.go:1", "nothing relevant here")
	return dir, env, a, b, c
}

// A keyword searches note bodies.
func TestLsKeywordSearchesBodies(t *testing.T) {
	dir, env, a, b, c := searchFixture(t)

	got := byID(t, mustList(t, dir, env, "kafka"))
	if len(got) != 2 {
		t.Fatalf("ls kafka = %d notes, want exactly A and B", len(got))
	}
	if _, ok := got[a.ID]; !ok {
		t.Fatalf("ls kafka missed the note whose body mentions kafka: %v", got)
	}
	if _, ok := got[b.ID]; !ok {
		t.Fatalf("ls kafka missed the second kafka note: %v", got)
	}
	if _, ok := got[c.ID]; ok {
		t.Fatalf("ls kafka surfaced the unrelated note %s", c.ID)
	}
}

// Two positionals combine a path filter with a keyword.
func TestLsPathAndKeywordCombine(t *testing.T) {
	dir, env, a, b, _ := searchFixture(t)

	for _, tc := range []struct {
		file string
		want string
	}{
		{"a.go", a.ID},
		{"b.go", b.ID},
	} {
		got := singleNote(t, mustList(t, dir, env, tc.file, "kafka"), "ls "+tc.file+" kafka")
		if got.ID != tc.want {
			t.Fatalf("ls %s kafka = %s, want %s", tc.file, got.ID, tc.want)
		}
	}
}

// A multi-token keyword (one quoted argument) is an AND over the note's
// body.
func TestLsMultiTokenKeywordIsAND(t *testing.T) {
	env := newEnv(t)
	dir := t.TempDir()
	writeLines(t, filepath.Join(dir, "d.go"), "x")
	writeLines(t, filepath.Join(dir, "e.go"), "x")
	d := mustAdd(t, dir, env, "d.go:1", "alpha marker, and beta details, in one body")
	mustAdd(t, dir, env, "e.go:1", "alpha marker only")

	got := singleNote(t, mustList(t, dir, env, "alpha beta"), `ls "alpha beta"`)
	if got.ID != d.ID {
		t.Fatalf(`ls "alpha beta" = %s, want the note carrying both tokens`, got.ID)
	}

	// A token nobody carries: empty result, never the whole board.
	if notes := mustList(t, dir, env, "alpha gamma"); len(notes) != 0 {
		t.Fatalf(`ls "alpha gamma" = %+v, want no matches`, notes)
	}
}

func TestLsKeywordWithoutHitsIsEmpty(t *testing.T) {
	dir, env, _, _, _ := searchFixture(t)
	if notes := mustList(t, dir, env, "zzznohit"); len(notes) != 0 {
		t.Fatalf("no-hit keyword returned the whole board: %+v", notes)
	}
}
