package supervise

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"syscall"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

func TestAKilledTestBinarysHelpersAreEndedByItsCustodian(t *testing.T) {
	t.Parallel()
	if killTheCustodyChild(t) {
		return
	}
	options, started := prepareExecutionOwner(t)
	key, err := identity.EncodeKey(armingHelpers.key)
	if err != nil {
		t.Fatal(err)
	}
	held, err := recordedHeld(options.Root)
	if err != nil {
		t.Fatal(err)
	}
	reportRef := func(name string, recorded identity.Ref) {
		exact, state, err := (identity.KernelProber{}).Probe(recorded.Pid)
		if err != nil || state != identity.Alive || !identity.SameIdentity(exact, recorded) {
			t.Fatalf("report %s identity: state=%v err=%v", name, state, err)
		}
		ref, err := identity.EncodeRef(exact.Ref())
		if err != nil {
			t.Fatal(err)
		}
		fmt.Printf("%s=%s\n", name, ref)
	}
	fmt.Printf("key=%s\n", key)
	reportRef("owner", identity.Ref{Pid: started.Owner.Pid, StartedAtSec: started.Owner.PidStartedAt, StartTicks: started.Owner.PidStartTicks, BootID: started.Owner.BootID})
	for _, component := range held {
		reportRef("component", component.Identity)
	}
	fmt.Println("ready")
	_, _ = io.Copy(io.Discard, os.Stdin)
}

// killTheCustodyChild runs the test again in a child binary, kills that
// binary once its helpers are in fixture custody and waits for the
// custodian to end them. It returns false in the child, which reports its
// helpers and blocks on stdin.
func killTheCustodyChild(t *testing.T) bool {
	t.Helper()
	const marker = "SUPERVISE_CUSTODY_CHILD"
	if os.Getenv(marker) == t.Name() {
		return false
	}
	timeout, _ := testenv.DeadlineRemaining(t)
	command := exec.Command(os.Args[0], "-test.run=^"+regexp.QuoteMeta(t.Name())+"$", "-test.timeout="+timeout.String())
	command.Env = append(os.Environ(), marker+"="+t.Name())
	command.Stderr = os.Stderr
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
	t.Cleanup(func() {
		if command.ProcessState == nil {
			_ = command.Process.Kill()
			_ = command.Wait()
		}
	})
	var key identity.FixtureKey
	var refs []identity.Ref
	ready := false
	var report []string
	lines := bufio.NewScanner(stdout)
	for lines.Scan() {
		report = append(report, lines.Text())
		if lines.Text() == "ready" {
			ready = true
			break
		}
		name, value, _ := strings.Cut(lines.Text(), "=")
		switch name {
		case "key":
			key, err = identity.ParseKey(value)
		case "owner", "component":
			var ref identity.Ref
			ref, err = identity.ParseRef(value)
			if err == nil {
				refs = append(refs, ref)
				t.Cleanup(func() { _ = identity.SignalExact(identity.KernelProber{}, ref, syscall.SIGKILL) })
			}
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	if !ready || len(refs) != 3 {
		t.Fatalf("child did not report its owner and two components: refs=%v ready=%t scan=%v\n%s", refs, ready, lines.Err(), strings.Join(report, "\n"))
	}
	survivors, err := identity.FixtureSurvivors(key)
	var got, want []int64
	for _, survivor := range survivors {
		got = append(got, survivor.Ref.Pid)
	}
	for _, ref := range refs {
		want = append(want, ref.Pid)
	}
	slices.Sort(got)
	slices.Sort(want)
	if err != nil || !slices.Equal(got, want) {
		t.Fatalf("helpers in fixture custody before kill: pids=%v want=%v err=%v", got, want, err)
	}
	if err := command.Process.Signal(syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	_ = command.Wait()
	testenv.Await(t, "the killed binary's custodian ends its helpers", func() bool {
		survivors, err := identity.FixtureSurvivors(key)
		if err != nil {
			t.Fatal(err)
		}
		if len(survivors) != 0 {
			return false
		}
		for _, ref := range refs {
			exact, state, err := (identity.KernelProber{}).Probe(ref.Pid)
			if err != nil || state != identity.Dead && (state != identity.Alive || identity.SameIdentity(exact, ref) && !exact.Zombie) {
				return false
			}
		}
		return true
	})
	return true
}
