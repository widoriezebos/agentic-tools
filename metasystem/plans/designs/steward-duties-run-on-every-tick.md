# Design: the lane's steward duties run on every tick, whatever the keeper says about its agent

- Id: 01M4Z1STEWARDTICK00000000A1
- Kind: design
- Status: accepted
- Goals: steward-duties-run-on-every-tick
- Evidence: lane test finding 323 (2026-10-15 03:12, the runs-advance extraction's accountable read); interventions S150–S151.

## Problem

The lane's keeper tick first decides about its landing agent, then runs the steward's duties (stale questions, incidents, flake fixes; `plain.StewardStep`). `landingStewardTick` (`cmd/metasystem/landing_agent.go:490`) returns before those duties whenever the keeper's outcome is `AgentNotKept`, `AgentFailed`, `AgentStarted`, or a launch that is not `AgentRunning`. Since the runs-advance extraction (0b48a1829), a pending reservation the keeper cannot claim makes it report `AgentFailed` ("the landing agent's keeper can't claim the start", `internal/landing/lane/agent.go:325`), so every tick skips the duties until a person runs `work stop`; before, the silent-agent row self-healed after 20 minutes. Separately, the keeper's recovery of a pending reservation on its own identity (`Unclaimed`, `lane/agent.go:399`) is wired in production at `landing_agent.go:398` (`Unclaimed: agent.unclaimed`) and no test exercises that wiring: removing it passes every test.

## Decisions

D1. The steward's duties run on every tick of the lane the steward keeps: only `AgentNotKept` (not this steward's lane) skips them. A failed, held or just-started agent never silences the duties; the tick's line carries both the keeper's word and the steward's.

D2. A reservation the keeper cannot claim is reported as held, in intent words ("the landing agent's start is held: <plain reason>; it is retried at the next wake"), not as failed; the tick's outcome for a held start is `AgentHeld`.

D3. The production keeper wiring (`newLandingAgentKeeper`: Running, Unclaimed, Start, Reap, Cancel) is proven by one test through the production constructor with a pending reservation on the keeper's own identity: the start is recovered; dropping `Unclaimed` from the wiring fails the test.

D4. UX: no internal outcome names in what the person reads; the health row for a silent agent keeps self-healing.

## Units

- U1 (D1–D4): brief `evidence/briefs/steward-duties-run-on-every-tick-U1-brief.md`. ≤100 production lines.

## Not in scope

The keeper's start decision itself, the reservation format, and the minors of finding 323 (fence check duplicated with Supervise; batch identity check inert in automatic mode; failed starts keep their brief; pre-binary reservations hold until `work stop`), which stay recorded for a later goal.
