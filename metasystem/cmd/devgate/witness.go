package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// The boundary-scoped gate witness (D33). One validation run, one gate: the
// outer suite runs the full gate inside a frozen export and hands the
// resulting witness to descendant validations, which skip re-proving
// byte-identical ENGINE content. Everything here fails toward the full
// gate: any doubt, run it all.

var (
	hex64          = regexp.MustCompile(`^[a-f0-9]{64}$`)
	positiveDigits = regexp.MustCompile(`^[1-9][0-9]*$`)
	digits         = regexp.MustCompile(`^[0-9]+$`)
	alternateInput = regexp.MustCompile(`\s-(modfile|overlay)(=|\s)`)
)

// witnessField extracts one field the way the gate always read the witness:
// per line, the last occurrence on the line, lines joined by newline.
func witnessField(content, pattern string) string {
	expression := regexp.MustCompile(`^.*` + pattern + `.*$`)
	var found []string
	for _, line := range strings.Split(strings.TrimSuffix(content, "\n"), "\n") {
		if match := expression.FindStringSubmatch(line); match != nil {
			found = append(found, match[1])
		}
	}
	return strings.Join(found, "\n")
}

// surfaceIdentity is the policy version and digest of one projection of root.
// The engine built from the judged bytes owns every projection; the version
// is compared beside the digest, and toolchain identity is an independent
// equality witness never mixed into a byte digest.
func (g *gateRun) surfaceIdentity(projection behaviorsurface.Projection, manifest string) (int, string, error) {
	policy, err := g.d.owners.surface()
	if err != nil {
		return 0, "", err
	}
	var digest string
	if manifest == "" {
		digest, err = policy.DigestWithPrefix(g.root, projection, "")
	} else {
		var data []byte
		data, err = os.ReadFile(manifest)
		if err != nil {
			return 0, "", err
		}
		if len(data) > 0 && data[len(data)-1] != 0 {
			return 0, "", errors.New("behavior-surface path manifest is not NUL-terminated")
		}
		parts := strings.Split(strings.TrimSuffix(string(data), "\x00"), "\x00")
		if len(data) == 0 {
			parts = nil
		}
		digest, err = policy.DigestListed(g.root, projection, parts)
	}
	if err != nil {
		return 0, "", err
	}
	if !hex64.MatchString(digest) || policy.Version < 1 {
		return 0, "", errors.New("the behavior-surface hash report is malformed")
	}
	return policy.Version, digest, nil
}

// toolchainIdentity hashes `go version` and the Go environment that selects
// what a build means.
func (g *gateRun) toolchainIdentity() (string, error) {
	var out bytes.Buffer
	env := g.env.list()
	if err := g.d.goTool(g.ctx, g.root, env, []string{"version"}, &out, nil); err != nil {
		return "", err
	}
	if err := g.d.goTool(g.ctx, g.root, env, []string{"env", "GOOS", "GOARCH", "GOFLAGS", "GOWORK", "GOEXPERIMENT", "CGO_ENABLED", "GOTOOLCHAIN"}, &out, nil); err != nil {
		return "", err
	}
	sum := sha256.Sum256(out.Bytes())
	return hex.EncodeToString(sum[:]), nil
}

func frozenToolchainRefusal(goflags string) string {
	if alternateInput.MatchString(" " + goflags + " ") {
		return "GOFLAGS may not contain -modfile or -overlay"
	}
	return ""
}

// witnessRefusal names why a tree where ./... could reach beyond the hashed
// closure refuses witness use in both directions: go.work or vendor/,
// replace directives, or any package outside cmd/ and internal/ (and the
// root package directory). The empty string accepts.
func (g *gateRun) witnessRefusal() string {
	if present(filepath.Join(g.root, "go.work")) || present(filepath.Join(g.root, "..", "go.work")) {
		return "go.work present"
	}
	if info, err := os.Stat(filepath.Join(g.root, "vendor")); err == nil && info.IsDir() {
		return "vendor/ present"
	}
	if module, err := os.ReadFile(filepath.Join(g.root, "go.mod")); err == nil {
		for _, line := range strings.Split(string(module), "\n") {
			if strings.HasPrefix(line, "replace") {
				return "go.mod carries replace directives"
			}
		}
	}
	var listed bytes.Buffer
	_ = g.d.goTool(g.ctx, g.root, g.env.list(), []string{"list", "-p=" + g.workers, "-f", "{{.Dir}}", "./..."}, &listed, nil)
	var outside []string
	for _, dir := range strings.Split(strings.TrimRight(listed.String(), "\n"), "\n") {
		if dir == "" || dir == g.root || strings.HasPrefix(dir, g.root+"/cmd") || strings.HasPrefix(dir, g.root+"/internal") {
			continue
		}
		outside = append(outside, dir)
	}
	if len(outside) > 0 {
		return "packages outside the hashed closure: " + strings.Join(outside, "\n")
	}
	return ""
}

// witnessFencedOff: seed and force runs never read or write witnesses.
func (g *gateRun) witnessFencedOff() bool {
	return g.env.get("METASYSTEM_COVERAGE_RATCHET_SEED") == "1" || g.env.get("METASYSTEM_GATE_FORCE") == "1"
}

// witnessAcceptable is the accept decision: the recorded engine digest, or a
// refusal whose reason goes to stderr. The state root and run id come from
// the controller's own environment, never from the witness; the witness must
// be a non-symlink regular 0600 file under the 0700 controller state root,
// written by a live controller this process descends from.
func (g *gateRun) witnessAcceptable() (string, bool) {
	stderr := g.d.stderr
	refuse := func(reason string) (string, bool) {
		fmt.Fprintln(stderr, reason)
		return "", false
	}
	witness, stateRoot, run := g.env.get("METASYSTEM_GATE_WITNESS"), g.env.get("METASYSTEM_GATE_WITNESS_ROOT"), g.env.get("METASYSTEM_GATE_WITNESS_RUN")
	if witness == "" || stateRoot == "" || run == "" {
		return refuse("witness handoff incomplete")
	}
	scope := g.env.get("METASYSTEM_GATE_WITNESS_CONSUMER_SCOPE")
	if scope != "ENGINE" && scope != "DELIVERY" {
		return refuse("witness consumer scope must be explicitly ENGINE or DELIVERY")
	}
	if g.witnessFencedOff() {
		return refuse("seed or force run; witness fenced off")
	}
	canonicalRoot, err := filepath.EvalSymlinks(stateRoot)
	if err == nil {
		canonicalRoot, err = filepath.Abs(canonicalRoot)
	}
	if info, statErr := os.Stat(canonicalRoot); err != nil || statErr != nil || !info.IsDir() {
		return refuse("witness state root unreadable")
	}
	if info, err := os.Lstat(witness); err != nil || !info.Mode().IsRegular() {
		return refuse("witness is not a plain regular file")
	}
	witnessDir, err := filepath.EvalSymlinks(filepath.Dir(witness))
	if err == nil {
		witnessDir, err = filepath.Abs(witnessDir)
	}
	canonicalWitness := filepath.Join(witnessDir, filepath.Base(witness))
	if err != nil || !strings.HasPrefix(canonicalWitness, canonicalRoot+"/") {
		return refuse("witness lies outside the controller state root")
	}
	rootInfo, _ := os.Stat(canonicalRoot)
	witnessInfo, err := os.Stat(canonicalWitness)
	if err != nil || rootInfo.Mode().Perm() != 0o700 || witnessInfo.Mode().Perm() != 0o600 {
		return refuse("witness permissions are not 0700/0600")
	}
	data, err := os.ReadFile(canonicalWitness)
	if err != nil {
		return refuse("witness is not a plain regular file")
	}
	content := string(data)
	recordedRun := witnessField(content, `"runId":"([^"]*)"`)
	recordedVersion := witnessField(content, `"policyVersion":([0-9][0-9]*)`)
	recordedDigest := witnessField(content, `"engineDigest":"([^"]*)"`)
	recordedPayload := witnessField(content, `"payloadDigest":"([^"]*)"`)
	recordedToolchain := witnessField(content, `"toolchainIdentity":"([^"]*)"`)
	recordedPayloadManifest := witnessField(content, `"payloadManifest":"([^"]*)"`)
	recordedManifestDigest := witnessField(content, `"manifestDigest":"([^"]*)"`)
	controllerPID := witnessField(content, `"controller":\{"pid":([0-9][0-9]*)`)
	controllerStarted := witnessField(content, `"controller":\{"pid":[0-9][0-9]*,"startedAtSec":([0-9][0-9]*)`)
	controllerTicks := witnessField(content, `"controller":\{"pid":[0-9][0-9]*,"startedAtSec":[0-9][0-9]*,"startTicks":([0-9][0-9]*)`)
	controllerBoot := witnessField(content, `"controller":\{"pid":[0-9][0-9]*,"startedAtSec":[0-9][0-9]*,"startTicks":[0-9][0-9]*,"bootId":"([^"]*)"`)
	if recordedRun != run || !positiveDigits.MatchString(recordedVersion) || !hex64.MatchString(recordedDigest) ||
		!hex64.MatchString(recordedToolchain) || !positiveDigits.MatchString(controllerPID) ||
		!positiveDigits.MatchString(controllerStarted) || !digits.MatchString(controllerTicks) {
		return refuse("witness run or projection identity is incomplete")
	}
	ticks, _ := strconv.ParseInt(controllerTicks, 10, 64)
	if ticks > 0 && controllerBoot == "" {
		return refuse("witness controller pair identity is incomplete")
	}
	if ticks == 0 && controllerBoot != "" {
		return refuse("witness controller pair identity is malformed")
	}
	// This is an outsider boundary, not a same-user privilege boundary: a
	// same-user process can already replace the engine or this program, and
	// that accepted risk gains no authority from the witness protocol.
	pid, _ := strconv.ParseInt(controllerPID, 10, 64)
	started, _ := strconv.ParseInt(controllerStarted, 10, 64)
	if g.d.owners.descendant(g.d.selfPid, identity.Ref{Pid: pid, StartedAtSec: started, StartTicks: ticks, BootID: controllerBoot}) != nil {
		return refuse("witness consumer is not descended from its exact live controller identity")
	}
	if refusal := g.witnessRefusal(); refusal != "" {
		return refuse("witness refused here: " + refusal)
	}
	if !hex64.MatchString(recordedManifestDigest) {
		return refuse("witness has no compatible complete-project manifest identity")
	}
	if _, err := g.d.owners.verify(g.root, recordedManifestDigest); err != nil {
		return refuse("full-tree manifest digest mismatch between witness and consumer")
	}
	localVersion, localDigest, err := g.surfaceIdentity(behaviorsurface.Engine, "")
	if err != nil {
		return refuse("consumer ENGINE surface identity could not be computed")
	}
	localToolchain, err := g.toolchainIdentity()
	if err != nil {
		return "", false
	}
	if strconv.Itoa(localVersion) != recordedVersion {
		return refuse(fmt.Sprintf("behavior-surface policy version mismatch between witness and consumer (theirs %s, ours ENGINE=%d)", recordedVersion, localVersion))
	}
	if localDigest != recordedDigest {
		return refuse(fmt.Sprintf("ENGINE surface digest mismatch between witness and consumer (theirs %s, ours %s)", recordedDigest[:8], localDigest[:8]))
	}
	if localToolchain != recordedToolchain {
		return refuse(fmt.Sprintf("toolchain identity mismatch between witness and consumer (theirs %s, ours %s)", recordedToolchain[:8], localToolchain[:8]))
	}
	if scope == "DELIVERY" {
		if !hex64.MatchString(recordedPayload) || recordedPayloadManifest == "" {
			return refuse("witness PAYLOAD identity is incomplete")
		}
		if strings.Contains(recordedPayloadManifest, "/") || strings.HasPrefix(recordedPayloadManifest, ".") {
			return refuse("witness payload manifest name is unsafe")
		}
		manifest := filepath.Join(canonicalRoot, recordedPayloadManifest)
		info, err := os.Lstat(manifest)
		if err != nil || !info.Mode().IsRegular() {
			return refuse("witness payload manifest is not a plain regular file")
		}
		if info.Mode().Perm() != 0o600 {
			return refuse("witness payload manifest permission is not 0600")
		}
		payloadVersion, payloadDigest, err := g.surfaceIdentity(behaviorsurface.Payload, manifest)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return "", false
		}
		if strconv.Itoa(payloadVersion) != recordedVersion {
			return refuse(fmt.Sprintf("behavior-surface policy version mismatch between witness and delivery PAYLOAD (theirs %s, ours %d)", recordedVersion, payloadVersion))
		}
		if payloadDigest != recordedPayload {
			return refuse(fmt.Sprintf("PAYLOAD surface digest mismatch between witness and delivery consumer (theirs %s, ours %s)", recordedPayload[:8], payloadDigest[:8]))
		}
	}
	return recordedDigest, true
}

// engineSkipAuthorized asks the prospective behavior-surface policy — the
// consuming source's, not a stale binary's — whether the witness may stand
// in for the ENGINE gate.
func (g *gateRun) engineSkipAuthorized() bool {
	policy, err := g.d.owners.surface()
	return err == nil && policy.SkipAllowed(behaviorsurface.WitnessScope, "witness-engine-gate")
}

// reuseEngineWitness accepts an ENGINE witness when it matches: compilation
// and the self-gating proof stay real, so the binary is rebuilt from the
// accepted content and stamped with its digest, and the acceptance is
// rechecked after the build. done reports that the gate's verdict is final.
func (g *gateRun) reuseEngineWitness() (int, bool) {
	d := g.d
	if g.env.get("METASYSTEM_GATE_WITNESS_CONSUMER_SCOPE") != "ENGINE" {
		fmt.Fprintln(d.stderr, "go gate: witness not accepted; running the full gate")
		return 0, false
	}
	digest, ok := g.witnessAcceptable()
	if !ok {
		fmt.Fprintln(d.stderr, "go gate: witness not accepted; running the full gate")
		return 0, false
	}
	if !g.engineSkipAuthorized() {
		fmt.Fprintln(d.stderr, "go gate: the witness holds, but the surface policy does not let it stand in; running the full gate")
		return 0, false
	}
	stamped := g.env.with("METASYSTEM_BUILD_STAMP=witness-" + digest[:12])
	if runBuild(g.ctx, nil, g.root, d.withEnvironment(stamped)) != 0 {
		fmt.Fprintln(d.stderr, "go gate: build failed under an accepted witness")
		return 1, true
	}
	if post, ok := g.witnessAcceptable(); ok && post == digest {
		g.witnessReused = true
		fmt.Fprintf(d.stdout, "go gate: PASSED (outer witness %s, this boundary)\n", digest[:8])
		return 0, true
	}
	fmt.Fprintln(d.stderr, "go gate: witness changed during the skip-path build; running the full gate")
	return 0, false
}

// consumeFrozen freezes the live tree and runs this same gate from the
// export. GOFLAGS is pinned to ownedGoFlags (read-only module selection,
// trimmed paths); GOMODCACHE stays
// inherited and shared, because the frozen go.sum pins module content while
// -mod=readonly forbids a dependency-set rewrite. Modfile and overlay flags
// would replace frozen inputs and are refused before the export is used.
func (g *gateRun) consumeFrozen(options gateOptions) int {
	d := g.d
	if refusal := frozenToolchainRefusal(g.env.get("GOFLAGS")); refusal != "" {
		fmt.Fprintf(d.stderr, "go gate: frozen witness consumer refused: %s\n", refusal)
		return 1
	}
	// The consumer ends by installing a fresh binary into the live tree —
	// the same mid-run swap the rebuild fence exists to prevent.
	if status := g.fence(); status != 0 {
		return status
	}
	frozen, err := d.owners.freeze(g.root)
	if err != nil {
		fmt.Fprintln(d.stderr, "gate witness-freeze:", err)
		fmt.Fprintln(d.stderr, "go gate: the witness could not freeze its live tree, so it is not used")
		return 1
	}
	g.consumerSnapshot = frozen.SnapshotRoot
	if !hex64.MatchString(frozen.Digest) || !isDir(frozen.Root) || !isDir(frozen.SnapshotRoot) {
		fmt.Fprintln(d.stderr, "go gate: freezing the witness tree returned an invalid hash or export path")
		return 1
	}
	export, err := filepath.EvalSymlinks(frozen.Root)
	if err != nil {
		export = frozen.Root
	}
	inner := g.env.with("GOFLAGS="+ownedGoFlags, "METASYSTEM_GATE_FROZEN_TOOLCHAIN=1",
		"METASYSTEM_GATE_WITNESS_CONSUMER_EXPORT="+export)
	var status int
	reused := false
	if options.witnessCheckOnly {
		status = runGate(g.ctx, export, inner, d, gateOptions{witnessCheckOnly: true})
	} else {
		marker, err := os.CreateTemp(tempDir(g.env), "tmp.")
		if err == nil {
			_ = marker.Close()
			inner.set("METASYSTEM_GATE_WITNESS_REUSE_OUT", marker.Name())
		}
		status = runGate(g.ctx, export, inner, d, gateOptions{})
		if err == nil {
			data, _ := os.ReadFile(marker.Name())
			reused = len(data) > 0
			_ = os.Remove(marker.Name())
		}
	}
	if status == 0 && !options.witnessCheckOnly {
		built := filepath.Join(export, "bin", "metasystem")
		if !executableFile(built) {
			fmt.Fprintln(d.stderr, "go gate: frozen witness consumer passed without producing its engine binary")
			status = 1
		} else if err := installBinary(built, g.root, d.selfPid); err != nil {
			status = 1
		}
	}
	g.witnessReused = reused && status == 0
	snapshot := g.consumerSnapshot
	g.consumerSnapshot = ""
	if err := d.owners.cleanupFreeze(snapshot); err != nil {
		fmt.Fprintln(d.stderr, "gate witness-freeze:", err)
		fmt.Fprintln(d.stderr, "go gate: frozen witness consumer could not release its owned project snapshot")
		status = 1
	}
	return status
}

// installBinary copies a proven engine over root's bin/metasystem through a
// temporary sibling and a rename.
func installBinary(built, root string, pid int64) error {
	data, err := os.ReadFile(built)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o755); err != nil {
		return err
	}
	stage := filepath.Join(root, "bin", ".metasystem.witness."+strconv.FormatInt(pid, 10))
	if err := os.WriteFile(stage, data, 0o755); err != nil {
		_ = os.Remove(stage)
		return err
	}
	if err := os.Rename(stage, filepath.Join(root, "bin", "metasystem")); err != nil {
		_ = os.Remove(stage)
		return err
	}
	return nil
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// witnessIdentity is what the snapshot-gated invocation proves.
type witnessIdentity struct {
	version, engine, payload, toolchain, payloadManifest string
}

func (g *gateRun) payloadManifestPath() string {
	return strings.TrimSuffix(g.env.get("METASYSTEM_GATE_WITNESS_WRITE"), ".json") + ".payload-paths.nul"
}

// writePayloadManifest lists the PAYLOAD projection, NUL-terminated, 0600.
func (g *gateRun) writePayloadManifest(path string) error {
	policy, err := g.d.owners.surface()
	if err != nil {
		return err
	}
	paths, err := policy.ListPaths(g.root, behaviorsurface.Payload)
	if err != nil {
		return err
	}
	var out bytes.Buffer
	for _, path := range paths {
		out.WriteString(path)
		out.WriteByte(0)
	}
	if err := os.WriteFile(path, out.Bytes(), 0o600); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}

// prepareWitnessWrite computes the identities the witness will carry and
// pre-builds the stamped engine a fresh snapshot lacks: the binary-driven
// fixtures skip without one, so a snapshot would otherwise measure the
// skipped-fixture coverage and refuse its own ratchet.
func (g *gateRun) prepareWitnessWrite() (witnessIdentity, bool) {
	d := g.d
	var identity witnessIdentity
	identity.payloadManifest = g.payloadManifestPath()
	if err := g.writePayloadManifest(identity.payloadManifest); err != nil {
		fmt.Fprintln(d.stderr, err)
		return identity, false
	}
	version, engine, err := g.surfaceIdentity(behaviorsurface.Engine, "")
	if err != nil {
		fmt.Fprintln(d.stderr, err)
		return identity, false
	}
	payloadVersion, payload, err := g.surfaceIdentity(behaviorsurface.Payload, identity.payloadManifest)
	if err != nil {
		fmt.Fprintln(d.stderr, err)
		return identity, false
	}
	if payloadVersion != version {
		fmt.Fprintln(d.stderr, "go gate: behavior-surface projections reported different policy versions")
		return identity, false
	}
	toolchain, err := g.toolchainIdentity()
	if err != nil {
		return identity, false
	}
	identity.version, identity.engine, identity.payload, identity.toolchain = strconv.Itoa(version), engine, payload, toolchain
	g.env.set("METASYSTEM_BUILD_STAMP", "witness-"+engine[:12])
	if runBuild(g.ctx, nil, g.root, d.withEnvironment(g.env).quiet()) != 0 {
		fmt.Fprintln(d.stderr, "go gate: snapshot pre-build failed")
		return identity, false
	}
	return identity, true
}

// writeWitness publishes the witness after the full proof, refusing when the
// frozen export changed while it was measured.
func (g *gateRun) writeWitness(prepared witnessIdentity, baselineRel string) int {
	d := g.d
	if refusal := g.witnessRefusal(); refusal != "" {
		fmt.Fprintf(d.stderr, "go gate: witness not written (%s)\n", refusal)
		return 0
	}
	manifestDigest := g.env.get("METASYSTEM_GATE_WITNESS_MANIFEST_DIGEST")
	if !hex64.MatchString(manifestDigest) {
		fmt.Fprintln(d.stderr, "go gate: complete-project manifest identity is required for witness publication")
		return 1
	}
	if _, err := d.owners.verify(g.root, manifestDigest); err != nil {
		fmt.Fprintln(d.stderr, "gate witness-verify:", err)
		fmt.Fprintln(d.stderr, "go gate: the frozen tree changed while the full gate ran; the witness is void")
		return 1
	}
	identity := prepared
	if identity.engine == "" {
		version, engine, err := g.surfaceIdentity(behaviorsurface.Engine, "")
		if err != nil {
			fmt.Fprintln(d.stderr, err)
			return 1
		}
		identity.version, identity.engine = strconv.Itoa(version), engine
	}
	if identity.payloadManifest == "" {
		identity.payloadManifest = g.payloadManifestPath()
		if err := g.writePayloadManifest(identity.payloadManifest); err != nil {
			fmt.Fprintln(d.stderr, err)
			return 1
		}
	}
	if identity.payload == "" {
		payloadVersion, payload, err := g.surfaceIdentity(behaviorsurface.Payload, identity.payloadManifest)
		if err != nil {
			fmt.Fprintln(d.stderr, err)
			return 1
		}
		if strconv.Itoa(payloadVersion) != identity.version {
			fmt.Fprintln(d.stderr, "go gate: behavior-surface projections reported different policy versions")
			return 1
		}
		identity.payload = payload
	}
	if identity.toolchain == "" {
		toolchain, err := g.toolchainIdentity()
		if err != nil {
			return 1
		}
		identity.toolchain = toolchain
	}
	controllerPID := g.env.get("METASYSTEM_GATE_WITNESS_CONTROLLER_PID")
	controllerStarted := g.env.get("METASYSTEM_GATE_WITNESS_CONTROLLER_STARTED_AT")
	controllerTicks := g.env.get("METASYSTEM_GATE_WITNESS_CONTROLLER_START_TICKS")
	controllerBoot := g.env.get("METASYSTEM_GATE_WITNESS_CONTROLLER_BOOT_ID")
	if !positiveDigits.MatchString(controllerPID) || !positiveDigits.MatchString(controllerStarted) || !digits.MatchString(controllerTicks) {
		fmt.Fprintln(d.stderr, "go gate: witness controller identity is incomplete")
		return 1
	}
	if ticks, _ := strconv.ParseInt(controllerTicks, 10, 64); ticks > 0 && controllerBoot == "" {
		fmt.Fprintln(d.stderr, "go gate: witness controller pair identity is incomplete")
		return 1
	} else if ticks == 0 && controllerBoot != "" {
		fmt.Fprintln(d.stderr, "go gate: witness controller pair identity is malformed")
		return 1
	}
	var goVersion bytes.Buffer
	_ = d.goTool(g.ctx, g.root, g.env.list(), []string{"version"}, &goVersion, nil)
	witness := fmt.Sprintf(`{"policyVersion":%s,"engineDigest":"%s","manifestDigest":"%s","payloadDigest":"%s","payloadManifest":"%s","toolchainIdentity":"%s","runId":"%s","controller":{"pid":%s,"startedAtSec":%s,"startTicks":%s,"bootId":"%s"},"passedAt":"%s","goVersion":"%s","ratchetBaseline":"%s","summary":"full gate in frozen proof tree"}`+"\n",
		identity.version, identity.engine, manifestDigest, identity.payload, filepath.Base(identity.payloadManifest), identity.toolchain,
		g.env.get("METASYSTEM_GATE_WITNESS_RUN"), controllerPID, controllerStarted, controllerTicks, controllerBoot,
		d.now().UTC().Format("2006-01-02T15:04:05Z"), strings.ReplaceAll(strings.TrimRight(goVersion.String(), "\n"), `"`, ""), baselineRel)
	path := g.env.get("METASYSTEM_GATE_WITNESS_WRITE")
	if err := os.WriteFile(path, []byte(witness), 0o600); err != nil || os.Chmod(path, 0o600) != nil {
		fmt.Fprintln(d.stderr, "go gate: witness could not be written")
		return 1
	}
	fmt.Fprintf(d.stdout, "go gate: witness written (%s)\n", identity.engine[:8])
	return 0
}
