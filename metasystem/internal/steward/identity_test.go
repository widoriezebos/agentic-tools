package steward

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIdentityMintVerifyRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "identity.json")
	want := InstallIdentity{RepoIdentity: "repo-ulid", Generation: 2, InstallPath: "/opt/x", MintedAt: "2026-08-20T09:00:00Z"}
	if err := MintIdentity(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := VerifyIdentity(path, "repo-ulid")
	if err != nil || got != want {
		t.Fatalf("round trip: %+v %v", got, err)
	}
}

func TestForeignRepositoryRefusesByName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "identity.json")
	if err := MintIdentity(path, InstallIdentity{RepoIdentity: "other", Generation: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyIdentity(path, "repo-ulid"); err == nil || !strings.Contains(err.Error(), "serves repository") {
		t.Fatalf("foreign identity must refuse by name: %v", err)
	}
}

func TestGroupReadableIdentityRefuses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "identity.json")
	if err := MintIdentity(path, InstallIdentity{RepoIdentity: "repo-ulid", Generation: 1}); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyIdentity(path, "repo-ulid"); err == nil || !strings.Contains(err.Error(), "owner-only") {
		t.Fatalf("loose permissions must refuse by name: %v", err)
	}
}

func TestAbsentAndMalformedIdentityRefuse(t *testing.T) {
	dir := t.TempDir()
	if _, err := VerifyIdentity(filepath.Join(dir, "missing.json"), "r"); err == nil {
		t.Fatal("absent identity must refuse")
	}
	torn := filepath.Join(dir, "torn.json")
	if err := os.WriteFile(torn, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyIdentity(torn, "r"); err == nil || !strings.Contains(err.Error(), "malformed") {
		t.Fatalf("malformed identity must refuse by name: %v", err)
	}
}

func TestZeroGenerationRefuses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "identity.json")
	if err := MintIdentity(path, InstallIdentity{RepoIdentity: "r", Generation: 0}); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyIdentity(path, "r"); err == nil || !strings.Contains(err.Error(), "generation") {
		t.Fatalf("generation zero must refuse: %v", err)
	}
}
