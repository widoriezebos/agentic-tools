# One writer, safe readers: sessions sharing a repository without interference

- Goal and current status: exactly one main session holds the write role per checkout, enforced mechanically; every other session is a first-class read-only advisor; a second session that wants to write gets a paved one-command path to its own worktree. Closes KI-21 as experienced and KI-22. Status: CONSOLIDATED REWRITE incorporating the MV-1 verification findings — one voice, no superseded text; awaiting verification round 2 on chain design-critic-20260807t094616z-ac0a.
- Next step: none
- In flight right now: nothing
- Waiting on: THE HUMAN's altitude ruling. Verification round 2 returned nine material (plans/mv2-findings-carried.md) against the clean rewrite — 74 findings total across three chains, and the last two rounds ran 9 and 9: at prose altitude against a large shipped codebase the finding surface is not converging. The orchestrator's recommendation: fold the nine, then IMPLEMENT — the remaining finding classes (adapter-owned transitions, lock semantics, matrix-versus-shipped-supervision) are exactly what the code-critic verifies against real code, as CC-1-6 already proved the loop can. The alternative is verification round 3 (the chain's last budgeted round) before implementing. Devin stays PARKED per the same day's decision.

## The evidence this stands on

On 2026-08-07 the human ran a second Claude session in this checkout. The
census and mains registry tracked both (observation held); enforcement did
not exist. The peer's transcript proves the turn-end hook commanded it to
start streams the first session was running, that both sessions shared one
git index, and that six critique rounds carried a detected-then-ignored
identity error (KI-22). Two chains critiqued this design: 47 findings
folded over six rounds, then a fresh verification chain returned nine more
(MV-1, all incorporated here). The settled decisions — the rescope to one
writer, death-only takeover, humans never gated, named residuals — carry
their dispositions under plans/dispositions/ and are not relitigated.

## Design

### D-1: identity

The start hook announces each main once, idempotently, keyed by (pid,
start time): a duplicate start event returns the existing record. The
announcement gains two fields (schema of the strict reader updated
accordingly): `mainId` — `main-<pidStartedAt>-<pid>-<rand6>`, minted at
first announce, naming one process lifetime, no cross-restart continuity —
and `commandHash`, the sha256 of the process's full command line as read
from the process table at announce (MV-1-5).

Identity comparisons use two rules, each stated where it applies:
- AUTHENTICATION (may a caller act?) matches pid, start time, AND a fresh
  process-table read of the command line against `commandHash` — all three
  together, re-read at every call. A same-second pid recycle must also
  reproduce the command line within one read; any mismatch refuses. That
  triple within one read is the accepted residual, and its failure
  direction is refusal.
- LIVENESS (is the holder dead?) matches pid and start time only; a false
  "alive" merely delays takeover, which is the safe direction.

### D-2: caller classification

One ordered walk over kernel facts; no environment markers exist anywhere
in this design (a hook subprocess cannot set its parent's environment).
First, the caller's own triple against the announcements — a match IS that
main. Otherwise walk parent-by-parent toward init: at each ancestor, test
the triple against announcements (match: authenticated as that main), then
the command line against the adapter signature registry (match: delegate
context — refused for every holder-gated operation). Reaching init with no
match classifies the caller as the human's own tools: passed untouched,
everywhere, always. Fail-open to humans is the stated, deliberate cost;
this is cooperative discipline against accident, not a defense against a
lying agent, matching the repository's declared trust model.

### D-3: the checkout lease

`artifacts/agents/mains/worktree-lease.json` carries holder mainId, pid,
pidStartedAt, commandHash, claimedAt, renewedAt, generation, and takeover
history. All mutations — claim, renew, takeover — run under a dedicated
flock on a lock sibling: take the lock, re-read, validate expected
generation, decide, write via temp-and-rename with generation incremented,
release. Renewal on turn boundaries is hygiene, not survival: the ONLY
takeover predicate is provable holder death by the liveness rule.
Permission-denied on the pid counts as alive. A live holder can never lose
the lease; a provably dead one loses it instantly.

At claim time the lock proves itself (two phases, bounded, unambiguous):
holding the lock, a spawned non-blocking second acquisition must fail with
would-block; after release, the same acquisition must succeed. Anything
else — including a probe that cannot run — refuses the claim.

### D-4: the authority matrix (MV-1-4)

Every write path in the control plane has exactly one row here.

| Caller class | Allowed writes |
| --- | --- |
| HOLDER (lease + authentication) | dispatch initial and follow-up, cancel, close, manual reap, supervision arming and shutdown, the agent commit wrapper, evidence collection via its stop hook |
| ANY MAIN (advisors included) | its own announcement, its own protocol-error cursor file, `second-session.sh`, all reads |
| SUPERVISION SET (owner, watcher, reaper, census — identified by their own instance tags, no main ancestor) | supervision state, census records, standing-reaper record transitions |
| DELEGATES | nothing in the control plane; their writes live in their worktrees and their own chain directories |
| HUMAN | everything, ungated |

A non-holder main's stop hook skips evidence collection (advisors do not
run destructive gc) and never commands open work: it reports
OWNED-ELSEWHERE and prints the second-session command. Dispatcher verbs
not listed for advisors refuse for advisors — including cancel, close, and
reap, which mutate records exactly like dispatch does.

### D-5: dispatch under the lease (MV-1-1, MV-1-3)

Two checks, one early and advisory, one final and authoritative:
- AT ENTRY, before any side effect — before worktree creation, payload or
  prompt writes, capability snapshots, or mission reservations — dispatch
  classifies the caller and reads the lease; a non-holder refuses here,
  with nothing yet written.
- AT RECORD CREATION AND LAUNCH: record creation happens inside the lease
  flock (verify holder and generation, write, release). The launch helper
  then re-takes the lease lock immediately before spawning the adapter,
  re-verifies that the generation still equals the record's, spawns under
  the lock, publishes the child identity to the record, and releases. A
  claim landing between record and launch bumps the generation, and the
  launch aborts — the fence now covers the full create-to-launch span,
  which the shipped code splits across the lines MV-1-1 names.

Every job record carries its dispatch-time lease generation and the
dispatching mainId.

### D-6: agent commits (MV-1-2)

Agents commit only through `scripts/agents/commit.sh`, which takes the
lease flock, verifies authentication and holdership, runs `git commit`
while HOLDING the lock, and releases after git returns — the lock spans
the actual mutation, not a pre-check. The pre-commit guard's role narrows
to enforcement of the wrapper: it classifies the committing process's
ancestry; an agent-classified caller whose commit is not running under the
wrapper (the guard probes the lease lock non-blockingly and requires
would-block, proving the lock is held around this commit) is refused with
the wrapper named. Human callers pass untouched, so the check-to-use race
survives only for the human's own commits — the sovereign case, accepted.

### D-7: death cleanup

Claiming a dead holder's lease runs a sweep before the successor's first
dispatch, enforced by a `reapedAfterClaim` stamp carrying the claim's
generation (a predecessor's stamp can never open a successor's dispatch).
The sweep KILLS running jobs whose recorded generation predates the claim.
With D-5's launch fence, a predecessor cannot launch after the sweep: its
launch re-check sees the successor's generation. Later-landing terminal
records fall to the standing reaper cadence, which judges by the same
generation rule. The sweep is idempotent; a crash mid-cleanup re-runs it.

### D-8: the second session, paved and isolated (MV-1-6)

`scripts/agents/second-session.sh` creates a git worktree in a sibling
directory, copies local configuration, and prints the cd command. The
worktree has its own artifacts root and arms its own supervision. Durable
evidence separates by checkout: the evidence path gains a segment
`sha256(resolved absolute checkout path)[:12]`, giving
`evidence.root/agents/<segment>/<chain>/…`. Job records already store
their full mirror path; the collector gains the segmented glob WHILE
KEEPING the legacy unsegmented glob permanently, so existing terminal
chains stay collectible. Proof includes: two distinct worktrees produce
distinct segments; a legacy-layout manifest is still collected; the audit
fixture runs the mediated surface inside a worktree and fails on any path
resolved against the primary checkout's artifacts OR on two checkouts
resolving to one evidence directory.

### D-9: return schema version 2 (KI-22, MV-1-7)

Each role return schema gains a version-2 variant: required
`schemaVersion` with constant 2; optional `claimed` object, additional
properties false, with optional string members `sessionId` and `model` —
the delegate's claims when they disagree with observation, retained for
the record, never canonical. Canonical fields: `sessionId` is the
adapter-observed value or the literal `unobserved`; `model.effective` is
the adapter-observed value or the literal `unreported` (V-1's shipped
literal). Normalization writes observed values over any claim and moves
disagreeing claims into `claimed`. Validation selects by presence: a
return without `schemaVersion` validates against the frozen v1 schema; one
with it, against v2; unknown versions refuse. The v1 path retires in the
release after all adapters emit v2. Fixtures: a v1 return passes v1; a v2
return with claims passes and its canonical fields hold observed values; a
v2 return missing `schemaVersion` fails; undeclared properties fail.

### D-10: protocol errors, keyed, atomic, surfaced (MV-1-8)

The chain root record gains `protocolErrors`: an array of
`{key, round, violation, detectedAt}` where `key` is
`sha256(jobId + round + violationText)[:16]`. Insertion happens in the
SAME locked record write that stamps the round's terminal
`failed/protocol_error` transition — one compare-and-swap updates status
and appends the key if absent, so retries and repeated validations cannot
double-count and no error can be recorded without its terminal transition.
Each main keeps `artifacts/agents/mains/<mainId>.protocol-cursor.json`
mapping chain roots to counts, initialized at announce to the then-current
counts; the stop hook reports growth beyond the cursor and advances it
only after the status line is emitted. A lease claim prints the
predecessor's outstanding totals once, from the sets themselves.

### D-11: the identity-hash filter (KI-19)

Each adapter ships `<runtime>-config-filter.v<N>.txt` naming the CLI's
self-written bookkeeping keys for a declared CLI version range, one
justification line per key citing documentation or an observed churn
record; the file is instruction-bearing and rides the loop. Only
enumerated keys are excluded from the identity hash; unknown keys hash;
an out-of-range CLI version hashes everything and warns. The filter
removes FALSE churn only: a peer changing real behavior (saving a default
model) rightly churns identity, and the refusal names the changed keys.
Per-main configuration isolation is recorded future work.

## Residuals, accepted in writing

- A peer editing shared files with its own editor is invisible to any
  harness; three refusals and the paved path stand between it and harm.
- The authentication triple can collide only within one process-table
  read; the failure direction is refusal.
- The human's own direct git commits are never gated, including their
  races; the human is sovereign here by decision, not oversight.

## Proof

- Identity: a recycled pid with matching start second but different
  command refuses; announcement is idempotent under repeated starts.
- Lease: live holder unclaimable regardless of clock; provable death
  claims instantly; two contenders — one generation winner; stale renewal
  demotes; the two-phase probe passes only as specified.
- Dispatch: a non-holder refuses AT ENTRY with zero side effects on disk;
  a claim between record creation and launch aborts the launch; job
  records carry generation and mainId.
- Commit: the wrapper holds the lock across git; an agent bypassing the
  wrapper is refused by the guard's would-block probe; a human direct
  commit passes.
- Cleanup: the generation-bound stamp gates the successor; the sweep kills
  stale-generation runners; a predecessor's launch after the claim aborts.
- Second session: distinct evidence segments for distinct checkouts;
  legacy manifests still collected; the in-worktree audit passes only on
  full isolation.
- Returns: the four schema fixtures of D-9.
- Protocol errors: double validation yields one entry; error-and-terminal
  are one write; a fresh main reports only post-announce growth; a claim
  prints inherited totals once.
- Authority matrix: every dispatcher verb tested from holder, advisor,
  delegate, and bare-shell callers; each cell of the matrix observed.

## What was deleted, deliberately

Live two-writer coexistence, TTL-expiry takeover, takeover-in-progress
adoption states, environment markers, and foreign-edit detection — each
indicted by critique, each removed rather than deferred. The census stays
observation-only for everything but the authority matrix's gated verbs.
