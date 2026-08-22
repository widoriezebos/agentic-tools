package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

// The command-layer genesis boundary: goalCaller classifies against the
// ONE root it is writing (never a caller-named second root — a root
// the caller chooses is an authority-laundering hole) and applies the
// matrix. A DELEGATE-shaped caller — every
// provisioning flow under agent ancestry looks like one to a virgin
// target — is admitted to genesis exactly when the ledger is
// adoption-shaped, and is refused every holder-only verb; the human
// passes both.

// delegateRoot builds a root whose adapter signatures match this test
// process's own command, so classifying a child of ours there reads
// DELEGATE (the lease package's own fixture pattern).
func delegateRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	command, ok := lease.ProcessCommand(int64(os.Getpid()), nil)
	if !ok {
		t.Skip("cannot read our own command to build a matching signature")
	}
	adapterDir := filepath.Join(root, "scripts/agents/adapters")
	if err := os.MkdirAll(adapterDir, 0o755); err != nil {
		t.Fatal(err)
	}
	line := "match " + regexp.QuoteMeta(command)
	script := "#!/bin/sh\n[ \"$1\" = signature ] && printf '%s\\n' '" + line + "'\n"
	if err := os.WriteFile(filepath.Join(adapterDir, "fake.sh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

// childPid spawns a child whose parent is this process, so its ancestry
// carries our (signature-matched or not) command.
func childPid(t *testing.T) int64 {
	t.Helper()
	cmd := exec.Command("/bin/sleep", "120")
	if err := cmd.Start(); err != nil {
		t.Fatalf("spawn child: %v", err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	return int64(cmd.Process.Pid)
}

func writeLedger(t *testing.T, root, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, "plans"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "plans", "goals.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// stageHumanTerminal pins the caller's controlling-terminal fact:
// HUMAN is positive-only, and a headless test runner must not decide
// these legs differently from a person's desk.
func stageHumanTerminal(t *testing.T, root string, pid int64) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	table := filepath.Join(t.TempDir(), "terminal-table.json")
	if err := os.WriteFile(table, []byte(fmt.Sprintf(`{"%d": {"terminal": true}}`, pid)), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", table)
}

func TestGoalCallerGenesisBoundary(t *testing.T) {
	goalFree := "# Goals\n\n## Goal-free: declared 2026-08-15T12:00:00Z by human over abc\n"
	withGoals := "# Goals\n\n## Current goal: solo — One goal\n- Origin: main\n- Next step: Do.\n"

	t.Run("delegate reconcile on an adoption-shaped root is genesis-admitted", func(t *testing.T) {
		root := delegateRoot(t)
		writeLedger(t, root, goalFree)
		caller, err := goalCaller(root, childPid(t), "reconcile")
		if err != nil {
			t.Fatalf("adoption-shaped genesis must admit a delegate: %v", err)
		}
		if caller.Class != "DELEGATE" || !caller.Genesis || caller.Holder {
			t.Fatalf("want a non-holder DELEGATE genesis caller, got %+v", caller)
		}
	})

	t.Run("delegate reconcile on a goal-bearing root is refused", func(t *testing.T) {
		root := delegateRoot(t)
		writeLedger(t, root, withGoals)
		if _, err := goalCaller(root, childPid(t), "reconcile"); err == nil ||
			!strings.Contains(err.Error(), "genesis admits a non-holder") {
			t.Fatalf("a goal-bearing ledger must refuse a delegate genesis: %v", err)
		}
	})

	t.Run("delegate open is holder-only refused", func(t *testing.T) {
		root := delegateRoot(t)
		if _, err := goalCaller(root, childPid(t), "open"); err == nil ||
			!strings.Contains(err.Error(), "lease holder") {
			t.Fatalf("open must stay holder-only for a delegate: %v", err)
		}
	})

	t.Run("human passes genesis and holder-only alike", func(t *testing.T) {
		root := t.TempDir()
		writeLedger(t, root, goalFree)
		pid := childPid(t)
		stageHumanTerminal(t, root, pid)
		caller, err := goalCaller(root, pid, "reconcile")
		if err != nil {
			t.Fatalf("human genesis: %v", err)
		}
		if caller.Class != "HUMAN" || !caller.Genesis {
			t.Fatalf("want a HUMAN genesis caller, got %+v", caller)
		}
		if _, err := goalCaller(root, pid, "open"); err != nil {
			t.Fatalf("human open: %v", err)
		}
	})

	t.Run("a broken shape probe refuses the shape, never the human", func(t *testing.T) {
		root := t.TempDir()
		writeLedger(t, root, goalFree)
		pid := childPid(t)
		stageHumanTerminal(t, root, pid)
		t.Setenv("PATH", t.TempDir()) // no git anywhere
		caller, err := goalCaller(root, pid, "reconcile")
		if err != nil {
			t.Fatalf("the human must keep genesis when the probe cannot run: %v", err)
		}
		if caller.Class != "HUMAN" || !caller.Genesis {
			t.Fatalf("want a HUMAN genesis caller, got %+v", caller)
		}
	})

	t.Run("a broken shape probe names itself in a machinery refusal", func(t *testing.T) {
		root := delegateRoot(t)
		writeLedger(t, root, goalFree)
		t.Setenv("PATH", t.TempDir())
		_, err := goalCaller(root, childPid(t), "reconcile")
		if err == nil || !strings.Contains(err.Error(), "adoption-shape probe failed") {
			t.Fatalf("a delegate refused on a broken probe must see the probe error: %v", err)
		}
	})

	t.Run("an accepted baseline ends genesis mode", func(t *testing.T) {
		root := delegateRoot(t)
		writeLedger(t, root, goalFree)
		if err := os.WriteFile(filepath.Join(root, "plans", "goals-accepted.json"), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := goalCaller(root, childPid(t), "reconcile"); err == nil ||
			!strings.Contains(err.Error(), "lease holder") {
			t.Fatalf("an initialized root must be holder-only even for reconcile: %v", err)
		}
	})
}
