package batchowner

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/cadence"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testrun"
)

// The landing owner runs on the checkout's git toplevel while the goal ledger
// lives in the nested module (…/checkout/metasystem/plans/goals). A cadence
// tick must read the ledger from the module, not the toplevel, or every tick
// refuses CADENCE_LEDGER_UNREADABLE with "the root record is missing" on a
// ledger that is committed and whole. The fixture is a real git repository
// with the module in a subdirectory; the tick is the production cadence.RunTick
// up to its testing preparation, which stops it once the ledger has projected.
func TestBatchOwnerCadenceTickReadsTheLedgerFromTheNestedModule(t *testing.T) {
	t.Parallel()
	top := t.TempDir()
	module := filepath.Join(top, "metasystem")
	if err := os.MkdirAll(filepath.Join(module, "plans", "goals"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(module, "go.mod"), []byte("module example.com/fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	backlog := goal.RenderRoot(&goal.RootRecord{Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1", SyncMode: goal.SyncLocal, Revision: 1})
	if err := os.WriteFile(filepath.Join(module, "plans", "goals", "backlog.md"), backlog, 0o644); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) string {
		t.Helper()
		command := exec.Command("git", append([]string{"-C", top, "-c", "user.name=fixture", "-c", "user.email=fixture@example.com", "-c", "commit.gpgsign=false"}, args...)...)
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
		return strings.TrimSpace(string(output))
	}
	git("init", "-q", "-b", "main")
	git("add", "-A")
	git("commit", "-q", "-m", "fixture ledger")
	commit := git("rev-parse", "HEAD")
	tree := git("rev-parse", "HEAD^{tree}")
	git("update-ref", goal.AcceptedRef, commit)
	git("config", "goal.sync-remote", "local")

	prepared := errors.New("fixture stops at the testing preparation")
	tick := func(root string) (string, error) {
		var preparedRoot string
		_, err := cadence.RunTick(root, cadence.Owner{
			FetchOrigin: func(string) (string, string, error) { return commit, tree, nil },
			Prepare: func(request testrun.SelectionRequest) (testrun.Preparation, error) {
				preparedRoot = request.Root
				return testrun.Preparation{}, prepared
			},
		}, time.Now)
		return preparedRoot, err
	}
	if _, err := tick(top); err == nil || !strings.Contains(err.Error(), "the root record is missing") {
		t.Fatalf("a tick on the toplevel = %v; want the refusal the lane owner reported", err)
	}
	preparedRoot, err := tick(BatchOwnerCadenceRoot(top))
	if err == nil || !strings.Contains(err.Error(), prepared.Error()) {
		t.Fatalf("cadence tick on the owner's cadence root = %v; want the ledger to project from the module and the tick to reach its preparation", err)
	}
	if preparedRoot != module {
		t.Fatalf("cadence preparation root = %q, want the module %q", preparedRoot, module)
	}
}
