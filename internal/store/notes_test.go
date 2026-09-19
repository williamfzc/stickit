package store

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/williamfzc/stickit/internal/anchor"
)

// files is an in-memory working tree for lazy validation: rel → content
// (missing key = missing file).
type files map[string]string

func (f files) content() Content {
	return func(rel string) ([]string, bool, error) {
		c, ok := f[rel]
		if !ok {
			return nil, false, nil
		}
		return strings.Split(strings.TrimSuffix(c, "\n"), "\n"), true, nil
	}
}

func openStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func addNote(t *testing.T, s *Store, f files, file string, start, end int, body string) Note {
	t.Helper()
	n := NewNote{RepoKey: "k1", File: file, Start: start, End: end, Body: body, Author: "a", Branch: "main"}
	if start > 0 {
		lines := strings.Split(strings.TrimSuffix(f[file], "\n"), "\n")
		blk, err := anchor.Lines(lines, start, end)
		if err != nil {
			t.Fatal(err)
		}
		n.Hash, n.NormHash = anchor.Hash(blk), anchor.NormHash(blk)
	}
	note, err := s.AddNote(n)
	if err != nil {
		t.Fatal(err)
	}
	return note
}

func list(t *testing.T, s *Store, f files, flt Filter) []Note {
	t.Helper()
	notes, err := s.List(flt, f.content())
	if err != nil {
		t.Fatal(err)
	}
	return notes
}

func TestParseTags(t *testing.T) {
	got := ParseTags("Fix #Gotcha then #gotcha and #multi_word-tag #1x")
	want := []string{"gotcha", "multi_word-tag", "1x"}
	if len(got) != len(want) {
		t.Fatalf("tags = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("tags = %v, want %v", got, want)
		}
	}
}

func TestAddListRoundtrip(t *testing.T) {
	s := openStore(t)
	f := files{"a.go": "one\ntwo\nthree\n"}
	n := addNote(t, s, f, "a.go", 2, 2, "watch out #gotcha")
	if n.ID == "" || n.Status != "active" || n.Drifted || n.Tags[0] != "gotcha" {
		t.Fatalf("unexpected note: %+v", n)
	}
	if n.StartLine == nil || *n.StartLine != 2 || n.EndLine == nil || *n.EndLine != 2 {
		t.Fatalf("anchor not stored: %+v", n)
	}
	notes := list(t, s, f, Filter{RepoKey: "k1"})
	if len(notes) != 1 || notes[0].ID != n.ID {
		t.Fatalf("ls = %+v", notes)
	}
}

func TestFileLevelNote(t *testing.T) {
	s := openStore(t)
	f := files{"doc.md": "prose\n"}
	n := addNote(t, s, f, "doc.md", 0, 0, "file-wide remark")
	if n.StartLine != nil || n.EndLine != nil {
		t.Fatalf("file-level note must have null lines: %+v", n)
	}
	// file gone → stale; back → active again
	delete(f, "doc.md")
	if got := list(t, s, f, Filter{RepoKey: "k1"})[0].Status; got != "stale" {
		t.Fatalf("deleted file must stale a file-level note, got %q", got)
	}
	f["doc.md"] = "prose\n"
	if got := list(t, s, f, Filter{RepoKey: "k1"})[0].Status; got != "active" {
		t.Fatalf("restored file must reactivate the note, got %q", got)
	}
}

func TestKeywordSearch(t *testing.T) {
	s := openStore(t)
	f := files{"a.go": "x\n", "b.go": "y\n"}
	addNote(t, s, f, "a.go", 1, 1, "parser assumes utf8; the legacy feed is gbk")
	addNote(t, s, f, "b.go", 1, 1, "unrelated")

	got := list(t, s, f, Filter{RepoKey: "k1", Keyword: "gbk"})
	if len(got) != 1 || got[0].File != "a.go" {
		t.Fatalf("keyword = %+v", got)
	}
	// multi-token keyword is an AND over the note's body
	if got := list(t, s, f, Filter{RepoKey: "k1", Keyword: "utf8 gbk"}); len(got) != 1 {
		t.Fatalf("AND search = %+v", got)
	}
	if got := list(t, s, f, Filter{RepoKey: "k1", Keyword: "utf8 missing"}); len(got) != 0 {
		t.Fatalf("unsatisfiable search = %+v", got)
	}
}

func TestResolveLifecycle(t *testing.T) {
	s := openStore(t)
	f := files{"a.go": "x\n"}
	n := addNote(t, s, f, "a.go", 1, 1, "done soon")
	if _, err := s.Resolve("k1", "nope"); err != ErrNotFound {
		t.Fatalf("resolve unknown = %v, want ErrNotFound", err)
	}
	if r, err := s.Resolve("k1", n.ID); err != nil || r.Status != "archived" {
		t.Fatalf("resolve = %+v, %v", r, err)
	}
	if _, err := s.Resolve("k1", n.ID); err != nil {
		t.Fatalf("resolve must be idempotent: %v", err)
	}
	if got := list(t, s, f, Filter{RepoKey: "k1"}); len(got) != 0 {
		t.Fatalf("resolved note must hide from default ls: %+v", got)
	}
	got := list(t, s, f, Filter{RepoKey: "k1", IncludeArchived: true})
	if len(got) != 1 || got[0].Status != "archived" {
		t.Fatalf("--all must show archived: %+v", got)
	}
}

func TestArchivedIsTerminal(t *testing.T) {
	s := openStore(t)
	f := files{"a.go": "x\n"}
	n := addNote(t, s, f, "a.go", 1, 1, "resolve me")
	if _, err := s.Resolve("k1", n.ID); err != nil {
		t.Fatal(err)
	}
	f["a.go"] = "changed\n" // anchor would go stale — but archived notes freeze
	got := list(t, s, f, Filter{RepoKey: "k1", IncludeArchived: true})
	if len(got) != 1 || got[0].Status != "archived" {
		t.Fatalf("archived note must never resurrect or restale: %+v", got)
	}
}

func TestDriftRebaselineNoChurn(t *testing.T) {
	s := openStore(t)
	f := files{"a.go": "one\ntwo\nthree\n"}
	addNote(t, s, f, "a.go", 2, 2, "about two")
	f["a.go"] = "zero\none\ntwo\nthree\n" // insert above
	first := list(t, s, f, Filter{RepoKey: "k1"})[0]
	if !first.Drifted || *first.StartLine != 3 {
		t.Fatalf("anchor must drift +1: %+v", first)
	}
	f["a.go"] = "zero\none\n  two\nthree\n" // whitespace-only reindent in place
	second := list(t, s, f, Filter{RepoKey: "k1"})[0]
	if !second.Drifted || *second.StartLine != 3 || second.Status != "active" {
		t.Fatalf("reindent must re-anchor in place: %+v", second)
	}
	third := list(t, s, f, Filter{RepoKey: "k1"})[0]
	if third.UpdatedAt != second.UpdatedAt {
		t.Fatalf("repeat read must not rewrite: %s -> %s", second.UpdatedAt, third.UpdatedAt)
	}
}

func TestStaleRecovery(t *testing.T) {
	s := openStore(t)
	f := files{"a.go": "one\ntwo\n"}
	addNote(t, s, f, "a.go", 2, 2, "about two")
	f["a.go"] = "one\nCHANGED\n"
	if got := list(t, s, f, Filter{RepoKey: "k1"})[0].Status; got != "stale" {
		t.Fatalf("changed line must stale: %q", got)
	}
	f["a.go"] = "one\ntwo\n"
	if got := list(t, s, f, Filter{RepoKey: "k1"})[0].Status; got != "active" {
		t.Fatalf("restored line must reactivate: %q", got)
	}
}

func TestHandoffExpiry(t *testing.T) {
	s := openStore(t)
	f := files{"a.go": "one\n"}
	addNote(t, s, f, "a.go", 1, 1, "left off here #handoff")
	delete(f, "a.go") // anchor lost on read
	if got := list(t, s, f, Filter{RepoKey: "k1"}); len(got) != 0 {
		t.Fatalf("expired handoff must vanish from default ls: %+v", got)
	}
	got := list(t, s, f, Filter{RepoKey: "k1", IncludeArchived: true})
	if len(got) != 1 || got[0].Status != "archived" {
		t.Fatalf("--all must show the expired handoff as archived: %+v", got)
	}
}

func TestMaintainExpiresStaleHandoffsOnWrite(t *testing.T) {
	s := openStore(t)
	f := files{"a.go": "one\n"}
	addNote(t, s, f, "a.go", 1, 1, "half done #handoff")
	f["a.go"] = "other\n"
	list(t, s, f, Filter{RepoKey: "k1"})         // read marks it stale
	addNote(t, s, f, "b.go", 0, 0, "next write") // maintain must archive it
	if got := list(t, s, f, Filter{RepoKey: "k1"}); len(got) != 1 || got[0].File != "b.go" {
		t.Fatalf("stale handoff must be archived by the next write: %+v", got)
	}
}

func TestGCAfterRetention(t *testing.T) {
	s := openStore(t)
	f := files{"a.go": "one\n"}
	n := addNote(t, s, f, "a.go", 1, 1, "old")
	if _, err := s.Resolve("k1", n.ID); err != nil {
		t.Fatal(err)
	}
	old := time.Now().UTC().Add(-Retention - time.Hour).Format(timeLayout)
	if _, err := s.DB.Exec(`UPDATE notes SET updated_at = ? WHERE id = ?`, old, n.ID); err != nil {
		t.Fatal(err)
	}
	addNote(t, s, f, "b.go", 0, 0, "trigger gc") // maintain must drop it for good
	var cnt int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM notes WHERE id = ?`, n.ID).Scan(&cnt); err != nil {
		t.Fatal(err)
	}
	if cnt != 0 {
		t.Fatal("archived note past retention must be garbage-collected")
	}
}

func TestRepoIsolation(t *testing.T) {
	s := openStore(t)
	f := files{"a.go": "one\n"}
	addNote(t, s, f, "a.go", 0, 0, "repo one")
	if got := list(t, s, f, Filter{RepoKey: "k2"}); len(got) != 0 {
		t.Fatalf("another board must see nothing: %+v", got)
	}
	if _, err := s.Resolve("k2", "whatever"); err != ErrNotFound {
		t.Fatal("ids are scoped to the board")
	}
}

func TestDirectoryFilter(t *testing.T) {
	s := openStore(t)
	f := files{"src/a.go": "x\n", "src/deep/b.go": "y\n", "top.go": "z\n"}
	addNote(t, s, f, "src/a.go", 0, 0, "one")
	addNote(t, s, f, "src/deep/b.go", 0, 0, "two")
	addNote(t, s, f, "top.go", 0, 0, "three")
	got := list(t, s, f, Filter{RepoKey: "k1", File: "src/", FileIsDir: true})
	if len(got) != 2 {
		t.Fatalf("directory filter = %+v", got)
	}
}

func TestDumpJSONL(t *testing.T) {
	s := openStore(t)
	f := files{"a.go": "one\n"}
	n := addNote(t, s, f, "a.go", 0, 0, "backed up")
	var buf bytes.Buffer
	if err := s.DumpJSONL(&buf); err != nil {
		t.Fatal(err)
	}
	line := buf.String()
	if !strings.Contains(line, `"id":"`+n.ID+`"`) || !strings.Contains(line, `"repo":"k1"`) {
		t.Fatalf("dump line missing fields: %s", line)
	}
}

func TestQuoteFtsToken(t *testing.T) {
	if got := quoteFtsToken(`weird"token`); got != `"weird""token"` {
		t.Fatalf("quotes must be escaped: %s", got)
	}
	if got := quoteFtsToken("plain"); got != `"plain"` {
		t.Fatalf("plain token = %s", got)
	}
}
