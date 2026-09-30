package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// gateRun is one invocation of the static or full gate against one root. A
// witness consumer runs a second gateRun against its frozen export, in this
// process, with its own environment copy.
// The environment variables a gate message names in the command that
// resolves it.
const (
	testWorkersVariable    = "METASYSTEM_TEST_WORKERS"
	concurrentGateVariable = "METASYSTEM_ALLOW_CONCURRENT_GATE"
)

type gateRun struct {
	ctx     context.Context
	root    string
	env     *environment
	d       deps
	workers string
	fast    bool
	// verbose prints the details a person asks for with --verbose, such
	// as the list of checks a passing static run made.
	verbose bool

	proofWorker bool

	installation bool
	scratch      string
	markers      []string

	// consumerLive is the live root a frozen consumer copies its proven
	// binary back to, and whose snapshot it releases.
	consumerSnapshot string

	// coverage evidence retention (the full gate's native stage onward).
	coverageLog      string
	packageList      string
	nativeRoot       string
	coverageKeep     string
	retentionPending bool

	// witnessReused is set when an accepted ENGINE witness replaced the
	// full proof (the reuse marker the sourced wrapper once read).
	witnessReused bool
}

type gateOptions struct {
	witnessCheckOnly bool
	goal             string
	capMin           string
}

var positiveMinutes = regexp.MustCompile(`^[1-9][0-9]*$`)

// ownedGoFlags is the one GOFLAGS value the gate owns (disk-lifetimes rule
// A2): the full gate pins it, a frozen tree accepts exactly it, and the
// arming snapshot, the witness consumer and the candidate verifier set it.
// The validator's canonical environment (cmd/metasystem/landing_verbs.go)
// and scripts/validate-metasystem.sh carry the same literal, because a
// validator landing runs this gate against a frozen tree.
const ownedGoFlags = "-mod=readonly -trimpath"

// runGateAction parses the gate action's arguments: the full gate, its
// witness probe, or (with --arm) the witness-producing controller that the
// sourced witness-gate.sh once was.
func runGateAction(ctx context.Context, args []string, root string, d deps) int {
	var options gateOptions
	var arm armOptions
	for len(args) > 0 {
		switch args[0] {
		case "--witness-check-only":
			options.witnessCheckOnly = true
			args = args[1:]
		case "--goal":
			if len(args) < 2 || args[1] == "" {
				fmt.Fprintln(d.stderr, "go gate: --goal needs an accepted goal id")
				return 2
			}
			options.goal, args = args[1], args[2:]
		case "--cap-min":
			if len(args) < 2 || !positiveMinutes.MatchString(args[1]) {
				fmt.Fprintln(d.stderr, "go gate: --cap-min needs positive minutes")
				return 2
			}
			options.capMin, args = args[1], args[2:]
		case "--arm":
			if len(args) < 2 {
				fmt.Fprintln(d.stderr, "witness-gate refused: --arm must be plain or none (got 'unset')")
				return 1
			}
			arm.fallback, args = args[1], args[2:]
			arm.requested = true
		case "--controller-pid":
			if len(args) < 2 {
				fmt.Fprintln(d.stderr, "go gate: --controller-pid needs a pid")
				return 2
			}
			pid, err := strconv.ParseInt(args[1], 10, 64)
			if err != nil || pid < 1 {
				fmt.Fprintln(d.stderr, "go gate: --controller-pid needs a pid")
				return 2
			}
			arm.controller, args = pid, args[2:]
		case "--state-out":
			if len(args) < 2 || args[1] == "" {
				fmt.Fprintln(d.stderr, "go gate: --state-out needs a path")
				return 2
			}
			arm.stateOut, args = args[1], args[2:]
		case "--delivery":
			arm.delivery, args = true, args[1:]
		default:
			fmt.Fprintf(d.stderr, "go gate: unknown argument %s\n", args[0])
			return 2
		}
	}
	if arm.requested {
		if options.witnessCheckOnly || options.goal != "" || options.capMin != "" {
			fmt.Fprintln(d.stderr, "go gate: --arm combines with no other gate mode")
			return 2
		}
		return runArm(ctx, root, d, arm)
	}
	if arm.controller != 0 || arm.stateOut != "" || arm.delivery {
		fmt.Fprintln(d.stderr, "go gate: --controller-pid, --state-out and --delivery apply only to --arm")
		return 2
	}
	if options.witnessCheckOnly && (options.goal != "" || options.capMin != "") {
		fmt.Fprintln(d.stderr, "go gate: --witness-check-only is a probe and combines with no other mode")
		return 2
	}
	return runGate(ctx, root, newEnvironment(d.environ()), d, options)
}

// runGate is one full gate (or witness probe) against root with env. The
// arming controller and the witness consumer call it again, in process, for
// a frozen export.
func runGate(ctx context.Context, root string, env *environment, d deps, options gateOptions) int {
	g := &gateRun{ctx: ctx, root: root, env: env, d: d}
	workers, ok := gateWorkers(env, d.stderr)
	if !ok {
		return 1
	}
	g.workers = workers
	env.set("GOMAXPROCS", workers)
	witnessReuseOut := env.get("METASYSTEM_GATE_WITNESS_REUSE_OUT")
	env.unset("METASYSTEM_GATE_WITNESS_REUSE_OUT")
	status := g.finish(g.full(options))
	if status == 0 && g.witnessReused && witnessReuseOut != "" {
		if err := os.WriteFile(witnessReuseOut, []byte("reused\n"), 0o600); err != nil {
			return 1
		}
	}
	return status
}

// finish is the gate's exit trap: the run marker goes, coverage evidence a
// post-native exit left behind is retained (failing the run if it cannot
// be), a consumer's frozen snapshot is released, and the scratch build goes.
func (g *gateRun) finish(status int) int {
	for _, marker := range g.markers {
		_ = os.Remove(marker)
	}
	g.markers = nil
	retentionFailed := false
	if g.retentionPending {
		if !g.retainCoverage("post-native exit before coverage consumers completed") {
			retentionFailed = true
		}
	}
	if g.consumerSnapshot != "" {
		_ = g.d.owners.cleanupFreeze(g.consumerSnapshot)
		g.consumerSnapshot = ""
	}
	g.dropScratch()
	if retentionFailed {
		return 1
	}
	return status
}

// full is the full gate: the retained-proof relaunch, the witness consumer
// and probe, the static stages, the cross-builds and govulncheck, the native
// race and coverage selection, the coverage ratchet, and the witness write.
func (g *gateRun) full(options gateOptions) int {
	d := g.d
	// The full proof admits the environment it actually executes. Pin
	// readonly module resolution before the outer proof reservation so
	// coverage producers and retained consumers identify one real Go
	// environment.
	if !options.witnessCheckOnly {
		g.env.set("GOFLAGS", ownedGoFlags)
	}
	g.authenticateWorker()
	if g.env.get("METASYSTEM_GO_GATE_RELAUNCHED") == "1" {
		if !g.proofWorker {
			fmt.Fprintln(d.stderr, "go gate: the relaunched gate is not an authorized test worker")
			return 1
		}
		g.env.unset("METASYSTEM_GO_GATE_RELAUNCHED")
	}
	// A standalone full gate enters the same retained proof owner as
	// validation and adoption. Only the Go owner may recognize a worker;
	// ambient progress or locator strings do not bypass the wrapper.
	if !options.witnessCheckOnly && !g.proofWorker {
		return g.relaunch(options)
	}
	if d.lookGo() != nil {
		fmt.Fprintln(d.stderr, "go gate: no go toolchain is on this shell's search path, so the committed engine cannot be built")
		return 1
	}
	if status := g.registerOnly("devgate gate"); status != 0 {
		return status
	}

	// A witness consumer first freezes its own tree, then runs this same
	// gate from the export, so every acceptance read, skip-path build,
	// post-build recheck and full fallback sees frozen bytes.
	if g.env.get("METASYSTEM_GATE_WITNESS") != "" && g.env.get("METASYSTEM_GATE_WITNESS_CONSUMER_EXPORT") != g.root {
		return g.consumeFrozen(options)
	}
	if g.env.get("METASYSTEM_GATE_FROZEN_TOOLCHAIN") == "1" && g.env.get("GOFLAGS") != ownedGoFlags {
		fmt.Fprintln(d.stderr, "go gate: the frozen test tree needs GOFLAGS="+ownedGoFlags)
		return 1
	}
	if options.witnessCheckOnly {
		digest, ok := g.witnessAcceptable()
		if !ok {
			return 3
		}
		if g.env.get("METASYSTEM_GATE_WITNESS_CONSUMER_SCOPE") == "ENGINE" && !g.engineSkipAuthorized() {
			fmt.Fprintln(d.stderr, "behavior-surface policy does not authorize the witness ENGINE gate")
			return 3
		}
		fmt.Fprintf(d.stdout, "witness acceptable: %s\n", digest[:8])
		return 0
	}
	if g.env.get("METASYSTEM_GATE_WITNESS") != "" {
		if status, done := g.reuseEngineWitness(); done {
			return status
		}
	}

	// The worker was authenticated before any gate stage could launch
	// descendants (a run that is not one relaunched above). The producer
	// decision and claim wait until the exact point this process starts the
	// full measurement, so a fence or static refusal cannot consume the slot.
	baselineRel := testpolicy.CoverageFloorsFile(runtime.GOOS)
	coverageCandidate := false
	if g.env.get("METASYSTEM_PROOF_CONTROL_ROOT") != "" || g.env.get("METASYSTEM_PROOF_ATTEMPT") != "" {
		if g.env.get("METASYSTEM_PROOF_CONTROL_ROOT") == "" {
			fmt.Fprintln(d.stderr, "go gate: the test worker's settings name no control root")
			return 1
		}
		// A legacy launch has a live control-root/process binding but no
		// admitted attempt, so it runs every stage without retained
		// coverage. An admitted unseeded full gate defers the source-identity
		// decision until measurement.
		if g.env.get("METASYSTEM_PROOF_ATTEMPT") != "" && g.env.get("METASYSTEM_COVERAGE_RATCHET_SEED") != "1" &&
			g.env.get("METASYSTEM_GATE_FORCE") != "1" {
			coverageCandidate = true
		}
	}
	if status := g.fence(); status != 0 {
		return status
	}
	if status := g.collectStatic(); status != 0 {
		return status
	}
	// The static proof artifact is not the runtime binary here (the final
	// build below is stamped), but it is the candidate engine the native
	// stage's resource custodians run as: they are engine entries, which this
	// program is not. It is removed once the native stage has passed.

	// The standing Linux signal (go-production-grade Phase 1, P3): both
	// Linux architectures cross-compile in every gate run.
	for _, arch := range []string{"amd64", "arm64"} {
		env := g.env.with("CGO_ENABLED=0", "GOOS=linux", "GOARCH="+arch).list()
		if d.goTool(g.ctx, g.root, env, []string{"build", "-trimpath", "-p=" + g.workers, "./..."}, d.stdout, d.stderr) != nil {
			fmt.Fprintf(d.stderr, "go gate: linux/%s cross-build failed\n", arch)
			return 1
		}
	}
	// govulncheck is last of the static stages: its cost belongs to the
	// vulnerability-database fetch, which the network owns, so every
	// deterministic check gets to fail first.
	if d.goTool(g.ctx, g.root, g.env.list(), []string{"run", "-trimpath", "-p=" + g.workers, govulncheckModule, "./..."}, d.stdout, d.stderr) != nil {
		fmt.Fprintln(d.stderr, "go gate: govulncheck v1.2.0 refused (or could not run)")
		return 1
	}

	// The snapshot-gated invocation (witness write) prepares here, after the
	// static verdicts and just ahead of the tests it serves: the digest
	// describes bytes nothing above mutates, and the stamp exports before
	// every build that consumes it.
	var written witnessIdentity
	if g.env.get("METASYSTEM_GATE_WITNESS_WRITE") != "" && !g.witnessFencedOff() && g.witnessRefusal() == "" {
		identity, ok := g.prepareWitnessWrite()
		if !ok {
			return 1
		}
		written = identity
	}

	// The coverage floor is executable, not prose: the native selection's
	// output feeds the ratchet, whose baseline only ever rises.
	baseline := filepath.Join(g.root, baselineRel)
	logFile, err := os.CreateTemp(tempDir(g.env), "tmp.")
	if err != nil {
		fmt.Fprintln(d.stderr, "go gate: cannot create the coverage log")
		return 1
	}
	g.coverageLog = logFile.Name()
	_ = logFile.Close()
	coverageHandoff := false
	coverage := proofrun.CoverageBeginOptions{ControlRoot: g.env.get("METASYSTEM_PROOF_CONTROL_ROOT"), ExecutionRoot: g.root,
		AttemptID: g.env.get("METASYSTEM_PROOF_ATTEMPT"), BaselinePath: baseline, ProducerClass: "full",
		ProducerPID: d.selfPid, CallerPID: d.selfPid}
	if coverageCandidate {
		eligible, err := d.owners.coverageEligible(coverage)
		switch {
		case err != nil:
			_ = os.Remove(g.coverageLog)
			fmt.Fprintln(d.stderr, "go gate: coverage producer eligibility could not be authenticated")
			return 1
		case eligible:
			if err := d.owners.coverageBegin(coverage); err != nil {
				_ = os.Remove(g.coverageLog)
				fmt.Fprintln(d.stderr, "go gate: coverage measurement could not start:", err)
				fmt.Fprintln(d.stderr, "go gate: could not claim the authenticated coverage producer slot")
				return 1
			}
			coverageHandoff = true
		}
	}

	if status := g.nativeTests(); status != 0 {
		return status
	}

	// Build the binary the shell fixtures and wrappers exec, through the one
	// shared fenced build: stamped with its source commit so its artifacts
	// self-attest, CGO pinned off. This run holds the fence; the build's own
	// consult is exempt inside this process.
	commit := g.env.get("METASYSTEM_BUILD_STAMP")
	if commit == "" {
		commit = "unknown"
		if head, err := d.git(g.ctx, g.root, "rev-parse", "--short", "HEAD"); err == nil {
			commit = strings.TrimSpace(string(head))
		}
	}
	if runBuild(g.ctx, nil, g.root, d.withEnvironment(g.env)) != 0 {
		if !g.retainCoverage("build failed") {
			fmt.Fprintln(d.stderr, "go gate: build failed and evidence retention failed")
		}
		return 1
	}

	// The coverage ratchet is joined against an independent package
	// inventory so a testless package cannot hide. Floors are per platform;
	// a seeding run enforces every other check and skips the floor join.
	listing, err := os.CreateTemp(tempDir(g.env), "tmp.")
	if err == nil {
		g.packageList = listing.Name()
		err = d.goTool(g.ctx, g.root, g.env.list(), []string{"list", "-p=" + g.workers, "./internal/..."}, listing, d.stderr)
		_ = listing.Close()
	}
	if err != nil {
		if !g.retainCoverage("go list failed; cannot join the coverage inventory") {
			fmt.Fprintln(d.stderr, "go gate: go list failed and evidence retention failed")
		}
		return 1
	}
	seed := g.env.get("METASYSTEM_COVERAGE_RATCHET_SEED") == "1"
	if !seed {
		if _, err := os.Stat(baseline); err != nil {
			if !g.retainCoverage("no coverage baseline for this platform (" + baselineRel + "); run the two-pass seed bootstrap first") {
				fmt.Fprintln(d.stderr, "go gate: missing baseline and evidence retention failed")
			}
			return 1
		}
		measured, _ := os.ReadFile(g.coverageLog)
		inventory, _ := os.ReadFile(g.packageList)
		refusals, err := coverageRatchet(baseline, measured, inventory)
		if err != nil {
			refusals = []string{err.Error()}
		}
		for _, line := range refusals {
			fmt.Fprintln(d.stderr, line)
		}
		if len(refusals) > 0 {
			if !g.retainCoverage("coverage ratchet refused") {
				fmt.Fprintln(d.stderr, "go gate: coverage ratchet refused and evidence retention failed")
			}
			return 1
		}
	}
	if coverageHandoff {
		evidence, err := d.owners.coverageComplete(proofrun.CoverageCompleteOptions{CoverageBeginOptions: coverage,
			CoverageLog: g.coverageLog, PackageInventory: g.packageList, ModulePrefix: "github.com/widoriezebos/agentic-tools/metasystem/"})
		if err != nil {
			fmt.Fprintln(d.stderr, "go gate: coverage measurement could not finish:", err)
			if !g.retainCoverage("authenticated coverage publication refused") {
				fmt.Fprintln(d.stderr, "go gate: authenticated coverage publication and evidence retention both failed")
			}
			return 1
		}
		fmt.Fprint(d.stdout, encodeEvidence(evidence))
	}
	if seed {
		if !g.retainCoverage("coverage seed inputs retained; floors were not enforced") {
			fmt.Fprintln(d.stderr, "go gate: coverage seed evidence retention failed; floors were not enforced")
			return 1
		}
	} else {
		g.retentionPending = false
		_ = os.RemoveAll(g.nativeRoot)
		_ = os.Remove(g.coverageLog)
		_ = os.Remove(g.packageList)
	}

	// The witness write (D33): only the controller's snapshot-gated
	// invocation sets METASYSTEM_GATE_WITNESS_WRITE.
	if g.env.get("METASYSTEM_GATE_WITNESS_WRITE") != "" && !g.witnessFencedOff() {
		if status := g.writeWitness(written, baselineRel); status != 0 {
			return status
		}
	}
	fmt.Fprintf(d.stdout, "go gate: PASSED (gofmt, shell parse, vet, race tests, coverage ratchet, build @ %s)\n", commit)
	return 0
}

// registerOnly records the run without consulting the fence: the full gate
// fences later, after its witness paths, exactly where the rebuild begins.
func (g *gateRun) registerOnly(name string) int {
	if !executableFile(filepath.Join(g.root, "bin", "metasystem")) {
		return 0
	}
	marker, err := g.d.owners.register(g.root, g.d.selfPid, name)
	if err != nil {
		fmt.Fprintln(g.d.stderr, err)
		fmt.Fprintln(g.d.stderr, "go gate: registration failed; refusing to run invisibly")
		return 1
	}
	if marker != "" {
		g.markers = append(g.markers, marker)
	}
	return 0
}

// authenticateWorker asks the trusted engine whether this process is inside
// an authenticated proof worker. An enrolled engine may be older than the
// candidate source executing this admitted command group; then the
// candidate's own verifier checks the inherited attempt and live custody.
// Authentication stays a child process: the trusted engine, not the
// candidate bytes, owns the answer.
func (g *gateRun) authenticateWorker() {
	var verifiers [][]string
	auth := g.env.get("METASYSTEM_PROOF_AUTH_BIN")
	if auth == "" {
		auth = filepath.Join(g.root, "bin", "metasystem")
	}
	if executableFile(auth) {
		verifiers = append(verifiers, []string{auth})
	}
	if g.env.get("METASYSTEM_PROOF_ATTEMPT") != "" && g.env.get("METASYSTEM_PROOF_CONTROL_ROOT") != "" && g.d.lookGo() == nil {
		verifiers = append(verifiers, []string{"env", "GOFLAGS=" + ownedGoFlags, "GOWORK=off", "CGO_ENABLED=0", "go", "run", "-trimpath", "-p=" + g.workers, "./cmd/metasystem"})
	}
	for _, verifier := range verifiers {
		// `proof-run worker-authorized` is an entry and keeps its argv
		// exactly, so an older trusted engine still answers it.
		args := append(append([]string(nil), verifier[1:]...), "proof-run", "worker-authorized", "--root", g.root)
		if g.d.tool(g.ctx, toolCall{dir: g.root, env: g.env.list(), name: verifier[0], args: args, stdout: io.Discard, stderr: io.Discard}) == nil {
			g.proofWorker = true
			return
		}
	}
}

// relaunch builds a private engine, asks it for the cost banner, and
// replaces this process with that engine's retained proof launch running
// the gate again as its authorized worker.
func (g *gateRun) relaunch(options gateOptions) int {
	d := g.d
	progress := filepath.Join(g.root, "artifacts", "agents", "supervision", "suite-progress.jsonl")
	logPath := filepath.Join(g.root, "artifacts", "agents", "supervision", "suite-logs",
		"go-gate-"+d.now().UTC().Format("20060102T150405Z")+"-"+strconv.FormatInt(d.selfPid, 10)+".log")
	tmp, err := os.MkdirTemp(tempDir(g.env), "metasystem-go-gate.")
	if err != nil {
		fmt.Fprintln(d.stderr, "go gate: could not prepare the kept test launch")
		return 1
	}
	engine := filepath.Join(tmp, "metasystem")
	if d.goTool(g.ctx, g.root, g.env.list(), []string{"build", "-trimpath", "-p=" + g.workers, "-o", engine, "./cmd/metasystem"}, d.stdout, d.stderr) != nil {
		return 1
	}
	var banner bytes.Buffer
	if d.tool(g.ctx, toolCall{dir: g.root, env: g.env.list(), name: engine, args: []string{"internal", "proof-run", "banner",
		"--suite", "go-gate", "--root", g.root, "--progress", progress, "--log", logPath}, stdout: &banner, stderr: d.stderr}) != nil {
		fmt.Fprintln(d.stderr, "go gate: could not prepare the kept test launch")
		return 1
	}
	argv := []string{engine, "internal", "proof-run", "launch", "--suite", "go-gate", "--root", g.root,
		"--conf", filepath.Join(g.root, "metasystem.conf"), "--progress", progress, "--log", logPath,
		"--banner", strings.TrimRight(banner.String(), "\n"), "--scope", "full", "--command-class", "go-gate"}
	if options.goal != "" {
		argv = append(argv, "--goal", options.goal)
	}
	if options.capMin != "" {
		argv = append(argv, "--cap-min", options.capMin)
	}
	argv = append(argv, "--tmp", tmp, "--", "env", "METASYSTEM_GO_GATE_RELAUNCHED=1", "METASYSTEM_PROOF_AUTH_BIN="+engine,
		"METASYSTEM_TEST_WORKERS="+g.workers, "go", "-C", g.root, "run", "-trimpath", "./cmd/devgate", "gate")
	// The launcher runs as this process's child rather than replacing it,
	// so a gate nested in the arming controller or a frozen consumer
	// returns to its caller; the launcher, not this process, is the proof's
	// recorded launcher either way.
	return exitStatus(d.tool(g.ctx, toolCall{dir: g.root, env: g.env.list(), name: engine, args: argv[1:], stdout: d.stdout, stderr: d.stderr}))
}

// nativeTests runs the full gate's native Go selection — the complete
// internal and command package inventory, native process partitions, the
// coverage merge, terminal checks, supervision and bounded diagnostic
// reruns — with the allowance inherited from the admitted parent.
func (g *gateRun) nativeTests() int {
	d := g.d
	root, err := os.MkdirTemp(tempDir(g.env), "metasystem-gate-native.")
	if err != nil {
		fmt.Fprintln(d.stderr, "go gate: cannot create the native log root")
		return 1
	}
	g.nativeRoot = root
	stderrPath := filepath.Join(root, "go-gate-tests.stderr.log")
	var stderr bytes.Buffer
	status := g.runNative(&stderr)
	_ = os.WriteFile(stderrPath, stderr.Bytes(), 0o600)
	if status != 0 {
		keep := filepath.Join(g.root, "artifacts", "agents", "gate-failures",
			d.now().UTC().Format("20060102T150405Z")+"-"+strconv.FormatInt(d.selfPid, 10)+".log")
		_ = os.MkdirAll(filepath.Dir(keep), 0o755)
		// The witness owner removes its failed snapshot, so the complete
		// diagnostic reaches the retained parent log before any cleanup.
		_, _ = d.stderr.Write(stderr.Bytes())
		if coverage, err := os.ReadFile(g.coverageLog); err == nil {
			_, _ = d.stderr.Write(coverage)
		}
		if moveEvidence(g.coverageLog, keep) == nil {
			if file, err := os.OpenFile(keep, os.O_APPEND|os.O_WRONLY, 0); err == nil {
				_, _ = file.Write(stderr.Bytes())
				_ = file.Close()
			}
		}
		_ = moveEvidence(root, strings.TrimSuffix(keep, ".log")+".native")
		rel, _ := filepath.Rel(g.root, keep)
		fmt.Fprintf(d.stderr, "go gate: native Go tests failed (output kept: %s)\n", filepath.ToSlash(rel))
		return 1
	}
	g.retentionPending = true
	g.dropScratch()
	_, _ = d.stderr.Write(stderr.Bytes())
	// An unreadable native log is an exit before the coverage consumers
	// completed: the gate's exit retains the evidence (finish).
	native, err := os.ReadFile(filepath.Join(root, "go-gate-native.log"))
	if err != nil {
		fmt.Fprintf(d.stderr, "go gate: native log unreadable: %v\n", err)
		return 1
	}
	_, _ = d.stdout.Write(native)
	return 0
}

// runNative is the native selection itself, writing the merged coverage to
// the coverage log and its diagnostics to stderr.
func (g *gateRun) runNative(stderr io.Writer) int {
	d := g.d
	workers, _ := strconv.Atoi(g.workers)
	ctx, stop := signal.NotifyContext(g.ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	engine, err := filepath.Abs(g.scratch)
	if g.scratch == "" || err != nil {
		fmt.Fprintln(stderr, "go gate tests: no candidate engine to run the fixture custodians")
		return 1
	}
	ctx = proofrun.WithResourceCustodyExecutable(ctx, engine)
	if g.env.get("METASYSTEM_PROOF_ATTEMPT") != "" {
		controlRoot, err := filepath.EvalSymlinks(g.env.get("METASYSTEM_PROOF_CONTROL_ROOT"))
		if err == nil {
			controlRoot, err = filepath.Abs(controlRoot)
		}
		if err != nil {
			fmt.Fprintln(stderr, "go gate tests:", err)
			return 3
		}
		release, leased, err := d.owners.hostResources(ctx, controlRoot)
		if err != nil {
			fmt.Fprintln(stderr, "go gate tests: the host resources passed down cannot be taken:", err)
			return 3
		}
		defer release()
		ctx = leased
	}
	result, status, err := d.owners.goGateTests(ctx, proofrun.GoGateTestRequest{Root: g.root, LogRoot: g.nativeRoot,
		Environment: g.env.with("METASYSTEM_TEST_WORKERS=" + g.workers).list(), Workers: workers})
	_ = os.WriteFile(g.coverageLog, result.Output, 0o600)
	for _, rerun := range result.Reruns {
		fmt.Fprintf(stderr, "go gate diagnostic rerun: %s.%s first=%s second=%s log=%s\n",
			rerun.Package, rerun.Test, rerun.First, rerun.Second, rerun.LogPath)
	}
	if err != nil {
		fmt.Fprintf(stderr, "go gate tests: %v (log: %s)\n", err, result.LogPath)
	}
	return status
}

// retainCoverage keeps the coverage input, the package inventory and the
// native evidence under the owning root's gate-failures, reporting reason.
func (g *gateRun) retainCoverage(reason string) bool {
	d := g.d
	owner := filepath.Join(g.root, "artifacts", "agents")
	if g.proofWorker && g.env.get("METASYSTEM_PROOF_EXECUTION_ROOT") != "" {
		owner = g.env.get("METASYSTEM_PROOF_EXECUTION_ROOT")
	} else if g.proofWorker && g.env.get("METASYSTEM_PROOF_CONTROL_ROOT") != "" {
		owner = g.env.get("METASYSTEM_PROOF_CONTROL_ROOT")
	}
	if g.coverageKeep == "" {
		g.coverageKeep = filepath.Join(owner, "gate-failures",
			d.now().UTC().Format("20060102T150405Z")+"-"+strconv.FormatInt(d.selfPid, 10)+"-coverage")
	}
	if err := os.MkdirAll(g.coverageKeep, 0o755); err != nil {
		fmt.Fprintf(d.stderr, "go gate: coverage evidence retention failed: cannot create %s\n", g.coverageKeep)
		return false
	}
	for _, move := range []struct{ from, to, what string }{
		{g.coverageLog, "coverage.jsonl", "coverage input"},
		{g.packageList, "package-inventory.txt", "package inventory"},
		{g.nativeRoot, "native", "native evidence"},
	} {
		if move.from == "" || !present(move.from) {
			continue
		}
		if err := moveEvidence(move.from, filepath.Join(g.coverageKeep, move.to)); err != nil {
			fmt.Fprintf(d.stderr, "go gate: coverage evidence retention failed: cannot retain %s\n", move.what)
			return false
		}
	}
	if info, err := os.Stat(filepath.Join(g.coverageKeep, "coverage.jsonl")); err != nil || !info.Mode().IsRegular() {
		fmt.Fprintln(d.stderr, "go gate: coverage evidence retention failed: retained bundle is incomplete")
		return false
	}
	if info, err := os.Stat(filepath.Join(g.coverageKeep, "native")); err != nil || !info.IsDir() {
		fmt.Fprintln(d.stderr, "go gate: coverage evidence retention failed: retained bundle is incomplete")
		return false
	}
	g.retentionPending = false
	fmt.Fprintf(d.stderr, "go gate: %s (evidence kept: %s)\n", reason, g.coverageKeep)
	return true
}

// moveEvidence moves a file or tree to a retention path. Temporary evidence
// and its owner's gate-failures may sit on different filesystems, where a
// rename cannot cross (EXDEV): the tree is then copied and the source removed.
func moveEvidence(from, to string) error {
	err := os.Rename(from, to)
	if err == nil || !errors.Is(err, syscall.EXDEV) {
		return err
	}
	if err := copyEvidence(from, to); err != nil {
		_ = os.RemoveAll(to)
		return err
	}
	return os.RemoveAll(from)
}

func copyEvidence(from, to string) error {
	return filepath.WalkDir(from, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		target := filepath.Join(to, rel)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		switch {
		case entry.IsDir():
			return os.MkdirAll(target, info.Mode().Perm()|0o700)
		case info.Mode().IsRegular():
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			return os.WriteFile(target, data, info.Mode().Perm())
		default:
			return fmt.Errorf("evidence %s is not a regular file or directory", path)
		}
	})
}

// verboseLine prints a detail only when --verbose asked for it.
func (g *gateRun) verboseLine(line string) {
	if g.verbose {
		fmt.Fprintln(g.d.stdout, "  "+line)
	}
}

// action is the devgate action this run answers as.
func (g *gateRun) action() string {
	if g.fast {
		return "static"
	}
	return "gate"
}
