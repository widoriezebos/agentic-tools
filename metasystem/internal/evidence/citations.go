package evidence

// The citation clause (design engine-owns-disk-lifetimes Part B 3.12 clause
// 3; DL4C-02, DL4C-14, DL4D-04, DL4D-10): an item is held while a record
// under the project's homes, the state root's records, memory, plans and
// docs, or a configured extra location cites it by path. The machine pass
// keeps one citation index per host, built by generations: a generation
// takes the inventory of every regular file under the roots (path, inode,
// size, mtime, ctime; every file whatever its size or content), then scans
// the inventory's files in order under a cursor, recording per file the
// "<segment>/<item>" tokens it names, across as many passes as the budget
// needs; it is published by atomic rename when the cursor reaches the end.
// Resumption is at the real boundaries: the walk persists the directories
// still to enter, and the scan cursor is (file index, byte offset) with
// the overlap window carried, so a huge inventory and a huge file both
// complete. At apply, the newest generation answers for every file whose
// tuple is unchanged; every file present now whose tuple is absent from it
// is scanned now. A file that cannot be read is Unknown for every item.
//
// The stated limit (Wido, 2026-09-28, accepted): a citation written after
// the apply-time scan and before the item's compaction, within one item's
// critical section, is not seen; record writers take no lock.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/project"
)

// CitationSchema names the index's files.
const CitationSchema = "metasystem.citation-index/1"

// citationToken is a path's "<segment>/<item>": twelve hex digits, a slash
// and a name. Every absolute spelling of a root, the ~/ spelling and every
// root-relative spelling contain it; a bare job or attempt id does not.
var citationToken = regexp.MustCompile(`[0-9a-f]{12}/[A-Za-z0-9._-]+`)

// citationOverlap is carried across chunk boundaries: the longest token
// (twelve digits, a slash, a file name) less one byte.
const citationOverlap = 12 + 1 + 255

// citationChunk is how much of a file one read scans.
const citationChunk = 1 << 20

// FileTuple is one inventoried file.
type FileTuple struct {
	Path    string `json:"path"`
	Inode   uint64 `json:"inode"`
	Size    int64  `json:"size"`
	MtimeNs int64  `json:"mtimeNs"`
	CtimeNs int64  `json:"ctimeNs"`
}

func tupleOf(path string, info os.FileInfo) FileTuple {
	tuple := FileTuple{Path: path, Size: info.Size(), MtimeNs: info.ModTime().UnixNano()}
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		tuple.Inode = uint64(stat.Ino)
		tuple.CtimeNs = ctimeNs(stat)
	}
	return tuple
}

// generationState is the running generation.
type generationState struct {
	Schema     string              `json:"schema"`
	Generation int                 `json:"generation"`
	Roots      []string            `json:"roots"`
	Pending    []string            `json:"pending"`
	Walked     bool                `json:"walked"`
	Files      []FileTuple         `json:"files"`
	Cursor     int                 `json:"cursor"`
	Offset     int64               `json:"offset"`
	Tail       []byte              `json:"tail,omitempty"`
	Hits       map[string][]string `json:"hits"`
	Unreadable map[string]string   `json:"unreadable,omitempty"`
}

// Generation is a published generation.
type Generation struct {
	Schema     string              `json:"schema"`
	Generation int                 `json:"generation"`
	Roots      []string            `json:"roots"`
	Files      []FileTuple         `json:"files"`
	Hits       map[string][]string `json:"hits"`
	Unreadable map[string]string   `json:"unreadable,omitempty"`
	Published  time.Time           `json:"published"`
}

// Citations is the host's citation index for one pass.
type Citations struct {
	// Dir is ~/.metasystem/stores/citations.
	Dir string
	// Roots are the citation roots of every armed installation.
	Roots func() ([]string, error)
	Now   time.Time
	Sync  diskstore.Syncer
	pass  *citationPass
	// generation is the newest published generation, read once per pass;
	// scanned are the changed files already scanned this pass.
	generation    *Generation
	generationErr error
	scanned       map[FileTuple][]string
}

// CitationRoots are one installation's roots (DL4D-04): every home of
// project.Homes over its roots, the state root's records, memory, plans
// and docs, and the evidence.citation-roots extras (absolute, or relative
// to the state root); never a list spelled relative to the installation.
func CitationRoots(installation, extras string) ([]string, error) {
	roots, err := project.ResolveRoots(installation)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, home := range project.Homes(roots) {
		paths = append(paths, home.Path)
	}
	for _, name := range []string{"records", "memory", "plans", "docs"} {
		paths = append(paths, roots.StateRoot.Path(name))
	}
	for _, extra := range strings.Split(extras, ",") {
		if extra = strings.TrimSpace(extra); extra == "" {
			continue
		}
		if !filepath.IsAbs(extra) {
			extra = roots.StateRoot.Path(extra)
		}
		paths = append(paths, filepath.Clean(extra))
	}
	return pruneNested(paths), nil
}

// pruneNested drops duplicates and every root inside another.
func pruneNested(paths []string) []string {
	sort.Strings(paths)
	var kept []string
	for _, path := range paths {
		if len(kept) > 0 {
			last := kept[len(kept)-1]
			if path == last || strings.HasPrefix(path, last+string(filepath.Separator)) {
				continue
			}
		}
		kept = append(kept, path)
	}
	return kept
}

func (c *Citations) statePath() string { return filepath.Join(c.Dir, "running.json") }

func (c *Citations) generationPath(n int) string {
	return filepath.Join(c.Dir, fmt.Sprintf("generation-%d.json", n))
}

// Newest reads the newest published generation; absent is os.ErrNotExist.
func (c *Citations) Newest() (Generation, error) {
	entries, err := os.ReadDir(c.Dir)
	if err != nil {
		return Generation{}, err
	}
	best := -1
	for _, entry := range entries {
		var n int
		if _, err := fmt.Sscanf(entry.Name(), "generation-%d.json", &n); err == nil && entry.Name() == fmt.Sprintf("generation-%d.json", n) && n > best {
			best = n
		}
	}
	if best < 0 {
		return Generation{}, os.ErrNotExist
	}
	data, err := os.ReadFile(c.generationPath(best))
	if err != nil {
		return Generation{}, err
	}
	var generation Generation
	if err := json.Unmarshal(data, &generation); err != nil || generation.Schema != CitationSchema {
		if err == nil {
			err = errors.New("not a citation record")
		}
		return Generation{}, fmt.Errorf("%s is unreadable: %w", c.generationPath(best), err)
	}
	return generation, nil
}

// Step advances the running generation within the context: the walk, then
// the scan; a finished scan is published and the next generation begins
// from a fresh inventory. It writes only under Dir.
func (c *Citations) Step(ctx context.Context) (published bool, err error) {
	roots, err := c.Roots()
	if err != nil {
		return false, err
	}
	state, err := c.readState()
	if err != nil || !sameRoots(state.Roots, roots) {
		next := 1
		if newest, err := c.Newest(); err == nil {
			next = newest.Generation + 1
		}
		state = generationState{Schema: CitationSchema, Generation: next, Roots: roots, Pending: append([]string(nil), roots...), Hits: map[string][]string{}}
	}
	defer func() {
		if !published {
			err = errors.Join(err, c.writeState(state))
		}
	}()
	for !state.Walked {
		if ctx.Err() != nil {
			return false, nil
		}
		state.walkOne()
	}
	for state.Cursor < len(state.Files) {
		if ctx.Err() != nil {
			return false, nil
		}
		state.scanOne(ctx)
	}
	generation := Generation{Schema: CitationSchema, Generation: state.Generation, Roots: state.Roots, Files: state.Files, Hits: state.Hits,
		Unreadable: state.Unreadable, Published: c.Now.UTC()}
	if newest, err := c.Newest(); err == nil && sameGeneration(newest, generation) {
		// Nothing changed since the newest generation: it still answers,
		// and the pass writes nothing (R-129).
		published = true
		if err := os.Remove(c.statePath()); err != nil && !errors.Is(err, os.ErrNotExist) {
			return false, err
		}
		return false, nil
	}
	data, err := json.Marshal(generation)
	if err != nil {
		return false, err
	}
	if err := c.Sync.WriteDurable(c.generationPath(state.Generation), data, fmt.Sprintf("gen-%d", c.Now.UnixNano())); err != nil {
		return false, err
	}
	published = true
	for n := state.Generation - 1; n > 0; n-- {
		if err := os.Remove(c.generationPath(n)); errors.Is(err, os.ErrNotExist) {
			break
		}
	}
	if err := os.Remove(c.statePath()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return true, err
	}
	return true, nil
}

func sameGeneration(a, b Generation) bool {
	left, _ := json.Marshal([]any{a.Roots, a.Files, a.Hits, a.Unreadable})
	right, _ := json.Marshal([]any{b.Roots, b.Files, b.Hits, b.Unreadable})
	return string(left) == string(right)
}

func sameRoots(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for index := range a {
		if a[index] != b[index] {
			return false
		}
	}
	return true
}

func (c *Citations) readState() (generationState, error) {
	data, err := os.ReadFile(c.statePath())
	if err != nil {
		return generationState{}, err
	}
	var state generationState
	if err := json.Unmarshal(data, &state); err != nil || state.Schema != CitationSchema {
		return generationState{}, errors.New("the citation record being written is unreadable")
	}
	if state.Hits == nil {
		state.Hits = map[string][]string{}
	}
	return state, nil
}

func (c *Citations) writeState(state generationState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return c.Sync.WriteDurable(c.statePath(), data, fmt.Sprintf("run-%d", c.Now.UnixNano()))
}

// walkOne enters one pending directory (or takes one pending file root).
func (s *generationState) walkOne() {
	if len(s.Pending) == 0 {
		s.Walked = true
		return
	}
	last := len(s.Pending) - 1
	path := s.Pending[last]
	s.Pending = s.Pending[:last]
	defer func() { s.Walked = len(s.Pending) == 0 }()
	info, err := os.Lstat(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return
	case err != nil:
		s.unreadable(path, err)
		return
	case info.Mode().IsRegular():
		s.Files = append(s.Files, tupleOf(path, info))
		return
	case !info.IsDir():
		return
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		s.unreadable(path, err)
		return
	}
	for index := len(entries) - 1; index >= 0; index-- {
		entry := entries[index]
		child := filepath.Join(path, entry.Name())
		switch {
		case entry.IsDir():
			s.Pending = append(s.Pending, child)
		case entry.Type().IsRegular():
			info, err := entry.Info()
			if err != nil {
				s.unreadable(child, err)
				continue
			}
			s.Files = append(s.Files, tupleOf(child, info))
		}
	}
}

func (s *generationState) unreadable(path string, err error) {
	if s.Unreadable == nil {
		s.Unreadable = map[string]string{}
	}
	s.Unreadable[path] = err.Error()
}

// scanOne scans the file at the cursor from its offset, one chunk at a
// time, carrying the overlap; a file whose tuple changed since the
// inventory is restarted from offset zero.
func (s *generationState) scanOne(ctx context.Context) {
	tuple := s.Files[s.Cursor]
	next := func() { s.Cursor, s.Offset, s.Tail = s.Cursor+1, 0, nil }
	info, err := os.Lstat(tuple.Path)
	if errors.Is(err, os.ErrNotExist) {
		next()
		return
	}
	if err != nil {
		s.unreadable(tuple.Path, err)
		next()
		return
	}
	if now := tupleOf(tuple.Path, info); now != tuple {
		s.Files[s.Cursor], s.Offset, s.Tail = now, 0, nil
		delete(s.Hits, tuple.Path)
	}
	file, err := os.Open(tuple.Path)
	if err != nil {
		s.unreadable(tuple.Path, err)
		next()
		return
	}
	defer file.Close()
	if _, err := file.Seek(s.Offset, io.SeekStart); err != nil {
		s.unreadable(tuple.Path, err)
		next()
		return
	}
	buffer := make([]byte, citationChunk)
	for {
		if ctx.Err() != nil {
			return
		}
		read, err := io.ReadFull(file, buffer)
		eof := errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF)
		if err != nil && !eof {
			s.unreadable(tuple.Path, err)
			next()
			return
		}
		window := append(append([]byte(nil), s.Tail...), buffer[:read]...)
		s.record(tuple.Path, tokensIn(window, eof))
		s.Offset += int64(read)
		if eof {
			delete(s.Unreadable, tuple.Path)
			next()
			return
		}
		if len(window) > citationOverlap {
			window = window[len(window)-citationOverlap:]
		}
		s.Tail = window
	}
}

func (s *generationState) record(path string, tokens []string) {
	for _, token := range tokens {
		if !contains(s.Hits[path], token) {
			s.Hits[path] = append(s.Hits[path], token)
		}
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// tokensIn finds the tokens of a window; unless it is the file's end, a
// token touching the window's end may continue in the next read and is
// left to it.
func tokensIn(window []byte, eof bool) []string {
	var tokens []string
	for _, match := range citationToken.FindAllIndex(window, -1) {
		if !eof && match[1] == len(window) {
			continue
		}
		tokens = append(tokens, string(window[match[0]:match[1]]))
	}
	return tokens
}

// scanWhole returns every token of one file, read in chunks under the
// context.
func scanWhole(ctx context.Context, path string) ([]string, error) {
	state := generationState{Files: []FileTuple{{Path: path}}, Hits: map[string][]string{}}
	if info, err := os.Lstat(path); err == nil {
		state.Files[0] = tupleOf(path, info)
	}
	state.scanOne(ctx)
	if reason, bad := state.Unreadable[path]; bad {
		return nil, errors.New(reason)
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return state.Hits[path], nil
}

// citationPass is one pass's answer set: the newest generation's hits for
// every unchanged file, and a fresh scan of every changed or new one.
type citationPass struct {
	hits    map[string][]string
	unknown string
	pending string
}

// Cited answers clause 3 for one item, at that item's own critical
// section (Round B2, F-7): the newest generation answers for unchanged
// files, and every file changed since it is scanned now (a file already
// scanned this pass with the same tuple is not read twice).
func (c *Citations) Cited(ctx context.Context, segment Segment, item Item) ([]string, string, string) {
	c.pass = c.build(ctx)
	if c.pass.unknown != "" || c.pass.pending != "" {
		return nil, c.pass.unknown, c.pass.pending
	}
	name := segment.Git
	if item.Kind == diskstore.KindEvents {
		name = segment.Installation
	}
	pattern := name + "/" + item.Name
	var files []string
	for path, tokens := range c.pass.hits {
		for _, token := range tokens {
			// The token ends at the next slash, so a path inside the item
			// cites it through its "<segment>/<item>" prefix, and a longer
			// name is another item.
			if token == pattern {
				files = append(files, path)
				break
			}
		}
	}
	sort.Strings(files)
	return files, "", ""
}

func (c *Citations) build(ctx context.Context) *citationPass {
	pass := &citationPass{hits: map[string][]string{}}
	if c.generation == nil && c.generationErr == nil {
		generation, err := c.Newest()
		c.generation, c.generationErr = &generation, err
	}
	generation, err := *c.generation, c.generationErr
	switch {
	case errors.Is(err, os.ErrNotExist):
		pass.pending = "the first citation generation has not completed yet"
		return pass
	case err != nil:
		pass.unknown = "the citation index cannot be read: " + err.Error()
		return pass
	}
	for path, reason := range generation.Unreadable {
		pass.unknown = "a citation file cannot be read: " + path + ": " + reason
		return pass
	}
	roots, err := c.Roots()
	if err != nil {
		pass.unknown = "the citation roots cannot be resolved: " + err.Error()
		return pass
	}
	known := map[FileTuple]bool{}
	for _, tuple := range generation.Files {
		known[tuple] = true
	}
	current := generationState{Pending: append([]string(nil), roots...), Hits: map[string][]string{}}
	for len(current.Pending) > 0 {
		if ctx.Err() != nil {
			pass.pending = "the citation roots were not walked within the pass budget"
			return pass
		}
		current.walkOne()
	}
	for path, reason := range current.Unreadable {
		pass.unknown = "a citation file cannot be read: " + path + ": " + reason
		return pass
	}
	for _, tuple := range current.Files {
		if known[tuple] {
			if tokens := generation.Hits[tuple.Path]; len(tokens) > 0 {
				pass.hits[tuple.Path] = tokens
			}
			continue
		}
		if tokens, done := c.scanned[tuple]; done {
			if len(tokens) > 0 {
				pass.hits[tuple.Path] = tokens
			}
			continue
		}
		tokens, err := scanWhole(ctx, tuple.Path)
		if ctx.Err() != nil {
			pass.pending = "more changed citation files than the pass budget scans; the running generation covers them"
			return pass
		}
		if err != nil {
			pass.unknown = "a citation file cannot be read: " + tuple.Path + ": " + err.Error()
			return pass
		}
		if c.scanned == nil {
			c.scanned = map[FileTuple][]string{}
		}
		c.scanned[tuple] = tokens
		if len(tokens) > 0 {
			pass.hits[tuple.Path] = tokens
		}
	}
	return pass
}
