package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

// TestIntentManualWorkDelivery submits hand-written changes from the main
// checkout as a goal's work through the real claim, goal worktree, commit,
// push, committed read, whole close, collection and publication owners; only
// the critic's model is a fixture. The in-repo brief and a background
// system record stay in the source, the source's index and files are
// unchanged, the review's decisions complete through the same public
// command, and a repeat rejoins the same version without a second commit or
// critic.
func TestIntentManualWorkDelivery(t *testing.T) {
	j := newJourneyBed(t)
	c, do := j.c, j.do
	root := c.root()
	os.MkdirAll(filepath.Join(root, "notes"), 0o700)
	os.WriteFile(filepath.Join(root, "notes", "brief.md"), []byte("Add the manual file.\n"), 0o644)
	os.MkdirAll(filepath.Join(root, "metasystem", "records"), 0o700)
	os.WriteFile(filepath.Join(root, "metasystem", "records", "narrator-digest.log"), []byte("a background record line\n"), 0o644)
	os.WriteFile(filepath.Join(root, "manual.txt"), []byte("manual work\n"), 0o644)
	indexBefore, _ := os.ReadFile(filepath.Join(root, ".git", "index"))
	statusBefore := connectionGit(t, root, "status", "--porcelain")
	submit := []string{"review", c.id, "--changes", "--brief", "notes/brief.md"}

	delegates := len(c.delegates)
	code, result := do(submit...)
	data := resultData(t, result)
	commit, _ := data["commit"].(string)
	if result.Outcome != intentInProgress || len(c.delegates) != delegates+1 || c.delegates[len(c.delegates)-1] != commit ||
		data["work"] != "main" || data["sameCheckout"] != false || jsonText(data["paths"]) != `["manual.txt"]` {
		t.Fatalf("manual submission: code=%d %+v", code, result)
	}
	left := jsonText(data["leftInSource"])
	if !strings.Contains(left, "notes/brief.md") || !strings.Contains(left, "metasystem/records/narrator-digest.log") {
		t.Fatalf("the brief and the background record must stay in the source: %s", left)
	}
	if got := connectionGit(t, root, "--git-dir", c.origin, "show", "refs/heads/goal/"+c.id+":manual.txt"); got != "manual work" {
		t.Fatalf("the published goal branch must carry the manual work: %q", got)
	}
	if names := connectionGit(t, root, "--git-dir", c.origin, "show", "--name-only", "--format=", commit); names != "manual.txt" {
		t.Fatalf("the unit commit holds exactly the captured paths: %q", names)
	}
	if message := connectionGit(t, root, "show", "-s", "--format=%B", commit); !strings.Contains(message, "Goal-Unit: "+c.id+"/main") {
		t.Fatalf("the unit commit is the goal's work main: %s", message)
	}
	indexAfter, _ := os.ReadFile(filepath.Join(root, ".git", "index"))
	if !bytes.Equal(indexBefore, indexAfter) || connectionGit(t, root, "status", "--porcelain") != statusBefore {
		t.Fatal("manual submission changed the source checkout's index or files")
	}

	critic := "crit" + strconv.Itoa(len(c.delegates))
	j.finish(c.worktree, critic, commit)
	code, result = do(submit...)
	if result.Outcome != intentInProgress || !strings.Contains(result.Decision, "review "+c.id+" --work main --dispositions FILE") ||
		len(c.delegates) != delegates+1 {
		t.Fatalf("the finished examination prints the public decision route and rejoins the version: code=%d %+v", code, result)
	}
	publications := c.publications
	code, result = do(append(slices.Clone(submit), "--dispositions", j.dispositions)...)
	if code != 0 || result.Outcome != intentConfirmed || c.publications != publications+1 || j.job(c.worktree, critic)["chainClosed"] != true ||
		result.Next == nil || !slices.Equal(result.Next.Argv[1:], []string{"land", c.id}) {
		t.Fatalf("the decisions close, collect and publish the read: code=%d %+v", code, result)
	}
	code, result = do(submit...)
	if code != 0 || (result.Outcome != intentConfirmed && result.Outcome != intentUnchanged) || resultData(t, result)["action"] != "rejoined" || len(c.delegates) != delegates+1 {
		t.Fatalf("a repeat rejoins the delivered version: code=%d %+v", code, result)
	}
	if status := c.landAdmission(); status.Prefix != 1 || len(status.Units) != 1 || status.Units[0].Commit != commit {
		t.Fatalf("landing admission must admit the manual unit by its published read: %+v", status)
	}
	code, result = do("review", c.id)
	if code != 0 || (result.Outcome != intentConfirmed && result.Outcome != intentUnchanged) || len(c.delegates) != delegates+1 {
		t.Fatalf("review G reaches the manual work's committed review without a new critic: code=%d %+v", code, result)
	}
	code, result = do("status", "goal", c.id, "--work", "main")
	if !strings.Contains(result.Summary, "collected and published") {
		t.Fatalf("status sees the manual work: code=%d %+v", code, result)
	}
	// Public land joins the manual unit into the actual batch.
	landOwners, counts := journeyLandOwners(t, j)
	var stdout, stderr bytes.Buffer
	runIntentIn(mustIntentCommand(t, "land"), []string{c.id, "--repo", root, "--json"}, &stdout, &stderr, root, landOwners)
	var landed intentResult
	if err := json.Unmarshal(stdout.Bytes(), &landed); err != nil {
		t.Fatalf("land printed no result: %v; %q %q", err, stdout.String(), stderr.String())
	}
	landData := resultData(t, landed)
	if landed.Outcome != intentInProgress || landData["route"] != "batch" || landData["joinedNow"] != true || counts.admissions != 1 {
		t.Fatalf("public land of the manual work: %+v counts %+v", landed, *counts)
	}
	record, err := batch.NewStore(j.landingRoot, identity.KernelProber{}).Load(landData["batchId"].(string))
	if err != nil || len(record.Units) != 1 || len(record.Units[0].Builds) != 1 {
		t.Fatalf("stored batch = %+v, %v", record, err)
	}
	if build := record.Units[0].Builds[0]; build.Commit != commit || build.Attestation.Source.Kind != "critic-root" || build.Attestation.Source.RootJob != critic {
		t.Fatalf("the member binds the manual version read by its closed critic: %+v", build)
	}
}

// manualDo runs one public command from cwd with the journey bed's owners.
func manualDo(t *testing.T, j *journeyBed, cwd string, args ...string) (int, intentResult) {
	t.Helper()
	command, ok := findIntentCommand(args[0])
	if !ok {
		t.Fatalf("no public command %q", args[0])
	}
	var stdout, stderr bytes.Buffer
	code := runIntentIn(command, append([]string{"--json"}, args[1:]...), &stdout, &stderr, cwd, j.owners)
	var result intentResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("%v printed no JSON result: %v; stdout=%q stderr=%q", args, err, stdout.String(), stderr.String())
	}
	return code, result
}

// TestManualSubmissionReplayAndAmend drives the real commit, push and range
// owners under the real checkout mutation lock: two work items from a
// distinct source, a lost commit response adopted from the range, a lost
// push reconciled by the repeat, a correction made in the goal worktree
// itself (source equals destination) that preserves the other work, a
// background record and refuses unrelated staging, a replayed correction
// that rejoins, stale and unnamed corrections refused, and a submission
// refused while another writer holds the checkout lock.
func TestManualSubmissionReplayAndAmend(t *testing.T) {
	j := newJourneyBed(t)
	c := j.c
	root := c.root()
	brief := filepath.Join(t.TempDir(), "brief.md")
	os.WriteFile(brief, []byte("Hand-written work.\n"), 0o600)
	os.WriteFile(filepath.Join(root, "alpha.txt"), []byte("alpha v1\n"), 0o644)
	code, result := manualDo(t, j, root, "review", c.id, "--changes", "--brief", brief, "--work", "alpha")
	alpha1, _ := resultData(t, result)["commit"].(string)
	if result.Outcome != intentInProgress || alpha1 == "" || c.commits != 1 {
		t.Fatalf("first work: code=%d %+v commits=%d", code, result, c.commits)
	}
	os.Remove(filepath.Join(root, "alpha.txt"))

	patch := filepath.Join(t.TempDir(), "beta.patch")
	os.WriteFile(patch, []byte("diff --git a/beta.txt b/beta.txt\nnew file mode 100644\n--- /dev/null\n+++ b/beta.txt\n@@ -0,0 +1 @@\n+beta\n"), 0o600)
	c.loseCommit, c.failPushes = true, 1
	code, result = manualDo(t, j, root, "review", c.id, "--patch", patch, "--brief", brief, "--work", "beta")
	data := resultData(t, result)
	beta, _ := data["commit"].(string)
	if result.Outcome != intentPartial || data["action"] != "adopted" || beta == "" || c.commits != 2 {
		t.Fatalf("a lost commit response is adopted from the range, then the lost push is reported: code=%d %+v commits=%d", code, result, c.commits)
	}
	code, result = manualDo(t, j, root, "review", c.id, "--patch", patch, "--brief", brief, "--work", "beta")
	if result.Outcome != intentInProgress || resultData(t, result)["action"] != "rejoined" || resultData(t, result)["commit"] != beta || c.commits != 2 {
		t.Fatalf("the repeat rejoins the version and publishes it: code=%d %+v commits=%d", code, result, c.commits)
	}
	if got := connectionGit(t, root, "--git-dir", c.origin, "show", "refs/heads/goal/"+c.id+":beta.txt"); got != "beta" {
		t.Fatalf("beta must be published: %q", got)
	}

	// A correction written in the goal worktree itself.
	worktree := c.worktree
	os.WriteFile(filepath.Join(worktree, "alpha.txt"), []byte("alpha v2\n"), 0o644)
	os.WriteFile(filepath.Join(worktree, "partial.txt"), []byte("staged version\n"), 0o644)
	connectionGit(t, worktree, "add", "partial.txt")
	os.WriteFile(filepath.Join(worktree, "partial.txt"), []byte("working version\n"), 0o644)
	amend := []string{"review", c.id, "--changes", "--brief", brief, "--work", "alpha", "--after", alpha1[:12], "--repo", root}
	code, result = manualDo(t, j, worktree, amend...)
	if result.Outcome != intentRefused || !strings.Contains(result.Summary, "partial.txt") || c.commits != 2 ||
		connectionGit(t, worktree, "show", ":partial.txt") != "staged version" {
		t.Fatalf("staging the submission would discard is refused and kept: code=%d %+v", code, result)
	}
	connectionGit(t, worktree, "rm", "-q", "-f", "--cached", "partial.txt")
	os.Remove(filepath.Join(worktree, "partial.txt"))
	os.MkdirAll(filepath.Join(worktree, "metasystem", "records"), 0o700)
	os.WriteFile(filepath.Join(worktree, "metasystem", "records", "narrator-digest.log"), []byte("a coordination append\n"), 0o644)
	code, result = manualDo(t, j, worktree, amend...)
	data = resultData(t, result)
	alpha2, _ := data["commit"].(string)
	if result.Outcome != intentInProgress || data["sameCheckout"] != true || alpha2 == "" || alpha2 == alpha1 || c.commits != 3 {
		t.Fatalf("the correction amends alpha: code=%d %+v commits=%d", code, result, c.commits)
	}
	if got := connectionGit(t, root, "--git-dir", c.origin, "show", "refs/heads/goal/"+c.id+":alpha.txt"); got != "alpha v2" {
		t.Fatalf("the amended alpha must be published: %q", got)
	}
	if got := connectionGit(t, root, "--git-dir", c.origin, "show", "refs/heads/goal/"+c.id+":beta.txt"); got != "beta" {
		t.Fatalf("the other work must survive the correction: %q", got)
	}
	if record, err := os.ReadFile(filepath.Join(worktree, "metasystem", "records", "narrator-digest.log")); err != nil || string(record) != "a coordination append\n" ||
		!strings.Contains(connectionGit(t, worktree, "status", "--porcelain"), "metasystem/") {
		t.Fatalf("the coordination append must stay unstaged in the goal worktree: %q %v", record, err)
	}
	code, result = manualDo(t, j, worktree, amend...)
	if resultData(t, result)["action"] != "rejoined" || resultData(t, result)["commit"] != alpha2 || c.commits != 3 {
		t.Fatalf("a replayed correction rejoins the amended version: code=%d %+v", code, result)
	}

	os.WriteFile(filepath.Join(worktree, "alpha.txt"), []byte("alpha v3\n"), 0o644)
	code, result = manualDo(t, j, worktree, amend...)
	if result.Outcome != intentRefused || !strings.Contains(result.Summary, "no longer current") || c.commits != 3 {
		t.Fatalf("a different correction of a replaced version is refused: code=%d %+v", code, result)
	}
	code, result = manualDo(t, j, worktree, "review", c.id, "--changes", "--brief", brief, "--work", "alpha", "--repo", root)
	if result.Outcome != intentRefused || !strings.Contains(result.Decision, "--after "+alpha2) || c.commits != 3 {
		t.Fatalf("an unnamed correction names the current version: code=%d %+v", code, result)
	}

	t.Setenv("METASYSTEM_LEASE_LOCK_WAIT_SEC", "0.2")
	release, err := lease.LockBounded(lease.LockPath(goalBranchHolderRoot(worktree)), "another writer")
	if err != nil {
		t.Fatal(err)
	}
	indexBefore := connectionGit(t, worktree, "diff", "--cached", "--name-only")
	code, result = manualDo(t, j, worktree, "review", c.id, "--changes", "--brief", brief, "--work", "alpha", "--after", alpha2, "--repo", root)
	release()
	if result.Outcome != intentRefused || !strings.Contains(result.Summary, "busy") || c.commits != 3 ||
		connectionGit(t, worktree, "diff", "--cached", "--name-only") != indexBefore {
		t.Fatalf("a submission while another writer holds the checkout lock is refused untouched: code=%d %+v", code, result)
	}
	code, result = manualDo(t, j, worktree, "review", c.id, "--changes", "--brief", brief, "--work", "alpha", "--after", alpha2, "--repo", root)
	if result.Outcome != intentInProgress || c.commits != 4 {
		t.Fatalf("once the lock is free the correction commits: code=%d %+v", code, result)
	}
}

// TestManualCaptureFromNestedCheckout captures from a directory nested in
// the checkout (as a nested metasystem/ installation is): changes outside
// that prefix are included, a tracked system record's append and the named
// in-repo brief stay out of the candidate, and the caller's index and files
// are unchanged. Git is the claim: the capture's private index is real.
func TestManualCaptureFromNestedCheckout(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		command := exec.Command("git", append([]string{"-C", root, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid"}, args...)...)
		command.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
		out, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q", "-b", "main")
	nested := filepath.Join(root, "metasystem", "app")
	for path, content := range map[string]string{"top.txt": "top\n", "metasystem/records/narrator-digest.log": "line one\n", "metasystem/app/code.go": "package app\n"} {
		os.MkdirAll(filepath.Dir(filepath.Join(root, path)), 0o700)
		os.WriteFile(filepath.Join(root, path), []byte(content), 0o644)
	}
	git("add", "-A")
	git("commit", "-q", "-m", "base")
	os.WriteFile(filepath.Join(root, "top.txt"), []byte("top, edited outside the nested prefix\n"), 0o644)
	os.MkdirAll(filepath.Join(root, "docs"), 0o700)
	os.WriteFile(filepath.Join(root, "docs", "new.md"), []byte("untracked\n"), 0o644)
	os.WriteFile(filepath.Join(root, "metasystem", "records", "narrator-digest.log"), []byte("line one\na background append\n"), 0o644)
	os.WriteFile(filepath.Join(nested, "brief.md"), []byte("the brief\n"), 0o644)
	os.WriteFile(filepath.Join(nested, "code.go"), []byte("package app // edited\n"), 0o644)
	git("add", "top.txt")
	index, _ := os.ReadFile(filepath.Join(root, ".git", "index"))
	status := git("status", "--porcelain")

	capture, err := captureManualAt(nested, "", filepath.Join(nested, "brief.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(capture.paths, []string{"docs/new.md", "metasystem/app/code.go", "top.txt"}) ||
		!slices.Equal(capture.omitted, []string{"metasystem/app/brief.md", "metasystem/records/narrator-digest.log"}) {
		t.Fatalf("captured %v, left %v", capture.paths, capture.omitted)
	}
	patch := string(capture.patch)
	if !strings.Contains(patch, "+top, edited outside the nested prefix") || strings.Contains(patch, "background append") || strings.Contains(patch, "the brief") {
		t.Fatalf("the candidate patch: %s", patch)
	}
	if after, _ := os.ReadFile(filepath.Join(root, ".git", "index")); !bytes.Equal(index, after) || git("status", "--porcelain") != status {
		t.Fatal("the capture changed the caller's index or files")
	}
	if explicit, err := captureManualAt(nested, filepath.Join(nested, "brief.md"), filepath.Join(nested, "brief.md")); err != nil || !explicit.explicit || string(explicit.patch) != "the brief\n" {
		t.Fatalf("a supplied patch is taken exactly: %+v %v", explicit, err)
	}
}

// TestManualStageRefusalPreservesCheckout: a distinct goal worktree refuses
// a patch that would need a three-way merge, and refuses to receive work
// while it has its own uncommitted change; in both cases its index and files
// stay byte-identical and nothing is committed.
func TestManualStageRefusalPreservesCheckout(t *testing.T) {
	j := newJourneyBed(t)
	c := j.c
	root := c.root()
	brief := filepath.Join(t.TempDir(), "brief.md")
	os.WriteFile(brief, []byte("Hand-written work.\n"), 0o600)
	os.WriteFile(filepath.Join(root, "alpha.txt"), []byte("alpha v1\n"), 0o644)
	if _, result := manualDo(t, j, root, "review", c.id, "--changes", "--brief", brief, "--work", "alpha"); result.Outcome != intentInProgress || c.commits != 1 {
		t.Fatalf("first work: %+v", result)
	}
	os.Remove(filepath.Join(root, "alpha.txt"))
	worktree := c.worktree
	snapshot := func() string {
		indexPath := connectionGit(t, worktree, "rev-parse", "--path-format=absolute", "--git-path", "index")
		index, err := os.ReadFile(indexPath)
		if err != nil {
			t.Fatal(err)
		}
		files, _ := os.ReadFile(filepath.Join(worktree, "alpha.txt"))
		return string(index) + "|" + string(files) + "|" + connectionGit(t, worktree, "status", "--porcelain") + "|" + connectionGit(t, worktree, "rev-parse", "HEAD") +
			"|" + connectionGit(t, worktree, "ls-files", "-s")
	}
	before := snapshot()
	conflicting := filepath.Join(t.TempDir(), "conflict.patch")
	os.WriteFile(conflicting, []byte("diff --git a/alpha.txt b/alpha.txt\n--- a/alpha.txt\n+++ b/alpha.txt\n@@ -1 +1 @@\n-alpha from another history\n+alpha rewritten\n"), 0o600)
	code, result := manualDo(t, j, root, "review", c.id, "--patch", conflicting, "--brief", brief, "--work", "gamma")
	if result.Outcome != intentRefused || !strings.Contains(result.Summary, "does not apply") || c.commits != 1 || snapshot() != before {
		t.Fatalf("a patch needing a three-way merge is refused untouched: code=%d %+v", code, result)
	}
	os.WriteFile(filepath.Join(worktree, "alpha.txt"), []byte("alpha, edited in the goal worktree\n"), 0o644)
	dirty := snapshot()
	good := filepath.Join(t.TempDir(), "good.patch")
	os.WriteFile(good, []byte("diff --git a/gamma.txt b/gamma.txt\nnew file mode 100644\n--- /dev/null\n+++ b/gamma.txt\n@@ -0,0 +1 @@\n+gamma\n"), 0o600)
	code, result = manualDo(t, j, root, "review", c.id, "--patch", good, "--brief", brief, "--work", "gamma")
	if result.Outcome != intentRefused || !strings.Contains(result.Summary, "uncommitted changes to alpha.txt") || c.commits != 1 || snapshot() != dirty {
		t.Fatalf("a goal worktree with its own change refuses to receive work untouched: code=%d %+v", code, result)
	}
}

// TestManualCommitRefusalRestoresStaging: a submission from the goal
// worktree itself whose commit the real branch commit owner refuses (a plan
// path is not unit work) restores the caller's staging exactly: a path the
// caller had fully staged stays staged, a path the submission staged is
// unstaged again, the files are untouched and nothing is committed.
func TestManualCommitRefusalRestoresStaging(t *testing.T) {
	j := newJourneyBed(t)
	c := j.c
	root := c.root()
	brief := filepath.Join(t.TempDir(), "brief.md")
	os.WriteFile(brief, []byte("Hand-written work.\n"), 0o600)
	os.WriteFile(filepath.Join(root, "code.go"), []byte("package code\n"), 0o644)
	os.WriteFile(filepath.Join(root, "helper.go"), []byte("package code // helper\n"), 0o644)
	if _, result := manualDo(t, j, root, "review", c.id, "--changes", "--brief", brief, "--work", "alpha"); result.Outcome != intentInProgress || c.commits != 1 {
		t.Fatalf("first work: %+v", result)
	}
	worktree := c.worktree
	os.WriteFile(filepath.Join(worktree, "code.go"), []byte("package code // pre-staged by the caller\n"), 0o644)
	connectionGit(t, worktree, "add", "code.go")
	os.WriteFile(filepath.Join(worktree, "helper.go"), []byte("package code // edited, not staged\n"), 0o644)
	os.MkdirAll(filepath.Join(worktree, "metasystem", "plans"), 0o700)
	os.WriteFile(filepath.Join(worktree, "metasystem", "plans", "note.md"), []byte("a plan is not unit work\n"), 0o644)
	state := func() string {
		index, err := os.ReadFile(connectionGit(t, worktree, "rev-parse", "--path-format=absolute", "--git-path", "index"))
		if err != nil {
			t.Fatal(err)
		}
		_ = index
		return connectionGit(t, worktree, "diff", "--cached", "--binary", "HEAD") + "|" + connectionGit(t, worktree, "ls-files", "-s") + "|" +
			connectionGit(t, worktree, "status", "--porcelain", "--untracked-files=all") + "|" + connectionGit(t, worktree, "rev-parse", "HEAD")
	}
	before := state()
	code, result := manualDo(t, j, worktree, "review", c.id, "--changes", "--brief", brief, "--work", "alpha", "--after", resultCommit(t, c), "--repo", root)
	if result.Outcome != intentRefused || !strings.Contains(result.Summary, "as it was before this submission") || c.commits != 1 {
		t.Fatalf("the commit owner's refusal: code=%d %+v", code, result)
	}
	if after := state(); after != before {
		t.Fatalf("the caller's staging changed:\nbefore %s\nafter  %s", before, after)
	}
	if staged := connectionGit(t, worktree, "diff", "--cached", "--name-only"); staged != "code.go" {
		t.Fatalf("the pre-staged path must stay staged and nothing else: %q", staged)
	}
}

// resultCommit is the goal branch's current unit commit of work alpha.
func resultCommit(t *testing.T, c *connectionBed) string {
	t.Helper()
	return connectionGit(t, c.worktree, "log", "-1", "--format=%H", "--grep", "Goal-Unit: "+c.id+"/alpha")
}

// TestManualUnknownGoalWritesNothing: a goal id the accepted ledger does not
// hold, including one shaped like a path, is refused before any input is
// frozen, so nothing is written under or beside the input custody directory.
func TestManualUnknownGoalWritesNothing(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	owners := bed.workOwners()
	brief := filepath.Join(t.TempDir(), "brief.md")
	os.WriteFile(brief, []byte("Hand-written work.\n"), 0o600)
	// A real checkout with a change, so only the goal check can stop it.
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"}, {"-c", "user.name=f", "-c", "user.email=f@example.invalid", "commit", "-qm", "base"}} {
		command := exec.Command("git", append([]string{"-C", bed.root()}, args...)...)
		command.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
		if out, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	os.WriteFile(filepath.Join(bed.root(), "change.txt"), []byte("a change\n"), 0o644)
	for _, id := range []string{"no-such-goal", "../../escape", "goal/../../escape"} {
		var stdout, stderr bytes.Buffer
		code := runIntentIn(mustIntentCommand(t, "review"), []string{"goal", id, "--changes", "--brief", brief, "--json"}, &stdout, &stderr, bed.root(), owners)
		var result intentResult
		if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
			t.Fatalf("%s: %v %q %q", id, err, stdout.String(), stderr.String())
		}
		if code == 0 || result.Outcome != intentRefused || !strings.Contains(result.Summary, "no goal") {
			t.Fatalf("goal %q: code=%d %+v", id, code, result)
		}
	}
	for _, pattern := range []string{filepath.Join(bed.root(), "artifacts", "agents", "intent-manual", "*"), filepath.Join(bed.root(), "artifacts", "agents", "escape"),
		filepath.Join(bed.root(), "artifacts", "escape"), filepath.Join(bed.root(), "escape")} {
		if found, _ := filepath.Glob(pattern); len(found) != 0 {
			t.Fatalf("an unknown goal wrote %v", found)
		}
	}
}

// TestManualPatchInGoalWorktreeAndChangedBrief: a patch supplied from inside
// the goal worktree is ordinary input, applied to its clean index and
// files, and rejoined on repeat; a patch that does not apply there leaves
// index and files as they were; and the same change resubmitted with a
// changed brief after its read is decided by the committed read owner's own
// frozen-brief binding, never reused as the earlier read.
func TestManualPatchInGoalWorktreeAndChangedBrief(t *testing.T) {
	j := newJourneyBed(t)
	c := j.c
	root := c.root()
	brief := filepath.Join(t.TempDir(), "brief.md")
	os.WriteFile(brief, []byte("Hand-written work.\n"), 0o600)
	os.WriteFile(filepath.Join(root, "alpha.txt"), []byte("alpha\n"), 0o644)
	_, first := manualDo(t, j, root, "review", c.id, "--changes", "--brief", brief, "--work", "alpha")
	alpha, _ := resultData(t, first)["commit"].(string)
	if first.Outcome != intentInProgress || c.commits != 1 {
		t.Fatalf("first work: %+v", first)
	}
	worktree := c.worktree
	patch := filepath.Join(t.TempDir(), "beta.patch")
	os.WriteFile(patch, []byte("diff --git a/beta.txt b/beta.txt\nnew file mode 100644\n--- /dev/null\n+++ b/beta.txt\n@@ -0,0 +1 @@\n+beta\n"), 0o600)
	submitBeta := []string{"review", c.id, "--patch", patch, "--brief", brief, "--work", "beta", "--repo", root}
	code, result := manualDo(t, j, worktree, submitBeta...)
	data := resultData(t, result)
	if result.Outcome != intentInProgress || data["sameCheckout"] != true || data["action"] != "committed" || c.commits != 2 {
		t.Fatalf("a patch supplied inside the goal worktree: code=%d %+v", code, result)
	}
	if code, result = manualDo(t, j, worktree, submitBeta...); resultData(t, result)["action"] != "rejoined" || c.commits != 2 {
		t.Fatalf("its repeat rejoins: code=%d %+v", code, result)
	}
	before := connectionGit(t, worktree, "status", "--porcelain", "--untracked-files=all") + connectionGit(t, worktree, "ls-files", "-s")
	conflict := filepath.Join(t.TempDir(), "conflict.patch")
	os.WriteFile(conflict, []byte("diff --git a/alpha.txt b/alpha.txt\n--- a/alpha.txt\n+++ b/alpha.txt\n@@ -1 +1 @@\n-another history\n+rewritten\n"), 0o600)
	code, result = manualDo(t, j, worktree, "review", c.id, "--patch", conflict, "--brief", brief, "--work", "gamma", "--repo", root)
	if result.Outcome != intentRefused || !strings.Contains(result.Summary, "does not apply") || c.commits != 2 ||
		connectionGit(t, worktree, "status", "--porcelain", "--untracked-files=all")+connectionGit(t, worktree, "ls-files", "-s") != before {
		t.Fatalf("a patch that does not apply leaves the goal worktree as it was: code=%d %+v", code, result)
	}

	changed := filepath.Join(t.TempDir(), "changed-brief.md")
	os.WriteFile(changed, []byte("A different brief for the same change.\n"), 0o600)
	delegates := len(c.delegates)
	code, result = manualDo(t, j, root, "review", c.id, "--changes", "--brief", changed, "--work", "alpha")
	if result.Outcome != intentRefused || !strings.Contains(result.Summary, "GOAL_READ_INVALID") || resultData(t, result)["commit"] != alpha ||
		len(c.delegates) != delegates || !strings.Contains(result.Decision, "--after "+alpha) {
		t.Fatalf("a changed brief for an already-read version is the read owner's refusal, with no new read: code=%d %+v", code, result)
	}
}

// TestIntentManualWorkLandsOnEndpoint: hand-written work goes from the main
// checkout through public submission, the independent committed review, the
// author's bound decisions and public land to the endpoint: origin main
// carries the manual file in a verified landing whose source is the manual
// unit. The critic's model and the landing proof's execution are the only
// fakes; commit, push, read, close, collection, publication, land-prep and
// land-push are the owners' own.
func TestIntentManualWorkLandsOnEndpoint(t *testing.T) {
	j := newJourneyBed(t)
	c, do := j.c, j.do
	root := c.root()
	brief := filepath.Join(t.TempDir(), "brief.md")
	os.WriteFile(brief, []byte("Add the delivered file.\n"), 0o600)
	os.WriteFile(filepath.Join(root, "delivered.txt"), []byte("hand-written and delivered\n"), 0o644)
	submit := []string{"review", c.id, "--changes", "--brief", brief}
	_, result := do(submit...)
	commit, _ := resultData(t, result)["commit"].(string)
	if result.Outcome != intentInProgress || commit == "" {
		t.Fatalf("submission: %+v", result)
	}
	critic := "crit" + strconv.Itoa(len(c.delegates))
	j.finish(c.worktree, critic, commit)
	if code, result := do(append(slices.Clone(submit), "--dispositions", j.dispositions)...); code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("decisions, close, collection and publication: code=%d %+v", code, result)
	}
	owners := j.owners
	delivery := defaultIntentDeliveryOwners()
	delivery.branchRead = owners.delivery.branchRead
	realProcess := owners.delivery.process
	var proved []string
	delivery.process = func(process intentProcess) intentProcessResult {
		if len(process.argv) > 2 && process.argv[1] == "landing" && process.argv[2] == "test-receipt" {
			tree := flagValue(process.argv, "--tree")
			proved = append(proved, tree)
			receipt, _ := json.Marshal(map[string]any{"schemaVersion": 3, "tree": tree, "exitStatus": 0,
				"time": "2026-09-26T12:00:00Z", "proof": map[string]any{"attemptId": "manual-landing-proof"}})
			return intentProcessResult{stdout: receipt}
		}
		return realProcess(process)
	}
	owners.delivery = delivery
	// The journey bed serves ledger acts from its fixture, so the goal's
	// land-ready state is written on the ledger as the whole-owner landing
	// fixture writes it; the landing itself is the owners' own.
	markJourneyLandReady(t, c)
	var stdout, stderr bytes.Buffer
	code := runIntentIn(mustIntentCommand(t, "land"), []string{c.id, "--repo", root, "--json"}, &stdout, &stderr, root, owners)
	var landed intentResult
	if err := json.Unmarshal(stdout.Bytes(), &landed); err != nil {
		t.Fatalf("land printed no result: %v; %q %q", err, stdout.String(), stderr.String())
	}
	main := connectionGit(t, root, "--git-dir", c.origin, "rev-parse", "refs/heads/main")
	if code != 0 || landed.Outcome != intentConfirmed || len(proved) == 0 || main == j.baseline {
		t.Fatalf("public land of the manual work: code=%d %+v", code, landed)
	}
	if got := connectionGit(t, root, "--git-dir", c.origin, "show", main+":delivered.txt"); got != "hand-written and delivered" {
		t.Fatalf("origin main does not carry the manual work: %q", got)
	}
	connectionGit(t, root, "fetch", "-q", "origin", "main")
	series, err := branch.VerifyLandedSeries(root, main)
	if err != nil || len(series) == 0 {
		t.Fatalf("landed series = %+v, %v", series, err)
	}
	if message := connectionGit(t, root, "log", "-1", "--format=%B", main); !strings.Contains(message, "Goal-Source: "+commit) || !strings.Contains(message, "Goal-Last: "+c.id) {
		t.Fatalf("the landing does not name the manual unit as its source:\n%s", message)
	}
}

func markJourneyLandReady(t *testing.T, c *connectionBed) {
	t.Helper()
	root := c.root()
	pagePath := filepath.Join(root, "plans", "goals", c.id+".md")
	data, err := os.ReadFile(pagePath)
	if err != nil {
		t.Fatal(err)
	}
	file, problems := goal.ParseFile(data)
	if len(problems) != 0 {
		t.Fatalf("parse goal page: %v", problems)
	}
	opid := goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FB0", "mac-cli", "m1")
	file.Revision++
	file.Landing = &goal.LandingRecord{At: "2026-09-26T09:00:00Z", Opid: opid}
	file.History = append(file.History, goal.HistoryLine{At: file.Landing.At, Opid: opid, Verb: "land-ready", Actor: "mac-cli+m1", Targets: []string{file.Id}, Keep: -1})
	os.WriteFile(pagePath, goal.RenderFile(file), 0o644)
	os.MkdirAll(filepath.Join(root, "metasystem", "memory"), 0o700)
	os.WriteFile(filepath.Join(root, "metasystem", "memory", "receipts.log"), []byte("1|1970-01-01T00:00:00Z|RECEIPT|type=seed|outcome=shipped\n"), 0o644)
	connectionGit(t, root, "add", "plans/goals/"+c.id+".md", "metasystem/memory/receipts.log")
	connectionGit(t, root, "commit", "-qm", "mark the goal land ready")
	connectionGit(t, root, "push", "-q", "origin", "HEAD:main")
	tip := connectionGit(t, root, "rev-parse", "HEAD")
	connectionGit(t, root, "update-ref", goal.LocalLedgerBranch, tip)
	connectionGit(t, root, "update-ref", goal.AcceptedRef, tip)
}

// TestIntentBriefBeforeFirstWorkspaceBuilds: before a goal has a workspace,
// the generated brief says build prepares it and marks no missing decision,
// and build with that exact brief prepares the workspace and builds.
func TestIntentBriefBeforeFirstWorkspaceBuilds(t *testing.T) {
	c := newConnectionBed(t)
	designs := filepath.Join(c.stateRoot(), "plans", "designs")
	os.MkdirAll(designs, 0o700)
	design := "# Standing validation\n\n- Kind: design\n- Id: 01M3CGR7CNZTS2NNTQCYRCF4JZ\n- Status: accepted\n- Goals: " + c.id + "\n\n" +
		"## Non-goals\n\nNo new ledger schema.\n\n## Units\n\n| Unit | Lines |\n| --- | ---: |\n| u1 | 40 |\n\n" +
		"## Return\n\nThe diff and the proof log.\n\n## Acceptance\n\nThe validator refuses a stale box.\n"
	os.WriteFile(filepath.Join(designs, c.id+".md"), []byte(design), 0o600)
	connectionGit(t, c.root(), "add", "-A")
	connectionGit(t, c.root(), "commit", "-q", "-m", "accepted design")
	if _, err := os.Stat(c.worktree); !os.IsNotExist(err) {
		t.Fatalf("the goal must have no workspace yet: %v", err)
	}
	code, result := c.do("brief", c.id, "--out", "brief.md")
	written, _ := os.ReadFile(filepath.Join(c.root(), "brief.md"))
	missing, _ := resultData(t, result)["missingDecisions"].([]any)
	if code != 0 || len(missing) != 0 || strings.Contains(string(written), "MISSING DECISION") || !strings.Contains(string(written), "metasystem build prepares it") ||
		strings.Contains(string(written), "worktree add") {
		t.Fatalf("brief before the first workspace: code=%d %+v\n%s", code, result, written)
	}
	c.edits = map[string]string{"built.txt": "built\n"}
	code, result = c.do(append([]string{"build", c.id, "u1", "--brief", "brief.md", "--check"}, workArgv...)...)
	if code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("build with the generated brief: code=%d %+v", code, result)
	}
	if _, err := os.Stat(c.worktree); err != nil {
		t.Fatalf("build prepared no workspace: %v", err)
	}
}

// TestIntentReviewSelectionAcrossProducers: with two committed work items
// awaiting review, bare review G starts nothing and names each exact
// command; status counts both and names each continuation; --work selects;
// and once one item's read is collected and published, bare review G goes
// to the one still awaiting review without a false ambiguity.
func TestIntentReviewSelectionAcrossProducers(t *testing.T) {
	j := newJourneyBed(t)
	c, root := j.c, j.c.root()
	brief := filepath.Join(t.TempDir(), "brief.md")
	os.WriteFile(brief, []byte("Hand-written work.\n"), 0o600)
	os.WriteFile(filepath.Join(root, "alpha.txt"), []byte("alpha\n"), 0o644)
	_, alpha := manualDo(t, j, root, "review", c.id, "--changes", "--brief", brief, "--work", "alpha")
	alphaCommit, _ := resultData(t, alpha)["commit"].(string)
	os.Remove(filepath.Join(root, "alpha.txt"))
	patch := filepath.Join(t.TempDir(), "beta.patch")
	os.WriteFile(patch, []byte("diff --git a/beta.txt b/beta.txt\nnew file mode 100644\n--- /dev/null\n+++ b/beta.txt\n@@ -0,0 +1 @@\n+beta\n"), 0o600)
	if _, beta := manualDo(t, j, root, "review", c.id, "--patch", patch, "--brief", brief, "--work", "beta"); beta.Outcome != intentInProgress {
		t.Fatalf("second item: %+v", beta)
	}
	delegates := len(c.delegates)
	code, ambiguous := manualDo(t, j, root, "review", c.id)
	if code != 2 || ambiguous.Outcome != intentRefused || !strings.Contains(ambiguous.Summary, "2 work items awaiting review") ||
		strings.Contains(ambiguous.Summary, "no built result") || len(c.delegates) != delegates ||
		fmt.Sprint(resultData(t, ambiguous)["candidates"]) != "[alpha beta]" {
		t.Fatalf("bare review with two awaiting items: code=%d %+v", code, ambiguous)
	}
	_, status := manualDo(t, j, root, "status", "goal", c.id)
	if !strings.Contains(status.Summary, "has 2 work items") {
		t.Fatalf("status counts both items: %+v", status)
	}
	// The printed command for beta selects beta.
	code, selected := manualDo(t, j, root, "review", c.id, "--work", "beta")
	if selected.Outcome == intentRefused || !slices.ContainsFunc(selected.Targets, func(target intentTarget) bool { return target.Kind == "work" && target.ID == "beta" }) {
		t.Fatalf("review --work beta: code=%d %+v", code, selected)
	}
	// Complete alpha's review; beta alone awaits review now.
	critic := ""
	for index, commit := range c.delegates {
		if commit == alphaCommit {
			critic = "crit" + strconv.Itoa(index+1)
		}
	}
	j.finish(c.worktree, critic, alphaCommit)
	if code, done := manualDo(t, j, root, "review", c.id, "--work", "alpha", "--dispositions", j.dispositions); code != 0 || done.Outcome != intentConfirmed {
		t.Fatalf("alpha's decisions: code=%d %+v", code, done)
	}
	code, inferred := manualDo(t, j, root, "review", c.id)
	if inferred.Outcome == intentRefused || !slices.ContainsFunc(inferred.Targets, func(target intentTarget) bool { return target.Kind == "work" && target.ID == "beta" }) {
		t.Fatalf("a reviewed item beside one awaiting review: code=%d %+v", code, inferred)
	}
}

// TestManualReviewFailedExaminationRetries: a manual submission whose
// examination ends without a return is not asked for findings; its printed
// continuation is the same review with --retry N, and following it reaches
// the branch read owner's retry of the same chain. Once that round
// completes, the retried request's own printed continuation collects it.
func TestManualReviewFailedExaminationRetries(t *testing.T) {
	j := newJourneyBed(t)
	c, root := j.c, j.c.root()
	brief := filepath.Join(t.TempDir(), "brief.md")
	os.WriteFile(brief, []byte("Hand-written work.\n"), 0o600)
	os.WriteFile(filepath.Join(root, "alpha.txt"), []byte("alpha\n"), 0o644)
	_, submitted := manualDo(t, j, root, "review", c.id, "--changes", "--brief", brief, "--work", "alpha")
	if submitted.Outcome != intentInProgress {
		t.Fatalf("submission: %+v", submitted)
	}
	critic := "crit" + strconv.Itoa(len(c.delegates))
	record := j.job(c.worktree, critic)
	for key, value := range map[string]any{"status": "timeout", "reason": "budget-cap", "round": 1, "endedAt": "2026-09-26T12:00:00Z", "groupDeathProvenAt": "2026-09-26T12:00:01Z",
		"reviewRoundLimit": 3, "destructiveReach": "DESIGN-BEARING", "dispatchMode": "fresh", "sessionId": critic + "-session",
		"capabilitySnapshot": "artifacts/agents/capabilities/close.json"} {
		record[key] = value
	}
	c.writeJSON(filepath.Join(c.worktree, "artifacts", "agents", "jobs", critic+".json"), record)
	// The reap folds the capped round, proven dead, into the chain's
	// register: neutrally, so a completed retry can close the chain.
	if outcome, err := dispatchcore.CritiqueRegisterAdvance(c.worktree, critic, critic); err != nil || outcome != "advanced" {
		t.Fatalf("register advance of the failed round = %q, %v", outcome, err)
	}
	code, failed := manualDo(t, j, root, "review", c.id, "--work", "alpha")
	if failed.Outcome != intentFailed || strings.Contains(failed.Decision, "--dispositions") || !strings.Contains(failed.Decision, "--retry 1") ||
		!strings.Contains(failed.Decision, "review "+c.id+" --work alpha") {
		t.Fatalf("a failed examination offers its retry, not a decisions file: code=%d %+v", code, failed)
	}
	// The printed continuation is the canonical public review, whichever
	// command printed it and whatever stale flags it carried.
	printed := func(result intentResult) []string {
		t.Helper()
		at := strings.Index(result.Decision, "metasystem ")
		if at < 0 {
			t.Fatalf("no command in %q", result.Decision)
		}
		return strings.Fields(result.Decision[at:])[1:]
	}
	want := []string{"review", c.id, "--work", "alpha", "--retry", "1"}
	for _, args := range [][]string{{"repair", "review", c.id}, {"review", c.id, "--work", "alpha", "--dispositions", j.dispositions}} {
		if _, again := manualDo(t, j, root, args...); !slices.Equal(printed(again), want) {
			t.Fatalf("%v printed %q, want %v", args, again.Decision, want)
		}
	}
	if !slices.Equal(printed(failed), want) {
		t.Fatalf("printed %q, want %v", failed.Decision, want)
	}
	// Following it enters the branch read owner's retry: one follow-up round
	// of the same chain with the frozen brief; a repeat rejoins it.
	code, retried := manualDo(t, j, root, printed(failed)...)
	if len(c.followUps) != 1 || !strings.HasPrefix(c.followUps[0], critic+" ") || retried.Outcome == intentFailed {
		t.Fatalf("the retry: code=%d %+v followUps=%v", code, retried, c.followUps)
	}
	if code, again := manualDo(t, j, root, printed(failed)...); len(c.followUps) != 1 || again.Outcome == intentFailed {
		t.Fatalf("a repeated retry started another round: code=%d %+v followUps=%v", code, again, c.followUps)
	}
	if _, err := os.Stat(filepath.Join(c.worktree, "artifacts", "agents", "jobs", critic+"-r2.json")); err != nil {
		t.Fatalf("the follow-up round of the same chain: %v", err)
	}
	// The retried request's printed continuation drops --retry: repeating
	// the retry would only rejoin the round, never collect it.
	wantNext := []string{"metasystem", "review", c.id, "--work", "alpha"}
	if retried.Next == nil || !slices.Equal(retried.Next.Argv, wantNext) {
		t.Fatalf("the retried request's continuation: %+v", retried.Next)
	}
	// Round 2 completes (the model is the fixture's) with one finding; the
	// printed continuation asks for its decision, and the decided review is
	// closed by the whole close owner, collected and published.
	alphaCommit, _ := resultData(t, submitted)["commit"].(string)
	j.finishRound(c.worktree, critic, critic+"-r2", alphaCommit, 2, []any{map[string]any{"id": "F1", "material": false}})
	code, decide := manualDo(t, j, root, retried.Next.Argv[1:]...)
	if decide.Outcome != intentInProgress || !strings.Contains(decide.Decision, "review "+c.id+" --work alpha --dispositions FILE") || strings.Contains(decide.Decision, "--retry") {
		t.Fatalf("the completed retried round: code=%d %+v", code, decide)
	}
	decided := append(slices.Clone(printed(decide)), j.dispositions)
	decided = slices.DeleteFunc(decided, func(word string) bool { return word == "FILE" })
	publications, delegates, reads := c.publications, len(c.delegates), c.commitReads
	code, done := manualDo(t, j, root, decided...)
	if code != 0 || done.Outcome != intentConfirmed || c.publications != publications+1 || c.commitReads != reads+1 || len(c.delegates) != delegates || len(c.followUps) != 1 {
		t.Fatalf("the decided retried round: code=%d %+v publications=%d reads=%d", code, done, c.publications, c.commitReads)
	}
	if closed := j.job(c.worktree, critic); closed["chainClosed"] != true {
		t.Fatalf("the chain is not closed by the close owner: %v", closed)
	}
	// A repeat finds the read collected and its publication current: no
	// critic, round, read commit or push is made again.
	reads = c.commitReads
	if code, again := manualDo(t, j, root, retried.Next.Argv[1:]...); code != 0 || again.Outcome != intentUnchanged ||
		c.commitReads != reads || len(c.delegates) != delegates || len(c.followUps) != 1 {
		t.Fatalf("a repeat after collection: code=%d %+v", code, again)
	}

	// A second work item whose examination is still running refuses a retry.
	os.WriteFile(filepath.Join(root, "beta.txt"), []byte("beta\n"), 0o644)
	os.Remove(filepath.Join(root, "alpha.txt"))
	if _, beta := manualDo(t, j, root, "review", c.id, "--changes", "--brief", brief, "--work", "beta"); beta.Outcome != intentInProgress {
		t.Fatalf("second item: %+v", beta)
	}
	code, live := manualDo(t, j, root, "review", c.id, "--work", "beta", "--retry", "1")
	if live.Outcome != intentRefused && live.Outcome != intentFailed || len(c.followUps) != 1 {
		t.Fatalf("a retry of a running examination: code=%d %+v followUps=%v", code, live, c.followUps)
	}
}

// TestManualReviewProtocolFailureNeedsAcceptedRisk: a round that failed on
// its protocol leaves an unproven finding in the chain's register. After a
// completed retry and the author's decisions, the whole close owner still
// refuses to close silently; review names the finding and the complete
// public accept-risk command, then the same review. Following that printed
// command reaches the review in its selected repository, and the agent
// remains unable to accept its risk. Declared human authority completes the
// same command and discharges only that recorded missing-evidence risk.
func TestManualReviewProtocolFailureNeedsAcceptedRisk(t *testing.T) {
	j := newJourneyBed(t)
	c, root := j.c, j.c.root()
	brief := filepath.Join(t.TempDir(), "brief.md")
	os.WriteFile(brief, []byte("Hand-written work.\n"), 0o600)
	os.WriteFile(filepath.Join(root, "alpha.txt"), []byte("alpha\n"), 0o644)
	_, submitted := manualDo(t, j, root, "review", c.id, "--changes", "--brief", brief, "--work", "alpha")
	alphaCommit, _ := resultData(t, submitted)["commit"].(string)
	if submitted.Outcome != intentInProgress || alphaCommit == "" {
		t.Fatalf("submission: %+v", submitted)
	}
	critic := "crit" + strconv.Itoa(len(c.delegates))
	record := j.job(c.worktree, critic)
	for key, value := range map[string]any{"status": "failed", "round": 1, "endedAt": "2026-09-26T12:00:00Z", "groupDeathProvenAt": "2026-09-26T12:00:01Z",
		"error": "the critic returned no result", "phase": "return", "reviewRoundLimit": 3, "destructiveReach": "DESIGN-BEARING", "dispatchMode": "fresh",
		"sessionId": critic + "-session", "capabilitySnapshot": "artifacts/agents/capabilities/close.json"} {
		record[key] = value
	}
	c.writeJSON(filepath.Join(c.worktree, "artifacts", "agents", "jobs", critic+".json"), record)
	// The reap folds the failed round: the register holds its unproven
	// protocol finding.
	if outcome, err := dispatchcore.CritiqueRegisterAdvance(c.worktree, critic, critic); err != nil || outcome != "advanced" {
		t.Fatalf("register advance of the failed round = %q, %v", outcome, err)
	}
	open, err := dispatchcore.CritiqueOpenFindingIDs(c.worktree, critic)
	if err != nil || len(open) != 1 || !strings.HasPrefix(open[0], "synthetic-") {
		t.Fatalf("the failed round's register: %v %v", open, err)
	}
	printed := func(text, from string) []string {
		t.Helper()
		at := strings.Index(text, from)
		if at < 0 {
			t.Fatalf("no %q in %q", from, text)
		}
		command := text[at:]
		if end := strings.Index(command, ";"); end >= 0 {
			command = command[:end]
		}
		return strings.Fields(command)[1:]
	}
	_, failed := manualDo(t, j, root, "review", c.id, "--work", "alpha")
	if _, retried := manualDo(t, j, root, printed(failed.Decision, "metasystem review")...); len(c.followUps) != 1 || retried.Next == nil {
		t.Fatalf("the retry: %+v followUps=%v", retried, c.followUps)
	}
	j.finishRound(c.worktree, critic, critic+"-r2", alphaCommit, 2, []any{map[string]any{"id": "F1", "material": false}})
	_, decide := manualDo(t, j, root, "review", c.id, "--work", "alpha")
	decided := slices.DeleteFunc(printed(decide.Decision, "metasystem review"), func(word string) bool { return word == "FILE" })
	decided = append(decided, j.dispositions)
	publications, reads := c.publications, c.commitReads
	code, refused := manualDo(t, j, root, decided...)
	risks, _ := resultData(t, refused)["riskFindings"].([]any)
	accept := "metasystem accept-risk " + c.id + " --finding " + open[0] + " --review " + critic + " --repo " + c.worktree + " --reason TEXT"
	if code == 0 || refused.Outcome != intentRefused || len(risks) != 1 || risks[0] != open[0] ||
		!strings.Contains(refused.Decision, accept) || !strings.Contains(refused.Decision, "then run metasystem review "+c.id+" --work alpha --dispositions "+j.dispositions) {
		t.Fatalf("an unproven finding after the retry: code=%d %+v", code, refused)
	}
	for _, internal := range []string{"goal accept-risk", "--chain", "critique-budget-rebind", "metasystem close", "dispatch.sh"} {
		if strings.Contains(refused.Decision, internal) || strings.Contains(refused.Summary, internal) || slices.ContainsFunc(refused.text, func(line string) bool { return strings.Contains(line, internal) }) {
			t.Fatalf("the remedy prescribes internal %q: %+v", internal, refused)
		}
	}
	if closed := j.job(c.worktree, critic); closed["chainClosed"] == true || c.publications != publications || c.commitReads != reads {
		t.Fatalf("the chain closed or was collected silently: %v publications=%d reads=%d", closed["chainClosed"], c.publications, c.commitReads)
	}
	// The printed accept-risk command, followed through the public parser
	// by this agent, accepts nothing and leaves the register unchanged.
	acceptArgs := printed(refused.Decision, "metasystem accept-risk")
	acceptArgs[len(acceptArgs)-1] = "the failed round examined nothing"
	// Goal worktrees share the goal ledger, but this request's authority
	// facts must describe the selected checkout where the review is kept.
	acceptAtReview := func(args ...string) (int, intentResult) {
		previous := c.facts.root
		c.facts.root = c.worktree
		defer func() { c.facts.root = previous }()
		return manualDo(t, j, root, args...)
	}
	code, agent := acceptAtReview(acceptArgs...)
	t.Logf("printed risk decision result: code=%d %+v", code, agent)
	if code == 0 || agent.Outcome == intentConfirmed || !strings.Contains(agent.Summary, "a person's act") {
		t.Fatalf("the risk decision did not reach the authority guard: code=%d %+v", code, agent)
	}
	if still, err := dispatchcore.CritiqueOpenFindingIDs(c.worktree, critic); err != nil || !slices.Equal(still, open) {
		t.Fatalf("the refused acceptance changed the register: %v %v", still, err)
	}
	// The bed's declared human proof authorizes the same public command.
	// The goal decision, risk record, register and proof writers stay real;
	// the accepted goal ledger uses the bed's existing repository adapter.
	writeFixtureEnrollment(t, c.worktree, "Wido")
	code, accepted := acceptAtReview(append(slices.Clone(acceptArgs), "--by", "Wido", "--fixture-human-authority")...)
	if code != 0 || accepted.Outcome != intentConfirmed {
		t.Fatalf("the authorized public risk decision: code=%d %+v", code, accepted)
	}
	if remaining, err := dispatchcore.CritiqueOpenFindingIDs(c.worktree, critic); err != nil || len(remaining) != 0 {
		t.Fatalf("the human decision did not discharge the exact risk: %v %v", remaining, err)
	}
}

// TestIntentReviewSelectionMixedProducers: a built, unread work item and a
// hand-written committed item both await review. Bare review G starts no
// examination and names both exact commands; --work selects each through
// its own producer; once the hand-written item's read is published, bare
// review G goes to the built item.
func TestIntentReviewSelectionMixedProducers(t *testing.T) {
	j := newJourneyBed(t)
	c, do, root := j.c, j.do, j.c.root()
	brief := c.brief("brief.md", "Build the unit.\n")
	os.WriteFile(filepath.Join(root, "hand.txt"), []byte("hand\n"), 0o644)
	_, hand := manualDo(t, j, root, "review", c.id, "--changes", "--brief", brief, "--work", "hand")
	handCommit, _ := resultData(t, hand)["commit"].(string)
	if hand.Outcome != intentInProgress || handCommit == "" {
		t.Fatalf("manual submission: %+v", hand)
	}
	os.Remove(filepath.Join(root, "hand.txt"))
	c.edits = map[string]string{"built.txt": "built\n"}
	if code, built := do(append([]string{"build", c.id, "built-unit", "--brief", brief, "--lines", "5"}, workCheck...)...); code != 0 || built.Outcome != intentConfirmed {
		t.Fatalf("build: code=%d %+v", code, built)
	}
	// While the build's result is uncommitted, the goal worktree receives no
	// other hand-written work.
	os.WriteFile(filepath.Join(root, "late.txt"), []byte("late\n"), 0o644)
	if _, late := manualDo(t, j, root, "review", c.id, "--changes", "--brief", brief, "--work", "late"); late.Outcome != intentRefused || !strings.Contains(late.Summary, "built.txt") {
		t.Fatalf("a submission onto an uncommitted build result: %+v", late)
	}
	os.Remove(filepath.Join(root, "late.txt"))
	delegates := len(c.delegates)
	code, ambiguous := manualDo(t, j, root, "review", c.id)
	if code != 2 || ambiguous.Outcome != intentRefused || len(c.delegates) != delegates ||
		fmt.Sprint(resultData(t, ambiguous)["candidates"]) != "[built-unit hand]" {
		t.Fatalf("bare review with a built and a manual item: code=%d %+v", code, ambiguous)
	}
	works := func(result intentResult) []string {
		var names []string
		for _, target := range result.Targets {
			if target.Kind == "work" {
				names = append(names, target.ID)
			}
		}
		return names
	}
	if _, selected := manualDo(t, j, root, "review", c.id, "--work", "built-unit"); !slices.Contains(works(selected), "built-unit") || len(c.delegates) != delegates+1 {
		t.Fatalf("review --work built-unit: %+v", selected)
	}
	if _, selected := manualDo(t, j, root, "review", c.id, "--work", "hand"); !slices.Contains(works(selected), "hand") {
		t.Fatalf("review --work hand: %+v", selected)
	}
	critic := ""
	for index, commit := range c.delegates {
		if commit == handCommit {
			critic = "crit" + strconv.Itoa(index+1)
		}
	}
	j.finish(c.worktree, critic, handCommit)
	if code, done := manualDo(t, j, root, "review", c.id, "--work", "hand", "--dispositions", j.dispositions); code != 0 || done.Outcome != intentConfirmed {
		t.Fatalf("the hand-written item's decisions: code=%d %+v", code, done)
	}
	if _, inferred := manualDo(t, j, root, "review", c.id); inferred.Outcome == intentRefused || !slices.Contains(works(inferred), "built-unit") {
		t.Fatalf("bare review once the manual item is published: %+v", inferred)
	}
}

// TestManualCleanReviewCompletesItself: hand-written work whose examination
// returns no findings completes its review without a decisions file: the
// real whole close owner closes the chain, the read is collected and
// published, and a replay starts no second commit or critic. A malformed
// return is not completed.
func TestManualCleanReviewCompletesItself(t *testing.T) {
	j := newJourneyBed(t)
	c, root := j.c, j.c.root()
	brief := filepath.Join(t.TempDir(), "brief.md")
	os.WriteFile(brief, []byte("Hand-written work.\n"), 0o600)
	os.WriteFile(filepath.Join(root, "clean.txt"), []byte("clean\n"), 0o644)
	_, submitted := manualDo(t, j, root, "review", c.id, "--changes", "--brief", brief)
	commit, _ := resultData(t, submitted)["commit"].(string)
	if submitted.Outcome != intentInProgress || commit == "" {
		t.Fatalf("submission: %+v", submitted)
	}
	source := connectionGit(t, root, "status", "--porcelain")
	critic := "crit" + strconv.Itoa(len(c.delegates))
	j.finishWith(c.worktree, critic, commit, []any{})
	commits, publications, delegates := c.commits, c.publications, len(c.delegates)
	code, done := manualDo(t, j, root, "review", c.id, "--work", "main")
	if code != 0 || done.Outcome != intentConfirmed || strings.Contains(done.Decision, "--dispositions") || j.job(c.worktree, critic)["chainClosed"] != true ||
		c.publications != publications+1 {
		t.Fatalf("a clean examination completes the review: code=%d %+v", code, done)
	}
	if code, again := manualDo(t, j, root, "review", c.id, "--work", "main"); code != 0 || (again.Outcome != intentUnchanged && again.Outcome != intentConfirmed) ||
		c.commits != commits || len(c.delegates) != delegates {
		t.Fatalf("a replay: code=%d %+v commits %d->%d critics %d->%d", code, again, commits, c.commits, delegates, len(c.delegates))
	}
	if _, status := manualDo(t, j, root, "status", "goal", c.id, "--work", "main"); !strings.Contains(status.Summary, "collected and published") {
		t.Fatalf("status after the clean review: %+v", status)
	}
	if connectionGit(t, root, "status", "--porcelain") != source {
		t.Fatal("the clean review changed the source checkout")
	}
	// A malformed return is not a clean examination and is not closed.
	os.Remove(filepath.Join(root, "clean.txt"))
	os.WriteFile(filepath.Join(root, "broken.txt"), []byte("broken\n"), 0o644)
	_, second := manualDo(t, j, root, "review", c.id, "--changes", "--brief", brief, "--work", "broken")
	brokenCommit, _ := resultData(t, second)["commit"].(string)
	if brokenCommit == "" {
		t.Fatalf("second submission: %+v", second)
	}
	brokenCritic := "crit" + strconv.Itoa(len(c.delegates))
	j.finishWith(c.worktree, brokenCritic, brokenCommit, []any{})
	os.WriteFile(filepath.Join(c.worktree, "artifacts", "agents", brokenCritic, "rounds", "1", "return.json"), []byte("{not json"), 0o644)
	if code, broken := manualDo(t, j, root, "review", c.id, "--work", "broken"); broken.Outcome == intentConfirmed || j.job(c.worktree, brokenCritic)["chainClosed"] == true {
		t.Fatalf("a malformed return was completed: code=%d %+v", code, broken)
	}
}
