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
goal it re-ranks and the row took the last line whatever it was. Fixed at `4b4afc5df` and then corrected at `67de893fc` after Sol's review found the fix
wrong in the other direction: the engine also folds a compaction into a goal's OWN event,
so skipping back past every rank line reported a just-reopened goal as done. The row now
keeps the date, which is exact, and withholds the verb when the last line is a rank clause,
because nothing stored says which goal the operation was about. Counted live through
`/api/backlog`: 47 rows falsely reading "done" before, 0 after; 89 of 155 rows now show
"last changed <date>" with no verb, which is the true shape of a ledger that has had large
priority reorderings, and reads as `opened 2026-09-16 08:44 · last changed 2026-09-19 22:35`. The verb histogram
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

## Every ledger state, rendered

The gap this note first recorded is now closed, without touching the live ledger. The
real published bundle was served from a throwaway static server on another port, with
`/api/backlog` answering faked payloads, and each state was opened in Chromium. This
exercises the real React, the real CSS and the real typed unions; only the payload is
synthetic.

All seven forms render, each naming what it observed and what the loop last did, with no
console error and no unhandled rejection in any of them:

| Form | What the pane says |
| --- | --- |
| `absent`, fetch `never` | names the ref, explains that `git clone` and `git fetch` never bring it, and counts the working tree's 155 and 430 as a working-tree fact |
| `absent`, fetch `running` | the same, plus "Fetching since 00:51:49." |
| `absent`, fetch `failed` | the same, plus git's own words and the retry time |
| `no-ledger` | names the tip, says the engine never sets the ref to such a commit, and gives the one terminal step the design sanctions |
| `broken` | names the unreadable ref and the last tick's words, and asks for repair |
| `unreadable` | names the tip, says the engine refuses a tree whole, and lists every typed problem one per line |
| `refused` | carries the engine's own sync-mode sentence and points at the two git config keys |

No rendered statement contains "goal fetch", "does not fetch" or "none read", and the only
terminal command anywhere is the `no-ledger` `git update-ref -d`, which is O7 and O20
holding on a screen rather than in a grep.

**One latent ambiguity found, not a defect today.** An empty `fetch.nextAt` renders as
"The server is stopping, so no fetch is due." But the loop's state before `Run` starts is
byte-identical to its state after `Run` returns: `outcome: never`, `nextAt` zero. The
pre-start reading is unreachable over HTTP, because `lifecycle.Serve` calls `Ready` — which
opens the gate — before it calls `server.Serve`, so no request is answered until the loop
has been released. The wording is therefore correct today, and it is correct only because
of that ordering. If the gate ever moved, the page would tell a starting server it was
stopping. Worth a distinguishing flag if anyone touches the wiring.

## Not established here

The human's half of the walkthrough: deleting the accepted ref on purpose and watching the
loop recreate it, the `goal.sync-branch` and `goal.sync-remote` flips against a live
server, and VoiceOver reading each lane heading with its count. The states above were
rendered from faked payloads, which proves the page renders them; it does not prove the
server produces them, and only the live provocations do that. The Go tests cover the
server's side.

Not run: `scripts/audit.test.ts`, which wedges when several checkouts of this
repository run it at once — its `npm audit` children sit at 0 % CPU indefinitely. It is
`g1-s8` machinery that this slice does not touch. Everything the slice does touch is in
the 14 files and 140 tests that pass.
