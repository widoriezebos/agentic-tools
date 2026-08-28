package identity

import (
	"errors"
	"runtime"
	"testing"
	"time"
)

type fixedFixtureKernel struct {
	exact Exact
	state Liveness
	err   error
}

func (r fixedFixtureKernel) ReadStart(int64) (Exact, Liveness, error) {
	return r.exact, r.state, r.err
}

func nativeFixtureEntry(generation int64) FixtureEntry {
	entry := FixtureEntry{StartedAt: 100, HasStartedAt: true}
	if runtime.GOOS == "linux" {
		entry.StartTicks = 7000 + generation
		entry.HasStartTicks = true
		entry.BootID = "boot-fixture"
		entry.HasBootID = true
		return entry
	}
	entry.StartedAtExactMicro = 100_000_000 + generation
	entry.HasStartedAtExactMicro = true
	return entry
}

func TestFixtureStartReaderUsesAuthorizedExactIdentity(t *testing.T) {
	kernel := fixedFixtureKernel{
		exact: Exact{Pid: 41, StartedAt: time.Unix(200, 0)}, state: Alive,
	}
	reader := FixtureStartReader{
		Kernel:  kernel,
		Fixture: mapProbe{"41": nativeFixtureEntry(1)},
	}
	exact, state, err := reader.ReadStart(41)
	if err != nil || state != Alive {
		t.Fatalf("fixture read: state=%s err=%v", state, err)
	}
	if ref := exact.Ref(); !ref.NativeExact() || ref.StartedAtSec != 100 {
		t.Fatalf("fixture exact identity was not returned: %+v", ref)
	}
}

func TestFixtureStartReaderFallsBackAndHonorsKernelDeath(t *testing.T) {
	kernelExact := Exact{Pid: 41, StartedAt: time.Unix(200, 0)}
	reader := FixtureStartReader{Kernel: fixedFixtureKernel{exact: kernelExact, state: Alive}}
	exact, state, err := reader.ReadStart(41)
	if err != nil || state != Alive || exact.StartedAt.Unix() != 200 {
		t.Fatalf("kernel fallback: exact=%+v state=%s err=%v", exact, state, err)
	}

	reader = FixtureStartReader{
		Kernel:  fixedFixtureKernel{state: Dead},
		Fixture: mapProbe{"41": nativeFixtureEntry(1)},
	}
	if _, state, err := reader.ReadStart(41); err != nil || state != Dead {
		t.Fatalf("kernel death did not veto fixture: state=%s err=%v", state, err)
	}
}

func TestFixtureStartReaderRejectsMalformedPresentRow(t *testing.T) {
	reader := FixtureStartReader{
		Kernel: fixedFixtureKernel{state: Unknown, err: errors.New("kernel denied")},
		Fixture: mapProbe{"41": {
			StartedAt: 100, HasStartedAt: true,
		}},
	}
	if _, state, err := reader.ReadStart(41); err == nil || state != Unknown {
		t.Fatalf("malformed fixture row: state=%s err=%v", state, err)
	}
}
