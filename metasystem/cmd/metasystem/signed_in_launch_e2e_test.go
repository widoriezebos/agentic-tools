package main

// Wido's ruling, end to end (g1-s72 §8, "One end-to-end test names the
// ruling"): "the browser is my terminal; if I am logged in there's no
// distinction." A launch the interface's starter writes under a signed-in
// session, carried by the sequencer to its enrollment step, and that step's
// `steward arm --launch-record` run for real, enrolls the new machine as the
// signed-in human's own: kind human-session, no word, no review date, and the
// session that launched it named on the identity.
//
// What is real and what stands in:
//   - The starter is launchStarterWith with its three seams: presence facts
//     the bed names, a spawn that does not detach but hands the test the
//     verb's argument list, and a fixed clock. The signed session's proof is
//     minted for the state root as the session store mints one.
//   - The sequencer is launch.Sequencer built the way seat_launch.go builds
//     it (RecordPath the record's file, Write the record's own SaveAt), over
//     a host, a runner and a presence that stand in for Git, the build and
//     the fleet. Steps one to six and eight to nine answer from the bed.
//   - Step seven's command is run for real: the exact name and arguments the
//     sequencer handed its runner, executed as the clone's installed engine
//     (this test binary, linked into the clone) in a detached chain, so the
//     whole `steward arm` verb runs, the real lease.ClassifyAt classifies
//     its caller against the kernel, and the real
//     steward.ArmSessionWithLineage mints the identity.
//   - Inside that verb two things stand in: Git (below), and the runner the
//     arm starts. The real `steward run` revives supervision, which no
//     hermetic bed can host, so the enrolled engine's `steward run` publishes
//     its live record and waits for TERM, as internal/steward's fake runner
//     does.
//
// No real Git (the project rule): every Git call the sequencer makes is
// stubbed through its Runner, and the arm chain runs with a stand-in named
// git first on its PATH (testdata/signedinlaunchgit). The arm verb binds the
// record's destination to `stateroot.RepositoryTop(--repo)`, which runs `git
// rev-parse --show-toplevel`, and reads the notify command with `git config`
// (internal/steward/notify.go); neither has a seam at the verb, so the
// stand-in answers those two from the bed and fails every other call as git
// fails outside a repository. The test passes with git absent from PATH.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot/stateroottest"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/lifecycle"
)

const (
	// The detached chain: the first helper starts the launcher stand-in in a
	// session of its own and exits, so the launcher is an orphan whose
	// ancestry holds no agent runtime and which has no terminal — the shape
	// of `seat launch` under a `ui serve` no agent started. The launcher runs
	// the arm command as its own child and reports how it ended.
	signedInLaunchDetacher = "test-helper-signed-in-launch-detacher"
	signedInLaunchLauncher = "test-helper-signed-in-launch-launcher"
	// signedInLaunchFakeRunner gates the enrolled engine's `steward run`
	// stand-in to the processes this test starts.
	signedInLaunchFakeRunner = "GO_WANT_SIGNED_IN_LAUNCH_FAKE_RUNNER"
	signedInLaunchStamp      = "0123456789abcdef0123456789abcdef01234567"
	signedInLaunchReference  = "01M3SESSIONREFERENCE000000"
)

// signedInLaunchOutcome is what the launcher stand-in reports on the pipe the
// test reads: the arm's exit code, its combined output, and the launcher's
// own parent at the moment it ran the arm.
type signedInLaunchOutcome struct {
	Code   int    `json:"code"`
	Output string `json:"output"`
	Parent int    `json:"parent"`
}

func init() {
	testHelperCommands[signedInLaunchDetacher] = func(args []string) int {
		// fd 3 is the test's pipe, handed down to the launcher; fd 4 of the
		// launcher is this process's liveness: its read end reaches EOF when
		// this process has exited.
		alive, held, err := os.Pipe()
		if err != nil {
			return 1
		}
		launcher := exec.Command(os.Args[0], append([]string{signedInLaunchLauncher}, args...)...)
		launcher.Env = os.Environ()
		launcher.ExtraFiles = []*os.File{os.NewFile(3, "signed-in-launch-report"), alive}
		if err := launcher.Start(); err != nil {
			return 1
		}
		_ = alive.Close()
		_ = launcher.Process.Release()
		_ = held // closed by this exit, which is what the launcher waits for
		return 0
	}
	testHelperCommands[signedInLaunchLauncher] = func(args []string) int {
		report := os.NewFile(3, "signed-in-launch-report")
		detacher := os.NewFile(4, "signed-in-launch-detacher")
		_, _ = io.Copy(io.Discard, detacher)
		_ = detacher.Close()
		if len(args) < 2 {
			return 2
		}
		var output bytes.Buffer
		arm := exec.Command(args[1], args[2:]...)
		arm.Dir = args[0]
		arm.Env = os.Environ()
		arm.Stdout, arm.Stderr = &output, &output
		outcome := signedInLaunchOutcome{Parent: os.Getppid()}
		if err := arm.Run(); err != nil {
			outcome.Code = -1
			var exit *exec.ExitError
			if errors.As(err, &exit) {
				outcome.Code = exit.ExitCode()
			}
			output.WriteString(err.Error())
		}
		outcome.Output = output.String()
		if err := json.NewEncoder(report).Encode(outcome); err != nil {
			return 1
		}
		return 0
	}
	// `steward run` from the enrolled engine: the stand-in when this test's
	// gate is set, the verb itself otherwise — the same dispatch TestMain
	// would make.
	testHelperCommands["steward"] = func(args []string) int {
		if os.Getenv(signedInLaunchFakeRunner) == "1" && len(args) >= 3 && args[0] == "run" && args[1] == "--repo" {
			return signedInLaunchRunner(args[2])
		}
		return dispatch(append([]string{"steward"}, args...))
	}
}

// signedInLaunchRunner publishes a live runner record for repo and waits for
// TERM: what the arm confirms before it answers.
func signedInLaunchRunner(repo string) int {
	terms := make(chan os.Signal, 1)
	signal.Notify(terms, syscall.SIGTERM)
	self, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		return 1
	}
	record, err := json.Marshal(steward.RunnerRecord{
		Pid: int64(os.Getpid()), StartTicks: self.StartTicks, BootID: self.BootID,
		PidStartedAt: self.StartedAt.Unix(), StartedAt: time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return 1
	}
	dir := filepath.Join(repo, "artifacts", "agents", "steward")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return 1
	}
	if err := os.WriteFile(filepath.Join(dir, "runner.json.tmp"), record, 0o644); err != nil {
		return 1
	}
	if err := os.Rename(filepath.Join(dir, "runner.json.tmp"), filepath.Join(dir, "runner.json")); err != nil {
		return 1
	}
	<-terms
	return 0
}

// signedInLaunchBed is the launching checkout, its state root, and where the
// machine lands.
type signedInLaunchBed struct {
	t           *testing.T
	roots       lifecycle.Roots
	destination string
	evidence    string
	// gitBin is the Git stand-in's directory and gitLog the calls it took.
	gitBin, gitLog string
	// commands is every command the sequencer handed its runner, in order.
	commands []launch.Command
	// armed is how the one real command ended.
	armed *signedInLaunchOutcome
}

const signedInLaunchInstallation = "metasystem"

func (b *signedInLaunchBed) install() string {
	return filepath.Join(b.destination, signedInLaunchInstallation)
}

// clone stands in for `git clone`: the destination becomes a checkout with
// the engine installed where the sequencer looks for it and a non-fixture
// configuration. Its notify command is the Git stand-in's answer.
func (b *signedInLaunchBed) clone(destination string) error {
	b.t.Helper()
	if destination != b.destination {
		return fmt.Errorf("cloned into %s, not the launch's %s", destination, b.destination)
	}
	if err := os.MkdirAll(b.install(), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(b.install(), "metasystem.conf"), []byte("# a machine of this fleet\n"), 0o644); err != nil {
		return err
	}
	installTestEngine(b.t, b.install())
	return nil
}

// buildSignedInLaunchGit builds the Git stand-in into a directory of its own,
// which the arm chain's PATH names first.
func buildSignedInLaunchGit(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	testenv.Link(t, testenv.Built(t, "./cmd/metasystem/testdata/signedinlaunchgit"), filepath.Join(dir, "git"))
	return dir
}

func (b *signedInLaunchBed) Run(command launch.Command) (string, error) {
	b.commands = append(b.commands, command)
	switch {
	case command.Name == "git" && len(command.Args) > 0 && command.Args[0] == "clone":
		return "", b.clone(command.Args[len(command.Args)-1])
	case command.Name == "git" && slices.Contains(command.Args, "rev-parse"):
		return signedInLaunchStamp + "\n", nil
	case command.Name == "git" && slices.Contains(command.Args, "--get"):
		return "", errors.New("exit status 1")
	case command.Name == "git", command.Name == "go":
		return "", nil
	case len(command.Args) >= 2 && command.Args[0] == "steward" && command.Args[1] == "arm":
		return b.armForReal(command)
	}
	return "", nil
}

// armForReal runs the sequencer's own arm command in the detached chain and
// answers as a runner answers: the output, and an error in the verb's words.
func (b *signedInLaunchBed) armForReal(command launch.Command) (string, error) {
	t := b.t
	t.Helper()
	read, write, err := os.Pipe()
	if err != nil {
		return "", err
	}
	defer read.Close()
	detacher := exec.Command(command.Name, append([]string{signedInLaunchDetacher, command.Dir, command.Name}, command.Args...)...)
	detacher.Dir = command.Dir
	detacher.Env = fixtureCommandEnvironment(t,
		signedInLaunchFakeRunner+"=1",
		"PATH="+b.gitBin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"SIGNED_IN_LAUNCH_GIT_TOP="+b.destination,
		"SIGNED_IN_LAUNCH_GIT_NOTIFY=true",
		"SIGNED_IN_LAUNCH_GIT_LOG="+b.gitLog,
	)
	detacher.ExtraFiles = []*os.File{write}
	detacher.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	output, runErr := detacher.CombinedOutput()
	_ = write.Close()
	if runErr != nil {
		return "", fmt.Errorf("the detached chain did not start: %v: %s", runErr, output)
	}
	// EOF when the launcher, the last holder of the write end, has exited.
	var outcome signedInLaunchOutcome
	if err := json.NewDecoder(read).Decode(&outcome); err != nil {
		return "", fmt.Errorf("the launcher reported nothing: %v", err)
	}
	b.armed = &outcome
	if outcome.Code != 0 {
		return outcome.Output, fmt.Errorf("steward arm exited %d: %s", outcome.Code, outcome.Output)
	}
	return outcome.Output, nil
}

func (b *signedInLaunchBed) Exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}
func (b *signedInLaunchBed) Canonical(path string) (string, error) { return path, nil }
func (b *signedInLaunchBed) MakeDir(path string) (bool, error) {
	return true, os.MkdirAll(path, 0o755)
}
func (b *signedInLaunchBed) EvidenceRoot(installation string) (config.EvidenceRoot, error) {
	return config.EvidenceRoot{Path: filepath.Join(b.evidence, filepath.Base(filepath.Dir(installation))), Origin: "default"}, nil
}
func (b *signedInLaunchBed) StateRoot(installation string) (string, error) {
	return filepath.Dir(installation), nil
}
func (b *signedInLaunchBed) CopyLocalConf(string, string) error { return nil }
func (b *signedInLaunchBed) MakeManifest(string) error          { return nil }
func (b *signedInLaunchBed) Stamp(string) (string, error)       { return signedInLaunchStamp, nil }

// Enrolled reads the clone's identity as it is on disk: absent before the
// arm, the arm's own mint after it.
func (b *signedInLaunchBed) Enrolled(installation string) (launch.Identity, bool) {
	top := canonicalPathOrSelf(installation)
	installed, err := steward.VerifyIdentity(steward.RepoIdentityPath(top), top)
	if err != nil {
		return launch.Identity{}, false
	}
	return launch.Identity{RepoIdentity: installed.RepoIdentity, Generation: installed.Generation}, true
}
func (b *signedInLaunchBed) SupervisionUp(string) bool   { return true }
func (b *signedInLaunchBed) Free(string) (uint64, error) { return 1 << 40, nil }
func (b *signedInLaunchBed) Size(string) (uint64, error) { return 1 << 20, nil }
func (b *signedInLaunchBed) Forget(string) error         { return nil }
func (b *signedInLaunchBed) Now() time.Time              { return starterNow }
func (b *signedInLaunchBed) After(time.Duration) <-chan time.Time {
	ticked := make(chan time.Time, 1)
	ticked <- starterNow
	return ticked
}

// Look is the fleet's copy of the new machine's presence: the record its own
// enrollment published.
func (b *signedInLaunchBed) Look(_ string, machine string) (seat.Record, bool, error) {
	enrolled, ok := b.Enrolled(b.install())
	if !ok {
		return seat.Record{}, false, nil
	}
	return seat.Record{Machine: machine, RepoIdentity: enrolled.RepoIdentity, Generation: enrolled.Generation}, true, nil
}

func canonicalPathOrSelf(path string) string {
	if resolved, err := canonicalPath(path); err == nil {
		return resolved
	}
	return path
}

// stopSignedInLaunchRunner stops the runner stand-in the arm started, by the
// pid its own record names, and joins its exit.
func stopSignedInLaunchRunner(t *testing.T, install string) {
	t.Helper()
	runner, alive := steward.LiveRunner(install)
	if !alive || runner.Pid < 1 {
		return
	}
	exact, state, err := (identity.KernelProber{}).Probe(runner.Pid)
	if err != nil || state != identity.Alive {
		return
	}
	if err := syscall.Kill(int(runner.Pid), syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
		t.Errorf("stop the runner stand-in %d: %v", runner.Pid, err)
		return
	}
	if err := testutil.AwaitExactExit(identity.KernelProber{}, exact.Ref()); err != nil {
		t.Errorf("the runner stand-in %d did not exit: %v", runner.Pid, err)
	}
}

// signedInLaunchRequest reads the verb's argument list the starter handed its
// spawn the way `seat launch` parses it: the request and the record's file.
func signedInLaunchRequest(t *testing.T, args []string) (launch.Request, string) {
	t.Helper()
	if len(args) < 2 || args[0] != "seat" || args[1] != "launch" || len(args)%2 != 0 {
		t.Fatalf("the starter spawned %v, not seat launch with flag pairs", args)
	}
	var request launch.Request
	record := ""
	for i := 2; i < len(args); i += 2 {
		switch args[i] {
		case "--from":
			request.From = args[i+1]
		case "--record":
			record = args[i+1]
		case "--machine":
			request.Machine = args[i+1]
		case "--destination":
			request.Destination = args[i+1]
		case "--resume":
			request.Resume = args[i+1]
		default:
			t.Fatalf("the starter handed the verb %s: %v", args[i], args)
		}
	}
	return request, record
}

func TestASignedInBrowserLaunchEnrollsTheMachineAsTheHumansOwn(t *testing.T) {
	t.Parallel()
	checkout, landing := canonicalTestDir(t), canonicalTestDir(t)
	bed := &signedInLaunchBed{
		t: t,
		roots: lifecycle.Roots{
			Checkout: checkout, Installation: stateroottest.Installation(t, filepath.Join(checkout, signedInLaunchInstallation)), StateRoot: stateroottest.State(t, canonicalTestDir(t)),
		},
		destination: filepath.Join(landing, "agentic-tools-m1f"),
		evidence:    canonicalTestDir(t),
		gitBin:      buildSignedInLaunchGit(t),
	}
	bed.gitLog = filepath.Join(t.TempDir(), "git.log")
	t.Cleanup(func() { stopSignedInLaunchRunner(t, canonicalPathOrSelf(bed.install())) })

	// 1. The starter, under a signed-in session whose proof is valid for
	// this state root. The session's own Human is not the handle: the
	// proof's is.
	signed := signedSession(t, bed.roots.StateRoot.Path(), signedInLaunchReference)
	if !signed.Proof.SessionValidFor(bed.roots.StateRoot.Path()) {
		t.Fatal("the minted proof is not valid for the state root")
	}
	var spawned []string
	starter := launchStarterWith(bed.roots, launchSeams{
		facts: func(launch.Request, launch.Record) (launch.Facts, error) {
			return launch.Facts{EvidenceRoot: filepath.Join(bed.evidence, "agentic-tools-ui")}, nil
		},
		spawn: func(roots lifecycle.Roots, asked launch.Request, record launch.Record, path string) error {
			spawned = launchArgs(roots, asked, record, path)
			return nil
		},
		now: func() time.Time { return starterNow },
	})
	started, err := starter(signed, launch.Request{Machine: "m1f", Destination: bed.destination})
	if err != nil {
		t.Fatalf("the starter refused a signed-in launch: %v", err)
	}
	proof := bed.roots.StateRoot.Path("artifacts", "agents", "authority", "proofs", launchProofOperation(started.Launch, starterNow)+".json")
	if _, err := os.Stat(proof); err != nil {
		t.Fatalf("the starter left no proof: %v", err)
	}

	// 2. The verb the starter spawned, as seat_launch.go runs it: the record
	// it names, admitted, and the sequencer over it.
	request, recordPath := signedInLaunchRequest(t, spawned)
	record, path, err := seatLaunchRecord(request, recordPath)
	if err != nil {
		t.Fatalf("seat launch could not read the starter's record: %v", err)
	}
	if record.Launch != started.Launch || path != recordPath {
		t.Fatalf("seat launch read launch %s at %s, want %s at %s", record.Launch, path, started.Launch, recordPath)
	}
	if err := launch.Admit(request, record); err != nil {
		t.Fatalf("seat launch refused the starter's own record: %v", err)
	}
	sequencer := &launch.Sequencer{
		Request: request, Installation: signedInLaunchInstallation,
		Host: bed, Runner: bed, Presence: bed, Clock: bed,
		Write:     func(written launch.Record) error { return launch.SaveAt(path, written, request.From) },
		Namespace: "signed-in-launch",
		GitBudget: time.Minute, BuildBudget: time.Minute,
		PresenceTick: time.Second, PresenceTicks: 1,
		RecordPath: path,
	}
	finished, err := sequencer.Run(record)
	if err != nil {
		t.Fatalf("the launch failed: %v\nsteps: %+v\narm: %+v", err, finished.Steps, bed.armed)
	}

	// 3. Step seven handed the arm the record and nothing about the human,
	// and the one real command it ran was that one.
	wantArm := []string{"steward", "arm", "--repo", bed.install(), "--launch-record", path}
	var armCommands [][]string
	for _, command := range bed.commands {
		if len(command.Args) >= 2 && command.Args[0] == "steward" && command.Args[1] == "arm" {
			armCommands = append(armCommands, command.Args)
		}
	}
	if len(armCommands) != 1 || !slices.Equal(armCommands[0], wantArm) {
		t.Fatalf("arm commands = %v, want exactly %v", armCommands, wantArm)
	}
	calls, _ := os.ReadFile(bed.gitLog)
	t.Logf("the arm ran under launcher parent %d and answered:\n%s\nGit calls the stand-in took:\n%s", bed.armed.Parent, bed.armed.Output, calls)
	if bed.armed == nil || !strings.Contains(bed.armed.Output, "armed as wido from a signed-in browser session") {
		t.Fatalf("the arm did not answer as a session enrollment: %+v", bed.armed)
	}
	var enrollment *launch.Step
	for i := range finished.Steps {
		if finished.Steps[i].Step == launch.StepEnrollment {
			enrollment = &finished.Steps[i]
		}
	}
	if enrollment == nil || enrollment.Outcome != launch.StepDone || enrollment.Words != "enrolled as wido from a signed-in browser session" {
		t.Fatalf("enrollment step = %+v", enrollment)
	}

	// 4. The ruling: the identity at the clone is the signed-in human's own.
	top := canonicalPathOrSelf(bed.install())
	installed, err := steward.VerifyIdentity(steward.RepoIdentityPath(top), top)
	if err != nil {
		t.Fatalf("the clone carries no readable identity: %v", err)
	}
	if installed.Enrollment != "human-session" {
		t.Fatalf("Enrollment = %q, want human-session", installed.Enrollment)
	}
	if installed.ReviewBy != "" || installed.TemporaryHumanWord != "" {
		t.Fatalf("the identity is temporary: word %q review by %q", installed.TemporaryHumanWord, installed.ReviewBy)
	}
	if installed.Session == nil {
		t.Fatal("the identity names no session")
	}
	want := steward.EnrolledSession{
		Provider: "browser", Human: "wido", Reference: signedInLaunchReference,
		Launch: started.Launch, From: checkout,
	}
	if *installed.Session != want {
		t.Fatalf("Session = %+v, want %+v", *installed.Session, want)
	}
	if installed.MintedBy != "human-session" {
		t.Fatalf("MintedBy = %q, want human-session", installed.MintedBy)
	}
}
