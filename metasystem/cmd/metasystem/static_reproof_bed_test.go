package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
)

// Ports of scripts/agents/static-reproof-fixtures.sh (IL-28): the landing
// boundary re-proves the static checks via `go run ./cmd/devgate static` before any
// commit concludes. The shape test reads commit.sh; the Git adapter tests
// drive the unchanged commit.sh (see commit_wrapper_bed_helpers_test.go).

// staticReproofEscape is the environment-escape scan of the shape leg.
var staticReproofEscape = regexp.MustCompile(`METASYSTEM[A-Z_]*(SKIP|FAST|REPROOF)`)

func TestStaticReproofCommitWrapperShape(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(filepath.Join(wrapperBedSource(t), "scripts", "agents", "commit.sh"))
	if err != nil {
		t.Fatal(err)
	}
	wrapper := string(data)
	lines := strings.Split(wrapper, "\n")
	firstLine := func(needle string) int {
		for number, line := range lines {
			if strings.Contains(line, needle) {
				return number + 1
			}
		}
		return 0
	}

	stanza := firstLine("IL-28 static re-proof")
	if stanza == 0 {
		t.Fatal("commit.sh lost its IL-28 stanza")
	}
	if tail := strings.Join(lines[stanza-1:], "\n"); !strings.Contains(tail, "run ./cmd/devgate static") {
		t.Fatal("the boundary does not invoke devgate static")
	}
	if !strings.Contains(wrapper, "devgate static --proof-out") {
		t.Fatal("the boundary's gate call lost its side-effect-free --proof-out")
	}
	gate, commit := firstLine("run ./cmd/devgate static"), firstLine(`git -C "$root" commit "${commit_trailers[@]}"`)
	if gate == 0 || commit == 0 || gate >= commit {
		t.Fatalf("the re-proof (line %d) does not precede the commit (line %d)", gate, commit)
	}
	if !strings.Contains(wrapper, "final commit message did not contain exactly one byte-exact Goal-Item stamped by --goal") {
		t.Fatal("the wrapper lost its final Goal-Item postcondition")
	}
	if match := staticReproofEscape.FindString(wrapper); match != "" {
		t.Fatalf("an environment escape appeared in the wrapper: %s", match)
	}

	// Mutation: a guard placed BEFORE the IL-28 marker must still be caught;
	// the scan reads the whole wrapper, never just the stanza.
	mutated := strings.Join(lines[:5], "\n") + "\n" +
		`[[ -n "${METASYSTEM_SKIP_REPROOF:-}" ]] && exec git commit "$@"` + "\n" +
		strings.Join(lines[5:], "\n")
	if !staticReproofEscape.MatchString(mutated) {
		t.Fatal("the escape scan missed a pre-marker environment guard")
	}
}

const staticReproofProvenance = "none change=0000000000000000000000000000000000000000000000000000000000000000"

// staticReproofLiveEngine is the stub live engine of the gate-coupling legs;
// weightRefusal adds the stale weight refusal and hands the transport mirror
// to the real engine.
func staticReproofLiveEngine(t *testing.T, weightRefusal bool) string {
	t.Helper()
	extra := ""
	if weightRefusal {
		extra = `  "gate weight-add") echo "stub weight refusal" >&2; exit 1 ;;
  "internal landing") ` + wrapperBedEngineExec(t) + ` ;;
`
	}
	return `#!/usr/bin/env bash
case "$1 $2" in
  "lease require-holder") echo '{}' ;;
  "proc started-at") echo 1 ;;
  "util token-hex") echo cafecafecafecafecafecafecafecafe ;;
  "lease commit-token") : ;;
  "json get")
    case " $* " in
      *" --field provenance "*) echo "` + staticReproofProvenance + `" ;;
      *" --field verdictTrailer "*) echo "would-refuse code=missing-declaration" ;;
      *" --field code "*) echo "missing-declaration" ;;
      *" --field mode "*) echo "refuse" ;;
    esac ;;
  "behavior-surface select")
    while IFS= read -r -d '' path; do
      case "$path" in artifacts/*|bin/*|plans/goals/*|plans/goals.md|plans/goals-accepted.json|memory/receipts.log|records/narrator-digest.log|metasystem.conf.local) ;;
        *) printf '%s\0' "$path" ;;
      esac
    done ;;
` + extra + `  *) : ;;
esac
exit 0
`
}

// staticReproofProofEngine is the stub proof-built engine: it marks its use,
// refuses the weight verb, passes the audit unless STATIC_REPROOF_AUDIT_RED is
// set, and answers the landing observation unless the evaluator is failed.
const staticReproofProofEngine = `#!/usr/bin/env bash
set -euo pipefail
[[ -z "${STATIC_REPROOF_POLICY_ENGINE_MARKER:-}" ]] || : >"$STATIC_REPROOF_POLICY_ENGINE_MARKER"
case "$1 $2" in
  "behavior-surface select")
    while IFS= read -r -d '' path; do
      case "$path" in artifacts/*|bin/*|plans/goals/*|plans/goals.md|plans/goals-accepted.json|memory/receipts.log|records/narrator-digest.log|metasystem.conf.local) ;;
        *) printf '%s\0' "$path" ;;
      esac
    done ;;
  "gate weight-add") echo "proof-engine weight refusal" >&2; exit 1 ;;
  "internal audit") [[ -z "${STATIC_REPROOF_AUDIT_RED:-}" ]] || exit 1 ;;
  "landing observe")
    if [[ -n "${STATIC_REPROOF_EVALUATOR_FAIL:-}" ]]; then
      exit 72
    else
      echo '{"mode":"refuse","code":"missing-declaration","provenance":"` + staticReproofProvenance + `","verdictTrailer":"would-refuse code=missing-declaration"}'
    fi ;;
  *) : ;;
esac
`

func staticReproofSHA(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// TestStaticReproofCommitWrapperGitAdapterGateCoupling ports the bed's legs
// 2-4 and 6-11 on a stubbed boundary: the copied wrapper resolves its engine
// and gate inside the fixture root, a stub engine answers the lease verbs and
// a stub gate flips the verdict. The legs share one repository in order.
func TestStaticReproofCommitWrapperGitAdapterGateCoupling(t *testing.T) {
	t.Parallel()
	env := wrapperBedEnvironment(t)
	scratch := wrapperBedDir(t)
	marker := filepath.Join(scratch, "proof-engine-used")
	env = append(env, "STATIC_REPROOF_POLICY_ENGINE_MARKER="+marker)
	root := filepath.Join(wrapperBedDir(t), "metasystem")
	wrapper := filepath.Join(root, "scripts", "agents", "commit.sh")
	engine := filepath.Join(root, "bin", "metasystem")
	gate := filepath.Join(root, "scripts", "agents", "devgate-static.sh")
	if err := os.MkdirAll(filepath.Join(root, "artifacts", "agents", "mains"), 0o755); err != nil {
		t.Fatal(err)
	}
	wrapperBedCopy(t, "scripts/agents/commit.sh", wrapper, 0o755)
	wrapperBedWrite(t, engine, staticReproofLiveEngine(t, false), 0o755)
	wrapperBedWrite(t, filepath.Join(root, "scripts", "agents", "proof-engine.sh"), staticReproofProofEngine, 0o755)
	// The boundary runs `go run ./cmd/devgate static`; the fixture's devgate
	// runs the stub scripts/agents/devgate-static.sh, tracked at the seed.
	writeFixtureDevgate(t, root)
	git := func(args ...string) string { t.Helper(); return wrapperBedGit(t, env, root, args...) }
	run := func(extra []string, args ...string) (string, error) {
		return wrapperBedRun(env, root, extra, wrapper, args...)
	}
	lands := func(leg string, extra []string, args ...string) {
		t.Helper()
		if output, err := run(extra, args...); err != nil {
			t.Fatalf("%s: a lawful landing was refused: %v\n%s", leg, err, output)
		}
	}
	refuses := func(leg string, extra []string, args ...string) string {
		t.Helper()
		output, err := run(extra, args...)
		if err == nil {
			t.Fatalf("%s: the commit was not refused:\n%s", leg, output)
		}
		return output
	}
	subject := func() string { t.Helper(); return git("log", "-1", "--format=%s") }
	message := func() string { t.Helper(); return git("log", "-1", "--format=%B") }

	git("init", "-q", "-b", "main")
	git("config", "metasystem.goal.machine", "fixture-machine")
	git("config", "user.name", "fixture")
	git("config", "user.email", "fixture@example.invalid")
	wrapperBedWrite(t, filepath.Join(root, "README"), "seed\n", 0o644)
	// The scaffold is tracked at the seed: the boundary's own input closure
	// refuses untracked files under scripts/.
	wrapperBedWrite(t, filepath.Join(root, ".gitignore"), "bin/\nartifacts/\n", 0o644)
	git("add", "-A")
	git("commit", "-qm", "seed")
	wrapperBedWrite(t, filepath.Join(root, "README"), "change\n", 0o644)
	git("add", "README")

	// Leg 2: a red fast gate refuses the commit and names the re-proof. The
	// stub is staged each time it changes.
	wrapperBedWrite(t, gate, "#!/usr/bin/env bash\nexit 1\n", 0o755)
	git("add", "scripts/agents/devgate-static.sh")
	output := refuses("red gate", nil, "__lease-held", "human", "-m", "must refuse")
	wrapperBedContainsAll(t, "red gate", output, "static re-proof failed")
	if _, err := wrapperBedGitResult(env, root, "diff", "--cached", "--quiet"); err == nil {
		t.Fatal("red gate: the refused commit concluded anyway")
	}

	// Leg 3: a green fast gate lets the commit conclude with the observer's
	// provenance, through the proof-built policy engine.
	wrapperBedWrite(t, gate, wrapperBedGoGateCopying(`"$(cd "$(dirname "$0")" && pwd -P)/proof-engine.sh"`), 0o755)
	git("add", "scripts/agents/devgate-static.sh")
	lands("green gate", nil, "__lease-held", "human", "-q", "-m", "concludes green")
	if got := subject(); got != "concludes green" {
		t.Fatalf("green gate: the commit did not land: %q", got)
	}
	wrapperBedContainsAll(t, "green gate", message(),
		"Landing-Provenance: "+staticReproofProvenance,
		"Landing-Provenance-Verdict: would-refuse code=missing-declaration")
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("green gate: the proof-built policy engine was not exercised: %v", err)
	}

	// Leg 3a: a failed evaluator refuses a managed agent commit honestly and
	// leaves the human path sovereign with a fallback stamp.
	wrapperBedAppend(t, filepath.Join(root, "README"), "evaluator unavailable\n")
	git("add", "README")
	before := git("rev-parse", "HEAD")
	output = refuses("evaluator failure", []string{"METASYSTEM_OWNER_LINEAGE=fixture-lineage", "STATIC_REPROOF_EVALUATOR_FAIL=1"},
		"__lease-held", "1", "-m", "must refuse unavailable evaluator")
	wrapperBedContainsAll(t, "evaluator failure", output, "landing evaluator failed",
		"would-refuse code=evaluator-unavailable", "README",
		"restore or rebuild the proof-built landing evaluator", "--chain <root-job-id>",
		"fix the Change-Class classification")
	if got := git("rev-parse", "HEAD"); got != before {
		t.Fatal("evaluator failure: an agent commit was created")
	}
	lands("evaluator failure human", []string{"STATIC_REPROOF_EVALUATOR_FAIL=1"},
		"__lease-held", "human", "-q", "-m", "human remains sovereign over evaluator failure")
	wrapperBedContainsAll(t, "evaluator failure human", message(), "Landing-Provenance-Verdict: would-refuse code=evaluator-unavailable")

	// Weight bookkeeping is non-fatal: both the proof-built engine and a
	// stale live engine refuse the weight verb.
	wrapperBedWrite(t, engine, staticReproofLiveEngine(t, true), 0o755)
	wrapperBedAppend(t, filepath.Join(root, "README"), "weight-refused\n")
	git("add", "README")
	lands("weight refusal", nil, "__lease-held", "human", "-q", "-m", "concludes despite weight refusal")
	if got := subject(); got != "concludes despite weight refusal" {
		t.Fatalf("weight refusal: the commit did not land: %q", got)
	}

	// Leg 4: a gate input differing between index and worktree refuses.
	red := filepath.Join(root, "internal", "red")
	wrapperBedWrite(t, filepath.Join(red, "red.go"), "package red\n", 0o644)
	git("add", "internal/red/red.go")
	wrapperBedWrite(t, filepath.Join(red, "red.go"), "package repaired\n", 0o644)
	output = refuses("diverged input", nil, "__lease-held", "human", "-m", "must refuse divergence")
	wrapperBedContainsAll(t, "diverged input", output, "not what the commit would record")
	git("add", "internal/red/red.go")
	lands("converged input", nil, "__lease-held", "human", "-q", "-m", "converged concludes")

	// Leg 6: an untracked gate input refuses by name.
	stray := filepath.Join(red, "stray.go")
	wrapperBedWrite(t, stray, "package stray\n", 0o644)
	wrapperBedAppend(t, filepath.Join(root, "README"), "tick\n")
	git("add", "README")
	output = refuses("untracked input", nil, "__lease-held", "human", "-m", "must refuse stray")
	wrapperBedContainsAll(t, "untracked input", output, "stray.go")
	if err := os.Remove(stray); err != nil {
		t.Fatal(err)
	}

	// Leg 7: a pathspec commit lands a tree the proof never judged; the
	// wrapper rolls it back softly and keeps the exact proved index.
	wrapperBedWrite(t, filepath.Join(red, "alpha.txt"), "alpha\n", 0o644)
	wrapperBedWrite(t, filepath.Join(red, "beta.txt"), "beta\n", 0o644)
	git("add", "internal/red/alpha.txt", "internal/red/beta.txt")
	before = git("rev-parse", "HEAD")
	stagedTree := git("write-tree")
	output = refuses("pathspec", nil, "__lease-held", "human", "-m", "must roll back", "internal/red/alpha.txt")
	wrapperBedContainsAll(t, "pathspec", output, "never judged")
	if got := git("rev-parse", "HEAD"); got != before {
		t.Fatal("pathspec: the commit was not rolled back")
	}
	if got := git("write-tree"); got != stagedTree {
		t.Fatal("pathspec: the rollback did not preserve the proved index tree")
	}
	engineBefore := staticReproofSHA(t, engine)
	lands("plain message", nil, "__lease-held", "human", "-q", "-m", "plain message concludes")
	if got := git("rev-parse", "HEAD^{tree}"); got != stagedTree {
		t.Fatal("plain message: the concluding commit did not record the proved tree")
	}
	if staticReproofSHA(t, engine) != engineBefore {
		t.Fatal("plain message: the landing boundary rewrote bin/metasystem")
	}

	// Leg 8b: --push lands on both declared remotes; the transport mirror
	// transports origin's true branch head, never a tag or a stale ref.
	origin := filepath.Join(wrapperBedDir(t), "origin.git")
	transport := filepath.Join(wrapperBedDir(t), "transport.git")
	for _, bare := range []string{origin, transport} {
		wrapperBedGit(t, env, filepath.Dir(bare), "init", "-q", "--bare", "-b", "main", bare)
	}
	git("remote", "add", "origin", origin)
	git("remote", "add", "transport", transport)
	git("push", "-q", "origin", "refs/heads/main:refs/heads/main")
	git("push", "-q", "transport", "refs/heads/main:refs/heads/main")
	wrapperBedWrite(t, filepath.Join(red, "landed.txt"), "landed\n", 0o644)
	git("add", "internal/red/landed.txt")
	lands("push", nil, "__lease-held", "human", "--push", "-q", "-m", "push lands both remotes")
	pushed := git("rev-parse", "HEAD")
	if wrapperBedGit(t, env, origin, "rev-parse", "main") != pushed || wrapperBedGit(t, env, transport, "rev-parse", "main") != pushed {
		t.Fatal("push: --push did not land both remotes")
	}
	wrapperBedWrite(t, filepath.Join(red, "origin-advances.txt"), "origin advances\n", 0o644)
	git("add", "internal/red/origin-advances.txt")
	git("commit", "-qm", "origin advances beyond its mirrors")
	trueHead := git("rev-parse", "HEAD")
	git("push", "-q", "origin", "refs/heads/main:refs/heads/main")
	wrapperBedGit(t, env, origin, "update-ref", "refs/tags/main", pushed)
	stale := git("rev-parse", pushed+"^")
	git("update-ref", "refs/remotes/origin/main", stale)
	if trueHead == pushed || stale == pushed ||
		wrapperBedGit(t, env, origin, "rev-parse", "refs/heads/main") != trueHead ||
		wrapperBedGit(t, env, origin, "rev-parse", "refs/tags/main") != pushed ||
		wrapperBedGit(t, env, transport, "rev-parse", "refs/heads/main") != pushed {
		t.Fatal("transport: the tag-plus-stale-ref bed is not discriminating")
	}
	var stdout, stderr bytes.Buffer
	if code := runLandingSyncTransportWith([]string{"--root", root}, &stdout, &stderr, nil); code != 0 {
		t.Fatalf("transport: the mirror refused a lawful sync: %d\n%s", code, stderr.String())
	}
	if git("rev-parse", "refs/remotes/origin/main") != trueHead {
		t.Fatal("transport: the mirror did not fetch origin's true branch head")
	}
	if wrapperBedGit(t, env, transport, "rev-parse", "refs/heads/main") != trueHead {
		t.Fatal("transport: the mirror pushed something other than origin's branch head")
	}
	git("remote", "remove", "origin")
	git("remote", "remove", "transport")

	// Leg 8: a red audit refuses by its own exact message.
	wrapperBedWrite(t, filepath.Join(red, "alpha.txt"), "gamma\n", 0o644)
	git("add", "internal/red/alpha.txt")
	output = refuses("red audit", []string{"STATIC_REPROOF_AUDIT_RED=1"}, "__lease-held", "human", "-m", "must refuse audit")
	wrapperBedContainsAll(t, "red audit", output, "the static re-proof failed (internal audit metasystem)")

	// Leg 9: an ignored gate input refuses by name.
	wrapperBedAppend(t, filepath.Join(root, ".gitignore"), "internal/red/generated.go\n")
	git("add", ".gitignore")
	generated := filepath.Join(red, "generated.go")
	wrapperBedWrite(t, generated, "package red\n", 0o644)
	output = refuses("ignored input", nil, "__lease-held", "human", "-m", "must refuse ignored")
	wrapperBedContainsAll(t, "ignored input", output, "generated.go")
	if err := os.Remove(generated); err != nil {
		t.Fatal(err)
	}
	lands("converged tail", nil, "__lease-held", "human", "-q", "-m", "audit and stage converge")

	// Leg 10: the landing names the enrolled machine.
	machine := ""
	for _, line := range strings.Split(message(), "\n") {
		if strings.HasPrefix(line, "Machine: ") {
			machine = line
			break
		}
	}
	if !strings.HasPrefix(machine, "Machine: fixture-machine+") {
		t.Fatalf("the landing's Machine trailer is not the enrolled nickname: %q", machine)
	}

	// Leg 11: a staged symlink replacing a projected directory refuses like a
	// symlink at a critical leaf.
	directoryTarget := filepath.Join(scratch, "projected-directory-target")
	subdirectoryTarget := filepath.Join(scratch, "projected-subdirectory-target")
	wrapperBedWrite(t, filepath.Join(directoryTarget, "project-rules.md"), "external proof input\n", 0o644)
	wrapperBedWrite(t, filepath.Join(subdirectoryTarget, "weight.go"), "package gaterun\n", 0o644)
	if err := os.Symlink(directoryTarget, filepath.Join(root, "docs")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(subdirectoryTarget, filepath.Join(root, "internal", "gaterun")); err != nil {
		t.Fatal(err)
	}
	git("add", "docs", "internal/gaterun")
	output = refuses("directory symlink", nil, "__lease-held", "human", "-m", "must refuse directory symlink")
	wrapperBedContainsAll(t, "directory symlink", output, "docs", "internal/gaterun")
}

// TestStaticReproofCommitWrapperGitAdapterStampsRealObservation ports
// TestRealCommitWrapperStampsParseableObservation: the real evaluator's
// observation reaches the commit, and the evaluator's refusals reach a goal-
// bound agent commit with their repair text while a human stays sovereign.
func TestStaticReproofCommitWrapperGitAdapterStampsRealObservation(t *testing.T) {
	t.Parallel()
	env := wrapperBedEnvironment(t)
	scratch := wrapperBedDir(t)
	proofEngine := filepath.Join(scratch, "real-observer-proof-engine")
	wrapperBedWrite(t, proofEngine, wrapperBedRealProofEngine(t), 0o755)
	liveEngine := `#!/usr/bin/env bash
case "$1 ${2:-}" in
  "lease require-holder") echo '{}' ;;
  "proc started-at") echo 1 ;;
  "util token-hex") echo cafecafecafecafecafecafecafecafe ;;
  "lease commit-token") : ;;
  *) ` + wrapperBedEngineExec(t) + ` ;;
esac
`
	goalFile := wrapperBedFixtureGoal("Prove that a held goal may carry its record.", "fixture-machine", "human")
	install := func(root string) {
		t.Helper()
		for _, dir := range []string{"scripts", "artifacts/agents/mains", "artifacts/agents/jobs"} {
			if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(dir)), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		wrapperBedCopy(t, "scripts/agents/commit.sh", filepath.Join(root, "scripts", "agents", "commit.sh"), 0o755)
		for _, policy := range []string{"scripts/agents/landing-classes.json", "scripts/agents/path-classes.txt", "memory/rulings.md"} {
			wrapperBedCopy(t, policy, filepath.Join(root, filepath.FromSlash(policy)), 0o644)
		}
		wrapperBedWrite(t, filepath.Join(root, "plans", "goals", "fx.md"), goalFile, 0o644)
		wrapperBedWrite(t, filepath.Join(root, "bin", "metasystem"), liveEngine, 0o755)
		wrapperBedWrite(t, filepath.Join(root, "scripts", "agents", "devgate-static.sh"), wrapperBedGoGateCopying(wrapperBedShellQuote(proofEngine)), 0o755)
		writeFixtureDevgate(t, root)
	}
	seed := func(top string) {
		t.Helper()
		wrapperBedGit(t, env, top, "init", "-q", "-b", "main")
		wrapperBedGit(t, env, top, "config", "user.name", "fixture")
		wrapperBedGit(t, env, top, "config", "user.email", "fixture@example.invalid")
		wrapperBedGit(t, env, top, "config", "metasystem.goal.machine", "fixture-machine")
	}
	commit := func(root, epoch string, args ...string) (string, error) {
		return wrapperBedRun(env, root, []string{"METASYSTEM_OWNER_LINEAGE=human"},
			filepath.Join(root, "scripts", "agents", "commit.sh"), append([]string{"__lease-held", epoch, "--goal", "fx"}, args...)...)
	}
	agentRefuses := func(leg, root string, args ...string) string {
		t.Helper()
		output, err := commit(root, "1", args...)
		if err == nil {
			t.Fatalf("%s: a goal-bound agent commit landed:\n%s", leg, output)
		}
		return output
	}
	lands := func(leg, root, epoch string, args ...string) {
		t.Helper()
		if output, err := commit(root, epoch, args...); err != nil {
			t.Fatalf("%s: the landing was refused: %v\n%s", leg, err, output)
		}
	}

	fixture := filepath.Join(wrapperBedDir(t), "real-observer")
	install(fixture)
	seed(fixture)
	git := func(args ...string) string { t.Helper(); return wrapperBedGit(t, env, fixture, args...) }
	message := func() string { t.Helper(); return git("log", "-1", "--format=%B") }
	wrapperBedWrite(t, filepath.Join(fixture, ".gitignore"), "artifacts/\n", 0o644)
	wrapperBedWrite(t, filepath.Join(fixture, "README"), "before\n", 0o644)
	git("add", "-A")
	git("commit", "-qm", "seed")
	wrapperBedWrite(t, filepath.Join(fixture, "README"), "after\n", 0o644)
	git("add", "README")

	var observation struct {
		Provenance, VerdictTrailer, Mode string
	}
	candidate := git("write-tree")
	if err := json.Unmarshal([]byte(wrapperBedEngine(t, env, "landing", "observe", "--root", fixture, "--tree", candidate)), &observation); err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^none change=[0-9a-f]{64}$`).MatchString(observation.Provenance) ||
		observation.VerdictTrailer != "would-refuse code=missing-declaration" || observation.Mode != "refuse" {
		t.Fatalf("the real evaluator returned an unexpected observation: %+v", observation)
	}
	lands("human stamp", fixture, "human", "-q", "-m", "human keeps refusing verdict")
	stamped := "\n" + message() + "\n"
	for _, want := range []string{"Landing-Provenance: " + observation.Provenance, "Landing-Provenance-Verdict: " + observation.VerdictTrailer} {
		if !strings.Contains(stamped, "\n"+want+"\n") {
			t.Fatalf("human stamp: the commit lost %q:\n%s", want, stamped)
		}
	}

	// A vendored installation names its base manifest in the unclassified
	// refusal.
	vendored := filepath.Join(wrapperBedDir(t), "vendored-observer")
	vendoredInstall := filepath.Join(vendored, "metasystem")
	install(vendoredInstall)
	seed(vendored)
	wrapperBedWrite(t, filepath.Join(vendored, ".gitignore"), "metasystem/artifacts/\n", 0o644)
	wrapperBedWrite(t, filepath.Join(vendoredInstall, "README"), "before\n", 0o644)
	wrapperBedGit(t, env, vendored, "add", "-A")
	wrapperBedGit(t, env, vendored, "commit", "-qm", "seed")
	wrapperBedWrite(t, filepath.Join(vendoredInstall, "README"), "after\n", 0o644)
	wrapperBedGit(t, env, vendored, "add", "metasystem/README")
	output := agentRefuses("unclassified", vendoredInstall, "--direct-fix", "register-carriage", "-m", "unclassified path must name its base policy")
	wrapperBedContainsAll(t, "unclassified", output, "would-refuse code=path-unclassified",
		"path README has no class in scripts/agents/path-classes.txt; no classified ancestor; add a row for README or its directory to scripts/agents/path-classes.txt")

	repairs := []string{"README", "--chain <root-job-id>", "fix the Change-Class classification"}
	wrapperBedWrite(t, filepath.Join(fixture, "README"), "missing declaration refuses agents\n", 0o644)
	git("add", "README")
	output = agentRefuses("missing declaration", fixture, "-m", "missing declaration must refuse")
	wrapperBedContainsAll(t, "missing declaration", output, append([]string{"would-refuse code=missing-declaration"}, repairs...)...)
	output = agentRefuses("orphaned revert", fixture, "--revert-of", strings.Repeat("0", 40), "-m", "orphaned revert parameter must refuse")
	wrapperBedContainsAll(t, "orphaned revert", output, append([]string{"would-refuse code=conflicting-declarations"}, repairs...)...)

	wrapperBedWrite(t, filepath.Join(fixture, "artifacts", "agents", "jobs", "fixture-chain.json"),
		`{"jobId":"fixture-chain","parentJob":null,"role":"implementer","destructiveReach":"DESIGN-BEARING","chainClosed":false}`+"\n", 0o644)
	output = agentRefuses("open chain", fixture, "--chain", "fixture-chain", "-m", "open chain must refuse agent")
	wrapperBedContainsAll(t, "open chain", output, "would-refuse code=chain-open")
	lands("open chain human", fixture, "human", "--chain", "fixture-chain", "-q", "-m", "human remains sovereign over chain-open")
	wrapperBedContainsAll(t, "open chain human", message(), "Landing-Provenance-Verdict: would-refuse code=chain-open")

	wrapperBedWrite(t, filepath.Join(fixture, "README"), "mechanical exception\n", 0o644)
	git("add", "README")
	wrapperBedWrite(t, filepath.Join(fixture, "artifacts", "agents", "jobs", "mechanical-chain.json"),
		`{"jobId":"mechanical-chain","parentJob":null,"role":"implementer","destructiveReach":"MECHANICAL","chainClosed":false}`+"\n", 0o644)
	lands("mechanical chain", fixture, "1", "--chain", "mechanical-chain", "-q", "-m", "mechanical chain exception lands")
	wrapperBedContainsAll(t, "mechanical chain", message(), "Landing-Provenance-Verdict: would-refuse code=chain-not-design-bearing")

	wrapperBedWrite(t, filepath.Join(fixture, "README"), "conflicting declarations\n", 0o644)
	git("add", "README")
	output = agentRefuses("conflicting declarations", fixture, "--chain", "fixture-chain", "--direct-fix", "exact-revert", "-m", "conflict must refuse")
	wrapperBedContainsAll(t, "conflicting declarations", output, append([]string{"would-refuse code=conflicting-declarations"}, repairs...)...)
	git("restore", "--staged", "--worktree", "README")

	floor := filepath.Join(fixture, "internal", "floor.txt")
	wrapperBedWrite(t, floor, "floor change\n", 0o644)
	git("add", "internal/floor.txt")
	output = agentRefuses("behavior floor", fixture, "--direct-fix", "register-carriage", "-m", "behavior floor must refuse")
	wrapperBedContainsAll(t, "behavior floor", output, "would-refuse code=direct-fix-floor-refused")
	git("restore", "--staged", "internal/floor.txt")
	if err := os.Remove(floor); err != nil {
		t.Fatal(err)
	}

	wrapperBedWrite(t, filepath.Join(fixture, "plans", "fx-design.md"), "held record\n", 0o644)
	git("add", "plans/fx-design.md")
	lands("held record", fixture, "1", "--direct-fix", "register-carriage", "-q", "-m", "held goal carries record")
	wrapperBedContainsAll(t, "held record", message(), "Landing-Provenance-Verdict: pass bar=b")
}

// The transport mirror's own refusals and default, on the verb with Git
// stubbed: an explicitly empty or newline-bearing branch dies by name before
// Git runs, and no argument is the one lawful default, main.
func TestLandingSyncTransportVerbRefusesUnlawfulBranchNames(t *testing.T) {
	t.Parallel()
	refuseGit := func(dir string, args ...string) (string, error) {
		t.Fatalf("an unlawful branch reached Git: %v", args)
		return "", nil
	}
	for _, branch := range []string{"", "main\nevil"} {
		var stdout, stderr bytes.Buffer
		code := runLandingSyncTransportWith([]string{"--root", "/checkout", branch}, &stdout, &stderr, landing.TransportGit(refuseGit))
		if code != 2 || !strings.Contains(stderr.String(), "is not a plain branch") {
			t.Fatalf("branch %q: code=%d stderr=%q", branch, code, stderr.String())
		}
	}
}

func TestLandingSyncTransportVerbDefaultsToMain(t *testing.T) {
	t.Parallel()
	var calls [][]string
	git := func(dir string, args ...string) (string, error) {
		if dir != "/checkout" {
			t.Fatalf("git ran in %q", dir)
		}
		calls = append(calls, append([]string(nil), args...))
		return "pushed\n", nil
	}
	var stdout, stderr bytes.Buffer
	if code := runLandingSyncTransportWith([]string{"--root", "/checkout"}, &stdout, &stderr, git); code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	if len(calls) != 2 || strings.Join(calls[0], " ") != "fetch --quiet origin +refs/heads/main:refs/remotes/origin/main" ||
		strings.Join(calls[1], " ") != "push transport refs/remotes/origin/main:refs/heads/main" || stdout.String() != "pushed\n" {
		t.Fatalf("calls=%q stdout=%q", calls, stdout.String())
	}
}
