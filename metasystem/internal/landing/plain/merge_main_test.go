package plain

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

func TestDrainAndProofPolicyAdmission(t *testing.T) {
	t.Parallel()
	for _, entrypoint := range []string{"start", "wait"} {
		for _, scenario := range []string{"idle agent", "queued agent needs person", "queued automatic proof", "direct person"} {
			t.Run(entrypoint+"/"+scenario, func(t *testing.T) {
				t.Parallel()
				b := newRepeatBed(t)
				git := b.seams.Git
				b.seams.Git = func(dir string, args ...string) (string, error) {
					if args[0] == "cat-file" || args[0] == "merge-base" && args[2] == args[3] {
						return "", nil
					}
					return git(dir, args...)
				}
				if strings.HasPrefix(scenario, "queued") {
					if _, _, err := HandIn(b.install, Line{Goal: "g", SHA: "commit"}); err != nil {
						t.Fatal(err)
					}
				}
				drainFixture(t, b.install)
				b.seams.Policy = func(string) (PolicyValue, error) {
					value := "auto"
					if scenario == "queued agent needs person" || scenario == "direct person" {
						value = "person"
					}
					return PolicyValue{Value: value, Source: "fixture"}, nil
				}
				if scenario == "direct person" {
					b.seams.Person = &ActProvenance{Kind: "proof", Person: "Wido"}
				}
				launches := 0
				b.seams.Launch = func([]string, string, string) (int64, error) {
					launches++
					return int64(os.Getpid()), nil
				}
				var err error
				if entrypoint == "start" {
					var running Running
					running, _, err = Start(b.install, b.checkout, b.seams)
					if err == nil && (running.Admission == nil || running.Admission.State != "launched" || launches != 1) {
						t.Fatalf("missing detached admission: %+v launches=%d", running, launches)
					}
				} else {
					var result Result
					result, err = Run(b.install, b.checkout, "printf 'LANDING-CHECKED\\t0\\n'", "", io.Discard, b.seams)
					if err == nil && (result.Result != Green || !result.CountedFull || (result.Person != nil) != (scenario == "direct person")) {
						t.Fatalf("missing full proof or person provenance: %+v", result)
					}
				}
				switch scenario {
				case "idle agent":
					var closed *AdmissionClosed
					if !errors.As(err, &closed) {
						t.Fatalf("idle proof crossed drain: %v", err)
					}
				case "queued agent needs person":
					var refusal *Refusal
					if !errors.As(err, &refusal) || refusal.Code != "LANE_PROOF_PERSON" {
						t.Fatalf("drain waived proof policy: %v", err)
					}
				default:
					if err != nil {
						t.Fatal(err)
					}
				}
				if err != nil {
					if _, recorded, _, readErr := ReadRunning(b.install, b.seams); readErr != nil || recorded || launches != 0 {
						t.Fatalf("refused proof started: recorded=%v launches=%d err=%v", recorded, launches, readErr)
					}
				}
			})
		}
	}
}

func TestDetachedPersonProofFinishesAfterDrain(t *testing.T) {
	t.Parallel()
	b := newRepeatBed(t)
	b.seams.Person = &ActProvenance{Kind: "proof", Person: "Wido"}
	b.seams.Policy = func(string) (PolicyValue, error) { return PolicyValue{Value: "person"}, nil }
	b.seams.Launch = func([]string, string, string) (int64, error) { return int64(os.Getpid()), nil }
	running, _, err := Start(b.install, b.checkout, b.seams)
	if err != nil {
		t.Fatal(err)
	}
	drainFixture(t, b.install)
	// A child consumes the recorded act rather than proving a new caller.
	b.seams.Person = nil
	result, err := Run(b.install, b.checkout, "printf 'LANDING-CHECKED\\t0\\n'", running.Attempt, io.Discard, b.seams)
	if err != nil || result.Result != Green || result.Person == nil || result.Person.Person != "Wido" || !result.CountedFull {
		t.Fatalf("admitted child lost the person's proof: %+v %v", result, err)
	}
	if _, err := Run(b.install, b.checkout, "exit 0", running.Attempt, io.Discard, b.seams); err == nil {
		t.Fatal("the detached admission was consumed twice")
	}
}

func TestCheckedHandInRetainsDrainAndExceptionChecks(t *testing.T) {
	t.Parallel()
	for _, boundary := range []string{"valid", "lane changed", "incidents changed", "queue changed", "agent"} {
		t.Run(boundary, func(t *testing.T) {
			t.Parallel()
			install := t.TempDir()
			record := lane.Record{Root: install, Install: install, CustodyEpoch: 1}
			line := Line{Goal: "g", SHA: "tip", ExpectEmpty: true, Exception: &Exception{
				Person:    &ActProvenance{Kind: "trunk-exception", Person: "Wido", Destination: record},
				Incidents: &IncidentCoverage{Open: []IncidentIdentity{}},
			}}
			if boundary == "queue changed" {
				if _, _, err := HandIn(install, Line{Goal: "g", SHA: "another-tip"}); err != nil {
					t.Fatal(err)
				}
			}
			drain := drainFixture(t, install)
			drain.State = DrainHeld
			if err := withLock(install, func() error { return writeDrain(install, drain) }); err != nil {
				t.Fatal(err)
			}
			seams := ProveSeams{
				Lane: func() (lane.Record, error) {
					current := record
					if boundary == "lane changed" {
						current.CustodyEpoch++
					}
					return current, nil
				},
				Git: func(string, ...string) (string, error) { return "main", nil },
				Incidents: func(string, string, string) ([]goal.TrunkRedEntry, error) {
					if boundary == "incidents changed" {
						return []goal.TrunkRedEntry{{Identity: "new", Opened: bedNow.Format(time.RFC3339), Class: goal.TrunkRedClassTrunkRed}}, nil
					}
					return nil, nil
				},
			}
			by := "Wido"
			if boundary == "agent" {
				by = ""
			}
			entry, added, err := HandInChecked(install, line, by, seams)
			current, readErr := ReadDrain(install)
			if readErr != nil || current == nil {
				t.Fatal(current, readErr)
			}
			if boundary == "valid" {
				if err != nil || !added || entry.Exception == nil || entry.DrainBy != "Wido" || current.State != DrainDraining {
					t.Fatalf("lost exception or drain authority: %+v %+v %v", entry, current, err)
				}
				return
			}
			if err == nil || added || current.State != DrainHeld {
				t.Fatalf("%s admitted work or reopened drain: %+v %+v %v", boundary, entry, current, err)
			}
			if boundary == "agent" {
				var closed *AdmissionClosed
				if !errors.As(err, &closed) {
					t.Fatalf("exception lent an agent drain authority: %v", err)
				}
			}
			entries, readErr := Entries(install)
			want := 0
			if boundary == "queue changed" {
				want = 1
			}
			if readErr != nil || len(entries) != want {
				t.Fatalf("refused hand-in changed queue: %+v %v", entries, readErr)
			}
		})
	}
}

func TestWakeTimerHonorsMaintenanceAndDrain(t *testing.T) {
	t.Parallel()
	for _, fence := range []string{"none", "maintenance", "drain", "both"} {
		t.Run(fence, func(t *testing.T) {
			t.Parallel()
			install := t.TempDir()
			if err := os.WriteFile(filepath.Join(install, "metasystem.conf"), []byte("metasystem.template=true\nproof.trunk-every=1h\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if fence == "drain" || fence == "both" {
				drainFixture(t, install)
			}
			reasons, err := wakeReasons(install, true, bedNow, bedNow, fence == "maintenance" || fence == "both")
			want := []string{WakeQueued}
			if fence == "none" {
				want = append(want, WakeFullDue)
			}
			if err != nil || strings.Join(reasons, ",") != strings.Join(want, ",") {
				t.Fatalf("%s: reasons=%v want=%v err=%v", fence, reasons, want, err)
			}
		})
	}
}

func TestDrainAccountsForRecordedBatch(t *testing.T) {
	t.Parallel()
	for _, state := range []string{"missing member", "damaged batch", "terminal", "empty proposal", "running operation"} {
		t.Run(state, func(t *testing.T) {
			t.Parallel()
			install := t.TempDir()
			drainFixture(t, install)
			batch := &Batch{ID: "selection", Base: "main", Lane: lane.Record{Root: install, Install: install, CustodyEpoch: 1}, State: BatchPrepared}
			if state != "empty proposal" {
				batch.Members = []GoalSHA{{Goal: "g", SHA: "tip"}}
			}
			if err := writeBatch(install, batch); err != nil {
				t.Fatal(err)
			}
			if state == "damaged batch" {
				if err := os.WriteFile(batchPath(install), []byte("{broken"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			notAncestor := exec.Command("/usr/bin/false").Run()
			seams := ProveSeams{Git: func(_ string, args ...string) (string, error) {
				if args[0] == "merge-base" && state == "missing member" {
					return "", notAncestor
				}
				return "main", nil
			}, AgentRunning: func() (bool, error) { return state == "running operation", nil }}
			progress, err := AdvanceDrain(install, install, seams)
			wantHeld := state == "terminal" || state == "empty proposal"
			wantUnknown := state == "missing member" || state == "damaged batch"
			if err != nil || (progress.Drain.State == DrainHeld) != wantHeld || (progress.Unknown != "") != wantUnknown {
				t.Fatalf("%s: progress=%+v err=%v", state, progress, err)
			}
			if state != "damaged batch" {
				current, err := ReadBatch(install)
				if err != nil || (current.State == BatchClosed) != wantHeld {
					t.Fatalf("%s: batch=%+v err=%v", state, current, err)
				}
			}
		})
	}
}

func TestStatusShowsPolicySelectionAndDrainTogether(t *testing.T) {
	t.Parallel()
	install := t.TempDir()
	record := lane.Record{Root: install, Install: install, CustodyEpoch: 1}
	if _, _, err := HandIn(install, Line{Goal: "g", SHA: "tip"}); err != nil {
		t.Fatal(err)
	}
	drainFixture(t, install)
	batch := &Batch{ID: "selection", Base: "main", Lane: record, State: BatchPrepared, Members: []GoalSHA{{Goal: "g", SHA: "tip"}}, Selector: PolicyValue{Value: "person"}}
	if err := writeBatch(install, batch); err != nil {
		t.Fatal(err)
	}
	notAncestor := exec.Command("/usr/bin/false").Run()
	seams := ProveSeams{Git: func(_ string, args ...string) (string, error) {
		if args[0] == "merge-base" {
			return "", notAncestor
		}
		return "main", nil
	}, Incidents: func(string, string, string) ([]goal.TrunkRedEntry, error) { return nil, nil },
		Policy: func(key string) (PolicyValue, error) {
			value := "auto"
			if key == "landing.batch" {
				value = "person"
			}
			return PolicyValue{Value: value, Source: "fixture"}, nil
		}}
	status := ReadStatus(t.TempDir(), record, lane.View{Root: &install}, seams)
	selected, ok := status.Batch.(*Batch)
	if len(status.Problems) != 0 || status.Drain == nil || status.Drain.By != "Wido" || status.DrainWaiting != 1 || status.Admission != "draining, 1 waiting" || status.BatchPolicy == nil || status.BatchPolicy.Value != "person" || len(status.PendingActions) != 1 || status.PendingActions[0].Subject.BatchID != batch.ID || !ok || selected.ID != batch.ID {
		t.Fatalf("status lost policy selection or drain: %+v", status)
	}
	drain, err := ReadDrain(install)
	if err != nil || drain.State != DrainDraining {
		t.Fatalf("status changed drain: %+v %v", drain, err)
	}
}
