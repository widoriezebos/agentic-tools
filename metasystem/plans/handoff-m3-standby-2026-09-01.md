# Handover: m3 to standby — 2026-09-01

Wido's order: Fable capacity is exhausted, m3 goes to standby, m0 (or
m0b) takes over. Everything m3 produced is pushed to origin/main; this
file is the successor's cold-start briefing. Nothing below depends on
m3's session context.

## Read first, in this order

1. `records/misc/idle-loss-2026-08-31.md` and
   `records/misc/idle-loss-2026-09-01.md` — the two incidents that
   shaped today's laws.
2. `memory/rulings.md` rows R-31-m3 (shared-machine load leniency),
   R-35-m3 (no machine reservations; load-fragile caps are defects),
   R-36-m3 (policy decisions get the critique loop), R-37-m3 (standing
   re-arm after engine rebuild, all machines), R-38-m3 (m3's overnight
   envelope — EXPIRED with this handover), plus m2's R-38-m2: THE
   LADDER LAW — nothing gets built without a proper backlog item, and
   everything on the backlog gets critiqued. No exceptions for size or
   urgency.
3. `docs/seat-communication.md` — how every message to Wido is written.

## THE BLOCKER the successor must clear first

`alert-escalation-channel` is CLAIMED BY m3 and BREACH-STOPPED
(stop-alert-escalation-channel-r18-f1). Release is refused; only
`metasystem goal resume --id alert-escalation-channel --by wido`
clears it, and that is a human act. Until it clears, m3's claim
occupies m3's quota slot — harmless while m3 is standby, EXCEPT that
the goal itself cannot be worked by anyone. Two paths:
- Wido runs the resume (then the successor may claim it), or
- m2's `breach-clock-and-budget-honesty` chain lands, which makes
  release lawful on a breach-stopped claim (that fix exists because of
  this exact wedge).

## What m3 owns and hands over (all queued, unclaimed, briefs on main)

- **critique-always** — flip `independentCritiqueRequired` to true for
  the MECHANICAL row in BOTH `scripts/agents/role-packets.json` and
  `requiredConfigurationByHazard` in `internal/dispatch/hazard.go`
  (they change together per the exact-equality law), so every chain
  needs an independent cross-family critique at closure. Carries
  Wido's verbatim order. Specimens: four uncritiqued MECHANICAL
  landings of 2026-08-31. COORDINATION: m2 owns the sibling mechanism
  `design-gate-at-dispatch` (refusing design-bearing builds without a
  certified design) — same neighborhood, distinct mechanism; whoever
  lands second rebases on the first.
- **failed-job-attention** — the steward raises escalation episodes
  when a delegate job under a claimed goal is terminally failed, and
  when a breach-stop fires (with the exact resume command in its
  text). Brief at `plans/failed-job-attention-brief.md` is INPUT
  MATERIAL ONLY: under the ladder law it needs a design round and a
  Sol design-critique before any build. This is the fix for the
  six-hour idle: health already knew for nine hours, nothing carried
  it.
- **supervision-hook-wrong-root** — the harness hook derives the git
  toplevel on nested checkouts, so on m3 it reported the OUTER repo's
  bootstrap world every turn and hook-freshness has been dead since
  enrollment. Verified by hand: running
  `bash scripts/agents/supervision-hook.sh claude stop` from
  `metasystem/` prints health for the outer root. CONFIRMED
  INDEPENDENTLY by m2 the same hour — its hook-freshness is dead
  identically — so this is the shape, not one machine: every seat's
  harness turn signal has been false since enrollment. Check m0's
  hook-freshness first; it is likely the same and the goal's fixture
  should cover the nested layout generally.
- **alert-escalation-channel** — design COMPLETE at revision 6
  (`plans/alert-channel-design.md`), critique register closed at
  revision 5 with zero findings, then revision 6 added section 11a
  (the slice-1 mechanical specification) answering the implementer's
  seven-gap stop. Slice 1 (channel core + working unthreaded Telegram
  adapter) is specified and unbuilt. Wido's binding design words:
  adapter abstraction (email/Slack/Telegram/WhatsApp by configuration
  alone), Telegram first, the session bridge as a second consumer,
  Slack threading through conversation identity. NEW and unfolded:
  Wido's word that the alert classes MUST include "delegate job failed
  under a claimed goal" and that the breach-stop's own stop must be a
  slice-1 producer — see `plans/alert-channel-fold7-brief.md`, written
  and never dispatched. A live Telegram send needs a bot token from
  Wido in local config; everything up to the wire is provable against
  the design's fake endpoint.
- Drafts awaiting the route, not goals: `law-becomes-software`,
  `provenance-anchored-rearm`, `dispatch-cap-necessity` (all in
  `plans/goals-drafts/`).

## Fleet state at handover

- m2: `breach-clock-and-budget-honesty` (mid-ladder, includes the
  wedge fix), `two-bars-for-changes`, `design-gate-at-dispatch`,
  `proof-harness-process-custody`.
- m0: `burn-without-delivery-tripwire` (first slice landed),
  `steward-tick-stop-on-failure`.
- Claims on the ledger are the anti-duplication mechanism: claim
  before touching, one claim per machine.

## Traps this seat hit — do not repeat

1. **Consume the watch verdict.** `dispatch.sh watch --job <id>` (Go,
   `internal/dispatch/watch.go`) exits 0 completed / 1 failed / other
   codes for other outcomes. Never pipe it through `tail` and lose the
   code; never wait on a worker's output artifacts — a dead worker
   writes none. Both idle incidents were this.
2. **Allowed is not intended.** Where the machinery permits something
   the ladder forbids, follow the ladder and open a backlog item for
   the gap. Order of authority: Wido's word, the paper, the rulings,
   the machinery's refusals last.
3. **Existing owner before any new surface.** The system once grew
   hundreds of verbs this way; a new verb/script/mechanism is
   design-bearing by nature.
4. **The narrator digest races every landing.** Expect a conflict on
   `records/narrator-digest.log` on almost every rebase; resolve as a
   union in timestamp order (both sides are real appends) and always
   re-stage it. `digest-landing-race` is the standing goal for it.
5. **Landing mechanics on this clone:** land with `--staged-only`,
   stash the untracked `../m3-seat-brief.md` first (the gate refuses
   unstaged bytes), and pass `--skip-transport` — this clone has no
   `transport` remote configured, so the mirror step fails 128 while
   the origin push succeeds. Wido has been asked twice for the
   transport URL; still open.
6. **Engine rebuilds:** after pulling a landing that changes engine
   contracts, rebuild `bin/metasystem` AND re-arm the steward under
   R-37-m3 (`steward arm --repo . --temporary-human-word "<order>"
   --review-by 2026-09-06`), then `metasystem up`.

## Open questions for Wido (none blocking)

- The alert channel's bridge RECEIVE loop: this design deliberately
  reserved it for `seat-mutual-awareness` (five enumerated
  obligations). If he meant one design to own the full bidirectional
  bridge, that is a scope change.
- The transport remote for this clone (or "m3 skips it").
- Devin CLI is not installed on this Mac; its capability snapshot will
  stay flagged until installed or the roster drops it.

## Standby state

m3's steward is armed and live (temporary enrollment, review
2026-09-06). If the machine is to be fully quiet, the human stops the
runner from an agent-free terminal; m3 has NOT killed it, because a
seat does not disarm its own supervision.
