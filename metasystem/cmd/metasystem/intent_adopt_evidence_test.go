package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/adopt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// system adopt prints the owner's evidence-root note and no longer asks the
// person to fill the evidence root: setting it is optional.
func TestSystemAdoptSaysTheEvidenceRootIsOptional(t *testing.T) {
	t.Parallel()
	installation := t.TempDir()
	if err := os.MkdirAll(filepath.Join(installation, "scripts", "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installation, "metasystem.conf"), []byte("metasystem.runtimes=claude\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cwd := t.TempDir()
	note := "evidence root: /home/x/metasystem-evidence/app (default; set evidence.root in metasystem.conf.local to change)"
	owners := defaultIntentOwners()
	owners.resolver = stateroot.NewResolver(func(path string) (string, error) { return path, nil }, os.Executable)
	owners.adopt = func(options adopt.Options) (adopt.Result, error) {
		return adopt.Result{SHA: "0123456789abcdef0123456789abcdef01234567", Target: options.Target, Notes: []string{note}}, nil
	}
	var stdout, stderr bytes.Buffer
	if code := runIntentIn(mustIntentCommand(t, "system adopt"), []string{"app", "--repo", installation}, &stdout, &stderr, cwd, owners); code != 0 {
		t.Fatalf("adopt = %d\n%s%s", code, stdout.String(), stderr.String())
	}
	text := strings.Join(strings.Fields(stdout.String()), " ")
	if !strings.Contains(text, note) ||
		!strings.Contains(text, "3. Optionally set models, tiers and the evidence root in metasystem.conf.local; the defaults above apply until you do.") ||
		strings.Contains(text, "durable evidence root") {
		t.Fatalf("adoption text:\n%s", text)
	}
}
