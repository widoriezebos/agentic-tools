package testrun

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/shellquote"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/strictjson"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// The worker capability compare tolerates what it does not know and the
// scratch environment policy is negotiated (DL4A-01): a worker listing no
// policies, as every worker before A5.2, gets v1; one listing this
// frontend's policy gets it; a differing known capability still refuses.
func TestScratchEnvironmentPolicyFollowsTheWorkerCapabilities(t *testing.T) {
	t.Parallel()
	reporting := func(edit func(map[string]any)) string {
		capabilities := CurrentWorkerCapabilities()
		data, err := json.Marshal(capabilities)
		if err != nil {
			t.Fatal(err)
		}
		fields := map[string]any{}
		if err := json.Unmarshal(data, &fields); err != nil {
			t.Fatal(err)
		}
		edit(fields)
		if data, err = json.Marshal(fields); err != nil {
			t.Fatal(err)
		}
		engine := filepath.Join(t.TempDir(), "engine")
		if err := testexec.WriteFile(engine, []byte("#!/bin/sh\nprintf '%s\\n' "+shellquote.Quote(string(data))+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		return engine
	}
	environment := []string{"PATH=/usr/bin:/bin"}
	for _, test := range []struct {
		name string
		edit func(map[string]any)
		want string
	}{
		{"worker listing no policies", func(fields map[string]any) { delete(fields, "scratchEnvironmentPolicies") }, proofrun.ScratchEnvironmentPolicyV1},
		{"worker reading only v1", func(fields map[string]any) {
			fields["scratchEnvironmentPolicies"] = []string{proofrun.ScratchEnvironmentPolicyV1}
		}, proofrun.ScratchEnvironmentPolicyV1},
		{"upgraded worker", func(fields map[string]any) {
			fields["scratchEnvironmentPolicies"] = []string{proofrun.ScratchEnvironmentPolicyV1, proofrun.ScratchEnvironmentPolicyV2}
		}, proofrun.ScratchEnvironmentPolicy},
		{"a capability this frontend does not know", func(fields map[string]any) { fields["laterCapability"] = 7 }, chooseScratchEnvironmentPolicy(CurrentWorkerCapabilities().ScratchEnvironmentPolicies)},
	} {
		capabilities, err := RequireWorkerCapabilities(t.Context(), reporting(test.edit), environment)
		if err != nil {
			t.Fatalf("%s: refused: %v", test.name, err)
		}
		if got := chooseScratchEnvironmentPolicy(capabilities.ScratchEnvironmentPolicies); got != test.want {
			t.Errorf("%s: policy = %q, want %q", test.name, got, test.want)
		}
	}
	_, err := RequireWorkerCapabilities(t.Context(), reporting(func(fields map[string]any) { fields["workerPolicyVersion"] = 99 }), environment)
	if !errors.Is(err, ErrWorkerPolicyUnsupported) || !strings.Contains(err.Error(), "install a matching engine release") {
		t.Fatalf("differing known capability: %v", err)
	}
	// This engine as its own worker writes its own policy without a probe.
	prepared := Preparation{FirstTestingTransition: true, PolicyEngine: "/nonexistent/engine"}
	if got := chooseScratchEnvironmentPolicy(workerScratchPolicies(t.Context(), prepared)); got != proofrun.ScratchEnvironmentPolicy {
		t.Fatalf("own worker policy = %q", got)
	}
	// An unreadable destination worker during revalidation is v1, never a refusal.
	prepared.FirstTestingTransition = false
	if got := chooseScratchEnvironmentPolicy(workerScratchPolicies(t.Context(), prepared)); got != proofrun.ScratchEnvironmentPolicyV1 {
		t.Fatalf("unreadable worker policy = %q", got)
	}
}

// A v1 descriptor from this launcher keeps the old wire (an old worker's
// strict decoder sees no new field) and the run-private caches; a v1
// descriptor from an old launcher is read by this worker.
func TestScratchEnvironmentV1WireAndReaderSupport(t *testing.T) {
	t.Parallel()
	control := t.TempDir()
	scratch, err := proofrun.CreateScratchRun(control)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = scratch.Cleanup(nil) })
	group := testpolicy.Group{ID: "g", Adapter: "command", EnvironmentMode: "explicit", Env: map[string]string{"PATH": "/usr/bin:/bin"}}
	request := proofrun.TestRunRequest{Environment: []string{"HOME=" + t.TempDir(), "GOENV=off", "GOCACHE=/outer/go-build"}}
	request.Contract.Groups = []testpolicy.Group{group}
	request.Plan.SelectedGroups = []string{group.ID}
	request.BindScratch(scratch, nil)
	if err := proofrun.PrepareScratchEnvironmentFor(&request, scratch, proofrun.ScratchEnvironmentPolicyV1); err != nil {
		t.Fatal(err)
	}
	wire, err := json.Marshal(request.ScratchEnvironment)
	if err != nil {
		t.Fatal(err)
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(wire, &keys); err != nil {
		t.Fatal(err)
	}
	for key := range keys {
		if !map[string]bool{"policy": true, "run": true, "root": true, "goEnv": true, "goEnvDigest": true, "groups": true}[key] {
			t.Fatalf("v1 descriptor wire carries %q, which an old worker's strict decoder refuses: %s", key, wire)
		}
	}
	packet := filepath.Join(t.TempDir(), "request.json")
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(packet, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	var worker proofrun.TestRunRequest
	if err := strictjson.Read(packet, &worker); err != nil {
		t.Fatal(err)
	}
	if err := proofrun.ValidateScratchEnvironment(worker, scratch); err != nil {
		t.Fatalf("upgraded worker refused a v1 descriptor: %v", err)
	}
	gocacheDir := filepath.Join(scratch.Root(), "gocache")
	if info, err := os.Stat(gocacheDir); err != nil || !info.IsDir() {
		t.Fatalf("v1 run-private cache %s: %v", gocacheDir, err)
	}
}

// The mixed-generation witness against a real built worker engine (3.1):
// set METASYSTEM_A5_WORKER_ENGINE to an engine built at another commit and
// METASYSTEM_A5_WORKER_POLICY to the policy this frontend must write for it.
func TestScratchEnvironmentPolicyAgainstABuiltWorker(t *testing.T) {
	t.Parallel()
	engine, want := os.Getenv("METASYSTEM_A5_WORKER_ENGINE"), os.Getenv("METASYSTEM_A5_WORKER_POLICY")
	if engine == "" || want == "" {
		t.Skip("set METASYSTEM_A5_WORKER_ENGINE and METASYSTEM_A5_WORKER_POLICY to a built worker and its expected policy")
	}
	capabilities, err := RequireWorkerCapabilities(t.Context(), engine, []string{"PATH=/usr/bin:/bin"})
	if err != nil {
		t.Fatal(err)
	}
	if got := chooseScratchEnvironmentPolicy(capabilities.ScratchEnvironmentPolicies); got != want {
		t.Fatalf("worker %s reports %+v: this frontend writes %q, want %q", engine, capabilities, got, want)
	}
	t.Logf("worker %s reports %+v: this frontend writes %s", engine, capabilities, want)
}

// The handshake is read from stdout only (structured-output U1): a note
// the engine writes on stderr never breaks its JSON answer.
func TestWorkerCapabilitiesReadStdoutOnly(t *testing.T) {
	t.Parallel()
	data, err := json.Marshal(CurrentWorkerCapabilities())
	if err != nil {
		t.Fatal(err)
	}
	engine := filepath.Join(t.TempDir(), "engine")
	script := "#!/bin/sh\nprintf 'a note for a person\\n' >&2\nprintf '%s\\n' " + shellquote.Quote(string(data)) + "\n"
	if err := testexec.WriteFile(engine, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := RequireWorkerCapabilities(t.Context(), engine, []string{"PATH=/usr/bin:/bin"}); err != nil {
		t.Fatalf("a stderr note broke the handshake: %v", err)
	}
}
