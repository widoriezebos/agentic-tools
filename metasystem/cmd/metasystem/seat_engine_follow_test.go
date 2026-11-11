package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/deploy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginebuild"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot/stateroottest"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// A child owns PATH and the registry home; behavior tests have no real Git fallback.
func centralFollowChild(t *testing.T, realGit bool) bool {
	t.Helper()
	if os.Getenv("CENTRAL_FOLLOW_CHILD") == "1" {
		return true
	}
	command := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^"+t.Name()+"$", "-test.count=1")
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if name != "METASYSTEM_SUPERVISION_REGISTRY_HOME" && name != "METASYSTEM_TESTENV_REGISTRY_NONCE" && name != "PATH" {
			command.Env = append(command.Env, entry)
		}
	}
	path := os.Getenv("PATH")
	if !realGit {
		tools := t.TempDir()
		script := `#!/bin/sh
if [ "$1" = -C ]; then root=$2; shift 2; fi
case "$*" in
 'remote get-url origin') echo 'https://example.invalid/central.git' ;;
 'rev-parse --show-toplevel') echo "$root" ;;
 *) exit 1 ;;
esac
`
		if err := testexec.WriteFile(filepath.Join(tools, "git"), []byte(script), 0700); err != nil {
			t.Fatal(err)
		}
		path = tools + string(os.PathListSeparator) + path
	}
	command.Env = append(command.Env, "CENTRAL_FOLLOW_CHILD=1", "PATH="+path)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("isolated follow: %v\n%s", err, output)
	}
	return false
}

type centralFollowClock struct{ *workClock }

func (centralFollowClock) After(time.Duration) <-chan time.Time { return make(chan time.Time) }

func centralFollowProbe(now time.Time) fixtureLifetimeProber {
	return fixtureLifetimeProber(func(pid int64) (identity.Exact, identity.Liveness, error) {
		if pid == int64(os.Getpid()) || pid == 41 {
			return identity.Exact{Pid: pid, StartedAt: now}, identity.Alive, nil
		}
		return identity.Exact{}, identity.Dead, nil
	})
}

type centralFollowBed struct {
	root, commit, binary, artifact, digest string
	rebuilds                               int
	data                                   []byte
	now                                    time.Time
}

func newCentralFollowBed(t *testing.T) *centralFollowBed {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	b := &centralFollowBed{root: root, commit: strings.Repeat("b", 40), now: time.Date(2026, 11, 11, 11, 0, 0, 0, time.UTC)}
	b.binary, b.artifact = filepath.Join(root, "bin", "metasystem"), filepath.Join(t.TempDir(), "artifact")
	if err := os.MkdirAll(filepath.Dir(b.binary), 0755); err != nil {
		t.Fatal(err)
	}
	injectedRunnerFile(t, filepath.Join(root, "metasystem.conf"), []byte("metasystem.engine-delivery=central\n"))
	injectedRunnerFile(t, filepath.Join(root, "deploy.json"), []byte(`{"schema":1,"adapter":{"argv":["fixture"]}}`))
	b.data = []byte(enginebuild.StampRecord(b.commit))
	b.digest = fmt.Sprintf("sha256:%x", sha256.Sum256(b.data))
	if err := testexec.WriteFile(b.artifact, b.data, 0755); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(b.binary, []byte(enginebuild.StampRecord(strings.Repeat("a", 40))), 0755); err != nil {
		t.Fatal(err)
	}
	b.centralEnroll(t)
	return b
}
func (b *centralFollowBed) centralEnroll(t *testing.T) {
	t.Helper()
	data, err := os.ReadFile(b.binary)
	if err != nil {
		t.Fatal(err)
	}
	if err := steward.MintIdentity(steward.RepoIdentityPath(b.root), steward.InstallIdentity{RepoIdentity: b.root, InstallPath: b.binary, InstallDigest: fmt.Sprintf("sha256:%x", sha256.Sum256(data)), Generation: 1, MintedAt: b.now.Format(time.RFC3339)}); err != nil {
		t.Fatal(err)
	}
}
func (b *centralFollowBed) centralDeployLine(t *testing.T, digest, commit string) {
	t.Helper()
	project, err := resolveDeployProject(deployOwners{origin: deploy.OriginURL, home: board.Home}, b.root, b.root)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(deploy.Line{Kind: deploy.KindDeploy, Outcome: deploy.OutcomeActive, Commit: commit, Artifact: b.artifact, Digest: digest, StartedAt: b.now, EndedAt: b.now})
	if err != nil {
		t.Fatal(err)
	}
	injectedRunnerFile(t, filepath.Join(project.dir, "deploys.jsonl"), append(data, '\n'))
}
func (b *centralFollowBed) centralGit(t *testing.T) func(context.Context, string, ...string) ([]byte, error) {
	t.Helper()
	return func(_ context.Context, root string, args ...string) ([]byte, error) {
		if root != b.root {
			t.Errorf("Git root=%s", root)
		}
		switch strings.Join(args, " ") {
		case "config --get metasystem.steward.rearm-resolve-seconds":
			return []byte("20"), nil
		case "config --local --no-includes --get metasystem.steward.landing-ref":
			return []byte("refs/remotes/origin/main"), nil
		case "fetch --progress origin main", "merge-base --is-ancestor " + b.commit + " refs/remotes/origin/main", "merge-base --is-ancestor " + b.commit + " HEAD", "rev-parse --show-prefix", "status --porcelain=v1 -z --untracked-files=no":
			return nil, nil
		case "rev-parse --verify " + b.commit + "^{commit}", "rev-parse --verify --quiet refs/remotes/origin/main^{commit}":
			return []byte(b.commit), nil
		default:
			t.Errorf("unexpected follow Git: %q", args)
			return nil, fmt.Errorf("unexpected Git")
		}
	}
}
func (b *centralFollowBed) centralRun(t *testing.T, passes int, start func() error, inspect func(*steward.TickConfig), operations ...steward.RearmOperations) {
	t.Helper()
	deadline := b.now.Add(time.Duration(passes) * time.Second)
	clock := &steward.HandoffClock{Now: func() time.Time { return b.now }, Sleep: func(d time.Duration) {
		b.now = b.now.Add(d)
		if !b.now.Before(deadline) {
			injectedRunnerFile(t, filepath.Join(b.root, "artifacts", "agents", "steward", "stop"), nil)
		}
	}}
	prober := centralFollowProbe(b.now)
	ops := steward.RearmOperations{Clock: centralFollowClock{&workClock{now: b.now}}, Git: b.centralGit(t), Prober: prober}
	if len(operations) > 0 {
		ops = operations[0]
	}
	var problem bytes.Buffer
	code := runStewardRunWithDependencies([]string{"--repo", b.root}, io.Discard, &problem,
		func(root string, _ steward.WorkerCensus, _ func() error, interval time.Duration, cfg steward.TickConfig) error {
			if inspect != nil {
				inspect(&cfg)
			}
			// Preserve the command's follow and rebuild closures while other duties stay idle.
			cfg = steward.TickConfig{Now: b.now, RunnerClock: clock, Prober: prober, FollowDeployedEngine: cfg.FollowDeployedEngine, RearmAtBoundary: cfg.RearmAtBoundary}
			return steward.RunLoop(root, fleetHandoffCensus{}, nil, interval, cfg)
		}, nil, clock, func(string) int { return 1 }, nil, stewardRunDependencies{Prober: prober, FollowOperations: ops, FollowStart: start, RearmBoundary: func(string, func() (bool, error)) (bool, error) { b.rebuilds++; return false, nil }})
	if code != 0 {
		t.Fatalf("runner exit=%d: %s", code, &problem)
	}
}

func TestCentralFollowRunnerInstallsOnce(t *testing.T) {
	t.Parallel()
	if !centralFollowChild(t, false) {
		return
	}
	b := newCentralFollowBed(t)
	b.centralDeployLine(t, strings.TrimPrefix(b.digest, "sha256:"), b.commit)
	starts, rebuilds := 0, 0
	b.centralRun(t, 1, func() error { starts++; return nil }, func(cfg *steward.TickConfig) {
		if cfg.FollowDeployedEngine == nil {
			t.Fatal("central follow missing")
		}
		replaced, err := cfg.RearmAtBoundary()
		if replaced || err != nil {
			t.Fatalf("central rebuild=%v %v", replaced, err)
		}
		rebuild := cfg.RearmAtBoundary
		cfg.RearmAtBoundary = func() (bool, error) { rebuilds++; return rebuild() }
	})
	data, err := os.ReadFile(b.binary)
	staged, _ := filepath.Glob(filepath.Join(b.root, "bin", ".metasystem.follow-*"))
	if len(staged) != 0 {
		t.Fatalf("staging left after success: %v", staged)
	}
	if err != nil || !bytes.Equal(data, b.data) || starts != 1 || rebuilds != 0 || b.rebuilds != 0 {
		t.Fatalf("bin=%q starts=%d rebuilds=%d err=%v", data, starts, rebuilds, err)
	}
	b.centralEnroll(t)
	b.centralRun(t, 1, func() error { starts++; return nil }, nil)
	if starts != 1 {
		t.Fatalf("prefix-free digest followed twice: %d", starts)
	}
}

func TestCentralFollowRejectsArtifactMismatch(t *testing.T) {
	t.Parallel()
	if !centralFollowChild(t, false) {
		return
	}
	for _, row := range []string{"stamp", "digest", "copied-digest"} {
		t.Run(row, func(t *testing.T) {
			b := newCentralFollowBed(t)
			digest, commit := b.digest, b.commit
			if row == "stamp" {
				commit = strings.Repeat("c", 40)
			}
			if row == "digest" {
				digest = "sha256:" + strings.Repeat("0", 64)
			}
			b.centralDeployLine(t, digest, commit)
			before, _ := os.ReadFile(b.binary)
			starts := 0
			if row == "copied-digest" {
				engine, err := activeDeployedEngine(b.root, b.root)
				if err != nil {
					t.Fatal(err)
				}
				if err := testexec.WriteFile(b.artifact, []byte("changed after validation"), 0755); err != nil {
					t.Fatal(err)
				}
				if err := installDeployedEngine(b.root, engine); err == nil {
					t.Fatal("copied digest mismatch accepted")
				}
			} else {
				b.centralRun(t, 10, func() error { starts++; return nil }, nil)
				log, err := os.ReadFile(filepath.Join(b.root, "artifacts", "agents", "steward", "runner.log"))
				if err != nil || strings.Count(string(log), "engine follow:") != 1 || !strings.Contains(string(log), "to follow by hand:") {
					t.Fatalf("log=%s err=%v", log, err)
				}
				paths, err := filepath.Glob(filepath.Join(b.root, "artifacts", "agents", "channel", "questions", "*.json"))
				if err != nil || len(paths) != 1 {
					t.Fatalf("follow questions=%v err=%v", paths, err)
				}
				var q struct {
					Wants     string
					RetiredAt time.Time
					Decision  *struct{ Kind, State string }
				}
				data, err := os.ReadFile(paths[0])
				if err != nil || json.Unmarshal(data, &q) != nil || q.Decision == nil || q.Decision.Kind != "engine-follow" || q.Decision.State != commit || !q.RetiredAt.IsZero() || !strings.HasPrefix(q.Wants, "cd "+b.root+" && cp -p "+b.artifact+" bin/.metasystem.hand") {
					t.Fatalf("follow question=%s err=%v", data, err)
				}
			}
			after, _ := os.ReadFile(b.binary)
			staged, _ := filepath.Glob(filepath.Join(b.root, "bin", ".metasystem.follow-*"))
			if !bytes.Equal(before, after) || starts != 0 || len(staged) != 0 {
				t.Fatalf("bin changed=%v starts=%d staged=%v", !bytes.Equal(before, after), starts, staged)
			}
		})
	}
}

func TestCentralFollowRenameFailureKeepsBin(t *testing.T) {
	t.Parallel()
	b := newCentralFollowBed(t)
	before, _ := os.ReadFile(b.binary)
	renames := 0
	err := installDeployedEngineWithRename(b.root, steward.DeployedEngine{Artifact: b.artifact, Digest: b.digest}, func(from, to string) error {
		renames++
		if to != b.binary || filepath.Dir(from) != filepath.Dir(b.binary) {
			t.Fatalf("rename %s -> %s", from, to)
		}
		return fmt.Errorf("rename refused")
	})
	after, readErr := os.ReadFile(b.binary)
	staged, _ := filepath.Glob(filepath.Join(b.root, "bin", ".metasystem.follow-*"))
	if err == nil || readErr != nil || !bytes.Equal(before, after) || len(staged) != 0 || renames != 1 {
		t.Fatalf("rename err=%v bin changed=%v read=%v staged=%v renames=%d", err, !bytes.Equal(before, after), readErr, staged, renames)
	}
}

func TestSourceFollowWiringAndRefreshOverride(t *testing.T) {
	t.Parallel()
	if !centralFollowChild(t, false) {
		return
	}
	for _, mode := range []string{"absent", "source", "central-no-contract", "central-refresh"} {
		t.Run(mode, func(t *testing.T) {
			b := newCentralFollowBed(t)
			switch mode {
			case "absent":
				injectedRunnerFile(t, filepath.Join(b.root, "metasystem.conf"), nil)
			case "source":
				injectedRunnerFile(t, filepath.Join(b.root, "metasystem.conf"), []byte("metasystem.engine-delivery=source\n"))
			case "central-no-contract":
				if err := os.Remove(filepath.Join(b.root, "deploy.json")); err != nil {
					t.Fatal(err)
				}
			}
			refreshes := 0
			var refresh func() (bool, error)
			if mode == "central-refresh" {
				refresh = func() (bool, error) { refreshes++; return true, nil }
			}
			code := runStewardRunWithDependencies([]string{"--repo", b.root}, io.Discard, io.Discard, func(_ string, _ steward.WorkerCensus, _ func() error, _ time.Duration, cfg steward.TickConfig) error {
				if cfg.FollowDeployedEngine != nil || cfg.RearmAtBoundary == nil {
					t.Fatalf("%s: follow=%v rearm=%v", mode, cfg.FollowDeployedEngine != nil, cfg.RearmAtBoundary != nil)
				}
				replaced, err := cfg.RearmAtBoundary()
				if err != nil || replaced != (mode == "central-refresh") {
					t.Fatalf("source/override=%v %v", replaced, err)
				}
				return nil
			}, nil, nil, func(string) int { return 1 }, refresh)
			if code != 0 || (mode == "central-refresh" && refreshes != 1) {
				t.Fatalf("exit=%d refreshes=%d", code, refreshes)
			}
		})
	}
}

func TestCentralHookSkipsEngineRebuild(t *testing.T) {
	t.Parallel()
	b := newCentralFollowBed(t)
	rebuilds := 0
	owner := hookOwners{engineBuild: func() *exec.Cmd { rebuilds++; t.Fatal("central hook rebuilt"); return nil }}
	behind, err := owner.EngineBehind(stateroottest.Installation(t, b.root), b.root)
	if behind {
		err = owner.StartEngineRebuild(stateroottest.Installation(t, b.root))
	}
	if err != nil || behind || rebuilds != 0 {
		t.Fatalf("behind=%v rebuilds=%d err=%v", behind, rebuilds, err)
	}
}

// Real Git history is the admission input: the old stamp's range contains an
// engine change, while the deployed stamp's range is empty after the fetch.
func TestCentralFollowClearsStaleBuildGitAdapter(t *testing.T) {
	t.Parallel()
	if !centralFollowChild(t, true) {
		return
	}
	b := newCentralFollowBed(t)
	git := func(ctx context.Context, root string, args ...string) ([]byte, error) {
		command := exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...)
		command.Env = append(gittree.ScrubbedEnviron(), "GIT_AUTHOR_DATE=2026-11-11T11:00:00Z", "GIT_COMMITTER_DATE=2026-11-11T11:00:00Z")
		return command.Output()
	}
	runGit := func(root string, args ...string) string {
		t.Helper()
		data, err := git(t.Context(), root, args...)
		if err != nil {
			t.Fatalf("git %v: %s %v", args, data, err)
		}
		return strings.TrimSpace(string(data))
	}
	runGit(b.root, "init", "-b", "main")
	runGit(b.root, "config", "user.name", "Fixture")
	runGit(b.root, "config", "user.email", "fixture@example.invalid")
	injectedRunnerFile(t, filepath.Join(b.root, ".gitignore"), []byte("bin/\nartifacts/\nplans/\n"))
	injectedRunnerFile(t, filepath.Join(b.root, "cmd", "metasystem", "main.go"), []byte("package main\n"))
	injectedRunnerFile(t, filepath.Join(b.root, "internal", "changed.go"), []byte("old\n"))
	runGit(b.root, "add", ".")
	runGit(b.root, "commit", "-m", "old engine")
	old := runGit(b.root, "rev-parse", "HEAD")
	if err := testexec.WriteFile(b.binary, []byte(enginebuild.StampRecord(old)), 0755); err != nil {
		t.Fatal(err)
	}
	b.centralEnroll(t)
	remote := filepath.Join(t.TempDir(), "remote.git")
	runGit(filepath.Dir(remote), "init", "--bare", remote)
	runGit(b.root, "remote", "add", "origin", remote)
	injectedRunnerFile(t, filepath.Join(b.root, "internal", "changed.go"), []byte("landed\n"))
	runGit(b.root, "add", ".")
	runGit(b.root, "commit", "-m", "landed engine")
	b.commit = runGit(b.root, "rev-parse", "HEAD")
	runGit(b.root, "push", "origin", "HEAD:main")
	runGit(b.root, "fetch", "origin", "main")
	runGit(b.root, "config", "metasystem.steward.landing-ref", "refs/remotes/origin/main")
	b.data = []byte(enginebuild.StampRecord(b.commit))
	b.digest = fmt.Sprintf("sha256:%x", sha256.Sum256(b.data))
	if err := testexec.WriteFile(b.artifact, b.data, 0755); err != nil {
		t.Fatal(err)
	}
	b.centralDeployLine(t, b.digest, b.commit)
	// The legacy ledger selects a held goal without manufacturing accepted-goal files.
	injectedRunnerFile(t, filepath.Join(b.root, "plans", "goals.md"), []byte("# Goals\n\n## Current goal: held — Finish the unit\n- Origin: main\n- Next step: Continue.\n"))
	home, err := board.Home()
	if err != nil {
		t.Fatal(err)
	}
	injectedRunnerWrite(t, filepath.Join(home, "unit", "follow-unit", "run.json"), map[string]any{"id": "follow-unit", "goal": "held", "state": "running", "rounds": []launch.UnitRound{{Steps: []launch.UnitStep{{Name: "build", State: launch.StepRunning, LaunchID: "follow-launch", StartedAt: b.now.Format(time.RFC3339)}}}}})
	ref := identity.Exact{Pid: 41, StartedAt: b.now}.Ref()
	record := launch.Record{ID: "follow-launch", State: launch.Running, Supervisor: &ref}
	store := launch.Store{Root: filepath.Join(home, "launch")}
	if err := store.Create(record); err != nil {
		t.Fatal(err)
	}
	admission := func() *intentResult {
		file, err := os.Open(b.binary)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		stamp, err := enginebuild.ReadStamp(file)
		if err != nil {
			t.Fatal(err)
		}
		inv := &intentInvocation{layout: stateroot.Layout{InstallationRoot: stateroottest.Installation(t, b.root)}, owners: intentOwners{work: intentWorkOwners{engineStamp: stamp, git: func(root string, args ...string) ([]byte, error) { return git(t.Context(), root, args...) }}}}
		return inv.buildEngineAdmission()
	}
	starts := 0
	ops := steward.RearmOperations{Clock: centralFollowClock{&workClock{now: b.now}}, Git: git, Prober: centralFollowProbe(b.now)}
	b.centralRun(t, 10, func() error { starts++; return nil }, nil, ops)
	if data, _ := os.ReadFile(b.binary); bytes.Equal(data, b.data) || starts != 0 {
		t.Fatalf("followed live launch: starts=%d bin=%q", starts, data)
	}
	if result := admission(); result == nil || !strings.Contains(strings.Join(result.Details, " "), "BUILD_ENGINE_STALE") {
		t.Fatalf("live launch admission=%+v", result)
	}
	record.FinishedAt = b.now.Format(time.RFC3339)
	// The unit step remains running between rounds; the ended launch is the boundary.
	injectedRunnerWrite(t, filepath.Join(home, "launch", record.ID, "record.json"), record)
	if err := os.Remove(filepath.Join(b.root, "artifacts", "agents", "steward", "stop")); err != nil {
		t.Fatal(err)
	}
	b.centralRun(t, 1, func() error { starts++; return nil }, nil, ops)
	if data, _ := os.ReadFile(b.binary); !bytes.Equal(data, b.data) || starts != 1 {
		log, _ := os.ReadFile(filepath.Join(b.root, "artifacts", "agents", "steward", "runner.log"))
		t.Fatalf("ended launch: starts=%d bin=%q log=%s", starts, data, log)
	}
	if result := admission(); result != nil {
		t.Fatalf("deployed admission=%+v", result)
	}
}
