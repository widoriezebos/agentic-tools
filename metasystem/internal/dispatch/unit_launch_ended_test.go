package dispatch

import (
	"errors"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

func TestUnitLaunchEndedUsesOnlyReadableTerminalLaunches(t *testing.T) {
	t.Parallel()
	// Regression for ended launches; preservation for unreadable and ordinary jobs.
	for _, state := range []launch.State{launch.Starting, launch.Running, launch.Completed, launch.Failed, launch.Cancelled} {
		t.Run(string(state), func(t *testing.T) {
			t.Parallel()
			id := "ended-projection-" + string(state)
			store := launch.Store{Root: t.TempDir()}
			if err := store.Create(launch.Record{ID: id, State: state}); err != nil {
				t.Fatal(err)
			}
			record := map[string]any{"jobId": id, "unitRun": "run", "operationId": "unit-launch:" + id, "status": "pending"}
			status := func(id string) (string, error) { record, err := store.Read(id); return string(record.State), err }
			if UnitLaunchEnded(record, status) != state.Terminal() {
				t.Fatalf("launch store projection for %s", state)
			}
			if UnitLaunchEnded(record, nil) {
				t.Fatal("missing default-store launch disappeared")
			}
			unreadable := func(string) (string, error) { return "completed", errors.New("unreadable launch") }
			if UnitLaunchEnded(record, unreadable) {
				t.Fatal("unreadable launch disappeared")
			}
			for _, key := range []string{"unitRun", "operationId"} {
				copy := map[string]any{}
				for k, v := range record {
					copy[k] = v
				}
				delete(copy, key)
				if UnitLaunchEnded(copy, func(string) (string, error) { t.Fatal("ordinary job read a unit launch"); return "completed", nil }) {
					t.Fatal("ordinary job disappeared")
				}
			}
			record["status"] = "completed"
			if UnitLaunchEnded(record, func(string) (string, error) { t.Fatal("terminal reservation read a launch"); return "completed", nil }) {
				t.Fatal("terminal reservation was projected again")
			}
		})
	}
}
