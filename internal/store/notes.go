package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/williamfzc/stickit/internal/anchor"
)

// Retention is how long archived notes survive before the lazy GC on write
// paths drops them for good.
const Retention = 30 * 24 * time.Hour

// ErrNotFound is returned when a note id does not exist in the board.
var ErrNotFound = errors.New("note not found")

// Note is a pinned sticky note as served to callers.
type Note struct {
	ID        string   `json:"id"`
	File      string   `json:"file"`
	StartLine *int     `json:"start_line"` // null for file-level notes
	EndLine   *int     `json:"end_line"`
	Status    string   `json:"status"` // active | stale | archived
	Drifted   bool     `json:"drifted"`
	Tags      []string `json:"tags"`
	Author    string   `json:"author"`
	Branch    *string  `json:"branch"` // null outside git and before the first commit
	Commit    *string  `json:"commit"` // HEAD at write time; null where branch is
	Body      string   `json:"body"`
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`
	Replies   []Reply  `json:"replies"`

	// write-time anchor hashes, carried for lazy validation; not served.
	anchorHash string
	anchorNorm string
}

// Reply is a threaded response to a note.
type Reply struct {
	Seq       int    `json:"seq"`
	Author    string `json:"author"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
}

// NewNote carries everything add needs to record one note. The hashes cover
// the anchored lines at write time; both empty for file-level notes.
type NewNote struct {
	RepoKey  string
	File     string
	Start    int
	End      int
	Hash     string
	NormHash string
	Body     string
	Author   string
	Branch   string
	Commit   string
}

var tagRe = regexp.MustCompile(`#[A-Za-z0-9][A-Za-z0-9_-]*`)

// ParseTags extracts unique lowercased #hashtags from a body, first-seen
// order preserved.
func ParseTags(body string) []string {
	seen := map[string]bool{}
	var tags []string
	for _, m := range tagRe.FindAllString(body, -1) {
		t := strings.ToLower(strings.TrimPrefix(m, "#"))
		if !seen[t] {
			seen[t] = true
			tags = append(tags, t)
		}
	}
	return tags
}

func hasTag(tags []string, want string) bool {
	for _, t := range tags {
		if t == want {
			return true
		}
	}
	return false
}

// Content resolves a board-relative file to its current lines for lazy
// validation; exists is false when the file is gone.
type Content func(rel string) (lines []string, exists bool, err error)

// AddNote records one note and its full-text entry in a single immediate
// transaction, running lazy maintenance first.
func (s *Store) AddNote(n NewNote) (Note, error) {
	tx, err := s.DB.Begin()
	if err != nil {
		return Note{}, err
	}
	defer tx.Rollback()
	if err := maintain(tx, n.RepoKey); err != nil {
		return Note{}, err
	}
	id, err := freshID(tx)
	if err != nil {
		return Note{}, err
	}
	ts := now()
	_, err = tx.Exec(`INSERT INTO notes
		(id, repo, file, start_line, end_line, content_hash, norm_hash,
		 body, tags, author, branch, "commit", status, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,'active', ?, ?)`,
		id, n.RepoKey, n.File, nullInt(n.Start), nullInt(n.End),
		nullStr(n.Hash), nullStr(n.NormHash), n.Body,
		nullStr(strings.Join(ParseTags(n.Body), " ")),
		n.Author, nullStr(n.Branch), nullStr(n.Commit), ts, ts)
	if err != nil {
		return Note{}, err
	}
	if _, err := tx.Exec(`INSERT INTO fts (body, kind, ref_id) VALUES (?, 'note', ?)`, n.Body, id); err != nil {
		return Note{}, err
	}
	if err := tx.Commit(); err != nil {
		return Note{}, err
	}
	return s.getNote(n.RepoKey, id)
}

// AddReply appends one threaded reply to a note. The note id is scoped to
// the board: a foreign id does not exist here.
func (s *Store) AddReply(repoKey, noteID, body, author string) (Reply, error) {
	tx, err := s.DB.Begin()
	if err != nil {
		return Reply{}, err
	}
	defer tx.Rollback()
	if err := maintain(tx, repoKey); err != nil {
		return Reply{}, err
	}
	var noteRepo string
	err = tx.QueryRow(`SELECT repo FROM notes WHERE id = ?`, noteID).Scan(&noteRepo)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && noteRepo != repoKey) {
		return Reply{}, ErrNotFound
	}
	if err != nil {
		return Reply{}, err
	}
	var seq int
	if err := tx.QueryRow(`SELECT COALESCE(MAX(seq), 0) + 1 FROM threads WHERE note_id = ?`, noteID).Scan(&seq); err != nil {
		return Reply{}, err
	}
	ts := now()
	if _, err := tx.Exec(`INSERT INTO threads (note_id, seq, author, body, created_at) VALUES (?,?,?,?,?)`,
		noteID, seq, author, body, ts); err != nil {
		return Reply{}, err
	}
	if _, err := tx.Exec(`INSERT INTO fts (body, kind, ref_id) VALUES (?, 'thread', ?)`, body, noteID); err != nil {
		return Reply{}, err
	}
	if err := tx.Commit(); err != nil {
		return Reply{}, err
	}
	return Reply{Seq: seq, Author: author, Body: body, CreatedAt: ts}, nil
}

// Resolve archives a note. Idempotent: resolving an already-archived note
// succeeds.
func (s *Store) Resolve(repoKey, id string) (Note, error) {
	tx, err := s.DB.Begin()
	if err != nil {
		return Note{}, err
	}
	defer tx.Rollback()
	if err := maintain(tx, repoKey); err != nil {
		return Note{}, err
	}
	res, err := tx.Exec(`UPDATE notes SET status = 'archived', updated_at = ? WHERE id = ? AND repo = ?`,
		now(), id, repoKey)
	if err != nil {
		return Note{}, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return Note{}, err
	}
	if n == 0 {
		return Note{}, ErrNotFound
	}
	if err := tx.Commit(); err != nil {
		return Note{}, err
	}
	return s.getNote(repoKey, id)
}

// Filter selects notes from one board.
type Filter struct {
	RepoKey string
	// File is an exact board-relative file when FileIsDir is false, or a
	// directory prefix ("src/") when true; "" for the whole board.
	File      string
	FileIsDir bool
	// Keyword is a free-text full-text query; "" to skip search.
	Keyword string
	// IncludeArchived lifts the default hiding of archived notes.
	IncludeArchived bool
}

// List serves notes for a board. Every read lazily re-validates anchors
// against the current files (moving them on drift, expiring stale #handoff
// notes) before results are filtered and ordered.
func (s *Store) List(f Filter, content Content) ([]Note, error) {
	ids, rank, err := s.searchIDs(f)
	if err != nil {
		return nil, err
	}
	notes, err := s.candidates(f, ids, rank)
	if err != nil {
		return nil, err
	}
	if err := s.revalidate(notes, content); err != nil {
		return nil, err
	}
	out := notes[:0]
	for _, n := range notes {
		if f.IncludeArchived || n.Status != "archived" {
			out = append(out, n)
		}
	}
	notes = out
	if len(rank) > 0 {
		sort.SliceStable(notes, func(i, j int) bool { return rank[notes[i].ID] < rank[notes[j].ID] })
	} else {
		sort.SliceStable(notes, func(i, j int) bool { return lessByPosition(notes[i], notes[j]) })
	}
	return notes, nil
}

func lessByPosition(a, b Note) bool {
	if a.File != b.File {
		return a.File < b.File
	}
	al, bl := lineKey(a), lineKey(b)
	if al != bl {
		return al < bl
	}
	return a.CreatedAt < b.CreatedAt
}

func lineKey(n Note) int {
	if n.StartLine == nil {
		return 0
	}
	return *n.StartLine
}

// searchIDs resolves the keyword to matching note ids in FTS rank order
// (note and reply bodies both count; a reply hit surfaces its note). A
// multi-token keyword is an AND at note level: every token must appear
// somewhere among the note's own body or its replies, not necessarily in
// one single row. No keyword → (nil, nil); a keyword with no hits → an
// empty set, never the whole board.
func (s *Store) searchIDs(f Filter) ([]string, map[string]int, error) {
	toks := strings.Fields(f.Keyword)
	if len(toks) == 0 {
		return nil, nil, nil
	}
	quoted := make([]string, len(toks))
	for i, t := range toks {
		quoted[i] = quoteFtsToken(t)
	}
	if len(toks) == 1 {
		ids, err := s.ftsMatch(quoted[0])
		if err != nil {
			return nil, nil, err
		}
		return ranked(ids, nil)
	}
	// Intersect the per-token hit sets, then present what survives in the
	// rank order of an OR query over all tokens.
	inter := map[string]bool{}
	first, err := s.ftsMatch(quoted[0])
	if err != nil {
		return nil, nil, err
	}
	for _, id := range first {
		inter[id] = true
	}
	for _, q := range quoted[1:] {
		hits, err := s.ftsMatch(q)
		if err != nil {
			return nil, nil, err
		}
		keep := map[string]bool{}
		for _, id := range hits {
			if inter[id] {
				keep[id] = true
			}
		}
		inter = keep
	}
	any, err := s.ftsMatch(strings.Join(quoted, " OR "))
	if err != nil {
		return nil, nil, err
	}
	return ranked(any, inter)
}

// ranked dedups id hits (optionally keeping only members of keep) in their
// FTS rank order, producing the id list and its rank index.
func ranked(ids []string, keep map[string]bool) ([]string, map[string]int, error) {
	order := []string{}
	rank := map[string]int{}
	for _, id := range ids {
		if keep != nil && !keep[id] {
			continue
		}
		if _, dup := rank[id]; !dup {
			rank[id] = len(order)
			order = append(order, id)
		}
	}
	return order, rank, nil
}

// ftsMatch returns the ref ids matching one FTS expression, best rank first.
func (s *Store) ftsMatch(expr string) ([]string, error) {
	rows, err := s.DB.Query(`SELECT ref_id FROM fts WHERE fts MATCH ? ORDER BY rank`, expr)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// quoteFtsToken makes one keyword token a safe quoted FTS5 phrase.
func quoteFtsToken(tok string) string {
	return `"` + strings.ReplaceAll(tok, `"`, `""`) + `"`
}

// anchorState carries what lazy validation needs beyond the served Note.
type anchorState struct {
	hash string
	norm string
}

func (s *Store) candidates(f Filter, ids []string, rank map[string]int) ([]Note, error) {
	q := `SELECT id, content_hash, norm_hash FROM notes WHERE repo = ?`
	args := []any{f.RepoKey}
	if ids != nil {
		if len(ids) == 0 {
			return nil, nil
		}
		q += ` AND id IN (` + placeholders(len(ids)) + `)`
		for _, id := range ids {
			args = append(args, id)
		}
	}
	if f.File != "" {
		if f.FileIsDir {
			q += ` AND (file = ? OR file LIKE ? ESCAPE '\')`
			args = append(args, strings.TrimSuffix(f.File, "/"), likeEscape(f.File)+"%")
		} else {
			q += ` AND file = ?`
			args = append(args, f.File)
		}
	}
	if !f.IncludeArchived {
		q += ` AND status != 'archived'`
	}
	q += ` ORDER BY file, start_line, created_at`
	rows, err := s.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	states := map[string]anchorState{}
	var notes []Note
	for rows.Next() {
		var id string
		var hash, norm sql.NullString
		if err := rows.Scan(&id, &hash, &norm); err != nil {
			return nil, err
		}
		states[id] = anchorState{hash: hash.String, norm: norm.String}
		n, err := s.getNote(f.RepoKey, id)
		if err != nil {
			return nil, err
		}
		notes = append(notes, n)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range notes {
		notes[i].anchorHash = states[notes[i].ID].hash
		notes[i].anchorNorm = states[notes[i].ID].norm
	}
	return notes, nil
}

// revalidate is the lazy half of the anchoring contract: hash the current
// lines, silently move and re-baseline drifted anchors, archive expired
// #handoff notes, and flag everything else that no longer matches as stale.
// A note whose anchor matches again is restored to active — staleness is a
// live view of the files, not history. Archived notes are terminal and never
// revalidated. Both the database and the in-memory notes are updated.
func (s *Store) revalidate(notes []Note, content Content) error {
	type move struct {
		start, end int
		hash, norm string
	}
	type change struct {
		note   *Note
		move   *move
		status string
	}
	var changes []change
	cache := map[string]struct {
		lines  []string
		exists bool
		err    error
	}{}
	linesOf := func(rel string) ([]string, bool, error) {
		c, ok := cache[rel]
		if !ok {
			lines, exists, err := content(rel)
			c = struct {
				lines  []string
				exists bool
				err    error
			}{lines, exists, err}
			cache[rel] = c
		}
		return c.lines, c.exists, c.err
	}
	for i := range notes {
		n := &notes[i]
		if n.Status == "archived" {
			continue
		}
		start, end := 0, 0
		if n.StartLine != nil {
			start, end = *n.StartLine, *n.EndLine
		}
		lines, exists, err := linesOf(n.File)
		if err != nil {
			return err
		}
		res := anchor.Validate(lines, exists, start, end, n.anchorHash, n.anchorNorm)
		switch res.State {
		case anchor.Fresh:
			if n.Status == "stale" {
				changes = append(changes, change{note: n, status: "active"})
			}
		case anchor.Drifted:
			// Re-baseline the hashes to the content at the new position so
			// the next read is fresh instead of re-drifting forever.
			blk := lines[res.Start-1 : res.End]
			ch := change{note: n, move: &move{res.Start, res.End, anchor.Hash(blk), anchor.NormHash(blk)}}
			if n.Status == "stale" {
				ch.status = "active"
			}
			changes = append(changes, ch)
		case anchor.Stale:
			if hasTag(n.Tags, "handoff") {
				changes = append(changes, change{note: n, status: "archived"})
			} else if n.Status != "stale" {
				changes = append(changes, change{note: n, status: "stale"})
			}
		}
	}
	if len(changes) == 0 {
		return nil
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	ts := now()
	for _, c := range changes {
		n := c.note
		status := n.Status
		if c.status != "" {
			status = c.status
		}
		if c.move != nil {
			if _, err := tx.Exec(`UPDATE notes SET start_line = ?, end_line = ?, content_hash = ?, norm_hash = ?,
				drifted_at = ?, updated_at = ?, status = ? WHERE id = ?`,
				c.move.start, c.move.end, nullStr(c.move.hash), nullStr(c.move.norm), ts, ts, status, n.ID); err != nil {
				return err
			}
			sv, ev := c.move.start, c.move.end
			n.StartLine, n.EndLine = &sv, &ev
			n.anchorHash, n.anchorNorm = c.move.hash, c.move.norm
			n.Drifted = true
		} else if _, err := tx.Exec(`UPDATE notes SET status = ?, updated_at = ? WHERE id = ?`, status, ts, n.ID); err != nil {
			return err
		}
		n.Status = status
		n.UpdatedAt = ts
	}
	return tx.Commit()
}

// DumpJSONL writes every note in the store, all boards, as JSON lines —
// the backup surface, not part of the agent contract.
func (s *Store) DumpJSONL(w io.Writer) error {
	rows, err := s.DB.Query(`SELECT id, repo FROM notes ORDER BY repo, file, created_at`)
	if err != nil {
		return err
	}
	defer rows.Close()
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	for rows.Next() {
		var id, repo string
		if err := rows.Scan(&id, &repo); err != nil {
			return err
		}
		n, err := s.getNote(repo, id)
		if err != nil {
			return err
		}
		dump := struct {
			Note
			Repo string `json:"repo"`
		}{n, repo}
		if err := enc.Encode(dump); err != nil {
			return err
		}
	}
	return rows.Err()
}

func (s *Store) getNote(repoKey, id string) (Note, error) {
	row := s.DB.QueryRow(`SELECT id, file, start_line, end_line, status, drifted_at,
		tags, author, branch, "commit", body, created_at, updated_at
		FROM notes WHERE id = ? AND repo = ?`, id, repoKey)
	var n Note
	var tags, driftedAt sql.NullString
	var start, end sql.NullInt64
	var branch, commit sql.NullString
	if err := row.Scan(&n.ID, &n.File, &start, &end, &n.Status, &driftedAt,
		&tags, &n.Author, &branch, &commit, &n.Body, &n.CreatedAt, &n.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Note{}, ErrNotFound
		}
		return Note{}, err
	}
	if start.Valid {
		v := int(start.Int64)
		n.StartLine = &v
	}
	if end.Valid {
		v := int(end.Int64)
		n.EndLine = &v
	}
	n.Drifted = driftedAt.Valid
	n.Tags = []string{}
	if tags.Valid && tags.String != "" {
		n.Tags = strings.Split(tags.String, " ")
	}
	if branch.Valid {
		s := branch.String
		n.Branch = &s
	}
	if commit.Valid {
		s := commit.String
		n.Commit = &s
	}
	replies, err := s.replies(id)
	if err != nil {
		return Note{}, err
	}
	n.Replies = replies
	return n, nil
}

func (s *Store) replies(noteID string) ([]Reply, error) {
	rows, err := s.DB.Query(`SELECT seq, author, body, created_at FROM threads WHERE note_id = ? ORDER BY seq`, noteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	replies := []Reply{}
	for rows.Next() {
		var r Reply
		if err := rows.Scan(&r.Seq, &r.Author, &r.Body, &r.CreatedAt); err != nil {
			return nil, err
		}
		replies = append(replies, r)
	}
	return replies, rows.Err()
}

// maintain is the lazy half of "maintenance is behavior": every write path
// expires stale #handoff notes and GCs archived notes past retention.
func maintain(tx *sql.Tx, repoKey string) error {
	rows, err := tx.Query(`SELECT id, tags FROM notes WHERE repo = ? AND status = 'stale'`, repoKey)
	if err != nil {
		return err
	}
	var expired []string
	for rows.Next() {
		var id, tags sql.NullString
		if err := rows.Scan(&id, &tags); err != nil {
			rows.Close()
			return err
		}
		var parsed []string
		if tags.Valid && tags.String != "" {
			parsed = strings.Split(tags.String, " ")
		}
		if hasTag(parsed, "handoff") {
			expired = append(expired, id.String)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, id := range expired {
		if _, err := tx.Exec(`UPDATE notes SET status = 'archived', updated_at = ? WHERE id = ?`, now(), id); err != nil {
			return err
		}
	}
	cutoff := time.Now().UTC().Add(-Retention).Format(timeLayout)
	gone, err := tx.Query(`SELECT id FROM notes WHERE repo = ? AND status = 'archived' AND updated_at < ?`, repoKey, cutoff)
	if err != nil {
		return err
	}
	var dead []string
	for gone.Next() {
		var id string
		if err := gone.Scan(&id); err != nil {
			gone.Close()
			return err
		}
		dead = append(dead, id)
	}
	if err := gone.Err(); err != nil {
		gone.Close()
		return err
	}
	gone.Close()
	for _, id := range dead {
		for _, q := range []string{
			`DELETE FROM threads WHERE note_id = ?`,
			`DELETE FROM fts WHERE ref_id = ?`,
			`DELETE FROM notes WHERE id = ?`,
		} {
			if _, err := tx.Exec(q, id); err != nil {
				return err
			}
		}
	}
	return nil
}

const idAlphabet = "abcdefghjkmnpqrstuvwxyz23456789"

func freshID(tx *sql.Tx) (string, error) {
	for i := 0; i < 5; i++ {
		buf := make([]byte, 10)
		if _, err := rand.Read(buf); err != nil {
			return "", err
		}
		for j, b := range buf {
			buf[j] = idAlphabet[int(b)%len(idAlphabet)]
		}
		id := string(buf)
		var one string
		err := tx.QueryRow(`SELECT id FROM notes WHERE id = ?`, id).Scan(&one)
		if errors.Is(err, sql.ErrNoRows) {
			return id, nil
		}
		if err != nil {
			return "", err
		}
	}
	return "", errors.New("could not allocate a note id")
}

const timeLayout = "2006-01-02T15:04:05.000000000Z"

func now() string { return time.Now().UTC().Format(timeLayout) }

func nullInt(v int) any {
	if v == 0 {
		return nil
	}
	return v
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

func likeEscape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	return strings.ReplaceAll(s, `_`, `\_`)
}
