package main

import (
	"encoding/json"
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
	home, checkout := t.TempDir(), t.TempDir()
	now := machineBedNow
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
	if err := plain.RecordIntervention(record.Install, plain.Intervention{At: now.Add(-10 * time.Minute).Format(time.RFC3339Nano), Row: "proof-beside-builds", Subject: "main", Outcome: "held: 1 builds and 0 unit checks run"}); err != nil {
		t.Fatal(err)
	}
	agent := newTestLandingAgent(func(a *landingAgent) {
		a.now = func() time.Time { return now }
		a.machine = func(string) (string, error) { return "landing", nil }
	})
	keeper := newLandingAgentKeeper(checkout, home, agent)
	contention := func() bool {
		t.Helper()
		for _, hold := range keeper.Holds {
			reason, err := hold(record.Install)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(reason, "proof start is held beside builds until") {
				return true
			}
		}
		return false
	}
	if !contention() {
		t.Fatal("keeper omitted the proof contention hold")
	}
	now = now.Add(21 * time.Minute)
	if contention() {
		t.Fatal("keeper kept the expired contention hold")
	}
}
