# Unit u7 of lane-first-run-fixes: a running trunk proof defers the trunk-red question

Working Mode: Implement. Design: `plans/lane-lands-finished-goals-design-brief.md` (the lane holds on main's red and asks the person only when nothing of its own can clear it); measured defect: finding 53 of the lane test (2026-10-10 12:57). Size: at most 120 production lines.

## Measured defect

At 12:56 the lane started its trunk proof of the new main (`landing prove --trunk`, attempt 20261010T105655). At 12:57 the landing agent, woken for the held batch, read the trunk-red register (main red since the morning's replay) and asked the person two channel questions for a `GOAL_LAND_TRUNK_RED` exception, one per waiting goal. The proof it was not waiting for proved main green at 13:51, and the keeper then withdrew both questions as stale (u6). The questions were noise: the lane had the means to clear the red itself and was already doing so.

## Behaviour to build

1. `landing status` (internal/landing/plain/status.go, the headline around the trunk-red branch at ~line 381 "hot-fix, then metasystem landing prove --trunk"): when the trunk-red register marks main red AND a trunk proof attempt is running (the running record with the trunk flag, as `last_proof.trunk`/the running proof already expose it), the headline says `main <sha> red; its trunk proof runs since HH:MM (attempt ID); wait` and offers NO person act and NO exception remedy. When no trunk proof runs, the headline stays as it is.
2. The landing agent's skill (skills/landing-agent/SKILL.md, case 3 / the "A `main` cause" sentence near line 66): with a trunk proof running, the agent ends its turn without asking; the keeper wakes it when the proof ends. Asking a person a trunk-red question is allowed only when no trunk proof is running and no hot-fix is in flight.
3. The keeper (the `landing agent` start decision in cmd/metasystem or internal/landing, where an open question holds the start): a running trunk proof holds the agent start for a trunk-red batch the same way an open question does, so the agent is not woken into a red it cannot act on; the status line names the reason (`held: trunk proof <attempt> running`).

## Tests

- status: a fixture with the trunk-red register red and a running trunk attempt yields the wait headline and no person act; the same fixture without the running attempt yields today's headline (both asserted).
- keeper: with a running trunk proof the step does not start the agent for the trunk-red batch and prints the hold reason; after the proof record ends, the next step starts it.
- skill: the existing skill-reading test(s) (`TestSkillLandingAgent*`) updated for the new sentence; every phrase on the traveling surface qualified (the audit `internal/audit` refuses bare "this gate"/"its job").
- Run every existing test touching the changed seams (status headline tests, keeper tests, skill tests), `go run ./cmd/devgate static`, and `go test -count=1 -timeout 30m ./internal/audit ./internal/landing/plain` plus the cmd tests by name.

## Rules

Never edit testing.json; never open metasystem.conf.local; no new verbs; the person is never refused anything (a person may still run the exception by hand). Report: diff --stat, each check's exit, the headline texts.
