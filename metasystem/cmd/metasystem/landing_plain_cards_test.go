package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

func TestPlainLanePushWritesLandedCards(t *testing.T) {
	t.Parallel()
	b := newDeployVerbBed(t, false)
	b.pushed = plain.PushOutcome{Old: "old", Commit: "head", Changed: true}
	owners := b.pushOwners()
	owners.lookupEnv = func(key string) (string, bool) {
		return filepath.Dir(b.home), key == "METASYSTEM_SUPERVISION_REGISTRY_HOME"
	}
	inside := map[string]bool{}
	owners.landing.contained = func(_, main string) func(string) (bool, error) {
		return func(sha string) (bool, error) { return main == "head" && inside[sha], nil }
	}
	counts := map[string]int{}
	owners.landing.unitsOnMain = func(_, main, goal string) (int, error) {
		counts[goal]++
		if main != "head" {
			t.Fatalf("count read at %s", main)
		}
		if goal == "unreadable" {
			return 0, errors.New("unit count unreadable")
		}
		if goal == "released" {
			card, _ := board.LiveCard(b.home, goal)
			card.Stage = board.StageReleased
			if err := board.WriteAt(b.home, card); err != nil {
				t.Fatal(err)
			}
		}
		return 3, nil
	}
	seat := board.Seat{Machine: "seat", Installation: "/seat/metasystem"}
	for _, goal := range []string{"ready", "building", "newer", "superseded", "missing", "unreadable", "released", "returned"} {
		sha := goal + "-sha"
		inside[sha] = true
		if _, _, err := plain.HandIn(b.root, plain.Line{Goal: goal, SHA: sha}); err != nil {
			t.Fatal(err)
		}
		if goal == "missing" {
			continue
		}
		card := board.Card{Seat: seat, Goal: goal, Stage: board.StageLandReady, Round: &board.Round{N: 2}, Writer: board.Writer{At: b.clock.Add(-time.Hour)}}
		if goal == "building" {
			card.Stage, card.Owner, card.Job = board.StageBuild, board.Self(), &board.Job{ID: "build-job"}
		}
		if err := board.WriteAt(b.home, card); err != nil {
			t.Fatal(err)
		}
	}
	for _, goal := range []string{"newer", "superseded"} {
		sha := goal + "-new"
		inside[sha] = goal == "superseded"
		if _, _, err := plain.HandIn(b.root, plain.Line{Goal: goal, SHA: sha}); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := plain.Return(b.root, "returned", "fix needed", b.clock); err != nil {
		t.Fatal(err)
	}
	code, result := b.push(owners)
	expectOutcome(t, "push", code, result, intentConfirmed)
	for goal, stage := range map[string]board.Stage{"ready": board.StageClaimedIdle, "building": board.StageBuild, "newer": board.StageJoined, "superseded": board.StageClaimedIdle} {
		card, live := board.LiveCard(b.home, goal)
		if !live || card.Stage != stage || card.Landed != 3 || card.Round == nil || card.Round.N != 2 || counts[goal] != 1 {
			t.Fatalf("%s after push: %+v live=%v counts=%v", goal, card, live, counts)
		}
		if goal == "building" {
			if card.Owner == nil || *card.Owner != *board.Self() || card.Job == nil || card.Job.ID != "build-job" || !card.LastProgressAt.Equal(b.clock) {
				t.Fatalf("the running build lost its owner, job or progress: %+v", card)
			}
		} else if card.Owner != nil || card.Job != nil || card.Proof != nil || card.Writer.Component != "landing-push" {
			t.Fatalf("waiting card kept work markers: %+v", card)
		}
		if goal != "building" {
			picture, _ := board.Read(b.home, []board.Seat{seat}, nil, b.clock.Add(time.Hour), time.Second)
			for _, unknown := range picture.Unknown {
				if unknown.Goal == goal {
					t.Fatalf("waiting card stalled: %+v", unknown)
				}
			}
		}
	}
	for _, goal := range []string{"unreadable", "released"} {
		if !detailsHave(result, "the landed card for "+goal+" was not written") {
			t.Fatalf("untold card %s: %+v", goal, result)
		}
	}
	if detailsHave(result, "the landed card for missing") || counts["missing"] != 0 {
		t.Fatalf("missing card was reported or counted: %+v counts=%v", result, counts)
	}
	if _, err := os.Stat(filepath.Join(board.Dir(b.home), seat.Machine, "missing.json")); !os.IsNotExist(err) {
		t.Fatalf("missing goal gained a card: %v", err)
	}
	if card, _ := board.LiveCard(b.home, "unreadable"); card.Stage != board.StageLandReady || card.Landed != 0 {
		t.Fatalf("unreadable count was written: %+v", card)
	}
	if counts["returned"] != 0 {
		t.Fatal("returned hand-in was selected")
	}
	owners.lookupEnv = func(string) (string, bool) { return "relative-home", true }
	if code, result = b.push(owners); code != 0 || result.Outcome != intentConfirmed || !detailsHave(result, "landed cards were not written") {
		t.Fatalf("bad board home undid the push: %d %+v", code, result)
	}
}

func TestWorkLandHandInWritesJoined(t *testing.T) {
	t.Parallel()
	for _, stage := range []board.Stage{board.StageLandReady, board.StageBuild} {
		t.Run(string(stage), func(t *testing.T) {
			b, _, _ := plainLaneBedWith(t, true, "critic-root", "critic-root")
			registry := t.TempDir()
			home := filepath.Join(registry, ".metasystem")
			seat := board.Seat{Machine: "seat", Installation: b.install}
			card := board.Card{Seat: seat, Goal: "standing-validation", Stage: stage, Landed: 2, Batch: "batch", Writer: board.Writer{At: time.Now()}}
			if stage == board.StageBuild {
				card.Owner, card.Job = board.Self(), &board.Job{ID: "running"}
			}
			if err := board.WriteAt(home, card); err != nil {
				t.Fatal(err)
			}
			owners := b.intentBed.owners()
			owners.delivery, owners.connection = b.owners, b.connection
			owners.work = b.work
			owners.lookupEnv = func(key string) (string, bool) { return registry, key == "METASYSTEM_SUPERVISION_REGISTRY_HOME" }
			owners.landing.by = func(string) string { return "seat" }
			code, result := b.runJSON(owners, "work", "land", card.Goal)
			expectOutcome(t, "hand-in", code, result, intentConfirmed)
			current, live := board.LiveCard(home, card.Goal)
			if !live || current.Landed != 2 {
				t.Fatalf("hand-in lost count or card: %+v", current)
			}
			if stage == board.StageBuild {
				if current.Stage != stage || current.Owner == nil || current.Job == nil || current.Job.ID != "running" || current.Batch != "batch" {
					t.Fatalf("hand-in replaced running work: %+v", current)
				}
			} else if current.Stage != board.StageJoined || current.Owner != nil || current.Job != nil || current.Batch != "" {
				t.Fatalf("hand-in did not wait: %+v", current)
			}
			if strings.Contains(strings.Join(result.Details, "\n"), "hand-in card") {
				t.Fatalf("card write failed: %+v", result)
			}
		})
	}
}

func TestWorkLandHandInCardDetails(t *testing.T) {
	t.Parallel()
	for _, state := range []string{"missing", "released", "ambiguous", "write-failed"} {
		t.Run(state, func(t *testing.T) {
			t.Parallel()
			b, _, _ := plainLaneBedWith(t, true, "critic-root", "critic-root")
			registry := t.TempDir()
			home := filepath.Join(registry, ".metasystem")
			card := board.Card{Seat: board.Seat{Machine: "seat", Installation: b.install}, Goal: "standing-validation", Stage: board.StageLandReady}
			if state == "released" {
				card.Stage = board.StageReleased
			}
			if state != "missing" {
				if err := board.WriteAt(home, card); err != nil {
					t.Fatal(err)
				}
			}
			if state == "ambiguous" {
				other := card
				other.Seat.Machine = "other-seat"
				if err := board.WriteAt(home, other); err != nil {
					t.Fatal(err)
				}
			}
			lock := filepath.Join(board.Dir(home), card.Seat.Machine, ".lock")
			if state == "write-failed" {
				if err := os.Remove(lock); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(lock, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			owners := b.intentBed.owners()
			owners.delivery, owners.connection = b.owners, b.connection
			owners.work = b.work
			owners.lookupEnv = func(key string) (string, bool) { return registry, key == "METASYSTEM_SUPERVISION_REGISTRY_HOME" }
			owners.landing.by = func(string) string { return "seat" }
			code, result := b.runJSON(owners, "work", "land", card.Goal)
			expectOutcome(t, "hand-in", code, result, intentConfirmed)
			if state == "write-failed" {
				if len(result.Details) != 1 || !strings.Contains(result.Details[0], "the hand-in card for "+card.Goal+" was not written: ") || !strings.Contains(result.Details[0], lock) {
					t.Fatalf("card write failure was not named: %+v", result)
				}
				if current, live := board.LiveCard(home, card.Goal); !live || current.Stage != board.StageLandReady {
					t.Fatalf("failed write changed the card: %+v live=%v", current, live)
				}
			} else if len(result.Details) != 0 {
				t.Fatalf("hand-in without a single live card has details: %+v", result)
			}
		})
	}
}
