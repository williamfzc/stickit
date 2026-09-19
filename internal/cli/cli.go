// Package cli parses argv, runs one command, and renders output: JSON when
// stdout is piped, a table on a terminal.
package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/williamfzc/stickit/internal/store"
)

// Exit codes, stable across versions.
const (
	ExitOK       = 0
	ExitUsage    = 1 // bad arguments or bad target
	ExitNotFound = 2 // a note id does not exist in this board
	ExitStore    = 3 // storage or environment failure
)

const usage = `stickit — sticky notes pinned to files

Usage:
  stickit add <file[:line[-line]]> "body"   [--reply-to <id>]
  stickit ls  [path] ["keyword"]            [--all]
  stickit resolve <id>

Auxiliary (not part of the agent surface):
  stickit --skill  print the agent skill snippet
  stickit dump     JSONL backup of every note

In ls, an argument naming an existing file or directory (or containing /)
filters by path; anything else is a full-text keyword. --all includes
archived notes.

JSON on a piped stdout, a table on a terminal. Exit codes: 0 ok, 1 usage,
2 not found, 3 store error.

Is an agent driving this tool? Read the three-line contract before use:
  stickit --skill`

const skillText = `Before editing a file:           stickit ls <file>
Learned something non-obvious:   stickit add <file:line> "... #gotcha"
Done with a note:                stickit resolve <id>
`

// Run executes one command and returns the process exit code.
func Run(argv []string, stdout, stderr io.Writer) int {
	o := &out{w: stdout, pretty: isTTY(stdout)}
	if len(argv) == 0 {
		return failUsage(stderr, errors.New("no command given; try --help"))
	}
	switch argv[0] {
	case "-h", "--help", "help":
		fmt.Fprintln(stdout, usage)
		return ExitOK
	case "--skill":
		fmt.Fprint(stdout, skillText)
		return ExitOK
	case "add":
		return cmdAdd(argv[1:], o, stderr)
	case "ls":
		return cmdLs(argv[1:], o, stderr)
	case "resolve":
		return cmdResolve(argv[1:], o, stderr)
	case "dump":
		return cmdDump(o, stderr)
	default:
		return failUsage(stderr, fmt.Errorf("unknown command %q", argv[0]))
	}
}

// failUsage reports a pre-parse usage failure: the usage text for a human on
// the terminal, one JSON error object for a machine reading a pipe.
func failUsage(stderr io.Writer, err error) int {
	if isTTY(stderr) {
		fmt.Fprintf(stderr, "stickit: %s\n\n%s\n", err.Error(), usage)
	} else {
		_ = jsonEncoder(stderr).Encode(map[string]string{"error": err.Error()})
	}
	return ExitUsage
}

// cmdAdd implements `add <file[:line[-line]]> "body"`, and with --reply-to
// the threaded variant that appends to an existing note.
func cmdAdd(argv []string, o *out, stderr io.Writer) int {
	pos, flags, err := parseFlags(argv, map[string]bool{"reply-to": true})
	if err != nil {
		return fail(stderr, o, ExitUsage, err)
	}
	b, err := detectBoard()
	if err != nil {
		return fail(stderr, o, ExitStore, err)
	}
	if replyTo := flags["reply-to"]; replyTo != "" {
		if len(pos) != 1 {
			return fail(stderr, o, ExitUsage, fmt.Errorf("usage: stickit add --reply-to <id> %q", "body"))
		}
		if strings.TrimSpace(pos[0]) == "" {
			return fail(stderr, o, ExitUsage, fmt.Errorf("reply body must not be empty"))
		}
		return withStore(stderr, o, func(s *store.Store) int {
			r, err := s.AddReply(b.Key, replyTo, pos[0], boardAuthor(b))
			if errors.Is(err, store.ErrNotFound) {
				return fail(stderr, o, ExitNotFound, fmt.Errorf("no note %s in this board", replyTo))
			}
			if err != nil {
				return fail(stderr, o, ExitStore, err)
			}
			o.reply(replyTo, r)
			return ExitOK
		})
	}
	if len(pos) != 2 {
		return fail(stderr, o, ExitUsage, fmt.Errorf("usage: stickit add <file[:line[-line]]> %q", "body"))
	}
	target, body := pos[0], pos[1]
	if strings.TrimSpace(body) == "" {
		return fail(stderr, o, ExitUsage, fmt.Errorf("note body must not be empty"))
	}
	t, err := parseTarget(b, target)
	if err != nil {
		return fail(stderr, o, ExitUsage, err)
	}
	note := store.NewNote{
		RepoKey: b.Key,
		File:    t.File,
		Start:   t.Start,
		End:     t.End,
		Body:    body,
		Author:  boardAuthor(b),
		Branch:  b.Branch,
		Commit:  b.Commit,
	}
	if t.Start > 0 {
		lines, err := readLines(t.AbsPath)
		if err != nil {
			return fail(stderr, o, ExitUsage, err)
		}
		blk, err := anchorLines(lines, t.Start, t.End)
		if err != nil {
			return fail(stderr, o, ExitUsage, err)
		}
		note.Hash = anchorHash(blk)
		note.NormHash = anchorNormHash(blk)
	}
	return withStore(stderr, o, func(s *store.Store) int {
		n, err := s.AddNote(note)
		if err != nil {
			return fail(stderr, o, ExitStore, err)
		}
		o.note(n)
		return ExitOK
	})
}

// cmdLs implements `ls [path] ["keyword"] [--all]`.
func cmdLs(argv []string, o *out, stderr io.Writer) int {
	pos, flags, err := parseFlags(argv, map[string]bool{"all": false})
	if err != nil {
		return fail(stderr, o, ExitUsage, err)
	}
	if len(pos) > 2 {
		return fail(stderr, o, ExitUsage, fmt.Errorf("usage: stickit ls [path] [\"keyword\"]"))
	}
	b, err := detectBoard()
	if err != nil {
		return fail(stderr, o, ExitStore, err)
	}
	f := store.Filter{
		RepoKey:         b.Key,
		IncludeArchived: flags["all"] != "",
	}
	if len(pos) >= 1 && isPathArg(pos[0]) {
		rel, err := boardRel(b, pos[0])
		if err != nil {
			return fail(stderr, o, ExitUsage, err)
		}
		f.File = rel
		f.FileIsDir = pathIsDir(b, pos[0])
		if len(pos) == 2 {
			f.Keyword = strings.TrimSpace(pos[1])
		}
	} else if len(pos) == 1 {
		f.Keyword = strings.TrimSpace(pos[0])
	} else if len(pos) == 2 {
		return fail(stderr, o, ExitUsage, fmt.Errorf("first argument %q does not name a path; pass the keyword alone", pos[0]))
	}
	return withStore(stderr, o, func(s *store.Store) int {
		notes, err := s.List(f, boardContent(b))
		if err != nil {
			return fail(stderr, o, ExitStore, err)
		}
		o.notes(notes)
		return ExitOK
	})
}

// cmdResolve implements `resolve <id>`.
func cmdResolve(argv []string, o *out, stderr io.Writer) int {
	pos, _, err := parseFlags(argv, map[string]bool{})
	if err != nil {
		return fail(stderr, o, ExitUsage, err)
	}
	if len(pos) != 1 {
		return fail(stderr, o, ExitUsage, fmt.Errorf("usage: stickit resolve <id>"))
	}
	b, err := detectBoard()
	if err != nil {
		return fail(stderr, o, ExitStore, err)
	}
	return withStore(stderr, o, func(s *store.Store) int {
		n, err := s.Resolve(b.Key, pos[0])
		if errors.Is(err, store.ErrNotFound) {
			return fail(stderr, o, ExitNotFound, fmt.Errorf("no note %s in this board", pos[0]))
		}
		if err != nil {
			return fail(stderr, o, ExitStore, err)
		}
		o.resolved(n)
		return ExitOK
	})
}

func cmdDump(o *out, stderr io.Writer) int {
	return withStore(stderr, o, func(s *store.Store) int {
		if err := s.DumpJSONL(o.w); err != nil {
			return fail(stderr, o, ExitStore, err)
		}
		return ExitOK
	})
}

// parseFlags splits --flag[=value] tokens from positionals; flags may appear
// anywhere. allowed lists the recognized flags; a true value marks a flag
// that takes a value. Anything else is a usage error — a mistyped flag must
// not pass silently.
func parseFlags(args []string, allowed map[string]bool) ([]string, map[string]string, error) {
	var positionals []string
	flags := map[string]string{}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "-" || !strings.HasPrefix(a, "-") {
			positionals = append(positionals, a)
			continue
		}
		name := strings.TrimLeft(a, "-")
		val, hasVal := "", false
		if eq := strings.Index(name, "="); eq >= 0 {
			name, val, hasVal = name[:eq], name[eq+1:], true
		}
		takesValue, ok := allowed[name]
		if !ok {
			return nil, nil, fmt.Errorf("unknown flag --%s", name)
		}
		if !takesValue {
			if hasVal {
				return nil, nil, fmt.Errorf("flag --%s takes no value", name)
			}
			flags[name] = "true"
			continue
		}
		if !hasVal {
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
				return nil, nil, fmt.Errorf("flag --%s needs a value", name)
			}
			i++
			val = args[i]
		}
		flags[name] = val
	}
	return positionals, flags, nil
}

// isTTY reports whether w is a terminal; piped stdout gets JSON.
func isTTY(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	st, err := f.Stat()
	if err != nil {
		return false
	}
	return st.Mode()&os.ModeCharDevice != 0
}

// fail renders one error: JSON when stderr is captured (the agent case),
// plain text when a human is reading the terminal. The exit code carries the
// machine meaning either way.
func fail(stderr io.Writer, o *out, code int, err error) int {
	msg := err.Error()
	if isTTY(stderr) {
		fmt.Fprintf(stderr, "stickit: %s\n", msg)
	} else {
		_ = jsonEncoder(stderr).Encode(map[string]string{"error": msg})
	}
	return code
}
