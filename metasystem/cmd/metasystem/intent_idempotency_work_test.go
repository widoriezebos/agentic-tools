package main

// Idempotency rows of the objects work, design and test (unit U-idem,
// R-129-ui). Each stateful witness runs the action twice through the public
// router and asserts the repeat exits 0 as unchanged and records nothing:
// no owner call, no goal publication, no changed or added file.

import (
	"bytes"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

func init() {
	for _, name := range []string{"work status", "work wait", "design show", "design list", "test wait", "test plan", "test list", "test status"} {
		registerIdempotency(name, idemRead, "reads or waits on recorded state and changes nothing", nil)
	}
	registerIdempotency("work review", idemCreation,
		"each examination is a new review round of its subject; the same request rejoins the round it started (a repeat reads its progress and findings), and a closed chain's repeated close is unchanged", nil)
	registerIdempotency("work revise", idemCreation,
		"each correction is a new attempt of the work; the request (work, N and the brief's bytes) is kept first, so the same request rejoins that attempt and never spends another", nil)
	registerIdempotency("design write", idemCreation,
		"each request is a new author attempt on the design document; the same request reports the same attempt (rejoined) without a launch, and --after N asks for one new attempt", nil)
	registerIdempotency("design review", idemCreation,
		"each critique is a new examination round of the design; a closed chain's repeated close is unchanged", nil)
	registerIdempotency("test run", idemCreation,
		"each call runs the risk-selected tests as a new proof attempt of the checkout as it is now; test status answers whether retained proof already covers a tree without running anything", nil)

	registerIdempotency("work brief", idemStateful, "the same scaffold already at --out is unchanged; different content there is refused and kept", witnessWorkBriefRepeat)
	registerIdempotency("work build", idemStateful, "the same goal, work and request reach the same attempt again and launch nothing", witnessWorkBuildRepeat)
	registerIdempotency("work stop", idemStateful, "a job that already ended is already stopped; a goal's stop that already completed is left as it is (TestWorkStopGoalCompletesItsRecordedStop)", witnessWorkStopRepeat)
	registerIdempotency("work rebase", idemStateful, "a branch already on main carries nothing twice and writes nothing", witnessWorkRebaseRepeat)
	registerIdempotency("work finish", idemStateful, "an already closed chain is unchanged", witnessWorkFinishRepeat)
	registerIdempotency("work land", idemStateful, "an already landed goal names its landing and pushes nothing", witnessWorkLandRepeat)
	registerIdempotency("design stop", idemStateful, "an attempt that already ended is already stopped", witnessDesignStopRepeat)
}

// workIdemSnapshot is every file under root with its bytes, ignoring lock
// files that a read may open.
func workIdemSnapshot(t *testing.T, root string) map[string][]byte {
	t.Helper()
	files := map[string][]byte{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || strings.HasSuffix(path, ".lock") {
			return err
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		files[path] = data
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func workIdemSameFiles(t *testing.T, label string, before, after map[string][]byte) {
	t.Helper()
	if !maps.EqualFunc(before, after, bytes.Equal) {
		var changed []string
		for path, data := range after {
			if old, found := before[path]; !found || !bytes.Equal(old, data) {
				changed = append(changed, path)
			}
		}
		for path := range before {
			if _, found := after[path]; !found {
				changed = append(changed, path+" (removed)")
			}
		}
		slices.Sort(changed)
		t.Fatalf("%s: the repeat changed files: %v", label, changed)
	}
}

func witnessWorkBriefRepeat(t *testing.T) {
	bed := newWorkBed(t)
	code, first, _ := bed.work("work", "brief", bed.id, "--out", "brief.md")
	if code != 0 || first.Outcome != intentConfirmed {
		t.Fatalf("first brief: code=%d %+v", code, first)
	}
	publications, files := bed.publications(), workIdemSnapshot(t, bed.root())
	code, again, _ := bed.work("work", "brief", bed.id, "--out", "brief.md")
	if code != 0 || again.Outcome != intentUnchanged || bed.publications() != publications {
		t.Fatalf("repeated brief: code=%d %+v", code, again)
	}
	workIdemSameFiles(t, "work brief", files, workIdemSnapshot(t, bed.root()))
}

func witnessWorkBuildRepeat(t *testing.T) {
	bed := newWorkBed(t)
	brief := bed.brief("a.md", "Build part a.\n\nMaximum reader tool calls: 5\n")
	args := append([]string{"work", "build", bed.id, "--work", "a", "--brief", brief, "--lines", "5", "--check"}, workArgv...)
	code, first, _ := bed.work(args...)
	if code != 0 || first.Outcome != intentConfirmed {
		t.Fatalf("first build: code=%d %+v", code, first)
	}
	launches, runs, publications := len(bed.starter.launched()), bed.runDirectories(), bed.publications()
	code, again, _ := bed.work(args...)
	if code != 0 || resultData(t, again)["run"] != resultData(t, first)["run"] || len(bed.starter.launched()) != launches ||
		!slices.Equal(bed.runDirectories(), runs) || bed.publications() != publications {
		t.Fatalf("repeated build: code=%d %+v launches=%d->%d", code, again, launches, len(bed.starter.launched()))
	}
}

func witnessWorkStopRepeat(t *testing.T) {
	b := newReferenceBed(t)
	b.dispatchJob("job-r", "running")
	owners := b.owners()
	var cancelled []string
	owners.processes.cancelDispatch = func(_, job string) (map[string]any, int, error) {
		cancelled = append(cancelled, job)
		// The owner's recorded transition: the job ends cancelled.
		path := filepath.Join(b.root(), "artifacts", "agents", "jobs", job+".json")
		if err := os.WriteFile(path, []byte(`{"jobId":"`+job+`","status":"cancelled","role":"implementer","endedAt":"2026-09-25T12:00:00Z"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		return map[string]any{"outcome": "CANCELLED", "headline": "cancelled", "jobId": job}, 0, nil
	}
	if code, first := b.runJSON(owners, "work", "stop", "j2:job-r"); code != 0 || first.Outcome != intentConfirmed || len(cancelled) != 1 {
		t.Fatalf("first stop: code=%d %+v", code, first)
	}
	publications, files := b.publications(), workIdemSnapshot(t, b.root())
	code, again := b.runJSON(owners, "work", "stop", "j2:job-r")
	if code != 0 || again.Outcome != intentUnchanged || len(cancelled) != 1 || b.publications() != publications ||
		again.Summary != "j2:job-r is already stopped: cancelled (at 2026-09-25T12:00:00Z)" {
		t.Fatalf("repeated stop: code=%d %+v cancels=%v", code, again, cancelled)
	}
	workIdemSameFiles(t, "work stop", files, workIdemSnapshot(t, b.root()))
}

// newWorkIdemDeliveryBed is the delivery bed without a Git repository: the
// close and hand-landing paths resolve the checkout through the resolver's
// injected top alone.
func newWorkIdemDeliveryBed(t *testing.T) *deliveryBed {
	t.Helper()
	bed := &deliveryBed{intentBed: newIntentBed(t, false, nil)}
	layout, err := stateroot.NewResolver(fakeTop(bed.root()), noExecutable).ResolveLayout(bed.root())
	if err != nil {
		t.Fatal(err)
	}
	bed.install = layout.InstallationRoot
	bed.owners = &intentDeliveryOwners{
		// The landing gate has its own beds (intent_landing_gate_test.go);
		// these witnesses prove the repeat of a landing the gate admitted.
		landingGate:  func(*intentInvocation, string, string) (string, error) { return "the bed's landing", nil },
		recordLanded: func(*intentInvocation, string) error { return nil },
		recordWriter: humanRecordWriter,
		process: func(process intentProcess) intentProcessResult {
			bed.calls = append(bed.calls, process.argv)
			if bed.handler == nil {
				t.Fatalf("unexpected owner process %v", process.argv)
			}
			return bed.handler(process)
		},
		// The close owner is the delegate lifecycle's close command (batch
		// 2 runs it in process); the bed sees it as one owner process named
		// close-owner, as the delivery bed does.
		closeOwner: func(root string, args []string) intentProcessResult {
			return bed.owners.process(intentProcess{argv: append([]string{"close-owner"}, args...), dir: root})
		},
		executable: func() (string, error) { return "/fake/bin/metasystem", nil },
		now:        func() time.Time { return time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC) },
		laneRoot:   func(string, time.Time) (string, bool, error) { return "", false, nil },
	}
	bed.owners.calls = processBackedOwnerCalls(func() (string, error) { return bed.owners.executable() },
		func(process intentProcess) intentProcessResult { return bed.owners.process(process) })
	return bed
}

func witnessWorkFinishRepeat(t *testing.T) {
	b := newWorkIdemDeliveryBed(t)
	b.writeJob(map[string]any{"jobId": "inv1", "role": "investigator", "status": "completed", "round": 1, "parentJob": nil})
	b.owners.rebind = func(string, string) (map[string]string, error) { return map[string]string{}, nil }
	b.handler = func(intentProcess) intentProcessResult {
		record := b.job("inv1")
		record["chainClosed"] = true
		b.writeJob(record)
		return intentProcessResult{}
	}
	if code, first := b.do("work", "finish", "j2:inv1"); code != 0 || first.Outcome != intentConfirmed || len(b.calls) != 1 {
		t.Fatalf("first finish: code=%d %+v", code, first)
	}
	publications, files := b.publications(), workIdemSnapshot(t, b.root())
	code, again := b.do("work", "finish", "j2:inv1")
	if code != 0 || again.Outcome != intentUnchanged || len(b.calls) != 1 || b.publications() != publications {
		t.Fatalf("repeated finish: code=%d %+v calls=%v", code, again, b.calls)
	}
	workIdemSameFiles(t, "work finish", files, workIdemSnapshot(t, b.root()))
}

func witnessWorkLandRepeat(t *testing.T) {
	b := newWorkIdemDeliveryBed(t)
	owners := &landingOwners{status: readBranch(2, "reader-record", "reader-record")}
	owners.install(b)
	code, first := b.do("work", "land", "standing-validation")
	if code != 0 || first.Outcome != intentConfirmed || len(owners.pushes) != 1 {
		t.Fatalf("first land: code=%d %+v", code, first)
	}
	candidates, preps, pushes, sweeps, calls := owners.candidates, len(owners.preps), len(owners.pushes), owners.sweeps, len(b.calls)
	publications, files := b.publications(), workIdemSnapshot(t, b.root())
	code, again := b.do("work", "land", "standing-validation")
	if code != 0 || again.Outcome != intentUnchanged || !strings.Contains(again.Summary, "land1") || owners.candidates != candidates ||
		len(owners.preps) != preps || len(owners.pushes) != pushes || owners.sweeps != sweeps || len(b.calls) != calls || b.publications() != publications {
		t.Fatalf("repeated land: code=%d %+v", code, again)
	}
	workIdemSameFiles(t, "work land", files, workIdemSnapshot(t, b.root()))
}

func witnessDesignStopRepeat(t *testing.T) {
	bed := newWorkBed(t)
	bed.starter.author = fakeAuthor(t, "The reader design.\n")
	if code, written := designRun(t, bed, "design", "write", bed.id, "--brief", bed.brief("design-request.md", "Design the reader.\n")); code != 0 || written.Outcome != intentConfirmed {
		t.Fatalf("design write: code=%d %+v", code, written)
	}
	launchRoot := bed.manager.Store.Root
	for round := 1; round <= 2; round++ {
		publications, files := bed.publications(), workIdemSnapshot(t, launchRoot)
		code, stopped := designRun(t, bed, "design", "stop", bed.id)
		if code != 0 || stopped.Outcome != intentUnchanged || !strings.Contains(stopped.Summary, "design attempt 1 is already stopped") || bed.publications() != publications {
			t.Fatalf("stop %d of an ended attempt: code=%d %+v", round, code, stopped)
		}
		workIdemSameFiles(t, "design stop", files, workIdemSnapshot(t, launchRoot))
	}
}
