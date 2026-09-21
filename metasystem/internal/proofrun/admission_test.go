package proofrun

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostload"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

type admissionCensusProcess struct {
	exact  identity.Exact
	parent int64
}

type admissionCensus struct {
	mu        sync.RWMutex
	processes map[int64]admissionCensusProcess
}

func (c *admissionCensus) replace(processes ...admissionCensusProcess) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.processes = make(map[int64]admissionCensusProcess, len(processes))
	for _, process := range processes {
		c.processes[process.exact.Pid] = process
	}
}

func (c *admissionCensus) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	process, ok := c.processes[pid]
	if !ok {
		return identity.Exact{}, identity.Dead, nil
	}
	return process.exact, identity.Alive, nil
}

func (c *admissionCensus) pids() ([]int64, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	pids := make([]int64, 0, len(c.processes))
	for pid := range c.processes {
		pids = append(pids, pid)
	}
	sort.Slice(pids, func(i, j int) bool { return pids[i] < pids[j] })
	return pids, nil
}

func (c *admissionCensus) parent(pid int64) (int64, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	process, ok := c.processes[pid]
	return process.parent, ok
}

func admissionProcess(pid, parent int64, started time.Time, argv ...string) admissionCensusProcess {
	return admissionCensusProcess{exact: identity.Exact{Pid: pid, StartedAt: started, Argv: argv, ArgvKnown: true}, parent: parent}
}

func installAdmissionCensus(t *testing.T, census *admissionCensus, cores int) {
	t.Helper()
	previous := loadSeams
	loadSeams.host = func(now time.Time) hostload.Sample {
		return hostload.Sample{Available: true, Cores: cores, Load1m: 1, At: now.UTC().Format(time.RFC3339Nano)}
	}
	loadSeams.launchers = countProofLaunchers
	loadSeams.nested = nestedProofLauncher
	loadSeams.prober = census
	loadSeams.pids = census.pids
	loadSeams.parent = census.parent
	t.Cleanup(func() { loadSeams = previous })
}

func censusAdmissionRequest(t *testing.T, max int, exact identity.Exact, commandClass string, now time.Time) AdmissionRequest {
	t.Helper()
	root, proofIdentity := proofAttemptFixture(t, commandClass)
	conf := filepath.Join(root, "admission.conf")
	writeTestFile(t, conf, []byte(AdmissionCapKey+"="+strconv.Itoa(max)+"\n"), 0o600)
	return candidateAdmission(AdmissionRequest{ControlRoot: root, ExecutionRoot: root, ConfPath: conf, GoalID: "goal-a", GoalRevision: 2,
		AccountingRevision: 2, ReservedMinutes: 2, Identity: proofIdentity, Launcher: processIdentity(exact, 0), Now: now})
}

func TestAdmissionCapResolvesFromConfigurationAndCores(t *testing.T) {
	tests := []struct {
		name, value, source string
		cores, want         int
		wantErr             bool
	}{
		{"absent-18", "", "cores", 18, 3, false},
		{"absent-8", "", "cores", 8, 1, false},
		{"absent-3", "", "cores", 3, 1, false},
		{"absent-negative", "", "cores", -1, 1, false},
		{"absent-key-in-a-populated-conf", "unrelated.value=1", "cores", 18, 3, false},
		{"configured", "5", "configured", 18, 5, false},
		{"disabled", "0", "configured", 18, 0, false},
		{"not-an-integer", "x", "", 18, 0, true},
		{"negative", "-1", "", 18, 0, true},
		{"too-large", "65", "", 18, 0, true},
		{"duplicate-key", AdmissionCapKey + "=1\n" + AdmissionCapKey + "=2", "", 18, 0, true},
		{"empty-path", "derived", "cores", 0, 1, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			conf := ""
			if test.value != "derived" {
				conf = filepath.Join(t.TempDir(), "metasystem.conf")
				content := test.value
				if content != "" && !strings.Contains(content, "=") {
					content = AdmissionCapKey + "=" + content
				}
				writeTestFile(t, conf, []byte(content+"\n"), 0o600)
			}
			admission, err := ResolveAdmissionCap(conf, test.cores)
			if test.wantErr {
				if err == nil || !strings.Contains(err.Error(), AdmissionCapKey) || test.name != "duplicate-key" && !strings.Contains(err.Error(), "0 through 64") {
					t.Fatalf("error = %v", err)
				}
				return
			}
			if err != nil || admission.Max != test.want || admission.Key != AdmissionCapKey || admission.Source != test.source {
				t.Fatalf("admission = %+v, error = %v", admission, err)
			}
		})
	}
}
func admissionRequest(t *testing.T, admissionMax, overlaps int, known, nested bool) AdmissionRequest {
	t.Helper()
	installFakeLoad(t, hostload.Sample{Available: true, Cores: 18}, overlaps, known)
	loadSeams.nested = func(int64) (bool, bool) { return nested, true }
	root, identity := proofAttemptFixture(t, "admission")
	conf := filepath.Join(root, "admission.conf")
	writeTestFile(t, conf, []byte(AdmissionCapKey+"="+strconv.Itoa(admissionMax)+"\n"), 0o600)
	launcher, err := CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	return candidateAdmission(AdmissionRequest{ControlRoot: root, ExecutionRoot: root, ConfPath: conf, GoalID: "goal-a", GoalRevision: 2,
		AccountingRevision: 2, ReservedMinutes: 2, Identity: identity, Launcher: launcher, Now: time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)})
}
func TestAdmissionCapExemptsNestedReceipts(t *testing.T) {
	request := admissionRequest(t, 1, 3, true, true)
	attempt, decision, err := ReserveLocked(candidateAdmission(request))
	if err != nil || decision.Disposition != DispositionExecuted || attempt.AttemptID == "" {
		t.Fatalf("nested reserve = %+v, %+v, %v", attempt, decision, err)
	}
	stored, err := ReadAttempt(request.ControlRoot, attempt.AttemptID)
	if err != nil || stored.AttemptID != attempt.AttemptID {
		t.Fatalf("nested attempt was not written: %+v, %v", stored, err)
	}
	joined, _, err := JoinedComponentDecisionLocked(candidateAdmission(request))
	if err != nil || joined.Disposition == DispositionAdmissionRefused {
		t.Fatalf("joined decision consulted admission cap: %+v, %v", joined, err)
	}
}
func TestAdmissionCapRefusesAtTheCap(t *testing.T) {
	for _, test := range []struct {
		name, want string
		overlaps   int
	}{{"at-cap", DispositionAdmissionRefused, 2}, {"below-cap", DispositionExecuted, 1}} {
		t.Run(test.name, func(t *testing.T) {
			request := admissionRequest(t, 2, test.overlaps, true, false)
			attempt, decision, err := ReserveLocked(candidateAdmission(request))
			if err != nil || decision.Disposition != test.want {
				t.Fatalf("reserve = %+v, %+v, %v", attempt, decision, err)
			}
			attempts, readErr := ReadAttempts(request.ControlRoot)
			if test.want == DispositionAdmissionRefused && (!reflect.DeepEqual(attempt, Attempt{}) || decision.ExitStatus != ExitAdmissionRefused || readErr != nil || len(attempts) != 0) {
				t.Fatalf("refusal left state: attempt=%+v decision=%+v retained=%d error=%v", attempt, decision, len(attempts), readErr)
			}
		})
	}
}

func TestAdmissionCapAdmitsBesideIdleBatchOwner(t *testing.T) {
	now := time.Date(2026, 9, 17, 9, 0, 0, 0, time.UTC)
	owner := admissionProcess(101, 1, now.Add(-time.Hour), "metasystem", "landing", "batch", "owner")
	requester := admissionProcess(201, 1, now.Add(-time.Minute), "metasystem", "test", "run")
	census := &admissionCensus{}
	census.replace(owner, requester)
	installAdmissionCensus(t, census, 8)

	request := censusAdmissionRequest(t, 1, requester.exact, "beside-idle-owner", now)
	attempt, decision, err := ReserveLocked(request)
	if err != nil || decision.Disposition != DispositionExecuted || attempt.AttemptID == "" {
		t.Fatalf("top-level reserve beside idle owner = %+v, %+v, %v", attempt, decision, err)
	}
	if attempt.Load == nil || !attempt.Load.Start.OverlapKnown || attempt.Load.Start.OverlappingHost != 0 {
		t.Fatalf("idle owner occupied a cap slot: %+v", attempt.Load)
	}
}

func TestAdmissionSlotLifecycleAndRefusalLeavesNoState(t *testing.T) {
	for _, ending := range []string{"normal-exit", "crash", "kill"} {
		t.Run(ending, func(t *testing.T) {
			now := time.Date(2026, 9, 17, 10, 0, 0, 0, time.UTC)
			first := admissionProcess(110, 1, now.Add(-2*time.Minute), "metasystem", "test", "run")
			second := admissionProcess(210, 1, now.Add(-time.Minute), "metasystem", "proof-run", "launch")
			census := &admissionCensus{}
			census.replace(first)
			installAdmissionCensus(t, census, 8)

			firstRequest := censusAdmissionRequest(t, 1, first.exact, "slot-holder-"+ending, now)
			firstRequest.AttemptID = "proof-slot-holder-" + ending
			firstAttempt, firstDecision, err := ReserveLocked(firstRequest)
			if err != nil || firstDecision.Disposition != DispositionExecuted || firstAttempt.AttemptID == "" {
				t.Fatalf("first top-level reserve = %+v, %+v, %v", firstAttempt, firstDecision, err)
			}

			census.replace(first, second)
			secondRequest := censusAdmissionRequest(t, 1, second.exact, "refused-"+ending, now.Add(time.Second))
			secondRequest.AttemptID = "proof-refused-" + ending
			secondRequest.ReservationOwner = &ReservationOwner{ControlRoot: secondRequest.ControlRoot, RunID: "run-" + ending,
				RunGeneration: 1, LaunchNonce: "launch-" + ending, GoalRevision: secondRequest.GoalRevision,
				ObligationRevision: 1, AttemptOrdinal: 1, Deadline: now.Add(time.Minute).Format(time.RFC3339Nano)}
			refusedAttempt, refused, err := ReserveLocked(secondRequest)
			if err != nil || refused.Disposition != DispositionAdmissionRefused || refused.ExitStatus != ExitAdmissionRefused ||
				!strings.Contains(refused.Reason, "retry=retry-when-a-launcher-ends") || !reflect.DeepEqual(refusedAttempt, Attempt{}) {
				t.Fatalf("second top-level reserve = %+v, %+v, %v", refusedAttempt, refused, err)
			}
			attempts, err := ReadAttempts(secondRequest.ControlRoot)
			if err != nil || len(attempts) != 0 {
				t.Fatalf("refused reserve retained %d attempt(s): %v", len(attempts), err)
			}

			// The refused launcher exits without creating another kind of slot.
			census.replace(first)
			if got, known := countProofLaunchers(999); !known || got != 1 {
				t.Fatalf("census after refusal = %d, known=%v, want only the first launcher", got, known)
			}
			// Normal exit, crash, and kill have the same slot lifecycle: once the
			// launcher is absent from the census, its nonterminal record cannot
			// keep the host slot occupied.
			census.replace()
			if got, known := countProofLaunchers(999); !known || got != 0 {
				t.Fatalf("census after %s = %d, known=%v, want no slot", ending, got, known)
			}

			// Reusing the refused attempt id proves that refusal left no record or
			// reservation behind. The restarted launcher is self, not an overlap.
			census.replace(second)
			admitted, decision, err := ReserveLocked(secondRequest)
			if err != nil || decision.Disposition != DispositionExecuted || admitted.AttemptID != secondRequest.AttemptID {
				t.Fatalf("reserve after %s = %+v, %+v, %v", ending, admitted, decision, err)
			}
		})
	}
}
func TestAdmissionRefusalNamesItsExpiryAndRuling(t *testing.T) {
	reason := (AdmissionCap{Max: 2, Key: AdmissionCapKey, Source: "configured"}).RefusalReason(LoadSample{OverlappingHost: 3, OverlapKnown: true}, true)
	for _, part := range []string{AdmissionCapKey, "admitted=2", "observed=3", "ruling=R-111-m1e", "tests-never-wait-on-wall-time:1e,2,3b", "engine-policy-binding-survives-drift-and-load:U4a,U4b"} {
		if !strings.Contains(reason, part) {
			t.Fatalf("reason %q does not contain %q", reason, part)
		}
	}
}
func TestAdmissionCapAdmitsWhenOverlapIsUnknown(t *testing.T) {
	request := admissionRequest(t, 1, 99, false, false)
	attempt, decision, err := ReserveLocked(candidateAdmission(request))
	if err != nil || decision.Disposition != DispositionAdmissionRefused || decision.ExitStatus != ExitAdmissionRefused ||
		!strings.Contains(decision.Reason, "ADMISSION_OVERLAP_UNKNOWN") || !reflect.DeepEqual(attempt, Attempt{}) {
		t.Fatalf("unknown overlap reserve = %+v, %+v, %v", attempt, decision, err)
	}
	stored, err := ReadAttempts(request.ControlRoot)
	if err != nil || len(stored) != 0 {
		t.Fatalf("unknown overlap refusal retained attempts: %+v, %v", stored, err)
	}
}

func TestInvalidFixtureHostLoadRefusesUnknownOverlap(t *testing.T) {
	request := admissionRequest(t, 1, 0, true, false)
	request.loadOptions = []loadSampleOption{withTestHostLoad("invalid")}
	attempt, decision, err := ReserveLocked(candidateAdmission(request))
	if err != nil || decision.Disposition != DispositionAdmissionRefused ||
		!strings.Contains(decision.Reason, "ADMISSION_OVERLAP_UNKNOWN") || !reflect.DeepEqual(attempt, Attempt{}) {
		t.Fatalf("invalid fixture reserve = %+v, %+v, %v", attempt, decision, err)
	}
}
func TestAdmissionCapRefusalPredicate(t *testing.T) {
	check := func(name string, max, observed int, known, nested, nestedKnown, want bool) {
		t.Run(name, func(t *testing.T) {
			if got := (AdmissionCap{Max: max}).Refuses(LoadSample{OverlappingHost: observed, OverlapKnown: known}, nested, nestedKnown); got != want {
				t.Fatalf("refuses = %v, want %v", got, want)
			}
		})
	}
	check("unknown-host", 1, 99, false, false, true, true)
	check("disabled", 0, 99, true, false, true, false)
	check("nested", 1, 99, true, true, true, false)
	check("nested-unknown-at-cap", 1, 99, true, false, false, true)
	check("nested-unknown-below-cap", 2, 1, true, false, false, false)
	check("at-cap", 2, 2, true, false, true, true)
	check("below-cap", 2, 1, true, false, true, false)
}

func TestAdmissionCapRefusesUnknownNestingAtCapacity(t *testing.T) {
	cap := AdmissionCap{Max: 2, Key: AdmissionCapKey, Source: "configured"}
	sample := LoadSample{OverlappingHost: 2, OverlapKnown: true}
	if !cap.Refuses(sample, false, false) {
		t.Fatal("an unreadable nesting census admitted a launcher at capacity")
	}
	if reason := cap.RefusalReason(sample, false); !strings.Contains(reason, "ADMISSION_NESTING_UNKNOWN") {
		t.Fatalf("unknown nesting reason = %q", reason)
	}
}
func TestAdmissionCapUsesTheRecordedStartSample(t *testing.T) {
	request := admissionRequest(t, 2, 0, true, false)
	calls := 0
	loadSeams.launchers = func(int64) (int, bool) {
		calls++
		if calls == 1 {
			return 1, true
		}
		return 99, true
	}
	attempt, decision, err := ReserveLocked(candidateAdmission(request))
	if err != nil || decision.Disposition != DispositionExecuted || calls != 1 || attempt.Load.Start.OverlappingHost != 1 {
		t.Fatalf("reserve sampled %d times: attempt=%+v decision=%+v error=%v", calls, attempt, decision, err)
	}
}

func TestReserveSkipsTheNestedCensusWhenTheCapCannotRefuse(t *testing.T) {
	for _, test := range []struct {
		name            string
		admissionMax    int
		overlapKnown    bool
		wantNestedCalls int
	}{
		{name: "disabled", admissionMax: 0, overlapKnown: true, wantNestedCalls: 0},
		{name: "overlap-unknown", admissionMax: 2, overlapKnown: false, wantNestedCalls: 0},
		{name: "enabled-and-known", admissionMax: 2, overlapKnown: true, wantNestedCalls: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := admissionRequest(t, test.admissionMax, 0, test.overlapKnown, false)
			calls := 0
			loadSeams.nested = func(int64) (bool, bool) {
				calls++
				return false, true
			}
			attempt, decision, err := ReserveLocked(candidateAdmission(request))
			wantDisposition := DispositionExecuted
			if !test.overlapKnown && test.admissionMax > 0 {
				wantDisposition = DispositionAdmissionRefused
			}
			if err != nil || decision.Disposition != wantDisposition || wantDisposition == DispositionExecuted && attempt.AttemptID == "" {
				t.Fatalf("reserve = %+v, %+v, %v", attempt, decision, err)
			}
			if calls != test.wantNestedCalls {
				t.Fatalf("nested census calls = %d, want %d", calls, test.wantNestedCalls)
			}
		})
	}
}

type admissionBatteryEvent struct {
	battery  string
	stage    string
	attempt  Attempt
	decision LaunchResult
	err      error
}

// The first call competes for the real nonblocking host guard. A busy caller
// retries only after the competing call has returned and released that guard.
func reserveConcurrentBattery(request AdmissionRequest, returned chan<- struct{}, competitorReturned, abort <-chan struct{}) (Attempt, LaunchResult, error) {
	attempt, decision, err := ReserveLocked(request)
	close(returned)
	if err == nil && decision.Disposition == DispositionAdmissionRefused && decision.Reason == "host proof admission is busy" {
		select {
		case <-competitorReturned:
		case <-abort:
			return attempt, decision, err
		}
		return ReserveLocked(request)
	}
	return attempt, decision, err
}

func TestConcurrentTopLevelAttemptsCompleteWithNestedAndJoinedReceipts(t *testing.T) {
	assertAdmissionHasNoWaitPath(t)
	now := time.Date(2026, 9, 17, 11, 0, 0, 0, time.UTC)
	topA := admissionProcess(120, 1, now.Add(-2*time.Minute), "metasystem", "test", "run")
	topB := admissionProcess(220, 1, now.Add(-2*time.Minute), "metasystem", "test", "run")
	childA := admissionProcess(121, topA.exact.Pid, now.Add(-time.Minute), "metasystem", "proof-run", "launch")
	childB := admissionProcess(221, topB.exact.Pid, now.Add(-time.Minute), "metasystem", "proof-run", "launch")
	extra := admissionProcess(320, 1, now.Add(-3*time.Minute), "metasystem", "test", "run")
	census := &admissionCensus{}
	census.replace(topA, topB)
	installAdmissionCensus(t, census, 18)

	type battery struct {
		name       string
		top, child admissionCensusProcess
		request    AdmissionRequest
	}
	batteries := []battery{
		{name: "a", top: topA, child: childA, request: censusAdmissionRequest(t, 2, topA.exact, "concurrent-a", now)},
		{name: "b", top: topB, child: childB, request: censusAdmissionRequest(t, 2, topB.exact, "concurrent-b", now)},
	}
	for index := range batteries {
		batteries[index].request.AttemptID = "proof-concurrent-" + batteries[index].name
	}

	events := make(chan admissionBatteryEvent, 8)
	startTop := make(chan struct{})
	nestedReceipts := make(chan struct{})
	joinedReceipts := make(chan struct{})
	finish := make(chan struct{})
	topReturned := []chan struct{}{make(chan struct{}), make(chan struct{})}
	nestedReturned := []chan struct{}{make(chan struct{}), make(chan struct{})}
	var nestedOnce, joinedOnce, finishOnce sync.Once
	var workers sync.WaitGroup
	defer func() {
		nestedOnce.Do(func() { close(nestedReceipts) })
		joinedOnce.Do(func() { close(joinedReceipts) })
		finishOnce.Do(func() { close(finish) })
		workers.Wait()
	}()
	for index, fixture := range batteries {
		fixture := fixture
		index := index
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-startTop
			topAttempt, topDecision, err := reserveConcurrentBattery(fixture.request, topReturned[index], topReturned[1-index], finish)
			events <- admissionBatteryEvent{battery: fixture.name, stage: "top", attempt: topAttempt, decision: topDecision, err: err}
			if err != nil || topDecision.Disposition != DispositionExecuted {
				return
			}
			<-nestedReceipts

			nestedRequest := fixture.request
			nestedRequest.AttemptID = "proof-concurrent-" + fixture.name + "-nested"
			nestedRequest.Now = now.Add(time.Second)
			nestedRequest.Launcher = processIdentity(fixture.child.exact, 0)
			nestedRequest.Identity.CommandClass += "-nested"
			nestedRequest.Identity.IdentityDigest = nestedRequest.Identity.digest()
			nestedAttempt, nestedDecision, err := reserveConcurrentBattery(nestedRequest, nestedReturned[index], nestedReturned[1-index], finish)
			events <- admissionBatteryEvent{battery: fixture.name, stage: "nested", attempt: nestedAttempt, decision: nestedDecision, err: err}
			if err != nil || nestedDecision.Disposition != DispositionExecuted {
				return
			}
			<-joinedReceipts

			componentIdentity := strings.Repeat(fixture.name, 64)
			joinedRequest := candidateAdmission(AdmissionRequest{ControlRoot: fixture.request.ControlRoot, GoalID: fixture.request.GoalID,
				GoalRevision: fixture.request.GoalRevision, AccountingRevision: fixture.request.AccountingRevision,
				ComponentIdentities: map[string]string{"group-" + fixture.name: componentIdentity}})
			joinedDecision, decided, err := JoinedComponentDecisionLocked(joinedRequest)
			if err == nil && decided {
				err = &unexpectedJoinedDecisionError{}
			} else if err == nil {
				_, err = BindJoinedTestComponentsLocked(fixture.request.ControlRoot, topAttempt.AttemptID, joinedRequest.ComponentIdentities)
				if err == nil {
					joinedDecision = LaunchResult{SchemaVersion: 1, Disposition: DispositionExecuted, AttemptID: topAttempt.AttemptID}
				}
			}
			events <- admissionBatteryEvent{battery: fixture.name, stage: "joined", decision: joinedDecision, err: err}
			if err != nil || joinedDecision.Disposition == DispositionAdmissionRefused {
				return
			}

			<-finish
			if _, err = FinalizeAttemptLocked(fixture.request.ControlRoot, nestedAttempt.AttemptID, TerminalSuccess, 0, "nested green", nil, now.Add(2*time.Second)); err == nil {
				topAttempt, err = FinalizeAttemptLocked(fixture.request.ControlRoot, topAttempt.AttemptID, TerminalSuccess, 0, "battery green", nil, now.Add(3*time.Second))
			}
			events <- admissionBatteryEvent{battery: fixture.name, stage: "complete", attempt: topAttempt, err: err}
		}()
	}
	close(startTop)

	seenTop := map[string]bool{}
	for range batteries {
		event := <-events
		if event.err != nil || event.stage != "top" || event.decision.Disposition != DispositionExecuted || event.attempt.AttemptID == "" {
			t.Fatalf("top-level battery event = %+v", event)
		}
		seenTop[event.battery] = true
	}
	if len(seenTop) != 2 {
		t.Fatalf("top-level batteries admitted = %v", seenTop)
	}

	// Once both top-level attempts hold their slots, an unrelated launcher
	// fills the census to the cap. Nested receipts must still bypass it, and
	// joined receipts never consult it.
	census.replace(topA, topB, childA, childB, extra)
	nestedOnce.Do(func() { close(nestedReceipts) })
	seenReceipts := map[string]map[string]bool{"a": {}, "b": {}}
	for count := 0; count < 2; count++ {
		event := <-events
		if event.err != nil || event.stage != "nested" || event.decision.Disposition != DispositionExecuted {
			t.Fatalf("nested battery event = %+v", event)
		}
		if event.attempt.AttemptID == "" {
			t.Fatalf("nested battery event = %+v", event)
		}
		seenReceipts[event.battery][event.stage] = true
	}
	joinedOnce.Do(func() { close(joinedReceipts) })
	for count := 0; count < 2; count++ {
		event := <-events
		if event.err != nil || event.stage != "joined" || event.decision.Disposition != DispositionExecuted {
			t.Fatalf("joined battery event = %+v", event)
		}
		seenReceipts[event.battery][event.stage] = true
	}
	for battery, stages := range seenReceipts {
		if !stages["nested"] || !stages["joined"] {
			t.Fatalf("battery %s receipt stages = %v", battery, stages)
		}
	}

	finishOnce.Do(func() { close(finish) })
	for range batteries {
		event := <-events
		if event.err != nil || event.stage != "complete" || event.attempt.Terminal == nil || event.attempt.Terminal.Result != TerminalSuccess {
			t.Fatalf("completed battery event = %+v", event)
		}
	}
}

type unexpectedJoinedDecisionError struct{}

func (*unexpectedJoinedDecisionError) Error() string {
	return "joined receipt was decided before its components were bound"
}

func TestAdmissionCapHasNoWaitPath(t *testing.T) {
	assertAdmissionHasNoWaitPath(t)
}

func TestHeldHostAdmissionGuardRefusesWithoutAllocatingAttempt(t *testing.T) {
	previousDirectory := hostAdmissionDirectoryForTest
	hostAdmissionDirectoryForTest = filepath.Join(t.TempDir(), "host-admission")
	t.Cleanup(func() { hostAdmissionDirectoryForTest = previousDirectory })
	directory, err := hostAdmissionDirectory()
	if err != nil {
		t.Fatal(err)
	}
	guard, acquired, err := tryHostFile(filepath.Join(directory, "admission.lock"))
	if err != nil || !acquired {
		t.Fatalf("hold host admission guard: acquired=%t err=%v", acquired, err)
	}
	defer guard.Close()
	request := admissionRequest(t, 0, 0, true, false)
	request.AttemptID = "held-guard-probe"
	samples := 0
	loadSeams.host = func(time.Time) hostload.Sample {
		samples++
		return hostload.Sample{Available: true, Cores: 18}
	}
	attempt, decision, err := ReserveLocked(request)
	if err != nil || decision.Disposition != DispositionAdmissionRefused || decision.ExitStatus != ExitAdmissionRefused ||
		attempt.AttemptID != "" || samples != 0 {
		t.Fatalf("contended admission allocated or sampled: attempt=%+v decision=%+v samples=%d err=%v", attempt, decision, samples, err)
	}
	attempts, err := ReadAttempts(request.ControlRoot)
	if err != nil || len(attempts) != 0 {
		t.Fatalf("contended admission retained partial attempt inventory: attempts=%+v err=%v", attempts, err)
	}
	t.Run("abort releases busy peer after competitor exits", func(t *testing.T) {
		returned := make(chan struct{})
		competitorReturned := make(chan struct{})
		abort := make(chan struct{})
		result := make(chan admissionBatteryEvent, 1)
		go func() {
			attempt, decision, err := reserveConcurrentBattery(request, returned, competitorReturned, abort)
			result <- admissionBatteryEvent{attempt: attempt, decision: decision, err: err}
		}()
		<-returned // The real guard has refused this worker's first call.
		close(abort)
		event := <-result
		if event.err != nil || event.decision.Disposition != DispositionAdmissionRefused || event.decision.Reason != "host proof admission is busy" || event.attempt.AttemptID != "" {
			t.Fatalf("aborted busy worker = %+v", event)
		}
	})
	t.Run("private fake roots retain real independent guards", func(t *testing.T) {
		fakeRoot := func() string {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			return root
		}
		firstRoot, secondRoot := fakeRoot(), fakeRoot()
		firstPath := filepath.Join(firstRoot, "admission")
		secondPath := filepath.Join(secondRoot, "admission")
		first, err := hostAdmissionDirectoryForRequest(firstRoot, firstPath)
		if err != nil || first != firstPath {
			t.Fatalf("first fixture directory = %q, %v", first, err)
		}
		second, err := hostAdmissionDirectoryForRequest(secondRoot, secondPath)
		if err != nil || second != secondPath || second == first {
			t.Fatalf("second fixture directory = %q, %v", second, err)
		}
		firstGuard, held, err := tryHostFile(filepath.Join(first, "admission.lock"))
		if err != nil || !held {
			t.Fatalf("first fixture guard = %t, %v", held, err)
		}
		defer firstGuard.Close()
		contender, held, err := tryHostFile(filepath.Join(first, "admission.lock"))
		if contender != nil {
			contender.Close()
		}
		if err != nil || held {
			t.Fatalf("shared fixture domain did not contend: held=%t err=%v", held, err)
		}
		secondGuard, held, err := tryHostFile(filepath.Join(second, "admission.lock"))
		if err != nil || !held {
			t.Fatalf("independent fixture domain was blocked: held=%t err=%v", held, err)
		}
		defer secondGuard.Close()
		if defaultPath, err := hostAdmissionDirectoryForRequest(firstRoot, ""); err != nil || defaultPath != directory {
			t.Fatalf("absent selector changed default directory: %q, %v", defaultPath, err)
		}
		if _, err := hostAdmissionDirectoryForRequest(request.ControlRoot, filepath.Join(t.TempDir(), "invalid")); err == nil {
			t.Fatal("nonfake control root selected private admission directory")
		}
		if _, err := hostAdmissionDirectoryForRequest(firstRoot, filepath.Join(string(filepath.Separator), "outside-proof-admission")); err == nil {
			t.Fatal("outside-temp path selected private admission directory")
		}
	})
}

func assertAdmissionHasNoWaitPath(t *testing.T) {
	t.Helper()
	_, testPath, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate admission test source")
	}
	targets := map[string]map[string]bool{
		"admission.go": {"ResolveAdmissionCap": true, "Refuses": true, "RefusalReason": true},
		"attempt.go":   {"ReserveLocked": true},
	}
	var violations []string
	for file, functions := range targets {
		path := filepath.Join(filepath.Dir(testPath), file)
		set := token.NewFileSet()
		parsed, err := parser.ParseFile(set, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}
		for _, declaration := range parsed.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || !functions[function.Name.Name] {
				continue
			}
			ast.Inspect(function.Body, func(node ast.Node) bool {
				switch expression := node.(type) {
				case *ast.ForStmt, *ast.RangeStmt, *ast.SelectStmt, *ast.SendStmt:
					violations = append(violations, file+":"+function.Name.Name+":"+set.Position(node.Pos()).String())
				case *ast.UnaryExpr:
					if expression.Op == token.ARROW {
						violations = append(violations, file+":"+function.Name.Name+":"+set.Position(node.Pos()).String())
					}
				case *ast.CallExpr:
					name, qualifier := "", ""
					switch called := expression.Fun.(type) {
					case *ast.Ident:
						name = called.Name
					case *ast.SelectorExpr:
						name = called.Sel.Name
						if selected, ok := called.X.(*ast.Ident); ok {
							qualifier = selected.Name
						}
					}
					lower := strings.ToLower(name)
					if strings.Contains(lower, "sleep") || strings.Contains(lower, "wait") || strings.Contains(lower, "block") ||
						strings.Contains(lower, "queue") || strings.Contains(lower, "acquire") ||
						qualifier == "time" && (name == "After" || name == "NewTimer" || name == "NewTicker") {
						violations = append(violations, file+":"+function.Name.Name+":"+set.Position(node.Pos()).String())
					}
				}
				return true
			})
		}
	}
	if len(violations) > 0 {
		sort.Strings(violations)
		t.Fatalf("admission must return a decision without waiting or sleeping: %s", strings.Join(violations, ", "))
	}
}
