package plain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"testing"
	"time"
)

func TestFreshProofExecutesSavedGreenRecord(t *testing.T) {
	t.Parallel()
	for _, inherited := range []bool{false, true} {
		t.Run(fmt.Sprintf("inherited=%t", inherited), func(t *testing.T) {
			t.Parallel()
			data, err := os.ReadFile("testdata/fresh-green-result.json")
			if err != nil {
				t.Fatal(err)
			}
			var prior Result
			if err := json.Unmarshal(data, &prior); err != nil {
				t.Fatal(err)
			}
			install := fixtureInstallation(t)
			if err := os.MkdirAll(Dir(install), 0700); err != nil {
				t.Fatal(err)
			}
			if err := appendLine(resultsPath(install), prior); err != nil {
				t.Fatal(err)
			}
			git := stubGit{commit: prior.Commit, tree: prior.Tree}
			if inherited {
				git.commit, git.tree = "cccccccccccccccc", "bbbbbbbbbbbbbbbbbb"
				git.changed = map[[2]string]string{{prior.Tree, git.tree}: "metasystem/plans/goals/fixture.md"}
			}
			now, err := time.Parse(time.RFC3339, prior.At)
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			seams := ProveSeams{Fresh: true, Now: func() time.Time { return now }, Git: git.run, Command: func(command *exec.Cmd) error {
				calls++
				if !slices.Contains(command.Env, "LANDING_PROOF_SCOPE=full") {
					t.Fatalf("fresh proof lost its request or narrowed its scope: %v", command.Env)
				}
				_, err := fmt.Fprint(command.Stdout, "landing environment fixture\nLANDING-CHECKED\t0\n")
				return err
			}}
			if _, ok, err := Settled(install, install, seams); err != nil || ok {
				t.Fatalf("fresh proof accepted saved green: %t %v", ok, err)
			}
			var out bytes.Buffer
			result, err := Run(install, install, "fixture", "", &out, seams)
			if err != nil || result.Result != Green || !result.Fresh || result.Scope != "full" || calls != 1 {
				t.Fatalf("fresh run calls=%d result=%+v err=%v\n%s", calls, result, err, &out)
			}
			records, err := readLines[Result](resultsPath(install))
			if err != nil || len(records) != 2 || records[0].Result != Green || records[1].Result != Green {
				t.Fatalf("fresh must append only its result after the saved green: records=%d err=%v", len(records), err)
			}
			stored, ok, err := LastResult(install)
			if err != nil || !ok || !stored.Fresh || stored.Attempt != result.Attempt {
				t.Fatalf("saved request differs: %+v ok=%t err=%v", stored, ok, err)
			}
		})
	}
}

func TestFreshGateBaselineKeepsRequestOnFailure(t *testing.T) {
	t.Parallel()
	install := fixtureInstallation(t)
	running := Running{Fresh: true, Gate: true, Commit: "single-parent", Tree: "tree", Attempt: "fresh-gate"}
	git := stubGit{commit: running.Commit, tree: running.Tree}
	baseline, ok, _ := gateBaseline(ProveSeams{Git: git.run}, install, install, "fixture", running, &bytes.Buffer{})
	if ok || baseline.Result != Red || !baseline.Fresh {
		t.Fatalf("failed baseline lost its fresh request: ok=%t result=%s fresh=%t reason=%s", ok, baseline.Result, baseline.Fresh, baseline.Reason)
	}
}
