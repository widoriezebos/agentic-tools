package dispatch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestExaminationRetryAdmissible: only a terminal critic round that ended
// without a return is admitted; live, completed, cancelled and returned
// rounds, and non-critic rounds, are refused with the reason.
func TestExaminationRetryAdmissible(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	round := func(status, errorName string) map[string]any {
		return map[string]any{"jobId": "crit", "role": "code-critic", "round": 1, "status": status, "error": errorName}
	}
	for _, refused := range []struct {
		record map[string]any
		want   string
	}{
		{round("running", ""), "still running"},
		{round("completed", ""), "completed"},
		{round("cancelled", ""), "cancelled"},
		{map[string]any{"jobId": "impl", "role": "implementer", "round": 1, "status": "failed"}, "not a implementer round"},
	} {
		if err := ExaminationRetryAdmissible(root, refused.record); err == nil || !strings.Contains(err.Error(), refused.want) {
			t.Errorf("%v = %v, want %q", refused.record, err, refused.want)
		}
	}
	for _, admitted := range []map[string]any{round("failed", "process-lost"), round("timeout", "")} {
		if err := ExaminationRetryAdmissible(root, admitted); err != nil {
			t.Errorf("%v refused: %v", admitted, err)
		}
	}
	// A recorded process that cannot be proven dead is never retried.
	unproven := round("failed", "process-lost")
	unproven["pid"] = float64(os.Getpid())
	if err := ExaminationRetryAdmissible(root, unproven); err == nil || !strings.Contains(err.Error(), "not proven stopped") {
		t.Errorf("a live or unprovable process = %v", err)
	}
	// The reaper's recorded group-death proof is the owner's proof.
	proven := round("timeout", "budget-cap")
	proven["pid"], proven["groupDeathProvenAt"] = float64(os.Getpid()), "2026-09-26T09:00:00Z"
	if err := ExaminationRetryAdmissible(root, proven); err != nil {
		t.Errorf("a round whose group death the reaper proved: %v", err)
	}
	returned := filepath.Join(root, "artifacts", "agents", "crit", "rounds", "1", "return.json")
	os.MkdirAll(filepath.Dir(returned), 0o700)
	os.WriteFile(returned, []byte(`{"findings":[]}`), 0o600)
	if err := ExaminationRetryAdmissible(root, round("failed", "process-lost")); err == nil || !strings.Contains(err.Error(), "wrote a return") {
		t.Errorf("a round with a return = %v", err)
	}
}
