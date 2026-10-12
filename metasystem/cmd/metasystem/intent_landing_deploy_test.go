package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/conflict"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginebuild"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot/stateroottest"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/up"
)

func landingDeployBed(t *testing.T) (*laneVerbBed, intentOwners, *int) {
	t.Helper()
	base := t.TempDir()
	b := &laneVerbBed{cwd: filepath.Join(base, "lane"), landingA: filepath.Join(base, "lane"), home: filepath.Join(base, "home")}
	helmMust(t, os.MkdirAll(filepath.Join(b.landingA, "metasystem"), 0755))
	helmMust(t, os.WriteFile(filepath.Join(b.landingA, "metasystem", "metasystem.conf"), []byte("metasystem.template=true\n"), 0600))
	b.landingA, b.cwd = realpath.Resolve(b.landingA), realpath.Resolve(b.cwd)
	b.cwd = b.landingA
	install := filepath.Join(b.landingA, "metasystem")
	b.prove = enrolledPersonProver(t, install, laneTestNow)
	data, err := json.Marshal(lane.Record{Root: b.landingA, Install: install, CustodyEpoch: 1, RegisteredBy: "Wido"})
	helmMust(t, err)
	helmMust(t, os.MkdirAll(lane.HostDir(b.home), 0755))
	helmMust(t, os.WriteFile(lane.RecordPath(b.home), data, 0600))
	helmMust(t, os.MkdirAll(filepath.Join(install, "bin"), 0755))
	builds := new(int)
	b.plainProve = plain.ProveSeams{Git: func(_ string, args ...string) (string, error) {
		if args[0] == "status" {
			return "", nil
		}
		if slices.Contains(args, "--show-prefix") {
			return "metasystem/", nil
		}
		if strings.Contains(strings.Join(args, " "), "MERGE_HEAD") {
			return "", exec.Command("false").Run()
		}
		return "main", nil
	}, Command: func(command *exec.Cmd) error {
		if command.Dir != install {
			t.Fatalf("wrong build root: %s", command.Dir)
		}
		*builds++
		if len(command.Args) != 6 || command.Args[4] != "--out" {
			t.Fatalf("build not staged: %v", command.Args)
		}
		return os.WriteFile(command.Args[5], []byte(enginebuild.StampRecord("main")), 0600)
	}}
	owners := b.owners()
	owners.resolver = stateroot.NewResolver(func(string) (string, error) { return b.landingA, nil }, os.Executable)
	owners.landing.restartSupervisor = func(string) error { return nil }
	return b, owners, builds
}

func runLandingDeploy(t *testing.T, b *laneVerbBed, owners intentOwners, flags ...string) (int, intentResult) {
	t.Helper()
	command, ok := findIntentAction("landing", "deploy")
	if !ok {
		t.Fatal("landing deploy was not registered")
	}
	var out, stderr bytes.Buffer
	code := runIntentIn(command, append(flags, "--json"), &out, &stderr, b.cwd, owners)
	var result intentResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("exit=%d decode=%v stdout=%s stderr=%s", code, err, &out, &stderr)
	}
	return code, result
}

func landingDeployRepeat(t *testing.T) {
	t.Helper()
	b, owners, builds := landingDeployBed(t)
	code, first := runLandingDeploy(t, b, owners, "--by", "Wido")
	if code != 0 || first.Outcome != intentConfirmed || *builds != 1 {
		t.Fatalf("deploy: %d %+v builds=%d", code, first, *builds)
	}
	before := idemTreeDigest(t, b.landingA)
	code, second := runLandingDeploy(t, b, owners, "--by", "Wido")
	if code != 0 || second.Outcome != intentUnchanged || second.Summary != "nothing to deploy" || *builds != 1 {
		t.Fatalf("repeat: %d %+v builds=%d", code, second, *builds)
	}
	idemSameTree(t, "repeated lane deploy", before, idemTreeDigest(t, b.landingA))
}

func TestLandingDeployRepeatAsSuccess(t *testing.T) {
	t.Parallel()
	landingDeployRepeat(t)
}

func TestLandingDeployRequiresTheEnrolledTerminal(t *testing.T) {
	t.Parallel()
	b, owners, builds := landingDeployBed(t)
	owners.prove = nil
	code, result := runLandingDeploy(t, b, owners, "--now", "--by", "Wido")
	if code == 0 || result.Outcome != intentRefused || *builds != 0 || !strings.Contains(result.Summary, "enrolled terminal") {
		t.Fatalf("unproven deploy: %d %+v builds=%d", code, result, *builds)
	}
	b, owners, builds = landingDeployBed(t)
	code, result = runLandingDeploy(t, b, owners, "--by", "Another")
	if code == 0 || result.Outcome != intentRefused || *builds != 0 || !strings.Contains(result.Summary, "does not match") {
		t.Fatalf("wrong person deploy: %d %+v", code, result)
	}
}

func TestLandingDeployNowKeepsAPausedLane(t *testing.T) {
	t.Parallel()
	b, owners, _ := landingDeployBed(t)
	_, err := lane.SetPause(b.home, "Wido", laneTestNow)
	helmMust(t, err)
	before := idemTreeDigest(t, b.home)
	code, result := runLandingDeploy(t, b, owners, "--now", "--by", "Wido")
	if code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("paused deploy: %d %+v", code, result)
	}
	idemSameTree(t, "deploy preserves the paused lane", before, idemTreeDigest(t, b.home))
}

func TestLandingDeployReplacesALiveSupervisor(t *testing.T) {
	t.Parallel()
	install := t.TempDir()
	scope := processScope{Checkout: install, Binary: filepath.Join(install, "bin", "metasystem"), Installation: stateroottest.Installation(t, install), Root: stateroottest.State(t, install)}
	called := false
	effects := processArmEffects{seed: func(string) (stewardLandingRefSeed, error) { return stewardLandingRefSeed{}, nil }, arm: func(root, binary string, _ processArmAuthority) (string, error) {
		if root != install || binary != scope.Binary {
			t.Fatal("wrong arming target")
		}
		return "", nil
	},
		up: func(options up.Options) up.Result {
			called = true
			if !options.IfDown || !options.RecoverOnly || options.Binary != scope.Binary || options.EnsureArmed == nil {
				t.Fatalf("left old supervisor live: %+v", options)
			}
			if _, err := options.EnsureArmed(supervise.EnsureOptions{OnlyIfDown: true}); err != nil {
				t.Fatal(err)
			}
			return up.Result{Outcome: "recovered"}
		},
	}
	replacements := 0
	ensure := func(options supervise.EnsureOptions) (supervise.EnsureResult, error) {
		replacements++
		if options.OnlyIfDown {
			t.Fatal("kept the old supervisor")
		}
		return supervise.EnsureResult{Action: "replaced"}, nil
	}
	if _, err := restartLandingSupervisor(scope, effects, ensure); err != nil || !called || replacements != 1 {
		t.Fatalf("supervisor: %v called=%t", err, called)
	}
}

func TestLandingDeployWaitsForTheRecordedGenerator(t *testing.T) {
	t.Parallel()
	install := t.TempDir()
	log := filepath.Join(install, "generator.log")
	helmMust(t, os.WriteFile(log, nil, 0600))
	helmMust(t, os.MkdirAll(plain.Dir(install), 0755))
	data, err := json.Marshal(plain.RunningRegeneration{Goal: "G", SHA: "tip", Log: log, Pid: 1234, Process: "writer", Since: laneTestNow.Format("2006-01-02T15:04:05Z07:00")})
	helmMust(t, err)
	helmMust(t, os.WriteFile(filepath.Join(plain.Dir(install), "regenerating.json"), data, 0600))
	for _, state := range []string{"resolving", "abandoned"} {
		for _, alive := range []bool{true, false} {
			settled, err := landingDeployMergeSettled(install, &plain.Fix{State: state}, plain.ProveSeams{Alive: func(plain.Running) bool { return alive }, ResumeKeeper: &lane.AgentKeeper{Running: func() (string, bool, error) { return "", false, nil }}})
			if want := state == "abandoned" && !alive; err != nil || settled != want {
				t.Fatalf("fix=%s generator alive=%t settled=%t err=%v", state, alive, settled, err)
			}
		}
	}
}

func TestLandingDeployPreservesLiveJoblessResolutionAfterMainMoves(t *testing.T) {
	t.Parallel()
	b, owners, builds := landingDeployBed(t)
	install := filepath.Join(b.landingA, "metasystem")
	b.alive = true
	fix := &plain.Fix{Attempt: "fix", Goal: "G", Commit: "old", Tip: "goal-tip", State: "resolving", Units: []string{"merge"}, Paths: []conflict.Path{{Path: "file.go"}}}
	helmMust(t, plain.WriteFix(install, fix))
	calls := []string{}
	owners.landing.plainProve.Git = func(_ string, args ...string) (string, error) {
		call := strings.Join(args, " ")
		calls = append(calls, call)
		switch call {
		case "fetch origin main", "status --porcelain=v1 -z --no-renames --untracked-files=all":
			return "", nil
		case "ls-tree --name-only main -- metasystem/plans/goals/trunk-red.json":
			return "", nil
		case "rev-parse --show-prefix":
			return "metasystem/", nil
		case "rev-parse --verify origin/main^{commit}":
			return "main", nil
		case "rev-parse --verify --quiet refs/heads/main^{commit}", "rev-parse --verify --quiet refs/remotes/origin/main^{commit}":
			return "main", nil
		case "symbolic-ref --quiet --short HEAD":
			return "main", nil
		case "rev-parse --verify MERGE_HEAD^{commit}":
			return "goal-tip", nil
		case "rev-parse --verify --quiet MERGE_HEAD^{commit}":
			return "goal-tip", nil
		case "rev-parse --verify HEAD^{commit}":
			return "old", nil
		case "diff --name-only --no-renames old main":
			return "metasystem/internal/file.go", nil
		case "merge --abort", "checkout --detach main":
			return "", nil
		default:
			t.Fatalf("unexpected Git: %s", call)
			return "", nil
		}
	}
	code, result := runLandingDeploy(t, b, owners, "--by", "Wido")
	after, err := plain.ReadFix(install)
	if code == 0 || result.Outcome != intentRefused || !strings.Contains(result.Summary, "give it back or wait") || after == nil || after.State != "resolving" || err != nil || *builds != 0 || slices.Contains(calls, "merge --abort") {
		t.Fatalf("live resolution discarded: exit=%d result=%+v fix=%+v err=%v calls=%v", code, result, after, err, calls)
	}
}
