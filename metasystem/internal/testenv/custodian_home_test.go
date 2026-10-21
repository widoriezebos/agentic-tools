package testenv

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

func custodianOwner(t *testing.T, pid int64) identity.Ref {
	t.Helper()
	owner := liveTestOwner(t)
	owner.Pid = pid
	return owner
}

func custodianLog(t *testing.T, home string, owner identity.Ref, text string) (string, *os.File) {
	t.Helper()
	path := fmt.Sprintf("%s.custodian-%d.log", home, owner.Pid)
	log, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND|unix.O_NOFOLLOW, 0o600)
	checkTestenv(t, err)
	t.Cleanup(func() { checkTestenv(t, unlockAndClose(log)) })
	_, err = log.WriteString(text)
	checkTestenv(t, err)
	checkTestenv(t, unix.Flock(int(log.Fd()), unix.LOCK_EX))
	return path, log
}

func quiet(owner identity.Ref) string {
	return identity.FixtureCustodianWatchLine(owner) + identity.FixtureCustodianCompletionLine(owner)
}

func TestACustodianIsTheLastOneOut(t *testing.T) {
	a, b, c, h := custodianOwner(t, 4242), custodianOwner(t, 4343), custodianOwner(t, 4444), custodianOwner(t, 4545)
	for _, name := range []string{"normal-end", "live-test", "second-custodian", "unquiet-own-log", "unquiet-other-log", "halted-sibling", "other-home"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			var home *registryHomeLease
			if name == "live-test" {
				var err error
				home, err = createRegistryHomeUnder(os.MkdirTemp, root)
				checkTestenv(t, err)
				t.Cleanup(func() { _ = unlockAndClose(home.lock) })
			} else {
				home = deadRegistryHomeUnder(t, root)
			}
			textA := quiet(a)
			if name == "unquiet-own-log" {
				textA = identity.FixtureCustodianWatchLine(a) + "fixture-custodian action=kill pid=1 carrier=record result=<nil>\n" + identity.FixtureCustodianCompletionLine(a)
			}
			logA, fileA := custodianLog(t, home.path, a, textA)
			var logB, logC, textC, logH, recordsH string
			var fileB *os.File
			suppliedHome := home.path
			switch name {
			case "second-custodian":
				logB, fileB = custodianLog(t, home.path, b, quiet(b))
			case "unquiet-other-log":
				logC = fmt.Sprintf("%s.custodian-%d.log", home.path, c.Pid)
				textC = identity.FixtureCustodianWatchLine(c) + "fixture-custodian action=kill pid=7 carrier=record result=<nil>\n" + identity.FixtureCustodianCompletionLine(c)
				checkTestenv(t, os.WriteFile(logC, []byte(textC), 0o600))
			case "halted-sibling":
				logH = fmt.Sprintf("%s.custodian-%d.log", home.path, h.Pid)
				recordsH = fmt.Sprintf("%s.fixture-refs-%d", home.path, h.Pid)
				checkTestenv(t, os.WriteFile(recordsH, nil, 0o600))
				checkTestenv(t, os.WriteFile(logH, []byte(identity.FixtureCustodianWatchLine(h)+"fixture-custodian action=halt after=6s\n"), 0o600))
			case "other-home":
				suppliedHome = deadRegistryHomeUnder(t, root).path
			}
			finishFixtureCustodian(logA, suppliedHome, a, fileA)
			requireSidecars(t, name == "unquiet-own-log", logA)
			requireSidecars(t, false, fmt.Sprintf("%s.fixture-refs-%d", home.path, a.Pid))
			requireSidecars(t, name != "normal-end", home.path)
			requireText := func(path, want string) {
				t.Helper()
				got, err := os.ReadFile(path)
				if err != nil || string(got) != want {
					t.Errorf("log %s = %q, error=%v; want unchanged text %q", path, got, err, want)
				}
			}
			var report bytes.Buffer
			switch name {
			case "live-test":
				checkTestenv(t, home.cleanupReporting(&report))
				requireSidecars(t, false, home.path)
			case "second-custodian":
				requireText(logB, quiet(b))
				finishFixtureCustodian(logB, home.path, b, fileB)
				requireSidecars(t, false, logB, home.path)
			case "unquiet-own-log":
				requireText(logA, textA)
			case "unquiet-other-log":
				requireText(logC, textC)
				removeDeadRegistryHomes(root, &report)
				if !strings.Contains(report.String(), "action=kill pid=7") {
					t.Errorf("sweep lost the other custodian's diagnostic: %q", report.String())
				}
				requireSidecars(t, false, home.path, logC)
			case "halted-sibling":
				requireSidecars(t, true, logH, recordsH)
			case "other-home":
				requireSidecars(t, true, suppliedHome)
			}
		})
	}
}

func TestACustodianRemovesItsHomeAtItsBinarysEnd(t *testing.T) {
	for _, mode := range []string{"normal-end", "killed"} {
		t.Run(mode, func(t *testing.T) {
			lease, err := createRegistryHomeUnder(os.MkdirTemp, t.TempDir())
			checkTestenv(t, err)
			t.Cleanup(func() { _ = os.RemoveAll(lease.path) })
			t.Cleanup(func() { _ = unlockAndClose(lease.lock) })
			command := exec.Command(os.Args[0], "-test.run=^TestRegistryOwnerProcess$", "-test.count=1", "-test.timeout=30m")
			command.Env = append(withoutRegistryOwnership(os.Environ()), "TESTENV_REGISTRY_HELPER_MODE=wait-return", supervisionRegistryHome+"="+lease.path, "METASYSTEM_TESTENV_REGISTRY_NONCE="+lease.nonce)
			stdin, err := command.StdinPipe()
			checkTestenv(t, err)
			t.Cleanup(func() { _ = stdin.Close() })
			stdout, err := command.StdoutPipe()
			checkTestenv(t, err)
			checkTestenv(t, command.Start())
			t.Cleanup(func() {
				if command.ProcessState == nil {
					_ = command.Process.Kill()
					_ = command.Wait()
				}
			})
			reader := bufio.NewReader(stdout)
			line, err := reader.ReadString('\n')
			if err != nil || line != lease.path+"\n" {
				t.Fatalf("helper home = %q, error=%v; want %q", line, err, lease.path)
			}
			drained := make(chan struct{})
			go func() {
				_, _ = io.Copy(io.Discard, reader)
				close(drained)
			}()
			logPath := fmt.Sprintf("%s.custodian-%d.log", lease.path, command.Process.Pid)
			log, err := os.Open(logPath)
			checkTestenv(t, err)
			t.Cleanup(func() { _ = unlockAndClose(log) })
			checkTestenv(t, unlockAndClose(lease.lock))
			if mode == "normal-end" {
				checkTestenv(t, stdin.Close())
			} else {
				checkTestenv(t, command.Process.Kill())
			}
			<-drained
			err = command.Wait()
			if (mode == "normal-end" && err != nil) || (mode == "killed" && err == nil) {
				t.Fatalf("helper %s exit = %v", mode, err)
			}
			Await(t, "the custodian of "+lease.path+" to exit", func() bool {
				err := unix.Flock(int(log.Fd()), unix.LOCK_EX|unix.LOCK_NB)
				if lockWouldBlock(err) {
					return false
				}
				checkTestenv(t, err)
				return true
			})
			requireSidecars(t, false, lease.path, logPath, fmt.Sprintf("%s.fixture-refs-%d", lease.path, command.Process.Pid))
		})
	}
}
