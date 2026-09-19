package board

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/williamfzc/stickit/internal/anchor"
)

// Target is a resolved pin location for add.
type Target struct {
	// File is the board-relative, slash-separated path of the pinned file.
	File string
	// AbsPath is the local file the note is anchored against.
	AbsPath string
	// Start and End are 1-based inclusive anchored lines; both 0 mean a
	// file-level note.
	Start, End int
}

var lineSpec = regexp.MustCompile(`^([0-9]+)(?:-([0-9]+))?$`)

// ParseTarget resolves an add target like "file", "file:40" or "file:40-50"
// against the board.
func ParseTarget(b Board, spec string) (Target, error) {
	file, ls := splitTarget(spec)
	if file == "" {
		return Target{}, fmt.Errorf("invalid target %q: missing file", spec)
	}
	t := Target{}
	if ls != "" {
		m := lineSpec.FindStringSubmatch(ls)
		if m == nil {
			return Target{}, fmt.Errorf("invalid target %q: lines must look like 40 or 40-50", spec)
		}
		t.Start, _ = strconv.Atoi(m[1])
		t.End = t.Start
		if m[2] != "" {
			t.End, _ = strconv.Atoi(m[2])
		}
		if t.Start == 0 || t.End < t.Start {
			return Target{}, fmt.Errorf("invalid target %q: line range must satisfy 1 <= start <= end", spec)
		}
	}
	abs, err := AbsUserPath(file)
	if err != nil {
		return Target{}, err
	}
	fi, err := os.Stat(abs)
	if err != nil {
		return Target{}, fmt.Errorf("no such file: %s", file)
	}
	if fi.IsDir() {
		return Target{}, fmt.Errorf("not a file: %s", file)
	}
	rel, err := RelFromBoard(b, abs)
	if err != nil {
		return Target{}, err
	}
	t.File = rel
	t.AbsPath = abs
	if t.Start > 0 {
		lines, err := anchor.ReadLines(abs)
		if err != nil {
			return Target{}, err
		}
		if t.End > len(lines) {
			return Target{}, fmt.Errorf("invalid target %q: line range must satisfy 1 <= start <= end <= %d (file has %d lines)", spec, len(lines), len(lines))
		}
	}
	return t, nil
}

// splitTarget separates "file:lines" at the last colon. A trailing segment
// that is not a line spec belongs to the filename when the whole spec names
// a real file (names may contain colons); otherwise the split stands, so a
// malformed line spec still teaches the syntax in the error.
func splitTarget(spec string) (file, ls string) {
	i := strings.LastIndex(spec, ":")
	if i < 0 {
		return spec, ""
	}
	suffix := spec[i+1:]
	if lineSpec.MatchString(suffix) {
		return spec[:i], suffix
	}
	if _, err := os.Stat(spec); err == nil {
		return spec, ""
	}
	return spec[:i], suffix
}

// RelFromBoard normalizes a user-supplied path (cwd-relative or absolute,
// already resolved to abs) to a worktree-relative, slash-separated path —
// the shared file key all worktrees of one board address — rejecting paths
// that escape the current working tree. The base is TopLevel, not Root: a
// linked worktree's files live under TopLevel, while Root is the main
// repository the whole board is keyed by.
func RelFromBoard(b Board, abs string) (string, error) {
	rel, err := filepath.Rel(b.TopLevel, abs)
	if err != nil {
		return "", fmt.Errorf("%s is not inside the working tree at %s", abs, b.TopLevel)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%s is outside the working tree at %s", abs, b.TopLevel)
	}
	return filepath.ToSlash(rel), nil
}
