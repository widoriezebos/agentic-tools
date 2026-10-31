package plain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/state"
)

func TestLandingHealthShadowDrift(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"pause", "members", "matching", "absent", "unreadable", "failed observation", "failed without shadow"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			b := newStatusBed(t)
			s := shadowSeams()
			if _, err := lane.SetPause(b.home, "fixture", bedNow); err != nil {
				t.Fatal(err)
			}
			if _, err := lane.ClearPauseAt(b.home, "", bedNow); err != nil {
				t.Fatal(err)
			}
			batch := &Batch{ID: "batch", Lane: b.record, Base: "main", State: BatchRunning, Open: true, Members: []GoalSHA{{Goal: "A", SHA: "a"}}}
			if err := writeBatch(b.install, batch); err != nil {
				t.Fatal(err)
			}
			if _, err := RefreshShadow(b.home, bedNow, "fixture", s); err != nil {
				t.Fatal(err)
			}
			pauseAt := bedNow.Add(12 * time.Minute)
			s.Now = func() time.Time { return bedNow.Add(30 * time.Minute) }
			switch scenario {
			case "pause":
				data, _ := json.Marshal(lane.Pause{By: "the person", At: pauseAt.Format(time.RFC3339Nano)})
				if err := os.WriteFile(filepath.Join(lane.HostDir(b.home), "landing-lane-paused.json"), data, 0600); err != nil {
					t.Fatal(err)
				}
			case "members":
				batch.Members = append(batch.Members, GoalSHA{Goal: "B", SHA: "b"})
				if err := writeBatch(b.install, batch); err != nil {
					t.Fatal(err)
				}
			case "absent":
				if err := os.Remove(filepath.Join(Dir(b.install), "state.json")); err != nil {
					t.Fatal(err)
				}
			case "unreadable":
				if err := os.WriteFile(filepath.Join(Dir(b.install), "state.json"), []byte("{"), 0600); err != nil {
					t.Fatal(err)
				}
			case "failed observation":
				b.unreadable("queue.jsonl")
			case "failed without shadow":
				if err := os.Remove(filepath.Join(Dir(b.install), "state.json")); err != nil {
					t.Fatal(err)
				}
				b.unreadable("queue.jsonl")
			}
			status := ObserveStatus(b.home, b.record, s, nil)
			if status.Health == nil {
				t.Fatalf("no health: %v", status.Problems)
			}
			rows := status.Health.Rows()
			var drift []Fired
			for _, row := range rows {
				if row.Row == "drift" || row.Row == "observation-failed" {
					drift = append(drift, row)
				}
			}
			if scenario == "matching" {
				if len(drift) != 0 {
					t.Fatalf("matching records drifted: %+v", drift)
				}
				return
			}
			if len(drift) != 1 {
				t.Fatalf("want one row, got %+v", drift)
			}
			want := []string{"no state record yet"}
			switch scenario {
			case "pause":
				want = []string{"shadow: lane running since " + bedNow.Local().Format("15:04"), "records: paused since " + pauseAt.Local().Format("15:04") + " by the person"}
			case "members":
				want = []string{"batch.members", "{A a}", "{B b}"}
			case "failed observation":
				want = []string{"shadow from " + bedNow.Local().Format("15:04"), "the observation failed:", "queue"}
			case "failed without shadow":
				want = []string{"no state record yet", "the observation failed:", "queue"}
			}
			data, err := json.Marshal(status.Health)
			if err != nil {
				t.Fatal(err)
			}
			for _, text := range []string{drift[0].Subject, status.Health.Line(), status.Health.Text(), string(data)} {
				for _, part := range want {
					if !strings.Contains(text, part) {
						t.Fatalf("missing %q: %s", part, text)
					}
				}
			}
			// The failed observation's time is its detail, never its subject.
			if at := "the observation at " + s.now().Local().Format("15:04") + " failed"; strings.HasPrefix(scenario, "failed") && (drift[0].Act != at || strings.Contains(drift[0].Subject, "observation at") || !strings.Contains(string(data), at)) {
				t.Fatalf("the failed observation's row: %+v, want its time %q in the detail only", drift[0], at)
			}
		})
	}
}

func TestLandingStewardShadowIsEvidenceOnly(t *testing.T) {
	t.Parallel()
	var baseline []string
	for _, shadow := range []bool{false, true} {
		b := newStewardBed(t)
		b.staleQuestion()
		if shadow {
			r := state.Record{Lane: state.Lane{Fact: state.Fact{Word: "paused", Since: state.Since{At: bedNow}}}, Agent: state.Agent{Word: "idle", Since: state.Since{At: bedNow.Add(-time.Minute)}}}
			data, err := json.Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			b.lines("state.json", json.RawMessage(data))
		}
		run := b.step(bedNow)
		if len(run.Interventions) != 1 {
			t.Fatalf("interventions: %+v", run)
		}
		if !shadow {
			baseline = append([]string(nil), b.calls...)
			continue
		}
		want := "paused by the person; lane paused since " + bedNow.Local().Format("15:04") + "; agent idle since " + bedNow.Add(-time.Minute).Local().Format("15:04")
		if run.Interventions[0].Shadow != want || !reflect.DeepEqual(b.calls, baseline) {
			t.Fatalf("shadow changed acts or lost evidence: %+v calls=%v baseline=%v", run, b.calls, baseline)
		}
		status := readHealthBed(t, b.statusBed, b.seams)
		if len(status.Health.Interventions) != 1 || status.Health.Interventions[0].Shadow != want {
			t.Fatalf("health lost shadow words: %+v", status.Health.Interventions)
		}
	}
}

func TestObservationRetakesEngineIncidentChange(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	seams := shadowSeams()
	calls := 0
	seams.Observed = func() {
		calls++
		if calls == 1 {
			if _, err := RecordEngineIncident(b.install, "prove", nil, bedNow, "engine failure", "stack"); err != nil {
				t.Fatal(err)
			}
		}
	}
	status := ObserveStatus(b.home, b.record, seams, nil)
	record := ShadowOf(status, status.Raw)
	if calls != 2 || len(status.Problems) > 0 || len(record.Engine.Incidents) != 1 {
		t.Fatalf("incoherent engine observation: reads=%d record=%+v problems=%v", calls, record.Engine, status.Problems)
	}
}

func TestObservationFingerprintIncludesProofProgressLog(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	log := filepath.Join(t.TempDir(), "proof.log")
	if err := os.WriteFile(log, []byte("landing planned 234\n"), 0600); err != nil {
		t.Fatal(err)
	}
	b.lines("running.json", Running{Log: log})
	before := recordsFingerprint(b.home)
	if err := os.WriteFile(log, []byte("landing planned 234\nlanding package unit 1 ok 2\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if recordsFingerprint(b.home) == before {
		t.Fatal("proof progress changed without invalidating the observation")
	}
}
