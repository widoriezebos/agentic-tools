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

	"github.com/widoriezebos/agentic-tools/metasystem/internal/project"
)

func runProjectID(args []string) int { return projectID(args, os.Stdout, os.Stderr) }

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
		fmt.Fprintln(stderr, "usage: metasystem internal project id")
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
		fmt.Fprintln(stderr, "usage: metasystem internal project check [--root DIR]")
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
		fmt.Fprintln(stderr, "usage: metasystem design find --goal ID [--root DIR] [--json]")
		return 2
	}
	read, code := projectRead(*root, stderr)
	if read == nil {
		return code
	}
	if !read.HasGoal(*goal) {
		fmt.Fprintf(stderr, "metasystem design find: the goal %s is not in the ledger\n", *goal)
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
