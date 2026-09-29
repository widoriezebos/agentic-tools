package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// The landing gate at every form of work land (g1-s70 D2).
//
// One function, goal.Gate, decides; every form calls it with the resolved goal
// and the candidate's branch tip before anything is proved or joined: work
// land G (the batch and the hand route), work land j2:J, the staged --message
// form when it names a goal, and the exceptional forms. The batch evaluates it
// again at every publication and retry against a fresh ledger
// (authorizeBatchMemberInProjection), the hand route again right before its
// push, and the landing path again right before every push of the staged and
// exceptional forms, each retry included (landingPathGate). --queue-only is
// not a landing and still enters Review.
//
// The candidate's tip is the goal branch at origin: the tip a review records
// and a land-without-sitting decision binds. A certified chain landed with
// work land j2:J has no branch tip, so at or above the tier no human word can
// be bound to it and it is refused; its work lands through the goal branch.

// landingGateSettings resolves the two settings of the installation at root.
func landingGateSettings(root string) (goal.GateSettings, error) {
	return goal.ResolveGateSettings(filepath.Join(root, "metasystem.conf"))
}

// productionIntentLandingGate evaluates the gate for one goal at tip against a
// freshly fetched ledger.
func productionIntentLandingGate(inv *intentInvocation, goalID, tip string) (string, error) {
	return freshLandingGate(inv.stateRoot, inv.layout.InstallationRoot, goalID, tip, inv.owners.dependencies, inv.owners.commandNow)
}

// freshLandingGate is goal.Gate for one goal at tip against a freshly fetched
// ledger and the installation's layered settings: the one evaluation a form's
// admission, the hand route's push and every push of the landing path make,
// as the batch's publication authority makes it over its own fresh projection.
func freshLandingGate(stateRoot, installationRoot, goalID, tip string, dependencies syncRequestDependencies, commandNow func(string) (time.Time, error)) (string, error) {
	endpoint, err := dependencies.endpoint(stateRoot)
	if err != nil {
		return "", err
	}
	now, err := commandNow(stateRoot)
	if err != nil {
		return "", err
	}
	projection, err := goal.Project(endpoint, true, now)
	if err != nil {
		return "", err
	}
	settings, err := landingGateSettings(installationRoot)
	if err != nil {
		return "", err
	}
	return goal.Gate(projection.Tree.Live[goalID], tip, settings)
}

// landingPathGate is the landing path's gate immediately before each push of
// a landing in a goal's name (g1-s70 D2): the staged --message form and the
// exceptional forms met the gate at admission, and a hold or a changed word
// that arrives while they prove, rebase or retry stops the push here.
func landingPathGate(root, goalID string) error {
	stateRoot, err := stateroot.NewResolver(stateroot.RepositoryTop, os.Executable).RootForInstallation(root)
	if err != nil {
		return err
	}
	return landingPathGateAt(stateRoot, root, goalID, defaultSyncRequestDependencies(), goalCommandNow, productionIntentBranchTip)
}

// landingPathGateAt reads the gate at the goal branch's tip at origin, the tip
// the admission read. An installation with no synced ledger carries none of
// the gate's facts and reads none, as its admission does.
func landingPathGateAt(stateRoot, root, goalID string, dependencies syncRequestDependencies, commandNow func(string) (time.Time, error), branchTip func(root, goalID string) (string, error)) error {
	if !converted(stateRoot) {
		return nil
	}
	tip, err := branchTip(root, goalID)
	if err != nil {
		return fmt.Errorf("the landing gate cannot read goal/%s at origin: %w", goalID, err)
	}
	_, err = freshLandingGate(stateRoot, root, goalID, tip, dependencies, commandNow)
	return err
}

// productionIntentBranchTip is the goal branch's tip at origin, or "" where
// origin has no branch for the goal.
func productionIntentBranchTip(root, goalID string) (string, error) {
	endpoint, err := goalBranchEndpoint(root)
	if err != nil {
		return "", err
	}
	tip, present, err := goalBranchOriginTip(root, endpoint, goalID)
	if err != nil || !present {
		return "", err
	}
	return tip, nil
}

// admitLanding is the gate at a form's admission: nil to proceed, or the
// refusal the person reads, naming the human verb that carries past it.
func (inv *intentInvocation) admitLanding(targets []intentTarget, goalID, tip string) *intentResult {
	gate := inv.delivery().landingGate
	if gate == nil {
		gate = productionIntentLandingGate
	}
	if _, err := gate(inv, goalID, tip); err != nil {
		result := landingGateRefusal(targets, goalID, err)
		return &result
	}
	return nil
}

func landingGateRefusal(targets []intentTarget, goalID string, err error) intentResult {
	var refusal *goal.GateRefusal
	if !errors.As(err, &refusal) {
		return intentResult{Targets: targets, Outcome: intentFailed, code: 1, Summary: "the landing gate cannot be read: " + err.Error() + "; nothing was landed"}
	}
	result := intentResult{Targets: targets, Outcome: intentRefused, code: 1, Summary: refusal.Reason + "; nothing was landed",
		Data: map[string]any{"code": refusal.Code}}
	if refusal.Code == goal.GateWaitsForHuman {
		result.Decision = "metasystem goal land-without-sitting " + goalID + " --reason TEXT"
	}
	return result
}

// intentBranchTip reads the goal branch's tip for a form that does not read
// the branch itself.
func (inv *intentInvocation) intentBranchTip(goalID string) string {
	read := inv.delivery().branchTip
	if read == nil {
		read = productionIntentBranchTip
	}
	tip, err := read(inv.layout.InstallationRoot, goalID)
	if err != nil {
		return ""
	}
	return tip
}

// landed reports that a land result is a confirmed publication of the goal's
// work: pushed by hand, or recorded landed by the batch owner.
func landed(result intentResult) bool {
	data, ok := result.Data.(map[string]any)
	if !ok {
		return false
	}
	if record, ok := data["landing"].(intentLanded); ok && record.Landing != "" {
		return true
	}
	return data["unitState"] == batch.UnitLanded
}

// noteLanded writes the holder's landed line once a landing's publication is
// confirmed, naming what it landed under. A seat that does not hold the goal
// (a person landing by hand) records nothing; the landing stands either way.
func (inv *intentInvocation) noteLanded(goalID string, result intentResult) intentResult {
	if !landed(result) {
		return result
	}
	record := inv.delivery().recordLanded
	if record == nil {
		record = productionRecordLanded
	}
	if err := record(inv, goalID); err != nil {
		if data, ok := result.Data.(map[string]any); ok {
			data["landedLine"] = err.Error()
		}
	}
	return result
}

func productionRecordLanded(inv *intentInvocation, goalID string) error {
	projection, _, problem := inv.projection()
	if problem != nil {
		return fmt.Errorf("%s", problem.Summary)
	}
	file := projection.Tree.Live[goalID]
	if file == nil {
		return fmt.Errorf("goal %s is not live", goalID)
	}
	settings, err := landingGateSettings(inv.layout.InstallationRoot)
	if err != nil {
		return err
	}
	request, err := syncReqWithProofAtWithDependencies("landed", inv.stateRoot, "", "", nil, inv.owners.commandNow, inv.owners.dependencies)
	if err != nil {
		return err
	}
	result, err := goal.RecordLanded(request, goalID, goal.LandedUnder(file, settings))
	if err == nil && result.Outcome != goal.OutcomeConfirmed && !result.Unchanged {
		err = fmt.Errorf("%s", result.Detail)
	}
	return err
}
