package dispatch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
)

func TestClaimLaunchRefusesClosedProcessCreationFence(t *testing.T) {
	root := t.TempDir()
	changedAt := "2026-09-07T10:00:00Z"
	if err := stopfence.Write(root, stopfence.Record{
		State: stopfence.StateClosed, Phase: stopfence.PhaseStopped,
		Generation: 7, ChangedAt: changedAt, Checkout: "/fixture/checkout",
		By: stopfence.Actor{Verb: "stop", Process: stopfence.Process{Pid: 71}},
	}); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 7, 10, 1, 0, 0, time.UTC)
	params := claimParamsForTest(root, "stopped-job")
	result, err := ClaimLaunch(params, claimDependenciesForTest(&now, identity.Verification{}))
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != ClaimRefusedStopped || result.Evidence["refusalClass"] != "stopped" || !looseEqual(result.Evidence["fenceGeneration"], 7) {
		t.Fatalf("closed-fence result = %+v", result)
	}
	wantDetail := "the metasystem is stopped for /fixture/checkout since " + changedAt + ", by stop pid 71\nat an agent-free terminal, run: metasystem arm --repo /fixture/checkout"
	if result.Detail != wantDetail {
		t.Fatalf("detail = %q, want %q", result.Detail, wantDetail)
	}
	if _, err := os.Stat(filepath.Join(root, "artifacts", "agents", "jobs", "stopped-job.json")); !os.IsNotExist(err) {
		t.Fatalf("closed-fence claim published a job record: %v", err)
	}
}

func TestFenceBeforeLaunchReportsStoppedBeforeSupervisionAdmission(t *testing.T) {
	root := t.TempDir()
	changedAt := "2026-09-07T10:00:00Z"
	if err := stopfence.Write(root, stopfence.Record{
		State: stopfence.StateClosed, Phase: stopfence.PhaseStopped,
		Generation: 7, ChangedAt: changedAt, Checkout: "/fixture/checkout",
		By: stopfence.Actor{Verb: "stop", Process: stopfence.Process{Pid: 71}},
	}); err != nil {
		t.Fatal(err)
	}
	result, err := FenceBeforeLaunch(root)
	if err != nil {
		t.Fatal(err)
	}
	wantDetail := "the metasystem is stopped for /fixture/checkout since " + changedAt + ", by stop pid 71\nat an agent-free terminal, run: metasystem arm --repo /fixture/checkout"
	if result.Outcome != LaunchFenceRefusedStopped || result.ObservedGeneration != 7 || result.Detail != wantDetail {
		t.Fatalf("fence-before-launch result = %+v, want stopped generation seven and detail %q", result, wantDetail)
	}
}

func TestFenceBeforeLaunchLeavesOpenCheckoutForLaterAdmission(t *testing.T) {
	result, err := FenceBeforeLaunch(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != LaunchFenceOpen || result.ObservedGeneration != 0 || result.Detail != "" {
		t.Fatalf("fence-before-launch result = %+v, want open generation zero", result)
	}
}

func TestDispatchChecksTheFenceBeforeCensusAdmission(t *testing.T) {
	data, err := os.ReadFile("../../scripts/agents/dispatch.sh")
	if err != nil {
		t.Fatal(err)
	}
	script := string(data)
	for _, test := range []struct {
		name, begin, end, after string
	}{
		{name: "fresh dispatch", begin: "dispatch_job() {", end: "\nauthorize_job_cap() {", after: "job claim-launch --preflight"},
		{name: "follow-up", begin: "follow_up() {", end: "\ncancel_job() {", after: "report_plan_drift"},
	} {
		t.Run(test.name, func(t *testing.T) {
			start := strings.Index(script, test.begin)
			end := strings.Index(script, test.end)
			if start < 0 || end <= start {
				t.Fatalf("%s section is absent", test.name)
			}
			section := script[start:end]
			fence := strings.Index(section, "require_open_dispatch_fence")
			census := strings.Index(section, "require_fresh_census")
			after := strings.Index(section, test.after)
			if fence < 0 || census < 0 || after < 0 || !(fence < census && census < after) {
				t.Fatalf("%s order is fence=%d census=%d next=%d", test.name, fence, census, after)
			}
		})
	}
}

func TestClaimLaunchPublishesGenerationAndCreationClaim(t *testing.T) {
	root := t.TempDir()
	if err := stopfence.Write(root, stopfence.Record{
		State: stopfence.StateOpen, Phase: stopfence.PhaseArmed,
		Generation: 4, ChangedAt: "2026-09-07T10:00:00Z", Checkout: "/fixture/checkout",
	}); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 7, 10, 1, 0, 0, time.UTC)
	params := claimParamsForTest(root, "fenced-job")
	result, err := ClaimLaunch(params, claimDependenciesForTest(&now, identity.Verification{}))
	if err != nil || result.Outcome != ClaimWON {
		t.Fatalf("claim = %+v, %v", result, err)
	}
	path, _ := result.Evidence["creationClaimPath"].(string)
	if path == "" || !looseEqual(result.Evidence["fenceGeneration"], 4) {
		t.Fatalf("claim evidence = %+v", result.Evidence)
	}
	record := readRecord(t, root, params.OpID)
	if !looseEqual(record["fenceGeneration"], 4) {
		t.Fatalf("job fenceGeneration = %v", record["fenceGeneration"])
	}
	claims, err := stopfence.Claims(root, 4)
	if err != nil || len(claims) != 1 || claims[0].Path != path || claims[0].Verb != "delegate" {
		t.Fatalf("creation claims = %+v, %v", claims, err)
	}
	if err := stopfence.CloseClaim(path); err != nil {
		t.Fatal(err)
	}
}

func TestFenceAfterLaunchDetectsStopRace(t *testing.T) {
	root := t.TempDir()
	changedAt := "2026-09-07T10:02:00Z"
	writeJSONFile(t, filepath.Join(root, "artifacts", "agents", "jobs"), "raced-job.json", map[string]any{
		"jobId": "raced-job", "status": "pending", "fenceGeneration": 3,
	})
	if err := stopfence.Write(root, stopfence.Record{
		State: stopfence.StateClosed, Phase: stopfence.PhaseStopping,
		Generation: 4, ChangedAt: changedAt, Checkout: "/fixture/checkout",
	}); err != nil {
		t.Fatal(err)
	}
	result, err := FenceAfterLaunch(root, "raced-job")
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != LaunchFenceRefusedStopped || result.FenceGeneration != 3 || result.ObservedGeneration != 4 {
		t.Fatalf("post-launch result = %+v", result)
	}
	if !strings.Contains(result.Detail, "stop unfinished for /fixture/checkout") || !strings.Contains(result.Detail, "while job raced-job started, it has been ended") || !strings.Contains(result.Detail, "metasystem stop --repo /fixture/checkout") {
		t.Fatalf("post-launch detail = %q", result.Detail)
	}
}

func TestFenceAfterLaunchAcceptsMatchingOpenGeneration(t *testing.T) {
	root := t.TempDir()
	writeJSONFile(t, filepath.Join(root, "artifacts", "agents", "jobs"), "open-job.json", map[string]any{
		"jobId": "open-job", "status": "pending", "fenceGeneration": 3,
	})
	if err := stopfence.Write(root, stopfence.Record{
		State: stopfence.StateOpen, Phase: stopfence.PhaseArmed, Generation: 3,
	}); err != nil {
		t.Fatal(err)
	}
	result, err := FenceAfterLaunch(root, "open-job")
	if err != nil || result.Outcome != LaunchFenceOpen {
		t.Fatalf("post-launch result = %+v, %v", result, err)
	}
}

func TestFenceAfterLaunchReportsStopAndArmAsRetryable(t *testing.T) {
	root := t.TempDir()
	writeJSONFile(t, filepath.Join(root, "artifacts", "agents", "jobs"), "rearmed-job.json", map[string]any{
		"jobId": "rearmed-job", "status": "pending", "fenceGeneration": 3,
	})
	if err := stopfence.Write(root, stopfence.Record{
		State: stopfence.StateOpen, Phase: stopfence.PhaseArmed, Generation: 5, Checkout: "/fixture/checkout",
	}); err != nil {
		t.Fatal(err)
	}
	result, err := FenceAfterLaunch(root, "rearmed-job")
	if err != nil || result.Outcome != LaunchFenceRefusedStopped || !strings.Contains(result.Detail, "checkout /fixture/checkout was stopped and armed again") || !strings.Contains(result.Detail, "caller may retry") || strings.Contains(result.Detail, "stop incomplete") {
		t.Fatalf("rearmed fence result = %+v, %v", result, err)
	}
}
