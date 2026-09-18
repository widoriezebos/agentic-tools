package branch

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

type BranchReadRequest struct {
	Repo, Remote, EndpointTip, BranchTip, GoalID, UnitCommit string
	Collect                                                  bool
	CheckClaim                                               func() error
	Gate                                                     func(string) (string, error)
	Delegate                                                 func(string, string, string) (string, error)
	Commit                                                   func(CommitReadRequest) (string, Attestation, error)
	NewID                                                    func(string) (string, error)
}

type BranchReadResult struct {
	State, RootJob, GateRunID, AttestationCommit string
}

type ReadGateRequest struct {
	Repo, GoalID, UnitCommit string
	Gate                     func(string) (string, error)
	NewID                    func(string) (string, error)
}

type branchReadRecord struct {
	SchemaVersion     int    `json:"schemaVersion"`
	Goal              string `json:"goal"`
	UnitCommit        string `json:"unitCommit"`
	Tree              string `json:"tree"`
	GateRunID         string `json:"gateRunId,omitempty"`
	RootJob           string `json:"rootJob,omitempty"`
	Brief             string `json:"brief,omitempty"`
	AttestationCommit string `json:"attestationCommit,omitempty"`
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

func branchReadPaths(repo, goal, commit string) (common, record, brief string, err error) {
	common, err = gitCommonDir(repo)
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
	_, err = atomicfile.WriteText(path, string(append(data, '\n')), common)
	return err
}

func resolveReadGate(request ReadGateRequest, common, recordPath string, record branchReadRecord, subject AttestationSubject) (branchReadRecord, GateObservation, error) {
	if record.GateRunID == "" {
		if request.Gate == nil {
			return record, GateObservation{}, fmt.Errorf("goal branch read has no gate command")
		}
		detached, err := (gittree.Workspace{Dir: request.Repo}).NewDetachedCommitWorktree(request.UnitCommit)
		if err != nil {
			return record, GateObservation{}, err
		}
		lastLine, gateErr := request.Gate(detached.Workspace().Dir)
		closeErr := detached.Close()
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
	subject, _, err := computeSubject(request.Repo, request.UnitCommit)
	if err != nil {
		return GateObservation{}, err
	}
	common, recordPath, _, err := branchReadPaths(request.Repo, request.GoalID, request.UnitCommit)
	if err != nil {
		return GateObservation{}, err
	}
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
	units, err := requestUnits(CommitRequest{Unit: request.Unit, Units: request.Units})
	if err != nil || len(units) == 0 {
		return GateObservation{}, fmt.Errorf("read commits need one or more distinct unit names")
	}
	state, err := inspectCommitBranch(CommitRequest{Repo: request.Repo, Remote: request.Remote, EndpointTip: request.EndpointTip,
		GoalID: request.GoalID, Units: units, OpID: request.OpID, Kind: Read, Transport: request.Transport})
	if err != nil {
		return GateObservation{}, err
	}
	commit, err := unitCommitInRange(request.Repo, request.EndpointTip, state.baseTip, request.GoalID, units)
	if err != nil {
		return GateObservation{}, err
	}
	return ResolveReadGate(ReadGateRequest{Repo: request.Repo, GoalID: request.GoalID, UnitCommit: commit, Gate: gate, NewID: newID})
}

func branchReadID(prefix string) (string, error) {
	raw := make([]byte, 8)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return prefix + "-" + time.Now().UTC().Format("20060102t150405z") + "-" + hex.EncodeToString(raw), nil
}

func branchUnit(repo, endpoint, tip, goal, commit string) (KindInfo, error) {
	commits, err := ValidateRange(repo, endpoint, tip, goal)
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

func branchReadBrief(goal, commit string) string {
	return "# Code read for goal branch unit\n\n" +
		"Review unit commit `" + commit + "` for goal `" + goal + "`.\n\n" +
		"Inspect its exact change with:\n\n```sh\ngit diff " + commit + "^ " + commit + " --\n```\n\n" +
		"The governing design is `metasystem/plans/goals-live-on-branches-design.md`, especially sections 5 and 10.1. " +
		"Report correctness, safety, contract, test, and compatibility defects that should refuse this commit. " +
		"Close the finding register cleanly only when no refusal-worthy defect remains.\n"
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

func criticTestChanges(repo, commit, job string) ([]TestChange, error) {
	paths, err := requiredTestChanges(repo, commit)
	if err != nil {
		return nil, err
	}
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
	info, err := branchUnit(request.Repo, request.EndpointTip, request.BranchTip, request.GoalID, request.UnitCommit)
	if err != nil {
		return result, err
	}
	subject, _, err := computeSubject(request.Repo, request.UnitCommit)
	if err != nil {
		return result, err
	}
	common, recordPath, briefPath, err := branchReadPaths(request.Repo, request.GoalID, request.UnitCommit)
	if err != nil {
		return result, err
	}
	record, err := loadBranchReadRecord(recordPath)
	if err != nil {
		return result, err
	}
	if record.Goal != "" && (record.Goal != request.GoalID || record.UnitCommit != request.UnitCommit || record.Tree != subject.Tree) {
		return result, fmt.Errorf("goal branch read record does not match the requested unit")
	}
	record.Goal, record.UnitCommit, record.Tree = request.GoalID, request.UnitCommit, subject.Tree
	result.GateRunID, result.RootJob, result.AttestationCommit = record.GateRunID, record.RootJob, record.AttestationCommit
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
		tests, testsErr := criticTestChanges(request.Repo, request.UnitCommit, record.RootJob)
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
		UnitCommit: request.UnitCommit, Gate: request.Gate, NewID: request.NewID}, common, recordPath, record, subject)
	if err != nil {
		return result, err
	}
	brief := branchReadBrief(request.GoalID, request.UnitCommit)
	if _, err := atomicfile.WriteText(briefPath, brief, common); err != nil {
		return result, err
	}
	if request.Delegate == nil {
		return result, fmt.Errorf("goal branch read has no delegate command")
	}
	job, err := request.Delegate(briefPath, request.GoalID, request.UnitCommit)
	if err != nil || job == "" {
		return result, errors.Join(fmt.Errorf("goal branch read could not dispatch its critic"), err)
	}
	record.RootJob, record.Brief = job, briefPath
	if err := saveBranchReadRecord(common, recordPath, record); err != nil {
		return result, err
	}
	result.State, result.RootJob, result.GateRunID = "dispatched", job, record.GateRunID
	return result, nil
}
