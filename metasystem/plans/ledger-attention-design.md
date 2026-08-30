# Ledger Attention — design

Goal: `plans/goals/ledger-attention.md`. Design half (1h, Fable lane).
The implementation half is a separate 2h Sol-lane slice; the plan in
§7 leaves it zero judgment calls. All file:line citations are relative
to the metasystem root and were traced on this tree (2026-08-30).

> Provenance: authored whole by the Fable design delegate
> (job ledger-attention-design-r2, claude-fable-5, fresh context).
> The delegate's native budget expired on the final Write call; the
> coordinator recovered these bytes verbatim from the job's stream
> (rounds/1/claude-stream.jsonl) and placed them unmodified per
> R-25b-m1 — carried, not authored.
> Revision 1: revised by the same design lane (job
> ledger-attention-design-r3, claude-fable-5) to close the ten
> material findings of the Sol critique round (job
> ledger-attention-design-critique-r1); the per-finding record is §9.
> Revision 2: revised by the same design lane (job
> ledger-attention-design-r5, claude-fable-5) to close the six
> re-opened findings and one new finding of Sol's round-2
> verification; the per-finding record is §9, "Revision 2".

## 1. The incident (why this exists)

Recorded evidence, 2026-08-30: machine m2's goal-ledger reads went
stale for hours. The accepted-goals ref
(`refs/metasystem/goals/accepted`, `internal/goal/txn.go:39`) advances
only when the local machine runs a path that fetches — a goal verb
publish (`internal/goal/txn.go:685`) or an explicit
`goal fetch` (`runGoalFetch`,
`cmd/metasystem/goalsync_verbs.go:401-424`; the fetch-first read path
is `Project` with fetchFirst, `internal/goal/project.go:31-36`). Pure
reads do not fetch: `goal list` (read-only by construction,
`cmd/metasystem/goal.go:267-270`) and the session turn verdict
(`internal/goal/turnverdict.go:459` calls `Project(endpoint, false,
…)`), both read the last accepted tree. m2 ran `git pull` diligently —
which moves `main`'s checkout, not the accepted ref — saw m1's
landings, and still reported "no pins" while an addressed goal
(vm-validation-sweep) and a new claimable goal (ledger-attention
itself) sat on origin for hours. A human relayed the nudge.

The class this design kills: **a machine whose accepted ref only moves
when it happens to publish**. The steward tick — the one process that
already runs on a cadence on every machine — gets a new component that
runs the read-side validator on every tick, so the accepted world is
never older than one tick plus one fetch, and surfaces what changed.

The stall-detection half already landed: `CurrentMarks` reads the
accepted ref tip as the ledger's content identity
(`internal/steward/marks.go:43-51`, commit 9781416). This design does
not duplicate it; it feeds it (§3, marks interaction).

## 2. The surfaced facts

The component surfaces exactly three fact kinds, computed per change
(§4: one change = one first-parent commit on the canonical branch) by
diffing consecutive ledger states — each state read as a tree at its
commit (`loadTree`, `internal/goal/verbs.go:84`, wrapped by the new
read-only `goal.ProjectAt`, §7.1), with the frontier from `goal.Next`
(`internal/goal/project.go:90`) and the machine nickname from
`goal.ResolveMachine` (`internal/goal/actor.go:21`):

- **New claimable goals**: ids in `Next(...).Ready` for this machine
  at this change's state that were not Ready at the previous state.
- **Pins addressed to this machine**: ids of live queued goals whose
  `Pinned` field (`internal/goal/file.go:36`) equals this machine's
  nickname at this change's state and did not at the previous state.
- **Queue reorderings** (LA-R1-001, the human-origin event class
  restored): the ordered queue changed. The queue's authoritative
  order is the coordinator-facing one — every live queued goal,
  sorted by `OpenedAt` then id — because that is the order the
  session turn verdict renders and names its head from
  (`queuedFrontier`, `internal/goal/turnverdict.go:517-551`, sort at
  `:541-546`). The lexicographic `sortedGoalIds`
  (`internal/goal/project.go:102`, `internal/goal/validate.go:472`)
  is an iteration order for membership computation, not a queue
  order; the r1 claim that every reader shares it was wrong — the
  verdict does not — and is withdrawn. The fact fires whenever the
  ordered id sequence at this change's state differs from the
  previous state's sequence in any way: a goal left, joined, or the
  relative order of members changed. Departures are one case of this
  class, not a replacement for it.

Three destinations, each with exact grammar, applied ONCE PER CHANGE
(§4). `<tip12>` is the first 12 hex digits of the change's commit;
`<machine>` is the resolved nickname; segments with no content are
omitted; a change carrying none of the three facts for this machine
surfaces nothing, but the dedupe frontier still advances (§4).

**Destination 1 — the narrator digest** (durable, via the entries
`NarrateDigest` appends, `internal/steward/narrate.go:47-83`), one
entry per change:

```
Kind:       highlight
SourceType: ledger
SourceID:   <the change's full commit oid>
Text:       The shared goal ledger moved to <tip12>: <segments>.
```

where `<segments>` is the `"; "`-join of, in this order:

```
<N> new claimable goal(s): <id1>, <id2>, …
<M> pin(s) addressed to <machine>: <id1>, …
the queue reordered: now <list> (was <list>)
```

and `<list>` renders the ordered queue as the comma-join of its first
five ids, then `, +<n> more` when longer.

**Destination 2 — the health row** (§5 grammar) plus the
delivery-gated notification queue: one durable `QueueNotification`
(`internal/steward/intervene.go:299-314`) PER CHANGE, with

```
Nonce:   ledger-attention-<the change's full commit oid>
Message: steward: the shared goal ledger moved to <tip12> — <segments>
```

Nonces are per change, so pending files coexist (one file per nonce,
`internal/steward/intervene.go:305`) and every change is delivered as
its own nudge; the same-nonce replace only dedupes re-surfacing of the
SAME change after a crash (§4). The best-effort tick narration line
(`narrationLine`, `internal/steward/narrate.go:123`) additionally gets
one note in its `notes` list, aggregated over the pass's changes:

```
the shared ledger moved (<K> change(s), <N> claimable, <M> pinned here)
```

**Destination 3 — the session turn verdict**, stated to its actual
mechanism (LA-R2-001). The verdict blocks the session once per
queue-digest change (`internal/goal/turnverdict.go:403-408`); the
digest covers every queued goal's id and revision in the authoritative
order (`internal/goal/turnverdict.go:527-551`), and a pin edit bumps
the pinned goal's revision (`set-pin`'s mutation runs `touch`,
`internal/goal/verbs.go:1857-1858`, and `touch` is Revision+1,
`internal/goal/verbs.go:98-99`) — so once this component keeps the
accepted ref current, a surfaced change re-blocks the next turn
exactly once over the current world. The block's text, however, names
only the FIRST queued row (`internal/goal/turnverdict.go:551`,
`:405-407`) and neither filters nor prioritizes pins: an addressed pin
may not be that row. The r1 claim that the pin "lands in front of the
coordinator as the existing queue block" is withdrawn as overreach —
the pin's IDENTITY reaches the coordinator through destinations 1 and
2; the verdict contributes one truthful re-block, no more. **No
turn-verdict code changes**, and no promise the verdict cannot keep.

## 3. The fetch

- **Cadence: every tick.** The tick's default cadence is 600 seconds
  (`TickSeconds`, `internal/steward/runner.go:51-59`); one small git
  fetch per ten minutes is cheaper than the per-operation fetch every
  goal verb already performs (`CaptureTip`,
  `internal/goal/txn.go:103`). No Nth-tick divisor and no extra config
  key. Fixtures that set `tick-seconds 1` fetch a local file-path
  remote; that is in-budget for tests.
- **The exact fetch**: `FetchAdvance`'s own sequence
  (`internal/goal/fetchadvance.go:30-82`) run DECOMPOSED into the
  same already-exported steps, so that the transport leg is killable
  (below) and the per-change diff is durably recorded BEFORE the
  accepted ref advances (§4). In order: a per-operation nonce; the
  bounded capture onto the ephemeral per-op ref
  (`refs/metasystem/goals/fetch/read-…`); then, in-process:
  `SyncModeGate` (`internal/goal/txn.go:195`), `AcceptanceGates`
  (foreign-ledger and rewind refusal,
  `internal/goal/fetchadvance.go:91`) when the accepted ref resolves,
  `ValidateCommit` (`internal/goal/validate.go:440`), the §4 durable
  observation write, and only then the forward-only CAS
  (`AdvanceAccepted` → `advanceAcceptedForward`,
  `internal/goal/txn.go:394-431`), with `CleanupRefs`
  (`internal/goal/txn.go:435`) deferred. Same gates, same refusals,
  same refs as the `goal fetch` verb (`runGoalFetch`,
  `cmd/metasystem/goalsync_verbs.go:401-424`) — only the ordering
  seam between validation and advance is new, and every step is an
  existing exported function.
- **Ref discipline**: the component may move exactly two refs: the
  ephemeral per-op fetch ref, and `refs/metasystem/goals/accepted` —
  forward only, CAS-guarded, and **never past what validates**,
  because the pass runs the identical gates and refuses any tip that
  fails them, leaving the accepted ref untouched. The component adds
  no ref writes of its own and never touches the canonical branch,
  HEAD, index, or worktree (the transaction engine's standing
  guarantee, `internal/goal/txn.go:10-12`).
- **The fetch is bounded and crash-contained (LA-R1-003).** The only
  slow, network-dependent step is the transport leg, and it runs in a
  git child THE STEWARD OWNS AND KILLS. New function
  `goal.CaptureTipBounded(e Endpoint, opid string, budget
  time.Duration) (string, error)` in `internal/goal/attention.go`:
  it runs exactly `CaptureTip`'s remote invocation (`git fetch
  --no-tags --refmap= <remote> +<branch>:<per-op ref>`,
  `internal/goal/txn.go:126-127`, under the same
  `environWithoutGitSteering` scrub, `internal/goal/genesis.go:125`)
  via `exec.CommandContext` with `SysProcAttr{Setpgid: true}`, a
  `Cmd.Cancel` that sends SIGTERM to the child's process group, and
  `Cmd.WaitDelay = 5s` escalating to SIGKILL (both fields available
  at the module's Go 1.26, go.mod:3), under a context deadline of
  `budget`; then reads the per-op ref with a plain `rev-parse`. On
  timeout the child and its whole process group are dead before the
  call returns: no orphaned subprocess survives the pass, and even a
  racing kill cannot advance the accepted ref, because the child only
  writes the per-op fetch ref — `AdvanceAccepted` runs in-process,
  after the child, never inside it. The r1 goroutine, package-memory
  single-flight flag, and stranded-child residual are all withdrawn:
  the wait is synchronous (bounded by the budget inside the tick's
  arbitration lock, `internal/steward/tick.go:108-112`; 60s is ~10%
  of the default 600s cadence, `TickSeconds`,
  `internal/steward/runner.go:51-59`), so a one-shot tick process
  (`runStewardTick`, `cmd/metasystem/steward_verbs.go:218`) exits
  with no fetch behind it, and a runner restarted over a crashed
  predecessor can at worst overlap that predecessor's orphaned
  CAPTURE — which touches only its own per-op ref and can advance
  nothing (the per-op-ref no-collision design,
  `internal/goal/txn.go:120-127`). The budget is
  `var ledgerAttentionFetchBudget = 60 * time.Second` — a package
  VAR, deliberately not a const, precisely so the §7 timeout test can
  shorten it and restore it; no config key. Stated residual: a
  SIGKILLed git can leave per-operation debris (a stale per-op ref or
  its lock file, unreferenced objects) — never a lock on the accepted
  ref or the canonical branch, which the child does not touch; the
  deferred `CleanupRefs` deletes the per-op ref when deletable, and
  the SIGTERM-first, five-second-grace wind-down lets git clean up in
  the common case.
- **Offline / no-remote behavior — fail quiet, record the attempt.**
  Any `FetchAdvance` error or timeout (no route, missing remote, a
  validation refusal, or the budget above) is recorded in the
  component's state file (`lastAttemptAt`, `lastFailure`,
  `failingSince`) and in the component-attempt evidence, and the pass
  returns a report; it never fails the tick, never queues a
  notification, and never produces a health red by itself. §5 defines
  when a *persistent* failure streak becomes health `unknown` (never
  `dead`) — a network blip inside the threshold stays `alive` and
  silent.
- **Guards — broken is not pre-bootstrap (LA-R1-010).** The pass
  guards on a new exported read-only accessor,
  `goal.AcceptedLedgerTip(root) (tip string, exists bool, err
  error)`, added in a new file `internal/goal/attention.go`: it
  delegates to `acceptedTipForGates` (`internal/goal/txn.go:142-173`,
  which distinguishes an absent accepted ref from a broken one —
  unreadable ref file, non-commit target) and, when the ref resolves,
  probes the root record with `cat-file -e <tip>:./<goalsPrefix>backlog.md`
  (the `NewWorld` probe, `internal/goal/actor.go:57-64`).
  Classification: `err != nil` → outcome `failed` with the error text
  recorded as `lastFailure` (so a broken accepted world is recorded
  and matures to health `unknown`, never silently read as healthy
  bootstrap); `exists == false`, or the tip lacks the root record
  (pre-migration) → outcome `pre-bootstrap`, bookkeeping saved,
  nothing else. `goal.NewWorld` is NOT the guard — it returns false
  for every resolution error (`internal/goal/actor.go:57-64`) and
  cannot tell broken from absent. The accessor is additive, touches
  no verb and no CAS path, and changes no existing file in
  `internal/goal`.
- **Single-machine mode** (`Endpoint.LocalMode`,
  `internal/goal/txn.go:64`): no remote ledger exists to watch; the
  pass records outcome `local` and does nothing further.
- **Placement in the tick**: the component runs after
  `ReapContinuations` (`internal/steward/tick.go:165-168`) and before
  `LoadEvidence`/`CurrentMarks` (`internal/steward/tick.go:170-179`).
  Consequence, stated deliberately: remote ledger movement advances
  the accepted ref before `CurrentMarks` reads it, so it registers as
  the world advancing and resets the stall aging. That is the landed
  intent of the marks change — the accepted tip *is* the ledger's
  content identity, "every goal movement" visible to stall detection
  (`internal/steward/marks.go:39-42`).

## 4. Dedupe — a nudge fires once per change

- **What one change IS (LA-R1-007)**: one commit on the canonical
  branch's first-parent spine. The ledger only moves by commits, so
  the sequence of first-parent tree states between the last diffed
  tip and the newly captured tip is exactly the sequence of ledger
  states, and the pass walks it: `git rev-list --reverse
  --first-parent <diffedTip>..<captured>`, reading each commit's
  state via `goal.ProjectAt` (§7.1, wrapping `loadTree`,
  `internal/goal/verbs.go:84`) and computing the §2 facts between
  consecutive states. Every change gets its own events even when
  several land between two ticks — the r1 frontier-only coalescing
  is withdrawn. A change carrying no §2 fact for this machine
  produces no event but still advances the diffed frontier. (A
  non-first-parent side commit is not a canonical ledger state; its
  content surfaces in its merge commit's change.) The identity of a
  change is its commit oid — never `AdvanceResult.Tip` (LA-R1-008
  stands: `advanceAcceptedForward` can report a tip the ref is
  already past, `internal/goal/txn.go:417-419`).
- **Events are durable BEFORE the world moves.** The pass appends
  the computed events to the `pending` list and advances `diffedTip`
  plus the three snapshots in ONE `atomicfile.WriteText` publication
  (§7, write point W4) — and only after that write returns
  `(true, nil)` does it run `AdvanceAccepted`. The accepted ref can
  therefore never hold a tip whose events are not already durably
  recorded. Sol's counterexample dies here: durable old tip S0, tip
  T1 making goal A claimable, a crash at ANY point — either the ref
  still reads S0 and T1 is rediffed commit-by-commit next pass, or
  the ref reads T1 and A's event is already in `pending`; a later T2
  claiming or parking A cannot erase the T1 event, it is on disk.
- **Mark home**: `artifacts/agents/steward/ledger-attention.json`,
  the component's single state file, beside the health record
  (`HealthRecordPath`, `internal/steward/health.go:127`). Schema
  (schema 2, all times RFC3339 UTC):

```json
{
  "schema": 2,
  "lastAttemptAt": "…",
  "lastOutcome": "advanced|current|failed|local|pre-bootstrap",
  "lastFailure": "",
  "failingSince": "",
  "remoteTip": "<the last captured-and-validated canonical tip>",
  "remoteTipAt": "<sampled AFTER the accepted ref provably held remoteTip>",
  "examinedTip": "<oid>",
  "movedAt": "",
  "diffedTip": "<the newest tip whose events are durably computed>",
  "ready": ["<Next().Ready at diffedTip>"],
  "pinned": ["<queued ids pinned here at diffedTip>"],
  "queue": ["<all queued ids at diffedTip, authoritative order>"],
  "pending": [
    {"tip": "<change commit oid>", "at": "…",
     "claimable": ["<id>"], "pins": ["<id>"],
     "queueNow": ["<id>"], "queueWas": ["<id>"]}
  ]
}
```

- **First run — baseline BEFORE the fetch (LA-R1-002).** When the
  state file is absent, the pass baselines from the world this
  machine has been acting on, *before* fetching: resolve the local
  accepted tip via `goal.AcceptedLedgerTip` (§3 guards run first),
  call `goal.Project(endpoint, false, now)`
  (`internal/goal/project.go:31`), and durably write `diffedTip =
  examinedTip = Projection.Tip` with the three snapshots and empty
  `pending`. Only when that write returns `(true, nil)` does the
  pass continue to the fetch (LA-R1-006: a baseline that is not
  provably durable is never built upon — §7 step c). On the rollout
  shape (local accepted at S0, remote at T1, no state file) the
  first tick surfaces the S0→T1 changes commit by commit and leaves
  `examinedTip = S0 ≠ remoteTip = T1`, starting the staleness clock
  — the incident is surfaced, not erased. History never replays:
  the baseline is the local live world (the auto-baseline principle
  of `scripts/watch-background-jobs.sh`, applied pre-fetch), and a
  machine already at the canonical tip surfaces nothing.
- **Surfacing and the mark — one nudge per change; exactly once in
  the normal path, at least once always (LA-R1-007).** Each pending
  event gets its own durable surfaces: one digest entry with
  `SourceID` = the change's commit oid, and one `QueueNotification`
  with the per-change nonce (§2). Events leave `pending` (the mark,
  one durable write) only after both surfaces landed for them. A
  crash between surfacing and the mark re-surfaces the same events
  next tick: the digest collapses the identical `SourceID`
  (`internal/narratordigest/digest_test.go:13-19`) and rewriting the
  same per-change nonce is idempotent — at-least-once, converging on
  exactly-once. At-most-once holds because a marked event never
  returns: its commit is at or behind `diffedTip` and its record is
  deleted. Volume is the requirement's own arithmetic, stated
  plainly: a machine that missed N changes surfaces N digest entries
  and N pending nudges on its first pass back — that IS "a nudge
  fires once per change". If the human wants coalescing above some
  threshold, that is a requirement change only the human can make
  (§9); it is not designed in.

## 5. Staleness health — the condition itself

A new health role, appended to the stable role vocabulary
(`internal/steward/health.go:44-57`) and to `healthRoleOrder`
(`internal/steward/health.go:59-72`):

```go
RoleLedgerAttention HealthRole = "ledger-attention"
```

- **The condition (dead/red)**: the ledger MOVED — `remoteTip`
  differs from `examinedTip` — and stayed unexamined past the
  threshold: `now − movedAt ≥ threshold`. `movedAt` is set by the
  first pass that observes the divergence and cleared when the tips
  agree again.
- **Config key and default**: git config
  `metasystem.steward.ledger-attention-stale-minutes`, default **30**
  (three default ticks), read with the exact pattern of `TickSeconds`
  (`internal/steward/runner.go:51-59`): a positive integer wins,
  anything else takes the default.
- **What clears it** (advances `examinedTip` to `remoteTip` and
  empties `movedAt`), checked at the start of every pass BEFORE the
  fetch, from purely local evidence — journal files and hook
  evidence need no network — and durably saved before the fetch is
  attempted (§7 step d precedes the fetch at step e; LA-R1-005: the
  r1 plan returned on fetch failure before these checks, so an
  offline pass never consumed the operator's clearing evidence; that
  ordering is withdrawn — a pass whose fetch then times out, is
  refused, or finds the remote unreachable has ALREADY cleared).
  Both rules bind to the accepted TIP (LA-R1-004):
  1. **A tip-recorded journal entry.** A goal verb records the
     captured-and-validated canonical tip durably into its journal
     entry (`RecordSteps` fills `Entry.FetchedOid`,
     `internal/goal/journal.go:260-283` and `:88`, called after the
     capture, gates, and whole-tree validation at
     `internal/goal/txn.go:609`). The pass reads
     `goal.Entries(repoRoot)` (`internal/goal/journal.go:401`) and
     clears when any entry's `FetchedOid` is non-empty and satisfies
     `git merge-base --is-ancestor <remoteTip> <FetchedOid>`
     (ancestor-or-equal): that verb provably examined a world at or
     past `remoteTip`. A crash between `CreateEntry`
     (`internal/goal/txn.go:540`) and `RecordSteps` leaves
     `FetchedOid` empty and clears nothing — the r1 mtime rule that
     this crash could fool is withdrawn. A later remote advance to T2
     is not cleared by a T1-recorded entry, because T2 is not an
     ancestor of T1.
  2. **A coordinator turn whose hook attempt BEGAN after this
     machine provably held `remoteTip`.** The supervision hook
     records its attempt (`steward hook-attempt` →
     `BeginHookAttempt`,
     `internal/steward/component_evidence.go:165-224`, `LastAttempt`
     set at `:216`) at `scripts/agents/supervision-hook.sh:92` —
     BEFORE it renders the turn verdict at
     `supervision-hook.sh:274` — and the render reads the accepted
     projection (`internal/goal/turnverdict.go:459`, `:523`). The
     accepted ref only moves forward (`advanceAcceptedForward`,
     `internal/goal/txn.go:404-431`), and `remoteTipAt` is a fresh
     clock sample taken AFTER the pass's own `AdvanceAccepted`
     confirms the ref holds `remoteTip` (§7 step h — never the
     tick-start `now` parameter, never before the fetch). Therefore:
     attempt begun after `remoteTipAt` ⇒ projection read after
     `remoteTipAt` ⇒ the turn examined `remoteTip` or a descendant.
     The pass clears when `loadComponentEvidenceForHealth(repoRoot,
     "supervision-hook")`
     (`internal/steward/component_evidence.go:395-413`) reports no
     pending durability marker and EITHER the current record shows
     `Result == ComponentOK && Outcome == "EMITTED" &&
     SuccessAttemptSeq == AttemptSeq` (the exact success binding
     `checkHookFreshnessAt` trusts,
     `internal/steward/health.go:310-314`) with
     `LastAttempt.After(remoteTipAt)`, OR any `AttemptHistory` entry
     (retained for the hook on success,
     `internal/steward/component_evidence.go:283-285`) has `Result
     == ComponentOK && Outcome == "EMITTED" &&
     AttemptedAt.After(remoteTipAt)`. The r1 rule comparing
     `LastSuccess` (completion time) is withdrawn: a turn can START
     before the tip arrives and COMPLETE after it — its verdict
     examined the OLD world, because the render at
     `supervision-hook.sh:274` precedes the completion record
     (`emit_stop_payload` at `:321`, recording at `:209-211`) — so
     completion-after proves nothing; attempt-begun-after does.
     Crash conservatism: a crash between `AdvanceAccepted` and the
     save that records `remoteTipAt` leaves the OLD pair in the
     state file; the next pass re-resolves and samples a LATER
     `remoteTipAt` than the true advance instant, which only demands
     a later turn — clearing can be delayed, never granted falsely.

  Stated residual: a bare-terminal `metasystem goal fetch`
  (`runGoalFetch`, `cmd/metasystem/goalsync_verbs.go:401-424`)
  advances the ref but records neither a journal entry nor hook
  evidence, so it does not — and must not — clear the row: fetching
  is not examining; that gap was the incident.
- **The check**: `checkLedgerAttention(repoRoot, now)` is a pure read
  of the state file (so `PreviewHealth`,
  `internal/steward/health.go:212`, stays side-effect free); all
  clearing writes happen in the tick component. Verdicts:
  - `alive` — `remoteTip == examinedTip`; reason
    `examined at the canonical tip <tip12>`.
  - `alive` — divergence or fetch failure younger than the threshold;
    reason `the shared ledger moved <age> ago; quiet until <threshold>m`
    or `the last fetch failed <age> ago; quiet until <threshold>m`.
  - `unknown` — the state file is missing/unreadable, or
    `failingSince` is older than the threshold; reason
    `the shared ledger has been unreachable for <age>: <lastFailure>`.
    A blip never reds: persistent unreachability is `unknown`, and the
    existing two-observation unknown alert
    (`internal/steward/health.go:359-368`) escalates it.
  - `dead` — the condition above; reason
    `the shared ledger moved to <tip12> <age> ago and is unexamined past <threshold>m`;
    remedy
    `complete one coordinator turn here, or run a journaling goal verb; 'metasystem goal fetch' does not examine`;
    (LA-R1-005: both named acts produce a durable clearing evidence
    from the tip-bound rules above — a completed turn writes
    supervision-hook `LastSuccess`, a journaling verb writes
    `Entry.FetchedOid`; the r1 remedy named a `goal list --fetch`
    flag that does not exist, `cmd/metasystem/goal.go:270-275`, and
    even the shipped `goal fetch` clears nothing, which the residual
    above now states instead of prescribing.)
    `NoAutomaticRemedy = true` (the retro-debt pattern,
    `internal/steward/health.go:270-272`), so `hasLawfulAutomaticRemedy`
    (`internal/steward/health.go:410`) yields `NO_LAWFUL_REMEDY` and
    the alert fires — no machinery restart can examine a ledger.
  - **Precedence (LA-R1-009)**: when both mature conditions hold at
    once — `failingSince` past the threshold AND an unexamined
    movement past the threshold — `dead` wins. Rationale: the
    movement to `remoteTip` was OBSERVED and validated before the
    fetches started failing; unreachability can only mean `remoteTip`
    understates how far the ledger has moved, never that the known
    unexamined movement is in doubt. The check evaluates the `dead`
    predicate first and falls through to `unknown` only when it does
    not hold.
- **Health row grammar** (rendered by the unchanged
  `HealthVerdict.Line`, `internal/steward/health.go:149`):

```
ledger-attention=dead (the shared ledger moved to <tip12> 47m ago and is unexamined past 30m; remedy: complete one coordinator turn here, or run a journaling goal verb; 'metasystem goal fetch' does not examine)
```

- **Approaching staleness noticing** (best-effort, before red): one
  `Noticing` added in `noticings` *before* the `ActNone` gate, like
  the provider-outage noticing (`internal/steward/narrate.go:205-216`),
  emitted while `0 < now − movedAt < threshold`:

```
Key:  ledger-unexamined
Line: noticing: the shared goal ledger moved <age> ago (tip <tip12>) and nothing here has examined it — a goal verb or a coordinator turn clears this (health goes red at <threshold>m)
```

## 6. Boundary — attention, never authority

R-27 binds: authority is the enumerated sum of held roles, and this
attention mechanism holds none. Concretely:

- The component runs **read-side machinery only**. `FetchAdvance` is
  the identical act a `metasystem goal fetch` performs; advancing the
  accepted ref onto a validated tip changes what this machine *sees*,
  never what it *holds*. The component never calls `Publish`
  (`internal/goal/txn.go:503`), never creates a journal intent of its
  own, never mutates a goal file, and never claims.
- **Surfacing a pin grants nothing.** A pin is a restriction on who
  *may* claim (`internal/goal/file.go:33-36`); the claim itself still
  goes through goal verbs under Law 1 (budgets before governed
  execution), the one-claim-per-machine quota, and Ruling S (landings
  alone). Every surfaced line states facts ("the queue holds…", "pins
  addressed to…"); none is an instruction, and no surfaced line is
  consumed by any automation — the readers are the coordinator session
  and the human.
- **No autonomous claiming**, in any failure mode: a red staleness row
  escalates to the operator (`NO_LAWFUL_REMEDY`); it never triggers a
  claim, a promote, or any verb.
- **Agent-agnostic by construction**: the ledger stays the only
  channel. The component reads git refs and writes steward-local
  files; nothing in it branches on runtime or model, and every
  destination (§2) is a runtime-neutral file or queue.

## 7. Implementation plan — the 2h Sol slice

Additive only; the tick's component-attempt pattern; no tick/watch
machinery redesign. Files and functions, in build order:

**1. New file `internal/goal/attention.go`** (read-only and
transport-bounded accessors; no existing file in `internal/goal`
changes):
- `AcceptedLedgerTip(root string) (tip string, exists bool, err
  error)` — delegates to `acceptedTipForGates`
  (`internal/goal/txn.go:142-173`); when the ref resolves, probes
  `cat-file -e <tip>:./<goalsPrefix>backlog.md` (the `NewWorld`
  probe, `internal/goal/actor.go:57-64`) and reports `exists=false`
  when the root record is absent.
- `CaptureTipBounded(e Endpoint, opid string, budget time.Duration)
  (string, error)` — the §3 killable capture.
- `ProjectAt(root, tip string) (Projection, error)` — `loadTree`
  (`internal/goal/verbs.go:84`) wrapped in a
  `Projection{Tip, Tree}`; the per-change state reader of §4.

**2. New file `internal/steward/ledgerattention.go`:**
- `ledgerAttentionStatePath(repoRoot string) string` →
  `artifacts/agents/steward/ledger-attention.json`.
- `type ledgerAttentionState struct` — exactly the §4 schema 2;
  `type LedgerAttentionEvent struct { Tip, At string; Claimable,
  Pins, QueueNow, QueueWas []string }`.
- `var ledgerAttentionFetchBudget = 60 * time.Second` (§3; a var so
  tests shorten it) and `var ledgerAttentionWriter =
  atomicfile.WriteText` (the `componentEvidenceWriter` stub pattern,
  `internal/steward/component_evidence.go:415`, for durability
  tests).
- `saveLedgerAttentionState(repoRoot string, s ledgerAttentionState)
  error` — the ONE writer; exactly `saveHealthRecord`'s mapping
  (`internal/steward/health.go:1182-1195`): a write error returns
  it, and the committed-but-durability-unknown `(false, nil)` of
  `atomicfile.WriteText` (`internal/atomicfile/atomicfile.go:76-81`,
  rule at `:108-113`) returns an error naming unknown durability.
  LA-R1-006's outcome mapping, total: EVERY save in the pass goes
  through this function, and a non-nil return anywhere makes the
  pass's outcome `failed` with `Failure` = the write error's text —
  carried in the report and recorded durably by the tick's
  `completeComponentAttempt` as `ComponentError`/`STATE_WRITE_FAILED`
  (the component-evidence file is a separate durable channel), so a
  state-write failure is always reported even though the state file
  itself could not be written.
- `ledgerAttentionStaleMinutes(repoRoot string) int` — the
  `TickSeconds` pattern on
  `metasystem.steward.ledger-attention-stale-minutes`, default 30.
- `type LedgerAttentionReport struct { Outcome, Tip, Failure string;
  Pending []LedgerAttentionEvent; MovedAt time.Time }` — `Pending`
  is every still-unsurfaced event, oldest first, including prior
  passes' leftovers.
- `RunLedgerAttention(repoRoot string, now time.Time)
  LedgerAttentionReport` — ordered steps and write points:
  - (a) `goal.ResolveEndpoint` error → save bookkeeping, `failed`;
    `LocalMode` → save bookkeeping, `local`, return.
  - (b) `goal.AcceptedLedgerTip`: error → save with `lastFailure`
    (set `failingSince` when empty), `failed`, return; absent → save,
    `pre-bootstrap`, return. An unreadable state file on load →
    `failed` with the parse error, no overwrite, return.
  - (c) no state file yet → the §4 pre-fetch baseline, write W1; a
    W1 error → `failed`, RETURN WITHOUT FETCHING (LA-R1-006: the
    accepted ref must never advance past a frontier that is not
    provably durable).
  - (d) the §5 clearing checks against the STORED
    `remoteTip`/`remoteTipAt` — journal `FetchedOid` ancestry, then
    the hook attempt rule — purely local, BEFORE any fetch
    (LA-R1-005); when anything cleared, write W2 immediately; a W2
    error → `failed`, return (clearing retries next pass).
  - (e) the §3 bounded capture (per-op nonce, `CaptureTipBounded`,
    deferred `CleanupRefs`); timeout or error → write W3
    (attempt + failure, set `failingSince` when empty), `failed`,
    return.
  - (f) in-process gates: `SyncModeGate`; `AcceptanceGates` when the
    accepted ref resolves; `ValidateCommit`. Any refusal → W3-style
    save, `failed`, return.
  - (g) the §4 per-commit walk from `diffedTip` to the captured tip;
    when it yields events or moves `diffedTip`, ONE write W4
    appending `pending` events and advancing `diffedTip` plus the
    `ready`/`pinned`/`queue` snapshots; a W4 error → `failed`,
    return — the ref is NOT advanced, the walk reruns next pass.
  - (h) `AdvanceAccepted(root, captured)`
    (`internal/goal/txn.go:394`); error → save `failed`, return
    (`pending` survives; the CAS retries next pass). On success:
    `remoteTip = captured`; when `remoteTip` changed, `remoteTipAt =
    time.Now()` sampled HERE, after the CAS (LA-R1-004 — never the
    `now` parameter); recompute `movedAt` (`remoteTip !=
    examinedTip` sets it to `remoteTipAt` when empty; equality
    clears it); clear `failingSince`; write W5; a W5 error →
    `failed`, return (events stay pending; the stale `remoteTipAt`
    is conservative per §5).
  - (i) report: `advanced` when the walk found commits, else
    `current`; `Pending` = all pending events.
- `PersistLedgerAttentionMark(repoRoot string, surfacedTips
  []string) error` — deletes exactly the named events from
  `pending`, one durable save (W6) through
  `saveLedgerAttentionState`; any error propagates and fails the
  tick.
- `checkLedgerAttention(repoRoot string, now time.Time) RoleVerdict`
  — pure read of the state file, §5 verdict table verbatim, `dead`
  evaluated before `unknown` (§5 precedence).

**3. `internal/steward/tick.go`:**
- `TickResult` (`tick.go:45-56`) gains
  `LedgerAttention LedgerAttentionReport`.
- Between `tick.go:168` and `tick.go:170`: `beginComponentAttempt(…,
  "ledger-attention", generation, selfExact.Ref(), time.Now())`
  (pattern of `tick.go:124`), call `RunLedgerAttention`, then
  `completeComponentAttempt` with `ComponentOK`/`PASS_COMPLETE`
  carrying the outcome, or `ComponentError`/`FETCH_FAILED` carrying
  the failure text (pattern of `tick.go:226-228`); either way the tick
  continues.
- After `NarrateDigest` succeeds (`tick.go:211-213`, having appended
  one §2 entry per pending event) and before `SaveEvidence`
  (`tick.go:214`): for each pending event, `QueueNotification` with
  its per-change nonce (§2) — an error queuing any event fails the
  tick before the mark, so the event stays pending and resurfaces —
  then `PersistLedgerAttentionMark` with every surfaced tip; a
  persist error fails the tick (the mark is durable state).

**4. `internal/steward/narrate.go`:**
- `NarrateDigest` (`narrate.go:47-83`): append the §2 digest entry
  when the report carries nonzero facts.
- `narrationLine` (`narrate.go:123-169`): the §2 note.
- `noticings` (`narrate.go:198-239`): the §5 `ledger-unexamined`
  noticing, placed with the outage noticing before the `ActNone` gate.

**5. `internal/steward/health.go`:**
- `RoleLedgerAttention` const (`health.go:44-57`); append to
  `healthRoleOrder` (`health.go:59-72`) and to the slice in
  `evaluateHealthRoles` (`health.go:239-255`), calling
  `checkLedgerAttention(repoRoot, now)`.
- `hasLawfulAutomaticRemedy` (`health.go:410`) needs no case: the
  default `false` plus `NoAutomaticRemedy=true` yields
  `NO_LAWFUL_REMEDY` as specified.

**6. Tests — new `internal/steward/ledgerattention_test.go` (file-path
bare remote fixtures, the `internal/goal` race-test pattern), plus one
grammar assertion each in `health_test.go` and `narrate_test.go`:**
- local mode: outcome `local`, no fetch, row alive.
- pre-bootstrap: outcome `pre-bootstrap`, row alive.
- movement surfaces once: remote publish → pass 1 advances accepted
  (assert the ref equals the remote tip) and reports the new claimable
  id; pass 2 reports nothing (dedupe).
- pin addressed here appears in `NewPins`; a pin addressed elsewhere
  does not (and its goal is absent from `NewClaimable`, matching
  `Next`'s frontier rules, `internal/goal/project.go:117-121`).
- queue reorderings (LA-R1-001): a goal claimed on the remote leaves
  the ordered queue ⇒ one event with `QueueWas`/`QueueNow` showing
  the sequence change; plus a direct unit test of the sequence
  comparator on synthetic ordered lists (same members, different
  order ⇒ fires), so the detector is proven for every reordering
  shape whether or not today's verbs can produce it.
- per-change nudges (LA-R1-007): two remote publishes between passes
  ⇒ two events, two digest entries with distinct `SourceID`s, two
  pending notification files with distinct nonces.
- transient fact across crash and tip advance (Sol's LA-R1-007
  scenario): baseline S0; remote T1 makes goal A claimable; run the
  pass (events durable, ref advanced) but skip the tick's surfacing
  (the simulated crash); remote T2 parks A; next full pass ⇒ A's
  claimable event still surfaces with `SourceID` = T1's commit oid.
- durability refusal (LA-R1-006): stub `ledgerAttentionWriter` to
  return `(false, nil)` ⇒ the baseline pass reports `failed` and
  performs NO fetch; the tick records
  `ComponentError`/`STATE_WRITE_FAILED`.
- offline clearing (LA-R1-005): dead row, unreachable remote path; a
  journal entry with `FetchedOid` at `remoteTip` ⇒ the failing pass
  still clears the row before its fetch fails.
- first run against a moved remote: local accepted at S0, remote at
  T1, no state file ⇒ the first pass surfaces the S0→T1 delta and
  leaves `examinedTip = S0` (LA-R1-002).
- broken accepted ref: a corrupt loose ref file ⇒ outcome `failed`
  with the error recorded, never `pre-bootstrap` (LA-R1-010).
- fetch timeout (LA-R1-003): override the `ledgerAttentionFetchBudget`
  VAR (restored via `t.Cleanup`) with a remote whose transport blocks
  ⇒ outcome `failed` within the budget, NO surviving process in the
  child's process group, the accepted ref unmoved, and the tick's
  later duties still run.
- offline: unreachable remote path ⇒ attempt recorded, row alive
  under the threshold, `unknown` past it (injected clock).
- staleness red: movement, no clearing evidence, past threshold ⇒
  `dead` with the exact §5 row grammar; then a journal entry with
  `FetchedOid` at (or descending from) `remoteTip` clears it, while
  an entry with empty `FetchedOid` does not; separately, hook
  evidence whose successful attempt BEGAN after `remoteTipAt`
  clears it, while an attempt begun before `remoteTipAt` whose
  completion lands after does not (LA-R1-004: the
  started-early-finished-late turn).
- precedence: movement observed, then fetches fail; both mature ⇒
  the row is `dead`, not `unknown` (LA-R1-009).
- crash safety: surface, then reset `surfacedTip` (simulated
  pre-mark crash) ⇒ resurfaced; digest holds one entry (SourceID
  dedupe).
- validation refusal: a corrupted remote tip leaves the accepted ref
  untouched and records a failure (never-advance-past-validates is
  `FetchAdvance`'s own contract, already covered by
  `internal/goal/fetchadvance_test.go`; this test only proves the
  component's quiet handling).

## 8. Non-goals

- No new verbs for claiming, and no new CLI surface at all.
- No authority anywhere: no autonomous claiming, promoting, parking,
  or any goal mutation by the component.
- No changes to goal verbs, the CAS/transaction engine
  (`internal/goal/txn.go`), or the watch verb. The one addition to
  package `goal` is the read-only `AcceptedLedgerTip` accessor in the
  NEW file `internal/goal/attention.go` (§7.1); no existing goal file
  changes.
- No redesign of tick or watch machinery; the component is one
  additive attempt in `RunTick`'s existing pattern.
- No changes to `internal/steward/marks.go` — the stall-detection half
  landed (9781416) and this design only feeds it.
- No changes to the turn verdict — it inherits currency from the
  accepted ref (§2, destination 3).
- No model- or runtime-specific behavior; the ledger remains the only
  inter-machine channel.

## 9. Revision record — Sol round 1 findings

- LA-R1-001 — revised: "queue reordering" replaced by "queue
  departures" (§2); same-set reordering is impossible under the
  lexicographic `sortedGoalIds` order every reader shares; recorded
  decision, no other authoritative ordering source exists today.
- LA-R1-002 — revised: first run baselines to the PRE-fetch local
  world, so the first pass surfaces stale remote movement and starts
  the staleness clock instead of erasing the incident (§4).
- LA-R1-003 — revised: the fetch runs on a goroutine under a 60s
  budget with a single-flight guard; timeout is fail-quiet-recorded
  and the tick always proceeds (§3).
- LA-R1-004 — revised: clearing is tip-bound — journal
  `Entry.FetchedOid` ancestry, and hook `LastSuccess` against the new
  `remoteTipAt` field via accepted-ref monotonicity; the mtime and
  `movedAt` correlations are withdrawn (§5).
- LA-R1-005 — revised: the dead-row remedy names acts that produce
  the clearing evidence (a coordinator turn, a journaling verb) and
  states that the shipped `metasystem goal fetch` does not examine;
  the nonexistent `goal list --fetch` is removed everywhere (§5, §1,
  §3, §6).
- LA-R1-006 — revised: the report carries complete `Ready`/`Pinned`
  snapshots, `PersistLedgerAttentionMark` takes only the report,
  every guard path saves state before returning, and the no-fact
  advance happens inside the pass itself (§7).
- LA-R1-007 — revised as a recorded decision: dedupe truth is the
  surfaced frontier, not a tip chain; every fact surfaces at least
  once across crashes and tip advances; the single queue slot
  coalesces undelivered movements deliberately, and "one nudge per
  change" is defined as at-most-once per change with at-least-one
  covering every change (§4).
- LA-R1-008 — revised: change identity and facts both come from one
  `Projection` (its `Tip` field); `AdvanceResult.Tip` is never the
  identity (§4).
- LA-R1-009 — revised: explicit precedence, `dead` before `unknown`,
  with rationale and a test (§5, §7).
- LA-R1-010 — revised: the guard is the new read-only
  `goal.AcceptedLedgerTip` built on `acceptedTipForGates`, so broken
  accepted state is recorded as `failed` (maturing to `unknown`)
  instead of masquerading as pre-bootstrap; `goal.NewWorld` is
  explicitly rejected as the guard (§3, §7).

## 10. Unresolved residue — Sol round 3 (goal:ledger-attention)

The critique chain ran three verification rounds (10 -> 7 -> 7 material
findings). The two requirement-substitution findings are closed: the
design now serves the human-origin goal text. The seven findings below
remain material and unresolved; the goal's attempt and job-minute
budgets are exhausted, so per the declared failsafe the design lands
with this residue recorded verbatim rather than chasing convergence.
Each MUST be closed or refuted with evidence during the implementation
slice; three of them (LA-R1-006, LA-R3-001, LA-R3-002) mark parts of
SS7 as unimplementable as written and take precedence over the plan
text they contradict.

### LA-R1-003 (high)

The bounded-fetch closure still cannot guarantee that the whole transport process group is dead when the timeout returns. An implementer following the specified Cmd.Cancel and Cmd.WaitDelay combination will kill only the direct Git process during escalation, allowing a descendant that ignores SIGTERM to survive and contradicting both crash containment and the named test.

Evidence: metasystem/plans/ledger-attention-design.md:186-218 specifies process-group SIGTERM followed by Cmd.WaitDelay escalation, and lines 709-713 require no surviving process in the group. The installed Go 1.26.6 os/exec implementation calls c.Process.Kill when WaitDelay expires; that is a direct-process kill, not a negative-process-group SIGKILL. The design names no second process-group signal in its Cancel function.

### LA-R1-004 (high)

Hook clearing remains unsound because its proof still assumes the accepted reference is globally monotonic. The current goal engine has a sanctioned human repair that can rewind that reference, so a later-starting turn can read an older world and falsely clear the newer stored tip.

Evidence: metasystem/plans/ledger-attention-design.md:407-446 derives examination from an attempt beginning after remoteTipAt and states that the accepted reference only moves forward. metasystem/internal/goal/accepted.go:16-82 explicitly defines RepairAcceptRemote as a human-reserved path that bypasses descent, while lines 113-127 directly set the accepted reference with an old-value assertion. Thus attention can record T1, repair can move the reference to older S0, and a hook begun afterward can read S0 yet satisfy the proposed clearing predicate for T1.

### LA-R1-006 (medium)

The state-write failure outcome remains contradictory and unimplementable without a new discriminator. One section requires STATE_WRITE_FAILED, while the tick integration maps the same report shape to FETCH_FAILED; the proof asserts the former.

Evidence: metasystem/plans/ledger-attention-design.md:567-580 requires state-save errors to complete component evidence as ComponentError/STATE_WRITE_FAILED. Lines 638-647 require a failed LedgerAttentionReport to complete as ComponentError/FETCH_FAILED. The report at lines 584-587 contains only Outcome, Tip, Failure, Pending, and MovedAt, with no failure kind, while lines 697-700 require the test to observe STATE_WRITE_FAILED. An implementer must invent an interface field, parse error text, or choose one conflicting outcome.

### LA-R1-007 (high)

The revised per-change dedupe now honors the human contract in intent, but its history walk assumes a first-parent topology that the accepted-reference rules do not guarantee. It can generate events for nonconsecutive lineage states or silently skip a sanctioned rewind, so once-per-change behavior remains incorrect.

Evidence: metasystem/plans/ledger-attention-design.md:269-285 declares git rev-list --reverse --first-parent <diffedTip>..<captured> to be exactly the consecutive ledger-state sequence. metasystem/internal/goal/fetchadvance.go:91-106 proves only that the prior accepted tip is some ancestor of the captured tip, not its first-parent ancestor. A merge whose second parent is the prior accepted tip passes that gate, but the proposed walk includes commits from the other first-parent lineage. Separately, metasystem/internal/goal/accepted.go:16-82 permits a human-authorized rewind, after which captured may be an ancestor or non-descendant of persisted diffedTip; the design specifies no recovery or event identity for that transition. Its write point W4 at design lines 286-296 and 612-619 persists pending events and advances diffedTip before the accepted-reference update, worsening this mismatch across a crash or compare-and-swap refusal.

### LA-R2-001 (medium)

The third destination is still overstated: a queue or pin change does not re-block a coordinator session that already has a claimed goal. Keeping the turn verdict unchanged therefore does not provide the promised once-per-change block in a common state.

Evidence: metasystem/plans/ledger-attention-design.md:132-148 says a surfaced queue-digest change re-blocks the next turn exactly once and requires no turn-verdict changes. In metasystem/internal/goal/turnverdict.go:446-479, a local claimed goal produces status ok. The switch at lines 387-408 then checks only that claimed goal's revision; queuedFrontier and its queue digest are consulted exclusively in the queued-only case. A pin to another queued goal therefore changes the queue digest but produces no session re-block while current work is claimed.

### LA-R3-001 (medium)

The implementation plan does not provide a callable nonce source for bounded capture. Package steward must supply CaptureTipBounded's operation identifier, but the only matching generator is private to package goal, leaving Sol to change an interface or invent duplicate generation rules.

Evidence: metasystem/plans/ledger-attention-design.md:550-551 defines CaptureTipBounded with a required opid string, and lines 605-606 direct RunLedgerAttention in package steward to create a per-operation nonce. metasystem/internal/goal/fetchadvance.go:123-129 defines readNonce as unexported. A repository-wide search found no exported goal nonce generator. The implementer must either export a new function, move generation inside CaptureTipBounded and change its signature, or duplicate the read- plus twelve-hex-character format in steward; the supposedly zero-judgment plan names none of these.

### LA-R3-002 (medium)

The revised proof plan still targets fields deleted by schema 2, so two required tests are not buildable as written.

Evidence: metasystem/plans/ledger-attention-design.md:302-322 defines pending events with a pins field and no surfacedTip. The report at lines 556-561 likewise exposes Pending events, not NewPins. Nevertheless, lines 680-682 require a pin to appear in NewPins, and lines 726-728 simulate a crash by resetting surfacedTip. Implementers must decide whether to inspect Pending, delete an event, skip write point W6, or construct another crash seam; those alternatives assert different behavior.
