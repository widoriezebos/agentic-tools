package plain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

func retiredHoldBed(t *testing.T) *stewardBed {
	t.Helper()
	b := newStewardBed(t)
	if err := os.WriteFile(filepath.Join(b.install, "metasystem.conf"), []byte("testing.contract=testing.json\nproof.trunk-every=24h\n"), 0600); err != nil {
		t.Fatal(err)
	}
	b.incidents()
	b.seams.Steward.Trunk = nil
	b.seams.Trunk, b.seams.Fresh = true, true
	b.seams.Executable = func() (string, error) { return "/fixture/engine", nil }
	b.seams.NewID = func() string { return "check-beside-builds" }
	b.seams.Lane = func() (lane.Record, error) { return b.record, nil }
	b.seams.CommandForCommit = func(string) (string, error) { return "the committed check command", nil }
	b.seams.Alive = func(Running) bool { return len(b.calls) > 0 }
	b.seams.HostWork = func() (int, int, error) { return 3, 2, nil }
	b.seams.Launch = func(argv []string, _, log string) (int64, error) {
		if !strings.Contains(strings.Join(argv, " "), "--trunk") {
			t.Fatalf("clock launched a different check: %v", argv)
		}
		if err := os.WriteFile(log, nil, 0600); err != nil {
			return 0, err
		}
		b.calls = append(b.calls, "launch")
		return int64(os.Getpid()), nil
	}
	return b
}

func TestTheLanesCheckStartsBesideRunningBuilds(t *testing.T) {
	t.Parallel()
	b := retiredHoldBed(t)
	hold, err := KeeperClock(b.record, func(lane.Record) error { return nil }, b.seams)
	if err != nil || len(b.calls) != 1 || !strings.Contains(hold, "proving main") {
		t.Fatalf("clock did not launch its due check: calls=%v hold=%q err=%v", b.calls, hold, err)
	}
	entries, err := readLines[Intervention](interventionsPath(b.install))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Row == "proof-beside-builds" {
			t.Fatalf("starting beside builds wrote an intervention: %+v", entry)
		}
	}
	run := b.step(bedNow)
	if strings.Contains(run.Line, "held beside builds") || strings.Contains(run.Line, "proof start is held") {
		t.Fatalf("steward kept the retired bound: %s", run.Line)
	}
	status := ReadStatus(b.home, b.record, b.view, b.seams)
	row := healthRow(status.Health, "proof-beside-builds")
	if len(status.Problems) != 0 || row == nil || !strings.Contains(row.Act, "3 builds and 2 unit checks run beside") || !strings.Contains(row.Act, "reported, not held") {
		t.Fatalf("running check lost its host observation: row=%+v problems=%v", row, status.Problems)
	}
}

func TestAnOldClocksRetryDeadlineIsHonouredOnce(t *testing.T) {
	t.Parallel()
	b := retiredHoldBed(t)
	retry := bedNow.Add(10 * time.Minute)
	old := map[string]any{"held": true, "credit": time.Time{}.String(), "retry-at": retry, "attempted": true}
	data, err := json.Marshal(old)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(clockPath(b.install)), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(clockPath(b.install), data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := KeeperClock(b.record, func(lane.Record) error { return nil }, b.seams); err != nil || len(b.calls) != 0 {
		t.Fatalf("old retry deadline was ignored: calls=%v err=%v", b.calls, err)
	}
	b.seams.Now = func() time.Time { return retry.Add(time.Second) }
	if _, err := KeeperClock(b.record, func(lane.Record) error { return nil }, b.seams); err != nil || len(b.calls) != 1 {
		t.Fatalf("old retry never ended: calls=%v err=%v", b.calls, err)
	}
	state, err := readClock(b.install)
	if err != nil || !state.RetryAt.IsZero() {
		t.Fatalf("old retry survived its wake: %+v %v", state, err)
	}
}
