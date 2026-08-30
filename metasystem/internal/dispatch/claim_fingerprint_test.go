package dispatch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type fingerprintGolden struct {
	Name     string                 `json:"name"`
	Request  CanonicalLaunchRequest `json:"request"`
	Expected string                 `json:"sha256"`
}

func TestLaunchFingerprintV1GoldenVectors(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "claim-fingerprint-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	var vectors []fingerprintGolden
	if err := json.Unmarshal(data, &vectors); err != nil {
		t.Fatal(err)
	}
	if len(vectors) < 3 {
		t.Fatalf("golden vectors = %d, want at least 3", len(vectors))
	}
	for _, vector := range vectors {
		t.Run(vector.Name, func(t *testing.T) {
			got, err := LaunchFingerprintV1(vector.Request)
			if err != nil {
				t.Fatal(err)
			}
			if got.Digest != vector.Expected || got.Version != 1 {
				t.Fatalf("fingerprint = version %d %s, want version 1 %s", got.Version, got.Digest, vector.Expected)
			}
		})
	}
}

func TestFingerprintV1DistinguishesAbsentFromExplicitEmpty(t *testing.T) {
	presentBytes, err := encodeLaunchFingerprintV1([]fingerprintWireField{{Present: true, Value: []byte{}}})
	if err != nil {
		t.Fatal(err)
	}
	absentBytes, err := encodeLaunchFingerprintV1([]fingerprintWireField{{Present: false}})
	if err != nil {
		t.Fatal(err)
	}
	if string(presentBytes) == string(absentBytes) {
		t.Fatal("the v1 wire encoding collapsed an explicit empty field into an absent field")
	}
}

func TestFingerprintCanonicalizesProductRoots(t *testing.T) {
	gitRoot := t.TempDir()
	realRoot := filepath.Join(gitRoot, "products")
	if err := os.MkdirAll(realRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(gitRoot, "products-link")
	if err := os.Symlink(realRoot, link); err != nil {
		t.Fatal(err)
	}

	base := launchFingerprintRequestForTest()
	base.ProductRoots = []string{"products/missing/tail", "products", "products"}
	first, err := CanonicalizeLaunchFingerprint(gitRoot, base, 120)
	if err != nil {
		t.Fatal(err)
	}

	other := base
	other.ProductRoots = []string{
		filepath.Join(link, "missing", "tail"),
		realRoot,
		filepath.Join(gitRoot, "products"),
	}
	second, err := CanonicalizeLaunchFingerprint(gitRoot, other, 120)
	if err != nil {
		t.Fatal(err)
	}
	if first.Digest != second.Digest {
		t.Fatalf("relative/reordered/duplicate/symlinked/missing-tail roots diverged: %s != %s\nfirst=%v\nsecond=%v",
			first.Digest, second.Digest, first.Request.ProductRoots, second.Request.ProductRoots)
	}
}

func TestFingerprintCanonicalizesModelAndDefaultCap(t *testing.T) {
	root := t.TempDir()
	explicit := launchFingerprintRequestForTest()
	explicit.Model = "GPT 5.6 Sol"
	explicit.CapMinutes = 120
	one, err := CanonicalizeLaunchFingerprint(root, explicit, 999)
	if err != nil {
		t.Fatal(err)
	}

	defaulted := explicit
	defaulted.Model = "gpt-5-6-sol"
	defaulted.CapMinutes = 0
	two, err := CanonicalizeLaunchFingerprint(root, defaulted, 120)
	if err != nil {
		t.Fatal(err)
	}
	if one.Digest != two.Digest {
		t.Fatalf("model aliases or explicit/default cap diverged: %s != %s", one.Digest, two.Digest)
	}
}

func TestFingerprintRefusesOperationalProductRoots(t *testing.T) {
	root := t.TempDir()
	request := launchFingerprintRequestForTest()
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	request.ProductRoots = []string{jobs}
	if _, err := CanonicalizeLaunchFingerprint(root, request, 120); err == nil {
		t.Fatal("the job registry was accepted as a product root")
	}
	worktree := filepath.Join(root, "artifacts", "agents", "worktrees", "job-a")
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatal(err)
	}
	request.ProductRoots = []string{filepath.Dir(worktree)}
	if _, err := CanonicalizeLaunchFingerprint(root, request, 120); err == nil {
		t.Fatal("the shared worktree container was accepted as one job's product root")
	}
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		t.Fatal(err)
	}
	registryLink := filepath.Join(worktree, "registry")
	if err := os.Symlink(jobs, registryLink); err != nil {
		t.Fatal(err)
	}
	request.ProductRoots = []string{registryLink}
	if _, err := CanonicalizeLaunchFingerprint(root, request, 120); err == nil {
		t.Fatal("a worktree root resolving into the job registry was accepted")
	}
	request.ProductRoots = []string{filepath.Join(root, ".git", "objects")}
	if _, err := CanonicalizeLaunchFingerprint(root, request, 120); err == nil {
		t.Fatal("Git metadata was accepted as a product root")
	}
}

func TestFingerprintAcceptsDelegateWorktreeProductRoot(t *testing.T) {
	root := t.TempDir()
	worktree := filepath.Join(root, "artifacts", "agents", "worktrees", "job-a")
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatal(err)
	}
	request := launchFingerprintRequestForTest()
	request.ProductRoots = []string{worktree}
	fingerprint, err := CanonicalizeLaunchFingerprint(root, request, 120)
	if err != nil {
		t.Fatalf("delegate worktree product root was refused: %v", err)
	}
	if len(fingerprint.Request.ProductRoots) != 1 || fingerprint.Request.ProductRoots[0] != resolvePath(worktree) {
		t.Fatalf("canonical product roots = %v, want %s", fingerprint.Request.ProductRoots, resolvePath(worktree))
	}
}

func TestFingerprintChangesForDispatchModeAndResumedSession(t *testing.T) {
	root := t.TempDir()
	fresh := launchFingerprintRequestForTest()
	freshFingerprint, err := CanonicalizeLaunchFingerprint(root, fresh, 120)
	if err != nil {
		t.Fatal(err)
	}

	resumed := "runtime-session-a"
	follow := fresh
	follow.DispatchMode = DispatchModeFollowUp
	follow.ResumedSessionID = &resumed
	followFingerprint, err := CanonicalizeLaunchFingerprint(root, follow, 120)
	if err != nil {
		t.Fatal(err)
	}
	if freshFingerprint.Digest == followFingerprint.Digest {
		t.Fatal("fresh and follow-up requests hashed identically")
	}

	resumed = "runtime-session-b"
	otherFollow, err := CanonicalizeLaunchFingerprint(root, follow, 120)
	if err != nil {
		t.Fatal(err)
	}
	if followFingerprint.Digest == otherFollow.Digest {
		t.Fatal("two resumed runtime sessions hashed identically")
	}
}

func launchFingerprintRequestForTest() LaunchFingerprintRequest {
	empty := ""
	return LaunchFingerprintRequest{
		SessionKey:               "runtime:session-a",
		DispatchMode:             DispatchModeFresh,
		ResumedSessionID:         &empty,
		Runtime:                  "codex",
		Model:                    "gpt-5.6-sol",
		Role:                     "implementer",
		LaunchMode:               LaunchModeWorktree,
		PermissionEnvelopeDigest: "1111111111111111111111111111111111111111111111111111111111111111",
		ProductRoots:             nil,
		CapMinutes:               120,
		InputHash:                "2222222222222222222222222222222222222222222222222222222222222222",
		DestructiveReach:         HazardMechanical,
	}
}
