package supervise

import (
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

type armingComponentProbe struct {
	exact identity.Exact
	state identity.Liveness
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
