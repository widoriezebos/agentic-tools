package census

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

type jobIdentityProbe map[int64]identity.FixtureEntry

func (p jobIdentityProbe) FixtureEntry(pid int64) (identity.FixtureEntry, bool) {
	entry, ok := p[pid]
	return entry, ok
}

func TestJobRecordJoinUsesTheRecordedExactIdentity(t *testing.T) {
	rows := []struct {
		name    string
		process Process
		record  identityRecord
		want    bool
		mode    identity.ComparisonMode
	}{
		{
			name:    "darwin exact match",
			process: Process{Pid: 41, Started: 100, StartedExactMicro: 100_000_001},
			record:  identityRecord{Pid: 41, Started: 100, StartedExactMicro: 100_000_001},
			want:    true, mode: identity.CompareDarwinMicroseconds,
		},
		{
			name:    "darwin same-second recycled pid",
			process: Process{Pid: 41, Started: 100, StartedExactMicro: 100_000_002},
			record:  identityRecord{Pid: 41, Started: 100, StartedExactMicro: 100_000_001},
			want:    false, mode: identity.CompareDarwinMicroseconds,
		},
		{
			name:    "linux exact match despite a drifted second",
			process: Process{Pid: 41, Started: 103, StartTicks: 7001, BootID: "boot-a"},
			record:  identityRecord{Pid: 41, Started: 100, StartTicks: 7001, BootID: "boot-a"},
			want:    true, mode: identity.CompareLinuxTicksBootID,
		},
		{
			name:    "linux same-second recycled pid",
			process: Process{Pid: 41, Started: 100, StartTicks: 7002, BootID: "boot-a"},
			record:  identityRecord{Pid: 41, Started: 100, StartTicks: 7001, BootID: "boot-a"},
			want:    false, mode: identity.CompareLinuxTicksBootID,
		},
		{
			name:    "legacy seconds are explicitly labeled",
			process: Process{Pid: 41, Started: 100, StartedExactMicro: 100_900_000},
			record:  identityRecord{Pid: 41, Started: 100},
			want:    true, mode: identity.CompareLegacySeconds,
		},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			got, mode := sameProcessIdentity(row.process, row.record)
			if got != row.want || mode != row.mode {
				t.Fatalf("join=%v mode=%s, want %v/%s", got, mode, row.want, row.mode)
			}
		})
	}
}

func TestAliveRecordedKeepsLegacyFixtureVerdictsDefinitive(t *testing.T) {
	pid := int64(os.Getpid())
	probe := jobIdentityProbe{pid: {
		StartedAt: 100, HasStartedAt: true,
	}}

	for _, row := range []struct {
		name    string
		started int64
		alive   bool
	}{
		{name: "same recorded second", started: 100, alive: true},
		{name: "different recorded second", started: 101, alive: false},
	} {
		t.Run(row.name, func(t *testing.T) {
			alive, mode := AliveRecorded(identity.Ref{
				Pid: pid, StartedAtSec: row.started,
			}, probe)
			if alive != row.alive || mode != identity.CompareLegacySeconds {
				t.Fatalf("legacy fixture verdict=%v mode=%s, want %v/%s", alive, mode, row.alive, identity.CompareLegacySeconds)
			}
		})
	}
}

func TestLiveCustodyPreservesPrimaryAndChildExactIdentity(t *testing.T) {
	root := t.TempDir()
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		t.Fatal(err)
	}
	record := `{
  "status":"running",
  "instanceTag":"job-tag",
  "pid":41,
  "pidStartedAt":100,
  "pidStartedAtExactMicro":100000001,
  "custodyProcesses":[
    {"pid":42,"pidStartedAt":100,"pidStartTicks":7002,"bootId":"boot-a","instanceTag":"job-tag"}
  ]
}`
	if err := os.WriteFile(filepath.Join(jobs, "job-a.json"), []byte(record), 0o644); err != nil {
		t.Fatal(err)
	}

	got := liveCustody(root)
	if len(got) != 2 {
		t.Fatalf("custody identities = %+v", got)
	}
	if got[0].StartedExactMicro != 100_000_001 {
		t.Fatalf("primary darwin identity was discarded: %+v", got[0])
	}
	if got[1].StartTicks != 7002 || got[1].BootID != "boot-a" {
		t.Fatalf("child linux identity pair was discarded: %+v", got[1])
	}
}
