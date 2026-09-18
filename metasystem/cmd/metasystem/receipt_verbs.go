package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/receipt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

var receiptLaunchStore = func() launch.Store { return launch.Store{} }

// runReceipt relays scripts/receipt.sh's calling convention: the action
// word, then one flag vocabulary shared by every action (a flag.FlagSet —
// a hand-rolled switch would re-implement flag semantics loosely). A
// retro summary may ride as the first positional argument after the
// action. The FlagSet's own messages are discarded so a misuse prints
// exactly the one-line usage its callers know.
func runReceipt(args []string) int {
	usage := func() {
		fmt.Fprintln(os.Stderr, "usage: metasystem receipt add|correct|check|stats|retro [flags] (see scripts/receipt.sh --help)")
	}
	if len(args) == 0 {
		usage()
		return 2
	}
	action := args[0]
	args = args[1:]
	opts := receipt.Options{
		Root:   ".",
		Skills: "none", Verify: "skipped", Corrections: "0", StopLoss: "no",
	}
	if action == "retro" && len(args) > 0 && !strings.HasPrefix(args[0], "--") {
		opts.Summary = args[0]
		args = args[1:]
	}
	flags := flag.NewFlagSet("receipt "+action, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
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
	flags.StringVar(&opts.MaxAgeDays, "max-age-days", "", "cadence age ceiling")
	flags.StringVar(&opts.MaxReceipts, "max-receipts", "", "cadence receipt ceiling")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			usage()
			return 0
		}
		usage()
		return 2
	}
	if flags.NArg() > 0 {
		usage()
		return 2
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
			fmt.Fprintln(os.Stderr, "receipt:", err)
			return 1
		}
		opts.File = filepath.Join(root, "receipts.log")
	}
	if action == "add" && *launchID != "" {
		record, err := receiptLaunchStore().Read(*launchID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "receipt launch %s: %v\n", *launchID, err)
			return 2
		}
		if !record.State.Terminal() {
			fmt.Fprintf(os.Stderr, "receipt launch %s is not terminal\n", *launchID)
			return 2
		}
		fillReceiptUsageFromLaunch(&opts, record)
	}
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
	default:
		usage()
		return 2
	}
	for _, line := range result.Out {
		fmt.Println(line)
	}
	for _, line := range result.Err {
		fmt.Fprintln(os.Stderr, line)
	}
	return result.Code
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
