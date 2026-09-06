package dispatch

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

const (
	independentCritiqueReferenceField = "independentCritiqueJobRef"
	liveProofReferenceField           = "liveProofEvidenceRef"
)

// StampClaimedReviewReference derives the reviewed chain from a newly
// published reservation and records the evidence pointer on that chain's
// root. The pointer is only an index: close-check remains the authority for
// completion, role, independence, effort, freshness, and terminal coverage.
func StampClaimedReviewReference(repoRoot, evidenceJob string) error {
	state := loadCritiqueState(repoRoot)
	evidence, present := state.records[evidenceJob]
	if !present {
		return fmt.Errorf("review evidence reservation %s is unreadable", evidenceJob)
	}
	if asString(evidence["dispatchMode"]) != string(DispatchModeFresh) {
		return nil
	}
	field, reviews, err := reviewReferenceBinding(evidenceJob, evidence)
	if err != nil {
		return err
	}
	if field == "" || reviews == "" {
		return nil
	}
	return stampReviewReference(repoRoot, state, evidenceJob, reviews, field, "")
}

// ReconcileReviewReference lawfully derives a pointer for evidence whose
// launch predates automatic stamping. Reconciliation accepts only completed
// evidence covering the reviewed chain's terminal work round; critic evidence
// must also have been folded into its canonical finding register.
func ReconcileReviewReference(repoRoot, rootJob, evidenceJob string) error {
	state := loadCritiqueState(repoRoot)
	evidence, present := state.records[evidenceJob]
	if !present {
		return fmt.Errorf("review evidence job %s is unreadable", evidenceJob)
	}
	if asString(evidence["status"]) != "completed" {
		return fmt.Errorf("review evidence job %s is not completed", evidenceJob)
	}
	field, reviews, err := reviewReferenceBinding(evidenceJob, evidence)
	if err != nil {
		return err
	}
	if field == "" {
		return fmt.Errorf("job %s is not a critic, warden, or verifier review", evidenceJob)
	}
	designCritic := asString(evidence["role"]) == "design-critic"
	criticReferenceJob := evidenceJob
	var criticRootRecord map[string]any
	if field == independentCritiqueReferenceField {
		if err := requireFoldedCritique(state, evidenceJob, evidence); err != nil {
			return err
		}
		if designCritic {
			var err error
			criticReferenceJob, criticRootRecord, err = requireClosedDesignCritique(state, evidenceJob)
			if err != nil {
				return err
			}
		}
	}
	members, err := chainMembers(filepath.Join(repoRoot, "artifacts", "agents", "jobs"), rootJob)
	if err != nil || len(members) == 0 {
		return fmt.Errorf("reviewed chain root %s is unreadable", rootJob)
	}
	for _, member := range members {
		if !TerminalStatus(asString(member.record["status"])) {
			return fmt.Errorf("reviewed chain %s is not terminal", rootJob)
		}
	}
	final, detail := finalHazardWorkState(members)
	if detail != "" {
		return fmt.Errorf("cannot derive review evidence for %s: %s", rootJob, detail)
	}
	if designCritic {
		if reviews != "" && reviews != final.job {
			return fmt.Errorf("design-critic root %s already reviews %s instead of terminal work round %s", criticReferenceJob, reviews, final.job)
		}
		reviews = final.job
		if err := requireDesignCritiquePairing(state, rootJob, criticReferenceJob, criticRootRecord, final); err != nil {
			return err
		}
		if err := requireDesignCritiqueTiming(criticReferenceJob, criticRootRecord, final); err != nil {
			return err
		}
	} else if reviews != final.job {
		return fmt.Errorf("review evidence job %s reviews %s instead of terminal work round %s", evidenceJob, reviews, final.job)
	}
	if err := stampReviewReference(repoRoot, state, criticReferenceJob, reviews, field, rootJob); err != nil {
		return err
	}
	if !designCritic {
		return nil
	}
	return stampDesignCriticReviews(repoRoot, criticReferenceJob, reviews)
}

func reviewReferenceBinding(evidenceJob string, evidence map[string]any) (field, reviews string, err error) {
	role := asString(evidence["role"])
	reviews = asString(evidence["reviews"])
	switch role {
	case "code-critic", "design-critic", "warden":
		field = independentCritiqueReferenceField
	case "verifier":
		if reviews == "" {
			return "", "", nil
		}
		field = liveProofReferenceField
	default:
		if reviews == "" {
			return "", "", nil
		}
		return "", "", fmt.Errorf("job %s role %s cannot carry a reviews binding", evidenceJob, role)
	}
	if role == "design-critic" && reviews == "" {
		return field, "", nil
	}
	if !validJobID.MatchString(reviews) {
		return "", "", fmt.Errorf("review evidence job %s has no valid reviews binding", evidenceJob)
	}
	return field, reviews, nil
}

func requireClosedDesignCritique(state critiqueState, evidenceJob string) (string, map[string]any, error) {
	criticRoot := state.chainRoot(evidenceJob)
	if criticRoot == "" {
		return "", nil, fmt.Errorf("design-critic evidence job %s has no valid chain root", evidenceJob)
	}
	root := state.records[criticRoot]
	register, present, err := critiqueFindingRegister(root)
	if err != nil {
		return "", nil, fmt.Errorf("design-critic root %s has a malformed finding register: %v", criticRoot, err)
	}
	if !present {
		return "", nil, fmt.Errorf("design-critic root %s has no canonical finding register", criticRoot)
	}
	foldedRound, err := findingRegisterRound(root, len(register))
	if err != nil {
		return "", nil, fmt.Errorf("design-critic root %s has malformed register round state: %v", criticRoot, err)
	}
	latest := state.latestMember(criticRoot)
	terminalRound, ok := numInt(latest["round"])
	if latest == nil || !ok || terminalRound < 1 {
		return "", nil, fmt.Errorf("design-critic root %s has no valid terminal round", criticRoot)
	}
	if foldedRound != terminalRound {
		return "", nil, fmt.Errorf("design-critic root %s has not been folded through terminal round %d; its register is folded through round %d", criticRoot, terminalRound, foldedRound)
	}
	if open := openRegisterFindingIDs(register); len(open) > 0 {
		return "", nil, fmt.Errorf("design-critic root %s has open or disputed finding identifiers: %s", criticRoot, strings.Join(open, ", "))
	}
	return criticRoot, root, nil
}

func requireDesignCritiquePairing(state critiqueState, reviewedRoot, criticRoot string, critic map[string]any, final hazardFinalWorkState) error {
	designValue, present := critic["design"]
	if !present {
		return nil
	}
	design, ok := designValue.(string)
	if !ok || design == "" {
		return fmt.Errorf("design-critic root %s carries a malformed design path", criticRoot)
	}
	resultPath := filepath.Join(state.agents, reviewedRoot, "rounds", fmt.Sprint(final.round), "return.json")
	result, err := readObject(resultPath)
	if err != nil {
		return fmt.Errorf("design-critic root %s names design %q but terminal work round %s has no readable diffBoundary", criticRoot, design, final.job)
	}
	boundary, boundaryOK := result["diffBoundary"].([]any)
	paired := boundaryOK && len(boundary) == 1
	if paired {
		path, pathOK := boundary[0].(string)
		paired = pathOK && path == design
	}
	if !paired {
		return fmt.Errorf("design-critic root %s names design %q but terminal work round %s has diffBoundary %v; expected exactly [%q]", criticRoot, design, final.job, result["diffBoundary"], design)
	}
	return nil
}

func requireDesignCritiqueTiming(criticRoot string, critic map[string]any, final hazardFinalWorkState) error {
	criticEnded := asString(critic["endedAt"])
	endedAt, err := parseRecordTime(criticEnded)
	if err != nil {
		return fmt.Errorf("design-critic root %s has no valid terminal end time %q", criticRoot, criticEnded)
	}
	if endedAt.Before(final.endedAt) {
		return fmt.Errorf("design-critic root %s ended at %s before final work round %s ended at %s; that final work round is unexamined and requires a later fresh design critique",
			criticRoot, criticEnded, final.job, final.endedAt.Format(time.RFC3339Nano))
	}
	return nil
}

func stampDesignCriticReviews(repoRoot, criticRoot, reviews string) error {
	return withRecordLock(repoRoot, criticRoot, func(recordPath string) error {
		root, err := readObject(recordPath)
		if err != nil || asString(root["jobId"]) != criticRoot || asString(root["role"]) != "design-critic" {
			return fmt.Errorf("design-critic root %s is unreadable", criticRoot)
		}
		if current := asString(root["reviews"]); current != "" && current != reviews {
			return fmt.Errorf("design-critic root %s already reviews %s instead of terminal work round %s", criticRoot, current, reviews)
		}
		root["reviews"] = reviews
		return writeRecord(recordPath, root)
	})
}

func requireFoldedCritique(state critiqueState, evidenceJob string, evidence map[string]any) error {
	criticRoot := state.chainRoot(evidenceJob)
	if criticRoot == "" {
		return fmt.Errorf("critic evidence job %s has no valid chain root", evidenceJob)
	}
	root := state.records[criticRoot]
	foldedRound, ok := numInt(root[findingRegisterRoundField])
	evidenceRound, roundOK := numInt(evidence["round"])
	if !ok || !roundOK || foldedRound < evidenceRound {
		return fmt.Errorf("critic evidence job %s has not been folded into its canonical finding register", evidenceJob)
	}
	return nil
}

func stampReviewReference(repoRoot string, state critiqueState, evidenceJob, reviews, field, expectedRoot string) error {
	reviewed, present := state.records[reviews]
	if !present {
		return fmt.Errorf("review evidence job %s names unreadable reviewed job %s", evidenceJob, reviews)
	}
	if asString(reviewed["role"]) != "implementer" {
		return fmt.Errorf("review evidence job %s must review an implementer job", evidenceJob)
	}
	rootJob := state.chainRoot(reviews)
	if rootJob == "" || rootJob == state.chainRoot(evidenceJob) {
		return fmt.Errorf("review evidence job %s does not review a distinct valid chain", evidenceJob)
	}
	if expectedRoot != "" && rootJob != expectedRoot {
		return fmt.Errorf("review evidence job %s belongs to reviewed chain %s, not %s", evidenceJob, rootJob, expectedRoot)
	}
	return withRecordLock(repoRoot, rootJob, func(recordPath string) error {
		root, err := readObject(recordPath)
		if err != nil || asString(root["jobId"]) != rootJob {
			return fmt.Errorf("reviewed chain root %s is unreadable", rootJob)
		}
		root[field] = evidenceJob
		return writeRecord(recordPath, root)
	})
}
