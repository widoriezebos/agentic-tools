package hooks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	degradedFormsScript = "scripts/agents/stop-degraded-forms.sh"
	degradedHook        = "scripts/agents/supervision-hook.sh"
	degradedTemplate    = "scripts/enforcement/claude-code-hooks.json"
	degradedGolden      = "internal/hooks/testdata/stop-degraded-forms.golden"
	degradedStatusText  = "Status un" + "available"
)

type degradedFormRow struct {
	outcome    string
	cause      string
	qualifiers string
	payload    string
}

func TestDegradedStopRendererIsSideEffectFree(t *testing.T) {
	root := degradedModuleRoot(t)
	script := filepath.Join(root, degradedFormsScript)
	rows := degradedGoldenRows(t, root)
	empty := t.TempDir()
	bashPath, err := exec.LookPath("bash")
	if err != nil {
		t.Fatal(err)
	}
	bashPath, err = filepath.Abs(bashPath)
	if err != nil {
		t.Fatal(err)
	}
	render := func(arguments ...string) ([]byte, []byte, int) {
		commandArguments := []string{"-i", "PATH=/nonexistent", bashPath, "--noprofile", "--norc", "-c", `. "$1"; shift; degraded_stop_form "$@"`, "renderer", script}
		command := exec.Command("/usr/bin/env", append(commandArguments, arguments...)...)
		command.Dir = empty
		var stdout, stderr bytes.Buffer
		command.Stdout = &stdout
		command.Stderr = &stderr
		err := command.Run()
		if err == nil {
			return stdout.Bytes(), stderr.Bytes(), 0
		}
		if exit, ok := err.(*exec.ExitError); ok {
			return stdout.Bytes(), stderr.Bytes(), exit.ExitCode()
		}
		t.Fatalf("run isolated renderer: %v", err)
		return nil, nil, -1
	}

	for _, row := range rows {
		arguments := []string{row.outcome, row.cause}
		if row.qualifiers != "" {
			arguments = append(arguments, strings.Split(row.qualifiers, ",")...)
		}
		stdout, stderr, code := render(arguments...)
		if code != 0 || string(stdout) != row.payload+"\n" || len(stderr) != 0 {
			t.Fatalf("render %s/%s/%s = exit %d stdout %q stderr %q", row.outcome, row.cause, row.qualifiers, code, stdout, stderr)
		}
	}

	allDeadline := degradedGoldenPayload(t, rows, "allowed", "deadline-expired", "record-update-failed,condition-log-failed,no-resolved-checkout")
	stdout, stderr, code := render("allowed", "deadline-expired", "no-resolved-checkout", "record-update-failed", "condition-log-failed", "record-update-failed")
	if code != 0 || string(stdout) != allDeadline+"\n" || len(stderr) != 0 {
		t.Fatalf("shuffled duplicate qualifiers = exit %d stdout %q stderr %q", code, stdout, stderr)
	}

	refusals := [][]string{
		{"unknown", "bare"},
		{"allowed", "unknown"},
		{"allowed", "deadline-expired", "unknown"},
		{"allowed", "engine-missing", "condition-log-failed"},
		{"blocked", "engine-missing"},
		{"blocked", "bare", "condition-log-failed"},
		{"allowed"},
	}
	for _, arguments := range refusals {
		stdout, stderr, code := render(arguments...)
		if code != 2 || len(stdout) != 0 || len(stderr) != 0 {
			t.Errorf("refusal %q = exit %d stdout %q stderr %q; want exit 2 and no output", arguments, code, stdout, stderr)
		}
	}
	entries, err := os.ReadDir(empty)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("pure renderer created files: %v", entries)
	}
}

func TestDegradedStopFormsMatchTheGoldenContract(t *testing.T) {
	root := degradedModuleRoot(t)
	script := filepath.Join(root, degradedFormsScript)
	golden, err := os.ReadFile(filepath.Join(root, degradedGolden))
	if err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := runDegradedForms(t, root, script, "--list")
	if code != 0 || !bytes.Equal(stdout, golden) || len(stderr) != 0 {
		t.Fatalf("--list differs from the golden: exit %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	rows := parseDegradedRows(t, golden)
	if len(rows) != 18 {
		t.Fatalf("golden has %d forms; want 18", len(rows))
	}
	for _, row := range rows {
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

	mutated := filepath.Join(t.TempDir(), "stop-degraded-forms.sh")
	source, err := os.ReadFile(script)
	if err != nil {
		t.Fatal(err)
	}
	changed := strings.Replace(string(source), "The steward must restore supervision.", "The steward must repair supervision.", 1)
	if changed == string(source) {
		t.Fatal("renderer remedy mutation did not change the source")
	}
	if err := testexec.WriteFile(mutated, []byte(changed), 0o755); err != nil {
		t.Fatal(err)
	}
	mutatedOutput, _, mutatedCode := runDegradedForms(t, root, mutated, "--list")
	if mutatedCode != 0 || bytes.Equal(mutatedOutput, golden) {
		t.Fatalf("changed renderer constant did not fail the golden comparison: exit %d", mutatedCode)
	}
}

func TestStopHookTakesDegradedFormsFromTheRenderer(t *testing.T) {
	root := degradedModuleRoot(t)
	script := filepath.Join(root, degradedFormsScript)
	stdout, stderr, code := runDegradedForms(t, root, script, "--check")
	if code != 0 {
		t.Fatalf("repository generated copies are stale: stdout %q stderr %q", stdout, stderr)
	}
	source := degradedRead(t, filepath.Join(root, degradedFormsScript))
	hook := degradedRead(t, filepath.Join(root, degradedHook))
	sourceBlock, _, _ := degradedMarkedBlock(t, source, "# BEGIN SOURCE degraded Stop forms", "# END SOURCE degraded Stop forms")
	hookBlock, hookOutsideBefore, hookOutsideAfter := degradedMarkedBlock(t, hook, "# BEGIN GENERATED degraded Stop forms", "# END GENERATED degraded Stop forms")
	if hookBlock != sourceBlock {
		t.Fatal("the hook's generated declarations and renderer differ from their source")
	}
	outside := hookOutsideBefore + hookOutsideAfter
	if strings.Contains(outside, degradedStatusText) {
		t.Fatal("the hook carries a degraded Status literal outside the generated block")
	}
	for _, cause := range []string{
		"stop-hook-output-was-unreadable",
		"engine missing",
		"engine does not answer path state-root",
		"payload staging failed",
		"stop deadline expired",
		"hook-bootstrap-failed",
	} {
		if strings.Contains(outside, "needs supervision repair; "+cause) {
			t.Errorf("the hook carries the %q degraded literal outside the generated block", cause)
		}
	}

	temporaryRoot, temporaryScript := degradedSyncTree(t, root)
	temporaryHook := filepath.Join(temporaryRoot, degradedHook)
	mutated := strings.Replace(degradedRead(t, temporaryHook), "Rebuild bin/metasystem.", "Repair bin/metasystem.", 1)
	if err := testexec.WriteFile(temporaryHook, []byte(mutated), 0o755); err != nil {
		t.Fatal(err)
	}
	_, staleError, staleCode := runDegradedForms(t, temporaryRoot, temporaryScript, "--check", "--root", temporaryRoot)
	if staleCode != 1 || !strings.Contains(string(staleError), degradedHook) {
		t.Fatalf("hand-edited hook block check = exit %d stderr %q", staleCode, staleError)
	}
}

func TestBedsTakeDegradedStopFormsFromTheRenderer(t *testing.T) {
	root := degradedModuleRoot(t)
	for _, relative := range []string{"scripts/agents/supervision-hook-fixtures.sh", "scripts/agents/supervision-fixtures.sh"} {
		contents := degradedRead(t, filepath.Join(root, relative))
		if !strings.Contains(contents, "stop-degraded-forms.sh") || !strings.Contains(contents, "degraded_stop_form") {
			t.Errorf("%s does not source and use the degraded Stop renderer", relative)
		}
		if strings.Contains(contents, degradedStatusText) {
			t.Errorf("%s carries a degraded payload literal", relative)
		}
	}

	scriptsRoot := filepath.Join(root, "scripts")
	err := filepath.WalkDir(scriptsRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		contents := degradedRead(t, path)
		if !strings.Contains(contents, degradedStatusText) {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		switch filepath.ToSlash(relative) {
		case degradedFormsScript, degradedTemplate:
			return nil
		case degradedHook:
			_, before, after := degradedMarkedBlock(t, contents, "# BEGIN GENERATED degraded Stop forms", "# END GENERATED degraded Stop forms")
			if !strings.Contains(before+after, degradedStatusText) {
				return nil
			}
		}
		return fmt.Errorf("%s carries the degraded status text outside an allowed generated copy", relative)
	})
	if err != nil {
		t.Fatal(err)
	}

	// No Go test needs a literal exception; add any future exception here with
	// the reason the test cannot read the shipped template or golden contract.
	allowedGoTestLiterals := map[string]string{}
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_test.go") {
			return nil
		}
		contents := degradedRead(t, path)
		if !strings.Contains(contents, degradedStatusText) {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if reason := allowedGoTestLiterals[filepath.ToSlash(relative)]; reason != "" {
			return nil
		}
		return fmt.Errorf("%s carries a degraded payload literal without a documented exception", relative)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestBootstrapFallbackIsGeneratedFromTheDeclarations(t *testing.T) {
	root := degradedModuleRoot(t)
	rows := degradedGoldenRows(t, root)
	payload := degradedGoldenPayload(t, rows, "allowed", "bootstrap-failed", "")
	want := "printf '%s\\n' '" + payload + "'"
	command := degradedClaudeStopCommand(t, filepath.Join(root, degradedTemplate))
	if got := shippedFallback(command); got != want {
		t.Fatalf("shipped bootstrap fallback = %q, want golden %q", got, want)
	}

	temporaryRoot, temporaryScript := degradedSyncTree(t, root)
	temporaryTemplate := filepath.Join(temporaryRoot, degradedTemplate)
	mutated := strings.Replace(degradedRead(t, temporaryTemplate), "hook-bootstrap-failed", "hook-bootstrap-broken", 1)
	if err := os.WriteFile(temporaryTemplate, []byte(mutated), 0o644); err != nil {
		t.Fatal(err)
	}
	_, staleError, staleCode := runDegradedForms(t, temporaryRoot, temporaryScript, "--check", "--root", temporaryRoot)
	if staleCode != 1 || !strings.Contains(string(staleError), degradedTemplate) {
		t.Fatalf("hand-edited template check = exit %d stderr %q", staleCode, staleError)
	}
	_, syncError, syncCode := runDegradedForms(t, temporaryRoot, temporaryScript, "--sync", "--root", temporaryRoot)
	if syncCode != 0 {
		t.Fatalf("template sync = exit %d stderr %q", syncCode, syncError)
	}
	if got := shippedFallback(degradedClaudeStopCommand(t, temporaryTemplate)); got != want {
		t.Fatalf("synced bootstrap fallback = %q, want %q", got, want)
	}
	_, checkError, checkCode := runDegradedForms(t, temporaryRoot, temporaryScript, "--check", "--root", temporaryRoot)
	if checkCode != 0 {
		t.Fatalf("check after sync = exit %d stderr %q", checkCode, checkError)
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

func degradedGoldenRows(t *testing.T, root string) []degradedFormRow {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join(root, degradedGolden))
	if err != nil {
		t.Fatal(err)
	}
	return parseDegradedRows(t, contents)
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

func runDegradedForms(t *testing.T, directory, script string, arguments ...string) ([]byte, []byte, int) {
	t.Helper()
	command := exec.Command("bash", append([]string{script}, arguments...)...)
	command.Dir = directory
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	if err == nil {
		return stdout.Bytes(), stderr.Bytes(), 0
	}
	if exit, ok := err.(*exec.ExitError); ok {
		return stdout.Bytes(), stderr.Bytes(), exit.ExitCode()
	}
	t.Fatalf("run %s %q: %v", script, arguments, err)
	return nil, nil, -1
}

func degradedMarkedBlock(t *testing.T, contents, begin, end string) (block, before, after string) {
	t.Helper()
	beginAt := strings.Index(contents, begin+"\n")
	if beginAt < 0 {
		t.Fatalf("missing marker %q", begin)
	}
	blockAt := beginAt + len(begin) + 1
	endAt := strings.Index(contents[blockAt:], "\n"+end)
	if endAt < 0 {
		t.Fatalf("missing marker %q", end)
	}
	endAt += blockAt
	return contents[blockAt:endAt], contents[:beginAt], contents[endAt+1+len(end):]
}

func degradedSyncTree(t *testing.T, root string) (string, string) {
	t.Helper()
	temporaryRoot := t.TempDir()
	for _, relative := range []string{degradedFormsScript, degradedHook, degradedTemplate} {
		source := filepath.Join(root, relative)
		destination := filepath.Join(temporaryRoot, relative)
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			t.Fatal(err)
		}
		contents, err := os.ReadFile(source)
		if err != nil {
			t.Fatal(err)
		}
		mode := os.FileMode(0o644)
		if relative != degradedTemplate {
			mode = 0o755
		}
		if err := testexec.WriteFile(destination, contents, mode); err != nil {
			t.Fatal(err)
		}
	}
	return temporaryRoot, filepath.Join(temporaryRoot, degradedFormsScript)
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
