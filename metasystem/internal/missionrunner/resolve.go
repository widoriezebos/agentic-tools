package missionrunner

// Taint resolution (host-implementer wall, HIW-O6): the ONLY two ways a
// tainted workspace reopens, both typed, both human-reserved. RESTORE
// names a recorded safe tree and the runner verifies the workspace equals
// it byte-for-byte before clearing; ADOPT_DISPUTED_TREE binds the
// observed tree, the human's identity, the reason, and the exact
// attribution claims being waived. Either resolution starts a NEW
// sequence segment — the resolution tree is E(next) — so every
// pre-resolution authorization refuses at the segment fence and needs
// fresh conformance. Adoption clears the operational taint but never
// fabricates authorization, never erases the violation record, and never
// earns delegation-floor credit. A generic free-text answer NEVER clears
// taint (the wall-violation ask refuses the answer path by name).

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
)

// ResolveTaint applies one typed resolution to one unresolved taint
// entry. variant is "restore" or "adopt-disputed-tree"; tree names the
// safe tree for restore and is IGNORED for adoption (the observed
// workspace is the adopted truth, computed here, never claimed).
func (e *Engine) ResolveTaint(taintID int64, variant, tree, resolvedBy, reason string, waived []string) int {
	// Human-reserved (the design's words): an agent classifying as MAIN,
	// DELEGATE, supervision, or adapter never resolves taint — the whole
	// point of the taint is that the machines stop until a human rules.
	if view, err := lease.Classify(e.Root, int64(os.Getpid())); err != nil {
		fmt.Fprintf(os.Stderr, "resolve refused: caller classification failed: %v\n", err)
		return 3
	} else if view.Class != lease.ClassHuman {
		fmt.Fprintf(os.Stderr, "resolve refused: taint resolution is a human-reserved act; this caller classifies %s\n", view.Class)
		return 3
	}
	resolvedBy = strings.TrimSpace(resolvedBy)
	reason = strings.TrimSpace(reason)
	if mission.BlankString(resolvedBy) || mission.BlankString(reason) {
		fmt.Fprintln(os.Stderr, "resolve refused: --by and --reason are required; the resolution records who ruled and why")
		return 2
	}
	// EVERY argument-shape refusal fires before the first state read
	// (round-4 finding 2, round-5 finding 1): entry verification may
	// reconcile — write and anchor a heal — and a malformed invocation
	// must not mutate anything on its way to a usage error. Cross-shape
	// arguments (waivers on restore, a tree on adoption), a nonpositive
	// taint id, and a malformed tree id all refuse here too.
	if taintID < 1 {
		fmt.Fprintln(os.Stderr, "resolve refused: the taint id must be a positive integer")
		return 2
	}
	switch variant {
	case "restore":
		if !resolveTreeRe.MatchString(tree) {
			fmt.Fprintln(os.Stderr, "resolve refused: restore names the recorded safe tree id (--restore <treeId>, 40-64 hex)")
			return 2
		}
		if len(waived) > 0 {
			fmt.Fprintln(os.Stderr, "resolve refused: restore waives nothing; --waives belongs to adoption")
			return 2
		}
	case "adopt-disputed-tree":
		if tree != "" {
			fmt.Fprintln(os.Stderr, "resolve refused: adoption binds the OBSERVED tree; a named tree belongs to restore")
			return 2
		}
		if len(waived) == 0 {
			fmt.Fprintln(os.Stderr, "resolve refused: adoption names the exact attribution claims being waived (--waives, repeatable)")
			return 2
		}
		for _, claim := range waived {
			if mission.BlankString(claim) {
				fmt.Fprintln(os.Stderr, "resolve refused: waived claims must be non-blank")
				return 2
			}
		}
	default:
		fmt.Fprintln(os.Stderr, "resolve refused: variant must be restore or adopt-disputed-tree")
		return 2
	}

	statePath := filepath.Join(e.missionDir(), "state.json")
	ledgerPath := filepath.Join(e.missionDir(), "ledger.md")
	// An UNPARSABLE ledger refuses before any verification or heal can
	// write (rounds 11-12, finding 1): a resolution over bytes the
	// ledger machinery cannot read would hand back a running mission
	// nothing can drive, and the anchor now refuses such bytes anyway —
	// the park deferred its anchor, the taint is recorded, the STOP
	// holds. The repair is byte-restoration from the anchor ref, then
	// this verb; the one-step lag-heal bridges the deferred anchor.
	if _, _, _, lerr := mission.ParseLedger(ledgerPath); lerr != nil {
		fmt.Fprintf(os.Stderr, "resolve refused: the mission ledger is unparsable (%v); restore its bytes from the anchor ref (git show <anchor>:%s), then re-run\n", lerr, missionLedgerRel(e.Mission))
		return 3
	}
	// The starting state must be ANCHOR-VERIFIED (round-2 finding 1): a
	// resolution that anchors an unanchored prior write would launder it.
	// A lagging anchor from a clean crash reconciles first — the same
	// heal the runner runs at resume. The VERIFIED hash is remembered:
	// the resolution's compare-and-write pins to it, so a state that
	// moves between this read and the write refuses instead of becoming
	// the resolution's silent base (round-3 finding 2).
	_, verifiedHash, err := mission.VerifyStateWithAnchor(statePath, e.Root, ledgerPath)
	if err != nil {
		if code, rerr := mission.Reconcile(statePath, e.Root, ledgerPath); rerr != nil || code != 0 {
			// The reconciliation refusal carries the ACTIONABLE repair
			// (round-16 finding 5) — surface it, not the generic
			// verification failure it explains.
			if rerr != nil {
				fmt.Fprintln(os.Stderr, rerr)
				return exitFor(rerr)
			}
			fmt.Fprintln(os.Stderr, err)
			return exitFor(err)
		}
		if _, verifiedHash, err = mission.VerifyStateWithAnchor(statePath, e.Root, ledgerPath); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return exitFor(err)
		}
	}
	state, err := readDocLabeled(statePath, "mission state", 7)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return exitFor(err)
	}
	if integrity, _ := state["integrity"].(map[string]any); integrity == nil || integrity["hash"] != verifiedHash {
		fmt.Fprintln(os.Stderr, "resolve refused: the mission state changed during verification; re-run")
		return 3
	}
	taint, _ := state["workspaceTaint"].(map[string]any)
	entries, _ := taint["entries"].([]any)
	var entry map[string]any
	for _, raw := range entries {
		candidate, _ := raw.(map[string]any)
		if candidate == nil {
			continue
		}
		if id, ok := jsonInt(candidate["taintId"]); ok && id == taintID {
			entry = candidate
			break
		}
	}
	if entry == nil {
		fmt.Fprintf(os.Stderr, "resolve refused: mission %s has no taint entry %d\n", e.Mission, taintID)
		return 3
	}
	if recorded, _ := entry["resolution"].(map[string]any); recorded != nil {
		// The resolution is durable but a crash may have left its asks
		// unanswered (round-2 finding 4): complete the tail from the
		// RECORDED ruling — never from this call's arguments — or
		// refuse when there is nothing left to do.
		if e.openBoundAsks(taintID) == 0 {
			fmt.Fprintf(os.Stderr, "resolve refused: taint %d is already resolved\n", taintID)
			return 3
		}
		if err := e.answerWallViolationAsks(taintID, recorded); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return exitFor(err)
		}
		// The recorded waiting list re-derives after the late answers
		// (round-3 finding 6) — a lawful non-resolution write.
		repaired := deepCopyDoc(state)
		repaired["waitingList"] = openAskIDs(asksDirPath(e.Root, e.Mission))
		// Pinned to the VERIFIED hash (successor finding 5): if the
		// runner moved the state meanwhile, this stale proposal refuses
		// instead of legally clearing a marker it never saw.
		tailFinal, err := e.writeStateWith(statePath, repaired, verifiedHash, mission.WriteState)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return exitFor(err)
		}
		tailIntegrity, _ := tailFinal["integrity"].(map[string]any)
		tailHash, _ := tailIntegrity["hash"].(string)
		if e.preAnchorHook != nil {
			// Test seam (round-14 finding 2): movement lands BEFORE the
			// pin is acquired.
			e.preAnchorHook()
		}
		// The tail's ledger pin ORIGINATES at the entry verification
		// (round-11 finding 2): the tail touches no ledger bytes, so the
		// lawful position is exactly the one the anchor tip certified
		// when this call verified it — never a post-write reread, which
		// would self-select moved bytes.
		anchoredSHA, err := e.verifiedLedgerPin()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return exitFor(err)
		}
		if err := e.anchorStatePinned(statePath, ledgerPath, resolvedBy, tailHash, anchoredSHA); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return exitFor(err)
		}
		fmt.Printf("mission=%s taint=%d resolution tail completed (recorded %s by %s)\n",
			e.Mission, taintID, recorded["variant"], recorded["resolvedBy"])
		return 0
	}

	// The workspace's CURRENT filtered projection — the tree both
	// variants rule about.
	workspace := gittree.Workspace{Dir: e.Root}
	observed, err := wallSnapshot(workspace, e.Mission)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve refused: cannot snapshot the workspace: %v\n", err)
		return 3
	}

	resolution := map[string]any{
		"resolvedAt": nowISO(),
		"resolvedBy": resolvedBy,
		"reason":     reason,
	}
	switch variant {
	case "restore":
		// The mission ledger sits OUTSIDE the restorable tree projection
		// (round-2 finding 2): a ledger-domain violation cannot be proved
		// restored by tree equality, and the park itself appended to the
		// disputed file. Adoption — with its named waivers and its
		// re-baselined anchored truth — is the one lawful closure.
		if reason, _ := entry["reason"].(string); strings.HasPrefix(reason, ledgerViolationPrefix) {
			fmt.Fprintf(os.Stderr, "resolve refused: taint %d disputes the mission ledger, which sits outside the restorable tree projection; use adopt-disputed-tree\n", taintID)
			return 3
		}
		// The named tree must BE a recorded SAFE tree (round-1 critique
		// F2: the violated post-tree also sits recorded and anchored in
		// wall.json — accepting it as a restore would be adoption without
		// its named waivers). Safe = the violated turn's pre-tree, an
		// accepted expected-tree point, or an earlier resolution's tree.
		if !e.recordedSafeTree(state, entry, tree) {
			fmt.Fprintf(os.Stderr, "resolve refused: %s is not a recorded safe tree for taint %d (the violated pre-tree, an accepted point, or an earlier resolution); to keep the disputed tree use adopt-disputed-tree\n", tree, taintID)
			return 3
		}
		// Exact equality, verified — never trusted: the human restores
		// the workspace first, then the runner proves it.
		if observed != tree {
			fmt.Fprintf(os.Stderr, "resolve refused: the workspace (%s) does not equal the named safe tree (%s); restore the files first\n", observed, tree)
			return 3
		}
		resolution["variant"] = "restore"
		resolution["treeId"] = tree
	case "adopt-disputed-tree":
		claims := make([]any, 0, len(waived))
		for _, claim := range waived {
			claims = append(claims, strings.TrimSpace(claim))
		}
		resolution["variant"] = "adopt-disputed-tree"
		resolution["treeId"] = observed
		resolution["waivedClaims"] = claims
	}

	// The resolution is a NAMED E-point (round-1 critique F3): it binds
	// the occurrence it will land at — pinned by the transition validator
	// exactly like an acceptance payload — and records the expected tree
	// it replaces, so the wall can measure the resolution's own delta.
	integrity, _ := state["integrity"].(map[string]any)
	chainSequence, ok := jsonInt(integrity["sequence"])
	if !ok {
		fmt.Fprintln(os.Stderr, "resolve refused: cannot read the state chain sequence")
		return 3
	}
	segment, ok := jsonInt(taint["segment"])
	if !ok {
		fmt.Fprintln(os.Stderr, "resolve refused: cannot read the taint segment")
		return 3
	}
	previousTree := ""
	if points := mission.ExpectedTreePoints(state); len(points) > 0 {
		previousTree = points[len(points)-1].Tree
	}
	if previousTree == "" {
		if openTurn, _ := state["openTurn"].(map[string]any); openTurn != nil {
			previousTree, _ = openTurn["preTree"].(string)
		}
	}
	if previousTree == "" {
		turnID, _ := entry["turnId"].(string)
		if wall, err := readJSONDoc(filepath.Join(missionDirPath(e.Root, e.Mission), "turns", turnID, "wall.json")); err == nil {
			previousTree, _ = wall["preTree"].(string)
		}
	}
	if previousTree == "" {
		fmt.Fprintln(os.Stderr, "resolve refused: cannot determine the pre-resolution expected tree")
		return 3
	}
	resolution["previousTree"] = previousTree
	resolution["sequencePoint"] = map[string]any{"sequence": chainSequence + 1, "segment": segment + 1}

	// The resolution tree is E(next): anchor it for the mission's life.
	resolvedTree, _ := resolution["treeId"].(string)
	if err := workspace.Anchor(e.Mission, resolvedTree); err != nil {
		fmt.Fprintf(os.Stderr, "resolve refused: cannot anchor the resolution tree: %v\n", err)
		return 3
	}

	// Multi-taint discipline (F5): the marker closes and the mission
	// unparks only when THIS resolution leaves no unresolved taint.
	remaining := 0
	for _, raw := range entries {
		candidate, _ := raw.(map[string]any)
		if candidate == nil || candidate["resolution"] != nil {
			continue
		}
		if id, ok := jsonInt(candidate["taintId"]); ok && id != taintID {
			remaining++
		}
	}

	// The verification-to-write gap closes to one re-check (F6): the
	// workspace must STILL be the ruled tree at the last moment before
	// the write; drift in between refuses rather than becoming the next
	// turn's silently grandfathered baseline. The residual instant
	// between this check and the write is named: a single-writer human
	// operation on a parked mission.
	recheck, err := wallSnapshot(workspace, e.Mission)
	if err != nil || recheck != observed {
		fmt.Fprintf(os.Stderr, "resolve refused: the workspace changed during resolution (%s -> %s); re-run\n", observed, recheck)
		return 3
	}
	// The LEDGER is outside that projection and gets its own recheck
	// (round-3 finding 3): late ledger bytes must never ride the
	// resolution's anchor into the guard's baseline. The one exception
	// is adopting a ledger-domain taint — there the current bytes ARE
	// the adopted, waived truth.
	ledgerDomain := strings.HasPrefix(func() string { r, _ := entry["reason"].(string); return r }(), ledgerViolationPrefix)
	ledgerSHA := ""
	if variant == "adopt-disputed-tree" && ledgerDomain {
		adoptedBytes, rerr := os.ReadFile(ledgerPath)
		if rerr != nil {
			fmt.Fprintf(os.Stderr, "resolve refused: cannot read the mission ledger: %v\n", rerr)
			return 3
		}
		ledgerSHA = sha256Hex(string(adoptedBytes))
	} else {
		anchored, currentLedger, lerr := mission.AnchoredLedgerTruth(e.Root, state, ledgerPath)
		if lerr != nil && !errors.Is(lerr, mission.ErrNoAnchor) {
			fmt.Fprintf(os.Stderr, "resolve refused: cannot verify the mission ledger: %v\n", lerr)
			return 3
		}
		if lerr == nil {
			if anchored != currentLedger {
				fmt.Fprintln(os.Stderr, "resolve refused: the mission ledger changed during resolution; reconcile and re-run")
				return 3
			}
			ledgerSHA = sha256Hex(currentLedger)
		}
	}

	proposed := deepCopyDoc(state)
	pTaint, _ := proposed["workspaceTaint"].(map[string]any)
	pEntries, _ := pTaint["entries"].([]any)
	for _, raw := range pEntries {
		candidate, _ := raw.(map[string]any)
		if candidate == nil {
			continue
		}
		if id, ok := jsonInt(candidate["taintId"]); ok && id == taintID {
			candidate["resolution"] = resolution
		}
	}
	pTaint["segment"] = segment + 1
	if remaining == 0 {
		proposed["openTurn"] = nil
		if proposed["parkReason"] == "wall-violation" {
			proposed["status"] = "running"
			proposed["parkReason"] = nil
			proposed["gatePassed"] = false
		}
	}
	// The waiting list the write records EXCLUDES the asks this
	// resolution is about to answer — the answers land right after the
	// write, and a crash in between leaves only late answers, never a
	// ruling that disagrees with state (round-2 finding 4). The order is
	// RECOVERABLE: write, anchor, answer-from-state; every later step
	// derives from the durable record, and the tail-completion path
	// above re-runs it.
	proposed["waitingList"] = e.openAskIDsExcludingTaint(taintID)
	written, err := e.writeStateResolution(statePath, proposed, verifiedHash)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return exitFor(err)
	}
	writtenIntegrity, _ := written["integrity"].(map[string]any)
	writtenHash, _ := writtenIntegrity["hash"].(string)
	// The anchor binds EXACTLY the ruled position (successor finding 2):
	// the state hash this resolution wrote and the ledger bytes its
	// recheck examined — anything that moved in between refuses.
	if err := e.anchorStatePinned(statePath, ledgerPath, resolvedBy, writtenHash, ledgerSHA); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return exitFor(err)
	}
	if err := e.answerWallViolationAsks(taintID, resolution); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return exitFor(err)
	}
	e.emit("taint-resolved", fmt.Sprintf("taint %d %s", taintID, variant), map[string]string{
		"missionId": e.Mission, "taintId": fmt.Sprintf("%d", taintID),
		"variant": variant, "resolvedBy": resolvedBy,
	})
	fmt.Printf("mission=%s taint=%d resolved=%s segment=%d tree=%s remaining=%d\n",
		e.Mission, taintID, variant, segment+1, resolvedTree, remaining)
	return 0
}

var resolveTreeRe = regexp.MustCompile(`^[0-9a-f]{40,64}$`)

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// verifiedLedgerPin is the ONE pin origin for anchors of positions
// whose write touches no ledger bytes (successor rounds 11-12): the
// anchor tip's recorded sha — the position the caller's entry
// verification proved — never a fresh file read, which would
// self-select any bytes moved since.
func (e *Engine) verifiedLedgerPin() (string, error) {
	return mission.AnchoredLedgerSHA(e.Root, e.Mission)
}

// recordedSafeTree reports whether a tree is RECORDED as safe for this
// taint, from HASH-CHAINED evidence only (round-2 finding 2: on-disk
// wall.json is rewritable by the offender): the open marker's pre-tree,
// a named expected-tree point, or an earlier resolution's tree. The
// violated POST-tree is deliberately not safe — keeping it is adoption,
// with its named waivers.
func (e *Engine) recordedSafeTree(state, entry map[string]any, tree string) bool {
	if openTurn, _ := state["openTurn"].(map[string]any); openTurn != nil {
		if pre, _ := openTurn["preTree"].(string); pre == tree {
			return true
		}
	}
	for _, point := range mission.ExpectedTreePoints(state) {
		if point.Tree == tree {
			return true
		}
	}
	taint, _ := state["workspaceTaint"].(map[string]any)
	entries, _ := taint["entries"].([]any)
	for _, raw := range entries {
		candidate, _ := raw.(map[string]any)
		if candidate == nil {
			continue
		}
		resolution, _ := candidate["resolution"].(map[string]any)
		if resolution == nil {
			continue
		}
		if resolved, _ := resolution["treeId"].(string); resolved == tree {
			return true
		}
	}
	return false
}

// answerWallViolationAsks closes THIS taint's open wall-violation asks
// from the STATE-RECORDED resolution — asks carry taintId since the park
// bound them (F5), and matching FAILS CLOSED (round-2 finding 7): an ask
// without a parseable bound taint id is never answered here. Idempotent:
// answered asks are skipped, so the tail-completion path re-runs cleanly.
func (e *Engine) answerWallViolationAsks(taintID int64, resolution map[string]any) error {
	variant, _ := resolution["variant"].(string)
	resolvedBy, _ := resolution["resolvedBy"].(string)
	asksDir := asksDirPath(e.Root, e.Mission)
	paths, _ := filepath.Glob(filepath.Join(asksDir, "*.json"))
	for _, path := range paths {
		ask, err := readJSONDoc(path)
		if err != nil {
			continue
		}
		if ask["reasonClass"] != "wall-violation" || ask["answeredAt"] != nil {
			continue
		}
		if bound, ok := jsonInt(ask["taintId"]); !ok || bound != taintID {
			continue
		}
		ask["answeredAt"] = nowISO()
		ask["answer"] = fmt.Sprintf("resolved by %s: taint %d %s (typed resolution; recorded in mission state)", resolvedBy, taintID, variant)
		if err := atomicWriteJSON(path, ask); err != nil {
			return err
		}
	}
	return nil
}

// openBoundAsks counts this taint's still-open wall-violation asks.
func (e *Engine) openBoundAsks(taintID int64) int {
	asksDir := asksDirPath(e.Root, e.Mission)
	paths, _ := filepath.Glob(filepath.Join(asksDir, "*.json"))
	open := 0
	for _, path := range paths {
		ask, err := readJSONDoc(path)
		if err != nil {
			continue
		}
		if ask["reasonClass"] != "wall-violation" || ask["answeredAt"] != nil {
			continue
		}
		if bound, ok := jsonInt(ask["taintId"]); ok && bound == taintID {
			open++
		}
	}
	return open
}

// openAskIDsExcludingTaint is the waiting list AFTER this resolution's
// answers land: every open ask except the ones bound to the taint being
// resolved.
func (e *Engine) openAskIDsExcludingTaint(taintID int64) []string {
	asksDir := asksDirPath(e.Root, e.Mission)
	ids := []string{}
	for _, askID := range openAskIDs(asksDir) {
		bound := false
		paths, _ := filepath.Glob(filepath.Join(asksDir, "*.json"))
		for _, path := range paths {
			ask, err := readJSONDoc(path)
			if err != nil || ask["askId"] != askID {
				continue
			}
			if ask["reasonClass"] == "wall-violation" {
				if id, ok := jsonInt(ask["taintId"]); ok && id == taintID {
					bound = true
				}
			}
			break
		}
		if !bound {
			ids = append(ids, askID)
		}
	}
	return ids
}
