package missionrunner

import (
	"os"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

func TestUnmergedOutsidePrefix(t *testing.T) {
	before := []string{"100644 a 1\tws/x", "100644 b 1\tsib/y"}
	// A workspace-only unmerged delta is lawful.
	if got := unmergedOutsidePrefix(before, []string{"100644 b 1\tsib/y"}, "ws/"); got != "" {
		t.Fatalf("workspace unmerged delta must be lawful: %q", got)
	}
	// A sibling unmerged entry appearing is unlawful.
	if got := unmergedOutsidePrefix([]string{}, []string{"100644 b 1\tsib/y"}, "ws/"); got == "" {
		t.Fatal("a sibling unmerged entry must be named")
	}
	// A malformed entry (no TAB) is outside any prefix → named.
	if got := unmergedOutsidePrefix([]string{}, []string{"no-tab-entry"}, "ws/"); got == "" {
		t.Fatal("a malformed unmerged entry must be named")
	}
}

func TestWorktreePostureDrift(t *testing.T) {
	base := gittree.WorktreeRecord{
		HeadOID: strings.Repeat("a", 40),
		Pseudorefs: []gittree.Pseudoref{
			{Name: "ORIG_HEAD", OIDs: []string{strings.Repeat("a", 40)}, Parseable: true},
		},
		Staged: gittree.StagedPosture{Tree: strings.Repeat("b", 40)},
	}
	recorded := map[string]any{
		"headOid": strings.Repeat("a", 40),
		"pseudorefs": []any{map[string]any{
			"name": "ORIG_HEAD", "oids": []any{strings.Repeat("a", 40)}, "parseable": true,
		}},
		"staged": map[string]any{"tree": strings.Repeat("b", 40), "unmerged": []any{}},
	}
	if drift := worktreePostureDrift(recorded, base); drift != "" {
		t.Fatalf("matching posture must not drift: %q", drift)
	}
	moved := base
	moved.HeadOID = strings.Repeat("c", 40)
	if drift := worktreePostureDrift(recorded, moved); !strings.Contains(drift, "HEAD") {
		t.Fatalf("moved HEAD must drift: %q", drift)
	}
	restaged := base
	restaged.Staged = gittree.StagedPosture{Tree: strings.Repeat("d", 40)}
	if drift := worktreePostureDrift(recorded, restaged); !strings.Contains(drift, "staged") {
		t.Fatalf("changed staged posture must drift: %q", drift)
	}
	refChanged := base
	refChanged.Pseudorefs = []gittree.Pseudoref{{Name: "ORIG_HEAD", OIDs: []string{strings.Repeat("e", 40)}, Parseable: true}}
	if drift := worktreePostureDrift(recorded, refChanged); !strings.Contains(drift, "pseudoref") {
		t.Fatalf("changed pseudoref must drift: %q", drift)
	}
	// A record with NO staged baseline can never prove the live staged
	// posture unchanged (WSS I11-7): a readability transition after
	// consumption must re-judge, not silently pass.
	bare := map[string]any{
		"headOid": strings.Repeat("a", 40),
		"pseudorefs": []any{map[string]any{
			"name": "ORIG_HEAD", "oids": []any{strings.Repeat("a", 40)}, "parseable": true,
		}},
	}
	if drift := worktreePostureDrift(bare, base); !strings.Contains(drift, "no recorded staged baseline") {
		t.Fatalf("a nil recorded staged posture must drift: %q", drift)
	}
}

func TestRefMapAndStagedFromDoc(t *testing.T) {
	refs := refMapFromDoc(map[string]any{"refs/heads/main": strings.Repeat("a", 40), "bad": 7})
	if refs["refs/heads/main"] != strings.Repeat("a", 40) || len(refs) != 1 {
		t.Fatalf("refMapFromDoc: %v", refs)
	}
	if refMapFromDoc("not-a-map") == nil {
		t.Fatal("refMapFromDoc must return an empty map, not nil, on bad input")
	}
	posture := stagedPostureFromDoc(map[string]any{"tree": strings.Repeat("a", 40), "unmerged": []any{"x"}})
	if posture == nil || posture.Tree != strings.Repeat("a", 40) || len(posture.Unmerged) != 1 {
		t.Fatalf("stagedPostureFromDoc: %+v", posture)
	}
	if stagedPostureFromDoc(nil) != nil {
		t.Fatal("nil staged doc must give nil posture")
	}
}

func TestScopeOriginFromPosture(t *testing.T) {
	origin := scopeOriginFromPosture(
		strings.Repeat("a", 40),
		map[string]any{"refs/heads/main": strings.Repeat("b", 40)},
		strings.Repeat("c", 40),
		map[string]any{"tree": strings.Repeat("d", 40), "unmerged": []any{}},
		strings.Repeat("e", 40),
		[]any{map[string]any{"path": "/w"}},
	)
	if origin.Head != strings.Repeat("a", 40) || origin.TopTree != strings.Repeat("c", 40) {
		t.Fatalf("origin head/top: %+v", origin)
	}
	if origin.OpenAnchor != "" {
		t.Fatal("between-turns origin must leave OpenAnchor empty")
	}
	if len(origin.Census) != 1 {
		t.Fatalf("census: %+v", origin.Census)
	}
}

func TestOriginPseudorefsAndUnchanged(t *testing.T) {
	origin := &scopeOrigin{Census: []map[string]any{{
		"path": "/w",
		"pseudorefs": []any{map[string]any{
			"name": "ORIG_HEAD", "oids": []any{strings.Repeat("a", 40)}, "parseable": true,
		}},
	}}}
	recorded := originPseudorefs(origin, "/w")
	if len(recorded["ORIG_HEAD"].oids) != 1 {
		t.Fatalf("originPseudorefs: %v", recorded)
	}
	live := gittree.Pseudoref{Name: "ORIG_HEAD", OIDs: []string{strings.Repeat("a", 40)}, Parseable: true}
	if !pseudorefUnchanged(recorded, live) {
		t.Fatal("matching pseudoref must read unchanged")
	}
	moved := gittree.Pseudoref{Name: "ORIG_HEAD", OIDs: []string{strings.Repeat("b", 40)}, Parseable: true}
	if pseudorefUnchanged(recorded, moved) {
		t.Fatal("moved pseudoref must read changed")
	}
	fresh := gittree.Pseudoref{Name: "MERGE_HEAD", OIDs: []string{strings.Repeat("a", 40)}, Parseable: true}
	if pseudorefUnchanged(recorded, fresh) {
		t.Fatal("an absent-from-origin pseudoref must read changed")
	}
	unparseable := gittree.Pseudoref{Name: "ORIG_HEAD", OIDs: []string{strings.Repeat("a", 40)}, Parseable: false}
	if pseudorefUnchanged(recorded, unparseable) {
		t.Fatal("a parseability change must read changed even with the same OID list")
	}
	// An already-unparseable ref rewritten to different unparseable
	// content but the same collected OID list must still re-judge: an
	// unparseable side is never compared as unchanged.
	unparseableOrigin := &scopeOrigin{Census: []map[string]any{{
		"path": "/w",
		"pseudorefs": []any{map[string]any{
			"name": "ORIG_HEAD", "oids": []any{strings.Repeat("a", 40)}, "parseable": false,
		}},
	}}}
	rec2 := originPseudorefs(unparseableOrigin, "/w")
	if pseudorefUnchanged(rec2, unparseable) {
		t.Fatal("an unparseable ref must never read unchanged, same OID list or not")
	}
}

func TestAgentBranchJob(t *testing.T) {
	root := t.TempDir()
	engine := &Engine{Root: root, Mission: "demo"}
	jobsDir := jobsDirPath(root)
	if err := os.MkdirAll(jobsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeJSONFile(t, jobsDir+"/live.json", map[string]any{"jobId": "live", "branch": "agent/live", "mission": "demo", "status": "running"})
	writeJSONFile(t, jobsDir+"/foreign.json", map[string]any{"jobId": "foreign", "branch": "agent/foreign", "mission": "other", "status": "running"})
	if engine.agentBranchJob("refs/heads/agent/live") != "live" {
		t.Fatal("a same-mission agent branch must resolve its job")
	}
	if engine.agentBranchJob("refs/heads/agent/foreign") != "" {
		t.Fatal("a foreign-mission job must not be admitted")
	}
	if engine.agentBranchJob("refs/heads/main") != "" {
		t.Fatal("a non-agent branch resolves no job")
	}
}
