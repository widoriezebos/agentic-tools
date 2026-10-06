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

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestFullReportsPackageFailuresAndRunsOnlyRequestedPackage(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, native, only, suffix string
		failure                    error
		exit                       int
		full                       bool
	}{
		{"green", `{"Action":"pass","Package":"example/unit"}`, "", "LANDING-CHECKED\t0\n", nil, 0, true},
		{"red", "{\"Action\":\"fail\",\"Package\":\"example/unit\",\"Test\":\"TestBroken\"}\n{\"Action\":\"fail\",\"Package\":\"example/unit\"}", "example/unit", "LANDING-FAILED\texample/unit\tTestBroken\nLANDING-CHECKED\t1\n", errors.New("exit status 1"), 1, false},
		{"build failure", `{"Action":"fail","Package":"example/unit"}`, "", "LANDING-FAILED\texample/unit\t\nLANDING-CHECKED\t1\n", errors.New("exit status 1"), 1, true},
		{"repository path", "{\"Action\":\"fail\",\"Package\":\"github.com/widoriezebos/agentic-tools/metasystem/internal/config\",\"Test\":\"TestBroken\"}\n{\"Action\":\"fail\",\"Package\":\"github.com/widoriezebos/agentic-tools/metasystem/internal/config\"}", "metasystem/internal/config", "LANDING-FAILED\tmetasystem/internal/config\tTestBroken\nLANDING-CHECKED\t1\n", errors.New("exit status 1"), 1, false},
		{"lost process", "", "", "LANDING-NOT-RUN\tenvironment\n", errors.New("cannot start"), 1, false},
		{"empty green", "", "", "LANDING-NOT-RUN\tenvironment\n", nil, 1, false},
		{"malformed report", "broken", "", "LANDING-NOT-RUN\tenvironment\n", nil, 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var out, stderr bytes.Buffer
			calls := 0
			command := func(argv []string, stdout, _ io.Writer) error {
				calls++
				switch calls {
				case 1:
					if !reflect.DeepEqual(argv, []string{"metasystem", "landing", "status", "--json"}) {
						t.Fatalf("status: %v", argv)
					}
					fmt.Fprint(stdout, `{"data":{"root":"/lane"}}`)
				case 2:
					if !reflect.DeepEqual(argv, []string{"metasystem", "settings", "show", "host.proof-vm", "--repo", "/lane", "--json"}) {
						t.Fatalf("settings: %v", argv)
					}
					fmt.Fprint(stdout, `{"data":{"settings":[{"Key":"host.proof-vm","Value":""}]}}`)
				case 3:
					pkg := tc.only
					if pkg == "" {
						pkg = "./..."
					}
					if relative, ok := strings.CutPrefix(pkg, "metasystem/"); ok {
						pkg = "./" + relative
					}
					if !reflect.DeepEqual(argv, []string{"go", "test", "-json", "-count=1", "-timeout", "30m", pkg}) {
						t.Fatalf("suite: %v", argv)
					}
					fmt.Fprintln(stdout, tc.native)
					return tc.failure
				default:
					if argv[1] == "test" {
						fmt.Fprintln(stdout, `{"Action":"pass","Package":"batch/unit"}`)
					}
					if argv[0] == "proof/.full-reporter" {
						fmt.Fprintf(stdout, `{"data":{"groups":[{"id":%q,"status":"passed","nativeLaunched":true,"nativeExitStatus":0}]}}`, argv[2])
					}
				}
				return nil
			}
			exit := run(&out, &stderr, func(key string) string {
				if key == "LANDING_ONLY" {
					return tc.only
				}
				return ""
			}, command, "../../testing.json")
			expectedCalls := 3
			if tc.full {
				contract, err := testpolicy.Load("../../testing.json")
				if err != nil {
					t.Fatal(err)
				}
				expectedCalls += 2
				for _, group := range contract.Groups {
					if group.Adapter == "section" {
						expectedCalls++
					}
				}
			}
			if exit != tc.exit || calls != expectedCalls || !strings.HasSuffix(out.String(), tc.suffix) {
				t.Fatalf("exit=%d calls=%d output=%s stderr=%s", exit, calls, &out, &stderr)
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
				if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0755); err != nil {
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
			for _, want := range []string{"archive\n" + strings.Repeat("a", 40), "shell\nfixture-vm\n--\nbash\n-c", "/tmp/metasystem-proof/" + strings.Repeat("a", 40), "mkdir -p", "tar -x", "/metasystem", "proof/full.sh --host"} {
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
	var calls [][]string
	command := func(argv []string, stdout, _ io.Writer) error {
		calls = append(calls, argv)
		switch {
		case argv[0] == "metasystem" && argv[1] == "landing":
			fmt.Fprint(stdout, `{"data":{"root":"/lane"}}`)
		case argv[0] == "metasystem":
			fmt.Fprint(stdout, `{"data":{"settings":[{"Key":"host.proof-vm","Value":""}]}}`)
		case argv[1] == "test":
			fmt.Fprintln(stdout, `{"Action":"pass","Package":"fixture/package"}`)
		case argv[0] == "proof/.full-reporter":
			id := argv[2]
			status, exit := "passed", 0
			if id == "section/gate-fail-open-tripwire" {
				status, exit = "failed", 1
			}
			fmt.Fprintf(stdout, `{"data":{"groups":[{"id":%q,"status":%q,"nativeLaunched":true,"nativeExitStatus":%d}]}}`, id, status, exit)
			if exit != 0 {
				return errors.New("exit status 1")
			}
		}
		return nil
	}
	exit := run(&out, &stderr, func(string) string { return "" }, command, "../../testing.json")
	if exit != 1 || !strings.HasSuffix(out.String(), "LANDING-FAILED\tsection/gate-fail-open-tripwire\t\nLANDING-CHECKED\t1\n") {
		t.Fatalf("exit=%d output=%s stderr=%s calls=%v", exit, &out, &stderr, calls)
	}
	contract, err := testpolicy.Load("../../testing.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, group := range contract.Groups {
		if group.Adapter != "section" {
			continue
		}
		count := 0
		for _, call := range calls {
			if reflect.DeepEqual(call, []string{"proof/.full-reporter", "--section", group.ID}) {
				count++
			}
		}
		if count != 1 {
			t.Fatalf("section %s executed %d times", group.ID, count)
		}
	}
	for _, want := range [][]string{
		{"go", "test", "-json", "-count=1", "-timeout", "30m", "-tags", "batchtest", "./cmd/metasystem/", "./internal/landing/..."},
		{"go", "run", "./cmd/devgate", "static"},
	} {
		found := false
		for _, call := range calls {
			found = found || reflect.DeepEqual(call, want)
		}
		if !found {
			t.Fatalf("missing command: %v in %v", want, calls)
		}
	}
}

func TestFullVMEnvironmentFailuresCannotCertify(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, git, lima string }{
		{"missing limactl", "printf archive", ""},
		{"VM stopped", "printf archive", "exit 1"},
		{"archive failed after remote green", "exit 1", "cat >/dev/null; printf 'LANDING-CHECKED\\t0\\n'"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			for name, body := range map[string]string{"git": tc.git, "limactl": tc.lima} {
				if body == "" {
					continue
				}
				if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+body+"\n"), 0755); err != nil {
					t.Fatal(err)
				}
			}
			var out, stderr bytes.Buffer
			command := func(argv []string, stdout, stderr io.Writer) error {
				if argv[0] == "git" {
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
	for _, tc := range []struct{ name, fault, suffix string }{
		{"batch failure", "batch-red", "LANDING-FAILED\tgo-batchtest\tTestBatch\nLANDING-CHECKED\t1\n"},
		{"static failure", "static-red", "LANDING-FAILED\tfast-static-build\t\nLANDING-CHECKED\t1\n"},
		{"batch environment after failed packages", "batch-environment", "LANDING-NOT-RUN\tenvironment\n"},
		{"static environment", "static-environment", "LANDING-NOT-RUN\tenvironment\n"},
		{"section environment", "section-environment", "LANDING-NOT-RUN\tenvironment\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var out, stderr bytes.Buffer
			command := func(argv []string, stdout, _ io.Writer) error {
				switch {
				case argv[0] == "proof/.full-reporter":
					if tc.fault == "section-environment" {
						fmt.Fprintf(stdout, `{"data":{"groups":[{"id":%q,"status":"unavailable","nativeLaunched":false}]}}`, argv[2])
						return errors.New("not started")
					}
					fmt.Fprintf(stdout, `{"data":{"groups":[{"id":%q,"status":"passed","nativeLaunched":true,"nativeExitStatus":0}]}}`, argv[2])
				case argv[1] == "test":
					batch := len(argv) > 7
					if batch && tc.fault == "batch-environment" {
						return errors.New("batch process did not start")
					}
					if (batch && tc.fault == "batch-red") || (!batch && tc.fault == "batch-environment") {
						fmt.Fprintln(stdout, `{"Action":"fail","Package":"fixture/unit","Test":"TestBatch"}`)
						fmt.Fprintln(stdout, `{"Action":"fail","Package":"fixture/unit"}`)
						return errors.New("exit status 1")
					}
					fmt.Fprintln(stdout, `{"Action":"pass","Package":"fixture/unit"}`)
				case argv[1] == "run":
					if tc.fault == "static-environment" {
						return errors.New("static process did not start")
					}
					if tc.fault == "static-red" {
						return exec.Command("/bin/sh", "-c", "exit 1").Run()
					}
				default:
					t.Fatalf("unexpected command: %v", argv)
				}
				return nil
			}
			exit := runHost(&out, &stderr, func(string) string { return "" }, command, "../../testing.json")
			if exit != 1 || !strings.HasSuffix(out.String(), tc.suffix) {
				t.Fatalf("exit=%d output=%s stderr=%s", exit, &out, &stderr)
			}
		})
	}
}

func init() { runtime.LockOSThread() }
