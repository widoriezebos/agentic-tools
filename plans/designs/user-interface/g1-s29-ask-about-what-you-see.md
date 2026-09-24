# g1-s29 Ask about what you see

- Kind: design
- Id: 01M37QJM3F6W43TDPD8RKR2DB5
- Status: done
- Goals: project-partner
- Cites: 01M34HS374KF1RSS3EWKBGD2WE

Revision 2, 2026-09-23, Claude on Fable, after Astra's critique of revision 1 ([g1-s29-astra-critique.md](g1-s29-astra-critique.md): nine material findings, "build after listed changes", every one a missing contract rather than a wrong idea; all nine are the contracts below). Revision 1 was on Wido's ask: "how do we bring this to the next level, where I can ask the agent about what I see on screen (and the agent can see that too)" and his ruling on the UX proposal: "so that is what we will do. Design this, get it critiqued by astra but do not get distracted from the UX design, and then implement." Builds on g1-s28, where the Partner reads and explains and is told a snapshot of the page a question was sent from.

## The principle

You never have to describe what you are looking at, and you can always see what the Partner is looking at. Everything below serves that sentence; the plumbing at the end exists only to make it true.

## The page

1. **Ask from the thing, not from the box.** Every object gets the same affordance, "Ask about this": in the context menu of a goal card and a list row, on a record's row in a Project tab and on its page, in a lane's header, on an Overview item. Choosing it makes the thing the subject, opens the drawer, focuses the composer, and shows three suggested questions for that kind of thing, as chips above the composer: for a goal "Why is it here?", "What would move it?", "What does its design say?"; for a design "Has its work shipped?", "What does it leave out?", "Which goals does it name?"; for a decision "What did it decide, and why?"; for a lane "What is in here, and why?"; for the board "What needs me here?", "What changed today?", "What is next up?"; for a document "Summarise this", "What does it rest on?". A chip sends its question. Suggested questions live in one register beside the help texts.
2. **Selected text is a subject.** Select a passage anywhere the interface renders prose and a small "Ask" appears beside the selection; choosing it sends the passage as the subject with the page it came from. Only saved text is shared, never an unsaved edit.
3. **Show what it sees.** The drawer's "about:" chip becomes "Seeing:" and says it in one line: "Backlog · board · 12 goals shown · filters tier 1 · tip 6984cde, 17:54". Clicking it opens a sheet with the exact block the Partner will be given for the next question. Every answer carries a small stamp, "from tip 6984cde, 17:54"; when the page's tip or observation has moved since, a muted "Refresh what it sees" appears on the chip and the next question refreshes it.
4. **Let it point back.** In an answer, a goal id, a record id or a record title the ledger or the records know becomes a link to its page. Hovering a goal link on the board rings that card with the landing's ring; clicking anywhere navigates. "Show me" is a hover.
5. **"This" keeps its meaning.** Every human message in the transcript wears the chip of the page and subject it was asked from; clicking the chip takes you to that page and subject. Weeks later the conversation still reads.
6. **Quiet activity.** While the Partner works, one line says what it is doing now; when done, "Looked at 3 things ▸" collapses under the answer and expands to the list.
7. **Two modes, one conversation.** The drawer for a quick question from anywhere; the focused page for a long discussion, where the subject is pinned beside the conversation, its summary as the pages show it (a goal's row, a record's head), so the thing under discussion never scrolls away. One conversation, one draft, in both.
8. **A good first minute.** An empty drawer offers the three suggested questions for the page you are on. Cmd/Ctrl+J focuses the composer from any page; Escape closes the drawer.

## The contracts

1. **One capture per question.** When a question is sent, the page captures what it shows: the tip and observation time it rendered from, the view, filters, Done window, tab, the chosen subject or passage, and the ids it names. The "Seeing:" sheet shows the block composed from that capture; the same block goes with the question and is kept with the message; the answer's stamp names it. If the server's facts are from a later tip than the page's, the block says both and so does the stamp. "Seeing:" always speaks of the *next* question ("Will see"); an answer's stamp speaks of what was given ("Saw"). "Refresh what it sees" prepares the next capture and changes no stamp.
2. **What it read is a list, not a count.** "Looked at N things ▸" expands to one line per read: what (a goal, a record, a lane, a passage), its source (the accepted tip or the file's revision, "as it stands"), and its outcome (read, failed, partial), with the returned excerpt behind it. A failed read is never counted as a look. The page snapshot is listed first, separately.
3. **The subject follows the page until you choose one.** A chosen subject (by "Ask about this" or a selection) stays until you clear it with the × on its chip or choose another; navigating does not change it; expanding to the focused page keeps it. The pinned panel shows the chosen subject, else the page's, with its identity and source, for every kind this slice offers: a goal's row, a record's head, a lane's rows, an Overview item's line, a passage's quote.
4. **The subject is never left out.** The chosen subject or passage goes first and whole; the block says "N of M rows supplied" and the tools take a cursor so the rest can be fetched. Suggested questions name their scope: "What needs me here?" is the Needs-you set on Overview and the lane's rows on the board; "What changed today?" is since local midnight through the changes the server knows; a question the page cannot scope is not suggested.
5. **A suggestion respects your draft.** With an empty composer a chip sends; with a draft present it inserts its words at the cursor and sends nothing. "Ask" on a selection attaches the quote as a removable chip above the composer, with its source, and focuses the composer; nothing is sent until you send. Inside the editor there is no selection Ask.
6. **One way in from every surface.** Cards and list rows: the existing context menu (right-click, Shift+F10, the Menu key) gains "Ask about this" as its first item; the list gains the same menu. Lanes: a small menu control in the header. Records: "Ask" in the page's actions row and in the row's menu. Overview items: "Ask" at the row's end on hover and focus; the row itself still navigates. Selection: the floating "Ask" appears only over reading surfaces, never over a draggable card. Ask moves focus to the composer; Escape from the composer returns it to where you came from; Escape closes the foremost thing first (menu, selection control, sheet, then drawer); Cmd/Ctrl+J focuses the composer unless a sheet is open. The first-minute chips say how to ask about a thing.
7. **Links point at one thing.** Only stable ids link, and titles only when unique. Hover or focus rings the one visible matching card and nothing else; clicking navigates, through the board's landing when the goal is hidden or elsewhere; hover never navigates or clears a filter. Six mentioned goals mean six links and no rings until one is hovered.
8. **A message chip returns you to what you meant.** The chip carries the capture of contract 1; clicking it opens that page with that view, filters, window and tab, through the board's landing mechanism, and the subject or passage anchor; a status line says what was applied. A subject that no longer exists, or a passage that has moved, is said in that line, never silently replaced by today's page.
9. **The tool server is the one named exception.** The interface server offers its read tools as a stdio tool server it starts itself; the permission rule admits calls to that server's named read operations and refuses everything else as before. The sandbox and the write fence are unchanged.

## Underneath

- **Read tools.** The interface server offers the Partner's session its read tools at session open, through the protocol's own tool-server hand-off (contract 9): `board(filters)`, `goal(id)`, `document(id)`, `records(kind)`, `questions()`, `overview()`, `notifications(limit)`, `search(text)`, each answering from the same readers the pages use, bounded in size with a cursor for the rest, and each call becoming a line in what it read (contract 2). The Partner can then look beyond the page it was asked from. The read-only rule and the sandbox of g1-s28 are unchanged; the tools read and nothing else.
- **Vocabulary.** The standing rule carries the interface's help texts, so the Partner speaks of tiers, lanes and arcs as the interface does.
- **The snapshot** of g1-s28 stays the Partner's account of "what the human sees now"; the tools are how it sees more.

## Built, 2026-09-23

Commits `2a326e201` (the Partner can look: the tool server, the looked list, the vocabulary) and `d5831d759` (the page). One finding for the record: Claude's adapter auto-allows calls to the handed-over tool server without a permission request, so the permission rule is a backstop that runtime does not exercise; it is proven with the fake server. The tool server is the engine itself, `metasystem ui tools`, over stdio.

## Later, when it hurts

- The Partner proposes, in the sheets the human confirms with.
- A picture of the screen, with consent, for visual questions.
- Threads per subject; more than one human; voice.
