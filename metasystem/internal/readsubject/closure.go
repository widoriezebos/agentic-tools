package readsubject

import (
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

	return Closure{CriticRoot: criticRoot, Round: round, Subject: subject, Mechanism: mechanism}, true, nil
}

func CleanRegister(value any) (bool, error) {
	items, ok := value.([]any)
	if !ok {
		return false, fmt.Errorf("finding register must be an array")
	}
	clean := true
	for index, raw := range items {
		entry, ok := raw.(map[string]any)
		if !ok {
			return false, fmt.Errorf("finding register entry %d is not an object with the canonical fields", index)
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
		if !legacy && !modern && !withGrain && !withFixture {
			return false, fmt.Errorf("finding register entry %d is not an object with the canonical fields", index)
		}
		status, ok := entry["status"].(string)
		if !ok {
			return false, fmt.Errorf("finding register entry %d has no string status", index)
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
				return false, fmt.Errorf("finding register entry %d has no string resolution", index)
			}
		}

		pairIsClean := status == "resolved" && resolution == "withdrawn"
		pairIsNonClean := (status == "open" || status == "disputed") && resolution == "" ||
			status == "resolved" && resolution == "out-of-scope" ||
			status == "deferred" && resolution == "deferred" ||
			status == "accepted-risk" && resolution == "accepted-risk"
		if !pairIsClean && !pairIsNonClean {
			return false, fmt.Errorf("finding register entry %d has status/resolution mismatch %q/%q", index, status, resolution)
		}
		clean = clean && pairIsClean
	}
	return clean, nil
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
	clean, err := CleanRegister(registerValue)
	if err != nil {
		return closure, true, fmt.Errorf("closure root %s has malformed finding register: %w", rootID, err)
	}
	if !clean {
		return closure, true, fmt.Errorf("closure root %s does not have a clean finding register", rootID)
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
		return closure, true, fmt.Errorf("closure root %s has no folded subject digest", rootID)
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
	if !ReturnBindsSubject(persisted, result) {
		return closure, true, fmt.Errorf("closure return for %s does not bind persisted subject %s", selectedJob, persisted.Digest())
	}
	return closure, true, nil
}

func readClosureAbsence(agents string, root map[string]any) (Closure, bool, error) {
	registerValue, registerPresent := root["findingRegister"]
	if !registerPresent {
		if _, digestPresent := root["findingRegisterSubjectDigest"]; digestPresent {
			return Closure{}, false, fmt.Errorf("closure is absent while a folded subject digest has no finding register")
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
			return Closure{}, false, fmt.Errorf("closure is absent while a folded subject digest has no folded round")
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
		return Closure{}, false, fmt.Errorf("closure is absent and the folded subject digest is invalid")
	}
	if !clean && !digestPresent {
		return Closure{}, false, nil
	}
	if foldedRound == 0 {
		if digestPresent {
			return Closure{}, false, fmt.Errorf("closure is absent while round zero carries a folded subject digest")
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
