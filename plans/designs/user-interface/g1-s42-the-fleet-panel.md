# g1-s42: the fleet panel — who is doing what, and is execution healthy

- Kind: design
- Id: 01M3BGYMY8N8MTWSZQ9RBT6Q47
- Status: done
- Goals: browser-interface seat-mutual-awareness

Slice 2 of the bridge (seat-mutual-awareness revision 7, section 10) and the
interface's Fleet section, designed together because the second is the
first's consumer. Author Fable, 2026-09-25. Every cite re-read at
`3a9ec4f9b` on ui-development.

## 1. What exists and binds

1. The shell already reserves the section: `routes.ts` lists `fleet` at
   `/fleet` with the icon Server and the sentence "A row per machine and
   seat, with its sessions, the jobs it is running, its last census and its
   health, each stamped with when it was observed", marked `projected:
   false`, so today the rail shows it and `Shell.tsx:273` renders the
   generic `SectionPane` placeholder for it. The interface design's own row
   (plans/designs/user-interface-design.md:34) asks the section one
   question: who is doing what, and is execution healthy?
2. Slice 1 of the bridge landed (`4700b8e1e`): `internal/seat` reads a
   presence copy (`Git.Fetch(namespace)` fetches both remote namespaces
   into `<namespace>/{metasystem,heads}/*`, `Git.Read`, `Join`), judges
   standings (`Fleet(FleetInput{This, Copy, Claims, ClaimsUnavailable,
   Previous, Now, Window})` returning `[]MachineStanding` with `Standing`,
   `Reason`, `AgeSeconds`, `Record`, `Holds`, `Since`, `Flag`, `This`),
   and prints a `Report` with `JSON()`. `cmd/metasystem/seat_verbs.go:60-140`
   assembles a report from the tick's copy, `seat.Claims(root)` at the
   accepted tip, `seat.LoadStandings` for frozen `since`, and
   `seat.LoadPublicationState` for this machine's own publishing. The
   design's rule for the interface stands: it fetches into its own namespace,
   `refs/metasystem/presence-ui`, at most once a minute, from its own call,
   never through the snapshot loop's ledger `Fetch` (bridge design section
   4, Opus F1).
3. The interface server: routes `/api/overview`, `/api/backlog`,
   `/api/project`, `/api/notifications`, `/api/partner`, `/api/interface`,
   `/api/session`, `/api/workspace` (internal/ui/httpd). The snapshot holder
   (`internal/ui/snapshot`) owns the ledger read and the freshness loop
   (`loop.go`: five seconds while a browser reads, thirty idle; any error
   from `Fetch` blanks the tip and backs the loop off). One `EventSource`
   in `src/notifications/stream.ts` carries the ledger's freshness events to
   every page, which re-reads its payload on them.
4. Holders on the board and on Overview: a backlog `Row` carries
   `Claim{Machine, Lineage, At}` (internal/backlog/project.go:288-289); the
   card's one extra line is `blockerOf(row)` (src/backlog/Board.tsx:882-905):
   a fence, a dependency park, an open blocker, an expired approval. The
   Overview's `Work.InProgress` rows carry `Seat{Machine, Lineage}`
   (internal/ui/overview/overview.go:179-190).
5. `goal next` (cmd/metasystem/goal.go:687) prints a `NextVerdict`
   (internal/goal/project.go:614-625): this machine's claims, fenced,
   landing, ready, blocked, awaiting, refused. It says nothing about the
   holders of other machines' claims.
6. The Partner's tools are one stdio server (`internal/ui/uitools`:
   `board`, `goal`, `document`, `records`, `questions`, `overview`,
   `notifications`, `search`, `describe`); the manifest
   (`internal/ui/manifest`) publishes sections, lanes, terms and questions
   to `/api/interface`; help terms live in `src/help/terms.ts` as one
   register of `HelpId`s; the page capture the Partner sees names the
   section and the rows shown (g1-s29).
7. The walkthrough fixture (`internal/ui/httpd/walkthrough`) serves every
   route from invented data for screenshots and tests; it knows goals with
   claims but no presence.
8. Host facts this seat can read without git: the steward's health verdict
   (artifacts/agents/steward/health.json, `{state, verdict}`, roles inside
   the verdict when armed), the publication state
   (artifacts/agents/steward/seat-presence.json), the standings file
   (seat-fleet.json), the delegate jobs (artifacts/agents/jobs, read by
   `seat.ReadJobs`). The checkout the interface serves may be unarmed, as
   this one is: no roles, no publication, no standings.

## 2. Decisions

- D1. One page answers one question. Fleet shows who is doing what and
  whether execution is healthy, from two sources and nothing else: the
  presence copy for standings, the accepted tip for who holds what. It does
  not invent a third source. Seats on this host that the ledger does not
  know are not shown, because the ledger never learns a hostname; that is
  the "later, when it hurts" list.
- D2. Presence is a flag, never an act, on every surface: the Fleet page,
  the board's card line, the Overview's In Progress rows, and `goal next`'s
  words. Nothing here steals, resumes or parks.
- D3. The interface fetches presence itself, into `refs/metasystem/presence-ui`,
  from one fetch owner started and stopped under the server's ownership
  gate (cmd/metasystem/ui.go:178), beside the snapshot loop and never
  through it. The owner's rules: one attempt in flight; at least sixty
  seconds between attempt starts, counting failures and reconnects; an
  attempt only while at least one browser holds the notifications
  EventSource open, which is the server's one explicit connection signal
  (the snapshot loop's "connected" is its own private observe-within-30s
  state, snapshot.go:116, loop.go:313, and is not read for this); no
  attempt after the last disconnect. It records last attempt, last success
  and last failure separately; a failure preserves the last success; a
  successful fetch that brought nothing is a success with an empty copy,
  distinct from never having fetched; nothing survives a server restart,
  and until the first attempt the page says so. Its metadata is written
  to artifacts/agents/ui/presence-fetch.json after every attempt, for the
  Partner's tool (section 6), which runs in another process and must own no
  fetcher of its own.
- D4. The interface never writes the steward's files. `since` is kept only
  when the tick's standings file holds an observation for that machine
  whose standing equals the one the page just judged; otherwise `since` is
  unknown, because `seat.Fleet` would otherwise mint the request's own
  instant as a first observation (standing.go:195, 218). The flag words
  are regenerated after that normalisation, never taken from
  `SilentHolder`'s empty-time branch, whose "no presence record" is false
  for an unreachable machine with a valid record. Undated silences sort
  after dated ones in "oldest silence first".
- D5. The page is composed on the server, like Overview: the browser draws
  one payload and judges nothing.

## 3. The payload: `GET /api/fleet`

Composed by a new `internal/ui/fleet` package from the seat package, once
per request, from the interface's own copy:

```
{
  "schemaVersion": 1,
  "readAt": "...",
  "copy": { "fetchedAt": "...", "source": "the interface" | "the tick" | "local refs", "problem": "" },
  "claims": { "tip": "5b9d958", "unavailable": "" },
  "this": { "machine": "m1u", "noNickname": false, "armed": true,
            "publication": { "lastSuccessAt": "...", "lastOutcome": "published", "rung": 1, "detail": "" } | null,
            "health": { "state": "healthy", "roles": [ { "role": "steward-runner", "status": "alive", "reason": "..." }, ... ] } | null,
            "running": { "role": "implementer", "round": 2, "goal": "..." } | null },
  "needsYou": [ { "goal": "tests-parallel-and-deterministic", "title": "...", "machine": "m1c", "standing": "unreachable", "since": "...", "flag": "held by a machine unreachable since 09:40" } ],
  "machines": [ { "machine": "m1e", "standing": "reachable", "reason": "...", "ageSeconds": 118, "since": "...",
                  "running": { ... } | null, "engine": "3f9c1e2", "generation": 4,
                  "holds": [ { "goal": "...", "title": "...", "lane": "In Progress", "flag": "" } ], "this": false } ]
}
```

The copy: on each request the package reads the interface's namespace; a
background fetcher refreshes it at most once a minute while any browser is
connected (the same "connected" the snapshot loop already knows), under
the seat package's bounded transport, and records `fetchedAt` or
`problem`; a failed fetch keeps the previous refs and says so. When the
interface's namespace has never been fetched, the page reads the tick's
copy and says `source: the tick`; in LocalMode it reads the local refs.

One captured ledger. Claims, `claims.tip`, titles and lanes all come from
one `snapshot.Observation` and its board projection, taken once per
request, never from `seat.Claims(root)`, which captures its own tip and
returns neither the tip nor the projection (ledger.go:29); two captures
could name a holder from one commit and a title from another. Standings
come from `seat.Fleet` and are then normalised per D4. Holds are joined
with titles and lanes from that same projection so a goal chip opens the
same goal page the board opens. `needsYou` is every hold whose machine is
unreachable, or unknown by absence, dated silences first and oldest first,
undated after.

The copy block says what the fetch owner knows: `attemptedAt`,
`succeededAt`, `problem`, and `source`, which is `the interface` after a
success, `the tick` while the interface has not yet succeeded and reads the
tick's copy (with `succeededAt` unknown, because the tick's standings
`ReadAt` is an observation time and not a fetch time, transition.go:102),
or `local refs` in LocalMode.

`this` is composed from the checkout's nickname, the publication state
(`seat.LoadPublicationState`, which already tells absent from unreadable,
state.go:62) and the steward's health file, which is the LAST RECORDED
verdict and is presented as such: the payload carries `health.observedAt`
and the page says "last recorded 12 min ago"; `armed` is not a boolean but
a word, `armed` when that verdict's steward-runner role was alive, `not
armed` when the verdict says so or no verdict exists, `stale` when the
verdict is older than three ticks, and an unreadable or malformed file is
named as such rather than read as absent (health.go:139, 219). `running`
for this seat comes from the local jobs, `NewestChain(ReadJobs(root))`,
with its unreadable explanation carried rather than shown as idle; for
every machine `running` keeps the published chain's nullable `startedAt`,
so a pending job says "not started" and the companion's disposition A10
stays honoured. Every field is written; absent facts are null or empty,
never missing.

## 4. The page

Route `/fleet`, `FleetPane`, replacing the placeholder; `projected: true`
in `routes.ts`. Three blocks, in this order, each with a help control from
the terms register (`fleet`, `presence`, `standing`, `seat`):

1. **Needs you**, shown only when non-empty: one line per held goal whose
   holder has gone silent, "tests-parallel-and-deterministic is held by
   m1c, unreachable since 09:40", the goal opening on its page, and a
   sentence that says what a human can do: steal or resume at a terminal,
   because the interface has no such act yet. Empty, the block is absent,
   not a "nothing needs you" line.
2. **This seat**: the nickname or "this checkout has no machine nickname and
   publishes no presence"; armed or "supervision is not armed here";
   "presence published 2 min ago on rung 1" or the publication problem in
   the steward's words; what is running, as one sentence; the health roles
   as a compact list, dead and unknown first with their reasons, alive
   collapsed to "12 roles alive" behind a disclosure.
3. **The fleet**: a table, one row per machine, this seat first, then
   machines with a flagged hold, then reachable, then unknown. Columns:
   machine; standing as a pill (reachable, unreachable, unknown, in the
   existing status tokens, never a new colour); seen, as an age with the
   instant behind a title ("2 min ago"), or "no presence" for unknown, with
   "since 09:40" beside an unreachable standing when known; running, as
   words; holds, as goal chips that open the goal page, each flagged chip
   carrying the flag words; engine and generation, muted, at the end. Under
   the table one quiet line: "presence fetched 40 s ago by the interface;
   claims from the ledger at 5b9d958" or the copy problem in its own words.

Empty fleet: "No seat has published presence yet. A seat publishes from its
steward tick once it runs this version and is armed." The page re-reads
its payload on one new event of the existing EventSource, `fleet`, which
the server emits after every presence attempt, success, failure or
unchanged, and on reconnect; the stream today carries notifications and
Partner events only (stream.ts:37, notifications.go:172), and Backlog
reads on mount and on Refresh, so without this event a mounted Fleet page
would never learn of new presence. The event carries no payload and takes
no replay id; the page reads `/api/fleet` again. Never a timer of its own.
At phone width the table stacks into cards, one per machine, standing
first.

The words for `unknown` keep the reason apart: "no presence record" when
the ref is absent, "presence unreadable: <refusal>" when malformed, and
"clock ahead by <d>" for a record from the future; "seen" comes from the
record when there is one, whatever the standing. A machine known only by
absence has not "gone silent": its flag says "held by m0b, which has
published no presence". The human's remedies are named conditionally,
because they are not interchangeable: `goal steal` reassigns a claim and
refuses a fenced one; `goal resume` lifts a breach fence and keeps the
owner (stop.go:464, 494; verbs.go:3583). The section's own sentence in
`routes.ts:94` and the `fleet` term in `terms.ts:143` are rewritten to
promise what the page shows, standings and holds, and no longer sessions
or a census.

The page capture the Partner sees is a Fleet capture of its own, not the
backlog fallback the server composes for a section it does not know
(context.go:364): both capture schemas (src/partner/api.ts, partner/conversation.go)
gain a bounded `fleet` shape, the machines shown with standing, age and
flags and the copy's provenance, the capture composer (capture.ts) fills
it on `/fleet`, and the server's context gains a Fleet branch that says
what was displayed and from which reading, distinct from any later tool
reading. "Why is m1c unreachable" is then a question the Partner answers
from the rows it was shown, and the `fleet` tool below gives it the rest.

## 5. The flag on every surface

- **Board**: the backlog payload today exposes `[]backlog.Row` directly
  (httpd/backlog.go:40); it gains a UI-owned row type wrapping the backlog
  row with `holder: { machine, standing, since, flag }` for a claimed row
  whose machine has a standing, so the backlog package is not taught about
  presence and the join happens in the httpd layer from the same
  observation the page was composed from. `blockerOf` shows the flag as the
  card's line immediately after a fence and before a dependency park, so a
  fenced card still says it is stopped and a held card that is merely
  silent says "held by m1c, unreachable since 09:40".
- **Overview**: `Work.InProgress` rows gain the same `holder`, filled in
  Overview's own composition from the same standing lookup, since that
  composition copies selected fields by hand (overview.go:615); the row
  shows the flag beside the seat.
- **`goal next`**: inside `runGoalNext` (cmd/metasystem/goal.go:622-740)
  the tree it already holds, `p.Tree`, is joined to the tick's copy without
  fetching, and one line goes to stderr per claim of another machine whose
  standing is unreachable, "goal X is held by m1c, unreachable since 09:40;
  a human reassigns it with goal steal", or absent, "... which has
  published no presence"; `NextVerdict`, stdout and the exit code do not
  change.

## 6. The Partner's `fleet` tool

One read tool, `fleet`, registered where the catalogue lives, in
`uitools` (`Operations`, `Answer`, `Catalogue`, uitools.go:52-96, mcp.go:88;
the manifest publishes sections and terms, not tools), using the bounded
result machinery every tool uses, and returning the report's text (this
seat first, then the machines) and the needs-you lines. The tool server
runs in its own process (cmd/metasystem/ui_tools.go:59) and owns no
fetcher: it reads the interface's namespace `refs/metasystem/presence-ui`
and the fetch owner's metadata file of D3, and its source line stamps both
the presence copy's provenance and the claims tip. The Partner's vocabulary
gains `standing`, `presence`, `reachable`, `unreachable`, `unknown` and
`rung`.

## 7. Authority, cost, what is not here

Nothing on these surfaces acts; the words name the human acts that exist at
a terminal. The interface's fetch is one bounded git call a minute while a
browser is open and none otherwise. Its remaining shared side effects with
the snapshot loop's fetch are git's own: both write `FETCH_HEAD` and may
trigger auto maintenance; the ledger reads its private operation ref and
never `FETCH_HEAD` (attention.go:681, 747), so this is churn, not a race
over a selected tip, and the build's two-callers test says so. Not here: seats on this host outside the
ledger, sessions and delegates per seat beyond the one chain in flight,
capacity, an act to steal or resume from the page, a notification of its own
(the steward's transitions already reach the bell when this seat is armed).

## 8. Verification and box

Server: unit tests on the new package with fakes (a presence copy, one
observation with its board projection, the health file, the publication
state) covering the composition, the ordering, the needs-you selection
with dated and undated silences, the `since` normalisation of D4 on an
unarmed checkout and on a transition the tick has not seen, the unarmed
and stale and unreadable health cases, a failed fetch keeping the old copy
and the last success, the never-fetched and successful-empty copies, the
sixty-second and one-in-flight and connected-only rules of the fetch owner
with an injected clock and a fake connection signal, the `fleet` stream
event after each attempt, the metadata file written for the tool; a test
that the fetcher never runs through the snapshot loop and that two callers
run side by side; the board and overview joins from one observation; the
`goal next` lines; the tool's registration and its bounded result. Browser: `FleetPane` tests on the payload
shapes (needs-you present and absent, unarmed seat, empty fleet, phone
width stacking); the existing guards (`cuts`, `literals`, `tokens`) stay
green; the walkthrough fixture gains presence for three machines
(reachable, unreachable holding a goal, unknown named by a claim) and the
`--proven` variant an armed seat, and the walkthrough screenshots at
desktop and phone width are the evidence. Budgets as always: from
`metasystem/` the fast gate and the touched packages; from `_app/` typecheck,
vitest and the bundle. Box: one build lane (Claude on Opus), one code read
(Codex on Sol), after Astra's read of this page; two attempts, 120 to 180
job-minutes, the engine half and the page half in one worktree because the
page is the engine half's only consumer.

## 9. Self-grade

High on the page: it is Overview's pattern with a table, over a payload
whose every field slice 1 already computes. Medium on the fetcher: a second
git caller beside the snapshot loop in one server must not contend with it,
and the once-a-minute rule and the bounded transport are what keep it
harmless; the build proves both with the two running together. The
weakest claim is `since` on an unarmed interface checkout, which is absent
by D4 and said so; if that reads as a gap in practice, the fix is arming
the seat, not a second standings file. Reject condition: Wido wants the
fleet page to act, in which case this is a different, human-authority
design.

## Dispositions (Astra read, 2026-09-25)

Ten material findings, each checked against the code it cites and folded.

| id | finding | fold |
|---|---|---|
| A1 | "connected" had no usable owner; the minute rule needed an attempt clock | one fetch owner under the server's ownership gate, the EventSource lifetime as the connection signal, one in flight, sixty seconds between starts (D3) |
| A2 | the stream carries no freshness event, so a mounted Fleet page would never update | one `fleet` event on the existing EventSource after every attempt and on reconnect (section 4) |
| A3 | the tick's `Previous` makes `seat.Fleet` mint the request's own `since` | `since` kept only when the tick's observation matches the judged standing; flag words regenerated (D4) |
| A4 | the tick's `ReadAt` is an observation time, so a fallback could call old refs freshly fetched | last attempt, last success and last failure kept apart; never-fetched and successful-empty distinguished (D3, section 3) |
| A5 | claims from one capture and titles from another; backlog rows exposed raw | everything from one observation; a UI-owned row type carries `holder`; Overview fills it in its own composition; `goal next` joins `p.Tree` (sections 3 and 5) |
| A6 | the health file is a past verdict presented as current | `observedAt` carried and shown; armed, not armed, stale, unreadable told apart (section 3) |
| A7 | `unknown` collapsed absent, malformed and clock-ahead; steal and resume are not interchangeable | distinct words per reason; remedies named conditionally (section 4) |
| A8 | `running` lost the pending distinction of A10 | nullable `startedAt` kept; this seat's source named (section 3) |
| A9 | the tool runs in another process and the catalogue lives in uitools | no tool-owned fetcher, a metadata file, registration through Operations, Answer and Catalogue (section 6) |
| A10 | a Fleet capture did not exist and the route's sentence promised sessions and a census | a bounded Fleet capture in both schemas and the composer; the sentence and the term rewritten (section 4) |

The git side effects the two fetchers still share, `FETCH_HEAD` and auto
maintenance, are named in section 7 as churn, not a race.

## Built (2026-09-25)

Landed on ui-development and main in one merge. Built by Claude on Opus,
read by Codex on Sol: eight findings, fixed in one round together with one
of the design owner's own from the screenshots.

| id | finding | fix |
|---|---|---|
| S1 | a failed presence read became an empty copy and false absence flags on every surface | a read failure withholds the judgement; no machine is reported absent and no flag is raised; the copy block says why |
| S2 | the Partner's tool trusted a previous server run's fetch metadata | the metadata is bound to a run id; the tool falls back to the tick's copy for another run's file; a failed metadata write is visible |
| S3 | the last disconnect and an attempt's admission were not ordered | admission runs under the watch's own lock |
| S4 | success and failure times carried the attempt's start | the injected clock is read again when the attempt ends |
| S5 | `goal next` printed a remedy for malformed and clock-ahead holders | one predicate, `NeedsHuman`, for the page and the verb: unreachable, or unknown by absence |
| S6 | the flag words embedded a UTC clock while the page rendered local time | the instant travels as RFC 3339 and each reader renders it; the terminal prints the instant |
| S7 | help controls named the wrong terms and some visible names had none | terms for Needs you, This seat, machine-held goals, Engine; the rung term attached where a rung shows |
| S8 | the fleet surface omitted the changed consumers | every changed path under internal/ui and cmd/metasystem, with two focused groups |
| S9 | the walkthrough never served a workspace, so every evidence header showed an error | thirteen lines in the fixture |

Departures the read accepted: the copy block's four times and `armed` as a
word; lane ids titled by the page; staleness as `seat.Threshold`; needs-you
limited to unreachable and unknown by absence; composition from one
observation instead of serving `seat fleet --json`; no isolation inventory
after the test overhaul; `goal next` importing the fleet package for one
account of silence; the fixture's `-fleet-every` knob. Not proved end to
end: a departing stream blocking an admission (asserted structurally under
the lock), and the tool's rejection of a stale file after a live restart
(unit-tested).
