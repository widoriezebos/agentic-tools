package proofrun

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
)

type hostSingleTryKey struct{}

var errNoRoomNow = errors.New("no test slot is free")

func withSingleTry(ctx context.Context) context.Context {
	return context.WithValue(ctx, hostSingleTryKey{}, true)
}

// LaneCheckRoom reserves the lane's test slot and passes its custody to checks.
func LaneCheckRoom(controlRoot string, person bool, log io.Writer) (attach func(*exec.Cmd), release func()) {
	// A fake-runtime checkout never reserves the host's real admission.
	if os.Getenv("METASYSTEM_PROOF_ADMISSION_TEST_DIR") == "" && fixtureauth.FixtureModeRoot(controlRoot) {
		return func(*exec.Cmd) {}, func() {}
	}
	return laneCheckRoomWith(func(ctx context.Context) (*HostResourceLease, error) {
		return AcquireHostResources(ctx, controlRoot, filepath.Join(controlRoot, "metasystem.conf"), "heavy", nil)
	}, person, log)
}

func laneCheckRoomWith(acquire func(context.Context) (*HostResourceLease, error), person bool, log io.Writer) (attach func(*exec.Cmd), release func()) {
	ctx := WithHostResourceWaitObserver(WithLaneCheckPriority(context.Background()), func() {
		fmt.Fprintln(log, "landing prove: waiting for room on this host to run tests")
	})
	if person {
		ctx = withSingleTry(ctx)
	}
	lease, err := acquire(ctx)
	if err != nil {
		if errors.Is(err, errNoRoomNow) {
			fmt.Fprintln(log, "landing prove: no test slot was free; this check, asked by a person, runs beside the tests that hold one")
		} else {
			fmt.Fprintf(log, "landing prove: the host's test admission could not be read (%v); this check runs without a slot\n", err)
		}
		return func(*exec.Cmd) {}, func() {}
	}
	return func(cmd *exec.Cmd) { AttachHostResourceLease(WithHostResourceLease(ctx, lease), cmd) }, func() { _ = lease.Close() }
}
