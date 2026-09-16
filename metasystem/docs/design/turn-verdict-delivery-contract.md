# The turn-verdict delivery contract

The turn verdict is ONE engine decision — `report turn-verdict` returns
`{shouldBlock, blockSource, goal, ledgerStatus, display, ...}` — and this
contract says what an adapter must do to claim conformance for its
runtime. The mechanism is identical for every runtime; only delivery
automation differs, and the table below says so in public
(goal-system design, D66: exchangeability — any runtime fills any seat).

## What conformance means

An adapter conforms when its runtime's turn end preserves the engine's
`shouldBlock` and `blockSource`, publishes one immutable report from the
frozen judgment, and emits exactly two logical human lines in one native
field. The first says what newly recorded owned work completed and is at most
144 UTF-8 bytes. The second names the task state, outcome and exact report
command and is at most 256 bytes; the pair is at most 401 bytes including its
one LF.
For the `shared-reason-v1` candidate envelope a block is exactly
`{"decision":"block","reason":"<line>"}` and an allowance is exactly
`{"systemMessage":"<line>"}`. A block must not also carry
`systemMessage`; the task line flags a required human decision or
supervision repair, and the report retains every cause, owner, remedy and
seat action. The command uses a permanently reserved exact short alias while
the report retains and verifies its full identity and digest. Presentation
failure cannot create or clear a block and retains the established one-line
degraded form.

The same rule applies while a foreground `metasystem wait` command holds the
runtime. Every adapter must implement this exact operation:

```text
wait-delivery --wait-id ID --nonce NONCE --deadline RFC3339-UTC --session SESSION-ID
```

Acceptance prints exactly `blocking` followed by one newline and exits zero.
Exit 2 declines blocking delivery. The Go adapter port turns that exit into the
named `ErrWaitDeliveryDeclined` refusal; it does not publish a pending wait, and
the ordinary turn-verdict decisions remain in force. No adapter output is
evidence that the watched source ended. The version-2 waiter row and a fresh
source observation remain the evidence owners.

Conformance also requires dated evidence for the actual host version,
effective configuration and instruction bytes, trusted hook firing, total
human transcript, exact report lookup from the seat's command environment,
both block and allowance behavior, and the finite continuation limit. A
source fixture or static declaration proves emission shape only. Missing or
stale observations remain unobserved.

## The universal fallback (no hooks required)

On a blocked Stop, the instruction entrypoint requires the seat to run the
exact `metasystem report stop-status --id ...` command before another work
action or Stop. A free seat then uses `goal next --machine <own-nick>
--fetch`; a held seat continues without another claim. On an allowed
report-bearing Stop, the task line itself says `Read, then continue lawful
work before stopping:` before the same exact command. This instruction is a
recovery route, not evidence that the host displayed or enforced the notice.

## Registered waits at turn end

The gate reads version-2 pending rows only from the resolved installation
state root's `artifacts/agents/waiters` directory. The live Stop session is the
authenticated session: the current lease and its one matching main
announcement prove the holder and lineage without deriving a session from the
announcement. A row affects the verdict only while both row sessions equal the
live Stop session and its main lineage, lease epoch, exact process birth,
deadline, boot clock, observation freshness, selector, source incarnation and
claimed goal all still match. A failed check is the same as no row. Unreadable
gate inputs, a closed checkout fence and attended-human stop authority keep
their existing precedence.

A valid job, run, proof-attempt or landing wait is work in flight. It covers
its own unwatched job or exact governed run, suppresses open work only when the
saved open-work signature still matches, and resets the idle-backlog refusal
counter exactly as a live delegate job does. An unrelated job or run remains
unwatched.

A human-launched run has no main coordinates. Its separate watched signal is
accepted only from the pending human-owner row for that exact run incarnation
when the waiter remains alive at its platform-exact recorded birth. This
signal prevents an unwatched-run warning; it grants no registered-wait
allowance.

A valid human-act wait is narrower. It suppresses matching-signature open work
only while the current claimed goal is also reported as waiting on a human. It
never exempts idle backlog. Channel answer waits use this rule. New open work
has a different signature and therefore keeps the ordinary block; the waiter
returns exit 6 when it next observes that change. New or revised claimable
work likewise clears the pending row with exit 6 before the seat takes its
claim turn. Backlog already present at registration does not end the wait.

Every effective row contributes a visible `WAITING` line naming its target and
deadline. A Stop verdict obtains these lines only from the gate; raw recovery
orientation remains available to session start and goal next without deciding
the Stop. `TestPendingWaitTurnVerdict`, `TestPendingWaitIdleBacklog` and
`TestWaitDeliveryContract` are the executable conformance boundary. A future
adapter must pass all three; a native notification may only accelerate the
next source read.

The `wait-stop-fake` bed drives both the installed plain command and the fake
Stop hook through `TestPendingWaitInstalledVerdicts`. It also runs
`TestPendingWaitFromChildShell`, which associates the runtime session through
the installed SessionStart hook, registers a wait from a child shell under its
own process identifier, and proves the matching and hostile Stop outcomes. The
`wait-stop-claude` bed additionally reaches a real Claude Code turn end while
the blocking command owns a pending row. Because that leg consumes a live
provider turn, the external runtime-bed orchestrator enables it with
`METASYSTEM_REAL_RUNTIME_BEDS=1`; the ordinary local supervision bed always
runs the fake leg.

## Conformance table (the DISTRIBUTION, not any installation)

The registry carries only the candidate expectation. Capability snapshots do
not carry Stop-delivery conformance evidence, and no runtime below is upgraded
by this source change alone.

| Runtime | State | Evidence |
| --- | --- | --- |
| claude | candidate `shared-reason-v1`; unobserved | Shipped mapping and fixtures do not yet prove the new compact transcript, report read, trust state, or eight-block continuation boundary on a real host. |
| codex | candidate `shared-reason-v1`; unobserved | Shipped mapping exists; effective hook trust, transcript, report read, duplication and continuation limit still require a real-host observation. |
| devin | mapping unknown; unobserved | The Stop envelope, public fields, report-read route and continuation limit remain unknown; no conforming mapping is claimed. |

The fake runtime carries the shared mapping only so repository fixtures can
exercise the complete seam. It is a synthetic harness, not host-conformance
evidence.

Which runtimes THIS checkout installed is answerable from
metasystem.conf and is not this table's job. The instruction audit
checks this table's claims against the shipped enforcement configs.
