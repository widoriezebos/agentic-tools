# g1-s45: what a seat is doing

- Kind: design
- Id: 01M3C1ESG2QVDTY8YWVMJ5PJTC
- Status: draft
- Goals: browser-interface seat-mutual-awareness

Wido, 2026-09-25: insight into what a machine is currently working on, the
delegates and what exactly, progress, maybe an ETA; "UX design this so it
fits neatly into the fleet page. Then build it." Headless seats are out of
scope by his word. Author Fable; every cite re-read at `2d1b77a1d`.

## 1. What exists and binds

1. **Delegates are records.** A seat's delegate jobs live under
   artifacts/agents/jobs, one JSON record each, read today by
   `seat.ReadJobs` for the keys `jobId`, `status`, `parentJob`, `role`,
   `round`, `goalId`, `createdAt`, `startedAt` (internal/seat/ledger.go:70-135).
   The lawful statuses are `pending-setup`, `pending`, `running` and the
   terminal ones (internal/dispatch/record.go:56-60). A job's reserved cap is
   `capRequest.minutes` on the record (dispatch/claim.go:763), and
   `settledJobMinutes(start, end, capMinutes)` is how dispatch counts what
   a job consumed (dispatch/budget.go:784).
2. **The goal's box and its consumption are projected.** `goalbudget.Budget`
   is the tuple on the goal (elapsed, attempts, reserved job minutes,
   active jobs, review rounds; internal/goalbudget/budget.go:82-88), a
   backlog `Row` carries it (internal/backlog/project.go:110), and
   `dispatch.ProjectConsumption(repoRoot, goalFile, now)` counts what the
   current accounting revision has used (dispatch/budget.go:275).
3. **Rounds.** A critique chain's round is read from its finding register
   (`findingRegisterRound`, dispatch/critique.go:124) and the limit is the
   goal's `ReviewRoundLimit`. A build lane's round is on the job record.
4. **Presence carries one chain.** The record's `chain` is the newest
   non-terminal job: root, job, role, round, goal, startedAt
   (internal/seat/record.go:50-57), composed by the tick from local jobs
   (`NewestChain`, publish.go). Readers accept a schema of 1 or more and
   ignore keys they do not know (record.go, D2 of the bridge revision 7), so
   a new key breaks no reader.
5. **The Fleet page** shows one Running column per machine from
   `runningWords(machine.running)` (src/fleet/FleetPane.tsx:361-385) and,
   for this seat, one "running" fact line (FleetPane.tsx:218). The payload's
   `Machine` carries `Running` (internal/ui/fleet/fleet.go:208-224). The
   phase the backlog projection names for a claimed goal is mostly "not
   recorded" (backlog/project.go:15, 217), so the chain's roles and rounds
   are the honest phase signal.
6. **The seat-communication law** (docs/seat-communication.md, rule 4):
   numbers wear units and meaning. And R-124: the smallest thing.

## 2. Decisions

- D1. **Progress is what the records prove; the ETA is a bound.** The page
  says the phase from the chain's roles and rounds, the minutes a job has
  used against the cap it reserved, and the goal's box used against its
  limits. "At most 79 minutes on this job" and "7 attempts left in the box"
  are the only forward-looking words, because the kit measures and bounds
  and never forecasts.
- D2. **Names and numbers only.** Goal ids, roles, rounds, statuses, times,
  minutes, counts. No delegate prose, no brief text, no transcript, on any
  surface, which keeps the bridge's rule that a seat's words never reach a
  human surface.
- D3. **One new key on the presence record, `working`,** composed by the
  tick beside `chain`, bounded in size, so every reader on every host sees
  the same thing; on this host the interface reads the job records
  directly and shows every job in flight, not only the newest chain.
- D4. **It fits the row.** The Running column becomes the phase sentence,
  and every machine row opens in place, a disclosure, never a new page, to
  its work: the goal, the chain, the box, the bound. The page's shape does
  not change.

## 3. The record: `working`

Added to the presence record, null when the machine holds no claim and
runs no job:

```
"working": {
  "goal": "g1-s15",
  "phase": { "role": "implementer", "round": 2, "roundLimit": 3 },
  "job": { "id": "20260925t…", "role": "implementer", "status": "running", "startedAt": "…",
           "minutesUsed": 41, "minutesCap": 120 },
  "box": { "attemptsUsed": 3, "attemptLimit": 10, "minutesUsed": 610, "minutesLimit": 720,
           "elapsed": "2d4h", "elapsedLimit": "5d" } | null,
  "chain": [ { "job": "…", "role": "critic", "round": 1, "status": "completed", "startedAt": "…", "endedAt": "…", "minutesUsed": 22, "minutesCap": 60 }, … ]
}
```

Composition, in the tick's `seat-presence` component from what it already
reads: `goal` and `job` from `NewestChain` and its record; `phase.role`
and `round` from that job, `roundLimit` from the goal's box
(`ReviewRoundLimit` for a critic, the build lane's attempt count for an
implementer, absent where the goal has no box); `minutesUsed` as
`settledJobMinutes(startedAt, now, cap)` for a running job, or the settled
value for an ended one, `minutesCap` from `capRequest.minutes`; `box` from
`dispatch.ProjectConsumption` on the goal file at the accepted tip against
the row's `Budget`, null when the goal carries no box; `chain` the members
of the newest chain's root, newest first, at most eight, each with its
status and minutes. Every number is an integer of minutes; every time is
RFC 3339 UTC. A field the records cannot supply is null, never guessed.
Size: under two kilobytes at the cap.

## 4. The page

The Running column of every machine row says the phase in one sentence:
"implementer round 2 of 3 on g1-s15 · 41 of 120 min", or "critic round 1
on g1-s19 · not started", or "idle". The row gains a disclosure control at
its start; opening it reveals, under the row and in the row's own width:

- **Goal**: the goal chip and its title, opening the goal page.
- **This job**: role, round, status, started, "41 of 120 min used; at most
  79 min left", the bound in words.
- **Box**: "attempt 3 of 10 · 610 of 720 min · 2 d 4 h of 5 d", each a
  small bar in the existing tokens, the elapsed one first because it is the
  one a human waits on; absent when the goal has no box, with the words
  "no box on this goal".
- **Chain**: a list, newest first, one line per member: role, round, status
  as the existing status chip, started or ended time, minutes used of cap.

For this seat the same block reads from the local job records and lists
every job in flight, grouped by goal, because the interface can see them
all; a machine elsewhere shows what its presence carried, and the block
says "as published 2 min ago". Help terms: `phase`, `box`, `bound` (why an
ETA here is a bound), `chain`. At phone width the block stacks and the bars
take the full width. The disclosure's open state is per viewer, in
localStorage under `ms.ui.fleet.open`, wrapped in try/catch like every
other key. No timer; the page already re-reads on the `fleet` event, and a
new tick of a machine moves its numbers.

The Partner's `fleet` tool prints the phase sentence per machine and, for a
named machine, the opened block as lines; the page capture carries the
phase sentence per row and which rows are open.

## 5. Not here

Headless seats, by Wido's word. The seat's four status strands. A machine
page of its own. Anything predicted rather than bounded. Delegate prose.

## 6. Verification and box

Go: unit tests on the composer with fake job records and a fake
consumption projection, an injected clock, covering the running, pending
and ended job, a chain over the cap of eight, a goal without a box, a
malformed job, and null where records cannot supply; the reader accepting a
record with and without `working`; `seat fleet` text with the phase
sentence. Frontend: pane tests on the phase sentence for each shape, the
disclosure open and closed, the bars' arithmetic, phone width; the guards
stay green. The walkthrough fixture publishes `working` for two machines
and jobs for this seat; screenshots at 1280 and 400 are the evidence.
Budgets as always. Box: one build lane (Claude on Opus), one code read
(Codex on Sol) with one fix round under R-124, after Astra's read; two
attempts, 90 to 150 job-minutes.

## 7. Self-grade

High on the page: a column's words and a disclosure over a payload the
tick already composes most of. Medium on the box: `ProjectConsumption`
is dispatch's projection and the tick must call it for the goal at the
accepted tip without a live dispatch; the build proves it on the fixture
and the design accepts null where it cannot. Weakest: the round limit for
an implementer is not one number in the kit the way the critic's is, so
"round 2 of 3" may read "round 2" for builds; the page says what it has.
Reject condition: Wido wants the delegate's own words on the page, which
the bridge's rule forbids and this design will not do.
