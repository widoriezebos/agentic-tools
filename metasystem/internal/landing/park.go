package landing

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/authority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/wiredoc"
)

const landingParkLimit = 10 * time.Second

var recoveryRefPattern = regexp.MustCompile(`^refs/metasystem/landing/recertifications/[0-9a-f]{64}/(?:source|merged)$`)
var parkReasonPattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

type RecoveryRef struct {
	Name string `json:"name"`
	OID  string `json:"oid"`
}

type ParkRecord struct {
	SchemaVersion   int           `json:"schemaVersion"`
	State           string        `json:"state"`
	Chain           string        `json:"chain"`
	Goal            *string       `json:"goal"`
	Actor           string        `json:"actor"`
	TargetCommit    string        `json:"targetCommit"`
	Reason          string        `json:"reason"`
	Detail          string        `json:"detail"`
	ParkedAt        string        `json:"parkedAt"`
	Recertification *string       `json:"recertification"`
	CandidateCommit *string       `json:"candidateCommit"`
	RecoveryRefs    []RecoveryRef `json:"recoveryRefs"`
	RecordDigest    string        `json:"recordDigest"`
}

type ParkParams struct {
	Root            string
	Chain           string
	TargetCommit    string
	Reason          string
	Detail          string
	Recertification string
	CandidateCommit string
	RecoveryRefs    []string
	CallerPID       int64
	Now             time.Time
}

type ParkResult struct {
	State      string `json:"state"`
	Reason     string `json:"reason"`
	ParkRecord string `json:"parkRecord"`
}

type ParkFailure struct {
	Cause string
	Err   error
}

func (e *ParkFailure) Error() string {
	return fmt.Sprintf("chain-recertification-park-failed cause=%s: %v", e.Cause, e.Err)
}
func (e *ParkFailure) Unwrap() error { return e.Err }

func parkDigest(record ParkRecord) (string, error) {
	record.RecordDigest = ""
	data, err := json.Marshal(record)
	if err != nil {
		return "", err
	}
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		return "", err
	}
	delete(value, "recordDigest")
	canonical, err := wiredoc.RenderValue(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), nil
}

func parkClassification(root string, callerPID int64) (lease.ClassifyResult, map[string]any, error) {
	view, err := lease.ClassifyVerb(root, callerPID)
	if err != nil {
		return view, nil, err
	}
	classification := map[string]any{"class": view.Class, "holder": view.Holder}
	if err := authority.Authorize("holder-only", classification, ""); err != nil {
		return view, classification, err
	}
	return view, classification, nil
}

func acceptedGoal(root, id string) (*goal.GoalFile, error) {
	workspace := gittree.Workspace{Dir: root}
	tip, err := workspace.ResolveCommit(goal.AcceptedRef)
	if err != nil {
		return nil, err
	}
	files, err := goal.ReadCommitGoals(root, tip)
	if err != nil {
		return nil, err
	}
	tree, problems := goal.ParseTreeFiles(files)
	if len(problems) > 0 {
		return nil, fmt.Errorf("accepted goal ledger is malformed: %s", problems[0])
	}
	found := tree.Live[id]
	if found == nil {
		return nil, fmt.Errorf("goal %s is absent from the accepted ledger", id)
	}
	return found, nil
}

func authenticatedParkActor(root, chain string, view lease.ClassifyResult) (actor string, goalValue *string, err error) {
	data, err := os.ReadFile(filepath.Join(root, "artifacts", "agents", "jobs", chain+".json"))
	if err != nil {
		return "", nil, err
	}
	var rootRecord map[string]any
	if json.Unmarshal(data, &rootRecord) != nil || rootRecord["jobId"] != chain || rootRecord["parentJob"] != nil || rootRecord["role"] != "implementer" {
		return "", nil, fmt.Errorf("chain root record is malformed")
	}
	goalID, _ := rootRecord["goalId"].(string)
	if goalID != "" && goalID != "none-explicit" {
		goalValue = &goalID
	}
	if view.Class == "HUMAN" {
		return "human", goalValue, nil
	}
	if view.Class != "MAIN" || !view.Holder {
		return "", nil, fmt.Errorf("landing park requires the authenticated holder or human")
	}
	machine, err := goal.ResolveMachine(root)
	if err != nil {
		return "", nil, err
	}
	holder, err := lease.CurrentHolder(root)
	if err != nil {
		return "", nil, fmt.Errorf("current holder lineage is unreadable: %w", err)
	}
	if holder.OwnerLineage == "" {
		return "", nil, fmt.Errorf("current holder lineage is unreadable")
	}
	actor = machine + "+" + holder.OwnerLineage
	if goalValue != nil {
		accepted, err := acceptedGoal(root, *goalValue)
		if err != nil {
			return "", nil, err
		}
		if accepted.State != goal.StateClaimed || accepted.Claimed == nil ||
			accepted.Claimed.Machine != machine || accepted.Claimed.Lineage != holder.OwnerLineage {
			return "", nil, fmt.Errorf("goal %s is not claimed by %s", *goalValue, actor)
		}
	}
	return actor, goalValue, nil
}

func canonicalDiagnosticRecertification(root, chain, supplied string) *string {
	if supplied == "" || filepath.IsAbs(supplied) || filepath.ToSlash(filepath.Clean(supplied)) != supplied ||
		strings.Contains(supplied, "..") {
		return nil
	}
	prefix, err := (gittree.Workspace{Dir: root}).Prefix()
	if err != nil {
		return nil
	}
	want := filepath.ToSlash(filepath.Join(prefix, "artifacts", "agents", "landing", "recertifications", chain)) + "/"
	if !strings.HasPrefix(supplied, want) || !strings.HasSuffix(supplied, "/record.json") {
		return nil
	}
	copyValue := supplied
	return &copyValue
}

func parkRecoveryRefs(workspace gittree.Workspace, names []string) ([]RecoveryRef, error) {
	unique := map[string]bool{}
	for _, name := range names {
		if !recoveryRefPattern.MatchString(name) {
			return nil, fmt.Errorf("recovery ref %q is not a recertification source/result anchor", name)
		}
		unique[name] = true
	}
	ordered := make([]string, 0, len(unique))
	for name := range unique {
		ordered = append(ordered, name)
	}
	sort.Strings(ordered)
	refs := make([]RecoveryRef, 0, len(ordered))
	for _, name := range ordered {
		oid, err := workspace.ResolveTree(name)
		if err != nil {
			return nil, err
		}
		refs = append(refs, RecoveryRef{Name: name, OID: oid})
	}
	return refs, nil
}

func parkLock(root, chain string, deadline time.Time) (func(), error) {
	lockPath := filepath.Join(root, "artifacts", "agents", "landing", "locks", chain+".d")
	pid, tag := int64(os.Getpid()), "metasystem"
	for {
		if err := dispatch.OwnerLockClaim(lockPath, pid, tag); err == nil {
			return func() { _ = dispatch.OwnerLockRelease(lockPath, pid, tag) }, nil
		} else if !errors.Is(err, dispatch.ErrOwnerLockBusy) {
			return nil, err
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("park lock timed out")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func refuseSymlinkedParkPath(root, directory string) error {
	root = filepath.Clean(root)
	current := root
	relative, err := filepath.Rel(root, directory)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("park directory escapes the project root")
	}
	for _, component := range strings.Split(relative, string(filepath.Separator)) {
		if component == "" || component == "." {
			continue
		}
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("park output path contains symlink %s", current)
		}
	}
	return nil
}

func parkRepositoryRelative(root, path string) (string, error) {
	top, err := (gittree.Workspace{Dir: root}).TopLevel()
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(top, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("park path is outside the repository")
	}
	return filepath.ToSlash(relative), nil
}

// Park durably records a stopped landing attempt without changing any goal,
// claim, fence, approval, obligation, branch, or Git ref.
func Park(params ParkParams) (ParkResult, error) {
	fail := func(err error) (ParkResult, error) {
		return ParkResult{}, &ParkFailure{Cause: params.Reason, Err: err}
	}
	if params.Root == "" || !landingID.MatchString(params.Chain) || !parkReasonPattern.MatchString(params.Reason) ||
		!knownRefusalCode(params.Reason) || params.Detail == "" {
		return fail(fmt.Errorf("park requires root, a valid chain, reason, and detail"))
	}
	if params.CallerPID <= 0 {
		return fail(fmt.Errorf("caller pid is required"))
	}
	workspace := gittree.Workspace{Dir: params.Root}
	target, err := workspace.ResolveCommit(params.TargetCommit)
	if err != nil || target != params.TargetCommit {
		return fail(fmt.Errorf("target is not one full commit object id"))
	}
	var candidate *string
	if params.CandidateCommit != "" {
		oid, err := workspace.ResolveCommit(params.CandidateCommit)
		if err != nil || oid != params.CandidateCommit {
			return fail(fmt.Errorf("candidate is not one full commit object id"))
		}
		candidate = &oid
	}
	refs, err := parkRecoveryRefs(workspace, params.RecoveryRefs)
	if err != nil {
		return fail(err)
	}
	deadline := time.Now().Add(landingParkLimit)
	release, err := parkLock(params.Root, params.Chain, deadline)
	if err != nil {
		return fail(err)
	}
	defer release()
	view, _, err := parkClassification(params.Root, params.CallerPID)
	if err != nil {
		return fail(err)
	}
	actor, goalValue, err := authenticatedParkActor(params.Root, params.Chain, view)
	if err != nil {
		return fail(err)
	}
	now := params.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	record := ParkRecord{
		SchemaVersion: 1, State: "parked", Chain: params.Chain, Goal: goalValue, Actor: actor,
		TargetCommit: target, Reason: params.Reason, Detail: params.Detail,
		ParkedAt:        now.UTC().Format(time.RFC3339Nano),
		Recertification: canonicalDiagnosticRecertification(params.Root, params.Chain, params.Recertification),
		CandidateCommit: candidate, RecoveryRefs: refs,
	}
	record.RecordDigest, err = parkDigest(record)
	if err != nil {
		return fail(err)
	}
	dir := filepath.Join(params.Root, "artifacts", "agents", "landing", "parks", params.Chain)
	if err := refuseSymlinkedParkPath(params.Root, dir); err != nil {
		return fail(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fail(fmt.Errorf("create park directory: %w", err))
	}
	path := filepath.Join(dir, record.RecordDigest+".json")
	encoded, err := wiredoc.RenderValue(record)
	if err != nil {
		return fail(err)
	}
	if existing, readErr := os.ReadFile(path); readErr == nil {
		if bytes.Equal(existing, encoded) {
			relative, _ := parkRepositoryRelative(params.Root, path)
			return ParkResult{State: "parked", Reason: params.Reason, ParkRecord: relative}, nil
		}
		return fail(fmt.Errorf("publish park record: existing record has different bytes"))
	} else if !os.IsNotExist(readErr) {
		return fail(fmt.Errorf("publish park record: %w", readErr))
	}
	temporary, err := os.CreateTemp(dir, ".park-record.*")
	if err != nil {
		return fail(fmt.Errorf("create temporary park record: %w", err))
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o644); err != nil {
		temporary.Close()
		return fail(err)
	}
	if _, err := temporary.Write(encoded); err != nil {
		temporary.Close()
		return fail(err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fail(err)
	}
	if err := temporary.Close(); err != nil {
		return fail(err)
	}
	// Hard-link publication is atomic and cannot replace an existing name.
	if err := os.Link(temporaryPath, path); err != nil {
		if errors.Is(err, os.ErrExist) {
			if existing, readErr := os.ReadFile(path); readErr == nil && bytes.Equal(existing, encoded) {
				relative, _ := parkRepositoryRelative(params.Root, path)
				return ParkResult{State: "parked", Reason: params.Reason, ParkRecord: relative}, nil
			}
		}
		return fail(fmt.Errorf("publish park record: %w", err))
	}
	if directory, err := os.Open(dir); err == nil {
		if syncErr := directory.Sync(); syncErr != nil {
			directory.Close()
			return fail(syncErr)
		}
		if closeErr := directory.Close(); closeErr != nil {
			return fail(closeErr)
		}
	} else {
		return fail(err)
	}
	if time.Now().After(deadline) {
		return fail(fmt.Errorf("park publication exceeded ten seconds"))
	}
	readback, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(readback, encoded) {
		return fail(fmt.Errorf("park record read-back failed: %w", err))
	}
	relative, err := parkRepositoryRelative(params.Root, path)
	if err != nil {
		return fail(err)
	}
	return ParkResult{State: "parked", Reason: params.Reason, ParkRecord: relative}, nil
}
