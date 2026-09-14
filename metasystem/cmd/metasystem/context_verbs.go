package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
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

func runContextReport(args []string) int {
	flags := flag.NewFlagSet("context report", flag.ContinueOnError)
	root := flags.String("root", "", "installation or containing template root")
	week := flags.String("week", "", "first UTC date in YYYY-MM-DD form")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" || *week == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem context report --root ROOT --week YYYY-MM-DD")
		return 2
	}
	weekStart, err := time.Parse("2006-01-02", *week)
	if err != nil || weekStart.Format("2006-01-02") != *week {
		fmt.Fprintln(os.Stderr, "metasystem context report: --week must be YYYY-MM-DD")
		return 2
	}
	stateRoot, err := goal.ResolveStateRoot(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "metasystem context report:", err)
		return 1
	}
	callsPath, reportPath, report, err := steward.WriteContextReport(stateRoot, weekStart, time.Now().UTC())
	if err != nil {
		var retired *steward.ContextEvidenceRetiredError
		if errors.As(err, &retired) {
			fmt.Fprintln(os.Stderr, retired.Error())
			return 9
		}
		fmt.Fprintln(os.Stderr, "metasystem context report:", err)
		return 1
	}
	verdict := "fail"
	if report.Pass {
		verdict = "pass"
	}
	fmt.Printf("calls=%s report=%s verdict=%s\n", callsPath, reportPath, strings.ToUpper(verdict))
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
