package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/events"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/report"
	metarun "github.com/widoriezebos/agentic-tools/metasystem/internal/run"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	usagecore "github.com/widoriezebos/agentic-tools/metasystem/internal/usage"
)

type waitCommandProber struct {
	pid   int64
	live  bool
	start int64
}

func announcedMainID(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var announcement lease.Announcement
	if err := json.Unmarshal(data, &announcement); err != nil || announcement.MainId == "" {
		t.Fatalf("announcement %s: %+v %v", path, announcement, err)
	}
	return announcement.MainId
}

func (p *waitCommandProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	if pid != p.pid || !p.live {
		return identity.Exact{}, identity.Dead, nil
	}
	if p.start == 0 {
		return identity.Exact{}, identity.Unknown, errors.New("fixture start time is unavailable")
	}
	return identity.Exact{Pid: pid, StartedAt: time.Unix(p.start, 0)}, identity.Alive, nil
}

func TestWaitVerbArgumentsAndResult(t *testing.T) {
	if code, _, problem := captureChannelOutput(t, func() int { return runWait(nil) }); code != metarun.ExitInvalidWait || !strings.Contains(problem, "exactly one") {
		t.Fatalf("missing selector code=%d stderr=%q", code, problem)
	}
	if code, _, problem := captureChannelOutput(t, func() int {
		return runWait([]string{"--job", "j", "--run", "r"})
	}); code != metarun.ExitInvalidWait || !strings.Contains(problem, "exactly one") {
		t.Fatalf("duplicate selector code=%d stderr=%q", code, problem)
	}
	if code, _, problem := captureChannelOutput(t, func() int {
		return runWait([]string{"--goal", "goal-a"})
	}); code != metarun.ExitInvalidWait || !strings.Contains(problem, "event must be") {
		t.Fatalf("incomplete goal selector code=%d stderr=%q", code, problem)
	}
	if code, _, problem := captureChannelOutput(t, func() int {
		return runWait([]string{"--resume", "not-a-wait-id"})
	}); code != metarun.ExitInvalidWait || !strings.Contains(problem, "identifier is invalid") {
		t.Fatalf("invalid resume identifier code=%d stderr=%q", code, problem)
	}
	result := metarun.WaitResult{SchemaVersion: 2, WaitID: strings.Repeat("a", 32), Selector: metarun.WaitSelector{Kind: "run", TargetID: "run-a"}, TargetIncarnation: metarun.WaiterTarget{Generation: 2, LaunchNonce: "n"}, ExitCode: metarun.ExitGreen, Reason: "run completed green", SourceOutcome: "green", SourceEvidence: "run:run-a:g2:n"}
	code, output, problem := captureChannelOutput(t, func() int { printWaitResult(result, true); return 0 })
	if code != 0 || problem != "" {
		t.Fatalf("JSON result code=%d stderr=%q", code, problem)
	}
	var decoded metarun.WaitResult
	if err := json.Unmarshal([]byte(output), &decoded); err != nil || decoded.WaitID != result.WaitID || decoded.TargetIncarnation.Generation != 2 {
		t.Fatalf("JSON result=%q decoded=%+v err=%v", output, decoded, err)
	}
	_, plain, _ := captureChannelOutput(t, func() int { printWaitResult(result, false); return 0 })
	if strings.Count(strings.TrimSpace(plain), "\n") != 0 || !strings.Contains(plain, "run-a") || !strings.Contains(plain, "run:run-a:g2:n") {
		t.Fatalf("plain result is not one complete line: %q", plain)
	}
}

func TestUnassociatedRegistrationRefusesNoRow(t *testing.T) {
	root := t.TempDir()
	self := int64(os.Getpid())
	exact, state, err := (identity.KernelProber{}).Probe(self)
	if err != nil || state != identity.Alive {
		t.Fatal(err)
	}
	announcement, err := lease.AnnounceWithPair(root, fmt.Sprintf("session-%d", self), self, exact.StartedAt.Unix(), exact.StartTicks, exact.BootID, "unassociated-wait", "fake", "lineage-wait")
	if err != nil {
		t.Fatal(err)
	}
	mainID := announcedMainID(t, announcement)
	jobDir := filepath.Join(root, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobDir, "job-unassociated.json"), []byte(`{"jobId":"job-unassociated","operationId":"reserve-a","round":1,"status":"completed","startedAt":"2026-09-15T10:00:00Z"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	originalPID, originalAdapter := waitCallerPID, waitAdapterPathForRuntime
	waitCallerPID = func() int64 { return self }
	waitAdapterPathForRuntime = func(string, string) (string, error) {
		return filepath.Abs(filepath.Join("..", "..", "scripts", "agents", "adapters", "fake.sh"))
	}
	t.Cleanup(func() { waitCallerPID, waitAdapterPathForRuntime = originalPID, originalAdapter })
	code, _, problem := captureChannelOutput(t, func() int { return runWait([]string{"--root", root, "--job", "job-unassociated"}) })
	rows, _ := filepath.Glob(filepath.Join(metarun.WaitersDir(root), "*.json"))
	if code != metarun.ExitWaiterBusy || !strings.Contains(problem, "authenticated runtime session") || len(rows) != 0 {
		t.Fatalf("unassociated registration code=%d stderr=%q rows=%v", code, problem, rows)
	}
	if err := lease.AssociateSession(root, mainID, "session-associated", "start", "clear"); err != nil {
		t.Fatal(err)
	}
	if code, _, problem = captureChannelOutput(t, func() int { return runWait([]string{"--root", root, "--job", "job-unassociated"}) }); code != 0 || problem != "" {
		t.Fatalf("associated registration code=%d stderr=%q", code, problem)
	}
}

func TestWaitClaimableFrontierIsAChange(t *testing.T) {
	baseline := []metarun.ClaimableGoal{{ID: "already-ready", Revision: 3}}
	if item, changed := metarun.ClaimableGoalChange(baseline, baseline); changed {
		t.Fatalf("an unchanged registration backlog woke the waiter: %+v", item)
	}
	if item, changed := metarun.ClaimableGoalChange(baseline, []metarun.ClaimableGoal{{ID: "already-ready", Revision: 4}}); !changed || item.ID != "already-ready" {
		t.Fatalf("a revision change was not actionable: %+v %t", item, changed)
	}
	if item, changed := metarun.ClaimableGoalChange(baseline, append(baseline, metarun.ClaimableGoal{ID: "newly-ready", Revision: 1})); !changed || item.ID != "newly-ready" {
		t.Fatalf("a newly claimable goal was not actionable: %+v %t", item, changed)
	}
}

func TestWaitProviderPollFailureStillReadsTheDurableSource(t *testing.T) {
	root := t.TempDir()
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobs, "job-a.json"), []byte(`{"jobId":"job-a","operationId":"reserve-a","round":1,"status":"running","startedAt":"2026-09-13T10:00:00Z"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	original := waitAdapterPathForRuntime
	waitAdapterPathForRuntime = func(string, string) (string, error) { return "/unused/fake-adapter", nil }
	t.Cleanup(func() { waitAdapterPathForRuntime = original })
	options, err := waitOptions(root, metarun.WaitSelector{Kind: "job", TargetID: "job-a"}, metarun.Caller{OwnerLineage: "lineage-a"}, "fake", func(context.Context) error {
		return errors.New("provider unavailable")
	})
	if err != nil {
		t.Fatal(err)
	}
	observation, err := options.Observe(context.Background(), metarun.WaitSelector{Kind: "job", TargetID: "job-a"}, metarun.WaiterTarget{}, "")
	if err != nil || !observation.Pending || observation.Incarnation.OperationID != "reserve-a" || !strings.Contains(observation.PollError, "provider unavailable") || observation.PollAt == "" {
		t.Fatalf("provider failure displaced the job observation: observation=%+v err=%v", observation, err)
	}
}

func TestWaitActionableCheckUsesNoGitAndHonorsItsContext(t *testing.T) {
	root := t.TempDir()
	writeWaitTestFile := func(path, body string, mode os.FileMode) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), mode); err != nil {
			t.Fatal(err)
		}
	}
	writeWaitTestFile(filepath.Join(root, "plans", "active.md"), "# P\n- Next step: Keep working.\n- Waiting on the human: none\n", 0o644)
	self := int64(os.Getpid())
	exact, state, err := (identity.KernelProber{}).Probe(self)
	if err != nil || state != identity.Alive {
		t.Fatalf("current process identity=%+v state=%s err=%v", exact, state, err)
	}
	announcementPath, err := lease.AnnounceWithPair(root, "action-session", self, exact.StartedAt.Unix(), exact.StartTicks, exact.BootID, "action-test", "fake", "action-lineage")
	if err != nil {
		t.Fatal(err)
	}
	mainID := announcedMainID(t, announcementPath)
	holder, err := lease.CurrentHolder(root)
	if err != nil {
		t.Fatal(err)
	}
	originalAdapter := waitAdapterPathForRuntime
	waitAdapterPathForRuntime = func(string, string) (string, error) { return "/unused/fake-adapter", nil }
	originalSignature := waitOpenWorkSignature
	originalCurrentHolder := waitCurrentHolder
	t.Cleanup(func() {
		waitAdapterPathForRuntime = originalAdapter
		waitOpenWorkSignature = originalSignature
		waitCurrentHolder = originalCurrentHolder
	})
	options, err := waitOptions(root, metarun.WaitSelector{Kind: "job", TargetID: "job-a"}, metarun.Caller{OwnerLineage: "action-lineage"}, "fake", nil)
	if err != nil {
		t.Fatal(err)
	}
	signature, err := report.OpenWorkSignature(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(t.TempDir(), "git-called")
	binDir := t.TempDir()
	writeWaitTestFile(filepath.Join(binDir, "git"), "#!/bin/sh\nprintf called >\"$WAIT_GIT_MARKER\"\nexit 99\n", 0o755)
	t.Setenv("PATH", binDir)
	t.Setenv("WAIT_GIT_MARKER", marker)
	row := metarun.Waiter{MainId: mainID, OwnerLineage: "action-lineage", ClaimEpoch: &holder.ClaimEpoch, OpenWorkSignature: signature}
	if reason, changed, actionErr := options.Actionable(context.Background(), row); actionErr != nil || changed || reason != "" {
		t.Fatalf("unchanged actionable check reason=%q changed=%t err=%v", reason, changed, actionErr)
	}
	if _, statErr := os.Stat(marker); !os.IsNotExist(statErr) {
		t.Fatalf("actionable check invoked Git: %v", statErr)
	}
	waitOpenWorkSignature = func(ctx context.Context, _ string) (string, error) {
		<-ctx.Done()
		return "", ctx.Err()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	started := time.Now()
	if _, _, actionErr := options.Actionable(ctx, row); !errors.Is(actionErr, context.DeadlineExceeded) || time.Since(started) > time.Second {
		t.Fatalf("slow actionable read err=%v elapsed=%s", actionErr, time.Since(started))
	}
	waitOpenWorkSignature = originalSignature
	waitCurrentHolder = func(ctx context.Context, _ string) (lease.CurrentHolderView, error) {
		<-ctx.Done()
		return lease.CurrentHolderView{}, ctx.Err()
	}
	ctx, cancel = context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	started = time.Now()
	if _, _, actionErr := options.Actionable(ctx, row); !errors.Is(actionErr, context.DeadlineExceeded) || time.Since(started) > time.Second {
		t.Fatalf("slow checkout-holder read err=%v elapsed=%s", actionErr, time.Since(started))
	}
}

func TestWaitPlainResumeRefusesChannelRegistration(t *testing.T) {
	root := t.TempDir()
	self := int64(os.Getpid())
	exact, state, err := (identity.KernelProber{}).Probe(self)
	if err != nil || state != identity.Alive {
		t.Fatalf("current process identity=%+v state=%s err=%v", exact, state, err)
	}
	announcementPath, err := lease.AnnounceWithPair(root, "channel-session", self, exact.StartedAt.Unix(), exact.StartTicks, exact.BootID, "channel-resume-test", "fake", "channel-lineage")
	if err != nil {
		t.Fatal(err)
	}
	mainID := announcedMainID(t, announcementPath)
	waitID := strings.Repeat("a", 32)
	row := metarun.Waiter{
		SchemaVersion: 2, WaitID: waitID, Nonce: strings.Repeat("b", 32), Kind: "goal", TargetID: "goal-a", OwnerDigest: metarun.OwnerDigest(mainID),
		MainId: mainID, OwnerLineage: "channel-lineage", State: "pending",
		Selector: metarun.WaitSelector{Kind: "goal", TargetID: "goal-a", GoalID: "goal-a", Event: "human-act", Verb: "answer", Question: "question-a", After: strings.Repeat("c", 40), Poll: "channel"},
	}
	if err := os.MkdirAll(metarun.WaitersDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(metarun.WaiterPath(root, row.Kind, row.TargetID, row.OwnerDigest), body, 0o600); err != nil {
		t.Fatal(err)
	}
	originalPID := waitCallerPID
	waitCallerPID = func() int64 { return self }
	t.Cleanup(func() { waitCallerPID = originalPID })
	code, _, problem := captureChannelOutput(t, func() int { return runWait([]string{"--root", root, "--resume", waitID}) })
	if code != metarun.ExitWaiterBusy || !strings.Contains(problem, "metasystem channel wait --resume "+waitID) {
		t.Fatalf("plain channel resume code=%d stderr=%q", code, problem)
	}
}

func TestWaitInstalledRunCommand(t *testing.T) {
	root := t.TempDir()
	self := int64(os.Getpid())
	exact, state, err := (identity.KernelProber{}).Probe(self)
	if err != nil || state != identity.Alive {
		t.Fatalf("current process identity=%+v state=%s err=%v", exact, state, err)
	}
	announcementPath, err := lease.AnnounceWithPair(root, "session-command", self, exact.StartedAt.Unix(), exact.StartTicks, exact.BootID, "wait-command-test", "fake", "lineage-command")
	if err != nil {
		t.Fatal(err)
	}
	mainID := announcedMainID(t, announcementPath)
	caller := metarun.Caller{Class: lease.ClassMain, MainId: mainID, OwnerLineage: "lineage-command", SessionId: "session-command"}
	prober := &waitCommandProber{pid: 912345, live: true, start: 5000}
	store := &metarun.Store{Root: root, Prober: prober, Now: func() time.Time { return time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC) }, Getpgid: func(pid int64) (int64, error) { return pid, nil }}
	nonce, err := store.Launch(caller, metarun.LaunchParams{Id: "run-command", Kind: "suite", Display: "run command fixture", Log: "artifacts/run-command.log", Expect: metarun.Expect{Green: "green", Red: "red", Hung: "hung", Unknown: "unknown"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Bind("run-command", nonce, prober.pid, prober.pid); err != nil {
		t.Fatal(err)
	}
	record, err := store.Read("run-command")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.WriteSidecar("run-command", record.Generation, record.LaunchNonce, 0); err != nil {
		t.Fatal(err)
	}
	prober.live = false
	if _, err := store.Assess("run-command"); err != nil {
		t.Fatal(err)
	}
	originalCallerPID := waitCallerPID
	waitCallerPID = func() int64 { return self }
	originalBootClock := waitBootClock
	waitBootClock = func() (string, time.Duration, error) { return "boot-command", time.Hour, nil }
	originalAdapterPath := waitAdapterPathForRuntime
	waitAdapterPathForRuntime = func(string, string) (string, error) {
		return filepath.Abs(filepath.Join("..", "..", "scripts", "agents", "adapters", "fake.sh"))
	}
	t.Cleanup(func() {
		waitCallerPID = originalCallerPID
		waitBootClock = originalBootClock
		waitAdapterPathForRuntime = originalAdapterPath
	})
	code, output, problem := 0, "", ""
	if binary := os.Getenv("METASYSTEM_WAIT_BINARY"); binary != "" {
		binary = testutil.InstalledWaitBinary(t, binary)
		cmd := exec.Command(binary, "wait", "--root", root, "--run", "run-command", "--timeout", "1m", "--json")
		data, commandErr := cmd.CombinedOutput()
		output = string(data)
		if commandErr != nil {
			problem = commandErr.Error()
			if exit, ok := commandErr.(*exec.ExitError); ok {
				code = exit.ExitCode()
			} else {
				code = -1
			}
		}
	} else {
		code, output, problem = captureChannelOutput(t, func() int {
			return runWait([]string{"--root", root, "--run", "run-command", "--timeout", "1m", "--json"})
		})
	}
	if code != metarun.ExitGreen || problem != "" {
		t.Fatalf("installed run wait code=%d stderr=%q output=%q", code, problem, output)
	}
	var result metarun.WaitResult
	if err := json.Unmarshal([]byte(output), &result); err != nil || result.ExitCode != metarun.ExitGreen || result.WaitID == "" {
		t.Fatalf("installed run wait result=%+v output=%q err=%v", result, output, err)
	}
	row, _, err := metarun.LoadWaiterByID(root, result.WaitID)
	if err != nil || row.Delivery != "blocking" || row.State != "ready" {
		t.Fatalf("installed run wait row=%+v err=%v", row, err)
	}
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobs, "job-command.json"), []byte(`{"jobId":"job-command","operationId":"reserve-command","round":2,"status":"completed","startedAt":"2026-09-13T10:00:00Z"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if binary := os.Getenv("METASYSTEM_WAIT_BINARY"); binary != "" {
		binary = testutil.InstalledWaitBinary(t, binary)
		cmd := exec.Command(binary, "wait", "--root", root, "--job", "job-command", "--timeout", "1m", "--json")
		data, commandErr := cmd.CombinedOutput()
		if commandErr != nil || !strings.Contains(string(data), `"exitCode":0`) {
			t.Fatalf("installed job wait output=%s err=%v", data, commandErr)
		}
		if err := os.MkdirAll(filepath.Join(root, "scripts", "agents"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("dispatch.cap-max=120\nmetasystem.runtimes=fake\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"coverage-ratchet.json", "coverage-ratchet-linux.json"} {
			if err := os.WriteFile(filepath.Join(root, "scripts", "agents", name), []byte(`{"floors":{"internal/proofrun":1},"exempt":{}}`), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		proofIdentity, err := proofrun.BuildProofIdentity(root, filepath.Join(root, "metasystem.conf"), "full", "wait-installed", []string{"gate"}, behaviorsurface.SupportedVersion)
		if err != nil {
			t.Fatal(err)
		}
		launcher, err := proofrun.CurrentProcessIdentity(nil)
		if err != nil {
			t.Fatal(err)
		}
		proofAttempt, _, err := proofrun.ReserveLocked(proofrun.AdmissionRequest{ControlRoot: root, ExecutionRoot: root, GoalID: "goal-a", GoalRevision: 1, AccountingRevision: 1, ReservedMinutes: 1, Identity: proofIdentity, Launcher: launcher, Now: time.Now().UTC()})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := proofrun.FinalizeAttempt(root, proofAttempt.AttemptID, proofrun.TerminalSuccess, 0, "installed proof terminal", nil, time.Now().UTC()); err != nil {
			t.Fatal(err)
		}
		cmd = exec.Command(binary, "wait", "--root", root, "--attempt", proofAttempt.AttemptID, "--timeout", "1m", "--json")
		data, commandErr = cmd.CombinedOutput()
		if commandErr != nil || !strings.Contains(string(data), `"exitCode":0`) {
			t.Fatalf("installed proof wait output=%s err=%v", data, commandErr)
		}
	} else {
		code, data, problem := captureChannelOutput(t, func() int {
			return runWait([]string{"--root", root, "--job", "job-command", "--timeout", "1m", "--json"})
		})
		if code != 0 || problem != "" || !strings.Contains(data, `"exitCode":0`) {
			t.Fatalf("job wait code=%d output=%s problem=%s", code, data, problem)
		}
	}
}

func TestWaitNotifyCommand(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(metarun.WaitersDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	waitID := strings.Repeat("a", 32)
	nonce := strings.Repeat("b", 32)
	rowPath := metarun.WaiterPath(root, "job", "notify-job", "owner")
	hintPath := rowPath + "." + nonce + ".hint"
	receiver, err := metarun.OpenFIFOHintReceiver(hintPath, waitID, nonce)
	if err != nil {
		t.Fatal(err)
	}
	defer receiver.Close()
	defer os.Remove(hintPath)
	row := metarun.Waiter{SchemaVersion: 2, WaitID: waitID, Nonce: nonce, Kind: "job", TargetID: "notify-job", OwnerDigest: "owner", State: "pending", Accelerator: "fifo", HintPath: hintPath}
	encoded, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rowPath, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	output, code := captureStdout(t, func() int {
		return runWait([]string{"notify", "--root", root, "--job", "notify-job"})
	})
	if code != 0 || !strings.Contains(output, "matched=1 delivered=1") {
		t.Fatalf("notify code=%d output=%q", code, output)
	}
	hinted, err := receiver.Wait(context.Background(), time.Second)
	if err != nil || !hinted {
		t.Fatalf("notify did not reach the private FIFO: hinted=%t err=%v", hinted, err)
	}
	if code := runWait([]string{"notify", "--job", "one", "--goal", "two"}); code != metarun.ExitInvalidWait {
		t.Fatalf("ambiguous notify exit=%d", code)
	}
}

func TestWaitMeasureVerb(t *testing.T) {
	emitReturn := func(root, id, runtimeName string, atEntry bool, previous, observed, returned int64) {
		t.Helper()
		fields := map[string]string{
			"waitId": id, "nonce": strings.Repeat("b", 32), "kind": "attempt", "targetId": id, "runtime": runtimeName, "mode": "register", "state": "ready",
			"sourceEvidence": "attempt:" + id + ":digest", "registeredAt": "2026-09-16T10:00:00Z", "returnedAt": "2026-09-16T10:00:01Z",
			"registeredBootNanos": "1", "prevObservedBootNanos": strconv.FormatInt(previous, 10), "observedBootNanos": strconv.FormatInt(observed, 10), "returnedBootNanos": strconv.FormatInt(returned, 10),
			"registeredBootId": "boot", "prevObservedBootId": "boot", "observedBootId": "boot", "returnedBootId": "boot", "atEntry": strconv.FormatBool(atEntry),
		}
		if err := (&events.Emitter{Component: "run", Pid: 30, PidStartedAt: 40}).EmitChecked(root, "wait-returned", "wait measure verb fixture", fields); err != nil {
			t.Fatal(err)
		}
	}
	makeRoot := func(atEntry bool, previous, observed, returned int64) string {
		root := t.TempDir()
		emitReturn(root, strings.Repeat("a", 32), "codex", atEntry, previous, observed, returned)
		return root
	}
	refuted := makeRoot(false, 2, 5, int64(70*time.Second))
	first, code := captureStdout(t, func() int { return runWait([]string{"measure", "--root", refuted, "--json"}) })
	second, again := captureStdout(t, func() int { return runWait([]string{"measure", "--root", refuted, "--json"}) })
	if code != 1 || again != 1 || first != second || !strings.Contains(first, `"verdict":"refuted"`) || strings.Contains(strings.ToLower(first), "pass") {
		t.Fatalf("refuted code=%d/%d output=%s", code, again, first)
	}
	if binary := os.Getenv("METASYSTEM_WAIT_BINARY"); binary != "" {
		command := exec.Command(binary, "wait", "measure", "--root", refuted, "--json")
		data, err := command.CombinedOutput()
		if err == nil || command.ProcessState.ExitCode() != 1 || string(data) != first {
			t.Fatalf("installed exit=%v output=%s", err, data)
		}
	}
	available := makeRoot(false, 10, 20, int64(time.Second))
	if output, code := captureStdout(t, func() int { return runWaitMeasure([]string{"--root", available, "--json"}) }); code != 0 || !strings.Contains(output, `"verdict":"unproven"`) {
		t.Fatalf("available exit=%d output=%s", code, output)
	}
	unavailable := makeRoot(true, 10, 20, int64(time.Second))
	if _, code := captureStdout(t, func() int { return runWaitMeasure([]string{"--root", unavailable}) }); code != 2 {
		t.Fatalf("unavailable exit=%d", code)
	}
	if code := runWaitMeasure([]string{"--since", "bad"}); code != metarun.ExitInvalidWait {
		t.Fatalf("invalid exit=%d", code)
	}

	grouped := t.TempDir()
	emitReturn(grouped, "codex-event", "codex", false, 10, 20, int64(time.Second))
	emitReturn(grouped, "fake-event", "fake", false, 10, 20, int64(time.Second))
	before, err := os.ReadFile(filepath.Join(grouped, "artifacts", "agents", "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	type answer struct {
		data string
		err  error
	}
	answers := make(chan answer, 2)
	for range 2 {
		go func() {
			measurement, measureErr := usagecore.MeasureWaits(grouped, usagecore.WaitMeasureOptions{})
			encoded, _ := json.Marshal(measurement)
			answers <- answer{data: string(encoded), err: measureErr}
		}()
	}
	one, two := <-answers, <-answers
	after, err := os.ReadFile(filepath.Join(grouped, "artifacts", "agents", "events.jsonl"))
	if err != nil || one.err != nil || two.err != nil || one.data != two.data || string(before) != string(after) || !strings.Contains(one.data, `"runtime":"codex"`) || !strings.Contains(one.data, `"runtime":"fake"`) {
		t.Fatalf("concurrent read one=%s/%v two=%s/%v unchanged=%t readErr=%v", one.data, one.err, two.data, two.err, string(before) == string(after), err)
	}

	duplicateRoot := t.TempDir()
	bootID, bootElapsed, err := identity.BootClock()
	if err != nil || bootElapsed < 5*time.Second {
		t.Fatalf("boot clock = %s/%s err=%v", bootID, bootElapsed, err)
	}
	publicationID := "job:job-duplicate:operation:r1:started:completed"
	jobReturn := map[string]string{
		"waitId": strings.Repeat("c", 32), "nonce": strings.Repeat("d", 32), "kind": "job", "targetId": "job-duplicate", "runtime": "codex", "mode": "register", "state": "ready",
		"sourceEvidence": "job:job-duplicate:operation:r1:started", "sourceOutcome": "completed", "registeredAt": "2026-09-16T10:00:00Z", "returnedAt": "2026-09-16T10:00:01Z",
		"registeredBootNanos": strconv.FormatInt((bootElapsed - 5*time.Second).Nanoseconds(), 10), "prevObservedBootNanos": strconv.FormatInt((bootElapsed - 4*time.Second).Nanoseconds(), 10),
		"observedBootNanos": strconv.FormatInt((bootElapsed - 2*time.Second).Nanoseconds(), 10), "returnedBootNanos": strconv.FormatInt((bootElapsed - time.Second).Nanoseconds(), 10),
		"registeredBootId": bootID, "prevObservedBootId": bootID, "observedBootId": bootID, "returnedBootId": bootID, "atEntry": "false",
	}
	if err := (&events.Emitter{Component: "run", Pid: 31, PidStartedAt: 41}).EmitChecked(duplicateRoot, "wait-returned", "duplicate publication fixture", jobReturn); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		args := []string{"--root", duplicateRoot, "--job", "job-duplicate", "--publication-id", publicationID, "--began-boot-nanos", strconv.FormatInt((bootElapsed - 3*time.Second).Nanoseconds(), 10), "--boot-id", bootID}
		if _, code := captureStdout(t, func() int { return runWaitNotify(args) }); code != 0 {
			t.Fatalf("duplicate notify exit=%d", code)
		}
	}
	stream, err := os.ReadFile(filepath.Join(duplicateRoot, "artifacts", "agents", "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	publicationCount := 0
	for _, line := range strings.Split(strings.TrimSpace(string(stream)), "\n") {
		var event map[string]any
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatal(err)
		}
		if event["event"] != "wait-published" {
			continue
		}
		began, beganErr := strconv.ParseInt(fmt.Sprint(event["beganBootNanos"]), 10, 64)
		published, publishedErr := strconv.ParseInt(fmt.Sprint(event["publishedBootNanos"]), 10, 64)
		if beganErr != nil || publishedErr != nil || began >= published {
			t.Fatalf("publication did not follow its began sample: began=%v/%v published=%v/%v", began, beganErr, published, publishedErr)
		}
		publicationCount++
	}
	if publicationCount != 2 {
		t.Fatalf("published event count=%d, want 2", publicationCount)
	}
	duplicate, err := usagecore.MeasureWaits(duplicateRoot, usagecore.WaitMeasureOptions{})
	if err != nil || len(duplicate.Runtimes) != 1 || len(duplicate.Runtimes[0].Samples) != 1 || duplicate.Runtimes[0].Samples[0].LooseEdgeReason != "publication-ambiguous" || duplicate.Runtimes[0].Samples[0].LowerEdge != "prev-observation" || duplicate.Runtimes[0].Samples[0].UpperEdge != "observation" || duplicate.Runtimes[0].Defects["publication-ambiguous"] != 1 {
		t.Fatalf("duplicate publication measurement=%+v err=%v", duplicate, err)
	}
}

func TestWaitSessionStartPrintsPendingRows(t *testing.T) {
	root := t.TempDir()
	self := int64(os.Getpid())
	exact, state, err := (identity.KernelProber{}).Probe(self)
	if err != nil || state != identity.Alive {
		t.Fatalf("current process identity=%+v state=%s err=%v", exact, state, err)
	}
	announcementPath, err := lease.AnnounceWithPair(root, "session-new", self, exact.StartedAt.Unix(), exact.StartTicks, exact.BootID, "wait-session-test", "fake", "lineage-a")
	if err != nil {
		t.Fatal(err)
	}
	mainID := announcedMainID(t, announcementPath)
	row := metarun.Waiter{
		SchemaVersion: 2, WaitID: strings.Repeat("a", 32), Nonce: strings.Repeat("b", 32),
		Kind: "attempt", TargetID: "attempt-a", OwnerLineage: "lineage-a", State: "pending",
		Deadline: "2026-09-14T12:00:00Z",
	}
	data, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(metarun.WaitersDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(metarun.WaitersDir(root), "attempt-attempt-a-owner.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	code, output, problem := captureChannelOutput(t, func() int {
		return runSessionStart([]string{"--root", root, "--session", "session-new"})
	})
	want := "WAITING attempt attempt-a until 2026-09-14T12:00:00Z: metasystem wait --resume " + strings.Repeat("a", 32)
	if code != 0 || strings.TrimSpace(output) != want || problem != "" {
		t.Fatalf("session start code=%d output=%q stderr=%q", code, output, problem)
	}
	if code, next, problem := captureChannelOutput(t, func() int { return runGoalNext([]string{"--root", root}) }); code != 0 || problem != "" || !strings.Contains(next, want) {
		t.Fatalf("goal next code=%d output=%q stderr=%q", code, next, problem)
	}
	code, verdictOutput, problem := captureChannelOutput(t, func() int {
		return runReportTurnVerdict([]string{"--root", root, "--session", "session-new", "--main-id", mainID})
	})
	var verdict struct {
		Display string `json:"display"`
	}
	if code != 0 || problem != "" || json.Unmarshal([]byte(verdictOutput), &verdict) != nil || strings.Contains(verdict.Display, want) {
		t.Fatalf("turn verdict code=%d output=%q stderr=%q display=%q", code, verdictOutput, problem, verdict.Display)
	}
	if binary := os.Getenv("METASYSTEM_WAIT_BINARY"); binary != "" {
		binary = testutil.InstalledWaitBinary(t, binary)
		if output, err := exec.Command("git", "-C", root, "init", "-q").CombinedOutput(); err != nil {
			t.Fatalf("initialize hook fixture repository: %v %s", err, output)
		}
		// The hook resolves state from its own installation, so the fixture's
		// durable wait row, hook, and canonical engine belong to one world. The
		// synthetic runtime intentionally has neither a staged signature adapter
		// nor a start-context channel; the announced holder must still recover its
		// durable wait row through the hook's system message.
		sourceHook, err := filepath.Abs(filepath.Join("..", "..", "scripts", "agents", "supervision-hook.sh"))
		if err != nil {
			t.Fatal(err)
		}
		hook := filepath.Join(root, "scripts", "agents", "supervision-hook.sh")
		canonicalEngine := filepath.Join(root, "bin", "metasystem")
		if err := os.MkdirAll(filepath.Dir(hook), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(canonicalEngine), 0o755); err != nil {
			t.Fatal(err)
		}
		hookBytes, err := os.ReadFile(sourceHook)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(hook, hookBytes, 0o755); err != nil {
			t.Fatal(err)
		}
		engineBytes, err := os.ReadFile(binary)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(canonicalEngine, engineBytes, 0o755); err != nil {
			t.Fatal(err)
		}
		wrapper := filepath.Join(t.TempDir(), "metasystem-hook-engine")
		wrapperSource := "#!/bin/sh\nif [ \"${1:-}\" = up ]; then printf '%s\\n' 'UP SUCCESS'; exit 0; fi\nexec \"${METASYSTEM_WAIT_REAL_ENGINE:?}\" \"$@\"\n"
		if err := os.WriteFile(wrapper, []byte(wrapperSource), 0o755); err != nil {
			t.Fatal(err)
		}
		command := exec.Command("bash", hook, "fake", "start")
		commandEnvironment := make([]string, 0, len(os.Environ())+2)
		for _, value := range os.Environ() {
			if strings.HasPrefix(value, "METASYSTEM_HOOK_DELEGATE_") || strings.HasPrefix(value, "METASYSTEM_BIN=") || strings.HasPrefix(value, "METASYSTEM_WAIT_REAL_ENGINE=") {
				continue
			}
			commandEnvironment = append(commandEnvironment, value)
		}
		command.Env = append(commandEnvironment, "METASYSTEM_BIN="+wrapper, "METASYSTEM_WAIT_REAL_ENGINE="+binary)
		command.Stdin = strings.NewReader(`{"session_id":"session-new","cwd":"` + root + `","source":"startup"}`)
		hookOutput, hookErr := command.CombinedOutput()
		if hookErr != nil || !strings.Contains(string(hookOutput), want) {
			t.Fatalf("session-start hook did not relay the durable wait line: err=%v output=%s", hookErr, hookOutput)
		}
		nonHolder := exec.Command("bash", hook, "fake", "start")
		nonHolder.Env = append(append([]string(nil), commandEnvironment...), "METASYSTEM_BIN="+wrapper, "METASYSTEM_WAIT_REAL_ENGINE="+binary)
		nonHolder.Stdin = strings.NewReader(`{"session_id":"another-session","cwd":"` + root + `","source":"startup"}`)
		nonHolderOutput, nonHolderErr := nonHolder.CombinedOutput()
		if nonHolderErr != nil || strings.Contains(string(nonHolderOutput), "WAITING attempt attempt-a") || strings.Contains(string(nonHolderOutput), "could not read durable wait recovery rows") {
			t.Fatalf("session-start hook treated a non-holder as a wait-row read failure: err=%v output=%s", nonHolderErr, nonHolderOutput)
		}
		t.Run("hook without lease omits recovery failure notice", func(t *testing.T) {
			leasePath := filepath.Join(root, "artifacts", "agents", "mains", "worktree-lease.json")
			leaseBytes, err := os.ReadFile(leasePath)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(leasePath); err != nil {
				t.Fatal(err)
			}
			withoutLease := exec.Command("bash", hook, "fake", "start")
			withoutLease.Env = append(append([]string(nil), commandEnvironment...), "METASYSTEM_BIN="+wrapper, "METASYSTEM_WAIT_REAL_ENGINE="+binary)
			withoutLease.Stdin = strings.NewReader(`{"session_id":"session-new","cwd":"` + root + `","source":"startup"}`)
			withoutLeaseOutput, withoutLeaseErr := withoutLease.CombinedOutput()
			if err := os.WriteFile(leasePath, leaseBytes, 0o600); err != nil {
				t.Fatal(err)
			}
			if withoutLeaseErr != nil || strings.Contains(string(withoutLeaseOutput), "Metasystem could not read durable wait recovery rows") {
				t.Fatalf("session-start hook treated an absent lease as a wait-row read failure: err=%v output=%s", withoutLeaseErr, withoutLeaseOutput)
			}
		})
	}
	row.State = "ready"
	data, _ = json.Marshal(row)
	if err := os.WriteFile(filepath.Join(metarun.WaitersDir(root), "attempt-attempt-a-owner.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	for name, invoke := range map[string]func() int{
		"session start": func() int { return runSessionStart([]string{"--root", root, "--session", "session-new"}) },
		"goal next":     func() int { return runGoalNext([]string{"--root", root}) },
		"turn verdict": func() int {
			return runReportTurnVerdict([]string{"--root", root, "--session", "session-new", "--main-id", mainID})
		},
	} {
		_, cleared, _ := captureChannelOutput(t, invoke)
		if strings.Contains(cleared, "WAITING attempt attempt-a") {
			t.Fatalf("%s advertised a cleared row: %q", name, cleared)
		}
	}
	if code, _, _ := captureChannelOutput(t, func() int {
		return runSessionStart([]string{"--root", root, "--session", "another-session"})
	}); code != metarun.ExitWaiterBusy {
		t.Fatalf("another session received this holder's recovery rows: exit=%d", code)
	}
}

func TestWaitSessionStartWithoutLeaseReturnsBusy(t *testing.T) {
	code, output, problem := captureChannelOutput(t, func() int {
		return runSessionStart([]string{"--root", t.TempDir(), "--session", "session-without-lease"})
	})
	if code != metarun.ExitWaiterBusy || output != "" || problem != "" {
		t.Fatalf("session start without lease code=%d output=%q stderr=%q", code, output, problem)
	}
}

type pendingWaitVerdictCommandOptions struct {
	jobStatus   string
	writeWaiter bool
}

func pendingWaitVerdictCommandFixture(t *testing.T, root, runtimeName string) (string, string) {
	t.Helper()
	return pendingWaitVerdictCommandFixtureWithOptions(t, root, runtimeName, pendingWaitVerdictCommandOptions{
		jobStatus: "pending-setup", writeWaiter: true,
	})
}

func pendingWaitVerdictCommandFixtureWithOptions(t *testing.T, root, runtimeName string, options pendingWaitVerdictCommandOptions) (string, string) {
	t.Helper()
	for _, name := range []string{"GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES"} {
		value, present := os.LookupEnv(name)
		if err := os.Unsetenv(name); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if present {
				_ = os.Setenv(name, value)
			} else {
				_ = os.Unsetenv(name)
			}
		})
	}
	for _, dir := range []string{
		filepath.Join(root, "plans"),
		filepath.Join(root, "artifacts", "agents", "jobs"),
		metarun.WaitersDir(root),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if output, err := exec.Command("git", "-C", root, "init", "-q", "-b", "main").CombinedOutput(); err != nil {
		t.Fatalf("initialize wait verdict fixture: %v %s", err, output)
	}
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes="+runtimeName+"\nrole.default.model."+runtimeName+"=fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	goals := []byte("# Goals\n\n## Current goal: wait-stop-goal — Hold the registered wait\n- Origin: main\n- Next step: Continue after the wait.\n")
	if err := os.WriteFile(filepath.Join(root, "plans", "goals.md"), goals, 0o644); err != nil {
		t.Fatal(err)
	}
	baseline, _ := json.Marshal(map[string]any{
		"schemaVersion": 1, "ledger": string(goals), "sha256": fmt.Sprintf("%x", sha256.Sum256(goals)),
	})
	if err := os.WriteFile(filepath.Join(root, "plans", "goals-accepted.json"), append(baseline, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "plans", "active.md"), []byte("# Active\n- Waiting on the human: none\n- Next step: Continue after the registered wait.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	self := int64(os.Getpid())
	exact, state, err := (identity.KernelProber{}).Probe(self)
	if err != nil || state != identity.Alive {
		t.Fatalf("current fixture process identity=%+v state=%s err=%v", exact, state, err)
	}
	processRef := exact.Ref()
	session := "wait-stop-" + runtimeName
	lineage := "wait-stop-lineage"
	announcement, err := lease.AnnounceWithPair(root, session, self, exact.StartedAt.Unix(), exact.StartTicks, exact.BootID, "wait-stop-fixture", runtimeName, lineage)
	if err != nil {
		t.Fatal(err)
	}
	mainID := announcedMainID(t, announcement)
	holder, err := lease.CurrentHolder(root)
	if err != nil {
		t.Fatal(err)
	}
	operationID := strings.Repeat("d", 32)
	if options.jobStatus == "" {
		options.jobStatus = "pending-setup"
	}
	job := map[string]any{
		"jobId": "wait-stop-job", "operationId": operationID, "status": options.jobStatus, "goalId": "wait-stop-goal",
	}
	target := metarun.WaiterTarget{OperationID: operationID}
	if options.jobStatus == "pending" {
		startedAt := time.Now().UTC().Add(-time.Second).Format(time.RFC3339Nano)
		job["mainId"], job["round"], job["startedAt"] = mainID, 1, startedAt
		target.Round, target.StartedAt = 1, startedAt
	}
	jobData, _ := json.Marshal(job)
	if err := os.WriteFile(filepath.Join(root, "artifacts", "agents", "jobs", "wait-stop-job.json"), append(jobData, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	if !options.writeWaiter {
		return session, mainID
	}
	signature, err := report.OpenWorkSignature(context.Background(), root)
	if err != nil || signature == "" {
		t.Fatalf("read fixture open-work signature: %q %v", signature, err)
	}
	bootID, elapsed, err := identity.BootClock()
	if err != nil || bootID == "" {
		t.Fatalf("read fixture boot clock: %q %s %v", bootID, elapsed, err)
	}
	now := time.Now().UTC()
	epoch := holder.ClaimEpoch
	row := metarun.Waiter{
		SchemaVersion: 2, WaitID: strings.Repeat("a", 32), Nonce: strings.Repeat("b", 32),
		Kind: "job", TargetID: "wait-stop-job", OwnerDigest: metarun.OwnerDigest(mainID),
		Pid: self, PidStartedAt: processRef.StartedAtSec, PidStartedAtMicro: processRef.StartedAtUnixMicro,
		PidStartTicks: processRef.StartTicks, BootID: processRef.BootID,
		Session: session, MainId: mainID, OwnerLineage: lineage, ClaimEpoch: &epoch, RuntimeSession: session,
		Selector:     metarun.WaitSelector{Kind: "job", TargetID: "wait-stop-job"},
		Target:       target,
		RegisteredAt: now.Add(-time.Second).Format(time.RFC3339Nano), Deadline: now.Add(time.Minute).Format(time.RFC3339Nano),
		BootDeadlineNanos: (elapsed + time.Minute).Nanoseconds(), DeadlineBootID: bootID, RemainingNanos: time.Minute.Nanoseconds(),
		LastObservedAt: now.Format(time.RFC3339Nano), LastObservedBootNanos: elapsed.Nanoseconds(),
		OpenWorkSignature: signature, State: "pending", Delivery: "blocking",
	}
	rowData, _ := json.Marshal(row)
	if err := os.WriteFile(metarun.WaiterPath(root, row.Kind, row.TargetID, row.OwnerDigest), append(rowData, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	return session, mainID
}

type installedHookResult struct {
	stdout    string
	stderr    string
	upCommand string
}

func boundedFixtureContext(t *testing.T, duration time.Duration) (context.Context, context.CancelFunc) {
	t.Helper()
	deadline := time.Now().Add(duration)
	if testDeadline, ok := t.Deadline(); ok && testDeadline.Before(deadline) {
		deadline = testDeadline
	}
	return context.WithDeadline(context.Background(), deadline)
}

func copyExecutableFixture(t *testing.T, source, target string) {
	t.Helper()
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, data, 0o755); err != nil {
		t.Fatal(err)
	}
}

func installPendingWaitHookFixture(t *testing.T, root, binary string) (hook, canonical, wrapper string) {
	t.Helper()
	sourceRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	for _, relative := range []string{
		"scripts/receipt.sh",
		"scripts/agents/supervision-hook.sh",
		"scripts/agents/evidence-gc.sh",
		"scripts/agents/arm-supervision.sh",
		"scripts/agents/dispatch.sh",
		"scripts/agents/adapters/fake.sh",
		"scripts/agents/adapters/runtime-common.sh",
		"scripts/watch-background-jobs.sh",
		"scripts/metasystem-config.sh",
	} {
		copyExecutableFixture(t, filepath.Join(sourceRoot, relative), filepath.Join(root, relative))
	}
	hook = filepath.Join(root, "scripts", "agents", "supervision-hook.sh")
	canonical = filepath.Join(root, "bin", "metasystem")
	copyExecutableFixture(t, binary, canonical)
	fixtureRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	canonical, err = filepath.EvalSymlinks(canonical)
	if err != nil {
		t.Fatal(err)
	}
	engine, err := os.ReadFile(canonical)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(engine)
	if err := os.MkdirAll(filepath.Dir(steward.RepoIdentityPath(fixtureRoot)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := steward.MintIdentity(steward.RepoIdentityPath(fixtureRoot), steward.InstallIdentity{
		RepoIdentity: fixtureRoot, Generation: 1, InstallPath: canonical,
		InstallDigest: fmt.Sprintf("sha256:%x", digest), MintedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Enrollment: steward.EnrollmentFixture,
	}); err != nil {
		t.Fatal(err)
	}
	wrapper = filepath.Join(t.TempDir(), "metasystem-hook-engine")
	wrapperSource := `#!/bin/sh
if [ "${1:-}" = up ]; then
  printf '%s' "$0" > "${METASYSTEM_WAIT_UP_COMMAND_FILE:?}"
  printf ' %s' "$@" >> "${METASYSTEM_WAIT_UP_COMMAND_FILE:?}"
  printf '\n' >> "${METASYSTEM_WAIT_UP_COMMAND_FILE:?}"
fi
exec "${METASYSTEM_WAIT_REAL_ENGINE:?}" "$@"
`
	if err := os.WriteFile(wrapper, []byte(wrapperSource), 0o755); err != nil {
		t.Fatal(err)
	}
	return hook, canonical, wrapper
}

func pendingWaitHookEnvironment(wrapper, canonical, now string) []string {
	environment := make([]string, 0, len(os.Environ())+4)
	for _, value := range os.Environ() {
		if strings.HasPrefix(value, "METASYSTEM_HOOK_DELEGATE_") ||
			strings.HasPrefix(value, "METASYSTEM_BIN=") ||
			strings.HasPrefix(value, "METASYSTEM_WAIT_REAL_ENGINE=") ||
			strings.HasPrefix(value, "METASYSTEM_WAIT_UP_COMMAND_FILE=") ||
			strings.HasPrefix(value, "METASYSTEM_FAKE_AGENT_ANCESTOR_PID=") ||
			strings.HasPrefix(value, "METASYSTEM_FAKE_PROCESS_IDENTITY_FILE=") ||
			strings.HasPrefix(value, "METASYSTEM_CENSUS_PROCESS_FILE=") ||
			strings.HasPrefix(value, "METASYSTEM_GOAL_NOW=") {
			continue
		}
		environment = append(environment, value)
	}
	environment = append(environment,
		"METASYSTEM_BIN="+wrapper,
		"METASYSTEM_WAIT_REAL_ENGINE="+canonical,
		"METASYSTEM_WAIT_UP_COMMAND_FILE="+wrapper+".up-command",
		"METASYSTEM_FAKE_AGENT_ANCESTOR_PID="+fmt.Sprint(os.Getpid()),
	)
	if now != "" {
		environment = append(environment, "METASYSTEM_GOAL_NOW="+now)
	}
	return environment
}

func runPendingWaitHook(t *testing.T, hook, wrapper, canonical, event, payload, label, now string) installedHookResult {
	t.Helper()
	outputDir := t.TempDir()
	stdoutPath := filepath.Join(outputDir, label+".stdout")
	stderrPath := filepath.Join(outputDir, label+".stderr")
	stdout, err := os.Create(stdoutPath)
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := os.Create(stderrPath)
	if err != nil {
		_ = stdout.Close()
		t.Fatal(err)
	}
	ctx, cancel := boundedFixtureContext(t, 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "/bin/bash", hook, "fake", event)
	command.Env = pendingWaitHookEnvironment(wrapper, canonical, now)
	command.Stdin = strings.NewReader(payload)
	command.Stdout, command.Stderr = stdout, stderr
	runErr := command.Run()
	closeStdoutErr, closeStderrErr := stdout.Close(), stderr.Close()
	stdoutData, stdoutErr := os.ReadFile(stdoutPath)
	stderrData, stderrErr := os.ReadFile(stderrPath)
	upCommandData, upCommandErr := os.ReadFile(wrapper + ".up-command")
	if os.IsNotExist(upCommandErr) {
		upCommandData, upCommandErr = []byte("not captured"), nil
	}
	if ctx.Err() != nil {
		fixtureRoot := filepath.Dir(filepath.Dir(filepath.Dir(hook)))
		announcements := lease.AnnouncementsFor(fixtureRoot, int64(os.Getpid()))
		t.Fatalf("%s hook exceeded its Go deadline: announcements=%+v up-command=%s stdout=%s stderr=%s",
			label, announcements, upCommandData, stdoutData, stderrData)
	}
	if runErr != nil || closeStdoutErr != nil || closeStderrErr != nil || stdoutErr != nil || stderrErr != nil || upCommandErr != nil {
		t.Fatalf("%s hook failed: run=%v close=(%v,%v) read=(%v,%v,%v) up-command=%s stdout=%s stderr=%s",
			label, runErr, closeStdoutErr, closeStderrErr, stdoutErr, stderrErr, upCommandErr, upCommandData, stdoutData, stderrData)
	}
	return installedHookResult{stdout: string(stdoutData), stderr: string(stderrData), upCommand: strings.TrimSpace(string(upCommandData))}
}

func pendingWaitFixtureNow(t *testing.T, root, waitID string) string {
	t.Helper()
	row, _, err := metarun.FindWaiterByID(root, waitID)
	if err != nil {
		t.Fatal(err)
	}
	observed, err := time.Parse(time.RFC3339Nano, row.LastObservedAt)
	if err != nil {
		t.Fatalf("waiter %s lastObservedAt: %v", waitID, err)
	}
	return observed.Add(30 * time.Second).Format(time.RFC3339Nano)
}

func pendingWaitVerdict(t *testing.T, root, session, mainID string) struct {
	ShouldBlock bool    `json:"shouldBlock"`
	BlockSource *string `json:"blockSource"`
	IdleRefusal bool    `json:"idleRefusal"`
	CountSpent  bool    `json:"countSpent"`
	Display     string  `json:"display"`
} {
	t.Helper()
	code, output, problem := captureChannelOutput(t, func() int {
		return runReportTurnVerdict([]string{"--root", root, "--session", session, "--main-id", mainID})
	})
	var verdict struct {
		ShouldBlock bool    `json:"shouldBlock"`
		BlockSource *string `json:"blockSource"`
		IdleRefusal bool    `json:"idleRefusal"`
		CountSpent  bool    `json:"countSpent"`
		Display     string  `json:"display"`
	}
	if code != 0 || problem != "" || json.Unmarshal([]byte(output), &verdict) != nil {
		t.Fatalf("turn verdict session=%s code=%d stderr=%q output=%q", session, code, problem, output)
	}
	return verdict
}

func countWaitingLines(display string) int {
	count := 0
	for _, line := range strings.Split(display, "\n") {
		if strings.HasPrefix(line, "WAITING") {
			count++
		}
	}
	return count
}

func assertUnwatchedWaitVerdict(t *testing.T, verdict struct {
	ShouldBlock bool    `json:"shouldBlock"`
	BlockSource *string `json:"blockSource"`
	IdleRefusal bool    `json:"idleRefusal"`
	CountSpent  bool    `json:"countSpent"`
	Display     string  `json:"display"`
}) {
	t.Helper()
	if !verdict.ShouldBlock || verdict.BlockSource == nil || *verdict.BlockSource != "unwatched-work" ||
		countWaitingLines(verdict.Display) != 0 {
		t.Fatalf("unassociated Stop verdict=%+v", verdict)
	}
}

func assertRegisteredWaitVerdict(t *testing.T, verdict struct {
	ShouldBlock bool    `json:"shouldBlock"`
	BlockSource *string `json:"blockSource"`
	IdleRefusal bool    `json:"idleRefusal"`
	CountSpent  bool    `json:"countSpent"`
	Display     string  `json:"display"`
}, waitingLine string) {
	t.Helper()
	lineCount := 0
	for _, line := range strings.Split(verdict.Display, "\n") {
		if line == waitingLine {
			lineCount++
		}
	}
	if verdict.ShouldBlock || verdict.BlockSource != nil || verdict.IdleRefusal || verdict.CountSpent ||
		lineCount != 1 || countWaitingLines(verdict.Display) != 1 || strings.Contains(strings.ToLower(verdict.Display), "unwatched") {
		t.Fatalf("associated Stop verdict=%+v waitingLine=%q", verdict, waitingLine)
	}
}

func writePendingWaitJobStatus(root, status string) error {
	path := filepath.Join(root, "artifacts", "agents", "jobs", "wait-stop-job.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var job map[string]any
	if err := json.Unmarshal(data, &job); err != nil {
		return err
	}
	job["status"] = status
	data, err = json.Marshal(job)
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func pendingWaiterRows(root string) ([]metarun.Waiter, error) {
	paths, err := filepath.Glob(filepath.Join(metarun.WaitersDir(root), "*.json"))
	if err != nil {
		return nil, err
	}
	rows := make([]metarun.Waiter, 0, len(paths))
	for _, path := range paths {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil, fmt.Errorf("read waiter row %s: %w", path, readErr)
		}
		var row metarun.Waiter
		if err := json.Unmarshal(data, &row); err != nil {
			return nil, fmt.Errorf("decode waiter row %s: %w", path, err)
		}
		if row.SchemaVersion != 2 {
			return nil, fmt.Errorf("waiter row %s has schema version %d, want 2", path, row.SchemaVersion)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func pollPendingWaiter(t *testing.T, root, target string) metarun.Waiter {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	if testDeadline, ok := t.Deadline(); ok && testDeadline.Before(deadline) {
		deadline = testDeadline
	}
	for time.Now().Before(deadline) {
		rows, err := pendingWaiterRows(root)
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range rows {
			if row.Kind == "job" && row.TargetID == target && row.State == "pending" {
				return row
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("waiter for %s did not become pending before the Go poll deadline", target)
	return metarun.Waiter{}
}

func writeWaiterFixture(t *testing.T, root string, row metarun.Waiter) {
	t.Helper()
	data, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(metarun.WaiterPath(root, row.Kind, row.TargetID, row.OwnerDigest), append(data, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}

func proveFixtureProcessOwnership(t *testing.T, pid int, instanceTag string) error {
	t.Helper()
	outputPath := filepath.Join(t.TempDir(), "process-command.txt")
	output, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	ctx, cancel := boundedFixtureContext(t, 2*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "ps", "-p", fmt.Sprint(pid), "-o", "command=")
	command.Stdout = output
	command.Stderr = output
	runErr := command.Run()
	closeErr := output.Close()
	data, readErr := os.ReadFile(outputPath)
	if ctx.Err() != nil {
		return fmt.Errorf("process ownership proof exceeded its Go deadline")
	}
	if runErr != nil || closeErr != nil || readErr != nil {
		return fmt.Errorf("process ownership proof failed: run=%v close=%v read=%v", runErr, closeErr, readErr)
	}
	if !strings.Contains(string(data), instanceTag) {
		return fmt.Errorf("process %d command %q does not carry fixture tag %q", pid, strings.TrimSpace(string(data)), instanceTag)
	}
	return nil
}

func assertPendingWaitVerdictOutput(t *testing.T, output []byte) {
	t.Helper()
	var verdict struct {
		ShouldBlock bool   `json:"shouldBlock"`
		Display     string `json:"display"`
	}
	if err := json.Unmarshal(output, &verdict); err != nil || verdict.ShouldBlock || !strings.Contains(verdict.Display, "WAITING: registered wait") {
		t.Fatalf("pending-wait verdict output=%s parsed=%+v err=%v", output, verdict, err)
	}
}

func TestPendingWaitInstalledVerdicts(t *testing.T) {
	root := t.TempDir()
	session, mainID := pendingWaitVerdictCommandFixture(t, root, "fake")
	code, output, problem := captureChannelOutput(t, func() int {
		return runReportTurnVerdict([]string{"--root", root, "--session", session, "--main-id", mainID})
	})
	if code != 0 || problem != "" {
		t.Fatalf("plain command handler code=%d stderr=%q output=%s", code, problem, output)
	}
	assertPendingWaitVerdictOutput(t, []byte(output))

	candidate := os.Getenv("METASYSTEM_WAIT_BINARY")
	if candidate == "" {
		return
	}
	binary := testutil.InstalledWaitBinary(t, candidate)

	plainRoot := t.TempDir()
	plainSession, plainMainID := pendingWaitVerdictCommandFixture(t, plainRoot, "fake")
	plain := exec.Command(binary, "report", "turn-verdict", "--root", plainRoot, "--session", plainSession, "--main-id", plainMainID)
	plainOutput, err := plain.CombinedOutput()
	if err != nil {
		t.Fatalf("installed plain turn verdict: %v %s", err, plainOutput)
	}
	assertPendingWaitVerdictOutput(t, plainOutput)

	hookRoot := t.TempDir()
	hookSession, _ := pendingWaitVerdictCommandFixture(t, hookRoot, "fake")
	sourceRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	hook := filepath.Join(hookRoot, "scripts", "agents", "supervision-hook.sh")
	evidenceGC := filepath.Join(hookRoot, "scripts", "agents", "evidence-gc.sh")
	canonical := filepath.Join(hookRoot, "bin", "metasystem")
	for _, target := range []string{hook, evidenceGC, canonical} {
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for source, target := range map[string]string{
		filepath.Join(sourceRoot, "scripts", "agents", "supervision-hook.sh"): hook,
		filepath.Join(sourceRoot, "scripts", "receipt.sh"):                    filepath.Join(hookRoot, "scripts", "receipt.sh"),
		binary: canonical,
	} {
		data, readErr := os.ReadFile(source)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if writeErr := os.WriteFile(target, data, 0o755); writeErr != nil {
			t.Fatal(writeErr)
		}
	}
	if err := os.WriteFile(evidenceGC, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	wrapper := filepath.Join(t.TempDir(), "metasystem-hook-engine")
	wrapperSource := `#!/bin/sh
if [ "${1:-}" = up ]; then printf '%s\n' 'up outcome=already-healthy'; exit 0; fi
if [ "${1:-}" = health ]; then
  printf '%s\n' '{"schemaVersion":1,"exitCode":0,"line":"HEALTH healthy — ","interventions":[],"verdict":{"schema":1,"observedAt":"2026-09-17T10:00:00Z","observation":1,"aggregate":"healthy","roles":[],"shouldAlert":false,"findingDigest":""}}'
  exit 0
fi
exec "${METASYSTEM_WAIT_REAL_ENGINE:?}" "$@"
`
	if err := os.WriteFile(wrapper, []byte(wrapperSource), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("METASYSTEM_GOAL_NOW", "2099-01-01T00:00:00Z")
	command := exec.Command("bash", hook, "fake", "stop")
	environment := make([]string, 0, len(os.Environ())+3)
	for _, value := range os.Environ() {
		if strings.HasPrefix(value, "METASYSTEM_BIN=") || strings.HasPrefix(value, "METASYSTEM_WAIT_REAL_ENGINE=") || strings.HasPrefix(value, "METASYSTEM_FAKE_AGENT_ANCESTOR_PID=") || strings.HasPrefix(value, "METASYSTEM_GOAL_NOW=") {
			continue
		}
		environment = append(environment, value)
	}
	command.Env = append(environment,
		"METASYSTEM_BIN="+wrapper,
		"METASYSTEM_WAIT_REAL_ENGINE="+canonical,
		"METASYSTEM_FAKE_AGENT_ANCESTOR_PID="+fmt.Sprint(os.Getpid()),
		"METASYSTEM_GOAL_NOW="+pendingWaitFixtureNow(t, hookRoot, strings.Repeat("a", 32)),
	)
	command.Stdin = strings.NewReader(`{"session_id":"` + hookSession + `","cwd":"` + hookRoot + `","hook_event_name":"Stop"}`)
	hookOutput, err := command.CombinedOutput()
	if err != nil || strings.Contains(string(hookOutput), `"decision":"block"`) || strings.Contains(string(hookOutput), "needs supervision repair") {
		reportText := "unavailable"
		var payload map[string]string
		if json.Unmarshal(hookOutput, &payload) == nil {
			message := payload["reason"]
			if message == "" {
				message = payload["systemMessage"]
			}
			if at := strings.LastIndex(message, "stop-status --id "); at >= 0 {
				alias := strings.TrimSpace(message[at+len("stop-status --id "):])
				if data, _, readErr := report.ReadStopStatus(hookRoot, alias); readErr == nil {
					reportText = string(data)
				}
			}
		}
		t.Fatalf("fake Stop hook did not allow the registered wait: err=%v output=%s report=%s", err, hookOutput, reportText)
	}
	artifact := filepath.Join(hookRoot, "artifacts", "agents", "supervision", "stop-verdicts", hookSession+".txt")
	artifactData, err := os.ReadFile(artifact)
	if err != nil || !strings.Contains(string(artifactData), "WAITING: registered wait") {
		t.Fatalf("fake Stop hook omitted its registered-wait evidence: %v %s", err, artifactData)
	}
}

func TestPendingWaitFromChildShell(t *testing.T) {
	candidate := os.Getenv("METASYSTEM_WAIT_BINARY")
	if candidate == "" {
		t.Skip("METASYSTEM_WAIT_BINARY is not set")
	}
	binary := testutil.InstalledWaitBinary(t, candidate)
	root := t.TempDir()
	_, mainID := pendingWaitVerdictCommandFixtureWithOptions(t, root, "fake", pendingWaitVerdictCommandOptions{
		jobStatus: "pending", writeWaiter: false,
	})
	runtimeSession := "8d91c146-8460-4bf1-9a48-04bc577c3aa4"
	hook, canonical, wrapper := installPendingWaitHookFixture(t, root, binary)
	shutdownOutputDir := t.TempDir()
	t.Cleanup(func() {
		stdout, stdoutErr := os.Create(filepath.Join(shutdownOutputDir, "shutdown.stdout"))
		stderr, stderrErr := os.Create(filepath.Join(shutdownOutputDir, "shutdown.stderr"))
		if stdoutErr != nil || stderrErr != nil {
			t.Errorf("prepare supervision shutdown output: stdout=%v stderr=%v", stdoutErr, stderrErr)
			return
		}
		ctx, cancel := boundedFixtureContext(t, 15*time.Second)
		defer cancel()
		command := exec.CommandContext(ctx, canonical, "up", "--metasystem-root", root, "--repo", root, "--shutdown")
		command.Stdout, command.Stderr = stdout, stderr
		runErr := command.Run()
		closeStdoutErr, closeStderrErr := stdout.Close(), stderr.Close()
		if ctx.Err() != nil || runErr != nil || closeStdoutErr != nil || closeStderrErr != nil {
			output, _ := os.ReadFile(filepath.Join(shutdownOutputDir, "shutdown.stdout"))
			problem, _ := os.ReadFile(filepath.Join(shutdownOutputDir, "shutdown.stderr"))
			t.Errorf("shut down fixture supervision: deadline=%v run=%v close=(%v,%v) stdout=%s stderr=%s",
				ctx.Err(), runErr, closeStdoutErr, closeStderrErr, output, problem)
		}
	})

	startPayload := fmt.Sprintf(`{"session_id":%q,"cwd":%q,"source":"clear"}`, runtimeSession, root)
	startHook := runPendingWaitHook(t, hook, wrapper, canonical, "start", startPayload, "associated-start", "")
	announcements := lease.AnnouncementsFor(root, int64(os.Getpid()))
	if len(announcements) != 1 || announcements[0].MainId != mainID ||
		announcements[0].RuntimeSession != runtimeSession || announcements[0].SessionId == runtimeSession {
		t.Fatalf("hook-associated announcement=%+v main=%s runtime-session=%s up-command=%s hook-stdout=%s hook-stderr=%s",
			announcements, mainID, runtimeSession, startHook.upCommand, startHook.stdout, startHook.stderr)
	}
	if rows, err := pendingWaiterRows(root); err != nil || len(rows) != 0 {
		t.Fatalf("fixture or SessionStart wrote a waiter row: rows=%+v err=%v", rows, err)
	}

	childOutputDir := t.TempDir()
	childStdoutPath := filepath.Join(childOutputDir, "child.stdout")
	childStderrPath := filepath.Join(childOutputDir, "child.stderr")
	childStdout, err := os.Create(childStdoutPath)
	if err != nil {
		t.Fatal(err)
	}
	childStderr, err := os.Create(childStderrPath)
	if err != nil {
		_ = childStdout.Close()
		t.Fatal(err)
	}
	child := exec.Command("/bin/bash", "-c", `exec "$0" job watch --root "$1" --job "$2" --caller-pid $$`, binary, root, "wait-stop-job")
	child.Stdout, child.Stderr = childStdout, childStderr
	if err := child.Start(); err != nil {
		_ = childStdout.Close()
		_ = childStderr.Close()
		t.Fatal(err)
	}
	childPID := child.Process.Pid
	childExit := make(chan error, 1)
	go func() { childExit <- child.Wait() }()
	childObserved := false
	var childWaitErr error
	var childWaitID string
	finishChild := func() error {
		if childObserved {
			return childWaitErr
		}
		if err := writePendingWaitJobStatus(root, "completed"); err != nil {
			return err
		}
		notifyStdout, err := os.Create(filepath.Join(childOutputDir, "notify.stdout"))
		if err != nil {
			return err
		}
		notifyStderr, err := os.Create(filepath.Join(childOutputDir, "notify.stderr"))
		if err != nil {
			_ = notifyStdout.Close()
			return err
		}
		notifyContext, cancel := boundedFixtureContext(t, 5*time.Second)
		notify := exec.CommandContext(notifyContext, binary, "wait", "notify", "--root", root, "--job", "wait-stop-job")
		notify.Stdout, notify.Stderr = notifyStdout, notifyStderr
		notifyErr := notify.Run()
		notifyContextErr := notifyContext.Err()
		cancel()
		closeNotifyStdoutErr, closeNotifyStderrErr := notifyStdout.Close(), notifyStderr.Close()
		if notifyContextErr != nil || notifyErr != nil || closeNotifyStdoutErr != nil || closeNotifyStderrErr != nil {
			return fmt.Errorf("notify child wait while waiting for wait notify --job wait-stop-job: context=%v run=%v close=(%v,%v)", notifyContextErr, notifyErr, closeNotifyStdoutErr, closeNotifyStderrErr)
		}
		completionContext, cancelCompletion := boundedFixtureContext(t, 5*time.Second)
		defer cancelCompletion()
		select {
		case childWaitErr = <-childExit:
			childObserved = true
		case <-completionContext.Done():
			completionErr := completionContext.Err()
			if err := proveFixtureProcessOwnership(t, childPID, filepath.Base(binary)); err != nil {
				return err
			}
			if err := child.Process.Kill(); err != nil {
				return err
			}
			reapContext, cancelReap := boundedFixtureContext(t, 5*time.Second)
			defer cancelReap()
			select {
			case childWaitErr = <-childExit:
				childObserved = true
			case <-reapContext.Done():
				return fmt.Errorf("wait for the killed child wait process to be reaped: %w", reapContext.Err())
			}
			return fmt.Errorf("wait for the child wait process to exit after notifying wait-stop-job: %w", completionErr)
		}
		closeStdoutErr, closeStderrErr := childStdout.Close(), childStderr.Close()
		if closeStdoutErr != nil || closeStderrErr != nil {
			return fmt.Errorf("close child output: stdout=%v stderr=%v", closeStdoutErr, closeStderrErr)
		}
		if childWaitErr != nil {
			return childWaitErr
		}
		for childWaitID != "" {
			select {
			case <-completionContext.Done():
				return fmt.Errorf("wait for registered wait %s to leave pending after its child exited: %w", childWaitID, completionContext.Err())
			default:
			}
			stored, _, findErr := metarun.FindWaiterByID(root, childWaitID)
			if findErr != nil {
				return fmt.Errorf("read registered wait %s after its child exited: %w", childWaitID, findErr)
			}
			if stored.State != "pending" {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		return nil
	}
	t.Cleanup(func() {
		if err := finishChild(); err != nil {
			output, _ := os.ReadFile(childStdoutPath)
			problem, _ := os.ReadFile(childStderrPath)
			t.Errorf("finish child wait: %v stdout=%s stderr=%s", err, output, problem)
		}
	})

	row := pollPendingWaiter(t, root, "wait-stop-job")
	childWaitID = row.WaitID
	if row.Session != runtimeSession || row.RuntimeSession != runtimeSession || row.MainId != mainID || row.Pid != int64(childPID) {
		t.Fatalf("child-shell waiter did not carry the runtime owner: row=%+v child-pid=%d", row, childPID)
	}
	waitingLine := fmt.Sprintf("WAITING: registered wait %s covers job wait-stop-job until %s", row.WaitID, row.Deadline)

	plainUnassociatedSession := "5f4981ee-a34c-41a7-a0ac-ec51e6cf51ad"
	assertUnwatchedWaitVerdict(t, pendingWaitVerdict(t, root, plainUnassociatedSession, mainID))
	beforeBlock := 0
	hookLog := filepath.Join(root, "artifacts", "agents", "supervision", "hooks.log")
	if data, readErr := os.ReadFile(hookLog); readErr == nil {
		beforeBlock = strings.Count(string(data), "stop response decision=block")
	} else if !os.IsNotExist(readErr) {
		t.Fatal(readErr)
	}
	hookUnassociatedSession := "d4e2c904-c6b1-457a-8ed3-2c1e4a0aef88"
	controlPayload := fmt.Sprintf(`{"session_id":%q,"cwd":%q,"hook_event_name":"Stop"}`, hookUnassociatedSession, root)
	controlHook := runPendingWaitHook(t, hook, wrapper, canonical, "stop", controlPayload, "unassociated-stop", "")
	if !strings.Contains(controlHook.stdout, `"decision":"block"`) {
		t.Fatalf("unassociated Stop hook did not block: stdout=%s stderr=%s", controlHook.stdout, controlHook.stderr)
	}
	logData, err := os.ReadFile(hookLog)
	if err != nil || strings.Count(string(logData), "stop response decision=block") != beforeBlock+1 {
		t.Fatalf("unassociated Stop hook log=%s err=%v", logData, err)
	}
	announcements = lease.AnnouncementsFor(root, int64(os.Getpid()))
	if len(announcements) != 1 || announcements[0].RuntimeSession != runtimeSession {
		t.Fatalf("late unassociated Stop rolled back the runtime session: %+v", announcements)
	}
	if rows, rowsErr := pendingWaiterRows(root); rowsErr != nil || len(rows) != 1 || rows[0].WaitID != row.WaitID || rows[0].Pid != int64(childPID) {
		t.Fatalf("the Stop gate wrote or replaced a waiter row: rows=%+v err=%v", rows, rowsErr)
	}

	assertRegisteredWaitVerdict(t, pendingWaitVerdict(t, root, runtimeSession, mainID), waitingLine)
	beforeAllow := strings.Count(string(logData), "stop response decision=allow")
	allowedPayload := fmt.Sprintf(`{"session_id":%q,"cwd":%q,"hook_event_name":"Stop"}`, runtimeSession, root)
	allowedHook := runPendingWaitHook(t, hook, wrapper, canonical, "stop", allowedPayload, "associated-stop", pendingWaitFixtureNow(t, root, row.WaitID))
	if strings.Contains(allowedHook.stdout, `"decision":"block"`) {
		t.Fatalf("associated Stop hook blocked: stdout=%s stderr=%s", allowedHook.stdout, allowedHook.stderr)
	}
	logData, err = os.ReadFile(hookLog)
	if err != nil || strings.Count(string(logData), "stop response decision=allow") != beforeAllow+1 {
		t.Fatalf("associated Stop hook log=%s err=%v", logData, err)
	}
	verdictArtifact := filepath.Join(root, "artifacts", "agents", "supervision", "stop-verdicts", runtimeSession+".txt")
	artifactData, err := os.ReadFile(verdictArtifact)
	if err != nil || strings.Count(string(artifactData), waitingLine) != 1 || countWaitingLines(string(artifactData)) != 1 {
		t.Fatalf("associated Stop artifact=%s err=%v", artifactData, err)
	}

	foreign := row
	foreign.WaitID, foreign.Nonce = strings.Repeat("c", 32), strings.Repeat("e", 32)
	foreign.MainId = "main-foreign"
	foreign.OwnerDigest = metarun.OwnerDigest(foreign.MainId)
	writeWaiterFixture(t, root, foreign)
	assertRegisteredWaitVerdict(t, pendingWaitVerdict(t, root, runtimeSession, mainID), waitingLine)
	beforeAllow = strings.Count(string(logData), "stop response decision=allow")
	t.Setenv("METASYSTEM_GOAL_NOW", "2099-01-01T00:00:00Z")
	allowedHook = runPendingWaitHook(t, hook, wrapper, canonical, "stop", allowedPayload, "associated-stop-with-hostile-rows", pendingWaitFixtureNow(t, root, row.WaitID))
	t.Setenv("METASYSTEM_GOAL_NOW", "")
	if strings.Contains(allowedHook.stdout, `"decision":"block"`) {
		t.Fatalf("hostile rows changed the associated Stop: stdout=%s stderr=%s", allowedHook.stdout, allowedHook.stderr)
	}
	logData, err = os.ReadFile(hookLog)
	if err != nil || strings.Count(string(logData), "stop response decision=allow") != beforeAllow+1 {
		t.Fatalf("associated hostile-row Stop hook log=%s err=%v", logData, err)
	}
	artifactData, err = os.ReadFile(verdictArtifact)
	if err != nil || strings.Count(string(artifactData), waitingLine) != 1 || countWaitingLines(string(artifactData)) != 1 {
		t.Fatalf("hostile-row Stop artifact=%s err=%v", artifactData, err)
	}
	hostileSession := "never-associated-hostile"
	assertUnwatchedWaitVerdict(t, pendingWaitVerdict(t, root, hostileSession, mainID))

	sleepSource, err := exec.LookPath("sleep")
	if err != nil {
		t.Fatal(err)
	}
	sleepTag := "metasystem-child-wait-dead-sleeper"
	sleepBinary := filepath.Join(t.TempDir(), sleepTag)
	if err := os.Symlink(sleepSource, sleepBinary); err != nil {
		t.Fatal(err)
	}
	sleepOutputDir := t.TempDir()
	sleepOutput, err := os.Create(filepath.Join(sleepOutputDir, "sleep.stdout"))
	if err != nil {
		t.Fatal(err)
	}
	sleepProblem, err := os.Create(filepath.Join(sleepOutputDir, "sleep.stderr"))
	if err != nil {
		_ = sleepOutput.Close()
		t.Fatal(err)
	}
	sleeper := exec.Command(sleepBinary, "60")
	sleeper.Stdout, sleeper.Stderr = sleepOutput, sleepProblem
	if err := sleeper.Start(); err != nil {
		_ = sleepOutput.Close()
		_ = sleepProblem.Close()
		t.Fatal(err)
	}
	sleeperReaped := false
	t.Cleanup(func() {
		if sleeperReaped {
			return
		}
		if proveErr := proveFixtureProcessOwnership(t, sleeper.Process.Pid, sleepTag); proveErr == nil {
			_ = sleeper.Process.Kill()
		}
		_ = sleeper.Wait()
		_ = sleepOutput.Close()
		_ = sleepProblem.Close()
	})
	sleeperExact, sleeperState, err := (identity.KernelProber{}).Probe(int64(sleeper.Process.Pid))
	if err != nil || sleeperState != identity.Alive {
		t.Fatalf("dead-row sleeper identity=%+v state=%s err=%v", sleeperExact, sleeperState, err)
	}
	if err := proveFixtureProcessOwnership(t, sleeper.Process.Pid, sleepTag); err != nil {
		t.Fatal(err)
	}
	if err := sleeper.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err := sleeper.Wait(); err == nil {
		t.Fatal("killed sleeper exited without its signal status")
	}
	sleeperReaped = true
	if err := sleepOutput.Close(); err != nil {
		t.Fatal(err)
	}
	if err := sleepProblem.Close(); err != nil {
		t.Fatal(err)
	}
	deadStartedAt := time.Now().UTC().Add(-time.Second).Format(time.RFC3339Nano)
	deadOperationID := strings.Repeat("f", 32)
	deadJob := map[string]any{
		"jobId": "dead-wait-job", "operationId": deadOperationID, "status": "pending", "goalId": "wait-stop-goal",
		"mainId": mainID, "round": 1, "startedAt": deadStartedAt,
	}
	deadJobData, _ := json.Marshal(deadJob)
	if err := os.WriteFile(filepath.Join(root, "artifacts", "agents", "jobs", "dead-wait-job.json"), append(deadJobData, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	dead := row
	dead.WaitID, dead.Nonce = strings.Repeat("a", 31)+"c", strings.Repeat("b", 31)+"d"
	dead.TargetID = "dead-wait-job"
	dead.Selector = metarun.WaitSelector{Kind: "job", TargetID: dead.TargetID}
	dead.Target = metarun.WaiterTarget{OperationID: deadOperationID, Round: 1, StartedAt: deadStartedAt}
	dead.Pid, dead.PidStartedAt = int64(sleeper.Process.Pid), sleeperExact.StartedAt.Unix()
	dead.PidStartedAtMicro, dead.PidStartTicks, dead.BootID = sleeperExact.StartedAt.UnixMicro(), sleeperExact.StartTicks, sleeperExact.BootID
	writeWaiterFixture(t, root, dead)
	runPendingWaitHook(t, hook, wrapper, canonical, "stop", allowedPayload, "associated-stop-with-dead-waiter", "")
	logData, err = os.ReadFile(hookLog)
	if err != nil {
		t.Fatalf("associated dead-waiter Stop hook log=%s err=%v", logData, err)
	}
	deadWaitingLine := fmt.Sprintf("WAITING: registered wait %s covers job dead-wait-job until %s", dead.WaitID, dead.Deadline)
	artifactData, err = os.ReadFile(verdictArtifact)
	if err != nil || strings.Count(string(artifactData), waitingLine) != 1 ||
		strings.Contains(string(artifactData), deadWaitingLine) || countWaitingLines(string(artifactData)) != 1 {
		t.Fatalf("dead-waiter Stop artifact=%s err=%v", artifactData, err)
	}

	beforeBlock = strings.Count(string(logData), "stop response decision=block")
	absentPayload := fmt.Sprintf(`{"cwd":%q,"hook_event_name":"Stop"}`, root)
	absentHook := runPendingWaitHook(t, hook, wrapper, canonical, "stop", absentPayload, "absent-session-stop", "")
	if !strings.Contains(absentHook.stdout, `"decision":"block"`) {
		t.Fatalf("sessionless Stop hook did not block: stdout=%s stderr=%s", absentHook.stdout, absentHook.stderr)
	}
	logData, err = os.ReadFile(hookLog)
	if err != nil || strings.Count(string(logData), "stop response decision=block") != beforeBlock+1 {
		t.Fatalf("sessionless Stop hook log=%s err=%v", logData, err)
	}
	if err := finishChild(); err != nil {
		output, _ := os.ReadFile(childStdoutPath)
		problem, _ := os.ReadFile(childStderrPath)
		t.Fatalf("child wait did not exit zero: %v stdout=%s stderr=%s", err, output, problem)
	}
	stored, _, findErr := metarun.FindWaiterByID(root, row.WaitID)
	if findErr != nil {
		t.Fatalf("read completed child waiter: %v", findErr)
	}
	if stored.State == "pending" {
		t.Fatalf("completed child left its waiter pending: %+v", stored)
	}
}

func TestWaitLeaseTakeoverRepairsAndResumes(t *testing.T) {
	root := t.TempDir()
	predecessor := exec.Command("sleep", "60")
	if err := predecessor.Start(); err != nil {
		t.Fatal(err)
	}
	predecessorPID := int64(predecessor.Process.Pid)
	predecessorExact, state, err := (identity.KernelProber{}).Probe(predecessorPID)
	if err != nil || state != identity.Alive {
		_ = predecessor.Process.Kill()
		t.Fatalf("predecessor identity=%+v %s %v", predecessorExact, state, err)
	}
	oldPath, err := lease.AnnounceWithPair(root, "old-session", predecessorPID, predecessorExact.StartedAt.Unix(), predecessorExact.StartTicks, predecessorExact.BootID, "wait-old", "fake", "")
	if err != nil {
		_ = predecessor.Process.Kill()
		t.Fatal(err)
	}
	oldMain := announcedMainID(t, oldPath)
	bootID, elapsed, err := identity.BootClock()
	if err != nil {
		_ = predecessor.Process.Kill()
		t.Fatal(err)
	}
	now := time.Now().UTC()
	waitID, nonce := strings.Repeat("c", 32), strings.Repeat("d", 32)
	epoch := int64(1)
	row := metarun.Waiter{
		SchemaVersion: 2, WaitID: waitID, Nonce: nonce, Kind: "job", TargetID: "job-takeover", OwnerDigest: metarun.OwnerDigest(oldMain),
		Pid: predecessorPID, PidStartedAt: predecessorExact.StartedAt.Unix(), PidStartTicks: predecessorExact.StartTicks, BootID: predecessorExact.BootID,
		Session: "old-session", MainId: oldMain, OwnerLineage: oldMain, ClaimEpoch: &epoch, RuntimeSession: "old-session",
		Selector: metarun.WaitSelector{Kind: "job", TargetID: "job-takeover"}, Target: metarun.WaiterTarget{OperationID: "reserve-takeover", StartedAt: "2026-09-13T10:00:00Z", Round: 2},
		RegisteredAt: now.Add(-time.Minute).Format(time.RFC3339Nano), Deadline: now.Add(time.Hour).Format(time.RFC3339Nano), DeadlineBootID: bootID,
		BootDeadlineNanos: (elapsed + time.Hour).Nanoseconds(), RemainingNanos: time.Hour.Nanoseconds(), LastObservedBootNanos: elapsed.Nanoseconds(), OpenWorkSignature: report.Scan(root).OpenWorkSignature(), State: "pending", Delivery: "blocking",
	}
	if err := os.MkdirAll(metarun.WaitersDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	rowPath := metarun.WaiterPath(root, row.Kind, row.TargetID, row.OwnerDigest)
	encoded, _ := json.Marshal(row)
	if err := os.WriteFile(rowPath, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobs, "job-takeover.json"), []byte(`{"jobId":"job-takeover","operationId":"reserve-takeover","round":2,"status":"completed","startedAt":"2026-09-13T10:00:00Z"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := predecessor.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = predecessor.Wait()
	self := int64(os.Getpid())
	selfExact, state, err := (identity.KernelProber{}).Probe(self)
	if err != nil || state != identity.Alive {
		t.Fatal(err)
	}
	if _, err := lease.AnnounceWithPair(root, "new-session", self, selfExact.StartedAt.Unix(), selfExact.StartTicks, selfExact.BootID, "wait-new", "fake", ""); err != nil {
		t.Fatal(err)
	}
	if lines, err := report.CurrentWaitingLines(root); err != nil || len(lines) != 1 || !strings.Contains(lines[0], waitID) {
		t.Fatalf("takeover waiting lines=%q err=%v", lines, err)
	}
	originalPID, originalAdapter := waitCallerPID, waitAdapterPathForRuntime
	waitCallerPID = func() int64 { return self }
	waitAdapterPathForRuntime = func(string, string) (string, error) {
		return filepath.Abs(filepath.Join("..", "..", "scripts", "agents", "adapters", "fake.sh"))
	}
	t.Cleanup(func() { waitCallerPID, waitAdapterPathForRuntime = originalPID, originalAdapter })
	code, output, problem := captureChannelOutput(t, func() int { return runWait([]string{"--root", root, "--resume", waitID, "--json"}) })
	var result metarun.WaitResult
	decodeErr := json.Unmarshal([]byte(output), &result)
	if code != 0 || problem != "" || decodeErr != nil || result.WaitID == waitID || result.PointerRepaired {
		t.Fatalf("takeover resume code=%d output=%q problem=%q result=%+v err=%v", code, output, problem, result, decodeErr)
	}
}
