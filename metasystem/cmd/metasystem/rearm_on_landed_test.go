package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

func TestLandedRearmDecidesFromTheThreeFacts(t *testing.T) {
	base := landedRearmFacts{LandingRef: "refs/remotes/origin/main", Tip: strings.Repeat("b", 40), Head: strings.Repeat("a", 40), Source: strings.Repeat("a", 40)}
	owns := base
	owns.SourceOwnsTip = true
	if d := decideLandedRearm(owns, "/c"); d.Rearm || d.Refusal != "" {
		t.Fatalf("an engine that owns the tip was not left alone: %+v", d)
	}
	landed := base
	landed.HeadIsAncestor = true
	if d := decideLandedRearm(landed, "/c"); !d.Rearm || d.Refusal != "" {
		t.Fatalf("a landed-only skew was not re-armed: %+v", d)
	}
	diverged := base
	diverged.HeadIsAncestor = false
	d := decideLandedRearm(diverged, "/c")
	if d.Rearm || !strings.Contains(d.Refusal, "TEST_POLICY_ENGINE_REQUIRED") || !strings.Contains(d.Refusal, base.Source) ||
		!strings.Contains(d.Refusal, base.Tip) || !strings.Contains(d.Refusal, "cause=engine-behind-tip") || !strings.Contains(d.Refusal, "fact=head-diverged") ||
		!strings.Contains(d.Refusal, "not an ancestor") || !strings.Contains(d.Refusal, landedRearmCommand("/c")) {
		t.Fatalf("a diverged checkout was not refused with the two commits and the command: %+v", d)
	}
	dirty := landed
	dirty.DirtyEnginePaths = []string{"metasystem/cmd/metasystem/main.go"}
	d = decideLandedRearm(dirty, "/c")
	if d.Rearm || !strings.Contains(d.Refusal, "fact=dirty-engine-paths") || !strings.Contains(d.Refusal, "dirty in engine inputs (metasystem/cmd/metasystem/main.go)") || !strings.Contains(d.Refusal, landedRearmCommand("/c")) {
		t.Fatalf("a checkout dirty in an engine input was not refused naming the path and the command: %+v", d)
	}
	// A delivery run that names the exact index it proves keeps the manual
	// path: a fast-forward would move that index under its receipt.
	named := landed
	named.NamedDeliveryTree = true
	d = decideLandedRearm(named, "/c")
	if d.Rearm || !strings.Contains(d.Refusal, "fact=named-delivery-tree") || !strings.Contains(d.Refusal, "names the exact index it proves") || !strings.Contains(d.Refusal, landedRearmCommand("/c")) {
		t.Fatalf("a delivery run naming its tree was not kept on the manual path: %+v", d)
	}
	// The engine is never rebuilt under a live attempt of this installation.
	busy := landed
	busy.LiveAttempts = []string{"proof-live-1"}
	d = decideLandedRearm(busy, "/c")
	if d.Rearm || !strings.Contains(d.Refusal, "fact=live-attempt") || !strings.Contains(d.Refusal, "live (proof-live-1)") {
		t.Fatalf("a rebuild under a live attempt was not refused: %+v", d)
	}
}

type landedRearmFixture struct {
	remote, seed, projectRoot, installation string
}

func readLandedRearmFactsForTest(ctx context.Context, installation, projectRoot, prefix, source string, ownsTip func(string) bool) (landedRearmFacts, error) {
	return readLandedRearmFacts(ctx, steward.SystemRearmClock(), 20, installation, projectRoot, prefix, source, func(tip string) (bool, error) {
		return ownsTip(tip), nil
	})
}

func landedGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", dir, "-c", "core.hooksPath=/dev/null", "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid",
		"-c", "clone.defaultRemoteName=origin", "-c", "init.defaultBranch=main", "-c", "diff.relative=false"}, args...)...)
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func failLandedGitSubcommand(t *testing.T, ctx context.Context, subcommand string) context.Context {
	return failLandedGitSubcommandWithStatus(t, ctx, subcommand, 128)
}

func failLandedGitSubcommandWithStatus(t *testing.T, ctx context.Context, subcommand string, status int) context.Context {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	wrapper := filepath.Join(dir, "git")
	script := `#!/usr/bin/env bash
set -euo pipefail
for argument in "$@"; do
  if [[ "$argument" == "${LANDED_REARM_FAIL_GIT_SUBCOMMAND:?}" ]]; then
    printf '%s\n' 'fatal: not a git repository (or any of the parent directories)' >&2
    exit "${LANDED_REARM_FAIL_GIT_STATUS:?}"
  fi
done
exec "${LANDED_REARM_REAL_GIT:?}" "$@"
`
	if err := testexec.WriteFile(wrapper, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	factory := landedRearmGitCommandFactory(func(commandContext context.Context, args ...string) *exec.Cmd {
		command := exec.CommandContext(commandContext, wrapper, args...)
		command.Env = append(gittree.ScrubbedEnviron(),
			"LANDED_REARM_REAL_GIT="+realGit,
			"LANDED_REARM_FAIL_GIT_SUBCOMMAND="+subcommand,
			"LANDED_REARM_FAIL_GIT_STATUS="+fmt.Sprint(status))
		return command
	})
	return context.WithValue(ctx, landedRearmGitCommandContextKey{}, factory)
}

func stallLandedGitSubcommandOnce(t *testing.T, ctx context.Context, subcommand string) (context.Context, string) {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	wrapper := filepath.Join(dir, "git")
	marker := filepath.Join(dir, "stalled")
	started := filepath.Join(dir, "started")
	release := filepath.Join(dir, "release")
	if err := syscall.Mkfifo(started, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(release, 0o600); err != nil {
		t.Fatal(err)
	}
	script := `#!/usr/bin/env bash
set -euo pipefail
for argument in "$@"; do
  if [[ "$argument" == "${LANDED_REARM_STALL_GIT_SUBCOMMAND:?}" && ! -e "${LANDED_REARM_GIT_STALLED_ONCE:?}" ]]; then
    : >"${LANDED_REARM_GIT_STALLED_ONCE:?}"
    printf started >"${LANDED_REARM_GIT_STARTED_FIFO:?}"
    IFS= read -r answer <"${LANDED_REARM_GIT_RELEASE_FIFO:?}"
  fi
done
exec "${LANDED_REARM_REAL_GIT:?}" "$@"
`
	if err := testexec.WriteFile(wrapper, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	factory := landedRearmGitCommandFactory(func(commandContext context.Context, args ...string) *exec.Cmd {
		command := exec.CommandContext(commandContext, wrapper, args...)
		command.Env = append(gittree.ScrubbedEnviron(),
			"LANDED_REARM_REAL_GIT="+realGit,
			"LANDED_REARM_STALL_GIT_SUBCOMMAND="+subcommand,
			"LANDED_REARM_GIT_STALLED_ONCE="+marker,
			"LANDED_REARM_GIT_STARTED_FIFO="+started,
			"LANDED_REARM_GIT_RELEASE_FIFO="+release)
		return command
	})
	return context.WithValue(ctx, landedRearmGitCommandContextKey{}, factory), started
}

// newLandedRearmFixture is a checkout with its installation under
// metasystem/ and a local remote whose main is the landing ref; the remote
// then lands a commit that changes an engine input (a fake landing).
func newLandedRearmFixture(t *testing.T) landedRearmFixture {
	t.Helper()
	root, err := canonicalPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	remote := filepath.Join(root, "remote.git")
	landedGit(t, root, "init", "-q", "--bare", "-b", "main", remote)
	seed := filepath.Join(root, "seed")
	landedGit(t, root, "init", "-q", "-b", "main", seed)
	for _, item := range []struct{ path, content string }{
		{"metasystem/metasystem.conf", "testing.contract=testing.json\n"},
		{"metasystem/cmd/metasystem/main.go", "package main\n"},
		{"metasystem/docs/notes.md", "notes\n"},
		{"metasystem/memory/receipts.log", "receipt=seed\n"},
		{"metasystem/records/narrator-digest.log", "digest=seed\n"},
	} {
		path := filepath.Join(seed, item.path)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(item.content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	landedGit(t, seed, "add", ".")
	landedGit(t, seed, "commit", "-qm", "seed")
	landedGit(t, seed, "remote", "add", "origin", remote)
	landedGit(t, seed, "push", "-q", "origin", "main")
	projectRoot := filepath.Join(root, "checkout")
	landedGit(t, root, "clone", "-q", remote, projectRoot)
	landedGit(t, projectRoot, "config", "metasystem.steward.landing-ref", "refs/remotes/origin/main")
	// The fake landing: the remote's main gains a commit that changes an
	// engine input; the checkout has not fetched it.
	if err := os.WriteFile(filepath.Join(seed, "metasystem", "cmd", "metasystem", "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	landedGit(t, seed, "add", ".")
	landedGit(t, seed, "commit", "-qm", "landed engine change")
	landedGit(t, seed, "push", "-q", "origin", "main")
	return landedRearmFixture{remote: remote, seed: seed, projectRoot: projectRoot, installation: filepath.Join(projectRoot, "metasystem")}
}

func landFixturePath(t *testing.T, fixture landedRearmFixture, path, content string) {
	t.Helper()
	seedPath := filepath.Join(fixture.seed, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(seedPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(seedPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	landedGit(t, fixture.seed, "add", ".")
	landedGit(t, fixture.seed, "commit", "-qm", "land path")
	landedGit(t, fixture.seed, "push", "-q", "origin", "main")
}

func TestLandedRearmReadsTheCheckoutAgainstItsRemote(t *testing.T) {
	fixture := newLandedRearmFixture(t)
	head := landedGit(t, fixture.projectRoot, "rev-parse", "HEAD")
	notOwned := func(string) bool { return false }
	facts, err := readLandedRearmFactsForTest(context.Background(), fixture.installation, fixture.projectRoot, "metasystem", head, notOwned)
	if err != nil {
		t.Fatal(err)
	}
	remoteTip := landedGit(t, fixture.remote, "rev-parse", "main")
	if facts.Remote != "origin" || facts.Branch != "main" || facts.Tip != remoteTip || facts.Head != head || facts.Tip == head {
		t.Fatalf("the fetch did not bring the landing ref to the remote's tip: %+v (remote main %s)", facts, remoteTip)
	}
	if !facts.HeadIsAncestor || len(facts.DirtyEnginePaths) != 0 || facts.SourceOwnsTip {
		t.Fatalf("a clean checkout behind the tip was not read as landed-only: %+v", facts)
	}
	// An engine that owns the tip needs no other fact.
	owned, err := readLandedRearmFactsForTest(context.Background(), fixture.installation, fixture.projectRoot, "metasystem", head, func(string) bool { return true })
	if err != nil || !owned.SourceOwnsTip {
		t.Fatalf("an engine owning the tip was not reported so: %+v %v", owned, err)
	}
	// A change outside the ENGINE projection is not dirt; one inside is.
	if err := os.WriteFile(filepath.Join(fixture.installation, "docs", "notes.md"), []byte("notes, edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	facts, err = readLandedRearmFactsForTest(context.Background(), fixture.installation, fixture.projectRoot, "metasystem", head, notOwned)
	if err != nil || len(facts.DirtyEnginePaths) != 0 {
		t.Fatalf("a docs edit counted as engine dirt: %+v %v", facts.DirtyEnginePaths, err)
	}
	if err := os.WriteFile(filepath.Join(fixture.installation, "cmd", "metasystem", "extra.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	facts, err = readLandedRearmFactsForTest(context.Background(), fixture.installation, fixture.projectRoot, "metasystem", head, notOwned)
	if err != nil || strings.Join(facts.DirtyEnginePaths, ",") != "metasystem/cmd/metasystem/extra.go" {
		t.Fatalf("an untracked engine file was not listed as dirt: %+v %v", facts.DirtyEnginePaths, err)
	}
	if err := os.Remove(filepath.Join(fixture.installation, "cmd", "metasystem", "extra.go")); err != nil {
		t.Fatal(err)
	}
	// A local commit the remote lacks: the checkout is not an ancestor.
	landedGit(t, fixture.projectRoot, "commit", "-qam", "local only")
	facts, err = readLandedRearmFactsForTest(context.Background(), fixture.installation, fixture.projectRoot, "metasystem", head, notOwned)
	if err != nil || facts.HeadIsAncestor {
		t.Fatalf("a checkout with a local commit was read as an ancestor of the tip: %+v %v", facts, err)
	}
	// A landing ref that cannot be fetched is judged as last fetched: an
	// engine that owns it runs, a re-arm from it is refused.
	landedGit(t, fixture.projectRoot, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "absent.git"))
	facts, err = readLandedRearmFactsForTest(context.Background(), fixture.installation, fixture.projectRoot, "metasystem", head, notOwned)
	if err != nil || facts.FetchErr == nil || facts.Tip != remoteTip {
		t.Fatalf("a failed fetch was not recorded against the last fetched tip: %+v %v", facts, err)
	}
	if d := decideLandedRearm(facts, fixture.projectRoot); d.Rearm || !strings.Contains(d.Refusal, "fact=fetch-failed") || !strings.Contains(d.Refusal, "could not be fetched") {
		t.Fatalf("a re-arm from an unfetchable tip was not refused: %+v", d)
	}
	if owned, err := readLandedRearmFactsForTest(context.Background(), fixture.installation, fixture.projectRoot, "metasystem", head, func(string) bool { return true }); err != nil || !owned.SourceOwnsTip {
		t.Fatalf("an engine owning the last fetched tip was refused for the fetch: %+v %v", owned, err)
	}
}

func TestLandedRearmFactsFailOnARepositoryFailureInsteadOfJudgingHeadUnlanded(t *testing.T) {
	fixture := newLandedRearmFixture(t)
	head := landedGit(t, fixture.projectRoot, "rev-parse", "HEAD")
	notOwned := func(string) bool { return false }
	facts, err := readLandedRearmFactsForTest(context.Background(), fixture.installation, fixture.projectRoot, "metasystem", head, notOwned)
	if err != nil || !facts.HeadIsAncestor {
		t.Fatalf("known ancestor did not produce a true fact: facts=%+v err=%v", facts, err)
	}

	if err := os.WriteFile(filepath.Join(fixture.installation, "cmd", "metasystem", "main.go"), []byte("package main\n// local failure-test commit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	landedGit(t, fixture.projectRoot, "commit", "-qam", "local only")
	facts, err = readLandedRearmFactsForTest(context.Background(), fixture.installation, fixture.projectRoot, "metasystem", head, notOwned)
	if err != nil || facts.HeadIsAncestor {
		t.Fatalf("exit status 1 did not remain a false fact without an error: facts=%+v err=%v", facts, err)
	}

	ctx := failLandedGitSubcommand(t, context.Background(), "merge-base")
	_, failureErr := readLandedRearmFactsForTest(ctx, fixture.installation, fixture.projectRoot, "metasystem", head, notOwned)
	if failureErr == nil || !strings.Contains(failureErr.Error(), "git merge-base --is-ancestor") ||
		!strings.Contains(failureErr.Error(), "fatal: not a git repository (or any of the parent directories)") ||
		errors.Is(failureErr, steward.ErrNotOwned) || errors.Is(failureErr, steward.ErrJudgmentStalled) {
		t.Fatalf("repository failure became a false ancestry fact: %v", failureErr)
	}
}

func TestLandedRearmRefusesARepositoryFailureReadingTheLandingRefWithGitDetail(t *testing.T) {
	t.Parallel()
	t.Run("negative and malformed values keep the shape message", func(t *testing.T) {
		fixture := newLandedRearmFixture(t)
		want := "trusted testing policy base requires local metasystem.steward.landing-ref shaped refs/remotes/<remote>/<branch>"
		for _, value := range []string{"", "refs/heads/main"} {
			if value == "" {
				landedGit(t, fixture.projectRoot, "config", "--unset-all", "metasystem.steward.landing-ref")
			} else {
				landedGit(t, fixture.projectRoot, "config", "metasystem.steward.landing-ref", value)
			}
			if _, _, _, err := landingRefParts(context.Background(), steward.SystemRearmClock(), 20, fixture.projectRoot); err == nil || err.Error() != want {
				t.Fatalf("landing-ref value %q changed its shape result: %v", value, err)
			}
		}
	})

	t.Run("repository failure", func(t *testing.T) {
		fixture := newLandedRearmFixture(t)
		head := landedGit(t, fixture.projectRoot, "rev-parse", "HEAD")
		ctx := failLandedGitSubcommand(t, context.Background(), "config")
		_, err := readLandedRearmFactsForTest(ctx, fixture.installation, fixture.projectRoot, "metasystem", head, func(string) bool { return false })
		refusal := judgmentRefusal(err, engineCheckoutFacts(fixture.projectRoot), "judging the enrolled engine against the landed tip failed")
		if err == nil || !strings.Contains(refusal.Error(), "cause=judgment-failed") || !strings.Contains(refusal.Error(), "git config") ||
			!strings.Contains(refusal.Error(), "fatal: not a git repository (or any of the parent directories)") ||
			strings.Contains(refusal.Error(), "trusted testing policy base requires local metasystem.steward.landing-ref shaped refs/remotes/<remote>/<branch>") {
			t.Fatalf("landing-ref repository failure became a landing-ref shape judgment: %v", refusal)
		}
	})

	t.Run("stall", func(t *testing.T) {
		fixture := newLandedRearmFixture(t)
		head := landedGit(t, fixture.projectRoot, "rev-parse", "HEAD")
		ctx, started := stallLandedGitSubcommandOnce(t, context.Background(), "config")
		clock := newScheduledRearmClock()
		result := make(chan error, 1)
		go func() {
			_, err := readLandedRearmFacts(ctx, clock, 20, fixture.installation, fixture.projectRoot, "metasystem", head, func(string) (bool, error) { return false, nil })
			result <- err
		}()
		if _, err := os.ReadFile(started); err != nil {
			t.Fatal(err)
		}
		clock.advance(21 * time.Second)
		err := <-result
		refusal := judgmentRefusal(err, engineCheckoutFacts(fixture.projectRoot), "judging the enrolled engine against the landed tip failed")
		if err == nil || !errors.Is(err, steward.ErrJudgmentStalled) ||
			!strings.Contains(refusal.Error(), "cause=judgment-stalled step=read-landing-ref seconds=20") ||
			strings.Contains(refusal.Error(), "trusted testing policy base requires local metasystem.steward.landing-ref shaped refs/remotes/<remote>/<branch>") {
			t.Fatalf("landing-ref stall became a landing-ref shape judgment: %v", refusal)
		}
	})
}

func TestLandedRearmRefusesARepositoryFailureResolvingCheckoutHeadWithGitDetail(t *testing.T) {
	t.Parallel()
	t.Run("negative answer keeps the no-HEAD message", func(t *testing.T) {
		fixture := newLandedRearmFixture(t)
		head := landedGit(t, fixture.projectRoot, "rev-parse", "HEAD")
		ctx := failLandedGitSubcommandWithStatus(t, context.Background(), "HEAD^{commit}", 1)
		_, err := readLandedRearmFactsForTest(ctx, fixture.installation, fixture.projectRoot, "metasystem", head, func(string) bool { return false })
		if err == nil || err.Error() != "testing requires a committed project HEAD" {
			t.Fatalf("checkout-HEAD negative answer changed its no-HEAD result: %v", err)
		}
	})

	t.Run("repository failure", func(t *testing.T) {
		fixture := newLandedRearmFixture(t)
		head := landedGit(t, fixture.projectRoot, "rev-parse", "HEAD")
		ctx := failLandedGitSubcommand(t, context.Background(), "HEAD^{commit}")
		_, err := readLandedRearmFactsForTest(ctx, fixture.installation, fixture.projectRoot, "metasystem", head, func(string) bool { return false })
		refusal := judgmentRefusal(err, engineCheckoutFacts(fixture.projectRoot), "judging the enrolled engine against the landed tip failed")
		if err == nil || !strings.Contains(refusal.Error(), "cause=judgment-failed") || !strings.Contains(refusal.Error(), "resolve-checkout-head") ||
			!strings.Contains(refusal.Error(), "git rev-parse --verify HEAD^{commit}") ||
			!strings.Contains(refusal.Error(), "fatal: not a git repository (or any of the parent directories)") ||
			strings.Contains(refusal.Error(), "testing requires a committed project HEAD") {
			t.Fatalf("checkout-HEAD repository failure became a no-HEAD judgment: %v", refusal)
		}
	})

	t.Run("stall", func(t *testing.T) {
		fixture := newLandedRearmFixture(t)
		head := landedGit(t, fixture.projectRoot, "rev-parse", "HEAD")
		ctx, started := stallLandedGitSubcommandOnce(t, context.Background(), "HEAD^{commit}")
		clock := newScheduledRearmClock()
		result := make(chan error, 1)
		go func() {
			_, err := readLandedRearmFacts(ctx, clock, 20, fixture.installation, fixture.projectRoot, "metasystem", head, func(string) (bool, error) { return false, nil })
			result <- err
		}()
		if _, err := os.ReadFile(started); err != nil {
			t.Fatal(err)
		}
		clock.advance(21 * time.Second)
		err := <-result
		refusal := judgmentRefusal(err, engineCheckoutFacts(fixture.projectRoot), "judging the enrolled engine against the landed tip failed")
		if err == nil || !errors.Is(err, steward.ErrJudgmentStalled) ||
			!strings.Contains(refusal.Error(), "cause=judgment-stalled step=resolve-checkout-head seconds=20") ||
			strings.Contains(refusal.Error(), "testing requires a committed project HEAD") {
			t.Fatalf("checkout-HEAD stall became a no-HEAD judgment: %v", refusal)
		}
	})
}

func TestLandedRearmFastForwardsRebuildsAndReArms(t *testing.T) {
	fixture := newLandedRearmFixture(t)
	head := landedGit(t, fixture.projectRoot, "rev-parse", "HEAD")
	facts, err := readLandedRearmFactsForTest(context.Background(), fixture.installation, fixture.projectRoot, "metasystem", head, func(string) bool { return false })
	if err != nil {
		t.Fatal(err)
	}
	var rebuiltIn, upInstallation, upScope string
	locked := 0
	previousFastForward, previousRebuild, previousUp, previousOpen, previousLock := landedRearmFastForward, landedRearmRebuild, landedRearmUp, landedRearmOpenEnrollment, landedRearmMutationLock
	upResult := upOutcome{Line: "up outcome=armed authority=writer re-armed=\"generation=2 previous=1\"", Outcome: "armed"}
	var upErr error
	enrolledGeneration := 2
	landedRearmRebuild = func(_ context.Context, installation string) error { rebuiltIn = installation; return nil }
	landedRearmUp = func(_ context.Context, installation, projectRoot string) (upOutcome, error) {
		upInstallation, upScope = installation, projectRoot
		return upResult, upErr
	}
	landedRearmOpenEnrollment = func(string) (steward.InstallIdentity, error) {
		return steward.InstallIdentity{Generation: enrolledGeneration}, nil
	}
	landedRearmMutationLock = func(string) (func(), error) { locked++; return func() { locked-- }, nil }
	t.Cleanup(func() {
		landedRearmFastForward, landedRearmRebuild, landedRearmUp, landedRearmOpenEnrollment, landedRearmMutationLock = previousFastForward, previousRebuild, previousUp, previousOpen, previousLock
	})
	record, err := landedRearmAct(context.Background(), fixture.installation, fixture.projectRoot, facts, 1)
	if err != nil {
		t.Fatal(err)
	}
	if locked != 0 {
		t.Fatalf("the proof mutation lock was not released: %d", locked)
	}
	if got := landedGit(t, fixture.projectRoot, "rev-parse", "HEAD"); got != facts.Tip {
		t.Fatalf("the checkout was not fast-forwarded to the tip: HEAD %s tip %s", got, facts.Tip)
	}
	if rebuiltIn != fixture.installation || upInstallation != fixture.installation || upScope != fixture.projectRoot {
		t.Fatalf("the rebuild or the re-arm ran elsewhere: rebuild=%s up=%s scope=%s", rebuiltIn, upInstallation, upScope)
	}
	if record == nil || record.SourceCommit != head || record.LandedTip != facts.Tip || record.PreviousGeneration != 1 || record.Generation != 2 ||
		record.ReArmed != upResult.Line || record.UpOutcome != "armed" || record.At == "" {
		t.Fatalf("the re-arm record is incomplete: %+v", record)
	}
	// up mints first and proves the session after: a detached run gets a
	// failed up over an enrollment that did advance, and continues on it.
	upResult = upOutcome{Line: "up outcome=failed failed=session-identity remedy=no runtime ancestor", Outcome: "failed", Failed: true}
	upErr = errors.New("bin/metasystem up --repo x: exit status 1: no runtime ancestor")
	enrolledGeneration = 3
	record, err = landedRearmAct(context.Background(), fixture.installation, fixture.projectRoot, facts, 2)
	if err != nil || record == nil || record.Generation != 3 || record.UpOutcome != "failed" {
		t.Fatalf("a failed up over an advanced enrollment did not continue: %+v %v", record, err)
	}
	// An enrollment that did not advance is a refusal with up's own words.
	enrolledGeneration = 3
	if _, err := landedRearmAct(context.Background(), fixture.installation, fixture.projectRoot, facts, 3); err == nil || !strings.Contains(err.Error(), "cause=rearm-failed") || !strings.Contains(err.Error(), "no runtime ancestor") {
		t.Fatalf("an enrollment that did not advance was not refused with up's words: %v", err)
	}
	// A refused decision runs none of the three acts and takes no lock.
	rebuiltIn, upInstallation, locked = "", "", 0
	diverged := facts
	diverged.HeadIsAncestor = false
	if _, err := landedRearmAct(context.Background(), fixture.installation, fixture.projectRoot, diverged, 3); err == nil || !strings.Contains(err.Error(), "cause=engine-behind-tip") || rebuiltIn != "" || upInstallation != "" || locked != 0 {
		t.Fatalf("a refusal reached the acts: err=%v rebuild=%s up=%s locked=%d", err, rebuiltIn, upInstallation, locked)
	}
	landedRearmFastForward = func(context.Context, string, string) error { return errors.New("blocked") }
	if _, err := performLandedRearm(context.Background(), fixture.installation, fixture.projectRoot, facts, 3); err == nil || !strings.Contains(err.Error(), "cause=fast-forward-blocked") {
		t.Fatalf("fast-forward failure lost its cause: %v", err)
	}
	landedRearmFastForward = func(context.Context, string, string) error { return nil }
	landedRearmRebuild = func(context.Context, string) error { return errors.New("compiler failed") }
	if _, err := performLandedRearm(context.Background(), fixture.installation, fixture.projectRoot, facts, 3); err == nil || !strings.Contains(err.Error(), "cause=rebuild-failed") {
		t.Fatalf("rebuild failure lost its cause: %v", err)
	}
	landedRearmRebuild = previousRebuild
	landedRearmMutationLock = func(string) (func(), error) { return nil, errors.New("busy") }
	if _, err := landedRearmAct(context.Background(), fixture.installation, fixture.projectRoot, facts, 3); err == nil || !strings.Contains(err.Error(), "cause=mutation-lock") {
		t.Fatalf("mutation-lock failure lost its cause: %v", err)
	}
	// The real up seam runs the rebuilt binary's own up verb from the
	// installation with the checkout as --repo.
	if !strings.HasSuffix(runtimeUpCommandFor(fixture.installation, fixture.projectRoot), filepath.Join("bin", "metasystem")+" up --repo "+fixture.projectRoot) {
		t.Fatal("the up seam does not name the rebuilt binary's own up")
	}
}

func TestLandedRearmRefusesAFastForwardBlockedByDirtyLedgers(t *testing.T) {
	for _, test := range []struct {
		name, path              string
		staged, existingSidecar bool
	}{
		{name: "unstaged ledger", path: "metasystem/memory/receipts.log"},
		{name: "staged ledger", path: "metasystem/records/narrator-digest.log", staged: true},
		{name: "pre-existing save sidecar", path: "metasystem/memory/receipts.log", existingSidecar: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newLandedRearmFixture(t)
			landFixturePath(t, fixture, test.path, "landed append\n")
			local := filepath.Join(fixture.projectRoot, filepath.FromSlash(test.path))
			if err := os.WriteFile(local, []byte("local append\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if test.staged {
				landedGit(t, fixture.projectRoot, "add", test.path)
			}
			if test.existingSidecar {
				short := landedGit(t, fixture.projectRoot, "rev-parse", "--short", "HEAD")
				if err := os.WriteFile(local+".local."+short+".0", []byte("older save\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			head := landedGit(t, fixture.projectRoot, "rev-parse", "HEAD")
			facts, err := readLandedRearmFactsForTest(context.Background(), fixture.installation, fixture.projectRoot, "metasystem", head, func(string) bool { return false })
			if err != nil {
				t.Fatal(err)
			}
			fastForwards := 0
			previous := landedRearmFastForward
			landedRearmFastForward = func(context.Context, string, string) error { fastForwards++; return nil }
			t.Cleanup(func() { landedRearmFastForward = previous })
			_, err = landedRearmAct(context.Background(), fixture.installation, fixture.projectRoot, facts, 1)
			ledgerPath := strings.TrimPrefix(test.path, "metasystem/")
			if err == nil || fastForwards != 0 || !strings.Contains(err.Error(), "cause=fast-forward-blocked") ||
				!strings.Contains(err.Error(), "ledger-path='"+test.path+"'") || !strings.Contains(err.Error(), "cp -p -n --") ||
				!strings.Contains(err.Error(), "$(git rev-parse --short HEAD)") || !strings.Contains(err.Error(), "while test -e") ||
				!strings.Contains(err.Error(), "git restore --staged --worktree --source=HEAD --") ||
				!strings.Contains(err.Error(), "append only the missing saved lines") || strings.Contains(err.Error(), "reset --hard") ||
				strings.Contains(err.Error(), "git checkout --") || !strings.Contains(err.Error(), ledgerPath) {
				t.Fatalf("dirty ledger was not refused before the fast-forward with a preserving unique remedy: err=%v fast-forwards=%d", err, fastForwards)
			}
		})
	}
}

func TestLandedRearmRefusesAncestorPathCollisionsBeforeFastForward(t *testing.T) {
	for _, test := range []struct {
		name, incoming, local string
	}{
		{name: "dirty descendant of incoming file", incoming: "metasystem/collision", local: "metasystem/collision/child"},
		{name: "dirty file ancestor of incoming path", incoming: "metasystem/collision/child", local: "metasystem/collision"},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newLandedRearmFixture(t)
			landFixturePath(t, fixture, test.incoming, "landed\n")
			local := filepath.Join(fixture.projectRoot, filepath.FromSlash(test.local))
			if err := os.MkdirAll(filepath.Dir(local), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(local, []byte("local\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			head := landedGit(t, fixture.projectRoot, "rev-parse", "HEAD")
			facts, err := readLandedRearmFactsForTest(context.Background(), fixture.installation, fixture.projectRoot, "metasystem", head, func(string) bool { return false })
			if err != nil {
				t.Fatal(err)
			}
			fastForwards := 0
			previous := landedRearmFastForward
			landedRearmFastForward = func(context.Context, string, string) error { fastForwards++; return nil }
			t.Cleanup(func() { landedRearmFastForward = previous })
			_, err = landedRearmAct(context.Background(), fixture.installation, fixture.projectRoot, facts, 1)
			if err == nil || fastForwards != 0 || !strings.Contains(err.Error(), "cause=fast-forward-blocked") || !strings.Contains(err.Error(), test.local) {
				t.Fatalf("ancestor collision reached the fast-forward or lost its blocker: err=%v fast-forwards=%d facts=%+v", err, fastForwards, facts)
			}
		})
	}
}

type scheduledRearmTimer struct {
	due   time.Time
	ready chan time.Time
	fired bool
}

type scheduledRearmClock struct {
	mu     sync.Mutex
	now    time.Time
	timers []*scheduledRearmTimer
}

func newScheduledRearmClock() *scheduledRearmClock {
	return &scheduledRearmClock{now: time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)}
}

func (clock *scheduledRearmClock) Now() time.Time {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	return clock.now
}

func (clock *scheduledRearmClock) After(duration time.Duration) <-chan time.Time {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	timer := &scheduledRearmTimer{due: clock.now.Add(duration), ready: make(chan time.Time, 1)}
	clock.timers = append(clock.timers, timer)
	return timer.ready
}

func (clock *scheduledRearmClock) advance(duration time.Duration) {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	clock.now = clock.now.Add(duration)
	for _, timer := range clock.timers {
		if !timer.fired && !timer.due.After(clock.now) {
			timer.fired = true
			timer.ready <- clock.now
		}
	}
}

func TestLandedRearmJudgmentIsProgressBounded(t *testing.T) {
	fixture := newLandedRearmFixture(t)
	head := landedGit(t, fixture.projectRoot, "rev-parse", "HEAD")
	previousFetch := landedRearmFetch
	t.Cleanup(func() { landedRearmFetch = previousFetch })
	clock := newScheduledRearmClock()
	landedRearmFetch = func(ctx context.Context, injected steward.RearmClock, seconds int, _, _, _ string) error {
		return steward.RunRearmStep(ctx, injected, time.Duration(seconds)*time.Second, "fetch", func(_ context.Context, funcProgress func()) error {
			clock.advance(15 * time.Second)
			funcProgress()
			return nil
		})
	}
	facts, err := readLandedRearmFacts(context.Background(), clock, 20, fixture.installation, fixture.projectRoot, "metasystem", head, func(string) (bool, error) {
		err := steward.RunRearmStep(context.Background(), clock, 20*time.Second, "compare", func(_ context.Context, funcProgress func()) error {
			clock.advance(15 * time.Second)
			funcProgress()
			return nil
		})
		return err == nil, err
	})
	if err != nil || !facts.SourceOwnsTip {
		t.Fatalf("two progressing 15-second steps inherited a 20-second total: facts=%+v err=%v", facts, err)
	}
}

func TestLandedRearmRefusesAStalledTipCompareByName(t *testing.T) {
	fixture := newLandedRearmFixture(t)
	head := landedGit(t, fixture.projectRoot, "rev-parse", "HEAD")
	previousFetch, previousAncestry, previousDirty := landedRearmFetch, landedRearmAncestry, landedRearmDirty
	t.Cleanup(func() {
		landedRearmFetch, landedRearmAncestry, landedRearmDirty = previousFetch, previousAncestry, previousDirty
	})
	landedRearmFetch = func(context.Context, steward.RearmClock, int, string, string, string) error { return nil }
	ancestryCalls, dirtyCalls := 0, 0
	landedRearmAncestry = func(context.Context, steward.RearmClock, int, string, string, string) (bool, error) {
		ancestryCalls++
		return true, nil
	}
	landedRearmDirty = func(context.Context, steward.RearmClock, int, string, string) ([]string, error) {
		dirtyCalls++
		return nil, nil
	}
	clock := newScheduledRearmClock()
	_, err := readLandedRearmFacts(context.Background(), clock, 20, fixture.installation, fixture.projectRoot, "metasystem", head, func(string) (bool, error) {
		stall := steward.RunRearmStep(context.Background(), clock, 20*time.Second, "compare", func(ctx context.Context, _ func()) error {
			clock.advance(21 * time.Second)
			<-ctx.Done()
			return ctx.Err()
		})
		return false, stall
	})
	refusal := judgmentRefusal(err, engineCheckoutFacts(fixture.projectRoot), "compare the enrolled engine with the landed tip")
	if err == nil || !strings.Contains(refusal.Error(), "cause=judgment-stalled step=compare seconds=20") || ancestryCalls != 0 || dirtyCalls != 0 {
		t.Fatalf("stalled compare did not refuse before later probes: err=%v refusal=%v ancestry=%d dirty=%d", err, refusal, ancestryCalls, dirtyCalls)
	}

	_, err = readLandedRearmFacts(context.Background(), clock, 20, fixture.installation, fixture.projectRoot, "metasystem", head, func(string) (bool, error) {
		return false, errors.New("compare process failed")
	})
	refusal = judgmentRefusal(err, engineCheckoutFacts(fixture.projectRoot), "compare the enrolled engine with the landed tip")
	if err == nil || !strings.Contains(refusal.Error(), "cause=judgment-failed") || ancestryCalls != 0 || dirtyCalls != 0 {
		t.Fatalf("failed compare did not refuse before later probes: err=%v refusal=%v ancestry=%d dirty=%d", err, refusal, ancestryCalls, dirtyCalls)
	}
}
