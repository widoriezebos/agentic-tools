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
	fmt.Printf("child=%d\nkey=%s\nregistry=%s\nready\n", child.Process.Pid, encoded, os.Getenv(supervisionRegistryHome))
	_, _ = io.Copy(io.Discard, os.Stdin)
	return nil
}

// SIGKILL during setup (DL3A-08): a test binary killed while its setup's
// tagged child runs leaves no untended writer. The custodian ends the child
// (FixtureSurvivors for the key reads empty within the custodian bound), and
// a following start removes the dead binary's registry home and namespace,
// with nothing writing into them.
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
	registry := report["registry"]
	if childPid < 1 || keyErr != nil || !strings.HasPrefix(filepath.Base(registry), registryHomePrefix) {
		_ = command.Process.Kill()
		_ = command.Wait()
		t.Fatalf("fixture did not report its child: %v %v", report, keyErr)
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
	if err := command.Process.Signal(syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	_ = command.Wait()

	// The wait ends when the child has exited: gone, or a zombie its new
	// parent has not reaped yet. A zombie still answers kill(pid, 0), so the
	// end is the wait's own verdict (it logs only when its bound expired).
	var waitLog strings.Builder
	waitForFixtureExit(identity.KernelProber{}, childRef, 20*time.Second, &waitLog)
	if survivors, err := identity.FixtureSurvivors(key); err != nil || len(survivors) != 0 || waitLog.Len() != 0 {
		t.Fatalf("the custodian did not end the setup child: survivors=%v err=%v %s", survivors, err, waitLog.String())
	}

	// A following start sweeps dead registry homes once their custodian has
	// settled; each start is its own attempt, bounded by count.
	for attempt := 1; ; attempt++ {
		following := exec.Command(os.Args[0], "-test.run=^TestMainWithSetupFixtureProcess$", "-test.count=1")
		following.Env = command.Env[:len(command.Env)-1]
		if output, err := following.CombinedOutput(); err != nil {
			t.Fatalf("a following start failed: %v %s", err, output)
		}
		if _, err := os.Stat(registry); os.IsNotExist(err) {
			break
		}
		if attempt == 50 {
			t.Fatalf("fifty following starts kept the dead binary's registry home %s", registry)
		}
	}
}
