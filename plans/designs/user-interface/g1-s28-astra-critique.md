# Astra's critique of g1-s28 revision 2

Produced 2026-09-23 by Codex on `gpt-6-astra`, read-only, against revision 2 of [g1-s28-a-partner-you-can-talk-to.md](g1-s28-a-partner-you-can-talk-to.md) at `72cffb3a3`, under R-121 and the design-critique skill's step-1 rule. Verbatim; the dispositions are in the design's revision 3.

---

The draft passes the step-1 gate: it names a usable outcome and a deferred list. Applying [R-121](../../../metasystem/memory/rulings.md:180), I found **six material findings**.

Evidence is from reading code, tests, and public documentation. I confirmed `ui-development` at `72cffb3a3`. No edits, tests, agent launches, or reads of `metasystem.conf.local`.

1. **Critical — Permission requests cannot enforce the promised read-only boundary.**

   [Design:18](g1-s28-a-partner-you-can-talk-to.md:18) makes the permission callback the enforcement point. The kit explicitly says it is only a backstop: the runtime’s selected mode supplies enforcement ([decide.go:53](../../../metasystem/internal/acp/decide.go:53)). Its live request handler ignores tool details and refuses every request; it does not invoke the effect classifier ([turn.go:352](../../../metasystem/internal/acp/turn.go:352)).

   ACP permits an agent to request permission; it does not require every tool execution to pass through that request. The request can contain only a tool-call ID, while classification fields are optional and `rawInput` has no common schema. Consequently, “could this command write?” is not generally decidable from ACP metadata. Titles and `kind` are insufficient authorization evidence. [ACP tool-call specification](https://agentclientprotocol.com/protocol/v1/tool-calls)

   This matters with ordinary configurations, without a malicious server: Claude’s permission callback is bypassed by earlier approvals, and Devin documents modes that automatically permit workspace edits or shell execution. [Claude permissions](https://code.claude.com/docs/en/agent-sdk/permissions), [Devin permissions](https://docs.devin.ai/cli/reference/permissions)

   **Failure scenario:** a preapproved tool writes before the client receives anything it can refuse. More seriously, an available shell or HTTP-capable tool posts to the interface’s existing human-act endpoint. Missing `Origin` and `Sec-Fetch-Site` headers pass the HTTP checks, and a request without a cookie can use the server’s boot authority ([httpd.go:234](../../../metasystem/internal/ui/httpd/httpd.go:234), [httpd.go:503](../../../metasystem/internal/ui/httpd/httpd.go:503), [acts.go:208](../../../metasystem/internal/ui/httpd/acts.go:208)). An agent-originated request could therefore become a human act.

   **Required change:** name the enforced tool/runtime restrictions below ACP, including inherited permissions, command execution, external tools, and access to human-act endpoints. Admit only configurations that establish those restrictions; deny unclassifiable requests. Human identity in conversation context must remain attribution, never authority. The existing inert Markdown renderer is suitable; it does not execute agent-provided HTML ([Markdown.tsx:10](../../../metasystem/internal/ui/web/_app/src/project/Markdown.tsx:10)).

2. **High — The reusable transport exists; the promised interactive host does not.**

   The existing code can initialise, create or capability-gate loading a session, prompt, stream updates, answer requests, and send cancellation. But its lifecycle differs materially from [design:16–24](g1-s28-a-partner-you-can-talk-to.md:16):

   - `PromptTurn` consumes the session permanently; another call returns `ErrSessionExhausted`. The test explicitly requires this ([driver.go:117](../../../metasystem/internal/acp/driver.go:117), [driver_test.go:488](../../../metasystem/internal/acp/driver_test.go:488)).
   - Calling `RunTurn` repeatedly is not an interactive-host substitute: each invocation initialises and creates/loads a session again ([turn.go:181](../../../metasystem/internal/acp/turn.go:181)).
   - Cancellation abandons the pending prompt. Its eventual response is deliberately treated as an unknown ID and kills the connection ([turn.go:407](../../../metasystem/internal/acp/turn.go:407)).
   - The current turn configuration has no model-selection field, and session setup retains only the session ID ([turn.go:91](../../../metasystem/internal/acp/turn.go:91)). A configured model is not yet a selected model.

   **Failure scenario:** the first answer works; the follow-up is rejected. Alternatively, Stop appears successful, but continuing requires recovery from a dead connection.

   **Required change:** explicitly name the many-turn host that reuses `Conn`: initialise once, retain the session, service updates/requests throughout its lifetime, settle cancellation before admitting another turn, and own process teardown/recovery. Include model selection and refusal when the configured model cannot be selected. This is exactly the new owner the [master integration table:169](../../../plans/designs/user-interface-design.md:169) already anticipates.

   Public capability evidence supports ACP as the seam, but not interchangeable enforcement:

   | Server | Session loading | Permission requests | Cancellation |
   |---|---|---|---|
   | Claude Code adapter `0.16.2` | Tagged source declares `loadSession: true` and implements replay. | Implemented; request details include title and raw input. | Implemented through SDK interruption. [Tagged source](https://raw.githubusercontent.com/agentclientprotocol/claude-agent-acp/v0.16.2/src/acp-agent.ts) |
   | Codex adapter `0.16.0` | Tagged source declares loading and implements restoration. | Implemented. | `session/cancel` is handled and forwarded to the thread. [Agent source](https://raw.githubusercontent.com/zed-industries/codex-acp/v0.16.0/src/codex_agent.rs), [thread source](https://raw.githubusercontent.com/zed-industries/codex-acp/v0.16.0/src/thread.rs) |
   | Devin | Public changelog explicitly describes `session/load` and ACP persistence/resume. | ACP permission prompts are documented. | Changelog describes cancelling active turns through ACP revert, but I found no explicit `session/cancel` contract or published initialise response. [Official changelog](https://docs.devin.ai/cli/changelog/stable) |
   | Claude Agent successor | Current upstream declares loading and implements permission requests/cancellation. | Supported. | Supported in current source. I verified the `0.81.1` release exists, but could not verify its exact source tag. [Current source](https://raw.githubusercontent.com/agentclientprotocol/claude-agent-acp/main/src/acp-agent.ts), [release](https://github.com/agentclientprotocol/claude-agent-acp/releases/tag/v0.81.1) |

   `loadSession` is negotiated; permission requests and cancellation are protocol methods, not equivalent capability booleans. Public claims do not establish the installed executable’s behavior. [ACP initialisation](https://agentclientprotocol.com/protocol/v1/initialization)

3. **High — Checkout access plus page labels cannot explain the state actually displayed.**

   [Design:24](g1-s28-a-partner-you-can-talk-to.md:24) supplies section, object, tab, and filters. It omits the displayed revision and observations.

   The board reads the accepted ledger commit, not working-tree goal files ([snapshot.go:125](../../../metasystem/internal/ui/snapshot/snapshot.go:125)). The browser retains that response until a mount, refresh, or act changes it ([state.ts:26](../../../metasystem/internal/ui/web/_app/src/backlog/state.ts:26)). Meanwhile, the current “about” context is merely a string ([about.tsx:17](../../../metasystem/internal/ui/web/_app/src/shell/about.tsx:17)).

   **Failure scenario:** the human asks why a visible goal is waiting. The Partner reads an older working-tree file or a newer accepted revision and explains a different state. Denying general command execution also removes the obvious fallback of running `git show`.

   **Required change:** capture structured context at submission: stable object identity, displayed revision/ledger tip, observation time, and a bounded snapshot of relevant displayed facts. Distinguish that snapshot from subsequent reads. This can use existing Go readers without introducing the deferred MCP endpoint.

4. **High — The single browser stream needs a Partner delivery and recovery contract.**

   The draft promises streaming but never names its browser transport. Today the sole stream listens only for `notification` events ([stream.ts:22](../../../metasystem/internal/ui/web/_app/src/notifications/stream.ts:22)); the guard permits exactly that one stream and prohibits timers ([cuts.test.ts:643](../../../metasystem/internal/ui/web/_app/src/cuts.test.ts:643)).

   Its server resumes from a steward-notification ID ([notifications.go:119](../../../metasystem/internal/ui/httpd/notifications.go:119)). An unknown ID starts at the journal’s end, replaying nothing ([notifications/notifications.go:125](../../../metasystem/internal/ui/notifications/notifications.go:125)).

   **Failure scenario:** Partner events introduce their own IDs; after disconnect, `Last-Event-ID` names a Partner event that the notification journal cannot find. Missed steward notifications disappear. Without Partner recovery, missing text chunks or the terminal event can also leave an incomplete answer permanently marked busy.

   **Required change:** name the existing stream as the shared transport, with distinct Partner event types and an explicit reconnect strategy for both consumers. Define how the transcript/current-turn snapshot joins live events without gaps or duplicates. Partner text must not become notification toasts. Server-side scheduling already exists and does not violate the frontend rule.

5. **Medium — The fresh-session rule conflates transcript retention with conversational memory.**

   [Design:24](g1-s28-a-partner-you-can-talk-to.md:24) can be read as opening a fresh session on every turn whenever loading is unsupported. Lack of `session/load` does not prevent further prompts on an existing live session. ACP explicitly supports subsequent prompts on that session. [Prompt lifecycle](https://agentclientprotocol.com/protocol/v1/prompt-turn)

   The same paragraph promises continuity but restores only ten messages. Line 18 includes loading, while line 33 defers sessions surviving server restart; those leave a real implementation choice.

   **Failure scenario:** an early message establishes “by waiting, I mean the approval blocker.” After six exchanges and a restart, that definition remains visible in the transcript but is absent from the Partner’s context. A generic “fresh session” activity line does not explain the lost memory.

   **Required change:** distinguish live-session continuation, retained transcript, and recovery context. Reuse the live session regardless of loading support. For step 1, explicitly choose fresh-session recovery after process loss and defer native loading. State precisely which history is supplied, preserve speaker/context attribution, bound its size, and visibly identify omitted history. No summarisation subsystem is required.

6. **Medium — The few-minute workflow lacks an accepted-turn and terminal-state contract.**

   “One turn at a time” and “Stop ends the turn” leave unspecified what happens to the composer, partial answer, and saved transcript after acceptance, rejection, failure, or interruption.

   Existing code has two separate drafts: one in the shell drawer and another in the focused conversation ([Shell.tsx:75](../../../metasystem/internal/ui/web/_app/src/shell/Shell.tsx:75), [Focused.tsx:40](../../../metasystem/internal/ui/web/_app/src/panes/Focused.tsx:40)). The ACP result distinguishes cancelled, refused, incomplete, and failed turns; it does not return a completed candidate for those outcomes ([turn.go:497](../../../metasystem/internal/acp/turn.go:497)).

   **Failure scenario:** the human sends a question, receives half an answer, and refreshes after the agent exits. A transcript of unspecified “messages” cannot tell the page whether the question was accepted, whether the answer is complete, or whether Retry will submit it twice. Expanding into the focused view can also strand an unsent draft.

   **Required change:** give one conversation owner responsibility for turn identity, admission, busy state, partial text, and terminal outcome. Define when sending clears the draft, how a competing send is handled, how Stop settles, and how reload/retry recovers the accepted turn. Share draft and conversation state across drawer and focused view. Missing installation/authentication should preserve the question and offer a concrete recovery action.

**Outside step 1**

- Keep proposals, human confirmation sheets, scoped mutation credentials, and the application MCP endpoint deferred. Their future details do not block this read-only conversation.
- Keep provider-native restart resume deferred; remove the competing loading branch from step 1. Conversation history still persists.
- Defer pinned subjects, selected passages, unsaved drafts, topic organisation, and multi-human isolation. The submission snapshot in finding 3 is sufficient now.
- Defer generic future-agent support beyond the three required runtimes. A command alone cannot certify a future server’s enforcement behavior.
- Supporting both Claude adapter packages is unnecessary for the first slice: select one. Also record that the old Codex repository now directs new installs to its successor; package migration alone is not another material finding. [Codex repository notice](https://github.com/zed-industries/codex-acp)

Proposed receipt, not written: `Read-only revision-2 design critique at 72cffb3a3; six material step-1 findings; code/tests/docs read; no execution.`

**Verdict: build after listed changes — 6 material findings.**

