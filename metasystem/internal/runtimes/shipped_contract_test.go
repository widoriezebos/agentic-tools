package runtimes

// The runtime-contract-audits section of the retired validate-metasystem.sh
// (verbs-object-action U7b): the shipped installation's runtime declarations
// agree with its enforcement configurations, adapter and host scripts, role
// assets and template skill assets. Static legs read the installation at
// ../..; the adapter legs execute the shipped adapter scripts as the section
// did, against an engine built from this tree, and write only to temp dirs.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

func shippedRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

// The template ships its optional model tiers and mode role overrides as
// commented examples; the only active mode role keys are the shared
// Claude/Fable design-author default.
func TestShippedTemplateConfigurationKeepsOptionalKeysDemoted(t *testing.T) {
	t.Parallel()
	conf, err := os.ReadFile(filepath.Join(shippedRoot(t), "metasystem.conf"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(conf)
	if got := len(regexp.MustCompile(`(?m)^# Example model\.tier\.[123]=`).FindAllString(text, -1)); got != 3 {
		t.Errorf("template demotion: %d commented model tier examples, want 3", got)
	}
	if got := len(regexp.MustCompile(`(?m)^# Example mode\.[a-z0-9-]+\.role\.`).FindAllString(text, -1)); got != 3 {
		t.Errorf("template demotion: %d commented mode role override examples, want 3", got)
	}
	active := regexp.MustCompile(`(?m)^mode\.[a-z0-9-]+\.role\..*$`).FindAllString(text, -1)
	want := []string{"mode.design.role.implementer.runtime=claude", "mode.design.role.implementer.model.claude=claude-fable-5-1"}
	if !reflect.DeepEqual(active, want) {
		t.Errorf("template demotion: active mode role keys %q differ from the shared Claude/Fable design-author default %q", active, want)
	}
	if regexp.MustCompile(`(?m)^model\.tier\.`).MatchString(text) {
		t.Error("template demotion: an optional model tier key is still active")
	}
}

// Every shipped enforcement configuration declares the three lifecycle
// events and routes them through the supervision hook.
func TestShippedEnforcementConfigurationsWireTheLifecycle(t *testing.T) {
	t.Parallel()
	root := shippedRoot(t)
	for _, name := range []string{"claude-code-hooks.json", "codex-hooks.json", "devin-hooks.json"} {
		data, err := os.ReadFile(filepath.Join(root, "scripts", "enforcement", name))
		if err != nil {
			t.Fatal(err)
		}
		var document struct {
			Hooks map[string]json.RawMessage `json:"hooks"`
		}
		if err := json.Unmarshal(data, &document); err != nil || document.Hooks == nil {
			t.Fatalf("%s has no hooks object: %v", name, err)
		}
		for _, event := range []string{"SessionStart", "Stop", "SessionEnd"} {
			if _, ok := document.Hooks[event]; !ok {
				t.Errorf("%s hooks lack the %s lifecycle event", name, event)
			}
		}
		hooks, _ := json.Marshal(document.Hooks)
		if !strings.Contains(string(hooks), "supervision-hook.sh") {
			t.Errorf("%s hooks never invoke supervision-hook.sh", name)
		}
	}
}

// The six dispatchable roles each ship a preamble, a capability declaration
// and a return schema.
func TestShippedDispatchableRoleAssetsExist(t *testing.T) {
	t.Parallel()
	root := shippedRoot(t)
	for _, role := range []string{"design-critic", "implementer", "code-critic", "verifier", "investigator", "behavior-judge"} {
		for _, rel := range []string{
			filepath.Join("scripts", "agents", "roles", role+".md"),
			filepath.Join("scripts", "agents", "roles", role+".requirements.json"),
			filepath.Join("scripts", "agents", "schemas", role+".schema.json"),
		} {
			if info, err := os.Stat(filepath.Join(root, rel)); err != nil || !info.Mode().IsRegular() {
				t.Errorf("missing %s role asset: %s", role, rel)
			}
		}
	}
}

// The template ships the full seven-skill set with every per-runtime
// profile.
func TestShippedTemplateSkillAssetsExist(t *testing.T) {
	t.Parallel()
	root := shippedRoot(t)
	for _, skill := range []string{"take-a-step-back", "design-critique", "code-critique", "verify", "refactor", "improve", "retro"} {
		for _, rel := range []string{"SKILL.md", "agents/claude-profile.md", "agents/devin/AGENT.md", "agents/openai.yaml"} {
			if _, err := os.Stat(filepath.Join(root, "skills", skill, filepath.FromSlash(rel))); err != nil {
				t.Errorf("missing template skill asset: skills/%s/%s", skill, rel)
			}
		}
	}
}

// buildShippedEngine builds this tree's engine once for the test that needs
// it; the adapter scripts construct their contract snapshots through it.
func buildShippedEngine(t *testing.T, ctx context.Context) string {
	t.Helper()
	engine := filepath.Join(t.TempDir(), "metasystem")
	build := exec.CommandContext(ctx, "go", "build", "-buildvcs=false", "-o", engine, "./cmd/metasystem")
	build.Dir = shippedRoot(t)
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build the shipped engine: %v\n%s", err, output)
	}
	return engine
}

func runShippedScript(t *testing.T, ctx context.Context, engine, script string, args ...string) (string, string, error) {
	t.Helper()
	command := exec.CommandContext(ctx, "bash", append([]string{script}, args...)...)
	command.Dir = shippedRoot(t)
	command.Env = append(os.Environ(), "METASYSTEM_BIN="+engine, "TMPDIR="+t.TempDir())
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	return stdout.String(), stderr.String(), err
}

// The declared runtime populations agree with the shipped adapter and host
// scripts: every common-lifecycle adapter exists, parses, advertises its ten
// verbs and binds its snapshot identity; every adapter's contract snapshot
// decodes with its own identity and the full production shape; every static
// enforcement map equals the adapter's own; every host advertises start-turn;
// and the fake adapter's envelope probe observes both denials.
func TestShippedAdapterScriptsHonorTheRuntimeRegistry(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	root := shippedRoot(t)
	engine := buildShippedEngine(t, ctx)

	common := WithCommonLifecycle()
	if len(common) == 0 {
		t.Fatal("the common-lifecycle population is empty")
	}
	for _, runtime := range common {
		adapter := filepath.Join("scripts", "agents", "adapters", runtime+".sh")
		info, err := os.Stat(filepath.Join(root, adapter))
		if err != nil {
			t.Fatalf("missing %s runtime adapter: %s", runtime, adapter)
		}
		if info.Mode().Perm()&0o111 == 0 {
			t.Errorf("%s runtime adapter is not executable: %s", runtime, adapter)
		}
		if _, stderr, err := runShippedScript(t, ctx, engine, "-n", adapter); err != nil {
			t.Errorf("%s adapter does not parse: %v\n%s", runtime, err, stderr)
		}
		stdout, stderr, _ := runShippedScript(t, ctx, engine, adapter, "--help")
		usage := stdout + stderr
		for _, verb := range []string{"identity", "config-identity", "signature", "enforcement-map", "contract", "probe", "dispatch", "follow-up", "cancel", "selftest"} {
			if !strings.Contains(usage, "adapters/"+runtime+".sh "+verb) {
				t.Errorf("%s adapter usage does not advertise %s", runtime, verb)
			}
		}
		source, err := os.ReadFile(filepath.Join(root, adapter))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(source), "adapter_common_init "+runtime) {
			t.Errorf("%s adapter does not bind its snapshot runtime identity", runtime)
		}
		if !strings.Contains(string(source), `write_capability_snapshot `+runtime+` "$version" "$hash"`) {
			t.Errorf("%s adapter does not write its named capability snapshot", runtime)
		}
	}

	adapters := WithAdapter()
	if len(adapters) == 0 {
		t.Fatal("the adapter population is empty")
	}
	for _, runtime := range adapters {
		adapter := filepath.Join("scripts", "agents", "adapters", runtime+".sh")
		stdout, stderr, err := runShippedScript(t, ctx, engine, adapter, "contract")
		if err != nil {
			t.Fatalf("%s adapter contract failed: %v\n%s", runtime, err, stderr)
		}
		var snapshot map[string]any
		if err := json.Unmarshal([]byte(stdout), &snapshot); err != nil {
			t.Fatalf("%s adapter contract snapshot is not JSON: %v\n%s", runtime, err, stdout)
		}
		if snapshot["runtime"] != runtime {
			t.Errorf("%s adapter contract snapshot carries wrong identity: %v", runtime, snapshot["runtime"])
		}
		for _, field := range []string{"cliVersion", "configHash", "capabilities", "permissions"} {
			if _, ok := snapshot[field]; !ok {
				t.Errorf("%s adapter contract snapshot lacks %s", runtime, field)
			}
		}
		enforcement, _ := snapshot["envelopeEnforcement"].(map[string]any)
		for _, field := range EnforcementFields {
			if _, ok := enforcement[field]; !ok {
				t.Errorf("%s adapter contract snapshot lacks envelopeEnforcement.%s", runtime, field)
			}
		}
	}

	compared := 0
	for _, runtime := range adapters {
		declaration, ok := Lookup(runtime)
		if !ok || declaration.ExpectedEnvelopeEnforcement == nil {
			continue
		}
		stdout, stderr, err := runShippedScript(t, ctx, engine, filepath.Join("scripts", "agents", "adapters", runtime+".sh"), "enforcement-map")
		if err != nil {
			t.Fatalf("%s adapter enforcement-map failed: %v\n%s", runtime, err, stderr)
		}
		var adapterMap map[string]string
		if err := json.Unmarshal([]byte(stdout), &adapterMap); err != nil {
			t.Fatalf("%s adapter enforcement map is not a string map: %v\n%s", runtime, err, stdout)
		}
		registryMap := map[string]string{}
		for field, value := range declaration.ExpectedEnvelopeEnforcement {
			registryMap[field] = string(value)
		}
		if len(adapterMap) != 3 || len(registryMap) != 3 {
			t.Errorf("%s enforcement map carries unexpected members (adapter=%d registry=%d)", runtime, len(adapterMap), len(registryMap))
		}
		if !reflect.DeepEqual(adapterMap, registryMap) {
			t.Errorf("%s adapter envelope enforcement %v drifted from the registry declaration %v", runtime, adapterMap, registryMap)
		}
		compared++
	}
	if compared < 3 {
		t.Errorf("only %d static enforcement maps compared: the population went missing", compared)
	}

	hosts := WithHost()
	if len(hosts) == 0 {
		t.Fatal("the host population is empty")
	}
	for _, runtime := range hosts {
		host := filepath.Join("scripts", "agents", "hosts", runtime+".sh")
		info, err := os.Stat(filepath.Join(root, host))
		if err != nil || info.Mode().Perm()&0o111 == 0 {
			t.Errorf("%s host adapter is missing or not executable: %s", runtime, host)
			continue
		}
		stdout, stderr, _ := runShippedScript(t, ctx, engine, host, "--help")
		if !strings.Contains(stdout+stderr, "start-turn") {
			t.Errorf("%s host adapter does not advertise start-turn", runtime)
		}
	}

	// The fake runtime is the only sandbox this suite owns: its probe drives
	// the denied write and network-call paths from a bare copied root and
	// reports the observed status.
	probeRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(probeRoot, "scripts", "agents", "adapters"), 0o755); err != nil {
		t.Fatal(err)
	}
	fake, err := os.ReadFile(filepath.Join(root, "scripts", "agents", "adapters", "fake.sh"))
	if err != nil {
		t.Fatal(err)
	}
	fakeCopy := filepath.Join(probeRoot, "scripts", "agents", "adapters", "fake.sh")
	if err := testexec.WriteFile(fakeCopy, fake, 0o755); err != nil {
		t.Fatal(err)
	}
	probeResult := filepath.Join(t.TempDir(), "fake-envelope-probe-result.json")
	probe := exec.CommandContext(ctx, fakeCopy, "probe")
	probe.Dir = probeRoot
	probe.Env = append(os.Environ(), "METASYSTEM_BIN="+engine, "METASYSTEM_FAKE_ENVELOPE_PROBE_RESULT="+probeResult, "TMPDIR="+t.TempDir())
	var probeOut, probeErr bytes.Buffer
	probe.Stdout, probe.Stderr = &probeOut, &probeErr
	if err := probe.Run(); err != nil {
		t.Fatalf("fake adapter probe failed: %v\n%s", err, probeErr.String())
	}
	snapshotPath := strings.TrimSpace(probeOut.String())
	var snapshot struct {
		EnvelopeEnforcement map[string]string `json:"envelopeEnforcement"`
	}
	if data, err := os.ReadFile(snapshotPath); err != nil || json.Unmarshal(data, &snapshot) != nil {
		t.Fatalf("fake probe snapshot %q unreadable: %v", snapshotPath, err)
	}
	if want := (map[string]string{"writeRoots": "mapped", "readRoots": "notEnforced", "network": "mapped"}); !reflect.DeepEqual(snapshot.EnvelopeEnforcement, want) {
		t.Errorf("fake snapshot envelope enforcement drifted: %v, want %v", snapshot.EnvelopeEnforcement, want)
	}
	var observed map[string]struct {
		Observed   string `json:"observed"`
		ExitStatus int    `json:"exitStatus"`
	}
	if data, err := os.ReadFile(probeResult); err != nil || json.Unmarshal(data, &observed) != nil {
		t.Fatalf("fake envelope probe result unreadable: %v", err)
	}
	var fields []string
	for field, outcome := range observed {
		fields = append(fields, fmt.Sprintf("%s=%s/%d", field, outcome.Observed, outcome.ExitStatus))
	}
	sort.Strings(fields)
	if got := strings.Join(fields, " "); got != "network=denied/77 writeRoots=denied/77" {
		t.Errorf("fake envelope probe did not observe both denials with status 77: %s", got)
	}
}
