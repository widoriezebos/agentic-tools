package dispatch

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
)

// ValidateHumanCarriedCritic proves that a cited critic root completed a
// zero-material review of the exact carried commit and folded its terminal
// round into a closed register.
func ValidateHumanCarriedCritic(repoRoot, rootJob, commit string) error {
	state := loadCritiqueState(repoRoot)
	root, present := state.records[rootJob]
	expected := "commit:" + commit
	if !present || asString(root["role"]) != "code-critic" || asString(root["reviews"]) != expected {
		return fmt.Errorf("expected a code-critic root job reviewing %s", expected)
	}
	latest := state.latestMember(rootJob)
	if latest == nil || asString(latest["status"]) != "completed" {
		return fmt.Errorf("expected a completed code-critic root job reviewing %s", expected)
	}
	latestID := asString(latest["jobId"])
	if err := requireFoldedCritique(state, latestID, latest); err != nil {
		return err
	}
	register, present, err := critiqueFindingRegister(root)
	if err != nil || !present || len(openRegisterFindingIDs(register)) != 0 {
		return fmt.Errorf("code-critic root %s has no closed finding register", rootJob)
	}
	round, ok := numInt(latest["round"])
	if !ok || round < 1 {
		return fmt.Errorf("code-critic root %s has no terminal round", rootJob)
	}
	data, err := os.ReadFile(filepath.Join(repoRoot, "artifacts", "agents", rootJob, "rounds", fmt.Sprint(round), "return.json"))
	if err != nil {
		return fmt.Errorf("code-critic root %s has no terminal return", rootJob)
	}
	var returned map[string]any
	if json.Unmarshal(data, &returned) != nil {
		return fmt.Errorf("code-critic root %s has a malformed terminal return", rootJob)
	}
	material, ok := numInt(returned["verdictMaterialCount"])
	if !ok || material != 0 {
		return fmt.Errorf("code-critic root %s did not return zero material findings", rootJob)
	}
	return nil
}

// ValidateCommitCriticClosure proves that a closed code-critic chain is bound
// to the exact persisted commit subject supplied by its consumer.
func ValidateCommitCriticClosure(repoRoot, rootJob string, subject readsubject.ReadSubject) (readsubject.Closure, error) {
	return ValidateCommitCriticClosureAt(filepath.Join(repoRoot, "artifacts", "agents"), rootJob, subject)
}

// ValidateCommitCriticClosureAt validates a closure from an explicit agents
// root so persisted evidence can be checked without the checkout that wrote it.
func ValidateCommitCriticClosureAt(agentsRoot, rootJob string, subject readsubject.ReadSubject) (readsubject.Closure, error) {
	closure, _, err := validateCommitCriticClosureAt(agentsRoot, rootJob, subject)
	return closure, err
}

// CommitCriticClosureFiles returns the exact chain evidence needed to repeat
// closure validation below another agents root.
func CommitCriticClosureFiles(agentsRoot, rootJob string, subject readsubject.ReadSubject) (readsubject.Closure, map[string][]byte, error) {
	closure, members, err := validateCommitCriticClosureAt(agentsRoot, rootJob, subject)
	if err != nil {
		return readsubject.Closure{}, nil, err
	}
	paths := make([]string, 0, len(members)+2)
	for _, member := range members {
		paths = append(paths, member.path)
	}
	round := fmt.Sprint(closure.Round)
	paths = append(paths,
		filepath.Join(agentsRoot, rootJob, "rounds", round, "subject.json"),
		filepath.Join(agentsRoot, rootJob, "rounds", round, "return.json"),
	)
	files := make(map[string][]byte, len(paths))
	for _, path := range paths {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readsubject.Closure{}, nil, readErr
		}
		rel, relErr := filepath.Rel(agentsRoot, path)
		if relErr != nil {
			return readsubject.Closure{}, nil, relErr
		}
		files[filepath.ToSlash(rel)] = data
	}
	return closure, files, nil
}

func validateCommitCriticClosureAt(agentsRoot, rootJob string, subject readsubject.ReadSubject) (readsubject.Closure, []chainMember, error) {
	state := loadCritiqueStateAt(agentsRoot)
	root, present := state.records[rootJob]
	if !present || asString(root["role"]) != "code-critic" {
		return readsubject.Closure{}, nil, fmt.Errorf("expected %s to be a code-critic root", rootJob)
	}
	members, err := chainMembers(filepath.Join(agentsRoot, "jobs"), rootJob)
	if err != nil {
		return readsubject.Closure{}, nil, err
	}
	records := make([]map[string]any, 0, len(members))
	for _, member := range members {
		records = append(records, member.record)
	}
	closure, closurePresent, err := readsubject.ReadClosedClosure(agentsRoot, root, records)
	if err != nil {
		return readsubject.Closure{}, nil, err
	}
	if !closurePresent {
		return readsubject.Closure{}, nil, fmt.Errorf("code-critic root %s has no clean closure", rootJob)
	}
	if closure.Subject.Kind != readsubject.SubjectCommit || !closure.Subject.Equal(subject) {
		return readsubject.Closure{}, nil, fmt.Errorf("code-critic root %s did not close on commit subject %s", rootJob, subject.Digest())
	}
	return closure, members, nil
}

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
	if validCommitReview.MatchString(reviews) {
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
	if validCommitReview.MatchString(reviews) {
		return fmt.Errorf("a commit subject carries no chain pointer; the obligation on the goal is its record")
	}
	designCritic := asString(evidence["role"]) == "design-critic"
	liveCritic := asString(evidence["role"]) == "code-critic" || asString(evidence["role"]) == "warden"
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
	} else if liveCritic {
		handled, err := reconcileClosedLiveCritique(repoRoot, state, rootJob, evidenceJob, evidence, final)
		if err != nil {
			return err
		}
		if handled {
			return nil
		}
		if reviews != final.job {
			return fmt.Errorf("review evidence job %s reviews %s instead of terminal work round %s", evidenceJob, reviews, final.job)
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

func reconcileClosedLiveCritique(repoRoot string, state critiqueState, reviewedRoot, suppliedJob string, evidence map[string]any, final hazardFinalWorkState) (bool, error) {
	criticRoot := state.chainRoot(suppliedJob)
	if criticRoot == "" {
		return false, fmt.Errorf("review evidence job %s has no valid critic chain root", suppliedJob)
	}
	root := state.records[criticRoot]
	criticMembers, err := chainMembers(filepath.Join(state.agents, "jobs"), criticRoot)
	if err != nil || len(criticMembers) == 0 {
		return false, reviewClosureBindingError(suppliedJob, criticRoot, root, "critic chain membership is unreadable")
	}
	members := make([]map[string]any, 0, len(criticMembers))
	for _, member := range criticMembers {
		members = append(members, member.record)
	}
	closure, present, err := readsubject.ReadClosedClosure(state.agents, root, members)
	if err != nil {
		return false, reviewClosureBindingError(suppliedJob, criticRoot, root, err.Error())
	}
	if !present {
		return false, nil
	}
	role := asString(evidence["role"])
	if asString(root["status"]) != "completed" {
		return false, reviewClosureBindingError(suppliedJob, criticRoot, root, "critic root is not completed")
	}
	if rootRole := asString(root["role"]); rootRole != role {
		return false, reviewClosureBindingError(suppliedJob, criticRoot, root, fmt.Sprintf("critic root role %q does not equal supplied member role %q", rootRole, role))
	}
	rootReviews := asString(root["reviews"])
	reviewed, reviewedPresent := state.records[rootReviews]
	if !reviewedPresent || asString(reviewed["role"]) != "implementer" {
		return false, reviewClosureBindingError(suppliedJob, criticRoot, root, fmt.Sprintf("critic root reviews binding %q does not name an implementer", rootReviews))
	}
	boundRoot := state.chainRoot(rootReviews)
	if boundRoot != reviewedRoot || boundRoot == criticRoot {
		return false, reviewClosureBindingError(suppliedJob, criticRoot, root, fmt.Sprintf("critic root reviews binding %q belongs to implementation chain %q instead of requested distinct chain %q", rootReviews, boundRoot, reviewedRoot))
	}
	if closure.Subject.Kind != readsubject.SubjectLive {
		return false, reviewClosureBindingError(suppliedJob, criticRoot, root, fmt.Sprintf("closure subject kind %q is not live", closure.Subject.Kind))
	}
	terminalSubject, _, subjectPresent, subjectErr := liveReadSubject(state, final.job)
	if subjectErr != nil || !subjectPresent {
		detail := "terminal work subject has no review.json and diff.patch"
		if subjectErr != nil {
			detail = subjectErr.Error()
		}
		return false, reviewClosureBindingError(suppliedJob, criticRoot, root, fmt.Sprintf("closure subject cannot bind terminal work round %s: %s", final.job, detail))
	}
	if !closure.Subject.Equal(terminalSubject) {
		return false, reviewClosureBindingError(suppliedJob, criticRoot, root, fmt.Sprintf("closure subject %s does not equal terminal work round %s subject %s", closure.Subject.Digest(), final.job, terminalSubject.Digest()))
	}
	if err := stampReviewReference(repoRoot, state, criticRoot, final.job, independentCritiqueReferenceField, reviewedRoot); err != nil {
		return false, reviewClosureBindingError(suppliedJob, criticRoot, root, fmt.Sprintf("canonical critic-root stamp failed: %v", err))
	}
	return true, nil
}

func reviewClosureBindingError(suppliedJob, criticRoot string, root map[string]any, detail string) error {
	round := "unknown"
	if closure, ok := root[closureField].(map[string]any); ok {
		if value, valid := numInt(closure["round"]); valid {
			round = fmt.Sprint(value)
		}
	}
	if round == "unknown" {
		if value, valid := numInt(root[findingRegisterRoundField]); valid {
			round = fmt.Sprint(value)
		}
	}
	return fmt.Errorf("review evidence job %s resolves to critic root %s closure round %s with invalid terminal-subject binding: %s", suppliedJob, criticRoot, round, detail)
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
	if role == "code-critic" && validCommitReview.MatchString(reviews) {
		return field, reviews, nil
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
