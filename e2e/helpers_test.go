// Package e2e drives the compiled stickit binary end to end: the suite builds
// the CLI once, then every test runs it as a subprocess in hermetic temporary
// directories with its own database and a sanitized environment.
package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

// agentName is the NOTES_AGENT identity injected into every subprocess,
// unless a test deliberately removes it to exercise the fallback.
const agentName = "e2e-agent"

// binPath is the stickit binary built once in TestMain.
var binPath string

func TestMain(m *testing.M) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		fmt.Fprintln(os.Stderr, "e2e: cannot locate the test source file")
		os.Exit(1)
	}
	repoRoot := filepath.Dir(filepath.Dir(thisFile))
	binDir, err := os.MkdirTemp("", "stickit-e2e-bin-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e: %v\n", err)
		os.Exit(1)
	}
	bin := filepath.Join(binDir, "stickit")
	// Inside `go test`, runtime.GOROOT() names the toolchain that built the
	// test binary; it can rebuild the package offline from the warm module
	// cache. The env pins keep the broken goenv shim out of the picture.
	build := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "build", "-o", bin, ".")
	build.Dir = repoRoot
	build.Env = append(os.Environ(), "GOPROXY=https://goproxy.cn,direct", "GOTOOLCHAIN=auto")
	if out, err := build.CombinedOutput(); err != nil {
		os.RemoveAll(binDir)
		fmt.Fprintf(os.Stderr, "e2e: building stickit: %v\n%s", err, out)
		os.Exit(1)
	}
	binPath = bin
	code := m.Run()
	os.RemoveAll(binDir)
	os.Exit(code)
}

// runCmd executes the binary once; err is non-nil only for harness failures
// (spawn error, timeout), never for a nonzero exit of the CLI itself.
func runCmd(dir string, env []string, args ...string) (stdout, stderr string, code int, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binPath, args...)
	cmd.Dir = dir
	cmd.Env = env
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err = cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			return out.String(), errb.String(), -1, err
		}
		err = nil
	}
	return out.String(), errb.String(), cmd.ProcessState.ExitCode(), nil
}

// run runs the binary and fails the test on harness errors.
func run(t *testing.T, dir string, env []string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	stdout, stderr, exitCode, err := runCmd(dir, env, args...)
	if err != nil {
		t.Fatalf("stickit %v: %v\nstderr: %s", args, err, stderr)
	}
	return stdout, stderr, exitCode
}

// mustRun asserts exit code 0 and returns stdout.
func mustRun(t *testing.T, dir string, env []string, args ...string) string {
	t.Helper()
	stdout, stderr, code := run(t, dir, env, args...)
	if code != 0 {
		t.Fatalf("stickit %v: exit %d, want 0\nstderr: %s", args, code, stderr)
	}
	return stdout
}

// baseEnv returns a hermetic subprocess environment: a private database, a
// fixed agent identity, and no inherited git configuration. Tests must never
// depend on the real HOME's gitconfig.
func baseEnv(db string) []string {
	var env []string
	for _, kv := range os.Environ() {
		switch {
		case strings.HasPrefix(kv, "STICKIT_DB="),
			strings.HasPrefix(kv, "NOTES_AGENT="),
			strings.HasPrefix(kv, "GIT_CONFIG_GLOBAL="),
			strings.HasPrefix(kv, "GIT_CONFIG_SYSTEM="),
			strings.HasPrefix(kv, "GIT_CONFIG_NOSYSTEM="):
			// dropped, replaced below
		default:
			env = append(env, kv)
		}
	}
	return append(env,
		"STICKIT_DB="+db,
		"NOTES_AGENT="+agentName,
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_CONFIG_NOSYSTEM=1",
	)
}

// newEnv returns a hermetic environment with its own private database.
func newEnv(t *testing.T) []string {
	t.Helper()
	return baseEnv(filepath.Join(t.TempDir(), "stickit.db"))
}

// withoutNotesAgent removes the agent identity so the author falls back to
// the git user name (and to "unknown" outside git).
func withoutNotesAgent(env []string) []string {
	var out []string
	for _, kv := range env {
		if !strings.HasPrefix(kv, "NOTES_AGENT=") {
			out = append(out, kv)
		}
	}
	return out
}

// git runs one git command in dir under the same hermetic environment.
func git(t *testing.T, dir string, env []string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = env
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		t.Fatalf("git -C %s %v: %v\nstderr: %s", dir, args, err, errb.String())
	}
	return strings.TrimSpace(out.String())
}

// newPlainDir returns a fresh non-git directory: its own board, branch null.
func newPlainDir(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

// newGitRepo returns a fresh git repository on branch main with one commit,
// identity configured repo-locally (user.name Tester) so the author
// fallback is testable without any global gitconfig.
func newGitRepo(t *testing.T, env []string) string {
	t.Helper()
	dir := t.TempDir()
	git(t, dir, env, "init", "-b", "main")
	git(t, dir, env, "config", "user.name", "Tester")
	git(t, dir, env, "config", "user.email", "tester@example.com")
	writeFile(t, filepath.Join(dir, "README.md"), "seed\n")
	git(t, dir, env, "add", ".")
	git(t, dir, env, "commit", "-m", "seed")
	return dir
}

// writeFile writes content to path, creating parent directories.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeLines joins lines with newlines and a trailing newline.
func writeLines(t *testing.T, path string, lines ...string) {
	t.Helper()
	writeFile(t, path, strings.Join(lines, "\n")+"\n")
}

// note mirrors the JSON the CLI serves for one note.
type note struct {
	ID        string   `json:"id"`
	File      string   `json:"file"`
	StartLine *int     `json:"start_line"` // null for file-level notes
	EndLine   *int     `json:"end_line"`
	Status    string   `json:"status"` // active | stale | archived
	Drifted   bool     `json:"drifted"`
	Tags      []string `json:"tags"`
	Author    string   `json:"author"`
	Branch    *string  `json:"branch"` // null outside git
	Body      string   `json:"body"`
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`
	Replies   []reply  `json:"replies"`
}

// reply mirrors one thread entry.
type reply struct {
	Seq       int    `json:"seq"`
	Author    string `json:"author"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
}

// replyOutput mirrors the JSON served by `add --reply-to`.
type replyOutput struct {
	NoteID string `json:"note_id"`
	reply
}

func decodeNote(t *testing.T, stdout, what string) note {
	t.Helper()
	var n note
	if err := json.Unmarshal([]byte(stdout), &n); err != nil {
		t.Fatalf("%s: decode stdout %q: %v", what, stdout, err)
	}
	return n
}

func decodeNotes(t *testing.T, stdout, what string) []note {
	t.Helper()
	var ns []note
	if err := json.Unmarshal([]byte(stdout), &ns); err != nil {
		t.Fatalf("%s: decode stdout %q: %v", what, stdout, err)
	}
	return ns
}

// decodeError asserts stderr is exactly one JSON object {"error": "..."}.
func decodeError(t *testing.T, stderr string) string {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(stderr), &m); err != nil {
		t.Fatalf("stderr is not one JSON object: %q", stderr)
	}
	if len(m) != 1 {
		t.Fatalf("stderr object must carry exactly the error key, got %v", m)
	}
	msg, ok := m["error"].(string)
	if !ok || msg == "" {
		t.Fatalf("stderr error must be a non-empty string, got %v", m)
	}
	return msg
}

// mustAdd pins one note and returns it decoded.
func mustAdd(t *testing.T, dir string, env []string, target, body string) note {
	t.Helper()
	return decodeNote(t, mustRun(t, dir, env, "add", target, body), "add "+target)
}

// mustList runs ls (with optional extra arguments) and decodes the board.
func mustList(t *testing.T, dir string, env []string, args ...string) []note {
	t.Helper()
	return decodeNotes(t, mustRun(t, dir, env, append([]string{"ls"}, args...)...), "ls")
}

// singleNote asserts the board holds exactly one note and returns it.
func singleNote(t *testing.T, notes []note, what string) note {
	t.Helper()
	if len(notes) != 1 {
		t.Fatalf("%s: got %d notes, want 1: %+v", what, len(notes), notes)
	}
	return notes[0]
}

// byID indexes notes by id, failing on duplicates.
func byID(t *testing.T, notes []note) map[string]note {
	t.Helper()
	m := make(map[string]note, len(notes))
	for _, n := range notes {
		if _, dup := m[n.ID]; dup {
			t.Fatalf("duplicate note id %s in %+v", n.ID, notes)
		}
		m[n.ID] = n
	}
	return m
}

func requireStatus(t *testing.T, n note, status, what string) {
	t.Helper()
	if n.Status != status {
		t.Fatalf("%s: status = %q, want %q (note %+v)", what, n.Status, status, n)
	}
}

func requireAnchor(t *testing.T, n note, start, end int, what string) {
	t.Helper()
	if n.StartLine == nil || n.EndLine == nil || *n.StartLine != start || *n.EndLine != end {
		t.Fatalf("%s: anchor = %v-%v, want %d-%d", what, n.StartLine, n.EndLine, start, end)
	}
}

func requireNoAnchor(t *testing.T, n note, what string) {
	t.Helper()
	if n.StartLine != nil || n.EndLine != nil {
		t.Fatalf("%s: file-level note must have null lines, got %v-%v", what, n.StartLine, n.EndLine)
	}
}

// requireBranch asserts the branch provenance; want "" means null.
func requireBranch(t *testing.T, n note, want, what string) {
	t.Helper()
	if want == "" {
		if n.Branch != nil {
			t.Fatalf("%s: branch = %q, want null", what, *n.Branch)
		}
		return
	}
	if n.Branch == nil || *n.Branch != want {
		t.Fatalf("%s: branch = %v, want %q", what, n.Branch, want)
	}
}

// keysOf lists the sorted keys of a decoded JSON object.
func keysOf(m map[string]json.RawMessage) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}
