package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/designgate"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batchowner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot/stateroottest"
)

// A design refusal returns the hand-in and holds its commit out of main.
func TestLandingPushChecksDesign(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"warn", "refuse"} {
		failures := []string{"", "chain", "record", "record-changed", "refs", "containment"}
		if mode == "warn" {
			failures = append(failures, "unproven")
		}
		for _, failure := range failures {
			t.Run(mode+"/"+failure, func(t *testing.T) {
				t.Parallel()
				bed := newDesignGateBed(t, 2)
				setDesignGateMode(t, bed, mode)
				page, data := designGatePage(t, bed, "- Critique: closed at round 2 on 0 material findings (WHO)\n")
				designGateBuild(t, bed, "u")
				if failure != "record" {
					if err := os.WriteFile(page, append(data, []byte("A changed approach.\n")...), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				if failure == "chain" {
					bed.designGate.chains = func(string, string, string) ([]designgate.Chain, error) { return nil, errors.New("chain unavailable") }
				}
				laneHome := t.TempDir()
				owners := bed.workOwners()
				layout, err := owners.resolver.ResolveLayout(bed.root())
				if err != nil {
					t.Fatal(err)
				}
				install := string(layout.InstallationRoot)
				policyPath := filepath.Join(t.TempDir(), "lane-policy.conf")
				if err := os.WriteFile(policyPath, []byte("landing.on-red=auto\n"), 0600); err != nil {
					t.Fatal(err)
				}
				owners.policies = config.PolicyReaders{
					Registry: func(string) (config.PolicyRegistry, error) { return config.PolicyRegistry{Lane: layout.GitRoot}, nil },
					ConfPath: func(string) (string, error) { return policyPath, nil },
					Helm:     func(string) helm.State { return helm.State{} },
				}
				if err := os.MkdirAll(filepath.Join(layout.GitRoot, ".git"), 0755); err != nil {
					t.Fatal(err)
				}
				for _, goal := range []string{bed.id, "already-on-main", "not-in-head"} {
					if _, _, err := plain.HandIn(install, plain.Line{Goal: goal, SHA: goal, At: laneTestNow.Format(time.RFC3339)}); err != nil {
						t.Fatal(err)
					}
				}
				var stdout, stderr bytes.Buffer
				inv := &intentInvocation{command: landingPushCommand(), input: intentInput{values: map[string][]string{"json": {"true"}}},
					owners: owners, layout: layout, stateRoot: bed.stateRoot(), stdout: &stdout, stderr: &stderr, cwd: layout.GitRoot}
				// Push rechecks the registered lane and trunk policy (lane-reads-its-policies.md:145).
				record := lane.Record{Root: layout.GitRoot, Install: install, CustodyEpoch: 1, RegisteredBy: "Wido"}
				registration, err := json.Marshal(record)
				helmMust(t, err)
				helmMust(t, os.MkdirAll(lane.HostDir(laneHome), 0700))
				helmMust(t, os.WriteFile(lane.RecordPath(laneHome), registration, 0600))
				laneLayout, err := record.Layout()
				if err != nil {
					t.Fatal(err)
				}
				pushes, reads := 0, 0
				push := func(_ string, _ string, _ time.Time, before func(old, head string) error) (plain.PushOutcome, error) {
					outcome := plain.PushOutcome{Old: "main", Commit: "head"}
					if failure == "refs" {
						return outcome, errors.New("refs unavailable")
					}
					if failure == "unproven" {
						return outcome, &plain.Refusal{Code: plain.CodeUnproven, Reason: "HEAD's tree was never proven, so nothing was pushed"}
					}
					if err := before(outcome.Old, outcome.Commit); err != nil {
						return outcome, err
					}
					pushes++
					outcome.Changed = true
					return outcome, nil
				}
				notAncestor := replayFalseState(t)
				admitted := laneAdmitted{home: laneHome, record: record, layout: laneLayout, installation: install, owners: laneVerbOwners{now: func() time.Time { return laneTestNow }, machine: func(string) (string, error) { return "fixture", nil }, push: push, plainProve: plain.ProveSeams{Git: func(_ string, args ...string) (string, error) {
					if args[0] == "rev-parse" {
						return "main", nil
					}
					if args[0] == "cat-file" {
						return "", nil
					}
					if args[0] == "merge-base" {
						return "", &exec.ExitError{ProcessState: notAncestor}
					}
					if args[0] == "ls-tree" {
						return "", nil
					}
					t.Fatalf("unexpected Git: %v", args)
					return "", nil
				}}}}
				effects := landingPushOwners{
					contains: func(_, ref string) func(string) (bool, error) {
						if ref != "head" && ref != "main" {
							t.Fatalf("checked a ref other than the push's fetched commits: %q", ref)
						}
						return func(sha string) (bool, error) {
							if failure == "containment" && sha == bed.id {
								return false, errors.New("containment unavailable")
							}
							return sha == "already-on-main" || ref == "head" && sha == bed.id, nil
						}
					},
					facts: func(id string) landing.DesignFacts {
						reads++
						if id != bed.id {
							t.Fatalf("checked an excluded hand-in: %s", id)
						}
						return inv.landingDesignFacts(inv.layout.InstallationRoot.Path(), id)
					},
					notify: func(plain.PushOutcome) error { return nil },
				}
				if strings.HasPrefix(failure, "record") {
					effects.record = func(string, plain.DesignCheck) error { return errors.New("record unavailable") }
				}
				code := runIntentLandingPushWithOwners(inv, admitted, effects)
				var result intentResult
				if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
					t.Fatal(err)
				}
				refuses := mode == "refuse" && (failure == "" || failure == "record-changed")
				if refuses {
					if !strings.Contains(strings.Join(result.Details, " "), "refused because: LANDING_DESIGN_NOT_STANDING governed-by=R-146-m1k") {
						t.Fatalf("refusal has no governing ruling: %+v", result)
					}
					if code != 1 || pushes != 0 || !strings.HasSuffix(result.Summary, "; nothing was pushed") || result.Next == nil || strings.Join(result.Next.Argv, " ") != "git -C "+layout.GitRoot+" checkout --detach origin/main" || !strings.Contains(result.Next.Reason, "git merge --no-ff SHA for each waiting sha, then metasystem landing prove") {
						t.Fatalf("refusal: code=%d pushes=%d result=%+v", code, pushes, result)
					}
					entry, ok, err := plain.Latest(install, bed.id)
					if err != nil || !ok || entry.State != plain.StateReturned || entry.Cause == nil || entry.Cause.Kind != "own" || entry.Cause.Goal != bed.id || entry.Cause.SHA != bed.id || entry.Cause.Evidence != entry.Reason || entry.Reason == "" {
						t.Fatalf("design refusal did not return its own hand-in with evidence: %+v ok=%v err=%v", entry, ok, err)
					}
					for _, goal := range []string{"already-on-main", "not-in-head"} {
						entry, ok, err := plain.Latest(install, goal)
						if err != nil || !ok || entry.State != plain.StateWaiting {
							t.Fatalf("excluded hand-in was changed: %+v ok=%v err=%v", entry, ok, err)
						}
					}
				} else if failure == "refs" || failure == "unproven" {
					if code != 1 || pushes != 0 || stderr.Len() != 0 || reads != 0 {
						t.Fatalf("push checked designs before its own checks passed: code=%d pushes=%d reads=%d output=%q", code, pushes, reads, stderr.String())
					}
				} else if code != 0 || pushes != 1 {
					t.Fatalf("push did not continue: code=%d pushes=%d output=%s", code, pushes, stderr.String())
				}
				checks, err := plain.DesignChecks(install)
				if err != nil {
					t.Fatal(err)
				}
				if failure == "refs" || failure == "unproven" {
					if len(checks) != 0 {
						t.Fatalf("recorded designs for a push that failed its own checks: %+v", checks)
					}
					if _, err := os.Stat(filepath.Join(plain.Dir(install), "design-gate.jsonl")); !os.IsNotExist(err) {
						t.Fatalf("created a design check file for a refused push: %v", err)
					}
				} else if strings.HasPrefix(failure, "record") {
					lines := 2
					if failure == "record-changed" && !refuses {
						lines = 4
					}
					if len(checks) != 0 || !strings.Contains(stderr.String(), strings.TrimSuffix(failure, "-changed")+" unavailable") || strings.Count(stderr.String(), "\n") != lines {
						t.Fatalf("failure pair or record: checks=%+v output=%q", checks, stderr.String())
					}
				} else {
					want := "design-changed"
					if failure != "" {
						want = "unchecked"
					}
					if len(checks) != 1 || checks[0].Goal != bed.id || checks[0].Commit != bed.id || checks[0].Verdict != want || checks[0].At != laneTestNow.Format(time.RFC3339) {
						t.Fatalf("lane record: %+v", checks)
					}
					if refuses {
						entry, _, err := plain.Latest(install, bed.id)
						if err != nil || entry.Cause.Evidence != checks[0].Reason {
							t.Fatalf("return lost the design-check reason: %+v check=%+v err=%v", entry, checks[0], err)
						}
					}
					if !refuses && (!strings.Contains(stderr.String(), checks[0].Reason+"\nmetasystem design ") || strings.Count(stderr.String(), "\n") != 2) {
						t.Fatalf("warning pair: %q", stderr.String())
					}
				}
				wantReads := 1
				if failure == "refs" || failure == "containment" || failure == "unproven" {
					wantReads = 0
				}
				if reads != wantReads {
					t.Fatalf("facts read %d times; want %d", reads, wantReads)
				}
				if refuses {
					before := idemTreeDigest(t, plain.Dir(install))
					rebuildNext := *result.Next
					stdout.Reset()
					stderr.Reset()
					result = intentResult{}
					code = runIntentLandingPushWithOwners(inv, admitted, effects)
					if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
						t.Fatal(err)
					}
					if code != 1 || result.Outcome != intentRefused || pushes != 0 || !strings.Contains(result.Summary, "returned goal "+bed.id) {
						t.Fatalf("unchanged HEAD landed a design-refused goal: code=%d pushes=%d result=%+v", code, pushes, result)
					}
					if result.Next == nil || strings.Join(result.Next.Argv, " ") != strings.Join(rebuildNext.Argv, " ") || result.Next.Reason != rebuildNext.Reason {
						t.Fatalf("repeat refusal did not name the same rebuild: %+v", result)
					}
					idemSameTree(t, "push of a returned commit", before, idemTreeDigest(t, plain.Dir(install)))
				}
			})
		}
	}
}

var laneTestNow = time.Date(2026, 9, 29, 18, 0, 0, 0, time.UTC)

// laneBed is a host with a lane home, two nested landing checkouts and two
// seats whose settings the test writes.
type laneBed struct {
	home, landingA, landingB, seatA, seatB string
	seams                                  batchowner.LandingLaneSeams
}

func newLaneBed(t *testing.T) *laneBed {
	t.Helper()
	base := t.TempDir()
	bed := &laneBed{home: filepath.Join(base, "home"), landingA: filepath.Join(base, "landing-a"), landingB: filepath.Join(base, "landing-b"),
		seatA: filepath.Join(base, "seat-a"), seatB: filepath.Join(base, "seat-b")}
	for _, dir := range []string{bed.home, bed.landingA, bed.landingB, bed.seatA, bed.seatB} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, seat := range []string{bed.seatA, bed.seatB} {
		if err := os.WriteFile(filepath.Join(seat, "metasystem.conf"), []byte("metasystem.runtimes=claude\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	bed.home, bed.landingA, bed.landingB = realpath.Resolve(bed.home), realpath.Resolve(bed.landingA), realpath.Resolve(bed.landingB)
	landingCheckout(t, bed.landingA)
	landingCheckout(t, bed.landingB)
	bed.seams = batchowner.LandingLaneSeams{Home: func() (string, error) { return bed.home, nil }}
	return bed
}

func (bed *laneBed) setRoot(t *testing.T, seat, root string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(seat, "metasystem.conf.local"), []byte("landing.batch-root="+root+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A person registers the lane; a seat naming it and a seat naming nothing
// both land through it.
func TestBatchRootServesThePersonsLane(t *testing.T) {
	t.Parallel()
	bed := newLaneBed(t)
	registerLane(t, bed.home, bed.landingA, "Wido", laneTestNow)
	bed.setRoot(t, bed.seatA, bed.landingA)
	for _, seat := range []string{bed.seatA, bed.seatB} {
		root, configured, err := bed.seams.BatchRoot(seat, laneTestNow)
		if err != nil || !configured || root != bed.landingA {
			t.Fatalf("seat %s = %q %v %v; want the person's lane %s", seat, root, configured, err, bed.landingA)
		}
	}
	record, ok, err := lane.Read(bed.home)
	if err != nil || !ok || record.Root != bed.landingA || record.RegisteredBy != "Wido" || record.Install != filepath.Join(bed.landingA, "metasystem") {
		t.Fatalf("record = %+v %v %v", record, ok, err)
	}
}

// work land asks lane.Resolve whether a lane is registered, and nothing
// more (simple lane, unit C): a lane a person is unsetting or has stopped
// is still the lane, so work land refuses beside it until the unset ends.
// The resolution is the production one over a real home.
func TestWorkLandAsksOnlyWhetherALaneIsRegistered(t *testing.T) {
	t.Parallel()
	bed := newLaneBed(t)
	registerLane(t, bed.home, bed.landingA, "Wido", laneTestNow)
	seams := lane.UnsetSeams{
		Settle: func(lane.Layout) (lane.Settlement, error) {
			return lane.Settlement{Live: []string{"the landing agent runs"}}, nil
		},
	}
	if report, err := lane.Unset(bed.home, "Wido", laneTestNow, false, seams); err != nil || report.Stopped != lane.StepSettled {
		t.Fatalf("unset = %+v %v", report, err)
	}
	if _, paused := lane.ReadPause(bed.home); !paused {
		t.Fatal("the unset's fence did not stop the lane")
	}
	production := batchowner.ProductionLandingLaneSeams()
	production.Home = func() (string, error) { return bed.home, nil }
	inv := &intentInvocation{layout: stateroot.Layout{InstallationRoot: stateroottest.Installation(t, bed.seatA)}, owners: intentOwners{delivery: &intentDeliveryOwners{
		laneRoot: production.BatchRoot, now: func() time.Time { return laneTestNow }}}}
	refused := inv.laneRegistered(nil)
	if refused == nil || refused.Summary != "this computer has a landing lane, which lands only a goal's branch; nothing was landed" {
		t.Fatalf("work land on a registered lane = %+v; want the lane's refusal and no other question asked", refused)
	}
}

// Neither the seat nor the host names a lane: today's answer, not configured.
func TestBatchRootWithoutAnyLaneIsNotConfigured(t *testing.T) {
	t.Parallel()
	bed := newLaneBed(t)
	if root, configured, err := bed.seams.BatchRoot(bed.seatA, laneTestNow); err != nil || configured || root != "" {
		t.Fatalf("no lane = %q %v %v", root, configured, err)
	}
	if _, ok, _ := lane.Read(bed.home); ok {
		t.Fatalf("a seat without a setting registered a lane")
	}
}

// A seat naming another checkout is refused by work land in plain words,
// naming both paths and the one fix, and nothing is joined.
func TestWorkLandRefusesASeatWhoseRootIsNotTheHostLane(t *testing.T) {
	t.Parallel()
	bed := newLaneBed(t)
	registerLane(t, bed.home, bed.landingA, "seat-a", laneTestNow)
	bed.setRoot(t, bed.seatB, bed.landingB)
	inv := &intentInvocation{layout: stateroot.Layout{InstallationRoot: stateroottest.Installation(t, bed.seatB)}, owners: intentOwners{delivery: &intentDeliveryOwners{
		laneRoot: bed.seams.BatchRoot, now: func() time.Time { return laneTestNow }}}}
	refused := inv.laneRegistered(nil)
	if refused == nil || refused.Outcome != intentRefused {
		t.Fatalf("seat B landed through another lane: %+v", refused)
	}
	for _, want := range []string{bed.landingA, bed.landingB, "registered by seat-a"} {
		if !strings.Contains(refused.Summary, want) {
			t.Errorf("summary %q lacks %q", refused.Summary, want)
		}
	}
	if strings.Contains(refused.Summary, lane.CodeMismatch) || !strings.Contains(strings.Join(refused.Details, " "), lane.CodeMismatch) {
		t.Errorf("the code %s belongs in the details, not line 1: %+v", lane.CodeMismatch, refused)
	}
	for _, want := range []string{"metasystem settings set landing.batch-root " + bed.landingA, "metasystem landing set " + bed.landingB} {
		if !strings.Contains(refused.Decision, want) {
			t.Errorf("decision %q lacks %q", refused.Decision, want)
		}
	}
}

// With no home for the lane, a seat keeps its own setting, as before U12:
// there is no host state, so nothing is registered, gated or kept.
func TestBatchRootWithoutALaneHomeKeepsTheSeatSetting(t *testing.T) {
	t.Parallel()
	bed := newLaneBed(t)
	if err := os.WriteFile(filepath.Join(bed.landingA, "metasystem.conf"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	bed.setRoot(t, bed.seatA, bed.landingA)
	noHome := func() (string, error) { return "", errors.New("no home") }
	seams := batchowner.LandingLaneSeams{Home: noHome}
	root, configured, err := seams.BatchRoot(bed.seatA, laneTestNow)
	if err != nil || !configured || realpath.Resolve(root) != bed.landingA {
		t.Fatalf("seat without a lane home = %q %v %v; want its own setting", root, configured, err)
	}
}

// Only a person's landing set registers a lane (design r10 §1): a seat whose
// landing.batch-root names a checkout, on a computer with no registered
// lane, writes no host record and lands its own work.
func TestSeatSettingNeverRegisters(t *testing.T) {
	t.Parallel()
	bed := newLaneBed(t)
	bed.setRoot(t, bed.seatA, bed.landingA)
	root, configured, err := bed.seams.BatchRoot(bed.seatA, laneTestNow)
	if err != nil || configured || root != "" {
		t.Fatalf("seat naming a lane on a computer with none = %q %v %v; want no lane, so the seat lands itself", root, configured, err)
	}
	if _, ok, _ := lane.Read(bed.home); ok {
		t.Fatalf("a seat's landing.batch-root registered the host's lane")
	}
}

// At cutover every seat meets an older engine's lane record: work land
// says so in plain words on line 1, names the one command on line 2, and
// keeps the code for --verbose and --json.
func TestWorkLandNamesLandingSetForAnOlderLaneRecord(t *testing.T) {
	t.Parallel()
	bed := newLaneBed(t)
	if err := os.MkdirAll(lane.HostDir(bed.home), 0o700); err != nil {
		t.Fatal(err)
	}
	old := `{"root":"` + bed.landingA + `","registeredBy":"m1e","at":"2026-09-29T18:00:00Z"}`
	if err := os.WriteFile(lane.RecordPath(bed.home), []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	inv := &intentInvocation{layout: stateroot.Layout{InstallationRoot: stateroottest.Installation(t, bed.seatA)}, owners: intentOwners{delivery: &intentDeliveryOwners{
		laneRoot: bed.seams.BatchRoot, now: func() time.Time { return laneTestNow }}}}
	refused := inv.laneRegistered(nil)
	if refused == nil || strings.Contains(refused.Summary, lane.CodeRecordIncomplete) || !strings.Contains(refused.Summary, "older engine") ||
		strings.Join(refused.next, " ") != "metasystem landing set "+bed.landingA || !strings.Contains(strings.Join(refused.Details, " "), lane.CodeRecordIncomplete) {
		t.Fatalf("work land on an older lane record = %+v", refused)
	}
}
