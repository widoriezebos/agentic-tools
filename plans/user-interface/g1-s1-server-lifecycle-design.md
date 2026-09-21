# g1-s1 Server lifecycle

- Gate 1, state `design-critiqued` pending round 2, author Claude with the human, 2026-09-21. Revision 2, after [critique round 1](g1-s1-server-lifecycle-design-critique.md).
- Refines the master at commit `94181f05a`: [Component responsibilities](../user-interface-design.md#component-responsibilities), [Scope and request principals](../user-interface-design.md#scope-and-request-principals), the browser protections in [Human acts from the browser](../user-interface-design.md#human-acts-from-the-browser), and [Trustworthy state and interaction](../user-interface-design.md#trustworthy-state-and-interaction).
- Carries decisions D1, D2, D8, D9, D10, and D11 from the [implementation plan](../user-interface-implementation-plan.md).
- Contributes to UID-R1-13 and to acceptance scenario 4 (the interface is usable with the engine stopped). It discharges neither alone.

All paths below are relative to `metasystem/` unless they start with `plans/`. Code references were verified by reading the source at commit `94181f05a`; nothing was executed.

## Outcome

A human runs `metasystem ui start` in a checkout and is told the address of a loopback HTTP server, which keeps running after the command returns. `metasystem ui status` says whether it runs and whether the executable on disk has changed since. `metasystem ui stop` ends it, and only it. `metasystem ui restart` replaces it with the executable now on disk, at the same address. The engine never signals it. The server answers a health route and refuses everything a foreign page or a non-loopback caller could send. It shows no views yet.

## Scope and non-goals

In scope: the `ui` command family (`start`, `serve`, `status`, `stop`, `restart`); the process lifecycle (detached launch, single instance, record, identity-checked stop); the HTTP listener with its request checks and response headers; the `ui.listen` configuration key; the documentation rows.

Not in scope: any view, template, or frontend bundle (`g1-s8`); any read model or API route (`g1-s2` to `g1-s7`); sign-in (gate 3); any mutation; making the engine account for this process (`g1-s17`, the next slice); remote access and TLS; a forced kill by `stop`; machine-readable `status` output.

## Existing code this builds on

| What | Where | Use |
| --- | --- | --- |
| Command table: `verb`, `family`, `families()` | `cmd/metasystem/main.go:17`, `:23`, `:29` | Add one `ui` family entry. Help text comes with it; `usage()` needs no edit |
| Internal verbs carry "(internal)" in their summary | `cmd/metasystem/main.go:50` | `ui serve` follows it |
| `upMetasystemRoot`, `upRepositoryScope`, `canonicalPath` | `cmd/metasystem/up.go` | Same package; call them directly to resolve the installation and the checkout |
| Steward runner: state under `artifacts/agents/steward`, exclusive non-blocking `flock`, identity record written atomically and removed on exit, `identity.KernelProber{}` probing its own pid | `internal/steward/runner.go:32` to `:149` | The pattern for state, single instance, and record |
| Stop fence check and creation claim in the same runner (`readOpenFence`, `stopfence.Creating`) | `internal/steward/runner.go` | The part **not** to copy; see Behaviour |
| `identity.Ref`, `Exact.Ref()`, `Prober` | `internal/identity/identity.go:25`, `:56`, `:177` | Recorded and live process identity; tests inject a fake `Prober` |
| `identity.AliveRef`; `identity.SignalExact`, `ErrGone`, `ErrUninspectable`, `SignalFunc`; `EncodeRef`, `ParseRef` | `identity.go:193`; `ref.go:132`, `:121`, `:129`; `ref.go:19`, `:40` | Status and stop. `SignalExact` proves the identity again immediately before signalling |
| `atomicfile.WriteText(path, text, anchor)` | `internal/atomicfile/atomicfile.go:76` | Publish the record; the anchor is the checkout |
| `config.Get(GetParams)`: flag, environment, `.local`, committed, default | `internal/config/resolve.go:112`, `:140` | Resolve `ui.listen`. Never read `metasystem.conf.local` directly; it holds secrets |
| Key constant, default constant, typed function in one small file | `internal/config/attention.go` | File shape only. It resolves through a different function; `ui.go` uses `config.Get` |
| `supervise.BuildStamp` | `internal/supervise/disk.go:144` | Shown for information. Read in `cmd/metasystem/ui.go` and passed down, so `internal/ui` does not import `supervise` |

## Contracts

### Commands

Exit codes: 0 success, 1 refusal or operational failure, 2 usage. Every verb accepts `--repo <path>` (default: the current directory, resolved with `upRepositoryScope`) and `--metasystem-root <path>` (default: derived by `upMetasystemRoot`). `start`, `serve`, and `restart` accept `--listen <host:port>`; `stop` and `restart` accept `--wait-seconds <s>`, default 15. `serve` also accepts `--ready-fd <n>`.

| Verb | Behaviour |
| --- | --- |
| `ui start` | Launches `ui serve` detached and waits for its readiness line |
| `ui serve` | The server in the foreground. Its summary ends "(internal)", but it is safe to run by hand for debugging: logs on standard error, Ctrl-C to stop |
| `ui status` | Reports the state. Exit 0 only when `running` |
| `ui stop` | Identity-checked SIGTERM, then waits for exit |
| `ui restart` | `stop`, then `start`. The address is resolved exactly as `start` resolves it, so it is the same address while the configuration is unchanged |

Every outcome has one line and one exit code. `<address>` is `host:port`; `<record>` is the path of `server.json`.

| Verb | Outcome | Line | Exit |
| --- | --- | --- | --- |
| `start` | ready | `interface running at http://<address> (pid <pid>)` | 0 |
| `start` | already running, record readable | `an interface server already runs for this checkout at http://<address>; stop it with: metasystem ui stop` | 1 |
| `start` | lock held, no readable record | `an interface server already runs for this checkout (address unknown: <record> is missing or unreadable)` | 1 |
| `start` | child reported `failed <message>` | `<message>` | 1 |
| `start` | no readiness line in time, or the pipe closed without one | `the interface server did not become ready; see <log path>` | 1 |
| `status` | `running` | `interface running at http://<address> (pid <pid>, started <RFC 3339>, build <stamp>)` | 0 |
| `status` | `running`, executable changed | the line above, then `the executable on disk differs from the one the interface is running; to pick it up: metasystem ui restart` | 0 |
| `status`, `stop` | `stopped` | `interface not running` | `status` 1, `stop` 0 |
| `status`, `stop` | `stale` | `interface not running (stale record from pid <pid> removed)` | `status` 1, `stop` 0 |
| `status`, `stop` | `uninspectable` | `cannot prove pid <pid> is the interface server; nothing was changed` | 1 |
| `status`, `stop` | `unreadable` | `a process holds the interface lock but <record> cannot be read; nothing was changed` | 1 |
| `status`, `stop` | `busy` | `the interface is starting or stopping; try again` | 1 |
| `stop` | `stopped-now` | `interface stopped` | 0 |
| `stop` | `timeout` | `interface (pid <pid>) did not stop within <s>s; it was sent SIGTERM and left running` | 1 |
| `restart` | stop outcome `stopped`, `stale`, or `stopped-now` | the stop line unless `stopped`, then the `start` outcome | the `start` exit |
| `restart` | any other stop outcome | the stop line; nothing is started | 1 |

### Packages

`internal/ui/lifecycle` owns the process: listen-address validation, the state directory, the lock, the record, launch, serve, status, stop, and restart. `internal/ui/httpd` owns the handler: request checks, response headers, and the two routes. Later slices add packages beside them under `internal/ui/`. `cmd/metasystem/ui.go` parses flags, resolves roots and configuration, wires the real spawn, prober, and signal handling into `lifecycle`, and prints the lines above. It makes no decision.

```go
// package lifecycle
func Dir(checkout string) string                 // <checkout>/artifacts/agents/ui
func ValidateListen(addr string) (string, error) // normalized host:port

type Record struct {
    SchemaVersion    int    `json:"schemaVersion"`    // 1; any other value is unreadable
    Process          string `json:"process"`          // identity.EncodeRef
    Address          string `json:"address"`          // bound host:port
    Checkout         string `json:"checkout"`
    StartedAt        string `json:"startedAt"`        // RFC 3339, UTC
    EngineBuild      string `json:"engineBuild"`      // information only
    ExecutableDigest string `json:"executableDigest"` // "sha256:<hex>" of the serving executable
}

type Options struct {
    Checkout         string
    Listen           string
    EngineBuild      string
    ExecutableDigest func() (string, error)            // nil: hash os.Executable()
    NewHandler       func(bound net.Addr, rec Record) http.Handler
    Prober           identity.Prober
    Ready            func(address string)               // once, after the record is published
    LockWait         time.Duration                      // default 5s
    ShutdownGrace    time.Duration                      // default 10s
    Now              func() time.Time
}
func Serve(ctx context.Context, o Options) error        // nil on a clean shutdown
type AlreadyRunningError struct{ Address string }       // Address "" when unknown

type State string // "running" "stopped" "stale" "uninspectable" "unreadable" "busy"
type Status struct {
    State  State
    Record *Record // set for running, stale, uninspectable
}
func Read(checkout string, prober identity.Prober) (Status, error)
func ExecutableChanged(rec Record, currentDigest string) bool // false when either is empty

type StopOutcome string // the six states above, plus "stopped-now" and "timeout"
type StopOptions struct {
    Prober   identity.Prober
    Send     identity.SignalFunc             // nil: the real signal
    WaitExit func(ctx context.Context) error // nil: wait on the lock
    Wait     time.Duration
}
func Stop(checkout string, o StopOptions) (StopOutcome, *Record, error)
func Restart(checkout string, o StopOptions, start func() error) (StopOutcome, *Record, error)

type LaunchSpec struct {
    Executable string
    Args       []string
    Dir        string // the checkout
    LogPath    string
}
type Child interface {
    ReadyLine(wait time.Duration) (string, error) // ErrReadyTimeout, io.EOF when closed without a full line
    Pid() int
    Kill() error
    Release() error
}
type Spawn func(LaunchSpec) (Child, error)
func Launch(spec LaunchSpec, spawn Spawn, readyWait time.Duration) (address string, pid int, err error)
func ExecSpawn(spec LaunchSpec) (Child, error) // the real one
```

```go
// package httpd
type Info struct{ Checkout, StartedAt, EngineBuild, ExecutableDigest string }
func New(info Info, bound net.Addr) http.Handler
```

### State on disk

`<checkout>/artifacts/agents/ui/` holds `server.flock`, `server.json`, `server.log`, and `server.log.1`. The directory is ignored by Git. No source or configuration lives there.

**Record rule.** `server.json` is created, replaced, or removed only by a process that holds the exclusive lock on `server.flock`. Everything below follows from it.

### Configuration

`internal/config/ui.go` adds `UIListenKey = "ui.listen"`, `DefaultUIListen = "127.0.0.1:7878"`, and `UIListen(confPath, flag string, flagSet bool) (string, error)`, which resolves the value through `config.Get` and returns it unvalidated. The loopback rule lives in one place, `lifecycle.ValidateListen`, and `cmd/metasystem/ui.go` applies it; `config` does not import `internal/ui`. `metasystem.conf` gains one commented line, `# ui.listen=127.0.0.1:7878`, with a sentence saying a machine sets its own port in the `.local` file.

`ValidateListen` accepts only a loopback IP literal (`net.ParseIP(host).IsLoopback()`) with a numeric port from 0 to 65535. Port 0 asks the kernel for a free port; the record and every printed address carry the bound port. An empty host, a non-loopback IP, and every host name, `localhost` included, are refused without a DNS lookup. The refusal for a name says to use `127.0.0.1`; the others say `remote browser access is not supported yet`.

### HTTP

The allowed Host set is computed from the bound address: the bound IP literal with the port (`127.0.0.1:<port>`, or `[::1]:<port>` for an IPv6 bind) and `localhost:<port>`. When the port is 80 the bare forms without a port are added. Host names compare without regard to case.

Checks run in this order. A refusal is one plain-text line that never echoes the offending value.

1. Method other than GET or HEAD: 405 with `Allow: GET, HEAD`. Gate 1 cannot mutate by construction.
2. `Host` not in the allowed set: 403. This defeats DNS rebinding.
3. `Sec-Fetch-Site` present and neither `same-origin` nor `none`: 403, **unless** the method is GET and the request is a top-level navigation, `Sec-Fetch-Mode: navigate` with `Sec-Fetch-Dest: document`. Following a link from another page is legitimate; the originating page cannot read the response, and framing is denied by the headers below.
4. `Origin` present and not `http://` followed by an allowed host: 403.

Every response, refusals included, carries `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy: same-origin`, `Cross-Origin-Resource-Policy: same-origin`, and `Cache-Control: no-store`. No `Access-Control-*` header is ever sent. The content security policy belongs to `g1-s8`, which knows the bundle.

Routes: `GET /-/health` returns 200 and `{"status":"ok","checkout":…,"startedAt":…,"engineBuild":…,"executableDigest":…}`. `GET /` returns 200 plain text, `MetaSystem interface: no views are installed in this build yet.` Anything else is 404. HEAD is answered by `net/http` for both.

Server timeouts: `ReadHeaderTimeout` 10s, `ReadTimeout` 30s, `IdleTimeout` 120s. `WriteTimeout` stays zero, because `g1-s7` adds a long-lived event stream.

## Behaviour

**Launch** (`lifecycle.Launch`, used by `start` and `restart`). Call `spawn`, then `ReadyLine(readyWait)`, default 10 seconds. The child writes exactly one newline-terminated line to the ready descriptor and closes it: `ready <host:port>` or `failed <message>`. `ready`: `Release` the child and return the address and pid. `failed`: return the message. `ErrReadyTimeout`, `io.EOF`, or any other line: `Kill` the child, which is still the launcher's own never-ready child, and return the not-ready error naming the log. A line cut off before its newline counts as `io.EOF`.

`ExecSpawn` runs the executable with the arguments plus `--ready-fd 3`, a new session (`Setsid`), standard input from the null device, standard output and error appended to `LogPath`, the checkout as working directory, and the write end of a fresh pipe as the first extra file. After the child starts, the parent **closes its own copy of the write end**; otherwise the read end never reaches end of file. `ReadyLine` sets a read deadline on the read end, which an `os.Pipe` supports. The parent never rotates or truncates the log.

**Serve.**
1. Validate the address; create the state directory.
2. Fast refusal: if the record parses and `identity.AliveRef` says alive, fail with `AlreadyRunningError` and its address.
3. Take the exclusive lock, blocking, for at most `LockWait`. A status or stop command holds it only for an instant, and a server that is shutting down releases it. On timeout fail with `AlreadyRunningError`, address empty unless a record is readable.
4. Holding the lock: remove any leftover record. If `server.log` exceeds 1 MiB, copy it to `server.log.1` and truncate it in place; the process's own append-mode descriptors remain valid.
5. Probe this process's own identity, which must be alive, and compute the executable digest.
6. Listen. A port in use fails with a message naming `--listen` and `ui.listen`.
7. Build the handler, publish the record atomically, call `Ready`.
8. Serve until the context ends; `cmd` ends it on SIGTERM or SIGINT. Shut down gracefully within `ShutdownGrace`, then close what remains.
9. Remove the record, then release the lock, in that order.

When `serve` was launched with `--ready-fd`, `cmd` writes the ready or failed line there and closes it. A write error is ignored: the launcher may have died, and the server is up regardless and visible to `ui status`. Go does not raise SIGPIPE for that descriptor.

**Read.** Read `server.json`. If it parses and its schema version is 1: alive is `running`; unknown is `uninspectable`, with no file touched. Otherwise, for a missing, dead, or unreadable record, probe the lock without blocking:

- Won: nobody serves. Re-read the record; if one is still there and still not alive, remove it. Release. The state is `stale` when a parseable dead record was removed, else `stopped`.
- Lost: somebody holds the lock. The state is `unreadable` when the record exists but does not parse or has another schema version, else `busy`. Nothing is touched.

**Stop.** Evaluate as `Read` does. `stopped`, `stale`, `uninspectable`, and `unreadable` are returned as they are. `busy`: wait for the lock up to `Wait`, release it, evaluate once more, and return whatever that gives. `running`: `identity.SignalExact` with SIGTERM. `ErrGone` is handled as a dead record. `ErrUninspectable` returns `uninspectable`; nothing was signalled. Sent: wait for exit by taking the blocking exclusive lock, bounded by `Wait`. Won: remove a record only if one remains and is not alive, release, return `stopped-now`. Timed out: prove the original identity once more; dead means `stopped-now`, because a new server took the lock during the wait and its record is not ours to touch; anything else is `timeout`. `stop` never sends SIGKILL.

**Restart.** `Stop`, then call `start` only when the outcome is `stopped`, `stale`, or `stopped-now`. It is performed by the executable that was invoked, so the new server is the executable on disk.

**Developing the server.** Rebuild with `scripts/agents/go-build.sh`, then `bin/metasystem ui restart`. `status` notices a rebuild even when the build stamp does not change, which is the normal case during development: every dirty tree is stamped `dev-<HEAD>-dirty` (`scripts/agents/go-build.sh:75`), so the comparison uses the executable's digest.

| Case | Required result |
| --- | --- |
| Two `start` commands at once | One server. The loser's child fails at step 2 or 3 and reports `failed`; its launcher prints that line and exits 1 |
| `status` while a server is between taking the lock and publishing its record | `busy`. The record of the new server is never removed by a reader, because a reader removes only after winning the lock |
| Server killed with SIGKILL | The kernel releases the lock. The next `status` or `stop` wins the probe and removes the record; the next `start` succeeds |
| Recorded pid reused by another process | `AliveRef` reports dead; never signalled |
| `server.json` truncated or of a future schema while a server runs | `unreadable`; nothing is signalled or removed. When nobody holds the lock, the file is removed under the lock |
| Port already in use | `failed` line naming `--listen` and `ui.listen` |
| Two checkouts on one machine | Independent: separate state directories, each with its own configured port |
| `restart` while the old server ignores SIGTERM | `timeout`, exit 1; no second server is started and the old one is left running |
| A link on another page opens the interface | Served, by the navigation exemption in check 3 |
| A script on another page fetches the health route | 403 by check 3 or 4, and unreadable to that page in any case |
| A request with no `Origin` and no `Sec-Fetch-Site` (a command-line client) | Served, subject to the method and Host checks |
| Engine stopped, stop fence closed | `ui start` succeeds. The stop fence (`internal/stopfence`) is **not** consulted and no creation claim is opened. The master requires the interface to work while the engine is stopped, and D9 requires independence from engine restarts |
| Engine `stop`, `up`, or `arm` while the interface is up | The interface is never signalled. It is not a run, job, steward, or supervised component; `stop` lists it as "not the metasystem's, not touched" (`internal/stoptransition/families.go:841` to `:900`) and records no survivor, so `arm` is unaffected. The janitor acts only on registry claims, which this process never opens |
| Engine running beside the interface, until `g1-s17` | **Known limitation, not fixed here.** The census classes the server UNTRACKED (`internal/census/run.go:208` to `:335`). The steward counts that (`internal/steward/census.go:74`) and, when no worker is alive, notifies instead of reviving (`verdict.go:130`); handoffs are held (`handoff.go:101`); the end-of-turn watchdog lists the process until it is acknowledged (`internal/supervise/watchdog.go:135`). The engine leaves the interface alone, but a running interface suppresses automatic revival in that checkout. `g1-s17` makes the engine account for the server and is the next slice |

The server writes no MAIN announcement, takes no lease, declares no brain, starts no engine, and claims no goal.

## Change boundary

New files: `cmd/metasystem/ui.go`; `internal/ui/lifecycle/` and `internal/ui/httpd/`, each with its sources, tests, and `testmain_test.go`; `internal/config/ui.go` and `internal/config/ui_test.go`.

Edited files, and nothing else in them: `cmd/metasystem/main.go` (one `ui` family entry in `families()`); `docs/architecture.md` (package-map rows for `ui/lifecycle` and `ui/httpd`, one row in the family table); `metasystem.conf` (the commented key and its sentence).

Must not be touched: `testing-parallel-ratchet.json`, `testing.json`, `internal/testenv`, `internal/testutil`, `internal/parallelratchet`, `internal/testselect`, `internal/testpolicy`, `internal/testexec`, `internal/hostload`, `internal/proofrun`, `cmd/metasystem/test*.go`, `cmd/metasystem/audit.go`, any existing `*_test.go` or `testmain_test.go`, and `internal/census`, `internal/stoptransition`, `internal/steward`, `internal/supervise`, `internal/identity`. Another agent is working in the test infrastructure, and the engine-side change belongs to `g1-s17`.

No new Go module dependency.

## Conventions the implementation must meet

- Each new package has `testmain_test.go` with `func TestMain(m *testing.M) { os.Exit(testenv.Main(m)) }`. Tests never call `os.Setenv`.
- Every top-level test and independent subtest calls `t.Parallel()`. A package absent from the parallel ratchet has a ceiling of zero serial tests, and the ratchet file may not be edited.
- No wall-clock sleeps and no polling loops in tests. Time, waiting, spawning, signalling, and hashing are injected (`Now`, `WaitExit`, `Spawn`, `Send`, `Prober`, `ExecutableDigest`); the context ends `Serve`.
- No fixed ports in tests: `127.0.0.1:0`. State under `t.TempDir()`. Assertions with `testutil.Expect` and `testutil.Require`.
- `cmd/metasystem/ui.go` contains routing, flag parsing, wiring, and printing only.

## Verification

Go tests to add:

- `httpd`: a table over method, Host (including port 80 bare forms, an IPv6 bind, and mixed case), `Sec-Fetch-Site` with and without the navigation exemption, and Origin, giving 405, 403, or 200; the five headers on success and on refusal; no `Access-Control-*` header; the health payload; refusals do not echo input.
- `lifecycle`, address: the `ValidateListen` table, including port 0, `localhost` refused, and no lookup for a name.
- `lifecycle`, serve: the record carries the bound port and is removed when the context ends; a leftover record is removed at start; a second `Serve` is refused by the fast path while the first runs; the lock wait expiring gives `AlreadyRunningError` with an empty address.
- `lifecycle`, read: every state, including that a reader who loses the lock probe removes nothing, and that a record replaced between the read and the probe is left alone.
- `lifecycle`, stop and restart: every `StopOutcome` with a fake prober and sender, including SIGTERM sent exactly once, the timeout that re-proves a dead identity as `stopped-now`, and `Restart` calling `start` only after `stopped`, `stale`, or `stopped-now`.
- `lifecycle`, launch, with a fake `Spawn`: `ready`, `failed`, timeout, end of file, a partial line, and an unknown line; the child is killed in the last four and released only on `ready`.
- `config`: default, flag over committed value, empty flag still wins when set.

Commands, from `metasystem/`: `go build ./...`; `go vet ./internal/ui/... ./internal/config/ ./cmd/metasystem/`; `go test ./internal/ui/... ./internal/config/`; and the `cmd/metasystem` tests that look families up (`TestUnitFamilyIsRegistered` and its neighbours in `launch_unit_test.go`, `goal_branch_test.go`, `launch_pack_test.go`). Run `bin/metasystem audit parallel-ratchet` without `--update` if it works, and note it if it does not.

Walkthrough with a real process, by Claude after the code critique: build `bin/metasystem`; `ui start` prints an address; `curl -i` on `/-/health` gives 200 with the five headers; a foreign `Host`, a foreign `Origin`, `Sec-Fetch-Site: cross-site` alone, and a POST give 403, 403, 403, and 405, while `cross-site` with the navigation headers gives 200; a real browser opens the address from a link on another page; a second `ui start` is refused with exit 1 and the running server's log is intact; `ui status` exits 0; rebuild, and `ui status` adds the changed-executable line; `ui restart` returns the same address and the line is gone; `ui stop` prints `interface stopped` and the record is gone; `ui status` exits 1; start again, `kill -9` the server, and `ui status` reports and removes the stale record. If an engine steward is active in the development checkout, keep the walkthrough short, because of the limitation above. The engine's own `stop` is not run there.

What stays outside the in-process tests is `ExecSpawn` alone: the real session, descriptor inheritance, and pipe. The walkthrough covers it.

## Open questions

None. Ruled by the human on 2026-09-21: no access token in gate 1 (D2); `127.0.0.1:7878` is the default address; `ui status` exits 1 when the interface is not running; restarting onto a new build must be easy (D10). `ui status --json` was removed under D11 until something needs it.
