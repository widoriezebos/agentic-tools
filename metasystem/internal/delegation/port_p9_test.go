package delegation_test

// U6b port slice p9: the delegation-owned legs of the small beds U6b retires
// (evidence-segment-fixtures.sh leg 1, actionable-metrics-fixtures.sh goal
// plumbing).

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegation"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegation/fake"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

// newCheckoutBed is newBed for an installation root nested in its own
// checkout (<checkout>/metasystem, the layout the retired bed built) whose
// evidence root is shared and outside every checkout. The repository scope
// is the checkout, which is what the job-record mirror derives its evidence
// segment from.
func newCheckoutBed(t *testing.T, evidence string) *bed {
	t.Helper()
	checkout, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(checkout, "metasystem")
	for _, dir := range []string{"artifacts/agents/jobs", "artifacts/agents/record-locks", "artifacts/agents/locks", "artifacts/agents/hb"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	conf := "evidence.root=" + evidence + "\nmetasystem.runtimes=fake\nrole.default.model.fake=fake-model\n"
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte(conf), 0o644); err != nil {
		t.Fatal(err)
	}
	doubles := fake.NewSet()
	doubles.Records.CreateFunc = func(job, source string) error { return dispatch.RecordCreate(root, job, source) }
	doubles.Records.SetupFunc = func(job, source string) error { return dispatch.RecordSetup(root, job, source) }
	doubles.Records.CASFunc = func(job, expect, target, patch string) (string, error) {
		return dispatch.RecordCAS(root, job, expect, target, patch)
	}
	holder := int64(4)
	doubles.Lease.RequireFunc = func(delegation.Invocation, *int64) (lease.HolderView, error) {
		return lease.HolderView{Class: lease.ClassHuman, Holder: true, ClaimEpoch: &holder}, nil
	}
	life, err := delegation.New(delegation.Config{Root: root, RepoScope: checkout, Engine: "/engine/metasystem"}, doubles.Ports())
	if err != nil {
		t.Fatal(err)
	}
	return &bed{t: t, root: root, doubles: doubles, life: life, noHandshake: map[string]bool{}}
}

// evidence-segment-fixtures leg 1: two checkouts of the same chain, each
// reaped through the lease-held entry, mirror into two distinct evidence
// segments, and each segment's manifest names its root job and covers the
// brief, the round's artifacts and the job record.
func TestTwoCheckoutsOfOneChainMirrorIntoDistinctEvidenceSegments(t *testing.T) {
	t.Parallel()
	evidence, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"checkout-a", "checkout-b"} {
		b := newCheckoutBed(t, evidence)
		b.writeFile("artifacts/agents/jobs/segment-chain.json", `{
  "jobId": "segment-chain",
  "parentJob": null,
  "round": 1,
  "status": "completed",
  "runtime": "fake",
  "capabilitySnapshot": "artifacts/agents/capabilities/absent.json",
  "mirror": null
}
`)
		b.writeFile("artifacts/agents/segment-chain/brief.md", "checkout="+name+"\n")
		b.writeFile("artifacts/agents/segment-chain/rounds/1/raw.out", "round evidence\n")
		requireExit(t, b.run("__reap-held", "--job", "segment-chain"), 0, b.stderr.String())
		if calls := b.calls("lease.Authorize"); len(calls) == 0 || !strings.HasSuffix(calls[0], "mode=holder-only job=") {
			t.Fatalf("%s: the held reap did not take holder-only authority: %v", name, calls)
		}
		mirror, ok := b.record("segment-chain")["mirror"].(map[string]any)
		if !ok || !strings.HasPrefix(fmt.Sprint(mirror["path"]), filepath.Join(evidence, "agents")+"/") {
			t.Fatalf("%s: the record was not stamped with its mirror: %v", name, b.record("segment-chain"))
		}
	}

	destinations, err := filepath.Glob(filepath.Join(evidence, "agents", "*", "segment-chain"))
	if err != nil {
		t.Fatal(err)
	}
	if len(destinations) != 2 {
		t.Fatalf("expected two mirrored segments, found %v", destinations)
	}
	if filepath.Base(filepath.Dir(destinations[0])) == filepath.Base(filepath.Dir(destinations[1])) {
		t.Fatalf("both checkouts mirrored into the same segment: %v", destinations)
	}
	briefs := map[string]bool{}
	for _, destination := range destinations {
		content, err := os.ReadFile(filepath.Join(destination, "manifest.json"))
		if err != nil {
			t.Fatal(err)
		}
		var manifest struct {
			RootJob string                     `json:"rootJob"`
			Files   map[string]json.RawMessage `json:"files"`
		}
		if err := json.Unmarshal(content, &manifest); err != nil {
			t.Fatalf("%s: %v", destination, err)
		}
		if manifest.RootJob != "segment-chain" {
			t.Fatalf("%s does not name its root job: %s", destination, content)
		}
		for _, key := range []string{"brief.md", "rounds/1/raw.out", "jobs/segment-chain.json"} {
			if _, ok := manifest.Files[key]; !ok {
				t.Fatalf("%s does not cover %s: %s", destination, key, content)
			}
		}
		brief, err := os.ReadFile(filepath.Join(destination, "brief.md"))
		if err != nil {
			t.Fatal(err)
		}
		briefs[string(brief)] = true
	}
	if !briefs["checkout=checkout-a\n"] || !briefs["checkout=checkout-b\n"] {
		t.Fatalf("each segment must carry its own checkout's brief: %v", briefs)
	}
}

// actionable-metrics O13 (the --goal plumbing the bed grepped out of
// dispatch.sh): a fresh dispatch binds exactly the goal it was given to the
// goal owner's stop authority, and a refused binding stops the dispatch by
// name before any claim or record exists.
func TestDispatchBindsTheGoalItWasGiven(t *testing.T) {
	t.Parallel()
	b := newGoalBed(t)
	var asked []string
	b.doubles.Goal.BindingFunc = func(goalID string) (delegation.GoalBinding, error) {
		asked = append(asked, goalID)
		return delegation.GoalBinding{}, errors.New("goal not accepted in this fixture")
	}
	brief := b.writeFile("brief.md", "Working Mode: implement\n\nDo the thing.\n")
	result := b.run("dispatch", "--role", "implementer", "--brief", brief, "--goal", "goal-alpha", "--destructive-reach", "MECHANICAL", "--job-id", "goal-job")
	requireExit(t, result, 1, b.stderr.String())
	if len(asked) != 1 || asked[0] != "goal-alpha" {
		t.Fatalf("goal binding asked for %v, want exactly goal-alpha", asked)
	}
	if !strings.Contains(b.stderr.String(), "cannot bind delegate operation to accepted goal goal-alpha stop authority") {
		t.Fatalf("stderr %q", b.stderr.String())
	}
	if exists(b.recordPath("goal-job")) {
		t.Fatal("a refused goal binding left a job record")
	}
}

// actionable-metrics O13: a follow-up takes no --goal; it inherits goalId
// from the chain's newest record and binds that goal.
func TestFollowUpInheritsTheGoalOfTheChainsNewestRecord(t *testing.T) {
	t.Parallel()
	b := newGoalBed(t)
	var asked []string
	b.doubles.Goal.BindingFunc = func(goalID string) (delegation.GoalBinding, error) {
		asked = append(asked, goalID)
		return delegation.GoalBinding{}, errors.New("goal not accepted in this fixture")
	}
	b.writeRecord("chain", map[string]any{
		"status": "completed", "round": 1, "parentJob": nil, "sessionId": "s-1",
		"requestedModel": "fake-model", "destructiveReach": "MECHANICAL", "goalId": "goal-root",
	})
	b.writeRecord("chain-r2", map[string]any{
		"status": "completed", "round": 2, "parentJob": "chain", "sessionId": "s-2",
		"requestedModel": "fake-model", "destructiveReach": "MECHANICAL", "goalId": "goal-newest",
	})
	message := b.writeFile("message.md", "follow up\n")
	result := b.run("follow-up", "--job", "chain", "--message", message)
	requireExit(t, result, 1, b.stderr.String())
	if len(asked) != 1 || asked[0] != "goal-newest" {
		t.Fatalf("goal binding asked for %v, want the newest record's goal-newest", asked)
	}
	if !strings.Contains(b.stderr.String(), "cannot bind follow-up chain-r3 to accepted goal goal-newest stop authority") {
		t.Fatalf("stderr %q", b.stderr.String())
	}
	if exists(b.recordPath("chain-r3")) {
		t.Fatal("a refused goal binding left a follow-up record")
	}
}

// newGoalBed is a stub-Git bed a dispatch or follow-up can run through up to
// its goal binding: the implementer role files, a fake-runtime roster, and a
// fresh, fingerprint-matched census over an armed state.
func newGoalBed(t *testing.T) *bed {
	t.Helper()
	b := newBed(t)
	conf := "evidence.root=" + filepath.Join(b.root, "evidence") + "\nmetasystem.runtimes=fake\nrole.default.runtime=fake\nrole.default.model.fake=fake-model\ndispatch.permissions.implementer=none\n"
	b.writeFile("metasystem.conf", conf)
	b.writeFile("scripts/agents/roles/implementer.md", "implementer\n")
	b.writeFile("scripts/agents/roles/implementer.requirements.json", "{}\n")
	for _, input := range []string{"bin/metasystem"} {
		b.writeFile(input, "fixture\n")
	}
	// The census fingerprint reads the rostered adapter's signature.
	b.installAsset("scripts/agents/adapters/fake.sh")
	b.installAsset("scripts/agents/adapters/runtime-common.sh")
	now := b.doubles.Clock.Now().Unix()
	heartbeat := b.writeFile("artifacts/agents/supervision/watcher.heartbeat.json",
		fmt.Sprintf(`{"pid":4242,"pidStartedAt":7,"instanceTag":"watcher","loadedCapMin":900,"observedAtEpoch":%d}`, now))
	state := fmt.Sprintf(`{"generation":1,"intervalSec":60,"components":{"watcher":{"pid":4242,"pidStartedAt":7,"instanceTag":"watcher","heartbeat":%q}}}`, heartbeat)
	b.writeFile("artifacts/agents/supervision/state.json", state)
	sum := sha256.Sum256([]byte(state))
	fingerprint, err := census.Fingerprint(b.root, b.root)
	if err != nil {
		t.Fatalf("census fingerprint: %v", err)
	}
	verdict, _ := json.Marshal(map[string]any{
		"schemaVersion": 2, "writer": "watch-background-jobs.sh", "verdict": "SUCCESS",
		"completedAtEpoch": now, "intervalSec": 60, "fingerprint": fingerprint,
		"counts": map[string]any{}, "inventory": []any{}, "diagnostics": []any{}, "errors": []any{},
		"generation": 1, "stateDigest": hex.EncodeToString(sum[:]),
	})
	b.writeFile("artifacts/agents/supervision/last-census.json", string(verdict))
	return b
}
