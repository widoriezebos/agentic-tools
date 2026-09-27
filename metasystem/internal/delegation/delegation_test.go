package delegation_test

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegation"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegation/fake"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

func TestNewRefusesAPartialPortSet(t *testing.T) {
	t.Parallel()
	complete := fake.NewSet().Ports()
	if _, err := delegation.New(complete); err != nil {
		t.Fatalf("a complete port set was refused: %v", err)
	}
	for _, missing := range []struct {
		name  string
		clear func(*delegation.Ports)
	}{
		{"lease", func(p *delegation.Ports) { p.Lease = nil }},
		{"steward", func(p *delegation.Ports) { p.Steward = nil }},
		{"adapter", func(p *delegation.Ports) { p.Adapter = nil }},
		{"goal", func(p *delegation.Ports) { p.Goal = nil }},
		{"record", func(p *delegation.Ports) { p.Records = nil }},
		{"event", func(p *delegation.Ports) { p.Events = nil }},
	} {
		ports := complete
		missing.clear(&ports)
		lifecycle, err := delegation.New(ports)
		if err == nil || lifecycle != nil {
			t.Fatalf("a port set without %s operations was accepted", missing.name)
		}
		if !strings.Contains(err.Error(), missing.name+" operations are not wired") {
			t.Fatalf("refusal does not name the %s port: %v", missing.name, err)
		}
	}
	if _, err := delegation.New(delegation.Ports{}); err == nil || strings.Count(err.Error(), "not wired") != 6 {
		t.Fatalf("an empty port set must name all six missing ports: %v", err)
	}
}

func TestSkeletonPhasesRefuseWithoutTouchingAnOwner(t *testing.T) {
	t.Parallel()
	doubles := fake.NewSet()
	lifecycle, err := delegation.New(doubles.Ports())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	invocation := delegation.Invocation{CallerPid: 4242}
	calls := map[delegation.Phase]func() (delegation.Outcome, error){
		delegation.PhaseDispatch: func() (delegation.Outcome, error) {
			return lifecycle.Dispatch(ctx, delegation.DispatchRequest{Invocation: invocation, Role: "implementer", JobID: "job-a"})
		},
		delegation.PhaseFollowUp: func() (delegation.Outcome, error) {
			return lifecycle.FollowUp(ctx, delegation.FollowUpRequest{Invocation: invocation, Job: "job-a"})
		},
		delegation.PhaseWatch:  func() (delegation.Outcome, error) { return lifecycle.Watch(ctx, "job-a") },
		delegation.PhaseStatus: func() (delegation.Outcome, error) { return lifecycle.Status(ctx, "job-a") },
		delegation.PhaseCancel: func() (delegation.Outcome, error) { return lifecycle.Cancel(ctx, invocation, "job-a") },
		delegation.PhaseClose: func() (delegation.Outcome, error) {
			return lifecycle.Close(ctx, delegation.CloseRequest{Invocation: invocation, Job: "job-a"})
		},
		delegation.PhaseReap: func() (delegation.Outcome, error) {
			return lifecycle.Reap(ctx, delegation.ReapRequest{Invocation: invocation, Job: "job-a"})
		},
	}
	if len(calls) != len(delegation.Phases()) {
		t.Fatalf("the test drives %d phases, the lifecycle declares %d", len(calls), len(delegation.Phases()))
	}
	for _, phase := range delegation.Phases() {
		call, ok := calls[phase]
		if !ok {
			t.Fatalf("phase %s has no driver in this test", phase)
		}
		outcome, err := call()
		if !errors.Is(err, delegation.ErrNotPorted) {
			t.Fatalf("%s: error = %v, want ErrNotPorted", phase, err)
		}
		if outcome.Phase != phase || outcome.Outcome != delegation.OutcomeNotPorted || outcome.ExitCode != 2 || outcome.JobID != "job-a" {
			t.Fatalf("%s: outcome = %+v", phase, outcome)
		}
	}
	if got := doubles.Log.Calls(); len(got) != 0 {
		t.Fatalf("a skeleton phase reached an owner: %v", got)
	}
}

// The lifecycle's phases are dispatch.sh's public commands, one for one,
// while the script exists. U6b deletes the script with this comparison.
func TestPhasesMirrorTheDispatchScriptRouter(t *testing.T) {
	t.Parallel()
	script := filepath.Join("..", "..", "scripts", "agents", "dispatch.sh")
	file, err := os.Open(script)
	if err != nil {
		t.Fatalf("dispatch.sh is the shape this skeleton mirrors; U6b removes this test with it: %v", err)
	}
	defer file.Close()
	arm := regexp.MustCompile(`^  ([a-z][a-z-]*)\) [a-z_]+ "\$@" ;;$`)
	var commands []string
	scanner := bufio.NewScanner(file)
	inRouter := false
	for scanner.Scan() {
		line := scanner.Text()
		if line == `case "$command" in` {
			inRouter = true
			continue
		}
		if inRouter && line == "esac" {
			break
		}
		if match := arm.FindStringSubmatch(line); inRouter && match != nil {
			commands = append(commands, match[1])
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	var phases []string
	for _, phase := range delegation.Phases() {
		phases = append(phases, string(phase))
	}
	if !reflect.DeepEqual(commands, phases) {
		t.Fatalf("dispatch.sh routes %v; the lifecycle declares %v", commands, phases)
	}
}

func TestOwnerPortsRequireARootAndComposeALifecycle(t *testing.T) {
	t.Parallel()
	if _, err := delegation.NewOwnerPorts(delegation.OwnerConfig{}); err == nil {
		t.Fatal("owner ports without a root were accepted")
	}
	ports, err := delegation.NewOwnerPorts(delegation.OwnerConfig{Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := delegation.New(ports); err != nil {
		t.Fatalf("the real owner wiring is incomplete: %v", err)
	}
}

func TestOwnerLeaseRefusesAnUnknownModeBeforeClassifying(t *testing.T) {
	t.Parallel()
	ports, err := delegation.NewOwnerPorts(delegation.OwnerConfig{Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	err = ports.Lease.Authorize(delegation.Invocation{CallerPid: int64(os.Getpid())}, "anyone", "job-a")
	if err == nil || !strings.Contains(err.Error(), `unknown control-plane mode "anyone"`) {
		t.Fatalf("unknown mode was not refused by name: %v", err)
	}
}

func TestOwnerRecordsReadRefusesAPathShapedJob(t *testing.T) {
	t.Parallel()
	ports, err := delegation.NewOwnerPorts(delegation.OwnerConfig{Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	for _, job := range []string{"", "..", "a/b"} {
		if _, err := ports.Records.Read(job); err == nil {
			t.Fatalf("record read accepted job %q", job)
		}
	}
	if _, err := ports.Records.Read("absent-job"); err == nil {
		t.Fatal("reading an absent record succeeded")
	}
}

func TestOwnerAdapterResultPatchIsTheAdapterOwners(t *testing.T) {
	t.Parallel()
	ports, err := delegation.NewOwnerPorts(delegation.OwnerConfig{Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "patch.json")
	if err := ports.Adapter.ResultPatch(output, "null", "supervision", ""); err != nil {
		t.Fatal(err)
	}
	var patch map[string]any
	content, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(content, &patch); err != nil {
		t.Fatal(err)
	}
	if value, present := patch["error"]; !present || value != nil || patch["phase"] != "supervision" {
		t.Fatalf("patch = %v", patch)
	}
}

const recordingAdapter = `#!/bin/sh
out=$(dirname "$0")/calls
case "$1" in
  config-identity) echo identity-7 ;;
  output-stream) shift; echo "stream:$2" ;;
  probe) echo probed >>"$out" ;;
  cancel) echo "cancel $*" >>"$out" ;;
  refuse) echo "refused on purpose" >&2; exit 3 ;;
  dispatch|follow-up)
    {
      echo "argv $*"
      echo "cwd $PWD"
      echo "author $GIT_AUTHOR_NAME $GIT_AUTHOR_EMAIL"
      echo "pgid $(ps -o pgid= -p $$ | tr -d ' ')"
      echo "pid $$"
    } >"$out.launch.tmp"
    mv "$out.launch.tmp" "$out.launch" ;;
esac
`

func adapterRoot(t *testing.T) (string, string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	adapters := filepath.Join(root, "scripts", "agents", "adapters")
	if err := os.MkdirAll(adapters, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(filepath.Join(adapters, "recorder.sh"), []byte(recordingAdapter), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(adapters, "unexecutable.sh"), []byte(recordingAdapter), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, adapters
}

func TestOwnerAdapterDrivesTheRuntimeScript(t *testing.T) {
	t.Parallel()
	root, adapters := adapterRoot(t)
	ports, err := delegation.NewOwnerPorts(delegation.OwnerConfig{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	identity, err := ports.Adapter.ConfigIdentity(ctx, "recorder")
	if err != nil || identity != "identity-7" {
		t.Fatalf("config identity = %q, %v", identity, err)
	}
	stream, err := ports.Adapter.OutputStream(ctx, "recorder", "/rounds/2")
	if err != nil || stream != "stream:/rounds/2" {
		t.Fatalf("output stream = %q, %v", stream, err)
	}
	if err := ports.Adapter.Probe(ctx, "recorder"); err != nil {
		t.Fatal(err)
	}
	if err := ports.Adapter.Cancel(ctx, "recorder", "job-a"); err != nil {
		t.Fatal(err)
	}
	calls, err := os.ReadFile(filepath.Join(adapters, "calls"))
	if err != nil {
		t.Fatal(err)
	}
	if string(calls) != "probed\ncancel cancel --job job-a\n" {
		t.Fatalf("adapter calls = %q", calls)
	}
	for _, runtime := range []string{"", "..", "a/b", "absent", "unexecutable"} {
		if _, err := ports.Adapter.ConfigIdentity(ctx, runtime); err == nil {
			t.Fatalf("runtime %q was driven", runtime)
		}
	}
}

func TestOwnerAdapterLaunchStartsADetachedSession(t *testing.T) {
	t.Parallel()
	root, adapters := adapterRoot(t)
	ports, err := delegation.NewOwnerPorts(delegation.OwnerConfig{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	request := delegation.AdapterLaunch{
		Runtime: "recorder", Verb: "dispatch", Job: "job-a", StartGate: "/gate/job-a.start",
		InstanceTag: "tag-1", LaunchCapability: "capability-1",
	}
	for _, refused := range []delegation.AdapterLaunch{
		{Runtime: "recorder", Verb: "cancel", Job: "job-a", StartGate: "g", InstanceTag: "t", LaunchCapability: "c"},
		{Runtime: "recorder", Verb: "dispatch", Job: "job-a", StartGate: "g", InstanceTag: "t"},
		{Runtime: "recorder", Verb: "dispatch", Job: "job-a", StartGate: "g", InstanceTag: "t", LaunchCapability: "c", ExecutionGuard: &delegation.ExecutionGuard{Root: root}},
	} {
		if _, err := ports.Adapter.Launch(ctx, refused); err == nil {
			t.Fatalf("launch %+v was accepted", refused)
		}
	}
	pid, err := ports.Adapter.Launch(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if pid <= 1 {
		t.Fatalf("launch returned pid %d", pid)
	}
	var status syscall.WaitStatus
	if _, err := syscall.Wait4(int(pid), &status, 0, nil); err != nil {
		t.Fatalf("the launched adapter could not be reaped: %v", err)
	}
	if !status.Exited() || status.ExitStatus() != 0 {
		t.Fatalf("the launched adapter ended %v", status)
	}
	content, err := os.ReadFile(filepath.Join(adapters, "calls.launch"))
	if err != nil {
		t.Fatalf("the launched adapter left no record: %v", err)
	}
	facts := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(content)), "\n") {
		key, value, _ := strings.Cut(line, " ")
		facts[key] = value
	}
	want := map[string]string{
		"argv":   "dispatch --job job-a --start-gate /gate/job-a.start --instance-tag tag-1 --launch-capability capability-1",
		"cwd":    root,
		"author": "job-a job-a@metasystem.invalid",
		"pgid":   strconv.FormatInt(pid, 10),
		"pid":    strconv.FormatInt(pid, 10),
	}
	var keys []string
	for key := range want {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if facts[key] != want[key] {
			t.Fatalf("launched adapter %s = %q, want %q (all: %v)", key, facts[key], want[key], facts)
		}
	}
}

func TestOwnerEventsNeverFailTheCaller(t *testing.T) {
	t.Parallel()
	ports, err := delegation.NewOwnerPorts(delegation.OwnerConfig{Root: filepath.Join(t.TempDir(), "absent"), Now: func() time.Time { return time.Unix(0, 0) }})
	if err != nil {
		t.Fatal(err)
	}
	ports.Events.Emit("job-created", "record created", map[string]string{"jobId": "job-a"})
}
