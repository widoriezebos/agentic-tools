package validate

// The integration authorization (the host-implementer wall): issued
// ONLY here, at the moment every implementer-return, critic-chain, and
// merge check has passed, proving atomically at issue time that
// apply(exact bound patch, exact bound base) = reviewed tree — with the
// patch digest and changed paths derived from those same bytes, never
// from any party's claim. Non-mission chains issue nothing; the wall
// exists between a mission host and its delegates.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/events"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/wiredoc"
)

// jobIdentityKeys is the named immutable subset of a job record that the
// authorization's jobRecordDigest covers. The full record mutates after
// issuance (mirror, chainClosed, usage), so the digest binds exactly the
// fields dispatch fixes for the record's whole life — the same set
// internal/dispatch refuses to patch.
var jobIdentityKeys = []string{
	"jobId", "role", "runtime", "round", "parentJob", "reviews",
	"workspaceRoot", "baseSha", "branch", "startedAt", "claimEpoch",
	"mainId", "capMin", "capDeadline", "capResolution",
	"mission", "missionIncarnation", "turnId", "stream",
}

// canonicalDigest is sha256 over the value's canonical wiredoc encoding —
// the repository's one canonical encoder, no ad-hoc serialization.
func canonicalDigest(value any) (string, error) {
	canonical, err := wiredoc.RenderValue(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), nil
}

// authorizationsDir is the pinned record location from the wall's named
// contracts.
func authorizationsDir(root, mission string) string {
	return filepath.Join(root, "artifacts", "agents", "missions", mission, "authorizations")
}

// issueAuthorization runs after a merge-stage success. For a non-mission
// chain it does nothing. For a mission chain it derives the exact patch
// between the boundary base tree and the final committed tree, proves the
// patch applies back to exactly the reviewed tree, binds the full
// provenance tuple, and durably writes the content-addressed record and
// patch. Any refusal here fails the merge: an unauthorizable mission
// chain is not accepted.
func (r *conformanceRun) issueAuthorization(finalTree string) error {
	missionName, _ := r.record["mission"].(string)
	if missionName == "" {
		return nil
	}
	incarnation, _ := r.record["missionIncarnation"].(string)
	stream, _ := r.record["stream"].(string)
	dispatchTurn, _ := r.record["turnId"].(string)
	if incarnation == "" || stream == "" || dispatchTurn == "" {
		return fmt.Errorf("mission chain carries incomplete provenance (incarnation/stream/turn); it predates the host-implementer wall and cannot be authorized — dispatch a fresh chain")
	}
	issuanceTurn := os.Getenv("METASYSTEM_MISSION_TURN")
	if issuanceTurn == "" {
		return fmt.Errorf("authorization issuance requires the current mission turn (METASYSTEM_MISSION_TURN); run conformance from inside the mission host turn")
	}

	workspace := r.projectWorkspace()
	baseTree, err := workspace.TreeOf(r.boundaryBase)
	if err != nil {
		return fmt.Errorf("cannot resolve the boundary base tree: %v", err)
	}
	patch, err := workspace.Diff(baseTree, finalTree)
	if err != nil {
		return err
	}
	changedPaths, err := workspace.ChangedPaths(baseTree, finalTree)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(patch)) == 0 || len(changedPaths) == 0 {
		return fmt.Errorf("an empty diff cannot be authorized; the chain shipped no reviewable change")
	}
	// Guardrail custody at issuance: a chain that changes the mission's
	// declared net needs the warden's review before it can be
	// authorized — the wall re-refuses unmarked guardrail touches at
	// consumption regardless, so this is the early, explainable refusal.
	guardrails, err := mission.VerifiedGuardrails(r.root, missionName)
	if err != nil {
		return err
	}
	guardrailTouch := ""
	for _, path := range changedPaths {
		if guardrails.Covers(path) {
			guardrailTouch = path
			break
		}
	}
	if guardrailTouch != "" {
		if failures := r.wardenReviewFailures(finalTree); len(failures) > 0 {
			return fmt.Errorf("authorization refused: this chain changes the guardrail %s and its warden review is not in order (%s); dispatch role warden with --reviews %s, close the chain at zero material findings, and re-run conformance", guardrailTouch, strings.Join(failures, "; "), r.job)
		}
	}
	// The trees this record names stay reachable for consumption-time
	// verification: the runner's staleness check dereferences both long
	// after the delegate's worktree is gone.
	repoWorkspace := gittree.Workspace{Dir: r.root}
	for _, tree := range []string{baseTree, finalTree} {
		if err := repoWorkspace.Anchor(missionName, tree); err != nil {
			return fmt.Errorf("cannot anchor the authorization's %s: %v", tree, err)
		}
	}
	// The atomic issue-time proof: the recorded patch applied to the
	// recorded base IS the reviewed tree, byte for byte.
	applied, err := workspace.Apply(baseTree, patch)
	if err != nil {
		return fmt.Errorf("issue-time apply proof failed: %v", err)
	}
	if applied != finalTree {
		return fmt.Errorf("issue-time apply proof failed: apply(patch, base) = %s, reviewed tree is %s", applied, finalTree)
	}

	identity := map[string]any{}
	for _, key := range jobIdentityKeys {
		identity[key] = r.record[key]
	}
	jobRecordDigest, err := canonicalDigest(identity)
	if err != nil {
		return fmt.Errorf("cannot digest the job record identity: %v", err)
	}
	baseSequence, baseSegment, err := missionBaseSequencePoint(r.root, missionName, baseTree)
	if err != nil {
		return err
	}
	patchSum := sha256.Sum256(patch)

	dir := authorizationsDir(r.root, missionName)
	supersedes, err := priorChainAuthorizations(dir, r.rootJob)
	if err != nil {
		return err
	}

	record := map[string]any{
		"schemaVersion":      1,
		"jobId":              r.job,
		"rootJob":            r.rootJob,
		"jobRecordDigest":    jobRecordDigest,
		"mission":            missionName,
		"missionIncarnation": incarnation,
		"stream":             stream,
		"dispatchTurn":       dispatchTurn,
		"issuanceTurn":       issuanceTurn,
		"baseTree":           baseTree,
		// The sequence point names WHICH occurrence of the base tree this
		// authorization was issued against (tree ids repeat): the E-point
		// whose tree IS the boundary base, found in the mission's
		// acceptance entries or the open turn — never the raw chain
		// sequence.
		"baseSequencePoint": map[string]any{"sequence": baseSequence, "segment": baseSegment},
		"reviewedTree":      finalTree,
		"patchDigest":       hex.EncodeToString(patchSum[:]),
		"changedPaths":      changedPaths,
		"supersedes":        supersedes,
	}
	// The lane fact rides the digest-covered record: consumers refuse a
	// guardrail touch without it, and it cannot be added after issuance.
	if guardrailTouch != "" {
		record["guardrailLane"] = true
	}
	// Digest-then-embed: the digest covers the record WITHOUT the
	// authorizationDigest field; the filename carries the same digest.
	digest, err := canonicalDigest(record)
	if err != nil {
		return fmt.Errorf("cannot digest the authorization record: %v", err)
	}
	record["authorizationDigest"] = digest

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if _, err := atomicfile.WriteText(filepath.Join(dir, digest+".patch"), string(patch), r.root); err != nil {
		return fmt.Errorf("cannot write the authorization patch: %v", err)
	}
	encoded, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	durable, err := atomicfile.WriteText(filepath.Join(dir, digest+".json"), string(encoded)+"\n", r.root)
	if err != nil {
		return fmt.Errorf("cannot write the authorization record: %v", err)
	}
	if !durable {
		// Committed but durability unknown: re-read before proceeding — the
		// authorization only exists once its bytes verifiably exist.
		reread, rerr := os.ReadFile(filepath.Join(dir, digest+".json"))
		if rerr != nil || !bytes.Equal(reread, append(encoded, '\n')) {
			return fmt.Errorf("authorization record durability is unverified; re-run conformance")
		}
	}
	events.EmitOnce(r.root, "conformance", "authorization-issued",
		"integration authorization issued for "+r.job,
		int64(os.Getpid()), 0, 1, map[string]string{
			"authorizationDigest": digest, "jobId": r.job, "missionId": missionName,
		})
	r.out = append(r.out, "integrationAuthorization="+digest)
	return nil
}

// wardenReviewFailures judges whether a COMPLETED warden review admits
// this chain's guardrail change: a top-level warden chain must review a
// job of this chain, be closed, have a completed final round whose
// return parses, report zero material findings with a consistent
// verdict count, and have reviewed EXACTLY the tree being authorized —
// a warden that merely exists, or reviewed different bytes, admits
// nothing. Empty means admitted. The warden chain answers to the same
// critique-exhaustion discipline the code-critic requirement enforces:
// a warden can no more be exhausted around than a critic can.
func (r *conformanceRun) wardenReviewFailures(finalTree string) []string {
	records := r.loadConformanceRecords()
	implementationIDs := map[string]bool{}
	for jobID := range records {
		if root, ok := chainRootIn(records, jobID); ok && root == r.rootJob {
			implementationIDs[jobID] = true
		}
	}
	var wardenIDs []string
	for jobID, record := range records {
		if record["role"] != "warden" || record["parentJob"] != nil {
			continue
		}
		if reviews, ok := record["reviews"].(string); ok && implementationIDs[reviews] {
			wardenIDs = append(wardenIDs, jobID)
		}
	}
	sort.Strings(wardenIDs)
	if len(wardenIDs) == 0 {
		return []string{"no warden chain reviews this implementation"}
	}
	var allFailures []string
	for _, wardenID := range wardenIDs {
		var failures []string
		wardenRoot := records[wardenID]
		var final map[string]any
		finalRound := -1.0
		for jobID, record := range records {
			if root, ok := chainRootIn(records, jobID); !ok || root != wardenID {
				continue
			}
			if round, ok := record["round"].(float64); ok && round > finalRound {
				finalRound, final = round, record
			}
		}
		if final == nil {
			failures = append(failures, fmt.Sprintf("warden chain %s has no readable rounds", wardenID))
			if wardenRoot["chainClosed"] != true {
				failures = append(failures, fmt.Sprintf("warden chain %s is not closed", wardenID))
			}
			allFailures = append(allFailures, failures...)
			continue
		}
		if wardenRoot["chainClosed"] != true {
			failures = append(failures, fmt.Sprintf("warden chain %s is not closed", wardenID))
		}
		if final["status"] != "completed" {
			failures = append(failures, fmt.Sprintf("warden chain %s final round is not completed", wardenID))
		}
		returnPath := filepath.Join(r.root, "artifacts", "agents", wardenID,
			"rounds", scalarText(final["round"]), "return.json")
		result := map[string]any{}
		returnData, err := os.ReadFile(returnPath)
		var parsedReturn any
		if err == nil {
			err = json.Unmarshal(returnData, &parsedReturn)
		}
		if err != nil {
			failures = append(failures, fmt.Sprintf("warden chain %s final return is unreadable", wardenID))
		} else if object, ok := parsedReturn.(map[string]any); ok {
			result = object
		} else {
			failures = append(failures, fmt.Sprintf("warden chain %s final return is not a JSON object", wardenID))
		}
		var materialIDs []string
		if findings, ok := result["findings"].([]any); ok {
			for _, item := range findings {
				finding, ok := item.(map[string]any)
				if !ok || finding["material"] != true {
					continue
				}
				if id, ok := finding["id"].(string); ok {
					materialIDs = append(materialIDs, id)
				} else {
					materialIDs = append(materialIDs, "<unnamed>")
				}
			}
		} else if len(result) > 0 {
			failures = append(failures, fmt.Sprintf("warden chain %s final return has no findings array", wardenID))
		}
		verdictZero := result["verdictMaterialCount"] == float64(0)
		if len(materialIDs) > 0 || !verdictZero {
			failures = append(failures, fmt.Sprintf("warden chain %s still reports material findings", wardenID))
		}
		successorText := func(successorJob string) (string, bool) {
			record, present := records[successorJob]
			if !present {
				return "", false
			}
			round, ok := record["round"].(float64)
			if !ok || round != float64(int64(round)) {
				return "", false
			}
			prompt := filepath.Join(r.root, "artifacts", "agents", r.rootJob,
				"rounds", strconv.FormatInt(int64(round), 10), "prompt.md")
			data, err := os.ReadFile(prompt)
			if err != nil {
				return "", false
			}
			return string(data), true
		}
		failures = append(failures, exhaustionDiscipline(wardenRoot, records, r.rootJob, successorText, final["round"], materialIDs, verdictZero)...)
		if reviewed, _ := result["reviewedTree"].(string); reviewed != finalTree {
			failures = append(failures, fmt.Sprintf("warden chain %s reviewed tree %s, not the tree being authorized", wardenID, scalarText(result["reviewedTree"])))
		}
		if len(failures) == 0 {
			return nil
		}
		allFailures = append(allFailures, failures...)
	}
	return allFailures
}

// missionBaseSequencePoint binds the issuance point: the E-sequence point
// whose tree IS the boundary base — the open turn's pre-tree under the
// current sequence point, or an earlier acceptance's post-tree under the
// occurrence it was accepted at. A base that is no named point refuses
// issuance outright: consumption could never verify it.
func missionBaseSequencePoint(root, missionID, baseTree string) (int64, int64, error) {
	statePath := filepath.Join(root, "artifacts", "agents", "missions", missionID, "state.json")
	data, err := os.ReadFile(statePath)
	if err != nil {
		return 0, 0, fmt.Errorf("authorization issuance cannot read the mission state: %v", err)
	}
	var state map[string]any
	if err := json.Unmarshal(data, &state); err != nil {
		return 0, 0, fmt.Errorf("authorization issuance cannot parse the mission state: %v", err)
	}
	// Matching the CURRENT point first is sound because the comparison is
	// FULL-TREE equality: if the boundary base byte-equals the current
	// expected tree, every file the review saw is identical to the tree
	// the patch will land on — relabeling an older occurrence to the
	// current one changes nothing the staleness predicate could measure.
	if openTurn, ok := state["openTurn"].(map[string]any); ok {
		if tree, _ := openTurn["preTree"].(string); tree == baseTree {
			sequence, segment := mission.CurrentSequencePoint(state)
			return sequence, segment, nil
		}
	}
	for _, point := range mission.ExpectedTreePoints(state) {
		if point.Tree == baseTree {
			return point.Sequence, point.Segment, nil
		}
	}
	// The refusal stays GENERIC on purpose: a baseline differing from
	// committed HEAD does not prove a sealed-dirty admission — the same
	// state arises lawfully mid-turn after a delegate merge advances
	// HEAD before the turn concludes, and there the remedy is to
	// conclude the turn, not re-provision. A sharper diagnosis needs
	// authenticated admission provenance the state does not yet carry.
	return 0, 0, fmt.Errorf("authorization refused: base tree %s is not a named expected-tree sequence point of mission %s; re-dispatch from the current expected tree", baseTree, missionID)
}

// priorChainAuthorizations lists the digests of every authorization already
// issued for the same chain — the new record supersedes them all, and
// eligibility is DERIVED (an authorization is superseded iff a later valid
// one names it), so a content-addressed record never mutates.
func priorChainAuthorizations(dir, rootJob string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	var digests []string
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		var prior struct {
			RootJob             string `json:"rootJob"`
			AuthorizationDigest string `json:"authorizationDigest"`
		}
		if json.Unmarshal(data, &prior) != nil {
			continue
		}
		if prior.RootJob == rootJob && prior.AuthorizationDigest == strings.TrimSuffix(name, ".json") {
			digests = append(digests, prior.AuthorizationDigest)
		}
	}
	sort.Strings(digests)
	if digests == nil {
		digests = []string{}
	}
	return digests, nil
}

// AuthorizationRecordDigest recomputes the canonical digest of an
// authorization record — the exact bytes issuance signed, with the
// embedded authorizationDigest omitted. Every consumer must call this
// before trusting ANY field: the filename match alone authenticates
// nothing.
func AuthorizationRecordDigest(record map[string]any) (string, error) {
	clone := make(map[string]any, len(record))
	for key, value := range record {
		if key == "authorizationDigest" {
			continue
		}
		clone[key] = value
	}
	return canonicalDigest(clone)
}

// JobIdentityDigest is the canonical digest of a job record's immutable
// identity subset — the value an integration authorization binds as
// jobRecordDigest, exported so the runner's adjudication can verify a
// certification against the SAME set conformance digested at issuance.
func JobIdentityDigest(record map[string]any) (string, error) {
	identity := map[string]any{}
	for _, key := range jobIdentityKeys {
		identity[key] = record[key]
	}
	return canonicalDigest(identity)
}
