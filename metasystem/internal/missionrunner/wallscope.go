package missionrunner

// The wall's snapshot scope: a mission
// host turn never ships implementer work, and "ship" has three carriers —
// the worktree, the index, and committed history — inside a repository
// that, in a nested checkout, is bigger than the workspace. This file
// owns the whole-posture capture, the accounting decomposition, and the
// judgment rules; wall.go's tree equation remains rule 7, unchanged.

import (
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

// wallCapture is ONE observable posture of the repository, captured
// whole so every rule judges the same instant and the final comparison
// sees exactly what was judged.
type wallCapture struct {
	Head           string
	HeadTree       string // the resolved head's filtered workspace tree
	Unborn         bool
	Branch         string
	Detached       bool
	RefMap         map[string]string // the mission's own namespace excluded
	MissionRefs    map[string]string // the mission's own namespace, judged structurally
	Census         []gittree.WorktreeRecord
	StagedTree     string // workspace staged projection, ledger-filtered
	StagedConflict string // non-empty: an unmerged WORKSPACE entry (a violation, not an error)
	Nested         bool
	TopTree        string
	TopStaged      gittree.StagedPosture
	Post           string   // the comparison-target-seeded worktree projection, ledger-filtered
	Steering       []string // history-steering files present (grafts, shallow) — a violation, not an error
	CapturedAt     string
}

// wallStateAnswer marks a capture defeat the REPOSITORY answered: the
// command ran and reported state the wall cannot account (corrupt refs,
// unparseable records). It is a violation — the host had custody; fail
// toward the wall — while could-not-run failures stay runner errors.
type wallStateAnswer struct{ msg string }

func (a *wallStateAnswer) Error() string { return a.msg }

// captureFailure types one capture-step failure: could-not-run keeps
// the runner's error ramp; everything else is a ran-and-answered
// repository state, returned as a wallStateAnswer.
func captureFailure(op string, err error) error {
	var run *gittree.RunFailure
	if errors.As(err, &run) {
		return failf(3, "%s: %v", op, err)
	}
	return &wallStateAnswer{msg: op + ": " + err.Error()}
}

// stateAnswerOf extracts the violation text of a wallStateAnswer, empty
// for every other error.
func stateAnswerOf(err error) string {
	var answer *wallStateAnswer
	if errors.As(err, &answer) {
		return answer.msg
	}
	return ""
}

// captureWallPostureStable retries a capture whose failure is a
// repository-state ANSWER: normal runner cleanup can remove
// a pseudoref or worktree between enumeration and read, so a transient
// answer must never become sticky evidence — only an answer that
// REPRODUCES on a fresh capture stands. Could-not-run failures keep the
// runner ramp immediately.
func (e *Engine) captureWallPostureStable(expectedTree string, declared map[string]bool) (*wallCapture, error) {
	var last error
	for attempt := 0; attempt < 3; attempt++ {
		capture, err := e.captureWallPosture(expectedTree, declared)
		if err == nil {
			return capture, nil
		}
		answer := stateAnswerOf(err)
		if answer == "" {
			return nil, err
		}
		if last != nil && stateAnswerOf(last) == answer {
			return nil, err
		}
		last = err
	}
	// Exhaustion with DISTINCT answers proved instability, not any one
	// answer: the sticky evidence must say so instead of
	// electing whichever transient came last.
	return nil, &wallStateAnswer{msg: "repository posture would not hold still during capture; last answer: " + stateAnswerOf(last)}
}

// captureWallPosture captures the full observable posture once: resolved
// HEAD, the symbolic head, the ref map, the worktree census with
// postures, both staged projections, the toplevel projection (nested),
// and the seeded worktree projection. Snapshots seed from the RESOLVED
// head, never the symbolic name, so no gap opens between judging and
// projecting.
func (e *Engine) captureWallPosture(expectedTree string, declared map[string]bool) (*wallCapture, error) {
	workspace := gittree.Workspace{Dir: e.Root}
	cap := &wallCapture{CapturedAt: nowISO()}
	steering, err := workspace.HistorySteeringFiles()
	if err != nil {
		return nil, captureFailure("wall capture cannot probe history steering", err)
	}
	cap.Steering = steering
	head, unborn, err := workspace.HeadCommit()
	if err != nil {
		return nil, captureFailure("wall capture cannot resolve HEAD", err)
	}
	cap.Head, cap.Unborn = head, unborn
	if !unborn {
		headTreeRaw, err := workspace.TreeOf(head)
		if err != nil {
			return nil, captureFailure("wall capture cannot read HEAD's tree", err)
		}
		headTree, err := workspace.FilterTree(headTreeRaw, []string{missionLedgerRel(e.Mission)})
		if err != nil {
			return nil, captureFailure("wall capture cannot project HEAD's tree", err)
		}
		cap.HeadTree = headTree
	}
	branch, detached, err := workspace.SymbolicHead()
	if err != nil {
		return nil, captureFailure("wall capture cannot read the symbolic head", err)
	}
	cap.Branch, cap.Detached = branch, detached
	refs, err := workspace.RefMap()
	if err != nil {
		return nil, captureFailure("wall capture cannot enumerate refs", err)
	}
	namespace := mission.MissionRefNamespace(e.Mission)
	cap.RefMap = map[string]string{}
	cap.MissionRefs = map[string]string{}
	for name, oid := range refs {
		if strings.HasPrefix(name, namespace) {
			cap.MissionRefs[name] = oid
			continue
		}
		cap.RefMap[name] = oid
	}
	census, err := workspace.WorktreeCensus()
	if err != nil {
		return nil, captureFailure("wall capture cannot enumerate worktrees", err)
	}
	sort.Slice(census, func(i, j int) bool { return census[i].Path < census[j].Path })
	cap.Census = census
	staged, err := workspace.StagedTree()
	switch {
	case err == nil:
		filtered, ferr := workspace.FilterTree(staged, []string{missionLedgerRel(e.Mission)})
		if ferr != nil {
			return nil, captureFailure("wall capture cannot filter the staged projection", ferr)
		}
		cap.StagedTree = filtered
	case errors.Is(err, gittree.ErrUnmergedWorkspaceIndex):
		// A conflicted WORKSPACE entry has no tree: an answer the rules
		// refuse toward the wall, never a runner error.
		cap.StagedConflict = err.Error()
	default:
		return nil, captureFailure("wall capture cannot project the staged workspace", err)
	}
	prefix, err := workspace.Prefix()
	if err != nil {
		return nil, captureFailure("wall capture cannot resolve the workspace prefix", err)
	}
	if prefix != "" {
		cap.Nested = true
		top, err := workspace.TopLevel()
		if err != nil {
			return nil, captureFailure("wall capture cannot resolve the toplevel", err)
		}
		topWorkspace := gittree.Workspace{Dir: top}
		if !cap.Unborn {
			topTree, err := topWorkspace.Snapshot(cap.Head)
			if err != nil {
				return nil, captureFailure("wall capture cannot snapshot the toplevel", err)
			}
			cap.TopTree = topTree
		}
		topStaged, err := workspace.TopStagedPosture()
		if err != nil {
			return nil, captureFailure("wall capture cannot project the toplevel index", err)
		}
		cap.TopStaged = topStaged
	}
	if expectedTree != "" {
		declaredPaths := make([]string, 0, len(declared))
		for path := range declared {
			declaredPaths = append(declaredPaths, path)
		}
		sort.Strings(declaredPaths)
		seedCommit := cap.Head
		if cap.Unborn {
			seedCommit = ""
		}
		post, err := workspace.SnapshotSeeded(seedCommit, expectedTree, declaredPaths)
		if err != nil {
			return nil, captureFailure("wall capture cannot project the worktree", err)
		}
		filtered, err := workspace.FilterTree(post, []string{missionLedgerRel(e.Mission)})
		if err != nil {
			return nil, captureFailure("wall capture cannot filter the worktree projection", err)
		}
		cap.Post = filtered
	}
	return cap, nil
}

// equalTo compares two captures whole — every observable, the capture
// instants excluded. The censuses compare by full posture.
func (c *wallCapture) equalTo(other *wallCapture) bool {
	if len(c.Steering) != len(other.Steering) {
		return false
	}
	for i := range c.Steering {
		if c.Steering[i] != other.Steering[i] {
			return false
		}
	}
	if c.Head != other.Head || c.HeadTree != other.HeadTree || c.Unborn != other.Unborn ||
		c.Branch != other.Branch || c.Detached != other.Detached ||
		c.StagedTree != other.StagedTree || c.StagedConflict != other.StagedConflict ||
		c.Nested != other.Nested || c.TopTree != other.TopTree ||
		!c.TopStaged.Equal(other.TopStaged) || c.Post != other.Post {
		return false
	}
	// The mission's own namespace is deliberately OUTSIDE the equality:
	// the runner's anchors and state publications move it lawfully
	// between captures, and every capture judges it structurally instead.
	if len(c.RefMap) != len(other.RefMap) {
		return false
	}
	for name, oid := range c.RefMap {
		if other.RefMap[name] != oid {
			return false
		}
	}
	if len(c.Census) != len(other.Census) {
		return false
	}
	for i := range c.Census {
		if !worktreeRecordEqual(c.Census[i], other.Census[i]) {
			return false
		}
	}
	return true
}

func worktreeRecordEqual(a, b gittree.WorktreeRecord) bool {
	if a.Path != b.Path || a.HeadOID != b.HeadOID || a.Branch != b.Branch ||
		a.Detached != b.Detached || a.Bare != b.Bare || a.Prunable != b.Prunable ||
		a.PostureReadable != b.PostureReadable || !a.Staged.Equal(b.Staged) {
		return false
	}
	if len(a.Pseudorefs) != len(b.Pseudorefs) {
		return false
	}
	for i := range a.Pseudorefs {
		x, y := a.Pseudorefs[i], b.Pseudorefs[i]
		if x.Name != y.Name || x.Parseable != y.Parseable || len(x.OIDs) != len(y.OIDs) {
			return false
		}
		for j := range x.OIDs {
			if x.OIDs[j] != y.OIDs[j] {
				return false
			}
		}
	}
	return true
}

// postureDoc renders the capture as the acceptance payload's posture
// block — the recorded authority every between-turns comparison reads.
func (c *wallCapture) postureDoc(missionID string) map[string]any {
	var topTree any
	var topStaged any
	if c.Nested {
		topTree = c.TopTree
		topStaged = mission.StagedPostureDoc(c.TopStaged)
	}
	refMap := map[string]any{}
	for name, oid := range c.RefMap {
		refMap[name] = oid
	}
	return map[string]any{
		"headCommitPost":     c.Head,
		"refMapPost":         refMap,
		"stagedTreePost":     c.StagedTree,
		"topTreePost":        topTree,
		"topStagedPost":      topStaged,
		"worktreeCensusPost": mission.WorktreeCensusDoc(c.Census),
		"capturedAt":         c.CapturedAt,
	}
}

// ---- the accounted set ----

// scopeAuth is one consumed authorization's decomposition facts.
type scopeAuth struct {
	digest          string
	changedPaths    []string
	reviewedTree    string
	reviewedEntries map[string]gittree.Entry
}

// wallAccountant answers rule membership: a candidate tree is ACCOUNTED
// against the open pre-tree when every consumed authorization's changed
// paths carry either that patch's entire pre-side or its entire
// post-side entries, declared host-artifact paths are content-free, and
// every other path equals the pre-tree. Membership is checked by
// DECOMPOSITION, never enumeration.
type wallAccountant struct {
	workspace gittree.Workspace
	state     map[string]any
	preTree   string
	named     map[string]bool
	reviewed  map[string]bool
	auths     []scopeAuth
	pathAuth  map[string]int
	declared  map[string]bool
	ledgerRel string
	// anchoredLedger is the anchored ledger blob OID ("" while the
	// mission has never anchored): the ONLY bytes any judged carrier may
	// hold at the ledger path.
	anchoredLedger string
	prefix         string
	originTop      string // the origin toplevel tree (nested checkouts): sibling scope for retained OIDs
	verdicts       map[string]string
}

// newWallAccountant builds the accountant for one inspection: the
// pre-tree origin, the named E-sequence points (accounted directly), the
// consumed authorizations with their reviewed entries, and the declared
// content-free paths.
func (e *Engine) newWallAccountant(preTree string, state map[string]any, auths []scopeAuth, declared map[string]bool) (*wallAccountant, error) {
	workspace := gittree.Workspace{Dir: e.Root}
	prefix, err := workspace.Prefix()
	if err != nil {
		return nil, failf(3, "wall accounting cannot resolve the workspace prefix: %v", err)
	}
	anchoredLedger := ""
	ledgerPath := filepath.Join(missionDirPath(e.Root, e.Mission), "ledger.md")
	if oid, aerr := mission.AnchoredLedgerBlobOID(e.Root, state, ledgerPath); aerr == nil {
		anchoredLedger = oid
	} else if !errors.Is(aerr, mission.ErrNoAnchor) {
		// Any other anchor failure is the runner's own — never a silent
		// pass that disables the raw-ledger carrier lane.
		return nil, failf(3, "wall accounting cannot resolve the anchored ledger blob: %v", aerr)
	}
	acct := &wallAccountant{
		workspace:      workspace,
		state:          state,
		preTree:        preTree,
		named:          map[string]bool{preTree: true},
		reviewed:       map[string]bool{},
		auths:          auths,
		pathAuth:       map[string]int{},
		declared:       declared,
		ledgerRel:      missionLedgerRel(e.Mission),
		anchoredLedger: anchoredLedger,
		prefix:         prefix,
		verdicts:       map[string]string{},
	}
	for _, point := range mission.ExpectedTreePoints(state) {
		acct.named[point.Tree] = true
	}
	if baseline, _ := state["initialBaseline"].(string); baseline != "" {
		acct.named[baseline] = true
	}
	for index, auth := range auths {
		acct.reviewed[auth.reviewedTree] = true
		for _, path := range auth.changedPaths {
			acct.pathAuth[path] = index
		}
	}
	return acct, nil
}

// noteExpected admits the fully composed expected tree by name.
func (a *wallAccountant) noteExpected(tree string) {
	if tree != "" {
		a.named[tree] = true
	}
}

// accountedTree reports "" for an accounted tree, else the offense.
// Strict accounting is NAMED-OR-DECOMPOSED only: a raw reviewed tree is
// an old base plus one patch and may revert later accepted paths, so
// reviewed membership is a separate lane reserved for merge side tips
// and permitted pseudoref cargo.
func (a *wallAccountant) accountedTree(tree string) (string, error) {
	if a.named[tree] {
		return "", nil
	}
	if verdict, seen := a.verdicts[tree]; seen {
		return verdict, nil
	}
	verdict, err := a.decompose(tree)
	if err != nil {
		return "", err
	}
	a.verdicts[tree] = verdict
	return verdict, nil
}

func (a *wallAccountant) decompose(tree string) (string, error) {
	changed, err := a.workspace.ChangedPaths(a.preTree, tree)
	if err != nil {
		return "", failf(3, "wall accounting cannot diff a candidate tree: %v", err)
	}
	involved := map[int]bool{}
	for _, path := range changed {
		if a.declared[path] {
			continue
		}
		if index, mine := a.pathAuth[path]; mine {
			involved[index] = true
			continue
		}
		return fmt.Sprintf("%s is neither authorized nor declared", path), nil
	}
	// Whole patches only: an authorization with ANY path moved must carry
	// its entire post-side — the reviewed object id and git mode on every
	// changed path (patches are pairwise disjoint, so any subset composes
	// deterministically without a recorded order).
	for index := range involved {
		auth := a.auths[index]
		got, err := a.workspace.Entries(tree, auth.changedPaths)
		if err != nil {
			return "", failf(3, "wall accounting cannot read a candidate tree: %v", err)
		}
		for _, path := range auth.changedPaths {
			if got[path] != auth.reviewedEntries[path] {
				return fmt.Sprintf("authorization %.12s is carried partially at %s (whole patches only)", auth.digest, path), nil
			}
		}
	}
	return "", nil
}

// rawLedgerCarrier refuses a tree whose UNFILTERED ledger entry carries
// anything but the anchored ledger blob as a regular
// file: every accounted/reviewed lane judges the workspace-filtered
// tree, and the filter must never launder a foreign — or mode-changed —
// ledger into reachable shipping history or pseudoref cargo.
func (a *wallAccountant) rawLedgerCarrier(tree, scope string) (string, error) {
	if a.anchoredLedger == "" {
		return "", nil // a mission that has never anchored (fresh beds)
	}
	entries, err := a.workspace.Entries(tree, []string{a.ledgerRel})
	if err != nil {
		return "", failf(3, "wall accounting cannot read %s's ledger carrier: %v", scope, err)
	}
	if entry, present := entries[a.ledgerRel]; present && (entry.OID != a.anchoredLedger || entry.Mode != "100644") {
		return fmt.Sprintf("%s carries an unauthorized mission-ledger entry (%s %s)", scope, entry.Mode, entry.OID), nil
	}
	return "", nil
}

// commitWorkspaceTree names a commit's filtered workspace tree, first
// judging the RAW ledger entry the filter is about to remove; a non-""
// violation names a ledger-smuggling carrier.
func (a *wallAccountant) commitWorkspaceTree(oid string) (tree, violation string, err error) {
	raw, err := a.workspace.TreeOf(oid)
	if err != nil {
		// Ran-and-answered unreadable history — a merge naming a missing
		// or corrupt object — is a VIOLATION on the host's watch, never
		// the runner's failure; only could-not-run rides the
		// ramp.
		var runFailure *gittree.RunFailure
		if errors.As(err, &runFailure) {
			return "", "", failf(3, "wall accounting cannot read commit %s: %v", oid, err)
		}
		return "", fmt.Sprintf("commit %s names unreadable history: %v", oid, err), nil
	}
	if violation, err := a.rawLedgerCarrier(raw, "commit "+oid); violation != "" || err != nil {
		return "", violation, err
	}
	filtered, err := a.workspace.FilterTree(raw, []string{a.ledgerRel})
	if err != nil {
		var runFailure *gittree.RunFailure
		if errors.As(err, &runFailure) {
			return "", "", failf(3, "wall accounting cannot filter commit %s: %v", oid, err)
		}
		return "", fmt.Sprintf("commit %s names unreadable history: %v", oid, err), nil
	}
	return filtered, "", nil
}

// accountedCommit reports "" for a commit whose filtered workspace tree
// is accounted, else the offense.
func (a *wallAccountant) accountedCommit(oid string) (string, error) {
	tree, violation, err := a.commitWorkspaceTree(oid)
	if violation != "" || err != nil {
		return violation, err
	}
	return a.accountedTree(tree)
}

// accountedOrReviewedCommit is the pseudoref-cargo lane's commit form.
func (a *wallAccountant) accountedOrReviewedCommit(oid string) (string, error) {
	tree, violation, err := a.commitWorkspaceTree(oid)
	if violation != "" || err != nil {
		return violation, err
	}
	return a.accountedOrReviewedTree(tree)
}

// accountedOrReviewedTree is the side-tip and pseudoref lane: the
// conformance-bound reviewed tree is admissible there — the lane
// reserved for merge side tips and permitted pseudoref cargo.
func (a *wallAccountant) accountedOrReviewedTree(tree string) (string, error) {
	if a.reviewed[tree] {
		return "", nil
	}
	return a.accountedTree(tree)
}

// accountedOID judges one retained object id (a pseudoref's cargo): a
// commit or tag judges by its workspace tree; a tree judges directly
// (AUTO_MERGE); a blob is never accounted.
func (a *wallAccountant) accountedOID(e *Engine, oid string) (string, error) {
	stdout, stderr, code := gitCaptured(e.Root, "cat-file", "-t", oid)
	if code == -1 {
		// Could-not-run is the runner's own failure, never a repository
		// verdict.
		return "", failf(3, "wall accounting could not probe retained object %s: %s", oid, firstDetail(stderr, stdout))
	}
	if code != 0 {
		return fmt.Sprintf("retained object %s is unreadable: %s", oid, firstDetail(stderr, stdout)), nil
	}
	switch strings.TrimSpace(stdout) {
	case "commit":
		if detail, err := a.accountedOrReviewedCommit(oid); err != nil || detail != "" {
			return detail, err
		}
		return a.retainedToplevelScope(e, oid+"^{tree}")
	case "tag":
		peeled, stderr, code := gitCaptured(e.Root, "rev-parse", oid+"^{commit}")
		if code == -1 {
			return "", failf(3, "wall accounting could not peel retained tag %s: %s", oid, strings.TrimSpace(stderr))
		}
		if code != 0 {
			return fmt.Sprintf("retained tag %s does not peel to a commit: %s", oid, strings.TrimSpace(stderr)), nil
		}
		if detail, err := a.accountedOrReviewedCommit(strings.TrimSpace(peeled)); err != nil || detail != "" {
			return detail, err
		}
		return a.retainedToplevelScope(e, strings.TrimSpace(peeled)+"^{tree}")
	case "tree":
		tree := oid
		if a.prefix != "" {
			sub, _, subCode := gitCaptured(e.Root, "rev-parse", oid+":"+strings.TrimSuffix(a.prefix, "/"))
			if subCode == -1 {
				// A failed probe is NOT "no subtree": continuing on the
				// toplevel tree would judge the wrong scope.
				return "", failf(3, "wall accounting could not resolve retained tree %s's workspace subtree", oid)
			}
			if subCode == 0 {
				tree = strings.TrimSpace(sub)
			}
		}
		// The raw ledger entry is judged before the filter here exactly
		// as on the commit lanes.
		if detail, err := a.rawLedgerCarrier(tree, "retained tree "+oid); err != nil || detail != "" {
			return detail, err
		}
		filtered, err := a.workspace.FilterTree(tree, []string{a.ledgerRel})
		if err != nil {
			var runFailure *gittree.RunFailure
			if errors.As(err, &runFailure) {
				return "", failf(3, "wall accounting could not filter retained tree %s: %v", oid, err)
			}
			return fmt.Sprintf("retained tree %s is unreadable", oid), nil
		}
		if detail, err := a.accountedOrReviewedTree(filtered); err != nil || detail != "" {
			return detail, err
		}
		return a.retainedToplevelScope(e, oid)
	default:
		return fmt.Sprintf("retained object %s is neither commit, tag, nor tree", oid), nil
	}
}

// retainedToplevelScope refuses a retained object whose WHOLE toplevel
// tree differs from the origin toplevel tree at any sibling path — a
// nested-checkout carrier can otherwise pair an accounted workspace
// subtree with arbitrary sibling blobs that the live-checkout fence
// never inspects. Vacuous at a toplevel install (no prefix).
func (a *wallAccountant) retainedToplevelScope(e *Engine, treeish string) (string, error) {
	if a.prefix == "" || a.originTop == "" {
		return "", nil
	}
	top, err := a.workspace.TopLevel()
	if err != nil {
		return "", failf(3, "wall inspection cannot resolve the toplevel: %v", err)
	}
	topWorkspace := gittree.Workspace{Dir: top}
	resolved, _, code := gitCaptured(e.Root, "rev-parse", treeish)
	if code == -1 {
		return "", failf(3, "wall accounting could not resolve retained object tree %s", treeish)
	}
	if code != 0 {
		return fmt.Sprintf("retained object tree %s is unreadable at toplevel", treeish), nil
	}
	// The origin is the COMMITTED HEAD toplevel tree (a.originTop), not a
	// worktree snapshot: comparing two committed trees means preexisting
	// worktree dirt never reads as payload. A sibling path the retained
	// object ADDS or MODIFIES against that committed baseline is the
	// laundering the check exists for; a path the object merely lacks
	// (the missing dirt) is not the object's payload.
	probe, err := a.retainedSiblingProbe(top, a.originTop, strings.TrimSpace(resolved))
	if err != nil {
		return "", failf(3, "wall inspection cannot diff a retained object at toplevel: %v", err)
	}
	entries, err := topWorkspace.Entries(strings.TrimSpace(resolved), probe)
	if err != nil {
		return "", failf(3, "wall inspection cannot read a retained object at toplevel: %v", err)
	}
	baseEntries, err := topWorkspace.Entries(a.originTop, probe)
	if err != nil {
		return "", failf(3, "wall inspection cannot read the origin toplevel: %v", err)
	}
	for _, path := range probe {
		if strings.HasPrefix(path, a.prefix) {
			continue
		}
		// Every probed sibling path is compared BOTH ways: an added,
		// modified, OR deleted sibling entry (present in one tree, absent
		// or different in the other) is payload the retained object must
		// not carry against the committed baseline.
		if baseEntries[path] != entries[path] {
			return fmt.Sprintf("retained object carries sibling payload at %s", path), nil
		}
	}
	return "", nil
}

// retainedSiblingProbe is the set of sibling paths that DIFFER between the
// committed origin and the retained object — the only paths whose
// direction (added/modified vs removed) the scope check need resolve.
func (a *wallAccountant) retainedSiblingProbe(top, origin, object string) ([]string, error) {
	topWorkspace := gittree.Workspace{Dir: top}
	changed, err := topWorkspace.ChangedPaths(origin, object)
	if err != nil {
		// A diff that cannot run — an unreadable or missing sibling
		// subtree included — is never a silent pass: the caller fails the
		// inspection loudly rather than admitting the object.
		return nil, err
	}
	probe := []string{}
	for _, path := range changed {
		if !strings.HasPrefix(path, a.prefix) {
			probe = append(probe, path)
		}
	}
	return probe, nil
}

// ---- rules ----

// scopeOrigin is the accounting origin one judgment runs from: the open
// record mid-turn, the previous acceptance's recorded posture between
// turns, the birth record for turn one.
type scopeOrigin struct {
	Head       string
	RefMap     map[string]string
	TopTree    string
	TopStaged  *gittree.StagedPosture
	StagedPost string
	Census     []map[string]any
	OpenAnchor string // expected turn-open-head value; "" skips the check
}

// couldNotRunSentinel marks a judgment probe that could not run; the
// judge converts it back into a runner error instead of a verdict.
const couldNotRunSentinel = "\x00could-not-run: "

// firstParentSegment lists the commits on the first-parent chain from
// head back to (excluding) origin, oldest first, proving origin IS on
// that chain — plain ancestry is not enough: an open commit reachable
// only through a merge's second parent leaves the walk no terminal.
// The walk is complete: a resource bound is the bounded-exec timeout,
// which surfaces as a runner error, never a semantic verdict.
func (e *Engine) firstParentSegment(origin, head string) ([]string, string) {
	if origin == head {
		return nil, ""
	}
	stdout, stderr, code := gitCaptured(e.Root, "rev-list", "--first-parent",
		head, "--not", origin)
	if code == -1 {
		// The command could not run at all: the runner's failure, never a
		// history verdict — surfaced through the error ramp by the caller.
		return nil, couldNotRunSentinel + firstDetail(stderr, stdout)
	}
	if code != 0 {
		return nil, fmt.Sprintf("committed HEAD retreated or rewrote history (open %s, now %s): %s",
			origin, head, firstDetail(stderr, stdout))
	}
	lines := strings.Fields(stdout)
	if len(lines) == 0 {
		return nil, fmt.Sprintf("committed HEAD retreated or rewrote history (open %s, now %s)", origin, head)
	}
	oldest := lines[len(lines)-1]
	parent, perr, pcode := gitCaptured(e.Root, "rev-parse", "--verify", "--quiet", oldest+"^1")
	if pcode == -1 {
		return nil, couldNotRunSentinel + firstDetail(perr, parent)
	}
	// A nonzero answer (the oldest listed commit is rootless) and a parent
	// that is not the open commit both mean the open commit is NOT on the
	// first-parent chain — the history verdict, not a runner failure.
	if pcode != 0 || strings.TrimSpace(parent) != origin {
		return nil, fmt.Sprintf("committed HEAD retreated or rewrote history (open %s, now %s)", origin, head)
	}
	// Oldest first for judgment order.
	chain := make([]string, 0, len(lines))
	for i := len(lines) - 1; i >= 0; i-- {
		chain = append(chain, lines[i])
	}
	return chain, ""
}

// judgeHeadChain runs rules 1-4: first-parent reachability, per-commit
// tree accounting, side-tip accounting with the --no-ff integration
// contract, and repository scope in nested checkouts.
func (e *Engine) judgeHeadChain(origin *scopeOrigin, cap *wallCapture, acct *wallAccountant) (string, error) {
	if cap.Unborn {
		return fmt.Sprintf("committed HEAD retreated or rewrote history (open %s, now unborn)", origin.Head), nil
	}
	chain, violation := e.firstParentSegment(origin.Head, cap.Head)
	if strings.HasPrefix(violation, couldNotRunSentinel) {
		return "", failf(3, "wall inspection cannot walk the first-parent chain: %s", strings.TrimPrefix(violation, couldNotRunSentinel))
	}
	if violation != "" {
		return violation, nil
	}
	var top gittree.Workspace
	if cap.Nested {
		topDir, err := acct.workspace.TopLevel()
		if err != nil {
			return "", failf(3, "wall inspection cannot resolve the toplevel: %v", err)
		}
		top = gittree.Workspace{Dir: topDir}
	}
	tipAccounted := ""
	if len(chain) > 0 {
		tip := chain[len(chain)-1]
		if detail, err := acct.accountedCommit(tip); err != nil {
			return "", err
		} else {
			tipAccounted = detail
		}
	}
	for _, commit := range chain {
		if violation, cerr := e.judgeCommitLedgerCarrier(commit, acct.state); cerr != nil || violation != "" {
			return violation, cerr
		}
		detail, err := acct.accountedCommit(commit)
		if err != nil {
			return "", err
		}
		if detail != "" {
			violation := fmt.Sprintf("commit %s advances HEAD with an unaccounted tree: %s", commit, detail)
			// A fast-forward of a reviewed multi-commit branch puts the
			// implementer's intermediate trees on the first-parent chain,
			// where whole-patch membership rightly fails; when the tip
			// itself is accounted, the remedy is named.
			if tipAccounted == "" && commit != chain[len(chain)-1] {
				violation += "; integrate reviewed branches with --no-ff"
			}
			return violation, nil
		}
		parents, perr := e.commitParents(commit)
		if perr != nil {
			return "", perr
		}
		if cap.Nested && len(parents) > 0 {
			// Rule 4, first-parent scope: each commit changes only
			// workspace-prefixed paths against its first parent, judged at
			// toplevel.
			changed, err := top.ChangedPaths(parents[0]+"^{tree}", commit+"^{tree}")
			if err != nil {
				return "", failf(3, "wall inspection cannot diff commit %s at toplevel: %v", commit, err)
			}
			for _, path := range changed {
				if !strings.HasPrefix(path, cap.topPrefix(acct)) {
					return fmt.Sprintf("sibling paths changed in a nested checkout on the host's watch: commit %s touches %s", commit, path), nil
				}
			}
		}
		// Rule 3: every non-first parent's filtered workspace tree equals
		// a consumed authorization's REVIEWED tree or is itself accounted.
		for _, side := range parents[1:] {
			sideTree, sideViolation, err := acct.commitWorkspaceTree(side)
			if err != nil {
				return "", err
			}
			detail := sideViolation
			if detail == "" {
				if detail, err = acct.accountedOrReviewedTree(sideTree); err != nil {
					return "", err
				}
			}
			if detail != "" {
				return fmt.Sprintf("commit %s advances HEAD with an unaccounted tree: merge side tip %s: %s", commit, side, detail), nil
			}
			// The repository-scope check runs for EVERY side parent — a
			// reviewed workspace subtree says nothing about sibling paths.
			if cap.Nested {
				// Rule 4, accumulated side scope: the side tip's whole
				// toplevel tree against the merge base with the
				// first-parent line must differ only at workspace-prefixed
				// paths — a sibling payload buried in an interior side
				// commit has empty immediate deltas everywhere else.
				baseOut, baseErr, baseCode := gitCaptured(e.Root, "merge-base", parents[0], side)
				if baseCode == -1 {
					return "", failf(3, "wall inspection cannot compute a merge base: %s", firstDetail(baseErr, baseOut))
				}
				if baseCode != 0 {
					// Ran-and-answered "no common ancestor" is a
					// VERDICT: an unrelated side chain has no scopable
					// base and is exactly the foreign-history carrier the
					// sibling fence exists to refuse.
					return fmt.Sprintf("sibling scope cannot bound commit %s: side chain %s shares no history with the mission line", commit, side), nil
				}
				base := strings.TrimSpace(baseOut)
				changed, err := top.ChangedPaths(base+"^{tree}", side+"^{tree}")
				if err != nil {
					return "", failf(3, "wall inspection cannot diff a side chain at toplevel: %v", err)
				}
				for _, path := range changed {
					if !strings.HasPrefix(path, cap.topPrefix(acct)) {
						return fmt.Sprintf("sibling paths changed in a nested checkout on the host's watch: side chain of %s touches %s", commit, path), nil
					}
				}
			}
		}
	}
	return "", nil
}

// topPrefix is the workspace prefix in toplevel path space.
func (c *wallCapture) topPrefix(acct *wallAccountant) string {
	return acct.prefix
}

// commitParents lists a commit's parents in order.
func (e *Engine) commitParents(oid string) ([]string, error) {
	stdout, stderr, code := gitCaptured(e.Root, "rev-list", "--parents", "-n", "1", oid)
	if code == -1 {
		return nil, failf(3, "wall inspection cannot read the parents of %s: %s", oid, firstDetail(stderr, stdout))
	}
	if code != 0 {
		// Ran-and-answered: the commit's history is unreadable on the
		// host's watch — a wall answer, not a runner error.
		return nil, &wallStateAnswer{msg: fmt.Sprintf("commit %s parents are unreadable: %s", oid, firstDetail(stderr, stdout))}
	}
	fields := strings.Fields(stdout)
	if len(fields) == 0 || fields[0] != oid {
		return nil, failf(3, "wall inspection cannot read the parents of %s", oid)
	}
	return fields[1:], nil
}

// judgeMissionNamespace judges the mission's own publication refs
// STRUCTURALLY at every capture: the state-anchors ref is authenticated
// through the anchor machinery (stronger than OID equality, no
// circularity); the turn-open-head ref must equal the recorded open
// commit; every other ref must be a self-naming tree anchor.
func (e *Engine) judgeMissionNamespace(missionRefs map[string]string, openAnchor string, state map[string]any) (string, error) {
	namespace := mission.MissionRefNamespace(e.Mission)
	// Deletion of a runner ref is as loud as motion. The discriminator is
	// TAMPER-RESISTANT: the state's own hash chain (validated before this
	// judgment ever runs) proves the mission wrote at least once, and in
	// production every state write anchors — so a state carrying an
	// integrity hash MUST keep its state-anchors ref. A deleted anchor and
	// a never-written state cannot be confused: a never-written state has
	// no chain to carry an integrity hash.
	integrityBlock, _ := state["integrity"].(map[string]any)
	chained := false
	if hash, _ := integrityBlock["hash"].(string); hash != "" {
		chained = true
		if _, present := missionRefs[namespace+"state-anchors"]; !present {
			return fmt.Sprintf("runner ref %sstate-anchors was deleted", namespace), nil
		}
	}
	if openAnchor != "" {
		// During a turn the ref is CAS-set to the open head: its deletion
		// or motion mid-turn breaks the accounting origin. Between turns
		// the ref is runner bookkeeping — dropped at conclusion as
		// hygiene, healed by the CAS overwrite at the next open — and a
		// crash tail (state concluded, delete lost) must not read as host
		// motion, so the quiet period leaves it unjudged: it ships
		// nothing, and every shipping carrier has its own fence.
		live, present := missionRefs[namespace+"turn-open-head"]
		if !present {
			return fmt.Sprintf("runner ref %sturn-open-head was deleted (recorded %s)", namespace, openAnchor), nil
		}
		if live != openAnchor {
			return fmt.Sprintf("runner ref %sturn-open-head moved (recorded %s, now %s)", namespace, openAnchor, live), nil
		}
	}
	// Every tree the state NAMES as an origin must keep its self-named
	// anchor ref, or a deletion breaks the reachability the accounting
	// walks.
	if chained {
		for _, tree := range e.requiredAnchorTrees(state) {
			if _, present := missionRefs[namespace+tree]; present {
				continue
			}
			// cat-file -e on the bare object id: exit 0 — the object
			// exists, so a live anchor's ref is gone (deletion); exit 1 —
			// a well-formed id that names nothing (a fake or collected
			// tree has no reachability to protect); any other nonzero — a
			// could-not-run failure, the runner's error, never a silent
			// skip that could admit a live anchor's deletion.
			_, stderr, code := gitCaptured(e.Root, "cat-file", "-e", tree)
			switch code {
			case 0:
				return fmt.Sprintf("runner tree anchor %s%s was deleted", namespace, tree), nil
			case 1:
				continue
			default:
				return "", failf(3, "wall inspection cannot probe tree anchor %s: %s", tree, strings.TrimSpace(stderr))
			}
		}
	}
	for name, oid := range missionRefs {
		short := strings.TrimPrefix(name, namespace)
		switch {
		case short == "state-anchors":
			// A PRESENT publication ref authenticates through the anchor
			// machinery — stronger than OID equality and free of the
			// self-reference a recorded map would carry. The check runs
			// against real hash-chained states only (absence and unit
			// judgment beds stay reconciliation's domain).
			if chained {
				// The PREFIX-tolerant authentication: mid-conclusion the
				// runner's own lawful ledger append extends the anchored
				// bytes before the acceptance write re-anchors, so exact
				// byte equality would refuse the runner's own lawful
				// interval; the anchored truth must still be a prefix with
				// every cross-check intact.
				if err := mission.AuthenticateLiveLedger(e.Root, state, filepath.Join(missionDirPath(e.Root, e.Mission), "ledger.md")); err != nil {
					// Could-not-run stays the runner's own;
					// only a ran-and-answered disagreement is a violation.
					var runFailure *gittree.RunFailure
					if errors.As(err, &runFailure) {
						return "", failf(3, "mission namespace authentication could not run: %v", err)
					}
					return fmt.Sprintf("runner ref %s does not authenticate against the anchored truth: %v", name, err), nil
				}
			}
		case short == "turn-open-head":
			// Presence and equality are judged upfront (mid-turn) or left
			// to the next open's CAS overwrite (the quiet period).
		default:
			if short != oid {
				return fmt.Sprintf("ref %s is not a runner anchor and was created in the mission's namespace", name), nil
			}
			// A self-named ref must BE a tree anchor: a commit parked
			// under its own id would retain history the fence never sees.
			stdout, stderr, code := gitCaptured(e.Root, "cat-file", "-t", oid)
			if code == -1 {
				return "", failf(3, "mission namespace probe could not run for %s: %s", name, firstDetail(stderr, stdout))
			}
			if code != 0 {
				return fmt.Sprintf("ref %s names an unreadable object: %s", name, firstDetail(stderr, stdout)), nil
			}
			if objectType := strings.TrimSpace(stdout); objectType != "tree" {
				return fmt.Sprintf("ref %s parks a %s in the anchor namespace; anchors are trees", name, objectType), nil
			}
			// A self-named tree ref retains a TREE object, which never
			// ships to HEAD, the index, or the worktree — the wall's
			// judgment space. Unreachable-tree retention is the isolation
			// tier's business (the same boundary as retained
			// pseudoref blobs); the type check above keeps a
			// history-carrying commit out of the anchor namespace.
		}
	}
	return "", nil
}

// requiredAnchorTrees lists the tree ids the state names as origins —
// the expected-tree points plus the recorded posture/toplevel trees.
// Each must keep its anchor ref, or a deletion breaks the reachability
// the accounting walks.
func (e *Engine) requiredAnchorTrees(state map[string]any) []string {
	seen := map[string]bool{}
	trees := []string{}
	add := func(tree string) {
		if tree != "" && !seen[tree] {
			seen[tree] = true
			trees = append(trees, tree)
		}
	}
	for _, point := range mission.ExpectedTreePoints(state) {
		add(point.Tree)
	}
	if origin := lastAcceptancePosture(state); origin != nil {
		add(origin.TopTree)
		if origin.TopStaged != nil {
			add(origin.TopStaged.Tree)
		}
		add(origin.StagedPost)
	}
	// The OPEN turn's own anchored origins (nested checkouts) — the wall
	// anchors topTree and topStaged.tree at open, and ExpectedTreePoints
	// deliberately excludes the current open turn, so they are named
	// here or their deletion goes unseen.
	if openTurn, _ := state["openTurn"].(map[string]any); openTurn != nil {
		if tree, _ := openTurn["topTree"].(string); tree != "" {
			add(tree)
		}
		if staged, _ := openTurn["topStaged"].(map[string]any); staged != nil {
			if tree, _ := staged["tree"].(string); tree != "" {
				add(tree)
			}
		}
	}
	return trees
}

// agentBranchJob names the dispatch job of an agent branch, empty when
// the ref is not an agent branch or no job record vouches for it.
func (e *Engine) agentBranchJob(ref string) string {
	job := strings.TrimPrefix(ref, "refs/heads/agent/")
	if job == ref || job == "" || strings.Contains(job, "/") {
		return ""
	}
	record, err := readJSONDoc(filepath.Join(jobsDirPath(e.Root), job+".json"))
	if err != nil {
		return ""
	}
	// THIS mission's dispatch records only: a foreign or forged job file
	// admits nothing — its branch falls to the everything-else fence and
	// surfaces visibly.
	if branch, _ := record["branch"].(string); branch != "agent/"+job || !numericEqual(record["mission"], e.Mission) {
		return ""
	}
	return job
}

// consumedJobs is the set of dispatch jobs whose authorization chains
// the mission has CONSUMED — their branches and worktrees hold still
// from consumption on.
func (e *Engine) consumedJobs(state map[string]any) (map[string]bool, error) {
	index, err := mission.ConsumedAuthorizations(state)
	if err != nil {
		return nil, err
	}
	jobs := map[string]bool{}
	authDir := filepath.Join(missionDirPath(e.Root, e.Mission), "authorizations")
	for digest := range index {
		record, err := readJSONDoc(filepath.Join(authDir, digest+".json"))
		if err != nil {
			// FAIL CLOSED: a consumed record that cannot be read cannot
			// prove which carriers hold still — deleting it must never
			// restore free motion.
			return nil, failf(3, "wall inspection cannot read consumed authorization %.12s: %v", digest, err)
		}
		if recomputed, derr := validate.AuthorizationRecordDigest(record); derr != nil || recomputed != digest {
			return nil, failf(3, "consumed authorization %.12s record bytes do not match their digest", digest)
		}
		for _, key := range []string{"jobId", "rootJob"} {
			if job, _ := record[key].(string); job != "" {
				jobs[job] = true
			}
		}
	}
	return jobs, nil
}

// judgeRefFence runs rule 5's exact, record-bound transition fence
// against the recorded origin ref map.
func (e *Engine) judgeRefFence(origin *scopeOrigin, cap *wallCapture, state map[string]any, consumed map[string]bool, acct *wallAccountant) (string, error) {
	// THE ACTIVE BRANCH is the one lawful non-runner transition: the
	// checked-out candidate branch must BE the branch the mission is
	// pinned to and must equal the same capture's resolved HEAD — a
	// same-tip detach or a switch to another branch violates.
	branch, _ := state["branch"].(string)
	activeRef := "refs/heads/" + branch
	if cap.Detached || cap.Branch != activeRef {
		observed := cap.Branch
		if cap.Detached {
			observed = "detached"
		}
		return fmt.Sprintf("the checkout left the mission branch %s (now %s)", branch, observed), nil
	}
	if tip, ok := cap.RefMap[activeRef]; !ok || tip != cap.Head {
		return fmt.Sprintf("the mission branch %s does not point at the resolved HEAD", branch), nil
	}
	if violation, err := e.judgeMissionNamespace(cap.MissionRefs, origin.OpenAnchor, state); err != nil || violation != "" {
		return violation, err
	}
	names := map[string]bool{}
	for name := range cap.RefMap {
		names[name] = true
	}
	for name := range origin.RefMap {
		names[name] = true
	}
	for name := range names {
		live, liveExists := cap.RefMap[name]
		recorded, recordedExists := origin.RefMap[name]
		if liveExists && recordedExists && live == recorded {
			continue
		}
		if name == activeRef {
			// Content is rules 1-4's business; position was proven above.
			continue
		}
		if job := e.agentBranchJob(name); job != "" {
			// A live implementer's branch is its workspace: it moves
			// freely while its chain's authorization is UNCONSUMED (rule
			// 5) — its bytes ship only through conformance, so an
			// unconsumed branch never reaches the mission product — and
			// holds still, motion AND deletion, from consumption on.
			if !consumed[job] {
				continue
			}
			// STATIONARY from consumption on (rule 5): the branch must
			// equal its recorded value exactly — deletion, motion, AND
			// reappearance (recorded absent, now present) all violate, so
			// no post-consumption byte re-enters through this lane.
			if !liveExists {
				return fmt.Sprintf("consumed implementer branch %s was deleted during the turn", name), nil
			}
			if !recordedExists {
				return fmt.Sprintf("consumed implementer branch %s reappeared after consumption (now %s)", name, live), nil
			}
			return fmt.Sprintf("consumed implementer branch %s moved after consumption (recorded %s, now %s)", name, recorded, live), nil
		}
		switch {
		case !recordedExists:
			return fmt.Sprintf("ref %s was created during the turn (now %s)", name, live), nil
		case !liveExists:
			return fmt.Sprintf("ref %s was deleted during the turn (recorded %s)", name, recorded), nil
		default:
			return fmt.Sprintf("ref %s moved during the turn (recorded %s, now %s)", name, recorded, live), nil
		}
	}
	return "", nil
}

// measureWorktreeRecordsPath is the runner's registry of measurement
// worktrees — the record that admits them to the census, written by the
// measurement side under the project root.
func measureWorktreeRecordsPath(projectRoot string) string {
	return filepath.Join(projectRoot, "artifacts", "agents", "measure-worktrees.jsonl")
}

// measureWorktreeRecord is one registered measurement worktree: the
// pinned candidate commit and the gate ref whose restored instrument
// bytes bound its lawful staged delta.
type measureWorktreeRecord struct {
	sha     string
	gateRef string
}

// recordedMeasureWorktrees reads the measurement-worktree registry:
// resolved path to its record.
func (e *Engine) recordedMeasureWorktrees() map[string]measureWorktreeRecord {
	data, err := os.ReadFile(measureWorktreeRecordsPath(e.Root))
	if err != nil {
		return map[string]measureWorktreeRecord{}
	}
	records := map[string]measureWorktreeRecord{}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		doc, err := decodeJSONDoc([]byte(line))
		if err != nil {
			continue
		}
		path, _ := doc["path"].(string)
		sha, _ := doc["sha"].(string)
		gateRef, _ := doc["gateRef"].(string)
		if path == "" || sha == "" {
			continue
		}
		if resolved, rerr := filepath.EvalSymlinks(path); rerr == nil {
			path = resolved
		}
		records[path] = measureWorktreeRecord{sha: sha, gateRef: gateRef}
	}
	return records
}

// judgeMeasureWorktree admits a registered measurement worktree only in
// the exact posture the runner creates: detached at the recorded pin,
// the pin already reachable from the judged HEAD (an ancestor retains
// nothing new — a registry entry can never launder a fresh commit), no
// private pseudorefs, and a staged delta that carries only the gate
// ref's restored instrument bytes.
func (e *Engine) judgeMeasureWorktree(record gittree.WorktreeRecord, pinned measureWorktreeRecord, capHead string) (string, error) {
	if !record.Detached || record.HeadOID != pinned.sha {
		return fmt.Sprintf("measurement worktree %s left its recorded tip", record.Path), nil
	}
	if capHead == "" {
		return fmt.Sprintf("measurement worktree %s cannot be judged against an unborn HEAD", record.Path), nil
	}
	if _, stderr, code := gitCaptured(e.Root, "merge-base", "--is-ancestor", pinned.sha, capHead); code != 0 {
		if code == -1 {
			return "", failf(3, "wall inspection cannot probe a measurement pin's ancestry: %s", strings.TrimSpace(stderr))
		}
		return fmt.Sprintf("measurement worktree %s pins a commit outside the judged history", record.Path), nil
	}
	// worktree-add itself leaves ORIG_HEAD at the pin; any private
	// pseudoref cargo beyond the pinned commit is a carrier.
	for _, ref := range record.Pseudorefs {
		if !ref.Parseable {
			return fmt.Sprintf("measurement worktree %s pseudoref %s carries unaccountable content", record.Path, ref.Name), nil
		}
		for _, oid := range ref.OIDs {
			if oid != pinned.sha {
				return fmt.Sprintf("measurement worktree %s pseudoref %s retains %s beyond its pin", record.Path, ref.Name, oid), nil
			}
		}
	}
	if !record.PostureReadable {
		return fmt.Sprintf("measurement worktree %s posture is unreadable; nothing can vouch for its carriers", record.Path), nil
	}
	workspace := gittree.Workspace{Dir: e.Root}
	pinTree, pinErr, code := gitCaptured(e.Root, "rev-parse", pinned.sha+"^{tree}")
	if code == -1 {
		return "", failf(3, "wall inspection cannot read a measurement pin: %s", strings.TrimSpace(pinErr))
	}
	if code != 0 {
		return fmt.Sprintf("measurement worktree %s pin is unreadable", record.Path), nil
	}
	changed, err := workspace.ChangedPaths(strings.TrimSpace(pinTree), record.Staged.Tree)
	if err != nil {
		return "", failf(3, "wall inspection cannot diff a measurement worktree's staged posture: %v", err)
	}
	if len(changed) == 0 && len(record.Staged.Unmerged) == 0 {
		return "", nil
	}
	if len(record.Staged.Unmerged) != 0 || pinned.gateRef == "" {
		return fmt.Sprintf("measurement worktree %s carries staged bytes outside its restored instruments", record.Path), nil
	}
	// The gate ref itself must live in the judged history: a registry
	// entry naming a fabricated instrument commit could otherwise bound
	// the staged delta with bytes the wall never saw.
	gateCommit, gateErr, gateCode := gitCaptured(e.Root, "rev-parse", "--verify", "--quiet", pinned.gateRef+"^{commit}")
	if gateCode == -1 {
		return "", failf(3, "wall inspection cannot resolve a measurement gate ref: %s", strings.TrimSpace(gateErr))
	}
	if gateCode != 0 {
		return fmt.Sprintf("measurement worktree %s records an unresolvable gate ref: %s", record.Path, strings.TrimSpace(gateErr)), nil
	}
	if _, ancestryErr, code := gitCaptured(e.Root, "merge-base", "--is-ancestor", strings.TrimSpace(gateCommit), capHead); code != 0 {
		if code == -1 {
			return "", failf(3, "wall inspection cannot probe a gate ref's ancestry: %s", strings.TrimSpace(ancestryErr))
		}
		return fmt.Sprintf("measurement worktree %s records a gate ref outside the judged history", record.Path), nil
	}
	gateEntries, err := workspace.Entries(pinned.gateRef+"^{tree}", changed)
	if err != nil {
		return "", failf(3, "wall inspection cannot read the gate ref's entries: %v", err)
	}
	stagedEntries, err := workspace.Entries(record.Staged.Tree, changed)
	if err != nil {
		return "", failf(3, "wall inspection cannot read a measurement worktree's staged entries: %v", err)
	}
	for _, path := range changed {
		if stagedEntries[path] != gateEntries[path] {
			return fmt.Sprintf("measurement worktree %s stages %s outside its restored instruments", record.Path, path), nil
		}
	}
	return "", nil
}

// dispatchWorktreeJob names the dispatch job whose disposable worktree
// lives at path, empty when the path is not a recorded job worktree.
func (e *Engine) dispatchWorktreeJob(path string) string {
	// Dispatch creates job worktrees under the PROJECT root's artifacts
	// tree — e.Root, not the repository toplevel, which differ in a
	// nested checkout.
	base := filepath.Join(e.Root, "artifacts", "agents", "worktrees")
	resolvedBase, err := filepath.EvalSymlinks(base)
	if err != nil {
		resolvedBase = base
	}
	rel, err := filepath.Rel(resolvedBase, path)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return ""
	}
	// The admitted worktree must BE the job's own directory: a nested
	// shadow under a valid job's path is nobody's record.
	if strings.Contains(filepath.ToSlash(rel), "/") {
		return ""
	}
	job := filepath.ToSlash(rel)
	record, err := readJSONDoc(filepath.Join(jobsDirPath(e.Root), job+".json"))
	if err != nil {
		return ""
	}
	if !numericEqual(record["mission"], e.Mission) {
		return ""
	}
	return job
}

// judgeWorktreeCensus runs rule 5's worktree lane: each worktree must be
// the mission workspace itself or one the runner's records name — a
// detached worktree under ignored space is otherwise a complete private
// carrier no main-checkout observable ever sees.
func (e *Engine) judgeWorktreeCensus(origin *scopeOrigin, cap *wallCapture, acct *wallAccountant, consumed map[string]bool) (string, error) {
	top, err := acct.workspace.TopLevel()
	if err != nil {
		return "", failf(3, "wall inspection cannot resolve the toplevel: %v", err)
	}
	resolvedTop, err := filepath.EvalSymlinks(top)
	if err != nil {
		resolvedTop = top
	}
	measured := e.recordedMeasureWorktrees()
	originByPath := map[string]map[string]any{}
	for _, record := range origin.Census {
		if path, _ := record["path"].(string); path != "" {
			originByPath[path] = record
		}
	}
	// Deletion is as loud as motion (rule 5): a consumed delegate
	// worktree recorded at the origin must still be present — removal
	// forgets a carrier the fence holds stationary.
	livePaths := map[string]bool{}
	for _, record := range cap.Census {
		livePaths[record.Path] = true
		if resolved, rerr := filepath.EvalSymlinks(record.Path); rerr == nil {
			livePaths[resolved] = true
		}
	}
	for path := range originByPath {
		if livePaths[path] {
			continue
		}
		if resolved, rerr := filepath.EvalSymlinks(path); rerr == nil && livePaths[resolved] {
			continue
		}
		if job := e.dispatchWorktreeJob(path); job != "" && consumed[job] {
			return fmt.Sprintf("consumed implementer worktree %s was deleted after consumption", path), nil
		}
	}
	for _, record := range cap.Census {
		resolved, rerr := filepath.EvalSymlinks(record.Path)
		if rerr != nil {
			resolved = record.Path
		}
		if resolved == resolvedTop {
			// The main checkout: its pseudorefs are the root-ref census.
			// PREEXISTING content recorded at the origin is not the
			// mission's product and is not judged (the delegate-fence
			// posture: motion from a boundary base, never preexisting
			// state); a pseudoref that CHANGED on the mission's watch must
			// carry only accounted-or-reviewed ids or be absent.
			recordedRefs := originPseudorefs(origin, record.Path)
			for _, ref := range record.Pseudorefs {
				if pseudorefUnchanged(recordedRefs, ref) {
					continue
				}
				if !ref.Parseable {
					return fmt.Sprintf("pseudoref %s carries content the census cannot account", ref.Name), nil
				}
				for _, oid := range ref.OIDs {
					detail, err := acct.accountedOID(e, oid)
					if err != nil {
						return "", err
					}
					if detail != "" {
						return fmt.Sprintf("pseudoref %s retains unaccounted bytes: %s", ref.Name, detail), nil
					}
				}
			}
			continue
		}
		if pinned, recorded := measured[resolved]; recorded {
			violation, err := e.judgeMeasureWorktree(record, pinned, cap.Head)
			if err != nil || violation != "" {
				return violation, err
			}
			continue
		}
		if job := e.dispatchWorktreeJob(resolved); job != "" {
			if !consumed[job] {
				continue // a delegate's worktree is its workspace until consumption
			}
			if !record.PostureReadable {
				return fmt.Sprintf("consumed implementer worktree %s posture is unreadable; nothing can vouch for its carriers", record.Path), nil
			}
			prior := originByPath[record.Path]
			if prior == nil {
				// STATIONARY from consumption on: a consumed worktree
				// absent at the recorded origin must stay absent — its
				// reappearance is a private carrier the fence forbids.
				return fmt.Sprintf("consumed implementer worktree %s reappeared after consumption", record.Path), nil
			}
			if violation := worktreePostureDrift(prior, record); violation != "" {
				return fmt.Sprintf("consumed implementer worktree %s %s", record.Path, violation), nil
			}
			continue
		}
		return fmt.Sprintf("unrecorded worktree %s (HEAD %s) is a private carrier on the host's watch", record.Path, record.HeadOID), nil
	}
	return "", nil
}

// originPseudorefs reads the origin census's pseudoref posture for one
// worktree path, name to ordered oids; nil parseable-flag info rides the
// doc shape.
type recordedPseudoref struct {
	oids      []string
	parseable bool
}

func originPseudorefs(origin *scopeOrigin, path string) map[string]recordedPseudoref {
	recorded := map[string]recordedPseudoref{}
	for _, record := range origin.Census {
		if p, _ := record["path"].(string); p != path {
			continue
		}
		refs, _ := record["pseudorefs"].([]any)
		for _, raw := range refs {
			ref, _ := raw.(map[string]any)
			if ref == nil {
				continue
			}
			name, _ := ref["name"].(string)
			parseable, _ := ref["parseable"].(bool)
			oids := []string{}
			if list, _ := ref["oids"].([]any); list != nil {
				for _, item := range list {
					if oid, _ := item.(string); oid != "" {
						oids = append(oids, oid)
					}
				}
			}
			recorded[name] = recordedPseudoref{oids: oids, parseable: parseable}
		}
	}
	return recorded
}

// pseudorefUnchanged reports whether a live pseudoref matches its
// recorded origin entry exactly — parseability included, so an
// unparseable edit that leaves the accounted OID list untouched still
// reads as changed and is re-judged.
func pseudorefUnchanged(recorded map[string]recordedPseudoref, live gittree.Pseudoref) bool {
	rec, present := recorded[live.Name]
	// An unparseable ref on EITHER side cannot be safely compared by its
	// collected OID list alone (two different unparseable contents can
	// share a list), so it never reads unchanged and is always re-judged.
	if !present || !rec.parseable || !live.Parseable || len(rec.oids) != len(live.OIDs) {
		return false
	}
	for i := range rec.oids {
		if rec.oids[i] != live.OIDs[i] {
			return false
		}
	}
	return true
}

// worktreePostureDrift compares a live worktree record against its
// recorded posture: HEAD, the private pseudoref census, and the logical
// staged serialization.
func worktreePostureDrift(recorded map[string]any, live gittree.WorktreeRecord) string {
	if head, _ := recorded["headOid"].(string); head != live.HeadOID {
		return fmt.Sprintf("moved its HEAD (recorded %s, now %s)", head, live.HeadOID)
	}
	recordedRefs, _ := recorded["pseudorefs"].([]any)
	liveRefs := live.Pseudorefs
	if len(recordedRefs) != len(liveRefs) {
		return "changed its private pseudoref census"
	}
	for i, raw := range recordedRefs {
		ref, _ := raw.(map[string]any)
		name, _ := ref["name"].(string)
		parseable, _ := ref["parseable"].(bool)
		oids, _ := ref["oids"].([]any)
		// An unparseable ref on either side is never proved unchanged by
		// its collected OID list — it always re-reads as drift.
		if name != liveRefs[i].Name || !parseable || !liveRefs[i].Parseable || len(oids) != len(liveRefs[i].OIDs) {
			return "changed its private pseudoref census"
		}
		for j, oid := range oids {
			if oid != liveRefs[i].OIDs[j] {
				return "changed its private pseudoref census"
			}
		}
	}
	staged, _ := recorded["staged"].(map[string]any)
	if staged == nil {
		// A consumed worktree whose record carries no staged baseline
		// can never be proved stationary: a readability transition after
		// consumption must re-judge, not silently pass.
		return "has no recorded staged baseline to prove its posture unchanged"
	}
	tree, _ := staged["tree"].(string)
	unmerged, _ := staged["unmerged"].([]any)
	if tree != live.Staged.Tree || len(unmerged) != len(live.Staged.Unmerged) {
		return "changed its staged posture"
	}
	for i, entry := range unmerged {
		if entry != live.Staged.Unmerged[i] {
			return "changed its staged posture"
		}
	}
	return ""
}

// judgeStaged runs rule 6: the workspace staged projection is ACCOUNTED
// (lawful subsets pass; equal-to-origin passes without re-judgment), and
// the toplevel staged posture moved only at workspace-prefixed paths.
func (e *Engine) judgeStaged(origin *scopeOrigin, cap *wallCapture, acct *wallAccountant) (string, error) {
	if cap.StagedConflict != "" {
		return "staged bytes unaccounted: " + cap.StagedConflict, nil
	}
	// Equal-to-HEAD's-tree is a member in its own right: a mission
	// admitted on a human-sealed dirty baseline lawfully keeps an index
	// that mirrors committed HEAD even though that tree decomposes
	// against nothing (the sealed tree is reviewable by equality).
	if cap.StagedTree != cap.HeadTree &&
		(origin.StagedPost == "" || cap.StagedTree != origin.StagedPost) {
		detail, err := acct.accountedTree(cap.StagedTree)
		if err != nil {
			return "", err
		}
		if detail != "" {
			return "staged bytes unaccounted: " + detail, nil
		}
	}
	if cap.Nested && origin.TopStaged != nil && !cap.TopStaged.Equal(*origin.TopStaged) {
		top, err := acct.workspace.TopLevel()
		if err != nil {
			return "", failf(3, "wall inspection cannot resolve the toplevel: %v", err)
		}
		topWorkspace := gittree.Workspace{Dir: top}
		changed, err := topWorkspace.ChangedPaths(origin.TopStaged.Tree, cap.TopStaged.Tree)
		if err != nil {
			return "", failf(3, "wall inspection cannot diff the toplevel staged posture: %v", err)
		}
		for _, path := range changed {
			if !strings.HasPrefix(path, acct.prefix) {
				return fmt.Sprintf("staged bytes unaccounted: sibling path %s moved in the toplevel index", path), nil
			}
		}
		// Unmerged entries are judged by serialization: sibling conflict
		// lines may neither appear nor vanish on the mission's watch.
		if unmergedDelta := unmergedOutsidePrefix(origin.TopStaged.Unmerged, cap.TopStaged.Unmerged, acct.prefix); unmergedDelta != "" {
			return "staged bytes unaccounted: " + unmergedDelta, nil
		}
	}
	return "", nil
}

// unmergedOutsidePrefix names an unmerged-entry transition outside the
// workspace prefix, empty when every delta is workspace-scoped.
func unmergedOutsidePrefix(before, after []string, prefix string) string {
	counts := map[string]int{}
	for _, entry := range before {
		counts[entry]++
	}
	for _, entry := range after {
		counts[entry]--
	}
	for entry, count := range counts {
		if count == 0 {
			continue
		}
		_, path, ok := strings.Cut(entry, "\t")
		if !ok || !strings.HasPrefix(path, prefix) {
			return fmt.Sprintf("unmerged entry %q changed in the toplevel index", entry)
		}
	}
	return ""
}

// judgeToplevelFence runs the sibling fence: every changed toplevel path
// since the origin carries the workspace prefix. ATTRIBUTION HONESTY: the
// violation asserts the change happened during the host's turn, never
// that the host authored it — a peer writing mid-turn surfaces the same
// way, which is the fence's own requirement: visibly.
func (e *Engine) judgeToplevelFence(origin *scopeOrigin, cap *wallCapture, acct *wallAccountant) (string, error) {
	if !cap.Nested || origin.TopTree == "" || origin.TopTree == cap.TopTree {
		return "", nil
	}
	top, err := acct.workspace.TopLevel()
	if err != nil {
		return "", failf(3, "wall inspection cannot resolve the toplevel: %v", err)
	}
	topWorkspace := gittree.Workspace{Dir: top}
	changed, err := topWorkspace.ChangedPaths(origin.TopTree, cap.TopTree)
	if err != nil {
		return "", failf(3, "wall inspection cannot diff the toplevel: %v", err)
	}
	sibling := []string{}
	for _, path := range changed {
		if !strings.HasPrefix(path, acct.prefix) {
			sibling = append(sibling, path)
		}
	}
	if len(sibling) > 0 {
		return "sibling paths changed in a nested checkout on the host's watch: " + strings.Join(sibling, ", "), nil
	}
	return "", nil
}

// judgeLedgerCarriers closes the filtered-path smuggling
// lane: the mission ledger is excluded from tree IDENTITY because it is
// force-tracked bookkeeping, but that exclusion must not let a host
// commit or stage ARBITRARY bytes into the ledger path. The committed
// HEAD ledger entry and the staged ledger entry must each be ABSENT or
// carry exactly the anchored ledger blob — the same authenticated truth
// the live-file guard proves for the worktree.
func (e *Engine) judgeLedgerCarriers(cap *wallCapture, state map[string]any) (string, error) {
	if cap.Unborn {
		return "", nil
	}
	ledgerPath := filepath.Join(missionDirPath(e.Root, e.Mission), "ledger.md")
	anchoredOID, err := mission.AnchoredLedgerBlobOID(e.Root, state, ledgerPath)
	if err != nil {
		if errors.Is(err, mission.ErrNoAnchor) {
			return "", nil // a mission that has never anchored (fresh beds)
		}
		// Any other anchor failure is the runner's own — never a silent
		// pass that disables the raw HEAD/index carrier comparison.
		return "", failf(3, "wall inspection cannot resolve the anchored ledger blob: %v", err)
	}
	workspace := gittree.Workspace{Dir: e.Root}
	ledgerRel := missionLedgerRel(e.Mission)
	headTree, err := workspace.TreeOf(cap.Head)
	if err != nil {
		return "", failf(3, "wall inspection cannot read HEAD's raw tree: %v", err)
	}
	rawStaged, serr := workspace.StagedTree()
	if serr != nil {
		if errors.Is(serr, gittree.ErrUnmergedWorkspaceIndex) {
			return "", nil // the conflicted-index refusal already fired in the staged rule
		}
		return "", failf(3, "wall inspection cannot project the raw staged tree: %v", serr)
	}
	for scope, tree := range map[string]string{"committed HEAD": headTree, "the index": rawStaged} {
		entries, err := workspace.Entries(tree, []string{ledgerRel})
		if err != nil {
			return "", failf(3, "wall inspection cannot read the ledger carrier in %s: %v", scope, err)
		}
		entry, present := entries[ledgerRel]
		// The COMPLETE entry authenticates — object id AND the regular
		// 100644 mode: an executable or symlink entry ships
		// a different object kind under the authenticated bytes.
		if present && (entry.OID != anchoredOID || entry.Mode != "100644") {
			return fmt.Sprintf("%s carries an unauthorized mission-ledger entry (%s %s)", scope, entry.Mode, entry.OID), nil
		}
	}
	return "", nil
}

// judgeCommitLedgerCarrier refuses a first-parent commit whose ledger
// path carries anything but the anchored ledger blob (or nothing).
func (e *Engine) judgeCommitLedgerCarrier(commit string, state map[string]any) (string, error) {
	ledgerPath := filepath.Join(missionDirPath(e.Root, e.Mission), "ledger.md")
	anchoredOID, err := mission.AnchoredLedgerBlobOID(e.Root, state, ledgerPath)
	if err != nil {
		if errors.Is(err, mission.ErrNoAnchor) {
			return "", nil // a mission that has never anchored
		}
		// Any other anchor error is the runner's own — never a silent
		// pass that disables the per-commit carrier check.
		return "", failf(3, "wall inspection cannot resolve the anchored ledger blob: %v", err)
	}
	workspace := gittree.Workspace{Dir: e.Root}
	tree, err := workspace.TreeOf(commit)
	if err != nil {
		// Ran-and-answered unreadable history — a first-parent commit
		// whose tree is missing or corrupt — is a violation on the
		// host's watch; only could-not-run rides the ramp.
		var runFailure *gittree.RunFailure
		if errors.As(err, &runFailure) {
			return "", failf(3, "wall inspection cannot read commit %s's tree: %v", commit, err)
		}
		return fmt.Sprintf("commit %s names unreadable history: %v", commit, err), nil
	}
	entries, err := workspace.Entries(tree, []string{missionLedgerRel(e.Mission)})
	if err != nil {
		var runFailure *gittree.RunFailure
		if errors.As(err, &runFailure) {
			return "", failf(3, "wall inspection cannot read commit %s's ledger carrier: %v", commit, err)
		}
		return fmt.Sprintf("commit %s names unreadable history: %v", commit, err), nil
	}
	if entry, present := entries[missionLedgerRel(e.Mission)]; present && (entry.OID != anchoredOID || entry.Mode != "100644") {
		return fmt.Sprintf("commit %s carries an unauthorized mission-ledger entry (%s %s)", commit, entry.Mode, entry.OID), nil
	}
	return "", nil
}

// judgeScope runs every snapshot-scope rule over one capture. The
// worktree equation (rule 7) stays in inspectWall.
func (e *Engine) judgeScope(origin *scopeOrigin, cap *wallCapture, acct *wallAccountant, state map[string]any) (string, error) {
	if len(cap.Steering) > 0 {
		// A grafts file forges every first-parent walk and a shallow
		// boundary truncates it: no accounting below is meaningful while
		// either exists.
		return "history-steering files present in the repository: " + strings.Join(cap.Steering, ", "), nil
	}
	consumed, err := e.consumedJobs(state)
	if err != nil {
		return "", err
	}
	// The committed HEAD toplevel tree is the retained-object sibling
	// baseline: two committed trees, so worktree dirt is never mistaken
	// for payload. Unborn or non-nested captures leave it empty (the
	// scope check is then vacuous).
	if cap.Nested && !cap.Unborn {
		committedTop, stderr, code := gitCaptured(e.Root, "rev-parse", cap.Head+"^{tree}")
		if code != 0 {
			// A probe that cannot resolve the committed toplevel tree is
			// the runner's failure — it must never silently vacate the
			// retained-object sibling scope.
			return "", failf(3, "wall inspection cannot resolve the committed toplevel tree: %s", firstDetail(stderr, committedTop))
		}
		acct.originTop = strings.TrimSpace(committedTop)
	}
	if violation, err := e.judgeLedgerCarriers(cap, state); violation != "" || err != nil {
		return violation, err
	}
	if violation, err := e.judgeHeadChain(origin, cap, acct); violation != "" || err != nil {
		return violation, err
	}
	if violation, err := e.judgeRefFence(origin, cap, state, consumed, acct); violation != "" || err != nil {
		return violation, err
	}
	if violation, err := e.judgeWorktreeCensus(origin, cap, acct, consumed); violation != "" || err != nil {
		return violation, err
	}
	if violation, err := e.judgeStaged(origin, cap, acct); violation != "" || err != nil {
		return violation, err
	}
	return e.judgeToplevelFence(origin, cap, acct)
}

// judgeCaptureIntegrity is the judgment every capture CONSUMER repeats
// on a fresh capture — history steering and the mission namespace —
// so a carrier planted between captures cannot ride an equality check
// that deliberately excludes the runner's own namespace.
func (e *Engine) judgeCaptureIntegrity(cap *wallCapture, openAnchor string, state map[string]any) (string, error) {
	if len(cap.Steering) > 0 {
		return "history-steering files present in the repository: " + strings.Join(cap.Steering, ", "), nil
	}
	if violation, err := e.judgeLedgerCarriers(cap, state); err != nil || violation != "" {
		return violation, err
	}
	return e.judgeMissionNamespace(cap.MissionRefs, openAnchor, state)
}

// scopeOriginFromOpenTurn builds the mid-turn origin from the open
// record's anchored fields. The census and staged origins come from the
// CHAIN's newest recorded posture (turn open and resume run the
// full accounting from the previous acceptance's recorded posture, turn
// one from the birth record) — the open record itself anchors HEAD, the
// ref map, and the toplevel trees.
func scopeOriginFromOpenTurn(openTurn map[string]any, state map[string]any) *scopeOrigin {
	origin := &scopeOrigin{}
	origin.Head, _ = openTurn["headCommit"].(string)
	origin.OpenAnchor = origin.Head
	origin.RefMap = refMapFromDoc(openTurn["refMap"])
	origin.TopTree, _ = openTurn["topTree"].(string)
	origin.TopStaged = stagedPostureFromDoc(openTurn["topStaged"])
	if prior := lastAcceptancePosture(state); prior != nil {
		origin.Census = prior.Census
		origin.StagedPost = prior.StagedPost
	}
	return origin
}

// scopeOriginFromPosture builds the between-turns origin from a recorded
// acceptance posture (or the birth record's admission origins).
func scopeOriginFromPosture(headCommit string, refMap any, topTree any, topStaged any, stagedPost string, census any) *scopeOrigin {
	// OpenAnchor stays empty between turns: turn-open-head holds the
	// PREVIOUS open's head (not this posture's), and it is not
	// load-bearing in the quiet period — the next open CAS-overwrites it
	// and continuity accounts from the recorded posture. Mid-turn
	// authentication uses the open record's own anchor.
	origin := &scopeOrigin{Head: headCommit, StagedPost: stagedPost}
	origin.RefMap = refMapFromDoc(refMap)
	if tree, _ := topTree.(string); tree != "" {
		origin.TopTree = tree
	}
	origin.TopStaged = stagedPostureFromDoc(topStaged)
	if list, _ := census.([]any); list != nil {
		for _, item := range list {
			if record, _ := item.(map[string]any); record != nil {
				origin.Census = append(origin.Census, record)
			}
		}
	}
	return origin
}

func refMapFromDoc(raw any) map[string]string {
	doc, _ := raw.(map[string]any)
	refs := map[string]string{}
	for name, value := range doc {
		if oid, _ := value.(string); oid != "" {
			refs[name] = oid
		}
	}
	return refs
}

func stagedPostureFromDoc(raw any) *gittree.StagedPosture {
	doc, _ := raw.(map[string]any)
	if doc == nil {
		return nil
	}
	tree, _ := doc["tree"].(string)
	posture := &gittree.StagedPosture{Tree: tree}
	if entries, _ := doc["unmerged"].([]any); entries != nil {
		for _, item := range entries {
			if line, _ := item.(string); line != "" {
				posture.Unmerged = append(posture.Unmerged, line)
			}
		}
	}
	return posture
}

// lastAcceptancePosture reads the newest recorded carrier posture from
// the CHAIN — never wall.json; evidence files are rewritable and prove
// nothing forward. A resolution that landed after the last acceptance
// is the newer origin (its posture is the ruled carrier state); the
// birth record's admission origins serve turn one.
func lastAcceptancePosture(state map[string]any) *scopeOrigin {
	var posture map[string]any
	bestSequence := int64(-1)
	turnLog, _ := state["turnLog"].([]any)
	for i := len(turnLog) - 1; i >= 0; i-- {
		entry, _ := turnLog[i].(map[string]any)
		if entry == nil {
			continue
		}
		wall, _ := entry["wall"].(map[string]any)
		if wall == nil {
			continue
		}
		point, _ := wall["sequencePoint"].(map[string]any)
		if sequence, ok := jsonInt(point["sequence"]); ok {
			posture = wall
			bestSequence = sequence
		}
		break
	}
	taint, _ := state["workspaceTaint"].(map[string]any)
	entries, _ := taint["entries"].([]any)
	for _, raw := range entries {
		entry, _ := raw.(map[string]any)
		if entry == nil {
			continue
		}
		resolution, _ := entry["resolution"].(map[string]any)
		if resolution == nil {
			continue
		}
		point, _ := resolution["sequencePoint"].(map[string]any)
		sequence, ok := jsonInt(point["sequence"])
		if !ok || sequence <= bestSequence {
			continue
		}
		if recorded, _ := resolution["posture"].(map[string]any); recorded != nil {
			posture = recorded
			bestSequence = sequence
		}
	}
	if posture != nil {
		head, _ := posture["headCommitPost"].(string)
		staged, _ := posture["stagedTreePost"].(string)
		return scopeOriginFromPosture(head, posture["refMapPost"], posture["topTreePost"], posture["topStagedPost"], staged, posture["worktreeCensusPost"])
	}
	origins, _ := state["admissionOrigins"].(map[string]any)
	if origins == nil {
		return nil
	}
	head, _ := origins["headCommit"].(string)
	return scopeOriginFromPosture(head, origins["refMap"], origins["topTree"], origins["topStaged"], "", origins["worktreeCensus"])
}
