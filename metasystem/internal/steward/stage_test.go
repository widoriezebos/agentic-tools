package steward

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stagedRepo carries the role and permissions files staging digests.
func stagedRepo(t *testing.T) string {
	root := gitRepoWithCurrentGoal(t)
	write := func(rel, body string) {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("scripts/agents/roles/steward-continuation.md", "# Role: steward-continuation\ncontract\n")
	write("scripts/agents/roles/steward-continuation.requirements.json", `{"required":[]}`)
	write("scripts/agents/schemas/steward-continuation.schema.json", `{"type":"object"}`)
	write("scripts/agents/permissions/workspace.json", `{"write":["workspace"]}`)
	top, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	idPath := RepoIdentityPath(top)
	if err := os.MkdirAll(filepath.Dir(idPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := MintIdentity(idPath, InstallIdentity{RepoIdentity: top, Generation: 1, InstallPath: "/bin/true", MintedAt: "2026-08-20T15:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestStagingBindsTheBytesThatWillRun(t *testing.T) {
	root := stagedRepo(t)
	it, err := StageIntent(root, "st-1", "fix-it", "job-9", "fake", "fixture", "worker provably dead")
	if err != nil {
		t.Fatal(err)
	}
	if it.Role != "steward-continuation" || it.Permissions != "workspace" {
		t.Fatalf("the role and preset are fixed, never chosen: %+v", it)
	}
	if err := VerifyStagedDigests(root, it); err != nil {
		t.Fatalf("unchanged bytes must verify: %v", err)
	}
	brief, err := os.ReadFile(BriefPath(root, "st-1"))
	if err != nil || !strings.Contains(string(brief), `"fix-it"`) {
		t.Fatalf("the brief names the goal: %q %v", brief, err)
	}
}

func TestDriftBetweenMintAndLaunchRefusesByField(t *testing.T) {
	root := stagedRepo(t)
	it, err := StageIntent(root, "st-2", "fix-it", "job-9", "fake", "fixture", "dead")
	if err != nil {
		t.Fatal(err)
	}
	rolePath := filepath.Join(root, "scripts", "agents", "roles", "steward-continuation.md")
	if err := os.WriteFile(rolePath, []byte("# tampered\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := VerifyStagedDigests(root, it); err == nil || !strings.Contains(err.Error(), "role contract drifted") {
		t.Fatalf("role drift must refuse by field: %v", err)
	}
}
