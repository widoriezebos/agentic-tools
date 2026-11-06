package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

func TestTheKeeperNeverHoldsTheAgentBesideBuilds(t *testing.T) {
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
		a.proofEffects.Room = noRoomForVerbFixture
		a.proofEffects.HostWork = func() (int, int, error) { return 1, 0, nil }
	})
	keeper := newLandingAgentKeeper(checkout, home, agent)
	for _, hold := range keeper.Holds {
		reason, err := hold(record.Install)
		if err != nil || strings.Contains(reason, "builds") {
			t.Fatalf("keeper retained a build hold: reason=%q err=%v", reason, err)
		}
	}
}
