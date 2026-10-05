package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/cliflags"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/receipt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/retrodebt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
)

var receiptLaunchStore = func() launch.Store { return launch.Store{} }

// runReceipt is the receipt ledger verb (metasystem receipt ACTION): the action
// word, then one flag vocabulary shared by every action (a flag.FlagSet —
// a hand-rolled switch would re-implement flag semantics loosely). A
// retro summary may ride as the first positional argument after the
// action. The FlagSet's own messages are discarded so a misuse prints
// exactly the one-line usage its callers know.
func runReceipt(args []string, stdout, stderr io.Writer) int {
	action, opts, result, env, code := receiptResult(args, stdout, stderr)
	if code >= 0 {
		return code
	}
	return printReceipt(stdout, stderr, env, action, opts, result)
}

// runReceiptRetro is receipt retro: the owner's retro action.
func runReceiptRetro(args []string, stdout, stderr io.Writer) int {
	return runReceipt(append([]string{"retro"}, args...), stdout, stderr)
}

// receiptStatus is receipt status: whether a retro is due, and the
// period's numbers (or, with --uncovered, the lines no retro has read),
// on one page; its exit code is the due check's (1 when a retro is due).
func receiptStatus(args []string, stdout, stderr io.Writer) int {
	if _, uncovered, rest := takeIntentFlag(args, "uncovered", false); uncovered {
		return runReceipt(append([]string{"uncovered"}, rest...), stdout, stderr)
	}
	all, _, checkArgs := takeIntentFlag(args, "all", false)
	if _, named, _ := takeIntentFlag(checkArgs, "file", true); !named {
		// Both reads use the one ledger, resolved once.
		root, err := stateroot.StateRoot(stateroot.Receipts)
		if err != nil {
			return refusePassthrough(stderr, 1, "the receipt ledger cannot be found: "+err.Error(),
				textui.Hint{Argv: []string{"metasystem", "system", "check"}, Reason: "names what is wrong here"})
		}
		checkArgs = append(slices.Clone(checkArgs), "--file", root.Path("receipts.log"))
	}
	_, opts, due, env, code := receiptResult(append([]string{"check"}, checkArgs...), stdout, stderr)
	if code >= 0 {
		return code
	}
	if due.Code > 1 {
		return printReceipt(stdout, stderr, env, "check", opts, due)
	}
	statsArgs := checkArgs
	if all == "true" {
		statsArgs = append(slices.Clone(checkArgs), "--all")
	}
	_, _, stats, _, code := receiptResult(append([]string{"stats"}, statsArgs...), stdout, stderr)
	if code >= 0 {
		return code
	}
	if stats.Code != 0 {
		return printReceipt(stdout, stderr, env, "stats", opts, stats)
	}
	page := textui.New(env)
	if due.Code == 1 {
		page.Mark(textui.Alert, strings.TrimPrefix(strings.Join(due.Err, "; "), "metasystem "))
		page.Hint(textui.Hint{Argv: []string{"metasystem", "receipt", "status", "--uncovered"}, Reason: "the lines the retro reads first"})
	} else {
		page.Headline(strings.Join(due.Out, "; "))
	}
	layReceiptStats(page, stats.Out, all == "true")
	repoRoot, err := goal.ResolveStateRoot(opts.Root)
	var open []retrodebt.Entry
	if err == nil {
		open, err = retrodebt.Open(repoRoot)
	}
	if err != nil {
		return refusePassthrough(stderr, 2, "the retro debt cannot be read: "+err.Error(),
			textui.Hint{Argv: []string{"metasystem", "system", "check"}, Reason: "names what is wrong here"})
	}
	debt := page.Section("Retro debt", "")
	if len(open) == 0 {
		debt.Text("none")
	}
	for _, item := range open {
		since := item.RaisedAt
		if at, err := time.Parse(time.RFC3339, since); err == nil {
			since = env.Time(at)
		}
		debt.Text(item.Kind + " " + item.Source + ": retro owed since " + since)
	}
	stream := stdout
	if due.Code == 1 {
		// Due is the check's failure: its page goes where failures go.
		stream = stderr
	}
	printPage(stream, page)
	return due.Code
}

// layReceiptStats lays the period's numbers out under one heading: each
// "name=count" the ledger reader counts, in a person's words.
func layReceiptStats(page *textui.Page, lines []string, all bool) {
	title := "Since the last retro"
	if all {
		title = "The whole ledger"
	}
	words := map[string]string{"receipts": "receipts", "corrections": "corrections", "caught_by_verify": "caught by verify",
		"stop_loss_triggered": "stop-loss triggered", "critique_waivers": "critique waivers", "span_days": "span in days"}
	section := page.Section(title, "")
	var items []string
	for index, line := range lines {
		name, value, counted := strings.Cut(line, "=")
		if !counted {
			// The goal ledger's open read items follow their heading.
			items = lines[index:]
			break
		}
		switch {
		case strings.HasPrefix(name, "outcome_"):
			name = strings.TrimPrefix(name, "outcome_")
		case strings.HasPrefix(name, "type_"):
			name = "type " + strings.TrimPrefix(name, "type_")
		case words[name] != "":
			name = words[name]
		}
		section.KV(name, textui.Plain(value))
	}
	if len(items) > 1 {
		read := page.Section(items[0], "")
		for _, item := range items[1:] {
			read.Text(item)
		}
	}
}

// layReceiptLines is each ledger line as a person reads it: when, what
// kind of work, its outcome, its goal and note; --verbose prints the
// lines themselves.
func layReceiptLines(page *textui.Page, lines []string) {
	env := page.Env()
	table := page.Section("", "--verbose prints the lines exactly").Table(textui.Column{}, textui.Column{}, textui.Column{}, textui.Column{Flex: true, Wrap: true})
	for _, line := range lines {
		fields := strings.Split(line, "|")
		if len(fields) < 3 {
			table.Row(textui.Plain(""), textui.Plain(""), textui.Plain(""), textui.Plain(line))
			continue
		}
		values := map[string]string{}
		for _, field := range fields[3:] {
			if name, value, found := strings.Cut(field, "="); found {
				values[name] = value
			}
		}
		when := fields[1]
		if epoch, err := strconv.ParseInt(fields[0], 10, 64); err == nil {
			when = env.Time(time.Unix(epoch, 0))
		}
		what := strings.ToLower(fields[2])
		if values["type"] != "" {
			what = values["type"]
		}
		about := values["note"]
		if goalID := values["goal"]; goalID != "" && goalID != "none" {
			about = goalID + ": " + about
		}
		table.Row(textui.Plain(when), textui.Plain(what), textui.Plain(values["outcome"]), textui.Plain(about))
	}
}

// printReceipt lays out one receipt action's result: an act's ✓ line and
// its notes, a read's lines, or a refusal's two lines.
func printReceipt(stdout, stderr io.Writer, env textui.Env, action string, opts receipt.Options, result receipt.Result) int {
	if result.Code != 0 || len(result.Out) == 0 {
		public := map[string]string{"add": "add", "correct": "add", "stats": "status", "retro": "retro", "check": "status", "uncovered": "status"}[action]
		return printOwnerLines(stdout, stderr, env.Repo, result.Out, result.Err, result.Code,
			textui.Hint{Argv: []string{"metasystem", "receipt", public, "--help"}, Reason: "its forms and options; nothing was recorded"})
	}
	page := textui.New(env)
	switch action {
	case "add", "correct", "retro":
		first := shortPaths(env, result.Out[0])
		page.Done(strings.ToUpper(first[:1]) + first[1:])
	case "uncovered":
		lines := result.Out
		if len(lines) == 1 && strings.HasPrefix(lines[0], "{") {
			// --json: the uncovered read as it always printed.
			_, _ = io.WriteString(stdout, lines[0]+"\n")
			return 0
		}
		// The lines and their token, read from the ledger the owner read.
		ledger, _ := os.ReadFile(opts.File)
		coverage := receipt.CoverageOf(string(ledger))
		page.Headline(textui.Count(len(coverage.Lines), "ledger line", "ledger lines")+" no retro has read", "token "+coverage.Token)
		if len(coverage.Lines) > 0 && page.Verbose() {
			// The lines a retro mines, exactly as the ledger holds them.
			section := page.Section("", "")
			for _, line := range coverage.Lines {
				section.Text(line)
			}
		} else if len(coverage.Lines) > 0 {
			layReceiptLines(page, coverage.Lines)
		}
		page.Hint(textui.Hint{Argv: []string{"metasystem", "receipt", "retro", "SUMMARY", "--covered", coverage.Token}, Reason: "records that a retro read them"})
		printPage(stdout, page)
		return 0
	default:
		page.Headline(shortPaths(env, result.Out[0]))
	}
	if len(result.Out) > 1 {
		section := page.Section("", "")
		for _, line := range result.Out[1:] {
			section.Text(shortPaths(env, line))
		}
	}
	for _, note := range result.Err {
		page.Section("", "").Item(textui.Alert, shortPaths(env, strings.TrimPrefix(note, "note: ")))
	}
	printPage(stdout, page)
	return 0
}

// receiptResult parses one receipt owner action's words and runs it: the
// action, its options, its result and the layout its page prints with; a
// code of 0 or more is a usage problem already printed.
func receiptResult(args []string, stdout, stderr io.Writer) (string, receipt.Options, receipt.Result, textui.Env, int) {
	var none receipt.Options
	env := passthroughEnv(stdout, "", false)
	if len(args) == 0 {
		return "", none, receipt.Result{}, env, refusePassthrough(stderr, 2, "receipt takes an action: add, status or retro; nothing was done",
			textui.Hint{Argv: []string{"metasystem", "receipt"}, Reason: "lists them"})
	}
	action := args[0]
	args = args[1:]
	// The public action each owner action answers as.
	public := map[string]string{"add": "add", "correct": "add", "stats": "status", "retro": "retro", "check": "status", "uncovered": "status"}[action]
	usage := func() int {
		if public == "" {
			return refusePassthrough(stderr, 2, "receipt has no action "+action+"; its actions are add, status and retro",
				textui.Hint{Argv: []string{"metasystem", "receipt"}, Reason: "lists them"})
		}
		return refusePassthrough(stderr, 2, "receipt "+public+" does not take these words; nothing was done",
			textui.Hint{Argv: []string{"metasystem", "receipt", public, "--help"}, Reason: "its forms and options"})
	}
	opts := receipt.Options{
		Root:   ".",
		Skills: "none", Verify: "skipped", Corrections: "0", StopLoss: "no",
	}
	if action == "retro" && len(args) > 0 && !strings.HasPrefix(args[0], "--") {
		opts.Summary = args[0]
		args = args[1:]
	}
	flags := newFlagSet("receipt "+public, stdout, stderr)
	pathFlagVar(flags, &opts.Root, "root", opts.Root, "checkout root")
	flags.StringVar(&opts.File, "file", opts.File, "receipt ledger file")
	flags.StringVar(&opts.Type, "type", "", "receipt type")
	flags.StringVar(&opts.Outcome, "outcome", "", "receipt outcome")
	flags.StringVar(&opts.Skills, "skills", opts.Skills, "skills used")
	flags.StringVar(&opts.Verify, "verify", opts.Verify, "verify outcome")
	flags.StringVar(&opts.Corrections, "corrections", opts.Corrections, "correction count")
	flags.StringVar(&opts.StopLoss, "stop-loss", opts.StopLoss, "stop-loss engaged")
	flags.Func("delegate", "runtime:model:job delegate entry (repeatable)", func(value string) error {
		opts.Delegates = append(opts.Delegates, value)
		return nil
	})
	flags.StringVar(&opts.Goal, "goal", "", "goal id this receipt belongs to")
	flags.StringVar(&opts.BuiltBy, "built-by", "", "builder classification: coordinator, delegate, or mixed")
	flags.StringVar(&opts.ReadTokens, "read-tokens", "", "tokens used by the read")
	flags.StringVar(&opts.ReadCalls, "read-calls", "", "tool calls used by the read")
	flags.StringVar(&opts.DesignTokens, "design-tokens", "", "tokens used by the design revision")
	flags.StringVar(&opts.DesignCalls, "design-calls", "", "tool calls used by the design revision")
	launchID := flags.String("launch", "", "terminal launch record that supplies usage")
	flags.StringVar(&opts.Requests, "requests", "", "requests made by the launch")
	flags.StringVar(&opts.ToolCalls, "tool-calls", "", "tool calls made by the launch")
	flags.StringVar(&opts.PeakContext, "peak-context", "", "largest context used by one request")
	flags.StringVar(&opts.CacheReadTokens, "cache-read-tokens", "", "tokens read from the cache")
	flags.StringVar(&opts.OutputTokens, "output-tokens", "", "output tokens produced")
	flags.StringVar(&opts.Note, "note", "", "free-text note")
	flags.StringVar(&opts.RefEpoch, "ref-epoch", "", "corrected line's epoch")
	flags.StringVar(&opts.RefSHA1, "ref-sha1", "", "corrected line's sha1")
	flags.StringVar(&opts.Field, "field", "", "corrected field")
	flags.StringVar(&opts.Was, "was", "", "corrected field's old value")
	flags.StringVar(&opts.NowValue, "now", "", "corrected field's new value")
	flags.StringVar(&opts.Reason, "reason", "", "correction reason")
	flags.BoolVar(&opts.All, "all", false, "count the whole ledger")
	flags.StringVar(&opts.Covered, "covered", "", "the token of receipt status --uncovered this retro covers")
	flags.BoolVar(&opts.JSON, "json", false, "print the uncovered read as JSON")
	flags.StringVar(&opts.MaxAgeDays, "max-age-days", "", "cadence age ceiling")
	flags.StringVar(&opts.MaxReceipts, "max-receipts", "", "cadence receipt ceiling")
	verbose := flags.Bool("verbose", false, "with --uncovered: the ledger lines exactly as they stand")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return action, opts, receipt.Result{}, env, 0
		}
		return action, opts, receipt.Result{}, env, 2
	}
	if flags.NArg() > 0 {
		return action, opts, receipt.Result{}, env, refusePassthrough(stderr, 2, fmt.Sprintf("%s takes no word %q; nothing was done", cliflags.Label(flags), flags.Arg(0)),
			textui.Hint{Argv: []string{"metasystem", "receipt", public, "--help"}, Reason: "its forms and options"})
	}
	flags.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "max-age-days":
			opts.MaxAgeSet = true
		case "max-receipts":
			opts.MaxReceiptsSet = true
		}
	})
	if opts.File == "" {
		root, err := stateroot.StateRoot(stateroot.Receipts)
		if err != nil {
			return action, opts, receipt.Result{}, env, refusePassthrough(stderr, 1, "the receipt ledger cannot be found: "+err.Error(),
				textui.Hint{Argv: []string{"metasystem", "system", "check"}, Reason: "names what is wrong here"})
		}
		opts.File = root.Path("receipts.log")
	}
	if action == "add" && *launchID != "" {
		record, err := receiptLaunchStore().Read(*launchID)
		if err != nil {
			return action, opts, receipt.Result{}, env, refusePassthrough(stderr, 2, fmt.Sprintf("launch %s cannot be read: %v; nothing was recorded", *launchID, err),
				textui.Hint{Argv: []string{"metasystem", "work", "status", "--all"}, Reason: "names the launches"})
		}
		if !record.State.Terminal() {
			return action, opts, receipt.Result{}, env, refusePassthrough(stderr, 2, fmt.Sprintf("launch %s has not ended, so its usage is not known yet; nothing was recorded", *launchID),
				textui.Hint{Argv: []string{"metasystem", "work", "wait", *launchID}, Reason: "then record the receipt"})
		}
		fillReceiptUsageFromLaunch(&opts, record)
	}
	env.Verbose = *verbose
	// The page's clock is the ledger's: a retro's age reads the same time.
	now := env.Now
	opts.Now = func() time.Time { return now }
	var result receipt.Result
	switch action {
	case "add":
		result = receipt.Add(opts)
	case "correct":
		result = receipt.Correct(opts)
	case "check":
		result = receipt.Check(opts)
	case "stats":
		result = receipt.Stats(opts)
	case "retro":
		result = receipt.Retro(opts)
	case "uncovered":
		result = receipt.Uncovered(opts)
	default:
		return action, opts, result, env, usage()
	}
	return action, opts, result, env, -1
}

func fillReceiptUsageFromLaunch(opts *receipt.Options, record launch.Record) {
	measurement := record.Measurement
	fill := func(target *string, value string) {
		if *target == "" {
			*target = value
		}
	}
	if opts.Type == "design" {
		fill(&opts.DesignTokens, strconv.FormatInt(measurement.TotalTokens(), 10))
		fill(&opts.DesignCalls, strconv.Itoa(measurement.ToolCalls))
	}
	if opts.Type == "review" {
		fill(&opts.ReadTokens, strconv.FormatInt(measurement.TotalTokens(), 10))
		fill(&opts.ReadCalls, strconv.Itoa(measurement.ToolCalls))
	}
	fill(&opts.Requests, strconv.Itoa(measurement.Calls))
	fill(&opts.ToolCalls, strconv.Itoa(measurement.ToolCalls))
	fill(&opts.PeakContext, strconv.FormatInt(measurement.PeakContext, 10))
	fill(&opts.CacheReadTokens, strconv.FormatInt(measurement.CacheReadTokens, 10))
	fill(&opts.OutputTokens, strconv.FormatInt(measurement.OutputTokens, 10))
}
