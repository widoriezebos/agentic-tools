package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/adapter"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/events"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/report"
	metarun "github.com/widoriezebos/agentic-tools/metasystem/internal/run"
	usagecore "github.com/widoriezebos/agentic-tools/metasystem/internal/usage"
)

var waitCallerPID = func() int64 { return int64(os.Getppid()) }
var waitBootClock = identity.BootClock
var waitPathStat = metarun.PathStat(os.Stat)
var errChannelPollNotDue = errors.New("channel provider poll is not due")
var waitOpenWorkSignature = report.OpenWorkSignature

const waitRegisteredFDEnvironment = "METASYSTEM_WAIT_REGISTERED_FD"

var waitCommandEvents = &events.Emitter{Component: "run", Pid: int64(os.Getpid())}

func emitWaitCommandEvent(root, event, summary string, fields map[string]string) error {
	if err := waitCommandEvents.EmitChecked(root, event, summary, fields); err != nil {
		return err
	}
	if event != "wait-registered" || os.Getenv(waitRegisteredFDEnvironment) == "" {
		return nil
	}
	descriptor, err := strconv.Atoi(os.Getenv(waitRegisteredFDEnvironment))
	if err != nil || descriptor < 3 {
		return fmt.Errorf("%s must name an inherited descriptor", waitRegisteredFDEnvironment)
	}
	ready := os.NewFile(uintptr(descriptor), "wait-registered")
	if ready == nil {
		return fmt.Errorf("%s descriptor is unavailable", waitRegisteredFDEnvironment)
	}
	if _, err := fmt.Fprintln(ready, fields["waitId"]); err != nil {
		_ = ready.Close()
		return err
	}
	return ready.Close()
}

var waitCurrentHolder = func(ctx context.Context, root string) (lease.CurrentHolderView, error) {
	type result struct {
		holder lease.CurrentHolderView
		err    error
	}
	done := make(chan result, 1)
	go func() {
		holder, err := lease.CurrentHolder(root)
		done <- result{holder: holder, err: err}
	}()
	select {
	case <-ctx.Done():
		return lease.CurrentHolderView{}, ctx.Err()
	case answer := <-done:
		return answer.holder, answer.err
	}
}

var waitAdapterPathForRuntime = func(root, runtimeName string) (string, error) {
	installation, err := upMetasystemRoot("")
	if err != nil {
		installation = root
	}
	adapterPath := filepath.Join(installation, "scripts", "agents", "adapters", runtimeName+".sh")
	if _, err := os.Stat(adapterPath); err != nil {
		return "", fmt.Errorf("wait delivery adapter %s is unavailable", runtimeName)
	}
	return adapterPath, nil
}

func runWait(args []string) int {
	if len(args) > 0 && args[0] == "register" {
		return runWaitRegister(args[1:])
	}
	if len(args) > 0 && args[0] == "end" {
		return runWaitEnd(args[1:])
	}
	if len(args) > 0 && args[0] == "notify" {
		return runWaitNotify(args[1:])
	}
	if len(args) > 0 && args[0] == "measure" {
		return runWaitMeasure(args[1:])
	}
	return runWaitWithPoll(args, nil)
}

func runWaitMeasure(args []string) int {
	flags := flag.NewFlagSet("wait measure", flag.ContinueOnError)
	root := pathFlag(flags, "root", ".", "checkout or installation state root")
	sinceText := flags.String("since", "", "include registrations at or after this RFC3339 stamp")
	untilText := flags.String("until", "", "read records no later than this RFC3339 stamp")
	runtimeName := flags.String("runtime", "", "include only this runtime")
	jsonOutput := flags.Bool("json", false, "print the typed report as JSON")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return metarun.ExitInvalidWait
	}
	parseBound := func(name, value string) (time.Time, bool) {
		if value == "" {
			return time.Time{}, true
		}
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			fmt.Fprintf(os.Stderr, "wait measure --%s must be RFC3339\n", name)
			return time.Time{}, false
		}
		return parsed, true
	}
	since, sinceOK := parseBound("since", *sinceText)
	until, untilOK := parseBound("until", *untilText)
	if !sinceOK || !untilOK || (!since.IsZero() && !until.IsZero() && until.Before(since)) {
		if sinceOK && untilOK {
			fmt.Fprintln(os.Stderr, "wait measure --until must not precede --since")
		}
		return metarun.ExitInvalidWait
	}
	stateRoot, err := goal.ResolveStateRoot(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return metarun.ExitWaiterIO
	}
	measurement, err := usagecore.MeasureWaits(stateRoot, usagecore.WaitMeasureOptions{Since: since, Until: until, Runtime: *runtimeName})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return metarun.ExitWaiterIO
	}
	if *jsonOutput {
		encoded, _ := json.Marshal(measurement)
		fmt.Println(string(encoded))
	} else {
		fmt.Print(usagecore.FormatWaitMeasurement(measurement))
	}
	return usagecore.WaitMeasurementExit(measurement)
}

func runWaitWithPoll(args []string, poll func(context.Context) error) int {
	return runWaitCommand(args, poll, waitCallerPID(), printWaitResult)
}

func runWaitCommand(args []string, poll func(context.Context) error, callerPID int64, printResult func(metarun.WaitResult, bool)) int {
	flags := flag.NewFlagSet("wait", flag.ContinueOnError)
	root := pathFlag(flags, "root", ".", "checkout or installation state root")
	job := flags.String("job", "", "delegate job identifier")
	runID := flags.String("run", "", "tracked run identifier")
	attempt := flags.String("attempt", "", "proof attempt identifier")
	goalID := flags.String("goal", "", "goal identifier")
	path := flags.String("path", "", "absolute path to observe")
	until := flags.String("until", "", "path condition: present or absent")
	resume := flags.String("resume", "", "durable wait identifier")
	event := flags.String("event", "", "goal event: landing or human-act")
	after := flags.String("after", "", "accepted commit before the event")
	verb := flags.String("verb", "", "human-act verb")
	question := flags.String("question", "", "question identifier for an answer")
	chain := flags.String("chain", "", "landing delegate-chain root")
	timeout := flags.Duration("timeout", 24*time.Hour, "positive wait deadline, at most 24h")
	jsonOutput := flags.Bool("json", false, "print the typed result as JSON")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return metarun.ExitInvalidWait
	}
	timeoutExplicit := false
	flags.Visit(func(item *flag.Flag) {
		if item.Name == "timeout" {
			timeoutExplicit = true
		}
	})
	selectors := 0
	for _, value := range []string{*job, *runID, *attempt, *goalID, *path, *resume} {
		if value != "" {
			selectors++
		}
	}
	if selectors != 1 {
		fmt.Fprintln(os.Stderr, "wait requires exactly one of --job, --run, --attempt, --goal, --path, or --resume")
		return metarun.ExitInvalidWait
	}
	if *resume == "" && *path == "" && *until != "" {
		fmt.Fprintln(os.Stderr, "wait --until requires --path")
		return metarun.ExitInvalidWait
	}
	if *timeout <= 0 || *timeout > 24*time.Hour {
		fmt.Fprintln(os.Stderr, "wait timeout must be positive and no longer than 24 hours")
		return metarun.ExitInvalidWait
	}
	selector := metarun.WaitSelector{}
	if *resume == "" {
		switch {
		case *job != "":
			selector = metarun.WaitSelector{Kind: "job", TargetID: *job}
		case *runID != "":
			selector = metarun.WaitSelector{Kind: "run", TargetID: *runID}
		case *attempt != "":
			selector = metarun.WaitSelector{Kind: "attempt", TargetID: *attempt}
		case *goalID != "":
			selector = metarun.WaitSelector{Kind: "goal", TargetID: *goalID, GoalID: *goalID, Event: *event, After: *after, Verb: *verb, Question: *question, Chain: *chain}
			if poll != nil {
				selector.Poll = "channel"
			}
		case *path != "":
			selector = metarun.WaitSelector{Kind: "path", TargetID: metarun.PathWaitTargetID(*path), Path: *path, Until: *until}
		}
		if err := metarun.ValidateWaitSelector(selector); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return metarun.ExitInvalidWait
		}
	} else {
		if *job != "" || *runID != "" || *attempt != "" || *goalID != "" || *path != "" || *until != "" || *event != "" || *after != "" || *verb != "" || *question != "" || *chain != "" {
			fmt.Fprintln(os.Stderr, "--resume accepts no replacement target, cursor, or event selector")
			return metarun.ExitInvalidWait
		}
		if !metarun.ValidWaitID(*resume) {
			fmt.Fprintln(os.Stderr, "wait identifier is invalid")
			return metarun.ExitInvalidWait
		}
	}
	stateRoot, err := goal.ResolveStateRoot(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return metarun.ExitWaiterIO
	}
	resolved, code, err := resolveWaitCaller(stateRoot, callerPID)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return code
	}
	view, owner, runtimeSession := resolved.view, resolved.owner, resolved.runtimeSession
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var resumeRow metarun.Waiter
	if *resume != "" {
		row, _, loadErr := metarun.FindWaiterByID(stateRoot, *resume)
		if loadErr != nil {
			result := metarun.WaitResult{SchemaVersion: 2, WaitID: *resume, ExitCode: metarun.ExitNoRecord, Reason: loadErr.Error(), SourceOutcome: "missing-registration", ReturnedAt: time.Now().UTC().Format(time.RFC3339Nano)}
			printResult(result, *jsonOutput)
			return result.ExitCode
		}
		resumeRow = row
		selector = row.Selector
		if selector.Poll == "channel" && poll == nil {
			fmt.Fprintf(os.Stderr, "this wait belongs to channel recovery; run %s\n", metarun.WaitResumeCommand(row))
			return metarun.ExitWaiterBusy
		}
		if selector.Poll != "channel" && poll != nil {
			fmt.Fprintln(os.Stderr, "channel wait cannot resume a wait that has no channel poll selector")
			return metarun.ExitInvalidWait
		}
	}
	options, err := waitOptions(stateRoot, selector, owner, view.Announcement.Runtime, poll)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return metarun.ExitWaiterIO
	}
	registerFresh := false
	if *resume != "" && view.ClaimEpoch != nil {
		lineage, succeeded, successionErr := report.SucceededWaitOwner(stateRoot, view.MainId, *view.ClaimEpoch, resumeRow.MainId)
		if successionErr != nil {
			fmt.Fprintln(os.Stderr, "wait owner succession could not be verified:", successionErr)
			return metarun.ExitWaiterIO
		}
		if succeeded {
			options.SucceededMainID = resumeRow.MainId
			options.SucceededOwnerLineage = lineage
			registerFresh = resumeRow.Session != runtimeSession || resumeRow.RuntimeSession != runtimeSession
		}
	}
	store := &metarun.Store{Root: stateRoot}
	var result metarun.WaitResult
	if *resume != "" && !registerFresh {
		renewal := time.Duration(0)
		if timeoutExplicit {
			renewal = *timeout
		}
		result = store.ResumeWait(ctx, *resume, owner, runtimeSession, renewal, options)
	} else {
		scanBudget := *timeout
		if scanBudget > 10*time.Second {
			scanBudget = 10 * time.Second
		}
		scanCtx, cancel := context.WithTimeout(ctx, scanBudget)
		openWorkSignature, scanErr := options.OpenWorkSignature(scanCtx)
		cancel()
		if scanErr != nil {
			if ctx.Err() != nil {
				result = metarun.WaitResult{SchemaVersion: 2, ExitCode: metarun.ExitInterrupted, Reason: "wait command was interrupted", SourceOutcome: "interrupted", ReturnedAt: time.Now().UTC().Format(time.RFC3339Nano)}
			} else if *timeout <= 10*time.Second && errors.Is(scanErr, context.DeadlineExceeded) {
				result = metarun.WaitResult{SchemaVersion: 2, ExitCode: metarun.ExitWaitDeadline, Reason: "this wait reached its deadline during the open-work scan", SourceOutcome: "wait-deadline", ReturnedAt: time.Now().UTC().Format(time.RFC3339Nano)}
			} else {
				result = metarun.WaitResult{SchemaVersion: 2, ExitCode: metarun.ExitWaiterIO, Reason: "the open-work signature could not be read: " + scanErr.Error(), SourceOutcome: "storage-failure", ReturnedAt: time.Now().UTC().Format(time.RFC3339Nano)}
			}
		} else {
			result = store.Wait(ctx, metarun.WaitRequest{Selector: selector, Owner: owner, RuntimeSession: runtimeSession, Timeout: *timeout, OpenWorkSignature: openWorkSignature}, options)
		}
	}
	printResult(result, *jsonOutput)
	return result.ExitCode
}

func runWaitNotify(args []string) int {
	flags := flag.NewFlagSet("wait notify", flag.ContinueOnError)
	root := pathFlag(flags, "root", ".", "checkout or installation state root")
	job := flags.String("job", "", "delegate job identifier")
	attempt := flags.String("attempt", "", "proof attempt identifier")
	goalID := flags.String("goal", "", "goal identifier")
	beganBootNanos := flags.Int64("began-boot-nanos", 0, "owner boot-clock sample before the durable write")
	bootID := flags.String("boot-id", "", "boot identity for the pre-write sample")
	publicationID := flags.String("publication-id", "", "durable publication join identity")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return metarun.ExitInvalidWait
	}
	if *beganBootNanos < 0 || (*beganBootNanos > 0) != (*bootID != "") {
		fmt.Fprintln(os.Stderr, "wait notify needs a positive --began-boot-nanos together with --boot-id, or neither")
		return metarun.ExitInvalidWait
	}
	selector := metarun.WaitHint{}
	for _, candidate := range []struct{ kind, value string }{{"job", *job}, {"attempt", *attempt}, {"goal", *goalID}} {
		if candidate.value == "" {
			continue
		}
		if selector.Kind != "" {
			fmt.Fprintln(os.Stderr, "wait notify requires exactly one of --job, --attempt, or --goal")
			return metarun.ExitInvalidWait
		}
		selector = metarun.WaitHint{Kind: candidate.kind, TargetID: candidate.value, BeganBootNanos: *beganBootNanos, BeganBootID: *bootID, PublicationID: *publicationID}
	}
	if selector.Kind == "" {
		fmt.Fprintln(os.Stderr, "wait notify requires exactly one of --job, --attempt, or --goal")
		return metarun.ExitInvalidWait
	}
	stateRoot, err := goal.ResolveStateRoot(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return metarun.ExitWaiterIO
	}
	delivery, err := metarun.NotifyWaiters(stateRoot, selector)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return metarun.ExitInvalidWait
	}
	fmt.Printf("wait hints matched=%d delivered=%d\n", delivery.Matched, delivery.Delivered)
	return 0
}

func waitOptions(root string, selector metarun.WaitSelector, owner metarun.Caller, runtimeName string, poll func(context.Context) error) (metarun.WaitOptions, error) {
	adapterPath, err := waitAdapterPathForRuntime(root, runtimeName)
	if err != nil {
		return metarun.WaitOptions{}, err
	}
	openHintReceiver := metarun.OpenFIFOHintReceiver
	if selector.Kind == "path" {
		openHintReceiver = nil
	}
	observe := func(ctx context.Context, selected metarun.WaitSelector, pinned metarun.WaiterTarget, lastTip string) (metarun.SourceObservation, error) {
		var observation metarun.SourceObservation
		var observeErr error
		switch selected.Kind {
		case "job":
			observation, observeErr = dispatchcore.ObserveJob(ctx, root, selected.TargetID, pinned, lastTip)
		case "run":
			observation, observeErr = (&metarun.Store{Root: root}).ObserveRun(ctx, selected, pinned, lastTip)
		case "attempt":
			observation, observeErr = proofrun.ObserveAttempt(ctx, root, selected, pinned, lastTip)
		case "goal":
			observation, observeErr = goal.ObserveLedgerForWait(ctx, root, selected, pinned, lastTip, owner.OwnerLineage)
		case "path":
			observation, observeErr = metarun.ObservePath(ctx, selected, waitPathStat)
		default:
			return metarun.SourceObservation{}, fmt.Errorf("unknown wait source %q", selected.Kind)
		}
		if poll != nil && observation.Pending {
			pollErr := poll(ctx)
			if !errors.Is(pollErr, errChannelPollNotDue) {
				observation.PollAt = time.Now().UTC().Format(time.RFC3339Nano)
				if pollErr != nil {
					observation.PollError = pollErr.Error()
				}
			}
		}
		return observation, observeErr
	}
	openWorkSignature := func(ctx context.Context) (string, error) {
		return waitOpenWorkSignature(ctx, root)
	}
	actionable := func(ctx context.Context, row metarun.Waiter) (string, bool, error) {
		select {
		case <-ctx.Done():
			return "", false, ctx.Err()
		default:
		}
		holder, err := waitCurrentHolder(ctx, root)
		if err != nil {
			return "", false, fmt.Errorf("the checkout wait owner can no longer be verified: %w", err)
		}
		claimEpochChanged := row.ClaimEpoch == nil || holder.ClaimEpoch != *row.ClaimEpoch
		if holder.MainId != row.MainId || holder.OwnerLineage != row.OwnerLineage || claimEpochChanged {
			return "the checkout wait owner changed", true, nil
		}
		signature, err := waitOpenWorkSignature(ctx, root)
		if err != nil {
			return "", false, err
		}
		if signature != row.OpenWorkSignature {
			return "the open-work signature changed", true, nil
		}
		return "", false, nil
	}
	if selector.Kind == "path" {
		openWorkSignature = func(context.Context) (string, error) { return "", nil }
		actionable = nil
	}
	return metarun.WaitOptions{
		Runtime:           runtimeName,
		Observe:           observe,
		OpenHintReceiver:  openHintReceiver,
		OpenWorkSignature: openWorkSignature,
		BootClock:         waitBootClock,
		EmitEvent:         emitWaitCommandEvent,
		Deliver: func(ctx context.Context, waitID, nonce string, deadline time.Time, session string) (string, bool, error) {
			answer, err := adapter.DeliverWait(ctx, adapterPath, adapter.WaitDeliveryRequest{WaitID: waitID, Nonce: nonce, Deadline: deadline, Session: session})
			if errors.Is(err, adapter.ErrWaitDeliveryDeclined) {
				return "", true, nil
			}
			if err != nil {
				return "", false, fmt.Errorf("%s wait-delivery adapter failed: %w", runtimeName, err)
			}
			return answer, false, err
		},
		Actionable: actionable,
	}, nil
}

func printWaitResult(result metarun.WaitResult, jsonOutput bool) {
	if jsonOutput {
		encoded, err := json.Marshal(result)
		if err != nil {
			fmt.Fprintln(os.Stderr, "wait result could not be encoded:", err)
			return
		}
		fmt.Println(string(encoded))
		return
	}
	incarnation, _ := json.Marshal(result.TargetIncarnation)
	fmt.Printf("WAIT %s %s %s exit=%d outcome=%q reason=%q evidence=%q returnEventFailed=%t incarnation=%s\n",
		result.WaitID, result.Selector.Kind, result.Selector.TargetID, result.ExitCode,
		strings.ReplaceAll(result.SourceOutcome, "\n", " "), strings.ReplaceAll(result.Reason, "\n", " "),
		strings.ReplaceAll(result.SourceEvidence, "\n", " "), result.ReturnEventFailed, incarnation)
}

func runSessionStart(args []string) int {
	flags := flag.NewFlagSet("session start", flag.ContinueOnError)
	root := pathFlag(flags, "root", ".", "checkout root")
	session := flags.String("session", "", "runtime session identifier")
	if flags.Parse(args) != nil || *session == "" || flags.NArg() != 0 {
		return metarun.ExitInvalidWait
	}
	stateRoot, err := goal.ResolveStateRoot(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return metarun.ExitWaiterIO
	}
	holder, err := lease.CurrentHolder(stateRoot)
	if err != nil {
		if errors.Is(err, lease.ErrLeaseAbsent) {
			return metarun.ExitWaiterBusy
		}
		fmt.Fprintln(os.Stderr, err)
		return metarun.ExitWaiterIO
	}
	if holder.SessionId != *session {
		fmt.Fprintln(os.Stderr, "session start does not name the checkout holder's current session")
		return metarun.ExitWaiterBusy
	}
	lines, err := report.CurrentWaitingLines(stateRoot)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return metarun.ExitWaiterIO
	}
	for _, line := range lines {
		fmt.Println(line)
	}
	return 0
}
