package plain

import (
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/loopstop"
)

func TestLaneQuestionsCarryTheirWords(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"stop", "proof-hold-tree", "proof-hold-batch", "classify", "gate", "main", "lane", "empty"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b := newStewardBed(t)
			now := time.Date(2031, 1, 1, 9, 0, 0, 0, time.UTC)
			seams := ProveSeams{Now: func() time.Time { return now }}
			tree, batch, attempt := strings.Repeat("a", 40), "batch-123456abcdef", "20261020T193000.000000000Z"
			want, identity := "The landing check for g needs your decision", ""
			switch name {
			case "proof-hold-tree", "proof-hold-batch":
				running := Running{Tree: tree}
				if name == "proof-hold-batch" {
					running.BatchID, identity = batch, batch
				} else {
					identity = tree
				}
				err := withLock(b.install, func() error {
					return holdProofLocked(b.install, running, seams, "LANE_PROOF_PERSON", "full check needs a person")
				})
				var refused *Refusal
				if err != nil && !errors.As(err, &refused) {
					t.Fatal(err)
				}
				want = "The landing check needs your decision"
			case "classify":
				seams.Policy = func(string) (PolicyValue, error) { return PolicyValue{Value: "person"}, nil }
				err := withLock(b.install, func() error {
					return redContinuationLocked(b.install, Result{Attempt: attempt, Tree: tree, Scope: "full", Cause: &loopstop.Cause{Kind: "own", Tests: []string{"TestX"}}, Log: t.TempDir() + "/proof.log"}, seams)
				})
				var refused *Refusal
				if !errors.As(err, &refused) {
					t.Fatalf("classification hold: %v", err)
				}
				want, identity = "The landing check failed and needs your classification", attempt
			default:
				stop := Stop{Loop: "lane-proof", Subject: "g", ProofAttempt: "proof", Decision: "stop", Handoff: "ask lane", At: now.Format(time.RFC3339Nano)}
				switch name {
				case "gate":
					stop.Loop = "lane-gate"
					want = "The landing gate for g needs your decision"
				case "main":
					stop.Subject = "main"
					want = "The landing check of main needs your decision"
				case "lane":
					stop.Subject = "lane"
					want = "The landing lane needs your decision"
				case "empty":
					stop.Subject = ""
					want = "The landing check needs your decision"
				}
				b.lines("stops.jsonl", stop)
			}
			if err := SyncPolicyQuestion(b.install, func(string) (string, error) { return "landing", nil }, now); err != nil {
				t.Fatal(err)
			}
			qs, unread := channel.WalkOpenQuestions(b.install)
			if len(qs) != 1 || len(unread) != 0 {
				t.Fatalf("lane questions: %+v %v", qs, unread)
			}
			q, err := channel.ReadQuestion(b.install, qs[0].ID)
			if err != nil || q.Plain == nil || *q.Plain != (channel.Plain{Decision: want, IfNothing: "the stop stays; this question closes when a later lane record closes it"}) {
				t.Fatalf("lane words: %+v, want %q, %v", q.Plain, want, err)
			}
			post := want + ".\nTo go on: " + channel.LaneStopCommand(q) + "\nIf you do nothing: the stop stays; this question closes when a later lane record closes it."
			if q.Text != post {
				t.Fatalf("lane post = %q, want %q", q.Text, post)
			}
			if (name == "proof-hold-tree" || name == "proof-hold-batch") && (regexp.MustCompile(`[0-9a-f]{12}`).MatchString(q.Text) || strings.Contains(q.Text, batch)) {
				t.Fatalf("identity in proof post: %s", q.Text)
			}
			words := q.Plain.Decision + q.Plain.IfNothing
			if regexp.MustCompile(`[0-9a-f]{12}`).MatchString(words) || strings.Contains(words, "20261020T193000") {
				t.Fatalf("identity in words: %s", words)
			}
			if identity != "" && !strings.Contains(strings.Join(q.Facts, " "), identity) {
				t.Fatalf("lost record identity: %q", q.Facts)
			}
			if name == "classify" && !strings.Contains(channel.LaneStopCommand(q), "--classify "+attempt) {
				t.Fatalf("lost classification command: %+v", q)
			}
		})
	}
}
