package steward

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	processidentity "github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

func rearmGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeRearmFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func commitRearmTree(t *testing.T, root, value string) string {
	t.Helper()
	writeRearmFile(t, filepath.Join(root, "cmd", "surface.txt"), value+"\n")
	rearmGit(t, root, "add", ".")
	rearmGit(t, root, "commit", "-q", "-m", value)
	return rearmGit(t, root, "rev-parse", "HEAD")
}

func initRearmRepo(t *testing.T) string {
	t.Helper()
	root := canonicalPath(t.TempDir())
	rearmGit(t, root, "init", "-q", "-b", "trunk")
	rearmGit(t, root, "config", "user.name", "test")
	rearmGit(t, root, "config", "user.email", "test@example.invalid")
	rearmGit(t, root, "config", "metasystem.steward.notify-command", "true")
	writeRearmFile(t, filepath.Join(root, "go.mod"), "module fixture.invalid/rearm\n")
	return root
}

func buildFakeRunner(t *testing.T, output, stamp string) {
	t.Helper()
	cmd := exec.Command("go", "build", "-modcacherw", "-buildvcs=false",
		"-ldflags", "-X github.com/widoriezebos/agentic-tools/metasystem/internal/supervise.BuildStamp="+stamp,
		"-o", output, "./testdata/fakerunner")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build fake runner: %v\n%s", err, out)
	}
}

type rearmBed struct {
	root, engine, replacement string
	first, second             string
}

func newRearmBed(t *testing.T, startRunner bool) rearmBed {
	t.Helper()
	t.Setenv("METASYSTEM_SUPERVISION_REGISTRY_HOME", t.TempDir())
	root := initRearmRepo(t)
	first := commitRearmTree(t, root, "first")
	second := commitRearmTree(t, root, "second")
	ref := "refs/remotes/origin/trunk"
	rearmGit(t, root, "update-ref", ref, second)
	rearmGit(t, root, "config", "--local", landingRefConfigKey, ref)
	engine := filepath.Join(root, "metasystem")
	replacement := filepath.Join(root, "metasystem.next")
	buildFakeRunner(t, engine, first)
	buildFakeRunner(t, replacement, second)
	if startRunner {
		if message, err := Arm(root, engine); err != nil || !strings.Contains(message, "armed") {
			t.Fatalf("initial arm: %q %v", message, err)
		}
	} else {
		digest, err := installDigest(engine)
		if err != nil {
			t.Fatal(err)
		}
		if err := MintIdentity(RepoIdentityPath(root), InstallIdentity{
			RepoIdentity: root, Generation: 1, InstallPath: engine, InstallDigest: digest,
			MintedAt: time.Now().UTC().Format(time.RFC3339), MintedBy: "human-terminal",
			HumanWitnessedGeneration: 1, HumanWitnessedAt: time.Now().UTC().Format(time.RFC3339), EngineBuild: first,
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Rename(replacement, engine); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = Disarm(root) })
	return rearmBed{root: root, engine: engine, replacement: replacement, first: first, second: second}
}

func TestWitnessResolverWalksPastTheSixtyFourthCandidate(t *testing.T) {
	root := initRearmRepo(t)
	first := commitRearmTree(t, root, "candidate-00")
	policy, err := behaviorsurface.Load()
	if err != nil {
		t.Fatal(err)
	}
	digest, err := policy.Digest(root, behaviorsurface.Engine)
	if err != nil {
		t.Fatal(err)
	}
	for candidate := 1; candidate < 65; candidate++ {
		commitRearmTree(t, root, fmt.Sprintf("candidate-%02d", candidate))
	}
	ref := "refs/remotes/origin/trunk"
	rearmGit(t, root, "update-ref", ref, "HEAD")
	resolved, err := resolveWitnessStamp(root, root, ref, digest[:12])
	if err != nil || resolved != first {
		t.Fatalf("the sixty-fifth reachable candidate did not resolve: got=%s want=%s err=%v", resolved, first, err)
	}
}

func TestWitnessResolverMapsANestedInstallation(t *testing.T) {
	outer := canonicalPath(t.TempDir())
	rearmGit(t, outer, "init", "-q", "-b", "trunk")
	rearmGit(t, outer, "config", "user.name", "test")
	rearmGit(t, outer, "config", "user.email", "test@example.invalid")
	root := filepath.Join(outer, "metasystem")
	writeRearmFile(t, filepath.Join(root, "go.mod"), "module fixture.invalid/nested\n")
	writeRearmFile(t, filepath.Join(root, "cmd", "surface.txt"), "nested\n")
	rearmGit(t, outer, "add", ".")
	rearmGit(t, outer, "commit", "-q", "-m", "nested")
	commit := rearmGit(t, outer, "rev-parse", "HEAD")
	ref := "refs/remotes/origin/trunk"
	rearmGit(t, outer, "update-ref", ref, commit)
	policy, _ := behaviorsurface.Load()
	digest, err := policy.Digest(root, behaviorsurface.Engine)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := resolveWitnessStamp(outer, root, ref, digest[:12])
	if err != nil || resolved != commit {
		t.Fatalf("nested witness resolution: got=%s want=%s err=%v", resolved, commit, err)
	}
}

func TestWitnessResolverChoosesNewestMatchingCommit(t *testing.T) {
	root := initRearmRepo(t)
	commitRearmTree(t, root, "shared-engine")
	policy, err := behaviorsurface.Load()
	if err != nil {
		t.Fatal(err)
	}
	digest, err := policy.Digest(root, behaviorsurface.Engine)
	if err != nil {
		t.Fatal(err)
	}
	writeRearmFile(t, filepath.Join(root, "docs", "first.txt"), "outside engine\n")
	rearmGit(t, root, "add", ".")
	rearmGit(t, root, "commit", "-q", "-m", "newest matching tree")
	newest := rearmGit(t, root, "rev-parse", "HEAD")
	ref := "refs/remotes/origin/trunk"
	rearmGit(t, root, "update-ref", ref, newest)
	resolved, err := resolveWitnessStamp(root, root, ref, digest[:12])
	if err != nil || resolved != newest {
		t.Fatalf("newest matching commit was not selected: got=%s want=%s err=%v", resolved, newest, err)
	}
}

func TestWitnessResolverRefusesUnlandedTreeDigest(t *testing.T) {
	root := initRearmRepo(t)
	trunk := commitRearmTree(t, root, "trunk")
	rearmGit(t, root, "checkout", "-q", "-b", "side")
	commitRearmTree(t, root, "unlanded-side")
	policy, err := behaviorsurface.Load()
	if err != nil {
		t.Fatal(err)
	}
	digest, err := policy.Digest(root, behaviorsurface.Engine)
	if err != nil {
		t.Fatal(err)
	}
	rearmGit(t, root, "checkout", "-q", "trunk")
	ref := "refs/remotes/origin/trunk"
	rearmGit(t, root, "update-ref", ref, trunk)
	if resolved, err := resolveWitnessStamp(root, root, ref, digest[:12]); err == nil || resolved != "" {
		t.Fatalf("unlanded witness digest resolved to %q: %v", resolved, err)
	}
}

func TestWitnessResolverPersistsTreeDigestsAcrossResolutions(t *testing.T) {
	root := initRearmRepo(t)
	commit := commitRearmTree(t, root, "cached")
	policy, err := behaviorsurface.Load()
	if err != nil {
		t.Fatal(err)
	}
	digest, err := policy.Digest(root, behaviorsurface.Engine)
	if err != nil {
		t.Fatal(err)
	}
	ref := "refs/remotes/origin/trunk"
	rearmGit(t, root, "update-ref", ref, commit)
	cachePath := filepath.Join(root, "artifacts", "agents", "steward", witnessDigestCacheName)
	writeRearmFile(t, cachePath, "not json\n")
	original := witnessTreeDigester
	originalWriter := witnessDigestCacheWriter
	t.Cleanup(func() {
		witnessTreeDigester = original
		witnessDigestCacheWriter = originalWriter
	})
	digestCalls := 0
	cacheWrites := 0
	witnessTreeDigester = func(ctx context.Context, toplevel, tree string, loaded behaviorsurface.Policy) (string, error) {
		digestCalls++
		return original(ctx, toplevel, tree, loaded)
	}
	witnessDigestCacheWriter = func(path, anchor string, entries map[string]string) {
		cacheWrites++
		writeWitnessDigestCache(path, anchor, entries)
	}
	resolved, err := resolveWitnessStamp(root, root, ref, digest[:12])
	if err != nil || resolved != commit || digestCalls == 0 || cacheWrites != 1 {
		t.Fatalf("first resolution did not rebuild the unreadable cache once: got=%s digest-calls=%d cache-writes=%d err=%v", resolved, digestCalls, cacheWrites, err)
	}
	cacheBytes, err := os.ReadFile(cachePath)
	if err != nil || !json.Valid(cacheBytes) {
		t.Fatalf("persistent witness cache was not rewritten as JSON: %v %q", err, cacheBytes)
	}
	digestCalls = 0
	cacheWrites = 0
	resolved, err = resolveWitnessStamp(root, root, ref, digest[:12])
	if err != nil || resolved != commit || digestCalls != 0 || cacheWrites != 1 {
		t.Fatalf("second resolution repeated a digest or wrote more than once: got=%s digest-calls=%d cache-writes=%d err=%v", resolved, digestCalls, cacheWrites, err)
	}
}

func TestWitnessResolverWritesCacheOnceOnExpiry(t *testing.T) {
	root := initRearmRepo(t)
	commitRearmTree(t, root, "older")
	newest := commitRearmTree(t, root, "newest")
	ref := "refs/remotes/origin/trunk"
	rearmGit(t, root, "update-ref", ref, newest)
	rearmGit(t, root, "config", "--local", rearmResolveSecondsConfig, "1")
	originalDigester := witnessTreeDigester
	originalWriter := witnessDigestCacheWriter
	t.Cleanup(func() {
		witnessTreeDigester = originalDigester
		witnessDigestCacheWriter = originalWriter
	})
	digestCalls := 0
	cacheWrites := 0
	witnessTreeDigester = func(ctx context.Context, _, _ string, _ behaviorsurface.Policy) (string, error) {
		digestCalls++
		if digestCalls == 1 {
			return strings.Repeat("f", 64), nil
		}
		<-ctx.Done()
		return "", ctx.Err()
	}
	witnessDigestCacheWriter = func(path, anchor string, entries map[string]string) {
		cacheWrites++
		writeWitnessDigestCache(path, anchor, entries)
	}
	resolved, err := resolveWitnessStamp(root, root, ref, "000000000000")
	if err == nil || resolved != "" || !strings.Contains(err.Error(), "exceeded the configured 1-second bound") {
		t.Fatalf("resolver expiry was not loud: resolved=%q err=%v", resolved, err)
	}
	if digestCalls < 2 || cacheWrites != 1 {
		t.Fatalf("expiry used %d digest calls and %d cache writes, want at least two calls and exactly one write", digestCalls, cacheWrites)
	}
}

func TestLandingRefReadIgnoresGlobalConfigurationAndOrdinaryBranchMovement(t *testing.T) {
	root := initRearmRepo(t)
	first := commitRearmTree(t, root, "first")
	ref := "refs/remotes/origin/trunk"
	rearmGit(t, root, "update-ref", ref, first)
	global := filepath.Join(t.TempDir(), "gitconfig")
	writeRearmFile(t, global, "[metasystem \"steward\"]\n\tlanding-ref = "+ref+"\n")
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	if _, err := readOwnedLandingRef(root); err == nil {
		t.Fatal("a global landing-ref value satisfied the installation")
	}
	included := filepath.Join(t.TempDir(), "included-config")
	writeRearmFile(t, included, "[metasystem \"steward\"]\n\tlanding-ref = "+ref+"\n")
	rearmGit(t, root, "config", "--local", "include.path", included)
	if _, err := readOwnedLandingRef(root); err == nil {
		t.Fatal("an included landing-ref value satisfied the installation")
	}
	rearmGit(t, root, "config", "--local", "--unset-all", "include.path")
	localRef := "refs/heads/trunk"
	rearmGit(t, root, "config", "--local", landingRefConfigKey, localRef)
	if _, err := readOwnedLandingRef(root); err == nil || !strings.Contains(err.Error(), localRef) || !strings.Contains(err.Error(), "refs/remotes/<remote>/<branch>") {
		t.Fatalf("a local branch ref was not refused with the expected remote-tracking shape: %v", err)
	}
	rearmGit(t, root, "config", "--local", landingRefConfigKey, ref)
	commitRearmTree(t, root, "ordinary-branch-movement")
	if got := rearmGit(t, root, "rev-parse", ref); got != first {
		t.Fatalf("ordinary branch movement advanced the remote-tracking ref: got=%s want=%s", got, first)
	}
}

func TestIdentityPublicationKeepsAndClearsDurabilityDoubt(t *testing.T) {
	root := canonicalPath(t.TempDir())
	path := RepoIdentityPath(root)
	original := identityWriter
	t.Cleanup(func() { identityWriter = original })
	id := InstallIdentity{RepoIdentity: root, Generation: 1, InstallPath: "/bin/true", MintedAt: "2026-09-06T00:00:00Z"}
	identityWriter = func(target, text, anchor string) (bool, error) {
		durable, err := original(target, text, anchor)
		if target == path && err == nil {
			return false, nil
		}
		return durable, err
	}
	if err := MintIdentity(path, id); err == nil || !strings.Contains(err.Error(), "durability is pending") {
		t.Fatalf("post-rename doubt was reported as durable success: %v", err)
	}
	if got, err := VerifyIdentity(path, root); err != nil || got.Generation != 1 {
		t.Fatalf("the visible generation was lost: %+v %v", got, err)
	}
	if _, err := os.Stat(identityDurabilityPendingPath(path)); err != nil {
		t.Fatalf("durability marker is absent: %v", err)
	}
	identityWriter = original
	if err := MintIdentity(path, id); err != nil {
		t.Fatalf("re-publication did not confirm durability: %v", err)
	}
	if _, err := os.Stat(identityDurabilityPendingPath(path)); !os.IsNotExist(err) {
		t.Fatalf("durability marker survived a confirmed publication: %v", err)
	}
	identityWriter = func(target, text, anchor string) (bool, error) {
		durable, err := original(target, text, anchor)
		if target == identityDurabilityPendingPath(path) && err == nil {
			return false, nil
		}
		return durable, err
	}
	if durable, err := publishIdentity(path, id); err != nil || durable {
		t.Fatalf("doubted marker publication did not become a pending outcome: durable=%v err=%v", durable, err)
	}
	if got, err := VerifyIdentity(path, root); err != nil || got.Generation != 1 {
		t.Fatalf("marker durability doubt stopped identity publication: %+v %v", got, err)
	}
	if _, err := os.Stat(identityDurabilityPendingPath(path)); err != nil {
		t.Fatalf("doubted durability marker is absent: %v", err)
	}
	identityWriter = original
	if err := MintIdentity(path, id); err != nil {
		t.Fatalf("marker-doubt re-publication did not confirm durability: %v", err)
	}
	if err := os.WriteFile(identityDurabilityPendingPath(path), []byte("pending\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyIdentity(path, root); err == nil || !strings.Contains(err.Error(), "absent") {
		t.Fatalf("missing identity did not use the unenrolled path: %v", err)
	}
	if _, err := os.Stat(identityDurabilityPendingPath(path)); err != nil {
		t.Fatalf("a verifier removed a marker owned by the arm path: %v", err)
	}
	rearmGit(t, root, "init", "-q", "-b", "trunk")
	rearmGit(t, root, "config", "metasystem.steward.notify-command", "true")
	_, armErr := arm(root, "/bin/true", false, false,
		func(_ InstallIdentity, priorErr error, _ enrolledBytes) (mintPlan, error) {
			return mintPlan{}, priorErr
		})
	if armErr == nil || !strings.Contains(armErr.Error(), "identity absent") {
		t.Fatalf("orphan marker did not retain the existing no-identity outcome: %v", armErr)
	}
	if _, err := os.Stat(identityDurabilityPendingPath(path)); !os.IsNotExist(err) {
		t.Fatalf("locked arm path did not remove the orphan durability marker: %v", err)
	}
}

func TestRearmStageTransitionsAndDurability(t *testing.T) {
	t.Run("BeforeMint to Minted", func(t *testing.T) {
		bed := newRearmBed(t, false)
		outcome, err := ReArmRebuiltEngine(bed.root, bed.root, bed.engine)
		if err != nil || outcome.Stage != StageMinted || outcome.Status != "re-armed" || outcome.StoppedRunnerPid != 0 {
			t.Fatalf("direct mint transition: %+v %v", outcome, err)
		}
	})

	t.Run("BeforeMint to StopAttempted", func(t *testing.T) {
		bed := newRearmBed(t, true)
		original := runnerStopWriter
		runnerStopWriter = func(string, []byte, os.FileMode) error { return errors.New("injected marker write failure") }
		t.Cleanup(func() { runnerStopWriter = original })
		outcome, err := ReArmRebuiltEngine(bed.root, bed.root, bed.engine)
		if err == nil || outcome.Stage != StageStopAttempted || !strings.Contains(err.Error(), "write restart marker") {
			t.Fatalf("marker-write transition: %+v %v", outcome, err)
		}
		installed, _ := VerifyIdentity(RepoIdentityPath(bed.root), bed.root)
		if installed.Generation != 1 {
			t.Fatalf("stop-attempt failure minted generation %d", installed.Generation)
		}
	})

	t.Run("StopAttempted to Stopped then mint failure", func(t *testing.T) {
		bed := newRearmBed(t, true)
		original := identityWriter
		identityWriter = func(target, text, anchor string) (bool, error) {
			if target == RepoIdentityPath(bed.root) {
				return false, errors.New("injected identity publication failure")
			}
			return original(target, text, anchor)
		}
		t.Cleanup(func() { identityWriter = original })
		outcome, err := ReArmRebuiltEngine(bed.root, bed.root, bed.engine)
		if err == nil || outcome.Stage != StageStopped || !strings.Contains(err.Error(), "identity publication") {
			t.Fatalf("stopped mint failure: %+v %v", outcome, err)
		}
		installed, _ := VerifyIdentity(RepoIdentityPath(bed.root), bed.root)
		if installed.Generation != 1 {
			t.Fatalf("failed mint changed generation to %d", installed.Generation)
		}
	})

	t.Run("Stopped to Minted with durability pending", func(t *testing.T) {
		bed := newRearmBed(t, true)
		original := identityWriter
		identityWriter = func(target, text, anchor string) (bool, error) {
			durable, err := original(target, text, anchor)
			if target == RepoIdentityPath(bed.root) && err == nil {
				return false, nil
			}
			return durable, err
		}
		t.Cleanup(func() { identityWriter = original })
		outcome, err := ReArmRebuiltEngine(bed.root, bed.root, bed.engine)
		if err != nil || outcome.Stage != StageMinted || !outcome.DurabilityPending || outcome.Status != "re-armed" {
			t.Fatalf("doubted mint transition: %+v %v", outcome, err)
		}
		verdict := checkStewardRunner(bed.root, time.Now(), processidentity.KernelProber{})
		if !strings.Contains(verdict.Reason, "(durability pending)") {
			t.Fatalf("health hid identity durability doubt: %+v", verdict)
		}
	})
}

func TestConcurrentRearmsMintOneGeneration(t *testing.T) {
	bed := newRearmBed(t, true)
	entered := make(chan struct{})
	release := make(chan struct{})
	calls := 0
	afterArmDecision = func() {
		calls++
		if calls == 1 {
			close(entered)
			<-release
		}
	}
	t.Cleanup(func() { afterArmDecision = nil })
	type result struct {
		outcome ReArmOutcome
		err     error
	}
	results := make(chan result, 2)
	go func() {
		outcome, err := ReArmRebuiltEngine(bed.root, bed.root, bed.engine)
		results <- result{outcome, err}
	}()
	<-entered
	go func() {
		outcome, err := ReArmRebuiltEngine(bed.root, bed.root, bed.engine)
		results <- result{outcome, err}
	}()
	close(release)
	first, second := <-results, <-results
	if first.err != nil || second.err != nil {
		t.Fatalf("concurrent rearm: first=%+v second=%+v", first, second)
	}
	statuses := map[string]int{first.outcome.Status: 1}
	statuses[second.outcome.Status]++
	if statuses["re-armed"] != 1 || statuses["already-current"] != 1 {
		t.Fatalf("concurrent statuses: %+v %+v", first.outcome, second.outcome)
	}
	installed, _ := VerifyIdentity(RepoIdentityPath(bed.root), bed.root)
	if installed.Generation != 2 {
		t.Fatalf("concurrent re-arms minted generation %d", installed.Generation)
	}
}

func TestRearmAndTemporaryHumanArmPreserveTheHumanWord(t *testing.T) {
	t.Run("temporary human arm wins the first lock", func(t *testing.T) {
		bed := newRearmBed(t, true)
		entered := make(chan struct{})
		release := make(chan struct{})
		calls := 0
		beforeArmLock = func() {
			calls++
			if calls == 1 {
				close(entered)
				<-release
			}
		}
		t.Cleanup(func() { beforeArmLock = nil })
		type result struct {
			outcome ReArmOutcome
			err     error
		}
		resultChannel := make(chan result, 1)
		go func() {
			outcome, err := ReArmRebuiltEngine(bed.root, bed.root, bed.engine)
			resultChannel <- result{outcome: outcome, err: err}
		}()
		<-entered
		if _, err := ArmTemporary(bed.root, bed.engine, "approved-word", "2026-09-30"); err != nil {
			t.Fatal(err)
		}
		close(release)
		got := <-resultChannel
		if got.err != nil || got.outcome.Status != "already-current" {
			t.Fatalf("re-arm did not observe the completed human arm: %+v", got)
		}
		installed, err := VerifyIdentity(RepoIdentityPath(bed.root), bed.root)
		if err != nil || installed.MintedBy != "human-word" || installed.TemporaryHumanWord != "approved-word" || installed.ReviewBy != "2026-09-30" || installed.Enrollment != EnrollmentTemporaryWord {
			t.Fatalf("the human word was not preserved: %+v %v", installed, err)
		}
	})

	t.Run("temporary human arm follows the machine mint", func(t *testing.T) {
		bed := newRearmBed(t, true)
		machine, err := ReArmRebuiltEngine(bed.root, bed.root, bed.engine)
		if err != nil || machine.Status != "re-armed" {
			t.Fatalf("machine re-arm: %+v %v", machine, err)
		}
		if _, err := ArmTemporary(bed.root, bed.engine, "approved-word", "2026-09-30"); err != nil {
			t.Fatal(err)
		}
		installed, err := VerifyIdentity(RepoIdentityPath(bed.root), bed.root)
		if err != nil || installed.Generation != machine.Generation+1 || installed.MintedBy != "human-word" || installed.Enrollment != EnrollmentTemporaryWord ||
			installed.HumanWitnessedGeneration != installed.Generation || installed.TemporaryHumanWord != "approved-word" {
			t.Fatalf("the later human arm did not witness its own generation: %+v %v", installed, err)
		}
	})
}

func TestMachineRebuildCarriesFixtureEnrollmentForward(t *testing.T) {
	bed := newRearmBed(t, false)
	installed, err := VerifyIdentity(RepoIdentityPath(bed.root), bed.root)
	if err != nil {
		t.Fatal(err)
	}
	installed.Enrollment = EnrollmentFixture
	if err := MintIdentity(RepoIdentityPath(bed.root), installed); err != nil {
		t.Fatal(err)
	}
	outcome, err := ReArmRebuiltEngine(bed.root, bed.root, bed.engine)
	if err != nil || outcome.Status != "re-armed" {
		t.Fatalf("machine re-arm: %+v %v", outcome, err)
	}
	installed, err = VerifyIdentity(RepoIdentityPath(bed.root), bed.root)
	if err != nil || installed.Enrollment != EnrollmentFixture {
		t.Fatalf("machine rebuild changed fixture enrollment: %+v %v", installed, err)
	}
}

func TestHumanArmBesideALiveLegacyRunnerNamesRestart(t *testing.T) {
	bed := newRearmBed(t, false)
	digest, err := installDigest(bed.engine)
	if err != nil {
		t.Fatal(err)
	}
	legacy := InstallIdentity{
		RepoIdentity: bed.root, Generation: 1, InstallPath: bed.engine, InstallDigest: digest,
		MintedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if err := MintIdentity(RepoIdentityPath(bed.root), legacy); err != nil {
		t.Fatal(err)
	}
	pinned, err := OpenEnrolledBinary(bed.root)
	if err != nil {
		t.Fatal(err)
	}
	if err := pinned.PrepareForExecution(); err != nil {
		pinned.Close()
		t.Fatal(err)
	}
	if _, err := launchRunner(bed.root, pinned); err != nil {
		pinned.Close()
		t.Fatal(err)
	}
	pinned.Close()
	message, err := Arm(bed.root, bed.engine)
	if err != nil || !strings.Contains(message, "already armed") || !strings.Contains(message, "steward restart") || !strings.Contains(message, "LEGACY") {
		t.Fatalf("plain arm hid the clearing verb: %q %v", message, err)
	}
	installed, _ := VerifyIdentity(RepoIdentityPath(bed.root), bed.root)
	if installed.Generation != 1 {
		t.Fatalf("plain arm minted generation %d", installed.Generation)
	}
	if _, err := Restart(bed.root, bed.engine); err != nil {
		t.Fatal(err)
	}
	installed, err = VerifyIdentity(RepoIdentityPath(bed.root), bed.root)
	if err != nil || installed.Generation != 2 || installed.MintedBy != "human-terminal" || installed.HumanWitnessedGeneration != 2 || installed.Enrollment != EnrollmentHumanTerminal {
		t.Fatalf("restart did not witness the replacement generation: %+v %v", installed, err)
	}
}

func TestCommandTimeDriftSurvivesTheStewardLaunchChain(t *testing.T) {
	bed := newRearmBed(t, false)
	// Restore the first engine so the staged enrollment is current.
	buildFakeRunner(t, bed.engine, bed.first)
	pinned, err := OpenEnrolledBinary(bed.root)
	if err != nil {
		t.Fatal(err)
	}
	defer pinned.Close()
	if err := pinned.PrepareForExecution(); err != nil {
		t.Fatal(err)
	}
	replacement := pinned.execPath + ".changed"
	if err := os.WriteFile(replacement, []byte("changed\n"), 0o500); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, pinned.execPath); err != nil {
		t.Fatal(err)
	}
	_, err = repairPinnedRunner(bed.root, pinned, nil, time.Second)
	if !errors.Is(err, ErrEnrollmentDrift) {
		t.Fatalf("command-time drift lost its typed cause: %v", err)
	}
}

func TestSignalFailureNamesStopAttempted(t *testing.T) {
	bed := newRearmBed(t, true)
	original := runnerSignal
	runnerSignal = func(pid int, signal syscall.Signal) error {
		if signal == syscall.SIGTERM {
			return errors.New("injected termination failure")
		}
		return original(pid, signal)
	}
	t.Cleanup(func() { runnerSignal = original })
	outcome, err := ReArmRebuiltEngine(bed.root, bed.root, bed.engine)
	if err == nil || outcome.Stage != StageStopAttempted || !strings.Contains(err.Error(), "stop runner pid") {
		t.Fatalf("signal failure transition: %+v %v", outcome, err)
	}
}
