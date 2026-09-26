package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel/phase"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/missionrunner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stoptransition"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/lifecycle"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/up"
)

// survivingFamily is one process that is still running after its stop.
type survivingFamily struct{}

func (survivingFamily) Name() string { return "worker" }
func (survivingFamily) Inventory() ([]stoptransition.Item, error) {
	return []stoptransition.Item{{Key: "worker-42", StatusLine: "worker pid 42 running",
		Survivor: stopfence.Survivor{Component: "worker", Pid: 42, Reason: "running"}}}, nil
}
func (survivingFamily) Stop(stoptransition.Item) (stoptransition.Outcome, error) {
	return stoptransition.Outcome{Line: "worker pid 42 did not exit", Complete: false,
		Survivor: stopfence.Survivor{Component: "worker", Pid: 42, Reason: "did not exit"}}, nil
}

// channelTransport is one ask's channel provider: it posts or refuses.
type channelTransport struct {
	fail   bool
	posts  int
	onPost func()
}

func (c *channelTransport) Post(context.Context, channel.DestinationConfig, string, *channel.MessageRef) (channel.MessageRef, error) {
	c.posts++
	if c.onPost != nil {
		c.onPost()
	}
	if c.fail {
		return channel.MessageRef{}, errors.New("provider refused the post")
	}
	return channel.MessageRef{ID: "m-1", ThreadID: "t-1"}, nil
}
func (c *channelTransport) Receive(context.Context, channel.DestinationConfig, []channel.MessageRef, channel.Cursor) ([]channel.Inbound, channel.Cursor, error) {
	return nil, "", nil
}
func (c *channelTransport) Confirm(context.Context, channel.DestinationConfig, channel.Cursor) error {
	return nil
}
func (c *channelTransport) Credential(context.Context, channel.DestinationConfig) (channel.CredentialIdentity, error) {
	return channel.CredentialIdentity{}, nil
}

// installationShapeAt makes dir a complete installation with an engine file.
func installationShapeAt(t *testing.T, dir string) {
	t.Helper()
	for _, sub := range []string{filepath.Join(dir, "scripts", "agents"), filepath.Join(dir, "bin")} {
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for name, content := range map[string]string{"metasystem.conf": "", filepath.Join("bin", "metasystem"): "engine"} {
		if err := testexec.WriteFile(filepath.Join(dir, name), []byte(content), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func TestIntentProcessCorrections(t *testing.T) {
	t.Parallel()

	t.Run("an adopted nested installation keeps process records at its repository", func(t *testing.T) {
		t.Parallel()
		b := newProcessBed(t)
		app := t.TempDir()
		installation := filepath.Join(app, "tools", "metasystem")
		installationShapeAt(t, installation)
		foreign := filepath.Join(t.TempDir(), "metasystem")
		installationShapeAt(t, foreign)
		owners := b.owners()
		owners.resolver = stateroot.NewResolver(fakeTop(app), noExecutable)
		owners.processes.process.repositoryTop = fakeTop(app)
		run := func(args ...string) (int, intentResult) {
			t.Helper()
			command, _ := findIntentCommand(args[0])
			var stdout, stderr bytes.Buffer
			code := runIntentIn(command, append(args[1:], "--json"), &stdout, &stderr, installation, owners)
			var result intentResult
			if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
				t.Fatalf("%v: %v %q %q", args, err, stdout.String(), stderr.String())
			}
			return code, result
		}
		if code, result := run("stop", "--installation", foreign); code != 2 || result.Outcome != intentRefused || !strings.Contains(result.Summary, "does not belong to the checkout") {
			t.Fatalf("foreign installation = %d %+v", code, result)
		}
		for _, root := range []string{app, installation, foreign} {
			if record, _ := stopfence.Read(root); record.State != stopfence.StateOpen || record.Generation != 0 {
				t.Fatalf("a refused foreign stop changed %s: %+v", root, record)
			}
		}
		if code, result := run("stop", "--installation", installation); code != 0 || result.Outcome != intentConfirmed {
			t.Fatalf("adopted stop = %d %+v", code, result)
		}
		if record, _ := stopfence.Read(app); record.State != stopfence.StateClosed {
			t.Fatalf("the repository's fence was not closed: %+v", record)
		}
		if record, _ := stopfence.Read(installation); record.Generation != 0 {
			t.Fatalf("stop wrote a second fence inside the installation: %+v", record)
		}
		code, doctor := run("doctor")
		encoded, _ := json.Marshal(doctor.Data)
		var preview steward.HookHealthPreview
		if err := json.Unmarshal(encoded, &preview); err != nil || !preview.Verdict.Stopped || code != preview.ExitCode {
			t.Fatalf("doctor did not read the repository's fence: %d %+v %v", code, preview, err)
		}
		if _, status := run("status"); status.Outcome != intentConfirmed || !samePath(status.Targets[0].ID, app) {
			t.Fatalf("status = %+v", status)
		}
	})

	t.Run("an incomplete stop is partial and names what survived", func(t *testing.T) {
		t.Parallel()
		b := newProcessBed(t)
		b.families = []stoptransition.Family{survivingFamily{}}
		code, result := b.runJSON(b.owners(), "stop", "checkout")
		if code == 0 || result.Outcome != intentPartial || strings.HasPrefix(result.Summary, "stopped") ||
			result.Next == nil || !slices.Equal(result.Next.Argv, []string{"metasystem", "stop"}) {
			t.Fatalf("stop with a survivor = %d %+v", code, result)
		}
		lines, _ := result.Data.(map[string]any)["lines"].([]any)
		if !strings.Contains(strings.Join(anyStrings(lines), "\n"), "42") {
			t.Fatalf("the survivor is not named: %v", lines)
		}
		if record := b.fence(); record.State != stopfence.StateClosed || len(record.NotStopped) == 0 {
			t.Fatalf("the fence does not record the survivor: %+v", record)
		}
	})

	t.Run("a start that fails after the fence opened is partial", func(t *testing.T) {
		t.Parallel()
		b := newProcessBed(t)
		if code, _ := b.runJSON(b.owners(), "stop"); code != 0 {
			t.Fatal("bed stop")
		}
		b.class = lease.ClassDelegate
		code, result := b.runJSON(b.owners(), "start")
		if code == 0 || result.Outcome != intentRefused || b.armCalls != 0 || b.fence().State != stopfence.StateClosed {
			t.Fatalf("agent start = %d %+v", code, result)
		}
		b.class, b.armErr = lease.ClassHuman, errors.New("the steward refused to arm")
		code, result = b.runJSON(b.owners(), "start")
		record := b.fence()
		data, _ := result.Data.(map[string]any)
		if code == 0 || result.Outcome != intentPartial || b.armCalls != 1 || data["fence"] != record.State+"/"+record.Phase || data["running"] == nil ||
			result.Next == nil || !slices.Equal(result.Next.Argv, []string{"metasystem", "start"}) {
			t.Fatalf("failed start = %d %+v; fence %+v", code, result, record)
		}
		b.armErr = nil
		if code, result := b.runJSON(b.owners(), "start"); code != 0 || result.Outcome != intentConfirmed {
			t.Fatalf("repeated start = %d %+v", code, result)
		}
	})

	t.Run("an ordinary question is recorded and its delivery reported", func(t *testing.T) {
		t.Parallel()
		b := newProcessBed(t)
		transport := &channelTransport{}
		owners := b.owners()
		owners.processes.ask = func(root string, in channelAskInput) (channel.Question, []string, int, error) {
			return askChannelQuestionVia(root, in, channelAskSurface{
				identity: func(string) (string, string, error) { return "m1e", "", nil },
				load:     func(string) (phase.Loaded, error) { return phase.Loaded{Provider: transport}, nil },
				cursor:   func(string) (string, bool, error) { return "", false, nil },
			})
		}
		example := []string{"ask", "goal-a", "--question", "Land slice 2 now?", "--option", "yes: land it", "--option", "no: wait for review", "--recommend", "yes"}
		code, result := b.runJSON(owners, example...)
		if code != 0 || result.Outcome != intentConfirmed || transport.posts != 1 || len(result.Targets) != 2 {
			t.Fatalf("ordinary ask = %d %+v", code, result)
		}
		recorded, err := channel.ReadQuestion(b.root(), result.Targets[1].ID)
		if err != nil || recorded.Kind != "other" || recorded.Thread == nil || recorded.Facts[0] != "Land slice 2 now?" {
			t.Fatalf("durable question = %+v %v", recorded, err)
		}

		transport.fail = true
		code, result = b.runJSON(owners, "ask", "goal-a", "--question", "Another?", "--option", "yes: go")
		if code == 0 || result.Outcome != intentInProgress || result.Next == nil || !slices.Equal(result.Next.Argv, []string{"metasystem", "ask", "--retry", result.Targets[1].ID}) {
			t.Fatalf("undelivered ask = %d %+v", code, result)
		}
		if pending, _ := channel.ReadQuestion(b.root(), result.Targets[1].ID); pending.Thread != nil || pending.Undelivered != 1 {
			t.Fatalf("undelivered question = %+v", pending)
		}

		// The question is written, then posted; the write that records the
		// delivery fails, so the owner returns the question with its error.
		questions := filepath.Join(b.root(), "artifacts", "agents", "channel", "questions")
		transport.fail = false
		owners.processes.question = channel.ReadQuestion
		transport.onPost = func() { _ = os.Chmod(questions, 0o555) }
		t.Cleanup(func() { _ = os.Chmod(questions, 0o755) })
		code, result = b.runJSON(owners, "ask", "goal-a", "--question", "A third?", "--option", "yes: go")
		_ = os.Chmod(questions, 0o755)
		if code == 0 || result.Outcome != intentPartial || len(result.Targets) != 2 || result.Targets[1].ID == "" {
			t.Fatalf("ask whose delivery record failed = %d %+v", code, result)
		}
		kept, err := channel.ReadQuestion(b.root(), result.Targets[1].ID)
		if err != nil || kept.State != "open" || kept.Thread != nil {
			t.Fatalf("the saved question = %+v %v", kept, err)
		}
		// The post succeeded but its thread was not saved: the result says so,
		// asks for the storage repair and offers no answer or poll step.
		data := result.Data.(map[string]any)
		if data["posted"] != true || data["savedThread"] != nil || !strings.Contains(result.Summary, "was posted to the channel, but saving its thread failed") ||
			!strings.Contains(result.Summary, "post it a second time") || !strings.HasPrefix(result.Decision, "repair the channel question storage") || result.Next != nil {
			t.Fatalf("posted but unsaved thread = %+v", result)
		}
	})

	t.Run("a stop-loss reset recorded before its ask write is partial", func(t *testing.T) {
		t.Parallel()
		b := newProcessBed(t)
		askPath := parkedMission(t, b.root(), "stop-loss", map[string]any{"askId": "stop-loss", "streamId": "primary",
			"reasonClass": "stop-loss", "question": "reset?", "stopLossKind": mission.StopLossKindStagnation})
		asks := filepath.Dir(askPath)
		if err := os.Chmod(asks, 0o555); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(asks, 0o755) })
		owners := b.owners()
		owners.processes.mission = func(root, id string) (*missionrunner.Engine, error) {
			engine := missionrunner.NewEngine(root, id)
			engine.AnchorEffect = func(string, string, string) error { return nil }
			return engine, nil
		}
		answer := "reset: the tail work is worth more cycles"
		code, result := b.runJSON(owners, "answer", "mission", "demo", "stop-loss", answer)
		ledgerPath := filepath.Join(filepath.Dir(asks), "ledger.md")
		ledger, _ := os.ReadFile(ledgerPath)
		if code != 3 || result.Outcome != intentPartial || missionAskAnswered(askPath) || !strings.Contains(string(ledger), "Stop-loss reset: ask=stop-loss") ||
			result.Next == nil || !slices.Equal(result.Next.Argv, []string{"metasystem", "answer", "mission", "demo", "stop-loss", answer}) {
			t.Fatalf("reset without ask write = %d %+v", code, result)
		}
		if err := os.Chmod(asks, 0o755); err != nil {
			t.Fatal(err)
		}
		code, result = b.runJSON(owners, result.Next.Argv[1:]...)
		ledger, _ = os.ReadFile(ledgerPath)
		if code != 0 || result.Outcome != intentConfirmed || !missionAskAnswered(askPath) || strings.Count(string(ledger), "Stop-loss reset:") != 2 {
			t.Fatalf("the lawful retry = %d %+v", code, result)
		}
	})

	t.Run("the interface reports its state and restart through one result", func(t *testing.T) {
		t.Parallel()
		b := newProcessBed(t)
		stateRoot := t.TempDir()
		startCode := 0
		starts := 0
		owners := b.owners()
		owners.processes.ui = func(verb string, _ lifecycle.Roots) (uiLifecycleResult, error) {
			if verb == "status" {
				result, state := lifecycle.StatusReport(stateRoot, identity.KernelProber{}, nil)
				return uiLifecycleResult{Result: result, State: state}, nil
			}
			report := lifecycle.RestartReportFor(stateRoot, lifecycle.StopOptions{Prober: identity.KernelProber{}}, func() lifecycle.Result {
				starts++
				if startCode != 0 {
					return lifecycle.Result{Lines: []string{"cannot bind the interface address"}, Code: startCode}
				}
				return lifecycle.Result{Lines: []string{"interface running at http://127.0.0.1:1 (pid 1)"}}
			})
			return uiLifecycleResult{Result: report.Result, Restart: &report}, nil
		}
		code, result := b.runJSON(owners, "ui")
		if code != 1 || result.Outcome != intentConfirmed || result.Data.(map[string]any)["state"] != string(lifecycle.Stopped) {
			t.Fatalf("ui status = %d %+v", code, result)
		}
		if code, result := b.runJSON(owners, "restart", "ui"); code != 0 || result.Outcome != intentConfirmed || starts != 1 {
			t.Fatalf("restart ui = %d %+v", code, result)
		}
		startCode = 1
		code, result = b.runJSON(owners, "restart", "ui")
		if code == 0 || result.Outcome != intentPartial || starts != 2 || result.Next == nil || !slices.Equal(result.Next.Argv, []string{"metasystem", "ui", "start"}) {
			t.Fatalf("restart ui that cannot start = %d %+v", code, result)
		}
		owners.processes.ui = func(string, lifecycle.Roots) (uiLifecycleResult, error) {
			return uiLifecycleResult{}, errors.New("no interface installation")
		}
		if code, result := b.runJSON(owners, "restart", "ui"); code != 1 || result.Outcome != intentRefused {
			t.Fatalf("refused restart ui = %d %+v", code, result)
		}
	})
}

func anyStrings(values []any) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		text, _ := value.(string)
		out = append(out, text)
	}
	return out
}

// adoptedBed is an application repository at app with its installation
// nested at app/tools/metasystem, and a foreign installation elsewhere.
type adoptedBed struct {
	*processBed
	app, installation, foreign string
	owners                     intentOwners
}

func newAdoptedBed(t *testing.T) *adoptedBed {
	t.Helper()
	b := &adoptedBed{processBed: newProcessBed(t), app: t.TempDir()}
	b.installation = filepath.Join(b.app, "tools", "metasystem")
	installationShapeAt(t, b.installation)
	b.foreign = filepath.Join(t.TempDir(), "metasystem")
	installationShapeAt(t, b.foreign)
	b.owners = b.processBed.owners()
	b.owners.resolver = stateroot.NewResolver(fakeTop(b.app), noExecutable)
	b.owners.processes.process.repositoryTop = fakeTop(b.app)
	return b
}

func (b *adoptedBed) run(cwd string, args ...string) (int, intentResult) {
	b.t.Helper()
	command, _ := findIntentCommand(args[0])
	var stdout, stderr bytes.Buffer
	code := runIntentIn(command, append(args[1:], "--json"), &stdout, &stderr, cwd, b.owners)
	var result intentResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		b.t.Fatalf("%v: %v %q %q", args, err, stdout.String(), stderr.String())
	}
	return code, result
}

func TestIntentProcessAdoptedRoots(t *testing.T) {
	t.Parallel()

	t.Run("start, status and stop share the repository's records from its top", func(t *testing.T) {
		t.Parallel()
		b := newAdoptedBed(t)
		var seeded, armed []string
		var ups []up.Options
		var transitions []string
		b.owners.processes.process.armSteps = processArmEffects{
			seed: func(root string) (stewardLandingRefSeed, error) {
				seeded = append(seeded, root)
				return stewardLandingRefSeed{}, nil
			},
			arm: func(root, binary string, _ processArmAuthority) (string, error) {
				armed = append(armed, root, binary)
				return "steward armed", nil
			},
			up: func(options up.Options) up.Result { ups = append(ups, options); return up.Result{Outcome: "READY"} },
		}.steps
		transition := b.owners.processes.process.transition
		b.owners.processes.process.transition = func(scope processScope, scale int) *stoptransition.Transition {
			transitions = append(transitions, scope.Root)
			return transition(scope, scale)
		}
		for index, args := range [][]string{{"stop"}, {"start"}, {"status"}, {"stop"}} {
			args = append(args, "--installation", b.installation)
			code, result := b.run(b.app, args...)
			if code != 0 || result.Outcome != intentConfirmed || !samePath(result.Targets[0].ID, b.app) {
				t.Fatalf("%v = %d %+v", args, code, result)
			}
			// stop, start, status, stop: the repository's one fence moves
			// 1 closed, 2 open, 2 open, 3 closed.
			want := []int64{1, 2, 2, 3}[index]
			if record, _ := stopfence.Read(b.app); record.Generation != want {
				t.Fatalf("after %v the repository fence is %+v", args, record)
			}
		}
		for _, root := range transitions {
			if !samePath(root, b.app) {
				t.Fatalf("a transition used %s", root)
			}
		}
		binary := filepath.Join(b.installation, "bin", "metasystem")
		if len(seeded) != 1 || !samePath(seeded[0], b.app) || len(armed) != 2 || !samePath(armed[0], b.app) || !samePath(armed[1], binary) {
			t.Fatalf("the steward was armed at %v, seeded at %v", armed, seeded)
		}
		if len(ups) != 1 || !samePath(ups[0].Root, b.app) || !samePath(ups[0].MetasystemRoot, b.installation) || !samePath(ups[0].Scope, b.app) || !ups[0].RecoverOnly {
			t.Fatalf("supervision started with %+v", ups)
		}
		if record, _ := stopfence.Read(b.installation); record.Generation != 0 {
			t.Fatalf("a second fence appeared inside the installation: %+v", record)
		}
		if record, _ := stopfence.Read(b.app); record.State != stopfence.StateClosed || record.Generation != 3 {
			t.Fatalf("the repository fence after stop, start, stop = %+v", record)
		}
	})

	t.Run("a named installation must belong to the selected checkout", func(t *testing.T) {
		t.Parallel()
		b := newAdoptedBed(t)
		if code, result := b.run(b.app, "status"); code != 2 || result.Outcome != intentRefused || !strings.Contains(result.Decision, "--installation DIR") {
			t.Fatalf("repository top without an installation = %d %+v", code, result)
		}
		if code, result := b.run(b.app, "stop", "--installation", b.foreign); code != 2 || !strings.Contains(result.Summary, "does not belong to the checkout") {
			t.Fatalf("foreign installation = %d %+v", code, result)
		}
		other := t.TempDir()
		if code, result := b.run(b.app, "stop", "--repo", other, "--installation", b.installation); code != 2 || result.Outcome != intentRefused {
			t.Fatalf("installation of another checkout = %d %+v", code, result)
		}
		if code, result := b.run(b.app, "stop", "--installation", filepath.Join(b.app, "tools")); code != 2 || !strings.Contains(result.Summary, "is not a metasystem installation") {
			t.Fatalf("not an installation = %d %+v", code, result)
		}
		for _, root := range []string{b.app, b.installation, b.foreign} {
			if record, _ := stopfence.Read(root); record.Generation != 0 {
				t.Fatalf("a refused call changed %s: %+v", root, record)
			}
		}
		if code, result := b.run(b.app, "stop", "--installation", b.installation); code != 0 || result.Outcome != intentConfirmed {
			t.Fatalf("repository top with its installation = %d %+v", code, result)
		}
	})

	t.Run("the interface resolves its roots from the selected repository", func(t *testing.T) {
		t.Parallel()
		b := newAdoptedBed(t)
		var seen []lifecycle.Roots
		b.owners.processes.ui = func(verb string, roots lifecycle.Roots) (uiLifecycleResult, error) {
			seen = append(seen, roots)
			result, state := lifecycle.StatusReport(roots.StateRoot, identity.KernelProber{}, nil)
			return uiLifecycleResult{Result: result, State: state}, nil
		}
		for _, call := range []struct {
			cwd  string
			args []string
		}{{b.app, []string{"ui", "--installation", b.installation}}, {b.installation, []string{"ui"}}, {t.TempDir(), []string{"ui", "--repo", b.app, "--installation", b.installation}}} {
			code, result := b.run(call.cwd, call.args...)
			if code != 1 || result.Outcome != intentConfirmed || result.Data.(map[string]any)["state"] != string(lifecycle.Stopped) {
				t.Fatalf("%v from %s = %d %+v", call.args, call.cwd, code, result)
			}
		}
		for _, roots := range seen {
			if !samePath(roots.Checkout, b.app) || !samePath(roots.Installation, b.installation) || !samePath(roots.StateRoot, b.app) {
				t.Fatalf("interface roots = %+v", roots)
			}
		}
		code, result := b.run(t.TempDir(), "restart", "ui")
		if code != 2 || result.Outcome != intentRefused || strings.Contains(result.Summary+result.Decision, "--metasystem-root") || !strings.Contains(result.Decision, "--installation DIR") {
			t.Fatalf("interface outside a repository = %d %+v", code, result)
		}
		if len(seen) != 3 {
			t.Fatalf("the lifecycle ran %d times", len(seen))
		}
	})
}
