# Design brief: ledger-reads-are-fresh

Working Mode: Design. This is a small follow-up: one fresh-ledger-decision unit using the existing projection, bounded transport and publication owners. It is split from `plans/designs/lane-drain-and-fresh-claims.md` after round 2 returned three material findings, down from six, with finding 1 repeating the production-caller class in Decision 2. The parent's stop rule ends that section's critique there; this brief preserves its text and required correction, without opening a ledger goal or authorizing implementation or spending.

Step 1 makes claim, internal next and session adoption use an accepted snapshot observed for that decision, with the hook's time limit respected. No new daemon, cache, records ref or publication batching belongs here. The parent proceeds with today's reads; stale projections can delay adoption until the next session start sees the reservation, while each mutation still validates ownership in its publication transaction.

## Round-2 finding 1: the synchronous hook caller has a smaller budget

**Material; accepted and split.** Putting `FreshProjection` inside the shared restamp callback reaches a synchronous production caller: `internal/hooks/runtime_hook_start.go:625` calls `ops.Up` and cannot checkpoint again until it returns. The repository-root `.claude/settings.json:30` gives SessionStart 15 seconds; `internal/goal/project.go:46` sets `defaultFreshProjectionTimeout` to 4 seconds (its surrounding comment concerns the different, 60-second Stop hook). The removed Decision 2 allowed an environment retry without allocating the session-start hook's remaining budget. Two attempts plus session preparation, restamping and rendering can consume the hook deadline before it reports pending adoption. Wiring all callers is insufficient without respecting their execution budgets; that production-caller class repeats round 1's finding 4.

**Required change, overriding the preserved text below.** On the hook path use one fresh-read attempt only, capped at **4 seconds including transport cancellation and cleanup**, at most 4/15 (about 27%) of the 15-second hook budget. Leave 11 seconds for the rest of the hook, and shorten or skip the attempt if the remaining hook deadline cannot accommodate that reserve. Propagate the hook's deadline through the shared callback; never start a second attempt or an unbounded wait on this path. On timeout, return partial preparation with **adoption pending**, its transport cause, and **`metasystem session start`** as the remedy; do not report adoption success or silently fall back to stale adoption. The design must account for preparation, subsequent publication and output within the enclosing deadline. Explicit session start and up retain their own declared budgets; the hook does not inherit their retry allowance.

**Required test.** Drive the real session-start hook entry with a hanging transport, using injected time and transport boundaries. Assert one attempt, cancellation and temporary-ref cleanup within the 4-second allowance, return within the 15-second hook budget, and visible adoption-pending output with the exact `metasystem session start` remedy. Follow that command with a repaired transport and observe adoption under unchanged ownership/accounting. A second hook attempt, ignoring cancellation, or dropping the pending result must turn the test red.

## The shared callback's three production callers

- Session-start hook: `cmd/metasystem/hook_entry.go:356`, reached synchronously through `internal/hooks/runtime_hook_start.go:625`.
- `metasystem up`: `cmd/metasystem/up.go:239`.
- `metasystem session start`: `cmd/metasystem/intent_process.go:682`.

Claim and internal next also consume fresh decisions. Cover all three adoption entrypoints as well as claim's post-preparation reread; changing only public session start leaves the hook and up stale.

## Five design questions

Answer each for every new function, record and act, including the shared callback, deadlines and pending result:

1. Who calls it in production?
2. How fresh is every state a decision reads?
3. Is the actor a person or an agent, with the helm reserved to a person and refusal of a person limited to preventing damage?
4. Can every refusal's remedy succeed when followed?
5. Does every unreadable input fail safe for the agent without vetoing a person's advisory override?

## Removed Decision 2, preserved verbatim

This is the source proposal, not an accepted design. Its retry/budget wording is superseded by the required correction above. References to U4 and U5 name the parent's personal-claim adoption and claim-preparation units.

## Decision 2: the three decision edges own a fresh accepted snapshot (U2)

**Sites.** `cmd/metasystem/intent_planning.go:1084` currently selects before actor handling; `cmd/metasystem/goal.go:428` makes next's fetch optional; `cmd/metasystem/intent_process.go:665` enters session preparation. `cmd/metasystem/intent_goals.go:27` is the optional-fetch display reader. `internal/goal/project.go:126` projects the accepted tree; `:178` wraps fetching in an outer timer. `internal/goal/fetchbounded.go:24` bounds and cleans up the transport. `internal/goal/fetchadvance.go:20` already returns the accepted tip. `internal/goal/txn.go:662` owns publication and its fresh transaction.

Add `goal.FreshProjection`: use `FetchAdvanceBounded`, then a small new pinned-tip variant of the projection loader at `internal/goal/project.go:137`, reading its returned `AdvanceResult.Tip`, with approval horizon at the observation time. Return the immutable projection and fetch observation (tip, time, outcome/cause) in the command's existing diagnostic result. Reuse the existing read-fetch duration; no extra timer, daemon or long-lived freshness cache. The network operation must have exited and its temporary refs been cleaned up before a timeout returns. Local mode loads the current local accepted tip without a network operation, retaining sync-mode and identity validation. A successful fetch of an unchanged, old commit is fresh observation; do not confuse commit age with failed freshness.

`goal next` is the internal command `metasystem internal goal next`.

After proving the actor, claim fetches before selecting or returning quota/held-work results; next fetches before ranking; session start fetches before any goal-based selection or adoption. Run `FreshProjection` inside the shared `restampStopCapabilityForUp` callback so public session start (`cmd/metasystem/intent_process.go:682`), the session-start hook (`cmd/metasystem/hook_entry.go:356`) and `metasystem up` (`cmd/metasystem/up.go:239`) all fetch before adoption. Pass each decision's snapshot to its downstream readers instead of having them immediately call `Project(false)` again; the callback owns the adoption snapshot for all three entrypoints. The display reader stays optional-fetch. `--fetch` remains compatible on next but is no longer necessary. Claim publication and every restamp still capture and reevaluate against the transaction's current tip; preflight freshness cannot replace that race check. U5 refreshes again after session preparation because ownership may have changed during it.

Transport timeout/failure means unknown freshness for an agent: no automatic selection, claim publication or adoption. One environment retry is allowed in this invocation under the existing retry allowance; after the second failure report its cause and stop, not an empty frontier. Validation/foreign-ledger failures are not retried as transport. A person's next can display the last accepted snapshot marked stale and non-authoritative. A person's **named** claim can proceed to the normal publication transaction despite preflight transport failure; that transaction must fetch and publish safely. An unnamed claim cannot guess a candidate from stale data: show it as stale and invite an explicit name, whose transaction still validates it. A person's session start may prepare supervision/the real discovered session while reporting unavailable backlog and disabling automatic goal selection/adoption. No synthetic session is created when no runtime ancestor exists. Ledger corruption is a shared-ownership safety failure; it never becomes permission to publish invalid data.

**Five answers.** `FreshProjection` and its observation are consumed by claim, internal next, shared restamp through session start/the hook/up and U5's post-preparation refresh; their production wiring ships in this unit. Each decision uses one just-fetched immutable tip, each mutation rechecks in publication, and time-based approval decisions use the current observation. Agents require fresh data; proved people may read stale data or prepare a session but never claim offline. The retry remedy is the same failed public command after the named transport problem is repaired, not a manual `goal list --fetch` prerequisite; a person supplies a goal name only when automatic selection is unavailable. Unknown freshness holds agent effects; unreadable advisory banners cannot veto a person's act, while an unreadable ledger/identity blocks unsafe shared mutation and reports the concrete source and repair requirement. No successful selection is reported on an error.
