package delegate

import (
	"context"
	"strings"
	"testing"
)

// stubDriver proves the contract is implementable: a minimal
// complete Driver the registry accepts.
type stubDriver struct{}

func (stubDriver) Declaration() Declaration { return Declaration{ProtocolServer: true} }
func (stubDriver) Open(context.Context, OpenRequest) (Session, error) {
	return nil, nil
}

// The registry wiring discipline: empty keys and duplicate
// registrations die at init, lookups name what is missing, and a
// complete driver registers and resolves.
func TestDriverRegistryWiring(t *testing.T) {
	if _, err := DriverFor(Key{"no-such-runtime", "acp"}); err == nil ||
		!strings.Contains(err.Error(), "no complete driver") {
		t.Fatalf("an unregistered driver lookup must refuse by name: %v", err)
	}
	mustPanic(t, "empty driver key", func() { RegisterDriver(Key{}, nil) })
	mustPanic(t, "missing transport", func() { RegisterDriver(Key{Runtime: "stub"}, stubDriver{}) })
	// The two-dimensional key holds a native driver and an emulator
	// for the SAME runtime side by side — the coexistence the ratified
	// (runtime, transport) identity exists for.
	RegisterDriver(Key{"stub-fixture", "acp"}, stubDriver{})
	RegisterDriver(Key{"stub-fixture", "legacy"}, stubDriver{})
	d, err := DriverFor(Key{"stub-fixture", "acp"})
	if err != nil || !d.Declaration().ProtocolServer {
		t.Fatalf("a registered complete driver must resolve: %v", err)
	}
	mustPanic(t, "duplicate driver key", func() { RegisterDriver(Key{"stub-fixture", "acp"}, stubDriver{}) })
}

// Port registration merges the two owners' halves under one key and
// refuses a field registered twice.
func TestPortMergeAndCollision(t *testing.T) {
	key := "merge-fixture"
	RegisterPorts(key, Ports{Usage: func(_, _ string) error { return nil }})
	RegisterPorts(key, Ports{HostReturn: func(_, _ string) error { return nil }})
	p, err := PortsFor(key)
	if err != nil || p.Usage == nil || p.HostReturn == nil {
		t.Fatalf("both halves must survive the merge: %+v %v", p, err)
	}
	if p.Settle != nil {
		t.Fatal("unregistered operations stay nil")
	}
	mustPanic(t, "field collision", func() {
		RegisterPorts(key, Ports{Usage: func(_, _ string) error { return nil }})
	})
	// Slice three's two fields obey the same law: explicit merge
	// branch, collision panic, nil-refusal by absence.
	RegisterPorts(key, Ports{Events: func(string) ([]Event, error) { return nil, nil }})
	RegisterPorts(key, Ports{HostBoundaryAskEvents: func(string) ([]Event, error) { return nil, nil }})
	p, err = PortsFor(key)
	if err != nil || p.Events == nil || p.HostBoundaryAskEvents == nil {
		t.Fatalf("slice-three fields must survive the merge: %v", err)
	}
	mustPanic(t, "Events collision", func() {
		RegisterPorts(key, Ports{Events: func(string) ([]Event, error) { return nil, nil }})
	})
	mustPanic(t, "HostBoundaryAskEvents collision", func() {
		RegisterPorts(key, Ports{HostBoundaryAskEvents: func(string) ([]Event, error) { return nil, nil }})
	})
	if _, err := PortsFor("no-such-runtime"); err == nil ||
		!strings.Contains(err.Error(), "no ports registered") {
		t.Fatalf("an unregistered ports lookup must refuse by name: %v", err)
	}
}

func mustPanic(t *testing.T, what string, run func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatalf("%s must panic at init time", what)
		}
	}()
	run()
}
