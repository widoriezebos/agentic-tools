package supervise

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginepin"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginepin/testdata/pinfixture"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

func armingExecutionChild(t *testing.T) bool {
	t.Helper()
	if os.Getenv("U12A_ARMING_CHILD") == t.Name() {
		return false
	}
	command := exec.Command(os.Args[0], "-test.run=^"+regexp.QuoteMeta(t.Name())+"$", "-test.v", "-test.timeout=30m")
	command.Env = append(os.Environ(), "U12A_ARMING_CHILD="+t.Name())
	output, err := command.CombinedOutput()
	t.Logf("%s", output)
	if err != nil {
		t.Fatal(err)
	}
	return true
}

func TestEnsureArmedJudgesTheOwnerAndComponentsByExecution(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	current, digest := pinfixture.Pin(t, root, 148, "current")
	obsolete, _ := pinfixture.Pin(t, root, 147, "obsolete")
	for _, fault := range []string{"current", "owner", "watcher", "reaper", "unknown-owner", "unknown-watcher", "unknown-reaper", "mixed-owner", "mixed-watcher", "empty-target"} {
		t.Run(fault, func(t *testing.T) {
			t.Parallel()
			observations := map[string]enginepin.Execution{}
			for i, role := range []string{"owner", "watcher", "reaper"} {
				path := current
				if fault == role || fault == "mixed-"+role {
					path = obsolete
				}
				exact := pinfixture.Process(int64(i+41), path)
				observations[role] = enginepin.Read(root, exact.Ref(), pinfixture.Prober{exact.Pid: exact})
			}
			unknown := ""
			if strings.HasPrefix(fault, "unknown-") {
				unknown = strings.TrimPrefix(fault, "unknown-")
			}
			if fault == "mixed-owner" {
				unknown = "watcher"
			}
			if fault == "mixed-watcher" {
				unknown = "reaper"
			}
			if unknown != "" {
				observations[unknown] = enginepin.Execution{Cause: "probe denied"}
			}
			target := digest
			if fault == "empty-target" {
				target = ""
			}
			got, err := obsoleteExecution(EnsureOptions{Root: root, TargetDigest: target, Executions: func(string) map[string]enginepin.Execution { return observations }})
			if unknown != "" {
				if err == nil || !strings.Contains(err.Error(), unknown) || !strings.Contains(err.Error(), "unverifiable") || !strings.Contains(err.Error(), "probe denied") {
					t.Fatalf("UNKNOWN accepted: obsolete=%t err=%v", got, err)
				}
			} else if want := fault != "current" && fault != "empty-target"; err != nil || got != want {
				t.Fatalf("judgment obsolete=%t want=%t err=%v", got, want, err)
			}
		})
	}
}

func prepareExecutionOwner(t *testing.T) (EnsureOptions, EnsureResult) {
	t.Helper()
	reapArmingOwnerProcesses(t)
	if err := os.Setenv("METASYSTEM_SUPERVISION_REGISTRY_HOME", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(os.Getenv("METASYSTEM_SUPERVISION_REGISTRY_HOME"), ".metasystem"), 0700); err != nil {
		t.Fatal(err)
	}
	options := armingOptions(t.TempDir())
	started, err := EnsureArmed(options)
	if err != nil || !started.Inspection.Armed() {
		t.Fatalf("start: %+v %v", started, err)
	}
	appendPreviousOwnerRows(t, filepath.Join(os.Getenv("METASYSTEM_SUPERVISION_REGISTRY_HOME"), ".metasystem", "armed-checkouts.jsonl"), options.Root, started)
	t.Cleanup(func() { _, _ = ShutdownAt(options.Root, options.Root, options.Root, options.OwnerTagPrefix, 1) })
	return options, started
}

func requireOwnerUntouched(t *testing.T, options EnsureOptions, before EnsureResult) {
	t.Helper()
	after, err := ReadArmingOwner(options.Root)
	if err != nil || after.Pid != before.Owner.Pid || armingOwnerLiveness(after) != identity.Alive {
		t.Fatalf("owner changed: %+v %v", after, err)
	}
	if _, err := os.Stat(filepath.Join(ownerLockDir(options.Root), "shutdown-intent.json")); !os.IsNotExist(err) {
		t.Fatalf("shutdown intent: %v", err)
	}
}

func TestRecoverOnlyArmingReplacesAnOwnerOnAnObsoleteEngine(t *testing.T) {
	t.Parallel()
	if armingExecutionChild(t) {
		return
	}
	options, started := prepareExecutionOwner(t)
	currentPath, currentDigest := pinfixture.Pin(t, options.Root, 148, "current")
	obsoletePath, _ := pinfixture.Pin(t, options.Root, 147, "obsolete")
	execution := func(path string) enginepin.Execution {
		exact := pinfixture.Process(41, path)
		return enginepin.Read(options.Root, exact.Ref(), pinfixture.Prober{41: exact})
	}
	options.TargetDigest = currentDigest
	observations := map[string]enginepin.Execution{"owner": execution(obsoletePath), "watcher": execution(currentPath), "reaper": execution(currentPath)}
	options.Executions = func(string) map[string]enginepin.Execution { return observations }
	options.OnlyIfDown = true
	replaced, err := EnsureArmed(options)
	if err != nil || replaced.Action != "replaced" || replaced.Owner.Pid == started.Owner.Pid {
		t.Fatalf("recover replacement: %+v %v", replaced, err)
	}
	appendPreviousOwnerRows(t, filepath.Join(os.Getenv("METASYSTEM_SUPERVISION_REGISTRY_HOME"), ".metasystem", "armed-checkouts.jsonl"), options.Root, replaced)
	observations["owner"] = execution(currentPath)
	same, err := EnsureArmed(options)
	if err != nil || same.Action != "not-needed" || same.Owner.Pid != replaced.Owner.Pid {
		t.Fatalf("repeat: %+v %v", same, err)
	}
	options.OnlyIfDown = false
	observations["owner"] = execution(obsoletePath)
	replacedAgain, err := EnsureArmed(options)
	if err != nil || replacedAgain.Action != "replaced" || replacedAgain.Owner.Pid == replaced.Owner.Pid {
		t.Fatalf("ordinary replacement: %+v %v", replacedAgain, err)
	}
	appendPreviousOwnerRows(t, filepath.Join(os.Getenv("METASYSTEM_SUPERVISION_REGISTRY_HOME"), ".metasystem", "armed-checkouts.jsonl"), options.Root, replacedAgain)
	for _, fault := range []string{"unknown", "changed-unknown", "mixed-owner", "mixed-watcher"} {
		for _, role := range []string{"owner", "watcher", "reaper"} {
			observations[role] = execution(currentPath)
		}
		unknown := "owner"
		if fault == "changed-unknown" {
			options.Fingerprint = "changed"
		}
		if fault == "mixed-owner" {
			observations["owner"] = execution(obsoletePath)
			unknown = "watcher"
		}
		if fault == "mixed-watcher" {
			observations["watcher"] = execution(obsoletePath)
			unknown = "reaper"
		}
		observations[unknown] = enginepin.Execution{Cause: "probe denied"}
		_, err := EnsureArmed(options)
		if err == nil || !strings.Contains(err.Error(), unknown) || !strings.Contains(err.Error(), "unverifiable") || !strings.Contains(err.Error(), "probe denied") {
			t.Fatalf("%s: %v", fault, err)
		}
		requireOwnerUntouched(t, options, replacedAgain)
	}
}

func armingPinObservations(t *testing.T, root, path string) map[string]enginepin.Execution {
	t.Helper()
	owner, err := ReadArmingOwner(root)
	if err != nil {
		t.Fatal(err)
	}
	prober := pinfixture.Prober{}
	add := func(ref identity.Ref) {
		prober[ref.Pid] = identity.Exact{Pid: ref.Pid, StartedAt: time.Unix(ref.StartedAtSec, 0), StartTicks: ref.StartTicks, BootID: ref.BootID, Exe: path, ExeKnown: true}
	}
	add(identity.Ref{Pid: owner.Pid, StartedAtSec: owner.PidStartedAt, StartTicks: owner.PidStartTicks, BootID: owner.BootID})
	held, err := recordedHeld(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, process := range held {
		add(process.Identity)
	}
	return ReadExecutions(root, prober)
}

func TestArmingReadsComponentsAfterTheReadinessWait(t *testing.T) {
	t.Parallel()
	if armingExecutionChild(t) {
		return
	}
	options, started := prepareExecutionOwner(t)
	options.OnlyIfDown = true
	current, digest := pinfixture.Pin(t, options.Root, 148, "current")
	options.TargetDigest = digest
	path := filepath.Join(SupervisionDir(options.Root), "state.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var document stateDocument
	if err = json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	watcher := document.Components["watcher"]
	delete(document.Components, "watcher")
	if err = writeArmingHelperJSON(path, document); err != nil {
		t.Fatal(err)
	}
	present := false
	options.Executions = func(root string) map[string]enginepin.Execution { return armingPinObservations(t, root, current) }

	prior := armingSleep
	t.Cleanup(func() { armingSleep = prior })
	armingSleep = func(time.Duration) {
		present = true
		document.Components["watcher"] = watcher
		if err := writeArmingHelperJSON(path, document); err != nil {
			t.Fatal(err)
		}
	}
	same, err := EnsureArmed(options)
	if err != nil || same.Action != "not-needed" || same.Owner.Pid != started.Owner.Pid || !present {
		t.Fatalf("read before readiness: %+v %v", same, err)
	}
	present = false
	delete(document.Components, "watcher")
	if err = writeArmingHelperJSON(path, document); err != nil {
		t.Fatal(err)
	}
	now := armingNow()
	priorNow := armingNow
	t.Cleanup(func() { armingNow = priorNow })
	armingNow = func() time.Time { return now }
	armingSleep = func(time.Duration) { now = now.Add(time.Hour) }
	_, err = EnsureArmed(options)
	if err == nil || !strings.Contains(err.Error(), "watcher") || !strings.Contains(err.Error(), "unverifiable") {
		t.Fatalf("absent watcher: %v", err)
	}
	requireOwnerUntouched(t, options, started)
}
