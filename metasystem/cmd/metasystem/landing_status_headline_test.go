package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

func TestLandingStatusKeepsQuestionAndMissingHandInHeadlines(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"question", "missing hand-in", "red and question"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			question := mode != "missing hand-in"
			bed, _, _, _, _ := landingRestartBed(t)
			owners := bed.owners()
			owners.landing.machine = func(string) (string, error) { return "landing", nil }
			want := "hand-in next at missing is not on origin"
			owners.landing.wake = func(string) lane.WakeSources {
				return lane.WakeSources{Reasons: func(string) ([]string, error) {
					if question {
						return nil, nil
					}
					return nil, errors.New(want)
				}}
			}
			if question {
				q, err := channel.Ask(channel.AskRequest{RepoRoot: bed.landingA, About: "lane", Kind: "other", Machine: "landing", Lineage: lane.AgentLineage, Facts: []string{"May I return the checkout to main?"}, Now: laneTestNow})
				if err != nil {
					t.Fatal(err)
				}
				want = "Waiting for a person's answer to question " + q.ID
			}
			if mode == "red and question" {
				writeCauseProof(t, bed.landingA, "results.jsonl", plain.Result{Trunk: true, Commit: "main", Tree: "main-tree", Result: plain.Red, Reason: "check failed", At: laneTestNow.Format(time.RFC3339)})
			}
			command, _ := findIntentAction("landing", "status")
			var out, problem bytes.Buffer
			code := runIntentIn(command, nil, &out, &problem, bed.cwd, owners)
			first := strings.Split(strings.TrimSpace(out.String()), "\n")[0]
			if mode == "red and question" {
				if !strings.HasPrefix(first, "main main proven red: check failed") {
					t.Fatalf("red headline hidden: %q", first)
				}
				out.Reset()
				code = runIntentIn(command, []string{"--json"}, &out, &problem, bed.cwd, owners)
				var result struct{ Summary string }
				if err := json.Unmarshal(out.Bytes(), &result); err != nil {
					t.Fatal(err)
				}
				first = result.Summary
			}
			if mode == "red and question" && (!strings.HasPrefix(first, "main main proven red: check failed") || strings.Index(first, "proven red") > strings.Index(first, want)) {
				t.Fatalf("red headline hidden by question: %q", first)
			}
			if code != 0 || !strings.Contains(first, want) {
				t.Fatalf("headline overwritten: exit=%d first=%q out=%s err=%s", code, first, &out, &problem)
			}
		})
	}
}
