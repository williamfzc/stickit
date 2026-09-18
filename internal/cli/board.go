package cli

import (
	"encoding/json"
	"io"
	"os"
	"strings"

	"github.com/williamfzc/stickit/internal/anchor"
	"github.com/williamfzc/stickit/internal/board"
	"github.com/williamfzc/stickit/internal/store"
)

func detectBoard() (board.Board, error) { return board.Detect() }

func boardAuthor(b board.Board) string { return board.Author(b) }

func parseTarget(b board.Board, spec string) (board.Target, error) {
	return board.ParseTarget(b, spec)
}

func readLines(path string) ([]string, error) { return anchor.ReadLines(path) }

func anchorLines(lines []string, start, end int) ([]string, error) {
	return anchor.Lines(lines, start, end)
}

func anchorHash(lines []string) string     { return anchor.Hash(lines) }
func anchorNormHash(lines []string) string { return anchor.NormHash(lines) }

func boardRel(b board.Board, p string) (string, error) {
	abs, err := board.AbsUserPath(p)
	if err != nil {
		return "", err
	}
	return board.RelFromBoard(b, abs)
}

// isPathArg reports whether a positional looks like a path filter: it names
// an existing file or directory, or carries path syntax.
func isPathArg(p string) bool {
	if strings.ContainsRune(p, '/') || strings.HasSuffix(p, "/") || p == "." || p == ".." {
		return true
	}
	if _, err := os.Stat(p); err == nil {
		return true
	}
	return false
}

// pathIsDir reports whether the argument addresses a directory, either
// because it exists as one or because the caller wrote it with a trailing
// separator.
func pathIsDir(b board.Board, p string) bool {
	if strings.HasSuffix(p, "/") {
		return true
	}
	abs, err := absUserPath(p)
	if err != nil {
		return false
	}
	fi, err := os.Stat(abs)
	return err == nil && fi.IsDir()
}

// boardContent adapts the board to the store's lazy validation: files are
// read from the invoking worktree, and a vanished file simply does not
// exist.
func boardContent(b board.Board) store.Content {
	return func(rel string) ([]string, bool, error) {
		lines, err := anchor.ReadLines(b.LocalPath(rel))
		if err != nil {
			if os.IsNotExist(err) {
				return nil, false, nil
			}
			return nil, false, err
		}
		return lines, true, nil
	}
}

func absUserPath(p string) (string, error) { return board.AbsUserPath(p) }

// withStore opens the global database for one operation and always closes
// it, mapping open failures to the store exit code.
func withStore(stderr io.Writer, o *out, fn func(*store.Store) int) int {
	path, err := store.DefaultPath()
	if err != nil {
		return fail(stderr, o, ExitStore, err)
	}
	s, err := store.Open(path)
	if err != nil {
		return fail(stderr, o, ExitStore, err)
	}
	defer s.Close()
	return fn(s)
}

func jsonEncoder(w io.Writer) *json.Encoder {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc
}
