# g1-s74 the breathing dot: Astra's critique, verbatim

Design critique lane: Codex on gpt-6-astra (R-133-ui). Each round is the report as returned by the companion task, unedited. Dispositions are at the design's foot.

## Round 1 (2026-09-29, task-mumofee4-9reel2, revision 1 at 4b00ec866)

**Round 1: five material findings.** Step 1 and its deferrals are explicit. The accepted visual premise can stand; the findings concern state accuracy and existing test guards.

Evidence was checked by reading the design and source at `4b00ec866`. Failures below are inferred from those paths, not runtime reproductions.

**S74-01 — High — material: yes — A start timestamp does not establish that a fleet job is running.**

D5 selects reachable machines whose `running.startedAt` is set. However, pending jobs can already have timestamps: [seat/publish.go:66](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/seat/publish.go:66). Fleet therefore uses job **status** to distinguish reservations from running work: [fleet/fleet.ts:219](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/fleet/fleet.ts:219). The local machine’s `working` list also comes from current job records, while its `running` field comes from published presence: [fleet/fleet.go:340](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/fleet/fleet.go:340).

**Concrete failure:** a reachable machine with a timestamped pending job gets a breathing dot while Fleet correctly says “pending.” Selecting only the newest chain can also miss another locally running job.

**Change to the design:** amend D5 to select actual running jobs from the existing `working` statuses, including all local jobs, and derive the accompanying words from that selection. State the treatment of older payloads that lack status rather than treating their timestamps as proof. Add fixtures for timestamped pending work and a local running job alongside a newer reservation.

**Test 1 — WRONG:** changes the selection predicate and its fixtures. **Test 2 — fails WORKS/SAFE:** the proposed predicate gives a false activity signal on an ordinary fleet read.

**S74-02 — Medium — material: yes — The doing line does not clear when a tool finishes.**

D1 relies on the stream’s doing line becoming empty between calls. The host emits doing on tool start, but completion emits only a look: [partner/host.go:996](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/partner/host.go:996), [partner/host.go:1040](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/partner/host.go:1040). Neither the server’s look handling nor the browser reducer clears doing: [partner/service.go:1523](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/partner/service.go:1523), [partner/conversation.ts:286](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/partner/conversation.ts:286).

**Concrete failure:** a read completes and the model thinks for another minute; the live line continues naming the completed read throughout that minute, including after reload.

**Change to the design:** include the transition that retires completed doing text, consistently in the stream and snapshot. A single completed tool followed by silence must show “Thinking”; add that fixture.

**Test 1 — WRONG:** requires a state transition absent from the proposed presentation change. **Test 2 — fails WORKS/SAFE:** “what it is doing now” silently reports finished work as current work.

**S74-03 — Medium — material: yes — The clock has two possible origins and no authoritative start in the accepted response.**

D4 says to initialize from the accepted send and reload from the server snapshot, but §6 changes only `Snapshot` and `Live`. Acceptance currently returns only a turn ID: [httpd/partner.go:268](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/httpd/partner.go:268). The browser supplies its own receipt-time timestamp: [partner/store.tsx:1100](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/partner/store.tsx:1100). Beats can precede acceptance, and the reducer deliberately preserves that earlier live state: [partner/conversation.ts:352](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/partner/conversation.ts:352). Snapshot loading likewise preserves live state when its sequence is ahead: [partner/conversation.ts:183](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/partner/conversation.ts:183).

**Concrete failure:** with five seconds between the server’s recorded start and receipt of acceptance, a receipt-based clock shows `0:01` six seconds after the real start; reload changes it to `0:06`. Preserving an early beat’s live state can also preserve an unset start unless metadata is merged deliberately.

**Change to the design:** define one server-recorded start instant, carry it through acceptance and the snapshot, and fill that metadata without discarding newer beats. Specify delayed-acceptance and beat-before-snapshot fixtures.

**Test 1 — DIFFERENT:** changes the response contract and reducer merge. **Test 2 — fails WORKS:** the promised clock continuity across the first reload is not established.

**S74-04 — Medium — material: yes — Adding a timer exception row does not permit `setInterval`.**

D4 and §6 prescribe one new exception row. The guard independently requires **zero** `setInterval` identifiers, then counts only `setTimeout` in every exception file: [cuts.test.ts:841](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/cuts.test.ts:841).

**Concrete failure:** the specified interval plus exception row fails both assertions.

**Change to the design:** explicitly amend the guard to permit and count the named interval while retaining the existing timeout allowance, network restrictions, and lifecycle-event restrictions. Keep timer cleanup on both termination and unmount.

**Test 1 — DIFFERENT:** changes the guard’s assertions. **Test 2 — fails WORKS:** an existing required test deterministically refuses the prescribed implementation.

**S74-05 — Medium — material: yes — The translucent halo needs an explicit guard classification.**

D2/§6 add `--ms-ok-halo`, while §8 calls for token and contrast checks. The token guard requires an exact registered set: [tokens.test.ts:85](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/tokens.test.ts:85). Contrast coverage permits only three unasserted tokens, and its calculator accepts only six-digit opaque hex: [contrast.test.ts:122](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/contrast.test.ts:122), [contrast.test.ts:76](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/contrast.test.ts:76).

**Concrete failure:** adding the halo fails token registration and contrast coverage; adding its translucent value as an ordinary contrast pair instead fails parsing.

**Change to the design:** register the token and classify its halo as translucent decoration in the existing contrast exemption assertion. Keep the solid dot’s contrast requirement. This needs no new colour-calculation machinery.

**Test 1 — DIFFERENT:** changes the permitted token and exemption sets. **Test 2 — fails WORKS:** the existing guards reject the specified token without this classification.

**Deferred and non-material**

- **S74-06 — Low — material: no — Consolidating placements or fleet readers.** The rail serves the accepted requirement to see other agents from another section; the title serves another browser tab. Sharing the shell and Fleet reader may reduce duplicate requests, but §5 explicitly defers it. **Test 1 — DIFFERENT; Test 2 — WORKS/SAFE.** No scope reduction or shared-reader mechanism is required.
- **S74-07 — Low — material: no — Tally conventions.** D1 should follow the existing account’s exclusion of `page` entries and failed reads; partial reads currently count among things seen: [Looked.tsx:28](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/partner/Looked.tsx:28). Preserve that convention in the tally fixture. **Test 1 — DIFFERENT if another convention is chosen; Test 2 — WORKS/SAFE under the existing convention.**
- Reconnecting/still-thinking treatments, faster presence, and halo clipping refinement remain deferred; no demonstrated first-use failure requires bringing those mechanisms into this slice.

**What I verified holds**

- D1 places the announced words and tally apart from the hidden clock and dot. It does not prescribe an announcement every second.
- D3 preserves the store-owned draft, attachment controls and proposal control, and calls the same Stop action as the composer. Their current owners support that arrangement.
- D3 explicitly reaches both title setters. The unread prefix and fallback titles can remain within `titleFor`; replacing the first transcript line with arriving answer text preserves g1-s31 D6.
- Reusing `loadFleet` adds no network call site. Fleet beats and stream-open callbacks already exist; no window lifecycle listener is necessary.
- Reduced motion and timer cleanup are explicit. Phone visibility of the title dot, crowded bar controls, and halo clipping still require the specified 400-pixel walkthrough; these were not visually verified.

**VERDICT: 5 material findings (fail test 2): S74-01, S74-02, S74-03, S74-04, S74-05.**

Commit reviewed: `4b00ec866e5ad8dec44f33c0d263d715b8eada07`, already checked out on `ui-development`; reviewed tracked paths matched that commit. Limitations: read-only review; mockup read as text; no tests, build, browser rendering or screen-reader checks run. No files changed. Proposed receipt, unwritten: “g1-s74 design critique, round 1: five material findings at 4b00ec866.”

Codex session ID: 01a0ed39-8891-72f3-aee6-f47dc1eedd30
Resume in Codex: codex resume 01a0ed39-8891-72f3-aee6-f47dc1eedd30
