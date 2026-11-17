package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegation"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegation/fake"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
)

var remedyClearProducer = flag.String("remedy-clear-producer", "", "compiled health fixture for the command bed")

func TestHealthRemediesAreActsThatClear(t *testing.T) {
	t.Parallel()
	if runHealthRemedyCommandChild(t) {
		return
	}
	b := newProcessBed(t)
	setHealthRemedyMissingBudget(t, b)
	incident := goal.TrunkRedEntry{ID: "main-red", Identity: "main-red", Group: "unit", Status: "failed", Opened: "2026-09-01T09:00:00Z",
		Failures:  []goal.TrunkRedFailure{{Name: "TestMain", Status: "failed", Report: "fixture.xml", Classname: "unit"}},
		Sightings: []goal.TrunkRedSighting{{Attempt: "main-red", BaseCommit: "main", SeenAt: "2026-09-01T09:00:00Z", Opid: goal.Opid("01J5X0000000000000000000F2", "mac-cli", "m1")}}}
	b.repo.commit(b.repo.accepted).files["plans/goals/trunk-red.json"] = goal.RenderTrunkRed([]goal.TrunkRedEntry{incident})
	record := map[string]any{"jobId": "dead-job", "goalId": nil, "status": "pending-setup", "runtime": "fake", "pid": 52001, "pidStartedAt": 1700052001, "pidStartTicks": 520010, "bootId": "health-bed-boot", "pgid": 52001}
	path := filepath.Join(b.root(), "artifacts", "agents", "jobs", "dead-job.json")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(record)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	owners := b.owners()
	owners.processes.cancelDispatch = func(root, job string) (map[string]any, int, error) {
		ports, err := delegation.NewOwnerPorts(delegation.OwnerConfig{Root: root, Engine: filepath.Join(root, "bin", "metasystem"), Host: engineHost{}, Now: func() time.Time { now, _ := b.commandNow(root); return now }})
		if err != nil {
			t.Fatal(err)
		}
		doubles := fake.NewSet()
		doubles.Lease.ClassifyFunc = func(delegation.Invocation) (lease.ClassifyResult, error) {
			return lease.ClassifyResult{Class: lease.ClassHuman}, nil
		}
		doubles.Lease.RequireFunc = func(delegation.Invocation, *int64) (lease.HolderView, error) {
			return lease.HolderView{Class: lease.ClassHuman}, nil
		}
		ports.Lease, ports.Process, ports.Git, ports.Clock = doubles.Lease, doubles.Process, doubles.Git, doubles.Clock
		lifecycle, err := delegation.New(delegation.Config{Root: root, RepoScope: root}, ports)
		if err != nil {
			t.Fatal(err)
		}
		var out, diagnostic bytes.Buffer
		result := lifecycle.Run(context.Background(), delegateLifecycleRequest(delegation.Env{RecordOutcome: true, DelegateInternal: true}, nil, &diagnostic), []string{"cancel", "--job", job})
		code := writeDelegateResult(result, "cancel", []string{"--cancel", job}, diagnostic.String(), &out, &diagnostic)
		var outcome map[string]any
		err = json.Unmarshal(out.Bytes(), &outcome)
		return outcome, code, err
	}
	owners.processes.health = healthRemedyPreview(t, b)
	check := func(want steward.HealthRole, status steward.HealthStatus) []string {
		t.Helper()
		_, result := b.runJSON(owners, "system", "check")
		data, _ := json.Marshal(result.Data)
		var preview struct {
			Verdict        steward.HealthVerdict
			PublicRemedies []struct {
				Role   steward.HealthRole
				Public []string
			}
		}
		if err := json.Unmarshal(data, &preview); err != nil {
			t.Fatal(err)
		}
		found := slices.IndexFunc(preview.Verdict.Roles, func(r steward.RoleVerdict) bool { return r.Role == want })
		if found < 0 {
			t.Fatalf("health role %s missing", want)
		}
		if preview.Verdict.Roles[found].Status != status {
			t.Fatalf("%s want %s: %+v", want, status, preview.Verdict.Roles[found])
		}
		for _, remedy := range preview.PublicRemedies {
			if remedy.Role == want {
				return remedy.Public
			}
		}
		return nil
	}
	for _, row := range []struct {
		role steward.HealthRole
		want []string
	}{
		{steward.RoleNonterminalJobs, []string{"metasystem", "work", "stop", "j2:dead-job"}},
		{steward.RoleClaimedGoalBudget, []string{"metasystem", "goal", "budget", bedGoal, "BOX"}},
		{steward.RoleTrunkRed, []string{"metasystem", "incident", "claim", "main-red", "--goal", "G"}},
	} {
		act := check(row.role, steward.HealthDead)
		if !slices.Equal(act, row.want) {
			t.Fatalf("%s remedy %v want %v", row.role, act, row.want)
		}
		for i, word := range act {
			if word == "BOX" {
				act[i] = "1d/10/720m/1/3"
			}
			if word == "G" {
				act[i] = bedGoal
			}
		}
		if code, result := b.runJSON(owners, act[1:]...); code != 0 {
			t.Fatalf("follow %v: %d %+v", act, code, result)
		}
		check(row.role, steward.HealthAlive)
	}
	if act := check(steward.RoleStewardRunner, steward.HealthDead); !slices.Equal(act, []string{"metasystem", "system", "start"}) {
		t.Fatalf("person's process remedy: %v", act)
	}
	// The table's human commands must exist and admit a person in the real
	// command inventory; the renderer cannot confer an agent-only verb.
	for _, fact := range []steward.RemedyFact{{Cause: steward.CauseForeignLineage, Goal: bedGoal}} {
		act, _ := publicHealthRemedy(steward.RoleVerdict{RemedyFacts: []steward.RemedyFact{fact}}, false, "human")
		command, _, ok := resolveIntentArgv(act[1:])
		if !ok || command.audience == "agent" {
			t.Fatalf("not a person's public act: %v", act)
		}
	}
	fence, err := stopfence.Read(b.root())
	if err != nil || fence.State != stopfence.StateOpen {
		t.Fatalf("remedies changed the process fence: %+v %v", fence, err)
	}
	ended, err := dispatchcore.ReadRecordObject(path)
	if err != nil || ended["status"] != "cancelled" {
		t.Fatalf("job never ended: %v %v", ended, err)
	}
}

func runHealthRemedyCommandChild(t *testing.T) bool {
	t.Helper()
	if *remedyClearProducer == "" {
		goBin, err := exec.LookPath("go")
		if err != nil {
			t.Fatal(err)
		}
		producer := filepath.Join(t.TempDir(), "steward.test")
		build := exec.Command(goBin, "test", "-c", "-timeout", "30m", "-o", producer, "./internal/steward")
		build.Dir = "../.."
		if out, err := build.CombinedOutput(); err != nil {
			t.Fatalf("build health bed: %v\n%s", err, out)
		}
		self, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		child := exec.Command(self, "-test.run=^"+t.Name()+"$", "-test.timeout=30m", "-remedy-clear-producer="+producer)
		child.Env = append(os.Environ(), "PATH="+t.TempDir())
		if out, err := child.CombinedOutput(); err != nil {
			t.Fatalf("Git-free command bed: %v\n%s", err, out)
		}
		return true
	}
	return false
}

func setHealthRemedyMissingBudget(t *testing.T, b *processBed) {
	t.Helper()
	// A missing budget is a parseable claimed goal, not a fabricated verdict.
	file, _ := b.acceptedGoal()
	file.Budget, file.Approved = nil, nil
	file.Claimed.At = "2026-09-01T09:55:00Z"
	for i := range file.History {
		if file.History[i].Verb == "claim" {
			file.History[i].At = file.Claimed.At
		}
	}
	file.StopCapability = &goal.StopCapability{Generation: 1, Revision: file.Claimed.Revision, Machine: file.Claimed.Machine, ClaimEpoch: 1, FenceEpoch: 1}
	b.addGoal(file)
}

func healthRemedyPreview(t *testing.T, b *processBed) func(string, string, time.Time) steward.HealthVerdict {
	t.Helper()
	return func(root, installation string, now time.Time) steward.HealthVerdict {
		accepted := b.repo.commit(b.repo.accepted).files
		ledger := filepath.Join(t.TempDir(), "ledger.json")
		data, err := json.Marshal(accepted)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(ledger, data, 0600); err != nil {
			t.Fatal(err)
		}
		output := filepath.Join(t.TempDir(), "verdict.json")
		command := exec.Command(*remedyClearProducer, "-test.run=^TestHealthRemedyTableNamesNoRead$", "-test.timeout=30m", "-remedy-bed-root="+root, "-remedy-bed-ledger="+ledger, "-remedy-bed-output="+output)
		if out, err := command.CombinedOutput(); err != nil {
			t.Fatalf("health preview: %v\n%s", err, out)
		}
		data, err = os.ReadFile(output)
		if err != nil {
			t.Fatal(err)
		}
		var verdict steward.HealthVerdict
		if err := json.Unmarshal(data, &verdict); err != nil {
			t.Fatal(err)
		}
		return verdict
	}
}
