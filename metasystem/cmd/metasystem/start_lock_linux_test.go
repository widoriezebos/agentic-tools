package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"golang.org/x/sys/unix"
)

func TestConcurrentSystemStartsKeepOneOwner(t *testing.T) {
	t.Parallel()
	root := canonicalTestDir(t)
	_, binary, _ := installPendingWaitHookFixture(t, root, intentTestEngine(t))
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\nwatch.interval-sec=1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Dir(binary)
	if err := testexec.WriteFile(filepath.Join(bin, "git"), []byte(`#!/bin/sh
case "$*" in
  *"rev-parse --show-toplevel") dirname "$(dirname "$0")" ;;
  *"--git-common-dir"|*"--git-dir") printf '.git\n' ;;
  *) exit 1 ;;
esac
`), 0755); err != nil {
		t.Fatal(err)
	}
	environment := []string{"PATH=" + bin + ":" + os.Getenv("PATH"), "METASYSTEM_BIN=" + binary, "METASYSTEM_FIXTURE_CAP_SCALE_MILLI=10000"}
	fixture := testutil.Fixture(t)
	t.Cleanup(func() {
		if err := retirePendingWaitSupervision(t, root); err != nil {
			t.Error(err)
		}
		if err := fixture.HoldOwnedChildren(); err != nil {
			t.Error(err)
		}
	})
	start := func() (func() string, func() (bool, string)) {
		cmd := exec.CommandContext(t.Context(), binary, "system", "start", "--if-down")
		cmd.Dir, cmd.Env = root, append(fixture.Env(os.Environ()), environment...)
		var output bytes.Buffer
		cmd.Stdout, cmd.Stderr = &output, &output
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		fixture.Record(cmd.Process.Pid)
		done := make(chan struct{})
		var err error
		go func() { err = cmd.Wait(); close(done) }()
		t.Cleanup(func() { <-done })
		return func() string {
				<-done
				if err != nil {
					t.Fatalf("start exited %v: %s", err, &output)
				}
				return output.String()
			}, func() (bool, string) {
				select {
				case <-done:
					return false, output.String()
				default:
					return true, ""
				}
			}
	}

	// A FIFO at the owner's log pauses the first start inside launchOwner.
	// Let it launch only once the other command has inspected its lock.
	dir := supervise.SupervisionDir(root)
	log := filepath.Join(dir, "owner.log")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := unix.Mkfifo(log, 0600); err != nil {
		t.Fatal(err)
	}
	first, firstAlive := start()
	testenv.AwaitFrom(t, "first start electing the owner", func() bool {
		_, err := os.Stat(filepath.Join(dir, "lock.d"))
		return err == nil
	}, firstAlive, nil)
	fd, err := unix.InotifyInit1(unix.IN_NONBLOCK | unix.IN_CLOEXEC)
	if err != nil {
		t.Fatal(err)
	}
	defer unix.Close(fd)
	record := filepath.Join(supervise.SupervisionDir(root), "cap-authority.lock.d", "owner.json")
	if _, err := unix.InotifyAddWatch(fd, record, unix.IN_CLOSE_NOWRITE); err != nil {
		t.Fatal(err)
	}
	second, secondAlive := start()
	testenv.AwaitFrom(t, "second start inspecting the first", func() bool {
		var event [4096]byte
		n, err := unix.Read(fd, event[:])
		if err != nil && err != unix.EAGAIN && err != unix.EINTR {
			t.Fatal(err)
		}
		return n > 0
	}, secondAlive, nil)
	reader, err := os.OpenFile(log, os.O_RDONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	go func() { _, _ = io.Copy(io.Discard, reader) }()
	started, waited := first(), second()
	owner, err := supervise.ReadArmingOwner(root)
	if err != nil {
		t.Fatal(err)
	}
	for action, output := range map[string]string{"started": started, "not-needed": waited} {
		want := fmt.Sprintf("component=supervision-owner outcome=%s detail=\"pid=%d generation=", action, owner.Pid)
		if !strings.Contains(output, want) || !strings.Contains(output, "up outcome=recovery-"+action) {
			t.Fatalf("missing %q in %s", want, output)
		}
	}
	solo, _ := start()
	if got := solo(); got != waited {
		t.Fatalf("uncontended recovery output changed:\n%s\nconcurrent:\n%s", got, waited)
	}
}
