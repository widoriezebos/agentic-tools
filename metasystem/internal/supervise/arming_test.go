package supervise

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

type armingComponentProbe struct {
	exact identity.Exact
	state identity.Liveness
}

type armingProbeFunc func(int64) (identity.Exact, identity.Liveness, error)

func (f armingProbeFunc) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	return f(pid)
}

func armingHelperArgs() ([]string, bool) {
	for index, argument := range os.Args {
		if argument == "--arming-owner-helper" {
			return os.Args[index+1:], true
		}
	}
	return nil, false
}

func writeArmingHelperJSON(path string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func TestArmingOwnerHelper(t *testing.T) {
	args, helper := armingHelperArgs()
	if !helper {
		t.Skip("only runs as an arming owner subprocess")
	}
	root := processArgument(args, "--repo")
	gate := processArgument(args, "--gate")
	tag := processArgument(args, "--tag")
	fingerprint := processArgument(args, "--fingerprint")
	interval, _ := strconv.Atoi(processArgument(args, "--interval"))
	watcherCap, _ := strconv.Atoi(processArgument(args, "--watcher-cap"))
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(gate); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the parent did not open the owner publication gate")
		}
		time.Sleep(10 * time.Millisecond)
	}
	exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("read helper identity: state=%s err=%v", state, err)
	}
	checkout := &DiskCheckout{
		Root: root, Self: exact.Ref(), SelfTag: tag, IntervalSec: interval,
		Fingerprint: fingerprint, WatcherCap: watcherCap,
	}
	generation := checkout.PriorGeneration() + 1
	held := make([]Held, 0, 2)
	for _, component := range []Component{Watcher, Reaper} {
		componentTag := tag + "-" + string(component) + "-1"
		command := exec.Command(os.Args[0], "-test.run=^TestTakeoverComponentHelper$", "--", "--takeover-component-helper", componentTag)
		command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		if err := command.Start(); err != nil {
			t.Fatal(err)
		}
		componentExact, componentState, componentErr := (identity.KernelProber{}).Probe(int64(command.Process.Pid))
		if componentErr != nil || componentState != identity.Alive {
			_ = command.Process.Kill()
			t.Fatalf("read %s helper identity: state=%s err=%v", component, componentState, componentErr)
		}
		held = append(held, Held{Component: component, Tag: componentTag, Identity: componentExact.Ref(), Generation: generation})
		_ = command.Process.Release()
	}
	if err := checkout.PublishState(held); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	supervisionDir := SupervisionDir(root)
	if err := writeArmingHelperJSON(filepath.Join(supervisionDir, "watcher.heartbeat.json"), map[string]any{
		"observedAtEpoch": now, "loadedCapMin": watcherCap,
	}); err != nil {
		t.Fatal(err)
	}
	if err := writeArmingHelperJSON(filepath.Join(supervisionDir, "reaper.heartbeat.json"), map[string]any{
		"observedAtEpoch": now,
	}); err != nil {
		t.Fatal(err)
	}
	if err := writeArmingHelperJSON(filepath.Join(supervisionDir, "last-census.json"), map[string]any{
		"verdict": "SUCCESS", "fingerprint": fingerprint, "generation": generation, "completedAtEpoch": now,
	}); err != nil {
		t.Fatal(err)
	}
	for {
		time.Sleep(time.Hour)
	}
}

func TestTakeoverComponentHelper(t *testing.T) {
	for _, argument := range os.Args {
		if argument == "--takeover-component-helper" {
			for {
				time.Sleep(time.Hour)
			}
		}
	}
	t.Skip("only runs as a takeover component subprocess")
}

func armingOwnerCommand(args ...string) (*exec.Cmd, error) {
	arguments := append([]string{"-test.run=^TestArmingOwnerHelper$", "--", "--arming-owner-helper"}, args...)
	return exec.Command(os.Args[0], arguments...), nil
}

func armingOptions(root string) EnsureOptions {
	return EnsureOptions{
		Root: root, MetasystemRoot: root, Scope: root, Command: armingOwnerCommand,
		Fingerprint: "fingerprint-a", IntervalSec: 1, WatcherCap: 330,
		WaitScaleMilli: 1, OwnerTagPrefix: "metasystem-supervision-owner-test-",
	}
}

func reapArmingOwnerProcesses(t *testing.T) {
	t.Helper()
	prior := releaseLaunchedOwner
	releaseLaunchedOwner = func(command *exec.Cmd) error {
		go func() { _ = command.Wait() }()
		return nil
	}
	t.Cleanup(func() { releaseLaunchedOwner = prior })
}

func fakeArmingOwnerLiveness(t *testing.T, states ...identity.Liveness) {
	t.Helper()
	prior := armingOwnerLiveness
	index := 0
	armingOwnerLiveness = func(ArmingOwner) identity.Liveness {
		if index >= len(states) {
			return states[len(states)-1]
		}
		state := states[index]
		index++
		return state
	}
	t.Cleanup(func() { armingOwnerLiveness = prior })
}

func TestTakeoverCensusFindsComponentsLaunchedBeforeStatePublication(t *testing.T) {
	root := t.TempDir()
	ownerTag := "metasystem-supervision-owner-repo-1"
	prior := enumerateTakeoverProcesses
	enumerateTakeoverProcesses = func(string) ([]census.Process, error) {
		return []census.Process{{
			Pid: 71, PGID: 71, Started: 100, Alive: true,
			Argv: "/engine supervise component --component watcher --tag " + ownerTag + "-watcher-1 --repo " + root,
		}}, nil
	}
	t.Cleanup(func() { enumerateTakeoverProcesses = prior })
	held, err := takeoverComponents(root, root, ownerTag)
	if err != nil {
		t.Fatal(err)
	}
	if len(held) != 1 || held[0].Component != Watcher || held[0].Identity.Pid != 71 {
		t.Fatalf("pre-publication watcher was absent from takeover set: %+v", held)
	}
}

func (p *armingComponentProbe) Probe(int64) (identity.Exact, identity.Liveness, error) {
	return p.exact, p.state, nil
}

func TestArmingOwnerRecordsAndPublishedGenerationRoundTrip(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(ownerLockDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := WriteArmingOwner(root, ArmingOwner{}); err == nil {
		t.Fatal("an incomplete owner identity was published")
	}
	owner := ArmingOwner{Pid: 41, PidStartedAt: 100, PidStartTicks: 900, BootID: "boot-a", InstanceTag: "owner-a"}
	if err := WriteArmingOwner(root, owner); err != nil {
		t.Fatal(err)
	}
	read, err := ReadArmingOwner(root)
	if err != nil || !sameArmingOwner(read, owner) {
		t.Fatalf("owner round trip: owner=%+v err=%v", read, err)
	}
	document := stateDocument{
		Owner:       stateIdentity{Pid: owner.Pid, PidStartedAt: owner.PidStartedAt, PidStartTicks: owner.PidStartTicks, BootID: owner.BootID, InstanceTag: owner.InstanceTag},
		Fingerprint: "fingerprint-a", IntervalSec: 7, DerivedWatcherCapMin: 330, Generation: 9,
	}
	if err := os.MkdirAll(SupervisionDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeArmingHelperJSON(filepath.Join(SupervisionDir(root), "state.json"), document); err != nil {
		t.Fatal(err)
	}
	published, err := ReadPublishedGeneration(root)
	if err != nil || published.Generation != 9 || published.Fingerprint != "fingerprint-a" || !sameArmingOwner(published.Owner, owner) {
		t.Fatalf("published generation round trip: generation=%+v err=%v", published, err)
	}
	if _, err := publishedForOwner(root, owner, 0); err != nil {
		t.Fatalf("the matching published owner was not accepted: %v", err)
	}
	other := owner
	other.InstanceTag = "owner-b"
	if _, err := publishedForOwner(root, other, 0); err == nil || !strings.Contains(err.Error(), "another owner") {
		t.Fatalf("a generation naming another owner was accepted: %v", err)
	}
	if failure := (&ComponentFailure{Component: "repo-watcher", Err: os.ErrPermission}); !errors.Is(failure, os.ErrPermission) || failure.Error() != os.ErrPermission.Error() {
		t.Fatalf("component failure did not preserve its cause: %v", failure)
	}
}

func TestArmingOwnerRecordsRejectMalformedAndIncompleteDocuments(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(ownerLockDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	path := ownerPath(root)
	if err := os.WriteFile(path, []byte("not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadArmingOwner(root); err == nil || !strings.Contains(err.Error(), "malformed") {
		t.Fatalf("malformed owner record was accepted: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"pid":41,"pidStartedAt":100,"instanceTag":""}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadArmingOwner(root); err == nil || !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("incomplete owner record was accepted: %v", err)
	}
	if err := os.MkdirAll(SupervisionDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(SupervisionDir(root), "state.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadPublishedGeneration(root); err == nil || !strings.Contains(err.Error(), "malformed") {
		t.Fatalf("malformed generation was accepted: %v", err)
	}
}

func TestGenerationMatchesAllArmingInputs(t *testing.T) {
	options := EnsureOptions{Fingerprint: "current", IntervalSec: 60, WatcherCap: 330}
	current := PublishedGeneration{Fingerprint: "current", IntervalSec: 60, WatcherCap: 330}
	if !generationMatches(current, options) {
		t.Fatal("the current engine generation did not match")
	}
	for name, mutate := range map[string]func(*PublishedGeneration){
		"fingerprint": func(generation *PublishedGeneration) { generation.Fingerprint = "older" },
		"interval":    func(generation *PublishedGeneration) { generation.IntervalSec = 30 },
		"ceiling":     func(generation *PublishedGeneration) { generation.WatcherCap = 300 },
	} {
		t.Run(name, func(t *testing.T) {
			generation := current
			mutate(&generation)
			if generationMatches(generation, options) {
				t.Fatal("an older engine generation matched current arming inputs")
			}
		})
	}
}

func TestSameArmingOwnerRequiresTheCompleteRecordedIdentity(t *testing.T) {
	owner := ArmingOwner{Pid: 7, PidStartedAt: 11, InstanceTag: "owner-tag"}
	if !sameArmingOwner(owner, owner) {
		t.Fatal("an identical owner did not match itself")
	}
	changed := owner
	changed.InstanceTag = "replacement-tag"
	if sameArmingOwner(owner, changed) {
		t.Fatal("an owner with another instance tag matched")
	}
}

func TestRecordedComponentStopAuthenticatesTagAndProvesGroupAbsent(t *testing.T) {
	probe := &armingComponentProbe{state: identity.Alive, exact: identity.Exact{
		Pid: 71, StartedAt: time.Unix(100, 0), StartTicks: 900, BootID: "boot-a",
		Argv: []string{"metasystem", "component-tag"}, ArgvKnown: true,
	}}
	held := Held{Component: Watcher, Tag: "component-tag", Identity: identity.Ref{
		Pid: 71, StartedAtSec: 100, StartTicks: 900, BootID: "boot-a",
	}}
	absent := false
	var signals []syscall.Signal
	control := recordedComponentControl{
		prober: probe,
		groupAbsent: func(int64) (bool, error) {
			return absent, nil
		},
		signalGroup: func(_ int64, signal syscall.Signal) error {
			signals = append(signals, signal)
			absent = true
			return nil
		},
	}
	if err := stopRecordedComponent(control, held, 1); err != nil {
		t.Fatal(err)
	}
	if len(signals) != 1 || signals[0] != syscall.SIGTERM {
		t.Fatalf("unexpected signals: %v", signals)
	}
}

func TestRecordedComponentStopRefusesReusedPidWhileGroupRemains(t *testing.T) {
	probe := &armingComponentProbe{state: identity.Alive, exact: identity.Exact{
		Pid: 71, StartedAt: time.Unix(100, 0), StartTicks: 901, BootID: "boot-a",
		Argv: []string{"stranger", "component-tag"}, ArgvKnown: true,
	}}
	held := Held{Component: Reaper, Tag: "component-tag", Identity: identity.Ref{
		Pid: 71, StartedAtSec: 100, StartTicks: 900, BootID: "boot-a",
	}}
	signalled := false
	control := recordedComponentControl{
		prober: probe,
		groupAbsent: func(int64) (bool, error) {
			return false, nil
		},
		signalGroup: func(int64, syscall.Signal) error {
			signalled = true
			return nil
		},
	}
	err := stopRecordedComponent(control, held, 1)
	if err == nil || !strings.Contains(err.Error(), "no longer tag-authenticated") {
		t.Fatalf("reused pid was not refused: %v", err)
	}
	if signalled {
		t.Fatal("reused pid was signalled")
	}
}

func TestRecordedComponentAuthenticationRefusesUnknownAndUnauthenticatedGroups(t *testing.T) {
	held := Held{Component: Watcher, Tag: "component-tag", Identity: identity.Ref{Pid: 71, StartedAtSec: 100}}
	t.Run("uninspectable identity", func(t *testing.T) {
		control := recordedComponentControl{
			prober:      &armingComponentProbe{state: identity.Unknown},
			groupAbsent: func(int64) (bool, error) { t.Fatal("unknown identity must not inspect its group"); return false, nil },
		}
		err := authenticateRecordedComponent(control, held)
		if err == nil || !strings.Contains(err.Error(), "uninspectable") {
			t.Fatalf("unknown identity was accepted: %v", err)
		}
	})
	t.Run("dead identity with absent group", func(t *testing.T) {
		control := recordedComponentControl{
			prober:      &armingComponentProbe{state: identity.Dead},
			groupAbsent: func(int64) (bool, error) { return true, nil },
		}
		if err := authenticateRecordedComponent(control, held); !errors.Is(err, errRecordedComponentGone) {
			t.Fatalf("an absent dead component did not become an idempotent stop: %v", err)
		}
		if err := stopRecordedComponent(control, held, 1); err != nil {
			t.Fatalf("an already absent component blocked takeover: %v", err)
		}
	})
	t.Run("dead identity with surviving group", func(t *testing.T) {
		control := recordedComponentControl{
			prober:      &armingComponentProbe{state: identity.Dead},
			groupAbsent: func(int64) (bool, error) { return false, nil },
		}
		err := authenticateRecordedComponent(control, held)
		if err == nil || !strings.Contains(err.Error(), "process group 71 remains") {
			t.Fatalf("an unauthenticated surviving group was accepted: %v", err)
		}
	})
	t.Run("group proof failure", func(t *testing.T) {
		control := recordedComponentControl{
			prober:      &armingComponentProbe{state: identity.Dead},
			groupAbsent: func(int64) (bool, error) { return false, os.ErrPermission },
		}
		if err := authenticateRecordedComponent(control, held); !errors.Is(err, os.ErrPermission) {
			t.Fatalf("group proof failure lost its cause: %v", err)
		}
	})
}

func TestRecordedComponentStopReportsSignalAndGroupProofFailures(t *testing.T) {
	probe := &armingComponentProbe{state: identity.Alive, exact: identity.Exact{
		Pid: 71, StartedAt: time.Unix(100, 0), Argv: []string{"component-tag"}, ArgvKnown: true,
	}}
	held := Held{Component: Reaper, Tag: "component-tag", Identity: identity.Ref{Pid: 71, StartedAtSec: 100}}
	t.Run("signal failure", func(t *testing.T) {
		control := recordedComponentControl{
			prober: probe, groupAbsent: func(int64) (bool, error) { return false, nil },
			signalGroup: func(int64, syscall.Signal) error { return os.ErrPermission },
		}
		if err := stopRecordedComponent(control, held, 1); !errors.Is(err, os.ErrPermission) || !strings.Contains(err.Error(), "signal recorded reaper") {
			t.Fatalf("signal failure was not named: %v", err)
		}
	})
	t.Run("absence proof failure", func(t *testing.T) {
		control := recordedComponentControl{
			prober: probe, groupAbsent: func(int64) (bool, error) { return false, os.ErrPermission },
			signalGroup: func(int64, syscall.Signal) error { return nil },
		}
		if err := stopRecordedComponent(control, held, 1); !errors.Is(err, os.ErrPermission) || !strings.Contains(err.Error(), "prove recorded reaper") {
			t.Fatalf("group proof failure was not named: %v", err)
		}
	})
}

func TestRecordedComponentStopEscalatesOnlyAfterReauthentication(t *testing.T) {
	probe := &armingComponentProbe{state: identity.Alive, exact: identity.Exact{
		Pid: 71, StartedAt: time.Unix(100, 0), Argv: []string{"component-tag"}, ArgvKnown: true,
	}}
	held := Held{Component: Watcher, Tag: "component-tag", Identity: identity.Ref{Pid: 71, StartedAtSec: 100}}
	killed := false
	var signals []syscall.Signal
	control := recordedComponentControl{
		prober:      probe,
		groupAbsent: func(int64) (bool, error) { return killed, nil },
		signalGroup: func(_ int64, signal syscall.Signal) error {
			signals = append(signals, signal)
			if signal == syscall.SIGKILL {
				killed = true
			}
			return nil
		},
	}
	if err := stopRecordedComponent(control, held, 1); err != nil {
		t.Fatal(err)
	}
	if len(signals) != 2 || signals[0] != syscall.SIGTERM || signals[1] != syscall.SIGKILL {
		t.Fatalf("stubborn component signal order: %v", signals)
	}
}

func TestRecordedComponentRefusesEscalationWhenIdentityBecomesUninspectable(t *testing.T) {
	exact := identity.Exact{Pid: 71, StartedAt: time.Unix(100, 0), Argv: []string{"component-tag"}, ArgvKnown: true}
	probes := 0
	prober := armingProbeFunc(func(int64) (identity.Exact, identity.Liveness, error) {
		probes++
		if probes == 1 {
			return exact, identity.Alive, nil
		}
		return identity.Exact{}, identity.Unknown, os.ErrPermission
	})
	held := Held{Component: Watcher, Tag: "component-tag", Identity: identity.Ref{Pid: 71, StartedAtSec: 100}}
	signals := 0
	control := recordedComponentControl{
		prober:      prober,
		groupAbsent: func(int64) (bool, error) { return false, nil },
		signalGroup: func(int64, syscall.Signal) error { signals++; return nil },
	}
	err := stopRecordedComponent(control, held, 1)
	if err == nil || !strings.Contains(err.Error(), "refuse SIGKILL") || !strings.Contains(err.Error(), "uninspectable") {
		t.Fatalf("escalation did not preserve the authentication refusal: %v", err)
	}
	if signals != 1 {
		t.Fatalf("an uninspectable component received %d signals, want only SIGTERM", signals)
	}
}

func TestOwnerStopRefusesUninspectableRecordedIdentityBeforeAnyMutation(t *testing.T) {
	fakeArmingOwnerLiveness(t, identity.Unknown)
	root := t.TempDir()
	owner := ArmingOwner{Pid: 41, PidStartedAt: 100, InstanceTag: "owner-tag"}
	if err := stopOwner(root, owner, 1, "test replacement"); err == nil || !strings.Contains(err.Error(), "uninspectable") {
		t.Fatalf("an uninspectable owner was accepted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(ownerLockDir(root), "shutdown-intent.json")); !os.IsNotExist(err) {
		t.Fatalf("an uninspectable owner received a shutdown intent: %v", err)
	}
}

func TestOwnerStopReauthenticatesAfterWritingTheShutdownIntent(t *testing.T) {
	root := t.TempDir()
	owner := ArmingOwner{Pid: 41, PidStartedAt: 100, InstanceTag: "owner-tag"}
	t.Run("owner died", func(t *testing.T) {
		fakeArmingOwnerLiveness(t, identity.Alive, identity.Alive, identity.Dead)
		if err := stopOwner(root, owner, 1, "test replacement"); err != nil {
			t.Fatalf("an owner that died before signalling blocked replacement: %v", err)
		}
	})
	t.Run("identity became unknown", func(t *testing.T) {
		fakeArmingOwnerLiveness(t, identity.Alive, identity.Alive, identity.Unknown)
		err := stopOwner(root, owner, 1, "test replacement")
		if err == nil || !strings.Contains(err.Error(), "before signalling") {
			t.Fatalf("an owner that became uninspectable was signalled: %v", err)
		}
	})
}

func TestEnsureArmedRefusesAnUninspectableLockOwner(t *testing.T) {
	fakeArmingOwnerLiveness(t, identity.Unknown)
	root := t.TempDir()
	if err := os.MkdirAll(ownerLockDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	owner := ArmingOwner{Pid: 41, PidStartedAt: 100, InstanceTag: "owner-tag"}
	if err := WriteArmingOwner(root, owner); err != nil {
		t.Fatal(err)
	}
	if _, err := EnsureArmed(armingOptions(root)); err == nil || !strings.Contains(err.Error(), "takeover is not authorized") {
		t.Fatalf("an uninspectable lock owner allowed takeover: %v", err)
	}
}

func TestTakeoverRefusalNamesTheRecordedComponent(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(SupervisionDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("sleep", "30")
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = command.Process.Kill(); _, _ = command.Process.Wait() })
	exact, state, err := (identity.KernelProber{}).Probe(int64(command.Process.Pid))
	if err != nil || state != identity.Alive {
		t.Fatalf("read recorded component identity: state=%s err=%v", state, err)
	}
	document := stateDocument{Generation: 4, Components: map[string]stateComponent{
		string(Reaper): {
			Pid: exact.Pid, PidStartedAt: exact.StartedAt.Unix(), PidStartTicks: exact.StartTicks,
			BootID: exact.BootID, InstanceTag: "tag-not-present-in-this-process",
		},
	}}
	if err := writeArmingHelperJSON(filepath.Join(SupervisionDir(root), "state.json"), document); err != nil {
		t.Fatal(err)
	}
	prior := enumerateTakeoverProcesses
	enumerateTakeoverProcesses = func(string) ([]census.Process, error) { return nil, nil }
	t.Cleanup(func() { enumerateTakeoverProcesses = prior })
	err = stopTakeoverComponents(root, root, "owner", 1)
	var componentFailure *ComponentFailure
	if !errors.As(err, &componentFailure) || componentFailure.Component != "job-reaper" || !strings.Contains(err.Error(), "no longer tag-authenticated") {
		t.Fatalf("takeover refusal lost the component and authentication reason: %v", err)
	}
}

func TestRecordedTakeoverStateRejectsMalformedAndIncompleteIdentity(t *testing.T) {
	root := t.TempDir()
	if held, err := recordedHeld(root); err != nil || held != nil {
		t.Fatalf("missing state was not an empty takeover set: held=%+v err=%v", held, err)
	}
	if err := os.MkdirAll(SupervisionDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(SupervisionDir(root), "state.json")
	if err := os.WriteFile(statePath, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := recordedHeld(root); err == nil || !strings.Contains(err.Error(), "malformed") {
		t.Fatalf("malformed takeover state was accepted: %v", err)
	}
	document := stateDocument{Generation: 2, Components: map[string]stateComponent{
		string(Watcher): {Pid: 41, PidStartedAt: 100},
	}}
	if err := writeArmingHelperJSON(statePath, document); err != nil {
		t.Fatal(err)
	}
	if _, err := recordedHeld(root); err == nil || !strings.Contains(err.Error(), "recorded watcher identity is incomplete") {
		t.Fatalf("incomplete takeover identity was accepted: %v", err)
	}
	if err := os.Chmod(statePath, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := recordedHeld(root); err == nil {
		t.Fatal("unreadable takeover state was accepted")
	}
}

func TestEnsureArmedRefusesIncompleteOptionsAndUnownedLocks(t *testing.T) {
	if _, err := EnsureArmed(EnsureOptions{}); err == nil || !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("incomplete arming options were accepted: %v", err)
	}
	root := t.TempDir()
	options := armingOptions(root)
	if err := os.MkdirAll(ownerLockDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ownerPath(root), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := EnsureArmed(options); err == nil || !strings.Contains(err.Error(), "no provable owner") {
		t.Fatalf("an unowned arming lock was accepted: %v", err)
	}
}

func TestEnsureArmedRefusesReservedCapacityBeforeLaunching(t *testing.T) {
	root := t.TempDir()
	jobDir := filepath.Join(root, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobDir, "job-a.json"), []byte(`{"jobId":"job-a","capMin":330,"status":"running"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := EnsureArmed(armingOptions(root)); err == nil || !strings.Contains(err.Error(), "does not strictly clear reserved cap") {
		t.Fatalf("reserved capacity did not fence arming: %v", err)
	}
	if _, err := os.Stat(ownerLockDir(root)); !os.IsNotExist(err) {
		t.Fatalf("a refused arming attempt retained the owner lock: %v", err)
	}
}

func TestDeadOwnerTakeoverReportsCensusFailureWithoutChangingTheLock(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(ownerLockDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	owner := ArmingOwner{Pid: 99999999, PidStartedAt: 1, InstanceTag: "dead-owner"}
	if err := WriteArmingOwner(root, owner); err != nil {
		t.Fatal(err)
	}
	prior := enumerateTakeoverProcesses
	enumerateTakeoverProcesses = func(string) ([]census.Process, error) { return nil, os.ErrPermission }
	t.Cleanup(func() { enumerateTakeoverProcesses = prior })
	if _, err := EnsureArmed(armingOptions(root)); !errors.Is(err, os.ErrPermission) || !strings.Contains(err.Error(), "dead-owner takeover refused") {
		t.Fatalf("census failure did not refuse dead-owner takeover: %v", err)
	}
	read, err := ReadArmingOwner(root)
	if err != nil || !sameArmingOwner(read, owner) {
		t.Fatalf("a refused takeover changed the owner lock: owner=%+v err=%v", read, err)
	}
}

func TestCapAuthorityLockTimesOutBehindAnotherArmer(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(SupervisionDir(root), "cap-authority.lock.d")
	if err := os.MkdirAll(filepath.Dir(directory), 0o755); err != nil {
		t.Fatal(err)
	}
	holder := exec.Command("sleep", "30")
	if err := holder.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = holder.Process.Kill(); _, _ = holder.Process.Wait() })
	if err := dispatch.OwnerLockClaim(directory, int64(holder.Process.Pid), "sleep"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = dispatch.OwnerLockRelease(directory, int64(holder.Process.Pid), "sleep") })
	if _, err := acquireCapAuthorityLock(root, 1); err == nil || !strings.Contains(err.Error(), "remained busy") {
		t.Fatalf("a second armer crossed the cap-authority lock: %v", err)
	}
}

func TestLaunchOwnerReportsCommandAndStartFailures(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(SupervisionDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	options := armingOptions(root)
	options.Command = func(...string) (*exec.Cmd, error) { return nil, os.ErrInvalid }
	if _, err := launchOwner(options, "owner-tag"); !errors.Is(err, os.ErrInvalid) {
		t.Fatalf("command construction failure lost its cause: %v", err)
	}
	options.Command = func(...string) (*exec.Cmd, error) { return exec.Command(filepath.Join(root, "missing-binary")), nil }
	if _, err := launchOwner(options, "owner-tag"); err == nil {
		t.Fatal("a missing owner binary was reported as launched")
	}
	options.Command = nil
	options.Binary = filepath.Join(root, "also-missing")
	if _, err := launchOwner(options, "owner-tag"); err == nil {
		t.Fatal("a missing configured owner binary was reported as launched")
	}
}

func TestCapAuthorityLockReportsInvalidRepositoryLayout(t *testing.T) {
	root := t.TempDir()
	blocked := filepath.Join(root, "blocked")
	if err := os.WriteFile(blocked, []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := acquireCapAuthorityLock(blocked, 1); err == nil {
		t.Fatal("cap authority lock was created below a file")
	}
}

func TestLaunchOwnerReportsEarlyExitAndPublicationFailures(t *testing.T) {
	reapArmingOwnerProcesses(t)
	t.Run("owner record", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(SupervisionDir(root), "lock.d", "owner.json"), 0o755); err != nil {
			t.Fatal(err)
		}
		options := armingOptions(root)
		if _, err := launchOwner(options, "owner-tag"); err == nil {
			t.Fatal("owner publication through a directory succeeded")
		}
	})
	t.Run("start gate", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(ownerLockDir(root), 0o755); err != nil {
			t.Fatal(err)
		}
		options := armingOptions(root)
		var mkdirErr error
		options.Command = func(args ...string) (*exec.Cmd, error) {
			gate := processArgument(args, "--gate")
			mkdirErr = os.Mkdir(gate, 0o755)
			if mkdirErr != nil {
				return nil, mkdirErr
			}
			return exec.Command("sh", "-c", "sleep 30"), nil
		}
		_, err := launchOwner(options, "owner-tag")
		if mkdirErr != nil {
			t.Fatal(mkdirErr)
		}
		if err == nil {
			t.Fatal("an owner whose start gate was blocked was accepted")
		}
	})
}

func TestTakeoverAndShutdownUtilityFailuresStayFailClosed(t *testing.T) {
	root := t.TempDir()
	prior := enumerateTakeoverProcesses
	enumerateTakeoverProcesses = func(string) ([]census.Process, error) { return nil, os.ErrPermission }
	if _, err := takeoverComponents(root, root, "owner"); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("takeover census failure lost its cause: %v", err)
	}
	enumerateTakeoverProcesses = prior
	if err := signalGroup(99999999, syscall.SIGTERM); err != nil {
		t.Fatalf("signalling an absent process group was not idempotent: %v", err)
	}
	if err := releaseDeadOwnerLock(root, ArmingOwner{}); err != nil {
		t.Fatalf("releasing an already absent owner lock failed: %v", err)
	}
	if err := os.MkdirAll(ownerLockDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ownerPath(root), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Shutdown(root, "owner", 1); err == nil {
		t.Fatal("shutdown accepted a malformed owner record")
	}
}

func TestTakeoverCensusRejectsForeignAndIncompleteComponents(t *testing.T) {
	root := t.TempDir()
	ownerTag := "owner-a"
	base := census.Process{
		Pid: 71, PGID: 71, Started: 100, Alive: true,
		Argv: "engine supervise component --component watcher --tag owner-a-watcher-1 --repo " + root,
	}
	t.Run("foreign repository", func(t *testing.T) {
		foreign := base
		foreign.Argv = strings.Replace(foreign.Argv, root, t.TempDir(), 1)
		if _, err := taggedTakeoverComponents(root, ownerTag, []census.Process{foreign}); err == nil || !strings.Contains(err.Error(), "another repository") {
			t.Fatalf("a foreign watcher entered the takeover set: %v", err)
		}
	})
	t.Run("incomplete process group", func(t *testing.T) {
		incomplete := base
		incomplete.PGID = 72
		if _, err := taggedTakeoverComponents(root, ownerTag, []census.Process{incomplete}); err == nil || !strings.Contains(err.Error(), "incomplete") {
			t.Fatalf("an incomplete watcher entered the takeover set: %v", err)
		}
	})
	t.Run("irrelevant processes", func(t *testing.T) {
		ignored := []census.Process{
			{Pid: 1, Alive: false, Argv: base.Argv},
			{Pid: 2, Alive: true, Argv: "unrelated process"},
			{Pid: 3, Alive: true, Argv: "engine --component other --tag owner-a-other-1 --repo " + root},
		}
		held, err := taggedTakeoverComponents(root, ownerTag, ignored)
		if err != nil || len(held) != 0 {
			t.Fatalf("irrelevant processes changed the takeover set: held=%+v err=%v", held, err)
		}
	})
}

func TestTakeoverMergeRefusesConflictingRecordedAndCensusIdentity(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(SupervisionDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	document := stateDocument{Generation: 7, Components: map[string]stateComponent{
		"watcher": {Pid: 71, PidStartedAt: 100, InstanceTag: "recorded-watcher"},
	}}
	if err := writeArmingHelperJSON(filepath.Join(SupervisionDir(root), "state.json"), document); err != nil {
		t.Fatal(err)
	}
	prior := enumerateTakeoverProcesses
	enumerateTakeoverProcesses = func(string) ([]census.Process, error) {
		return []census.Process{{
			Pid: 71, PGID: 71, Started: 101, Alive: true,
			Argv: "engine --component watcher --tag owner-a-watcher-1 --repo " + root,
		}}, nil
	}
	t.Cleanup(func() { enumerateTakeoverProcesses = prior })
	if _, err := takeoverComponents(root, root, "owner-a"); err == nil || !strings.Contains(err.Error(), "conflicting") {
		t.Fatalf("conflicting component custody was accepted: %v", err)
	}
}

func TestDeadOwnerTakeoverSweepsPrePublicationWatcher(t *testing.T) {
	reapArmingOwnerProcesses(t)
	root := t.TempDir()
	if err := os.MkdirAll(ownerLockDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	deadCommand := exec.Command("sleep", "30")
	deadCommand.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := deadCommand.Start(); err != nil {
		t.Fatal(err)
	}
	deadExact, state, err := (identity.KernelProber{}).Probe(int64(deadCommand.Process.Pid))
	if err != nil || state != identity.Alive {
		_ = deadCommand.Process.Kill()
		t.Fatalf("read owner before crash: state=%s err=%v", state, err)
	}
	ownerTag := "metasystem-supervision-owner-test-crashed"
	deadOwner := ArmingOwner{
		Pid: deadExact.Pid, PidStartedAt: deadExact.StartedAt.Unix(), PidStartTicks: deadExact.StartTicks,
		BootID: deadExact.BootID, InstanceTag: ownerTag,
	}
	if err := WriteArmingOwner(root, deadOwner); err != nil {
		t.Fatal(err)
	}
	if err := deadCommand.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err := deadCommand.Wait(); err == nil {
		t.Fatal("the deliberately crashed owner exited successfully")
	}

	watcherTag := ownerTag + "-watcher-1"
	componentArgs := []string{
		"-test.run=^TestTakeoverComponentHelper$", "--", "--takeover-component-helper",
		"supervise", "component", "--component", "watcher", "--tag", watcherTag, "--repo", root,
	}
	componentCommand := exec.Command(os.Args[0], componentArgs...)
	componentCommand.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := componentCommand.Start(); err != nil {
		t.Fatal(err)
	}
	componentDone := make(chan error, 1)
	go func() { componentDone <- componentCommand.Wait() }()
	t.Cleanup(func() {
		_ = componentCommand.Process.Kill()
		select {
		case <-componentDone:
		case <-time.After(time.Second):
		}
	})
	componentExact, state, err := (identity.KernelProber{}).Probe(int64(componentCommand.Process.Pid))
	if err != nil || state != identity.Alive {
		t.Fatalf("read pre-publication watcher: state=%s err=%v", state, err)
	}
	prior := enumerateTakeoverProcesses
	enumerateTakeoverProcesses = func(string) ([]census.Process, error) {
		return []census.Process{{
			Pid: componentExact.Pid, PGID: componentExact.Pid, Started: componentExact.StartedAt.Unix(),
			StartTicks: componentExact.StartTicks, BootID: componentExact.BootID, Alive: true,
			Argv: strings.Join(componentExact.Argv, " "),
		}}, nil
	}
	t.Cleanup(func() { enumerateTakeoverProcesses = prior })

	result, err := EnsureArmed(armingOptions(root))
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != "taken-over" || !result.Inspection.Armed() || result.Generation != 1 {
		t.Fatalf("dead-owner takeover did not establish a verified generation: %+v", result)
	}
	select {
	case <-componentDone:
	case <-time.After(2 * time.Second):
		t.Fatal("the pre-publication watcher survived the takeover sweep")
	}
	enumerateTakeoverProcesses = func(string) ([]census.Process, error) { return nil, nil }
	if err := ShutdownAt(root, root, "metasystem-supervision-owner-test-", 1); err != nil {
		t.Fatal(err)
	}
}

func TestLiveGenerationReplacementStopsAndReplacesTheRecordedOwner(t *testing.T) {
	reapArmingOwnerProcesses(t)
	root := t.TempDir()
	prior := enumerateTakeoverProcesses
	enumerateTakeoverProcesses = func(string) ([]census.Process, error) { return nil, nil }
	t.Cleanup(func() { enumerateTakeoverProcesses = prior })
	options := armingOptions(root)
	started, err := EnsureArmed(options)
	if err != nil || started.Action != "started" || !started.Inspection.Armed() {
		t.Fatalf("initial generation: result=%+v err=%v", started, err)
	}
	verified, err := EnsureArmed(options)
	if err != nil || verified.Action != "verified" || verified.Owner.Pid != started.Owner.Pid {
		t.Fatalf("matching generation did not join its owner: result=%+v err=%v", verified, err)
	}
	options.OnlyIfDown = true
	notNeeded, err := EnsureArmed(options)
	if err != nil || notNeeded.Action != "not-needed" || notNeeded.Owner.Pid != started.Owner.Pid {
		t.Fatalf("recovery-only arming replaced a live owner: result=%+v err=%v", notNeeded, err)
	}
	options.OnlyIfDown = false
	options.Fingerprint = "fingerprint-b"
	replaced, err := EnsureArmed(options)
	if err != nil {
		t.Fatal(err)
	}
	if replaced.Action != "replaced" || replaced.Owner.Pid == started.Owner.Pid || replaced.Generation != 2 || !replaced.Inspection.Armed() {
		t.Fatalf("older generation was not replaced: first=%+v replacement=%+v", started, replaced)
	}
	if err := Shutdown(root, "foreign-owner-prefix", 1); err == nil || !strings.Contains(err.Error(), "another repository") {
		t.Fatalf("shutdown accepted a foreign owner prefix: %v", err)
	}
	if err := ShutdownAt(root, root, "metasystem-supervision-owner-test-", 1); err != nil {
		t.Fatal(err)
	}
	if err := Shutdown(root, "metasystem-supervision-owner-test-", 1); err != nil {
		t.Fatalf("repeated shutdown was not idempotent: %v", err)
	}
}

func TestLiveOwnerWithoutPublishedGenerationRefusesRecoveryJoin(t *testing.T) {
	root := t.TempDir()
	ownerTag := "metasystem-supervision-owner-test-unpublished"
	command := exec.Command(os.Args[0], "-test.run=^TestTakeoverComponentHelper$", "--", "--takeover-component-helper", ownerTag)
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = command.Process.Kill(); _, _ = command.Process.Wait() })
	exact, state, err := (identity.KernelProber{}).Probe(int64(command.Process.Pid))
	if err != nil || state != identity.Alive {
		t.Fatalf("read unpublished owner: state=%s err=%v", state, err)
	}
	if err := os.MkdirAll(ownerLockDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	owner := ArmingOwner{
		Pid: exact.Pid, PidStartedAt: exact.StartedAt.Unix(), PidStartTicks: exact.StartTicks,
		BootID: exact.BootID, InstanceTag: ownerTag,
	}
	if err := WriteArmingOwner(root, owner); err != nil {
		t.Fatal(err)
	}
	options := armingOptions(root)
	options.OnlyIfDown = true
	if _, err := EnsureArmed(options); err == nil || !strings.Contains(err.Error(), "did not publish a verifiable generation") {
		t.Fatalf("an unpublished live owner was joined: %v", err)
	}
	read, err := ReadArmingOwner(root)
	if err != nil || !sameArmingOwner(read, owner) {
		t.Fatalf("a refused recovery join changed the owner: owner=%+v err=%v", read, err)
	}
}

func TestGenerationReplacementStopsWhenComponentCensusFails(t *testing.T) {
	reapArmingOwnerProcesses(t)
	root := t.TempDir()
	prior := enumerateTakeoverProcesses
	enumerateTakeoverProcesses = func(string) ([]census.Process, error) { return nil, nil }
	t.Cleanup(func() { enumerateTakeoverProcesses = prior })
	options := armingOptions(root)
	started, err := EnsureArmed(options)
	if err != nil || started.Action != "started" || !started.Inspection.Armed() {
		t.Fatalf("initial generation: result=%+v err=%v", started, err)
	}
	enumerateTakeoverProcesses = func(string) ([]census.Process, error) { return nil, os.ErrPermission }
	options.Fingerprint = "replacement-fingerprint"
	if _, err := EnsureArmed(options); !errors.Is(err, os.ErrPermission) || !strings.Contains(err.Error(), "generation replacement refused") {
		t.Fatalf("component census failure did not stop generation replacement: %v", err)
	}
	enumerateTakeoverProcesses = func(string) ([]census.Process, error) { return nil, nil }
	if err := ShutdownAt(root, root, "metasystem-supervision-owner-test-", 1); err != nil {
		t.Fatal(err)
	}
}

func TestDeadOwnerLockReleaseIsFencedByTheRecordedIdentity(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(ownerLockDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	owner := ArmingOwner{Pid: 41, PidStartedAt: 100, InstanceTag: "owner-a"}
	if err := WriteArmingOwner(root, owner); err != nil {
		t.Fatal(err)
	}
	other := owner
	other.InstanceTag = "owner-b"
	if err := releaseDeadOwnerLock(root, other); err == nil || !strings.Contains(err.Error(), "another owner") {
		t.Fatalf("a successor's lock was removed: %v", err)
	}
	if err := writeShutdownIntent(root, owner, "test cleanup"); err != nil {
		t.Fatal(err)
	}
	if err := releaseDeadOwnerLock(root, owner); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(ownerLockDir(root)); !os.IsNotExist(err) {
		t.Fatalf("released owner lock still exists: %v", err)
	}
}

func TestArmingUtilitiesPreserveOnlyUnrelatedEnvironmentAndIdentity(t *testing.T) {
	environment := withoutExecutionID([]string{"A=1", "METASYSTEM_EXECUTION_ID=delegated", "B=2"})
	if strings.Join(environment, ",") != "A=1,B=2" {
		t.Fatalf("execution attribution was not isolated: %v", environment)
	}
	legacy := ArmingOwner{Pid: 7, PidStartedAt: 11, InstanceTag: "owner-tag"}
	paired := legacy
	paired.PidStartTicks, paired.BootID = 99, "boot-a"
	shiftedSecond := paired
	shiftedSecond.PidStartedAt++
	if !sameArmingOwner(paired, shiftedSecond) {
		t.Fatal("paired owner identity incorrectly depended on the drift-prone epoch second")
	}
	rebooted := paired
	rebooted.BootID = "boot-b"
	if sameArmingOwner(paired, rebooted) {
		t.Fatal("an owner identity from another boot matched")
	}
	if got := scaledWait(2, 500); got != time.Second {
		t.Fatalf("scaled wait rounded incorrectly: %s", got)
	}
	if got := scaledWait(2, 0); got != 2*time.Second {
		t.Fatalf("default wait scale = %s, want 2s", got)
	}
	if got := processArgument([]string{"--component", "watcher", "--tag"}, "--tag"); got != "" {
		t.Fatalf("a missing argument acquired a value: %q", got)
	}
}
