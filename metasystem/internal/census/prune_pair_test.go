package census

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// The announcement prune treats the pair as
// EXCLUSIVE when both sides carry it — a recycled pid whose start second
// happens to match but whose ticks differ is a stranger and the stale
// announcement is pruned; a drifted second with a matching pair survives.
func TestAnnouncementPrunePairExclusive(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "artifacts", "agents", "mains")
	os.MkdirAll(dir, 0o755)
	write := func(name string, ann map[string]any) string {
		data, _ := json.Marshal(ann)
		path := filepath.Join(dir, name)
		os.WriteFile(path, append(data, '\n'), 0o644)
		return path
	}
	base := func(extra map[string]any) map[string]any {
		ann := map[string]any{
			"sessionId": "s-x", "pid": 9001, "pidStartedAt": 1786991670,
			"pgid": 9001, "runtime": "fake", "instanceTag": "tag-x",
			"announcedAt": "2026-08-17T00:00:00Z",
		}
		for k, v := range extra {
			ann[k] = v
		}
		return ann
	}
	driftedPath := write("s-x-9001.json", base(map[string]any{
		"pidStartTicks": 707949, "bootId": "boot-a",
	}))
	stalePath := write("s-y-9002.json", func() map[string]any {
		ann := base(map[string]any{"pidStartTicks": 111111, "bootId": "boot-a"})
		ann["sessionId"] = "s-y"
		ann["pid"] = 9002
		return ann
	}())

	processes := []Process{
		// Same pid, drifted second, matching pair: the announced main.
		{Pid: 9001, Started: 1786991674, StartTicks: 707949, BootID: "boot-a", Alive: true},
		// Same pid, SAME second, different ticks: a recycled stranger.
		{Pid: 9002, Started: 1786991670, StartTicks: 222222, BootID: "boot-a", Alive: true},
	}
	var errs []string
	live := announcementsList(root, processes, nil, &errs)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(live) != 1 || live[0].Pid != 9001 {
		t.Fatalf("live set wrong: %+v", live)
	}
	if _, err := os.Stat(driftedPath); err != nil {
		t.Fatal("drifted-second announcement with a matching pair was pruned")
	}
	if _, err := os.Stat(stalePath); !os.IsNotExist(err) {
		t.Fatal("same-second wrong-pair announcement survived")
	}
}
