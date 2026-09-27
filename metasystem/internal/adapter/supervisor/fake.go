package supervisor

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/adapter"
)

// The fake runtime: the deterministic protocol simulator the fixture beds
// run instead of a real CLI (formerly scripts/agents/adapters/fake.sh and
// scripts/agents/hosts/fake.sh). It implements the one runtime operation
// interface for both roles: its "CLI" is an in-process stand-in
// (Launch.Simulated) that acts out the FAKE:<behavior> markers of a delegate
// round (fake_round.go) or the FAKEHOST:<behavior> marker of a mission host
// turn (fake_host.go), and the shared delegate round and host turn do
// everything around it.

// fakeConfigIdentity is the fake's fixed configuration identity: it has no
// runtime configuration inputs, so fixtures can select snapshots without
// probing.
const fakeConfigIdentity = `{"cliVersion":"fake-1","configHash":"fake-config-v1","configKeyHashes":{},"runtime":"fake"}`

func fakeIdentity(Deps) (string, error) { return fakeConfigIdentity, nil }

// fakeOps is the fake runtime's operations.
type fakeOps struct{ builtinOps }

func init() {
	register(runtimeAdapter{
		name:           "fake",
		configIdentity: fakeIdentity,
		probe:          fakeProbe,
		contract:       fakeContract,
		outputStream: func(_ Deps, roundDir string) (string, error) {
			return roundDir + "/events.jsonl", nil
		},
		selftest:   fakeSelftest,
		probeUsage: "probe --root ROOT [--profile current|old|unverified-network] [--age-days N]",
		ops: fakeOps{builtinOps{name: "fake", usage: "native", host: true,
			configIdentity: fakeIdentity, probe: fakeProbe, contract: fakeContract, selftest: fakeSelftest}},
	})
}

// Prepare lays out a delegate round's or a host turn's simulated CLI.
func (fakeOps) Prepare(t *Turn) (Launch, error) {
	if t.Role == RoleHost {
		return prepareFakeHost(t)
	}
	return prepareFakeRound(t)
}

// Observe reports the delegate round's handshake once the simulated CLI
// reached it.
func (fakeOps) Observe(t *Turn, o Observation) (Events, error) {
	return o.Launch.Private.(*fakeRound).observe(t), nil
}

// Finalize answers the round's or the turn's outcome.
func (fakeOps) Finalize(t *Turn, in FinalInput) (Final, error) {
	if host, ok := in.Launch.Private.(*fakeHost); ok {
		return host.finalize(t)
	}
	return in.Launch.Private.(*fakeRound).finalize(in)
}

// fakeInterval validates one millisecond interval the way the script's
// fixture_milliseconds_to_sleep did.
func fakeInterval(d Deps, name string, fallback int) (time.Duration, error) {
	raw := d.Getenv(name)
	if raw == "" {
		raw = strconv.Itoa(fallback)
	}
	value, err := strconv.Atoi(raw)
	if !positiveIntegerRE.MatchString(raw) || err != nil {
		return 0, errors.New("fake adapter interval must be a positive integer in milliseconds")
	}
	return time.Duration(value) * time.Millisecond, nil
}

// shellSpace is the [[:space:]] class within a line.
const shellSpace = " \t\v\f\r"

// fakeArgument is the first `Fake-Argument:` line of the prompt, captured
// as data and never executed.
func fakeArgument(prompt string) string {
	data, err := os.ReadFile(prompt)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		if value, found := strings.CutPrefix(line, "Fake-Argument:"); found {
			return strings.TrimLeft(value, shellSpace)
		}
	}
	return ""
}

func appendText(path, text string) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.WriteString(file, text); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

// touchStrict is touch(1): create or bump the mtime, reporting failure.
func touchStrict(path string) error {
	now := time.Now()
	if err := os.Chtimes(path, now, now); err == nil {
		return nil
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	return file.Close()
}

// fakeRecordBuildCachePath mirrors job_build_cache_env for the fake's
// rounds, so they record the same cache path a real runtime's rounds would:
// only a job worktree under artifacts/agents/worktrees whose git dir is a
// linked worktree's, with the go-cache and go-tmp directories made. It
// differs from recordBuildCachePath in exactly the script's ways: no
// staticcheck directory, and a failed mkdir records an empty path.
func fakeRecordBuildCachePath(git GitQuery, agents, workspace, roundDir string) {
	cache := ""
	if jobsRoot, ok := realDir(filepath.Join(agents, "worktrees")); ok {
		ws, _ := realDir(workspace)
		if strings.HasPrefix(ws+"/", jobsRoot+"/") {
			if gitdir, ok := git(workspace, "rev-parse", "--absolute-git-dir"); ok && strings.Contains(gitdir, "/.git/worktrees/") {
				cache = filepath.Join(gitdir, "metasystem-build-cache", "go-cache")
				if os.MkdirAll(cache, 0o755) != nil || os.MkdirAll(filepath.Join(gitdir, "metasystem-build-cache", "go-tmp"), 0o755) != nil {
					cache = ""
				}
			}
		}
	}
	_ = os.WriteFile(filepath.Join(roundDir, "build-cache.txt"), []byte(cache+"\n"), 0o644)
}

// startFakeHold starts an `ENGINE util hold --tag TAG [flags]` child in this
// supervisor's process group; the janitor recognizes it by that argv.
func startFakeHold(d Deps, tag string, flags ...string) (*child, error) {
	command := exec.Command(d.Engine, append([]string{"util", "hold", "--tag", tag}, flags...)...)
	command.Env = d.Environ
	command.Stdout = d.Stdout
	command.Stderr = d.Stderr
	return startChild(command)
}

// stopHold is `kill -TERM PID; wait PID`.
func stopHold(hold *child) {
	_ = hold.command.Process.Signal(syscall.SIGTERM)
	hold.wait()
}

// fakeGuardedStatus is the retired guarded verbs' exit taxonomy: 0 allowed,
// 77 refused by the envelope, 1 an error.
func fakeGuardedStatus(d Deps, allowed bool, err error) int {
	switch {
	case err != nil:
		fmt.Fprintln(d.Stderr, err)
		return 1
	case allowed:
		return 0
	}
	return 77
}

func tempBase(d Deps) string {
	if dir := d.Getenv("TMPDIR"); dir != "" {
		return dir
	}
	return "/tmp"
}

// probeFakeEnvelopeMechanism proves the fake's envelope refuses a denied
// write and a denied network call before a snapshot declares them mapped.
func probeFakeEnvelopeMechanism(d Deps) error {
	dir, err := os.MkdirTemp(tempBase(d), "metasystem-fake-envelope-probe.*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	permissions := filepath.Join(dir, "permissions.json")
	target := filepath.Join(dir, "denied-write.txt")
	if err := os.WriteFile(permissions, []byte("{\"readRoots\":[],\"writeRoots\":[],\"network\":\"deny\"}\n"), 0o644); err != nil {
		return err
	}
	allowed, err := adapter.FakeGuardedWrite(permissions, target)
	writeStatus := fakeGuardedStatus(d, allowed, err)
	allowed, err = adapter.FakeGuardedNetwork(permissions, "127.0.0.1", "9")
	networkStatus := fakeGuardedStatus(d, allowed, err)
	_, statErr := os.Lstat(target)
	if writeStatus != 77 || networkStatus != 77 || statErr == nil {
		return errors.New("fake envelope mechanism did not refuse a denied write and network call")
	}
	if result := d.Getenv("METASYSTEM_FAKE_ENVELOPE_PROBE_RESULT"); result != "" {
		document := fmt.Sprintf("{\"network\":{\"exitStatus\":%d,\"observed\":\"denied\"},\"writeRoots\":{\"exitStatus\":%d,\"observed\":\"denied\"}}\n", networkStatus, writeStatus)
		if err := os.WriteFile(result, []byte(document), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// fakeHandshakeSeconds is the simulator's session-established window: two
// seconds scaled by the fixture cap scale like every other fixture ceiling
// (a fixed two-second default is a red gate on a busy machine): the suite's
// exported METASYSTEM_FIXTURE_CAP_SCALE_MILLI, else the calibration floor
// 8000 (main's fake.sh after U3 cut it loose from fixture-budget.sh).
func fakeHandshakeSeconds(d Deps) (int, error) {
	milli := int64(8000)
	if raw := d.Getenv("METASYSTEM_FIXTURE_CAP_SCALE_MILLI"); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if !positiveIntegerRE.MatchString(raw) || err != nil {
			return 0, errors.New("METASYSTEM_FIXTURE_CAP_SCALE_MILLI must be a positive integer")
		}
		milli = value
	}
	return int((2*milli + 999) / 1000), nil
}

var digitsRE = regexp.MustCompile(`^[0-9]+$`)

func fakeProbe(d Deps, args []string) int {
	// Fault hook for the snapshot self-heal fixtures: an unhealable probe.
	if d.Getenv("METASYSTEM_FAKE_PROBE_FAIL") != "" {
		fmt.Fprintln(d.Stderr, "scripted probe failure")
		return 1
	}
	profile, ageRaw := "current", "0"
	for len(args) > 0 {
		if len(args) < 2 {
			registry["fake"].usage(d)
			return 2
		}
		switch args[0] {
		case "--profile":
			profile = args[1]
		case "--age-days":
			ageRaw = args[1]
		default:
			registry["fake"].usage(d)
			return 2
		}
		args = args[2:]
	}
	switch profile {
	case "current", "old", "unverified-network":
	default:
		registry["fake"].usage(d)
		return 2
	}
	ageDays, err := strconv.Atoi(ageRaw)
	if !digitsRE.MatchString(ageRaw) || err != nil {
		registry["fake"].usage(d)
		return 2
	}
	if err := probeFakeEnvelopeMechanism(d); err != nil {
		fmt.Fprintln(d.Stderr, err)
		return 1
	}
	handshake, err := fakeHandshakeSeconds(d)
	if err != nil {
		fmt.Fprintln(d.Stderr, err)
		return 1
	}
	path, err := adapter.WriteFakeCapabilitySnapshot(filepath.Join(d.agents(), "capabilities"), profile, ageDays, handshake)
	if err != nil {
		fmt.Fprintln(d.Stderr, err)
		return 1
	}
	fmt.Fprintln(d.Stdout, path)
	return 0
}

// fakeContract rides the fake's real construction path — the profile-driven
// snapshot writer — with the deterministic current profile: no shared
// lifecycle helper, because the standalone shape is exactly what fake
// proves possible.
func fakeContract(d Deps) ([]byte, error) {
	dir, err := os.MkdirTemp(tempBase(d), "metasystem-contract.*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	path, err := adapter.WriteFakeCapabilitySnapshot(dir, "current", 0, 1)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}

// designBrief is the brief template with its Working Mode line set to
// design (the script's sed).
func designBrief(template string) ([]byte, error) {
	data, err := os.ReadFile(template)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(data), "\n")
	for index, line := range lines {
		if strings.HasPrefix(line, "Working Mode:") {
			lines[index] = "Working Mode: design"
		}
	}
	return []byte(strings.Join(lines, "\n")), nil
}

// engineDelegate runs `ENGINE internal delegate ARGS` with extra environment
// and returns its status.
func engineDelegate(d Deps, stdout io.Writer, env []string, args ...string) int {
	// The self-test's children run this binary: the delegate front door
	// admits --adapter-selftest only from a parent of the same executable.
	command := exec.Command(d.self(), append([]string{"internal", "delegate"}, args...)...)
	command.Env = withEnv(d.Environ, env...)
	command.Stdout = stdout
	command.Stderr = d.Stderr
	return exitStatus(command.Run())
}

// fakeSelftest drives the full protocol sequence through the engine's
// delegate front door — a design dispatch, its follow-up, and a held round
// cancelled — then records the pass.
func fakeSelftest(d Deps) int {
	quiet := d
	quiet.Stdout = discard{}
	if code := fakeProbe(quiet, nil); code != 0 {
		return code
	}
	dir, err := os.MkdirTemp(tempBase(d), "metasystem-fake-selftest.*")
	if err != nil {
		fmt.Fprintln(d.Stderr, err)
		return 1
	}
	id := fmt.Sprintf("fake-selftest-%s-%d", d.Clock.Now().UTC().Format("20060102t150405z"), d.Pid)
	templates := filepath.Join(d.Root, "scripts", "agents", "templates")
	rootEnv := "METASYSTEM_DELEGATE_ROOT=" + d.Root
	internalEnv := "METASYSTEM_DELEGATE_SELFTEST_INTERNAL=1"

	brief, err := designBrief(filepath.Join(templates, "brief.md"))
	if err == nil {
		err = os.WriteFile(filepath.Join(dir, "brief.md"), brief, 0o644)
	}
	if err != nil {
		fmt.Fprintln(d.Stderr, err)
		return 1
	}
	if code := engineDelegate(d, d.Stdout, []string{rootEnv, internalEnv}, "--adapter-selftest", "fake",
		"--brief", filepath.Join(dir, "brief.md"), "--workspace", d.Root, "--op", id, "--wait"); code != 0 {
		return code
	}
	follow, err := os.ReadFile(filepath.Join(templates, "follow-up.md"))
	if err == nil {
		err = os.WriteFile(filepath.Join(dir, "follow.md"), follow, 0o644)
	}
	if err != nil {
		fmt.Fprintln(d.Stderr, err)
		return 1
	}
	if code := engineDelegate(d, d.Stdout, []string{rootEnv}, "--follow-up", id,
		"--brief", filepath.Join(dir, "follow.md"), "--wait"); code != 0 {
		return code
	}
	cancel, err := designBrief(filepath.Join(templates, "brief.md"))
	if err == nil {
		err = os.WriteFile(filepath.Join(dir, "cancel.md"), append(cancel, []byte("\nFAKE:timeout\n")...), 0o644)
	}
	if err != nil {
		fmt.Fprintln(d.Stderr, err)
		return 1
	}
	if code := engineDelegate(d, discard{}, []string{rootEnv, internalEnv}, "--adapter-selftest", "fake",
		"--brief", filepath.Join(dir, "cancel.md"), "--workspace", d.Root, "--op", id+"-cancel"); code != 0 {
		return code
	}
	if code := engineDelegate(d, d.Stdout, []string{rootEnv}, "--cancel", id+"-cancel"); code != 0 {
		return code
	}
	selftests := filepath.Join(d.agents(), "selftests")
	if err := os.MkdirAll(selftests, 0o755); err != nil {
		fmt.Fprintln(d.Stderr, err)
		return 1
	}
	if err := adapter.WriteFakeSelftestRecord(filepath.Join(selftests, id+".json"), id); err != nil {
		fmt.Fprintln(d.Stderr, err)
		return 1
	}
	fmt.Fprintln(d.Stdout, "fake adapter selftest passed: full protocol sequence and denied-envelope mechanism probes")
	return 0
}
