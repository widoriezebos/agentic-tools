# g1-s27 Overview: what matters now

- Kind: design
- Id: 01M375RPSQF8K5A6K88FACNP3G
- Status: accepted
- Cites: 01M34HS374KF1RSS3EWKBGD2WE

Revision 1, 2026-09-23, Claude on Fable, on Wido's ask: "Let's build a project overview. This is to be a landing page of sorts ... come in from the UX perspective. What would you expect to see here? Design it and implement it." The master design's rule for the section is the design's spine: lead with "What needs me?" and "What changed since I last looked?", every entry linking to its record, a useful visit of a few minutes without reading a stream of activity, and a last-visit marker kept per human in server-local preference state.

## Outcome

Overview is the page a human lands on when they come back to the project. In one screen, top to bottom in importance, it answers: what needs me, what changed while I was away, what is being worked on now, what the project's memory holds, and whether anything is wrong. Every number is a link to the place where the thing is done. When nothing needs the human, it says so plainly; a calm page is the good outcome, not an empty one.

## The page

Two columns at desk width, one column at phone width, in this order:

1. **Needs you.** The primary block. Rows, each a count and the first three items with links, only for what is non-zero: goals awaiting your approval (To Do, not approved, by priority; each links to its page); open questions (most recent first); drafts awaiting acceptance (records with `Status: draft`); designs whose work has all landed but are not marked done (from g1-s26); alerts and handoffs from the steward in the last seven days (they open the notifications panel); and, when nothing proves a human on this seat, one row "Sign in to act". Empty: "Nothing needs you."
2. **Since your last visit.** The window is named: "since yesterday 18:02", or "in the last 24 hours" on a first visit. Goals concluded, goals that moved (the ledger's last verb and time), records changed (kind chip, title), and the count of steward messages, each list capped at five with "and N more →" to its section. Empty: "Nothing changed since your last visit."
3. **Work now.** In Progress goals with seat, phase and since; Next up, the first three of Ready for Work by rank; Waiting with its count and the oldest blocker's reason; and a lane strip, To Do · Ready · In Progress · Review · Waiting · Done today, every count a link that lands on the board.
4. **The project's memory.** Intent with its chapter count and the vision's first sentence; Doctrine with its chapters; Decisions (n, k draft); Designs (n, k done, and "m of n goals done" for the ones in flight); Open questions. Each links to its tab.
5. **Health.** One calm line when all is well: "Ledger synced 14:37 · records check clean". Otherwise the problems, each linking to its place: sync stale or failed, record refusals, steward messages that macOS did not deliver.

Each block title carries a help icon. The page has the refresh icon in the header's section cluster, like the others, and no timer.

## The last visit

A visit is a span of looking, not a click. The server keeps, per human and checkout, in `artifacts/agents/ui/visits.json`: when they were last seen, when the current visit began, and when the previous visit ended. A read more than thirty minutes after the last one begins a new visit; the "since" the page shows is the end of the previous visit, so pressing Refresh during a visit never narrows the window. The human is the session's, else the boot proof's, else the configured `ui.human`, else the seat itself. Losing the file changes only the comparison window.

## Data

One route, `GET /api/overview`, composes the page on the server from what it already reads: the project records (which gain `changedAt`, the file's modification time), the backlog projection (lanes, last change and verb, claims, waiting), the notifications journal, the session, and the records check. The page is one read, one shape, and the composition rules are tested on the server.

## Revision 2: at a glance

2026-09-23, on Wido's review of revision 1 on this seat's ledger: "There's a lot of text here and it looks very crowded. I need something much more accessible, intuitive and easy to navigate." Revision 1 said everything in sentences and every block competed for the eye. Revision 2 keeps the blocks and their order and changes how they are read: **numbers before words, one line per thing, detail on demand.** The payload does not change.

1. **A glance strip across the top.** Six tiles, each one number, a short label and a link, in this order: Needs you (the total, in the accent colour when non-zero), In progress, Next up (Ready for Work), Waiting, Since your visit (the count of changes), Health (a word, not a number: "ok", or "attention" in the warning colour). Tabular figures, the label under the number. Three per row at phone width.
2. **Needs you is a list of kinds, not of items.** One row per kind, only when non-zero: a count badge, the kind ("Goals awaiting your approval", "Open questions", "Drafts to accept", "Designs to mark done", "Alerts and handoffs"), and a chevron; the row is a disclosure that opens the first three items, each one line. "Sign in to act" is the last row when it applies. Empty: "Nothing needs you." with the check icon.
3. **Since your last visit is a short timeline.** Under the window line, up to five entries, each one line: the time at the left in the mono face, a small chip for what happened (the ledger's verb, "landed", or the record's kind), the title clamped to one line, and the whole line a link. The counts per kind sit above as muted text: "2 moved · 0 landed · 15 records · 3 messages". "See all" goes to the board or the Project tab.
4. **Work now is cards and a strip.** Each In Progress goal is a compact card: id in mono, the title on one line, then the seat chip, the phase chip and "since 14:38". Next up is a numbered list, one line each. Waiting is one line: the count and the oldest with its reason in a tooltip, not on the page. The lane strip becomes pills with a count each, the lane's colour as a dot, in the board's order.
5. **The project's memory is five tiles** in a grid, each an icon, a number and a word: Intent (chapters), Doctrine (chapters), Decisions (with "n draft" small), Designs (with "n done" small), Open questions. The vision sentence leaves this page; it lives on Project.
6. **Health is one row of three pills**, each a coloured dot and two or three words: the ledger ("synced 14:37", "30 minutes behind", "fetch failed"), the records ("check clean", "2 refusals"), deliveries ("all delivered", "1 undelivered"). A pill that is not fine links to its place. Only when something is not fine does a line of detail appear beneath.

Rules for every row: one line, ellipsised, the time or count right-aligned in the muted mono; the row is the link, not a blue sentence inside it; a chip is for state, never for decoration. Block titles are eyebrows in small capitals with their help icon; blocks are separated by space, not by heavier borders. The result is a page read in one glance and navigated by number.

## Later, when it hurts

- Fleet summaries and cost on the page, when Fleet is projected.
- Examination findings and production problems as entries, when the Application section exists.
- A per-human marker keyed by an encoded identity rather than the handle.
