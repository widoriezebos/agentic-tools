package main

// metasystem project type-designs: the reviewed mapping that gives the kit's
// own history the head a design is supposed to carry.
//
// The machine proposes and a human disposes. Without --apply the verb writes
// one plan — a table with a line per historical file — and changes nothing
// else. The proposal comes from two facts and no inference: the file name,
// which by the old convention is the goal's id followed by -design.md, and
// what the ledger says about that goal. A file the two do not agree about is
// left unresolved with the reason written beside it, because a guessed scope
// or a guessed status is worse than none: the pane would report a design as
// shipped, or as the whole project's, on the strength of a file name.
//
// With --apply the verb reads the plan back, honours whatever a human wrote in
// it, validates every head and every fresh id against the project's whole
// namespace BEFORE it writes a byte, and then types the resolved files one at
// a time. It never moves a file, never touches either goal-ledger directory,
// and never writes outside the historical home; a second --apply has nothing
// left to do, because every typed file is then a record.

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/project"
)

func runProjectTypeDesigns(args []string) int {
	return projectTypeDesigns(args, os.Stdout, os.Stderr)
}

// typingPlanName is the plan's file name, in the historical home beside the
// files it is about. It does not end in -design.md, so the home does not read
// its own plan as one of the designs it types.
const typingPlanName = "designs-typing.md"

// The four words the resolution column carries. Only resolved is applied;
// record and malformed are files --apply has nothing to do to, and unresolved
// is the human's list.
const (
	typingResolved   = "resolved"
	typingUnresolved = "unresolved"
	typingRecord     = "record"
	typingMalformed  = "malformed"
)

// typingOpeningRunes is how much of a file's own opening is read for a goal it
// names or a status word that contradicts the proposal. A design that
// disagrees with its file name says so where it introduces itself; reading
// further would turn every mention of a neighbouring goal in a body into a
// refusal to type the file at all.
const typingOpeningRunes = 400

// typingSuperseded is the word a concluded goal's own Concluded line uses when
// something else took the work over. The ledger has one state for both
// endings, so this line is where a goal that shipped and a goal that was
// replaced are told apart.
var typingSuperseded = regexp.MustCompile(`(?i)\bsupersed`)

// typingLine is one row of the plan: the file, what the machine proposes for
// it, where it stands, and why it stands there.
type typingLine struct {
	File       string
	Goal       string
	Status     string
	Resolution string
	Note       string
}

// typingFile is one file of the historical home, with the bytes it holds and
// the mode it holds them under.
type typingFile struct {
	Name string
	Text string
	Mode os.FileMode
}

func projectTypeDesigns(args []string, stdout, stderr io.Writer) int {
	flags := projectFlags("type-designs", stderr)
	root := pathFlag(flags, "root", ".", "a path at or below the metasystem installation")
	apply := flags.Bool("apply", false, "type the plan's resolved files instead of proposing")
	if flags.Parse(args) != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: metasystem project type-designs [--root DIR] [--apply]")
		return 2
	}
	return typeDesigns(*root, *apply, project.NewID, stdout, stderr)
}

// typeDesigns is the verb with its identity source supplied, so a test can
// prove what happens when a minted id collides with one the project already
// declares — which is the one failure that must leave every file untouched.
func typeDesigns(root string, apply bool, mint func() (string, error), stdout, stderr io.Writer) int {
	read, code := projectRead(root, stderr)
	if read == nil {
		return code
	}
	home, found := typingHome(read)
	if !found {
		fmt.Fprintln(stderr, "metasystem project type-designs: this installation has no historical design home;"+
			" only the self-hosted layout carries the kit's own past")
		return 1
	}
	files, err := typingRead(home)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if apply {
		return typingApply(read, home, files, mint, stdout, stderr)
	}
	lines := typingProposal(read, home, files)
	plan := typingPlanText(home, lines)
	if err := typingWritePlan(read, home, plan); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprint(stdout, plan)
	fmt.Fprintf(stderr, "wrote %s: %s\n", typingPlanRel(home), typingTally(lines))
	return 0
}

// typingHome is the historical home, which only the self-hosted layout has.
func typingHome(read *project.Project) (project.Home, bool) {
	for _, home := range read.Homes {
		if home.Glob != "" {
			return home, true
		}
	}
	return project.Home{}, false
}

// typingRead reads the home's own files, in name order, from inside the home
// and nowhere else: a name is listed and opened through the home's root, so a
// symlink whose target lies outside it does not open and no subdirectory is
// descended.
func typingRead(home project.Home) ([]typingFile, error) {
	root, err := os.OpenRoot(home.Path)
	if err != nil {
		return nil, fmt.Errorf("metasystem project type-designs: open %s: %w", home.Rel, err)
	}
	defer func() { _ = root.Close() }()
	entries, err := os.ReadDir(home.Path)
	if err != nil {
		return nil, fmt.Errorf("metasystem project type-designs: read %s: %w", home.Rel, err)
	}
	var files []typingFile
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		matched, matchErr := filepath.Match(home.Glob, entry.Name())
		if matchErr != nil || !matched {
			continue
		}
		file, err := root.Open(entry.Name())
		if err != nil {
			return nil, fmt.Errorf("metasystem project type-designs: open %s/%s: %w", home.Rel, entry.Name(), err)
		}
		info, statErr := file.Stat()
		data, readErr := io.ReadAll(file)
		_ = file.Close()
		if statErr != nil {
			return nil, fmt.Errorf("metasystem project type-designs: stat %s/%s: %w", home.Rel, entry.Name(), statErr)
		}
		if !info.Mode().IsRegular() {
			continue
		}
		if readErr != nil {
			return nil, fmt.Errorf("metasystem project type-designs: read %s/%s: %w", home.Rel, entry.Name(), readErr)
		}
		files = append(files, typingFile{Name: entry.Name(), Text: string(data), Mode: info.Mode().Perm()})
	}
	sort.SliceStable(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	return files, nil
}

// typingProposal is one line per historical file, in name order.
func typingProposal(read *project.Project, home project.Home, files []typingFile) []typingLine {
	lines := make([]typingLine, 0, len(files))
	for _, file := range files {
		lines = append(lines, typingPropose(read, home, file))
	}
	return lines
}

// typingPropose reads one file. A file that already declares a Kind has a head
// already: it is a record, or it is a malformed declaration, and either way
// the pass leaves it alone — retyping a record would remint an identity, and
// repairing a broken head is a judgement no file name supports. Everything
// else is history the pass may type.
func typingPropose(read *project.Project, home project.Home, file typingFile) typingLine {
	rel := home.Rel + "/" + file.Name
	record, problems, isRecord := project.ParseRecord(rel, file.Text)
	if isRecord && record.Declares("Kind") {
		if len(problems) == 0 {
			return typingLine{
				File: file.Name, Goal: strings.Join(record.Goals, " "), Status: record.Status,
				Resolution: typingRecord, Note: "already declares " + record.ID,
			}
		}
		return typingLine{File: file.Name, Resolution: typingMalformed, Note: problems[0].Message}
	}

	line := typingLine{File: file.Name, Resolution: typingUnresolved}
	named := read.Goal(strings.TrimSuffix(strings.TrimSuffix(file.Name, ".md"), "-design"))
	if named == nil {
		line.Note = "no goal matched"
		return line
	}
	line.Goal = named.ID
	line.Status = typingStatus(*named)
	opening := typingOpening(file.Text)
	if other := typingNamesAnother(read, opening, named.ID); other != "" {
		line.Note = "the file names goal " + other
		return line
	}
	if word := typingContradiction(opening, line.Status); word != "" {
		line.Note = "the file says " + word
		return line
	}
	line.Resolution = typingResolved
	return line
}

// typingStatus is what the ledger says about the goal, and nothing more. A
// live goal's design is accepted — the work is still ahead of it. A concluded
// goal's is done only when the goal shipped: a goal that was abandoned, and a
// goal whose own conclusion says something else superseded it, leave behind a
// design that was never built, and calling that done would tell every reader
// of the pane that it was.
func typingStatus(one project.Goal) string {
	switch {
	case one.Live():
		return project.StatusAccepted
	case one.State == goal.StateAbandoned, typingSuperseded.MatchString(one.Concluded):
		return project.StatusSuperseded
	default:
		return project.StatusDone
	}
}

// typingOpening is the file's own first lines: its title, and the first
// characters of whatever follows. Characters are runes, so a body that opens
// with an em dash is cut where a reader would cut it.
func typingOpening(text string) string {
	first := text
	if end := strings.IndexByte(text, '\n'); end >= 0 {
		first = text[:end]
	}
	runes := []rune(text)
	if len(runes) > typingOpeningRunes {
		runes = runes[:typingOpeningRunes]
	}
	return first + "\n" + string(runes)
}

// typingNamesAnother is the ledger goal this opening names that is not the one
// the file name proposes, or nothing. The id has to stand on its own — a goal
// whose id is a prefix of a longer one is not named by a mention of the longer
// one — and the answer is the first in id order, so two mentions in one
// opening always produce the same note.
func typingNamesAnother(read *project.Project, opening, proposed string) string {
	var named []string
	for _, one := range read.Goals {
		if one.ID == proposed {
			continue
		}
		if typingWholeWord(opening, one.ID) {
			named = append(named, one.ID)
		}
	}
	if len(named) == 0 {
		return ""
	}
	sort.Strings(named)
	return named[0]
}

// typingContradiction is the status word this opening carries that is not the
// proposed one, or nothing. The four words of the grammar are the whole of
// what is looked for: a file that introduces itself as superseded contradicts
// a proposal of accepted, whatever the ledger says about the goal it is named
// for.
func typingContradiction(opening, proposed string) string {
	for _, status := range project.Statuses {
		if status == proposed {
			continue
		}
		if typingWholeWord(opening, status) {
			return status
		}
	}
	return ""
}

// typingWholeWord reports whether the text names this word on its own, with
// neither a letter, a digit nor a hyphen against either end, so that a goal id
// is not found inside a longer id and "done" is not found inside "abandoned".
// The comparison folds case, because a title writes Superseded and a body
// writes superseded.
func typingWholeWord(text, word string) bool {
	lowered, wanted := strings.ToLower(text), strings.ToLower(word)
	for at := 0; at < len(lowered); {
		found := strings.Index(lowered[at:], wanted)
		if found < 0 {
			return false
		}
		start := at + found
		end := start + len(wanted)
		if !typingWordByte(lowered, start-1) && !typingWordByte(lowered, end) {
			return true
		}
		at = start + 1
	}
	return false
}

func typingWordByte(text string, at int) bool {
	if at < 0 || at >= len(text) {
		return false
	}
	c := text[at]
	return c == '-' || c == '_' || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')
}

// typingApply types every resolved line of the reviewed plan.
func typingApply(read *project.Project, home project.Home, files []typingFile,
	mint func() (string, error), stdout, stderr io.Writer) int {
	planPath := filepath.Join(home.Path, typingPlanName)
	data, err := os.ReadFile(planPath)
	if err != nil {
		fmt.Fprintf(stderr, "metasystem project type-designs: no reviewed plan at %s:"+
			" run without --apply first, then review it\n", typingPlanRel(home))
		return 1
	}
	reviewed := typingParsePlan(string(data))
	held := map[string]typingFile{}
	for _, file := range files {
		held[file.Name] = file
	}

	// Every head is composed and every id is minted before one byte is
	// written, so a collision or a refused head costs nothing: the files are
	// exactly as they were.
	taken := typingNamespace(read)
	var writes []typingFile
	var typed []typingLine
	var refusals []string
	for _, line := range reviewed {
		if line.Resolution != typingResolved {
			continue
		}
		file, present := held[line.File]
		if !present {
			refusals = append(refusals, line.File+": the historical home holds no such file")
			continue
		}
		record, problems, isRecord := project.ParseRecord(home.Rel+"/"+file.Name, file.Text)
		if isRecord && record.Declares("Kind") {
			if len(problems) == 0 {
				continue // already typed; a repeat --apply has nothing to do
			}
			refusals = append(refusals, file.Name+": "+problems[0].Message)
			continue
		}
		if !oneOf(project.Statuses, line.Status) {
			refusals = append(refusals, file.Name+": the status "+typingQuoted(line.Status)+
				" is not one of "+strings.Join(project.Statuses, ", "))
			continue
		}
		if line.Goal != "" && !read.HasGoal(line.Goal) {
			refusals = append(refusals, file.Name+": the goal "+line.Goal+" is not in the ledger")
			continue
		}
		id, err := mint()
		if err != nil {
			refusals = append(refusals, file.Name+": "+err.Error())
			continue
		}
		if where, declared := taken[id]; declared {
			refusals = append(refusals, file.Name+": the id "+id+" is already declared by "+where)
			continue
		}
		taken[id] = home.Rel + "/" + file.Name
		typedText := typingCompose(file.Name, file.Text, id, line.Status, line.Goal)
		if refusal := typingRefuseHead(home.Rel+"/"+file.Name, typedText, id, line); refusal != "" {
			refusals = append(refusals, refusal)
			continue
		}
		writes = append(writes, typingFile{Name: file.Name, Text: typedText, Mode: file.Mode})
		typed = append(typed, typingLine{File: file.Name, Goal: line.Goal, Status: line.Status, Note: id})
	}
	if len(refusals) > 0 {
		fmt.Fprintln(stderr, "metasystem project type-designs: nothing was written:")
		for _, refusal := range refusals {
			fmt.Fprintln(stderr, "  "+refusal)
		}
		return 1
	}

	for _, write := range writes {
		if err := typingWriteFile(read, home, write); err != nil {
			fmt.Fprintln(stderr, err)
			fmt.Fprintf(stderr, "metasystem project type-designs: %d file(s) were already typed;"+
				" run without --apply to see what is left\n", len(typed))
			return 1
		}
	}

	// The plan is rewritten from what is now on disk, so the typed files stand
	// in it as the records they have become and a repeat --apply is a no-op.
	after, code := projectRead(read.Roots.Installation, stderr)
	if after == nil {
		return code
	}
	afterHome, found := typingHome(after)
	if !found {
		fmt.Fprintln(stderr, "metasystem project type-designs: the historical home is gone")
		return 1
	}
	afterFiles, err := typingRead(afterHome)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	lines := typingProposal(after, afterHome, afterFiles)
	if err := typingWritePlan(after, afterHome, typingPlanText(afterHome, lines)); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	fmt.Fprintf(stdout, "typed %d file(s) in %s:\n", len(typed), home.Rel)
	for _, one := range typed {
		fmt.Fprintf(stdout, "  %s\t%s\t%s\t%s\n", one.File, typingGoalCell(one.Goal), one.Status, one.Note)
	}
	unresolved := typingUnresolvedOf(lines)
	fmt.Fprintf(stdout, "unresolved %d file(s), for a human to read:\n", len(unresolved))
	for _, one := range unresolved {
		fmt.Fprintf(stdout, "  %s\t%s\n", one.File, one.Note)
	}
	return 0
}

// typingRefuseHead puts the bytes about to be written through the very parser
// that will read them back, and refuses before writing rather than leaving a
// refused record on disk.
func typingRefuseHead(rel, text, id string, line typingLine) string {
	record, problems, isRecord := project.ParseRecord(rel, text)
	if !isRecord {
		return line.File + ": the typed file would not parse as a record"
	}
	for _, problem := range problems {
		return line.File + ": " + problem.Message
	}
	switch {
	case record.Kind != project.KindDesign:
		return line.File + ": the typed head declares " + typingQuoted(record.Kind) + ", not a design"
	case record.ID != id:
		return line.File + ": the typed head declares another id"
	case record.Status != line.Status:
		return line.File + ": the typed head declares another status"
	case line.Goal == "" && len(record.Goals) != 0:
		return line.File + ": the typed head names a goal the plan does not"
	case line.Goal != "" && (len(record.Goals) != 1 || record.Goals[0] != line.Goal):
		return line.File + ": the typed head does not name " + line.Goal
	}
	return ""
}

// typingNamespace is every id the project declares, pages and register rows
// alike, against what declared it. An id is unique across the whole project,
// so a fresh one is judged against all of it and not against the designs.
func typingNamespace(read *project.Project) map[string]string {
	taken := map[string]string{}
	for _, record := range read.Records {
		if record.ID != "" {
			taken[record.ID] = record.Path
		}
	}
	for _, question := range read.Questions {
		if question.ID != "" {
			taken[question.ID] = "the question register"
		}
	}
	return taken
}

// typingCompose is the bytes one historical file becomes: its title, a blank
// line, the head, a blank line, and then everything it already held, byte for
// byte.
//
// A file whose first line is not a title gets one from its name and keeps its
// own first line as the body's first, because a document that opens with a
// status paragraph has said something its author meant and a pass that dropped
// it would be editing rather than typing. Legacy bullets are body too: they
// sit after the head's blank line, where the parser reads them as the prose
// they always were.
//
// The blank line after the head is written only when the body does not already
// open with one, so no file gains a blank line it did not have; nothing the
// file held is removed either way.
func typingCompose(name, text, id, status, goalID string) string {
	title, body := typingTitle(name, text)
	head := []string{
		"- Kind: " + project.KindDesign,
		"- Id: " + id,
		"- Status: " + status,
	}
	if goalID != "" {
		head = append(head, "- Goals: "+goalID)
	}
	typed := title + "\n\n" + strings.Join(head, "\n") + "\n"
	switch {
	case body == "":
		return typed
	case strings.HasPrefix(body, "\n"), strings.HasPrefix(body, "\r\n"):
		return typed + body
	default:
		return typed + "\n" + body
	}
}

// typingTitle is the file's own title line and what follows it, or a title
// read from the file name and the whole file as the body. The test for a title
// is the parser's own, so a file this answers a synthesized title for is
// exactly a file the parser would have refused to read a title from.
func typingTitle(name, text string) (string, string) {
	first, rest := text, ""
	if end := strings.IndexByte(text, '\n'); end >= 0 {
		first, rest = text[:end], text[end+1:]
	}
	if strings.HasPrefix(first, "# ") && strings.TrimSpace(strings.TrimPrefix(first, "#")) != "" {
		return strings.TrimRight(first, "\r"), rest
	}
	return "# " + typingTitleFromName(name), text
}

// typingTitleFromName reads a title out of the file name: its words, capitalised,
// which is the whole of what the name carries.
func typingTitleFromName(name string) string {
	words := strings.Split(strings.TrimSuffix(name, ".md"), "-")
	for index, word := range words {
		if word == "" {
			continue
		}
		words[index] = strings.ToUpper(word[:1]) + word[1:]
	}
	return strings.Join(strings.Fields(strings.Join(words, " ")), " ")
}

// typingWriteFile publishes one typed file in place: atomically, so no reader
// and no interrupted run ever sees half a document, and under the mode it
// already had, so a pass over a hundred tracked files changes their content
// and nothing else about them.
func typingWriteFile(read *project.Project, home project.Home, file typingFile) error {
	if err := typingBareName(file.Name); err != nil {
		return err
	}
	path := filepath.Join(home.Path, file.Name)
	if _, err := atomicfile.WriteText(path, file.Text, read.Roots.Checkout); err != nil {
		return fmt.Errorf("metasystem project type-designs: write %s/%s: %w", home.Rel, file.Name, err)
	}
	if err := os.Chmod(path, file.Mode); err != nil {
		return fmt.Errorf("metasystem project type-designs: restore the mode of %s/%s: %w", home.Rel, file.Name, err)
	}
	return nil
}

func typingWritePlan(read *project.Project, home project.Home, plan string) error {
	path := filepath.Join(home.Path, typingPlanName)
	if _, err := atomicfile.WriteText(path, plan, read.Roots.Checkout); err != nil {
		return fmt.Errorf("metasystem project type-designs: write %s: %w", typingPlanRel(home), err)
	}
	return nil
}

// typingBareName is the whole of the write set's boundary: a name of the home
// itself, with no directory in it and no step out of it, so the pass can reach
// neither plans/goals, nor records/goals, nor anything else the ledger owns.
func typingBareName(name string) error {
	if name == "" || name == "." || name == ".." ||
		strings.ContainsAny(name, `/\`) || filepath.IsAbs(name) {
		return fmt.Errorf("metasystem project type-designs: %q is not a file of the historical home", name)
	}
	return nil
}

func typingPlanRel(home project.Home) string {

	return home.Rel + "/" + typingPlanName
}

// typingPlanText renders the reviewed mapping. It is derived from the files
// and the ledger and carries no instant, so running the pass twice over an
// unchanged checkout writes the same bytes twice.
func typingPlanText(home project.Home, lines []typingLine) string {
	var out strings.Builder
	out.WriteString("# Typing the historical designs\n\n")
	out.WriteString("The machine's proposal for the kit's own history in `" + home.Rel + "`, for a human to\n")
	out.WriteString("review. `metasystem project type-designs` rewrites this table from the files and the\n")
	out.WriteString("ledger; `--apply` reads it back and types every line that says `resolved`.\n\n")
	out.WriteString("The goal is proposed from the file name, the status from what the ledger says about\n")
	out.WriteString("that goal. A line is `resolved` only when the file's own opening agrees with both;\n")
	out.WriteString("otherwise it is `unresolved` and the note says why. Edit a line's goal or status, or\n")
	out.WriteString("change `unresolved` to `resolved`, and `--apply` honours what you wrote. A line that\n")
	out.WriteString("says `record` is already typed and a line that says `malformed` carries a head no\n")
	out.WriteString("file name can repair; `--apply` leaves both alone.\n\n")
	out.WriteString("| file | proposed goal | proposed status | resolution | note |\n")
	out.WriteString("| --- | --- | --- | --- | --- |\n")
	for _, line := range lines {
		fmt.Fprintf(&out, "| %s | %s | %s | %s | %s |\n",
			typingCell(line.File), typingCell(line.Goal), typingCell(line.Status),
			typingCell(line.Resolution), typingCell(line.Note))
	}
	return out.String()
}

// typingCell writes one value into a Markdown table: on one line, and with a
// pipe escaped so that a refusal quoting a line of someone's file cannot add a
// column to the row that reports it.
func typingCell(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	return strings.ReplaceAll(value, "|", `\|`)
}

// typingParsePlan reads the reviewed table back. Rows before the header are
// not rows and prose after the table ends it, which is how the register's own
// reader reads a table; a row with fewer cells than the header has said
// nothing in the ones it left out.
func typingParsePlan(text string) []typingLine {
	var lines []typingLine
	started := false
	for _, raw := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(raw)
		if !strings.HasPrefix(trimmed, "|") {
			if started {
				break
			}
			continue
		}
		cells := typingCells(trimmed)
		if !started {
			started = len(cells) > 0 && strings.EqualFold(cells[0], "file")
			continue
		}
		if typingDelimiter(cells) {
			continue
		}
		lines = append(lines, typingLine{
			File:       typingAt(cells, 0),
			Goal:       typingAt(cells, 1),
			Status:     typingAt(cells, 2),
			Resolution: typingAt(cells, 3),
			Note:       typingAt(cells, 4),
		})
	}
	return lines
}

// typingCells splits one row on its unescaped pipes and gives each cell back
// the pipes typingCell escaped.
func typingCells(line string) []string {
	trimmed := strings.TrimSuffix(strings.TrimPrefix(line, "|"), "|")
	var cells []string
	var cell strings.Builder
	for index := 0; index < len(trimmed); index++ {
		switch {
		case trimmed[index] == '\\' && index+1 < len(trimmed) && trimmed[index+1] == '|':
			cell.WriteByte('|')
			index++
		case trimmed[index] == '|':
			cells = append(cells, strings.TrimSpace(cell.String()))
			cell.Reset()
		default:
			cell.WriteByte(trimmed[index])
		}
	}
	return append(cells, strings.TrimSpace(cell.String()))
}

func typingDelimiter(cells []string) bool {
	for _, value := range cells {
		if strings.Trim(value, "-: ") != "" || value == "" {
			return false
		}
	}
	return len(cells) > 0
}

func typingAt(cells []string, index int) string {
	if index >= len(cells) {
		return ""
	}
	return cells[index]
}

func typingUnresolvedOf(lines []typingLine) []typingLine {
	var collected []typingLine
	for _, line := range lines {
		if line.Resolution == typingUnresolved {
			collected = append(collected, line)
		}
	}
	return collected
}

// typingTally is the one line that says what the plan proposes, so a run that
// printed a hundred rows still ends with a number a human can act on.
func typingTally(lines []typingLine) string {
	counts := map[string]int{}
	for _, line := range lines {
		counts[line.Resolution]++
	}
	parts := make([]string, 0, 4)
	for _, resolution := range []string{typingResolved, typingUnresolved, typingRecord, typingMalformed} {
		parts = append(parts, fmt.Sprintf("%d %s", counts[resolution], resolution))
	}
	return strings.Join(parts, ", ")
}

// typingGoalCell is how a typed line names the scope it was given: the goal,
// or the word the project uses for what belongs to the whole of it.
func typingGoalCell(goalID string) string {
	if goalID == "" {
		return project.WholeProject
	}
	return goalID
}

func typingQuoted(value string) string { return `"` + value + `"` }
