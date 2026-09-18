package e2e

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"
)

// Two distinct repos sharing one database keep separate boards: notes never
// leak across repos even when the file paths coincide.
func TestReposAreIsolatedInSharedDatabase(t *testing.T) {
	env := baseEnv(filepath.Join(t.TempDir(), "stickit.db")) // one database, two repos
	repoA := newGitRepo(t, env)
	repoB := newGitRepo(t, env)
	writeLines(t, filepath.Join(repoA, "file.go"), "one", "two")
	writeLines(t, filepath.Join(repoB, "file.go"), "one", "two")
	a := mustAdd(t, repoA, env, "file.go:1", "pinned in repo A")
	b := mustAdd(t, repoB, env, "file.go:1", "pinned in repo B")

	gotA := singleNote(t, mustList(t, repoA, env), "ls in repo A")
	if gotA.ID != a.ID || gotA.Body != a.Body {
		t.Fatalf("ls in repo A = %+v, want only repo A's note %s", gotA, a.ID)
	}
	gotB := singleNote(t, mustList(t, repoB, env), "ls in repo B")
	if gotB.ID != b.ID || gotB.Body != b.Body {
		t.Fatalf("ls in repo B = %+v, want only repo B's note %s", gotB, b.ID)
	}
}

// Outside git, a plain directory is its own board with null branch.
func TestPlainDirectoryIsItsOwnBoard(t *testing.T) {
	env := newEnv(t)
	dir := newPlainDir(t)
	writeLines(t, filepath.Join(dir, "todo.txt"), "buy milk")

	n := mustAdd(t, dir, env, "todo.txt", "file-level note outside git")
	requireBranch(t, n, "", "plain dir")
	got := singleNote(t, mustList(t, dir, env), "ls in plain dir")
	if got.ID != n.ID {
		t.Fatalf("ls in plain dir = %+v, want %s", got, n.ID)
	}
}

// The board belongs to the repo, not to a worktree: a note pinned in a
// linked worktree is visible from the main worktree with its branch
// provenance intact.
func TestWorktreesShareOneBoard(t *testing.T) {
	env := newEnv(t)
	repo := newGitRepo(t, env)
	writeLines(t, filepath.Join(repo, "file.go"), "alpha", "bravo", "charlie", "delta", "echo")
	git(t, repo, env, "add", "file.go")
	git(t, repo, env, "commit", "-m", "file")

	wt := filepath.Join(t.TempDir(), "wt")
	git(t, repo, env, "worktree", "add", "-b", "feature-x", wt)

	fromWT := mustAdd(t, wt, env, "file.go:2", "pinned from the worktree")
	requireBranch(t, fromWT, "feature-x", "add in worktree")

	onMain := mustAdd(t, repo, env, "file.go:4", "pinned on main")
	requireBranch(t, onMain, "main", "add on main")

	all := byID(t, mustList(t, repo, env, "file.go"))
	if len(all) != 2 {
		t.Fatalf("ls file.go from the main worktree = %v, want both notes", all)
	}
	requireBranch(t, all[fromWT.ID], "feature-x", "worktree note read from main")
	requireBranch(t, all[onMain.ID], "main", "main note read from main")
}

// Commit provenance is a write-time snapshot: each note keeps the HEAD it
// was pinned at, even after the branch moves on; a repository before its
// first commit records none.
func TestCommitProvenanceIsWriteTime(t *testing.T) {
	env := newEnv(t)
	repo := newGitRepo(t, env)
	writeLines(t, filepath.Join(repo, "file.go"), "one", "two", "three", "four", "five")

	first := mustAdd(t, repo, env, "file.go:1", "pinned at the seed commit")
	firstCommit := git(t, repo, env, "rev-parse", "HEAD")
	requireCommit(t, first, firstCommit, "first add")

	writeLines(t, filepath.Join(repo, "file.go"), "one", "two", "three", "four", "five", "six")
	git(t, repo, env, "add", "file.go")
	git(t, repo, env, "commit", "-m", "grow the file")
	secondCommit := git(t, repo, env, "rev-parse", "HEAD")
	if secondCommit == firstCommit {
		t.Fatal("test bug: HEAD did not move")
	}

	second := mustAdd(t, repo, env, "file.go:6", "pinned at the second commit")
	requireCommit(t, second, secondCommit, "second add")

	all := byID(t, mustList(t, repo, env, "file.go"))
	requireCommit(t, all[first.ID], firstCommit, "old note after HEAD moved")
	requireCommit(t, all[second.ID], secondCommit, "new note")

	// An unborn repository has no revision to record.
	unborn := t.TempDir()
	git(t, unborn, env, "init", "-b", "main")
	writeLines(t, filepath.Join(unborn, "file.go"), "one")
	u := mustAdd(t, unborn, env, "file.go:1", "no commits yet")
	requireBranch(t, u, "main", "unborn repo")
	requireCommit(t, u, "", "unborn repo")
}

// Author identity: NOTES_AGENT wins; without it the repo's git user.name
// applies; outside git, "unknown".
func TestAuthorResolution(t *testing.T) {
	env := newEnv(t)
	repo := newGitRepo(t, env) // repo-local user.name = Tester
	writeLines(t, filepath.Join(repo, "file.go"), "one", "two")

	agent := mustAdd(t, repo, env, "file.go:1", "agent note")
	if agent.Author != agentName {
		t.Fatalf("author = %q, want NOTES_AGENT %q", agent.Author, agentName)
	}

	humanEnv := withoutNotesAgent(env)
	human := mustAdd(t, repo, humanEnv, "file.go:1", "human note")
	if human.Author != "Tester" {
		t.Fatalf("author = %q, want the repo-local git user.name Tester", human.Author)
	}

	plain := newPlainDir(t)
	writeLines(t, filepath.Join(plain, "file.go"), "one")
	anon := mustAdd(t, plain, humanEnv, "file.go:1", "anonymous note")
	if anon.Author != "unknown" {
		t.Fatalf("author outside git without identity = %q, want unknown", anon.Author)
	}
	requireBranch(t, anon, "", "plain dir")
}

// Concurrent writers on one database all land, from a cold start: WAL plus
// busy_timeout makes SQLite the concurrency authority, and store.Open
// retries the schema/journal initialization race so the very first burst
// survives too.
func TestConcurrentAddsAllLand(t *testing.T) {
	env := newEnv(t)
	repo := newGitRepo(t, env)
	writeLines(t, filepath.Join(repo, "file.go"), "one", "two", "three")

	const writers = 8
	codes := make([]int, writers)
	var wg sync.WaitGroup
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _, codes[i], _ = runCmd(repo, env, "add", "file.go:2", fmt.Sprintf("concurrent note %d", i))
		}(i)
	}
	wg.Wait()
	for i, code := range codes {
		if code != 0 {
			t.Fatalf("concurrent add %d: exit %d, want 0", i, code)
		}
	}

	notes := mustList(t, repo, env, "file.go")
	if len(notes) != writers {
		t.Fatalf("board holds %d notes, want %d", len(notes), writers)
	}
	// byID fails the test on duplicate ids.
	byID(t, notes)
}
