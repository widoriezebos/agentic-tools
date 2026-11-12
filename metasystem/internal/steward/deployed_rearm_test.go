package steward

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginebuild"
)

func TestRearmDeployedEngineAdmitsOnlyMatchingSource(t *testing.T) {
	t.Parallel()
	for _, row := range []string{"matching", "legacy", "no-source", "wrong-digest", "wrong-commit", "unreadable"} {
		t.Run(row, func(t *testing.T) {
			t.Parallel()
			root := canonicalPath(t.TempDir())
			binary := filepath.Join(root, "bin", "metasystem")
			old, source, head := rearmID(1), rearmID(2), rearmID(3)
			writeRearmFile(t, binary, "old bytes")
			if err := os.Chmod(binary, 0o755); err != nil {
				t.Fatal(err)
			}
			digest, err := installDigest(binary)
			if err != nil {
				t.Fatal(err)
			}
			clock := newManualRearmClock()
			mintedBy := "human-terminal"
			if row == "legacy" {
				mintedBy = ""
			}
			if err := MintIdentity(RepoIdentityPath(root), InstallIdentity{RepoIdentity: root, InstallPath: binary, InstallDigest: digest, Generation: 4, MintedAt: clock.Now().Format(time.RFC3339), MintedBy: mintedBy, EngineBuild: old, HumanWitnessedGeneration: 4, HumanWitnessedAt: clock.Now().Format(time.RFC3339)}); err != nil {
				t.Fatal(err)
			}
			// The fixture uses the same linked stamp record as real engine artifacts.
			writeRearmFile(t, binary, enginebuild.StampRecord(source))
			digest, err = installDigest(binary)
			if err != nil {
				t.Fatal(err)
			}
			expected := append(append(rearmOwnedRef(root, "refs/remotes/origin/main", head), rearmBuild(root, source, source, 0)), rearmAncestor(root, source, "refs/remotes/origin/main", 0))
			if row != "matching" && row != "legacy" && row != "unreadable" {
				expected = append(expected, rearmExpected(root, head, nil, "rev-parse", "--verify", "HEAD^{commit}"))
			}
			deps := rearmTestDeps(t, expected...)
			deps.clock = clock
			deps.prober = seatBusyDeadProber{}
			launches := 0
			deps.launch = func(string, *EnrolledBinary, string) (RunnerRecord, error) {
				launches++
				return RunnerRecord{Pid: 99}, nil
			}
			deps.deployed = func() (DeployedEngine, bool, error) {
				d := DeployedEngine{Commit: source, Digest: digest}
				if row == "wrong-digest" {
					d.Digest = "sha256:" + strings.Repeat("0", 64)
				}
				if row == "wrong-commit" {
					d.Commit = old
				}
				if row == "unreadable" {
					return DeployedEngine{}, false, errors.New("deployment unreadable")
				}
				return d, row != "no-source", nil
			}
			if err := NoteDeferredRearm(root, source, old, clock.Now()); err != nil {
				t.Fatal(err)
			}
			out, err := reArmRebuiltEngineWithDeps(deps, root, root, binary)
			enrolled, readErr := VerifyIdentity(RepoIdentityPath(root), root)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if row == "matching" || row == "legacy" {
				witnessed := 4
				if row == "legacy" {
					witnessed = 0
				}
				if err != nil || out.Generation != 5 || enrolled.Generation != 5 || enrolled.MintedBy != "deployed-engine" || enrolled.HumanWitnessedGeneration != witnessed || (row == "legacy" && enrolled.HumanWitnessedAt != "") || enrolled.LandedCommit != source || RearmDeferredLine(root) != "" || launches != 1 {
					t.Fatalf("out=%+v identity=%+v error=%v launches=%d", out, enrolled, err, launches)
				}
			} else {
				want := ErrRearmDeferred
				if row == "unreadable" {
					want = ErrEnrollmentDrift
				}
				if !errors.Is(err, want) || enrolled.Generation != 4 || launches != 0 {
					t.Fatalf("out=%+v identity=%+v error=%v launches=%d", out, enrolled, err, launches)
				}
			}
		})
	}
}

func TestDeployedEnrollmentProvenanceAndDispatch(t *testing.T) {
	t.Parallel()
	root, it := authorizationBed(t)
	it.HumanWitnessedGeneration, it.HumanWitnessedAt = 1, "2026-11-11T11:00:00Z"
	consumedIntentOnDisk(t, root, it)
	id, err := VerifyIdentity(RepoIdentityPath(root), root)
	if err != nil {
		t.Fatal(err)
	}
	id.Generation, id.MintedBy = 2, "deployed-engine"
	id.HumanWitnessedGeneration, id.HumanWitnessedAt = it.HumanWitnessedGeneration, it.HumanWitnessedAt
	id.EngineBuild, id.LandedCommit, id.LandingRef = rearmID(2), rearmID(2), "refs/remotes/origin/main"
	if err := MintIdentity(RepoIdentityPath(root), id); err != nil {
		t.Fatal(err)
	}
	if _, err := AuthorizeDispatch(root, it.Nonce); err != nil {
		t.Fatalf("deployed re-arm lost authorization: %v", err)
	}
	want := "enrollment generation 2 machine-minted (deployed engine, engine " + rearmID(2) + ", landed " + rearmID(2)[:7] + " on refs/remotes/origin/main) above human-witnessed generation 1 of " + it.HumanWitnessedAt
	if got := EnrollmentProvenance(id); got != want {
		t.Fatalf("provenance=%q want=%q", got, want)
	}
	// A terminal mint ends this approval even if the carried witness is unchanged.
	id.Generation, id.MintedBy = 3, "human-terminal"
	if err := MintIdentity(RepoIdentityPath(root), id); err != nil {
		t.Fatal(err)
	}
	if _, err := AuthorizeDispatch(root, it.Nonce); err == nil || !strings.Contains(err.Error(), "reinstalled since this was approved") {
		t.Fatalf("terminal mint authorized old intent: %v", err)
	}
}

func TestDeployedEnrollmentRequiresLandedProvenance(t *testing.T) {
	t.Parallel()
	root := canonicalPath(t.TempDir())
	source := rearmID(2)
	binary := filepath.Join(root, "bin", "metasystem")
	writeRearmFile(t, binary, enginebuild.StampRecord(source))
	if err := os.Chmod(binary, 0755); err != nil {
		t.Fatal(err)
	}
	digest, err := installDigest(binary)
	if err != nil {
		t.Fatal(err)
	}
	if err := MintIdentity(RepoIdentityPath(root), InstallIdentity{RepoIdentity: root, InstallPath: binary, InstallDigest: digest, Generation: 2, MintedAt: "2026-11-11T11:00:00Z", MintedBy: "deployed-engine"}); err != nil {
		t.Fatal(err)
	}
	pinned, err := OpenEnrolledBinary(root)
	if err != nil {
		t.Fatal(err)
	}
	defer pinned.Close()
	deps := rearmTestDeps(t, rearmBuild(root, source, source, 0), rearmAncestor(root, source, source, 0))
	if err := pinned.verifySourceAtDestinationWithDeps(deps, newManualRearmClock(), root, source); err == nil || !strings.Contains(err.Error(), "missing its landed source or landing ref") {
		t.Fatalf("incomplete deployed provenance: %v", err)
	}
}
