package fake

import (
	"context"
	"errors"
	"io"
	"reflect"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegation"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gaterun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

// TestUnsetDoublesAnswerZeroAndRecordInOrder pins the package contract: an
// unset function field answers the zero value and nil (or the documented
// refusal), and every call lands in the shared Log in call order.
func TestUnsetDoublesAnswerZeroAndRecordInOrder(t *testing.T) {
	t.Parallel()
	set := NewSet()
	ctx := context.Background()
	inv := delegation.Invocation{CallerPid: 7}
	one := int64(1)

	if got, err := set.Lease.Classify(inv); err != nil || !reflect.DeepEqual(got, lease.ClassifyResult{}) {
		t.Fatalf("Classify = %+v, %v", got, err)
	}
	if got, err := set.Lease.RequireHolder(inv, &one); err != nil || !reflect.DeepEqual(got, lease.HolderView{}) {
		t.Fatalf("RequireHolder = %+v, %v", got, err)
	}
	if got, err := set.Lease.Renew(inv); err != nil || got != (lease.RenewResult{}) {
		t.Fatalf("Renew = %+v, %v", got, err)
	}
	if err := set.Lease.Authorize(inv, delegation.AuthorityHolderOnly, "j1"); err != nil {
		t.Fatal(err)
	}
	if got, err := set.Steward.AuthorizeDispatch(inv, "intent"); err != nil || got != (steward.DispatchAuthorization{}) {
		t.Fatalf("AuthorizeDispatch = %+v, %v", got, err)
	}
	if !set.Adapter.Installed("codex") {
		t.Fatal("an unlisted runtime reported not installed")
	}
	if got, err := set.Adapter.ConfigIdentity(ctx, "codex"); err != nil || got != "" {
		t.Fatalf("ConfigIdentity = %q, %v", got, err)
	}
	if err := set.Adapter.Probe(ctx, "codex"); err != nil {
		t.Fatal(err)
	}
	if got, err := set.Adapter.OutputStream(ctx, "codex", "/round"); err != nil || got != "" {
		t.Fatalf("OutputStream = %q, %v", got, err)
	}
	if pid, err := set.Adapter.Launch(ctx, delegation.AdapterLaunch{Runtime: "codex", Verb: "dispatch", Job: "j1"}); err != nil || pid != 0 {
		t.Fatalf("Launch = %d, %v", pid, err)
	}
	if err := set.Adapter.Cancel(ctx, "codex", "j1"); err != nil {
		t.Fatal(err)
	}
	if err := set.Adapter.ResultPatch("/out", "none", "done", "/usage"); err != nil {
		t.Fatal(err)
	}
	if got := set.Goal.LedgerIdentity(); got != "" {
		t.Fatalf("LedgerIdentity = %q", got)
	}
	if got, err := set.Goal.Binding("g1"); err != nil || got != (delegation.GoalBinding{}) {
		t.Fatalf("Binding = %+v, %v", got, err)
	}
	if err := set.Records.Create("j1", "/src"); err != nil {
		t.Fatal(err)
	}
	if err := set.Records.Setup("j1", "/src"); err != nil {
		t.Fatal(err)
	}
	if got, err := set.Records.CAS("j1", "a", "b", "/patch"); err != nil || got != "" {
		t.Fatalf("CAS = %q, %v", got, err)
	}
	if got, err := set.Records.Read("j1"); err != nil || got != nil {
		t.Fatalf("Read = %v, %v", got, err)
	}
	if got := set.Host.WaitJob(ctx, "/root", "j1", 7); got != (delegation.WaitOutcome{}) {
		t.Fatalf("WaitJob = %+v", got)
	}
	if got := set.Host.WatchJob(ctx, "/root", "j1", 7, "/progress"); got != 0 {
		t.Fatalf("WatchJob = %d", got)
	}
	if got, err := set.Guard.Acquire("/root", 7, "suite", time.Second, time.Second, io.Discard); err != nil || got != gaterun.GuardAcquired {
		t.Fatalf("Acquire = %v, %v", got, err)
	}
	if err := set.Guard.Release("/root", 7); err != nil {
		t.Fatal(err)
	}

	want := []string{
		"lease.Classify pid=7",
		"lease.RequireHolder pid=7 epoch=1",
		"lease.Renew pid=7",
		"lease.Authorize pid=7 mode=holder-only job=j1",
		"steward.AuthorizeDispatch pid=7 intent=intent",
		"adapter.Installed runtime=codex",
		"adapter.ConfigIdentity runtime=codex",
		"adapter.Probe runtime=codex",
		"adapter.OutputStream runtime=codex round=/round",
		"adapter.Launch runtime=codex verb=dispatch job=j1",
		"adapter.Cancel runtime=codex job=j1",
		"adapter.ResultPatch output=/out failure=none phase=done",
		"goal.LedgerIdentity",
		"goal.Binding goal=g1",
		"records.Create job=j1",
		"records.Setup job=j1",
		"records.CAS job=j1 a->b",
		"records.Read job=j1",
		"host.WaitJob job=j1",
		"host.WatchJob job=j1 progress=/progress",
		"guard.Acquire owner=suite",
		"guard.Release pid=7",
	}
	if got := set.Log.Calls(); !reflect.DeepEqual(got, want) {
		t.Fatalf("log =\n%q\nwant\n%q", got, want)
	}
	if !reflect.DeepEqual(set.Guard.Released, []int64{7}) {
		t.Fatalf("released = %v", set.Guard.Released)
	}
	// Calls is a copy: appending to it never reaches the log.
	calls := set.Log.Calls()
	_ = append(calls[:1], "forged")
	if set.Log.Calls()[1] != want[1] {
		t.Fatal("mutating a Calls copy rewrote the log")
	}
}

func TestUnsetRefusingDoublesRefuse(t *testing.T) {
	t.Parallel()
	set := NewSet()
	ctx := context.Background()
	if _, err := set.Goal.BreachStop("g1", 3, time.Time{}, "wido"); err == nil {
		t.Fatal("an unscripted breach stop succeeded")
	}
	if _, err := set.Host.BreachStopOrderingHuman(ctx, "/root", 7, time.Time{}); err == nil {
		t.Fatal("an unscripted ordering human was proven")
	}
	request := delegation.ExtendBudgetRequest{GoalID: "g1"}
	if out, code := set.Host.ExtendBudget(ctx, request); out != "" || code != 1 {
		t.Fatalf("unscripted ExtendBudget = %q, %d; want a refusal", out, code)
	}
	if !reflect.DeepEqual(set.Host.ExtendCalled, []delegation.ExtendBudgetRequest{request}) {
		t.Fatalf("ExtendCalled = %+v", set.Host.ExtendCalled)
	}
	want := []string{
		"goal.BreachStop goal=g1 revision=3 orderedBy=wido",
		"host.BreachStopOrderingHuman caller=7",
		"host.ExtendBudget goal=g1",
	}
	if got := set.Log.Calls(); !reflect.DeepEqual(got, want) {
		t.Fatalf("log = %q, want %q", got, want)
	}
}

func TestScriptedDoublesAnswerFromTheirFunctions(t *testing.T) {
	t.Parallel()
	set := NewSet()
	ctx := context.Background()
	inv := delegation.Invocation{CallerPid: 9}
	refusal := errors.New("scripted refusal")
	set.Lease.ClassifyFunc = func(got delegation.Invocation) (lease.ClassifyResult, error) {
		return lease.ClassifyResult{Class: "MAIN", Holder: got.CallerPid == 9}, nil
	}
	set.Lease.RequireFunc = func(delegation.Invocation, *int64) (lease.HolderView, error) { return lease.HolderView{}, refusal }
	set.Lease.RenewFunc = func(delegation.Invocation) (lease.RenewResult, error) { return lease.RenewResult{Revision: 4}, nil }
	set.Lease.AuthorizeFunc = func(delegation.Invocation, delegation.AuthorityMode, string) error { return refusal }
	set.Steward.AuthorizeFunc = func(_ delegation.Invocation, intent string) (steward.DispatchAuthorization, error) {
		return steward.DispatchAuthorization{Goal: intent}, nil
	}
	set.Adapter.NotInstalled = map[string]bool{"devin": true}
	set.Adapter.ConfigIdentityFunc = func(runtime string) (string, error) { return "id-" + runtime, nil }
	set.Adapter.ProbeFunc = func(string) error { return refusal }
	set.Adapter.OutputStreamFunc = func(_, round string) (string, error) { return round + "/stream", nil }
	set.Adapter.LaunchFunc = func(request delegation.AdapterLaunch) (int64, error) { return 4321, nil }
	set.Adapter.CancelFunc = func(string, string) error { return refusal }
	set.Adapter.ResultPatchFunc = func(string, string, string, string) error { return refusal }
	set.Goal.Identity = func() string { return "ledger-1" }
	set.Goal.BindingFunc = func(id string) (delegation.GoalBinding, error) {
		return delegation.GoalBinding{GoalID: id, Revision: 2}, nil
	}
	set.Goal.BreachStopFunc = func(id string, revision uint64, _ time.Time, _ string) (goal.StopBatch, error) {
		return goal.StopBatch{GoalID: id, GoalRevision: revision}, nil
	}
	set.Records.CreateFunc = func(string, string) error { return refusal }
	set.Records.SetupFunc = func(string, string) error { return refusal }
	set.Records.CASFunc = func(string, string, string, string) (string, error) { return "observed", refusal }
	set.Records.ReadFunc = func(job string) (map[string]any, error) { return map[string]any{"jobId": job}, nil }
	set.Host.WaitFunc = func(string, string, int64) delegation.WaitOutcome {
		return delegation.WaitOutcome{Code: 3, Output: "late"}
	}
	set.Host.WatchFunc = func(string, string, int64, string) int { return 5 }
	set.Host.ExtendFunc = func(delegation.ExtendBudgetRequest) (string, int) { return "extended", 0 }
	set.Host.OrderingHumanFunc = func(string, int64, time.Time) (string, error) { return "wido", nil }
	set.Guard.AcquireFunc = func(string, int64, string) (gaterun.GuardResult, error) { return gaterun.GuardResult(1), refusal }

	if got, _ := set.Lease.Classify(inv); got.Class != "MAIN" || !got.Holder {
		t.Fatalf("Classify = %+v", got)
	}
	if _, err := set.Lease.RequireHolder(inv, nil); !errors.Is(err, refusal) {
		t.Fatalf("RequireHolder err = %v", err)
	}
	if got, _ := set.Lease.Renew(inv); got.Revision != 4 {
		t.Fatalf("Renew = %+v", got)
	}
	if err := set.Lease.Authorize(inv, delegation.AuthorityHolderOnly, "j1"); !errors.Is(err, refusal) {
		t.Fatalf("Authorize err = %v", err)
	}
	if got, _ := set.Steward.AuthorizeDispatch(inv, "g7"); got.Goal != "g7" {
		t.Fatalf("AuthorizeDispatch = %+v", got)
	}
	if set.Adapter.Installed("devin") || !set.Adapter.Installed("codex") {
		t.Fatal("NotInstalled was not honoured")
	}
	if got, _ := set.Adapter.ConfigIdentity(ctx, "codex"); got != "id-codex" {
		t.Fatalf("ConfigIdentity = %q", got)
	}
	if err := set.Adapter.Probe(ctx, "codex"); !errors.Is(err, refusal) {
		t.Fatalf("Probe err = %v", err)
	}
	if got, _ := set.Adapter.OutputStream(ctx, "codex", "/r"); got != "/r/stream" {
		t.Fatalf("OutputStream = %q", got)
	}
	if pid, _ := set.Adapter.Launch(ctx, delegation.AdapterLaunch{}); pid != 4321 {
		t.Fatalf("Launch = %d", pid)
	}
	if err := set.Adapter.Cancel(ctx, "codex", "j1"); !errors.Is(err, refusal) {
		t.Fatalf("Cancel err = %v", err)
	}
	if err := set.Adapter.ResultPatch("", "", "", ""); !errors.Is(err, refusal) {
		t.Fatalf("ResultPatch err = %v", err)
	}
	if got := set.Goal.LedgerIdentity(); got != "ledger-1" {
		t.Fatalf("LedgerIdentity = %q", got)
	}
	if got, _ := set.Goal.Binding("g2"); got.GoalID != "g2" || got.Revision != 2 {
		t.Fatalf("Binding = %+v", got)
	}
	if got, err := set.Goal.BreachStop("g2", 2, time.Time{}, "wido"); err != nil || got.GoalID != "g2" || got.GoalRevision != 2 {
		t.Fatalf("BreachStop = %+v, %v", got, err)
	}
	if err := set.Records.Create("j1", ""); !errors.Is(err, refusal) {
		t.Fatalf("Create err = %v", err)
	}
	if err := set.Records.Setup("j1", ""); !errors.Is(err, refusal) {
		t.Fatalf("Setup err = %v", err)
	}
	if got, err := set.Records.CAS("j1", "a", "b", ""); got != "observed" || !errors.Is(err, refusal) {
		t.Fatalf("CAS = %q, %v", got, err)
	}
	if got, _ := set.Records.Read("j1"); got["jobId"] != "j1" {
		t.Fatalf("Read = %v", got)
	}
	if got := set.Host.WaitJob(ctx, "", "j1", 9); got.Code != 3 || got.Output != "late" {
		t.Fatalf("WaitJob = %+v", got)
	}
	if got := set.Host.WatchJob(ctx, "", "j1", 9, ""); got != 5 {
		t.Fatalf("WatchJob = %d", got)
	}
	if out, code := set.Host.ExtendBudget(ctx, delegation.ExtendBudgetRequest{}); out != "extended" || code != 0 {
		t.Fatalf("ExtendBudget = %q, %d", out, code)
	}
	if who, err := set.Host.BreachStopOrderingHuman(ctx, "", 9, time.Time{}); who != "wido" || err != nil {
		t.Fatalf("BreachStopOrderingHuman = %q, %v", who, err)
	}
	if got, err := set.Guard.Acquire("", 9, "suite", 0, 0, nil); got != gaterun.GuardResult(1) || !errors.Is(err, refusal) {
		t.Fatalf("Acquire = %v, %v", got, err)
	}
}

func TestLeaseHeldRunsTheBodyUnlessRefused(t *testing.T) {
	t.Parallel()
	bodyErr := errors.New("body failed")
	ran := 0
	body := func() error { ran++; return bodyErr }
	held := &Lease{Log: &Log{}}
	if err := held.Held(delegation.Invocation{CallerPid: 1}, nil, body); !errors.Is(err, bodyErr) || ran != 1 {
		t.Fatalf("Held = %v after %d runs; want the body's error after one run", err, ran)
	}
	refusal := errors.New("not the holder")
	held.HeldRefusal = refusal
	if err := held.Held(delegation.Invocation{CallerPid: 1}, nil, body); !errors.Is(err, refusal) || ran != 1 {
		t.Fatalf("refused Held = %v after %d runs; want the refusal and no run", err, ran)
	}
	if got := held.Log.Calls(); !reflect.DeepEqual(got, []string{"lease.Held pid=1 epoch=none", "lease.Held pid=1 epoch=none"}) {
		t.Fatalf("log = %q", got)
	}
}

func TestADoubleWithoutALogStillAnswers(t *testing.T) {
	t.Parallel()
	var log *Log
	log.add("ignored %d", 1)
	if log.Calls() != nil {
		t.Fatal("a nil log returned calls")
	}
	records := &Records{}
	if err := records.Create("j1", ""); err != nil {
		t.Fatal(err)
	}
}

func TestProcessTableAnswersFromItsScript(t *testing.T) {
	t.Parallel()
	process := NewSet().Process
	process.Tags[10] = "tag-a"
	process.Unknown = map[int64]bool{11: true}
	process.Starts = map[int64]int64{10: 1_700_000_000}
	for _, tc := range []struct {
		pid  int64
		tag  string
		want string
	}{{10, "tag-a", "live"}, {10, "tag-b", "stale"}, {10, "", "stale"}, {11, "tag-a", "unknown"}, {12, "tag-a", "dead"}} {
		if got := process.TagState(tc.pid, tc.tag); got != tc.want {
			t.Errorf("TagState(%d, %q) = %q, want %q", tc.pid, tc.tag, got, tc.want)
		}
	}
	if !process.Exists(10) || process.Exists(12) {
		t.Fatal("Exists does not follow the tag table")
	}
	if started, err := process.StartedAt(10); err != nil || started != 1_700_000_000 {
		t.Fatalf("StartedAt(10) = %d, %v", started, err)
	}
	if _, err := process.StartedAt(12); err == nil {
		t.Fatal("a dead pid reported a start")
	}

	process.Groups[20], process.Groups[21], process.Groups[22] = true, true, true
	process.Owned[20] = true
	process.Survivors = map[int64]bool{21: true}
	process.Immortal = map[int64]bool{22: true}
	if !process.GroupOwned("", 20, "") || process.GroupOwned("", 21, "") {
		t.Fatal("GroupOwned does not follow the owned table")
	}
	for _, pgid := range []int64{20, 21, 22} {
		if err := process.SignalGroup(pgid, delegation.SignalTerm); err != nil {
			t.Fatal(err)
		}
	}
	if process.GroupExists(20) || !process.GroupExists(21) || !process.GroupExists(22) {
		t.Fatal("TERM must end an ordinary group and spare a survivor and an immortal one")
	}
	for _, pgid := range []int64{21, 22} {
		if err := process.SignalGroup(pgid, delegation.SignalKill); err != nil {
			t.Fatal(err)
		}
	}
	if process.GroupExists(21) || !process.GroupExists(22) {
		t.Fatal("KILL must end a survivor and spare only an immortal group")
	}
	if want := []string{"20:15", "21:15", "22:15", "21:9", "22:9"}; !reflect.DeepEqual(process.Signals, want) {
		t.Fatalf("signals = %v, want %v", process.Signals, want)
	}
}

func TestProcessClaimAndCustodyOverrides(t *testing.T) {
	t.Parallel()
	process := NewSet().Process
	refusal := errors.New("no table")
	process.ClaimProcessesFunc = func() (delegation.ClaimProcesses, error) { return delegation.ClaimProcesses{}, refusal }
	if _, err := process.ClaimProcesses(); !errors.Is(err, refusal) {
		t.Fatalf("ClaimProcesses err = %v, want the scripted one", err)
	}
	process.CustodyTargets = func(record map[string]any) ([]int64, error) { return []int64{99}, nil }
	if got, err := process.CustodyGroups(map[string]any{}); err != nil || !reflect.DeepEqual(got, []int64{99}) {
		t.Fatalf("CustodyGroups = %v, %v", got, err)
	}
	if got := process.Log.Calls(); !reflect.DeepEqual(got, []string{"process.ClaimProcesses"}) {
		t.Fatalf("log = %q", got)
	}
}

func TestProcessClaimReadsTheRealTableWithoutAScript(t *testing.T) {
	t.Setenv("METASYSTEM_CENSUS_PROCESS_FILE", "")
	t.Setenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", "")
	process := &Process{Root: t.TempDir()}
	claim, err := process.ClaimProcesses()
	if err != nil {
		t.Fatal(err)
	}
	if claim.Reader == nil || claim.Scanner == nil || claim.Verifier == nil {
		t.Fatalf("real claim processes are incomplete: %+v", claim)
	}
}

func TestGitAnswersOnlyScriptedCalls(t *testing.T) {
	t.Parallel()
	set := NewSet()
	ctx := context.Background()
	effects := 0
	set.Git.Responses["*|status"] = GitResponse{Stdout: "any dir"}
	set.Git.Responses["/repo|status"] = GitResponse{Stdout: "this dir", Effect: func() { effects++ }}
	set.Git.Responses["*|merge"] = GitResponse{Stdout: "partial", Stderr: "conflict", Code: 1}

	if out, _, err := set.Git.Run(ctx, "/repo", "status"); err != nil || string(out) != "this dir" || effects != 1 {
		t.Fatalf("dir-scripted status = %q, %v (effects %d)", out, err, effects)
	}
	if out, _, err := set.Git.Run(ctx, "/other", "status"); err != nil || string(out) != "any dir" {
		t.Fatalf("wildcard status = %q, %v", out, err)
	}
	out, stderr, err := set.Git.Run(ctx, "/repo", "merge")
	var exit *delegation.GitExitError
	if !errors.As(err, &exit) || exit.Code != 1 || exit.Stderr != "conflict" || string(out) != "partial" || string(stderr) != "conflict" {
		t.Fatalf("failing merge = %q %q %v", out, stderr, err)
	}
	if _, _, err := set.Git.Run(ctx, "/repo", "push"); !errors.As(err, &exit) || exit.Code != 128 {
		t.Fatalf("unscripted call = %v, want exit 128", err)
	}
	if want := []string{"/repo|status", "/other|status", "/repo|merge", "/repo|push"}; !reflect.DeepEqual(set.Git.Calls, want) {
		t.Fatalf("git calls = %v, want %v", set.Git.Calls, want)
	}
}

func TestClockAdvancesOnlyOnSleep(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	var seen []time.Time
	clock := &Clock{Current: start, OnSleep: func(now time.Time) { seen = append(seen, now) }}
	if !clock.Now().Equal(start) {
		t.Fatalf("Now = %v", clock.Now())
	}
	clock.Sleep(time.Minute)
	clock.Sleep(2 * time.Second)
	want := start.Add(time.Minute + 2*time.Second)
	if !clock.Now().Equal(want) {
		t.Fatalf("Now after sleeps = %v, want %v", clock.Now(), want)
	}
	if len(seen) != 2 || !seen[0].Equal(start.Add(time.Minute)) || !seen[1].Equal(want) {
		t.Fatalf("OnSleep saw %v", seen)
	}
	if !NewSet().Clock.Now().Equal(time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)) {
		t.Fatal("the set's clock does not start at its fixed instant")
	}
}

func TestEventsKeepACopyOfTheirFields(t *testing.T) {
	t.Parallel()
	events := &Events{Log: &Log{}}
	fields := map[string]string{"jobId": "j1"}
	events.Emit("job-verdict", "done", fields)
	fields["jobId"] = "rewritten"
	if len(events.Emitted) != 1 || events.Emitted[0].Name != "job-verdict" || events.Emitted[0].Summary != "done" || events.Emitted[0].Fields["jobId"] != "j1" {
		t.Fatalf("emitted = %+v", events.Emitted)
	}
	if got := events.Log.Calls(); !reflect.DeepEqual(got, []string{"events.Emit event=job-verdict"}) {
		t.Fatalf("log = %q", got)
	}
}

func TestSetPortsComposeALifecycle(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	lifecycle, err := delegation.New(delegation.Config{Root: root, RepoScope: root}, NewSet().Ports())
	if err != nil || lifecycle == nil {
		t.Fatalf("the fake set is not a complete port set: %v", err)
	}
}
