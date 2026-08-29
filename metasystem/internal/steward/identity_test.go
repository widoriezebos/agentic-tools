package steward

import (
	"errors"
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

func TestVerifyEnrolledBinaryDetectsByteDrift(t *testing.T) {
	root := canonicalPath(t.TempDir())
	bin := filepath.Join(root, "metasystem")
	if err := os.WriteFile(bin, []byte("accepted engine\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	digest, err := installDigest(bin)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(RepoIdentityPath(root)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := MintIdentity(RepoIdentityPath(root), InstallIdentity{
		RepoIdentity: canonicalPath(root), Generation: 3,
		InstallPath: bin, InstallDigest: digest, MintedAt: "2026-08-28T00:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}
	if installed, err := VerifyEnrolledBinary(root); err != nil || installed.Generation != 3 {
		t.Fatalf("accepted engine did not verify: %+v %v", installed, err)
	}
	if err := os.WriteFile(bin, []byte("changed engine\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyEnrolledBinary(root); !errors.Is(err, ErrEnrollmentDrift) {
		t.Fatalf("byte drift was not typed ENROLLMENT_DRIFT: %v", err)
	}
}

func TestPinnedEnrollmentExecutesTheVerifiedInodeAfterPathReplacement(t *testing.T) {
	root := canonicalPath(t.TempDir())
	bin := filepath.Join(root, "metasystem")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nprintf 'accepted-engine\\n'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	digest, err := installDigest(bin)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(RepoIdentityPath(root)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := MintIdentity(RepoIdentityPath(root), InstallIdentity{
		RepoIdentity: root, Generation: 4, InstallPath: bin, InstallDigest: digest,
	}); err != nil {
		t.Fatal(err)
	}
	pinned, err := OpenEnrolledBinary(root)
	if err != nil {
		t.Fatal(err)
	}
	defer pinned.Close()
	if err := pinned.PrepareForExecution(); err != nil {
		t.Fatal(err)
	}
	temporary := bin + ".replacement"
	if err := os.WriteFile(temporary, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(temporary, bin); err != nil {
		t.Fatal(err)
	}
	command, err := pinned.Command()
	if err != nil {
		t.Fatal(err)
	}
	output, err := command.CombinedOutput()
	if err != nil || string(output) != "accepted-engine\n" {
		t.Fatalf("execution did not use the verified inode: output=%q err=%v", output, err)
	}
}

func TestPinnedEnrollmentRejectsChangedBytesInThePreparedInode(t *testing.T) {
	root := canonicalPath(t.TempDir())
	bin := filepath.Join(root, "metasystem")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	digest, err := installDigest(bin)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(RepoIdentityPath(root)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := MintIdentity(RepoIdentityPath(root), InstallIdentity{
		RepoIdentity: root, Generation: 5, InstallPath: bin, InstallDigest: digest,
	}); err != nil {
		t.Fatal(err)
	}
	pinned, err := OpenEnrolledBinary(root)
	if err != nil {
		t.Fatal(err)
	}
	defer pinned.Close()
	if err := pinned.PrepareForExecution(); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(pinned.execPath, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pinned.execPath, []byte("#!/bin/sh\nexit 1\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := pinned.Command(); !errors.Is(err, ErrEnrollmentDrift) {
		t.Fatalf("changed prepared inode was not rejected as ENROLLMENT_DRIFT: %v", err)
	}
}
