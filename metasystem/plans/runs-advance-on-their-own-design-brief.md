# Design brief: runs-advance-on-their-own (plan goal 4)

Working Mode: Design
Opened 2026-10-07 in Wido's word. Read first: `plans/machinery-open-findings-plan-2026-10-06.md` findings 5 (the driver), 10 (idle gaps: 59 gaps over ten minutes, 38 idle hours in four days, the ten largest all "read ended, next build not started"), 16, the mechanisms page (the driver as the boundary's consumer, the driver policy `seat.driver`), the practices table rows that name goal 4, and the "Learned 2026-10-07" section. Depends on goal 3's boundary event and goal 2's round records. SIZE: each unit at most 250 production lines, the goal at most 5 units.

## Intent

The driver consumes each step's end and starts the next step without a person: build, attest and read, correction, commit, next unit, hand-in; under `seat.driver=person` it asks instead.

## Must deliver (from the plan and from 10-07)

1. The driver as the consumer of the boundary event: one next step per record, idempotent (a repeat whose effect holds is a success).
2. The bounded waiter as machinery: no finished job stays unread (practice row; 1a: about 120 idle minutes).
3. **Main merged before each unit's build (10-07):** when origin/main moved since the branch's last merge, `work build` merges it into the goal branch before the builder starts, so conflicts are small and met by the builder; the integration tree is proven as before (person-claims on 10-07: 7 conflicts and a merge job at integration because U1 was built from a main that then moved).
4. **Each unit built from the goal branch's current tip** and integrated right after its read (practice row).
5. **Capacity-aware start (10-07):** the driver starts a build or a suite only within goal 3's load policy; otherwise it queues and says so.

## Not in this goal

Host state and the limit marks (goal 3). The stop rules and the round records (goal 2). The scaffold (goal 6).

## Shape

As the accepted designs (see plans/fleet-survives-its-providers-design-brief.md, "Shape").

## Added 2026-10-08: pending capacity steps, resume and recovery

Destination of the pending-step part of old U4 in [fleet-survives-its-providers](designs/fleet-survives-its-providers.md), **decided by m1e for Wido, 2026-10-08, reversible**. The fleet design retains P4's minimal completed-unit event and its headless `session handoff` consumer, and P5's `Manager.Start` admission from current load, `host.builds=auto|N|person` and declared `host.load-max`. This goal consumes those results; it does not rebuild provider state, load policy, the boundary or the session end.

Extend the existing step record/driver at `internal/launch/read_sequence.go:37` with retained capacity waiting: operation/goal/unit identity, original exact command/target/authority, wait start/end and reason. Today a start error before a launch record exists leaves StepStarting; it must not become a completed unit, failed execution, correction round or spent command deadline. Own the pending-step queue, public work status/re-entry, automatic resume and cancellation/recovery here. One operation has one effect; after a wait, re-read current policy/load, registration and exact target/authority before asking Manager.Start to reserve it. Cancelled or completed work cannot launch again; an unknown process cannot be reaped as dead. Keep queue minutes separate from executed build/suite minutes for the process-cost reader.

The boundary is keyed by seat/session/goal/unit/completed outcome and bound to its durable handoff. Consume it once, after the predecessor handoff/end and the applicable settings/engine/budget owners have completed their boundary actions; do not make a generic acknowledgement framework. A queued step is pending, never a boundary. Reuse existing launch/step reconciliation and bounded waiting, including crash between reservation and supervisor start, rather than adding a poller to the fleet goal. Under `seat.driver=person`, prepare the exact next act and wait; a person's explicit act retains its existing authority.

Acceptance drives public work build/status with interleaved capacity refusal, later capacity, repeated delivery, changed target, revoked authority, cancellation and restart. It proves one start, no lost pending work, no false completed boundary and no correction charged for environment waiting. Removing the real retained-step adapter must fail that test. Re-estimate this transferred work with the rest of the driver design within five units of at most 250 production lines.
