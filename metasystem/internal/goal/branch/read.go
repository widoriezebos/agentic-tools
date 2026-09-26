package branch

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"golang.org/x/sys/unix"
)

const ReadDispatchPendingCode = "GOAL_READ_DISPATCH_PENDING"

type BranchReadRequest struct {
	Repo, Remote, EndpointTip, BranchTip, GoalID, UnitCommit string
	BriefPath, Runtime, Model                                string
	Collect                                                  bool
	// Retry names a failed examination round of the recorded critic chain
	// to examine once more; FollowUp starts that round in the same chain.
	Retry      int64
	FollowUp   func(rootJob, brief string) (string, error)
	CheckClaim func() error
	Gate       func(string) (string, error)
	Delegate   func(string, string, string, string, string) (string, error)
	Commit     func(CommitReadRequest) (string, Attestation, error)
	NewID      func(string) (string, error)
	Repository BranchReadRepository
}

type BranchReadResult struct {
	State, RootJob, GateRunID, AttestationCommit string
	// Published is set by InspectBranchRead: the collected attestation is
	// contained in the goal branch's origin tip the push owner last
	// recorded, so the remote holds it as far as this checkout knows.
	Published bool
	// Retry is the examination round a retry admitted, or rejoined.
	Retry string
}

// ReadNeverLaunchedError is returned only when the delegate boundary proves
// that no critic process was started. Other launch failures remain uncertain.
type ReadNeverLaunchedError struct{ Err error }

func (e *ReadNeverLaunchedError) Error() string { return e.Err.Error() }
func (e *ReadNeverLaunchedError) Unwrap() error { return e.Err }

type ReadGateRequest struct {
	Repo, GoalID, UnitCommit string
	Gate                     func(string) (string, error)
	NewID                    func(string) (string, error)
	Repository               BranchReadRepository
}

type branchReadRecord struct {
	SchemaVersion     int    `json:"schemaVersion"`
	Goal              string `json:"goal"`
	UnitCommit        string `json:"unitCommit"`
	Tree              string `json:"tree"`
	GateRunID         string `json:"gateRunId,omitempty"`
	RootJob           string `json:"rootJob,omitempty"`
	Brief             string `json:"brief,omitempty"`
	BriefInputSHA256  string `json:"briefInputSha256,omitempty"`
	Runtime           string `json:"runtime,omitempty"`
	Model             string `json:"model,omitempty"`
	DispatchPending   bool   `json:"dispatchPending,omitempty"`
	DispatchRetryable bool   `json:"dispatchRetryable,omitempty"`
	FrozenBriefSHA256 string `json:"frozenBriefSha256,omitempty"`
	// Retries maps a failed examination round to the round its retry
	// admitted ("pending" before the follow-up reported it).
	Retries           map[string]string `json:"retries,omitempty"`
	AttestationCommit string            `json:"attestationCommit,omitempty"`
}

func gitCommonDir(repo string) (string, error) {
	out, err := gitOutput(repo, "rev-parse", "--git-common-dir")
	if err != nil {
		return "", err
	}
	path := strings.TrimSpace(string(out))
	if !filepath.IsAbs(path) {
		path = filepath.Join(repo, path)
	}
	return filepath.Clean(path), nil
}

func branchReadPathsWithRepository(repository BranchReadRepository, repo, goal, commit string) (common, record, brief string, err error) {
	common, err = repository.CommonDir(repo)
	if err != nil {
		return "", "", "", err
	}
	dir := filepath.Join(common, "metasystem", "goal-reads", goal)
	return common, filepath.Join(dir, commit+".json"), filepath.Join(dir, commit+".md"), nil
}

func loadBranchReadRecord(path string) (branchReadRecord, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return branchReadRecord{SchemaVersion: 1}, nil
	}
	if err != nil {
		return branchReadRecord{}, err
	}
	var record branchReadRecord
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&record); err != nil || record.SchemaVersion != 1 {
		return branchReadRecord{}, fmt.Errorf("goal branch read record is malformed")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return branchReadRecord{}, fmt.Errorf("goal branch read record is malformed")
	}
	return record, nil
}

func saveBranchReadRecord(common, path string, record branchReadRecord) error {
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	durable, err := atomicfile.WriteText(path, string(append(data, '\n')), common)
	if err != nil {
		return err
	}
	if !durable {
		return fmt.Errorf("%s: read record publication is visible but durability is unknown", ReadDispatchPendingCode)
	}
	return nil
}

func lockBranchRead(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	lock, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(lock.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		lock.Close()
		return nil, operationRefusal(ReadDispatchPendingCode, "another read of this unit is in progress")
	}
	return lock, nil
}

func resolveReadGate(request ReadGateRequest, common, recordPath string, record branchReadRecord, subject AttestationSubject) (branchReadRecord, GateObservation, error) {
	if record.GateRunID == "" {
		if request.Gate == nil {
			return record, GateObservation{}, fmt.Errorf("goal branch read has no gate command")
		}
		repository := branchReadRepositoryFor(request.Repository)
		dir, closeDetached, err := repository.Detached(request.Repo, request.UnitCommit)
		if err != nil {
			return record, GateObservation{}, err
		}
		lastLine, gateErr := request.Gate(dir)
		closeErr := closeDetached()
		if gateErr != nil {
			return record, GateObservation{}, errors.Join(operationRefusal(ReadUngatedCode, "%s", strings.TrimSpace(lastLine)), closeErr)
		}
		if closeErr != nil {
			return record, GateObservation{}, closeErr
		}
		newID := request.NewID
		if newID == nil {
			newID = branchReadID
		}
		record.GateRunID, err = newID("goal-read-gate")
		if err != nil {
			return record, GateObservation{}, err
		}
		if err := saveBranchReadRecord(common, recordPath, record); err != nil {
			return record, GateObservation{}, err
		}
	}
	return record, GateObservation{Kind: "go-gate-fast", Tree: subject.Tree, RunID: record.GateRunID}, nil
}

func ResolveReadGate(request ReadGateRequest) (GateObservation, error) {
	repository := branchReadRepositoryFor(request.Repository)
	subject, err := repository.Subject(request.Repo, request.UnitCommit)
	if err != nil {
		return GateObservation{}, err
	}
	common, recordPath, _, err := branchReadPathsWithRepository(repository, request.Repo, request.GoalID, request.UnitCommit)
	if err != nil {
		return GateObservation{}, err
	}
	lock, err := lockBranchRead(recordPath)
	if err != nil {
		return GateObservation{}, err
	}
	defer func() {
		_ = unix.Flock(int(lock.Fd()), unix.LOCK_UN)
		_ = lock.Close()
	}()
	record, err := loadBranchReadRecord(recordPath)
	if err != nil {
		return GateObservation{}, err
	}
	if record.Goal != "" && (record.Goal != request.GoalID || record.UnitCommit != request.UnitCommit || record.Tree != subject.Tree) {
		return GateObservation{}, fmt.Errorf("goal branch read record does not match the requested unit")
	}
	record.Goal, record.UnitCommit, record.Tree = request.GoalID, request.UnitCommit, subject.Tree
	_, observation, err := resolveReadGate(request, common, recordPath, record, subject)
	return observation, err
}

func ResolveCommitReadGate(request CommitReadRequest, gate func(string) (string, error), newID func(string) (string, error)) (GateObservation, error) {
	if request.Inputs != nil && (request.Transport == nil || request.GateRepository == nil) {
		return GateObservation{}, fmt.Errorf("read commit raw transport and gate repository are required")
	}
	units, err := requestUnits(CommitRequest{Unit: request.Unit, Units: request.Units})
	if err != nil || len(units) == 0 {
		return GateObservation{}, fmt.Errorf("read commits need one or more distinct unit names")
	}
	repository, err := request.Inputs.repository()
	if err != nil {
		return GateObservation{}, err
	}
	state, err := repository.inspectCommitBranch(CommitRequest{Repo: request.Repo, Remote: request.Remote, EndpointTip: request.EndpointTip,
		GoalID: request.GoalID, Units: units, OpID: request.OpID, Kind: Read, Transport: request.Transport})
	if err != nil {
		return GateObservation{}, err
	}
	reads, _, err := request.Inputs.readers()
	if err != nil {
		return GateObservation{}, err
	}
	commit, err := unitCommitInRangeWithReads(reads, request.Repo, request.EndpointTip, state.baseTip, request.GoalID, units)
	if err != nil {
		return GateObservation{}, err
	}
	return ResolveReadGate(ReadGateRequest{Repo: request.Repo, GoalID: request.GoalID, UnitCommit: commit, Gate: gate, NewID: newID, Repository: request.GateRepository})
}

func branchReadID(prefix string) (string, error) {
	raw := make([]byte, 8)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return prefix + "-" + time.Now().UTC().Format("20060102t150405z") + "-" + hex.EncodeToString(raw), nil
}

func branchUnitWithRepository(repository BranchReadRepository, repo, endpoint, tip, goal, commit string) (KindInfo, error) {
	commits, err := repository.Range(repo, endpoint, tip, goal)
	if err != nil {
		return KindInfo{}, err
	}
	for _, candidate := range commits {
		if candidate.ID == commit && candidate.Kind == Unit {
			return KindInfo{Kind: Unit, Units: candidate.Units, Unit: candidate.Unit, CommitID: candidate.ID}, nil
		}
	}
	return KindInfo{}, operationRefusal(ReadInvalidCode, "commit %s is not a Goal-Unit commit of goal %s's branch", commit, goal)
}

// branchReadDefaultMode is the Working Mode header of a critic brief whose
// supplied prose declares none (docs/working-modes.md: implement is the default).
const branchReadDefaultMode = "Working Mode: implement"

func branchReadBriefWithRepository(repository BranchReadRepository, repo, endpoint, goal, commit string, supplied []byte) (string, error) {
	commits, err := repository.Range(repo, endpoint, commit, goal)
	if err != nil {
		return "", err
	}
	brief := "# Code read for goal branch unit\n\n" +
		"Review unit commit `" + commit + "` for goal `" + goal + "`.\n\n" +
		"Inspect its exact change with:\n\n```sh\ngit diff " + commit + "^ " + commit + " --\n```\n\n" +
		"Review against the goal's accepted requirements and the unit's actual behavior. " +
		"Report correctness, safety, contract, test, and compatibility defects that should refuse this commit. " +
		"Close the finding register cleanly only when no refusal-worthy defect remains.\n"
	for _, fold := range commits {
		if fold.Kind != Plan {
			continue
		}
		entries, err := repository.Entries(repo, fold.ID)
		if err != nil {
			return "", err
		}
		brief += "\nGoal-Plan fold `" + fold.ID + "`:\n"
		for _, entry := range entries {
			brief += "- `" + entry.Path + "`\n"
		}
	}
	if len(supplied) != 0 {
		brief += "\n# Supplied accepted implementation brief (frozen at dispatch)\n\n" + string(supplied) + "\n"
	}
	// Dispatch admits a critic brief only with exactly one filled Working
	// Mode header. Headerless prose reads in the default implement mode; a
	// supplied header is kept as written and a malformed or repeated one is
	// refused here rather than at dispatch.
	switch _, declared, err := dispatch.BriefTextMode([]byte(brief)); {
	case declared == 0:
		brief = branchReadDefaultMode + "\n\n" + brief
	case err != nil:
		return "", operationRefusal(ReadInvalidCode, "goal branch brief for unit %s must declare exactly one filled Working Mode header", commit)
	}
	return brief, nil
}

func branchReadInput(path string) ([]byte, string, error) {
	if path == "" {
		return nil, "", nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, "", fmt.Errorf("read goal branch brief %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return nil, "", fmt.Errorf("goal branch brief %s is not a regular file", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("read goal branch brief %s: %w", path, err)
	}
	if len(data) == 0 {
		return nil, "", fmt.Errorf("goal branch brief %s is empty", path)
	}
	sum := sha256.Sum256(data)
	return data, hex.EncodeToString(sum[:]), nil
}

func branchReadJobState(repo, job string) (string, error) {
	record, err := dispatch.ReadRecordObject(filepath.Join(repo, "artifacts", "agents", "jobs", job+".json"))
	if err != nil {
		return "", err
	}
	lens := dispatch.JobRecordOf(record)
	if lens.JobID() != job || lens.Role() != "code-critic" || lens.ParentJob() != "" {
		return "", fmt.Errorf("recorded reader root %s is not a code-critic root", job)
	}
	return lens.Status(), nil
}

func criticTestChangesWithRepository(repository BranchReadRepository, repo, commit, job string) ([]TestChange, error) {
	entries, err := repository.Entries(repo, commit)
	if err != nil {
		return nil, err
	}
	paths := testPathsFromEntries(entries)
	changes := make([]TestChange, 0, len(paths))
	for _, path := range paths {
		changes = append(changes, TestChange{Path: path, ReaderWord: "reviewed by critic root " + job})
	}
	return changes, nil
}

func RunBranchRead(request BranchReadRequest) (result BranchReadResult, err error) {
	if err := CheckCommitAccess(request.GoalID, request.CheckClaim); err != nil {
		return result, err
	}
	repository := branchReadRepositoryFor(request.Repository)
	info, err := branchUnitWithRepository(repository, request.Repo, request.EndpointTip, request.BranchTip, request.GoalID, request.UnitCommit)
	if err != nil {
		return result, err
	}
	subject, err := repository.Subject(request.Repo, request.UnitCommit)
	if err != nil {
		return result, err
	}
	common, recordPath, briefPath, err := branchReadPathsWithRepository(repository, request.Repo, request.GoalID, request.UnitCommit)
	if err != nil {
		return result, err
	}
	lock, err := lockBranchRead(recordPath)
	if err != nil {
		return result, err
	}
	defer func() {
		_ = unix.Flock(int(lock.Fd()), unix.LOCK_UN)
		_ = lock.Close()
	}()
	record, err := loadBranchReadRecord(recordPath)
	if err != nil {
		return result, err
	}
	if record.Goal != "" && (record.Goal != request.GoalID || record.UnitCommit != request.UnitCommit || record.Tree != subject.Tree) {
		return result, fmt.Errorf("goal branch read record does not match the requested unit")
	}
	supplied, inputSHA256, err := branchReadInput(request.BriefPath)
	if err != nil {
		return result, err
	}
	if (record.RootJob != "" || record.DispatchPending || record.DispatchRetryable) && (inputSHA256 != "" && inputSHA256 != record.BriefInputSHA256 ||
		request.Runtime != "" && request.Runtime != record.Runtime || request.Model != "" && request.Model != record.Model) {
		return result, operationRefusal(ReadInvalidCode, "goal branch read already dispatched with different brief or runtime/model overrides")
	}
	if record.DispatchPending && record.RootJob == "" {
		return result, operationRefusal(ReadDispatchPendingCode, "critic dispatch outcome is unknown for unit %s; inspect the job store before recovery", request.UnitCommit)
	}
	record.Goal, record.UnitCommit, record.Tree = request.GoalID, request.UnitCommit, subject.Tree
	result.GateRunID, result.RootJob, result.AttestationCommit = record.GateRunID, record.RootJob, record.AttestationCommit
	if request.Retry > 0 {
		return retryBranchRead(request, common, recordPath, record, result)
	}
	if record.AttestationCommit != "" {
		result.State = "already-collected"
		return result, nil
	}
	if record.RootJob != "" {
		status, stateErr := branchReadJobState(request.Repo, record.RootJob)
		if stateErr != nil {
			return result, stateErr
		}
		if !dispatch.TerminalStatus(status) {
			result.State = "open"
			return result, nil
		}
		if !request.Collect {
			result.State = "closed"
			return result, nil
		}
		// A Goal-Read installed by an earlier collection whose record save
		// was lost is adopted, never committed a second time.
		// An injected read repository answers no Git of its own, so the
		// reconciliation runs on the Git repository path only.
		if installed, found, installedErr := installedBranchReadFor(request, info, record); installedErr != nil {
			return result, installedErr
		} else if found {
			record.AttestationCommit = installed
			if err := saveBranchReadRecord(common, recordPath, record); err != nil {
				return result, err
			}
			result.State, result.AttestationCommit = "collected", installed
			return result, nil
		}
		tests, testsErr := criticTestChangesWithRepository(repository, request.Repo, request.UnitCommit, record.RootJob)
		if testsErr != nil {
			return result, testsErr
		}
		commit := request.Commit
		if commit == nil {
			commit = CommitRead
		}
		attestationCommit, _, commitErr := commit(CommitReadRequest{Repo: request.Repo, Remote: request.Remote,
			EndpointTip: request.EndpointTip, GoalID: request.GoalID, Units: info.Units, OpID: record.GateRunID + "-collect",
			CheckClaim: request.CheckClaim, RootJob: record.RootJob, GateRunID: record.GateRunID, GateTree: record.Tree, TestsChanged: tests})
		if commitErr != nil {
			return result, commitErr
		}
		record.AttestationCommit = attestationCommit
		if err := saveBranchReadRecord(common, recordPath, record); err != nil {
			return result, err
		}
		result.State, result.AttestationCommit = "collected", attestationCommit
		return result, nil
	}
	if request.Collect {
		return result, operationRefusal(ReadInvalidCode, "no critic root has been dispatched for unit %s", request.UnitCommit)
	}
	record, _, err = resolveReadGate(ReadGateRequest{Repo: request.Repo, GoalID: request.GoalID,
		UnitCommit: request.UnitCommit, Gate: request.Gate, NewID: request.NewID, Repository: repository}, common, recordPath, record, subject)
	if err != nil {
		return result, err
	}
	if request.Delegate == nil {
		return result, fmt.Errorf("goal branch read has no delegate command")
	}
	var brief string
	effectiveRuntime, effectiveModel := request.Runtime, request.Model
	if record.DispatchRetryable {
		if record.Brief != briefPath || record.FrozenBriefSHA256 == "" || record.GateRunID == "" {
			return result, operationRefusal(ReadDispatchPendingCode, "frozen critic dispatch intent is incomplete for unit %s", request.UnitCommit)
		}
		frozen, readErr := os.ReadFile(briefPath)
		if readErr != nil {
			return result, operationRefusal(ReadDispatchPendingCode, "frozen critic brief for unit %s is unreadable: %v", request.UnitCommit, readErr)
		}
		sum := sha256.Sum256(frozen)
		if hex.EncodeToString(sum[:]) != record.FrozenBriefSHA256 {
			return result, operationRefusal(ReadDispatchPendingCode, "frozen critic brief for unit %s changed", request.UnitCommit)
		}
		effectiveRuntime, effectiveModel = record.Runtime, record.Model
	} else {
		brief, err = branchReadBriefWithRepository(repository, request.Repo, request.EndpointTip, request.GoalID, request.UnitCommit, supplied)
		if err != nil {
			return result, err
		}
		durable, writeErr := atomicfile.WriteText(briefPath, brief, common)
		if writeErr != nil {
			return result, writeErr
		}
		if !durable {
			return result, operationRefusal(ReadDispatchPendingCode, "frozen read brief durability is unknown; no critic was launched")
		}
		record.Brief, record.BriefInputSHA256 = briefPath, inputSHA256
		record.Runtime, record.Model = effectiveRuntime, effectiveModel
		sum := sha256.Sum256([]byte(brief))
		record.FrozenBriefSHA256 = hex.EncodeToString(sum[:])
	}
	record.DispatchPending, record.DispatchRetryable = true, false
	if err := saveBranchReadRecord(common, recordPath, record); err != nil {
		return result, err
	}
	job, err := request.Delegate(briefPath, request.GoalID, request.UnitCommit, effectiveRuntime, effectiveModel)
	if err != nil || job == "" {
		var neverLaunched *ReadNeverLaunchedError
		if job == "" && errors.As(err, &neverLaunched) {
			record.DispatchPending, record.DispatchRetryable = false, true
			if saveErr := saveBranchReadRecord(common, recordPath, record); saveErr != nil {
				return result, fmt.Errorf("%s: pre-launch refusal could not be recorded for retry: %w", ReadDispatchPendingCode, errors.Join(err, saveErr))
			}
		}
		return result, errors.Join(fmt.Errorf("goal branch read could not dispatch its critic"), err)
	}
	record.RootJob, record.DispatchPending, record.DispatchRetryable = job, false, false
	if err := saveBranchReadRecord(common, recordPath, record); err != nil {
		return result, fmt.Errorf("%s: critic root %s may be running; recording its outcome failed: %w", ReadDispatchPendingCode, job, err)
	}
	result.State, result.RootJob, result.GateRunID = "dispatched", job, record.GateRunID
	return result, nil
}

// installedBranchRead finds the Goal-Read of this unit commit already on the
// local goal branch and adopts it only when its attestation validates and
// binds this record's critic root, fast-gate run and subject tree.
func installedBranchRead(request BranchReadRequest, info KindInfo, record branchReadRecord) (string, bool, error) {
	tip, present, err := localBranchTip(request.Repo, goalBranchRef(request.GoalID))
	if err != nil || !present {
		return "", false, err
	}
	commits, err := ValidateRange(request.Repo, request.EndpointTip, tip, request.GoalID)
	if err != nil {
		return "", false, err
	}
	for _, commit := range commits {
		if commit.Kind != Read {
			continue
		}
		kind, err := KindOf(request.Repo, commit.ID, request.GoalID)
		if err != nil {
			return "", false, err
		}
		if kind.CommitID != request.UnitCommit {
			continue
		}
		att, err := ValidateAttestation(request.Repo, request.EndpointTip, request.GoalID, unitList(info.Units), request.UnitCommit)
		if err != nil {
			return "", false, operationRefusal(ReadInvalidCode, "installed read %s of %s does not validate: %v", commit.ID, request.UnitCommit, err)
		}
		if att.Source.RootJob != record.RootJob || att.Gate.RunID != record.GateRunID || att.Subject.Tree != record.Tree {
			return "", false, operationRefusal(ReadInvalidCode, "installed read %s of %s binds critic %s gate %s, not this record's %s %s",
				commit.ID, request.UnitCommit, att.Source.RootJob, att.Gate.RunID, record.RootJob, record.GateRunID)
		}
		return commit.ID, true, nil
	}
	return "", false, nil
}

func installedBranchReadFor(request BranchReadRequest, info KindInfo, record branchReadRecord) (string, bool, error) {
	if request.Repository != nil {
		return "", false, nil
	}
	return installedBranchRead(request, info, record)
}

// retryBranchRead examines the recorded critic chain's failed round once
// more, in the same chain. The mapping from the failed round to its retry is
// saved under the read lock before the follow-up starts, so repeating the
// same retry rejoins it, even after the retry itself failed; the dispatch
// policy decides whether the failed round may be retried at all.
func retryBranchRead(request BranchReadRequest, common, recordPath string, record branchReadRecord, result BranchReadResult) (BranchReadResult, error) {
	if record.RootJob == "" {
		return result, operationRefusal(ReadInvalidCode, "no examination of unit %s has been dispatched, so there is nothing to retry", request.UnitCommit)
	}
	if record.AttestationCommit != "" {
		return result, operationRefusal(ReadInvalidCode, "the read of unit %s is collected; its examination is not retried", request.UnitCommit)
	}
	key := strconv.FormatInt(request.Retry, 10)
	records, err := dispatch.ChainRecords(request.Repo, record.RootJob)
	if err != nil {
		return result, err
	}
	var newest map[string]any
	var newestRound int64
	for _, one := range records {
		if round, parseErr := strconv.ParseInt(fmt.Sprint(one["round"]), 10, 64); parseErr == nil && round >= newestRound {
			newest, newestRound = one, round
		}
	}
	admitted := record.Retries[key]
	if admitted == "pending" && newestRound == request.Retry+1 {
		// The follow-up started but its report was lost: adopt the round.
		admitted, _ = newest["jobId"].(string)
		record.Retries[key] = admitted
		if err := saveBranchReadRecord(common, recordPath, record); err != nil {
			return result, err
		}
	}
	if admitted != "" && admitted != "pending" {
		result.State, result.Retry = "retry-joined", admitted
		return result, nil
	}
	if newest == nil || newestRound != request.Retry {
		return result, operationRefusal(ReadInvalidCode, "examination round %d is not the newest round of %s (the newest is %d)", request.Retry, record.RootJob, newestRound)
	}
	if err := dispatch.ExaminationRetryAdmissible(request.Repo, newest); err != nil {
		return result, operationRefusal(ReadInvalidCode, "%v", err)
	}
	if request.FollowUp == nil || record.Brief == "" {
		return result, fmt.Errorf("goal branch read has no follow-up command or frozen brief for a retry")
	}
	if record.Retries == nil {
		record.Retries = map[string]string{}
	}
	record.Retries[key] = "pending"
	if err := saveBranchReadRecord(common, recordPath, record); err != nil {
		return result, err
	}
	job, err := request.FollowUp(record.RootJob, record.Brief)
	if err != nil || job == "" {
		return result, errors.Join(fmt.Errorf("the examination retry could not start"), err)
	}
	record.Retries[key] = job
	if err := saveBranchReadRecord(common, recordPath, record); err != nil {
		return result, fmt.Errorf("%s: retry round %s may be running; recording it failed: %w", ReadDispatchPendingCode, job, err)
	}
	result.State, result.Retry = "dispatched", job
	return result, nil
}

// InspectBranchRead reads one unit commit's read record without locking or
// writing: its critic root and, once collected, its attestation. A unit with
// no record yet has neither.
func InspectBranchRead(repo, goalID, unitCommit string) (BranchReadResult, error) {
	_, recordPath, _, err := branchReadPathsWithRepository(branchReadRepositoryFor(nil), repo, goalID, unitCommit)
	if err != nil {
		return BranchReadResult{}, err
	}
	record, err := loadBranchReadRecord(recordPath)
	if err != nil {
		return BranchReadResult{}, err
	}
	result := BranchReadResult{RootJob: record.RootJob, GateRunID: record.GateRunID, AttestationCommit: record.AttestationCommit}
	switch {
	case record.AttestationCommit != "":
		result.State = "collected"
		if result.Published, err = attestationPublished(repo, goalID, record.AttestationCommit); err != nil {
			return result, err
		}
	case record.RootJob != "":
		result.State = "examining"
	}
	return result, nil
}

// attestationPublished reports whether the push owner's recorded origin tip
// for the goal branch contains the attestation. No recorded tip means the
// branch was never published from this checkout.
func attestationPublished(repo, goalID, attestation string) (bool, error) {
	out, err := gitOutput(repo, "for-each-ref", "--format=%(objectname)", originTipRef(goalID))
	if err != nil {
		return false, err
	}
	tip := strings.TrimSpace(string(out))
	if tip == "" {
		return false, nil
	}
	if tip == attestation {
		return true, nil
	}
	return ancestor(repo, attestation, tip)
}
