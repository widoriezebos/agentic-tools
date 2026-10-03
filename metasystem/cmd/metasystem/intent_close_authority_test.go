package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/authority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

// humanRecordWriter is the record-writer authority owner's answer for a
// person's classification, the one the fixture close wrappers present.
func humanRecordWriter(_, job string) (string, error) {
	if err := authority.Authorize("record-writer", map[string]any{"class": "HUMAN"}, job); err != nil {
		return "record-writer-refused", err
	}
	return "", nil
}

// TestIntentCloseRecordWriterPreflight: the preflight asks the real
// record-writer owner. This process, classified for real once announced as
// the lease holder, is admitted; an agent session that does not hold the
// work is refused, and a public close then starts nothing and offers no
// repair. The refused caller is named rather than classified, so the test
// means the same from a person's terminal, which the owner admits.
func TestIntentCloseRecordWriterPreflight(t *testing.T) {
	t.Parallel()
	refused := newDeliveryBed(t)
	notHolding := func(_, job string) (string, error) {
		return recordWriterAdmits(lease.ClassifyResult{Class: lease.ClassMain}, nil, job)
	}
	if cause, err := notHolding(refused.install, "crit1"); err == nil || cause != "record-writer-refused" {
		t.Fatalf("a caller without authority: %q %v", cause, err)
	}
	refused.owners.recordWriter = notHolding
	refused.writeJob(map[string]any{"jobId": "crit1", "role": "code-critic", "status": "completed", "round": 1, "findingRegister": []any{}})
	refused.writeReturn("crit1", 1, "crit1")
	decisions := refused.root() + "/decisions.md"
	refused.writeFile(decisions, deliveryDispositionsHeader)
	code, result := refused.do("work", "finish", "j2:crit1", "--dispositions", decisions)
	data, _ := result.Data.(map[string]any)
	if code == 0 || result.Outcome != intentRefused || len(refused.calls) != 0 || data["cause"] == nil || result.Next != nil {
		t.Fatalf("a refused preflight: %d %+v calls=%v", code, result, refused.calls)
	}

	admitted := newDeliveryBed(t)
	announceProofFixtureHolder(t, admitted.install)
	if cause, err := recordWriterPreflight(admitted.install, "crit1"); err != nil {
		t.Fatalf("the authenticated lease holder: %q %v", cause, err)
	}

	// The same session closing from its unarmed goal worktree, where nothing
	// is announced, is still the holder: its lease is the primary checkout's.
	git := func(args ...string) {
		t.Helper()
		command := exec.Command("git", append([]string{"-C", admitted.root(), "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid"}, args...)...)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
	git("commit", "--allow-empty", "-q", "-m", "base")
	worktree := filepath.Join(t.TempDir(), "goal-worktree")
	git("worktree", "add", "-q", "-b", "goal/crit", worktree)
	top, err := filepath.EvalSymlinks(admitted.root())
	if err != nil {
		t.Fatal(err)
	}
	install, err := filepath.EvalSymlinks(admitted.install)
	if err != nil {
		t.Fatal(err)
	}
	relative, err := filepath.Rel(top, install)
	if err != nil {
		t.Fatal(err)
	}
	worktreeInstall := filepath.Join(worktree, relative)
	if err := os.MkdirAll(worktreeInstall, 0o755); err != nil {
		t.Fatal(err)
	}
	if cause, err := recordWriterPreflight(worktreeInstall, "crit1"); err != nil {
		t.Fatalf("the lease holder in its unarmed goal worktree: %q %v", cause, err)
	}
}
