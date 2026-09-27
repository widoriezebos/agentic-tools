package supervisor

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
)

// TestRuntimeAdapterContracts is the runtime-contract-audits rows of
// scripts/validate-metasystem.sh, ported with the adapters: every declared
// adapter has a supervisor, proves its contract through the real snapshot
// construction path with the FULL production shape, and a static
// envelope-enforcement map equals the registry declaration over exactly the
// three fields.
func TestRuntimeAdapterContracts(t *testing.T) {
	t.Parallel()
	declared := runtimes.WithAdapter()
	sort.Strings(declared)
	if strings.Join(Runtimes(), " ") != strings.Join(declared, " ") {
		t.Fatalf("supervised runtimes %v, declared adapters %v", Runtimes(), declared)
	}
	compared := 0
	for _, name := range declared {
		a := registry[name]
		for verb, present := range map[string]bool{
			"configIdentity": a.configIdentity != nil, "probe": a.probe != nil, "contract": a.contract != nil,
			"outputStream": a.outputStream != nil, "supervise": a.supervise != nil, "selftest": a.selftest != nil,
		} {
			if !present {
				t.Errorf("%s supervisor does not offer %s", name, verb)
			}
		}
		if a.contract == nil {
			continue
		}
		data, err := a.contract(Deps{Getenv: func(string) string { return "" }})
		if err != nil {
			t.Fatalf("%s contract: %v", name, err)
		}
		var snapshot map[string]any
		if err := json.Unmarshal(data, &snapshot); err != nil {
			t.Fatalf("%s contract is not JSON: %v", name, err)
		}
		if snapshot["runtime"] != name {
			t.Errorf("%s contract snapshot carries identity %v", name, snapshot["runtime"])
		}
		for _, member := range []string{"cliVersion", "configHash", "capabilities", "permissions", "envelopeEnforcement"} {
			if _, ok := snapshot[member]; !ok {
				t.Errorf("%s contract snapshot lacks %s", name, member)
			}
		}
		enforcement, _ := snapshot["envelopeEnforcement"].(map[string]any)
		if len(enforcement) != len(runtimes.EnforcementFields) {
			t.Errorf("%s contract enforcement %v is not exactly %v", name, enforcement, runtimes.EnforcementFields)
		}
		for _, field := range runtimes.EnforcementFields {
			if _, ok := enforcement[field]; !ok {
				t.Errorf("%s contract enforcement lacks %s", name, field)
			}
		}
		declaration, _ := runtimes.Lookup(name)
		if declaration.ExpectedEnvelopeEnforcement == nil {
			continue
		}
		for _, field := range runtimes.EnforcementFields {
			if enforcement[field] != string(declaration.ExpectedEnvelopeEnforcement[field]) {
				t.Errorf("%s adapter envelope enforcement %s=%v drifted from the registry %s", name, field, enforcement[field], declaration.ExpectedEnvelopeEnforcement[field])
			}
		}
		compared++
	}
	if compared < 3 {
		t.Fatalf("only %d static enforcement maps compared — the population went missing", compared)
	}
}

// TestSupervisorSignatureAndLocalConfigVerbs: the small read verbs answer
// from the registry for every supervised runtime.
func TestSupervisorSignatureAndLocalConfigVerbs(t *testing.T) {
	t.Parallel()
	for _, name := range Runtimes() {
		var stdout, stderr strings.Builder
		d := Deps{Root: "/installation", Stdout: &stdout, Stderr: &stderr, Getenv: func(string) string { return "" }}
		newDeps := func(string) Deps { return d }
		if code := Main([]string{name, "signature", "--root", "/installation"}, newDeps); code != 0 {
			t.Fatalf("%s signature = %d: %s", name, code, stderr.String())
		}
		text, _ := runtimes.SignatureText(name)
		if stdout.String() != text {
			t.Fatalf("%s signature printed %q, registry %q", name, stdout.String(), text)
		}
		stdout.Reset()
		if code := Main([]string{name, "local-config-paths", "--root", "/installation"}, newDeps); code != 0 {
			t.Fatalf("%s local-config-paths = %d", name, code)
		}
		paths, _ := runtimes.LocalConfigPaths(name)
		want := ""
		for _, path := range paths {
			want += path + "\n"
		}
		if stdout.String() != want {
			t.Fatalf("%s local-config-paths printed %q, want %q", name, stdout.String(), want)
		}
		stdout.Reset()
		if code := Main([]string{name, "wait-delivery", "--root", "/installation", "--wait-id", "w", "--nonce", "n",
			"--deadline", "2026-09-27T10:00:00Z", "--session", "s"}, newDeps); code != 0 || stdout.String() != "blocking\n" {
			t.Fatalf("%s wait-delivery = %d %q", name, code, stdout.String())
		}
		if code := Main([]string{name, "wait-delivery", "--root", "/installation", "--wait-id", "w"}, newDeps); code != 2 {
			t.Fatalf("%s incomplete wait-delivery = %d, want 2", name, code)
		}
		if code := Main([]string{name, "dispatch"}, newDeps); code != 2 {
			t.Fatalf("%s dispatch without --root = %d, want 2", name, code)
		}
	}
}
