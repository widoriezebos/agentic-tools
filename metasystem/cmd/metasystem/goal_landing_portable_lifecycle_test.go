package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	goalbranch "github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batchowner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// This fixture exercises the real generic owner, goal handover, native
// command/JUnit runner, receipt consumer and bare-origin transport. Its goal
// branch critique closure is a fixture record; no TestResult or batch state
// transition is synthesized.
func TestGLEBatchPortableOwnerLandsRealCommandApplication(t *testing.T) {
	runGLEBatchPortableOwnerMovedOrigin(t)
}

func waitForPortableOwnerLease(t *testing.T, root string, pid int, exited <-chan struct{}, exitErr func() error) {
	t.Helper()
	deadline, bounded := t.Deadline()
	for {
		current, err := lease.CurrentHolder(root)
		if err == nil && current.Pid == int64(pid) {
			return
		}
		select {
		case <-exited:
			t.Fatalf("landing owner %d exited before acquiring the checkout: %v", pid, exitErr())
		case <-t.Context().Done():
			t.Fatalf("landing owner %d did not acquire the checkout before test cancellation: %v", pid, t.Context().Err())
		default:
		}
		if bounded && !time.Now().Before(deadline) {
			t.Fatalf("landing owner %d did not acquire the checkout before the test deadline: %v", pid, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func holdPortableFixtureSlots(t *testing.T, root string, max int, slots *[]*os.File) {
	t.Helper()
	directory, selected, err := proofrun.FixtureHostAdmissionDirectory(root)
	if err != nil || !selected {
		t.Fatalf("capacity fixture has no authenticated private admission namespace: selected=%t err=%v", selected, err)
	}
	guard, err := os.OpenFile(filepath.Join(directory, "admission.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer guard.Close()
	deadline, bounded := t.Deadline()
	for {
		if err := syscall.Flock(int(guard.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err == nil {
			break
		} else if !errors.Is(err, syscall.EWOULDBLOCK) {
			t.Fatalf("lock private host admission guard: %v", err)
		}
		if bounded && !time.Now().Before(deadline) {
			t.Fatal("private host admission guard did not become available before test deadline")
		}
		select {
		case <-t.Context().Done():
			t.Fatalf("private host admission guard did not become available: %v", t.Context().Err())
		default:
		}
		time.Sleep(20 * time.Millisecond)
	}
	defer syscall.Flock(int(guard.Fd()), syscall.LOCK_UN)
	for index := range max {
		path := filepath.Join(directory, fmt.Sprintf("slot-%02d", index))
		file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
		if err != nil {
			t.Fatal(err)
		}
		if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
			_ = file.Close()
			t.Fatalf("private host slot %d was not free behind named gate: %v", index, err)
		}
		*slots = append(*slots, file)
	}
}

func runGLEBatchPortableOwnerMovedOrigin(t *testing.T) {
	batchE2EProcessEnvironment.Lock()
	t.Cleanup(batchE2EProcessEnvironment.Unlock)
	portable := newPortableProofFixture(t)
	// The owner reexecs its own binary for tip and prefix proofs. A fixed
	// command basename carries the existing test load sampler through that
	// child chain, while all public admission and resource paths still run.
	portable.engine = filepath.Join(filepath.Dir(portable.engine), proofrun.TestHostLoadCommandName("0"))
	groupB := portable.group("app-b", "b")
	groupB.Inputs = append(groupB.Inputs, "app/shared-b.txt")
	portable.contract.Groups = append(portable.contract.Groups, groupB, portable.group("app-c", "c"))
	portable.contract.Surfaces = []testpolicy.Surface{
		{ID: "app-a", Paths: []string{"app/a.txt"}, Standard: []string{"app-a"}, Critical: []string{"app-a-observed"}},
		{ID: "app-b", Paths: []string{"app/b.txt", "app/shared-b.txt"}, Standard: []string{"app-b"}, Critical: []string{"app-b-observed"}},
		{ID: "app-c", Paths: []string{"app/c.txt"}, Standard: []string{"app-c"}, Critical: []string{"app-c-observed"}},
		{ID: "control", Paths: []string{"testing.json", "records/**", "plans/goals/**", "memory/**", "metasystem/**", "scripts/**", "metasystem.conf"}, Standard: []string{"app-a"}},
	}
	portable.contract.Cadence = []string{"app-a", "app-b", "app-c"}
	portable.write("app/b.txt", "green b\n", 0o644)
	portable.write("app/c.txt", "green c\n", 0o644)
	portable.writeContract()
	portable.writeBytes("plans/goals/backlog.md", goal.RenderRoot(&goal.RootRecord{
		Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1", SyncMode: goal.SyncRemote, Revision: 1,
	}), 0o644)
	if err := os.Remove(filepath.Join(portable.root, "plans", "goals", "portable.md")); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"goal-a", "goal-b", "goal-c"} {
		path := filepath.Join(portable.root, "plans", "goals", id+".md")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		file, problems := goal.ParseFile(data)
		if len(problems) != 0 {
			t.Fatalf("parse seed goal %s: %v", id, problems)
		}
		lineage := "lineage-" + id
		file.Claimed.Machine, file.Claimed.Lineage = id, lineage
		file.StopCapability.Machine = id
		file.History[1].Actor = id + "+" + lineage
		file.History[1].Opid = goal.Opid(strings.ToUpper(strings.Split(string(file.History[1].Opid), "-")[0]), id, lineage)
		portable.writeBytes("plans/goals/"+id+".md", goal.RenderFile(file), 0o644)
	}
	config, err := os.ReadFile(filepath.Join(portable.root, "metasystem.conf")) // Synthetic fixture config.
	if err != nil {
		t.Fatal(err)
	}
	portable.write("metasystem.conf", string(config)+"goal.human.portable=Portable Fixture <portable@example.invalid>\n", 0o644)
	buildScript := fmt.Sprintf(`#!/bin/sh
set -eu
if [ "${1:-}" = "--trimpath" ]; then
  out="$3"
  mkdir -p "$(dirname "$out")"
  printf 'build\n' >> %s
  printf '#!/bin/sh\n# candidate %%s\nexec %%s "$@"\n' "${METASYSTEM_BUILD_STAMP:-$(git rev-parse HEAD)}" %s > "$out"
else
  mkdir -p bin
  printf '#!/bin/sh\nif [ "${1:-}" = up ]; then exit 0; fi\nexec %%s "$@"\n' %s > bin/metasystem
fi
chmod +x "${out:-bin/metasystem}"
`, strconv.Quote(portable.buildCounter), strconv.Quote(portable.engine), strconv.Quote(portable.engine))
	portable.write("scripts/agents/go-build.sh", buildScript, 0o755)
	portable.write("scripts/agents/commit.sh", batchE2ECommitScript, 0o755)
	portable.commit("seed remote portable batch application")
	baseCommit := portable.git("rev-parse", "HEAD")
	origin := filepath.Join(t.TempDir(), "origin.git")
	if output, err := exec.Command("git", "init", "-q", "-b", "main", "--bare", origin).CombinedOutput(); err != nil {
		t.Fatalf("create portable origin: %v: %s", err, output)
	}
	portable.git("remote", "add", "origin", origin)
	portable.git("push", "-q", "origin", "main")
	portable.git("update-ref", goal.AcceptedRef, baseCommit)

	landing := filepath.Join(t.TempDir(), "landing")
	bed := &batchE2EFixture{t: t, origin: origin, landing: landing, seats: map[string]string{}, now: time.Now().UTC()}
	bed.clone(landing, "landing")
	engine := &batchE2EEngine{path: portable.engine}
	bed.enrollPolicyEngine(baseCommit, engine)
	for _, member := range []struct{ id, input string }{{"goal-a", "a"}, {"goal-b", "b"}, {"goal-c", "c"}} {
		seat := filepath.Join(t.TempDir(), member.id)
		bed.clone(seat, member.id)
		bed.seats[member.id] = seat
		(&batchE2EFixture{t: t, landing: seat, now: bed.now}).enrollPolicyEngine(baseCommit, engine)
		bed.announce(seat, "lineage-"+member.id)
		portableGoalBranch(t, bed, seat, member.id, member.input)
	}
	portable.root = landing
	originalPlanner, originalPrefix := batchowner.BatchTreePlanExecutable, batchowner.BatchPrefixReceiptExecutable
	batchowner.BatchTreePlanExecutable = func() (string, error) { return portable.engine, nil }
	batchowner.BatchPrefixReceiptExecutable = func() (string, error) { return portable.engine, nil }
	t.Cleanup(func() {
		batchowner.BatchTreePlanExecutable, batchowner.BatchPrefixReceiptExecutable = originalPlanner, originalPrefix
	})
	for key, value := range map[string]string{
		"GO_WANT_BATCH_E2E_COMMAND": "1", "METASYSTEM_OWNER_LINEAGE": batchowner.LandingOwnerLineage,
		"METASYSTEM_PROOF_ADMISSION_TEST_DIR":     portable.admissionDir,
		"METASYSTEM_PROOF_ADMISSION_FIXTURE_ROOT": landing,
	} {
		t.Setenv(key, value)
	}
	// Join requires a real landing holder. Suspend its cadence before any
	// member joins, then simulate owner death so a separate CLI tick proves
	// recovery and succession from the persisted batch record.
	ownerCommand := landingBatchChild(portable.proofCommand, portable.commandEnvironment(), portable.engine,
		"owner", "--root", bed.seats["goal-a"], "--landing-root", landing, "--max-wait", "1m", "--interval", "1h")
	ownerCommand.Dir = landing
	ownerCommand.Env = append(ownerCommand.Env, "METASYSTEM_OWNER_LINEAGE="+batchowner.LandingOwnerLineage)
	if err := ownerCommand.Start(); err != nil {
		t.Fatalf("start landing owner: %v", err)
	}
	ownerExited := make(chan struct{})
	var ownerExitErr error
	go func() { ownerExitErr = ownerCommand.Wait(); close(ownerExited) }()
	ownerStopped := false
	t.Cleanup(func() {
		if !ownerStopped {
			_ = ownerCommand.Process.Kill()
			<-ownerExited
		}
	})
	waitForPortableOwnerLease(t, landing, ownerCommand.Process.Pid, ownerExited, func() error { return ownerExitErr })
	if err := ownerCommand.Process.Signal(syscall.SIGSTOP); err != nil {
		t.Fatalf("suspend fixture owner cadence: %v", err)
	}
	join := func(goalID string) string {
		code, stdout, stderr := runOnOwnStreams(func(stdout, stderr io.Writer) int {
			return runLandingBatch([]string{"join", "--root", bed.seats[goalID], "--goal", goalID, "--last"}, stdout, stderr)
		})
		if code != 0 {
			t.Fatalf("join %s exited %d: %s", goalID, code, stderr)
		}
		var joined struct {
			BatchID string `json:"batchId"`
		}
		if err := json.Unmarshal([]byte(stdout), &joined); err != nil || joined.BatchID == "" {
			t.Fatalf("join %s returned %q: %v", goalID, stdout, err)
		}
		return joined.BatchID
	}
	batchID := join("goal-a")
	for _, goalID := range []string{"goal-b", "goal-c"} {
		if got := join(goalID); got != batchID {
			t.Fatalf("%s joined %s, want %s", goalID, got, batchID)
		}
	}
	checkStatus := func(want string) batch.Record {
		code, stdout, stderr := runOnOwnStreams(func(stdout, stderr io.Writer) int {
			return runLandingBatch([]string{"status", "--root", bed.seats["goal-a"], "--batch", batchID}, stdout, stderr)
		})
		if code != 0 {
			t.Fatalf("batch status exited %d: %s", code, stderr)
		}
		var view batchowner.BatchStatusOutput
		if err := json.Unmarshal([]byte(stdout), &view); err != nil || len(view.Batches) != 1 || view.Batches[0].State != want {
			t.Fatalf("batch status at %s: %q, parsed=%+v, err=%v", want, stdout, view, err)
		}
		record := bed.load(batchID)
		if record.State != want {
			t.Fatalf("stored batch state=%s want %s", record.State, want)
		}
		return record
	}
	open := checkStatus(batch.StateOpen)
	oldestJoinedIn := func(record batch.Record) time.Time {
		joined := map[string]bool{}
		for _, unit := range record.Units {
			if unit.State == batch.UnitJoined {
				joined[unit.GoalID] = true
			}
		}
		var oldest time.Time
		for _, entry := range record.History {
			fields := strings.Fields(entry.Detail)
			if entry.Verb != "join" || len(fields) != 2 || fields[1] != "joined" || !joined[fields[0]] {
				continue
			}
			at, err := time.Parse(time.RFC3339Nano, entry.At)
			if err != nil {
				t.Fatal(err)
			}
			if oldest.IsZero() || at.Before(oldest) {
				oldest = at
			}
		}
		return oldest
	}
	oldestJoined := oldestJoinedIn(open)
	if oldestJoined.IsZero() {
		t.Fatal("joined portable batch has no durable join time")
	}
	// A single tick may use the quiet window when the host is idle. Bind the
	// fixture clock past the declared maximum instead, so busy-host load does
	// not decide whether this owner exercises its proof and landing path.
	tickAt := oldestJoined.Add(time.Minute + time.Second)
	if now := time.Now().UTC(); now.After(tickAt) {
		tickAt = now
	}
	t.Setenv("METASYSTEM_GOAL_NOW", tickAt.Format(time.RFC3339Nano))
	if err := ownerCommand.Process.Kill(); err != nil {
		t.Fatalf("stop fixture owner: %v", err)
	}
	<-ownerExited
	ownerStopped = true
	tick := func() batch.Record {
		command := landingBatchChild(portable.proofCommand, portable.commandEnvironment(), portable.engine,
			"tick", "--root", bed.seats["goal-a"], "--landing-root", landing, "--max-wait", "1m", "--batch", batchID)
		command.Dir = landing
		command.Env = append(command.Env, "METASYSTEM_OWNER_LINEAGE="+batchowner.LandingOwnerLineage)
		output, commandErr := command.CombinedOutput()
		if commandErr != nil {
			t.Fatalf("batch tick: %v: %s; state=%s", commandErr, output, bed.load(batchID).State)
		}
		return bed.load(batchID)
	}
	first := tick()
	if first.State != batch.StateLanding || first.Proof == nil || first.Proof.Status != "green" {
		t.Fatalf("real command/JUnit tip proof did not enter landing: %+v", first)
	}
	portable.observe("owner-first-tip", first.TipTree, time.Now())
	checkStatus(batch.StateLanding)
	_, native := portable.counts()
	if native["a"] != 1 || native["b"] != 1 || native["c"] != 1 {
		t.Fatalf("tip native executions=%v, want A/B/C once", native)
	}
	peer := filepath.Join(t.TempDir(), "peer")
	batchE2EGit(t, "", "clone", "-q", origin, peer)
	batchE2EConfigureGit(t, peer, "peer")
	if err := os.WriteFile(filepath.Join(peer, "app", "shared-b.txt"), []byte("shared b v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	batchE2EGit(t, peer, "add", "--", "app/shared-b.txt")
	batchE2EGit(t, peer, "commit", "-qm", "move only B's declared input")
	batchE2EGit(t, peer, "push", "-q", "origin", "main")
	movedTip := bed.originTip()
	if movedTip == baseCommit {
		t.Fatal("fixture did not move bare origin")
	}
	reopened := tick()
	if reopened.State != batch.StateOpen || reopened.Proof != nil || len(reopened.Receipts) != 0 || bed.originTip() != movedTip {
		t.Fatalf("moved declared B input did not reopen proof before publication: state=%s proof=%v receipts=%d origin=%s", reopened.State, reopened.Proof, len(reopened.Receipts), bed.originTip())
	}
	checkStatus(batch.StateOpen)
	_, native = portable.counts()
	if native["a"] != 1 || native["b"] != 1 || native["c"] != 1 {
		t.Fatalf("origin move ran native work before reproof: %v", native)
	}
	reproved := tick()
	// Proof.BaseCommit is the enrolled policy base; Record.BaseTree is the
	// fetched application trunk that the reassembled series actually used.
	movedTree := batchE2EGit(t, origin, "rev-parse", movedTip+"^{tree}")
	if reproved.State != batch.StateLanding || reproved.Proof == nil || reproved.Proof.Status != "green" || reproved.BaseTree != movedTree {
		if reproved.Proof == nil {
			t.Fatalf("moved-base reproof did not enter landing: state=%s proof=nil wantTree=%s", reproved.State, movedTree)
		}
		t.Fatalf("moved-base reproof did not enter landing: state=%s status=%s baseTree=%s wantTree=%s", reproved.State, reproved.Proof.Status, reproved.BaseTree, movedTree)
	}
	if reproved.TipTree == first.TipTree {
		t.Fatal("reproof retained the old cumulative tip tree")
	}
	portable.observe("owner-moved-reproof", reproved.TipTree, time.Now())
	_, native = portable.counts()
	if native["a"] != 1 || native["b"] != 2 || native["c"] != 1 {
		t.Fatalf("moved B input did not invalidate only B: %v", native)
	}
	second := tick()
	if second.State != batch.StateLanded {
		t.Fatalf("owner landing state=%s: %+v", second.State, second)
	}
	checkStatus(batch.StateLanded)
	if len(second.Units) != 3 {
		t.Fatalf("landed batch has %d members, want 3", len(second.Units))
	}
	for _, member := range []struct{ id, input string }{{"goal-a", "a"}, {"goal-b", "b"}, {"goal-c", "c"}} {
		body := batchE2EGit(t, origin, "show", "refs/heads/main:app/"+member.input+".txt")
		if !strings.Contains(body, "goal contribution") {
			t.Fatalf("origin lacks %s contribution: %q", member.id, body)
		}
	}
	if got := batchE2EGit(t, origin, "show", "refs/heads/main:app/shared-b.txt"); got != "shared b v2" {
		t.Fatalf("final atomic origin omitted moved B input: %q", got)
	}
	for _, unit := range second.Units {
		if unit.State == batch.UnitJoining {
			t.Fatalf("member %s remained joining after landing", unit.GoalID)
		}
	}
	receipts := batchE2EGit(t, origin, "show", "refs/heads/main:memory/receipts.log")
	for _, id := range []string{"goal-a", "goal-b", "goal-c"} {
		if !strings.Contains(receipts, "|goal="+id+"|") {
			t.Fatalf("origin receipt log lacks %s: %s", id, receipts)
		}
	}
	_, native = portable.counts()
	if native["a"] != 1 || native["b"] != 2 || native["c"] != 1 {
		t.Fatalf("landed native work changed beyond B: %v", native)
	}
	if got := bed.originTip(); got == baseCommit {
		t.Fatal("atomic landing did not advance the bare origin")
	}
}

// A member can cross its live elapsed stop boundary while the native proof is
// queued for host capacity. The owner must consume the new fence, return that
// member, and prove the remaining member before publishing the origin.
func TestGLEBatchPortableNativeCapacityWaitEjectsElapsedFencedMember(t *testing.T) {
	batchE2EProcessEnvironment.Lock()
	t.Cleanup(batchE2EProcessEnvironment.Unlock)
	// Every child this test starts carries the fixture's owner tag, so a
	// descendant that left the owner's process group or session (a governed
	// run, a proof launcher's own group) is reaped when this test ends, not
	// when the package's custodian drains at the end of the whole suite.
	fixture := testutil.Fixture(t)
	t.Cleanup(func() {
		if err := fixture.HoldOwnedChildren(); err != nil {
			t.Errorf("hold the fixture's surviving children for teardown: %v", err)
		}
	})
	// Civil and boot clocks advance only when the fixture advances them. Native
	// process deadlines remain governed by the test context and kernel clock.
	t0 := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	const fixtureBootID = "portable-native-capacity-fixture"
	fixtureBootAtT0 := 6 * time.Hour
	portable := newPortableProofFixture(t)
	portable.engine = filepath.Join(filepath.Dir(portable.engine), proofrun.TestHostLoadCommandName("0"))
	portable.contract.SchemaVersion = testpolicy.ExecutionContractSchemaVersion
	portable.contract.Groups[0].Phase, portable.contract.Groups[0].EnvironmentMode = "acceptance", "inherit"
	groupB := portable.group("app-b", "b")
	groupB.Phase, groupB.EnvironmentMode = "acceptance", "inherit"
	groupB.Resources = testpolicy.GroupResources{Class: "heavy", Exclusive: []string{"batch-native-gate"}}
	portable.contract.Groups = append(portable.contract.Groups, groupB)
	portable.contract.Surfaces = []testpolicy.Surface{
		{ID: "app-a", Paths: []string{"app/a.txt"}, Standard: []string{"app-a"}, Critical: []string{"app-a-observed"}},
		{ID: "app-b", Paths: []string{"app/b.txt"}, Standard: []string{"app-b"}, Critical: []string{"app-b-observed"}},
		{ID: "control", Paths: []string{"testing.json", "records/**", "plans/goals/**", "memory/**", "metasystem/**", "scripts/**", "metasystem.conf"}, Standard: []string{"app-a"}},
	}
	portable.contract.Cadence = []string{"app-a", "app-b"}
	portable.write("app/b.txt", "green b\n", 0o644)
	portable.writeContract()
	portable.writeBytes("plans/goals/backlog.md", goal.RenderRoot(&goal.RootRecord{
		Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1", SyncMode: goal.SyncRemote, Revision: 1,
	}), 0o644)
	if err := os.Remove(filepath.Join(portable.root, "plans", "goals", "portable.md")); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"goal-a", "goal-b"} {
		path := filepath.Join(portable.root, "plans", "goals", id+".md")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		file, problems := goal.ParseFile(data)
		if len(problems) != 0 {
			t.Fatalf("parse seed goal %s: %v", id, problems)
		}
		file.OpenedAt = t0.Add(-6 * time.Minute).Format(time.RFC3339)
		file.Approved.At = t0.Format(time.RFC3339)
		file.Claimed.At = t0.Add(-time.Minute).Format(time.RFC3339)
		if file.Claimed.EpisodeAt != "" {
			file.Claimed.EpisodeAt = file.Claimed.At
		}
		for index := range file.History {
			switch file.History[index].Verb {
			case "open":
				file.History[index].At = file.OpenedAt
			case "claim":
				file.History[index].At = file.Claimed.At
			case "approve":
				file.History[index].At = file.Approved.At
			}
		}
		lineage := "lineage-" + id
		file.Claimed.Machine, file.Claimed.Lineage = id, lineage
		file.StopCapability.Machine = id
		file.History[1].Actor = id + "+" + lineage
		file.History[1].Opid = goal.Opid(strings.ToUpper(strings.Split(string(file.History[1].Opid), "-")[0]), id, lineage)
		if id == "goal-b" {
			file.Budget.ElapsedLimit = "3m"
			file.Approved.Digest = goal.ApprovalDigest(file.Intent, file.Tier, *file.Budget, file.Risk)
		}
		portable.writeBytes("plans/goals/"+id+".md", goal.RenderFile(file), 0o644)
	}
	config, err := os.ReadFile(filepath.Join(portable.root, "metasystem.conf")) // Synthetic fixture config.
	if err != nil {
		t.Fatal(err)
	}
	portable.write("metasystem.conf", string(config)+"goal.human.portable=Portable Fixture <portable@example.invalid>\n", 0o644)
	buildScript := fmt.Sprintf(`#!/bin/sh
set -eu
if [ "${1:-}" = "--trimpath" ]; then
  out="$3"
  mkdir -p "$(dirname "$out")"
  printf 'build\n' >> %s
  printf '#!/bin/sh\n# candidate %%s\nexec %%s "$@"\n' "${METASYSTEM_BUILD_STAMP:-$(git rev-parse HEAD)}" %s > "$out"
else
  mkdir -p bin
  printf '#!/bin/sh\nif [ "${1:-}" = up ]; then exit 0; fi\nexec %%s "$@"\n' %s > bin/metasystem
fi
chmod +x "${out:-bin/metasystem}"
`, strconv.Quote(portable.buildCounter), strconv.Quote(portable.engine), strconv.Quote(portable.engine))
	portable.write("scripts/agents/go-build.sh", buildScript, 0o755)
	portable.write("scripts/agents/commit.sh", batchE2ECommitScript, 0o755)
	// The steward may append during the stop. Keep the real append-only
	// registers present in the base tree so rearming can prove their suffixes.
	portable.write("records/narrator-digest.log", "", 0o644)
	portable.write("memory/receipts.log", "", 0o644)
	portable.commit("seed elapsed-fence portable batch application")
	baseCommit := portable.git("rev-parse", "HEAD")
	origin := filepath.Join(t.TempDir(), "origin.git")
	if output, err := exec.Command("git", "init", "-q", "-b", "main", "--bare", origin).CombinedOutput(); err != nil {
		t.Fatalf("create portable origin: %v: %s", err, output)
	}
	portable.git("remote", "add", "origin", origin)
	portable.git("push", "-q", "origin", "main")
	portable.git("update-ref", goal.AcceptedRef, baseCommit)
	landing := filepath.Join(t.TempDir(), "landing")
	bed := &batchE2EFixture{t: t, origin: origin, landing: landing, seats: map[string]string{}, now: t0}
	bed.clone(landing, "landing")
	engine := &batchE2EEngine{path: portable.engine}
	bed.enrollPolicyEngine(baseCommit, engine)
	for _, member := range []struct{ id, input string }{{"goal-a", "a"}, {"goal-b", "b"}} {
		seat := filepath.Join(t.TempDir(), member.id)
		bed.clone(seat, member.id)
		bed.seats[member.id] = seat
		(&batchE2EFixture{t: t, landing: seat, now: bed.now}).enrollPolicyEngine(baseCommit, engine)
		bed.announce(seat, "lineage-"+member.id)
		portableGoalBranch(t, bed, seat, member.id, member.input)
	}
	portable.root = landing
	for key, value := range map[string]string{
		"GO_WANT_BATCH_E2E_COMMAND": "1", "METASYSTEM_OWNER_LINEAGE": batchowner.LandingOwnerLineage,
		"METASYSTEM_PROOF_ADMISSION_TEST_DIR":     portable.admissionDir,
		"METASYSTEM_PROOF_ADMISSION_FIXTURE_ROOT": landing,
	} {
		t.Setenv(key, value)
	}
	t.Setenv("METASYSTEM_GOAL_NOW", t0.Format(time.RFC3339))
	t.Setenv("METASYSTEM_GOAL_BOOT_ID", fixtureBootID)
	t.Setenv("METASYSTEM_GOAL_BOOT_NANOS", strconv.FormatInt(fixtureBootAtT0.Nanoseconds(), 10))
	clockEnvironment := func(at time.Time) []string {
		var env []string
		for _, entry := range portable.commandEnvironment() {
			if !strings.HasPrefix(entry, "METASYSTEM_GOAL_NOW=") &&
				!strings.HasPrefix(entry, "METASYSTEM_GOAL_BOOT_ID=") &&
				!strings.HasPrefix(entry, "METASYSTEM_GOAL_BOOT_NANOS=") {
				env = append(env, entry)
			}
		}
		elapsed := fixtureBootAtT0 + at.Sub(t0)
		return fixture.Env(append(env,
			"METASYSTEM_GOAL_NOW="+at.Format(time.RFC3339),
			"METASYSTEM_GOAL_BOOT_ID="+fixtureBootID,
			"METASYSTEM_GOAL_BOOT_NANOS="+strconv.FormatInt(elapsed.Nanoseconds(), 10),
		))
	}
	// The owner and the successor each lead a process group of their own;
	// killing one kills its whole group before the leader is reaped, so no
	// child of it outlives the kill.
	killGroup := func(command *exec.Cmd) error {
		if err := syscall.Kill(-command.Process.Pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
			return err
		}
		return nil
	}
	commandAt := func(at time.Time, args ...string) *exec.Cmd {
		return portable.proofCommand.command(clockEnvironment(at), portable.engine, args...)
	}
	batchAt := func(at time.Time, args ...string) *exec.Cmd {
		return landingBatchChild(portable.proofCommand, clockEnvironment(at), portable.engine, args...)
	}
	owner := batchAt(t0, "owner", "--root", bed.seats["goal-a"], "--landing-root", landing, "--max-wait", "1m", "--interval", "1h")
	owner.Dir = landing
	owner.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := owner.Start(); err != nil {
		t.Fatal(err)
	}
	ownerExited := make(chan struct{})
	var ownerExitErr error
	go func() { ownerExitErr = owner.Wait(); close(ownerExited) }()
	ownerStopped := false
	t.Cleanup(func() {
		if !ownerStopped {
			_ = killGroup(owner)
			<-ownerExited
		}
	})
	waitForPortableOwnerLease(t, landing, owner.Process.Pid, ownerExited, func() error { return ownerExitErr })
	if err := owner.Process.Signal(syscall.SIGSTOP); err != nil {
		t.Fatal(err)
	}
	join := func(goalID string) string {
		command := batchAt(t0, "join", "--root", bed.seats[goalID], "--goal", goalID, "--last")
		command.Dir = bed.seats[goalID]
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("join %s: %v: %s", goalID, err, output)
		}
		var joined struct {
			BatchID string `json:"batchId"`
		}
		if err := json.Unmarshal(output, &joined); err != nil || joined.BatchID == "" {
			t.Fatalf("join %s returned %q: %v", goalID, output, err)
		}
		return joined.BatchID
	}
	batchID := join("goal-a")
	if got := join("goal-b"); got != batchID {
		t.Fatalf("B joined %s, want %s", got, batchID)
	}
	if err := killGroup(owner); err != nil {
		t.Fatal(err)
	}
	<-ownerExited
	ownerStopped = true
	_, nativeBefore := portable.counts()
	conf := filepath.Join(landing, "metasystem.conf")
	controlRoot, err := canonicalProofRoot(landing)
	if err != nil {
		t.Fatal(err)
	}
	priorAttempts, err := proofrun.ReadAttempts(controlRoot)
	if err != nil {
		t.Fatal(err)
	}
	priorIDs := map[string]bool{}
	for _, attempt := range priorAttempts {
		priorIDs[attempt.AttemptID] = true
	}
	// A public one-shot tick uses the governing fixture clock to close the
	// elapsed membership window, then holds proof while we saturate capacity.
	successor := batchAt(t0.Add(time.Minute+time.Second), "tick", "--root", bed.seats["goal-a"], "--landing-root", landing, "--max-wait", "1m", "--batch", batchID)
	successor.Dir = landing
	successor.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	var ownerOutput bytes.Buffer
	successor.Stdout, successor.Stderr = &ownerOutput, &ownerOutput
	if err := successor.Start(); err != nil {
		t.Fatal(err)
	}
	ownerDone := make(chan error, 1)
	go func() { ownerDone <- successor.Wait() }()
	successorFinished := false
	t.Cleanup(func() {
		if !successorFinished {
			_ = killGroup(successor)
			<-ownerDone
		}
	})
	// Let the owner close membership before occupying a host resource. The
	// owner's load sample intentionally treats another lease as active work.
	planned := false
	planningTicker := time.NewTicker(20 * time.Millisecond)
	defer planningTicker.Stop()
	for !planned {
		if state := bed.load(batchID).State; state == batch.StateProving {
			planned = true
			break
		}
		select {
		case err := <-ownerDone:
			successorFinished = true
			t.Fatalf("successor owner ended before proof planning: %v: %s; state=%s", err, ownerOutput.String(), bed.load(batchID).State)
		case <-t.Context().Done():
			t.Fatalf("successor owner did not plan proof before test cancellation: %v; state=%s", t.Context().Err(), bed.load(batchID).State)
		case <-planningTicker.C:
		}
	}
	if !planned {
		t.Fatalf("owner did not plan a proof before native gate: state=%s owner=%s", bed.load(batchID).State, ownerOutput.String())
	}
	_, beforeGate := portable.counts()
	if beforeGate["a"] != nativeBefore["a"] || beforeGate["b"] != nativeBefore["b"] {
		t.Fatalf("member command started before native gate: before=%v current=%v", nativeBefore, beforeGate)
	}
	named, err := proofrun.AcquireHostResources(context.Background(), landing, conf, "cheap", []string{"batch-native-gate"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = named.Close() })
	admitted := false
	producerID := ""
	admissionTicker := time.NewTicker(50 * time.Millisecond)
	defer admissionTicker.Stop()
	for !admitted {
		attempts, readErr := proofrun.ReadAttempts(controlRoot)
		if readErr != nil {
			t.Fatal(readErr)
		}
		for _, attempt := range attempts {
			if !priorIDs[attempt.AttemptID] && attempt.TestOwned["app-b"] != "" {
				admitted = true
				producerID = attempt.AttemptID
			}
		}
		if admitted {
			break
		}
		select {
		case err := <-ownerDone:
			successorFinished = true
			t.Fatalf("successor owner ended before native admission: %v: %s; state=%s attempts=%d", err, ownerOutput.String(), bed.load(batchID).State, len(attempts))
		case <-t.Context().Done():
			t.Fatalf("successor owner did not admit native attempt before test cancellation: %v; state=%s attempts=%d", t.Context().Err(), bed.load(batchID).State, len(attempts))
		case <-admissionTicker.C:
		}
	}
	if !admitted {
		attempts, readErr := proofrun.ReadAttempts(controlRoot)
		builds, counts := portable.counts()
		t.Fatalf("real native attempt was not admitted before wait: state=%s attempts=%v read=%v builds=%d commands=%v owner=%s", bed.load(batchID).State, attempts, readErr, builds, counts, ownerOutput.String())
	}
	_, nativeQueued := portable.counts()
	if nativeQueued["a"] != nativeBefore["a"] || nativeQueued["b"] != nativeBefore["b"] {
		t.Fatalf("member commands started behind named resource gate: before=%v queued=%v", nativeBefore, nativeQueued)
	}
	var slots []*os.File
	t.Cleanup(func() {
		for _, slot := range slots {
			_ = slot.Close()
		}
	})
	admissionCap, err := proofrun.ResolveAdmissionCap(conf, runtime.NumCPU())
	if err != nil || admissionCap.Max < 1 {
		t.Fatalf("resolve synthetic host admission cap: %+v, %v", admissionCap, err)
	}
	holdPortableFixtureSlots(t, landing, admissionCap.Max, &slots)
	if len(slots) != admissionCap.Max {
		t.Fatalf("private host slots held = %d, want all %d", len(slots), admissionCap.Max)
	}
	if err := named.Close(); err != nil {
		t.Fatal(err)
	}
	capacityProbe, probeCancel := context.WithCancel(t.Context())
	capacityBlocked := false
	capacityProbe = proofrun.WithHostResourceWaitObserver(capacityProbe, func() {
		capacityBlocked = true
		probeCancel()
	})
	if contender, err := proofrun.AcquireHostResources(capacityProbe, landing, conf, "heavy", nil); err == nil {
		_ = contender.Close()
		t.Fatalf("native capacity was not saturated by %d real host leases", admissionCap.Max)
	} else if !capacityBlocked || !errors.Is(err, context.Canceled) {
		t.Fatalf("capacity probe did not reach a failed scan: blocked=%t err=%v", capacityBlocked, err)
	}
	probeCancel()
	_, nativeQueued = portable.counts()
	if nativeQueued["a"] != nativeBefore["a"] || nativeQueued["b"] != nativeBefore["b"] {
		t.Fatalf("member commands started while all host slots were held: before=%v queued=%v", nativeBefore, nativeQueued)
	}
	t1 := t0.Add(4 * time.Minute)
	fetch := commandAt(t1, "goal", "fetch", "--root", controlRoot)
	fetch.Dir = controlRoot
	if output, fetchErr := fetch.CombinedOutput(); fetchErr != nil {
		t.Fatalf("public goal fetch before steward stop: %v: %s", fetchErr, output)
	}
	endpoint, err := goal.ResolveEndpoint(controlRoot)
	if err != nil {
		t.Fatal(err)
	}
	projection, err := goal.Project(endpoint, true, t1)
	if err != nil || projection.Tree == nil || projection.Tree.Live["goal-b"] == nil || projection.Tree.Live["goal-b"].Claimed == nil {
		t.Fatalf("live B goal before elapsed stop: projection=%+v err=%v", projection, err)
	}
	if projection.Tree.Live["goal-b"].IsLandingClaim() {
		t.Fatal("B unexpectedly entered land-ready; elapsed stop is suspended for landing claims")
	}
	revision := projection.Tree.Live["goal-b"].Claimed.Revision
	// The breach-stop route scan is the dispatch owner's (its internal verb
	// went in U9a); the steward runner below consumes the same routes.
	routes, routesErr := dispatchcore.FindBreachStops(controlRoot, t1)
	breachRoute := false
	for _, route := range routes {
		if route.GoalID == "goal-b" && route.Revision == revision && route.Failure == "" {
			breachRoute = true
		}
	}
	if routesErr != nil || !breachRoute {
		t.Fatalf("elapsed breach route absent for active B: revision=%d routes=%+v error=%v budget=%+v", revision, routes, routesErr, dispatchcore.ProjectBudget(controlRoot, projection.Tree.Live["goal-b"], t1))
	}
	// The enrolled steward is the authorized stop custodian while the owner
	// holds the checkout: its resident runner (`steward run`, its own session
	// as steward arm launches it) runs the delegate lifecycle's breach stop in
	// process under the real classifier. The runner's terminal fact is staged
	// false in the fixture identity table, so the verdict is STEWARD wherever
	// the suite runs, never the test process's own terminal.
	portable.git("config", "metasystem.steward.tick-seconds", "1")
	runner, runnerLog, runnerExited := startPortableStewardRunner(t, portable.engine, controlRoot, clockEnvironment(t1))
	var completedStopID string
	for completedStopID == "" {
		select {
		case runErr := <-runnerExited:
			t.Fatalf("the steward runner exited before B's elapsed stop completed: %v: %s", runErr, readPortableRunnerLog(t, runnerLog))
		case <-t.Context().Done():
			t.Fatalf("the steward runner did not complete B's elapsed stop for revision %d before test cancellation: %s", revision, readPortableRunnerLog(t, runnerLog))
		case <-time.After(100 * time.Millisecond):
		}
		observed, observeErr := goal.Project(endpoint, true, t1)
		if observeErr != nil || observed.Tree == nil || observed.Tree.Live["goal-b"] == nil || observed.Tree.Live["goal-b"].StopFence == nil {
			continue
		}
		stopID := observed.Tree.Live["goal-b"].StopFence.StopID
		if batch, batchErr := goal.ReadStopBatch(controlRoot, stopID); batchErr == nil && batch.State == goal.StopBatchComplete {
			completedStopID = stopID
		}
	}
	stopPortableStewardRunner(t, runner, runnerExited)
	stopOutput := readPortableRunnerLog(t, runnerLog)
	stopBatch, err := goal.ReadStopBatch(controlRoot, completedStopID)
	if err != nil || stopBatch.State != goal.StopBatchComplete || stopBatch.GoalID != "goal-b" || stopBatch.GoalRevision != revision {
		t.Fatalf("real stop batch did not complete B's exact revision %d: id=%s batch=%+v err=%v", revision, completedStopID, stopBatch, err)
	}
	projection, err = goal.Project(endpoint, true, t1)
	if err != nil || projection.Tree == nil || projection.Tree.Live["goal-b"] == nil || !projection.Tree.Live["goal-b"].IsFencedClaim() {
		t.Fatalf("elapsed stop did not publish B's live fence: revision=%d steward=%s projection=%+v err=%v", revision, stopOutput, projection, err)
	}
	queuedAttempt, err := proofrun.ReadAttempt(controlRoot, producerID)
	if err != nil {
		t.Fatal(err)
	}
	if queuedAttempt.GoalID != "goal-b" || queuedAttempt.GoalRevision != revision || queuedAttempt.CancellationIntent == "" ||
		queuedAttempt.Terminal == nil || queuedAttempt.Terminal.Result != proofrun.TerminalCancelled {
		t.Fatalf("stop missed exact queued proof: attempt=%s goal=%s revision=%d cancellation=%q terminal=%+v processes=%v steward=%s", producerID,
			queuedAttempt.GoalID, queuedAttempt.GoalRevision, queuedAttempt.CancellationIntent, queuedAttempt.Terminal, queuedAttempt.ProcessKeys, stopOutput)
	}
	_, nativeQueued = portable.counts()
	if nativeQueued["a"] != nativeBefore["a"] || nativeQueued["b"] != nativeBefore["b"] {
		t.Fatalf("member commands started before capacity release: before=%v queued=%v", nativeBefore, nativeQueued)
	}
	for _, slot := range slots {
		if err := slot.Close(); err != nil {
			t.Fatal(err)
		}
	}
	slots = nil
	select {
	case err := <-ownerDone:
		successorFinished = true
		red := bed.load(batchID)
		if red.State != batch.StateDiagnosing || red.Proof == nil || red.Proof.Status != "failed" {
			t.Fatalf("public capacity-wait proof tick did not retain its red result: %v: %s; state=%s proof=%+v", err, ownerOutput.String(), red.State, red.Proof)
		}
	case <-t.Context().Done():
		t.Fatalf("public capacity-wait proof tick did not finish after release before test cancellation: %v; state=%s", t.Context().Err(), bed.load(batchID).State)
	}
	var survivorClocks []time.Time
	for step := range 12 {
		current := bed.load(batchID)
		if current.State == batch.StateLanded && current.Units[0].State == batch.UnitLanded {
			break
		}
		followAt := t1.Add(time.Duration(step+1) * time.Minute)
		survivorClocks = append(survivorClocks, followAt)
		follow := batchAt(followAt, "tick", "--root", bed.seats["goal-a"], "--landing-root", landing, "--max-wait", "1m", "--batch", batchID)
		follow.Dir = landing
		output, tickErr := follow.CombinedOutput()
		if tickErr != nil {
			t.Fatalf("public survivor tick %d: %v: %s; state=%s", step+1, tickErr, output, bed.load(batchID).State)
		}
		current = bed.load(batchID)
		t.Logf("post-fence public tick %d: batch=%s A=%s B=%s", step+1, current.State, current.Units[0].State, current.Units[1].State)
	}
	landed := bed.load(batchID)
	if landed.State != batch.StateLanded || len(landed.Units) != 2 || landed.Units[0].State != batch.UnitLanded || landed.Units[1].State != batch.UnitEjected ||
		!strings.Contains(landed.Units[1].Failure, "state=fenced") || landed.Proof == nil || landed.Proof.Status != "green" {
		t.Fatalf("fenced member did not yield green survivor landing: state=%s units=%+v proof=%+v history=%+v owner=%s", landed.State, landed.Units, landed.Proof, landed.History, ownerOutput.String())
	}
	if got := batchE2EGit(t, origin, "show", "refs/heads/main:app/a.txt"); !strings.Contains(got, "goal contribution") {
		t.Fatalf("survivor A absent from origin: %q", got)
	}
	if got := batchE2EGit(t, origin, "show", "refs/heads/main:app/b.txt"); strings.Contains(got, "goal contribution") {
		t.Fatalf("fenced B was published to origin: %q", got)
	}
	receipts := batchE2EGit(t, origin, "show", "refs/heads/main:memory/receipts.log")
	if !strings.Contains(receipts, "|goal=goal-a|") || strings.Contains(receipts, "|goal=goal-b|") {
		t.Fatalf("survivor-only receipts were not published: %s", receipts)
	}
	_, nativeAfter := portable.counts()
	if nativeAfter["a"] <= nativeBefore["a"] {
		t.Fatalf("survivor proof never ran its real command/JUnit check: before=%v after=%v", nativeBefore, nativeAfter)
	}
	requirePortablePrefixTimestampEvidence(t, controlRoot, queuedAttempt, landed, t1, survivorClocks)
}

func requirePortablePrefixTimestampEvidence(t *testing.T, controlRoot string, cancelled proofrun.Attempt, landed batch.Record, cancelClock time.Time, survivorClocks []time.Time) {
	t.Helper()
	cancelledIdentity := cancelled.TestOwned["app-a"]
	if cancelledIdentity == "" || cancelled.TestOwned["app-b"] == "" || cancelled.TestOwned["app-b"] == cancelledIdentity {
		t.Fatalf("cancelled union does not retain distinct A and B ownership: %+v", cancelled.TestOwned)
	}
	if cancelled.Terminal == nil || cancelled.Terminal.Result != proofrun.TerminalCancelled {
		t.Fatalf("cancelled union has no cancellation terminal: %+v", cancelled.Terminal)
	}
	cancelledAt, err := time.Parse(time.RFC3339Nano, cancelled.Terminal.At)
	if err != nil || !cancelledAt.Equal(cancelClock) {
		t.Fatalf("cancelled union terminal is outside its governing fixture clock: at=%q want=%s err=%v", cancelled.Terminal.At, cancelClock.Format(time.RFC3339Nano), err)
	}

	if landed.Proof == nil || landed.Proof.Status != "green" || landed.Proof.GroupIdentities["app-a"] != cancelledIdentity {
		t.Fatalf("survivor proof is not the matching green app-a tip: cancelled=%s proof=%+v", cancelledIdentity, landed.Proof)
	}
	reusedProducerID, reusePresent := landed.Proof.Reuse["app-a"]
	executed := false
	for _, groupID := range landed.Proof.Executions {
		if groupID == "app-a" {
			executed = true
			break
		}
	}
	if reusePresent && reusedProducerID == "" {
		t.Fatalf("survivor proof has empty app-a reuse provenance: %+v", landed.Proof)
	}
	if reusedProducerID != "" && executed {
		t.Fatalf("survivor proof both reused and executed app-a: %+v", landed.Proof)
	}
	selectorStatus := "reused"
	selectedProducerID := reusedProducerID
	if selectedProducerID == "" {
		if !executed || landed.Proof.AttemptID == "" {
			t.Fatalf("survivor proof has no app-a producer: %+v", landed.Proof)
		}
		selectorStatus = "executed"
		selectedProducerID = landed.Proof.AttemptID
	}
	producer, err := proofrun.ReadAttempt(controlRoot, selectedProducerID)
	if err != nil {
		t.Fatalf("read selected app-a producer %s: %v", selectedProducerID, err)
	}
	if producer.AttemptID != selectedProducerID || producer.Terminal == nil || producer.Terminal.Result != proofrun.TerminalSuccess || producer.TestResult == nil {
		t.Fatalf("selected app-a producer is not a terminal native green: %+v", producer)
	}
	if cancelled.FreshnessEpisode != "" || cancelled.FreshnessBinding != "" || cancelled.FreshnessExpiresAt != "" ||
		producer.FreshnessEpisode != "" || producer.FreshnessBinding != "" || producer.FreshnessExpiresAt != "" ||
		!proofrun.MatchesTestResultFreshness(cancelled, *producer.TestResult, "app-a") ||
		!proofrun.MatchesTestResultFreshness(producer, *producer.TestResult, "app-a") {
		t.Fatalf("ordinary app-a proof has inconsistent freshness: cancelled=(%q,%q,%q) producer=(%q,%q,%q)",
			cancelled.FreshnessEpisode, cancelled.FreshnessBinding, cancelled.FreshnessExpiresAt,
			producer.FreshnessEpisode, producer.FreshnessBinding, producer.FreshnessExpiresAt)
	}

	var green proofrun.GroupResult
	for _, group := range producer.TestResult.Groups {
		if group.ID == "app-a" {
			green = group
			break
		}
	}
	if green.ID == "" || green.Status != "passed" || !green.CollectionComplete || !proofrun.NativeTestProducer(producer, green) ||
		green.ExecutionIdentity != cancelledIdentity || producer.TestOwned["app-a"] != cancelledIdentity {
		t.Fatalf("selected app-a record is not the matching owned native green: cancelled=%s producer-owned=%s group=%+v proof=%+v",
			cancelledIdentity, producer.TestOwned["app-a"], green, landed.Proof)
	}
	startedAt, startErr := time.Parse(time.RFC3339Nano, producer.StartedAt)
	groupStartedAt, groupStartErr := time.Parse(time.RFC3339Nano, green.StartedAt)
	groupEndedAt, groupEndErr := time.Parse(time.RFC3339Nano, green.EndedAt)
	producerEndedAt, producerEndErr := time.Parse(time.RFC3339Nano, producer.Terminal.At)
	if startErr != nil || groupStartErr != nil || groupEndErr != nil || producerEndErr != nil {
		t.Fatalf("selected app-a timestamps are malformed: attempt-start=%q group=(%q,%q) terminal=%q errors=(%v,%v,%v,%v)",
			producer.StartedAt, green.StartedAt, green.EndedAt, producer.Terminal.At, startErr, groupStartErr, groupEndErr, producerEndErr)
	}
	clockMatched := false
	for _, at := range survivorClocks {
		if startedAt.Equal(at) && groupStartedAt.Equal(at) && groupEndedAt.Equal(at) && producerEndedAt.Equal(at) {
			clockMatched = true
			break
		}
	}
	if !clockMatched || !groupEndedAt.After(cancelledAt) {
		t.Fatalf("selector timestamps do not retain the safe fixture-clock order: cancelled=%s producer-start=%s group=(%s,%s) producer-end=%s clocks=%v",
			cancelledAt.Format(time.RFC3339Nano), startedAt.Format(time.RFC3339Nano), groupStartedAt.Format(time.RFC3339Nano),
			groupEndedAt.Format(time.RFC3339Nano), producerEndedAt.Format(time.RFC3339Nano), survivorClocks)
	}
	if green.DurationMS < 0 || green.CPUSeconds < 0 {
		t.Fatalf("selected app-a physical observations are invalid: duration-ms=%d cpu-seconds=%f", green.DurationMS, green.CPUSeconds)
	}

	marker := struct {
		SchemaVersion       int     `json:"schemaVersion"`
		HistoricalR2Stamps  string  `json:"historicalR2Stamps"`
		Component           string  `json:"component"`
		ExecutionIdentity   string  `json:"executionIdentity"`
		CancelledAttempt    string  `json:"cancelledAttempt"`
		CancelledTerminalAt string  `json:"cancelledTerminalAt"`
		NativeGreenAttempt  string  `json:"nativeGreenAttempt"`
		NativeGreenStarted  string  `json:"nativeGreenStartedAt"`
		NativeGreenEnded    string  `json:"nativeGreenEndedAt"`
		PhysicalDurationMS  int64   `json:"physicalDurationMs"`
		PhysicalCPUSeconds  float64 `json:"physicalCpuSeconds"`
		FreshnessEpisode    string  `json:"freshnessEpisode"`
		FreshnessBinding    string  `json:"freshnessBinding"`
		FreshnessExpiresAt  string  `json:"freshnessExpiresAt"`
		SelectorOrder       string  `json:"selectorOrder"`
		SelectorStatus      string  `json:"selectorStatus"`
		SelectedAttempt     string  `json:"selectedAttempt"`
	}{
		SchemaVersion: 1, HistoricalR2Stamps: "unavailable", Component: "app-a", ExecutionIdentity: cancelledIdentity,
		CancelledAttempt: cancelled.AttemptID, CancelledTerminalAt: cancelled.Terminal.At,
		NativeGreenAttempt: producer.AttemptID, NativeGreenStarted: green.StartedAt, NativeGreenEnded: green.EndedAt,
		PhysicalDurationMS: green.DurationMS, PhysicalCPUSeconds: green.CPUSeconds,
		FreshnessEpisode: producer.FreshnessEpisode, FreshnessBinding: producer.FreshnessBinding, FreshnessExpiresAt: producer.FreshnessExpiresAt,
		SelectorOrder: "cancelled-terminal-at<green-group-ended-at", SelectorStatus: selectorStatus, SelectedAttempt: selectedProducerID,
	}
	encoded, err := json.Marshal(marker)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("TB-CMD003-EVIDENCE %s", encoded)
}

func portableGoalBranch(t *testing.T, bed *batchE2EFixture, seat, goalID, input string) {
	t.Helper()
	base := batchE2EGit(t, seat, "rev-parse", "origin/main")
	path := "app/" + input + ".txt"
	if err := os.WriteFile(filepath.Join(seat, filepath.FromSlash(path)), []byte("green "+input+" goal contribution\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	batchE2EGit(t, seat, "add", "--", path)
	commit, err := goalbranch.CommitStaged(goalbranch.CommitRequest{Repo: seat, Remote: "origin", EndpointTip: base,
		GoalID: goalID, Unit: "u1", OpID: "build-" + goalID, Kind: goalbranch.Unit, CheckClaim: func() error { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	subject, present, err := dispatchcore.ComputeReadSubject(dispatchcore.ReadSubjectRequest{RepoRoot: seat, Role: "code-critic", Reviews: "commit:" + commit})
	if err != nil || !present {
		t.Fatalf("compute %s read subject: present=%t error=%v", goalID, present, err)
	}
	job := "critic-" + goalID
	bed.writeJSON(filepath.Join(seat, "artifacts", "agents", "jobs", job+".json"), map[string]any{
		"jobId": job, "goalId": nil, "role": "code-critic", "round": 1, "status": "completed", "chainClosed": true,
		"findingRegister": []any{}, "findingRegisterRound": 1, "findingRegisterSubjectDigest": subject.Digest(),
		"closure": map[string]any{"criticRoot": job, "round": 1, "subject": subject, "mechanism": "clean"},
	})
	bed.writeJSON(filepath.Join(seat, "artifacts", "agents", job, "rounds", "1", "subject.json"), subject)
	bed.writeJSON(filepath.Join(seat, "artifacts", "agents", job, "rounds", "1", "return.json"), map[string]any{
		"jobId": job, "round": 1, "reviewedTree": subject.Tree,
	})
	if _, _, err := goalbranch.CommitRead(goalbranch.CommitReadRequest{Repo: seat, Remote: "origin", EndpointTip: base,
		GoalID: goalID, Unit: "u1", OpID: "read-" + goalID, RootJob: job, GateRunID: "fast-" + goalID,
		GateTree: subject.Tree, CheckClaim: func() error { return nil }}); err != nil {
		t.Fatal(err)
	}
	batchE2EGit(t, seat, "push", "-q", "origin", "refs/heads/goal/"+goalID+":refs/heads/goal/"+goalID)
	if tip := batchE2EGit(t, bed.origin, "rev-parse", "refs/heads/goal/"+goalID); tip == base {
		t.Fatalf("%s goal branch did not advance origin", goalID)
	}
}

// startPortableStewardRunner starts the enrolled engine's resident runner as
// steward arm does (steward run in its own session) with its terminal fact
// staged false: a shell gate holds the process until its pid's fact is in
// the fixture identity table, then execs the runner in that same process, so
// the classifier never reads the kernel's terminal for it.
func startPortableStewardRunner(t *testing.T, engine, repo string, environment []string) (*exec.Cmd, string, <-chan error) {
	t.Helper()
	dir := t.TempDir()
	table := filepath.Join(dir, "runner-identity.json")
	log := filepath.Join(dir, "runner.log")
	output, err := os.Create(log)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = output.Close() })
	runner := exec.Command("/bin/sh", "-c", `read -r gate && exec "$0" "$@"`, engine, "steward", "run", "--repo", repo, "--lineage", "no-lease")
	runner.Dir = repo
	runner.Env = append(append([]string(nil), environment...), "METASYSTEM_FAKE_PROCESS_IDENTITY_FILE="+table)
	runner.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	runner.Stdout, runner.Stderr = output, output
	gate, err := runner.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	pid := 0
	testenv.ReapFixtureProcessGroups(t, []testenv.FixtureProcessGroup{{
		Verb:    "portable steward runner",
		Resolve: func() (int, bool, error) { return pid, pid != 0, nil },
	}})
	if err := runner.Start(); err != nil {
		t.Fatal(err)
	}
	pid = runner.Process.Pid
	exited := make(chan error, 1)
	go func() { exited <- runner.Wait() }()
	if err := os.WriteFile(table, fmt.Appendf(nil, `{"%d": {"terminal": false}}`, runner.Process.Pid), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := gate.Write([]byte("go\n")); err != nil {
		t.Fatalf("release the steward runner gate: %v", err)
	}
	if err := gate.Close(); err != nil {
		t.Fatal(err)
	}
	return runner, log, exited
}

// stopPortableStewardRunner ends the runner's session and waits for its exit.
func stopPortableStewardRunner(t *testing.T, runner *exec.Cmd, exited <-chan error) {
	t.Helper()
	if err := syscall.Kill(-runner.Process.Pid, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("stop the steward runner: %v", err)
	}
	select {
	case <-exited:
	case <-t.Context().Done():
		t.Fatalf("the steward runner did not exit after SIGTERM before test cancellation")
	}
}

func readPortableRunnerLog(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		return err.Error()
	}
	return string(data)
}

// landingBatchHelperCommand runs the landing batch owner, a one-shot tick or
// a join in a child of this test binary. The engine has no landing batch
// entry any more (production runs the owner as the supervised landing-owner
// component, and work land joins in process); these beds keep each owner in
// a process of its own so it holds the landing lease under its own pid, can
// be stopped with SIGSTOP and killed, and a successor tick recovers from the
// persisted record alone. The child re-executes the enrolled fixture engine
// for tree plans and prefix receipts, as the former engine entry did with its
// own executable, and commits through the planted commit script, as that
// plantedcommit engine did.
const landingBatchHelperCommand = "test-helper-landing-batch"

const landingBatchHelperEngine = "METASYSTEM_TEST_LANDING_BATCH_ENGINE"

func init() {
	testHelperCommands[landingBatchHelperCommand] = func(args []string) int {
		if engine := os.Getenv(landingBatchHelperEngine); engine != "" {
			executable := func() (string, error) { return engine, nil }
			batchowner.BatchTreePlanExecutable, batchowner.BatchPrefixReceiptExecutable = executable, executable
		}
		// The enrolled fixture engine is a plantedcommit build; the child
		// commits each unit through the bed's planted commit script as it did.
		batchowner.BatchCommitBoundary = plantedOrLandingCommit
		return runLandingBatch(args, os.Stdout, os.Stderr)
	}
}

func landingBatchChild(fixture proofBinaryFixture, environment []string, engine string, args ...string) *exec.Cmd {
	fixture.t.Helper()
	command := fixture.command(environment, commandTestExecutable(fixture.t), append([]string{landingBatchHelperCommand}, args...)...)
	command.Env = append(command.Env, "GO_WANT_BATCH_E2E_COMMAND=1", landingBatchHelperEngine+"="+engine)
	return command
}
