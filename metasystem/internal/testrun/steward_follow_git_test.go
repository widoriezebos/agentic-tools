package testrun

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginebuild"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

type deployedGitBed struct {
	root, writer, remote, old, commit, tip string
	clock                                  *scheduledRearmClock
}

func followGitOutput(ctx context.Context, root string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...)
	cmd.Env = append(gittree.ScrubbedEnviron(), "GIT_AUTHOR_DATE=2026-11-11T11:00:00Z", "GIT_COMMITTER_DATE=2026-11-11T11:00:00Z")
	return cmd.Output()
}

func followGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	out, err := followGitOutput(context.Background(), root, args...)
	if err != nil {
		t.Fatalf("git %v: %v output=%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

const (
	followRiskPath  = "records/counselor/accepted-risk-register.jsonl"
	followRiskFirst = "{\"id\":\"first\"}\n"
	followRiskBase  = followRiskFirst + "{\"id\":\"base\"}\n"
	followRiskLocal = "{\"id\":\"local-one\"}\n{\"id\":\"local-two\"}\n"
	followRiskMain  = "{\"id\":\"main\"}\n"
)

func newDeployedGitBed(t *testing.T, nested bool) *deployedGitBed {
	t.Helper()
	return newDeployedGitBedWithRisk(t, nested, followRiskBase)
}

func newDeployedGitBedWithRisk(t *testing.T, nested bool, deployedRisk string) *deployedGitBed {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	b := &deployedGitBed{root: filepath.Join(dir, "seat"), writer: filepath.Join(dir, "writer"), remote: filepath.Join(dir, "remote.git"), clock: newScheduledRearmClock()}
	followGit(t, dir, "init", "--bare", b.remote)
	followGit(t, dir, "init", "-b", "main", b.writer)
	followGit(t, b.writer, "config", "user.name", "Fixture")
	followGit(t, b.writer, "config", "user.email", "fixture@example.invalid")
	prefix := ""
	if nested {
		prefix = "metasystem"
	}
	writeTestingFixtureFile(t, filepath.Join(b.writer, prefix, "content.txt"), []byte("old\n"), 0o644)
	writeTestingFixtureFile(t, filepath.Join(b.writer, prefix, "local.txt"), []byte("local base\n"), 0o644)
	writeTestingFixtureFile(t, filepath.Join(b.writer, prefix, "src/product.go"), []byte("package product\n"), 0o644)
	writeTestingFixtureFile(t, filepath.Join(b.writer, prefix, followRiskPath), []byte(followRiskBase), 0o644)
	writeTestingFixtureFile(t, filepath.Join(b.writer, prefix, "records/counselor/carried-landings.jsonl"), []byte("base\n"), 0o644)
	for _, register := range landing.AppendOnlyRegisters() {
		writeTestingFixtureFile(t, filepath.Join(b.writer, prefix, register), []byte("base\n"), 0o644)
	}
	followGit(t, b.writer, "add", ".")
	followGit(t, b.writer, "commit", "-m", "old")
	b.old = followGit(t, b.writer, "rev-parse", "HEAD")
	followGit(t, b.writer, "remote", "add", "origin", b.remote)
	followGit(t, b.writer, "push", "origin", "HEAD:main")
	followGit(t, dir, "clone", "--no-local", "--branch", "main", b.remote, b.root)
	if nested {
		b.root = filepath.Join(b.root, "metasystem")
	}
	followGit(t, b.root, "config", "metasystem.steward.landing-ref", boundaryLandingRef)
	writeTestingFixtureFile(t, filepath.Join(b.writer, prefix, followRiskPath), []byte(deployedRisk), 0o644)
	writeTestingFixtureFile(t, filepath.Join(b.writer, prefix, "content.txt"), []byte("deployed\n"), 0o644)
	followGit(t, b.writer, "add", ".")
	followGit(t, b.writer, "commit", "-m", "deployed")
	b.commit = followGit(t, b.writer, "rev-parse", "HEAD")
	writeTestingFixtureFile(t, filepath.Join(b.writer, prefix, "content.txt"), []byte("later tip\n"), 0o644)
	followGit(t, b.writer, "add", ".")
	followGit(t, b.writer, "commit", "-m", "later")
	b.tip = followGit(t, b.writer, "rev-parse", "HEAD")
	followGit(t, b.writer, "push", "origin", "HEAD:main")
	return b
}

func (b *deployedGitBed) follow(t *testing.T) DeployedFollow {
	t.Helper()
	return DeployedFollowDependencies(context.Background(), b.root, b.clock.Now, func() (bool, error) { return true, nil }, func() (steward.DeployedEngine, error) {
		return steward.DeployedEngine{Commit: b.commit, Digest: "deployed"}, nil
	}, func(steward.DeployedEngine) error { return nil }, steward.RearmOperations{Clock: b.clock, Git: followGitOutput})
}

// Real object transfer and remote-ref updates are the adapter behavior under test;
// scripted Git answers cannot prove a fetch actually makes a commit resolvable.
func TestDeployedFollowLandedFetchGitAdapter(t *testing.T) {
	t.Parallel()
	for _, row := range []string{"missing-fetches", "missing-fetch-fails", "stale-ref-fetches", "stale-ref-fetch-fails", "fetched-ref-lacks-commit"} {
		t.Run(row, func(t *testing.T) {
			t.Parallel()
			b := newDeployedGitBed(t, false)
			if strings.HasPrefix(row, "stale-ref") || row == "fetched-ref-lacks-commit" {
				followGit(t, b.root, "fetch", "origin", "main")
				followGit(t, b.root, "update-ref", boundaryLandingRef, b.old)
			}
			if strings.HasSuffix(row, "fetch-fails") {
				followGit(t, b.root, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "unavailable.git"))
			}
			if row == "fetched-ref-lacks-commit" {
				followGit(t, b.writer, "push", "--force", "origin", b.old+":main")
			}
			f := b.follow(t)
			binary := filepath.Join(b.root, "bin", "metasystem")
			writeTestingFixtureFile(t, binary, []byte(enginebuild.StampRecord(b.old)), 0o755)
			oldBytes, err := os.ReadFile(binary)
			if err != nil {
				t.Fatal(err)
			}
			if err := steward.MintIdentity(steward.RepoIdentityPath(b.root), steward.InstallIdentity{RepoIdentity: b.root, InstallPath: binary, InstallDigest: fmt.Sprintf("sha256:%x", sha256.Sum256(oldBytes)), Generation: 1, MintedAt: b.clock.Now().Format(time.RFC3339), MintedBy: "human-terminal", EngineBuild: b.old}); err != nil {
				t.Fatal(err)
			}
			artifact := filepath.Join(t.TempDir(), "artifact")
			data := []byte(enginebuild.StampRecord(b.commit))
			writeTestingFixtureFile(t, artifact, data, 0o755)
			deployed := steward.DeployedEngine{Commit: b.commit, Artifact: artifact, Digest: fmt.Sprintf("sha256:%x", sha256.Sum256(data))}
			f.Deployed = func() (steward.DeployedEngine, error) { return deployed, nil }
			installs, starts, launches := 0, 0, 0
			f.Install = func(steward.DeployedEngine) error { installs++; return testexec.WriteFile(binary, data, 0o755) }
			// This row isolates landing ancestry from checkout movement; the public
			// re-arm must accept the deployed stamp while HEAD remains behind it.
			f.Forward = func(string) error { return nil }
			f.Start = func() error {
				starts++
				git := func(ctx context.Context, root string, args ...string) ([]byte, error) {
					if strings.Join(args, " ") == "config --get metasystem.steward.notify-command" {
						return []byte("fixture-notify"), nil
					}
					return followGitOutput(ctx, root, args...)
				}
				rearm := steward.ReArmDeployedEngine(func() (steward.DeployedEngine, bool, error) { return deployed, true, nil }, steward.RearmOperations{Clock: b.clock, Git: git, Prober: followTestProber{}, LaunchRunner: func(string, *steward.EnrolledBinary, string) (steward.RunnerRecord, error) {
					launches++
					return steward.RunnerRecord{Pid: 99}, nil
				}})
				out, err := rearm(b.root, b.root, binary)
				if err != nil {
					return err
				}
				enrolled, err := steward.VerifyIdentity(steward.RepoIdentityPath(b.root), b.root)
				if err != nil {
					return err
				}
				if out.Generation != 2 || enrolled.MintedBy != "deployed-engine" || enrolled.LandedCommit != b.commit {
					t.Fatalf("out=%+v enrolled=%+v", out, enrolled)
				}
				return nil
			}
			replaced, _, err := FollowDeployedEngineAtBoundary(b.root, f)
			want := row == "missing-fetches" || row == "stale-ref-fetches"
			if err != nil || replaced != want || installs != boolCount(want) || starts != boolCount(want) || launches != boolCount(want) {
				if !replaced {
					t.Log(followTestLog(t, b.root))
				}
				t.Fatalf("row=%s replaced=%v err=%v installs=%d starts=%d launches=%d", row, replaced, err, installs, starts, launches)
			}
			if !want {
				log := followTestLog(t, b.root)
				if strings.Count(log, "engine follow:") != 1 {
					t.Fatalf("log=%s", log)
				}
			}
		})
	}
}

func boolCount(b bool) int {
	if b {
		return 1
	}
	return 0
}

// Git's merge and append-only register preservation are the adapter claims:
// the deployed commit must win over a later remote tip without losing records.
func TestDeployedFollowForwardGitAdapter(t *testing.T) {
	t.Parallel()
	t.Run("accepted risk", testDeployedFollowRisk)
	for _, row := range []string{"behind", "past", "registers-only", "tracked-change", "diverged", "nested-registers"} {
		t.Run(row, func(t *testing.T) {
			t.Parallel()
			b := newDeployedGitBed(t, row == "nested-registers")
			f := b.follow(t)
			if err := f.Landed(b.commit); err != nil {
				t.Fatal(err)
			}
			if row == "past" {
				followGit(t, b.root, "merge", "--ff-only", b.tip)
			}
			if row == "diverged" {
				followGit(t, b.root, "config", "user.name", "Fixture")
				followGit(t, b.root, "config", "user.email", "fixture@example.invalid")
				writeTestingFixtureFile(t, filepath.Join(b.root, "diverged.txt"), []byte("seat change\n"), 0o644)
				followGit(t, b.root, "add", "diverged.txt")
				followGit(t, b.root, "commit", "-m", "diverged")
			}
			if row == "tracked-change" {
				writeTestingFixtureFile(t, filepath.Join(b.root, "local.txt"), []byte("dirty\n"), 0o644)
			}
			registers := row == "behind" || row == "registers-only" || row == "nested-registers"
			if registers {
				for _, register := range landing.AppendOnlyRegisters() {
					writeTestingFixtureFile(t, filepath.Join(b.root, register), []byte("base\nseat row\n"), 0o644)
				}
			}
			before := followGit(t, b.root, "rev-parse", "HEAD")
			installs, starts := 0, 0
			f.EnrolledDigest = func() (string, error) { return "old", nil }
			f.Install = func(steward.DeployedEngine) error { installs++; return nil }
			f.Start = func() error { starts++; return nil }
			replaced, _, err := FollowDeployedEngineAtBoundary(b.root, f)
			blocked := row == "tracked-change" || row == "diverged"
			after := followGit(t, b.root, "rev-parse", "HEAD")
			wantHead := b.commit
			if row == "past" || blocked {
				wantHead = before
			}
			if err != nil || replaced == blocked || after != wantHead || installs != boolCount(!blocked) || starts != boolCount(!blocked) {
				t.Fatalf("replaced=%v err=%v before=%s after=%s want=%s installs=%d starts=%d", replaced, err, before, after, wantHead, installs, starts)
			}
			if blocked {
				log := followTestLog(t, b.root)
				reason := "HEAD has commits the deployed commit lacks"
				if row == "tracked-change" {
					reason = "the checkout has tracked changes"
				}
				if !strings.Contains(log, reason) {
					t.Fatalf("log=%s", log)
				}
			}
			if registers {
				for _, register := range landing.AppendOnlyRegisters() {
					data, err := os.ReadFile(filepath.Join(b.root, register))
					if err != nil || string(data) != "base\nseat row\n" {
						t.Fatalf("register %s=%s err=%v", register, data, err)
					}
				}
			}
		})
	}
}

func testDeployedFollowRisk(t *testing.T) {
	const dirtyRisk = "the checkout has tracked changes:  M metasystem/" + followRiskPath
	const recovery = "metasystem/fast-forward-recovery.txt"
	const receipts = "memory/receipts.log"
	const otherRegister = "records/counselor/carried-landings.jsonl"
	local := followRiskBase + followRiskLocal
	for _, tc := range []struct {
		name, deployed, working, extraPath, refusal, wantSuffix string
		alreadyAt, leftover, staged, mode                       bool
	}{
		{name: "clean deployment rewrote register", deployed: followRiskMain},
		{name: "two local lines", working: local, wantSuffix: followRiskLocal},
		{name: "main and staged and unstaged local lines", deployed: followRiskBase + followRiskMain, working: local, staged: true, wantSuffix: followRiskLocal},
		{name: "arrived row", deployed: followRiskBase + followRiskMain, working: followRiskBase + followRiskMain},
		{name: "conflicting row", deployed: followRiskBase + "{\"id\":\"local-two\",\"title\":\"changed\"}\n", working: local, staged: true, refusal: `accepted risk "local-two" differs between the checkout and the deployment`},
		{name: "arrived among new rows", deployed: followRiskBase + followRiskMain, working: followRiskBase + followRiskLocal + followRiskMain + "{\"id\":\"last\"}\n", staged: true, wantSuffix: followRiskLocal + "{\"id\":\"last\"}\n"},
		{name: "deployment rewrote locally dirty register", deployed: followRiskMain, working: local, refusal: dirtyRisk},
		{name: "inserted before base", working: followRiskFirst + followRiskLocal + strings.TrimPrefix(followRiskBase, followRiskFirst), refusal: dirtyRisk},
		{name: "changed base", working: followRiskMain, refusal: dirtyRisk},
		{name: "removed base", working: followRiskFirst, refusal: dirtyRisk},
		{name: "partial line", working: followRiskBase + "unfinished", refusal: dirtyRisk},
		{name: "mode with append", working: local, mode: true, refusal: dirtyRisk},
		{name: "dirty code", working: local, extraPath: "src/product.go", refusal: "the checkout has tracked changes:  M metasystem/src/product.go"},
		{name: "two registers", working: local, extraPath: receipts, wantSuffix: followRiskLocal},
		{name: "other counselor register", working: local, extraPath: otherRegister, refusal: "the checkout has tracked changes:  M metasystem/" + otherRegister},
		{name: "recovery before movement", leftover: true, refusal: "fast-forward recovery file already exists at "},
		{name: "recovery after movement", alreadyAt: true, leftover: true, refusal: "fast-forward recovery file already exists at "},
		{name: "rewrite after movement", alreadyAt: true, working: followRiskMain, refusal: dirtyRisk},
		{name: "append after movement", alreadyAt: true, working: local, wantSuffix: followRiskLocal},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			deployed := tc.deployed
			if deployed == "" {
				deployed = followRiskBase
			}
			b := newDeployedGitBedWithRisk(t, true, deployed)
			f := b.follow(t)
			if err := f.Landed(b.commit); err != nil {
				t.Fatal(err)
			}
			if tc.alreadyAt {
				followGit(t, b.root, "merge", "--ff-only", b.commit)
			}
			path := filepath.Join(b.root, followRiskPath)
			if tc.staged {
				writeTestingFixtureFile(t, path, []byte(followRiskBase+strings.SplitAfter(followRiskLocal, "\n")[0]), 0644)
				followGit(t, b.root, "add", "--", followRiskPath)
			}
			if tc.working != "" {
				writeTestingFixtureFile(t, path, []byte(tc.working), 0644)
			}
			if tc.mode {
				if err := os.Chmod(path, 0755); err != nil {
					t.Fatal(err)
				}
			}
			if tc.extraPath != "" {
				writeTestingFixtureFile(t, filepath.Join(b.root, tc.extraPath), []byte("base\nextra\n"), 0644)
			}
			sentinel := followGit(t, b.root, "rev-parse", "--path-format=absolute", "--git-path", recovery)
			if tc.leftover {
				writeTestingFixtureFile(t, sentinel, []byte("reconcile this suffix\n"), 0644)
			}
			before := followGit(t, b.root, "rev-parse", "HEAD")
			f.EnrolledDigest = func() (string, error) { return "old", nil }
			installs, starts := 0, 0
			f.Install = func(steward.DeployedEngine) error { installs++; return nil }
			f.Start = func() error { starts++; return nil }
			followed, refused, err := FollowDeployedEngineAtBoundary(b.root, f)
			wantFollow := tc.refusal == ""
			if err != nil || followed != wantFollow || installs != boolCount(wantFollow) || starts != boolCount(wantFollow) {
				t.Fatalf("followed=%v refusal=%+v err=%v installs=%d starts=%d", followed, refused, err, installs, starts)
			}
			wantHead, wantBytes := b.commit, deployed
			if !wantFollow {
				reason := tc.refusal
				if tc.leftover {
					reason += sentinel
				}
				if refused == nil || refused.Reason != reason {
					t.Fatalf("refusal=%+v want %q", refused, reason)
				}
				wantHead, wantBytes = before, followRiskBase
				if tc.working != "" {
					wantBytes = tc.working
				}
			} else {
				wantBytes += tc.wantSuffix
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != wantBytes || followGit(t, b.root, "rev-parse", "HEAD") != wantHead {
				t.Fatalf("risk=%q want %q, head wanted %s, err=%v", got, wantBytes, wantHead, err)
			}
			if !wantFollow && tc.staged && followGit(t, b.root, "show", ":metasystem/"+followRiskPath) != strings.TrimSpace(followRiskBase+strings.SplitAfter(followRiskLocal, "\n")[0]) {
				t.Fatal("refusal changed the staged risk rows")
			}
			if wantFollow && !tc.alreadyAt && followGit(t, b.root, "diff", "--cached", "--name-only") != "" {
				t.Fatal("follow staged local rows")
			}
			if tc.extraPath != "" {
				got, err := os.ReadFile(filepath.Join(b.root, tc.extraPath))
				if err != nil || string(got) != "base\nextra\n" {
					t.Fatalf("second file=%q err=%v", got, err)
				}
			}
			data, err := os.ReadFile(sentinel)
			if tc.leftover && (err != nil || string(data) != "reconcile this suffix\n") || !tc.leftover && !os.IsNotExist(err) {
				t.Fatalf("recovery=%q err=%v", data, err)
			}
		})
	}
}
