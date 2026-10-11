# Lane and seat share one state: stop records, enrollment and the roster

- Kind: design
- Id: 01M4KVVG3D2AXR9H0WHN2FT89K
- Status: accepted
- Critique: Astra round 1 (2026-10-10 23:4x) stopped at R-121-m0 (first slice undefined); round 2 (00:0x) material 0 for step 1 (D2 shared enrollment); SS-01 wording fixed. Accepted for step 1; D1/D3/D4 re-critiqued when they are next.
- Goals: shared-state-in-the-ledger
- Requested: by Wido on 2026-10-10 23:35 ("execute this"), item 4 of the brittleness diagnosis

## Problem (measured, lane test findings 61, 72, 72b, 77, 80, 82, 84, 86)

The seat (agentic-tools-m1e) and the lane (agentic-tools-landing) are two installs of one repository. Each has its own metasystem.conf.local, artifacts and enrollment proof. On 2026-10-10: the lane ran its build jobs on Opus because the roster lived only in the seat's conf.local (twice: role.* keys set, then launch.* keys); a person's `goal resume` failed on the seat because the stop batch record was in the lane install's artifacts/agents/goal-stops; `goal resume` read a stale accepted-ledger projection on the seat while `goal done` read the goal file; and one enrollment per machine made lane acts and seat acts take turns ("Wido enrolled another one"), so a landed goal could not be concluded from the lane pane. Every one of these cost an intervention.

## Decisions

D1. **Stop records are ledger records.** The breach-stop custodian writes the stop batch proof into the ledger (records/stops/<stop-id>.json, committed and pushed with the breach-stop act) and the goal file keeps the fence; `goal resume` and `goal budget` prove the stop batch complete from the ledger, from any install; the install-local artifacts/agents/goal-stops stays as a cache only. Existing fences are read from either place during migration.

D2. **One person, every install on the machine.** The enrollment proof names the machine and the terminal; a person act on ANY install of the machine resolves the person from the machine's enrolled terminal (the proof is read from one machine-level location, not the install's artifacts). `landing start/stop/return`, `goal done`, `work land` and the question verbs all use it; the "Wido enrolled another one" refusal disappears for installs on the same machine. An install still records its own custody; only the WHO comes from the machine.

D3. **The roster follows the registration.** `landing set` copies the registering seat's launch.* roster keys (runtime, model, effort, sandbox) into the lane install's conf.local at registration and whenever the seat's roster changes (`settings set` on a seat with a registered lane offers `--lane too`, default yes for launch.* keys); `landing status` shows the lane's roster and warns when it differs from the registering seat's. Secrets never move; only the roster keys.

D4. **Projections fetch before a person acts.** Every person verb that reads the ledger projection (resume, done, budget, approve, claim, return) fetches first, as `goal resume` now does (fence hot-fix 24572665e); the fetch is one call per act, bounded by the existing deadline.

## Threats and nets

- A stop record in the ledger could be edited by hand: the goal file's fence stays the authority; the record is checked against the capability and fence fields and must read COMPLETE, as today (stop.go ~229/296; no record hash exists today, so D1 adds one when it moves the record).
- Machine-level enrollment widens a person's reach to every install: that is the person's own machine; agents never gain it (the proof still requires the human terminal).
- Roster copy writes into another install's conf.local: only declared launch.* keys, never secrets, and only at a person's act.

## Step 1: the smallest thing that works first

D2 alone: one person for every install on the machine. Migration: an install that holds only the legacy per-install proof promotes it to the machine-level file on first use (no re-enrol); `landing resolve` and `question withdraw` keep their agent audience; `landing stop` stays open to agents (only `landing start` is the person's). The enrollment proof is read from one machine-level location, and the named person verbs (goal done, goal resume, goal budget, work land, landing start, landing stop, landing return, question withdraw) resolve the person from the machine's enrolled terminal whatever install they act on. This removes the two-pane dance that cost the most today (fence, conclusion, lane start) and needs no data migration. Acceptance of step 1: with the seat and the lane installs on one machine and one enrolled terminal, `goal done` on a landed goal and `landing start` both succeed from the same pane; the "Wido enrolled another one" refusal no longer appears between the two installs; an agent session still cannot pass as the person.

Deferred, in this order: D1 stop records in the ledger (U1, with a migration that reads either place), D3 the roster follows the registration (U3), D4 fetch-before-act for the remaining person verbs (U4). Until D1 lands, a person's resume of a fenced goal runs where the stop record lives (the lane install), as recorded in the lane test.

## Units (each at most 250 production lines)

U2 first as step 1 (D2); then U1. D1 stop records in the ledger (write, read from either place, prove from the ledger; migration note).
U2. D2 machine-level person resolution for the named verbs; tests across two installs of one repository.
U3. D3 roster follows the registration (`landing set` copy; `settings set --lane too`; status warning).
U4. D4 fetch-before-act for the remaining person verbs.

## Acceptance

With the seat and the lane on one machine: a person resumes and concludes a fenced goal from either pane; the lane's first build after `landing set` runs on the seat's roster; no "enrolled another one" refusal between the two installs. Stop rule: Astra critique to material zero, at most three rounds; one unit at a time, Opus read each, each unit rehearsed (lane-rehearsal-before-landing) before landing.
