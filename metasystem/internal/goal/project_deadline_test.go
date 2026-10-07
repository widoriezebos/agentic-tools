package goal

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/boundedexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

func TestProjectionArtificialDeadlineKillsFetchAndCleansRefs(t *testing.T) {
	t.Parallel()
	root, bin := t.TempDir(), t.TempDir()
	ready, blocked := filepath.Join(bin, "ready"), filepath.Join(bin, "blocked")
	for _, path := range []string{ready, blocked} {
		if err := syscall.Mkfifo(path, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	readyFile, err := os.OpenFile(ready, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer readyFile.Close()
	// PATH contains only this adapter. The fetch publishes temporary refs,
	// then holds at a FIFO until the artificial process deadline kills it.
	script := `#!/bin/sh
if [ "$5" = fetch ]; then
 ref=${9#*:}
 printf '%s' "$ref" > "$FIXTURE_ROOT/fetch-ref"
 printf '%s' "$ref" > "$FIXTURE_ROOT/txn-ref"
 printf '%s' "$$" > "$FIXTURE_READY"
 exec /bin/cat "$FIXTURE_BLOCKED"
fi
if [ "$5" = update-ref ] && [ "$6" = -d ]; then
 printf '%s\n' "$7" >> "$FIXTURE_ROOT/deleted"
 case "$7" in
  */fetch/*) /bin/rm "$FIXTURE_ROOT/fetch-ref" ;;
  */txn/*) /bin/rm "$FIXTURE_ROOT/txn-ref" ;;
  *) exit 92 ;;
 esac
 exit 0
fi
exit 93
`
	if err := testexec.WriteFile(filepath.Join(bin, "git"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "accepted"), []byte("accepted-tip"), 0o600); err != nil {
		t.Fatal(err)
	}
	endpoint := Endpoint{Root: root, Remote: "blocked", Branch: "refs/heads/main", commandEnv: []string{
		"PATH=" + bin, "FIXTURE_ROOT=" + root, "FIXTURE_READY=" + ready, "FIXTURE_BLOCKED=" + blocked,
	}}
	var outer, process atomic.Int32
	pid := 0
	deadline := func(wait time.Duration) <-chan time.Time {
		event := make(chan time.Time)
		switch wait {
		case defaultFreshProjectionTimeout:
			outer.Add(1)
		case defaultFreshFetchProcessTimeout:
			process.Add(1)
			data := make([]byte, 32)
			n, err := readyFile.Read(data)
			if err != nil {
				t.Errorf("fetch barrier: %v", err)
				close(event)
				return event
			}
			parsed, err := strconv.Atoi(string(data[:n]))
			if err != nil {
				t.Errorf("fetch pid=%q: %v", data[:n], err)
			}
			pid = parsed
			close(event)
		case 5 * time.Second: // The process reap's deadline also belongs to this fixture.
		default:
			t.Errorf("unexpected bound %s", wait)
		}
		return event
	}
	projection, err := ProjectWithDeadline(endpoint, true, time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC), deadline)
	if !errors.Is(err, boundedexec.ErrTimedOut) || projection.Tree != nil || !strings.Contains(err.Error(), "git fetch") {
		t.Fatalf("expired projection: %+v %v", projection, err)
	}
	if outer.Load() != 1 || process.Load() != 1 || pid <= 0 {
		t.Fatalf("injected deadlines: outer=%d process=%d pid=%d", outer.Load(), process.Load(), pid)
	}
	if err := syscall.Kill(pid, 0); err != syscall.ESRCH {
		t.Fatalf("fetch %d survived its deadline: %v", pid, err)
	}
	for _, name := range []string{"fetch-ref", "txn-ref"} {
		if _, err := os.Stat(filepath.Join(root, name)); !os.IsNotExist(err) {
			t.Fatalf("temporary %s remained: %v", name, err)
		}
	}
	deletions, err := os.ReadFile(filepath.Join(root, "deleted"))
	lines := strings.Fields(string(deletions))
	if err != nil || len(lines) != 2 || !strings.Contains(lines[0], "/fetch/read-") || lines[1] != strings.Replace(lines[0], "/fetch/", "/txn/", 1) {
		t.Fatalf("exact-operation cleanup: %q %v", deletions, err)
	}
	if accepted, err := os.ReadFile(filepath.Join(root, "accepted")); err != nil || string(accepted) != "accepted-tip" {
		t.Fatalf("accepted state changed: %q %v", accepted, err)
	}
	t.Logf("artificial deadline reaped fetch %d and removed both operation refs", pid)
}
