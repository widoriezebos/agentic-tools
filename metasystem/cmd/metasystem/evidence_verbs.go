package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/evidence"
)

// The evidence family owns the durable-evidence lifecycle: raw run evidence
// under gitignored artifacts/ is mirrored to the evidence root before it
// counts as disposable, and disposable evidence eventually gets disposed of.

// runEvidenceGC runs one collection pass: collect closed terminal chains
// verified against the mirror manifest, prune mirrored job records past the
// grace window, sweep per-job residue, prune empty non-spine directories, and
// copy-then-age flight-recorder archives.
func runEvidenceGC(args []string) int {
	flags := flag.NewFlagSet("evidence gc", flag.ContinueOnError)
	root := flags.String("root", "", "checkout root")
	evidenceRoot := flags.String("evidence", "", "durable evidence root (absolute path)")
	grace := flags.Float64("grace-seconds", 5400, "seconds a mirrored terminal chain's job records stay after mirroring")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" || *evidenceRoot == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem evidence gc --root DIR --evidence DIR [--grace-seconds SEC]")
		return 2
	}
	if err := evidence.GC(*root, *evidenceRoot, *grace, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
