package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

func TestUnitCheckSignals(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"default", "legacy", "bounded", "complete"} {
		for _, phase := range []string{"command", "admission"} {
			for _, sig := range []syscall.Signal{syscall.SIGTERM, syscall.SIGINT} {
				t.Run(kind+"/"+phase+"/"+sig.String(), func(t *testing.T) {
					t.Parallel()
					directory := t.TempDir()
					command := exec.Command(os.Args[0], "-test.run=^TestTheUnitCheckWaitsBehindTheLanesCheckAndHandsOnItsSlot$", "--", "unit-check-signal-child", kind, phase, directory)
					command.Env = testutil.Fixture(t).Env(os.Environ())
					output, err := os.Create(filepath.Join(directory, "output"))
					if err != nil {
						t.Fatal(err)
					}
					defer output.Close()
					command.Stdout, command.Stderr = output, output
					if err := command.Start(); err != nil {
						t.Fatal(err)
					}
					done := make(chan error, 1)
					go func() { done <- command.Wait() }()
					t.Cleanup(func() { _ = command.Process.Kill() })
					testenv.Await(t, "unit check reached signal boundary", func() bool {
						if phase == "admission" {
							paths, _ := filepath.Glob(filepath.Join(directory, "admission", "wait-run-*"))
							return len(paths) > 0
						}
						_, err := os.Stat(filepath.Join(directory, "ready"))
						return err == nil
					})
					if err := command.Process.Signal(sig); err != nil {
						t.Fatal(err)
					}
					var ended bool
					var waitErr error
					testenv.Await(t, "unit check stopped on signal", func() bool {
						select {
						case waitErr = <-done:
							ended = true
						default:
						}
						return ended
					})
					if kind != "bounded" {
						var exit *exec.ExitError
						if !errors.As(waitErr, &exit) {
							t.Fatalf("legacy check did not terminate on signal: %v", waitErr)
						}
						status, ok := exit.Sys().(syscall.WaitStatus)
						if !ok || !status.Signaled() || status.Signal() != sig {
							t.Fatalf("wrong termination: %v", waitErr)
						}
					} else if waitErr != nil {
						t.Fatalf("bounded cancellation fixture failed: %v", waitErr)
					}
					if _, err := os.Stat(filepath.Join(directory, "audits")); !os.IsNotExist(err) {
						t.Fatal("audits ran after cancellation")
					}
				})
			}
		}
	}
}

func runUnitCheckSignalChild(t *testing.T, kind, phase, directory string) {
	hold := filepath.Join(directory, "hold")
	if err := syscall.Mkfifo(hold, 0600); err != nil {
		t.Fatal(err)
	}
	testenv.HoldFIFO(t, hold)
	command := "printf ready > " + shellCommand([]string{filepath.Join(directory, "ready")}) + "; read line < " + shellCommand([]string{hold})
	body := "proof.cheap=" + command + "\nproof.audits=printf ran > " + shellCommand([]string{filepath.Join(directory, "audits")}) + "\nproof.deadline=15\n"
	if kind != "default" {
		declared := kind
		if kind == "complete" {
			declared = "bounded"
		}
		body += "proof.check-kind=" + declared + "\n"
	}
	b := declaredCheckBed(t, body)
	hook := b.workOwnersHook
	b.workOwnersHook = func(o *intentWorkOwners) {
		hook(o)
		git := o.git
		o.git = func(root string, args ...string) ([]byte, error) {
			if slices.Equal(args, []string{"rev-parse", "--show-toplevel"}) {
				return []byte(b.worktree), nil
			}
			if len(args) > 0 && (args[0] == "ls-files" || args[0] == "diff" && slices.Contains(args, "--name-only")) {
				return nil, nil
			}
			return git(root, args...)
		}
		o.checkContext = func(c *launch.CheckContext) {
			c.Context = proofrun.WithResourceCustodyExecutable(c.Context, waitCandidateBuiltHere)
		}
	}
	code, built, _ := b.work("work", "build", b.id, "signal", "--brief", b.brief("signal.md", "Build.\n"), "--lines", "100")
	if code != 0 {
		t.Fatal("fixture build failed")
	}
	if phase == "admission" {
		conf := filepath.Join(b.root(), "metasystem.conf")
		writeTestingFixtureFile(t, conf, []byte("metasystem.runtimes=fake\n"+proofrun.AdmissionCapKey+"=1\n"), 0600)
		t.Setenv("METASYSTEM_PROOF_ADMISSION_TEST_DIR", filepath.Join(directory, "admission"))
		t.Setenv("METASYSTEM_PROOF_ADMISSION_FIXTURE_ROOT", b.root())
		holder, err := proofrun.AcquireHostResources(proofrun.WithLaneCheckPriority(context.Background()), b.root(), conf, "heavy", nil)
		if err != nil {
			t.Fatal(err)
		}
		defer holder.Close()
	}
	engine := filepath.Join(directory, "metasystem")
	if err := testexec.WriteFile(engine, []byte("#!/bin/sh\n"+command+"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	owners := b.workOwners()
	owners.processes.executable = func() (string, error) { return engine, nil }
	owners.lookupEnv = func(key string) (string, bool) {
		if key == launch.KindEnv {
			return "proof", true
		}
		if key == "METASYSTEM_PUBLICATION_BASE" && kind == "complete" {
			return "base", true
		}
		return "", false
	}
	cmd, args, ok := resolveIntentArgv([]string{"test", "run", "--unit-run", resultData(t, built)["run"].(string), "--repo", b.root(), "--json"})
	if !ok {
		t.Fatal("unit check command unavailable")
	}
	var stdout bytes.Buffer
	code = runIntentIn(cmd, args, &stdout, os.Stderr, b.worktree, owners)
	if kind != "bounded" || code != 1 || !strings.Contains(stdout.String(), "cancelled") {
		t.Fatalf("wrong cancellation result: code=%d", code)
	}
}
