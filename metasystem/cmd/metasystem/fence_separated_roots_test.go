package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// On separated roots journal recovery enrolls the ledger fence for the
// installation, whose engine the commit hook runs, and never for the state
// root the journal lives under.
func TestGoalSyncRecoverLedgerFenceSeparatedRoots(t *testing.T) {
	checkout, installation := separatedContextRoots(t)
	checkout, _ = filepath.EvalSymlinks(checkout)
	installation, _ = filepath.EvalSymlinks(installation)
	helmMust(t, os.MkdirAll(filepath.Join(checkout, "plans", "goals"), 0o755),
		os.WriteFile(filepath.Join(checkout, "plans", "goals", "backlog.md"), nil, 0o644))
	var fenced []string
	owners := intentOwners{resolver: stateroot.NewResolver(fakeTop(checkout), noExecutable)}
	owners.dependencies.ensureGuard = func(root string) error {
		fenced = append(fenced, root)
		return errors.New("fence stub")
	}
	command, rest, _ := resolveIntentArgv([]string{"goal", "sync", "--recover"})
	var out bytes.Buffer
	runIntentIn(command, rest, &out, &out, installation, owners)
	if len(fenced) != 1 || fenced[0] != installation || !strings.Contains(out.String(), "fence stub") {
		t.Fatalf("fenced %q, want only %q; output:\n%s", fenced, installation, out.String())
	}
}

// On separated roots the helm return enrolls the ledger fence for the
// installation the layout names, while the state root, the checkout, serves
// the rest.
func TestHelmReturnLedgerFenceSeparatedRoots(t *testing.T) {
	t.Parallel()
	b := newReturnBed(t, "refs/heads/main")
	b.owners.helm.stdinTerminal = func() bool { return false }
	helmMust(t, os.WriteFile(filepath.Join(b.inst, "metasystem.conf"), nil, 0o644))
	var fenced []stateroot.Installation
	b.owners.helm.fence = func(installation stateroot.Installation) error {
		fenced = append(fenced, installation)
		return nil
	}
	code, out := b.run("helm", "return", "--repo", b.inst)
	inst, err := filepath.EvalSymlinks(b.inst)
	helmMust(t, err)
	if code != 0 || !strings.Contains(out, "the ledger hook is enrolled\n") || len(fenced) != 1 || fenced[0].Path() != inst {
		t.Fatalf("return %d fenced %q, want only the installation %q:\n%s", code, fenced, inst, out)
	}
}

// The goal family's --root reaches the fence as a plain string; a directory
// without metasystem.conf is refused before anything is enrolled.
func TestEnsureGuardEnrolledParseInstallationRefusesAStateDirectory(t *testing.T) {
	t.Parallel()
	if err := ensureGuardEnrolled(t.TempDir()); err == nil || !strings.Contains(err.Error(), "is not a metasystem installation") {
		t.Fatalf("a state directory reached the fence: %v", err)
	}
}
