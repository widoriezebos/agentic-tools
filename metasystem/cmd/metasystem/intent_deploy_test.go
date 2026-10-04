package main

// The deploy verbs and the lane's call (landing-deploys-the-engine,
// Decisions 2 to 4) over the fixture adapter, with Git stubbed: origin's
// URL and main's tip are the bed's, and a clean tree is a directory.

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/deploy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

// deployVerbBed is a checkout whose installation is its root, on a
// computer whose ~/.metasystem is home.
type deployVerbBed struct {
	t     *testing.T
	root  string
	home  string
	state string
	tip   string
	// landed is the deploy.json every commit's clean tree holds.
	landed []byte
	// person, when set, is why no person proves at this terminal.
	person error
	path   string
	clock  time.Time
	// launched are the detached starts the lane made: argv, then dir and log.
	launched [][]string
	pushed   plain.PushOutcome
}

func newDeployVerbBed(t *testing.T, declared bool) *deployVerbBed {
	t.Helper()
	base := realpath.Resolve(t.TempDir())
	b := &deployVerbBed{t: t, root: filepath.Join(base, "checkout"), home: filepath.Join(base, "home", ".metasystem"),
		state: filepath.Join(base, "adapter"), path: "/usr/bin:/bin", clock: time.Date(2026, 10, 3, 9, 0, 0, 0, time.UTC)}
	for _, dir := range []string{b.root, b.home, b.state} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(b.root, "metasystem.conf"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if declared {
		var err error
		b.landed, err = json.Marshal(deploy.Contract{Schema: 1, Adapter: deploy.Adapter{
			Argv: []string{testenv.Built(t, "./internal/deploy/testdata/fixtureadapter"), b.state}}})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(b.root, "deploy.json"), b.landed, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return b
}

func (b *deployVerbBed) owners() intentOwners {
	notARepository := func(string) (string, error) { return "", errors.New("not a repository") }
	return intentOwners{resolver: stateroot.NewResolver(notARepository, os.Executable), deploy: deployOwners{
		home: func() (string, error) { return b.home, nil },
		person: func(string) (string, error) {
			if b.person != nil {
				return "", b.person
			}
			return "Wido", nil
		},
		now: func() time.Time {
			b.clock = b.clock.Add(time.Second)
			return b.clock
		},
		origin: func(string) (string, error) { return "git@github.com:example/project.git", nil },
		git: func(string) deploy.Git {
			return deploy.Git{FetchMain: func() (string, error) { return b.tip, nil }, AddTree: func(dir, commit string) error {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					return err
				}
				for name, data := range map[string][]byte{"COMMIT": []byte(commit), "metasystem.conf": nil, "deploy.json": b.landed} {
					if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
						return err
					}
				}
				return nil
			}, RemoveTree: os.RemoveAll}
		},
		path:       func() string { return b.path },
		executable: func() (string, error) { return "/engines/metasystem", nil },
		launch: func(argv []string, dir, log string) (int64, error) {
			b.launched = append(b.launched, argv, []string{dir, log})
			return 4242, nil
		},
	}}
}

func (b *deployVerbBed) run(words ...string) (int, string) {
	b.t.Helper()
	command, ok := findIntentAction(words[0], words[1])
	if !ok {
		b.t.Fatalf("no public command %s %s", words[0], words[1])
	}
	var stdout, stderr bytes.Buffer
	code := runIntentIn(command, words[2:], &stdout, &stderr, b.root, b.owners())
	return code, stdout.String() + stderr.String()
}

func (b *deployVerbBed) result(words ...string) (int, intentResult) {
	b.t.Helper()
	code, text := b.run(append(words, "--json")...)
	var result intentResult
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		b.t.Fatalf("%v --json: %v\n%s", words, err, text)
	}
	return code, result
}

// dir is the project's shared deploy directory.
func (b *deployVerbBed) dir() string {
	key, err := deploy.ProjectKey("git@github.com:example/project.git")
	if err != nil {
		b.t.Fatal(err)
	}
	return deploy.Dir(b.home, key)
}

func (b *deployVerbBed) record() string {
	data, _ := os.ReadFile(filepath.Join(b.dir(), "deploys.jsonl"))
	return string(data)
}

func (b *deployVerbBed) calls() string {
	data, _ := os.ReadFile(filepath.Join(b.state, "calls"))
	return string(data)
}

// deployed runs deploy now on tip and requires it confirmed.
func (b *deployVerbBed) deployed(tip string) {
	b.t.Helper()
	b.tip = tip
	if code, text := b.run("deploy", "now"); code != 0 || !strings.Contains(text, "deployed v-"+tip) {
		b.t.Fatalf("deploy now of %s = %d\n%s", tip, code, text)
	}
}

func detailsHave(result intentResult, text string) bool {
	return strings.Contains(strings.Join(result.Details, "\n"), text)
}

// A project without deploy.json has no deploy: each verb says so, exits 0
// and changes nothing.
func TestDeployVerbsWithoutAContractSayTheProjectHasNoDeploy(t *testing.T) {
	t.Parallel()
	requireNoDeploy(t, newDeployVerbBed(t, false))
}

// deploy.contract naming a file the checkout lacks declares no deploy, even
// beside a deploy.json: each verb says so and changes nothing.
func TestDeployVerbsWhoseContractFileIsMissingSayTheProjectHasNoDeploy(t *testing.T) {
	t.Parallel()
	b := newDeployVerbBed(t, true)
	if err := os.WriteFile(filepath.Join(b.root, "metasystem.conf"), []byte("deploy.contract=ops/deploy.json\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	requireNoDeploy(t, b)
}

func requireNoDeploy(t *testing.T, b *deployVerbBed) {
	t.Helper()
	b.tip = "c1"
	for _, action := range []string{"status", "now", "rollback", "pause", "resume"} {
		if code, text := b.run("deploy", action); code != 0 || !strings.Contains(text, "declares no deploy") || strings.Contains(text, "export PATH") {
			t.Fatalf("deploy %s without a contract = %d\n%s", action, code, text)
		}
	}
	if _, err := os.Stat(filepath.Join(b.home, "deploy")); !os.IsNotExist(err) || b.calls() != "" {
		t.Fatalf("a project without a deploy wrote %v or called an adapter %q", err, b.calls())
	}
}

// deploy now deploys main's tip and, asked again, says the deploy already
// holds and writes nothing; the record names who deployed.
func TestDeployNowDeploysMainsTipAndThenHolds(t *testing.T) {
	t.Parallel()
	b := newDeployVerbBed(t, true)
	b.tip = "c1"
	if code, text := b.run("deploy", "now", "--by", "the landing lane"); code != 0 || !strings.Contains(text, "deployed v-c1 (c1)") {
		t.Fatalf("deploy now = %d\n%s", code, text)
	}
	record := b.record()
	code, result := b.result("deploy", "now")
	if code != 0 || result.Outcome != intentUnchanged || !strings.Contains(result.Summary, "already holds") || b.record() != record {
		t.Fatalf("a repeated deploy now = %d %+v", code, result)
	}
	if code, text := b.run("deploy", "status", "--verify"); code != 0 || !strings.Contains(text, "deployed v-c1 (c1)") || !strings.Contains(text, "by the landing lane") ||
		!strings.Contains(text, "the adapter reports v-c1 ("+filepath.Join(b.state, "artifacts", "c1")+") active") || strings.Contains(text, "not the record's") {
		t.Fatalf("status = %d\n%s", code, text)
	}
}

// The checkout's deploy.json only says that the project declares a deploy:
// dirty, naming an adapter no landed tree holds, it changes nothing deploy
// now calls.
func TestDeployNowCallsTheAdapterMainsTipNames(t *testing.T) {
	t.Parallel()
	b := newDeployVerbBed(t, true)
	if err := os.WriteFile(filepath.Join(b.root, "deploy.json"), []byte(`{"schema":1,"adapter":{"argv":["./not-landed"]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	b.deployed("c1")
	if got := b.calls(); got != "version c1\nbuild c1\nactivate c1\nversion c1\n" {
		t.Fatalf("calls = %q", got)
	}
}

// Once a run or rollback hears version answer none after a deploy, deploy
// status says nothing is active, and that is no failure.
func TestDeployStatusSaysNothingIsActiveAfterVersionAnswersNone(t *testing.T) {
	t.Parallel()
	b := newDeployVerbBed(t, true)
	b.deployed("c1")
	if err := os.Remove(filepath.Join(b.state, "active.json")); err != nil {
		t.Fatal(err)
	}
	if code, text := b.run("deploy", "rollback"); code != 1 || !strings.Contains(text, "no deploy to roll back to") {
		t.Fatalf("rollback = %d\n%s", code, text)
	}
	if code, text := b.run("deploy", "status"); code != 0 || !strings.Contains(text, "nothing is active since") || strings.Contains(text, "last failure") {
		t.Fatalf("status = %d\n%s", code, text)
	}
}

// deploy now while paused is refused as DEPLOY_PAUSED, naming deploy
// resume; nothing is built, and status shows main's tip waiting.
func TestDeployNowWhilePausedIsRefusedNamingResume(t *testing.T) {
	t.Parallel()
	b := newDeployVerbBed(t, true)
	b.deployed("c1")
	if code, text := b.run("deploy", "pause", "--reason", "maintenance"); code != 0 || !strings.Contains(text, "paused deploys") {
		t.Fatalf("pause = %d\n%s", code, text)
	}
	b.tip = "c2"
	code, result := b.result("deploy", "now")
	if code != 1 || result.Outcome != intentRefused || !detailsHave(result, deploy.CodePaused) || result.Next == nil || !strings.Contains(strings.Join(result.Next.Argv, " "), "deploy resume") {
		t.Fatalf("deploy now while paused = %d %+v", code, result)
	}
	if strings.Contains(b.calls(), "build c2") {
		t.Fatal("a paused deploy built main's tip")
	}
	if code, text := b.run("deploy", "status"); code != 0 || !strings.Contains(text, "main's tip c2 is not deployed yet") || !strings.Contains(text, "paused by Wido") || !strings.Contains(text, "maintenance") {
		t.Fatalf("status while paused = %d\n%s", code, text)
	}
}

// Rollback, pause and resume are a person's acts: a seat is refused and
// nothing changes.
func TestDeployPersonActsAreRefusedForASeat(t *testing.T) {
	t.Parallel()
	b := newDeployVerbBed(t, true)
	b.deployed("c1")
	b.deployed("c2")
	b.person = errors.New("no enrolled person proves this terminal")
	record := b.record()
	pause := filepath.Join(b.dir(), "pause.json")
	for _, action := range []string{"rollback", "pause"} {
		if code, text := b.run("deploy", action); code != 3 || !strings.Contains(text, "only a person may") {
			t.Fatalf("a seat's deploy %s = %d\n%s", action, code, text)
		}
		if _, err := os.Stat(pause); !os.IsNotExist(err) || b.record() != record {
			t.Fatalf("a seat's deploy %s paused or wrote", action)
		}
	}
	b.person = nil
	if code, text := b.run("deploy", "pause"); code != 0 {
		t.Fatalf("a person's pause = %d\n%s", code, text)
	}
	b.person = errors.New("no enrolled person proves this terminal")
	if code, text := b.run("deploy", "resume"); code != 3 || !strings.Contains(text, "only a person may") {
		t.Fatalf("a seat's deploy resume = %d\n%s", code, text)
	}
	if _, err := os.Stat(pause); err != nil || strings.Contains(b.calls(), "rollback") {
		t.Fatalf("a seat's resume lifted the pause (%v) or a seat rolled back", err)
	}
}

// A rollback with no deploy before the newest one is refused as
// DEPLOY_NO_PREVIOUS and changes nothing: main's tip waiting is not built.
func TestDeployRollbackWithNoPreviousIsRefused(t *testing.T) {
	t.Parallel()
	b := newDeployVerbBed(t, true)
	b.deployed("c1")
	b.tip = "c2"
	record := b.record()
	code, result := b.result("deploy", "rollback")
	if code != 1 || result.Outcome != intentRefused || !detailsHave(result, deploy.CodeNoPrevious) || !strings.Contains(result.Summary, "so nothing was changed") {
		t.Fatalf("rollback with no previous = %d %+v", code, result)
	}
	if _, err := os.Stat(filepath.Join(b.dir(), "pause.json")); !os.IsNotExist(err) || b.record() != record || strings.Contains(b.calls(), "build c2") {
		t.Fatal("a refused rollback paused, wrote or built")
	}
}

// While a deploy holds the lock, rollback and resume are refused as
// DEPLOY_RUNNING, naming deploy pause, and change nothing; once it is free,
// resume lifts the pause.
func TestDeployRollbackAndResumeDuringADeployAreRefusedNamingPause(t *testing.T) {
	t.Parallel()
	b := newDeployVerbBed(t, true)
	b.deployed("c1")
	b.deployed("c2")
	if code, text := b.run("deploy", "pause"); code != 0 {
		t.Fatalf("pause = %d\n%s", code, text)
	}
	held, err := lock.File(filepath.Join(b.dir(), "lock"), 0o600, lock.TryExclusive)
	if err != nil {
		t.Fatal(err)
	}
	record, pause := b.record(), filepath.Join(b.dir(), "pause.json")
	for _, action := range []string{"rollback", "resume"} {
		code, result := b.result("deploy", action)
		if code != 1 || result.Outcome != intentRefused || !detailsHave(result, deploy.CodeRunning) || result.Next == nil || strings.Join(result.Next.Argv[len(result.Next.Argv)-2:], " ") != "deploy pause" {
			t.Fatalf("deploy %s during a deploy = %d %+v", action, code, result)
		}
		if _, err := os.Stat(pause); err != nil || b.record() != record {
			t.Fatalf("a refused deploy %s lifted the pause (%v) or wrote", action, err)
		}
	}
	if err := held.Release(); err != nil {
		t.Fatal(err)
	}
	if code, text := b.run("deploy", "resume"); code != 0 || !strings.Contains(text, "lifted the pause by Wido") {
		t.Fatalf("resume once the lock is free = %d\n%s", code, text)
	}
}

// A person's rollback returns to the deploy before the newest one and
// pauses; asked again it holds; resume deploys main's tip again.
func TestDeployRollbackPausesAndResumeDeploysMainsTip(t *testing.T) {
	t.Parallel()
	b := newDeployVerbBed(t, true)
	b.deployed("c1")
	b.deployed("c2")
	if code, text := b.run("deploy", "rollback"); code != 0 || !strings.Contains(text, "rolled back to v-c1 (c1) and paused deploys") {
		t.Fatalf("rollback = %d\n%s", code, text)
	}
	record := b.record()
	if code, text := b.run("deploy", "rollback"); code != 0 || !strings.Contains(text, "already holds") || b.record() != record {
		t.Fatalf("a repeated rollback = %d\n%s", code, text)
	}
	if code, text := b.run("deploy", "status", "--history", "5"); code != 0 || !strings.Contains(text, "main's tip c2 is not deployed yet") || !strings.Contains(text, "rollback c1 active by Wido") {
		t.Fatalf("status after the rollback = %d\n%s", code, text)
	}
	if code, text := b.run("deploy", "resume"); code != 0 || !strings.Contains(text, "lifted the pause by Wido; deployed v-c2 (c2)") {
		t.Fatalf("resume = %d\n%s", code, text)
	}
}

// deploy status prints the line that puts ~/.metasystem/bin on PATH only
// when it is not there.
func TestDeployStatusPrintsThePathLineWhenTheBinIsNotOnPath(t *testing.T) {
	t.Parallel()
	b := newDeployVerbBed(t, true)
	b.deployed("c1")
	want := `export PATH="` + filepath.Join(b.home, "bin") + `:$PATH"`
	if code, text := b.run("deploy", "status"); code != 0 || !strings.Contains(text, want) {
		t.Fatalf("status without the bin on PATH = %d\n%s", code, text)
	}
	b.path = "/usr/bin:" + filepath.Join(b.home, "bin") + "/"
	if code, text := b.run("deploy", "status"); code != 0 || strings.Contains(text, "export PATH") {
		t.Fatalf("status with the bin on PATH = %d\n%s", code, text)
	}
}

// pushBed is b's checkout registered as the landing lane, whose push is
// the bed's pushed outcome.
func (b *deployVerbBed) pushOwners() intentOwners {
	owners := b.owners()
	owners.landing = laneVerbOwners{
		home:   func() (string, error) { return b.home, nil },
		now:    func() time.Time { return b.clock },
		probe:  func(string) (lane.OwnerProbe, error) { return lane.OwnerProbe{}, nil },
		ready:  func(string) error { return nil },
		person: func(string) (string, error) { return "", errors.New("the lane is not a person") },
		push: func(string, string, time.Time) (plain.PushOutcome, error) {
			return b.pushed, nil
		},
	}
	return owners
}

func (b *deployVerbBed) push(overrides ...intentOwners) (int, intentResult) {
	b.t.Helper()
	record, err := json.Marshal(lane.Record{Root: b.root, Install: b.root, CustodyEpoch: 1, RegisteredBy: "Wido", At: "2026-10-03T09:00:00Z"})
	if err != nil {
		b.t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(lane.RecordPath(b.home)), 0o755); err != nil {
		b.t.Fatal(err)
	}
	if err := os.WriteFile(lane.RecordPath(b.home), record, 0o644); err != nil {
		b.t.Fatal(err)
	}
	command, _ := findIntentAction("landing", "push")
	var stdout, stderr bytes.Buffer
	owners := b.pushOwners()
	if len(overrides) > 0 {
		owners = overrides[0]
	}
	code := runIntentIn(command, []string{"--json"}, &stdout, &stderr, b.root, owners)
	var result intentResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		b.t.Fatalf("landing push --json: %v\n%s%s", err, stdout.String(), stderr.String())
	}
	return code, result
}

// After a push that changed main the lane starts deploy now detached from
// its installation and says so in one detail line; a push that changed
// nothing starts nothing; the last failed deploy is told by one more line.
func TestLandingPushStartsDeployNowWhenMainChanged(t *testing.T) {
	t.Parallel()
	b := newDeployVerbBed(t, true)
	b.pushed = plain.PushOutcome{Old: "c0", Commit: "c1", Changed: true}
	code, result := b.push()
	want := []string{"/engines/metasystem", "deploy", "now", "--by", "the landing lane"}
	if code != 0 || len(b.launched) != 2 || strings.Join(b.launched[0], " ") != strings.Join(want, " ") || b.launched[1][0] != b.root ||
		b.launched[1][1] != filepath.Join(b.dir(), "lane.log") || !detailsHave(result, "started metasystem deploy now") {
		t.Fatalf("push = %d %+v; launched %q", code, result, b.launched)
	}
	b.pushed = plain.PushOutcome{Old: "c1", Commit: "c1"}
	if _, result := b.push(); len(b.launched) != 2 || detailsHave(result, "deploy") {
		t.Fatalf("a push that changed nothing = %+v; launched %q", result, b.launched)
	}
	// The detached deploy of c1 fails its build; the next push tells it.
	if err := os.MkdirAll(filepath.Join(b.state, "script"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(b.state, "script", "build.json"), []byte(`[{"exit":1,"reason":"compile error"}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	b.tip = "c1"
	if code, text := b.run("deploy", "now", "--by", "the landing lane"); code != 1 || !strings.Contains(text, "build-failed") {
		t.Fatalf("the failing deploy = %d\n%s", code, text)
	}
	b.pushed = plain.PushOutcome{Old: "c1", Commit: "c2", Changed: true}
	if _, result := b.push(); !detailsHave(result, "the last deploy failed: build-failed of c1: the adapter's build failed: compile error") || len(b.launched) != 4 {
		t.Fatalf("the push after a failed deploy = %+v", result)
	}
	// While deploys are paused the lane starts nothing and says so.
	if code, text := b.run("deploy", "pause"); code != 0 {
		t.Fatalf("pause = %d\n%s", code, text)
	}
	b.pushed = plain.PushOutcome{Old: "c2", Commit: "c3", Changed: true}
	if _, result := b.push(); !detailsHave(result, "deploys are paused by Wido, so no deploy was started") || len(b.launched) != 4 {
		t.Fatalf("a push while paused = %+v; launched %q", result, b.launched)
	}
}

// A deploy whose first version call fails is told by its line: deploy now
// says so, deploy status shows it as the last failure, and the next push
// names it.
func TestDeployWhoseVersionFailsIsToldByStatusAndTheNextPush(t *testing.T) {
	t.Parallel()
	b := newDeployVerbBed(t, true)
	if err := os.MkdirAll(filepath.Join(b.state, "script"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(b.state, "script", "version.json"), []byte(`[{"exit":1,"reason":"no state"}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	b.tip = "c1"
	if code, text := b.run("deploy", "now"); code != 1 || !strings.Contains(text, "failed (version-failed)") {
		t.Fatalf("deploy now = %d\n%s", code, text)
	}
	if _, text := b.run("deploy", "status"); !strings.Contains(text, "last failure: version-failed of c1") {
		t.Fatalf("deploy status =\n%s", text)
	}
	b.pushed = plain.PushOutcome{Old: "c0", Commit: "c1", Changed: true}
	if _, result := b.push(); !detailsHave(result, "the last deploy failed: version-failed of c1: what is active can't be learned: the adapter's version failed: no state") {
		t.Fatalf("the push after it = %+v", result)
	}
}

// A lane whose project declares no deploy does nothing more after its
// push.
func TestLandingPushWithoutADeployContractStartsNoDeploy(t *testing.T) {
	t.Parallel()
	b := newDeployVerbBed(t, false)
	b.pushed = plain.PushOutcome{Old: "c0", Commit: "c1", Changed: true}
	if code, result := b.push(); code != 0 || len(b.launched) != 0 || detailsHave(result, "deploy") {
		t.Fatalf("push without a deploy = %d %+v; launched %q", code, result, b.launched)
	}
	if _, err := os.Stat(filepath.Join(b.home, "deploy")); !os.IsNotExist(err) {
		t.Fatal("a push without a deploy made a deploy directory")
	}
}
