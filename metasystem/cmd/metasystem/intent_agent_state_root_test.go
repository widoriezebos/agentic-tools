package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
)

// The goal ledger lives at the installation's state root. In a checkout
// whose installation is a subdirectory, agent ask --goal reads the goals
// there: read at the checkout's top, no goal exists and every goal is
// refused as not open.
func TestAgentAskReadsTheGoalsAtTheInstallationBelowTheCheckout(t *testing.T) {
	t.Parallel()
	b := newAgentBed(t, "m1a")
	installation := filepath.Join(b.root, "metasystem")
	if err := os.MkdirAll(installation, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installation, "metasystem.conf"), []byte("metasystem.template=true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	owners := b.owners()
	var read []string
	owners.agent.ledger = func(root string) (board.Ownership, error) {
		read = append(read, root)
		return b.ledger, nil
	}
	command, rest, ok := resolveIntentArgv([]string{"agent", "ask", "--goal", "goal-x", "--text", "are you there?"})
	if !ok {
		t.Fatal("no public command agent ask")
	}
	var stdout, stderr bytes.Buffer
	if code := runIntentIn(command, rest, &stdout, &stderr, b.root, owners); code != 0 {
		t.Fatalf("agent ask --goal goal-x = %d %q %q; want it sent", code, stdout.String(), stderr.String())
	}
	want, err := filepath.EvalSymlinks(installation)
	if err != nil {
		t.Fatal(err)
	}
	for _, root := range read {
		if got, err := filepath.EvalSymlinks(root); err != nil || got != want {
			t.Fatalf("the goals were read at %s; want the installation %s", root, installation)
		}
	}
	if len(read) == 0 {
		t.Fatal("agent ask --goal read no goals")
	}
}
