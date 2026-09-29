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

## Round 2 (the failsafe)

Read of revision 2 at `c51695f49`, brief `~/LocalStorage/agentic-tools-evidence/signed-in-launch-20260929/astra-design-brief-r2.md`. Verbatim.

---

Two material findings remain under round 2’s narrowed scope. Both have bounded fixes and can become implementation test obligations; no third critique round is needed.

**S72-01 — High — material: yes — The launch verb can retarget a genuine session record**

**Claim and evidence:** The remaining path does not require fabricated JSON. A named record is loaded even without `--resume` ([seat_launch.go:209](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/cmd/metasystem/seat_launch.go:209)). Only the resume branch takes the destination from that record ([seat_launch.go:88](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/cmd/metasystem/seat_launch.go:88)). For a fresh invocation, `Sequencer.Run` overwrites the record’s machine and destination from command arguments and persists it before running the steps ([sequence.go:167](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/seat/launch/sequence.go:167)).

**Concrete failure:** A steward-owned launch invocation mistakenly retains a previous browser launch’s `--record` path while supplying a new machine and destination. The engine itself rewrites that genuine record’s destination, retaining its proposed session enrollment. D2 then compares the new clone with the already-rewritten destination and accepts the binding. The originating checkout’s steward need not be recognized against the fresh clone: recognition uses the target’s enrollment ([classify.go:513](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/lease/classify.go:513)). With no recognized ancestor or terminal, the caller becomes `UNTRUSTED`, which D2 admits.

This grants a different machine permanent enrollment under the previous human’s session. The caller supplied a stale record path; it did not forge a record. That satisfies the round-two condition for reopening S72-01.

**Change to the design:** Make a loaded session record’s destination authoritative for both named-record entry and resume. Reject conflicting command arguments before any write or launch step. Preserve that binding throughout the sequencer. Add a fixture using a genuine session record for destination A with a fresh invocation requesting B; assert refusal, no record mutation and no enrollment. This requires no token or ancestry-binding mechanism.

**Test 1 — DIFFERENT/WRONG:** WRONG: the current record-loading and rewriting flow defeats D2’s destination invariant.  
**Test 2 — WORKS/SAFE:** **SAFE fails:** authority for one machine can enroll another.

**S72-04 — Medium — material: yes — Retaining the CLI pair creates new undiscarded records that show review dates**

**Claim and evidence:** The S72-03 fold deliberately retains the temporary enrollment step’s words ([design:150–167](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s72-signed-in-is-enough.md:150)). Those words include the date ([sequence.go:683](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/seat/launch/sequence.go:683)). The fleet reads CLI and browser launches from the same record directory ([ui_fleet.go:79](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/cmd/metasystem/ui_fleet.go:79)); the full card renders `Step.Words` verbatim ([LaunchCard.tsx:143](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/fleet/LaunchCard.tsx:143)).

**Concrete failure:** Run the retained CLI pair path from the UI checkout. If enrollment succeeds and supervision subsequently fails, its new, undiscarded record appears on the fleet page with “temporary enrollment, review due …”. The same text can appear while that launch is running. Discarding the sole historical record does not prevent this case.

This meets the brief’s exception to S72-04’s deferral: an undiscarded record would display the words.

**Change to the design:** Keep temporary provenance in the CLI output and durable record, but suppress the successful temporary enrollment narration in the fleet card. Add a fixture for a new pair-based launch with successful enrollment and failed supervision. Do not substitute a claim of session enrollment.

**Test 1 — DIFFERENT/WRONG:** DIFFERENT: D5 must cover step narration as well as the dedicated review line.  
**Test 2 — WORKS/SAFE:** **WORKS fails:** the fleet page still displays a review date.

**Deferred and non-material:** Deliberate same-user record forgery remains outside the revised threat model. Verdict age, revocation, existing-machine migration and filtering discarded historical records remain deferred.

**What I verified holds:** S72-02 prevents the new flag from reaching a legacy clone by keeping legacy records unstamped and retaining a named refusal when enrollment is needed. S72-03 restores the CLI flags, validation and forwarding path, with explicit exclusion beside session enrollment. The existing enrollment-skip behavior remains available. D3’s identity-reader and rebuild requirements remain intact. All four round-one IDs have explicit dispositions.

Proposed receipt, unwritten: `g1-s72 design critique round 2 at c51695f49; S72-02/03 closed; S72-01/04 remain as destination-binding and fleet-rendering fixture obligations.`

Reviewed commit: `c51695f491b3d2b664e9cc82b0a60de0b3fe1287`. Evidence was read; proposed failures were inferred, not executed. No files changed, tests ran or subagents launched. Prohibited configuration and artifact contents were not read; the historical-record inventory remains the design author’s stated evidence.

**VERDICT: 2 material findings (fail test 2): S72-01, S72-04.**

