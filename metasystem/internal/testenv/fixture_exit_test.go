package testenv

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"syscall"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

type exitProbeFunc func(int64) (identity.Exact, identity.Liveness, error)

func (f exitProbeFunc) Probe(pid int64) (identity.Exact, identity.Liveness, error) { return f(pid) }

func TestExitScanReadsNothingWithoutAMintedKey(t *testing.T) {
	exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("probe test process: state=%v err=%v", state, err)
	}
	key := identity.FixtureKey{Owner: exact.Ref(), Test: "TestExitScanHelper", Nonce: "1234abcd"}
	for _, test := range []struct {
		name                                    string
		keys                                    []identity.FixtureKey
		class                                   identity.FixtureSurvivorClass
		scanErr                                 error
		inputCode, wantCode, wantScan, wantSent int
		zombie, wantNamed                       bool
	}{
		{name: "no key", inputCode: 7, wantCode: 7},
		{name: "certain running survivor", keys: []identity.FixtureKey{key}, class: identity.FixtureSurvivorCertain, wantCode: 1, wantScan: 1, wantSent: 1, wantNamed: true},
		{name: "zombie", keys: []identity.FixtureKey{key}, class: identity.FixtureSurvivorCertain, zombie: true, wantScan: 1},
		{name: "scan error", keys: []identity.FixtureKey{key}, scanErr: errors.New("process table unavailable"), wantCode: 1, wantScan: 1, wantNamed: true},
		{name: "unproven survivor", keys: []identity.FixtureKey{key}, class: identity.FixtureSurvivorUnreadable, wantCode: 1, wantScan: 1, wantNamed: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			running, scans, sent := true, 0, 0
			prober := exitProbeFunc(func(int64) (identity.Exact, identity.Liveness, error) {
				if !running {
					return identity.Exact{}, identity.Dead, nil
				}
				observed := exact
				observed.Zombie = test.zombie
				return observed, identity.Alive, nil
			})
			scan := func(identity.FixtureKey) ([]identity.FixtureSurvivor, error) {
				scans++
				if test.scanErr != nil {
					return nil, test.scanErr
				}
				return []identity.FixtureSurvivor{{Class: test.class, Ref: exact.Ref(), Exe: "/tmp/child", Argv: []string{"child", "arg"}}}, nil
			}
			signal := func(pid int, signal syscall.Signal) error {
				sent++
				running = false
				return nil
			}
			var output bytes.Buffer
			code := exitScan(test.inputCode, test.keys, scan, prober, signal, &output)
			if code != test.wantCode || scans != test.wantScan || sent != test.wantSent || strings.Contains(output.String(), key.Test) != test.wantNamed || test.wantNamed && test.scanErr == nil && !strings.Contains(output.String(), "pid=") {
				t.Fatalf("code=%d scans=%d sent=%d named=%v output=%q", code, scans, sent, strings.Contains(output.String(), key.Test), output.String())
			}
		})
	}
}
