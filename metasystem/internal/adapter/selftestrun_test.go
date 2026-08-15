package adapter

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestValidateSelftestModel(t *testing.T) {
	if err := ValidateSelftestModel("claude", "opus-4"); err != nil {
		t.Fatalf("a filled model refused: %v", err)
	}
	for _, model := range []string{"", "<model>", "claude-<fill>"} {
		if ValidateSelftestModel("claude", model) == nil {
			t.Fatalf("placeholder model %q accepted", model)
		}
	}
}

// The denial taxonomy: a mapped declaration demands a denial-shaped failure,
// a notEnforced one asserts nothing in either direction.
func TestSelftestAttemptVerdict(t *testing.T) {
	cases := []struct {
		name    string
		outcome AttemptOutcome
		wantErr string
	}{
		{"not-enforced is silence even on completion",
			AttemptOutcome{Declared: "notEnforced", Status: "completed", EvidencePresent: true}, ""},
		{"mapped with evidence went through",
			AttemptOutcome{Declared: "mapped", Status: "failed", Error: "empty_reply", EvidencePresent: true},
			"went through"},
		{"mapped but completed",
			AttemptOutcome{Declared: "mapped", Status: "completed"}, "completed instead of being denied"},
		{"empty_reply is a denial",
			AttemptOutcome{Declared: "mapped", Status: "failed", Error: "empty_reply"}, ""},
		{"protocol_error is a denial",
			AttemptOutcome{Declared: "mapped", Status: "failed", Error: "protocol_error"}, ""},
		{"runtime_error is a denial",
			AttemptOutcome{Declared: "mapped", Status: "failed", Error: "runtime_error"}, ""},
		{"unresumable is not a denial",
			AttemptOutcome{Declared: "mapped", Status: "failed", Error: "unresumable"}, "which is not a denial"},
		{"an absent error is not a denial",
			AttemptOutcome{Declared: "mapped", Status: ""}, "which is not a denial"},
	}
	for _, c := range cases {
		err := SelftestAttemptVerdict("stub", "write", "writeRoots", c.outcome)
		if c.wantErr == "" && err != nil {
			t.Fatalf("%s: unexpected refusal: %v", c.name, err)
		}
		if c.wantErr != "" && (err == nil || !strings.Contains(err.Error(), c.wantErr)) {
			t.Fatalf("%s: verdict %v does not carry %q", c.name, err, c.wantErr)
		}
	}
}

func TestReturnProvesMarker(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "return.json")
	write := func(text string) {
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(`{"evidence":{"observations":["saw PERMITTED_READ:abc in the file"]},"findings":[]}`)
	if ok, err := ReturnProvesMarker(path, "PERMITTED_READ:abc"); err != nil || !ok {
		t.Fatalf("nested marker not found: ok=%v err=%v", ok, err)
	}
	// A marker in key position is not evidence.
	write(`{"PERMITTED_READ:abc":"nothing"}`)
	if ok, _ := ReturnProvesMarker(path, "PERMITTED_READ:abc"); ok {
		t.Fatal("a key-position marker satisfied an evidence assertion")
	}
	// A parsed read refuses what a byte grep would have accepted.
	write(`not json PERMITTED_READ:abc`)
	if ok, err := ReturnProvesMarker(path, "PERMITTED_READ:abc"); err == nil || ok {
		t.Fatalf("invalid JSON accepted: ok=%v err=%v", ok, err)
	}
	if ok, err := ReturnProvesMarker(filepath.Join(dir, "absent.json"), "x"); err == nil || ok {
		t.Fatalf("absent file accepted: ok=%v err=%v", ok, err)
	}
}

// The brief's bytes are pinned against the shell heredoc this replaced
// (make_selftest_brief): an independent spelling of the same document.
func TestSelftestBriefBytes(t *testing.T) {
	restore := now
	now = func() time.Time { return time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC) }
	defer func() { now = restore }()
	expected := `Working Mode: design
Orchestrator Identity: adapter-selftest
Date: 2026-08-14

# Goal

Probe the thing.

# Workspace

Use only the current scratch workspace. Do not modify plans.

# Inputs

The runtime role preamble and this brief are complete.

# Constraints

Keep the response short. Perform every explicitly requested probe.

# Expected Return

Return only schema-valid JSON for the design-critic role. Use no findings unless a requested probe fails.

# Acceptance Criteria

The requested behavior is visible in the evidence observations.

# Gap Rule

stop and report a gap; never fill it silently.
`
	if got := selftestBrief("Probe the thing."); got != expected {
		t.Fatalf("brief bytes drifted from the shell heredoc:\n%q\nwant:\n%q", got, expected)
	}
}

// stageSelftestFixture builds a checkout whose dispatch.sh is a stub runtime:
// dispatch records a completed job with a stable session and native usage,
// the permissions job's return embeds the workspace's PERMITTED_READ line
// (and the skill marker when the workspace carries one), the split attempt
// legs fail as empty_reply denials, and cancel flips the record to
// cancelled. The self-test's orchestration then runs end to end with no real
// model in the loop.
func stageSelftestFixture(t *testing.T, writeEnforcement, networkEnforcement string) string {
	t.Helper()
	root := t.TempDir()
	agents := filepath.Join(root, "artifacts", "agents")
	for _, dir := range []string{
		filepath.Join(agents, "jobs"),
		filepath.Join(agents, "capabilities"),
		filepath.Join(root, "scripts", "agents"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	snapshot := map[string]any{
		"runtime":    "stub",
		"capturedAt": "2026-08-14T00:00:00Z",
		"envelopeEnforcement": map[string]any{
			"writeRoots": writeEnforcement,
			"network":    networkEnforcement,
		},
	}
	data, _ := json.Marshal(snapshot)
	if err := os.WriteFile(filepath.Join(agents, "capabilities", "stub-1.0-fixture.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	dispatch := `#!/usr/bin/env bash
set -euo pipefail
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
jobs="$root/artifacts/agents/jobs"
verb=$1; shift
job= workspace=
while (($#)); do
  case "$1" in
    --job|--job-id) job=$2; shift 2 ;;
    --workspace) workspace=$2; shift 2 ;;
    --role|--brief|--runtime|--permissions|--message) shift 2 ;;
    *) shift ;;
  esac
done
record() { # job, status, error
  printf '{"jobId":"%s","status":"%s","error":"%s","sessionId":"sess-1","usage":{"availability":"native","inputTokens":1,"outputTokens":2}}\n' \
    "$1" "$2" "$3" >"$jobs/$1.json"
}
case "$verb" in
  dispatch)
    case "$job" in
      *-permissions-write|*-permissions-fetch)
        record "$job" failed empty_reply ;;
      *-permissions)
        record "$job" completed ""
        mkdir -p "$root/artifacts/agents/$job/rounds/1"
        marker=$(grep '^PERMITTED_READ:' "$workspace/permitted.txt")
        skill=""
        if [[ -f "$workspace/skills/metasystem-selftest/SKILL.md" ]]; then
          skill=$(tail -1 "$workspace/skills/metasystem-selftest/SKILL.md")
        fi
        printf '{"findings":[],"evidence":{"observations":["%s","%s"]}}\n' "$marker" "$skill" \
          >"$root/artifacts/agents/$job/rounds/1/return.json" ;;
      *)
        record "$job" completed "" ;;
    esac ;;
  follow-up)
    record "$job-r2" completed "" ;;
  cancel)
    record "$job" cancelled "" ;;
  status)
    [[ -f "$jobs/$job.json" ]] || { echo unknown; exit 0; }
    sed -n 's/.*"status":"\([a-z]*\)".*/\1/p' "$jobs/$job.json" ;;
  reap) ;;
esac
`
	for path, content := range map[string]string{
		filepath.Join(root, "scripts", "agents", "dispatch.sh"):     dispatch,
		filepath.Join(root, "scripts", "assert-return-complete.sh"): "#!/usr/bin/env bash\nexit 0\n",
		filepath.Join(root, "adapter.sh"):                           "#!/usr/bin/env bash\nexit 0\n",
	} {
		if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestSelftestRunMergedLegs(t *testing.T) {
	root := stageSelftestFixture(t, "mapped", "mapped")
	var out strings.Builder
	p := SelftestParams{
		Root: root, Runtime: "stub", AdapterPath: filepath.Join(root, "adapter.sh"),
		Usage: "native", TurnCeilingSec: 10,
	}
	if err := SelftestRun(p, "stub-model", &out); err != nil {
		t.Fatalf("merged-leg selftest failed: %v", err)
	}
	if !strings.Contains(out.String(), "stub adapter selftest passed") {
		t.Fatalf("missing the pass line: %q", out.String())
	}
	records, _ := filepath.Glob(filepath.Join(root, "artifacts", "agents", "selftests", "*.json"))
	if len(records) != 1 {
		t.Fatalf("expected one pass record, found %v", records)
	}
	record, err := readObject(records[0])
	if err != nil {
		t.Fatal(err)
	}
	proven := fmt.Sprintf("%v", record["provenBehaviorally"])
	for _, tag := range []string{"forbidden-write-denied", "denied-network", "usage-extraction"} {
		if !strings.Contains(proven, tag) {
			t.Fatalf("pass record does not prove %s: %s", tag, proven)
		}
	}
}

func TestSelftestRunSplitLegsWithDevinChecks(t *testing.T) {
	root := stageSelftestFixture(t, "mapped", "notEnforced")
	var out strings.Builder
	p := SelftestParams{
		Root: root, Runtime: "stub", AdapterPath: filepath.Join(root, "adapter.sh"),
		Usage: "native", TurnCeilingSec: 10, DenialEndsTurn: true,
	}
	devinProbe, err := SelftestProbeFor("devin", "symlinked-skill-discovery")
	if err != nil {
		t.Fatal(err)
	}
	p.Probe = &devinProbe
	if err := SelftestRun(p, "stub-model", &out); err != nil {
		t.Fatalf("split-leg selftest failed: %v", err)
	}
	// The notEnforced fetch attempt records that nothing was asserted.
	logs, _ := filepath.Glob(filepath.Join(root, "artifacts", "agents", "jobs", "*-permissions-fetch.log"))
	if len(logs) != 1 {
		t.Fatalf("expected the fetch attempt's unasserted note, found %v", logs)
	}
	note, _ := os.ReadFile(logs[0])
	if !strings.Contains(string(note), "declares network notEnforced; no containment is asserted") {
		t.Fatalf("unasserted note wrong: %q", note)
	}
	matches, globErr := filepath.Glob(filepath.Join(root, "artifacts", "agents", "selftests", "*.json"))
	record, err := readObject(mustOne(t, matches, globErr))
	if err != nil {
		t.Fatal(err)
	}
	proven := fmt.Sprintf("%v", record["provenBehaviorally"])
	if !strings.Contains(proven, "symlinked-skill-discovery") || strings.Contains(proven, "denied-network") {
		t.Fatalf("split-leg pass record wrong: %s", proven)
	}
}

func TestSelftestRunRefusesSessionDrift(t *testing.T) {
	root := stageSelftestFixture(t, "mapped", "mapped")
	// A follow-up that lands on a NEW session is the resume-identity defect
	// the leg exists to catch.
	dispatch := filepath.Join(root, "scripts", "agents", "dispatch.sh")
	text, err := os.ReadFile(dispatch)
	if err != nil {
		t.Fatal(err)
	}
	drifted := strings.Replace(string(text),
		`  follow-up)
    record "$job-r2" completed "" ;;`,
		`  follow-up)
    record "$job-r2" completed ""
    sed -i.bak 's/sess-1/sess-2/' "$jobs/$job-r2.json" ;;`, 1)
	if drifted == string(text) {
		t.Fatal("fixture drift edit did not apply")
	}
	if err := os.WriteFile(dispatch, []byte(drifted), 0o755); err != nil {
		t.Fatal(err)
	}
	p := SelftestParams{
		Root: root, Runtime: "stub", AdapterPath: filepath.Join(root, "adapter.sh"),
		Usage: "native", TurnCeilingSec: 10,
	}
	err = SelftestRun(p, "stub-model", &strings.Builder{})
	if err == nil || !strings.Contains(err.Error(), "resumed a different session") {
		t.Fatalf("session drift not refused: %v", err)
	}
}

func mustOne(t *testing.T, matches []string, err error) string {
	t.Helper()
	if err != nil || len(matches) != 1 {
		t.Fatalf("expected exactly one match: %v %v", matches, err)
	}
	return matches[0]
}

// The tripwire's positive path: a connection is answered once and its bytes
// become the request log — the very evidence a denied fetch got through.
func TestSelftestTripwireRecordsTheOneRequest(t *testing.T) {
	requestLog := filepath.Join(t.TempDir(), "network-requested")
	port, stop, err := startTripwire(requestLog, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	connection, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	if _, err := connection.Write([]byte("GET /nonce-xyz HTTP/1.1\r\n\r\n")); err != nil {
		t.Fatal(err)
	}
	reply := make([]byte, 64)
	n, _ := connection.Read(reply)
	if !strings.Contains(string(reply[:n]), "200 OK") {
		t.Fatalf("tripwire reply wrong: %q", reply[:n])
	}
	stop()
	logged, err := os.ReadFile(requestLog)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(logged), "GET /nonce-xyz") {
		t.Fatalf("request log missing the request: %q", logged)
	}
}

// The orchestration's refusal branches, each driven end to end against a
// broken variant of the fixture.
func TestSelftestRunRefusals(t *testing.T) {
	breakFixture := func(root, old, new string) {
		t.Helper()
		dispatch := filepath.Join(root, "scripts", "agents", "dispatch.sh")
		text, err := os.ReadFile(dispatch)
		if err != nil {
			t.Fatal(err)
		}
		edited := strings.Replace(string(text), old, new, 1)
		if edited == string(text) {
			t.Fatalf("fixture edit did not apply: %q", old)
		}
		if err := os.WriteFile(dispatch, []byte(edited), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	params := func(root string) SelftestParams {
		return SelftestParams{
			Root: root, Runtime: "stub", AdapterPath: filepath.Join(root, "adapter.sh"),
			Usage: "native", TurnCeilingSec: 10,
		}
	}

	t.Run("placeholder model", func(t *testing.T) {
		err := SelftestRun(params(stageSelftestFixture(t, "mapped", "mapped")), "<model>", &strings.Builder{})
		if err == nil || !strings.Contains(err.Error(), "filled role.default.model.stub") {
			t.Fatalf("placeholder model accepted: %v", err)
		}
	})
	t.Run("non-positive ceiling", func(t *testing.T) {
		p := params(stageSelftestFixture(t, "mapped", "mapped"))
		p.TurnCeilingSec = 0
		if err := SelftestRun(p, "stub-model", &strings.Builder{}); err == nil {
			t.Fatal("zero ceiling accepted")
		}
	})
	t.Run("adapter probe refusal aborts", func(t *testing.T) {
		root := stageSelftestFixture(t, "mapped", "mapped")
		if err := os.WriteFile(filepath.Join(root, "adapter.sh"),
			[]byte("#!/usr/bin/env bash\nexit 3\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := SelftestRun(params(root), "stub-model", &strings.Builder{}); err == nil {
			t.Fatal("failing adapter accepted")
		}
	})
	t.Run("failed main dispatch", func(t *testing.T) {
		root := stageSelftestFixture(t, "mapped", "mapped")
		breakFixture(root, `      *)
        record "$job" completed "" ;;`, `      *)
        record "$job" failed runtime_error ;;`)
		err := SelftestRun(params(root), "stub-model", &strings.Builder{})
		if err == nil || !strings.Contains(err.Error(), "selftest dispatch failed") {
			t.Fatalf("failed dispatch not refused: %v", err)
		}
	})
	t.Run("unrecorded cancellation", func(t *testing.T) {
		root := stageSelftestFixture(t, "mapped", "mapped")
		breakFixture(root, `  cancel)
    record "$job" cancelled "" ;;`, `  cancel)
    : ;;`)
		err := SelftestRun(params(root), "stub-model", &strings.Builder{})
		if err == nil || !strings.Contains(err.Error(), "cancellation was not recorded") {
			t.Fatalf("missing cancellation not refused: %v", err)
		}
	})
	t.Run("mapped write attempt that completed", func(t *testing.T) {
		root := stageSelftestFixture(t, "mapped", "mapped")
		breakFixture(root, `      *-permissions-write|*-permissions-fetch)
        record "$job" failed empty_reply ;;`, `      *-permissions-write|*-permissions-fetch)
        record "$job" completed "" ;;`)
		p := params(root)
		p.DenialEndsTurn = true
		err := SelftestRun(p, "stub-model", &strings.Builder{})
		if err == nil || !strings.Contains(err.Error(), "completed instead of being denied") {
			t.Fatalf("completed attempt not refused: %v", err)
		}
	})
}

// The remaining evidence refusals: a return that cannot prove the permitted
// read, a mapped write that landed despite a denied-shaped attempt record,
// and a follow-up leg that fails outright.
func TestSelftestRunEvidenceRefusals(t *testing.T) {
	edit := func(root, old, new string) {
		t.Helper()
		dispatch := filepath.Join(root, "scripts", "agents", "dispatch.sh")
		text, err := os.ReadFile(dispatch)
		if err != nil {
			t.Fatal(err)
		}
		edited := strings.Replace(string(text), old, new, 1)
		if edited == string(text) {
			t.Fatalf("fixture edit did not apply: %q", old)
		}
		if err := os.WriteFile(dispatch, []byte(edited), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	params := func(root string) SelftestParams {
		return SelftestParams{
			Root: root, Runtime: "stub", AdapterPath: filepath.Join(root, "adapter.sh"),
			Usage: "native", TurnCeilingSec: 10,
		}
	}
	t.Run("return without the nonce", func(t *testing.T) {
		root := stageSelftestFixture(t, "mapped", "mapped")
		edit(root, `marker=$(grep '^PERMITTED_READ:' "$workspace/permitted.txt")`, `marker="unproven"`)
		err := SelftestRun(params(root), "stub-model", &strings.Builder{})
		if err == nil || !strings.Contains(err.Error(), "did not prove the permitted read") {
			t.Fatalf("unproven read accepted: %v", err)
		}
	})
	t.Run("mapped write landed anyway", func(t *testing.T) {
		root := stageSelftestFixture(t, "mapped", "mapped")
		edit(root, `      *-permissions)
        record "$job" completed ""`, `      *-permissions)
        record "$job" completed ""
        touch "$workspace/forbidden.txt"`)
		err := SelftestRun(params(root), "stub-model", &strings.Builder{})
		if err == nil || !strings.Contains(err.Error(), "permission mapping allowed a forbidden write") {
			t.Fatalf("landed write accepted: %v", err)
		}
	})
	t.Run("failed follow-up", func(t *testing.T) {
		root := stageSelftestFixture(t, "mapped", "mapped")
		edit(root, `  follow-up)
    record "$job-r2" completed "" ;;`, `  follow-up)
    record "$job-r2" failed runtime_error ;;`)
		err := SelftestRun(params(root), "stub-model", &strings.Builder{})
		if err == nil || !strings.Contains(err.Error(), "selftest follow-up failed") {
			t.Fatalf("failed follow-up accepted: %v", err)
		}
	})
	t.Run("missing devin skill proof", func(t *testing.T) {
		root := stageSelftestFixture(t, "mapped", "mapped")
		edit(root, `skill=$(tail -1 "$workspace/skills/metasystem-selftest/SKILL.md")`, `skill="unproven"`)
		p := params(root)
		devinProbe, probeErr := SelftestProbeFor("devin", "symlinked-skill-discovery")
		if probeErr != nil {
			t.Fatal(probeErr)
		}
		p.Probe = &devinProbe
		err := SelftestRun(p, "stub-model", &strings.Builder{})
		if err == nil || !strings.Contains(err.Error(), "symlinked .agents/skills discovery") {
			t.Fatalf("missing skill proof accepted: %v", err)
		}
	})
}

// adjudicateValidate's three verdicts: normalization failure becomes the
// violation text, an incomplete return lists its violations, and the
// completeness judgment runs against the job's real layout.
func TestAdjudicateValidate(t *testing.T) {
	dir := t.TempDir()
	recordPath := filepath.Join(dir, "job.json")
	if err := os.WriteFile(recordPath, []byte(`{"effectiveModel": "m1"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	params := AdjudicateParams{
		Root:         dir,
		Job:          "j",
		RecordPath:   recordPath,
		ReturnPath:   filepath.Join(dir, "return.json"),
		MarkdownPath: filepath.Join(dir, "return.md"),
		SessionID:    "s1",
	}

	garbage := filepath.Join(dir, "garbage.out")
	if err := os.WriteFile(garbage, []byte("no json here"), 0o644); err != nil {
		t.Fatal(err)
	}
	text, err := adjudicateValidate(params, garbage, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(text, "return normalization failed: ") {
		t.Fatalf("normalization failure text wrong: %q", text)
	}

	// A normalizable reply whose job layout is incomplete yields violation
	// lines, one per finding.
	reply := filepath.Join(dir, "reply.out")
	if err := os.WriteFile(reply, []byte(`{"schemaVersion": 2, "jobId": "j", "round": 1, "runtime": "codex",
	  "sessionId": "s1", "model": {"effective": "m1"}, "evidence": [], "gaps": [], "mode": "design"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	text, err = adjudicateValidate(params, reply, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "violation: ") {
		t.Fatalf("incomplete job produced no violations: %q", text)
	}
}

// waitForJob gives up at the runtime's ceiling when a job never terminates.
func TestSelftestWaitForJobCeiling(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "scripts", "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "scripts", "agents", "dispatch.sh"),
		[]byte("#!/usr/bin/env bash\n[[ $1 == status ]] && echo running\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	p := SelftestParams{Root: root, Runtime: "stub", TurnCeilingSec: 1}
	start := time.Now()
	if p.waitForJob("stuck-job") {
		t.Fatal("a stuck job read as terminal")
	}
	if time.Since(start) > 5*time.Second {
		t.Fatal("ceiling did not bound the wait")
	}
}
