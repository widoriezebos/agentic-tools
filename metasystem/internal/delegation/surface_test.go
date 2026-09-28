package delegation_test

import (
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegation"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegation/fake"
)

func TestEnvFromEnvironReadsEachInvocationVariable(t *testing.T) {
	t.Parallel()
	environ := map[string]string{
		"METASYSTEM_DELEGATE_OUTCOME_FILE":            "/outcome",
		"METASYSTEM_DELEGATE_INTERNAL":                "1",
		"METASYSTEM_DELEGATE_CLAIM_CAPABILITY":        "cap",
		"METASYSTEM_OWNER_LINEAGE":                    "lineage",
		"METASYSTEM_MISSION_ID":                       "m1",
		"METASYSTEM_MISSION_LEASE":                    "lease",
		"METASYSTEM_MISSION_TURN":                     "t1",
		"METASYSTEM_FIXTURE_CAP_SCALE_MILLI":          "500",
		"METASYSTEM_HANDSHAKE_POLL_INTERVAL_MS":       "20",
		"METASYSTEM_DISPATCH_FIXTURE_HAZARD":          "hazard",
		"METASYSTEM_CHECKOUT_EXECUTION_GUARD_FIXTURE": "guard",
		"METASYSTEM_CHECKOUT_EXECUTION_GUARD_ROOT":    "/guard-root",
		"METASYSTEM_FIXTURE_PAUSE_BEFORE_LAUNCH":      "/pause",
		"TMPDIR":                                      "/tmpdir",
	}
	got := delegation.EnvFromEnviron(func(key string) (string, bool) {
		value, ok := environ[key]
		return value, ok
	})
	want := delegation.Env{
		RecordOutcome: true, DelegateInternal: true, ClaimCapability: "cap", OwnerLineage: "lineage",
		MissionID: "m1", MissionLease: "lease", MissionTurn: "t1", FixtureCapScaleMilli: "500",
		HandshakePollMS: "20", FixtureHazard: "hazard", GuardFixture: "guard", GuardFixtureRoot: "/guard-root",
		FixturePauseBeforeLaunch: "/pause", TempDir: "/tmpdir",
	}
	if got != want {
		t.Fatalf("EnvFromEnviron = %+v\nwant %+v", got, want)
	}
	// The internal marker is exactly "1"; any other value is an operator.
	environ = map[string]string{"METASYSTEM_DELEGATE_INTERNAL": "true"}
	if got := delegation.EnvFromEnviron(func(key string) (string, bool) {
		value, ok := environ[key]
		return value, ok
	}); got != (delegation.Env{}) {
		t.Fatalf("a marker other than 1 read as %+v, want the zero Env", got)
	}
}

func TestLockTagOfIsTheWholeCommandLine(t *testing.T) {
	t.Parallel()
	if got := delegation.LockTagOf([]string{"/bin/metasystem", "internal", "delegate", "--cancel", "j1"}); got != "/bin/metasystem internal delegate --cancel j1" {
		t.Fatalf("LockTagOf = %q", got)
	}
	if got := delegation.LockTagOf(nil); got != "" {
		t.Fatalf("LockTagOf(nil) = %q", got)
	}
}

func TestErrorTextsNameTheirExit(t *testing.T) {
	t.Parallel()
	cases := []struct {
		err  error
		want string
	}{
		{&delegation.Exit{Code: 4}, "exit 4"},
		{&delegation.Exit{Code: 2, Message: "refused: bad brief"}, "refused: bad brief"},
		{&delegation.GitExitError{Code: 128, Stderr: "not a repository"}, "git exited 128: not a repository"},
		{&delegation.AdapterCancelError{Code: 5}, "adapter cancel exited 5"},
		{&delegation.AdapterCancelError{Code: 5, Output: "runtime refused"}, "runtime refused"},
	}
	for _, tc := range cases {
		if got := tc.err.Error(); got != tc.want {
			t.Errorf("%T.Error() = %q, want %q", tc.err, got, tc.want)
		}
	}
	if delegation.ExitCode(&delegation.Exit{Code: 4}) != 4 {
		t.Fatal("ExitCode lost the exit code")
	}
}

func TestLifecycleExposesItsRootAndPorts(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	doubles := fake.NewSet()
	ports := doubles.Ports()
	life, err := delegation.New(delegation.Config{Root: root, RepoScope: root}, ports)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if life.Root() != resolved {
		t.Fatalf("Root() = %q, want the resolved %q", life.Root(), root)
	}
	if !reflect.DeepEqual(life.Ports(), ports) {
		t.Fatal("Ports() is not the wired port set")
	}
}

// TestOwnerProcessAnswersFromTheKernel drives the real process owner
// against this test process and a pid no kernel issues, and pins the
// refusals that keep it from ever signalling init or every group.
func TestOwnerProcessAnswersFromTheKernel(t *testing.T) {
	t.Parallel()
	const absent int64 = 1 << 30
	root := t.TempDir()
	ports := ownerPorts(t, root)
	process := ports.Process
	own := int64(os.Getpid())
	group := int64(syscall.Getpgrp())

	if !process.Exists(own) || process.Exists(0) || process.Exists(absent) {
		t.Fatal("Exists does not follow the kernel")
	}
	if !process.GroupExists(group) || process.GroupExists(0) || process.GroupExists(absent) {
		t.Fatal("GroupExists does not follow the kernel")
	}
	if got := process.TagState(absent, "job-1"); got != "dead" {
		t.Fatalf("TagState(absent) = %q, want dead", got)
	}
	if got := process.TagState(own, "delegation-test-tag-carried-by-nothing"); got != "stale" {
		t.Fatalf("TagState(own, foreign tag) = %q, want stale", got)
	}
	if started, err := process.StartedAt(own); err != nil || started < 1 {
		t.Fatalf("StartedAt(own) = %d, %v", started, err)
	}
	if _, err := process.StartedAt(absent); err == nil {
		t.Fatal("StartedAt(absent) reported a start")
	}
	for _, pgid := range []int64{0, 1} {
		if err := process.SignalGroup(pgid, delegation.SignalTerm); err == nil {
			t.Fatalf("SignalGroup(%d) was not refused", pgid)
		}
		if process.GroupOwned("", pgid, "job-1") {
			t.Fatalf("GroupOwned(%d) claimed ownership", pgid)
		}
	}
	if process.GroupOwned("", group, "") {
		t.Fatal("GroupOwned without a tag claimed ownership")
	}
	claim, err := process.ClaimProcesses()
	if err != nil || claim.Reader == nil || claim.Scanner == nil || claim.Verifier == nil {
		t.Fatalf("ClaimProcesses = %+v, %v", claim, err)
	}
	if now := ports.Clock.Now(); now.IsZero() {
		t.Fatal("the owner clock reads zero")
	}
}
