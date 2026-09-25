# g1-s50: Stickies — a notepad that knows what it is about

- Kind: design
- Id: 01M3CZM7BF6JTCRG6RDAN2YGAW
- Status: draft
- Goals: browser-interface

Wido, 2026-09-25: "I want a notepad of sorts where I can put reminders.
Maybe something like stickies. Stored as well, UX wise, how would we do
this and how can we make this both simple and powerful, especially if
stickies relate to goals, designs etc etc. UX cap on", and "go" the same
day. Author Fable. Step 1; every cite re-read at `8b7d09b7e`.

## 1. What exists and binds

1. **The interface keeps per-human state in the state root, outside git.**
   Overview's last-visit marker lives in `artifacts/agents/ui/visits.json`
   under one mutex with atomic writes (internal/ui/overview/visit.go:93-135);
   the directory is ignored by git; the Fleet page keeps its fetch
   metadata beside it (fleet/fetch.go:177). The human a request acts as
   is known to the server: the signed-in session's, else the boot
   authority's, else the seat's configured handle
   (internal/ui/httpd/session.go:88-100).
2. **Checkout writes need no ledger authority.** The record and question
   routes take `checkoutHand`, "a request from this browser to this
   loopback server; the write lands in the checkout and records no ledger
   authority" (httpd/describe.go:27-30, 55-67), with POST, host, site and
   origin checks and a bounded JSON body.
3. **A side panel exists.** The notifications panel opens from a bell in
   the header, closes on Escape, lists cards and loads older ones
   (src/notifications/Bell.tsx:23-48, Panel.tsx:24-33, store.tsx:47-61);
   the header holds its icon buttons (src/shell/Header.tsx:68-101).
4. **Picking a goal exists.** `GoalPicker` offers the ledger's goals by id
   and intent, refusing one already chosen (src/shell/GoalPicker.tsx:39,
   130-166); the goal page knows its goal from the route
   (`/backlog/goal/:id`), the document reader its path.
5. **The Partner sees what the page shows.** Each page composes a capture,
   `captureOf`, and the drawer's "Seeing" line says it
   (src/partner/capture.ts:45-293).
6. **The master keeps personal working material out of the project's
   records** until the working-material owner exists: notes are
   server-local sitting material, examiners never read them
   (user-interface-design.md:383-385, 896). Ids are minted by
   `project.NewID` (internal/project/project.go:275).

## 2. The human, and the jobs

The human is in the middle of a sitting, sees something, and wants to
not lose it: "ask Sol about the retry", "check g1-s45 tomorrow", "the
launch sheet's wording is off". They want to jot it in two seconds
without leaving the page, and to find it again where it belongs. They
need the note to be theirs: private, durable, never a project record and
never a seat's input. What it must do: stay out of the ledger and the
records; survive restarts and rebuilds; know what it is about so it
comes back when that thing is on screen.

Jobs:

- **J1. Jot it now, from anywhere.** One field, one key, done.
- **J2. Say what it is about, with one click.** The thing on screen, or a
  goal picked by name; the note then shows where that thing is.
- **J3. See mine at a glance and strike them off.** Open ones first,
  newest first; done ones kept but out of the way.
- **J4. Ask the Partner about them.** "What did I want to remember about
  the Fleet page?" The Partner sees what the panel shows.
- **J5. Never lose one.** Written to disk on every change, per human,
  outside git, bounded.

Decisions:

- D1. **A sticky is private state, not a record.** One file,
  `artifacts/agents/ui/stickies.json`, in the state root beside the
  visit marker, one owner package with the same mutex and atomic write;
  keyed by human; each sticky `{id, text, about: [{kind, id}], createdAt,
  updatedAt, doneAt}`; text at most 2000 characters, one human at most
  500 stickies, both refused beyond with the words. Seats never read it;
  the examiner's boundary is untouched because the file is neither a
  record nor in git.
- D2. **The panel is the notepad.** A sticky-note icon in the header
  beside the bell, with the count of open stickies; it opens a side
  panel in the notifications panel's chrome: the composer at the top,
  then open stickies newest first, then "Done (n)" behind a disclosure.
  A sticky is a card in the marker tokens the sign-in bar uses, so it
  reads as a note: the text, the about chips as links, the age, "Done"
  and "Edit" and "Remove". Enter in the composer saves; Escape closes
  the panel.
- D3. **About is one click.** The composer offers the thing on screen as a
  chip already on: the goal on a goal page, the document on a document
  page, nothing elsewhere; and "Add a goal…" through `GoalPicker`. A
  sticky may be about several things. Records other than goals are
  picked only from their own page in step 1.
- D4. **Stickies come back where they belong.** A goal page shows the open
  stickies about that goal under its header, with "Add a sticky" opening
  the panel with the goal chip on; the document reader the same for its
  path. The panel itself is the whole list.
- D5. **The Partner sees the panel.** The page capture carries the open
  stickies about the page's subject and the panel's count; a `stickies`
  tool and acting on them are step 2.
- D6. **Acts are the human's own, not the ledger's.** Create, edit, mark
  done or undone, remove: each a POST with the checkout-write policy, each
  answering the whole list; no sign-in demanded beyond the human the
  server already knows; when it knows none, the panel says "sign in so
  your stickies are yours".
- D7. **Phone width**: the panel is the full width, as the notifications
  panel is.

## 3. The payload and the routes

```
GET  /api/stickies                → { "schemaVersion": 1, "human": "Wido",
                                      "stickies": [ { "id": "…", "text": "…", "about": [ { "kind": "goal", "id": "g1-s45" } ],
                                                      "createdAt": "…", "updatedAt": "…", "doneAt": "" } ],
                                      "counts": { "open": 3, "done": 12 } }
POST /api/stickies                { "text": "…", "about": [ … ] }            → the same
POST /api/stickies/<id>           { "text"?: "…", "about"?: [ … ], "done"?: true|false } → the same
POST /api/stickies/<id>/remove    {}                                         → the same
```

`about.kind` is `goal` or `record`; an unknown kind or an empty id is
refused. Order: open by `createdAt` descending, done by `doneAt`
descending. The store's file is the whole truth; a missing file is no
stickies. Route ids `stickies`, `add-sticky`, `edit-sticky`,
`remove-sticky` in the describe table with `checkoutHand`.

## 4. Not here, step 2 and later

A `stickies` Partner tool and the Partner acting on a sticky. Picking a
record by name from the composer. Stickies on Overview. Due dates,
colours, ordering by hand. Sharing a sticky with a seat, which would make
it an ask or a goal. Sync across machines, which the working-material
owner brings at gate 5.

## 5. Verification and box

Go: the store's create, edit, done, remove, order and both bounds, a
missing file, per-human separation, the atomic write; the routes'
policy, bodies and refusals. Frontend: the bell-side button and count,
the panel's composer with Enter and Escape, the about chip from a goal
page and from a document page, the picker adding a goal, done and edit
and remove, "Done (n)", the goal page's block and its "Add a sticky", the
capture; the guards stay green. The walkthrough fixture keeps a store
with six stickies for the fixture's human; screenshots at 1280 and 400:
the panel with the composer, a sticky about a goal on that goal's page,
phone. Budgets as always. Box: one build lane (Claude on Opus), one code
read (Codex on Sol) with one fix round under R-124, after Astra's read;
two attempts, 120 to 180 job-minutes.

## 6. Self-grade

High on D1, D2, D6: one small store in the pattern the visit marker set,
routes in the pattern the record writes set, a panel in the pattern the
bell set. Medium on D3 and D4: the "thing on screen" is derived from the
route, which is exact for goals and documents and absent elsewhere.
Weakest: private state on one host, which the master defers to the
working-material owner; the page says a sticky lives on this seat.
