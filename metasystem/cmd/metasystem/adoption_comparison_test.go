package main

import (
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
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginebuild"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// The adoption delivery comparison: two adopted installations, made by the
// public `system adopt` action from a frozen snapshot of this source, each
// driven through its own delivery machinery. The filled target proves its
// native application delivery, the covenant evidence gate and the SessionStart
// audit surface; the copied target proves its registration audits. Both are
// expensive (an engine build and a test run on an adopted target each), so the
// test runs only when the adoption-fixtures section asks for it.
func TestAdoptionComparisonSelectedScenarios(t *testing.T) {
	if os.Getenv("METASYSTEM_ADOPTION_COMPARISON") != "1" {
		t.Skip("the adoption delivery comparison runs in the adoption-fixtures section only")
	}
	selected, err := proofrun.FixtureScenarios("adoption", "comparison")
	want := []string{"filled-target-delivery", "copied-registration-setup", "copied-registration-positive", "copy-drift-source", "copy-drift-registration"}
	if err != nil || !reflect.DeepEqual(selected, want) {
		t.Fatalf("adoption scenario selection: %v, %v", selected, err)
	}
	source, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	frozen, err := proofrun.Freeze(source)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = frozen.Close() })
	source = frozen.Root
	bed, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("METASYSTEM_SUPERVISION_REGISTRY_HOME", filepath.Join(bed, "registry"))
	baseEnvironment := append(receiptCanaryEnvironment(), "GOFLAGS=-mod=readonly", "METASYSTEM_OWNER_LINEAGE=adoption-comparison", "METASYSTEM_SUPERVISION_REGISTRY_HOME="+filepath.Join(bed, "registry"),
		// A fixed worker count: an automatic one follows host load and would
		// change the proof's environment identity between run and verify.
		"METASYSTEM_TESTING_WORKERS=3")
	environment := append([]string(nil), baseEnvironment...)
	run := func(cwd string, extra []string, argv ...string) (string, int) {
		t.Helper()
		cmd := exec.Command(argv[0], argv[1:]...)
		cmd.Dir, cmd.Env = cwd, append(append([]string(nil), environment...), extra...)
		started := time.Now()
		output, err := cmd.CombinedOutput()
		code := 0
		if err != nil {
			if exit, ok := err.(*exec.ExitError); ok {
				code = exit.ExitCode()
			} else {
				t.Fatalf("start %q in %s: %v", argv, cwd, err)
			}
		}
		t.Logf("command=%q cwd=%s status=%d elapsed=%s\n%s", argv, cwd, code, time.Since(started), output)
		return string(output), code
	}
	mustRun := func(cwd string, argv ...string) string {
		t.Helper()
		output, code := run(cwd, nil, argv...)
		if code != 0 {
			// Retain the native failure before the disposable target is removed.
			attempts, _ := proofrun.ReadAttempts(cwd)
			for _, attempt := range attempts {
				if attempt.TestResult == nil {
					continue
				}
				for _, group := range attempt.TestResult.Groups {
					if group.Status != "passed" && group.LogPath != "" {
						log, err := os.ReadFile(group.LogPath)
						t.Logf("native group %s status=%s log=%s read=%v\n%s", group.ID, group.Status, group.LogPath, err, log)
					}
				}
			}
			t.Fatalf("command %q failed with %d:\n%s", argv, code, output)
		}
		return output
	}
	isolateFixtureAdmission := func() {
		// Keep the adopted installation's real runtime configuration. A
		// separate config-backed synthetic fixture root authorizes a private
		// host slot for this disposable target's nested proof and audit probes.
		authorityRoot := t.TempDir()
		if err := os.WriteFile(filepath.Join(authorityRoot, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		environment = append(append([]string(nil), baseEnvironment...),
			"METASYSTEM_PROOF_ADMISSION_TEST_DIR="+filepath.Join(t.TempDir(), "host-admission"),
			"METASYSTEM_PROOF_ADMISSION_FIXTURE_ROOT="+authorityRoot)
	}
	gitProbe := exec.Command("git", "-C", source, "rev-parse", "--is-inside-work-tree")
	if err := gitProbe.Run(); err != nil {
		runReceiptGit(t, source, "init", "-q", "-b", "main")
		runReceiptGit(t, source, "config", "user.name", "adoption source")
		runReceiptGit(t, source, "config", "user.email", "fixture@example.invalid")
	}
	runReceiptGit(t, source, "config", "metasystem.goal.machine", "fixture-machine")
	runReceiptGit(t, source, "add", ".")
	if status := strings.TrimSpace(runReceiptGit(t, source, "status", "--porcelain")); status != "" {
		runReceiptGit(t, source, "-c", "user.name=adoption source", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "exact comparison source")
	}
	commit := runReceiptGit(t, source, "rev-parse", "HEAD")
	sourceEngine := filepath.Join(source, "bin", "metasystem")
	mustRun(source, "go", "build", "-buildvcs=false", "-ldflags", enginebuild.StampLinkerFlags(commit), "-o", sourceEngine, "./cmd/metasystem")

	prepare := func(name, runtimes string, copySkills bool) string {
		t.Helper()
		target := filepath.Join(bed, name)
		if err := os.MkdirAll(target, 0755); err != nil {
			t.Fatal(err)
		}
		runReceiptGit(t, target, "init", "-q", "-b", "main")
		runReceiptGit(t, target, "config", "user.name", "adoption fixture")
		runReceiptGit(t, target, "config", "user.email", "fixture@example.invalid")
		writeReceiptFixture(t, target, "README.md", "application source\n")
		runReceiptGit(t, target, "add", ".")
		runReceiptGit(t, target, "commit", "-qm", "application base")
		argv := []string{sourceEngine, "system", "adopt", target, "--runtimes", runtimes}
		if copySkills {
			argv = append(argv, "--copy-skills")
		}
		mustRun(source, argv...)
		fillAdoptionHarnessConf(t, filepath.Join(target, "metasystem.conf"), filepath.Join(bed, name+"-comparison-evidence"))
		fillAdoptionHarnessTestingContract(t, filepath.Join(source, "testing.json"), filepath.Join(target, "testing.json"))
		if name == "filled" {
			// Only the filled target carries a covenant: its evidence table lives
			// under docs/, which the copied target's payload digest must match.
			prepareFilledTargetCovenant(t, target)
		}
		fillAdoptionPlaceholders(t, filepath.Join(target, "docs", "project-rules.md"))
		var contract testpolicy.Contract
		if name == "filled" {
			// An adopted app owns its tests. Re-running the source engine's
			// entire cmd suite would recursively launch other adoption beds;
			// the outer proof already governs that engine source.
			contract = adoptionAppTestingContract()
		} else {
			data, err := os.ReadFile(filepath.Join(target, "testing.json"))
			if err != nil || json.Unmarshal(data, &contract) != nil {
				t.Fatalf("read copied contract: %v", err)
			}
		}
		if err := contract.Validate(); err != nil {
			t.Fatalf("adoption testing contract: %v", err)
		}
		data, err := json.MarshalIndent(contract, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		writeReceiptFixture(t, target, "testing.json", string(data)+"\n")
		seedAdoptionComparisonGoal(t, target)
		runReceiptGit(t, target, "add", "-A")
		// Fixture genesis precedes the public proof; subsequent commands keep
		// adoption's installed hooks and use the accepted local goal.
		runReceiptGit(t, target, "-c", "core.hooksPath=/dev/null", "commit", "-qm", "reviewed filled initialization and fixture goal")
		commit := runReceiptGit(t, target, "rev-parse", "HEAD")
		remote := filepath.Join(bed, name+"-origin.git")
		runReceiptGit(t, bed, "init", "-q", "--bare", remote)
		runReceiptGit(t, target, "remote", "add", "origin", remote)
		runReceiptGit(t, target, "push", "-q", "-u", "origin", "main")
		runReceiptGit(t, target, "update-ref", goal.LocalLedgerBranch, commit)
		runReceiptGit(t, target, "update-ref", goal.AcceptedRef, commit)
		runReceiptGit(t, target, "config", "metasystem.steward.landing-ref", "refs/remotes/origin/main")
		// The actual reviewed source is committed before resolving the policy
		// base, and these compiled bytes carry that source's exact commit stamp.
		engine := filepath.Join(target, "bin", "metasystem")
		mustRun(target, "go", "build", "-buildvcs=false", "-ldflags", enginebuild.StampLinkerFlags(commit), "-o", engine, "./cmd/metasystem")
		digest, err := fileSHA256(engine)
		if err != nil {
			t.Fatal(err)
		}
		if err := steward.MintIdentity(steward.RepoIdentityPath(target), steward.InstallIdentity{RepoIdentity: target, Generation: 1, InstallPath: engine, InstallDigest: "sha256:" + digest, MintedAt: time.Now().UTC().Format(time.RFC3339), Enrollment: steward.EnrollmentFixture, EngineBuild: commit, LandedCommit: commit, LandingRef: "refs/remotes/origin/main"}); err != nil {
			t.Fatal(err)
		}
		pid := int64(os.Getpid())
		started, ok := lease.StartedAt(pid, nil)
		if !ok {
			t.Fatal("adoption fixture main identity is unreadable")
		}
		if _, err := lease.Announce(target, "adoption-"+name, pid, started, "adoption-"+name, "codex", "adoption-comparison"); err != nil {
			t.Fatal(err)
		}
		if name != "filled" {
			// The copied target's assertions are registration-only (setup,
			// digest, drift, orphan) and use the built engine. The filled
			// target separately proves its own native application delivery.
			return target
		}
		tree := runReceiptGit(t, target, "write-tree")
		isolateFixtureAdmission()
		first := filepath.Join(bed, name+"-first.json")
		plan := mustRun(target, engine, "test", "plan", "--root", target, "--goal", "adoption-goal", "--tree", tree, "--mode", "auto", "--purpose", "delivery")
		if !strings.Contains(plan, "groups=adopted-app-go") || strings.Contains(plan, "go-batchtest") {
			t.Fatalf("adopted app plan selected the wrong tests:\n%s", plan)
		}
		mustRun(target, engine, "internal", "test", "run", "--root", target, "--goal", "adoption-goal", "--tree", tree, "--mode", "auto", "--purpose", "delivery", "--cap-min", "10", "--result", first)
		mustRun(target, engine, "internal", "test", "verify", "--root", target, "--goal", "adoption-goal", "--tree", tree)
		repeat := filepath.Join(bed, name+"-repeat.json")
		output, code := run(target, nil, engine, "internal", "test", "run", "--root", target, "--goal", "adoption-goal", "--tree", tree, "--mode", "auto", "--purpose", "delivery", "--cap-min", "10", "--result", repeat)
		if code != proofrun.ExitReusableSuccess {
			t.Fatalf("completed prerequisite was not reused: status=%d\n%s", code, output)
		}
		var original, reused proofrun.TestResult
		firstBytes, e1 := os.ReadFile(first)
		repeatBytes, e2 := os.ReadFile(repeat)
		if e1 != nil || e2 != nil || json.Unmarshal(firstBytes, &original) != nil || json.Unmarshal(repeatBytes, &reused) != nil || original.AttemptID == "" || original.AttemptID != reused.AttemptID {
			t.Fatal("prerequisite reuse lost exact terminal attempt authority")
		}
		if len(original.Groups) != 1 || original.Groups[0].ID != "adopted-app-go" ||
			original.Groups[0].Status != "passed" || !original.Groups[0].NativeLaunched || !original.Groups[0].CollectionComplete ||
			original.LaunchCounts.Test != 1 || !original.Delivery.Sufficient ||
			len(original.Groups[0].Observed) != 1 || original.Groups[0].Observed[0].Name != "TestAdoptedAppGreeting" || original.Groups[0].Observed[0].Status != "passed" {
			t.Fatalf("adopted app proof lacked one real passing Go test: %+v", original)
		}
		// Both real test runs removed their scratch roots and records.
		for _, pattern := range []string{filepath.Join(target, "artifacts", "agents", "proof-runs", "scratch", "*"),
			filepath.Join(target, "*", "artifacts", "agents", "proof-runs", "scratch", "*")} {
			if left, _ := filepath.Glob(pattern); len(left) != 0 {
				t.Fatalf("test run left scratch behind: %v", left)
			}
		}
		return target
	}
	// registrationCheck runs the adopted target's host registration check,
	// the owner of its registration rules, and reports whether it passed.
	registrationCheck := func(target, runtimes string, copySkills bool) (string, bool) {
		argv := []string{filepath.Join(target, "bin", "metasystem"), "internal", "runtime", "setup", "--repo", target, "--runtimes", runtimes}
		if copySkills {
			argv = append(argv, "--copy-skills")
		}
		output, code := run(target, nil, append(argv, "--check")...)
		return output, code == 0
	}
	refusedRegistration := func(target, runtimes string, copySkills bool, what, message string) {
		t.Helper()
		output, passed := registrationCheck(target, runtimes, copySkills)
		if passed {
			t.Fatalf("the registration check missed %s", what)
		}
		if !strings.Contains(output, message) {
			t.Fatalf("the refusal of %s did not say %q:\n%s", what, message, output)
		}
	}

	// The filled target: its own delivery, then fault injections against
	// the same covenant owner and the SessionStart audit surface.
	filled := prepare("filled", "claude", false)
	engine := filepath.Join(filled, "bin", "metasystem")
	mustRun(filled, engine, "internal", "test", "verify", "--root", filled, "--tree", runReceiptGit(t, filled, "write-tree"))
	// The adopted target proves itself with what its shipped CI runs: the
	// testing contract (its delivery proof above reran and verified) and the
	// system check, which judges the covenant's shape; the covenant evidence
	// gate passes on the green table.
	// Its exit reflects the checkout's live supervision, which a fixture
	// target does not run; the covenant verdict is the adoption claim.
	check, checkCode := run(filled, nil, engine, "system", "check", "--repo", filled, "--json")
	var checked struct {
		Data struct {
			Covenant struct {
				Present, Valid bool
			} `json:"covenant"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(check), &checked); err != nil || !checked.Data.Covenant.Present || !checked.Data.Covenant.Valid {
		t.Fatalf("the filled target's system check did not find its covenant valid (%d, %v):\n%s", checkCode, err, check)
	}
	if evidence := mustRun(filled, engine, "internal", "covenant", "evidence", "--root", filled); !strings.Contains(evidence, "(proof greets): ") {
		t.Fatalf("the filled target's covenant evidence gate did not judge its requirement:\n%s", evidence)
	}
	for _, path := range []string{"wow.md", "internal/hooks/runtime_hook_start.go", "internal/hooks/runtime_hook_start_test.go"} {
		if _, err := os.Stat(filepath.Join(filled, path)); err != nil {
			t.Fatalf("the installed target omitted the SessionStart audit surface: %s", path)
		}
	}
	assertion := filepath.Join(filled, "internal", "hooks", "runtime_hook_start_test.go")
	hidden := filepath.Join(bed, "runtime-hook-start-test.go")
	if err := os.Rename(assertion, hidden); err != nil {
		t.Fatal(err)
	}
	output, code := run(filled, nil, engine, "internal", "audit", "hook-start-exits", "--root", filled)
	if err := os.Rename(hidden, assertion); err != nil {
		t.Fatal(err)
	}
	if code == 0 || !strings.Contains(output, "hook start exit audit could not read "+assertion) {
		t.Fatalf("the SessionStart audit passed or refused without naming its missing assertion source (%d):\n%s", code, output)
	}
	evidencePath := filepath.Join(filled, "docs", "covenant-evidence.md")
	green, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(evidencePath, []byte(strings.Replace(string(green), "| greets |", "| salutes |", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if output, code := run(filled, nil, engine, "internal", "covenant", "evidence", "--root", filled); code == 0 || !strings.Contains(output, "bound to proof greets in the covenant but records proof salutes") {
		t.Fatalf("the evidence gate accepted or misnamed a table recording another proof (%d):\n%s", code, output)
	}
	if err := os.WriteFile(evidencePath, green, 0o644); err != nil {
		t.Fatal(err)
	}
	covenantPath := filepath.Join(filled, "covenant.json")
	covenantBytes, err := os.ReadFile(covenantPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(covenantPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("covenant-that-does-not-exist.json", covenantPath); err != nil {
		t.Fatal(err)
	}
	if output, code := run(filled, nil, engine, "internal", "covenant", "evidence", "--root", filled); code == 0 || !strings.Contains(output, "symlink") {
		t.Fatalf("the evidence gate accepted or misnamed a dangling covenant symlink (%d):\n%s", code, output)
	}
	if err := os.Remove(covenantPath); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(covenantPath, covenantBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	// A drifted Claude profile is named by the registration audits.
	profile := filepath.Join(filled, ".claude", "agents", "verify.md")
	profileBytes, err := os.ReadFile(profile)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(profile, append(append([]byte(nil), profileBytes...), []byte("drift\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	refusedRegistration(filled, "claude", false, "a drifted claude profile", "changed profile .claude/agents/verify.md")
	if err := os.WriteFile(profile, profileBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	// A source-delivery target whose go.mod vanished fails its proof.
	gomod := filepath.Join(filled, "go.mod")
	if err := os.Rename(gomod, gomod+".hidden"); err != nil {
		t.Fatal(err)
	}
	output, code = run(filled, nil, engine, "test", "run", "--repo", filled, "--goal", "adoption-goal")
	if err := os.Rename(gomod+".hidden", gomod); err != nil {
		t.Fatal(err)
	}
	// The engine source is a relevant input of the target's proof: with it
	// gone the working tree is not the tree any proof covers, and the run
	// refuses instead of reading as "no engine expected".
	if code == 0 || !strings.Contains(output, "delivery candidate differs from relevant working-tree inputs") {
		t.Fatalf("a source-delivery target without go.mod proved green or misnamed its refusal (%d):\n%s", code, output)
	}
	// A registered link to a pruned skill is named as dangling.
	if err := os.RemoveAll(filepath.Join(filled, "skills", "take-a-step-back")); err != nil {
		t.Fatal(err)
	}
	refusedRegistration(filled, "claude", false, "a dangling registered skill link", "registered skill link is dangling: .claude/skills/take-a-step-back")

	// The copied target: registration setup, byte equality with the
	// source outside tailoring, and drift of each copied registration.
	copied := prepare("copied", "claude,codex", true)
	isolateFixtureAdmission()
	copiedEngine := filepath.Join(copied, "bin", "metasystem")
	if setup := mustRun(copied, copiedEngine, "internal", "runtime", "setup", "--repo", copied, "--runtimes", "claude,codex", "--copy-skills", "--check"); !strings.Contains(setup, "TEST_CONTRACT_READY") {
		t.Fatalf("copied registration setup passed without a ready testing contract:\n%s", setup)
	}
	surfacePolicy, err := behaviorsurface.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, projection := range []behaviorsurface.Projection{behaviorsurface.Engine, behaviorsurface.Payload} {
		sourceDigest, sourceErr := surfacePolicy.DigestWithPrefix(source, projection, "")
		targetDigest, targetErr := surfacePolicy.DigestWithPrefix(copied, projection, "")
		if sourceErr != nil || targetErr != nil {
			t.Fatalf("%s digest: source %v, target %v", projection, sourceErr, targetErr)
		}
		if sourceDigest != targetDigest {
			t.Fatalf("copied target changed non-tailored %s bytes", projection)
		}
	}
	if output, passed := registrationCheck(copied, "claude,codex", true); !passed {
		t.Fatalf("copied-skills registration check failed:\n%s", output)
	}
	for _, registration := range []string{".claude/skills/verify", ".agents/skills/verify"} {
		skill := filepath.Join(copied, registration, "SKILL.md")
		original, err := os.ReadFile(skill)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(skill, append(append([]byte(nil), original...), []byte("drift\n")...), 0o644); err != nil {
			t.Fatal(err)
		}
		refusedRegistration(copied, "claude,codex", true, "a drifted copy at "+registration, registration+": existing copied skill differs from its source at SKILL.md")
		if err := os.WriteFile(skill, original, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// fillAdoptionHarnessConf points evidence at the fixture sandbox and gives
// every rostered role a fixture model, so a nested validation never resolves
// a real model or writes evidence outside the fixture tree. Tier 1 then names
// exactly the fixture models (sorted, deduplicated) and every deeper tier
// empties.
func fillAdoptionHarnessConf(t *testing.T, path, evidence string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	modelKey := regexp.MustCompile(`^(role\.[a-z0-9-]+|mode\.[a-z0-9-]+\.role\.[a-z0-9-]+)\.model\.([a-z0-9-]+)$`)
	models := map[string]bool{}
	var lines []string
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		key, value, found := strings.Cut(line, "=")
		if !found {
			lines = append(lines, line)
			continue
		}
		switch {
		case key == "evidence.root":
			value = evidence
		case key == "dispatch.cap-min":
			// The target's own delivery validation reserves this many
			// minutes; its watchdog stops the suite at that deadline.
			value = "40"
		case modelKey.MatchString(key):
			runtime := modelKey.FindStringSubmatch(key)[2]
			value = "fixture-" + runtime + "-model"
			models[runtime+":"+value] = true
		case key == "model.tier.1":
			value = "\x00models"
		case strings.HasPrefix(key, "model.tier."):
			value = ""
		}
		lines = append(lines, key+"="+value)
	}
	pairs := make([]string, 0, len(models))
	for pair := range models {
		pairs = append(pairs, pair)
	}
	sort.Strings(pairs)
	text := strings.ReplaceAll(strings.Join(lines, "\n")+"\n", "model.tier.1=\x00models", "model.tier.1="+strings.Join(pairs, ","))
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// fillAdoptionHarnessTestingContract performs the application's ordinary
// reviewed tailoring of the explicitly incomplete adopted contract: the engine
// source sits at the application root rather than below a metasystem/ prefix.
// Every group and assertion of the source contract is kept.
func fillAdoptionHarnessTestingContract(t *testing.T, source, target string) {
	t.Helper()
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ReplaceAll(string(data), `"metasystem/`, `"`)
	text = strings.ReplaceAll(text, `"cwd":"metasystem"`, `"cwd":"."`)
	text = strings.ReplaceAll(text, `"cwd": "metasystem"`, `"cwd": "."`)
	if err := os.WriteFile(target, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// fillAdoptionPlaceholders replaces every <placeholder> with a filled value,
// the ordinary fixture tailoring of docs/project-rules.md.
func fillAdoptionPlaceholders(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(regexp.MustCompile(`<[^>]*>`).ReplaceAllString(string(data), "filled")), 0o644); err != nil {
		t.Fatal(err)
	}
}

// prepareFilledTargetCovenant gives the filled target a fully valid covenant,
// its canonical evidence table and present dependencies, so the green
// delivery proves the covenant evidence gate fired rather than skipped.
func prepareFilledTargetCovenant(t *testing.T, target string) {
	t.Helper()
	writeReceiptFixture(t, target, "gate.sh", "#!/usr/bin/env bash\nset -euo pipefail\nexec go run ./src\n")
	writeReceiptFixture(t, target, "src/app.go", "package main\n\nimport \"fmt\"\n\nfunc appOutput() string { return \"hello\\nmetric=greets=0\\n\" }\n\nfunc main() { fmt.Print(appOutput()) }\n")
	writeReceiptFixture(t, target, "src/app_test.go", "package main\n\nimport \"testing\"\n\nfunc TestAdoptedAppGreeting(t *testing.T) {\n\tif got, want := appOutput(), \"hello\\nmetric=greets=0\\n\"; got != want {\n\t\tt.Fatalf(\"app output = %q, want %q\", got, want)\n\t}\n}\n")
	writeReceiptFixture(t, target, "covenant.json", `{
  "schemaVersion": 1,
  "identity": {"name": "adopt-bed", "entryPoint": "bash gate.sh", "sourcePaths": ["src/"]},
  "requirements": [
    {"id": "1", "ref": "criterion 1: the app greets by name", "proof": "greets"}
  ],
  "battery": {"command": "bash gate.sh", "metric": "greets", "direction": "max", "threshold": ">=1"},
  "budgets": [],
  "guards": [],
  "guardrails": ["gate.sh", "docs/covenant-evidence.md"]
}
`)
	writeReceiptFixture(t, target, "docs/covenant-evidence.md", "# Covenant evidence — adopt-bed\n\n| criterion id | criterion | proof id | kind | exact command | repo deps | evidence source | status |\n| --- | --- | --- | --- | --- | --- | --- | --- |\n| 1 | The app greets by name | greets | repo | bash gate.sh | gate.sh,src/app.go | gate.sh runs the entrypoint | observed |\n\nWired: 1. Floating: 0.\n")
}

func adoptionAppTestingContract() testpolicy.Contract {
	const groupID = "adopted-app-go"
	return testpolicy.Contract{
		SchemaVersion: testpolicy.ExecutionContractSchemaVersion,
		ProjectRisk:   testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
		Surfaces: []testpolicy.Surface{{
			ID: "adopted-app", Paths: []string{"src/**", "testing.json", "go.mod", "go.sum"},
			DependsOn: []string{}, Standard: []string{groupID}, Deep: []string{}, Critical: []string{"adopted-app-output"},
		}},
		Groups: []testpolicy.Group{{
			ID: groupID, Phase: "acceptance", EnvironmentMode: "inherit",
			Kind: "unit", Adapter: "go", CWD: ".",
			Inputs: []string{"src/app.go", "src/app_test.go", "go.mod", "go.sum"}, Outputs: []string{},
			Tools:       []testpolicy.Tool{{ID: "go", Executable: "go", VersionArgs: []string{"version"}}},
			Obligations: []string{"adopted-app-output"}, Platforms: []string{"any"}, TargetMS: 10000,
			Packages: []string{"src"}, Tests: json.RawMessage(`["TestAdoptedAppGreeting"]`),
		}},
		Always:  testpolicy.Always{Canary: []string{groupID}, Standard: []string{groupID}},
		Unknown: []string{groupID}, Cadence: []string{groupID},
	}
}

func seedAdoptionComparisonGoal(t *testing.T, root string) {
	t.Helper()
	runReceiptGit(t, root, "config", "metasystem.goal.machine", "fixture-machine")
	runReceiptGit(t, root, "config", "goal.sync-remote", "local")
	runReceiptGit(t, root, "config", "goal.sync-branch", goal.LocalLedgerBranch)
	now := time.Now().UTC()
	budget := &goal.Budget{ElapsedLimit: "4h", AttemptLimit: 8, ReservedJobMinutesLimit: 240, ActiveJobLimit: 1, ReviewRoundLimit: 0}
	risk := &goal.RiskRecord{Severity: 1, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "The fixture validates one isolated adopted installation."}
	intent := "Validate the actual adopted delivery contract."
	g := &goal.GoalFile{Id: "adoption-goal", State: goal.StateClaimed, Tier: 1, Risk: risk, Intent: intent, Origin: goal.OriginMain, NextStep: "Run delivery.", OpenedAt: now.Add(-2 * time.Minute).Format(time.RFC3339), Revision: 3, Budget: budget,
		Claimed:        &goal.ClaimRecord{Machine: "fixture-machine", Lineage: "adoption-comparison", At: now.Add(-time.Minute).Format(time.RFC3339), Revision: 2, AccountingRevision: 2},
		Approved:       &goal.ApprovalRecord{By: "human:fixture", At: now.Add(-30 * time.Second).Format(time.RFC3339), Revision: 3, Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FAZ", "fixture-machine", "adoption-comparison"), Authority: goal.ApprovalAuthorityProven, Digest: goal.ApprovalDigest(intent, 1, *budget, risk)},
		StopCapability: &goal.StopCapability{Generation: 3, Revision: 2, Machine: "fixture-machine", ClaimEpoch: 1}}
	for i, verb := range []string{"open", "claim", "approve"} {
		actor, at := "fixture-machine+adoption-comparison", g.OpenedAt
		opid := goal.Opid(fmt.Sprintf("01ARZ3NDEKTSV4RRFFQ69G5FA%d", i), "fixture-machine", "adoption-comparison")
		if verb == "claim" {
			at = g.Claimed.At
		} else if verb == "approve" {
			actor, at, opid = "human:fixture", g.Approved.At, g.Approved.Opid
		}
		g.History = append(g.History, goal.HistoryLine{At: at, Opid: opid, Verb: verb, Actor: actor, Keep: -1})
	}
	writeReceiptFixture(t, root, "plans/goals/backlog.md", string(goal.RenderRoot(&goal.RootRecord{Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1", SyncMode: goal.SyncLocal, Revision: 1})))
	writeReceiptFixture(t, root, "plans/goals/adoption-goal.md", string(goal.RenderFile(g)))
}
