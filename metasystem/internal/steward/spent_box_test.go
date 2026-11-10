package steward

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func spentGoal(id string) *goal.GoalFile {
	file := claimQuotaHeld(id)
	file.Claimed.At = claimQuotaClock.Format(time.RFC3339)
	file.Claimed.EpisodeAt = file.Claimed.At
	file.Claimed.EpisodeRevision = 3
	file.History[len(file.History)-1].At = file.Claimed.At
	return file
}

type spentTickBed struct {
	channelFixture
	goals []*goal.GoalFile
	cfg   TickConfig
}

func newSpentTickBed(t *testing.T, goals ...*goal.GoalFile) *spentTickBed {
	t.Helper()
	if len(goals) == 0 {
		goals = []*goal.GoalFile{spentGoal("spent-a")}
	}
	fixture := newUnservedChannelFixture(t)
	conf, err := os.ReadFile(filepath.Join(fixture.root, "metasystem.conf"))
	if err != nil {
		t.Fatal(err)
	}
	conf = append(conf, []byte(config.ElapsedGracePercentKey+"=50\n")...)
	files := map[string][]byte{"metasystem.conf": conf}
	for _, file := range goals {
		seat, err := json.Marshal(map[string]string{"launchId": "seat-" + file.Id, "goal": file.Id,
			"startedAt": claimQuotaClock.Format(time.RFC3339), "reapedAt": claimQuotaClock.Add(8 * time.Hour).Format(time.RFC3339)})
		if err != nil {
			t.Fatal(err)
		}
		files["artifacts/agents/steward/seats/"+file.Id+".json"] = seat
	}
	fixture.root = claimQuotaTickBed(t, files, goals...)
	return &spentTickBed{channelFixture: fixture, goals: goals, cfg: TickConfig{AskHome: t.TempDir(),
		Units: func(_ string, id string) ([]UnitStage, error) {
			if id == "spent-a" {
				return []UnitStage{{Stage: "built, ready for review"}}, nil
			}
			return nil, nil
		}}}
}

func (b *spentTickBed) project(t *testing.T, file *goal.GoalFile, at time.Time, elapsed time.Duration, band dispatch.ElapsedBudgetState) {
	t.Helper()
	p := dispatch.ProjectBudget(b.root, file, at)
	if p.Status != dispatch.BudgetKnown || p.Elapsed != elapsed || p.ElapsedState != band {
		t.Fatalf("%s at %s: projection %+v unknown %+v, want known, %s, %s", file.Id, at, p, p.Unknown, elapsed, band)
	}
}

func (b *spentTickBed) tick(t *testing.T, at time.Time) TickResult {
	t.Helper()
	b.cfg.Now = at
	result, err := RunTick(b.root, b.cfg, fakeCensus{})
	if err != nil {
		t.Fatal(err)
	}
	duties, err := loadDuties(b.root)
	if duty, ok := duties["spent-box-question"]; err != nil || !ok || duty.Error != "" || !duty.At.Equal(at) {
		t.Fatalf("spent-box duty = %+v, %v", duties, err)
	}
	return result
}

func spentQuestions(t *testing.T, root string, open bool) []channel.Question {
	t.Helper()
	all, bad := channel.WalkQuestions(root)
	if len(bad) != 0 {
		t.Fatalf("unreadable questions: %v", bad)
	}
	var out []channel.Question
	for _, q := range all {
		if !open || q.State == "open" {
			out = append(out, q)
		}
	}
	return out
}

func oneSpentQuestion(t *testing.T, b *spentTickBed) channel.Question {
	t.Helper()
	qs := spentQuestions(t, b.root, true)
	if len(qs) != 1 || qs[0].Decision == nil || qs[0].Decision.Kind != channel.SpentBoxDecision {
		t.Fatalf("want one spent-box question: %+v", qs)
	}
	return qs[0]
}

func (b *spentTickBed) ask(t *testing.T) channel.Question {
	t.Helper()
	b.project(t, b.goals[0], claimQuotaClock.Add(5*time.Hour), 5*time.Hour, dispatch.AdmissionClosedElapsed)
	b.tick(t, claimQuotaClock.Add(5*time.Hour))
	return oneSpentQuestion(t, b)
}

func wantSpentRetired(t *testing.T, root, id string) {
	t.Helper()
	q, err := channel.ReadQuestion(root, id)
	if err != nil || q.State != "closed" || q.RetiredAt.IsZero() || q.ClosedBecause == "" {
		t.Fatalf("question not retired: %+v %v", q, err)
	}
}

// Executed work keeps the clock counting while the person has a question.
func (b *spentTickBed) executeThroughQuestion(t *testing.T, id string) {
	t.Helper()
	data, err := json.Marshal(map[string]any{"runtime": "local", "jobId": "working-" + id, "operationId": "working-" + id,
		"goalId": id, "goalRevision": 3, "capMin": 1, "status": "completed",
		"startedAt": claimQuotaClock.Add(5 * time.Hour).Format(time.RFC3339), "endedAt": claimQuotaClock.Add(7 * time.Hour).Format(time.RFC3339)})
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(b.root, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "working-"+id+".json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestASpentBoxAsksOnceThroughTheTick(t *testing.T) {
	t.Parallel()
	t.Run("asks once", func(t *testing.T) {
		t.Parallel()
		b := newSpentTickBed(t)
		q := b.ask(t)
		decision := "Goal spent a has used its 4h box; nothing new starts. It has built work waiting."
		raised := "its time box becomes 2d4h, two more working days; the other limits stay and may still block work. Undo: goal budget back to 4h."
		left := "nothing new starts; running work may still spend. This question stays open, so you can still raise the box."
		want := channel.Decision{Kind: channel.SpentBoxDecision, Subject: "spent-a", State: dispatch.SpentBoxState(b.goals[0], b.cfg.Now)}
		if !q.Contract || *q.Decision != want || !strings.HasPrefix(q.Decision.State, "4h/4/240m/2/0 from ") || q.Recommendation != "Raise the box" || len(q.Options) != 2 ||
			!slices.Equal(q.Options[0].Act, []string{"metasystem", "goal", "budget", "spent-a", "2d4h/4/240m/2/0"}) || q.Options[1].Act != nil || q.Options[0].Consequence != raised || q.Options[1].Consequence != left ||
			!strings.HasPrefix(q.Text, decision+"\n\n★ Raise the box — "+raised+"\nLeave it — "+left) || !strings.Contains(q.Text, "If you do nothing: the goal stands until you raise its box or release it.") {
			t.Fatalf("spent-box contract = %+v\n%s", q, q.Text)
		}
		b.project(t, b.goals[0], b.cfg.Now, 5*time.Hour, dispatch.AdmissionClosedElapsed)
		b.tick(t, b.cfg.Now)
		if all := spentQuestions(t, b.root, false); len(all) != 1 {
			t.Fatalf("asked twice: %+v", all)
		}
		b.serve(t)
		loaded := loadSignalChannel(b.root).loaded
		_, err := channel.Poll(context.Background(), channel.PollConfig{RepoRoot: b.root, Destination: "fleet", ProviderName: "fake", Provider: loaded.Provider, DestinationConfig: loaded.Destination, Now: b.cfg.Now})
		if err != nil {
			t.Fatal(err)
		}
		if posts := b.posts(t); len(posts) != 1 || !strings.HasPrefix(posts[0], decision) {
			t.Fatalf("poll posts: %q", posts)
		}
	})
	t.Run("retired by a raise in the ledger", func(t *testing.T) {
		t.Parallel()
		b := newSpentTickBed(t)
		q := b.ask(t)
		f := b.goals[0]
		f.Revision++
		f.Claimed.Revision = f.Revision
		f.StopCapability.Revision = f.Revision
		f.StopCapability.Generation = f.Revision
		f.Budget.ElapsedLimit = "2d4h"
		f.Claimed.At = claimQuotaClock.Add(5*time.Hour + 15*time.Minute).Format(time.RFC3339)
		f.History = append(f.History, goal.HistoryLine{At: f.Claimed.At, Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FC6", "m1e", "coordinator"), Verb: "set-budget", Actor: "human:Wido", Targets: []string{f.Id}, Keep: -1})
		f.Approved.Digest = goal.ApprovalDigest(f.Intent, f.Tier, *f.Budget, f.Risk)
		claimQuotaAccept(t, b.root, f)
		at := claimQuotaClock.Add(5*time.Hour + 30*time.Minute)
		b.project(t, f, at, 5*time.Hour+15*time.Minute, "")
		b.tick(t, at)
		wantSpentRetired(t, b.root, q.ID)
		if all := spentQuestions(t, b.root, false); len(all) != 1 {
			t.Fatalf("raise asked again: %+v", all)
		}
	})
	t.Run("a consumption extension asks anew with the extended limits", func(t *testing.T) {
		t.Parallel()
		b := newSpentTickBed(t)
		q := b.ask(t)
		f := b.goals[0]
		f.Budget.AttemptLimit = 8
		f.Budget.ReservedJobMinutesLimit = 480
		f.Revision++
		stamp := claimQuotaClock.Add(5*time.Hour + 15*time.Minute).Format(time.RFC3339)
		opid := goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FC7", "m1e", "coordinator")
		f.BudgetExtension = &goal.BudgetExtensionRecord{At: stamp, Opid: opid, AttemptLimitFrom: 4, AttemptLimitTo: 8, ReservedJobMinutesFrom: 240, ReservedJobMinutesTo: 480, EvidenceKind: "receipt", EvidenceID: "advanced", EvidenceAt: stamp}
		f.History = append(f.History, goal.HistoryLine{At: stamp, Opid: opid, Verb: "extend-budget", Actor: "m1e+coordinator", Targets: []string{f.Id}, Keep: -1, Reason: "attemptLimit 4->8 reservedJobMinutesLimit 240->480 evidence=receipt:advanced@" + stamp})
		claimQuotaAccept(t, b.root, f)
		at := claimQuotaClock.Add(5*time.Hour + 30*time.Minute)
		b.project(t, f, at, 5*time.Hour, dispatch.AdmissionClosedElapsed)
		b.tick(t, at)
		wantSpentRetired(t, b.root, q.ID)
		next := oneSpentQuestion(t, b)
		if next.ID == q.ID || !strings.HasPrefix(next.Decision.State, "4h/8/480m/2/0 from ") || !slices.Equal(next.Options[0].Act, []string{"metasystem", "goal", "budget", "spent-a", "2d4h/8/480m/2/0"}) {
			t.Fatalf("extension lost current tuple: %+v", next)
		}
	})
	t.Run("a risk raise that keeps the box keeps the question at the helm past the breach limit", func(t *testing.T) {
		t.Parallel()
		f := spentGoal("risk-g")
		f.Tier = 2
		f.Budget.ReviewRoundLimit = 5
		f.Risk = &goal.RiskRecord{Severity: 2, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "bounded change"}
		f.Approved.Digest = goal.ApprovalDigest(f.Intent, f.Tier, *f.Budget, f.Risk)
		b := newSpentTickBed(t, f)
		takeHelmFixture(t, b.root)
		q := b.ask(t)
		b.executeThroughQuestion(t, f.Id)
		f.Revision++
		f.Tier = 3
		f.Risk.Severity = 3
		f.Risk.Basis = "larger consequence observed"
		f.Claimed.Revision = f.Revision
		f.StopCapability.Revision = f.Revision
		f.StopCapability.Generation = f.Revision
		f.Approved.Revision = f.Revision
		f.Approved.At = claimQuotaClock.Add(6 * time.Hour).Format(time.RFC3339)
		f.Approved.Opid = goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FC3", "m1e", "coordinator")
		f.Approved.Authority = "raise=" + f.Approved.Opid
		f.Approved.Digest = goal.ApprovalDigest(f.Intent, f.Tier, *f.Budget, f.Risk)
		f.History = append(f.History, goal.HistoryLine{At: f.Approved.At, Opid: f.Approved.Opid, Verb: "edit", Actor: "m1e+coordinator", Targets: []string{f.Id}, Keep: -1, Reason: "Misclassified: from=2 to=3 evidence=observed"})
		claimQuotaAccept(t, b.root, f)
		b.cfg.BreachStop = func(string, uint64) (string, error) { t.Fatal("helm stopped work"); return "", nil }
		at := claimQuotaClock.Add(7 * time.Hour)
		b.project(t, f, at, 7*time.Hour, dispatch.ElapsedBreach)
		result := b.tick(t, at)
		if next := oneSpentQuestion(t, b); next.ID != q.ID || !next.RetiredAt.IsZero() || len(spentQuestions(t, b.root, false)) != 1 || len(result.GoalStops) != 0 {
			t.Fatalf("risk raise replaced question: %+v", next)
		}
	})
	t.Run("retired when the stop asks", func(t *testing.T) {
		t.Parallel()
		b := newSpentTickBed(t)
		q := b.ask(t)
		b.executeThroughQuestion(t, "spent-a")
		stops := 0
		b.cfg.BreachStop = func(string, uint64) (string, error) { stops++; return "stopped", nil }
		at := claimQuotaClock.Add(7 * time.Hour)
		for range 2 {
			b.project(t, b.goals[0], at, 7*time.Hour, dispatch.ElapsedBreach)
			b.tick(t, at)
			qs := spentQuestions(t, b.root, true)
			if len(qs) != 1 || qs[0].Decision.Kind != "breach-stop" {
				t.Fatalf("stop questions: %+v", qs)
			}
			wantSpentRetired(t, b.root, q.ID)
		}
		if stops != 2 {
			t.Fatalf("stops=%d", stops)
		}
	})
	t.Run("a recorded landing retires it and asks nothing more", func(t *testing.T) {
		t.Parallel()
		b := newSpentTickBed(t)
		q := b.ask(t)
		f := b.goals[0]
		spentLanding(f, true)
		claimQuotaAccept(t, b.root, f)
		at := claimQuotaClock.Add(5*time.Hour + 30*time.Minute)
		for range 2 {
			b.project(t, f, at, 4*time.Hour+30*time.Minute, dispatch.AdmissionClosedElapsed)
			b.tick(t, at)
			wantSpentRetired(t, b.root, q.ID)
			if qs := spentQuestions(t, b.root, true); len(qs) != 0 {
				t.Fatalf("landed goal asks: %+v", qs)
			}
		}
	})
	t.Run("an already landed goal asks nothing a handed-in goal asks", func(t *testing.T) {
		t.Parallel()
		landed, handed := spentGoal("landed-b"), spentGoal("handed-c")
		spentLanding(landed, true)
		spentLanding(handed, false)
		b := newSpentTickBed(t, landed, handed)
		at := claimQuotaClock.Add(5 * time.Hour)
		b.project(t, landed, at, 4*time.Hour+30*time.Minute, dispatch.AdmissionClosedElapsed)
		b.project(t, handed, at, 4*time.Hour+30*time.Minute, dispatch.AdmissionClosedElapsed)
		b.tick(t, at)
		if q := oneSpentQuestion(t, b); q.Goal != "handed-c" {
			t.Fatalf("hand-in suppressed: %+v", q)
		}
	})
	t.Run("built work waiting is positive evidence", func(t *testing.T) {
		t.Parallel()
		b := newSpentTickBed(t, spentGoal("run-d"), spentGoal("pub-e"), spentGoal("dirty-f"))
		b.cfg.Units = func(_ string, id string) ([]UnitStage, error) {
			stages := map[string]string{"run-d": "running", "pub-e": "reviewed; its read is collected and published (attestation abc1234)"}
			if stage := stages[id]; stage != "" {
				return []UnitStage{{Stage: stage}}, nil
			}
			return nil, nil
		}
		b.cfg.Workspace = func(_ string, id string) (bool, error) { return id == "dirty-f", nil }
		at := claimQuotaClock.Add(5 * time.Hour)
		for _, f := range b.goals {
			b.project(t, f, at, 5*time.Hour, dispatch.AdmissionClosedElapsed)
		}
		b.tick(t, at)
		qs := spentQuestions(t, b.root, true)
		if len(qs) != 3 {
			t.Fatalf("questions=%+v", qs)
		}
		for _, q := range qs {
			if strings.HasSuffix(q.Plain.Decision, "It has built work waiting.") != (q.Goal == "dirty-f") {
				t.Fatalf("false evidence: %+v", q)
			}
		}
	})
	t.Run("the helm asks and stops nothing", func(t *testing.T) {
		t.Parallel()
		b := newSpentTickBed(t)
		takeHelmFixture(t, b.root)
		b.cfg.BreachStopReady = func() bool { t.Fatal("helm checked stop readiness"); return false }
		b.cfg.BreachStop = func(string, uint64) (string, error) { t.Fatal("helm stopped work"); return "", nil }
		at := claimQuotaClock.Add(5 * time.Hour)
		b.project(t, b.goals[0], at, 5*time.Hour, dispatch.AdmissionClosedElapsed)
		if result := b.tick(t, at); result.Decision.Verdict != VerdictHelm {
			t.Fatalf("verdict=%+v", result.Decision)
		}
		q := oneSpentQuestion(t, b)
		b.executeThroughQuestion(t, "spent-a")
		at = claimQuotaClock.Add(7 * time.Hour)
		b.project(t, b.goals[0], at, 7*time.Hour, dispatch.ElapsedBreach)
		b.tick(t, at)
		if next := oneSpentQuestion(t, b); next.ID != q.ID || len(spentQuestions(t, b.root, false)) != 1 {
			t.Fatalf("helm replaced question: %+v", next)
		}
	})
	t.Run("another machine's checkout asks nothing and keeps nothing", func(t *testing.T) {
		t.Parallel()
		claim := newSpentTickBed(t)
		claim.ask(t)
		b := newSpentTickBed(t)
		seatGit(t, b.root, "config", "metasystem.goal.machine", "ui")
		at := claimQuotaClock.Add(5 * time.Hour)
		b.project(t, b.goals[0], at, 5*time.Hour, dispatch.AdmissionClosedElapsed)
		b.tick(t, at)
		if qs := spentQuestions(t, b.root, false); len(qs) != 0 {
			t.Fatalf("another machine asks: %+v", qs)
		}
		// Copy the claim machine's question into this store, as a stale local ask.
		q := oneSpentQuestion(t, claim)
		_, err := channel.Ask(channel.AskRequest{RepoRoot: b.root, Goal: q.Goal, Kind: q.Kind, Contract: q.Contract, Now: at, Recommendation: q.Recommendation, Decision: q.Decision, Facts: q.Facts, Plain: q.Plain, Options: q.Options})
		if err != nil {
			t.Fatal(err)
		}
		stale := oneSpentQuestion(t, b)
		b.project(t, b.goals[0], at, 5*time.Hour, dispatch.AdmissionClosedElapsed)
		b.tick(t, at)
		wantSpentRetired(t, b.root, stale.ID)
	})
	t.Run("the breach scanner is unchanged", func(t *testing.T) {
		t.Parallel()
		spent, breached := spentGoal("a-spent"), spentGoal("b-breached")
		breached.Claimed.At = claimQuotaClock.Add(-2 * time.Hour).Format(time.RFC3339)
		breached.Claimed.EpisodeAt = breached.Claimed.At
		breached.History[len(breached.History)-1].At = breached.Claimed.At
		b := newSpentTickBed(t, spent, breached)
		// The breached goal's own seat started with its earlier claim.
		path := filepath.Join(b.root, "artifacts/agents/steward/seats/b-breached.json")
		data, _ := json.Marshal(map[string]string{"launchId": "seat-b-breached", "goal": "b-breached", "startedAt": breached.Claimed.At, "reapedAt": claimQuotaClock.Add(8 * time.Hour).Format(time.RFC3339)})
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
		var stopped []string
		b.cfg.BreachStop = func(id string, _ uint64) (string, error) { stopped = append(stopped, id); return "stopped", nil }
		at := claimQuotaClock.Add(5 * time.Hour)
		b.project(t, spent, at, 5*time.Hour, dispatch.AdmissionClosedElapsed)
		b.project(t, breached, at, 7*time.Hour, dispatch.ElapsedBreach)
		b.tick(t, at)
		if !slices.Equal(stopped, []string{"b-breached"}) {
			t.Fatalf("stopped=%q", stopped)
		}
		qs := spentQuestions(t, b.root, true)
		found := false
		for _, q := range qs {
			if q.Goal == "a-spent" && q.Decision.Kind == channel.SpentBoxDecision {
				found = true
			}
		}
		if !found {
			t.Fatalf("spent box missing: %+v", qs)
		}
		routes, err := dispatch.FindBreachStops(b.root, at)
		if err != nil || len(routes) != 1 || routes[0].GoalID != "b-breached" {
			t.Fatalf("routes=%+v %v", routes, err)
		}
		boxes, err := dispatch.FindAdmissionClosed(b.root, at)
		if err != nil || len(boxes) != 1 || boxes[0].GoalID != "a-spent" {
			t.Fatalf("boxes=%+v %v", boxes, err)
		}
	})
}

func spentLanding(f *goal.GoalFile, landed bool) {
	f.Landing = &goal.LandingRecord{At: claimQuotaClock.Add(4*time.Hour + 30*time.Minute).Format(time.RFC3339), Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FC4", "m1e", "coordinator")}
	if landed {
		f.History = append(f.History, goal.HistoryLine{At: claimQuotaClock.Add(5*time.Hour + time.Minute).Format(time.RFC3339), Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FC5", "m1e", "coordinator"), Verb: goal.LandedVerb, Reason: "landed under landing.review.auto-after=0, tier 3 below human-from-tier=4", Actor: "m1e+coordinator", Targets: []string{f.Id}, Keep: -1})
	}
}

func TestBuiltWorkWaitingNeedsPositiveEvidence(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		stage string
		want  bool
	}{
		{"built, ready for review", true}, {"committed, ready for review", true}, {"committed and under independent examination", true}, {"committed, ready to land without a read", true},
		{"reviewed; its read is collected but not yet published (attestation abc1234)", true}, {"review refused: x", true}, {"the read of attempt 2 could not be collected twice; …", true},
		{"starting", false}, {"running", false}, {"cancelled; new work starts under another name", false}, {"dropped; prior reads retained", false}, {"drop pending: requested", false}, {"stopped: x; y", false}, {"stopped (failed), needs a revision", false}, {"transferred; its destinations remain required", false}, {"reviewed; its read is collected and published (attestation abc1234)", false},
	} {
		t.Run(row.stage, func(t *testing.T) {
			t.Parallel()
			cfg := TickConfig{Units: func(string, string) ([]UnitStage, error) { return []UnitStage{{Stage: row.stage}}, nil }}
			if got := builtWorkWaiting("root", cfg, "g"); got != row.want {
				t.Fatalf("waiting=%v want %v", got, row.want)
			}
		})
	}
	for _, row := range []struct {
		name string
		cfg  TickConfig
		want bool
	}{
		{"workspace", TickConfig{Workspace: func(string, string) (bool, error) { return true, nil }}, true},
		{"workspace error", TickConfig{Workspace: func(string, string) (bool, error) { return true, errors.New("unreadable") }}, false},
		{"units error", TickConfig{Units: func(string, string) ([]UnitStage, error) {
			return []UnitStage{{Stage: "committed"}}, errors.New("unreadable")
		}}, false},
		{"no readers", TickConfig{}, false},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			if got := builtWorkWaiting("root", row.cfg, "g"); got != row.want {
				t.Fatalf("waiting=%v want %v", got, row.want)
			}
		})
	}
}
