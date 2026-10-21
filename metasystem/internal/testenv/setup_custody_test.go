package testenv

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"golang.org/x/sys/unix"
)

const (
	setupCustodyFixtureEnv = "TESTENV_SETUP_CUSTODY_FIXTURE"
	setupCustodySleeperEnv = "TESTENV_SETUP_CUSTODY_SLEEPER"
)

// setupCustodySleeper is the long child: it waits for a signal that never
// comes, the way a slow `go build` outlives a killed parent.
func setupCustodySleeper() {
	wake := make(chan os.Signal, 1)
	signal.Notify(wake, syscall.SIGUSR1)
	<-wake
}

// setupCustodyFixture is a package setup that starts a long child the way
// the wait candidate's `go build` starts: tagged with a fixture key this
// binary registered, inheriting the namespace. It reports the child and
// then blocks until its parent kills it.
func setupCustodyFixture() error {
	key, word, err := SetupFixtureTag("setup-child")
	if err != nil {
		return err
	}
	child := exec.Command(os.Args[0])
	child.Env = append(os.Environ(), setupCustodySleeperEnv+"=1", word)
	if err := child.Start(); err != nil {
		return err
	}
	encoded, _ := identity.EncodeKey(key)
	custodian, _ := FixtureCustodian()
	encodedCustodian, _ := identity.EncodeRef(custodian)
	fmt.Printf("child=%d\nkey=%s\nregistry=%s\ncustodian=%s\nready\n", child.Process.Pid, encoded, os.Getenv(supervisionRegistryHome), encodedCustodian)
	_, _ = io.Copy(io.Discard, os.Stdin)
	return nil
}

// Killing a binary during setup leaves its custodian responsible for ending
// the tagged child. The custodian's log lock signals its exit; a following
// start then reports the retained log and removes the settled registry home.
func TestSetupChildIsEndedWhenTheBinaryIsKilled(t *testing.T) {
	t.Parallel()
	command := exec.Command(os.Args[0], "-test.run=^TestMainWithSetupFixtureProcess$", "-test.count=1")
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if name != supervisionRegistryHome && name != registryOwnerNonce && name != setupCustodyFixtureEnv && name != identity.RunOwnerEnv {
			command.Env = append(command.Env, entry)
		}
	}
	command.Env = append(command.Env, setupCustodyFixtureEnv+"=1")
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stdin.Close()
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	report := map[string]string{}
	lines := bufio.NewScanner(stdout)
	for lines.Scan() && lines.Text() != "ready" {
		if name, value, ok := strings.Cut(lines.Text(), "="); ok {
			report[name] = value
		}
	}
	childPid, _ := strconv.Atoi(report["child"])
	key, keyErr := identity.ParseKey(report["key"])
	custodian, custodianErr := identity.ParseRef(report["custodian"])
	registry := report["registry"]
	if childPid < 1 || keyErr != nil || custodianErr != nil || !strings.HasPrefix(filepath.Base(registry), registryHomePrefix) {
		_ = command.Process.Kill()
		_ = command.Wait()
		t.Fatalf("fixture did not report its child: %v %v %v", report, keyErr, custodianErr)
	}
	t.Cleanup(func() { _ = syscall.Kill(childPid, syscall.SIGKILL) })
	childExact, childState, probeErr := identity.KernelProber{}.Probe(int64(childPid))
	if probeErr != nil || childState != identity.Alive {
		t.Fatalf("probe the setup child: %v %v", childState, probeErr)
	}
	childRef := childExact.Ref()
	evidence := setupCustodyEvidence{registry: registry, owner: key.Owner, custodian: custodian, child: childRef, prober: identity.KernelProber{}}
	remaining := func() (time.Duration, bool) { return DeadlineRemaining(t) }
	t.Logf("setup fixture registry=%s owner=%+v custodian=%+v child=%+v", registry, key.Owner, custodian, childRef)
	if survivors, err := identity.FixtureSurvivors(key); err != nil || len(survivors) != 1 {
		t.Fatalf("the tagged setup child is not visible as the key's fixture before the kill: %v %v", survivors, err)
	}

	logPath := registry + ".custodian-" + strconv.Itoa(command.Process.Pid) + ".log"
	log, err := os.Open(logPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = unlockAndClose(log) })
	if err := unix.Flock(int(log.Fd()), unix.LOCK_EX|unix.LOCK_NB); !lockWouldBlock(err) {
		t.Fatalf("live custodian did not hold its log lock: %v", err)
	}
	if err := command.Process.Signal(syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	_ = command.Wait()

	followingStart := func() {
		t.Helper()
		following := exec.Command(os.Args[0], "-test.run=^TestMainWithSetupFixtureProcess$", "-test.count=1")
		following.Env = command.Env[:len(command.Env)-1]
		if output, err := following.CombinedOutput(); err != nil {
			t.Fatalf("a following start failed: %v %s", err, output)
		}
	}
	evidence.wait(t, "the custodian to exit", func() bool {
		err := unix.Flock(int(log.Fd()), unix.LOCK_EX|unix.LOCK_NB)
		if lockWouldBlock(err) {
			return false
		}
		if err != nil {
			t.Fatalf("lock the custodian log: %v", err)
		}
		return true
	}, awaitPause, remaining, nil, nil)
	if err := unlockAndClose(log); err != nil {
		t.Fatal(err)
	}
	exact, state, err := identity.KernelProber{}.Probe(childRef.Pid)
	if !fixtureExited(exact, state, err, childRef) {
		t.Fatalf("setup child remains after custodian exit: identity=%+v state=%v zombie=%t err=%v", exact.Ref(), state, exact.Zombie, err)
	}
	if survivors, err := identity.FixtureSurvivors(key); err != nil || len(survivors) != 0 {
		t.Fatalf("the custodian did not end the setup child: survivors=%v err=%v", survivors, err)
	}
	t.Log(evidence.describe())
	followingStart()
	if _, err := os.Stat(registry); !os.IsNotExist(err) {
		t.Fatalf("a following start kept the dead binary's settled registry home %s: %v", registry, err)
	}
}
