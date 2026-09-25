# g1-s42: the fleet panel — who is doing what, and is execution healthy

- Kind: design
- Id: 01M3BGYMY8N8MTWSZQ9RBT6Q47
- Status: draft
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
  on its own goroutine, once a minute at most and only while a browser is
  connected, and swallows its own failures into the page's copy line. The
  snapshot loop is not touched.
- D4. The interface never writes the steward's files. `since` comes from
  the tick's standings file when there is one and is absent otherwise; the
  page says "since unknown here" rather than inventing an instant.
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

Standings come from `seat.Fleet` with `Previous` from the tick's
standings file read read-only, so `since` agrees with the notifications
the steward sent. Holds are joined with titles and lanes from the
snapshot's own board projection so a goal chip opens the same goal page the
board opens. `needsYou` is every hold whose machine is unreachable or
unknown, oldest silence first.

`this` is composed from the checkout's nickname, the publication state and
the steward health verdict where they exist; `armed` is whether the health
verdict names a live steward runner. Every field is written; absent facts
are null or empty, never missing.

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
its payload on the ledger stream's events like Backlog does, and never on a
timer of its own. At phone width the table stacks into cards, one per
machine, standing first.

The page capture the Partner sees names the section "Fleet" and lists the
machines with their standing and flags, so "why is m1c unreachable" is a
question the Partner can answer from the rows it was shown, and the
`fleet` tool below gives it the rest.

## 5. The flag on every surface

- **Board**: a backlog row gains `holder: { machine, standing, since, flag }`
  when its claim's machine is in the presence copy or unknown by absence.
  `blockerOf` shows the flag as the card's line for In Progress cards that
  have no fence and no park: "held by m1c, unreachable since 09:40". The
  join is done in the httpd layer where the board projection and the fleet
  standings both exist; the backlog package is not taught about presence.
- **Overview**: `Work.InProgress` rows gain the same `holder` and the row
  shows the flag beside the seat.
- **`goal next`**: after the verdict, one line on stderr per claim of another
  machine whose presence in the tick's copy is unreachable or absent, "goal
  X is held by m1c, unreachable since 09:40; a human decides with goal steal
  or goal resume", read from the tick's copy without fetching; nothing
  changes in the verdict itself.

## 6. The Partner's `fleet` tool

One read tool, `fleet`, returning the report's text (this seat first, then
the machines) and the needs-you lines, from the same package the page uses,
with the copy's age as its source line the way every other tool names its
source. Added to the manifest's tool catalogue and to the Partner's
vocabulary (`standing`, `presence`, `reachable`, `unreachable`,
`unknown`, `rung`).

## 7. Authority, cost, what is not here

Nothing on these surfaces acts; the words name the human acts that exist at
a terminal. The interface's fetch is one bounded git call a minute while a
browser is open and none otherwise. Not here: seats on this host outside the
ledger, sessions and delegates per seat beyond the one chain in flight,
capacity, an act to steal or resume from the page, a notification of its own
(the steward's transitions already reach the bell when this seat is armed).

## 8. Verification and box

Server: unit tests on the new package with fakes (a presence copy, claims,
a board projection, the health file) covering the composition, the ordering,
the needs-you selection, the unarmed checkout, a failed fetch keeping the
old copy, and the once-a-minute rule with an injected clock; a test that
the fetcher never runs through the snapshot loop; the board and overview
joins; the `goal next` lines. Browser: `FleetPane` tests on the payload
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
