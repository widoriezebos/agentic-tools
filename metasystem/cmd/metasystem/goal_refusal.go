package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalbudget"
)

type humanVerbValues struct {
	verb, root, lineage, id, by, approvedRef, temporaryWord, reviewBy string
	boxTyped                                                          string
	box                                                               *goal.Budget
	standingBox                                                       *goal.Budget
	tierBox                                                           *goal.Budget
	state                                                             string
	fixtureHumanAuthority, stopFence, byTyped                         bool
	rawArgs                                                           []string
	// report, when set, receives the refusal for the public intent commands
	// to render; the legacy calls print it below exactly as before.
	report *ownerReport
}

type humanVerbRemedy struct {
	command string
	words   string
}

func humanProofRemedy(values *humanVerbValues, fixture bool, temporaryWord, reviewBy string) humanVerbRemedy {
	if fixture && (temporaryWord != "" || reviewBy != "") {
		return humanVerbRemedy{command: values.sameCommandWithout("temporary-human-word", "review-by")}
	}
	if temporaryWord != "" || reviewBy != "" {
		return humanVerbRemedy{words: "supply a valid temporary human word and review date together"}
	}
	return humanVerbRemedy{words: "run this at the enrolled terminal"}
}

func newHumanVerbValues(verb string, args []string) *humanVerbValues {
	return &humanVerbValues{verb: verb, root: ".", byTyped: argumentHasFlag(args, "by"), rawArgs: append([]string(nil), args...)}
}

func (values *humanVerbValues) bindSyncFlags(flags *syncFlags) {
	if flags == nil {
		return
	}
	values.root, values.lineage, values.id, values.by = flags.root, flags.lineage, flags.id, flags.by
	values.approvedRef, values.temporaryWord, values.reviewBy = flags.approvedRef, flags.temporaryWord, flags.reviewBy
	values.fixtureHumanAuthority = flags.fixtureHumanAuthority
}

func (values *humanVerbValues) bindGoalView(file *goal.GoalFile, tierBox goal.Budget) {
	values.tierBox = &tierBox
	if file == nil {
		return
	}
	values.state, values.stopFence = file.State, file.StopFence != nil
	if file.Budget != nil {
		standing := *file.Budget
		values.standingBox = &standing
	}
}

func refuseHumanVerb(values *humanVerbValues, code int, sentence string, remedy humanVerbRemedy) int {
	sentence = strings.Join(strings.Fields(strings.TrimSpace(sentence)), " ")
	sentence = strings.TrimSuffix(sentence, ".") + "."
	if values.report != nil {
		values.report.refusal = &ownerRefusal{code: code, sentence: sentence, remedy: remedy}
		return code
	}
	fmt.Fprintf(os.Stderr, "goal %s: %s\n", values.verb, sentence)
	if remedy.command != "" {
		fmt.Fprintln(os.Stderr, "run:", remedy.command)
	} else {
		fmt.Fprintln(os.Stderr, "no command completes this:", strings.TrimSpace(remedy.words))
	}
	return code
}

func (values *humanVerbValues) budgetCommand(box string) string {
	return values.renderBudgetCommand(box, true)
}

func (values *humanVerbValues) budgetCommandWithoutApprovedRef(box string) string {
	return values.renderBudgetCommand(box, false)
}

func (values *humanVerbValues) renderBudgetCommand(box string, includeApprovedRef bool) string {
	args := []string{"metasystem", "goal", "budget"}
	if values.root != "" && values.root != "." {
		args = append(args, "--root", values.root)
	}
	if values.lineage != "" {
		args = append(args, "--lineage", values.lineage)
	}
	if values.id != "" {
		args = append(args, "--id", values.id)
	}
	if values.fixtureHumanAuthority {
		args = append(args, "--fixture-human-authority")
	}
	if values.byTyped && values.by != "" {
		args = append(args, "--by", values.by)
	}
	if values.temporaryWord != "" {
		args = append(args, "--temporary-human-word", values.temporaryWord)
	}
	if values.reviewBy != "" {
		args = append(args, "--review-by", values.reviewBy)
	}
	if includeApprovedRef && values.approvedRef != "" {
		args = append(args, "--approved-ref", values.approvedRef)
	}
	if box != "" {
		args = append(args, box)
	}
	return shellCommand(args)
}

func (values *humanVerbValues) suggestedBudgetBox() string {
	if values.box != nil {
		return goalbudget.FormatBox(*values.box)
	}
	if values.standingBox != nil {
		return goalbudget.FormatBox(*values.standingBox)
	}
	if values.tierBox != nil {
		return goalbudget.FormatBox(*values.tierBox)
	}
	return ""
}

func (values *humanVerbValues) sameCommandWithout(drop ...string) string {
	dropped := map[string]bool{}
	for _, name := range drop {
		dropped[strings.TrimPrefix(name, "--")] = true
	}
	args := []string{"metasystem", "goal", values.verb}
	for index := 0; index < len(values.rawArgs); index++ {
		token := values.rawArgs[index]
		if !strings.HasPrefix(token, "--") {
			args = append(args, token)
			continue
		}
		nameValue := strings.TrimPrefix(token, "--")
		name, _, joined := strings.Cut(nameValue, "=")
		if !dropped[name] {
			args = append(args, token)
		}
		if joined || goalBooleanFlag(name) || index+1 >= len(values.rawArgs) {
			continue
		}
		index++
		if !dropped[name] {
			args = append(args, values.rawArgs[index])
		}
	}
	return shellCommand(args)
}

func shellCommand(args []string) string {
	quoted := make([]string, len(args))
	for index, arg := range args {
		if arg != "" && strings.IndexFunc(arg, func(r rune) bool {
			return !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || strings.ContainsRune("_@%+=:,./-", r))
		}) == -1 {
			quoted[index] = arg
		} else {
			quoted[index] = "'" + strings.ReplaceAll(arg, "'", "'\"'\"'") + "'"
		}
	}
	return strings.Join(quoted, " ")
}

func argumentHasFlag(args []string, wanted string) bool {
	prefix := "--" + wanted
	for _, arg := range args {
		if arg == prefix || strings.HasPrefix(arg, prefix+"=") {
			return true
		}
	}
	return false
}

func goalBooleanFlag(name string) bool {
	switch name {
	case "claim", "refresh-only", "sweep", "preview", "fixture-human-authority":
		return true
	default:
		return false
	}
}
