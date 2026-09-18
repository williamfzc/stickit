package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestParseFlags(t *testing.T) {
	cases := []struct {
		name      string
		args      []string
		allowed   map[string]bool
		pos       []string
		flags     map[string]string
		wantError string
	}{
		{
			name:    "value flag anywhere",
			args:    []string{"file.go:3", "--reply-to", "ab12", "body"},
			allowed: map[string]bool{"reply-to": true},
			pos:     []string{"file.go:3", "body"},
			flags:   map[string]string{"reply-to": "ab12"},
		},
		{
			name:    "equals form",
			args:    []string{"--reply-to=xyz", "body"},
			allowed: map[string]bool{"reply-to": true},
			pos:     []string{"body"},
			flags:   map[string]string{"reply-to": "xyz"},
		},
		{
			name:      "unknown flag rejected",
			args:      []string{"--al"},
			allowed:   map[string]bool{"all": false},
			wantError: "unknown flag --al",
		},
		{
			name:      "value flag must not swallow another flag",
			args:      []string{"--reply-to", "--all", "body"},
			allowed:   map[string]bool{"reply-to": true},
			wantError: "flag --reply-to needs a value",
		},
		{
			name:      "missing value",
			args:      []string{"--reply-to"},
			allowed:   map[string]bool{"reply-to": true},
			wantError: "flag --reply-to needs a value",
		},
		{
			name:      "bool flag takes no value",
			args:      []string{"--all=true"},
			allowed:   map[string]bool{"all": false},
			wantError: "flag --all takes no value",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pos, flags, err := parseFlags(c.args, c.allowed)
			if c.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), c.wantError) {
					t.Fatalf("err = %v, want %q", err, c.wantError)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if strings.Join(pos, "|") != strings.Join(c.pos, "|") {
				t.Errorf("positionals = %v, want %v", pos, c.pos)
			}
			for k, v := range c.flags {
				if flags[k] != v {
					t.Errorf("flag %s = %q, want %q", k, flags[k], v)
				}
			}
		})
	}
}

func TestRunExitCodesAndErrorJSON(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("STICKIT_DB", dir+"/db.sqlite")
	t.Chdir(dir) // a plain non-git directory is its own board

	run := func(args ...string) (int, string, string) {
		var out, errb bytes.Buffer
		code := Run(args, &out, &errb)
		return code, out.String(), errb.String()
	}

	if code, _, _ := run(); code != ExitUsage {
		t.Errorf("no args: code = %d, want %d", code, ExitUsage)
	}
	if code, out, _ := run("--help"); code != ExitOK || !strings.Contains(out, "Usage") {
		t.Errorf("--help: code = %d", code)
	}
	if code, _, _ := run("bogus"); code != ExitUsage {
		t.Errorf("unknown command: code = %d, want %d", code, ExitUsage)
	}
	code, _, errOut := run("resolve", "missing1")
	if code != ExitNotFound {
		t.Errorf("resolve missing: code = %d, want %d", code, ExitNotFound)
	}
	var decoded map[string]string
	if err := json.Unmarshal([]byte(strings.TrimSpace(errOut)), &decoded); err != nil || decoded["error"] == "" {
		t.Errorf("stderr must be one JSON error object, got %q (err %v)", errOut, err)
	}
}
