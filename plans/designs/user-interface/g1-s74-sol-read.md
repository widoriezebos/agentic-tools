# g1-s74 the breathing dot: Sol's code read, verbatim

Code-review lane: Codex on gpt-6-sol (R-133-ui). Each pass is the report as returned by the companion task, unedited. The rulings and the fixes are in the design's Built section.

## Read 1 (2026-09-29, task-mumq922s-lcya88, build at b556dc52f)

# Code-read report: g1-s74, the breathing dot

**Three material findings remain.** The new stale-snapshot path can remove a sent question, the phone layout hides the persistent Partner dot, and leaving a Project document can leave the wrong section in the page title. This is a read-only review of `2adc52d0d..b556dc52f`.

## Findings

| ID | Severity | Material | Claim and concrete failure | Fix | R-124 answer |
| --- | --- | --- | --- | --- | --- |
| **SOL-S74-01** | High | **Yes** | When a snapshot taken before Send arrives after the 202 response, `loaded` preserves the live turn but replaces `store.messages` with the older snapshot’s messages ([conversation.ts:176](/Users/wido/LocalStorage/GitHub/agentic-tools-dot/metasystem/internal/ui/web/_app/src/partner/conversation.ts:176), [conversation.ts:199](/Users/wido/LocalStorage/GitHub/agentic-tools-dot/metasystem/internal/ui/web/_app/src/partner/conversation.ts:199), [conversation.ts:225](/Users/wido/LocalStorage/GitHub/agentic-tools-dot/metasystem/internal/ui/web/_app/src/partner/conversation.ts:225)). The just-sent question disappears; a terminal beat then appends an answer without it. The race test checks only `live`, so it misses the lost message ([live.test.tsx:175](/Users/wido/LocalStorage/GitHub/agentic-tools-dot/metasystem/internal/ui/web/_app/src/shell/live.test.tsx:175)). | Preserve the locally held messages when the snapshot is classified as preceding the turn. Assert that the question remains through the terminal beat. | **Yes.** A normal first send can leave an answer without its question. |
| **SOL-S74-02** | Medium | **Yes** | On a phone, CSS hides `.ms-drawer-title-dot` even with the drawer open ([shell.css:1374](/Users/wido/LocalStorage/GitHub/agentic-tools-dot/metasystem/internal/ui/web/_app/src/shell/shell.css:1374)). The closed-bar line is absent while open ([Drawer.tsx:173](/Users/wido/LocalStorage/GitHub/agentic-tools-dot/metasystem/internal/ui/web/_app/src/shell/Drawer.tsx:173)), and the answer-place line disappears at the first answer text ([Transcript.tsx:403](/Users/wido/LocalStorage/GitHub/agentic-tools-dot/metasystem/internal/ui/web/_app/src/partner/Transcript.tsx:403)). A running Partner can therefore have no dot visible in the open phone drawer, contrary to D3’s open-or-closed title dot. | Keep the title dot visible in the phone layout and place it so the 400 px bar remains usable. Verify the open drawer after answer text starts. | **Yes.** The on-page activity indicator disappears during an ordinary phone turn. |
| **SOL-S74-03** | Medium | **Yes** | `Shell` writes the section title in a layout effect ([Shell.tsx:223](/Users/wido/LocalStorage/GitHub/agentic-tools-dot/metasystem/internal/ui/web/_app/src/shell/Shell.tsx:223)), while `DocumentPane` writes “Project” in its passive-effect cleanup ([DocumentPane.tsx:207](/Users/wido/LocalStorage/GitHub/agentic-tools-dot/metasystem/internal/ui/web/_app/src/project/DocumentPane.tsx:207)). On navigation away from a document, React runs the layout write before that passive cleanup; the cleanup can leave “Project” as the tab title on the destination section. The installed React DOM runs layout effects before passive unmount effects ([React DOM:19839](/Users/wido/LocalStorage/GitHub/agentic-tools-dot/metasystem/internal/ui/web/_app/node_modules/react-dom/cjs/react-dom-client.development.js:19839), [React DOM:20130](/Users/wido/LocalStorage/GitHub/agentic-tools-dot/metasystem/internal/ui/web/_app/node_modules/react-dom/cjs/react-dom-client.development.js:20130)). | Remove the document title write from `DocumentPane`’s cleanup; `Shell` already writes the destination title. Keep the document-specific write while the pane is mounted. | **Yes.** The title feature gives the wrong section after a routine navigation, including while its working dot is shown. |

## Departures from the design

- **SOL-S74-01:** D4 requires a stale snapshot to fill a missing start without losing newer turn state ([design:180](/Users/wido/LocalStorage/GitHub/agentic-tools-dot/plans/designs/user-interface/g1-s74-the-breathing-dot.md:180)). The added stale-snapshot branch keeps `live` but replaces the locally sent question ([conversation.ts:197](/Users/wido/LocalStorage/GitHub/agentic-tools-dot/metasystem/internal/ui/web/_app/src/partner/conversation.ts:197)).
- **SOL-S74-02:** D3 requires the drawer title’s dot while open or closed ([design:169](/Users/wido/LocalStorage/GitHub/agentic-tools-dot/plans/designs/user-interface/g1-s74-the-breathing-dot.md:169)). The phone rule hides it in both states ([shell.css:1377](/Users/wido/LocalStorage/GitHub/agentic-tools-dot/metasystem/internal/ui/web/_app/src/shell/shell.css:1377)).
- **SOL-S74-03:** D3 requires the title to follow section changes while busy ([design:176](/Users/wido/LocalStorage/GitHub/agentic-tools-dot/plans/designs/user-interface/g1-s74-the-breathing-dot.md:176)). Changing `Shell` to a layout effect without changing the document cleanup creates the ordering above ([Shell.tsx:226](/Users/wido/LocalStorage/GitHub/agentic-tools-dot/metasystem/internal/ui/web/_app/src/shell/Shell.tsx:226), [DocumentPane.tsx:210](/Users/wido/LocalStorage/GitHub/agentic-tools-dot/metasystem/internal/ui/web/_app/src/project/DocumentPane.tsx:210)).
- **Nonmaterial typing difference:** §6 specifies `startedAt` on the 202 and snapshot ([design:234](/Users/wido/LocalStorage/GitHub/agentic-tools-dot/plans/designs/user-interface/g1-s74-the-breathing-dot.md:234)); the frontend types make both fields optional ([api.ts:463](/Users/wido/LocalStorage/GitHub/agentic-tools-dot/metasystem/internal/ui/web/_app/src/partner/api.ts:463), [api.ts:640](/Users/wido/LocalStorage/GitHub/agentic-tools-dot/metasystem/internal/ui/web/_app/src/partner/api.ts:640)). The Go responses populate them, so this does not block first use.

## What I verified holds

The three commits are the stated build, the worktree is clean, and `git diff --check` passed. By code read, the server records one admit instant for the 202 and running snapshot; completed calls emit a doing beat naming the earliest remaining call or an empty string; the tally follows the Looked count; the clock is outside the polite status span; and Fleet eligibility uses reachable standing plus `running` job status. The closed bar retains its draft, chips, proposal bar and store Stop path.

I ran the focused frontend tests: **154 tests across seven files passed**, including the live line, drawer, Fleet predicate, title, conversation, cut guard and contrast tests.

## Deferred and limitations

The sandbox refused Go’s temporary build directory, so I could not independently run Go tests, coverage floors or the bundle-current test. The builder’s report records passing Go checks and coverage of 85.1% for `partner` and 88.7% for `httpd`; those figures remain builder-reported evidence. I did not independently run the browser walkthrough or a screen reader. The title lifecycle finding is established by code and the installed React effect order, without a browser reproduction.

Proposed receipt line, **unwritten**: “Read g1-s74 at `b556dc52f`; three material first-use defects found in stale snapshot transcript preservation, phone activity visibility and document-title navigation.”

**VERDICT: 3 material findings: SOL-S74-01, SOL-S74-02, SOL-S74-03. Commit read: `b556dc52f`.**

Codex session ID: 01a0ed68-4b07-7443-8833-5356dcbcd2d5
Resume in Codex: codex resume 01a0ed68-4b07-7443-8833-5356dcbcd2d5

## Re-read of fix round 1 (2026-09-29, task-mumr6q0s-v6p02m, fix at 2e6020dfc)

# Fix-round re-read: the breathing dot

I read the single fix commit, `b96e77023..2e6020dfc`, on branch `g1-s74`. **All three earlier material findings are fixed.**

1. **SOL-S74-01 — FIXED.** When a snapshot predates the running turn, `loaded` keeps the page’s messages, including the sent question ([conversation.ts:199](/Users/wido/LocalStorage/GitHub/agentic-tools-dot/metasystem/internal/ui/web/_app/src/partner/conversation.ts:199), [conversation.ts:208](/Users/wido/LocalStorage/GitHub/agentic-tools-dot/metasystem/internal/ui/web/_app/src/partner/conversation.ts:208)). The extended race test checks the question followed by the answer after the terminal beat, confirms the turn ends, and retains the check that a snapshot holding the question ends it ([live.test.tsx:175](/Users/wido/LocalStorage/GitHub/agentic-tools-dot/metasystem/internal/ui/web/_app/src/shell/live.test.tsx:175)).

2. **SOL-S74-02 — FIXED.** The title dot renders only when the drawer is open and busy; the closed bar has its live-line dot ([Drawer.tsx:167](/Users/wido/LocalStorage/GitHub/agentic-tools-dot/metasystem/internal/ui/web/_app/src/shell/Drawer.tsx:167), [Drawer.test.tsx:169](/Users/wido/LocalStorage/GitHub/agentic-tools-dot/metasystem/internal/ui/web/_app/src/shell/Drawer.test.tsx:169)). The phone rule no longer hides it ([shell.css:1374](/Users/wido/LocalStorage/GitHub/agentic-tools-dot/metasystem/internal/ui/web/_app/src/shell/shell.css:1374)). The [400 px dark screenshot](/Users/wido/LocalStorage/agentic-tools-evidence/breathing-dot-20260929/breathing-dot/answer-place-open-400-dark.png) shows the dot at the open bar’s start; the [walkthrough notes](/Users/wido/LocalStorage/agentic-tools-evidence/breathing-dot-20260929/breathing-dot/walkthrough-notes.txt:69) record that the answer’s first words had already replaced the live line.

3. **SOL-S74-03 — FIXED.** `DocumentPane` now runs its “Project” cleanup in a layout effect, before the shell writes the destination title, while its document-name write remains passive ([DocumentPane.tsx:207](/Users/wido/LocalStorage/GitHub/agentic-tools-dot/metasystem/internal/ui/web/_app/src/project/DocumentPane.tsx:207), [Shell.tsx:223](/Users/wido/LocalStorage/GitHub/agentic-tools-dot/metasystem/internal/ui/web/_app/src/shell/Shell.tsx:223)). **The builder’s departure on fix 3 is justified.** The [browser title table](/Users/wido/LocalStorage/agentic-tools-evidence/breathing-dot-20260929/breathing-dot/walkthrough-notes.txt:47) shows that moving the whole effect to the layout phase makes “Project” overwrite the document name when a turn starts or ends there. The committed split preserves that name and gives Backlog its own title on navigation; the notes also cover the Project index and titles after the turn ends.

**New material defects:** None found in the fix-round diff.

**Deferred:** The previously noted optional `startedAt` frontend types remain outside this round ([api.ts:463](/Users/wido/LocalStorage/GitHub/agentic-tools-dot/metasystem/internal/ui/web/_app/src/partner/api.ts:463), [api.ts:640](/Users/wido/LocalStorage/GitHub/agentic-tools-dot/metasystem/internal/ui/web/_app/src/partner/api.ts:640)). The Project-index title’s missing unread count is pre-existing ([walkthrough notes:65](/Users/wido/LocalStorage/agentic-tools-evidence/breathing-dot-20260929/breathing-dot/walkthrough-notes.txt:65)).

I ran `npm test`: **127 files, 1,808 tests passed**. `git diff --check` passed. I read the builder’s browser evidence but did not replay its walkthrough. Proposed receipt, unwritten: “Re-read g1-s74 fix commit `2e6020dfc`; all three prior material findings fixed, with no new material finding.”

**VERDICT: 0 material findings: none. Commit read: `2e6020dfc` (`b96e77023..2e6020dfc`).**

Codex session ID: 01a0ed80-3e2c-74f2-9d32-7482c01e7757
Resume in Codex: codex resume 01a0ed80-3e2c-74f2-9d32-7482c01e7757
