package main

import "runtime"

// The engine replaces itself with syscall.Exec in several roles: proof-run
// custody-exec hands its pid to the command its custodian bound, the runtime
// hook execs its tool gate, a landed re-arm re-execs the newer engine, and the
// fake host execs its hold. Each does so from the main goroutine. On Linux an
// exec from any thread but the thread-group leader first kills the other
// threads and leaves the leader exiting, then zombie, until the execing
// thread takes over the pid; for that window /proc/<pid>/stat reads as an
// ended process while the process lives on. The resource custodian polls
// exactly that stat for its bound worker, took the window for completion and
// killed the command (`go list`: "signal: killed" beside "native descendants
// survived direct worker completion"). Locking the main goroutine to the main
// thread from init, which runtime.main already runs on that thread, makes
// every such exec happen on the leader: the kernel keeps it running through
// the exec and never shows the window.
func init() {
	runtime.LockOSThread()
}
