package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

func claimAreaDesign(t *testing.T, bed *intentBed, id, areas string) {
	t.Helper()
	relative := "plans/designs/" + id + ".md"
	text := fmt.Sprintf("# Edit areas\n\n- Kind: design\n- Id: design-%s\n- Status: accepted\n- Goals: %s\n- Areas: %s\n\n| Unit | Lines | Areas |\n| --- | --- | --- |\n| U1 | 20 | [] |\n", id, id, areas)
	path := filepath.Join(bed.root(), filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	bed.repo.commit(bed.repo.accepted).files[relative] = []byte(text)
}

func claimAreasBed(t *testing.T) (*intentBed, intentOwners, *resolveVerbFixture) {
	t.Helper()
	bed, owners, lane := claimLaneBed(t)
	owners.delivery.claimMain = func(string) (string, error) { return "local-main", nil }
	first := bed.goalFile(bedGoal)
	first.Claimed.Machine = "other-seat"
	bed.addGoal(first)
	return bed, owners, lane
}

func TestGoalClaimAreasSharedArc(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"whole arc", "complete arc", "individual member", "foreign overlap"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			bed, owners, _ := claimAreasBed(t)
			first := bed.goalFile(bedGoal)
			foreign := *first.Claimed
			first.State, first.Claimed = goal.StateApproved, nil
			first.Arc = "shared-arc"
			bed.addGoal(first)
			second := bed.goalFile("second-goal")
			second.Arc = first.Arc
			bed.addGoal(second)
			for _, id := range []string{bedGoal, "second-goal"} {
				claimAreaDesign(t, bed, id, "metasystem/shared/**")
			}
			if mode == "foreign overlap" {
				third := bed.goalFile("third-goal")
				foreign.Machine, foreign.Lineage = "other-seat", "other-session"
				foreign.AreaSnapshot = goal.AreaSnapshot{Known: true, Areas: []string{"metasystem/shared/**"}, Source: "foreign-design@" + strings.Repeat("a", 64)}
				third.State, third.Claimed = goal.StateClaimed, &foreign
				bed.addGoal(third)
			} else if mode != "whole arc" {
				if code, result := bed.runJSON(owners, "goal", "claim", bedGoal); code != 0 {
					t.Fatalf("first member: exit=%d %+v", code, result)
				}
			}
			before := bed.publications()
			args := []string{"goal", "claim", "second-goal"}
			if mode != "individual member" {
				args = append(args, "--arc")
			}
			code, result := bed.runJSON(owners, args...)
			if mode == "foreign overlap" {
				if code == 0 || bed.publications() != before || !strings.Contains(result.Summary, "third-goal") || !strings.Contains(result.Summary, "literal prefixes") {
					t.Fatalf("foreign overlap admitted: exit=%d %+v", code, result)
				}
				for _, id := range []string{bedGoal, "second-goal"} {
					if bed.goalFile(id).Claimed != nil {
						t.Fatalf("refused arc partially claimed %s", id)
					}
				}
				return
			}
			if code != 0 || bed.publications() != before+1 {
				t.Fatalf("shared arc: exit=%d %+v", code, result)
			}
			for _, id := range []string{bedGoal, "second-goal"} {
				file := bed.goalFile(id)
				if file.State != goal.StateClaimed || file.Claimed == nil || file.Claimed.Machine != "mac-cli" || file.Claimed.Lineage != "m1" || !file.Claimed.Known || !slices.Equal(file.Claimed.Areas, []string{"metasystem/shared/**"}) {
					t.Fatalf("arc member %s not bound to claimant and shared areas: %+v", id, file)
				}
			}
		})
	}
}

func TestGoalClaimAreasAdmission(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, first, second string
		refuse              bool
	}{
		{"future overlap", "metasystem/future/*.go", "metasystem/future/*.md", true},
		{"future overlap arc", "metasystem/future/*.go", "metasystem/future/*.md", true},
		{"disjoint", "metasystem/a/**", "metasystem/b/**", false},
		{"legacy unknown", "", "metasystem/a/**", false},
		{"pinned unknown", "metasystem/future/**", "metasystem/future/new.go", false},
		{"candidate unknown", "metasystem/a/**", "", false},
		{"malformed candidate", "metasystem/a/**", "bad[", false},
		{"malformed blocker", "bad[", "metasystem/a/**", false},
		{"explicit empty", "metasystem/a/**", "[]", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			bed, owners, _ := claimAreasBed(t)
			if test.first != "" {
				claimAreaDesign(t, bed, bedGoal, test.first)
			}
			if test.second != "" {
				claimAreaDesign(t, bed, "second-goal", test.second)
			}
			if test.name == "pinned unknown" {
				first := bed.goalFile(bedGoal)
				first.Claimed.AreaSnapshot = goal.AreaSnapshot{Warnings: []string{"areas unknown for goal " + bedGoal}}
				bed.addGoal(first)
			}
			before := bed.publications()
			args := []string{"goal", "claim", "second-goal"}
			if test.name == "future overlap arc" {
				args = append(args, "--arc")
			}
			code, result := bed.runJSON(owners, args...)
			if test.refuse {
				if code == 0 || bed.publications() != before || !strings.Contains(result.Summary, bedGoal) || !strings.Contains(result.Summary, "literal prefixes") || (result.Next == nil || !strings.Contains(result.Next.Reason, "after goal "+bedGoal+" lands or is dropped")) {
					t.Fatalf("overlap admission: exit=%d result=%+v", code, result)
				}
				return
			}
			if code != 0 || bed.publications() != before+1 {
				t.Fatalf("claim: exit=%d result=%+v", code, result)
			}
			file := bed.goalFile("second-goal")
			unknown := strings.Contains(test.name, "unknown") || strings.Contains(test.name, "malformed")
			if unknown && (!strings.Contains(result.Summary, "areas unknown") || !strings.Contains(file.History[len(file.History)-1].Reason, "areas unknown")) {
				t.Fatalf("warning not published: %+v %+v", result, file.Claimed)
			}
			if test.second != "" && !strings.Contains(test.name, "malformed candidate") && (!file.Claimed.Known || !strings.Contains(file.Claimed.Source, "design-second-goal@")) {
				t.Fatalf("snapshot not pinned: %+v", file.Claimed)
			}
		})
	}
}

func TestGoalClaimAreasQueueLifecycle(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"waiting", "returned", "records", "released waiting", "stale main", "contained", "records only contained", "dropped"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			bed, owners, lane := claimAreasBed(t)
			claimAreaDesign(t, bed, bedGoal, "metasystem/a/**")
			claimAreaDesign(t, bed, "second-goal", "metasystem/a/new.go")
			snapshot := ownersSnapshot(t, bed, bedGoal)
			if _, _, err := plain.HandIn(lane.install, plain.Line{Goal: bedGoal, SHA: "code-sha", Records: scenario == "records only contained", AreaSnapshot: snapshot}); err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "returned":
				if _, _, err := plain.Return(lane.install, bedGoal, "needs correction", time.Date(2026, 10, 6, 10, 0, 0, 0, time.UTC)); err != nil {
					t.Fatal(err)
				}
			case "records":
				if _, _, err := plain.HandIn(lane.install, plain.Line{Goal: bedGoal, SHA: "record-sha", Records: true, AreaSnapshot: snapshot}); err != nil {
					t.Fatal(err)
				}
			case "released waiting":
				first := bed.goalFile(bedGoal)
				first.Claimed.Machine, first.Claimed.Lineage = "mac-cli", "m1"
				bed.addGoal(first)
				if code, result := bed.runJSON(owners, "goal", "release", bedGoal, "--reason", "the hand-in keeps its areas"); code != 0 {
					t.Fatalf("release: exit=%d %+v", code, result)
				}
			case "contained", "records only contained":
				owners.delivery.laneContains = func(sha, main string) (bool, error) {
					if main != "local-main" {
						t.Fatalf("containment used ledger tip %q", main)
					}
					return sha == "code-sha", nil
				}
			case "stale main":
				// The landing is visible at the fetched tip, but local main
				// has not advanced to contain it.
				owners.delivery.laneContains = func(sha, main string) (bool, error) {
					return sha == "code-sha" && main != "local-main", nil
				}
			case "dropped":
				first := bed.goalFile(bedGoal)
				first.State, first.Claimed = goal.StateApproved, nil
				bed.addGoal(first)
				if _, _, err := plain.Return(lane.install, bedGoal, "dropped", time.Date(2026, 10, 6, 10, 0, 0, 0, time.UTC)); err != nil {
					t.Fatal(err)
				}
			}
			before := bed.publications()
			code, result := bed.runJSON(owners, "goal", "claim", "second-goal")
			released := scenario == "contained" || scenario == "records only contained" || scenario == "dropped"
			if released {
				if code != 0 || bed.publications() != before+1 {
					t.Fatalf("area not released: exit=%d %+v", code, result)
				}
			} else if code == 0 || bed.publications() != before || !strings.Contains(result.Summary, bedGoal) {
				t.Fatalf("area exclusion lost: exit=%d %+v", code, result)
			}
		})
	}
}

func ownersSnapshot(t *testing.T, bed *intentBed, id string) goal.AreaSnapshot {
	t.Helper()
	endpoint, err := bed.dependencies().endpoint(bed.root())
	if err != nil {
		t.Fatal(err)
	}
	snapshot := goal.DesignAreasAt(endpoint, bed.repo.accepted, id)
	if !snapshot.Known {
		t.Fatalf("fixture areas: %+v", snapshot)
	}
	return snapshot
}

func TestGoalClaimAreasQueueUnknown(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"corrupt queue", "no lane"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			bed, owners, lane := claimAreasBed(t)
			first := bed.goalFile(bedGoal)
			first.State, first.Claimed = goal.StateApproved, nil
			bed.addGoal(first)
			claimAreaDesign(t, bed, "second-goal", "metasystem/a/**")
			path := filepath.Join(plain.Dir(lane.install), "queue.jsonl")
			if scenario == "corrupt queue" {
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte("not json\n"), 0600); err != nil {
					t.Fatal(err)
				}
			} else {
				owners.delivery.laneRoot = func(string, time.Time) (string, bool, error) { return "", false, nil }
			}
			before := bed.publications()
			code, result := bed.runJSON(owners, "goal", "claim", "second-goal")
			file := bed.goalFile("second-goal")
			if code != 0 || bed.publications() != before+1 {
				t.Fatalf("queue advice held publication: exit=%d %+v", code, result)
			}
			warning := strings.Join(file.Claimed.Warnings, ";")
			if scenario == "corrupt queue" {
				if !strings.Contains(warning, "areas unknown") || !strings.Contains(warning, path) {
					t.Fatalf("queue path missing: %q", warning)
				}
			} else if warning != "" {
				t.Fatalf("no lane warned: %q", warning)
			}
		})
	}
}

func TestGoalClaimAreasFrontier(t *testing.T) {
	t.Parallel()
	bed, owners, _ := claimAreasBed(t)
	claimAreaDesign(t, bed, bedGoal, "metasystem/a/**")
	claimAreaDesign(t, bed, "second-goal", "metasystem/a/new.go")
	claimAreaDesign(t, bed, "third-goal", "metasystem/b/**")
	code, result := bed.runJSON(owners, "goal", "claim")
	if code != 0 || bed.goalFile("third-goal").Claimed == nil || bed.goalFile("second-goal").Claimed != nil {
		t.Fatalf("frontier offered overlap: exit=%d %+v", code, result)
	}
}

func TestWorkLandCarriesClaimAreas(t *testing.T) {
	t.Parallel()
	bed, _, install := plainLaneBedWith(t, true, "critic-root", "critic-root")
	file := bed.goalFile(bedGoal)
	snapshot := goal.AreaSnapshot{Known: true, Areas: []string{"metasystem/future/**"}, Source: "accepted-design@" + strings.Repeat("a", 64)}
	file.Claimed.AreaSnapshot = snapshot
	bed.addGoal(file)
	code, result := bed.do("work", "land", bedGoal)
	entries, err := plain.Entries(install)
	if code != 0 || err != nil || len(entries) != 1 || !entries[0].Known || entries[0].Source != snapshot.Source || strings.Join(entries[0].Areas, ",") != "metasystem/future/**" {
		t.Fatalf("hand-in snapshot: exit=%d %+v entries=%+v err=%v", code, result, entries, err)
	}
}

type claimRetryRepository struct {
	goal.Repository
	before func()
	called bool
}

func (repo *claimRetryRepository) Publish(parent, commit string) (goal.CASOutcome, error) {
	if !repo.called {
		repo.called = true
		repo.before()
	}
	return repo.Repository.Publish(parent, commit)
}

func TestGoalClaimAreasPublicationRetry(t *testing.T) {
	t.Parallel()
	bed, owners, _ := claimAreasBed(t)
	first := bed.goalFile(bedGoal)
	first.State, first.Claimed = goal.StateApproved, nil
	first.Pinned = "other-seat"
	bed.addGoal(first)
	claimAreaDesign(t, bed, bedGoal, "metasystem/future/**")
	claimAreaDesign(t, bed, "second-goal", "metasystem/future/new.go")
	endpoint, err := bed.dependencies().endpoint(bed.root())
	if err != nil {
		t.Fatal(err)
	}
	repository := &claimRetryRepository{Repository: endpoint.Repository}
	repository.before = func() {
		peerRoot := t.TempDir()
		peerEndpoint := goal.Endpoint{Root: peerRoot, Remote: endpoint.Remote, Branch: endpoint.Branch, Repository: endpoint.Repository}
		if err := os.WriteFile(filepath.Join(peerRoot, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0600); err != nil {
			t.Fatal(err)
		}
		request := goal.VerbRequest{Endpoint: peerEndpoint, Actor: goal.Actor{Machine: "other-seat", Lineage: "other-session"}, Ulid: "01J5X00000000000000000F010", Now: time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC), ClaimEpoch: 1}
		result, err := goal.Claim(request, bedGoal)
		if err != nil || result.Outcome != goal.OutcomeConfirmed {
			t.Fatalf("competing publication: %+v %v", result, err)
		}
	}
	owners.dependencies.endpoint = func(string) (goal.Endpoint, error) { copy := endpoint; copy.Repository = repository; return copy, nil }
	before := bed.publications()
	code, result := bed.runJSON(owners, "goal", "claim")
	if code == 0 || !repository.called || bed.publications() != before+1 || bed.goalFile("second-goal").Claimed != nil || !strings.Contains(result.Summary, bedGoal) {
		t.Fatalf("stale frontier bypassed admission: exit=%d %+v", code, result)
	}
}

func releasedAreasHandInBed(t *testing.T, overlap bool) (*deliveryBed, string, string) {
	t.Helper()
	bed, _, install := plainLaneBedWith(t, true, "critic-root", "critic-root")
	bed.lineage = "m1"
	announceProofFixtureHolder(t, bed.root())
	bed.owners.laneLatest = func(install, id, _ string) (plain.Entry, bool, error) { return plain.Latest(install, id) }
	bed.owners.laneContains = func(string, string) (bool, error) { return false, nil }
	claimAreaDesign(t, bed.intentBed, bedGoal, "metasystem/future/**")
	existingDesign := filepath.Join(bed.root(), "plans", "designs", "landing-work.md")
	data, err := os.ReadFile(existingDesign)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(existingDesign, append(data, []byte("\nAreas: []\n")...), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := plain.HandIn(install, plain.Line{Goal: bedGoal, Branch: "goal/" + bedGoal, SHA: strings.Repeat("2", 40)}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := plain.Return(install, bedGoal, "needs another hand-in", bed.owners.now()); err != nil {
		t.Fatal(err)
	}
	first := bed.goalFile(bedGoal)
	blocker := *first
	blocker.History = append([]goal.HistoryLine{}, first.History...)
	blocker.Id = "blocking-goal"
	claim := *first.Claimed
	claim.Machine = "other-seat"
	area := "metasystem/disjoint/**"
	if overlap {
		area = "metasystem/future/new.go"
	}
	claim.AreaSnapshot = goal.AreaSnapshot{Known: true, Areas: []string{area}, Source: "blocker-design@" + strings.Repeat("a", 64)}
	blocker.Claimed = &claim
	for index := range blocker.History {
		blocker.History[index].Targets = []string{blocker.Id}
	}
	bed.addGoal(&blocker)
	if code, result := bed.do("goal", "release", bedGoal, "--reason", "the branch can be handed in without a claim"); code != 0 {
		t.Fatalf("release: exit=%d %+v", code, result)
	}
	bed.owners.claimMain = func(string) (string, error) { return "local-main", nil }
	return bed, install, blocker.Id
}

func TestWorkLandReleasedAreasAdmission(t *testing.T) {
	t.Parallel()
	for _, overlap := range []bool{false, true} {
		t.Run(fmt.Sprintf("overlap=%v", overlap), func(t *testing.T) {
			t.Parallel()
			bed, install, blocker := releasedAreasHandInBed(t, overlap)
			bed.lineage = ""
			_, reader := enrollGoalSyncTerminal(t, bed.root(), "ttys:hand_in")
			owners := bed.intentBed.owners()
			owners.delivery, owners.connection, owners.work = bed.owners, bed.connection, bed.work
			owners.prove = func(root string, _ int64, _ humanauthority.Reader, _, _ string, now time.Time) (humanauthority.Proof, error) {
				if root != bed.root() {
					t.Fatalf("terminal proof root = %q, want %q", root, bed.root())
				}
				return humanauthority.Prove(bed.root(), reader.exact.Pid, reader, now)
			}
			before := bed.publications()
			code, result := bed.runJSON(owners, "work", "land", bedGoal, "--again")
			entries, err := plain.Entries(install)
			if err != nil || bed.publications() != before || bed.goalFile(bedGoal).Claimed != nil {
				t.Fatalf("hand-in changed the ledger: exit=%d %+v entries=%+v err=%v", code, result, entries, err)
			}
			if code != 0 || result.Outcome != intentConfirmed || len(entries) != 2 || !entries[1].Known || !strings.Contains(entries[1].Source, "design-"+bedGoal+"@") || strings.Join(entries[1].Areas, ",") != "metasystem/future/**" {
				t.Fatalf("person's released hand-in: exit=%d %+v entries=%+v", code, result, entries)
			}
			warnings := strings.Join(entries[1].Warnings, "; ")
			if overlap {
				for _, text := range []string{blocker, "metasystem/future/**", "metasystem/future/new.go", "overlap"} {
					if !strings.Contains(result.Summary, text) || !strings.Contains(warnings, text) {
						t.Fatalf("overlap warning lacks %q: result=%+v entry=%+v", text, result, entries[1])
					}
				}
				if !strings.Contains(result.Summary, "warning:") || strings.Contains(result.Summary, "goal claim") || strings.Contains(warnings, "goal claim") {
					t.Fatalf("person's warning suggests claiming: result=%+v entry=%+v", result, entries[1])
				}
			} else if warnings != "" || strings.Contains(result.Summary, "overlap") {
				t.Fatalf("disjoint hand-in warned: result=%+v entry=%+v", result, entries[1])
			}
		})
	}
}

func TestWorkLandReleasedAreasAgentRetry(t *testing.T) {
	t.Parallel()
	bed, install, blocker := releasedAreasHandInBed(t, true)
	before := bed.publications()
	code, result := bed.do("work", "land", bedGoal, "--again")
	entries, err := plain.Entries(install)
	if code == 0 || result.Outcome != intentRefused || err != nil || len(entries) != 1 || entries[0].State != plain.StateReturned || bed.publications() != before {
		t.Fatalf("agent's overlapping hand-in: exit=%d %+v entries=%+v err=%v", code, result, entries, err)
	}
	for _, text := range []string{blocker, "metasystem/future/**", "metasystem/future/new.go", "literal prefixes"} {
		if !strings.Contains(result.Summary, text) {
			t.Fatalf("refusal lacks %q: %+v", text, result)
		}
	}
	want := []string{"metasystem", "work", "land", bedGoal, "--again", "--json"}
	if result.Next == nil || !slices.Equal(result.Next.Argv, want) || !strings.Contains(result.Next.Reason, "lands or is dropped") || strings.Contains(result.Summary, "goal claim") {
		t.Fatalf("refusal does not retry the hand-in: %+v next=%+v", result, result.Next)
	}
	owners := bed.intentBed.owners()
	owners.delivery, owners.connection, owners.work = bed.owners, bed.connection, bed.work
	code, stdout, stderr := bed.run(owners, "work", "land", bedGoal, "--again")
	if code == 0 || !strings.Contains(stdout+stderr, shellCommand(want[:len(want)-1])) || strings.Contains(stdout+stderr, "goal claim") {
		t.Fatalf("printed refusal does not retry the hand-in: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	blockerSHA := strings.Repeat("3", 40)
	if _, _, err := plain.HandIn(install, plain.Line{Goal: blocker, SHA: blockerSHA, AreaSnapshot: bed.goalFile(blocker).Claimed.AreaSnapshot}); err != nil {
		t.Fatal(err)
	}
	bed.owners.laneContains = func(sha, main string) (bool, error) { return sha == blockerSHA && main == "local-main", nil }
	code, stdout, stderr = bed.run(owners, result.Next.Argv[1:]...)
	var retried intentResult
	if err := json.Unmarshal([]byte(stdout), &retried); err != nil {
		t.Fatalf("printed retry gave no JSON: exit=%d stdout=%q stderr=%q err=%v", code, stdout, stderr, err)
	}
	entry, ok, err := plain.Latest(install, bedGoal)
	if code != 0 || retried.Outcome != intentConfirmed || !ok || err != nil || entry.State != plain.StateWaiting || !entry.Known || len(entry.Warnings) != 0 || bed.publications() != before || bed.goalFile(bedGoal).Claimed != nil {
		t.Fatalf("printed retry after blocker landed: exit=%d %+v entry=%+v err=%v", code, retried, entry, err)
	}
}

func TestWorkLandReleasedAreasNeedsTerminalProof(t *testing.T) {
	t.Parallel()
	bed, install, _ := releasedAreasHandInBed(t, true)
	bed.lineage = ""
	_, reader := enrollGoalSyncTerminal(t, bed.root(), "ttys:enrolled")
	reader.terminalID = "ttys:another"
	owners := bed.intentBed.owners()
	owners.delivery, owners.connection, owners.work = bed.owners, bed.connection, bed.work
	owners.prove = func(root string, _ int64, _ humanauthority.Reader, _, _ string, now time.Time) (humanauthority.Proof, error) {
		if root != bed.root() {
			t.Fatalf("terminal proof root = %q, want %q", root, bed.root())
		}
		return humanauthority.Prove(bed.root(), reader.exact.Pid, reader, now)
	}
	code, result := bed.runJSON(owners, "work", "land", bedGoal, "--again")
	entries, err := plain.Entries(install)
	if code == 0 || result.Outcome != intentRefused || err != nil || len(entries) != 1 || entries[0].State != plain.StateReturned {
		t.Fatalf("empty lineage bypassed terminal proof: exit=%d %+v entries=%+v err=%v", code, result, entries, err)
	}
}

func TestWorkLandDoneRecordsSkipAreasAdmission(t *testing.T) {
	t.Parallel()
	bed := newRecordsBed(t)
	first := bed.goalFile(bedGoal)
	blocker := *first
	blocker.History = append([]goal.HistoryLine{}, first.History...)
	blocker.Id = "blocking-goal"
	claim := *first.Claimed
	claim.Machine = "other-seat"
	claim.AreaSnapshot = goal.AreaSnapshot{Known: true, Areas: []string{"metasystem/future/new.go"}, Source: "blocker-design@" + strings.Repeat("a", 64)}
	blocker.Claimed = &claim
	for index := range blocker.History {
		blocker.History[index].Targets = []string{blocker.Id}
	}
	bed.addGoal(&blocker)
	announceProofFixtureHolder(t, bed.root())
	if code, result := bed.runJSON(bed.owners, "goal", "done", bedGoal, "--lineage", "m1", "--reason", "the code has landed"); code != 0 {
		t.Fatalf("done: exit=%d %+v", code, result)
	}
	claimAreaDesign(t, bed.intentBed, bedGoal, "metasystem/future/**")
	before := bed.publications()
	code, result := bed.land(recordsPath)
	entries, err := plain.Entries(bed.lane)
	if code != 0 || result.Outcome != intentConfirmed || bed.publications() != before || err != nil || len(entries) != 1 || !entries[0].Records || !entries[0].Known || !strings.Contains(entries[0].Source, "design-"+bedGoal+"@") || strings.Join(entries[0].Areas, ",") != "metasystem/future/**" {
		t.Fatalf("done records hand-in: exit=%d %+v entries=%+v err=%v", code, result, entries, err)
	}
}
