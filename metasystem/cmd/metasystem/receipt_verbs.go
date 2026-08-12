package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/receipt"
)

// runReceipt relays scripts/receipt.sh's calling convention: the action
// word, then one flag vocabulary shared by every action, exactly as the
// shell parsed it. A retro summary may ride as the first positional
// argument after the action.
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
		Root: ".", File: "plans/receipts.log",
		Skills: "none", Verify: "skipped", Corrections: "0", StopLoss: "no",
	}
	if action == "retro" && len(args) > 0 && !strings.HasPrefix(args[0], "--") {
		opts.Summary = args[0]
		args = args[1:]
	}
	need := func(i int) (string, bool) {
		if i+1 >= len(args) {
			usage()
			return "", false
		}
		return args[i+1], true
	}
	for i := 0; i < len(args); i++ {
		value, valueOK := "", true
		switch args[i] {
		case "--root":
			value, valueOK = need(i)
			opts.Root = value
		case "--file":
			value, valueOK = need(i)
			opts.File = value
		case "--type":
			value, valueOK = need(i)
			opts.Type = value
		case "--outcome":
			value, valueOK = need(i)
			opts.Outcome = value
		case "--skills":
			value, valueOK = need(i)
			opts.Skills = value
		case "--verify":
			value, valueOK = need(i)
			opts.Verify = value
		case "--corrections":
			value, valueOK = need(i)
			opts.Corrections = value
		case "--stop-loss":
			value, valueOK = need(i)
			opts.StopLoss = value
		case "--delegate":
			value, valueOK = need(i)
			opts.Delegates = append(opts.Delegates, value)
		case "--note":
			value, valueOK = need(i)
			opts.Note = value
		case "--ref-epoch":
			value, valueOK = need(i)
			opts.RefEpoch = value
		case "--ref-sha1":
			value, valueOK = need(i)
			opts.RefSHA1 = value
		case "--field":
			value, valueOK = need(i)
			opts.Field = value
		case "--was":
			value, valueOK = need(i)
			opts.Was = value
		case "--now":
			value, valueOK = need(i)
			opts.NowValue = value
		case "--reason":
			value, valueOK = need(i)
			opts.Reason = value
		case "--all":
			opts.All = true
			continue // boolean flag: no value to skip
		case "--max-age-days":
			value, valueOK = need(i)
			opts.MaxAgeDays = value
			opts.MaxAgeSet = true
		case "--max-receipts":
			value, valueOK = need(i)
			opts.MaxReceipts = value
			opts.MaxReceiptsSet = true
		case "-h", "--help":
			usage()
			return 0
		default:
			usage()
			return 2
		}
		if !valueOK {
			return 2
		}
		i++
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
