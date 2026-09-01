Working Mode: implement
Orchestrator Identity: m3 (lineage mac-m3, dispatch delegate under goal delegate-wait-primitive)
Date: 2026-09-01

# Goal

An engine verb, `metasystem job wait --job <id> [--repo <root>]`, in
Go: blocks until the job's record reaches a terminal status, always
terminates, always reports what happened. Wido's word binds the form:
"hard deterministic machinery... Go territory enforcing your
behaviour." Two seats in two days waited on artifacts a dead worker
never writes (records/misc/idle-loss-2026-08-31.md,
idle-loss-2026-09-01.md); this verb is the one lawful way to wait.

# Workspace

The dispatch-created job worktree, branched from main. Expected
touched paths (declare all, WITH the metasystem/ prefix): a new file
in internal/dispatch (wait.go + wait_test.go), the verb wiring in
cmd/metasystem (follow how existing job verbs register), and
optionally a thin scripts/agents/wait-job.sh shim relaying to the
engine per the kill-shell pattern (see scripts/receipt.sh for the shim
idiom). Nothing else.

# Mechanical rules — no judgment calls

1. Input: --job (validated by the same id shape the dispatch verbs
   use), --repo defaulting like sibling verbs. Record path:
   artifacts/agents/jobs/<id>.json.
2. TERMINAL statuses: the engine's existing terminalStatuses map is
   the single source of truth — reference it directly, never a copied
   list.
3. Poll cadence 5 seconds. Patience per the R-35 law, progress-based:
   track the record's bytes (hash or mtime+size); unchanged for 15
   minutes while non-terminal → exit 3 after printing the last-seen
   status. Overall hard bound 2 hours → exit 4, same printing. Both
   bounds and the cadence injectable (flags or config) so tests run in
   milliseconds — injectability is for tests, the defaults are the
   law.
4. Missing record: wait up to 60 seconds (injectable) for it to
   appear, then exit 5.
5. On terminal, print exactly one line to stdout:
   `job=<id> status=<status> failureReason=<reason-or-none>`
   Exit 0 completed, 1 failed, 2 any other terminal.
6. Read-only: the verb writes no state anywhere.

TESTS (Go, in wait_test.go, no process visibility needed — records are
plain files in a temp dir): (a) record flips to completed mid-wait →
exit 0 semantics and the exact line; (b) record already failed with a
failureReason → immediate failed result carrying it; (c) no record →
no-record result after the grace; (d) unchanged non-terminal record →
stalled at the (shrunk) bound; (e) hard bound fires when the record
keeps changing but never terminates. Follow the cmd/metasystem verb
test idiom for the CLI wiring.

# Constraints

- Go only for the machinery; the optional shim contains zero logic.
- gofmt/vet clean; no test weakened. Wall-clock budget: 35 minutes.

# Expected Return

Version-2 implementer JSON; complete prefixed diffBoundary; evidence
includes `go test ./internal/dispatch/ -run Wait -count=1` (must be
green in your sandbox — plain files only) and `go build ./...`.

# Gap Rule

stop and report a gap; never fill it silently.
