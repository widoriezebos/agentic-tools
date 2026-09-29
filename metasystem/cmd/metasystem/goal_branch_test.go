package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	dispatchmodel "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestGoalBranchReadDelegateUsesBinarySeamAndReturnsWithoutWaiting(t *testing.T) {
	dir := t.TempDir()
	argsPath := filepath.Join(dir, "args")
	binary := filepath.Join(dir, "metasystem")
	script := "#!/usr/bin/env bash\nset -euo pipefail\nprintf '%s\\n' \"$*\" >\"$ARGS_PATH\"\nprintf '{\"outcome\":\"WON\",\"headline\":\"started\",\"jobId\":\"critic-fake\"}\\n'\n"
	if err := testexec.WriteFile(binary, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ARGS_PATH", argsPath)
	job, err := readDelegate(scriptDelegator(binary), dir, filepath.Join(dir, "brief.md"), "goal-a", strings.Repeat("a", 40), "", "")
	if err != nil || job != "critic-fake" {
		t.Fatalf("job=%q err=%v", job, err)
	}
	args, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(args)
	for _, want := range []string{"delegate --role code-critic", "--reviews commit:" + strings.Repeat("a", 40), "--goal goal-a", "--brief "} {
		if !strings.Contains(text, want) {
			t.Fatalf("delegate args %q omit %q", text, want)
		}
	}
	if strings.Contains(text, "--wait") {
		t.Fatalf("delegate args wait for the model: %q", text)
	}
}

func TestGLEGoalBranchReadPassesFrozenBriefAndSolOverrideToDelegate(t *testing.T) {
	f := newBranchRawFixture(t, false, false)
	worktree, unit := f.installation, f.unit
	raw := f.dependencies()
	code, stdout, stderr := 0, "", ""
	input := filepath.Join(t.TempDir(), "accepted-design.md")
	design := "Accepted implementation design: exact input identity and declared ownership.\n"
	writeTestingFixtureFile(t, input, []byte(design), 0o644)
	argsPath, copyPath := filepath.Join(t.TempDir(), "argv"), filepath.Join(t.TempDir(), "frozen-copy")
	binary := filepath.Join(t.TempDir(), "metasystem")
	script := "#!/usr/bin/env bash\nset -euo pipefail\nprintf '%s\\n' \"$@\" >\"$ARGS_PATH\"\n" +
		"while (( $# )); do if [[ \"$1\" == --brief ]]; then cp \"$2\" \"$BRIEF_COPY\"; fi; shift; done\n" +
		"printf '{\"outcome\":\"WON\",\"headline\":\"started\",\"jobId\":\"critic-fake\"}\\n'\n"
	if err := testexec.WriteFile(binary, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ARGS_PATH", argsPath)
	t.Setenv("BRIEF_COPY", copyPath)
	code, stdout, stderr = runOnOwnStreams(func(stdout, stderr io.Writer) int {
		return runGoalBranchReadWith([]string{"--root", worktree, "--goal", "standing-validation", "--unit", unit,
			"--brief", input, "--runtime", "codex", "--model", "gpt-5.6-sol"}, goalBranchReadDependencies{
			Delegator: scriptDelegator(binary), Gate: func(string) (string, error) { return "green", nil }, Raw: raw,
		}, stdout, stderr)
	})
	if code != 0 || stderr != "" || !strings.Contains(stdout, "state=dispatched") {
		t.Fatalf("branch read: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	argv, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"delegate\n", "--role\ncode-critic\n", "--reviews\ncommit:" + unit + "\n", "--runtime\ncodex\n", "--model\ngpt-5.6-sol\n"} {
		if !strings.Contains(string(argv), want) {
			t.Fatalf("delegate argv %q omit %q", argv, want)
		}
	}
	copyBody, err := os.ReadFile(copyPath)
	if err != nil || !strings.Contains(string(copyBody), design) || strings.Contains(string(copyBody), "goals-live-on-branches-design.md") {
		t.Fatalf("frozen delegate brief=%q err=%v", copyBody, err)
	}
	if err := os.WriteFile(input, []byte("changed source\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if body, err := os.ReadFile(copyPath); err != nil || string(body) != string(copyBody) {
		t.Fatalf("delegate input changed after source edit: %q err=%v", body, err)
	}
}

func TestGLEGoalBranchNestedReadCollectWritesProjectAttestation(t *testing.T) {
	f := newBranchRawFixture(t, true, false)
	installation, unit := f.installation, f.unit
	raw := f.dependencies()
	code, stdout, stderr := 0, "", ""
	base := f.base
	const job = "critic-nested"
	writeClosure := func() {
		t.Helper()
		subject, present, err := dispatchmodel.ComputeReadSubjectWithFacts(dispatchmodel.ReadSubjectRequest{
			RepoRoot: installation, Role: "code-critic", Reviews: "commit:" + unit,
		}, f)
		if err != nil || !present {
			t.Fatalf("nested subject present=%t err=%v", present, err)
		}
		record := map[string]any{"jobId": job, "role": "code-critic", "round": 1, "status": "completed",
			"reviews": "commit:" + unit, "goalId": "standing-validation", "goalRevision": 1,
			"findingRegister": []any{}, "findingRegisterRound": 1, "findingRegisterSubjectDigest": subject.Digest(),
			"chainClosed": true, "closure": map[string]any{"criticRoot": job, "round": 1, "subject": subject, "mechanism": "clean"}}
		for path, value := range map[string]any{
			"jobs/" + job + ".json":        record,
			job + "/rounds/1/subject.json": subject,
			job + "/rounds/1/return.json":  map[string]any{"jobId": job, "round": 1, "reviewedTree": subject.Tree},
		} {
			body, err := json.MarshalIndent(value, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			writeTestingFixtureFile(t, filepath.Join(installation, "artifacts", "agents", filepath.FromSlash(path)), append(body, '\n'), 0o644)
		}
	}
	deps := goalBranchReadDependencies{Gate: func(string) (string, error) { return "green", nil },
		Delegate: func(_, _, _, _, _ string) (string, error) { writeClosure(); return job, nil }, Commit: branch.CommitRead, Raw: raw}
	args := []string{"--root", installation, "--goal", "standing-validation", "--unit", unit}
	code, stdout, stderr = runOnOwnStreams(func(stdout, stderr io.Writer) int { return runGoalBranchReadWith(args, deps, stdout, stderr) })
	if code != 0 || stderr != "" || !strings.Contains(stdout, "state=dispatched") {
		t.Fatalf("nested read dispatch: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	code, stdout, stderr = runOnOwnStreams(func(stdout, stderr io.Writer) int {
		return runGoalBranchReadWith(append(append([]string{}, args...), "--collect"), deps, stdout, stderr)
	})
	if code != 0 || stderr != "" || !strings.Contains(stdout, "state=collected") {
		t.Fatalf("nested read collect: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if _, err := branch.ValidateAttestationWithReads(f, installation, base, "standing-validation", "u1", unit); err != nil {
		t.Fatalf("nested attestation validation: %v", err)
	}
	if _, err := os.Stat(filepath.Join(installation, "records", "reads", "standing-validation", unit+".json")); err != nil {
		t.Fatalf("attestation missing from installation records: %v", err)
	}
	if !f.staged || !f.published || f.tip != f.read {
		t.Fatalf("read publication: staged=%t published=%t tip=%s", f.staged, f.published, f.tip)
	}
}

func TestGLEGoalBranchReadRejectsDuplicateAndEmptyContextFlags(t *testing.T) {
	for _, name := range []string{"brief", "runtime", "model"} {
		for _, test := range []struct {
			label, want string
			args        []string
		}{
			{label: "duplicate", want: "only once", args: []string{"--" + name, "first", "--" + name, "second"}},
			{label: "empty", want: "nonempty value", args: []string{"--" + name + "="}},
		} {
			t.Run(name+"/"+test.label, func(t *testing.T) {
				args := append([]string{"--goal", "goal-a", "--unit", strings.Repeat("a", 40)}, test.args...)
				code, stdout, stderr := runOnOwnStreams(func(stdout, stderr io.Writer) int {
					return runGoalBranchReadWith(args, goalBranchReadDependencies{}, stdout, stderr)
				})
				if code != 2 || stdout != "" || !strings.Contains(stderr, test.want) {
					t.Fatalf("flags %v: code=%d stdout=%q stderr=%q", test.args, code, stdout, stderr)
				}
			})
		}
	}
}

func TestGLEGoalBranchReadRetriesStructuredRosterRefusalWithFrozenContext(t *testing.T) {
	f := newBranchRawFixture(t, false, false)
	worktree, unit := f.installation, f.unit
	raw := f.dependencies()
	code, stdout, stderr := 0, "", ""
	input := filepath.Join(t.TempDir(), "accepted-design.md")
	accepted := "accepted design frozen before the roster refusal\n"
	writeTestingFixtureFile(t, input, []byte(accepted), 0o644)
	stateDir := t.TempDir()
	firstPath, launchPath := filepath.Join(stateDir, "first"), filepath.Join(stateDir, "native-launches")
	argvPath, briefCopy := filepath.Join(stateDir, "argv"), filepath.Join(stateDir, "brief")
	binary := filepath.Join(stateDir, "metasystem")
	script := "#!/usr/bin/env bash\nset -euo pipefail\n" +
		"if [[ ! -f \"$FIRST_PATH\" ]]; then touch \"$FIRST_PATH\"; printf '{\"outcome\":\"REFUSED-ROSTER\",\"headline\":\"refused\",\"detail\":\"requested roster pair refused before launch\"}\\n'; exit 1; fi\n" +
		"printf '%s\\n' \"$@\" >\"$ARGV_PATH\"\n" +
		"while (( $# )); do if [[ \"$1\" == --brief ]]; then cp \"$2\" \"$BRIEF_COPY\"; fi; shift; done\n" +
		"printf 'native\\n' >>\"$LAUNCH_PATH\"\nprintf '{\"outcome\":\"WON\",\"headline\":\"started\",\"jobId\":\"critic-retry\"}\\n'\n"
	if err := testexec.WriteFile(binary, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FIRST_PATH", firstPath)
	t.Setenv("LAUNCH_PATH", launchPath)
	t.Setenv("ARGV_PATH", argvPath)
	t.Setenv("BRIEF_COPY", briefCopy)
	deps := goalBranchReadDependencies{Delegator: scriptDelegator(binary), Gate: func(string) (string, error) { return "green", nil }, Raw: raw}
	code, _, stderr = runOnOwnStreams(func(stdout, stderr io.Writer) int {
		return runGoalBranchReadWith([]string{"--root", worktree, "--goal", "standing-validation", "--unit", unit,
			"--brief", input, "--runtime", "codex", "--model", "gpt-5.6-sol"}, deps, stdout, stderr)
	})
	if code != 1 || !strings.Contains(stderr, "REFUSED-ROSTER") {
		t.Fatalf("structured prelaunch refusal: code=%d stderr=%q", code, stderr)
	}
	if _, err := os.Stat(launchPath); !os.IsNotExist(err) {
		t.Fatalf("prelaunch refusal recorded a native launch: %v", err)
	}
	if err := os.WriteFile(input, []byte("changed after refusal\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr = runOnOwnStreams(func(stdout, stderr io.Writer) int {
		return runGoalBranchReadWith([]string{"--root", worktree, "--goal", "standing-validation", "--unit", unit}, deps, stdout, stderr)
	})
	if code != 0 || stderr != "" || !strings.Contains(stdout, "state=dispatched") {
		t.Fatalf("frozen retry: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	argv, err := os.ReadFile(argvPath)
	if err != nil || !strings.Contains(string(argv), "--runtime\ncodex\n") || !strings.Contains(string(argv), "--model\ngpt-5.6-sol\n") {
		t.Fatalf("retry argv=%q err=%v", argv, err)
	}
	body, err := os.ReadFile(briefCopy)
	if err != nil || !strings.Contains(string(body), accepted) || strings.Contains(string(body), "changed after refusal") {
		t.Fatalf("retry brief=%q err=%v", body, err)
	}
	launches, err := os.ReadFile(launchPath)
	if err != nil || string(launches) != "native\n" {
		t.Fatalf("native launches=%q err=%v", launches, err)
	}
}

func TestGLEGoalBranchReadKeepsUncertainDelegateOutcomePending(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "metasystem")
	script := "#!/usr/bin/env bash\nset -euo pipefail\nprintf '{\"outcome\":\"HANDSHAKE-FAILED\",\"headline\":\"refused\",\"detail\":\"session did not establish\",\"jobId\":\"critic-uncertain\"}\\n'\nexit 3\n"
	if err := testexec.WriteFile(binary, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := readDelegate(scriptDelegator(binary), dir, filepath.Join(dir, "brief.md"), "goal-a", strings.Repeat("a", 40), "codex", "gpt-5.6-sol")
	var never *branch.ReadNeverLaunchedError
	var structured *readDelegateOutcomeError
	if err == nil || errors.As(err, &never) || !errors.As(err, &structured) ||
		structured.Outcome.Outcome != "HANDSHAKE-FAILED" || structured.Outcome.JobID != "critic-uncertain" ||
		structured.Outcome.Detail != "session did not establish" {
		t.Fatalf("uncertain outcome err=%v structured=%+v", err, structured)
	}
}

type commandRedRunner struct {
	runs []branch.DiagnosticRun
}

func (f *commandRedRunner) Run(run branch.DiagnosticRun) (branch.DiagnosticResult, error) {
	f.runs = append(f.runs, run)
	return branch.DiagnosticResult{AttemptID: "endpoint-clean", Green: true}, nil
}

type commandLandingProgress struct {
	lines []string
	next  []string
}

func (f *commandLandingProgress) RecordLandingProgress(line, next string) error {
	f.lines = append(f.lines, line)
	f.next = append(f.next, next)
	return nil
}

func goalBranchMainCLIFixtureBelow(t *testing.T, lineage, subdir string) (string, string, string) {
	t.Helper()
	return goalBranchMainFixture(t, lineage, subdir, false)
}

// goalBranchMainFixture builds the main checkout; template declares the
// nested installation the template (metasystem.template=true, committed).
func goalBranchMainFixture(t *testing.T, lineage, subdir string, template bool) (string, string, string) {
	t.Helper()
	repo := syncedClaimedGoalFixture(t)
	root := repo
	if subdir != "." {
		root = filepath.Join(repo, subdir)
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatal(err)
		}
		for _, path := range []string{"metasystem.conf", "plans", "scripts"} {
			goalSyncMutationGit(t, repo, "mv", path, filepath.ToSlash(filepath.Join(subdir, path)))
		}
		// The untracked fence engine moves with its installation.
		if err := os.Rename(filepath.Join(repo, "bin"), filepath.Join(root, "bin")); err != nil {
			t.Fatal(err)
		}
	}
	rootPath := filepath.Join(root, "plans", "goals", "backlog.md")
	rootBytes, err := os.ReadFile(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	rootRecord, problems := goal.ParseRoot(rootBytes)
	if len(problems) != 0 {
		t.Fatalf("parse root record: %v", problems)
	}
	rootRecord.SyncMode = goal.SyncRemote
	writeTestingFixtureFile(t, rootPath, goal.RenderRoot(rootRecord), 0o644)
	writeTestingFixtureFile(t, filepath.Join(repo, "metasystem", "memory", "receipts.log"), []byte("seed receipt\n"), 0o644)
	writeTestingFixtureFile(t, filepath.Join(repo, "metasystem", "records", "narrator-digest.log"), []byte("seed digest\n"), 0o644)
	writeTestingFixtureFile(t, filepath.Join(root, ".gitignore"), []byte("artifacts/\nbin/\n"), 0o644)
	if template {
		conf, err := os.ReadFile(filepath.Join(root, "metasystem.conf"))
		if err != nil {
			t.Fatal(err)
		}
		writeTestingFixtureFile(t, filepath.Join(root, "metasystem.conf"), append(conf, []byte("metasystem.template=true\n")...), 0o644)
	}
	goalSyncMutationGit(t, repo, "add", ".")
	goalSyncMutationGit(t, root, "commit", "-qm", "remote goal fixture")
	goalSyncMutationGit(t, root, "update-ref", goal.LocalLedgerBranch, "HEAD")
	goalSyncMutationGit(t, root, "update-ref", goal.AcceptedRef, "HEAD")
	upstream := filepath.Join(t.TempDir(), "upstream.git")
	goalSyncMutationGit(t, filepath.Dir(upstream), "init", "-q", "--bare", upstream)
	goalSyncMutationGit(t, root, "remote", "add", "upstream", upstream)
	goalSyncMutationGit(t, root, "config", "goal.sync-remote", "upstream")
	base := goalSyncMutationGit(t, root, "rev-parse", "HEAD")
	goalSyncMutationGit(t, root, "push", "-q", "upstream", "HEAD:main")
	exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("read fixture process identity: state=%s err=%v", state, err)
	}
	if _, err := lease.AnnounceWithPair(root, "goal-branch-fixture", int64(os.Getpid()), exact.StartedAt.Unix(), exact.StartTicks, exact.BootID, "fixture", "fake", lineage); err != nil {
		t.Fatalf("announce fixture lease holder: %v", err)
	}
	return root, upstream, base
}

func goalBranchCLIFixture(t *testing.T, lineage string) (string, string, string) {
	return goalBranchCLIFixtureBelow(t, lineage, ".")
}

func goalBranchCLIFixtureBelow(t *testing.T, lineage, subdir string) (string, string, string) {
	t.Helper()
	return goalBranchCLIFixtureAt(t, lineage, subdir, false)
}

// goalBranchTemplateCLIFixture is the self-hosted template layout: the
// installation nested at metasystem/ declares itself the template.
func goalBranchTemplateCLIFixture(t *testing.T, lineage string) (string, string, string) {
	t.Helper()
	return goalBranchCLIFixtureAt(t, lineage, "metasystem", true)
}

func goalBranchCLIFixtureAt(t *testing.T, lineage, subdir string, template bool) (string, string, string) {
	t.Helper()
	main, upstream, base := goalBranchMainFixture(t, lineage, subdir, template)
	worktreeTop := filepath.Join(t.TempDir(), "goal-worktree")
	goalSyncMutationGit(t, main, "worktree", "add", "-q", "-b", "goal/standing-validation", worktreeTop, base)
	return filepath.Join(worktreeTop, subdir), upstream, base
}

func goalBranchCheckoutState(t *testing.T, root string) string {
	t.Helper()
	parts := []string{
		goalSyncMutationGit(t, root, "symbolic-ref", "-q", "HEAD"),
		goalSyncMutationGit(t, root, "rev-parse", "HEAD^{commit}"),
		goalSyncMutationGit(t, root, "write-tree"),
		goalSyncMutationGit(t, root, "diff", "--binary", "HEAD"),
		goalSyncMutationGit(t, root, "status", "--porcelain=v1", "--untracked-files=all"),
	}
	return strings.Join(parts, "\n---\n")
}

func dirtyGoalBranchLedgers(t *testing.T, root string) {
	t.Helper()
	for _, rel := range []string{"metasystem/memory/receipts.log", "metasystem/records/narrator-digest.log"} {
		path := filepath.Join(root, filepath.FromSlash(rel))
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		writeTestingFixtureFile(t, path, append(body, []byte("live append\n")...), 0o644)
	}
}

func TestGoalBranchVerbsRunFromTheHoldersLinkedWorktree(t *testing.T) {
	worktree, _, _ := goalBranchCLIFixture(t, "m1")
	main, linked := linkedWorktreeMainCheckout(worktree)
	if !linked {
		t.Fatalf("fixture %s is not a linked worktree", worktree)
	}
	dirtyGoalBranchLedgers(t, main)
	mainBefore := goalBranchCheckoutState(t, main)
	record := "metasystem/records/decisions/linked-worktree.md"
	writeTestingFixtureFile(t, filepath.Join(worktree, filepath.FromSlash(record)), []byte("holder record\n"), 0o644)
	goalSyncMutationGit(t, worktree, "add", record)

	code, stdout, stderr := runOnOwnStreams(func(stdout, stderr io.Writer) int {
		return goalBranchTestCommand([]string{"commit", "--goal", "standing-validation", "--kind", "plan", "--root", worktree}, stdout, stderr)
	})
	tip := strings.TrimSpace(stdout)
	if code != 0 || stderr != "" || len(tip) != 40 {
		t.Fatalf("commit from linked worktree: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	code, _, stderr = runOnOwnStreams(func(stdout, stderr io.Writer) int {
		return goalBranchTestCommand([]string{"push", "--goal", "standing-validation", "--root", worktree, "--opid", "linked-holder-push"}, stdout, stderr)
	})
	if code != 0 || stderr != "" {
		t.Fatalf("push from linked worktree: code=%d stderr=%q", code, stderr)
	}
	if after := goalBranchCheckoutState(t, main); after != mainBefore {
		t.Fatalf("linked verbs changed the main checkout:\nbefore=%s\nafter=%s", mainBefore, after)
	}
	if head := goalSyncMutationGit(t, worktree, "rev-parse", "HEAD^{commit}"); head != tip {
		t.Fatalf("worktree HEAD=%s, want %s", head, tip)
	}
	remote := strings.Fields(goalSyncMutationGit(t, worktree, "ls-remote", "--refs", "upstream", "refs/heads/goal/standing-validation"))
	if len(remote) != 2 || remote[0] != tip {
		t.Fatalf("origin goal branch=%v, want %s", remote, tip)
	}
}

func TestGoalBranchHolderResolutionPreservesInstallationSubdirectory(t *testing.T) {
	worktree, _, _ := goalBranchCLIFixtureBelow(t, "m1", "metasystem")
	record := "records/decisions/nested-installation.md"
	writeTestingFixtureFile(t, filepath.Join(worktree, filepath.FromSlash(record)), []byte("nested holder record\n"), 0o644)
	goalSyncMutationGit(t, worktree, "add", record)

	code, stdout, stderr := runOnOwnStreams(func(stdout, stderr io.Writer) int {
		return goalBranchTestCommand([]string{"commit", "--goal", "standing-validation", "--kind", "plan", "--root", worktree}, stdout, stderr)
	})
	if code != 0 || stderr != "" || len(strings.TrimSpace(stdout)) != 40 {
		main, linked := linkedWorktreeMainCheckout(worktree)
		holderRoot := goalBranchHolderRoot(worktree)
		holder, holderErr := lease.CurrentHolder(holderRoot)
		t.Fatalf("commit from nested installation: code=%d stdout=%q stderr=%q main=%q linked=%t holder-root=%q holder=%+v holder-err=%v",
			code, stdout, stderr, main, linked, holderRoot, holder, holderErr)
	}
}

func TestGoalBranchClaimRequiresMachineAndLineage(t *testing.T) {
	f := newBranchRawFixtureWithReadReplies(t, false, false, false, "another-lineage")
	assertUnchanged := goalBranchRawCheckoutSnapshot(t, f)
	code, stdout, stderr := runOnOwnStreams(func(stdout, stderr io.Writer) int {
		return goalBranchTestCommitWith([]string{"--goal", "standing-validation", "--kind", "unit", "--unit", "u1", "--root", f.installation}, f.dependencies(), stdout, stderr)
	})
	if code != 1 || stdout != "" || !strings.Contains(stderr, branch.NotHolderCode) {
		t.Fatalf("claim mismatch: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	assertUnchanged()
	if f.ledger.accepted != strings.Repeat("0", 39)+"1" || f.ledger.canonical != strings.Repeat("0", 39)+"1" ||
		f.tip != f.unit || f.ledger.builds != 0 || f.ledger.publications != 0 || f.staged || f.published {
		t.Fatal("claim refusal changed raw goal state")
	}
	t.Run("class refusal preserves checkout", goalBranchCommitRefusalPreservesCheckout)
}

func goalBranchCommitRefusalPreservesCheckout(t *testing.T) {
	f := newBranchRawFixtureWithReadReplies(t, false, false, true)
	writeTestingFixtureFile(t, filepath.Join(f.project, "metasystem", "plans", "wrong.md"), []byte("wrong\n"), 0o644)
	dependencies := f.dependencies()
	staged := false
	dependencies.ReadInputs.Facts.Staged = func(repo string) ([]string, error) {
		if repo != f.installation || staged {
			t.Fatalf("unexpected staged paths read: %s", repo)
		}
		staged = true
		return []string{"metasystem/plans/wrong.md"}, nil
	}
	dependencies.ReadInputs.Effects.Open = func(string, string, bool) (string, func(), error) {
		t.Fatal("class refusal reached build")
		return "", nil, nil
	}
	assertUnchanged := goalBranchRawCheckoutSnapshot(t, f)
	beforeAccepted, beforeCanonical, beforeTip := f.ledger.accepted, f.ledger.canonical, f.tip
	code, stdout, stderr := runOnOwnStreams(func(stdout, stderr io.Writer) int {
		return goalBranchTestCommitWith([]string{"--goal", "standing-validation", "--kind", "unit", "--unit", "u1", "--root", f.installation}, dependencies, stdout, stderr)
	})
	if code != 1 || stdout != "" || !strings.Contains(stderr, branch.RangeCode) || !staged {
		t.Fatalf("class refusal: code=%d stdout=%q stderr=%q staged=%t", code, stdout, stderr, staged)
	}
	assertUnchanged()
	if f.ledger.accepted != beforeAccepted || f.ledger.canonical != beforeCanonical || f.tip != beforeTip ||
		f.ledger.builds != 0 || f.ledger.publications != 0 || f.published {
		t.Fatal("class refusal changed raw goal state")
	}
}

func goalBranchRawCheckoutSnapshot(t *testing.T, f *branchRawFixture) func() {
	t.Helper()
	paths := []string{".git/HEAD", ".git/index", ".git/refs/heads/goal/standing-validation", ".git/staged-record"}
	for _, path := range paths {
		writeTestingFixtureFile(t, filepath.Join(f.project, filepath.FromSlash(path)), []byte("opaque sentinel: "+path+"\n"), 0o600)
	}
	if err := os.MkdirAll(filepath.Join(f.installation, "artifacts", "agents", "mains"), 0o755); err != nil {
		t.Fatal(err)
	}
	unchanged := f.unchangedFiles(paths...)
	before := watchTreeHash(t, f.project)
	return func() {
		unchanged()
		if after := watchTreeHash(t, f.project); after != before {
			t.Fatalf("refusal changed fixture tree: before=%s after=%s", before, after)
		}
	}
}

func TestGoalBranchCommitAndPushUseEndpointRemote(t *testing.T) {
	root, _, base := goalBranchCLIFixture(t, "m1")
	writeTestingFixtureFile(t, filepath.Join(root, "metasystem", "code.go"), []byte("package fixture\n"), 0o644)
	goalSyncMutationGit(t, root, "add", "metasystem/code.go")
	code, stdout, stderr := runOnOwnStreams(func(stdout, stderr io.Writer) int {
		return goalBranchTestCommand([]string{"commit", "--goal", "standing-validation", "--kind", "unit", "--unit", "u1", "--root", root}, stdout, stderr)
	})
	tip := strings.TrimSpace(stdout)
	if code != 0 || stderr != "" || len(tip) != 40 || goalSyncMutationGit(t, root, "rev-parse", tip+"^") != base {
		t.Fatalf("commit: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	code, stdout, stderr = runOnOwnStreams(func(stdout, stderr io.Writer) int {
		return goalBranchTestCommand([]string{"push", "--goal", "standing-validation", "--root", root, "--opid", "remote-witness"}, stdout, stderr)
	})
	remote := strings.Fields(goalSyncMutationGit(t, root, "ls-remote", "--refs", "upstream", "refs/heads/goal/standing-validation"))
	if code != 0 || stderr != "" || !strings.HasPrefix(stdout, "pushed ") || len(remote) != 2 || remote[0] != tip {
		t.Fatalf("push: code=%d stdout=%q stderr=%q remote=%v", code, stdout, stderr, remote)
	}
}

func TestGoalBranchCommitAcceptsBuildUnitList(t *testing.T) {
	f := newBranchRawFixtureWithReadReplies(t, false, false, true)
	writeTestingFixtureFile(t, filepath.Join(f.project, "metasystem", "build.go"), []byte("package fixture\n"), 0o644)
	dependencies := f.dependencies()
	facts, effects := &dependencies.ReadInputs.Facts, &dependencies.ReadInputs.Effects
	facts.Tip = func(repo, ref string) (string, bool, error) {
		if repo != f.installation {
			t.Fatalf("tip repo %s", repo)
		}
		switch ref {
		case "refs/heads/goal/standing-validation":
			return f.tip, true, nil
		case "refs/metasystem/goals/origin/standing-validation":
			return "", false, nil
		default:
			t.Fatalf("tip ref %s", ref)
			return "", false, nil
		}
	}
	worktree := ""
	facts.Head = func(repo string) (string, error) {
		if repo == f.installation {
			return f.unit, nil
		}
		if repo != "" && repo == worktree {
			return f.read, nil
		}
		t.Fatalf("head repo %s", repo)
		return "", nil
	}
	facts.Index = func(repo string) (string, error) {
		if repo != f.installation {
			t.Fatalf("index repo %s", repo)
		}
		return f.tree, nil
	}
	facts.HeadRef = func(repo string) string {
		if repo != f.installation {
			t.Fatalf("head ref repo %s", repo)
		}
		return "refs/heads/goal/standing-validation"
	}
	facts.Staged = func(repo string) ([]string, error) {
		if repo != f.installation {
			t.Fatalf("staged repo %s", repo)
		}
		return []string{"metasystem/build.go"}, nil
	}
	facts.Patch = func(repo string) ([]byte, error) {
		if repo != f.installation {
			t.Fatalf("patch repo %s", repo)
		}
		return []byte("build patch\n"), nil
	}
	facts.Unstaged = func(repo string) ([]string, error) {
		if repo != f.installation {
			t.Fatalf("unstaged repo %s", repo)
		}
		return nil, nil
	}
	facts.Changes = func(repo, index, tip string) ([]string, error) {
		if repo != f.installation || index != f.tree || tip != f.read {
			t.Fatalf("changes %s %s %s", repo, index, tip)
		}
		return []string{"metasystem/build.go"}, nil
	}
	applyCalls, closed := 0, false
	effects.Open = func(repo, base string, amend bool) (string, func(), error) {
		if repo != f.installation || base != f.unit || amend || worktree != "" {
			t.Fatalf("open %s %s amend=%t", repo, base, amend)
		}
		var err error
		worktree, err = os.MkdirTemp(f.project, "build-")
		return worktree, func() { closed = true; os.RemoveAll(worktree) }, err
	}
	effects.Apply = func(dir string, patch []byte) error {
		if dir == "" || dir != worktree || string(patch) != "build patch\n" || applyCalls != 0 || closed {
			t.Fatalf("apply %s %q", dir, patch)
		}
		applyCalls++
		return nil
	}
	var subject, trailer string
	effects.Commit = func(dir, s, tr string, amend bool) error {
		if dir != worktree || amend || subject != "" || applyCalls != 1 || closed ||
			s != "goal standing-validation units 5+6+7a+7b" || tr != "Goal-Unit: standing-validation/5+6+7a+7b" {
			t.Fatalf("commit %s %q %q amend=%t", dir, s, tr, amend)
		}
		subject, trailer = s, tr
		f.add(f.base+"\n", "merge-base", f.base, f.read)
		f.add(f.unit+" "+f.base+"\n"+f.read+" "+f.unit+"\n", "rev-list", "--first-parent", "--reverse", "--parents", f.base+".."+f.read)
		f.add(trailer+"\n", "show", "-s", "--format=%(trailers:only,unfold=true)", f.read)
		f.add(f.read+" "+f.unit+"\n", "rev-list", "--parents", "-n", "1", f.read)
		f.responses[branchRawKey("diff-tree", "-r", "-z", "--no-renames", "--full-index", f.read+"^", f.read)] = branchRawEntry("metasystem/build.go")
		return nil
	}
	checkedOut := false
	effects.Checkout = func(repo, before, after string) error {
		if repo != f.installation || before != f.tree || after != f.read || checkedOut || !closed || subject == "" {
			t.Fatalf("checkout %s %s %s", repo, before, after)
		}
		checkedOut = true
		return nil
	}
	publishCalls := 0
	effects.Publish = func(repo, goalID, old, next, origin string) error {
		if repo != f.installation || goalID != "standing-validation" || old != f.unit || next != f.read || origin != "" || trailer == "" || !checkedOut || publishCalls != 0 {
			t.Fatalf("publish %s %s %s %s %s", repo, goalID, old, next, origin)
		}
		publishCalls++
		f.tip, f.published = next, true
		return nil
	}
	code, stdout, stderr := runOnOwnStreams(func(stdout, stderr io.Writer) int {
		return goalBranchTestCommitWith([]string{"--goal", "standing-validation", "--kind", "unit", "--unit", "5", "--unit", "6", "--unit", "7a", "--unit", "7b", "--root", f.installation}, dependencies, stdout, stderr)
	})
	tip := strings.TrimSpace(stdout)
	if code != 0 || stderr != "" || len(tip) != 40 || tip != f.read || f.tip != tip || applyCalls != 1 || publishCalls != 1 || !f.published || subject == "" ||
		!strings.Contains(trailer, "Goal-Unit: standing-validation/5+6+7a+7b") {
		t.Fatalf("multi-unit commit: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestGoalBranchTestingContractIncludesReadDependencies(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testing.json"))
	if err != nil {
		t.Fatal(err)
	}
	var contract struct {
		Groups []struct {
			ID, Kind string
			Inputs   []string
			Tests    json.RawMessage
		} `json:"groups"`
	}
	if err := json.Unmarshal(data, &contract); err != nil {
		t.Fatal(err)
	}
	for _, group := range contract.Groups {
		if group.ID != "goal-decision-standard" {
			continue
		}
		inputs := map[string]bool{}
		for _, input := range group.Inputs {
			inputs[input] = true
		}
		for _, required := range []string{"metasystem/internal/dispatch/**", "metasystem/internal/readsubject/**"} {
			if !inputs[required] {
				t.Fatalf("goal-decision-standard does not include %s", required)
			}
		}
		var registered []string
		if err := json.Unmarshal(group.Tests, &registered); err != nil {
			t.Fatalf("goal-decision-standard tests: %v", err)
		}
		tests := map[string]bool{}
		for _, test := range registered {
			tests[test] = true
		}
		for _, required := range []string{
			"TestAmendKeepsUnstagedTrackedEdits",
			"TestAmendRefusesBeforeOverwritingUnstagedTrackedEdit",
			"TestAmendInstallFailureRestoresCheckout",
			"TestLastArcGoalConclusionRaisesRetroDebtWhenSweepFails",
			"TestReadAdoptionKeepsUntrackedScratchFile",
		} {
			if !tests[required] {
				t.Fatalf("goal-decision-standard does not run %s", required)
			}
		}
		return
	}
	t.Fatal("goal-decision-standard is absent")
}

func TestGoalBranchCommitIsTheGuardedCommitWrapper(t *testing.T) {
	root, _, _ := goalBranchCLIFixture(t, "m1")
	exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("probe linked-worktree holder: state=%s err=%v", state, err)
	}
	if _, err := lease.AnnounceWithPair(root, "goal-branch-linked-fixture", exact.Pid, exact.StartedAt.Unix(),
		exact.StartTicks, exact.BootID, "fixture", "fake", "m1"); err != nil {
		t.Fatal(err)
	}
	holder, err := lease.ClassifyVerb(root, exact.Pid)
	if err != nil || holder.Class != lease.ClassMain || !holder.Holder || holder.Announcement == nil || holder.Announcement.OwnerLineage != "m1" {
		t.Fatalf("linked-worktree caller is not the m1 MAIN holder: holder=%+v err=%v", holder, err)
	}
	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	moduleRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	build := exec.Command("go", "build", "-o", filepath.Join(bin, "metasystem"), "./cmd/metasystem")
	build.Dir = moduleRoot
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build fixture engine: %s: %v", output, err)
	}
	// The hook runs the built engine's pre-commit entry: the production guard.
	common := goalSyncMutationGit(t, root, "rev-parse", "--path-format=absolute", "--git-common-dir")
	hook := filepath.Join(common, "hooks", "pre-commit")
	writeTestingFixtureFile(t, hook, []byte("#!/bin/sh\nexec "+shellQuote(filepath.Join(bin, "metasystem"))+" internal pre-commit --root "+shellQuote(root)+"\n"), 0o755)

	product := filepath.Join(root, "metasystem", "guarded.go")
	writeTestingFixtureFile(t, product, []byte("package fixture\n"), 0o644)
	goalSyncMutationGit(t, root, "add", "metasystem/guarded.go")
	raw := exec.Command("git", "-C", root, "commit", "-m", "raw commit must refuse")
	output, rawErr := raw.CombinedOutput()
	if rawErr == nil || !strings.Contains(string(output), "live wrapper ancestry token is missing") {
		t.Fatalf("raw git commit err=%v output=%q", rawErr, output)
	}
	goalSyncMutationGit(t, root, "add", "-u")
	code, stdout, stderr := runOnOwnStreams(func(stdout, stderr io.Writer) int {
		return goalBranchTestCommand([]string{"commit", "--goal", "standing-validation", "--kind", "unit", "--unit", "guard", "--root", root}, stdout, stderr)
	})
	if code != 0 || len(strings.TrimSpace(stdout)) != 40 || stderr != "" {
		t.Fatalf("guarded verb: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(root, "artifacts", "agents", "mains", "worktree-commit-token.json")); !os.IsNotExist(err) {
		t.Fatalf("wrapper token survived: %v", err)
	}
}

func TestGoalBranchLastLandingSweepsGoalBranch(t *testing.T) {
	root, _, base := goalBranchCLIFixture(t, "m1")
	writeTestingFixtureFile(t, filepath.Join(root, "metasystem", "landed.go"), []byte("package fixture\n"), 0o644)
	goalSyncMutationGit(t, root, "add", "metasystem/landed.go")
	code, stdout, stderr := runOnOwnStreams(func(stdout, stderr io.Writer) int {
		return goalBranchTestCommand([]string{"commit", "--goal", "standing-validation", "--kind", "unit", "--unit", "last", "--root", root}, stdout, stderr)
	})
	unit := strings.TrimSpace(stdout)
	if code != 0 || stderr != "" || len(unit) != 40 {
		t.Fatalf("unit commit: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	code, _, stderr = runOnOwnStreams(func(stdout, stderr io.Writer) int {
		return goalBranchTestCommand([]string{"push", "--goal", "standing-validation", "--root", root, "--opid", "last-push"}, stdout, stderr)
	})
	if code != 0 || stderr != "" {
		t.Fatalf("goal push: code=%d stderr=%q", code, stderr)
	}
	digest, err := branch.UnitDigest(root, unit)
	if err != nil {
		t.Fatal(err)
	}
	message := "land goal standing-validation\n\nGoal-Unit: standing-validation/last\nGoal-Digest: " + digest + "\nGoal-Source: " + unit + "\nGoal-Last: standing-validation\nLanded-By: fixture\n"
	landing := goalSyncMutationGit(t, root, "commit-tree", unit+"^{tree}", "-p", base, "-m", message)
	goalSyncMutationGit(t, root, "push", "-q", "upstream", landing+":refs/heads/landing/standing-validation")
	prepared := t.TempDir()
	trunk := "endpoint=" + base + "\ncandidate=" + goalSyncMutationGit(t, root, "rev-parse", landing+"^{tree}") + "\nlanding=" + landing + "\nbranch=landing/standing-validation\n"
	writeTestingFixtureFile(t, filepath.Join(prepared, "trunk"), []byte(trunk), 0o644)
	code, stdout, stderr = runOnOwnStreams(func(stdout, stderr io.Writer) int {
		return goalBranchTestCommand([]string{"land-push", "--goal", "standing-validation", "--prepared", prepared, "--root", root}, stdout, stderr)
	})
	if code != 0 || stderr != "" || !strings.Contains(stdout, "landed "+landing) {
		t.Fatalf("land-push: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if refs := goalSyncMutationGit(t, root, "ls-remote", "--refs", "upstream", "refs/heads/goal/standing-validation", "refs/heads/landing/standing-validation"); refs != "" {
		t.Fatalf("last landing left branches: %s", refs)
	}
}

func TestGoalBranchLandPrepRoutesLocalRedCandidate(t *testing.T) {
	root, _, _ := goalBranchCLIFixtureBelow(t, "m1", "metasystem")
	goalSyncMutationGit(t, root, "config", "goal.human.Wido", "Wido Approver <wido@example.invalid>")
	pagePath := filepath.Join(root, "plans", "goals", "standing-validation.md")
	pageData, err := os.ReadFile(pagePath)
	if err != nil {
		t.Fatal(err)
	}
	file, problems := goal.ParseFile(pageData)
	if len(problems) != 0 {
		t.Fatalf("parse goal page: %v", problems)
	}
	landReadyOpid := goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FB0", "mac-cli", "m1")
	file.Revision++
	file.Landing = &goal.LandingRecord{At: "2026-09-17T09:00:00Z", Opid: landReadyOpid}
	file.History = append(file.History, goal.HistoryLine{At: file.Landing.At, Opid: landReadyOpid,
		Verb: "land-ready", Actor: "mac-cli+m1", Targets: []string{file.Id}, Keep: -1})
	writeTestingFixtureFile(t, pagePath, goal.RenderFile(file), 0o644)
	writeTestingFixtureFile(t, filepath.Join(root, "memory", "receipts.log"),
		[]byte("1|1970-01-01T00:00:00Z|RECEIPT|type=seed|outcome=shipped\n"), 0o644)
	goalSyncMutationGit(t, root, "add", "plans/goals/standing-validation.md", "memory/receipts.log")
	goalSyncMutationGit(t, root, "commit", "-qm", "mark fixture land ready")
	base := goalSyncMutationGit(t, root, "rev-parse", "HEAD")
	goalSyncMutationGit(t, root, "push", "-q", "upstream", "HEAD:main")
	goalSyncMutationGit(t, root, "update-ref", goal.LocalLedgerBranch, base)
	goalSyncMutationGit(t, root, "update-ref", goal.AcceptedRef, base)
	claim := func() error { return nil }
	writeTestingFixtureFile(t, filepath.Join(root, "owned.go"), []byte("package fixture\n"), 0o644)
	goalSyncMutationGit(t, root, "add", "owned.go")
	unit, err := branch.CommitStaged(branch.CommitRequest{Repo: root, Remote: "upstream", EndpointTip: base,
		GoalID: "standing-validation", Unit: "u1", OpID: "red-command-unit", Kind: branch.Unit, CheckClaim: claim})
	if err != nil {
		t.Fatal(err)
	}
	digest, err := branch.UnitDigest(root, unit)
	if err != nil {
		t.Fatal(err)
	}
	readerRecord := "metasystem/records/misc/red-command-read.md"
	writeTestingFixtureFile(t, filepath.Join(filepath.Dir(root), filepath.FromSlash(readerRecord)), []byte(unit+" "+digest+"\n"), 0o644)
	unitTree := goalSyncMutationGit(t, root, "rev-parse", unit+"^{tree}")
	if _, _, err := branch.CommitRead(branch.CommitReadRequest{Repo: root, Remote: "upstream", EndpointTip: base,
		GoalID: "standing-validation", Unit: "u1", OpID: "red-command-read", ReaderRecord: readerRecord,
		GateRunID: "fast-clean", GateTree: unitTree, CheckClaim: claim}); err != nil {
		t.Fatal(err)
	}
	writeTestingFixtureFile(t, filepath.Join(root, "plans", "fold.md"), []byte("folded plan\n"), 0o644)
	goalSyncMutationGit(t, root, "add", "plans/fold.md")
	if _, err := branch.CommitStaged(branch.CommitRequest{Repo: root, Remote: "upstream", EndpointTip: base,
		GoalID: "standing-validation", OpID: "red-command-fold", Kind: branch.Plan, CheckClaim: claim}); err != nil {
		t.Fatal(err)
	}
	if _, err := branch.Push(branch.PushRequest{Repo: root, Remote: "upstream", EndpointTip: base,
		GoalID: "standing-validation", OpID: "red-command-push", CheckClaim: claim}); err != nil {
		t.Fatal(err)
	}
	branchTip := goalSyncMutationGit(t, root, "rev-parse", "refs/heads/goal/standing-validation")

	endpointTip := base
	status, statusErr := branch.InspectStatus(root, endpointTip, branchTip, "standing-validation")
	if statusErr != nil || status.Prefix != 1 {
		t.Fatalf("red command branch status = %+v err=%v", status, statusErr)
	}

	discoveryReceipt := filepath.Join(t.TempDir(), "discovery.json")
	writeTestingFixtureFile(t, discoveryReceipt, []byte(`{"schemaVersion":3,"tree":"0000000000000000000000000000000000000000","exitStatus":1,"time":"2026-09-17T10:00:00Z","proof":{"attemptId":"discover"},"testing":{"attemptId":"discover","delivery":{"failingGroups":["folded"]}}}`), 0o644)
	_, discoveryErr := branch.PrepareLanding(branch.LandRequest{Repo: root, Remote: "upstream", EndpointTip: endpointTip,
		BranchTip: branchTip, GoalID: "standing-validation", Out: filepath.Join(t.TempDir(), "discovery"),
		TestReceipt: discoveryReceipt, Last: true, LandingReady: true, GoalPage: string(goal.RenderFile(file)), ApprovedBy: file.Approved.By,
		Seat: "mac-cli", CheckClaim: claim})
	const marker = "candidate workspace "
	markerAt := strings.LastIndex(fmt.Sprint(discoveryErr), marker)
	if markerAt < 0 {
		t.Fatalf("candidate identity discovery = %v", discoveryErr)
	}
	projected := strings.TrimSpace(fmt.Sprint(discoveryErr)[markerAt+len(marker):])
	if len(projected) != 40 {
		t.Fatalf("candidate identity = %q", projected)
	}

	contract := testpolicy.Contract{Groups: []testpolicy.Group{
		{ID: "folded", Inputs: []string{"metasystem/plans/fold.md"}},
		{ID: "outside", Inputs: []string{"outside/**"}},
	}}
	runner := &commandRedRunner{}
	progress := &commandLandingProgress{}
	admissions := 0
	dependencies := goalBranchLandPrepDependencies{
		Prepare:         branch.PrepareLanding,
		LoadContract:    func(string) (testpolicy.Contract, error) { return contract, nil },
		AdmitDiagnostic: func() error { admissions++; return nil }, Runner: runner, Progress: progress,
	}
	landingBefore := goalSyncMutationGit(t, root, "ls-remote", "--refs", "upstream", "refs/heads/landing/standing-validation")
	for index, group := range []string{"folded", "outside"} {
		receiptPath := filepath.Join(t.TempDir(), group+".json")
		body := fmt.Sprintf(`{"schemaVersion":3,"tree":%q,"exitStatus":1,"time":"2026-09-17T10:00:0%dZ","proof":{"attemptId":%q},"testing":{"attemptId":%q,"delivery":{"failingGroups":[%q]}}}`,
			projected, index, "red-"+group, "red-"+group, group)
		writeTestingFixtureFile(t, receiptPath, []byte(body+"\n"), 0o644)
		out := filepath.Join(t.TempDir(), "red-out")
		code, stdout, stderr := runOnOwnStreams(func(stdout, stderr io.Writer) int {
			return runGoalBranchLandPrepWith([]string{"--goal", "standing-validation", "--root", root,
				"--out", out, "--test-receipt", receiptPath, "--last"}, dependencies, stdout, stderr)
		})
		if code != 0 || stderr != "" || !strings.Contains(stdout, "classification=goal-red") {
			t.Fatalf("%s red command: code=%d stdout=%q stderr=%q", group, code, stdout, stderr)
		}
		if _, err := os.Stat(out); !os.IsNotExist(err) {
			t.Fatalf("red command wrote output %s: %v", out, err)
		}
	}
	if len(progress.lines) != 2 || len(runner.runs) != 1 || runner.runs[0].Tree != endpointTip ||
		runner.runs[0].Groups[0] != "outside" || admissions != 1 {
		t.Fatalf("red routing progress=%v runs=%+v admissions=%d", progress.lines, runner.runs, admissions)
	}
	for _, line := range progress.lines {
		fields := strings.Fields(line)
		landing := ""
		for _, field := range fields {
			if strings.HasPrefix(field, "landing=") {
				landing = strings.TrimPrefix(field, "landing=")
			}
		}
		if len(landing) != 40 {
			t.Fatalf("red proof has no local landing commit: %s", line)
		}
		goalSyncMutationGit(t, root, "cat-file", "-e", landing+"^{commit}")
	}
	landingAfter := goalSyncMutationGit(t, root, "ls-remote", "--refs", "upstream", "refs/heads/landing/standing-validation")
	if landingBefore != landingAfter {
		t.Fatalf("red command moved landing ref: before=%q after=%q", landingBefore, landingAfter)
	}
}
