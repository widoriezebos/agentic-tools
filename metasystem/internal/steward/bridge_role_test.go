package steward

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
)

// testRole is one steward's bridge role over a fixture home, its watch a
// poll the test never ticks and its heartbeat a channel the test drives.
func testRole(home string, seat board.Seat, now time.Time) (*bridgeRole, chan time.Time) {
	ticks := make(chan time.Time)
	role := &bridgeRole{
		home:  home,
		seats: func() ([]board.Seat, error) { return []board.Seat{seat}, nil },
		settings: func() (config.Board, time.Duration) {
			return config.Board{Keep: 24 * time.Hour, Poll: time.Hour}, 20 * time.Minute
		},
		now:   func() time.Time { return now },
		watch: func(string, time.Duration) *board.Watcher { return board.PollWatch(nil) },
		ticks: func(time.Duration) (<-chan time.Time, func()) { return ticks, func() {} },
	}
	return role, ticks
}

// abandon ends the role as a holder killed without cleanup would: the
// listener closes without removing its pathname and the flock is released.
func (r *bridgeRole) abandon() {
	r.listener.(*net.UnixListener).SetUnlinkOnClose(false)
	close(r.stop)
	<-r.done
	r.watcher.Close()
	r.held.Release()
	r.held = nil
}

func inode(t *testing.T, path string) uint64 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Sys().(*syscall.Stat_t).Ino
}

func boardListing(t *testing.T, dir string) []string {
	t.Helper()
	var names []string
	filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err == nil && !entry.IsDir() {
			// The inode names each file's one version: an atomic rewrite
			// publishes a new one, so a write is seen even when its bytes
			// are the same.
			relative, _ := filepath.Rel(dir, path)
			info, statErr := os.Stat(path)
			if statErr != nil {
				return statErr
			}
			names = append(names, fmt.Sprintf("%s@%d", relative, info.Sys().(*syscall.Stat_t).Ino))
		}
		return err
	})
	return names
}

// subscribeRaw subscribes on the wire and returns the reader after the
// snapshot.
func subscribeRaw(t *testing.T, home string) (net.Conn, *bufio.Reader) {
	t.Helper()
	conn, err := board.Dial(home)
	if err != nil {
		t.Fatalf("dial the bridge: %v", err)
	}
	if _, err := io.WriteString(conn, `{"subscribe":{"kinds":["card"]}}`+"\n"); err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(conn)
	if line, err := reader.ReadString('\n'); err != nil || !strings.HasPrefix(line, `{"snapshot":`) {
		t.Fatalf("snapshot %q: %v", line, err)
	}
	return conn, reader
}

// TestOnlyTheStewardHoldingTheFlockRunsTheBridge (R25, U10c-1): two stewards'
// cycles against one fixture home: the first wins .bridge.flock and serves a
// real unix socket under a short temporary path; the second reports bridge
// held by pid N and serves nothing; when the first ends without cleanup (its
// listener closed, the pathname left behind) the second wins at its next
// cycle, removes the stale socket under the flock and binds; the lock
// file's inode is the same before and after the failover; the bridge writes
// nothing under the board but its sweep of a terminal card older than
// board.keep-hours; a regular file at the socket path is
// BRIDGE_SOCKET_PATH_OCCUPIED and the flock is released; a clean exit
// removes the socket.
func TestOnlyTheStewardHoldingTheFlockRunsTheBridge(t *testing.T) {
	t.Parallel()
	home, err := os.MkdirTemp("/tmp", "brr")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(home) })
	now := time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC)
	seat := board.Seat{Machine: "m1b", Installation: "/c/b/metasystem"}
	for _, card := range []board.Card{
		{Seat: seat, Goal: "old-landed", Stage: board.StageLanded, Writer: board.Writer{At: now.Add(-48 * time.Hour)}},
		{Seat: seat, Goal: "live", Stage: board.StageLandReady, Writer: board.Writer{At: now.Add(-time.Minute)}},
	} {
		if err := board.WriteAt(home, card); err != nil {
			t.Fatal(err)
		}
	}
	first, _ := testRole(home, seat, now)
	second, secondTicks := testRole(home, seat, now)
	if line := first.Step(); !strings.HasPrefix(line, "bridge serving at "+board.SocketPath(home)) {
		t.Fatalf("first steward: %q", line)
	}
	lockInode := inode(t, board.LockPath(home))
	if line := second.Step(); line != fmt.Sprintf("bridge held by pid %d", os.Getpid()) {
		t.Fatalf("second steward: %q", line)
	}
	conn, _ := subscribeRaw(t, home)
	conn.Close()
	if line := first.Step(); !strings.HasPrefix(line, "bridge serving") {
		t.Fatalf("the holder's next cycle: %q", line)
	}

	// A process this test binary forks for another parallel test holds a
	// copy of every descriptor between its fork and its exec, the listener
	// included, and a copy keeps an in-process listener accepting after its
	// close. The fixture holds such a copy on purpose, so the failover is
	// proved in exactly that state; the abandoned holder's own listener is
	// read directly instead of through a dial the copy would answer.
	copied, err := first.listener.(*net.UnixListener).File()
	if err != nil {
		t.Fatal(err)
	}
	defer copied.Close()
	abandoned := first.listener
	first.abandon()
	if board.BridgeState(home) != board.BridgeLive {
		t.Fatal("the abandoned pathname should still stand")
	}
	if _, err := abandoned.Accept(); !errors.Is(err, net.ErrClosed) {
		t.Fatalf("the abandoned holder's listener is closed: %v", err)
	}
	before := boardListing(t, board.Dir(home))
	if line := second.Step(); !strings.HasPrefix(line, "bridge serving at ") {
		t.Fatalf("the failover: %q", line)
	}
	if got := inode(t, board.LockPath(home)); got != lockInode {
		t.Fatalf("the lock file was recreated: inode %d, was %d", got, lockInode)
	}
	conn, reader := subscribeRaw(t, home)
	secondTicks <- now
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(line, `{"heartbeat":`) {
			break
		}
	}
	conn.Close()
	after := boardListing(t, board.Dir(home))
	if want := slices.DeleteFunc(slices.Clone(before), func(name string) bool { return strings.HasPrefix(name, "m1b/old-landed.json@") }); !slices.Equal(after, want) {
		t.Fatalf("the bridge wrote under the board:\nbefore %v\nafter  %v", before, after)
	}

	second.Close()
	if board.BridgeState(home) != board.BridgeAbsent {
		t.Fatal("a clean exit left the socket")
	}
	if err := os.WriteFile(board.SocketPath(home), []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	third, _ := testRole(home, seat, now)
	if line := third.Step(); !strings.Contains(line, "BRIDGE_SOCKET_PATH_OCCUPIED") {
		t.Fatalf("a regular file at the path: %q", line)
	}
	held, err := lock.File(board.LockPath(home), 0o600, lock.TryExclusive)
	if err != nil {
		t.Fatalf("the refused role kept the flock: %v", err)
	}
	held.Release()
}
