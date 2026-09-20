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

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	usagepkg "github.com/widoriezebos/agentic-tools/metasystem/internal/usage"
)

type contextStatusOutput struct {
	Diagnostic bool                `json:"diagnostic"`
	Role       steward.RoleVerdict `json:"role"`
	Reading    contextReadingView  `json:"reading"`
	Window     contextWindowView   `json:"window"`
}

type contextWindowView struct {
	Tokens                 int64  `json:"tokens"`
	Key                    string `json:"key"`
	Source                 string `json:"source"`
	Ceiling                int64  `json:"ceiling"`
	CeilingKey             string `json:"ceilingKey"`
	CeilingSource          string `json:"ceilingSource"`
	CeilingAboveWindow     bool   `json:"ceilingAboveWindow"`
	Shipped                int64  `json:"shipped"`
	ShippedSource          string `json:"shippedSource"`
	ShippedDiffersFromConf bool   `json:"shippedDiffersFromConf"`
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
var contextHandoffNow = func() time.Time { return time.Now().UTC() }

func runContextStatus(args []string) int {
	flags := flag.NewFlagSet("context status", flag.ContinueOnError)
	root := pathFlag(flags, "root", "", "installation or containing template root")
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
	window, windowErr := contextWindow(stateRoot)
	if *asJSON {
		printJSON(contextStatusOutput{Diagnostic: transcriptSupplied, Role: role, Reading: projectContextReading(reading), Window: window})
	} else {
		fmt.Println(role.Line())
		if windowErr == nil {
			fmt.Println(windowLine(window))
		}
	}
	if windowErr != nil {
		fmt.Fprintln(os.Stderr, "metasystem context status:", windowErr)
		return 1
	}
	if readErr != nil {
		fmt.Fprintln(os.Stderr, "metasystem context status:", readErr)
		return 1
	}
	return 0
}

func contextWindow(root string) (contextWindowView, error) {
	confPath := filepath.Join(root, "metasystem.conf")
	settings := launch.DefaultSettings()
	confExists := true
	if _, statErr := os.Stat(confPath); os.IsNotExist(statErr) {
		confExists = false
	} else if statErr != nil {
		return contextWindowView{}, statErr
	}
	var err error
	if confExists {
		settings, err = launch.ResolveSettings(confPath, os.LookupEnv)
	}
	if err != nil {
		return contextWindowView{}, err
	}
	budget, err := config.ContextBudget(root)
	if err != nil {
		return contextWindowView{}, err
	}
	windowSource := "default"
	for _, value := range settings.Values {
		if value.Key == launch.SeatWindowKey {
			windowSource = value.Source
			break
		}
	}
	ceilingParams := config.GetParams{Key: config.ContextCeilingTokensKey, ConfPath: confPath, Default: strconv.FormatInt(config.DefaultContextCeilingTokens, 10), DefaultSet: true}
	ceilingSource := "default"
	if confExists {
		ceilingSource, err = config.KeyOrigin(ceilingParams)
	}
	if err != nil {
		return contextWindowView{}, err
	}
	shipped, err := launch.LoadShippedSeatWindow(root, settings.SeatWindow)
	if err != nil {
		return contextWindowView{}, err
	}
	// ceiling-above-window warns that the handoff ceiling sits past the point
	// where the harness compacts. With no window imposed, that point belongs to
	// the runtime and this comparison has nothing to compare, so it must not fire
	// on every root just because the configured number is 0.
	return contextWindowView{Tokens: settings.SeatWindow, Key: launch.SeatWindowKey, Source: windowSource, Ceiling: budget.Ceiling, CeilingKey: config.ContextCeilingTokensKey, CeilingSource: ceilingSource, CeilingAboveWindow: settings.SeatWindow > 0 && budget.Ceiling > settings.SeatWindow, Shipped: shipped.Tokens, ShippedSource: shipped.Source, ShippedDiffersFromConf: shipped.DiffersFromConf}, nil
}

func windowLine(window contextWindowView) string {
	size := fmt.Sprintf("%d tokens", window.Tokens)
	if window.Tokens == 0 {
		size = "none imposed, the runtime's own"
	}
	line := fmt.Sprintf("window: %s (%s, %s); ceiling: %d tokens (%s, %s)", size, window.Key, window.Source, window.Ceiling, window.CeilingKey, window.CeilingSource)
	if window.CeilingAboveWindow {
		line += "; ceiling-above-window"
	}
	if window.Shipped == 0 {
		line += "; shipped: none (claude-code-hooks.json)"
	} else {
		line += fmt.Sprintf("; shipped: %d (claude-code-hooks.json)", window.Shipped)
	}
	if window.ShippedDiffersFromConf {
		line += " shipped-differs-from-conf"
	}
	return line
}

func runContextReport(args []string) int {
	flags := flag.NewFlagSet("context report", flag.ContinueOnError)
	root := pathFlag(flags, "root", "", "installation or containing template root")
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
	root := pathFlag(flags, "root", "", "installation or containing template root")
	cancel := flags.String("cancel", "", "live handoff nonce to cancel")
	by := flags.String("by", "", "name of the attending human")
	note := flags.String("note", "", "lessons note in the runtime's configured directory")
	noDelegates := flags.Bool("no-delegates", false, "declare that no background task remains in flight")
	asJSON := flags.Bool("json", false, "print a bounded result")
	var scratchValues []string
	var delegateValues []string
	flags.Func("scratch", "purpose=P,path=REL[,required=true|false]", func(value string) error {
		scratchValues = append(scratchValues, value)
		return nil
	})
	flags.Func("delegate", "id=ID[,asked=TEXT][,output=PATH]", func(value string) error {
		delegateValues = append(delegateValues, value)
		return nil
	})
	if flags.Parse(args) != nil {
		return 2
	}
	cancelSupplied, bySupplied, noteSupplied, noDelegatesSupplied := false, false, false, false
	flags.Visit(func(option *flag.Flag) {
		cancelSupplied = cancelSupplied || option.Name == "cancel"
		bySupplied = bySupplied || option.Name == "by"
		noteSupplied = noteSupplied || option.Name == "note"
		noDelegatesSupplied = noDelegatesSupplied || option.Name == "no-delegates"
	})
	if *root == "" || flags.NArg() != 0 || (cancelSupplied && (*cancel == "" || len(scratchValues) != 0 || len(delegateValues) != 0 || noteSupplied || noDelegatesSupplied)) ||
		(bySupplied && (!cancelSupplied || strings.TrimSpace(*by) == "")) {
		fmt.Fprintln(os.Stderr, contextHandoffUsage)
		return 2
	}
	stateRoot, err := goal.ResolveStateRoot(*root)
	if err != nil {
		return contextVerbError("handoff", err)
	}
	if cancelSupplied {
		caller, err := contextHandoffCaller(stateRoot)
		if err != nil {
			return contextVerbError("handoff", err)
		}
		canceller := steward.HandoffCanceller{Caller: caller}
		if bySupplied {
			act := &steward.HandoffHumanAct{By: *by}
			canceller.Human = act
			if caller.Class == lease.ClassHuman {
				act.Proof, _ = proveSessionStopHuman(stateRoot, int64(os.Getppid()), time.Now().UTC())
				holder, err := currentContextHandoffHolder(stateRoot)
				if err != nil {
					return contextVerbError("handoff", err)
				}
				act.HolderMainId, act.HolderSession, act.ClaimEpoch = holder.MainId, holder.SessionId, holder.ClaimEpoch
			}
		}
		if err := steward.CancelHandoff(stateRoot, *cancel, canceller); err != nil {
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
	declarations, err := parseContextDelegates(delegateValues)
	if err != nil || (len(delegateValues) != 0 && *noDelegates) {
		if err == nil {
			err = fmt.Errorf("--delegate and --no-delegates cannot be combined")
		}
		fmt.Fprintln(os.Stderr, "metasystem context handoff:", err)
		return 2
	}
	if *note == "" {
		return contextVerbError("handoff", &steward.HandoffRefusal{Code: "HANDOFF_NOTE_MISSING"})
	}
	caller, err := contextHandoffCaller(stateRoot)
	if err != nil {
		return contextVerbError("handoff", err)
	}
	caller, err = steward.ResolveHandoffCaller(stateRoot, caller)
	if err != nil {
		return contextVerbError("handoff", err)
	}
	toplevel := contextHandoffToplevel(stateRoot)
	readOptions := usagepkg.ReadOptions{Toplevel: toplevel, Installation: stateRoot}
	transcript, transcriptResolved := "", false
	claudeMemory := ""
	if caller.Runtime == "claude" {
		if candidate, reason := usagepkg.ClaudeTranscript(caller.Session, readOptions); reason == "" {
			transcript, transcriptResolved = candidate, true
		}
		if candidate, reason := usagepkg.MemoryDirectory(readOptions); reason == "" {
			claudeMemory = candidate
		}
	}
	tasks, notices, err := usagepkg.InFlightTasks(transcript, readOptions)
	if err != nil {
		return contextVerbError("handoff", err)
	}
	for _, notice := range notices {
		fmt.Fprintln(os.Stderr, notice.Text)
	}
	now := contextHandoffNow()
	delegates, err := contextHandoffDelegates(declarations, *noDelegates, transcriptResolved, tasks, now)
	if err != nil {
		return contextVerbError("handoff", err)
	}
	noteDirectory, err := config.ContextHandoffNoteDirectory(stateRoot, caller.Runtime, claudeMemory)
	if err != nil {
		return contextVerbError("handoff", err)
	}
	result, err := steward.Handoff(stateRoot, caller, steward.HandoffRecord{
		Scratch: scratch, Delegates: delegates, NotePath: *note, NoteDirectory: noteDirectory,
	}, now, filepath.Join(stateRoot, "memory", "receipts.log"))
	if err != nil {
		return contextVerbError("handoff", err)
	}
	if *asJSON {
		printJSON(contextHandoffOutput{Nonce: result.Nonce, StatePath: result.StatePath, Digest: result.StateDigest,
			IntentPath: result.IntentPath})
	} else {
		fmt.Printf("handoff recorded: %s state=%s sha256=%s\n", result.Nonce, result.StatePath, result.StateDigest)
	}
	return 0
}

const contextHandoffUsage = "usage: metasystem context handoff --root ROOT --note PATH (--no-delegates | --delegate id=ID[,asked=TEXT][,output=PATH]...) [--scratch purpose=P,path=REL[,required=true|false]]... [--json] | --root ROOT --cancel NONCE [--by HUMAN]"

type contextDelegateArg struct {
	id, asked, output   string
	askedSet, outputSet bool
}

func parseContextDelegates(values []string) ([]contextDelegateArg, error) {
	result := make([]contextDelegateArg, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		fields := map[string]string{}
		for _, item := range strings.Split(value, ",") {
			key, field, ok := strings.Cut(item, "=")
			_, duplicate := fields[key]
			if !ok || duplicate || (key != "id" && key != "asked" && key != "output") {
				return nil, fmt.Errorf("--delegate must be id=ID[,asked=TEXT][,output=PATH]")
			}
			fields[key] = field
		}
		id := strings.TrimSpace(fields["id"])
		if id == "" || seen[id] {
			return nil, fmt.Errorf("--delegate must name one unique non-empty id")
		}
		seen[id] = true
		_, askedSet := fields["asked"]
		_, outputSet := fields["output"]
		result = append(result, contextDelegateArg{id: id, asked: fields["asked"], output: fields["output"], askedSet: askedSet, outputSet: outputSet})
	}
	return result, nil
}

func contextHandoffDelegates(declarations []contextDelegateArg, noDelegates, transcriptResolved bool, tasks []usagepkg.Task, now time.Time) ([]steward.HandoffDelegate, error) {
	if noDelegates {
		var ids []string
		for _, task := range tasks {
			if task.InFlight {
				ids = append(ids, task.ID)
			}
		}
		if len(ids) > 0 {
			return nil, &steward.HandoffRefusal{Code: "HANDOFF_TASKS_IN_FLIGHT", Detail: "ids=" + strings.Join(ids, ",")}
		}
		return nil, nil
	}
	if len(declarations) == 0 {
		return nil, &steward.HandoffRefusal{Code: "HANDOFF_DELEGATES_UNDECLARED"}
	}
	byID := make(map[string]usagepkg.Task, len(tasks))
	for _, task := range tasks {
		byID[task.ID] = task
	}
	declared := make(map[string]bool, len(declarations))
	result := make([]steward.HandoffDelegate, 0, len(declarations))
	for _, declaration := range declarations {
		delegate := steward.HandoffDelegate{ID: declaration.id, Asked: declaration.asked, Output: declaration.output,
			DeclaredAt: now.Format(time.RFC3339Nano)}
		if transcriptResolved {
			task, known := byID[declaration.id]
			if !known {
				return nil, &steward.HandoffRefusal{Code: "HANDOFF_DELEGATE_UNKNOWN", Detail: "id=" + declaration.id}
			}
			if declaration.outputSet && declaration.output != task.Output {
				return nil, &steward.HandoffRefusal{Code: "HANDOFF_DELEGATE_OUTPUT_MISMATCH", Detail: "id=" + declaration.id}
			}
			if !declaration.askedSet {
				delegate.Asked = task.Asked
			}
			if !declaration.outputSet {
				delegate.Output = task.Output
			}
			delegate.ToolUseID, delegate.Kind, delegate.Terminal = task.ToolUseID, task.Kind, !task.InFlight
		}
		var missing []string
		if delegate.Asked == "" {
			missing = append(missing, "asked")
		}
		if delegate.Output == "" {
			missing = append(missing, "output")
		}
		if len(missing) > 0 {
			return nil, &steward.HandoffRefusal{Code: "HANDOFF_DELEGATE_INCOMPLETE", Detail: "id=" + declaration.id + " missing=" + strings.Join(missing, ",")}
		}
		declared[declaration.id] = true
		result = append(result, delegate)
	}
	if transcriptResolved {
		for _, task := range tasks {
			if task.InFlight && !declared[task.ID] {
				return nil, &steward.HandoffRefusal{Code: "HANDOFF_DELEGATE_UNDECLARED", Detail: "id=" + task.ID}
			}
		}
	}
	return result, nil
}

func contextHandoffToplevel(root string) string {
	current := filepath.Clean(root)
	for {
		if info, err := os.Stat(filepath.Join(current, ".git")); err == nil && (info.IsDir() || info.Mode().IsRegular()) {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			return root
		}
		current = parent
	}
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
	root := pathFlag(flags, "root", "", "installation or containing template root")
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
	root := pathFlag(flags, "root", "", "installation or containing template root")
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
