package steward

import (
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
)

// bridgeRole is the host board's bridge as a role of the steward that wins
// the board's flock (batch-lane design D14-r2, R25): every runner cycle a
// steward that does not hold ~/.metasystem/host/board/.bridge.flock tries it
// without blocking; the one that wins binds the bridge's socket under the
// flock and runs the bridge in its own process until it exits. The kernel
// releases the flock when the holder dies, so another steward wins it at its
// next cycle. The role reports and decides nothing, and keeps running at the
// helm.
type bridgeRole struct {
	home string
	// seats are the armed seats of this host the bridge reads.
	seats func() ([]board.Seat, error)
	// settings are the board settings and the stall bound.
	settings func() (config.Board, time.Duration)
	now      func() time.Time
	watch    func(dir string, poll time.Duration) *board.Watcher
	ticks    func(time.Duration) (<-chan time.Time, func())

	held       *lock.FileLock
	listener   net.Listener
	watcher    *board.Watcher
	stopTicker func()
	stop       chan struct{}
	done       chan error
}

// newBridgeRole is the production role of the steward guarding top.
func newBridgeRole(top string) *bridgeRole {
	home, err := board.Home()
	if err != nil {
		home = ""
	}
	return &bridgeRole{
		home:  home,
		seats: hostSeats,
		settings: func() (config.Board, time.Duration) {
			conf := filepath.Join(top, "metasystem.conf")
			settings, err := config.ResolveBoard(conf)
			if err != nil {
				settings = config.DefaultBoard()
			}
			stall := config.DefaultPipelineSettings().Stall
			if landing, err := (config.BatchLanding{}).WithPipeline(conf); err == nil {
				stall = landing.Pipeline.Stall
			}
			return settings, stall
		},
		now:   func() time.Time { return time.Now().UTC() },
		watch: board.Watch,
		ticks: func(every time.Duration) (<-chan time.Time, func()) {
			ticker := time.NewTicker(every)
			return ticker.C, ticker.Stop
		},
	}
}

// hostSeats are the armed checkouts of the host registry that still exist,
// each named by its enrolled nickname; a checkout without one is no seat.
func hostSeats() ([]board.Seat, error) {
	path, err := registry.DefaultPath()
	if err != nil {
		return nil, err
	}
	checkouts, err := registry.ArmedCheckouts(path)
	if err != nil {
		return nil, err
	}
	var seats []board.Seat
	for _, checkout := range checkouts {
		if _, err := os.Stat(checkout); errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if machine, err := goal.ResolveMachine(checkout); err == nil && machine != "" {
			seats = append(seats, board.Seat{Machine: machine, Installation: checkout})
		}
	}
	return seats, nil
}

// Step is the role's one act per runner cycle, and its report line.
func (r *bridgeRole) Step() string {
	if r.home == "" {
		return "bridge not serving: no home for the host board"
	}
	if r.held != nil {
		select {
		case err := <-r.done:
			// The bridge ended by itself: give the flock back and try again
			// below, as any steward would at its next cycle.
			r.done = nil
			r.release()
			if err != nil {
				return "bridge ended: " + err.Error() + "; it is tried again next cycle"
			}
		default:
			return "bridge serving at " + board.SocketPath(r.home) + fmt.Sprintf(" (this steward, pid %d)", os.Getpid())
		}
	}
	if _, err := board.EnsureBoard(r.home); err != nil {
		return "bridge not serving: " + err.Error()
	}
	held, err := lock.File(board.LockPath(r.home), 0o600, lock.TryExclusive)
	if err != nil {
		if lock.Busy(err) {
			return "bridge held by " + holderOf(board.LockPath(r.home))
		}
		return "bridge not serving: " + err.Error()
	}
	// The holder's pid, written into the lock file it keeps open: the file
	// is never removed, so its inode stays the same across failovers.
	file := held.File()
	if err := file.Truncate(0); err == nil {
		_, _ = file.WriteAt([]byte(strconv.Itoa(os.Getpid())+"\n"), 0)
	}
	listener, err := board.Listen(r.home)
	if err != nil {
		held.Release()
		return "bridge not serving: " + err.Error() + "; the flock is released and the role tries again next cycle"
	}
	settings, stall := r.settings()
	r.held, r.listener = held, listener
	r.watcher = r.watch(board.Dir(r.home), settings.Poll)
	ticks, stopTicker := r.ticks(board.DefaultHeartbeat)
	r.stopTicker = stopTicker
	r.stop, r.done = make(chan struct{}), make(chan error, 1)
	bridge := &board.Bridge{Home: r.home, Seats: r.seats, Stall: stall, Keep: settings.Keep, Now: r.now}
	go func(stop chan struct{}, done chan error, events <-chan struct{}) {
		done <- bridge.Run(stop, listener, events, ticks)
	}(r.stop, r.done, r.watcher.Events())
	return "bridge serving at " + board.SocketPath(r.home) + fmt.Sprintf(" (this steward, pid %d)", os.Getpid())
}

// holderOf names the flock's holder from the pid it wrote.
func holderOf(path string) string {
	data, err := os.ReadFile(path)
	if pid := strings.TrimSpace(string(data)); err == nil && pid != "" {
		return "pid " + pid
	}
	return "another steward"
}

// Close is the holder's clean exit: the bridge stops, its listener closes
// and removes the socket it created, and the flock is released.
func (r *bridgeRole) Close() {
	if r.held == nil {
		return
	}
	if r.done != nil {
		close(r.stop)
		<-r.done
		r.done = nil
	}
	r.release()
}

func (r *bridgeRole) release() {
	if r.watcher != nil {
		r.watcher.Close()
		r.watcher = nil
	}
	if r.stopTicker != nil {
		r.stopTicker()
		r.stopTicker = nil
	}
	if r.held != nil {
		r.held.Release()
		r.held = nil
	}
	r.listener = nil
}
