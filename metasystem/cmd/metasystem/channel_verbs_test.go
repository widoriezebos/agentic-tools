package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/brain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	channelFake "github.com/widoriezebos/agentic-tools/metasystem/internal/channel/fake"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	metarun "github.com/widoriezebos/agentic-tools/metasystem/internal/run"
)

func captureChannelOutput(t *testing.T, run func() int) (int, string, string) {
	t.Helper()
	return captureCommandOutput(t, true, true, run)
}

func TestHCL12KindCarryRequiresWants(t *testing.T) {
	valid := "carry workspace=" + strings.Repeat("a", 40) + " goal=g past=missing-declaration"
	tests := []struct {
		name string
		args []string
	}{
		{name: "missing", args: []string{"--kind", "carry"}},
		{name: "malformed", args: []string{"--kind", "carry", "--wants", "carry workspace=short goal=g past=missing-declaration"}},
		{name: "extra field", args: []string{"--kind", "carry", "--wants", valid + " budget=forged"}},
		{name: "budget flag", args: []string{"--kind", "carry", "--wants", valid, "--attempt-limit", "1"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stderr, code := captureStderr(t, func() int { return runChannelAsk(test.args) })
			if code != 2 || !strings.Contains(stderr, "requires --wants exactly") || !strings.Contains(stderr, "refuses every budget flag") {
				t.Fatalf("carry channel question admitted invalid input: exit=%d stderr=%q", code, stderr)
			}
		})
	}
}

func TestConfigurationIndependentChannelVerbs(t *testing.T) {
	root := t.TempDir()
	q, err := channel.Ask(channel.AskRequest{RepoRoot: root, Goal: "g", Kind: "other", Machine: "m", Facts: []string{"fact"}})
	if err != nil {
		t.Fatal(err)
	}
	q.Answer = &channel.Answer{Text: "yes", Phase: "matched"}
	b, err := json.Marshal(q)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "artifacts", "agents", "channel", "questions", q.ID+".json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	// Close persists the exported answer without requiring any channel configuration.
	if err := channel.Close(root, q.ID, "test", nil, channel.DestinationConfig{}); err != nil {
		t.Fatal(err)
	}
	if code, _, problem := captureChannelOutput(t, func() int { return runChannelShow([]string{"--root", root, "--question", q.ID}) }); code != 0 {
		t.Fatal(problem)
	}
	if code, _, problem := captureChannelOutput(t, func() int { return runChannelWait([]string{"--root", root, "--question", q.ID}) }); code != 67 || !strings.Contains(problem, "no ledgerCursor") {
		t.Fatalf("legacy cursor-less question was not refused: code=%d stderr=%q", code, problem)
	}
	if code, out, problem := captureChannelOutput(t, func() int { return runChannelFakeCode([]string{"--secret", "JBSWY3DPEHPK3PXP", "--at", "59"}) }); code != 0 || strings.TrimSpace(out) == "" {
		t.Fatal(code, out, problem)
	}
	if _, err := os.Stat(filepath.Join(root, "metasystem.conf")); !os.IsNotExist(err) {
		t.Fatal("configuration-independent verbs created or required metasystem.conf")
	}
}

func TestChannelFakeServeRequiresBoundedLifetime(t *testing.T) {
	t.Parallel()
	deps := fixtureLifetimeTestDependencies(nil, nil, nil)
	called := false
	code := runChannelFakeServeWithDependencies([]string{"--dir", t.TempDir()}, deps, func(context.Context, string, chan<- string) error {
		called = true
		return nil
	})
	if code != 2 || called {
		t.Fatalf("unbounded fake server exit=%d called=%t, want refusal before serve", code, called)
	}
}

func TestChannelFakeServeStopsAtInjectedExpiry(t *testing.T) {
	t.Parallel()
	expiry := make(chan time.Time, 1)
	deps := fixtureLifetimeTestDependencies(nil, func(time.Duration) <-chan time.Time { return expiry }, nil)
	started := make(chan struct{})
	done := make(chan int, 1)
	dir := t.TempDir()
	go func() {
		done <- runChannelFakeServeWithDependencies([]string{"--dir", dir, "--max-seconds", "1"}, deps, func(ctx context.Context, _ string, _ chan<- string) error {
			close(started)
			<-ctx.Done()
			return nil
		})
	}()
	<-started
	expiry <- time.Unix(1, 0)
	if code := <-done; code != 0 {
		t.Fatalf("injected-expiry fake server exit = %d, want 0", code)
	}
}

func TestWaitChannelAnswer(t *testing.T) {
	root := t.TempDir()
	runReceiptGit(t, root, "init", "-q", "-b", "main")
	runReceiptGit(t, root, "config", "metasystem.goal.machine", "m")
	fakeDir, _ := commandFakeBed(t)
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("channel.destination.fleet.adapter=fake\nchannel.destination.fleet.fake.dir="+fakeDir+"\nchannel.human.slack.user-id=human-a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf.local"), []byte("channel.human.totp-secret=JBSWY3DPEHPK3PXP\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("METASYSTEM_OWNER_LINEAGE", "lineage-a")
	questionA, err := channel.Ask(channel.AskRequest{RepoRoot: root, Goal: "goal-a", Kind: "other", Machine: "m", Facts: []string{"first"}, LedgerCursor: strings.Repeat("a", 40)})
	if err != nil {
		t.Fatal(err)
	}
	questionB, err := channel.Ask(channel.AskRequest{RepoRoot: root, Goal: "goal-a", Kind: "other", Machine: "m", Facts: []string{"second"}, LedgerCursor: strings.Repeat("b", 40)})
	if err != nil {
		t.Fatal(err)
	}
	original := channelWaitCommand
	defer func() { channelWaitCommand = original }()
	var received []string
	channelWaitCommand = func(args []string, poll func(context.Context) error) int {
		if poll == nil {
			t.Fatal("channel wait did not carry its provider poll into the wait cycle")
		}
		received = append([]string(nil), args...)
		return 23
	}
	code, _, problem := captureChannelOutput(t, func() int {
		return runChannelWait([]string{"--root", root, "--question", questionA.ID, "--timeout", "60", "--poll-seconds", "7"})
	})
	if code != 23 || problem != "" {
		t.Fatalf("translated wait code=%d stderr=%q", code, problem)
	}
	joined := strings.Join(received, "\x00")
	for _, required := range []string{"--goal\x00goal-a", "--event\x00human-act", "--verb\x00answer", "--question\x00" + questionA.ID, "--after\x00" + strings.Repeat("a", 40)} {
		if !strings.Contains(joined, required) {
			t.Fatalf("translated answer wait lacks %q: %v", required, received)
		}
	}
	if strings.Contains(joined, questionB.ID) || strings.Contains(joined, strings.Repeat("b", 40)) {
		t.Fatalf("the other question leaked into the selector: %v", received)
	}
	if !strings.Contains(joined, "--timeout\x001h0m0s") {
		t.Fatalf("the integer-minute channel timeout was not translated to the wait deadline: %v", received)
	}
	legacy, err := channel.Ask(channel.AskRequest{RepoRoot: root, Goal: "goal-a", Kind: "other", Machine: "m", Facts: []string{"legacy"}})
	if err != nil {
		t.Fatal(err)
	}
	code, _, problem = captureChannelOutput(t, func() int { return runChannelWait([]string{"--root", root, "--question", legacy.ID}) })
	if code != 67 || !strings.Contains(problem, "no ledgerCursor") {
		t.Fatalf("legacy question code=%d stderr=%q", code, problem)
	}
	code, _, problem = captureChannelOutput(t, func() int {
		return runChannelWait([]string{"--root", root, "--question", legacy.ID, "--after", strings.Repeat("c", 40)})
	})
	if code != 23 || problem != "" || !strings.Contains(strings.Join(received, "\x00"), "--after\x00"+strings.Repeat("c", 40)) {
		t.Fatalf("explicit legacy cursor was not translated: code=%d args=%v stderr=%q", code, received, problem)
	}
	channelWaitCommand = func(args []string, poll func(context.Context) error) int {
		answered, readErr := channel.ReadQuestion(root, questionA.ID)
		if readErr != nil {
			t.Fatal(readErr)
		}
		answered.Answer = &channel.Answer{Text: "accepted answer text", Phase: "matched"}
		body, marshalErr := json.Marshal(answered)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		if writeErr := os.WriteFile(filepath.Join(root, "artifacts", "agents", "channel", "questions", questionA.ID+".json"), body, 0o600); writeErr != nil {
			t.Fatal(writeErr)
		}
		return 0
	}
	code, out, problem := captureChannelOutput(t, func() int { return runChannelWait([]string{"--root", root, "--question", questionA.ID}) })
	if code != 0 || problem != "" || !strings.Contains(out, "accepted answer text") {
		t.Fatalf("accepted answer output code=%d stdout=%q stderr=%q", code, out, problem)
	}
	waitID := strings.Repeat("e", 32)
	channelRow := metarun.Waiter{
		SchemaVersion: 2, WaitID: waitID, Nonce: strings.Repeat("f", 32), Kind: "goal", TargetID: "goal-a", OwnerDigest: "owner-a", State: "pending",
		Selector: metarun.WaitSelector{Kind: "goal", TargetID: "goal-a", GoalID: "goal-a", Event: "human-act", Verb: "answer", Question: questionA.ID, After: strings.Repeat("a", 40), Poll: "channel"},
	}
	if err := os.MkdirAll(metarun.WaitersDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	rowBody, err := json.Marshal(channelRow)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(metarun.WaiterPath(root, channelRow.Kind, channelRow.TargetID, channelRow.OwnerDigest), rowBody, 0o600); err != nil {
		t.Fatal(err)
	}
	channelWaitCommand = func(args []string, poll func(context.Context) error) int {
		if poll == nil || !strings.Contains(strings.Join(args, "\x00"), "--resume\x00"+waitID) {
			t.Fatalf("channel resume did not restore its provider poll and durable selector: args=%v poll-present=%t", args, poll != nil)
		}
		return 0
	}
	code, out, problem = captureChannelOutput(t, func() int { return runChannelWait([]string{"--root", root, "--resume", waitID}) })
	if code != 0 || problem != "" || strings.TrimSpace(out) != "accepted answer text" {
		t.Fatalf("channel resume code=%d stdout=%q stderr=%q", code, out, problem)
	}

	unconfigured := t.TempDir()
	questionC, err := channel.Ask(channel.AskRequest{RepoRoot: unconfigured, Goal: "goal-a", Kind: "other", Machine: "m", Facts: []string{"unconfigured"}, LedgerCursor: strings.Repeat("d", 40)})
	if err != nil {
		t.Fatal(err)
	}
	called := false
	channelWaitCommand = func([]string, func(context.Context) error) int { called = true; return 0 }
	code, _, problem = captureChannelOutput(t, func() int { return runChannelWait([]string{"--root", unconfigured, "--question", questionC.ID}) })
	if code != 1 || called || !strings.Contains(problem, "requires a configured channel provider") {
		t.Fatalf("unconfigured provider code=%d called=%t stderr=%q", code, called, problem)
	}

	// The installed two-clone answer sequence is exercised by
	// TestWaitGoalLandingAndHumanAct; this command test owns question routing,
	// compatibility flags, provider polling, and accepted answer output.
}

func commandFakeBed(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	ready := make(chan string, 1)
	go func() { done <- channelFake.ServeReady(ctx, dir, ready) }()
	select {
	case base := <-ready:
		t.Cleanup(func() {
			cancel()
			if err := <-done; err != nil {
				t.Error(err)
			}
		})
		return dir, strings.TrimSpace(base)
	case err := <-done:
		cancel()
		t.Fatalf("fake did not start: %v", err)
		return "", ""
	}
}

func TestChannelStatusPostSeedsAnUnbootedBrainStatus(t *testing.T) {
	dir, _ := commandFakeBed(t)
	root := syncedClaimedGoalFixture(t)
	conf, err := os.OpenFile(filepath.Join(root, "metasystem.conf"), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fmt.Fprintf(conf, "channel.destination.fleet.adapter=fake\nchannel.destination.fleet.fake.dir=%s\n", dir); err != nil {
		_ = conf.Close()
		t.Fatal(err)
	}
	if err := conf.Close(); err != nil {
		t.Fatal(err)
	}
	record := brain.Record{
		Schema: brain.Schema, Ledger: goal.ExistingLedgerIdentity(root), Machine: "mac-cli",
		DeclaredBy: "Wido", DeclaredAt: "2026-09-07T00:00:00Z",
	}
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(brain.Path(root)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(brain.Path(root), append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(brain.StatusPath(root)); !os.IsNotExist(err) {
		t.Fatalf("unbooted fixture unexpectedly had a status file: %v", err)
	}

	code, _, problem := captureChannelOutput(t, func() int { return runChannelStatus([]string{"--root", root, "--post"}) })
	if code != 0 || problem != "" {
		t.Fatalf("first status post failed: code=%d stderr=%q", code, problem)
	}
	status, err := brain.ReadStatus(root)
	if err != nil || status.Line != brain.StatusLine(record) || status.LastPostedAt == "" {
		t.Fatalf("first status post did not seed and mark brain status: status=%+v err=%v", status, err)
	}
}

func TestTelegramPeekWorksWithoutConfiguredAdapterOrChatID(t *testing.T) {
	dir, base := commandFakeBed(t)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("channel.destination.fleet.telegram.api-base="+base+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(config.EnvName("channel.destination.fleet.telegram.bot-token"), "environment-token")
	if err := os.WriteFile(filepath.Join(dir, "replies.jsonl"), []byte(`{"face":"telegram","user":7001,"chat":1000,"text":"hello from the phone"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, problem := captureChannelOutput(t, func() int { return runChannelTelegram([]string{"peek", "--root", root}) })
	if code != 0 || strings.TrimSpace(out) != "chat=1000 user=7001 text=hello from the phone" || problem != "" {
		t.Fatal(code, out, problem)
	}
}

func TestTelegramPeekTokenNeverAppearsInErrors(t *testing.T) {
	const token = "command-secret-token"
	t.Setenv(config.EnvName("channel.destination.fleet.telegram.bot-token"), token)
	cases := []struct {
		name string
		base func() (string, func())
	}{
		{"transport", func() (string, func()) { return "http://127.0.0.1:1", func() {} }},
		{"redirect", func() (string, func()) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, "/bot"+token+r.URL.Path, http.StatusFound)
			}))
			return s.URL, s.Close
		}},
		{"echoed 401", func() (string, func()) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
				fmt.Fprintf(w, `{"ok":false,"description":%q}`, token)
			}))
			return s.URL, s.Close
		}},
		{"malformed JSON", func() (string, func()) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, token+" not json") }))
			return s.URL, s.Close
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			base, cleanup := tc.base()
			defer cleanup()
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("channel.destination.fleet.telegram.api-base="+base+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			code, _, problem := captureChannelOutput(t, func() int { return runChannelTelegram([]string{"peek", "--root", root}) })
			if code == 0 || strings.Contains(problem, token) || strings.Contains(problem, "/bot"+token+"/") {
				t.Fatal(code, problem)
			}
		})
	}
}

func TestChannelStatusUsesOnlyTheRootAuthorizedSemanticClock(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		runtime     string
		fixtureNow  string
		want        string
		wantFailure bool
	}{
		{name: "fake root uses exact clock", runtime: "fake", fixtureNow: "2026-09-07T00:00:00Z", want: "this machine status 2026-09-07 00:00 +0000"},
		{name: "normal root ignores malformed override", runtime: "none", fixtureNow: "not-a-time", want: "this machine status "},
		{name: "fake root refuses malformed clock", runtime: "fake", fixtureNow: "not-a-time", want: "METASYSTEM_GOAL_NOW must be an RFC3339 timestamp", wantFailure: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes="+test.runtime+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			command := exec.Command(commandTestExecutable(t), "channel", "status", "--root", root)
			command.Env = fixtureCommandEnvironment(t, "TZ=UTC", "METASYSTEM_GOAL_NOW="+test.fixtureNow)
			output, err := command.CombinedOutput()
			if test.wantFailure {
				if err == nil || !strings.Contains(string(output), test.want) {
					t.Fatalf("channel status output = %q, error = %v; want failure containing %q", output, err, test.want)
				}
				return
			}
			if err != nil || !strings.Contains(string(output), test.want) {
				t.Fatalf("channel status output = %q, error = %v; want %q", output, err, test.want)
			}
			if test.runtime != "fake" && strings.Contains(string(output), "2026-09-07 00:00 +0000") {
				t.Fatalf("normal root used fixture time: %s", output)
			}
		})
	}
}
