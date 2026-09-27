package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	metarun "github.com/widoriezebos/agentic-tools/metasystem/internal/run"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	usagecore "github.com/widoriezebos/agentic-tools/metasystem/internal/usage"
)

// supAWriteJob writes one delegate job record in the shape the wait source
// reads.
func supAWriteJob(t *testing.T, root, job, status string) {
	t.Helper()
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		t.Fatal(err)
	}
	record := `{"jobId":"` + job + `","operationId":"reserve-` + job + `","round":1,"status":"` + status + `","startedAt":"2026-09-13T12:00:00Z"}`
	if err := os.WriteFile(filepath.Join(jobs, job+".json"), []byte(record), 0o600); err != nil {
		t.Fatal(err)
	}
}

// Ported from supervision-fixtures.sh wait-restart. The bed named
// TestWaitRestartRecoveryReplay, which de01a54b2 renamed to
// TestSuccessorRefusesWrongSessionRow, so the bed selected nothing in
// internal/run and its installed leg (gated on METASYSTEM_WAIT_BINARY, with
// a sleep poll for the pending row) had not run since. Here the package's
// TestMain always supplies the source-built candidate, and the waiter's own
// registration descriptor says when the row is pending: an installed waiter
// killed while pending leaves a durable row that an installed resume
// recovers to the source's terminal result.
func TestSupervisionBedAWaitRestartKilledWaiterResumesFromItsRow(t *testing.T) {
	t.Parallel()
	candidate := os.Getenv("METASYSTEM_WAIT_BINARY")
	if candidate == "" {
		t.Fatal("the package TestMain did not supply METASYSTEM_WAIT_BINARY")
	}
	binary := testutil.InstalledWaitBinary(t, candidate)
	root := t.TempDir()
	self := int64(os.Getpid())
	exact, state, err := (identity.KernelProber{}).Probe(self)
	if err != nil || state != identity.Alive {
		t.Fatalf("current process identity=%+v state=%s err=%v", exact, state, err)
	}
	if _, err := lease.AnnounceWithPair(root, "wait-restart-session", self, exact.StartedAt.Unix(), exact.StartTicks, exact.BootID, "wait-restart-test", "fake", "wait-restart-lineage"); err != nil {
		t.Fatal(err)
	}
	supAWriteJob(t, root, "job-restart", "running")

	readyRead, readyWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	fixture := testutil.Fixture(t)
	waiter := exec.Command(binary, "internal", "wait", "--root", root, "--job", "job-restart", "--timeout", "1m", "--json")
	waiter.Env = fixture.Env(append(os.Environ(), waitRegisteredFDEnvironment+"=3"))
	waiter.ExtraFiles = []*os.File{readyWrite}
	var waiterOutput strings.Builder
	waiter.Stdout, waiter.Stderr = &waiterOutput, &waiterOutput
	if err := waiter.Start(); err != nil {
		_ = readyRead.Close()
		_ = readyWrite.Close()
		t.Fatal(err)
	}
	_ = readyWrite.Close()
	fixture.Record(waiter.Process.Pid)
	done := make(chan error, 1)
	go func() { done <- waiter.Wait() }()
	joined := false
	t.Cleanup(func() {
		if !joined {
			_ = waiter.Process.Kill()
			<-done
		}
	})
	line, readErr := bufio.NewReader(readyRead).ReadString('\n')
	_ = readyRead.Close()
	waitID := strings.TrimSpace(line)
	if readErr != nil || !metarun.ValidWaitID(waitID) {
		_ = waiter.Process.Kill()
		joined = true
		waitErr := <-done
		t.Fatalf("installed waiter registration signal=%q err=%v exit=%v output=%s", line, readErr, waitErr, waiterOutput.String())
	}
	row, _, err := metarun.FindWaiterByID(root, waitID)
	if err != nil || row.State != "pending" || row.Kind != "job" || row.TargetID != "job-restart" || row.Pid != int64(waiter.Process.Pid) {
		t.Fatalf("registered row=%+v err=%v", row, err)
	}
	if err := waiter.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	joined = true
	var exit *exec.ExitError
	if waitErr := <-done; !errors.As(waitErr, &exit) {
		t.Fatalf("killed installed waiter did not end by its signal: %v output=%s", waitErr, waiterOutput.String())
	}
	if orphan, _, err := metarun.FindWaiterByID(root, waitID); err != nil || orphan.State != "pending" {
		t.Fatalf("the killed waiter's row did not stay pending for recovery: row=%+v err=%v", orphan, err)
	}

	supAWriteJob(t, root, "job-restart", "completed")
	resume := exec.Command(binary, "internal", "wait", "--root", root, "--resume", waitID, "--json")
	resume.Env = fixture.Env(os.Environ())
	resumed, err := resume.CombinedOutput()
	if err != nil {
		t.Fatalf("installed resume did not recover from the row: err=%v output=%s", err, resumed)
	}
	var result metarun.WaitResult
	if err := json.Unmarshal(resumed, &result); err != nil || result.ExitCode != metarun.ExitGreen || result.State != "ready" {
		t.Fatalf("installed resume result=%+v err=%v output=%s", result, err, resumed)
	}
	final, _, err := metarun.FindWaiterByID(root, result.WaitID)
	if err != nil || final.State == "pending" {
		t.Fatalf("the recovered wait stayed pending: row=%+v err=%v", final, err)
	}
}

// supAMeasuredWait drives one wait through the production wait owner in this
// process, on one virtual clock, with the production source observers and
// event emitter. publish runs once, after the wait is registered and has read
// its source pending; onSleep runs once, at the wait's first poll pause.
type supAMeasuredWait struct {
	selector metarun.WaitSelector
	hinted   bool
	publish  func()
	onSleep  func()
}

func supAForbidSleep(t *testing.T) func() {
	return func() { t.Error("the hinted wait fell back to its poll pause") }
}

func supARunMeasuredWait(t *testing.T, root string, clock *virtualWaitClock, wait supAMeasuredWait) metarun.WaitResult {
	t.Helper()
	registered, published, slept := false, false, false
	options := metarun.WaitOptions{
		Runtime: "fake",
		Observe: func(ctx context.Context, selected metarun.WaitSelector, pinned metarun.WaiterTarget, lastTip string) (metarun.SourceObservation, error) {
			var observation metarun.SourceObservation
			var err error
			switch selected.Kind {
			case "job":
				observation, err = dispatchcore.ObserveJob(ctx, root, selected.TargetID, pinned, lastTip)
			case "run":
				observation, err = (&metarun.Store{Root: root}).ObserveRun(ctx, selected, pinned, lastTip)
			default:
				t.Fatalf("unexpected wait source %q", selected.Kind)
			}
			if err == nil && observation.Pending && registered && !published && wait.publish != nil {
				published = true
				wait.publish()
			}
			return observation, err
		},
		Deliver: func(context.Context, string, string, time.Time, string) (string, bool, error) {
			return "blocking", false, nil
		},
		EmitEvent: func(root, event, summary string, fields map[string]string) error {
			if event == "wait-registered" {
				registered = true
			}
			return emitWaitCommandEvent(root, event, summary, fields)
		},
	}
	if wait.hinted {
		options.OpenHintReceiver = metarun.OpenFIFOHintReceiver
	}
	clock.apply(&options)
	virtualSleep := options.Sleep
	options.Sleep = func(ctx context.Context, duration time.Duration) error {
		if !slept && wait.onSleep != nil {
			slept = true
			wait.onSleep()
		}
		return virtualSleep(ctx, duration)
	}
	owner := metarun.Caller{Class: lease.ClassMain, MainId: "main-wait-measure", OwnerLineage: "lineage-wait-measure", SessionId: "session-wait-measure"}
	result := (&metarun.Store{Root: root}).Wait(context.Background(), metarun.WaitRequest{
		Selector: wait.selector, Owner: owner, RuntimeSession: owner.SessionId, Timeout: 45 * time.Second,
	}, options)
	if result.ExitCode != metarun.ExitGreen || result.State != "ready" {
		t.Fatalf("%s wait did not return green: %+v", wait.selector.TargetID, result)
	}
	if wait.publish != nil && !published {
		t.Fatalf("%s wait never read its source pending after registration", wait.selector.TargetID)
	}
	if wait.onSleep != nil && !wait.hinted && !slept {
		t.Fatalf("%s wait returned without a poll pause", wait.selector.TargetID)
	}
	return result
}

const supARunRecord = `{"schemaVersion":1,"runId":"run-unpublished","kind":"suite","display":"unpublished run","custody":"wrapped","generation":1,"fenceGeneration":1,"pid":null,"pidStartedAt":null,"pgid":null,"launchNonce":"0123456789abcdef0123456789abcdef","log":"run.log","startedAt":"2026-09-16T10:00:00Z","mainId":null,"ownerLineage":null,"claimEpoch":null,"sessionId":"","goalId":"","staleAfterMin":30,"hungSince":null,"windDownMin":2,"evidence":{"mode":"exit-sidecar"},"expect":{},%s}`

// Ported from supervision-fixtures.sh wait-measure-fake (its job and run
// legs; the landing leg belongs to land.sh). A job wait whose publication
// hint reaches it through its FIFO, a job wait that has no hint and returns on
// its poll, and a run wait whose source publishes no wait-published event at
// all are each measured from the events and rows the waits themselves wrote:
// every target is sampled, nothing is unavailable, the report stays
// unproven, and the unhinted bracket is wider than the hinted one.
func TestSupervisionBedAWaitMeasureJoinsLiveWaits(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	clock := newVirtualWaitClock(time.Date(2026, 9, 16, 10, 0, 0, 0, time.UTC))
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeJob := func(job, status string) {
		t.Helper()
		record := `{"jobId":"` + job + `","operationId":"op-` + job + `","round":1,"status":"` + status + `","startedAt":"2026-09-16T10:00:00Z"}`
		if status == "completed" {
			record = strings.TrimSuffix(record, "}") + `,"endedAt":"2026-09-16T10:00:01Z"}`
		}
		if err := os.WriteFile(filepath.Join(jobs, job+".json"), []byte(record), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	// Store.Wait creates the hint FIFO before its registration creates the
	// waiters directory, so the first wait in a fresh state root registers
	// without the accelerator; the bed's control wait ran in that state and
	// never proved hint delivery. This test gives the control its directory.
	if err := os.MkdirAll(metarun.WaitersDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	writeJob("job-control", "running")
	supARunMeasuredWait(t, root, clock, supAMeasuredWait{
		selector: metarun.WaitSelector{Kind: "job", TargetID: "job-control"},
		hinted:   true,
		publish: func() {
			bootID, began, err := clock.BootClock()
			if err != nil {
				t.Fatal(err)
			}
			writeJob("job-control", "completed")
			delivery, err := metarun.NotifyWaiters(root, metarun.WaitHint{
				Kind: "job", TargetID: "job-control", PublicationID: "job:job-control:op-job-control:r1:2026-09-16T10:00:00Z:completed",
				BeganBootNanos: began.Nanoseconds(), BeganBootID: bootID,
			}, clock.BootClock)
			if err != nil || delivery.Matched != 1 || delivery.Delivered != 1 {
				t.Fatalf("the control publication did not reach its waiter: %+v %v", delivery, err)
			}
		},
		onSleep: supAForbidSleep(t),
	})

	writeJob("job-unhinted", "running")
	supARunMeasuredWait(t, root, clock, supAMeasuredWait{
		selector: metarun.WaitSelector{Kind: "job", TargetID: "job-unhinted"},
		onSleep:  func() { writeJob("job-unhinted", "completed") },
	})

	runs := filepath.Join(root, "artifacts", "agents", "runs")
	if err := os.MkdirAll(runs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeRun := func(tail string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(runs, "run-unpublished.json"), []byte(fmt.Sprintf(supARunRecord, tail)), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	writeRun(`"status":"launching","provisionalVerdict":null,"terminalSeq":null,"acked":false,"error":null,"exitCode":null,"endedAt":null`)
	supARunMeasuredWait(t, root, clock, supAMeasuredWait{
		selector: metarun.WaitSelector{Kind: "run", TargetID: "run-unpublished"},
		onSleep: func() {
			writeRun(`"status":"green","provisionalVerdict":null,"terminalSeq":1,"acked":false,"error":null,"exitCode":0,"endedAt":"2026-09-16T10:00:01Z"`)
		},
	})

	measurement, err := usagecore.MeasureWaits(root, usagecore.WaitMeasureOptions{})
	if err != nil {
		t.Fatal(err)
	}
	report := usagecore.FormatWaitMeasurement(measurement)
	if measurement.Verdict != "unproven" || !strings.Contains(report, "wait measurement verdict=unproven") {
		t.Fatalf("the measurement did not remain unproven:\n%s", report)
	}
	if len(measurement.Runtimes) != 1 || measurement.Runtimes[0].Runtime != "fake" || measurement.Runtimes[0].Counts["unavailable"] != 0 || !strings.Contains(report, "unavailable=0") {
		t.Fatalf("the measurement produced an unavailable sample:\n%s", report)
	}
	samples := map[string]usagecore.WaitMeasureSample{}
	for _, sample := range measurement.Runtimes[0].Samples {
		samples[sample.TargetID] = sample
	}
	for _, target := range []string{"job-control", "job-unhinted", "run-unpublished"} {
		if _, ok := samples[target]; !ok || !strings.Contains(report, `"targetId":"`+target+`"`) {
			t.Fatalf("the measurement omitted %s:\n%s", target, report)
		}
	}
	control, unhinted, run := samples["job-control"], samples["job-unhinted"], samples["run-unpublished"]
	if control.LowerEdge != "publication-began" || control.LooseEdgeReason != "" {
		t.Errorf("the hinted control did not join its publication: %+v", control)
	}
	if unhinted.LooseEdgeReason != "no-matching-publication" || run.LooseEdgeReason != "no-matching-publication" {
		t.Errorf("an unpublished wait joined a publication: unhinted=%+v run=%+v", unhinted, run)
	}
	if unhinted.UncertaintyNanos <= control.UncertaintyNanos {
		t.Errorf("unhinted bracket was not wider: control=%d unhinted=%d\n%s", control.UncertaintyNanos, unhinted.UncertaintyNanos, report)
	}
}
