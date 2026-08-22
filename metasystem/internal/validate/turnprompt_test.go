package validate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testPreamble = "# Orchestrator\n\nFollow the contract.\n"

// promptFixture builds a metasystem root with the shipped preamble, a
// turn directory with its record, and a valid assembled prompt, then
// lets the caller distort any piece before validation.
func promptFixture(t *testing.T) (root, promptPath, turnDir string) {
	t.Helper()
	root = t.TempDir()
	writeFile(t, filepath.Join(root, "scripts", "agents", "roles", "orchestrator.md"), testPreamble)
	turnDir = filepath.Join(root, "turn")
	writeFile(t, filepath.Join(turnDir, "turn.json"), `{"missionId":"m-1","turnId":"t-1"}`)
	promptPath = filepath.Join(root, "prompt.txt")
	writeFile(t, promptPath, validPrompt())
	return root, promptPath, turnDir
}

func validPrompt() string {
	header := strings.Join([]string{
		"Mission-Id: m-1",
		"Turn-Id: t-1",
		"Cycle: 3",
		"Host-Session: session-1",
		"Runtime: claude",
		"Model: strong",
		"Reconciliation: none",
	}, "\n")
	sha := strings.Repeat("a", 40)
	sections := strings.Join([]string{
		"## Mission Contract",
		"the sealed contract text",
		"",
		"## Ledger Tail",
		"<<<DATA>>>",
		"1\tcontract-improved\tnone\tgate green",
		"2\tno-progress\t" + sha + "\tno gain",
		"<<<END>>>",
		"",
		"## Human Answers",
		"<<<DATA>>>",
		"ask-0\tstream-a\t2026-08-18T00:00:00Z\tmay we use opus\tyes, option A, this mission only",
		"<<<END>>>",
		"",
		"## Open Asks",
		"<<<DATA>>>",
		"ask-1\tstream-a\treserved-decision\tmay we drop the flag",
		"<<<END>>>",
		"",
		"## Streams",
		"<<<DATA>>>",
		"stream-a\tactive\tship it\ton track\task-0",
		"stream-b\tdone\tship more\tfinished\tnone",
		"<<<END>>>",
		"",
		"## Reconciliation",
		"<<<DATA>>>",
		"(none)",
		"<<<END>>>",
		"",
		"## Landed Returns",
		"<<<DATA>>>",
		"(none)",
		"<<<END>>>",
		"",
		"## This Turn",
		"Decide the next step.",
		"",
	}, "\n")
	return header + "\n\n" + testPreamble + "\n" + sections
}

func TestTurnPromptAccepts(t *testing.T) {
	root, promptPath, turnDir := promptFixture(t)
	if violation := TurnPrompt(root, promptPath, turnDir); violation != nil {
		t.Fatalf("valid prompt rejected: [%s] %s", violation.Check, violation.Message)
	}
}

func TestTurnPromptRejects(t *testing.T) {
	cases := []struct {
		name      string
		distort   func(prompt string) string
		wantCheck string
	}{
		{"carriage return", func(p string) string { return strings.Replace(p, "Cycle: 3", "Cycle: 3\r", 1) }, "framing"},
		{"header order", func(p string) string {
			return strings.Replace(p, "Mission-Id: m-1\nTurn-Id: t-1", "Turn-Id: t-1\nMission-Id: m-1", 1)
		}, "headers"},
		{"identity mismatch", func(p string) string { return strings.Replace(p, "Turn-Id: t-1", "Turn-Id: t-2", 1) }, "identity"},
		{"preamble drift", func(p string) string { return strings.Replace(p, "Follow the contract.", "Follow the vibes.", 1) }, "preamble"},
		{"missing heading", func(p string) string { return strings.Replace(p, "## Streams", "## Streamz", 1) }, "headings"},
		{"unfenced records", func(p string) string { return strings.Replace(p, "## Open Asks\n<<<DATA>>>\n", "## Open Asks\n", 1) }, "fencing"},
		{"mixed none", func(p string) string {
			return strings.Replace(p, "ask-1\tstream-a\treserved-decision\tmay we drop the flag",
				"ask-1\tstream-a\treserved-decision\tmay we drop the flag\n(none)", 1)
		}, "records"},
		{"unordered ledger", func(p string) string {
			return strings.Replace(p, "1\tcontract-improved\tnone\tgate green\n2\tno-progress",
				"2\tcontract-improved\tnone\tgate green\n1\tno-progress", 1)
		}, "records"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			root, promptPath, turnDir := promptFixture(t)
			if err := os.WriteFile(promptPath, []byte(testCase.distort(validPrompt())), 0o644); err != nil {
				t.Fatal(err)
			}
			violation := TurnPrompt(root, promptPath, turnDir)
			if violation == nil {
				t.Fatal("distorted prompt accepted")
			}
			if violation.Check != testCase.wantCheck {
				t.Fatalf("check = %s (%s), want %s", violation.Check, violation.Message, testCase.wantCheck)
			}
		})
	}
}

func TestTurnPromptLedgerTailBestMarker(t *testing.T) {
	sha := strings.Repeat("a", 40)
	// Records with and without the best token are both legal (the named
	// grammar migration); a fifth field outside yes/no is not.
	root, promptPath, turnDir := promptFixture(t)
	mixed := strings.Replace(validPrompt(),
		"2\tno-progress\t"+sha+"\tno gain",
		"2\tno-progress\t"+sha+"\tno gain\tno\n3\tcontract-improved\t"+sha+"\tnew high\tyes", 1)
	if err := os.WriteFile(promptPath, []byte(mixed), 0o644); err != nil {
		t.Fatal(err)
	}
	if violation := TurnPrompt(root, promptPath, turnDir); violation != nil {
		t.Fatalf("mixed-vintage ledger tail rejected: [%s] %s", violation.Check, violation.Message)
	}

	bad := strings.Replace(validPrompt(),
		"2\tno-progress\t"+sha+"\tno gain",
		"2\tno-progress\t"+sha+"\tno gain\tmaybe", 1)
	if err := os.WriteFile(promptPath, []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	violation := TurnPrompt(root, promptPath, turnDir)
	if violation == nil || violation.Check != "records" || !strings.Contains(violation.Message, "best marker") {
		t.Fatalf("an invalid best marker must be refused: %v", violation)
	}

	oversize := strings.Replace(validPrompt(),
		"2\tno-progress\t"+sha+"\tno gain",
		"2\tno-progress\t"+sha+"\tno gain\tyes\textra", 1)
	if err := os.WriteFile(promptPath, []byte(oversize), 0o644); err != nil {
		t.Fatal(err)
	}
	if violation := TurnPrompt(root, promptPath, turnDir); violation == nil || violation.Check != "records" {
		t.Fatalf("a six-field ledger record must be refused: %v", violation)
	}
}

// The Landed Returns section is the seventh validated heading: data rows and
// all three marker forms (invalid, unreadable, overflow) pass the strict
// three-field grammar, while a malformed row is refused.
func TestTurnPromptLandedReturnsRows(t *testing.T) {
	root, promptPath, turnDir := promptFixture(t)
	rows := strings.Join([]string{
		"chain-a\t2\tartifacts/agents/chain-a/rounds/2/return.json",
		"chain-b\tinvalid\tartifacts/agents/chain-b/rounds/1/return.json",
		"chain-c\tunreadable\tnone",
		"overflow\t3\tnone",
	}, "\n")
	prompt := strings.Replace(validPrompt(),
		"## Landed Returns\n<<<DATA>>>\n(none)\n<<<END>>>",
		"## Landed Returns\n<<<DATA>>>\n"+rows+"\n<<<END>>>", 1)
	if err := os.WriteFile(promptPath, []byte(prompt), 0o644); err != nil {
		t.Fatal(err)
	}
	if violation := TurnPrompt(root, promptPath, turnDir); violation != nil {
		t.Fatalf("landed rows and markers must pass strict validation: [%s] %s", violation.Check, violation.Message)
	}

	bad := strings.Replace(validPrompt(),
		"## Landed Returns\n<<<DATA>>>\n(none)\n<<<END>>>",
		"## Landed Returns\n<<<DATA>>>\nchain-a\t2\n<<<END>>>", 1)
	if err := os.WriteFile(promptPath, []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	if violation := TurnPrompt(root, promptPath, turnDir); violation == nil || violation.Check != "records" {
		t.Fatalf("a two-field landed row must be refused: %+v", violation)
	}

	missing := strings.Replace(validPrompt(),
		"## Landed Returns\n<<<DATA>>>\n(none)\n<<<END>>>\n\n", "", 1)
	if err := os.WriteFile(promptPath, []byte(missing), 0o644); err != nil {
		t.Fatal(err)
	}
	if violation := TurnPrompt(root, promptPath, turnDir); violation == nil || violation.Check != "headings" {
		t.Fatalf("a six-heading prompt must be refused: %+v", violation)
	}
}

func TestTurnPromptRejectsRecordMismatch(t *testing.T) {
	root, promptPath, turnDir := promptFixture(t)
	writeFile(t, filepath.Join(turnDir, "turn.json"), `{"missionId":"m-1","turnId":""}`)
	violation := TurnPrompt(root, promptPath, turnDir)
	if violation == nil || violation.Check != "turn-record" {
		t.Fatalf("empty turnId must fail the turn-record check, got %+v", violation)
	}
}

func TestTurnPromptAcceptsRunnerRaisedAsks(t *testing.T) {
	// The runner itself raises fence and stop-loss asks; a prompt carrying
	// one must validate, or the first fence refusal poisons every later
	// turn into prompt-refused, parking the whole run.
	for _, reason := range []string{"fence", "stop-loss"} {
		root, promptPath, turnDir := promptFixture(t)
		raw, err := os.ReadFile(promptPath)
		if err != nil {
			t.Fatal(err)
		}
		prompt := strings.Replace(string(raw),
			"ask-1\tstream-a\treserved-decision\tmay we drop the flag",
			"ask-1\tstream-a\treserved-decision\tmay we drop the flag\nfence-bound\tnone\t"+reason+"\tchoose whether to amend the contract", 1)
		if err := os.WriteFile(promptPath, []byte(prompt), 0o644); err != nil {
			t.Fatal(err)
		}
		if violation := TurnPrompt(root, promptPath, turnDir); violation != nil {
			t.Fatalf("%s ask rejected: [%s] %s", reason, violation.Check, violation.Message)
		}
	}
}
