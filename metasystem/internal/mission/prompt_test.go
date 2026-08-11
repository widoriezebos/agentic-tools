package mission

import (
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
		"# Mission m1\n\n```mission\nfence.cycles=10\nfence.jobs=4\nstream.s1=Do the thing\n```\n")

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
		"## This Turn",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("assembled prompt is missing %q\n---\n%s", want, got)
		}
	}
	// Fence headroom interpolates into This Turn: 10-3 cycles, 4-2 jobs.
	if !strings.Contains(got, "Fence headroom: cycles=7,jobs=2") {
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

	// The eight sections appear exactly once and in the fixed order.
	markers := []string{
		"Mission-Id: m1",
		"# Orchestrator",
		"## Mission Contract",
		"## Ledger Tail",
		"## Open Asks",
		"## Streams",
		"## Reconciliation",
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
	big := "# Mission m1\n\n```mission\nfence.cycles=10\nfence.jobs=4\n```\n" + strings.Repeat("x", 2048)
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
