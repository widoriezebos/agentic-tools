package steward

// U5a (engine-owns-disk-lifetimes Part B, 3.5 "Engine pins", DL3B-08): the
// preparation lease and the pin rule.

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"golang.org/x/sys/unix"
)

var pinNow = time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)

type pinProber map[int64]identity.Liveness

func (p pinProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	state, ok := p[pid]
	if !ok {
		state = identity.Dead
	}
	return identity.Exact{Pid: pid, StartedAt: pinNow}, state, nil
}

func writePin(t *testing.T, top string, generation int, digit string) string {
	t.Helper()
	path := filepath.Join(top, "artifacts", "agents", "steward", "engine-pins", fmt.Sprintf("generation-%d-%s", generation, strings.Repeat(digit, 64)))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(path, []byte("#!/bin/sh\n"), 0o500); err != nil {
		t.Fatal(err)
	}
	// Every preparation creates the preparation lock beside its pins.
	if err := os.WriteFile(filepath.Join(filepath.Dir(path), ".prepare.flock"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeComponent(t *testing.T, top, name string, generation int, pid int64) {
	t.Helper()
	path := filepath.Join(top, "artifacts", "agents", "steward", "components", name+".json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(ComponentEvidence{Component: name, Generation: generation, Pid: pid, PidStartedAt: pinNow.Unix()})
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

// censusReader answers the use census from fields: complete, one engine
// process whose executable is exe, or one unreadable pid.
func censusReader(exe string, gap bool) *diskstore.CensusReader {
	reader := diskstore.CensusReader{
		UID:        501,
		Pids:       func() ([]int64, error) { return []int64{4242}, nil },
		ProcessUID: func(int64) (uint32, bool) { return 501, true },
		Use: func(int64) (identity.ProcessUse, error) {
			if gap {
				return identity.ProcessUse{}, fmt.Errorf("permission denied")
			}
			return identity.ProcessUse{Cwd: "/", Executable: exe}, nil
		},
		Command: func(int64) string { return "metasystem steward" },
	}
	return &reader
}

func pinPass(top string, census *diskstore.CensusReader, class PinClass) diskstore.PassOptions {
	registry := diskstore.CheckoutRegistry(top)
	return diskstore.PassOptions{Kind: "checkout", Name: top, Registry: registry, LockPath: filepath.Join(registry.Dir, ".sweep.flock"),
		ReportPath: diskstore.CheckoutReportPath(top), Mode: diskstore.ModeApply, Now: pinNow, Clock: func() time.Time { return pinNow },
		Entropy: rand.Reader, Classes: []diskstore.Class{class}, CensusReader: census}
}

func exists(t *testing.T, path string) bool {
	t.Helper()
	_, err := os.Stat(path)
	return err == nil
}

// Four pins (installed, live component, held by a preparer, stale): the
// pass keeps three and removes the stale one; a fifth run by a live
// process is kept too.
func TestThePinRuleKeepsEveryPinSomethingUses(t *testing.T) {
	t.Parallel()
	top := canonicalPath(t.TempDir())
	installed := writePin(t, top, 7, "a")
	component := writePin(t, top, 6, "b")
	prepared := writePin(t, top, 5, "c")
	running := writePin(t, top, 4, "d")
	stale := writePin(t, top, 3, "e")
	deadComponent := writePin(t, top, 2, "f")
	writeComponent(t, top, "supervisor", 6, 100)
	writeComponent(t, top, "old-runner", 2, 200)
	lease, err := os.Open(prepared)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	if err := unix.Flock(int(lease.Fd()), unix.LOCK_SH); err != nil {
		t.Fatal(err)
	}
	class := PinClass{Top: top, Prober: pinProber{100: identity.Alive},
		Installed: func() (InstallIdentity, error) {
			return InstallIdentity{Generation: 7, InstallDigest: "sha256:" + strings.Repeat("a", 64), MintedAt: pinNow.Add(-48 * time.Hour).Format(time.RFC3339)}, nil
		}, Grace: 24 * time.Hour}
	if _, err := diskstore.RunPass(context.Background(), pinPass(top, censusReader(running, false), class)); err != nil {
		t.Fatal(err)
	}
	for _, kept := range []string{installed, component, prepared, running} {
		if !exists(t, kept) {
			t.Fatalf("%s must be kept", filepath.Base(kept))
		}
	}
	for _, gone := range []string{stale, deadComponent} {
		if exists(t, gone) {
			t.Fatalf("%s is used by nothing and goes", filepath.Base(gone))
		}
	}
}

func TestThePinRuleRemovesNothingWithoutACompleteCensusOrWhilePreparing(t *testing.T) {
	t.Parallel()
	for name, setup := range map[string]func(t *testing.T, top string) (*diskstore.CensusReader, func()){
		"an incomplete census": func(t *testing.T, top string) (*diskstore.CensusReader, func()) {
			return censusReader("", true), func() {}
		},
		"a held .prepare.flock": func(t *testing.T, top string) (*diskstore.CensusReader, func()) {
			lock, err := os.OpenFile(filepath.Join(top, "artifacts", "agents", "steward", "engine-pins", ".prepare.flock"), os.O_CREATE|os.O_RDWR, 0o600)
			if err != nil {
				t.Fatal(err)
			}
			if err := unix.Flock(int(lock.Fd()), unix.LOCK_EX); err != nil {
				t.Fatal(err)
			}
			return censusReader("/elsewhere", false), func() { _ = lock.Close() }
		},
		"an unreadable component record": func(t *testing.T, top string) (*diskstore.CensusReader, func()) {
			path := filepath.Join(top, "artifacts", "agents", "steward", "components", "torn.json")
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("{torn"), 0o600); err != nil {
				t.Fatal(err)
			}
			return censusReader("/elsewhere", false), func() {}
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			top := canonicalPath(t.TempDir())
			stale := writePin(t, top, 3, "e")
			reader, done := setup(t, top)
			defer done()
			class := PinClass{Top: top, Prober: pinProber{}, Installed: func() (InstallIdentity, error) { return InstallIdentity{}, os.ErrNotExist }}
			report, err := diskstore.RunPass(context.Background(), pinPass(top, reader, class))
			if err != nil {
				t.Fatal(err)
			}
			if !exists(t, stale) {
				t.Fatal("nothing is removed")
			}
			if len(report.Pending) == 0 {
				t.Fatalf("the report says why the class waits: %+v", report)
			}
		})
	}
}

// The preparation lease lasts from PrepareForExecution to Close: a command
// prepared and not yet started keeps its pin through a generation
// replacement and a sweep, and starts; Close releases the lease even after
// a failed start; two commands under one lease both start.
func TestThePreparationLeaseKeepsAPreparedPinUntilClose(t *testing.T) {
	t.Parallel()
	root, _ := makeEnrolledBinaryFixture(t, 3, []byte("#!/bin/sh\nexit 0\n"))
	pinned, err := OpenEnrolledBinary(root)
	if err != nil {
		t.Fatal(err)
	}
	// The pin is written with forks excluded, so no parallel test's child
	// holds its write descriptor when it is executed (ETXTBSY).
	if err := testexec.Locked(pinned.PrepareForExecution); err != nil {
		t.Fatal(err)
	}
	first, err := pinned.Command()
	if err != nil {
		t.Fatal(err)
	}
	pin := pinned.execPath
	// The generation is replaced: generation 3 is no longer installed.
	class := PinClass{Top: root, Prober: pinProber{},
		Installed: func() (InstallIdentity, error) {
			return InstallIdentity{Generation: 4, InstallDigest: "sha256:" + strings.Repeat("9", 64), MintedAt: pinNow.Add(-48 * time.Hour).Format(time.RFC3339)}, nil
		}, Grace: 24 * time.Hour}
	if _, err := diskstore.RunPass(context.Background(), pinPass(root, censusReader("/elsewhere", false), class)); err != nil {
		t.Fatal(err)
	}
	if !exists(t, pin) {
		t.Fatal("a prepared, unstarted command's pin is kept by its lease")
	}
	second, err := pinned.Command()
	if err != nil {
		t.Fatal(err)
	}
	for _, command := range []interface{ Run() error }{first, second} {
		if err := command.Run(); err != nil {
			t.Fatalf("both commands under one lease start: %v", err)
		}
	}
	failed, err := pinned.Command("unused")
	if err != nil {
		t.Fatal(err)
	}
	failed.Path = filepath.Join(root, "no-such-engine")
	if err := failed.Start(); err == nil {
		t.Fatal("the fixture's start should fail")
	}
	if err := pinned.Close(); err != nil {
		t.Fatal(err)
	}
	if held, err := probePin(pin); err != nil || held {
		t.Fatalf("Close releases the lease after a failed start: held=%v err=%v", held, err)
	}
	if _, err := diskstore.RunPass(context.Background(), pinPass(root, censusReader("/elsewhere", false), class)); err != nil {
		t.Fatal(err)
	}
	if exists(t, pin) {
		t.Fatal("once closed, the old generation's pin goes")
	}
}

// A legacy candidate-engines/<identity> entry is a stray with its idle time
// and is never removed by a pass (DL2-11); the v2 namespace is not.
func TestLegacyCandidateEnginesAreStraysNeverRemoved(t *testing.T) {
	t.Parallel()
	top := canonicalPath(t.TempDir())
	cache := filepath.Join(top, "artifacts", "agents", "candidate-engines")
	legacy := filepath.Join(cache, strings.Repeat("1", 40))
	for _, path := range []string{filepath.Join(legacy, "metasystem"), filepath.Join(cache, "v2", strings.Repeat("2", 40), "metasystem")} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := testexec.WriteFile(path, []byte("engine"), 0o500); err != nil {
			t.Fatal(err)
		}
		old := pinNow.Add(-72 * time.Hour)
		for _, aged := range []string{path, filepath.Dir(path)} {
			if err := os.Chtimes(aged, old, old); err != nil {
				t.Fatal(err)
			}
		}
	}
	options := pinPass(top, censusReader("/elsewhere", false), PinClass{})
	options.Classes = []diskstore.Class{LegacyCandidateEngines{Top: top}}
	for range 2 {
		report, err := diskstore.RunPass(context.Background(), options)
		if err != nil {
			t.Fatal(err)
		}
		if len(report.Strays) != 1 || report.Strays[0].Path != legacy || report.Strays[0].IdleSecs < 71*3600 {
			t.Fatalf("the legacy entry is one stray with its idle time: %+v", report.Strays)
		}
		if !exists(t, filepath.Join(legacy, "metasystem")) {
			t.Fatal("a pass never removes a legacy candidate engine")
		}
	}
}

// Round D2 F-2: an older engine's test run holds no lease on its policy
// engine's pin between its plan and its first worker, so a pin stays for
// disk.pin-grace-hours after its generation stopped being installed (a
// re-arm N -> N+1 during an old run's build gap keeps pin N).
func TestAReplacedGenerationsPinStaysForTheGrace(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct {
		mintedAgo time.Duration
		kept      bool
	}{
		"re-armed an hour ago":  {mintedAgo: time.Hour, kept: true},
		"re-armed two days ago": {mintedAgo: 48 * time.Hour, kept: false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			top := canonicalPath(t.TempDir())
			previous := writePin(t, top, 3, "e")
			writePin(t, top, 4, "a")
			class := PinClass{Top: top, Prober: pinProber{}, Grace: 24 * time.Hour,
				Installed: func() (InstallIdentity, error) {
					return InstallIdentity{Generation: 4, InstallDigest: "sha256:" + strings.Repeat("a", 64), MintedAt: pinNow.Add(-c.mintedAgo).Format(time.RFC3339)}, nil
				}}
			report, err := diskstore.RunPass(context.Background(), pinPass(top, censusReader("/elsewhere", false), class))
			if err != nil {
				t.Fatal(err)
			}
			if exists(t, previous) != c.kept {
				t.Fatalf("pin of generation 3 kept=%v, want %v: %+v", exists(t, previous), c.kept, report.Kept)
			}
		})
	}
}

// Rule 1 and Round D2 F-6: what cannot say when a generation stopped being
// installed, or pins without their preparation lock, hold the class.
func TestPinsHoldWhenTheirTimeOrLockCannotBeRead(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct {
		minted  string
		noFlock bool
	}{
		"an installed identity without its minting time": {minted: ""},
		"an unparseable minting time":                    {minted: "yesterday"},
		"an absent .prepare.flock":                       {minted: pinNow.Add(-48 * time.Hour).Format(time.RFC3339), noFlock: true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			top := canonicalPath(t.TempDir())
			stale := writePin(t, top, 3, "e")
			if c.noFlock {
				if err := os.Remove(filepath.Join(filepath.Dir(stale), ".prepare.flock")); err != nil {
					t.Fatal(err)
				}
			}
			class := PinClass{Top: top, Prober: pinProber{}, Grace: 24 * time.Hour,
				Installed: func() (InstallIdentity, error) {
					return InstallIdentity{Generation: 4, InstallDigest: "sha256:" + strings.Repeat("a", 64), MintedAt: c.minted}, nil
				}}
			report, err := diskstore.RunPass(context.Background(), pinPass(top, censusReader("/elsewhere", false), class))
			if err != nil {
				t.Fatal(err)
			}
			if !exists(t, stale) || len(report.Pending) == 0 {
				t.Fatalf("nothing is removed and the class is pending: %+v", report)
			}
		})
	}
}
