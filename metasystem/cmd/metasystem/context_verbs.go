package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	usagepkg "github.com/widoriezebos/agentic-tools/metasystem/internal/usage"
)

type contextStatusOutput struct {
	Diagnostic bool                `json:"diagnostic"`
	Role       steward.RoleVerdict `json:"role"`
	Reading    contextReadingView  `json:"reading"`
}

type contextReadingView struct {
	Capability     usagepkg.Capability  `json:"capability"`
	Latest         *usagepkg.CallSample `json:"latest"`
	Reason         string               `json:"reason"`
	NewSamples     int                  `json:"newSamples"`
	NewMarkers     int                  `json:"newMarkers"`
	PreviousReadAt time.Time            `json:"previousReadAt"`
	Cursor         contextCursorView    `json:"cursor"`
}

type contextCursorView struct {
	Offset         int64     `json:"offset"`
	Line           int64     `json:"line"`
	SidechainCount int64     `json:"sidechainCount"`
	SamplesBytes   int64     `json:"samplesBytes"`
	LastReadAt     time.Time `json:"lastReadAt"`
}

func runContextStatus(args []string) int {
	flags := flag.NewFlagSet("context status", flag.ContinueOnError)
	root := flags.String("root", "", "installation or containing template root")
	runtimeName := flags.String("runtime", "", "explicit runtime")
	session := flags.String("session", "", "explicit session")
	transcript := flags.String("transcript", "", "call transcript override")
	asJSON := flags.Bool("json", false, "print structured status")
	if flags.Parse(args) != nil {
		return 2
	}
	transcriptSupplied := false
	flags.Visit(func(option *flag.Flag) {
		if option.Name == "transcript" {
			transcriptSupplied = true
		}
	})
	if *root == "" || flags.NArg() != 0 || (*runtimeName == "") != (*session == "") || (transcriptSupplied && *transcript == "") {
		fmt.Fprintln(os.Stderr, "usage: metasystem context status --root ROOT [--runtime R --session S] [--transcript PATH] [--json]")
		return 2
	}
	if *runtimeName != "" {
		if _, ok := runtimes.Lookup(*runtimeName); !ok {
			fmt.Fprintf(os.Stderr, "metasystem context status: unknown runtime: %s\n", *runtimeName)
			return 1
		}
	}
	stateRoot, err := goal.ResolveStateRoot(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "metasystem context status:", err)
		return 1
	}
	role, reading, readErr := steward.ContextBudgetLine(stateRoot, stateRoot, time.Now().UTC(), steward.ContextOptions{
		Runtime: *runtimeName, Session: *session, Transcript: *transcript,
	})
	if *asJSON {
		printJSON(contextStatusOutput{Diagnostic: transcriptSupplied, Role: role, Reading: projectContextReading(reading)})
	} else {
		fmt.Println(role.Line())
	}
	if readErr != nil {
		fmt.Fprintln(os.Stderr, "metasystem context status:", readErr)
		return 1
	}
	return 0
}

func projectContextReading(reading usagepkg.Reading) contextReadingView {
	return contextReadingView{
		Capability: reading.Capability, Latest: reading.Latest, Reason: reading.Reason,
		NewSamples: reading.NewSamples, NewMarkers: reading.NewMarkers,
		PreviousReadAt: reading.PreviousReadAt,
		Cursor: contextCursorView{
			Offset: reading.Cursor.Offset, Line: reading.Cursor.Line,
			SidechainCount: reading.Cursor.SidechainCount,
			SamplesBytes:   reading.Cursor.SamplesBytes, LastReadAt: reading.Cursor.LastReadAt,
		},
	}
}
