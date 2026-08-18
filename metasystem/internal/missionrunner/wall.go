package missionrunner

// The tree equation (host-implementer wall, HIW-O3): after EVERY host
// exit — accepted, rejected, capped, failed, or never launched — the
// shippable projection must equal the anchored pre-tree plus the exact
// authorized patches this turn consumed plus the contract-declared
// host-artifact delta, and nothing else. A mismatch never reaches
// measurement or any completion-gate success: the evidence is persisted,
// the workspace is tainted, and the mission parks for a human.

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/validate"
)

// wallInspection is the tree equation's outcome: the three trees, the
// consumed authorization order, and the violation (empty on a pass).
type wallInspection struct {
	PreTree        string
	ExpectedTree   string
	PostTree       string
	OrderedDigests []string
	Violation      string
}

// document renders the inspection for wall.json — the turn-dir evidence —
// carrying the violation beside the payload when there is one.
func (w *wallInspection) document() map[string]any {
	verdict := "passed"
	if w.Violation != "" {
		verdict = "violated"
	}
	digests := make([]any, 0, len(w.OrderedDigests))
	for _, digest := range w.OrderedDigests {
		digests = append(digests, digest)
	}
	doc := map[string]any{
		"verdict": verdict, "preTree": w.PreTree, "expectedTree": w.ExpectedTree,
		"postTree": w.PostTree, "orderedDigests": digests,
	}
	if w.Violation != "" {
		doc["violation"] = w.Violation
	}
	return doc
}

// The protected-path table (design HIW: denied always, even inside
// otherwise host-declared locations): the instruction machinery the wall
// rides on, the signed mission contracts, and the instruction ledgers.
var protectedArtifactPrefixes = []string{"scripts/agents/"}

var protectedArtifactFiles = map[string]bool{
	"plans/goals.md":              true,
	"plans/goals-accepted.json":   true,
	"plans/instruction-ledger.md": true,
	"plans/known-issues.md":       true,
}

// protectedArtifactPath reports whether the wall's protected-path table
// denies a declaration: exact files, protected prefixes, and the signed
// mission contracts (plans/mission-*.contract.md).
func protectedArtifactPath(path string) bool {
	if protectedArtifactFiles[path] {
		return true
	}
	for _, prefix := range protectedArtifactPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return strings.HasPrefix(path, "plans/mission-") && strings.HasSuffix(path, ".contract.md")
}

// parseHostArtifacts parses the contract's declared host-artifact files:
// comma-separated canonical repository-relative FILES — default deny, no
// globs, no traversal, no protected paths. The returned violation string
// is a wall refusal, not a runner error: a contract declaring an unlawful
// artifact set fails the equation, never the process.
func parseHostArtifacts(value string) (map[string]bool, string) {
	declared := map[string]bool{}
	if strings.TrimSpace(value) == "" {
		return declared, ""
	}
	for _, raw := range strings.Split(value, ",") {
		path := strings.TrimSpace(raw)
		switch {
		case path == "":
			return nil, "contract wall.host-artifacts declares an empty path"
		case strings.HasPrefix(path, "/"), strings.Contains(path, ".."), strings.Contains(path, "\\"):
			return nil, fmt.Sprintf("contract wall.host-artifacts path %q is not a canonical repository-relative file", path)
		case strings.ContainsAny(path, "*?["):
			return nil, fmt.Sprintf("contract wall.host-artifacts path %q is a glob; only exact files may be declared", path)
		}
		if protectedArtifactPath(path) {
			return nil, fmt.Sprintf("contract wall.host-artifacts declares the protected path %q", path)
		}
		declared[path] = true
	}
	return declared, ""
}

// inspectWall proves the tree equation for one concluded turn: the post
// snapshot equals the pre-tree plus each consumed authorization's exact
// patch (pairwise disjoint, applied with no fuzz) plus a delta touching
// only declared host-artifact files that no consumed patch touched. Any
// failure to PROVE is a violation; an error is the runner's own (git
// unavailable, unreadable workspace), never a judgment.
func inspectWall(root, missionID, preTree string, state map[string]any, certified []map[string]any, declared map[string]bool, declarationViolation string) (*wallInspection, error) {
	workspace := gittree.Workspace{Dir: root}
	inspection := &wallInspection{PreTree: preTree, ExpectedTree: preTree, OrderedDigests: []string{}}
	postTree, err := wallSnapshot(workspace, missionID)
	if err != nil {
		return nil, failf(3, "wall inspection cannot snapshot the workspace: %v", err)
	}
	inspection.PostTree = postTree
	if declarationViolation != "" {
		inspection.Violation = declarationViolation
		return inspection, nil
	}

	authDir := filepath.Join(missionDirPath(root, missionID), "authorizations")
	currentSequence, currentSegment := mission.CurrentSequencePoint(state)
	namedPoints := mission.ExpectedTreePoints(state)
	consumedPaths := map[string]string{}
	expected := preTree
	for _, entry := range certified {
		if verdict, _ := entry["verdict"].(string); verdict != "accepted" {
			continue
		}
		digest, _ := entry["authorizationDigest"].(string)
		record, err := readJSONDoc(filepath.Join(authDir, digest+".json"))
		if err != nil {
			inspection.Violation = fmt.Sprintf("consumed authorization %.12s record is unreadable", digest)
			return inspection, nil
		}
		// Authenticate before trusting any field (round-2 finding 1): a
		// coordinated rewrite of reviewedTree/changedPaths/patchDigest
		// beside matching patch bytes must die here, not pass everywhere.
		if recomputed, err := validate.AuthorizationRecordDigest(record); err != nil || recomputed != digest {
			inspection.Violation = fmt.Sprintf("authorization %.12s record bytes do not match their digest", digest)
			return inspection, nil
		}
		changed, _ := record["changedPaths"].([]any)
		changedPaths := make([]string, 0, len(changed))
		for _, item := range changed {
			path, _ := item.(string)
			if prior, taken := consumedPaths[path]; taken {
				inspection.Violation = fmt.Sprintf("authorizations %.12s and %.12s overlap on %s; a combined authorization is required", prior, digest, path)
				return inspection, nil
			}
			consumedPaths[path] = digest
			changedPaths = append(changedPaths, path)
		}
		// The staleness predicate (HIW-R3-01/R5-01, critique F-2): the
		// authorization's base must BE a named E-sequence point — matched
		// by occurrence identity, not tree id alone — and either it IS
		// the current expected tree, or every accepted change between
		// them is disjoint from this authorization's paths.
		baseTree, _ := record["baseTree"].(string)
		basePoint, _ := record["baseSequencePoint"].(map[string]any)
		baseSequence, sOK := jsonInt(basePoint["sequence"])
		baseSegment, gOK := jsonInt(basePoint["segment"])
		if !sOK || !gOK {
			inspection.Violation = fmt.Sprintf("authorization %.12s binds no base sequence point", digest)
			return inspection, nil
		}
		// The segment fence fires FIRST (round-3 finding 3): a resolution
		// advanced the live segment, so every authorization from an older
		// segment refuses — even when RESTORE repeated its base tree.
		if baseSegment != currentSegment {
			inspection.Violation = fmt.Sprintf("authorization %.12s predates a workspace resolution; fresh conformance is required", digest)
			return inspection, nil
		}
		current := baseTree == preTree && baseSequence == currentSequence
		if !current {
			named := false
			for _, point := range namedPoints {
				if point.Sequence == baseSequence && point.Segment == baseSegment {
					named = point.Tree == baseTree
					break
				}
			}
			if !named {
				inspection.Violation = fmt.Sprintf("authorization %.12s base is not a named expected-tree sequence point", digest)
				return inspection, nil
			}
			// EVERY accepted change since the named point, turn by turn
			// (round-2 finding 4): an endpoint diff collapses a path that
			// changed and later reverted, exactly the ambiguity occurrence
			// identity exists to catch.
			stale := ""
			for _, raw := range turnLogOf(state) {
				entry, _ := raw.(map[string]any)
				wall, _ := entry["wall"].(map[string]any)
				point, _ := wall["sequencePoint"].(map[string]any)
				pointSequence, sOK := jsonInt(point["sequence"])
				pointSegment, gOK := jsonInt(point["segment"])
				if !sOK || !gOK || pointSequence <= baseSequence {
					continue
				}
				if pointSegment != baseSegment {
					stale = "a workspace resolution intervened"
					break
				}
				turnPre, _ := wall["preTree"].(string)
				turnPost, _ := wall["postTree"].(string)
				accepted, err := workspace.ChangedPaths(turnPre, turnPost)
				if err != nil {
					return nil, failf(3, "wall inspection cannot diff an intervening turn: %v", err)
				}
				for _, path := range accepted {
					if _, mine := consumedPathOf(changedPaths, path); mine {
						stale = "accepted changes since its base touch " + path
						break
					}
				}
				if stale != "" {
					break
				}
			}
			if stale != "" {
				inspection.Violation = fmt.Sprintf("authorization %.12s is stale: %s; fresh conformance is required", digest, stale)
				return inspection, nil
			}
		}
		patch, err := os.ReadFile(filepath.Join(authDir, digest+".patch"))
		if err != nil {
			inspection.Violation = fmt.Sprintf("consumed authorization %.12s patch bytes are missing", digest)
			return inspection, nil
		}
		// The patch bytes authenticate against the digest issuance bound
		// (critique F-3): a swapped .patch beside an intact record must
		// never let host-authored bytes satisfy the equation.
		recordedPatchDigest, _ := record["patchDigest"].(string)
		patchSum := sha256.Sum256(patch)
		if hex.EncodeToString(patchSum[:]) != recordedPatchDigest {
			inspection.Violation = fmt.Sprintf("consumed authorization %.12s patch bytes do not match the issued patchDigest", digest)
			return inspection, nil
		}
		applied, err := workspace.Apply(expected, patch)
		if err != nil {
			inspection.Violation = fmt.Sprintf("authorized patch %.12s does not apply cleanly to the expected tree: %v", digest, err)
			return inspection, nil
		}
		// Exactly the reviewed bytes (r5 HIW-R5-01's closing equality):
		// every entry this authorization changed must carry the SAME
		// object id and git mode as in the reviewed tree — a hunk that
		// exact-applies while the file drifted elsewhere refuses here.
		reviewedTree, _ := record["reviewedTree"].(string)
		wantEntries, err := workspace.Entries(reviewedTree, changedPaths)
		if err != nil {
			inspection.Violation = fmt.Sprintf("authorization %.12s reviewed tree is unreadable: %v", digest, err)
			return inspection, nil
		}
		gotEntries, err := workspace.Entries(applied, changedPaths)
		if err != nil {
			return nil, failf(3, "wall inspection cannot read the applied tree: %v", err)
		}
		for _, path := range changedPaths {
			if wantEntries[path] != gotEntries[path] {
				inspection.Violation = fmt.Sprintf("authorization %.12s applied, but %s does not carry the reviewed object id and mode", digest, path)
				return inspection, nil
			}
		}
		expected = applied
		inspection.ExpectedTree = expected
		inspection.OrderedDigests = append(inspection.OrderedDigests, digest)
	}

	// A declared artifact path must be reachable without traversing a
	// symlink (round-2 finding 6): a declared path beneath a symlinked
	// ancestor writes OUTSIDE the repository while producing no tree
	// delta, so the tree equation alone would never see it.
	if len(declared) > 0 {
		probe := []string{}
		for path := range declared {
			for _, ancestor := range ancestorPaths(path) {
				probe = append(probe, ancestor)
			}
			probe = append(probe, path)
		}
		entries, err := workspace.Entries(postTree, probe)
		if err != nil {
			return nil, failf(3, "wall inspection cannot read declared artifact ancestry: %v", err)
		}
		for _, path := range probe {
			if entries[path].Mode == "120000" {
				inspection.Violation = fmt.Sprintf("declared host artifact path traverses the symlink %s", path)
				return inspection, nil
			}
		}
	}

	if expected == postTree {
		return inspection, nil
	}
	delta, err := workspace.ChangedPaths(expected, postTree)
	if err != nil {
		return nil, failf(3, "wall inspection cannot diff the expected tree: %v", err)
	}
	for _, path := range delta {
		if !declared[path] {
			inspection.Violation = fmt.Sprintf("undeclared host-authored change: %s (the workspace differs from pre-tree + authorized patches on a path the contract does not declare)", path)
			return inspection, nil
		}
		if digest, taken := consumedPaths[path]; taken {
			inspection.Violation = fmt.Sprintf("declared host artifact %s overwrites bytes reviewed under authorization %.12s", path, digest)
			return inspection, nil
		}
	}
	return inspection, nil
}

// turnLogOf reads the state's turn log, tolerating absence.
func turnLogOf(state map[string]any) []any {
	log, _ := state["turnLog"].([]any)
	return log
}

// ancestorPaths lists every proper ancestor of a repository-relative path.
func ancestorPaths(path string) []string {
	ancestors := []string{}
	for i, r := range path {
		if r == '/' {
			ancestors = append(ancestors, path[:i])
		}
	}
	return ancestors
}

// missionLedgerRel is the mission's own bookkeeping path — excluded from
// the wall's tree identity (legacy branches carry historical on-branch
// anchor commits that force-tracked it) and guarded byte-for-byte against
// the anchor ref instead (round-3 findings 1 and 2; round-4 finding 1).
func missionLedgerRel(missionID string) string {
	return "artifacts/agents/missions/" + missionID + "/ledger.md"
}

// wallSnapshot projects the workspace into the wall's identity space: the
// shippable snapshot with the mission's own ledger filtered out.
func wallSnapshot(workspace gittree.Workspace, missionID string) (string, error) {
	tree, err := workspace.Snapshot("HEAD")
	if err != nil {
		return "", err
	}
	return workspace.FilterTree(tree, []string{missionLedgerRel(missionID)})
}

// guardLedgerInTurn proves the mission ledger is byte-identical to the
// AUTHENTICATED anchored truth while a turn is in flight (round-4
// finding 2): the baseline comes from the runner-owned anchor ref with
// every cross-check — state hash, cycle, path, sha — never from whatever
// commit last touched a path, so a host committing its own alteration
// can never become its own baseline. No legitimate writer touches the
// ledger between open and conclusion, so ANY difference is a violation.
// At resume the guard does not run: reconciliation just verified the
// anchored truth (including the legitimate ledger-ahead single-append)
// through the same machinery.
func (e *Engine) guardLedgerInTurn(state map[string]any, ledgerPath string) (string, error) {
	anchored, current, err := mission.AnchoredLedgerTruth(e.Root, state, ledgerPath)
	if errors.Is(err, mission.ErrNoAnchor) {
		return "", nil
	}
	if err != nil {
		return "mission ledger disagrees with the anchored truth: " + err.Error(), nil
	}
	if anchored != current {
		return "mission ledger bytes were modified during the turn", nil
	}
	return "", nil
}

// consumedPathOf reports whether path is one of the authorization's own
// changed paths (the staleness intersection).
func consumedPathOf(changedPaths []string, path string) (string, bool) {
	for _, candidate := range changedPaths {
		if candidate == path {
			return candidate, true
		}
	}
	return "", false
}

// wallGate runs the inspection for a concluding turn and, on violation,
// executes the refusal in the binding order: evidence, ledger booking for
// the reserved cycle, then ONE state write carrying the taint entry and
// the park — before any measurement or completion-gate path can run. The
// bool reports whether the turn was intercepted.
func (e *Engine) wallGate(statePath, ledger, turnID, turnDir string, cycle int64, certified []map[string]any, atResume bool) (map[string]any, bool, error) {
	diskState, err := readDocLabeled(statePath, "mission state", 3)
	if err != nil {
		return nil, false, err
	}
	openTurn, ok := diskState["openTurn"].(map[string]any)
	if !ok {
		return nil, false, failf(3, "wall inspection needs the open-turn marker; this turn was opened by a pre-wall runner — conclude it with that runner or re-provision the mission")
	}
	preTree, _ := openTurn["preTree"].(string)
	_, values, _, err := e.parseContract(true)
	if err != nil {
		return nil, false, err
	}
	declared, declarationViolation := parseHostArtifacts(values["wall.host-artifacts"])
	// The ledger guard replaces the tree identity the filter removed
	// (round-3/round-4): in-turn, live bytes must equal the anchored
	// truth exactly; at resume, reconciliation has already verified it.
	if declarationViolation == "" && !atResume {
		guardViolation, err := e.guardLedgerInTurn(diskState, ledger)
		if err != nil {
			return nil, false, err
		}
		if guardViolation != "" {
			declarationViolation = guardViolation
		}
	}
	inspection, err := inspectWall(e.Root, e.Mission, preTree, diskState, certified, declared, declarationViolation)
	if err != nil {
		return nil, false, err
	}
	// Every tree the evidence names stays reachable (critique F-7):
	// garbage collection must never eat a tree that acceptance entries,
	// violation evidence, or a later staleness check will dereference.
	workspace := gittree.Workspace{Dir: e.Root}
	for _, tree := range []string{inspection.ExpectedTree, inspection.PostTree} {
		if tree == "" || tree == preTree {
			continue
		}
		if err := workspace.Anchor(e.Mission, tree); err != nil {
			return nil, false, failf(3, "wall inspection cannot anchor %s: %v", tree, err)
		}
	}
	if err := atomicWriteJSON(filepath.Join(turnDir, "wall.json"), inspection.document()); err != nil {
		return nil, false, err
	}
	if inspection.Violation == "" {
		return nil, false, nil
	}

	e.emit("wall-violation", clipSummary(inspection.Violation), map[string]string{
		"missionId": e.Mission, "turnId": turnID, "error": inspection.Violation,
	})
	if _, err := patchTurn(filepath.Join(turnDir, "turn.json"), map[string]any{
		"status": "failed", "outcome": "wall-violation", "error": "wall-violation",
		"detail": inspection.Violation, "endedAt": nowISO(),
	}); err != nil {
		return nil, false, err
	}
	branch, _ := diskState["branch"].(string)
	candidateSHA, err := e.gitRevParse(branch)
	if err != nil {
		return nil, false, err
	}
	// A ledger-ahead crash already booked this cycle (round-2 finding 3):
	// the block is the narrative truth and appending it twice would refuse
	// on contiguity BEFORE the taint and park could land.
	alreadyBooked := false
	if _, _, cycles, perr := mission.ParseLedger(ledger); perr == nil && int64(len(cycles)) >= cycle {
		alreadyBooked = true
	}
	if !alreadyBooked {
		observed := "unmeasurable:" + strings.ReplaceAll("wall violation: "+inspection.Violation, "\n", " ")
		if err := e.appendLedger(diskState, ledger, cycle, "no-progress", candidateSHA, observed, nil); err != nil {
			return nil, false, err
		}
	}
	current, err := readDocLabeled(statePath, "mission state", 3)
	if err != nil {
		return nil, false, err
	}
	outcome, err := ParkProposal(e.Root, e.Mission, current, "wall-violation", nowISO())
	if err != nil {
		return nil, false, err
	}
	if err := setLedgerCycles(outcome.State, cycle); err != nil {
		return nil, false, err
	}
	if err := appendTaintEntry(outcome.State, turnID, inspection.Violation); err != nil {
		return nil, false, err
	}
	final, err := e.applyPark(statePath, ledger, turnID, outcome)
	return final, true, err
}

// appendTaintEntry books the violation in the monotonic taint ledger —
// in the same proposal as the park, so the taint and the stop are one
// write. Resolution stays null: only a human's typed RESTORE or
// ADOPT_DISPUTED_TREE ever clears it.
func appendTaintEntry(state map[string]any, turnID, reason string) error {
	taint, ok := state["workspaceTaint"].(map[string]any)
	if !ok {
		return failf(3, "mission state carries no workspaceTaint ledger")
	}
	next, ok := jsonInt(taint["next"])
	if !ok || next < 1 {
		return failf(3, "mission workspaceTaint next id is invalid")
	}
	entries, ok := taint["entries"].([]any)
	if !ok {
		return failf(3, "mission workspaceTaint entries are invalid")
	}
	taint["entries"] = append(entries, map[string]any{
		"taintId": next, "turnId": turnID, "reason": reason,
		"setAt": nowISO(), "resolution": nil,
	})
	taint["next"] = next + 1
	return nil
}

// unresolvedTaint names the first unresolved taint entry, empty when the
// workspace is clean: the resume-time STOP — a tainted mission never
// opens another turn until a human resolution clears the taint.
func unresolvedTaint(state map[string]any) string {
	taint, _ := state["workspaceTaint"].(map[string]any)
	entries, _ := taint["entries"].([]any)
	for _, item := range entries {
		entry, _ := item.(map[string]any)
		if entry != nil && entry["resolution"] == nil {
			reason, _ := entry["reason"].(string)
			return reason
		}
	}
	return ""
}
