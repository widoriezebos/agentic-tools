package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/report"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

func sessionStopFileBed(t *testing.T) (string, map[string][]byte) {
	t.Helper()
	root := t.TempDir()
	goals := filepath.Join(root, "plans", "goals")
	if err := os.MkdirAll(goals, 0o755); err != nil {
		t.Fatal(err)
	}
	rootRecord := &goal.RootRecord{
		Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1",
		SyncMode: goal.SyncLocal, Revision: 1,
	}
	rootBytes := goal.RenderRoot(rootRecord)
	if err := os.WriteFile(filepath.Join(goals, "backlog.md"), rootBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	budget := goal.Budget{
		ElapsedLimit: "4h", AttemptLimit: 4,
		ReservedJobMinutesLimit: 240, ActiveJobLimit: 2,
	}
	waiting := &goal.GoalFile{
		Id: "waiting", State: goal.StateApproved, Tier: 3, Intent: "Claim shared work", Origin: goal.OriginMain,
		NextStep: "Claim and dispatch it.", OpenedAt: "2026-08-23T00:00:00Z", Revision: 2,
		Budget: &budget,
		History: []goal.HistoryLine{{
			At: "2026-08-23T00:00:00Z", Opid: "01ARZ3NDEKTSV4RRFFQ69G5FAV-bed-m1-00000000",
			Verb: "open", Actor: "bed-m1+coordinator", Targets: []string{"waiting"}, Keep: -1,
		}, {
			At: "2026-08-23T00:01:00Z", Opid: "01ARZ3NDEKTSV4RRFFQ69G5FAY-bed-m1-00000001",
			Verb: "approve", Actor: "human:Wido", Targets: []string{"waiting"}, Keep: -1,
		}},
	}
	waiting.Approved = &goal.ApprovalRecord{
		By: "human:Wido", At: waiting.History[1].At, Revision: 2, Opid: waiting.History[1].Opid,
		Authority: goal.ApprovalAuthorityProven, Digest: goal.ApprovalDigest(waiting.Intent, waiting.Tier, budget),
	}
	waitingBytes := goal.RenderFile(waiting)
	if err := os.WriteFile(filepath.Join(goals, "waiting.md"), waitingBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	return root, map[string][]byte{
		"plans/goals/backlog.md": append([]byte(nil), rootBytes...),
		"plans/goals/waiting.md": append([]byte(nil), waitingBytes...),
	}
}

func sessionStopRawInputs(t *testing.T, root string, repository goal.Repository) (func(string) (goal.Endpoint, error), func(string) (string, error)) {
	t.Helper()
	resolve := func(requested string) (goal.Endpoint, error) {
		if requested != root {
			return goal.Endpoint{}, fmt.Errorf("unexpected goal endpoint root %q", requested)
		}
		return goal.Endpoint{Root: root, Remote: "local", Branch: goal.LocalLedgerBranch, Repository: repository}, nil
	}
	machine := func(requested string) (string, error) {
		return goal.ResolveMachineWithConfig(requested, func(configRoot, key string) (string, error) {
			if configRoot != root || key != "metasystem.goal.machine" {
				return "", fmt.Errorf("unexpected machine config lookup %q %q", configRoot, key)
			}
			return "bed-m1", nil
		})
	}
	return resolve, machine
}

func sessionStopBed(t *testing.T) string {
	t.Helper()
	root, _ := sessionStopFileBed(t)
	goalSyncMutationGit(t, root, "init", "-q", "-b", "main")
	goalSyncMutationGit(t, root, "config", "metasystem.goal.machine", "bed-m1")
	goalSyncMutationGit(t, root, "config", "goal.sync-remote", "local")
	goalSyncMutationGit(t, root, "config", "user.name", "session-stop-fixture")
	goalSyncMutationGit(t, root, "config", "user.email", "session-stop@example.invalid")
	goalSyncMutationGit(t, root, "add", "plans/goals")
	goalSyncMutationGit(t, root, "commit", "-q", "-m", "session stop bed")
	goalSyncMutationGit(t, root, "update-ref", goal.AcceptedRef, "HEAD")
	return root
}

func sessionStopLiveRef(t *testing.T) (humanauthority.ProcessRef, map[string]any) {
	t.Helper()
	exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("probe fixture process: %v %v", state, err)
	}
	human := humanauthority.ProcessRef{
		PID: exact.Pid, PIDStartedAt: exact.StartedAt.Unix(),
		StartTicks: exact.StartTicks, BootID: exact.BootID,
	}
	leaseRecord := map[string]any{
		"holderMainId": "main-1", "pid": exact.Pid,
		"pidStartedAt": exact.StartedAt.Unix(), "claimEpoch": 7,
	}
	if exact.StartTicks > 0 && exact.BootID != "" {
		leaseRecord["pidStartTicks"] = exact.StartTicks
		leaseRecord["bootId"] = exact.BootID
	}
	return human, leaseRecord
}

func installSessionStopLease(t *testing.T, root string, record map[string]any) {
	t.Helper()
	path := filepath.Join(root, "artifacts", "agents", "mains", "worktree-lease.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func installSessionStopAnnouncement(t *testing.T, root, session, main string, human humanauthority.ProcessRef) {
	t.Helper()
	record := map[string]any{
		"sessionId": session, "mainId": main, "pid": human.PID,
		"pidStartedAt": human.PIDStartedAt, "pgid": human.PID,
		"runtime": "claude", "instanceTag": "session-stop-command-fixture",
		"commandHash": strings.Repeat("b", 64), "announcedAt": "2026-09-02T09:59:00Z",
	}
	if human.StartTicks > 0 && human.BootID != "" {
		record["pidStartTicks"] = human.StartTicks
		record["bootId"] = human.BootID
	}
	path := filepath.Join(root, "artifacts", "agents", "mains", session+"-"+fmt.Sprint(human.PID)+".json")
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

type sessionStopCommandAuthorityReader struct {
	pid   int64
	exact identity.Exact
}

func (r sessionStopCommandAuthorityReader) Read(pid int64) (humanauthority.Snapshot, error) {
	if pid == r.pid {
		exact := r.exact
		exact.Argv = []string{"attended-human-shell"}
		exact.ArgvKnown = true
		return humanauthority.Snapshot{
			Exact: exact, Executable: "/fixture/attended-human-shell", ExecutableKnown: true,
			OwnerUID: 501, OwnerKnown: true,
			ParentPID: 1, ParentKnown: true, TerminalID: "tty-session-stop", TerminalKnown: true,
		}, nil
	}
	return commandAuthoritySystemRootSnapshot(), nil
}

func (r sessionStopCommandAuthorityReader) SessionLeader(int64) (int64, error) {
	return r.pid, nil
}

func sessionStopCommandProof(t *testing.T, root string, now time.Time) humanauthority.Proof {
	t.Helper()
	adapters := filepath.Join(root, "scripts", "agents", "adapters")
	if err := os.MkdirAll(adapters, 0o755); err != nil {
		t.Fatal(err)
	}
	adapter := "#!/bin/sh\n[ \"$1\" = signature ] && printf '%s\\n' 'match never-an-attended-human-shell'\n"
	if err := testexec.WriteFile(filepath.Join(adapters, "human-fixture.sh"), []byte(adapter), 0o755); err != nil {
		t.Fatal(err)
	}
	exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("probe command fixture process: %v %v", state, err)
	}
	reader := sessionStopCommandAuthorityReader{pid: exact.Pid, exact: exact}
	proof, err := humanauthority.ProveTerminal(root, exact.Pid, reader, now)
	if err != nil {
		t.Fatal(err)
	}
	if proof.AuthorityGrade() != humanauthority.GradeTerminal || !proof.TerminalValidFor(root) || proof.ValidFor(root) {
		t.Fatalf("session stop proof has the wrong grade: %+v", proof)
	}
	return proof
}

func stubSessionStopCommand(t *testing.T) {
	t.Helper()
	originalClassify := classifySessionStopCaller
	originalHolder := currentSessionStopHolder
	originalView := classifySessionStopView
	originalProof := proveSessionStopHuman
	originalNow := sessionStopNow
	t.Cleanup(func() {
		classifySessionStopCaller = originalClassify
		currentSessionStopHolder = originalHolder
		classifySessionStopView = originalView
		proveSessionStopHuman = originalProof
		sessionStopNow = originalNow
	})
}

func TestProveSessionStopHumanKeepsThePopulatedProof(t *testing.T) {
	proof, err := proveSessionStopHuman(t.TempDir(), 1<<30, time.Date(2026, 9, 15, 9, 0, 0, 0, time.UTC))
	if err == nil || proof.Outcome != humanauthority.OutcomeUnreadable {
		t.Fatalf("proof outcome=%q err=%v", proof.Outcome, err)
	}
}

func TestSessionStopAttendedHumanEndsQuietly(t *testing.T) {
	root, acceptedFiles := sessionStopFileBed(t)
	human, leaseRecord := sessionStopLiveRef(t)
	installSessionStopLease(t, root, leaseRecord)
	installSessionStopAnnouncement(t, root, "session-human", "main-1", human)
	stubSessionStopCommand(t)
	epoch := int64(7)
	now := time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)
	proof := sessionStopCommandProof(t, root, now)
	classifySessionStopCaller = func(string, int64) (lease.Classification, error) {
		return lease.Classification{Class: lease.ClassHuman}, nil
	}
	currentSessionStopHolder = func(string) (lease.CurrentHolderView, error) {
		return lease.CurrentHolderView{MainId: "main-1", SessionId: "session-human", Pid: human.PID}, nil
	}
	classifySessionStopView = func(string, int64) (lease.ClassifyResult, error) {
		return lease.ClassifyResult{Class: lease.ClassHuman, ClaimEpoch: &epoch}, nil
	}
	proveSessionStopHuman = func(string, int64, time.Time) (humanauthority.Proof, error) {
		return proof, nil
	}
	sessionStopNow = func() time.Time { return now }

	_, code := captureStdout(t, func() int {
		return runSessionStop([]string{"--root", root, "--by", "Wido"})
	})
	if code != 0 {
		t.Fatalf("an attended human must obtain the one-shot authorization: exit %d", code)
	}
	store := &goal.Store{Root: root, Now: func() time.Time {
		return time.Date(2026, 9, 2, 10, 1, 0, 0, time.UTC)
	}}
	repository := &sessionStopAcceptedRepository{tip: "session-stop-accepted-tip", files: acceptedFiles}
	endpoint := goal.Endpoint{Root: root, Remote: "local", Repository: repository}
	seat := goal.TurnVerdictOptions{SeatActor: goal.Actor{Machine: "bed-m1"}}
	verdict, err := store.TurnVerdictAtEndpoint(endpoint, "bed-m1", goal.ScanResult{}, "session-human", "", "main-1", seat)
	if err != nil || verdict.ShouldBlock || !strings.Contains(verdict.Display, "Wido") {
		t.Fatalf("the attended human must end quietly: %+v %v", verdict, err)
	}
	second, err := store.TurnVerdictAtEndpoint(endpoint, "bed-m1", goal.ScanResult{}, "session-human", "", "main-1", seat)
	if err != nil || !second.ShouldBlock || second.BlockSource == nil || *second.BlockSource != "idle-backlog" {
		t.Fatalf("one attended-human marker must end exactly one session: %+v %v", second, err)
	}
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if repository.fetches == 0 || repository.releases != repository.fetches || len(repository.captured) != 0 {
		t.Fatalf("accepted repository fetches=%d releases=%d active=%d", repository.fetches, repository.releases, len(repository.captured))
	}
}

func TestSessionStopAgentClassifiedCallerCannotReachTheWriter(t *testing.T) {
	root, _ := sessionStopFileBed(t)
	stubSessionStopCommand(t)
	classifySessionStopCaller = func(string, int64) (lease.Classification, error) {
		return lease.Classification{Class: lease.ClassDelegate}, nil
	}
	currentSessionStopHolder = func(string) (lease.CurrentHolderView, error) {
		t.Fatal("an agent-classified caller reached holder lookup")
		return lease.CurrentHolderView{}, nil
	}
	proveSessionStopHuman = func(string, int64, time.Time) (humanauthority.Proof, error) {
		t.Fatal("an agent-classified caller reached human proof")
		return humanauthority.Proof{}, nil
	}

	_, code := captureStdout(t, func() int {
		return runSessionStop([]string{"--root", root, "--by", "Agent"})
	})
	if code != 3 {
		t.Fatalf("an agent-classified caller must be refused before persistence: exit %d", code)
	}
	if entries, err := os.ReadDir(filepath.Join(root, "artifacts", "agents", "session-stops")); err == nil && len(entries) > 0 {
		t.Fatalf("the refused command wrote authorization bytes: %v", entries)
	}
}

func TestReportTurnVerdictHeldClaimWritesRealSeatIdleIntent(t *testing.T) {
	root, files := sessionStopFileBed(t)
	budget := goal.Budget{
		ElapsedLimit: "4h", AttemptLimit: 4,
		ReservedJobMinutesLimit: 240, ActiveJobLimit: 2,
	}
	held := &goal.GoalFile{
		Id: "held", State: goal.StateApproved, Tier: 3, Intent: "Continue the held work", Origin: goal.OriginMain,
		NextStep: "Continue it.", OpenedAt: "2026-08-22T00:00:00Z", Revision: 2,
		Budget: &budget,
		History: []goal.HistoryLine{{
			At: "2026-08-22T00:00:00Z", Opid: "01ARZ3NDEKTSV4RRFFQ69G5FAZ-bed-m1-00000002",
			Verb: "open", Actor: "bed-m1+coordinator", Targets: []string{"held"}, Keep: -1,
		}, {
			At: "2026-08-22T00:01:00Z", Opid: "01ARZ3NDEKTSV4RRFFQ69G5FB0-bed-m1-00000003",
			Verb: "approve", Actor: "human:Wido", Targets: []string{"held"}, Keep: -1,
		}},
	}
	held.Approved = &goal.ApprovalRecord{
		By: "human:Wido", At: held.History[1].At, Revision: 2, Opid: held.History[1].Opid,
		Authority: goal.ApprovalAuthorityProven, Digest: goal.ApprovalDigest(held.Intent, held.Tier, budget),
	}
	heldBytes := goal.RenderFile(held)
	if err := os.WriteFile(filepath.Join(root, "plans", "goals", "held.md"), heldBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	write := func(relative, content string) {
		t.Helper()
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("metasystem.conf", "metasystem.runtimes=claude\nrole.steward-continuation.runtime=claude\nrole.steward-continuation.model.claude=fixture\n")
	write("scripts/agents/roles/steward-continuation.md", "# Role: steward-continuation\nContinue the named claim.\n")
	write("scripts/agents/roles/steward-continuation.requirements.json", "{\"required\":[]}")
	write("scripts/agents/schemas/steward-continuation.schema.json", "{\"type\":\"object\"}")
	write("scripts/agents/permissions/workspace.json", "{\"write\":[\"workspace\"]}")
	repository := &proofAdmissionRepository{top: root, root: root, commits: map[string]proofAdmissionCommit{}, operations: map[string]string{}}
	repository.seed(map[string][]byte{
		"metasystem/plans/goals/backlog.md": files["plans/goals/backlog.md"],
		"metasystem/plans/goals/waiting.md": files["plans/goals/waiting.md"],
		"metasystem/plans/goals/held.md":    heldBytes,
	})
	resolve, machine := sessionStopRawInputs(t, root, repository)
	endpoint, err := resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	operationID, err := goal.NewOperationULID()
	if err != nil {
		t.Fatal(err)
	}
	claimResult, err := goal.Claim(goal.VerbRequest{
		Endpoint: endpoint, Actor: goal.Actor{Machine: "bed-m1", Lineage: "main-1"},
		Ulid: operationID, Now: time.Now(), ClaimEpoch: 7,
	}, "held")
	if err != nil || (claimResult.Outcome != goal.OutcomeConfirmed && claimResult.Outcome != goal.OutcomeConfirmedLate) {
		t.Fatalf("fixture claim failed: %+v %v", claimResult, err)
	}
	claimed := repository.goalFile(t, "held")
	if claimed.State != goal.StateClaimed || claimed.Claimed == nil || claimed.Claimed.Machine != "bed-m1" || claimed.Claimed.Lineage != "main-1" {
		t.Fatalf("real claim did not persist held ownership: %+v", claimed)
	}

	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(steward.RepoIdentityPath(root)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := steward.MintIdentity(steward.RepoIdentityPath(root), steward.InstallIdentity{
		RepoIdentity: absoluteRoot, Generation: 1, InstallPath: "/bin/true", MintedAt: "2026-09-06T12:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}
	human, _ := sessionStopLiveRef(t)
	if _, err := lease.Announce(root, "held-command-session", human.PID, human.PIDStartedAt,
		"held-command-fixture", "claude", "main-1"); err != nil {
		t.Fatal(err)
	}
	holder, err := lease.CurrentHolder(root)
	if err != nil {
		t.Fatal(err)
	}

	var last goal.Verdict
	for stop := 1; stop <= 3; stop++ {
		args := []string{"--root", root, "--session", "held-command-session", "--main-id", holder.MainId}
		if stop == 3 {
			args = append(args, "--stop-hook-active")
		}
		stdout, code := captureStdout(t, func() int { return runReportTurnVerdictWithInputs(args, resolve, machine) })
		if code != 0 {
			t.Fatalf("turn-verdict stop %d exited %d: %s", stop, code, stdout)
		}
		var verdict goal.Verdict
		if err := json.Unmarshal([]byte(stdout), &verdict); err != nil {
			t.Fatal(err)
		}
		last = verdict
		if stop < 3 && !verdict.ShouldBlock {
			t.Fatalf("turn-verdict stop %d did not block: %+v", stop, verdict)
		}
		if stop == 3 && verdict.ShouldBlock {
			t.Fatalf("third turn-verdict did not finish its local handoff: %+v", verdict)
		}
	}
	intents, err := steward.LiveIntents(root)
	if err != nil || len(intents) != 1 {
		t.Fatalf("the command did not persist exactly one real intent: %+v %v; verdict=%+v", intents, err, last)
	}
	intent := intents[0]
	if intent.Reason != "seatIdle" || intent.Goal != "held" || intent.ClaimNeeded ||
		intent.SeatActor == nil || intent.SeatActor.Machine != "bed-m1" ||
		intent.SeatActor.Lineage != "main-1" || intent.SeatClaimEpoch != holder.ClaimEpoch {
		t.Fatalf("the command-layer intent lost the held goal or seat authority: %+v", intent)
	}
}

func TestReportTurnVerdictFactsWriteFailurePreservesCompletedVerdict(t *testing.T) {
	original := turnVerdictFactsWriter
	t.Cleanup(func() { turnVerdictFactsWriter = original })

	invoke := func(root, session, factsPath string, repository goal.Repository) (string, string, int) {
		resolve, machine := sessionStopRawInputs(t, root, repository)
		var stdout string
		stderr, code := captureStderr(t, func() int {
			var inner int
			stdout, inner = captureStdout(t, func() int {
				return runReportTurnVerdictWithInputs([]string{"--root", root, "--session", session, "--facts-file", factsPath}, resolve, machine)
			})
			return inner
		})
		return stdout, stderr, code
	}
	assertWire := func(stdout, stderr, factsPath string, wantBlock bool) {
		t.Helper()
		if !strings.Contains(stderr, "frozen facts could not be written: injected auxiliary failure") {
			t.Fatalf("writer failure diagnostic = %q", stderr)
		}
		if _, err := os.Lstat(factsPath); !os.IsNotExist(err) {
			t.Fatalf("failed auxiliary publication left bytes at %s: %v", factsPath, err)
		}
		var object map[string]any
		if err := json.Unmarshal([]byte(stdout), &object); err != nil {
			t.Fatalf("completed verdict stdout is not JSON: %q: %v", stdout, err)
		}
		var typed goal.Verdict
		if err := json.Unmarshal([]byte(stdout), &typed); err != nil {
			t.Fatal(err)
		}
		canonical, err := json.Marshal(typed)
		if err != nil || string(canonical)+"\n" != stdout {
			t.Fatalf("completed verdict wire changed: %q: %v", stdout, err)
		}
		if blocked, ok := object["shouldBlock"].(bool); !ok || blocked != wantBlock {
			t.Fatalf("completed verdict lost its control result: %v", object)
		}
		if wantBlock && object["blockSource"] != "idle-backlog" {
			t.Fatalf("completed block lost its source: %v", object)
		}
		if !wantBlock && object["blockSource"] != nil {
			t.Fatalf("completed allowance gained a source: %v", object)
		}
	}

	turnVerdictFactsWriter = func(string, string, string) (bool, error) {
		return false, errors.New("injected auxiliary failure")
	}
	blockRoot, blockFiles := sessionStopFileBed(t)
	blockRepository := &sessionStopAcceptedRepository{tip: "session-stop-accepted-tip", files: blockFiles}
	blockFacts := filepath.Join(t.TempDir(), "block-facts.json")
	stdout, stderr, code := invoke(blockRoot, "facts-writer-block", blockFacts, blockRepository)
	if code != 0 {
		t.Fatalf("completed block exited %d: stdout=%q stderr=%q", code, stdout, stderr)
	}
	assertWire(stdout, stderr, blockFacts, true)

	allowRoot, allowFiles := sessionStopFileBed(t)
	allowRepository := &sessionStopAcceptedRepository{tip: "session-stop-accepted-tip", files: allowFiles}
	allowResolve, allowMachine := sessionStopRawInputs(t, allowRoot, allowRepository)
	for attempt := 1; attempt < 3; attempt++ {
		if output, runCode := captureStdout(t, func() int {
			return runReportTurnVerdictWithInputs([]string{"--root", allowRoot, "--session", "facts-writer-allow"}, allowResolve, allowMachine)
		}); runCode != 0 {
			t.Fatalf("allowance setup attempt %d exited %d: %s", attempt, runCode, output)
		}
	}
	allowFacts := filepath.Join(t.TempDir(), "allow-facts.json")
	stdout, stderr, code = invoke(allowRoot, "facts-writer-allow", allowFacts, allowRepository)
	if code != 0 {
		t.Fatalf("completed allowance exited %d: stdout=%q stderr=%q", code, stdout, stderr)
	}
	assertWire(stdout, stderr, allowFacts, false)
}

func TestReportTurnVerdictCompletionCapturePreservesVerdict(t *testing.T) {
	original := turnVerdictCompletionWriter
	t.Cleanup(func() { turnVerdictCompletionWriter = original })
	t.Setenv("METASYSTEM_GATES_RUNNING", "1")
	t.Setenv("METASYSTEM_GOAL_NOW", "2026-09-14T12:00:00Z")

	root, files := sessionStopFileBed(t)
	repository := &sessionStopAcceptedRepository{tip: "session-stop-accepted-tip", files: files}
	resolve, machine := sessionStopRawInputs(t, root, repository)
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	jobDir := filepath.Join(root, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		t.Fatal(err)
	}
	job := `{"jobId":"returned","operationId":"operation-returned","mainId":"main-completion","machineId":"bed-m1","ownerLineage":"lineage-completion","claimEpoch":7,"startedAt":"2026-09-14T10:00:00Z","endedAt":"2026-09-14T10:01:00Z","status":"completed","role":"implementer","goalId":"waiting"}`
	if err := os.WriteFile(filepath.Join(jobDir, "returned.json"), []byte(job), 0o600); err != nil {
		t.Fatal(err)
	}
	completionPath := filepath.Join(t.TempDir(), "completion.json")
	factsPath := filepath.Join(t.TempDir(), "facts.json")
	args := []string{"--root", root, "--session", "completion-capture", "--main-id", "main-completion", "--facts-file", factsPath, "--completion-file", completionPath}
	stdout, code := captureStdout(t, func() int { return runReportTurnVerdictWithInputs(args, resolve, machine) })
	if code != 0 {
		t.Fatalf("completion capture exited %d: %s", code, stdout)
	}
	var facts goal.TurnVerdictFacts
	factsBytes, err := os.ReadFile(factsPath)
	if err != nil || json.Unmarshal(factsBytes, &facts) != nil {
		t.Fatal(err)
	}
	if len(facts.Scan.Jobs) != 0 {
		t.Fatalf("terminal completion changed judgment scan membership: %+v", facts.Scan.Jobs)
	}
	data, err := os.ReadFile(completionPath)
	if err != nil {
		t.Fatal(err)
	}
	var observation report.StopCompletionObservation
	if err := json.Unmarshal(data, &observation); err != nil {
		t.Fatal(err)
	}
	if len(observation.Records) != 1 || observation.Records[0].Id != "returned" || observation.Records[0].Ownership != "owned" ||
		observation.Identity.Installation != root || observation.Identity.Session != "completion-capture" || observation.Identity.MainId != "main-completion" {
		t.Fatalf("completion observation = %+v", observation)
	}

	snapshot := func() map[string]string {
		t.Helper()
		got := map[string]string{}
		for _, base := range []string{filepath.Join(root, "plans"), filepath.Join(root, "artifacts")} {
			_ = filepath.WalkDir(base, func(path string, entry os.DirEntry, walkErr error) error {
				if walkErr != nil || entry.IsDir() {
					return walkErr
				}
				bytes, readErr := os.ReadFile(path)
				if readErr != nil {
					return readErr
				}
				got[path] = string(bytes)
				return nil
			})
		}
		return got
	}
	failedPath := filepath.Join(t.TempDir(), "failed-completion.json")
	failedFactsPath := filepath.Join(t.TempDir(), "failed-facts.json")
	var atWriter map[string]string
	turnVerdictCompletionWriter = func(string, string, string) (bool, error) {
		atWriter = snapshot()
		return false, errors.New("injected completion failure")
	}
	var failedStdout string
	stderr, failedCode := captureStderr(t, func() int {
		failedStdout, code = captureStdout(t, func() int {
			return runReportTurnVerdictWithInputs([]string{"--root", root, "--session", "completion-capture", "--main-id", "main-completion", "--facts-file", failedFactsPath, "--completion-file", failedPath}, resolve, machine)
		})
		return code
	})
	failedFactsBytes, factsErr := os.ReadFile(failedFactsPath)
	var failedFacts goal.TurnVerdictFacts
	if factsErr != nil || json.Unmarshal(failedFactsBytes, &failedFacts) != nil {
		t.Fatalf("read failed-invocation facts: %v", factsErr)
	}
	wantStdoutBytes, marshalErr := json.Marshal(failedFacts.Verdict)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	wantStdout := string(wantStdoutBytes) + "\n"
	if failedCode != 0 || failedStdout != wantStdout || !strings.Contains(stderr, "completion observation could not be written: injected completion failure") {
		t.Fatalf("sidecar failure changed its completed verdict: code=%d stdout=%q want=%q stderr=%q", failedCode, failedStdout, wantStdout, stderr)
	}
	if _, err := os.Lstat(failedPath); !os.IsNotExist(err) {
		t.Fatalf("failed completion publication left a sidecar: %v", err)
	}
	after := snapshot()
	if atWriter == nil || !reflect.DeepEqual(atWriter, after) {
		t.Fatalf("completion sidecar failure changed state after the completed judgment: at-writer=%v after=%v", atWriter, after)
	}
}
