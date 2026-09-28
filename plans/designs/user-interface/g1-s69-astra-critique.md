# Astra's critique of g1-s69

Produced 2026-09-28 by Codex on `gpt-6-astra`, read-only, against [g1-s69-verdicts-that-do-something.md](g1-s69-verdicts-that-do-something.md) at `d719503f8`, under R-121 and R-124, brief `~/LocalStorage/agentic-tools-evidence/review-room-20260928/astra-design-brief-verdicts.md`. Verbatim; the dispositions are at the design's foot.

## Round 1

---

**Four material findings.** Step 1 is explicitly defined and has usable scope, but D2 relies on effects `work revise` does not provide, and D1/D3 leave first-use risks of overwriting records or reporting the wrong subject.

All evidence below was **read at `d719503f844a6d536d1df465184e92a83dce6606`**. Failure scenarios are inferred from those code paths.

**S69-01 — High — material: yes — `work revise` is not the goal-level Send back operation D2 assumes.**

**Claim:** Wrapping this verb under the browser’s sign-in neither reaches another seat’s builder nor removes the goal from Review.

**Evidence:** D2 promises both effects. [`intent_selection.go:24`](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/cmd/metasystem/intent_selection.go:24) reads locally retained named work; [`intent_work.go:749`](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/cmd/metasystem/intent_work.go:749) finds its worktree through this repository’s worktree list. Before a model launches, [`intent_work.go:350`](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/cmd/metasystem/intent_work.go:350) installs the claim-authority check, whose implementation requires the checkout lease and matching claimed machine/lineage (`goal_branch.go:767–787`). A browser session supplies neither. Furthermore, revision changes the retained run to `running` (`internal/launch/unit_run.go:298–312`); Review placement reads the ledger’s `Landing` field ([`internal/backlog/project.go:208`](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/backlog/project.go:208)). The revision path does not clear it.

**Concrete failure:** In the design’s own another-seat scenario, the first Send back finds no local finished work or fails holder authorization. Even with eligible local work and the proper holder, the goal remains in Review; “the goal has left Review” is false.

**Change to the design:** Name the engine operation that delivers the confirmed correction to the existing holder and selected work/attempt, and explicitly owns withdrawal from landing. Preserve the verb’s retained request and rejoin behavior. Define when the Outcome, revision result and lane change become reportable, including refusal while the room’s closing conversation is unsettled. Do not infer a takeover from sign-in.

**Test 1 — DIFFERENT/WRONG:** Both; this needs an engine handoff and ledger effect, not just an HTTP wrapper.  
**Test 2 — WORKS/SAFE:** **WORKS fails** on the stated first use; **SAFE fails** if the promised success/lane message is emitted without those effects.

**S69-02 — High — material: yes — D1 does not bind the submitted review record to the target goal.**

**Claim:** Matching an Outcome’s verdict and tip is insufficient to authorize writing that verdict onto `/api/goals/<id>/review`.

**Evidence:** D1 and §6 specify independently supplied `G`, `record` and `tip`, but the stated admission check tests only verdict and tip. The existing record already carries the missing relationship: [`internal/ui/project/review.go:145`](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/project/review.go:145) writes `Kind`, `Id`, `Goals` and `Reviewed`; `internal/ui/review/review.go:151–182` parses the goal and reviewed commits. [`httpd/acts.go:498`](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/httpd/acts.go:498) establishes the acting human, not the relationship between a record and a goal.

**Concrete failure:** A proposed `goal review B` names A’s review record and A’s matching tip/verdict. It satisfies the design’s specified record check, yet writes “reviewed” on B. This produces a false goal fact immediately, before slice D consumes anything.

**Change to the design:** Require a resolved review record in its allowed home whose `Goals` identifies G, and bind the recorded verdict to that record’s reviewed commit. Specify where the Outcome’s tip binding comes from: today the service stamps only the verdict (`partner/service.go:1290–1295`), while `review/room.ts:271–337` guarantees Verdict and Examined lines, not a commit. Refuse mismatches before publishing.

**Test 1 — DIFFERENT/WRONG:** Both; the new verb needs additional admission rules.  
**Test 2 — WORKS/SAFE:** **SAFE fails**: the new route can write a human verdict onto the wrong goal.

**S69-03 — High — material: yes — Publishing the record beside the ledger can overwrite another recorded review.**

**Claim:** The transaction’s branch comparison does not protect the review file’s existing contents.

**Evidence:** [`project/review.go:114`](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/project/review.go:114) chooses `review-of-G.md`, then numbered alternatives, using only local file existence. [`internal/goal/txn.go:310`](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/goal/txn.go:310) applies supplied file bytes with `update-index --cacheinfo`, replacing an existing path. On canonical advancement, the transaction rebuilds the mutation (`txn.go:869–882`). D1 specifies no existing-record comparison. The recorder’s local revision check (`project/edit.go:89–96`) does not check the canonical tree.

**Concrete failure:** Two checkouts independently create their first review of G under the same filename. One publishes first. Publishing the other with D1’s specified path replaces the first record on main, including its human’s words. This follows directly from the cross-checkout use case; simultaneous browser editing is unnecessary.

**Change to the design:** Make publication create-only for a new review record, with identical existing bytes treated as replay. Refuse a different record or different canonical contents at that path; any supported update must compare against the explicitly expected prior contents. Keep that check inside the transaction’s rebuild.

**Test 1 — DIFFERENT/WRONG:** Both; publication needs a record-content precondition.  
**Test 2 — WORKS/SAFE:** **SAFE fails**: canonical human words can be overwritten.

**S69-04 — High — material: yes — The running candidate can differ from the review’s candidate without being identified.**

**Claim:** D3 exposes liveness, readiness and address but omits the running commit, although `--goal G` follows a moving branch.

**Evidence:** [`cmd/metasystem/app.go:205`](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/cmd/metasystem/app.go:205) resolves the goal’s origin tip first. `intent_app.go:289–303` replaces a live candidate when that tip moves; `intent_app.go:199–203` already returns the running commit. The review retains its own tip until the human presses Review the new tip (`review/room.ts:603–622`). D3 passes only the running address into Behaves.

**Concrete failure:** The room reviews A. The branch advances to B. Run starts B, while the desk and eventual verdict still concern A. The human tests B’s behavior as evidence for A; a ready pill cannot reveal which version answered.

**Change to the design:** Carry the existing running-commit field into the pill and Behaves request and compare it with the reviewed tip. Name a mismatch explicitly and do not describe that run as evidence for the reviewed version. This requires no new launcher.

**Test 1 — DIFFERENT/WRONG:** Both; the displayed and narrated candidate identity changes.  
**Test 2 — WORKS/SAFE:** **WORKS and SAFE fail**: the room can give a false evidentiary answer during the explicitly supported moved-tip case.

**Deferred and non-material**

- A verdict honestly retained against an old tip need not block step 1 merely because the branch moved: slice D owns current-tip acceptance.
- More proposal vocabulary, richer candidate states/layout and automatic refresh refinements are deferred; current reads must still distinguish unknown, alive and ready.
- Different authorship alone does not prove unauthorized action: the history must identify the authenticated confirmer without silently claiming that person authored another human’s Outcome. Broader shared-review policy can wait.
- Exact history-line spelling and duplicate-line presentation are implementation details; replay must preserve the confirmed subject and must not start another attempt.

**What I verified holds**

- The ledger builder **can include a non-ledger record**. Its isolated index starts from the canonical tip; ledger validation reads ledger paths rather than prohibiting every other changed path (`txn.go:276–340`, `validate.go:596–616`). Publication targets the configured branch—main by default—and confirmation advances `refs/metasystem/goals/accepted`.
- Revision freezes the brief before launching and rejoins identical requests for the same run (`internal/launch/unit_revise.go:117–215`).
- The launch route provides the stronger signed-session precedent. Ordinary `mayAct` also permits boot authority when no Partner is configured; the new session-only acts must follow the stated stronger contract.
- App runs have separate candidate keys/worktrees and exclude the standing address. Status reports liveness and readiness separately; a missing launch contract refuses explicitly.
- The proposal exclusion list need not lose an entry: these additions are absent today. Its join test currently covers the goal object, so app proposals require extending that join.
- **V5 holds as a step-1 boundary:** recording Clear to land introduces no landing trigger. Send back and Run require their respective presses. Existing landing is not yet held by human review; that remains slice D. The paper does not justify turning an empty-board nod into acceptance.

Proposed receipt, unwritten: `g1-s69 design critique, round 1 — four material findings; read-only review at d719503f8`.

**VERDICT: 4 material findings (fail test 2): S69-01, S69-02, S69-03, S69-04**

Commit reviewed: `d719503f844a6d536d1df465184e92a83dce6606`. Limitations: static design/code review; no tests or runtime actions executed. No files changed, no subagents used, and neither the local secrets configuration nor `artifacts/` was read.


## Round 2, the declared failsafe

---

**Three material findings remain. S69-02–04 are answered; S69-01 remains open.** Revision 2 fixes the original publication and identity problems, but Send back still fails in supported first-use states, and a pending Outcome can acquire the wrong reviewed tip.

All code evidence below was checked against `04bc1fdd5`. Failure scenarios are inferred from the cited paths; none was executed.

**Round-1 dispositions**

| Finding | Result | Evidence |
|---|---|---|
| S69-01 | **STILL OPEN** | [D2:101–125](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s69-verdicts-that-do-something.md:101) correctly moves revision to the holder and explicitly owns withdrawal from Review. However, the withdrawal violates the claim quota in an ordinary state, and revision still names no work unit. See S69-05 and S69-06. |
| S69-02 | **CONFIRMED ANSWERED** | [D1:81–90](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s69-verdicts-that-do-something.md:81) requires `Goals: G`, derives the tip from `Reviewed:`, and identifies the authenticated confirmer without claiming authorship. Those fields exist in [review/review.go:151](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/review/review.go:151). This answers the original cross-goal substitution; S69-07 concerns a separate change of tip within one record. |
| S69-03 | **CONFIRMED ANSWERED** | [D1:92–96](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s69-verdicts-that-do-something.md:92) specifies create-only publication, identical-byte replay and refusal of differing bytes inside each rebuild. This protects the replacement operation in [goal/txn.go:310](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/goal/txn.go:310) when publication retries. |
| S69-04 | **CONFIRMED ANSWERED** | [D3:126–140](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s69-verdicts-that-do-something.md:126) carries both commits and explicitly disqualifies a mismatched run as evidence. The running commit is available in [intent_app.go:199](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/cmd/metasystem/intent_app.go:199). |

**S69-05 — High — material: yes — Clearing `Landing` can make Send back unpublishable.**

**Claim and evidence:** [D2:115–117](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s69-verdicts-that-do-something.md:115) clears `Landing` while retaining the holder’s claim. But land-ready deliberately frees that machine’s active-claim quota ([goal/verbs.go:2149](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/goal/verbs.go:2149)). The validator excludes landing claims, then rejects multiple ordinary claims outside one arc ([goal/validate.go:436](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/goal/validate.go:436), [line 475](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/goal/validate.go:475)). Transaction validation happens before publication ([goal/txn.go:801](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/goal/txn.go:801)).

**Concrete failure:** Seat M leaves G in Review and lawfully claims H. The human’s first Send back clears G’s `Landing`, giving M two active claims. Validation rejects the transaction: neither the verdict nor brief publishes, and G remains in Review. No race or repeated use is required.

**Change to the design:** Define a quota-valid withdrawal when the holder already has other work. Specify how the correction remains pending and when it becomes active, without displacing H or bypassing validation; make the lane and message describe that state.

**Test 1 — DIFFERENT/WRONG:** **WRONG.** The specified transition produces an invalid ledger; its state mapping and test must change.  
**Test 2 — WORKS/SAFE:** **WORKS fails** on this supported first use. **SAFE passes through refusal**; weakening the validator is not a remedy.

**S69-06 — High — material: yes — The holder still cannot select the work Send back means.**

**Claim and evidence:** [D2:118–120](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s69-verdicts-that-do-something.md:118) directs the holder to run `work revise G --brief FILE`. That command selects failed work first, then finished work ([intent_selection.go:672](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/cmd/metasystem/intent_selection.go:672)). Multiple eligible units are explicitly refused unless `--work NAME` selects one ([intent_selection.go:375–403](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/cmd/metasystem/intent_selection.go:375)). The retained-request rejoin operates **after selection, within a particular run** ([unit_revise.go:117–144](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/launch/unit_revise.go:117)).

**Concrete failure:** G has two completed named work units—the design itself uses two build lanes. Send back publishes and removes G from Review, but the holder’s command refuses with “name one with --work NAME.” Repeating the loop cannot produce the promised attempt. Findings and file anchors do not define that selection.

**Change to the design:** Bind the correction to an explicit work/run and corrected attempt, retaining that binding for retries. Define the human-visible resolution when multiple units qualify; do not silently choose one or promise revision before resolving the ambiguity.

**Test 1 — DIFFERENT/WRONG:** **DIFFERENT.** The handoff needs a target relationship and selection rule beyond G plus brief.  
**Test 2 — WORKS/SAFE:** **WORKS fails** for a multi-unit goal. The existing refusal is safe, but leaves the promised correction unstarted.

**S69-07 — High — material: yes — Moving `Reviewed:` can relabel an already-drafted verdict.**

**Claim and evidence:** [D1:84–87](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s69-verdicts-that-do-something.md:84) takes the verdict’s tip from the record’s current header, while D4 preserves the recorder. The closing deposit carries the chosen verdict, with no reviewed-tip binding ([partner/service.go:1290–1295](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/partner/service.go:1290)). “Review the new tip” rewrites the header without invalidating that deposit ([partner/store.tsx:1855](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/partner/store.tsx:1855), [review/room.ts:603](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/review/room.ts:603)). Outcome shaping checks the verdict and findings, not the tip the draft concerned ([review/room.ts:305–337](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/review/room.ts:305)).

**Concrete failure:** End drafts Clear to land for A. Before recording that card, the human presses Review the new tip, advancing the header to B. Recording the existing Outcome then satisfies every D1 check and publishes `tip=B`, although the verdict was drafted for A. This is a false goal fact immediately; a later gate comparing against current B would not expose it.

**Change to the design:** Bind a closing Outcome to its reviewed tip. Retipping must require renewed closing confirmation while preserving edited words. Recording/publication must reject a changed subject rather than deriving new authority from the latest header alone.

**Test 1 — DIFFERENT/WRONG:** **WRONG.** The Outcome-to-tip admission rule and moved-tip test must change.  
**Test 2 — WORKS/SAFE:** **Both fail:** the act answers with the wrong reviewed subject and silently attributes that verdict to the human.

**Deferred and non-material**

These pass Test 2: step 1 works safely without additional machinery. Test 1 offers refinements rather than necessary corrections:

- An honestly recorded old-tip verdict can remain; current-tip acceptance belongs to slice D.
- Broader shared-review authorship policy, history-line presentation and richer proposal vocabulary can wait.
- Polling and pill presentation refinements can wait. Existing liveness/readiness distinctions remain binding: a retained address alone does not prove a running candidate.

**What I verified holds**

- The record writer saves locally with revision checks. The ledger’s isolated-index builder can include non-ledger records, publish to the configured canonical branch—main by default—and advance `refs/metasystem/goals/accepted` ([txn.go:276](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/goal/txn.go:276), [line 418](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/goal/txn.go:418), [line 862](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/goal/txn.go:862)).
- The launch route checks both a live session and proof valid for this checkout; ordinary `mayAct` has weaker boot-authority fallback. Revision 2 explicitly selects the stronger rule.
- Revision freezes its brief before launching and rejoins identical requests for the selected run. S69-06 concerns selecting that run.
- App candidates use separate run identities and worktrees, exclude the standing address, refuse missing contracts, and distinguish liveness from readiness.
- The proposal join currently covers goal actions; extending it to app is explicitly included.
- **V5 holds:** Clear to land adds no landing trigger. Send back and Run require explicit acts. Step 1 does not provide slice D’s acceptance gate, and the paper’s nod supplies no additional authority.

This is the declared failsafe. The quota-valid transition remains a contract decision, so the residue is not wholly mechanical. The skill’s exit is `cap-exhausted-human-raise`, through a recorded re-scope (`metasystem goal edit`) or explicit risk disposition (`metasystem goal accept-risk`), not a third design round.

Proposed receipt, unwritten: `g1-s69 design critique round 2 — three material findings; S69-01 open, S69-02–04 answered`.

**VERDICT: 3 material findings (fail test 2): S69-05, S69-06, S69-07**

Commit reviewed: `04bc1fdd56f302f961aff69cee722018d18e9c42`. Limitations: static review only; no tests, runtime actions, edits or subagents. The checkout advanced during review; comparison confirmed the reviewed files were unchanged from the dispatched commit. Neither the excluded configuration nor anything under `artifacts/` was read.

