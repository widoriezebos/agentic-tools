package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

func TestLandingPausedPersonProofRefusesChildArgvReplay(t *testing.T) {
	t.Parallel()
	for _, dead := range []bool{false, true} {
		t.Run(map[bool]string{false: "live child", true: "dead child"}[dead], func(t *testing.T) {
			t.Parallel()
			b := newReplayVerbBed(t)
			helmMust(t, os.WriteFile(filepath.Join(b.install, "metasystem.conf"), []byte("metasystem.template=true\nproof.full=fixture\n"), 0600))
			b.owners.resolver = stateroot.NewResolver(func(string) (string, error) { return b.root, nil }, os.Executable)
			b.prepareBatch(t)
			_, err := lane.SetPause(b.home, "Wido", laneTestNow)
			helmMust(t, err)
			b.owners.prove = enrolledPersonProver(t, b.install, laneTestNow)
			var childArgv []string
			waited := false
			child := exec.Command("cat")
			input, err := child.StdinPipe()
			helmMust(t, err)
			t.Cleanup(func() { _ = input.Close() })
			b.owners.landing.plainProve.Executable = func() (string, error) { return "/fixture/engine", nil }
			b.owners.landing.plainProve.Launch = func(argv []string, _, _ string) (int64, error) {
				childArgv = slices.Clone(argv[1:])
				if err := child.Start(); err != nil {
					return 0, err
				}
				t.Cleanup(func() {
					if !waited {
						_ = child.Process.Kill()
						_ = child.Wait()
					}
				})
				return int64(child.Process.Pid), nil
			}
			if code, text := b.run(t, b.root, "prove"); code != 0 {
				t.Fatalf("person's detached proof: %d %s", code, text)
			}
			running, recorded, alive, err := plain.ReadRunning(b.install, b.owners.landing.plainProve)
			if err != nil || !recorded || !alive || running.Person == nil || len(childArgv) == 0 {
				t.Fatalf("person's proof not recorded alive: %+v recorded=%v alive=%v argv=%v err=%v", running, recorded, alive, childArgv, err)
			}
			if dead {
				helmMust(t, child.Process.Kill())
				_ = child.Wait()
				waited = true
			}
			b.owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
				return humanauthority.Proof{}, errors.New("agent replay")
			}
			b.fail = func(*exec.Cmd, string) (string, error) { return "LANDING-CHECKED\t0\n", nil }
			code, text := b.run(t, b.root, childArgv[1:]...)
			if code == 0 || !strings.Contains(text, "landing prove") || b.full != 0 || len(b.runs) != 0 {
				t.Fatalf("agent replay: exit=%d executions=%v output=%s", code, b.runs, text)
			}
			last, found, err := plain.LastResult(b.install)
			if dead {
				if err != nil || !found || last.Result != plain.Red || last.Attempt != running.Attempt || last.Cause == nil || last.Cause.Name != "lost-process" {
					t.Fatalf("dead child did not record its lost-process result: %+v found=%v err=%v", last, found, err)
				}
			} else if err != nil || found {
				t.Fatalf("live replay wrote a result: %+v found=%v err=%v", last, found, err)
			}
			current, recorded, alive, err := plain.ReadRunning(b.install, b.owners.landing.plainProve)
			if err != nil || recorded == dead || !dead && (!alive || current.Attempt != running.Attempt) {
				t.Fatalf("replay changed the child's state: %+v recorded=%v alive=%v err=%v", current, recorded, alive, err)
			}
			if _, paused := lane.ReadPause(b.home); !paused {
				t.Fatal("replay removed the pause")
			}
		})
	}
}
