package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// The witness scenarios below port scripts/agents/witness-gate-fixtures.sh.
// Each refusal world breaks gofmt (status 79), so an accidental acceptance
// returns green while the lawful full fallback fails at the proof the
// witness would have omitted.

func brokenProofWorld(t *testing.T) *gateWorld {
	w := newGateWorld(t)
	w.statuses["gofmt"], w.outputs["gofmt"] = 79, "fixture: gofmt proof is broken\n"
	return w
}

// frozenConsumer freezes the world's tree, writes the controller's witness
// for the frozen bytes, and copies them to a separate consumer directory.
func frozenConsumer(t *testing.T, w *gateWorld, name string, controller int64) (string, witness) {
	t.Helper()
	frozen, err := proofrun.Freeze(w.root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = proofrun.CleanupFrozenExport(frozen.SnapshotRoot) })
	wit := w.writeWitness(name, frozen.Root, controller)
	consumer := filepath.Join(filepath.Dir(w.root), "consumer-"+name, "metasystem")
	copyTree(t, frozen.Root, consumer)
	if err := os.MkdirAll(filepath.Join(consumer, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(filepath.Join(consumer, "bin", "metasystem"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return consumer, wit
}

func TestWitnessRefusalsReachTheFullProof(t *testing.T) {
	t.Parallel()
	controller := int64(os.Getppid())
	for _, test := range []struct {
		name  string
		scope string
		setup func(t *testing.T, w *gateWorld, wit *witness)
		// refusal is the reason the acceptance names on stderr.
		refusal string
	}{
		{name: "wrong-start-identity", scope: "ENGINE", refusal: "witness consumer is not descended from its exact live controller identity",
			setup: func(t *testing.T, w *gateWorld, wit *witness) {
				editWitness(t, wit.path, func(s string) string {
					started := regexp.MustCompile(`"startedAtSec":([0-9]+)`).FindStringSubmatch(s)[1]
					value, _ := strconv.ParseInt(started, 10, 64)
					return strings.Replace(s, `"startedAtSec":`+started, `"startedAtSec":`+strconv.FormatInt(value+1, 10), 1)
				})
			}},
		{name: "changed-payload", scope: "ENGINE", refusal: "full-tree manifest digest mismatch between witness and consumer",
			setup: func(t *testing.T, w *gateWorld, _ *witness) {
				w.write("docs/payload.md", "payload changed after ENGINE proof\n")
			}},
		{name: "changed-payload-delivery", scope: "DELIVERY", refusal: "go gate: witness not accepted; running the full gate",
			setup: func(t *testing.T, w *gateWorld, _ *witness) {
				w.write("docs/payload.md", "payload changed after ENGINE proof\n")
			}},
		{name: "changed-engine", scope: "ENGINE", refusal: "full-tree manifest digest mismatch between witness and consumer",
			setup: func(t *testing.T, w *gateWorld, _ *witness) {
				w.write("internal/fixture/fixture.go", "package fixture\nvar Changed = true\n")
			}},
		{name: "absent-scope", scope: "", refusal: "go gate: witness not accepted; running the full gate"},
		{name: "policy-version", scope: "ENGINE", refusal: "behavior-surface policy version mismatch between witness and consumer (theirs 999",
			setup: func(t *testing.T, w *gateWorld, wit *witness) {
				editWitness(t, wit.path, func(s string) string {
					return regexp.MustCompile(`"policyVersion":[0-9]+`).ReplaceAllString(s, `"policyVersion":999`)
				})
			}},
		{name: "toolchain-mismatch", scope: "ENGINE", refusal: "toolchain identity mismatch between witness and consumer (theirs ffffffff",
			setup: func(t *testing.T, w *gateWorld, wit *witness) {
				editWitness(t, wit.path, func(s string) string {
					return regexp.MustCompile(`"toolchainIdentity":"[^"]*"`).ReplaceAllString(s, `"toolchainIdentity":"`+strings.Repeat("f", 64)+`"`)
				})
			}},
		{name: "foreign-run", scope: "ENGINE", refusal: "witness run or projection identity is incomplete",
			setup: func(t *testing.T, w *gateWorld, wit *witness) { wit.run = "other-run" }},
		{name: "foreign-root", scope: "ENGINE", refusal: "witness lies outside the controller state root",
			setup: func(t *testing.T, w *gateWorld, wit *witness) {
				other := filepath.Join(w.tmp, "other-state")
				if err := os.MkdirAll(other, 0o700); err != nil {
					t.Fatal(err)
				}
				wit.root = other
			}},
		{name: "deleted-witness", scope: "ENGINE", refusal: "witness is not a plain regular file",
			setup: func(t *testing.T, w *gateWorld, wit *witness) {
				if err := os.Remove(wit.path); err != nil {
					t.Fatal(err)
				}
			}},
		{name: "loose-permissions", scope: "ENGINE", refusal: "witness permissions are not 0700/0600",
			setup: func(t *testing.T, w *gateWorld, wit *witness) {
				if err := os.Chmod(wit.path, 0o644); err != nil {
					t.Fatal(err)
				}
			}},
		{name: "seed-fenced-off", scope: "ENGINE", refusal: "seed or force run; witness fenced off",
			setup: func(t *testing.T, w *gateWorld, _ *witness) { w.setenv("METASYSTEM_GATE_FORCE=1") }},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			w := brokenProofWorld(t)
			wit := w.writeWitness(test.name, w.root, controller)
			if test.setup != nil {
				test.setup(t, w, &wit)
			}
			w.setenv(wit.env(test.scope)...)
			code := w.gate()
			if !w.reachedFullProof(code) {
				t.Fatalf("refusal skipped the deliberately broken proof: exit %d\n%s", code, w.output())
			}
			if !strings.Contains(w.stderr.String(), test.refusal) {
				t.Fatalf("stderr lacks %q:\n%s", test.refusal, w.stderr.String())
			}
			if strings.Contains(w.stdout.String(), "outer witness") {
				t.Fatalf("a refused witness was reported as reused")
			}
		})
	}
}

func TestWitnessForeignControllerIsCorrelationWithoutAuthority(t *testing.T) {
	t.Parallel()
	w := brokenProofWorld(t)
	wit := w.writeWitness("foreign-ancestry", w.root, foreignController(t))
	w.setenv(wit.env("ENGINE")...)
	if code := w.gate(); !w.reachedFullProof(code) ||
		!strings.Contains(w.stderr.String(), "witness consumer is not descended from its exact live controller identity") {
		t.Fatalf("exit %d:\n%s", code, w.output())
	}
}

func TestWitnessWithoutAnyWitnessTheBrokenProofIsAHardFailure(t *testing.T) {
	t.Parallel()
	w := brokenProofWorld(t)
	if code := w.gate(); !w.reachedFullProof(code) {
		t.Fatalf("exit %d:\n%s", code, w.output())
	}
}

func TestWitnessEngineAcceptanceRebuildsFromThePrivateFrozenExport(t *testing.T) {
	t.Parallel()
	w := brokenProofWorld(t)
	consumer, wit := frozenConsumer(t, w, "frozen-identical", int64(os.Getppid()))
	// Runtime state below artifacts/ is outside the manifest and still reuses.
	if err := os.MkdirAll(filepath.Join(consumer, "artifacts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(consumer, "artifacts", "state"), []byte("runtime-only mutation\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	w.setenv(wit.env("ENGINE")...)
	if code := w.gateAt(consumer); code != 0 {
		t.Fatalf("matching ENGINE did not skip: exit %d\n%s", code, w.output())
	}
	if !strings.Contains(w.stdout.String(), "go gate: PASSED (outer witness ") {
		t.Fatalf("acceptance did not report witness reuse:\n%s", w.stdout.String())
	}
	builds := w.called("go build")
	if len(builds) != 1 {
		t.Fatalf("builds = %v, want the one stamped rebuild", builds)
	}
	build := builds[0]
	if build.dir == consumer || !regexp.MustCompile(`/metasystem-witness-freeze-[^/]+/tree$`).MatchString(build.dir) {
		t.Fatalf("accepted witness built in %s, want a private frozen export", build.dir)
	}
	if !regexp.MustCompile(`^witness-[0-9a-f]{12}$`).MatchString(stampOf(build.args)) {
		t.Fatalf("rebuild stamp %q does not carry the witness digest", stampOf(build.args))
	}
	if envValue(build.env, "GOFLAGS") != "-mod=readonly -trimpath" || envValue(build.env, "METASYSTEM_GATE_FROZEN_TOOLCHAIN") != "1" {
		t.Fatalf("frozen build environment: %q", build.env)
	}
	installed, err := os.ReadFile(filepath.Join(consumer, "bin", "metasystem"))
	if err != nil || !strings.Contains(string(installed), "stamp witness-") {
		t.Fatalf("the proven engine was not installed into the live consumer: %q %v", installed, err)
	}
	if len(w.called("gofmt")) != 0 {
		t.Fatalf("an accepted witness still ran the full proof")
	}
	if _, err := os.Stat(filepath.Dir(build.dir)); !os.IsNotExist(err) {
		t.Fatalf("the consumer's frozen snapshot survived: %v", err)
	}

	probe := brokenProofWorld(t)
	probe.setenv(wit.env("ENGINE")...)
	if code := probe.gateAt(consumer, "--witness-check-only"); code != 0 || !strings.HasPrefix(probe.stdout.String(), "witness acceptable: ") {
		t.Fatalf("check-only on the identical copy: exit %d\n%s", code, probe.output())
	}
	if len(probe.called("go build")) != 0 {
		t.Fatalf("the probe built an engine")
	}
}

func TestWitnessMutationDuringTheSkipBuildRunsTheFullGate(t *testing.T) {
	t.Parallel()
	w := brokenProofWorld(t)
	consumer, wit := frozenConsumer(t, w, "frozen-consumer-recheck", int64(os.Getppid()))
	w.buildHook = func(root, _ string) {
		if err := os.WriteFile(filepath.Join(root, "internal", "fixture", "fixture.go"), []byte("package fixture\n// mutated during build\n"), 0o644); err != nil {
			t.Error(err)
		}
	}
	w.setenv(wit.env("ENGINE")...)
	code := w.gateAt(consumer)
	if !w.reachedFullProof(code) || !strings.Contains(w.stderr.String(), "go gate: witness changed during the skip-path build; running the full gate") {
		t.Fatalf("post-build mismatch: exit %d\n%s", code, w.output())
	}
}

func TestWitnessClosureMutationAndForeignControllerOnFrozenConsumers(t *testing.T) {
	t.Parallel()
	mutated := brokenProofWorld(t)
	consumer, wit := frozenConsumer(t, mutated, "frozen-closure-mutation", int64(os.Getppid()))
	if err := os.WriteFile(filepath.Join(consumer, "internal", "fixture", "fixture.go"), []byte("package fixture\nvar Changed = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mutated.setenv(wit.env("ENGINE")...)
	if code := mutated.gateAt(consumer); !mutated.reachedFullProof(code) {
		t.Fatalf("a changed closure byte skipped the full gate: exit %d\n%s", code, mutated.output())
	}
	// The full fallback ran in the consumer's own frozen export, never the
	// live consumer tree.
	for _, dir := range mutated.gofmtDir {
		if dir == consumer || !regexp.MustCompile(`/metasystem-witness-freeze-[^/]+/tree$`).MatchString(dir) {
			t.Fatalf("full fallback ran in %s", dir)
		}
	}

	foreign := brokenProofWorld(t)
	consumer, wit = frozenConsumer(t, foreign, "frozen-foreign-controller", foreignController(t))
	foreign.setenv(wit.env("ENGINE")...)
	if code := foreign.gateAt(consumer); !foreign.reachedFullProof(code) {
		t.Fatalf("right bytes compensated for a foreign controller: exit %d\n%s", code, foreign.output())
	}
}

func TestWitnessFrozenConsumerRefusesAlternateGoInputs(t *testing.T) {
	t.Parallel()
	for _, flags := range []string{"-modfile=outside.mod", "-overlay=outside.json", "-mod=mod -overlay outside.json"} {
		w := newGateWorld(t)
		wit := w.writeWitness("frozen-flag", w.root, int64(os.Getppid()))
		w.setenv(append(wit.env("ENGINE"), "GOFLAGS="+flags)...)
		if code := w.gate("--witness-check-only"); code != 1 ||
			!strings.Contains(w.stderr.String(), "go gate: frozen witness consumer refused: GOFLAGS may not contain -modfile or -overlay") {
			t.Fatalf("GOFLAGS=%q: exit %d\n%s", flags, code, w.output())
		}
	}
}

func TestWitnessDeliveryScopeComparesThePayload(t *testing.T) {
	t.Parallel()
	w := newGateWorld(t)
	consumer, wit := frozenConsumer(t, w, "delivery", int64(os.Getppid()))
	w.setenv(wit.env("DELIVERY")...)
	if code := w.gateAt(consumer, "--witness-check-only"); code != 0 {
		t.Fatalf("identical DELIVERY: exit %d\n%s", code, w.output())
	}
	changed := newGateWorld(t)
	changed.setenv(wit.env("DELIVERY")...)
	editWitness(t, wit.path, func(s string) string {
		return regexp.MustCompile(`"payloadDigest":"[^"]*"`).ReplaceAllString(s, `"payloadDigest":"`+strings.Repeat("0", 64)+`"`)
	})
	if code := changed.gateAt(consumer, "--witness-check-only"); code != 3 ||
		!strings.Contains(changed.stderr.String(), "PAYLOAD surface digest mismatch between witness and delivery consumer (theirs 00000000") {
		t.Fatalf("changed PAYLOAD authorized DELIVERY reuse: exit %d\n%s", code, changed.output())
	}
	unsafe := newGateWorld(t)
	unsafe.setenv(wit.env("DELIVERY")...)
	editWitness(t, wit.path, func(s string) string {
		return strings.Replace(s, `"payloadManifest":"payload-paths.nul"`, `"payloadManifest":"../payload-paths.nul"`, 1)
	})
	if code := unsafe.gateAt(consumer, "--witness-check-only"); code != 3 || !strings.Contains(unsafe.stderr.String(), "witness payload manifest name is unsafe") {
		t.Fatalf("unsafe manifest name: exit %d\n%s", code, unsafe.output())
	}
}

func TestWitnessWithoutACompleteManifestIsRefusedExplicitly(t *testing.T) {
	t.Parallel()
	w := newGateWorld(t)
	wit := w.writeWitness("manifestless-legacy", w.root, int64(os.Getppid()))
	editWitness(t, wit.path, func(s string) string {
		return regexp.MustCompile(`"manifestDigest":"[a-f0-9]*",`).ReplaceAllString(s, "")
	})
	w.setenv(wit.env("ENGINE")...)
	if code := w.gate("--witness-check-only"); code != 3 ||
		!strings.Contains(w.stderr.String(), "witness has no compatible complete-project manifest identity") {
		t.Fatalf("exit %d\n%s", code, w.output())
	}
}

func TestWitnessPolicyWithoutTheSkipFamilyRunsTheFullGate(t *testing.T) {
	t.Parallel()
	w := brokenProofWorld(t)
	consumer, wit := frozenConsumer(t, w, "unauthorized-skip", int64(os.Getppid()))
	w.owners.surface = func() (behaviorsurface.Policy, error) {
		policy, err := behaviorsurface.Load()
		policy.WitnessSkips = nil
		return policy, err
	}
	w.setenv(wit.env("ENGINE")...)
	if code := w.gateAt(consumer); !w.reachedFullProof(code) ||
		!strings.Contains(w.stderr.String(), "go gate: the witness holds, but the surface policy does not let it stand in; running the full gate") {
		t.Fatalf("exit %d\n%s", code, w.output())
	}
	probe := brokenProofWorld(t)
	probe.owners.surface = w.owners.surface
	probe.setenv(wit.env("ENGINE")...)
	if code := probe.gateAt(consumer, "--witness-check-only"); code != 3 ||
		!strings.Contains(probe.stderr.String(), "behavior-surface policy does not authorize the witness ENGINE gate") {
		t.Fatalf("probe exit %d\n%s", code, probe.output())
	}
}
