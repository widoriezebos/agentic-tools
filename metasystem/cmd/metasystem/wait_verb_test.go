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
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/report"
	metarun "github.com/widoriezebos/agentic-tools/metasystem/internal/run"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
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

func pendingWaitVerdictCommandFixture(t *testing.T, root, runtimeName string) (string, string) {
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
	job := map[string]any{
		"jobId": "wait-stop-job", "operationId": operationID, "status": "pending-setup", "goalId": "wait-stop-goal",
	}
	jobData, _ := json.Marshal(job)
	if err := os.WriteFile(filepath.Join(root, "artifacts", "agents", "jobs", "wait-stop-job.json"), append(jobData, '\n'), 0o644); err != nil {
		t.Fatal(err)
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
		Target:       metarun.WaiterTarget{OperationID: operationID},
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
		filepath.Join(sourceRoot, "scripts", "agents", "evidence-gc.sh"):      evidenceGC,
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
	wrapper := filepath.Join(t.TempDir(), "metasystem-hook-engine")
	wrapperSource := "#!/bin/sh\nif [ \"${1:-}\" = up ]; then printf '%s\\n' 'up outcome=already-healthy'; exit 0; fi\nexec \"${METASYSTEM_WAIT_REAL_ENGINE:?}\" \"$@\"\n"
	if err := os.WriteFile(wrapper, []byte(wrapperSource), 0o755); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("bash", hook, "fake", "stop")
	environment := make([]string, 0, len(os.Environ())+3)
	for _, value := range os.Environ() {
		if strings.HasPrefix(value, "METASYSTEM_BIN=") || strings.HasPrefix(value, "METASYSTEM_WAIT_REAL_ENGINE=") || strings.HasPrefix(value, "METASYSTEM_FAKE_AGENT_ANCESTOR_PID=") {
			continue
		}
		environment = append(environment, value)
	}
	command.Env = append(environment,
		"METASYSTEM_BIN="+wrapper,
		"METASYSTEM_WAIT_REAL_ENGINE="+canonical,
		"METASYSTEM_FAKE_AGENT_ANCESTOR_PID="+fmt.Sprint(os.Getpid()),
	)
	command.Stdin = strings.NewReader(`{"session_id":"` + hookSession + `","cwd":"` + hookRoot + `","hook_event_name":"Stop"}`)
	hookOutput, err := command.CombinedOutput()
	if err != nil || strings.Contains(string(hookOutput), `"decision":"block"`) {
		t.Fatalf("fake Stop hook did not allow the registered wait: err=%v output=%s", err, hookOutput)
	}
	artifact := filepath.Join(hookRoot, "artifacts", "agents", "supervision", "stop-verdicts", hookSession+".txt")
	artifactData, err := os.ReadFile(artifact)
	if err != nil || !strings.Contains(string(artifactData), "WAITING: registered wait") {
		t.Fatalf("fake Stop hook omitted its registered-wait evidence: %v %s", err, artifactData)
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
