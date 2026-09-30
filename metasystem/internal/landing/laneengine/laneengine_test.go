package laneengine

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

// enrollBed is a nested lane checkout (checkout root != installation root)
// whose installation carries a steward enrollment the test writes through
// the steward's own publication.
type enrollBed struct {
	checkout, installation, installPath string
}

func newEnrollBed(t *testing.T) *enrollBed {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	bed := &enrollBed{checkout: filepath.Join(base, "lane")}
	bed.installation = filepath.Join(bed.checkout, "metasystem")
	bed.installPath = filepath.Join(bed.installation, "bin", "metasystem")
	for _, dir := range []string{filepath.Dir(bed.installPath), filepath.Join(bed.installation, "artifacts", "agents", "steward")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return bed
}

// enroll mints generation 3 with the given digest; the enrolled bytes at
// the install path are written as given.
func (bed *enrollBed) enroll(t *testing.T, digest, stamp string, bytes []byte) steward.InstallIdentity {
	t.Helper()
	if bytes != nil {
		if err := os.WriteFile(bed.installPath, bytes, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	id := steward.InstallIdentity{RepoIdentity: bed.installation, Generation: 3, InstallPath: bed.installPath,
		InstallDigest: digest, MintedAt: "2026-09-30T18:00:00Z", Enrollment: steward.EnrollmentHumanTerminal, EngineBuild: stamp}
	if err := steward.MintIdentity(steward.RepoIdentityPath(bed.installation), id); err != nil {
		t.Fatal(err)
	}
	return id
}

func digestOf(bytes []byte) string { return fmt.Sprintf("sha256:%x", sha256.Sum256(bytes)) }

func runningBytes(t *testing.T) []byte {
	t.Helper()
	path, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func refusalOf(t *testing.T, err error, code string) *Refusal {
	t.Helper()
	var refusal *Refusal
	if !errors.As(err, &refusal) || refusal.Code != code {
		t.Fatalf("err = %v; want a %s refusal", err, code)
	}
	if refusal.Message == "" || strings.Contains(refusal.Message, "\n") || len(refusal.Argv) == 0 {
		t.Fatalf("refusal %+v is not one plain line and one command", refusal)
	}
	return refusal
}

// The check hashes the executable this process really runs (the test
// binary) and compares it with the lane's enrollment: the enrolled bytes
// pass, anything else is refused, and an enrollment it cannot read refuses
// too (fail closed).
func TestRequireSelfAdmitsOnlyTheEnrolledRunningExecutable(t *testing.T) {
	t.Parallel()
	bed := newEnrollBed(t)
	retry := []string{"landing", "engine", "advance"}
	refusalOf(t, requireSelfErr(bed, retry), CodeEnrollmentUnknown)

	running := runningBytes(t)
	bed.enroll(t, digestOf(running), "abc1234", running)
	identity, err := RequireSelf(bed.checkout, bed.installation, retry)
	if err != nil || identity.Running != digestOf(running) || identity.Enrolled.Generation != 3 {
		t.Fatalf("the enrolled executable = %+v %v; want admitted", identity, err)
	}

	other := append([]byte("hand-rebuilt "), running[:64]...)
	bed.enroll(t, digestOf(other), "abc1234", other)
	refusal := refusalOf(t, requireSelfErr(bed, retry), CodeNotEnrolled)
	if !strings.Contains(refusal.Detail, digestOf(running)) || !strings.Contains(refusal.Detail, digestOf(other)) {
		t.Fatalf("detail %q names neither digest", refusal.Detail)
	}
}

func requireSelfErr(bed *enrollBed, retry []string) error {
	_, err := RequireSelf(bed.checkout, bed.installation, retry)
	return err
}

// Line 2 of the refusal is the one way back: the enrolled engine when its
// bytes are intact, the enrolled pin's advance when the install path was
// rebuilt by hand, and a person's system start when neither survives.
func TestRequireSelfNamesTheWayBack(t *testing.T) {
	t.Parallel()
	bed := newEnrollBed(t)
	retry := []string{"landing", "engine", "advance"}
	enrolled := []byte("the enrolled engine")
	id := bed.enroll(t, digestOf(enrolled), "abc1234", enrolled)

	refusal := refusalOf(t, requireSelfErr(bed, retry), CodeNotEnrolled)
	if want := append([]string{bed.installPath}, retry...); !slices.Equal(refusal.Argv, want) {
		t.Fatalf("intact enrolled bytes: argv %q, want %q", refusal.Argv, want)
	}

	if err := os.WriteFile(bed.installPath, []byte("rebuilt by hand"), 0o755); err != nil {
		t.Fatal(err)
	}
	refusal = refusalOf(t, requireSelfErr(bed, retry), CodeNotEnrolled)
	if want := []string{"metasystem", "system", "start", "--repo", bed.checkout}; !slices.Equal(refusal.Argv, want) {
		t.Fatalf("no enrolled bytes anywhere: argv %q, want %q", refusal.Argv, want)
	}
	if !strings.Contains(refusal.Message, "rebuilt") {
		t.Fatalf("message %q does not say the engine was rebuilt", refusal.Message)
	}

	pin := steward.EnrolledExecutionPath(bed.installation, id)
	if err := os.MkdirAll(filepath.Dir(pin), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pin, enrolled, 0o700); err != nil {
		t.Fatal(err)
	}
	refusal = refusalOf(t, requireSelfErr(bed, retry), CodeNotEnrolled)
	if want := []string{pin, "landing", "engine", "advance"}; !slices.Equal(refusal.Argv, want) {
		t.Fatalf("enrolled pin intact: argv %q, want %q", refusal.Argv, want)
	}
}

// A publication needs the proof's policy engine to be the enrolled one; the
// candidate engine (the engine the proven tree builds) is free.
func TestPublishNeedsTheEnrolledPolicyEngine(t *testing.T) {
	t.Parallel()
	hex := strings.Repeat("ab", 32)
	enrolled := steward.InstallIdentity{Generation: 4, InstallDigest: "sha256:" + hex}
	reprove := []string{"metasystem", "landing", "prove", "--batch", "B", "--subject", "batch"}
	for _, policy := range []string{hex, "sha256:" + hex, strings.ToUpper(hex)} {
		if err := RequirePolicyEngine(enrolled, ProofEngines{Policy: policy, Candidate: strings.Repeat("cd", 32)}, reprove); err != nil {
			t.Fatalf("policy %q with a different candidate refused: %v", policy, err)
		}
	}
	for _, policy := range []string{"", strings.Repeat("cd", 32), "sha256:"} {
		refusal := refusalOf(t, RequirePolicyEngine(enrolled, ProofEngines{Policy: policy, Candidate: hex}, reprove), CodePolicyNotEnrolled)
		if !slices.Equal(refusal.Argv, reprove) {
			t.Fatalf("argv %q, want the re-proof %q", refusal.Argv, reprove)
		}
	}
	if err := RequirePolicyEngine(steward.InstallIdentity{}, ProofEngines{Policy: "", Candidate: ""}, reprove); err == nil {
		t.Fatal("an empty enrollment matched an empty policy digest")
	}
}
