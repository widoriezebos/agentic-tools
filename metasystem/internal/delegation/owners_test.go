package delegation_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegation"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegation/fake"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

func ownerPorts(t *testing.T, root string) delegation.Ports {
	t.Helper()
	ports, err := delegation.NewOwnerPorts(delegation.OwnerConfig{Root: root, Host: fake.NewSet().Host})
	if err != nil {
		t.Fatal(err)
	}
	return ports
}

func TestOwnerPortsRequireARootAndComposeALifecycle(t *testing.T) {
	t.Parallel()
	if _, err := delegation.NewOwnerPorts(delegation.OwnerConfig{Host: fake.NewSet().Host}); err == nil {
		t.Fatal("owner ports without a root were accepted")
	}
	if _, err := delegation.NewOwnerPorts(delegation.OwnerConfig{Root: t.TempDir()}); err == nil {
		t.Fatal("owner ports without host operations were accepted")
	}
	root := t.TempDir()
	if _, err := delegation.New(delegation.Config{Root: root, RepoScope: root}, ownerPorts(t, root)); err != nil {
		t.Fatalf("the real owner wiring is incomplete: %v", err)
	}
}

func TestOwnerLeaseRefusesAnUnknownModeBeforeClassifying(t *testing.T) {
	t.Parallel()
	ports := ownerPorts(t, t.TempDir())
	err := ports.Lease.Authorize(delegation.Invocation{CallerPid: int64(os.Getpid())}, "anyone", "job-a")
	if err == nil || !strings.Contains(err.Error(), `unknown control-plane mode "anyone"`) {
		t.Fatalf("unknown mode was not refused by name: %v", err)
	}
}

func TestOwnerRecordsReadRefusesAPathShapedJob(t *testing.T) {
	t.Parallel()
	ports := ownerPorts(t, t.TempDir())
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
	ports := ownerPorts(t, t.TempDir())
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

// recordingEngine stands in for the installation's engine: its
// delegate-supervisor entry answers the adapter's small verbs for the one
// runtime it knows ("recorder") and records the calls and the launched
// supervisor's process facts beside itself.
const recordingEngine = `#!/bin/sh
[ "$1" = delegate-supervisor ] || { echo "not the supervisor entry: $*" >&2; exit 64; }
runtime=$2 verb=$3
shift 3
[ "$1" = --root ] || { echo "no --root: $*" >&2; exit 64; }
root=$2
shift 2
out=$(dirname "$0")/calls
[ "$runtime" = recorder ] || { echo "unknown runtime $runtime" >&2; exit 2; }
case "$verb" in
  signature) echo 'match ^recorder$' ;;
  config-identity) echo identity-7 ;;
  output-stream) echo "stream:$2" ;;
  probe) echo "probed $root" >>"$out" ;;
  cancel) echo "cancel $*" >>"$out" ;;
  refuse) echo "refused on purpose" >&2; exit 3 ;;
  dispatch|follow-up)
    {
      echo "argv $verb $*"
      echo "root $root"
      echo "cwd $PWD"
      echo "author $GIT_AUTHOR_NAME $GIT_AUTHOR_EMAIL"
      echo "pgid $(ps -o pgid= -p $$ | tr -d ' ')"
      echo "pid $$"
    } >"$out.launch.tmp"
    mv "$out.launch.tmp" "$out.launch"
    if [ -p "$out.launched" ]; then : >"$out.launched"; fi ;;
  *) echo "unknown verb $verb" >&2; exit 2 ;;
esac
`

// engineRoot is an installation whose bin/metasystem is the recording
// engine; it returns the root and the directory the engine records into.
func engineRoot(t *testing.T) (string, string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(filepath.Join(bin, "metasystem"), []byte(recordingEngine), 0o755); err != nil {
		t.Fatal(err)
	}
	return root, bin
}

// The adapter owner is the engine's delegate-supervisor entry (U6a): every
// small verb runs `ENGINE delegate-supervisor RUNTIME VERB --root ROOT`, and
// a runtime the installation does not resolve is refused, never driven.
func TestOwnerAdapterDrivesTheDelegateSupervisorEntry(t *testing.T) {
	t.Parallel()
	root, recorded := engineRoot(t)
	ports := ownerPorts(t, root)
	ctx := context.Background()
	if !ports.Adapter.Installed("recorder") {
		t.Fatal("the entry's signature read did not report the recorder runtime installed")
	}
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
	calls, err := os.ReadFile(filepath.Join(recorded, "calls"))
	if err != nil {
		t.Fatal(err)
	}
	if string(calls) != "probed "+root+"\ncancel --job job-a\n" {
		t.Fatalf("adapter calls = %q", calls)
	}
	for _, runtime := range []string{"", "..", "a/b", "-x", "absent"} {
		if ports.Adapter.Installed(runtime) {
			t.Fatalf("runtime %q reported installed", runtime)
		}
		if _, err := ports.Adapter.ConfigIdentity(ctx, runtime); err == nil {
			t.Fatalf("runtime %q was driven", runtime)
		}
	}
}

func TestOwnerAdapterLaunchStartsADetachedSession(t *testing.T) {
	t.Parallel()
	root, recorded := engineRoot(t)
	ports := ownerPorts(t, root)
	ctx := context.Background()
	request := delegation.AdapterLaunch{
		Runtime: "recorder", Verb: "dispatch", Job: "job-a", StartGate: "/gate/job-a.start",
		InstanceTag: "tag-1", LaunchCapability: "capability-1",
	}
	for _, refused := range []delegation.AdapterLaunch{
		{Runtime: "recorder", Verb: "cancel", Job: "job-a", StartGate: "g", InstanceTag: "t", LaunchCapability: "c"},
		{Runtime: "recorder", Verb: "dispatch", Job: "job-a", StartGate: "g", InstanceTag: "t"},
		{Runtime: "recorder", Verb: "dispatch", Job: "job-a", StartGate: "g", InstanceTag: "t", LaunchCapability: "c", ExecutionGuard: &delegation.ExecutionGuard{Root: root}},
		{Runtime: "../x", Verb: "dispatch", Job: "job-a", StartGate: "g", InstanceTag: "t", LaunchCapability: "c"},
	} {
		if _, err := ports.Adapter.Launch(ctx, refused); err == nil {
			t.Fatalf("launch %+v was accepted", refused)
		}
	}
	// The launched supervisor signals through a named pipe once its record
	// is in place: the read blocks on that event, not on a clock.
	launched := filepath.Join(recorded, "calls.launched")
	if err := syscall.Mkfifo(launched, 0o600); err != nil {
		t.Fatal(err)
	}
	// The supervisor is detached in its own session and blocks opening the
	// pipe until the read below; a failure before that read must not leave
	// it blocked for good.
	var launchedPid int64
	testenv.ReapFixtureProcessGroups(t, []testenv.FixtureProcessGroup{{
		Verb:    "detached recorder supervisor",
		Resolve: func() (int, bool, error) { return int(launchedPid), launchedPid > 1, nil },
	}})
	pid, err := ports.Adapter.Launch(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	launchedPid = pid
	if pid <= 1 {
		t.Fatalf("launch returned pid %d", pid)
	}
	if _, err := os.ReadFile(launched); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(recorded, "calls.launch"))
	if err != nil {
		t.Fatalf("the launched supervisor left no record: %v", err)
	}
	facts := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(content)), "\n") {
		key, value, _ := strings.Cut(line, " ")
		facts[key] = value
	}
	want := map[string]string{
		"argv":   "dispatch --job job-a --start-gate /gate/job-a.start --instance-tag tag-1 --launch-capability capability-1",
		"root":   root,
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
			t.Fatalf("launched supervisor %s = %q, want %q (all: %v)", key, facts[key], want[key], facts)
		}
	}
}

func TestOwnerEventsNeverFailTheCaller(t *testing.T) {
	t.Parallel()
	ports, err := delegation.NewOwnerPorts(delegation.OwnerConfig{Root: filepath.Join(t.TempDir(), "absent"), Host: fake.NewSet().Host, Now: func() time.Time { return time.Unix(0, 0) }})
	if err != nil {
		t.Fatal(err)
	}
	ports.Events.Emit("job-created", "record created", map[string]string{"jobId": "job-a"})
}

func TestOwnerLeaseHeldRunsAStewardCallerWithoutTheLease(t *testing.T) {
	t.Parallel()
	ports := ownerPorts(t, filepath.Join(t.TempDir(), "no-lease-here"))
	ran := false
	err := ports.Lease.Held(delegation.Invocation{CallerPid: int64(os.Getpid()), CallerClass: lease.ClassSteward}, nil, func() error {
		ran = true
		return errors.New("the write's own verdict")
	})
	if !ran || err == nil || err.Error() != "the write's own verdict" {
		t.Fatalf("a STEWARD caller must run ungated with fn's own verdict: ran=%v err=%v", ran, err)
	}
}
