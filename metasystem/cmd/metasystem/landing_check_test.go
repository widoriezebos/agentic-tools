package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/conflict"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestLandingResolveCheckCostAndCollectionReuse(t *testing.T) {
	t.Parallel()
	root, _ := impactGitAdapterBed(t)
	data, err := os.ReadFile(filepath.Join(root, "testing.json"))
	if err != nil {
		t.Fatal(err)
	}
	var contract testpolicy.Contract
	if err := json.Unmarshal(data, &contract); err != nil {
		t.Fatal(err)
	}
	count := filepath.Join(t.TempDir(), "checks")
	group := contract.Groups[0]
	group.ID = "fast-static-build"
	group.Kind = "static"
	group.Inputs = []string{"cmd/**"}
	group.Argv = []string{"/bin/sh", "-c", fmt.Sprintf("printf checked >> %s", shellCommand([]string{count}))}
	contract.Groups = append(contract.Groups, group)
	data, err = json.Marshal(contract)
	if err != nil {
		t.Fatal(err)
	}
	impactWrite(t, root, "testing.json", string(data))
	impactWrite(t, root, "metasystem.conf", "testing.contract=testing.json\nlanding.impact-max-share=30\n")
	impactWrite(t, root, ".gitignore", "artifacts/\n")
	impactWrite(t, root, "cmd/metasystem/main_test.go", "package main\nimport \"testing\"\nfunc TestMainBehavior(t *testing.T) { t.Parallel() }\n")
	testingFixtureGit(t, root, "add", ".")
	testingFixtureGit(t, root, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "check contract")
	base := strings.TrimSpace(testingFixtureGit(t, root, "rev-parse", "HEAD"))
	impactWrite(t, root, "internal/launch/value.go", "package launch\nconst Value = false\n")
	impactWrite(t, root, "cmd/metasystem/main.go", "package main\nfunc main() { _ = 1 }\n")
	testingFixtureGit(t, root, "add", ".")
	run := func(wantCount int, reused bool) {
		t.Helper()
		code, out, problem := impactPublic(t, root, "--base", base, "--check")
		if code != 0 {
			t.Fatalf("check exit=%d out=%s problem=%s", code, out, problem)
		}
		got, err := os.ReadFile(count)
		if err != nil || string(got) != strings.Repeat("checked", wantCount) {
			t.Fatalf("checks=%s err=%v", got, err)
		}
		if reused {
			if !strings.Contains(out, "check reused from the job") || strings.Contains(out, "landing group") {
				t.Fatalf("collection ran a check: %s", out)
			}
		} else {
			if !strings.Contains(out, "check: fast-static-build only; impact would cover 50% of 4 packages") || !strings.Contains(out, "landing group fast-static-build green") || strings.Contains(out, "landing group unit/") {
				t.Fatalf("expensive check: %s", out)
			}
		}
		t.Log(out)
	}
	run(1, false)
	run(1, true)
	impactWrite(t, root, "cmd/metasystem/main.go", "package main\nfunc main() { _ = 2 }\n")
	testingFixtureGit(t, root, "add", ".")
	run(2, false)
	// A different comparison base cannot borrow the preceding job's green.
	identity, err := repairCheckIdentity(root, base)
	if err != nil {
		t.Fatal(err)
	}
	identity.Base = "another-base"
	if reused, _ := repairCheckCache(root, identity, false); reused {
		t.Fatal("reused another base")
	}
	record, err := os.ReadFile(filepath.Join(plain.Dir(root), "repair-check.json"))
	if err != nil || !strings.Contains(string(record), "fast-static-build only") {
		t.Fatalf("reason=%s err=%v", record, err)
	}
}

// These adapter checks prove persistence against the real Git index while the check runs.
func TestLandingCheckSaveWorkingTreeGitAdapter(t *testing.T) {
	t.Parallel()
	for _, changeIndex := range []bool{false, true} {
		t.Run(fmt.Sprintf("staged tree changes %v", changeIndex), func(t *testing.T) {
			t.Parallel()
			root, base := impactGitAdapterBed(t)
			data, err := os.ReadFile(filepath.Join(root, "testing.json"))
			if err != nil {
				t.Fatal(err)
			}
			var contract testpolicy.Contract
			if err := json.Unmarshal(data, &contract); err != nil {
				t.Fatal(err)
			}
			group := contract.Groups[0]
			group.ID, group.Kind, group.Inputs = "fast-static-build", "static", []string{"cmd/**"}
			if changeIndex {
				group.Argv = []string{"git", "add", "stage-me.txt"}
			}
			contract.Groups = append(contract.Groups, group)
			data, err = json.Marshal(contract)
			if err != nil {
				t.Fatal(err)
			}
			impactWrite(t, root, "testing.json", string(data))
			impactWrite(t, root, ".gitignore", "artifacts/\n")
			impactWrite(t, root, "cmd/metasystem/main_test.go", "package main\nimport \"testing\"\nfunc TestBehavior(t *testing.T) { t.Parallel() }\n")
			testingFixtureGit(t, root, "add", ".")
			impactWrite(t, root, "internal/launch/value.go", "package launch\nconst Value = false\n")
			impactWrite(t, root, "stage-me.txt", "unstaged\n")
			code, out, problem := impactPublic(t, root, "--base", base, "--check")
			if code != 0 || !strings.Contains(out, "wall time") {
				t.Fatalf("exit=%d out=%s error=%s", code, out, problem)
			}
			record, err := os.ReadFile(filepath.Join(plain.Dir(root), "repair-check.json"))
			if changeIndex {
				if !os.IsNotExist(err) || !strings.Contains(out, "staged tree changed during the check") {
					t.Fatalf("record=%s err=%v out=%s", record, err, out)
				}
			} else {
				var check repairCheck
				if err != nil || json.Unmarshal(record, &check) != nil || check.Tree == "" || check.Base != base || check.DurationMS <= 0 {
					t.Fatalf("record=%s err=%v", record, err)
				}
				code, out, problem = impactPublic(t, root, "--base", base, "--check")
				if code != 0 || !strings.Contains(out, "check reused from the job") {
					t.Fatalf("reuse=%d out=%s err=%s", code, out, problem)
				}
			}
			t.Log(out)
		})
	}
}

func TestLandingCheckFixRecordsEveryOutcomeGitAdapter(t *testing.T) {
	t.Parallel()
	for _, outcome := range []string{"red", "green", "unsaved"} {
		t.Run(outcome, func(t *testing.T) {
			t.Parallel()
			root, base := impactGitAdapterBed(t)
			data, err := os.ReadFile(filepath.Join(root, "testing.json"))
			if err != nil {
				t.Fatal(err)
			}
			var contract testpolicy.Contract
			if err := json.Unmarshal(data, &contract); err != nil {
				t.Fatal(err)
			}
			group := contract.Groups[0]
			group.ID, group.Kind, group.Inputs = "fast-static-build", "static", []string{"cmd/**"}
			if outcome == "green" {
				group.Argv = []string{"/bin/sh", "-c", "sleep 2"}
			}
			if outcome == "red" {
				group.Argv = []string{"/bin/sh", "-c", "exit 1"}
			}
			if outcome == "unsaved" {
				group.Argv = []string{"git", "add", "stage-me.txt"}
			}
			contract.Groups = append(contract.Groups, group)
			data, err = json.Marshal(contract)
			if err != nil {
				t.Fatal(err)
			}
			impactWrite(t, root, "testing.json", string(data))
			impactWrite(t, root, ".gitignore", "artifacts/\n")
			impactWrite(t, root, "cmd/metasystem/main_test.go", "package main\nimport \"testing\"\nfunc TestBehavior(t *testing.T) { t.Parallel() }\n")
			testingFixtureGit(t, root, "add", ".")
			impactWrite(t, root, "stage-me.txt", "unstaged\n")
			fix := &plain.Fix{Goal: "goal", Units: []string{"lane-merge-1"}, Commit: base, Tip: "tip", State: "resolving", Attempt: "attempt", Paths: []conflict.Path{{Path: "code.go", Class: conflict.Builder}}}
			if err := plain.WriteFix(root, fix); err != nil {
				t.Fatal(err)
			}
			code, out, problem := impactPublic(t, root, "--base", base, "--check")
			wantCode, wantResult := 0, "green"
			if outcome == "red" {
				wantCode, wantResult = 1, "red"
			}
			if code != wantCode {
				t.Fatalf("exit=%d out=%s problem=%s", code, out, problem)
			}
			record, err := os.ReadFile(filepath.Join(plain.Dir(root), "fixes", "attempt.json"))
			if err != nil {
				t.Fatal(err)
			}
			var fields struct {
				Minutes float64 `json:"check_minutes"`
				Result  string  `json:"check_result"`
			}
			if err := json.Unmarshal(record, &fields); err != nil {
				t.Fatal(err)
			}
			if fields.Minutes <= 0 || fields.Result != wantResult {
				t.Fatalf("last check missing from fix: %s", record)
			}
			if outcome == "green" {
				if fields.Minutes < 2.0/60 {
					t.Fatalf("real check duration missing: %s", record)
				}
				t.Logf("fix record after real check: %s", record)
				for reuse := 1; reuse <= 2; reuse++ {
					code, out, problem := impactPublic(t, root, "--base", base, "--check")
					if code != 0 || !strings.Contains(out, "check reused from the job") || strings.Contains(out, "landing group") {
						t.Fatalf("reuse %d: exit=%d out=%s problem=%s", reuse, code, out, problem)
					}
					after, err := os.ReadFile(filepath.Join(plain.Dir(root), "fixes", "attempt.json"))
					if err != nil || !bytes.Equal(record, after) {
						t.Fatalf("reuse %d changed the real check record: before=%s after=%s err=%v", reuse, record, after, err)
					}
					t.Logf("fix record after reuse %d: %s", reuse, after)
				}
			}
			_, cacheErr := os.Stat(filepath.Join(plain.Dir(root), "repair-check.json"))
			if outcome != "green" && !os.IsNotExist(cacheErr) {
				t.Fatalf("failed or unsaved check cached: %v", cacheErr)
			}
			if outcome == "unsaved" && !strings.Contains(out, "staged tree changed during the check") {
				t.Fatalf("missing save reason: %s", out)
			}
			t.Logf("%s check: %.6f minutes, result %s", outcome, fields.Minutes, fields.Result)
		})
	}
}
