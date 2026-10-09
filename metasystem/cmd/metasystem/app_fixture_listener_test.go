package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/applaunch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/lifecycle"
)

// configureRun hands a held listener through the real supervisor and child
// spawns. Readiness comes from the application's FIFO acknowledgement.
func (b *appBed) configureRun(run *appRun) {
	run.stopWait = fixtureDeadlineRemaining(b.t)
	run.probe = func(contract applaunch.Contract, address string) error {
		return applaunch.ProbeWithTimeout(contract, address, fixtureDeadlineRemaining(b.t))
	}
	var held *appListener
	if run.ref == "" && b.listener != nil {
		run.allocate = func(run *appRun) error {
			held, b.listener = b.listener, nil
			file, err := held.listener.File()
			if err != nil {
				return err
			}
			run.address, run.extraFiles = held.address, []*os.File{file}
			b.t.Cleanup(func() { file.Close() })
			return nil
		}
	}
	run.spawn = func(spec applaunch.LaunchSpec) (lifecycle.Child, error) {
		// A deliberately missing invocation engine still reaches the real spawn.
		if spec.Executable != b.fixtureEngine {
			child, err := applaunch.ExecSpawn(spec)
			for _, file := range spec.ExtraFiles {
				file.Close()
			}
			if held != nil {
				held.listener.Close()
			}
			return child, err
		}
		gate := filepath.Join(b.t.TempDir(), "application-ready")
		makeFixtureFIFO(b.t, gate)
		binary, err := os.Executable()
		if err != nil {
			return nil, err
		}
		wrapper := filepath.Join(b.t.TempDir(), "fixture-engine")
		listenerEnv := ""
		if len(spec.ExtraFiles) != 0 {
			listenerEnv = "export METASYSTEM_FIXTURE_HELD_LISTENER=1\n"
		}
		script := "#!/bin/sh\nexport GO_WANT_BATCH_E2E_COMMAND=1\n" + listenerEnv + "exec " + shellQuote(binary) + " fixture-app-serve " + shellQuote(gate) + " \"$@\"\n"
		if err := testexec.WriteFile(wrapper, []byte(script), 0o700); err != nil {
			return nil, err
		}
		spec.Executable = wrapper
		child, err := applaunch.ExecSpawn(spec)
		// The child owns the inherited descriptors before the parent closes them.
		for _, file := range spec.ExtraFiles {
			file.Close()
		}
		if held != nil {
			held.listener.Close()
		}
		return child, err
	}
}

// The fixture supervisor enters the same internal serve owner as the engine.
func init() { testHelperCommands["fixture-app-serve"] = fixtureAppServe }

func fixtureAppServe(args []string) int {
	gate := args[0]
	var readyError error
	code := runAppServeWithDependencies(args[3:], os.Stdout, os.Stderr, func(options *applaunch.SuperviseOptions) {
		options.Spawn = func(spec applaunch.ChildSpec) (applaunch.Child, error) {
			if os.Getenv("METASYSTEM_FIXTURE_HELD_LISTENER") == "1" {
				listener := os.NewFile(4, "held application listener")
				defer listener.Close()
				spec.ExtraFiles = []*os.File{listener}
				spec.Argv = append(spec.Argv, "--listen-fd", "3")
			}
			spec.Argv = append(spec.Argv, "--ready-fifo", gate)
			return applaunch.ExecChild(spec)
		}
		options.ReadyDeadline = func(time.Duration) <-chan time.Time {
			reader, err := os.Open(gate)
			if err != nil {
				readyError = err
				return closedFixtureDeadline()
			}
			defer reader.Close()
			var event [6]byte
			if _, err := io.ReadFull(reader, event[:]); err != nil {
				readyError = err
				return closedFixtureDeadline()
			}
			if string(event[:]) != "ready\n" {
				readyError = fmt.Errorf("application readiness = %q", event)
				return closedFixtureDeadline()
			}
			return nil
		}
	})
	if readyError != nil {
		fmt.Fprintln(os.Stderr, readyError)
		return 1
	}
	return code
}

func closedFixtureDeadline() <-chan time.Time {
	expired := make(chan time.Time)
	close(expired)
	return expired
}

// A held address remains unavailable to another process until the real child
// has inherited it, and the public stop closes the last descriptor.
func TestAppStartTransfersHeldListener(t *testing.T) {
	t.Parallel()
	held := appHeldPort(t)
	bed := newAppBed(t, appHTTPContract(appFixtureApp(t), held))
	competitor, err := net.Listen("tcp", held.address)
	if err == nil {
		competitor.Close()
		t.Fatal("the fixture released its address before start")
	}
	if code, out := bed.run("app", "start"); code != 0 {
		t.Fatalf("held-listener start: %d\n%s", code, out)
	}
	if !answered(held.address) {
		t.Fatal("the application did not inherit the held listener")
	}
	if code, out := bed.run("app", "stop"); code != 0 {
		t.Fatalf("held-listener stop: %d\n%s", code, out)
	}
	if answered(held.address) {
		t.Fatal("the stopped application still holds its listener")
	}
}

func appExitGate(t *testing.T) (string, func()) {
	t.Helper()
	gate := filepath.Join(t.TempDir(), "application-exit")
	makeFixtureFIFO(t, gate)
	held, err := os.OpenFile(gate, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { held.Close() })
	return gate, func() {
		if _, err := held.Write([]byte("exit\n")); err != nil {
			t.Fatal(err)
		}
	}
}

// appAllocationPorts holds a standing port and three adjacent range ports.
// The caller releases only the next port it expects the production walk to use.
func appAllocationPorts(t *testing.T) []*appListener {
	t.Helper()
	for {
		standing := appHeldPort(t)
		host, port, err := net.SplitHostPort(standing.address)
		if err != nil {
			t.Fatal(err)
		}
		low, err := strconv.Atoi(port)
		if err != nil {
			t.Fatal(err)
		}
		ports := []*appListener{standing}
		if low <= 65532 {
			for offset := 1; offset <= 3; offset++ {
				address := net.JoinHostPort(host, strconv.Itoa(low+offset))
				listener, err := net.Listen("tcp", address)
				if err != nil {
					break
				}
				held := &appListener{listener: listener.(*net.TCPListener), address: address}
				t.Cleanup(func() { held.listener.Close() })
				ports = append(ports, held)
			}
		}
		if len(ports) == 4 {
			return ports
		}
		for _, held := range ports {
			held.listener.Close()
		}
	}
}

func appAllocationRange(ports []*appListener) string {
	_, low, _ := net.SplitHostPort(ports[1].address)
	_, high, _ := net.SplitHostPort(ports[3].address)
	return low + "-" + high
}
