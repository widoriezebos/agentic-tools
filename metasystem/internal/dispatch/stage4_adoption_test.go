package dispatch

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

type fixedAdoptionScanner struct{ result census.TaggedProcessCensus }

func (s fixedAdoptionScanner) ScanTag(string, time.Time) census.TaggedProcessCensus { return s.result }

type processTableAdoptionScanner struct{ table *stage4ProcessTable }

func (s processTableAdoptionScanner) ScanTag(tag string, reservationCreatedAt time.Time) census.TaggedProcessCensus {
	return census.ScanTaggedProcesses(tag, census.TaggedScanDependencies{
		PIDs: s.table.PIDs, Signal: func(int64) error { return nil }, Reader: s.table,
		MatchesTag:           func(argv []string, wanted string) bool { return false },
		ReservationCreatedAt: reservationCreatedAt,
	})
}

func adoptionExactAt(pid int64, startedAt time.Time) identity.Exact {
	exact := stage4Exact(pid, pid)
	exact.StartedAt = startedAt
	return exact
}

func adoptionCandidate(pid, pgid, micro int64) census.TaggedProcess {
	return census.TaggedProcess{PID: pid, PGID: pgid, Identity: stage4Exact(pid, micro)}
}

func foreignAdoptionUnknowns(count int) []census.IndeterminateProcess {
	unknowns := make([]census.IndeterminateProcess, 0, count)
	for index := 0; index < count; index++ {
		unknowns = append(unknowns, census.IndeterminateProcess{
			PID: int64(index + 1), Reason: "signal-zero-permission-denied", Universe: census.ProcessUniverseForeign,
		})
	}
	return unknowns
}

func createReconciliationReservation(t *testing.T, root, opid string, creator identity.Exact) (LaunchFingerprintRequest, string) {
	t.Helper()
	now := time.Unix(1_786_000_000, 0).UTC()
	params := claimParamsForTest(root, opid)
	deps := claimDependenciesForTest(&now, identity.Verification{})
	deps.CreatorPID = creator.Pid
	deps.IdentityReader = &stage4ProcessTable{
		starts:      map[int64]identity.Exact{creator.Pid: creator},
		startStates: map[int64]identity.Liveness{creator.Pid: identity.Alive},
	}
	result, err := ClaimLaunch(params, deps)
	if err != nil || result.Outcome != ClaimWON {
		t.Fatalf("create reservation: %+v %v", result, err)
	}
	return params.Request, filepath.Join(root, "artifacts", "agents", "jobs", opid+".json")
}

func TestZeroLeaderAdoptionChoosesEldestAndMarksLeaderless(t *testing.T) {
	root := t.TempDir()
	creator := stage4Exact(80, 600_000_001)
	request, path := createReconciliationReservation(t, root, "zero-leader", creator)
	record := readRecord(t, root, "zero-leader")
	tag := asString(record["instanceTag"])

	result, err := ReconcileReservation(root, "zero-leader", ReconciliationDependencies{
		Now: func() time.Time { return time.Unix(1_786_000_100, 0).UTC() },
		Scanner: fixedAdoptionScanner{result: census.TaggedProcessCensus{Tagged: []census.TaggedProcess{
			adoptionCandidate(82, 91, 602_000_001),
			adoptionCandidate(81, 90, 601_000_001),
		}}},
		Creator: &stage4ProcessTable{
			starts:      map[int64]identity.Exact{80: creator},
			startStates: map[int64]identity.Liveness{80: identity.Alive},
		},
	})
	if err != nil || result.Outcome != ReconciliationAdopted || !result.Leaderless {
		t.Fatalf("zero-leader reconciliation = %+v err=%v", result, err)
	}
	got, _ := readObject(path)
	if asString(got["status"]) != "pending" || !looseEqual(got["pid"], int64(81)) || got["leaderless"] != true {
		t.Fatalf("leaderless adoption record = %+v", got)
	}
	if items, _ := got["custodyProcesses"].([]any); len(items) != 1 {
		t.Fatalf("other tagged survivors were not custody-added: %+v", got["custodyProcesses"])
	} else if entry, _ := items[0].(map[string]any); !looseEqual(entry["pgid"], int64(91)) {
		t.Fatalf("cross-group tagged survivor lost its group: %+v", entry)
	}
	if asString(got["instanceTag"]) != tag {
		t.Fatalf("adoption changed reservation generation: %v", got["instanceTag"])
	}
	_ = request
}

func TestExactlyOneLeaderAdoptsLeaderAndCustodiesEveryOtherTag(t *testing.T) {
	root := t.TempDir()
	creator := stage4Exact(83, 625_000_001)
	createReconciliationReservation(t, root, "one-leader", creator)
	result, err := ReconcileReservation(root, "one-leader", ReconciliationDependencies{
		Scanner: fixedAdoptionScanner{result: census.TaggedProcessCensus{Tagged: []census.TaggedProcess{
			adoptionCandidate(84, 92, 627_000_001),
			adoptionCandidate(82, 82, 626_000_001),
		}}},
		Creator: &stage4ProcessTable{},
	})
	if err != nil || result.Outcome != ReconciliationAdopted || result.PrimaryPID != 82 || result.Leaderless {
		t.Fatalf("single-leader reconciliation = %+v err=%v", result, err)
	}
	record := readRecord(t, root, "one-leader")
	items, _ := record["custodyProcesses"].([]any)
	if !looseEqual(record["pgid"], int64(82)) || len(items) != 1 {
		t.Fatalf("single-leader adoption record = %+v", record)
	}
	entry, _ := items[0].(map[string]any)
	if !looseEqual(entry["pid"], int64(84)) || !looseEqual(entry["pgid"], int64(92)) {
		t.Fatalf("other nonce-tagged process was not custody-added across groups: %+v", entry)
	}
}

func TestUnknownObservationDefersEvenWhenCreatorIsDead(t *testing.T) {
	root := t.TempDir()
	creator := stage4Exact(85, 650_000_001)
	createReconciliationReservation(t, root, "unknown-observation", creator)
	var events []string
	result, err := ReconcileReservation(root, "unknown-observation", ReconciliationDependencies{
		Scanner: fixedAdoptionScanner{result: census.TaggedProcessCensus{
			Indeterminate: []census.IndeterminateProcess{{PID: 86, PGID: 86, Reason: "identity-INDETERMINATE"}},
		}},
		Creator: &stage4ProcessTable{startStates: map[int64]identity.Liveness{85: identity.Dead}},
		Emit:    func(line string) { events = append(events, line) },
	})
	if err != nil || result.Outcome != ReconciliationDeferred || result.UnknownCount != 1 {
		t.Fatalf("unknown observation reconciliation = %+v err=%v", result, err)
	}
	if len(events) != 1 || !strings.Contains(events[0], "REAP-DEFERRED") {
		t.Fatalf("unknown observation event = %v", events)
	}
	if got := readRecord(t, root, "unknown-observation"); asString(got["status"]) != "pending-setup" {
		t.Fatalf("unknown observation became absence: %+v", got)
	}
}

func TestElevenOldSameUserUnknownsReconcileCreatorAbandonment(t *testing.T) {
	root := t.TempDir()
	creator := stage4Exact(188, 653_000_001)
	createReconciliationReservation(t, root, "old-daemons", creator)
	record := readRecord(t, root, "old-daemons")
	createdAt, err := parseRecordTime(asString(record["createdAt"]))
	if err != nil {
		t.Fatalf("reservation createdAt: %v", err)
	}
	table := &stage4ProcessTable{
		starts:      map[int64]identity.Exact{},
		startStates: map[int64]identity.Liveness{},
		argvKnown:   map[int64]bool{},
	}
	for index := 0; index < 11; index++ {
		pid := int64(200 + index)
		table.pids = append(table.pids, pid)
		table.starts[pid] = adoptionExactAt(pid, createdAt.Add(-census.ReservationStartTimeSlack-time.Second))
		table.startStates[pid] = identity.Alive
		table.argvKnown[pid] = false
	}
	result, err := ReconcileReservation(root, "old-daemons", ReconciliationDependencies{
		Scanner: processTableAdoptionScanner{table: table},
		Creator: &stage4ProcessTable{startStates: map[int64]identity.Liveness{188: identity.Dead}},
	})
	if err != nil || result.Outcome != ReconciliationCreatorAbandoned || result.UnknownCount != 0 {
		t.Fatalf("old-daemon reconciliation = %+v err=%v", result, err)
	}
	record = readRecord(t, root, "old-daemons")
	if asString(record["error"]) != "creator-abandoned" {
		t.Fatalf("old daemons blocked creator abandonment: %+v", record)
	}
	reconciliation, _ := record["reconciliation"].(map[string]any)
	observations, _ := reconciliation["unknownProcesses"].([]any)
	if len(observations) != 11 {
		t.Fatalf("old-daemon evidence count = %d, want 11: %+v", len(observations), reconciliation)
	}
	for _, item := range observations {
		observation, _ := item.(map[string]any)
		if asString(observation["universe"]) != "excluded-by-age" {
			t.Fatalf("old-daemon evidence lost a classification: %+v", reconciliation)
		}
	}
}

func TestPostReservationUnknownDefersAndReportsAgeExclusion(t *testing.T) {
	root := t.TempDir()
	creator := stage4Exact(189, 654_000_001)
	createReconciliationReservation(t, root, "new-unknown", creator)
	record := readRecord(t, root, "new-unknown")
	createdAt, err := parseRecordTime(asString(record["createdAt"]))
	if err != nil {
		t.Fatalf("reservation createdAt: %v", err)
	}
	table := &stage4ProcessTable{
		starts: map[int64]identity.Exact{
			190: adoptionExactAt(190, createdAt.Add(-census.ReservationStartTimeSlack-time.Second)),
			191: adoptionExactAt(191, createdAt.Add(time.Second)),
		},
		startStates: map[int64]identity.Liveness{190: identity.Alive, 191: identity.Alive},
		argvKnown:   map[int64]bool{190: false, 191: false},
		pids:        []int64{190, 191},
	}
	var events []string
	result, err := ReconcileReservation(root, "new-unknown", ReconciliationDependencies{
		Scanner: processTableAdoptionScanner{table: table},
		Creator: &stage4ProcessTable{startStates: map[int64]identity.Liveness{189: identity.Dead}},
		Emit:    func(line string) { events = append(events, line) },
	})
	if err != nil || result.Outcome != ReconciliationDeferred || result.UnknownCount != 1 {
		t.Fatalf("post-reservation unknown reconciliation = %+v err=%v", result, err)
	}
	if len(events) != 1 || !strings.Contains(events[0], "unknown=1 foreign=0 excludedByAge=1") {
		t.Fatalf("age-exclusion deferral event = %v", events)
	}
	record = readRecord(t, root, "new-unknown")
	reconciliation, _ := record["reconciliation"].(map[string]any)
	observations, _ := reconciliation["unknownProcesses"].([]any)
	if len(observations) != 2 {
		t.Fatalf("age classification dropped an observation: %+v", reconciliation)
	}
	if first, _ := observations[0].(map[string]any); asString(first["universe"]) != "excluded-by-age" {
		t.Fatalf("old observation classification = %+v", first)
	}
	if second, _ := observations[1].(map[string]any); asString(second["universe"]) != "signalable" {
		t.Fatalf("new observation classification = %+v", second)
	}
}

func TestUnreadableStartSameUserStillDefers(t *testing.T) {
	root := t.TempDir()
	creator := stage4Exact(192, 655_000_001)
	createReconciliationReservation(t, root, "unreadable-start", creator)
	table := &stage4ProcessTable{
		startStates: map[int64]identity.Liveness{193: identity.Unknown},
		pids:        []int64{193},
	}
	var events []string
	result, err := ReconcileReservation(root, "unreadable-start", ReconciliationDependencies{
		Scanner: processTableAdoptionScanner{table: table},
		Creator: &stage4ProcessTable{startStates: map[int64]identity.Liveness{192: identity.Dead}},
		Emit:    func(line string) { events = append(events, line) },
	})
	if err != nil || result.Outcome != ReconciliationDeferred || result.UnknownCount != 1 {
		t.Fatalf("unreadable-start reconciliation = %+v err=%v", result, err)
	}
	if len(events) != 1 || !strings.Contains(events[0], "unknown=1 foreign=0 excludedByAge=0") {
		t.Fatalf("unreadable-start deferral event = %v", events)
	}
	record := readRecord(t, root, "unreadable-start")
	reconciliation, _ := record["reconciliation"].(map[string]any)
	observations, _ := reconciliation["unknownProcesses"].([]any)
	if len(observations) != 1 {
		t.Fatalf("unreadable-start evidence = %+v", reconciliation)
	}
	observation, _ := observations[0].(map[string]any)
	if asString(observation["universe"]) != "signalable" {
		t.Fatalf("unreadable start was age-excluded: %+v", observation)
	}
}

func TestForeignUnknownsDoNotBlockCreatorAbandonment(t *testing.T) {
	root := t.TempDir()
	creator := stage4Exact(186, 651_000_001)
	createReconciliationReservation(t, root, "foreign-observation", creator)
	var events []string
	result, err := ReconcileReservation(root, "foreign-observation", ReconciliationDependencies{
		Scanner: fixedAdoptionScanner{result: census.TaggedProcessCensus{
			Indeterminate: foreignAdoptionUnknowns(1),
		}},
		Creator: &stage4ProcessTable{startStates: map[int64]identity.Liveness{186: identity.Dead}},
		Emit:    func(line string) { events = append(events, line) },
	})
	if err != nil || result.Outcome != ReconciliationCreatorAbandoned || result.UnknownCount != 0 {
		t.Fatalf("foreign observation reconciliation = %+v err=%v", result, err)
	}
	if len(events) != 0 {
		t.Fatalf("creator abandonment emitted a deferral: %v", events)
	}
	record := readRecord(t, root, "foreign-observation")
	if asString(record["status"]) != "failed" || asString(record["error"]) != "creator-abandoned" {
		t.Fatalf("foreign observation blocked creator abandonment: %+v", record)
	}
	reconciliation, _ := record["reconciliation"].(map[string]any)
	unknowns, _ := reconciliation["unknownProcesses"].([]any)
	if len(unknowns) != 1 {
		t.Fatalf("reconciliation evidence lost the foreign observation: %+v", reconciliation)
	}
	unknown, _ := unknowns[0].(map[string]any)
	if asString(unknown["universe"]) != "foreign" {
		t.Fatalf("reconciliation evidence lost the foreign classification: %+v", reconciliation)
	}
}

func TestForeignUnknownDeferralReportsZeroUnknownsWithinUniverse(t *testing.T) {
	root := t.TempDir()
	creator := stage4Exact(187, 652_000_001)
	createReconciliationReservation(t, root, "foreign-creator-alive", creator)
	var events []string
	result, err := ReconcileReservation(root, "foreign-creator-alive", ReconciliationDependencies{
		Scanner: fixedAdoptionScanner{result: census.TaggedProcessCensus{
			Indeterminate: foreignAdoptionUnknowns(302),
		}},
		Creator: &stage4ProcessTable{
			starts:      map[int64]identity.Exact{187: creator},
			startStates: map[int64]identity.Liveness{187: identity.Alive},
		},
		Emit: func(line string) { events = append(events, line) },
	})
	if err != nil || result.Outcome != ReconciliationDeferred || result.UnknownCount != 0 {
		t.Fatalf("foreign-only deferral = %+v err=%v", result, err)
	}
	if len(events) != 1 || !strings.Contains(events[0], "unknown=0 foreign=302 excludedByAge=0") {
		t.Fatalf("foreign-only deferral event = %v", events)
	}
}

func TestCompleteAbsenceDefersWhileCreatorIsAlive(t *testing.T) {
	root := t.TempDir()
	creator := stage4Exact(87, 675_000_001)
	createReconciliationReservation(t, root, "creator-still-launching", creator)
	var events []string
	result, err := ReconcileReservation(root, "creator-still-launching", ReconciliationDependencies{
		Scanner: fixedAdoptionScanner{result: census.TaggedProcessCensus{}},
		Creator: &stage4ProcessTable{
			starts:      map[int64]identity.Exact{87: creator},
			startStates: map[int64]identity.Liveness{87: identity.Alive},
		},
		Emit: func(line string) { events = append(events, line) },
	})
	if err != nil || result.Outcome != ReconciliationDeferred || result.Reason != "complete-census-absence-creator-alive" {
		t.Fatalf("live creator absence = %+v err=%v", result, err)
	}
	if len(events) != 1 || !strings.Contains(events[0], "REAP-DEFERRED") {
		t.Fatalf("live creator deferral event = %v", events)
	}
	if got := readRecord(t, root, "creator-still-launching"); asString(got["status"]) != "pending-setup" {
		t.Fatalf("a live creator's reservation was terminal-stamped: %+v", got)
	}
}

func TestMultipleLeadersDeferAndEmitReapDeferred(t *testing.T) {
	root := t.TempDir()
	creator := stage4Exact(90, 700_000_001)
	createReconciliationReservation(t, root, "multi-leader", creator)
	var events []string
	result, err := ReconcileReservation(root, "multi-leader", ReconciliationDependencies{
		Scanner: fixedAdoptionScanner{result: census.TaggedProcessCensus{Tagged: []census.TaggedProcess{
			adoptionCandidate(91, 91, 701_000_001),
			adoptionCandidate(92, 92, 702_000_001),
		}}},
		Creator: &stage4ProcessTable{},
		Emit:    func(line string) { events = append(events, line) },
	})
	if err != nil || result.Outcome != ReconciliationDeferred || result.Reason != "multiple-tagged-leaders" {
		t.Fatalf("multi-leader reconciliation = %+v err=%v", result, err)
	}
	if len(events) != 1 || !strings.Contains(events[0], "REAP-DEFERRED") {
		t.Fatalf("deferral event = %v", events)
	}
	if got := readRecord(t, root, "multi-leader"); asString(got["status"]) != "pending-setup" {
		t.Fatalf("deferred reservation advanced: %+v", got)
	}
}

func TestClaimReconcilingCompletesThroughAdoption(t *testing.T) {
	root := t.TempDir()
	now := time.Unix(1_786_000_000, 0).UTC()
	params := claimParamsForTest(root, "reconcile-adopt")
	deps := claimDependenciesForTest(&now, identity.Verification{})
	deps.Reconcile = func(root, job string) (ReconciliationResult, error) {
		return ReconcileReservation(root, job, ReconciliationDependencies{
			Now: func() time.Time { return now },
			Scanner: fixedAdoptionScanner{result: census.TaggedProcessCensus{Tagged: []census.TaggedProcess{
				adoptionCandidate(101, 101, 801_000_001),
			}}},
			Creator: deps.IdentityReader,
		})
	}
	if first, err := ClaimLaunch(params, deps); err != nil || first.Outcome != ClaimWON {
		t.Fatalf("claim won = %+v err=%v", first, err)
	}
	params.Wait = true
	result, err := ClaimLaunch(params, deps)
	if err != nil || result.Outcome != ClaimBound {
		t.Fatalf("reconciling adoption did not bind: %+v err=%v", result, err)
	}
}

func TestClaimReconcilingCompletesThroughCreatorAbandonment(t *testing.T) {
	root := t.TempDir()
	now := time.Unix(1_786_000_000, 0).UTC()
	params := claimParamsForTest(root, "reconcile-abandoned")
	deps := claimDependenciesForTest(&now, identity.Verification{})
	if first, err := ClaimLaunch(params, deps); err != nil || first.Outcome != ClaimWON {
		t.Fatalf("claim won = %+v err=%v", first, err)
	}
	deadCreator := &stage4ProcessTable{startStates: map[int64]identity.Liveness{deps.CreatorPID: identity.Dead}}
	deps.Reconcile = func(root, job string) (ReconciliationResult, error) {
		return ReconcileReservation(root, job, ReconciliationDependencies{
			Now:     func() time.Time { return now.Add(time.Minute) },
			Scanner: fixedAdoptionScanner{result: census.TaggedProcessCensus{}},
			Creator: deadCreator,
		})
	}
	params.Wait = true
	result, err := ClaimLaunch(params, deps)
	if err != nil || result.Outcome != ClaimOutcome("REPLAYED-failed") {
		t.Fatalf("creator abandonment did not finish reconciliation: %+v err=%v", result, err)
	}
	if got := readRecord(t, root, "reconcile-abandoned"); asString(got["error"]) != "creator-abandoned" {
		t.Fatalf("creator abandonment record = %+v", got)
	}
}
