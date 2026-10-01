package main

// The enrolled policy engine of a fixture checkout, and the session lease
// it holds: the command application beds prove through a real engine build
// enrolled for their checkout, as a lane or seat installation is.

import (
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginebuild"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// fixturePolicyEngine is one engine build at path, stamped with commit once
// built; beds that share it build it once.
type fixturePolicyEngine struct {
	sync.Mutex
	path, commit string
}

// enrollFixturePolicyEngine builds shared at commit when it is not built
// yet and enrolls it as root's engine, minted at now.
func enrollFixturePolicyEngine(t *testing.T, root string, now time.Time, commit string, shared *fixturePolicyEngine) {
	t.Helper()
	shared.Lock()
	defer shared.Unlock()
	if shared.commit != "" && shared.commit != commit {
		t.Fatalf("shared fixture engine commit=%s want %s", shared.commit, commit)
	}
	if shared.commit == "" {
		shared.commit = commit
		buildFixturePolicyEngine(t, commit, shared.path)
	}
	canonicalRoot, err := canonicalProofRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	canonicalEngine, err := canonicalPath(shared.path)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := fileSHA256(canonicalEngine)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(steward.RepoIdentityPath(canonicalRoot)), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := steward.MintIdentity(steward.RepoIdentityPath(canonicalRoot), steward.InstallIdentity{
		RepoIdentity: canonicalRoot, Generation: 1, InstallPath: canonicalEngine,
		InstallDigest: "sha256:" + digest, MintedAt: now.Format(time.RFC3339),
		Enrollment: steward.EnrollmentFixture, EngineBuild: commit,
	}); err != nil {
		t.Fatal(err)
	}
}

// buildFixturePolicyEngine builds this source's engine at engine, stamped
// with commit: the production build.
func buildFixturePolicyEngine(t *testing.T, commit, engine string) {
	t.Helper()
	sourceRoot := testutil.MustSourceRoot(t)
	command := exec.Command("go", "build", "-buildvcs=false", "-ldflags", enginebuild.StampLinkerFlags(commit), "-o", engine, "./cmd/metasystem")
	command.Dir = sourceRoot
	command.Env = os.Environ()
	if output, buildErr := command.CombinedOutput(); buildErr != nil {
		t.Fatalf("build enrolled fixture engine: %v: %s", buildErr, output)
	}
}

// announceFixtureHolder makes this process root's lease holder under
// lineage.
func announceFixtureHolder(t *testing.T, root, lineage string) {
	t.Helper()
	exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("probe fixture process: state=%s err=%v", state, err)
	}
	if _, err := lease.AnnounceWithPair(root, "session-"+lineage, int64(os.Getpid()), exact.StartedAt.Unix(), exact.StartTicks, exact.BootID, "fixture-holder", "metasystem", lineage); err != nil {
		t.Fatal(err)
	}
}
