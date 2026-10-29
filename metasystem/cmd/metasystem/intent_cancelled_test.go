package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

func cancelledWorkBed(t *testing.T) (*workBed, string) {
	t.Helper()
	bed := newWorkBedWith(t, workApprovedBox)
	bed.starter.fail["proof"] = true
	brief := bed.brief("cancelled.md", "Build this work.\n")
	code, result, _ := bed.work(append([]string{"work", "build", bed.id, "--work", "main", "--brief", brief, "--lines", "5"}, workCheck...)...)
	if code != 1 {
		t.Fatalf("build: %d %+v", code, result)
	}
	run := resultData(t, result)["run"].(string)
	if _, err := (&launch.UnitRunner{Manager: bed.manager, Root: bed.unitRoot, Git: workGit{bed}}).CancelRun(run); err != nil {
		t.Fatal(err)
	}
	bed.starter.fail["proof"] = false
	return bed, run
}

func cancelledCommand(bed *workBed, args ...string) (int, intentResult) {
	owners := bed.workOwners()
	owners.connection.endpointTip = func(string, goal.Endpoint) (string, error) { return "base-commit", nil }
	owners.connection.laneFix = func(string, string, string) *plain.Fix { return nil }
	owners.work.inspectRead = func(string, string, string) (branch.BranchReadResult, error) { return branch.BranchReadResult{}, nil }
	code, result := bed.runJSON(owners, args...)
	bed.recordReadDirs(result)
	return code, result
}

func TestCancelledWorkSaysBuildAnew(t *testing.T) {
	t.Parallel()
	bed, run := cancelledWorkBed(t)
	brief := bed.brief("correction.md", "Correct this work.\n")
	launched := len(bed.starter.launched())
	code, status := cancelledCommand(bed, "work", "status", bed.id)
	encoded, err := json.Marshal(status)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(bed.unitRoot, run, "run.json"))
	if code != 0 || !strings.Contains(string(encoded), "cancelled; new work starts under another name") || strings.Contains(string(encoded), "work wait") {
		t.Fatalf("cancelled status: %d %+v", code, status)
	}
	want := []string{"metasystem", "work", "build", bed.id, "--work", "NAME", "--brief", "FILE", "--check", "COMMAND"}
	if status.Next == nil || !slices.Equal(status.Next.Argv, want) {
		t.Fatalf("cancelled status next: %+v, want %v", status.Next, want)
	}
	for _, args := range [][]string{
		{"work", "revise", bed.id, "--brief", brief, "--reason", "Start over", "--by", "Wido"},
		{"work", "review", bed.id},
		{"work", "revise", "run:" + run, "--brief", brief, "--reason", "Start over", "--by", "Wido"},
	} {
		code, result := cancelledCommand(bed, args...)
		if code == 0 || result.Outcome != intentRefused || !strings.Contains(result.Summary, "cancelled") || result.Next == nil || !slices.Equal(result.Next.Argv, want) {
			t.Fatalf("cancelled %v: %d %+v", args, code, result)
		}
	}
	after, err := os.ReadFile(filepath.Join(bed.unitRoot, run, "run.json"))
	acts, _ := filepath.Glob(filepath.Join(bed.root(), "artifacts", "agents", "channel", "unit-stop-acts", "*.json"))
	if err != nil || string(data) != string(after) || len(acts) != 0 || len(bed.starter.launched()) != launched {
		t.Fatalf("cancelled command wrote an act, changed the run or launched: acts=%v err=%v", acts, err)
	}
}

func TestCancelledWorkDoesNotCompeteWithItsReplacement(t *testing.T) {
	t.Parallel()
	bed, old := cancelledWorkBed(t)
	bed.starter.fail["proof"] = true
	brief := bed.brief("replacement.md", "Build replacement work.\n")
	_, result, _ := bed.work(append([]string{"work", "build", bed.id, "--work", "replacement", "--brief", brief, "--lines", "5"}, workCheck...)...)
	replacement := resultData(t, result)["run"].(string)
	bed.starter.fail["proof"] = false
	code, result := cancelledCommand(bed, "work", "revise", bed.id, "--brief", brief, "--reason", "Correct the failed proof", "--by", "Wido")
	if code != 0 || resultData(t, result)["run"] != replacement || replacement == old {
		t.Fatalf("old cancelled run competed with its replacement: %d %+v", code, result)
	}
}

func TestACancelledRunHidesNoHandSubmission(t *testing.T) {
	t.Parallel()
	bed, _ := cancelledWorkBed(t)
	commit := strings.Repeat("c", 40)
	bed.workOwnersHook = func(owners *intentWorkOwners) {
		read := owners.git
		owners.git = func(dir string, args ...string) ([]byte, error) {
			switch {
			case slices.Equal(args, []string{"rev-parse", "--verify", "HEAD^{commit}"}):
				return []byte(commit), nil
			case len(args) > 0 && args[0] == "rev-list":
				return []byte(commit + " base-commit\n"), nil
			case slices.Equal(args, []string{"show", "-s", "--format=%(trailers:only,unfold=true)", commit}):
				return []byte("Goal-Unit: " + bed.id + "/main\n"), nil
			case len(args) > 0 && args[0] == "diff-tree":
				return []byte(":000000 100644 " + strings.Repeat("0", 40) + " " + strings.Repeat("b", 40) + " A\x00unit.go\x00"), nil
			}
			return read(dir, args...)
		}
	}
	code, result := cancelledCommand(bed, "work", "status", bed.id)
	data, err := json.Marshal(result.Data)
	if err != nil || code != 0 || !strings.Contains(string(data), `"source":"goal branch"`) || !strings.Contains(string(data), `"commit":"`+commit+`"`) {
		t.Fatalf("cancelled run hid manual work: %d %+v %v", code, result, err)
	}
}
