package deploy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAutomaticRecordSettlesPerCommit(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name     string
		outcomes []string
		settled  string
	}{
		{"launched", []string{AutomaticLaunched}, ""},
		{"failed-wins", []string{AutomaticLaunched, RunFailed, AutomaticLaunched, RunBusy}, RunFailed},
		{"unsettled", []string{RunError, RunBusy, RunPaused}, ""},
		{"newest-settling", []string{RunDeployed, RunSkipped}, RunSkipped},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			dir := filepath.Join(t.TempDir(), "record")
			lines, err := AutomaticLines(dir)
			if err != nil || len(lines) != 0 {
				t.Fatalf("missing record = %+v, %v", lines, err)
			}
			at := time.Date(2026, 10, 22, 10, 0, 0, 0, time.UTC)
			for _, outcome := range row.outcomes {
				if err := AppendAutomatic(dir, Automatic{Commit: "a", Outcome: outcome, At: at}); err != nil {
					t.Fatal(err)
				}
			}
			if err := AppendAutomatic(dir, Automatic{Commit: "b", Outcome: RunCurrent, At: at}); err != nil {
				t.Fatal(err)
			}
			lines, err = AutomaticLines(dir)
			if err != nil || len(lines) != len(row.outcomes)+1 {
				t.Fatalf("record = %+v, %v", lines, err)
			}
			states := AutomaticStates(lines)
			state := states["a"]
			got := ""
			if state.Settled != nil {
				got = state.Settled.Outcome
			}
			if got != row.settled || state.Last.Outcome != row.outcomes[len(row.outcomes)-1] || !state.Last.At.Equal(at) || states["b"].Settled.Outcome != RunCurrent {
				t.Fatalf("state = %+v, settled %q; want %q", states, got, row.settled)
			}
		})
	}
	t.Run("damaged", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := automaticPath(dir)
		if err := os.WriteFile(path, []byte("{\n"), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := AutomaticLines(dir); err == nil || !strings.Contains(err.Error(), path+" can't be read") {
			t.Fatalf("damaged record error = %v", err)
		}
	})
}
