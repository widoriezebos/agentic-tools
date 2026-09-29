package delegation_test

// Ported from dispatch-fixtures.sh cluster e (lines 4401-4911) over the
// dispatch integration bed: whole mission-scoped dispatches (the signed
// dispatch envelope, the mission fences and their asks, mission provenance
// in the record and the packet), the approved escalation's record, and the
// continuation after a cap under a runtime without resume. Real Git is the
// claim: brief authority, the record build over the workspace head, the
// worktree and the follow-up rebase plan read it inside internal/dispatch,
// and the signed envelope verifies its contract against a fetched origin.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/adapter"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/contract"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegation"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
)

const p7JobCapMin = 60

// p7MissionContract is mission-alpha's contract: the four lifecycle fences,
// the per-job cap, and a dispatch envelope that admits the escalated fake
// pair. sealed adds the seal block the envelope requires.
func p7MissionContract(wallHours, cycles, jobs, concurrency int, envelope bool) string {
	text := fmt.Sprintf("```mission\nfence.wall-clock-hours=%d\nfence.cycles=%d\nfence.jobs=%d\nfence.concurrency=%d\nfence.job-cap-min=%d\n",
		wallHours, cycles, jobs, concurrency, p7JobCapMin)
	if envelope {
		text += "envelope.dispatch-allow=fake:fake-escalated,fake:fake-model\n"
	}
	text += "```\n"
	if envelope {
		text += "\n```mission-seal\nsealed.version=1\n```\n"
		text += fmt.Sprintf("\nApproval: name=Fixture-Human; date=2026-08-06; contract-sha256=%s\n", contract.CanonicalContractHash(text))
	}
	return text
}

// writeMission lays down one mission: its contract, fence counters pinned to
// the contract's raw bytes (the runner-owned pin), and a live lease held by
// this test process, whose argv carries the instance tag.
func (b *bed) writeMission(mission, contractText string, fences map[string]any) {
	b.t.Helper()
	contractPath := b.writeFile("plans/mission-"+mission+".contract.md", contractText)
	raw, err := os.ReadFile(contractPath)
	if err != nil {
		b.t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	now := b.doubles.Clock.Now().UTC().Format("2006-01-02T15:04:05Z")
	counters := map[string]any{"schemaVersion": 1, "missionId": mission, "startedAt": now, "cycles": 0,
		"reservations": map[string]any{}, "approvedContractSha256": hex.EncodeToString(sum[:])}
	for key, value := range fences {
		counters[key] = value
	}
	encoded, _ := json.Marshal(counters)
	b.writeFile("artifacts/agents/missions/"+mission+"/fences.json", string(encoded)+"\n")
	pgid, err := unix.Getpgid(0)
	if err != nil {
		b.t.Fatal(err)
	}
	lease, _ := json.Marshal(map[string]any{"missionId": mission, "pid": os.Getpid(), "pgid": pgid,
		"instanceTag": os.Args[0], "startedAt": now, "renewedAt": now})
	b.writeFile("artifacts/agents/missions/"+mission+"/lease.json", string(lease)+"\n")
}

func (b *bed) missionLease(mission string) string {
	return filepath.Join(b.root, "artifacts", "agents", "missions", mission, "lease.json")
}

// publishToOrigin commits the bed's plans and pushes them to a bare origin
// whose default branch is declared, so a signed contract verifies against
// the fetched origin; the census is re-attested for the new head.
func (b *bed) publishToOrigin() {
	b.t.Helper()
	origin := filepath.Join(b.t.TempDir(), "origin.git")
	if output, err := exec.Command("git", "init", "-q", "--bare", "-b", "main", origin).CombinedOutput(); err != nil {
		b.t.Fatalf("git init origin: %v: %s", err, output)
	}
	b.git("remote", "add", "origin", origin)
	b.git("add", "plans")
	b.git("commit", "-qm", "sign the mission contracts")
	b.git("push", "-qu", "origin", "main")
	b.git("remote", "set-head", "origin", "-a")
	b.armSupervision()
}

func (b *bed) missionEnv(turn string) delegation.Env {
	env := b.dispatchEnv("fresh")
	env.MissionTurn = turn
	return env
}

func (b *bed) readText(relative string) string {
	b.t.Helper()
	content, err := os.ReadFile(filepath.Join(b.root, relative))
	if err != nil {
		b.t.Fatal(err)
	}
	return string(content)
}

func (b *bed) fenceReservations(mission string) map[string]any {
	b.t.Helper()
	var fences struct {
		Reservations map[string]any `json:"reservations"`
	}
	if err := json.Unmarshal([]byte(b.readText("artifacts/agents/missions/"+mission+"/fences.json")), &fences); err != nil {
		b.t.Fatal(err)
	}
	return fences.Reservations
}

// envelope-model-override, mission-explicit, mission-inherited, mission-cap
// and the mission provenance asserts (fixture lines 4575-4700, 4827-4830):
// a mission-scoped dispatch binds its turn and stream into the record and
// its mission into the packet, reserves its fence slot, admits a pair only
// the signed dispatch envelope allows, and refuses a cap above the signed
// per-job cap as an authorization refusal that raises no fence ask. An
// unstamped dispatch in the same checkout gains no mission authority.
func TestP7MissionScopedDispatchIntegration(t *testing.T) {
	t.Parallel()
	b := newDispatchBed(t)
	// The investigator is assigned to main; a runtime override resolves its
	// implied model from the role's own key.
	conf := b.readText("metasystem.conf")
	conf = strings.Replace(conf, "role.investigator.runtime=fake\n", "", 1)
	b.writeFile("metasystem.conf", conf+"role.investigator.runtime=main\nrole.investigator.model.fake=fake-implied-model\n")
	b.writeMission("mission-alpha", p7MissionContract(2, 10, 20, 8, true), nil)
	b.publishToOrigin()
	brief := b.brief("brief.md", "implement", "Do the thing.")
	dispatchArgs := func(job string, extra ...string) []string {
		return append([]string{"dispatch", "--role", "implementer", "--brief", brief, "--job-id", job}, extra...)
	}

	// Explicit mission scope.
	result := b.runEnv(b.missionEnv("mission-alpha-t1-fixture"), dispatchArgs("mission-explicit", "--mission", "mission-alpha", "--stream", "main")...)
	requireExit(t, result, 0, b.stderr.String())
	// Inherited mission scope.
	env := b.missionEnv("mission-alpha-t1-fixture")
	env.MissionID, env.MissionLease = "mission-alpha", b.missionLease("mission-alpha")
	result = b.runEnv(env, dispatchArgs("mission-inherited", "--stream", "main")...)
	requireExit(t, result, 0, b.stderr.String())
	for _, job := range []string{"mission-explicit", "mission-inherited"} {
		record := b.record(job)
		if record["mission"] != "mission-alpha" || record["turnId"] != "mission-alpha-t1-fixture" || record["stream"] != "main" {
			t.Fatalf("%s lost its mission provenance: mission=%v turnId=%v stream=%v", job, record["mission"], record["turnId"], record["stream"])
		}
		prompt := b.readText("artifacts/agents/" + job + "/rounds/1/prompt.md")
		if !regexp.MustCompile(`(?m)^Mission: mission-alpha$`).MatchString(prompt) {
			t.Fatalf("%s prompt lost its mission line", job)
		}
		if _, reserved := b.fenceReservations("mission-alpha")[job]; !reserved {
			t.Fatalf("%s holds no mission fence reservation", job)
		}
	}

	// The signed envelope admits the escalated pair without a TTY approval.
	result = b.runEnv(b.missionEnv("mission-alpha-t1-fixture"),
		dispatchArgs("envelope-model-override", "--model", "fake-escalated", "--mission", "mission-alpha", "--stream", "main")...)
	requireExit(t, result, 0, b.stderr.String())
	if record := b.record("envelope-model-override"); record["requestedModel"] != "fake-escalated" || record["escalationApproval"] != nil {
		t.Fatalf("the envelope-admitted override: requestedModel=%v escalationApproval=%v", record["requestedModel"], record["escalationApproval"])
	}

	// A runtime override away from a main-assigned role: the implied pair
	// outside the envelope refuses naming the envelope entry to add; the
	// same override with an enveloped model is admitted.
	investigator := b.brief("investigator.md", "take-a-step-back", "Look into it.")
	result = b.runEnv(b.missionEnv("mission-alpha-t1-fixture"), "dispatch", "--role", "investigator", "--brief", investigator,
		"--runtime", "fake", "--job-id", "envelope-runtime-implied", "--mission", "mission-alpha", "--stream", "main")
	requireExit(t, result, 1, b.stderr.String())
	if !strings.Contains(b.stderr.String(), "add fake:fake-implied-model to a signed envelope.dispatch-allow") {
		t.Fatalf("stderr %q", b.stderr.String())
	}
	if _, err := os.Stat(b.recordPath("envelope-runtime-implied")); !os.IsNotExist(err) {
		t.Fatalf("the refused runtime override left a record: %v", err)
	}
	result = b.runEnv(b.missionEnv("mission-alpha-t1-fixture"), "dispatch", "--role", "investigator", "--brief", investigator,
		"--runtime", "fake", "--model", "fake-model", "--job-id", "envelope-runtime-override", "--mission", "mission-alpha", "--stream", "main")
	requireExit(t, result, 0, b.stderr.String())
	if record := b.record("envelope-runtime-override"); record["runtime"] != "fake" || record["mission"] != "mission-alpha" {
		t.Fatalf("the envelope-admitted runtime override: runtime=%v mission=%v", record["runtime"], record["mission"])
	}

	// Above the signed per-job cap: an authorization refusal, no fence ask.
	result = b.runEnv(b.missionEnv("mission-alpha-t1-fixture"),
		dispatchArgs("mission-cap", "--mission", "mission-alpha", "--stream", "main", "--cap-min", fmt.Sprint(p7JobCapMin+1))...)
	requireExit(t, result, 1, b.stderr.String())
	if !strings.Contains(b.stderr.String(), fmt.Sprintf("mission fence refused requested cap %dm above signed fence.job-cap-min=%dm", p7JobCapMin+1, p7JobCapMin)) {
		t.Fatalf("stderr %q", b.stderr.String())
	}
	if _, err := os.Stat(filepath.Join(b.root, "artifacts/agents/missions/mission-alpha/asks/fence-bound.json")); !os.IsNotExist(err) {
		t.Fatalf("an authorization refusal wrote a fence-bound ask: %v", err)
	}
	if _, reserved := b.fenceReservations("mission-alpha")["mission-cap"]; reserved {
		t.Fatal("the refused over-cap dispatch kept a fence reservation")
	}

	// A mission dispatch outside a runner turn, or without a stream, refuses.
	result = b.runEnv(b.dispatchEnv("fresh"), dispatchArgs("mission-no-turn", "--mission", "mission-alpha", "--stream", "main")...)
	requireExit(t, result, 2, b.stderr.String())
	if !strings.Contains(b.stderr.String(), "mission dispatch requires a runner turn") {
		t.Fatalf("stderr %q", b.stderr.String())
	}

	// An unstamped dispatch gains no mission authority.
	result = b.runEnv(b.dispatchEnv("fresh"), dispatchArgs("happy")...)
	requireExit(t, result, 0, b.stderr.String())
	if record := b.record("happy"); record["mission"] != nil {
		t.Fatalf("unstamped interactive dispatch gained mission authority: %v", record["mission"])
	}
}

// fence-wall, fence-cycles, fence-jobs and fence-concurrency (fixture lines
// 4702-4761): a mission whose fence is spent refuses the job naming the
// fence, writes the batched ask naming it, and leaves no job and no
// reservation behind.
func TestP7MissionFenceRefusalsRaiseTheBatchedAskIntegration(t *testing.T) {
	t.Parallel()
	b := newDispatchBed(t)
	now := b.doubles.Clock.Now().UTC().Format("2006-01-02T15:04:05Z")
	past := map[string]any{"reservedAt": "2000-01-01T00:00:00Z", "capMin": 30}
	cases := []struct {
		mission, reason string
		contract        string
		fences          map[string]any
	}{
		{"mission-wall", "wall-clock-hours", p7MissionContract(1, 10, 10, 2, false), map[string]any{"startedAt": "2000-01-01T00:00:00Z"}},
		{"mission-cycles", "cycles", p7MissionContract(2, 1, 10, 2, false), map[string]any{"startedAt": now, "cycles": 1}},
		{"mission-jobs", "jobs", p7MissionContract(2, 10, 1, 2, false), map[string]any{"reservations": map[string]any{"prior": past}}},
		{"mission-concurrency", "concurrency", p7MissionContract(2, 10, 10, 1, false), map[string]any{"reservations": map[string]any{"active": past}}},
	}
	for _, tc := range cases {
		b.writeMission(tc.mission, tc.contract, tc.fences)
	}
	brief := b.brief("brief.md", "implement", "Do the thing.")
	for _, tc := range cases {
		job := "fence-" + tc.reason
		result := b.runEnv(b.missionEnv(tc.mission+"-t1-fixture"),
			"dispatch", "--role", "implementer", "--brief", brief, "--job-id", job, "--mission", tc.mission, "--stream", "main", "--wait")
		requireExit(t, result, 1, b.stderr.String())
		if !strings.Contains(b.stderr.String(), "mission fence refused job ("+tc.reason+")") {
			t.Fatalf("%s: stderr %q", tc.mission, b.stderr.String())
		}
		ask, err := os.ReadFile(filepath.Join(b.root, "artifacts/agents/missions", tc.mission, "asks/fence-bound.json"))
		if err != nil || !strings.Contains(string(ask), "`"+tc.reason+"`") {
			t.Fatalf("%s: the refusal wrote no batched ask naming %s: %v %s", tc.mission, tc.reason, err, ask)
		}
		if _, reserved := b.fenceReservations(tc.mission)[job]; reserved {
			t.Fatalf("%s: the refused job kept a fence reservation", tc.mission)
		}
		if _, err := os.Stat(b.recordPath(job)); !os.IsNotExist(err) {
			t.Fatalf("%s: the refused job left a record: %v", tc.mission, err)
		}
		if len(b.calls("host.WaitJob")) != 0 {
			t.Fatalf("%s: a refused dispatch waited", tc.mission)
		}
	}
}

// mission-timeout (fixture lines 4763-4801): a mission job's waiter that
// reports the budget-cap timeout maps to the dispatcher's exit 4, and the
// post-wait reap aggregates the terminal job into the mission's usage.
func TestP7MissionWaiterTimeoutExitsFourIntegration(t *testing.T) {
	t.Parallel()
	b := newDispatchBed(t)
	b.writeMission("mission-timeout", p7MissionContract(2, 10, 10, 2, false), nil)
	brief := b.brief("brief.md", "implement", "Hold until the cap.")
	b.doubles.Host.WaitFunc = func(root, job string, _ int64) delegation.WaitOutcome {
		patch := filepath.Join(b.t.TempDir(), "timeout.json")
		if err := os.WriteFile(patch, []byte(`{"error":"budget-cap","phase":"supervision"}`), 0o600); err != nil {
			t.Error(err)
		}
		if _, err := dispatch.RecordCAS(root, job, "running", "timeout", patch); err != nil {
			t.Errorf("the waiter's timeout verdict: %v", err)
		}
		return delegation.WaitOutcome{Code: 2}
	}
	result := b.runEnv(b.missionEnv("mission-timeout-t1-fixture"),
		"dispatch", "--role", "implementer", "--brief", brief, "--job-id", "mission-timeout-job", "--mission", "mission-timeout", "--stream", "main", "--wait")
	requireExit(t, result, 4, b.stderr.String())
	if record := b.record("mission-timeout-job"); record["status"] != "timeout" || record["mission"] != "mission-timeout" {
		t.Fatalf("record status=%v mission=%v", record["status"], record["mission"])
	}
	if _, err := os.Stat(filepath.Join(b.root, "artifacts/agents/missions/mission-timeout/usage.json")); err != nil {
		t.Fatalf("the post-wait reap did not aggregate the mission usage: %v", err)
	}
}

// escalation-approved (fixture lines 4551-4573): an approved escalation
// records who approved it, when, and the facts the prompt displayed.
func TestP7ApprovedEscalationRecordsTheApprovalIntegration(t *testing.T) {
	t.Parallel()
	b := newDispatchBed(t)
	brief := b.brief("brief.md", "implement", "Do the thing.")
	b.stderr.Reset()
	env := b.dispatchEnv("fresh")
	result := b.life.Run(t.Context(), delegation.Request{
		Invocation: delegation.Invocation{CallerPid: int64(os.Getpid())},
		Env:        env, LockTag: "delegation-test-lock-tag", Stderr: &b.stderr,
		Stdin: strings.NewReader("APPROVE Fixture Human\n"), StdinTTY: true, StderrTTY: true,
	}, []string{"dispatch", "--role", "implementer", "--brief", brief, "--model", "fake-escalated", "--approve-escalation", "--job-id", "escalation-approved"})
	requireExit(t, result, 0, b.stderr.String())
	approval, _ := b.record("escalation-approved")["escalationApproval"].(map[string]any)
	if approval == nil {
		t.Fatalf("the approved escalation recorded no approval: %v", b.record("escalation-approved"))
	}
	want := map[string]string{
		"name": "Fixture Human", "rosterResolution": "fake:fake-model", "requestedPair": "fake:fake-escalated",
		"costDirection": "unranked (model tiers absent; overrides always escalate)",
	}
	for key, value := range want {
		if approval[key] != value {
			t.Fatalf("escalationApproval.%s = %v, want %q", key, approval[key], value)
		}
		if key != "name" {
			label := map[string]string{"rosterResolution": "Roster resolution", "requestedPair": "Requested pair", "costDirection": "Cost direction"}[key]
			if !strings.Contains(b.stderr.String(), label+": "+value+"\n") {
				t.Fatalf("the prompt did not display the recorded %s: %q", key, b.stderr.String())
			}
		}
	}
	approvedAt, _ := approval["approvedAt"].(string)
	if approvedAt != b.doubles.Clock.Now().UTC().Format("2006-01-02T15:04:05Z") {
		t.Fatalf("escalation approval stamp %q", approvedAt)
	}
}

// capped-old (fixture lines 4393-4414): under a runtime profile without
// resume, a round cut off at its cap continues in the same worktree through
// a fresh-context successor told what happened: the prior brief and the
// prior-worktree paragraph, never a prior return the capped round never
// wrote, and never the killed session.
func TestP7CappedContinuationUnderAProfileWithoutResumeIntegration(t *testing.T) {
	t.Parallel()
	b := newDispatchBed(t)
	capabilities := filepath.Join(b.root, "artifacts", "agents", "capabilities")
	if err := os.RemoveAll(capabilities); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.WriteFakeCapabilitySnapshot(capabilities, "old", 0, 20); err != nil {
		t.Fatal(err)
	}
	brief := b.brief("brief.md", "implement", "Write the marker.")
	result := b.runEnv(b.dispatchEnv("fresh"), "dispatch", "--role", "implementer", "--brief", brief, "--job-id", "capped-old", "--worktree")
	requireExit(t, result, 0, b.stderr.String())
	root := b.record("capped-old")
	workspace, _ := root["workspaceRoot"].(string)
	if root["launchMode"] != "worktree" || workspace == "" {
		t.Fatalf("the capped round did not run in a worktree: %v", root)
	}
	marker := "metasystem/old-marker.txt"
	if err := os.MkdirAll(filepath.Join(workspace, "metasystem"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, marker), []byte("work in progress\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The reaper cut the round off at its cap.
	patch := filepath.Join(t.TempDir(), "capped.json")
	if err := os.WriteFile(patch, []byte(`{"error":"budget-cap","phase":"supervision","groupDeathProvenAt":"2026-09-27T12:00:00Z"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := dispatch.RecordCAS(b.root, "capped-old", "running", "timeout", patch); err != nil {
		t.Fatal(err)
	}

	b.doubles.Host.WaitFunc = func(root, job string, _ int64) delegation.WaitOutcome {
		done := filepath.Join(b.t.TempDir(), "done.json")
		if err := os.WriteFile(done, []byte(`{"error":null,"phase":"done"}`), 0o600); err != nil {
			t.Error(err)
		}
		if _, err := dispatch.RecordCAS(root, job, "running", "completed", done); err != nil {
			t.Errorf("complete %s: %v", job, err)
		}
		return delegation.WaitOutcome{Code: 0}
	}
	message := b.writeFile("follow.md", "Working Mode: implement\n\nFinish the marker.\n")
	followEnv := b.dispatchEnv("follow-up")
	result = b.runEnv(followEnv, "follow-up", "--job", "capped-old", "--message", message, "--wait")
	requireExit(t, result, 0, b.stderr.String())

	child := b.record("capped-old-r2")
	if child["continuation"] != "after-cap" || child["resumeMode"] != "fresh-context" || child["parentJob"] != "capped-old" || child["status"] != "completed" {
		t.Fatalf("the continuation round did not record its shape: %v", child)
	}
	if child["sessionId"] == b.record("capped-old")["sessionId"] {
		t.Fatal("the continuation resumed the killed session")
	}
	prompt := b.readText("artifacts/agents/capped-old/rounds/2/prompt.md")
	for _, want := range []string{"# Prior Worktree", "cut off at its", marker, "# Prior Brief"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("the continuation packet lacks %q", want)
		}
	}
	if strings.Contains(prompt, "# Prior Return") {
		t.Fatal("the continuation packet carried a prior return the capped round never wrote")
	}
	sources, _ := json.Marshal(child["composition"].(map[string]any)["sources"])
	if !strings.Contains(string(sources), `"source":"engine:prior-worktree"`) || strings.Contains(string(sources), `"source":"engine:prior-return"`) {
		t.Fatalf("the continuation composition lost its prior-worktree provenance: %s", sources)
	}
	if _, err := os.Stat(filepath.Join(b.root, "artifacts/agents/capped-old/rounds/2/prior-worktree.md")); err != nil {
		t.Fatalf("the continuation paragraph was not kept in the round directory: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workspace, marker)); err != nil {
		t.Fatalf("the continuation lost the capped round's work: %v", err)
	}
	newest, err := dispatch.LatestChainRecord(filepath.Join(b.root, "artifacts", "agents", "jobs"), "capped-old")
	if err != nil || filepath.Base(newest) != "capped-old-r2.json" {
		t.Fatalf("the chain's newest record is %s (%v), want the completed continuation", newest, err)
	}
	if len(b.launches) != 2 || b.launches[1].Job != "capped-old-r2" {
		t.Fatalf("launches %+v", b.launches)
	}
}

// unverified-deny and waived-deny (fixture lines 4417-4457): a snapshot that
// cannot verify a field the envelope restricts refuses the dispatch before
// any job exists, and the refusal stands (a policy refusal is never
// laundered by a fresh probe); the role's waiver of the runtime's declared
// residual admits the same dispatch.
func TestP7UnverifiedRestrictiveFieldRefusesUntilWaivedIntegration(t *testing.T) {
	t.Parallel()
	b := newDispatchBed(t)
	capabilities := filepath.Join(b.root, "artifacts", "agents", "capabilities")
	if err := os.RemoveAll(capabilities); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.WriteFakeCapabilitySnapshot(capabilities, "unverified-network", 0, 20); err != nil {
		t.Fatal(err)
	}
	brief := b.brief("brief.md", "implement", "Do the thing.")
	result := b.runEnv(b.dispatchEnv("fresh"), "dispatch", "--role", "implementer", "--brief", brief, "--permissions", "critic", "--job-id", "unverified-deny")
	requireExit(t, result, 1, b.stderr.String())
	if !strings.Contains(b.stderr.String(), "cannot enforce restrictive permission field network") {
		t.Fatalf("stderr %q", b.stderr.String())
	}
	if probes := b.calls("adapter.Probe"); len(probes) != 0 {
		t.Fatalf("a policy refusal was re-probed: %v", probes)
	}
	if _, err := os.Stat(b.recordPath("unverified-deny")); !os.IsNotExist(err) {
		t.Fatalf("the refused dispatch left a record: %v", err)
	}
	// The waived half lives with the selector (internal/capability
	// TestSelectRestrictiveFieldRefusedThenWaived): role
	// requirements are compiled into the engine, not an installation file.
}
