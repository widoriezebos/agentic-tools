package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
			if !strings.Contains(out, "check: fast-static-build only; impact would cover 33%") || !strings.Contains(out, "landing group fast-static-build green") || strings.Contains(out, "landing group unit/") {
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
