package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testgoal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestLandingProveRecordsEachFlakeOnce(t *testing.T) {
	t.Parallel()
	for _, main := range []bool{false, true} {
		t.Run(fmt.Sprintf("main=%t", main), func(t *testing.T) {
			t.Parallel()
			b := newReplayVerbBed(t)
			ledger := newGoalCLIBed(t, goalCLISeed{checkout: b.install})
			ledger.setNow(laneTestNow)
			b.trunk = main
			conf := "metasystem.template=true\nproof.full=fixture\ntesting.contract=testing.json\n"
			if err := os.WriteFile(filepath.Join(b.install, "metasystem.conf"), []byte(conf), 0600); err != nil {
				t.Fatal(err)
			}
			names := []string{"TestOne/with/slashes", "TestTwo"}
			at := laneTestNow.Add(-2 * time.Hour).Format(time.RFC3339)
			legacy := goal.TrunkRedEntry{ID: "flaky:u/a", Identity: "flaky:u/a", Group: "u/a", Class: goal.TrunkRedClassPendingFlake, Status: "failed", Opened: at, Holds: []string{},
				Failures:  []goal.TrunkRedFailure{{Report: "legacy.log", Classname: "u/a", Name: names[0]}, {Report: "legacy.log", Classname: "u/a", Name: names[1]}},
				Sightings: []goal.TrunkRedSighting{{Attempt: "legacy-red", BaseCommit: "legacy", BaseTree: "legacy-tree", Where: "tip", Tree: "legacy-tree", LogPath: "legacy.log", SeenAt: at, Opid: goal.Opid("01J5X0000000000000000000F0", "lane", "fixture"), Rerun: &goal.TrunkRedRerun{Attempt: "legacy-green", LogPath: "legacy-green.log"}}}}
			allowance := legacy
			allowance.ID, allowance.Identity, allowance.Group, allowance.Class = "allowanced", "allowanced", "u/b", goal.TrunkRedClassKnownFlake
			allowance.Failures = []goal.TrunkRedFailure{{Report: "allowance.log", Classname: "u/b", Name: "TestLegacy"}}
			allowance.Owner = goal.TrunkRedOwner{Machine: "person-seat", Since: at, How: "approver", By: "Wido"}
			allowance.AllowanceUntil = laneTestNow.Add(time.Hour).Format(time.RFC3339)
			seed, err := ledger.repo.Files(ledger.tip(), "")
			if err != nil {
				t.Fatal(err)
			}
			seed["plans/goals/trunk-red.json"] = goal.RenderTrunkRed([]goal.TrunkRedEntry{legacy, allowance})
			ledger.repo = testgoal.New(seed, ledger.clock(), ledger.tip())
			endpoint, err := ledger.endpoint(b.install)
			if err != nil {
				t.Fatal(err)
			}
			read := func() []goal.TrunkRedEntry {
				t.Helper()
				entries, problems := goal.ParseTrunkRed([]byte(ledger.accepted("plans/goals/trunk-red.json")))
				if len(problems) != 0 {
					t.Fatal(problems)
				}
				return entries
			}
			contract := testpolicy.Contract{SchemaVersion: 1,
				ProjectRisk: testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
				Fallback:    "residual", Surfaces: []testpolicy.Surface{{ID: "unit", Paths: []string{"u/a/**"}, Standard: []string{"check"}}, {ID: "other", Paths: []string{"b/**", "metasystem/plans/goals/**"}}, {ID: "residual"}},
				Groups: []testpolicy.Group{{ID: "check", Kind: "unit", Adapter: "go", CWD: ".", Inputs: []string{"u/**"}, Platforms: []string{"any"}, TargetMS: 1, Packages: []string{"u/a"}, Tests: json.RawMessage(`"all"`)}},
				Always: testpolicy.Always{Canary: []string{"check"}}, Unknown: []string{"check"}, Cadence: []string{"check"}}
			data, err := json.Marshal(contract)
			if err != nil || contract.Validate() != nil {
				t.Fatalf("contract: %v %v", err, contract.Validate())
			}
			mainCommit := ledger.tip()
			changed, corrupt := "b/change.go", false
			invalidRegister := ""
			git := b.owners.landing.plainProve.Git
			b.owners.landing.plainProve.Git = func(dir string, args ...string) (string, error) {
				switch {
				case strings.Join(args, " ") == "rev-parse --verify origin/main^{commit}":
					return mainCommit, nil
				case len(args) == 2 && args[0] == "show" && args[1] == "origin/main:metasystem/testing.json":
					return string(data), nil
				case len(args) == 2 && args[0] == "show" && args[1] == "origin/main:metasystem/plans/goals/trunk-red.json":
					if corrupt {
						return "{", nil
					}
					if invalidRegister != "" {
						return invalidRegister, nil
					}
					return ledger.accepted("plans/goals/trunk-red.json"), nil
				case len(args) == 3 && args[0] == "diff" && args[1] == "--name-only":
					return changed, nil
				case strings.Join(args, " ") == "ls-tree -z origin/main:u/a":
					return "100644 blob abc\tunit.go\x00", nil
				case strings.Join(args, " ") == "ls-tree -z origin/main:u/b":
					return "100644 blob abc\tlegacy.go\x00", nil
				}
				return git(dir, args...)
			}
			b.owners.landing.plainProve.Judge = nil
			b.owners.landing.mainEndpoint = func(string) (goal.Endpoint, error) { return endpoint, nil }
			var request goal.VerbRequest
			var observation goal.FlakeRecordArgs
			publications := 0
			b.owners.landing.recordFlake = func(r goal.VerbRequest, args goal.FlakeRecordArgs) (goal.FlakeRecordResult, error) {
				request, observation = r, args
				before := ledger.tip()
				result, err := goal.RecordFlake(r, args)
				if ledger.tip() != before {
					publications++
				}
				return result, err
			}
			failure := &exec.ExitError{ProcessState: replayFalseState(t)}
			b.fail = func(_ *exec.Cmd, only string) (string, error) {
				if only == "" {
					return "LANDING-FAILED\tu/a\t" + strings.Join(names, " ") + "\nLANDING-CHECKED\t1\n", failure
				}
				return "LANDING-CHECKED\t0\n", nil
			}
			if !main {
				b.prepareBatch(t)
			}
			words := []string{"prove", "--wait", "--json"}
			if main {
				words = append(words, "--trunk")
			}
			code, out := b.run(t, b.root, words...)
			var proof struct{ Data plain.Result }
			if code != 0 || json.Unmarshal([]byte(out), &proof) != nil || proof.Data.Result != plain.Green || len(b.runs) != 2 || publications != 1 {
				t.Fatalf("public prove: exit=%d %s; runs=%v publications=%d", code, out, b.runs, publications)
			}
			entries := read()
			if len(entries) != 4 {
				t.Fatalf("two failed tests were not recorded separately: %+v", entries)
			}
			for _, name := range names {
				identityBytes, _ := json.Marshal([2]string{"u/a", name})
				key := fmt.Sprintf("flake:%x", sha256.Sum256(identityBytes))
				index := slices.IndexFunc(entries, func(e goal.TrunkRedEntry) bool { return e.Identity == key })
				if index < 0 {
					t.Fatalf("missing exact test identity %s", name)
				}
				entry := entries[index]
				if entry.TestUnit != "u/a" || entry.TestName != name || entry.Class != goal.TrunkRedClassFlake || entry.FixGoal != "fix-flaky-u-a" || entry.Closed != nil || entry.AllowanceUntil != "" || entry.Owner != (goal.TrunkRedOwner{}) || len(entry.Sightings) != 1 {
					t.Fatalf("test entry: %+v", entry)
				}
				s := entry.Sightings[0]
				if (s.Where == "") != main || s.BaseTree != proof.Data.Tree || s.Attempt != observation.Attempt || s.LogPath != observation.LogPath || s.Rerun == nil || s.Rerun.Attempt != observation.Rerun.Attempt || s.Rerun.LogPath != observation.Rerun.LogPath || s.Rerun.LogPath == s.LogPath {
					t.Fatalf("captured attempts and location: %+v observation=%+v", s, observation)
				}
				for _, log := range []string{s.LogPath, s.Rerun.LogPath} {
					if _, err := os.Stat(log); err != nil {
						t.Fatal(err)
					}
				}
			}
			fix, problems := goal.ParseFile([]byte(ledger.goalRecord("fix-flaky-u-a")))
			if len(problems) != 0 || fix.Approved != nil || fix.Claimed != nil || fix.State != goal.StateQueued || strings.Count(fix.NextStep, "Flaky:") != 1 {
				t.Fatalf("one unapproved fix: %+v %v", fix, problems)
			}
			before := ledger.tip()
			replay, err := goal.RecordFlake(request, observation)
			if err != nil || ledger.tip() != before || replay.Sightings != 2 || !reflect.DeepEqual(read(), entries) {
				t.Fatalf("replay added a sighting: %+v %v", replay, err)
			}
			peer := endpoint
			peer.Root = t.TempDir()
			if err := os.WriteFile(filepath.Join(peer.Root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0600); err != nil {
				t.Fatal(err)
			}
			peerRequest := request
			peerRequest.Endpoint = peer
			if replay, err := goal.RecordFlake(peerRequest, observation); err != nil || replay.Sightings != 2 || ledger.tip() != before || !reflect.DeepEqual(read(), entries) {
				t.Fatalf("replay from another journal added a sighting: %+v %v", replay, err)
			}
			facts := goal.FlakeFacts(entries)
			if len(facts) != 3 {
				t.Fatalf("legacy and per-test facts did not join: %+v", facts)
			}
			for _, fact := range facts {
				if fact.TestUnit == "u/a" && (len(fact.Sightings) != 2 || fact.Sightings[0].LogPath != "legacy.log" || fact.FixGoal != "fix-flaky-u-a") {
					t.Fatalf("legacy history or newer fix lost: %+v", fact)
				}
			}
			overlap := append([]goal.TrunkRedEntry(nil), entries...)
			legacyIndex := slices.IndexFunc(overlap, func(e goal.TrunkRedEntry) bool { return e.ID == legacy.ID })
			newIndex := slices.IndexFunc(overlap, func(e goal.TrunkRedEntry) bool { return e.Class == goal.TrunkRedClassFlake })
			// Retained package history can reference the same attempt as a test entry.
			overlap[legacyIndex].Sightings = append(append([]goal.TrunkRedSighting(nil), legacy.Sightings...), entries[newIndex].Sightings[0])
			joined := goal.FlakeFacts(overlap)
			for i := range facts {
				if !reflect.DeepEqual(joined[i].TrunkRedEntry, facts[i].TrunkRedEntry) || len(joined[i].Sources) != len(facts[i].Sources) {
					t.Fatalf("overlapping legacy evidence counted twice or hid current state: %+v", joined)
				}
			}
			if !slices.ContainsFunc(joined, func(f goal.FlakeFact) bool {
				return slices.ContainsFunc(f.Sources, func(e goal.TrunkRedEntry) bool {
					return e.ID == allowance.ID && e.Owner == allowance.Owner && e.AllowanceUntil == allowance.AllowanceUntil
				})
			}) {
				t.Fatal("the compatibility reader lost the legacy allowance and owner")
			}
			slices.Reverse(overlap)
			if reversed := goal.FlakeFacts(overlap); !reflect.DeepEqual(reversed, joined) {
				t.Fatalf("record order hid current state: %+v", reversed)
			}
			judge := landingFlakeJudge(b.install, b.owners.landing.plainProve.Git)
			failed := []plain.FailedUnit{{Unit: "u/a", Tests: names}, {Unit: "u/b", Tests: []string{"TestLegacy"}}}
			judged, err := judge(b.root, proof.Data.Commit, failed)
			if err != nil || !judged["u/a"].Known || judged["u/a"].Affected || !judged["u/b"].Known {
				t.Fatalf("judge did not accept new and allowanced facts: %+v %v", judged, err)
			}
			changed = "u/a/change.go"
			judged, err = judge(b.root, proof.Data.Commit, failed)
			if err != nil || !judged["u/a"].Affected {
				t.Fatalf("affected test received an unaffected judgement: %+v %v", judged, err)
			}
			changed = "unowned/file"
			uncertainContract := contract
			uncertainContract.Fallback, uncertainContract.Surfaces = "", contract.Surfaces[:2]
			data, _ = json.Marshal(uncertainContract)
			judged, err = judge(b.root, proof.Data.Commit, failed)
			if err != nil || !judged["u/a"].Affected {
				t.Fatalf("uncertain surface granted an unaffected judgement: %+v %v", judged, err)
			}
			data, _ = json.Marshal(contract)
			invalidRegister = string(goal.RenderTrunkRed(slices.DeleteFunc(append([]goal.TrunkRedEntry(nil), entries...), func(e goal.TrunkRedEntry) bool { return e.ID == legacy.ID })))
			judged, err = judge(b.root, proof.Data.Commit, failed)
			if err != nil || !judged["u/a"].Known {
				t.Fatalf("test-keyed facts required a legacy package row: %+v %v", judged, err)
			}
			invalidRegister = ""
			changed = "b/change.go"
			unknown, err := judge(b.root, proof.Data.Commit, []plain.FailedUnit{{Unit: "u/a", Tests: []string{"TestUnknown"}}})
			if err != nil || unknown["u/a"].Known {
				t.Fatalf("unnamed history granted repeat permission: %+v %v", unknown, err)
			}
			code, text, stderr := ledger.public("incident", "list", "--json")
			var listed struct {
				Data struct {
					Incidents []goal.TrunkRedEntry `json:"incidents"`
				}
			}
			want := 0
			if main {
				want = 2
			}
			if code != 0 || json.Unmarshal([]byte(text), &listed) != nil || len(listed.Data.Incidents) != want {
				t.Fatalf("public incidents: exit=%d %s %s", code, text, stderr)
			}
			ledger.setNow(laneTestNow.Add(time.Minute))
			key := goal.FlakeIdentity("u/a", names[0])
			if code, text, stderr := ledger.public("incident", "close", key, "--reason", "Fixed the intermittent test."); code != 0 {
				t.Fatalf("person close: exit=%d %s %s", code, text, stderr)
			}
			judged, err = judge(b.root, proof.Data.Commit, failed)
			if err != nil || judged["u/a"].Known {
				t.Fatalf("older legacy history hid the test closure: %+v %v", judged, err)
			}
			remaining, err := judge(b.root, proof.Data.Commit, []plain.FailedUnit{{Unit: "u/a", Tests: names[1:]}})
			if err != nil || !remaining["u/a"].Known {
				t.Fatalf("closing one test closed its sibling: %+v %v", remaining, err)
			}
			later, recurrence := request, observation
			later.Ulid, later.Now = "01J5X0000000000000000000F9", laneTestNow.Add(2*time.Minute)
			recurrence.Tests, recurrence.Attempt, recurrence.Rerun.Attempt = names[:1], "later-red", "later-green"
			if _, err := goal.RecordFlake(later, recurrence); err != nil {
				t.Fatal(err)
			}
			judged, err = judge(b.root, proof.Data.Commit, failed)
			if err != nil || !judged["u/a"].Known {
				t.Fatalf("older closure hid a later sighting: %+v %v", judged, err)
			}
			current := read()
			for _, old := range []goal.TrunkRedEntry{legacy, allowance} {
				index := slices.IndexFunc(current, func(e goal.TrunkRedEntry) bool { return e.ID == old.ID })
				if index < 0 || !reflect.DeepEqual(current[index], old) {
					t.Fatalf("legacy entry changed: %+v", current)
				}
			}
			for _, damage := range []func(*goal.TrunkRedEntry){
				func(e *goal.TrunkRedEntry) { e.TestName = "WrongTest" },
				func(e *goal.TrunkRedEntry) { e.AllowanceUntil = laneTestNow.Add(time.Hour).Format(time.RFC3339) },
				func(e *goal.TrunkRedEntry) { e.Sightings[0].Rerun = nil },
				func(e *goal.TrunkRedEntry) { e.Sightings[0].Rerun.Attempt = e.Sightings[0].Attempt },
				func(e *goal.TrunkRedEntry) { e.Sightings[0].LogPath = "" },
			} {
				var broken []goal.TrunkRedEntry
				data, _ := json.Marshal(current)
				if err := json.Unmarshal(data, &broken); err != nil {
					t.Fatal(err)
				}
				index := slices.IndexFunc(broken, func(e goal.TrunkRedEntry) bool { return e.ID == key })
				damage(&broken[index])
				invalidRegister = string(goal.RenderTrunkRed(broken))
				judged, err = judge(b.root, proof.Data.Commit, failed)
				if err == nil || judged["u/a"].Known {
					t.Fatalf("invalid fact granted repeat permission: %+v %v", judged, err)
				}
			}
			invalidRegister = ""
			corrupt = true
			judged, err = judge(b.root, proof.Data.Commit, failed)
			if err == nil || judged["u/a"].Known {
				t.Fatalf("unreadable history authorized a repeat: %+v %v", judged, err)
			}
		})
	}
}
