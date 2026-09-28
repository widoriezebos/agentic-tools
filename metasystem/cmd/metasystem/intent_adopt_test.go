package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// The public adoption action: its help names the adoptable runtimes, and every
// refusal a person can meet before anything is written names the way forward.
// The adoption owner's behavior is proven in internal/adopt.
func TestSystemAdoptHelpAndRefusalsBeforeAnyEffect(t *testing.T) {
	t.Parallel()
	command := mustIntentCommand(t, "system adopt")
	var help, problem bytes.Buffer
	if code := runIntentIn(command, []string{"--help"}, &help, &problem, t.TempDir(), defaultIntentOwners()); code != 0 || problem.Len() != 0 {
		t.Fatalf("help exited %d: %s", code, problem.String())
	}
	for _, want := range append([]string{"metasystem system adopt TARGET", "--runtimes", "--enable", "--copy-skills", "docs/metasystem-reconciliation.md"}, runtimes.Adoptable()...) {
		if !strings.Contains(help.String(), want) {
			t.Fatalf("help does not name %q:\n%s", want, help.String())
		}
	}

	// The template is found from --repo or the current directory, as for
	// every public action; the repository top is stubbed.
	installation := t.TempDir()
	if err := os.MkdirAll(installation, 0o755); err != nil {
		t.Fatal(err)
	}
	if marker, err := os.OpenFile(filepath.Join(installation, "metasystem.conf"), os.O_CREATE|os.O_WRONLY, 0o644); err != nil {
		t.Fatal(err)
	} else {
		marker.Close()
	}
	if err := os.WriteFile(filepath.Join(installation, "metasystem.conf"), []byte("metasystem.runtimes=claude\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cwd := t.TempDir()
	target := filepath.Join(cwd, "app")
	owners := defaultIntentOwners()
	owners.resolver = stateroot.NewResolver(func(path string) (string, error) { return path, nil }, os.Executable)
	for _, test := range []struct {
		name string
		args []string
		want string
		next string
	}{
		{"no target", []string{"--repo", installation}, "name the repository to adopt into", "metasystem system adopt TARGET"},
		{"not in a template", []string{"app"}, "is not inside a metasystem template checkout", "metasystem system adopt app --repo TEMPLATE"},
		{"unknown runtime", []string{"app", "--repo", installation, "--runtimes", "codez"}, "unknown or non-adoptable runtime: codez", "metasystem system adopt TARGET --runtimes"},
		{"empty runtime selection", []string{"app", "--repo", installation, "--runtimes="}, "--runtimes cannot be empty", "metasystem system adopt TARGET --runtimes"},
		{"none mixed", []string{"app", "--repo", installation, "--runtimes", "none,claude"}, "cannot be combined", "metasystem system adopt TARGET --runtimes"},
	} {
		var stdout, stderr bytes.Buffer
		code := runIntentIn(command, append(test.args, "--json"), &stdout, &stderr, cwd, owners)
		var result intentResult
		if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
			t.Fatalf("%s: not one JSON result: %v\n%s%s", test.name, err, stdout.String(), stderr.String())
		}
		if code != 2 || result.Outcome != intentRefused || !strings.Contains(result.Summary, test.want) {
			t.Fatalf("%s: code=%d result=%+v", test.name, code, result)
		}
		if result.Next == nil || !strings.HasPrefix(strings.Join(result.Next.Argv, " "), test.next) {
			t.Fatalf("%s: the refusal names no way forward: %+v", test.name, result.Next)
		}
		if _, err := os.Lstat(target); err == nil {
			t.Fatalf("%s: a refusal before any effect created the target", test.name)
		}
	}
}
