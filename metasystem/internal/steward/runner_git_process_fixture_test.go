package steward

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

const (
	processGitHelperArg = "-steward-private-git-helper"
	processGitConfigEnv = "STEWARD_PRIVATE_GIT_CONFIG"
	processGitHead      = "1f16522e7087346edabaa45f5079b9369ca282f3"
)

type processGitReply struct {
	Cwd    string   `json:"cwd"`
	Args   []string `json:"args"`
	Stdout []byte   `json:"stdout"`
	Stderr []byte   `json:"stderr"`
	Exit   int      `json:"exit"`
}

type processGitConfig struct {
	Replies    []processGitReply `json:"replies"`
	Unexpected string            `json:"unexpected"`
	Events     string            `json:"events"`
}

func newProcessGitFixture(t *testing.T) string {
	t.Helper()
	root, environment := newProcessGitFixtureEnvironment(t)
	for _, entry := range environment {
		key, value, _ := strings.Cut(entry, "=")
		t.Setenv(key, value)
	}
	return root
}

func newProcessGitFixtureEnvironment(t *testing.T) (string, []string) {
	t.Helper()
	rawRoot := t.TempDir()
	root := canonicalPath(rawRoot)
	logDir := os.Getenv("STEWARD_PROCESS_GIT_PROOF_DIR")
	if logDir == "" {
		logDir = root
	}
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatal(err)
	}
	stem := filepath.Join(logDir, t.Name())
	denied := stem + "-denied.log"
	if err := os.WriteFile(denied, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	denyDir := filepath.Join(root, "git-deny")
	if err := os.Mkdir(denyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }
	deny := "#!/bin/sh\nprintf 'denied git: %s\\n' \"$*\" >> " + quote(denied) + "\nexit 98\n"
	if err := testexec.WriteFile(filepath.Join(denyDir, "git"), []byte(deny), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "plans"), 0o755); err != nil {
		t.Fatal(err)
	}
	ledger := "# Goals\n\n## Current goal: fix-it — Repair the thing\n- Origin: main\n- Next step: Repair it.\n"
	if err := os.WriteFile(filepath.Join(root, "plans", "goals.md"), []byte(ledger), 0o644); err != nil {
		t.Fatal(err)
	}
	bin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	config := processGitConfig{Unexpected: stem + "-unexpected.log", Events: stem + "-events.log"}
	for _, path := range []string{config.Unexpected, config.Events} {
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	add := func(at string, args []string, output string, exit int) {
		config.Replies = append(config.Replies, processGitReply{Cwd: at, Args: args, Stdout: []byte(output), Exit: exit})
	}
	withRoot := func(args ...string) []string { return append([]string{"-C", root}, args...) }
	add(cwd, withRoot("rev-parse", "--git-common-dir"), ".git\n", 0)
	add(cwd, withRoot("rev-parse", "--git-dir"), ".git\n", 0)
	add(cwd, withRoot("config", "--get", "metasystem.steward.notify-command"), "true\n", 0)
	add(cwd, withRoot("config", "--get", "metasystem.steward.tick-seconds"), "", 1)
	add(cwd, withRoot("rev-parse", "--show-toplevel"), root+"\n", 0)
	add(cwd, withRoot("rev-parse", "HEAD"), processGitHead+"\n", 0)
	add(cwd, withRoot("rev-parse", "--verify", "--quiet", "refs/metasystem/goals/accepted"), "", 1)
	// Seat presence rides every resident tick: this fixture repository has no
	// origin, so the presence fetch fails as Git would and the copy is empty.
	config.Replies = append(config.Replies, processGitReply{Cwd: cwd, Args: withRoot("-c", "core.logAllRefUpdates=false", "fetch", "--no-tags", "--refmap=", "--atomic", "--prune", "origin",
		"+refs/metasystem/presence/*:refs/metasystem/presence-copy/metasystem/*", "+refs/heads/presence/*:refs/metasystem/presence-copy/heads/*"),
		Stderr: []byte("fatal: 'origin' does not appear to be a git repository\n"), Exit: 128})
	add(cwd, withRoot("-c", "core.logAllRefUpdates=false", "for-each-ref", "--format=%(refname)", "refs/metasystem/presence-copy/metasystem"), "", 0)
	add(cwd, withRoot("-c", "core.logAllRefUpdates=false", "for-each-ref", "--format=%(refname)", "refs/metasystem/presence-copy/heads"), "", 0)
	for _, key := range []string{"goal.sync-remote", "goal.sync-branch", "metasystem.goal.machine"} {
		add(root, []string{"config", "--get", key}, "", 1)
	}
	for _, args := range [][]string{
		{"rev-parse", "--verify", "--quiet", "refs/metasystem/goals/accepted"},
		{"rev-parse", "--verify", "--quiet", "refs/metasystem/goals/accepted^{commit}"},
		{"show-ref", "--verify", "--quiet", "refs/metasystem/goals/accepted"},
	} {
		add(root, args, "", 1)
	}
	add(root, []string{"rev-parse", "--path-format=absolute", "--git-common-dir"}, filepath.Join(root, ".git")+"\n", 0)
	add(root, []string{"ls-tree", "-r", "--name-only", "HEAD", "--", "plans/goals/", "records/goals/"}, "", 0)
	pins := []string{"-c", "core.fileMode=true", "-c", "diff.noprefix=false", "-c", "diff.mnemonicPrefix=false", "-c", "apply.ignoreWhitespace=no", "-c", "core.logAllRefUpdates=false", "-c", "core.useReplaceRefs=false", "-c", "gc.auto=0", "-c", "maintenance.auto=false"}
	for _, reply := range []struct {
		args   []string
		output string
	}{
		{[]string{"rev-parse", "--show-toplevel"}, root + "\n"},
		{[]string{"rev-parse", "--show-prefix"}, "\n"},
		{[]string{"rev-parse", "--verify", "--quiet", "HEAD^{commit}"}, processGitHead + "\n"},
	} {
		add(cwd, append(append(withRoot(), pins...), reply.args...), reply.output, 0)
	}
	logArgs := []string{"-c", "core.useReplaceRefs=false", "-c", "gc.auto=0", "-c", "maintenance.auto=false", "log", "--topo-order", "--reverse", "--format=%x1e%H%x1f%cI%x1f%(trailers:key=Goal-Transaction,valueonly,separator=%x1d)", "--numstat", "--no-renames", processGitHead, "--"}
	add(cwd, withRoot(logArgs...), "\x1e"+processGitHead+"\x1f2026-09-24T08:41:15+02:00\x1f\n\n5\t0\tplans/goals.md\n", 0)
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(root, "git-replies.json")
	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	shimDir := filepath.Join(root, "git-shim")
	if err := os.Mkdir(shimDir, 0o755); err != nil {
		t.Fatal(err)
	}
	shim := "#!/bin/sh\nGORACE=atexit_sleep_ms=0 exec " + quote(bin) + " " + processGitHelperArg + " \"$@\"\n"
	if err := testexec.WriteFile(filepath.Join(shimDir, "git"), []byte(shim), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Logf("Git fixture root raw=%q canonical=%q", rawRoot, root)
	t.Cleanup(func() { checkProcessGitFixture(t, config, denied) })
	return root, []string{processGitConfigEnv + "=" + configPath,
		"PATH=" + strings.Join([]string{shimDir, denyDir, os.Getenv("PATH")}, string(os.PathListSeparator))}
}

func TestStewardBoundaryRefreshUsesServedInstallation(t *testing.T) {
	t.Parallel()
	root, environment := newProcessGitFixtureEnvironment(t)
	bin, err := filepath.Abs("../../bin/metasystem")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(filepath.Dir(bin)) == root {
		t.Fatal("the enrolled binary must belong to a different installation")
	}
	for _, path := range []string{"go.mod", "cmd/devgate/main.go", "cmd/metasystem/main.go"} {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("package main\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	configPath := filepath.Join(root, "git-replies.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	var config processGitConfig
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	pins := []string{"-C", root, "-c", "core.useReplaceRefs=false", "-c", "core.hooksPath=/dev/null", "-c", "gc.auto=0", "-c", "maintenance.auto=false"}
	for _, reply := range []struct {
		args   []string
		output string
	}{
		{[]string{"config", "--local", "--no-includes", "--get", "metasystem.steward.landing-ref"}, "refs/remotes/origin/main\n"},
		{[]string{"fetch", "--progress", "origin", "main"}, ""},
		{[]string{"rev-parse", "--verify", "refs/remotes/origin/main^{commit}"}, processGitHead + "\n"},
		{[]string{"rev-parse", "--verify", "HEAD^{commit}"}, processGitHead + "\n"},
		{[]string{"merge-base", "--is-ancestor", processGitHead, processGitHead}, ""},
		{[]string{"diff", "--name-only", "--no-renames", "--no-relative", "-z", "HEAD", "--"}, ""},
		{[]string{"ls-files", "--others", "--exclude-standard", "--full-name", "-z"}, ""},
		{[]string{"diff", "--name-only", "--no-renames", "--no-relative", "-z", processGitHead, processGitHead, "--"}, ""},
	} {
		config.Replies = append(config.Replies, processGitReply{Cwd: cwd, Args: append(append([]string(nil), pins...), reply.args...), Stdout: []byte(reply.output)})
	}
	config.Replies = append(config.Replies, processGitReply{Cwd: cwd, Args: []string{"-C", root, "config", "--get", "metasystem.steward.rearm-resolve-seconds"}, Exit: 1})
	data, err = json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	digest, err := installDigest(bin)
	if err != nil {
		t.Fatal(err)
	}
	if err := MintIdentity(RepoIdentityPath(root), InstallIdentity{RepoIdentity: root, Generation: 1, InstallPath: bin,
		InstallDigest: digest, EngineBuild: processGitHead, LandedCommit: processGitHead, MintedAt: time.Now().UTC().Format(time.RFC3339)}); err != nil {
		t.Fatal(err)
	}
	if err := NoteDeferredRearm(root, processGitHead, "old", time.Now()); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	command := exec.Command(bin, "steward", "run", "--repo", root)
	command.Env = append(os.Environ(), environment...)
	command.Stdout, command.Stderr = &output, &output
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := stopRunnerLoop(root); err != nil {
			t.Error(err)
		}
		if err := command.Wait(); err != nil {
			t.Errorf("runner exit: %v\n%s", err, output.String())
		}
	})
	testenv.Await(t, "the served checkout's fetch to clear its stale deferral", func() bool {
		calls, err := os.ReadFile(config.Unexpected)
		if err != nil || len(calls) != 0 {
			t.Fatalf("boundary refresh made an undeclared Git call: %s %v", calls, err)
		}
		return RearmDeferredLine(root) == ""
	})
}

func appendProcessGitEvent(path, line string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(line + "\n")
	return err
}

func runProcessGitHelper(args []string) int {
	configPath := os.Getenv(processGitConfigEnv)
	data, err := os.ReadFile(configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "private git config:", err)
		return 97
	}
	var config processGitConfig
	if err := json.Unmarshal(data, &config); err != nil {
		fmt.Fprintln(os.Stderr, "private git config:", err)
		return 97
	}
	pid := os.Getpid()
	exact, state, err := (identity.KernelProber{}).ReadStart(int64(pid))
	if err != nil || state != identity.Alive {
		fmt.Fprintln(os.Stderr, "private git identity:", state, err)
		return 97
	}
	if err := appendProcessGitEvent(config.Events, fmt.Sprintf("start %d %d %d %d %s", pid, os.Getppid(), exact.StartedAt.UnixMicro(), exact.StartTicks, exact.BootID)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 97
	}
	defer func() { _ = appendProcessGitEvent(config.Events, fmt.Sprintf("done %d", pid)) }()
	cwd, cwdErr := os.Getwd()
	input, inputErr := io.ReadAll(os.Stdin)
	if cwdErr == nil && inputErr == nil && len(input) == 0 {
		for _, reply := range config.Replies {
			if cwd == reply.Cwd && reflect.DeepEqual(args, reply.Args) {
				_, _ = os.Stdout.Write(reply.Stdout)
				_, _ = os.Stderr.Write(reply.Stderr)
				return reply.Exit
			}
		}
	}
	_ = appendProcessGitEvent(config.Unexpected, fmt.Sprintf("cwd=%q argv=%q stdin=%d cwdErr=%v stdinErr=%v", cwd, args, len(input), cwdErr, inputErr))
	fmt.Fprintln(os.Stderr, "undeclared private Git call")
	return 97
}

func checkProcessGitFixture(t *testing.T, config processGitConfig, denied string) {
	t.Helper()
	data, err := os.ReadFile(config.Events)
	if err != nil {
		t.Error(err)
		return
	}
	starts := map[int]identity.Ref{}
	completed := map[int]bool{}
	childCalls := 0
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			t.Errorf("malformed Git helper event %q", line)
			continue
		}
		pid, err := strconv.Atoi(fields[1])
		if err != nil {
			t.Errorf("malformed Git helper pid %q", line)
			continue
		}
		switch fields[0] {
		case "start":
			if len(fields) < 5 {
				t.Errorf("malformed Git helper start %q", line)
				continue
			}
			micro, microErr := strconv.ParseInt(fields[3], 10, 64)
			ticks, ticksErr := strconv.ParseInt(fields[4], 10, 64)
			if microErr != nil || ticksErr != nil {
				t.Errorf("malformed Git helper identity %q", line)
				continue
			}
			exact := identity.Exact{Pid: int64(pid), StartedAt: time.UnixMicro(micro), StartTicks: ticks}
			if len(fields) == 6 {
				exact.BootID = fields[5]
			}
			ref := exact.Ref()
			if !ref.NativeExact() {
				t.Errorf("non-native Git helper identity %q", line)
				continue
			}
			starts[pid] = ref
			if fields[2] != strconv.Itoa(os.Getpid()) {
				childCalls++
			}
		case "done":
			completed[pid] = true
		default:
			t.Errorf("malformed Git helper event %q", line)
		}
	}
	completedCount, interruptedGone := 0, 0
	for pid, ref := range starts {
		// A helper the killed CLI left running is ended here, by its exact
		// identity; its leaving the kernel's table is the event awaited.
		if !processGitHelperGone(ref) {
			exact, state, readErr := (identity.KernelProber{}).ReadStart(ref.Pid)
			if readErr == nil && state == identity.Alive && identity.Compare(exact, ref).Matches {
				_ = syscall.Kill(pid, syscall.SIGKILL)
				testenv.Await(t, fmt.Sprintf("killed Git helper %d to exit", pid), func() bool { return processGitHelperGone(ref) })
			}
		}
		if refreshed, readErr := os.ReadFile(config.Events); readErr == nil {
			completed[pid] = strings.Contains(string(refreshed), fmt.Sprintf("done %d\n", pid))
		}
		gone := processGitHelperGone(ref)
		if !gone {
			t.Errorf("Git helper %d survived fixture cleanup (completed=%t)", pid, completed[pid])
		}
		if completed[pid] {
			completedCount++
		} else if gone {
			interruptedGone++
		}
	}
	if childCalls == 0 {
		t.Error("installed CLI made no Git helper calls")
	}
	for _, path := range []string{config.Unexpected, denied} {
		contents, err := os.ReadFile(path)
		if err != nil || len(contents) != 0 {
			t.Errorf("Git fixture log %s must exist and be empty: %q %v", path, contents, err)
		}
	}
	t.Logf("Git helpers started=%d completed=%d interrupted-and-gone=%d installed-child calls=%d", len(starts), completedCount, interruptedGone, childCalls)
}

func processGitHelperGone(ref identity.Ref) bool {
	if !ref.NativeExact() {
		return false
	}
	prober := identity.KernelProber{}
	exact, state, _ := prober.ReadStart(ref.Pid)
	if state == identity.Dead || (state == identity.Alive && exact.Zombie) {
		return true
	}
	if state != identity.Alive {
		return false
	}
	return !identity.Compare(exact, ref).Matches
}
