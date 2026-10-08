package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/validate"
)

// reviewStoppedUnit applies the collected decision before a correction brief
// or another examination can be composed. Transfers publish completion debt
// before closing the source examination.
func (inv *intentInvocation) reviewStoppedUnit(targets []intentTarget, root, reviewRoot, returnPath string, work *reviewWorkContext) *intentResult {
	record, err := inv.unitRunner().Status(work.run)
	if err != nil || len(record.Rounds) == 0 {
		return &intentResult{Outcome: intentFailed, code: 1, Summary: "the unit's review decision cannot be read"}
	}
	round := record.Rounds[len(record.Rounds)-1]
	stop := round.Stop
	if stop != nil && (stop.Handoff == "stopped unreadable-policy" || stop.Handoff == "stopped unreadable-inherited-findings") && work.subject != nil {
		if err := work.retain(*work.subject); err != nil {
			return &intentResult{Outcome: intentFailed, code: 1, Summary: "the review decision could not be retained", Details: []string{err.Error()}, next: inv.workArgv(record, "review")}
		}
		record, err = inv.unitRunner().Status(work.run)
		if err != nil || len(record.Rounds) == 0 {
			return &intentResult{Outcome: intentFailed, code: 1, Summary: "the unit's review decision cannot be read"}
		}
		round = record.Rounds[len(record.Rounds)-1]
		stop = round.Stop
	}
	if stop == nil {
		return &intentResult{Outcome: intentRefused, code: 1, Summary: "the unit has no collected stop decision; nothing was started"}
	}
	data := map[string]any{"goal": record.Goal, "work": record.Unit, "run": record.ID, "stop": stop, "reads": round.Reads}
	again := inv.workArgv(record, "review")
	if stop.Handoff == "stopped unreadable-policy" {
		_, policyErr := inv.unitRunner().ReviewPolicy()
		repair, reason := inv.reviewPolicyRepair(policyErr)
		return &intentResult{Targets: targets, Outcome: intentRefused, code: 1, Data: data,
			Summary: "review.stop cannot be read; " + shellCommand(repair) + "; the unit stays stopped",
			next:    repair, nextReason: reason, Details: []string{record.PolicyError}}
	}
	if strings.HasPrefix(stop.Handoff, "stopped ") {
		if round.UnknownRetries == 0 && stop.Handoff != "stopped unreadable-policy" {
			// The committed critic retry owner permits one examination of the
			// same subject. Its executions do not add a unit round.
			data["retry"] = 1
			if work.retry > 0 {
				return &intentResult{Targets: targets, Outcome: intentRefused, code: 1, Data: data, Summary: "the fresh examination still has unknown inputs; the unit stays stopped", next: inv.workArgv(record, "revise", "--brief", "FILE", "--reason", "TEXT", "--by", "NAME")}
			}
			return &intentResult{Targets: targets, Outcome: intentInProgress, code: 1, Data: data, Summary: "the read's stop inputs are unknown; one fresh examination is available", next: append(again, "--retry", strconv.FormatInt(work.subject.ExaminationRound, 10)), nextReason: "examines this subject once without another build"}
		}
		return &intentResult{Targets: targets, Outcome: intentRefused, code: 1, Data: data, Summary: "the unit is stopped: " + stop.Handoff, next: inv.workArgv(record, "revise", "--brief", "FILE", "--reason", "TEXT", "--by", "NAME")}
	}
	if stop.Decision == "close" {
		if err := channel.RecordUnitStopAct(inv.layout.InstallationRoot.Path(), channel.UnitStopAct{ID: fmt.Sprintf("%s:close:%d", record.ID, stop.Attempt), Goal: record.Goal, Loop: stop.Loop, Subject: stop.Subject, Attempt: stop.Attempt, Kind: "work-review", Reason: "clean read", At: inv.unitStopNow(), UnitClosed: true}); err != nil {
			return &intentResult{Outcome: intentFailed, code: 1, Summary: "the unit closed, but its questions need reconciliation", Details: []string{err.Error()}, next: again, nextReason: "reconciles this clean read"}
		}
		return nil
	}
	if stop.Decision != "stop" {
		return nil
	}
	policy, policyErr := inv.unitRunner().ReviewPolicy()
	if policyErr != nil {
		repair, reason := inv.reviewPolicyRepair(policyErr)
		return &intentResult{Targets: targets, Outcome: intentRefused, code: 1, Summary: "review.stop cannot be read; " + shellCommand(repair) + "; nothing was transferred", next: repair, nextReason: reason, Details: []string{policyErr.Error()}}
	}
	pages, problem := inv.acceptedDesignPaths(record.Goal)
	if problem != nil {
		return problem
	}
	required, declared := false, false
	for _, page := range pages {
		units, err := launch.DeclaredUnits(page)
		if launch.UnsizedMissing(err) == "units-table" {
			continue
		}
		if err != nil {
			return &intentResult{Outcome: intentRefused, code: 1, Summary: "the accepted unit requirements cannot be read; the unit stays open", Details: []string{err.Error()}}
		}
		body, readErr := os.ReadFile(page)
		if readErr != nil {
			return &intentResult{Outcome: intentFailed, code: 1, Summary: readErr.Error()}
		}
		work.dropRequirements += page + "\x00" + string(body)
		declared = true
		for _, unit := range units {
			required = required || unit.Name == record.Unit
		}
	}
	projection, _, problem := inv.projectionWithFetch(true)
	if problem != nil {
		return problem
	}
	file, _ := goalRecord(projection, record.Goal)
	if file == nil {
		return &intentResult{Outcome: intentRefused, code: 1, Summary: "the goal is unavailable; the unit stays open"}
	}
	inherited := false
	for _, obligation := range file.ReviewObligations {
		if obligation.TargetUnit == record.Unit || obligation.SourceUnit == record.Unit {
			required = true
			inherited = true
		}
	}
	if !declared && !required {
		return &intentResult{Targets: targets, Outcome: intentRefused, code: 1, Data: data, Summary: "the goal has no accepted Units table; requiredness is unknown, so this unit stays open"}
	}
	work.dropRevision = file.Revision
	work.dropRequirements = launch.UnitResultDigest(work.dropRequirements)
	if required {
		work.dropRequirements = ""
	}
	destination := strings.TrimPrefix(stop.Handoff, "split ")
	generated := filepath.Join(round.Directory, "stop-dispositions.md")
	var findings []intentFinding
	originalEvidence := map[string]readsubject.Finding{}
	for _, read := range round.Reads {
		for _, f := range read.Findings {
			originalEvidence[f.ID] = f
			findings = append(findings, intentFinding{ID: f.ID, Title: f.Claim, Claim: f.Claim, Material: f.Material})
		}
	}
	binding := reviewBinding{Goal: record.Goal, Work: record.Unit, Attempt: round.Number, Subject: reviewSubjectIdentity(*work.subject), Examination: reviewRoot, Round: work.subject.ExaminationRound}
	binding.Return, _, err = reviewReturnDigest(returnPath)
	if err != nil {
		return &intentResult{Outcome: intentFailed, code: 1, Summary: err.Error()}
	}
	path := inv.flagPath("dispositions")
	if path != "" {
		decisions, violations := validate.Dispositions(path)
		if len(violations) > 0 {
			return &intentResult{Outcome: intentRefused, code: 1, Summary: "the stop dispositions cannot be joined", text: violations}
		}
		for _, decision := range decisions {
			if strings.HasPrefix(decision, "dropped:") {
				return inv.applyUnitDrop(targets, work)
			}
		}
		if required && !inherited {
			if policy == "person" {
				if _, _, problem := inv.actingAs("work review transfer", record.Goal, actorHuman); problem != nil {
					return problem
				}
			}
			if violations := validate.CritiqueClosed(returnPath, path); len(violations) > 0 {
				return &intentResult{Outcome: intentRefused, code: 1, Summary: "every finding needs a bound disposition", text: violations}
			}
			body, readErr := os.ReadFile(path)
			bound, bindErr := readReviewBinding(body)
			if readErr != nil || bindErr != nil || bound != binding {
				return &intentResult{Outcome: intentRefused, code: 1, Summary: "the dispositions answer another read; nothing was transferred"}
			}
			var obligations []goal.ReviewObligation
			for _, f := range findings {
				if !f.Material {
					continue
				}
				decision := decisions[f.ID]
				if !strings.HasPrefix(decision, "split: ") {
					return &intentResult{Outcome: intentRefused, code: 1, Summary: "the stopped unit's unresolved material must transfer through split: UNIT"}
				}
				target := strings.TrimSpace(strings.TrimPrefix(decision, "split: "))
				if target == "" || target == record.Unit || strings.ContainsAny(target, "/\\ \t\r\n") {
					return &intentResult{Outcome: intentRefused, code: 1, Summary: "a split needs a distinct unit name"}
				}
				obligations = append(obligations, goal.ReviewObligation{Finding: f.ID, Chain: reviewRoot, Artifact: record.Unit, Test: "destination code read covers inherited finding and source change", SourceUnit: record.Unit, TargetUnit: target, OriginalRead: strings.TrimSuffix(f.ID, ":"+strings.Split(f.ID, ":")[len(strings.Split(f.ID, ":"))-1]), OriginalFinding: f.ID, StopReference: stop.Evidence, TransferredOnce: true, SourceCommit: work.subject.Commit, OriginalEvidence: originalEvidence[f.ID]})
			}
			planBytes, planErr := os.ReadFile(record.Plan)
			var plan launch.UnitPlan
			if planErr == nil {
				planErr = json.Unmarshal(planBytes, &plan)
			}
			if planErr != nil {
				return &intentResult{Outcome: intentFailed, code: 1, Summary: "the retained source plan cannot be read", Details: []string{planErr.Error()}, next: again}
			}
			requirementPath := plan.Build.Brief
			if !filepath.IsAbs(requirementPath) {
				requirementPath = filepath.Join(record.PlanDirectory, requirementPath)
			}
			requirement, requirementErr := os.ReadFile(requirementPath)
			if requirementErr != nil {
				return &intentResult{Outcome: intentFailed, code: 1, Summary: "the source requirement cannot be read", Details: []string{requirementErr.Error()}, next: again}
			}
			var estimate int64
			for _, step := range record.Rounds[0].Steps {
				if strings.HasPrefix(step.Name, "build") {
					build, readErr := inv.unitRunner().Manager.Store.Read(step.LaunchID)
					if readErr != nil {
						return &intentResult{Outcome: intentFailed, code: 1, Summary: "the source size cannot be read", next: again}
					}
					estimate += build.DeclaredLines
				}
			}
			if estimate <= 0 {
				return &intentResult{Outcome: intentRefused, code: 1, Summary: "the source size is unknown; the unit stays open", next: again}
			}
			works, workErr := inv.unitRunner().NamedWork(record.Worktree, record.Goal)
			if workErr != nil {
				return &intentResult{Outcome: intentFailed, code: 1, Summary: "destination names cannot be checked", next: again, Details: []string{workErr.Error()}}
			}
			for _, o := range obligations {
				for _, named := range works {
					if named.Unit == o.TargetUnit {
						same := false
						for _, prior := range file.ReviewObligations {
							same = same || prior.SourceUnit == record.Unit && prior.TargetUnit == o.TargetUnit && prior.StopReference == stop.Evidence
						}
						if !same {
							return &intentResult{Outcome: intentRefused, code: 1, Summary: "the destination name belongs to other work; choose a distinct split unit", next: append(again, "--dispositions", path)}
						}
					}
				}
				brief := filepath.Join(round.Directory, o.TargetUnit+"-brief.md")
				reads, _ := json.MarshalIndent(round.Reads, "", "  ")
				history, _ := json.MarshalIndent(record.Rounds, "", "  ")
				dispositions, dispositionErr := os.ReadFile(path)
				if dispositionErr != nil {
					return &intentResult{Outcome: intentFailed, code: 1, Summary: "the stop decisions cannot be read", next: again}
				}
				text := goal.TransferBrief(record.Goal, o, estimate, requirement, reads, dispositions, history)
				if err := os.WriteFile(brief, []byte(text), 0o600); err != nil {
					return &intentResult{Outcome: intentFailed, code: 1, Summary: "the destination brief cannot be written", Details: []string{err.Error()}, next: again}
				}
			}
			result := inv.goalAct(record.Goal, "transfer findings", inv.syncOwner("defer-findings", []string{"--root", inv.stateRoot, "--id", record.Goal}, nil, false, func(req goal.VerbRequest, _ *syncFlags) (goal.PublishResult, error) {
				return goal.DeferFindings(req, record.Goal, obligations)
			}, "id"))
			if result.Outcome != intentConfirmed && result.Outcome != intentUnchanged {
				return &result
			}
			ids := []string{}
			destinations := []string{}
			for _, o := range obligations {
				ids = append(ids, o.Finding)
				if !slices.Contains(destinations, o.TargetUnit) {
					destinations = append(destinations, o.TargetUnit)
				}
			}
			if err := dispatchcore.CritiqueTransferClose(root, reviewRoot, ids, stop.Evidence); err != nil {
				return &intentResult{Outcome: intentFailed, code: 1, Summary: "the transfer is published, but source closure is pending: " + err.Error(), next: append(again, "--dispositions", path)}
			}
			work.subject.TransferredTo = destinations
			if err := work.retain(*work.subject); err != nil {
				return &intentResult{Outcome: intentFailed, code: 1, Summary: "the transfer is published, but the unit's closure is pending: " + err.Error()}
			}
			if err := channel.RecordUnitStopAct(inv.layout.InstallationRoot.Path(), channel.UnitStopAct{ID: record.ID + ":transfer", Goal: record.Goal, Loop: stop.Loop, Subject: stop.Subject, Attempt: stop.Attempt, Findings: ids, Kind: "work-review", Reason: "transferred to " + strings.Join(destinations, ", "), At: inv.unitStopNow(), UnitClosed: true}); err != nil {
				return &intentResult{Outcome: intentFailed, code: 1, Summary: err.Error()}
			}
			nextBuild := inv.publicArgv("work", "build", record.Goal, "--work", destinations[0], "--brief", filepath.Join(round.Directory, destinations[0]+"-brief.md"))
			for _, proof := range plan.Proof {
				nextBuild = append(nextBuild, "--check")
				nextBuild = append(nextBuild, proof.Argv...)
			}
			return &intentResult{Targets: targets, Outcome: intentConfirmed, Data: data, Summary: "the source read closed as transferred; its destinations remain required", next: nextBuild, nextReason: "builds the transferred work before completion"}
		}
	}
	proposal := decisionsDocument(binding, findings)
	for _, f := range findings {
		decision := "noted"
		if f.Material {
			if required && !inherited {
				decision = "split: " + destination
			} else {
				decision = "dropped: remove this optional unit"
			}
		}
		proposal = strings.Replace(proposal, "| "+f.ID+" | DECIDE |", "| "+f.ID+" | "+decision+" |", 1)
	}
	if _, err := os.Stat(generated); os.IsNotExist(err) {
		if err := os.WriteFile(generated, []byte(proposal), 0o600); err != nil {
			return &intentResult{Outcome: intentFailed, code: 1, Summary: err.Error()}
		}
	}
	needs := inv.workArgv(record, "revise", "--brief", "FILE", "--reason", "TEXT", "--by", "NAME")
	acts := []string{"work-revise", "goal-done"}
	if !required {
		needs = append(again, "--dispositions", generated)
		acts = append(acts, "work-drop")
	}
	if required && !inherited {
		needs = append(again, "--dispositions", generated)
		if policy == "person" {
			needs = append(needs, "--by", "NAME")
		}
		acts = []string{"work-review"}
	}
	machine := ""
	if file.Claimed != nil {
		machine = file.Claimed.Machine
	}
	for _, f := range findings {
		if !f.Material {
			continue
		}
		findingNeeds := slices.Clone(needs)
		findingActs := slices.Clone(acts)
		if _, err := dispatchcore.CritiqueRegisterDecisionFinding(inv.layout.InstallationRoot.Path(), reviewRoot, f.ID, record.Goal); !required && err == nil {
			findingActs = append(findingActs, "goal-accept-risk")
		}
		for i, value := range findingNeeds {
			if value == "FINDING" {
				findingNeeds[i] = f.ID
			}
		}
		_, err := channel.Ask(channel.AskRequest{RepoRoot: inv.layout.InstallationRoot.Path(), Goal: record.Goal, Kind: "other", Machine: machine, Facts: []string{stop.Class, f.ID + ": " + f.Title, "Prepared dispositions: " + generated}, Recommendation: "Run the requested act; the recorded result closes only its matching finding or unit.", UnitStop: &channel.UnitStopQuestion{Loop: stop.Loop, Subject: stop.Subject, Attempt: stop.Attempt, Finding: f.ID, Review: reviewRoot, Needs: shellCommand(findingNeeds), AcceptableActs: findingActs}, Now: inv.unitStopNow()})
		if err != nil {
			return &intentResult{Outcome: intentFailed, code: 1, Summary: "the stop is recorded, but its question is pending: " + err.Error()}
		}
	}
	if required && !inherited && !inv.input.has("dispositions") && policy != "person" {
		inv.input.values["dispositions"] = []string{generated}
		defer delete(inv.input.values, "dispositions")
		return inv.reviewStoppedUnit(targets, root, reviewRoot, returnPath, work)
	}
	data["template"] = generated
	return &intentResult{Targets: targets, Outcome: intentInProgress, code: 1, Data: data, Summary: "the unit stopped: " + stop.Class + "; each unresolved finding has its own requested act", next: needs, nextReason: "applies the recorded stop without another automatic correction"}
}
