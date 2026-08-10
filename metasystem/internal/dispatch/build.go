package dispatch

import (
	"fmt"
	"strconv"
)

// The record templates dispatch writes. BuildSetup produces the pending-setup
// reservation husk; BuildRecord and BuildFollowRecord produce the full pending
// records that RecordSetup swaps in. Keeping the shapes here means the same
// package owns what a record looks like and how it is written.

// nullableString maps "" to JSON null, matching the record convention that an
// absent optional field is stored as null rather than empty.
func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

// nullableEpoch parses a claim epoch, mapping "" to null. A non-empty epoch
// must be an integer.
func nullableEpoch(value string) (any, error) {
	if value == "" {
		return nil, nil
	}
	epoch, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid claim epoch %q", value)
	}
	return epoch, nil
}

// BuildSetup writes the pending-setup reservation record: the minimal husk
// that reserves a job id before the full record is assembled. A non-empty
// parent marks a follow-up reservation.
func BuildSetup(output, job, role, parent, mainID, claimEpoch string) error {
	epoch, err := nullableEpoch(claimEpoch)
	if err != nil {
		return err
	}
	record := map[string]any{
		"jobId":      job,
		"role":       role,
		"status":     "pending-setup",
		"phase":      "setup",
		"error":      nil,
		"mainId":     nullableString(mainID),
		"claimEpoch": epoch,
		"createdAt":  nowISO(),
	}
	if parent != "" {
		record["parentJob"] = parent
	}
	return writeRecord(output, record)
}

// capAuthority is the cap resolution a dispatch was authorized under: the
// capMin/capDeadline pair plus the provenance of the decision.
type capAuthority struct {
	capMin   any
	deadline any
	source   map[string]any
}

// readCapAuthority loads a cap-resolution file (capMin, capDeadline, source).
func readCapAuthority(path string) (capAuthority, error) {
	value, err := readObject(path)
	if err != nil {
		return capAuthority{}, fmt.Errorf("cannot read cap resolution: %v", err)
	}
	source, ok := value["source"].(map[string]any)
	if !ok {
		return capAuthority{}, fmt.Errorf("cap resolution has no source object")
	}
	return capAuthority{capMin: value["capMin"], deadline: value["capDeadline"], source: source}, nil
}

// capResolutionField renders the authority into the record's capResolution
// object.
func (a capAuthority) resolutionField() map[string]any {
	return map[string]any{
		"requestedMin": a.capMin,
		"rule":         a.source["rule"],
		"origin":       a.source["origin"],
		"truncatedBy":  a.source["truncatedBy"],
		"deadline":     a.deadline,
	}
}

// BuildRecordParams carries everything a fresh dispatch record is assembled
// from. File-valued fields are read here so the record shape and its inputs
// stay in one place.
type BuildRecordParams struct {
	Output          string
	Job             string
	Role            string
	Mission         string
	MissionTurn     string
	Runtime         string
	Workspace       string
	CapResolution   string // cap-resolution file
	Model           string
	Overridden      bool
	Snapshot        string
	InputBytes      int64
	InputHash       string
	Permissions     string // requested-permissions envelope file
	Fallbacks       string // JSON array of capability fallbacks
	Signal          bool
	HandshakeBudget int64
	ApprovalName    string
	ApprovedAt      string
	RosterPair      string
	RequestedPair   string
	CostDirection   string
	Reviews         string
	MainID          string
	ClaimEpoch      string
}

// BuildRecord assembles the full pending record for a fresh dispatch: chain
// identity, workspace pin (base SHA and branch read from the workspace),
// permissions, cap authority, capability snapshot, and input digest.
func BuildRecord(p BuildRecordParams) error {
	base, err := gitOutput(p.Workspace, "rev-parse", "HEAD")
	if err != nil {
		return fmt.Errorf("workspace is not a git worktree")
	}
	branch, err := gitOutput(p.Workspace, "branch", "--show-current")
	if err != nil {
		return fmt.Errorf("cannot read the workspace branch: %v", err)
	}
	authority, err := readCapAuthority(p.CapResolution)
	if err != nil {
		return err
	}
	permissions, err := readObject(p.Permissions)
	if err != nil {
		return fmt.Errorf("cannot read the requested permissions: %v", err)
	}
	fallbacks, err := decodeJSONValue([]byte(p.Fallbacks))
	if err != nil {
		return fmt.Errorf("invalid capability fallbacks: %v", err)
	}
	epoch, err := nullableEpoch(p.ClaimEpoch)
	if err != nil {
		return err
	}
	var escalation any
	if p.ApprovalName != "" {
		escalation = map[string]any{
			"name":             p.ApprovalName,
			"approvedAt":       p.ApprovedAt,
			"rosterResolution": p.RosterPair,
			"requestedPair":    p.RequestedPair,
			"costDirection":    p.CostDirection,
		}
	}
	record := map[string]any{
		"jobId":         p.Job,
		"role":          p.Role,
		"mission":       nullableString(p.Mission),
		"runtime":       p.Runtime,
		"round":         1,
		"parentJob":     nil,
		"reviews":       nullableString(p.Reviews),
		"status":        "pending",
		"phase":         "handshake",
		"error":         nil,
		"mainId":        nullableString(p.MainID),
		"claimEpoch":    epoch,
		"workspaceRoot": resolvePath(p.Workspace),
		"baseSha":       base,
		"branch":        branch,
		"permissions": map[string]any{
			"requested":           permissions,
			"effective":           nil,
			"enforcementSnapshot": p.Snapshot,
		},
		"capMin":                       authority.capMin,
		"capDeadline":                  authority.deadline,
		"capResolution":                authority.resolutionField(),
		"pid":                          nil,
		"pidStartedAt":                 nil,
		"pgid":                         nil,
		"instanceTag":                  "metasystem-job-" + p.Job,
		"custodyProcesses":             []any{},
		"sessionId":                    nil,
		"turnId":                       nullableString(p.MissionTurn),
		"requestedModel":               p.Model,
		"effectiveModel":               nil,
		"overridden":                   p.Overridden,
		"capabilitySnapshot":           p.Snapshot,
		"escalationApproval":           escalation,
		"capabilityFallbacks":          fallbacks,
		"sessionEstablishedSignal":     p.Signal,
		"sessionEstablishedTimeoutSec": p.HandshakeBudget,
		"input": map[string]any{
			"bytes":    p.InputBytes,
			"hash":     p.InputHash,
			"delivery": "stdin",
		},
		"startedAt":           nowISO(),
		"endedAt":             nil,
		"usage":               nil,
		"mirror":              nil,
		"chainClosed":         false,
		"runnerClosed":        false,
		"critiqueExhaustions": []any{},
	}
	return writeRecord(p.Output, record)
}

// BuildFollowRecordParams carries the inputs for a follow-up round record.
type BuildFollowRecordParams struct {
	Output          string
	Parent          string // parent (latest) record file
	Job             string
	Round           int64
	ParentJob       string
	Snapshot        string
	Fallbacks       string
	Signal          bool
	HandshakeBudget int64
	ResumeMode      string
	InputBytes      int64
	InputHash       string
	MissionTurn     string
	MainID          string
	ClaimEpoch      string
	CapResolution   string
}

// BuildFollowRecord assembles a follow-up round's pending record: chain
// identity inherited from the parent record, a fresh cap authority, and the
// resume mode the launch will use. Only a resumed round carries the parent's
// session id forward — a fresh-context round starts a new session.
func BuildFollowRecord(p BuildFollowRecordParams) error {
	parent, err := readObject(p.Parent)
	if err != nil {
		return fmt.Errorf("cannot read the parent record: %v", err)
	}
	for _, key := range []string{"role", "mission", "runtime", "reviews", "workspaceRoot", "baseSha", "branch", "permissions", "requestedModel"} {
		if _, present := parent[key]; !present {
			return fmt.Errorf("parent record is missing %s", key)
		}
	}
	requested, ok := parent["permissions"].(map[string]any)["requested"]
	if !ok {
		return fmt.Errorf("parent record has no permissions.requested object")
	}
	authority, err := readCapAuthority(p.CapResolution)
	if err != nil {
		return err
	}
	fallbacks, err := decodeJSONValue([]byte(p.Fallbacks))
	if err != nil {
		return fmt.Errorf("invalid capability fallbacks: %v", err)
	}
	epoch, err := nullableEpoch(p.ClaimEpoch)
	if err != nil {
		return err
	}
	var session any
	if p.ResumeMode == "resumed" {
		var present bool
		if session, present = parent["sessionId"]; !present {
			return fmt.Errorf("parent record has no session id to resume")
		}
	}
	record := map[string]any{
		"role":           parent["role"],
		"mission":        parent["mission"],
		"runtime":        parent["runtime"],
		"reviews":        parent["reviews"],
		"workspaceRoot":  parent["workspaceRoot"],
		"baseSha":        parent["baseSha"],
		"branch":         parent["branch"],
		"requestedModel": parent["requestedModel"],
		"jobId":          p.Job,
		"round":          p.Round,
		"parentJob":      p.ParentJob,
		"status":         "pending",
		"phase":          "handshake",
		"error":          nil,
		"mainId":         nullableString(p.MainID),
		"claimEpoch":     epoch,
		"permissions": map[string]any{
			"requested":           requested,
			"effective":           nil,
			"enforcementSnapshot": p.Snapshot,
		},
		"pid":                          nil,
		"pidStartedAt":                 nil,
		"pgid":                         nil,
		"custodyProcesses":             []any{},
		"instanceTag":                  "metasystem-job-" + p.Job,
		"sessionId":                    session,
		"turnId":                       nullableString(p.MissionTurn),
		"capMin":                       authority.capMin,
		"capDeadline":                  authority.deadline,
		"capResolution":                authority.resolutionField(),
		"effectiveModel":               nil,
		"overridden":                   false,
		"capabilitySnapshot":           p.Snapshot,
		"capabilityFallbacks":          fallbacks,
		"sessionEstablishedSignal":     p.Signal,
		"sessionEstablishedTimeoutSec": p.HandshakeBudget,
		"resumeMode":                   p.ResumeMode,
		"input": map[string]any{
			"bytes":    p.InputBytes,
			"hash":     p.InputHash,
			"delivery": "stdin",
		},
		"startedAt": nowISO(),
		"endedAt":   nil,
		"usage":     nil,
		"mirror":    nil,
	}
	return writeRecord(p.Output, record)
}
