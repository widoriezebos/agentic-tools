package dispatch

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// writeJSONFile marshals a value into dir/name and returns the path.
func writeJSONFile(t *testing.T, dir, name string, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal %s: %v", name, err)
	}
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", name, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

func readJSONFile(t *testing.T, path string) map[string]any {
	t.Helper()
	record, err := readObject(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return record
}

// jobsDirWithChain lays down a root record, a round-2 follow-up, and an
// unrelated job.
func jobsDirWithChain(t *testing.T) string {
	t.Helper()
	jobs := t.TempDir()
	writeJSONFile(t, jobs, "root-a.json", map[string]any{
		"jobId": "root-a", "round": 1, "parentJob": nil, "status": "completed",
		"runtime": "fake", "usage": map[string]any{"inputTokens": 10, "outputTokens": 2},
	})
	writeJSONFile(t, jobs, "root-a-r2.json", map[string]any{
		"jobId": "root-a-r2", "round": 2, "parentJob": "root-a", "status": "failed",
		"runtime": "fake", "usage": map[string]any{"inputTokens": 5,
			"cost": map[string]any{"amount": 1.5, "currency": "USD"}},
	})
	writeJSONFile(t, jobs, "other.json", map[string]any{
		"jobId": "other", "round": 1, "parentJob": nil, "status": "running",
	})
	return jobs
}

func TestLatestChainRecordPicksHighestRound(t *testing.T) {
	jobs := jobsDirWithChain(t)
	path, err := LatestChainRecord(jobs, "root-a")
	if err != nil {
		t.Fatalf("LatestChainRecord: %v", err)
	}
	if filepath.Base(path) != "root-a-r2.json" {
		t.Fatalf("latest = %s, want root-a-r2.json", path)
	}
	if _, err := LatestChainRecord(jobs, "nobody"); err == nil {
		t.Fatalf("expected a refusal for an unknown chain")
	}
}

func TestChainMemberStatusesTerminalOnly(t *testing.T) {
	jobs := jobsDirWithChain(t)
	lines, err := ChainMemberStatuses(jobs, "root-a", true)
	if err != nil {
		t.Fatalf("ChainMemberStatuses: %v", err)
	}
	want := []string{"root-a-r2|failed", "root-a|completed"}
	if strings.Join(lines, ",") != strings.Join(want, ",") {
		t.Fatalf("members = %v, want %v", lines, want)
	}
}

func TestChainUsageAggregatesAndDetectsUnchanged(t *testing.T) {
	jobs := jobsDirWithChain(t)
	patch := filepath.Join(t.TempDir(), "usage.json")
	unchanged, err := ChainUsage(jobs, "root-a", patch)
	if err != nil || unchanged {
		t.Fatalf("ChainUsage first pass: unchanged=%v err=%v", unchanged, err)
	}
	value := readJSONFile(t, patch)
	usage, _ := value["chainUsage"].(map[string]any)
	tokens := usage["tokens"].(map[string]any)["fake"].(map[string]any)
	if tokens["inputTokens"].(json.Number).String() != "15" {
		t.Fatalf("inputTokens = %v, want 15", tokens["inputTokens"])
	}
	if tokens["reasoningTokens"] != nil {
		t.Fatalf("reasoningTokens = %v, want null", tokens["reasoningTokens"])
	}
	if usage["cost"].(map[string]any)["USD"].(json.Number).String() != "1.5" {
		t.Fatalf("cost = %v, want USD 1.5", usage["cost"])
	}

	// Stamp the aggregate onto the root record; the second pass is unchanged.
	root := readJSONFile(t, filepath.Join(jobs, "root-a.json"))
	root["chainUsage"] = usage
	writeRecord(filepath.Join(jobs, "root-a.json"), root)
	unchanged, err = ChainUsage(jobs, "root-a", patch)
	if err != nil || !unchanged {
		t.Fatalf("ChainUsage second pass: unchanged=%v err=%v, want true", unchanged, err)
	}
}

func TestCustodyAddDedupesAndRefusesTerminal(t *testing.T) {
	root := t.TempDir()
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	writeJSONFile(t, jobs, "job-c.json", map[string]any{
		"jobId": "job-c", "status": "running", "instanceTag": "metasystem-job-job-c",
		"custodyProcesses": []any{map[string]any{"pid": 41, "pidStartedAt": 5, "instanceTag": "metasystem-job-job-c"}},
	})
	// The exact identity again collapses to one entry; a recycled pid with a
	// different start time is a distinct process and keeps its own entry.
	if err := custodyAddFixed(root, "job-c", 41, 5); err != nil {
		t.Fatalf("CustodyAdd exact duplicate: %v", err)
	}
	record := readJSONFile(t, filepath.Join(jobs, "job-c.json"))
	if items := record["custodyProcesses"].([]any); len(items) != 1 {
		t.Fatalf("custody entries after duplicate = %d, want 1", len(items))
	}
	if err := custodyAddFixed(root, "job-c", 41, 9); err != nil {
		t.Fatalf("CustodyAdd recycled pid: %v", err)
	}
	record = readJSONFile(t, filepath.Join(jobs, "job-c.json"))
	if items := record["custodyProcesses"].([]any); len(items) != 2 {
		t.Fatalf("custody entries after recycled pid = %d, want 2", len(items))
	}

	writeJSONFile(t, jobs, "job-done.json", map[string]any{
		"jobId": "job-done", "status": "completed", "instanceTag": "t",
	})
	err := custodyAddFixed(root, "job-done", 1, 1)
	var op *OpError
	if err == nil || !asOpError(err, &op) || op.Code != 1 || op.Message != "" {
		t.Fatalf("terminal custody add = %v, want silent exit 1", err)
	}
}

// custodyAddFixed drives the new CustodyAdd contract for legacy call
// sites: a fixed live identity at the given start second with a stable
// process group.
func custodyAddFixed(root, job string, pid, startedSec int64) error {
	exact := identity.Exact{Pid: pid, StartedAt: time.Unix(startedSec, 0)}
	return CustodyAdd(root, job, pid,
		fixedStartReader{exact: exact, state: identity.Alive},
		func(int64) (int64, error) { return 4000, nil })
}

func asOpError(err error, target **OpError) bool {
	op, ok := err.(*OpError)
	if ok {
		*target = op
	}
	return ok
}

func TestHandshakeEvalVerdicts(t *testing.T) {
	dir := t.TempDir()
	requested := map[string]any{
		"readRoots": []any{"/repo"}, "writeRoots": []any{},
		"network": "deny", "approvals": "deny", "tools": "read-only",
	}
	record := writeJSONFile(t, dir, "record.json", map[string]any{
		"jobId": "j", "permissions": map[string]any{
			"requested": requested, "effective": nil, "enforcementSnapshot": "snap.json",
		},
	})
	output := filepath.Join(dir, "out.json")

	matching := writeJSONFile(t, dir, "effective-ok.json", map[string]any{
		"readRoots": []any{"/repo"}, "writeRoots": []any{},
		"network": "deny", "approvals": "deny", "tools": "read-only",
	})
	if err := HandshakeEval(record, matching, "sess-1", "turn-1", "model-x", true, output); err != nil {
		t.Fatalf("HandshakeEval: %v", err)
	}
	result := readJSONFile(t, output)
	if result["target"] != "running" {
		t.Fatalf("target = %v, want running", result["target"])
	}
	patch := result["patch"].(map[string]any)
	if patch["sessionId"] != "sess-1" || patch["envelopeTurn"] != "turn-1" || patch["error"] != nil {
		t.Fatalf("running patch = %v", patch)
	}
	// turnId is immutable dispatch provenance; the handshake patch must
	// never name it (host-implementer wall).
	if _, present := patch["turnId"]; present {
		t.Fatalf("handshake patch names immutable turnId: %v", patch)
	}

	wider := writeJSONFile(t, dir, "effective-wide.json", map[string]any{
		"network": "allow", "readRoots": []any{"/repo", "/etc"},
	})
	if err := HandshakeEval(record, wider, "sess-1", "", "model-x", true, output); err != nil {
		t.Fatalf("HandshakeEval wide: %v", err)
	}
	result = readJSONFile(t, output)
	patch = result["patch"].(map[string]any)
	if result["target"] != "failed" || patch["error"] != "permissions_mismatch:network,readRoots" {
		t.Fatalf("wide verdict = %v / %v", result["target"], patch["error"])
	}

	silent := writeJSONFile(t, dir, "effective-silent.json", map[string]any{"network": "deny"})
	if err := HandshakeEval(record, silent, "", "", "model-x", true, output); err != nil {
		t.Fatalf("HandshakeEval silent: %v", err)
	}
	result = readJSONFile(t, output)
	patch = result["patch"].(map[string]any)
	if result["target"] != "failed" || patch["error"] != "handshake_missing_session_id" {
		t.Fatalf("missing-session verdict = %v / %v", result["target"], patch["error"])
	}
	if _, present := patch["sessionId"]; present {
		t.Fatalf("missing-session patch must not carry a sessionId")
	}
}

func TestComputeReapFacts(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC()
	iso := func(at time.Time) string { return at.Format("2006-01-02T15:04:05Z") }

	abandoned := writeJSONFile(t, dir, "setup-old.json", map[string]any{
		"status": "pending-setup", "createdAt": iso(now.Add(-11 * time.Minute)),
	})
	facts, err := ComputeReapFacts(abandoned, 2, now)
	if err != nil || !facts.SetupAbandoned {
		t.Fatalf("old pending-setup facts = %+v err=%v", facts, err)
	}
	fresh := writeJSONFile(t, dir, "setup-new.json", map[string]any{
		"status": "pending-setup", "createdAt": iso(now.Add(-1 * time.Minute)),
	})
	if facts, _ = ComputeReapFacts(fresh, 2, now); facts.SetupAbandoned {
		t.Fatalf("fresh pending-setup marked abandoned")
	}

	// Inside the launch-stamped handshake deadline the job is deferred; past
	// it (plus grace) it is not.
	waiting := writeJSONFile(t, dir, "waiting.json", map[string]any{
		"status": "pending", "sessionEstablishedTimeoutSec": 30,
		"handshakeDeadline": now.Unix() + 10, "startedAt": iso(now),
	})
	if facts, _ = ComputeReapFacts(waiting, 2, now); !facts.HandshakeWaiting {
		t.Fatalf("job inside its handshake deadline is not waiting")
	}
	overdue := writeJSONFile(t, dir, "overdue.json", map[string]any{
		"status": "pending", "sessionEstablishedTimeoutSec": 30,
		"handshakeDeadline": now.Unix() - 5, "startedAt": iso(now),
	})
	if facts, _ = ComputeReapFacts(overdue, 2, now); facts.HandshakeWaiting {
		t.Fatalf("job past its handshake deadline still waiting")
	}

	// A running job with a session is out of its handshake, and its budget
	// verdict matches the supervision reaper's.
	expired := writeJSONFile(t, dir, "expired.json", map[string]any{
		"status": "running", "sessionId": "s", "sessionEstablishedTimeoutSec": 30,
		"startedAt": iso(now.Add(-2 * time.Hour)), "capMin": 60,
	})
	facts, err = ComputeReapFacts(expired, 2, now)
	if err != nil || facts.HandshakeWaiting || !facts.BudgetExpired {
		t.Fatalf("expired running facts = %+v err=%v", facts, err)
	}
}

func TestExpandPermissions(t *testing.T) {
	dir := t.TempDir()
	repo := t.TempDir()
	// A writable workspace is a REAL git worktree: the
	// expansion derives the commit's git-metadata roots from it.
	gitCmd := func(workdir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", workdir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	gitCmd(repo, "init", "-q", "-b", "main")
	os.WriteFile(filepath.Join(repo, "f.txt"), []byte("x\n"), 0o644)
	gitCmd(repo, "add", ".")
	gitCmd(repo, "-c", "user.name=t", "-c", "user.email=t@x", "commit", "-qm", "base")
	workspace := filepath.Join(repo, "wt")
	gitCmd(repo, "worktree", "add", "-q", "-b", "agent/expand-test", workspace)
	envelope := writeJSONFile(t, dir, "envelope.json", map[string]any{
		"readRoots": []any{".", "docs"}, "writeRoots": []any{"<worktree>"},
		"network": "allow", "approvals": "deny", "tools": "read-only",
	})
	output := filepath.Join(dir, "expanded.json")
	if err := ExpandPermissions(envelope, repo, workspace, true, "builder", "deny", output); err != nil {
		t.Fatalf("ExpandPermissions: %v", err)
	}
	expanded := readJSONFile(t, output)
	if expanded["preset"] != "builder" || expanded["network"] != "deny" {
		t.Fatalf("expanded = %v", expanded)
	}
	reads := expanded["readRoots"].([]any)
	if reads[0] != resolvePath(repo) || reads[1] != resolvePath(filepath.Join(repo, "docs")) {
		t.Fatalf("readRoots = %v", reads)
	}
	if expanded["writeRoots"].([]any)[0] != resolvePath(workspace) {
		t.Fatalf("writeRoots = %v", expanded["writeRoots"])
	}

	if err := ExpandPermissions(envelope, repo, workspace, false, "builder", "", output); err == nil ||
		!strings.Contains(err.Error(), "require --worktree") {
		t.Fatalf("writable without worktree = %v", err)
	}
	escape := writeJSONFile(t, dir, "escape.json", map[string]any{
		"readRoots": []any{}, "writeRoots": []any{"/"},
		"network": "deny", "approvals": "deny", "tools": "read-only",
	})
	if err := ExpandPermissions(escape, repo, workspace, true, "custom", "", output); err == nil ||
		!strings.Contains(err.Error(), "escapes the job worktree") {
		t.Fatalf("escaping write root = %v", err)
	}
}

func TestBriefMode(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.md")
	os.WriteFile(good, []byte("Title\nWorking Mode: deep-work\nBody\n"), 0o644)
	mode, err := BriefMode(good)
	if err != nil || mode != "deep-work" {
		t.Fatalf("BriefMode = %q, %v", mode, err)
	}
	for name, content := range map[string]string{
		"none.md":        "Title\n",
		"double.md":      "Working Mode: a\nWorking Mode: b\n",
		"placeholder.md": "Working Mode: <fill me>\n",
	} {
		path := filepath.Join(dir, name)
		os.WriteFile(path, []byte(content), 0o644)
		if _, err := BriefMode(path); err == nil {
			t.Fatalf("%s: expected a refusal", name)
		}
	}
}

func TestBuildRecordsCohereWithLifecycle(t *testing.T) {
	root := t.TempDir()
	tmp := t.TempDir()
	workspace := filepath.Join(tmp, "ws")
	os.MkdirAll(workspace, 0o755)
	for _, args := range [][]string{
		{"init", "-q"}, {"config", "user.email", "t@example.invalid"}, {"config", "user.name", "t"},
		{"commit", "-q", "--allow-empty", "-m", "base"},
	} {
		cmd := exec.Command("git", append([]string{"-C", workspace}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	capResolution := filepath.Join(tmp, "cap.json")
	if err := WriteCapResolution(capResolution, 120, "built-in", "default"); err != nil {
		t.Fatalf("WriteCapResolution: %v", err)
	}
	permissions := writeJSONFile(t, tmp, "perm.json", map[string]any{
		"preset": "none", "readRoots": []any{}, "writeRoots": []any{},
		"network": "deny", "approvals": "deny", "tools": "read-only",
	})

	setup := filepath.Join(tmp, "setup.json")
	if err := BuildSetup(setup, "job-b", "implementer", "", "main-1", "7", "", 0, capResolution, "", ""); err != nil {
		t.Fatalf("BuildSetup: %v", err)
	}
	setupRecord := readJSONFile(t, setup)
	if setupRecord["capMin"].(json.Number).String() != "120" {
		t.Fatalf("published reservation husk does not carry its final capMin: %v", setupRecord["capMin"])
	}
	if err := RecordCreate(root, "job-b", setup); err != nil {
		t.Fatalf("RecordCreate from built setup: %v", err)
	}

	recordPath := filepath.Join(tmp, "record.json")
	err := BuildRecord(BuildRecordParams{
		Output: recordPath, Job: "job-b", Role: "implementer", Runtime: "fake",
		Workspace: workspace, CapResolution: capResolution, Model: "m1",
		Snapshot: "artifacts/agents/capabilities/x.json", InputBytes: 12, InputHash: "h",
		Permissions: permissions, Fallbacks: "[]", Signal: true, HandshakeBudget: 20,
		MainID: "main-1", ClaimEpoch: "7",
		LaunchMode: LaunchModeSharedCheckout, OutputStream: "/tmp/out-stream.jsonl",
	})
	if err != nil {
		t.Fatalf("BuildRecord: %v", err)
	}
	if err := RecordSetup(root, "job-b", recordPath); err != nil {
		t.Fatalf("RecordSetup from built record: %v", err)
	}
	record := readJSONFile(t, filepath.Join(root, "artifacts", "agents", "jobs", "job-b.json"))
	if record["capMin"].(json.Number).String() != "120" || record["baseSha"] == "" ||
		record["operationId"] != "job-b" || record["goalId"] != nil || record["goalRevision"] != nil {
		t.Fatalf("built record identity = capMin %v baseSha %v", record["capMin"], record["baseSha"])
	}

	// The follow-up round inherits the parent identity and resumes its session.
	parent := readJSONFile(t, filepath.Join(root, "artifacts", "agents", "jobs", "job-b.json"))
	parent["sessionId"] = "sess-9"
	parentPath := filepath.Join(tmp, "parent.json")
	writeRecord(parentPath, parent)
	followPath := filepath.Join(tmp, "follow.json")
	err = BuildFollowRecord(BuildFollowRecordParams{
		Output: followPath, Parent: parentPath, Job: "job-b-r2", Round: 2,
		ParentJob: "job-b", Snapshot: "artifacts/agents/capabilities/x.json",
		Fallbacks: "[]", Signal: true, HandshakeBudget: 20, ResumeMode: "resumed",
		InputBytes: 3, InputHash: "h2", MainID: "main-1", ClaimEpoch: "7",
		CapResolution: capResolution,
		LaunchMode:    LaunchModeSharedCheckout, OutputStream: "/tmp/out-stream.jsonl",
	})
	if err != nil {
		t.Fatalf("BuildFollowRecord: %v", err)
	}
	follow := readJSONFile(t, followPath)
	if follow["sessionId"] != "sess-9" || follow["parentJob"] != "job-b" || follow["resumeMode"] != "resumed" {
		t.Fatalf("follow record = sessionId %v parentJob %v resumeMode %v",
			follow["sessionId"], follow["parentJob"], follow["resumeMode"])
	}
}

// Mission provenance is complete-or-refused at build, bound from the
// mission's own fences, and a follow-up refuses when the mission has been
// re-provisioned under a new incarnation.
func TestMissionProvenanceTuple(t *testing.T) {
	root := t.TempDir()
	tmp := t.TempDir()
	workspace := filepath.Join(tmp, "ws")
	os.MkdirAll(workspace, 0o755)
	for _, args := range [][]string{
		{"init", "-q"}, {"config", "user.email", "t@example.invalid"}, {"config", "user.name", "t"},
		{"commit", "-q", "--allow-empty", "-m", "base"},
	} {
		cmd := exec.Command("git", append([]string{"-C", workspace}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	capResolution := filepath.Join(tmp, "cap.json")
	if err := WriteCapResolution(capResolution, 120, "built-in", "default"); err != nil {
		t.Fatal(err)
	}
	permissions := writeJSONFile(t, tmp, "perm.json", map[string]any{
		"preset": "none", "readRoots": []any{}, "writeRoots": []any{},
		"network": "deny", "approvals": "deny", "tools": "read-only",
	})
	incarnationA := strings.Repeat("a", 64)
	fencesDir := filepath.Join(root, "artifacts", "agents", "missions", "m-one")
	os.MkdirAll(fencesDir, 0o755)
	writeFences := func(digest string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(fencesDir, "fences.json"),
			[]byte(`{"schemaVersion":1,"missionId":"m-one","approvedContractSha256":"`+digest+`"}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeFences(incarnationA)

	base := BuildRecordParams{
		Output: filepath.Join(tmp, "r.json"), Job: "j1", Role: "implementer",
		Runtime: "fake", Workspace: workspace, CapResolution: capResolution,
		Permissions: permissions, Fallbacks: "[]", Root: root,
		LaunchMode: LaunchModeSharedCheckout, OutputStream: "/tmp/out-stream.jsonl",
	}

	partial := base
	partial.Mission = "m-one"
	partial.Stream = "main"
	if err := BuildRecord(partial); err == nil {
		t.Fatal("mission dispatch without a turn built a record")
	}
	partial.MissionTurn = "m-one-t1"
	partial.Stream = ""
	if err := BuildRecord(partial); err == nil {
		t.Fatal("mission dispatch without a stream built a record")
	}

	complete := base
	complete.Mission = "m-one"
	complete.MissionTurn = "m-one-t1"
	complete.Stream = "main"
	if err := BuildRecord(complete); err != nil {
		t.Fatalf("complete tuple refused: %v", err)
	}
	record := readJSONFile(t, complete.Output)
	if record["missionIncarnation"] != incarnationA || record["stream"] != "main" || record["turnId"] != "m-one-t1" {
		t.Fatalf("provenance not bound: %v %v %v", record["missionIncarnation"], record["stream"], record["turnId"])
	}

	// Follow-up under the same incarnation, inside a turn, builds; without
	// a turn it refuses (F9); after a re-provision (same mission id, new
	// approved digest) it refuses (F8); and a pre-wall MISSION chain (no
	// recorded incarnation) refuses by name while a pre-wall NON-mission
	// chain stays followable (F11).
	parentPath := filepath.Join(tmp, "parent.json")
	writeRecord(parentPath, record)
	follow := BuildFollowRecordParams{
		Output: filepath.Join(tmp, "f.json"), Parent: parentPath, Job: "j1-r2",
		Round: 2, ParentJob: "j1", Fallbacks: "[]", ResumeMode: "fresh-context",
		CapResolution: capResolution, Root: root, MissionTurn: "m-one-t2",
		LaunchMode: LaunchModeSharedCheckout, OutputStream: "/tmp/out-stream.jsonl",
	}
	if err := BuildFollowRecord(follow); err != nil {
		t.Fatalf("same-incarnation follow-up refused: %v", err)
	}
	noTurn := follow
	noTurn.MissionTurn = ""
	if err := BuildFollowRecord(noTurn); err == nil {
		t.Fatal("mission follow-up without a turn built a record")
	}
	writeFences(strings.Repeat("b", 64))
	if err := BuildFollowRecord(follow); err == nil {
		t.Fatal("follow-up crossed a mission re-provision")
	}

	legacyMission := map[string]any{}
	for k, v := range record {
		legacyMission[k] = v
	}
	delete(legacyMission, "missionIncarnation")
	delete(legacyMission, "stream")
	legacyPath := filepath.Join(tmp, "legacy-mission.json")
	writeRecord(legacyPath, legacyMission)
	legacyFollow := follow
	legacyFollow.Parent = legacyPath
	if err := BuildFollowRecord(legacyFollow); err == nil || !strings.Contains(err.Error(), "predates the host-implementer wall") {
		t.Fatalf("pre-wall mission chain not refused by name: %v", err)
	}

	// The one owner classifies the two failures distinctly on the public
	// path too: an ABSENT incarnation key is
	// "predates the wall"; a present-but-different value is "re-provisioned".
	if err := VerifyChainIncarnation(root, "m-one", map[string]any{}); err == nil || !strings.Contains(err.Error(), "predates the host-implementer wall") {
		t.Fatalf("absent incarnation key misclassified: %v", err)
	}
	if err := VerifyChainIncarnation(root, "m-one", map[string]any{"missionIncarnation": incarnationA}); err == nil || !strings.Contains(err.Error(), "re-provisioned") {
		t.Fatalf("stale incarnation misclassified: %v", err)
	}

	legacyPlain := map[string]any{}
	for k, v := range legacyMission {
		legacyPlain[k] = v
	}
	legacyPlain["mission"] = nil
	legacyPlain["turnId"] = nil
	plainPath := filepath.Join(tmp, "legacy-plain.json")
	writeRecord(plainPath, legacyPlain)
	plainFollow := follow
	plainFollow.Parent = plainPath
	plainFollow.MissionTurn = ""
	if err := BuildFollowRecord(plainFollow); err != nil {
		t.Fatalf("pre-wall non-mission chain became unfollowable: %v", err)
	}
}

func TestCensusFreshVerdicts(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	statePath := writeJSONFile(t, dir, "state.json", map[string]any{"generation": 3})
	stateBytes, _ := os.ReadFile(statePath)
	digest := sha256.Sum256(stateBytes)
	base := map[string]any{
		"schemaVersion": 2, "writer": "watch-background-jobs.sh", "verdict": "SUCCESS",
		"completedAtEpoch": now.Unix() - 5, "intervalSec": 30, "fingerprint": "f",
		"counts": map[string]any{}, "inventory": []any{}, "diagnostics": []any{}, "errors": []any{},
		"generation": 3, "stateDigest": hex.EncodeToString(digest[:]),
	}
	verdict := writeJSONFile(t, dir, "census.json", base)
	if err := CensusFresh(verdict, statePath, "arm.sh", "/repo", "", now); err != nil {
		t.Fatalf("fresh census refused: %v", err)
	}
	// The fingerprint half of the gate: matching
	// expectation passes, anything else refuses with the one verdict text.
	if err := CensusFresh(verdict, statePath, "arm.sh", "/repo", "f", now); err != nil {
		t.Fatalf("matching fingerprint refused: %v", err)
	}
	if err := CensusFresh(verdict, statePath, "arm.sh", "/repo", "other", now); err == nil ||
		!strings.Contains(err.Error(), "census fingerprint does not match the armed code") {
		t.Fatalf("fingerprint mismatch = %v", err)
	}

	stale := map[string]any{}
	for k, v := range base {
		stale[k] = v
	}
	stale["completedAtEpoch"] = now.Unix() - 500
	verdict = writeJSONFile(t, dir, "census-stale.json", stale)
	err := CensusFresh(verdict, statePath, "arm.sh", "/repo", "", now)
	if err == nil || !strings.Contains(err.Error(), "census verdict is stale") {
		t.Fatalf("stale census = %v", err)
	}

	mismatch := map[string]any{}
	for k, v := range base {
		mismatch[k] = v
	}
	mismatch["generation"] = 2
	verdict = writeJSONFile(t, dir, "census-gen.json", mismatch)
	err = CensusFresh(verdict, statePath, "arm.sh", "/repo", "", now)
	if err == nil || !strings.Contains(err.Error(), "censusGeneration=2 armedGeneration=3") {
		t.Fatalf("generation mismatch = %v", err)
	}
}

func TestWatcherCeiling(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	heartbeat := writeJSONFile(t, dir, "hb.json", map[string]any{
		"pid": 42, "pidStartedAt": 100, "instanceTag": "w-1",
		"loadedCapMin": 240, "observedAtEpoch": now.Unix() - 3,
	})
	state := writeJSONFile(t, dir, "state.json", map[string]any{
		"intervalSec": 30,
		"components": map[string]any{"watcher": map[string]any{
			"pid": 42, "pidStartedAt": 100, "instanceTag": "w-1", "heartbeat": heartbeat,
		}},
	})
	ceiling, err := WatcherCeiling(state, now)
	if err != nil || ceiling != 240 {
		t.Fatalf("WatcherCeiling = %d, %v", ceiling, err)
	}

	writeJSONFile(t, dir, "hb.json", map[string]any{
		"pid": 43, "pidStartedAt": 100, "instanceTag": "w-1",
		"loadedCapMin": 240, "observedAtEpoch": now.Unix(),
	})
	if _, err := WatcherCeiling(state, now); err == nil ||
		!strings.Contains(err.Error(), "does not match the armed watcher") {
		t.Fatalf("identity mismatch = %v", err)
	}
}

// mirrorFixture builds a repo with one terminal single-record chain plus its
// payload, returning (repoRoot, evidence, job).
func mirrorFixture(t *testing.T) (string, string, string) {
	t.Helper()
	repo := t.TempDir()
	evidence := t.TempDir()
	agents := filepath.Join(repo, "artifacts", "agents")
	writeJSONFile(t, filepath.Join(agents, "jobs"), "job-m.json", map[string]any{
		"jobId": "job-m", "round": 1, "parentJob": nil, "status": "completed",
		"role": "implementer", "mirror": nil,
		"capabilitySnapshot": "artifacts/agents/capabilities/snap.json",
	})
	os.WriteFile(filepath.Join(agents, "jobs", "job-m.log"), []byte("log\n"), 0o644)
	payload := filepath.Join(agents, "job-m")
	os.MkdirAll(filepath.Join(payload, "rounds", "1"), 0o755)
	os.WriteFile(filepath.Join(payload, "brief.md"), []byte("brief\n"), 0o644)
	os.WriteFile(filepath.Join(payload, "rounds", "1", "diff.patch"), []byte("patch\n"), 0o644)
	writeJSONFile(t, filepath.Join(agents, "capabilities"), "snap.json", map[string]any{"ok": true})
	return repo, evidence, "job-m"
}

func TestMirrorAndCloseCheck(t *testing.T) {
	repo, evidence, job := mirrorFixture(t)
	result := filepath.Join(t.TempDir(), "result.json")
	if err := Mirror(repo, repo, evidence, job, job, result); err != nil {
		t.Fatalf("Mirror: %v", err)
	}
	first := readJSONFile(t, result)
	if first["unchanged"] != false || asString(first["manifest"]) == "" {
		t.Fatalf("first mirror result = %v", first)
	}
	// A second pass with nothing changed is a no-op with the same manifest.
	if err := Mirror(repo, repo, evidence, job, job, result); err != nil {
		t.Fatalf("Mirror second pass: %v", err)
	}
	second := readJSONFile(t, result)
	if second["unchanged"] != true || second["manifest"] != first["manifest"] {
		t.Fatalf("second mirror result = %v", second)
	}

	// Closing requires the mirror stamp on the root record.
	jobs := filepath.Join(repo, "artifacts", "agents", "jobs")
	if err := CloseCheck(repo, job); err == nil {
		t.Fatalf("CloseCheck passed without a mirror stamp")
	}
	record := readJSONFile(t, filepath.Join(jobs, job+".json"))
	record["mirror"] = map[string]any{"path": asString(first["path"]), "manifest": first["manifest"]}
	writeRecord(filepath.Join(jobs, job+".json"), record)
	// The record changed after mirroring, so the manifest is stale until the
	// mirror runs once more.
	if err := Mirror(repo, repo, evidence, job, job, result); err != nil {
		t.Fatalf("Mirror after stamp: %v", err)
	}
	if err := CloseCheck(repo, job); err != nil {
		t.Fatalf("CloseCheck: %v", err)
	}

	// A drifted implementer diff refuses the close.
	os.WriteFile(filepath.Join(repo, "artifacts", "agents", job, "rounds", "1", "diff.patch"), []byte("drift\n"), 0o644)
	if err := CloseCheck(repo, job); err == nil ||
		!strings.Contains(err.Error(), "stale implementer diff.patch") {
		t.Fatalf("stale diff close = %v", err)
	}
}

func TestMirrorRefusesEvidenceInsideRepository(t *testing.T) {
	repo, _, job := mirrorFixture(t)
	result := filepath.Join(t.TempDir(), "result.json")
	err := Mirror(repo, repo, filepath.Join(repo, "evidence"), job, job, result)
	if err == nil || !strings.Contains(err.Error(), "inside the repository") {
		t.Fatalf("evidence inside repo = %v", err)
	}
}

// critiqueFixture lays out a register-backed design-critic chain at round
// three with one open severe finding.
func critiqueFixture(t *testing.T) (repo string) {
	t.Helper()
	repo = t.TempDir()
	writeCapRound(t, repo, "crit", "design-critic", 1, false,
		[]any{registerFindingValue("F-1", true, "direct evidence")}, []any{registerRigor("F-1", "severe")})
	writeCapRound(t, repo, "crit", "design-critic", 2, false, []any{}, []any{})
	writeCapRound(t, repo, "crit", "design-critic", 3, false, []any{}, []any{})
	return repo
}

func TestCritiqueExhaustionDesignCritic(t *testing.T) {
	repo := critiqueFixture(t)
	dir := t.TempDir()

	vague := filepath.Join(dir, "vague.md")
	os.WriteFile(vague, []byte("please continue\n"), 0o644)
	_, err := CritiqueExhaustionAdvance(repo, "crit", "design-critic", vague, "crit-r4")
	if err == nil || !strings.Contains(err.Error(), "F-1") {
		t.Fatalf("unenumerated successor = %v", err)
	}

	named := filepath.Join(dir, "named.md")
	os.WriteFile(named, []byte("Addressing F-1 head-on.\n"), 0o644)
	action, err := CritiqueExhaustionAdvance(repo, "crit", "design-critic", named, "crit-r4")
	if err != nil || action != "recorded" {
		t.Fatalf("enumerated successor = %q, %v", action, err)
	}
	root := readJSONFile(t, filepath.Join(repo, "artifacts", "agents", "jobs", "crit.json"))
	entries := root["critiqueExhaustions"].([]any)
	entry := entries[0].(map[string]any)
	if entry["successorJobId"] != "crit-r4" {
		t.Fatalf("owned root entry = %v", entry)
	}

	// A recorded second exhaustion at a different round refuses outright.
	jobs := filepath.Join(repo, "artifacts", "agents", "jobs")
	rootRecord := readJSONFile(t, filepath.Join(jobs, "crit.json"))
	rootRecord["critiqueExhaustions"] = []any{map[string]any{"round": 6, "successorJobId": "other"}}
	writeRecord(filepath.Join(jobs, "crit.json"), rootRecord)
	_, err = CritiqueExhaustionAdvance(repo, "crit", "design-critic", named, "crit-r4")
	if err == nil || !strings.Contains(err.Error(), "second critique exhaustion") {
		t.Fatalf("second exhaustion = %v", err)
	}
}

func TestCritiqueExhaustionRoundOffBudget(t *testing.T) {
	repo := t.TempDir()
	writeCapRound(t, repo, "crit", "design-critic", 1, false,
		[]any{registerFindingValue("F-9", true, "direct evidence")}, []any{registerRigor("F-9", "severe")})
	writeCapRound(t, repo, "crit", "design-critic", 2, false, []any{}, []any{})
	message := filepath.Join(t.TempDir(), "m.md")
	os.WriteFile(message, []byte("x\n"), 0o644)
	action, err := CritiqueExhaustionAdvance(repo, "crit", "design-critic", message, "next")
	if err != nil || action != "none" {
		t.Fatalf("off-budget round = %q, %v", action, err)
	}
}

func TestValidateMissionRefusals(t *testing.T) {
	root := t.TempDir()
	if err := ValidateMission(root, "Bad_ID", "/x"); err == nil ||
		!strings.Contains(err.Error(), "invalid mission id") {
		t.Fatalf("invalid id = %v", err)
	}
	lease := filepath.Join(root, "elsewhere", "lease.json")
	if err := ValidateMission(root, "m-1", lease); err == nil ||
		!strings.Contains(err.Error(), "non-canonical") {
		t.Fatalf("non-canonical lease = %v", err)
	}
	canonical := filepath.Join(root, "artifacts", "agents", "missions", "m-1", "lease.json")
	os.MkdirAll(filepath.Dir(canonical), 0o755)
	os.WriteFile(canonical, []byte(`{"missionId":"m-1"}`), 0o644)
	if err := ValidateMission(root, "m-1", canonical); err == nil ||
		!strings.Contains(err.Error(), "invalid shape or identity") {
		t.Fatalf("bad shape = %v", err)
	}
}

// A timed-out (or failed, or cancelled) implementer round never delivered a
// diff: CloseCheck must not demand one, or the chain is permanently
// uncloseable and the runner dies at mission
// end. A COMPLETED round without its deliverable is
// still a violation, and undelivered rounds' OTHER evidence must still be
// mirrored.
func TestCloseCheckToleratesUndeliveredImplementerRounds(t *testing.T) {
	repo, evidence, job := mirrorFixture(t)
	agents := filepath.Join(repo, "artifacts", "agents")

	// A follow-up round that timed out: record + round dir, no diff.patch.
	writeJSONFile(t, filepath.Join(agents, "jobs"), job+"-r2.json", map[string]any{
		"jobId": job + "-r2", "round": 2, "parentJob": job, "status": "timeout",
		"role": "implementer", "mirror": nil,
		"capabilitySnapshot": "artifacts/agents/capabilities/snap.json",
	})
	payload := filepath.Join(agents, job)
	os.MkdirAll(filepath.Join(payload, "rounds", "2"), 0o755)
	os.WriteFile(filepath.Join(payload, "rounds", "2", "raw.out"), []byte("killed at cap\n"), 0o644)

	// Mirror both members, stamp the root, mirror once more so the manifest
	// carries the stamped record's current state.
	result := filepath.Join(t.TempDir(), "result.json")
	for _, member := range []string{job, job + "-r2"} {
		if err := Mirror(repo, repo, evidence, job, member, result); err != nil {
			t.Fatalf("Mirror %s: %v", member, err)
		}
	}
	jobs := filepath.Join(agents, "jobs")
	first := readJSONFile(t, result)
	record := readJSONFile(t, filepath.Join(jobs, job+".json"))
	record["mirror"] = map[string]any{"path": asString(first["path"]), "manifest": first["manifest"]}
	writeRecord(filepath.Join(jobs, job+".json"), record)
	if err := Mirror(repo, repo, evidence, job, job, result); err != nil {
		t.Fatalf("Mirror after stamp: %v", err)
	}

	if err := CloseCheck(repo, job); err != nil {
		t.Fatalf("a timed-out round without a diff must not block the close: %v", err)
	}

	// A COMPLETED round without a diff closes too: the diff exists only
	// once the host's conformance review runs, so an unreviewed round has
	// none by construction — the delegation floor, not the close, owns
	// that workflow verdict.
	r2 := readJSONFile(t, filepath.Join(jobs, job+"-r2.json"))
	r2["status"] = "completed"
	writeRecord(filepath.Join(jobs, job+"-r2.json"), r2)
	if err := Mirror(repo, repo, evidence, job, job+"-r2", result); err != nil {
		t.Fatalf("Mirror r2: %v", err)
	}
	if err := CloseCheck(repo, job); err != nil {
		t.Fatalf("an unreviewed completed round must not wedge the close: %v", err)
	}

	// But a diff the MANIFEST knows and the disk lost is evidence loss.
	if err := os.Remove(filepath.Join(repo, "artifacts", "agents", job, "rounds", "1", "diff.patch")); err != nil {
		t.Fatal(err)
	}
	if err := CloseCheck(repo, job); err == nil ||
		!strings.Contains(err.Error(), "vanished after mirroring") {
		t.Fatalf("a mirrored-then-lost diff must refuse: %v", err)
	}
}

// dispatch-supervise-7: corrupt lineage has ONE verdict — a member of a
// cyclic or broken chain belongs to NO chain, in both walkers.
func TestLineageRootOneVerdict(t *testing.T) {
	jobs := filepath.Join(t.TempDir(), "jobs")
	os.MkdirAll(jobs, 0o755)
	writeJSONFile(t, jobs, "root.json", map[string]any{"jobId": "root", "status": "completed"})
	writeJSONFile(t, jobs, "kid.json", map[string]any{"jobId": "kid", "parentJob": "root", "status": "completed"})
	// A cycle pair: the OLD disk walker attributed whichever record its
	// broken walk stopped on; the one verdict is NO chain.
	writeJSONFile(t, jobs, "cyc-a.json", map[string]any{"jobId": "cyc-a", "parentJob": "cyc-b", "status": "completed"})
	writeJSONFile(t, jobs, "cyc-b.json", map[string]any{"jobId": "cyc-b", "parentJob": "cyc-a", "status": "completed"})
	// A non-string parent: no chain.
	writeJSONFile(t, jobs, "warp.json", map[string]any{"jobId": "warp", "parentJob": 7, "status": "completed"})
	// A parent outside the table: no chain.
	writeJSONFile(t, jobs, "orphan.json", map[string]any{"jobId": "orphan", "parentJob": "gone", "status": "completed"})

	members, err := chainMembers(jobs, "root")
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, member := range members {
		ids = append(ids, asString(member.record["jobId"]))
	}
	sort.Strings(ids)
	if len(ids) != 2 || ids[0] != "kid" || ids[1] != "root" {
		t.Fatalf("chain members = %v, want [kid root]", ids)
	}
}

// Code-critic chain exhaustion: a critic may never own the successor, while a
// recorded implementer successor reopens the budget.
func TestCritiqueExhaustionCodeCriticChain(t *testing.T) {
	repo := t.TempDir()
	agents := filepath.Join(repo, "artifacts", "agents")
	writeCapRound(t, repo, "critic", "code-critic", 1, false,
		[]any{registerFindingValue("F-10", true, "direct evidence")}, []any{registerRigor("F-10", "severe")})
	rootPath := filepath.Join(agents, "jobs", "critic.json")
	rootRecord := readJSONFile(t, rootPath)
	rootRecord["reviews"] = "impl"
	writeRecord(rootPath, rootRecord)
	writeCapRound(t, repo, "critic", "code-critic", 2, false, []any{}, []any{})
	writeCapRound(t, repo, "critic", "code-critic", 3, false, []any{}, []any{})
	message := filepath.Join(t.TempDir(), "m.md")
	os.WriteFile(message, []byte("Fix F-10 in the implementation follow-up.\n"), 0o644)

	// A critic-owned successor is refused toward the implementer follow-up.
	_, err := CritiqueExhaustionAdvance(repo, "critic", "code-critic", message, "critic-r4")
	if err == nil || !strings.Contains(err.Error(), "implementer follow-up") {
		t.Fatalf("critic-owned successor = %v", err)
	}

	// A recorded implementer successor reopens the critic budget: none.
	rootRecord = readJSONFile(t, filepath.Join(agents, "jobs", "critic.json"))
	rootRecord["critiqueExhaustions"] = []any{map[string]any{
		"round": 3, "openFindingIds": []any{"F-10"}, "successorJobId": "impl-r2",
	}}
	writeRecord(filepath.Join(agents, "jobs", "critic.json"), rootRecord)
	action, err := CritiqueExhaustionAdvance(repo, "critic", "code-critic", message, "critic-r4")
	if err != nil || action != "none" {
		t.Fatalf("recorded implementer successor should reopen: action=%q err=%v", action, err)
	}
}
