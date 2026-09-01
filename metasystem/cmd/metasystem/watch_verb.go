package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	watchsurface "github.com/widoriezebos/agentic-tools/metasystem/internal/watch"
)

// runWatch keeps the established delegate-job waiter available as
// `metasystem watch --job ...`; without --job it emits a zero-write snapshot
// of verdicts already persisted by their owning producers.
func runWatch(args []string) int {
	if requestsJobWait(args) {
		return runJobWatchVerb(args)
	}

	flags := flag.NewFlagSet("watch", flag.ContinueOnError)
	root := flags.String("root", ".", "checkout root")
	asJSON := flags.Bool("json", false, "emit the typed snapshot as JSON")
	if flags.Parse(args) != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem watch [--root DIR] [--json]")
		return 2
	}

	snapshot := watchsurface.Read(*root)
	if *asJSON {
		encoded, err := json.MarshalIndent(snapshot, "", "  ")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
		fmt.Println(string(encoded))
	} else {
		printWatchSnapshot(snapshot)
	}
	return snapshot.ExitCode()
}

func requestsJobWait(args []string) bool {
	for _, arg := range args {
		if arg == "--job" || strings.HasPrefix(arg, "--job=") {
			return true
		}
	}
	return false
}

func printWatchSnapshot(snapshot watchsurface.Snapshot) {
	label := snapshot.Aggregate
	if snapshot.Empty {
		fmt.Printf("WATCH %s empty: no persisted tracked items\n", label)
	} else {
		fmt.Printf("WATCH %s\n", label)
	}
	for _, section := range snapshot.Sections {
		fmt.Printf("%-14s %-8s %d item(s) [%s]\n", section.Class, section.Verdict, len(section.Items), section.Store)
		for _, item := range section.Items {
			coordinates := ""
			if item.GoalField != "" {
				coordinates += " goal-field=" + string(item.GoalField)
			}
			if item.GoalID != "" {
				coordinates += " goal=" + item.GoalID
			}
			if item.Stage != "" {
				coordinates += " stage=" + item.Stage
			}
			if item.ObservedAt != "" {
				coordinates += " observed-at=" + item.ObservedAt
			}
			fmt.Printf("  %s %s %s%s [%s]", item.Kind, item.ID, item.Verdict, coordinates, item.Evidence)
			if item.Problem != "" {
				fmt.Printf(": %s", item.Problem)
			}
			fmt.Println()
		}
	}
}
