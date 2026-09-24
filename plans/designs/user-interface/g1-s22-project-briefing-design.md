# g1-s22 Project, step 2: a briefing, a reader, a way to contribute

- Kind: design
- Id: 01M34HS374KF1RSS3EWKBGD2WE
- Status: draft
- Goals: browser-interface
- Cites: 01M348YTJ2EY11F37Y5KBVNSER 01M348YTJ290F6VMX6TWV1M1WZ
- Supersedes: 01M348YTJ2688CC90GYDBYGJB5

Revision 2, 2026-09-23, on Wido's ruling: the Contribute column becomes one contextual New action in the tab strip; a record's scope comes from the page it is written from; intent and doctrine never name goals.

Revision 1, 2026-09-22, Claude on Fable, after Wido's review of step 1 in the browser: "a bunch of documents filtered and ordered somehow; open one and it is a huge text blob; I can contribute nothing" — and his placement of the Brain along the bottom. The clickable mockup, built from this checkout's real record, is the design's picture; this page is its contract. Step 1 (the memory system, sections 2–4) is the structure it reads; this step adds no kind, no home and no field.

## Outcome

A human opens Project and reads a briefing, not a list: what this project is, in the intent's own words; how it is shaped, in the doctrine's; what has been decided; which designs govern and which shipped; what is open. Opening a record reads like a chapter of a book — the head as a strip of facts, the siblings on the left, the outline on the right, previous and next at the bottom. Contributing is one click: a draft decision, design, question or fact in the right home with its head filled in, opened for writing. The Project Partner is a drawer along the bottom that always names the page it is about, and every action above can be done with it.

## What changes, and what it rests on

| Change | Rests on (step 1) | New |
| --- | --- | --- |
| **Briefing.** Each kind opens with its own words: the intent index's first paragraph, the doctrine's, a design's or decision's first paragraph as its one-line summary. Books render as numbered tables of contents in two columns. Designs group by status, shipped ones in one collapsed run. Empty kinds carry their action. | `Kind`, `Status`, `Areas`, the books' `## Chapters` | the convention that a record's first paragraph after the head is its summary; the payload carries it |
| **Goal page, under Backlog.** Wido, 2026-09-22: the backlog is seen at Backlog, never at Project. Project's left rail is an outline of its own sections. A goal's page lives at `/backlog/goal/<id>`, reached from the backlog list and the board, and shows the goal's own intent (its `Intent:` line and state from the ledger, with a link to it in the Backlog), then the designs, decisions and open questions whose `Goals` name it, and a Contribute block whose drafts carry `Goals: <id>`. Doctrine and the intent book are project-level and are not repeated per goal. Areas were removed on 2026-09-22 (Wido): goals are the only subdivision. | `Goals:`, the goal ledger | nothing |
| **Reader.** Breadcrumb; the head as a facts strip (kind and area as an eyebrow, status as a control, id and path in mono, "Open in editor"); relationships as links (rests on, superseded by, referenced by); the left rail lists the siblings — the area's designs, or the book's chapters — with previous and next; the right rail is the sticky outline; the column is 76 characters wide. | `Cites`, `Supersedes`, the reverse index, the books' reading order | the reading route stops rendering the head as body text |
| **Contribute.** Four actions — record a decision, new design, ask a question, note a fact — each creating a draft record in its home with a fresh id, `Status: draft`, the current area, and `Affects` when started from a record; the ADR template for decisions. Status is a select on the facts strip that rewrites the one head line. "Open in editor" opens the file locally. | the homes, the head grammar, `project id` | **one write route on loopback**: create a record file; rewrite a `Status:` line; append a question row. It is the human editing their own checkout through the browser; it records no authority, and it is the surface gate 3's principals will attach to |
| **The Project Partner drawer.** A bar along the bottom with the composer and the context it would receive ("Project · Interface", "g1-s21 · Outcome"); opening it lifts a panel under the page. The right column is freed for Contribute and the outline. | — | the dock moves from the right to the bottom; the shell's layout changes for every section |

## Is the structure sufficient?

Yes, for reading, navigating and contributing at this step. Nothing in the briefing or the reader needs a field step 1 does not have; the summary is a convention, not a key. What the structure cannot yet do is prove who accepted what — `Status` is a line anyone can edit — and that is the first row of the memory system's "later, when it hurts" table, due when a second decision-maker joins or an acceptance is disputed, not before.

## With the agent

Everything a human can do here can be done together with the seat's agent, and the design makes that one surface, not two. Every contribution is a **proposal in a sheet**: the exact change as it will be written — a new record page, a changed `Status:` line, a question's answer, an intent chapter — shown before it is written, editable, and confirmed by the human. Who filled the sheet is the only difference: the human typed it, or the Project Partner drafted it from the conversation in the drawer, whose context always names what the sheet is about ("New decision · Interface", "g1-s21 · Status"). The write route is the same in both cases and runs on the human's confirmation. This is the master's own rule — "an agent's proposed change appears as an editable before-and-after comparison with its stated reason; the human can accept, adjust, reject" (`plans/designs/user-interface-design.md`, "Backlog item detail") — applied to the Project section, and it is why step 2 builds the sheet now, before the agent is connected: at gate 3 the agent gains a way to fill it and nothing else changes.

**The word.** Wido, 2026-09-22: "brain" is not the word for the agent we collaborate with on the whole seat. The interface stops using it: the drawer, its copy, the empty states and the title say the chosen word instead. The engine keeps its `brain` verb family, `internal/brain` and the role packet until a deliberate rename, because those are the kit's names for the seat's process, not the interface's name for the collaborator. Wido chose **Project Partner** (2026-09-22): the partner for the project — its intent, architecture, decisions, backlog and priorities. The drawer, its copy, the empty states and the tab title say "Project Partner"; the context line reads "Project Partner · about g1-s21".

## Slices

1. **Briefing and reader** (~4h): the payload gains summaries and chapter summaries; the pane renders the briefing and the area page; the reader gains the facts strip, the rails, previous/next and the relationship links; the head is no longer rendered as text. Read-only.
2. **Contribute and the drawer** (~4h): the write route (create a record, set a status, append a question), the four actions and the decision template, "Open in editor"; the Brain dock becomes the bottom drawer in the shell.

Verification: canary only — the changed packages' tests, typecheck, the frontend tests, the bundle; then a browser walkthrough by Claude, then Wido.

## For Wido

One decision: **the write route on loopback now** (recommended: yes — it is the same act as opening the file in an editor, and without it the section stays a viewer), or read-only with "Open in editor" as the only way in until gate 3.
