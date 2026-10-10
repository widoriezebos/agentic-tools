# Task: revise the design fleet-survives-its-providers after critique round 2

Working Mode: Design
Revise plans/designs/fleet-survives-its-providers.md in place (keep Status: draft). Round 2 (Opus) found 4 material findings; fold each with a concrete change and add a "Round 2 findings and changes" table. Cap: unit <=250 production lines, <=5 units.

1. P2: the landing agent writes the old per-checkout mark (cmd/metasystem/landing_agent.go:183, outage.Record) and reads it to hold its start (:307, outage.StandingAt). Add both to P2's moved writers and readers and to the caller inventory; the P2 test drives a landing-agent limit result.
2. P2/P3: name a person's act that clears one provider's mark (a person's `machine` act, or a person's start that succeeds clears its provider), never refused; keep the 2-minute probe (internal/steward/runner.go:454, :497) whenever the reset is unknown or beyond outage.Horizon; a mark never pauses a person's act.
3. P5: StartSpec (internal/launch/launch.go:50) has no actor; name the field `work build` fills after its existing actor check, and limit the load/cap check to Kind == build (reads, design and the landing agent through Manager.Start are never held by load).
4. P4: name who ends the headless session and how (the instruction to stop at the boundary, plus the steward signalling the predecessor once the handoff binding is durable); emit the boundary event for every seat and bind the handoff only for headless seats; the P4 test asserts both (goal 4 needs the boundary under seat.driver=person).
Also (borderline, fold it): P1 fixes the suite serialization centrally (a compiled default of 1 full suite per host, retiring the R-111-m1e temporary expiry for this cap, internal/proofrun/admission.go:15-21, metasystem.conf:101) so it does not depend on each checkout's configuration.

Do not touch code, memory/, records/ or the ledger. Never open any metasystem.conf.local. Return: units with estimates and what changed per finding.
