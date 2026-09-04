package dispatch

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	critiqueModel "github.com/widoriezebos/agentic-tools/metasystem/internal/critique"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"golang.org/x/sys/unix"
)

const findingRegisterField = "findingRegister"
const findingRegisterRoundField = "findingRegisterRound"
const reviewRoundLimitField = "reviewRoundLimit"
const criticRoundsConsumedField = "criticRoundsConsumed"

type registerFinding struct {
	FindingID      string
	Critic         string
	RigorClass     critiqueModel.RigorClass
	FactsDigest    string
	Facts          any
	Artifact       string
	Title          string
	Status         string
	Resolution     string
	DecisionOpID   string
	Evidence       string
	EvidenceDigest string
	Multiplicity   int64
}

type reviewedSubject struct {
	ReviewsTarget string
	ReviewedTree  string
}

func (subject reviewedSubject) matches(other reviewedSubject) bool {
	return (subject.ReviewsTarget != "" && subject.ReviewsTarget == other.ReviewsTarget) ||
		(subject.ReviewedTree != "" && subject.ReviewedTree == other.ReviewedTree)
}

func (subject reviewedSubject) String() string {
	return fmt.Sprintf("reviews=%q reviewedTree=%q", subject.ReviewsTarget, subject.ReviewedTree)
}

// CritiqueRegisterAdvance folds one terminal critic attempt into the canonical
// register on its chain root. Completed attempts consume their return;
// failures consume a synthetic unproven finding. The operation serializes the
// cross-root conflict check and the root-record write. A retry for an already
// folded round returns unchanged without reading or publishing its return.
func CritiqueRegisterAdvance(repoRoot, rootJob, roundJob string) (outcome string, err error) {
	return withFindingRegisterLock(repoRoot, func() (string, error) {
		state := loadCritiqueState(repoRoot)
		roundRecord, present := state.records[roundJob]
		if !present {
			return "", fmt.Errorf("critic round job record %s is unreadable", roundJob)
		}
		if state.chainRoot(roundJob) != rootJob {
			return "", fmt.Errorf("critic round %s does not belong to chain root %s", roundJob, rootJob)
		}
		role := asString(roundRecord["role"])
		if role != "design-critic" && role != "code-critic" && role != "warden" {
			return "", fmt.Errorf("job %s is not a critic round", roundJob)
		}
		status := asString(roundRecord["status"])
		failedAttempt := status == "failed"
		// A cancelled round folds as a terminal that consumes NO cap
		// and contributes NO findings: the critique never happened,
		// but an unfoldable status would wedge every follow-up behind
		// it (first drawn live 2026-08-29: a coordinator cancel
		// deadlocked its own chain against the fold gate).
		cancelledRound := status == "cancelled"
		if status != "completed" && !failedAttempt && !cancelledRound {
			return "", fmt.Errorf("critic round %s is neither completed, failed, nor cancelled", roundJob)
		}
		round, ok := numInt(roundRecord["round"])
		if !ok || round < 1 {
			return "", fmt.Errorf("critic round %s has an invalid round number", roundJob)
		}
		lockErr := withRecordLock(repoRoot, rootJob, func(recordPath string) error {
			root, rootErr := readObject(recordPath)
			if rootErr != nil {
				return fmt.Errorf("critique root record %s is unreadable: %v", rootJob, rootErr)
			}
			register, _, registerErr := critiqueFindingRegister(root)
			if registerErr != nil {
				return fmt.Errorf("critique root record %s has a malformed finding register: %v", rootJob, registerErr)
			}
			foldedRound, roundErr := findingRegisterRound(root, len(register))
			if roundErr != nil {
				return fmt.Errorf("critique root record %s has malformed register round state: %v", rootJob, roundErr)
			}
			if round <= foldedRound {
				outcome = "unchanged"
				return nil
			}
			if round != foldedRound+1 {
				return refuse(3, "critique register round %d cannot advance before round %d has been folded", round, foldedRound+1)
			}

			advanced := register
			if cancelledRound {
				// Neutral fold: the round number advances so the chain
				// unwedges, the register's findings and caps are
				// untouched — a cancellation is nobody's critique.
			} else if failedAttempt {
				advanced = foldProtocolError(register, role, roundJob, roundRecord)
			} else {
				resultPath := filepath.Join(state.agents, rootJob, "rounds", fmt.Sprint(round), "return.json")
				result, readErr := readObject(resultPath)
				if readErr != nil {
					return fmt.Errorf("critique return for job %s is unreadable: %v", roundJob, readErr)
				}
				if asString(result["jobId"]) != roundJob {
					return fmt.Errorf("critique return for job %s carries a different job identifier", roundJob)
				}
				returnedRound, roundOK := numInt(result["round"])
				if !roundOK || returnedRound != round {
					return fmt.Errorf("critique return for job %s carries a different round number", roundJob)
				}
				findings, findingsOK := result["findings"].([]any)
				if !findingsOK {
					return fmt.Errorf("critique return for job %s has no findings array", roundJob)
				}
				subject, subjectErr := critiqueSubjectForRound(repoRoot, state, root, role, result)
				if subjectErr != nil {
					return subjectErr
				}
				var demotions []any
				advanced, demotions, registerErr = foldCritiqueFindings(register, role, roundJob, findings, result["rigor"], subject, round)
				if registerErr != nil {
					return registerErr
				}
				if len(demotions) > 0 {
					current := []any{}
					if value, present := root["demotions"]; present {
						var ok bool
						current, ok = value.([]any)
						if !ok {
							return fmt.Errorf("critique root record %s has malformed demotions", rootJob)
						}
					}
					root["demotions"] = append(current, demotions...)
				}
			}
			if conflictErr := refuseCrossRootClassConflict(state, rootJob, advanced); conflictErr != nil {
				return conflictErr
			}
			accounting, accountingErr := critiqueRoundAccounting(repoRoot, state, rootJob, root)
			if accountingErr != nil {
				return malformedRoundAccounting(rootJob, accountingErr)
			}
			if !cancelledRound && !accounting.consumedMissing {
				accounting.consumed++
			}
			root[reviewRoundLimitField] = accounting.limit
			root[criticRoundsConsumedField] = accounting.consumed
			after := encodeFindingRegister(advanced)
			root[findingRegisterField] = after
			root[findingRegisterRoundField] = round
			if writeErr := writeRecord(recordPath, root); writeErr != nil {
				return writeErr
			}
			outcome = "advanced"
			return nil
		})
		return outcome, lockErr
	})
}

type critiqueRoundAccount struct {
	limit, consumed int64
	consumedMissing bool
}

func critiqueRoundAccounting(repoRoot string, state critiqueState, rootJob string, root map[string]any) (critiqueRoundAccount, error) {
	var account critiqueRoundAccount
	limitValue, limitPresent := root[reviewRoundLimitField]
	if limitPresent {
		limit, ok := numInt(limitValue)
		if !ok || limit < 1 || limit > 255 {
			return account, fmt.Errorf("reviewRoundLimit is not a positive eight-bit integer")
		}
		account.limit = limit
	} else {
		revision, _ := numInt(root["goalRevision"])
		limit, err := goalReviewRoundLimit(repoRoot, asString(root["goalId"]), uint64(max(revision, 0)))
		if err != nil || limit == 0 {
			return account, fmt.Errorf("cannot resolve a positive goal review-round limit: %v", err)
		}
		account.limit = int64(limit)
	}

	consumedValue, consumedPresent := root[criticRoundsConsumedField]
	if consumedPresent {
		consumed, ok := numInt(consumedValue)
		if !ok || consumed < 0 {
			return account, fmt.Errorf("criticRoundsConsumed is not a non-negative integer")
		}
		account.consumed = consumed
		return account, nil
	}
	account.consumedMissing = true
	for jobID, record := range state.records {
		if state.chainRoot(jobID) != rootJob {
			continue
		}
		status := asString(record["status"])
		if status == "completed" || status == "failed" {
			account.consumed++
		}
	}
	return account, nil
}

func malformedRoundAccounting(rootJob string, err error) error {
	return fmt.Errorf("critique root record %s has malformed round accounting: %v; next: job critique-budget-rebind --root-job %s", rootJob, err, rootJob)
}

func foldProtocolError(register []registerFinding, role, roundJob string, roundRecord map[string]any) []registerFinding {
	advanced := append([]registerFinding(nil), register...)
	id := syntheticProtocolFindingID(role, roundJob)
	for _, finding := range advanced {
		if finding.FindingID == id {
			return advanced
		}
	}
	advanced = append(advanced, registerFinding{
		FindingID: id, Critic: roundJob, RigorClass: critiqueModel.Unproven,
		FactsDigest: digestJSON(nil), Status: "open",
		EvidenceDigest: digestJSON(map[string]any{
			"error": roundRecord["error"], "phase": roundRecord["phase"], "protocolError": roundRecord["protocolError"],
		}), Multiplicity: 1,
	})
	return advanced
}

func syntheticProtocolFindingID(role, roundJob string) string {
	sum := sha256.Sum256(canonicalJSON([]any{"protocol_error", role, roundJob}))
	return "synthetic-" + hex.EncodeToString(sum[:])
}

func openRegisterFindingIDs(register []registerFinding) []string {
	var ids []string
	for _, finding := range register {
		if finding.Status == "open" || finding.Status == "disputed" {
			ids = append(ids, finding.FindingID)
		}
	}
	sort.Strings(ids)
	return ids
}

// CritiqueOpenFindingIDs returns the sorted open and disputed finding
// identifiers carried by a critic chain's canonical register. The record lock
// joins this read to register advances, so prompt assembly never observes a
// partially published register.
func CritiqueOpenFindingIDs(repoRoot, rootJob string) (ids []string, err error) {
	err = withRecordLock(repoRoot, rootJob, func(recordPath string) error {
		root, readErr := readObject(recordPath)
		if readErr != nil {
			return fmt.Errorf("critique root record %s is unreadable: %v", rootJob, readErr)
		}
		role := asString(root["role"])
		if role != "design-critic" && role != "code-critic" && role != "warden" {
			return fmt.Errorf("job %s is not a critic chain root", rootJob)
		}
		registerValue, present := root[findingRegisterField]
		if !present {
			return fmt.Errorf("critic chain %s has no canonical finding register", rootJob)
		}
		register, decodeErr := decodeFindingRegister(registerValue)
		if decodeErr != nil {
			return fmt.Errorf("critic chain %s has a malformed finding register: %v", rootJob, decodeErr)
		}
		ids = openRegisterFindingIDs(register)
		return nil
	})
	return ids, err
}

type CritiqueDecisionFinding struct {
	FindingID, Chain, GoalID, RigorClass, Artifact, Title, Claim, Evidence string
	Facts                                                                  any
}

func CritiqueRegisterDecisionFinding(repoRoot, rootJob, findingID, goalID string) (CritiqueDecisionFinding, error) {
	var result CritiqueDecisionFinding
	err := withRecordLock(repoRoot, rootJob, func(path string) error {
		root, err := readObject(path)
		if err != nil {
			return err
		}
		if asString(root["goalId"]) != goalID {
			return fmt.Errorf("critic chain %s is not bound to goal %s", rootJob, goalID)
		}
		register, err := decodeFindingRegister(root[findingRegisterField])
		if err != nil {
			return err
		}
		for _, f := range register {
			if f.FindingID == findingID {
				// The accepted-risk status is readable only so the three-step
				// transaction can be replayed after all steps landed. Creation
				// still transitions exclusively from open or disputed below.
				if f.Status != "open" && f.Status != "disputed" && f.Status != "accepted-risk" {
					return fmt.Errorf("finding %s is not open or disputed", findingID)
				}
				if f.RigorClass == critiqueModel.Bounded {
					return fmt.Errorf("bounded findings defer at close, not by acceptance")
				}
				claim, evidence := f.Title, f.Evidence
				if claim == "" || evidence == "" {
					if originalClaim, originalEvidence, ok := criticFindingText(repoRoot, rootJob, f.Critic, findingID); ok {
						claim, evidence = originalClaim, originalEvidence
					}
				}
				result = CritiqueDecisionFinding{FindingID: f.FindingID, Chain: rootJob, GoalID: goalID, RigorClass: string(f.RigorClass), Artifact: f.Artifact, Title: f.Title, Claim: claim, Evidence: evidence, Facts: f.Facts}
				return nil
			}
		}
		return fmt.Errorf("finding %s is not in critic root %s", findingID, rootJob)
	})
	return result, err
}

func criticFindingText(repoRoot, rootJob, criticJob, findingID string) (string, string, bool) {
	record, err := readObject(filepath.Join(repoRoot, "artifacts", "agents", "jobs", criticJob+".json"))
	if err != nil {
		return "", "", false
	}
	round, ok := numInt(record["round"])
	if !ok || round < 1 {
		return "", "", false
	}
	result, err := readObject(filepath.Join(repoRoot, "artifacts", "agents", rootJob, "rounds", fmt.Sprint(round), "return.json"))
	if err != nil {
		return "", "", false
	}
	findings, _ := result["findings"].([]any)
	for _, raw := range findings {
		finding, _ := raw.(map[string]any)
		if asString(finding["id"]) == findingID {
			return asString(finding["claim"]), asString(finding["evidence"]), true
		}
	}
	return "", "", false
}

func CritiqueRegisterAcceptRisk(repoRoot, rootJob, findingID, opid string) error {
	_, err := withFindingRegisterLock(repoRoot, func() (string, error) {
		return "", withRecordLock(repoRoot, rootJob, func(path string) error {
			root, err := readObject(path)
			if err != nil {
				return err
			}
			register, err := decodeFindingRegister(root[findingRegisterField])
			if err != nil {
				return err
			}
			found := false
			changed := false
			for i := range register {
				if register[i].FindingID == findingID {
					found = true
					if register[i].Status == "accepted-risk" && register[i].DecisionOpID == opid {
						return nil
					}
					if register[i].Status != "open" && register[i].Status != "disputed" {
						return fmt.Errorf("finding %s is not open or disputed", findingID)
					}
					register[i].Status = "accepted-risk"
					register[i].Resolution = "accepted-risk"
					register[i].DecisionOpID = opid
					changed = true
				}
			}
			if !found {
				return fmt.Errorf("finding %s is absent", findingID)
			}
			if changed {
				root[findingRegisterField] = encodeFindingRegister(register)
				return writeRecord(path, root)
			}
			return nil
		})
	})
	return err
}

func CritiqueRegisterResolveOutOfScope(repoRoot, rootJob string, findingIDs []string) error {
	_, err := withFindingRegisterLock(repoRoot, func() (string, error) {
		return "", withRecordLock(repoRoot, rootJob, func(path string) error {
			root, err := readObject(path)
			if err != nil {
				return err
			}
			register, err := decodeFindingRegister(root[findingRegisterField])
			if err != nil {
				return err
			}
			wanted := map[string]bool{}
			for _, id := range findingIDs {
				wanted[id] = true
			}
			for _, f := range register {
				if wanted[f.FindingID] && (f.RigorClass == critiqueModel.Severe || f.RigorClass == critiqueModel.Unproven) {
					return fmt.Errorf("finding %s is %s and cannot be resolved out-of-scope", f.FindingID, f.RigorClass)
				}
			}
			changed := false
			for i := range register {
				if wanted[register[i].FindingID] {
					delete(wanted, register[i].FindingID)
					if register[i].Status == "resolved" && register[i].Resolution == "out-of-scope" {
						continue
					}
					if register[i].Status != "open" && register[i].Status != "disputed" {
						return fmt.Errorf("finding %s is not open or disputed", register[i].FindingID)
					}
					register[i].Status = "resolved"
					register[i].Resolution = "out-of-scope"
					changed = true
				}
			}
			if len(wanted) > 0 {
				ids := make([]string, 0, len(wanted))
				for id := range wanted {
					ids = append(ids, id)
				}
				sort.Strings(ids)
				return fmt.Errorf("finding identifiers are absent: %s", strings.Join(ids, ", "))
			}
			if changed {
				root[findingRegisterField] = encodeFindingRegister(register)
				return writeRecord(path, root)
			}
			return nil
		})
	})
	return err
}

func CritiqueRegisterClose(repoRoot, rootJob string) (string, error) {
	return withFindingRegisterLock(repoRoot, func() (string, error) {
		state := loadCritiqueState(repoRoot)
		outcome := "closed"
		err := withRecordLock(repoRoot, rootJob, func(path string) error {
			root, err := readObject(path)
			if err != nil {
				return err
			}
			register, present, err := critiqueFindingRegister(root)
			if err != nil {
				return err
			}
			if !present {
				return nil
			}
			var unresolved []int
			var blockers []string
			for i, f := range register {
				if f.Status == "open" || f.Status == "disputed" {
					unresolved = append(unresolved, i)
					if f.RigorClass == critiqueModel.Severe || f.RigorClass == critiqueModel.Unproven {
						blockers = append(blockers, fmt.Sprintf("finding %s artifact=%s is %s and blocks close", f.FindingID, f.Artifact, f.RigorClass))
					}
				}
				if f.Resolution == "out-of-scope" && (f.RigorClass == critiqueModel.Severe || f.RigorClass == critiqueModel.Unproven) {
					blockers = append(blockers, fmt.Sprintf("finding %s artifact=%s is illegally resolved out-of-scope", f.FindingID, f.Artifact))
				}
			}
			if len(blockers) > 0 {
				return fmt.Errorf("%s; next: goal accept-risk --finding <id> --chain <root> --by <human> --why, or raise the goal budget and run job critique-budget-rebind", strings.Join(blockers, "\n"))
			}
			if len(unresolved) == 0 {
				return nil
			}
			accounting, err := critiqueRoundAccounting(repoRoot, state, rootJob, root)
			if err != nil {
				return malformedRoundAccounting(rootJob, err)
			}
			if accounting.consumed < accounting.limit {
				return fmt.Errorf("review budget is not exhausted; dispatch the next round")
			}
			goalID := asString(root["goalId"])
			machine := asString(root["machineId"])
			lineage := asString(root["mainId"])
			epoch, _ := numInt(root["claimEpoch"])
			if goalID == "" || machine == "" || lineage == "" || epoch < 1 {
				return fmt.Errorf("critic root has no complete owning goal pair")
			}
			endpoint, err := goal.ResolveEndpoint(repoRoot)
			if err != nil {
				return err
			}
			req := goal.VerbRequest{Endpoint: endpoint, Actor: goal.Actor{Machine: machine, Lineage: lineage}, Ulid: deterministicULID(rootJob), Now: time.Now().UTC(), ClaimEpoch: epoch}
			obligations := make([]goal.ReviewObligation, 0, len(unresolved))
			for _, i := range unresolved {
				f := register[i]
				obligations = append(obligations, goal.ReviewObligation{Finding: f.FindingID, Chain: rootJob, Artifact: f.Artifact, Test: "prove: " + strings.Split(f.Title, "\n")[0], State: "open"})
			}
			if _, err := goal.DeferFindings(req, goalID, obligations); err != nil {
				return err
			}
			opid := goal.Opid(req.Ulid, machine, lineage)
			for _, i := range unresolved {
				register[i].Status = "deferred"
				register[i].Resolution = "deferred"
				register[i].DecisionOpID = opid
			}
			root[findingRegisterField] = encodeFindingRegister(register)
			outcome = "deferred"
			return writeRecord(path, root)
		})
		return outcome, err
	})
}

func deterministicULID(value string) string {
	const alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	sum := sha256.Sum256([]byte(value))
	out := make([]byte, 26)
	out[0] = '0'
	for i := 1; i < len(out); i++ {
		out[i] = alphabet[sum[(i-1)%len(sum)]&31]
	}
	return string(out)
}

func CritiqueBudgetRebind(repoRoot, rootJob string) (outcome string, err error) {
	return withFindingRegisterLock(repoRoot, func() (string, error) {
		state := loadCritiqueState(repoRoot)
		err := withRecordLock(repoRoot, rootJob, func(path string) error {
			root, e := readObject(path)
			if e != nil {
				return e
			}
			role := asString(root["role"])
			if role != "design-critic" && role != "code-critic" && role != "warden" {
				return fmt.Errorf("job %s is not a critic chain root", rootJob)
			}
			goalID := asString(root["goalId"])
			if goalID == "" {
				return fmt.Errorf("critic chain %s is not bound to a goal", rootJob)
			}
			revision, _, e := ResolveGoalRevision(repoRoot, goalID)
			if e != nil {
				return e
			}
			limit, e := goalReviewRoundLimit(repoRoot, goalID, revision)
			if e != nil || limit == 0 {
				return fmt.Errorf("cannot resolve a positive goal review-round limit: %v", e)
			}
			opid := fmt.Sprintf("critique-budget-rebind-%s-r%d", rootJob, revision)
			_, limitPresent := root[reviewRoundLimitField]
			_, consumedPresent := root[criticRoundsConsumedField]
			if binding, ok := root["critiqueBudgetBinding"].(map[string]any); ok && asString(binding["opid"]) == opid && limitPresent && consumedPresent {
				outcome = "unchanged"
				return nil
			}
			root[reviewRoundLimitField] = limit
			if !consumedPresent {
				var consumed int64
				for jobID, record := range state.records {
					if state.chainRoot(jobID) == rootJob && (asString(record["status"]) == "completed" || asString(record["status"]) == "failed") {
						consumed++
					}
				}
				root[criticRoundsConsumedField] = consumed
			}
			root["critiqueBudgetBinding"] = map[string]any{"goalId": goalID, "goalRevision": revision, "opid": opid}
			if e := writeRecord(path, root); e != nil {
				return e
			}
			outcome = "rebound"
			return nil
		})
		return outcome, err
	})
}

func findingRegisterRound(root map[string]any, registerSize int) (int64, error) {
	value, present := root[findingRegisterRoundField]
	if !present {
		if registerSize == 0 {
			return 0, nil
		}
		return 0, fmt.Errorf("the last folded round is absent from a non-empty register")
	}
	round, ok := numInt(value)
	if !ok || round < 0 {
		return 0, fmt.Errorf("the last folded round is not a non-negative integer")
	}
	return round, nil
}

func withFindingRegisterLock(root string, fn func() (string, error)) (string, error) {
	lockPath := filepath.Join(root, "artifacts", "agents", "finding-register.lock")
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return "", err
	}
	handle, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return "", fmt.Errorf("cannot open finding-register lock: %w", err)
	}
	defer handle.Close()
	deadline := time.Now().Add(recordLockWait())
	for {
		if err := unix.Flock(int(handle.Fd()), unix.LOCK_EX|unix.LOCK_NB); err == nil {
			break
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("finding-register lock is busy after %s", recordLockWait())
		}
		time.Sleep(50 * time.Millisecond)
	}
	defer unix.Flock(int(handle.Fd()), unix.LOCK_UN)
	return fn()
}

func decodeFindingRegister(value any) ([]registerFinding, error) {
	items, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("the register is not an array")
	}
	seen := map[string]bool{}
	register := make([]registerFinding, 0, len(items))
	for index, raw := range items {
		entry, ok := raw.(map[string]any)
		if !ok || (len(entry) != 7 && len(entry) != 13) {
			return nil, fmt.Errorf("entry %d is not an object with the canonical fields", index)
		}
		finding := registerFinding{
			FindingID:      asString(entry["findingId"]),
			Critic:         asString(entry["critic"]),
			RigorClass:     critiqueModel.RigorClass(asString(entry["rigorClass"])),
			FactsDigest:    asString(entry["factsDigest"]),
			Facts:          entry["facts"],
			Artifact:       asString(entry["artifact"]),
			Title:          asString(entry["title"]),
			Status:         asString(entry["status"]),
			Resolution:     asString(entry["resolution"]),
			DecisionOpID:   asString(entry["decisionOpid"]),
			Evidence:       asString(entry["evidence"]),
			EvidenceDigest: asString(entry["evidenceDigest"]),
		}
		if len(entry) == 7 && finding.Status == "resolved" {
			finding.Resolution = "withdrawn"
		}
		finding.Multiplicity, ok = numInt(entry["multiplicity"])
		if finding.FindingID == "" || finding.Critic == "" || !finding.RigorClass.Valid() ||
			!hexDigest64.MatchString(finding.FactsDigest) || !hexDigest64.MatchString(finding.EvidenceDigest) ||
			!ok || finding.Multiplicity < 1 ||
			(finding.Status != "open" && finding.Status != "resolved" && finding.Status != "disputed" && finding.Status != "deferred" && finding.Status != "accepted-risk") {
			return nil, fmt.Errorf("entry %d has invalid canonical values", index)
		}
		if len(entry) == 13 {
			unresolved := finding.Status == "open" || finding.Status == "disputed"
			if unresolved && (finding.Resolution != "" || finding.DecisionOpID != "") {
				return nil, fmt.Errorf("entry %d carries a resolution while unresolved", index)
			}
			if !unresolved && finding.Resolution == "" {
				return nil, fmt.Errorf("entry %d is non-open without a resolution", index)
			}
			validResolution := finding.Status == "resolved" && (finding.Resolution == "withdrawn" || finding.Resolution == "out-of-scope") ||
				finding.Status == "deferred" && finding.Resolution == "deferred" && finding.DecisionOpID != "" ||
				finding.Status == "accepted-risk" && finding.Resolution == "accepted-risk" && finding.DecisionOpID != ""
			if !unresolved && !validResolution {
				return nil, fmt.Errorf("entry %d has a status/resolution mismatch", index)
			}
		}
		if seen[finding.FindingID] {
			return nil, fmt.Errorf("finding identifier %s appears more than once", finding.FindingID)
		}
		seen[finding.FindingID] = true
		register = append(register, finding)
	}
	return register, nil
}

// critiqueFindingRegister is the compatibility boundary shared by register
// advance and both close readers. An absent register is a chain created before
// slice 2b and follows the pre-register path; a present malformed register is
// always an error.
func critiqueFindingRegister(root map[string]any) ([]registerFinding, bool, error) {
	value, present := root[findingRegisterField]
	if !present {
		return nil, false, nil
	}
	register, err := decodeFindingRegister(value)
	return register, true, err
}

func encodeFindingRegister(register []registerFinding) []any {
	items := make([]any, len(register))
	for index, finding := range register {
		items[index] = map[string]any{
			"findingId": finding.FindingID, "critic": finding.Critic,
			"rigorClass": string(finding.RigorClass), "factsDigest": finding.FactsDigest,
			"status": finding.Status, "evidenceDigest": finding.EvidenceDigest,
			"multiplicity": finding.Multiplicity, "facts": finding.Facts,
			"artifact": finding.Artifact, "title": finding.Title,
			"resolution": finding.Resolution, "decisionOpid": finding.DecisionOpID,
			"evidence": finding.Evidence,
		}
	}
	return items
}

type critiqueSubject struct {
	paths          map[string]bool
	repoRoot, tree string
	legacy         bool
}

func critiqueSubjectForRound(repoRoot string, state critiqueState, root map[string]any, role string, result map[string]any) (critiqueSubject, error) {
	s := critiqueSubject{paths: map[string]bool{}, repoRoot: repoRoot}
	if root["reviews"] == nil && root["declaredOutputs"] == nil {
		s.legacy = true
		return s, nil
	}
	if role == "design-critic" {
		outputs, ok := root["declaredOutputs"].([]any)
		if !ok || len(outputs) == 0 {
			return s, fmt.Errorf("design-critic root has no declared outputs; dispatch it with --outputs <file>")
		}
		for _, output := range outputs {
			path := asString(output)
			ref, err := critiqueModel.ParseArtifactRef(path)
			if err != nil || ref.Kind != critiqueModel.ArtifactPath {
				return s, fmt.Errorf("design-critic root has malformed declaredOutputs")
			}
			s.paths[path] = true
		}
		s.tree = asString(result["reviewedCommit"])
		return s, nil
	}
	reviewedJob := asString(root["reviews"])
	reviewed, ok := state.records[reviewedJob]
	if !ok || asString(reviewed["role"]) != "implementer" {
		return s, fmt.Errorf("critic root does not name a reviewed implementer round")
	}
	round, ok := numInt(reviewed["round"])
	if !ok || round < 1 {
		return s, fmt.Errorf("reviewed implementer round is malformed")
	}
	diffPath := filepath.Join(state.agents, state.chainRoot(reviewedJob), "rounds", fmt.Sprint(round), "diff.patch")
	data, err := os.ReadFile(diffPath)
	if err != nil {
		return s, fmt.Errorf("reviewed implementer round has no diff.patch; run conformance --stage review first")
	}
	installPrefix, err := projectInstallPrefix(repoRoot)
	if err != nil {
		return s, fmt.Errorf("cannot derive the reviewed project's install prefix: %v", err)
	}
	canonicalPath := func(projectRelative string) string {
		if installPrefix == "" {
			return projectRelative
		}
		return installPrefix + "/" + projectRelative
	}
	for _, line := range strings.Split(string(data), "\n") {
		var path string
		if strings.HasPrefix(line, "diff --git a/") {
			header := strings.TrimPrefix(line, "diff --git a/")
			if oldPath, newPath, found := strings.Cut(header, " b/"); found {
				for _, candidate := range []string{oldPath, newPath} {
					candidate = canonicalPath(candidate)
					if ref, e := critiqueModel.ParseArtifactRef(candidate); e == nil && ref.Kind == critiqueModel.ArtifactPath {
						s.paths[candidate] = true
					}
				}
			}
		}
		if strings.HasPrefix(line, "rename from ") {
			path = strings.TrimPrefix(line, "rename from ")
		}
		if strings.HasPrefix(line, "rename to ") {
			path = strings.TrimPrefix(line, "rename to ")
		}
		if path != "" {
			path = canonicalPath(path)
			if ref, e := critiqueModel.ParseArtifactRef(path); e == nil && ref.Kind == critiqueModel.ArtifactPath {
				s.paths[path] = true
			}
		}
	}
	if len(s.paths) == 0 {
		return s, fmt.Errorf("reviewed implementer round has no changed paths in diff.patch; run conformance --stage review first")
	}
	s.tree = asString(result["reviewedTree"])
	return s, nil
}

func (s critiqueSubject) demoteReason(value string) string {
	ref, err := critiqueModel.ParseArtifactRef(value)
	if err != nil {
		return "material finding has no canonical artifact"
	}
	if s.legacy {
		return ""
	}
	switch ref.Kind {
	case critiqueModel.ArtifactPath:
		if !s.paths[ref.Path] {
			return "artifact is outside the reviewed subject set"
		}
	case critiqueModel.ArtifactRename:
		if !s.paths[ref.Old] && !s.paths[ref.New] {
			return "neither rename side is in the reviewed subject set"
		}
	case critiqueModel.ArtifactNew:
		if !s.paths[ref.Path] {
			return "NEW artifact is not a declared or changed output"
		}
		if s.tree == "" || !artifactAbsentFromTree(s.repoRoot, s.tree, ref.Path) {
			return "NEW artifact is present in or unproven absent from the reviewed tree"
		}
	}
	return ""
}

func artifactAbsentFromTree(repoRoot, tree, path string) bool {
	if exec.Command("git", "-C", repoRoot, "cat-file", "-e", tree+"^{tree}").Run() != nil {
		return false
	}
	return exec.Command("git", "-C", repoRoot, "cat-file", "-e", tree+":"+path).Run() != nil
}

func foldCritiqueFindings(register []registerFinding, role, roundJob string, findings []any, rigorValue any, subject critiqueSubject, round int64) ([]registerFinding, []any, error) {
	advanced := append([]registerFinding(nil), register...)
	var demotions []any
	byID := map[string]int{}
	for index, finding := range advanced {
		byID[finding.FindingID] = index
	}
	rigorRows := rigorRowsByID(rigorValue)
	identities := findingIdentities(role, findings)
	processed := map[string]bool{}
	for index, raw := range findings {
		finding, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		identity := identities[index]
		id := identity.FindingID
		if processed[id] {
			continue
		}
		processed[id] = true
		material, materialOK := finding["material"].(bool)
		if !materialOK {
			continue
		}
		existingIndex, exists := byID[id]
		row := takeRigorRow(rigorRows, asString(finding["id"]))
		artifact := asString(row["artifact"])
		if material {
			if reason := subject.demoteReason(artifact); reason != "" {
				demotions = append(demotions, map[string]any{"round": round, "findingId": id, "artifact": artifact, "reason": reason})
				continue
			}
		}
		if !material {
			if exists {
				advanced[existingIndex].Status = "resolved"
				advanced[existingIndex].Resolution = "withdrawn"
			}
			continue
		}

		factsDigest := digestJSON(row["facts"])
		recurring := false
		for _, prior := range advanced {
			if prior.Status == "deferred" && prior.Artifact == artifact && prior.FactsDigest == factsDigest {
				recurring = true
				break
			}
		}
		class := critiqueModel.NormalizeWire(row["rigorClass"], row["facts"], row["reopeningTrigger"], recurring)
		title := strings.TrimSpace(strings.Split(strings.ReplaceAll(asString(finding["claim"]), "\r\n", "\n"), "\n")[0])
		candidate := registerFinding{
			FindingID: id, Critic: roundJob, RigorClass: class,
			FactsDigest: factsDigest, Facts: row["facts"], Artifact: artifact, Title: title, Status: "open",
			Evidence: asString(finding["evidence"]), EvidenceDigest: digestJSON(finding["evidence"]), Multiplicity: identity.Multiplicity,
		}
		if !exists {
			byID[id] = len(advanced)
			advanced = append(advanced, candidate)
			continue
		}
		current := advanced[existingIndex]
		if candidate.Multiplicity > current.Multiplicity {
			advanced[existingIndex].Multiplicity = candidate.Multiplicity
			current.Multiplicity = candidate.Multiplicity
		}
		if current.Critic == roundJob && current.RigorClass == candidate.RigorClass &&
			current.FactsDigest == candidate.FactsDigest && current.EvidenceDigest == candidate.EvidenceDigest {
			continue
		}
		class = critiqueModel.NormalizeWire(row["rigorClass"], row["facts"], row["reopeningTrigger"], true)
		candidate.RigorClass = class
		if rigorRank(class) < rigorRank(current.RigorClass) {
			advanced[existingIndex].Status = "disputed"
			continue
		}
		if rigorRank(class) > rigorRank(current.RigorClass) {
			candidate.Critic = current.Critic
			candidate.Multiplicity = current.Multiplicity
			advanced[existingIndex] = candidate
		}
	}
	return advanced, demotions, nil
}

func rigorRowsByID(value any) map[string][]map[string]any {
	rows := map[string][]map[string]any{}
	items, _ := value.([]any)
	for _, raw := range items {
		if row, ok := raw.(map[string]any); ok {
			id := asString(row["findingId"])
			rows[id] = append(rows[id], row)
		}
	}
	return rows
}

func takeRigorRow(rows map[string][]map[string]any, id string) map[string]any {
	queue := rows[id]
	if len(queue) == 0 {
		return map[string]any{}
	}
	row := queue[0]
	rows[id] = queue[1:]
	return row
}

func rigorRank(class critiqueModel.RigorClass) int {
	switch class {
	case critiqueModel.Severe:
		return 3
	case critiqueModel.Unproven:
		return 2
	default:
		return 1
	}
}

type findingIdentity struct {
	FindingID    string
	Multiplicity int64
}

func findingIdentities(role string, findings []any) []findingIdentity {
	rawCounts := map[string]int64{}
	claimedIDs := map[string]bool{}
	for _, raw := range findings {
		finding, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		id := asString(finding["id"])
		if id != "" && id == strings.TrimSpace(id) {
			rawCounts[id]++
			claimedIDs[id] = true
		}
	}
	identities := make([]findingIdentity, len(findings))
	counts := map[string]int64{}
	for index, raw := range findings {
		finding, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		rawID := asString(finding["id"])
		id := rawID
		if rawID == "" || rawID != strings.TrimSpace(rawID) || rawCounts[rawID] > 1 {
			id = syntheticFindingID(role, finding, claimedIDs)
		}
		identities[index].FindingID = id
		counts[id]++
	}
	for index := range identities {
		if identities[index].FindingID != "" {
			identities[index].Multiplicity = counts[identities[index].FindingID]
		}
	}
	return identities
}

func syntheticFindingID(role string, finding map[string]any, claimedIDs map[string]bool) string {
	tuple := []any{role, normalizedFindingText(finding["claim"]), normalizedFindingText(finding["evidence"])}
	for salt := 0; ; salt++ {
		value := tuple
		if salt > 0 {
			value = append(append([]any{}, tuple...), salt)
		}
		sum := sha256.Sum256(canonicalJSON(value))
		id := "synthetic-" + hex.EncodeToString(sum[:])
		if !claimedIDs[id] {
			return id
		}
	}
}

func normalizedFindingText(value any) string {
	text, ok := value.(string)
	if !ok {
		return string(canonicalJSON(value))
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	return strings.TrimSpace(text)
}

func refuseCrossRootClassConflict(state critiqueState, currentRoot string, prospective []registerFinding) error {
	currentSubject := findingRegisterSubject(state, currentRoot)
	otherClasses := map[string]map[critiqueModel.RigorClass]string{}
	otherSubjects := map[string]reviewedSubject{}
	ids := make([]string, 0, len(state.records))
	for id := range state.records {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		record := state.records[id]
		if id == currentRoot || record["parentJob"] != nil {
			continue
		}
		subject := findingRegisterSubject(state, id)
		if !currentSubject.matches(subject) {
			continue
		}
		value, present := record[findingRegisterField]
		if !present {
			continue
		}
		register, err := decodeFindingRegister(value)
		if err != nil {
			return fmt.Errorf("finding register on chain root %s is malformed; waiting on the human is the only remedy: %v", id, err)
		}
		for _, finding := range register {
			if otherClasses[finding.FindingID] == nil {
				otherClasses[finding.FindingID] = map[critiqueModel.RigorClass]string{}
			}
			otherClasses[finding.FindingID][finding.RigorClass] = id
			otherSubjects[id] = subject
		}
	}
	for _, finding := range prospective {
		for class, root := range otherClasses[finding.FindingID] {
			if class != finding.RigorClass {
				return fmt.Errorf("finding %s has conflicting rigor classes %s and %s on chain roots %s and %s, whose reviewed subjects are %s and %s; waiting on the original critic or the human is the only remedy",
					finding.FindingID, finding.RigorClass, class, currentRoot, root, currentSubject, otherSubjects[root])
			}
		}
	}
	return nil
}

func findingRegisterSubject(state critiqueState, root string) reviewedSubject {
	subject := reviewedSubject{ReviewsTarget: asString(state.records[root]["reviews"])}
	bestRound := int64(0)
	for job, record := range state.records {
		if state.chainRoot(job) != root {
			continue
		}
		round, ok := numInt(record["round"])
		if !ok || round < bestRound {
			continue
		}
		result, err := readObject(filepath.Join(state.agents, root, "rounds", fmt.Sprint(round), "return.json"))
		if err != nil {
			continue
		}
		tree := asString(result["reviewedTree"])
		if tree != "" {
			subject.ReviewedTree = tree
			bestRound = round
		}
	}
	return subject
}

func digestJSON(value any) string {
	sum := sha256.Sum256(canonicalJSON(value))
	return hex.EncodeToString(sum[:])
}

func canonicalJSON(value any) []byte {
	data, _ := json.Marshal(value)
	return data
}
