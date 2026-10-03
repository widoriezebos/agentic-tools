package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
)

// staticcheckModule is pinned to release 2026.2 (module v0.8.0;
// go-production-grade Phase 0d): the frozen version keeps every checkout
// judging by the same rules, and a tool run that cannot start fails the gate
// loudly rather than skipping silently. It rides the compile cache vet just
// filled, so its verdict lands seconds after vet's.
const staticcheckModule = "honnef.co/go/tools/cmd/staticcheck@v0.8.0"

// govulncheckModule is pinned like staticcheck (Phase 0d). Version pins must
// use parser and SSA tooling compatible with the repository compiler.
const govulncheckModule = "golang.org/x/vuln/cmd/govulncheck@v1.2.0"

// runStatic is the fast static/build leaf: the dependency and parallel
// ratchets and the static stages, plus the engine build — seconds end to end,
// for tight edit loops and the fast-static-build group. It is not a landing
// gate: no race tests, no cross-builds, no govulncheck, no coverage ratchet,
// and it refuses the witness protocol outright. The full gate remains the
// landing requirement. It is selected by the action word only, never by an
// environment variable: an exported variable outlives the edit loop it
// served and would silently weaken the suites that make the full gate a
// landing requirement.
func runStatic(ctx context.Context, args []string, root string, d deps) int {
	proofOut, verbose := "", false
	for len(args) > 0 {
		switch args[0] {
		case "--verbose":
			verbose = true
			args = args[1:]
		case "--proof-out":
			if len(args) < 2 || args[1] == "" {
				fmt.Fprintln(d.stderr, "go gate: --proof-out needs a path")
				return 2
			}
			proofOut = args[1]
			args = args[2:]
		default:
			fmt.Fprintf(d.stderr, "go gate: unknown argument %s\n", args[0])
			return 2
		}
	}
	env := newEnvironment(d.environ())
	workers, ok := gateWorkers(env, d.stderr)
	if !ok {
		return 1
	}
	env.set("GOMAXPROCS", workers)
	g := &gateRun{ctx: ctx, root: root, env: env, d: d, workers: workers, fast: true, verbose: verbose}
	return g.finish(g.static(proofOut))
}

// gateWorkers is the inherited test-worker allowance, which also bounds the
// Go toolchain's parallelism in every stage.
func gateWorkers(env *environment, stderr io.Writer) (string, bool) {
	workers := env.get("METASYSTEM_TEST_WORKERS")
	if workers == "" {
		workers = "1"
	}
	if !positiveInteger.MatchString(workers) {
		fmt.Fprintf(stderr, "go gate: the test worker count %q is not a positive integer\nrun: unset %s\n", workers, testWorkersVariable)
		return "", false
	}
	return workers, true
}

// static runs the fast leaf, returning the exit status.
func (g *gateRun) static(proofOut string) int {
	d := g.d
	// Fast mode is an edit-loop tool, not a landing gate: it must neither
	// consume nor produce a witness, so a witness handoff arriving alongside
	// it is a contradiction to refuse loudly.
	if g.env.get("METASYSTEM_GATE_WITNESS") != "" || g.env.get("METASYSTEM_GATE_WITNESS_WRITE") != "" {
		fmt.Fprintln(d.stderr, "go gate: fast mode cannot join the witness protocol; run the full gate")
		return 1
	}
	if d.lookGo() != nil {
		fmt.Fprintln(d.stderr, "go gate: no go toolchain is on this shell's search path, so the committed engine cannot be built")
		return 1
	}
	// A forbidden script dependency is cheaper and more urgent than every
	// test in the edit-loop gate, so the ratchets run before any test process
	// and their refusal is the gate's refusal.
	if out, ok := d.owners.dependencyRatchet(g.root); !ok {
		fmt.Fprint(d.stderr, out)
		return 1
	}
	if out, ok := d.owners.parallelRatchet(g.root); !ok {
		fmt.Fprint(d.stderr, out)
		return 1
	}
	if status := g.registerAndFence("devgate static"); status != 0 {
		return status
	}
	if status := g.collectStatic(); status != 0 {
		return status
	}
	// Publish the exact engine the collected static stage built. The
	// temporary sibling plus comparison keeps publication atomic and proves
	// that no second compile or byte substitution occurred.
	target := proofOut
	if target == "" {
		target = filepath.Join(g.root, "bin", "metasystem")
	} else if !filepath.IsAbs(target) {
		target = filepath.Join(g.root, target)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		fmt.Fprintln(d.stderr, "go gate: publish static build failed")
		return 1
	}
	publish := target + ".gate." + strconv.FormatInt(d.selfPid, 10)
	built, err := os.ReadFile(g.scratch)
	if err == nil {
		err = os.WriteFile(publish, built, 0o755)
	}
	if err != nil {
		_ = os.Remove(publish)
		fmt.Fprintln(d.stderr, "go gate: publish static build failed")
		return 1
	}
	if published, err := os.ReadFile(publish); err != nil || !bytes.Equal(published, built) {
		_ = os.Remove(publish)
		fmt.Fprintln(d.stderr, "go gate: published static build changed bytes")
		return 1
	}
	if err := os.Chmod(publish, 0o755); err != nil {
		_ = os.Remove(publish)
		fmt.Fprintln(d.stderr, "go gate: publish static build failed")
		return 1
	}
	if err := os.Rename(publish, target); err != nil {
		_ = os.Remove(publish)
		fmt.Fprintln(d.stderr, "go gate: publish static build failed")
		return 1
	}
	g.dropScratch()
	if g.installation {
		fmt.Fprintln(d.stdout, "go gate: fast checks passed; landing still needs the full gate\nrun: go run ./cmd/devgate gate")
		g.verboseLine("checked: dependency ratchet, parallel ratchet, gofmt, shell parse, vet, staticcheck,")
		g.verboseLine("  dead code, refusal register, SessionStart exit audit, Stop decision surface audit, build")
	} else {
		fmt.Fprintln(d.stdout, "go gate: fast checks passed; landing still needs the full gate\nrun: go run ./cmd/devgate gate")
		g.verboseLine("checked: dependency ratchet, parallel ratchet, gofmt, shell parse, vet, staticcheck,")
		g.verboseLine("  dead code, refusal register, build")
	}
	return 0
}

// registerAndFence records this gate run (goal-system GOAL-17: an unrecorded
// supported gate was invisible to the turn-end scan) and refuses when a
// foreign gate run is live in the checkout. Markers are per pid and die with
// their process; the fence exempts this process's own chain, so only a
// rebuild against someone else's live run is refused. Both are skipped where
// no engine binary exists yet: the bootstrap residual, bounded by this very
// gate's build step.
func (g *gateRun) registerAndFence(name string) int {
	d := g.d
	if executableFile(filepath.Join(g.root, "bin", "metasystem")) {
		marker, err := d.owners.register(g.root, d.selfPid, name)
		if err != nil {
			fmt.Fprintln(d.stderr, err)
			fmt.Fprintln(d.stderr, "go gate: registration failed; refusing to run invisibly")
			return 1
		}
		if marker != "" {
			g.markers = append(g.markers, marker)
		}
	}
	return g.fence()
}

func (g *gateRun) fence() int {
	d := g.d
	if g.env.get("METASYSTEM_ALLOW_CONCURRENT_GATE") == "1" || !executableFile(filepath.Join(g.root, "bin", "metasystem")) {
		return 0
	}
	holders := d.fence(g.root, d.selfPid)
	for _, holder := range holders {
		fmt.Fprintf(d.stderr, "gate %s is running as pid %d\n", holder.Gate, holder.Pid)
	}
	if len(holders) > 0 {
		fmt.Fprintf(d.stderr, "go gate: a live gate run owns this checkout; rebuilding now would swap its binary mid-run\n"+
			"run: %s=1 go run ./cmd/devgate %s  (only to run beside it anyway)\n", concurrentGateVariable, g.action())
		return 1
	}
	return 0
}

// collectStatic runs every static check regardless of earlier reds and
// reports the verdicts as ONE block (Ruling P / commit-gate-collect): a red
// gofmt never hides what vet and staticcheck would have said, so one run
// teaches everything it can. Any red refuses; no check weakens. On success
// g.scratch holds the engine built from the judged bytes.
func (g *gateRun) collectStatic() int {
	d := g.d
	var reds []string
	env := g.env.list()

	var version, toolchain bytes.Buffer
	_ = d.goTool(g.ctx, g.root, env, []string{"version"}, &version, io.Discard)
	_ = d.goTool(g.ctx, g.root, env, []string{"env", "GOTOOLCHAIN"}, &toolchain, io.Discard)
	fmt.Fprintf(d.stdout, "go gate: effective Go: %s\n", strings.TrimRight(version.String(), "\n"))
	fmt.Fprintf(d.stdout, "go gate: GOTOOLCHAIN: %s\n", strings.TrimRight(toolchain.String(), "\n"))

	// gofmt is a hard gate. Its exit status is captured, so a missing or
	// crashing gofmt refuses the gate instead of passing silently
	// (go-production-grade B8). The file list is gathered first: gofmt
	// recurses into every directory and skips only dot-prefixed names, so a
	// directory argument would hand it an installed frontend dependency tree.
	files, err := g.goFiles()
	if err != nil {
		reds = append(reds, fmt.Sprintf("gofmt file list failed: %v", err))
	}
	var gofmtOut bytes.Buffer
	gofmtErr := d.tool(g.ctx, toolCall{dir: g.root, env: env, name: "gofmt", args: append([]string{"-l", "--"}, files...),
		stdout: &gofmtOut, stderr: &gofmtOut})
	unformatted := strings.TrimRight(gofmtOut.String(), "\n")
	if gofmtErr != nil {
		reds = append(reds, fmt.Sprintf("gofmt itself failed (status %d): %s", exitStatus(gofmtErr), unformatted))
	} else if unformatted != "" {
		var names strings.Builder
		for _, name := range strings.Fields(unformatted) {
			names.WriteString(name + " ")
		}
		reds = append(reds, "gofmt would change: "+names.String())
	}

	reds = append(reds, g.shellParse(env)...)

	var vetOut bytes.Buffer
	if d.goTool(g.ctx, g.root, env, []string{"vet", "-trimpath", "-p=" + g.workers, "./..."}, &vetOut, &vetOut) != nil {
		reds = append(reds, "go vet failed:\n"+strings.TrimRight(vetOut.String(), "\n"))
	}
	var staticcheckOut bytes.Buffer
	if d.goTool(g.ctx, g.root, env, []string{"run", "-trimpath", "-p=" + g.workers, staticcheckModule, "./..."}, &staticcheckOut, &staticcheckOut) != nil {
		reds = append(reds, "staticcheck 2026.2 (module v0.8.0) refused (or could not run):\n"+strings.TrimRight(staticcheckOut.String(), "\n"))
	}
	reds = append(reds, g.deadCode()...)

	scratch, err := os.CreateTemp(tempDir(g.env), "metasystem-gate-collect.")
	if err == nil {
		g.scratch = scratch.Name()
		_ = scratch.Close()
		if runBuild(g.ctx, []string{"--out", g.scratch}, g.root, d.withEnvironment(g.env).quiet()) != 0 {
			reds = append(reds, "build failed (devgate build)")
			g.dropScratch()
		}
	} else {
		reds = append(reds, "build failed (devgate build)")
	}
	if g.scratch != "" {
		if owner, err := d.owners.runOwnerRef(d.selfPid); err != nil {
			reds = append(reds, "run owner export failed")
		} else {
			g.env.set("METASYSTEM_RUN_OWNER", owner)
			env = g.env.list()
		}
	}
	// The run-state audit type-checks every package, so it runs only once
	// the build has passed: a compile error is the build's red, not its own.
	if g.scratch != "" {
		if out, ok := d.owners.rootAuditRatchet(g.root); ok {
			fmt.Fprint(d.stdout, out)
		} else {
			reds = append(reds, "run-state audit refused:\n"+strings.TrimRight(out, "\n"))
		}
	}
	if g.fast {
		var refusalOut bytes.Buffer
		if d.goTool(g.ctx, g.root, env, []string{"test", "-trimpath", "-p=" + g.workers, "-count=1", "./internal/refusal"}, &refusalOut, &refusalOut) != nil {
			reds = append(reds, "refusal register failed:\n"+strings.TrimRight(refusalOut.String(), "\n"))
		}
	}

	// wow.md marks both template and adopted installations. The audit source
	// is a second positive signal so deleting the marker cannot downgrade a
	// damaged installation to a fixture. A partial tree that owns neither
	// signal owns no SessionStart hook; a dangling signal still requires the
	// audit.
	g.installation = present(filepath.Join(g.root, "wow.md")) || present(filepath.Join(g.root, "internal", "audit", "hookstartexits.go"))
	if g.scratch != "" && g.installation {
		if out, ok := d.owners.hookStartExits(g.root); !ok {
			reds = append(reds, "SessionStart exit audit failed:\n"+strings.TrimRight(out, "\n"))
		}
	}
	if !g.installation {
		fmt.Fprintln(d.stdout, "go gate: SessionStart exit audit not applicable to this script fixture")
	}

	stopApplicable := false
	if g.installation {
		if _, err := d.git(g.ctx, g.root, "rev-parse", "--is-inside-work-tree"); err == nil {
			stopApplicable = true
		}
	}
	if g.scratch != "" && stopApplicable {
		if out, ok := d.owners.stopSurface(g.root); ok {
			fmt.Fprint(d.stdout, out)
		} else {
			reds = append(reds, "Stop decision surface audit failed:\n"+strings.TrimRight(out, "\n"))
		}
	}
	if !stopApplicable {
		fmt.Fprintln(d.stdout, "go gate: Stop decision surface audit not applicable to this tree")
	}

	// The project's declared memory — the intent and doctrine books, the
	// decisions, the designs and the question register — is checked here,
	// in milliseconds over a few hundred files. A refusal joins the static
	// reds, so a design with a duplicate id, a malformed head or a Goals line
	// naming no ledger goal blocks every delivery that consumes this gate's
	// retained proof (the homes are declared inputs of fast-static-build).
	if g.scratch != "" && g.installation {
		if out, ok := d.owners.projectCheck(g.root); !ok {
			reds = append(reds, "the project check refused a design, decision or question record:\n"+strings.TrimRight(out, "\n"))
		}
	}

	if len(reds) > 0 {
		fmt.Fprintf(d.stderr, "go gate: %d static check(s) red — the complete block:\n", len(reds))
		for _, red := range reds {
			fmt.Fprintf(d.stderr, "--- %s\n", red)
		}
		return 1
	}
	return 0
}

// goFiles is every Go file Git sees, tracked or new, never an ignored tree,
// minus index entries deleted only in the working tree (which gofmt would
// refuse). A tree that is not a repository — a frozen witness export, or a
// plain copy of the source — is walked instead, pruning node_modules by name
// so that installing frontend dependencies cannot change a gate verdict.
func (g *gateRun) goFiles() ([]string, error) {
	var files []string
	if _, err := g.d.git(g.ctx, g.root, "rev-parse", "--is-inside-work-tree"); err == nil {
		listed, err := g.d.git(g.ctx, g.root, "ls-files", "-z", "--cached", "--others", "--exclude-standard", "--", "*.go")
		if err != nil {
			return nil, err
		}
		for _, name := range strings.Split(string(listed), "\x00") {
			if name == "" {
				continue
			}
			if info, err := os.Stat(filepath.Join(g.root, name)); err == nil && info.Mode().IsRegular() {
				files = append(files, name)
			}
		}
		return files, nil
	}
	for _, dir := range []string{"internal", "cmd"} {
		start := filepath.Join(g.root, dir)
		if _, err := os.Stat(start); err != nil {
			continue
		}
		_ = filepath.WalkDir(start, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if entry.IsDir() && entry.Name() == "node_modules" {
				return filepath.SkipDir
			}
			if entry.Type().IsRegular() && strings.HasSuffix(entry.Name(), ".go") {
				rel, _ := filepath.Rel(g.root, path)
				files = append(files, rel)
			}
			return nil
		})
	}
	return files, nil
}

// shellParse runs `bash -n` on every shell script the gate sees, one file per
// run because bash parses only its first operand. A script that does not
// parse fails the moment it is sourced or run, and nothing else in the gate
// executes every script, so without this stage a syntax error can sit on
// main unseen (dispatch-fixtures.sh did, for a day). Every unparsable script
// is named in one red; a bash that cannot start is its own red.
func (g *gateRun) shellParse(env []string) []string {
	files, err := g.shellFiles()
	if err != nil {
		return []string{fmt.Sprintf("shell parse file list failed: %v", err)}
	}
	var broken strings.Builder
	for _, file := range files {
		var out bytes.Buffer
		err := g.d.tool(g.ctx, toolCall{dir: g.root, env: env, name: "bash", args: []string{"-n", "--", file}, stdout: &out, stderr: &out})
		if err == nil {
			continue
		}
		if status := exitStatus(err); status == 127 {
			return []string{fmt.Sprintf("shell parse: bash itself failed (status %d): %s", status, strings.TrimRight(out.String(), "\n"))}
		}
		fmt.Fprintf(&broken, "%s\n", file)
		if text := strings.TrimRight(out.String(), "\n"); text != "" {
			fmt.Fprintf(&broken, "%s\n", text)
		}
	}
	if broken.Len() == 0 {
		return nil
	}
	return []string{"shell parse failed (bash -n):\n" + strings.TrimRight(broken.String(), "\n")}
}

// shellFiles is every *.sh file Git sees, tracked or new, or, outside a
// repository, every one a walk of the tree finds; node_modules is never
// judged, since installing frontend dependencies must not change a verdict.
func (g *gateRun) shellFiles() ([]string, error) {
	var files []string
	keep := func(name string) bool {
		return strings.HasSuffix(name, ".sh") && !slices.Contains(strings.Split(filepath.ToSlash(name), "/"), "node_modules")
	}
	if _, err := g.d.git(g.ctx, g.root, "rev-parse", "--is-inside-work-tree"); err == nil {
		listed, err := g.d.git(g.ctx, g.root, "ls-files", "-z", "--cached", "--others", "--exclude-standard", "--", "*.sh")
		if err != nil {
			return nil, err
		}
		for _, name := range strings.Split(string(listed), "\x00") {
			if name == "" || !keep(name) {
				continue
			}
			if info, err := os.Stat(filepath.Join(g.root, name)); err == nil && info.Mode().IsRegular() {
				files = append(files, name)
			}
		}
		return files, nil
	}
	err := filepath.WalkDir(g.root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && entry.Name() == "node_modules" {
			return filepath.SkipDir
		}
		if entry.Type().IsRegular() && strings.HasSuffix(entry.Name(), ".sh") {
			rel, _ := filepath.Rel(g.root, path)
			files = append(files, rel)
		}
		return nil
	})
	return files, err
}

func (g *gateRun) dropScratch() {
	if g.scratch != "" {
		_ = os.Remove(g.scratch)
		g.scratch = ""
	}
}

// present reports a path that exists or is a dangling symbolic link.
func present(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// tempDir is where the gate stages disposable files: the TMPDIR it was
// given, else its process's registered scratch root (Part B R1), which the
// gate releases before it exits. With neither, the empty root makes each
// allocation use TMPDIR's default, and that allocation fails loudly only if
// that fails too.
func tempDir(env *environment) string {
	if dir := env.get("TMPDIR"); dir != "" {
		return dir
	}
	scratch, _ := diskstore.ProcessScratch()
	return scratch
}

// quiet discards the output of an in-process build whose verdict the gate
// reports itself.
func (d deps) quiet() deps {
	d.stdout, d.stderr = io.Discard, io.Discard
	return d
}
