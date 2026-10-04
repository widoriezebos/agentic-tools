package main

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gaterun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

func TestStaticPublishesTheCollectedBuildWithoutRecompiling(t *testing.T) {
	t.Parallel()
	w := newGateWorld(t)
	proof := filepath.Join(w.tmp, "proof-engine")
	if code := w.static("--proof-out", proof, "--verbose"); code != 0 {
		t.Fatalf("exit %d:\n%s", code, w.output())
	}
	builds := w.called("go build")
	if len(builds) != 1 {
		t.Fatalf("builds = %v, want exactly the collected static build", builds)
	}
	published, err := os.ReadFile(proof)
	if err != nil || !strings.HasPrefix(string(published), "engine built in "+w.root) {
		t.Fatalf("proof engine = %q, %v; want the collected build's bytes", published, err)
	}
	if info, err := os.Stat(proof); err != nil || info.Mode().Perm() != 0o755 {
		t.Fatalf("proof engine mode: %v %v", info, err)
	}
	if data, _ := os.ReadFile(filepath.Join(w.root, "bin", "metasystem")); string(data) != "#!/bin/sh\nexit 0\n" {
		t.Fatalf("--proof-out touched bin/metasystem: %q", data)
	}
	want := "go gate: fast checks passed; landing still needs the full gate\nrun: go run ./cmd/devgate gate\n" +
		"  checked: dependency ratchet, parallel ratchet, gofmt, shell parse, vet, staticcheck,\n" +
		"    dead code, refusal register, SessionStart exit audit, Stop decision surface audit, build\n"
	if !strings.HasSuffix(w.stdout.String(), want) {
		t.Fatalf("stdout does not end with the fast pass line:\n%s", w.stdout.String())
	}
	for _, line := range []string{"go gate: effective Go: go version go1.fixture darwin/arm64", "go gate: GOTOOLCHAIN: auto",
		"go gate: Stop decision surface audit not applicable to this tree"} {
		if !strings.Contains(w.stdout.String(), line+"\n") {
			t.Fatalf("stdout lacks %q:\n%s", line, w.stdout.String())
		}
	}
	// Every Go phase inherits the worker allowance, and the refusal register
	// names this gate as its run owner.
	for _, fragment := range []string{"go vet -trimpath -p=3 ./cmd/... ./internal/...", "go run -trimpath -p=3 " + staticcheckModule + " ./cmd/... ./internal/...", "go test -trimpath -p=3 -count=1 ./internal/refusal", "go build -p=3 -buildvcs=false"} {
		if len(w.called(fragment)) != 1 {
			t.Fatalf("missing %q in %v", fragment, w.calls)
		}
	}
	// Fast mode trims by argv and never exports GOFLAGS (disk-lifetimes A2).
	assertGoCompilesTrimmed(t, w)
	for _, call := range w.calls {
		if _, set := newEnvironment(call.env).lookup("GOFLAGS"); call.name == "go" && set {
			t.Fatalf("static mode exported GOFLAGS to %s: %q", call, call.env)
		}
	}
	defaulted := newGateWorld(t)
	defaulted.env = slices.DeleteFunc(defaulted.env, func(entry string) bool { return strings.HasPrefix(entry, "METASYSTEM_TEST_WORKERS=") })
	if code := defaulted.static(); code != 0 || len(defaulted.called("go vet -trimpath -p=1 ./cmd/... ./internal/...")) != 1 || len(defaulted.called("go test -trimpath -p=1 -count=1 ./internal/refusal")) != 1 {
		t.Fatalf("a direct caller without an allowance did not get one worker: exit %d %v", code, defaulted.calls)
	}
	refusal := w.called("go test")[0]
	if envValue(refusal.env, "METASYSTEM_RUN_OWNER") == "" || envValue(refusal.env, "GOMAXPROCS") != "3" {
		t.Fatalf("refusal register env lacks the run owner or GOMAXPROCS: %q", refusal.env)
	}
	if markers, _ := filepath.Glob(filepath.Join(w.root, "artifacts", "agents", "supervision", "gate-runs", "*")); len(markers) != 0 {
		t.Fatalf("run markers survived the gate: %v", markers)
	}
	if leftovers, _ := filepath.Glob(filepath.Join(w.tmp, "metasystem-gate-collect.*")); len(leftovers) != 0 {
		t.Fatalf("scratch build survived: %v", leftovers)
	}
}

func TestStaticWithoutProofOutReplacesTheInstalledEngine(t *testing.T) {
	t.Parallel()
	w := newGateWorld(t)
	if code := w.static(); code != 0 {
		t.Fatalf("exit %d:\n%s", code, w.output())
	}
	data, err := os.ReadFile(filepath.Join(w.root, "bin", "metasystem"))
	if err != nil || !strings.HasPrefix(string(data), "engine built in") {
		t.Fatalf("bin/metasystem = %q, %v", data, err)
	}
}

func TestStaticCollectsEveryRedInOneBlock(t *testing.T) {
	t.Parallel()
	w := newGateWorld(t)
	w.outputs["gofmt"] = "internal/fixture/fixture.go\ncmd/metasystem/main.go\n"
	w.statuses["vet"], w.outputs["vet"] = 1, "vet: fixture.go:1: bad\n"
	w.statuses["staticcheck"], w.outputs["staticcheck"] = 1, "SA4006 unused\n"
	w.statuses["refusal"], w.outputs["refusal"] = 1, "--- FAIL: TestRegister\n"
	w.owners.hookStartExits = func(string) (string, bool) { return "hook start exit audit: bare exit\n", false }
	w.owners.stopSurface = func(string) (string, bool) { return "removed: x\n", false }
	w.owners.projectCheck = func(string) (string, bool) { return "plans/designs/x.md:1: duplicate id\n", false }
	w.gitInside = true
	w.gitFiles = []string{"internal/fixture/fixture.go", "cmd/metasystem/main.go"}
	if code := w.static(); code != 1 {
		t.Fatalf("exit %d, want 1:\n%s", code, w.output())
	}
	stderr := w.stderr.String()
	for _, want := range []string{
		"go gate: 7 static check(s) red — the complete block:\n",
		"--- gofmt would change: internal/fixture/fixture.go cmd/metasystem/main.go \n",
		"--- go vet failed:\nvet: fixture.go:1: bad\n",
		"--- staticcheck 2026.2 (module v0.8.0) refused (or could not run):\nSA4006 unused\n",
		"--- refusal register failed:\n--- FAIL: TestRegister\n",
		"--- SessionStart exit audit failed:\nhook start exit audit: bare exit\n",
		"--- Stop decision surface audit failed:\nremoved: x\n",
		"--- the project check refused a design, decision or question record:\nplans/designs/x.md:1: duplicate id\n",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr lacks %q:\n%s", want, stderr)
		}
	}
	if data, _ := os.ReadFile(filepath.Join(w.root, "bin", "metasystem")); string(data) != "#!/bin/sh\nexit 0\n" {
		t.Fatalf("a red static gate published an engine: %q", data)
	}
}

// A red engine build carries the build's own words: a red that says only
// "build failed" leaves the reader to rerun the build by hand to learn why.
func TestStaticBuildRedCarriesTheBuildsOwnWords(t *testing.T) {
	t.Parallel()
	w := newGateWorld(t)
	w.statuses["build"], w.outputs["build"] = 1, "cmd/metasystem/main.go:1:1: syntax error\n"
	if code := w.static(); code != 1 {
		t.Fatalf("exit %d, want 1:\n%s", code, w.output())
	}
	if want := "--- build failed (devgate build):\ncmd/metasystem/main.go:1:1: syntax error\ngo-build: build failed\n"; !strings.Contains(w.stderr.String(), want) {
		t.Fatalf("stderr lacks %q:\n%s", want, w.stderr.String())
	}
}

func TestStaticRefusesABrokenGofmtByName(t *testing.T) {
	t.Parallel()
	w := newGateWorld(t)
	w.statuses["gofmt"], w.outputs["gofmt"] = 7, "shim: gofmt is broken\n"
	if code := w.static(); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(w.stderr.String(), "--- gofmt itself failed (status 7): shim: gofmt is broken\n") {
		t.Fatalf("broken gofmt not named:\n%s", w.stderr.String())
	}
}

func TestStaticRatchetsRefuseBeforeAnyToolRuns(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"dependency", "parallel"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			w := newGateWorld(t)
			refusal := "PARALLEL_RATCHET_REFUSED: serial Go test count increased\n"
			if name == "dependency" {
				refusal = "dependency ratchet: scripts/x.sh: python3 undeclared\n"
				w.owners.dependencyRatchet = func(string) (string, bool) { return refusal, false }
				w.owners.parallelRatchet = func(string) (string, bool) {
					t.Error("the parallel ratchet ran after a dependency refusal")
					return "", true
				}
			} else {
				w.owners.parallelRatchet = func(string) (string, bool) { return refusal, false }
			}
			if code := w.static(); code != 1 {
				t.Fatalf("exit %d, want 1", code)
			}
			if w.stderr.String() != refusal {
				t.Fatalf("stderr = %q, want the ratchet's own refusal", w.stderr.String())
			}
			if len(w.called("go ")) != 0 || len(w.called("gofmt")) != 0 {
				t.Fatalf("a ratchet refusal still ran tools: %v", w.calls)
			}
		})
	}
}

func TestStaticAndGateRefuseOnTheRootAuditRatchet(t *testing.T) {
	t.Parallel()
	refusal := "run-state audit: new crossing x/x.go x.F: variable stateRoot -> filepath.Join(_, \"artifacts\") (owner none; found 1, listed 0)\n"
	for _, gate := range []string{"static", "gate"} {
		w := newGateWorld(t)
		w.owners.rootAuditRatchet = func(string) (string, bool) { return refusal, false }
		run := w.static
		if gate == "gate" {
			run = w.gate
		}
		if code := run(); code != 1 || !strings.Contains(w.stderr.String(), "--- run-state audit refused:\n"+refusal) {
			t.Fatalf("%s: exit %d stderr %q", gate, code, w.stderr.String())
		}
		if len(w.nativeCalls) != 0 {
			t.Fatalf("%s: a refused run-state audit still paid the native selection", gate)
		}
		if data, _ := os.ReadFile(filepath.Join(w.root, "bin", "metasystem")); string(data) != "#!/bin/sh\nexit 0\n" {
			t.Fatalf("%s: a refused run-state audit published an engine: %q", gate, data)
		}
	}
	passing := newGateWorld(t)
	if code := passing.static(); code != 0 || !strings.Contains(passing.stdout.String(), "run-state audit passed: 0 open site(s)\n") {
		t.Fatalf("a passing audit: exit %d stdout %q", code, passing.stdout.String())
	}
}

func TestStaticRefusesTheWitnessProtocol(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"METASYSTEM_GATE_WITNESS", "METASYSTEM_GATE_WITNESS_WRITE"} {
		w := newGateWorld(t)
		w.setenv(name + "=/somewhere/witness.json")
		if code := w.static(); code != 1 || !strings.Contains(w.stderr.String(), "go gate: fast mode cannot join the witness protocol; run the full gate") {
			t.Fatalf("%s: exit %d stderr %q", name, code, w.stderr.String())
		}
	}
}

func TestStaticFenceRefusesAForeignLiveGate(t *testing.T) {
	t.Parallel()
	w := newGateWorld(t)
	foreign := foreignController(t)
	if _, err := gaterun.Register(w.root, foreign, "fence-fixture"); err != nil {
		t.Fatal(err)
	}
	if code := w.static(); code != 1 {
		t.Fatalf("exit %d, want 1:\n%s", code, w.output())
	}
	if !strings.Contains(w.stderr.String(), "gate fence-fixture is running as pid ") ||
		!strings.Contains(w.stderr.String(), "rebuilding now would swap its binary mid-run\nrun: METASYSTEM_ALLOW_CONCURRENT_GATE=1 go run ./cmd/devgate static") {
		t.Fatalf("fence refusal not named:\n%s", w.stderr.String())
	}
	if len(w.called("gofmt")) != 0 {
		t.Fatalf("a fenced gate ran its static stages")
	}
	allowed := newGateWorld(t)
	allowed.root = w.root
	allowed.setenv("METASYSTEM_ALLOW_CONCURRENT_GATE=1")
	if code := allowed.static(); code != 0 {
		t.Fatalf("override exit %d:\n%s", code, allowed.output())
	}
}

func TestStaticScriptFixtureScopeSkipsTheInstallationAudits(t *testing.T) {
	t.Parallel()
	w := newGateWorld(t)
	if err := os.Remove(filepath.Join(w.root, "wow.md")); err != nil {
		t.Fatal(err)
	}
	audited := false
	w.owners.hookStartExits = func(string) (string, bool) { audited = true; return "", true }
	w.owners.projectCheck = func(string) (string, bool) { audited = true; return "", true }
	w.gitInside = true
	if code := w.static(); code != 0 {
		t.Fatalf("exit %d:\n%s", code, w.output())
	}
	if audited {
		t.Fatalf("a partial tree ran the installation audits")
	}
	for _, line := range []string{"go gate: SessionStart exit audit not applicable to this script fixture\n",
		"go gate: Stop decision surface audit not applicable to this tree\n",
		"go gate: fast checks passed; landing still needs the full gate\nrun: go run ./cmd/devgate gate\n"} {
		if !strings.Contains(w.stdout.String(), line) {
			t.Fatalf("stdout lacks %q:\n%s", line, w.stdout.String())
		}
	}
	// Any wow.md entry, even a dangling one, makes the tree an installation.
	dangling := newGateWorld(t)
	_ = os.Remove(filepath.Join(dangling.root, "wow.md"))
	if err := os.Symlink("missing-wow-target", filepath.Join(dangling.root, "wow.md")); err != nil {
		t.Fatal(err)
	}
	dangling.owners.hookStartExits = func(string) (string, bool) { return "hook start exit audit: unreadable hook\n", false }
	if code := dangling.static(); code != 1 || !strings.Contains(dangling.stderr.String(), "SessionStart exit audit failed") {
		t.Fatalf("a dangling wow.md was downgraded to a fixture: exit %d\n%s", code, dangling.output())
	}
	// The audit source is a second positive signal: a dangling one still
	// makes the tree an installation.
	w2 := newGateWorld(t)
	_ = os.Remove(filepath.Join(w2.root, "wow.md"))
	if err := os.MkdirAll(filepath.Join(w2.root, "internal", "audit"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(w2.root, "missing"), filepath.Join(w2.root, "internal", "audit", "hookstartexits.go")); err != nil {
		t.Fatal(err)
	}
	if code := w2.static("--verbose"); code != 0 || !strings.Contains(w2.stdout.String(), "SessionStart exit audit, Stop decision surface audit, build\n") {
		t.Fatalf("dangling audit signal: exit %d\n%s", code, w2.output())
	}
}

func TestStaticGofmtListComesFromGitOrAPrunedWalk(t *testing.T) {
	t.Parallel()
	w := newGateWorld(t)
	w.gitInside = true
	w.gitFiles = []string{"internal/fixture/fixture.go", "internal/deleted.go", "cmd/metasystem/main.go"}
	if code := w.static(); code != 0 {
		t.Fatalf("exit %d:\n%s", code, w.output())
	}
	if !strings.Contains(w.stdout.String(), "stop decision surface: fixture; added 0, moved 0, removed 0\n") {
		t.Fatalf("the Stop decision surface report was not printed:\n%s", w.stdout.String())
	}
	gofmt := w.called("gofmt")[0]
	if want := []string{"-l", "--", "internal/fixture/fixture.go", "cmd/metasystem/main.go"}; !slices.Equal(gofmt.args, want) {
		t.Fatalf("gofmt args = %q, want %q (index entries deleted only in the tree dropped)", gofmt.args, want)
	}

	walked := newGateWorld(t)
	walked.write("internal/ui/web/_app/node_modules/dep/dep.go", "package dep\n")
	walked.write("cmd/devgate/main.go", "package main\n")
	if code := walked.static(); code != 0 {
		t.Fatalf("exit %d:\n%s", code, walked.output())
	}
	args := walked.called("gofmt")[0].args
	if slices.ContainsFunc(args, func(arg string) bool { return strings.Contains(arg, "node_modules") }) ||
		!slices.Contains(args, filepath.Join("cmd", "devgate", "main.go")) || !slices.Contains(args, filepath.Join("internal", "fixture", "fixture.go")) {
		t.Fatalf("walked gofmt args = %q", args)
	}
}

func TestStaticUsageAndWorkerRefusals(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name   string
		args   []string
		env    string
		code   int
		stderr string
	}{
		{name: "unknown argument", args: []string{"--fast"}, code: 2, stderr: "go gate: unknown argument --fast"},
		{name: "proof out without path", args: []string{"--proof-out"}, code: 2, stderr: "go gate: --proof-out needs a path"},
		{name: "invalid workers", env: "METASYSTEM_TEST_WORKERS=0", code: 1, stderr: "go gate: the test worker count \"0\" is not a positive integer\nrun: unset METASYSTEM_TEST_WORKERS"},
	} {
		w := newGateWorld(t)
		if test.env != "" {
			w.setenv(test.env)
		}
		if code := w.static(test.args...); code != test.code || !strings.Contains(w.stderr.String(), test.stderr) {
			t.Fatalf("%s: exit %d stderr %q", test.name, code, w.stderr.String())
		}
		if len(w.calls) != 0 {
			t.Fatalf("%s: refusal ran tools %v", test.name, w.calls)
		}
	}
}

func TestCallerPathDropsTheGoRootBinGoRunPrepends(t *testing.T) {
	t.Parallel()
	goroot := t.TempDir()
	bin := filepath.Join(goroot, "bin")
	for _, dir := range []string{bin, filepath.Join(goroot, "pkg", "tool")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, tool := range []string{"go", "gofmt"} {
		if err := testexec.WriteFile(filepath.Join(bin, tool), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	shim := t.TempDir()
	for _, test := range []struct{ path, want string }{
		{bin + ":" + shim + ":/usr/bin", shim + ":/usr/bin"},
		{bin + ":" + bin + ":/usr/bin", bin + ":/usr/bin"},
		{shim + ":" + bin, shim + ":" + bin},
		{bin, bin},
		{filepath.Join(shim, "bin") + ":/usr/bin", filepath.Join(shim, "bin") + ":/usr/bin"},
	} {
		if got := callerPath(test.path); got != test.want {
			t.Fatalf("callerPath(%q) = %q, want %q", test.path, got, test.want)
		}
	}
}

func TestStaticRefusesAShellScriptThatDoesNotParse(t *testing.T) {
	t.Parallel()
	w := newGateWorld(t)
	// The apostrophe inside a single-quoted pattern: the shape that left
	// dispatch-fixtures.sh unparsable on main for a day.
	w.write("scripts/agents/broken.sh", "#!/usr/bin/env bash\nif grep -q 'it's here' x; then\n  echo yes\nfi\n")
	w.write("scripts/agents/fine.sh", "#!/usr/bin/env bash\necho fine\n")
	// node_modules is never judged, broken or not.
	w.write("internal/ui/web/_app/node_modules/dep/broken.sh", "if then fi (\n")
	if code := w.static(); code != 1 {
		t.Fatalf("exit %d, want 1:\n%s", code, w.output())
	}
	stderr := w.stderr.String()
	if !strings.Contains(stderr, "go gate: 1 static check(s) red — the complete block:\n") ||
		!strings.Contains(stderr, "--- shell parse failed (bash -n):\n"+filepath.Join("scripts", "agents", "broken.sh")+"\n") {
		t.Fatalf("unparsable script not named:\n%s", stderr)
	}
	if strings.Contains(stderr, "node_modules") || strings.Contains(stderr, "fine.sh") {
		t.Fatalf("shell parse judged a script it must not:\n%s", stderr)
	}
	parsed := w.called("bash -n --")
	if len(parsed) != 2 {
		t.Fatalf("bash -n calls = %v, want one per script outside node_modules", parsed)
	}
	if data, _ := os.ReadFile(filepath.Join(w.root, "bin", "metasystem")); string(data) != "#!/bin/sh\nexit 0\n" {
		t.Fatalf("a red shell parse published an engine: %q", data)
	}
}

func TestStaticShellParseReadsGitsListAndNamesABrokenBash(t *testing.T) {
	t.Parallel()
	w := newGateWorld(t)
	w.gitInside = true
	w.write("scripts/agents/tracked.sh", "echo tracked\n")
	w.write("scripts/agents/ignored.sh", "if then\n")
	w.gitFiles = []string{"internal/fixture/fixture.go", "scripts/agents/tracked.sh", "scripts/agents/deleted.sh",
		"internal/ui/web/_app/node_modules/dep/x.sh"}
	if code := w.static(); code != 0 {
		t.Fatalf("exit %d:\n%s", code, w.output())
	}
	parsed := w.called("bash -n --")
	if len(parsed) != 1 || !slices.Equal(parsed[0].args, []string{"-n", "--", "scripts/agents/tracked.sh"}) {
		t.Fatalf("bash -n calls = %v, want Git's listed, present script only", parsed)
	}
	if gofmt := w.called("gofmt")[0]; !slices.Equal(gofmt.args, []string{"-l", "--", "internal/fixture/fixture.go"}) {
		t.Fatalf("gofmt args = %q; the shell list leaked into the Go list", gofmt.args)
	}

	broken := newGateWorld(t)
	broken.write("scripts/agents/tracked.sh", "echo tracked\n")
	broken.statuses["bash"], broken.outputs["bash"] = 127, "bash: not found\n"
	if code := broken.static(); code != 1 ||
		!strings.Contains(broken.stderr.String(), "--- shell parse: bash itself failed (status 127): bash: not found\n") {
		t.Fatalf("broken bash: exit %d\n%s", code, broken.output())
	}
}

// The dead-code stage runs deadcode with the tests as roots once per
// supported platform: a function neither a program nor any test reaches on
// every platform is dead, and the gate names it. A planted function with no
// caller is exactly that.
func TestStaticRefusesAFunctionNothingReaches(t *testing.T) {
	t.Parallel()
	w := newGateWorld(t)
	planted := "internal/fixture/fixture.go:3:6: unreachable func: Planted\n"
	w.outputs["deadcode-darwin"], w.outputs["deadcode-linux"] = planted, planted
	if code := w.static(); code != 1 {
		t.Fatalf("exit %d, want 1:\n%s", code, w.output())
	}
	want := "--- dead code: deadcode (golang.org/x/tools v0.50.0, tests as roots) finds functions nothing reaches on darwin and linux; delete them, or allowlist a deliberate one with its reason in cmd/devgate/deadcode.go:\ninternal/fixture/fixture.go:3:6: unreachable func: Planted\n"
	if !strings.Contains(w.stderr.String(), want) {
		t.Fatalf("stderr lacks %q:\n%s", want, w.stderr.String())
	}
	if installs := w.called("go install -trimpath " + deadcodeModule); len(installs) != 1 || envValue(installs[0].env, "GOOS") != "" || envValue(installs[0].env, "GOBIN") == "" {
		t.Fatalf("deadcode must be installed once, for the host, into a scratch GOBIN: %v", installs)
	}
	var platforms []string
	for _, call := range w.calls {
		if filepath.Base(call.name) == "deadcode" && strings.Join(call.args, " ") == "-test ./cmd/... ./internal/..." {
			platforms = append(platforms, envValue(call.env, "GOOS"))
		}
	}
	if slices.Sort(platforms); strings.Join(platforms, ",") != "darwin,linux" {
		t.Fatalf("deadcode platforms = %v, want darwin and linux", platforms)
	}
}

// A function one platform's files reach is alive: only what is unreachable
// on every platform is dead.
func TestStaticDeadCodeIsWhatEveryPlatformLeavesUnreached(t *testing.T) {
	t.Parallel()
	w := newGateWorld(t)
	w.outputs["deadcode-darwin"] = "internal/fixture/proc.go:9:6: unreachable func: linuxOnly\n"
	if code := w.static(); code != 0 {
		t.Fatalf("a function the linux build reaches was refused: exit %d:\n%s", code, w.output())
	}
}

func TestStaticDeadCodeThatCannotRunIsRed(t *testing.T) {
	t.Parallel()
	uninstallable := newGateWorld(t)
	uninstallable.statuses["deadcode-install"], uninstallable.outputs["deadcode-install"] = 1, "go: offline\n"
	if code := uninstallable.static(); code != 1 || !strings.Contains(uninstallable.stderr.String(), "--- dead code check could not install deadcode (golang.org/x/tools v0.50.0):\ngo: offline\n") {
		t.Fatalf("a dead-code tool that could not install passed: exit %d:\n%s", code, uninstallable.output())
	}
	w := newGateWorld(t)
	w.statuses["deadcode-linux"], w.outputs["deadcode-linux"] = 1, "go: module lookup disabled\n"
	if code := w.static(); code != 1 || !strings.Contains(w.stderr.String(), "--- dead code check could not run on linux (deadcode golang.org/x/tools v0.50.0):\ngo: module lookup disabled\n") {
		t.Fatalf("a dead-code check that could not run passed: exit %d:\n%s", code, w.output())
	}
}

func TestDeadCodeAllowlistEntriesCarryAReasonAndAreStillDead(t *testing.T) {
	t.Parallel()
	for key, reason := range deadCodeAllowed {
		if strings.TrimSpace(reason) == "" || !strings.Contains(key, "#") {
			t.Errorf("allowlist entry %q needs the form file.go#Name and a reason", key)
		}
	}
	findings := parseDeadCode("a/b.go:1:6: unreachable func: Kept\nnoise line\nc/d.go:2:6: unreachable func: T.Method\n")
	if len(findings) != 2 || findings[0].key != "a/b.go#Kept" || findings[1].key != "c/d.go#T.Method" {
		t.Fatalf("findings = %+v", findings)
	}
}

// TestStaticJudgesOnlyTheListedTrees: vet, staticcheck and deadcode judge the
// top-level trees of the files Git lists, never ./..., so a source copy under
// the ignored artifacts/ is neither vetted nor checked.
func TestStaticJudgesOnlyTheListedTrees(t *testing.T) {
	t.Parallel()
	if got := goPatterns([]string{"internal/a/b.go", "cmd/x/main.go", "doc.go", "internal/c.go"}); strings.Join(got, " ") != ". ./cmd/... ./internal/..." {
		t.Fatalf("goPatterns = %q; want the root and one pattern per top-level tree", got)
	}
	w := newGateWorld(t)
	w.gitInside = true
	w.write("artifacts/agents/suite-failures/x/copy.go", "package copy\n\nfunc broken( {\n")
	w.gitFiles = []string{"internal/fixture/fixture.go", "cmd/metasystem/main.go"}
	if code := w.static(); code != 0 {
		t.Fatalf("exit %d:\n%s", code, w.output())
	}
	judged := 0
	for _, call := range w.calls {
		args := strings.Join(call.args, " ")
		if !strings.HasPrefix(args, "vet ") && !strings.Contains(args, staticcheckModule) && filepath.Base(call.name) != "deadcode" {
			continue
		}
		judged++
		if strings.Contains(args, "./...") || strings.Contains(args, "artifacts") || !strings.HasSuffix(args, " ./cmd/... ./internal/...") {
			t.Errorf("%s judges %q; want ./cmd/... ./internal/... only", call.name, args)
		}
	}
	if judged != 4 {
		t.Fatalf("judging calls = %d; want vet, staticcheck and deadcode on two platforms", judged)
	}
}

// The pinned static tools resolve from this computer's module cache alone:
// staticcheck's run and deadcode's install carry the cache as their only
// module proxy, read once from go env GOMODCACHE, while the other stages
// keep the caller's environment.
func TestStaticRunsThePinnedToolsFromTheModuleCacheOnly(t *testing.T) {
	t.Parallel()
	w := newGateWorld(t)
	if code := w.static(); code != 0 {
		t.Fatalf("exit %d:\n%s", code, w.output())
	}
	if reads := w.called("go env GOMODCACHE"); len(reads) != 1 {
		t.Fatalf("go env GOMODCACHE calls = %v, want one per static run", reads)
	}
	want := "file://" + w.modCache + "/cache/download"
	staticcheck := w.called("go run -trimpath -p=3 " + staticcheckModule)
	install := w.called("go install -trimpath " + deadcodeModule)
	if len(staticcheck) != 1 || envValue(staticcheck[0].env, "GOPROXY") != want {
		t.Fatalf("staticcheck must run with GOPROXY=%s: %v", want, staticcheck)
	}
	if len(install) != 1 || envValue(install[0].env, "GOPROXY") != want {
		t.Fatalf("deadcode must install with GOPROXY=%s: %v", want, install)
	}
	for _, call := range w.called("go vet") {
		if _, set := newEnvironment(call.env).lookup("GOPROXY"); set {
			t.Fatalf("vet was given a module proxy: %q", call.env)
		}
	}
}

// A pinned tool the module cache lacks is red with the warm-up command a
// person runs on a computer with network; static never fetches it itself.
func TestStaticNamesTheWarmUpWhenTheModuleCacheLacksAPinnedTool(t *testing.T) {
	t.Parallel()
	w := newGateWorld(t)
	missing := func(module string) string {
		return "go: " + module + ": " + module + ": reading file://" + w.modCache + "/cache/download/" +
			strings.Replace(module, "@", "/@v/", 1) + ".info: no such file or directory\n"
	}
	w.statuses["staticcheck"], w.outputs["staticcheck"] = 1, missing(staticcheckModule)
	w.statuses["deadcode-install"], w.outputs["deadcode-install"] = 1, missing(deadcodeModule)
	if code := w.static(); code != 1 {
		t.Fatalf("exit %d, want 1:\n%s", code, w.output())
	}
	warm := "run: go run ./cmd/devgate warm  (from metasystem/, on a computer with network)\n"
	for _, want := range []string{
		"--- staticcheck 2026.2 (module v0.8.0) is not in this computer's module cache\n" + warm + missing(staticcheckModule),
		"--- deadcode (golang.org/x/tools v0.50.0) is not in this computer's module cache\n" + warm + missing(deadcodeModule),
	} {
		if !strings.Contains(w.stderr.String(), want) {
			t.Fatalf("stderr lacks %q:\n%s", want, w.stderr.String())
		}
	}
}

// Without a module cache to read from, the pinned tools cannot run offline,
// so they do not run and the static stage is red with the cause.
func TestStaticWithoutAModuleCacheIsRed(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name  string
		world func(*gateWorld)
		want  string
	}{
		{"empty answer", func(w *gateWorld) { w.modCache = "" }, "go env GOMODCACHE answered nothing\n"},
		{"failed", func(w *gateWorld) {
			w.modCache = ""
			w.statuses["gomodcache"], w.outputs["gomodcache"] = 1, "go: GOMODCACHE entry is relative\n"
		}, "go env GOMODCACHE failed (status 1): go: GOMODCACHE entry is relative\n"},
	} {
		w := newGateWorld(t)
		test.world(w)
		if code := w.static(); code != 1 {
			t.Fatalf("%s: exit %d, want 1:\n%s", test.name, code, w.output())
		}
		want := "--- staticcheck and dead code did not run: the module cache could not be located\n" + test.want
		if !strings.Contains(w.stderr.String(), want) {
			t.Fatalf("%s: stderr lacks %q:\n%s", test.name, want, w.stderr.String())
		}
		if ran := append(w.called(staticcheckModule), w.called(deadcodeModule)...); len(ran) != 0 {
			t.Fatalf("%s: the pinned tools ran without the module cache: %v", test.name, ran)
		}
	}
}

// warm installs each pinned static tool once with the caller's environment,
// so the go command fills the module cache from wherever the caller fetches
// modules, then removes its scratch GOBIN.
func TestWarmFillsTheModuleCacheForBothPinnedTools(t *testing.T) {
	t.Parallel()
	w := newGateWorld(t)
	w.setenv("GOPROXY=https://proxy.example,direct")
	if code := run(context.Background(), []string{"warm"}, w.root, w.deps()); code != 0 {
		t.Fatalf("exit %d:\n%s", code, w.output())
	}
	installs := w.called("go install -trimpath ")
	if len(installs) != 2 || installs[0].args[2] != staticcheckModule || installs[1].args[2] != deadcodeModule {
		t.Fatalf("installs = %v, want staticcheck then deadcode", installs)
	}
	for _, call := range installs {
		bin := envValue(call.env, "GOBIN")
		if envValue(call.env, "GOPROXY") != "https://proxy.example,direct" || !strings.HasPrefix(bin, filepath.Join(w.tmp, "metasystem-devgate-warm.")) {
			t.Fatalf("%s: env %q, want the caller's GOPROXY and a scratch GOBIN", call, call.env)
		}
		if _, err := os.Stat(bin); !os.IsNotExist(err) {
			t.Fatalf("scratch GOBIN %s survived warm: %v", bin, err)
		}
	}
	want := "devgate warm: staticcheck is in this computer's module cache\n" +
		"devgate warm: deadcode is in this computer's module cache\n"
	if w.stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", w.stdout.String(), want)
	}

	failed := newGateWorld(t)
	failed.statuses["deadcode-install"], failed.outputs["deadcode-install"] = 1, "go: dial tcp: no route to host\n"
	if code := run(context.Background(), []string{"warm"}, failed.root, failed.deps()); code != 1 ||
		!strings.Contains(failed.stderr.String(), "devgate warm: deadcode could not be installed ("+deadcodeModule+"):\ngo: dial tcp: no route to host\n") {
		t.Fatalf("a failed warm-up: exit %d:\n%s", code, failed.output())
	}
}
