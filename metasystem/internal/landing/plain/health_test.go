package plain

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/conflict"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginebuild"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostload"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/repoproof"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

func healthSeams(now time.Time) ProveSeams {
	return ProveSeams{Now: func() time.Time { return now }, Machine: func(string) (string, error) { return "landing", nil }, Incidents: func(string, string, string) ([]goal.TrunkRedEntry, error) { return nil, nil }, Git: func(_ string, args ...string) (string, error) {
		if args[0] == "symbolic-ref" {
			return "main", nil
		}
		return "main", nil
	}, Load: func(time.Time) hostload.Sample { return hostload.Sample{Available: true, Cores: 8, Load1m: 2} }}
}
func healthRow(h *Health, name string) *Fired {
	if h == nil {
		return nil
	}
	for _, r := range h.Rows() {
		if r.Row == name {
			return &r
		}
	}
	return nil
}
func readHealthBed(t *testing.T, b *statusBed, s ProveSeams) Status {
	t.Helper()
	status := readStatus(b.home, b.record, b.view, s, b.git(nil, nil))
	if status.Health == nil {
		t.Fatalf("health unreadable: %v", status.Problems)
	}
	return status
}
func TestLandingHealthQuestionsThroughStatus(t *testing.T) {
	t.Parallel()
	old, next := strings.Repeat("a", 40), strings.Repeat("b", 40)
	for _, tc := range []struct {
		name, fact, main, premise string
		stale                     bool
	}{{"moved", "Main is at commit " + old, next, "gone", true}, {"same", "Main is at commit " + old, old, "current", false}, {"prose", "May I repair this?", next, "unknown", false}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b := newStatusBed(t)
			s := healthSeams(bedNow)
			s.Git = func(_ string, args ...string) (string, error) {
				if args[0] == "rev-parse" {
					return tc.main, nil
				}
				return "", nil
			}
			_, err := channel.Ask(channel.AskRequest{RepoRoot: b.install, About: "lane", Machine: "landing", Lineage: lane.AgentLineage, Kind: "other", Facts: []string{tc.fact}, Wants: "a repair decision", Now: bedNow.Add(-31 * time.Minute)})
			if err != nil {
				t.Fatal(err)
			}
			status := readHealthBed(t, b, s)
			h := status.Health
			if len(h.Questions) != 1 || h.Questions[0].Premise != tc.premise || (healthRow(h, "stale-question") != nil) != tc.stale || healthRow(h, "question-open") == nil {
				t.Fatalf("questions=%+v rows=%+v", h.Questions, h.Rows())
			}
		})
	}
}
func TestLandingHealthAgentWaitAndSilence(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, record string
		waiting      bool
		silent       bool
	}{{"waiting", "", true, true}, {"empty", "", false, true}, {"proof", "proof", true, false}, {"fix", "fix", true, false}, {"review", "review", false, false}, {"regeneration", "regeneration", false, false}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b := newStatusBed(t)
			s := healthSeams(bedNow)
			since := bedNow.Add(-55 * time.Minute).Format(time.RFC3339)
			b.view.Owner = lane.OwnerView{State: lane.OwnerRunning, Since: &since}
			b.lines("agent-idle.json", agentIdle{EndedAt: since, Step: &AgentStep{Verb: "work land", StartedAt: since}})
			if tc.waiting {
				b.lines("queue.jsonl", Line{Goal: "g", SHA: "g-sha", At: since})
			}
			switch tc.record {
			case "proof":
				b.lines("running.json", Running{Attempt: "p", Since: since, Pid: 42})
				s.Alive = func(Running) bool { return true }
			case "fix", "review":
				state := "running"
				if tc.record == "review" {
					state = "reviewing"
				}
				if err := WriteFix(b.install, &Fix{Attempt: "fix-1", State: state, ReadStartedAt: since, Goal: "g", Units: []string{"unit"}, Job: "job", Commit: "main"}); err != nil {
					t.Fatal(err)
				}
			case "regeneration":
				log := filepath.Join(b.checkout, "regeneration.log")
				if err := os.WriteFile(log, []byte("working"), 0600); err != nil {
					t.Fatal(err)
				}
				b.lines("regenerating.json", RunningRegeneration{Goal: "g", State: "running", Since: since, Pid: 42, Log: log})
				s.Alive = func(Running) bool { return true }
			}
			status := readHealthBed(t, b, s)
			h := status.Health
			if h.Agent.Silent != tc.silent || (healthRow(h, "silent-agent") != nil) != tc.silent {
				t.Fatalf("agent=%+v rows=%+v", h.Agent, h.Rows())
			}
			if tc.record != "" && !strings.Contains(h.Agent.WaitingOn, tc.record) {
				t.Fatalf("waiting on %q", h.Agent.WaitingOn)
			}
			if tc.silent {
				want := "with nothing to do"
				if tc.waiting {
					want = "with 1 goal waiting"
				}
				if !strings.Contains(healthRow(h, "silent-agent").Subject, want) {
					t.Fatal(h.Rows())
				}
				if !tc.waiting && (status.AgentProgress == nil || status.AgentProgress.Finding == "") {
					t.Fatal("health discarded the existing empty-batch idle finding")
				}
			}
		})
	}
}

func TestLandingHealthAgentAgeUsesActivitySince(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		running bool
		idle    int
		minutes float64
		silent  bool
	}{
		{"finished a minute ago", false, 1, 1, false},
		{"step still running", true, 0, 50, false},
		{"idle without an explanation", false, 25, 25, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b := newStatusBed(t)
			start := bedNow.Add(-50 * time.Minute).Format(time.RFC3339)
			b.view.Owner = lane.OwnerView{State: lane.OwnerRunning, Since: &start}
			b.lines("queue.jsonl", Line{Goal: "g", SHA: "tip", At: start})
			step := AgentStep{Verb: "work review", StartedAt: start, PID: 42}
			if tc.running {
				b.lines("agent-step.json", step)
			} else {
				b.lines("agent-idle.json", agentIdle{EndedAt: bedNow.Add(-time.Duration(tc.idle) * time.Minute).Format(time.RFC3339), Step: &step})
			}
			s := healthSeams(bedNow)
			s.Alive = func(Running) bool { return true }
			h := readHealthBed(t, b, s).Health
			if h.Agent.Minutes == nil || *h.Agent.Minutes != tc.minutes || h.Agent.Verb != step.Verb || h.Agent.Silent != tc.silent || (healthRow(h, "silent-agent") != nil) != tc.silent {
				t.Fatalf("agent=%+v rows=%+v", h.Agent, h.Rows())
			}
			if tc.running && !strings.Contains(h.Agent.WaitingOn, step.Verb+" since ") || !tc.running && h.Agent.WaitingOn != "" {
				t.Fatalf("waiting on %q", h.Agent.WaitingOn)
			}
		})
	}
}
func TestLandingHealthIncidentsFollowRecordedProof(t *testing.T) {
	t.Parallel()
	for _, proved := range []bool{false, true} {
		t.Run(fmt.Sprint(proved), func(t *testing.T) {
			t.Parallel()
			b := newStatusBed(t)
			s := healthSeams(bedNow)
			s.Incidents = func(string, string, string) ([]goal.TrunkRedEntry, error) {
				return []goal.TrunkRedEntry{{ID: "red", TestUnit: "unit", TestName: "TestRed", Class: goal.TrunkRedClassTrunkRed, Sightings: []goal.TrunkRedSighting{{Attempt: "p", BaseCommit: "old"}}}}, nil
			}
			records := []any{Result{Attempt: "p", Commit: "old", Trunk: true, Result: Red, Cause: &Cause{Kind: "main"}, Failed: []FailedUnit{{Unit: "unit", Tests: []string{"TestRed"}}}}}
			if proved {
				records = append(records, Result{Attempt: "new", Commit: "main", Trunk: true, Result: Green})
			}
			b.lines("results.jsonl", records...)
			h := readHealthBed(t, b, s).Health
			if len(h.Incidents) != 1 || !h.Incidents[0].MainMoved || h.Incidents[0].Unproven == proved || (healthRow(h, "unproven-incidents") != nil) == proved {
				t.Fatalf("incidents=%+v rows=%+v", h.Incidents, h.Rows())
			}
		})
	}
}
func TestLandingHealthProofRowsAndOldSamples(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		repeat bool
		load   bool
		row    string
	}{{"contention", false, true, "suspected-contention"}, {"flake", true, true, "confirmed-flake"}, {"unmeasured", false, false, ""}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b := newStatusBed(t)
			r := Result{Result: Red, Load: 9, Failed: []FailedUnit{{Unit: "unit", Tests: []string{"TestRed"}}}}
			if tc.load {
				r.LoadStart = &hostload.Sample{Available: true, Cores: 8, Load1m: 9}
			}
			if tc.repeat {
				r.FlakeRepeats = []Running{{Attempt: "repeat"}}
				r.RepeatComplete = true
				r.Flakes = []FlakeRecord{{FailedUnit: r.Failed[0], Repeat: "alone", RepeatOutputs: map[string]repoproof.TestOutput{"TestRed": {Outcome: "pass"}}}}
			}
			b.lines("results.jsonl", r)
			h := readHealthBed(t, b, healthSeams(bedNow)).Health
			if tc.row != "" && healthRow(h, tc.row) == nil {
				t.Fatal(h.Rows())
			}
			if tc.repeat && healthRow(h, "suspected-contention") != nil {
				t.Fatal(h.Rows())
			}
			if !tc.load && (h.Load.AtStart != nil || h.Load.AtEnd != nil || !strings.Contains(h.Text(), "unmeasured")) {
				t.Fatalf("old samples: %+v", h.Load)
			}
		})
	}
}
func TestLandingHealthDurationThresholdsAndLine(t *testing.T) {
	t.Parallel()
	for _, minutes := range []float64{50, 70} {
		t.Run(fmt.Sprint(minutes), func(t *testing.T) {
			t.Parallel()
			b := newStatusBed(t)
			duration := 40.
			var records []any
			for i := 0; i < 5; i++ {
				records = append(records, Result{Attempt: fmt.Sprint(i), Result: Green, Scope: "full", Minutes: &duration, LoadStart: &hostload.Sample{Available: true, Load1m: 33}, LoadEnd: &hostload.Sample{Available: true, Load1m: 12}})
			}
			b.lines("results.jsonl", records...)
			b.lines("running.json", Running{Attempt: "running", Since: bedNow.Add(-time.Duration(minutes) * time.Minute).Format(time.RFC3339), Pid: 42})
			s := healthSeams(bedNow)
			s.Alive = func(Running) bool { return true }
			h := readHealthBed(t, b, s).Health
			if h.Proofs.Median == nil || *h.Proofs.Median != 40 || (healthRow(h, "proof-drift") != nil) != (minutes > 60) {
				t.Fatalf("proofs=%+v rows=%v", h.Proofs, h.Rows())
			}
			if row := healthRow(h, "proof-drift"); row != nil && !strings.Contains(row.Act, "load at start 33/end 12") {
				t.Fatalf("recorded proof samples omitted: %+v", row)
			}
		})
	}
	b := newStatusBed(t)
	b.lines("queue.jsonl", Line{Goal: "g", SHA: "g-sha", At: bedNow.Add(-95 * time.Minute).Format(time.RFC3339)})
	if _, err := lane.SetPauseBecause(b.home, "person", "maintenance", bedNow.Add(-31*time.Minute)); err != nil {
		t.Fatal(err)
	}
	h := readHealthBed(t, b, healthSeams(bedNow)).Health
	r := healthRow(h, "overdue-goal")
	if r == nil || r.Minutes != "95" || !strings.Contains(h.Line(), "2 rows fire: overdue-goal, paused") {
		t.Fatalf("line=%s rows=%+v", h.Line(), h.Rows())
	}
	h = readHealthBed(t, newStatusBed(t), healthSeams(bedNow)).Health
	if h.Line() != "lane idle; nothing fires" {
		t.Fatal(h.Line())
	}
}
func TestLandingHealthEngineStampAndLaneDiff(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	old := b.originMain()
	if err := os.MkdirAll(filepath.Join(b.install, "bin"), 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(b.install, "bin", "metasystem")
	build := testenv.Go("build", "-buildvcs=false", "-ldflags", enginebuild.StampLinkerFlags(old), "-o", path, "../../enginebuild/testdata/stampprobe")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("stamped fixture binary: %v %s", err, out)
	}

	record := lane.Record{Root: b.checkout, Install: b.install}
	view := lane.View{Root: &b.checkout}
	s := ProveSeams{Now: func() time.Time { return bedNow }}
	for _, laneChange := range []bool{false, true} {
		dir := filepath.Join(b.install, "unrelated")
		if laneChange {
			dir = filepath.Join(b.install, "internal", "landing")
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "health-marker"), []byte("change"), 0600); err != nil {
			t.Fatal(err)
		}
		b.git(b.checkout, "add", filepath.Join(dir, "health-marker"))
		b.git(b.checkout, "commit", "--quiet", "-m", "advance main")
		b.git(b.checkout, "push", "--quiet", "origin", "main")
		status := ReadStatus(t.TempDir(), record, view, s)
		if status.Health == nil || status.Health.Engine.Revision != old || status.Health.Engine.Stale != laneChange {
			t.Fatalf("health=%+v problems=%v", status.Health, status.Problems)
		}
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	status := ReadStatus(t.TempDir(), record, view, s)
	if status.Health == nil || status.Health.Engine.Known || status.Health.Engine.Stale || !strings.Contains(status.Health.Text(), "unknown") {
		t.Fatalf("missing stamp: %+v", status.Health)
	}
}
func TestLandingHealthAbandonedResolutionReadOnly(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	s := healthSeams(bedNow)
	if err := WriteFix(b.install, &Fix{Attempt: "merge", Commit: "main", Tip: "old", State: "resolving", Goal: "g", Units: []string{"unit"}, Paths: []conflict.Path{{Path: "x"}}}); err != nil {
		t.Fatal(err)
	}
	s.Git = func(_ string, args ...string) (string, error) {
		if slices.Contains(args, "MERGE_HEAD^{commit}") {
			cmd := exec.Command("git", "rev-parse", "--verify", "MERGE_HEAD^{commit}")
			cmd.Dir = b.checkout
			_, err := cmd.Output()
			return "", err
		}
		return "main", nil
	}
	path := filepath.Join(Dir(b.install), "fixes", "merge.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s.ReadOnly = true
	status := readHealthBed(t, b, s)
	if status.RunningFix != nil || healthRow(status.Health, "abandoned-resolution") == nil {
		t.Fatalf("status=%+v rows=%+v", status, status.Health.Rows())
	}
	after, err := os.ReadFile(path)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("health changed fix: %v", err)
	}
	s.ReadOnly = false
	status = readHealthBed(t, b, s)
	if status.RunningFix != nil || healthRow(status.Health, "abandoned-resolution") == nil {
		t.Fatalf("status lost abandonment: %+v", status)
	}
	stored, err := readFix(b.install, true)
	if err != nil || stored == nil || stored.State != "abandoned" {
		t.Fatalf("status did not persist abandonment: %+v %v", stored, err)
	}
}

func TestLandingHealthBatchBoundaryAndRecordedEligibility(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, policy                 string
		after, person, unknown, held bool
		want, reason                 string
	}{{"before", "auto", false, false, false, false, "admissible", ""}, {"another depth", "auto", false, false, false, false, "admissible", ""}, {"unknown eligibility", "auto", false, false, false, false, "unknown", "time"}, {"after", "auto", true, false, false, false, "admissible", ""}, {"cap one", "1", false, false, false, false, "not admissible", "cap"}, {"person", "auto", false, true, false, false, "not admissible", "person"}, {"person seal action", "auto", false, false, false, false, "not admissible", "person"}, {"unknown seal", "auto", false, false, true, false, "unknown", "seal"}, {"held", "auto", false, false, false, true, "not admissible", "held"}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b := newStatusBed(t)
			b.record.CustodyEpoch = 1
			seal := bedNow.Add(-5 * time.Minute)
			arrival := seal.Add(-2 * time.Minute)
			if tc.after {
				arrival = seal.Add(2 * time.Minute)
			}
			handedAt := arrival.Format(time.RFC3339)
			if tc.name == "unknown eligibility" {
				handedAt = "unrecorded"
			}
			b.lines("queue.jsonl", Line{Goal: "g", SHA: "g-tip", At: handedAt, Held: tc.held})
			if tc.held {
				b.lines("queue.jsonl", Line{Goal: "g", SHA: "g-tip", At: arrival.Format(time.RFC3339)}, Line{Goal: "g", SHA: "g-tip", At: arrival.Format(time.RFC3339), Outcome: StateWaiting, Held: true, Reason: "held"})
			}
			batch := &Batch{ID: "batch", Base: "main", Lane: b.record, State: BatchPrepared, CreatedAt: seal.Add(-10 * time.Minute).Format(time.RFC3339), SealedAt: seal.Format(time.RFC3339), Members: []GoalSHA{{Goal: "member", SHA: "member-tip"}}, Selector: PolicyValue{Value: tc.policy}}
			if tc.name == "another depth" {
				batch.DepthClass = "cheap"
				_, class, err := selectDepthClass(b.install, b.checkout, []Entry{{Goal: "g"}}, ProveSeams{GoalTier: func(string, string) (uint8, error) { return 3, nil }})
				if err != nil || class != "full" {
					t.Fatalf("fixture depth: %s %v", class, err)
				}
			}
			if tc.person {
				batch.Person = &ActProvenance{Person: "person", Kind: "selection", Root: b.checkout, CheckedAt: bedNow, TerminalGeneration: 1, TerminalRef: humanauthority.ProcessRef{PID: 1}, Destination: b.record, Subject: batch.Members}
			}
			if tc.name == "person seal action" {
				batch.Actions = []BatchAction{{Kind: "seal", By: "person", At: batch.SealedAt}}
			}
			if tc.unknown {
				batch.SealedAt = ""
			}
			if err := writeBatch(b.install, batch); err != nil {
				t.Fatal(err)
			}
			h := readHealthBed(t, b, healthSeams(bedNow)).Health
			if h.Batch == nil || len(h.Batch.Waiting) != 1 {
				t.Fatalf("batch %+v", h.Batch)
			}
			fact := h.Batch.Waiting[0]
			if fact.Admission != tc.want || !strings.Contains(fact.Reason, tc.reason) || (healthRow(h, "batch-of-one") != nil) != (tc.want == "admissible" && !tc.after) {
				t.Fatalf("fact=%+v rows=%+v", fact, h.Rows())
			}
			if tc.after && fact.Boundary != "arrived after the boundary" {
				t.Fatal(fact)
			}
		})
	}
}

func TestLandingHealthComparableProofsAndDuplicateAttempts(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	full, impact := 40., 4.
	var records []any
	records = append(records, Result{Attempt: "full", Result: Green, Scope: "full", Minutes: &full})
	for i := 0; i < 5; i++ {
		r := Result{Attempt: fmt.Sprint(i), Result: Green, Scope: "impact", Minutes: &impact}
		records = append(records, r, r)
	}
	b.lines("results.jsonl", records...)
	b.lines("running.json", Running{Attempt: "current", Since: bedNow.Add(-50 * time.Minute).Format(time.RFC3339), Pid: 42})
	s := healthSeams(bedNow)
	s.Alive = func(Running) bool { return true }
	h := readHealthBed(t, b, s).Health
	if h.Proofs.Median == nil || *h.Proofs.Median != 40 || len(h.Proofs.Durations) != 1 || h.Proofs.Drift {
		t.Fatalf("different scopes mixed: %+v", h.Proofs)
	}
	durations := comparableProofs([]Result{{Attempt: "one", Result: Green, Scope: "full", Minutes: &full}, {Attempt: "one", Result: Green, Scope: "full", Minutes: &full}}, "full")
	if len(durations) != 1 {
		t.Fatalf("one attempt counted twice: %v", durations)
	}
}

func TestLandingHealthProofScopeAndAttemptSelection(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, scope string
		impact      bool
		admitted    bool
	}{
		{"full", "full", false, false},
		{"impact", "impact", true, false},
		{"admitted impact", "impact", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b := newStatusBed(t)
			var records []any
			for i, minutes := range []float64{100, 38, 67, 45, 41, 44} {
				r := Result{Attempt: fmt.Sprint(i), Result: Green, Scope: tc.scope, Minutes: &minutes}
				records = append(records, r)
				if i == 2 {
					for range 4 {
						records = append(records, r)
					}
				}
			}
			other := "impact"
			if tc.scope == other {
				other = "full"
			}
			short := 4.
			for i := range 5 {
				records = append(records, Result{Attempt: fmt.Sprintf("other-%d", i), Result: Green, Scope: other, Minutes: &short})
			}
			b.lines("results.jsonl", records...)
			running := Running{Attempt: "current", Impact: tc.impact, Since: bedNow.Add(-50 * time.Minute).Format(time.RFC3339), Pid: 42}
			if tc.admitted {
				running.Admission = &ExecutionAdmission{Scope: tc.scope}
			}
			b.lines("running.json", running)
			s := healthSeams(bedNow)
			s.Alive = func(Running) bool { return true }
			h := readHealthBed(t, b, s).Health
			if h.Proofs.Median == nil || *h.Proofs.Median != 44 || len(h.Proofs.Durations) != 5 || h.Proofs.Drift {
				t.Fatalf("scope or duplicate attempts biased median: %+v", h.Proofs)
			}
		})
	}
	minutes := 40.
	legacy := Result{Result: Green, Scope: "full", Minutes: &minutes}
	if got := comparableProofs([]Result{legacy, legacy}, "full"); len(got) != 1 {
		t.Fatalf("unidentified results changed the full-proof estimator's selection: %v", got)
	}
}

func TestLandingHealthRepeatUsesRecordedTestOutcomes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ outcome, want string }{{"pass", Green}, {"fail", Red}, {"", "none"}} {
		t.Run(tc.want, func(t *testing.T) {
			t.Parallel()
			r := Result{Flakes: []FlakeRecord{{FailedUnit: FailedUnit{Unit: "unit", Tests: []string{"TestRed"}}, Repeat: "alone", RepeatOutputs: map[string]repoproof.TestOutput{"TestRed": {Outcome: tc.outcome}}}}}
			if got := healthRepeat(r); got != tc.want {
				t.Fatalf("recorded %s gave %s", tc.outcome, got)
			}
		})
	}
}

func TestLandingHealthAgeBoundaries(t *testing.T) {
	t.Parallel()
	for _, age := range []int{20, 21, 30, 31, 60, 61} {
		t.Run(fmt.Sprint(age), func(t *testing.T) {
			t.Parallel()
			b := newStatusBed(t)
			since := bedNow.Add(-time.Duration(age) * time.Minute)
			stamp := since.Format(time.RFC3339)
			b.view.Owner = lane.OwnerView{State: lane.OwnerRunning, Since: &stamp}
			b.lines("queue.jsonl", Line{Goal: "g", SHA: "tip", At: stamp})
			if _, err := lane.SetPauseBecause(b.home, "person", "maintenance", since); err != nil {
				t.Fatal(err)
			}
			if _, err := channel.Ask(channel.AskRequest{RepoRoot: b.install, About: "lane", Machine: "landing", Lineage: lane.AgentLineage, Kind: "other", Facts: []string{"May I repair this?"}, Now: since}); err != nil {
				t.Fatal(err)
			}
			h := readHealthBed(t, b, healthSeams(bedNow)).Health
			for _, r := range []struct {
				name string
				want bool
			}{{"silent-agent", age > 20}, {"paused", age > 30}, {"question-open", age > 30}, {"overdue-goal", age > 60}} {
				if (healthRow(h, r.name) != nil) != r.want {
					t.Fatalf("at %d min: %s fired=%t, want %t", age, r.name, healthRow(h, r.name) != nil, r.want)
				}
			}
		})
	}
}

func TestLandingHealthProvingAndLandedAge(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	stamp := bedNow.Add(-95 * time.Minute).Format(time.RFC3339)
	b.lines("queue.jsonl", Line{Goal: "g", SHA: "tip", At: stamp})
	b.lines("running.json", Running{Attempt: "running", Commit: "candidate", Since: stamp, Pid: 42})
	s := healthSeams(bedNow)
	s.Alive = func(Running) bool { return true }
	git := b.git(nil, nil)
	git.contains = func(commit, sha string) (bool, error) { return commit == "candidate" && sha == "tip", nil }
	status := readStatus(b.home, b.record, b.view, s, git)
	if status.Health == nil || len(status.Health.Goals) != 1 || status.Health.Goals[0].State != "proving" || *status.Health.Goals[0].Minutes != 95 {
		t.Fatalf("proving health: %+v, problems %v", status.Health, status.Problems)
	}
	if err := os.Remove(runningPath(b.install)); err != nil {
		t.Fatal(err)
	}
	b.lines("pushes.jsonl", Pushed{Old: "old", Commit: "main", At: bedNow.Add(-10 * time.Minute).Format(time.RFC3339)})
	status = readStatus(b.home, b.record, b.view, s, b.git([]string{"tip"}, map[string][]string{"old..main": {"tip"}}))
	if status.Health == nil || len(status.Health.Goals) != 1 || status.Health.Goals[0].State != StateLanded || *status.Health.Goals[0].Minutes != 85 {
		t.Fatalf("landed health: %+v, problems %v", status.Health, status.Problems)
	}
}

func TestLandingHealthAdmissionUsesQueueAtSeal(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	b.record.CustodyEpoch = 1
	seal := bedNow.Add(-5 * time.Minute)
	b.lines("queue.jsonl", Line{Goal: "g", SHA: "tip", At: seal.Add(-2 * time.Minute).Format(time.RFC3339)}, Line{Goal: "g", SHA: "tip", Outcome: StateWaiting, Held: true, Reason: "held after seal", At: seal.Add(time.Minute).Format(time.RFC3339)})
	batch := &Batch{ID: "batch", Base: "main", Lane: b.record, State: BatchPrepared, CreatedAt: seal.Add(-3 * time.Minute).Format(time.RFC3339), SealedAt: seal.Format(time.RFC3339), Selector: PolicyValue{Value: "auto"}, Members: []GoalSHA{{Goal: "member", SHA: "member-tip"}}}
	if err := writeBatch(b.install, batch); err != nil {
		t.Fatal(err)
	}
	h := readHealthBed(t, b, healthSeams(bedNow)).Health
	if healthRow(h, "batch-of-one") == nil || h.Batch.Waiting[0].Admission != "admissible" {
		t.Fatalf("a later hold rewrote boundary eligibility: %+v", h.Batch)
	}
}

func TestLandingHealthAdmissionUsesIncidentsAtSeal(t *testing.T) {
	t.Parallel()
	seal := bedNow.Add(-5 * time.Minute)
	before, after := seal.Add(-time.Minute).Format(time.RFC3339), seal.Add(time.Minute).Format(time.RFC3339)
	for _, tc := range []struct {
		name, opened, closed, want, reason string
	}{
		{"open at seal", before, "", "not admissible", "held: main has an open incident"},
		{"closed after seal", before, after, "not admissible", "held: main has an open incident"},
		{"closed at seal", before, seal.Format(time.RFC3339), "admissible", "automatic"},
		{"opened at seal", seal.Format(time.RFC3339), "", "not admissible", "held: main has an open incident"},
		{"opened after seal", after, "", "admissible", "automatic"},
		{"unknown opening", "", "", "unknown", "incidents"},
		{"malformed opening", "unrecorded", "", "unknown", "incidents"},
		{"malformed closure", before, "unrecorded", "unknown", "incidents"},
		{"closure before opening", after, before, "unknown", "incidents"},
		{"no incident", "", "", "admissible", "automatic"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b := newStatusBed(t)
			b.record.CustodyEpoch = 1
			b.lines("queue.jsonl", Line{Goal: "g", SHA: "tip", At: before})
			batch := &Batch{ID: "batch", Base: "main", Lane: b.record, State: BatchPrepared, CreatedAt: before, SealedAt: seal.Format(time.RFC3339), Selector: PolicyValue{Value: "auto"}, Members: []GoalSHA{{Goal: "member", SHA: "member-tip"}}}
			if err := writeBatch(b.install, batch); err != nil {
				t.Fatal(err)
			}
			s := healthSeams(bedNow)
			s.Incidents = func(string, string, string) ([]goal.TrunkRedEntry, error) {
				if tc.name == "no incident" {
					return nil, nil
				}
				incident := goal.TrunkRedEntry{ID: "red", Identity: "red", Opened: tc.opened}
				if tc.closed != "" {
					incident.Closed = &goal.TrunkRedClosure{At: tc.closed}
				}
				return []goal.TrunkRedEntry{incident}, nil
			}
			h := readHealthBed(t, b, s).Health
			if h.Batch == nil || len(h.Batch.Waiting) != 1 {
				t.Fatalf("batch=%+v", h.Batch)
			}
			fact := h.Batch.Waiting[0]
			if fact.Admission != tc.want || !strings.Contains(fact.Reason, tc.reason) || (healthRow(h, "batch-of-one") != nil) != (tc.want == "admissible") {
				t.Fatalf("fact=%+v rows=%+v", fact, h.Rows())
			}
		})
	}
}

func TestLandingHealthBatchShowsSelectedWaitingMembers(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	b.record.CustodyEpoch = 1
	at := bedNow.Add(-10 * time.Minute).Format(time.RFC3339)
	b.lines("queue.jsonl", Line{Goal: "g", SHA: "tip", At: at})
	batch := &Batch{ID: "batch", Base: "main", Lane: b.record, State: BatchPrepared, CreatedAt: at, SealedAt: bedNow.Add(-5 * time.Minute).Format(time.RFC3339), Selector: PolicyValue{Value: "auto"}, Members: []GoalSHA{{Goal: "g", SHA: "tip"}}}
	if err := writeBatch(b.install, batch); err != nil {
		t.Fatal(err)
	}
	h := readHealthBed(t, b, healthSeams(bedNow)).Health
	if h.Batch == nil || len(h.Batch.Waiting) != 1 || h.Batch.Waiting[0].Goal != "g" || !strings.Contains(h.Batch.Waiting[0].Reason, "already selected") || healthRow(h, "batch-of-one") != nil {
		t.Fatalf("selected waiting facts: %+v", h.Batch)
	}
}
