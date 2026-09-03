package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	channelFake "github.com/widoriezebos/agentic-tools/metasystem/internal/channel/fake"
	channelSlack "github.com/widoriezebos/agentic-tools/metasystem/internal/channel/slack"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func conf(root, key, def string) (string, error) {
	v, code, err := config.Get(config.GetParams{Key: key, ConfPath: filepath.Join(root, "metasystem.conf"), Default: def, DefaultSet: def != ""})
	if code != 0 {
		return "", err
	}
	return v, nil
}
func secret(root, key string) (string, error) {
	if v, ok := os.LookupEnv(config.EnvName(key)); ok {
		return v, nil
	}
	path := filepath.Join(root, "metasystem.conf")
	if v, ok, err := config.ConfLookup(path+".local", key); err == nil && ok {
		return v, nil
	}
	if v, ok, _ := config.ConfLookup(path, key); ok && v != "" {
		return "", fmt.Errorf("committed secret setting %s is ignored", key)
	}
	return "", fmt.Errorf("no value configured for %s", key)
}

type loadedChannel struct {
	provider            channel.Provider
	dest                channel.DestinationConfig
	adapter, user, totp string
}

func loadChannel(root string, withHuman bool) (loadedChannel, error) {
	adapter, err := conf(root, "channel.destination.fleet.adapter", "")
	if err != nil {
		return loadedChannel{}, channel.ErrUnconfigured("fleet channel is not configured")
	}
	var p channel.Provider
	var d channel.DestinationConfig
	switch adapter {
	case "fake":
		dir, e := conf(root, "channel.destination.fleet.fake.dir", "")
		if e != nil {
			return loadedChannel{}, channel.ErrUnconfigured("fake not serving")
		}
		p, d, err = channelFake.Provider(dir)
	case "slack":
		cid, e := conf(root, "channel.destination.fleet.slack.channel-id", "")
		if e != nil {
			return loadedChannel{}, e
		}
		token, e := secret(root, "channel.destination.fleet.slack.bot-token")
		if e != nil {
			return loadedChannel{}, e
		}
		base, _ := conf(root, "channel.destination.fleet.slack.api-base", "https://slack.com/api")
		d = channel.DestinationConfig{Name: "fleet", Provider: "slack", ChannelID: cid, Token: token, APIBase: base, Secrets: []string{token}}
		p = channelSlack.New(nil)
	default:
		return loadedChannel{}, fmt.Errorf("unknown channel adapter %q", adapter)
	}
	l := loadedChannel{provider: p, dest: d, adapter: adapter}
	if withHuman {
		l.user, err = conf(root, "channel.human.slack.user-id", "")
		if err != nil {
			return l, err
		}
		l.totp, err = secret(root, "channel.human.totp-secret")
		if err != nil {
			return l, err
		}
	}
	return l, err
}
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

func runChannelStatus(args []string) int {
	f := flag.NewFlagSet("channel status", flag.ContinueOnError)
	root := f.String("root", ".", "repository root")
	post := f.Bool("post", false, "post now")
	if f.Parse(args) != nil {
		return 2
	}
	machine, err := goal.ResolveMachine(*root)
	if err != nil {
		machine = "this machine"
	}
	text, err := channel.ComposeReport(channel.ReportConfig{RepoRoot: *root, Machine: machine, Now: time.Now()})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println(text)
	if *post {
		l, e := loadChannel(*root, false)
		if e != nil {
			fmt.Fprintln(os.Stderr, e)
			return 1
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		ref, e := l.provider.Post(ctx, l.dest, text, nil)
		if e != nil {
			fmt.Fprintln(os.Stderr, e)
			return 1
		}
		state := channel.StatusState{LastPost: time.Now().UTC(), ContentDigest: channel.Digest(text), Ref: ref}
		if e = channel.SaveStatusState(*root, state); e != nil {
			fmt.Fprintln(os.Stderr, e)
			return 1
		}
	}
	return 0
}

func runChannelAsk(args []string) int {
	f := flag.NewFlagSet("channel ask", flag.ContinueOnError)
	root := f.String("root", ".", "repository root")
	id := f.String("goal", "", "goal id")
	kind := f.String("kind", "", "question kind")
	recommend := f.String("recommend", "", "recommended option")
	wants := f.String("wants", "", "strict answer token")
	elapsed := f.String("elapsed-limit", "", "proposed resume elapsed limit for a stop question")
	attempts := f.Int64("attempt-limit", 0, "proposed resume attempt limit for a stop question")
	minutes := f.Int64("reserved-job-minutes-limit", 0, "proposed resume reserved job minutes for a stop question")
	active := f.Int64("active-job-limit", 0, "proposed resume active job limit for a stop question")
	var facts, options repeatedStrings
	f.Var(&facts, "fact", "question fact")
	f.Var(&options, "option", "label: consequence")
	if f.Parse(args) != nil {
		return 2
	}
	if *kind == "stop" {
		budget, budgetErr := goal.NewBudget(*elapsed, *attempts, *minutes, *active)
		if budgetErr != nil {
			fmt.Fprintln(os.Stderr, "a stop question requires a complete valid proposed resume tuple:", budgetErr)
			return 2
		}
		*wants = goal.ResumeApprovalToken(*id, budget)
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
	l, e := loadChannel(*root, false)
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		l = loadedChannel{}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	q, e := channel.Ask(channel.AskRequest{Context: ctx, RepoRoot: *root, Goal: *id, Kind: *kind, Machine: machine, Lineage: lineage, Facts: facts, Options: opts, Recommendation: *recommend, Wants: *wants, Provider: l.provider, Destination: l.dest, Now: time.Now()})
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
	root := f.String("root", ".", "repository root")
	id := f.String("question", "", "question id")
	if f.Parse(args) != nil || *id == "" {
		return "", "", false
	}
	return *root, *id, true
}
func runChannelWait(args []string) int {
	f := flag.NewFlagSet("channel wait", flag.ContinueOnError)
	root := f.String("root", ".", "repository root")
	id := f.String("question", "", "question id")
	timeout := f.Int("timeout", 0, "timeout in minutes")
	if f.Parse(args) != nil || *id == "" {
		return 2
	}
	deadline := time.Time{}
	if *timeout > 0 {
		deadline = time.Now().Add(time.Duration(*timeout) * time.Minute)
	}
	for {
		q, err := channel.ReadQuestion(*root, *id)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if q.Answer != nil {
			fmt.Println(q.Answer.Text)
			return 0
		}
		if !deadline.IsZero() && time.Now().After(deadline) {
			fmt.Fprintln(os.Stderr, "channel wait timed out")
			return 1
		}
		time.Sleep(200 * time.Millisecond)
	}
}
func runChannelPoll(args []string) int {
	f := flag.NewFlagSet("channel poll", flag.ContinueOnError)
	root := f.String("root", ".", "repository root")
	if f.Parse(args) != nil {
		return 2
	}
	l, err := loadChannel(*root, true)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	machine, lineage, err := channelIdentity(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	r, err := channel.Poll(ctx, channel.PollConfig{RepoRoot: *root, Destination: "fleet", ProviderName: l.adapter, HumanUserID: l.user, TOTPSecret: l.totp, Machine: machine, Lineage: lineage, Provider: l.provider, DestinationConfig: l.dest, Now: time.Now()})
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
	root := f.String("root", ".", "repository root")
	id := f.String("question", "", "question id")
	because := f.String("because", "", "withdrawal reason")
	if f.Parse(args) != nil || *id == "" || *because == "" {
		return 2
	}
	l, _ := loadChannel(*root, false)
	if err := channel.Close(*root, *id, *because, l.provider, l.dest); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
func runChannelFakeServe(args []string) int {
	f := flag.NewFlagSet("channel fake-serve", flag.ContinueOnError)
	dir := f.String("dir", "", "state directory")
	if f.Parse(args) != nil || *dir == "" {
		return 2
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := channelFake.Serve(ctx, *dir); err != nil {
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

var _ = strconv.Itoa

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
