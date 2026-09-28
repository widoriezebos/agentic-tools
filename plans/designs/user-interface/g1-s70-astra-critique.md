# Astra's critique of g1-s70

Produced 2026-09-28 by Codex on `gpt-6-astra`, read-only, against [g1-s70-the-landing-gate.md](g1-s70-the-landing-gate.md) at `81acda402`, under R-121 and R-124, brief `~/LocalStorage/agentic-tools-evidence/review-room-20260928/astra-design-brief-landing-gate.md`. Verbatim; the dispositions are at the design's foot.

## Round 1

---

**S70-01 — High; material: yes. The standing-sitting hold cannot reach the landing seat.**

**Evidence:** [S70:80](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s70-the-landing-gate.md:80) makes the review record the hold’s source. However, [partner/review.go:155](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/partner/review.go:155) reads standing sittings from the human’s private conversations; [conversation.go:1299](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/partner/conversation.go:1299) ends one by clearing that private mark. `httpd/review.go:197`’s `standingOf` describes the goal’s **lane**, not whether a sitting stands. S69:21 explicitly establishes that the reviewing checkout’s record is unavailable to the claimant.

**Concrete failure:** A tier-1 goal reaches its deadline while the human reviews it in another checkout. The landing seat sees no standing record and lands it. This violates R-132-ui on the first ordinary cross-checkout review.

**Change to the design:** Give the hold shared, authoritative start/end facts through the existing goal ledger. Establish the hold before reporting the sitting open; Step out preserves it, and ending releases it. The gate, clock and displayed hold must consume that same fact. Committing the record only when the review ends cannot protect the intervening sitting.

**Test 1:** **WRONG** — the specified source cannot enforce the hold.  
**Test 2:** **SAFE: no** — work lands while the human’s sitting forbids it.

**S70-02 — High; material: yes. D6 leaves alternative landing forms outside a clearly defined gate.**

**Evidence:** S70:76 requires the gate, but [S70:104](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s70-the-landing-gate.md:104) says other `work land` forms do not change. Those forms have separate execution paths: [intent_delivery.go:1448](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/cmd/metasystem/intent_delivery.go:1448) dispatches `--message`, `j2:J`, and exceptional landings before ordinary `landGoal`. [intent_delivery.go:1523](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/cmd/metasystem/intent_delivery.go:1523) resolves a job’s goal and joins its landing batch. The cited [landing/advance.go:31](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/landing/advance.go:31) is checkout rebasing, with no goal or verdict input.

**Concrete failure:** An otherwise certified tier-2 goal in Review has neither permission line. Preserving the existing `work land j2:J` behavior lets it join the batch and land despite a gate added only to the ordinary goal path. Staged and exceptional forms require the same explicit coverage decision.

**Change to the design:** State that every form publishing the governed goal’s work enforces the same gate against its resolved candidate. Name the enforcement boundary, including asynchronous batch publication. Preserve command syntax, not a policy bypass. An existing exception must not silently become a current-tip decision to skip the sitting. Keep `--queue-only` able to enter Review.

**Test 1:** **DIFFERENT/WRONG** — D2 and D6 currently permit incompatible implementations.  
**Test 2:** **SAFE: no** — the unchanged alternative paths can publish without either required human act.

**S70-03 — High; material: yes. The specified configuration reader ignores the promised local override.**

**Evidence:** [S70:72](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s70-the-landing-gate.md:72) specifies `ConfValue`. [config/conf.go:39](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/config/conf.go:39) reads exactly the supplied file and returns a default when absent or unreadable. It does not layer `.local`. That behavior belongs to [config/resolve.go:172](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/config/resolve.go:172), whose local lookup starts at line 193.

**Concrete failure:** The committed/default threshold is 2; the human sets the local threshold to 1. Following D1 literally, the engine still treats tier 1 as eligible for unattended landing. The setting that requires human permission is silently ignored.

**Change to the design:** Require the existing layered resolution for both keys. The gate, clock and settings display must use the same effective values and accurately report their source. Include a synthetic local threshold of 1 over committed 2 in the verification.

**Test 1:** **WRONG** — the named reader does not implement the specified precedence.  
**Test 2:** **SAFE: no** — the engine can land work the effective setting reserves for a human.

**S70-04 — Medium; material: yes. The silence clock has no defined resumption rule after a sitting ends without a verdict.**

**Evidence:** [S70:86](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s70-the-landing-gate.md:86) requires no human act since the original `Landing.At`. [S65:391](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s65-the-review-sitting.md:391) explicitly says “End without a verdict releases the clock.” S69:86 performs no goal act for that outcome. The existing [LandingRecord](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/goal/file.go:363) contains only `At` and `Opid`; ending the sitting clears its private mark.

**Concrete failure:** A tier-1 goal enters Review at 14:02; the human starts a sitting at 15:00 and ends without a verdict at 16:00. A literal “no human act since 14:02” predicate never becomes true again. Ignoring that predicate instead leaves the implementer choosing an original deadline, remaining grace, or a fresh grace period. The card cannot state one truthful deadline until this is settled.

**Change to the design:** Define which human events interrupt the grace period and exactly how the supported no-verdict exit resumes it. Carry any required shared timestamp with S70-01’s lifecycle facts, and derive the card’s deadline from that rule. Add a fixture covering this first sitting and its end.

**Test 1:** **DIFFERENT/WRONG** — the choices produce different landing times, including permanent ineligibility.  
**Test 2:** **WORKS: no** — the literal rule defeats the promised release. **SAFE: yes** for the permanently stalled interpretation; that does not make it work.

**Deferred and non-material**

- Offline failover, another scheduler, finer cadence and additional concurrency machinery: deferred. **Material: no. Test 1: DIFFERENT, yes. Test 2: WORKS/SAFE, yes** for the declared running-holder case using existing ownership.
- Route spelling and sheet reuse are implementation details, not additional blockers: existing goal acts use `/api/backlog/goals/`, and the current `DecideSheet` is in `partner/Deposit.tsx`. The new route can be wired consistently. **Material: no. Test 1: DIFFERENT, potentially. Test 2: WORKS/SAFE, yes.**

**What I verified holds**

- Step 1 and its deferred list are explicit, satisfying R-121’s entry condition.
- I read R-132-ui verbatim. Its boundary is **below** versus **at or above** the configured threshold. Default 2 is compatible; the ruling does not mandate a numerical default.
- A resident steward loop exists at [runner.go:214](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/steward/runner.go:214), and [goal/project.go:744](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/goal/project.go:744) separates this machine’s landing claims. No independent timer is necessary. Expiry means eligibility on a running pass, not guaranteed publication at that instant.
- D4 already requires human authority, a reason and tip binding. Existing [act.SignedIn](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/act/act.go:190) validates the human and session against the proof; goal authority admission is per action. Rejection of nonhuman invocation and stale-tip permission follows the stated contract.
- The refusal register and Decisions `Need` provide suitable extension points. A batch join is explicitly `in-progress`, so the history must say “landed” only after confirmed publication.

**VERDICT: 4 material findings (fail test 2): S70-01, S70-02, S70-03, S70-04**

Commit reviewed: `81acda402b39acf547bdac7a9af2f9a6e4c503b6`.

Limitations: static, read-only review; failures above are inferred from the design and traced code, not executed reproductions. S69 was reviewed only as D’s dependency. No subagents, edits, `.local` contents or `artifacts/` contents were used. Proposed receipt, unwritten: “S70 design critique round 1 — four material findings.”

