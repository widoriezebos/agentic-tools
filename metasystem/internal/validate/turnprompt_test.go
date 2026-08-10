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
		"## Open Asks",
		"<<<DATA>>>",
		"ask-1\tstream-a\treserved-decision\tmay we drop the flag",
		"<<<END>>>",
		"",
		"## Streams",
		"<<<DATA>>>",
		"stream-a\tactive\tship it\ton track",
		"stream-b\tdone\tship more\tfinished",
		"<<<END>>>",
		"",
		"## Reconciliation",
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

func TestTurnPromptRejectsRecordMismatch(t *testing.T) {
	root, promptPath, turnDir := promptFixture(t)
	writeFile(t, filepath.Join(turnDir, "turn.json"), `{"missionId":"m-1","turnId":""}`)
	violation := TurnPrompt(root, promptPath, turnDir)
	if violation == nil || violation.Check != "turn-record" {
		t.Fatalf("empty turnId must fail the turn-record check, got %+v", violation)
	}
}
