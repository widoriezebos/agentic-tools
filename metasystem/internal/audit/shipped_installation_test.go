package audit

// Static contract checks over the shipped metasystem installation. These were
// the engine-delivery-contract, metasystem-audit and static-contract-audits
// sections of the retired validate-metasystem.sh (verbs-object-action U7b):
// each reads the real installation tree at ../.. and writes nothing into it.

import (
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func shippedInstallationRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "metasystem.conf")); err != nil {
		t.Fatalf("shipped installation root %s has no metasystem.conf: %v", root, err)
	}
	return root
}

// The shipped installation passes the full metasystem audit as the suite ran
// it: placeholders tolerated, since only the template's surrounding repository
// (absent from a payload-only copy) marks the tree as the template; the
// placeholder rule itself is TestAuditMetasystemRefusals'.
func TestShippedInstallationPassesMetasystemAudit(t *testing.T) {
	t.Parallel()
	root := shippedInstallationRoot(t)
	for _, options := range []AuditOptions{{AllowPlaceholders: true}} {
		result, err := AuditMetasystem(root, options)
		if err != nil {
			t.Fatalf("metasystem audit (allow placeholders=%v) could not run on the shipped installation: %v", options.AllowPlaceholders, err)
		}
		if len(result.Violations) != 0 {
			t.Fatalf("metasystem audit (allow placeholders=%v) refused the shipped installation: %v\n%s",
				options.AllowPlaceholders, result.Violations, strings.Join(result.Report, "\n"))
		}
	}
}

// The metasystem audit is the engine's own work and depends on no external
// command (the retired suite's IL-3 leg ran it under a PATH holding only cat,
// find, grep, sort, tr and wc). Every helper of AuditMetasystem lives in
// metasystem.go, and neither it nor the runtime registry it consults may
// import os/exec.
func TestMetasystemAuditSpawnsNoExternalCommand(t *testing.T) {
	t.Parallel()
	files := []string{"metasystem.go"}
	registry, err := filepath.Glob(filepath.Join("..", "runtimes", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range registry {
		if !strings.HasSuffix(path, "_test.go") {
			files = append(files, path)
		}
	}
	for _, path := range files {
		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, spec := range parsed.Imports {
			if spec.Path.Value == `"os/exec"` {
				t.Errorf("%s imports os/exec: the metasystem audit must not depend on an external command", path)
			}
		}
	}
}

// The delivery mode is declared, never inferred (D33): the engine compiles
// metasystem.engine-delivery=source. The template may declare central once
// for its own seats (R-151-m1e); adoption drops that declaration. Source
// delivery ships the Go module that declares the metasystem module path.
func TestShippedInstallationDeclaresItsEngineDelivery(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, conf string
		wantError  bool
	}{
		{"compiled default", "", false},
		{"template seats", "metasystem.engine-delivery=central\n", false},
		{"explicit source", "metasystem.engine-delivery=source\n", true},
		{"unknown delivery", "metasystem.engine-delivery=other\n", true},
		{"duplicate declaration", "metasystem.engine-delivery=central\nmetasystem.engine-delivery=central\n", true},
	} {
		t.Run(row.name, func(t *testing.T) {
			if err := templateEngineDeliveryError([]byte(row.conf)); (err != nil) != row.wantError {
				t.Fatalf("template delivery declaration error = %v, want error %v", err, row.wantError)
			}
		})
	}
	root := shippedInstallationRoot(t)
	conf, err := os.ReadFile(filepath.Join(root, "metasystem.conf"))
	if err != nil {
		t.Fatal(err)
	}
	if err := templateEngineDeliveryError(conf); err != nil {
		t.Fatal(err)
	}
	if mode, ok := config.CompiledDefault("metasystem.engine-delivery"); !ok || mode != "source" {
		t.Fatalf("the template ships its engine as source; the compiled metasystem.engine-delivery=%q", mode)
	}
	module, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatalf("metasystem.engine-delivery=source but go.mod is absent: the engine source did not ship: %v", err)
	}
	if !regexp.MustCompile(`(?m)^module github\.com/widoriezebos/agentic-tools/metasystem$`).Match(module) {
		t.Fatal("metasystem Go source present but go.mod does not declare the metasystem module: damaged template")
	}
}

func templateEngineDeliveryError(conf []byte) error {
	overrides := regexp.MustCompile(`(?m)^metasystem\.engine-delivery=([^\r\n]*)`).FindAllSubmatch(conf, -1)
	if len(overrides) > 1 {
		return fmt.Errorf("metasystem.conf overrides the compiled metasystem.engine-delivery %d times", len(overrides))
	}
	if len(overrides) == 1 && string(overrides[0][1]) != "central" {
		return fmt.Errorf("the template's own seats must declare central delivery, got %q", overrides[0][1])
	}
	return nil
}

// The assets the always-loaded instructions and the agent protocol route to
// are present. The lists are the script's two routed-asset loops; entries
// the unit that deletes a file removes with it.
func TestShippedInstallationRoutedAssetsExist(t *testing.T) {
	t.Parallel()
	root := shippedInstallationRoot(t)
	routed := []string{
		"docs/project-rules.md",
		"docs/orchestration.md",
		"docs/collaboration.md",
		"docs/design/design-principles.md",
		"docs/design/design-obligation-gate.md",
		"docs/examples/design-obligation-matrix.md",
		"docs/examples/step-back-ledger.md",
		".gitattributes",
		"memory/instruction-ledger.md",
		"internal/adopt/github-actions-metasystem.yml",
		"internal/runtimes/enforcement/claude-code-hooks.json",
		"internal/runtimes/enforcement/codex-hooks.json",
		"internal/runtimes/enforcement/devin-hooks.json",
		"docs/examples/mission-contract.md",
		"docs/examples/mission-cron.example",
		"docs/project-adaptation.md",
		"docs/metasystem-reconciliation.md",
		"docs/working-modes.md",
		"docs/working-with-agents.md",
		"plans/README.md",
	}
	protocol := []string{
		"internal/protocol/templates/brief.md",
		"internal/protocol/templates/follow-up.md",
		"internal/protocol/templates/host-turn-instruction.md",
		"internal/protocol/roles/orchestrator.md",
		"internal/protocol/schemas/orchestrator.schema.json",
		"internal/protocol/permissions/none.json",
		"internal/protocol/permissions/workspace.json",
		"metasystem.conf",
		"internal/protocol/templates/design-common.md",
		"internal/pathclass/path-classes.txt",
		"internal/landing/landing-classes.json",
		"internal/events/event-registry.json",
	}
	for _, list := range []struct {
		kind  string
		paths []string
	}{{"routed asset", routed}, {"agent protocol asset", protocol}} {
		for _, rel := range list.paths {
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
				t.Errorf("missing %s: %s", list.kind, rel)
			}
		}
	}
}

// The engine's data is compiled into the engine from the packages that own
// it; the installation carries no scripts tree for it to drift back into.
func TestShippedInstallationHasNoScriptsTree(t *testing.T) {
	t.Parallel()
	root := shippedInstallationRoot(t)
	if _, err := os.Lstat(filepath.Join(root, "scripts")); !os.IsNotExist(err) {
		t.Fatalf("metasystem/scripts exists (%v): engine data belongs in the Go package that reads it, compiled in with go:embed", err)
	}
}
