package plain

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/state"
)

func assertUnrunMain(t *testing.T, install, checkout string, proof Result) Status {
	t.Helper()
	if _, err := os.Stat(filepath.Join(install, "metasystem.conf")); os.IsNotExist(err) {
		if err := os.WriteFile(filepath.Join(install, "metasystem.conf"), []byte("proof.trunk-every=1h\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	home := t.TempDir()
	record := lane.Record{Root: checkout, Install: install, CustodyEpoch: 1}
	shadowRegister(t, home, record)
	seams := shadowSeams()
	seams.ReadOnly = true
	seams.Git = func(_ string, args ...string) (string, error) {
		if len(args) > 0 && args[0] == "rev-parse" {
			return proof.Commit, nil
		}
		return "", nil
	}
	seams.Alive = func(Running) bool { return false }
	seams.Incidents = func(string, string, string) ([]goal.TrunkRedEntry, error) {
		return []goal.TrunkRedEntry{{ID: "unrelated-flake", Class: goal.TrunkRedClassFlake}}, nil
	}
	if _, err := RefreshShadow(home, bedNow, "fixture", seams); err != nil {
		t.Fatal(err)
	}
	view := lane.View{Root: &checkout, Owner: lane.OwnerView{State: lane.OwnerIdle}}
	status := readStatus(home, record, view, seams, laneGit{main: func() (string, error) { return proof.Commit, nil }, contains: func(string, string) (bool, error) { return false, nil }})
	if status.Shadow.Main.Word != "unproven" || !status.Shadow.Main.CheckNotRun || len(status.TrunkIncidents) != 0 || len(status.Shadow.Main.OpenIncidents) != 0 || status.Shadow.Main.Flakes != "" {
		t.Fatalf("unrun status: %+v", status)
	}
	encoded, err := json.Marshal(status)
	if err != nil {
		t.Fatal(err)
	}
	description := state.Describe(*status.Shadow, state.RenderInputs{})
	if !strings.Contains(description.Headline, "main's last check could not run") || !strings.Contains(description.Headline, proof.Reason) || strings.Contains(string(encoded), "unrelated-flake") || strings.Contains(description.Person, "land a fix on main") {
		t.Fatalf("unrun rendering: %s; %s; %s", description.Headline, description.Person, encoded)
	}
	return status
}

func TestNonRunReportsNeverAuthorizeProof(t *testing.T) {
	t.Parallel()
	for _, scope := range []string{"trunk", "full", "scoped"} {
		for _, exit := range []int{0, 1} {
			t.Run(fmt.Sprintf("%s/exit-%d", scope, exit), func(t *testing.T) {
				t.Parallel()
				b := newScopeBed(t)
				if scope != "scoped" {
					b.git.changed[[2]string{b.base.Tree, b.git.tree}] = "metasystem/code.go"
				}
				b.seams.Git = b.git.run
				b.seams.Trunk = scope == "trunk"
				b.seams.Command = func(cmd *exec.Cmd) error {
					cmd.Args = []string{"/bin/sh", "-c", fmt.Sprintf("printf 'LANDING-NOT-RUN\\tfixture disk unavailable\\n'; exit %d", exit)}
					return cmd.Run()
				}
				result := b.run(t)
				if result.Result != Red || !result.didNotRun() || result.CountedFull || result.Repeat != "allowed" || !strings.Contains(result.Reason, "fixture disk unavailable") || result.FullAt == result.At || result.FullTree == result.Tree || result.Scope != map[string]string{"trunk": "full", "full": "full", "scoped": "scoped"}[scope] {
					t.Fatalf("non-run counted or reusable: %+v", result)
				}
				pushSeams := b.seams
				notAncestor := exec.Command("/usr/bin/false").Run()
				pushSeams.Git = func(dir string, args ...string) (string, error) {
					if strings.Join(args, " ") == "rev-parse --verify refs/remotes/origin/main^{commit}" {
						return "older-main", nil
					}
					switch args[0] {
					case "fetch", "cat-file":
						return "", nil
					case "merge-base":
						return "", notAncestor
					}
					return b.git.run(dir, args...)
				}
				_, pushErr := PushChecked(b.install, b.checkout, bedNow, nil, pushSeams)
				var refusal *Refusal
				if !errors.As(pushErr, &refusal) || refusal.Code != CodeRed {
					t.Fatalf("non-run did not refuse push for its saved verdict: %v", pushErr)
				}
				if due, err := fullProofDue(b.install, bedNow); err != nil || !due {
					t.Fatalf("non-run clock: %v %v", due, err)
				}
				status := ShadowOf(Status{}, RawFacts{Main: result.Commit, Results: []Result{b.base, result}})
				if status.Main.Word != "unproven" {
					t.Fatalf("main acquired an unrun proof: %+v", status.Main)
				}
				if scope == "trunk" {
					assertUnrunMain(t, b.install, b.checkout, result)
					if due, err := fullProofDue(b.install, bedNow); err != nil || !due {
						t.Fatalf("unrun clock: %v %v", due, err)
					}
				}
			})
		}
	}
}

func TestLatestMainNonRunReplacesEarlierVerdicts(t *testing.T) {
	t.Parallel()
	for _, earlier := range []string{Green, Red} {
		for _, retry := range []string{"next wake", "recorded time", "spent"} {
			t.Run(earlier+"/"+retry, func(t *testing.T) {
				t.Parallel()
				b := newStatusBed(t)
				first := Result{Attempt: "earlier", Trunk: true, Commit: "main", Tree: "tree", Result: earlier, CountedFull: true, At: bedNow.Add(-2 * time.Hour).Format(time.RFC3339), Cause: &Cause{Kind: "main"}}
				latest := Result{Attempt: "latest", Trunk: true, Commit: "main", Tree: "tree", Result: Red, Reason: "fixture storage unavailable", Repeat: "allowed", At: bedNow.Format(time.RFC3339), Cause: &Cause{Kind: "environment", Name: "lost-process"}}
				if retry == "spent" {
					latest.Repeat = ""
				}
				other := Result{Attempt: "other-batch", Commit: "other", Tree: "other-tree", Result: Green, At: bedNow.Format(time.RFC3339), Reason: "flake on main: another check", Flakes: []FlakeRecord{{FailedUnit: FailedUnit{Unit: "another/unit", Tests: []string{"TestOther"}}}}}
				b.lines("results.jsonl", first, latest, other)
				if retry == "recorded time" {
					b.lines("clock.json", clockWake{At: bedNow, RetryAt: bedNow.Add(time.Hour)})
				}
				status := assertUnrunMain(t, b.install, b.checkout, latest)
				description := state.Describe(*status.Shadow, state.RenderInputs{})
				want := "at its next wake"
				if retry == "recorded time" {
					want = bedNow.Add(time.Hour).Local().Format("2006-01-02 15:04 MST")
				}
				if retry == "spent" {
					want = "metasystem landing prove --trunk"
				}
				if !strings.Contains(description.Headline, want) {
					t.Fatalf("missing retry advice %q: %+v", want, description)
				}
				if retry != "recorded time" {
					if _, err := os.Stat(clockPath(b.install)); !os.IsNotExist(err) {
						t.Fatalf("invented clock: %v", err)
					}
				}
			})
		}
	}
}

func TestFullClockCreditsOnlyCompletedMainChecks(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"green", "main red", "unrun"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			install := t.TempDir()
			if err := os.WriteFile(filepath.Join(install, "metasystem.conf"), []byte("proof.trunk-every=1h\n"), 0600); err != nil {
				t.Fatal(err)
			}
			proof := Result{Attempt: "proof", Trunk: true, Scope: "full", Commit: "main", Result: Green, At: bedNow.Format(time.RFC3339)}
			if kind == "main red" {
				proof.Result, proof.CountedFull, proof.Cause = Red, true, &Cause{Kind: "main"}
			}
			if kind == "unrun" {
				proof.Result, proof.Cause = Red, &Cause{Kind: "environment", Name: "lost-process"}
			}
			if err := os.MkdirAll(Dir(install), 0700); err != nil {
				t.Fatal(err)
			}
			if err := appendLine(resultsPath(install), proof); err != nil {
				t.Fatal(err)
			}
			due, err := fullProofDue(install, bedNow)
			if err != nil || due != (kind == "unrun") {
				t.Fatalf("clock %s: %v %v", kind, due, err)
			}
			if kind == "main red" {
				shadow := ShadowOf(Status{}, RawFacts{Main: "main", Results: []Result{proof}})
				if shadow.Main.Word != Red {
					t.Fatalf("genuine main red lost: %+v", shadow.Main)
				}
			}
		})
	}
}

func TestKeeperRetriesNonRunWithinExistingAllowance(t *testing.T) {
	t.Parallel()
	b := newKeeperClockBed(t)
	b.batch = nil
	b.wake(t)
	running, _, _, err := ReadRunning(b.install, b.seams)
	if err != nil || b.trunks != 1 {
		t.Fatalf("start: %v", err)
	}
	b.alive = false
	b.seams.Trunk = true
	b.seams.Command = func(cmd *exec.Cmd) error {
		fmt.Fprint(cmd.Stdout, "LANDING-NOT-RUN\tfixture storage unavailable\n")
		return nil
	}
	first, err := Run(b.install, b.checkout, "fixture", running.Attempt, io.Discard, b.seams)
	if err != nil || first.Repeat != "allowed" || !first.didNotRun() {
		t.Fatalf("first non-run: %+v %v", first, err)
	}
	status := assertUnrunMain(t, b.install, b.checkout, first)
	if !strings.Contains(state.Describe(*status.Shadow, state.RenderInputs{}).Headline, "at its next wake") {
		t.Fatal("non-run invented a retry delay")
	}
	b.wake(t)
	if b.trunks != 2 {
		t.Fatalf("next wake did not retry: %d", b.trunks)
	}
	running, _, _, err = ReadRunning(b.install, b.seams)
	if err != nil {
		t.Fatal(err)
	}
	b.alive = false
	second, err := Run(b.install, b.checkout, "fixture", running.Attempt, io.Discard, b.seams)
	if err != nil || second.Repeat == "allowed" || !second.didNotRun() {
		t.Fatalf("second non-run: %+v %v", second, err)
	}
	b.wake(t)
	if b.trunks != 2 {
		t.Fatalf("spent allowance started a third proof: %d", b.trunks)
	}
	clock, err := readClock(b.install)
	if err != nil || !clock.RetryAt.Equal(bedNow.Add(4*time.Hour)) {
		t.Fatalf("refusal backoff: %+v %v", clock, err)
	}
	status = assertUnrunMain(t, b.install, b.checkout, second)
	if !strings.Contains(state.Describe(*status.Shadow, state.RenderInputs{}).Headline, "metasystem landing prove --trunk") {
		t.Fatal("spent allowance lost person action")
	}
	b.wake(t)
	if b.trunks != 2 {
		t.Fatal("launch happened before RetryAt")
	}
}

func TestKeeperNonRunLaunchRefusalKeepsRecordedBackoff(t *testing.T) {
	t.Parallel()
	for _, previous := range []bool{false, true} {
		t.Run(fmt.Sprint("previous-nonrun=", previous), func(t *testing.T) {
			t.Parallel()
			b := newKeeperClockBed(t)
			b.batch = nil
			proof := Result{Attempt: "unrun", Trunk: true, Commit: "main", Tree: "main-tree", Result: Red, Cause: &Cause{Kind: "environment", Name: "lost-process"}, Repeat: "allowed", Reason: "fixture storage unavailable", At: bedNow.Format(time.RFC3339)}
			if previous {
				b.save(t, proof)
			}
			launches := 0
			b.seams.Launch = func([]string, string, string) (int64, error) {
				launches++
				return 0, fmt.Errorf("fixture launch refused")
			}
			b.wake(t)
			clock, err := readClock(b.install)
			if err != nil || !clock.RetryAt.Equal(bedNow.Add(4*time.Hour)) || launches != 1 {
				t.Fatalf("launch refusal clock: %+v %v launches=%d", clock, err, launches)
			}
			expected := proof
			expected.Reason = "fixture launch refused"
			status := assertUnrunMain(t, b.install, b.checkout, expected)
			description := state.Describe(*status.Shadow, state.RenderInputs{})
			if !strings.Contains(description.Headline, clock.RetryAt.Local().Format("2006-01-02 15:04 MST")) {
				t.Fatalf("recorded retry missing: %+v", description)
			}
			b.wake(t)
			if launches != 1 {
				t.Fatal("refusal retried before its interval")
			}
			b.seams.Now = func() time.Time { return bedNow.Add(4 * time.Hour) }
			b.keeper.Now = b.seams.Now
			b.wake(t)
			if launches != 2 {
				t.Fatalf("refusal never retried after its interval: %d", launches)
			}
		})
	}
}

func TestTrunkIncidentAttributionUsesOnlyTheSelectedResult(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name   string
		failed []FailedUnit
		nonRun bool
		want   []string
	}{
		{name: "same test", failed: []FailedUnit{{Unit: "fixture/unit", Tests: []string{"TestA"}}}, want: []string{"replay-a"}},
		{name: "different test", failed: []FailedUnit{{Unit: "fixture/unit", Tests: []string{"TestOther"}}}},
		{name: "non-run", failed: []FailedUnit{{Unit: "fixture/unit", Tests: []string{"TestA"}}}, nonRun: true},
		{name: "all failures", failed: []FailedUnit{{Unit: "fixture/unit", Tests: []string{"TestA", "TestB"}}, {Unit: "fixture/other", Tests: []string{"TestC"}}}, want: []string{"replay-a", "replay-b", "replay-c", "replay-unit"}},
		{name: "no failures"},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			b := newStatusBed(t)
			proof := Result{Attempt: "main-red", Trunk: true, Commit: "main", Tree: "tree", Result: Red, CountedFull: true, Cause: &Cause{Kind: "main"}, Failed: row.failed, At: bedNow.Format(time.RFC3339)}
			if row.nonRun {
				proof.CountedFull, proof.Cause, proof.Reason = false, &Cause{Kind: "environment"}, "fixture unavailable"
			}
			// Main's incidents were opened by an earlier batch replay, not its trunk proof.
			incidents := []goal.TrunkRedEntry{
				{ID: "replay-a", Identity: "red:fixture/unit:TestA"},
				{ID: "replay-b", Identity: "red:fixture/unit:TestB"},
				{ID: "replay-c", Identity: "red:fixture/other:TestC"},
				{ID: "replay-unit", Identity: "red:fixture/other"},
				{ID: "unrelated-flake", Identity: "flake:fixture/unit:TestA", Class: goal.TrunkRedClassFlake},
				{ID: "closed-incident", Identity: "red:fixture/unit:TestA", Closed: &goal.TrunkRedClosure{}},
				{ID: "same-attempt-other-test", Identity: "red:fixture/unit:TestUnrelated"},
			}
			for index := range incidents {
				incidents[index].Sightings = []goal.TrunkRedSighting{{Attempt: "batch-replay-1"}}
			}
			incidents[len(incidents)-1].Sightings[0].Attempt = proof.Attempt
			b.lines("results.jsonl", proof, Result{Attempt: "batch-green", Commit: "batch", Result: Green})
			b.lines("queue.jsonl", Line{Goal: "waiting-goal", SHA: "waiting-sha", Seat: "fixture", At: proof.At})
			seams := shadowSeams()
			seams.Incidents = func(string, string, string) ([]goal.TrunkRedEntry, error) { return incidents, nil }
			if _, err := RefreshShadow(b.home, bedNow, "fixture", seams); err != nil {
				t.Fatal(err)
			}
			status := readStatus(b.home, b.record, b.view, seams, b.git(nil, nil))
			encoded, err := json.Marshal(status)
			if err != nil {
				t.Fatal(err)
			}
			var decoded Status
			if err := json.Unmarshal(encoded, &decoded); err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(decoded.TrunkIncidents, row.want) || !slices.Equal(decoded.Shadow.Main.OpenIncidents, row.want) {
				t.Fatalf("incident attribution: want %v; trunk %v; shadow %v", row.want, decoded.TrunkIncidents, decoded.Shadow.Main.OpenIncidents)
			}
			description := state.Describe(*status.Shadow, state.RenderInputs{})
			wantWord := Red
			if row.nonRun {
				wantWord = "unproven"
			}
			if status.Shadow.Main.Word != wantWord {
				t.Fatalf("main verdict: %+v", status.Shadow.Main)
			}
			if len(row.want) > 0 {
				if !strings.Contains(description.Headline, "(incident "+strings.Join(row.want, ", ")+")") || !strings.Contains(description.Headline, "metasystem landing land-anyway waiting-goal --reason TEXT") {
					t.Fatalf("red main lost its incidents or escape: %s", description.Headline)
				}
			} else if strings.Contains(description.Headline, "(incident ") || strings.Contains(description.Headline, "land-anyway") {
				t.Fatalf("unrelated incident or escape: %s", description.Headline)
			}
			for _, incident := range incidents {
				if !slices.Contains(row.want, incident.ID) && strings.Contains(description.Headline, incident.ID) {
					t.Fatalf("unrelated incident %s: %s; %s", incident.ID, encoded, description.Headline)
				}
			}
		})
	}
}
