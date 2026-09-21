# g1-s10 walkthrough evidence

Run by Claude on Opus, 2026-09-22, against `ui-development` at `1991a0a56`, served by a
real detached `metasystem ui` process at `127.0.0.1:7878` and driven in a real Chromium
through Playwright. The design's walkthrough (`g1-s10-backlog-data-path-design.md:278`)
asks for several numbers to be recorded rather than asserted; this is that record. The
human's half — the deliberate `update-ref -d`, the `sync-branch` and `sync-remote` flips,
VoiceOver — is not here and is theirs to run.

## The ledger, as observed

| Fact | Observed |
| --- | --- |
| Accepted tip | `c5d517f427e35e19c2944ab9aefee4d9c9cd9e6c`, equal to `origin/main` |
| Committed | 2026-09-21 20:39 local |
| Ledger state | `read` |
| Sync mode | `remote` |
| First tick after restart | `outcome: current`, "already at the canonical tip" |
| Stale line | shown; the tip was 3–4 h old throughout |
| `git status --porcelain` around every restart and tick | empty |

The loop's first tick ran a real fetch against the real remote on a real process start and
found the tip already canonical. That is O20's healing path exercised end to end outside a
test, and O1's claim surviving it: the working tree and HEAD were untouched.

## Lane counts

`Draft not read · To Do 119 · Ready for Work 2 · In Progress 1 · Review and Verification 0
· Waiting 33 · Unknown 0`, and 430 closed behind the toggle. The live lanes sum to 155,
which is the design's inventory exactly, and 155 + 430 = 585, which is every record on
disk: 155 under `plans/goals/` and 430 under `records/goals/`.

`bin/metasystem goal list --root .` prints `claimed=1 approved=2 queued=119 parked=33
done=429 abandoned=1`, which is the same partition by another route.

The two approved goals both appear in **Ready for Work**, not To Do, which is what the
builder predicted from their budgets sitting inside the committed tier boxes and their
`authority=proven` approvals being unexpirable.

## Two defects the walkthrough found, both fixed

**The rows reported another goal's verb.** 47 of the 155 live rows read "last done" while
queued or parked, because a priority compaction writes its verb into the history of every
goal it re-ranks and the row took the last line whatever it was. Fixed at `4b4afc5df`;
counted live before and after through `/api/backlog`: 47, then 0. The verb histogram
afterwards is `unapprove 58 · edit 42 · park 32 · open 19 · release 2 · approve 1 ·
set-budget 1`, which sums to 155 and contains no `set-priority`, every one of those having
been a fan-out.

**The list could not be scanned.** Each row printed its record's whole intent, so one goal
filled four screens and 119 of them were unreadable. The intent is now clamped to three
lines by CSS over the whole text — measured 60 px closed, 220 px open on the first row —
and opening a row releases the clamp and reveals the next step. Nothing was shortened, so
selection, find-in-page and a screen reader still reach every word.

## The page asks for nothing it was not asked for

Measured in the browser, counting only `/api/*` and `/-/*`:

| Moment | Requests |
| --- | --- |
| Load of `/backlog` | `/api/backlog`, `/api/workspace` — one each |
| `focus`, `online`, `visibilitychange`, then 4 s idle | none |
| Refresh button | `/api/backlog` — one |
| A further 20 s idle | none |

So the page never polls, and its freshness is the server's loop rather than a timer in the
browser. This is the cut guard's promise (O9) holding in a real browser rather than in its
own scanner, and it is the architectural point of the slice: the loop moved the freshness
cost to the server so the page could stay quiet.

## Layout, theme and contrast

No horizontal scroll at 1,280, 800, 599 or 400 px, and no element wider than the viewport
outside an `overflow-x: auto` container at any of them. Both themes render from the
viewer's `prefers-color-scheme`: light is `#1F1E1B` on `#F7F6F3`, dark `#ECEAE4` on
`#191917`, which compute to **15.42:1** and **14.63:1** — both above WCAG AAA's 7:1 for
normal text, against the 4.5:1 the design's table requires.

Zero Content-Security-Policy violations and zero failed requests on every page visited, in
both themes and at every width. No request left the origin.

## Not established here

The rendered ledger statements. Five of the six ledger states and four of the five fetch
outcomes have never been seen on a screen, because provoking them means deleting the
accepted ref or flipping the sync configuration, which the design reserves for the human's
word. `read`/`current` is the only pair observed live; the rest are covered by Go tests
against faked observations, and the app has no DOM harness, so their React rendering is
held by typecheck over exhaustive unions rather than by a rendering test.

Also not run: `scripts/audit.test.ts`, which wedges when several checkouts of this
repository run it at once — its `npm audit` children sit at 0 % CPU indefinitely. It is
`g1-s8` machinery that this slice does not touch. Everything the slice does touch is in
the 14 files and 140 tests that pass.
