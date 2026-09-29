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
   browser never sees.
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
   ago"), never seconds. The fleet's dots say what a role is in: alive
   `--ms-ok`, dead `--ms-danger`, unknown `--ms-marker`
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
   only on the toasts (`notifications/Toasts.tsx:39`).

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
  sends it, or "Thinking" when there is none; the tally, from
  `live.looked`, "3 read", with ", 1 failed" or ", 2 partial" appended
  in the marker colour when a look ended so, and nothing before the
  first look; the clock, mm:ss since the turn began (h:mm:ss past an
  hour), in the mono face with tabular digits. The words and the tally
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
  while a turn runs, open or closed, for when the transcript is
  scrolled away. (c) The closed bar while a turn runs: the disabled
  field is replaced by the live line and a Stop button that calls the
  store's stop; the Seeing chip, the proposal bar and the handed chip
  stay where they are; when the turn ends the field returns with the
  draft as it was. (d) The page title carries a leading "● " while a
  turn runs: `titleFor` takes the flag and both its callers pass it, so
  a section change while busy keeps the dot, and the dot goes when the
  answer is in.
- D4. **The clock is honest across a reload.** The service records when
  the running turn began and the snapshot carries it (`startedAt`, RFC
  3339, empty when idle); `Live` carries it too, set from the accepted
  send on this page and from the snapshot on a reload. One timer: the
  live line mounts a 1 s `setInterval` only while a turn runs and clears
  it when the turn ends or the line unmounts; it reads nothing and
  reaches no network; the cut guard gains one exception row naming the
  file and Wido's grant of 2026-09-29 ("Make it happen", on a proposal
  that named the clock and its timer). The clock is text, so reduced
  motion does not stop it.
- D5. **The rail's Fleet dot.** The shell reads `/api/fleet` on mount
  and again on each `fleet` beat and stream open, the three the Fleet
  page already uses, and holds the answer above the rail. A machine
  counts as working when its standing is reachable and its `running`
  has a `startedAt`; this seat counts the same way. When any does, the
  Fleet row carries the dot as a badge on its icon, visible collapsed
  and expanded, and the row's accessible name and its collapsed hint
  gain the words, one machine per line, "m1e: build round 2 on
  verbs-match-intent" (from `runningWords`). No clock: presence cannot
  tick. On the Fleet page the Running column takes the same dot before
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

No new route. `Snapshot` gains `startedAt` (RFC 3339, "" when idle).
`Live` gains `startedAt`. `titleFor` gains a `working` flag.
`tokens.css` gains `--ms-ok-halo` in both themes and its Tailwind
mirror. The cut guard's `TIMER_EXCEPTIONS` gains one row for the live
line's clock; the shell's fleet read is the existing call site in
`fleet/api.ts` and adds no row. Copy: "Thinking" (no ellipsis), "N
read", ", N failed", ", N partial"; the bar's Stop is the composer's
"Stop"; the rail's words "<machine>: <runningWords>".

## 7. Not here, later

The states named in §5; a header pill; a sound; a toast when the
answer lands; an elapsed clock for machines, which would need presence
in seconds.

## 8. Verification and box

Go: the snapshot carries the turn's start while a turn runs and "" when
idle, and the start survives a re-read mid-turn. Frontend (vitest): the
live line from a store with a running turn and no doing says "Thinking"
with the clock; with a doing and two reads says the doing and "2 read";
with a failed look says ", 1 failed" in the marker class; with the
first text the line is gone; the words span is `role="status"` polite
and the clock and the dot are hidden; the closed bar shows the live
line and Stop while busy and the field with the draft otherwise, the
proposal bar and the handed chip in both; the drawer's title carries
the dot only while busy; `titleFor` with the flag, the unread prefix
kept; the rail's Fleet row from a page with one reachable running
machine carries the dot and the words, and none from an unreachable
machine, a reservation without a start, or no page; the cut guard's
rows (one timer in the named file, no new network site); the tokens
and contrast tests over the halo token; reduced motion turns the
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
are the ones the shell already has. Medium on D4: the one timer is a
deliberate exception to a cut Wido keeps tight, and the start time is a
small server change. Medium on D5: presence granularity means the dot
can outlive a job by minutes, and the words name the source. Weakest:
the doing line's words are the runtime's tool titles, which the design
does not rewrite; if they are terse the line leans on the tally and the
clock.
