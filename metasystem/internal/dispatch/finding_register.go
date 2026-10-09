package dispatch

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	critiqueModel "github.com/widoriezebos/agentic-tools/metasystem/internal/critique"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/loopstop"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
	"golang.org/x/sys/unix"
)

const findingRegisterField = "findingRegister"
const findingRegisterRoundField = "findingRegisterRound"
const findingRegisterSubjectDigestField = "findingRegisterSubjectDigest"
const materialByRoundField = "materialByRound"
const reviewRoundLimitField = "reviewRoundLimit"
const criticRoundsConsumedField = "criticRoundsConsumed"

// acceptancesBoundField lists, one plain line each, the accepted risks that
// predate content binding and were bound to the finding as it stood when a
// fold or stamp first met them (R-142-m1e).
const acceptancesBoundField = "acceptancesBound"

func init() {
	// These fields are written only by the locked finding-register owner.
	// Register them here so generic record transitions cannot pre-seed proof.
	dedicatedMetadataFields[closureField] = true
	dedicatedMetadataFields["read"] = true
	dedicatedMetadataFields["readDigest"] = true
	dedicatedMetadataFields["unknownExaminationRetryFrom"] = true
	dedicatedMetadataFields["designStop"] = true
	dedicatedMetadataFields["findingRegisterStop"] = true
	dedicatedMetadataFields["inheritedFindings"] = true
	dedicatedMetadataFields["examinationRetryOf"] = true
	dedicatedMetadataFields[findingRegisterSubjectDigestField] = true
	dedicatedMetadataFields[cleanReadRoundsField] = true
	dedicatedMetadataFields[materialByRoundField] = true
	dedicatedMetadataFields[acceptancesBoundField] = true
}

type registerFinding struct {
	FindingID      string
	Critic         string
	RigorClass     critiqueModel.RigorClass
	Grain          string
	Fixture        string
	Class          string
	Where          string
	Change         string
	Resolves       string
	Relation       string
	TransferStop   string
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
	// AcceptedDigest is the readsubject.AcceptedFindingDigest a person's
	// accepted risk covers; set only on an accepted-risk entry.
	AcceptedDigest string

	// PlaceholderRound names the attempt that returned no review evidence.
	PlaceholderRound int64
}

// acceptanceDigest is the digest an acceptance of f must carry: its content
// only (readsubject.AcceptedFindingDigest).
func (f registerFinding) acceptanceDigest() string {
	return readsubject.AcceptedFindingDigest(encodeFindingRegister([]registerFinding{f})[0].(map[string]any))
}

// reopenChangedAcceptances reopens every accepted risk whose content is no
// longer what the person accepted: the acceptance does not cover it, so the
// finding is open again and needs a new decision (F4). A re-review of another
// tree that reports the same content keeps the acceptance.
// An acceptance that predates content binding is bound first
// (bindPredatingAcceptances), so it is never reopened for lacking a digest.
func reopenChangedAcceptances(register []registerFinding) {
	for i := range register {
		f := &register[i]
		if f.Status == "accepted-risk" && f.AcceptedDigest != "" && f.AcceptedDigest != f.acceptanceDigest() {
			f.Status, f.Resolution, f.DecisionOpID, f.AcceptedDigest = "open", "", "", ""
		}
	}
}

// bindPredatingAcceptances binds every accepted risk recorded before content
// binding (no digest) to the finding as it stands, and records one plain line
// for each on the root. A person's recorded decision is never overruled by
// the machine (R-142-m1e): from here on only a change of the finding's content
// reopens it. It reports whether it bound any.
func bindPredatingAcceptances(root map[string]any, register []registerFinding) (bool, error) {
	var lines []any
	for i := range register {
		f := &register[i]
		if f.Status != "accepted-risk" || f.AcceptedDigest != "" {
			continue
		}
		f.AcceptedDigest = f.acceptanceDigest()
		lines = append(lines, map[string]any{
			"findingId": f.FindingID, "decisionOpid": f.DecisionOpID, "acceptedDigest": f.AcceptedDigest,
			"line": fmt.Sprintf("finding %s: the acceptance (decision %s) predates content binding, so it covers the finding as it stood here; bound to digest %s", f.FindingID, f.DecisionOpID, f.AcceptedDigest),
		})
	}
	if len(lines) == 0 {
		return false, nil
	}
	current := []any{}
	if value, present := root[acceptancesBoundField]; present {
		var ok bool
		if current, ok = value.([]any); !ok {
			return false, fmt.Errorf("%s is not an array", acceptancesBoundField)
		}
	}
	root[acceptancesBoundField] = append(current, lines...)
	return true, nil
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
	return CritiqueRegisterAdvanceWithFacts(repoRoot, rootJob, roundJob, gitCritiqueSubjectFacts{})
}

// CritiqueRegisterAdvanceWithFacts folds a completed return against the
// supplied repository subject facts. Collection and register ownership are
// identical to CritiqueRegisterAdvance.
func CritiqueRegisterAdvanceWithFacts(repoRoot, rootJob, roundJob string, facts CritiqueSubjectFacts) (string, error) {
	return critiqueRegisterAdvance(repoRoot, rootJob, roundJob, facts)
}

func critiqueRegisterAdvance(repoRoot, rootJob, roundJob string, facts critiqueSubjectFacts) (outcome string, err error) {
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
		// A round cut off at its cap whose process group the reaper proved
		// dead also folds neutrally: it returned nothing to decide, so it
		// invents no finding, and the round number still advances so the
		// chain's round cap keeps counting it.
		cappedRound := status == "timeout" && asString(roundRecord["groupDeathProvenAt"]) != ""
		if status == "timeout" && !cappedRound {
			return "", fmt.Errorf("critic round %s timed out but its process group is not proven dead", roundJob)
		}
		if status != "completed" && !failedAttempt && !cancelledRound && !cappedRound {
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
			if role == "design-critic" && asString(root["unknownExaminationRetryFrom"]) == roundJob && root["designStop"] != nil && status == "completed" {
				if _, err := CollectExamination(repoRoot, roundJob); err != nil {
					outcome = "unchanged"
					return nil
				}
				retained, _ := root["read"].(map[string]any)
				if asString(retained["id"]) != roundJob {
					foldedRound = round - 1
					root[findingRegisterRoundField] = foldedRound
				}
			}
			if round <= foldedRound {
				outcome = "unchanged"
				if role == "design-critic" && root["designStop"] != nil && status == "completed" {
					if _, err := CollectExamination(repoRoot, roundJob); err != nil {
						return err
					}
					delete(root, "designStop")
					delete(root, "findingRegisterStop")
					return writeRecord(recordPath, root)
				}
				return nil
			}
			if round != foldedRound+1 {
				return refuse(3, "critique register round %d cannot advance before round %d has been folded", round, foldedRound+1).withRun(jobStatusRun(rootJob))
			}
			// A predating acceptance is bound to the content it stands on
			// before this round can change it (R-142-m1e).
			if _, bindErr := bindPredatingAcceptances(root, register); bindErr != nil {
				return fmt.Errorf("critique root record %s: %v", rootJob, bindErr)
			}
			state.records[rootJob] = root
			materialHistoryValue, materialHistoryPresent := root[materialByRoundField]
			if !materialHistoryPresent {
				materialHistoryValue = []any{}
			}
			_, historyPresent := root[cleanReadRoundsField]
			cleanReads, historyErr := cleanReadsForRoot(state, rootJob, root)
			if historyErr != nil {
				return historyErr
			}

			advanced := register
			var roundMaterial int64
			var completedSubject ReadSubject
			completedSubjectPresent := false
			completedSubjectBound := false
			if cancelledRound || cappedRound {
				// Neutral fold: the round number advances so the chain
				// unwedges, the register's findings and caps are
				// untouched — a cancellation is nobody's critique.
			} else if failedAttempt {
				advanced = foldProtocolError(register, role, roundJob, roundRecord)
				roundMaterial = int64(len(advanced) - len(register))
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
				subject, subjectErr := critiqueSubjectForRoundWithFacts(repoRoot, state, root, role, result, facts)
				if subjectErr != nil {
					return subjectErr
				}
				persisted, subjectPresent, readSubjectErr := readsubject.ReadRoundSubject(state.agents, rootJob, round)
				if readSubjectErr != nil {
					return fmt.Errorf("critique root record %s round %d has a malformed subject: %v", rootJob, round, readSubjectErr)
				}
				delete(root, findingRegisterSubjectDigestField)
				if subjectPresent {
					if validationErr := validateReadSubjectForRole(role, persisted); validationErr != nil {
						return fmt.Errorf("critique root record %s round %d has a malformed subject: %v", rootJob, round, validationErr)
					}
					root[findingRegisterSubjectDigestField] = persisted.Digest()
				}
				if role == "design-critic" && persisted.DesignPage != "" {
					if _, err := CollectExamination(repoRoot, roundJob); err != nil {
						return err
					}
				}
				completedSubject = persisted
				completedSubjectPresent = subjectPresent
				completedSubjectBound = subjectPresent && readsubject.ReturnBindsSubject(persisted, result)
				if subjectPresent && !completedSubjectBound {
					advanced = foldUnboundReturn(register, role, roundJob, persisted, result)
					roundMaterial = int64(len(advanced) - len(register))
				} else {
					var demotions []any
					version, _ := numInt(result["schemaVersion"])
					if role == "code-critic" && version == 6 || role == "design-critic" && persisted.DesignPage != "" {
						read, err := CollectExamination(repoRoot, roundJob)
						if err != nil {
							return err
						}
						data, digest := read.Canonical()
						var value any
						if err := json.Unmarshal(data, &value); err != nil {
							return err
						}
						root["read"], root["readDigest"] = value, digest
						readPath := filepath.Join(filepath.Dir(resultPath), "read.json")
						if prior, err := os.ReadFile(readPath); err == nil {
							if string(prior) != string(data) {
								return fmt.Errorf("examination %s immutable read changed", roundJob)
							}
						} else if !os.IsNotExist(err) {
							return err
						} else if _, err := atomicWriteText(readPath, data); err != nil {
							return err
						}
						if role == "design-critic" {
							encoded, _ := json.Marshal(read.Findings)
							json.Unmarshal(encoded, &findings)
							rows := rigorRowsByID(result["rigor"])
							rigor := []any{}
							for _, finding := range read.Findings {
								row := takeRigorRow(rows, finding.ID)
								row["findingId"], row["artifact"] = finding.ID, persisted.DesignPath
								rigor = append(rigor, row)
							}
							result["rigor"] = rigor
						}
						for i, raw := range findings {
							f := raw.(map[string]any)
							old := asString(f["id"])
							f["id"] = read.Findings[i].ID
							rigor, _ := result["rigor"].([]any)
							for _, rawRow := range rigor {
								row, _ := rawRow.(map[string]any)
								if asString(row["findingId"]) == old {
									row["findingId"] = read.Findings[i].ID
								}
							}
						}
					}
					if role == "code-critic" && version == 6 {
						register, registerErr = admitInheritedResolutions(register, root["inheritedFindings"], findings)
						if registerErr != nil {
							return registerErr
						}
					}
					advanced, demotions, roundMaterial, registerErr = foldCritiqueFindingsVersioned(register, role, roundJob, findings, result["rigor"], version, subject, round)
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
			}
			if completedSubjectBound {
				if err := supersedePlaceholders(state, rootJob, advanced, completedSubject, round); err != nil {
					return err
				}
			}
			reopenChangedAcceptances(advanced)
			if conflictErr := refuseCrossRootClassConflict(state, rootJob, advanced); conflictErr != nil {
				return conflictErr
			}
			accounting, accountingErr := critiqueRoundAccounting(repoRoot, state, rootJob, root)
			if accountingErr != nil {
				return malformedRoundAccounting(rootJob, accountingErr)
			}
			if !cancelledRound && !accounting.consumedMissing && asString(roundRecord["examinationRetryOf"]) == "" && (role != "design-critic" || status == "completed") {
				accounting.consumed++
			}
			root[reviewRoundLimitField] = accounting.limit
			root[criticRoundsConsumedField] = accounting.consumed
			after := encodeFindingRegister(advanced)
			if completedSubjectPresent && completedSubjectBound {
				clean, cleanErr := readsubject.CleanRegister(after)
				if cleanErr != nil {
					return fmt.Errorf("cannot validate folded clean read at round %d: %v", round, cleanErr)
				}
				if clean {
					cleanReads, historyErr = appendCleanRead(cleanReads, CleanReadRound{Round: round, Subject: completedSubject})
					if historyErr != nil {
						return historyErr
					}
				}
			}
			root[findingRegisterField] = after
			if role == "design-critic" && completedSubjectBound {
				delete(root, "designStop")
				delete(root, "findingRegisterStop")
			}
			root[findingRegisterRoundField] = round
			materialHistory, historyErr := appendMaterialRound(materialHistoryValue, round, roundMaterial, cancelledRound)
			if historyErr != nil {
				return historyErr
			}
			if materialHistoryPresent || len(materialHistory) > 0 {
				root[materialByRoundField] = materialHistory
			}
			if historyPresent || len(cleanReads) > 0 {
				root[cleanReadRoundsField] = encodeCleanReadRounds(cleanReads)
			}
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
	return critiqueRoundAccountingWithReads(repoRoot, state, rootJob, root, concreteGoalAdmissionReads())
}

func critiqueRoundAccountingWithReads(repoRoot string, state critiqueState, rootJob string, root map[string]any, reads goalAdmissionReads) (critiqueRoundAccount, error) {
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
		resolution, err := goalReviewRoundLimitWithReads(repoRoot, asString(root["goalId"]), uint64(max(revision, 0)), asString(root["role"]), reads)
		if err != nil || resolution.roleLimit == 0 {
			return account, fmt.Errorf("cannot resolve a positive goal review-round limit: %v", err)
		}
		account.limit = int64(resolution.roleLimit)
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
		if (status == "completed" || status == "failed" && asString(root["role"]) != "design-critic") && jobID != asString(root["unknownExaminationRetryFrom"]) && asString(record["examinationRetryOf"]) == "" {
			account.consumed++
		}
	}
	return account, nil
}

func malformedRoundAccounting(rootJob string, err error) error {
	return fmt.Errorf("the round count of critique %s is damaged: %v\ncontinuing or closing that work repairs it (work revise, design review, work review --dispositions, work finish)", rootJob, err)
}

func foldProtocolError(register []registerFinding, role, roundJob string, roundRecord map[string]any) []registerFinding {
	advanced := append([]registerFinding(nil), register...)
	id := syntheticProtocolFindingID(role, roundJob)
	for _, finding := range advanced {
		if finding.FindingID == id {
			return advanced
		}
	}
	round, _ := numInt(roundRecord["round"])
	advanced = append(advanced, registerFinding{
		FindingID: id, Critic: roundJob, RigorClass: critiqueModel.Unproven,
		PlaceholderRound: round,
		Grain:            "invariant",
		FactsDigest:      digestJSON(nil), Status: "open",
		EvidenceDigest: digestJSON(map[string]any{
			"error": roundRecord["error"], "phase": roundRecord["phase"], "protocolError": roundRecord["protocolError"],
		}), Multiplicity: 1,
	})
	return advanced
}

func supersedePlaceholders(state critiqueState, rootJob string, register []registerFinding, subject ReadSubject, round int64) error {
	if subject.Kind != SubjectCommit || subject.Commit == "" {
		return nil
	}
	for i := range register {
		f := &register[i]
		if f.Status != "open" && f.Status != "disputed" {
			continue
		}
		if f.PlaceholderRound == 0 && state.chainRoot(f.Critic) == rootJob &&
			f.FindingID == syntheticProtocolFindingID(asString(state.records[rootJob]["role"]), f.Critic) {
			f.PlaceholderRound, _ = numInt(state.records[f.Critic]["round"])
		}
		if f.PlaceholderRound < 1 || f.PlaceholderRound >= round {
			continue
		}
		prior, present, err := readsubject.ReadRoundSubject(state.agents, rootJob, f.PlaceholderRound)
		if err != nil {
			return fmt.Errorf("cannot read placeholder round %d: %w", f.PlaceholderRound, err)
		}
		if present && prior.Kind == SubjectCommit && prior.Commit == subject.Commit {
			f.Status, f.Resolution = "resolved", fmt.Sprintf("superseded by round %d", round)
		}
	}
	return nil
}

func syntheticProtocolFindingID(role, roundJob string) string {
	sum := sha256.Sum256(canonicalJSON([]any{"protocol_error", role, roundJob}))
	return "synthetic-" + hex.EncodeToString(sum[:])
}

func foldUnboundReturn(register []registerFinding, role, roundJob string, subject ReadSubject, result map[string]any) []registerFinding {
	advanced := append([]registerFinding(nil), register...)
	id := syntheticUnboundFindingID(role, roundJob)
	for _, finding := range advanced {
		if finding.FindingID == id {
			return advanced
		}
	}
	advanced = append(advanced, registerFinding{
		FindingID: id, Critic: roundJob, RigorClass: critiqueModel.Unproven,
		Grain:  "invariant",
		Status: "open", Title: "critic return is not bound to the persisted read subject",
		FactsDigest: digestJSON(nil),
		EvidenceDigest: digestJSON(map[string]any{
			"subjectDigest": subject.Digest(), "reviewedTree": result["reviewedTree"], "reviewedCommit": result["reviewedCommit"],
		}),
		Multiplicity: 1,
	})
	return advanced
}

func syntheticUnboundFindingID(role, roundJob string) string {
	return readsubject.UnboundReturnFindingID(role, roundJob)
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
	// Digest is the readsubject.AcceptedFindingDigest of the register
	// finding as shown; CritiqueRegisterAcceptRisk stamps only that content.
	Digest string
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
				// still transitions only from open, disputed or refuted below.
				if f.Status != "open" && f.Status != "disputed" && f.Status != "accepted-risk" && !f.refuted() {
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
				// Synthetic findings record why review evidence is missing. The
				// register's exact identity and digest describe that failure;
				// accepting its risk does not turn it into a successful review.
				missing := ""
				switch findingID {
				case syntheticProtocolFindingID(asString(root["role"]), f.Critic):
					missing = "the review did not return usable evidence"
				case syntheticUnboundFindingID(asString(root["role"]), f.Critic):
					missing = "the review result is not bound to the work examined"
				}
				if missing != "" {
					if claim == "" {
						claim = missing
					}
					if evidence == "" {
						evidence = fmt.Sprintf("Review %s records that %s (evidence digest %s).", f.Critic, missing, f.EvidenceDigest)
					}
				}
				title := f.Title
				if title == "" {
					title = claim
				}
				result = CritiqueDecisionFinding{FindingID: f.FindingID, Chain: rootJob, GoalID: goalID, RigorClass: string(f.RigorClass), Artifact: f.Artifact, Title: title, Claim: claim, Evidence: evidence, Facts: f.Facts,
					Digest: f.acceptanceDigest()}
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

// ErrAcceptedFindingChanged says the register finding is no longer the content
// the person was shown when they accepted its risk.
var ErrAcceptedFindingChanged = errors.New("changed since it was shown for acceptance")

// CritiqueRegisterAcceptRisk stamps a person's accepted risk on the register
// finding, bound to acceptedDigest (see CritiqueRegisterStampAcceptedRisk).
func CritiqueRegisterAcceptRisk(repoRoot, rootJob, findingID, opid, acceptedDigest string) error {
	_, err := CritiqueRegisterStampAcceptedRisk(repoRoot, rootJob, findingID, opid, acceptedDigest)
	return err
}

// CritiqueRegisterStampAcceptedRisk records a person's accepted risk on the
// register entry and says whether it changed the register: false when the
// entry already carries that decision over that content. The acceptance is
// bound to acceptedDigest, the CritiqueDecisionFinding.Digest of the snapshot
// the person decided on; a finding whose content changed since that snapshot is refused with ErrAcceptedFindingChanged and stays
// open (F4). A chain that closed before its register earned a closure records
// the clean closure the acceptance earns in the same write, also when the
// acceptance is repeated; when that closure cannot be computed the acceptance
// is still recorded and the chain stays without a closure.
func CritiqueRegisterStampAcceptedRisk(repoRoot, rootJob, findingID, opid, acceptedDigest string) (bool, error) {
	stamped := false
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
			// A predating acceptance this stamp meets is bound to the
			// finding as it stands (R-142-m1e); that alone is not a stamp.
			bound, err := bindPredatingAcceptances(root, register)
			if err != nil {
				return err
			}
			found := false
			changed := false
			for i := range register {
				if register[i].FindingID == findingID {
					found = true
					if acceptedDigest == "" || register[i].acceptanceDigest() != acceptedDigest {
						return fmt.Errorf("finding %s %w; read it again and decide anew", findingID, ErrAcceptedFindingChanged)
					}
					if register[i].Status == "accepted-risk" && register[i].DecisionOpID == opid && register[i].AcceptedDigest == acceptedDigest {
						continue
					}
					if register[i].Status != "open" && register[i].Status != "disputed" && !register[i].refuted() {
						return fmt.Errorf("finding %s is not open or disputed", findingID)
					}
					register[i].Status = "accepted-risk"
					register[i].Resolution = "accepted-risk"
					register[i].DecisionOpID = opid
					register[i].AcceptedDigest = acceptedDigest
					changed = true
				}
			}
			if !found {
				return fmt.Errorf("finding %s is absent", findingID)
			}
			// An open chain records its closure when it closes. A closure
			// that cannot be computed stays absent; the person's acceptance
			// is written all the same.
			writeClosure := false
			var closure Closure
			_, hasClosure := root[closureField]
			if closed, _ := root["chainClosed"].(bool); closed && !hasClosure {
				if computed, earned, closureErr := cleanClosure(loadCritiqueState(repoRoot), rootJob, root, register); closureErr == nil {
					closure, writeClosure = computed, earned
				}
			}
			if changed || bound || writeClosure {
				root[findingRegisterField] = encodeFindingRegister(register)
				if writeClosure {
					root[closureField] = encodeClosure(closure)
				}
				if err := writeRecord(path, root); err != nil {
					return err
				}
				stamped = changed
			}
			return nil
		})
	})
	return stamped, err
}

// CritiqueAcceptedRiskFindingIDs returns the sorted finding identifiers a
// critic chain's register holds as a person's accepted risk. A chain without
// a register holds none.
func CritiqueAcceptedRiskFindingIDs(repoRoot, rootJob string) (ids []string, err error) {
	err = withRecordLock(repoRoot, rootJob, func(path string) error {
		root, readErr := readObject(path)
		if readErr != nil {
			return fmt.Errorf("critique root record %s is unreadable: %v", rootJob, readErr)
		}
		register, _, decodeErr := critiqueFindingRegister(root)
		if decodeErr != nil {
			return fmt.Errorf("critic chain %s has a malformed finding register: %v", rootJob, decodeErr)
		}
		for _, f := range register {
			if f.acceptedRisk() {
				ids = append(ids, f.FindingID)
			}
		}
		sort.Strings(ids)
		return nil
	})
	return ids, err
}

// CritiqueRegisterResolveOutOfScope resolves the named findings as
// out-of-scope, refusing a severe or unproven one. A finding a person accepted
// as risk is already resolved by that record: its out-of-scope row changes
// nothing and is not judged against its severity.
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
				if wanted[f.FindingID] && !f.acceptedRisk() && (f.RigorClass == critiqueModel.Severe || f.RigorClass == critiqueModel.Unproven) {
					return fmt.Errorf("finding %s is %s and cannot be resolved out-of-scope", f.FindingID, f.RigorClass)
				}
			}
			changed := false
			for i := range register {
				if wanted[register[i].FindingID] {
					delete(wanted, register[i].FindingID)
					if register[i].acceptedRisk() || register[i].Status == "resolved" && register[i].Resolution == "out-of-scope" {
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
			// A material finding the fold demoted (its artifact lies outside
			// the reviewed subject) never enters the register: it blocks
			// nothing, so out-of-scope has nothing to resolve for it. Only an
			// identifier neither registered nor demoted is absent.
			demotions, _ := root["demotions"].([]any)
			for _, raw := range demotions {
				if demotion, ok := raw.(map[string]any); ok {
					delete(wanted, asString(demotion["findingId"]))
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

// CritiqueRegisterApplyDecisions carries a design author's validated decisions
// into the register before the close (g1-s66 D4): a refuted finding is
// resolved as refuted, which is not an accepted risk; an accepted finding of
// an earlier round that the follow-up examination did not raise again is
// resolved as accepted, its amendment having been read; an out-of-scope one as
// the out-of-scope resolver does, refusing a severe or unproven finding. Only
// open or disputed findings are decided here: one the critic withdrew or the
// close deferred keeps its resolution, and an id the register does not carry
// (a finding that was never material) has nothing to decide. The final round's
// own accepted findings stay open for classification, except non-severe
// acceptances folded at a one-examination design close.
func CritiqueRegisterApplyDecisions(repoRoot, rootJob string, decisions map[string]string) error {
	for id, resolution := range decisions {
		if resolution != "refuted" && resolution != "accepted" && resolution != "out-of-scope" && resolution != "folded" {
			return fmt.Errorf("finding %s: %q is not a decision the register records; it records refuted, accepted and out-of-scope", id, resolution)
		}
	}
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
			changed := false
			for i := range register {
				resolution, decided := decisions[register[i].FindingID]
				if !decided || (register[i].Status != "open" && register[i].Status != "disputed") {
					continue
				}
				if resolution == "folded" {
					round, _ := numInt(root[findingRegisterRoundField])
					limit, _ := numInt(root[reviewRoundLimitField])
					first, _ := firstDesignReturn(repoRoot, rootJob, max(limit, round))
					if asString(root["role"]) != "design-critic" || round != first || DesignRoundLimit(repoRoot, rootJob, limit) != round {
						return fmt.Errorf("finding %s can only be folded at a one-examination design close", register[i].FindingID)
					}
					if register[i].RigorClass == critiqueModel.Severe {
						continue
					}
				}
				if resolution == "out-of-scope" && (register[i].RigorClass == critiqueModel.Severe || register[i].RigorClass == critiqueModel.Unproven) {
					return fmt.Errorf("finding %s is %s and cannot be resolved out-of-scope", register[i].FindingID, register[i].RigorClass)
				}
				register[i].Status, register[i].Resolution = "resolved", resolution
				changed = true
			}
			if !changed {
				return nil
			}
			root[findingRegisterField] = encodeFindingRegister(register)
			return writeRecord(path, root)
		})
	})
	return err
}

func CritiqueRegisterClose(repoRoot, rootJob string) (string, error) {
	return critiqueRegisterClose(repoRoot, rootJob, deferReviewObligations)
}

func critiqueRegisterCloseWithReads(repoRoot, rootJob string, reads goalAdmissionReads) (string, error) {
	return critiqueRegisterClose(repoRoot, rootJob, func(repoRoot, rootJob, goalID, machine, lineage string, epoch int64, obligations []goal.ReviewObligation) (string, error) {
		return deferReviewObligationsWithReads(repoRoot, rootJob, goalID, machine, lineage, epoch, obligations, reads)
	})
}

type deferReviewObligationsFunc func(string, string, string, string, string, int64, []goal.ReviewObligation) (string, error)

func critiqueRegisterClose(repoRoot, rootJob string, deferFindings deferReviewObligationsFunc) (string, error) {
	return withFindingRegisterLock(repoRoot, func() (string, error) {
		state := loadCritiqueState(repoRoot)
		outcome := "closed"
		err := withRecordLock(repoRoot, rootJob, func(path string) error {
			root, err := readObject(path)
			if err != nil {
				return err
			}
			if asString(root["role"]) == "design-critic" && root["findingRegisterStop"] != nil {
				data, _ := json.Marshal(root["findingRegisterStop"])
				var stop loopstop.Stop
				if err := json.Unmarshal(data, &stop); err != nil || len(stop.Required) == 0 {
					return fmt.Errorf("the design stop has no readable person remedy")
				}
				return fmt.Errorf("design evidence is unknown\nrun: %s", stop.Required[0])
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
			var blockerIDs []string
			for i, f := range register {
				if f.Status == "open" || f.Status == "disputed" {
					unresolved = append(unresolved, i)
					if f.RigorClass == critiqueModel.Severe || f.RigorClass == critiqueModel.Unproven {
						blockers = append(blockers, fmt.Sprintf("finding %s (in %s) is %s and blocks close", f.FindingID, f.Artifact, f.RigorClass))
						blockerIDs = append(blockerIDs, f.FindingID)
					}
				}
				if f.Resolution == "out-of-scope" && (f.RigorClass == critiqueModel.Severe || f.RigorClass == critiqueModel.Unproven) {
					blockers = append(blockers, fmt.Sprintf("finding %s (in %s) is severe or unproven, so it cannot be out-of-scope", f.FindingID, f.Artifact))
					blockerIDs = append(blockerIDs, f.FindingID)
				}
			}
			if len(blockers) > 0 {
				foldedRound, roundOK := numInt(root[findingRegisterRoundField])
				if roundOK && designFinalRound(repoRoot, state, rootJob, root, foldedRound) {
					return designCapHumanRaise(asString(root["goalId"]), foldedRound, designCapHumanFindingIDs(root, register, unresolved, blockerIDs))
				}
				return fmt.Errorf("%s\na person accepts each risk with metasystem goal accept-risk, or raises the goal's budget with metasystem goal budget", strings.Join(blockers, "\n"))
			}
			if len(unresolved) == 0 {
				// Section 4 bullet 3 closes a clean folded round.
				return nil
			}
			foldedRound, err := findingRegisterRound(root, len(register))
			if err != nil {
				return err
			}
			designFinal := designFinalRound(repoRoot, state, rootJob, root, foldedRound)
			useFixture := false
			if designFinal {
				// Section 4 bullet 4 defers only fixture-backed mechanical findings on a falling trajectory;
				// bullet 5 sends every other non-clean row to the human without another automatic round.
				humanIDs := designCapHumanFindingIDs(root, register, unresolved, nil)
				if len(humanIDs) > 0 {
					return designCapHumanRaise(asString(root["goalId"]), foldedRound, humanIDs)
				}
				useFixture = true
			}
			if !designFinal {
				accounting, accountingErr := critiqueRoundAccounting(repoRoot, state, rootJob, root)
				if accountingErr != nil {
					return malformedRoundAccounting(rootJob, accountingErr)
				}
				if accounting.consumed < accounting.limit {
					return fmt.Errorf("review budget is not exhausted; dispatch the next round")
				}
			}
			goalID := asString(root["goalId"])
			machine := asString(root["machineId"])
			lineage := asString(root["mainId"])
			epoch, _ := numInt(root["claimEpoch"])
			if goalID == "" || machine == "" || lineage == "" || epoch < 1 {
				return fmt.Errorf("critic root has no complete owning goal pair")
			}
			obligations := make([]goal.ReviewObligation, 0, len(unresolved))
			for _, i := range unresolved {
				f := register[i]
				test := strings.Split(f.Title, "\n")[0]
				fixture := ""
				if useFixture {
					test = f.Fixture
					fixture = f.Fixture
				}
				obligations = append(obligations, goal.ReviewObligation{Finding: f.FindingID, Chain: rootJob, Artifact: f.Artifact, Test: "prove: " + test, Fixture: fixture, State: "open"})
			}
			opid, err := deferFindings(repoRoot, rootJob, goalID, machine, lineage, epoch, obligations)
			if err != nil {
				return err
			}
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

// RecordDesignEvidenceStop retains the same unknown outcome on the design
// round and its register without charging an examination or resolving findings.
func RecordDesignEvidenceStop(repoRoot, rootJob string, stop loopstop.Stop) error {
	_, err := withFindingRegisterLock(repoRoot, func() (string, error) {
		return "stopped", withRecordLock(repoRoot, rootJob, func(path string) error {
			root, err := readObject(path)
			if err != nil {
				return err
			}
			if asString(root["role"]) != "design-critic" || stop.Loop != "design-round" || stop.Subject != rootJob || stop.Decision != "stop" || !strings.HasPrefix(stop.Handoff, "stopped ") {
				return fmt.Errorf("unknown design stop must name its design critique root")
			}
			root["designStop"], root["findingRegisterStop"] = stop, stop
			return writeRecord(path, root)
		})
	})
	return err
}

func deferReviewObligations(repoRoot, rootJob, goalID, machine, lineage string, epoch int64, obligations []goal.ReviewObligation) (string, error) {
	return deferReviewObligationsWithReads(repoRoot, rootJob, goalID, machine, lineage, epoch, obligations, concreteGoalAdmissionReads())
}

func deferReviewObligationsWithReads(repoRoot, rootJob, goalID, machine, lineage string, epoch int64, obligations []goal.ReviewObligation, reads goalAdmissionReads) (string, error) {
	endpoint, err := reads.ResolveEndpoint(repoRoot)
	if err != nil {
		return "", err
	}
	req := goal.VerbRequest{Endpoint: endpoint, Actor: goal.Actor{Machine: machine, Lineage: lineage}, Ulid: deterministicULID(rootJob), Now: time.Now().UTC(), ClaimEpoch: epoch}
	result, err := goal.DeferFindings(req, goalID, obligations)
	if err != nil {
		return "", err
	}
	// A rejected publish returns no error; the register must not record a deferral the goal does not carry.
	if result.Outcome != goal.OutcomeConfirmed && result.Outcome != goal.OutcomeConfirmedLate {
		return "", fmt.Errorf("goal %s defer-findings ended %s: %s; the findings stay open", goalID, result.Outcome, result.Detail)
	}
	return goal.Opid(req.Ulid, machine, lineage), nil
}

// designFinalRound reports whether the folded design has used its allowed
// examinations within the frozen review budget.
func designFinalRound(repoRoot string, state critiqueState, rootJob string, root map[string]any, foldedRound int64) bool {
	if asString(root["role"]) != "design-critic" {
		return false
	}
	if account, err := critiqueRoundAccounting(repoRoot, state, rootJob, root); err == nil {
		return foldedRound >= DesignRoundLimit(repoRoot, rootJob, account.limit)
	}
	return foldedRound >= reviewRoundCeiling(repoRoot)
}

// reviewRoundCeiling is metasystem.budget.review-round-max for the checkout,
// or its compiled default when the configuration cannot be read.
func reviewRoundCeiling(repoRoot string) int64 {
	maximum, err := config.ReviewRoundMax(filepath.Join(repoRoot, "metasystem.conf"))
	if err != nil {
		maximum, _ = strconv.ParseUint(config.MustDefault(config.ReviewRoundMaxKey), 10, 64)
	}
	return int64(maximum)
}

func designCapHumanRaise(goalID string, round int64, findingIDs []string) error {
	unique := map[string]bool{}
	for _, id := range findingIDs {
		unique[id] = true
	}
	ids := make([]string, 0, len(unique))
	for id := range unique {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	first := "F"
	if len(ids) > 0 {
		first = ids[0]
	}
	return &OpError{Code: CritiqueCapExhaustedExitCode, Reason: CritiqueCapExhaustedReason,
		Message: fmt.Sprintf("design round %d left findings %s for a person to decide\nrun: metasystem goal accept-risk %s --finding %s --reason TEXT, or re-scope it with metasystem goal edit %s",
			round, strings.Join(ids, ", "), goalID, first, goalID)}
}

func designCapHumanFindingIDs(root map[string]any, register []registerFinding, unresolved []int, ids []string) []string {
	for _, i := range unresolved {
		f := register[i]
		if f.Grain != "mechanical" || f.Fixture == "" {
			ids = append(ids, f.FindingID)
		}
	}
	if !fallingMaterialTrajectory(root[materialByRoundField]) {
		for _, i := range unresolved {
			ids = append(ids, register[i].FindingID)
		}
	}
	return ids
}

// fallingMaterialTrajectory reports whether the last folded round's material
// count is below the round before it.
func fallingMaterialTrajectory(value any) bool {
	history, ok := value.([]any)
	if !ok {
		return false
	}
	var last int64
	var counts []int64
	for _, raw := range history {
		row, ok := raw.(map[string]any)
		round, roundOK := numInt(row["round"])
		count, countOK := numInt(row["material"])
		if !ok || len(row) != 2 || !roundOK || !countOK || round <= last || count < 0 {
			return false
		}
		last = round
		counts = append(counts, count)
	}
	return len(counts) >= 2 && counts[len(counts)-1] < counts[len(counts)-2]
}

func cleanClosure(state critiqueState, rootJob string, root map[string]any, register []registerFinding) (Closure, bool, error) {
	foldedRound, err := findingRegisterRound(root, len(register))
	if err != nil {
		return Closure{}, false, err
	}
	if foldedRound < 1 {
		return Closure{}, false, nil
	}
	// A person's accepted risk and an out-of-scope ruling close to a read
	// the landing takes; the closure records the accepted risks.
	landable, risks, err := readsubject.LandableRegister(encodeFindingRegister(register))
	if err != nil {
		return Closure{}, false, err
	}
	if !landable {
		return Closure{}, false, nil
	}
	for jobID, record := range state.records {
		round, ok := numInt(record["round"])
		if ok && round > foldedRound && state.chainRoot(jobID) == rootJob {
			return Closure{}, false, nil
		}
	}
	subject, present, err := readsubject.ReadRoundSubject(state.agents, rootJob, foldedRound)
	if err != nil {
		return Closure{}, false, err
	}
	if !present {
		return Closure{}, false, nil
	}
	want := Closure{CriticRoot: rootJob, Round: foldedRound, Subject: subject, Mechanism: "clean", AcceptedRisks: risks}
	existing, present, err := readsubject.ReadClosure(root)
	if err != nil {
		return Closure{}, false, err
	}
	if present {
		if existing.CriticRoot == want.CriticRoot && existing.Round == want.Round && existing.Mechanism == want.Mechanism && existing.Subject.Equal(want.Subject) && readsubject.RecordedAcceptedRisksHold(existing.AcceptedRisks, want.AcceptedRisks) {
			return want, false, nil
		}
		return Closure{}, false, fmt.Errorf("critique %s was already closed at round %d, so it is not closed again at round %d (subjects %s, %s)", rootJob, existing.Round, want.Round, existing.Subject.Digest(), want.Subject.Digest())
	}
	roundJob, roundRecord, err := critiqueRecordForRound(state, rootJob, foldedRound)
	if err != nil {
		return Closure{}, false, err
	}
	if asString(roundRecord["status"]) != "completed" {
		return Closure{}, false, nil
	}
	resultPath := filepath.Join(state.agents, rootJob, "rounds", fmt.Sprint(foldedRound), "return.json")
	result, err := readObject(resultPath)
	if err != nil {
		return Closure{}, false, fmt.Errorf("critique return for job %s is unreadable while closing: %v", roundJob, err)
	}
	returnedRound, roundOK := numInt(result["round"])
	if asString(result["jobId"]) != roundJob || !roundOK || returnedRound != foldedRound {
		return Closure{}, false, nil
	}
	// A round whose return names other work closes on its persisted subject
	// only when a person accepted the round's unbound-return finding.
	if !readsubject.ReturnBindsSubject(subject, result) && !readsubject.AcceptsUnboundReturn(risks, asString(root["role"]), roundJob) {
		return Closure{}, false, nil
	}
	foldedSubjectDigest := asString(root[findingRegisterSubjectDigestField])
	if foldedSubjectDigest == "" || subject.Digest() != foldedSubjectDigest {
		return Closure{}, false, fmt.Errorf("critic root %s round %d closure subject %s is not the folded subject %s", rootJob, foldedRound, subject.Digest(), foldedSubjectDigest)
	}
	return want, true, nil
}

type missingCritiqueRoundRecordError struct {
	rootJob string
	round   int64
}

func (e *missingCritiqueRoundRecordError) Error() string {
	return fmt.Sprintf("critic root %s has no record for folded round %d", e.rootJob, e.round)
}

func critiqueRecordForRound(state critiqueState, rootJob string, round int64) (string, map[string]any, error) {
	var roundJob string
	var roundRecord map[string]any
	for jobID, record := range state.records {
		candidateRound, ok := numInt(record["round"])
		if !ok || candidateRound != round || state.chainRoot(jobID) != rootJob {
			continue
		}
		if roundRecord != nil {
			return "", nil, fmt.Errorf("critic root %s carries multiple records for folded round %d", rootJob, round)
		}
		roundJob, roundRecord = jobID, record
	}
	if roundRecord == nil {
		return "", nil, &missingCritiqueRoundRecordError{rootJob: rootJob, round: round}
	}
	return roundJob, roundRecord, nil
}

func encodeClosure(closure Closure) map[string]any {
	data, _ := json.Marshal(closure)
	var encoded map[string]any
	_ = json.Unmarshal(data, &encoded)
	return encoded
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

// CritiqueChainBudgetRebind carries the goal's current review-round limit
// onto every open critic register a review of rootJob continues: rootJob
// itself when it is a critic root, and when it is an implementer root each
// code-critic or warden root whose reviewed job belongs to its chain (the
// roots CritiqueExhaustionAdvance inspects). It returns each root's outcome;
// any other role has none. A root whose goal is no longer a live claimed
// goal keeps its stored limit ("kept"): only a claimed goal's budget can have
// been raised for the work under review. Every other failure is returned.
func CritiqueChainBudgetRebind(repoRoot, rootJob string) (map[string]string, error) {
	return critiqueChainBudgetRebindWithReads(repoRoot, rootJob, concreteGoalAdmissionReads())
}

func critiqueChainBudgetRebindWithReads(repoRoot, rootJob string, reads goalAdmissionReads) (map[string]string, error) {
	state := loadCritiqueState(repoRoot)
	root, present := state.records[rootJob]
	if !present {
		return nil, fmt.Errorf("critique root record %s is unreadable", rootJob)
	}
	var roots []string
	switch asString(root["role"]) {
	case "design-critic", "code-critic", "warden":
		roots = []string{rootJob}
	case "implementer":
		implementation := map[string]bool{}
		for id := range state.records {
			if state.chainRoot(id) == rootJob {
				implementation[id] = true
			}
		}
		for id, record := range state.records {
			role := asString(record["role"])
			if (role == "code-critic" || role == "warden") && record["parentJob"] == nil && implementation[asString(record["reviews"])] {
				roots = append(roots, id)
			}
		}
		sort.Strings(roots)
	}
	outcomes := map[string]string{}
	for _, critic := range roots {
		if closed, _ := state.records[critic]["chainClosed"].(bool); closed {
			continue
		}
		if goalID := asString(state.records[critic]["goalId"]); goalID != "" {
			if _, _, err := resolveGoalRevisionWithReads(repoRoot, goalID, reads); err != nil {
				outcomes[critic] = "kept"
				continue
			}
		}
		outcome, err := critiqueBudgetRebindWithReads(repoRoot, critic, reads)
		if err != nil {
			return outcomes, fmt.Errorf("critique root %s: %w", critic, err)
		}
		outcomes[critic] = outcome
	}
	return outcomes, nil
}

func critiqueBudgetRebindWithReads(repoRoot, rootJob string, reads goalAdmissionReads) (outcome string, err error) {
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
			var revision uint64
			var tier uint8
			if goalID != "" {
				revision, tier, e = resolveGoalRevisionWithReads(repoRoot, goalID, reads)
				if e != nil {
					return e
				}
			}
			resolution, e := goalReviewRoundLimitWithReads(repoRoot, goalID, revision, role, reads)
			if e != nil {
				return fmt.Errorf("cannot resolve a positive goal review-round limit: %v", e)
			}
			limit := resolution.rebindLimit()
			if limit == 0 {
				return fmt.Errorf("cannot resolve a positive goal review-round limit: resolved limit is zero")
			}
			opid := fmt.Sprintf("critique-budget-rebind-%s-r%d", rootJob, revision)
			storedLimit, limitPresent := numInt(root[reviewRoundLimitField])
			_, consumedPresent := root[criticRoundsConsumedField]
			if binding, ok := root["critiqueBudgetBinding"].(map[string]any); ok && asString(binding["opid"]) == opid && limitPresent && storedLimit == int64(limit) && consumedPresent {
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
			root["critiqueBudgetBinding"] = map[string]any{"goalId": goalID, "goalRevision": revision, "goalTier": tier, "opid": opid}
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
	wait := recordLockWait()
	if !flockWithin(handle, wait, recordLockClock{}) {
		return "", fmt.Errorf("finding-register lock is busy after %s", wait)
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
		fieldCount := len(entry)
		for _, extra := range []string{"class", "where", "change", "resolves", "relation", "transferStop"} {
			if _, present := entry[extra]; present {
				fieldCount--
			}
		}
		var placeholderRound int64
		if value, marked := entry["placeholderRound"]; marked {
			var roundOK bool
			placeholderRound, roundOK = numInt(value)
			if !roundOK || placeholderRound < 1 {
				return nil, fmt.Errorf("entry %d has an invalid placeholder round", index)
			}
			fieldCount--
		}
		if !ok || (fieldCount != 7 && fieldCount != 13 && fieldCount != 14 && fieldCount != 15 && fieldCount != 16) {
			return nil, fmt.Errorf("entry %d is not an object with the canonical fields", index)
		}
		// The sixteenth field binds an accepted risk to the content accepted.
		acceptedDigest, acceptedDigestPresent := entry["acceptedDigest"]
		if (fieldCount == 16) != acceptedDigestPresent {
			return nil, fmt.Errorf("entry %d is not an object with the canonical fields", index)
		}
		if acceptedDigestPresent {
			if digest, isString := acceptedDigest.(string); !isString || !hexDigest64.MatchString(digest) || asString(entry["status"]) != "accepted-risk" {
				return nil, fmt.Errorf("entry %d records accepted content outside an accepted risk", index)
			}
		}
		finding := registerFinding{
			FindingID:      asString(entry["findingId"]),
			Critic:         asString(entry["critic"]),
			RigorClass:     critiqueModel.RigorClass(asString(entry["rigorClass"])),
			Grain:          "invariant",
			FactsDigest:    asString(entry["factsDigest"]),
			Facts:          entry["facts"],
			Artifact:       asString(entry["artifact"]),
			Title:          asString(entry["title"]),
			Status:         asString(entry["status"]),
			Resolution:     asString(entry["resolution"]),
			DecisionOpID:   asString(entry["decisionOpid"]),
			Evidence:       asString(entry["evidence"]),
			EvidenceDigest: asString(entry["evidenceDigest"]),
			AcceptedDigest: asString(acceptedDigest),
		}
		finding.Class, finding.Where, finding.Change = asString(entry["class"]), asString(entry["where"]), asString(entry["change"])
		finding.Resolves, finding.Relation, finding.TransferStop = asString(entry["resolves"]), asString(entry["relation"]), asString(entry["transferStop"])
		finding.PlaceholderRound = placeholderRound
		if fieldCount >= 14 {
			finding.Grain = asString(entry["grain"])
		}
		if fieldCount >= 15 {
			finding.Fixture = asString(entry["fixture"])
		}
		if fieldCount == 7 && finding.Status == "resolved" {
			finding.Resolution = "withdrawn"
		}
		finding.Multiplicity, ok = numInt(entry["multiplicity"])
		if finding.FindingID == "" || finding.Critic == "" || !finding.RigorClass.Valid() || (finding.Grain != "mechanical" && finding.Grain != "invariant") ||
			!hexDigest64.MatchString(finding.FactsDigest) || !hexDigest64.MatchString(finding.EvidenceDigest) ||
			!ok || finding.Multiplicity < 1 ||
			(finding.Status != "open" && finding.Status != "resolved" && finding.Status != "disputed" && finding.Status != "deferred" && finding.Status != "accepted-risk" && finding.Status != "transferred") {
			return nil, fmt.Errorf("entry %d has invalid canonical values", index)
		}
		if fieldCount >= 13 {
			unresolved := finding.Status == "open" || finding.Status == "disputed"
			if unresolved && (finding.Resolution != "" || finding.DecisionOpID != "") {
				return nil, fmt.Errorf("entry %d carries a resolution while unresolved", index)
			}
			if !unresolved && finding.Resolution == "" {
				return nil, fmt.Errorf("entry %d is non-open without a resolution", index)
			}
			validResolution := finding.Status == "transferred" && finding.Resolution == "transferred" && finding.TransferStop != "" || finding.Status == "resolved" && (finding.Resolution == "withdrawn" || finding.Resolution == "out-of-scope" ||
				finding.Resolution == "refuted" || finding.Resolution == "accepted" || finding.Resolution == "folded" || readsubject.SupersededPlaceholder(entry)) ||
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
		grain := finding.Grain
		// An unset or unrecognized grain is invariant, never mechanical, because the close table reads this field.
		if grain != "mechanical" && grain != "invariant" {
			grain = "invariant"
		}
		entry := map[string]any{
			"findingId": finding.FindingID, "critic": finding.Critic,
			"rigorClass": string(finding.RigorClass), "grain": grain, "fixture": finding.Fixture, "factsDigest": finding.FactsDigest,
			"status": finding.Status, "evidenceDigest": finding.EvidenceDigest,
			"multiplicity": finding.Multiplicity, "facts": finding.Facts,
			"artifact": finding.Artifact, "title": finding.Title,
			"resolution": finding.Resolution, "decisionOpid": finding.DecisionOpID,
			"evidence": finding.Evidence,
		}
		if finding.Status == "accepted-risk" && finding.AcceptedDigest != "" {
			entry["acceptedDigest"] = finding.AcceptedDigest
		}
		if finding.PlaceholderRound > 0 {
			entry["placeholderRound"] = finding.PlaceholderRound
		}
		if finding.Class != "" {
			entry["class"], entry["where"], entry["change"], entry["resolves"], entry["relation"] = finding.Class, finding.Where, finding.Change, finding.Resolves, finding.Relation
		}
		if finding.TransferStop != "" {
			entry["transferStop"] = finding.TransferStop
		}
		items[index] = entry
	}
	return items
}

type critiqueSubject struct {
	paths          map[string]bool
	repoRoot, tree string
	legacy         bool
	facts          critiqueSubjectFacts
}

// CritiqueSubjectFacts supplies the immutable repository facts used to bind
// a critic's artifacts to its reviewed subject.
type CritiqueSubjectFacts interface {
	ChangedPaths(root, commit string) ([]string, error)
	CommitTree(root, commit string) (string, error)
	InstallPrefix(root string) (string, error)
	ArtifactAbsent(root, tree, path string) bool
}

type critiqueSubjectFacts = CritiqueSubjectFacts

type gitCritiqueSubjectFacts struct{}

func (gitCritiqueSubjectFacts) ChangedPaths(root, commit string) ([]string, error) {
	output, err := exec.Command("git", "-C", root, "diff-tree", "-r", "--no-commit-id", "--name-only", commit+"^", commit).Output()
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, path := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		path = filepath.ToSlash(strings.TrimSpace(path))
		if path != "" {
			paths = append(paths, path)
		}
	}
	return paths, nil
}

func (gitCritiqueSubjectFacts) CommitTree(root, commit string) (string, error) {
	tree, err := exec.Command("git", "-C", root, "rev-parse", commit+"^{tree}").Output()
	return strings.TrimSpace(string(tree)), err
}

func (gitCritiqueSubjectFacts) InstallPrefix(root string) (string, error) {
	return projectInstallPrefix(root)
}

func (gitCritiqueSubjectFacts) ArtifactAbsent(root, tree, path string) bool {
	return artifactAbsentFromTree(root, tree, path)
}

func critiqueSubjectForRoundWithFacts(repoRoot string, state critiqueState, root map[string]any, role string, result map[string]any, facts critiqueSubjectFacts) (critiqueSubject, error) {
	s := critiqueSubject{paths: map[string]bool{}, repoRoot: repoRoot, facts: facts}
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
	if role == "code-critic" && validCommitReview.MatchString(reviewedJob) {
		commit := strings.TrimPrefix(reviewedJob, "commit:")
		paths, err := facts.ChangedPaths(repoRoot, commit)
		if err != nil {
			return s, fmt.Errorf("commit subject %s is not a readable non-root commit: %v", reviewedJob, err)
		}
		for _, path := range paths {
			if path != "" {
				s.paths[path] = true
			}
		}
		if len(s.paths) == 0 {
			return s, fmt.Errorf("commit subject %s has no changed paths", reviewedJob)
		}
		tree, err := facts.CommitTree(repoRoot, commit)
		if err != nil {
			return s, fmt.Errorf("commit subject %s has no readable tree: %v", reviewedJob, err)
		}
		s.tree = tree
		return s, nil
	}
	reviewed, ok := state.records[reviewedJob]
	if !ok || asString(reviewed["role"]) != "implementer" {
		return s, fmt.Errorf("critic root does not name a reviewed implementer round")
	}
	round, ok := numInt(reviewed["round"])
	if !ok || round < 1 {
		return s, fmt.Errorf("reviewed implementer round is malformed")
	}
	diffPath := filepath.Join(state.agents, state.chainRoot(reviewedJob), "rounds", fmt.Sprint(round), "diff.patch")
	relativeDiffPath, relativeErr := filepath.Rel(repoRoot, diffPath)
	if relativeErr != nil {
		relativeDiffPath = diffPath
	}
	relativeDiffPath = filepath.ToSlash(relativeDiffPath)
	data, err := os.ReadFile(diffPath)
	if err != nil {
		return s, fmt.Errorf("round %d (%s) has no recorded changes at %s\nfirst run validate conformance --stage review --job %s", round, reviewedJob, relativeDiffPath, reviewedJob)
	}
	installPrefix, err := facts.InstallPrefix(repoRoot)
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
		return s, fmt.Errorf("round %d (%s) changes no file in %s\nfirst run validate conformance --stage review --job %s", round, reviewedJob, relativeDiffPath, reviewedJob)
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
		if s.tree == "" || s.facts == nil || !s.facts.ArtifactAbsent(s.repoRoot, s.tree, ref.Path) {
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
	advanced, demotions, _, err := foldCritiqueFindingsVersioned(register, role, roundJob, findings, rigorValue, 0, subject, round)
	return advanced, demotions, err
}

func foldCritiqueFindingsVersioned(register []registerFinding, role, roundJob string, findings []any, rigorValue any, schemaVersion int64, subject critiqueSubject, round int64) ([]registerFinding, []any, int64, error) {
	advanced := append([]registerFinding(nil), register...)
	var demotions []any
	var admitted int64
	byID := map[string]int{}
	for index, finding := range advanced {
		byID[finding.FindingID] = index
	}
	rigorRows := rigorRowsByID(rigorValue)
	identities := findingIdentities(role, findings)
	processed := map[string]bool{}
	admittedIDs := map[string]bool{}
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
		if resolvedID := asString(finding["resolves"]); resolvedID != "" {
			priorIndex, present := byID[resolvedID]
			if !present || advanced[priorIndex].Class != asString(finding["class"]) || advanced[priorIndex].Where != asString(finding["where"]) {
				return nil, nil, 0, fmt.Errorf("finding %s resolves unknown or different prior evidence %s", id, resolvedID)
			}
			if !material {
				advanced[priorIndex].Status, advanced[priorIndex].Resolution = "resolved", "withdrawn"
			}
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
		admittedID := asString(finding["id"])
		if admittedID == "" {
			admittedID = id
		}
		admittedIDs[admittedID] = true
		admitted = int64(len(admittedIDs))

		factsDigest := digestJSON(row["facts"])
		recurring := false
		for _, prior := range advanced {
			if prior.Status == "deferred" && prior.Artifact == artifact && prior.FactsDigest == factsDigest {
				recurring = true
				break
			}
		}
		class := critiqueModel.NormalizeWire(row["rigorClass"], row["facts"], row["reopeningTrigger"], recurring)
		grain := "invariant"
		if schemaVersion == 5 && asString(row["grain"]) == "mechanical" {
			grain = "mechanical"
		}
		fixture := ""
		if schemaVersion == 5 {
			fixture = asString(row["fixture"])
		}
		title := strings.TrimSpace(strings.Split(strings.ReplaceAll(asString(finding["claim"]), "\r\n", "\n"), "\n")[0])
		candidate := registerFinding{
			FindingID: id, Critic: roundJob, RigorClass: class,
			Class: asString(finding["class"]), Where: asString(finding["where"]), Change: asString(finding["change"]), Resolves: asString(finding["resolves"]), Relation: asString(finding["relation"]),
			Grain:       grain,
			Fixture:     fixture,
			FactsDigest: factsDigest, Facts: row["facts"], Artifact: artifact, Title: title, Status: "open",
			Evidence: asString(finding["evidence"]), EvidenceDigest: digestJSON(finding["evidence"]), Multiplicity: identity.Multiplicity,
		}
		if !exists {
			byID[id] = len(advanced)
			advanced = append(advanced, candidate)
			continue
		}
		current := advanced[existingIndex]
		// The close table reads grain, so a re-report may strengthen it to invariant but never weaken it.
		if current.Grain == "invariant" {
			candidate.Grain = current.Grain
		}
		advanced[existingIndex].Grain = candidate.Grain
		advanced[existingIndex].Fixture = candidate.Fixture
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
			// A lower-rigor re-report disputes only a finding still
			// unresolved. A decided entry (resolved, deferred, or a person's
			// accepted risk) keeps its decision and the content it was
			// decided on; a higher rigor reopens it, and so does an
			// equal-rigor re-report of an accepted risk whose content
			// changed (reopenChangedAcceptances, F4).
			if current.Status == "open" {
				advanced[existingIndex].Status = "disputed"
			}
			continue
		}
		if rigorRank(class) > rigorRank(current.RigorClass) {
			candidate.Critic = current.Critic
			candidate.Multiplicity = current.Multiplicity
			advanced[existingIndex] = candidate
			continue
		}
		if current.Status == "accepted-risk" {
			// An equal-rigor re-report of an accepted risk carries what the
			// critic now reports, so reopenChangedAcceptances can compare it
			// with the content the person accepted (F4).
			candidate.Critic = current.Critic
			candidate.Multiplicity = current.Multiplicity
			candidate.Status, candidate.Resolution = current.Status, current.Resolution
			candidate.DecisionOpID, candidate.AcceptedDigest = current.DecisionOpID, current.AcceptedDigest
			advanced[existingIndex] = candidate
		}
	}
	return advanced, demotions, admitted, nil
}

func appendMaterialRound(value any, round, material int64, cancelled bool) ([]any, error) {
	history, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("materialByRound is not an array")
	}
	var last int64
	for index, raw := range history {
		row, rowOK := raw.(map[string]any)
		rowRound, roundOK := numInt(row["round"])
		rowMaterial, materialOK := numInt(row["material"])
		if !rowOK || len(row) != 2 || !roundOK || !materialOK || rowRound <= last || rowMaterial < 0 {
			return nil, fmt.Errorf("materialByRound entry %d is not canonical", index)
		}
		last = rowRound
	}
	result := append([]any(nil), history...)
	if cancelled {
		return result, nil
	}
	if round <= last {
		return nil, fmt.Errorf("materialByRound cannot append round %d after round %d", round, last)
	}
	return append(result, map[string]any{"round": round, "material": material}), nil
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
				return fmt.Errorf("finding %s is rated %s here and %s elsewhere; the critic or a person settles it\n(critiques %s and %s, subjects %s and %s)",
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

// refuted is a finding its author refuted at a close that the critic has not
// withdrawn: not landable by itself, and still open to a person's accepted
// risk, which overrides the critic's standing finding (R-142).
func (f registerFinding) refuted() bool {
	return f.Status == "resolved" && f.Resolution == "refuted"
}

// acceptedRisk reports whether a person's recorded decision resolved f.
func (f registerFinding) acceptedRisk() bool {
	return f.Status == "accepted-risk" && f.DecisionOpID != ""
}
