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
	"runtime"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestFullReportsPackageFailuresAndRunsOnlyRequestedPackage(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, status, suffix string
		fault                bool
	}{
		{"green", "ok", "LANDING-CHECKED\t0\n", false},
		{"red", "fail", "LANDING-FAILED\tmetasystem/internal/config\tTestBroken\nLANDING-CHECKED\t1\n", false},
		{"missing package", "missing", "LANDING-FAILED\tmetasystem/internal/config\t\nLANDING-CHECKED\t1\n", false},
		{"lost process", "", "LANDING-NOT-RUN\tenvironment\n", true},
		{"empty green", "", "LANDING-NOT-RUN\tenvironment\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var out, stderr bytes.Buffer
			hooks := HostRunners{Environment: func() (string, error) { return "fixture", nil }, Native: func(r proofrun.NativeInventoryRequest) (proofrun.NativeInventoryResult, error) {
				if !reflect.DeepEqual(r.Packages, []string{"internal/config"}) || len(r.BuildTags) != 0 {
					t.Fatalf("selection: %+v", r)
				}
				if tc.fault {
					return proofrun.NativeInventoryResult{}, errors.New("cannot start")
				}
				result := proofrun.NativeInventoryResult{}
				if tc.status != "" {
					result.Execution = []proofrun.PackageExecution{{Package: "github.com/widoriezebos/agentic-tools/metasystem/internal/config", Shard: 1, Status: tc.status}}
				}
				if tc.status == "fail" {
					result.Observed = []proofrun.NativeTestIdentity{{Classname: result.Execution[0].Package, Name: "TestBroken", Status: "failed"}}
				}
				return result, nil
			}}
			calls := 0
			command := func(argv []string, stdout, stderr io.Writer) error {
				calls++
				if calls == 1 {
					fmt.Fprint(stdout, `{"data":{"root":"/lane"}}`)
				} else if calls == 2 {
					fmt.Fprint(stdout, `{"data":{"settings":[{"Key":"host.proof-vm","Value":""}]}}`)
				} else {
					t.Fatalf("unexpected command %v", argv)
				}
				return nil
			}
			exit := run(&out, &stderr, func(key string) string {
				if key == "LANDING_ONLY" {
					return "metasystem/internal/config"
				}
				return ""
			}, command, "../../testing.json", hooks)
			want := 1
			if tc.status == "ok" {
				want = 0
			}
			if exit != want || calls != 2 || !strings.HasPrefix(out.String(), "landing environment fixture\n") || !strings.HasSuffix(out.String(), tc.suffix) {
				t.Fatalf("exit=%d calls=%d out=%s err=%s", exit, calls, &out, &stderr)
			}
		})
	}
}

func TestFullVMTransfersCommitAndKeepsReportLast(t *testing.T) {
	t.Parallel()
	for _, commit := range []string{"", "candidate"} {
		t.Run("commit="+commit, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			log := filepath.Join(dir, "arguments")
			for name, body := range map[string]string{
				"git":     "#!/bin/sh\nprintf '%s\\n' \"$@\" >> \"$PROOF_ARGS\"\nprintf archive\n",
				"limactl": "#!/bin/sh\nprintf '%s\\n' \"$@\" >> \"$PROOF_ARGS\"\ncat >/dev/null\nprintf 'VM native output\\nLANDING-FAILED\\tfixture/vm\\tTestVM\\nLANDING-CHECKED\\t1\\n'\nexit 1\n",
			} {
				if err := testexec.WriteFile(filepath.Join(dir, name), []byte(body), 0755); err != nil {
					t.Fatal(err)
				}
			}
			var out, stderr bytes.Buffer
			command := func(argv []string, stdout, stderr io.Writer) error {
				switch argv[0] {
				case "metasystem":
					if argv[1] == "landing" {
						fmt.Fprint(stdout, `{"data":{"root":"/lane"}}`)
					} else {
						fmt.Fprint(stdout, `{"data":{"settings":[{"Key":"host.proof-vm","Value":"fixture-vm"}]}}`)
					}
					return nil
				case "git":
					if len(argv) > 2 && argv[1] == "-C" {
						f, err := os.OpenFile(log, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
						if err != nil {
							return err
						}
						defer f.Close()
						fmt.Fprintln(f, strings.Join(argv[1:], "\n"))
						if argv[3] == "bundle" {
							return os.WriteFile(argv[5], []byte("bundle"), 0644)
						}
						return nil
					}
					if reflect.DeepEqual(argv, []string{"git", "rev-parse", "--show-toplevel"}) {
						fmt.Fprint(stdout, "/fixture-root")
						return nil
					}
					want := commit
					if want == "" {
						want = "HEAD"
					}
					if !reflect.DeepEqual(argv, []string{"git", "rev-parse", "--verify", want + "^{commit}"}) {
						t.Fatalf("commit resolution: %v", argv)
					}
					fmt.Fprint(stdout, strings.Repeat("a", 40))
					return nil
				}
				c := exec.Command(argv[0], argv[1:]...)
				c.Env = append(os.Environ(), "PATH="+dir+":/usr/bin:/bin", "PROOF_ARGS="+log)
				c.Stdout, c.Stderr = stdout, stderr
				return c.Run()
			}
			exit := run(&out, &stderr, func(key string) string {
				if key == "LANDING_COMMIT" {
					return commit
				}
				return ""
			}, command, "../../testing.json")
			data, err := os.ReadFile(log)
			if exit != 1 || err != nil || out.String() != "VM native output\nLANDING-FAILED\tfixture/vm\tTestVM\nLANDING-CHECKED\t1\n" {
				t.Fatalf("exit=%d output=%s stderr=%s args=%s err=%v", exit, &out, &stderr, data, err)
			}
			for _, want := range []string{"-C\n/fixture-root\nbundle\ncreate", "shell\nfixture-vm\n--\nbash\n-c", "/tmp/metasystem-proof/" + strings.Repeat("a", 40), "mkdir -p", "git clone --no-checkout", "checkout --force --detach", "/metasystem", "proof/full.sh --host"} {
				if !strings.Contains(string(data), want) {
					t.Fatalf("missing %q in %s", want, data)
				}
			}
			if strings.Contains(string(data), "rm ") {
				t.Fatal("VM transfer must never remove its directory")
			}
		})
	}
}

func TestFullRunsBatchStaticAndSectionsBeforeReporting(t *testing.T) {
	t.Parallel()
	var out, stderr bytes.Buffer
	static := false
	batches := 0
	hooks := HostRunners{Environment: func() (string, error) { return "fixture", nil }, Native: func(r proofrun.NativeInventoryRequest) (proofrun.NativeInventoryResult, error) {
		if !static {
			t.Fatal("tests preceded static")
		}
		if len(r.BuildTags) > 0 {
			batches++
			if !reflect.DeepEqual(r.Packages, []string{"cmd/metasystem", "internal/landing/..."}) {
				t.Fatalf("batch %+v", r)
			}
		}
		return proofrun.NativeInventoryResult{Execution: []proofrun.PackageExecution{{Package: "fixture/package", Shard: 1, Status: "ok"}}}, nil
	}}
	sections := map[string]int{}
	command := func(argv []string, stdout, _ io.Writer) error {
		if reflect.DeepEqual(argv, []string{"go", "run", "./cmd/devgate", "static"}) {
			static = true
			return nil
		}
		if argv[0] != "proof/.full-reporter" {
			t.Fatalf("unexpected %v", argv)
		}
		id := argv[2]
		sections[id]++
		status, exit := "passed", 0
		if id == "section/gate-fail-open-tripwire" {
			status, exit = "failed", 1
		}
		fmt.Fprintf(stdout, `{"data":{"groups":[{"id":%q,"status":%q,"nativeLaunched":true,"nativeExitStatus":%d}]}}`, id, status, exit)
		if exit != 0 {
			return errors.New("exit status 1")
		}
		return nil
	}
	exit := runHost(&out, &stderr, func(string) string { return "" }, command, "../../testing.json", hooks)
	if exit != 1 || batches != 1 || !strings.HasSuffix(out.String(), "LANDING-FAILED\tsection/gate-fail-open-tripwire\t\nLANDING-CHECKED\t1\n") {
		t.Fatalf("exit=%d batch=%d out=%s err=%s", exit, batches, &out, &stderr)
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
}

func TestFullVMEnvironmentFailuresCannotCertify(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, git, lima string }{
		{"missing limactl", "printf archive", ""},
		{"VM stopped", "printf archive", "exit 1"},
		{"bundle failed after remote green", "exit 1", "cat >/dev/null; printf 'LANDING-CHECKED\\t0\\n'"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			for name, body := range map[string]string{"git": tc.git, "limactl": tc.lima} {
				if body == "" {
					continue
				}
				if err := testexec.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+body+"\n"), 0755); err != nil {
					t.Fatal(err)
				}
			}
			var out, stderr bytes.Buffer
			command := func(argv []string, stdout, stderr io.Writer) error {
				if argv[0] == "git" {
					if len(argv) > 2 && argv[1] == "-C" {
						if argv[3] == "bundle" {
							if tc.name == "bundle failed after remote green" {
								return errors.New("bundle failed")
							}
							return os.WriteFile(argv[5], []byte("bundle"), 0644)
						}
						return nil
					}
					fmt.Fprint(stdout, strings.Repeat("a", 40))
					return nil
				}
				c := exec.Command("/bin/bash", argv[1:]...)
				c.Env = append(os.Environ(), "PATH="+dir+":/usr/bin:/bin")
				c.Stdout, c.Stderr = stdout, stderr
				return c.Run()
			}
			exit := runVM(&out, &stderr, func(string) string { return "" }, command, "fixture-vm")
			if exit != 1 || !strings.HasSuffix(out.String(), "LANDING-NOT-RUN\tenvironment\n") {
				t.Fatalf("exit=%d output=%s stderr=%s", exit, &out, &stderr)
			}
		})
	}
}

func TestRepositorySectionRunnerExecutesArchiveWithoutGit(t *testing.T) {
	t.Parallel()
	module := filepath.Join(t.TempDir(), "metasystem")
	if err := os.MkdirAll(module, 0755); err != nil {
		t.Fatal(err)
	}
	contract, err := testpolicy.Load("../../testing.json")
	if err != nil {
		t.Fatal(err)
	}
	id := "section/gate-fail-open-tripwire"
	for i, group := range contract.Groups {
		if group.ID != id {
			continue
		}
		group.Argv = []string{"/bin/bash", "bed.sh"}
		group.Inputs = []string{"metasystem/bed.sh"}
		group.Tools = []testpolicy.Tool{{ID: "bash", Executable: "/bin/bash", VersionArgs: []string{"--version"}}}
		contract.Groups[i] = group
	}
	data, err := json.Marshal(contract)
	if err != nil {
		t.Fatal(err)
	}
	for file, data := range map[string][]byte{
		"testing.json": data,
		"bed.sh":       []byte("printf 'native section failed\\n'; printf ran > section-ran; exit 1\n"),
	} {
		if err := os.WriteFile(filepath.Join(module, file), data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	var out, stderr bytes.Buffer
	exit := runSection(&out, &stderr, module, id, []string{"PATH=" + t.TempDir(), "HOME=" + t.TempDir()})
	marker, err := os.ReadFile(filepath.Join(module, "section-ran"))
	if exit != 1 || err != nil || string(marker) != "ran" || !strings.Contains(out.String(), `"status":"failed"`) || !strings.Contains(out.String(), `"nativeExitStatus":1`) || !strings.Contains(stderr.String(), "native section failed") {
		t.Fatalf("exit=%d output=%s stderr=%s marker=%s error=%v", exit, &out, &stderr, marker, err)
	}
}

func TestMain(m *testing.M) {
	if exit, handled := Custodian(os.Args[1:], os.Stderr); handled {
		os.Exit(exit)
	}
	os.Exit(testenv.Main(m))
}

func TestFullLegFailuresKeepTheirGroupAndEnvironmentVerdicts(t *testing.T) {
	t.Parallel()
	for _, fault := range []string{"batch-red", "batch-environment", "static-red", "static-environment", "section-environment", "environment"} {
		t.Run(fault, func(t *testing.T) {
			t.Parallel()
			var out, stderr bytes.Buffer
			hooks := HostRunners{Environment: func() (string, error) {
				if fault == "environment" {
					return "", errors.New("no toolchain")
				}
				return "fixture", nil
			}, Native: func(r proofrun.NativeInventoryRequest) (proofrun.NativeInventoryResult, error) {
				if len(r.BuildTags) > 0 && fault == "batch-environment" {
					return proofrun.NativeInventoryResult{}, errors.New("not started")
				}
				status := "ok"
				if len(r.BuildTags) > 0 && fault == "batch-red" {
					status = "fail"
				}
				return proofrun.NativeInventoryResult{Execution: []proofrun.PackageExecution{{Package: "fixture/unit", Shard: 1, Status: status}}}, nil
			}}
			command := func(argv []string, stdout, _ io.Writer) error {
				if argv[0] == "go" {
					if fault == "static-environment" {
						return errors.New("not started")
					}
					if fault == "static-red" {
						return exec.Command("/bin/sh", "-c", "exit 1").Run()
					}
					return nil
				}
				if fault == "section-environment" {
					fmt.Fprintf(stdout, `{"data":{"groups":[{"id":%q,"status":"unavailable","nativeLaunched":false}]}}`, argv[2])
					return errors.New("not started")
				}
				fmt.Fprintf(stdout, `{"data":{"groups":[{"id":%q,"status":"passed","nativeLaunched":true,"nativeExitStatus":0}]}}`, argv[2])
				return nil
			}
			suffix := "LANDING-NOT-RUN\tenvironment\n"
			if fault == "batch-red" {
				suffix = "LANDING-FAILED\tgo-batchtest\t\nLANDING-CHECKED\t1\n"
			}
			if fault == "static-red" {
				suffix = "LANDING-FAILED\tfast-static-build\t\nLANDING-CHECKED\t1\n"
			}
			exit := runHost(&out, &stderr, func(string) string { return "" }, command, "../../testing.json", hooks)
			if exit != 1 || !strings.HasSuffix(out.String(), suffix) {
				t.Fatalf("exit=%d out=%s err=%s", exit, &out, &stderr)
			}
		})
	}
}

func init() { runtime.LockOSThread() }

func TestFullReplaySelectionsKeepGroupsAndPackagesSeparate(t *testing.T) {
	t.Parallel()
	for _, unknown := range []bool{false, true} {
		t.Run(fmt.Sprintf("unknown=%t", unknown), func(t *testing.T) {
			t.Parallel()
			var out, problem bytes.Buffer
			selections := "fast-static-build go-batchtest section/gate-fail-open-tripwire metasystem/internal/config=TestA,TestB metasystem/internal/lease"
			groups := []string{"fast-static-build", "go-batchtest", "section/gate-fail-open-tripwire"}
			packages := []string{}
			hooks := HostRunners{Environment: func() (string, error) { return "fixture", nil }, Groups: func(ids []string) ([]proofrun.NamedGroupResult, error) {
				if !reflect.DeepEqual(ids, groups) {
					t.Fatalf("group selections=%v", ids)
				}
				results := []proofrun.NamedGroupResult{}
				for _, id := range ids {
					results = append(results, proofrun.NamedGroupResult{ID: id, Status: "green"})
				}
				return results, nil
			}, Native: func(request proofrun.NativeInventoryRequest) (proofrun.NativeInventoryResult, error) {
				if len(request.Packages) != 1 {
					t.Fatalf("packages=%v", request.Packages)
				}
				pkg := request.Packages[0]
				packages = append(packages, pkg)
				if pkg == "internal/config" && !reflect.DeepEqual(request.Tests, []string{"TestA", "TestB"}) || pkg == "internal/lease" && len(request.Tests) != 0 || !strings.HasPrefix(pkg, "internal/") {
					t.Fatalf("native selection=%+v", request)
				}
				return proofrun.NativeInventoryResult{Execution: []proofrun.PackageExecution{{Package: "github.com/widoriezebos/agentic-tools/metasystem/" + pkg, Status: "ok", Shard: 1}}}, nil
			}}
			if unknown {
				selections = "no-such-thing"
				hooks.Native = nil
			}
			code := runHost(&out, &problem, func(key string) string {
				if key == "LANDING_ONLY" {
					return selections
				}
				return ""
			}, nil, "../../testing.json", hooks)
			if unknown {
				if code != 1 || !strings.HasSuffix(out.String(), "LANDING-NOT-RUN\tenvironment\n") {
					t.Fatalf("unknown selection exit=%d out=%s error=%s", code, &out, &problem)
				}
				return
			}
			if code != 0 || !reflect.DeepEqual(packages, []string{"internal/config", "internal/lease"}) || !strings.HasSuffix(out.String(), "LANDING-CHECKED\t0\n") {
				t.Fatalf("mixed selection exit=%d packages=%v out=%s error=%s", code, packages, &out, &problem)
			}
		})
	}
}
