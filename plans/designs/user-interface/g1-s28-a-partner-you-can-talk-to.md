# g1-s28 A Partner you can talk to

- Kind: design
- Id: 01M37EX4M5VVXQBPH137CTM89V
- Status: draft
- Cites: 01M34HS374KF1RSS3EWKBGD2WE

Revision 2, 2026-09-23, Claude on Fable, after Wido's two rulings: "Both claude and codex I'm told have a separate server component that will do acp for them ... we should definitely factor that in" and "we must have the design for this critiqued by Astra first before we implement". Revision 1 wrapped Claude and Codex behind the seam through their command lines; revision 2 drops those emulators, because every runtime has an ACP server. Draft until Astra has read it. Revision 1 was on Wido's ask: "I want to get started on that one so that we have an integrated agent that we can discuss anything, including what is visible on the UI ... make sure it is agent independent (i.e. supports claude, codex and devin and some future agent). Get me the smallest thing that works now." The master design's "The brain" and its integration table are the target: an ACP session host, an MCP endpoint of bounded tools, proposals that become sheets, shared selection. This is step 1 of that, and nothing in it has to be undone to get there.

## Outcome

A human opens the drawer and talks with the Project Partner about the project and about what is on the page: "why is this goal waiting?", "summarise the doctrine", "what did the last three decisions decide?". The Partner reads the same checkout the pages read, remembers the conversation across page loads and server restarts, answers as it thinks, and can be stopped mid-answer. It does not write and it does not act: those come as proposals in the sheets that exist, in step 2. Which agent is the Partner is one setting, and adding an agent is one file.

## The seam is ACP, and nothing else

The Agent Client Protocol is the shape the master design names, and every runtime speaks it as a server over stdio today: Devin natively (`devin acp`), Claude Code through `@zed-industries/claude-code-acp` (the official `@agentclientprotocol/claude-agent-acp` is the same shape), Codex through `@zed-industries/codex-acp`. So the interface has **one** client, the kit's own `internal/acp` (it already drives Devin), and a runtime is nothing but a command that starts an ACP server. Step 1 wires the three; a future agent is one more command in one setting.

What the client owns, at the one protocol point the master design puts it: initialise; open a session in the checkout, or load one where the server declares that capability; send a prompt turn; receive the session's updates as a stream (agent text as it arrives, tool calls with their titles as activity, the turn's end, an error); cancel; and **answer every permission request the server raises**. The Partner's standing rule is a policy at that point, not a flag on a command line: reads and searches are allowed, anything that writes a file or runs a command that could write is refused, and the refusal is shown to the human as an activity line. No runtime's flags, log format or session ids exist above the seam.

Settings: `ui.partner.runtime` (`claude` | `codex` | `devin`, default `claude`), `ui.partner.model` (default by runtime: `claude-opus-5-5`, `gpt-6-sol`, Devin's own), and `ui.partner.command.<runtime>`, defaulting to `claude-code-acp`, `codex-acp` and `devin acp`. A command that is not installed, or a server that refuses to initialise or is not signed in, yields one message in the drawer with the server's own words and, for the two adapters, the one line that installs them. The interface never installs anything itself.

## The conversation

One conversation per human per checkout (the session's human, else the boot proof's, else the configured one, else the seat), kept under `artifacts/agents/ui/partner/`: the transcript as one JSON line per message, and the ACP session id. The ACP server process stays alive for the conversation while the interface server lives, idle for at most an hour; when the server has restarted, or the runtime declares no session loading, the next turn opens a fresh session and gives it the transcript's last ten messages as context, and says so in an activity line, so the human's memory and the Partner's do not part. One turn at a time. Every turn carries the page: the section, the goal or document open, its tab, the board's filters, and the Partner's standing rule (it reads and explains; it does not write; it says when it cannot see something). The drawer's "about:" chip is that context made visible.

The drawer and the focused page render the transcript, the human's turns and the Partner's, the answer streaming in as text and, once complete, rendered as Markdown through the reader's own renderer. While the Partner works, one muted activity line says what it is doing; a Stop control ends the turn. The composer's "unavailable" chip goes. The model and the runtime come from `ui.partner.runtime` and `ui.partner.model` (defaults: claude, `claude-opus-5-5`; codex defaults to `gpt-6-sol`).

## Later, when it hurts

- Proposals: the Partner fills a sheet the human confirms, through the shared operations, with its authorship recorded and never the human's authority.
- The MCP endpoint of bounded tools, and the ACP host in the seat's process, so a sitting can carry a Partner with write tools under the brain's fences.
- "Discuss this" from a card, a row or a selection; sharing an unsaved edit as draft context.
- Usage and cost on the page from the turn's end; more than one human; a session that survives the server's own restart where the runtime can load one.
