package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/conflict"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

func TestLandingRestartKeepsHeldSupersededMember(t *testing.T) {
	t.Parallel()
	b := newSelectionBed(t)
	b.owners.prove = enrolledPersonProver(t, b.seat, b.now)
	members := []plain.GoalSHA{{Goal: "a", SHA: "sha-a"}}
	batch := plain.Batch{ID: "batch", Lane: b.record, Base: "main", Members: members, State: plain.BatchPrepared, SealedAt: b.now.Format(time.RFC3339Nano)}
	writeQuestionFixture(t, filepath.Join(plain.Dir(b.lane), "batch.json"), batch)
	_, _, err := plain.HandIn(b.lane, plain.Line{Goal: "a", SHA: "sha-a-2", At: b.now.Add(time.Minute).Format(time.RFC3339Nano)})
	helmMust(t, err)
	b.owners.landing.plainProve.Incidents = func(string, string, string) ([]goal.TrunkRedEntry, error) {
		return []goal.TrunkRedEntry{{Identity: "red", Opened: b.now.Format(time.RFC3339)}}, nil
	}
	writeQuestionFixture(t, lane.AgentStatePath(b.home), lane.AgentState{Barren: 2, BarrenFingerprint: "same"})
	b.keeper.Fingerprint = func(string) (string, error) { return "same", nil }
	code, out := b.run(t, b.seat, "landing", "restart", "--json")
	got := b.batch(t)
	if code != 0 || got.ID != batch.ID || !slices.Equal(got.Members, members) || len(got.Actions) != 1 || got.Actions[0].Kind != "restart" || got.Actions[0].Pending || len(got.Actions[0].With) != 0 || got.Person == nil || !slices.Equal(got.Person.Subject, members) || b.starts != 0 {
		t.Fatalf("restart exit=%d starts=%d batch=%+v output=%s", code, b.starts, got, out)
	}
	state, err := lane.ReadAgentState(b.home)
	helmMust(t, err)
	if state.Barren != 2 {
		t.Fatalf("superseded selection lifted hold: %+v", state)
	}
	b.owners.prove = enrolledPersonProver(t, b.lane, b.now)
	code, out = b.run(t, b.lane, "landing", "run", "--goals", "a", "--json")
	got = b.batch(t)
	state, err = lane.ReadAgentState(b.home)
	helmMust(t, err)
	if code != 0 || !slices.Equal(got.Members, []plain.GoalSHA{{Goal: "a", SHA: "sha-a-2"}}) || b.starts != 1 || state.Barren != 0 {
		t.Fatalf("explicit selection exit=%d starts=%d state=%+v batch=%+v output=%s", code, b.starts, state, got, out)
	}
	t.Logf("restart and explicit run: exit 0; held old commit preserved, next selection started the agent")
}

func TestLandingResumePreservedMergeOutcome(t *testing.T) {
	t.Parallel()
	for _, paused := range []bool{true, false} {
		t.Run(map[bool]string{true: "paused", false: "running"}[paused], func(t *testing.T) {
			t.Parallel()
			b := newSelectionBed(t)
			b.owners.prove = enrolledPersonProver(t, b.seat, b.now)
			b.owners.landing.person = func(string) (string, error) { return "Wido", nil }
			store := launch.Store{Root: t.TempDir()}
			helmMust(t, store.Create(launch.Record{ID: "job", State: launch.Completed}))
			b.owners.landing.plainProve.ResolutionStore = &store
			git := b.owners.landing.plainProve.Git
			b.owners.landing.plainProve.Git = func(dir string, args ...string) (string, error) {
				if args[0] == "rev-parse" && strings.Contains(args[len(args)-1], "MERGE_HEAD") {
					return "tip", nil
				}
				return git(dir, args...)
			}
			fix := &plain.Fix{Attempt: "fix", Goal: "a", Job: "job", Units: []string{"p"}, State: "resolving", Commit: "main", Tip: "tip", Paths: []conflict.Path{{Path: "app.go"}}}
			helmMust(t, plain.WriteFix(b.lane, fix))
			b.keeper = lane.AgentKeeper{Home: b.home, Self: b.lane, Now: func() time.Time { return b.now },
				Running: func() (string, bool, error) { return "agent", !paused, nil },
				Start:   func(string, lane.Wake) (string, error) { b.starts++; return "agent", nil },
				Sources: lane.WakeSources{Reasons: func(string) ([]string, error) { return []string{plain.WakeQueued}, nil }},
			}
			if paused {
				_, err := lane.SetPause(b.home, "Wido", b.now)
				helmMust(t, err)
				_, _, err = plain.SetDrain(b.lane, plain.Drain{By: "Wido", At: b.now.Format(time.RFC3339)})
				helmMust(t, err)
			}
			before, err := os.ReadFile(filepath.Join(plain.Dir(b.lane), "fixes", "fix.json"))
			helmMust(t, err)
			code, out := b.run(t, b.seat, "landing", "resume", "--json")
			var result intentResult
			helmMust(t, json.Unmarshal([]byte(out), &result))
			want, starts := intentUnchanged, 0
			if paused {
				want, starts = intentConfirmed, 1
			}
			after, err := os.ReadFile(filepath.Join(plain.Dir(b.lane), "fixes", "fix.json"))
			helmMust(t, err)
			_, stillPaused := lane.ReadPause(b.home)
			drain, err := plain.ReadDrain(b.lane)
			helmMust(t, err)
			if code != 0 || result.Outcome != want || b.starts != starts || stillPaused || drain != nil || string(before) != string(after) {
				t.Fatalf("resume exit=%d starts=%d paused=%v drain=%+v outcome=%s want=%s output=%s", code, b.starts, stillPaused, drain, result.Outcome, want, out)
			}
			t.Logf("landing resume --json: exit=%d output=%s", code, out)
		})
	}
}
