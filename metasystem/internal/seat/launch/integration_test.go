package launch

// One narrow integration test: the steps that are git and the filesystem, run
// for real against a fixture repository in a temporary directory.
//
// It stops before enrollment, deliberately and by construction — it runs the
// five steps it names and no others. Arming mints an identity and spawns a
// runner, `up` starts supervision rings, and presence reaches a remote; all
// three are the steward's own to prove, and a test that ran them here would
// be launching a machine on the machine running the tests.
//
// What is real here is what cannot be proven with a fake: that `git clone`
// from a local checkout produces a repository whose origin can be rewritten
// to the fleet's bare remote, that the endpoint and human keys this verb
// copies actually land on it, that a tracking fetch from that bare remote
// creates refs/remotes/origin/main, that the local configuration is copied as
// bytes with evidence.root rewritten through the engine's own writer, that
// the nickname reaches git configuration, and that the clone's own engine
// fetches the fleet's ledger from the bare fixture and accepts its tip.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// mixedRunner runs git and the real engine, and answers only for the two
// commands this test does not exercise.
//
// The engine is built once into the clone, because the ledger step is one of
// the things only a real run can prove: `goal fetch` validates the canonical
// tree it finds on the remote and creates the accepted ref from it, and a
// stubbed answer would prove that the sequencer runs a command rather than
// that a machine joins a ledger.
type mixedRunner struct {
	real OSRunner
	said map[string]string
	ran  []string
}

func (r *mixedRunner) Run(command Command) (string, error) {
	r.ran = append(r.ran, commandKey(command))
	if answer, canned := r.said[commandKey(command)]; canned {
		return answer, nil
	}
	return r.real.Run(command)
}

func TestAClonedMachineIsAFleetSeatBeforeItIsEnrolled(t *testing.T) {
	t.Parallel()
	bed := t.TempDir()
	bare := filepath.Join(bed, "origin.git")
	source := filepath.Join(bed, "agentic-tools")
	target := filepath.Join(bed, "agentic-tools-m1f")
	evidence := filepath.Join(bed, "evidence", "m1u")
	fixtureRepository(t, bare, source)
	if err := os.MkdirAll(evidence, 0o755); err != nil {
		t.Fatal(err)
	}

	sourceConf := filepath.Join(source, install, "metasystem.conf")
	runner := &mixedRunner{said: map[string]string{
		// The launching seat's own engine is this test binary's module and
		// not a built executable, so the two commands the SOURCE side runs
		// are answered rather than run. Everything the clone's own engine
		// does — the ledger fetch, the orientation — is real.
		"metasystem config get --key evidence.root --conf " + sourceConf: evidence + "\n",
		"metasystem validate session-isolation --source-root " + source + " --destination-root " + target +
			" --manifest " + filepath.Join(target, install, "artifacts", "agents", "ui", "local-config-paths") +
			" --harness-root " + filepath.Join(source, install): "",
		"metasystem config validate --conf " + filepath.Join(target, install, "metasystem.conf") +
			" --repo " + target: "",
	}}
	sequencer := &Sequencer{
		Request: Request{
			Machine: machineName, From: source, Destination: target,
			Word: humanWord, ReviewBy: reviewBy,
		},
		Installation: install, OriginURL: bare,
		Host: OSHost{}, Runner: runner, Clock: Wall{},
		Write:     func(Record) error { return nil },
		GitBudget: 60 * time.Second, BuildBudget: time.Minute,
	}
	record := Record{Launch: launchID, Machine: machineName, Destination: target, Outcome: OutcomeRunning}
	run := func(names ...string) {
		t.Helper()
		for _, name := range names {
			ran, err := sequencer.step(name, &record)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			if ran.outcome != StepDone {
				t.Fatalf("%s = %q, want done", name, ran.outcome)
			}
		}
	}
	run(StepClone, StepTracking, StepConfiguration, StepNickname)
	// The engine step is the one thing this test stands in for: go-build.sh
	// runs the gate fence, which has no business running inside a test. What
	// the build produces is an engine at this path, so the test produces one
	// and the ledger step below runs against the real thing.
	buildEngine(t, filepath.Join(target, install, "bin", "metasystem"))
	run(StepLedger)

	if got := git(t, target, "remote", "get-url", "origin"); got != bare {
		t.Fatalf("origin = %q, want the fleet's own remote %q", got, bare)
	}
	if got := git(t, target, "config", "--get", "goal.sync-remote"); got != "origin" {
		t.Fatalf("goal.sync-remote = %q", got)
	}
	if got := git(t, target, "config", "--get", "goal.human.wido"); got != "Wido <wido@example.invalid>" {
		t.Fatalf("goal.human.wido = %q", got)
	}
	if got := git(t, target, "config", "--get", "metasystem.steward.landing-ref"); got != "refs/remotes/origin/main" {
		t.Fatalf("landing-ref = %q", got)
	}
	if got := git(t, target, "config", "--get", "metasystem.goal.machine"); got != machineName {
		t.Fatalf("nickname = %q, want %q", got, machineName)
	}
	// The tracking fetch of step 2, which the ledger's own advance never
	// makes: without it refs/remotes/origin/main does not exist on a clone.
	if git(t, target, "rev-parse", "--verify", "refs/remotes/origin/main") == "" {
		t.Fatal("the clone has no remote-tracking ref for the fleet's branch")
	}
	// A key that is this checkout's git identity rather than a fleet
	// endpoint does not travel.
	if out, err := exec.Command("git", "-C", target, "config", "--local", "--get", "user.email").Output(); err == nil {
		t.Fatalf("the clone inherited user.email = %q", strings.TrimSpace(string(out)))
	}

	copied, err := os.ReadFile(filepath.Join(target, install, "metasystem.conf.local"))
	if err != nil {
		t.Fatalf("the local configuration did not reach the clone: %v", err)
	}
	sibling := filepath.Join(filepath.Dir(mustCanonical(t, evidence)), machineName)
	if !strings.Contains(string(copied), "evidence.root="+sibling) {
		t.Fatalf("the copied configuration is\n%s\nwant evidence.root=%s", copied, sibling)
	}
	// The bytes this verb never parses are carried through unread.
	if !strings.Contains(string(copied), "fleet.channel.secret=a-fixture-secret") {
		t.Fatalf("the copied configuration lost the bytes it was supposed to carry:\n%s", copied)
	}
	if _, err := os.Stat(sibling); err != nil {
		t.Fatalf("the new machine's evidence root was not created: %v", err)
	}
	// The ledger step, run for real against the bare fixture: `goal fetch`
	// validated the canonical tree it found there and created this clone's
	// own accepted ref from it, which is what makes the machine a seat of
	// this fleet rather than a copy of a directory.
	accepted := git(t, target, "rev-parse", "--verify", "refs/metasystem/goals/accepted")
	if accepted == "" {
		t.Fatal("the ledger step created no accepted ref on the clone")
	}
	if accepted != git(t, target, "rev-parse", "--verify", "refs/remotes/origin/main") {
		t.Fatalf("the accepted ref is %s, which is not the tip the bare fixture published", accepted)
	}
	if record.Orientation == "" {
		t.Fatal("the record carries no orientation line from the clone's own reading")
	}
	if record.ClonedCommit == "" {
		t.Fatal("the record does not say which commit was cloned")
	}
	if !record.Created.Destination || !record.Created.Nickname || !record.Created.EvidenceRoot {
		t.Fatalf("created = %+v", record.Created)
	}
	for _, key := range runner.ran {
		if strings.HasPrefix(key, "metasystem steward arm") || strings.HasPrefix(key, "metasystem up") {
			t.Fatalf("this test reached %q; it stops before enrollment", key)
		}
	}
}

// fixtureRepository builds a bare remote and a checkout of it that looks
// enough like a seat of this fleet to be cloned: an installation directory
// with a configuration in it, the endpoint and human keys in local git
// configuration, and an unversioned metasystem.conf.local beside the
// versioned one.
func fixtureRepository(t *testing.T, bare, source string) {
	t.Helper()
	run(t, "", "git", "init", "--quiet", "--bare", "-b", "main", bare)
	run(t, "", "git", "init", "--quiet", "-b", "main", source)
	run(t, source, "git", "config", "user.name", "fixture")
	run(t, source, "git", "config", "user.email", "fixture@example.invalid")
	run(t, source, "git", "config", "goal.sync-remote", "origin")
	run(t, source, "git", "config", "goal.sync-branch", "refs/heads/main")
	run(t, source, "git", "config", "metasystem.steward.landing-ref", "refs/remotes/origin/main")
	run(t, source, "git", "config", "goal.human.wido", "Wido <wido@example.invalid>")
	if err := os.MkdirAll(filepath.Join(source, install), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(source, install, "metasystem.conf"), "evidence.root=/not/this/one\n")
	// One lawful ledger, so the clone's own `goal fetch` has a canonical tree
	// to validate and an accepted ref to create. It is the smallest tree the
	// validator accepts: a root record and one queued goal.
	if err := os.MkdirAll(filepath.Join(source, "plans", "goals"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(source, "plans", "goals", "backlog.md"), string(goal.RenderRoot(&goal.RootRecord{
		Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "2",
		SyncMode: goal.SyncRemote, Revision: 1,
	})))
	write(t, filepath.Join(source, "plans", "goals", "fixture-goal.md"), string(goal.RenderFile(&goal.GoalFile{
		Id: "fixture-goal", State: goal.StateQueued, Intent: "Join the fleet.", Origin: "main",
		NextStep: "Start it.", OpenedAt: "2026-08-23T00:00:00Z", Revision: 1,
		History: []goal.HistoryLine{{
			At: "2026-08-23T00:00:00Z", Opid: "01ARZ3NDEKTSV4RRFFQ69G5FAV-bed-00000000",
			Verb: "open", Actor: "bed-m1+coordinator", Targets: []string{"fixture-goal"}, Keep: -1,
		}},
	})))
	if err := os.WriteFile(filepath.Join(source, ".gitignore"), []byte(install+"/metasystem.conf.local\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, source, "git", "add", "-A")
	run(t, source, "git", "commit", "--quiet", "-m", "fixture")
	run(t, source, "git", "remote", "add", "origin", bare)
	run(t, source, "git", "push", "--quiet", "origin", "main")
	// The seat's own local file: synthetic, in a temporary directory, and
	// never this repository's.
	write(t, filepath.Join(source, install, "metasystem.conf.local"),
		"evidence.root=/the/launching/seat\nfleet.channel.secret=a-fixture-secret\n")
}

func run(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	command := exec.Command(name, args...)
	command.Dir = dir
	command.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(args, " "), err, out)
	}
}

func write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func git(t *testing.T, root string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", root}, args...)...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// buildEngine compiles this module's own engine to a path, which is what the
// build step would have produced there.
func buildEngine(t *testing.T, at string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(at), 0o755); err != nil {
		t.Fatal(err)
	}
	build := exec.Command("go", "build", "-o", at, "github.com/widoriezebos/agentic-tools/metasystem/cmd/metasystem")
	build.Dir = filepath.Join("..", "..", "..")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build the engine for the clone: %v\n%s", err, out)
	}
}

func mustCanonical(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}
