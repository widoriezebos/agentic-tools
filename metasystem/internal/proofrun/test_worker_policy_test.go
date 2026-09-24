package proofrun

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func applyCurrentWorkerPolicy(result *TestResult) {
	admissionMaximum := 0
	result.WorkerPolicyVersion = TestWorkerPolicyVersion
	result.Workers = 1
	result.AdmissionMaximum = &admissionMaximum
}

func proofWorkerPointer(value int) *int { return &value }

func TestWorkerAllowanceResolutionAndReservedEnvironment(t *testing.T) {
	t.Parallel()
	request := TestRunRequest{Workers: 6, Environment: []string{"PATH=/bin", TestWorkersEnvironment + "=999"}}
	for _, specimen := range []struct {
		name    string
		adapter string
		share   *int
		want    int
	}{
		{name: "omitted-command", adapter: "command", want: 1},
		{name: "whole-command", adapter: "command", share: proofWorkerPointer(0), want: 6},
		{name: "declared-section", adapter: "section", share: proofWorkerPointer(4), want: 4},
		{name: "performance-whole-attempt", adapter: "command", share: proofWorkerPointer(2), want: 6},
		{name: "go-partition", adapter: "go", want: 1},
	} {
		t.Run(specimen.name, func(t *testing.T) {
			t.Parallel()
			group := testpolicy.Group{ID: specimen.name, Adapter: specimen.adapter,
				Resources: testpolicy.GroupResources{Workers: specimen.share}, Env: map[string]string{TestWorkersEnvironment: "888"}}
			if specimen.name == "performance-whole-attempt" {
				group.Kind = "performance"
			}
			workers, err := EffectiveGroupWorkers(request, group)
			if err != nil || workers != specimen.want {
				t.Fatalf("workers=%d want=%d err=%v", workers, specimen.want, err)
			}
			environment := groupTestEnvironment(request, group)
			found := 0
			for _, entry := range environment {
				if strings.HasPrefix(entry, TestWorkersEnvironment+"=") {
					found++
					if entry != TestWorkersEnvironment+"="+strconv.Itoa(specimen.want) {
						t.Fatalf("reserved environment=%q", entry)
					}
				}
			}
			if found != 1 {
				t.Fatalf("reserved environment entries=%d in %v", found, environment)
			}
		})
	}
	if got := EffectiveTestWorkers(TestRunRequest{}); got != 1 {
		t.Fatalf("legacy zero request workers=%d", got)
	}
	tooLarge := testpolicy.Group{ID: "too-large", Adapter: "command", Resources: testpolicy.GroupResources{Workers: proofWorkerPointer(7)}}
	if _, err := EffectiveGroupWorkers(request, tooLarge); err == nil || !strings.Contains(err.Error(), "requests 7 workers") {
		t.Fatalf("oversized share error=%v", err)
	}
}

func TestWorkerPolicyChangesPreparedIdentityAndRejectsMismatch(t *testing.T) {
	t.Parallel()
	group := testpolicy.Group{ID: "command", Adapter: "command", Resources: testpolicy.GroupResources{Workers: proofWorkerPointer(0)}}
	request := TestRunRequest{Workers: 2}
	first := groupExecutionIdentity(request, group, ".", "inputs", "environment", nil, nil)
	request.Workers = 3
	second := groupExecutionIdentity(request, group, ".", "inputs", "environment", nil, nil)
	group.Resources.Workers = proofWorkerPointer(2)
	third := groupExecutionIdentity(request, group, ".", "inputs", "environment", nil, nil)
	if first == second || second == third || first == third {
		t.Fatalf("worker policy did not partition execution identity: %s %s %s", first, second, third)
	}
	request.Contract.Groups = []testpolicy.Group{group}
	request.Plan.SelectedGroups = []string{group.ID}
	request.PreparedGroups = map[string]PreparedGroupExecution{group.ID: {WorkerPolicyVersion: TestWorkerPolicyVersion, Workers: 1}}
	if err := ValidateTestWorkerRequest(request); err == nil || !strings.Contains(err.Error(), "prepared worker policy differs") {
		t.Fatalf("prepared mismatch error=%v", err)
	}
	performance := group
	performance.Kind = "performance"
	performance.Resources.Workers = proofWorkerPointer(2)
	performanceTwo := groupExecutionIdentity(request, performance, ".", "inputs", "environment", nil, nil)
	performance.Resources.Workers = proofWorkerPointer(5)
	performanceFive := groupExecutionIdentity(request, performance, ".", "inputs", "environment", nil, nil)
	workers, workerErr := EffectiveGroupWorkers(request, performance)
	if workerErr != nil || workers != request.Workers || performanceTwo == performanceFive {
		t.Fatalf("performance allowance or protected declaration identity diverged: workers=%d identities=%s/%s err=%v",
			workers, performanceTwo, performanceFive, workerErr)
	}
}

func TestNewTestResultExposesWorkerPolicyAndUnlimitedAdmission(t *testing.T) {
	t.Parallel()
	result := NewTestResult(TestRunRequest{Workers: 9, AdmissionMaximum: 0})
	if result.WorkerPolicyVersion != TestWorkerPolicyVersion || result.Workers != 9 || result.AdmissionMaximum == nil || *result.AdmissionMaximum != 0 {
		t.Fatalf("worker result=%+v", result)
	}
	encoded, err := json.Marshal(result)
	if err != nil || !strings.Contains(string(encoded), `"admissionMaximum":0`) {
		t.Fatalf("unlimited admission was not reported: %s err=%v", encoded, err)
	}
}

func TestCommandJUnitConsumerUsesAllowanceRetainsRedAndCancels(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	script := `#!/bin/sh
set -eu
mode=$1
report=$2
: "${METASYSTEM_TEST_WORKERS:?}"
: "${WORKER_STATE:?}"
: "${WORKER_INTERNAL_READY_FIFO:?}"
: "${WORKER_POOL_READY_FIFO:?}"
i=0
pids=
while [ "$i" -lt "$METASYSTEM_TEST_WORKERS" ]; do
  i=$((i+1))
  (
    printf x >"$WORKER_INTERNAL_READY_FIFO"
    IFS= read -r released <"$WORKER_STATE/release-$i.fifo"
  ) </dev/null >/dev/null 2>&1 &
  child=$!
  printf '%s\n' "$child" >"$WORKER_STATE/pid-$i"
  pids="$pids $child"
done
dd if="$WORKER_INTERNAL_READY_FIFO" bs=1 count="$METASYSTEM_TEST_WORKERS" of=/dev/null 2>/dev/null
maximum=0
for pid in $pids; do
  kill -0 "$pid" 2>/dev/null && maximum=$((maximum+1))
done
printf '%s' "$maximum" >"$WORKER_STATE/maximum"
if [ "$mode" = cancel ]; then
  printf x >"$WORKER_POOL_READY_FIFO"
else
  i=1
  while [ "$i" -le "$METASYSTEM_TEST_WORKERS" ]; do
    printf 'release\n' >"$WORKER_STATE/release-$i.fifo"
    i=$((i+1))
  done
fi
for pid in $pids; do wait "$pid"; done
mkdir -p "$report"
if [ "$mode" = red ]; then
  printf '<testsuite><testcase classname="portable" name="allowance"><failure message="retained red"/></testcase></testsuite>\n' >"$report/result.xml"
  exit 9
fi
printf '<testsuite><testcase classname="portable" name="allowance"/></testsuite>\n' >"$report/result.xml"
`
	tree := strings.Repeat("1", 40)
	snapshot := newTestSnapshotFactory(t, root, tree, map[string]testSnapshotEntry{
		"runner.sh": testSnapshotFile(script, 0o755),
	}, 4)
	whole := 0
	run := func(t *testing.T, mode string, workers int, cancel bool) GroupResult {
		t.Helper()
		state := t.TempDir()
		internalReady := filepath.Join(state, "internal-ready.fifo")
		poolReady := filepath.Join(state, "pool-ready.fifo")
		for _, path := range []string{internalReady, poolReady} {
			if err := syscall.Mkfifo(path, 0o600); err != nil {
				t.Fatal(err)
			}
		}
		for index := 1; index <= workers; index++ {
			if err := syscall.Mkfifo(filepath.Join(state, "release-"+strconv.Itoa(index)+".fifo"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		internalReadyOwner, err := os.OpenFile(internalReady, os.O_RDWR, 0o600)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := internalReadyOwner.Close(); err != nil {
				t.Errorf("close internal readiness FIFO: %v", err)
			}
		}()
		group := testpolicy.Group{ID: "portable-" + mode, Kind: "component", Adapter: "command", CWD: ".",
			Inputs: []string{"runner.sh"}, Outputs: []string{"reports"}, Platforms: []string{"any"}, TargetMS: 1000,
			Resources: testpolicy.GroupResources{Workers: &whole}, Argv: []string{"sh", "runner.sh", mode, "reports"},
			Reports: []string{"reports"}, Format: "junit-xml",
			ExpectedTests: []testpolicy.ExpectedTest{{Report: "reports/result.xml", Classname: "portable", Name: "allowance"}}}
		baseEnvironment := dropTestEnvironmentName(gittree.ScrubbedEnviron(), identity.FixtureCustodianEnv)
		request := TestRunRequest{ProjectRoot: root, CandidateTree: tree, openCandidate: snapshot.open, Workers: workers,
			Environment: append(baseEnvironment, "WORKER_STATE="+state,
				"WORKER_INTERNAL_READY_FIFO="+internalReady, "WORKER_POOL_READY_FIFO="+poolReady),
			LogRoot: filepath.Join(root, "logs", mode)}
		var observed GroupResult
		var children []identity.Ref
		if !cancel {
			observed = runTestGroup(context.Background(), request, group)
		} else {
			ctx, stop := context.WithCancel(context.Background())
			result := make(chan GroupResult, 1)
			go func() { result <- runTestGroup(ctx, request, group) }()
			pipe, err := os.Open(poolReady)
			if err != nil {
				t.Fatal(err)
			}
			one := make([]byte, 1)
			_, readErr := io.ReadFull(pipe, one)
			closeErr := pipe.Close()
			if readErr != nil || closeErr != nil || string(one) != "x" {
				t.Fatalf("cancellation readiness=%q read=%v close=%v", one, readErr, closeErr)
			}
			for index := 1; index <= workers; index++ {
				data, err := os.ReadFile(filepath.Join(state, "pid-"+strconv.Itoa(index)))
				pid, parseErr := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
				exact, live, probeErr := (identity.KernelProber{}).Probe(pid)
				if err != nil || parseErr != nil || probeErr != nil || live != identity.Alive || !exact.Ref().NativeExact() {
					t.Fatalf("worker %d identity data=%q read=%v parse=%v probe=%v live=%s exact=%+v", index, data, err, parseErr, probeErr, live, exact)
				}
				children = append(children, exact.Ref())
			}
			stop()
			observed = <-result
			for _, child := range children {
				exact, live, err := (identity.KernelProber{}).Probe(child.Pid)
				released := err == nil && (live == identity.Dead ||
					(live == identity.Alive && (!identity.SameIdentity(exact, child) || exact.Zombie)))
				if !released {
					t.Fatalf("cancelled exact worker remains executable after consumer completion: ref=%+v exact=%+v live=%s err=%v", child, exact, live, err)
				}
			}
		}
		maximum, err := os.ReadFile(filepath.Join(state, "maximum"))
		if err != nil || string(maximum) != strconv.Itoa(workers) {
			t.Fatalf("active worker maximum=%q want=%d err=%v", maximum, workers, err)
		}
		return observed
	}
	green := run(t, "green", 3, false)
	if green.Status != "passed" || !green.CollectionComplete {
		t.Fatalf("green allowance result=%+v", green)
	}
	red := run(t, "red", 4, false)
	if red.Status != "failed" || !red.CollectionComplete || red.NativeExitStatus == nil || *red.NativeExitStatus != 9 || len(red.Observed) != 1 || red.Observed[0].Status != "failed" {
		t.Fatalf("red allowance result=%+v", red)
	}
	cancelled := run(t, "cancel", 2, true)
	if cancelled.Status != "cancelled" || !cancelled.NativeLaunched {
		t.Fatalf("cancelled allowance result=%+v", cancelled)
	}
	recovered := run(t, "green-after-cancel", 2, false)
	if recovered.Status != "passed" || !recovered.CollectionComplete {
		t.Fatalf("worker allowance was not reusable after the cancellation drain: %+v", recovered)
	}
}
