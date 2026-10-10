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
	"slices"
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
	hooks.Groups = func(ids []string) ([]proofrun.NamedGroupResult, error) {
		if !reflect.DeepEqual(ids, []string{"fast-static-build"}) {
			t.Fatalf("static selection: %v", ids)
		}
		static = true
		return []proofrun.NamedGroupResult{{ID: "fast-static-build", Status: "green"}}, nil
	}
	sections := map[string]int{}
	command := func(argv []string, stdout, _ io.Writer) error {
		if argv[0] != "/fixture/run/reporter" {
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
	// Every section of the real contract is a cadence group; this contract
	// takes one of them out of the cadence so a landing proves it.
	contractPath := landingContractWithoutCadence(t, "section/gate-fail-open-tripwire")
	exit := runHost(&out, &stderr, func(key string) string {
		if key == "METASYSTEM_FULL_REPORTER" {
			return "/fixture/run/reporter"
		}
		return ""
	}, command, contractPath, hooks)
	if exit != 1 || batches != 1 || !strings.HasSuffix(out.String(), "LANDING-FAILED\tsection/gate-fail-open-tripwire\t\nLANDING-CHECKED\t1\n") {
		t.Fatalf("exit=%d batch=%d out=%s err=%s", exit, batches, &out, &stderr)
	}
	contract, err := testpolicy.Load(contractPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, group := range contract.Groups {
		if group.Adapter != "section" {
			continue
		}
		if slices.Contains(contract.Cadence, group.ID) {
			if sections[group.ID] != 0 || !strings.Contains(out.String(), "landing cadence "+group.ID+" deferred\n") {
				t.Fatalf("cadence section %s ran %d times or was not deferred: %s", group.ID, sections[group.ID], &out)
			}
			continue
		}
		if sections[group.ID] != 1 {
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
				if len(r.BuildTags) > 0 && fault == "batch-red" || len(r.BuildTags) == 0 && fault == "batch-environment" {
					status = "fail"
				}
				result := proofrun.NativeInventoryResult{Execution: []proofrun.PackageExecution{{Package: "fixture/unit", Shard: 1, Status: status}}}
				if status == "fail" {
					result.Observed = []proofrun.NativeTestIdentity{{Classname: "fixture/unit", Name: "TestBatch", Status: "failed"}}
				}
				return result, nil
			}}
			hooks.Groups = func(ids []string) ([]proofrun.NamedGroupResult, error) {
				if !reflect.DeepEqual(ids, []string{"fast-static-build"}) {
					t.Fatalf("static selection: %v", ids)
				}
				if fault == "static-environment" {
					return nil, errors.New("not started")
				}
				status := "green"
				if fault == "static-red" {
					status = "red"
				}
				return []proofrun.NamedGroupResult{{ID: "fast-static-build", Status: status}}, nil
			}
			command := func(argv []string, stdout, _ io.Writer) error {
				if fault == "section-environment" {
					fmt.Fprintf(stdout, `{"data":{"groups":[{"id":%q,"status":"unavailable","nativeLaunched":false}]}}`, argv[2])
					return errors.New("not started")
				}
				fmt.Fprintf(stdout, `{"data":{"groups":[{"id":%q,"status":"passed","nativeLaunched":true,"nativeExitStatus":0}]}}`, argv[2])
				return nil
			}
			suffix := "LANDING-NOT-RUN\tenvironment\n"
			if fault == "batch-red" {
				suffix = "LANDING-FAILED\tgo-batchtest\tTestBatch\nLANDING-CHECKED\t1\n"
			}
			if fault == "static-red" {
				suffix = "LANDING-FAILED\tfast-static-build\t\nLANDING-CHECKED\t1\n"
			}
			// The section faults need a section a landing runs: one taken out of the cadence.
			exit := runHost(&out, &stderr, func(string) string { return "" }, command, landingContractWithoutCadence(t, "section/go-engine-gate"), hooks)
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

// The shell entrypoint must give concurrent invocations distinct build outputs.
func TestFullScriptBuildsPrivateReporters(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "proof"), 0o755); err != nil {
		t.Fatal(err)
	}
	script, err := os.ReadFile("../../proof/full.sh")
	if err != nil {
		t.Fatal(err)
	}
	entry := filepath.Join(root, "proof", "full.sh")
	if err := testexec.WriteFile(entry, script, 0o755); err != nil {
		t.Fatal(err)
	}
	tools := t.TempDir()
	// The compiled stand-in reports both its own path and the path its sections inherit.
	goStub := `#!/bin/sh
[ "$1" = build ] && [ "$2" = -o ] && [ "$4" = ./proof ] || exit 64
cat >"$3" <<'REPORTER'
#!/bin/sh
printf '%s\n%s\n' "$0" "$METASYSTEM_FULL_REPORTER"
printf 'ready\n' >"$REPORTER_STARTED"
read -r release <&3
exit "${REPORTER_EXIT:-0}"
REPORTER
chmod +x "$3"
`
	if err := testexec.WriteFile(filepath.Join(tools, "go"), []byte(goStub), 0o755); err != nil {
		t.Fatal(err)
	}
	type result struct {
		output []byte
		err    error
	}
	results := make(chan result, 2)
	var releases []*os.File
	var startedPaths []string
	for run := range 2 {
		releaseRead, releaseWrite, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_ = releaseWrite.Close()
			_ = releaseRead.Close()
		})
		releases = append(releases, releaseWrite)
		started := filepath.Join(root, fmt.Sprintf("reporter-started-%d", run))
		startedPaths = append(startedPaths, started)
		go func() {
			cmd := exec.Command("/bin/sh", entry)
			cmd.Env = append(os.Environ(), "PATH="+tools+":/usr/bin:/bin", "TMPDIR="+root, "REPORTER_EXIT=7", "REPORTER_STARTED="+started)
			cmd.ExtraFiles = []*os.File{releaseRead}
			output, err := cmd.CombinedOutput()
			results <- result{output, err}
		}()
	}
	// Both reporters must be running before either can exit.
	testenv.Await(t, "both concurrent reporters to start", func() bool {
		select {
		case got := <-results:
			t.Fatalf("reporter exited before release: %v\n%s", got.err, got.output)
		default:
		}
		for _, path := range startedPaths {
			if _, err := os.Stat(path); err != nil {
				return false
			}
		}
		return true
	})
	for _, release := range releases {
		if _, err := release.WriteString("release\n"); err != nil {
			t.Fatal(err)
		}
		_ = release.Close()
	}
	paths := map[string]bool{}
	for range 2 {
		got := <-results
		var exit *exec.ExitError
		if !errors.As(got.err, &exit) || exit.ExitCode() != 7 {
			t.Fatalf("reporter exit was lost: %v\n%s", got.err, got.output)
		}
		lines := strings.Split(strings.TrimSpace(string(got.output)), "\n")
		if len(lines) != 2 || lines[0] != lines[1] || !strings.HasPrefix(lines[0], root+string(os.PathSeparator)) {
			t.Fatalf("the reporter and its sections do not share a private path: %q", got.output)
		}
		if paths[lines[0]] {
			t.Fatalf("concurrent runs built to the same path: %s", lines[0])
		}
		paths[lines[0]] = true
		if info, err := os.Stat(lines[0]); err != nil || !info.Mode().IsRegular() || info.Mode()&0o111 == 0 {
			t.Fatalf("private reporter must remain executable after exit: %v", err)
		}
	}
}

// landingContractWithoutCadence copies the repository contract with one group
// taken out of its cadence list, so a landing proves that section.
func landingContractWithoutCadence(t *testing.T, id string) string {
	t.Helper()
	data, err := os.ReadFile("../../testing.json")
	if err != nil {
		t.Fatal(err)
	}
	var contract map[string]any
	if err := json.Unmarshal(data, &contract); err != nil {
		t.Fatal(err)
	}
	var cadence []any
	for _, entry := range contract["cadence"].([]any) {
		if entry != id {
			cadence = append(cadence, entry)
		}
	}
	contract["cadence"] = cadence
	edited, err := json.Marshal(contract)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "testing.json")
	if err := os.WriteFile(path, edited, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
