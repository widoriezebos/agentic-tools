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
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
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

func runContextStatus(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("session handoff", stdout, stderr)
	root := pathFlag(flags, "root", "", "installation or containing template root")
	runtimeName := flags.String("runtime", "", "explicit runtime")
	session := flags.String("session", "", "explicit session")
	transcript := flags.String("transcript", "", "call transcript override")
	asJSON := flags.Bool("json", false, "print structured status")
	verbose := flags.Bool("verbose", false, "also the refusal's code and facts")
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
		return refusePassthrough(stderr, 2, "these options form no context budget reading (--runtime goes with --session); nothing was read",
			textui.Hint{Argv: []string{"metasystem", "session", "handoff", "--help"}, Reason: "the --status form and its options"})
	}
	if *runtimeName != "" {
		if _, ok := runtimes.Lookup(*runtimeName); !ok {
			return refusePassthrough(stderr, 1, "there is no runtime "+*runtimeName+"; nothing was read",
				textui.Hint{Argv: []string{"metasystem", "session", "handoff", "--status"}, Reason: "reads this session's own"})
		}
	}
	stateRoot, err := goal.ResolveStateRoot(*root)
	if err != nil {
		return contextVerbError(stderr, "status", err, *verbose)
	}
	// The thresholds and the call samples are the installation's
	// configuration and run state, wherever the state root lives.
	installation, err := installationFromRootFlag(*root)
	if err != nil {
		return contextVerbError(stderr, "status", err, *verbose)
	}
	role, reading, readErr := steward.ContextBudgetLine(installation, time.Now().UTC(), steward.ContextOptions{
		Runtime: *runtimeName, Session: *session, Transcript: *transcript,
	})
	window, windowErr := contextWindow(installation)
	if *asJSON {
		writeJSONLine(stdout, stderr, contextStatusOutput{Diagnostic: transcriptSupplied, Role: role, Reading: projectContextReading(reading), Window: window})
	} else {
		page := passthroughPage(stdout, stateRoot, *verbose)
		page.Headline(fmt.Sprintf("The context budget is %s: %s", role.Status, role.Reason))
		section := page.Section("", "")
		if role.Remedy != "" {
			section.KV("remedy", textui.Plain(role.Remedy))
		}
		if windowErr == nil {
			section.Text(windowLine(window))
		}
		printPage(stdout, page)
	}
	if windowErr != nil {
		return contextVerbError(stderr, "status", windowErr, *verbose)
	}
	if readErr != nil {
		return contextVerbError(stderr, "status", readErr, *verbose)
	}
	return 0
}

func contextWindow(installation stateroot.Installation) (contextWindowView, error) {
	confPath := installation.Path("metasystem.conf")
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
	budget, err := config.ContextBudget(installation)
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
	ceilingParams := config.GetParams{Key: config.ContextCeilingTokensKey, ConfPath: confPath}
	ceilingSource := "default"
	if confExists {
		ceilingSource, err = config.KeyOrigin(ceilingParams)
	}
	if err != nil {
		return contextWindowView{}, err
	}
	shippedSettings, err := shippedClaudeSettings()
	if err != nil {
		return contextWindowView{}, err
	}
	shipped, err := launch.LoadShippedSeatWindow(shippedSettings, settings.SeatWindow)
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

func runContextHandoff(args []string, stdout, stderr io.Writer) int {
	return runContextHandoffWithInputs(args, contextHandoffInputs{goal.ResolveMachine, goal.ReadClaimableBudgetedWork}, stdout, stderr)
}

type contextHandoffInputs struct {
	resolveMachine func(string) (string, error)
	readWork       func(string, time.Time) (goal.ClaimableBudgetedWork, error)
}

func runContextHandoffWithInputs(args []string, inputs contextHandoffInputs, stdout, stderr io.Writer) int {
	flags := newFlagSet("session handoff", stdout, stderr)
	root := pathFlag(flags, "root", "", "installation or containing template root")
	cancel := flags.String("cancel", "", "live handoff nonce to cancel")
	by := flags.String("by", "", "name of the attending human")
	note := flags.String("note", "", "lessons note in the runtime's configured directory")
	noDelegates := flags.Bool("no-delegates", false, "declare that no background task remains in flight")
	asJSON := flags.Bool("json", false, "print a bounded result")
	verbose := flags.Bool("verbose", false, "also a refusal's code and facts")
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
		reason, remedy, _ := strings.Cut(contextHandoffUsage, "\nrun: ")
		return refusePassthrough(stderr, 2, reason, textui.Hint{Argv: strings.Fields(remedy), Reason: "its forms and what each needs"})
	}
	stateRoot, err := goal.ResolveStateRoot(*root)
	if err != nil {
		return contextVerbError(stderr, "handoff", err, *verbose)
	}
	// Who is asking, the note directory and the transcript are read from the
	// installation; the handoff record and its proof stay with the state root.
	installation, err := installationFromRootFlag(*root)
	if err != nil {
		return contextVerbError(stderr, "handoff", err, *verbose)
	}
	if cancelSupplied {
		caller, err := contextHandoffCallerWithMachine(installation, stateRoot, inputs.resolveMachine)
		if err != nil {
			return contextVerbError(stderr, "handoff", err, *verbose)
		}
		canceller := steward.HandoffCanceller{Caller: caller}
		if bySupplied {
			act := &steward.HandoffHumanAct{By: *by}
			canceller.Human = act
			if caller.Class == lease.ClassHuman {
				act.Proof, _ = proveSessionStopHuman(stateRoot, int64(os.Getppid()), time.Now().UTC())
				holder, err := currentContextHandoffHolder(installation.Path())
				if err != nil {
					return contextVerbError(stderr, "handoff", err, *verbose)
				}
				act.HolderMainId, act.HolderSession, act.ClaimEpoch = holder.MainId, holder.SessionId, holder.ClaimEpoch
			}
		}
		err = steward.CancelHandoff(stateRoot, *cancel, canceller)
		var already *steward.HandoffAlreadyCancelled
		if errors.As(err, &already) {
			// The cancellation already held (R-129-ui).
			page := passthroughPage(stdout, stateRoot, *verbose)
			page.Headline("Handoff " + *cancel + " is already cancelled; nothing to do")
			printPage(stdout, page)
			return 0
		}
		if err != nil {
			return contextVerbError(stderr, "handoff", err, *verbose)
		}
		page := passthroughPage(stdout, stateRoot, *verbose)
		page.Done("Handoff " + *cancel + " cancelled")
		printPage(stdout, page)
		return 0
	}
	help := textui.Hint{Argv: []string{"metasystem", "session", "handoff", "--help"}, Reason: "its forms and what each needs"}
	scratch, err := parseContextScratch(scratchValues)
	if err != nil {
		return refusePassthrough(stderr, 2, err.Error()+"; nothing was handed off", help)
	}
	declarations, err := parseContextDelegates(delegateValues)
	if err != nil || (len(delegateValues) != 0 && *noDelegates) {
		if err == nil {
			err = fmt.Errorf("--delegate and --no-delegates cannot be combined")
		}
		return refusePassthrough(stderr, 2, err.Error()+"; nothing was handed off", help)
	}
	if *note == "" {
		return contextVerbError(stderr, "handoff", &steward.HandoffRefusal{Code: "HANDOFF_NOTE_MISSING"}, *verbose)
	}
	caller, err := contextHandoffCallerWithMachine(installation, stateRoot, inputs.resolveMachine)
	if err != nil {
		return contextVerbError(stderr, "handoff", err, *verbose)
	}
	caller, err = steward.ResolveHandoffCaller(stateRoot, caller)
	if err != nil {
		return contextVerbError(stderr, "handoff", err, *verbose)
	}
	toplevel := contextHandoffToplevel(stateRoot)
	readOptions := usagepkg.ReadOptions{Toplevel: toplevel, Installation: installation.Path()}
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
		return contextVerbError(stderr, "handoff", err, *verbose)
	}
	for _, notice := range notices {
		fmt.Fprintln(stderr, notice.Text)
	}
	now := contextHandoffNow()
	delegates, err := contextHandoffDelegates(declarations, *noDelegates, transcriptResolved, tasks, now)
	if err != nil {
		return contextVerbError(stderr, "handoff", err, *verbose)
	}
	noteDirectory, err := config.ContextHandoffNoteDirectory(installation, caller.Runtime, claudeMemory)
	if err != nil {
		return contextVerbError(stderr, "handoff", err, *verbose)
	}
	result, err := steward.HandoffWithWorkReader(stateRoot, caller, steward.HandoffRecord{
		Scratch: scratch, Delegates: delegates, NotePath: *note, NoteDirectory: noteDirectory,
	}, now, filepath.Join(stateRoot, "memory", "receipts.log"), inputs.readWork)
	if err != nil {
		return contextVerbError(stderr, "handoff", err, *verbose)
	}
	if *asJSON {
		writeJSONLine(stdout, stderr, contextHandoffOutput{Nonce: result.Nonce, StatePath: result.StatePath, Digest: result.StateDigest,
			IntentPath: result.IntentPath})
	} else {
		page := passthroughPage(stdout, stateRoot, *verbose)
		page.Done("Handoff recorded: " + result.Nonce)
		page.Facts(textui.KV{Key: "state", Value: []textui.Span{textui.Plain(page.Env().Path(result.StatePath)), textui.Dim("  sha256 " + result.StateDigest[:min(12, len(result.StateDigest))])}})
		printPage(stdout, page)
	}
	return 0
}

// contextHandoffUsage is the answer to a handoff whose options do not form
// one of its forms; the help page lists them.
const contextHandoffUsage = "a handoff takes --note with --no-delegates or --delegate, or --cancel alone; nothing was done\n" +
	"run: metasystem session handoff --help"

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
				return nil, errors.New("--delegate takes id=<id>[,asked=<text>][,output=<file>]")
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

// contextHandoffCallerWithMachine classifies the caller from the
// installation's announcements, custody records, adapters and steward
// intents, all run state; the machine name is the state root's.
func contextHandoffCallerWithMachine(installation stateroot.Installation, stateRoot string, resolveMachine func(string) (string, error)) (steward.HandoffCaller, error) {
	classified, err := classifyContextHandoffCaller(installation.Path(), installation.Path(), int64(os.Getppid()))
	if err != nil {
		return steward.HandoffCaller{}, fmt.Errorf("who is asking for the handoff cannot be told: %w", err)
	}
	machine, err := resolveMachine(stateRoot)
	if err != nil {
		return steward.HandoffCaller{}, err
	}
	caller := steward.HandoffCaller{Class: classified.Class, MainId: classified.MainId, Machine: machine}
	if classified.Class == lease.ClassDelegate {
		jobID, active, err := steward.ConsumedActiveJob(installation.Path())
		if err != nil || !active {
			return caller, err
		}
		delegate, err := hookContextHandoffDelegate(installation.Path(), installation.Path(), jobID, int64(os.Getppid()))
		if err == nil && delegate.Delegate && delegate.JobID == jobID {
			caller.JobId = jobID
		}
		return caller, err
	}
	if classified.Class != lease.ClassMain || classified.Announcement == nil {
		return caller, nil
	}
	holder, err := currentContextHandoffHolder(installation.Path())
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

func runContextVerify(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("session handoff", stdout, stderr)
	root := pathFlag(flags, "root", "", "installation or containing template root")
	nonce := flags.String("nonce", "", "handoff nonce")
	verbose := flags.Bool("verbose", false, "also a refusal's code and facts")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" || *nonce == "" || flags.NArg() != 0 {
		return refusePassthrough(stderr, 2, "verifying a handoff takes its nonce: --verify NONCE; nothing was verified",
			textui.Hint{Argv: []string{"metasystem", "session", "handoff", "--help"}, Reason: "its forms and what each needs"})
	}
	stateRoot, err := goal.ResolveStateRoot(*root)
	if err != nil {
		return contextVerbError(stderr, "verify", err, *verbose)
	}
	digest, err := steward.VerifyHandoffState(stateRoot, *nonce)
	if err != nil {
		return contextVerbError(stderr, "verify", err, *verbose)
	}
	page := passthroughPage(stdout, stateRoot, *verbose)
	page.Done("Handoff " + *nonce + " verified")
	page.Facts(textui.KV{Key: "state", Value: []textui.Span{textui.Plain("sha256 " + digest)}})
	printPage(stdout, page)
	return 0
}

// contextVerbError prints a context verb's refusal: a handoff refusal's
// code in words with the command that shows the forms, its code and facts
// only with --verbose; any other failure with the check that names what is
// wrong.
func contextVerbError(stderr io.Writer, verb string, err error, verbose bool) int {
	page := passthroughPage(stderr, "", verbose)
	var refusal *steward.HandoffRefusal
	if errors.As(err, &refusal) {
		words := strings.ToLower(strings.ReplaceAll(strings.TrimPrefix(refusal.Code, "HANDOFF_"), "_", " "))
		page.Refusal("the handoff was refused: "+words+"; nothing was done",
			textui.Hint{Argv: []string{"metasystem", "session", "handoff", "--help"}, Reason: "its forms and what each needs"})
		if verbose {
			page.Section("Details", "").Text(strings.NewReplacer("\r", " ", "\n", " ").Replace(refusal.Error()))
		}
		printPage(stderr, page)
		return 9
	}
	page.Refusal(fmt.Sprintf("%s stopped: %v", contextPublicName(verb), err), textui.Hint{Argv: []string{"metasystem", "system", "check"}, Reason: "names what is wrong here"})
	printPage(stderr, page)
	return 1
}

// contextPublicName is the spelling a person or agent runs for a context
// verb: the session actions for the three that have one.
func contextPublicName(verb string) string {
	switch verb {
	case "handoff":
		return "session handoff"
	case "status":
		return "session handoff --status"
	case "verify":
		return "session handoff --verify"
	}
	return "internal context " + verb
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
