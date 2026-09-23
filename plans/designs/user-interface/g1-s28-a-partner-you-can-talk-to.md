# g1-s28 A Partner you can talk to

- Kind: design
- Id: 01M37EX4M5VVXQBPH137CTM89V
- Status: accepted
- Cites: 01M34HS374KF1RSS3EWKBGD2WE

Revision 1, 2026-09-23, Claude on Fable, on Wido's ask: "I want to get started on that one so that we have an integrated agent that we can discuss anything, including what is visible on the UI ... make sure it is agent independent (i.e. supports claude, codex and devin and some future agent). Get me the smallest thing that works now." The master design's "The brain" and its integration table are the target: an ACP session host, an MCP endpoint of bounded tools, proposals that become sheets, shared selection. This is step 1 of that, and nothing in it has to be undone to get there.

## Outcome

A human opens the drawer and talks with the Project Partner about the project and about what is on the page: "why is this goal waiting?", "summarise the doctrine", "what did the last three decisions decide?". The Partner reads the same checkout the pages read, remembers the conversation across page loads and server restarts, answers as it thinks, and can be stopped mid-answer. It does not write and it does not act: those come as proposals in the sheets that exist, in step 2. Which agent is the Partner is one setting, and adding an agent is one file.

## The seam

One contract in the interface server, `internal/ui/partner`, in the ACP shape the master design names: open a session (fresh, or resumed from an opaque token the runtime gave back), prompt a turn with text, receive the turn's events as a stream (text as it arrives, activity such as "reading plans/goals/g1-s23.md", done with what the runtime reports, or an error), cancel. Nothing above the seam knows a runtime's flags, its log format or its session ids. Runtime names are the kit's own, `metasystem.runtimes`.

Step 1 wires three runtimes behind it:

- **claude**: Claude Code headless, one turn per message, streaming, resumed from the previous turn, with a read-only tool allowlist (read, search, list, and the kit's read-only verbs). The page's context goes in an appended system prompt.
- **codex**: `codex exec --json` in the read-only sandbox, resumed with `codex exec resume`. The context goes at the head of the prompt in a delimited block.
- **devin**: `devin acp`, driven by the kit's own ACP transport, `internal/acp`; the first runtime that speaks the seam's shape natively.

A runtime that is configured but not installed, or not signed in, refuses the first turn with the runtime's own words; the drawer shows them.

## The conversation

One conversation per human per checkout (the session's human, else the boot proof's, else the configured one, else the seat), kept under `artifacts/agents/ui/partner/`: the transcript as one JSON line per message, and the runtime's resume token. One turn at a time. Every turn carries the page: the section, the goal or document open, its tab, the board's filters, and the Partner's standing rule (it reads and explains; it does not write; it says when it cannot see something). The drawer's "about:" chip is that context made visible.

The drawer and the focused page render the transcript, the human's turns and the Partner's, the answer streaming in as text and, once complete, rendered as Markdown through the reader's own renderer. While the Partner works, one muted activity line says what it is doing; a Stop control ends the turn. The composer's "unavailable" chip goes. The model and the runtime come from `ui.partner.runtime` and `ui.partner.model` (defaults: claude, `claude-opus-5-5`; codex defaults to `gpt-6-sol`).

## Later, when it hurts

- Proposals: the Partner fills a sheet the human confirms, through the shared operations, with its authorship recorded and never the human's authority.
- The MCP endpoint of bounded tools, and the ACP host in the seat's process, so a sitting can carry a Partner with write tools under the brain's fences.
- "Discuss this" from a card, a row or a selection; sharing an unsaved edit as draft context.
- Claude and Codex natively through ACP once their adapters speak it; more than one human; usage and cost on the page.
