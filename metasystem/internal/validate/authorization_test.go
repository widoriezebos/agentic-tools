package validate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

// missionize stamps the fixture's implementer record with the complete
// wall provenance tuple and gives the mission a readable state chain.
func (f *conformanceFixture) missionize(mission string) {
	path := filepath.Join(f.controller, "artifacts", "agents", "jobs", "impl.json")
	data, err := os.ReadFile(path)
	if err != nil {
		f.t.Fatal(err)
	}
	var record map[string]any
	json.Unmarshal(data, &record)
	record["mission"] = mission
	record["missionIncarnation"] = strings.Repeat("a", 64)
	record["stream"] = "main"
	record["turnId"] = mission + "-t1"
	f.writeJSON("artifacts/agents/jobs/impl.json", record)
	f.writeJSON("artifacts/agents/missions/"+mission+"/state.json", map[string]any{
		"integrity": map[string]any{"sequence": 4},
	})
}

func (f *conformanceFixture) authorization(mission, digest string) map[string]any {
	f.t.Helper()
	path := filepath.Join(authorizationsDir(f.controller, mission), digest+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		f.t.Fatalf("authorization record unreadable: %v", err)
	}
	var record map[string]any
	if err := json.Unmarshal(data, &record); err != nil {
		f.t.Fatalf("authorization record unparseable: %v", err)
	}
	return record
}

func issuedDigest(t *testing.T, out []string) string {
	t.Helper()
	for _, line := range out {
		if strings.HasPrefix(line, "integrationAuthorization=") {
			return strings.TrimPrefix(line, "integrationAuthorization=")
		}
	}
	t.Fatalf("no integrationAuthorization line in %v", out)
	return ""
}

// The HIW-O2 issuance join: a mission chain passing every merge check gets
// a content-addressed authorization whose patch provably applies back to
// exactly the reviewed tree; the digest domain omits the embedded digest;
// re-issuance supersedes; and every refusal fails the merge itself.
func TestMissionAuthorizationIssuance(t *testing.T) {
	f := newConformanceFixture(t)
	appendFile(t, filepath.Join(f.worktree, "source.txt"), "changed\n")
	f.writeImplementer("", "source.txt")
	f.missionize("m-fix")

	expectConformance(t, f, "review", 0, "")
	reviewedTree := f.reviewedTree()
	f.commitWorktree()
	f.writeFollowUp()
	f.writeCritic(reviewedTree, "", "", "critic-model")

	// Outside a runner turn, an accepted merge still refuses issuance —
	// and therefore the merge.
	os.Unsetenv("METASYSTEM_MISSION_TURN")
	expectConformance(t, f, "merge", 1, "requires the current mission turn")

	t.Setenv("METASYSTEM_MISSION_TURN", "m-fix-t2")
	out, _ := expectConformance(t, f, "merge", 0, "integrationAuthorization=")
	digest := issuedDigest(t, out)

	record := f.authorization("m-fix", digest)
	if record["dispatchTurn"] != "m-fix-t1" || record["issuanceTurn"] != "m-fix-t2" ||
		record["mission"] != "m-fix" || record["stream"] != "main" ||
		record["missionIncarnation"] != strings.Repeat("a", 64) {
		t.Fatalf("provenance tuple wrong: %v", record)
	}
	point, _ := record["baseSequencePoint"].(map[string]any)
	if point["sequence"] != float64(4) || point["segment"] != float64(0) {
		t.Fatalf("sequence point wrong: %v", point)
	}
	if supersedes, _ := record["supersedes"].([]any); len(supersedes) != 0 {
		t.Fatalf("first authorization supersedes %v", supersedes)
	}

	// Digest domain: sha256 over the canonical encoding of the record
	// WITHOUT the embedded digest equals the embedded digest and filename.
	stripped := map[string]any{}
	for k, v := range record {
		if k != "authorizationDigest" {
			stripped[k] = v
		}
	}
	recomputed, err := canonicalDigest(stripped)
	if err != nil {
		t.Fatal(err)
	}
	if recomputed != digest || record["authorizationDigest"] != digest {
		t.Fatalf("digest domain broken: recomputed %s, embedded %v, filename %s",
			recomputed, record["authorizationDigest"], digest)
	}

	// The stored patch applies to the stored base and yields exactly the
	// reviewed tree.
	patch, err := os.ReadFile(filepath.Join(authorizationsDir(f.controller, "m-fix"), digest+".patch"))
	if err != nil {
		t.Fatal(err)
	}
	workspace := gittree.Workspace{Dir: f.worktree}
	applied, err := workspace.Apply(record["baseTree"].(string), patch)
	if err != nil {
		t.Fatalf("stored patch refused: %v", err)
	}
	if applied != record["reviewedTree"].(string) || applied != reviewedTree {
		t.Fatalf("stored patch applies to %s, want %s", applied, reviewedTree)
	}

	// Re-issuance supersedes: a second accepted merge mints a successor
	// naming its predecessor; eligibility is derived, nothing mutates.
	out, _ = expectConformance(t, f, "merge", 0, "integrationAuthorization=")
	second := issuedDigest(t, out)
	if second == digest {
		t.Fatal("re-issuance did not mint a successor")
	}
	successor := f.authorization("m-fix", second)
	supersedes, _ := successor["supersedes"].([]any)
	if len(supersedes) != 1 || supersedes[0] != digest {
		t.Fatalf("successor supersedes %v, want [%s]", supersedes, digest)
	}

	// A mission whose state chain is unreadable cannot bind a sequence
	// point, so the merge refuses.
	os.Remove(filepath.Join(f.controller, "artifacts", "agents", "missions", "m-fix", "state.json"))
	expectConformance(t, f, "merge", 1, "cannot read the mission state")
	f.writeJSON("artifacts/agents/missions/m-fix/state.json", map[string]any{
		"integrity": map[string]any{"sequence": 4},
	})

	// Incomplete provenance (a pre-wall mission chain) refuses by name.
	path := filepath.Join(f.controller, "artifacts", "agents", "jobs", "impl.json")
	data, _ := os.ReadFile(path)
	var partial map[string]any
	json.Unmarshal(data, &partial)
	delete(partial, "stream")
	f.writeJSON("artifacts/agents/jobs/impl.json", partial)
	expectConformance(t, f, "merge", 1, "predates the host-implementer wall")
}

// The wall's issuance preconditions on the FULL merge path (slice-3
// critique F-3): a mission waiver refuses outright, a mission chain with
// no critic record accepts nothing and issues nothing, and a tampered
// review-stage diff artifact changes nothing — issuance derives its patch
// from the trees, never from an artifact.
func TestMissionMergeIssuancePreconditions(t *testing.T) {
	f := newConformanceFixture(t)
	appendFile(t, filepath.Join(f.worktree, "docs", "note.md"), "small edit\n")
	f.writeImplementer("prose-under-30", "docs/note.md")
	f.missionize("m-gate")
	t.Setenv("METASYSTEM_MISSION_TURN", "m-gate-t2")

	expectConformance(t, f, "review", 0, "")
	f.commitWorktree()

	// A mission waiver refuses by name, and no authorization exists after.
	expectConformance(t, f, "merge", 1, "a mission chain cannot waive critique")
	if _, err := os.Stat(authorizationsDir(f.controller, "m-gate")); !os.IsNotExist(err) {
		t.Fatal("refused mission waiver left an authorization")
	}

	// The same chain without the waiver and without any critic record
	// refuses at critic closure — still no authorization.
	record, ok := readJobRecord(filepath.Join(f.controller, "artifacts", "agents", "jobs"), "impl")
	if !ok {
		t.Fatal("cannot read the fixture job record")
	}
	delete(record, "critiqueWaived")
	f.writeJSON("artifacts/agents/jobs/impl.json", record)
	expectConformance(t, f, "merge", 1, "merge requires a code-critic chain")
	if _, err := os.Stat(authorizationsDir(f.controller, "m-gate")); !os.IsNotExist(err) {
		t.Fatal("critic-less mission merge left an authorization")
	}

	// With critic closure the merge issues — even though the review-stage
	// diff artifact has been tampered with: the authorization's patch is
	// derived from the trees at issue time, not read from any artifact.
	reviewedTree := f.reviewedTree()
	tampered := filepath.Join(f.controller, "artifacts", "agents", "impl", "rounds", "1", "diff.patch")
	if err := os.WriteFile(tampered, []byte("garbage that is not a patch\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	f.writeFollowUp()
	f.writeCritic(reviewedTree, "", "", "critic-model")
	out, _ := expectConformance(t, f, "merge", 0, "integrationAuthorization=")
	digest := issuedDigest(t, out)
	patch, err := os.ReadFile(filepath.Join(authorizationsDir(f.controller, "m-gate"), digest+".patch"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(patch), "garbage") {
		t.Fatal("issuance trusted the tampered review artifact")
	}
	applied, err := gittree.Workspace{Dir: f.worktree}.Apply(
		f.authorization("m-gate", digest)["baseTree"].(string), patch)
	if err != nil || applied != reviewedTree {
		t.Fatalf("derived patch does not reproduce the reviewed tree: %v %s", err, applied)
	}
}

// An empty diff refuses on the FULL merge path: critic closure over an
// unchanged tree is still not authorizable, and the refusal fails the
// merge itself.
func TestMissionMergeRefusesEmptyDiff(t *testing.T) {
	f := newConformanceFixture(t)
	f.writeImplementer("", "docs/note.md") // declared but never changed
	f.missionize("m-empty")
	t.Setenv("METASYSTEM_MISSION_TURN", "m-empty-t2")

	expectConformance(t, f, "review", 0, "")
	f.git(f.worktree, "-c", "user.name=m", "-c", "user.email=m@x", "commit", "-q", "--allow-empty", "-m", "no change")
	reviewedTree := f.reviewedTree()
	f.writeFollowUp()
	f.writeCritic(reviewedTree, "", "", "critic-model")
	expectConformance(t, f, "merge", 1, "empty diff cannot be authorized")
	if _, err := os.Stat(authorizationsDir(f.controller, "m-empty")); !os.IsNotExist(err) {
		t.Fatal("refused empty-diff merge left an authorization")
	}
}

// An empty diff has nothing to authorize: the issuance join refuses it
// directly (a chain that shipped no change must not satisfy any
// delegation floor downstream).
func TestAuthorizationRefusesEmptyDiff(t *testing.T) {
	f := newConformanceFixture(t)
	f.writeImplementer("", "source.txt")
	f.missionize("m-empty")
	t.Setenv("METASYSTEM_MISSION_TURN", "m-empty-t2")

	record, ok := readJobRecord(filepath.Join(f.controller, "artifacts", "agents", "jobs"), "impl")
	if !ok {
		t.Fatal("cannot read the fixture job record")
	}
	run := &conformanceRun{
		root: f.controller, job: "impl", rootJob: "impl",
		record: record, workspace: f.worktree,
		boundaryBase: f.baseSha,
	}
	finalTree := f.git(f.worktree, "rev-parse", "HEAD^{tree}")
	err := run.issueAuthorization(finalTree)
	if err == nil || !strings.Contains(err.Error(), "empty diff cannot be authorized") {
		t.Fatalf("empty diff not refused: %v", err)
	}
	if _, statErr := os.Stat(authorizationsDir(f.controller, "m-empty")); !os.IsNotExist(statErr) {
		t.Fatal("refused issuance left an authorizations directory")
	}
}
