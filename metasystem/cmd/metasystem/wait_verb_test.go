package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hooks"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/report"
	metarun "github.com/widoriezebos/agentic-tools/metasystem/internal/run"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

type waitCommandProber struct {
	pid   int64
	live  bool
	start int64
}

type waitPathFileInfo struct {
	size     int64
	modified time.Time
}

func (info waitPathFileInfo) Name() string       { return "result" }
func (info waitPathFileInfo) Size() int64        { return info.size }
func (info waitPathFileInfo) Mode() os.FileMode  { return 0o600 }
func (info waitPathFileInfo) ModTime() time.Time { return info.modified }
func (info waitPathFileInfo) IsDir() bool        { return false }
func (info waitPathFileInfo) Sys() any           { return nil }

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
	if code, _, problem := captureChannelOutput(t, func(stdout, stderr io.Writer) int { return runWait(nil, stdout, stderr) }); code != metarun.ExitInvalidWait || !strings.Contains(problem, "exactly one") {
		t.Fatalf("missing selector code=%d stderr=%q", code, problem)
	}
	if code, _, problem := captureChannelOutput(t, func(stdout, stderr io.Writer) int {
		return runWait([]string{"--job", "j", "--run", "r"}, stdout, stderr)
	}); code != metarun.ExitInvalidWait || !strings.Contains(problem, "exactly one") {
		t.Fatalf("duplicate selector code=%d stderr=%q", code, problem)
	}
	if code, _, problem := captureChannelOutput(t, func(stdout, stderr io.Writer) int {
		return runWait([]string{"--goal", "goal-a"}, stdout, stderr)
	}); code != metarun.ExitInvalidWait || !strings.Contains(problem, "event must be") {
		t.Fatalf("incomplete goal selector code=%d stderr=%q", code, problem)
	}
	if code, _, problem := captureChannelOutput(t, func(stdout, stderr io.Writer) int {
		return runWait([]string{"--resume", "not-a-wait-id"}, stdout, stderr)
	}); code != metarun.ExitInvalidWait || !strings.Contains(problem, "identifier is invalid") {
		t.Fatalf("invalid resume identifier code=%d stderr=%q", code, problem)
	}
	result := metarun.WaitResult{SchemaVersion: 2, WaitID: strings.Repeat("a", 32), Selector: metarun.WaitSelector{Kind: "run", TargetID: "run-a"}, TargetIncarnation: metarun.WaiterTarget{Generation: 2, LaunchNonce: "n"}, ExitCode: metarun.ExitGreen, Reason: "run completed green", SourceOutcome: "green", SourceEvidence: "run:run-a:g2:n"}
	code, output, problem := captureChannelOutput(t, func(stdout, stderr io.Writer) int { writeWaitResult(stdout, result, true); return 0 })
	if code != 0 || problem != "" {
		t.Fatalf("JSON result code=%d stderr=%q", code, problem)
	}
	var decoded metarun.WaitResult
	if err := json.Unmarshal([]byte(output), &decoded); err != nil || decoded.WaitID != result.WaitID || decoded.TargetIncarnation.Generation != 2 {
		t.Fatalf("JSON result=%q decoded=%+v err=%v", output, decoded, err)
	}
	_, plain, _ := captureChannelOutput(t, func(stdout, stderr io.Writer) int { writeWaitResult(stdout, result, false); return 0 })
	if strings.Count(strings.TrimSpace(plain), "\n") != 0 || !strings.Contains(plain, "run-a") || !strings.Contains(plain, "run:run-a:g2:n") {
		t.Fatalf("plain result is not one complete line: %q", plain)
	}
}

func TestWaitPathSelectorIsValidated(t *testing.T) {
	clean := filepath.Join(string(filepath.Separator), "tmp", "metasystem-path-selector")
	dirty := clean + string(filepath.Separator) + ".." + string(filepath.Separator) + filepath.Base(clean)
	cases := []struct {
		name string
		args []string
		want string
	}{
		{name: "path is one selector", args: []string{"--path", clean, "--until", "present", "--job", "job-a"}, want: "exactly one"},
		{name: "path is absolute", args: []string{"--path", "relative", "--until", "present"}, want: "absolute"},
		{name: "path is clean", args: []string{"--path", dirty, "--until", "present"}, want: "clean"},
		{name: "until is known", args: []string{"--path", clean, "--until", "ready"}, want: "present or absent"},
		{name: "until is required", args: []string{"--path", clean}, want: "--until"},
		{name: "until needs path", args: []string{"--job", "job-a", "--until", "present"}, want: "--until requires --path"},
		{name: "resume replaces no path fields", args: []string{"--resume", strings.Repeat("a", 32), "--until", "present"}, want: "replacement"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			code, _, problem := captureChannelOutput(t, func(stdout, stderr io.Writer) int { return runWait(test.args, stdout, stderr) })
			if code != metarun.ExitInvalidWait || !strings.Contains(problem, test.want) {
				t.Fatalf("code=%d stderr=%q want=%q", code, problem, test.want)
			}
		})
	}
	selector := metarun.WaitSelector{Kind: "path", TargetID: metarun.PathWaitTargetID(clean), Path: clean, Until: "absent"}
	if err := metarun.ValidateWaitSelector(selector); err != nil {
		t.Fatalf("valid path selector: %v", err)
	}
	originalAdapter, originalStat, originalSignature := waitDeliveryRuntime, waitPathStat, waitOpenWorkSignature
	waitDeliveryRuntime = func(string, string) (string, error) { return "fake", nil }
	waitPathStat = func(string) (os.FileInfo, error) {
		return waitPathFileInfo{size: 4, modified: time.Date(2026, 9, 17, 10, 0, 0, 0, time.UTC)}, nil
	}
	waitOpenWorkSignature = func(context.Context, string) (string, error) {
		t.Fatal("path options read the open-work signature")
		return "", nil
	}
	t.Cleanup(func() {
		waitDeliveryRuntime = originalAdapter
		waitPathStat = originalStat
		waitOpenWorkSignature = originalSignature
	})
	selector.Until = "present"
	options, err := waitOptions(t.TempDir(), selector, metarun.Caller{}, "fake", nil)
	if err != nil {
		t.Fatal(err)
	}
	observation, err := options.Observe(context.Background(), selector, metarun.WaiterTarget{}, "")
	if err != nil || observation.Pending || observation.Evidence == "" || options.OpenHintReceiver != nil || options.Actionable != nil {
		t.Fatalf("path options observation=%+v hint=%t actionable=%t err=%v", observation, options.OpenHintReceiver != nil, options.Actionable != nil, err)
	}
	if signature, err := options.OpenWorkSignature(context.Background()); err != nil || signature != "" {
		t.Fatalf("path open-work baseline=%q err=%v", signature, err)
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
	originalPID, originalAdapter := waitCallerPID, waitDeliveryRuntime
	waitCallerPID = func() int64 { return self }
	waitDeliveryRuntime = func(string, string) (string, error) { return "fake", nil }
	t.Cleanup(func() { waitCallerPID, waitDeliveryRuntime = originalPID, originalAdapter })
	// A refusal before the wait starts is the wait's result, printed where
	// the caller reads every outcome.
	code, output, problem := captureChannelOutput(t, func(stdout, stderr io.Writer) int {
		return runWait([]string{"--root", root, "--job", "job-unassociated"}, stdout, stderr)
	})
	rows, _ := filepath.Glob(filepath.Join(metarun.WaitersDir(root), "*.json"))
	if code != metarun.ExitWaiterBusy || !strings.Contains(output, "authenticated runtime session") || len(rows) != 0 {
		t.Fatalf("unassociated registration code=%d stdout=%q stderr=%q rows=%v", code, output, problem, rows)
	}
	if err := lease.AssociateSession(root, mainID, "session-associated", "start", "clear"); err != nil {
		t.Fatal(err)
	}
	if code, _, problem = captureChannelOutput(t, func(stdout, stderr io.Writer) int {
		return runWait([]string{"--root", root, "--job", "job-unassociated"}, stdout, stderr)
	}); code != 0 || problem != "" {
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
	original := waitDeliveryRuntime
	waitDeliveryRuntime = func(string, string) (string, error) { return "fake", nil }
	t.Cleanup(func() { waitDeliveryRuntime = original })
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
		if err := testexec.WriteFile(path, []byte(body), mode); err != nil {
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
	originalAdapter := waitDeliveryRuntime
	waitDeliveryRuntime = func(string, string) (string, error) { return "fake", nil }
	originalSignature := waitOpenWorkSignature
	originalCurrentHolder := waitCurrentHolder
	t.Cleanup(func() {
		waitDeliveryRuntime = originalAdapter
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
	ctx, cancel := context.WithDeadline(context.Background(), time.Unix(1, 0))
	defer cancel()
	if _, _, actionErr := options.Actionable(ctx, row); !errors.Is(actionErr, context.DeadlineExceeded) {
		t.Fatalf("actionable read did not honor the expired context: %v", actionErr)
	}
	waitOpenWorkSignature = originalSignature
	waitCurrentHolder = func(ctx context.Context, _ string) (lease.CurrentHolderView, error) {
		<-ctx.Done()
		return lease.CurrentHolderView{}, ctx.Err()
	}
	ctx, cancel = context.WithDeadline(context.Background(), time.Unix(1, 0))
	defer cancel()
	if _, _, actionErr := options.Actionable(ctx, row); !errors.Is(actionErr, context.DeadlineExceeded) {
		t.Fatalf("checkout-holder read did not honor the expired context: %v", actionErr)
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
	code, _, problem := captureChannelOutput(t, func(stdout, stderr io.Writer) int {
		return runWait([]string{"--root", root, "--resume", waitID}, stdout, stderr)
	})
	if code != metarun.ExitWaiterBusy || !strings.Contains(problem, "metasystem work wait wait:"+waitID) {
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
	originalAdapterPath := waitDeliveryRuntime
	waitDeliveryRuntime = func(string, string) (string, error) { return "fake", nil }
	t.Cleanup(func() {
		waitCallerPID = originalCallerPID
		waitDeliveryRuntime = originalAdapterPath
	})
	code, output, problem := 0, "", ""
	{
		code, output, problem = captureChannelOutput(t, func(stdout, stderr io.Writer) int {
			return runWait([]string{"--root", root, "--run", "run-command", "--timeout", "1m", "--json"}, stdout, stderr)
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
		testutil.InstalledWaitBinary(t, binary)
		cmd := exec.Command(commandTestExecutable(t), waitHelperCommand, "--root", root, "--job", "job-command", "--timeout", "1m", "--json")
		cmd.Env = append(os.Environ(), "GO_WANT_BATCH_E2E_COMMAND=1")
		data, commandErr := cmd.CombinedOutput()
		if commandErr != nil || !strings.Contains(string(data), `"exitCode":0`) {
			t.Fatalf("installed job wait output=%s err=%v", data, commandErr)
		}
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatal(err)
		}
		if marker, err := os.OpenFile(filepath.Join(root, "metasystem.conf"), os.O_CREATE|os.O_WRONLY, 0o644); err != nil {
			t.Fatal(err)
		} else {
			marker.Close()
		}
		if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("dispatch.cap-max=120\nmetasystem.runtimes=fake\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		pinProofBinaryFixture(t, root)
		// The synthetic proof reservation must not contend with the proof run
		// executing this test; the fake-runtime root authorizes this temp slot.
		t.Setenv("METASYSTEM_PROOF_ADMISSION_TEST_DIR", filepath.Join(root, "proof-admission"))
		t.Setenv("METASYSTEM_PROOF_ADMISSION_FIXTURE_ROOT", root)
		for _, name := range []string{"testing-coverage-floors.json", "testing-coverage-floors-linux.json"} {
			if err := os.WriteFile(filepath.Join(root, name), []byte(`{"floors":{"internal/proofrun":1},"exempt":{}}`), 0o600); err != nil {
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
		proofAttempt, decision, err := proofrun.ReserveLocked(candidateProofAdmission(proofrun.AdmissionRequest{ControlRoot: root, ExecutionRoot: root, GoalID: "goal-a", GoalRevision: 1, AccountingRevision: 1, ReservedMinutes: 1, Identity: proofIdentity, Launcher: launcher, Now: time.Now().UTC()}))
		if err != nil {
			t.Fatal(err)
		}
		requireProofReservationNotAdmissionRefused(t, decision)
		if _, err := proofrun.FinalizeAttempt(root, proofAttempt.AttemptID, proofrun.TerminalSuccess, 0, "installed proof terminal", nil, time.Now().UTC()); err != nil {
			t.Fatal(err)
		}
		cmd = exec.Command(commandTestExecutable(t), waitHelperCommand, "--root", root, "--attempt", proofAttempt.AttemptID, "--timeout", "1m", "--json")
		cmd.Env = append(os.Environ(), "GO_WANT_BATCH_E2E_COMMAND=1")
		data, commandErr = cmd.CombinedOutput()
		if commandErr != nil || !strings.Contains(string(data), `"exitCode":0`) {
			t.Fatalf("installed proof wait output=%s err=%v", data, commandErr)
		}
	} else {
		code, data, problem := captureChannelOutput(t, func(stdout, stderr io.Writer) int {
			return runWait([]string{"--root", root, "--job", "job-command", "--timeout", "1m", "--json"}, stdout, stderr)
		})
		if code != 0 || problem != "" || !strings.Contains(data, `"exitCode":0`) {
			t.Fatalf("job wait code=%d output=%s problem=%s", code, data, problem)
		}
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
	code, output, problem := captureChannelOutput(t, func(stdout, stderr io.Writer) int {
		return sessionStartRecovery(root, "session-new", stdout, stderr)
	})
	want := "WAITING attempt attempt-a until 2026-09-14T12:00:00Z: metasystem work wait wait:" + strings.Repeat("a", 32)
	if code != 0 || strings.TrimSpace(output) != want || problem != "" {
		t.Fatalf("session start code=%d output=%q stderr=%q", code, output, problem)
	}
	if code, next, problem := captureChannelOutput(t, func(stdout, stderr io.Writer) int { return runGoalNext([]string{"--root", root}, stdout, stderr) }); code != 0 || problem != "" || !strings.Contains(next, want) {
		t.Fatalf("goal next code=%d output=%q stderr=%q", code, next, problem)
	}
	code, verdictOutput, problem := captureChannelOutput(t, func(stdout, stderr io.Writer) int {
		return runReportTurnVerdict([]string{"--root", root, "--session", "session-new", "--main-id", mainID}, stdout, stderr)
	})
	var verdict struct {
		Display string `json:"display"`
	}
	if code != 0 || problem != "" || json.Unmarshal([]byte(verdictOutput), &verdict) != nil || strings.Contains(verdict.Display, want) {
		t.Fatalf("turn verdict code=%d output=%q stderr=%q display=%q", code, verdictOutput, problem, verdict.Display)
	}
	if binary := os.Getenv("METASYSTEM_WAIT_BINARY"); binary != "" {
		testutil.InstalledWaitBinary(t, binary)
		if output, err := exec.Command("git", "-C", root, "init", "-q").CombinedOutput(); err != nil {
			t.Fatalf("initialize hook fixture repository: %v %s", err, output)
		}
		// The hook resolves state from its own installation, so the fixture's
		// durable wait row, hook, and canonical engine belong to one world. The
		// synthetic runtime intentionally has neither a staged signature adapter
		// nor a start-context channel; the announced holder must still recover its
		// durable wait row through the hook's system message.
		canonicalEngine := filepath.Join(root, "bin", "metasystem")
		// metasystem.conf marks the installation the state root resolves.
		if err := os.MkdirAll(filepath.Dir(canonicalEngine), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
		engineBytes, err := os.ReadFile(binary)
		if err != nil {
			t.Fatal(err)
		}
		if err := testexec.WriteFile(canonicalEngine, engineBytes, 0o755); err != nil {
			t.Fatal(err)
		}
		wrapper := filepath.Join(t.TempDir(), "metasystem-hook-engine")
		wrapperSource := "#!/bin/sh\nif [ \"${1:-}\" = up ]; then printf '%s\\n' 'UP SUCCESS'; exit 0; fi\nexec \"${METASYSTEM_WAIT_REAL_ENGINE:?}\" \"$@\"\n"
		if err := testexec.WriteFile(wrapper, []byte(wrapperSource), 0o755); err != nil {
			t.Fatal(err)
		}
		command := directHookCommand(wrapper, root)
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
		nonHolder := directHookCommand(wrapper, root)
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
			withoutLease := directHookCommand(wrapper, root)
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
	for name, invoke := range map[string]func(stdout, stderr io.Writer) int{
		"session start": func(stdout, stderr io.Writer) int { return sessionStartRecovery(root, "session-new", stdout, stderr) },
		"goal next":     func(stdout, stderr io.Writer) int { return runGoalNext([]string{"--root", root}, stdout, stderr) },
		"turn verdict": func(stdout, stderr io.Writer) int {
			return runReportTurnVerdict([]string{"--root", root, "--session", "session-new", "--main-id", mainID}, stdout, stderr)
		},
	} {
		_, cleared, _ := captureChannelOutput(t, invoke)
		if strings.Contains(cleared, "WAITING attempt attempt-a") {
			t.Fatalf("%s advertised a cleared row: %q", name, cleared)
		}
	}
	if code, _, _ := captureChannelOutput(t, func(stdout, stderr io.Writer) int {
		return sessionStartRecovery(root, "another-session", stdout, stderr)
	}); code != metarun.ExitWaiterBusy {
		t.Fatalf("another session received this holder's recovery rows: exit=%d", code)
	}
}

func TestWaitSessionStartWithoutLeaseReturnsBusy(t *testing.T) {
	code, output, problem := captureChannelOutput(t, func(stdout, stderr io.Writer) int {
		return sessionStartRecovery(t.TempDir(), "session-without-lease", stdout, stderr)
	})
	if code != metarun.ExitWaiterBusy || output != "" || problem != "" {
		t.Fatalf("session start without lease code=%d output=%q stderr=%q", code, output, problem)
	}
}

type pendingWaitVerdictCommandOptions struct {
	jobStatus            string
	writeWaiter          bool
	requireHook          bool
	requireChildShellGit bool
}

// installPendingWaitGit provides only the Git answers used by an ordinary
// checkout before its accepted goal reference exists. Every other command is
// recorded and refused, including commands from a different working directory.
func installPendingWaitGit(t *testing.T, root string, requireHook, requireChildShellGit bool) {
	t.Helper()
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	hookCwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	hookCwd, err = filepath.EvalSymlinks(hookCwd)
	if err != nil {
		t.Fatal(err)
	}
	shimDir := t.TempDir()
	supported := filepath.Join(shimDir, "supported.log")
	unexpected := filepath.Join(shimDir, "unexpected.log")
	for _, path := range []string{supported, unexpected} {
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	script := "#!/bin/sh\nGORACE=atexit_sleep_ms=0\nexport GORACE\nexec \"$METASYSTEM_WAIT_GIT_HELPER\" -test.run=^TestPendingWaitGitHelper$ -- \"$@\"\n"
	if err := testexec.WriteFile(filepath.Join(shimDir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("METASYSTEM_WAIT_GIT_ROOT", canonicalRoot)
	t.Setenv("METASYSTEM_WAIT_GIT_HELPER", os.Args[0])
	t.Setenv("METASYSTEM_WAIT_GIT_HOOK_CWD", hookCwd)
	t.Setenv("METASYSTEM_WAIT_GIT_SUPPORTED", supported)
	t.Setenv("METASYSTEM_WAIT_GIT_UNEXPECTED", unexpected)
	childShell := "0"
	if requireChildShellGit {
		childShell = "1"
	}
	t.Setenv("METASYSTEM_WAIT_GIT_CHILD_SHELL", childShell)
	t.Setenv("PATH", shimDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Cleanup(func() {
		calls, callsErr := os.ReadFile(supported)
		denied, deniedErr := os.ReadFile(unexpected)
		if callsErr != nil || deniedErr != nil {
			t.Errorf("read pending-wait Git transcript: supported=%v unexpected=%v", callsErr, deniedErr)
			return
		}
		t.Logf("pending-wait Git supported calls root=%s:\n%s", canonicalRoot, calls)
		if len(denied) != 0 {
			t.Errorf("pending-wait Git unexpected calls root=%s:\n%s", canonicalRoot, denied)
		}
		transcript := string(calls)
		if !strings.Contains(transcript, "cwd="+canonicalRoot+" arg=<rev-parse> arg=<--verify> arg=<--quiet> arg=<refs/metasystem/goals/accepted>") {
			t.Errorf("pending-wait Git accepted-ref probe missing for %s", canonicalRoot)
		}
		if !strings.Contains(transcript, "cwd="+canonicalRoot+" arg=<config> arg=<--get> arg=<metasystem.goal.machine>") {
			t.Errorf("pending-wait Git machine-config probe missing for %s", canonicalRoot)
		}
		if requireHook && !strings.Contains(transcript, "cwd="+hookCwd+" arg=<-C> arg=<"+canonicalRoot+"> arg=<rev-parse> arg=<--path-format=absolute> arg=<--git-dir> arg=<--git-common-dir>") {
			t.Errorf("pending-wait Git hook checkout probe missing for %s", canonicalRoot)
		}
		if requireHook && !strings.Contains(transcript, "cwd="+hookCwd+" arg=<-C> arg=<"+canonicalRoot+"> arg=<rev-parse> arg=<--show-toplevel>") {
			t.Errorf("pending-wait Git repository-top probe missing for %s", canonicalRoot)
		}
	})
}

type pendingWaitGitReply struct {
	stdout, stderr string
	status         int
}

// TestPendingWaitGitHelper answers only the Git reads admitted by the wait fixture.
// Each invocation records its decision before returning to the installed command.
func TestPendingWaitGitHelper(t *testing.T) {
	if os.Getenv("METASYSTEM_WAIT_GIT_HELPER") == "" {
		return
	}
	var argv []string
	for i, arg := range os.Args {
		if arg == "--" {
			argv = os.Args[i+1:]
			break
		}
	}
	cwd, cwdErr := os.Getwd()
	if cwdErr == nil {
		cwd, cwdErr = filepath.EvalSymlinks(cwd)
	}
	call := "cwd=" + cwd
	for _, arg := range argv {
		call += " arg=<" + arg + ">"
	}
	root := os.Getenv("METASYSTEM_WAIT_GIT_ROOT")
	hookCwd := os.Getenv("METASYSTEM_WAIT_GIT_HOOK_CWD")
	childShell := os.Getenv("METASYSTEM_WAIT_GIT_CHILD_SHELL") == "1"
	reply, supported := pendingWaitGitAnswer(root, hookCwd, cwd, childShell, argv)
	if cwdErr != nil || len(argv) == 0 || root == "" || hookCwd == "" {
		supported = false
	}
	logPath := os.Getenv("METASYSTEM_WAIT_GIT_UNEXPECTED")
	if supported {
		logPath = os.Getenv("METASYSTEM_WAIT_GIT_SUPPORTED")
	}
	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		fmt.Fprintln(os.Stderr, "record pending-wait Git call:", err)
		os.Exit(97)
	}
	_, writeErr := fmt.Fprintln(file, call)
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		fmt.Fprintln(os.Stderr, "record pending-wait Git call:", writeErr, closeErr)
		os.Exit(97)
	}
	if !supported {
		fmt.Fprintln(os.Stderr, "unexpected pending-wait git call:", call)
		os.Exit(97)
	}
	fmt.Fprint(os.Stdout, reply.stdout)
	fmt.Fprint(os.Stderr, reply.stderr)
	os.Exit(reply.status)
}

func pendingWaitGitAnswer(root, hookCwd, cwd string, childShell bool, argv []string) (pendingWaitGitReply, bool) {
	answer := func(stdout, stderr string, status int) (pendingWaitGitReply, bool) {
		return pendingWaitGitReply{stdout: stdout, stderr: stderr, status: status}, true
	}
	if cwd == hookCwd && len(argv) >= 3 && argv[0] == "-C" {
		commandRoot, err := filepath.EvalSymlinks(argv[1])
		if err == nil && commandRoot == root {
			args := argv[2:]
			switch {
			case slices.Equal(args, []string{"rev-parse", "--path-format=absolute", "--git-dir", "--git-common-dir"}):
				gitDir := filepath.Join(root, ".git") + "\n"
				return answer(gitDir+gitDir, "", 0)
			case slices.Equal(args, []string{"config", "--get", "metasystem.goal.machine"}):
				return answer("", "", 1)
			case slices.Equal(args, []string{"rev-parse", "--show-toplevel"}):
				return answer(root+"\n", "", 0)
			}
			if childShell {
				switch {
				case slices.Equal(args, []string{"rev-parse", "--git-common-dir"}),
					slices.Equal(args, []string{"rev-parse", "--git-dir"}):
					return answer(".git\n", "", 0)
				case slices.Equal(args, []string{"config", "--get", "metasystem.steward.tick-seconds"}),
					slices.Equal(args, []string{"config", "--get", "metasystem.steward.notify-command"}),
					slices.Equal(args, []string{"rev-parse", "--verify", "--quiet", "refs/metasystem/goals/accepted"}):
					return answer("", "", 1)
				case slices.Equal(args, []string{"rev-parse", "HEAD"}):
					return answer("HEAD\n", "fatal: ambiguous argument 'HEAD': unknown revision or path not in the working tree.\nUse '--' to separate paths from revisions, like this:\n'git <command> [<revision>...] -- [<file>...]'\n", 128)
				// The real SessionStart starts a resident steward whose first
				// tick may read fleet presence before disarm. This checkout has
				// no remote, so the fetch fails as Git would, and both copied
				// presence namespaces are empty.
				case slices.Equal(args, []string{"-c", "core.logAllRefUpdates=false", "fetch", "--no-tags", "--refmap=", "--atomic", "--prune", "origin", "+refs/metasystem/presence/*:refs/metasystem/presence-copy/metasystem/*", "+refs/heads/presence/*:refs/metasystem/presence-copy/heads/*"}):
					return answer("", "fatal: 'origin' does not appear to be a git repository\nfatal: Could not read from remote repository.\n\nPlease make sure you have the correct access rights\nand the repository exists.\n", 128)
				case slices.Equal(args, []string{"-c", "core.logAllRefUpdates=false", "for-each-ref", "--format=%(refname)", "refs/metasystem/presence-copy/metasystem"}),
					slices.Equal(args, []string{"-c", "core.logAllRefUpdates=false", "for-each-ref", "--format=%(refname)", "refs/metasystem/presence-copy/heads"}):
					return answer("", "", 0)
				}
				pinned := []string{"-c", "core.fileMode=true", "-c", "diff.noprefix=false", "-c", "diff.mnemonicPrefix=false", "-c", "apply.ignoreWhitespace=no", "-c", "core.logAllRefUpdates=false", "-c", "core.useReplaceRefs=false", "-c", "gc.auto=0", "-c", "maintenance.auto=false", "rev-parse"}
				if len(args) >= len(pinned) && slices.Equal(args[:len(pinned)], pinned) {
					switch {
					case slices.Equal(args[len(pinned):], []string{"--show-toplevel"}):
						return answer(root+"\n", "", 0)
					case slices.Equal(args[len(pinned):], []string{"--show-prefix"}):
						return answer("\n", "", 0)
					case slices.Equal(args[len(pinned):], []string{"--git-dir"}):
						return answer(".git\n", "", 0)
					case slices.Equal(args[len(pinned):], []string{"--verify", "--quiet", "HEAD^{commit}"}):
						return answer("", "", 1)
					}
				}
			}
		}
	}
	if cwd == root {
		switch {
		case childShell && slices.Equal(argv, []string{"ls-tree", "-r", "--name-only", "HEAD", "--", "plans/goals/", "records/goals/"}):
			return answer("", "fatal: Not a valid object name HEAD\n", 128)
		case slices.Equal(argv, []string{"rev-parse", "--verify", "--quiet", "refs/metasystem/goals/accepted"}),
			slices.Equal(argv, []string{"rev-parse", "--verify", "--quiet", "refs/metasystem/goals/accepted^{commit}"}),
			slices.Equal(argv, []string{"show-ref", "--verify", "--quiet", "refs/metasystem/goals/accepted"}),
			slices.Equal(argv, []string{"config", "--get", "goal.sync-remote"}),
			slices.Equal(argv, []string{"config", "--get", "goal.sync-branch"}),
			slices.Equal(argv, []string{"config", "--get", "metasystem.goal.machine"}):
			return answer("", "", 1)
		case slices.Equal(argv, []string{"rev-parse", "--path-format=absolute", "--git-common-dir"}):
			return answer(filepath.Join(root, ".git")+"\n", "", 0)
		}
	}
	return pendingWaitGitReply{}, false
}

type pendingWaitCommandFixture struct {
	session      string
	mainID       string
	ownerLineage string
}

func pendingWaitVerdictCommandFixture(t *testing.T, root, runtimeName string) (string, string) {
	t.Helper()
	fixture := pendingWaitVerdictCommandFixtureWithOptions(t, root, runtimeName, pendingWaitVerdictCommandOptions{
		jobStatus: "pending-setup", writeWaiter: true,
	})
	return fixture.session, fixture.mainID
}

func pendingWaitVerdictCommandFixtureWithOptions(t *testing.T, root, runtimeName string, options pendingWaitVerdictCommandOptions) pendingWaitCommandFixture {
	t.Helper()
	installPendingWaitGit(t, root, options.requireHook, options.requireChildShellGit)
	if options.requireChildShellGit {
		if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	fixtureNow := time.Now().UTC().Truncate(time.Second)
	fixtureBootElapsed := 2 * time.Hour
	t.Setenv("METASYSTEM_GOAL_NOW", fixtureNow.Format(time.RFC3339))
	t.Setenv("METASYSTEM_GOAL_BOOT_ID", "wait-verdict-fixture-boot")
	t.Setenv("METASYSTEM_GOAL_BOOT_NANOS", strconv.FormatInt(fixtureBootElapsed.Nanoseconds(), 10))
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
	fixture := pendingWaitCommandFixture{session: session, ownerLineage: lineage}
	announcement, err := lease.AnnounceWithPair(root, session, self, exact.StartedAt.Unix(), exact.StartTicks, exact.BootID, "wait-stop-fixture", runtimeName, lineage)
	if err != nil {
		t.Fatal(err)
	}
	mainID := announcedMainID(t, announcement)
	fixture.mainID = mainID
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
		startedAt := fixtureNow.Add(-time.Second).Format(time.RFC3339Nano)
		job["mainId"], job["round"], job["startedAt"] = mainID, 1, startedAt
		target.Round, target.StartedAt = 1, startedAt
	}
	jobData, _ := json.Marshal(job)
	if err := os.WriteFile(filepath.Join(root, "artifacts", "agents", "jobs", "wait-stop-job.json"), append(jobData, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	if !options.writeWaiter {
		return fixture
	}
	signature, err := report.OpenWorkSignature(context.Background(), root)
	if err != nil || signature == "" {
		t.Fatalf("read fixture open-work signature: %q %v", signature, err)
	}
	bootID, elapsed, err := goalCommandBootClock(root)
	if err != nil || bootID == "" {
		t.Fatalf("read fixture boot clock: %q %s %v", bootID, elapsed, err)
	}
	// The verb under test reads "now" through the fixture clock authority, which
	// honours METASYSTEM_GOAL_NOW on a fake-runtime root like this one. Stamping
	// the row from the wall clock instead left the deadline unreachable whenever
	// an ambient fixture clock was set, and the real-origin rehearsal sets one
	// process-wide before it drives any verb, so its dependent-package gate ran
	// this fixture against a "now" seven hours in the past.
	now, err := goalCommandNow(root)
	if err != nil {
		t.Fatal(err)
	}
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
	return fixture
}

type installedHookResult struct {
	stdout string
	stderr string
	up     *hooks.UpRequest
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
	if err := testexec.WriteFile(target, data, 0o755); err != nil {
		t.Fatal(err)
	}
}

func installPendingWaitHookFixture(t *testing.T, root, binary string) (hook, canonical string, owners *pendingWaitHookOwners) {
	t.Helper()
	// The runtime settings run the engine's hook entry in the installation.
	hook = root
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
	return hook, canonical, &pendingWaitHookOwners{hookOwners: hookOwners{diagnostics: io.Discard}, installed: canonical}
}

type installedWaitFixture struct {
	*testutil.ProcessFixture
	ownerLineage string
}

func newInstalledWaitFixture(t *testing.T, ownerLineage string) *installedWaitFixture {
	t.Helper()
	return &installedWaitFixture{ProcessFixture: testutil.Fixture(t), ownerLineage: ownerLineage}
}

func (fixture *installedWaitFixture) ownerEnvironment(base []string) []string {
	environment := make([]string, 0, len(base)+1)
	for _, value := range base {
		if !strings.HasPrefix(value, "METASYSTEM_OWNER_LINEAGE=") {
			environment = append(environment, value)
		}
	}
	return append(environment, "METASYSTEM_OWNER_LINEAGE="+fixture.ownerLineage)
}

func (fixture *installedWaitFixture) Env(base []string) []string {
	return fixture.ProcessFixture.Env(fixture.ownerEnvironment(base))
}

func TestInstalledWaitFixtureReplacesInheritedOwnerLineage(t *testing.T) {
	t.Parallel()
	fixture := installedWaitFixture{ownerLineage: "wait-stop-lineage"}
	environment := fixture.ownerEnvironment([]string{
		"PATH=/fixture/bin",
		"METASYSTEM_OWNER_LINEAGE=diagnostic-hostile-lineage",
	})
	var lineages []string
	for _, value := range environment {
		if strings.HasPrefix(value, "METASYSTEM_OWNER_LINEAGE=") {
			lineages = append(lineages, value)
		}
	}
	if len(lineages) != 1 || lineages[0] != "METASYSTEM_OWNER_LINEAGE="+fixture.ownerLineage {
		t.Fatalf("installed wait environment owner lineages=%q", lineages)
	}
}

func runRecordedInstalledWaitCommand(fixture *installedWaitFixture, command *exec.Cmd) ([]byte, error) {
	var output bytes.Buffer
	command.Stdout, command.Stderr = &output, &output
	if err := command.Start(); err != nil {
		return output.Bytes(), err
	}
	fixture.Record(command.Process.Pid)
	err := command.Wait()
	if holdErr := fixture.HoldOwnedChildren(); holdErr != nil {
		err = errors.Join(err, holdErr)
	}
	return append([]byte(nil), output.Bytes()...), err
}

func installPendingWaitSupervisionCleanup(t *testing.T, fixture *installedWaitFixture, canonical, root string) func() error {
	t.Helper()
	stopped := false
	stop := func() error {
		if stopped {
			return nil
		}
		environment := make([]string, 0, len(os.Environ())+1)
		for _, value := range os.Environ() {
			if !strings.HasPrefix(value, "METASYSTEM_FIXTURE_CAP_SCALE_MILLI=") {
				environment = append(environment, value)
			}
		}
		environment = fixture.Env(append(environment, "METASYSTEM_FIXTURE_CAP_SCALE_MILLI=1"))
		if err := retireUnpublishedPendingWaitOwner(root, environment); err != nil {
			return err
		}
		var failures []error
		for _, arguments := range [][]string{
			{"up", "--metasystem-root", root, "--repo", root, "--shutdown"},
		} {
			command := exec.Command(canonical, arguments...)
			command.Env = environment
			output, err := runRecordedInstalledWaitCommand(fixture, command)
			if err != nil {
				failures = append(failures, fmt.Errorf("%s: %w: %s", strings.Join(arguments, " "), err, output))
			}
		}
		// The steward's disarm owner, which the retired internal steward
		// disarm ran.
		if _, err := steward.Disarm(root); err != nil {
			failures = append(failures, fmt.Errorf("steward disarm: %w", err))
		}
		if len(failures) == 0 {
			stopped = true
		}
		return errors.Join(failures...)
	}
	t.Cleanup(func() {
		if err := stop(); err != nil {
			t.Errorf("stop installed supervision: %v", err)
		}
	})
	return stop
}

// retireUnpublishedPendingWaitOwner settles the one owner state up --shutdown
// refuses: a live owner the registry does not yet name ("with no registry
// checkout"). The fixture's SessionStart arms with a one-second scaled budget,
// so on a loaded machine the recorded owner can still be unscheduled, before
// its write-ahead relaunched row, when the stop runs. The owner is frozen
// first, so the registry read and the decision cannot race its append. If the
// row exists, the owner resumes and the orderly shutdown stops it. If not, the
// owner has launched no component (the row precedes every launch), so the
// fixture ends its own process and awaits that exact exit; the shutdown then
// takes the recorded dead-owner path. The production refusal is unchanged.
func retireUnpublishedPendingWaitOwner(root string, environment []string) error {
	owner, err := supervise.ReadArmingOwner(root)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read the recorded supervision owner before shutdown: %w", err)
	}
	registryPath := ""
	for _, entry := range environment {
		if value, ok := strings.CutPrefix(entry, "METASYSTEM_SUPERVISION_REGISTRY_HOME="); ok {
			registryPath = filepath.Join(value, ".metasystem", "armed-checkouts.jsonl")
		}
	}
	if registryPath == "" {
		if registryPath, err = registry.DefaultPath(); err != nil {
			return err
		}
	}
	prober := identity.KernelProber{}
	ref := identity.Ref{Pid: owner.Pid, StartedAtSec: owner.PidStartedAt, StartTicks: owner.PidStartTicks, BootID: owner.BootID}
	if identity.AliveTaggedRef(prober, ref, owner.InstanceTag) != identity.Alive {
		return nil
	}
	if err := syscall.Kill(int(owner.Pid), syscall.SIGSTOP); err != nil {
		return nil // already gone: the shutdown's dead-owner path applies
	}
	// A stopped process keeps its pid, so this probe decides whether the
	// frozen process is still the recorded owner.
	if identity.AliveTaggedRef(prober, ref, owner.InstanceTag) != identity.Alive {
		_ = syscall.Kill(int(owner.Pid), syscall.SIGCONT)
		return nil
	}
	_, published, readErr := registry.OwnerCheckoutPath(registryPath, owner.InstanceTag)
	if readErr != nil || published {
		_ = syscall.Kill(int(owner.Pid), syscall.SIGCONT)
		return nil
	}
	if err := syscall.Kill(int(owner.Pid), syscall.SIGKILL); err != nil {
		return fmt.Errorf("end the unpublished supervision owner %s (pid %d): %w", owner.InstanceTag, owner.Pid, err)
	}
	if err := testutil.AwaitExactExit(prober, ref); err != nil {
		return fmt.Errorf("unpublished supervision owner %s did not exit: %w", owner.InstanceTag, err)
	}
	return nil
}

// pendingWaitHealthyPreview is the healthy hook preview the installed wait
// fixtures substitute for the health owner, as their wrapper engine did.
const pendingWaitHealthyPreview = `{"schemaVersion":1,"exitCode":0,"line":"HEALTH healthy — ","interventions":[],"verdict":{"schema":1,"observedAt":"2026-09-18T10:00:00Z","observation":1,"aggregate":"healthy","roles":[],"shouldAlert":false,"findingDigest":""}}`

// pendingWaitHookOwners runs the production hook owners with the fixture's
// substitutions: the health preview is healthy, and up either arms for real
// while its request is recorded or answers already-healthy without arming.
type pendingWaitHookOwners struct {
	hookOwners
	installed  string
	fakeUp     bool
	upRequests []hooks.UpRequest
}

// EngineBehind keeps the generation cutover out of these runs: the fixture
// installation's engine is the one under test, never behind its sources.
func (pendingWaitHookOwners) EngineBehind(string, string) (bool, error) { return false, nil }

// EvidenceGC keeps the fixture installation's evidence collection inert, as
// its stub evidence-gc.sh did: the test process is not an authenticated main.
func (pendingWaitHookOwners) EvidenceGC(string, io.Writer) int { return 0 }

func (o *pendingWaitHookOwners) Up(request hooks.UpRequest, stdout, stderr io.Writer) int {
	o.upRequests = append(o.upRequests, request)
	if o.fakeUp {
		fmt.Fprintln(stdout, "up outcome=already-healthy")
		return 0
	}
	// The installed engine arms, as the wrapper's pass-through did: this
	// test binary is not the enrolled engine.
	arguments := []string{"up", "--metasystem-root", request.MetasystemRoot, "--repo", request.Repo,
		"--session", request.Session, "--pid", request.Pid, "--start-time", request.StartTime, "--tag", request.Tag}
	if request.NoRuntimeSession {
		arguments = append(arguments, "--no-runtime-session")
	} else {
		arguments = append(arguments, "--runtime-session", request.RuntimeSession)
	}
	if request.StartSource != "" {
		arguments = append(arguments, "--start-source", request.StartSource)
	}
	command := exec.Command(o.installed, arguments...)
	command.Env = append(os.Environ(), "METASYSTEM_AGENT_RUNTIME="+request.Runtime)
	command.Stdout, command.Stderr = stdout, stderr
	if err := command.Run(); err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			return exitError.ExitCode()
		}
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func (o *pendingWaitHookOwners) HealthPreview(string, string) (string, int) {
	return pendingWaitHealthyPreview + "\n", 0
}

// pendingWaitHookEnvironment is the environment the hook runs with: this
// test process is the fake agent that owns the session.
func pendingWaitHookEnvironment(now string) map[string]string {
	environment := map[string]string{
		"METASYSTEM_HOOK_DELEGATE_STATE_ROOT": "", "METASYSTEM_HOOK_DELEGATE_INSTALLATION_ROOT": "", "METASYSTEM_HOOK_DELEGATE_JOB": "",
		"METASYSTEM_BIN": "", "METASYSTEM_FAKE_PROCESS_IDENTITY_FILE": "", "METASYSTEM_CENSUS_PROCESS_FILE": "",
		"METASYSTEM_FAKE_AGENT_ANCESTOR_PID": fmt.Sprint(os.Getpid()),
		"METASYSTEM_FIXTURE_CAP_SCALE_MILLI": "1",
		"METASYSTEM_GOAL_NOW":                now,
	}
	return environment
}

// runPendingWaitHook runs the runtime hook entry in this process: the
// fixture's environment is this test's, the fake agent is this process, and
// a Stop runs its worker path directly.
func runPendingWaitHook(t *testing.T, fixture *installedWaitFixture, hook string, owners *pendingWaitHookOwners, event, payload, label, now string) installedHookResult {
	t.Helper()
	environment := pendingWaitHookEnvironment(now)
	if fixture != nil {
		for _, entry := range fixture.Env(nil) {
			name, value, _ := strings.Cut(entry, "=")
			environment[name] = value
		}
	}
	environment["METASYSTEM_STOP_DEADLINE_PARENT"] = ""
	if event == "stop" {
		environment["METASYSTEM_STOP_DEADLINE_PARENT"] = fmt.Sprint(os.Getpid())
	}
	for name, value := range environment {
		t.Setenv(name, value)
	}
	var stdout, stderr bytes.Buffer
	before := len(owners.upRequests)
	status := hooks.RunRuntimeHook(hooks.Invocation{
		Runtime: "fake", Event: event, Stdin: strings.NewReader(payload), Stdout: &stdout, Stderr: &stderr,
		Lookup: os.LookupEnv, Pid: os.Getpid(), Ppid: os.Getpid(), Installation: hook,
		Now: time.Now, Environ: os.Environ, TempDir: t.TempDir(),
	}, owners)
	var holdErr error
	if fixture != nil {
		holdErr = fixture.HoldOwnedChildren()
	}
	if status != 0 || holdErr != nil {
		t.Fatalf("%s hook failed: status=%d hold=%v stdout=%s stderr=%s", label, status, holdErr, stdout.String(), stderr.String())
	}
	result := installedHookResult{stdout: stdout.String(), stderr: stderr.String()}
	if len(owners.upRequests) > before {
		result.up = &owners.upRequests[len(owners.upRequests)-1]
	}
	return result
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
	code, output, problem := captureChannelOutput(t, func(stdout, stderr io.Writer) int {
		return runReportTurnVerdict([]string{"--root", root, "--session", session, "--main-id", mainID}, stdout, stderr)
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
	exact, state, err := (identity.KernelProber{}).Probe(int64(pid))
	if err != nil || state != identity.Alive {
		return fmt.Errorf("process %d ownership identity is %s: %w", pid, state, err)
	}
	owned := exact.ExeKnown && filepath.Base(exact.Exe) == instanceTag
	for _, word := range exact.Argv {
		owned = owned || filepath.Base(word) == instanceTag
	}
	if !owned {
		return fmt.Errorf("process %d executable=%q argv=%q does not carry fixture tag %q", pid, exact.Exe, exact.Argv, instanceTag)
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
	code, output, problem := captureChannelOutput(t, func(stdout, stderr io.Writer) int {
		return runReportTurnVerdict([]string{"--root", root, "--session", session, "--main-id", mainID}, stdout, stderr)
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

	// The installed binary has no plain turn-verdict verb since U9a: the
	// handler above is judged in process, and the installed binary through
	// its Stop hook below.
	hookRoot := t.TempDir()
	hookFixture := pendingWaitVerdictCommandFixtureWithOptions(t, hookRoot, "fake", pendingWaitVerdictCommandOptions{
		jobStatus: "pending-setup", writeWaiter: true, requireHook: true,
	})
	hookSession := hookFixture.session
	hook := hookRoot
	canonical := filepath.Join(hookRoot, "bin", "metasystem")
	if err := os.MkdirAll(filepath.Dir(canonical), 0o755); err != nil {
		t.Fatal(err)
	}
	for source, target := range map[string]string{
		binary: canonical,
	} {
		data, readErr := os.ReadFile(source)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if writeErr := testexec.WriteFile(target, data, 0o755); writeErr != nil {
			t.Fatal(writeErr)
		}
	}
	owners := &pendingWaitHookOwners{hookOwners: hookOwners{diagnostics: io.Discard}, fakeUp: true}
	stopRun := runPendingWaitHook(t, nil, hook, owners, "stop",
		`{"session_id":"`+hookSession+`","cwd":"`+hookRoot+`","hook_event_name":"Stop"}`,
		"installed-verdict-stop", pendingWaitFixtureNow(t, hookRoot, strings.Repeat("a", 32)))
	hookOutput := []byte(stopRun.stdout)
	if strings.Contains(string(hookOutput), `"decision":"block"`) || strings.Contains(string(hookOutput), "needs supervision repair") {
		reportText := "unavailable"
		var payload map[string]string
		if json.Unmarshal(hookOutput, &payload) == nil {
			message := payload["reason"]
			if message == "" {
				message = payload["systemMessage"]
			}
			if at := strings.LastIndex(message, "session status --id "); at >= 0 {
				alias := strings.TrimSpace(message[at+len("session status --id "):])
				if data, _, readErr := report.ReadStopStatus(hookRoot, alias); readErr == nil {
					reportText = string(data)
				}
			}
		}
		t.Fatalf("fake Stop hook did not allow the registered wait: output=%s stderr=%s report=%s", hookOutput, stopRun.stderr, reportText)
	}
	artifact := filepath.Join(hookRoot, "artifacts", "agents", "supervision", "stop-verdicts", hookSession+".txt")
	artifactData, err := os.ReadFile(artifact)
	if err != nil || !strings.Contains(string(artifactData), "WAITING: registered wait") {
		t.Fatalf("fake Stop hook omitted its registered-wait evidence: %v %s", err, artifactData)
	}
}

func TestRegisteredLocalAndHumanWaitsInstalledVerdicts(t *testing.T) {
	candidate := os.Getenv("METASYSTEM_WAIT_BINARY")
	if candidate == "" {
		t.Fatal("METASYSTEM_WAIT_BINARY is required for the installed wait-verdict proof")
	}
	binary := testutil.InstalledWaitBinary(t, candidate)
	root := t.TempDir()
	commandFixture := pendingWaitVerdictCommandFixtureWithOptions(t, root, "fake", pendingWaitVerdictCommandOptions{jobStatus: "pending", requireHook: true})
	fixture := newInstalledWaitFixture(t, commandFixture.ownerLineage)
	session := commandFixture.session
	hook, _, owners := installPendingWaitHookFixture(t, root, binary)
	owners.fakeUp = true
	payload := fmt.Sprintf(`{"session_id":%q,"cwd":%q,"hook_event_name":"Stop"}`, session, root)

	child := exec.Command("tail", "-f", "/dev/null")
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	childStopped := false
	stopChild := func() {
		t.Helper()
		if childStopped {
			return
		}
		childStopped = true
		if err := child.Process.Kill(); err != nil {
			t.Errorf("kill tracked child: %v", err)
		}
		if err := child.Wait(); err != nil {
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) {
				t.Errorf("reap tracked child: %v", err)
			}
		}
	}
	t.Cleanup(stopChild)

	register := exec.Command(commandTestExecutable(t), sessionWaitHelperCommand, "--root", root, "--pid", fmt.Sprint(child.Process.Pid), "--label", "installed local build", "--job", "wait-stop-job", "--timeout", "1h", "--json")
	register.Env = fixture.Env(append(os.Environ(), "GO_WANT_BATCH_E2E_COMMAND=1"))
	registerOutput, err := register.CombinedOutput()
	var local metarun.Waiter
	if err != nil || json.Unmarshal(registerOutput, &local) != nil || local.Kind != "local" {
		t.Fatalf("register installed local wait: err=%v output=%s row=%+v", err, registerOutput, local)
	}
	liveHook := runPendingWaitHook(t, fixture, hook, owners, "stop", payload, "registered-local-live", local.RegisteredAt)
	artifact := filepath.Join(root, "artifacts", "agents", "supervision", "stop-verdicts", session+".txt")
	liveArtifact, artifactErr := os.ReadFile(artifact)
	if strings.Contains(liveHook.stdout, `"decision":"block"`) || artifactErr != nil || !strings.Contains(string(liveArtifact), "WAITING: installed local build") {
		t.Fatalf("live registered local wait did not allow Stop: stdout=%s stderr=%s artifactErr=%v artifact=%s", liveHook.stdout, liveHook.stderr, artifactErr, liveArtifact)
	}

	stopChild()
	deadHook := runPendingWaitHook(t, fixture, hook, owners, "stop", payload, "registered-local-dead", local.RegisteredAt)
	deadArtifact, artifactErr := os.ReadFile(artifact)
	if !strings.Contains(deadHook.stdout, `"decision":"block"`) || artifactErr != nil || strings.Contains(string(deadArtifact), "WAITING: installed local build") {
		t.Fatalf("dead registered local wait allowed Stop: stdout=%s stderr=%s artifactErr=%v artifact=%s", deadHook.stdout, deadHook.stderr, artifactErr, deadArtifact)
	}

	humanCommand := exec.Command(commandTestExecutable(t), sessionWaitHelperCommand, "--root", root, "--question", "May the installed run stop?", "--timeout", "1h", "--json")
	humanCommand.Env = fixture.Env(append(os.Environ(), "GO_WANT_BATCH_E2E_COMMAND=1"))
	humanOutput, err := humanCommand.CombinedOutput()
	var human metarun.Waiter
	if err != nil || json.Unmarshal(humanOutput, &human) != nil || human.Kind != "human" {
		t.Fatalf("register installed human wait: err=%v output=%s row=%+v", err, humanOutput, human)
	}
	humanHook := runPendingWaitHook(t, fixture, hook, owners, "stop", payload, "registered-human", human.RegisteredAt)
	humanArtifact, artifactErr := os.ReadFile(artifact)
	if strings.Contains(humanHook.stdout, `"decision":"block"`) || artifactErr != nil || !strings.Contains(string(humanArtifact), "WAITING: human answer to May the installed run stop?") {
		t.Fatalf("registered human wait did not allow Stop: stdout=%s stderr=%s", humanHook.stdout, humanHook.stderr)
	}
}

func TestPendingWaitOldPhysicalObservationUsesSemanticClock(t *testing.T) {
	root := t.TempDir()
	session, mainID := pendingWaitVerdictCommandFixture(t, root, "fake")
	rows, err := pendingWaiterRows(root)
	if err != nil || len(rows) != 1 || rows[0].Pid != int64(os.Getpid()) {
		t.Fatalf("process-owned waiter rows=%+v err=%v", rows, err)
	}
	row := rows[0]
	// The physical observation is a fixed instant long before the wall
	// clock; the semantic clock stands thirty seconds after it, inside the
	// wait's deadline.
	delayedObservation := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	row.RegisteredAt = delayedObservation.Add(-time.Second).Format(time.RFC3339Nano)
	row.LastObservedAt = delayedObservation.Format(time.RFC3339Nano)
	row.Deadline = delayedObservation.Add(time.Minute).Format(time.RFC3339Nano)
	writeWaiterFixture(t, root, row)
	t.Setenv(goalNowEnvironment, delayedObservation.Add(30*time.Second).Format(time.RFC3339Nano))
	assertRegisteredWaitVerdict(t, pendingWaitVerdict(t, root, session, mainID),
		fmt.Sprintf("WAITING: registered wait %s covers job wait-stop-job until %s", row.WaitID, row.Deadline))
}

func TestPendingWaitFromChildShell(t *testing.T) {
	candidate := os.Getenv("METASYSTEM_WAIT_BINARY")
	if candidate == "" {
		t.Fatal("METASYSTEM_WAIT_BINARY is required for the installed child-wait proof")
	}
	binary := testutil.InstalledWaitBinary(t, candidate)
	root := t.TempDir()
	commandFixture := pendingWaitVerdictCommandFixtureWithOptions(t, root, "fake", pendingWaitVerdictCommandOptions{
		jobStatus: "pending", writeWaiter: false, requireHook: true, requireChildShellGit: true,
	})
	fixture := newInstalledWaitFixture(t, commandFixture.ownerLineage)
	mainID := commandFixture.mainID
	runtimeSession := "8d91c146-8460-4bf1-9a48-04bc577c3aa4"
	hook, canonical, owners := installPendingWaitHookFixture(t, root, binary)
	stopSupervision := installPendingWaitSupervisionCleanup(t, fixture, canonical, root)
	startNow, err := goalCommandNow(root)
	if err != nil {
		t.Fatal(err)
	}
	startPayload := fmt.Sprintf(`{"session_id":%q,"cwd":%q,"source":"clear"}`, runtimeSession, root)
	startHook := runPendingWaitHook(t, fixture, hook, owners, "start", startPayload, "associated-start", startNow.Format(time.RFC3339Nano))
	if startHook.up == nil || startHook.up.RuntimeSession != runtimeSession || startHook.up.NoRuntimeSession ||
		startHook.up.StartSource != "clear" {
		t.Fatalf("installed SessionStart did not propagate association arguments: up=%+v stdout=%s stderr=%s", startHook.up, startHook.stdout, startHook.stderr)
	}
	announcements := lease.AnnouncementsFor(root, int64(os.Getpid()))
	if len(announcements) != 1 || announcements[0].MainId != mainID ||
		announcements[0].RuntimeSession != runtimeSession || announcements[0].SessionId == runtimeSession {
		t.Fatalf("associated announcement=%+v main=%s runtime-session=%s", announcements, mainID, runtimeSession)
	}
	if rows, err := pendingWaiterRows(root); err != nil || len(rows) != 0 {
		t.Fatalf("fixture or SessionStart wrote a waiter row: rows=%+v err=%v", rows, err)
	}
	if err := stopSupervision(); err != nil {
		t.Fatalf("stop installed supervision after association proof: %v", err)
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
	readyRead, readyWrite, err := os.Pipe()
	if err != nil {
		_ = childStdout.Close()
		_ = childStderr.Close()
		t.Fatal(err)
	}
	child := exec.Command("/bin/bash", "-c", `exec "$0" work wait "j2:$2" --exit-code --repo "$1" --caller-pid $$`, binary, root, "wait-stop-job")
	child.Env = fixture.Env(append(os.Environ(), waitRegisteredFDEnvironment+"=3"))
	child.ExtraFiles = []*os.File{readyWrite}
	child.Stdout, child.Stderr = childStdout, childStderr
	if err := child.Start(); err != nil {
		_ = readyRead.Close()
		_ = readyWrite.Close()
		_ = childStdout.Close()
		_ = childStderr.Close()
		t.Fatal(err)
	}
	_ = readyWrite.Close()
	fixture.Record(child.Process.Pid)
	childPID := child.Process.Pid
	childExit := make(chan error, 1)
	go func() { childExit <- child.Wait() }()
	childObserved := false
	var childWaitErr error
	readyLine, readyErr := bufio.NewReader(readyRead).ReadString('\n')
	_ = readyRead.Close()
	childWaitID := strings.TrimSpace(readyLine)
	if readyErr != nil || !metarun.ValidWaitID(childWaitID) {
		t.Fatalf("child wait registration signal = %q, %v", readyLine, readyErr)
	}
	finishChild := func() error {
		if childObserved {
			return childWaitErr
		}
		if err := writePendingWaitJobStatus(root, "completed"); err != nil {
			return err
		}
		stateRoot, stateErr := goal.ResolveStateRoot(root)
		if stateErr != nil {
			return stateErr
		}
		if _, notifyErr := metarun.NotifyWaiters(stateRoot, metarun.WaitHint{Kind: "job", TargetID: "wait-stop-job"}); notifyErr != nil {
			return fmt.Errorf("notify child wait for job wait-stop-job: %v", notifyErr)
		}
		childWaitErr = <-childExit
		childObserved = true
		closeStdoutErr, closeStderrErr := childStdout.Close(), childStderr.Close()
		if closeStdoutErr != nil || closeStderrErr != nil {
			return fmt.Errorf("close child output: stdout=%v stderr=%v", closeStdoutErr, closeStderrErr)
		}
		if childWaitErr != nil {
			return childWaitErr
		}
		stored, _, findErr := metarun.FindWaiterByID(root, childWaitID)
		if findErr != nil {
			return fmt.Errorf("read registered wait %s after its child exited: %w", childWaitID, findErr)
		}
		if stored.State == "pending" {
			return fmt.Errorf("registered wait %s remained pending after its child exited", childWaitID)
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

	row, _, err := metarun.FindWaiterByID(root, childWaitID)
	if err != nil || row.Kind != "job" || row.TargetID != "wait-stop-job" || row.State != "pending" {
		t.Fatalf("ready child wait registration = %+v, %v", row, err)
	}
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
	controlHook := runPendingWaitHook(t, fixture, hook, owners, "stop", controlPayload, "unassociated-stop", "")
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

	registeredVerdict := pendingWaitVerdict(t, root, runtimeSession, mainID)
	if registeredVerdict.ShouldBlock {
		diagnostic, _ := os.ReadFile(filepath.Join(root, "artifacts", "agents", "supervision", "stop-verdicts", runtimeSession+".txt"))
		t.Logf("associated wait diagnostic: row=%+v artifact=%s", row, diagnostic)
	}
	assertRegisteredWaitVerdict(t, registeredVerdict, waitingLine)
	beforeAllow := strings.Count(string(logData), "stop response decision=allow")
	allowedPayload := fmt.Sprintf(`{"session_id":%q,"cwd":%q,"hook_event_name":"Stop"}`, runtimeSession, root)
	allowedHook := runPendingWaitHook(t, fixture, hook, owners, "stop", allowedPayload, "associated-stop", pendingWaitFixtureNow(t, root, row.WaitID))
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
	allowedHook = runPendingWaitHook(t, fixture, hook, owners, "stop", allowedPayload, "associated-stop-with-hostile-rows", pendingWaitFixtureNow(t, root, row.WaitID))
	t.Setenv("METASYSTEM_GOAL_NOW", pendingWaitFixtureNow(t, root, row.WaitID))
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

	// The dead waiter's process is a shell whose own argv carries the tag and
	// which reports readiness from its running image, then blocks reading a
	// pipe this test holds. A sleep started through a tagged symlink carried
	// the tag only in argv, and Linux can read a just-exec'd process's argv
	// empty until the new image publishes it ("executable=/usr/bin/sleep
	// argv=[] does not carry fixture tag", batch 12 VM).
	sleepTag := "metasystem-child-wait-dead-sleeper"
	sleepReadyRead, sleepReadyWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer sleepReadyRead.Close()
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
	sleeper := exec.Command("/bin/sh", "-c", `printf 'ready\n' >&3; exec 3>&-; read -r _`, sleepTag)
	sleeper.Stdout, sleeper.Stderr = sleepOutput, sleepProblem
	sleeper.ExtraFiles = []*os.File{sleepReadyWrite}
	sleepHold, err := sleeper.StdinPipe()
	if err != nil {
		_ = sleepReadyWrite.Close()
		_ = sleepOutput.Close()
		_ = sleepProblem.Close()
		t.Fatal(err)
	}
	defer sleepHold.Close()
	if err := sleeper.Start(); err != nil {
		_ = sleepReadyWrite.Close()
		_ = sleepOutput.Close()
		_ = sleepProblem.Close()
		t.Fatal(err)
	}
	_ = sleepReadyWrite.Close()
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
	if line, readErr := bufio.NewReader(sleepReadyRead).ReadString('\n'); readErr != nil || line != "ready\n" {
		t.Fatalf("dead-row sleeper readiness = %q, %v", line, readErr)
	}
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
	deadStartedAt := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339Nano)
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
	runPendingWaitHook(t, fixture, hook, owners, "stop", allowedPayload, "associated-stop-with-dead-waiter", pendingWaitFixtureNow(t, root, row.WaitID))
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
	absentHook := runPendingWaitHook(t, fixture, hook, owners, "stop", absentPayload, "absent-session-stop", "")
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
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	stableNow := time.Date(2026, 9, 13, 11, 0, 0, 0, time.UTC)
	stableBootElapsed := 48 * time.Hour
	t.Setenv(goalNowEnvironment, stableNow.Format(time.RFC3339))
	t.Setenv(goalBootIDEnvironment, "fixture-boot")
	t.Setenv(goalBootNanosEnvironment, strconv.FormatInt(stableBootElapsed.Nanoseconds(), 10))
	beforeOptions, err := waitRegisterOptions(root)
	if err != nil {
		t.Fatal(err)
	}
	beforeNow := beforeOptions.Now()
	bootID, elapsed, err := beforeOptions.BootClock()
	if err != nil {
		t.Fatal(err)
	}
	readyRead, readyWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	releaseRead, releaseWrite, err := os.Pipe()
	if err != nil {
		_ = readyRead.Close()
		_ = readyWrite.Close()
		t.Fatal(err)
	}
	predecessor := exec.Command("/bin/sh", "-c", "printf 'ready\\n' >&3; IFS= read -r _ <&4")
	predecessor.ExtraFiles = []*os.File{readyWrite, releaseRead}
	if err := predecessor.Start(); err != nil {
		_ = readyRead.Close()
		_ = readyWrite.Close()
		_ = releaseRead.Close()
		_ = releaseWrite.Close()
		t.Fatal(err)
	}
	_ = readyWrite.Close()
	_ = releaseRead.Close()
	predecessorDone := make(chan error, 1)
	go func() { predecessorDone <- predecessor.Wait() }()
	predecessorJoined := false
	t.Cleanup(func() {
		_ = releaseWrite.Close()
		if !predecessorJoined {
			_ = predecessor.Process.Kill()
			<-predecessorDone
		}
	})
	ready, readyErr := bufio.NewReader(readyRead).ReadString('\n')
	_ = readyRead.Close()
	if readyErr != nil || ready != "ready\n" {
		t.Fatalf("predecessor readiness=%q err=%v", ready, readyErr)
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
	waitID, nonce := strings.Repeat("c", 32), strings.Repeat("d", 32)
	epoch := int64(1)
	row := metarun.Waiter{
		SchemaVersion: 2, WaitID: waitID, Nonce: nonce, Kind: "job", TargetID: "job-takeover", OwnerDigest: metarun.OwnerDigest(oldMain),
		Pid: predecessorPID, PidStartedAt: predecessorExact.StartedAt.Unix(), PidStartTicks: predecessorExact.StartTicks, BootID: predecessorExact.BootID,
		Session: "old-session", MainId: oldMain, OwnerLineage: oldMain, ClaimEpoch: &epoch, RuntimeSession: "old-session",
		Selector: metarun.WaitSelector{Kind: "job", TargetID: "job-takeover"}, Target: metarun.WaiterTarget{OperationID: "reserve-takeover", StartedAt: "2026-09-13T10:00:00Z", Round: 2},
		RegisteredAt: beforeNow.Add(-time.Minute).Format(time.RFC3339Nano), Deadline: beforeNow.Add(time.Hour).Format(time.RFC3339Nano), DeadlineBootID: bootID,
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
	if err := <-predecessorDone; err == nil {
		t.Fatal("killed predecessor exited successfully")
	}
	predecessorJoined = true
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
	originalPID, originalAdapter := waitCallerPID, waitDeliveryRuntime
	waitCallerPID = func() int64 { return self }
	waitDeliveryRuntime = func(string, string) (string, error) { return "fake", nil }
	t.Cleanup(func() { waitCallerPID, waitDeliveryRuntime = originalPID, originalAdapter })
	code, output, problem := captureChannelOutput(t, func(stdout, stderr io.Writer) int {
		return runWait([]string{"--root", root, "--resume", waitID, "--json"}, stdout, stderr)
	})
	var result metarun.WaitResult
	decodeErr := json.Unmarshal([]byte(output), &result)
	if code != 0 || problem != "" || decodeErr != nil || result.WaitID == waitID || result.PointerRepaired {
		t.Fatalf("takeover resume code=%d output=%q problem=%q result=%+v err=%v", code, output, problem, result, decodeErr)
	}
	afterOptions, err := waitRegisterOptions(root)
	if err != nil {
		t.Fatal(err)
	}
	afterBootID, afterElapsed, err := afterOptions.BootClock()
	if err != nil || afterOptions.Now() != beforeNow || afterBootID != bootID || afterElapsed != elapsed {
		t.Fatalf("resume crossed semantic clocks: now=%s/%s boot=%s/%s elapsed=%s/%s err=%v",
			beforeNow, afterOptions.Now(), bootID, afterBootID, elapsed, afterElapsed, err)
	}
}

// directHookCommand runs the engine's hook entry as the runtime settings do:
// in the installation directory, through the engine the settings select. It
// launches the SessionStart hook entry, never a proof attempt, so it stays
// outside proofBinaryFixture.command (TestProofAttemptBinaryLaunchesUseSharedIsolation).
func directHookCommand(hookEntry, installation string) *exec.Cmd {
	command := exec.Command(hookEntry, "internal", "hook", "fake", "start")
	command.Dir = installation
	return command
}

// runWait drives the wait owner work wait reaches, with the caller the
// command registers and its result printed as the command prints it.
func runWait(args []string, stdout, stderr io.Writer) int {
	return runWaitCommand(args, nil, waitCallerPID(), func(result metarun.WaitResult, jsonOutput bool) {
		writeWaitResult(stdout, result, jsonOutput)
	}, stdout, stderr)
}

// waitHelperCommand and sessionWaitHelperCommand run the wait owner (the one
// work wait reaches) and the session wait registration in a child of this
// test binary, a process of its own that a bed can kill and restart.
const (
	waitHelperCommand        = "test-helper-wait"
	sessionWaitHelperCommand = "test-helper-session-wait"
)

func init() {
	// The helpers are processes of their own: they print on their own
	// standard streams, which the bed reads through their pipes.
	testHelperCommands[waitHelperCommand] = func(args []string) int { return runWait(args, os.Stdout, os.Stderr) }
	testHelperCommands[sessionWaitHelperCommand] = func(args []string) int { return runSessionWait(args, os.Stdout, os.Stderr) }
}
