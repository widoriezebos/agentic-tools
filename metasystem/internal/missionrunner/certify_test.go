package missionrunner

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/validate"
)

// The certification wall's test bed: a mission with a live incarnation,
// one dispatched job whose record identity digests cleanly, and one
// authorization record binding exactly that job. Each test then breaks
// one strand and asserts the claim is refused for that reason alone.

const certIncarnation = "1111111111111111111111111111111111111111111111111111111111111111"

func certDigest(fill string) string {
	return strings.Repeat(fill, 64)
}

func certBed(t *testing.T) (root string, state map[string]any, jobRecord map[string]any) {
	t.Helper()
	root = t.TempDir()
	missionID := "demo"
	jobRecord = map[string]any{
		"jobId": "job-cert", "role": "implementer", "runtime": "codex", "round": 1,
		"parentJob": nil, "reviews": []any{"job-critic"}, "workspaceRoot": "wt",
		"baseSha": "abc", "branch": "agent/job-cert", "startedAt": "2026-08-18T00:00:00Z",
		"claimEpoch": nil, "mainId": "m", "capMin": 30, "capDeadline": nil,
		"capResolution": nil, "mission": missionID, "missionIncarnation": certIncarnation,
		"turnId": "demo-t1", "stream": "s-app", "status": "completed",
	}
	writeJSONFile(t, filepath.Join(jobsDirPath(root), "job-cert.json"), jobRecord)
	writeJSONFile(t, filepath.Join(missionDirPath(root, missionID), "fences.json"), map[string]any{
		"approvedContractSha256": certIncarnation,
	})
	state = map[string]any{"turnLog": []any{}}
	return root, state, jobRecord
}

// certAuthorization mints a SELF-CONSISTENT record: the content (after
// any mutation) is canonically digested and written under that digest —
// consumption authenticates the bytes, so fixtures fabricate content, not
// digests.
func certAuthorization(t *testing.T, root string, mutate func(map[string]any)) string {
	t.Helper()
	jobRecord, err := readJSONDoc(filepath.Join(jobsDirPath(root), "job-cert.json"))
	if err != nil {
		t.Fatal(err)
	}
	identityDigest, err := validate.JobIdentityDigest(jobRecord)
	if err != nil {
		t.Fatal(err)
	}
	record := map[string]any{
		"schemaVersion": 1, "jobId": "job-cert", "rootJob": "job-cert",
		"jobRecordDigest": identityDigest, "mission": "demo",
		"missionIncarnation": certIncarnation, "stream": "s-app",
		"dispatchTurn": "demo-t1", "issuanceTurn": "demo-t1",
		"baseTree": certDigest("b")[:40], "reviewedTree": certDigest("c")[:40],
		"baseSequencePoint": map[string]any{"sequence": 0, "segment": 0},
		"patchDigest":       certDigest("d"), "changedPaths": []any{"main.go"},
		"supersedes": []any{},
	}
	if mutate != nil {
		mutate(record)
	}
	digest, err := validate.AuthorizationRecordDigest(record)
	if err != nil {
		t.Fatal(err)
	}
	record["authorizationDigest"] = digest
	dir := filepath.Join(missionDirPath(root, "demo"), "authorizations")
	writeJSONFile(t, filepath.Join(dir, digest+".json"), record)
	return digest
}

func certClaim(verdict, digest string) map[string]any {
	entry := map[string]any{"jobId": "job-cert", "verdict": verdict, "evidence": "critic closed"}
	if digest == "" {
		entry["authorizationDigest"] = nil
	} else {
		entry["authorizationDigest"] = digest
	}
	return entry
}

func TestCertifiedClaimVerifies(t *testing.T) {
	root, state, _ := certBed(t)
	digest := certAuthorization(t, root, nil)
	certified, rejected, err := adjudicateCertified(root, "demo", state, []map[string]any{certClaim("accepted", digest)})
	if err != nil {
		t.Fatal(err)
	}
	if len(rejected) != 0 || len(certified) != 1 {
		t.Fatalf("a clean claim must verify: certified=%v rejected=%v", certified, rejected)
	}
	if certified[0]["authorizationDigest"] != digest || certified[0]["verdict"] != "accepted" {
		t.Fatalf("adjudicated entry: %v", certified[0])
	}
}

func TestCertifiedClaimRejections(t *testing.T) {
	cases := []struct {
		name   string
		bed    func(t *testing.T, root string, state map[string]any) string
		reason string
	}{
		{"missing digest on accepted", func(t *testing.T, root string, state map[string]any) string {
			certAuthorization(t, root, nil)
			return ""
		}, "must name its integration authorization"},
		{"authorization record absent", func(t *testing.T, root string, state map[string]any) string {
			return certDigest("a")
		}, "does not exist"},
		{"rewritten record fails authentication", func(t *testing.T, root string, state map[string]any) string {
			digest := certAuthorization(t, root, nil)
			path := filepath.Join(missionDirPath(root, "demo"), "authorizations", digest+".json")
			record, _ := readJSONDoc(path)
			record["reviewedTree"] = certDigest("9")[:40]
			writeJSONFile(t, path, record)
			return digest
		}, "does not exist"},
		{"issued for a different job", func(t *testing.T, root string, state map[string]any) string {
			return certAuthorization(t, root, func(r map[string]any) { r["jobId"] = "job-other" })
		}, "different job"},
		{"issued for a different mission", func(t *testing.T, root string, state map[string]any) string {
			return certAuthorization(t, root, func(r map[string]any) { r["mission"] = "other" })
		}, "different mission"},
		{"incarnation mismatch", func(t *testing.T, root string, state map[string]any) string {
			return certAuthorization(t, root, func(r map[string]any) { r["missionIncarnation"] = certDigest("9") })
		}, "different mission incarnation"},
		{"job record drifted since issuance", func(t *testing.T, root string, state map[string]any) string {
			digest := certAuthorization(t, root, nil)
			record, _ := readJSONDoc(filepath.Join(jobsDirPath(root), "job-cert.json"))
			record["branch"] = "agent/tampered"
			writeJSONFile(t, filepath.Join(jobsDirPath(root), "job-cert.json"), record)
			return digest
		}, "no longer matches"},
		{"superseded round", func(t *testing.T, root string, state map[string]any) string {
			digest := certAuthorization(t, root, nil)
			certAuthorization(t, root, func(r map[string]any) {
				r["supersedes"] = []any{digest}
			})
			return digest
		}, "superseded"},
		{"forked chain", func(t *testing.T, root string, state map[string]any) string {
			digest := certAuthorization(t, root, nil)
			certAuthorization(t, root, func(r map[string]any) { r["issuanceTurn"] = "demo-t9" })
			return digest
		}, "forked chain"},
		{"already consumed", func(t *testing.T, root string, state map[string]any) string {
			digest := certAuthorization(t, root, nil)
			tree := certDigest("f")[:40]
			state["turnLog"] = []any{map[string]any{
				"turnId": "demo-t0", "consumedAuthorizations": []any{digest},
				"wall": map[string]any{"verdict": "passed", "preTree": tree,
					"expectedTree": tree, "postTree": tree, "orderedDigests": []any{digest},
					"sequencePoint":  map[string]any{"sequence": 1, "segment": 0},
					"headCommitPost": strings.Repeat("c", 40), "refMapPost": map[string]any{},
					"stagedTreePost": tree, "topTreePost": nil, "topStagedPost": nil,
					"worktreeCensusPost": []any{}, "capturedAt": "2026-01-01T00:00:00Z"},
			}}
			return digest
		}, "already consumed by turn demo-t0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, state, _ := certBed(t)
			claim := certClaim("accepted", tc.bed(t, root, state))
			certified, rejected, err := adjudicateCertified(root, "demo", state, []map[string]any{claim})
			if err != nil {
				t.Fatal(err)
			}
			if len(certified) != 0 || len(rejected) != 1 {
				t.Fatalf("claim must be refused: certified=%v rejected=%v", certified, rejected)
			}
			reason, _ := rejected[0]["reason"].(string)
			if !strings.Contains(reason, tc.reason) {
				t.Fatalf("refusal reason %q does not name %q", reason, tc.reason)
			}
		})
	}
}

func TestCertifiedRejectedVerdictConsumesNothing(t *testing.T) {
	root, state, _ := certBed(t)
	// A rejected certification is lawful with a null digest, and any digest
	// it does name is normalized away — nothing unverified reaches the log.
	claims := []map[string]any{certClaim("rejected", ""), certClaim("rejected", certDigest("a"))}
	certified, rejected, err := adjudicateCertified(root, "demo", state, claims)
	if err != nil {
		t.Fatal(err)
	}
	if len(rejected) != 0 || len(certified) != 2 {
		t.Fatalf("rejected-verdict reports are lawful: certified=%v rejected=%v", certified, rejected)
	}
	for _, entry := range certified {
		if entry["authorizationDigest"] != nil {
			t.Fatalf("a rejection consumed an authorization: %v", entry)
		}
	}
}

func TestCertifiedClaimNeedsARealJob(t *testing.T) {
	root, state, _ := certBed(t)
	ghost := map[string]any{"jobId": "job-ghost", "verdict": "rejected", "evidence": "x", "authorizationDigest": nil}
	certified, rejected, err := adjudicateCertified(root, "demo", state, []map[string]any{ghost})
	if err != nil {
		t.Fatal(err)
	}
	if len(certified) != 0 || len(rejected) != 1 {
		t.Fatalf("a certification of a job that never existed must be refused: %v", certified)
	}
}

// The conclusion plumbing ships ONLY adjudicated certifications:
// a forged claim in the raw return must never reach the turn log, even
// when the verdict file beside it carries the verified list.
func TestConcludeFilesShipsAdjudicatedCertifiedOnly(t *testing.T) {
	root := t.TempDir()
	missionID := "demo"
	seedFences(t, root, missionID)
	dir := t.TempDir()
	adjudicated := []any{map[string]any{
		"jobId": "job-cert", "verdict": "accepted",
		"evidence": "critic closed", "authorizationDigest": certDigest("a"),
	}}
	files := map[string]map[string]any{
		"state.json": cycleState(activeStreams()),
		"turn.json": {"turnId": "demo-t3-ab12", "missionId": missionID, "cycle": 3,
			"runtime": "fake", "model": "fixture", "hostSession": nil},
		"verdict.json": {"streams": activeStreams(), "accepted": []any{},
			"rejected": []any{}, "certified": adjudicated},
		"return.json": {"certified": []any{map[string]any{
			"jobId": "job-forged", "verdict": "accepted",
			"evidence": "trust me", "authorizationDigest": certDigest("9"),
		}}, "factsForLedger": []any{}, "gaps": []any{}},
		"result.json":      {"sessionId": "sess-1"},
		"measurement.json": {"gatePassed": false, "measurement": nil},
	}
	for name, doc := range files {
		writeJSONFile(t, filepath.Join(dir, name), doc)
	}
	writeJSONFile(t, filepath.Join(missionDirPath(root, missionID), "turns", "demo-t3-ab12", "wall.json"),
		map[string]any{"verdict": "passed", "preTree": certDigest("b")[:40],
			"expectedTree": certDigest("c")[:40], "postTree": certDigest("c")[:40],
			"orderedDigests": []any{certDigest("a")},
			"posture": map[string]any{
				"headCommitPost": strings.Repeat("c", 40), "refMapPost": map[string]any{},
				"stagedTreePost": certDigest("c")[:40], "topTreePost": nil, "topStagedPost": nil,
				"worktreeCensusPost": []any{}, "capturedAt": "2026-01-01T00:00:00Z"}})
	proposed, err := ConcludeFiles(root, missionID, filepath.Join(dir, "state.json"),
		filepath.Join(dir, "turn.json"), filepath.Join(dir, "verdict.json"),
		filepath.Join(dir, "return.json"), filepath.Join(dir, "result.json"),
		filepath.Join(dir, "measurement.json"))
	if err != nil {
		t.Fatal(err)
	}
	entry := proposed["turnLog"].([]any)[0].(map[string]any)
	certified, _ := entry["certified"].([]any)
	if len(certified) != 1 {
		t.Fatalf("turn log certified: %v", entry["certified"])
	}
	shipped := certified[0].(map[string]any)
	if shipped["jobId"] != "job-cert" || shipped["authorizationDigest"] != certDigest("a") {
		t.Fatalf("the raw return's claim leaked into the turn log: %v", shipped)
	}
}
