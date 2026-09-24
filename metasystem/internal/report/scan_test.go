package report

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/brain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testgoal"
)

func TestClipDetailKeepsValidUTF8AtTheByteBoundary(t *testing.T) {
	full := strings.Repeat("a", 199) + "é" + " complete detail"
	clipped := clipDetail(full)
	if len(clipped) > 200 || !utf8.ValidString(clipped) || clipped != strings.Repeat("a", 199) {
		t.Fatalf("byte-safe clip = bytes %d %q", len(clipped), clipped)
	}

	invalidBeforeBoundary := strings.Repeat("a", 40) + string([]byte{0xff}) + strings.Repeat("b", 200)
	clipped = clipDetail(invalidBeforeBoundary)
	if clipped != invalidBeforeBoundary[:200] {
		t.Fatalf("an earlier invalid byte moved the clip boundary: bytes=%d", len(clipped))
	}
}

type scanProber struct {
	verdicts map[int64]identity.Liveness
	starts   map[int64]int64
}

func (f scanProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	verdict, ok := f.verdicts[pid]
	if !ok {
		return identity.Exact{}, identity.Dead, nil
	}
	switch verdict {
	case identity.Alive:
		return identity.Exact{Pid: pid, StartedAt: time.Unix(f.starts[pid], 0)}, identity.Alive, nil
	case identity.Unknown:
		return identity.Exact{}, identity.Unknown, errors.New("procfs unreadable")
	default:
		return identity.Exact{}, identity.Dead, nil
	}
}

func writeFile(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func newDraftScanRoot(t *testing.T) (string, *scanReadFixture) {
	t.Helper()
	root := resolveRepo(t.TempDir())
	fixture := newScanReadFixture(t, root)
	fixture.world = true
	fixture.setDraftFiles()
	for name, data := range fixture.files {
		writeFile(t, root, name, string(data))
	}
	return root, fixture
}

func TestBrainDeclarationScanIncludesPendingSetupBeforeDeclaration(t *testing.T) {
	root := resolveRepo(t.TempDir())
	absent := newScanReadFixture(t, root)
	absent.absent = true
	writeFile(t, root, "artifacts/agents/jobs/pending-setup.json",
		`{"jobId":"pending-setup","status":"pending-setup","role":"implementer","runtime":"fake","chainClosed":true}`)
	absent.expect("identity", "endpoint", "accepted", "world")
	ordinary := scanWithProberAtWithReads(root, identity.KernelProber{}, absent.committed, absent.reads())
	absent.checked(0)
	if len(ordinary.Busy) != 0 || len(ordinary.Jobs) != 0 {
		t.Fatalf("ordinary undeclared scan changed trunk's in-flight set: %+v", ordinary)
	}
	absent.expect("endpoint", "accepted", "world")
	declaration := scanForBrainDeclarationWithReads(root, absent.reads())
	absent.checked(0)
	if len(declaration.Busy) != 1 || declaration.Busy[0].Id != "pending-setup" || len(declaration.Jobs) != 1 || declaration.Jobs[0].Status != "pending-setup" {
		t.Fatalf("brain declaration scan missed pending-setup: %+v", declaration)
	}

	declaredRoot, declaredReads := newDraftScanRoot(t)
	writeFile(t, declaredRoot, "artifacts/agents/jobs/pending-setup.json",
		`{"jobId":"pending-setup","status":"pending-setup","role":"implementer","runtime":"fake","chainClosed":true}`)
	record := brain.Record{
		Schema: brain.Schema, Ledger: scanFixtureLedger, Machine: "bed-m1",
		DeclaredBy: "Wido", DeclaredAt: "2026-09-07T00:00:00Z",
	}
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, declaredRoot, "artifacts/agents/brain.json", string(data)+"\n")
	declaredReads.expect("identity", "endpoint", "accepted", "files", "commitTime", "world", "machine", "endpoint", "accepted", "files", "commitTime")
	declared := scanWithProberAtWithReads(declaredRoot, identity.KernelProber{}, declaredReads.committed, declaredReads.reads())
	declaredReads.checked(2)
	if len(declared.Busy) != 1 || declared.Busy[0].Id != "pending-setup" || len(declared.Jobs) != 1 || declared.Jobs[0].Status != "pending-setup" {
		t.Fatalf("declared brain scan did not classify pending-setup as in flight: %+v", declared)
	}
}

func TestQuestionAndDraftScansKeepUndeclaredVerdictUnchanged(t *testing.T) {
	root, fixture := newDraftScanRoot(t)
	writeFile(t, root, "artifacts/agents/channel/questions/valid.json",
		`{"id":"valid","goal":"draft-here","kind":"other","machine":"bed-m2","openedAt":"2026-09-07T00:01:00Z","wants":"Choose the safe option.","state":"open"}`)
	writeFile(t, root, "artifacts/agents/channel/questions/broken.json", `{broken`)

	fixture.machine = ""
	fixture.expect("identity", "endpoint", "accepted", "files", "commitTime", "world", "machine")
	unreadable := scanWithProberAtWithReads(root, identity.KernelProber{}, fixture.committed, fixture.reads())
	fixture.checked(1)
	if len(unreadable.Questions) != 1 || unreadable.Questions[0].Id != "valid" {
		t.Fatalf("valid question was hidden by its malformed neighbor: %+v", unreadable.Questions)
	}
	details := strings.Join(unreadable.Unreadable, "\n")
	if !strings.Contains(details, "question scan:") || !strings.Contains(details, "broken.json") ||
		!strings.Contains(details, "draft scan:") || !strings.Contains(details, "no machine nickname") {
		t.Fatalf("question and draft read failures were not named: %v", unreadable.Unreadable)
	}

	// Question and draft read failures do not make an undeclared checkout uncertain.
	repo := testgoal.New(fixture.files, fixture.committed, scanFixtureTip)
	verdict, err := (&goal.Store{Root: root}).TurnVerdictAtEndpoint(
		goal.Endpoint{Root: root, Remote: "local", Repository: repo},
		"bed-m1", unreadable, "undeclared-report-scan", "", "")
	if err != nil || verdict.ShouldBlock || strings.Contains(verdict.Display, "UNCERTAIN") {
		t.Fatalf("brain-only scanner failures changed an undeclared checkout's verdict: %+v %v", verdict, err)
	}

	fixture.machine = "bed-m1"
	fixture.expect("identity", "endpoint", "accepted", "files", "commitTime", "world", "machine", "endpoint", "accepted", "files", "commitTime")
	readable := scanWithProberAtWithReads(root, identity.KernelProber{}, fixture.committed, fixture.reads())
	fixture.checked(2)
	if len(readable.Drafts) != 1 || readable.Drafts[0].Id != "draft-here" ||
		readable.Drafts[0].Detail != "Have a human approve this draft." {
		t.Fatalf("machine-owned queued draft was not scanned: %+v", readable.Drafts)
	}
	if len(readable.Questions) != 1 || readable.Questions[0].Id != "valid" {
		t.Fatalf("valid question disappeared after enrollment: %+v", readable.Questions)
	}

	for _, test := range []struct {
		name  string
		set   func(*scanReadFixture)
		calls []string
		want  string
	}{
		{"endpoint error", func(f *scanReadFixture) { f.endpointErr = errors.New("endpoint unavailable") },
			[]string{"identity", "endpoint", "world", "machine", "endpoint"}, "endpoint unavailable"},
		{"accepted read error", func(f *scanReadFixture) { f.acceptedErr = errors.New("accepted read failed") },
			[]string{"identity", "endpoint", "accepted", "world", "machine", "endpoint", "accepted"}, "accepted read failed"},
		{"committed files error", func(f *scanReadFixture) { f.filesErr = errors.New("committed files unreadable") },
			[]string{"identity", "endpoint", "accepted", "files", "world", "machine", "endpoint", "accepted", "files"}, "committed files unreadable"},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newScanReadFixture(t, root)
			f.world = true
			f.setDraftFiles()
			test.set(f)
			f.expect(test.calls...)
			scan := scanWithProberAtWithReads(root, identity.KernelProber{}, f.committed, f.reads())
			f.checked(0)
			if !strings.Contains(strings.Join(scan.Unreadable, "\n"), "draft scan: "+test.want) {
				t.Fatalf("draft read failure missing: %v", scan.Unreadable)
			}
		})
	}
}

// The scanner classifies runner records three-way from file
// facts — live counts busy, completed/crashed/identity-mismatch do not,
// Unknown joins Unreadable and never reads as dead — and a stale
// heartbeat sidecar surviving completion changes nothing (nothing reads
// heartbeat freshness; the record+kernel rule is the one authority).
func TestScanResultClassification(t *testing.T) {
	root := t.TempDir()
	prober := scanProber{
		verdicts: map[int64]identity.Liveness{
			301: identity.Alive,
			302: identity.Dead,
			303: identity.Unknown,
			304: identity.Alive,
		},
		starts: map[int64]int64{301: 7000, 304: 8000},
	}
	runners := "artifacts/agents/missions/runners/"
	writeFile(t, root, runners+"m-live.json", `{"missionId":"m-live","status":"running","pid":301,"pidStartedAt":7000}`)
	writeFile(t, root, runners+"m-done.json", `{"missionId":"m-done","status":"completed","pid":301,"pidStartedAt":7000}`)
	// The stale sidecar: a heartbeat surviving completion.
	writeFile(t, root, runners+"m-done.heartbeat", `{"pid":301}`)
	writeFile(t, root, runners+"m-crashed.json", `{"missionId":"m-crashed","status":"running","pid":302,"pidStartedAt":7000}`)
	writeFile(t, root, runners+"m-unknown.json", `{"missionId":"m-unknown","status":"running","pid":303,"pidStartedAt":7000}`)
	writeFile(t, root, runners+"m-reused.json", `{"missionId":"m-reused","status":"running","pid":304,"pidStartedAt":1}`)

	// A live delegate job and one unparsable record.
	writeFile(t, root, "artifacts/agents/jobs/j1.json", `{"jobId":"j1","role":"impl","runtime":"codex","status":"running"}`)
	writeFile(t, root, "artifacts/agents/jobs/broken.json", `{nope`)

	scan := scanWithProberWithoutGoal(t, root, prober)

	var busyKinds []string
	for _, item := range scan.Busy {
		busyKinds = append(busyKinds, item.Kind+":"+item.Id)
	}
	joined := strings.Join(busyKinds, " ")
	if !strings.Contains(joined, "mission:m-live") {
		t.Fatalf("live mission not busy: %v", busyKinds)
	}
	for _, wrong := range []string{"m-done", "m-crashed", "m-reused"} {
		if strings.Contains(joined, wrong) {
			t.Fatalf("%s counted busy: %v", wrong, busyKinds)
		}
	}
	if !strings.Contains(joined, "job:j1") {
		t.Fatalf("live job not busy: %v", busyKinds)
	}

	// Unknown liveness and the unparsable job record both surface.
	unreadable := strings.Join(scan.Unreadable, " ")
	if !strings.Contains(unreadable, "m-unknown") {
		t.Fatalf("unknown-liveness runner did not surface: %v", scan.Unreadable)
	}
	if !strings.Contains(unreadable, "broken.json") {
		t.Fatalf("unparsable job record did not surface: %v", scan.Unreadable)
	}
}

// Another checkout's activity can never suppress this checkout's
// goal — the scan is file-fact scoped to its root, so a sibling root full
// of live work leaves this root idle.
func TestOtherCheckoutNeverSuppresses(t *testing.T) {
	parent := t.TempDir()
	busy := filepath.Join(parent, "busy-checkout")
	quiet := filepath.Join(parent, "quiet-checkout")
	prober := scanProber{
		verdicts: map[int64]identity.Liveness{401: identity.Alive},
		starts:   map[int64]int64{401: 9000},
	}
	writeFile(t, busy, "artifacts/agents/missions/runners/m-busy.json",
		`{"missionId":"m-busy","status":"running","pid":401,"pidStartedAt":9000}`)
	writeFile(t, busy, "artifacts/agents/jobs/j9.json",
		`{"jobId":"j9","role":"impl","runtime":"codex","status":"running"}`)
	os.MkdirAll(quiet, 0o755)

	scan := scanWithProberWithoutGoal(t, quiet, prober)
	if len(scan.Busy) != 0 {
		t.Fatalf("a sibling checkout's work counted busy here: %+v", scan.Busy)
	}
	if len(scan.Unreadable) != 0 {
		t.Fatalf("a quiet checkout reads unreadable: %v", scan.Unreadable)
	}
}

// Plans classify into Open, WaitingOnHuman, and Stale — with goals.md
// invisible to the plan scan (only the goal parser reads the ledger).
func TestScanPlansClassification(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "plans/active.md", "# P\n- Next step: Do the thing.\n- In flight right now: none\n")
	writeFile(t, root, "plans/waiting.md", "# P\n- Next step: Later.\n- Waiting on the human: a ruling\n")
	writeFile(t, root, "plans/goals.md", "# Goals\n- Next step: NEVER SCANNED\n")

	scan := scanWithProberWithoutGoal(t, root, scanProber{})
	if len(scan.Open) != 1 || !strings.Contains(scan.Open[0].Detail, "active.md") {
		t.Fatalf("open classification wrong: %+v", scan.Open)
	}
	if len(scan.WaitingOnHuman) != 1 || !strings.Contains(scan.WaitingOnHuman[0].Detail, "waiting.md") {
		t.Fatalf("waiting classification wrong: %+v", scan.WaitingOnHuman)
	}
	for _, item := range append(scan.Open, scan.WaitingOnHuman...) {
		if strings.Contains(item.Detail, "goals.md") {
			t.Fatalf("the ledger leaked into the plan scan: %+v", item)
		}
	}
}

func TestWaitOpenWorkSignatureReadsOnlyPlanStepsUnderContext(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "plans/active.md", "# P\n- Next step: Do the thing.\n- Waiting on the human: none\n")
	writeFile(t, root, "plans/waiting.md", "# P\n- Next step: Later.\n- Waiting on the human: a ruling\n")
	want := scanWithoutGoal(t, root).OpenWorkSignature()
	got, err := OpenWorkSignature(context.Background(), root)
	if err != nil || got != want || got == "" {
		t.Fatalf("plan-only signature=%q want=%q err=%v", got, want, err)
	}
	realReader := readOpenWorkPlan
	t.Cleanup(func() { readOpenWorkPlan = realReader })
	reads := 0
	entered := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan struct{})
	readOpenWorkPlan = func(_ string) ([]byte, error) {
		defer close(finished)
		reads++
		close(entered)
		<-release
		return nil, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() {
		_, resultErr := OpenWorkSignature(ctx, root)
		result <- resultErr
	}()
	<-entered
	cancel()
	err = <-result
	close(release)
	<-finished
	if !errors.Is(err, context.Canceled) || reads != 1 {
		t.Fatalf("canceled plan reader err=%v reads=%d", err, reads)
	}
}

// The monitor facility's scan fills: job facts with waiter liveness, run
// facts with lifecycle and attestation, live runs joining Busy, and the
// attestation freshness/identity rules.
func TestMonitorFacts(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	prober := scanProber{
		verdicts: map[int64]identity.Liveness{601: identity.Alive, 602: identity.Alive},
		starts:   map[int64]int64{601: 7000, 602: 7500},
	}
	nonce := strings.Repeat("cd", 16)
	writeFile(t, root, "artifacts/agents/jobs/j1.json",
		`{"jobId":"j1","role":"impl","runtime":"codex","status":"running","mainId":"main-x","startedAt":"2026-08-15T10:00:00Z"}`)
	writeFile(t, root, "artifacts/agents/runs/live-run.json",
		`{"schemaVersion":1,"runId":"live-run","kind":"suite","display":"suite work","custody":"wrapped",`+
			`"generation":1,"pid":601,"pidStartedAt":7000,"pgid":601,"launchNonce":"`+nonce+`",`+
			`"log":"/tmp/x.log","startedAt":"2026-08-15T10:00:00Z","mainId":"main-x","sessionId":"s","goalId":"",`+
			`"staleAfterMin":30,"windDownMin":10,"evidence":{"mode":"exit-sidecar"},`+
			`"expect":{"green":"gg","red":"rr","hung":"hh","unknown":"uu"},"status":"running","acked":false}`)
	writeFile(t, root, "artifacts/agents/runs/broken-run.json", "{torn")
	// A fresh attestation by the ARMED live watcher covering the run's
	// triple — the armed state supplies the identity and the interval.
	writeFile(t, root, "artifacts/agents/supervision/state.json",
		`{"schemaVersion":1,"components":{"watcher":{"pid":602,"pidStartedAt":7500,"instanceTag":"w"}},"intervalSec":300}`)
	writeFile(t, root, "artifacts/agents/supervision/runs-pass.json",
		`{"completedAt":"`+now.Format("2006-01-02T15:04:05Z")+`",`+
			`"watcherPid":602,"watcherStart":7500,`+
			`"scannedRuns":[{"id":"live-run","generation":1,"launchNonce":"`+nonce+`"}]}`)

	scan := scanWithProberAtWithoutGoal(t, root, prober, now)
	if len(scan.Jobs) != 1 || scan.Jobs[0].MainId != "main-x" || scan.Jobs[0].WaiterLive {
		t.Fatalf("job facts wrong: %+v", scan.Jobs)
	}
	if len(scan.Runs) != 1 {
		t.Fatalf("run facts wrong: %+v", scan.Runs)
	}
	fact := scan.Runs[0]
	if !fact.Supervised || fact.ProbeState != "alive" || fact.ExpectGreen != "gg" {
		t.Fatalf("run fact fields wrong: %+v", fact)
	}
	busyHasRun := false
	for _, item := range scan.Busy {
		if item.Kind == "run" && strings.Contains(item.Detail, "live-run") {
			busyHasRun = true
		}
	}
	if !busyHasRun {
		t.Fatal("live run missing from Busy")
	}
	if len(scan.RunUnreadable) != 1 || !strings.Contains(scan.RunUnreadable[0], "broken-run") {
		t.Fatalf("torn run record did not surface: %v", scan.RunUnreadable)
	}

	// A FUTURE-stamped attestation supervises nothing.
	writeFile(t, root, "artifacts/agents/supervision/runs-pass.json",
		`{"completedAt":"`+now.Add(time.Hour).Format("2006-01-02T15:04:05Z")+`",`+
			`"watcherPid":602,"watcherStart":7500,`+
			`"scannedRuns":[{"id":"live-run","generation":1,"launchNonce":"`+nonce+`"}]}`)
	scan = scanWithProberAtWithoutGoal(t, root, prober, now)
	if scan.Runs[0].Supervised {
		t.Fatal("a future-stamped attestation was believed")
	}

	// An attestation older than TWICE the armed interval supervises
	// nothing — the bound is the loaded interval, not a constant.
	writeFile(t, root, "artifacts/agents/supervision/state.json",
		`{"schemaVersion":1,"components":{"watcher":{"pid":602,"pidStartedAt":7500,"instanceTag":"w"}},"intervalSec":1}`)
	writeFile(t, root, "artifacts/agents/supervision/runs-pass.json",
		`{"completedAt":"`+now.Add(-time.Minute).Format("2006-01-02T15:04:05Z")+`",`+
			`"watcherPid":602,"watcherStart":7500,`+
			`"scannedRuns":[{"id":"live-run","generation":1,"launchNonce":"`+nonce+`"}]}`)
	scan = scanWithProberAtWithoutGoal(t, root, prober, now)
	if scan.Runs[0].Supervised {
		t.Fatal("an attestation beyond 2x the armed interval was believed")
	}

	// An attestation from someone OTHER than the armed watcher supervises
	// nothing, however fresh and alive.
	writeFile(t, root, "artifacts/agents/supervision/state.json",
		`{"schemaVersion":1,"components":{"watcher":{"pid":777,"pidStartedAt":1,"instanceTag":"w"}},"intervalSec":300}`)
	writeFile(t, root, "artifacts/agents/supervision/runs-pass.json",
		`{"completedAt":"`+now.Format("2006-01-02T15:04:05Z")+`",`+
			`"watcherPid":602,"watcherStart":7500,`+
			`"scannedRuns":[{"id":"live-run","generation":1,"launchNonce":"`+nonce+`"}]}`)
	scan = scanWithProberAtWithoutGoal(t, root, prober, now)
	if scan.Runs[0].Supervised {
		t.Fatal("a non-armed watcher's attestation was believed")
	}
	writeFile(t, root, "artifacts/agents/supervision/state.json",
		`{"schemaVersion":1,"components":{"watcher":{"pid":602,"pidStartedAt":7500,"instanceTag":"w"}},"intervalSec":300}`)

	// A DEAD watcher's attestation supervises nothing.
	writeFile(t, root, "artifacts/agents/supervision/runs-pass.json",
		`{"completedAt":"`+now.Format("2006-01-02T15:04:05Z")+`",`+
			`"watcherPid":602,"watcherStart":7500,`+
			`"scannedRuns":[{"id":"live-run","generation":1,"launchNonce":"`+nonce+`"}]}`)
	prober.verdicts[602] = identity.Dead
	scan = scanWithProberAtWithoutGoal(t, root, prober, now)
	if scan.Runs[0].Supervised {
		t.Fatal("a dead watcher's attestation was believed")
	}
}
