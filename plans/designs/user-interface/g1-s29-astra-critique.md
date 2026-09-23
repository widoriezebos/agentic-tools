# Astra's critique of g1-s29 revision 1

Produced 2026-09-23 by Codex on `gpt-6-astra`, read-only, against revision 1 of [g1-s29-ask-about-what-you-see.md](g1-s29-ask-about-what-you-see.md) at `76d051a69`, under R-121, with Wido's instruction to critique the user experience first. Verbatim; the dispositions are the design's revision 2.

---

The step-1 gate passes if **“The page” plus “Underneath” is the first slice**, with “Later, when it hurts” deferred. On that reading, I found **nine material findings**: each changes what this slice builds.

Evidence: read the designs, prior critique, master design, relevant implementation, installed Claude adapter source, and public protocol/provider sources. Confirmed clean `ui-development` at `76d051a69`. No edits, tests, agent launches, or reads of `metasystem/metasystem.conf.local`. Scenarios below are inferred failure cases, not observed runtime failures.

1. **F1 — High: “Seeing:” can certify a different page from the one the human sees.**

   A human sees a goal waiting, inspects the preview, then sends “Why is it waiting?” after the ledger advances. The existing server composes context again at submission; the board supplies identities but no displayed tip or observation time. The Partner can receive newer facts while the browser still shows the waiting card. Adding a preview and stamp over that path makes the discrepancy look trustworthy. See [context composition](../../../metasystem/internal/ui/partner/context.go:92), [submission](../../../metasystem/internal/ui/partner/service.go:296), and [board context](../../../metasystem/internal/ui/web/_app/src/backlog/BacklogPane.tsx:187).

   **Step-1 change:** item 3 needs one captured context shared by preview, submission, and retained message. Define what happens when displayed facts cannot be recovered. Distinguish **prepared for the next question** from **already supplied**: clicking “Refresh what it sees” must not imply the live Partner received anything before another question, or retroactively update an answer’s stamp.

2. **F2 — High: the answer stamp and “Looked at” list do not account for what the Partner subsequently reads.**

   A human asks about a design. The Partner receives page context from ledger tip A, reads the document’s current revision, then reads a goal at tip B. An answer stamped simply “from tip A” misattributes its evidence. A failed document read can also appear among “Looked at 3 things,” suggesting successful inspection.

   This is concrete: the current host records tool-call titles but ignores their completion updates and results. Documents already come from the working tree, independently of the accepted ledger. See [activity handling](../../../metasystem/internal/ui/partner/host.go:643) and [document context](../../../metasystem/internal/ui/partner/context.go:494).

   **Step-1 change:** items 3 and 6 must distinguish the submitted page snapshot from additional reads. The expanded list needs identifiable subjects, revisions/observation times, success or failure, and inspectable supplied content or excerpts with omissions marked. A ledger tip alone cannot stamp a document answer. Quiet presentation is appropriate; hiding failed or partial reads behind a successful-sounding count is not.

3. **F3 — High: choosing a subject does not define how long “this” keeps referring to it.**

   A human chooses Ask on goal A, opens its design B from an answer, then asks “What would move it?” Does “it” mean A, B, or the current page? Expanding into the focused page raises the same question: the current store derives context from the mounted page, whose context disappears when it unmounts. See [conversation context selection](../../../metasystem/internal/ui/web/_app/src/partner/store.tsx:101).

   Items 1, 5, and 7 describe selecting and pinning but omit replacement, removal, and precedence.

   **Step-1 change:** define one subject lifecycle shared by drawer and focused page. State when browsing follows the page, when an explicit subject remains fixed, and how the human changes or clears it. Expanding must preserve that subject. The pinned panel must identify the revision being discussed and support the subjects this slice offers—including a lane, Overview item, or passage—not only a goal row or record head. Its displayed summary must agree with what the question carries.

4. **F4 — High: bounded context can omit the very thing the human is asking about.**

   A human scrolls to the thirtieth goal in a lane and asks “What is in here, and why?” Today only the first 25 identities per lane travel, and the context has a 12,000-byte target. “40 goals shown” therefore need not mean 40 goals supplied. The new tools promise bounded results but provide no continuation or coverage contract. See [browser limit](../../../metasystem/internal/ui/web/_app/src/backlog/BacklogPane.tsx:177) and [context limits](../../../metasystem/internal/ui/partner/context.go:58).

   **Step-1 change:** prioritise the explicitly chosen object or passage; expose supplied versus omitted scope to both parties; and provide a bounded way to retrieve omitted material. Specify limits and overflow behavior for passages and tool results, not merely “bounded.”

   Suggested questions must respect that scope. “What needs me here?” needs a defined filtered set; “What changed today?” needs a defined time window and sufficient evidence, rather than silently substituting a limited recent list.

5. **F5 — High: the read-tool hand-off is feasible, but the unchanged permission contract can refuse its calls.**

   A human chooses “What does its design say?” The Partner discovers an application tool, but its request is classified as `other` without a checkout path. The current [permission handler](../../../metasystem/internal/ui/partner/permission.go:64) refuses it. Installed Claude’s generic MCP tool presentation has precisely that shape. This prevents the promised interaction without any malicious behavior.

   The compatibility evidence supports a narrow implementation:

   | Runtime | Evidence read |
   |---|---|
   | Claude | Installed `0.81.1` merges servers from `session/new` into its SDK configuration; the empty `_meta` MCP map does **not** erase that hand-off. [Source](/opt/homebrew/lib/node_modules/@agentclientprotocol/claude-agent-acp/dist/acp-agent.js:6365) |
   | Codex | Current adapter source handles stdio and HTTP hand-offs and explicitly rejects SSE. [Source](https://github.com/agentclientprotocol/codex-acp/blob/main/src/CodexAcpClient.ts) |
   | Devin | Release `3000.11.1` explicitly fixes MCP servers supplied through `session/new`/`session/load`. Installed-version behavior remains unverified. [Release notes](https://docs.devin.ai/cli/changelog/stable) |

   **Step-1 change:** name the application server as the specific exception to the earlier no-MCP rule, choose its transport, and define admission of its known read operations. ACP requires stdio support; HTTP requires capability negotiation, which the [current host](../../../metasystem/internal/ui/partner/host.go:252) does not retain. [ACP specification](https://agentclientprotocol.com/protocol/v1/session-setup#mcp-servers)

   Keep the existing write fence and human-credential separation. The [sandbox](../../../metasystem/internal/ui/partner/confine.go:47) denies checkout writes; it does not establish the new server’s permitted operations. This needs a small explicit integration contract, not a replacement ACP host.

6. **F6 — Medium: “the same affordance” lacks a consistent discovery and input contract.**

   A first-time human tabs through the board and opens the drawer. Nothing teaches them that object-specific Ask lives behind right-click. On the list, rows do not currently have the board’s menu access. On an Overview item, the whole row may already navigate or open a panel. Selecting prose on a draggable card introduces another competing gesture.

   The board already supports Shift+F10/Menu, restores menu focus, and uses the card as a drag target. Retired split cards bypass that menu entirely. See [card interaction](../../../metasystem/internal/ui/web/_app/src/backlog/Board.tsx:554), [menu focus](../../../metasystem/internal/ui/web/_app/src/backlog/CardMenu.tsx:42), and [Overview actions](../../../metasystem/internal/ui/web/_app/src/overview/OverviewPane.tsx:621).

   **Step-1 change:** give items 1, 2, and 8 a compact interaction table covering each surface: discoverable entry, keyboard equivalent, and selection/drag precedence. Reuse the existing card menu and teach its shortcut in the first-minute experience. Define focus transfer into the composer and back to the source. Escape must dismiss the foremost menu, selection control, or sheet before closing the drawer; Cmd/Ctrl+J must respect modal and busy states.

7. **F7 — Medium: suggestion chips can discard an existing draft, and selection “Ask” has ambiguous submission behavior.**

   A human has composed half a careful question, chooses Ask on another object, then clicks a suggestion. The existing `send(text)` path clears the shared draft after acceptance—even when the submitted text was supplied separately. That newly exposed interaction can erase the unfinished question. See [send behavior](../../../metasystem/internal/ui/web/_app/src/partner/store.tsx:107).

   For selected text, “sends the passage as the subject” leaves another choice: does Ask merely attach it and focus the composer, or immediately submit the passage without a question?

   **Step-1 change:** specify those transitions. Preserve existing draft text when a suggestion sends, or insert suggestions when a draft exists. Make the selected quotation visible and removable before submission, with its source revision. Define busy/refusal behavior for both paths. The saved-text restriction should exclude unsaved editor content explicitly; it must never substitute saved words for a visibly selected draft.

8. **F8 — Medium: automatic links and hover rings can point ambiguously or appear to do nothing.**

   An answer names six goals, including one hidden by filters and another outside the current page. Hovering those links cannot ring a visible card. Meanwhile, a record title may identify more than one record, and an incidental mention is not necessarily a recommendation to inspect it.

   **Step-1 change:** item 4 needs deterministic identity resolution: stable IDs, and title linking only when unambiguous. Hover or keyboard focus should mark the single corresponding visible card for the duration of that interaction. Hidden/off-page targets need a clear click destination; hover should not navigate, clear filters, or ring every mentioned goal.

   The existing landing behavior already reveals hidden goals and explains changed filters. Reuse it for deliberate navigation. [Landing rules](../../../metasystem/internal/ui/web/_app/src/backlog/showing.ts:159) Define “clicking anywhere navigates” as clicking the reference—not the surrounding answer.

9. **F9 — Medium: a message chip cannot yet return the human to the meaningful page state.**

   Weeks later, a human clicks the chip on “Why are these waiting?” Originally they saw a filtered board and one lane; their browser now prefers the list. Opening `/backlog` reconstructs neither the set nor the viewpoint that made “these” meaningful. A passage may also have moved or disappeared.

   The current message capture uses `location.pathname`; view state is partly held separately, and document headings travel as text rather than a return anchor. See [message capture](../../../metasystem/internal/ui/web/_app/src/partner/store.tsx:104) and [document context](../../../metasystem/internal/ui/web/_app/src/project/DocumentPane.tsx:310).

   **Step-1 change:** item 5 must specify the saved return state: relevant view, filters, window, tab, and subject/passage anchor. Distinguish reopening the current object from inspecting the captured context. Missing or changed subjects need an explicit outcome. Full historical page reconstruction is unnecessary; silently presenting today’s page as the old context is unacceptable.

**Outside step 1**

- Screenshots and questions about visual appearance remain deferred; structured object context does not require screen capture.
- Proposals, mutation tools, brain occupancy, and broader human-authority machinery remain under the previously settled boundaries.
- Multiple pinned subjects, subject threads, multiple humans, voice, sharing unsaved edits, and provider-native restart loading remain deferred.
- Guided tours, automatic cross-page highlighting, and complete historical UI reconstruction are unnecessary for these findings.
- “Three questions” versus the one or two examples supplied for some kinds is editorial unless it changes the actual interaction. Exact chip styling and ring animation timing should not keep critique open.

Proposed receipt, not written: `Read-only UX design critique of g1-s29 revision 1 at 76d051a69; nine material step-1 findings; repository and adapter evidence read; no edits, tests, or agents.`

**Verdict: build after listed changes — 9 material findings.**


