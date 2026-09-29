package identity

import "errors"

// ErrNoSuchProcess reports that a per-process kernel read found no process:
// it exited (or was reaped) between the snapshot that named it and the read.
// A caller sampling a live tree treats it as a member that left, never as a
// reader failure.
var ErrNoSuchProcess = errors.New("identity: no such process")
