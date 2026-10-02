package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/brain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel/phase"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	metarun "github.com/widoriezebos/agentic-tools/metasystem/internal/run"
)

func channelIdentity(root string) (string, string, error) {
	return channelIdentityWithMachine(root, nil)
}

func channelIdentityWithMachine(root string, resolveMachine func(string) (string, error)) (string, string, error) {
	return channelIdentityFor(root, resolveMachine, os.Getenv("METASYSTEM_OWNER_LINEAGE"))
}

// channelIdentityFor is the channel ledger identity under an explicit
// lineage, the one an owner call carries.
func channelIdentityFor(root string, resolveMachine func(string) (string, error), lin string) (string, string, error) {
	if resolveMachine == nil {
		resolveMachine = goal.ResolveMachine
	}
	m, err := resolveMachine(root)
	if err != nil {
		return "", "", err
	}
	if lin == "" {
		return "", "", errors.New("the channel is changed only from an agent session, and this shell names none")
	}
	return m, lin, nil
}

func channelPollContext(root string) (context.Context, context.CancelFunc, error) {
	raw, err := phase.Get(root, "channel.poll-timeout-sec", "15")
	if err != nil {
		return nil, nil, err
	}
	seconds, err := strconv.ParseInt(raw, 10, 64)
	const maxSeconds = int64(^uint64(0)>>1) / int64(time.Second)
	if err != nil || seconds < 1 || seconds > maxSeconds {
		return nil, nil, fmt.Errorf("channel.poll-timeout-sec must be a positive integer of seconds")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(seconds)*time.Second)
	return ctx, cancel, nil
}

// postLanded tells the channel at root of a landing on main by its plain
// sentences of what was delivered, once per sha (Decision 7 of the
// blocked-agent-asks-the-human design). No sentence or no channel
// configured is nothing to do; the error is a failed post, kept for one
// retry, and never fails the landing.
func postLanded(root, text, sha string, now time.Time) error {
	ctx, cancel, err := channelPollContext(root)
	if err != nil {
		return err
	}
	defer cancel()
	return phase.NotifyLanded(ctx, root, text, sha, now)
}

// channelStatus composes the checkout's status report, printing it and, with
// post, publishing it to the configured channel and the brain's status.
func channelStatus(checkout string, postNow bool, stdout, stderr io.Writer, resolveMachine func(string) (string, error), resolveEndpoint func(string) (goal.Endpoint, error), landingLog func(string, time.Time) ([]byte, error)) int {
	root, post := &checkout, &postNow
	now, err := goalCommandNow(*root)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if resolveMachine == nil {
		resolveMachine = goal.ResolveMachine
	}
	machine, err := resolveMachine(*root)
	if err != nil {
		machine = "this machine"
	}
	config := channel.ReportConfig{RepoRoot: *root, Machine: machine, Now: now}
	var text, approvalGoal string
	var endpoint goal.Endpoint
	if resolveEndpoint == nil && landingLog == nil {
		text, approvalGoal, err = channel.ComposeStatusReport(config)
	} else {
		var resolveErr error
		endpoint, resolveErr = resolveEndpoint(*root)
		if resolveErr != nil {
			fmt.Fprintln(stderr, resolveErr)
			return 1
		}
		text, approvalGoal, err = channel.ComposeStatusReportAtEndpoint(config, endpoint, landingLog)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, text)
	if *post {
		l, e := phase.Load(*root, false)
		if e != nil {
			fmt.Fprintln(stderr, e)
			return 1
		}
		if l.Provider == nil {
			return 0
		}
		ctx, cancel, e := channelPollContext(*root)
		if e != nil {
			fmt.Fprintln(stderr, e)
			return 1
		}
		defer cancel()
		ref, e := l.Provider.Post(ctx, l.Destination, text, nil)
		if e != nil {
			fmt.Fprintln(stderr, e)
			return 1
		}
		state := channel.StatusState{LastPost: now.UTC(), ContentDigest: channel.Digest(text), Ref: ref, GoalID: approvalGoal}
		if e = channel.SaveStatusState(*root, state); e != nil {
			fmt.Fprintln(stderr, e)
			return 1
		}
		ledgerIdentity := ""
		if resolveEndpoint == nil {
			ledgerIdentity = goal.ExistingLedgerIdentity(*root)
		} else {
			ledgerIdentity = goal.ExistingLedgerIdentityAtEndpoint(endpoint)
		}
		brainState := brain.Read(*root, ledgerIdentity)
		if brainState.State == brain.Declared {
			postedAt := now.UTC()
			if e = brain.WriteStatus(*root, *brainState.Record, postedAt); e != nil {
				fmt.Fprintln(stderr, e)
				return 1
			}
			if e = brain.MarkStatusPosted(*root, postedAt); e != nil {
				fmt.Fprintln(stderr, e)
				return 1
			}
		}
	}
	return 0
}

// channelAskInput is one question as its asker states it; Options are
// "label: consequence".
type channelAskInput struct {
	// About is what a question without a goal is about: lane or machine.
	Goal, About, Kind, Recommendation, Wants string
	Facts, Options                           []string
	Budget                                   *goal.Budget
}

// askChannelQuestion opens one durable question thread through the
// configured channel. Warnings are reported problems the ask continued past.
func askChannelQuestion(root string, in channelAskInput) (channel.Question, []string, int, error) {
	return askChannelQuestionVia(root, in, channelAskSurface{identity: channelIdentity, load: func(root string) (phase.Loaded, error) { return phase.Load(root, false) }, cursor: goal.AcceptedLedgerTip})
}

// channelAskSurface is who asks and through which configured channel; tests
// give one ask its own identity and transport.
type channelAskSurface struct {
	identity func(root string) (string, string, error)
	load     func(root string) (phase.Loaded, error)
	cursor   func(root string) (string, bool, error)
}

// askChannelQuestionVia is askChannelQuestion through a given surface. A
// question the owner recorded before a later step failed is returned with
// the error, so the caller can still name it.
func askChannelQuestionVia(root string, in channelAskInput, surface channelAskSurface) (channel.Question, []string, int, error) {
	machine, lineage, err := surface.identity(root)
	if err != nil {
		return channel.Question{}, nil, 1, err
	}
	opts := []channel.Option{}
	for _, raw := range in.Options {
		label, consequence, ok := strings.Cut(raw, ":")
		if !ok {
			return channel.Question{}, nil, 2, errors.New("--option wants label: consequence")
		}
		opts = append(opts, channel.Option{Label: strings.TrimSpace(label), Consequence: strings.TrimSpace(consequence)})
	}
	var warnings []string
	l, e := surface.load(root)
	if e != nil {
		warnings = append(warnings, e.Error())
		l = phase.Loaded{}
	}
	ctx, cancel, e := channelPollContext(root)
	if e != nil {
		return channel.Question{}, warnings, 1, e
	}
	defer cancel()
	// A question about the lane or the machine is waited for on its record,
	// not on the goal ledger, so it needs no ledger position.
	ledgerCursor := ""
	if in.Goal != "" {
		cursor, cursorExists, cursorErr := surface.cursor(root)
		if cursorErr != nil {
			return channel.Question{}, warnings, 1, fmt.Errorf("channel ask could not read the accepted ledger cursor: %v", cursorErr)
		}
		if cursorExists {
			ledgerCursor = cursor
		}
	}
	now, e := goalCommandNow(root)
	if e != nil {
		return channel.Question{}, warnings, 1, e
	}
	q, existing, e := channel.AskOrFind(channel.AskRequest{Context: ctx, RepoRoot: root, Goal: in.Goal, About: in.About, Kind: in.Kind, Machine: machine, Lineage: lineage, Facts: in.Facts, Options: opts, Recommendation: in.Recommendation, Wants: in.Wants, Budget: in.Budget, Provider: l.Provider, Destination: l.Destination, Now: now, LedgerCursor: ledgerCursor})
	if e != nil {
		return q, warnings, 1, e
	}
	if existing {
		return q, warnings, 0, errQuestionAlreadyOpen
	}
	return q, warnings, 0, nil
}

// errQuestionAlreadyOpen is an ask whose exact question already stands open
// (R-129-ui): the question is returned with it, and nothing was asked again.
var errQuestionAlreadyOpen = errors.New("this exact question is already open; nothing was asked again")

// channelWaitCommand is the durable wait cycle a channel wait drives: its
// selector argv, the provider poll, the waiting caller and where the result
// is printed.
var channelWaitCommand = func(args []string, poll func(context.Context) error, callerPID int64, stdout, stderr io.Writer) int {
	return runWaitCommand(args, poll, callerPID, func(result metarun.WaitResult, jsonOutput bool) { writeWaitResult(stdout, result, jsonOutput) }, stdout, stderr)
}

// channelWaitWith waits for one channel question's answer under an explicit
// invocation context: callerPID is the waiting caller the durable wait
// registers (a process entry's parent, or the current process on an edge
// that replaced a child), lineage is the channel ledger identity's, and the
// report goes to the caller's streams.
func channelWaitWith(callerPID int64, lineage string, stdout, stderr io.Writer, args []string, resolveMachine func(string) (string, error)) int {
	f := newFlagSet("channel wait", stdout, stderr)
	root := pathFlag(f, "root", ".", "repository root")
	id := f.String("question", "", "question id")
	resume := f.String("resume", "", "durable channel wait identifier")
	after := f.String("after", "", "accepted ledger cursor (required for legacy question records)")
	timeoutMinutes := f.Int("timeout", 0, "timeout in minutes (default 24 hours)")
	pollSeconds := f.Int("poll-seconds", 30, "channel provider poll interval in seconds (0 disables polling)")
	if f.Parse(args) != nil || f.NArg() != 0 || (*id == "") == (*resume == "") || *timeoutMinutes < 0 || *timeoutMinutes > 24*60 || *pollSeconds < 0 {
		return 2
	}
	questionID := *id
	if *resume != "" {
		stateRoot, resolveErr := goal.ResolveStateRoot(*root)
		if resolveErr != nil {
			fmt.Fprintln(stderr, resolveErr)
			return 65
		}
		row, _, rowErr := metarun.FindWaiterByID(stateRoot, *resume)
		if rowErr != nil {
			fmt.Fprintln(stderr, "channel wait cannot read its durable registration:", rowErr)
			return 4
		}
		if row.Selector.Poll != "channel" {
			fmt.Fprintln(stderr, "channel wait cannot resume a wait that has no channel poll selector")
			return 67
		}
		questionID = row.Selector.Question
	}
	q, err := channel.ReadQuestion(*root, questionID)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if q.Goal == "" && *resume == "" {
		return channelGoallessWait(*root, q, time.Duration(*timeoutMinutes)*time.Minute, time.Duration(*pollSeconds)*time.Second, callerPID, lineage, stdout, stderr, resolveMachine)
	}
	cursor := ""
	if *resume == "" {
		cursor = *after
		if cursor == "" {
			cursor = q.LedgerCursor
		}
		if cursor == "" {
			fmt.Fprintln(stderr, "this older question has no recorded position to wait from\npass --after with the position recorded before it was asked")
			return 67
		}
	} else if *after != "" {
		fmt.Fprintln(stderr, "channel wait --resume accepts no replacement ledger cursor")
		return 67
	}
	loaded, err := phase.Load(*root, true)
	if err != nil {
		fmt.Fprintln(stderr, "channel wait provider is unavailable:", err)
		return 1
	}
	if loaded.Provider == nil {
		fmt.Fprintln(stderr, "channel wait requires a configured channel provider")
		return 1
	}
	machine, lineage, err := channelIdentityFor(*root, resolveMachine, lineage)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	nextPoll := time.Time{}
	poll := func(ctx context.Context) error {
		pollAt := time.Now()
		if *pollSeconds == 0 || (!nextPoll.IsZero() && pollAt.Before(nextPoll)) {
			return errChannelPollNotDue
		}
		nextPoll = pollAt.Add(time.Duration(*pollSeconds) * time.Second)
		now, nowErr := goalCommandNow(*root)
		if nowErr != nil {
			return nowErr
		}
		_, pollErr := channel.Poll(ctx, channel.PollConfig{
			RepoRoot: *root, Destination: "fleet", ProviderName: loaded.Adapter,
			HumanUserID: loaded.HumanUserID, TOTPSecret: loaded.TOTPSecret,
			Machine: machine, Lineage: lineage, Provider: loaded.Provider,
			DestinationConfig: loaded.Destination, Now: now, MaxDispositions: 5,
		})
		return pollErr
	}
	waitArgs := []string{"--root", *root}
	if *resume != "" {
		waitArgs = append(waitArgs, "--resume", *resume)
	} else {
		waitArgs = append(waitArgs, "--goal", q.Goal, "--event", "human-act", "--verb", "answer", "--question", q.ID, "--after", cursor)
	}
	if *timeoutMinutes > 0 {
		waitArgs = append(waitArgs, "--timeout", (time.Duration(*timeoutMinutes) * time.Minute).String())
	} else if *resume == "" {
		waitArgs = append(waitArgs, "--timeout", (24 * time.Hour).String())
	}
	code := channelWaitCommand(waitArgs, poll, callerPID, stdout, stderr)
	if code == 0 {
		answered, readErr := channel.ReadQuestion(*root, q.ID)
		if readErr != nil || answered.Answer == nil {
			fmt.Fprintln(stderr, "channel wait matched an answer act but its accepted answer text is unavailable")
			return 1
		}
		fmt.Fprintln(stdout, answered.Answer.Text)
	}
	return code
}

// channelHumanWaitClock is the goal-less wait's clock and pause; tests
// replace both together.
var channelHumanWaitClock = struct {
	now   func() time.Time
	sleep func(context.Context, time.Duration)
}{now: time.Now, sleep: func(ctx context.Context, d time.Duration) {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}}

// channelGoallessWait waits for the answer to a question that names no
// goal. There is no goal ledger act to observe, so it registers the human
// wait of the session that waits, polls the channel, and reads the question
// record until its answer is recorded; the registration ends with the wait.
func channelGoallessWait(root string, q channel.Question, timeout, pollEvery time.Duration, callerPID int64, lineage string, stdout, stderr io.Writer, resolveMachine func(string) (string, error)) int {
	if timeout <= 0 {
		timeout = 24 * time.Hour
	}
	loaded, err := phase.Load(root, true)
	if err != nil {
		fmt.Fprintln(stderr, "channel wait provider is unavailable:", err)
		return 1
	}
	if loaded.Provider == nil {
		fmt.Fprintln(stderr, "channel wait requires a configured channel provider")
		return 1
	}
	machine, lineage, err := channelIdentityFor(root, resolveMachine, lineage)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	stateRoot, err := goal.ResolveStateRoot(root)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return metarun.ExitWaiterIO
	}
	options, err := waitRegisterOptions(stateRoot)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return metarun.ExitWaiterIO
	}
	resolved, code, err := resolveWaitCaller(stateRoot, callerPID)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return code
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	scanCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	openWorkSignature, err := waitOpenWorkSignature(scanCtx, stateRoot)
	cancel()
	if err != nil {
		fmt.Fprintln(stderr, "the open-work signature could not be read:", err)
		return metarun.ExitWaiterIO
	}
	store := &metarun.Store{Root: stateRoot}
	row, err := store.RegisterDetachedWait(metarun.RegisterWaitRequest{
		Kind: "human", Question: q.ID, Owner: resolved.owner, RuntimeSession: resolved.runtimeSession,
		Runtime: resolved.view.Announcement.Runtime, Timeout: timeout, OpenWorkSignature: openWorkSignature,
	}, options)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return metarun.WaiterExitCode(err)
	}
	end := func() {
		if _, endErr := store.EndDetachedWait(row.WaitID, resolved.owner, resolved.runtimeSession, options); endErr != nil {
			fmt.Fprintln(stderr, "the human wait registration could not be ended:", endErr)
		}
	}
	clock := channelHumanWaitClock
	deadline := clock.now().Add(timeout)
	nextPoll := time.Time{}
	for {
		if pollEvery > 0 && !clock.now().Before(nextPoll) {
			nextPoll = clock.now().Add(pollEvery)
			now, nowErr := goalCommandNow(root)
			if nowErr == nil {
				pollCtx, pollCancel := context.WithTimeout(ctx, time.Minute)
				_, nowErr = channel.Poll(pollCtx, channel.PollConfig{
					RepoRoot: root, Destination: "fleet", ProviderName: loaded.Adapter,
					HumanUserID: loaded.HumanUserID, TOTPSecret: loaded.TOTPSecret,
					Machine: machine, Lineage: lineage, Provider: loaded.Provider,
					DestinationConfig: loaded.Destination, Now: now, MaxDispositions: 5,
				})
				pollCancel()
			}
			if nowErr != nil {
				fmt.Fprintln(stderr, "channel poll failed; the wait goes on:", nowErr)
			}
		}
		current, readErr := channel.ReadQuestion(root, q.ID)
		switch {
		case readErr != nil:
			end()
			fmt.Fprintln(stderr, readErr)
			return 1
		case current.Answer != nil && current.Answer.Phase != "matched":
			end()
			fmt.Fprintln(stdout, current.Answer.Text)
			return 0
		case current.State == "closed" && current.Answer == nil:
			end()
			fmt.Fprintf(stderr, "question %s was withdrawn, so no answer will come\nrun: metasystem question show channel:%s\n", q.ID, q.ID)
			return 1
		case ctx.Err() != nil:
			end()
			fmt.Fprintln(stderr, "the wait was interrupted before the answer came")
			return metarun.ExitInterrupted
		case !clock.now().Before(deadline):
			end()
			fmt.Fprintf(stderr, "question %s is not answered yet; the person answers in its channel thread\nrun: metasystem question wait channel:%s\n", q.ID, q.ID)
			return metarun.ExitWaitDeadline
		}
		clock.sleep(ctx, 2*time.Second)
	}
}
