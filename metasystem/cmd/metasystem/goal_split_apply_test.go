package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalrevision"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/retrodebt"
)

// A failed confirming fetch is a transport failure after a real publication.
type splitConfirmRepository struct {
	goal.Repository
	loseConfirm   bool
	losePublish   bool
	published     bool
	beforePublish func()
}

func (r *splitConfirmRepository) Publish(old, next string) (goal.CASOutcome, error) {
	if r.beforePublish != nil {
		r.beforePublish()
	}
	if r.losePublish {
		r.losePublish = false
		return goal.CASUnknown, errors.New("fixture publication unavailable")
	}
	outcome, err := r.Repository.Publish(old, next)
	if outcome == goal.CASLanded {
		r.published = true
	}
	return outcome, err
}

func (r *splitConfirmRepository) Capture(opid string) (string, error) {
	if r.loseConfirm && r.published {
		r.loseConfirm = false
		return "", errors.New("fixture confirming fetch unavailable")
	}
	return r.Repository.Capture(opid)
}

func TestGoalSplitAppliesExistingMarkdownPlan(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"apply", "lost confirmation", "unlanded split", "active work", "unclaimed active work", "unknown work", "borrowed name", "cycle", "collision"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			bed := newGoalCLIBed(t, goalCLISeed{allowTerminalProof: true})
			bed.announceHolder()
			gcliLedgerMust(t, bed, "goal", "release", "ship-widget", "--reason", "Free the seat.")
			gcliBudgetOpen(t, bed, "source", gcliBudgetTierTwo, "Split the responsibility.")
			gcliBudgetHumanMust(t, bed, "goal", "approve", "source", "--budget", "norm")
			gcliLedgerMust(t, bed, "goal", "group", "source", "prior-group")
			gcliLedgerMust(t, bed, "goal", "claim", "source")
			gcliLedgerMust(t, bed, "goal", "edit", "source", "--label", "after-claim")
			gcliBudgetOpen(t, bed, "dependent", gcliBudgetTierOne, "Wait for the source.", "--blocked-by", "source")
			source, problems := goal.ParseFile([]byte(bed.goalRecord("source")))
			if len(problems) != 0 {
				t.Fatal(problems)
			}
			source.Blocked = []string{"port-engine"}
			source.ReviewObligations = []goal.ReviewObligation{{Finding: "f1", Chain: "r1", Artifact: "reader.go", Test: "TestReader", State: "open"}}
			if scenario == "apply" || scenario == "lost confirmation" {
				source.Obligation = &goal.GovernedObligation{
					Revision: source.Revision, BudgetRevision: source.Claimed.Revision, State: goal.ObligationDraft, Owner: "Wido",
					Effects: []goal.GoverningEffect{goal.EffectAuthorizeSpend},
					Assumptions: goal.ObligationAssumptions{Recurrence: goal.SingleExperiment, Platform: "fixture", ToolchainIdentity: "fixture",
						SurfaceDigest: strings.Repeat("a", 64), MaxActiveJobs: 1, TimingEnvelopeSeconds: 1800, ObservationSource: "run-terminal-record"},
					Triggers: goal.HumanReviewTriggers{ValueJudgment: "yes", Reversibility: "compensable", SevereHarm: "unknown", UnfamiliarApproach: "no",
						TestDiscrimination: "strong", CorrelatedAssumptionRisk: "yes", AuthorityScopeChange: "no", DestructiveReach: "none"},
				}
			}
			gcliLedgerInstall(t, bed, "fixture-obligation", goal.Change{Path: "plans/goals/source.md", Content: goal.RenderFile(source)})
			jobs := filepath.Join(bed.root, "artifacts", "agents", "jobs")
			if err := os.MkdirAll(jobs, 0o755); err != nil {
				t.Fatal(err)
			}
			record := map[string]any{"jobId": "design-read", "operationId": "design-read", "goalId": "source", "goalRevision": source.Claimed.Revision,
				"capMin": 5, "status": "completed", "role": "design-critic", "reviewChainCounted": true, "parentJob": nil,
				"startedAt": bed.clock().Add(-time.Minute).Format(time.RFC3339), "endedAt": bed.clock().Format(time.RFC3339),
				"pid": 123, "pidStartedAt": 1}
			if scenario == "active work" || scenario == "unclaimed active work" {
				record["status"] = "running"
				delete(record, "endedAt")
			}
			if scenario == "unknown work" {
				record["goalRevision"] = "unreadable"
			}
			data, err := json.Marshal(record)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(jobs, "design-read.json"), data, 0o644); err != nil {
				t.Fatal(err)
			}
			if scenario == "unclaimed active work" {
				gcliLedgerMust(t, bed, "goal", "release", "source", "--reason", "Release while design work is running.")
			}
			beforeSpend := dispatchcore.ProjectConsumption(bed.root, source, bed.clock())
			plan := "# split source\n\n## member child-one\n- Intent: Build the reader.\n- Next step: Write its brief.\n- BlockedBy: fix-docs\n\n## member child-two\n- Intent: Build the writer.\n- Next step: Write its brief.\n- BlockedBy: child-one\n"
			if scenario == "cycle" {
				plan = strings.Replace(plan, "- BlockedBy: fix-docs\n", "- BlockedBy: child-two\n", 1)
			}
			if scenario == "collision" {
				plan = strings.Replace(plan, "child-one", "fix-docs", 1)
			}
			path := filepath.Join(bed.root, "members.md")
			if err := os.WriteFile(path, []byte(plan), 0o644); err != nil {
				t.Fatal(err)
			}
			args := []string{"goal", "split", "source", "--plan", path, "--by", "Wido"}
			if scenario == "borrowed name" {
				bed.prove = newGcliAuthorityCaller(t, bed, gcliAuthorityAgent).prove
				gcliLedgerRefused(t, bed, "enroll", args...)
				return
			}
			if scenario == "active work" || scenario == "unclaimed active work" || scenario == "unknown work" || scenario == "cycle" || scenario == "collision" {
				before := bed.tip()
				code, out, errOut := bed.public(args...)
				if code == 0 || bed.tip() != before {
					t.Fatalf("%s published: code=%d out=%s err=%s", scenario, code, out, errOut)
				}
				for _, id := range []string{"child-one", "child-two"} {
					if bed.accepted("plans/goals/"+id+".md") != "" {
						t.Fatalf("refused split opened %s", id)
					}
				}
				t.Logf("%s refused: %s%s", scenario, out, errOut)
				return
			}
			transport := &splitConfirmRepository{Repository: bed.repo, loseConfirm: scenario == "lost confirmation", losePublish: scenario == "unlanded split"}
			transport.beforePublish = func() {
				path, err := goalrevision.Path(bed.root, source.Id, source.Claimed.Revision)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := os.Stat(path); err != nil {
					t.Fatalf("split did not hold the claim's reservation lock at publication: %v", err)
				}
			}
			public := func(argv ...string) (int, string, string) {
				command, rest, ok := resolveIntentArgv(argv)
				if !ok {
					t.Fatal(argv)
				}
				var out, errOut bytes.Buffer
				owners := bed.owners(&out, &errOut)
				owners.dependencies.endpoint = func(root string) (goal.Endpoint, error) {
					ep, err := bed.endpoint(root)
					ep.Repository = transport
					return ep, err
				}
				code := runIntentIn(command, rest, &out, &errOut, bed.root, owners)
				return code, out.String(), errOut.String()
			}
			oldTip := bed.tip()
			code, out, errOut := public(args...)
			if scenario == "unlanded split" {
				if code == 0 || bed.tip() != oldTip {
					t.Fatalf("unlanded split claimed success: %d %s%s", code, out, errOut)
				}
				entries, err := goal.Entries(bed.root)
				if err != nil {
					t.Fatal(err)
				}
				for _, entry := range entries {
					if entry.Intent.Verb != "split" || entry.Phase != goal.PhasePushed {
						continue
					}
					entry.Owner.Pid, entry.Owner.PidStartedAt, entry.Owner.StartTicks, entry.Owner.BootID = 99999999, 1, 0, ""
					data, err := json.Marshal(entry)
					if err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(bed.root, "artifacts", "agents", "goal-transactions", entry.Opid+".json"), data, 0o644); err != nil {
						t.Fatal(err)
					}
				}
				code, out, errOut = public("goal", "sync", "--recover")
				if code != 0 || bed.tip() != oldTip {
					t.Fatalf("recovery replayed person authority: %d %s%s", code, out, errOut)
				}
				entries, err = goal.Entries(bed.root)
				if err != nil {
					t.Fatal(err)
				}
				refusedReplay := false
				for _, entry := range entries {
					if entry.Intent.Verb == "split" && entry.Outcome == goal.OutcomeRejected && strings.Contains(entry.Evidence, "person") {
						refusedReplay = true
					}
				}
				if !refusedReplay || bed.accepted("plans/goals/child-one.md") != "" {
					t.Fatal("unlanded person split was replayed instead of requiring the person's rerun")
				}
				code, out, errOut = public(args...)
			}
			if scenario == "lost confirmation" {
				if code == 0 || !strings.Contains(out+errOut, "confirming") || bed.tip() != oldTip {
					t.Fatalf("lost confirmation claimed success: %d %s%s", code, out, errOut)
				}
				entries, err := goal.Entries(bed.root)
				if err != nil {
					t.Fatal(err)
				}
				pushed := ""
				for _, entry := range entries {
					if entry.Intent.Verb == "split" && entry.Phase == goal.PhasePushed {
						pushed = entry.Opid
					}
				}
				if pushed == "" {
					t.Fatal("lost confirmation did not stay pushed")
				}
				code, out, errOut = public("goal", "sync", "--recover")
				if code != 0 {
					t.Fatalf("recover: %d %s%s", code, out, errOut)
				}
				entry, err := goal.ReadEntry(bed.root, pushed)
				if err != nil || entry.Outcome != goal.OutcomeConfirmed {
					t.Fatalf("not confirmed: %+v %v", entry, err)
				}
				t.Logf("goal sync --recover: %s%s", out, errOut)
			} else if code != 0 {
				t.Fatalf("split: %d %s%s", code, out, errOut)
			}
			var shown struct {
				Data struct {
					Where string
					Goal  *goal.GoalFile
				}
			}
			if err := json.Unmarshal([]byte(gcliLedgerMust(t, bed, "goal", "show", "source", "--json")), &shown); err != nil {
				t.Fatal(err)
			}
			parent := shown.Data.Goal
			if shown.Data.Where != "live" || parent == nil || parent.State != goal.StateSplit || parent.Conclude != "" || parent.Claimed != nil || parent.Split == nil || parent.Split.PriorState != goal.StateClaimed || parent.Ratified == nil {
				t.Fatalf("parent was concluded or lost lineage: %+v", shown)
			}
			if parent.Intent != source.Intent || parent.NextStep != source.NextStep || !reflect.DeepEqual(parent.Blocked, source.Blocked) || parent.Arc != source.Arc ||
				!reflect.DeepEqual(parent.Budget, source.Budget) || !reflect.DeepEqual(parent.Approved, source.Approved) || !reflect.DeepEqual(parent.ReviewObligations, source.ReviewObligations) || !reflect.DeepEqual(parent.Obligation, source.Obligation) || parent.Episode == nil {
				t.Fatalf("source responsibility or allowance lost: %+v", parent)
			}
			afterSpend := dispatchcore.ProjectConsumption(bed.root, parent, bed.clock())
			if beforeSpend.Status != dispatchcore.BudgetKnown || beforeSpend.Attempts != 1 || beforeSpend.ReservedJobMinutes == 0 || afterSpend.Attempts != beforeSpend.Attempts || afterSpend.ReservedJobMinutes != beforeSpend.ReservedJobMinutes {
				t.Fatalf("spending lost: before=%+v after=%+v", beforeSpend, afterSpend)
			}
			if bed.accepted("records/goals/source.md") != "" || strings.Contains(bed.accepted("plans/goals/backlog.md"), "- source opid=") {
				t.Fatal("source archived or retired")
			}
			if !reflect.DeepEqual(parent.Split.Children, []string{"child-one", "child-two"}) {
				t.Fatal(parent.Split)
			}
			for _, id := range parent.Split.Children {
				child, problems := goal.ParseFile([]byte(bed.goalRecord(id)))
				if len(problems) != 0 {
					t.Fatal(problems)
				}
				if child.State != goal.StateQueued || child.Approved != nil || child.Budget != nil || child.Episode != nil || child.SplitFrom != "source" || child.Arc != "" || !containsTestString(child.Blocked, "source") || !containsTestString(child.Blocked, "port-engine") {
					t.Fatalf("child not held and independent: %+v", child)
				}
			}
			dependent, problems := goal.ParseFile([]byte(bed.goalRecord("dependent")))
			if len(problems) != 0 || !reflect.DeepEqual(dependent.Blocked, []string{"source"}) {
				t.Fatalf("dependent rewritten: %+v %v", dependent, problems)
			}
			debts, err := retrodebt.Open(bed.root)
			if err != nil || len(debts) != 0 {
				t.Fatalf("live source raised old-arc debt: %+v %v", debts, err)
			}
			tip := bed.tip()
			code, out, errOut = public(args...)
			if code != 0 || bed.tip() != tip || !strings.Contains(out, "already split into") {
				t.Fatalf("repeat: %d %s%s", code, out, errOut)
			}
			if err := os.WriteFile(path, []byte(strings.Replace(plan, "Build the reader.", "A different reader.", 1)), 0o644); err != nil {
				t.Fatal(err)
			}
			gcliLedgerRefused(t, bed, "reverse", args...)
			gcliBudgetHumanMust(t, bed, "goal", "edit", "child-one", "--risk", gcliBudgetTierOne, "--basis", "child risk")
			gcliBudgetHumanMust(t, bed, "goal", "approve", "child-one", "--budget", "norm")
			if ready := gcliLedgerMust(t, bed, "goal", "list", "--ready"); strings.Contains(ready, "next ready goal: child-one") {
				t.Fatalf("approval released source hold: %s", ready)
			}
			gcliBudgetHumanMust(t, bed, "goal", "unblock", "child-one", "--on", "source")
			gcliLedgerRefused(t, bed, "blocked", "goal", "claim", "child-one")
			gcliBudgetHumanMust(t, bed, "goal", "unblock", "child-one", "--on", "fix-docs")
			gcliLedgerMust(t, bed, "goal", "claim", "child-one")
			t.Log("goal show: parent live and split; two children held and unapproved; spending preserved; rerun held; explicit approval/unblock allowed child claim")
		})
	}
}

func containsTestString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
