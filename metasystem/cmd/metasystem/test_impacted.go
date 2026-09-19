package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testimpact"
	"io"
	"os"
	"os/signal"
	"syscall"
)

func runTestImpacted(args []string) int {
	jsonOutput := false
	for _, argument := range args {
		jsonOutput = jsonOutput || argument == "--json"
	}
	flags := flag.NewFlagSet("test impacted", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	base := flags.String("base", "", "base commit-ish (default HEAD)")
	list := flags.Bool("list", false, "list selected commands without running them")
	flags.BoolVar(&jsonOutput, "json", jsonOutput, "emit one JSON envelope")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		envelope := testimpact.Envelope{SchemaVersion: testimpact.SchemaVersion,
			Error: &testimpact.Failure{Code: testimpact.CodeUsage, Message: "usage: metasystem test impacted [--base COMMIT-ISH] [--list] [--json]"}}
		writeImpactedOutput(jsonOutput, envelope)
		fmt.Fprintln(os.Stderr, testimpact.CodeUsage+":", envelope.Error.Message)
		return 2
	}
	cwd, err := os.Getwd()
	if err != nil {
		envelope := testimpact.Envelope{SchemaVersion: testimpact.SchemaVersion,
			Error: &testimpact.Failure{Code: testimpact.CodeInputInvalid, Message: err.Error()}}
		writeImpactedOutput(jsonOutput, envelope)
		return 2
	}
	mode := testimpact.ModeRun
	if *list {
		mode = testimpact.ModeList
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	envelope, code := testimpact.Run(ctx, testimpact.Options{Start: cwd, Base: *base, Mode: mode,
		Stderr: os.Stderr})
	writeImpactedOutput(jsonOutput, envelope)
	if envelope.Error != nil {
		fmt.Fprintln(os.Stderr, envelope.Error.Code+":", envelope.Error.Message)
	}
	return code
}
func writeImpactedOutput(jsonOutput bool, envelope testimpact.Envelope) {
	if jsonOutput {
		_ = testimpact.WriteEnvelope(os.Stdout, envelope)
		return
	}
	testimpact.WriteSummary(os.Stdout, envelope)
}
