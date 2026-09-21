// Package project reads the thread of intent: the six subsections of the
// master's Project section, over documents that already exist at their
// canonical locations in this checkout.
//
// Nothing here writes, and nothing here asks for a document to be duplicated
// to make a screen appear. A subsection with no source says so and names where
// it looked; a document is listed where it lives, with the engine's ownership
// answer beside it.
package project

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/covenant"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// Roots names the three directories this package reads from. It is declared
// here, rather than imported from the lifecycle slice, for the reason that
// slice's own comment gives: taking its types closes an import loop.
type Roots struct{ Checkout, Installation, StateRoot string }

// SchemaVersion is the shape of the thread resource the interface reads.
const SchemaVersion = 1

// The three states a subsection is in. Not-recorded is an absence this
// workspace could fill; not-projected is a view this build does not have.
const (
	stateRecorded     = "recorded"
	stateNotRecorded  = "not-recorded"
	stateNotProjected = "not-projected"
)

// Thread is the whole of Project, read once, as it was at readAt.
type Thread struct {
	SchemaVersion int          `json:"schemaVersion"`
	ReadAt        string       `json:"readAt"`
	Subsections   []Subsection `json:"subsections"`
}

// Subsection is one of the master's six. LookedFor is what was looked for and
// is not there, with absolute paths, so "Not yet recorded" is a statement a
// human can act on rather than a shrug.
type Subsection struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	State     string    `json:"state"`
	LookedFor []string  `json:"lookedFor"`
	Covenant  *Covenant `json:"covenant"`
	Purpose   *Purpose  `json:"purpose"`
	Groups    []Group   `json:"groups"`
}

// Group is one named run of documents inside a subsection. An unnamed group
// carries an empty title, and the pane shows its rows without a heading.
type Group struct {
	ID        string  `json:"id"`
	Title     string  `json:"title"`
	Documents []Entry `json:"documents"`
}

// Entry is one document in the catalogue, named the way the resolver names it,
// so the same triple opens it.
type Entry struct {
	Kind       string `json:"kind"`
	ID         string `json:"id"`
	Title      string `json:"title"`
	Owner      string `json:"owner"`
	Bytes      int64  `json:"bytes"`
	ModifiedAt string `json:"modifiedAt"`
	State      string `json:"state"`
	Reason     string `json:"reason"`
}

// Purpose is the one paragraph the adopted project's rules file declares.
type Purpose struct {
	Path string `json:"path"`
	Text string `json:"text"`
}

// Covenant is the app's own declaration, as this interface shows it. A
// covenant that is present but cannot be read carries its path and the
// refusal, which is shown rather than hidden.
type Covenant struct {
	Path         string        `json:"path"`
	Error        string        `json:"error,omitempty"`
	Identity     *Identity     `json:"identity,omitempty"`
	Requirements []Requirement `json:"requirements,omitempty"`
	Battery      *Battery      `json:"battery,omitempty"`
	Budgets      []Budget      `json:"budgets,omitempty"`
	Guards       []Guard       `json:"guards,omitempty"`
	Guardrails   []string      `json:"guardrails,omitempty"`
}

type Identity struct {
	Name        string   `json:"name"`
	EntryPoint  string   `json:"entryPoint"`
	SourcePaths []string `json:"sourcePaths"`
}

type Requirement struct {
	ID    string `json:"id"`
	Ref   string `json:"ref"`
	Proof string `json:"proof"`
}

type Battery struct {
	Command   string `json:"command"`
	Metric    string `json:"metric"`
	Direction string `json:"direction"`
	Threshold string `json:"threshold"`
}

type Budget struct {
	Metric    string  `json:"metric"`
	Bound     float64 `json:"bound"`
	Direction string  `json:"direction"`
}

type Guard struct {
	Name    string  `json:"name"`
	Command string  `json:"command"`
	Cadence int64   `json:"cadence"`
	Floor   float64 `json:"floor"`
}

// ReadThread reads the whole section. It is called per request: a document
// written while the server runs is read without a restart, and "read at" says
// how old what the human is looking at is.
//
// The design names this Thread, which is also the name of what it answers;
// Go allows one of the two, and the type keeps it, because the resource's
// contract is spelled project.Thread wherever it is wired.
func ReadThread(roots Roots, now time.Time) (Thread, error) {
	layout, err := stateroot.ResolveLayout(roots.Installation)
	if err != nil {
		return Thread{}, fmt.Errorf("cannot resolve the layout of the installation at %s: %w", roots.Installation, err)
	}
	root, err := os.OpenRoot(roots.Checkout)
	if err != nil {
		return Thread{}, fmt.Errorf("cannot open the checkout at %s: %w", roots.Checkout, err)
	}
	defer func() { _ = root.Close() }()

	reader := &catalogue{roots: roots, root: root, selfHosted: layout.Template, owners: map[string]string{}}
	thread := Thread{SchemaVersion: SchemaVersion, ReadAt: stamp(now), Subsections: []Subsection{}}
	for _, section := range subsections {
		thread.Subsections = append(thread.Subsections, reader.subsection(section))
	}
	return thread, nil
}

// catalogue carries one request's reading: the anchored root every entry is
// opened through, the mode, and the ownership answers already asked for, so
// the oracle is asked once per catalogue directory and per single file rather
// than once per document.
type catalogue struct {
	roots      Roots
	root       *os.Root
	selfHosted bool
	owners     map[string]string
}

func (c *catalogue) subsection(section sectionRow) Subsection {
	out := Subsection{
		ID:        section.id,
		Title:     section.title,
		State:     stateNotRecorded,
		LookedFor: []string{},
		Groups:    []Group{},
	}
	if section.notProjected {
		out.State = stateNotProjected
		return out
	}
	found := false
	for _, row := range entries {
		if row.subsection != section.id {
			continue
		}
		if row.selfHostedOnly && !c.selfHosted {
			continue
		}
		c.read(row, &out, &found)
	}
	if found && !section.alwaysNotRecorded {
		out.State = stateRecorded
	}
	return out
}

// read applies one catalogue row to the subsection it belongs to.
func (c *catalogue) read(row entryRow, out *Subsection, found *bool) {
	for _, resolved := range c.resolve(row.roots) {
		switch row.kind {
		case kindCovenant:
			c.covenant(resolved, out, found)
		case kindPurpose:
			c.purpose(resolved, row, out, found)
		default:
			c.documents(resolved, row, out, found)
		}
	}
}

// resolve answers the roots one row is read against, in order, with the
// duplicate dropped: the state root and the checkout are one directory in an
// adopted workspace, and one source is not two.
func (c *catalogue) resolve(kinds []rootKind) []resolvedRoot {
	out := []resolvedRoot{}
	seen := map[string]bool{}
	for _, kind := range kinds {
		directory := c.directory(kind)
		if seen[directory] {
			continue
		}
		seen[directory] = true
		prefix, beneath := c.prefix(directory)
		out = append(out, resolvedRoot{directory: directory, prefix: prefix, beneath: beneath})
	}
	return out
}

func (c *catalogue) directory(kind rootKind) string {
	switch kind {
	case rootInstallation:
		return c.roots.Installation
	case rootStateRoot:
		return c.roots.StateRoot
	default:
		return c.roots.Checkout
	}
}

// prefix is a root's checkout-relative location, and whether it lies beneath
// the checkout at all. Both the installation and the state root do in every
// layout the launcher admits; a root that does not is read from no further,
// and the paths it would have held are still named as looked for.
func (c *catalogue) prefix(directory string) (string, bool) {
	relative, err := filepath.Rel(c.roots.Checkout, directory)
	if err != nil {
		return "", false
	}
	slashed := filepath.ToSlash(relative)
	if slashed == ".." || strings.HasPrefix(slashed, "../") {
		return "", false
	}
	if slashed == "." {
		return "", true
	}
	return slashed, true
}

// id is the checkout-relative id of a path beneath one root.
func (r resolvedRoot) id(relative string) string {
	if r.prefix == "" {
		return relative
	}
	return r.prefix + "/" + relative
}

func (r resolvedRoot) absolute(relative string) string {
	return filepath.Join(r.directory, filepath.FromSlash(relative))
}

// documents lists what one row names beneath one root, and records what it
// looked for and did not find: an exact path that is not there, and, for a row
// that names a directory, the directory itself when nothing in it was listed.
func (c *catalogue) documents(root resolvedRoot, row entryRow, out *Subsection, found *bool) {
	group := Group{ID: row.groupID, Title: row.groupTitle, Documents: []Entry{}}
	names := row.files
	directory := ""
	switch row.kind {
	case kindGlob:
		directory = row.dir
	case kindPaper:
		directory = paperDir
	case kindDesigns:
		directory = plansDir
		group.ID = row.groupID + "-" + slug(root.id(plansDir))
		group.Title = row.groupTitle + " in " + root.id(plansDir)
	}
	if !root.beneath {
		// A root outside the checkout is read no further; what it would have
		// held is still named, so the absence has an address.
		if directory != "" {
			out.LookedFor = append(out.LookedFor, root.absolute(directory))
		}
		for _, name := range names {
			out.LookedFor = append(out.LookedFor, root.absolute(name))
		}
		return
	}
	switch row.kind {
	case kindGlob:
		names = c.matches(root, row.dir, row.pattern)
	case kindPaper:
		names = c.paper(root)
	case kindDesigns:
		names = c.designs(root)
	}
	for _, name := range names {
		entry, present := c.entry(root, name)
		if !present {
			out.LookedFor = append(out.LookedFor, root.absolute(name))
			continue
		}
		group.Documents = append(group.Documents, entry)
	}
	if len(group.Documents) == 0 {
		if directory != "" {
			out.LookedFor = append(out.LookedFor, root.absolute(directory))
		}
		return
	}
	*found = true
	out.Groups = appendGroup(out.Groups, group)
}

// entry reads one catalogue file through the same anchored root the document
// route opens: the descriptor decides that it is a regular file, and gives the
// size and the modification time; the head read gives the title.
func (c *catalogue) entry(root resolvedRoot, name string) (Entry, bool) {
	id := root.id(name)
	file, err := c.root.Open(id)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Entry{}, false
		}
		// A name that is there but cannot be opened — a link that leaves the
		// checkout is the case that matters — is listed with its reason rather
		// than silently dropped, because dropping it would claim it is absent.
		return Entry{
			Kind: "document", ID: id, Title: path.Base(id),
			Owner: c.owner(root.absolute(name)), State: stateUnreadable, Reason: err.Error(),
		}, true
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return Entry{}, false
	}
	head, err := io.ReadAll(io.LimitReader(file, titleHead))
	if err != nil {
		return Entry{
			Kind: "document", ID: id, Title: path.Base(id),
			Owner: c.owner(root.absolute(name)), State: stateUnreadable, Reason: err.Error(),
		}, true
	}
	entry := Entry{
		Kind:       "document",
		ID:         id,
		Title:      titleOf(id, head),
		Owner:      c.owner(root.absolute(name)),
		Bytes:      info.Size(),
		ModifiedAt: stamp(info.ModTime()),
		State:      stateReadable,
	}
	switch {
	case info.Size() > maxDocumentBytes:
		// Advisory: this is the size at this moment, and the route decides
		// again from its own read when the document is opened.
		entry.State = stateTooLarge
		entry.Reason = fmt.Sprintf("the file is larger than %s, so it is not read here", humanLimit())
		entry.Title = path.Base(id)
	case !utf8.Valid(trimPartialRune(head)):
		entry.State = stateUnreadable
		entry.Reason = "the file is not valid UTF-8 text"
		entry.Title = path.Base(id)
	}
	return entry, true
}

// matches lists one directory through the anchored root and answers the names
// matching a pattern, in name order.
func (c *catalogue) matches(root resolvedRoot, directory, pattern string) []string {
	names := []string{}
	for _, name := range c.names(root.id(directory)) {
		if matched, err := path.Match(pattern, name); err == nil && matched {
			names = append(names, directory+"/"+name)
		}
	}
	sort.Strings(names)
	return names
}

// names lists the regular-file entries of one directory beneath the checkout.
// A directory that is not there is not an error here: the caller records what
// it looked for.
func (c *catalogue) names(id string) []string {
	file, err := c.root.Open(id)
	if err != nil {
		return nil
	}
	defer func() { _ = file.Close() }()
	listed, err := file.ReadDir(-1)
	if err != nil {
		return nil
	}
	names := []string{}
	for _, entry := range listed {
		if entry.IsDir() {
			continue
		}
		names = append(names, entry.Name())
	}
	return names
}

// subdirectories lists the directory names of one directory beneath the
// checkout, in name order.
func (c *catalogue) subdirectories(id string) []string {
	file, err := c.root.Open(id)
	if err != nil {
		return nil
	}
	defer func() { _ = file.Close() }()
	listed, err := file.ReadDir(-1)
	if err != nil {
		return nil
	}
	names := []string{}
	for _, entry := range listed {
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	return names
}

// paper answers the chapters in reading order: the index first, then the
// numbered chapters by name, which is the order the paper itself is in.
func (c *catalogue) paper(root resolvedRoot) []string {
	names := []string{}
	chapters := []string{}
	for _, name := range c.names(root.id(paperDir)) {
		switch {
		case name == "index.md":
			names = append(names, paperDir+"/"+name)
		case chapterName(name):
			chapters = append(chapters, paperDir+"/"+name)
		}
	}
	sort.Strings(chapters)
	return append(names, chapters...)
}

// chapterName reports the paper's chapter spelling: two digits, a hyphen, and
// a Markdown name.
func chapterName(name string) bool {
	matched, err := path.Match("[0-9][0-9]-*.md", name)
	return err == nil && matched
}

// designs answers the live designs beneath one plans directory: its own
// design documents and those one level down, never the goal records, which
// have their own view.
func (c *catalogue) designs(root resolvedRoot) []string {
	names := c.matches(root, plansDir, "*-design.md")
	for _, directory := range c.subdirectories(root.id(plansDir)) {
		if directory == "goals" || directory == "goals-drafts" {
			continue
		}
		names = append(names, c.matches(root, plansDir+"/"+directory, "*-design.md")...)
	}
	sort.Strings(names)
	return names
}

// covenant reads the app's declaration from the first of its homes that holds
// one. A covenant that cannot be read is reported where it was found.
//
// The probe through the anchored root decides only whether to say "not
// recorded" or to read; the read itself is covenant.Load, which is the one
// home's own safe read: one open with no-follow, the shape judged on the held
// handle, and a symlink refused outright. Its path is a fixed name beneath a
// root the launcher validated, never anything a request named, so the pair is
// a presence question and an answer, not a resolve and a reopen.
func (c *catalogue) covenant(root resolvedRoot, out *Subsection, found *bool) {
	if out.Covenant != nil {
		return
	}
	absolute := root.absolute(covenant.Filename)
	if !root.beneath || !c.regularFile(root.id(covenant.Filename)) {
		out.LookedFor = append(out.LookedFor, absolute)
		return
	}
	loaded, err := covenant.Load(absolute)
	if err != nil {
		out.Covenant = &Covenant{Path: absolute, Error: err.Error()}
		*found = true
		return
	}
	out.Covenant = describeCovenant(absolute, loaded)
	*found = true
}

// regularFile reports whether one name beneath the checkout is a regular file,
// judged on the descriptor the anchored open gives.
func (c *catalogue) regularFile(id string) bool {
	file, err := c.root.Open(id)
	if err != nil {
		return false
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	return err == nil && info.Mode().IsRegular()
}

// purpose reads the one paragraph the rules file declares. The template's own
// placeholder is not a purpose, and neither is an absent line.
func (c *catalogue) purpose(root resolvedRoot, row entryRow, out *Subsection, found *bool) {
	if out.Purpose != nil || len(row.files) == 0 {
		return
	}
	name := row.files[0]
	absolute := root.absolute(name)
	text := ""
	if root.beneath {
		text = c.purposeLine(root.id(name))
	}
	if text == "" {
		out.LookedFor = append(out.LookedFor, absolute)
		return
	}
	out.Purpose = &Purpose{Path: absolute, Text: text}
	*found = true
}

func (c *catalogue) purposeLine(id string) string {
	file, err := c.root.Open(id)
	if err != nil {
		return ""
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(io.LimitReader(file, maxDocumentBytes+1))
	if err != nil || len(data) > maxDocumentBytes || !utf8.Valid(data) {
		return ""
	}
	const marker = "- Purpose:"
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSuffix(line, "\r")
		if !strings.HasPrefix(line, marker) {
			continue
		}
		text := strings.TrimSpace(strings.TrimPrefix(line, marker))
		text = strings.Trim(text, "`")
		if text == "" || text == "<one paragraph>" {
			return ""
		}
		return text
	}
	return ""
}

// owner asks the ownership oracle once per path and remembers the answer, so
// one thread request runs a handful of subprocesses rather than one per
// document. It is metadata: no answer here selects or reads any bytes.
func (c *catalogue) owner(absolute string) string {
	if answer, asked := c.owners[absolute]; asked {
		return answer
	}
	answer, _ := ownerOf(c.roots.Installation, absolute)
	c.owners[absolute] = answer
	return answer
}

func describeCovenant(path string, loaded *covenant.Covenant) *Covenant {
	described := &Covenant{
		Path: path,
		Identity: &Identity{
			Name:        loaded.Identity.Name,
			EntryPoint:  loaded.Identity.EntryPoint,
			SourcePaths: loaded.Identity.SourcePaths,
		},
		Requirements: []Requirement{},
		Battery: &Battery{
			Command:   loaded.Battery.Command,
			Metric:    loaded.Battery.Metric,
			Direction: loaded.Battery.Direction,
			Threshold: loaded.Battery.Threshold,
		},
		Budgets:    []Budget{},
		Guards:     []Guard{},
		Guardrails: loaded.Guardrails,
	}
	for _, requirement := range loaded.Requirements {
		described.Requirements = append(described.Requirements,
			Requirement{ID: requirement.ID, Ref: requirement.Ref, Proof: requirement.Proof})
	}
	for _, budget := range loaded.Budgets {
		described.Budgets = append(described.Budgets,
			Budget{Metric: budget.Metric, Bound: budget.Bound, Direction: budget.Direction})
	}
	for _, guard := range loaded.Guards {
		described.Guards = append(described.Guards,
			Guard{Name: guard.Name, Command: guard.Command, Cadence: guard.Cadence, Floor: guard.Floor})
	}
	if described.Guardrails == nil {
		described.Guardrails = []string{}
	}
	return described
}

// appendGroup adds a group's documents to the group of the same id when one is
// already there, so two roots that answer one named group read as one run.
func appendGroup(groups []Group, group Group) []Group {
	for index := range groups {
		if groups[index].ID == group.ID {
			groups[index].Documents = append(groups[index].Documents, group.Documents...)
			return groups
		}
	}
	return append(groups, group)
}

// trimPartialRune drops the incomplete rune a fixed-length head read can end
// with, so a long document is not called unreadable for being cut mid-letter.
func trimPartialRune(head []byte) []byte {
	for cut := 0; cut < 4 && cut < len(head); cut++ {
		if utf8.Valid(head[:len(head)-cut]) {
			return head[:len(head)-cut]
		}
	}
	return head
}

// slug names a group after the directory it was read from, in the same
// spelling an element id can carry.
func slug(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, "/", "-"), ".", "-")
}
