package launch

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type countedTranscript struct {
	ClaudeHeadless
	reads       map[string]int
	inventory   *string
	inventories int
}

func (a *countedTranscript) measureTranscript(session string, measurement *Measurement, files func(string) ([]string, error)) error {
	a.reads[session]++
	return a.ClaudeHeadless.measureTranscript(session, measurement, func(root string) ([]string, error) {
		paths, err := files(root)
		if len(paths) > 0 && a.inventory != &paths[0] {
			a.inventory = &paths[0]
			a.inventories++
		}
		return paths, err
	})
}

func TestCapacityUsageReadsTranscriptsOnce(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	adapter := &countedTranscript{ClaudeHeadless: ClaudeHeadless{ProjectsRoot: t.TempDir()}, reads: map[string]int{}}
	manager := &Manager{Store: Store{Root: t.TempDir()}, Now: func() time.Time { return now }, Adapters: map[string]Adapter{"claude-headless": adapter}}
	for _, session := range []string{"recent", "empty"} {
		row := fmt.Sprintf(`{"type":"assistant","timestamp":%q,"message":{"id":%q,"usage":{"input_tokens":1}}}`+"\n", now.Add(-time.Minute).Format(time.RFC3339Nano), session)
		if session == "empty" {
			row = "{}\n"
		}
		if err := os.WriteFile(filepath.Join(adapter.ProjectsRoot, session+".jsonl"), []byte(row), 0600); err != nil {
			t.Fatal(err)
		}
		for i := range 2 {
			if err := manager.Store.Create(Record{ID: fmt.Sprintf("%s-%d", session, i), Adapter: "claude-headless", State: Completed, FinishedAt: now.Format(time.RFC3339Nano), AdapterData: map[string]json.RawMessage{"sessionID": json.RawMessage(fmt.Sprintf("%q", session))}}); err != nil {
				t.Fatal(err)
			}
		}
	}
	for i := range 2 {
		adapter.inventory, adapter.inventories, adapter.reads = nil, 0, map[string]int{}
		usage, err := manager.CapacityUsage(now)
		if err != nil || len(usage.Sessions) != 1 || adapter.inventories != 1 || adapter.reads["recent"] != 1 || adapter.reads["empty"] != 1 {
			t.Fatalf("display read %d: usage=%+v err=%v inventories=%d reads=%v", i, usage, err, adapter.inventories, adapter.reads)
		}
	}
}
