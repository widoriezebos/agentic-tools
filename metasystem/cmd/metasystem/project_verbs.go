package main

// The project family: a read-only window on the project's own memory — the
// intent, the doctrine, the decisions and the designs that declare themselves
// in their heads, each in its home, and the open questions the register holds
// as rows. Nothing here writes, and nothing here decides: the one reader
// answers, and these verbs print what it answered.
//
// Each verb is a thin wrapper over a function that takes its streams, so the
// tests drive the same decisions the binary does without a process's one
// standard output between them.

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/project"
)

func runProjectID(args []string) int    { return projectID(args, os.Stdout, os.Stderr) }
func runProjectList(args []string) int  { return projectList(args, os.Stdout, os.Stderr) }
func runProjectShow(args []string) int  { return projectShow(args, os.Stdout, os.Stderr) }
func runProjectTree(args []string) int  { return projectTree(args, os.Stdout, os.Stderr) }
func runProjectCheck(args []string) int { return projectCheck(args, os.Stdout, os.Stderr) }

func runProjectDesignOf(args []string) int { return projectDesignOf(args, os.Stdout, os.Stderr) }

// projectID prints a fresh identity for a record. An id is any string the
// project keeps unique and never changes; this is for those who would rather
// not invent one.
func projectID(args []string, stdout, stderr io.Writer) int {
	flags := projectFlags("id", stderr)
	if flags.Parse(args) != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: metasystem project id")
		return 2
	}
	id, err := project.NewID()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, id)
	return 0
}

// projectList prints one line per record of one kind: the declared facts, then
// the title, then the path, tab separated, because only the title may carry a
// space.
func projectList(args []string, stdout, stderr io.Writer) int {
	flags := projectFlags("list", stderr)
	root := pathFlag(flags, "root", ".", "a path at or below the metasystem installation")
	goal := flags.String("goal", "", "only records about this ledger goal")
	status := flags.String("status", "", "only records with this status")
	kind, ok := projectSubject(flags, args)
	if !ok {
		fmt.Fprintln(stderr, "usage: metasystem project list KIND [--root DIR] [--goal ID] [--status STATUS]")
		return 2
	}
	if !oneOf(project.QueryKinds, kind) {
		fmt.Fprintf(stderr, "metasystem project list: %q is not one of %s\n", kind, strings.Join(project.QueryKinds, ", "))
		return 2
	}
	if *status != "" && !oneOf(project.QueryStatuses, *status) {
		fmt.Fprintf(stderr, "metasystem project list: %q is not one of %s\n", *status, strings.Join(project.QueryStatuses, ", "))
		return 2
	}
	read, code := projectRead(*root, stderr)
	if read == nil {
		return code
	}
	for _, record := range read.List(kind, project.ListOptions{Goal: *goal, Status: *status}) {
		fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\t%s\t%s\n",
			record.Kind, record.ID, record.Status, strings.Join(record.Goals, " "), record.Title, record.Path)
	}
	return 0
}

// projectShow prints one record: what it declares, which goals it is about,
// where it lives, what it names, and what names it — the half of a reference
// the record cannot declare itself.
func projectShow(args []string, stdout, stderr io.Writer) int {
	flags := projectFlags("show", stderr)
	root := pathFlag(flags, "root", ".", "a path at or below the metasystem installation")
	id, ok := projectSubject(flags, args)
	if !ok {
		fmt.Fprintln(stderr, "usage: metasystem project show ID [--root DIR]")
		return 2
	}
	read, code := projectRead(*root, stderr)
	if read == nil {
		return code
	}
	record := read.Record(id)
	if record == nil {
		fmt.Fprintf(stderr, "metasystem project show: no record declares the id %s\n", id)
		return 1
	}
	fmt.Fprintln(stdout, record.Title)
	fmt.Fprintln(stdout, "kind: "+record.Kind)
	fmt.Fprintln(stdout, "id: "+record.ID)
	fmt.Fprintln(stdout, "status: "+record.Status)
	fmt.Fprintln(stdout, "goals: "+strings.Join(record.Goals, " "))
	fmt.Fprintln(stdout, "path: "+record.Path)
	fmt.Fprintln(stdout, "home: "+record.Home)
	for _, key := range []string{"Cites", "Affects", "Governs", "By", "Supersedes", "Answers"} {
		if references := record.References(key); len(references) > 0 {
			fmt.Fprintln(stdout, strings.ToLower(key)+": "+strings.Join(references, " "))
		}
	}
	for _, reference := range read.ReferencedBy(id) {
		fmt.Fprintf(stdout, "referenced by: %s %s %s\n",
			reference.ID, strings.ToLower(reference.Key), reference.Path)
	}
	return 0
}

// projectTree prints the ledger's goals, each with what the project holds
// about it, and the project-wide bucket last: what belongs to the whole rather
// than to one goal.
func projectTree(args []string, stdout, stderr io.Writer) int {
	flags := projectFlags("tree", stderr)
	root := pathFlag(flags, "root", ".", "a path at or below the metasystem installation")
	if flags.Parse(args) != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: metasystem project tree [--root DIR]")
		return 2
	}
	read, code := projectRead(*root, stderr)
	if read == nil {
		return code
	}
	goals, whole := read.Tree()
	for _, one := range goals {
		fmt.Fprintln(stdout, projectGoalHeading(one.Goal))
		projectCounts(stdout, one.Counts)
	}
	fmt.Fprintln(stdout, project.WholeProject)
	projectCounts(stdout, whole)
	return 0
}

// projectIntentWidth is how much of a goal's intent one line of the tree
// carries. An intent is a paragraph in this ledger; the tree is a shape, and a
// shape that wrapped over five lines per goal would not be one.
const projectIntentWidth = 100

// projectGoalHeading is one goal's line: its id, where it stands, and why it
// is open, in that order and separated the way the indexes separate a name
// from what reads it.
func projectGoalHeading(one project.Goal) string {
	return one.ID + " — " + one.State + " — " + projectTruncate(one.Intent, projectIntentWidth)
}

// projectTruncate keeps at most width characters, the ellipsis included, so
// the line is bounded whatever the ledger wrote. Characters are counted as
// runes: a truncation that split one would print a byte that is not a letter.
func projectTruncate(value string, width int) string {
	runes := []rune(value)
	if len(runes) <= width {
		return value
	}
	return string(runes[:width-1]) + "…"
}

func projectCounts(stdout io.Writer, counts project.Counts) {
	fmt.Fprintln(stdout, "  kinds: "+projectTally(project.QueryKinds, counts.Kind))
	fmt.Fprintln(stdout, "  status: "+projectTally(project.QueryStatuses, counts.Status))
}

func projectTally(order []string, counts map[string]int) string {
	parts := make([]string, 0, len(order))
	for _, name := range order {
		parts = append(parts, fmt.Sprintf("%s %d", name, counts[name]))
	}
	return strings.Join(parts, ", ")
}

// projectCheck prints every refusal the homes carry, anchored at the file and
// line a human can act on, and refuses if there is one. A clean project gets
// one line saying what was read, so silence never has to mean success.
func projectCheck(args []string, stdout, stderr io.Writer) int {
	flags := projectFlags("check", stderr)
	root := pathFlag(flags, "root", ".", "a path at or below the metasystem installation")
	if flags.Parse(args) != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: metasystem project check [--root DIR]")
		return 2
	}
	read, code := projectRead(*root, stderr)
	if read == nil {
		return code
	}
	for _, problem := range read.Problems {
		fmt.Fprintln(stdout, problem.String())
	}
	if len(read.Problems) > 0 {
		return 1
	}
	fmt.Fprintf(stdout, "project check passed: %d record(s) in %d home(s), %d question(s), %d ledger goal(s)\n",
		len(read.Records), len(read.Homes), len(read.Questions), len(read.Goals))
	return 0
}

// designOfRefusal is what a goal with no design record is told, and it names
// the section that says what a design record is rather than repeating it.
const designOfRefusal = "no design record names goal %s: a design is a record, see docs/design/design-obligation-gate.md\n"

// designOfAnswer is the answer --json prints: the goal asked about, and the
// design records that name it. The list is present and empty rather than
// absent when there is none, so one reader reads both answers.
type designOfAnswer struct {
	Goal    string           `json:"goal"`
	Designs []designOfRecord `json:"designs"`
}

// designOfRecord is one design record as this verb reports it: what it is
// called in the project, where it stands, and where to read it.
type designOfRecord struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Path   string `json:"path"`
}

// projectDesignOf answers the one question a seat asks before it calls a
// design done, and the one the design critique asks before it attacks a
// design: does a design record name this goal? It prints every design record
// whose Goals names it — id, status and path — and refuses when there is
// none, because a design the resolver cannot find is a document the goal page,
// the Partner and the next seat will never see.
//
// A goal the ledger does not have is refused the way the resolver refuses one,
// and refused before the answer: "there is no design for it" would be a true
// sentence about a goal that does not exist.
func projectDesignOf(args []string, stdout, stderr io.Writer) int {
	flags := projectFlags("design-of", stderr)
	root := pathFlag(flags, "root", ".", "a path at or below the metasystem installation")
	goal := flags.String("goal", "", "the ledger goal the design is for")
	asJSON := flags.Bool("json", false, "print the answer as one JSON object")
	if flags.Parse(args) != nil {
		return 2
	}
	if flags.NArg() != 0 || *goal == "" {
		fmt.Fprintln(stderr, "usage: metasystem project design-of --goal ID [--root DIR] [--json]")
		return 2
	}
	read, code := projectRead(*root, stderr)
	if read == nil {
		return code
	}
	if !read.HasGoal(*goal) {
		fmt.Fprintf(stderr, "metasystem project design-of: the goal %s is not in the ledger\n", *goal)
		return 2
	}
	found := read.List(project.KindDesign, project.ListOptions{Goal: *goal})
	if *asJSON {
		answer := designOfAnswer{Goal: *goal, Designs: make([]designOfRecord, 0, len(found))}
		for _, record := range found {
			answer.Designs = append(answer.Designs,
				designOfRecord{ID: record.ID, Status: record.Status, Path: record.Path})
		}
		encoded, err := json.Marshal(answer)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintln(stdout, string(encoded))
	} else {
		for _, record := range found {
			fmt.Fprintf(stdout, "%s\t%s\t%s\n", record.ID, record.Status, record.Path)
		}
	}
	if len(found) == 0 {
		fmt.Fprintf(stderr, designOfRefusal, *goal)
		return 1
	}
	return 0
}

// projectSubject takes the one word a verb is about — a kind, an id — from
// anywhere among its flags. Go's flag package stops at the first argument that
// is not a flag, and "project list design --status draft" reads the way a human
// would write it, so the subject is lifted out and the rest is parsed again.
func projectSubject(flags *flag.FlagSet, args []string) (string, bool) {
	if flags.Parse(args) != nil || flags.NArg() == 0 {
		return "", false
	}
	subject := flags.Arg(0)
	rest := append([]string(nil), flags.Args()[1:]...)
	if flags.Parse(rest) != nil || flags.NArg() != 0 {
		return "", false
	}
	return subject, true
}

func projectFlags(verb string, stderr io.Writer) *flag.FlagSet {
	flags := flag.NewFlagSet("project "+verb, flag.ContinueOnError)
	flags.SetOutput(stderr)
	return flags
}

// projectRead resolves the roots from the named installation and reads every
// home once. A nil project is a refusal already printed.
func projectRead(root string, stderr io.Writer) (*project.Project, int) {
	roots, err := project.ResolveRoots(root)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return nil, 1
	}
	read, err := project.Read(roots)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return nil, 1
	}
	return read, 0
}

func oneOf(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
