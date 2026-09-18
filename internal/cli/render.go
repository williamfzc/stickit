package cli

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/williamfzc/stickit/internal/store"
)

// out renders command results: JSON on a piped stdout, a table on a TTY.
type out struct {
	w      io.Writer
	pretty bool
}

func (o *out) notes(notes []store.Note) {
	if o.pretty {
		o.table(notes)
		return
	}
	if notes == nil {
		notes = []store.Note{}
	}
	_ = jsonEncoder(o.w).Encode(notes)
}

func (o *out) note(n store.Note) {
	if o.pretty {
		fmt.Fprintf(o.w, "added %s %s\n", n.ID, loc(n))
		return
	}
	_ = jsonEncoder(o.w).Encode(n)
}

func (o *out) reply(noteID string, r store.Reply) {
	if o.pretty {
		fmt.Fprintf(o.w, "reply %d added to %s\n", r.Seq, noteID)
		return
	}
	_ = jsonEncoder(o.w).Encode(struct {
		NoteID string `json:"note_id"`
		store.Reply
	}{noteID, r})
}

func (o *out) resolved(n store.Note) {
	if o.pretty {
		fmt.Fprintf(o.w, "resolved %s\n", n.ID)
		return
	}
	_ = jsonEncoder(o.w).Encode(n)
}

func (o *out) table(notes []store.Note) {
	tw := tabwriter.NewWriter(o.w, 2, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tSTATUS\tWHERE\tAUTHOR\tNOTE")
	if len(notes) == 0 {
		fmt.Fprintln(tw, "(no notes)\t\t\t\t")
		tw.Flush()
		return
	}
	for _, n := range notes {
		st := n.Status
		if n.Status == "active" {
			st = "-"
		}
		if n.Drifted {
			st += " ~drifted"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			n.ID, st, loc(n), provenance(n), oneLine(n.Body))
		for _, r := range n.Replies {
			fmt.Fprintf(tw, "  ↳ %d\t\t\t%s\t%s\n", r.Seq, r.Author, oneLine(r.Body))
		}
	}
	tw.Flush()
}

func loc(n store.Note) string {
	if n.StartLine == nil {
		return n.File
	}
	if *n.StartLine == *n.EndLine {
		return fmt.Sprintf("%s:%d", n.File, *n.StartLine)
	}
	return fmt.Sprintf("%s:%d-%d", n.File, *n.StartLine, *n.EndLine)
}

func provenance(n store.Note) string {
	branch := ""
	if n.Branch != nil {
		branch = "@" + *n.Branch
	}
	if len(n.Tags) > 0 {
		return n.Author + branch + " " + "#" + strings.Join(n.Tags, " #")
	}
	return n.Author + branch
}

// oneLine flattens a body for the table and keeps it short.
func oneLine(s string) string {
	s = strings.ReplaceAll(s, "\n", " ⏎ ")
	runes := []rune(s)
	if len(runes) > 60 {
		return string(runes[:59]) + "…"
	}
	return s
}
