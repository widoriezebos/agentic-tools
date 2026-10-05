package steward

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/outage"
)

func TestRunnerLogsProviderHoldOnceAndStartsOnNextKeeperTick(t *testing.T) {
	t.Parallel()
	for _, clears := range []bool{false, true} {
		t.Run(map[bool]string{false: "expires", true: "cleared"}[clears], func(t *testing.T) {
			t.Parallel()
			home, root := t.TempDir(), t.TempDir()
			if err := os.MkdirAll(lane.HostDir(home), 0o755); err != nil {
				t.Fatal(err)
			}
			data, _ := json.Marshal(lane.Record{Root: root, Install: root, CustodyEpoch: 1})
			if err := os.WriteFile(filepath.Join(lane.HostDir(home), "landing-lane.json"), data, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, _, err := plain.HandIn(root, plain.Line{Goal: "waiting", SHA: "tip"}); err != nil {
				t.Fatal(err)
			}
			now := time.Date(2026, 10, 4, 16, 0, 0, 0, time.UTC)
			if _, err := outage.Record(root, "overloaded", "fixture", "fixture", now); err != nil {
				t.Fatal(err)
			}
			starts, alive := 0, false
			keeper := lane.AgentKeeper{Home: home, Self: root, Now: func() time.Time { return now },
				Sources: lane.WakeSources{Reasons: func(string) ([]string, error) {
					waiting, err := plain.Waiting(root)
					if err != nil || len(waiting) == 0 {
						return nil, err
					}
					return []string{plain.WakeQueued}, nil
				}},
				Running: func() (string, bool, error) { return "fixture-agent", alive, nil },
				Start:   func(string, lane.Wake) (string, error) { starts++; alive = true; return "fixture-agent", nil },
				Waiting: func(install, _ string) (int, error) { entries, err := plain.Waiting(install); return len(entries), err },
				Holds: []func(string) (string, error){func(string) (string, error) {
					if _, held := outage.StandingAt(root, now); held {
						return "the model provider is held", nil
					}
					return "", nil
				}},
			}
			var log bytes.Buffer
			keeping := &laneKeeping{step: keeper.Run, log: &log}
			for range 3 {
				keeping.run()
				now = now.Add(laneRecheck)
			}
			if starts != 0 || strings.Count(log.String(), "the model provider is held") != 1 {
				t.Fatalf("held: starts=%d, log=%q", starts, log.String())
			}
			if clears {
				if err := outage.Clear(root); err != nil {
					t.Fatal(err)
				}
			} else {
				now = now.Add(outage.Horizon)
			}
			if stopped := runnerWait(root, laneRecheck+time.Second, runnerLoopDependencies{
				Now: func() time.Time { return now }, Sleep: func(d time.Duration) { now = now.Add(d) },
			}, nil, keeping, nil); stopped {
				t.Fatal("keeper stopped")
			}
			if starts != 1 || !strings.Contains(log.String(), "lane agent started by the keeper: 1 waiting line(s)") {
				t.Fatalf("next tick: starts=%d, log=%q", starts, log.String())
			}
		})
	}
}
