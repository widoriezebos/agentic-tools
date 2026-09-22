package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/brain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	channelFake "github.com/widoriezebos/agentic-tools/metasystem/internal/channel/fake"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel/phase"
	channelTelegram "github.com/widoriezebos/agentic-tools/metasystem/internal/channel/telegram"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	metarun "github.com/widoriezebos/agentic-tools/metasystem/internal/run"
)

func channelIdentity(root string) (string, string, error) {
	m, err := goal.ResolveMachine(root)
	if err != nil {
		return "", "", err
	}
	lin := os.Getenv("METASYSTEM_OWNER_LINEAGE")
	if lin == "" {
		return "", "", fmt.Errorf("export METASYSTEM_OWNER_LINEAGE for channel ledger operations")
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

func runChannelStatus(args []string) int {
	f := flag.NewFlagSet("channel status", flag.ContinueOnError)
	root := pathFlag(f, "root", ".", "repository root")
	post := f.Bool("post", false, "post now")
	if f.Parse(args) != nil {
		return 2
	}
	now, err := goalCommandNow(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	machine, err := goal.ResolveMachine(*root)
	if err != nil {
		machine = "this machine"
	}
	text, approvalGoal, err := channel.ComposeStatusReport(channel.ReportConfig{RepoRoot: *root, Machine: machine, Now: now})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println(text)
	if *post {
		l, e := phase.Load(*root, false)
		if e != nil {
			fmt.Fprintln(os.Stderr, e)
			return 1
		}
		if l.Provider == nil {
			return 0
		}
		ctx, cancel, e := channelPollContext(*root)
		if e != nil {
			fmt.Fprintln(os.Stderr, e)
			return 1
		}
		defer cancel()
		ref, e := l.Provider.Post(ctx, l.Destination, text, nil)
		if e != nil {
			fmt.Fprintln(os.Stderr, e)
			return 1
		}
		state := channel.StatusState{LastPost: now.UTC(), ContentDigest: channel.Digest(text), Ref: ref, GoalID: approvalGoal}
		if e = channel.SaveStatusState(*root, state); e != nil {
			fmt.Fprintln(os.Stderr, e)
			return 1
		}
		brainState := brain.Read(*root, goal.ExistingLedgerIdentity(*root))
		if brainState.State == brain.Declared {
			postedAt := now.UTC()
			if e = brain.WriteStatus(*root, *brainState.Record, postedAt); e != nil {
				fmt.Fprintln(os.Stderr, e)
				return 1
			}
			if e = brain.MarkStatusPosted(*root, postedAt); e != nil {
				fmt.Fprintln(os.Stderr, e)
				return 1
			}
		}
	}
	return 0
}

func runChannelAsk(args []string) int {
	f := flag.NewFlagSet("channel ask", flag.ContinueOnError)
	root := pathFlag(f, "root", ".", "repository root")
	id := f.String("goal", "", "goal id")
	kind := f.String("kind", "", "question kind")
	recommend := f.String("recommend", "", "recommended option")
	wants := f.String("wants", "", "strict answer token")
	elapsed := f.String("elapsed-limit", "", "proposed elapsed limit for a budget or stop question")
	attempts := f.Int64("attempt-limit", 0, "proposed attempt limit for a budget or stop question")
	minutes := f.Int64("reserved-job-minutes-limit", 0, "proposed reserved job minutes for a budget or stop question")
	active := f.Int64("active-job-limit", 0, "proposed active job limit for a budget or stop question")
	reviewRounds := f.Int64("review-round-limit", -1, "proposed critic review-round limit for a budget or stop question")
	var facts, options repeatedStrings
	f.Var(&facts, "fact", "question fact")
	f.Var(&options, "option", "label: consequence")
	if f.Parse(args) != nil {
		return 2
	}
	if *kind == "carry" {
		budgetGiven := *elapsed != "" || *attempts != 0 || *minutes != 0 || *active != 0 || *reviewRounds != -1
		if !goal.ValidCarryToken(*wants) || budgetGiven {
			fmt.Fprintln(os.Stderr, "channel ask --kind carry requires --wants exactly `carry workspace=<sha40> goal=<id> past=<name>` and refuses every budget flag")
			return 2
		}
	}
	var proposedBudget *goal.Budget
	if *kind == "stop" || *kind == "budget-above-norm" {
		budget, budgetErr := goal.NewBudget(*elapsed, *attempts, *minutes, *active, *reviewRounds)
		if budgetErr != nil {
			fmt.Fprintf(os.Stderr, "a %s question requires a complete valid proposed budget tuple: %v\n", *kind, budgetErr)
			return 2
		}
		if *kind == "stop" {
			*wants = goal.ResumeApprovalToken(*id, budget)
		} else {
			proposedBudget = &budget
		}
	}
	machine, lineage, err := channelIdentity(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	opts := []channel.Option{}
	for _, raw := range options {
		label, consequence, ok := strings.Cut(raw, ":")
		if !ok {
			fmt.Fprintln(os.Stderr, "--option wants label: consequence")
			return 2
		}
		opts = append(opts, channel.Option{Label: strings.TrimSpace(label), Consequence: strings.TrimSpace(consequence)})
	}
	l, e := phase.Load(*root, false)
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		l = phase.Loaded{}
	}
	ctx, cancel, e := channelPollContext(*root)
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		return 1
	}
	defer cancel()
	ledgerCursor, cursorExists, cursorErr := goal.AcceptedLedgerTip(*root)
	if cursorErr != nil {
		fmt.Fprintln(os.Stderr, "channel ask could not read the accepted ledger cursor:", cursorErr)
		return 1
	}
	if !cursorExists {
		ledgerCursor = ""
	}
	now, e := goalCommandNow(*root)
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		return 1
	}
	q, e := channel.Ask(channel.AskRequest{Context: ctx, RepoRoot: *root, Goal: *id, Kind: *kind, Machine: machine, Lineage: lineage, Facts: facts, Options: opts, Recommendation: *recommend, Wants: *wants, Budget: proposedBudget, Provider: l.Provider, Destination: l.Destination, Now: now, LedgerCursor: ledgerCursor})
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		return 1
	}
	fmt.Println(q.ID)
	return 0
}
func runChannelShow(args []string) int {
	root, id, ok := channelQuestionFlags("show", args)
	if !ok {
		return 2
	}
	q, err := channel.ReadQuestion(root, id)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	printJSON(q)
	return 0
}
func channelQuestionFlags(name string, args []string) (string, string, bool) {
	f := flag.NewFlagSet("channel "+name, flag.ContinueOnError)
	root := pathFlag(f, "root", ".", "repository root")
	id := f.String("question", "", "question id")
	if f.Parse(args) != nil || *id == "" {
		return "", "", false
	}
	return *root, *id, true
}

var channelWaitCommand = runWaitWithPoll

func runChannelWait(args []string) int {
	f := flag.NewFlagSet("channel wait", flag.ContinueOnError)
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
			fmt.Fprintln(os.Stderr, resolveErr)
			return 65
		}
		row, _, rowErr := metarun.FindWaiterByID(stateRoot, *resume)
		if rowErr != nil {
			fmt.Fprintln(os.Stderr, "channel wait cannot read its durable registration:", rowErr)
			return 4
		}
		if row.Selector.Poll != "channel" {
			fmt.Fprintln(os.Stderr, "channel wait cannot resume a wait that has no channel poll selector")
			return 67
		}
		questionID = row.Selector.Question
	}
	q, err := channel.ReadQuestion(*root, questionID)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	cursor := ""
	if *resume == "" {
		cursor = *after
		if cursor == "" {
			cursor = q.LedgerCursor
		}
		if cursor == "" {
			fmt.Fprintln(os.Stderr, "channel wait: this legacy question has no ledgerCursor; pass --after with the accepted cursor recorded before the question")
			return 67
		}
	} else if *after != "" {
		fmt.Fprintln(os.Stderr, "channel wait --resume accepts no replacement ledger cursor")
		return 67
	}
	loaded, err := phase.Load(*root, true)
	if err != nil {
		fmt.Fprintln(os.Stderr, "channel wait provider is unavailable:", err)
		return 1
	}
	if loaded.Provider == nil {
		fmt.Fprintln(os.Stderr, "channel wait requires a configured channel provider")
		return 1
	}
	machine, lineage, err := channelIdentity(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
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
	code := channelWaitCommand(waitArgs, poll)
	if code == 0 {
		answered, readErr := channel.ReadQuestion(*root, q.ID)
		if readErr != nil || answered.Answer == nil {
			fmt.Fprintln(os.Stderr, "channel wait matched an answer act but its accepted answer text is unavailable")
			return 1
		}
		fmt.Println(answered.Answer.Text)
	}
	return code
}
func runChannelPoll(args []string) int {
	f := flag.NewFlagSet("channel poll", flag.ContinueOnError)
	root := pathFlag(f, "root", ".", "repository root")
	if f.Parse(args) != nil {
		return 2
	}
	l, err := phase.Load(*root, true)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if l.Provider == nil {
		return 0
	}
	machine, lineage, err := channelIdentity(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	ctx, cancel, err := channelPollContext(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer cancel()
	now, err := goalCommandNow(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	r, err := channel.Poll(ctx, channel.PollConfig{RepoRoot: *root, Destination: "fleet", ProviderName: l.Adapter, HumanUserID: l.HumanUserID, TOTPSecret: l.TOTPSecret, Machine: machine, Lineage: lineage, Provider: l.Provider, DestinationConfig: l.Destination, Now: now})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if r.Busy {
		fmt.Println("busy")
		return 0
	}
	printJSON(r)
	return 0
}
func runChannelClose(args []string) int {
	f := flag.NewFlagSet("channel close", flag.ContinueOnError)
	root := pathFlag(f, "root", ".", "repository root")
	id := f.String("question", "", "question id")
	because := f.String("because", "", "withdrawal reason")
	if f.Parse(args) != nil || *id == "" || *because == "" {
		return 2
	}
	l, _ := phase.Load(*root, false)
	if err := channel.Close(*root, *id, *because, l.Provider, l.Destination); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
func runChannelFakeServe(args []string) int {
	return runChannelFakeServeWithDependencies(args, defaultFixtureLifetimeDependencies(), channelFake.ServeReady)
}

func runChannelFakeServeWithDependencies(args []string, deps fixtureLifetimeDependencies, serve func(context.Context, string, chan<- string) error) int {
	f := flag.NewFlagSet("channel fake-serve", flag.ContinueOnError)
	dir := f.String("dir", "", "state directory")
	maxSecondsText := f.String("max-seconds", "0", "terminal lifetime when no owner leash closes")
	readyFD := f.Int("ready-fd", 0, "inherited descriptor that receives the listening address")
	if f.Parse(args) != nil || *dir == "" {
		return 2
	}
	maxSeconds, err := strconv.ParseInt(*maxSecondsText, 10, 64)
	if err != nil || maxSeconds < 0 {
		fmt.Fprintln(os.Stderr, "channel fake serve: --max-seconds must be a non-negative integer of seconds")
		return 2
	}
	signalContext, stopSignal := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stopSignal()
	ctx, stopLifetime, err := fixtureLifetimeContext(signalContext, maxSeconds, deps)
	if err != nil {
		fmt.Fprintln(os.Stderr, "channel fake serve:", err)
		return 2
	}
	defer stopLifetime()
	if *readyFD == 0 {
		if err := serve(ctx, *dir, nil); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}
	if *readyFD < 3 {
		fmt.Fprintln(os.Stderr, "channel fake serve: --ready-fd must name an inherited descriptor")
		return 2
	}
	readyFile := os.NewFile(uintptr(*readyFD), "channel-fake-ready")
	if readyFile == nil {
		fmt.Fprintln(os.Stderr, "channel fake serve: readiness descriptor is unavailable")
		return 2
	}
	info, statErr := readyFile.Stat()
	if statErr != nil || info.Mode()&os.ModeNamedPipe == 0 {
		_ = readyFile.Close()
		fmt.Fprintln(os.Stderr, "channel fake serve: readiness descriptor must be a pipe")
		return 2
	}
	ready := make(chan string, 1)
	served := make(chan error, 1)
	go func() { served <- serve(ctx, *dir, ready) }()
	select {
	case address := <-ready:
		_, err = fmt.Fprintln(readyFile, address)
		closeErr := readyFile.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			stopLifetime()
			<-served
			fmt.Fprintln(os.Stderr, "channel fake serve: publish readiness:", err)
			return 1
		}
	case err = <-served:
		_ = readyFile.Close()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}
	if err := <-served; err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
func runChannelFakeCode(args []string) int {
	f := flag.NewFlagSet("channel fake-code", flag.ContinueOnError)
	secretValue := f.String("secret", "", "base32 secret")
	at := f.Int64("at", 0, "Unix time")
	if f.Parse(args) != nil || *secretValue == "" {
		return 2
	}
	t := time.Now()
	if *at != 0 {
		t = time.Unix(*at, 0)
	}
	code, err := channel.TOTPCode(*secretValue, t)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println(code)
	return 0
}

func runChannelFake(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "channel fake needs serve or code")
		return 2
	}
	switch args[0] {
	case "serve":
		return runChannelFakeServe(args[1:])
	case "code":
		return runChannelFakeCode(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown channel fake verb %q\n", args[0])
		return 2
	}
}

func runChannelTelegram(args []string) int {
	if len(args) == 0 || args[0] != "peek" {
		fmt.Fprintln(os.Stderr, "channel telegram needs peek")
		return 2
	}
	f := flag.NewFlagSet("channel telegram peek", flag.ContinueOnError)
	root := pathFlag(f, "root", ".", "repository root")
	if f.Parse(args[1:]) != nil {
		return 2
	}
	const tokenKey = phase.TelegramBotTokenKey
	token, err := phase.Secret(*root, tokenKey)
	if err != nil {
		fmt.Fprintln(os.Stderr, channel.ErrUnconfigured(tokenKey+": "+err.Error()))
		return 1
	}
	base, err := phase.Get(*root, phase.TelegramAPIBaseKey, "https://api.telegram.org")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	dest := channel.DestinationConfig{Provider: "telegram", Token: token, APIBase: base, Secrets: []string{token}}
	ctx, cancel, err := channelPollContext(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer cancel()
	updates, err := channelTelegram.New(nil).Peek(ctx, dest)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	for _, update := range updates {
		text := []rune(update.Text)
		if len(text) > 40 {
			text = text[:40]
		}
		fmt.Printf("chat=%d user=%d text=%s\n", update.ChatID, update.UserID, string(text))
	}
	return 0
}
