package proofrun

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostload"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

func TestMain(m *testing.M) {
	if os.Getenv("GO_WANT_GO_SHARD_HELPER") == "1" {
		terminated := shardFixtureTermination()
		if os.Getenv("GO_SHARD_DESCENDANT") == "1" {
			<-terminated
			os.Exit(0)
		}
		descendant := exec.Command(os.Args[0])
		descendant.Env = append(os.Environ(), "GO_SHARD_DESCENDANT=1")
		if err := descendant.Start(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(96)
		}
		stateRoot := os.Getenv("ACTIVE_SHARD_STATE_DIR")
		pid := os.Getpid()
		temporary := filepath.Join(stateRoot, fmt.Sprintf(".%d.tmp", pid))
		ready := filepath.Join(stateRoot, fmt.Sprintf("%d.ready", pid))
		state := []byte(fmt.Sprintf("child=%d\ndescendant=%d\n", pid, descendant.Process.Pid))
		if err := os.WriteFile(temporary, state, 0o600); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(97)
		}
		if err := os.Rename(temporary, ready); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(97)
		}
		fmt.Fprintln(os.Stderr, os.Getenv("ACTIVE_SHARD_DIAGNOSTIC"))
		<-terminated
		os.Exit(0)
	}
	if custodyExecWitnessEntrypoint() {
		os.Exit(0)
	}
	if code, handled := resourceCustodyTestEntrypoint(); handled {
		os.Exit(code)
	}
	installDeterministicTestLoadReaders()
	if err := os.Unsetenv(TestHostLoadEnvironment); err != nil {
		panic(err)
	}
	declarations := []testenv.Declaration{}
	helperProcess := proofrunSubprocessHelper() || os.Getenv("METASYSTEM_PROOFRUN_TEST_CANDIDATE_ENGINE") == "1"
	if helperProcess {
		declarations = testenv.DeclareInheritedControls()
	}
	if err := os.Unsetenv("METASYSTEM_PROOFRUN_TEST_CANDIDATE_ENGINE"); err != nil {
		panic(err)
	}
	// This exact real-process custody helper is owned by its parent test,
	// which holds exact refs and a bounded cleanup. Giving the helper a second
	// testenv fixture custodian would kill the product custodian when the
	// launcher is deliberately killed, obscuring the behavior under test.
	if hostResourceCustodyHelperInvocation() || hostResourceNestedCustodyHelperInvocation() {
		os.Exit(m.Run())
	}
	// Independent test binaries have independent proof roots. Give their host
	// admission guard the same isolation; tests exercising real contention
	// explicitly replace this directory with their shared fixture directory.
	admissionRoot := ""
	if !helperProcess && os.Getenv(identity.FixtureCustodianEnv) != "1" {
		var err error
		admissionRoot, err = os.MkdirTemp("", "metasystem-proofrun-admission.")
		if err != nil {
			panic(err)
		}
		hostAdmissionDirectoryForTest = filepath.Join(admissionRoot, "host-admission")
	}
	code := testenv.Main(m, declarations...)
	if admissionRoot != "" {
		if err := os.RemoveAll(admissionRoot); err != nil {
			fmt.Fprintln(os.Stderr, "remove test host admission directory:", err)
		}
	}
	os.Exit(code)
}

func shardFixtureTermination() <-chan os.Signal {
	terminated := make(chan os.Signal, 1)
	signal.Notify(terminated, syscall.SIGTERM, os.Interrupt)
	return terminated
}

const custodyExecWitnessEnvironment = "METASYSTEM_CUSTODY_EXEC_WITNESS"

type custodyExecWitness struct {
	Args      []string `json:"args"`
	Directory string   `json:"directory"`
	Marker    string   `json:"marker"`
}

func custodyExecWitnessEntrypoint() bool {
	destination := os.Getenv(custodyExecWitnessEnvironment)
	if destination == "" {
		return false
	}
	directory, err := os.Getwd()
	if err == nil {
		data, marshalErr := json.Marshal(custodyExecWitness{Args: os.Args, Directory: directory, Marker: os.Getenv("CUSTODY_EXEC_MARKER")})
		err = marshalErr
		if err == nil {
			err = os.WriteFile(destination, data, 0o600)
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "custody exec witness:", err)
		os.Exit(91)
	}
	os.Exit(0)
	return true
}

func resourceCustodyTestEntrypoint() (int, bool) {
	if len(os.Args) < 3 || os.Args[1] != "proof-run" {
		return 0, false
	}
	switch os.Args[2] {
	case "watchdog":
		flags := flag.NewFlagSet("proof-run watchdog", flag.ContinueOnError)
		resourceCustody := flags.Bool("resource-custody", false, "")
		parentPID := flags.Int64("custody-parent-pid", 0, "")
		parentStarted := flags.Int64("custody-parent-started-at", 0, "")
		parentMicro := flags.Int64("custody-parent-start-micro", 0, "")
		parentTicks := flags.Int64("custody-parent-start-ticks", 0, "")
		parentBoot := flags.String("custody-parent-boot-id", "", "")
		controlFD := flags.Int("custody-control-fd", 0, "")
		readyFD := flags.Int("custody-ready-fd", 0, "")
		markerFD := flags.Int("custody-marker-fd", 0, "")
		markerPath := flags.String("custody-marker-path", "", "")
		if flags.Parse(os.Args[3:]) != nil || flags.NArg() != 0 || !*resourceCustody {
			return 2, true
		}
		err := RunResourceCustodian(ResourceCustodyOptions{Launcher: identity.Ref{Pid: *parentPID,
			StartedAtSec: *parentStarted, StartedAtUnixMicro: *parentMicro, StartTicks: *parentTicks, BootID: *parentBoot},
			ControlFD: *controlFD, ReadyFD: *readyFD, MarkerFD: *markerFD, MarkerPath: *markerPath})
		if err != nil {
			fmt.Fprintln(os.Stderr, "test resource custodian:", err)
			return 1, true
		}
		return 0, true
	case "custody-exec":
		flags := flag.NewFlagSet("proof-run custody-exec", flag.ContinueOnError)
		readyFD := flags.Int("ready-fd", 0, "")
		releaseFD := flags.Int("release-fd", 0, "")
		path := flags.String("path", "", "")
		if flags.Parse(os.Args[3:]) != nil || flags.NArg() < 1 || *readyFD < 3 || *releaseFD < 3 || *path == "" {
			return 2, true
		}
		ready := os.NewFile(uintptr(*readyFD), "custody-exec-ready")
		release := os.NewFile(uintptr(*releaseFD), "custody-exec-release")
		if _, err := ready.Write([]byte("ready\n")); err != nil {
			return 1, true
		}
		_ = ready.Close()
		var token [1]byte
		if count, err := release.Read(token[:]); err != nil || count != 1 || token[0] != 1 {
			return 1, true
		}
		_ = release.Close()
		if err := syscall.Exec(*path, flags.Args(), os.Environ()); err != nil {
			return 1, true
		}
		return 0, true
	default:
		return 0, false
	}
}

func hostResourceCustodyHelperInvocation() bool {
	return hostResourceCustodyHelperArgs(os.Args, os.Getenv("METASYSTEM_HOST_CUSTODY_HELPER"))
}

func hostResourceCustodyHelperArgs(args []string, helperMode string) bool {
	return hostResourceCustodyHelperArgsUnder(args, helperMode, os.TempDir())
}

func hostResourceCustodyHelperArgsUnder(args []string, helperMode, temporaryRoot string) bool {
	if helperMode != "1" || len(args) != 11 ||
		args[1] != "-test.run=^TestGLEHostResourceCustodyProcessHelper$" || args[2] != "--" || args[3] != "launcher" {
		return false
	}
	root := args[4]
	relative, err := filepath.Rel(temporaryRoot, root)
	return err == nil && filepath.IsAbs(root) && relative != "." && relative != ".." &&
		!strings.HasPrefix(relative, ".."+string(filepath.Separator)) && fixtureauth.FixtureModeRoot(root)
}

func TestHostResourceCustodyHelperInvocation(t *testing.T) {
	t.Parallel()
	newRoot := func(parent, config string) string {
		root, err := os.MkdirTemp(parent, "custody-helper.")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.RemoveAll(root) })
		if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte(config), 0o600); err != nil {
			t.Fatal(err)
		}
		return root
	}
	layoutRoot := t.TempDir()
	temporaryRoot, ordinaryParent := filepath.Join(layoutRoot, "controlled-temp"), filepath.Join(layoutRoot, "checkout")
	for _, parent := range []string{temporaryRoot, ordinaryParent} {
		if err := os.MkdirAll(parent, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	fixtureRoot, nonFixtureRoot := newRoot(temporaryRoot, "metasystem.runtimes=fake\n"), newRoot(temporaryRoot, "")
	nonTemporaryRoot := newRoot(ordinaryParent, "metasystem.runtimes=fake\n")
	exact := []string{"proofrun.test", "-test.run=^TestGLEHostResourceCustodyProcessHelper$", "--", "launcher", fixtureRoot, "conf", "engine", "worker", "grandchild", "ready", "release"}
	invoked := func(args []string, mode string) bool {
		return hostResourceCustodyHelperArgsUnder(args, mode, temporaryRoot)
	}
	if !invoked(exact, "1") || !hostResourceCustodyHelperArgs(exact, "1") {
		t.Fatal("exact helper invocation was refused")
	}
	cases := []struct {
		name string
		edit func([]string) []string
	}{
		{"old arity", func(args []string) []string { return args[:10] }},
		{"extra argument", func(args []string) []string { return append(args, "extra") }},
		{"wrong test name", func(args []string) []string { args[1] = "-test.run=wrong"; return args }},
		{"wrong mode", func(args []string) []string { args[3] = "worker"; return args }},
		{"non-temporary root", func(args []string) []string { args[4] = nonTemporaryRoot; return args }},
		{"non-fixture root", func(args []string) []string { args[4] = nonFixtureRoot; return args }},
	}
	for _, test := range cases {
		args := append([]string(nil), exact...)
		if invoked(test.edit(args), "1") {
			t.Errorf("%s: malformed helper invocation was accepted", test.name)
		}
	}
}

func hostResourceNestedCustodyHelperInvocation() bool {
	args := os.Args
	if os.Getenv("METASYSTEM_NESTED_CUSTODY_HELPER") != "1" || len(args) != 9 ||
		args[1] != "-test.run=^TestHostResourceNestedCustodySubprocess$" || args[2] != "--" ||
		(args[3] != "launcher" && args[3] != "worker") {
		return false
	}
	root := args[4]
	relative, err := filepath.Rel(os.TempDir(), root)
	return err == nil && filepath.IsAbs(root) && relative != "." && relative != ".." &&
		!strings.HasPrefix(relative, ".."+string(filepath.Separator)) && fixtureauth.FixtureModeRoot(root)
}

type deadTestProber struct{}

func (deadTestProber) Probe(int64) (identity.Exact, identity.Liveness, error) {
	return identity.Exact{}, identity.Dead, nil
}

func installDeterministicTestLoadReaders() {
	loadSeams = loadReaders{
		host: func(now time.Time) hostload.Sample {
			return hostload.Sample{At: now.UTC().Format(time.RFC3339Nano), Available: true, Cores: 18}
		},
		launchers: func(int64) (int, bool) { return 0, true },
		nested:    func(int64) (bool, bool) { return false, true },
		prober:    deadTestProber{},
		pids:      func() ([]int64, error) { return nil, nil },
		parent:    func(int64) (int64, bool) { return 0, false },
	}
}

// useRealLoadReaders is the only opt-in from package tests to the machine's
// load and process census. Tests that do not call it stay host-independent.
func useRealLoadReaders(t *testing.T) {
	t.Helper()
	previous := loadSeams
	loadSeams = realLoadReaders()
	t.Cleanup(func() { loadSeams = previous })
}

func proofrunSubprocessHelper() bool {
	for _, name := range []string{
		"GO_WANT_COVERAGE_SCRIPT_HELPER",
		"GO_WANT_LEGACY_PROOF_WORKER",
		"GO_WANT_PROOF_ENTRYPOINT_HELPER",
		"GO_WANT_SUPERVISOR_HELPER",
		"GO_WANT_VERSION_IDENTITY_HELPER",
		"GO_WANT_WITNESS_GATE_HELPER",
	} {
		if os.Getenv(name) != "" {
			return true
		}
	}
	return false
}
