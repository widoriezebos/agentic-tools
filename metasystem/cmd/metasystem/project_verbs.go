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
	area := flags.String("area", "", "only records naming this area")
	status := flags.String("status", "", "only records with this status")
	kind, ok := projectSubject(flags, args)
	if !ok {
		fmt.Fprintln(stderr, "usage: metasystem project list KIND [--root DIR] [--area SLUG] [--status STATUS]")
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
	for _, record := range read.List(kind, project.ListOptions{Area: *area, Status: *status}) {
		fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\t%s\t%s\n",
			record.Kind, record.ID, record.Status, strings.Join(record.Areas, " "), record.Title, record.Path)
	}
	return 0
}

// projectShow prints one record: what it declares, where it lives, what it
// names, and what names it — the half of a reference the record cannot
// declare itself.
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
	fmt.Fprintln(stdout, "areas: "+strings.Join(record.Areas, " "))
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

// projectTree prints the declared areas, each with what it holds, and the
// project bucket last: what belongs to the whole rather than to a part.
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
	areas, whole := read.Tree()
	for _, area := range areas {
		heading := area.Area.Slug
		if area.Area.Name != "" {
			heading += " — " + area.Area.Name
		}
		fmt.Fprintln(stdout, heading)
		projectCounts(stdout, area.Counts)
	}
	fmt.Fprintln(stdout, project.ProjectArea)
	projectCounts(stdout, whole)
	return 0
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
	fmt.Fprintf(stdout, "project check passed: %d record(s) in %d home(s), %d declared area(s), %d question(s)\n",
		len(read.Records), len(read.Homes), len(read.Areas), len(read.Questions))
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
