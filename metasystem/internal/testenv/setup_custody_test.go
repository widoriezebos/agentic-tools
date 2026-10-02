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

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
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

// SIGKILL during setup (DL3A-08): a test binary killed while its setup's
// tagged child runs leaves no untended writer. The custodian ends the child
// (FixtureSurvivors for the key reads empty), and a following start removes
// the dead binary's registry home and namespace, with nothing writing into
// them. A start sweeps a home only once its custodian has settled (HomeSettled:
// its records removed and its log's lock freed by its exit), so the test holds
// the custodian stopped to prove a start keeps the home until then, and waits
// on that settlement, never on a number of starts, before the start that must
// sweep it.
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
	if survivors, err := identity.FixtureSurvivors(key); err != nil || len(survivors) != 1 {
		t.Fatalf("the tagged setup child is not visible as the key's fixture before the kill: %v %v", survivors, err)
	}

	// Hold the custodian before the kill, so it cannot settle before the
	// first following start looks at the home.
	if err := identity.SignalExact(identity.KernelProber{}, custodian, syscall.SIGSTOP); err != nil {
		t.Fatalf("stop the fixture custodian: %v", err)
	}
	resume := func() { _ = identity.SignalExact(identity.KernelProber{}, custodian, syscall.SIGCONT) }
	t.Cleanup(resume)
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
	// The owner is dead, but its custodian has neither ended the child nor
	// released its log: the home is not settled and a start keeps it.
	followingStart()
	if _, err := os.Stat(registry); err != nil {
		t.Fatalf("a following start removed the registry home of an unsettled custodian: %v", err)
	}
	resume()

	// The wait ends when the child has exited: gone, or a zombie its new
	// parent has not reaped yet. A zombie still answers kill(pid, 0).
	Await(t, "the custodian to end the setup child", func() bool {
		exact, state, err := identity.KernelProber{}.Probe(childRef.Pid)
		return fixtureExited(exact, state, err, childRef)
	})
	if survivors, err := identity.FixtureSurvivors(key); err != nil || len(survivors) != 0 {
		t.Fatalf("the custodian did not end the setup child: survivors=%v err=%v", survivors, err)
	}

	// The custodian settles on its own after the child: it removes its records
	// and exits, freeing its log's lock. Another binary's start may sweep the
	// home first, which is the same outcome.
	Await(t, "the custodian to settle the dead binary's registry home", func() bool {
		_, err := os.Lstat(registry)
		return os.IsNotExist(err) || HomeSettled(registry)
	})
	followingStart()
	if _, err := os.Stat(registry); !os.IsNotExist(err) {
		t.Fatalf("a following start kept the dead binary's settled registry home %s: %v", registry, err)
	}
}
