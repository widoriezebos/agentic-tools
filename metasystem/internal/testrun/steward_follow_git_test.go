package testrun

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
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

func newDeployedGitBed(t *testing.T, nested bool) *deployedGitBed {
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
			f.EnrolledDigest = func() (string, error) { return "old", nil }
			installs, starts := 0, 0
			f.Install = func(steward.DeployedEngine) error { installs++; return nil }
			// Landing ancestry is independent of checkout movement.
			f.Forward = func(string) error { return nil }
			f.Start = func() error { starts++; return nil }
			replaced, err := FollowDeployedEngineAtBoundary(b.root, f)
			want := row == "missing-fetches" || row == "stale-ref-fetches"
			if err != nil || replaced != want || installs != boolCount(want) || starts != boolCount(want) {
				if !replaced {
					t.Log(followTestLog(t, b.root))
				}
				t.Fatalf("row=%s replaced=%v err=%v installs=%d starts=%d", row, replaced, err, installs, starts)
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
			replaced, err := FollowDeployedEngineAtBoundary(b.root, f)
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
