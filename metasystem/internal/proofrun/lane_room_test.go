package proofrun

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestTheLanesRoomWaitsOrTriesOnceAndNeverStopsTheCheck(t *testing.T) {
	t.Parallel()
	for _, row := range []string{"lane waits", "person at capacity", "person at guard", "unreadable claim"} {
		t.Run(row, func(t *testing.T) {
			t.Parallel()
			directory, conf := isolatedHostResources(t)
			var holder *HostResourceLease
			var guard *os.File
			var err error
			switch row {
			case "person at guard":
				guard, _, err = tryHostFile(filepath.Join(directory, "admission.lock"))
				defer releaseHostProbe(guard)
			case "unreadable claim":
				err = os.WriteFile(filepath.Join(directory, "lease-heavy-"+strings.Repeat("a", 32)), []byte("unreadable"), 0o600)
			default:
				holder, err = acquireHostResourcesIn(t.Context(), directory, directory, conf, "heavy", nil, nil)
				defer holder.Close()
			}
			if err != nil {
				t.Fatal(err)
			}
			var log bytes.Buffer
			waited := make(chan struct{})
			checks := 0
			acquire := func(ctx context.Context) (*HostResourceLease, error) {
				ctx = WithHostResourceWaitObserver(ctx, func() { close(waited) })
				return acquireHostResourcesIn(ctx, directory, directory, conf, "heavy", nil, func() error {
					checks++
					// A person must finish before reaching a retry's check hook.
					if row != "lane waits" && checks == 3 {
						return errors.New("a person's single try reached the retry hook")
					}
					return nil
				})
			}
			type room struct {
				attach  func(*exec.Cmd)
				release func()
			}
			done := make(chan room, 1)
			go func() {
				a, r := laneCheckRoomWith(acquire, strings.HasPrefix(row, "person"), &log)
				done <- room{a, r}
			}()
			if row == "lane waits" {
				// The wait observer follows the first failed scan and its log line.
				select {
				case <-waited:
				case result := <-done:
					result.release()
					t.Fatal("lane returned before its failed-scan observer")
				}
				_ = holder.Close()
			}
			result := <-done
			defer result.release()
			cmd := exec.Command("/usr/bin/true")
			cmd.Env = []string{"KEEP=value"}
			result.attach(cmd)
			if row == "lane waits" {
				if strings.Count(log.String(), "waiting for room") != 1 || len(cmd.ExtraFiles) == 0 || !strings.Contains(strings.Join(cmd.Env, "\n"), inheritedHostResourceFDs+"=") {
					t.Fatalf("waiting lane did not retain and attach its slot: log=%q files=%v env=%v", log.String(), cmd.ExtraFiles, cmd.Env)
				}
				if slot, free, err := tryHostFile(filepath.Join(directory, "slot-00")); err != nil || free {
					if slot != nil {
						_ = releaseHostProbe(slot)
					}
					t.Fatalf("lane's slot was free: free=%t err=%v", free, err)
				}
			} else {
				want := "asked by a person, runs beside"
				if row == "unreadable claim" {
					want = "could not be read ("
				}
				if !strings.Contains(log.String(), want) || len(cmd.ExtraFiles) != 0 || len(cmd.Env) != 1 {
					t.Fatalf("check did not continue without a slot: log=%q files=%v env=%v", log.String(), cmd.ExtraFiles, cmd.Env)
				}
			}
			if markers, _ := filepath.Glob(filepath.Join(directory, "wait-*")); len(markers) != 0 {
				t.Fatalf("room returned with wait markers: %v", markers)
			}
		})
	}
}
