package e2e

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The command surface: help, skill, exit codes and error shapes.

func TestHelpPrintsUsage(t *testing.T) {
	env := newEnv(t)
	dir := newPlainDir(t)
	for _, arg := range []string{"-h", "--help", "help"} {
		stdout, _, code := run(t, dir, env, arg)
		if code != 0 {
			t.Fatalf("stickit %s: exit %d, want 0", arg, code)
		}
		if !strings.Contains(stdout, "Usage:") {
			t.Fatalf("stickit %s: usage missing from stdout: %q", arg, stdout)
		}
	}
}

func TestSkillPrintsSnippet(t *testing.T) {
	env := newEnv(t)
	dir := newPlainDir(t)
	stdout, _, code := run(t, dir, env, "--skill")
	if code != 0 {
		t.Fatalf("--skill: exit %d, want 0", code)
	}
	if !strings.Contains(stdout, "stickit ls") {
		t.Fatalf("--skill: no contract lines in stdout: %q", stdout)
	}
}

// --help must route agents to the skill: the three-line contract is the
// whole onboarding, so the flag has to be visible from the options block.
func TestHelpRoutesAgentsToSkill(t *testing.T) {
	env := newEnv(t)
	dir := newPlainDir(t)
	stdout, _, code := run(t, dir, env, "--help")
	if code != 0 {
		t.Fatalf("--help: exit %d, want 0", code)
	}
	if !strings.Contains(stdout, "stickit --skill") {
		t.Fatalf("--help does not mention --skill: %q", stdout)
	}
}

// No arguments and unknown commands are usage errors. stderr is piped in
// these tests, so even pre-parse failures must be one JSON error object —
// machine-first, same shape as every other failure.
func TestNoArgsIsUsageError(t *testing.T) {
	env := newEnv(t)
	dir := newPlainDir(t)
	stdout, stderr, code := run(t, dir, env)
	if code != 1 {
		t.Fatalf("stickit: exit %d, want 1", code)
	}
	if stdout != "" {
		t.Fatalf("stickit: stdout = %q, want empty", stdout)
	}
	if msg := decodeError(t, stderr); !strings.Contains(msg, "no command") {
		t.Fatalf("stickit: error = %q, want a no-command message", msg)
	}
}

func TestUnknownCommandIsUsageError(t *testing.T) {
	env := newEnv(t)
	dir := newPlainDir(t)
	stdout, stderr, code := run(t, dir, env, "frobnicate")
	if code != 1 {
		t.Fatalf("stickit frobnicate: exit %d, want 1", code)
	}
	if stdout != "" {
		t.Fatalf("stickit frobnicate: stdout = %q, want empty", stdout)
	}
	if msg := decodeError(t, stderr); !strings.Contains(msg, "unknown command") {
		t.Fatalf("stickit frobnicate: error = %q, want an unknown-command message", msg)
	}
}

// Usage errors detected after flag parsing render exactly one JSON error
// object on stderr.
func TestUsageErrorsAreJSON(t *testing.T) {
	env := newEnv(t)
	dir := newPlainDir(t)
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"add without arguments", []string{"add"}, "usage: stickit add"},
		{"add with only a target", []string{"add", "file.go"}, "usage: stickit add"},
		{"add with blank body", []string{"add", "file.go:2", "  "}, "body must not be empty"},
		{"add with unknown flag", []string{"add", "--reply-to", "abcdefgh"}, "unknown flag"},
		{"ls with unknown flag", []string{"ls", "--frobnicate"}, "unknown flag"},
		{"ls with three positionals", []string{"ls", "file.go", "one", "two"}, "usage: stickit ls"},
		{"ls with keyword then positional", []string{"ls", "nosuchword", "more"}, "does not name a path"},
		{"resolve without id", []string{"resolve"}, "usage: stickit resolve"},
		{"resolve with two ids", []string{"resolve", "one", "two"}, "usage: stickit resolve"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, code := run(t, dir, env, tc.args...)
			if code != 1 {
				t.Fatalf("stickit %v: exit %d, want 1 (stderr %q)", tc.args, code, stderr)
			}
			if stdout != "" {
				t.Fatalf("stickit %v: stdout = %q, want empty", tc.args, stdout)
			}
			if msg := decodeError(t, stderr); !strings.Contains(msg, tc.want) {
				t.Fatalf("stickit %v: error %q, want it to contain %q", tc.args, msg, tc.want)
			}
		})
	}
}

// A bad pin target is a usage error: missing file, malformed or out-of-range
// lines, or a directory.
func TestBadTargetsAreUsageErrors(t *testing.T) {
	env := newEnv(t)
	dir := t.TempDir()
	writeLines(t, filepath.Join(dir, "file.go"), "one", "two", "three", "four", "five")
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		target string
		want   string
	}{
		{"missing file", "missing.txt:3", "no such file: missing.txt"},
		{"malformed line spec", "file.go:abc", "lines must look like"},
		{"line zero", "file.go:0", "1 <= start <= end"},
		{"inverted range", "file.go:3-2", "1 <= start <= end"},
		{"range past EOF", "file.go:99", "must satisfy 1 <= start <= end <= 5"},
		{"directory as target", "sub:1", "not a file: sub"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, code := run(t, dir, env, "add", tc.target, "body")
			if code != 1 {
				t.Fatalf("add %s: exit %d, want 1 (stderr %q)", tc.target, code, stderr)
			}
			if stdout != "" {
				t.Fatalf("add %s: stdout = %q, want empty", tc.target, stdout)
			}
			if msg := decodeError(t, stderr); !strings.Contains(msg, tc.want) {
				t.Fatalf("add %s: error %q, want it to contain %q", tc.target, msg, tc.want)
			}
		})
	}
}

// An unknown note id in this board is exit 2.
func TestNoteNotFoundIsExitTwo(t *testing.T) {
	env := newEnv(t)
	dir := t.TempDir()
	writeLines(t, filepath.Join(dir, "file.go"), "one", "two")

	stdout, stderr, code := run(t, dir, env, "resolve", "nowhere00")
	if code != 2 {
		t.Fatalf("resolve nowhere00: exit %d, want 2", code)
	}
	if stdout != "" {
		t.Fatalf("resolve nowhere00: stdout = %q, want empty", stdout)
	}
	if msg := decodeError(t, stderr); !strings.Contains(msg, "no note nowhere00") {
		t.Fatalf("resolve nowhere00: error = %q", msg)
	}
}

// An unusable database location is a store error: exit 3, JSON on stderr.
func TestStoreFailureIsExitThree(t *testing.T) {
	dir := newPlainDir(t)
	blocker := filepath.Join(dir, "blocker")
	writeFile(t, blocker, "not a directory\n")
	broken := baseEnv(filepath.Join(blocker, "stickit.db"))

	stdout, stderr, code := run(t, dir, broken, "ls")
	if code != 3 {
		t.Fatalf("ls with broken db: exit %d, want 3 (stderr %q)", code, stderr)
	}
	if stdout != "" {
		t.Fatalf("ls with broken db: stdout = %q, want empty", stdout)
	}
	decodeError(t, stderr)
}

// A whitespace-only keyword is not a filter: the (empty) board comes back.
func TestWhitespaceKeywordOnEmptyBoard(t *testing.T) {
	env := newEnv(t)
	dir := newPlainDir(t)
	stdout, _, code := run(t, dir, env, "ls", "  ")
	if code != 0 {
		t.Fatalf("ls %q: exit %d, want 0", "  ", code)
	}
	if got := strings.TrimSpace(stdout); got != "[]" {
		t.Fatalf("ls %q: stdout = %q, want []", "  ", got)
	}
}

// dump is the backup surface: one JSON object per line, every board in the
// database, each carrying the repo key and the note id.
func TestDumpIsJSONL(t *testing.T) {
	env := baseEnv(filepath.Join(t.TempDir(), "stickit.db"))
	repoA := newGitRepo(t, env)
	repoB := newGitRepo(t, env)
	writeLines(t, filepath.Join(repoA, "a.txt"), "one", "two")
	writeLines(t, filepath.Join(repoB, "b.txt"), "one", "two")
	mustAdd(t, repoA, env, "a.txt:1", "note pinned in repo A")
	mustAdd(t, repoB, env, "b.txt:1", "note pinned in repo B")

	stdout := mustRun(t, repoA, env, "dump")
	lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("dump produced %d lines, want 2: %q", len(lines), stdout)
	}
	repos := map[string]bool{}
	for i, ln := range lines {
		var obj map[string]any
		if err := json.Unmarshal([]byte(ln), &obj); err != nil {
			t.Fatalf("dump line %d is not one JSON object: %q", i+1, ln)
		}
		if id, _ := obj["id"].(string); id == "" {
			t.Fatalf("dump line %d has no id: %q", i+1, ln)
		}
		repo, _ := obj["repo"].(string)
		if repo == "" {
			t.Fatalf("dump line %d has no repo key: %q", i+1, ln)
		}
		repos[repo] = true
	}
	if len(repos) != 2 {
		t.Fatalf("dump must carry both boards' keys, got %v", repos)
	}
}

// On a TTY, ls renders a table instead of JSON. script(1) allocates that
// TTY; BSD and util-linux speak different syntaxes, so the invocation is
// per-platform.
func TestTTYPrintsTable(t *testing.T) {
	script, err := exec.LookPath("script")
	if err != nil {
		t.Skip("script(1) is not available")
	}
	env := newEnv(t)
	dir := newPlainDir(t)
	writeLines(t, filepath.Join(dir, "file.go"), "one", "two")
	n := mustAdd(t, dir, env, "file.go:1", "table row #gotcha")

	var cmd *exec.Cmd
	if runtime.GOOS == "darwin" {
		cmd = exec.Command(script, "-q", "/dev/null", binPath, "ls")
	} else {
		cmd = exec.Command(script, "-qec", binPath+" ls", "/dev/null")
	}
	cmd.Dir = dir
	cmd.Env = env
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("script -q /dev/null %s ls: %v", binPath, err)
	}
	got := strings.ReplaceAll(string(out), "\r", "")
	if !strings.Contains(got, "STATUS") {
		t.Fatalf("table header missing from TTY output: %q", got)
	}
	if !strings.Contains(got, n.ID) {
		t.Fatalf("note id %s missing from TTY output: %q", n.ID, got)
	}
	var notes []note
	if err := json.Unmarshal([]byte(got), &notes); err == nil {
		t.Fatalf("TTY output must be a table, but it decoded as JSON: %q", got)
	}
}
