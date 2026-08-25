package main

// The evidence verb is the traceability gate: it joins the covenant's
// requirements to the app-owned evidence table and refuses intent
// that nothing on file backs. It never re-runs a proof and never
// writes a byte — recorded statuses are claims, and the success line
// says so every time.

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/covenant"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/evidencetable"
	"golang.org/x/sys/unix"
)

// runCovenantEvidence judges traceability, declared-dependency
// presence, and status lawfulness. Exit 0 traceable (report notes may
// ride along), 1 refused or unreadable, 2 usage.
func runCovenantEvidence(args []string) int {
	flags := flag.NewFlagSet("covenant evidence", flag.ContinueOnError)
	root := flags.String("root", ".", "repository root holding covenant.json and "+evidencetable.TableFilename)
	asJSON := flags.Bool("json", false, "emit the full report as JSON")
	if flags.Parse(args) != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem covenant evidence [--root DIR] [--json]")
		return 2
	}
	cov, err := covenant.Load(filepath.Join(*root, covenant.Filename))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	rootFD, err := evidencetable.OpenRoot(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer unix.Close(rootFD)
	table, err := evidencetable.LoadTable(rootFD, *root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	report := evidencetable.Judge(cov, table, rootFD)
	if *asJSON {
		encoded, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Println(string(encoded))
	} else {
		printEvidenceProse(report)
	}
	if report.Outcome != "traceable" {
		return 1
	}
	return 0
}

func printEvidenceProse(report *evidencetable.Report) {
	for _, refusal := range report.Refusals {
		fmt.Printf("REFUSED %s: %s\n", refusal.Kind, refusal.Detail)
	}
	for _, pair := range report.Pairs {
		line := fmt.Sprintf("requirement %s (proof %s): %s", pair.ID, pair.Proof, pair.Verdict)
		if pair.Assessment != "" {
			line += fmt.Sprintf(" [%s: %s]", pair.Status, pair.Assessment)
		}
		fmt.Println(line)
	}
	for _, orphan := range report.Orphans {
		fmt.Printf("orphan row %s %q (proof %s, %s)%s\n", orphan.CriterionID, orphan.Criterion,
			orphan.Proof, orphan.Status, formatOrphanNotes(orphan.Notes))
	}
	for _, note := range report.Notes {
		fmt.Println("note:", note)
	}
	if report.Outcome == "traceable" {
		fmt.Printf("evidence traceable: %s (%d requirement(s), wired %d, floating %d); recorded statuses are claims on file, not re-verified here\n",
			report.App, len(report.Pairs), report.Counts.DerivedWired, report.Counts.DerivedFloating)
	} else {
		fmt.Printf("evidence refused: %s (%d refusal(s))\n", report.App, len(report.Refusals))
	}
}

func formatOrphanNotes(notes []string) string {
	if len(notes) == 0 {
		return ""
	}
	return " — " + strings.Join(notes, "; ")
}
