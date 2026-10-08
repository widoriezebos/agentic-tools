package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/repoproof"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testrun"
)

func TestFlakesAppearThroughTestStatusAndMainIncidents(t *testing.T) {
	t.Parallel()
	b := newGoalCLIBed(t, goalCLISeed{})
	b.setNow(laneTestNow)
	endpoint, err := b.endpoint(b.root)
	if err != nil {
		t.Fatal(err)
	}
	names := []string{"TestOne/sub/leaf", "TestTwo"}
	original, repeat := filepath.Join(t.TempDir(), "original.log"), filepath.Join(t.TempDir(), "repeat.log")
	writeTestingFixtureFile(t, original, []byte(flakeTestEvents(names, "fail")), 0600)
	writeTestingFixtureFile(t, repeat, []byte(flakeTestEvents(names, "pass")), 0600)
	outputs, err := repoproof.TestEvidence(original, "u/a", names, "fail")
	if err != nil {
		t.Fatal(err)
	}
	repeatOutputs, err := repoproof.TestEvidence(repeat, "u/a", names, "pass")
	if err != nil {
		t.Fatal(err)
	}
	request := goal.VerbRequest{Endpoint: endpoint, Actor: goal.Actor{Machine: b.machine, Lineage: b.lineage},
		Ulid: "01J5X0000000000000000000F1", Now: b.clock()}
	tree := strings.Repeat("a", 40)
	observation := goal.FlakeRecordArgs{Unit: "u/a", Tests: names, Commit: "candidate", Tree: tree, Attempt: "original-red",
		LogPath: original, Repeat: "alone", Where: "tip", Outputs: outputs, RepeatOutputs: repeatOutputs,
		Rerun: goal.TrunkRedRerun{Attempt: "repeat-green", LogPath: repeat}}
	if _, err := goal.RecordFlake(request, observation); err != nil {
		t.Fatal(err)
	}
	read := func() []goal.TrunkRedEntry {
		t.Helper()
		entries, problems := goal.ParseTrunkRed([]byte(b.accepted("plans/goals/trunk-red.json")))
		if len(problems) > 0 {
			t.Fatal(problems)
		}
		return entries
	}
	entries := read()
	if len(entries) != 2 || entries[0].Closed != nil || entries[1].Closed != nil {
		t.Fatalf("a passing repeat closed a test: %+v", entries)
	}
	fix := entries[0].FixGoal
	proof := proofrun.TestResult{CandidateTree: tree, RequiredGroups: []string{"check"}, SelectedGroups: []string{"check"}}
	proof.RecomputeDelivery()
	if proof.Delivery.Sufficient || !slices.Equal(proof.Delivery.MissingGroups, []string{"check"}) {
		t.Fatalf("fixture did not start without proof: %+v", proof)
	}
	inputs := testStatusInputs{endpoint: b.endpoint, now: b.commandNow,
		verify: func(r testrun.SelectionRequest) (proofrun.TestResult, error) {
			if r.Tree != tree || r.Root != b.root {
				t.Fatalf("verification subject changed: %+v", r)
			}
			return proof, nil
		}}
	status := func(jsonOutput bool) (int, string, string) {
		t.Helper()
		var stdout, stderr bytes.Buffer
		command, _, ok := resolveIntentArgv([]string{"test", "status"})
		if !ok {
			t.Fatal("test status has no public route")
		}
		args := []string{"--root", b.root, "--tree", tree}
		if jsonOutput {
			args = append(args, "--json")
		}
		code := runPassthrough(command, func(args []string, stdout, stderr io.Writer) int {
			return runTestStatusWithInputs(args, stdout, stderr, inputs)
		}, args, &stdout, &stderr)
		return code, stdout.String(), stderr.String()
	}
	var decoded struct {
		proofrun.TestResult
		Flakes           []goal.FlakeFact `json:"flakes"`
		FlakeInformation string           `json:"flakeInformation"`
	}
	before := b.tip()
	for _, green := range []bool{false, true} {
		if green {
			proof.Groups = []proofrun.GroupResult{{ID: "check", Status: "passed"}}
			proof.RecomputeDelivery()
		}
		want := 1
		if green {
			want = 0
		}
		code, text, refusal := status(false)
		if code != want || !green && !strings.Contains(refusal, "no passing test run covers") || green && !strings.Contains(text, "proven for delivery") {
			t.Fatalf("flake changed proof verdict: exit=%d stdout=%s stderr=%s", code, text, refusal)
		}
		for _, words := range []string{names[0], names[1], "original fail", "repeat pass", "original-red", "repeat-green", original, repeat, fix, outputs[names[0]].Path, repeatOutputs[names[0]].Digest} {
			if !strings.Contains(strings.Join(strings.Fields(text), " "), words) {
				t.Fatalf("test status omitted %q: %s", words, text)
			}
		}
		code, text, refusal = status(true)
		if code != want || json.Unmarshal([]byte(text), &decoded) != nil || !reflect.DeepEqual(decoded.TestResult, proof) || len(decoded.Flakes) != 2 || decoded.FlakeInformation != "available" {
			t.Fatalf("JSON lost observations or changed proof: exit=%d %s %s", code, text, refusal)
		}
		for _, fact := range decoded.Flakes {
			if len(fact.Sightings) != 1 || fact.FixGoal != fix || fact.Closed != nil || fact.Sightings[0].Output == nil || fact.Sightings[0].Rerun.Output == nil || !reflect.DeepEqual(*fact.Sightings[0].Output, outputs[fact.TestName]) || !reflect.DeepEqual(*fact.Sightings[0].Rerun.Output, repeatOutputs[fact.TestName]) {
				t.Fatalf("JSON lost exact output evidence: %+v", fact)
			}
		}
	}
	if b.tip() != before {
		t.Fatal("test status published a change")
	}
	code, text, refusal := b.public("incident", "list", "--json")
	if code != 0 || strings.Contains(text, names[0]) || strings.Contains(text, names[1]) {
		t.Fatalf("candidate-only tests became main incidents: %d %s %s", code, text, refusal)
	}
	// The original main failure and an explicitly allowanced legacy flake stay distinct.
	at := b.clock().Format(time.RFC3339)
	mainRed := goal.TrunkRedEntry{ID: "main-red", Identity: "main-red", Group: "u/a", Status: "failed", Opened: at,
		Failures:  []goal.TrunkRedFailure{{Name: "TestMainRed", Status: "failed", Report: original, Classname: "u/a"}},
		Sightings: []goal.TrunkRedSighting{{Attempt: "main-red", BaseCommit: "main", SeenAt: at, Opid: goal.Opid("01J5X0000000000000000000F2", b.machine, b.lineage)}}, Holds: []string{"held-batch"}}
	allowance := mainRed
	allowance.ID, allowance.Identity, allowance.Group, allowance.Class = "legacy-allowed", "legacy-allowed", "u/legacy", goal.TrunkRedClassKnownFlake
	allowance.Holds = nil
	allowance.Failures = []goal.TrunkRedFailure{{Name: "TestLegacy", Status: "failed", Report: original, Classname: "u/legacy"}}
	allowance.AllowanceUntil = b.clock().Add(time.Hour).Format(time.RFC3339)
	allowance.Owner = goal.TrunkRedOwner{Machine: "person-seat", Since: at, How: "approver", By: "Wido"}
	publishRegister := func(data []byte, opid string) {
		t.Helper()
		parent, err := b.repo.Capture(opid)
		if err != nil {
			t.Fatal(err)
		}
		tip, err := b.repo.Build(opid, parent, []goal.Change{{Path: "plans/goals/trunk-red.json", Content: data}}, "incident fixture")
		if err != nil {
			t.Fatal(err)
		}
		if outcome, err := b.repo.Publish(parent, tip); err != nil || outcome != goal.CASLanded {
			t.Fatalf("publish: %s %v", outcome, err)
		}
		if err := b.repo.AcceptedCAS(parent, tip); err != nil {
			t.Fatal(err)
		}
	}
	publishRegister(goal.RenderTrunkRed(append(entries, mainRed, allowance)), goal.Opid("01J5X0000000000000000000F4", b.machine, b.lineage))
	endpoint, _ = b.endpoint(b.root)
	request.Endpoint, request.Ulid, request.Now = endpoint, "01J5X0000000000000000000F3", b.clock().Add(time.Minute)
	observation.Where, observation.Commit, observation.Attempt, observation.Rerun.Attempt = "main", "main", "main-original", "main-repeat"
	if _, err := goal.RecordFlake(request, observation); err != nil {
		t.Fatal(err)
	}
	code, text, refusal = b.public("incident", "list")
	for _, words := range []string{names[0], names[1], "main-original", "main-repeat", "allowance until", mainRed.ID, allowance.ID} {
		if code != 0 || !strings.Contains(strings.Join(strings.Fields(text), " "), words) {
			t.Fatalf("main incident view omitted %q: %d %s %s", words, code, text, refusal)
		}
	}
	var listed struct {
		Data struct{ Incidents, TrackedDefects []goal.TrunkRedEntry }
	}
	code, text, refusal = b.public("incident", "list", "--json")
	if code != 0 || json.Unmarshal([]byte(text), &listed) != nil || len(listed.Data.Incidents) != 3 || len(listed.Data.TrackedDefects) != 1 {
		t.Fatalf("main incident identity or legacy allowance disappeared: %d %s %s", code, text, refusal)
	}
	target := entries[0].ID
	if code, text, refusal = b.public("incident", "claim", target, "--goal", fix); code != 0 {
		t.Fatalf("claim existing flake: %d %s %s", code, text, refusal)
	}
	prior := read()
	if code, text, refusal = b.public("incident", "close", target, "--reason", "Fixed the intermittent test."); code != 0 {
		t.Fatalf("person close existing flake: %d %s %s", code, text, refusal)
	}
	for _, entry := range read() {
		if entry.ID == target {
			if entry.Closed == nil || entry.FixGoal != fix {
				t.Fatalf("claim or close changed the wrong target: %+v", entry)
			}
			continue
		}
		index := slices.IndexFunc(prior, func(old goal.TrunkRedEntry) bool { return old.ID == entry.ID })
		if index < 0 || !reflect.DeepEqual(entry, prior[index]) {
			t.Fatalf("claim/close changed sibling, main blocking or legacy authority: %+v", entry)
		}
	}
	// A current closure stays current even when this tree also has an older sighting.
	code, text, refusal = status(true)
	if code != 0 || json.Unmarshal([]byte(text), &decoded) != nil || len(decoded.Flakes) != 2 || !slices.ContainsFunc(decoded.Flakes, func(f goal.FlakeFact) bool { return f.ID == target && f.Closed != nil }) {
		t.Fatalf("status hid the current closure: %d %s %s", code, text, refusal)
	}
	// A different tree's observations cannot leak into this status.
	otherTree := tree
	tree = strings.Repeat("b", 40)
	proof.CandidateTree = tree
	code, text, refusal = status(true)
	if code != 0 || json.Unmarshal([]byte(text), &decoded) != nil || len(decoded.Flakes) != 0 {
		t.Fatalf("status included another tree's flakes: %d %s %s", code, text, refusal)
	}
	tree, proof.CandidateTree = otherTree, otherTree
	// Corrupt accepted metadata is unavailable, independently of green or missing proof.
	publishRegister([]byte("{"), goal.Opid("01J5X0000000000000000000F5", b.machine, b.lineage))
	for _, green := range []bool{true, false} {
		want := 0
		if !green {
			proof.Groups = nil
			proof.RecomputeDelivery()
			want = 1
		}
		for _, jsonOutput := range []bool{false, true} {
			code, text, refusal = status(jsonOutput)
			if code != want || !strings.Contains(text, "flake information unavailable") {
				t.Fatalf("unreadable flake information changed proof: %d %s %s", code, text, refusal)
			}
			if jsonOutput && (json.Unmarshal([]byte(text), &decoded) != nil || !reflect.DeepEqual(decoded.TestResult, proof) || len(decoded.Flakes) != 0) {
				t.Fatalf("unreadable metadata became a green flake: %s", text)
			}
		}
	}
	// The real verifier also refuses an installation without its testing contract.
	inputs.verify = nil
	code, text, refusal = status(true)
	// An unreadable proof writes no JSON result (an empty one would read as "no proof yet"); the refusal is the answer.
	if code != 1 || strings.TrimSpace(text) != "" || refusal == "" {
		t.Fatalf("metadata hid the real proof-read failure: %d %s %s", code, text, refusal)
	}
	if _, err := os.Stat(filepath.Join(b.root, "artifacts", "agents", "proof")); !os.IsNotExist(err) {
		t.Fatalf("status created a proof launch: %v", err)
	}
}

func TestFlakeAttemptsAppearInLandingStatus(t *testing.T) {
	t.Parallel()
	b := newResolveVerbFixture(t)
	proof := plain.Result{Commit: "candidate", Tree: "candidate-tree", Result: plain.Red,
		Reason: "the repeat passed but its observation could not be published", Attempt: "original-red", Log: "original.log",
		At: laneTestNow.Format(time.RFC3339), Repeat: "started", RepeatComplete: true,
		FlakeRepeats: []plain.Running{{Attempt: "repeat-green", Tree: "candidate-tree", Commit: "candidate", Log: "repeat.log"}}}
	if err := os.MkdirAll(plain.Dir(b.install), 0700); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(proof)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(plain.Dir(b.install), "results.jsonl")
	writeTestingFixtureFile(t, path, append(data, '\n'), 0600)
	for _, jsonOutput := range []bool{false, true} {
		words := []string{"status"}
		if jsonOutput {
			words = append(words, "--json")
		}
		code, text := b.run(t, b.root, words...)
		if code != 0 {
			t.Fatalf("landing status: %d %s", code, text)
		}
		if jsonOutput {
			var decoded struct {
				Data struct {
					Last plain.Result `json:"last_proof"`
				}
			}
			if json.Unmarshal([]byte(text), &decoded) != nil || !reflect.DeepEqual(decoded.Data.Last, proof) {
				t.Fatalf("status changed the held result: %s", text)
			}
		} else {
			normalized := strings.Join(strings.Fields(text), " ")
			for _, words := range []string{proof.Reason, "red", "original-red", "repeat-green", "original.log", "repeat.log"} {
				if !strings.Contains(normalized, words) {
					t.Fatalf("landing status omitted %q: %s", words, text)
				}
			}
		}
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(after, append(data, '\n')) {
		t.Fatalf("status rewrote the proof: %v", err)
	}
}
