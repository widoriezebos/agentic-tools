package steward

import (
	"errors"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

func TestReplacementSignalsSelectedRunner(t *testing.T) {
	type delivery struct {
		signal syscall.Signal
		at     time.Duration
	}
	term := delivery{syscall.SIGTERM, 0}
	cont := delivery{syscall.SIGCONT, 0}
	kill := delivery{syscall.SIGKILL, 2 * time.Second}
	for _, tc := range []struct {
		name    string
		want    []delivery
		problem string
	}{
		{"ordinary", []delivery{term, cont, kill}, ""},
		{"TERM finds no process", []delivery{term}, ""},
		{"reuse after TERM", []delivery{term}, ""},
		{"unknown before TERM", nil, "pid 41"},
		{"runner unpublishes after TERM then exits", []delivery{term}, ""},
		{"missing announcement during wait", []delivery{term, cont}, "announcement"},
		{"replaced announcement during wait", []delivery{term, cont}, "announcement"},
		{"new announcement at reused PID", []delivery{term, cont}, "announcement"},
		{"reuse at KILL", []delivery{term, cont}, ""},
		{"unknown after KILL", []delivery{term, cont, kill}, "pid 41"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.MkdirAll(runnerDir(root), 0o755); err != nil {
				t.Fatal(err)
			}
			selected := RunnerRecord{Pid: 41, BootID: "boot", StartTicks: 19, PidStartedAt: 100}
			if err := writeJSONAtomic(runnerRecordPath(root), selected); err != nil {
				t.Fatal(err)
			}
			process := identity.Exact{Pid: 41, BootID: "boot", StartTicks: 19, StartedAt: time.Unix(100, 0)}
			state := identity.Alive
			if tc.name == "unknown before TERM" {
				state = identity.Unknown
			}
			probe := censusSeamProbe(func(pid int64) (identity.Exact, identity.Liveness, error) {
				if pid != selected.Pid {
					return identity.Exact{Pid: pid, StartedAt: time.Unix(101, 0)}, identity.Alive, nil
				}
				if state == identity.Unknown {
					return identity.Exact{}, state, errors.New("process table unavailable")
				}
				return process, state, nil
			})
			var elapsed time.Duration
			var sent []delivery
			signal := func(pid int, sig syscall.Signal) error {
				if int64(pid) != selected.Pid {
					t.Fatalf("signal to pid %d", pid)
				}
				sent = append(sent, delivery{sig, elapsed})
				switch sig {
				case syscall.SIGTERM:
					switch tc.name {
					case "TERM finds no process":
						state = identity.Dead
						return syscall.ESRCH
					case "runner unpublishes after TERM then exits":
						if err := os.Remove(runnerRecordPath(root)); err != nil {
							t.Fatal(err)
						}
					case "reuse after TERM":
						process.StartTicks++
					}
				case syscall.SIGKILL:
					state = identity.Dead
					if tc.name == "unknown after KILL" {
						state = identity.Unknown
					}
				}
				return nil
			}
			sleep := func(d time.Duration) {
				if d != 50*time.Millisecond {
					t.Fatalf("poll interval = %v", d)
				}
				elapsed += d
				if elapsed == d {
					switch tc.name {
					case "runner unpublishes after TERM then exits":
						state = identity.Dead
					case "missing announcement during wait":
						if err := os.Remove(runnerRecordPath(root)); err != nil {
							t.Fatal(err)
						}
					case "new announcement at reused PID":
						process.StartTicks++
						newRunner := selected
						newRunner.StartTicks = process.StartTicks
						if err := writeJSONAtomic(runnerRecordPath(root), newRunner); err != nil {
							t.Fatal(err)
						}
					case "replaced announcement during wait":
						if err := writeJSONAtomic(runnerRecordPath(root), RunnerRecord{Pid: 42, PidStartedAt: 101}); err != nil {
							t.Fatal(err)
						}
					}
				}
				if tc.name == "reuse at KILL" && elapsed == 2*time.Second {
					process.StartTicks++
				}
			}
			err := stopRunnerForReplacementWithClock(root, selected, signal, func() time.Time { return time.Unix(0, 0).Add(elapsed) }, sleep, probe)
			if !reflect.DeepEqual(sent, tc.want) || (err != nil) != (tc.problem != "") || err != nil && !strings.Contains(err.Error(), tc.problem) {
				t.Fatalf("deliveries %v, error %v; want %v, problem %q", sent, err, tc.want, tc.problem)
			}
		})
	}
}

func TestLaunchRejectsForeignRunnerAnnouncement(t *testing.T) {
	root := canonicalPath(t.TempDir())
	engine, err := exec.LookPath("true")
	if err != nil {
		t.Fatal(err)
	}
	digest, err := installDigest(engine)
	if err != nil {
		t.Fatal(err)
	}
	if err := MintIdentity(RepoIdentityPath(root), InstallIdentity{RepoIdentity: root, Generation: 1, InstallPath: canonicalPath(engine), InstallDigest: digest, Enrollment: EnrollmentFixture, MintedAt: time.Now().UTC().Format(time.RFC3339)}); err != nil {
		t.Fatal(err)
	}
	binary, err := OpenEnrolledBinary(root)
	if err != nil {
		t.Fatal(err)
	}
	defer binary.Close()
	if err := binary.PrepareForExecution(); err != nil {
		t.Fatal(err)
	}
	self, state, err := (identity.KernelProber{}).ReadStart(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("fixture process: %v %v", state, err)
	}
	if err := writeJSONAtomic(runnerRecordPath(root), RunnerRecord{Pid: self.Pid, StartTicks: self.StartTicks, BootID: self.BootID, PidStartedAt: self.StartedAt.Unix()}); err != nil {
		t.Fatal(err)
	}
	if rec, err := launchRunner(root, binary, ""); err == nil {
		t.Fatalf("accepted foreign pid %d", rec.Pid)
	}
}

type replacementClock struct{ now time.Time }

func (c *replacementClock) Now() time.Time { return c.now }
func (c *replacementClock) After(d time.Duration) <-chan time.Time {
	c.now = c.now.Add(d)
	ch := make(chan time.Time, 1)
	ch <- c.now
	return ch
}

func TestArmReplacementUsesSelectedProcessAndClock(t *testing.T) {
	for _, unavailable := range []bool{false, true} {
		b := newExecutionBed(t)
		start := b.clock()
		clock := &replacementClock{start}
		b.deps.clock = clock
		if unavailable {
			b.deps.prober = censusSeamProbe(func(int64) (identity.Exact, identity.Liveness, error) {
				return identity.Exact{}, identity.Unknown, errors.New("process table unavailable")
			})
		}
		var signals []syscall.Signal
		b.deps.signal = func(pid int, sig syscall.Signal) error {
			signals = append(signals, sig)
			if sig == syscall.SIGKILL {
				delete(b.processes, int64(pid))
			}
			return nil
		}
		b.deps.launch = func(string, *EnrolledBinary, string) (RunnerRecord, error) {
			b.launches++
			return b.record(t, b.replacement), nil
		}
		got, err := armWithRearmDeps(b.root, b.pinned.Install.InstallPath, true, false, true, "", humanMintDecision("human-terminal", "", "", EnrollmentHumanTerminal), b.deps)
		if unavailable {
			if err == nil || !strings.Contains(err.Error(), "pid 41") || b.launches != 0 || len(signals) != 0 {
				t.Fatalf("unknown runner: launches %d, signals %v, error %v", b.launches, signals, err)
			}
		} else if err != nil || got.RunnerPid != b.replacement.Pid || !reflect.DeepEqual(signals, []syscall.Signal{syscall.SIGTERM, syscall.SIGCONT, syscall.SIGKILL}) || clock.Now().Sub(start) != 2*time.Second {
			t.Fatalf("arm: pid %d, signals %v, elapsed %v, error %v", got.RunnerPid, signals, clock.Now().Sub(start), err)
		}
	}
}
