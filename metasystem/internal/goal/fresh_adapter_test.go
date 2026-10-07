package goal

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

func TestFreshProjectionCancelsGitAndCleansActualRef(t *testing.T) {
	t.Parallel()
	rootRecord := bedRoot()
	rootRecord.SyncMode = SyncRemote
	root := validatedBed(t, "2026-08-23T09:00:00Z", rootRecord, map[string][]byte{"alpha.md": RenderFile(bedGoal("alpha"))})
	mustGit(t, root, "update-ref", AcceptedRef, "HEAD")
	before := metasystemRefs(t, root)
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	ready, blocked := filepath.Join(bin, "ready"), filepath.Join(bin, "blocked")
	deletions := filepath.Join(bin, "deletions")
	for _, path := range []string{ready, blocked} {
		if err := syscall.Mkfifo(path, 0600); err != nil {
			t.Fatal(err)
		}
	}
	// The adapter writes a real Git ref, announces its pid through a FIFO and
	// waits for cancellation. All other operations use the actual Git binary.
	script := fmt.Sprintf(`#!/bin/sh
for arg do
 if [ "$arg" = fetch ]; then
  for ref do :; done
  ref=${ref##*:}
  '%s' -C '%s' update-ref "$ref" HEAD || exit 91
  txn=$(printf '%%s' "$ref" | sed 's@/fetch/@/txn/@')
  '%s' -C '%s' update-ref "$txn" HEAD || exit 92
  printf '%%s' "$$" > '%s'
  exec /bin/cat '%s'
 fi
 if [ "$arg" = update-ref ]; then
  printf '%%s\n' "$*" >> '%s'
 fi
done
exec '%s' "$@"
`, git, root, git, root, ready, blocked, deletions, git)
	if err := testexec.WriteFile(filepath.Join(bin, "git"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	readyFile, err := os.OpenFile(ready, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer readyFile.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	e := Endpoint{Root: root, Remote: "origin", Branch: "refs/heads/main", commandEnv: []string{"PATH=" + bin + ":" + os.Getenv("PATH")}}
	type answer struct {
		projection  Projection
		observation Observation
		err         error
	}
	done := make(chan answer, 1)
	go func() {
		now := time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)
		p, observation, err := freshProjection(ctx, e, func() (time.Time, error) { return time.Date(2026, 8, 23, 9, 0, 0, 0, time.UTC), nil }, freshTestTiming(&now))
		done <- answer{p, observation, err}
	}()
	readyPID := make(chan string, 1)
	go func() { data := make([]byte, 32); n, _ := readyFile.Read(data); readyPID <- string(data[:n]) }()
	var data string
	select {
	case data = <-readyPID:
	case result := <-done:
		t.Fatalf("transport did not reach the handshake: %+v", result)
	}
	var pid int
	if _, err := fmt.Sscan(data, &pid); err != nil || pid <= 0 {
		t.Fatalf("adapter pid=%q: %v", data, err)
	}
	if refs := metasystemRefs(t, root); !strings.Contains(refs, "refs/metasystem/goals/fetch/read-") || !strings.Contains(refs, "refs/metasystem/goals/txn/read-") {
		t.Fatalf("adapter did not create its actual ref: %s", refs)
	}
	cancel()
	result := <-done
	if result.err == nil || result.observation.Outcome != "unavailable" || result.projection.Tree != nil || !strings.Contains(result.err.Error(), "canceled") {
		t.Fatalf("cancelled fetch became fresh: %+v", result)
	}
	if err := syscall.Kill(pid, 0); err != syscall.ESRCH {
		t.Fatalf("transport process %d survived cancellation: %v", pid, err)
	}
	if after := metasystemRefs(t, root); after != before {
		t.Fatalf("temporary ref survived or accepted tip changed: before=%s after=%s", before, after)
	}
	dataBytes, err := os.ReadFile(deletions)
	if err != nil || strings.Count(string(dataBytes), "update-ref") != 1 || !strings.Contains(string(dataBytes), "update-ref --stdin") {
		t.Fatalf("cleanup must delete both refs in one process: %q %v", dataBytes, err)
	}
}
