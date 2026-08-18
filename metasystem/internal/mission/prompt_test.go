package mission

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// promptSandbox lays out a minimal but complete mission tree under a temp repo
// and returns the repo root. The two prompt-size settings are pinned through
// the environment so assembly is independent of any ambient configuration.
func promptSandbox(t *testing.T) string {
	t.Helper()
	t.Setenv("METASYSTEM_MISSION_LEDGER_TAIL_CYCLES", "5")
	t.Setenv("METASYSTEM_MISSION_MAX_PROMPT_KB", "256")
	repo := t.TempDir()

	write := func(rel, content string) {
		path := filepath.Join(repo, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write("metasystem.conf", "metasystem.runtimes=fake\n")
	write("scripts/agents/roles/orchestrator.md", "# Orchestrator\n\nYou orchestrate.\n")
	write("scripts/agents/templates/host-turn-instruction.md",
		"Cycle: <cycle-number>\nFence headroom: <fence-headroom>\nReconciliation: <yes | no>\n\nAdvance active streams.\n")
	write("plans/mission-m1.contract.md",
		"# Mission m1\n\n```mission\nfence.cycles=10\nfence.jobs=4\nfence.concurrency=2\nstream.s1=Do the thing\n```\n")

	base := "artifacts/agents/missions/m1"
	write(base+"/ledger.md",
		"# Mission Ledger\n\n- Cycle budget: 5\n- No-gain budget: 3\n\n"+
			"### Cycle 1\n- Classification: contract-improved; candidate-sha=abc123; observed=score=0.5\n\n"+
			"### Cycle 2\n- Classification: no-progress; candidate-sha=def456; observed=score=0.6\n")
	write(base+"/fences.json", `{"cycles":3,"reservations":{"job-a":{},"job-b":{}}}`)
	write(base+"/state.json",
		`{"missionId":"m1","streams":{`+
			`"s1":{"state":"active","goal":"Do the thing","reason":null},`+
			`"s2":{"state":"parked-stop-loss","goal":"Second goal","reason":"stop-loss"}},`+
			`"turnLog":[{"turnId":"t0","outcome":"host-failure","detail":"host crashed"}]}`)
	write(base+"/asks/ask-2.json",
		`{"askId":"ask-2","streamId":"s1","reasonClass":"fence","question":"Approve the budget?","answeredAt":null}`)
	write(base+"/asks/ask-1.json",
		`{"askId":"ask-1","streamId":null,"reasonClass":"design","question":"Which approach?"}`)
	write(base+"/asks/ask-0.json",
		`{"askId":"ask-0","streamId":"s1","reasonClass":"design","question":"Answered already","answeredAt":"2026-01-01T00:00:00Z"}`)
	write(base+"/turns/t1/turn.json",
		`{"missionId":"m1","turnId":"t1","cycle":2,"runtime":"claude","model":"opus","reconciliation":false,"hostSession":"sess-123"}`)

	return repo
}

func TestAssemblePromptByteStable(t *testing.T) {
	repo := promptSandbox(t)
	out1 := filepath.Join(t.TempDir(), "prompt-1.txt")
	out2 := filepath.Join(t.TempDir(), "prompt-2.txt")

	if err := AssemblePrompt(repo, "m1", "t1", out1); err != nil {
		t.Fatal(err)
	}
	if err := AssemblePrompt(repo, "m1", "t1", out2); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(out1)
	if err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(out2)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatal("the same inputs must produce identical prompt bytes")
	}

	got := string(first)
	for _, want := range []string{
		"Mission-Id: m1",
		"Turn-Id: t1",
		"Cycle: 2",
		"Host-Session: sess-123",
		"Runtime: claude",
		"Model: opus",
		"Reconciliation: no",
		"# Orchestrator",
		"## Mission Contract",
		"## Ledger Tail",
		"<<<DATA>>>",
		"<<<END>>>",
		"## Open Asks",
		"## Streams",
		"## Reconciliation",
		"## Landed Returns",
		"## This Turn",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("assembled prompt is missing %q\n---\n%s", want, got)
		}
	}
	// Fence headroom interpolates into This Turn: 10-3 cycles, 4-2 jobs.
	if !strings.Contains(got, "Fence headroom: cycles=7,jobs=2,concurrency=") {
		t.Fatalf("fence headroom was not interpolated:\n%s", got)
	}
	// A trailing newline terminates the prompt; the placeholder never leaks.
	if !strings.HasSuffix(got, "\n") || strings.Contains(got, "<cycle-number>") {
		t.Fatalf("prompt framing is wrong:\n%s", got)
	}
}

func TestAssemblePromptOrdersAndFramesData(t *testing.T) {
	repo := promptSandbox(t)
	out := filepath.Join(t.TempDir(), "prompt.txt")
	if err := AssemblePrompt(repo, "m1", "t1", out); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	text := string(got)

	// The nine sections appear exactly once and in the fixed order.
	markers := []string{
		"Mission-Id: m1",
		"# Orchestrator",
		"## Mission Contract",
		"## Ledger Tail",
		"## Open Asks",
		"## Streams",
		"## Reconciliation",
		"## Landed Returns",
		"## This Turn",
	}
	last := -1
	for _, marker := range markers {
		at := strings.Index(text, marker)
		if at < 0 {
			t.Fatalf("section %q missing", marker)
		}
		if at <= last {
			t.Fatalf("section %q is out of order", marker)
		}
		last = at
	}

	// The open asks are ordered by id, and the answered ask is excluded.
	if strings.Contains(text, "ask-0") {
		t.Fatal("an answered ask must not appear in Open Asks")
	}
	first := strings.Index(text, "ask-1")
	second := strings.Index(text, "ask-2")
	if first < 0 || second < 0 || first > second {
		t.Fatalf("open asks are not ordered by id: ask-1@%d ask-2@%d", first, second)
	}
	// A null stream id renders as the neutral placeholder, tab-delimited.
	if !strings.Contains(text, "ask-1\tnone\tdesign\tWhich approach?") {
		t.Fatalf("a null field did not render as a neutral one-line placeholder:\n%s", text)
	}
	// Reconciliation not required for this turn: the block is the placeholder.
	reconIdx := strings.Index(text, "## Reconciliation")
	thisTurnIdx := strings.Index(text, "## This Turn")
	reconBlock := text[reconIdx:thisTurnIdx]
	if !strings.Contains(reconBlock, "(none)") {
		t.Fatalf("reconciliation was not required and should be empty:\n%s", reconBlock)
	}
}

func TestPromptLedgerRecordsCarryTheBestMarker(t *testing.T) {
	dir := t.TempDir()
	ledger := filepath.Join(dir, "ledger.md")
	text := "# Mission Ledger\n\n- Cycle budget: 5\n- No-gain budget: 3\n\n" +
		"### Cycle 1\n- Classification: contract-improved; candidate-sha=abc123; observed=score=0.5\n\n" +
		"### Cycle 2\n- Classification: no-progress; candidate-sha=def456; observed=score=0.4; best=no\n\n" +
		"Stop-loss reset: ask=stop-loss; reason=keep going\n\n" +
		"### Cycle 3\n- Classification: contract-improved; candidate-sha=def456; observed=score=0.9; best=yes\n"
	if err := os.WriteFile(ledger, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	records, err := promptLedgerRecords(ledger, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 3 {
		t.Fatalf("records: %v", records)
	}
	// A marker-less line stays a four-field record; a marked line gains the
	// marker as a fifth field with the observed value intact.
	if len(records[0]) != 4 || records[0][3] != "score=0.5" {
		t.Fatalf("marker-less record changed shape: %v", records[0])
	}
	if len(records[1]) != 5 || records[1][3] != "score=0.4" || records[1][4] != "no" {
		t.Fatalf("best=no record: %v", records[1])
	}
	if len(records[2]) != 5 || records[2][4] != "yes" {
		t.Fatalf("best=yes record: %v", records[2])
	}
}

func TestAssemblePromptReconciliationSurfacesPriorTurn(t *testing.T) {
	repo := promptSandbox(t)
	// Rewrite the turn to require reconciliation.
	turnPath := filepath.Join(repo, "artifacts/agents/missions/m1/turns/t1/turn.json")
	if err := os.WriteFile(turnPath, []byte(
		`{"missionId":"m1","turnId":"t1","cycle":2,"runtime":"claude","model":"opus","reconciliation":true,"hostSession":null}`),
		0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "prompt.txt")
	if err := AssemblePrompt(repo, "m1", "t1", out); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	text := string(got)
	if !strings.Contains(text, "Reconciliation: yes") {
		t.Fatal("the header must announce reconciliation is required")
	}
	if !strings.Contains(text, "t0\thost-failure\thost crashed") {
		t.Fatalf("the prior failed turn must surface in Reconciliation:\n%s", text)
	}
	if !strings.Contains(text, "Host-Session: none") {
		t.Fatal("a null host session must render as the neutral placeholder")
	}
}

func TestAssemblePromptRejectsBadIdentity(t *testing.T) {
	repo := promptSandbox(t)
	out := filepath.Join(t.TempDir(), "prompt.txt")
	if err := AssemblePrompt(repo, "M1", "t1", out); err == nil {
		t.Fatal("an id outside the grammar must be refused")
	}
	if err := AssemblePrompt(repo, "m1", "does-not-exist", out); err == nil {
		t.Fatal("a missing turn record must be refused")
	}
}

func TestAssemblePromptEnforcesSizeCeiling(t *testing.T) {
	repo := promptSandbox(t)
	t.Setenv("METASYSTEM_MISSION_MAX_PROMPT_KB", "1")
	out := filepath.Join(t.TempDir(), "prompt.txt")
	// Make the contract dominate so the oversized block is named.
	big := "# Mission m1\n\n```mission\nfence.cycles=10\nfence.jobs=4\nfence.concurrency=2\n```\n" + strings.Repeat("x", 2048)
	if err := os.WriteFile(filepath.Join(repo, "plans/mission-m1.contract.md"), []byte(big), 0o644); err != nil {
		t.Fatal(err)
	}
	err := AssemblePrompt(repo, "m1", "t1", out)
	if err == nil {
		t.Fatal("a prompt over the size ceiling must be refused")
	}
	if !strings.Contains(err.Error(), "oversized block: ## Mission Contract") {
		t.Fatalf("the refusal must name the oversized block, got: %v", err)
	}
}

func TestPromptConfigValueResolutionOrder(t *testing.T) {
	repo := t.TempDir()
	conf := filepath.Join(repo, "metasystem.conf")
	if err := os.WriteFile(conf, []byte("mission.max-prompt-kb=128\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The committed file supplies the value when nothing overrides it.
	if v, err := promptConfigValue(repo, "mission.max-prompt-kb", "256"); err != nil || v != "128" {
		t.Fatalf("committed value should resolve: %q, %v", v, err)
	}
	// The default applies when the key is absent everywhere.
	if v, err := promptConfigValue(repo, "mission.ledger-tail-cycles", "5"); err != nil || v != "5" {
		t.Fatalf("default should resolve: %q, %v", v, err)
	}
	// A local override outranks the committed file.
	if err := os.WriteFile(conf+".local", []byte("mission.max-prompt-kb=64\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if v, err := promptConfigValue(repo, "mission.max-prompt-kb", "256"); err != nil || v != "64" {
		t.Fatalf("local override should win: %q, %v", v, err)
	}
	// The environment outranks every file.
	t.Setenv("METASYSTEM_MISSION_MAX_PROMPT_KB", "32")
	if v, err := promptConfigValue(repo, "mission.max-prompt-kb", "256"); err != nil || v != "32" {
		t.Fatalf("environment override should win: %q, %v", v, err)
	}
	// A duplicate key is a configuration error.
	if err := os.WriteFile(conf, []byte("mission.cycles=1\nmission.cycles=2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := promptConfigValue(repo, "mission.cycles", "x"); err == nil {
		t.Fatal("a duplicated configuration key must be an error")
	}
}

// Patience prompt projection (plans/patience-satellite-4.md r2/P4-017,
// r9/P4-061, r10/P4-068): lines derive from the final cycle block's
// annotations; detail lines filter against current chain-closed flags;
// overflow and excluded lines are exempt.
func TestPatiencePromptLines(t *testing.T) {
	dir := t.TempDir()
	ledger := filepath.Join(dir, "ledger.md")
	if err := InitLedger(ledger, 5, 3); err != nil {
		t.Fatal(err)
	}
	sha := strings.Repeat("a", 40)
	if err := AppendCycle(ledger, 1, "no-progress", sha, "score=0", "",
		PatienceChainAnnotation("chain-open", 4, 2),
		PatienceChainAnnotation("chain-shut", 5, 2),
		PatienceOrphanAnnotation("orphan-x", 1),
		PatienceExcludedAnnotation(2),
		PatienceOverflowAnnotation(3)); err != nil {
		t.Fatal(err)
	}
	jobs := filepath.Join(dir, "jobs")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeJSON := func(name string, doc string) {
		if err := os.WriteFile(filepath.Join(jobs, name), []byte(doc), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeJSON("chain-open.json", `{"jobId":"chain-open","chainClosed":false}`)
	writeJSON("chain-shut.json", `{"jobId":"chain-shut","chainClosed":true}`)

	lines := patiencePromptLines(ledger, jobs)
	want := []string{
		"Patience: chain chain-open has 4 unwitnessed rounds (floor 2) — certify landed value or close the chain.",
		"Patience: orphan job orphan-x has unwitnessed spend — certify landed value or flag it to the human.",
		"Patience: 2 record(s) excluded from patience — flag it to the human.",
		"Patience: 3 more chains need attention (see ledger).",
	}
	if len(lines) != len(want) {
		t.Fatalf("projection wrong: %v", lines)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Fatalf("line %d: got %q want %q", i, lines[i], want[i])
		}
	}
	// Only the FINAL cycle projects; an unannotated later cycle silences all.
	if err := AppendCycle(ledger, 2, "contract-improved", sha, "score=1", "yes"); err != nil {
		t.Fatal(err)
	}
	if lines := patiencePromptLines(ledger, jobs); len(lines) != 0 {
		t.Fatalf("stale cycle projected: %v", lines)
	}
	// A missing ledger projects nothing and never fails assembly.
	if lines := patiencePromptLines(filepath.Join(dir, "absent.md"), jobs); lines != nil {
		t.Fatalf("missing ledger projected: %v", lines)
	}
}

// GOAL-09 both ways: a usable Current goal projects exactly one optional
// section between the contract and the ledger tail, and the assembled
// prompt passes the turn-prompt grammar; a degraded or absent ledger
// produces no line and never blocks assembly.
func TestPromptGoalSection(t *testing.T) {
	repo := promptSandbox(t)

	// Absent ledger: no line, assembly clean.
	out := filepath.Join(t.TempDir(), "prompt-absent.txt")
	if err := AssemblePrompt(repo, "m1", "t1", out); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(out)
	if strings.Contains(string(data), "## Serving goal") {
		t.Fatal("an absent ledger projected a goal section")
	}

	// A usable Current goal: the one-line block lands after the contract.
	seedLedger := "# Goals\n\n## Current goal: ship-it — Ship the whole thing\n- Origin: human\n- Next step: Land it.\n"
	write := func(rel, content string) {
		path := filepath.Join(repo, rel)
		os.MkdirAll(filepath.Dir(path), 0o755)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("plans/goals.md", seedLedger)
	// Baseline in step (the projection requires a fully usable ledger).
	gs := &goalStoreForTest{repo: repo}
	gs.accept(t, seedLedger)

	out2 := filepath.Join(t.TempDir(), "prompt-goal.txt")
	if err := AssemblePrompt(repo, "m1", "t1", out2); err != nil {
		t.Fatal(err)
	}
	text, _ := os.ReadFile(out2)
	idx := strings.Index(string(text), "## Serving goal\nship-it — Ship the whole thing")
	contractIdx := strings.Index(string(text), "## Mission Contract")
	ledgerIdx := strings.Index(string(text), "## Ledger Tail")
	if idx < 0 || idx < contractIdx || idx > ledgerIdx {
		t.Fatalf("goal section missing or misplaced (contract=%d goal=%d ledger=%d)", contractIdx, idx, ledgerIdx)
	}

	// Degraded (manual edit, baseline mismatch): the line disappears,
	// assembly still succeeds.
	write("plans/goals.md", seedLedger+"\n## Queued goal: extra — More\n- Origin: main\n- Next step: Q.\n")
	out3 := filepath.Join(t.TempDir(), "prompt-degraded.txt")
	if err := AssemblePrompt(repo, "m1", "t1", out3); err != nil {
		t.Fatal(err)
	}
	text3, _ := os.ReadFile(out3)
	if strings.Contains(string(text3), "## Serving goal") {
		t.Fatal("a degraded ledger projected a goal section")
	}
}

// goalStoreForTest writes the accepted baseline the way the goal verbs
// do, without importing their unexported plumbing: full bytes + digest.
type goalStoreForTest struct{ repo string }

func (g *goalStoreForTest) accept(t *testing.T, ledger string) {
	t.Helper()
	sum := sha256.Sum256([]byte(ledger))
	baseline := map[string]any{
		"schemaVersion": 1,
		"ledger":        ledger,
		"sha256":        hex.EncodeToString(sum[:]),
	}
	data, err := json.MarshalIndent(baseline, "", " ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(g.repo, "plans", "goals-accepted.json")
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Issue #11's regression: a human's answer reaches the orchestrator. The
// stream's answeredAsk names an answered ask; the next prompt carries the
// answer VERBATIM in ## Human Answers, the stream row names the ask, and
// an ask superseded by a sharper successor is not listed as open.
func TestAssemblePromptCarriesHumanAnswers(t *testing.T) {
	repo := promptSandbox(t)
	base := "artifacts/agents/missions/m1"
	write := func(rel, content string) {
		path := filepath.Join(repo, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	answer := "Option A. The code critic is authorized on runtime claude with model claude-opus-5 for this mission only."
	write(base+"/asks/ask-2-1.json",
		`{"askId":"ask-2-1","streamId":"s1","reasonClass":"reserved-decision","question":"May the critic run on opus?",`+
			`"answeredAt":"2026-08-18T14:59:12Z","answer":"`+answer+`","supersedes":"ask-1-1","supersededBy":null}`)
	write(base+"/asks/ask-1-1.json",
		`{"askId":"ask-1-1","streamId":"s1","reasonClass":"reserved-decision","question":"The older duplicate question",`+
			`"answeredAt":null,"answer":null,"supersedes":null,"supersededBy":"ask-2-1"}`)
	write(base+"/state.json",
		`{"missionId":"m1","streams":{`+
			`"s1":{"state":"active","goal":"Do the thing","reason":null,"answeredAsk":"ask-2-1"},`+
			`"s2":{"state":"parked-stop-loss","goal":"Second goal","reason":"stop-loss"}},`+
			`"turnLog":[]}`)

	out := filepath.Join(t.TempDir(), "prompt.txt")
	if err := AssemblePrompt(repo, "m1", "t1", out); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	text := string(got)

	if !strings.Contains(text, "## Human Answers") {
		t.Fatalf("prompt lacks the Human Answers section:\n%s", text)
	}
	if !strings.Contains(text, answer) {
		t.Fatal("the human's answer must appear VERBATIM in the prompt")
	}
	answers := text[strings.Index(text, "## Human Answers"):strings.Index(text, "## Open Asks")]
	if !strings.Contains(answers, "ask-2-1\ts1\t2026-08-18T14:59:12Z\tMay the critic run on opus?\t"+answer) {
		t.Fatalf("the answer row is malformed:\n%s", answers)
	}
	openAsks := text[strings.Index(text, "## Open Asks"):strings.Index(text, "## Streams")]
	if strings.Contains(openAsks, "ask-1-1") {
		t.Fatal("a superseded ask must not be listed as open")
	}
	if strings.Contains(openAsks, "ask-2-1") {
		t.Fatal("an answered ask must not be listed as open")
	}
	if !strings.Contains(text, "s1\tactive\tDo the thing\tnone\task-2-1") {
		t.Fatalf("the stream row must name its answered ask:\n%s", text)
	}
}
