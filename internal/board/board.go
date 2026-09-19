// Package board resolves the scope every stickit command operates on: the
// repository root (or plain directory) that owns the notes, plus the
// provenance — branch and author — recorded on writes.
//
// The board's database lives with the workspace — inside the git dir (so
// all worktrees of one repo share a single board and it moves and dies
// with the project); outside git, in a .stickit directory of the folder.
package board

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Board is the resolved scope of the current invocation.
type Board struct {
	// Root is the workspace the board belongs to: the repository root
	// (via git-common-dir, so all worktrees share one board), or the
	// directory itself outside git.
	Root string
	// TopLevel is the working tree the current invocation reads files from:
	// the worktree root inside git, Root otherwise.
	TopLevel string
	// InGit reports whether the board is a git repository.
	InGit bool
	// Branch is the checked-out branch ("" outside git, "HEAD" when
	// detached).
	Branch string
	// Commit is the HEAD revision at detection time ("" outside git and
	// before the first commit) — the finer half of write-time provenance.
	Commit string
	// DBPath is where this board's database lives: inside the git dir for
	// repositories (so all worktrees share one board and `git clean`
	// cannot touch it), in a .stickit directory otherwise. The board is
	// born, moves and dies with its project.
	DBPath string
}

// Detect resolves the board for the current working directory.
func Detect() (Board, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return Board{}, fmt.Errorf("resolve working directory: %w", err)
	}
	cwd = resolve(filepath.Clean(cwd))
	if gitDir, err := git(cwd, "rev-parse", "--git-common-dir"); err == nil && gitDir != "" {
		if abs, err := absFrom(cwd, gitDir); err == nil {
			// A regular repo keeps its git dir in a ".git" subdirectory of
			// the root; a bare repo's git dir is the root itself.
			root := abs
			if filepath.Base(abs) == ".git" {
				root = filepath.Dir(abs)
			}
			root = resolve(filepath.Clean(root))
			top := root
			if t, err := git(cwd, "rev-parse", "--show-toplevel"); err == nil && t != "" {
				if abs, err := absFrom(cwd, t); err == nil {
					top = resolve(abs)
				}
			}
			// --show-current works on an unborn branch (no commits yet),
			// where --abbrev-ref HEAD would fail; it is empty when HEAD
			// is detached, so fall back to the ref name in that case.
			branch, _ := git(cwd, "branch", "--show-current")
			if branch == "" {
				if abbrev, err := git(cwd, "rev-parse", "--abbrev-ref", "HEAD"); err == nil {
					branch = abbrev
				}
			}
			// rev-parse HEAD fails before the first commit; the commit half
			// of provenance is then simply absent, like it is outside git.
			commit, _ := git(cwd, "rev-parse", "HEAD")
			return Board{
				Root:     root,
				TopLevel: top,
				InGit:    true,
				Branch:   branch,
				Commit:   commit,
				DBPath:   filepath.Join(abs, "stickit", "board.db"),
			}, nil
		}
	}
	return Board{
		Root:     cwd,
		TopLevel: cwd,
		DBPath:   filepath.Join(cwd, ".stickit", "board.db"),
	}, nil
}

// Author returns the note author: NOTES_AGENT for agents, the git user name
// as a fallback for humans, "unknown" when neither is set.
func Author(b Board) string {
	if a := strings.TrimSpace(os.Getenv("NOTES_AGENT")); a != "" {
		return a
	}
	if b.InGit {
		if name, err := git(b.Root, "config", "user.name"); err == nil && name != "" {
			return name
		}
	}
	return "unknown"
}

// LocalPath maps a board-relative file to the copy the current invocation
// reads: notes are validated against the reader's own worktree.
func (b Board) LocalPath(rel string) string {
	return filepath.Join(b.TopLevel, filepath.FromSlash(rel))
}

// git runs one git command in dir and returns its trimmed stdout.
func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	var out strings.Builder
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return strings.TrimSpace(out.String()), nil
}

// AbsUserPath resolves a user-supplied path (cwd-relative or absolute)
// against the working directory, canonicalized through symlinks so it
// agrees with the real paths git reports (macOS /tmp → /private/tmp).
func AbsUserPath(p string) (string, error) {
	var abs string
	if filepath.IsAbs(p) {
		abs = filepath.Clean(p)
	} else {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		abs = filepath.Clean(filepath.Join(cwd, p))
	}
	return resolve(abs), nil
}

// resolve canonicalizes p through symlinks, resolving the deepest existing
// ancestor when p itself does not exist yet.
func resolve(p string) string {
	if rp, err := filepath.EvalSymlinks(p); err == nil {
		return rp
	}
	parent := filepath.Dir(p)
	if parent == p {
		return p
	}
	return filepath.Join(resolve(parent), filepath.Base(p))
}

// absFrom resolves p, which git printed relative to base.
func absFrom(base, p string) (string, error) {
	if filepath.IsAbs(p) {
		return filepath.Clean(p), nil
	}
	return filepath.Abs(filepath.Join(base, p))
}
