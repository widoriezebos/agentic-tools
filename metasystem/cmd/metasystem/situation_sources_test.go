package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

var situationNow = time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)

// situationLaneBed registers a lane on a Git checkout the lane's status
// reads, with a proof-beside-builds hold written ten minutes ago.
func situationLaneBed(t *testing.T) string {
	t.Helper()
	home, checkout := t.TempDir(), t.TempDir()
	install := filepath.Join(checkout, "metasystem")
	if err := os.MkdirAll(install, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(install, "metasystem.conf"), []byte("metasystem.template=true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"add", "-A"},
		{"-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "lane"}, {"update-ref", "refs/remotes/origin/main", "HEAD"}} {
		if out, err := exec.Command("git", append([]string{"-C", checkout}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	data, err := json.Marshal(lane.Record{Root: checkout, Install: install, CustodyEpoch: 1, RegisteredBy: "Wido"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(lane.HostDir(home), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lane.RecordPath(home), data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := plain.RecordIntervention(install, plain.Intervention{At: situationNow.Add(-10 * time.Minute).Format(time.RFC3339Nano),
		Row: "proof-beside-builds", Subject: "main", Outcome: "held: 1 builds and 0 unit checks run"}); err != nil {
		t.Fatal(err)
	}
	return home
}

func TestTheSituationAndTheLaneSayOneHold(t *testing.T) {
	t.Parallel()
	home := situationLaneBed(t)
	parent := t.TempDir()
	store := launch.Store{Root: filepath.Join(parent, "launch")}
	if err := store.Create(launch.Record{ID: "host-build", Kind: "build", State: launch.Running}); err != nil {
		t.Fatal(err)
	}
	now := situationNow
	read := func() steward.Situation {
		t.Helper()
		src := situationSources(defaultIntentOwners(), steward.SituationSources{Now: func() time.Time { return now },
			HostWork: (&launch.Manager{Store: store}).HostWork, LaneHome: func() (string, error) { return home, nil }})
		s := steward.ReadSituation(t.TempDir(), src)
		if s.Unavailable["lane"] != "" || s.Lane.Health == nil {
			t.Fatalf("the lane does not read: %q problems=%v", s.Unavailable["lane"], s.Lane.Problems)
		}
		if s.Unavailable["ledger"] == "" || s.Unavailable["quota"] == "" {
			t.Fatalf("a checkout with no ledger reads one: %v", s.Unavailable)
		}
		return s
	}
	row := func(s steward.Situation) bool {
		for _, fired := range s.Lane.Health.Drift {
			if fired.Row == "proof-beside-builds" {
				return true
			}
		}
		return false
	}

	s := read()
	if s.Hold == nil || s.Hold.Subject != "main" || !s.Hold.Until.Equal(situationNow.Add(20*time.Minute)) || !row(s) {
		t.Fatalf("while the build runs: hold=%+v row=%v", s.Hold, row(s))
	}
	if _, err := store.Update("host-build", func(r *launch.Record) error { r.State = launch.Completed; return nil }); err != nil {
		t.Fatal(err)
	}
	now = situationNow.Add(time.Minute)
	if s = read(); s.Hold != nil || row(s) || s.Unavailable["hold"] != "" {
		t.Fatalf("after the build completed: hold=%+v row=%v unavailable=%q", s.Hold, row(s), s.Unavailable["hold"])
	}
	if err := os.Rename(filepath.Join(parent, "launch"), filepath.Join(parent, "launch-gone")); err != nil {
		t.Fatal(err)
	}
	now = situationNow.Add(2 * time.Minute)
	if s = read(); s.Hold == nil || !strings.Contains(s.Unavailable["hold"], store.Root) {
		t.Fatalf("an unreadable store holds: hold=%+v unavailable=%q", s.Hold, s.Unavailable["hold"])
	}
}
