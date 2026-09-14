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
frozen judgment, and emits one compact human line of at most 256 UTF-8 bytes.
For the `shared-reason-v1` candidate envelope a block is exactly
`{"decision":"block","reason":"<line>"}` and an allowance is exactly
`{"systemMessage":"<line>"}`. A block must not also carry
`systemMessage`; the compact line flags a required human decision or
supervision repair, and the report retains every cause, owner, remedy and
seat action. Presentation failure cannot create or clear a block.

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
--fetch`; a held seat continues without another claim. An allowed Stop adds
no report-read turn. This instruction is a recovery route, not evidence that
the host displayed or enforced the notice.

## Conformance table (the DISTRIBUTION, not any installation)

The registry carries only the candidate expectation. Installation and host
observation live in dated `capabilities.stopDelivery` snapshots; no runtime
below is upgraded by this source change alone.

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
