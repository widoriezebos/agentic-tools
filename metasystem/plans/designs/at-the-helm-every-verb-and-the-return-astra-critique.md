# Astra round 1 (task-mumuiyu8-ibb685, commit 4cf8b855d), verbatim

Four material findings: two authority-scope failures and two failures in the stated first-use flow. The design names usable slices and a deferred list, so it passes the initial scope test. Evidence below was checked by reading the specified commit; proposed behavior is inferred, not executed.

**HB-01 — High — material: yes — Existing background jobs can inherit the holder’s authority.**

**Claim and evidence:** The design’s safety argument depends on every job running in a linked worktree ([design:30](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/plans/designs/at-the-helm-every-verb-and-the-return.md:30)). The launcher supports `shared-checkout`: automatic worktree selection is skipped when `--workspace` is supplied, and the shared path can be the primary checkout ([dispatch_phase.go:321](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/delegation/dispatch_phase.go:321), lines 413–432). The restriction on writable shared workspaces applies to review roles, not every job (lines 478–479). The runtime actually starts in that workspace ([round.go:356](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/adapter/supervisor/round.go:356)).

**Concrete failure:** A previously launched, writable, non-review job in the primary checkout continues running when the person takes the helm. Its `goal approve` receives the holder’s proof. No deliberate `cd` or guard bypass is necessary.

**Change to the design:** Exclude verified job and supervision custody in the command-layer admission callback, while preserving the explicitly admitted Partner and accompanying agent. Reverse the proposed test that admits a known job solely because its cwd is primary. The direct package-import cycle is real, but the injected callback already lives where `lease` is available.

**Test 1:** DIFFERENT — admission and its safety fixture change.  
**Test 2:** WORKS without it: yes; SAFE without it: **no**.

**HB-02 — High — material: yes — A helm grant can authorize a different repository.**

**Claim and evidence:** `helmAdmits(root, pid)` checks `helm.Active(cwd)` and the primary-checkout predicate for cwd, without requiring that the supplied authority root belongs to that seat ([design:34](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/plans/designs/at-the-helm-every-verb-and-the-return.md:34)). Meanwhile, `--repo` selects a different mutation root ([intent.go:625](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/cmd/metasystem/intent.go:625)). The proposed constructor then marks its proof observed for that supplied root; existing `ValidFor` checks the observed root, not the grant’s checkout ([authority.go:203](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/humanauthority/authority.go:203)).

**Concrete failure:** From primary checkout A, where Wido holds the helm, an agent runs `goal approve G --repo /checkout/B`. With B otherwise enrolled and usable, the proposal grants authority from A and binds it to B, although B has no active helm.

**Change to the design:** Require the mutation target and admitted cwd to resolve to the same canonical seat. Enforce the intended checkout restriction for an explicit `--repo` too. Add an A-active/B-inactive refusal fixture.

**Test 1:** DIFFERENT — grant scope and validation change.  
**Test 2:** WORKS without it: yes; SAFE without it: **no**.

**HB-03 — High — material: yes — The specified helm proof cannot supply `done`’s lineage.**

**Claim and evidence:** The constructor specifies `TerminalRef` and `TerminalGeneration`, but omits the private observed terminal ID ([design:32](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/plans/designs/at-the-helm-every-verb-and-the-return.md:32)). `done --by` takes the stopping-request path ([goalsync_mutations.go:1817](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/cmd/metasystem/goalsync_mutations.go:1817)). With no supplied or inherited lineage, that path accepts the proposed terminal-valid proof, calls `ObservedTerminalID()`, and immediately refuses an empty value: “a human stopping act did not retain its observed terminal identity” ([goalsync_mutations.go:661](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/cmd/metasystem/goalsync_mutations.go:661)). It never reaches the later enrollment-derived fallback.

**Concrete failure:** In the transcript’s no-lineage case, `goal done` fails both during the helm and when return injects the same constructor after removing the signature. The terminal reference and generation alone do not resolve this.

**Change to the design:** Populate the constructor’s observed terminal ID from the enrollment it already reads, or explicitly derive the matching lineage before invoking the stopping owner. Require a test through the real `done` owner with no lineage and, for return, an already removed signature. A mocked `done` seam cannot establish success.

**Test 1:** DIFFERENT — constructor/request data and the success fixture change.  
**Test 2:** WORKS without it: **no**; SAFE without it: yes, because it refuses.

**HB-04 — High — material: yes — Answering yes to feedback does not produce a valid read request.**

**Claim and evidence:** Return creates a patch and invokes `runIntentReviewDiagnostic`, but specifies no brief creation or argument ([design:47](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/plans/designs/at-the-helm-every-verb-and-the-return.md:47)). That adapter refuses immediately without `--brief` ([intent_manual_review.go:40](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/cmd/metasystem/intent_manual_review.go:40)). Calling the lower-level owner directly does not avoid the requirement: `StartRead` requires a brief path and later reads a nonempty file ([standalone_read.go:133](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/launch/standalone_read.go:133), lines 175–177).

**Concrete failure:** After the two ordinary commits, the person answers yes to independent feedback. The patch exists, but no read starts. Returning exit 0 does not satisfy the promised answer’s effect.

**Change to the design:** Automatically create a small review brief naming the commits, review scope and feedback-only purpose; pass it with the patch. Include it in the printed noninteractive command. Test the real diagnostic owner far enough to observe a launched read, rather than merely observing a call to the `read` seam.

**Test 1:** DIFFERENT — return must prepare and pass another required input.  
**Test 2:** WORKS without it: **no**; SAFE without it: yes, because it refuses.

**Deferred and non-material**

Under the brief’s first-use definition, these do not keep the review open:

- Amend/rebase/merge attribution, repeated trees, unusual commit dates under `--since`, and pushes to differently named branches: later, when those histories occur. Test 1: DIFFERENT; Test 2: WORKS/SAFE without expansion: yes for the stated straight-line case.
- Existing open reads, carry obligations, breach stops, unavailable runtimes, and filesystem failures: later recovery cases, not introduced by the described open/approve/commit/push sequence. Test 1: DIFFERENT; Test 2: WORKS/SAFE without added recovery: yes for that case.
- Yield deduplication and richer goal readback: `open` can prove repeatedly, and the proposed person-proof subject lacks the goal ID. Test 1: DIFFERENT; Test 2: WORKS/SAFE without richer reporting: yes; distinguish proof yields from distinct acts.
- Automatic goal inference on main: the explicit goal-ID question already provides a usable fallback. Test 1: DIFFERENT; Test 2: WORKS/SAFE without inference: yes.

**What I verified holds**

- `doneRequest` permits an unclaimed, approved human-origin goal to conclude with terminal-grade authority; no claim or landing receipt is required ([verbs.go:2586](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/goal/verbs.go:2586)).
- `ValidFor` and `TerminalValidFor` do not reread a live helm signature. Removing it first therefore does not itself invalidate the proposed in-process return proof.
- Parsed proof JSON lacks the private observation fields. The proposed UI boot exclusion addresses the identified long-lived authority cache.
- Standalone diagnostic reads use their own launch sequence without the goal-claim callback ([standalone_read.go:452](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/launch/standalone_read.go:452)).
- The signature-first return structure, existing ledger-fence owner, and `--recover-only --if-down` supervision route are suitable reuse points. The latter requires no new human proof.

The smallest viable correction is to repair these existing boundaries and supply the missing inputs; the first-use case does not justify a new queue or state machine.

Proposed receipt, unwritten: `Design critique r1, helm slices 2–3: four material findings; static review only; no edits.`

**VERDICT: 4 material findings (fail test 2): HB-01, HB-02, HB-03, HB-04.**

Commit: `4cf8b855dd9ef306d9f68f6118b51d4bc6aaeaa6`. Slice 1 inspected at `origin/helm-acts-s1`, resolving to `603afe09b03f4200fc9dacb4923d9860b1fcad30`. Limitations: read-only review; no tests or runtime reproductions; no private configuration or artifact contents read; no files changed.

Codex session ID: 01a0edd5-d847-7af2-b197-f49a2982f3a2
Resume in Codex: codex resume 01a0edd5-d847-7af2-b197-f49a2982f3a2


# Astra round 2, the failsafe (task-mumustv2-qcllb5, commit f837ac0da), verbatim

All four findings are **FIXED in the revised design**. No new first-use failure meets Test 2. Round 2 can close.

Reviewed commit: `f837ac0da95f185169d1398d9ed945467df9896c`.

| Finding | Disposition | Exact point resolved |
|---|---|---|
| HB-01 | **FIXED** | The [revised admission rule](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/plans/designs/at-the-helm-every-verb-and-the-return.md:30) checks the whole ancestry chain for supervision, adapter-supervisor custody, and steward plumbing. It therefore covers jobs running in the primary checkout despite an intervening agent runtime. Wiring through `cmd/metasystem` avoids the import cycle. |
| HB-02 | **FIXED** | [Authority root and cwd must resolve to the same common Git directory](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/plans/designs/at-the-helm-every-verb-and-the-return.md:34), closing the grant from a held seat to another repository through `--repo`. |
| HB-03 | **FIXED** | [The constructor now carries the private observed terminal ID](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/plans/designs/at-the-helm-every-verb-and-the-return.md:32). This supplies the field required by the [no-lineage stopping path](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/cmd/metasystem/goalsync_mutations.go:661). The proposed fixture exercises the real done owner before and after signature removal. |
| HB-04 | **FIXED** | [Return generates a nonempty brief beside the patch](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/plans/designs/at-the-helm-every-verb-and-the-return.md:47), and the [disposition explicitly passes both paths](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/plans/designs/at-the-helm-every-verb-and-the-return.md:83). This satisfies the diagnostic adapter’s required `--brief` and `StartRead`’s file check. The proposed command-output fixture includes both flags. |

**Deferred and non-material.** Older wording still says all jobs use linked worktrees, defers machinery ancestry, calls `lease` untouched, and shows a read seam without a brief argument. The explicit revision-2 rules and fixtures resolve those inconsistencies sufficiently for implementation. Prose cleanup: material **no**; Test 1 **SAME**; Test 2 **WORKS/SAFE without it: yes**.

Merge/amend/rebase attribution, unusual push destinations, pre-existing goal-state refusals, yield deduplication, and automatic goal inference on main remain outside the specified first use.

**Verified holds:**

- The no-flags actor path proves authority and resolves the holder before forwarding `--by`; [the current owner entry supports this](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/cmd/metasystem/intent_goals.go:175).
- A human-origin goal can conclude without a claim or landing receipt when its existing blockers and review obligations are absent. The [done checks](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/goal/verbs.go:2586) support the transcript.
- Proof validation does not require a live helm signature, so return’s deliberately constructed proof can survive signature removal. The design retains the take-grade and UI-boot exclusions.
- The proposed brief satisfies the current standalone-read requirements; the read remains unbound to the concluded goal.

This was a static design review, not implementation certification. No files changed or tests ran; restricted configuration and artifact files were not read.

Proposed receipt, **unwritten**: `Design critique round 2 at f837ac0: HB-01–04 fixed; no new first-use material findings; critique loop closed.`

VERDICT: 0 material findings (fail test 2): none

Codex session ID: 01a0edd5-d847-7af2-b197-f49a2982f3a2
Resume in Codex: codex resume 01a0edd5-d847-7af2-b197-f49a2982f3a2
