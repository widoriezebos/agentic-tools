# Astra's critique of g1-s72

Produced 2026-09-29 by Codex on `gpt-6-astra`, read-only, against [g1-s72-signed-in-is-enough.md](g1-s72-signed-in-is-enough.md) at `7269ad21c`, under R-121 and R-124, brief `~/LocalStorage/agentic-tools-evidence/signed-in-launch-20260929/astra-design-brief.md`. Verbatim; the dispositions are at the design's foot.

## Round 1

---

Four material findings. The main issue is D2: an owner-only record plus an `UNTRUSTED` caller does not establish that a signed-in human authorized enrollment.

All evidence below was read at `7269ad21c`. The failures are inferred from the proposed design and existing code, not reproduced by execution.

**S72-01 — High — material: yes — D2 permits permanent enrollment without a signed-in human**

**Claim and evidence:** [D2](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s72-signed-in-is-enough.md:115) accepts caller-owned JSON containing asserted session facts and admits `UNTRUSTED`. The classifier deliberately returns that class for an unrecognized headless process ([classify.go:448–476](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/lease/classify.go:448)). Its steward recognition is repository-local: it authenticates only the target repository’s installed binary ([classify.go:513](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/lease/classify.go:513)); an existing test explicitly excludes another repository’s steward ([classify_test.go:445](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/lease/classify_test.go:445)).

**Concrete failure:** An unattended same-user shell creates a 0600 record with the intended destination and invented provider, human and session, then invokes the proposed arm command. It passes the stated checks without signing in or possessing a server-issued verdict. A launching checkout’s steward also need not classify as `STEWARD` against a fresh destination. Today the ungated word path leaves a durable temporary label; this path would mint permanent, human-witnessed provenance.

**Change to the design:** Require a verifiable handoff from the server’s accepted session before a headless caller can mint `human-session`. File ownership and destination binding cannot supply that fact. Replace the blanket “UNTRUSTED admitted” obligation with cases distinguishing a genuine detached launch from fabricated JSON and another checkout’s steward. Exercise the real classifier, alongside injected-class tests.

**Test 1 — DIFFERENT/WRONG:** WRONG: the proposed gate can mint a false human witness; the handoff contract must change.  
**Test 2 — WORKS/SAFE:** **SAFE fails.** This is the brief’s explicit same-user mint boundary.

**S72-02 — High — material: yes — A browser retry can invoke the new flag on an old clone’s engine**

**Claim and evidence:** D1 stamps legacy records on retry; D4 consequently selects `--launch-record`. However, resume retains the clone’s existing HEAD ([sequence.go:265](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/seat/launch/sequence.go:265)). Tracking only fetches, and the engine step accepts a binary stamped with that retained commit or builds from that clone’s source ([sequence.go:428–458](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/seat/launch/sequence.go:428)). The old arm parser has no `--launch-record` flag ([steward_verbs.go:328](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/cmd/metasystem/steward_verbs.go:328)).

**Concrete failure:** A pre-slice launch cloned and built successfully, then stopped before enrollment. After updating the launching checkout, pressing Retry stamps a session verdict, skips the already-current old engine, and invokes that engine with an unknown flag. Rebuilding unchanged old source has the same result.

**Change to the design:** Specify how a legacy destination obtains a compatible enrollment implementation before step 7, preserving its existing work. JSON decoding compatibility alone is insufficient. Add an obligation with a legacy record, an old clone HEAD and an old engine, verifying that one browser retry completes with the intended enrollment kind.

**Test 1 — DIFFERENT/WRONG:** WRONG: retry requires an additional compatibility decision in the engine/enrollment flow.  
**Test 2 — WORKS/SAFE:** **WORKS fails** on the first retry of this existing launch.

**S72-03 — Medium — material: yes — D4 removes the existing temporary-word CLI path**

**Claim and evidence:** [D4](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s72-signed-in-is-enough.md:153) removes both `seat launch` flags and the request fields that carry them. They are currently accepted and validated by [seat_launch.go:45–60](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/cmd/metasystem/seat_launch.go:45), then forwarded to arm by [sequence.go:670–672](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/seat/launch/sequence.go:670).

**Concrete failure:** An existing caller invoking `seat launch` or `seat launch --resume` with the temporary-word pair receives an unknown-flag error. Keeping the pair on `steward arm`, as D6 promises, does not preserve its enclosing launch caller.

**Change to the design:** Retain the optional CLI pair and its forwarding branch for callers without a session enrollment. Remove the browser’s fields and requirements as planned; reject combining the pair with session enrollment. This preserves existing behavior without adding a new mechanism.

**Test 1 — DIFFERENT/WRONG:** DIFFERENT: D4 must retain the CLI compatibility branch.  
**Test 2 — WORKS/SAFE:** **WORKS fails** for an existing temporary-word caller, explicitly protected by the brief.

**S72-04 — Medium — material: yes — Old step text still displays the review date**

**Claim and evidence:** Successful enrollment currently persists `"temporary enrollment, review due " + ReviewBy` in `Step.Words` ([sequence.go:683](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/seat/launch/sequence.go:683)). The full card renders every step’s words verbatim ([LaunchCard.tsx:123–151](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/fleet/LaunchCard.tsx:123)). D5 removes the separate review line and folded clause but leaves this rendering path intact.

**Concrete failure:** Open the updated fleet page with an old launch that enrolled successfully and then failed during supervision. Its full card still says “temporary enrollment, review due …”, even though `Record.ReviewBy` no longer decodes into the new structure. No retry is needed to expose it.

**Change to the design:** Suppress the legacy successful enrollment narration in the fleet presentation while retaining the historical record. Do not replace it with a claim that the old machine has session enrollment. Add a card fixture containing the old `Step.Words` and a later failed step.

**Test 1 — DIFFERENT/WRONG:** DIFFERENT: the removal must cover persisted step narration.  
**Test 2 — WORKS/SAFE:** **WORKS fails** the explicit requirement that the page show no temporary enrollment or review date.

**Deferred and non-material:** Verdict age limits, revocation, fleet-wide re-enrollment and fleet-row provenance can remain deferred; they pass Test 2 for this slice. I found no reason to add machinery for them.

**What I verified holds:**

- Step 1 and its deferrals are explicit; the signed-in-session premise remains accepted.
- The route already requires a live session and root-bound proof. Session references are distinct from bearer credentials; `RecordSessionProof` produces audit evidence.
- D3 identifies the necessary accepted-kind, mint, rebuild-carry and provenance changes. The other inspected identity readers introduce no additional kind restriction.
- The normal detached `ui serve` → `seat launch` chain is not itself a runtime signature or steward-family invocation.
- D5 covers the current form, retry fields, dedicated review displays, help entries and walkthrough hook. D6 correctly preserves goal-relay authority fields.

Proposed receipt, unwritten: `g1-s72 design critique round 1 at 7269ad21c; four material findings: authority handoff, legacy-engine retry, CLI compatibility, historical review text.`

**VERDICT: 4 material findings (fail test 2): S72-01, S72-02, S72-03, S72-04.**

Commit reviewed: `7269ad21cf09c6d83bcbe74e5f4460cf3c3cdef0`. Limitations: static review only; no tests, launches or browser checks ran. No files were changed, no subagents were used, and neither the prohibited configuration nor artifact contents were read.

Codex session ID: 01a0ebff-d8a1-7f82-a7b0-7a56bfbd7395
Resume in Codex: codex resume 01a0ebff-d8a1-7f82-a7b0-7a56bfbd7395
