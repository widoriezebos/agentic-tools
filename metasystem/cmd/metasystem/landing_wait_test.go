package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gaterun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"golang.org/x/sys/unix"
)

func TestLandingWaitOutlastsUnitDeadlineAndReattaches(t *testing.T) {
	t.Parallel()
	for _, gate := range []bool{false, true} {
		for _, attach := range []bool{false, true} {
			t.Run(fmt.Sprintf("gate=%t/attach=%t", gate, attach), func(t *testing.T) {
				t.Parallel()
				b := newMergeGateBed(t)
				b.fail = func(*exec.Cmd, string) (string, error) { return "LANDING-CHECKED\t0\n", nil }
				b.prepareBatch(t)
				now := laneTestNow
				b.owners.landing.plainProve.Now = func() time.Time { return now }
				git := b.owners.landing.plainProve.Git
				b.owners.landing.plainProve.Git = func(dir string, args ...string) (string, error) {
					body, err := git(dir, args...)
					if len(args) == 2 && args[0] == "show" && strings.HasSuffix(args[1], ":metasystem/metasystem.conf") {
						body += "proof.deadline=15\n"
					}
					return body, err
				}
				owners := proofWaitFixtureOwners(t, b.owners, b.install, b.root)
				pollChild := owners.landing.plainProve.WaitPoll
				polls := 0
				owners.landing.plainProve.WaitPoll = func() {
					polls++
					if polls == 1 {
						now = now.Add(20 * time.Minute)
						return
					}
					pollChild()
				}
				args := []string{"--json"}
				if gate {
					args = append(args, "--gate")
				}
				run := func(args []string) (int, string) {
					var output bytes.Buffer
					code := runIntentIn(mustIntentCommand(t, "landing prove"), args, &output, &output, b.root, owners)
					return code, output.String()
				}
				if attach {
					if code, output := run(args); code != 0 {
						t.Fatalf("background start: exit=%d %s", code, output)
					}
					now = now.Add(20 * time.Minute)
				}
				args = append(args, "--wait")
				var first plain.Result
				for repeat := 0; repeat < 2; repeat++ {
					code, output := run(args)
					var got struct{ Data plain.Result }
					if err := json.Unmarshal([]byte(output), &got); err != nil || code != 0 || got.Data.Result != plain.Green {
						t.Fatalf("wait %d: exit=%d err=%v %s", repeat, code, err, output)
					}
					if repeat == 0 {
						first = got.Data
					} else if !reflect.DeepEqual(first, got.Data) {
						t.Fatalf("repeat wait replaced the recorded result: %+v %+v", first, got.Data)
					}
				}
				if polls != 2 {
					t.Fatalf("wait reran the child: polls=%d", polls)
				}
			})
		}
	}
}

func TestLandingWaitReturnsDetachedRecordedResult(t *testing.T) {
	t.Parallel()
	for _, exit := range []int{0, 9} {
		t.Run(fmt.Sprintf("exit=%d", exit), func(t *testing.T) {
			t.Parallel()
			bed := newPlainVerbBed(t)
			bed.cwd = bed.checkout
			bed.setCommand(t, fmt.Sprintf("printf 'private %%s stream\\nLANDING-CHECKED\\t0\\n' proof; exit %d", exit))
			bed.owners.prove = enrolledPersonProver(t, bed.installation, laneTestNow)
			launches := 0
			launch := bed.owners.landing.plainProve.Launch
			bed.owners.landing.plainProve.Launch = func(argv []string, dir, log string) (int64, error) { launches++; return launch(argv, dir, log) }
			bed.realLaunch = bed.owners.landing.plainProve.Launch
			code, output := bed.run(t, "landing", "prove", "--wait", "--json")
			var got struct{ Data plain.Result }
			if err := json.Unmarshal([]byte(output), &got); err != nil {
				t.Fatalf("wait output: %v %s", err, output)
			}
			saved, found, err := plain.LastResult(bed.installation)
			wantCode, wantResult := 0, plain.Green
			if exit != 0 {
				wantCode, wantResult = 1, plain.Red
			}
			if code != wantCode || launches != 1 || err != nil || !found || got.Data.Result != wantResult || !reflect.DeepEqual(got.Data, saved) || got.Data.Person == nil || got.Data.Person.Person != "Wido" || strings.Contains(output, "private proof stream") {
				t.Fatalf("wait bypassed detached saved result: exit=%d launches=%d result=%+v saved=%+v err=%v output=%s", code, launches, got.Data, saved, err, output)
			}
			log, err := os.ReadFile(saved.Log)
			if err != nil || !strings.Contains(string(log), "private proof stream") {
				t.Fatalf("proof stream missing from log: %s %v", log, err)
			}
		})
	}
}

func TestLandingWaitKilledDetachedRunReturnsRed(t *testing.T) {
	t.Parallel()
	bed := newPlainVerbBed(t)
	bed.cwd = bed.checkout
	bed.setCommand(t, "printf 'LANDING-CHECKED\\t0\\n'")
	bed.owners.prove = enrolledPersonProver(t, bed.installation, laneTestNow)
	completed := make(chan error, 1)
	blocked := filepath.Join(t.TempDir(), "child-exit.fifo")
	if err := unix.Mkfifo(blocked, 0600); err != nil {
		t.Fatal(err)
	}
	var pid int64
	bed.owners.landing.plainProve.Launch = func(argv []string, dir, log string) (int64, error) {
		var err error
		pid, err = gaterun.LaunchDetached(gaterun.DetachedLaunch{Argv: argv, Dir: dir, Log: log, Completed: completed,
			Env: []string{"GO_WANT_BATCH_E2E_COMMAND=1", "METASYSTEM_SUPERVISION_REGISTRY_HOME=" + bed.registry, engineChildExited + "=" + blocked}})
		return pid, err
	}
	bed.realLaunch = bed.owners.landing.plainProve.Launch
	killed := false
	bed.owners.landing.plainProve.WaitPoll = func() {
		if killed {
			return
		}
		killed = true
		if err := syscall.Kill(-int(pid), syscall.SIGKILL); err != nil {
			t.Fatal(err)
		}
		if err := <-completed; err == nil {
			t.Fatal("killed child exited successfully")
		}
	}
	code, output := bed.run(t, "landing", "prove", "--wait", "--json")
	var got struct{ Data plain.Result }
	if err := json.Unmarshal([]byte(output), &got); err != nil {
		t.Fatalf("killed wait output: %v %s", err, output)
	}
	if code != 1 || !killed || got.Data.Result != plain.Red || got.Data.Cause == nil || got.Data.Cause.Name != "lost-process" || got.Data.Person == nil {
		t.Fatalf("killed detached check did not return its red: %d %s", code, output)
	}
	saved, found, err := plain.LastResult(bed.installation)
	if err != nil || !found || !reflect.DeepEqual(saved, got.Data) {
		t.Fatalf("killed wait invented a result: %+v %v", saved, err)
	}
}
