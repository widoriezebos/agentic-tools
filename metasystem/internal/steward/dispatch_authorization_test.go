package steward

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// authorizationBed is a checkout with an enrolled installation and one
// consumed continuation intent whose staged bytes match their digests.
func authorizationBed(t *testing.T) (string, Intent) {
	t.Helper()
	root, err := canonicalExistingPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	write := func(relative, content string) string {
		path := filepath.Join(root, relative)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	digest := func(path string) string {
		sum, err := digestFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return sum
	}
	it := testIntent("auth-1")
	it.Role, it.Permissions, it.JobId = "steward-continuation", "workspace", "steward-auth-1"
	it.RoleDigest = digest(write("scripts/agents/roles/steward-continuation.md", "# Role\n"))
	it.ReqDigest = digest(write("scripts/agents/roles/steward-continuation.requirements.json", `{"required":[]}`))
	it.SchemaDigest = digest(write("scripts/agents/schemas/steward-continuation.schema.json", `{"type":"object"}`))
	it.PermsDigest = digest(write("scripts/agents/permissions/workspace.json", `{"write":["workspace"]}`))
	it.BriefDigest = digest(write(strings.TrimPrefix(BriefPath(root, it.Nonce), root+"/"), "# Continue\n"))
	it.RepoIdentity, it.InstallGen = root, 1
	if err := os.MkdirAll(filepath.Dir(RepoIdentityPath(root)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := MintIdentity(RepoIdentityPath(root), InstallIdentity{RepoIdentity: root, Generation: 1, InstallPath: "/bin/true", MintedAt: "2026-09-27T09:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	consumedIntentOnDisk(t, root, it)
	return root, it
}

// The dispatcher's half of the continuation gate (moved out of the steward
// authorize-dispatch verb in U6-seam and called in-process by the delegate
// lifecycle since U6b): its admitted tuple and every refusal message.
func TestAuthorizeDispatchAdmitsTheStagedTupleAndNamesEachRefusal(t *testing.T) {
	root, it := authorizationBed(t)
	authorization, err := AuthorizeDispatch(root, it.Nonce)
	if err != nil {
		t.Fatal(err)
	}
	want := DispatchAuthorization{Goal: "fix-it", JobId: "steward-auth-1", Runtime: "fake", Model: "fixture",
		Role: "steward-continuation", Permissions: "workspace", Brief: BriefPath(root, it.Nonce)}
	if authorization != want {
		t.Fatalf("authorization %+v, want %+v", authorization, want)
	}
	encoded, err := json.Marshal(authorization)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]any
	if err := json.Unmarshal(encoded, &wire); err != nil {
		t.Fatal(err)
	}
	var keys []string
	for key := range wire {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if !reflect.DeepEqual(keys, []string{"brief", "goal", "jobId", "model", "permissions", "role", "runtime"}) {
		t.Fatalf("wire keys %v", keys)
	}

	if _, err := AuthorizeDispatch(root, "never-minted"); err == nil || !strings.HasPrefix(err.Error(), "no consumed intent never-minted") {
		t.Fatalf("an unknown intent: %v", err)
	}
	if err := os.WriteFile(BriefPath(root, it.Nonce), []byte("# Tampered\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := AuthorizeDispatch(root, it.Nonce); err == nil || err.Error() != "staged brief drifted since the authorization was minted" {
		t.Fatalf("a tampered brief: %v", err)
	}
	stamped := it
	stamped.Nonce, stamped.LaunchStamped = "auth-2", true
	consumedIntentOnDisk(t, root, stamped)
	if _, err := AuthorizeDispatch(root, "auth-2"); err == nil || err.Error() != "intent auth-2 already launched; a replay authorizes nothing" {
		t.Fatalf("a stamped intent: %v", err)
	}

	superseded, superseding := authorizationBed(t)
	superseding.InstallGen = 2
	consumedIntentOnDisk(t, superseded, superseding)
	if _, err := AuthorizeDispatch(superseded, superseding.Nonce); err == nil ||
		!strings.Contains(err.Error(), "minted under installation generation 2") || !strings.Contains(err.Error(), "a superseded authorization launches nothing") {
		t.Fatalf("a superseded generation: %v", err)
	}
}
