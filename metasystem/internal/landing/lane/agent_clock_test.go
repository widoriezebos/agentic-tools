package lane

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestKeeperClockOwnsPreparationAndHonoursAgentFences(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"fallback", "running", "paused", "explicit", "other checkout", "clock unreadable"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			home, checkout := t.TempDir(), t.TempDir()
			module := filepath.Join(checkout, "metasystem")
			for _, dir := range []string{module, HostDir(home)} {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
			}
			data, err := json.Marshal(Record{Root: checkout, Install: module, CustodyEpoch: 1})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(RecordPath(home), data, 0600); err != nil {
				t.Fatal(err)
			}
			now, reasons := laneNow, []string{queuedReason}
			agent := &fakeAgent{}
			keeper := agent.keeper(home, module, &now, wakeFor(&reasons))
			clocks, preparations := 0, 0
			keeper.Prepare = func(Record) error { preparations++; return errors.New("selection needs a person") }
			keeper.Clock = func(record Record, prepare func(Record) error) (string, error) {
				clocks++
				if record.Root != checkout {
					t.Fatalf("clock at %s", record.Root)
				}
				_ = prepare(record)
				if mode == "clock unreadable" {
					return "", errors.New("clock cannot be read")
				}
				return "clock due; proving main", nil
			}
			switch mode {
			case "running":
				agent.running = "agent"
			case "paused":
				if _, err := SetPause(home, "Wido", now); err != nil {
					t.Fatal(err)
				}
			case "explicit":
				keeper.Explicit = true
			case "other checkout":
				keeper.Self = t.TempDir()
			}
			result := keeper.Run()
			wanted := mode == "fallback" || mode == "clock unreadable"
			if (clocks == 1) != wanted || len(agent.starts) != 0 {
				t.Fatalf("clock fence: %+v clocks=%d starts=%d", result, clocks, len(agent.starts))
			}
			if wanted && preparations != 1 {
				t.Fatal("clock did not own the batch attempt")
			}
			if mode == "fallback" && !strings.Contains(result.Line, "proving main") {
				t.Fatalf("preparation refusal hid fallback: %+v", result)
			}
			if mode == "clock unreadable" && !strings.Contains(result.Line, "clock cannot be read") {
				t.Fatalf("clock error hidden: %+v", result)
			}
		})
	}
}
