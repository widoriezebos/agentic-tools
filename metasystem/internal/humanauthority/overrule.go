package humanauthority

import (
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/shellquote"
)

// OverruleLine gives the person's command, remaining effects and compensation.
func OverruleLine(d Decision) (line, remainder, compensation string) {
	if d.Outcome != "confirmed" && d.Outcome != "partial" {
		return "", "", ""
	}
	switch d.Class {
	case "lane-recovery":
		compensation = "landing stop --reason " + shellquote.Quote("overruled "+d.ID)
		remainder = "proofs already started and goals already selected stay as they are"
	case "budget":
		goal := ""
		args := d.Argv[min(2, len(d.Argv)):]
		for i := 0; i < len(args); i++ {
			arg := args[i]
			if strings.HasPrefix(arg, "--") && arg != "--" && arg != "--json" && arg != "--verbose" && !strings.Contains(arg, "=") {
				i++
				continue
			}
			if !strings.HasPrefix(arg, "--") {
				goal = arg
				break
			}
		}
		if goal == "" {
			return "", "", ""
		}
		compensation = "goal budget " + shellquote.Word(goal) + " undo-one-more-day " + shellquote.Word(d.ID)
		remainder = "job time spent under the extra day stays spent"
	case "run-release":
		remainder = "the cancelled processes and the released worktree stay released; metasystem work build G --work U starts the unit again"
	default:
		return "", "", ""
	}
	act := make([]string, len(d.Argv))
	for i, word := range d.Argv {
		act[i] = shellquote.Word(word)
	}
	line = "metasystem decision record --overrules " + shellquote.Word(d.ID) +
		" --text 'overrule' --rejected " + shellquote.Quote("the deputy's act") +
		" --undo " + shellquote.Quote(strings.Join(act, " "))
	if compensation != "" {
		line += " -- " + compensation
	}
	return line, remainder, compensation
}
