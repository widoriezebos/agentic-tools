package steward

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
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
	t.Setenv("PATH", denyDir+string(os.PathListSeparator)+os.Getenv("PATH"))
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
	t.Setenv(processGitConfigEnv, configPath)
	t.Setenv("PATH", shimDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Logf("Git fixture root raw=%q canonical=%q", rawRoot, root)
	t.Cleanup(func() { checkProcessGitFixture(t, config, denied) })
	return root
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
	deadline := time.Now().Add(3 * time.Second)
	completedCount, interruptedGone := 0, 0
	for pid, ref := range starts {
		for time.Now().Before(deadline) {
			if completed[pid] && processGitHelperGone(ref) {
				break
			}
			time.Sleep(10 * time.Millisecond)
			if refreshed, readErr := os.ReadFile(config.Events); readErr == nil {
				completed[pid] = strings.Contains(string(refreshed), fmt.Sprintf("done %d\n", pid))
			}
		}
		if !processGitHelperGone(ref) {
			exact, state, readErr := (identity.KernelProber{}).ReadStart(ref.Pid)
			if readErr == nil && state == identity.Alive && identity.Compare(exact, ref).Matches {
				_ = syscall.Kill(pid, syscall.SIGKILL)
				for attempt := 0; attempt < 200 && !processGitHelperGone(ref); attempt++ {
					time.Sleep(10 * time.Millisecond)
				}
			}
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
