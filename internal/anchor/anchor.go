// Package anchor pins notes to lines: hashing, lazy validation and the
// whitespace-insensitive fuzzy re-anchor that lets notes drift gracefully
// instead of lying.
package anchor

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

// Reach is how far (in lines, each direction) a lost anchor searches for its
// original content before it is declared stale.
const Reach = 50

// State is the outcome of validating an anchor against the current file.
type State int

const (
	Fresh State = iota
	Drifted
	Stale
)

// Result reports the validated state and the (possibly moved) anchor.
type Result struct {
	State State
	Start int
	End   int
}

// Hash hashes the exact lines (as split by ReadLines) joined with newlines.
func Hash(lines []string) string {
	return hashOf(strings.Join(lines, "\n"))
}

// NormHash hashes the lines with whitespace collapsed, so re-anchoring can
// still recognize content that merely moved or was reindented.
func NormHash(lines []string) string {
	norm := make([]string, len(lines))
	for i, l := range lines {
		norm[i] = Normalize(l)
	}
	return hashOf(strings.Join(norm, "\n"))
}

// Normalize collapses a line's whitespace, for whitespace-insensitive
// comparison.
func Normalize(line string) string {
	return strings.Join(strings.Fields(line), " ")
}

// ReadLines reads a file and splits it into lines, without a trailing empty
// line. CRLF is normalized so line content hashing is platform-stable.
func ReadLines(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	s := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(s, "\n")
	if n := len(lines); n > 0 && lines[n-1] == "" {
		lines = lines[:n-1]
	}
	return lines, nil
}

// Lines returns the 1-based inclusive range [start,end] of lines.
func Lines(all []string, start, end int) ([]string, error) {
	if start < 1 || end < start || end > len(all) {
		return nil, fmt.Errorf("line range %d-%d outside file (%d lines)", start, end, len(all))
	}
	return all[start-1 : end], nil
}

// Validate checks an anchored range against the current file content.
// start == 0 is a file-level note: existing is the only freshness it needs.
// lines is nil when the file is missing.
func Validate(lines []string, fileExists bool, start, end int, hash, normHash string) Result {
	if start == 0 || !fileExists {
		if fileExists {
			return Result{State: Fresh, Start: 0, End: 0}
		}
		return Result{State: Stale}
	}
	if start >= 1 && end <= len(lines) && Hash(lines[start-1:end]) == hash {
		return Result{State: Fresh, Start: start, End: end}
	}
	if normHash != "" {
		span := end - start
		for off := 0; off <= Reach; off++ {
			for _, s := range offsets(start, off) {
				e := s + span
				if s < 1 || e > len(lines) {
					continue
				}
				if NormHash(lines[s-1:e]) == normHash {
					return Result{State: Drifted, Start: s, End: e}
				}
			}
		}
	}
	return Result{State: Stale}
}

// offsets yields candidate start lines nearest-first: the original position,
// then one line up and down, and so on out to Reach.
func offsets(start, off int) []int {
	if off == 0 {
		return []int{start}
	}
	return []int{start - off, start + off}
}

func hashOf(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
