package delegation_test

// Port of dispatch-fixtures.sh cluster c (lines 3301-3867): the lifecycle
// orchestration that needs no Git — the critique mutation callbacks'
// authority and the implementation close's evidence reconciliation.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegation"
)

// 3672-3697: a delegate-shaped caller reaches neither internal critique
// mutation. The refusal is the owner's holder-only verdict for the named
// chain root, raised before the register is read or the admission result
// written; an already-folded retry would otherwise have returned unchanged.
func TestPortP5CritiqueMutationCallbacksRefuseADelegateCaller(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.writeRecord("flag-runtime", map[string]any{"status": "completed", "role": "code-critic", "round": 1, "parentJob": nil})
	before, err := os.ReadFile(b.recordPath("flag-runtime"))
	if err != nil {
		t.Fatal(err)
	}
	var authorized []string
	b.doubles.Lease.AuthorizeFunc = func(_ delegation.Invocation, mode delegation.AuthorityMode, job string) error {
		authorized = append(authorized, string(mode)+" "+job)
		return errors.New("control-plane write requires the authenticated lease holder")
	}
	subject := b.writeFile("subject.json", `{"kind":"live"}`)
	result := filepath.Join(b.root, "admission.json")
	for _, argv := range [][]string{
		{"__critique-register-advance", "--root-job", "flag-runtime", "--round-job", "flag-runtime-r3"},
		{"__critique-read-admission", "--root-job", "flag-runtime", "--role", "code-critic", "--round", "4", "--subject-file", subject, "--result", result},
	} {
		authorized = nil
		outcome := b.run(argv...)
		if outcome.ExitCode == 0 || !strings.Contains(b.stderr.String(), "control-plane write requires the authenticated lease holder") {
			t.Fatalf("%s: exit %d stderr %q", argv[0], outcome.ExitCode, b.stderr.String())
		}
		if len(authorized) != 1 || authorized[0] != "holder-only flag-runtime" {
			t.Fatalf("%s authorized %v, want one holder-only check for the chain root", argv[0], authorized)
		}
		if len(outcome.Stdout) != 0 {
			t.Fatalf("%s reached its owner: stdout %q", argv[0], outcome.Stdout)
		}
	}
	if exists(result) {
		t.Fatal("a refused read admission wrote its result")
	}
	if after, _ := os.ReadFile(b.recordPath("flag-runtime")); string(after) != string(before) {
		t.Fatal("a refused critique mutation changed the chain root")
	}
}

// follow_up_read_closes_terminal_work (3544-3634), lifecycle half: close
// --reconcile-evidence runs the review-reference reconciliation for the
// chain root under the lease and holder-only authority before anything of
// the close; a refused reconciliation leaves the chain open and releases
// its lock, and a malformed evidence id is usage before the lease.
func TestPortP5CloseReconcilesReviewEvidenceBeforeClosing(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.writeRecord("work", map[string]any{"status": "completed", "round": 1, "parentJob": nil})

	result := b.run("close", "--job", "work", "--reconcile-evidence", "Bad_Id")
	requireExit(t, result, 2, b.stderr.String())
	if !strings.Contains(b.stderr.String(), "invalid review evidence job id: Bad_Id") || len(b.calls("lease.Held")) != 0 {
		t.Fatalf("stderr %q calls %v", b.stderr.String(), b.doubles.Log.Calls())
	}

	result = b.run("close", "--job", "work", "--reconcile-evidence", "work", "--reconcile-evidence", "other")
	requireExit(t, result, 2, b.stderr.String())

	result = b.run("close", "--job", "work", "--reconcile-evidence", "absent-critic")
	if result.ExitCode != 1 {
		t.Fatal("a reconciliation against an absent critic let the close proceed")
	}
	if !strings.Contains(b.stderr.String(), "review evidence job absent-critic is unreadable") {
		t.Fatalf("the refusal is not the reconciliation's: %q", b.stderr.String())
	}
	if record := b.record("work"); record["chainClosed"] == true || record["independentCritiqueJobRef"] != nil {
		t.Fatalf("a refused reconciliation touched the chain: %v", record)
	}
	authorize := b.calls("lease.Authorize")
	if len(authorize) == 0 || !strings.Contains(authorize[0], "mode=holder-only job=work") {
		t.Fatalf("the reconciliation ran without holder-only authority for the root: %v", authorize)
	}
	held := b.calls("lease.Held")
	if len(held) != 1 {
		t.Fatalf("the reconciliation ran outside the lease: %v", b.doubles.Log.Calls())
	}
	if exists(filepath.Join(b.root, "artifacts", "agents", "locks", "work.d")) {
		t.Fatal("the refused close kept the chain lock")
	}
}
