package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
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

type contextHandoffOutput struct {
	Nonce      string `json:"nonce"`
	StatePath  string `json:"statePath"`
	Digest     string `json:"digest"`
	IntentPath string `json:"intentPath"`
}

var classifyContextHandoffCaller, currentContextHandoffHolder, hookContextHandoffDelegate = lease.ClassifyAt, lease.CurrentHolder, lease.HookDelegate

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

func runContextHandoff(args []string) int {
	flags := flag.NewFlagSet("context handoff", flag.ContinueOnError)
	root := flags.String("root", "", "installation or containing template root")
	cancel := flags.String("cancel", "", "live handoff nonce to cancel")
	asJSON := flags.Bool("json", false, "print a bounded result")
	var scratchValues []string
	flags.Func("scratch", "purpose=P,path=REL[,required=true|false]", func(value string) error {
		scratchValues = append(scratchValues, value)
		return nil
	})
	if flags.Parse(args) != nil {
		return 2
	}
	cancelSupplied := false
	flags.Visit(func(option *flag.Flag) { cancelSupplied = cancelSupplied || option.Name == "cancel" })
	if *root == "" || flags.NArg() != 0 || (cancelSupplied && (*cancel == "" || len(scratchValues) != 0)) {
		fmt.Fprintln(os.Stderr, "usage: metasystem context handoff --root ROOT [--scratch purpose=P,path=REL[,required=true|false]]... [--json] | --root ROOT --cancel NONCE")
		return 2
	}
	stateRoot, err := goal.ResolveStateRoot(*root)
	if err != nil {
		return contextVerbError("handoff", err)
	}
	if cancelSupplied {
		if err := steward.CancelHandoff(stateRoot, *cancel); err != nil {
			return contextVerbError("handoff", err)
		}
		fmt.Printf("handoff cancelled: %s\n", *cancel)
		return 0
	}
	scratch, err := parseContextScratch(scratchValues)
	if err != nil {
		fmt.Fprintln(os.Stderr, "metasystem context handoff:", err)
		return 2
	}
	caller, err := contextHandoffCaller(stateRoot)
	if err != nil {
		return contextVerbError("handoff", err)
	}
	result, err := steward.Handoff(stateRoot, caller, scratch, time.Now().UTC(), filepath.Join(stateRoot, "memory", "receipts.log"))
	if err != nil {
		return contextVerbError("handoff", err)
	}
	if *asJSON {
		printJSON(contextHandoffOutput{Nonce: result.Nonce, StatePath: result.StatePath, Digest: result.StateDigest,
			IntentPath: filepath.Join(stateRoot, "artifacts", "agents", "steward", "intents", result.Nonce+".json")})
	} else {
		fmt.Printf("handoff recorded: %s state=%s sha256=%s\n", result.Nonce, result.StatePath, result.StateDigest)
	}
	return 0
}

func contextHandoffCaller(stateRoot string) (steward.HandoffCaller, error) {
	classified, err := classifyContextHandoffCaller(stateRoot, stateRoot, int64(os.Getppid()))
	if err != nil {
		return steward.HandoffCaller{}, fmt.Errorf("classify caller: %w", err)
	}
	machine, err := goal.ResolveMachine(stateRoot)
	if err != nil {
		return steward.HandoffCaller{}, err
	}
	caller := steward.HandoffCaller{Class: classified.Class, MainId: classified.MainId, Machine: machine}
	if classified.Class == lease.ClassDelegate {
		jobID, active, err := steward.ConsumedActiveJob(stateRoot)
		if err != nil || !active {
			return caller, err
		}
		delegate, err := hookContextHandoffDelegate(stateRoot, stateRoot, jobID, int64(os.Getppid()))
		if err == nil && delegate.Delegate && delegate.JobID == jobID {
			caller.JobId = jobID
		}
		return caller, err
	}
	if classified.Class != lease.ClassMain || classified.Announcement == nil {
		return caller, nil
	}
	holder, err := currentContextHandoffHolder(stateRoot)
	if err != nil {
		return steward.HandoffCaller{}, err
	}
	announcement := classified.Announcement
	caller.HolderMainId, caller.Runtime, caller.Session, caller.Tag = holder.MainId, announcement.Runtime, announcement.SessionId, announcement.InstanceTag
	caller.Ref = identity.Ref{Pid: announcement.Pid, StartedAtSec: announcement.PidStartedAt,
		StartTicks: announcement.PidStartTicks, BootID: announcement.BootID}
	return caller, nil
}

func parseContextScratch(values []string) ([]steward.ScratchArg, error) {
	result := make([]steward.ScratchArg, 0, len(values))
	for _, value := range values {
		fields := map[string]string{}
		for _, item := range strings.Split(value, ",") {
			key, field, ok := strings.Cut(item, "=")
			_, duplicate := fields[key]
			if !ok || duplicate || (key != "purpose" && key != "path" && key != "required") {
				return nil, fmt.Errorf("--scratch must be purpose=P,path=REL[,required=true|false]")
			}
			fields[key] = field
		}
		requiredValue, requiredSupplied := fields["required"]
		if fields["purpose"] == "" || fields["path"] == "" || (requiredSupplied && requiredValue != "true" && requiredValue != "false") {
			return nil, fmt.Errorf("--scratch must be purpose=P,path=REL[,required=true|false]")
		}
		required, _ := strconv.ParseBool(requiredValue)
		result = append(result, steward.ScratchArg{Purpose: fields["purpose"], Path: fields["path"], Required: required})
	}
	return result, nil
}

func runContextVerify(args []string) int {
	flags := flag.NewFlagSet("context verify", flag.ContinueOnError)
	root := flags.String("root", "", "installation or containing template root")
	nonce := flags.String("nonce", "", "handoff nonce")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" || *nonce == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem context verify --root ROOT --nonce NONCE")
		return 2
	}
	stateRoot, err := goal.ResolveStateRoot(*root)
	if err != nil {
		return contextVerbError("verify", err)
	}
	digest, err := steward.VerifyHandoffState(stateRoot, *nonce)
	if err != nil {
		return contextVerbError("verify", err)
	}
	fmt.Printf("ok sha256=%s\n", digest)
	return 0
}

func runContextPrune(args []string) int {
	flags := flag.NewFlagSet("context prune", flag.ContinueOnError)
	root := flags.String("root", "", "installation or containing template root")
	olderThanValue := flags.String("older-than", "14d", "positive duration or integer day count")
	if flags.Parse(args) != nil {
		return 2
	}
	olderThan, err := parseContextDuration(*olderThanValue)
	if *root == "" || flags.NArg() != 0 || err != nil {
		fmt.Fprintln(os.Stderr, "usage: metasystem context prune --root ROOT [--older-than 14d]")
		return 2
	}
	stateRoot, err := goal.ResolveStateRoot(*root)
	if err != nil {
		return contextVerbError("prune", err)
	}
	result, pruneErr := steward.PruneContext(stateRoot, olderThan, time.Now().UTC())
	fmt.Printf("pruned call-sessions=%d\n", result.CallSessions)
	for _, path := range result.Handoffs {
		fmt.Printf("pruned handoff=%s\n", path)
	}
	if pruneErr != nil {
		return contextVerbError("prune", pruneErr)
	}
	return 0
}

func parseContextDuration(value string) (time.Duration, error) {
	if duration, err := time.ParseDuration(value); err == nil && duration > 0 {
		return duration, nil
	}
	if len(value) < 2 || value[len(value)-1] != 'd' {
		return 0, fmt.Errorf("duration must be positive")
	}
	days, err := strconv.ParseUint(value[:len(value)-1], 10, 64)
	maxDays := uint64((time.Duration(1<<63 - 1)) / (24 * time.Hour))
	if err != nil || days == 0 || days > maxDays {
		return 0, fmt.Errorf("duration must be positive")
	}
	return time.Duration(days) * 24 * time.Hour, nil
}

func contextVerbError(verb string, err error) int {
	var refusal *steward.HandoffRefusal
	if errors.As(err, &refusal) {
		fmt.Fprintln(os.Stderr, strings.NewReplacer("\r", " ", "\n", " ").Replace(refusal.Error()))
		return 9
	}
	fmt.Fprintf(os.Stderr, "metasystem context %s: %v\n", verb, err)
	return 1
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
