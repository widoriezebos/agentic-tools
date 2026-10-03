package validate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
)

// The mission-runner scenario checked the runner's assembled prompt against
// the SHIPPED orchestrator preamble and ran a contract carrying a stray
// "## Streams" heading into the prompt checker. These drive the real
// assembler (mission.AssemblePrompt) into the checker the runner runs
// before every launch.

func u6bportAssembledPrompt(t *testing.T, intent string) (root, promptPath, turnDir string) {
	t.Helper()
	root = t.TempDir()
	write := func(rel, content string) {
		t.Helper()
		writeFile(t, filepath.Join(root, rel), content)
	}
	write("metasystem.conf", "metasystem.runtimes=fake\n")
	write("plans/mission-m1.contract.md", "# Intent\n\nAdvance the candidate.\n"+intent+
		"\n# Non-goals\n\nDo not deploy.\n\n```mission\nfence.cycles=5\nfence.jobs=4\nfence.concurrency=1\nstream.primary=Advance.\n```\n")
	base := "artifacts/agents/missions/m1"
	write(base+"/ledger.md", "# Mission Ledger\n\n- Cycle budget: 5\n- No-gain budget: 3\n")
	write(base+"/fences.json", `{"cycles":1,"reservations":{}}`)
	write(base+"/state.json", `{"missionId":"m1","streams":{"primary":{"state":"active","goal":"Advance.","reason":null}},"turnLog":[]}`)
	turnDir = filepath.Join(root, base, "turns", "m1-t1-abcd")
	writeFile(t, filepath.Join(turnDir, "turn.json"),
		`{"missionId":"m1","turnId":"m1-t1-abcd","cycle":1,"runtime":"fake","model":"fake-model","reconciliation":false,"hostSession":null}`)
	promptPath = filepath.Join(turnDir, "prompt.md")
	if err := mission.AssemblePrompt(root, root, "m1", "m1-t1-abcd", promptPath); err != nil {
		t.Fatalf("assemble: %v", err)
	}
	return root, promptPath, turnDir
}

// runner-cycle: the assembled prompt opens with the machine header, then the
// shipped preamble byte-for-byte, and passes the checker.
func TestU6bPortAssembledPromptCarriesTheShippedPreamble(t *testing.T) {
	t.Parallel()
	root, promptPath, turnDir := u6bportAssembledPrompt(t, "")
	if violation := TurnPrompt(root, promptPath, turnDir); violation != nil {
		t.Fatalf("the assembled prompt fails the checker: [%s] %s", violation.Check, violation.Message)
	}
}

// runner-bad-prompt: a contract whose prose carries a "## Streams" line
// duplicates a prompt heading, and the checker refuses the launch on the
// headings check.
func TestU6bPortContractHeadingIsRefusedByThePromptChecker(t *testing.T) {
	t.Parallel()
	root, promptPath, turnDir := u6bportAssembledPrompt(t, "## Streams\n")
	violation := TurnPrompt(root, promptPath, turnDir)
	if violation == nil || violation.Check != "headings" {
		t.Fatalf("a contract heading must be refused on headings: %+v", violation)
	}
}

// The preamble cases of host_prompt_opens_with_preamble: a changed first
// byte and a prompt that ends inside the preamble are both refused.
func TestU6bPortPreambleChangedOrTruncatedIsRefused(t *testing.T) {
	t.Parallel()
	valid := validPrompt()
	headerEnd := strings.Index(valid, "\n\n") + 2
	cases := map[string]string{
		"changed first byte": valid[:headerEnd] + "!" + valid[headerEnd+1:],
		"truncated":          valid[:headerEnd+len(testPreamble)/2],
	}
	for name, prompt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root, promptPath, turnDir := promptFixture(t)
			if !strings.HasSuffix(prompt, "\n") {
				prompt += "\n"
			}
			if err := os.WriteFile(promptPath, []byte(prompt), 0o644); err != nil {
				t.Fatal(err)
			}
			violation := TurnPrompt(root, promptPath, turnDir)
			if violation == nil || violation.Check != "preamble" {
				t.Fatalf("%s preamble must be refused on preamble: %+v", name, violation)
			}
		})
	}
}
