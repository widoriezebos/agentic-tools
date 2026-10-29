package stoptransition

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJobStopReportsAnEndedUnitLaunch(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, suffix string
		unitLaunch   bool
	}{
		{"unit-launch", ": already ended completed; its reservation now carries the launch's outcome", true},
		{"ordinary-job", ": cancelled (cancel path, TERM)", false},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			path := filepath.Join(root, "artifacts/agents/jobs/unit-reader.json")
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			record := map[string]any{"jobId": "unit-reader", "status": "pending"}
			if row.unitLaunch {
				record["unitRun"], record["operationId"] = "unit", "unit-launch:unit-reader"
			}
			writeRecord := func() error {
				data, err := json.Marshal(record)
				if err != nil {
					return err
				}
				return os.WriteFile(path, data, 0o644)
			}
			if err := writeRecord(); err != nil {
				t.Fatal(err)
			}
			family := newJobFamily(LocalConfig{Root: root, CancelJob: func(id string) (string, error) {
				if id != "unit-reader" {
					t.Fatalf("cancel received %s", id)
				}
				record["status"] = "completed"
				if row.unitLaunch {
					record["unitExecuted"] = true
				}
				return "", writeRecord()
			}})
			items, err := family.Inventory()
			if err != nil || len(items) != 1 {
				t.Fatalf("inventory = %+v, %v", items, err)
			}
			outcome, err := family.Stop(items[0])
			if err != nil || !outcome.Complete || !strings.HasSuffix(outcome.Line, row.suffix) || (!row.unitLaunch && strings.Contains(outcome.Line, "already ended")) {
				t.Fatalf("stop = %+v, %v", outcome, err)
			}
		})
	}
}
