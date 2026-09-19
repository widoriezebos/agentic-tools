package main

import (
	"bytes"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"testing"
)

const procDefaultSignalsHelperEnv = "GO_WANT_PROC_DEFAULT_SIGNALS_HELPER"

func TestProcDefaultSignalsHelper(t *testing.T) {
	t.Parallel()

	if os.Getenv(procDefaultSignalsHelperEnv) != "1" {
		return
	}
	// The Go runtime may claim signals during startup, so recreate the
	// disposition the launcher supplied before exercising the verb.
	signal.Ignore(syscall.SIGINT, syscall.SIGQUIT, syscall.SIGHUP)
	for index, argument := range os.Args {
		if argument == "--" {
			os.Exit(runProcDefaultSignals(os.Args[index:]))
		}
	}
	os.Exit(runProcDefaultSignals(nil))
}

func TestProcDefaultSignalsRestoresIgnoredSignals(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		script string
		signal syscall.Signal
	}{
		{name: "INT", script: "trap - INT; kill -INT $$; echo alive", signal: syscall.SIGINT},
		{name: "QUIT", script: "trap - QUIT; kill -QUIT $$; echo alive", signal: syscall.SIGQUIT},
	} {
		t.Run(test.name, func(t *testing.T) {
			command := exec.Command("bash", "-c",
				`trap "" INT QUIT; exec "$0" -test.run=^TestProcDefaultSignalsHelper$ "$@"`,
				os.Args[0], "--", "/bin/sh", "-c", test.script)
			command.Env = append(os.Environ(), procDefaultSignalsHelperEnv+"=1")
			output, err := command.CombinedOutput()
			if bytes.Contains(output, []byte("alive")) {
				t.Fatalf("command survived %s with output %q", test.name, output)
			}
			exitError, ok := err.(*exec.ExitError)
			if !ok {
				t.Fatalf("command error = %v, want termination by %s; output %q", err, test.name, output)
			}
			status, ok := exitError.Sys().(syscall.WaitStatus)
			if !ok || !status.Signaled() || status.Signal() != test.signal {
				t.Fatalf("command status = %v, want termination by %s; output %q", exitError.Sys(), test.name, output)
			}
		})
	}

	t.Run("refusals", func(t *testing.T) {
		code, _, stderr := captureCommandOutput(t, false, true, func() int {
			return runProcDefaultSignals([]string{"--"})
		})
		if code != 2 || !strings.Contains(stderr, "a command is required after --") {
			t.Fatalf("proc default-signals without a command = exit %d, stderr %q; want exit 2 and refusal text", code, stderr)
		}
		code, _, stderr = captureCommandOutput(t, false, true, func() int {
			return runProcDefaultSignals([]string{"--", "/nonexistent/metasystem-no-such-command"})
		})
		if code != 127 || !strings.Contains(stderr, "/nonexistent/metasystem-no-such-command") {
			t.Fatalf("proc default-signals with an absent command = exit %d, stderr %q; want exit 127 and command path", code, stderr)
		}
	})
}
