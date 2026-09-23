# g1-s29 Ask about what you see

- Kind: design
- Id: 01M37QJM3F6W43TDPD8RKR2DB5
- Status: draft
- Cites: 01M34HS374KF1RSS3EWKBGD2WE

Revision 1, 2026-09-23, Claude on Fable, on Wido's ask: "how do we bring this to the next level, where I can ask the agent about what I see on screen (and the agent can see that too)" and his ruling on the UX proposal: "so that is what we will do. Design this, get it critiqued by astra but do not get distracted from the UX design, and then implement." Draft until Astra has read it. Builds on g1-s28, where the Partner reads and explains and is told a snapshot of the page a question was sent from.

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

## Underneath

- **Read tools.** The interface server offers the Partner's session a small read-only tool server at session open, through the protocol's own tool-server hand-off: `board(filters)`, `goal(id)`, `document(id)`, `records(kind)`, `questions()`, `overview()`, `notifications(limit)`, `search(text)`, each answering from the same readers the pages use, bounded in size, and each call becoming an activity line. The Partner can then look beyond the page it was asked from. The read-only rule and the sandbox of g1-s28 are unchanged; the tools read and nothing else.
- **Vocabulary.** The standing rule carries the interface's help texts, so the Partner speaks of tiers, lanes and arcs as the interface does.
- **The snapshot** of g1-s28 stays the Partner's account of "what the human sees now"; the tools are how it sees more.

## Later, when it hurts

- The Partner proposes, in the sheets the human confirms with.
- A picture of the screen, with consent, for visual questions.
- Threads per subject; more than one human; voice.
