package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalrevision"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testprovider"
)

func TestGoalSplitReversesUnstartedChildren(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"claimed parent", "queued parent", "parked parent", "approved parent", "claim", "released claim", "reservation", "run", "commit", "unknown", "agent", "borrowed name", "lost confirmation", "claim at publication", "released group claim", "ungrouped claim"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			bed := newGoalCLIBed(t, goalCLISeed{allowTerminalProof: true})
			bed.announceHolder()
			home := testprovider.Register(t, bed.root)
			publicWith := func(args []string, configure func(*intentOwners)) (int, string, string) {
				t.Helper()
				command, rest, ok := resolveIntentArgv(args)
				if !ok {
					t.Fatal(args)
				}
				var out, errOut bytes.Buffer
				owners := bed.owners(&out, &errOut)
				configure(&owners)
				code := runIntentIn(command, rest, &out, &errOut, bed.root, owners)
				return code, out.String(), errOut.String()
			}

			gcliLedgerMust(t, bed, "goal", "release", "ship-widget", "--reason", "Free the seat.")
			gcliBudgetOpen(t, bed, "source", gcliBudgetTierOne, "Deliver the source.")
			if scenario != "queued parent" && scenario != "parked parent" {
				gcliBudgetHumanMust(t, bed, "goal", "approve", "source", "--budget", "norm")
			}
			if scenario == "parked parent" {
				gcliBudgetHumanMust(t, bed, "goal", "pause", "source", "--reason", "Await a decision.")
			}
			if scenario == "claimed parent" {
				gcliLedgerMust(t, bed, "goal", "claim", "source")
			}
			read := func(id string) *goal.GoalFile {
				t.Helper()
				var shown struct {
					Data struct {
						Where string
						Goal  *goal.GoalFile
					}
				}
				if err := json.Unmarshal([]byte(gcliLedgerMust(t, bed, "goal", "show", id, "--history", "--json")), &shown); err != nil {
					t.Fatal(err)
				}
				if shown.Data.Where != "live" || shown.Data.Goal == nil {
					t.Fatalf("not live: %+v", shown)
				}
				return shown.Data.Goal
			}
			before := read("source")
			if scenario == "claimed parent" {
				before.ReviewObligations = []goal.ReviewObligation{{Finding: "f1", Chain: "r1", Artifact: "reader.go", Test: "TestReader", State: "open"}}
				before.Obligation = &goal.GovernedObligation{
					Revision: before.Revision, BudgetRevision: before.Claimed.Revision, State: goal.ObligationDraft, Owner: "Wido", Effects: []goal.GoverningEffect{goal.EffectAuthorizeSpend},
					Assumptions: goal.ObligationAssumptions{Recurrence: goal.SingleExperiment, Platform: "fixture", ToolchainIdentity: "fixture", SurfaceDigest: strings.Repeat("a", 64), MaxActiveJobs: 1, TimingEnvelopeSeconds: 1800, ObservationSource: "run-terminal-record"},
					Triggers:    goal.HumanReviewTriggers{ValueJudgment: "yes", Reversibility: "compensable", SevereHarm: "unknown", UnfamiliarApproach: "no", TestDiscrimination: "strong", CorrelatedAssumptionRisk: "yes", AuthorityScopeChange: "no", DestructiveReach: "none"},
				}
				gcliLedgerInstall(t, bed, "fixture-obligation", goal.Change{Path: "plans/goals/source.md", Content: goal.RenderFile(before)})
				before = read("source")
				dir := filepath.Join(bed.root, "artifacts", "agents", "jobs")
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				data, err := json.Marshal(map[string]any{"jobId": "source-design", "operationId": "source-design", "goalId": "source", "goalRevision": before.Claimed.Revision, "status": "completed", "capMin": 5, "role": "design-critic", "reviewChainCounted": true, "parentJob": nil, "startedAt": bed.clock().Add(-time.Minute).Format(time.RFC3339), "endedAt": bed.clock().Format(time.RFC3339), "pid": 123, "pidStartedAt": 1})
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "source-design.json"), data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			spendBefore := dispatchcore.ProjectConsumption(bed.root, before, bed.clock(), home)

			plan := "# split source\n\n## member child-one\n- Intent: Build one.\n- Next step: Write one.\n\n## member child-two\n- Intent: Build two.\n- Next step: Write two.\n- BlockedBy: child-one\n"
			path := filepath.Join(bed.root, "members.md")
			if err := os.WriteFile(path, []byte(plan), 0600); err != nil {
				t.Fatal(err)
			}
			gcliLedgerMust(t, bed, "goal", "split", "source", "--plan", path, "--by", "Wido")
			split := read("source")
			if shown := gcliLedgerMust(t, bed, "goal", "show", "source"); strings.Contains(shown, "(reversed ") {
				t.Fatalf("active split shown as reversed: %s", shown)
			}
			gcliBudgetHumanMust(t, bed, "goal", "edit", "child-one", "--risk", gcliBudgetTierOne, "--basis", "child risk")
			gcliBudgetHumanMust(t, bed, "goal", "approve", "child-one", "--budget", "norm")
			if scenario == "claim" || scenario == "released claim" || scenario == "claim at publication" {
				gcliBudgetHumanMust(t, bed, "goal", "unblock", "child-one", "--on", "source")
				if scenario != "claim at publication" {
					gcliLedgerMust(t, bed, "goal", "claim", "child-one")
				}
				if scenario == "released claim" {
					gcliLedgerMust(t, bed, "goal", "release", "child-one", "--reason", "Return the seat.")
				}
			}
			if scenario == "released group claim" || scenario == "ungrouped claim" {
				gcliBudgetOpen(t, bed, "group-holder", gcliBudgetTierOne, "Work the group.")
				gcliBudgetHumanMust(t, bed, "goal", "approve", "group-holder", "--budget", "norm")
				gcliLedgerMust(t, bed, "goal", "group", "group-holder", "children")
				gcliLedgerMust(t, bed, "goal", "claim", "group-holder")
				gcliBudgetHumanMust(t, bed, "goal", "unblock", "child-one", "--on", "source")
				gcliLedgerMust(t, bed, "goal", "group", "child-one", "children")
				if read("child-one").Claimed == nil {
					t.Fatal("group did not claim child")
				}
				if scenario == "ungrouped claim" {
					gcliLedgerMust(t, bed, "goal", "ungroup", "child-one")
					child := read("child-one")
					if child.Claimed != nil || child.Episode != nil || child.Arc != "" {
						t.Fatalf("ungroup retained a claim, episode or group: %+v", child)
					}
				} else {
					gcliLedgerMust(t, bed, "goal", "release", "child-one", "--reason", "Return the group.")
					gcliBudgetHumanMust(t, bed, "goal", "approve", "child-one", "--budget", "norm")
				}
			}
			if scenario == "commit" {
				child := read("child-one")
				child.Sliced = &goal.SlicedRecord{Machine: bed.machine, Lineage: bed.lineage, Revision: child.Revision, At: bed.clock().Format(time.RFC3339)}
				gcliLedgerInstall(t, bed, "fixture-commit", goal.Change{Path: "plans/goals/child-one.md", Content: goal.RenderFile(child)})
			}
			if scenario == "reservation" || scenario == "run" || scenario == "unknown" {
				dir := filepath.Join(bed.root, "artifacts", "agents", "jobs")
				if scenario == "run" {
					dir = filepath.Join(bed.root, "artifacts", "agents", "runs")
				}
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				data, err := json.Marshal(map[string]any{"jobId": "child-work", "operationId": "child-work", "goalId": "child-one", "goalRevision": read("child-one").Revision, "status": "pending", "capMin": 5})
				if err != nil {
					t.Fatal(err)
				}
				if scenario == "unknown" {
					data = []byte("{")
				}
				if err := os.WriteFile(filepath.Join(dir, "child-work.json"), data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			args := []string{"goal", "split", "source", "--reverse", "--reason", "Keep the responsibility together.", "--by", "Wido"}
			if scenario == "agent" || scenario == "borrowed name" {
				bed.prove = newGcliAuthorityCaller(t, bed, gcliAuthorityAgent).prove
				if scenario == "agent" {
					args = args[:len(args)-2]
				}
				gcliBudgetRefused(t, bed, "enroll", args...)
				return
			}
			if scenario == "claim at publication" {
				transport := &splitConfirmRepository{Repository: bed.repo}
				unclaimed := goal.RenderFile(read("child-one"))
				gcliLedgerMust(t, bed, "goal", "claim", "child-one")
				claimed := goal.RenderFile(read("child-one"))
				gcliLedgerInstall(t, bed, "fixture-before-remote-claim", goal.Change{Path: "plans/goals/child-one.md", Content: unclaimed})
				transport.beforePublish = func() {
					transport.beforePublish = nil
					gcliLedgerInstall(t, bed, "fixture-remote-claim", goal.Change{Path: "plans/goals/child-one.md", Content: claimed})
				}
				code, out, errOut := publicWith(args, func(owners *intentOwners) {
					owners.dependencies.endpoint = func(root string) (goal.Endpoint, error) {
						ep, err := bed.endpoint(root)
						ep.Repository = transport
						return ep, err
					}
				})
				if code == 0 || !strings.Contains(out+errOut, "child-one") || read("source").State != goal.StateSplit || read("child-one").Claimed == nil {
					t.Fatalf("child start passed compensation: %d %s%s", code, out, errOut)
				}
				return
			}
			if scenario == "claim" || scenario == "released claim" || scenario == "reservation" || scenario == "run" || scenario == "commit" || scenario == "unknown" || scenario == "released group claim" || scenario == "ungrouped claim" {
				text := gcliBudgetRefused(t, bed, "child-one", args...)
				if scenario == "unknown" && !strings.Contains(text, "unreadable") {
					t.Fatal(text)
				}
				if read("source").State != goal.StateSplit {
					t.Fatal("refusal reversed the parent")
				}
				entries, err := goal.Entries(bed.root)
				if err != nil {
					t.Fatal(err)
				}
				retained := false
				for _, entry := range entries {
					if entry.Intent.Verb == "split-reverse" && entry.Intent.Args["reason"] == "Keep the responsibility together." {
						retained = true
					}
				}
				if !retained {
					t.Fatal("requested reversal was lost")
				}
				return
			}
			if scenario == "lost confirmation" {
				transport := &splitConfirmRepository{Repository: bed.repo, loseConfirm: true}
				public := func(args ...string) (int, string, string) {
					return publicWith(args, func(owners *intentOwners) {
						owners.dependencies.endpoint = func(root string) (goal.Endpoint, error) {
							endpoint, err := bed.endpoint(root)
							endpoint.Repository = transport
							return endpoint, err
						}
					})
				}
				code, out, errOut := public(args...)
				if code == 0 || !strings.Contains(out+errOut, "confirm") {
					t.Fatalf("lost confirmation: %d %s%s", code, out, errOut)
				}
				gcliLedgerMust(t, bed, "goal", "sync", "--recover")
			} else {
				code, out, errOut := publicWith(args, func(owners *intentOwners) {
					publish := owners.dependencies.endpoint
					owners.dependencies.endpoint = func(root string) (goal.Endpoint, error) {
						endpoint, err := publish(root)
						transport := &splitConfirmRepository{Repository: bed.repo}
						transport.beforePublish = func() {
							for _, id := range []string{"child-one", "child-two"} {
								lockPath, err := goalrevision.Path(root, id, read(id).Revision)
								if err != nil {
									t.Fatal(err)
								}
								if _, err := os.Stat(lockPath); err != nil {
									t.Fatalf("reversal missing reservation lock for %s: %v", id, err)
								}
							}
						}
						endpoint.Repository = transport
						return endpoint, err
					}
				})
				if code != 0 {
					t.Fatalf("reverse: %d %s%s", code, out, errOut)
				}
			}
			after := read("source")
			shown := gcliLedgerMust(t, bed, "goal", "show", "source")
			wantReversal := "(reversed " + bed.clock().In(time.Local).Format("15:04") + " by Wido)"
			if !strings.Contains(shown, "Split into goals") || !strings.Contains(shown, "child-one, child-two "+wantReversal) {
				t.Fatalf("show omitted reversal date or person: want %q in %s", wantReversal, shown)
			}
			if scenario == "claimed parent" {
				spendAfter := dispatchcore.ProjectConsumption(bed.root, after, bed.clock(), home)
				if spendBefore.Status != dispatchcore.BudgetKnown || spendBefore.Attempts != 1 || spendBefore.ReservedJobMinutes == 0 || spendAfter.Attempts != spendBefore.Attempts || spendAfter.ReservedJobMinutes != spendBefore.ReservedJobMinutes {
					t.Fatalf("spending lost: before=%+v after=%+v", spendBefore, spendAfter)
				}
			}
			wantState := before.State
			if wantState == goal.StateClaimed {
				wantState = goal.StateApproved
			}
			if after.State != wantState || after.Claimed != nil || !reflect.DeepEqual(after.Parked, before.Parked) || !reflect.DeepEqual(after.Split, split.Split) || !reflect.DeepEqual(after.Budget, before.Budget) || !reflect.DeepEqual(after.Approved, before.Approved) || !reflect.DeepEqual(after.Obligation, before.Obligation) || !reflect.DeepEqual(after.ReviewObligations, before.ReviewObligations) || after.Intent != before.Intent || after.NextStep != before.NextStep || !reflect.DeepEqual(after.Episode, split.Episode) {
				t.Fatalf("source lost responsibility: before=%+v after=%+v", before, after)
			}
			for _, id := range split.Split.Children {
				child := read(id)
				if child.State != goal.StateParked || child.Parked == nil || child.Parked.Because != "split reversed: Keep the responsibility together." || child.Approved != nil || child.Budget != nil || child.SplitFrom != "source" || len(child.History) < 2 {
					t.Fatalf("child not retired with lineage: %+v", child)
				}
			}
			tip := bed.tip()
			out := gcliLedgerMust(t, bed, args...)
			if tip != bed.tip() || !strings.Contains(out, "already reversed") {
				t.Fatalf("repeat rewrote effect: %s", out)
			}
			if err := os.WriteFile(path, []byte(plan), 0600); err != nil {
				t.Fatal(err)
			}
			gcliBudgetRefused(t, bed, "rename the members", "goal", "split", "source", "--plan", path, "--by", "Wido")
			gcliBudgetRefused(t, bed, "mutually exclusive", append(args, "--plan", path)...)
			if scenario == "approved parent" {
				renamed := strings.ReplaceAll(strings.ReplaceAll(plan, "child-one", "child-three"), "child-two", "child-four")
				if err := os.WriteFile(path, []byte(renamed), 0600); err != nil {
					t.Fatal(err)
				}
				gcliLedgerMust(t, bed, "goal", "split", "source", "--plan", path, "--by", "Wido")
				if shown := gcliLedgerMust(t, bed, "goal", "show", "source"); strings.Contains(shown, "(reversed ") || !strings.Contains(shown, "child-four, child-three") {
					t.Fatalf("new split hidden by its prior reversal: %s", shown)
				}
			}
			t.Logf("goal split source --reverse: parent %s; children parked without approval; lineage retained; repeat holds", wantState)
		})
	}
}
