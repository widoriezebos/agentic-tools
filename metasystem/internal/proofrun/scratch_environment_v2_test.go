package proofrun

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// Under v2 each group's managed values live in its lease; two overlapping
// runs differ only by slot, their environment identities are equal, and the
// descriptor validates. v1 keeps the per-run layout.
func TestScratchEnvironmentV2PlacesGroupsInLeases(t *testing.T) {
	t.Parallel()
	fixture := newScratchEnvFixture(t)
	groups := []testpolicy.Group{{ID: "go-inherit", Adapter: "go", Env: map[string]string{"PATH": "/usr/bin:/bin"}}}
	base := append(append([]string(nil), fixture.base...), "GOCACHE=/machine/go-build", "STATICCHECK_CACHE=/machine/staticcheck")
	prepare := func(policy string) (TestRunRequest, *ScratchRun) {
		request := scratchEnvRequest(base, groups)
		run, err := CreateScratchRun(fixture.control)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = run.Cleanup(nil) })
		if err := PrepareScratchEnvironmentFor(&request, run, policy); err != nil {
			t.Fatal(err)
		}
		return request, run
	}
	first, firstRun := prepare(ScratchEnvironmentPolicyV2)
	second, secondRun := prepare(ScratchEnvironmentPolicyV2)
	for _, pair := range []struct {
		request TestRunRequest
		run     *ScratchRun
	}{{first, firstRun}, {second, secondRun}} {
		if err := ValidateScratchEnvironment(pair.request, pair.run); err != nil {
			t.Fatal(err)
		}
		values := pair.request.ScratchEnvironment.managedValues("go-inherit")
		lease := pair.request.ScratchEnvironment.leaseOf("go-inherit")
		if lease == "" || !strings.HasPrefix(values["TMPDIR"], lease+string(filepath.Separator)) || !strings.HasPrefix(values["HOME"], lease+string(filepath.Separator)) ||
			values["GOCACHE"] != "/machine/go-build" || values["STATICCHECK_CACHE"] != "/machine/staticcheck" {
			t.Fatalf("v2 managed values = %v lease=%q", values, lease)
		}
	}
	if first.ScratchEnvironment.leaseOf("go-inherit") == second.ScratchEnvironment.leaseOf("go-inherit") {
		t.Fatal("overlapping runs share one slot")
	}
	if a, b := scratchEnvDigests(first), scratchEnvDigests(second); a["go-inherit"] != b["go-inherit"] {
		t.Fatalf("v2 identities differ across slots: %v %v", a, b)
	}
	legacy, _ := prepare(ScratchEnvironmentPolicyV1)
	if legacy.ScratchEnvironment.Policy != ScratchEnvironmentPolicyV1 || legacy.ScratchEnvironment.leaseOf("go-inherit") != "" ||
		!strings.HasPrefix(legacy.ScratchEnvironment.managedValues("go-inherit")["TMPDIR"], legacy.ScratchEnvironment.Root) {
		t.Fatalf("v1 layout changed: %+v", legacy.ScratchEnvironment)
	}
	forged := *first.ScratchEnvironment
	forged.Groups = append([]ScratchEnvironmentGroup(nil), forged.Groups...)
	forged.Groups[0].Lease = second.ScratchEnvironment.leaseOf("go-inherit")
	first.ScratchEnvironment = &forged
	if err := ValidateScratchEnvironment(first, firstRun); err == nil {
		t.Fatal("a descriptor naming another run's lease validated")
	}
}
