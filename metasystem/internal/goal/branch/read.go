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
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatchproc"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/landpath"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
	"golang.org/x/sys/unix"
)

const ReadDispatchPendingCode = "GOAL_READ_DISPATCH_PENDING"

// ReadDispatchFailed heads the error of a read whose critic dispatch failed;
// the dispatch's own account follows on the next lines.
const ReadDispatchFailed = "goal branch read could not dispatch its critic"

type BranchReadRequest struct {
	Repo, Remote, EndpointTip, BranchTip, GoalID, UnitCommit string
	BriefPath, Runtime, Model                                string
	BuildBriefSHA256                                         string
	Collect                                                  bool
	UnitRead                                                 []byte
	// Join has BriefPath start a read only: a read of this build already
	// started (by the other review form, under its own brief) is joined.
	Join bool
	// Selected is the installation that asked for the read (the seat's
	// checkout) when it is not Repo. The caller supplies its registry and
	// settings to Delegate. A file the brief cites that the critic's tree
	// lacks is frozen from it, or from Repo, into Repo.
	Selected string
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
	// CustodyDeath is the custody owner's death-proof dependencies for a
	// retry of a round with a recorded process; zero means the
	// installation's own process-tag matcher.
	CustodyDeath dispatch.CustodyDeathDependencies
}

type BranchReadResult struct {
	State, RootJob, GateRunID, AttestationCommit string
	DispatchRefusal                              string
	// Published is set by InspectBranchRead: the collected attestation is
	// contained in the goal branch's origin tip the push owner last
	// recorded, so the remote holds it as far as this checkout knows.
	Published bool
	// Retry is the examination round a retry admitted, or rejoined.
	Retry string
}

// ReadNeverLaunchedError is returned only when no critic process was started:
// the delegate boundary said so, or its failure left no launchable critic job
// record for this request. The review may be requested again.
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
	BriefInputPath    string `json:"briefInputPath,omitempty"`
	Runtime           string `json:"runtime,omitempty"`
	Model             string `json:"model,omitempty"`
	DispatchPending   bool   `json:"dispatchPending,omitempty"`
	DispatchRetryable bool   `json:"dispatchRetryable,omitempty"`
	DispatchRefusal   string `json:"dispatchRefusal,omitempty"`
	FrozenBriefSHA256 string `json:"frozenBriefSha256,omitempty"`
	// BriefFromBuild marks supplied bytes that match the build's digest.
	BriefFromBuild bool `json:"briefFromBuild,omitempty"`
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

// BuildBriefAdmitted reports whether briefPath is a goal read's composed
// brief whose record marks it as carrying the unit's build brief and whose
// bytes are the ones recorded. Any read or parse failure answers false.
func BuildBriefAdmitted(briefPath string) bool {
	commit, found := strings.CutSuffix(filepath.Base(briefPath), ".md")
	dir := filepath.Dir(briefPath)
	if !found || commit == "" || filepath.Base(filepath.Dir(dir)) != "goal-reads" || filepath.Base(filepath.Dir(filepath.Dir(dir))) != "metasystem" {
		return false
	}
	record, err := loadBranchReadRecord(filepath.Join(dir, commit+".json"))
	if err != nil || !record.BriefFromBuild || record.Brief != briefPath {
		return false
	}
	data, err := os.ReadFile(briefPath)
	if err != nil {
		return false
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]) == record.FrozenBriefSHA256
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

func lockBranchRead(goalID, path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	held, err := lock.File(path+".lock", 0o600, lock.TryExclusive)
	var lockErr *lock.LockError
	if errors.As(err, &lockErr) {
		return nil, operationRefusal(ReadDispatchPendingCode, "another review of this build of goal %s is running\nrun: metasystem work wait %s", goalID, goalID)
	}
	if err != nil {
		return nil, err
	}
	return held.File(), nil
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
	lock, err := lockBranchRead(request.GoalID, recordPath)
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
	return KindInfo{}, operationRefusal(ReadInvalidCode, "commit %s is not a build on goal %s's branch\nrun: metasystem work status %s", commit, goal, goal)
}

// branchReadDefaultMode is the Working Mode header of a critic brief whose
// supplied prose declares none (docs/working-modes.md: implement is the default).
const branchReadDefaultMode = "Working Mode: implement"

func branchReadBriefWithRepository(repository BranchReadRepository, repo, endpoint, goal, commit string, supplied, packet []byte, buildDigest string) (string, error) {
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
	brief += string(packet)
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
		heading := "Corrected implementation brief (given at review)"
		sum := sha256.Sum256(supplied)
		if buildDigest != "" && hex.EncodeToString(sum[:]) == buildDigest {
			heading = "Supplied accepted implementation brief (frozen at dispatch)"
		}
		brief += "\n# " + heading + "\n\n" + string(supplied) + "\n"
	}
	// Dispatch admits a critic brief only with exactly one filled Working
	// Mode header. Headerless prose reads in the default implement mode; a
	// supplied header is kept as written and a malformed or repeated one is
	// refused here rather than at dispatch.
	switch _, declared, err := dispatch.BriefTextMode([]byte(brief)); {
	case declared == 0:
		brief = branchReadDefaultMode + "\n\n" + brief
	case err != nil:
		return "", operationRefusal(ReadInvalidCode, "the brief for build %s needs exactly one filled Working Mode header\nrun: metasystem work review %s --brief FILE", commit, goal)
	}
	return brief, nil
}

// freezeBranchReadDrafts freezes the files the brief cites that the
// critic's tree lacks and the seat's checkout holds (an untracked trace, a
// draft brief): each copy is content-addressed under Repo's artifacts, which
// lie inside the critic's checkout, and named on a frozen-input line the
// brief-authority admission reads (dispatch.FrozenInputLine). A cited path
// no checkout holds is not frozen, so the admission still refuses it. When
// the cited paths can't be read, nothing is frozen.
func freezeBranchReadDrafts(request BranchReadRequest, brief string) (string, error) {
	if request.Repository != nil {
		// An injected read repository answers no Git of its own, so
		// drafts are frozen on the Git repository path only.
		return brief, nil
	}
	drafts, err := dispatch.BriefDraftsHeld([]byte(brief), request.Repo, request.Selected, request.Repo, primaryInstallation(request.Repo))
	if err != nil || len(drafts) == 0 {
		return brief, nil
	}
	var lines []string
	for _, draft := range drafts {
		sum := sha256.Sum256(draft.Content)
		digest := hex.EncodeToString(sum[:])
		copyPath := filepath.Join(request.Repo, "artifacts", "agents", "goal-reads", request.GoalID, "frozen", digest[:16], filepath.Base(filepath.FromSlash(draft.Path)))
		if err := os.MkdirAll(filepath.Dir(copyPath), 0o755); err != nil {
			return "", err
		}
		durable, err := atomicfile.WriteText(copyPath, string(draft.Content), request.Repo)
		if err != nil {
			return "", err
		}
		if !durable {
			return "", operationRefusal(ReadDispatchPendingCode, "a file the review brief cites may not be saved to disk, so no reviewer was started\nrun: metasystem work review %s", request.GoalID)
		}
		lines = append(lines, dispatch.FrozenInputLine(draft.Path, digest, copyPath))
	}
	return brief + "\n# Frozen files\n\nThese files are frozen as the seat's checkout held them when the review was asked. Read each from its copy,\nnever from the checkout, which may have moved since:\n\n" + strings.Join(lines, "\n") + "\n", nil
}

// primaryInstallation is the installation folder of repo's primary
// checkout, where the seat keeps its files when the review is asked from a
// goal worktree; "" when it can't be told.
func primaryInstallation(repo string) string {
	common, err := gitCommonDir(repo)
	if err != nil || filepath.Base(common) != ".git" {
		return ""
	}
	prefix, err := gitOutput(repo, "rev-parse", "--show-prefix")
	if err != nil {
		return ""
	}
	return filepath.Join(filepath.Dir(common), filepath.FromSlash(strings.TrimSpace(string(prefix))))
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

// CriticStore names the installation whose artifacts/agents holds the
// records of critic root job, for a verb run at installation repo. A
// critic's records stay in the installation that dispatched it: a goal
// worktree, or the primary checkout that serves it. So repo keeps its own
// store when it holds the critic's record (or no critic is named); else the
// store of the system that serves repo (landpath.SystemInstallation: its
// primary checkout's installation when repo is an unarmed linked worktree)
// when that holds it; else the goal worktree that system serves
// (landpath.ServedInstallations) which holds it, so a verb run at the
// primary finds a critic dispatched in a goal worktree. When none holds the
// record, the serving system's store when repo is a worktree, else repo.
func CriticStore(repo, job string) string {
	if job == "" {
		return repo
	}
	holds := func(installation string) bool {
		_, err := os.Lstat(filepath.Join(installation, "artifacts", "agents", "jobs", job+".json"))
		return !os.IsNotExist(err)
	}
	if holds(repo) {
		return repo
	}
	root := repo
	if resolved, err := filepath.EvalSymlinks(repo); err == nil {
		root = resolved
	}
	git := func(call landpath.GitCall) landpath.GitResult {
		out, err := gitOutput(call.Dir, call.Args...)
		if err != nil {
			return landpath.GitResult{Code: 1}
		}
		return landpath.GitResult{Stdout: out}
	}
	installation, checkout := landpath.SystemInstallation(func(args ...string) landpath.GitResult {
		return git(landpath.GitCall{Dir: root, Args: args})
	}, root)
	if checkout != "" && holds(installation) {
		return installation
	}
	for _, served := range landpath.ServedInstallations(git, installation) {
		if served != root && holds(served) {
			return served
		}
	}
	if checkout == "" {
		return repo
	}
	return installation
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

// reservedCriticRoot names the critic root reserved for one request to review
// a unit commit, or none. The delegate boundary starts a critic process only
// from a job record it wrote first, and it names that record after the request:
// the job id is derived from the goal, the goal revision the record names and
// the brief the critic reads. A critic root is this request's only when its job
// id is the one derived from this goal, the record's own goal revision and the
// frozen brief. Another caller's examination of the same commit read another
// brief and has another id, so it is never this request's critic. For the
// holder of the read lock a matching record is this request's critic whatever
// its status, and no matching record proves that this request started no
// critic process. A record that cannot be read may be that reservation, so it
// is an error and never "none".
//
// A record without a positive round is not a match. The round number is
// written when a reservation completes its setup and becomes launchable, and a
// process is started only from a launchable record. A reservation without it
// ended in setup: it started no process, and it has no round for the review
// to report or to retry.
func reservedCriticRoot(repo, goalID, unitCommit, frozenBriefSHA256 string) (string, error) {
	dir := filepath.Join(repo, "artifacts", "agents", "jobs")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	var root string
	var newest time.Time
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".json") {
			continue
		}
		record, err := dispatch.ReadRecordObject(filepath.Join(dir, name))
		if err != nil {
			return "", fmt.Errorf("%s: %w", name, err)
		}
		lens := dispatch.JobRecordOf(record)
		reviews, _ := record["reviews"].(string)
		round, _ := lens.Round()
		if lens.Role() != "code-critic" || lens.ParentJob() != "" || lens.GoalID() != goalID || reviews != "commit:"+unitCommit || round < 1 {
			continue
		}
		// A record that names no goal revision, and a request with no frozen
		// brief, have no derived id, so nothing binds the one to the other.
		revision, _ := lens.GoalRevision()
		derived, err := dispatch.DefaultOperationID(goalID, revision, dispatch.DispatchModeFresh, "code-critic", frozenBriefSHA256, "")
		if revision < 1 || err != nil || lens.JobID() != derived {
			continue
		}
		// A critic root that cannot be read back by its job id cannot be
		// adopted, and passing over it would start a second critic.
		if lens.JobID() != strings.TrimSuffix(name, ".json") {
			return "", fmt.Errorf("%s: its job id is not its file name", name)
		}
		createdAt, _ := record["createdAt"].(string)
		created, _ := time.Parse(time.RFC3339, createdAt)
		if root == "" || created.After(newest) {
			root, newest = lens.JobID(), created
		}
	}
	return root, nil
}

// settleBranchReadDispatch decides a request that was saved as pending and
// has no critic root, from the job records: the reserved critic becomes its
// root, and a request with no reservation may be dispatched again from its
// frozen brief. Unreadable job records refuse and leave the request pending.
func settleBranchReadDispatch(request BranchReadRequest, common, recordPath string, record branchReadRecord) (branchReadRecord, error) {
	root, err := reservedCriticRoot(request.Repo, request.GoalID, request.UnitCommit, record.FrozenBriefSHA256)
	if err != nil {
		return record, operationRefusal(ReadDispatchPendingCode, "the job records can't be read, so whether the reviewer of build %s started is unknown: %s\nrun: metasystem work status %s", request.UnitCommit, firstLine(err), request.GoalID)
	}
	record.RootJob, record.DispatchPending, record.DispatchRetryable = root, false, root == ""
	if root != "" {
		record.DispatchRefusal = ""
	}
	return record, saveBranchReadRecord(common, recordPath, record)
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
	lock, err := lockBranchRead(request.GoalID, recordPath)
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
	if request.Join && record.RootJob != "" {
		// A read is of one build: the request joins the one already started
		// and its brief starts nothing.
		supplied, inputSHA256 = nil, ""
	}
	if record.DispatchPending && record.RootJob == "" {
		// An earlier dispatch never reported back. The job records say
		// whether it reserved a critic.
		if record, err = settleBranchReadDispatch(request, common, recordPath, record); err != nil {
			return result, err
		}
	}
	// A request binds only what was dispatched: a critic root. A start the
	// dispatch refused (retryable, no root) started nothing, so a request
	// naming a brief, runtime or model starts over with them; a request
	// naming none repeats the saved start.
	restart := record.RootJob == "" && record.DispatchRetryable && (inputSHA256 != "" || request.Runtime != "" || request.Model != "")
	if !restart && record.RootJob != "" && (inputSHA256 != "" && inputSHA256 != record.BriefInputSHA256 ||
		request.Runtime != "" && request.Runtime != record.Runtime || request.Model != "" && request.Model != record.Model) {
		return result, operationRefusal(ReadBriefChangedCode, "this build's review already started with another brief, runtime or model in critic job %s\nrun: metasystem work review %s", record.RootJob, request.GoalID)
	}
	if restart {
		record.DispatchRetryable = false
		record.DispatchRefusal = ""
	}
	record.Goal, record.UnitCommit, record.Tree = request.GoalID, request.UnitCommit, subject.Tree
	result.GateRunID, result.RootJob, result.AttestationCommit = record.GateRunID, record.RootJob, record.AttestationCommit
	if request.UnitRead != nil && record.RootJob != "" {
		return result, operationRefusal(ReadInvalidCode, "this build's review already started with a critic; its unit read cannot replace that critic\nrun: metasystem work review %s", request.GoalID)
	}
	if request.Retry > 0 {
		return retryBranchRead(request, common, recordPath, record, result)
	}
	if record.AttestationCommit != "" {
		result.State = "already-collected"
		return result, nil
	}
	if request.UnitRead != nil {
		bundle, err := validateUnitReadBundle(request.UnitRead, request.GoalID, subject)
		if err != nil {
			return result, err
		}
		record, _, err = resolveReadGate(ReadGateRequest{Repo: request.Repo, GoalID: request.GoalID,
			UnitCommit: request.UnitCommit, Gate: request.Gate, NewID: request.NewID, Repository: repository}, common, recordPath, record, subject)
		if err != nil {
			return result, err
		}
		entries, err := repository.Entries(request.Repo, request.UnitCommit)
		if err != nil {
			return result, err
		}
		var tests []TestChange
		for _, path := range testPathsFromEntries(entries) {
			tests = append(tests, TestChange{Path: path, ReaderWord: "reviewed by unit read " + bundle.ReadLaunch})
		}
		commit := request.Commit
		if commit == nil {
			commit = CommitRead
		}
		attestation, _, err := commit(CommitReadRequest{Repo: request.Repo, Remote: request.Remote,
			EndpointTip: request.EndpointTip, GoalID: request.GoalID, Units: info.Units, OpID: record.GateRunID + "-collect",
			CheckClaim: request.CheckClaim, UnitRead: request.UnitRead, GateRunID: record.GateRunID, GateTree: record.Tree, TestsChanged: tests})
		if err != nil {
			return result, err
		}
		record.AttestationCommit = attestation
		if err := saveBranchReadRecord(common, recordPath, record); err != nil {
			return result, err
		}
		result.State, result.AttestationCommit, result.GateRunID = "collected", attestation, record.GateRunID
		return result, nil
	}
	if record.RootJob != "" {
		store := CriticStore(request.Repo, record.RootJob)
		status, stateErr := branchReadJobState(store, record.RootJob)
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
		collect := CommitReadRequest{Repo: request.Repo, Remote: request.Remote,
			EndpointTip: request.EndpointTip, GoalID: request.GoalID, Units: info.Units, OpID: record.GateRunID + "-collect",
			CheckClaim: request.CheckClaim, RootJob: record.RootJob, GateRunID: record.GateRunID, GateTree: record.Tree, TestsChanged: tests}
		if store != request.Repo {
			collect.CriticStore = store
		}
		attestationCommit, _, commitErr := commit(collect)
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
		return result, operationRefusal(ReadInvalidCode, "no reviewer has started on build %s, so there is nothing to collect\nrun: metasystem work review %s", request.UnitCommit, request.GoalID)
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
			return result, operationRefusal(ReadDispatchPendingCode, "the saved start of build %s's review is incomplete\nrun: metasystem work status %s", request.UnitCommit, request.GoalID)
		}
		frozen, readErr := os.ReadFile(briefPath)
		if readErr != nil {
			return result, operationRefusal(ReadDispatchPendingCode, "the saved review brief of build %s can't be read: %v\nrun: metasystem work status %s", request.UnitCommit, readErr, request.GoalID)
		}
		sum := sha256.Sum256(frozen)
		if hex.EncodeToString(sum[:]) != record.FrozenBriefSHA256 {
			return result, operationRefusal(ReadDispatchPendingCode, "the saved review brief of build %s changed after the review started\nrun: metasystem work status %s", request.UnitCommit, request.GoalID)
		}
		effectiveRuntime, effectiveModel = record.Runtime, record.Model
	} else {
		var packet []byte
		if request.Join && filepath.Base(request.BriefPath) == "follow-up.md" {
			packet, err = os.ReadFile(filepath.Join(filepath.Dir(request.BriefPath), "read-context.md"))
			if err != nil && !os.IsNotExist(err) {
				return result, err
			}
		}
		brief, err = branchReadBriefWithRepository(repository, request.Repo, request.EndpointTip, request.GoalID, request.UnitCommit, supplied, packet, request.BuildBriefSHA256)
		if err != nil {
			return result, err
		}
		if brief, err = freezeBranchReadDrafts(request, brief); err != nil {
			return result, err
		}
		durable, writeErr := atomicfile.WriteText(briefPath, brief, common)
		if writeErr != nil {
			return result, writeErr
		}
		if !durable {
			return result, operationRefusal(ReadDispatchPendingCode, "the review brief may not be saved to disk, so no reviewer was started\nrun: metasystem work review %s", request.GoalID)
		}
		record.Brief, record.BriefInputSHA256 = briefPath, inputSHA256
		record.BriefInputPath = request.BriefPath
		record.Runtime, record.Model = effectiveRuntime, effectiveModel
		sum := sha256.Sum256([]byte(brief))
		record.FrozenBriefSHA256 = hex.EncodeToString(sum[:])
		record.BriefFromBuild = inputSHA256 != "" && inputSHA256 == request.BuildBriefSHA256
	}
	record.DispatchPending, record.DispatchRetryable = true, false
	record.DispatchRefusal = ""
	if err := saveBranchReadRecord(common, recordPath, record); err != nil {
		return result, err
	}
	job, err := request.Delegate(briefPath, request.GoalID, request.UnitCommit, effectiveRuntime, effectiveModel)
	if err != nil || job == "" {
		failure := errors.Join(errors.New(ReadDispatchFailed), err)
		var neverLaunched *ReadNeverLaunchedError
		if job == "" && errors.As(err, &neverLaunched) {
			record.DispatchPending, record.DispatchRetryable = false, true
			record.DispatchRefusal = firstLine(err)
			if saveErr := saveBranchReadRecord(common, recordPath, record); saveErr != nil {
				return result, fmt.Errorf("%s: pre-launch refusal could not be recorded for retry: %w", ReadDispatchPendingCode, errors.Join(err, saveErr))
			}
			return result, failure
		}
		// The delegate did not say whether it reserved a critic before it
		// failed. The job records do.
		record.DispatchRefusal = firstLine(err)
		settled, settleErr := settleBranchReadDispatch(request, common, recordPath, record)
		if settleErr != nil {
			return result, errors.Join(failure, settleErr)
		}
		if settled.RootJob == "" {
			return result, &ReadNeverLaunchedError{Err: failure}
		}
		return result, failure
	}
	record.RootJob, record.DispatchPending, record.DispatchRetryable = job, false, false
	if err := saveBranchReadRecord(common, recordPath, record); err != nil {
		return result, fmt.Errorf("%s: critic root %s may be running; recording its outcome failed: %w", ReadDispatchPendingCode, job, err)
	}
	result.State, result.RootJob, result.GateRunID = "dispatched", job, record.GateRunID
	return result, nil
}

// installedBranchRead finds the newest Goal-Read of this unit commit already
// on the local goal branch and adopts it only when its attestation validates
// and binds this record's critic root, fast-gate run and subject tree. A unit
// whose current attestation comes from a reader record or a unit read has
// no critic read installed, so a critic's read is still to be committed.
func installedBranchRead(reads attestationReads, request BranchReadRequest, info KindInfo, record branchReadRecord, tip string) (string, bool, error) {
	commits, err := reads.Range(request.Repo, request.EndpointTip, tip, request.GoalID)
	if err != nil {
		return "", false, err
	}
	for index := len(commits) - 1; index >= 0; index-- {
		commit := commits[index]
		if commit.Kind != Read {
			continue
		}
		kind, err := reads.Kind(request.Repo, commit.ID, request.GoalID)
		if err != nil {
			return "", false, err
		}
		if kind.CommitID != request.UnitCommit {
			continue
		}
		att, err := validateAttestation(reads, request.Repo, "", request.EndpointTip, request.GoalID, unitList(info.Units), request.UnitCommit, map[string]bool{})
		if err != nil {
			return "", false, operationRefusal(ReadInvalidCode, "the recorded review %s of build %s doesn't hold up: %s\nrun: metasystem work review %s", commit.ID, request.UnitCommit, firstLine(err), request.GoalID)
		}
		if att.Source.Kind == "reader-record" || att.Source.Kind == "unit-read" {
			return "", false, nil
		}
		if att.Source.RootJob != record.RootJob || att.Gate.RunID != record.GateRunID || att.Subject.Tree != record.Tree {
			return "", false, operationRefusal(ReadInvalidCode, "the recorded review %s of build %s came from reviewer %s and check %s, not %s and %s\nrun: metasystem work status %s",
				commit.ID, request.UnitCommit, att.Source.RootJob, att.Gate.RunID, record.RootJob, record.GateRunID, request.GoalID)
		}
		return commit.ID, true, nil
	}
	return "", false, nil
}

func installedBranchReadFor(request BranchReadRequest, info KindInfo, record branchReadRecord) (string, bool, error) {
	if request.Repository != nil {
		return "", false, nil
	}
	tip, present, err := localBranchTip(request.Repo, goalBranchRef(request.GoalID))
	if err != nil || !present {
		return "", false, err
	}
	return installedBranchRead(gitAttestationReads{}, request, info, record, tip)
}

// retryBranchRead examines the recorded critic chain's failed round once
// more, in the same chain. The mapping from the failed round to its retry is
// saved under the read lock before the follow-up starts, so repeating the
// same retry rejoins it, even after the retry itself failed; the dispatch
// policy decides whether the failed round may be retried at all.
func retryBranchRead(request BranchReadRequest, common, recordPath string, record branchReadRecord, result BranchReadResult) (BranchReadResult, error) {
	if record.RootJob == "" {
		return result, operationRefusal(ReadInvalidCode, "no review of build %s has started, so there is nothing to retry\nrun: metasystem work review %s", request.UnitCommit, request.GoalID)
	}
	if record.AttestationCommit != "" {
		return result, operationRefusal(ReadInvalidCode, "build %s's review is finished and collected, so it isn't retried\nnothing to do; its result stands", request.UnitCommit)
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
		return result, operationRefusal(ReadInvalidCode, "round %d is not the newest round of this review; the newest is %d\nrun: metasystem work review %s --retry %d", request.Retry, newestRound, request.GoalID, newestRound)
	}
	deps := request.CustodyDeath
	if deps.MatchesTag == nil {
		deps.MatchesTag = dispatchproc.PositionedJobTagAt(request.Repo)
	}
	if err := dispatch.ExaminationRetryAdmissibleWith(request.Repo, newest, deps); err != nil {
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
	case record.DispatchRetryable:
		result.State, result.DispatchRefusal = "review refused", record.DispatchRefusal
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
