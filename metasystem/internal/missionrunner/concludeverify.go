package missionrunner

// The post-publication verification: the acceptance
// append stays the single commit point, but the turn CONCLUDES only when
// a fresh capture matches the posture the acceptance recorded — a
// mismatch appends a violation over the acceptance before any success
// surfaces. An acceptance entry with no verification entry is the
// defined consumed-but-unconcluded state; resume re-runs this
// verification deterministically against the recorded posture, so a
// crash between the two writes can never leave a completed mission over
// unprobed motion, and consumption is never double-spent because the
// commit point already landed.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path/filepath"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
)

// jsonDocEqual compares two JSON-shaped values by canonical encoding.
func jsonDocEqual(a, b any) bool {
	aBytes, aErr := json.Marshal(a)
	bBytes, bErr := json.Marshal(b)
	return aErr == nil && bErr == nil && string(aBytes) == string(bBytes)
}

// acceptancePayloadMismatch verifies a proposed acceptance entry's wall
// payload against the capture this process judged: the posture block,
// the composed expected tree, and the projected post tree must all be
// EXACTLY what the verified capture says — evidence files transport the
// values, but the capture is the authority.
func acceptancePayloadMismatch(proposed map[string]any, turnID string, ctx *wallScopeContext, gatePassed bool, measurement any) string {
	entry := acceptanceEntryFor(proposed, turnID)
	if entry == nil {
		// A proposal without an acceptance entry answers to the
		// transition validator's own shape rules; the capture-authority
		// check binds only what claims to BE the acceptance.
		return ""
	}
	wall, _ := entry["wall"].(map[string]any)
	if pre, _ := wall["preTree"].(string); pre != ctx.PreTree {
		return "acceptance payload names a pre-tree the turn never opened on"
	}
	if expected, _ := wall["expectedTree"].(string); expected != ctx.Expected {
		return "acceptance payload names an expected tree the inspection never composed"
	}
	if post, _ := wall["postTree"].(string); post != ctx.Capture.Post {
		return "acceptance payload names a post tree the capture never projected"
	}
	ordered := make([]any, 0, len(ctx.OrderedDigests))
	for _, digest := range ctx.OrderedDigests {
		ordered = append(ordered, digest)
	}
	if !jsonDocEqual(wall["orderedDigests"], ordered) {
		return "acceptance payload consumption order differs from the inspection"
	}
	if !jsonDocEqual(entry["consumedAuthorizations"], ordered) {
		return "acceptance payload consumption list differs from the inspection"
	}
	if capturedAt, _ := wall["capturedAt"].(string); capturedAt != ctx.Capture.CapturedAt {
		return "acceptance payload capture instant differs from the verified capture"
	}
	// The recovery record answers to the gate this process ran, exactly
	// like every other wall field: a payload claiming a recovery the gate
	// never performed — or omitting one it did — was built over tampered
	// evidence.
	if !jsonDocEqual(wall["recovered"], ctx.Recovered) {
		return "acceptance payload recovery record differs from the gate's verdict"
	}
	if recorded, _ := entry["gatePassed"].(bool); recorded != gatePassed {
		return "acceptance payload gate verdict differs from the measured truth"
	}
	if !jsonDocEqual(entry["measurement"], measurement) {
		return "acceptance payload measurement differs from the measured truth"
	}
	verified := ctx.Capture.postureDoc(missionIDOfPosture(proposed))
	for field, want := range verified {
		if field == "capturedAt" {
			continue
		}
		if !jsonDocEqual(wall[field], want) {
			return "acceptance payload posture differs from the verified capture at " + field
		}
	}
	return ""
}

// missionIDOfPosture reads the proposal's mission id for posture
// rendering.
func missionIDOfPosture(proposed map[string]any) string {
	id, _ := proposed["missionId"].(string)
	return id
}

// acceptanceEntryFor finds the acceptance entry (wall payload and all)
// a verification concludes, nil when none exists.
func acceptanceEntryFor(state map[string]any, turnID string) map[string]any {
	turnLog, _ := state["turnLog"].([]any)
	for i := len(turnLog) - 1; i >= 0; i-- {
		entry, _ := turnLog[i].(map[string]any)
		if entry == nil {
			continue
		}
		if id, _ := entry["turnId"].(string); id != turnID {
			continue
		}
		if kind, _ := entry["kind"].(string); kind == mission.WallVerificationKind {
			continue
		}
		if entry["wall"] != nil {
			return entry
		}
	}
	return nil
}

// captureMismatch compares a live capture against the acceptance
// payload's RECORDED posture — the chain, never wall.json — and names
// the first carrier that moved; empty means the postures match.
// A non-nil error is the RUNNER's own failure to judge —
// the fail ramp, never false wall evidence.
func (e *Engine) captureMismatch(capture *wallCapture, wall map[string]any, state map[string]any) (string, error) {
	openAnchor := ""
	if openTurn, _ := state["openTurn"].(map[string]any); openTurn != nil {
		openAnchor, _ = openTurn["headCommit"].(string)
	}
	if violation, err := e.judgeCaptureIntegrity(capture, openAnchor, state); err != nil || violation != "" {
		if err != nil {
			return "", err
		}
		return violation, nil
	}
	if capture.StagedConflict != "" {
		return "the workspace index became conflicted: " + capture.StagedConflict, nil
	}
	if head, _ := wall["headCommitPost"].(string); capture.Head != head || capture.Unborn {
		return "committed HEAD moved after the acceptance write", nil
	}
	branch, _ := state["branch"].(string)
	if capture.Detached || capture.Branch != "refs/heads/"+branch {
		return "the checkout left the mission branch after the acceptance write", nil
	}
	if post, _ := wall["postTree"].(string); capture.Post != post {
		return "the worktree projection moved after the acceptance write", nil
	}
	if staged, _ := wall["stagedTreePost"].(string); capture.StagedTree != staged {
		return "the staged projection moved after the acceptance write", nil
	}
	liveRefs := map[string]any{}
	for name, oid := range capture.RefMap {
		liveRefs[name] = oid
	}
	if !jsonDocEqual(liveRefs, wall["refMapPost"]) {
		return "the ref map moved after the acceptance write", nil
	}
	var topTree any
	var topStaged any
	if capture.Nested {
		topTree = capture.TopTree
		topStaged = mission.StagedPostureDoc(capture.TopStaged)
	}
	if !jsonDocEqual(topTree, wall["topTreePost"]) {
		return "the toplevel tree moved after the acceptance write", nil
	}
	if !jsonDocEqual(topStaged, wall["topStagedPost"]) {
		return "the toplevel staged posture moved after the acceptance write", nil
	}
	if !jsonDocEqual(mission.WorktreeCensusDoc(capture.Census), wall["worktreeCensusPost"]) {
		return "the worktree census moved after the acceptance write", nil
	}
	return "", nil
}

// verifyAcceptance runs the post-publication verification for the
// newest acceptance of one turn: re-capture the posture, compare it to
// the recorded one, and either conclude the turn with the verification
// entry or append a violation over the acceptance (taint, park). It
// reports the resulting state and whether the mission was parked.
func (e *Engine) verifyAcceptance(statePath, ledger, turnID, turnDir string, cycle int64, declared map[string]bool) (map[string]any, bool, error) {
	diskState, err := readDocLabeled(statePath, "mission state", 3)
	if err != nil {
		return nil, false, err
	}
	entry := acceptanceEntryFor(diskState, turnID)
	if entry == nil {
		return nil, false, failf(3, "post-verification found no acceptance entry for turn %s", turnID)
	}
	wall, _ := entry["wall"].(map[string]any)
	expected, _ := wall["expectedTree"].(string)
	capture, err := e.captureWallPostureStable(expected, declared)
	if err != nil {
		if answer := stateAnswerOf(err); answer != "" {
			final, perr := e.parkWallViolation(statePath, ledger, turnID, turnDir, cycle, answer, diskState, true, "")
			return final, true, perr
		}
		return nil, false, err
	}
	mismatch, merr := e.captureMismatch(capture, wall, diskState)
	if merr != nil {
		// A ran-and-answered probe defeat is a wall answer here exactly
		// as at the gate; only could-not-run keeps the ramp.
		if answer := stateAnswerOf(merr); answer != "" {
			mismatch = answer
		} else {
			return nil, false, merr
		}
	}
	if mismatch != "" {
		violation := "repository moved between the acceptance write and its post-verification: " + mismatch
		// The evidence records the posture that caused the park BESIDE the
		// violation — sticky for every re-execution of this park.
		if evidence, rerr := readJSONDoc(filepath.Join(turnDir, "wall.json")); rerr == nil {
			evidence["violation"] = violation
			evidence["verdict"] = "violated"
			evidence["postureAtVerification"] = capture.postureDoc(e.Mission)
			if werr := atomicWriteJSON(filepath.Join(turnDir, "wall.json"), evidence); werr != nil {
				return nil, false, werr
			}
		}
		final, perr := e.parkWallViolation(statePath, ledger, turnID, turnDir, cycle, violation, diskState, true, "")
		return final, true, perr
	}
	final, err := e.concludeVerification(statePath, ledger, turnID, cycle, capture.CapturedAt)
	if err != nil {
		return nil, false, err
	}
	// SUCCESS surfaces only here: an accepted turn's record turns
	// terminal after — never before — its posture verified (crash
	// between acceptance and verification leaves a non-terminal record,
	// and this same path completes it at resume).
	if entry := acceptanceEntryFor(final, turnID); entry != nil {
		if outcome, _ := entry["outcome"].(string); outcome == "completed" {
			if _, perr := patchTurn(filepath.Join(turnDir, "turn.json"), map[string]any{
				"status": "completed", "outcome": "completed", "error": nil,
				"detail": "host return accepted; posture verified", "endedAt": nowISO(),
			}); perr != nil {
				return nil, false, perr
			}
		}
	}
	return final, false, nil
}

// concludeVerification is the concluding write: the verification entry
// appends, the open-turn marker dies, and completion — the one success
// outcome the acceptance deferred — lands when the acceptance's gate
// passed.
func (e *Engine) concludeVerification(statePath, ledger, turnID string, cycle int64, capturedAt string) (map[string]any, error) {
	diskState, err := readDocLabeled(statePath, "mission state", 3)
	if err != nil {
		return nil, err
	}
	entry := acceptanceEntryFor(diskState, turnID)
	if entry == nil {
		return nil, failf(3, "post-verification found no acceptance entry for turn %s", turnID)
	}
	gatePassed, _ := entry["gatePassed"].(bool)
	// The pin origin for the closing anchor is read BEFORE the write,
	// through the authenticated pass itself: the bytes the
	// hash covers are the bytes the anchor machinery just proved equal
	// to the acceptance's anchored truth — never a separate reread.
	provenSHA := ""
	ledgerPath := filepath.Join(e.missionDir(), "ledger.md")
	if anchored, current, terr := mission.AnchoredLedgerTruth(e.Root, diskState, ledgerPath); terr == nil {
		if anchored != current {
			return nil, failf(3, "mission ledger moved between the acceptance anchor and its verification")
		}
		sum := sha256.Sum256([]byte(current))
		provenSHA = hex.EncodeToString(sum[:])
	} else if !errors.Is(terr, mission.ErrNoAnchor) {
		return nil, failf(3, "post-verification cannot read the anchored ledger truth: %v", terr)
	}
	proposed := deepCopyDoc(diskState)
	turnLog, _ := proposed["turnLog"].([]any)
	proposed["turnLog"] = append(turnLog, map[string]any{
		"turnId": turnID, "kind": mission.WallVerificationKind,
		"capturedAt": capturedAt, "verdict": "clean",
	})
	proposed["openTurn"] = nil
	if gatePassed {
		proposed["status"] = "completed"
		proposed["parkReason"] = nil
		proposed["gatePassed"] = true
	}
	updated, err := e.writeState(statePath, proposed)
	if err != nil {
		return nil, err
	}
	// Terminal delivery runs AFTER the concluding write is durable:
	// the write owns the transition, and a crash on either side
	// of the delivery heals idempotently at resume — while the closing
	// anchor below still binds the annotated bytes because delivery
	// precedes it. The append itself refuses if the ledger moved past
	// the proven position.
	pinSHA := provenSHA
	if gatePassed {
		pinSHA = e.deliverLandedUnconsumed(ledgerPath, cycle, updated, provenSHA)
	}
	// The open-commit anchor retires only AFTER the concluding write is
	// durable: dropping it first would break the mid-turn presence
	// invariant if the write failed, while a crash after the write leaves
	// a stale ref the quiet period ignores and the next open CAS-overwrites.
	e.dropTurnOpenHead()
	if pinSHA != "" {
		if err := e.anchorPinnedTo(statePath, ledger, turnID, stateIntegrityHash(updated), pinSHA); err != nil {
			return nil, err
		}
		return updated, nil
	}
	if err := e.anchor(statePath, ledger, turnID); err != nil {
		return nil, err
	}
	return updated, nil
}

// healTerminalPublication idempotently finishes a COMPLETED mission's
// terminal publication at public resume: a crash between
// the verification write and its delivery or its anchor must strand
// neither — delivery re-runs line-idempotently and reconciliation heals
// the anchor lag — so the terminal-status refusal answers the human
// over consistent records instead of a state-integrity park.
func (e *Engine) healTerminalPublication(statePath string, state map[string]any) error {
	ledgerPath := filepath.Join(e.missionDir(), "ledger.md")
	// Reconciliation ONLY — never a late delivery: if the
	// completion's delivery failed but its anchor published the current
	// hash, appending annotations now would mutate a ledger already
	// anchored at this state — the one shape no heal admits (anchor
	// cadence refuses same-hash re-binding by design). A lost delivery
	// stays lost; the returns remain recoverable in the tree
	// (terminal delivery's stated best-effort boundary). The
	// crash-after-delivery case needs no re-delivery: its annotation
	// suffix is exactly what reconciliation's terminal-delivery-lag
	// shape re-anchors.
	if code, err := mission.Reconcile(statePath, e.Root, ledgerPath); err != nil || code != 0 {
		detail := ""
		if err != nil {
			detail = err.Error()
		}
		return failf(3, "mission terminal reconciliation failed: %s", detail)
	}
	return nil
}

// completePendingVerification finishes the consumed-but-unconcluded
// state at resume: the interval between the acceptance write and its
// verification is DEFINED, and this is its deterministic continuation.
func (e *Engine) completePendingVerification(statePath, ledger string, state map[string]any, turnID string) (map[string]any, bool, error) {
	openTurn, _ := state["openTurn"].(map[string]any)
	cycle, _ := jsonInt(openTurn["cycle"])
	turnDir := filepath.Join(e.missionDir(), "turns", turnID)
	// Detected evidence is STICKY here exactly as at the gate: a
	// post-verification violation whose park crashed re-executes with the
	// recorded violation verbatim — a repository restored to innocence
	// afterwards does not un-happen the detection.
	if prior, perr := readJSONDoc(filepath.Join(turnDir, "wall.json")); perr == nil {
		if recorded, _ := prior["violation"].(string); recorded != "" {
			final, ferr := e.parkWallViolation(statePath, ledger, turnID, turnDir, cycle, recorded, state, true, "")
			return final, true, ferr
		}
	}
	_, values, _, err := e.parseContract(true)
	if err != nil {
		return nil, false, err
	}
	declared, declarationViolation := parseHostArtifacts(values["wall.host-artifacts"])
	if declarationViolation != "" {
		final, perr := e.parkWallViolation(statePath, ledger, turnID, turnDir, cycle, declarationViolation, state, true, "")
		return final, true, perr
	}
	return e.verifyAcceptance(statePath, ledger, turnID, turnDir, cycle, declared)
}

// repairTerminalTurnRecords re-derives the terminal turn-record
// projection from the durable chain: every VERIFIED completed
// acceptance whose turn record is still non-terminal gets its terminal
// patch — the record is a projection of state, never an authority.
func (e *Engine) repairTerminalTurnRecords(state map[string]any) error {
	turnLog, _ := state["turnLog"].([]any)
	for _, item := range turnLog {
		entry, _ := item.(map[string]any)
		if entry == nil {
			continue
		}
		if kind, _ := entry["kind"].(string); kind != mission.WallVerificationKind {
			continue
		}
		turnID, _ := entry["turnId"].(string)
		acceptance := acceptanceEntryFor(state, turnID)
		if acceptance == nil {
			continue
		}
		if outcome, _ := acceptance["outcome"].(string); outcome != "completed" {
			continue
		}
		turnPath := filepath.Join(e.missionDir(), "turns", turnID, "turn.json")
		record, err := readJSONDoc(turnPath)
		if err != nil {
			continue
		}
		if recorded, _ := record["outcome"].(string); recorded == "completed" {
			continue
		}
		if _, err := patchTurn(turnPath, map[string]any{
			"status": "completed", "outcome": "completed", "error": nil,
			"detail": "host return accepted; posture verified", "endedAt": nowISO(),
		}); err != nil {
			return err
		}
	}
	return nil
}

// dropTurnOpenHead removes the runner-owned open-commit anchor — turn
// bookkeeping that retires with the turn. Best effort: a leftover ref is
// unjudged in the quiet period and CAS-overwritten at the next open, so
// a failed delete costs nothing.
func (e *Engine) dropTurnOpenHead() {
	ref := mission.MissionRefNamespace(e.Mission) + "turn-open-head"
	gitCaptured(e.Root, "update-ref", "-d", ref)
}
