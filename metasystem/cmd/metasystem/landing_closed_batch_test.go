package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

func TestClosedBatchNeverBlocksTheNextSelection(t *testing.T) {
	t.Parallel()
	for _, reason := range []string{"returned", "all selected members are accounted for on main or by queue outcomes", "confirmed push accounts for the selected members"} {
		for _, missing := range []string{"none", "commit", "queue"} {
			for _, active := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/active=%t", reason, missing, active), func(t *testing.T) {
					t.Parallel()
					b := newReplayVerbBed(t)
					b.prepareBatch(t)
					batch, err := plain.ReadBatch(b.install)
					helmMust(t, err)
					batch.State, batch.ClosureReason = plain.BatchClosed, reason
					data, err := json.Marshal(batch)
					helmMust(t, err)
					helmMust(t, os.WriteFile(filepath.Join(plain.Dir(b.install), "batch.json"), data, 0600))
					if missing == "queue" {
						helmMust(t, os.Remove(filepath.Join(plain.Dir(b.install), "queue.jsonl")))
					} else {
						for _, member := range batch.Members {
							_, _, err := plain.ReturnProven(b.install, member.Goal, "own", "fixture returned", true, "fixture", laneTestNow)
							helmMust(t, err)
						}
					}
					_, _, err = plain.HandIn(b.install, plain.Line{Goal: "next", SHA: "sha-next"})
					helmMust(t, err)
					seams := b.owners.landing.plainProve
					seams.AgentRunning = func() (bool, error) { return active, nil }
					seams.Policy = func(string) (plain.PolicyValue, error) { return plain.PolicyValue{Value: "auto"}, nil }
					falseState := replayFalseState(t)
					git := seams.Git
					seams.Git = func(dir string, args ...string) (string, error) {
						if missing == "commit" && args[0] == "cat-file" && strings.HasPrefix(args[2], "sha-a^") {
							return "", fmt.Errorf("git cat-file: %w", &exec.ExitError{ProcessState: falseState})
						}
						if args[0] == "merge-base" && strings.HasPrefix(args[2], "sha-") {
							return "", fmt.Errorf("git merge-base: %w", &exec.ExitError{ProcessState: falseState})
						}
						return git(dir, args...)
					}
					selected, err := plain.SelectBatch(b.install, b.root, batch.Lane, seams)
					if err != nil || selected == nil || selected.ID == batch.ID || selected.State != plain.BatchPrepared || selected.Members[len(selected.Members)-1].Goal != "next" {
						t.Fatalf("closed batch blocked next selection: selected=%+v err=%v", selected, err)
					}
				})
			}
		}
	}
}

func TestClosedBatchWithNoEligibleGoalsSelectsNothing(t *testing.T) {
	t.Parallel()
	for _, held := range []bool{false, true} {
		t.Run(fmt.Sprintf("held=%t", held), func(t *testing.T) {
			t.Parallel()
			b := newReplayVerbBed(t)
			b.prepareBatch(t)
			batch, err := plain.ReadBatch(b.install)
			helmMust(t, err)
			batch.State, batch.ClosureReason = plain.BatchClosed, "returned"
			data, err := json.Marshal(batch)
			helmMust(t, err)
			helmMust(t, os.WriteFile(filepath.Join(plain.Dir(b.install), "batch.json"), data, 0600))
			helmMust(t, os.Remove(filepath.Join(plain.Dir(b.install), "queue.jsonl")))
			if held {
				line := plain.Line{Goal: "next", SHA: "sha-next"}
				initial, err := json.Marshal(line)
				helmMust(t, err)
				line.Outcome, line.Held, line.Reason = plain.StateWaiting, true, "environment hold"
				data, err := json.Marshal(line)
				helmMust(t, err)
				lines := append(append(initial, '\n'), append(data, '\n')...)
				helmMust(t, os.WriteFile(filepath.Join(plain.Dir(b.install), "queue.jsonl"), lines, 0600))
			}
			seams := b.owners.landing.plainProve
			seams.AgentRunning = func() (bool, error) { return true, nil }
			seams.Policy = func(string) (plain.PolicyValue, error) { return plain.PolicyValue{Value: "person"}, nil }
			selected, err := plain.SelectBatch(b.install, b.root, batch.Lane, seams)
			if err != nil || selected != nil {
				t.Fatalf("closed batch returned as selected: selected=%+v err=%v", selected, err)
			}
			stored, err := plain.ReadBatch(b.install)
			helmMust(t, err)
			if stored.ID != batch.ID || stored.State != plain.BatchClosed {
				t.Fatalf("closed history changed: %+v", stored)
			}
		})
	}
}
