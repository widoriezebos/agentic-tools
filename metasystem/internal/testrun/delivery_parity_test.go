package testrun

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// A seat's checkout always holds bytes no commit records: the engine's
// per-run state under the installation's artifacts, its local configuration,
// built outputs under bin, and records a hook appends to after staging. A group that
// declares the whole installation (metasystem/**) as its input must not turn
// those into drift: the delivery-parity snapshot leaves exactly what the
// commit boundary's LANDING projection leaves out at candidate bytes, so a
// hand-made change is judged on the candidate it will commit. A declared
// input the commit would record still refuses.
func TestDeliveryParityLeavesWhatTheCommitNeverRecordsAtCandidateBytes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write := func(relative, text string) {
		writeTestingFixtureFile(t, filepath.Join(root, filepath.FromSlash(relative)), []byte(text), 0o644)
	}
	testingFixtureGit(t, root, "init", "-q")
	write(".gitignore", "metasystem/metasystem.conf.local\n")
	write("metasystem/.gitignore", "artifacts/\nbin/\n/metasystem\n")
	write("metasystem/metasystem.conf", "testing.contract=testing.json\n")
	write("metasystem/testing.json", "{}\n")
	write("metasystem/cmd/tool/main.go", "package main\n")
	write("metasystem/memory/receipts.log", "one\n")
	write("metasystem/records/narrator-digest.log", "one\n")
	testingFixtureGit(t, root, "add", "-A")
	testingFixtureGit(t, root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "-m", "base")

	// The hand-made change: two staged records.
	write("metasystem/memory/receipts.log", "one\ntwo\n")
	write("metasystem/records/narrator-digest.log", "one\ntwo\n")
	testingFixtureGit(t, root, "add", "metasystem/memory/receipts.log", "metasystem/records/narrator-digest.log")
	candidate := strings.TrimSpace(testingFixtureGit(t, root, "write-tree"))

	// What the seat holds that no commit records, including this very
	// landing's own output log, which is new on every attempt.
	write("metasystem/artifacts/agents/output/land-attempt.log", "attempt 1\n")
	write("metasystem/metasystem.conf.local", "launch.role=local\n")
	write("metasystem/bin/tool", "built\n")
	// A hook appended to a record after it was staged.
	write("metasystem/records/narrator-digest.log", "one\ntwo\nthree\n")

	contract := testpolicy.Contract{Groups: []testpolicy.Group{{ID: "whole-installation", Inputs: []string{"metasystem/**"}}}}
	plan := testpolicy.Plan{SelectedGroups: []string{"whole-installation"}}
	workspace := gittree.Workspace{Dir: root}
	check := func() error {
		return CheckDeliveryInputParity(candidate, "metasystem/", "testing.json", contract, plan, deliveryParitySnapshot(workspace, "metasystem/"))
	}
	if err := check(); err != nil {
		t.Fatalf("bytes no commit records moved the delivery candidate: %v", err)
	}
	write("metasystem/artifacts/agents/output/land-attempt.log", "attempt 2\n")
	if err := check(); err != nil {
		t.Fatalf("a second attempt's own log moved the delivery candidate: %v", err)
	}

	// An unstaged edit to a declared input the commit would record refuses.
	write("metasystem/cmd/tool/main.go", "package main\n\nfunc main() {}\n")
	if err := check(); err == nil || !strings.Contains(err.Error(), "delivery candidate differs from relevant working-tree inputs") {
		t.Fatalf("unstaged declared input did not refuse: %v", err)
	}
	testingFixtureGit(t, root, "checkout", "--", "metasystem/cmd/tool/main.go")
	// So does an ignored declared input inside the projection: it is not
	// in the candidate, and the commit boundary would refuse it too.
	write("metasystem/.gitignore", "artifacts/\nbin/\n/metasystem\ncmd/tool/generated.go\n")
	testingFixtureGit(t, root, "add", "metasystem/.gitignore")
	candidate = strings.TrimSpace(testingFixtureGit(t, root, "write-tree"))
	if err := check(); err != nil {
		t.Fatalf("restaged candidate: %v", err)
	}
	write("metasystem/cmd/tool/generated.go", "package main\n")
	if err := check(); err == nil || !strings.Contains(err.Error(), "delivery candidate differs from relevant working-tree inputs") {
		t.Fatalf("ignored declared input inside the landing projection did not refuse: %v", err)
	}
}
