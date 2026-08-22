package mission

// Snapshot-scope state shapes: the recorded origins and postures that
// make HEAD, the ref map, the staged projections, and the worktree
// census accountable across turn and crash boundaries. Every shape
// validates by EXACT key set — new authoritative fields are a schema
// decision, never a silent addition.

import (
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

// MissionRefNamespace is the runner's self-owned publication namespace
// for one mission. Every recorded ref map omits it — the state-anchors
// ref's tip after any state write contains the very state hash the
// record would freeze, a content-addressed self-reference — and the
// fence judges the namespace structurally at every capture instead.
func MissionRefNamespace(missionID string) string {
	return "refs/metasystem/missions/" + missionID + "/"
}

// RecordableRefMap renders a live ref map for a state record, the
// mission's own publication namespace omitted.
func RecordableRefMap(refs map[string]string, missionID string) map[string]any {
	prefix := MissionRefNamespace(missionID)
	doc := map[string]any{}
	for name, oid := range refs {
		if strings.HasPrefix(name, prefix) {
			continue
		}
		doc[name] = oid
	}
	return doc
}

// StagedPostureDoc renders a staged posture for a state record.
func StagedPostureDoc(posture gittree.StagedPosture) map[string]any {
	unmerged := make([]any, 0, len(posture.Unmerged))
	for _, entry := range posture.Unmerged {
		unmerged = append(unmerged, entry)
	}
	return map[string]any{"tree": posture.Tree, "unmerged": unmerged}
}

// WorktreeCensusDoc renders a worktree census for a state record.
func WorktreeCensusDoc(census []gittree.WorktreeRecord) []any {
	doc := make([]any, 0, len(census))
	for _, record := range census {
		pseudorefs := make([]any, 0, len(record.Pseudorefs))
		for _, ref := range record.Pseudorefs {
			oids := make([]any, 0, len(ref.OIDs))
			for _, oid := range ref.OIDs {
				oids = append(oids, oid)
			}
			pseudorefs = append(pseudorefs, map[string]any{
				"name": ref.Name, "oids": oids, "parseable": ref.Parseable,
			})
		}
		var staged any
		if record.PostureReadable {
			staged = StagedPostureDoc(record.Staged)
		}
		doc = append(doc, map[string]any{
			"path": record.Path, "headOid": record.HeadOID, "branch": record.Branch,
			"detached": record.Detached, "bare": record.Bare, "prunable": record.Prunable,
			"postureReadable": record.PostureReadable, "pseudorefs": pseudorefs, "staged": staged,
		})
	}
	return doc
}

// CaptureAdmissionOrigins captures the accounting origins the birth
// record carries beside E0: the initial headCommit, the toplevel tree
// and staged posture (nested checkouts only), the recordable ref map,
// and the worktree census with postures. The recorded toplevel trees are
// anchored in the mission's namespace so garbage collection cannot eat
// the origins a later inspection diffs against.
func CaptureAdmissionOrigins(root, missionID string) (map[string]any, error) {
	workspace := gittree.Workspace{Dir: root}
	head, unborn, err := workspace.HeadCommit()
	if err != nil {
		return nil, err
	}
	if unborn {
		return nil, stateErr("mission admission requires a committed HEAD")
	}
	refs, err := workspace.RefMap()
	if err != nil {
		return nil, err
	}
	census, err := workspace.WorktreeCensus()
	if err != nil {
		return nil, err
	}
	prefix, err := workspace.Prefix()
	if err != nil {
		return nil, err
	}
	origins := map[string]any{
		"headCommit":     head,
		"topTree":        nil,
		"topStaged":      nil,
		"refMap":         RecordableRefMap(refs, missionID),
		"worktreeCensus": WorktreeCensusDoc(census),
		"capturedAt":     time.Now().UTC().Format("2006-01-02T15:04:05Z"),
	}
	if prefix != "" {
		top, err := workspace.TopLevel()
		if err != nil {
			return nil, err
		}
		topWorkspace := gittree.Workspace{Dir: top}
		topTree, err := topWorkspace.Snapshot(head)
		if err != nil {
			return nil, err
		}
		topStaged, err := workspace.TopStagedPosture()
		if err != nil {
			return nil, err
		}
		for _, tree := range []string{topTree, topStaged.Tree} {
			if err := workspace.Anchor(missionID, tree); err != nil {
				return nil, err
			}
		}
		origins["topTree"] = topTree
		origins["topStaged"] = StagedPostureDoc(topStaged)
	}
	return origins, nil
}

// VerifyBaselineIsLive checks that a supplied E0 IS the workspace's
// live filtered projection: the identity-space tree the wall preflight
// would record for a clean or human-sealed admission at this instant.
func VerifyBaselineIsLive(root, missionID, baseline string) error {
	workspace := gittree.Workspace{Dir: root}
	raw, err := workspace.Snapshot("HEAD")
	if err != nil {
		return stateErr("cannot verify the supplied baseline: %v", err)
	}
	record, err := workspace.FilterTree(raw, []string{"artifacts/agents/missions/" + missionID + "/ledger.md"})
	if err != nil {
		return stateErr("cannot verify the supplied baseline: %v", err)
	}
	if record != baseline {
		return stateErr("supplied baseline %s is not the live filtered projection %s; every mission is born from what the repository holds", baseline, record)
	}
	return nil
}

// ValidateStagedPosture checks the logical staged posture shape: the
// stage-0 entries as a tree id and every unmerged entry serialized
// beside it. nil is lawful where a scope does not apply (a toplevel
// install has no separate toplevel index).
func ValidateStagedPosture(raw any) error {
	if raw == nil {
		return nil
	}
	posture, ok := raw.(map[string]any)
	if !ok || !exactKeys(posture, "tree", "unmerged") {
		return stateErr("staged posture has an invalid shape")
	}
	if tree, _ := posture["tree"].(string); !treeIDRe.MatchString(tree) {
		return stateErr("staged posture tree is invalid")
	}
	unmerged, ok := posture["unmerged"].([]any)
	if !ok {
		return stateErr("staged posture unmerged entries must be an array")
	}
	for _, item := range unmerged {
		if entry, ok := item.(string); !ok || entry == "" {
			return stateErr("staged posture unmerged entries must be non-empty strings")
		}
	}
	return nil
}

// ValidateRefMap checks a recorded ref map: ref names to object ids.
func ValidateRefMap(raw any) error {
	refs, ok := raw.(map[string]any)
	if !ok {
		return stateErr("recorded ref map must be an object")
	}
	for name, value := range refs {
		if name == "" {
			return stateErr("recorded ref map carries an empty ref name")
		}
		if oid, _ := value.(string); !treeIDRe.MatchString(oid) {
			return stateErr("recorded ref map value for %s is invalid", name)
		}
	}
	return nil
}

// ValidateWorktreeCensus checks the recorded worktree census: each
// admitted worktree with its posture — HEAD, the private pseudoref
// census, and the logical staged posture.
func ValidateWorktreeCensus(raw any) error {
	census, ok := raw.([]any)
	if !ok {
		return stateErr("worktree census must be an array")
	}
	for _, item := range census {
		record, ok := item.(map[string]any)
		if !ok || !exactKeys(record, "path", "headOid", "branch", "detached", "bare", "prunable", "postureReadable", "pseudorefs", "staged") {
			return stateErr("worktree census record has an invalid shape")
		}
		if path, _ := record["path"].(string); path == "" {
			return stateErr("worktree census record path is invalid")
		}
		if head, _ := record["headOid"].(string); head != "" && !treeIDRe.MatchString(head) {
			return stateErr("worktree census record HEAD is invalid")
		}
		if _, ok := record["branch"].(string); !ok {
			return stateErr("worktree census record branch is invalid")
		}
		for _, field := range []string{"detached", "bare", "prunable", "postureReadable"} {
			if _, ok := record[field].(bool); !ok {
				return stateErr("worktree census record %s is invalid", field)
			}
		}
		pseudorefs, ok := record["pseudorefs"].([]any)
		if !ok {
			return stateErr("worktree census pseudorefs must be an array")
		}
		for _, refItem := range pseudorefs {
			ref, ok := refItem.(map[string]any)
			if !ok || !exactKeys(ref, "name", "oids", "parseable") {
				return stateErr("worktree census pseudoref has an invalid shape")
			}
			if name, _ := ref["name"].(string); name == "" {
				return stateErr("worktree census pseudoref name is invalid")
			}
			if _, ok := ref["parseable"].(bool); !ok {
				return stateErr("worktree census pseudoref parseable flag is invalid")
			}
			oids, ok := ref["oids"].([]any)
			if !ok {
				return stateErr("worktree census pseudoref oids must be an array")
			}
			for _, oidItem := range oids {
				if oid, _ := oidItem.(string); !treeIDRe.MatchString(oid) {
					return stateErr("worktree census pseudoref oid is invalid")
				}
			}
		}
		if err := ValidateStagedPosture(record["staged"]); err != nil {
			return err
		}
	}
	return nil
}

// validateAdmissionOrigins checks the birth record's accounting origins:
// the observable posture admission captured beside E0, so turn-1
// continuity and a crash between admission and first open have a durable
// authority, not a silent adoption.
func validateAdmissionOrigins(raw any) error {
	origins, ok := raw.(map[string]any)
	if !ok || !exactKeys(origins, "headCommit", "topTree", "topStaged", "refMap", "worktreeCensus", "capturedAt") {
		return stateErr("mission admissionOrigins has an invalid shape")
	}
	if oid, _ := origins["headCommit"].(string); !treeIDRe.MatchString(oid) {
		return stateErr("mission admissionOrigins headCommit is invalid")
	}
	if origins["topTree"] != nil {
		if tree, _ := origins["topTree"].(string); !treeIDRe.MatchString(tree) {
			return stateErr("mission admissionOrigins topTree is invalid")
		}
	}
	if err := ValidateStagedPosture(origins["topStaged"]); err != nil {
		return err
	}
	if err := ValidateRefMap(origins["refMap"]); err != nil {
		return err
	}
	if err := ValidateWorktreeCensus(origins["worktreeCensus"]); err != nil {
		return err
	}
	if s, _ := origins["capturedAt"].(string); parseISO(s) != nil {
		return stateErr("mission admissionOrigins capturedAt is invalid")
	}
	return nil
}

// ValidateRecordedPosture checks one recorded carrier posture — the
// block acceptance payloads and resolution entries carry as the next
// accounting origin.
func ValidateRecordedPosture(posture map[string]any, label string) error {
	if posture == nil {
		return stateErr("%s carries no recorded posture", label)
	}
	if !exactKeys(posture, "headCommitPost", "refMapPost", "stagedTreePost", "topTreePost", "topStagedPost", "worktreeCensusPost", "capturedAt") {
		return stateErr("%s posture has an invalid shape", label)
	}
	if oid, _ := posture["headCommitPost"].(string); !treeIDRe.MatchString(oid) {
		return stateErr("%s headCommitPost is invalid", label)
	}
	if tree, _ := posture["stagedTreePost"].(string); !treeIDRe.MatchString(tree) {
		return stateErr("%s stagedTreePost is invalid", label)
	}
	if posture["topTreePost"] != nil {
		if tree, _ := posture["topTreePost"].(string); !treeIDRe.MatchString(tree) {
			return stateErr("%s topTreePost is invalid", label)
		}
	}
	if err := ValidateRefMap(posture["refMapPost"]); err != nil {
		return stateErr("%s refMapPost is invalid: %v", label, err)
	}
	if err := ValidateStagedPosture(posture["topStagedPost"]); err != nil {
		return stateErr("%s topStagedPost is invalid: %v", label, err)
	}
	if err := ValidateWorktreeCensus(posture["worktreeCensusPost"]); err != nil {
		return stateErr("%s worktreeCensusPost is invalid: %v", label, err)
	}
	if s, _ := posture["capturedAt"].(string); parseISO(s) != nil {
		return stateErr("%s capturedAt is invalid", label)
	}
	return nil
}

// WallVerificationKind marks the post-verification turn-log entry: the
// acceptance append stays the single commit point, but the turn
// concludes only when a fresh capture matches the recorded posture — an
// acceptance entry with no verification entry is the DEFINED
// consumed-but-unconcluded state that resume completes
// deterministically.
const WallVerificationKind = "wall-verification"

// isVerificationEntry reports whether a turn-log entry is a
// post-verification record rather than an acceptance.
func isVerificationEntry(entry map[string]any) bool {
	kind, _ := entry["kind"].(string)
	return kind == WallVerificationKind
}

// validateVerificationEntry checks the post-verification entry shape.
func validateVerificationEntry(entry map[string]any) error {
	if !exactKeys(entry, "turnId", "kind", "capturedAt", "verdict") {
		return stateErr("mission wall-verification entry has an invalid shape")
	}
	if turn, _ := entry["turnId"].(string); !idRe.MatchString(turn) {
		return stateErr("mission wall-verification entry turn id is invalid")
	}
	if s, _ := entry["capturedAt"].(string); parseISO(s) != nil {
		return stateErr("mission wall-verification entry capturedAt is invalid")
	}
	if v, _ := entry["verdict"].(string); v != "clean" {
		return stateErr("mission wall-verification entry verdict is invalid")
	}
	return nil
}

// validateVerificationAppend checks one appended post-verification entry
// against the write's own transition: it concludes the open turn, whose
// acceptance landed and is not yet verified, and the marker dies in the
// same write.
func validateVerificationAppend(previous, next map[string]any, entry map[string]any) error {
	if err := validateVerificationEntry(entry); err != nil {
		return err
	}
	turnID, _ := entry["turnId"].(string)
	openTurn, _ := previous["openTurn"].(map[string]any)
	if openTurn == nil {
		return stateErr("mission wall-verification entry has no open turn to conclude")
	}
	if open, _ := openTurn["turnId"].(string); open != turnID {
		return stateErr("mission wall-verification entry does not name the open turn")
	}
	if next["openTurn"] != nil {
		return stateErr("mission wall-verification entry must conclude the open turn in its own write")
	}
	// The acceptance must have COMMITTED in a prior write: a single
	// write carrying both the acceptance and its "verification" would
	// collapse the two phases the interval exists to separate.
	prevLog, _ := previous["turnLog"].([]any)
	accepted := false
	for _, item := range prevLog {
		logged, _ := item.(map[string]any)
		if logged == nil || isVerificationEntry(logged) {
			continue
		}
		if id, _ := logged["turnId"].(string); id == turnID && logged["wall"] != nil {
			accepted = true
		}
	}
	if !accepted {
		return stateErr("mission wall-verification entry has no committed acceptance to conclude")
	}
	verifications := 0
	nextLog, _ := next["turnLog"].([]any)
	for _, item := range nextLog {
		logged, _ := item.(map[string]any)
		if logged == nil || !isVerificationEntry(logged) {
			continue
		}
		if id, _ := logged["turnId"].(string); id == turnID {
			verifications++
		}
	}
	if verifications != 1 {
		return stateErr("mission wall-verification entry repeats a conclusion")
	}
	return nil
}

// UnverifiedAcceptance names the turn id of the newest acceptance entry
// that has no post-verification entry — the consumed-but-unconcluded
// state a crash between the two writes leaves. Empty when every
// acceptance is concluded.
func UnverifiedAcceptance(state map[string]any) string {
	turnLog, _ := state["turnLog"].([]any)
	// A verification concludes only an acceptance that PRECEDES it:
	// ordering is part of the binding, not just the turn id.
	verifiedAfter := map[string]int{}
	for index, item := range turnLog {
		entry, _ := item.(map[string]any)
		if entry == nil || !isVerificationEntry(entry) {
			continue
		}
		turn, _ := entry["turnId"].(string)
		verifiedAfter[turn] = index
	}
	for i := len(turnLog) - 1; i >= 0; i-- {
		entry, _ := turnLog[i].(map[string]any)
		if entry == nil || isVerificationEntry(entry) {
			continue
		}
		if entry["wall"] == nil {
			continue
		}
		turn, _ := entry["turnId"].(string)
		if index, ok := verifiedAfter[turn]; !ok || index < i {
			return turn
		}
		return ""
	}
	return ""
}
