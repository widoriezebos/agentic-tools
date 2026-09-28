package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	degradedTemplate   = "scripts/enforcement/claude-code-hooks.json"
	degradedGolden     = "internal/hooks/testdata/stop-degraded-forms.golden"
	degradedStatusText = "Status un" + "available"
)

type degradedFormRow struct {
	outcome    string
	cause      string
	qualifiers string
	payload    string
}

// TestDegradedStopFormsMatchTheGoldenContract proves the one renderer against
// the golden contract: every admitted form, in order, each one JSON object of
// the right shape, and every inadmissible request refused.
func TestDegradedStopFormsMatchTheGoldenContract(t *testing.T) {
	t.Parallel()
	root := degradedModuleRoot(t)
	golden, err := os.ReadFile(filepath.Join(root, degradedGolden))
	if err != nil {
		t.Fatal(err)
	}
	if list := DegradedStopFormList(); list != string(golden) {
		t.Fatalf("rendered forms differ from the golden:\n%s", list)
	}
	rows := parseDegradedRows(t, golden)
	if len(rows) != 18 {
		t.Fatalf("golden has %d forms; want 18", len(rows))
	}
	for _, row := range rows {
		var qualifiers []string
		if row.qualifiers != "" {
			qualifiers = strings.Split(row.qualifiers, ",")
		}
		form, err := DegradedStopForm(row.outcome, row.cause, qualifiers...)
		if err != nil || form != row.payload {
			t.Fatalf("render %s/%s/%s = %q, %v", row.outcome, row.cause, row.qualifiers, form, err)
		}
		var object map[string]any
		if err := json.Unmarshal([]byte(row.payload), &object); err != nil {
			t.Errorf("%s/%s payload is not one JSON object: %v", row.outcome, row.cause, err)
			continue
		}
		if row.outcome == "allowed" {
			if len(object) != 1 || object["systemMessage"] == nil {
				t.Errorf("allowed %s payload shape = %#v", row.cause, object)
			}
		} else if len(object) != 2 || object["decision"] != "block" || object["reason"] == nil {
			t.Errorf("blocked %s payload shape = %#v", row.cause, object)
		}
	}
	allDeadline := degradedGoldenPayload(t, rows, "allowed", "deadline-expired", "record-update-failed,condition-log-failed,no-resolved-checkout")
	if form, err := DegradedStopForm("allowed", "deadline-expired", "no-resolved-checkout", "record-update-failed", "condition-log-failed", "record-update-failed"); err != nil || form != allDeadline {
		t.Fatalf("shuffled duplicate qualifiers = %q, %v", form, err)
	}
	for _, arguments := range [][]string{
		{"unknown", "bare"},
		{"allowed", "unknown"},
		{"allowed", "deadline-expired", "unknown"},
		{"allowed", "engine-missing", "condition-log-failed"},
		{"blocked", "engine-missing"},
		{"blocked", "bare", "condition-log-failed"},
	} {
		if form, err := DegradedStopForm(arguments[0], arguments[1], arguments[2:]...); err == nil || form != "" {
			t.Errorf("refusal %q = %q, %v; want a refusal and no form", arguments, form, err)
		}
	}
}

// TestDegradedFormsHaveOneSource keeps every degraded payload literal in the
// renderer: Go sources, tests and scripts name forms through it; only the
// golden contract, and the shipped launcher
// fallback carry the text.
func TestDegradedFormsHaveOneSource(t *testing.T) {
	t.Parallel()
	root := degradedModuleRoot(t)
	allowed := map[string]bool{
		"internal/hooks/degraded_forms.go":                   true,
		"internal/hooks/testdata/stop-degraded-forms.golden": true,
		degradedTemplate: true,
	}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if entry.IsDir() {
			switch relative {
			case "artifacts", "records", "plans", "memory", "node_modules", ".git":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(relative, ".go") && !strings.HasSuffix(relative, ".sh") && !strings.HasSuffix(relative, ".json") && !strings.HasSuffix(relative, ".golden") {
			return nil
		}
		contents := degradedRead(t, path)
		if !strings.Contains(contents, degradedStatusText) {
			return nil
		}
		if !allowed[relative] {
			return fmt.Errorf("%s carries a degraded Stop payload literal outside its one source", relative)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestBootstrapFallbackIsGeneratedFromTheDeclarations proves the shipped
// Claude Stop launcher's fallback is the renderer's bootstrap form.
func TestBootstrapFallbackIsGeneratedFromTheDeclarations(t *testing.T) {
	t.Parallel()
	root := degradedModuleRoot(t)
	want := "printf '%s\\n' '" + mustForm(t, "allowed", "bootstrap-failed") + "'"
	command := degradedClaudeStopCommand(t, filepath.Join(root, degradedTemplate))
	if got := shippedFallback(command); got != want {
		t.Fatalf("shipped bootstrap fallback = %q, want the renderer's %q", got, want)
	}
}

func degradedModuleRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func degradedRead(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(contents)
}

func parseDegradedRows(t *testing.T, contents []byte) []degradedFormRow {
	t.Helper()
	lines := strings.Split(strings.TrimSuffix(string(contents), "\n"), "\n")
	rows := make([]degradedFormRow, 0, len(lines))
	for lineNumber, line := range lines {
		fields := strings.SplitN(line, "\t", 4)
		if len(fields) != 4 {
			t.Fatalf("golden line %d has %d fields; want 4", lineNumber+1, len(fields))
		}
		rows = append(rows, degradedFormRow{fields[0], fields[1], fields[2], fields[3]})
	}
	return rows
}

func degradedGoldenPayload(t *testing.T, rows []degradedFormRow, outcome, cause, qualifiers string) string {
	t.Helper()
	for _, row := range rows {
		if row.outcome == outcome && row.cause == cause && row.qualifiers == qualifiers {
			return row.payload
		}
	}
	t.Fatalf("golden has no %s/%s/%s form", outcome, cause, qualifiers)
	return ""
}

func degradedClaudeStopCommand(t *testing.T, path string) string {
	t.Helper()
	var config struct {
		Hooks struct {
			Stop []struct {
				Hooks []struct {
					Command string `json:"command"`
				} `json:"hooks"`
			} `json:"Stop"`
		} `json:"hooks"`
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(contents, &config); err != nil {
		t.Fatal(err)
	}
	if len(config.Hooks.Stop) != 1 || len(config.Hooks.Stop[0].Hooks) != 1 {
		t.Fatalf("%s has no single Claude Stop command", path)
	}
	return config.Hooks.Stop[0].Hooks[0].Command
}
