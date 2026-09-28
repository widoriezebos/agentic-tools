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
		if !strings.Contains(string(hooks), "metasystem internal hook ") {
			t.Errorf("%s hooks never invoke the engine's hook entry", name)
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

// The fake runtime honors its registry declaration through the engine's
// delegate supervisor: its envelope probe, run from a bare root, snapshots the
// declared enforcement map and observes both denials. The per-runtime adapter
// and host script contracts retired with the adapters' port to Go (batch 2,
// U6a): internal/adapter/supervisor's TestRuntimeAdapterContracts and the
// internal/missionrunner/hostturn host tests own them.
func TestShippedAdapterScriptsHonorTheRuntimeRegistry(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	engine := buildShippedEngine(t, ctx)

	probeRoot := t.TempDir()
	probeResult := filepath.Join(t.TempDir(), "fake-envelope-probe-result.json")
	probe := exec.CommandContext(ctx, engine, "delegate-supervisor", "fake", "probe", "--root", probeRoot)
	probe.Dir = probeRoot
	probe.Env = append(os.Environ(), "METASYSTEM_FAKE_ENVELOPE_PROBE_RESULT="+probeResult, "TMPDIR="+t.TempDir())
	var probeOut, probeErr bytes.Buffer
	probe.Stdout, probe.Stderr = &probeOut, &probeErr
	if err := probe.Run(); err != nil {
		t.Fatalf("fake runtime probe failed: %v\n%s", err, probeErr.String())
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
