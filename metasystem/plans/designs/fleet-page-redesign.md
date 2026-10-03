# The Fleet page, redesigned: one screen, one line per thing, one button each

- Kind: design
- Id: 01M40AVPXKBX1BPNJ9K77FHVWG
- Status: accepted
- Goals: fleet-page-redesign

Critique: Codex Astra (tier 2, run directly), closed after round 2, the failsafe (material per round 6, 0; round 2 confirmed FR-01 to FR-07 closed). Wido approved step 1 as drawn on 2026-10-03 (channel answer "yes").

Wido, 2026-10-03: "a mess from a UX perspective. Long page with a lot of text, and it states .. needs you but it is unclear what I can do and where from this."

Evidence: `~/metasystem-evidence/agentic-tools-ui/fleet-page-redesign-20261003/`: `today/` is the live page this morning (full-height screenshots and word counts), `mocks/` the redesign, one file per state (listed at the end of section 3). Earlier screens: `agentic-tools-evidence/fleet-panel-ux/fleet-0925-top.png` and `fleet-panel-ux-step2-20261003/screens/fleet-2a-*.png`.

What binds this page. R-121-m0 and R-124-m1u: the smallest thing first; a finding is material only when step 1 does not work or is not safe without it. R-11: if the layout needs explaining, it fails. R-5: a number ships with its act or not at all. R-142-ui: the signed-in browser may pause and resume the lane and stop a machine on this computer, never the one serving the page, with a second press for Stop; R-125-m1u and R-128-ui: a session is admitted per verb and nothing else that asks a terminal-grade proof admits one; R-129-ui: a repeat is success. R-126-m1e: a command shown here is the public verb, as a person would type it. R-142-m1e and R-143-m1e: the machine never overrules a person, and before a person overrides anything the impact is said in plain words; this is why Stop says what it ends before its second press. The accepted `fleet-panel-ux.md` (J1 to J6, folds R1-01 to R3-01) and `fleet-panel-ux-step2.md` (2a reviewed clean and waiting to land; 2b's Pause, Resume and Stop) stand whole. This page changes what is on screen by default, not the facts or the authority behind them.

## 1. Step back

Who looks: Wido, alone, between other things. The laptop at 1440 px, or the phone in the pub at 390 px. A visit lasts seconds. The glance must answer "is all fine?" and, when not, "what is wrong, and what do I press?".

J1 to J6 hold unchanged. What changes: J2 gets a shape, one line and one button per item; J3's table keeps only what a person decides on (what each seat does, what it holds, when it was seen); nothing about "this seat" is a question of its own, J1 already covers it, so it stops being a section.

Decisions made on this page: is anything wrong; is a seat stuck; should the lane run; is the queue moving. Acts, one per thing: Resume, Pause, Stop a machine, Open log, Open goal, Answer, Land now. Where no button exists yet, one command to copy.

## 2. Today's page, reviewed (live, 2026-10-03 09:28, step 2a counted as landed)

Numbers (Playwright, `today/today-report.json`): 1430 words in the pane at 1440 px, 2901 px tall (3.2 screens). 1416 words at 390 px, 6475 px tall (7.7 screens). By section at 390 px: verdict 37, Needs you 28, This seat 102, the fleet 254, the landing lane 995 (seven "Came back" reasons of 60 to 120 words each). Sixteen "Details" disclosures. 54 column labels repeated on the phone's machine cards (six per row, nine rows).

What a person cannot tell: whether the one Needs you item is true (it is not; section 4); that m1f has been stalled since 08:20 (without 2a the row says "revising, round 6 of 20" with a live dot); what was delivered today without reading 40-word sentences; what "rung 1", "the window this seat judges by" or "claims from the accepted ledger" mean.

What a person cannot do: resume or pause the lane, stop a stuck machine (2b, not built); anything about the health item except go to a terminal; anything about a return except open the goal. The only button is Land now.

Internals shown by default: a column of nine engine hashes; generation numbers; a pid (steward-runner); eight role names in monospace (steward-runner, supervision-owner, repo-watcher, census-freshness, narrator-freshness, session-main, stop-hook-duration, capability-snapshots); a file path (/Users/wido/.../metasystem.conf); "153h56m38s"; a setting key (metasystem.runtimes); the scope sentence under the verdict, 24 words on every visit; the provenance line under the table; commit hashes and log paths inside the Came back reasons; goal ids as link text in opened rows; Box and Chain blocks with job ids and caps; the "12 roles alive" list.

## 3. The redesign

One screen at a glance. In the mocks the whole page at 1440 px is 1131 to 1321 px tall (1.3 to 1.5 screens) and holds 356 to 409 words, sidebar included, against 1430 today; the first screen holds 280 to 330, and the first phone screen 120. Top to bottom:

1. Verdict strip, one line: "All good on this computer" or "N things need you", then the counts: seats working, waiting to land, updated at, and a short qualifier that stays visible, "questions: this checkout only" (R2-8; FR-05). The long scope sentence moves into the strip's "?" help; the provenance line becomes the title of "updated". Green only when Needs you is empty and every section read (R1-05 unchanged).
2. Needs you, drawn only when it holds something. Each item is one line (the subject, what is wrong, since when; no id, pid, hash or path) and one act at the right end. A second, quiet line only where a reason exists, cut to one line; the whole reason is behind the goal (a return) or the log (a red proof). The items and their act:
   - the lane paused by X since T, with the reason: [Resume] (2b); until 2b lands, the command `metasystem landing start`;
   - the lane cannot run, with the reason, and its retry hint as the quiet explanation line (the hint is prose, and the lane keeps its one-command `Fix` out of the payload): the command `metasystem landing status` (FR-03);
   - the last proof is red, with the exit sentence: [Open log] (2a);
   - "Title" came back, first sentence of the reason under it: [Open goal];
   - "Title" on m1f has not moved since 08:20 (revising): [Stop m1f] (2b), whose second press says "Stop m1f? It ends its seat and every job on it; its steward will not start it again. [Yes, stop it]"; until 2b, the command `metasystem machine stop m1f`;
   - N goals held by m2a, unreachable since T, the titles under it, and under each title its own command with the real goal id, `metasystem goal claim <goal> --take-over --reason "<why>"`, the public form, which asks a reason (FR-02; no button exists; Talk and Forget are their own goals);
   - a seat asks a question: [Answer];
   - this computer's steward is not running: the command `metasystem system start`;
   - this computer's health check failed, with the reason: the command `metasystem system check`, and a "Details" disclosure listing each failing check's reason, one line each, no role names (FR-04);
   - this computer's health record could not be read, with the reader's reason; or its last check could not decide (the record's aggregate is "unknown"), with the undecided checks' reasons: the same command, `metasystem system check`. Both are items, so the verdict is never green over them; the facts are already in the page's health payload, nothing new is collected (FR-01).

   The one-line rule: a subject the person knows by name (the lane, a goal's title in quotes, a machine), what is wrong, since when, a full stop. Under 110 characters at 1440 px; it wraps on the phone. A reason the record carries goes on the quiet second line, cut at its first sentence. Nothing in the line is a verb to type: the act is the button, or the one command in code on the second line. One word per state, as before: Paused and Resume, Running and Pause, stalled, came back.
3. The fleet table, kept, four columns: Machine (with "this seat"), Doing (the live dot; a stalled seat reads "stalled, revising, 68 min" in the marker colour with no dot), Holds (titles, one line each), Seen ("6 min ago"; "unreachable, 3 d" in red; and the unknown standings in their own visible words from the standing's existing reason: "no presence", "presence unreadable", "unknown, clock ahead by 2 h"; FR-06). The Standing pill goes: Seen says it, and the standing's full reason becomes the Seen cell's title. Engine and generation move into the opened row, with the jobs, Box and Chain as today, and later (2b) [Stop]. On the phone a row is a card with no labels: "m1f, stalled, revising, 68 min", then the titles, then "6 min ago".
4. Landing lane: one heading line, "Landing lane, Running, 2 waiting, nothing proving", with [Pause] or [Resume] (2b) and [Land now] when it is offered. Then Waiting (title, seat, age), Proving (title, elapsed), Landed today (time and the delivered sentence, J6, kept whole). Came back is a closed disclosure, "Came back (7)", not "today": it keeps older returns nobody handed in again (FR-07); what still needs the person is already in Needs you, the rest is history. The lane's Details and each item's Details stay as they are.
5. "This seat" is no longer a section. Its facts: the name stays on its table row; armed, stale or not armed becomes a Needs you item only when it is wrong (the steward not running) and silence otherwise; "presence published 2 min ago on rung 1" moves into the row's disclosure as "published 2 min ago"; the running line is already the Doing column; the health line and the role rows become the health item's disclosure; "12 roles alive" is dropped.

Where each fact on today's page goes:

| Today | Goes |
|---|---|
| verdict words and counts | kept |
| scope sentence | "questions: this checkout only" stays beside the counts; the sentence goes behind the verdict's ? |
| Needs you words and the todo sentence | one line; the todo becomes the button or one command |
| This seat: name | the table row's "this seat" |
| This seat: armed, stale, not armed | Needs you, only when wrong |
| This seat: presence published, rung | row disclosure |
| This seat: running words | the Doing column, already |
| This seat: health line, dead and unknown roles | the health item's disclosure, reasons only |
| This seat: health record unreadable, or its aggregate unknown | a Needs you item with the reason and `metasystem system check` |
| This seat: 12 roles alive | dropped |
| table: Standing pill | dropped; Seen says unreachable, no presence, presence unreadable or unknown, with the reason as its title |
| table: Seen, Doing, Holds | kept |
| table: Engine column | row disclosure |
| opened row: engine, generation, jobs, Box, Chain | kept, opened |
| provenance line | title of "updated" |
| board and fleet read troubles | kept where they are, drawn only when they happen |
| lane state word, Land now | kept, on one line with the counts |
| Waiting, Proving, Landed today | kept |
| Came back | closed disclosure with a count |
| lane Details, item Details | kept |
| Launch a machine, the launch card | kept |

Mocks (`mocks/`, this morning's data): `all-good.html` and `all-good-1440.png`; `one-thing.html` and `one-thing-1440.png` (a red proof, [Open log]); `lane-red.html` and `lane-red-1440.png` (paused, [Resume]; a return, [Open goal]); `seat-stuck.html` and `seat-stuck-1440.png` (m1f stalled, [Stop m1f]); `phone.html` and `phone-390.png` (two items at 390 px).

## 4. Step 1 (R-121-m0)

The smallest slice that already makes the page better: the top of the page and the table. No server change, no new authority, no new fact.

Builds: `panel.ts`: a Need carries one act (open goal, answer, open log) or one command, never both, and the health item carries its failing reasons. `FleetPane.tsx`: Needs you draws the line and its act; the "This seat" block goes; the table drops Standing and Engine into the opened row; the phone cards drop their labels; the scope sentence and the provenance line move to the help and the title. `LandingLane.tsx`: the heading line with its counts; Came back as a closed disclosure with a count. `fleet.ts`: the published line for the row disclosure. `fleet.css`, `help/terms.ts` (the scope under the verdict's term; "this-seat" goes), the tests beside each, and the bundle in the same commit.

Proof, failing first. `panel.test.ts`: every Need has exactly one of act or command; the stuck item's command names its machine; a silent machine's item carries one `goal claim <goal> --take-over --reason` command per held goal with that goal's id; the lane-cannot-run item's command is `landing status` and its hint is the explanation line; the health item carries `system check`, the failing reasons and none of the alive roles; an unreadable health record and an "unknown" aggregate are each an item with their reasons, and the verdict over either is not "All good"; the Seen words for no presence, malformed presence and a clock too far ahead are the three visible labels. `FleetPane.test.tsx`: no "This seat" heading; no engine hash and no "reachable" in a row's default cells; the opened row carries the engine and the generation; a Needs you item renders its one button or its one command; the verdict strip carries "questions: this checkout only"; a phone card renders no cell label. `LandingLane.test.tsx`: "Came back (N)" is closed and counts every return not handed in again; the heading says the waiting count. Browser check: the walkthrough harness with one stalled card and one red proof, at 390 and 1440 px; the pane under 450 words at 1440 px; the first phone screen holds the verdict and Needs you.

The wrong health fact. "This computer's steward has not recorded its health since 2026-09-22 18:21" is read from `<checkout>/artifacts/agents/steward/health.json`, a record left at the checkout root on 2026-09-22; the steward writes beneath the state root, `<checkout>/metasystem/artifacts/agents/steward/health.json`, and that record is minutes old. Commit e5617a850 (ui-shows-true-facts, on main since 06:53) made every reader of the steward's records read the state root, and `TestTheSelfHostedLayoutReadsWhatTheStewardWroteBeneathTheStateRoot` (`ui_register_test.go`) pins it. The server on 7878 was built from 8136f0421 at 06:33, twenty minutes before. Nothing to build: step 1's browser check runs against a server built from main, which means 7878 restarted onto one, an act for the hand that runs it. Should the live page still say it after that, the next suspect is `armedWord` in `fleet.go`, and step 1 gets a test for it then.

Threat model: the template default (our own agents and operators make mistakes; nobody attacks); step 1 acts on nothing. Rabbit holes, refused: a dashboard or charts; a settings page for columns; rewriting the lane's reasons in the browser (the lane writes them, see below); scrubbing hashes out of reasons with patterns; a timer (the page's own refresh stays).

Later, when it hurts: 2b's Pause, Resume and Stop (designed, ruled by R-142-ui; they drop into the item slots this page leaves); a plain first sentence per return reason, written by the lane; per-check remedies in the health payload and the lane's one-command `Fix` in its payload, so those items name the exact fix instead of the diagnostic verb; Talk and Forget; fleet-wide questions; the log tailed in the page; progress bars; the dark theme checked; the stale `artifacts/` folder at the m1e checkout root removed.

## 5. Open questions for Wido

Answered 2026-10-03 by Wido in the channel: "yes" to all six as recommended; step 1 builds as drawn, and 7878 restarts onto a build from main.

1. "This seat" goes as a section; its facts move as the table says. Recommended: yes.
2. The Standing pill and the Engine column leave the table; Seen says unreachable, the engine sits behind the row. You said you like the table, so this is yours. Recommended: yes.
3. Came back closed by default, with a count. Recommended: yes.
4. Return reasons cut to one line by default, the whole behind Open goal. Recommended: yes.
5. Build order: this step 1, then 2b. Recommended: step 1 first; 2b is one day of its own and lands into the same slots.
6. Restart 7878 onto a build from main now, so the health fact is right today. Recommended: yes.

## Critique round 1 (Astra)

Six material, all in step 1, each folded above with its smallest change and no new mechanism; every claim was checked in the code first.

| Id | Material | Disposition |
|---|---|---|
| FR-01 | yes | folded: an unreadable health record and an "unknown" aggregate are Needs you items with their reasons and `system check`; the verdict is never green over them (healthNeeds dropped both) |
| FR-02 (R-126-m1e) | yes | folded: the public form `goal claim <goal> --take-over --reason`, one per held goal with its real id (`goal steal` is not a verb) |
| FR-03 (R-126-m1e) | yes | folded: the lane-cannot-run item offers `landing status`; the retry hint stays as the explanation line (it is prose; `Fix` is not in the payload) |
| FR-04 | yes | folded: the health item's one command is `system check`; Details keeps the failing reasons |
| FR-05 | yes | folded: "questions: this checkout only" stays visible beside the counts; the long sentence moves to the help (R2-8) |
| FR-06 | yes | folded: Seen shows "no presence", "presence unreadable" and "unknown, clock ahead by ..." as visible words from the existing reason; only the column goes |
| FR-07 | no | folded anyway, one word: "Came back (N)", since the list keeps older unresolved returns |

## Critique round 2 (Astra, the failsafe)

FR-01 to FR-07 all closed; no new finding. VERDICT: 0 material.
