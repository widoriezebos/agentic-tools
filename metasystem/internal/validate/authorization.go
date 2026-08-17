package validate

// The integration authorization (host-implementer wall, HIW-O2): issued
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
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/events"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
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
	mission, _ := r.record["mission"].(string)
	if mission == "" {
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

	baseTree, err := r.git(r.workspace, nil, "rev-parse", r.boundaryBase+"^{tree}")
	if err != nil {
		return fmt.Errorf("cannot resolve the boundary base tree: %v", err)
	}
	workspace := gittree.Workspace{Dir: r.workspace}
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
	sequence, err := missionStateSequence(r.root, mission)
	if err != nil {
		return err
	}
	patchSum := sha256.Sum256(patch)

	dir := authorizationsDir(r.root, mission)
	supersedes, err := priorChainAuthorizations(dir, r.rootJob)
	if err != nil {
		return err
	}

	record := map[string]any{
		"schemaVersion":      1,
		"jobId":              r.job,
		"rootJob":            r.rootJob,
		"jobRecordDigest":    jobRecordDigest,
		"mission":            mission,
		"missionIncarnation": incarnation,
		"stream":             stream,
		"dispatchTurn":       dispatchTurn,
		"issuanceTurn":       issuanceTurn,
		"baseTree":           baseTree,
		// The sequence point names WHICH occurrence of the base tree this
		// authorization was issued against (r5 HIW-R5-01: tree ids repeat).
		// segment is 0 until mission state v2 introduces taint-resolution
		// segments; consumption treats {sequence, segment} as one identity.
		"baseSequencePoint": map[string]any{"sequence": sequence, "segment": 0},
		"reviewedTree":      finalTree,
		"patchDigest":       hex.EncodeToString(patchSum[:]),
		"changedPaths":      changedPaths,
		"supersedes":        supersedes,
	}
	// Digest-then-embed (r5): the digest covers the record WITHOUT the
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
			"authorizationDigest": digest, "jobId": r.job, "missionId": mission,
		})
	r.out = append(r.out, "integrationAuthorization="+digest)
	return nil
}

// missionStateSequence reads the mission state chain's current sequence
// number — a plain read that BINDS the issuance point; verifying the chain
// stays mission-owned at consumption.
func missionStateSequence(root, mission string) (int64, error) {
	statePath := filepath.Join(root, "artifacts", "agents", "missions", mission, "state.json")
	data, err := os.ReadFile(statePath)
	if err != nil {
		return 0, fmt.Errorf("authorization issuance cannot read the mission state: %v", err)
	}
	var state struct {
		Integrity struct {
			Sequence *int64 `json:"sequence"`
		} `json:"integrity"`
	}
	if err := json.Unmarshal(data, &state); err != nil || state.Integrity.Sequence == nil {
		return 0, fmt.Errorf("authorization issuance cannot read the mission state sequence")
	}
	return *state.Integrity.Sequence, nil
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
