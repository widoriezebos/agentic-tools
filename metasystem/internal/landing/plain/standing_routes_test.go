package plain

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostload"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

func finishStandingAdmission(t *testing.T, b *standingBed) Result {
	t.Helper()
	running, recorded, _, err := ReadRunning(b.install, b.seams)
	if err != nil || !recorded || running.Admission.Scope != "scoped" || running.Admission.Decision.Standing == nil {
		t.Fatalf("standing admission %+v %v", running, err)
	}
	log, err := os.Create(running.Log)
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	r, err := Run(b.install, b.checkout, "fixture-proof", running.Attempt, log, b.seams)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestStandingRedStewardRoutesReachDetachedRun(t *testing.T) {
	t.Parallel()
	for _, route := range []string{"unproven-incidents", "stale-question", "suspected-contention", "observation", "unproven-incidents environment", "stale-question environment", "suspected-contention environment", "observation environment"} {
		t.Run(route, func(t *testing.T) {
			t.Parallel()
			route, _, changed := strings.Cut(route, " environment")
			b := newStandingBed(t, false)
			if changed {
				b.environment = "changed toolchain"
			}
			b.git.tree = b.red.Tree
			b.setContract(t)
			if route == "observation" {
				if err := RecoverMovedMain(b.install, b.checkout, b.seams); err != nil {
					t.Fatal(err)
				}
			} else {
				steward := newStewardBed(t)
				steward.install, steward.checkout = b.install, b.checkout
				steward.record.Root, steward.record.Install = b.checkout, b.install
				shadowRegister(t, steward.home, steward.record)
				fallback, git := steward.seams.Git, b.seams.Git
				b.seams.Git = func(dir string, args ...string) (string, error) {
					if args[0] == "symbolic-ref" {
						return "main", nil
					}
					out, err := git(dir, args...)
					if err != nil && strings.Contains(err.Error(), "is not stubbed") {
						return fallback(dir, args...)
					}
					return out, err
				}
				b.seams.Machine, b.seams.Load, b.seams.Steward = steward.seams.Machine, steward.seams.Load, steward.seams.Steward
				b.seams.Steward.Trunk = nil
				if route == "stale-question" {
					steward.staleQuestion()
				}
				if route == "suspected-contention" {
					b.red.LoadEnd = &hostload.Sample{Available: true, Cores: 2, Load1m: 10}
					b.save(t, b.red)
					for i := range b.incidents {
						b.incidents[i].Sightings[0].BaseCommit = b.git.commit
					}
				}
				run, err := StewardStep(steward.home, bedNow, b.seams)
				if err != nil || len(run.Interventions) != 1 || run.Interventions[0].Row != route {
					t.Fatalf("route %s: %+v %v", route, run, err)
				}
			}
			r := finishStandingAdmission(t, b)
			if changed {
				if r.Scope != "full" || r.Result != Green || !r.CountedFull || r.FullTree != r.Tree || r.FullAt != r.At || !r.OwedSince.IsZero() || len(b.calls) != 2 {
					t.Fatalf("route skipped environment observation %+v calls=%d", r, len(b.calls))
				}
				return
			}
			b.provenance(t, r, map[string]string{"plans": "standing", "shared": "unproved", "unrelated": "unproved", "unit/cmd/metasystem": "standing"})
			if r.Result != Red || len(r.Ran) != 0 || b.launches != 1 || len(b.calls) != 1 {
				t.Fatalf("route %s %+v launches=%d calls=%d", route, r, b.launches, len(b.calls))
			}
			if err := RecoverMovedMain(b.install, b.checkout, b.seams); err != nil {
				t.Fatal(err)
			}
			if b.launches != 1 {
				t.Fatal("exact red restarted")
			}
		})
	}
}

func TestStandingRedAdmissionRejectsChangedStandingRecord(t *testing.T) {
	t.Parallel()
	b := newStandingBed(t, false)
	running, _, err := Start(b.install, b.checkout, b.seams)
	if err != nil {
		t.Fatal(err)
	}
	// The separately serialized execution is the original admission.
	data, err := json.Marshal(running.Admission.Decision)
	if err != nil {
		t.Fatal(err)
	}
	var altered admittedScope
	if err := json.Unmarshal(data, &altered); err != nil {
		t.Fatal(err)
	}
	altered.Standing.Commit = "different-red"
	running.Admission.Decision = &altered
	if err := writeRunning(b.install, running); err != nil {
		t.Fatal(err)
	}
	_, err = Run(b.install, b.checkout, "fixture-proof", running.Attempt, b.output, b.seams)
	if err == nil || !strings.Contains(err.Error(), "admitted check scope changed") || len(b.calls) != 0 {
		t.Fatalf("changed evidence executed %v calls=%d", err, len(b.calls))
	}
}

func TestStandingRedDebtRetainsCandidateRepairProtection(t *testing.T) {
	t.Parallel()
	b := newStandingBed(t, false)
	_ = b.detached(t)
	if err := os.WriteFile(filepath.Join(b.install, "metasystem.conf"), []byte("testing.contract=testing.json\nproof.trunk-every=4h\n"), 0600); err != nil {
		t.Fatal(err)
	}
	checkpoint := Running{Attempt: "candidate", Commit: "candidate", Tree: "candidate-tree", Checkpoint: true}
	b.save(t, Result{Attempt: "candidate", Commit: "candidate", Tree: "candidate-tree", Result: Red, Cause: &Cause{Kind: "own"}})
	if err := writeRunning(b.install, checkpoint); err != nil {
		t.Fatal(err)
	}
	b.seams.Now = func() time.Time { return bedNow.Add(time.Hour) }
	subjectReads := 0
	git := b.seams.Git
	b.seams.Git = func(dir string, args ...string) (string, error) {
		if args[0] == "rev-parse" {
			subjectReads++
		}
		return git(dir, args...)
	}
	before := b.launches
	hold, err := KeeperClock(lane.Record{Root: b.checkout, Install: b.install, CustodyEpoch: 1}, func(lane.Record) error { return nil }, b.seams)
	if err != nil || hold != "" || b.launches != before || subjectReads != 0 || !strings.Contains(ClockReason(b.install), "repair") {
		t.Fatalf("debt bypassed repair: %s %v launches=%d clock=%s", hold, err, b.launches, ClockReason(b.install))
	}
}

func TestStandingRedClassificationContinuationPinsFullDepth(t *testing.T) {
	t.Parallel()
	for _, current := range []bool{false, true} {
		t.Run(fmt.Sprint(current), func(t *testing.T) {
			t.Parallel()
			b := newStandingBed(t, false)
			red := b.red
			red.Failed = red.Failed[:1]
			red.Repeat = "allowed"
			red.Commit = b.git.commit
			red.Tree = b.git.tree
			if !current {
				red.Commit = "historical"
			}
			b.save(t, red)
			b.seams.Person = &ActProvenance{Person: "Wido", Root: b.install}
			b.seams.CommandForCommit = func(string) (string, error) { return "fixture-proof", nil }
			b.seams.NewID = func() string { return "classification" }
			got, err := ClassifyAttempt(b.install, b.checkout, red.Attempt, b.seams)
			if err != nil {
				t.Fatal(err)
			}
			if !current {
				if got.NextFull != nil {
					t.Fatal("historical target admitted a full")
				}
				return
			}
			if got.NextFull == nil || got.NextFull.Scope != "full" || !got.NextFull.CountedFull || got.NextFull.ClassificationOf != red.Attempt {
				t.Fatalf("classification entered standing selector %+v", got)
			}
			fulls := 0
			for _, cmd := range b.calls {
				if commandEnv(cmd, "LANDING_ONLY") == "" && commandEnv(cmd, "LANDING_PROOF_SCOPE") == "full" {
					fulls++
				}
			}
			if fulls != 1 {
				t.Fatalf("full executions=%d", fulls)
			}
		})
	}
}

func TestStandingRedClosedIncidentsReassessExactCommit(t *testing.T) {
	t.Parallel()
	b := newStandingBed(t, false)
	_ = b.detached(t)
	for i := range b.incidents {
		b.incidents[i].Closed = &goal.TrunkRedClosure{How: "hand"}
	}
	b.seams.NewID = func() string { return "after-closure" }
	if err := RecoverMovedMain(b.install, b.checkout, b.seams); err != nil {
		t.Fatal(err)
	}
	running, _, _, err := ReadRunning(b.install, b.seams)
	if err != nil || b.launches != 2 || running.Admission.Scope != "full" {
		t.Fatalf("closed incidents kept red accepted %+v %v launches=%d", running, err, b.launches)
	}
}

func TestStandingRedNamedPrerequisiteRepeats(t *testing.T) {
	t.Parallel()
	b := newStandingBed(t, false)
	b.contract.Groups[0].ID = "fast-static-build"
	b.contract.Groups[0].Requires = []string{"refusal-register-standard"}
	b.contract.Groups[2].ID = "refusal-register-standard"
	b.contract.Groups[2].Inputs = []string{"metasystem/records/refusals/**"}
	b.contract.Surfaces[0].Standard = []string{"fast-static-build"}
	b.contract.Unknown = []string{"fast-static-build"}
	b.red.Failed[0].Unit = "refusal-register-standard"
	b.incidents[0].Identity = "red:refusal-register-standard"
	b.save(t, b.red)
	b.git.changed[[2]string{b.red.Tree, b.git.tree}] = "metasystem/plans/designs/page.md"
	b.setContract(t)
	r := b.detached(t)
	b.provenance(t, r, map[string]string{"fast-static-build": "ran", "refusal-register-standard": "ran", "unrelated": "unproved", "unit/cmd/metasystem": "standing"})
	if !strings.Contains(r.ScopeReason, "prerequisites repeated: refusal-register-standard") || len(r.Failed) != 1 || r.Failed[0].Unit != "metasystem/cmd/metasystem" {
		t.Fatalf("standing prerequisite suppressed %+v", r)
	}
}

func TestStandingRedCadenceStillRequestsFull(t *testing.T) {
	t.Parallel()
	b := newStandingBed(t, false)
	if err := os.WriteFile(filepath.Join(b.install, "metasystem.conf"), []byte("testing.contract=testing.json\nproof.trunk-every=1m\n"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := KeeperClock(lane.Record{Root: b.checkout, Install: b.install, CustodyEpoch: 1}, func(lane.Record) error { return nil }, b.seams)
	if err != nil {
		t.Fatal(err)
	}
	running, _, _, err := ReadRunning(b.install, b.seams)
	if err != nil || running.Admission.Scope != "full" || !running.Fresh {
		t.Fatalf("cadence inherited standing red %+v %v", running, err)
	}
	log, err := os.Create(running.Log)
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	r, err := Run(b.install, b.checkout, "fixture-proof", running.Attempt, log, b.seams)
	if err != nil || r.Scope != "full" || !r.CountedFull || len(b.calls) != 1 {
		t.Fatalf("cadence execution %+v %v", r, err)
	}
}
