# g1-s45: what a seat is doing

- Kind: design
- Id: 01M3C1ESG2QVDTY8YWVMJ5PJTC
- Status: accepted
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
   `capRequest.minutes` on the record (dispatch/claim.go:763); dispatch
   enforces a recorded `capDeadline` where one exists and otherwise
   `startedAt` plus the cap (reapfacts.go:133, brief.go:533). Its settlement
   of consumed minutes (`settledJobMinutes`, budget.go:784, unexported)
   rounds up, has a one-minute floor, clamps at the cap and, for a terminal
   job, depends on process identity and ownership proof (budget.go:490);
   nothing here recomputes it.
2. **The goal's box and its consumption are projected.** `goalbudget.Budget`
   is the tuple on the goal (elapsed, attempts, reserved job minutes,
   active jobs, review rounds; internal/goalbudget/budget.go:82-88), a
   backlog `Row` carries it (internal/backlog/project.go:110), and
   `dispatch.ProjectConsumption(repoRoot, goalFile, now)` counts, for the
   goal's budget episode, `Attempts` (build attempts, proof attempts and
   governed runs alike) and `ReservedJobMinutes` (open jobs counted at
   their full cap, terminal ones at their settled charge, proof and run
   charges added; budget.go:275, 488, 619). It computes no elapsed time,
   which only `ProjectBudget` under the authority lens does (budget.go:346,
   877). It needs a parsed goal file from the accepted tip
   (`AcceptedLedgerTip` then `ProjectAt`, goal/attention.go:559). A budget's
   `d` is eight hours (goalbudget/budget.go:43). Its cost is a scan of the
   job directory, the goal's proof attempts, obligations and runs, per call.
3. **Rounds.** A job's current round is on its record; `findingRegisterRound`
   names the last folded round (finding_register.go:919). A critic chain's
   effective limit is the value dispatch froze on the chain root's own
   `reviewRoundLimit` (build.go:321-673; a design critic's is two whatever
   the goal stores), read back by accounting (finding_register.go:266). A
   build has no round limit of its own: the goal's attempt limit covers
   other work too.
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

- D1. **Progress is what the records prove; the ETA is the cap.** The page
  says the phase from the chain's roles and rounds, how long a job has run
  and the cap it reserved, and the goal's box used against its limits. The
  only forward-looking words are "its cap ends in 79 min", computed from
  the recorded enforcement deadline where the job carries one and
  otherwise from its start plus its cap, rounded up, and "7 attempts left
  in the box". They name the cap, not completion, and the help term says
  so, because the kit measures and bounds and never forecasts.
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
  "phase": { "role": "implementer", "round": 2, "roundLimit": null },
  "job": { "id": "20260925t…", "role": "implementer", "status": "running", "startedAt": "…",
           "capMinutes": 120, "capEndsAt": "…" },
  "box": { "attempts": 3, "attemptLimit": 10, "reservedMinutes": 610, "reservedMinutesLimit": 720,
           "problem": "" } | null,
  "chain": [ { "job": "…", "role": "critic", "round": 1, "status": "completed", "startedAt": "…", "endedAt": "…", "capMinutes": 60 }, … ]
}
```

Composition, in the tick's `seat-presence` component. `ReadJobs` is
extended to retain what the records already carry and it drops today:
status, `endedAt`, `capRequest.minutes` and the recorded cap deadline
(ledger.go:95). Then: `goal` and `job` from `NewestChain` and its record;
`phase.role` and `round` from that job; `roundLimit` for a critic from the
chain root's frozen `reviewRoundLimit`, null for a build, which shows its
round without a denominator; `job.capMinutes` from `capRequest.minutes`
and `job.capEndsAt` from the recorded deadline, else `startedAt` plus the
cap, null while the job has not started; `box` from
`dispatch.ProjectConsumption` on the goal file read at the accepted tip
(`AcceptedLedgerTip`, `ProjectAt`), with `attempts` mapped to its
`Attempts` and `reservedMinutes` to its `ReservedJobMinutes`, both against
the row's `Budget`; no elapsed figure in step 1, because that projection
computes none; when the goal carries no box, `box` is null; when the
projection is not `KNOWN`, `box` carries its `problem` and no numbers,
never zeros; `chain` the members sharing the newest chain's root, composed
locally from the parent links, newest first, at most eight, each with its
status, start, end and cap, and no consumed minutes, because settlement is
dispatch's and unexported. Every time is RFC 3339 UTC; every duration the
page shows is computed from times. A field the records cannot supply is
null, never guessed. The record is bounded by its eight entries.

## 4. The page

The Running column of every machine row says the phase in one sentence:
"implementer round 2 on g1-s15 · running 41 min, cap 120", "critic round 1
of 2 on g1-s19 · not started", or "idle". The row gains a disclosure control at
its start; opening it reveals, under the row and in the row's own width:

- **Goal**: the goal chip and its title, opening the goal page.
- **This job**: role, round, status, started, "running 41 min, cap 120;
  its cap ends in 79 min", the remaining time rounded up from `capEndsAt`
  and named as the cap, never as completion.
- **Box**: "attempt 3 of 10 · 610 of 720 reserved min", two small bars in
  the existing tokens, with the words that reserved minutes count open jobs
  at their full cap; durations in hours, never calendar days, because a
  budget's day is eight hours; "no box on this goal" when the goal has
  none, and the projection's own problem when it could not be computed.
- **Chain**: a list, newest first, one line per member: role, round, status
  as the existing status chip, started and ended times, the cap.

For this seat the same block reads from the local job records and lists
every job in flight, grouped by goal, because the interface can see them
all; a machine elsewhere shows what its presence carried, and the block
says "as published 2 min ago". Help terms: `phase`, `box`, `bound` (why an
ETA here is a bound), `chain`. At phone width the block stacks and the bars
take the full width. The disclosure's open state is per viewer, in
localStorage under `ms.ui.fleet.open`, wrapped in try/catch like every
other key. No timer; the page already re-reads on the `fleet` event, which follows
the interface's next presence fetch, so a machine's new tick shows after
that fetch.

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

## Dispositions (Astra read, 2026-09-25, under R-124)

Three material findings, each one step 1 would answer falsely without, and
four deferred. Every code claim checked before folding.

| id | finding | fold |
|---|---|---|
| F1 | the named projection computes no elapsed time, its minutes count open reservations in full, and a budget day is eight hours | attempts and reserved minutes with honest words, no elapsed bar in step 1, hours never days, the projection's problem shown instead of zeros |
| F2 | "at most N min left" does not hold under settlement rounding and the recorded cap deadline | the page says when the cap ends, from the recorded deadline or start plus cap, rounded up, named as the cap; no consumed minutes are recomputed |
| F3 | a critic's limit is the chain root's frozen value, a build has none | the root's `reviewRoundLimit` for critics, no denominator for builds |

Deferred, step 1 works without them: the record's byte size (bounded by
eight entries, no byte promise); the tick's cost per call, a scan of the
job directory and the goal's attempts and runs, accepted until measured;
two descriptions corrected in section 1 (the last folded round, the budget
episode); the refresh wording.
