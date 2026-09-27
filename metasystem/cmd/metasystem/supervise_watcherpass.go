package main

import (
	"fmt"
	"io"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
)

// superviseWatcherPass is one census pass as a standalone writer, with this
// process as the writer: it claims the census-writer lock, heartbeats,
// publishes the verdict, and writes CENSUS-SLOW warnings to warn.
// A capMin below one attests the interval as the loaded cap.
func superviseWatcherPass(root, scope, supervisionDir, heartbeat, tag string, interval, capMin int, warn io.Writer) error {
	if capMin < 1 {
		capMin = interval
	}
	censusScope := scope
	if censusScope == "" {
		censusScope = root
	}

	self := identity.Ref{Pid: int64(os.Getpid())}
	if exact, state, err := (identity.KernelProber{}).Probe(self.Pid); err == nil && state == identity.Alive {
		self.StartedAtSec = exact.StartedAt.Unix()
	}
	lock := &supervise.CensusWriterLock{Dir: supervisionDir, Self: self, Tag: tag, Prober: identity.KernelProber{}}
	if err := lock.Claim(); err != nil {
		return err
	}
	defer lock.Release()

	_ = supervise.WriteHeartbeat(heartbeat, "watcher", self, tag, interval, capMin)

	cfg := watcherConfig(root, root, censusScope, supervisionDir, interval)
	cfg.Warn = func(message string) { fmt.Fprintln(warn, message) }
	if err := cfg.WatcherPass(); err != nil {
		return fmt.Errorf("supervise watcher-pass: %w", err)
	}
	_ = supervise.WriteHeartbeat(heartbeat, "watcher", self, tag, interval, capMin)
	return nil
}
