package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// The batch tip proof is a delivery run, and a delivery run proves the exact
// index of the root it is given: it refuses a named tree that is not that
// index, and it refuses to move the checkout under a named tree, so the
// launcher owns the positioning. The batch control root deliberately stays on
// the batch base, because the base is what arms the engine that judges, so the
// tip has to reach the run some other way.
//
// Wiring the run at the control root instead made every batch whose units
// changed anything refuse with "delivery candidate must equal the real
// whole-project index", since the tip tree differs from the base tree by
// construction. No seam-level test could see that: the proof's rearm and launch
// seams are both doubled, and the defect lives precisely between them. Only the
// hour-long real-origin rehearsal caught it. This pins the invariant directly.
//
// Parallel although it swaps a package-level seam: batchTipProofExecutable has
// exactly one reader, launchBatchTipProof, and the only other test that calls
// it refuses at the candidate-tree guard before reaching the seam, so no other
// goroutine ever reads what this test writes.
func TestBatchTipProofRunsWhereTheIndexIsTheTipTree(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		command := exec.Command("git", append([]string{"-C", repo}, args...)...)
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, output)
		}
		return strings.TrimSpace(string(output))
	}
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(repo, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git("init", "--quiet", "-b", "main")
	git("config", "user.email", "seat@invalid")
	git("config", "user.name", "Seat")
	write("base.txt", "base\n")
	git("add", "base.txt")
	git("commit", "--quiet", "-m", "base")
	baseTree := git("rev-parse", "HEAD^{tree}")

	write("unit.txt", "a joined unit changed something\n")
	git("add", "unit.txt")
	git("commit", "--quiet", "-m", "land goal-a/u1")
	tip, tipTree := git("rev-parse", "HEAD"), git("rev-parse", "HEAD^{tree}")
	if tipTree == baseTree {
		t.Fatal("fixture needs a tip tree that differs from the base tree")
	}

	// Leave the control root on the base, exactly as rearmBatchBase leaves it.
	git("reset", "--quiet", "--hard", "HEAD~1")
	if staged := git("write-tree"); staged != baseTree {
		t.Fatalf("control root should sit on the batch base, staged %s", staged)
	}

	// The stub stands in for the engine and records the tree it was asked to
	// prove next to the real staged index of the root it was handed.
	observed := filepath.Join(t.TempDir(), "observed.txt")
	stub := filepath.Join(t.TempDir(), "stub-engine")
	script := "#!/bin/sh\n" +
		"root=; tree=; result=\n" +
		"while [ $# -gt 0 ]; do\n" +
		"  case \"$1\" in\n" +
		"    --root) root=$2; shift 2 ;;\n" +
		"    --tree) tree=$2; shift 2 ;;\n" +
		"    --result) result=$2; shift 2 ;;\n" +
		"    *) shift ;;\n" +
		"  esac\n" +
		"done\n" +
		"printf '%s %s\\n' \"$tree\" \"$(git -C \"$root\" write-tree)\" > " + observed + "\n" +
		"printf '{}' > \"$result\"\n"
	if err := testexec.WriteFile(stub, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	previous := batchTipProofExecutable
	batchTipProofExecutable = func() (string, error) { return stub, nil }
	t.Cleanup(func() { batchTipProofExecutable = previous })

	if _, err := launchBatchTipProof(batchProofLaunch{
		Root:         repo,
		BatchID:      "batch-a",
		GoalID:       "goal-a",
		Tree:         tipTree,
		CandidateTip: tip,
		ResultPath:   filepath.Join(t.TempDir(), "result.json"),
		Mode:         testpolicy.ModeStandard,
	}); err != nil {
		t.Fatalf("launch batch tip proof: %v", err)
	}

	recorded, err := os.ReadFile(observed)
	if err != nil {
		t.Fatalf("the launcher never ran the engine: %v", err)
	}
	fields := strings.Fields(string(recorded))
	if len(fields) != 2 {
		t.Fatalf("unreadable observation %q", strings.TrimSpace(string(recorded)))
	}
	if fields[0] != tipTree {
		t.Fatalf("the run was asked to prove %s, want the tip tree %s", fields[0], tipTree)
	}
	if fields[1] != fields[0] {
		t.Fatalf("delivery proves the real index: run root staged %s while proving %s", fields[1], fields[0])
	}

	// The control root is left exactly as it was found, on the base.
	if staged := git("write-tree"); staged != baseTree {
		t.Fatalf("the proof moved the control root: staged %s, want the base %s", staged, baseTree)
	}
}
