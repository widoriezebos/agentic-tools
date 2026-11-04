package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

type laneRoomLog struct {
	bytes.Buffer
	wait chan struct{}
}

func (w *laneRoomLog) Write(p []byte) (int, error) {
	n, err := w.Buffer.Write(p)
	if bytes.Contains(p, []byte("waiting for room")) {
		close(w.wait)
	}
	return n, err
}

func TestTheLaneReservesWithItsPriorityThroughItsProductionSeams(t *testing.T) {
	root := t.TempDir()
	conf := filepath.Join(root, "metasystem.conf")
	writeTestingFixtureFile(t, conf, []byte("metasystem.runtimes=fake\n"+proofrun.AdmissionCapKey+"=1\n"), 0o600)
	directory := filepath.Join(root, "admission")
	t.Setenv("METASYSTEM_PROOF_ADMISSION_TEST_DIR", directory)
	t.Setenv("METASYSTEM_PROOF_ADMISSION_FIXTURE_ROOT", root)
	holder, err := proofrun.AcquireHostResources(t.Context(), root, conf, "heavy", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Close()
	seams := laneVerbOwners{plainProve: plain.ProveSeams{Now: func() time.Time { return time.Unix(100, 0) }, Git: func(string, ...string) (string, error) {
		t.Fatal("reserving room consulted Git")
		return "", nil
	}}}.proveSeams(root)
	if seams.Room == nil {
		t.Fatal("production seams have no room reservation")
	}
	log := &laneRoomLog{wait: make(chan struct{})}
	type room struct {
		attach  func(*exec.Cmd)
		release func()
	}
	done := make(chan room, 1)
	go func() { a, r := seams.Room(root, false, log); done <- room{a, r} }()
	// The line is written only after the failed scan publishes its waiter.
	select {
	case <-log.wait:
	case r := <-done:
		r.release()
		t.Fatalf("room returned without waiting: %s", log.String())
	}
	markers, err := filepath.Glob(filepath.Join(directory, "wait-lane-*"))
	if err != nil || len(markers) != 1 {
		_ = holder.Close()
		r := <-done
		r.release()
		t.Fatalf("lane priority waiter absent: %v %v", markers, err)
	}
	marker, err := os.Open(markers[0])
	if err != nil {
		_ = holder.Close()
		r := <-done
		r.release()
		t.Fatal(err)
	}
	probeErr := syscall.Flock(int(marker.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	_ = marker.Close()
	_ = holder.Close()
	r := <-done
	defer r.release()
	if probeErr != syscall.EWOULDBLOCK && probeErr != syscall.EAGAIN {
		t.Fatalf("lane waiter was not held: %v", probeErr)
	}
	ctx, cancel := context.WithCancel(t.Context())
	waited := false
	ctx = proofrun.WithHostResourceWaitObserver(ctx, func() { waited = true; cancel() })
	contender, err := proofrun.AcquireHostResources(ctx, root, conf, "heavy", nil)
	cancel()
	if contender != nil {
		_ = contender.Close()
	}
	if !waited || !errors.Is(err, context.Canceled) {
		t.Fatalf("lane did not hold its slot: waited=%t err=%v", waited, err)
	}
	command := exec.Command("/usr/bin/true")
	r.attach(command)
	if len(command.ExtraFiles) == 0 || !strings.Contains(strings.Join(command.Env, "\n"), "METASYSTEM_PROOF_RESOURCE_FDS=") {
		t.Fatal("production room did not attach custody")
	}
	r.release()
	contender, err = proofrun.AcquireHostResources(t.Context(), root, conf, "heavy", nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = contender.Close()
}
