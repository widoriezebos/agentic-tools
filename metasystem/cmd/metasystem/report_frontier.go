package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/report"
)

// runReportFrontier relays scripts/frontier.sh's calling convention:
// the action word, then the flag set the shell always accepted.
func runReportFrontier(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem report frontier record|challenge|status [flags]")
		return 2
	}
	action := args[0]
	flags := flag.NewFlagSet("report frontier "+action, flag.ContinueOnError)
	opts := report.FrontierOptions{Repo: "."}
	flags.StringVar(&opts.File, "file", "plans/frontier", "frontier file")
	flags.StringVar(&opts.Score, "score", "", "candidate score")
	flags.StringVar(&opts.Eval, "eval", "", "evaluation command that produced the score")
	flags.StringVar(&opts.Artifact, "artifact", "", "artifact path")
	flags.StringVar(&opts.MinDelta, "min-delta", "", "noise floor")
	flags.StringVar(&opts.MaxAge, "max-age-minutes", "", "measurement window")
	flags.StringVar(&opts.Direction, "direction", "", "max or min")
	flags.BoolVar(&opts.Force, "force", false, "re-baseline after an evaluation change")
	if flags.Parse(args[1:]) != nil {
		return 2
	}
	var lines []string
	var ferr *report.FrontierError
	switch action {
	case "record":
		lines, ferr = report.FrontierRecord(opts)
	case "challenge":
		lines, ferr = report.FrontierChallenge(opts)
	case "status":
		lines, ferr = report.FrontierStatus(opts)
	default:
		fmt.Fprintln(os.Stderr, "usage: metasystem report frontier record|challenge|status [flags]")
		return 2
	}
	for _, line := range lines {
		fmt.Println(line)
	}
	if ferr != nil {
		fmt.Fprintln(os.Stderr, ferr.Message)
		return ferr.Code
	}
	return 0
}
