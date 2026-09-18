package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestAnchorLifecycle walks one note through the whole drift story on a
// five-line file: silent drift on insertion, whitespace-insensitive
// re-anchoring (without re-drift churn on later reads), staleness on content
// replacement, recovery when the content returns, and staleness when the
// content moves beyond the ±50-line re-anchor reach.
func TestAnchorLifecycle(t *testing.T) {
	env := newEnv(t)
	dir := newPlainDir(t)
	file := filepath.Join(dir, "file.go")
	writeLines(t, file, "alpha", "bravo", "charlie", "delta", "echo")

	mustAdd(t, dir, env, "file.go:2", "watch line two")
	board := func(what string) note {
		return singleNote(t, mustList(t, dir, env), what)
	}

	// Fresh anchor.
	got := board("fresh")
	requireAnchor(t, got, 2, 2, "fresh")
	requireStatus(t, got, "active", "fresh")
	if got.Drifted {
		t.Fatal("fresh: a just-pinned note must not be flagged drifted")
	}

	// One line inserted above: the anchor silently follows and is flagged.
	writeLines(t, file, "zero", "alpha", "bravo", "charlie", "delta", "echo")
	got = board("after insert above")
	requireAnchor(t, got, 3, 3, "after insert above")
	requireStatus(t, got, "active", "after insert above")
	if !got.Drifted {
		t.Fatal("after insert above: a moved anchor must be flagged drifted")
	}

	// Whitespace-only reindent of the anchored line: still active and
	// drifted. The move is re-baselined, so a second consecutive read must
	// not touch updated_at — otherwise anchors would re-drift forever.
	writeLines(t, file, "zero", "alpha", "   bravo   ", "charlie", "delta", "echo")
	got = board("after reindent")
	requireAnchor(t, got, 3, 3, "after reindent")
	requireStatus(t, got, "active", "after reindent")
	if !got.Drifted {
		t.Fatal("after reindent: a re-anchored note stays flagged drifted")
	}
	again := board("second read after reindent")
	if again.UpdatedAt != got.UpdatedAt {
		t.Fatalf("second consecutive ls changed updated_at from %s to %s — the anchor is re-drifting",
			got.UpdatedAt, again.UpdatedAt)
	}

	// The anchored content is replaced: the note goes stale, visibly.
	writeLines(t, file, "zero", "alpha", "brave", "charlie", "delta", "echo")
	got = board("after replacing content")
	requireStatus(t, got, "stale", "after replacing content")

	// The content comes back: the note recovers to active.
	writeLines(t, file, "zero", "alpha", "   bravo   ", "charlie", "delta", "echo")
	got = board("after restoring content")
	requireStatus(t, got, "active", "after restoring content")
	if !got.Drifted {
		t.Fatal("after restoring content: the drifted flag persists")
	}

	// The content survives but moves farther than the ±50-line reach.
	far := make([]string, 0, 66)
	for i := 0; i < 60; i++ {
		far = append(far, fmt.Sprintf("filler %02d", i))
	}
	far = append(far, "zero", "alpha", "   bravo   ", "charlie", "delta", "echo")
	writeLines(t, file, far...)
	got = board("after moving content beyond reach")
	requireStatus(t, got, "stale", "after moving content beyond reach")
}

// A #handoff note whose file vanished expires outright: hidden from the
// default board, archived under --all. A #gotcha note on the same missing
// file stays visible, flagged stale.
func TestHandoffArchivedWhenFileDeleted(t *testing.T) {
	env := newEnv(t)
	dir := newPlainDir(t)
	writeLines(t, filepath.Join(dir, "hand.go"), "one")
	writeLines(t, filepath.Join(dir, "got.go"), "one")
	handoff := mustAdd(t, dir, env, "hand.go:1", "finish the migration #handoff")
	gotcha := mustAdd(t, dir, env, "got.go:1", "parser assumes gbk input #gotcha")

	if err := os.Remove(filepath.Join(dir, "hand.go")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "got.go")); err != nil {
		t.Fatal(err)
	}

	notes := mustList(t, dir, env)
	if len(notes) != 1 || notes[0].ID != gotcha.ID {
		t.Fatalf("default ls = %+v, want only the gotcha note %s", notes, gotcha.ID)
	}
	requireStatus(t, notes[0], "stale", "deleted file stales the gotcha note")

	all := byID(t, mustList(t, dir, env, "--all"))
	if len(all) != 2 {
		t.Fatalf("ls --all = %v, want both notes", all)
	}
	requireStatus(t, all[handoff.ID], "archived", "handoff note after expiry")
	requireStatus(t, all[gotcha.ID], "stale", "gotcha note after expiry")
}

// Line content hashing is platform-stable: CRLF files validate fresh.
func TestCRLFNoteStaysFresh(t *testing.T) {
	env := newEnv(t)
	dir := newPlainDir(t)
	writeFile(t, filepath.Join(dir, "crlf.txt"), "one\r\ntwo\r\nthree\r\n")

	mustAdd(t, dir, env, "crlf.txt:2", "anchored across CRLF")
	got := singleNote(t, mustList(t, dir, env), "ls CRLF")
	requireAnchor(t, got, 2, 2, "CRLF")
	requireStatus(t, got, "active", "CRLF")
	if got.Drifted {
		t.Fatal("CRLF: an unchanged CRLF file must not mark the note drifted")
	}
}
