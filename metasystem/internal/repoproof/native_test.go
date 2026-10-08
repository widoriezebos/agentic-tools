package repoproof

import (
	"bytes"
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
)

func TestFullShardsRunEveryPackageAndAPanicLosesOneShard(t *testing.T) {
	t.Parallel()
	if os.Getenv("FULL_SHARD_BUILD") == "1" {
		body := "#!/bin/sh\nexec \"$FULL_SHARD_TEST_BINARY\" -test.run '^TestFullShardsRunEveryPackageAndAPanicLosesOneShard$' -- \"$@\"\n"
		if err := testexec.WriteFile("proof/.full-reporter", []byte(body), 0755); err != nil {
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
				if argv[0] != "proof/.full-reporter" {
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
