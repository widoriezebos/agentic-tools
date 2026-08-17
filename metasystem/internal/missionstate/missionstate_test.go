package missionstate

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// fakeProber answers with a scripted liveness per pid.
type fakeProber struct {
	verdicts map[int64]identity.Liveness
	starts   map[int64]int64
}

func (f fakeProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	verdict, ok := f.verdicts[pid]
	if !ok {
		return identity.Exact{}, identity.Dead, nil
	}
	switch verdict {
	case identity.Alive:
		start := f.starts[pid]
		return identity.Exact{Pid: pid, StartedAt: time.Unix(start, 0)}, identity.Alive, nil
	case identity.Unknown:
		return identity.Exact{}, identity.Unknown, errors.New("procfs unreadable")
	default:
		return identity.Exact{}, identity.Dead, nil
	}
}

func writeRecord(t *testing.T, root, name, body string) string {
	t.Helper()
	dir := filepath.Join(root, "artifacts", "agents", "missions", "runners")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// The rule, three-way, across every record shape the design names: live,
// completed, failed, crashed (running claim, dead process), identity
// mismatch (pid reused at a different start), identity unknown, and
// malformed identity fields.
func TestClassifyThreeWay(t *testing.T) {
	prober := fakeProber{
		verdicts: map[int64]identity.Liveness{
			101: identity.Alive,
			102: identity.Dead,
			103: identity.Unknown,
			104: identity.Alive,
		},
		starts: map[int64]int64{101: 1000, 104: 2000},
	}
	cases := []struct {
		name   string
		record RecordFields
		want   Verdict
	}{
		{"live", RecordFields{Status: "running", Pid: 101, PidStartedAt: 1000}, Active},
		{"completed", RecordFields{Status: "completed", Pid: 101, PidStartedAt: 1000}, Inactive},
		{"failed", RecordFields{Status: "failed", Pid: 101, PidStartedAt: 1000}, Inactive},
		{"crashed", RecordFields{Status: "running", Pid: 102, PidStartedAt: 1000}, Inactive},
		{"identity-mismatch", RecordFields{Status: "running", Pid: 104, PidStartedAt: 1234}, Inactive},
		{"identity-unknown", RecordFields{Status: "running", Pid: 103, PidStartedAt: 1000}, Indeterminate},
		{"malformed-pid", RecordFields{Status: "running", Pid: 0, PidStartedAt: 1000}, Indeterminate},
		{"malformed-start", RecordFields{Status: "running", Pid: 101, PidStartedAt: 0}, Indeterminate},
	}
	for _, c := range cases {
		if got := Classify(c.record, prober); got != c.want {
			t.Errorf("%s: got %s want %s", c.name, got, c.want)
		}
	}
}

// Survey classifies per record, surfaces unreadable and unparsable records
// instead of skipping them, and treats a missing directory as empty.
func TestSurveyClassifiesAndSurfaces(t *testing.T) {
	root := t.TempDir()
	prober := fakeProber{
		verdicts: map[int64]identity.Liveness{201: identity.Alive, 202: identity.Unknown},
		starts:   map[int64]int64{201: 5000},
	}

	writeRecord(t, root, "m-live.json", `{"missionId":"m-live","status":"running","pid":201,"pidStartedAt":5000}`)
	writeRecord(t, root, "m-done.json", `{"missionId":"m-done","status":"completed","pid":201,"pidStartedAt":5000}`)
	writeRecord(t, root, "m-unknown.json", `{"missionId":"m-unknown","status":"running","pid":202,"pidStartedAt":5000}`)
	writeRecord(t, root, "m-garbage.json", `{not json`)

	result := Survey(root, prober)
	active := result.ActiveMissions()
	if len(active) != 1 || active[0].MissionId != "m-live" {
		t.Fatalf("active missions wrong: %+v", active)
	}
	if !result.Indeterminate() {
		t.Fatal("the unknown-liveness runner did not surface as indeterminate")
	}
	// The garbage record and the unknown runner both land in Unreadable.
	if len(result.Unreadable) != 2 {
		t.Fatalf("unreadable inventory wrong: %v", result.Unreadable)
	}
	if active[0].Detail == "" || len(active[0].Detail) > 200 {
		t.Fatalf("detail unbounded or empty: %q", active[0].Detail)
	}

	empty := Survey(t.TempDir(), prober)
	if len(empty.Runners) != 0 || len(empty.Unreadable) != 0 {
		t.Fatalf("missing directory is not an empty result: %+v", empty)
	}
}

// Issue #1: a pair-bearing runner record classifies Active under clock
// drift; a legacy record keeps the old semantics. This is the decision
// surface behind `mission-runner.sh status` — the observed false
// "abandoned reason=runner-process-gone".
type driftProber struct{ shift int64 }

func (p driftProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	return identity.Exact{
		Pid: pid, StartedAt: time.Unix(1786991670+p.shift, 0),
		StartTicks: 707949, BootID: "boot-aaaa",
	}, identity.Alive, nil
}

func TestClassifySurvivesClockDrift(t *testing.T) {
	paired := RecordFields{Status: "running", Pid: 40723, PidStartedAt: 1786991670,
		PidStartTicks: 707949, BootID: "boot-aaaa"}
	if got := Classify(paired, driftProber{4}); got != Active {
		t.Fatalf("paired record under drift: want Active, got %v", got)
	}
	legacy := RecordFields{Status: "running", Pid: 40723, PidStartedAt: 1786991670}
	if got := Classify(legacy, driftProber{4}); got != Inactive {
		t.Fatalf("legacy record under drift keeps old semantics: got %v", got)
	}
}
