package proofrun

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostload"
)

// installFakeLoad scripts the host sample and the host-wide launcher count
// the attribution reads, so no scheduler is in the loop.
func installFakeLoad(t *testing.T, sample hostload.Sample, launchers int, known bool) {
	t.Helper()
	previous := loadSeams
	loadSeams.host = func(now time.Time) hostload.Sample {
		sample.At = now.UTC().Format(time.RFC3339Nano)
		return sample
	}
	loadSeams.launchers = func(int64) (int, bool) { return launchers, known }
	t.Cleanup(func() { loadSeams = previous })
}

func TestIsProofLauncherArgv(t *testing.T) {
	for argv, want := range map[string]bool{
		"/seat/m1b/metasystem/bin/metasystem proof-run launch --suite testing --root /x": true,
		"metasystem proof-run launch":                true,
		"metasystem proof-run status":                false,
		"metasystem launch proof-run":                false,
		"perl -e sleep proof-run launch":             false,
		"bash -c metasystem proof-run launch":        false,
		"/seat/bin/metasystem.test proof-run launch": false,
		"go test ./internal/proofrun -run launch":    false,
		"": false,
	} {
		if got := isProofLauncherArgv(strings.Fields(argv)); got != want {
			t.Fatalf("%q recognised as launcher = %v, want %v", argv, got, want)
		}
	}
}

func TestTopLevelLaunchersCountBatteriesNotTheirFamilies(t *testing.T) {
	// pid 1 launchd; 90 a joined launch above this seat's battery launcher
	// 100 (self), which runs a bed launcher 110 under a fixture shell 105;
	// 200 another seat's battery launcher with its own nested launcher 210;
	// 300 a lone launcher; 400 a process that only looks busy.
	rows := []processRow{
		{pid: 1}, {pid: 90, parent: 1, launcher: true}, {pid: 100, parent: 90, launcher: true},
		{pid: 105, parent: 100}, {pid: 110, parent: 105, launcher: true},
		{pid: 200, parent: 1, launcher: true}, {pid: 205, parent: 200}, {pid: 210, parent: 205, launcher: true},
		{pid: 300, parent: 1, launcher: true}, {pid: 400, parent: 1},
	}
	if got := topLevelLaunchers(rows, 100); got != 2 {
		t.Fatalf("top-level launchers outside self's family = %d, want 2 (200 and 300)", got)
	}
	if got := topLevelLaunchers(rows, 300); got != 2 {
		t.Fatalf("seen from the lone launcher = %d, want 2 (the joined battery 90 counts once, and 200)", got)
	}
	if got := topLevelLaunchers(rows, 999); got != 3 {
		t.Fatalf("seen from a process outside every family = %d, want 3 (90's battery, 200's, 300)", got)
	}
	if got := topLevelLaunchers(nil, 100); got != 0 {
		t.Fatalf("an empty table counted %d", got)
	}
}

func TestLoadSampleLoadedAndDescribe(t *testing.T) {
	quiet := LoadSample{Sample: hostload.Sample{Available: true, Cores: 18, Load1m: 3.5, Load5m: 4, Load15m: 4.5}, OverlapKnown: true}
	if quiet.Loaded() {
		t.Fatalf("a quiet host read loaded: %s", quiet.Describe())
	}
	// One other launcher on an otherwise idle box is overlap, not load.
	overlapped := quiet
	overlapped.OverlappingHost = 1
	if overlapped.Loaded() || !strings.Contains(overlapped.Describe(), "1 other proof launcher(s) on the host") {
		t.Fatalf("one launcher on an idle box read loaded: %s", overlapped.Describe())
	}
	// Another launcher with the load at half the cores is load.
	crowded := overlapped
	crowded.Load1m = 9
	if !crowded.Loaded() {
		t.Fatalf("a launcher on a half-loaded box did not read loaded: %s", crowded.Describe())
	}
	// A saturated box is load whatever the count.
	saturated := quiet
	saturated.Load1m = 25.2
	if !saturated.Loaded() || !strings.Contains(saturated.Describe(), "load 25.20/4.00/4.50 on 18 cores") {
		t.Fatalf("a saturated host did not read loaded: %s", saturated.Describe())
	}
	unknown := LoadSample{Sample: hostload.Sample{Cores: 18, Detail: "no reader"}, OverlappingLocal: 1}
	if unknown.Loaded() || !strings.Contains(unknown.Describe(), "host overlap unknown") || !strings.Contains(unknown.Describe(), "load unavailable") {
		t.Fatalf("an unreadable host was not described as such: %s", unknown.Describe())
	}
}

func TestConsumptionBounded(t *testing.T) {
	for _, rule := range []string{"cpu-budget/none+zero-window/30m", "cpu-budget/600s+zero-window/none", "cpu-budget/none+zero-window/30m+ticks/assumed-100"} {
		if !consumptionBounded(rule) {
			t.Fatalf("%q is a consumption bound", rule)
		}
	}
	for _, rule := range []string{"cpu-budget/none+zero-window/none", "cpu-budget/none+zero-window/none+ticks/assumed-100", "", "ticks/assumed-100"} {
		if consumptionBounded(rule) {
			t.Fatalf("%q is no consumption bound", rule)
		}
	}
}

func TestReserveAndFinalizeRecordTheHostLoad(t *testing.T) {
	installFakeLoad(t, hostload.Sample{Available: true, Cores: 18, Load1m: 25.2, Load5m: 21.8, Load15m: 17.8}, 2, true)
	root, identity := proofAttemptFixture(t, "load-attribution")
	launcher, err := CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 12, 17, 0, 0, 0, time.UTC)
	request := AdmissionRequest{ControlRoot: root, ExecutionRoot: root, GoalID: "goal-a", GoalRevision: 3,
		AccountingRevision: 2, ReservedMinutes: 4, Identity: identity, Launcher: launcher, Now: now}
	attempt, _, err := ReserveLocked(request)
	if err != nil {
		t.Fatal(err)
	}
	if attempt.Load == nil || attempt.Load.End != nil || attempt.Load.Start.OverlappingHost != 2 || !attempt.Load.Start.OverlapKnown ||
		attempt.Load.Start.Load1m != 25.2 || attempt.Load.Start.Cores != 18 || attempt.Load.Start.At != "2026-09-12T17:00:00Z" {
		t.Fatalf("start sample = %+v", attempt.Load)
	}
	stored, err := ReadAttempt(root, attempt.AttemptID)
	if err != nil || stored.Load == nil || stored.Load.Start.OverlappingHost != 2 {
		t.Fatalf("the start sample did not survive the record: %+v, %v", stored.Load, err)
	}
	// A failure while the host is still crowded is attributed to the load.
	installFakeLoad(t, hostload.Sample{Available: true, Cores: 18, Load1m: 30, Load5m: 25, Load15m: 20}, 3, true)
	failed, err := FinalizeAttempt(root, attempt.AttemptID, TerminalFailed, 23, "gate failed", nil, now.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if failed.Load.End == nil || failed.Load.End.OverlappingHost != 3 || failed.Load.End.At != "2026-09-12T17:02:00Z" || failed.Terminal.Attribution != LoadAttribution {
		t.Fatalf("end sample or attribution = %+v, terminal %+v", failed.Load.End, failed.Terminal)
	}
	// A success under load carries the sample but no attribution.
	green := identity
	green.CommandClass = "green-under-load"
	green.IdentityDigest = green.digest()
	request.Identity = green
	succeeded, _, err := ReserveLocked(request)
	if err != nil {
		t.Fatal(err)
	}
	succeeded, err = FinalizeAttempt(root, succeeded.AttemptID, TerminalSuccess, 0, "green", nil, now.Add(time.Minute))
	if err != nil || succeeded.Load.End == nil || succeeded.Terminal.Attribution != "" {
		t.Fatalf("success under load = %+v, %v", succeeded.Terminal, err)
	}
	// A cancellation under load is not a failure the load caused: the
	// sample is recorded, the label is not.
	cancelledIdentity := identity
	cancelledIdentity.CommandClass = "cancelled-under-load"
	cancelledIdentity.IdentityDigest = cancelledIdentity.digest()
	request.Identity = cancelledIdentity
	cancelled, _, err := ReserveLocked(request)
	if err != nil {
		t.Fatal(err)
	}
	cancelled, err = FinalizeAttempt(root, cancelled.AttemptID, TerminalCancelled, 1, "stop", nil, now.Add(time.Minute))
	if err != nil || cancelled.Load.End == nil || !cancelled.Load.End.Loaded() || cancelled.Terminal.Attribution != "" {
		t.Fatalf("cancellation under load = %+v, %v", cancelled.Terminal, err)
	}
	// A failure on a quiet host is nobody's load.
	installFakeLoad(t, hostload.Sample{Available: true, Cores: 18, Load1m: 2, Load5m: 2, Load15m: 2}, 0, true)
	quiet := identity
	quiet.CommandClass = "failed-quietly"
	quiet.IdentityDigest = quiet.digest()
	request.Identity = quiet
	lonely, _, err := ReserveLocked(request)
	if err != nil {
		t.Fatal(err)
	}
	lonely, err = FinalizeAttempt(root, lonely.AttemptID, TerminalFailed, 1, "red", nil, now.Add(time.Minute))
	if err != nil || lonely.Terminal.Attribution != "" || lonely.Load.End.Loaded() {
		t.Fatalf("quiet failure = %+v, %+v, %v", lonely.Terminal, lonely.Load.End, err)
	}
}

func TestOldRecordsFinalizeWithoutAFabricatedStartSample(t *testing.T) {
	installFakeLoad(t, hostload.Sample{Available: true, Cores: 18, Load1m: 30}, 2, true)
	root, identity := proofAttemptFixture(t, "old-record")
	launcher, err := CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 12, 17, 0, 0, 0, time.UTC)
	attempt, _, err := ReserveLocked(AdmissionRequest{ControlRoot: root, ExecutionRoot: root, GoalID: "goal-a", GoalRevision: 3,
		AccountingRevision: 2, ReservedMinutes: 4, Identity: identity, Launcher: launcher, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	// The record as an engine before the attribution wrote it: no load block.
	attempt.Load = nil
	if err := writeAttempt(attempt); err != nil {
		t.Fatal(err)
	}
	finished, err := FinalizeAttempt(root, attempt.AttemptID, TerminalFailed, 1, "red", nil, now.Add(2*time.Minute))
	if err != nil || finished.Load == nil || finished.Load.Start.Available || finished.Load.Start.Detail != "not sampled at start" ||
		finished.Load.End == nil || !finished.Load.End.Available || finished.Terminal.Attribution != LoadAttribution {
		t.Fatalf("old record finalized = %+v, terminal %+v, %v", finished.Load, finished.Terminal, err)
	}
}

func TestRetryDecisionNamesThePriorAttemptsLoad(t *testing.T) {
	installFakeLoad(t, hostload.Sample{Available: true, Cores: 18, Load1m: 25.2, Load5m: 21.8, Load15m: 17.8}, 2, true)
	root, identity := proofAttemptFixture(t, "retry-under-load")
	launcher, err := CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 12, 17, 0, 0, 0, time.UTC)
	request := AdmissionRequest{ControlRoot: root, ExecutionRoot: root, GoalID: "goal-a", GoalRevision: 3,
		AccountingRevision: 2, ReservedMinutes: 4, Identity: identity, Launcher: launcher, Now: now}
	failed, _, err := ReserveLocked(request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := FinalizeAttempt(root, failed.AttemptID, TerminalFailed, 23, "gate failed", nil, now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	evidencePath := filepath.Join(root, "failure.log")
	if err := os.WriteFile(evidencePath, []byte("the coverage group timed out under load\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	decisionPath := filepath.Join(root, "retry.json")
	encoded, _ := json.Marshal(RetryDecision{SchemaVersion: 1, PriorAttempt: failed.AttemptID, Cause: "load", EvidencePath: "failure.log", Rationale: "the box was crowded"})
	if err := os.WriteFile(decisionPath, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	request.RetryDecisionPath = decisionPath
	request.Now = now.Add(3 * time.Minute)
	retry, _, err := ReserveLocked(request)
	if err != nil || retry.Retry == nil {
		t.Fatalf("retry = %+v, %v", retry, err)
	}
	if retry.Retry.PriorLoad == nil || retry.Retry.PriorLoad.End == nil || retry.Retry.PriorLoad.End.OverlappingHost != 2 ||
		!strings.HasPrefix(retry.Retry.PriorAttribution, "load: 2 other proof launcher(s) on the host") {
		t.Fatalf("the retry does not name the prior's load: %+v %q", retry.Retry.PriorLoad, retry.Retry.PriorAttribution)
	}
}

func TestFilePatienceDefectsNamesFailedConsumptionBoundedGroups(t *testing.T) {
	root := t.TempDir()
	attempt := Attempt{AttemptID: "proof-load-1", GoalID: "goal-a", ControlRoot: root, Terminal: &AttemptTerminal{Result: TerminalFailed}, TestResult: &TestResult{Groups: []GroupResult{
		{ID: "internal/goal", Status: "failed", ProgressRule: "cpu-budget/none+zero-window/30m"},
		{ID: "internal/lease", Status: "passed", ProgressRule: "cpu-budget/none+zero-window/30m"},
		{ID: "section/land-fixtures", Status: "failed", ProgressRule: "cpu-budget/none+zero-window/none"},
		{ID: "coverage", Status: "failed", ProgressRule: "cpu-budget/600s+zero-window/none"},
	}}}
	crowded := LoadSample{Sample: hostload.Sample{At: "2026-09-12T17:02:00Z", Available: true, Cores: 18, Load1m: 30}, OverlappingHost: 3, OverlapKnown: true}
	filed, err := filePatienceDefects(attempt, crowded)
	if err != nil || len(filed) != 2 || filed[0].Group != "internal/goal" || filed[1].Group != "coverage" || filed[0].Load.OverlappingHost != 3 {
		t.Fatalf("filed = %+v, %v", filed, err)
	}
	data, err := os.ReadFile(PatienceDefectsPath(root))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("record lines = %q", lines)
	}
	var first PatienceDefect
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil || first.AttemptID != "proof-load-1" || first.Group != "internal/goal" || first.ProgressRule != "cpu-budget/none+zero-window/30m" {
		t.Fatalf("first line = %+v, %v", first, err)
	}
	// A quiet host files nothing; neither does an attempt without testing
	// evidence, nor a cancelled one.
	quiet := crowded
	quiet.OverlappingHost, quiet.Load1m = 0, 2
	if filed, err := filePatienceDefects(attempt, quiet); err != nil || filed != nil {
		t.Fatalf("a quiet host filed %+v, %v", filed, err)
	}
	if filed, err := filePatienceDefects(Attempt{AttemptID: "proof-load-2", ControlRoot: root, Terminal: &AttemptTerminal{Result: TerminalFailed}}, crowded); err != nil || filed != nil {
		t.Fatalf("an attempt without testing evidence filed %+v, %v", filed, err)
	}
	cancelled := attempt
	cancelled.Terminal = &AttemptTerminal{Result: TerminalCancelled}
	if filed, err := filePatienceDefects(cancelled, crowded); err != nil || filed != nil {
		t.Fatalf("a cancelled attempt filed %+v, %v", filed, err)
	}
}

func TestUnreadableAttemptRecordsAreSkippedNotFatal(t *testing.T) {
	installFakeLoad(t, hostload.Sample{Available: true, Cores: 18, Load1m: 1}, 0, true)
	root, identity := proofAttemptFixture(t, "skip-unreadable")
	launcher, err := CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 12, 17, 0, 0, 0, time.UTC)
	good, _, err := ReserveLocked(AdmissionRequest{ControlRoot: root, ExecutionRoot: root, GoalID: "goal-a", GoalRevision: 3,
		AccountingRevision: 2, ReservedMinutes: 4, Identity: identity, Launcher: launcher, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	// A record whose control root no longer matches its path (a checkout
	// moved, a record copied): unreadable by design, and it must not hide
	// the good one.
	data, err := os.ReadFile(filepath.Join(attemptsDir(root), good.AttemptID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	damaged := strings.Replace(string(data), good.AttemptID, "proof-aaaaaaaa-0000000000000000", 1)
	if err := os.WriteFile(filepath.Join(attemptsDir(root), "proof-aaaaaaaa-0000000000000000.json"), []byte(strings.Replace(damaged, root, "/elsewhere", 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	attempts, unreadable := readAttemptsSkippingUnreadable(root)
	if len(attempts) != 1 || attempts[0].AttemptID != good.AttemptID || len(unreadable) != 1 || !strings.Contains(unreadable[0], "contradicts its path") {
		t.Fatalf("attempts = %d, unreadable = %v", len(attempts), unreadable)
	}
	live, err := LiveAttempts(root, nil)
	if err != nil || len(live) != 2 || live[0].Unreadable == "" || live[1].AttemptID != good.AttemptID {
		t.Fatalf("live = %+v, %v", live, err)
	}
	if liveAttemptsOtherThan(root, "proof-other") != 1 {
		t.Fatal("the live count lost the readable attempt")
	}
}
