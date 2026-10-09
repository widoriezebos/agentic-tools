package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/landpath"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
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
// exceptional forms, each retry included (pushGate). --queue-only is not a
// landing and still enters Review.
//
// The candidate's tip is the goal branch at origin: the tip a review records
// and a land-without-sitting decision binds. A certified chain landed with
// work land j2:J publishes the head of its candidate branch, so that commit is
// its tip (chainHead): a word recorded at it lets the chain land, a word at any
// other commit refuses naming both.

// landingGateSettings reads the settings of the installation serving root.
func landingGateSettings(root string, serving func(string) (string, string)) (goal.GateSettings, error) {
	installation, _ := serving(root)
	return goal.ResolveGateSettings(filepath.Join(installation, "metasystem.conf"))
}

// productionIntentLandingGate evaluates the gate for one goal at tip against a
// freshly fetched ledger.
func productionIntentLandingGate(inv *intentInvocation, goalID, tip string) (string, error) {
	return freshLandingGate(inv.stateRoot, inv.layout.InstallationRoot.Path(), goalID, tip, inv.owners.dependencies, inv.owners.commandNow)
}

// freshLandingGate is goal.Gate for one goal at tip against a freshly fetched
// ledger and the installation's layered settings: the one evaluation a form's
// admission, the hand route's push and every push of the landing path make,
// as the batch's publication authority makes it over its own fresh projection.
func freshLandingGate(stateRoot, installationRoot, goalID, tip string, dependencies syncRequestDependencies, commandNow func(string) (time.Time, error)) (string, error) {
	endpoint, err := dependencies.endpoint(installationRoot)
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
	settings, err := landingGateSettings(installationRoot, delegationToolInstallation)
	if err != nil {
		return "", err
	}
	return goal.Gate(projection.Tree.Live[goalID], tip, settings)
}

// pushGate is the gate the landing path reads immediately before each push of
// a staged or exceptional form this invocation admitted (g1-s70 D2): the same
// gate the admission read, at the goal branch's tip at origin, against a
// freshly fetched ledger, so a hold or a changed word that arrives while the
// landing proves, rebases or retries stops the push.
func (inv *intentInvocation) pushGate() func(root, goalID string) error {
	return func(_, goalID string) error {
		gate := inv.delivery().landingGate
		if gate == nil {
			gate = productionIntentLandingGate
		}
		_, err := gate(inv, goalID, inv.intentBranchTip(goalID))
		if err == nil {
			return nil
		}
		// The landing path tells the person the gate's two lines.
		result := landingGateRefusal(nil, goalID, err)
		return &landpath.GateRefusal{Reason: strings.TrimSuffix(result.Summary, ", so nothing was landed"), Run: result.next,
			Then: result.nextReason, Details: result.Details, Cause: err.Error()}
	}
}

// productionIntentBranchTip is the goal branch's tip at origin, or "" where
// origin has no branch for the goal.
func productionIntentBranchTip(root, goalID string) (string, error) {
	endpoint, err := branch.MainEndpoint(root)
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

// landingGateRefusal is the gate's refusal in the two lines a person reads:
// why the goal may not land now, and the one command that lets it; the
// gate's code and its full reason are details.
func landingGateRefusal(targets []intentTarget, goalID string, err error) intentResult {
	var refusal *goal.GateRefusal
	if !errors.As(err, &refusal) {
		return intentResult{Targets: targets, Outcome: intentFailed, code: 1,
			Summary: "whether goal " + goalID + " may land can't be read now, so nothing was landed",
			next:    []string{"metasystem", "goal", "sync"}, nextReason: "then repeat this command",
			Details: []string{"the landing gate could not be read: " + err.Error()}}
	}
	result := intentResult{Targets: targets, Outcome: intentRefused, code: 1, Data: map[string]any{"code": refusal.Code},
		Details: []string{"refused because: " + refusal.Error()}}
	command := ""
	if at := strings.LastIndex(refusal.Reason, "metasystem goal "); at >= 0 {
		command = strings.TrimSpace(refusal.Reason[at:])
	}
	switch refusal.Code {
	case goal.GateHeldBySitting:
		held, _, _ := strings.Cut(refusal.Reason, ";")
		result.Summary = held + ", so nothing was landed"
		result.nextReason = "once the review is over; then repeat this command"
	case goal.GateWaitsForHuman:
		why := refusal.Reason
		if _, after, found := strings.Cut(why, "waits for a person: "); found {
			why, _, _ = strings.Cut(after, "; a person reviews")
		}
		result.Summary = "goal " + goalID + " waits for a person's word before it lands: " + why
		result.nextReason = "a person lets it land; or review it with metasystem goal review " + goalID
		if command == "" {
			command = "metasystem goal land-without-sitting " + goalID + " --reason TEXT"
		}
	default:
		result.Summary = refusal.Reason + ", so nothing was landed"
	}
	if command != "" {
		result.next = shellWords(command)
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
	tip, err := read(inv.layout.InstallationRoot.Path(), goalID)
	if err != nil {
		return ""
	}
	return tip
}

// landed reports that a land result is a confirmed publication of the goal's
// work: pushed by hand.
func landed(result intentResult) bool {
	data, ok := result.Data.(map[string]any)
	if !ok {
		return false
	}
	record, ok := data["landing"].(intentLanded)
	return ok && record.Landing != ""
}

// noteLanded writes the holder's landed line once a landing's publication is
// confirmed, naming what it landed under. A seat that does not hold the goal
// (a person landing by hand) records nothing; the landing stands either way.
func (inv *intentInvocation) noteLanded(goalID string, result intentResult) intentResult {
	data, _ := result.Data.(map[string]any)
	if entry, ok := data["queue"].(plain.Entry); ok && entry.State == plain.StateLanded && result.Outcome == intentUnchanged {
		inv.landingWords = entry.Landing.Words()
	}
	if !landed(result) && inv.landingWords == "" {
		return result
	}
	// A landing on main is the one piece of news the channel carries
	// (Decision 7), once per sha; a failed post is kept for a retry.
	if landing, ok := data["landing"].(intentLanded); ok && landing.Landing != "" {
		// The message is the plain sentence of what it delivered: this
		// command's --delivered, else the one recorded with the landing.
		text := strings.TrimSpace(inv.input.text("delivered"))
		if text == "" {
			text = landing.Delivered
		}
		if err := postLanded(inv.layout.InstallationRoot.Path(), text, landing.Landing, inv.delivery().now()); err != nil {
			data["landedNotice"] = err.Error()
		}
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
	settings, err := landingGateSettings(inv.layout.InstallationRoot.Path(), delegationToolInstallation)
	if err != nil {
		return err
	}
	request, err := syncReqWithProofAtWithDependencies("landed", inv.stateRoot, "", "", nil, inv.owners.commandNow, inv.owners.dependencies)
	if err != nil {
		return err
	}
	under := goal.LandedUnder(file, settings)
	if inv.landingWords != "" {
		under += "; " + inv.landingWords
	}
	result, err := goal.RecordLanded(request, goalID, under)
	if err == nil && result.Outcome != goal.OutcomeConfirmed && !result.Unchanged {
		err = fmt.Errorf("%s", result.Detail)
	}
	return err
}
