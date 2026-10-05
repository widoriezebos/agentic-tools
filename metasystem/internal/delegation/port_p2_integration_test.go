package delegation_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/adapter"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegation"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/validate"
)

// completeOnWait makes the bed's job waiter the fake runtime finishing the
// round: the waited job's record moves running -> completed and the waiter
// answers success.
func (b *bed) completeOnWait() {
	b.doubles.Host.WaitFunc = func(root, job string, _ int64) delegation.WaitOutcome {
		b.completeJob(job)
		return delegation.WaitOutcome{Code: 0}
	}
}

// completeJob concludes a running job's record as the fake runtime would:
// the canned role return in its round directory, then the terminal CAS.
func (b *bed) completeJob(job string) {
	b.t.Helper()
	roundDir := b.roundDir(job)
	if err := adapter.WriteFakeReturn(b.recordPath(job), filepath.Join(roundDir, "prompt.md"), filepath.Join(roundDir, "return.json")); err != nil {
		b.t.Fatalf("fake return for %s: %v", job, err)
	}
	patch := filepath.Join(b.t.TempDir(), "complete.json")
	if err := os.WriteFile(patch, []byte(`{"exitCode":0}`), 0o600); err != nil {
		b.t.Fatal(err)
	}
	if _, err := dispatch.RecordCAS(b.root, job, "running", "completed", patch); err != nil {
		b.t.Fatalf("complete %s: %v", job, err)
	}
}

const fixtureDesign = "plans/designs/fixture.md"

// designPage commits a design page for a design critic to review and
// returns its declared-outputs manifest.
func (b *bed) designPage() string {
	b.t.Helper()
	b.writeFile(fixtureDesign, "# Fixture design\n\nA page under review.\n")
	b.git("add", fixtureDesign)
	b.git("commit", "-qm", "fixture design page")
	return b.writeFile("declared-outputs.txt", fixtureDesign+"\n")
}

// roundDir is a job's round directory under its chain root's payload.
func (b *bed) roundDir(job string) string {
	b.t.Helper()
	record := b.record(job)
	root := job
	for parent, _ := b.record(root)["parentJob"].(string); parent != ""; parent, _ = b.record(root)["parentJob"].(string) {
		root = parent
	}
	round, _ := record["round"].(float64)
	if round < 1 {
		round = 1
	}
	return filepath.Join(b.root, "artifacts", "agents", root, "rounds", jsonInt(int64(round)))
}

func (b *bed) readFile(relative string) string {
	b.t.Helper()
	content, err := os.ReadFile(filepath.Join(b.root, relative))
	if err != nil {
		b.t.Fatal(err)
	}
	return string(content)
}

// Fixture leg_happy (lines 1683-1696): a design-critic dispatch with --wait
// completes, status answers completed, and the record keeps the reviewed
// design path.
func TestPortP2DispatchIntegrationDesignCriticHappyWaitCompletes(t *testing.T) {
	t.Parallel()
	b := newDispatchBed(t)
	b.completeOnWait()
	brief := b.brief("happy.md", "design", "Review the design.")
	outputs := b.designPage()
	result := b.runEnv(b.dispatchEnv("fresh"), "dispatch", "--role", "design-critic", "--outputs", outputs,
		"--design", fixtureDesign, "--brief", brief, "--job-id", "happy", "--wait")
	requireExit(t, result, 0, b.stderr.String())
	// A waited dispatch answers through its exit code, not the job line.
	if len(result.Stdout) != 0 || len(b.calls("host.WaitJob job=happy")) != 1 {
		t.Fatalf("stdout %q calls %v", result.Stdout, b.calls("host."))
	}
	status := b.run("status", "--job", "happy")
	requireExit(t, status, 0, b.stderr.String())
	if string(status.Stdout) != "completed\n" {
		t.Fatalf("status %q", status.Stdout)
	}
	if design := b.record("happy")["design"]; design != fixtureDesign {
		t.Fatalf("the design-critic record lost its reviewed design path: %v", design)
	}
}

// Fixture change_design_page_after_round_one and leg_happy_follow_up (lines
// 1791-1824): after round one the reviewed design page changes; the
// follow-up round publishes a subject whose content digest is the changed
// page's, and its packet tells the round its cap and its return-by minute.
func TestPortP2FollowUpIntegrationPublishesTheChangedDesignSubject(t *testing.T) {
	t.Parallel()
	b := newDispatchBed(t)
	b.completeOnWait()
	brief := b.brief("happy.md", "design", "Review the design.")
	outputs := b.designPage()
	requireExit(t, b.runEnv(b.dispatchEnv("fresh"), "dispatch", "--role", "design-critic", "--outputs", outputs,
		"--design", fixtureDesign, "--brief", brief, "--job-id", "happy", "--wait"), 0, b.stderr.String())
	b.p5WriteJSON("artifacts/agents/happy/rounds/1/return.json", b.criticalDesignReturn("happy"))

	subject := func(round string) map[string]any {
		t.Helper()
		value, err := dispatch.ReadRecordObject(filepath.Join(b.root, "artifacts", "agents", "happy", "rounds", round, "subject.json"))
		if err != nil {
			t.Fatalf("round %s subject: %v", round, err)
		}
		return value
	}
	first := subject("1")
	if first["designPath"] != fixtureDesign || first["contentDigest"] == "" {
		t.Fatalf("round one subject %v", first)
	}
	page := filepath.Join(b.root, fixtureDesign)
	content, err := os.ReadFile(page)
	if err != nil {
		t.Fatal(err)
	}
	changed := append(content, []byte("\nFixture design revision for happy-follow-up.\n")...)
	if err := os.WriteFile(page, changed, 0o644); err != nil {
		t.Fatal(err)
	}
	digest := p2SHA256Hex(changed)
	if digest == first["contentDigest"] {
		t.Fatal("the fixture did not change the reviewed design page")
	}

	message := b.writeFile("follow.md", "Please address the findings.\n")
	result := b.runEnv(b.dispatchEnv("follow-up"), "follow-up", "--job", "happy", "--message", message, "--wait")
	requireExit(t, result, 0, b.stderr.String())
	if got := subject("2")["contentDigest"]; got != digest {
		t.Fatalf("the follow-up published subject digest %v, want the changed page's %s", got, digest)
	}
	prompt := b.readFile("artifacts/agents/happy/rounds/2/prompt.md")
	match := returnByLine.FindStringSubmatch(prompt)
	if match == nil {
		t.Fatalf("the follow-up packet carries no return-by line:\n%s", prompt)
	}
	capMin, _ := strconv.Atoi(match[1])
	returnBy, _ := strconv.Atoi(match[2])
	if capMin < 1 || returnBy >= capMin {
		t.Fatalf("return-by minute %d is not before the cap %d", returnBy, capMin)
	}
}

var returnByLine = regexp.MustCompile(`Your round is capped at (\d+) minutes from its reservation; write your return by minute (\d+), naming what is left\.`)

func p2SHA256Hex(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

// Fixture HCL-09 in leg_review_target_flag_runtime (lines 1766-1789): a
// landed commit is a first-class code-critic subject. The dispatcher binds
// the exact reviews value, freezes the parent-to-commit patch and the exact
// tree without an implementer job, and a second critic chain may review the
// same commit.
func TestPortP2DispatchIntegrationCodeCriticReviewsACommit(t *testing.T) {
	t.Parallel()
	b := newDispatchBed(t)
	b.completeOnWait()
	b.writeFile("metasystem/commit-subject.txt", "commit subject fixture\n")
	b.git("add", "--", "metasystem/commit-subject.txt")
	b.git("commit", "-qm", "commit subject fixture")
	commit := b.git("rev-parse", "HEAD")
	tree := b.git("rev-parse", commit+"^{tree}")
	expectedPatch := b.gitRaw("diff", "--binary", "--full-index", commit+"^", commit)
	brief := b.brief("code.md", "implement", "Review the commit.")

	for _, job := range []string{"commit-subject", "commit-subject-two"} {
		result := b.runEnv(b.dispatchEnv("fresh"), "dispatch", "--role", "code-critic", "--brief", brief,
			"--reviews", "commit:"+commit, "--runtime", "fake", "--job-id", job, "--wait")
		requireExit(t, result, 0, b.stderr.String())
		if reviews := b.record(job)["reviews"]; reviews != "commit:"+commit {
			t.Fatalf("%s lost its exact reviews binding: %v", job, reviews)
		}
		round := filepath.Join("artifacts", "agents", job, "rounds", "1")
		if got := strings.TrimSpace(b.readFile(filepath.Join(round, "reviewedTree"))); got != tree {
			t.Fatalf("%s froze tree %q, want %q", job, got, tree)
		}
		if got := b.readFile(filepath.Join(round, "diff.patch")); got != expectedPatch {
			t.Fatalf("%s diff is not the exact parent-to-commit patch:\n%s\nwant\n%s", job, got, expectedPatch)
		}
		if b.record(job)["status"] != "completed" {
			t.Fatalf("%s did not complete: %v", job, b.record(job))
		}
	}
}

// gitRaw is git's exact standard output in the bed's repository.
func (b *bed) gitRaw(args ...string) string {
	b.t.Helper()
	command := exec.Command("git", append([]string{"-C", b.root}, args...)...)
	output, err := command.Output()
	if err != nil {
		b.t.Fatalf("git %v: %v", args, err)
	}
	return string(output)
}

// Fixture leg_review_target_flag_runtime (lines 1716-1764): an implementer
// dispatched into its own worktree keeps a writable envelope rooted there;
// a code critic reviewing it with an explicit --runtime records the
// override, denies network without an explicit permissions key (the
// implementer keeps network), binds its reviews target, and derives the
// independent-critique reference onto the reviewed chain root.
func TestPortP2DispatchIntegrationFlagRuntimeCriticReviewsAWorktreeImplementer(t *testing.T) {
	t.Parallel()
	b := newDispatchBed(t)
	b.completeOnWait()
	implementerBrief := b.brief("review-target.md", "implement", "Change the review target.")
	requireExit(t, b.runEnv(b.dispatchEnv("fresh"), "dispatch", "--role", "implementer", "--brief", implementerBrief,
		"--job-id", "review-target", "--worktree", "--wait"), 0, b.stderr.String())
	target := b.record("review-target")
	workspace, _ := target["workspaceRoot"].(string)
	if target["launchMode"] != "worktree" || !strings.HasPrefix(workspace, filepath.Join(b.root, "artifacts", "agents", "worktrees")) {
		t.Fatalf("the implementer did not run in its own worktree: %v", target)
	}
	// The lifecycle expands the requested envelope; the runtime adapter
	// materializes the effective one at its handshake (adapter-owned).
	requested, _ := nested(target, "permissions", "requested").(map[string]any)
	writeRoots, _ := json.Marshal(requested["writeRoots"])
	if !strings.Contains(string(writeRoots), `"`+workspace+`"`) || requested["tools"] != "runtime-default" {
		t.Fatalf("the implementer role lost its writable worktree envelope: %v", target["permissions"])
	}

	// Give the implementer a real, declared change and persist its
	// conformance review before a critic reads the subject.
	if err := os.MkdirAll(filepath.Join(workspace, "metasystem"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "metasystem", "review-target.txt"), []byte("review target subject\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	returnPath := filepath.Join(b.root, "artifacts", "agents", "review-target", "rounds", "1", "return.json")
	returned, err := dispatch.ReadRecordObject(returnPath)
	if err != nil {
		t.Fatal(err)
	}
	returned["diffBoundary"] = []string{"metasystem/review-target.txt"}
	encoded, _ := json.Marshal(returned)
	if err := os.WriteFile(returnPath, append(encoded, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, errs, code := validate.Conformance(b.root, "review", "review-target"); code != 0 {
		t.Fatalf("conformance review exit %d: %v %v", code, out, errs)
	}
	// The artifact namespace is nested although the adoption is at its Git
	// top level: the persisted diff keeps the nested path.
	if patch := b.readFile("artifacts/agents/review-target/rounds/1/diff.patch"); !strings.Contains(patch,
		"diff --git a/metasystem/review-target.txt b/metasystem/review-target.txt") {
		t.Fatalf("the review-target conformance diff lost the nested path:\n%s", patch)
	}

	codeBrief := b.brief("code.md", "implement", "Review the implementation.")
	result := b.runEnv(b.dispatchEnv("fresh"), "dispatch", "--role", "code-critic", "--brief", codeBrief,
		"--reviews", "review-target", "--runtime", "fake", "--job-id", "flag-runtime", "--wait")
	requireExit(t, result, 0, b.stderr.String())
	critic := b.record("flag-runtime")
	if critic["runtime"] != "fake" || critic["overridden"] != true {
		t.Fatalf("the flag runtime override was not recorded as overridden fake: %v", critic)
	}
	if network := nested(critic, "permissions", "requested", "network"); network != "deny" {
		t.Fatalf("a code critic without an explicit permissions key did not deny network: %v", network)
	}
	target = b.record("review-target")
	if network := nested(target, "permissions", "requested", "network"); network != "allow" {
		t.Fatalf("the implementer did not retain network access: %v", network)
	}
	if critic["reviews"] != "review-target" {
		t.Fatalf("the critic record lost its reviews binding: %v", critic["reviews"])
	}
	if target["independentCritiqueJobRef"] != "flag-runtime" {
		t.Fatalf("the critic claim did not derive its reference onto the reviewed chain root: %v", target)
	}
}

func nested(object map[string]any, path ...string) any {
	var value any = object
	for _, key := range path {
		current, ok := value.(map[string]any)
		if !ok {
			return nil
		}
		value = current[key]
	}
	return value
}

// Fixture leg_default_role (lines 1698-1708) and the structured-budget
// custody checks (lines 1538-1541): a verifier with no role entry falls back
// to role.default.runtime, dispatches with the zero-write preset into the
// shared checkout, and completes. Fixture structured-budget-opid-mismatch
// (lines 1545-1557): the same operation id with a changed brief refuses as
// REFUSED-OPID-MISMATCH and leaves the standing record untouched.
func TestPortP2DispatchIntegrationDefaultRoleVerifierAndOpidMismatch(t *testing.T) {
	t.Parallel()
	b := newDispatchBed(t)
	b.completeOnWait()
	brief := b.brief("verifier.md", "verify", "Verify the thing.")
	requireExit(t, b.runEnv(b.dispatchEnv("fresh"), "dispatch", "--role", "verifier", "--brief", brief,
		"--permissions", "none", "--job-id", "default-role", "--wait"), 0, b.stderr.String())
	record := b.record("default-role")
	if record["status"] != "completed" || record["runtime"] != "fake" || record["launchMode"] != "shared-checkout" ||
		record["workspaceRoot"] != b.root {
		t.Fatalf("the default-role verifier did not complete in shared-checkout custody: %v", record)
	}

	changed := b.brief("verifier-changed.md", "verify", "Verify the thing.", "This changes the retry fingerprint.")
	launches := len(b.launches)
	before := b.readFile("artifacts/agents/jobs/default-role.json")
	result := b.runEnv(b.dispatchEnv("fresh"), "dispatch", "--role", "verifier", "--brief", changed,
		"--permissions", "none", "--job-id", "default-role")
	if result.ExitCode == 0 {
		t.Fatalf("a changed brief under a standing operation id was admitted: %q", result.Stdout)
	}
	if outcome := outcomeOf(t, result); outcome["outcome"] != "REFUSED-OPID-MISMATCH" {
		t.Fatalf("outcome %v stderr %q", outcome, b.stderr.String())
	}
	if !strings.Contains(string(result.Stdout), `"outcome":"REFUSED-OPID-MISMATCH"`) {
		t.Fatalf("the refusal was not printed: %q", result.Stdout)
	}
	if b.readFile("artifacts/agents/jobs/default-role.json") != before || len(b.launches) != launches {
		t.Fatal("the refused retry touched the standing record or launched")
	}
}
