package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/repoproof"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testrun"
)

func TestRepositoryProofSettingsCheckAndHostFact(t *testing.T) {
	t.Parallel()
	b := newWorkBed(t)
	root, owners := b.root(), b.workOwners()
	owners.work.config = configSettingWithDefault
	owners.contractReady = func(root string, _ bool) (string, int, error) {
		_, contract, path, err := testrun.LoadContract(root)
		return path, len(contract.Groups), err
	}
	conf, err := os.ReadFile("../../metasystem.conf")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), conf, 0o644); err != nil {
		t.Fatal(err)
	}
	contract, err := os.ReadFile("../../testing.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "testing.json"), contract, 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := runIntentIn(mustIntentCommand(t, "settings check"), nil, &stdout, &stderr, root, owners)
	if code != 0 {
		t.Fatalf("settings check: %d %s%s", code, &stdout, &stderr)
	}
	for _, tc := range []struct{ local, value, source string }{{"", "", "default"}, {"host.proof-vm=fixture-vm\n", "fixture-vm", "conf-local"}} {
		if err := os.WriteFile(filepath.Join(root, "metasystem.conf.local"), []byte(tc.local), 0o600); err != nil {
			t.Fatal(err)
		}
		stdout.Reset()
		stderr.Reset()
		code = runIntentIn(mustIntentCommand(t, "settings show"), []string{"host.proof-vm", "--json"}, &stdout, &stderr, root, owners)
		var result struct {
			Data struct{ Settings []launch.Setting }
		}
		if err := json.Unmarshal(stdout.Bytes(), &result); err != nil || code != 0 || len(result.Data.Settings) != 1 || result.Data.Settings[0].Value != tc.value || result.Data.Settings[0].Source != tc.source {
			t.Fatalf("host fact: %d %s%s, %v", code, &stdout, &stderr, err)
		}
	}
}

func TestRepositoryLandingProofRunsHostSuiteAndReports(t *testing.T) {
	t.Parallel()
	b := newResolveVerbFixture(t)
	conf, err := os.ReadFile("../../metasystem.conf")
	if err != nil {
		t.Fatal(err)
	}
	script, err := os.ReadFile("../../proof/full.sh")
	if err != nil {
		t.Fatal(err)
	}
	tools, binary := repositoryProofTools(t)
	b.owners.landing.plainProve = plain.ProveSeams{Now: func() time.Time { return laneTestNow }, Git: func(_ string, args ...string) (string, error) {
		switch strings.Join(args, " ") {
		case "rev-parse --verify HEAD^{commit}":
			return "head", nil
		case "rev-parse --verify HEAD^{tree}":
			return "tree", nil
		case "show head:metasystem/metasystem.conf":
			return string(conf), nil
		case "cat-file -e head^{commit}", "worktree prune":
			return "", nil
		case "show origin/main:metasystem/testing.json", "show origin/main:metasystem/plans/goals/trunk-red.json":
			return "", errors.New("not declared")
		}
		if len(args) == 5 && args[0] == "worktree" && args[1] == "add" {
			dir := filepath.Join(args[3], "metasystem", "proof")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return "", err
			}
			if err := os.WriteFile(filepath.Join(dir, "full.sh"), script, 0o644); err != nil {
				return "", err
			}
			contract, err := os.ReadFile("../../testing.json")
			if err != nil {
				return "", err
			}
			return "", os.WriteFile(filepath.Join(dir, "..", "testing.json"), contract, 0o644)
		}
		if len(args) == 4 && args[0] == "worktree" && args[1] == "remove" {
			return "", nil
		}
		return "", fmt.Errorf("unexpected stub Git: %v", args)
	}, Command: func(command *exec.Cmd) error {
		command.Env = append(command.Env, "PATH="+tools+":/usr/bin:/bin", "REPOSITORY_PROOF_TEST_BINARY="+binary, "REPOSITORY_PROOF_TEST_PROCESS=1")
		return command.Run()
	}}
	b.owners.resolver = stateroot.NewResolver(fakeTop(b.root), noExecutable)
	code, output := b.run(t, b.root, "prove", "--wait", "--json")
	var result struct{ Data plain.Result }
	if err := json.Unmarshal([]byte(output), &result); err != nil || code != 0 || result.Data.Result != plain.Green || !result.Data.CountedFull {
		t.Fatalf("landing prove: %d %s, %v", code, output, err)
	}
	log, err := os.ReadFile(result.Data.Log)
	if err != nil || !strings.Contains(string(log), `"Package":"fixture/package"`) || !strings.HasSuffix(string(log), "LANDING-CHECKED\t0\n") {
		t.Fatalf("proof log: %q, %v", log, err)
	}
}

func TestRepositoryBatchtestFailureReplaysAndStaysRed(t *testing.T) {
	t.Parallel()
	for _, failure := range []string{"test", "package", "missing"} {
		t.Run(failure, func(t *testing.T) {
			t.Parallel()
			b := newReplayVerbBed(t)
			if err := os.WriteFile(filepath.Join(b.install, "metasystem.conf"), []byte("proof.full=sh proof/full.sh\n"), 0644); err != nil {
				t.Fatal(err)
			}
			git := b.owners.landing.plainProve.Git
			b.owners.landing.plainProve.Git = func(root string, args ...string) (string, error) {
				output, err := git(root, args...)
				if err == nil && len(args) == 5 && args[0] == "worktree" && args[1] == "add" {
					for _, file := range []string{"proof/full.sh", "testing.json"} {
						data, readErr := os.ReadFile(filepath.Join("../..", file))
						if readErr != nil {
							return "", readErr
						}
						path := filepath.Join(args[3], "metasystem", file)
						if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
							return "", err
						}
						if err := os.WriteFile(path, data, 0644); err != nil {
							return "", err
						}
					}
				}
				return output, err
			}
			tools, binary := repositoryProofTools(t)
			b.fail = func(command *exec.Cmd, _ string) (string, error) {
				command.Env = append(command.Env, "PATH="+tools+":/usr/bin:/bin", "REPOSITORY_PROOF_TEST_BINARY="+binary, "REPOSITORY_PROOF_TEST_PROCESS=1", "REPOSITORY_PROOF_BATCH_FAILURE="+failure)
				var output bytes.Buffer
				stdout := command.Stdout
				defer func() { command.Stdout = stdout }()
				command.Stdout = &output
				err := command.Run()
				return output.String(), err
			}
			result := b.prove(t)
			if result.Cause == nil || result.Cause.Kind != "main" || result.Repeat != "" || !result.CountedFull || len(result.Failed) != 1 || result.Failed[0].Unit != "go-batchtest" {
				t.Fatalf("batchtest classification: %+v runs=%v", result, b.runs)
			}
			if !reflect.DeepEqual(b.runs, []string{"merge-b:", "main:go-batchtest"}) {
				t.Fatalf("batchtest replay ran the wrong selection: %v", b.runs)
			}
			log, err := os.ReadFile(result.Cause.Evidence)
			if err != nil || !strings.Contains(string(log), "landing group go-batchtest red") || !strings.HasSuffix(string(log), "LANDING-FAILED\tgo-batchtest\t\nLANDING-CHECKED\t1\n") {
				t.Fatalf("batchtest replay lost its red: %s, %v", log, err)
			}
		})
	}
}

func repositoryProofTools(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range []string{"go", "metasystem"} {
		body := "#!/bin/sh\nexec \"$REPOSITORY_PROOF_TEST_BINARY\" -test.run '^TestRepositoryProofProcess$' -- " + tool + " \"$@\"\n"
		if err := testexec.WriteFile(filepath.Join(dir, tool), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return dir, binary
}

func repositoryProofScript(t *testing.T) string {
	t.Helper()
	reporter := "../../proof/.full-reporter"
	before, beforeErr := os.ReadFile(reporter)
	if beforeErr != nil && !os.IsNotExist(beforeErr) {
		t.Fatal(beforeErr)
	}
	beforeInfo, statErr := os.Stat(reporter)
	if statErr != nil && !os.IsNotExist(statErr) {
		t.Fatal(statErr)
	}
	t.Cleanup(func() {
		afterInfo, afterStatErr := os.Stat(reporter)
		if statErr == nil && (afterStatErr != nil || beforeInfo.Mode() != afterInfo.Mode() || !beforeInfo.ModTime().Equal(afterInfo.ModTime())) {
			t.Error("repository reporter metadata changed")
		}
		after, afterErr := os.ReadFile(reporter)
		if !bytes.Equal(before, after) || os.IsNotExist(beforeErr) != os.IsNotExist(afterErr) {
			t.Errorf("repository reporter changed: before error=%v after error=%v", beforeErr, afterErr)
		}
	})
	root := t.TempDir()
	for _, file := range []string{"proof/full.sh", "testing.json"} {
		data, err := os.ReadFile(filepath.Join("../..", file))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, file)), 0755); err != nil {
			t.Fatal(err)
		}
		if err := testexec.WriteFile(filepath.Join(root, file), data, 0755); err != nil {
			t.Fatal(err)
		}
	}
	return filepath.Join(root, "proof/full.sh")
}

func TestRepositoryProofScriptKeepsFailureReportLast(t *testing.T) {
	t.Parallel()
	tools, binary := repositoryProofTools(t)
	command := exec.Command("/bin/sh", repositoryProofScript(t))
	command.Env = append(os.Environ(), "PATH="+tools+":/usr/bin:/bin", "REPOSITORY_PROOF_TEST_BINARY="+binary, "REPOSITORY_PROOF_TEST_PROCESS=1", "REPOSITORY_PROOF_TEST_RED=1", "LANDING_ONLY=")
	output, err := command.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 1 || !strings.HasSuffix(string(output), "LANDING-FAILED\tfixture/package\tTestBroken\nLANDING-CHECKED\t1\n") {
		t.Fatalf("red script: output=%s error=%v", output, err)
	}
}

func TestRepositoryProofScriptHostModeNeedsNoLaneSettings(t *testing.T) {
	t.Parallel()
	tools, binary := repositoryProofTools(t)
	command := exec.Command("/bin/sh", repositoryProofScript(t), "--host")
	command.Env = append(os.Environ(), "PATH="+tools+":/usr/bin:/bin", "REPOSITORY_PROOF_TEST_BINARY="+binary, "REPOSITORY_PROOF_TEST_PROCESS=1", "REPOSITORY_PROOF_TEST_HOST=1", "LANDING_ONLY=")
	output, err := command.CombinedOutput()
	if err != nil || !strings.HasSuffix(string(output), "LANDING-CHECKED\t0\n") {
		t.Fatalf("host script: output=%s error=%v", output, err)
	}
}

func TestRepositoryProofProcess(t *testing.T) {
	t.Parallel()
	if os.Getenv("REPOSITORY_PROOF_TEST_PROCESS") != "1" {
		return
	}
	args := os.Args
	for len(args) > 0 && args[0] != "--" {
		args = args[1:]
	}
	if len(args) < 2 {
		os.Exit(2)
	}
	args = args[1:]
	if os.Getenv("REPOSITORY_PROOF_TEST_HOST") == "1" && args[0] == "metasystem" {
		fmt.Fprintln(os.Stderr, "the extracted candidate cannot consult host lane settings")
		os.Exit(1)
	}
	switch {
	case reflect.DeepEqual(args, []string{"proof", "--host"}):
		os.Exit(repoproof.RunHost(os.Stdout, os.Stderr, os.Getenv, repoproof.Execute, repositoryProofRunners()))
	case len(args) == 5 && reflect.DeepEqual(args[:3], []string{"go", "build", "-o"}) && args[4] == "./proof":
		body := "#!/bin/sh\nexec \"$REPOSITORY_PROOF_TEST_BINARY\" -test.run '^TestRepositoryProofProcess$' -- proof \"$@\"\n"
		if err := testexec.WriteFile(args[3], []byte(body), 0o755); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if err := os.Chmod(args[3], 0o755); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case reflect.DeepEqual(args, []string{"proof"}):
		os.Exit(repoproof.Run(os.Stdout, os.Stderr, os.Getenv, repoproof.Execute, repositoryProofRunners()))
	case reflect.DeepEqual(args, []string{"go", "test", "-json", "-count=1", "-timeout", "30m", "./..."}):
		if os.Getenv("REPOSITORY_PROOF_TEST_RED") == "1" {
			fmt.Println("{\"Action\":\"fail\",\"Package\":\"fixture/package\",\"Test\":\"TestBroken\"}\n{\"Action\":\"fail\",\"Package\":\"fixture/package\"}")
			os.Exit(1)
		}
		fmt.Println(`{"Action":"pass","Package":"fixture/package"}`)
	case reflect.DeepEqual(args, []string{"go", "test", "-json", "-count=1", "-timeout", "30m", "-tags", "batchtest", "./cmd/metasystem/", "./internal/landing/..."}):
		fmt.Println(`{"Action":"pass","Package":"fixture/batch"}`)
	case reflect.DeepEqual(args, []string{"go", "run", "./cmd/devgate", "static"}):
		fmt.Println("fixture static passed")
	case len(args) == 3 && reflect.DeepEqual(args[:2], []string{"proof", "--section"}):
		fmt.Fprintf(os.Stdout, `{"data":{"groups":[{"id":%q,"status":"passed","nativeLaunched":true,"nativeExitStatus":0}]}}`+"\n", args[2])
	case reflect.DeepEqual(args, []string{"metasystem", "landing", "status", "--json"}):
		fmt.Println(`{"data":{"root":"/fixture-lane"}}`)
	case reflect.DeepEqual(args, []string{"metasystem", "settings", "show", "host.proof-vm", "--repo", "/fixture-lane", "--json"}):
		fmt.Println(`{"data":{"settings":[{"Key":"host.proof-vm","Value":""}]}}`)
	default:
		fmt.Fprintln(os.Stderr, "unexpected proof command", args)
		os.Exit(2)
	}
	os.Exit(0)
}

func repositoryProofRunners() repoproof.HostRunners {
	return repoproof.HostRunners{Environment: func() (string, error) { return "fixture toolchain", nil }, Native: func(request proofrun.NativeInventoryRequest) (proofrun.NativeInventoryResult, error) {
		result := proofrun.NativeInventoryResult{Execution: []proofrun.PackageExecution{{Package: "fixture/package", Shard: 1, Status: "ok"}}, Output: []byte(`{"Action":"pass","Package":"fixture/package"}` + "\n")}
		if failure := os.Getenv("REPOSITORY_PROOF_BATCH_FAILURE"); failure != "" && len(request.BuildTags) > 0 {
			const pkg = "github.com/widoriezebos/agentic-tools/metasystem/cmd/metasystem"
			result.Failed = true
			result.Execution[0].Package, result.Execution[0].Status = pkg, "fail"
			result.Output = []byte(fmt.Sprintf("{\"Action\":\"fail\",\"Package\":%q}\n", pkg))
			if failure == "test" {
				result.Observed = []proofrun.NativeTestIdentity{{Classname: pkg, Name: "TestBatchBroken", Status: "failed"}}
				result.Output = append([]byte(fmt.Sprintf("{\"Action\":\"fail\",\"Package\":%q,\"Test\":\"TestBatchBroken\"}\n", pkg)), result.Output...)
			}
			if failure == "missing" {
				result.Execution[0].Status = "missing"
				result.Missing = []proofrun.NativeTestIdentity{{Classname: pkg, Name: "TestBatchBroken", Status: "missing"}}
			}
		}
		if os.Getenv("REPOSITORY_PROOF_TEST_RED") == "1" && len(request.BuildTags) == 0 {
			result.Failed = true
			result.Output = []byte("{\"Action\":\"fail\",\"Package\":\"fixture/package\",\"Test\":\"TestBroken\"}\n{\"Action\":\"fail\",\"Package\":\"fixture/package\"}\n")
			result.Execution[0].Status = "fail"
			result.Observed = []proofrun.NativeTestIdentity{{Classname: "fixture/package", Name: "TestBroken", Status: "failed"}}
		}
		return result, nil
	}, Groups: func(ids []string) ([]proofrun.NamedGroupResult, error) {
		if os.Getenv("REPOSITORY_PROOF_BATCH_FAILURE") == "" || !reflect.DeepEqual(ids, []string{"go-batchtest"}) {
			return nil, fmt.Errorf("unexpected replay groups: %v", ids)
		}
		return []proofrun.NamedGroupResult{{ID: "go-batchtest", Status: "red", Output: "batchtest failure"}}, nil
	}}
}
