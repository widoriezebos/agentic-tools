package dispatch

import (
	"fmt"
	"path/filepath"
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
	if field == "" {
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
	if field == independentCritiqueReferenceField {
		if err := requireFoldedCritique(state, evidenceJob, evidence); err != nil {
			return err
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
	if reviews != final.job {
		return fmt.Errorf("review evidence job %s reviews %s instead of terminal work round %s", evidenceJob, reviews, final.job)
	}
	return stampReviewReference(repoRoot, state, evidenceJob, reviews, field, rootJob)
}

func reviewReferenceBinding(evidenceJob string, evidence map[string]any) (field, reviews string, err error) {
	role := asString(evidence["role"])
	reviews = asString(evidence["reviews"])
	switch role {
	case "code-critic", "warden":
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
	if !validJobID.MatchString(reviews) {
		return "", "", fmt.Errorf("review evidence job %s has no valid reviews binding", evidenceJob)
	}
	return field, reviews, nil
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
