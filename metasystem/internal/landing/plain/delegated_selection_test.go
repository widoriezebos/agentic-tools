package plain

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
)

func TestADelegatedSelectionCarriesNoPersonOverride(t *testing.T) {
	t.Parallel()
	for _, name := range []string{
		"held", "pause", "stop with reason", "unreadable pause", "other stop", "unknown lane stop",
		"incident", "gone proof", "waiting changed", "fingerprint changed", "fingerprint unreadable",
		"merge", "not held", "no hold identity", "no grant", "no decision", "no session", "another class",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b := newResumeBed(t)
			b.queue(t)
			state := lane.AgentState{Barren: 2, BarrenAt: bedNow.Add(-time.Minute).Format(time.RFC3339Nano),
				BarrenFingerprint: "same", BarrenLaunches: []string{"first", "last"}}
			writeState := func() {
				t.Helper()
				data, err := json.Marshal(state)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(lane.AgentStatePath(b.home), data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			writeState()
			if err := RecordBarrenStop(b.install, state, "unchanged after two runs", bedNow.Add(-time.Minute), b.seams); err != nil {
				t.Fatal(err)
			}
			d := DelegatedProvenance{Grant: "G1", Decision: "D1", Session: "main-1", Class: "lane-recovery",
				Delegate: "Wido", CheckedAt: bedNow, Hold: DelegatedHold{BarrenAt: "caller-supplied"}}
			fingerprint := func(root string) (string, error) {
				for _, path := range []string{lockPath(b.install), lane.LockPath(b.home)} {
					held, err := lock.File(path, 0600, lock.TryExclusive)
					if held != nil {
						held.Release()
					}
					if !lock.Busy(err) {
						t.Fatalf("held state read without its lock: %s: %v", path, err)
					}
				}
				return b.keeper.Fingerprint(root)
			}
			entries, err := Entries(b.install)
			if err != nil {
				t.Fatal(err)
			}
			goals := []string{}
			for _, entry := range entries {
				if entry.State == StateWaiting {
					goals = append(goals, entry.Goal)
				}
			}
			switch name {
			case "pause":
				_, err = lane.SetPause(b.home, "Wido", bedNow)
			case "stop with reason":
				_, err = lane.SetPauseBecause(b.home, "Wido", "maintenance", bedNow)
			case "unreadable pause":
				err = os.WriteFile(filepath.Join(lane.HostDir(b.home), "landing-lane-paused.json"), []byte("bad json"), 0600)
			case "other stop", "unknown lane stop":
				loop := "lane-proof"
				if name == "unknown lane stop" {
					loop = "lane-return"
				}
				err = appendLine(stopsPath(b.install), Stop{Loop: loop, Subject: "lane", Decision: "stop", At: bedNow.Format(time.RFC3339Nano)})
			case "incident":
				b.incidents = []goal.TrunkRedEntry{{Identity: "red", Opened: bedNow.Format(time.RFC3339)}}
			case "gone proof":
				err = writeRunning(b.install, Running{Attempt: "gone", Pid: 99})
			case "waiting changed":
				_, _, err = HandIn(b.install, Line{Goal: "third", SHA: "third", At: bedNow.Format(time.RFC3339)})
			case "fingerprint changed":
				fingerprint = func(string) (string, error) { return "changed", nil }
			case "fingerprint unreadable":
				fingerprint = func(string) (string, error) { return "", errors.New("unreadable fingerprint") }
			case "merge":
				b.merge = "tip"
			case "not held":
				state.Barren = 1
				writeState()
			case "no hold identity":
				state.BarrenAt = ""
				writeState()
			case "no grant":
				d.Grant = ""
			case "no decision":
				d.Decision = ""
			case "no session":
				d.Session = ""
			case "another class":
				d.Class = "budget"
			}
			if err != nil {
				t.Fatal(err)
			}
			before := resumeSnapshot(t, b.install, b.home)
			batch, err := SelectDelegatedBatch(b.install, b.home, b.record.Root, b.record, goals, d, fingerprint, b.seams)
			if name != "held" {
				if err == nil || batch != nil || !reflect.DeepEqual(before, resumeSnapshot(t, b.install, b.home)) {
					t.Fatalf("refused selection changed records: batch=%+v err=%v", batch, err)
				}
				want := map[string]string{"pause": "pause", "stop with reason": "pause", "unreadable pause": "pause",
					"other stop": "stop other", "unknown lane stop": "stop other", "incident": "incident",
					"gone proof": "process is gone", "waiting changed": "waiting goals changed", "fingerprint changed": "barren hold",
					"fingerprint unreadable": "unreadable fingerprint", "merge": "merge in progress", "not held": "barren hold",
					"no hold identity": "identity", "no grant": "grant", "no decision": "decision", "no session": "session", "another class": "lane-recovery"}[name]
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("refusal lost its cause %q: %v", want, err)
				}
				return
			}
			if err != nil || batch == nil {
				t.Fatalf("held selection failed: %+v %v", batch, err)
			}
			wantHold := DelegatedHold{BarrenAt: state.BarrenAt, Fingerprint: state.BarrenFingerprint, Launch: "last"}
			if batch.Person != nil || batch.Delegated == nil || batch.Delegated.Decision != d.Decision || batch.Delegated.Hold != wantHold || batch.Delegated.Kind != "selection" || batch.Delegated.Destination != b.record || !slices.Equal(batch.Delegated.Subject, batch.Members) || len(batch.Members) != 2 {
				t.Fatalf("selection lost its exact delegation: %+v", batch)
			}
			if batch.Delegated.BarrenStopAt != state.BarrenAt {
				t.Fatalf("wrong barren stop bound: %+v", batch.Delegated)
			}
			stored, err := RecordedDelegatedBatch(b.install, b.record, batch.ID)
			if err != nil || !reflect.DeepEqual(stored, batch) {
				t.Fatalf("selection was not durable: %+v %v", stored, err)
			}
			stops, err := OpenStops(b.install)
			if err != nil || len(stops) != 0 {
				t.Fatalf("barren stop was not closed: %+v %v", stops, err)
			}
		})
	}
}
