package readsubject

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
)

const closureField = "closure"

type Closure struct {
	CriticRoot string      `json:"criticRoot"`
	Round      int64       `json:"round"`
	Subject    ReadSubject `json:"subject"`
	Mechanism  string      `json:"mechanism"`
	// AcceptedRisks names each finding of the closed register that a
	// person's goal accept-risk decided, so the read records what it lands
	// over. Absent on a register with no accepted risk.
	AcceptedRisks []AcceptedRisk `json:"acceptedRisks,omitempty"`
}

// AcceptedRisk is one register finding a person accepted as a risk, with the
// decision operation that recorded the act and the AcceptedFindingDigest of
// the content the person accepted.
type AcceptedRisk struct {
	FindingID      string `json:"findingId"`
	DecisionOpID   string `json:"decisionOpid"`
	AcceptedDigest string `json:"acceptedDigest"`
}

// AcceptedFindingDigest is the canonical digest of what a person accepts as a
// risk: the register finding's identity, rigor class, artifact, title, facts
// and evidence digests. An acceptance covers exactly this digest; a finding
// whose digest has changed is not accepted.
//
// Content only, not the reviewed subject (Wido 2026-10-03): "You are asked
// again only when the problem changes. A rebase with the same finding keeps
// your acceptance." A re-review of another tree that reports the same
// finding keeps the acceptance.
func AcceptedFindingDigest(entry map[string]any) string {
	content := map[string]string{}
	for _, field := range []string{"findingId", "rigorClass", "artifact", "title", "factsDigest", "evidenceDigest"} {
		content[field] = stringValue(entry[field])
	}
	data, _ := json.Marshal(content)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// SameAcceptedRisks says whether two closures name the same accepted risks.
func SameAcceptedRisks(a, b []AcceptedRisk) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// RecordedAcceptedRisksHold says whether the accepted risks a closure recorded
// are the ones its register holds now. A closure written before content
// binding names no digest; its acceptance is never overruled (R-142-m1e), so
// it holds while the finding and decision operation are the same.
func RecordedAcceptedRisksHold(recorded, register []AcceptedRisk) bool {
	if len(recorded) != len(register) {
		return false
	}
	for i := range recorded {
		predating := recorded[i].AcceptedDigest == "" && recorded[i].FindingID == register[i].FindingID && recorded[i].DecisionOpID == register[i].DecisionOpID
		if recorded[i] != register[i] && !predating {
			return false
		}
	}
	return true
}

func ReadClosure(root map[string]any) (Closure, bool, error) {
	value, present := root[closureField]
	if !present {
		return Closure{}, false, nil
	}
	object, ok := value.(map[string]any)
	if !ok {
		return Closure{}, true, fmt.Errorf("closure must be an object")
	}

	criticRoot, ok := object["criticRoot"].(string)
	if !ok || !validJobID.MatchString(criticRoot) {
		return Closure{}, true, fmt.Errorf("closure criticRoot %q is not a valid job identifier", criticRoot)
	}
	round, ok := strictInteger(object["round"])
	if !ok || round < 1 {
		return Closure{}, true, fmt.Errorf("closure round must be a positive integer")
	}
	subject, subjectPresent, err := DecodeReadSubject(object["subject"])
	if err != nil {
		return Closure{}, true, fmt.Errorf("closure subject: %w", err)
	}
	if !subjectPresent {
		return Closure{}, true, fmt.Errorf("closure subject is missing")
	}
	mechanism, ok := object["mechanism"].(string)
	if !ok || mechanism != "clean" {
		return Closure{}, true, fmt.Errorf("closure mechanism %q is unknown", mechanism)
	}

	var risks []AcceptedRisk
	if value, present := object["acceptedRisks"]; present {
		items, ok := value.([]any)
		if !ok || len(items) == 0 {
			return Closure{}, true, fmt.Errorf("closure acceptedRisks must be a non-empty array when present")
		}
		for index, item := range items {
			entry, ok := item.(map[string]any)
			finding, findingOK := entry["findingId"].(string)
			opid, opidOK := entry["decisionOpid"].(string)
			if !ok || len(entry) < 2 || len(entry) > 3 || !findingOK || finding == "" || !opidOK || opid == "" {
				return Closure{}, true, fmt.Errorf("closure accepted risk %d must name a finding and its decision operation", index)
			}
			// A closure written before acceptances were bound to their
			// content names no digest (RecordedAcceptedRisksHold).
			digest, digestOK := entry["acceptedDigest"].(string)
			if len(entry) == 3 && (!digestOK || digest == "") {
				return Closure{}, true, fmt.Errorf("closure accepted risk %d must record which content was accepted", index)
			}
			risks = append(risks, AcceptedRisk{FindingID: finding, DecisionOpID: opid, AcceptedDigest: digest})
		}
	}

	return Closure{CriticRoot: criticRoot, Round: round, Subject: subject, Mechanism: mechanism, AcceptedRisks: risks}, true, nil
}

// CleanRegister says whether a finding register is clean: empty, or every
// entry resolved as withdrawn or folded. It is the clean read of D3/D7.
func CleanRegister(value any) (bool, error) {
	clean, _, _, err := classifyRegister(value)
	return clean, err
}

// LandableRegister says whether a closed register yields the read a landing
// takes: every entry is withdrawn, folded, ruled out-of-scope (never a severe or
// unproven finding), or accepted as a risk by a person's recorded act whose
// accepted digest is the entry's AcceptedFindingDigest (an acceptance that
// predates content binding has none and covers the finding as it stands). It
// returns the accepted risks the read must record. Open, disputed, deferred,
// refuted or accepted (a fix required) entries, and an acceptance of other
// content, are not landable.
func LandableRegister(value any) (bool, []AcceptedRisk, error) {
	_, landable, risks, err := classifyRegister(value)
	return landable, risks, err
}

func classifyRegister(value any) (bool, bool, []AcceptedRisk, error) {
	items, ok := value.([]any)
	if !ok {
		return false, false, nil, fmt.Errorf("finding register must be an array")
	}
	clean, landable := true, true
	var risks []AcceptedRisk
	for index, raw := range items {
		entry, ok := raw.(map[string]any)
		if !ok {
			return false, false, nil, fmt.Errorf("finding register entry %d is not an object with the canonical fields", index)
		}
		legacy := hasExactFields(entry,
			"findingId", "critic", "rigorClass", "factsDigest", "status", "evidenceDigest", "multiplicity")
		modern := hasExactFields(entry,
			"findingId", "critic", "rigorClass", "factsDigest", "facts", "artifact", "title",
			"status", "resolution", "decisionOpid", "evidence", "evidenceDigest", "multiplicity")
		withGrain := hasExactFields(entry,
			"findingId", "critic", "rigorClass", "grain", "factsDigest", "facts", "artifact", "title",
			"status", "resolution", "decisionOpid", "evidence", "evidenceDigest", "multiplicity")
		withFixture := hasExactFields(entry,
			"findingId", "critic", "rigorClass", "grain", "fixture", "factsDigest", "facts", "artifact", "title",
			"status", "resolution", "decisionOpid", "evidence", "evidenceDigest", "multiplicity")
		withAcceptance := hasExactFields(entry,
			"findingId", "critic", "rigorClass", "grain", "fixture", "factsDigest", "facts", "artifact", "title",
			"status", "resolution", "decisionOpid", "evidence", "evidenceDigest", "multiplicity", "acceptedDigest")
		if !legacy && !modern && !withGrain && !withFixture && !withAcceptance {
			return false, false, nil, fmt.Errorf("finding register entry %d is not an object with the canonical fields", index)
		}
		status, ok := entry["status"].(string)
		if !ok {
			return false, false, nil, fmt.Errorf("finding register entry %d has no string status", index)
		}
		acceptedDigest, digestOK := entry["acceptedDigest"].(string)
		if withAcceptance && (status != "accepted-risk" || !digestOK || acceptedDigest == "") {
			return false, false, nil, fmt.Errorf("finding register entry %d records accepted content outside an accepted risk", index)
		}
		resolution := ""
		if legacy {
			if status == "resolved" {
				resolution = "withdrawn"
			}
		} else {
			var resolutionOK bool
			resolution, resolutionOK = entry["resolution"].(string)
			if !resolutionOK {
				return false, false, nil, fmt.Errorf("finding register entry %d has no string resolution", index)
			}
		}

		pairIsClean := status == "resolved" && (resolution == "withdrawn" || resolution == "folded")
		pairIsNonClean := (status == "open" || status == "disputed") && resolution == "" ||
			status == "resolved" && (resolution == "out-of-scope" || resolution == "refuted" || resolution == "accepted") ||
			status == "deferred" && resolution == "deferred" ||
			status == "accepted-risk" && resolution == "accepted-risk"
		if !pairIsClean && !pairIsNonClean {
			return false, false, nil, fmt.Errorf("finding register entry %d has status/resolution mismatch %q/%q", index, status, resolution)
		}
		clean = clean && pairIsClean
		class, _ := entry["rigorClass"].(string)
		opid, _ := entry["decisionOpid"].(string)
		finding, _ := entry["findingId"].(string)
		switch {
		case pairIsClean:
		case status == "resolved" && resolution == "out-of-scope" && class != "severe" && class != "unproven":
		case status == "accepted-risk" && opid != "" && !withAcceptance:
			// An acceptance recorded before content binding carries no
			// digest. A person's recorded decision is never overruled
			// (R-142-m1e): it covers the finding as it stands, and the
			// first fold that meets it binds that content.
			risks = append(risks, AcceptedRisk{FindingID: finding, DecisionOpID: opid, AcceptedDigest: AcceptedFindingDigest(entry)})
		case status == "accepted-risk" && opid != "" && acceptedDigest == AcceptedFindingDigest(entry):
			// Only a person's goal accept-risk records this pair, with
			// the decision operation that carried the act, and it covers
			// only the finding content the person saw.
			risks = append(risks, AcceptedRisk{FindingID: finding, DecisionOpID: opid, AcceptedDigest: acceptedDigest})
		default:
			landable = false
		}
	}
	if !landable {
		risks = nil
	}
	return clean, landable, risks, nil
}

// UnboundReturnFindingID is the identifier of the register finding a fold
// files for a round of roundJob, in a chain of role, whose return does not
// bind the round's persisted subject.
func UnboundReturnFindingID(role, roundJob string) string {
	data, _ := json.Marshal([]any{"unbound_return", role, roundJob})
	sum := sha256.Sum256(data)
	return "synthetic-" + hex.EncodeToString(sum[:])
}

// AcceptsUnboundReturn says whether accepted risks name the unbound-return
// finding of roundJob's round: a person accepted that the round's return
// names other work, so the read closes on the round's persisted subject.
func AcceptsUnboundReturn(risks []AcceptedRisk, role, roundJob string) bool {
	id := UnboundReturnFindingID(role, roundJob)
	for _, risk := range risks {
		if risk.FindingID == id {
			return true
		}
	}
	return false
}

func ReturnBindsSubject(subject ReadSubject, result map[string]any) bool {
	switch subject.Kind {
	case SubjectLive:
		return stringValue(result["reviewedTree"]) == subject.ReviewedProjectTree
	case SubjectCommit:
		return stringValue(result["reviewedTree"]) == subject.Tree
	case SubjectDesign:
		return stringValue(result["reviewedCommit"]) == subject.ReviewedCommit
	default:
		return false
	}
}

func ReadClosedClosure(agents string, root map[string]any, members []map[string]any) (Closure, bool, error) {
	closure, present, err := ReadClosure(root)
	if err != nil {
		return Closure{}, present, err
	}
	if !present {
		return readClosureAbsence(agents, root)
	}

	rootID, ok := root["jobId"].(string)
	if !ok || !validJobID.MatchString(rootID) || rootID != closure.CriticRoot {
		return closure, true, fmt.Errorf("closure critic root %q does not match supplied root job %q", closure.CriticRoot, rootID)
	}
	role, _ := root["role"].(string)
	if role != "code-critic" && role != "design-critic" && role != "warden" {
		return closure, true, fmt.Errorf("closure root %s is not a critic root", rootID)
	}
	if parent, parentPresent := root["parentJob"]; parentPresent && parent != nil {
		return closure, true, fmt.Errorf("closure root %s names parent job %q", rootID, stringValue(parent))
	}
	rootRound, ok := strictInteger(root["round"])
	if !ok || rootRound != 1 {
		return closure, true, fmt.Errorf("closure root %s must be round 1", rootID)
	}
	rootStatus, _ := root["status"].(string)
	if !terminalStatus(rootStatus) {
		return closure, true, fmt.Errorf("closure root %s is not terminal", rootID)
	}
	closed, ok := root["chainClosed"].(bool)
	if !ok || !closed {
		return closure, true, fmt.Errorf("closure root %s is not closed", rootID)
	}

	highestRound := int64(0)
	highestStatus := ""
	selectedJob := ""
	rootSeen := false
	seenJobs := make(map[string]bool, len(members))
	seenRounds := make(map[int64]bool, len(members))
	for index, member := range members {
		jobID, ok := member["jobId"].(string)
		if !ok || !validJobID.MatchString(jobID) {
			return closure, true, fmt.Errorf("closure member %d has invalid job identifier %q", index, jobID)
		}
		if seenJobs[jobID] {
			return closure, true, fmt.Errorf("closure membership repeats job %s", jobID)
		}
		seenJobs[jobID] = true
		round, ok := strictInteger(member["round"])
		if !ok || round < 1 {
			return closure, true, fmt.Errorf("closure member %s has invalid round", jobID)
		}
		if seenRounds[round] {
			return closure, true, fmt.Errorf("closure membership has more than one member at round %d", round)
		}
		seenRounds[round] = true
		if parent, parentPresent := member["parentJob"]; parentPresent && parent != nil {
			parentID, parentOK := parent.(string)
			if !parentOK || !validJobID.MatchString(parentID) {
				return closure, true, fmt.Errorf("closure member %s has invalid parent job", jobID)
			}
		}
		status, _ := member["status"].(string)
		if !terminalStatus(status) {
			return closure, true, fmt.Errorf("closure member %s at round %d is not terminal", jobID, round)
		}
		if jobID == rootID {
			if round != 1 {
				return closure, true, fmt.Errorf("closure root member %s is not round 1", rootID)
			}
			rootSeen = true
		}
		if round > highestRound {
			highestRound = round
			highestStatus = status
			selectedJob = jobID
		}
	}
	if !rootSeen {
		return closure, true, fmt.Errorf("closure membership does not include root %s", rootID)
	}
	if highestStatus != "completed" {
		return closure, true, fmt.Errorf("closure last member %s at round %d is %s, not completed", selectedJob, highestRound, highestStatus)
	}
	if closure.Round != highestRound {
		return closure, true, fmt.Errorf("closure round %d is not the last critic round %d", closure.Round, highestRound)
	}

	registerValue, registerPresent := root["findingRegister"]
	if !registerPresent {
		return closure, true, fmt.Errorf("closure root %s has no finding register", rootID)
	}
	landable, risks, err := LandableRegister(registerValue)
	if err != nil {
		return closure, true, fmt.Errorf("closure root %s has malformed finding register: %w", rootID, err)
	}
	if !landable {
		return closure, true, fmt.Errorf("closure root %s does not have a clean finding register", rootID)
	}
	if !RecordedAcceptedRisksHold(closure.AcceptedRisks, risks) {
		return closure, true, fmt.Errorf("closure root %s records accepted risks %v, but its register holds %v", rootID, closure.AcceptedRisks, risks)
	}
	foldedRound, ok := strictInteger(root["findingRegisterRound"])
	if !ok || foldedRound < 1 {
		return closure, true, fmt.Errorf("closure root %s has invalid folded round", rootID)
	}
	if closure.Round != foldedRound {
		return closure, true, fmt.Errorf("closure round %d is not folded round %d", closure.Round, foldedRound)
	}

	persisted, subjectPresent, err := ReadRoundSubject(agents, rootID, closure.Round)
	if err != nil {
		return closure, true, fmt.Errorf("closure round %d subject is unreadable: %w", closure.Round, err)
	}
	if !subjectPresent {
		return closure, true, fmt.Errorf("closure round %d has no persisted subject", closure.Round)
	}
	if !persisted.Equal(closure.Subject) {
		return closure, true, fmt.Errorf("closure subject %s does not equal persisted subject %s", closure.Subject.Digest(), persisted.Digest())
	}
	foldedDigest, ok := root["findingRegisterSubjectDigest"].(string)
	if !ok || foldedDigest == "" {
		return closure, true, fmt.Errorf("closure root %s names no folded subject", rootID)
	}
	if persisted.Digest() != foldedDigest {
		return closure, true, fmt.Errorf("persisted subject %s does not equal folded subject %s", persisted.Digest(), foldedDigest)
	}

	result, err := readObject(roundFile(agents, rootID, closure.Round, "return.json"))
	if err != nil {
		return closure, true, fmt.Errorf("closure return for round %d is unreadable: %w", closure.Round, err)
	}
	returnedRound, roundOK := strictInteger(result["round"])
	if stringValue(result["jobId"]) != selectedJob || !roundOK || returnedRound != closure.Round {
		return closure, true, fmt.Errorf("closure return does not name selected member %s at round %d", selectedJob, closure.Round)
	}
	if !ReturnBindsSubject(persisted, result) && !AcceptsUnboundReturn(closure.AcceptedRisks, role, selectedJob) {
		return closure, true, fmt.Errorf("closure return for %s does not bind persisted subject %s", selectedJob, persisted.Digest())
	}
	return closure, true, nil
}

func readClosureAbsence(agents string, root map[string]any) (Closure, bool, error) {
	registerValue, registerPresent := root["findingRegister"]
	if !registerPresent {
		if _, digestPresent := root["findingRegisterSubjectDigest"]; digestPresent {
			return Closure{}, false, fmt.Errorf("closure is absent while a folded subject has no finding register")
		}
		return Closure{}, false, nil
	}
	clean, err := CleanRegister(registerValue)
	if err != nil {
		return Closure{}, false, fmt.Errorf("closure is absent and the finding register is malformed: %w", err)
	}

	foldedValue, foldedPresent := root["findingRegisterRound"]
	if !foldedPresent {
		if _, digestPresent := root["findingRegisterSubjectDigest"]; digestPresent {
			return Closure{}, false, fmt.Errorf("closure is absent while a folded subject has no folded round")
		}
		return Closure{}, false, nil
	}
	foldedRound, ok := strictInteger(foldedValue)
	if !ok || foldedRound < 0 {
		return Closure{}, false, fmt.Errorf("closure is absent and the folded round is invalid")
	}
	digestValue, digestPresent := root["findingRegisterSubjectDigest"]
	foldedDigest, digestIsString := digestValue.(string)
	if digestPresent && (!digestIsString || foldedDigest == "") {
		return Closure{}, false, fmt.Errorf("closure is absent and the folded subject's checksum is invalid")
	}
	if !clean && !digestPresent {
		return Closure{}, false, nil
	}
	if foldedRound == 0 {
		if digestPresent {
			return Closure{}, false, fmt.Errorf("closure is absent while round zero names a folded subject")
		}
		return Closure{}, false, nil
	}

	rootID, ok := root["jobId"].(string)
	if !ok || !validJobID.MatchString(rootID) {
		return Closure{}, false, fmt.Errorf("closure is absent and folded subject root %q is invalid", rootID)
	}
	_, subjectPresent, err := ReadRoundSubject(agents, rootID, foldedRound)
	if err != nil {
		return Closure{}, false, fmt.Errorf("closure is absent and folded subject is unreadable: %w", err)
	}
	if !subjectPresent {
		if digestPresent {
			return Closure{}, false, fmt.Errorf("closure is absent and recorded folded subject %s is lost", foldedDigest)
		}
		return Closure{}, false, nil
	}
	if !clean {
		return Closure{}, false, nil
	}
	return Closure{}, false, fmt.Errorf("completed clean subject-bearing fold at round %d has no closure", foldedRound)
}

func strictInteger(value any) (int64, bool) {
	switch typed := value.(type) {
	case json.Number:
		integer, err := typed.Int64()
		return integer, err == nil
	case float64:
		if typed == float64(int64(typed)) {
			return int64(typed), true
		}
	case int64:
		return typed, true
	case int:
		return int64(typed), true
	}
	return 0, false
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}

func terminalStatus(status string) bool {
	switch status {
	case "completed", "failed", "cancelled", "timeout":
		return true
	default:
		return false
	}
}

func hasExactFields(value map[string]any, fields ...string) bool {
	if len(value) != len(fields) {
		return false
	}
	for _, field := range fields {
		if _, present := value[field]; !present {
			return false
		}
	}
	return true
}

func roundFile(agents, root string, round int64, name string) string {
	return filepath.Join(agents, root, "rounds", strconv.FormatInt(round, 10), name)
}
