package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/brain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/ownercall"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/contractmerge"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testrun"
)

type policyBed struct {
	seat, lane, coordinator, outside string
	now                              time.Time
	owners                           intentOwners
	registry                         config.PolicyRegistry
}

func newPolicyBed(t *testing.T) *policyBed {
	t.Helper()
	b := &policyBed{now: time.Date(2026, 10, 6, 10, 0, 0, 0, time.UTC)}
	for _, root := range []*string{&b.seat, &b.lane, &b.coordinator, &b.outside} {
		*root = t.TempDir()
		*root, _ = filepath.EvalSymlinks(*root)
	}
	for _, root := range []string{b.seat, b.lane, b.coordinator} {
		for _, dir := range []string{".git", "scripts/agents"} {
			if err := os.MkdirAll(filepath.Join(root, dir), 0700); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=claude\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	b.registry = config.PolicyRegistry{Lane: b.lane, Coordinator: b.coordinator}
	b.owners = intentOwners{resolver: stateroot.NewResolver(func(path string) (string, error) { seat, err := helm.Locate(path); return seat.Checkout, err }, noExecutable), commandNow: func(string) (time.Time, error) { return b.now, nil }, lookupEnv: func(string) (string, bool) { return "", false }}
	b.owners.policies.Registry = func(string) (config.PolicyRegistry, error) { return b.registry, nil }
	return b
}
func (b *policyBed) run(t *testing.T, cwd, verb string, args ...string) (int, intentResult, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := runIntentIn(mustIntentCommand(t, "settings "+verb), append(args, "--json"), &stdout, &stderr, cwd, b.owners)
	var result intentResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("%s: %v %q %q", verb, err, stdout.String(), stderr.String())
	}
	return code, result, stdout.String()
}
func (b *policyBed) show(t *testing.T, key string) config.PolicyResolution {
	t.Helper()
	code, result, _ := b.run(t, b.lane, "show", key)
	if code != 0 {
		t.Fatalf("show: %d %+v", code, result)
	}
	data, _ := json.Marshal(result.Data)
	var policy config.PolicyResolution
	if err := json.Unmarshal(data, &policy); err != nil {
		t.Fatal(err)
	}
	return policy
}
func policyBytes(t *testing.T, root, key string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, ".git", "metasystem", "policy", key+".json"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestPolicySettingsAuthorityAndRoutedRemedy(t *testing.T) {
	t.Parallel()
	b := newPolicyBed(t)
	b.owners.prove = enrolledPersonProver(t, b.seat, b.now)
	for _, key := range []string{"landing.batch", "landing.proof", "landing.on-red", "landing.trunk-red", "seat.driver", "review.stop", "goal.raise", "question.route"} {
		if !authoritySettings[key] {
			t.Fatalf("%s is not in the authority gate", key)
		}
		code, result, _ := b.run(t, b.seat, "set", key, "person")
		if code != 0 {
			t.Fatalf("%s: %d %+v", key, code, result)
		}
		destination := b.seat
		if config.PolicyScope(key) == "lane" {
			destination = b.lane
		}
		if key == "question.route" {
			destination = b.coordinator
		}
		before := policyBytes(t, destination, key)
		localBefore, err := os.ReadFile(filepath.Join(destination, "metasystem.conf.local"))
		if err != nil {
			t.Fatal(err)
		}
		personProof := b.owners.prove
		for _, prove := range []goalAuthorityProver{fixedFixtureGoalAuthority, func(root string, _ int64, _ humanauthority.Reader, _, _ string, at time.Time) (humanauthority.Proof, error) {
			return humanauthority.HelmProof(root, humanauthority.HelmGrant{By: "Wido", Grant: "grant"}, at)
		}} {
			b.owners.prove = prove
			code, result, _ = b.run(t, b.seat, "set", key, "auto")
			if code == 0 {
				t.Fatalf("agent/grant wrote %s: %+v", key, result)
			}
			if !bytes.Equal(before, policyBytes(t, destination, key)) {
				t.Fatal("refusal changed attribution")
			}
			after, _ := os.ReadFile(filepath.Join(destination, "metasystem.conf.local"))
			if !bytes.Equal(localBefore, after) {
				t.Fatal("refusal changed value")
			}
		}
		b.owners.prove = personProof
		if code, result, _ := b.run(t, b.seat, "set", key, "auto", "--repo", destination); code != 0 {
			t.Fatalf("printed remedy: %d %+v", code, result)
		}
		data := policyBytes(t, destination, key)
		if !bytes.Contains(data, []byte(`"set-by": "Wido"`)) {
			t.Fatalf("unproved name: %s", data)
		}
	}
	if p := b.show(t, "landing.proof"); p.SetBy != "Wido" {
		t.Fatalf("another key invalidated attribution: %+v", p)
	}
	// The public refusal supplies a command the seat-only enrollment can run.
	b.owners.prove = fixedFixtureGoalAuthority
	_, _, raw := b.run(t, b.seat, "set", "landing.batch", "2")
	var envelope struct {
		Next struct {
			Argv []string `json:"argv"`
		} `json:"next"`
	}
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(raw, "--repo") || !strings.Contains(raw, b.lane) {
		t.Fatalf("missing destination remedy: %s", raw)
	}
	b.owners.prove = enrolledPersonProver(t, b.seat, b.now)
	if code, result, _ := b.run(t, b.seat, "set", envelope.Next.Argv[3:]...); code != 0 {
		t.Fatalf("seat-only enrollment: %d %+v", code, result)
	}
	resolved := b.show(t, "landing.batch")
	if resolved.Value != "2" || resolved.SetBy != "Wido" || resolved.Checkout != b.lane || resolved.At != b.now {
		t.Fatalf("provenance: %+v", resolved)
	}
	original := policyBytes(t, b.lane, "landing.batch")
	b.now = b.now.Add(time.Hour)
	if code, result, _ := b.run(t, b.seat, "set", "landing.batch", "2"); code != 0 || result.Outcome != intentUnchanged {
		t.Fatalf("repeat: %d %+v", code, result)
	}
	if !bytes.Equal(original, policyBytes(t, b.lane, "landing.batch")) {
		t.Fatal("repeat replaced original attribution")
	}
}

func TestPolicySettingsDestinationFallbackAndUnnamedEnrollment(t *testing.T) {
	t.Parallel()
	for _, unnamed := range []bool{false, true} {
		b := newPolicyBed(t)
		prove := enrolledPersonProver(t, b.lane, b.now)
		if unnamed {
			path := filepath.Join(b.lane, "artifacts", "agents", "authority", "human-terminal.json")
			enrollment, err := humanauthority.ReadEnrollment(b.lane)
			if err != nil {
				t.Fatal(err)
			}
			enrollment.Human = ""
			data, _ := json.Marshal(enrollment)
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
		}
		var roots []string
		b.owners.prove = func(root string, pid int64, reader humanauthority.Reader, word, review string, at time.Time) (humanauthority.Proof, error) {
			roots = append(roots, root)
			return prove(root, pid, reader, word, review, at)
		}
		for _, cwd := range []string{b.seat, b.outside} {
			roots = nil
			code, result, _ := b.run(t, cwd, "set", "landing.batch", "2", "--repo", b.lane)
			if code != 0 {
				t.Fatalf("fallback from %s: %d %+v", cwd, code, result)
			}
			if cwd == b.outside && (len(roots) != 1 || roots[0] != b.lane) {
				t.Fatalf("outside proof roots: %v", roots)
			}
			if cwd == b.seat && (len(roots) != 2 || roots[0] != b.seat || roots[1] != b.lane) {
				t.Fatalf("fallback order: %v", roots)
			}
		}
		want := "Wido"
		if unnamed {
			want = "author unknown"
		}
		if got := b.show(t, "landing.batch").SetBy; got != want {
			t.Fatalf("setter %q, want %q", got, want)
		}
	}
	b := newPolicyBed(t)
	install := filepath.Join(b.seat, "metasystem")
	if err := os.Remove(filepath.Join(b.seat, "metasystem.conf")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(install, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(install, "metasystem.conf"), []byte("metasystem.template=true\nmetasystem.runtimes=claude\n"), 0600); err != nil {
		t.Fatal(err)
	}
	b.owners.prove = enrolledPersonProver(t, install, b.now)
	if code, result, _ := b.run(t, b.seat, "set", "landing.batch", "4", "--repo", b.lane); code != 0 {
		t.Fatalf("template calling enrollment: %d %+v", code, result)
	}
	if p := b.show(t, "landing.batch"); p.SetBy != "Wido" {
		t.Fatalf("template attribution: %+v", p)
	}

}

func TestPolicySettingsResolutionHelmAndDamage(t *testing.T) {
	t.Parallel()
	b := newPolicyBed(t)
	b.owners.prove = enrolledPersonProver(t, b.seat, b.now)
	b.registry.Coordinator = ""
	if p := b.show(t, "landing.batch"); p.Value != "auto" || p.Source != "built-in" {
		t.Fatalf("undeclared coordinator: %+v", p)
	}
	b.registry.Coordinator = b.coordinator
	if err := os.WriteFile(filepath.Join(b.coordinator, "metasystem.conf"), []byte("landing.batch=3\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if p := b.show(t, "landing.batch"); p.Value != "3" || p.Source != "coordinator/conf" {
		t.Fatalf("coordinator: %+v", p)
	}
	if code, result, _ := b.run(t, b.seat, "set", "landing.batch", "2"); code != 0 {
		t.Fatalf("set: %d %+v", code, result)
	}
	b.owners.lookupEnv = func(key string) (string, bool) { return "4", key == config.EnvName("landing.batch") }
	// The seat's environment cannot change the destination lane's policy.
	if code, result, _ := b.run(t, b.seat, "show", "landing.batch", "--repo", b.lane); code != 0 {
		t.Fatalf("routed show: %d %+v", code, result)
	} else {
		data, _ := json.Marshal(result.Data)
		if !bytes.Contains(data, []byte(`"value":"2"`)) {
			t.Fatalf("environment escaped checkout: %s", data)
		}
	}
	if p := b.show(t, "landing.batch"); p.Value != "4" || p.Source != "env" || p.SetBy != "set outside settings; author unknown" {
		t.Fatalf("environment: %+v", p)
	}
	b.owners.lookupEnv = func(string) (string, bool) { return "", false }
	if _, err := helm.Write(b.lane, helm.Record{By: "Wido", At: b.now.Format(time.RFC3339)}); err != nil {
		t.Fatal(err)
	}
	if p := b.show(t, "landing.batch"); p.Value != "person" || p.Previous == nil || p.Previous.Value != "2" || p.HelmHolder != "Wido" {
		t.Fatalf("helm: %+v", p)
	}
	if code, result, _ := b.run(t, b.seat, "set", "landing.batch", "5"); code != 0 || !strings.Contains(result.Summary, "applies when") {
		t.Fatalf("held write: %d %+v", code, result)
	}
	if p := b.show(t, "landing.batch"); p.Previous.Value != "5" {
		t.Fatalf("held previous: %+v", p)
	}
	var record config.PolicyRecord
	if err := json.Unmarshal(policyBytes(t, b.lane, "landing.batch"), &record); err != nil {
		t.Fatal(err)
	}
	if record.Previous.Value != "5" || record.Previous.Source != "conf-local" {
		t.Fatalf("held metadata hid the latest underlying value: %+v", record)
	}

	if _, err := helm.Remove(b.lane); err != nil {
		t.Fatal(err)
	}
	if p := b.show(t, "landing.batch"); p.Value != "5" {
		t.Fatalf("return lost newer value: %+v", p)
	}
	metadata := filepath.Join(b.lane, ".git", "metasystem", "policy", "landing.batch.json")
	if err := os.WriteFile(metadata, []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if p := b.show(t, "landing.batch"); !strings.Contains(p.SetBy, "author unknown") {
		t.Fatalf("broken metadata: %+v", p)
	}
	if code, result, _ := b.run(t, b.seat, "set", "landing.batch", "6"); code != 0 {
		t.Fatalf("repair: %d %+v", code, result)
	}
	if err := os.WriteFile(filepath.Join(b.lane, "metasystem.conf.local"), []byte("# keep me\nlanding.batch=broken\nunknown.bytes=preserve\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := b.run(t, b.lane, "show", "landing.batch"); code == 0 {
		t.Fatal("corrupt policy silently resolved")
	}
	if code, result, _ := b.run(t, b.seat, "set", "landing.batch", "7"); code != 0 {
		t.Fatalf("person repair: %d %+v", code, result)
	}
	local, _ := os.ReadFile(filepath.Join(b.lane, "metasystem.conf.local"))
	if !bytes.Contains(local, []byte("unknown.bytes=preserve")) || !bytes.Contains(local, []byte("# keep me")) {
		t.Fatalf("repair lost bytes: %s", local)
	}
	b.owners.policies.Registry = func(string) (config.PolicyRegistry, error) {
		return config.PolicyRegistry{}, fmt.Errorf("registry unreadable")
	}
	if code, _, _ := b.run(t, b.seat, "set", "landing.batch", "8"); code == 0 {
		t.Fatal("guessed destination")
	}
	if code, result, _ := b.run(t, b.seat, "set", "landing.batch", "8", "--repo", b.lane); code != 0 {
		t.Fatalf("explicit person replacement: %d %+v", code, result)
	}
}

func TestPolicySettingsGrammarScopeAndCommittedTiming(t *testing.T) {
	t.Parallel()
	b := newPolicyBed(t)
	b.owners.prove = enrolledPersonProver(t, b.seat, b.now)
	for _, value := range []string{"0", "-1", "1.5", "auto person", "", "1e3"} {
		if code, _, _ := b.run(t, b.seat, "set", "landing.batch", value); code == 0 {
			t.Fatalf("accepted %q", value)
		}
	}
	for _, value := range []string{"0", "9999999999999999999999999999999999"} {
		if code, result, _ := b.run(t, b.seat, "set", "review.stop", value); code != 0 {
			t.Fatalf("unbounded decimal %q: %d %+v", value, code, result)
		}
	}
	b.registry.Coordinator = ""
	if code, result, raw := b.run(t, b.seat, "set", "question.route", "person"); code == 0 || !strings.Contains(raw, "--declare") || !strings.Contains(raw, "--repo") {
		t.Fatalf("no coordinator remedy: %d %+v %s", code, result, raw)
	}
	if _, err := helm.Write(b.lane, helm.Record{By: "Wido", At: b.now.Format(time.RFC3339)}); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"boundary", "now"} {
		if err := os.WriteFile(filepath.Join(b.lane, "metasystem.conf"), []byte("settings.apply="+value+"\n"), 0600); err != nil {
			t.Fatal(err)
		}
		if p := b.show(t, "settings.apply"); p.Value != value || p.Source != "conf" || p.Previous != nil {
			t.Fatalf("timing: %+v", p)
		}
		if code, _, _ := b.run(t, b.seat, "set", "settings.apply", value); code == 0 {
			t.Fatal("timing gained a writer")
		}
	}
	// Structural validation needs no registered or live lane.
	b.registry = config.PolicyRegistry{}
	for _, text := range []string{"landing.batch=0\n", "review.stop=-1\n", "settings.apply=person\n"} {
		if err := os.WriteFile(filepath.Join(b.lane, "metasystem.conf"), []byte(text), 0600); err != nil {
			t.Fatal(err)
		}

		if strings.HasPrefix(text, "settings.apply=") {
			code, result, raw := b.run(t, b.lane, "show", "settings.apply")
			if code == 0 || !strings.Contains(result.Decision, "edit settings.apply") || strings.Contains(raw, "settings set") {
				t.Fatalf("read-only timing repair: %d %+v %s", code, result, raw)
			}
		}
		if code, _, raw := b.run(t, b.lane, "check"); code == 0 || !strings.Contains(raw, strings.Split(text, "=")[0]) {
			t.Fatalf("check accepted %q: %d %s", text, code, raw)
		}
	}
}

func TestPolicySettingsCheckWithoutSessionOrLane(t *testing.T) {
	t.Parallel()
	b := newPolicyBed(t)
	b.registry = config.PolicyRegistry{}
	body := "metasystem.runtimes=claude\nmetasystem.template=true\nproof.full=true\nproof.cheap=true\nproof.audits=true\nproof.deadline=15\nreview.stop=0\nlanding.batch=2\nsettings.apply=boundary\n"
	if err := os.WriteFile(filepath.Join(b.lane, "metasystem.conf"), []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	contract, err := contractmerge.Render(testingMergeFixture())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(b.lane, "testing.json"), contract, 0600); err != nil {
		t.Fatal(err)
	}
	b.owners.contractReady = func(root string, _ bool) (string, int, error) {
		_, contract, path, err := testrun.LoadContract(root)
		return path, len(contract.Groups), err
	}
	if code, result, _ := b.run(t, b.lane, "check"); code != 0 {
		t.Fatalf("structural check: %d %+v", code, result)
	}
	if err := os.WriteFile(filepath.Join(b.lane, "metasystem.conf.local"), []byte("settings.apply=now\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if code, _, raw := b.run(t, b.lane, "check"); code == 0 || !strings.Contains(raw, "committed repository declaration") {
		t.Fatalf("local timing scope: %d %s", code, raw)
	}
}

func TestPolicySettingsProductionRegistryAbsentAndCorrupt(t *testing.T) {
	t.Parallel()
	b := newPolicyBed(t)
	ledger := newIntentBed(t, false, nil)
	dependencies := ledger.owners().dependencies
	b.owners.dependencies.endpoint = func(string) (goal.Endpoint, error) { return dependencies.endpoint(ledger.root()) }
	registryHome := t.TempDir()
	b.owners.lookupEnv = func(key string) (string, bool) {
		if key == "METASYSTEM_SUPERVISION_REGISTRY_HOME" {
			return registryHome, true
		}
		return "", false
	}
	b.owners.policies.Registry = nil
	if p := b.show(t, "landing.batch"); p.Value != "auto" || p.Source != "built-in" {
		t.Fatalf("production undeclared: %+v", p)
	}
	endpoint, err := dependencies.endpoint(ledger.root())
	if err != nil {
		t.Fatal(err)
	}
	id := goal.ExistingLedgerIdentityAtEndpoint(endpoint)
	if id == "" {
		t.Fatal("fixture ledger has no identity")
	}
	if _, err := brain.Declare(brain.DeclareOptions{StateRoot: b.coordinator, RegistryHome: registryHome, LedgerIdentity: id, Machine: "policy-fixture", DeclaredBy: "Wido", Now: b.now}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(b.coordinator, "metasystem.conf"), []byte("landing.batch=3\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if p := b.show(t, "landing.batch"); p.Value != "3" || p.Source != "coordinator/conf" {
		t.Fatalf("production coordinator fallback: %+v", p)
	}
	// The same public reader distinguishes a damaged declaration from absence.
	data, err := os.ReadFile(brain.Path(b.coordinator))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(brain.Path(b.lane)), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(brain.Path(b.lane), data, 0600); err != nil {
		t.Fatal(err)
	}
	if code, result, raw := b.run(t, b.lane, "show", "landing.batch"); code != 0 {
		t.Fatalf("production declared: %d %+v %s", code, result, raw)
	}
	if err := os.WriteFile(brain.Path(b.lane), []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	if code, _, raw := b.run(t, b.lane, "show", "landing.batch"); code == 0 || !strings.Contains(raw, "declaration") {
		t.Fatalf("production corrupt: %d %s", code, raw)
	}
	if err := os.Remove(brain.Path(b.lane)); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(lane.RecordPath(filepath.Join(registryHome, ".metasystem"))), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lane.RecordPath(filepath.Join(registryHome, ".metasystem")), []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	if code, _, raw := b.run(t, b.lane, "show", "landing.batch"); code == 0 || !strings.Contains(raw, "landing-lane.json") {
		t.Fatalf("production corrupt lane: %d %s", code, raw)
	}
}

func TestPolicySettingsExecutableReadAndRefusal(t *testing.T) {
	t.Parallel()
	b := newPolicyBed(t)
	tools := t.TempDir()
	// Only repository location is supplied; an accepted ledger is absent.
	script := "#!/bin/sh\ncase \"$*\" in *--show-toplevel*) printf '%s\\n' \"$2\";; *) exit 1;; esac\n"
	if err := testexec.WriteFile(filepath.Join(tools, "git"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	binary := os.Getenv("METASYSTEM_WAIT_BINARY")
	if binary == "" {
		t.Fatal("package setup did not build the command")
	}
	var extraEnv []string
	run := func(args ...string) ([]byte, error) {
		command := exec.Command(binary, args...)
		command.Dir = b.lane
		command.Env = append(os.Environ(), "PATH="+tools, "METASYSTEM_SUPERVISION_REGISTRY_HOME="+t.TempDir())
		command.Env = append(command.Env, extraEnv...)
		return command.CombinedOutput()
	}
	output, err := run("settings", "show", "landing.batch", "--repo", b.lane, "--json")
	if err != nil || !bytes.Contains(output, []byte(`"value": "auto"`)) || !bytes.Contains(output, []byte(`"source": "built-in"`)) {
		t.Fatalf("executable show: %v %s", err, output)
	}
	t.Logf("metasystem settings show landing.batch --repo FIXTURE --json: %s", output)
	output, err = run("settings", "set", "landing.batch", "0", "--repo", b.lane, "--json")
	if err == nil || !bytes.Contains(output, []byte("decimal integer at least 1")) {
		t.Fatalf("executable invalid cap: %v %s", err, output)
	}
	t.Logf("metasystem settings set landing.batch 0 --repo FIXTURE --json: %s", output)
	extraEnv = []string{config.EnvName("landing.batch") + "=0"}
	output, err = run("settings", "check", "--repo", b.lane, "--json")
	if err == nil || !bytes.Contains(output, []byte("environment: landing.batch")) {
		t.Fatalf("executable environment validation: %v %s", err, output)
	}
	output, err = run("settings", "show", "landing.batch", "--repo", b.lane, "--json")
	var envelope struct {
		Next struct {
			Argv []string `json:"argv"`
		} `json:"next"`
	}
	if err == nil || json.Unmarshal(output, &envelope) != nil || strings.Join(envelope.Next.Argv, " ") != "unset METASYSTEM_LANDING_BATCH" {
		t.Fatalf("executable environment remedy: %v %s", err, output)
	}
	t.Logf("settings show with an invalid environment override: %s", output)
	extraEnv = nil
	output, err = run("settings", "show", "landing.batch", "--repo", b.lane, "--json")
	if err != nil || !bytes.Contains(output, []byte(`"value": "auto"`)) {
		t.Fatalf("executable after unsetting override: %v %s", err, output)
	}
	seat, err := helm.Locate(b.lane)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(seat.Dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(seat.Signature, []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	output, err = run("settings", "show", "landing.batch", "--repo", b.lane, "--json")
	if err == nil || json.Unmarshal(output, &envelope) != nil || strings.Join(envelope.Next.Argv, " ") != "metasystem helm return --repo "+b.lane {
		t.Fatalf("executable helm remedy: %v %s", err, output)
	}
	t.Logf("settings show with a malformed helm signature: %s", output)
	output, err = run(append(envelope.Next.Argv[1:], "--json")...)
	if err == nil || !helm.Active(b.lane).Active || !bytes.Contains(output, []byte("terminal")) {
		t.Fatalf("nonterminal return: %v %s", err, output)
	}
	// The remedy is the person's act at their terminal, through the public verb.
	b.owners.helm = helmOwners{reader: person(), pid: func() int64 { return 20 }, now: func() time.Time { return b.now }, stdinTerminal: func() bool { return false }}
	var returned bytes.Buffer
	if code := runIntentIn(mustIntentCommand(t, "helm return"), envelope.Next.Argv[3:], &returned, &returned, b.lane, b.owners); code != 0 {
		t.Fatalf("person's remedy: %d %s", code, returned.String())
	}
	output, err = run("settings", "show", "landing.batch", "--repo", b.lane, "--json")
	if err != nil || !bytes.Contains(output, []byte(`"value": "auto"`)) {
		t.Fatalf("executable after returning helm: %v %s", err, output)
	}

}

func TestPolicySettingsWriteFailuresAndOwnerRecheck(t *testing.T) {
	t.Parallel()
	b := newPolicyBed(t)
	b.owners.prove = enrolledPersonProver(t, b.seat, b.now)
	reads := 0
	b.owners.policies.Registry = func(string) (config.PolicyRegistry, error) {
		reads++
		if reads > 1 {
			return config.PolicyRegistry{Lane: b.coordinator}, nil
		}
		return b.registry, nil
	}
	if code, _, raw := b.run(t, b.seat, "set", "landing.batch", "2"); code == 0 || !strings.Contains(raw, "owner changed") {
		t.Fatalf("owner recheck: %d %s", code, raw)
	}
	if _, err := os.Stat(filepath.Join(b.lane, "metasystem.conf.local")); !os.IsNotExist(err) {
		t.Fatalf("stale target wrote a setting: %v", err)
	}
	b.owners.policies.Registry = func(string) (config.PolicyRegistry, error) { return b.registry, nil }
	path := filepath.Join(b.lane, ".git", "metasystem", "policy", "landing.batch.json")
	if err := os.MkdirAll(path, 0700); err != nil {
		t.Fatal(err)
	}
	if code, _, raw := b.run(t, b.seat, "set", "landing.batch", "2"); code == 0 || !strings.Contains(raw, "incomplete") {
		t.Fatalf("metadata I/O failure: %d %s", code, raw)
	}
	if p := b.show(t, "landing.batch"); p.Value != "2" || !strings.Contains(p.SetBy, "author unknown") {
		t.Fatalf("partial write fabricated attribution: %+v", p)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if code, result, _ := b.run(t, b.seat, "set", "landing.batch", "3"); code != 0 {
		t.Fatalf("retry: %d %+v", code, result)
	}
	if p := b.show(t, "landing.batch"); p.SetBy != "Wido" {
		t.Fatalf("retry attribution: %+v", p)
	}
	if err := os.WriteFile(filepath.Join(b.lane, "metasystem.conf.local"), []byte("landing.batch=4\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if p := b.show(t, "landing.batch"); p.Value != "4" || !strings.Contains(p.SetBy, "author unknown") {
		t.Fatalf("manual bytes inherited old setter: %+v", p)
	}
}

func TestPolicySettingsShowRemedies(t *testing.T) {
	t.Parallel()
	for _, cause := range []string{"environment", "file", "helm", "coordinator", "coordinator-pointer", "lane"} {
		t.Run(cause, func(t *testing.T) {
			t.Parallel()
			b := newPolicyBed(t)
			b.owners.prove = enrolledPersonProver(t, b.lane, b.now)
			b.owners.helm = helmOwners{reader: person(), pid: func() int64 { return 20 }, now: func() time.Time { return b.now }, stdinTerminal: func() bool { return false }}
			key := "landing.batch"
			registryHome := t.TempDir()
			lookup := map[string]string{"METASYSTEM_SUPERVISION_REGISTRY_HOME": registryHome}
			b.owners.lookupEnv = func(key string) (string, bool) { value, ok := lookup[key]; return value, ok }
			var want []string
			switch cause {
			case "environment":
				lookup[config.EnvName(key)] = "broken"
				want = []string{"unset", config.EnvName(key)}
			case "file":
				if err := os.WriteFile(filepath.Join(b.lane, "metasystem.conf"), []byte(key+"=broken\n"), 0600); err != nil {
					t.Fatal(err)
				}
				want = []string{"metasystem", "settings", "set", key, "auto", "--repo", b.lane}
			case "helm":
				seat, err := helm.Locate(b.lane)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.MkdirAll(seat.Dir, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(seat.Signature, []byte("broken"), 0600); err != nil {
					t.Fatal(err)
				}
				want = []string{"metasystem", "helm", "return", "--repo", b.lane}
			case "coordinator", "coordinator-pointer":
				b.owners.policies.Registry = nil
				ledger := newIntentBed(t, false, nil)
				dependencies := ledger.owners().dependencies
				b.owners.dependencies.endpoint = func(string) (goal.Endpoint, error) { return dependencies.endpoint(ledger.root()) }
				endpoint, err := dependencies.endpoint(ledger.root())
				if err != nil {
					t.Fatal(err)
				}
				id := goal.ExistingLedgerIdentityAtEndpoint(endpoint)
				if id == "" {
					t.Fatal("fixture ledger has no identity")
				}
				root := b.lane
				if cause == "coordinator-pointer" {
					root = b.coordinator
					if _, err := brain.Declare(brain.DeclareOptions{StateRoot: root, RegistryHome: registryHome, LedgerIdentity: id, Machine: "fixture", DeclaredBy: "Wido", Now: b.now}); err != nil {
						t.Fatal(err)
					}
				}
				if err := os.MkdirAll(filepath.Dir(brain.Path(root)), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(brain.Path(root), []byte("broken"), 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.MkdirAll(filepath.Join(root, "plans", "goals"), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, "plans", "goals", "backlog.md"), nil, 0600); err != nil {
					t.Fatal(err)
				}
				// Supply the terminal gate at the owner boundary; withdrawal
				// still removes the real declaration and host pointer.
				calls := intentOwnerCalls{brain: func(choice string, _ ownercall.Process, _ io.Writer, _ io.Writer, target, by string) int {
					if choice != "withdraw" || target != root || by == "" {
						t.Fatalf("coordinator remedy: %s %s %s", choice, target, by)
					}
					if _, err := brain.Withdraw(target, registryHome, id); err != nil {
						t.Fatal(err)
					}
					return 0
				}}
				b.owners.delivery = &intentDeliveryOwners{calls: &calls}
				want = []string{"metasystem", "settings", "coordinator", "--withdraw", "--by", "NAME", "--repo", root}
			case "lane":
				b.owners.policies.Registry = nil
				bed := newLaneVerbBed(t)
				bed.home = filepath.Join(registryHome, ".metasystem")
				b.owners.landing = bed.owners().landing
				if err := os.WriteFile(filepath.Join(bed.landingA, "metasystem", "metasystem.conf"), []byte("metasystem.template=true\n"), 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.MkdirAll(filepath.Dir(lane.RecordPath(bed.home)), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(lane.RecordPath(bed.home), []byte("broken"), 0600); err != nil {
					t.Fatal(err)
				}
				want = []string{"metasystem", "landing", "set", "PATH"}
				b.registry.Lane = bed.landingA
			}
			code, _, raw := b.run(t, b.lane, "show", key)
			if code == 0 {
				t.Fatal("damaged policy was readable")
			}
			var envelope struct {
				Next struct {
					Argv []string `json:"argv"`
				} `json:"next"`
			}
			if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
				t.Fatal(err)
			}
			argv := envelope.Next.Argv
			if len(argv) != len(want) {
				t.Fatalf("remedy %v, want %v: %s", argv, want, raw)
			}
			for i := range want {
				if want[i] == "NAME" {
					if argv[i] == "" {
						t.Fatal("missing person in remedy")
					}
				} else if argv[i] != want[i] {
					t.Fatalf("remedy %v, want %v", argv, want)
				}
			}
			if argv[0] == "unset" {
				delete(lookup, argv[1])
			} else {
				if cause == "lane" {
					argv[3] = b.registry.Lane
				}
				var stdout, stderr bytes.Buffer
				code := runIntentIn(mustIntentCommand(t, argv[1]+" "+argv[2]), append(argv[3:], "--json"), &stdout, &stderr, b.lane, b.owners)
				if code != 0 {
					t.Fatalf("printed remedy failed: %d %s %s", code, stdout.String(), stderr.String())
				}
			}
			if code, result, raw := b.run(t, b.lane, "show", key); code != 0 {
				t.Fatalf("read after printed remedy: %d %+v %s", code, result, raw)
			}
		})
	}
}

func TestPolicySettingsLinkedWorktreeEnrollment(t *testing.T) {
	t.Parallel()
	for _, enrolled := range []string{"worktree", "primary", "destination"} {
		t.Run(enrolled, func(t *testing.T) {
			t.Parallel()
			b := newPolicyBed(t)
			worktree := t.TempDir()
			worktree, _ = filepath.EvalSymlinks(worktree)
			gitdir := filepath.Join(b.seat, ".git", "worktrees", "linked")
			if err := os.MkdirAll(gitdir, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(gitdir, "commondir"), []byte("../..\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(worktree, ".git"), []byte("gitdir: "+gitdir+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(worktree, "metasystem.conf"), []byte("metasystem.runtimes=claude\n"), 0600); err != nil {
				t.Fatal(err)
			}
			cwd := filepath.Join(worktree, "child")
			if err := os.MkdirAll(cwd, 0700); err != nil {
				t.Fatal(err)
			}
			b.owners.resolver = stateroot.NewResolver(func(path string) (string, error) {
				if path == worktree || strings.HasPrefix(path, worktree+string(filepath.Separator)) {
					return worktree, nil
				}
				seat, err := helm.Locate(path)
				return seat.Checkout, err
			}, noExecutable)
			roots := []string{worktree, b.seat, b.lane}
			index := 0
			if enrolled == "primary" {
				index = 1
			}
			if enrolled == "destination" {
				index = 2
			}
			b.owners.prove = enrolledPersonProver(t, roots[index], b.now)
			var attempts []string
			b.owners.commandNow = func(root string) (time.Time, error) {
				attempts = append(attempts, root)
				return b.now, nil
			}
			if code, result, raw := b.run(t, cwd, "set", "landing.batch", "2"); code != 0 {
				t.Fatalf("linked worktree write: %d %+v %s", code, result, raw)
			}
			if strings.Join(attempts, "\n") != strings.Join(roots[:index+1], "\n") {
				t.Fatalf("proof roots: %v, want %v", attempts, roots[:index+1])
			}
			if p := b.show(t, "landing.batch"); p.Value != "2" || p.SetBy != "Wido" || p.Checkout != b.lane {
				t.Fatalf("worktree setter attribution: %+v", p)
			}
		})
	}
}

func TestPolicySettingsGrantRefusalRecorded(t *testing.T) {
	t.Parallel()
	b := newPolicyBed(t)
	proofs := make(map[string]humanauthority.Proof)
	for _, root := range []string{b.seat, b.lane} {
		proof, err := humanauthority.HelmProof(root, humanauthority.HelmGrant{By: "Wido", Grant: "grant"}, b.now)
		if err != nil {
			t.Fatal(err)
		}
		proofs[root] = proof
	}
	b.owners.prove = func(root string, _ int64, _ humanauthority.Reader, _, _ string, at time.Time) (humanauthority.Proof, error) {
		proof, ok := proofs[root]
		if !ok || !proof.CheckedAt.Equal(at) {
			t.Fatalf("no grant proof for %s at %s", root, at)
		}
		return proof, nil
	}
	if code, _, _ := b.run(t, b.seat, "set", "landing.batch", "2"); code == 0 {
		t.Fatal("grant wrote a policy")
	}
	for _, root := range []string{b.seat, b.lane} {
		data, err := os.ReadFile(humanauthority.AttorneyLogPath(root))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(data, []byte(`refused grant=grant by=Wido act="settings set landing.batch"`)) {
			t.Fatalf("missing refusal: %s", data)
		}
		if bytes.Count(data, []byte("refused grant=")) != 1 {
			t.Fatalf("duplicate refusal: %s", data)
		}
	}
}

// TestPolicySettingsShowBlankCoordinatorPointerIsNone: a blank coordinator
// pointer names no coordinator (as brain.Declare reads it), so a policy read
// resolves past that layer instead of refusing with a remedy that cannot fix it.
func TestPolicySettingsShowBlankCoordinatorPointerIsNone(t *testing.T) {
	t.Parallel()
	b := newPolicyBed(t)
	b.owners.policies.Registry = nil
	registryHome := t.TempDir()
	lookup := map[string]string{"METASYSTEM_SUPERVISION_REGISTRY_HOME": registryHome}
	b.owners.lookupEnv = func(key string) (string, bool) { value, ok := lookup[key]; return value, ok }
	ledger := newIntentBed(t, false, nil)
	dependencies := ledger.owners().dependencies
	b.owners.dependencies.endpoint = func(string) (goal.Endpoint, error) { return dependencies.endpoint(ledger.root()) }
	endpoint, err := dependencies.endpoint(ledger.root())
	if err != nil {
		t.Fatal(err)
	}
	id := goal.ExistingLedgerIdentityAtEndpoint(endpoint)
	if id == "" {
		t.Fatal("fixture ledger has no identity")
	}
	pointer := brain.PointerPath(registryHome, id)
	if err := os.MkdirAll(filepath.Dir(pointer), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pointer, []byte("  \n"), 0600); err != nil {
		t.Fatal(err)
	}
	if code, _, raw := b.run(t, b.lane, "show", "review.stop"); code != 0 {
		t.Fatalf("a blank coordinator pointer refused the read: %d %s", code, raw)
	}
}
