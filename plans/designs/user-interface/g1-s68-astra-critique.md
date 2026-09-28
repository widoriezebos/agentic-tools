# Astra's critique of g1-s68

Produced 2026-09-28 by Codex on `gpt-6-astra`, read-only, against [g1-s68-ask-what-happened.md](g1-s68-ask-what-happened.md) at `3e1262633`, under R-121 and R-124, brief `~/LocalStorage/agentic-tools-evidence/review-room-20260928/astra-design-brief-ask-what-happened.md`. Verbatim; the dispositions are at the design's foot.

## Round 1

---

Four material findings remain. Step 1 and its deferrals are explicit. Evidence below was checked by reading commit `3e1262633`; failure scenarios are inferred, not runtime demonstrations.

**S68-01 — High — material: yes. Trouble capture needs an explicit secret exclusion.**

**Claim:** D1’s instruction to carry “the arguments the page sent” includes credentials at an expressly included refusal site.

**Evidence:** The [design:113](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s68-ask-what-happened.md:113) carries arguments, and §6 persists and forwards them. [SignInSheet.tsx:20](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/shell/SignInSheet.tsx:20) explicitly forbids handing over the secret; its request at line 62 includes the code, and its refusal renders at line 127. [session.ts:105](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/shell/session.ts:105) confirms the submitted body contains `code`.

**Concrete failure:** A sign-in attempt fails. Applying D1 literally, Ask carries its request arguments into the transcript and model prompt. The 1,000-character argument bound does not protect a six-digit secret. Raw error text can likewise contain an echoed credential.

**Change to the design:** Specify safe diagnostic fields rather than wholesale request arguments. Explicitly exclude authentication codes and credentials before persistence or prompt composition, including known secret values echoed in a sentence. Add a fixture proving the sign-in code reaches neither destination.

**Test 1 — DIFFERENT/WRONG:** DIFFERENT capture contract; literal implementation is WRONG for authentication.
**Test 2 — WORKS/SAFE:** **Not SAFE** without the exclusion: asking discloses a secret the existing interface deliberately withholds.

**S68-02 — High — material: yes. The automatic question has no draft-preservation contract.**

**Claim:** D2 specifies immediate submission and a busy composer fallback, but does not preserve words already in the composer.

**Evidence:** [Design:119](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s68-ask-what-happened.md:119) specifies these two paths. The existing [store.tsx:958](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/partner/store.tsx:958) accepts supplied text instead of the draft, then unconditionally clears the draft at line 979 and retires attachments at line 984. Existing suggested questions explicitly protect half-written words at [store.tsx:1282](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/partner/store.tsx:1282).

**Concrete failure:** The human has an unfinished question, then presses Ask beside a refusal. Reusing `send(TroubleRequest)` submits the explanation request and silently deletes that unfinished question. Filling the composer directly on the busy path can overwrite it instead.

**Change to the design:** Declare that a trouble request preserves unrelated draft text and attachments on acceptance, refusal and busy fallback. Define how the pending trouble coexists with a nonempty composer; retain the existing draft-preservation behavior. Test the first Ask with an occupied composer, both idle and busy.

**Test 1 — DIFFERENT/WRONG:** DIFFERENT send/clear ownership; straightforward reuse is WRONG.
**Test 2 — WORKS/SAFE:** **Not SAFE** without it: the first press can lose the human’s words.

**S68-03 — Medium — material: yes. Several required surfaces cannot reach the existing Partner owner.**

**Claim:** “Every site converted (mechanical)” omits a necessary connection for the bell, toasts and sign-in sheet.

**Evidence:** [Shell.tsx:73](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/shell/Shell.tsx:73) places `PartnerProvider` inside `Shell`. [App.tsx:30](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/App.tsx:30) places the identity and notification providers above it. Those providers render their own surfaces as siblings of their children: [notifications/store.tsx:161](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/notifications/store.tsx:161) and [identity.tsx:137](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/shell/identity.tsx:137). Outside `PartnerProvider`, [partner/store.tsx:581](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/partner/store.tsx:581) supplies empty state and no-op actions.

**Concrete failure:** Replacing a bell row with a `Trouble` that uses the existing Partner context displays a working-looking Ask button that sends nothing. It also lacks the room’s conversation selection. The structural class guard would still pass.

**Change to the design:** Name how these surfaces reach the live Partner owner—by placing their rendering beneath it or passing an explicit connection. Preserve the sign-in sheet and its retry behavior. Test Ask from an actual notification-provider row while a room is active, rather than only rendering `Trouble` with a mocked Partner.

**Test 1 — DIFFERENT/WRONG:** DIFFERENT component placement or interface; mechanical replacement alone is WRONG.
**Test 2 — WORKS/SAFE:** **Does not WORK** at the first bell-row press.

**S68-04 — Medium — material: yes. The promised recovery card exceeds the proposal mechanism.**

**Claim:** D3 treats an available interface operation as an available Apply card. These are different capabilities.

**Evidence:** The [design:83](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s68-ask-what-happened.md:83) promises an “Open the review room” card; D3 generalizes that promise at line 136. [uitools/propose.go:44](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/uitools/propose.go:44) lists the ten supported goal actions, none of which opens a room. Unknown actions are refused at [propose.go:497](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/uitools/propose.go:497). The shipped [Partner skill:33](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/partner/project-partner.skill.md:33) likewise limits proposals to goal actions.

**Concrete failure:** The example recovery cannot produce its promised Apply card through the existing tool. A fake Partner can illustrate the card while the real Partner’s proposal is refused.

**Change to the design:** Limit Apply cards to the existing proposal catalogue. Give navigation, retry and terminal recoveries their existing links or instructions. Replace the example with a supported recovery; do not expand proposal authority merely to satisfy the illustration. Include a negative fixture for unsupported recovery cards.

**Test 1 — DIFFERENT/WRONG:** DIFFERENT recovery mapping; the current promise produces a WRONG capability claim.
**Test 2 — WORKS/SAFE:** **Does not WORK** for the design’s own example. Existing rejection protects authority but does not deliver the promised recovery.

**Deferred and non-material**

- **S68-05 — Low — material: no.** “Every code has a row” is false: `Exclusions` and hand-recorded `ProseRows` exist ([register.go:391](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/refusal/register.go:391), [register.go:470](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/refusal/register.go:470)); HTTP codes include `one-line` and `partner`. D4 already specifies “the register has no row for CODE.” **Test 1:** no different step-1 mechanism needed. **Test 2:** WORKS and SAFE with that honest fallback; wider coverage waits.
- **S68-06 — Low — material: no.** Correct the implementation pointers: Composer lives under `shell/`; the turn API uses `/turns` and `about`; ten proposal actions are distinct from the reader catalogue ([api.ts:613](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/partner/api.ts:613), [uitools.go:92](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/uitools/uitools.go:92)). **Test 1:** bookkeeping differs, with “routes unchanged” resolving intent. **Test 2:** WORKS and SAFE without redesign.
- Broader structural-guard coverage and real-model answer-quality evaluation can follow use. **Test 1:** stronger verification would differ. **Test 2:** neither is independently shown necessary for first-use safety; the fake walkthrough must not be presented as proof of real-model quality.

**What I verified holds**

The fixed explanatory question is compatible with an explicit Ask press; it does not authorize applying a recovery. Existing turns support server-owned interface attribution ([service.go:653](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/partner/service.go:653)). Conversation selection already distinguishes rooms from the drawer ([store.tsx:846](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/partner/store.tsx:846)), and submission takes that conversation explicitly.

D4 can match `Row.Code` directly; existing tests do so ([register_test.go:300](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/refusal/register_test.go:300)). D3 explicitly requires reading causes and admitting unknowns. D6 is coherent as “no project record, sticky or receipt”: §6 expressly retains the conversation.

Proposed receipt, unwritten: “S68 design critique round 1: four material findings concerning secret capture, draft preservation, Partner access from global surfaces, and unsupported recovery cards.”

**VERDICT: 4 material findings (fail test 2): S68-01, S68-02, S68-03, S68-04**

Commit reviewed: `3e1262633880efe47927f499772d97895f5f2814`. Limitations: static review only; no tests, browser walkthrough or real Partner run. No edits or subagents; prohibited paths were not read.


## Round 2, the declared failsafe

---

All four round-1 findings are answered in revision 2. Two material gaps remain in D2. Evidence was read at `0324a0cc4`; the failure scenarios below are inferred from the design and existing code, not runtime demonstrations.

**Round-1 dispositions**

| Finding | Disposition | Evidence and tests after the fold |
|---|---|---|
| S68-01 | **CONFIRMED ANSWERED** | [Design:119](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s68-ask-what-happened.md:119) excludes request arguments, requires secret scrubbing before retention/transmission, and names a sign-in fixture. This answers the credential-bearing request at [SignInSheet.tsx:62](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/shell/SignInSheet.tsx:62). **Test 1:** DIFFERENT capture contract. **Test 2:** SAFE against the original disclosure when implemented as specified. |
| S68-02 | **CONFIRMED ANSWERED** | [Design:139](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s68-ask-what-happened.md:139) requires a separate send path preserving draft and attachments across acceptance, refusal and waiting. It explicitly avoids [store.tsx:979](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/partner/store.tsx:979), which clears the draft. **Test 1:** DIFFERENT send ownership. **Test 2:** SAFE against the original loss of words. |
| S68-03 | **CONFIRMED ANSWERED** | [Design:150](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s68-ask-what-happened.md:150) supplies the missing connection through a registered callback above the providers, bypassing the no-op context at [store.tsx:581](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/partner/store.tsx:581). **Test 1:** DIFFERENT connection. **Test 2:** WORKS for the original no-op defect. Reaching a usable conversation surface remains a separate issue below. |
| S68-04 | **CONFIRMED ANSWERED** | [Design:167](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s68-ask-what-happened.md:167) limits cards to the proposal grammar; the room example becomes a link. This matches [propose.go:497](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/uitools/propose.go:497), which rejects unsupported actions. **Test 1:** DIFFERENT recovery mapping. **Test 2:** WORKS and remains SAFE within existing proposal authority. |

**S68-07 — High — material: yes. A pending trouble needs a conversation destination fixed at Ask.**

**Claim:** The new pending chip preserves words but does not specify which conversation owns the waiting request. “The next Send sends it first” combines unsafely with selecting the conversation at submission time.

**Evidence:** [Design:131](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s68-ask-what-happened.md:131) makes Ask the question in the conversation the human is in; line 147 defers it until the next Send, and line 158 relies on the current conversation selection. [store.tsx:850](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/partner/store.tsx:850) derives that selection from the current address. Its navigation effect at line 884 replaces the displayed conversation, while [store.tsx:974](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/partner/store.tsx:974) sends to the presently selected `where`.

**Concrete failure:** In room A, while a turn runs, the human presses Ask. The trouble waits. They navigate to room B and press Send. Following the specified pending-first rule and existing conversation selection sends A’s already-requested explanation into B. Preserving the draft does not prevent writing into the wrong conversation.

**Change to the design:** Store the originating conversation with the pending trouble at Ask. Offer and send that pending question only in its owning conversation; navigation must not silently retarget it. Add a named fixture, `pending_trouble_stays_in_origin_room`: Ask while A is busy, switch to B, verify B cannot send A’s pending trouble, return to A and send it with the original draft and attachments intact.

**Test 1 — DIFFERENT/WRONG:** DIFFERENT pending-state contract and destination selection; blindly following the current Send destination is WRONG.

**Test 2 — WORKS/SAFE:** **Not SAFE** without destination ownership: an ordinary first busy request can be written into another conversation.

**S68-08 — Medium — material: yes. Registration proves a live store, not a reachable conversation.**

**Claim:** The new callback solves the provider connection, but its visibility gate admits Ask where the answer or pending Send remains inaccessible.

**Evidence:** [Design:150](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s68-ask-what-happened.md:150) gates the control on callback registration. The notification panel remains outside the shell’s work-area provider ([notifications/store.tsx:161](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/notifications/store.tsx:161)); the default work-area context has no layer ([workmodal.tsx:48](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/shell/workmodal.tsx:48)), so [Sheet.tsx:68](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/shell/Sheet.tsx:68) makes it window-modal. Sign-in explicitly uses that modality at [SignInSheet.tsx:78](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/shell/SignInSheet.tsx:78).

There is also a permanent instance: [Shell.tsx:416](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/shell/Shell.tsx:416) places the whole room inside an error boundary while the Partner provider survives above it. [ErrorBoundary.tsx:26](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/shell/ErrorBoundary.tsx:26) continues rendering its fallback until reload.

**Concrete failure:** Ask from the bell successfully submits behind a modal that still blocks the conversation. More decisively, Ask from a caught room-render failure reaches the surviving store, but the room’s conversation renderer has been replaced by the fallback. The response exists without a usable surface for reading it or continuing.

**Change to the design:** Require the registered Ask path to reveal a usable conversation, not merely submit. Specify the bell’s dismissal and a sign-in handoff that preserves its form and pending retry. Where the sole conversation renderer has failed, use the already-declared honest unavailable fallback unless an existing working surface can be reached. Add `trouble_ask_reaches_usable_partner`, checking actual visibility and keyboard access through the real providers and the room boundary.

**Test 1 — DIFFERENT/WRONG:** DIFFERENT reveal and availability behavior; registration alone gives the WRONG availability signal.

**Test 2 — WORKS/SAFE:** **Does not WORK** for the included caught-room-throw case: the first press cannot deliver a usable conversation.

**Deferred and non-material**

- **S68-09 — Low — material: no.** The class-name guard at [design:218](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s68-ask-what-happened.md:218) cannot prove that every future error uses `Trouble`; notification messages already use other names at [Panel.tsx:91](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/notifications/Panel.tsx:91). **Test 1:** stronger guard coverage would be DIFFERENT. **Test 2:** WORKS and SAFE after the explicitly required current-site conversions; broader enforcement waits.
- Real-model answer-quality evaluation remains deferred. **Test 1:** verification would differ. **Test 2:** no independent first-use failure is established merely because the walkthrough uses a fake Partner; that walkthrough cannot certify real-model grounding.

**What I verified holds**

Step 1 and its deferrals are explicit. A deliberate Ask press authorizes the fixed explanatory question, without authorizing recovery. The existing server-owned interface attribution supports that distinction ([service.go:653](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/partner/service.go:653)).

The refusal reader can match `Row.Code` directly, as [register_test.go:300](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/refusal/register_test.go:300) does. Revision 2 explicitly gives exclusions, uncoded refusals and HTTP-only codes the honest “no row” response. The new reader belongs in the reader catalogue, without expanding proposal actions.

Both Partner skill copies require owner-backed claims and admission of unread evidence. D3 adds the appropriate cause and recovery constraints. D6 remains coherent as no project record, sticky or receipt: the conversation itself is expressly retained.

Proposed receipt, unwritten: “S68 design critique round 2: four prior findings answered; two material findings on pending conversation ownership and access to the answer.”

**VERDICT: 2 material findings (fail test 2): S68-07, S68-08**

Commit reviewed: `0324a0cc4b0a3ce71265aee42d67e829995c3b41`. Limitations: static design/code review only; no tests, browser walkthrough or real Partner run. No edits or subagents. Prohibited paths were not read.

