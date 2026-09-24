package goal

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type residueGateRepository struct {
	*strictAbsentRepository
	calls    []string
	sentinel error
}

func (r *residueGateRepository) Capture(opid string) (string, error) {
	r.calls = append(r.calls, "capture:"+opid)
	return "", r.sentinel
}

func (r *residueGateRepository) Release(opid string) error {
	r.calls = append(r.calls, "release:"+opid)
	return nil
}

// The R-4 gate runs BEFORE any transaction: a residue-naming conclusion
// without a resolvable goal link refuses at the door.
func TestDoneRefusesUnscheduledResidueInTheConclusion(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	sentinel := errors.New("capture stopped after residue gate")
	repository := &residueGateRepository{strictAbsentRepository: &strictAbsentRepository{t: t}, sentinel: sentinel}
	request := VerbRequest{
		Endpoint: Endpoint{Root: root, Repository: repository},
		Ulid:     "01J5X00000000000000000R400",
		Actor:    Actor{Machine: "mac-a", Lineage: "residue-gate"},
	}
	if request.Endpoint.Repository != repository || request.Endpoint.Root != root {
		t.Fatal("residue repository is not bound to the requested endpoint")
	}
	opid := request.opid()
	assertNoRepositoryCalls := func() {
		t.Helper()
		if len(repository.calls) != 0 || repository.accepted.Load() != 0 {
			t.Fatalf("residue gate touched repository: calls=%v accepted=%d", repository.calls, repository.accepted.Load())
		}
	}
	assertCaptured := func(opids ...string) {
		t.Helper()
		if len(repository.calls) != len(opids)*2 || repository.accepted.Load() != 0 {
			t.Fatalf("repository boundary = %v, accepted=%d", repository.calls, repository.accepted.Load())
		}
		for i, wantOpid := range opids {
			if repository.calls[i*2] != "capture:"+wantOpid || repository.calls[i*2+1] != "release:"+wantOpid {
				t.Fatalf("repository call order = %v, want capture then release for %s", repository.calls, wantOpid)
			}
		}
	}

	_, err := Done(request, "any-goal", "landed X; the cache half is residue for later")
	if err == nil || !strings.Contains(err.Error(), "residue is a scheduled debt") {
		t.Fatalf("unlinked residue conclusion must refuse by name: %v", err)
	}
	assertNoRepositoryCalls()

	_, err = Done(request, "any-goal", "landed X; residual cache work rides goal:ghost-item")
	if err == nil || !strings.Contains(err.Error(), "goal:ghost-item does not resolve") {
		t.Fatalf("a dangling residue link must refuse by id: %v", err)
	}
	assertNoRepositoryCalls()

	if err := os.MkdirAll(filepath.Join(root, "plans", "goals"), 0o755); err != nil {
		t.Fatal(err)
	}
	goalPath := filepath.Join(root, "plans", "goals", "cache-half.md")
	if err := os.WriteFile(goalPath, []byte("# goal\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = Done(request, "any-goal", "landed X; the cache residue rides goal:cache-half")
	if err != sentinel {
		t.Fatalf("linked residue did not reach the capture boundary: %v", err)
	}
	assertCaptured(opid)

	request.Ulid = "01J5X00000000000000000R401"
	secondOpid := request.opid()
	_, err = Done(request, "any-goal", "landed X cleanly; nothing remains")
	if err != sentinel {
		t.Fatalf("residue-free conclusion did not reach the capture boundary: %v", err)
	}
	assertCaptured(opid, secondOpid)
	if content, err := os.ReadFile(goalPath); err != nil || string(content) != "# goal\n" {
		t.Fatalf("resolvable goal file changed: %q, %v", content, err)
	}
	entries, err := os.ReadDir(filepath.Dir(goalPath))
	if err != nil || len(entries) != 1 || entries[0].Name() != "cache-half.md" {
		t.Fatalf("resolvable goal directory changed: %v, %v", entries, err)
	}
}
