package board

import (
	"os"
	"path/filepath"
	"testing"
)

// wtBoard simulates a linked worktree: the board is keyed by the main
// repository Root, but the user's files live under TopLevel. Both are
// symlink-resolved, as board.Detect does in production.
func wtBoard(t *testing.T) (Board, string) {
	t.Helper()
	real := func(p string) string {
		rp, err := filepath.EvalSymlinks(p)
		if err != nil {
			t.Fatal(err)
		}
		return rp
	}
	main := real(t.TempDir())
	wt := real(t.TempDir())
	file := filepath.Join(wt, "src", "a.go")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("l1\nl2\nl3\nl4\nl5\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return Board{Root: main, TopLevel: wt, InGit: true, Branch: "feature-x"}, file
}

func TestRelFromBoardWorktree(t *testing.T) {
	b, file := wtBoard(t)
	rel, err := RelFromBoard(b, file)
	if err != nil {
		t.Fatalf("a file inside a linked worktree must be addressable: %v", err)
	}
	if rel != "src/a.go" {
		t.Fatalf("rel = %q, want src/a.go", rel)
	}
}

func TestRelFromBoardRejectsEscape(t *testing.T) {
	b, _ := wtBoard(t)
	outside := filepath.Join(t.TempDir(), "other.txt")
	if err := os.WriteFile(outside, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := RelFromBoard(b, outside); err == nil {
		t.Fatal("a file outside the working tree must be rejected")
	}
}

func TestParseTargetForms(t *testing.T) {
	b, _ := wtBoard(t)
	t.Chdir(b.TopLevel) // user paths resolve against the working directory
	colon := filepath.Join(b.TopLevel, "weird:name.txt")
	if err := os.WriteFile(colon, []byte("l1\nl2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	empty := filepath.Join(b.TopLevel, "empty.txt")
	if err := os.WriteFile(empty, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		spec      string
		file      string
		start     int
		end       int
		wantError bool
	}{
		{spec: "nope.go", wantError: true},       // no such file
		{spec: "src/a.go:0", wantError: true},    // zero line
		{spec: "src/a.go:5-3", wantError: true},  // inverted range
		{spec: "src/a.go:1-x", wantError: true},  // malformed lines
		{spec: "src/a.go:1-99", wantError: true}, // range past EOF
		{spec: "src", wantError: true},           // a directory, not a file
		{spec: "empty.txt:1", wantError: true},   // an empty file has no lines
		{spec: "src/a.go", file: "src/a.go"},
		{spec: "src/a.go:1", file: "src/a.go", start: 1, end: 1},
		{spec: "src/a.go:2-3", file: "src/a.go", start: 2, end: 3},
		// A name containing a colon: the whole spec names a real file, and a
		// numeric suffix after it is still a line spec.
		{spec: "weird:name.txt", file: "weird:name.txt"},
		{spec: "weird:name.txt:2", file: "weird:name.txt", start: 2, end: 2},
	}
	for _, c := range cases {
		tg, err := ParseTarget(b, c.spec)
		if c.wantError {
			if err == nil {
				t.Errorf("ParseTarget(%q) must fail", c.spec)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseTarget(%q) failed: %v", c.spec, err)
			continue
		}
		if tg.File != c.file || tg.Start != c.start || tg.End != c.end {
			t.Errorf("ParseTarget(%q) = %+v, want file %q %d-%d", c.spec, tg, c.file, c.start, c.end)
		}
		if !filepath.IsAbs(tg.AbsPath) {
			t.Errorf("ParseTarget(%q) abs path %q must be absolute", c.spec, tg.AbsPath)
		}
	}
}

func TestAbsUserPathSymlinks(t *testing.T) {
	// The file need not exist yet: the deepest existing ancestor is resolved.
	abs, err := AbsUserPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AbsUserPath(filepath.Join(abs, "missing", "child.txt")); err != nil {
		t.Fatalf("missing descendants must resolve through their ancestor: %v", err)
	}
}
