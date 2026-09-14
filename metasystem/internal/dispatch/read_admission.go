package dispatch

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
)

const cleanReadRoundsField = "cleanReadRounds"

const (
	readAdmittedDecision  = "ADMITTED"
	redundantReadRefusal  = "REDUNDANT_READ"
	concurrentReadRefusal = "CONCURRENT_READ"
)

type CleanReadRound struct {
	Round   int64                   `json:"round"`
	Subject readsubject.ReadSubject `json:"subject"`
}

type ReadAdmissionResult struct {
	Decision          string `json:"decision"`
	CriticRoot        string `json:"criticRoot,omitempty"`
	Round             int64  `json:"round,omitempty"`
	SubjectDigest     string `json:"subjectDigest"`
	EventID           string `json:"eventId,omitempty"`
	EventRecorded     bool   `json:"eventRecorded"`
	EventDurable      bool   `json:"eventDurable"`
	LatestCriticRound int64  `json:"latestCriticRound,omitempty"`
}

type cleanReadCandidate struct {
	root        string
	read        CleanReadRound
	latestRound int64
	closeable   bool
	closedLive  bool
}

// CritiqueReadAdmission decides whether a computed subject still needs a
// critic. It serializes the evidence read with folds and refusal appends, but
// releases that lock before returning so callers can mirror safely.
func CritiqueReadAdmission(repoRoot, role, rootJob string, round int64, subject readsubject.ReadSubject) (result ReadAdmissionResult, err error) {
	result = ReadAdmissionResult{SubjectDigest: subject.Digest()}
	if !validJobID.MatchString(rootJob) {
		return result, fmt.Errorf("critic root %q is not a valid job identifier", rootJob)
	}
	if round < 1 {
		return result, fmt.Errorf("critic read round must be a positive integer")
	}
	if err := validateReadSubjectForRole(role, subject); err != nil {
		return result, err
	}
	if subject.Kind == readsubject.SubjectCommit {
		result.Decision = readAdmittedDecision
		return result, nil
	}

	_, err = withFindingRegisterLock(repoRoot, func() (string, error) {
		state := loadCritiqueState(repoRoot)
		if requestingRoot, present := state.records[rootJob]; present {
			if state.chainRoot(rootJob) != rootJob {
				return "", fmt.Errorf("requesting critic root %s is not a chain root", rootJob)
			}
			if recordedRole := asString(requestingRoot["role"]); recordedRole != role {
				return "", fmt.Errorf("requesting critic root %s has role %q, not %q", rootJob, recordedRole, role)
			}
			if !readRootInSubjectScope(state, requestingRoot, subject) {
				return "", fmt.Errorf("requesting critic root %s does not name the candidate subject scope", rootJob)
			}
		}

		roots := scopedCriticRoots(state, role, subject)
		var redundant []cleanReadCandidate
		for _, criticRoot := range roots {
			root := state.records[criticRoot]
			if criticRoot != rootJob &&
				!rootMayHaveEqualCleanRead(root, subject) &&
				(subject.Kind != readsubject.SubjectLive || !rootHasEqualPersistedLiveSubject(state, criticRoot, subject)) {
				continue
			}
			reads, readErr := cleanReadsForRoot(state, criticRoot, root)
			if readErr != nil {
				return "", readErr
			}
			var latestEqual *CleanReadRound
			for i := range reads {
				read := &reads[i]
				if !read.Subject.Equal(subject) {
					continue
				}
				if latestEqual == nil || read.Round > latestEqual.Round {
					latestEqual = read
				}
			}
			if latestEqual != nil {
				latest, _, latestErr := highestCriticMember(state, criticRoot, role)
				if latestErr != nil {
					return "", latestErr
				}
				closeable, closedLive := cleanReadCloseState(repoRoot, state, criticRoot, root, latestEqual.Round, latest)
				redundant = append(redundant, cleanReadCandidate{
					root: criticRoot, read: *latestEqual, latestRound: latest,
					closeable: closeable, closedLive: closedLive,
				})
			}
		}
		if len(redundant) > 0 {
			sort.Slice(redundant, func(i, j int) bool {
				if redundant[i].closeable != redundant[j].closeable {
					return redundant[i].closeable
				}
				if redundant[i].root != redundant[j].root {
					return redundant[i].root < redundant[j].root
				}
				return redundant[i].read.Round > redundant[j].read.Round
			})
			prior := redundant[0]
			result.Decision = redundantReadRefusal
			result.CriticRoot = prior.root
			result.Round = prior.read.Round
			result.LatestCriticRound = prior.latestRound

			nonce := make([]byte, 12)
			if _, randomErr := rand.Read(nonce); randomErr != nil {
				return "", redundantReadError(result, prior, fmt.Sprintf("cannot allocate refusal event id: %v", randomErr))
			}
			result.EventID = prior.root + "-" + strconv.FormatInt(prior.read.Round, 10) + "-" + hex.EncodeToString(nonce)
			event := ReadRefusal{
				ID: result.EventID, Reason: redundantReadRefusal, Role: role,
				CriticRoot: prior.root, Round: prior.read.Round, Subject: subject,
				RefusedAt: time.Now().UTC().Format(time.RFC3339Nano),
			}
			path := filepath.Join(state.agents, prior.root, "reads-refused.jsonl")
			durable, appendErr := appendReadRefusalLocked(path, repoRoot, event)
			if appendErr != nil {
				return "", redundantReadError(result, prior, fmt.Sprintf("refusal event was not recorded: %v", appendErr))
			}
			result.EventRecorded = true
			result.EventDurable = durable
			detail := ""
			if !durable {
				detail = "the refusal event is published, but its crash durability is not proven; mirror the prior root to repair durable evidence"
			}
			return "", redundantReadError(result, prior, detail)
		}

		if subject.Kind == readsubject.SubjectLive {
			for _, criticRoot := range roots {
				if criticRoot == rootJob {
					continue
				}
				if !rootHasEqualPersistedLiveSubject(state, criticRoot, subject) {
					continue
				}
				root := state.records[criticRoot]
				latestRound, latest, latestErr := highestCriticMember(state, criticRoot, role)
				if latestErr != nil {
					return "", latestErr
				}
				register, _, registerErr := critiqueFindingRegister(root)
				if registerErr != nil {
					return "", fmt.Errorf("critic root %s has a malformed finding register: %v", criticRoot, registerErr)
				}
				foldedRound, foldedErr := findingRegisterRound(root, len(register))
				if foldedErr != nil {
					return "", fmt.Errorf("critic root %s has malformed register round state: %v", criticRoot, foldedErr)
				}
				if latestRound <= foldedRound || asString(latest["status"]) == "cancelled" {
					continue
				}
				persisted, present, subjectErr := readsubject.ReadRoundSubject(state.agents, criticRoot, latestRound)
				if subjectErr != nil {
					return "", fmt.Errorf("critic root %s round %d has a malformed live subject: %v", criticRoot, latestRound, subjectErr)
				}
				if !present {
					continue
				}
				if validateErr := validateReadSubjectForRole(role, persisted); validateErr != nil {
					return "", fmt.Errorf("critic root %s round %d has a malformed live subject: %v", criticRoot, latestRound, validateErr)
				}
				if !persisted.Equal(subject) {
					continue
				}
				result.Decision = concurrentReadRefusal
				result.CriticRoot = criticRoot
				result.Round = latestRound
				result.LatestCriticRound = latestRound
				return "", &OpError{
					Code:    11,
					Reason:  concurrentReadRefusal,
					Message: fmt.Sprintf("CONCURRENT_READ: critic root %s has outstanding round %d for subject %s; let that read finish and fold, or cancel it with dispatch.sh cancel --job %s, then dispatch the critic again", criticRoot, latestRound, result.SubjectDigest, asString(latest["jobId"])),
				}
			}
		}
		return "", nil
	})
	if err == nil {
		result.Decision = readAdmittedDecision
	}
	return result, err
}

func redundantReadError(result ReadAdmissionResult, prior cleanReadCandidate, detail string) error {
	next := fmt.Sprintf("next: dispatch.sh close --job %s", prior.root)
	if prior.closedLive {
		next = fmt.Sprintf("next: dispatch.sh close --job %s --reconcile-evidence %s; completion still checks terminal coverage and required evidence", prior.read.Subject.ImplementerRoot, prior.root)
	} else if prior.latestRound > prior.read.Round {
		next = fmt.Sprintf("critic root %s now has later round %d, so clean read round %d cannot close that newer state; resolve and fold the later work before dispatching another equal read", prior.root, prior.latestRound, prior.read.Round)
	} else if !prior.closeable {
		next = fmt.Sprintf("critic root %s cannot presently close from read round %d; inspect its current closure and fold evidence before dispatching another equal read", prior.root, prior.read.Round)
	}
	message := fmt.Sprintf("REDUNDANT_READ: critic root %s already proved a clean critic read at round %d for subject %s; %s", prior.root, prior.read.Round, result.SubjectDigest, next)
	if detail != "" {
		message += "; " + detail
	}
	return &OpError{
		Code:    11,
		Reason:  redundantReadRefusal,
		Message: message,
	}
}

func scopedCriticRoots(state critiqueState, role string, subject readsubject.ReadSubject) []string {
	var roots []string
	for jobID, record := range state.records {
		if state.chainRoot(jobID) != jobID || asString(record["role"]) != role || !readRootInSubjectScope(state, record, subject) {
			continue
		}
		roots = append(roots, jobID)
	}
	sort.Strings(roots)
	return roots
}

func readRootInSubjectScope(state critiqueState, root map[string]any, subject readsubject.ReadSubject) bool {
	switch subject.Kind {
	case readsubject.SubjectLive:
		reviewed := asString(root["reviews"])
		return reviewed != "" && state.chainRoot(reviewed) == subject.ImplementerRoot
	case readsubject.SubjectDesign:
		return asString(root["design"]) == subject.DesignPath
	default:
		return false
	}
}

// cleanReadsForRoot returns only observations whose immutable subject and
// return evidence still agree. The history field is authoritative for older
// folds; only an absent field permits deriving the current folded round.
func cleanReadsForRoot(state critiqueState, rootJob string, root map[string]any) ([]CleanReadRound, error) {
	if asString(root["jobId"]) != rootJob || !validJobID.MatchString(rootJob) {
		return nil, fmt.Errorf("clean read root %s has inconsistent identity", rootJob)
	}
	role := asString(root["role"])
	if role != "design-critic" && role != "code-critic" && role != "warden" {
		return nil, fmt.Errorf("clean read root %s has non-critic role %q", rootJob, role)
	}
	if parent, present := root["parentJob"]; present && parent != nil {
		return nil, fmt.Errorf("clean read root %s names a parent job", rootJob)
	}
	if rootRound, ok := numInt(root["round"]); !ok || rootRound != 1 {
		return nil, fmt.Errorf("clean read root %s is not round one", rootJob)
	}
	register, registerPresent, err := critiqueFindingRegister(root)
	if err != nil {
		return nil, fmt.Errorf("clean read root %s has a malformed finding register: %v", rootJob, err)
	}
	historyValue, historyPresent := root[cleanReadRoundsField]
	if !registerPresent {
		if historyPresent {
			return nil, fmt.Errorf("clean read root %s has history without a finding register", rootJob)
		}
		return nil, nil
	}
	foldedRound, err := findingRegisterRound(root, len(register))
	if err != nil {
		return nil, fmt.Errorf("clean read root %s has malformed register round state: %v", rootJob, err)
	}
	if historyPresent {
		items, ok := historyValue.([]any)
		if !ok {
			return nil, fmt.Errorf("clean read history on root %s must be an ordered array", rootJob)
		}
		reads := make([]CleanReadRound, 0, len(items))
		lastRound := int64(0)
		for index, raw := range items {
			entry, ok := raw.(map[string]any)
			if !ok || len(entry) != 2 {
				return nil, fmt.Errorf("clean read history entry %d on root %s must contain only round and subject", index, rootJob)
			}
			entryRound, ok := numInt(entry["round"])
			if !ok || entryRound < 1 || entryRound > foldedRound {
				return nil, fmt.Errorf("clean read history entry %d on root %s has round outside folded bound %d", index, rootJob, foldedRound)
			}
			if entryRound <= lastRound {
				return nil, fmt.Errorf("clean read history on root %s has duplicate, conflicting, or unordered round %d", rootJob, entryRound)
			}
			subject, present, decodeErr := readsubject.DecodeReadSubject(entry["subject"])
			if decodeErr != nil || !present {
				return nil, fmt.Errorf("clean read history entry %d on root %s has a malformed subject: %v", index, rootJob, decodeErr)
			}
			if validateErr := validateReadSubjectForRole(role, subject); validateErr != nil {
				return nil, fmt.Errorf("clean read history entry %d on root %s: %v", index, rootJob, validateErr)
			}
			proven, evidenceErr := validateCleanReadEvidence(state, rootJob, role, entryRound, subject)
			if evidenceErr != nil {
				return nil, evidenceErr
			}
			if !proven {
				lastRound = entryRound
				continue
			}
			if entryRound == foldedRound {
				current, currentErr := validateCurrentCleanFold(rootJob, root, subject)
				if currentErr != nil {
					return nil, currentErr
				}
				if !current {
					lastRound = entryRound
					continue
				}
			}
			reads = append(reads, CleanReadRound{Round: entryRound, Subject: subject})
			lastRound = entryRound
		}
		return reads, nil
	}

	if foldedRound == 0 {
		return nil, nil
	}
	clean, err := readsubject.CleanRegister(root[findingRegisterField])
	if err != nil {
		return nil, fmt.Errorf("clean read root %s has a malformed finding register: %v", rootJob, err)
	}
	if !clean {
		return nil, nil
	}
	digest := asString(root[findingRegisterSubjectDigestField])
	if digest == "" {
		return nil, nil
	}
	persisted, present, err := readsubject.ReadRoundSubject(state.agents, rootJob, foldedRound)
	if err != nil {
		return nil, fmt.Errorf("clean read root %s round %d subject is malformed: %v", rootJob, foldedRound, err)
	}
	if !present {
		return nil, nil
	}
	if validateErr := validateReadSubjectForRole(role, persisted); validateErr != nil {
		return nil, fmt.Errorf("clean read root %s round %d: %v", rootJob, foldedRound, validateErr)
	}
	if persisted.Digest() != digest {
		return nil, nil
	}
	proven, err := validateCleanReadEvidence(state, rootJob, role, foldedRound, persisted)
	if err != nil {
		return nil, err
	}
	if !proven {
		return nil, nil
	}
	return []CleanReadRound{{Round: foldedRound, Subject: persisted}}, nil
}

func validateCurrentCleanFold(rootJob string, root map[string]any, subject readsubject.ReadSubject) (bool, error) {
	clean, err := readsubject.CleanRegister(root[findingRegisterField])
	if err != nil {
		return false, fmt.Errorf("clean read root %s has a malformed current register: %v", rootJob, err)
	}
	if !clean {
		return false, fmt.Errorf("clean read root %s claims its non-clean current fold as a clean observation", rootJob)
	}
	digest := asString(root[findingRegisterSubjectDigestField])
	if digest == "" || digest != subject.Digest() {
		return false, nil
	}
	return true, nil
}

func validateCleanReadEvidence(state critiqueState, rootJob, role string, round int64, subject readsubject.ReadSubject) (bool, error) {
	roundJob, record, err := critiqueRecordForRound(state, rootJob, round)
	if err != nil {
		if _, missing := err.(*missingCritiqueRoundRecordError); missing {
			return false, nil
		}
		return false, fmt.Errorf("clean read root %s round %d has invalid member evidence: %v", rootJob, round, err)
	}
	if asString(record["role"]) != role {
		return false, fmt.Errorf("clean read root %s round %d member %s has role %q, not %q", rootJob, round, roundJob, asString(record["role"]), role)
	}
	status := asString(record["status"])
	if !validCriticReadStatus(status) {
		return false, fmt.Errorf("clean read root %s round %d member %s has invalid status %q", rootJob, round, roundJob, status)
	}
	if status != "completed" {
		return false, nil
	}
	persisted, present, err := readsubject.ReadRoundSubject(state.agents, rootJob, round)
	if err != nil {
		return false, fmt.Errorf("clean read root %s round %d subject is malformed: %v", rootJob, round, err)
	}
	if !present {
		return false, nil
	}
	if !persisted.Equal(subject) {
		return false, fmt.Errorf("clean read root %s round %d does not match its persisted subject", rootJob, round)
	}
	resultPath := filepath.Join(state.agents, rootJob, "rounds", strconv.FormatInt(round, 10), "return.json")
	result, err := readObject(resultPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("clean read root %s round %d return is unreadable: %v", rootJob, round, err)
	}
	returnedRound, ok := numInt(result["round"])
	if asString(result["jobId"]) != roundJob || !ok || returnedRound != round {
		return false, fmt.Errorf("clean read root %s round %d return coordinates do not name member %s", rootJob, round, roundJob)
	}
	if !readsubject.ReturnBindsSubject(persisted, result) {
		return false, fmt.Errorf("clean read root %s round %d return does not bind persisted subject %s", rootJob, round, persisted.Digest())
	}
	return true, nil
}

// rootMayHaveEqualCleanRead only selects roots whose clean-read state can
// affect this subject. cleanReadsForRoot remains the authority that validates
// the selected root and its supporting evidence.
func rootMayHaveEqualCleanRead(root map[string]any, subject readsubject.ReadSubject) bool {
	if asString(root[findingRegisterSubjectDigestField]) == subject.Digest() {
		return true
	}
	items, ok := root[cleanReadRoundsField].([]any)
	if !ok {
		return false
	}
	for _, raw := range items {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		observed, present, err := readsubject.DecodeReadSubject(entry["subject"])
		if err == nil && present && observed.Equal(subject) {
			return true
		}
	}
	return false
}

func rootHasEqualPersistedLiveSubject(state critiqueState, rootJob string, subject readsubject.ReadSubject) bool {
	paths, _ := filepath.Glob(filepath.Join(state.agents, rootJob, "rounds", "*", "subject.json"))
	for _, path := range paths {
		round, err := strconv.ParseInt(filepath.Base(filepath.Dir(path)), 10, 64)
		if err != nil || round < 1 {
			continue
		}
		persisted, present, readErr := readsubject.ReadRoundSubject(state.agents, rootJob, round)
		if readErr != nil {
			return true
		}
		if present && persisted.Equal(subject) {
			return true
		}
	}
	return false
}

func highestCriticMember(state critiqueState, rootJob, role string) (int64, map[string]any, error) {
	var latest map[string]any
	latestRound := int64(0)
	seenRounds := map[int64]string{}
	for jobID, record := range state.records {
		if state.chainRoot(jobID) != rootJob {
			continue
		}
		round, ok := numInt(record["round"])
		if !ok || round < 1 {
			return 0, nil, fmt.Errorf("critic root %s has member %s with malformed round state", rootJob, jobID)
		}
		if prior := seenRounds[round]; prior != "" {
			return 0, nil, fmt.Errorf("critic root %s has tied round %d on members %s and %s", rootJob, round, prior, jobID)
		}
		seenRounds[round] = jobID
		if asString(record["role"]) != role {
			return 0, nil, fmt.Errorf("critic root %s has member %s with role %q, not %q", rootJob, jobID, asString(record["role"]), role)
		}
		if !validCriticReadStatus(asString(record["status"])) {
			return 0, nil, fmt.Errorf("critic root %s has member %s with invalid status %q", rootJob, jobID, asString(record["status"]))
		}
		if latest == nil || round > latestRound {
			latestRound, latest = round, record
		}
	}
	if latest == nil {
		return 0, nil, fmt.Errorf("critic root %s has no readable members", rootJob)
	}
	return latestRound, latest, nil
}

func validCriticReadStatus(status string) bool {
	switch status {
	case "pending-setup", "pending", "running", "completed", "failed", "cancelled", "timeout":
		return true
	default:
		return false
	}
}

func cleanReadCloseState(repoRoot string, state critiqueState, rootJob string, root map[string]any, readRound, latestRound int64) (closeable, closedLive bool) {
	if readRound != latestRound {
		return false, false
	}
	register, present, err := critiqueFindingRegister(root)
	if err != nil || !present {
		return false, false
	}
	foldedRound, err := findingRegisterRound(root, len(register))
	if err != nil || foldedRound != readRound {
		return false, false
	}
	clean, err := readsubject.CleanRegister(root[findingRegisterField])
	if err != nil || !clean {
		return false, false
	}
	if _, closurePresent := root[closureField]; !closurePresent {
		closed, _ := root["chainClosed"].(bool)
		return !closed && CloseCheck(repoRoot, rootJob) == nil, false
	}
	members := make([]map[string]any, 0)
	for jobID, record := range state.records {
		if state.chainRoot(jobID) == rootJob {
			members = append(members, record)
		}
	}
	closure, present, err := readsubject.ReadClosedClosure(state.agents, root, members)
	valid := present && err == nil
	return valid, valid && closure.Subject.Kind == readsubject.SubjectLive
}

func appendCleanRead(rounds []CleanReadRound, next CleanReadRound) ([]CleanReadRound, error) {
	for _, existing := range rounds {
		if existing.Round != next.Round {
			continue
		}
		if existing.Subject.Equal(next.Subject) {
			return rounds, nil
		}
		return nil, fmt.Errorf("clean read round %d conflicts with its existing subject", next.Round)
	}
	rounds = append(rounds, next)
	sort.Slice(rounds, func(i, j int) bool { return rounds[i].Round < rounds[j].Round })
	return rounds, nil
}

func encodeCleanReadRounds(rounds []CleanReadRound) []any {
	encoded := make([]any, len(rounds))
	for i, read := range rounds {
		encoded[i] = map[string]any{"round": read.Round, "subject": encodeReadSubject(read.Subject)}
	}
	return encoded
}

func encodeReadSubject(subject readsubject.ReadSubject) map[string]any {
	data, _ := json.Marshal(subject)
	var encoded map[string]any
	_ = json.Unmarshal(data, &encoded)
	return encoded
}

func validateReadSubjectForRole(role string, subject readsubject.ReadSubject) error {
	nonempty := func(value string) bool { return value != "" && strings.TrimSpace(value) == value }
	switch subject.Kind {
	case readsubject.SubjectLive:
		if role != "code-critic" && role != "warden" {
			return fmt.Errorf("role %s cannot read a live critique subject", role)
		}
		if !validJobID.MatchString(subject.ImplementerRoot) ||
			(subject.ReviewedMember != "" && !validJobID.MatchString(subject.ReviewedMember)) ||
			!nonempty(subject.ReviewedProjectTree) || !nonempty(subject.DiffDigest) {
			return fmt.Errorf("live critique subject is malformed")
		}
	case readsubject.SubjectDesign:
		cleanPath := filepath.ToSlash(filepath.Clean(subject.DesignPath))
		if role != "design-critic" {
			return fmt.Errorf("role %s cannot read a design critique subject", role)
		}
		if cleanPath != subject.DesignPath || !strings.HasPrefix(cleanPath, "metasystem/") ||
			!nonempty(subject.ContentDigest) || !nonempty(subject.DeclaredOutputsDigest) ||
			(subject.ReviewedCommit != "" && !nonempty(subject.ReviewedCommit)) {
			return fmt.Errorf("design critique subject is malformed")
		}
	case readsubject.SubjectCommit:
		if role != "code-critic" {
			return fmt.Errorf("role %s cannot read a commit critique subject", role)
		}
		if !nonempty(subject.Commit) || (subject.Parent != "" && !nonempty(subject.Parent)) || !nonempty(subject.Tree) || !nonempty(subject.DiffDigest) {
			return fmt.Errorf("commit critique subject is malformed")
		}
	default:
		return fmt.Errorf("critique subject kind %q is unknown", subject.Kind)
	}
	return nil
}
