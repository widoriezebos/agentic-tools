# g1-s74: the breathing dot — the interface shows that an agent is alive, and at what

- Kind: design
- Id: 01M3PKCYPPYHDD8R0VFREV24Q6
- Status: draft
- Goals: browser-interface

Wido, 2026-09-29: "I would like a visual indicator when the project
partner or any other agent is active, like a spinner of sorts. What do
you propose? Put on your UX cap and come up with the best possible way
of indicating clearly that an agent is still active." On the proposal,
a live mockup kept as `~/LocalStorage/agentic-tools-evidence/
breathing-dot-20260929/breathing-dot-mockup.html`: "Yeah, I like the
proposal. Make it happen." Author Fable. Every cite re-read at
`d2127aa2b`. Refines g1-s31 D6 (the working line) and g1-s9's motion
rule; builds on g1-s45 (what a seat is doing) for the machines' half.

## 1. What exists and binds

1. **The working line.** While a turn runs, `Running()` holds the
   answer's place with one italic muted paragraph, the doing line or
   "Thinking…" (`web/_app/src/partner/Transcript.tsx:392-420`, the
   paragraph at `:404`), under an opacity pulse of 1800 ms
   (`partner/partner.css:248-272`, reduced motion at `:268`); the first
   words of the answer replace it (`Transcript.tsx:401`). g1-s31 D6
   rules it so, and the Looked list waits for the end of the turn.
   Nothing outside the transcript reads the turn: the composer disables
   its field and swaps Send for Stop (`shell/Composer.tsx:153`,
   `:180-188`); the closed drawer's field is disabled under the same
   placeholder (`shell/Drawer.tsx:173-186`). The line has no
   `role="status"`; nothing in the page has `aria-busy`.
2. **What the page knows of a turn.** `busy(store)` is
   `store.live.turn !== ""` (`partner/conversation.ts:132-134`). `Live`
   carries `doing`, one line replaced by the next and kept nowhere, and
   `looked`, each read with its outcome `read | partial | failed`
   (`conversation.ts:36-70`; `partner/api.ts:164-171`), filled by the
   `doing` and `look` beats (`conversation.ts:284-291`) and, on a
   reload mid-answer, from the snapshot (`:190-210`), which carries
   `busy, turn, partial, partialSeq, activity, doing, looked` and no
   start time (`internal/ui/partner/service.go:97-112`). A doing beat
   is a tool call's title as the runtime sends it and a look is its
   completion (`internal/ui/partner/host.go:99-110`, `:1006`, `:1040`).
   The beats ride the page's one EventSource under event type `partner`
   (`notifications/stream.ts:35-50`; server
   `internal/ui/httpd/notifications.go:127`, `:171-182`,
   `httpd/partner.go:702-720`), with a heartbeat comment every 25 s the
   browser never surfaces; every stream open re-reads the snapshot
   (`stream.ts:92`, `partner/store.tsx:1008-1013`). The service tells
   the seat's record when a turn starts and ends (`service.go:589-609`,
   `cmd/metasystem/ui.go:358-363`), which `ui status` prints and the
   browser never sees. The 202 that accepts a question carries the
   turn's id and nothing else (`httpd/partner.go:268-273`), and the
   running turn holds no start (`service.go:202-235`, made at `:765`).
   A completed call retires nothing: the host emits a doing beat when a
   call starts (`host.go:1006`) and a look when it ends (`:1040`), and
   neither the service (`service.go:1523-1526`) nor the reducer
   (`conversation.ts:284-291`) clears the doing line, though the host's
   own words say the line that announced a call goes when it becomes a
   look (`host.go:99-106`). A question's first beats can arrive before
   its 202, and `asked` keeps a turn that has already begun
   (`conversation.ts:352-366`); a snapshot behind the beats is not
   applied over them (`:175-184`).
3. **The shell.** The rail has no Partner row, by Wido's word: the
   drawer is on every page (`shell/Rail.tsx:9-13`). Its bar is 48 px
   and always on screen (`Drawer.tsx:161-166`,
   `shell/shell.css:522-557`), holding the field while the drawer is
   closed; open, the composer is in the transcript. The header's right
   cluster leads with the bell "because it is the only control there
   that changes on its own" (`shell/Header.tsx:20-26`, `:90-110`). The
   page title is composed in one function from the section, the
   identity and the unread count (`title.ts:27-29`), set at
   `shell/Shell.tsx:220` and `project/DocumentPane.tsx:208`.
4. **What the page knows of other agents.** The Fleet page reads
   `/api/fleet` on mount (`fleet/api.ts:330`) and again on each `fleet`
   beat and stream open (`fleet/FleetPane.tsx:146-147`; the beat
   carries nothing, `stream.ts:39-50`). A machine's `running` is
   `{job, role, round, goal, startedAt}`, `startedAt` null on a
   reservation not begun (`fleet/api.ts:76-85`); its standing is
   `reachable | unreachable | unknown` (`:23`); this seat's own
   `running` is on the page too (`FleetPane.tsx:275`). The words are
   `runningWords` (`fleet/fleet.ts:88-104`, "idle" for none) and
   `jobWords` (`:232-252`). Presence arrives in minutes ("seen 6 min
   ago"), never seconds. A job's start stamp is not its running:
   dispatch stamps `startedAt` on a record it creates pending, so the
   status and not the stamp says whether anything runs
   (`internal/seat/publish.go:62-70`, `fleet/fleet.ts:214-229`). A
   machine elsewhere carries the one chain its presence published as
   its `working` list; this seat's row carries every job in flight from
   the local records (`internal/ui/fleet/fleet.go:341-353`,
   `fleet/api.ts:174-200`); `workingWords` says the first, "(and N more
   in flight)" (`fleet.ts:206-216`). The fleet's dots say what a role is
   in: alive `--ms-ok`, dead `--ms-danger`, unknown `--ms-marker`
   (`fleet/fleet.css:180-199`).
5. **The house rules on motion and loading.** "Loading has a shape, not
   a spinner" (`shell/controls.tsx:64-67`, the skeleton `aria-hidden`);
   motion is 120 and 180 ms, the skeleton pulse 1.2 s, and under
   `prefers-reduced-motion: reduce` every duration is 0 (g1-s9 `:153`,
   `shell.css:1390-1403`); durations are literals, there are no motion
   tokens. The cut guard forbids `setInterval` and `setTimeout` outside
   a named exception row, and there is one, the room's drafts, granted
   by Wido on 2026-09-28 (`web/_app/src/cuts.test.ts:56-70`). Every
   colour is a token in both themes (`tokens.css:15-38`, `:41-65`;
   `tokens.test.ts`; `contrast.test.ts`: text 4.5:1, non-text 3:1).
   `role="status"` is for outcomes and refusals; `aria-live="polite"`
   only on the toasts (`notifications/Toasts.tsx:39`). The guard
   asserts that no file names `setInterval` at all and counts only
   `setTimeout` in an exception file (`cuts.test.ts:841-849`). The token
   table must equal the registered set exactly (`tokens.test.ts:85-88`);
   the contrast test parses six-digit colours only and allows exactly
   three tokens to go unasserted, border, scrim and shadow
   (`contrast.test.ts:76-80`, `:122-127`). The Looked line counts what
   an answer saw with the page's own entry excluded and a failed read
   named, never counted (`partner/Looked.tsx:23-45`).

## 2. What you want

- **A1. To see it is alive.** From wherever I am, a glance tells me the
  Partner is still working.
- **A2. To see at what, and for how long.** Not a symbol: what it is
  doing now, how much it has read, how long it has been at it, so a
  long think reads as a long think and not as a hang.
- **A3. The same for the machines.** A build running on m1e shows from
  the Backlog too.
- **A4. Nothing when nothing.** Idle has no ornament.

## 3. The moment

You press Send. In the answer's place a small green dot breathes
beside "Thinking · 0:03". Two seconds later the line says the read the
Partner is at, then "1 read · 0:05". You close the drawer to look at
the board; the bar keeps the same line, Stop beside it, and the tab
reads "● Backlog · MetaSystem". You leave for another tab; when the dot
in the title goes, the answer is in. Meanwhile m1e reports a build on
verbs-match-intent: the rail's Fleet row carries the dot until presence
says the job is over, and its tooltip names who and what. Under
reduced motion the dot stands still and the clock still counts.

## 4. Decisions

- D1. **The live line, one component.** A mark of four parts, rendered
  by one component wherever it appears: the dot, 8 px, `--ms-ok`, the
  fleet's own "alive"; the words, the turn's doing line as the stream
  sends it, or "Thinking" when there is none, and a completed call's
  line is retired: when a call ends the host says, as a doing beat,
  what is still in flight, the earliest started of the remaining calls
  or nothing, so a read followed by a minute of thought shows
  "Thinking" for that minute, on this page and after a reload alike;
  the tally, from `live.looked`, counted as the Looked line counts, the
  page's own entry excluded, a failed read never among the counted, a
  partial read among them: "3 read", with ", 1 failed" appended in the
  marker colour when a read failed, and nothing before the first look;
  the clock, mm:ss since the turn began (h:mm:ss past an hour), in the
  mono face with tabular digits. The words and the tally
  are one `role="status"` `aria-live="polite"` span, so a screen reader
  hears each change of doing and nothing else; the dot and the clock
  are `aria-hidden`. The first words of the answer replace the whole
  line, as now, and the Looked list stays the account of a finished
  turn (g1-s31 D6 holds). The opacity pulse and its keyframes go: the
  motion is the dot's, and the words stay legible.
- D2. **The dot breathes.** One `@keyframes` of 2400 ms, ease-out,
  infinite: a halo, drawn as `box-shadow`, widens from 0 to 9 px and
  fades from the halo colour to transparent while the dot scales to
  1.18 at 30% and back. The halo colour is a token beside the ok pair in
  both themes, `--ms-ok-halo`, the ok colour at 40% alpha. Under
  `prefers-reduced-motion: reduce` the animation is none and the halo
  absent; the words, tally and clock carry the state. Nothing else on
  the page animates for a turn. Not a spinner: rotation says "blocked,
  wait", and the human is not blocked; a breath says "present".
- D3. **Three places, and the title.** (a) The answer's place: D1
  replaces the paragraph. (b) The drawer's title carries the dot alone
  while a turn runs and the drawer is open, for when the transcript is
  scrolled away; closed, the bar's live line is the mark, and a second
  dot beside it would say one thing twice (amended after the build's
  screenshots and Sol's read, 2026-09-29). On the phone, where the
  title is hidden, the dot alone stands at the bar's start while the
  drawer is open. (c) The closed bar while a turn runs: the disabled
  field is replaced by the live line and a Stop button that calls the
  store's stop; the Seeing chip, the proposal bar and the handed chip
  stay where they are; when the turn ends the field returns with the
  draft as it was. (d) The page title carries a leading "● " while a
  turn runs: `titleFor` takes the flag and both its callers pass it, so
  a section change while busy keeps the dot, and the dot goes when the
  answer is in.
- D4. **The clock has one origin, the server's.** The service records
  the instant it admits the turn, on the running turn; the 202 carries
  it as `startedAt` beside the turn's id; the snapshot carries it while
  the turn runs (RFC 3339, "" when idle). `Live.startedAt` is set by
  `asked` from the 202 whether or not beats already arrived, keeping
  everything they brought; `loaded` keeps a live state that is ahead of
  the snapshot and fills its start from the snapshot when it is empty;
  a turn this page learns of from a snapshot alone (an act's answer, a
  walk, a trouble) takes the snapshot's. The page never uses its own
  receipt time. One timer: the live line mounts a 1 s `setInterval`
  only while a turn runs and clears it when the turn ends or the line
  unmounts; it reads nothing and reaches no network. The cut guard's
  rows gain the timer's name: a row says which timer a file may set and
  how many times; the assertion that no file names `setInterval` is
  replaced by one that only files whose row names it do; and the live
  line's row grants one `setInterval`, which repeats by nature, by
  Wido's grant of 2026-09-29 ("Make it happen", on a proposal that
  named the clock and its timer). The clock is text, so reduced motion
  does not stop it.
- D5. **The rail's Fleet dot.** The shell reads `/api/fleet` on mount
  and again on each `fleet` beat and stream open, the three the Fleet
  page already uses, and holds the answer above the rail. A machine
  counts as working when its standing is reachable and one of its
  `working` entries has a job whose status is `running`, the one word
  dispatch keeps for work in hand; a reservation's start stamp proves
  nothing, and a job with no status is not working. This seat's row
  carries every local job, so a running job beside a newer reservation
  counts. When any machine is working, the Fleet row carries the dot as
  a badge on its icon, visible collapsed and expanded, and the row's
  accessible name and its collapsed hint gain the words, one line per
  running job, "m1e: implementer round 2 on verbs-match-intent ·
  running 41 min", the phase words of the entry that lit the dot
  (`phaseWords`, `fleet.ts:182-195`), never the machine's first entry,
  which may be a reservation. The Fleet table's own wording is
  unchanged. No clock of this slice's own: the minutes in those words
  are the Fleet page's, from presence. On the Fleet page the Running column takes the same dot before
  its words for a working machine. Before the read lands, or after it
  fails, the rail says nothing.
- D6. **Two feeds of one mark.** The Partner's turn and the machines
  feed the same component; nothing joins them into a count or a pill.
- D7. **Nothing else changes.** The composer's Stop, the busy refusal of
  a second send, the trouble held while busy, the Looked list, the fleet
  table's words, the bell, the skeleton.

## 5. Step 1, the smallest thing that works

D1 to D7 as one slice. Not in it: "Still thinking" after a silent half
minute; a hollow dot with "Reconnecting" when the stream drops; words
for the machines in the header; the critique loop's "the critic
reading" as a feed (it has no push today, only a read on the design's
page); a dot on the Overview's in-progress goal; a done mark in the
title; one owner for the fleet read shared by the shell and the Fleet
page.

## 6. Payload and routes

No new route. The running turn holds its start; the question's 202
carries `startedAt` beside `turn`; `Snapshot` gains `startedAt` (RFC
3339, "" when idle); `Live` gains `startedAt`. The host emits a doing
beat on a call's completion naming what is still in flight or "".
`titleFor` gains a `working` flag. `tokens.css` gains `--ms-ok-halo` in
both themes and its Tailwind mirror, registered in the token table's
set, and named beside border, scrim and shadow among the tokens the
contrast test does not assert, as translucent decoration; the solid
dot's pair, `ok` on `surface-2`, stands, and `ok` on `surface` joins it
for the transcript. The cut guard's `TIMER_EXCEPTIONS` rows carry the
timer's name and count; the live line's row grants one `setInterval`;
the shell's fleet read is the existing call site in `fleet/api.ts` and
adds no row. Copy: "Thinking" (no ellipsis), "N read", ", N failed";
the bar's Stop is the composer's "Stop"; the rail's words "<machine>:
<phaseWords of each running job>".

## 7. Not here, later

The states named in §5; a header pill; a sound; a toast when the
answer lands; an elapsed clock for machines, which would need presence
in seconds.

## 8. Verification and box

Go: the 202 and the snapshot carry the turn's start while a turn runs
and the snapshot "" when idle, the same instant in both; a call's
completion emits a doing beat of "" when no call remains and of the
earliest remaining call's title when one does, and the snapshot's
`doing` follows. Frontend (vitest): the live line from a store with a
running turn and no doing says "Thinking" with the clock; with a doing
and two reads says the doing and "2 read"; with a failed look says
", 1 failed" in the marker class and does not count it; a page entry
is not counted; one completed call followed by silence says "Thinking";
a 202 arriving after the turn's first beats sets the start and keeps
the beats; a snapshot behind the beats fills an empty start and
replaces nothing else; with the first text the line is gone; the words span is `role="status"` polite
and the clock and the dot are hidden; the closed bar shows the live
line and Stop while busy and the field with the draft otherwise, the
proposal bar and the handed chip in both; the drawer's title carries
the dot only while busy; `titleFor` with the flag, the unread prefix
kept; the rail's Fleet row from a page with one reachable machine whose
job is `running` carries the dot and the words, and none from an
unreachable machine, a `pending` job with a start stamp, a job with no
status, or no page, and this seat's row with a running job beside a
newer reservation counts and its words name the running job, not the
reservation listed first (`rail_words_follow_running_job`); the cut
guard's rows (one `setInterval` in
the named file and no other, no new network site); the tokens and
contrast tests over the halo token; reduced motion turns the
animation off, held by a static assertion over the stylesheet.
Walkthrough: the three placements and the title with a turn in flight,
staged through the walkthrough's fixture Partner or a staged snapshot,
and the rail's dot from a staged fleet page; 1280 and 400, light and
dark, reduced motion once. Landing checks as g1-s71 §8; the coverage
floors of `internal/ui/partner` and `internal/ui/httpd` held. Box:
Astra's critique (two rounds), one Opus 5.5 lane, one Sol read with one
fix round; 60 to 120 job-minutes.

## 9. Self-grade

High on D1 to D3: the state already exists in the store and the places
are the ones the shell already has; the one server change D1 needs, a
doing beat on completion, is what the host's own comment already
promises. Medium on D4: the one timer is a deliberate exception to a
cut Wido keeps tight, and the start travels through three shapes.
Medium on D5: presence granularity means the dot can outlive a job by
minutes, and the words name the source. Weakest: the doing line's
words are the runtime's tool titles, which the design does not
rewrite; if they are terse the line leans on the tally and the clock.

## Dispositions (Astra round 1, 2026-09-29, under R-121 and R-124)

Read of revision 1 at `4b00ec866`, verbatim in
`g1-s74-astra-critique.md`. Five material findings, all folded; every
cited line re-read at whole-function depth before folding.

| id | finding | fold |
|---|---|---|
| S74-01 | a start stamp does not say a fleet job runs: dispatch stamps `startedAt` on a record it creates pending (`internal/seat/publish.go:62-70`), the Fleet page reads status (`fleet.ts:214-229`), and this seat's row carries every local job (`fleet.go:341-353`) | D5: working = reachable and a `working` job whose status is `running`; a stamp proves nothing, no status is not working; this seat's every job counts; the words are `workingWords`; §8 fixtures for a stamped pending job and a local running job beside a newer reservation |
| S74-02 | the doing line is never retired: a completion emits a look only (`host.go:1040`) and neither the service (`service.go:1523`) nor the reducer (`conversation.ts:286`) clears `doing`, so a finished read stays "now" through a minute of thought | D1: the host emits a doing beat on completion naming what is still in flight or nothing; §8 fixture: one completed call then silence says "Thinking" |
| S74-03 | the clock had two origins: the 202 carries only the turn (`httpd/partner.go:268-273`), `asked` stamps the page's receipt time (`store.tsx:1100`) and keeps a turn whose beats came first (`conversation.ts:352`), and a snapshot behind the beats is not applied (`:183`) | D4: one origin, the server's admit instant, carried by the 202 and the snapshot; `asked` sets it keeping the beats; `loaded` fills it when empty; §8 fixtures for a late 202 and a snapshot behind the beats |
| S74-04 | an exception row does not admit `setInterval`: the guard asserts no file names it and counts `setTimeout` only (`cuts.test.ts:841-849`) | D4 and §6: rows carry the timer's name and count; the no-`setInterval` assertion becomes "only where a row names it"; the live line's row grants one |
| S74-05 | the halo token fails two guards: the token table must equal the registered set (`tokens.test.ts:85-88`) and the contrast test parses six-digit colours only, allowing exactly border, scrim and shadow unasserted (`contrast.test.ts:76-80`, `:122-127`) | §6: register the token; name it beside border, scrim and shadow as translucent decoration; the solid dot keeps its asserted pair and gains `ok` on `surface` |

Non-material, taken anyway: S74-07, the tally counts as the Looked
line counts (page excluded, a failed read named and never counted, a
partial read counted; `Looked.tsx:23-45`), and ", N partial" is
dropped. S74-06, one owner for the fleet read, stays in §5's later
list. Astra also verified, and the design leans on, that the words and
the clock are separate spans; that the bar swap's controls have owners
that survive it; that both title setters are reached; and that
`loadFleet` from the shell adds no call site.

**Round 2, the declared failsafe (2026-09-29, at `2c84875a8`):**
S74-02, S74-03, S74-04 and S74-05 confirmed answered; no new
material finding. S74-01 held on one point: the predicate was right
but the words were not, since `workingWords` names a machine's first
entry and counts the rest (`fleet.ts:206-216`), so a local machine
with a pending job listed before a running one would breathe for the
running job and name the pending one. Fold, in D5, §6 and §8: the
rail's words are the phase words of each entry that lit the dot
(`phaseWords`, `fleet.ts:182-195`), one line per running job; the
Fleet table's wording unchanged; fixture
`rail_words_follow_running_job`. Astra also noted, and the build must
honour, that the host's `calls` map holds no start order
(`host.go:380`), so "earliest started of the remaining" needs one
recorded; that `loaded`'s ahead branch returns the live state
unchanged today (`conversation.ts:194`) and must fill the start there;
that the service has an injectable clock (`service.go:157`); and that
a failed local jobs read yields no jobs, so `workingProblem` lights
nothing. Closed at the failsafe round on one fold, with one scoped
confirmation read on S74-01 alone.

**Confirmation read (2026-09-29, at `08123ffde`):** S74-01 confirmed
answered, zero material. The loop is CLOSED.
