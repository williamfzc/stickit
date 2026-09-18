package anchor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalize(t *testing.T) {
	cases := []struct{ in, want string }{
		{"  foo   bar ", "foo bar"},
		{"\ta\t\tb", "a b"},
		{"", ""},
	}
	for _, c := range cases {
		if got := Normalize(c.in); got != c.want {
			t.Errorf("Normalize(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestReadLines(t *testing.T) {
	p := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(p, []byte("a\r\nb\r\nc"), 0o644); err != nil {
		t.Fatal(err)
	}
	lines, err := ReadLines(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 3 || lines[0] != "a" || lines[2] != "c" {
		t.Fatalf("CRLF not normalized: %q", lines)
	}
}

func TestLinesBounds(t *testing.T) {
	all := []string{"a", "b", "c"}
	if _, err := Lines(all, 0, 1); err == nil {
		t.Error("start 0 must be rejected")
	}
	if _, err := Lines(all, 2, 1); err == nil {
		t.Error("end < start must be rejected")
	}
	if _, err := Lines(all, 1, 4); err == nil {
		t.Error("end past EOF must be rejected")
	}
	blk, err := Lines(all, 2, 3)
	if err != nil || len(blk) != 2 {
		t.Fatalf("Lines(2,3) = %v, %v", blk, err)
	}
}

func validateAt(t *testing.T, lines []string, start, end int) Result {
	t.Helper()
	h, nh := Hash(lines[start-1:end]), NormHash(lines[start-1:end])
	return Validate(lines, true, start, end, h, nh)
}

func TestValidateFresh(t *testing.T) {
	lines := []string{"a", "b", "c"}
	if r := validateAt(t, lines, 2, 2); r.State != Fresh || r.Start != 2 {
		t.Fatalf("unchanged content must be fresh: %+v", r)
	}
}

func TestValidateDriftDown(t *testing.T) {
	// anchor "b" at line 2, then a line is inserted above
	orig := []string{"a", "b", "c"}
	h, nh := Hash(orig[1:2]), NormHash(orig[1:2])
	moved := []string{"x", "a", "b", "c"}
	r := Validate(moved, true, 2, 2, h, nh)
	if r.State != Drifted || r.Start != 3 || r.End != 3 {
		t.Fatalf("insert above must drift +1: %+v", r)
	}
}

func TestValidateDriftUpAndWhitespace(t *testing.T) {
	// anchor "b c" at line 3; the line above is deleted and the anchor is reindented
	orig := []string{"a", "x", "  b c"}
	h, nh := Hash(orig[2:3]), NormHash(orig[2:3])
	moved := []string{"a", "\tb c"}
	r := Validate(moved, true, 3, 3, h, nh)
	if r.State != Drifted || r.Start != 2 {
		t.Fatalf("delete above + reindent must drift -1: %+v", r)
	}
}

func TestValidateStale(t *testing.T) {
	orig := []string{"a", "b"}
	h, nh := Hash(orig[1:2]), NormHash(orig[1:2])
	if r := Validate([]string{"a", "z"}, true, 2, 2, h, nh); r.State != Stale {
		t.Fatalf("changed content must be stale: %+v", r)
	}
	if r := Validate(nil, false, 2, 2, h, nh); r.State != Stale {
		t.Fatalf("missing file must be stale: %+v", r)
	}
}

func TestValidateBeyondReach(t *testing.T) {
	// one line anchored; the content moves Reach+1 lines down: stale
	orig := []string{"target"}
	h, nh := Hash(orig), NormHash(orig)
	moved := make([]string, Reach+2)
	for i := range moved {
		moved[i] = "filler"
	}
	moved[Reach+1] = "target"
	if r := Validate(moved, true, 1, 1, h, nh); r.State != Stale {
		t.Fatalf("content past reach must be stale: %+v", r)
	}
	moved[Reach] = "target" // nearest-first scan must find it just inside
	if r := Validate(moved, true, 1, 1, h, nh); r.State != Drifted || r.Start != Reach+1 {
		t.Fatalf("content just inside reach must drift: %+v", r)
	}
}

func TestValidateFileLevel(t *testing.T) {
	if r := Validate([]string{"a"}, true, 0, 0, "", ""); r.State != Fresh {
		t.Fatalf("existing file must keep a file-level note fresh: %+v", r)
	}
	if r := Validate(nil, false, 0, 0, "", ""); r.State != Stale {
		t.Fatalf("deleted file must stale a file-level note: %+v", r)
	}
}
