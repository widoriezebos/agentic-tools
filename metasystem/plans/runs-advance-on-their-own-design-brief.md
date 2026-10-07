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
