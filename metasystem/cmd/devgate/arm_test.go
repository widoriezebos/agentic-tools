package main

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

// armState reads the state file the controller hands its caller.
func readArmState(t *testing.T, path string) (map[string]string, []string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	values := map[string]string{}
	var unsets []string
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if names, ok := strings.CutPrefix(line, "unset "); ok {
			unsets = append(unsets, strings.Fields(names)...)
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		name, value, _ := strings.Cut(line, "=")
		values[name] = strings.TrimSuffix(strings.TrimPrefix(value, "'"), "'")
	}
	return values, unsets
}

func (w *gateWorld) arm(fallback string, extra ...string) (int, string) {
	state := filepath.Join(filepath.Dir(w.root), "arm-state.sh")
	args := append([]string{"--arm", fallback, "--controller-pid", strconv.Itoa(os.Getppid()), "--state-out", state}, extra...)
	return w.gate(args...), state
}

func TestArmProducesAWitnessFromACompleteFrozenExport(t *testing.T) {
	t.Parallel()
	w := newGateWorld(t)
	w.write("application.txt", "dirty parent input\n")
	code, state := w.arm("none")
	if code != 0 {
		t.Fatalf("exit %d:\n%s", code, w.output())
	}
	for _, line := range []string{"gate witness armed from complete frozen project\n", "gate witness armed for this run's nested validations\n"} {
		if !strings.Contains(w.stdout.String(), line) {
			t.Fatalf("stdout lacks %q:\n%s", line, w.stdout.String())
		}
	}
	values, unsets := readArmState(t, state)
	witnessPath := values["METASYSTEM_GATE_WITNESS"]
	if values["witness_state"] == "" || values["METASYSTEM_GATE_WITNESS_ROOT"] != values["witness_state"] ||
		witnessPath != filepath.Join(values["witness_state"], "witness.json") || values["witness_engine_reused"] != "0" ||
		!strings.HasPrefix(values["METASYSTEM_GATE_WITNESS_RUN"], "run-"+strconv.Itoa(os.Getpid())+"-") ||
		len(unsets) != 1 || unsets[0] != "METASYSTEM_GATE_WITNESS_EXPORT" {
		t.Fatalf("state = %v unsets %v", values, unsets)
	}
	if info, err := os.Stat(values["witness_state"]); err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("witness state root: %v %v", info, err)
	}
	data, err := os.ReadFile(witnessPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	for _, pattern := range []string{`"manifestDigest":"[a-f0-9]{64}"`, `"summary":"full gate in frozen proof tree"`,
		`"controller":\{"pid":` + strconv.Itoa(os.Getppid()) + `,`, `"ratchetBaseline":"scripts/agents/coverage-ratchet`,
		`"goVersion":"go version go1.fixture darwin/arm64"`, `"payloadManifest":"witness.payload-paths.nul"`} {
		if !regexp.MustCompile(pattern).MatchString(content) {
			t.Fatalf("witness lacks %s:\n%s", pattern, content)
		}
	}
	// The recorded manifest is this very tree's: the witness verifies here.
	digest := regexp.MustCompile(`"manifestDigest":"([a-f0-9]{64})"`).FindStringSubmatch(content)[1]
	if _, err := proofrun.Verify(w.root, digest); err != nil {
		t.Fatalf("the armed witness does not verify against its producer: %v", err)
	}
	// The full proof ran inside the private export, and its stamped engine
	// was published into the live tree.
	for _, dir := range w.gofmtDir {
		if !regexp.MustCompile(`/metasystem-witness-freeze-[^/]+/tree$`).MatchString(dir) {
			t.Fatalf("the producing gate ran in %s", dir)
		}
	}
	installed, err := os.ReadFile(filepath.Join(w.root, "bin", "metasystem"))
	if err != nil || !strings.Contains(string(installed), "stamp witness-") {
		t.Fatalf("published engine = %q %v", installed, err)
	}
	build := w.called("-ldflags")[0]
	if envValue(build.env, "METASYSTEM_GATE_WITNESS_WRITE") == "" || envValue(build.env, "METASYSTEM_PROOF_EXECUTION_ROOT") != w.root {
		t.Fatalf("snapshot gate environment: %q", build.env)
	}

	// The armed witness is then accepted by a consumer of the same bytes.
	consumer := newGateWorld(t)
	consumer.root = w.root
	consumer.statuses["gofmt"] = 79
	consumer.setenv("METASYSTEM_GATE_WITNESS="+witnessPath, "METASYSTEM_GATE_WITNESS_ROOT="+values["witness_state"],
		"METASYSTEM_GATE_WITNESS_RUN="+values["METASYSTEM_GATE_WITNESS_RUN"], "METASYSTEM_GATE_WITNESS_CONSUMER_SCOPE=ENGINE")
	if code := consumer.gate(); code != 0 || !strings.Contains(consumer.stdout.String(), "outer witness") {
		t.Fatalf("the armed witness was not reusable: exit %d\n%s", code, consumer.output())
	}
}

func TestArmRefusesAlternateGoInputs(t *testing.T) {
	t.Parallel()
	w := newGateWorld(t)
	w.setenv("GOFLAGS=-overlay=outside.json")
	if code, _ := w.arm("none"); code != 1 || !strings.Contains(w.stderr.String(), "witness gate arming voided: GOFLAGS may not contain -modfile or -overlay") {
		t.Fatalf("exit %d\n%s", code, w.output())
	}
}

func TestArmConsumesAnInheritedWitness(t *testing.T) {
	t.Parallel()
	// A refused inherited witness under plain fallback runs the complete
	// gate from the consumer's frozen export, never the live tree, and the
	// caller scrubs the refused handoff.
	w := brokenProofWorld(t)
	wit := w.writeWitness("inherited", w.root, int64(os.Getppid()))
	w.write("internal/fixture/fixture.go", "package fixture\nvar Changed = true\n")
	w.setenv(wit.env("")...)
	code, state := w.arm("plain")
	if !w.reachedFullProof(code) {
		t.Fatalf("wrapper fallback did not reach the broken full proof: exit %d\n%s", code, w.output())
	}
	for _, dir := range w.gofmtDir {
		if dir == w.root || !regexp.MustCompile(`/metasystem-witness-freeze-[^/]+/tree$`).MatchString(dir) {
			t.Fatalf("wrapper fallback ran in %s", dir)
		}
	}
	values, unsets := readArmState(t, state)
	if values["witness_engine_reused"] != "0" || strings.Join(unsets, " ") !=
		"METASYSTEM_GATE_WITNESS METASYSTEM_GATE_WITNESS_ROOT METASYSTEM_GATE_WITNESS_RUN METASYSTEM_GATE_WITNESS_EXPORT" {
		t.Fatalf("state = %v unsets %v", values, unsets)
	}

	// With no fallback, a refused witness runs nothing.
	none := brokenProofWorld(t)
	wit = none.writeWitness("inherited-none", none.root, int64(os.Getppid()))
	none.write("internal/fixture/fixture.go", "package fixture\nvar Changed = true\n")
	none.setenv(wit.env("")...)
	if code, _ := none.arm("none"); code != 0 || len(none.gofmtDir) != 0 {
		t.Fatalf("no-fallback refusal ran a gate: exit %d gofmt %v\n%s", code, none.gofmtDir, none.output())
	}

	// An accepted inherited witness is reused and kept.
	reused := newGateWorld(t)
	reused.statuses["gofmt"] = 79
	wit = reused.writeWitness("inherited-accepted", reused.root, int64(os.Getppid()))
	reused.setenv(wit.env("")...)
	code, state = reused.arm("none")
	values, unsets = readArmState(t, state)
	if code != 0 || values["witness_engine_reused"] != "1" || len(unsets) != 0 {
		t.Fatalf("accepted inherited witness: exit %d state %v unsets %v\n%s", code, values, unsets, reused.output())
	}
}

func TestArmFallbackChoices(t *testing.T) {
	t.Parallel()
	refuseFreeze := func(w *gateWorld) {
		w.owners.freeze = func(string) (proofrun.FrozenExport, error) { return proofrun.FrozenExport{}, errors.New("unfreezable") }
	}
	plain := brokenProofWorld(t)
	refuseFreeze(plain)
	if code, _ := plain.arm("plain"); !plain.reachedFullProof(code) || !strings.Contains(plain.stderr.String(), "witness gate preparation did not complete") {
		t.Fatalf("plain preparation fallback: exit %d\n%s", code, plain.output())
	}
	if len(plain.gofmtDir) != 1 || plain.gofmtDir[0] != plain.root {
		t.Fatalf("plain fallback gates = %v, want one live-tree gate", plain.gofmtDir)
	}

	none := brokenProofWorld(t)
	refuseFreeze(none)
	if code, state := none.arm("none"); code != 0 || len(none.gofmtDir) != 0 {
		t.Fatalf("none preparation fallback ran a gate: exit %d\n%s", code, none.output())
	} else if values, _ := readArmState(t, state); values["witness_state"] != "" {
		t.Fatalf("an unprepared witness left state %v", values)
	}

	for _, env := range []string{"METASYSTEM_GATE_FORCE=1", "METASYSTEM_COVERAGE_RATCHET_SEED=1"} {
		ineligible := brokenProofWorld(t)
		ineligible.setenv(env)
		if code, _ := ineligible.arm("plain"); !ineligible.reachedFullProof(code) || len(ineligible.gofmtDir) != 1 || ineligible.gofmtDir[0] != ineligible.root {
			t.Fatalf("%s: exit %d gates %v", env, code, ineligible.gofmtDir)
		}
	}
	delivery := brokenProofWorld(t)
	if code, _ := delivery.arm("plain", "--delivery"); !delivery.reachedFullProof(code) || delivery.gofmtDir[0] != delivery.root {
		t.Fatalf("delivery: exit %d gates %v", code, delivery.gofmtDir)
	}
	if code, _ := brokenProofWorld(t).arm("none", "--delivery"); code != 0 {
		t.Fatalf("an ineligible no-fallback caller ran something: %d", code)
	}
}

func TestArmSnapshotFailureIsTerminalAndARefusalFallsBack(t *testing.T) {
	t.Parallel()
	failed := brokenProofWorld(t)
	code, state := failed.arm("plain")
	if code != 1 || !strings.Contains(failed.stderr.String(), "witness gate failed in its frozen project (exit 1): the red above is the answer, no fallback") {
		t.Fatalf("exit %d\n%s", code, failed.output())
	}
	if len(failed.gofmtDir) != 1 {
		t.Fatalf("an executed gate was retried: %v", failed.gofmtDir)
	}
	if values, _ := readArmState(t, state); values["witness_state"] != "" {
		t.Fatalf("a failed arming left witness state %v", values)
	}

	// Exit 3 is a refusal inside the frozen project: the plain fallback runs
	// the live tree's gate (here, the standalone relaunch it performs).
	refused := newGateWorld(t)
	refused.statuses["worker"], refused.statuses["launch"] = 1, 3
	code, _ = refused.arm("plain")
	launches := refused.called("proof-run launch")
	if code != 3 || !strings.Contains(refused.stderr.String(), "witness gate refused in its frozen project (reason above); falling back to the plain gate") ||
		len(launches) != 2 || launches[0].dir == refused.root || launches[1].dir != refused.root {
		t.Fatalf("exit %d launches %v\n%s", code, launches, refused.output())
	}
	refusedNone := newGateWorld(t)
	refusedNone.statuses["worker"], refusedNone.statuses["launch"] = 1, 3
	if code, _ = refusedNone.arm("none"); code != 3 || len(refusedNone.called("proof-run launch")) != 1 {
		t.Fatalf("no-fallback refusal: exit %d\n%s", code, refusedNone.output())
	}

	// A tree outside the hashed closure proves without publishing a witness.
	unpublished := newGateWorld(t)
	unpublished.outputs["list-dirs"] = "/elsewhere/package\n"
	code, _ = unpublished.arm("plain")
	if code != 1 || !strings.Contains(unpublished.stderr.String(), "go gate: witness not written (packages outside the hashed closure: /elsewhere/package)") ||
		!strings.Contains(unpublished.stderr.String(), "witness gate completed without publishing witness evidence") {
		t.Fatalf("exit %d\n%s", code, unpublished.output())
	}
}

func TestArmControllerMustBeThisRunsAncestor(t *testing.T) {
	t.Parallel()
	w := brokenProofWorld(t)
	state := filepath.Join(filepath.Dir(w.root), "arm-state.sh")
	code := w.gate("--arm", "none", "--controller-pid", strconv.FormatInt(foreignController(t), 10), "--state-out", state)
	if code != 0 || !strings.Contains(w.stderr.String(), "is not this gate's live ancestor in its session") ||
		!strings.Contains(w.stderr.String(), "witness gate preparation did not complete") || len(w.gofmtDir) != 0 {
		t.Fatalf("a foreign controller armed a witness: exit %d\n%s", code, w.output())
	}
	for _, pid := range []string{"1", "0"} {
		w := newGateWorld(t)
		code := w.gate("--arm", "none", "--controller-pid", pid, "--state-out", state)
		if code == 0 && !strings.Contains(w.stderr.String(), "witness gate preparation did not complete") {
			t.Fatalf("controller %s armed: exit %d\n%s", pid, code, w.output())
		}
	}
}
