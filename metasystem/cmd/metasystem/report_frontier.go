package main

import (
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/report"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
)

// runExperimentRecord, runExperimentChallenge and runExperimentStatus are
// the experiment actions: the frontier report's action word, then its
// flags.
func runExperimentRecord(args []string, stdout, stderr io.Writer) int {
	return runReportFrontier(append([]string{"record"}, args...), stdout, stderr)
}

func runExperimentChallenge(args []string, stdout, stderr io.Writer) int {
	return runReportFrontier(append([]string{"challenge"}, args...), stdout, stderr)
}

func runExperimentStatus(args []string, stdout, stderr io.Writer) int {
	return runReportFrontier(append([]string{"status"}, args...), stdout, stderr)
}

// runReportFrontier is the frontier report's one calling convention:
// the action word, then the flag set the shell always accepted.
func runReportFrontier(args []string, stdout, stderr io.Writer) int {
	usage := textui.Hint{Argv: []string{"metasystem", "experiment"}, Reason: "lists its actions"}
	if len(args) == 0 {
		return refusePassthrough(stderr, 2, "experiment takes an action: record, challenge, status or check; nothing was done", usage)
	}
	action := args[0]
	flags := newFlagSet("experiment "+action, stdout, stderr)
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
		return refusePassthrough(stderr, 2, "experiment has no action "+action+"; nothing was done", usage)
	}
	if ferr != nil {
		// A usage problem names the action's options; a verdict (not beaten,
		// noise, expired, a dirty tree) leaves the frontier as it stands.
		hint := textui.Hint{Argv: []string{"metasystem", "experiment", action, "--help"}, Reason: "its options"}
		if ferr.Code == 1 {
			hint = textui.Hint{Reason: "the frontier stands; metasystem experiment status shows it"}
		}
		return refusePassthrough(stderr, ferr.Code, ferr.Message, hint)
	}
	page := passthroughPage(stdout, "", false)
	switch {
	case action == "status" && len(lines) > 0 && strings.Contains(lines[0], "="):
		layFrontier(page, lines)
	case action == "record" && len(lines) == 2:
		page.Done(capitalized(lines[0]))
		page.Hint(textui.Hint{Reason: shortPaths(page.Env(), lines[1])})
	case action == "challenge":
		page.Done(capitalized(lines[0]))
	default:
		page.Headline(capitalized(shortPaths(page.Env(), strings.Join(lines, "; "))))
	}
	printPage(stdout, page)
	return 0
}

// layFrontier is the recorded frontier: its score and direction, then when
// and at which commit it was recorded, its noise floor, window, evaluation
// and artifact. The frontier file is a record of name=value lines.
func layFrontier(page *textui.Page, lines []string) {
	env := page.Env()
	fields := map[string]string{}
	for _, line := range strings.Split(strings.Join(lines, "\n"), "\n") {
		if name, value, found := strings.Cut(line, "="); found {
			fields[name] = value
		}
	}
	better := "higher is better"
	if fields["direction"] == "min" {
		better = "lower is better"
	}
	page.Headline("Frontier: score "+fields["score"], better)
	recorded := "at " + textui.SHA(fields["sha"])
	if epoch, err := strconv.ParseInt(fields["recorded_epoch"], 10, 64); err == nil {
		recorded = env.Time(time.Unix(epoch, 0)) + " " + recorded
	}
	window := "none"
	if fields["max_age_minutes"] != "" {
		window = fields["max_age_minutes"] + " minutes"
	}
	page.Facts(
		textui.KV{Key: "recorded", Value: []textui.Span{textui.Plain(recorded)}},
		textui.KV{Key: "noise floor", Value: []textui.Span{textui.Plain(fields["min_delta"])}},
		textui.KV{Key: "window", Value: []textui.Span{textui.Plain(window)}},
		textui.KV{Key: "evaluation", Value: []textui.Span{textui.Plain(fields["eval"])}},
		textui.KV{Key: "artifact", Value: []textui.Span{textui.Plain(env.Path(fields["artifact"]))}},
	)
}

// capitalized is a sentence with its first letter raised.
func capitalized(text string) string {
	if text == "" {
		return text
	}
	return strings.ToUpper(text[:1]) + text[1:]
}
