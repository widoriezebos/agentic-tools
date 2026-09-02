package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

func sessionStopBed(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	goalSyncMutationGit(t, root, "init", "-q", "-b", "main")
	goalSyncMutationGit(t, root, "config", "metasystem.goal.machine", "bed-m1")
	goalSyncMutationGit(t, root, "config", "goal.sync-remote", "local")
	goalSyncMutationGit(t, root, "config", "user.name", "session-stop-fixture")
	goalSyncMutationGit(t, root, "config", "user.email", "session-stop@example.invalid")

	goals := filepath.Join(root, "plans", "goals")
	if err := os.MkdirAll(goals, 0o755); err != nil {
		t.Fatal(err)
	}
	rootRecord := &goal.RootRecord{
		Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1",
		SyncMode: goal.SyncLocal, Revision: 1,
	}
	if err := os.WriteFile(filepath.Join(goals, "backlog.md"), goal.RenderRoot(rootRecord), 0o644); err != nil {
		t.Fatal(err)
	}
	budget := goal.Budget{
		ElapsedLimit: "4h", AttemptLimit: 4,
		ReservedJobMinutesLimit: 240, ActiveJobLimit: 2,
	}
	waiting := &goal.GoalFile{
		Id: "waiting", State: goal.StateQueued, Intent: "Claim shared work", Origin: goal.OriginMain,
		NextStep: "Claim and dispatch it.", OpenedAt: "2026-08-23T00:00:00Z", Revision: 1,
		Budget: &budget,
		History: []goal.HistoryLine{{
			At: "2026-08-23T00:00:00Z", Opid: "01ARZ3NDEKTSV4RRFFQ69G5FAV-bed-m1-00000000",
			Verb: "open", Actor: "bed-m1+coordinator", Targets: []string{"waiting"}, Keep: -1,
		}},
	}
	if err := os.WriteFile(filepath.Join(goals, "waiting.md"), goal.RenderFile(waiting), 0o644); err != nil {
		t.Fatal(err)
	}
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
			ParentPID: 1, ParentKnown: true, TerminalID: "tty-session-stop", TerminalKnown: true,
		}, nil
	}
	return humanauthority.Snapshot{
		Exact:      identity.Exact{Pid: 1, StartedAt: time.Unix(1, 0), Argv: []string{"fixture-init"}, ArgvKnown: true},
		Executable: "/fixture/init", ExecutableKnown: true,
		ParentPID: 1, ParentKnown: true, TerminalID: "tty-session-stop", TerminalKnown: true,
	}, nil
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
	if err := os.WriteFile(filepath.Join(adapters, "human-fixture.sh"), []byte(adapter), 0o755); err != nil {
		t.Fatal(err)
	}
	exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("probe command fixture process: %v %v", state, err)
	}
	reader := sessionStopCommandAuthorityReader{pid: exact.Pid, exact: exact}
	if _, err := humanauthority.Enroll(root, exact.Pid, reader, now); err != nil {
		t.Fatal(err)
	}
	proof, err := humanauthority.Prove(root, exact.Pid, reader, now)
	if err != nil {
		t.Fatal(err)
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

func TestSessionStopAttendedHumanEndsQuietly(t *testing.T) {
	root := sessionStopBed(t)
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
	verdict, err := store.TurnVerdict(goal.ScanResult{}, "session-human", "", "main-1")
	if err != nil || verdict.ShouldBlock || !strings.Contains(verdict.Display, "Wido") {
		t.Fatalf("the attended human must end quietly: %+v %v", verdict, err)
	}
	second, err := store.TurnVerdict(goal.ScanResult{}, "session-human", "", "main-1")
	if err != nil || !second.ShouldBlock || second.BlockSource == nil || *second.BlockSource != "idle-backlog" {
		t.Fatalf("one attended-human marker must end exactly one session: %+v %v", second, err)
	}
}

func TestSessionStopAgentClassifiedCallerCannotReachTheWriter(t *testing.T) {
	root := sessionStopBed(t)
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
