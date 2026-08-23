package missionrunner

// The tree equation (the host-implementer wall): after EVERY host
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
	"sort"
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
	Unaccounted    []string
	// UndeclaredOnly marks the one violation class the recovery ladder's
	// mechanical rung may touch: workspace bytes diverging from the
	// composed expected tree on undeclared paths. Every other class —
	// declaration, ledger, authorization accounting, carriers — is set
	// with this false and stays on the human path.
	UndeclaredOnly bool
	// Auths carries the authenticated consumed-authorization facts for
	// the snapshot-scope accountant (decomposition membership).
	Auths []scopeAuth
	// Scope carries the snapshot-scope observables for wall.json, and
	// Posture the recorded acceptance posture block.
	Scope   map[string]any
	Posture map[string]any
	// Recovered, on a pass, is the recovery record riding the evidence
	// into the acceptance chain: the violation that was mechanically
	// restored before this pass verdict, and what the restore touched.
	Recovered map[string]any
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
	if w.Scope != nil {
		doc["scope"] = w.Scope
	}
	if w.Posture != nil {
		doc["posture"] = w.Posture
	}
	if w.Violation != "" {
		doc["violation"] = w.Violation
		unaccounted := make([]any, 0, len(w.Unaccounted))
		for _, path := range w.Unaccounted {
			unaccounted = append(unaccounted, path)
		}
		doc["unaccounted"] = unaccounted
	} else if w.Recovered != nil {
		doc["recovered"] = w.Recovered
	}
	return doc
}

// The protected-path table (denied always, even inside
// otherwise host-declared locations): the instruction machinery the wall
// rides on, the signed mission contracts, and the instruction ledgers.
// plans/goals/ covers the multi-machine ledger whole — the live
// set AND the done/ archive: goal files change only
// through goal verbs, never through a mission's host artifacts.
var protectedArtifactPrefixes = []string{"scripts/agents/", "plans/goals/"}

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
func inspectWall(root, missionID, preTree string, state map[string]any, certified []map[string]any, declared map[string]bool, declarationViolation string, snapshot func(expected string) (string, error)) (*wallInspection, error) {
	workspace := gittree.Workspace{Dir: root}
	inspection := &wallInspection{PreTree: preTree, ExpectedTree: preTree, OrderedDigests: []string{}}
	if declarationViolation != "" {
		inspection.Violation = declarationViolation
		return inspection, nil
	}

	authDir := filepath.Join(missionDirPath(root, missionID), "authorizations")
	currentSequence, currentSegment := mission.CurrentSequencePoint(state)
	namedPoints := mission.ExpectedTreePoints(state)
	// The intervening-change set is ONE chronological list:
	// acceptance deltas turn by turn, PLUS each
	// resolution's own delta — every path that differs between the
	// pre-resolution expected tree and the ruled tree. The
	// rule is path-sensitive: a blanket segment fence would refuse
	// whole segments of work a resolution never touched.
	type interveningDelta struct {
		sequence int64
		from, to string
		kind     string
	}
	deltas := []interveningDelta{}
	for _, raw := range turnLogOf(state) {
		entry, _ := raw.(map[string]any)
		wall, _ := entry["wall"].(map[string]any)
		point, _ := wall["sequencePoint"].(map[string]any)
		pointSequence, ok := jsonInt(point["sequence"])
		if !ok {
			continue
		}
		turnPre, _ := wall["preTree"].(string)
		turnPost, _ := wall["postTree"].(string)
		deltas = append(deltas, interveningDelta{pointSequence, turnPre, turnPost, "accepted changes"})
	}
	stateTaint, _ := state["workspaceTaint"].(map[string]any)
	for _, raw := range func() []any { entries, _ := stateTaint["entries"].([]any); return entries }() {
		entry, _ := raw.(map[string]any)
		if entry == nil {
			continue
		}
		resolution, _ := entry["resolution"].(map[string]any)
		if resolution == nil {
			continue
		}
		point, _ := resolution["sequencePoint"].(map[string]any)
		pointSequence, ok := jsonInt(point["sequence"])
		if !ok {
			continue
		}
		from, _ := resolution["previousTree"].(string)
		to, _ := resolution["treeId"].(string)
		deltas = append(deltas, interveningDelta{pointSequence, from, to, "a workspace resolution"})
	}
	sort.SliceStable(deltas, func(i, j int) bool { return deltas[i].sequence < deltas[j].sequence })
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
		// Authenticate before trusting any field: a
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
		// The staleness predicate: the
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
		current := baseTree == preTree && baseSequence == currentSequence && baseSegment == currentSegment
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
			// EVERY intervening change since the named point — accepted
			// turns AND resolutions, occurrence by
			// occurrence: an endpoint diff collapses a path
			// that changed and later reverted, and a blanket fence
			// refuses work a resolution never touched.
			stale := ""
			for _, delta := range deltas {
				if delta.sequence <= baseSequence {
					continue
				}
				changed, err := workspace.ChangedPaths(delta.from, delta.to)
				if err != nil {
					return nil, failf(3, "wall inspection cannot diff an intervening change: %v", err)
				}
				for _, path := range changed {
					if _, mine := consumedPathOf(changedPaths, path); mine {
						stale = delta.kind + " since its base touch " + path
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
		// The patch bytes authenticate against the digest issuance bound:
		// a swapped .patch beside an intact record must
		// never let host-authored bytes satisfy the equation.
		recordedPatchDigest, _ := record["patchDigest"].(string)
		patchSum := sha256.Sum256(patch)
		if hex.EncodeToString(patchSum[:]) != recordedPatchDigest {
			inspection.Violation = fmt.Sprintf("consumed authorization %.12s patch bytes do not match the issued patchDigest", digest)
			return inspection, nil
		}
		applied, err := workspace.Apply(expected, patch)
		if err != nil {
			// Could-not-run is the runner's own: only git's
			// ran-and-refused answer indicts the authorization evidence.
			var runFailure *gittree.RunFailure
			if errors.As(err, &runFailure) {
				return nil, failf(3, "wall inspection could not run the authorization patch: %v", err)
			}
			inspection.Violation = fmt.Sprintf("authorized patch %.12s does not apply cleanly to the expected tree: %v", digest, err)
			return inspection, nil
		}
		// Exactly the reviewed bytes — the equation's closing equality:
		// every entry this authorization changed must carry the SAME
		// object id and git mode as in the reviewed tree — a hunk that
		// exact-applies while the file drifted elsewhere refuses here.
		reviewedTree, _ := record["reviewedTree"].(string)
		wantEntries, err := workspace.Entries(reviewedTree, changedPaths)
		if err != nil {
			var runFailure *gittree.RunFailure
			if errors.As(err, &runFailure) {
				return nil, failf(3, "wall inspection could not read the reviewed tree: %v", err)
			}
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
		inspection.Auths = append(inspection.Auths, scopeAuth{
			digest: digest, changedPaths: changedPaths,
			reviewedTree: reviewedTree, reviewedEntries: wantEntries,
		})
	}

	// The worktree projection seeds FROM THE COMPARISON TARGET (the fully
	// composed expected tree), so tracked-and-ignored membership follows
	// the comparison's own right-hand side.
	// The callback's error passes through UNWRAPPED: the caller types it
	// (a repository answer parks; a could-not-run failure stays the
	// runner's).
	postTree, err := snapshot(expected)
	if err != nil {
		return nil, err
	}
	inspection.PostTree = postTree

	// A declared artifact path must be reachable without traversing a
	// symlink: a declared path beneath a symlinked
	// ancestor writes OUTSIDE the repository while producing no tree
	// delta, so the tree equation alone would never see it.
	if len(declared) > 0 {
		probe := []string{}
		for path := range declared {
			probe = append(probe, ancestorPaths(path)...)
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
			inspection.UndeclaredOnly = true
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
// the anchor ref instead.
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

// wallPreflight enforces the wall's REPOSITORY PRECONDITIONS before any
// turn exists: core.fileMode is explicitly pinned true — a
// fileMode-off repository hides mode-bit drift from the tree equation —
// and, at mission START, the initial filtered projection is CLEAN
// (equal to HEAD's filtered tree) or exactly the tree the human sealed
// as wall.sealed-baseline in the signed contract. Violations refuse
// before the first turn ever opens.
// admittedBaseline runs the START baseline admission and returns the
// admitted filtered tree: clean (equal to HEAD's projection) or exactly
// the human-sealed wall.sealed-baseline. Shared by the launch preflight
// and state initialization, which RECORDS what it admits as E0.
// contractShapeRefusal stats the live contract WITHOUT dereferencing and
// refuses anything that exists but is not a regular file. A symlink is
// never the signed contract: hash-object and Stat both dereference, so a
// link to a sealed copy in ignored space would present the approved
// bytes while E0 records an unsigned 120000 entry pointing at mutable
// bytes. Any other non-regular object (a FIFO above all) would HANG the
// blocking contract read instead of refusing. Honest absence falls
// through: an absent contract is contract preflight's refusal to name.
func contractShapeRefusal(contractAbs string) error {
	info, err := os.Lstat(contractAbs)
	if err != nil || info.Mode().IsRegular() {
		return nil
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return failf(3, "wall preflight refused: the mission contract is a symlink; the signed contract must be a regular committed file")
	}
	return failf(3, "wall preflight refused: the mission contract path is occupied by a non-regular object (%s); the signed contract must be a regular committed file", info.Mode())
}

func (e *Engine) checkFileModePinned() error {
	// The invariant is an explicit REPOSITORY-LOCAL pin normalized to
	// git's own boolean: any spelling git reads as true satisfies it
	// (strictness guards the invariant, not a spelling), and a global
	// or system value satisfies nothing — this repository must carry
	// the pin itself.
	stdout, _, code := gitCaptured(e.Root, "config", "--local", "--type=bool", "--get", "core.fileMode")
	if code != 0 || strings.TrimSpace(stdout) != "true" {
		return failf(3, "wall preflight refused: core.fileMode is not pinned true in this repository; run `git config core.fileMode true` (mode-bit drift must be visible to the tree equation)")
	}
	return nil
}

func (e *Engine) admittedBaseline(values map[string]string, approved []byte) (string, error) {
	// The pin is re-checked at EVERY admission: the launching parent's
	// check ran in another process, and the gap between the two
	// processes is not trusted.
	if err := e.checkFileModePinned(); err != nil {
		return "", err
	}
	workspace := gittree.Workspace{Dir: e.Root}
	contractRel := filepath.Join("plans", "mission-"+e.Mission+".contract.md")
	exclude := []string{missionLedgerRel(e.Mission), contractRel}
	raw, err := workspace.Snapshot("HEAD")
	if err != nil {
		return "", failf(3, "wall preflight cannot snapshot the workspace: %v", err)
	}
	observed, err := workspace.FilterTree(raw, exclude)
	if err != nil {
		return "", failf(3, "wall preflight cannot project the workspace: %v", err)
	}
	// HEAD in the workspace's OWN path space: a raw HEAD^{tree} is the
	// toplevel tree, whose paths miss every workspace-relative lookup in
	// a nested checkout.
	headTree, err := workspace.HeadTree()
	if err != nil {
		return "", failf(3, "wall preflight cannot resolve HEAD's tree: %v", err)
	}
	committed, err := workspace.FilterTree(headTree, exclude)
	if err != nil {
		return "", failf(3, "wall preflight cannot project the committed tree: %v", err)
	}
	// The LIVE contract must be exactly the COMMITTED contract — bytes
	// AND mode. The exclusion below removes it from the dirt decision,
	// and the recorded E0 includes its live entry, so any divergence
	// here — an executable-bit flip, an edit in the preflight gap —
	// would otherwise ride into expected state as bytes nobody signed.
	// Both sides come from trees of the SAME instant: reading the live
	// file separately would leave a swap window where the snapshot
	// captures one contract, the check blesses another, and the
	// snapshotted one becomes E0.
	contractEntries, err := workspace.Entries(raw, []string{contractRel})
	if err != nil {
		return "", failf(3, "wall preflight cannot read the snapshot's contract entry: %v", err)
	}
	committedContract, err := workspace.Entries(headTree, []string{contractRel})
	if err != nil {
		return "", failf(3, "wall preflight cannot read the committed contract entry: %v", err)
	}
	live, liveExists := contractEntries[contractRel]
	if !liveExists {
		return "", failf(3, "wall preflight refused: the mission contract is absent from the workspace snapshot; the signed contract must be a regular committed file")
	}
	// A SYMLINK is never the signed contract: the snapshot records it as
	// a 120000 entry whose blob is the target path, which E0 would then
	// pin as bytes nobody signed. Named here from the same snapshot the
	// record uses; the public ladder names it even earlier
	// (lstatContractRefusingSymlink).
	if live.Mode == "120000" {
		return "", failf(3, "wall preflight refused: the mission contract is a symlink; the signed contract must be a regular committed file")
	}
	want := committedContract[contractRel]
	if want.OID != live.OID || want.Mode != live.Mode {
		return "", failf(3, "wall preflight refused: the mission contract differs from its committed form (bytes or mode); commit the signed contract exactly as approved")
	}
	// Local HEAD alone cannot vouch for the contract: a different
	// contract committed over HEAD after the pin would satisfy the
	// committed-form check while the mission runs the pinned one, and
	// E0 would record bytes nobody approved. The snapshot's entry must
	// BE the approved bytes — the verified snapshot at launch, the
	// pinned copy at birth.
	approvedOID, err := blobOID(e.Root, approved)
	if err != nil {
		return "", failf(3, "wall preflight cannot hash the approved contract bytes: %v", err)
	}
	if live.OID != approvedOID {
		return "", failf(3, "wall preflight refused: the workspace contract does not match the approved contract bytes; the mission must run exactly the contract that was pinned")
	}
	// The EFFECTIVE replacement namespace must be empty at admission and
	// stay empty: an active mapping re-routes later unpinned git
	// operations — the completion gate above all — to bytes the wall
	// never judged. Every runner surface pins useReplaceRefs off and
	// strips GIT_REPLACE_REF_BASE, so the effective namespace is
	// refs/replace.
	refs, err := workspace.RefMap()
	if err != nil {
		return "", failf(3, "wall preflight cannot enumerate refs: %v", err)
	}
	for name := range refs {
		if strings.HasPrefix(name, "refs/replace/") {
			return "", failf(3, "wall preflight refused: the replacement namespace is not empty (%s); remove the replace refs before any mission runs", name)
		}
	}
	// The DECISION compares contract-excluded projections (the fixpoint
	// break); the RECORDED baseline is the wall's own identity space —
	// the ledger-only filter every later E-point lives in — captured
	// from the SAME snapshot instant.
	record, err := workspace.FilterTree(raw, []string{missionLedgerRel(e.Mission)})
	if err != nil {
		return "", failf(3, "wall preflight cannot project the identity space: %v", err)
	}
	// Staged admission at mission start: the staged projection must
	// equal HEAD's filtered tree OR the admitted baseline itself — a
	// sealed mission whose index mirrors the sealed state is lawful (the
	// sealed tree is reviewable by equality); anything else is refused
	// like a dirty worktree, and a conflicted workspace index refuses
	// toward the wall.
	staged, serr := workspace.StagedTree()
	if serr != nil {
		if errors.Is(serr, gittree.ErrUnmergedWorkspaceIndex) {
			return "", failf(3, "wall preflight refused: %v; resolve or reset the index before any mission runs", serr)
		}
		return "", failf(3, "wall preflight cannot project the staged workspace: %v", serr)
	}
	stagedIdentity, err := workspace.FilterTree(staged, []string{missionLedgerRel(e.Mission)})
	if err != nil {
		return "", failf(3, "wall preflight cannot project the staged identity: %v", err)
	}
	committedIdentity, err := workspace.FilterTree(headTree, []string{missionLedgerRel(e.Mission)})
	if err != nil {
		return "", failf(3, "wall preflight cannot project the committed identity: %v", err)
	}
	if stagedIdentity != committedIdentity && stagedIdentity != record {
		return "", failf(3, "wall preflight refused: the staged projection %s equals neither HEAD's tree nor the admitted baseline; commit or reset the index", stagedIdentity)
	}
	if observed == committed {
		return record, nil
	}
	if sealed := values["wall.sealed-baseline"]; sealed == observed {
		return record, nil
	}
	return "", failf(3, "wall preflight refused: the initial baseline is dirty (filtered projection %s does not equal HEAD's %s); commit the difference, or seal wall.sealed-baseline=%s in the signed contract", observed, committed, observed)
}

func (e *Engine) wallPreflight(mode string, values map[string]string, approved []byte) error {
	if err := e.checkFileModePinned(); err != nil {
		return err
	}
	if mode != "start" {
		return nil
	}
	_, err := e.admittedBaseline(values, approved)
	return err
}

// blobOID names the git blob object a byte sequence would store as, in
// the repository's own hash algorithm.
func blobOID(root string, content []byte) (string, error) {
	tmp, err := os.CreateTemp("", "metasystem-blob-oid.*")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return "", err
	}
	tmp.Close()
	stdout, stderr, code := gitCaptured(root, "hash-object", "--no-filters", "--", tmp.Name())
	if code != 0 {
		return "", fmt.Errorf("git hash-object: %s", firstDetail(stderr, stdout))
	}
	return strings.TrimSpace(stdout), nil
}

// guardLedgerInTurn proves the mission ledger is byte-identical to the
// AUTHENTICATED anchored truth while a turn is in flight:
// the baseline comes from the runner-owned anchor ref with
// every cross-check — state hash, cycle, path, sha — never from whatever
// commit last touched a path, so a host committing its own alteration
// can never become its own baseline. No legitimate writer touches the
// ledger between open and conclusion, so ANY difference is a violation.
// At resume the guard does not run: reconciliation just verified the
// anchored truth (including the legitimate ledger-ahead single-append)
// through the same machinery.
// ledgerViolationPrefix marks every taint whose violation domain is the
// mission ledger — the file the wall's tree projection deliberately
// filters. resolve-taint refuses RESTORE for these by
// prefix: tree equality cannot prove a ledger restored.
const ledgerViolationPrefix = "mission ledger"

func (e *Engine) guardLedgerInTurn(state map[string]any, ledgerPath string) (string, error) {
	anchored, current, err := mission.AnchoredLedgerTruth(e.Root, state, ledgerPath)
	if errors.Is(err, mission.ErrNoAnchor) {
		return "", nil
	}
	if err != nil {
		// A git invocation that could not run is the RUNNER's failure —
		// only a ran-and-answered anchor disagreement is
		// the host's violation.
		var runFailure *gittree.RunFailure
		if errors.As(err, &runFailure) {
			return "", failf(3, "mission ledger guard could not run: %v", err)
		}
		return ledgerViolationPrefix + " disagrees with the anchored truth: " + err.Error(), nil
	}
	if anchored != current {
		return ledgerViolationPrefix + " bytes were modified during the turn", nil
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

// wallScopeContext hands a passed inspection's identity to the
// acceptance path: the composed expected tree, the declared paths, and
// the capture the payload records — so the write can re-verify that
// exactly what was judged is what publishes.
type wallScopeContext struct {
	PreTree        string
	Expected       string
	OrderedDigests []string
	Declared       map[string]bool
	Capture        *wallCapture
	OpenAnchor     string
	// Recovered carries the recovery record of a pass that was reached
	// through the mechanical rung (or inherited from this turn's prior
	// published pass), for the acceptance payload to book into the chain.
	Recovered map[string]any
}

// wallGate runs the inspection for a concluding turn and, on violation,
// executes the refusal in the binding order: evidence, ledger booking for
// the reserved cycle, then ONE state write carrying the taint entry and
// the park — before any measurement or completion-gate path can run. The
// bool reports whether the turn was intercepted; on a pass, the returned
// context carries the verified capture for the acceptance write.
// With allowRecovery set (the main conclusion only — never a resume
// re-drive, where the crash itself is the doubt that belongs to the
// human), a violation first tries the mechanical rung of the recovery
// ladder: see attemptWallRecovery. inPassRecovery is the recovery record
// of THIS process's earlier gate round (the acceptance stability loop
// hands its live context down on a re-run); it is the ONLY way a
// recovery record reaches a pass — rewritable evidence is never
// promoted into the chain.
func (e *Engine) wallGate(statePath, ledger, turnID, turnDir string, cycle int64, certified []map[string]any, atResume, allowRecovery bool, inPassRecovery map[string]any) (*wallScopeContext, map[string]any, bool, error) {
	diskState, err := readDocLabeled(statePath, "mission state", 3)
	if err != nil {
		return nil, nil, false, err
	}
	// Detected evidence is STICKY: a
	// wall.json already recording a violation is never re-inspected
	// into a pass — a crash between the evidence write and the park
	// re-executes the park with the recorded violation verbatim.
	// A prior document recording a PASS WITH a recovery block, reached
	// with no live in-pass record and no landed acceptance, is the other
	// crash tail: the recovery published its evidence but its anchored
	// record never landed. Evidence files are rewritable and prove
	// nothing forward, so the offense parks verbatim for the human —
	// never re-earns an automatic pass whose record already vanished
	// once.
	if prior, perr := readJSONDoc(filepath.Join(turnDir, "wall.json")); perr == nil {
		if recorded, _ := prior["violation"].(string); recorded != "" {
			// A persisted recovery context replays onto the ask beside the
			// verbatim violation: decoration only — the taint reason and
			// every state decision still come from the violation itself.
			priorNote, _ := prior["recovery"].(string)
			final, ferr := e.parkWallViolation(statePath, ledger, turnID, turnDir, cycle, recorded, diskState, true, priorNote)
			return nil, final, true, ferr
		}
		if verdict, _ := prior["verdict"].(string); verdict == "passed" && inPassRecovery == nil {
			if record, _ := prior["recovered"].(map[string]any); record != nil && acceptanceEntryFor(diskState, turnID) == nil {
				recorded, _ := record["violation"].(string)
				if recorded == "" {
					recorded = "a published recovery names no violation"
				}
				final, ferr := e.parkWallViolation(statePath, ledger, turnID, turnDir, cycle, recorded, diskState, true,
					"a mechanical recovery published its pass but the mission stopped before the record landed; the workspace was restored and re-verified once — verify it and resolve by name (a pre-tree restore loses nothing durable: unconsumed authorizations survive as records and re-drive after unpark)")
				return nil, final, true, ferr
			}
		}
	}
	inheritedRecovery := inPassRecovery
	var recoveryNote string
	openTurn, ok := diskState["openTurn"].(map[string]any)
	if !ok {
		return nil, nil, false, failf(3, "wall inspection needs the open-turn marker; this turn was opened by a pre-wall runner — conclude it with that runner or re-provision the mission")
	}
	preTree, _ := openTurn["preTree"].(string)
	_, values, _, err := e.parseContract(true)
	if err != nil {
		return nil, nil, false, err
	}
	declared, declarationViolation := parseHostArtifacts(values["wall.host-artifacts"])
	// The ledger guard replaces the tree identity the filter removed:
	// in-turn, live bytes must equal the anchored
	// truth exactly; at resume, reconciliation has already verified it.
	if declarationViolation == "" && !atResume {
		guardViolation, err := e.guardLedgerInTurn(diskState, ledger)
		if err != nil {
			return nil, nil, false, err
		}
		if guardViolation != "" {
			declarationViolation = guardViolation
		}
	}
	origin := scopeOriginFromOpenTurn(openTurn, diskState)
	workspace := gittree.Workspace{Dir: e.Root}
	inspection, capture, stable, err := e.runWallInspection(preTree, diskState, certified, declared, declarationViolation, origin)
	if err != nil {
		return nil, nil, false, err
	}
	// The tail runs once per inspection round. A violating round may earn
	// ONE mechanical recovery attempt (the rung below); its restore is
	// re-verified by a full fresh round over the whole posture — never by
	// rechecking only the paths the violation named.
	var recovered map[string]any
	attempted := false
	for {
		// Every tree the evidence names stays reachable:
		// garbage collection must never eat a tree that acceptance entries,
		// violation evidence, or a later staleness check will dereference.
		anchored := []string{inspection.ExpectedTree, inspection.PostTree}
		if capture != nil {
			anchored = append(anchored, capture.StagedTree)
			if capture.Nested {
				anchored = append(anchored, capture.TopTree, capture.TopStaged.Tree)
			}
		}
		for _, tree := range anchored {
			if tree == "" || tree == preTree {
				continue
			}
			if err := workspace.Anchor(e.Mission, tree); err != nil {
				return nil, nil, false, failf(3, "wall inspection cannot anchor %s: %v", tree, err)
			}
		}
		// Unaccounted paths ride the violation evidence:
		// the delta the equation could not explain.
		if inspection.Violation != "" && inspection.PostTree != "" {
			baseline := inspection.ExpectedTree
			if baseline == "" {
				baseline = preTree
			}
			if baseline != "" {
				if changed, derr := workspace.ChangedPaths(baseline, inspection.PostTree); derr == nil {
					inspection.Unaccounted = changed
				}
			}
		}
		if inspection.Violation == "" {
			if recovered == nil {
				recovered = inheritedRecovery
			}
			inspection.Recovered = recovered
		}
		if err := atomicWriteJSON(filepath.Join(turnDir, "wall.json"), inspection.document()); err != nil {
			return nil, nil, false, err
		}
		if inspection.Violation == "" {
			if attempted {
				e.emit("recovery-inspected", "wall violation mechanically restored and re-verified for turn "+turnID, map[string]string{
					"missionId": e.Mission, "turnId": turnID, "verdict": "recovered",
				})
			}
			return &wallScopeContext{
				PreTree: preTree, Expected: inspection.ExpectedTree,
				OrderedDigests: inspection.OrderedDigests,
				Declared:       declared, Capture: capture, OpenAnchor: origin.OpenAnchor,
				Recovered: inspection.Recovered,
			}, nil, false, nil
		}
		if !allowRecovery || attempted {
			if attempted {
				recoveryNote = "a mechanical restore was attempted and the whole-posture re-verification still refused: " + inspection.Violation
				e.emit("recovery-inspected", "mechanical restore did not re-verify clean; the taint stands: "+clipSummary(inspection.Violation), map[string]string{
					"missionId": e.Mission, "turnId": turnID, "verdict": "failed",
				})
			}
			break
		}
		attempted = true
		block, refusal, ok := e.attemptWallRecovery(inspection, capture, stable, declarationViolation, declared, origin, diskState, turnID)
		if !ok {
			recoveryNote = "the mechanical rung refused: " + refusal
			break
		}
		recovered = block
		if e.postRestoreHook != nil {
			// Test seam: the window between a successful restore and its
			// whole-posture re-verification — a late mutation landing here
			// must become a fresh violation, never a laundered pass.
			e.postRestoreHook()
		}
		inspection, capture, stable, err = e.runWallInspection(preTree, diskState, certified, declared, declarationViolation, origin)
		if err != nil {
			return nil, nil, false, err
		}
	}

	// The evidence tells the ladder's story too: when the rung was
	// consulted, the violating wall.json carries what was tried or why
	// it refused beside the violation the taint books — the turn dir
	// answers the whole question without a walk through the event
	// stream. Sticky crash tails never rewrite evidence; their context
	// rides the ask alone.
	// Best-effort by contract: the sticky violating document is already
	// durable, and a failed enrichment must never convert "park and
	// taint" into a runner error — the note still reaches the ask.
	if recoveryNote != "" {
		doc := inspection.document()
		doc["recovery"] = recoveryNote
		_ = atomicWriteJSON(filepath.Join(turnDir, "wall.json"), doc)
	}
	final, err := e.parkWallViolation(statePath, ledger, turnID, turnDir, cycle, inspection.Violation, diskState, true, recoveryNote)
	return nil, final, true, err
}

// runWallInspection is the one-resolution-per-inspection loop: the posture
// is captured once, every rule judges that capture, and the same capture
// is re-taken and compared whole before the verdict is acted on. A changed
// capture re-runs the inspection; a repository that will not hold still is
// itself a violation. The returned stable flag reports whether the verdict
// was judged over bytes that held still.
func (e *Engine) runWallInspection(preTree string, diskState map[string]any, certified []map[string]any, declared map[string]bool, declarationViolation string, origin *scopeOrigin) (*wallInspection, *wallCapture, bool, error) {
	var inspection *wallInspection
	var capture *wallCapture
	stable := false
	for attempt := 0; attempt < 3 && !stable; attempt++ {
		capture = nil
		var err error
		inspection, err = inspectWall(e.Root, e.Mission, preTree, diskState, certified, declared, declarationViolation,
			func(expected string) (string, error) {
				snapped, cerr := e.captureWallPostureStable(expected, declared)
				if cerr != nil {
					return "", cerr
				}
				capture = snapped
				return snapped.Post, nil
			})
		if err != nil {
			// A repository ANSWER that defeats capture is a violation —
			// the host had custody — while could-not-run failures keep
			// the runner's error ramp.
			if answer := stateAnswerOf(err); answer != "" {
				inspection = &wallInspection{PreTree: preTree, ExpectedTree: preTree, OrderedDigests: []string{}, Violation: answer}
				break
			}
			return nil, nil, false, err
		}
		if inspection.Violation == "" && capture != nil {
			acct, aerr := e.newWallAccountant(preTree, diskState, inspection.Auths, declared)
			if aerr != nil {
				return nil, nil, false, aerr
			}
			acct.noteExpected(inspection.ExpectedTree)
			violation, jerr := e.judgeScope(origin, capture, acct, diskState)
			if jerr != nil {
				// A ran-and-answered probe defeat inside the judgment is
				// a wall answer, not the runner's error.
				if answer := stateAnswerOf(jerr); answer != "" {
					violation = answer
				} else {
					return nil, nil, false, jerr
				}
			}
			inspection.Violation = violation
		}
		if capture != nil {
			inspection.Scope = scopeEvidence(origin, capture)
			inspection.Posture = capture.postureDoc(e.Mission)
		}
		if inspection.Violation != "" {
			if capture == nil {
				break
			}
			// A VIOLATING verdict is acted on only over a stable
			// capture: the capture is many probes, not one atomic
			// read, and lawful motion mid-capture can tear the snapshot
			// into an internally inconsistent posture whose violation is
			// an artifact. The verdict stands only if it was judged over
			// bytes that hold still; a torn capture re-runs the whole
			// inspection, and exhaustion is the bounded
			// would-not-hold-still verdict, never a sticky false park.
			recheck, cerr := e.captureWallPostureStable(inspection.ExpectedTree, declared)
			if cerr != nil {
				if answer := stateAnswerOf(cerr); answer != "" {
					inspection.Violation = answer
					break
				}
				return nil, nil, false, cerr
			}
			if recheck.equalTo(capture) {
				stable = true
				break
			}
			inspection.Violation = ""
			continue
		}
		if capture == nil {
			break
		}
		recheck, cerr := e.captureWallPostureStable(inspection.ExpectedTree, declared)
		if cerr != nil {
			if answer := stateAnswerOf(cerr); answer != "" {
				inspection.Violation = answer
				break
			}
			return nil, nil, false, cerr
		}
		// The re-taken capture is re-judged for steering and namespace
		// integrity: those live outside the whole-capture equality on
		// purpose (the runner's own publications move the namespace), so
		// a carrier planted between captures must fail HERE.
		if violation, jerr := e.judgeCaptureIntegrity(recheck, origin.OpenAnchor, diskState); jerr != nil && stateAnswerOf(jerr) == "" {
			return nil, nil, false, jerr
		} else if violation != "" || jerr != nil {
			if violation == "" {
				violation = stateAnswerOf(jerr)
			}
			inspection.Violation = violation
			break
		}
		stable = recheck.equalTo(capture)
	}
	if inspection.Violation == "" && capture != nil && !stable {
		inspection.Violation = "repository would not hold still during inspection"
	}
	return inspection, capture, stable, nil
}

// attemptWallRecovery is the mechanical rung of the recovery ladder
// (D117, slice A): for the dominant violation ONLY — undeclared workspace
// content diverging from the composed expected tree, judged over a stable
// capture, with every carrier clean and no prior recovery in this
// mission — it puts the tree's bytes back and reports what it restored.
// The caller re-verifies the whole posture with a full fresh inspection
// before any pass is published. Every refusal here — mixed domain, dirty
// carrier, unstable capture, repeat offense, unmaterializable entry —
// returns false and leaves the taint for the existing human path.
func (e *Engine) attemptWallRecovery(inspection *wallInspection, capture *wallCapture, stable bool, declarationViolation string, declared map[string]bool, origin *scopeOrigin, diskState map[string]any, turnID string) (map[string]any, string, bool) {
	refuse := func(why string) (map[string]any, string, bool) {
		e.emit("recovery-inspected", "mechanical restore refused ("+why+"); the taint stands: "+clipSummary(inspection.Violation), map[string]string{
			"missionId": e.Mission, "turnId": turnID, "verdict": "refused",
		})
		return nil, why, false
	}
	if declarationViolation != "" {
		return refuse("the violation is in the declaration or ledger domain")
	}
	if !inspection.UndeclaredOnly {
		return refuse("the violation is not undeclared workspace content")
	}
	if capture == nil || !stable {
		return refuse("the verdict was not judged over a stable capture")
	}
	if inspection.ExpectedTree == "" || inspection.PostTree == "" {
		return refuse("the inspection carries no restore target")
	}
	if missionHasRecoveredAcceptance(diskState) {
		return refuse("a repeat offense in this mission belongs to the human")
	}
	// The inspection returns at the FIRST undeclared path, so a second
	// domain can hide behind the named violation: a declared artifact
	// overwriting reviewed bytes later in the delta would only surface
	// after a restore had already mutated the workspace. Mixed domains
	// refuse before any byte moves.
	for _, path := range inspection.Unaccounted {
		if !declared[path] {
			continue
		}
		for _, auth := range inspection.Auths {
			if _, taken := consumedPathOf(auth.changedPaths, path); taken {
				return refuse("a declared artifact disputes reviewed bytes; mixed domains belong to the human")
			}
		}
	}
	// Clean carriers, judged at RESTORE time on a FRESH capture that
	// must equal the judged one whole: the violating branch's equality
	// confirmation deliberately excludes the mission namespace, so a
	// carrier planted between the verdict and this rung must fail here —
	// and bytes that moved since the verdict refuse the restore outright.
	recheck, cerr := e.captureWallPostureStable(inspection.ExpectedTree, declared)
	if cerr != nil {
		return refuse("the restore-time capture could not be taken")
	}
	if !recheck.equalTo(capture) {
		return refuse("the repository moved between the verdict and the restore")
	}
	acct, aerr := e.newWallAccountant(inspection.PreTree, diskState, inspection.Auths, declared)
	if aerr != nil {
		return refuse("the carrier accounting could not be built")
	}
	acct.noteExpected(inspection.ExpectedTree)
	if violation, jerr := e.judgeScope(origin, recheck, acct, diskState); jerr != nil || violation != "" {
		return refuse("a carrier is not clean")
	}
	if violation, jerr := e.judgeCaptureIntegrity(recheck, origin.OpenAnchor, diskState); jerr != nil || violation != "" {
		return refuse("a carrier is not clean")
	}
	// The restore set is the unexplained delta MINUS the declared host
	// artifacts: a declared path differing from the expected tree is
	// lawful and must survive the restore untouched.
	paths := make([]string, 0, len(inspection.Unaccounted))
	for _, path := range inspection.Unaccounted {
		if !declared[path] {
			paths = append(paths, path)
		}
	}
	if len(paths) == 0 {
		return refuse("the unexplained delta is empty")
	}
	workspace := gittree.Workspace{Dir: e.Root}
	if err := workspace.MaterializePaths(inspection.ExpectedTree, paths); err != nil {
		return refuse(err.Error())
	}
	restored := make([]any, 0, len(paths))
	for _, path := range paths {
		restored = append(restored, path)
	}
	return map[string]any{
		"violation":     inspection.Violation,
		"restoredPaths": restored,
		"restoredAt":    nowISO(),
	}, "", true
}

// missionHasRecoveredAcceptance reports whether any acceptance entry in
// the chain already carries a recovery record: the mechanical rung runs
// once per mission, and a second offense is a repeat that belongs to the
// human.
func missionHasRecoveredAcceptance(state map[string]any) bool {
	for _, item := range turnLogOf(state) {
		entry, _ := item.(map[string]any)
		if entry == nil {
			continue
		}
		if wall, _ := entry["wall"].(map[string]any); wall != nil && wall["recovered"] != nil {
			return true
		}
	}
	return false
}

// scopeEvidence renders the snapshot-scope observables for wall.json.
func scopeEvidence(origin *scopeOrigin, capture *wallCapture) map[string]any {
	scope := map[string]any{
		"headCommitOpen": origin.Head,
		"headCommitNow":  capture.Head,
		"stagedTree":     capture.StagedTree,
	}
	if capture.Nested {
		scope["topTreeOpen"] = origin.TopTree
		scope["topTreeNow"] = capture.TopTree
	}
	deltas := []any{}
	names := map[string]bool{}
	for name := range capture.RefMap {
		names[name] = true
	}
	for name := range origin.RefMap {
		names[name] = true
	}
	for name := range names {
		live, liveExists := capture.RefMap[name]
		recorded, recordedExists := origin.RefMap[name]
		switch {
		case liveExists && recordedExists && live == recorded:
		case !recordedExists:
			deltas = append(deltas, fmt.Sprintf("created %s %s", name, live))
		case !liveExists:
			deltas = append(deltas, fmt.Sprintf("deleted %s (was %s)", name, recorded))
		default:
			deltas = append(deltas, fmt.Sprintf("moved %s %s -> %s", name, recorded, live))
		}
	}
	sort.Slice(deltas, func(i, j int) bool { return deltas[i].(string) < deltas[j].(string) })
	scope["refDeltas"] = deltas
	return scope
}

// parkWallViolation executes the wall refusal ramp for one violation:
// evidence event, turn record failure, ledger booking for the reserved
// cycle, then ONE state write carrying the taint entry and the park.
// Shared by the conclusion gate (bookLedger true — its open-turn marker
// proves the reserved gap to reconciliation) and the reservation-time
// E-continuity check (bookLedger FALSE: no marker
// exists yet, so a ledger block here would be unhealable if the process
// died before the state write; the reserved cycle instead heals as a
// lost turn through the existing reserve/append machinery).
// recoveryNote, when non-empty, is the recovery ladder's context for the
// human: what the mechanical rung tried or why it refused. It rides the
// wall-violation ask only — the taint reason stays the violation itself.
func (e *Engine) parkWallViolation(statePath, ledger, turnID, turnDir string, cycle int64, violation string, diskState map[string]any, bookLedger bool, recoveryNote string) (map[string]any, error) {
	payload := map[string]string{
		"missionId": e.Mission, "turnId": turnID, "error": violation,
	}
	// The event payload carries the committed-HEAD observable beside the
	// violation (records stay the authority, events stay observability).
	if head, _, code := gitCaptured(e.Root, "rev-parse", "--verify", "--quiet", "HEAD^{commit}"); code == 0 {
		payload["headCommit"] = strings.TrimSpace(head)
	}
	e.emit("wall-violation", clipSummary(violation), payload)
	if _, err := patchTurn(filepath.Join(turnDir, "turn.json"), map[string]any{
		"status": "failed", "outcome": "wall-violation", "error": "wall-violation",
		"detail": violation, "endedAt": nowISO(),
	}); err != nil {
		return nil, err
	}
	booked := false
	if bookLedger {
		branch, _ := diskState["branch"].(string)
		candidateSHA, err := e.gitRevParse(branch)
		if err != nil {
			// The TAINT outranks the narrative here too: a
			// host that deleted the mission branch must not stop the
			// detected violation from parking — the booking is skipped
			// and named, like every other unbookable ledger.
			e.emit("wall-violation", "ledger booking skipped: "+clipSummary(err.Error()), map[string]string{
				"missionId": e.Mission, "turnId": turnID, "error": err.Error(),
			})
			candidateSHA = ""
		}
		if candidateSHA != "" {
			// A ledger-ahead crash already booked this
			// cycle: the block is the narrative truth and appending
			// it twice would refuse on contiguity BEFORE the taint and
			// park could land.
			alreadyBooked := false
			if _, _, cycles, perr := mission.ParseLedger(ledger); perr == nil && int64(len(cycles)) >= cycle {
				alreadyBooked = true
			}
			booked = alreadyBooked
			if !alreadyBooked {
				observed := "unmeasurable:" + strings.ReplaceAll("wall violation: "+violation, "\n", " ")
				if _, err := e.appendLedger(diskState, ledger, cycle, "no-progress", candidateSHA, observed, nil); err != nil {
					// The TAINT outranks the narrative:
					// a host edit that made the ledger unparsable must
					// not stop the violation from becoming a resolvable
					// taint — the booking is skipped, named in the event,
					// and the reserved cycle heals once the ledger is
					// ruled back to health.
					e.emit("wall-violation", "ledger booking skipped: "+clipSummary(err.Error()), map[string]string{
						"missionId": e.Mission, "turnId": turnID, "error": err.Error(),
					})
				} else {
					booked = true
				}
			}
		}
	}
	current, err := readDocLabeled(statePath, "mission state", 3)
	if err != nil {
		return nil, err
	}
	outcome, err := ParkProposal(e.Root, e.Mission, current, "wall-violation", nowISO())
	if err != nil {
		return nil, err
	}
	if booked {
		if err := setLedgerCycles(outcome.State, cycle); err != nil {
			return nil, err
		}
	}
	// The ask binds to the taint it parks for: resolution
	// answers exactly its own taint's asks, never a sibling's.
	currentTaint, _ := outcome.State["workspaceTaint"].(map[string]any)
	nextTaintID, _ := jsonInt(currentTaint["next"])
	for _, ask := range outcome.Asks {
		if ask["reasonClass"] == "wall-violation" {
			ask["taintId"] = nextTaintID
			if recoveryNote != "" {
				ask["recoveryNote"] = recoveryNote
			}
		}
	}
	taintID, err := appendTaintEntry(outcome.State, turnID, violation)
	if err != nil {
		return nil, err
	}
	e.emit("taint-set", "workspace taint set by "+turnID, map[string]string{
		"missionId": e.Mission, "turnId": turnID,
		"taintId": fmt.Sprintf("%d", taintID), "error": clipSummary(violation),
	})
	return e.applyPark(statePath, ledger, turnID, outcome)
}

// appendTaintEntry books the violation in the monotonic taint ledger —
// in the same proposal as the park, so the taint and the stop are one
// write. Resolution stays null: only a human's typed RESTORE or
// ADOPT_DISPUTED_TREE ever clears it.
func appendTaintEntry(state map[string]any, turnID, reason string) (int64, error) {
	taint, ok := state["workspaceTaint"].(map[string]any)
	if !ok {
		return 0, failf(3, "mission state carries no workspaceTaint ledger")
	}
	next, ok := jsonInt(taint["next"])
	if !ok || next < 1 {
		return 0, failf(3, "mission workspaceTaint next id is invalid")
	}
	entries, ok := taint["entries"].([]any)
	if !ok {
		return 0, failf(3, "mission workspaceTaint entries are invalid")
	}
	taint["entries"] = append(entries, map[string]any{
		"taintId": next, "turnId": turnID, "reason": reason,
		"setAt": nowISO(), "resolution": nil,
	})
	taint["next"] = next + 1
	return next, nil
}

// hasOpenWallAskForUnresolvedTaint reports whether any open
// wall-violation ask is bound to a taint the state still records as
// UNRESOLVED. Only such an ask suppresses a new one at park time.
func hasOpenWallAskForUnresolvedTaint(asksDir string, state map[string]any) bool {
	unresolved := map[int64]bool{}
	taint, _ := state["workspaceTaint"].(map[string]any)
	entries, _ := taint["entries"].([]any)
	for _, item := range entries {
		entry, _ := item.(map[string]any)
		if entry == nil || entry["resolution"] != nil {
			continue
		}
		if id, ok := jsonInt(entry["taintId"]); ok {
			unresolved[id] = true
		}
	}
	paths, _ := filepath.Glob(filepath.Join(asksDir, "*.json"))
	for _, path := range paths {
		doc, err := readJSONDoc(path)
		if err != nil {
			continue
		}
		if doc["reasonClass"] != "wall-violation" || doc["answeredAt"] != nil || doc["supersededBy"] != nil {
			continue
		}
		if id, ok := jsonInt(doc["taintId"]); ok && unresolved[id] {
			return true
		}
	}
	return false
}

// repairResolvedTaintAsks answers, at runner start, any wall-violation
// ask still open for a taint the state records as RESOLVED — the crash
// tail of a resolution whose answers never landed.
// Pure propagation of the human's recorded ruling, never a decision.
func (e *Engine) repairResolvedTaintAsks(state map[string]any) error {
	taint, _ := state["workspaceTaint"].(map[string]any)
	entries, _ := taint["entries"].([]any)
	for _, item := range entries {
		entry, _ := item.(map[string]any)
		if entry == nil {
			continue
		}
		resolution, _ := entry["resolution"].(map[string]any)
		if resolution == nil {
			continue
		}
		id, ok := jsonInt(entry["taintId"])
		if !ok {
			continue
		}
		if e.openBoundAsks(id) == 0 {
			continue
		}
		if err := e.answerWallViolationAsks(id, resolution); err != nil {
			return err
		}
	}
	return nil
}

// orphanedViolationEvidence finds a turn whose wall.json records a
// violation that never reached the taint ledger — the crash window
// between the evidence write and the park.
// Sticky evidence must survive that window even when no
// open-turn marker exists (the reservation-drift branch), so the next
// reservation re-executes the park before opening anything.
func (e *Engine) orphanedViolationEvidence(state map[string]any) (turnID, violation string) {
	booked := map[string]bool{}
	for _, raw := range taintEntryList(state) {
		entry, _ := raw.(map[string]any)
		if entry == nil {
			continue
		}
		if turn, _ := entry["turnId"].(string); turn != "" {
			booked[turn] = true
		}
	}
	pending := mission.UnverifiedAcceptance(state)
	for _, raw := range turnLogOf(state) {
		entry, _ := raw.(map[string]any)
		if entry == nil {
			continue
		}
		turn, _ := entry["turnId"].(string)
		if turn == "" || turn == pending {
			// A consumed-but-unconcluded acceptance books nothing: its
			// sticky post-verification evidence must still re-park.
			continue
		}
		booked[turn] = true
	}
	paths, _ := filepath.Glob(filepath.Join(missionDirPath(e.Root, e.Mission), "turns", "*", "wall.json"))
	for _, path := range paths {
		doc, err := readJSONDoc(path)
		if err != nil {
			continue
		}
		recorded, _ := doc["violation"].(string)
		if recorded == "" {
			continue
		}
		turn := filepath.Base(filepath.Dir(path))
		if !booked[turn] {
			return turn, recorded
		}
	}
	return "", ""
}

func taintEntryList(state map[string]any) []any {
	taint, _ := state["workspaceTaint"].(map[string]any)
	entries, _ := taint["entries"].([]any)
	return entries
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
