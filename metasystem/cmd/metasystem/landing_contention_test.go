package main

import (
	"encoding/json"
	"errors"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLandingKeeperHoldsTheAgentDuringContention(t *testing.T) {
	t.Parallel()
	type observation struct {
		builds int
		err    error
		calls  int
	}
	// bed wires the production keeper with a counting observation of host work.
	bed := func(t *testing.T, holdLine bool) (func() (bool, error), *time.Time, *observation) {
		t.Helper()
		home, checkout := t.TempDir(), t.TempDir()
		now := machineBedNow
		seen := &observation{builds: 1}
		record := lane.Record{Root: checkout, Install: filepath.Join(checkout, "metasystem"), CustodyEpoch: 1, RegisteredBy: "Wido"}
		data, err := json.Marshal(record)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(lane.HostDir(home), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(lane.RecordPath(home), data, 0600); err != nil {
			t.Fatal(err)
		}
		if holdLine {
			if err := plain.RecordIntervention(record.Install, plain.Intervention{At: now.Add(-10 * time.Minute).Format(time.RFC3339Nano), Row: "proof-beside-builds", Subject: "main", Outcome: "held: 1 builds and 0 unit checks run"}); err != nil {
				t.Fatal(err)
			}
		}
		agent := newTestLandingAgent(func(a *landingAgent) {
			a.now = func() time.Time { return now }
			a.machine = func(string) (string, error) { return "landing", nil }
			a.proofEffects.HostWork = func() (int, int, error) { seen.calls++; return seen.builds, 0, seen.err }
		})
		keeper := newLandingAgentKeeper(checkout, home, agent)
		contention := func() (bool, error) {
			t.Helper()
			for _, hold := range keeper.Holds {
				reason, err := hold(record.Install)
				if err != nil {
					return true, err
				}
				if strings.Contains(reason, "proof start is held beside builds until") {
					return true, nil
				}
			}
			return false, nil
		}
		return contention, &now, seen
	}
	t.Run("deadline", func(t *testing.T) {
		t.Parallel()
		contention, now, _ := bed(t, true)
		if held, err := contention(); !held || err != nil {
			t.Fatalf("keeper omitted the proof contention hold: %v", err)
		}
		*now = now.Add(21 * time.Minute)
		if held, err := contention(); held || err != nil {
			t.Fatalf("keeper kept the expired contention hold: %v", err)
		}
	})
	t.Run("the build ends", func(t *testing.T) {
		t.Parallel()
		contention, now, seen := bed(t, true)
		seen.builds = 0
		*now = now.Add(time.Minute)
		if held, err := contention(); held || err != nil {
			t.Fatalf("keeper held after the builds ended: %v", err)
		}
	})
	t.Run("unknown holds", func(t *testing.T) {
		t.Parallel()
		contention, _, seen := bed(t, true)
		seen.err = errors.New("the launch store cannot be read")
		if held, _ := contention(); !held {
			t.Fatal("keeper read an unobservable host as quiet")
		}
	})
	t.Run("no hold line", func(t *testing.T) {
		t.Parallel()
		contention, _, seen := bed(t, false)
		if held, err := contention(); held || err != nil || seen.calls != 0 {
			t.Fatalf("held=%v err=%v observations=%d", held, err, seen.calls)
		}
	})
}
