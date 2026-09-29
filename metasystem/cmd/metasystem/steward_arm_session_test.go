package main

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// g1-s72 D2: `steward arm --launch-record <path>` reads the verdict the
// interface stamped, binds it to one clone, and classifies its caller.

const sessionArmTestLaunch = "01M3NZQN2WCRCZB8JVHE8XPH59"

// writeSessionLaunchRecord writes a session-enrolled launch record where the
// interface keeps it, under <launcher>/artifacts/agents/ui/launches/, owner-only.
func writeSessionLaunchRecord(t *testing.T, launcher, destination string, mutate func(*launch.Record)) string {
	t.Helper()
	record := launch.Record{
		Launch:      sessionArmTestLaunch,
		Machine:     "m1f",
		Destination: destination,
		Enrollment: &launch.Enrollment{
			Kind: launch.EnrollmentHumanSession, Provider: "browser", Human: "wido",
			Session: "session-01", At: "2026-09-29T08:00:00Z",
		},
	}
	if mutate != nil {
		mutate(&record)
	}
	path, err := launch.Path(launcher, sessionArmTestLaunch)
	if err != nil {
		t.Fatal(err)
	}
	if err := launch.SaveAt(path, record, launcher); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func canonicalTestDir(t *testing.T) string {
	t.Helper()
	dir, err := canonicalPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func fixedRepositoryTop(top string) func(string) (string, error) {
	return func(string) (string, error) { return top, nil }
}

func TestSessionArmReadsTheRecordsVerdict(t *testing.T) {
	t.Parallel()
	launcher, clone := canonicalTestDir(t), canonicalTestDir(t)
	path := writeSessionLaunchRecord(t, launcher, clone, nil)
	got, err := sessionEnrollmentFromRecord(filepath.Join(clone, "metasystem"), path, fixedRepositoryTop(clone))
	if err != nil {
		t.Fatalf("a genuine session record was refused: %v", err)
	}
	want := steward.EnrolledSession{Provider: "browser", Human: "wido", Reference: "session-01", Launch: sessionArmTestLaunch, From: launcher}
	if got != want {
		t.Fatalf("enrolled session = %+v, want %+v", got, want)
	}
}

func TestSessionArmRefusesABadRecordByName(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name   string
		mutate func(*launch.Record)
		mode   os.FileMode
		want   string
	}{
		{name: "group-readable", mode: 0o640, want: "is not owner-only (mode 640)"},
		{name: "world-readable", mode: 0o604, want: "is not owner-only (mode 604)"},
		{name: "no enrollment", mutate: func(r *launch.Record) { r.Enrollment = nil }, want: "carries no signed-in session enrollment"},
		{name: "other kind", mutate: func(r *launch.Record) { r.Enrollment.Kind = "human-terminal" }, want: `enrollment kind "human-terminal", not "human-session"`},
		{name: "no provider", mutate: func(r *launch.Record) { r.Enrollment.Provider = "" }, want: "enrollment has no provider"},
		{name: "no human", mutate: func(r *launch.Record) { r.Enrollment.Human = " " }, want: "enrollment has no human"},
		{name: "no session", mutate: func(r *launch.Record) { r.Enrollment.Session = "" }, want: "enrollment has no session"},
		{name: "other launch id", mutate: func(r *launch.Record) { r.Launch = "01M3NZQN2WCRCZB8JVHE8XPH5A" }, want: "names launch 01M3NZQN2WCRCZB8JVHE8XPH5A"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			launcher, clone := canonicalTestDir(t), canonicalTestDir(t)
			path := writeSessionLaunchRecord(t, launcher, clone, test.mutate)
			if test.mode != 0 {
				if err := os.Chmod(path, test.mode); err != nil {
					t.Fatal(err)
				}
			}
			_, err := sessionEnrollmentFromRecord(clone, path, fixedRepositoryTop(clone))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("record %s: error = %v, want it to name %q", test.name, err, test.want)
			}
		})
	}
}

func TestSessionArmRefusesAnotherCheckoutsDestination(t *testing.T) {
	t.Parallel()
	launcher, clone, other := canonicalTestDir(t), canonicalTestDir(t), canonicalTestDir(t)
	path := writeSessionLaunchRecord(t, launcher, other, nil)
	_, err := sessionEnrollmentFromRecord(clone, path, fixedRepositoryTop(clone))
	want := "names destination " + other + ", not this repository " + clone
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("other destination: error = %v, want %q", err, want)
	}
}

func TestSessionArmRefusesARecordOutsideALaunchesDirectory(t *testing.T) {
	t.Parallel()
	launcher, clone := canonicalTestDir(t), canonicalTestDir(t)
	path := writeSessionLaunchRecord(t, launcher, clone, nil)
	stray := filepath.Join(launcher, sessionArmTestLaunch+".json")
	if err := os.Rename(path, stray); err != nil {
		t.Fatal(err)
	}
	_, err := sessionEnrollmentFromRecord(clone, stray, fixedRepositoryTop(clone))
	if err == nil || !strings.Contains(err.Error(), "is not a launch record under a checkout's artifacts/agents/ui/launches") {
		t.Fatalf("stray record: error = %v", err)
	}
}

func TestSessionArmRefusesAnAbsentRecord(t *testing.T) {
	t.Parallel()
	launcher, clone := canonicalTestDir(t), canonicalTestDir(t)
	path, err := launch.Path(launcher, sessionArmTestLaunch)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sessionEnrollmentFromRecord(clone, path, fixedRepositoryTop(clone)); err == nil || !strings.Contains(err.Error(), "launch record absent") {
		t.Fatalf("absent record: error = %v", err)
	}
}

func TestSessionArmClassifiesItsCaller(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		class    string
		admitted bool
	}{
		{class: lease.ClassMain},
		{class: lease.ClassDelegate},
		{class: lease.ClassSteward},
		{class: lease.ClassSupervision},
		{class: lease.ClassAdapterSupervisor},
		{class: lease.ClassHuman, admitted: true},
		{class: lease.ClassUntrusted, admitted: true},
	} {
		t.Run(test.class, func(t *testing.T) {
			t.Parallel()
			classify := func(repo, root string, caller int64) (lease.Classification, error) {
				if caller != int64(os.Getppid()) {
					t.Errorf("classified pid %d, want the arm's parent %d", caller, os.Getppid())
				}
				return lease.Classification{Class: test.class}, nil
			}
			err := sessionCallerCheck("/fixture/clone", "/fixture/clone", classify)
			if test.admitted {
				if err != nil {
					t.Fatalf("%s caller refused: %v", test.class, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), "caller classified "+test.class) {
				t.Fatalf("%s caller: error = %v, want a refusal naming the class", test.class, err)
			}
		})
	}
	unreadable := func(string, string, int64) (lease.Classification, error) {
		return lease.Classification{}, errors.New("process table unreadable")
	}
	if err := sessionCallerCheck("/fixture/clone", "/fixture/clone", unreadable); err == nil || !strings.Contains(err.Error(), "process table unreadable") {
		t.Fatalf("unreadable ancestry: error = %v", err)
	}
}

func TestSessionArmRefusesTheTemporaryPairBesideARecord(t *testing.T) {
	for _, pair := range [][]string{
		{"--temporary-human-word", "Wido authorizes", "--review-by", "2026-10-09"},
		{"--temporary-human-word", "Wido authorizes"},
		{"--review-by", "2026-10-09"},
	} {
		called := false
		deps := stewardArmDeps{repositoryTop: fixedRepositoryTop("/fixture/clone"), landingRefGit: noGitForLandingRef{},
			armSession: func(string, string, steward.EnrolledSession, string) (string, error) {
				called = true
				return "", nil
			}}
		args := append([]string{"--repo", t.TempDir(), "--launch-record", "/fixture/record.json"}, pair...)
		stderr, code := captureStderr(t, func() int { return runStewardArmWith(args, deps) })
		if code != 2 || called || !strings.Contains(stderr, "--launch-record cannot be combined with --temporary-human-word or --review-by") {
			t.Fatalf("pair %v beside --launch-record = code %d called %v stderr %q", pair, code, called, stderr)
		}
	}
}

/* ------------------------------------------ the real classifier, once -- */

// The detached chain the browser's launch really runs: `ui serve` spawns
// `seat launch` under setsid with no terminal, and `seat launch` runs the
// clone's `steward arm --launch-record`. The launcher stand-in below is a
// child started with Setsid and no terminal; the arm is a grandchild running
// the whole verb from an installed engine path, classified by lease.ClassifyAt
// against the kernel. The installation is fixture-mode (metasystem.runtimes=
// fake) only so the agent CLI this suite may run under is not a recognised
// runtime; nothing about the chain is staged.
const (
	sessionArmLauncherHelper = "test-helper-session-arm-launcher"
	sessionArmVerbHelper     = "test-helper-session-arm-verb"
	sessionArmOutEnv         = "METASYSTEM_TEST_SESSION_ARM_OUT"
	sessionArmCloneEnv       = "METASYSTEM_TEST_SESSION_ARM_CLONE"
)

// noGitForLandingRef is a checkout Git cannot read: landing-ref seeding
// reports not seeded and nothing reaches a real Git.
type noGitForLandingRef struct{}

func (noGitForLandingRef) Output(string, ...string) ([]byte, error) {
	return nil, errors.New("no git in this fixture")
}

func (noGitForLandingRef) CombinedOutput(string, ...string) ([]byte, error) {
	return nil, errors.New("no git in this fixture")
}

func init() {
	testHelperCommands[sessionArmLauncherHelper] = func(args []string) int {
		if len(args) < 1 {
			return 2
		}
		arm := exec.Command(args[0], append([]string{sessionArmVerbHelper}, args[1:]...)...)
		arm.Stdin, arm.Stdout, arm.Stderr = nil, os.Stdout, os.Stderr
		if err := arm.Run(); err != nil {
			var exit *exec.ExitError
			if errors.As(err, &exit) {
				return exit.ExitCode()
			}
			return 1
		}
		return 0
	}
	testHelperCommands[sessionArmVerbHelper] = func(args []string) int {
		// The clone is synthetic: its repository top is the directory the
		// test names, and the landing-ref seeding sees no Git at all.
		clone := os.Getenv(sessionArmCloneEnv)
		return runStewardArmWith(args, stewardArmDeps{
			repositoryTop: fixedRepositoryTop(clone),
			landingRefGit: noGitForLandingRef{},
			armSession: func(repo, bin string, session steward.EnrolledSession, lineage string) (string, error) {
				data, err := json.Marshal(session)
				if err != nil {
					return "", err
				}
				if err := os.WriteFile(os.Getenv(sessionArmOutEnv), data, 0o600); err != nil {
					return "", err
				}
				return "steward armed (test)", nil
			},
		})
	}
}

func installTestEngine(t *testing.T, root string) string {
	t.Helper()
	engine := filepath.Join(root, "bin", "metasystem")
	if err := os.MkdirAll(filepath.Dir(engine), 0o755); err != nil {
		t.Fatal(err)
	}
	source := commandTestExecutable(t)
	if err := os.Link(source, engine); err == nil {
		return engine
	}
	// A copy is an executable a parallel test may fork beside: it is
	// written through testexec, under the fork lock.
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(engine, data, 0o755); err != nil {
		t.Fatal(err)
	}
	return engine
}

func TestSessionArmAdmitsTheGenuineDetachedChainThroughTheRealClassifier(t *testing.T) {
	t.Parallel()
	launcher, clone := canonicalTestDir(t), canonicalTestDir(t)
	if err := os.WriteFile(filepath.Join(clone, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	engine := installTestEngine(t, clone)
	record := writeSessionLaunchRecord(t, launcher, clone, nil)
	out := filepath.Join(t.TempDir(), "armed.json")

	launcherProcess := exec.Command(engine, sessionArmLauncherHelper, engine, "--repo", clone, "--launch-record", record)
	launcherProcess.Dir = clone
	// PATH names an empty directory: no Git is reachable from the chain.
	launcherProcess.Env = fixtureCommandEnvironment(t, sessionArmOutEnv+"="+out, sessionArmCloneEnv+"="+clone, "PATH="+t.TempDir())
	launcherProcess.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	output, err := launcherProcess.CombinedOutput()
	if err != nil {
		t.Fatalf("detached chain refused: %v\n%s", err, output)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("the arm was never called: %v\n%s", err, output)
	}
	var got steward.EnrolledSession
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	want := steward.EnrolledSession{Provider: "browser", Human: "wido", Reference: "session-01", Launch: sessionArmTestLaunch, From: launcher}
	if got != want {
		t.Fatalf("armed session = %+v, want %+v\n%s", got, want, output)
	}
}
