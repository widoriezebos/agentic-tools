package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

func incidentRecorderFixture(t *testing.T, b *intentBed, machine string) laneVerbOwners {
	t.Helper()
	return laneVerbOwners{now: func() time.Time { return syncRequestTestNow },
		machine: func(string) (string, error) { return machine, nil }, mainEndpoint: func(string) (goal.Endpoint, error) { return b.dependencies().endpoint(b.root()) }}
}

func incidentCheckFixture(t *testing.T, tests ...string) plain.Result {
	t.Helper()
	log := filepath.Join(t.TempDir(), "main-unit.log")
	if err := os.WriteFile(log, []byte("unit failed on main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return plain.Result{Result: plain.Red, Commit: "main", Tree: "main-tree", Attempt: "check-main", Log: log,
		Failed: []plain.FailedUnit{{Unit: "internal/unit", Tests: tests}}}
}

// incidentProveFixture drives the lane command while its incident register uses
// an isolated in-memory main endpoint. Git and the check command are per-test effects.
func incidentProveFixture(t *testing.T, unchanged bool) (*replayVerbBed, *intentBed, func(string, string)) {
	t.Helper()
	lane := newReplayVerbBed(t)
	register := newIntentBed(t, false, nil)
	owners := incidentRecorderFixture(t, register, "finder-one")
	lane.owners.landing.mainEndpoint, lane.owners.landing.machine = owners.mainEndpoint, owners.machine
	lane.owners.landing.now = owners.now
	mainCommit, mainTree := "main", "main-tree"
	git := lane.owners.landing.plainProve.Git
	lane.owners.landing.plainProve.Git = func(dir string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		switch {
		case joined == "rev-parse --verify --quiet refs/remotes/origin/main^{commit}":
			return mainCommit, nil
		case joined == "rev-parse --verify "+mainCommit+"^{tree}":
			return mainTree, nil
		case joined == "rev-parse --verify HEAD^{tree}" && unchanged:
			return mainTree, nil
		case args[0] == "log":
			if unchanged {
				return "", nil
			}
			text, err := git(dir, args...)
			return strings.ReplaceAll(text, " main ", " "+mainCommit+" "), err
		default:
			return git(dir, args...)
		}
	}
	move := func(commit, tree string) {
		mainCommit, mainTree = commit, tree
		if unchanged {
			lane.head = commit
		} else {
			lane.head = "merge-b-ledger"
		}
	}
	if unchanged {
		lane.head = mainCommit
	}
	lane.fail = func(_ *exec.Cmd, only string) (string, error) {
		switch only {
		case "u/a":
			return "LANDING-FAILED\tu/a\tTestOne TestTwo\nLANDING-CHECKED\t1\n", errors.New("red")
		case "u/b":
			return "LANDING-FAILED\tu/b\tTestThree\nLANDING-CHECKED\t1\n", errors.New("red")
		case "u/green":
			return "LANDING-CHECKED\t0\n", nil
		default:
			return "LANDING-FAILED\tu/a\tTestOne TestTwo\nLANDING-FAILED\tu/green\tTestBatchOnly\nLANDING-FAILED\tu/b\tTestThree\nLANDING-CHECKED\t3\n", errors.New("red")
		}
	}
	return lane, register, move
}

func TestIncidentLandingProveListsEachMainFailureAndKeepsMainUnchangedOnRepeat(t *testing.T) {
	t.Parallel()
	for _, unchanged := range []bool{false, true} {
		t.Run(fmt.Sprintf("unchanged main=%v", unchanged), func(t *testing.T) {
			t.Parallel()
			lane, b, move := incidentProveFixture(t, unchanged)
			before := b.repo.canonical
			check := lane.prove(t)
			if check.Cause == nil || check.Cause.Kind != "main" || check.Scope != "full" || !check.CountedFull || b.repo.canonical == before {
				t.Fatalf("main's red was not recorded: %+v tip=%s", check, b.repo.canonical)
			}
			code, result := b.runJSON(b.owners(), "incident", "list")
			data, _ := json.Marshal(result.Data)
			var listed struct{ Incidents []goal.TrunkRedEntry }
			want := map[string]string{"TestOne": "u/a", "TestTwo": "u/a", "TestThree": "u/b"}
			if unchanged {
				want["TestBatchOnly"] = "u/green"
			}
			if err := json.Unmarshal(data, &listed); err != nil || code != 0 || len(listed.Incidents) != len(want) {
				t.Fatalf("list: code=%d data=%s err=%v", code, data, err)
			}
			for _, entry := range listed.Incidents {
				if len(entry.Failures) != 1 || len(entry.Sightings) != 1 {
					t.Fatalf("test evidence lost: %+v", entry)
				}
				test := entry.Failures[0].Name
				unit, found := want[test]
				log := check.Log
				attempt := check.Attempt
				if !unchanged {
					index := 1
					if unit == "u/b" {
						index = 3
					}
					attempt += "-replay-1"
					log = filepath.Join(plain.Dir(lane.install), "proofs", fmt.Sprintf("%s-%d.log", attempt, index))
				}
				contents, err := os.ReadFile(log)
				if err != nil || !found || entry.Identity != "red:"+unit+":"+test || entry.Group != unit || entry.Failures[0].Report != log || entry.Owner != (goal.TrunkRedOwner{}) || entry.Sightings[0].BaseCommit != "main" || entry.Sightings[0].BaseTree != "main-tree" || entry.Sightings[0].Batch != attempt || entry.Sightings[0].LogPath != log || entry.Sightings[0].LogDigest != fmt.Sprintf("%x", sha256.Sum256(contents)) {
					t.Fatalf("test's own main evidence lost: %+v log=%s err=%v", entry, log, err)
				}
				delete(want, test)
			}
			if len(want) != 0 {
				t.Fatalf("missing tests: %v", want)
			}
			code, stdout, stderr := b.run(b.owners(), "incident", "list")
			stdout = strings.Join(strings.Fields(stdout), " ")
			if code != 0 || !strings.Contains(stdout, "; test TestOne; evidence: ") || !strings.Contains(stdout, "; test TestTwo; evidence: ") || !strings.Contains(stdout, "; test TestThree; evidence: ") {
				t.Fatalf("test and log absent from the public list: %d %s %s", code, stdout, stderr)
			}
			// A ledger move changes main's commit while the same failures stand.
			parent := b.repo.canonical
			backlog := append(bytes.Clone(b.repo.commit(parent).files["plans/goals/backlog.md"]), '\n')
			tip, err := b.repo.Build(goal.Opid("01J5X0000000000000000000X2", "mac-cli", "m1"), parent,
				[]goal.Change{{Path: "plans/goals/backlog.md", Content: backlog}}, "ledger move fixture")
			if err != nil {
				t.Fatal(err)
			}
			if outcome, err := b.repo.Publish(parent, tip); err != nil || outcome != goal.CASLanded {
				t.Fatalf("ledger move: %v %v", outcome, err)
			}
			if err := b.repo.AcceptedCAS(parent, tip); err != nil {
				t.Fatal(err)
			}
			move(tip, "ledger-tree")
			if check := lane.prove(t); check.Cause.Kind != "main" || b.repo.canonical != tip {
				t.Fatalf("repeated proof moved main: %+v tip=%s want=%s", check, b.repo.canonical, tip)
			}
			_, result = b.runJSON(b.owners(), "incident", "list")
			data, _ = json.Marshal(result.Data)
			if err := json.Unmarshal(data, &listed); err != nil {
				t.Fatal(err)
			}
			for _, entry := range listed.Incidents {
				if len(entry.Sightings) != 1 {
					t.Fatalf("repeated proof appended a sighting: %+v", entry)
				}
			}
		})
	}
}

func TestIncidentLandingProveRecordingFailureKeepsRedAndReportsReason(t *testing.T) {
	t.Parallel()
	for _, unchanged := range []bool{false, true} {
		t.Run(fmt.Sprintf("unchanged main=%v", unchanged), func(t *testing.T) {
			t.Parallel()
			lane, b, _ := incidentProveFixture(t, unchanged)
			tip := b.repo.canonical
			calls := 0
			lane.owners.landing.recordMain = func(goal.VerbRequest, goal.TrunkRedRecordArgs) (goal.PublishResult, error) {
				calls++
				return goal.PublishResult{}, errors.New("incident store unavailable")
			}
			result := lane.prove(t)
			if result.Cause.Kind != "main" || !result.CountedFull || result.Repeat != "" || calls != 1 || b.repo.canonical != tip || !strings.Contains(result.Reason, "the proving command ended: red") || !strings.Contains(result.Reason, "incident store unavailable") {
				t.Fatalf("recording changed or hid the red: %+v calls=%d tip=%s", result, calls, b.repo.canonical)
			}
			last, found, err := plain.LastResult(lane.install)
			if err != nil || !found || last.Reason != result.Reason || last.Result != plain.Red {
				t.Fatalf("recorded result hid the incident error: %+v found=%v err=%v", last, found, err)
			}
		})
	}
}

func TestIncidentRecorderAddsNewTestWithoutRepeatingExisting(t *testing.T) {
	t.Parallel()
	b := newIntentBed(t, false, nil)
	owners := incidentRecorderFixture(t, b, "finder-one")
	check := incidentCheckFixture(t, "TestOne", "TestTwo")
	if err := owners.proveSeams(b.root()).RecordMain([]plain.Result{check}); err != nil {
		t.Fatal(err)
	}
	tip := b.repo.canonical
	owners.machine = func(string) (string, error) { return "finder-two", nil }
	check.Commit, check.Tree, check.Attempt = "later-main", "ledger-tree", "later-check"
	if err := owners.proveSeams(b.root()).RecordMain([]plain.Result{check}); err != nil || b.repo.canonical != tip {
		t.Fatalf("another finder repeated the same failures: %v tip=%s want=%s", err, b.repo.canonical, tip)
	}
	check.Failed[0].Tests = []string{"TestOne", "TestThree"}
	if err := owners.proveSeams(b.root()).RecordMain([]plain.Result{check}); err != nil {
		t.Fatal(err)
	}
	_, result := b.runJSON(b.owners(), "incident", "list")
	data, _ := json.Marshal(result.Data)
	var listed struct{ Incidents []goal.TrunkRedEntry }
	if err := json.Unmarshal(data, &listed); err != nil || len(listed.Incidents) != 3 {
		t.Fatalf("later test in the same unit was hidden: %s %v", data, err)
	}
	for _, entry := range listed.Incidents {
		if len(entry.Sightings) != 1 {
			t.Fatalf("mixed new and repeated failures appended a repeat: %+v", entry)
		}
	}
}

func TestIncidentLandingProveRecordsOnlyMainFailures(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"own", "incomplete main", "not run", "green"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			lane, b, _ := incidentProveFixture(t, false)
			tip, calls := b.repo.canonical, 0
			lane.owners.landing.recordMain = func(goal.VerbRequest, goal.TrunkRedRecordArgs) (goal.PublishResult, error) {
				calls++
				return goal.PublishResult{}, errors.New("unexpected incident")
			}
			lane.fail = func(cmd *exec.Cmd, only string) (string, error) {
				if name == "green" {
					return "LANDING-CHECKED\t0\n", nil
				}
				if name == "not run" {
					return "LANDING-NOT-RUN\tbusy\n", errors.New("busy")
				}
				if only == "" {
					return replayFailure, errors.New("red")
				}
				if name == "incomplete main" {
					return "no complete report\n", errors.New("red")
				}
				if _, err := os.Stat(filepath.Join(cmd.Dir, "b")); err == nil {
					return replayFailure, errors.New("red")
				}
				return "LANDING-CHECKED\t0\n", nil
			}
			code, out := lane.run(t, lane.root, "prove", "--wait", "--json")
			wantCode := 1
			if name == "green" {
				wantCode = 0
			}
			if code != wantCode || calls != 0 || b.repo.canonical != tip {
				t.Fatalf("%s recorded an incident: code=%d calls=%d tip=%s output=%s", name, code, calls, b.repo.canonical, out)
			}
		})
	}
}

func TestIncidentRecorderRecordsBuildFailureAndEveryUnitLog(t *testing.T) {
	t.Parallel()
	b := newIntentBed(t, false, nil)
	owners := incidentRecorderFixture(t, b, "finder")
	build := incidentCheckFixture(t)
	test := incidentCheckFixture(t, "TestOther")
	test.Failed[0].Unit = "internal/other"
	if err := owners.proveSeams(b.root()).RecordMain([]plain.Result{build, test}); err != nil {
		t.Fatal(err)
	}
	_, result := b.runJSON(b.owners(), "incident", "list")
	data, _ := json.Marshal(result.Data)
	var listed struct{ Incidents []goal.TrunkRedEntry }
	if err := json.Unmarshal(data, &listed); err != nil || len(listed.Incidents) != 2 {
		t.Fatalf("list: %s %v", data, err)
	}
	for _, entry := range listed.Incidents {
		if entry.Group == "internal/unit" && (entry.Identity != "red:internal/unit" || len(entry.Failures) != 0 || entry.Sightings[0].LogPath != build.Log) || entry.Group == "internal/other" && entry.Sightings[0].LogPath != test.Log {
			t.Fatalf("unit's own log or unnamed build failure lost: %+v", entry)
		}
	}
}

func TestIncidentLandingPushClearsOnlyNewestFreshFullGreen(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"fresh full", "scoped", "inherited", "without scope", "other full tree", "old full time", "unrelated tree", "not pushed", "clear failure", "changed entry", "old accepted", "without register", "bad register"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b := newIntentBed(t, false, nil)
			oldAccepted := b.repo.accepted
			owners := incidentRecorderFixture(t, b, "finder")
			if err := owners.proveSeams(b.root()).RecordMain([]plain.Result{incidentCheckFixture(t, "TestOne", "TestTwo")}); err != nil {
				t.Fatal(err)
			}
			pushed := b.repo.commit(b.repo.canonical)
			pushed.files = obligationFilesCopy(pushed.files)
			if name == "without register" {
				delete(pushed.files, "plans/goals/trunk-red.json")
			}
			if name == "bad register" {
				pushed.files["plans/goals/trunk-red.json"] = []byte("not JSON")
			}
			b.repo.commits["head"] = pushed
			if name == "old accepted" {
				b.repo.accepted = oldAccepted
			}
			lane := newResolveVerbFixture(t)
			stopData, err := json.Marshal(plain.Stop{Loop: "lane-proof", Decision: "stop", Handoff: "hold red:internal/unit:TestOne", Cause: &plain.Cause{Kind: "main", Name: "red:internal/unit:TestOne"}})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(plain.Dir(lane.install), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(plain.Dir(lane.install), "stops.jsonl"), append(stopData, '\n'), 0o600); err != nil {
				t.Fatal(err)
			}
			lane.owners.landing.mainEndpoint, lane.owners.landing.machine = owners.mainEndpoint, owners.machine
			lane.owners.landing.now = owners.now
			proof := plain.Result{Result: plain.Green, Tree: "pushed-tree", Commit: "head", Scope: "full", At: syncRequestTestNow.Format(time.RFC3339), Attempt: "full-green", FullTree: "pushed-tree", FullAt: syncRequestTestNow.Format(time.RFC3339)}
			newest := proof
			switch name {
			case "scoped":
				newest.Scope = "scoped"
			case "inherited":
				newest.Reason = "inherits green from tree prior"
			case "without scope":
				newest.Scope = ""
			case "other full tree":
				newest.FullTree = "older-tree"
			case "old full time":
				newest.FullAt = syncRequestTestNow.Add(-time.Minute).Format(time.RFC3339)
			case "unrelated tree":
				newest.Tree, newest.FullTree = "other-tree", "other-tree"
			}
			if name == "unrelated tree" {
				writeCauseProof(t, lane.install, "results.jsonl", newest)
			} else {
				writeCauseProof(t, lane.install, "results.jsonl", proof, newest)
			}
			lane.owners.landing.clearMain = func(r goal.VerbRequest, args goal.TrunkRedClearArgs) (goal.PublishResult, error) {
				if args.ExpectedEntry.ID != args.Entry || args.Attempt != proof.Attempt || args.BaseTree != proof.Tree || args.BaseCommit != proof.Commit || !args.Executed {
					t.Fatalf("clear is not bound to the observed entry and proof: %+v", args)
				}
				if name == "clear failure" {
					return goal.PublishResult{Outcome: goal.OutcomeRejected, Detail: "not cleared"}, nil
				}
				if name == "changed entry" {
					args.ExpectedEntry.Status = "not-run"
				}
				return goal.ClearTrunkRed(r, args)
			}
			lane.owners.landing.push = func(_, _ string, _ time.Time, before func(string, string) error) (plain.PushOutcome, error) {
				out := plain.PushOutcome{Old: "old", Commit: "head", Tree: "pushed-tree", Changed: name != "not pushed"}
				return out, before(out.Old, out.Commit)
			}
			command, _ := findIntentAction("landing", "push")
			command = laneCommand(command, func(inv *intentInvocation, admitted laneAdmitted) int {
				return runIntentLandingPushWithOwners(inv, admitted, landingPushOwners{notify: func(plain.PushOutcome) error { return nil }})
			})
			var stdout, stderr bytes.Buffer
			code := runIntentIn(command, []string{"--json"}, &stdout, &stderr, lane.root, lane.owners)
			wantCode := 0
			if name == "clear failure" || name == "changed entry" || name == "bad register" {
				wantCode = 1
				if !strings.Contains(stdout.String(), "incidents could not be cleared") {
					t.Fatalf("post-push failure hidden: %s", &stdout)
				}
			}
			if code != wantCode {
				t.Fatalf("push: %d %s %s", code, &stdout, &stderr)
			}
			b.repo.accepted = b.repo.canonical
			_, result := b.runJSON(b.owners(), "incident", "list")
			data, _ := json.Marshal(result.Data)
			var listed struct{ Incidents []goal.TrunkRedEntry }
			if err := json.Unmarshal(data, &listed); err != nil {
				t.Fatal(err)
			}
			want := 2
			if name == "fresh full" || name == "old accepted" {
				want = 0
			}
			if len(listed.Incidents) != want {
				t.Fatalf("%s cleared wrong entries: %s", name, data)
			}
			stop, err := plain.NewestStop(lane.install)
			if err != nil || (stop == nil) != (want == 0) {
				t.Fatalf("%s: stop closure disagrees with incident clearance: %+v %v", name, stop, err)
			}
		})
	}
}

func TestLandingMainStopNamesIncidentInStatusAndQuestion(t *testing.T) {
	t.Parallel()
	b, register, _ := incidentProveFixture(t, false)
	if err := os.WriteFile(filepath.Join(b.install, "go.mod"), []byte("module fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(b.install, "metasystem.conf"), []byte("metasystem.template=true\nproof.full=fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	b.owners.landing.view = func(string) lane.View {
		return lane.View{Root: &b.root, Owner: lane.OwnerView{State: lane.OwnerIdle}, Summary: "the landing lane is idle"}
	}
	mainRed := b.fail
	b.fail = func(cmd *exec.Cmd, only string) (string, error) {
		if only != "" {
			return "LANDING-CHECKED\t0\n", nil
		}
		return mainRed(cmd, only)
	}
	b.prove(t)
	b.fail = mainRed
	b.prove(t)
	stop, err := plain.NewestStop(b.install)
	identity := "red:u/a:TestOne"
	if err != nil || stop == nil || stop.Attempt != 2 || stop.Cause.Name != identity || stop.Handoff != "hold "+identity || stop.Command() != "metasystem incident list" {
		t.Fatalf("main stop has no incident handoff: %+v %v", stop, err)
	}
	code, text := b.run(t, b.root, "status")
	if code != 0 || !strings.Contains(oneSpaced(text), stop.Command()) || !strings.Contains(oneSpaced(text), identity) {
		t.Fatalf("status: %d %s", code, text)
	}
	data, err := json.Marshal(stop)
	if err != nil {
		t.Fatal(err)
	}
	q, err := channel.Ask(channel.AskRequest{RepoRoot: b.install, About: "lane", Kind: "other", Machine: "finder-one", Lineage: lane.AgentLineage,
		Facts: []string{stop.Command(), "lane stop: " + string(data), "evidence: " + stop.Evidence}, Now: syncRequestTestNow})
	if err != nil {
		t.Fatal(err)
	}
	b.owners.processes.question = channel.ReadQuestion
	action, _ := findIntentAction("question", "show")
	var stdout, stderr bytes.Buffer
	code = runIntentIn(action, []string{"channel:" + q.ID}, &stdout, &stderr, b.install, b.owners)
	if code != 0 || !strings.HasPrefix(stdout.String(), stop.Command()+"\n") || !strings.Contains(stdout.String(), identity) {
		t.Fatalf("question: %d %s %s", code, &stdout, &stderr)
	}
	proof := plain.Result{Result: plain.Green, Commit: register.repo.canonical, Tree: "green-tree", Scope: "full", At: syncRequestTestNow.Format(time.RFC3339), FullAt: syncRequestTestNow.Format(time.RFC3339), FullTree: "green-tree", Attempt: "clearing-green"}
	if err := b.owners.landing.clearLandingIncidents(b.install, proof); err != nil {
		t.Fatal(err)
	}
	if stop, err := plain.NewestStop(b.install); err != nil || stop != nil {
		t.Fatalf("cleared incident still holds: %+v %v", stop, err)
	}
}

func TestIncidentRecorderRejectsMissingOrInconsistentEvidence(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"missing log", "different main", "green", "rejected", "empty"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b := newIntentBed(t, false, nil)
			owners := incidentRecorderFixture(t, b, "finder")
			checks := []plain.Result{incidentCheckFixture(t, "TestOne")}
			switch name {
			case "missing log":
				checks[0].Log += ".missing"
			case "different main":
				other := checks[0]
				other.Tree = "other"
				checks = append(checks, other)
			case "green":
				checks[0].Result = plain.Green
			case "empty":
				checks = nil
			case "rejected":
				owners.recordMain = func(goal.VerbRequest, goal.TrunkRedRecordArgs) (goal.PublishResult, error) {
					return goal.PublishResult{Outcome: goal.OutcomeRejected}, nil
				}
			}
			tip := b.repo.canonical
			err := owners.proveSeams(b.root()).RecordMain(checks)
			if (err == nil) != (name == "empty") || b.repo.canonical != tip {
				t.Fatalf("bad evidence changed the register: %v tip=%s", err, b.repo.canonical)
			}
		})
	}
}
