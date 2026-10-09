package repoproof

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestBatchPassRunsOnlyTaggedTestsAndReportsEmptyPass(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"tagged", "empty", "failure"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			files := map[string]string{
				"go.mod":                       "module example.com/batch\n\ngo 1.23\n",
				"cmd/metasystem/plain_test.go": "package main\nimport \"testing\"\nfunc TestPlain(t *testing.T) { t.Parallel() }\n",
			}
			if mode != "empty" {
				body := ""
				if mode == "failure" {
					body = `t.Fatal("tagged failure")`
				}
				files["cmd/metasystem/batch_test.go"] = "//go:build batchtest\n\npackage main\nimport \"testing\"\nfunc TestBatch(t *testing.T) { t.Parallel(); " + body + " }\n"
			}
			for name, source := range files {
				path := filepath.Join(root, name)
				if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(source), 0600); err != nil {
					t.Fatal(err)
				}
			}
			calls := 0
			hooks := HostRunners{Environment: func() (string, error) { return "fixture", nil }, Native: func(request proofrun.NativeInventoryRequest) (proofrun.NativeInventoryResult, error) {
				calls++
				request.Root, request.Workers = root, 2
				request.Environment = proofrun.TestingEnvironment(os.Environ(), map[string]string{"GOWORK": "off", "GOTOOLCHAIN": "local", "GOPROXY": "off", "GOSUMDB": "off"})
				result, err := proofrun.RunNativeInventory(t.Context(), request)
				if err != nil {
					t.Fatal(err)
				}
				want := []proofrun.NativeTestIdentity{{Report: "go-test-json", Classname: "example.com/batch/cmd/metasystem", Name: "TestPlain", Status: "passed"}}
				if len(request.BuildTags) > 0 {
					if mode == "empty" {
						if len(result.Execution) != 0 || len(result.Output) != 0 || result.LogPath != "" {
							t.Fatalf("empty batch launched a process: %+v", result)
						}
						want = nil
					} else {
						want[0].Name = "TestBatch"
						if mode == "failure" {
							want[0].Status = "failed"
						}
						if len(result.Execution) != 1 {
							t.Fatalf("batch execution=%+v", result.Execution)
						}
					}
				}
				if !reflect.DeepEqual(result.Observed, want) || len(result.Missing) != 0 || len(result.Unexpected) != 0 {
					t.Fatalf("observed=%+v want=%+v missing=%+v unexpected=%+v", result.Observed, want, result.Missing, result.Unexpected)
				}
				return result, nil
			}}
			var out, problem bytes.Buffer
			getenv := func(key string) string {
				switch key {
				case "LANDING_PROOF_SCOPE":
					return "scoped"
				case "LANDING_PROOF_PACKAGES":
					return "metasystem/cmd/metasystem"
				}
				return ""
			}
			code := runHost(&out, &problem, getenv, func(argv []string, _, _ io.Writer) error { t.Fatalf("unexpected command %v", argv); return nil }, "../../testing.json", hooks)
			wantExit, status := 0, "green"
			if mode == "failure" {
				wantExit, status = 1, "red"
				if !strings.Contains(out.String(), "LANDING-FAILED\tgo-batchtest\tTestBatch\n") {
					t.Fatal(out.String())
				}
			}
			if code != wantExit || calls != 2 || !strings.Contains(out.String(), "landing group go-batchtest "+status+" ") {
				t.Fatalf("exit=%d calls=%d output=%s error=%s", code, calls, &out, &problem)
			}
		})
	}
}

func TestFullPartitionsRunEveryPackageOnceAndAPanicLosesOneUnit(t *testing.T) {
	t.Parallel()
	for _, panicTest := range []bool{false, true} {
		t.Run(fmt.Sprintf("panic=%t", panicTest), func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			panicBody := ""
			if panicTest {
				panicBody = `panic("deliberate panic")`
			}
			for name, source := range map[string]string{
				"go.mod": "module example.com/proof\n\ngo 1.23\n",
				"cmd/metasystem/main_test.go": `package main
import "testing"
func TestA(t *testing.T) { t.Parallel(); ` + panicBody + ` }
func TestB(t *testing.T) { t.Parallel() }
func TestC(t *testing.T) { t.Parallel() }
func TestD(t *testing.T) { t.Parallel() }
func TestE(t *testing.T) { t.Parallel() }
func TestF(t *testing.T) { t.Parallel() }
func TestG(t *testing.T) { t.Parallel() }
func TestH(t *testing.T) { t.Parallel() }
func TestI(t *testing.T) { t.Parallel() }
func TestJ(t *testing.T) { t.Parallel() }
func TestK(t *testing.T) { t.Parallel() }
func TestL(t *testing.T) { t.Parallel() }
`,
				"internal/launch/launch_test.go": `package launch
import "testing"
func TestA(t *testing.T) { t.Parallel() }
func TestB(t *testing.T) { t.Parallel() }
`,
				"internal/empty/empty.go": "package empty\n",
			} {
				path := filepath.Join(root, name)
				if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(source), 0600); err != nil {
					t.Fatal(err)
				}
			}
			goPath, err := exec.LookPath("go")
			if err != nil {
				t.Fatal(err)
			}
			tools := t.TempDir()
			if err := os.Symlink(goPath, filepath.Join(tools, "go")); err != nil {
				t.Fatal(err)
			}
			environment := proofrun.TestingEnvironment(os.Environ(), map[string]string{
				"PATH": tools, "GOWORK": "off", "GOTOOLCHAIN": "local", "GOPROXY": "off", "GOSUMDB": "off",
			})
			const big = "example.com/proof/cmd/metasystem"
			natives, static, sections := 0, false, map[string]int{}
			hooks := HostRunners{Environment: func() (string, error) { return "fixture", nil }, Native: func(request proofrun.NativeInventoryRequest) (proofrun.NativeInventoryResult, error) {
				natives++
				if !static {
					t.Fatal("static did not run first")
				}
				request.Root, request.Environment, request.Workers = root, environment, 2
				// The batch pass has no tagged-only tests in this fixture.
				if len(request.BuildTags) > 0 {
					request.Packages = []string{"cmd/metasystem", "internal/launch"}
				}
				result, err := proofrun.RunNativeInventory(t.Context(), request)
				if err != nil {
					t.Fatal(err)
				}
				counts := map[string]int{}
				for _, unit := range result.Execution {
					counts[unit.Package]++
					if unit.Package != big && unit.Status != "ok" {
						t.Fatalf("panic lost another package: %+v", unit)
					}
				}
				want := map[string]int{}
				if len(request.BuildTags) == 0 {
					want = map[string]int{big: 6, "example.com/proof/internal/launch": 1}
				}
				if len(request.BuildTags) == 0 {
					want["example.com/proof/internal/empty"] = 1
				}
				if !reflect.DeepEqual(counts, want) {
					t.Fatalf("package runs=%v want=%v", counts, want)
				}
				runs := map[string]int{}
				for _, line := range strings.Split(string(result.Output), "\n") {
					var event struct{ Action, Package, Test string }
					if json.Unmarshal([]byte(line), &event) == nil && event.Action == "run" && event.Test != "" {
						runs[event.Package+"/"+event.Test]++
					}
				}
				for _, name := range []string{"TestA", "TestB", "TestC", "TestD", "TestE", "TestF", "TestG", "TestH", "TestI", "TestJ", "TestK", "TestL"} {
					want := 0
					if len(request.BuildTags) == 0 {
						want = 1
					}
					if runs[big+"/"+name] != want {
						t.Fatalf("%s ran %d times, want %d", name, runs[big+"/"+name], want)
					}
				}
				for _, name := range []string{"TestA", "TestB"} {
					want := 0
					if len(request.BuildTags) == 0 {
						want = 1
					}
					if runs["example.com/proof/internal/launch/"+name] != want {
						t.Fatalf("small package test did not run exactly once: %v", runs)
					}
				}
				wantFailure := panicTest && len(request.BuildTags) == 0
				if result.Failed != wantFailure || len(result.Unexpected) != 0 || (!wantFailure && len(result.Missing) != 0) {
					t.Fatalf("native result=%+v", result)
				}
				if wantFailure && (len(result.Missing) != 1 || result.Missing[0].Name != "TestG") {
					t.Fatalf("panic must lose only its shard: %+v", result.Missing)
				}
				return result, nil
			}}
			command := func(argv []string, stdout, _ io.Writer) error {
				if reflect.DeepEqual(argv, []string{"go", "run", "./cmd/devgate", "static"}) {
					static = true
					return nil
				}
				if natives != 2 || len(argv) != 3 || argv[1] != "--section" {
					t.Fatalf("section ran before native passes: %v", argv)
				}
				sections[argv[2]]++
				fmt.Fprintf(stdout, `{"data":{"groups":[{"id":%q,"status":"passed","nativeLaunched":true,"nativeExitStatus":0}]}}`, argv[2])
				return nil
			}
			var out, problem bytes.Buffer
			code := runHost(&out, &problem, func(string) string { return "" }, command, "../../testing.json", hooks)
			wantExit := 0
			if panicTest {
				wantExit = 1
			}
			if code != wantExit || natives != 2 || !strings.Contains(out.String(), "landing package example.com/proof/internal/launch ") {
				t.Fatalf("exit=%d native=%d output=%s error=%s", code, natives, &out, &problem)
			}
			contract, err := testpolicy.Load("../../testing.json")
			if err != nil {
				t.Fatal(err)
			}
			for _, group := range contract.Groups {
				if group.Adapter == "section" && sections[group.ID] != 1 {
					t.Fatalf("section %s runs %d", group.ID, sections[group.ID])
				}
			}
			if panicTest {
				for _, line := range []string{
					"LANDING-FAILED\texample.com/proof/cmd/metasystem\tTestA TestG(did not report)\n",
					"LANDING-CHECKED\t1\n",
				} {
					if !strings.Contains(out.String(), line) {
						t.Fatalf("missing %q in %s", line, &out)
					}
				}
			} else if !strings.HasSuffix(out.String(), "LANDING-CHECKED\t0\n") {
				t.Fatal(out.String())
			}
		})
	}
}

func TestFullShardsRunEveryPackageAndAPanicLosesOneShard(t *testing.T) {
	t.Parallel()
	if os.Getenv("FULL_SHARD_BUILD") == "1" {
		var output string
		for index, arg := range os.Args {
			if arg == "-o" && index+1 < len(os.Args) {
				output = os.Args[index+1]
			}
		}
		if output == "" {
			t.Fatal("reporter build did not specify its output path")
		}
		body := "#!/bin/sh\nexec \"$FULL_SHARD_TEST_BINARY\" -test.run '^TestFullShardsRunEveryPackageAndAPanicLosesOneShard$' -- \"$@\"\n"
		if err := testexec.WriteFile(output, []byte(body), 0755); err != nil {
			t.Fatal(err)
		}
		os.Exit(0)
	}
	const pkg = "github.com/widoriezebos/agentic-tools/metasystem/cmd/metasystem"
	for _, mode := range []string{"panic", "TestMain", "unexpected", "scoped package", "scoped named batch", "scoped group", "group replay", "red group", "unknown group"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			if selected := os.Getenv("FULL_SHARD_CASE"); selected != "" && selected != mode {
				return
			}
			if os.Getenv("FULL_SHARD_REPORT") != "1" {
				root := t.TempDir()
				for _, file := range []string{"proof/full.sh", "testing.json"} {
					data, err := os.ReadFile(filepath.Join("../..", file))
					if err != nil {
						t.Fatal(err)
					}
					path := filepath.Join(root, file)
					if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, data, 0644); err != nil {
						t.Fatal(err)
					}
				}
				tools := t.TempDir()
				body := "#!/bin/sh\nFULL_SHARD_BUILD=1 exec \"$FULL_SHARD_TEST_BINARY\" -test.run '^TestFullShardsRunEveryPackageAndAPanicLosesOneShard$' -- \"$@\"\n"
				if err := testexec.WriteFile(filepath.Join(tools, "go"), []byte(body), 0755); err != nil {
					t.Fatal(err)
				}
				binary, err := os.Executable()
				if err != nil {
					t.Fatal(err)
				}
				command := exec.Command("/bin/sh", "proof/full.sh", "--host")
				command.Dir = root
				command.Env = append(os.Environ(), "PATH="+tools+":/usr/bin:/bin", "FULL_SHARD_TEST_BINARY="+binary, "FULL_SHARD_CASE="+mode, "FULL_SHARD_REPORT=1")
				output, err := command.CombinedOutput()
				exit := 0
				if err != nil {
					var failure *exec.ExitError
					if !errors.As(err, &failure) {
						t.Fatal(err)
					}
					exit = failure.ExitCode()
				}
				want := 0
				if mode == "panic" || mode == "TestMain" || mode == "unexpected" || mode == "red group" || mode == "unknown group" {
					want = 1
				}
				if exit != want || !strings.HasPrefix(string(output), "landing environment stable toolchain\n") {
					t.Fatalf("shell exit=%d output=%s err=%v", exit, output, err)
				}
				return
			}
			var out, stderr bytes.Buffer
			env := map[string]string{}
			switch mode {
			case "scoped package":
				env = map[string]string{"LANDING_PROOF_SCOPE": "scoped", "LANDING_PROOF_PACKAGES": "metasystem/internal/launch"}
			case "scoped named batch":
				env = map[string]string{"LANDING_PROOF_SCOPE": "scoped", "LANDING_PROOF_PACKAGES": "metasystem/cmd/metasystem=TestA,TestB"}
			case "scoped group", "red group", "unknown group":
				env = map[string]string{"LANDING_PROOF_SCOPE": "scoped", "LANDING_PROOF_GROUPS": "verb-ratchet"}
				if mode == "unknown group" {
					env["LANDING_PROOF_GROUPS"] = "unknown"
				}
			case "group replay":
				env = map[string]string{"LANDING_PROOF_SCOPE": "scoped", "LANDING_PROOF_GROUPS": "fast-static-build verb-ratchet", "LANDING_ONLY": "verb-ratchet"}
			}
			env["METASYSTEM_FULL_REPORTER"] = os.Getenv("METASYSTEM_FULL_REPORTER")
			static, natives, groups := false, 0, 0
			hooks := HostRunners{Environment: func() (string, error) { return "stable toolchain", nil }, Native: func(r proofrun.NativeInventoryRequest) (proofrun.NativeInventoryResult, error) {
				natives++
				if mode == "scoped package" && (!reflect.DeepEqual(r.Packages, []string{"internal/launch"}) || len(r.BuildTags) != 0) {
					t.Fatalf("scoped inventory %+v", r)
				}
				if mode == "scoped named batch" && (!reflect.DeepEqual(r.Packages, []string{"cmd/metasystem"}) || !reflect.DeepEqual(r.Tests, []string{"TestA", "TestB"})) {
					t.Fatalf("named inventory %+v", r)
				}
				if env["LANDING_PROOF_SCOPE"] == "" && !static {
					t.Fatal("static did not run first")
				}
				result := proofrun.NativeInventoryResult{Execution: []proofrun.PackageExecution{{Package: pkg, Shard: 1, Status: "ok"}, {Package: pkg, Shard: 2, Status: "ok"}, {Package: "github.com/widoriezebos/agentic-tools/metasystem/internal/launch", Shard: 1, Status: "ok"}}}
				if natives == 1 {
					switch mode {
					case "panic":
						result.Failed = true
						result.Execution[1].Status = "missing"
						result.Missing = []proofrun.NativeTestIdentity{{Classname: pkg, Name: "TestB", Status: "missing-terminal"}, {Classname: pkg, Name: "TestC", Status: "missing"}}
					case "TestMain":
						result.Failed = true
						result.Execution[1].Status = "fail"
						result.Observed = []proofrun.NativeTestIdentity{{Classname: pkg, Name: "TestA", Status: "passed"}}
					case "unexpected":
						result.Failed = true
						result.Execution[1].Status = "fail"
						result.Unexpected = []proofrun.NativeTestIdentity{{Classname: pkg, Name: "TestSurprise", Status: "passed"}}
					}
				}
				return result, nil
			}, Groups: func(ids []string) ([]proofrun.NamedGroupResult, error) {
				groups++
				if mode == "unknown group" {
					return nil, errors.New("unknown group")
				}
				if !reflect.DeepEqual(ids, []string{"verb-ratchet"}) {
					t.Fatalf("declared group became package or selection widened: %v", ids)
				}
				status := "green"
				if mode == "red group" {
					status = "red"
				}
				return []proofrun.NamedGroupResult{{ID: "verb-ratchet", Status: status, DurationMS: 7}}, nil
			}}
			command := func(argv []string, stdout, _ io.Writer) error {
				if reflect.DeepEqual(argv, []string{"go", "run", "./cmd/devgate", "static"}) {
					static = true
					return nil
				}
				if argv[0] != env["METASYSTEM_FULL_REPORTER"] {
					t.Fatalf("unexpected command %v", argv)
				}
				fmt.Fprintf(stdout, `{"data":{"groups":[{"id":%q,"status":"passed","nativeLaunched":true,"nativeExitStatus":0}]}}`, argv[2])
				return nil
			}
			exit := RunHost(&out, &stderr, func(key string) string { return env[key] }, command, hooks)
			want := 0
			if mode == "panic" || mode == "TestMain" || mode == "unexpected" || mode == "red group" || mode == "unknown group" {
				want = 1
			}
			if exit != want || !strings.HasPrefix(out.String(), "landing environment stable toolchain\n") {
				t.Fatalf("exit=%d out=%s err=%s", exit, &out, &stderr)
			}
			switch mode {
			case "panic":
				for _, line := range []string{"landing package metasystem/cmd/metasystem 1 ok", "landing package metasystem/cmd/metasystem 2 missing", "landing package metasystem/internal/launch 1 ok", "LANDING-FAILED\tmetasystem/cmd/metasystem\tTestB(did not report) TestC(did not report)\nLANDING-CHECKED\t1\n"} {
					if !strings.Contains(out.String(), line) {
						t.Fatalf("missing %q in %s", line, &out)
					}
				}
			case "TestMain":
				if !strings.HasSuffix(out.String(), "LANDING-FAILED\tmetasystem/cmd/metasystem\t\nLANDING-CHECKED\t1\n") {
					t.Fatal(out.String())
				}
			case "unexpected":
				if !strings.Contains(out.String(), "LANDING-FAILED\tmetasystem/cmd/metasystem\tTestSurprise") {
					t.Fatal(out.String())
				}
			case "scoped package":
				if natives != 1 || static || groups != 0 {
					t.Fatalf("calls native=%d static=%v groups=%d", natives, static, groups)
				}
			case "scoped named batch":
				if natives != 2 || static || groups != 0 {
					t.Fatalf("batch calls %d", natives)
				}
			case "scoped group", "group replay", "red group":
				if natives != 0 || static || groups != 1 || !strings.Contains(out.String(), "landing group verb-ratchet ") {
					t.Fatalf("native=%d static=%v groups=%d out=%s", natives, static, groups, &out)
				}
			case "unknown group":
				if natives != 0 || !strings.HasSuffix(out.String(), "LANDING-NOT-RUN\tenvironment\n") {
					t.Fatal(out.String())
				}
			}
			fmt.Print(out.String())
			fmt.Fprint(os.Stderr, stderr.String())
			os.Exit(exit)
		})
	}
}
