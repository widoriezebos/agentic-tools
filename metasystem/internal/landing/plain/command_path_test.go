package plain

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

var declaredPathStage = flag.String("declared-path-stage", "", "fixture command stage")

func TestDeclaredProofAndFixFindRunningEngine(t *testing.T) {
	t.Parallel()
	if *declaredPathStage == "run" {
		runPinnedInstallationFixture(t)
		return
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	pin := filepath.Join(t.TempDir(), "generation-7-hash")
	if err := testexec.WriteFile(pin, data, 0700); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"competing", "absent-path", "missing", "not-executable"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			competitor := filepath.Join(t.TempDir(), "metasystem")
			marker := filepath.Join(t.TempDir(), "competitor-ran")
			if err := testexec.WriteFile(competitor, []byte("#!/bin/sh\nprintf competitor > '"+marker+"'\nexit 92\n"), 0700); err != nil {
				t.Fatal(err)
			}
			child := exec.CommandContext(t.Context(), pin, "-test.run=^TestDeclaredProofAndFixFindRunningEngine$", "-test.timeout=30m", "-declared-path-stage=run")
			for _, entry := range os.Environ() {
				if !strings.HasPrefix(entry, "PATH=") {
					child.Env = append(child.Env, entry)
				}
			}
			if mode != "absent-path" {
				child.Env = append(child.Env, "PATH="+filepath.Dir(competitor))
			}
			child.Env = append(child.Env, "LANE_PATH_MODE="+mode)
			if output, err := child.CombinedOutput(); err != nil {
				t.Fatalf("pinned engine %s: %v\n%s", mode, err, output)
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatalf("competing engine ran: %v", err)
			}
		})
	}
}

func runPinnedInstallationFixture(t *testing.T) {
	t.Helper()
	engine, _ := os.Executable()
	if filepath.Base(engine) != "generation-7-hash" {
		t.Fatalf("fixture is not pinned: %s", engine)
	}
	checkout, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	install := filepath.Join(checkout, "metasystem")
	home := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "engine.log")
	fixtureEngine(t, install)
	binary := filepath.Join(install, "bin", "metasystem")
	// Record the literal argument and the execution directory in the child itself.
	script := "#!/bin/sh\nprintf '%s|%s\\n' \"$1\" \"$PWD\" >> '" + logPath + "'\n"
	if err := testexec.WriteFile(binary, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	mode := os.Getenv("LANE_PATH_MODE")
	if mode == "missing" {
		if err := os.Remove(binary); err != nil {
			t.Fatal(err)
		}
	}
	if mode == "not-executable" {
		if err := os.Chmod(binary, 0600); err != nil {
			t.Fatal(err)
		}
	}
	missing := mode == "missing" || mode == "not-executable"
	git := stubGit{commit: "main", tree: "main-tree", main: "main", installPrefix: "metasystem"}
	seams := ProveSeams{Git: git.run, Now: func() time.Time { return bedNow }, Trunk: true}
	command := "set -e; metasystem 'literal; $(no shell)'; printf 'LANDING-CHECKED\\t0\\n'"
	if mode == "absent-path" {
		command = "set -e; env metasystem 'literal; $(no shell)'; printf 'LANDING-CHECKED\\t0\\n'"
	}
	result, err := Run(install, checkout, command, "", io.Discard, seams)
	if err != nil {
		t.Fatal(err)
	}
	persisted, ok, err := LastResult(install)
	if err != nil || !ok || persisted.Result != result.Result {
		t.Fatalf("saved result: %+v %v", persisted, err)
	}
	if missing {
		if result.Result != Red || !result.didNotRun() || result.CountedFull || result.Repeat != "allowed" || !strings.Contains(result.Reason, "not executable") {
			t.Fatalf("invalid installation: %+v", result)
		}
		if _, err := os.Stat(logPath); !os.IsNotExist(err) {
			t.Fatalf("child started: %v", err)
		}
		current, recorded, _, readErr := ReadRunning(install, seams)
		if readErr != nil || !recorded || current.ProofProcess != "" {
			t.Fatalf("invalid installation started a child: %+v %v", current, readErr)
		}
		assertUnrunMain(t, install, checkout, result)
		return
	}
	if result.Result != Green || !result.CountedFull {
		t.Fatalf("proof: %+v", result)
	}
	data, err := os.ReadFile(logPath)
	if err != nil || !strings.HasPrefix(string(data), "literal; $(no shell)|") || strings.Contains(string(data), "|"+install+"\n") {
		t.Fatalf("proof did not use detached execution directory: %s %v", data, err)
	}
	// Start keeps launching the pinned engine while the declared command finds bin.
	launchSeams := seams
	launchSeams.Executable = os.Executable
	launchSeams.Fresh = true
	launchSeams.Launch = func(argv []string, _, _ string) (int64, error) {
		if argv[0] != engine {
			t.Fatalf("Start launched %q, want pin %q", argv[0], engine)
		}
		return int64(os.Getpid()), nil
	}
	if _, _, err := Start(install, checkout, launchSeams); err != nil {
		t.Fatal(err)
	}
	if err := removeRunning(install, seams); err != nil {
		t.Fatal(err)
	}
	cwd, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(lane.LockPath(home)), 0700); err != nil {
		t.Fatal(err)
	}
	log, err := os.CreateTemp(t.TempDir(), "regeneration.log")
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	for _, argv := range [][]string{{"metasystem", "literal; $(no shell)"}, {"env", "metasystem", "literal; $(no shell)"}, {"/bin/sh", "-c", "metasystem 'literal; $(no shell)'"}} {
		if err := runRegeneration(home, install, RunningRegeneration{Goal: "fixture", Log: log.Name()}, argv, cwd, log, nil); err != nil {
			t.Fatal(err)
		}
	}
	data, err = os.ReadFile(logPath)
	if err != nil || strings.Count(string(data), "literal; $(no shell)|"+cwd+"\n") != 3 {
		t.Fatalf("regeneration engine/argv/cwd: %s %v", data, err)
	}
	// A saved batch failure goes through the classifier's detached replay.
	saved := Result{Attempt: "saved-red", Commit: "tip", Tree: "tip-tree", Result: Red, Scope: "impact", Cause: &Cause{Kind: "unclassified"}, Failed: []FailedUnit{{Unit: "fixture/unit"}}}
	if err := appendLine(resultsPath(install), saved); err != nil {
		t.Fatal(err)
	}
	classify := seams
	classify.Trunk = false
	classify.Person = &ActProvenance{Person: "fixture", Kind: "classification"}
	notAncestor := exec.Command("/usr/bin/false").Run()
	classify.Git = func(dir string, args ...string) (string, error) {
		switch strings.Join(args, " ") {
		case "fetch origin main", "cat-file -e base^{commit}", "cat-file -e tip^{commit}":
			return "", nil
		case "rev-parse --verify --quiet refs/remotes/origin/main^{commit}":
			return "base", nil
		case "rev-parse --verify base^{tree}":
			return "base-tree", nil
		case "show -s --format=%P tip":
			return "base", nil
		case "merge-base --is-ancestor tip base":
			return "", notAncestor
		case "log --first-parent --merges --reverse --format=%H %P base..tip":
			return "", nil
		}
		return git.run(dir, args...)
	}
	classify.NewID = func() string { return "classify" }
	classify.CommandForCommit = func(string) (string, error) {
		return "set -e; metasystem 'saved replay'; printf 'LANDING-FAILED\\tfixture/unit\\t\\nLANDING-CHECKED\\t1\\n'; exit 1", nil
	}
	classified, err := ClassifyAttempt(install, checkout, saved.Attempt, classify)
	if err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(logPath)
	if err != nil || classified.Cause == nil || classified.Cause.Kind != "main" || !strings.Contains(string(data), "saved replay|") {
		t.Fatalf("saved replay did not use installation: %+v %s %v", classified, data, err)
	}
	current, ok, _, err := ReadRunning(install, classify)
	if err != nil || !ok || current.ProofProcess == "" {
		t.Fatalf("classification lost execution ownership: %+v %v", current, err)
	}
	// A saved trunk failure also repeats a named unit in its detached worktree.
	savedRepeat := Result{Attempt: "saved-repeat", Commit: "main", Tree: "repeat-tree", Result: Red, Trunk: true, Scope: "impact", Cause: &Cause{Kind: "unclassified"}, Failed: []FailedUnit{{Unit: "fixture/unit", Tests: []string{"TestA"}}}}
	if err := appendLine(resultsPath(install), savedRepeat); err != nil {
		t.Fatal(err)
	}
	classify.NewID = func() string { return "repeat-classify" }
	classify.CommandForCommit = func(string) (string, error) {
		return "set -e; metasystem 'saved repeat'; printf 'LANDING-FAILED\\tfixture/unit\\tTestA\\nLANDING-CHECKED\\t1\\n'; exit 1", nil
	}
	repeated, err := ClassifyAttempt(install, checkout, savedRepeat.Attempt, classify)
	if err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(logPath)
	if err != nil || repeated.Cause == nil || repeated.Cause.Kind != "main" || !strings.Contains(string(data), "saved repeat|") {
		t.Fatalf("saved repeat did not use installation: %+v %s %v", repeated, data, err)
	}
	current, ok, _, err = ReadRunning(install, classify)
	if err != nil || !ok || current.ProofProcess == "" {
		t.Fatalf("repeat classification lost ownership: %+v %v", current, err)
	}
	// Stop after the repair command, before any merge mutation.
	fix := &Fix{Goal: "fixture", Tip: "tip", Commit: "base", State: "resolving", Attempt: "repair"}
	if err := WriteFix(install, fix); err != nil {
		t.Fatal(err)
	}
	afterLaunch := fmt.Errorf("fixture stopped after the repair check")
	repairSeams := ResolveSeams{Git: func(string, ...string) (string, error) { return "", afterLaunch }}
	err = commitResolvedMerge(home, install, checkout, fix, "metasystem 'repair literal'", repairSeams, func(...string) (string, error) { return "", afterLaunch })
	if !errors.Is(err, afterLaunch) {
		t.Fatalf("repair did not reach its post-check boundary: %v", err)
	}
	data, err = os.ReadFile(logPath)
	if err != nil || !strings.Contains(string(data), "repair literal|"+install+"\n") {
		t.Fatalf("repair did not use installation: %s %v", data, err)
	}
	fmt.Println("pinned proof, saved replay, repair and literal regeneration used the registered installation")
}
