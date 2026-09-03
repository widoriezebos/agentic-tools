Working Mode: design
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal severity-tiered-rigor)
Date: 2026-09-03

# Goal

Fold the design review of the tiering machinery into revision 3 of
metasystem/plans/severity-tiered-rigor-design.md. The review (chain
str-design-cc1, Sol) returned fourteen material findings against
revision 2's section "Revision 2 (2026-09-03): the tier is the budget,
the material stop closes the loop". This is the goal's one fold under
R-55-m1's shape (design, one review, one fold, one closing review, then
build) and R-60-m1's rule (a finding is material only if it changes
what gets built and names the artifact).

# Workspace

The delegate worktree the dispatcher created for this job. Change only
metasystem/plans/severity-tiered-rigor-design.md: append a section
"Revision 3 (2026-09-03): the review folded" that answers every finding
below by number, amending the mechanism points and the build lists in
place where the finding says so. Do not rewrite revision 2's text; the
new section supersedes what it names. Keep it under 250 lines.

# The two points that are Wido's, not yours

Do not decide these; write each as an open decision with the options
and the design's recommendation, so the human answers in one word:

- STR2-RULING-CONFLICT-06: whether a fourth critic round can ever be
  dispatched (R-60-m1 makes review depth part of the risk-based budget
  with no separate cap; the shipped skill says three rounds then stop).
- STR2-RESERVATION-RECOMMENDATION-14: the reserved minutes per tier box
  while every dispatch reserves the full 120-minute cap (the seat's
  recommendation is attempts times the dispatch cap; the review says
  that is sound only if 120 becomes an enforced maximum).

# The findings, verbatim from the critic

## STR2-TIER-AUTHORITY-01 (critical)

Claim: The tier must be authorization- and revision-bound before it can control dispatch or landing. As written, an owner could change an approved Tier 3 goal to Tier 1 and preserve the approval, while an active critic chain has no immutable tier snapshot. Amend metasystem/internal/goal/file.go, metasystem/internal/goal/approval.go, metasystem/internal/goal/verbs.go, and the goal-binding fields in metasystem/internal/dispatch/ so approval, claims, and chain roots bind the tier and define who may downgrade it.

Evidence: ApprovalDigest currently hashes only intent and the four-field budget. Existing goal editing rejects an approved intent change but permits other owner edits without invalidating approval. Dispatch resolves a claimed goal revision, whereas Revision 2 makes Tier freely editable and says dispatch reads the goal's tier.

## STR2-GOAL-SWEEP-02 (high)

Claim: The open-goal sweep is not a lawful migration as specified. It must be a sequence of normal goal-ledger transactions with a named authorized actor and an explicit classification decision for each goal, not an unqualified mutation in the part-one landing. Amend the migration owner and procedure in metasystem/plans/severity-tiered-rigor-design.md and add its transaction implementation to the part-one artifact list.

Evidence: metasystem/internal/goal/verbs.go refuses a non-human actor editing another actor's claimed goal, and every lawful edit is published as a revisioned goal transaction. Revision 2 assigns the sweep to “the coordinator” without authority, classification inputs, conflict handling, or a rule for concurrently claimed goals. Tier classification is a judgment, so it cannot be inferred safely merely to make the sweep idempotent.

## STR2-BUDGET-TUPLE-03 (high)

Claim: The five-number budget and its approval behavior need one complete contract. Amend metasystem/internal/goalbudget/budget.go, metasystem/internal/goal/budget.go, metasystem/internal/goal/file.go, metasystem/internal/goal/approval.go, metasystem/internal/goal/verbs.go, metasystem/internal/goal/norm.go, and metasystem/cmd/metasystem/goalsync_mutations.go to define the review-round member, bind tier and all five values into approval evidence, and distinguish an inside-box budget assignment from changing an already approved execution tuple. The current unchanged minute token cannot authorize an above-box review-round increase.

Evidence: All current budget constructors, parsers, renderers, synchronization flags, and approval digests require exactly four values. The only setter is SetBudgetApproved. The strict norm reference contains only goal, reserved minutes, and goal revision. Revision 2 simultaneously calls the box four-number notation, calls review rounds its fifth number, permits unapproved inside-box changes, and says the unchanged minute token approves raising review rounds.

## STR2-CONFIG-TOMBSTONE-04 (medium)

Claim: Deleting the old norm constant will not refuse stale configuration. Add an explicit tombstone check to metasystem/internal/config/validate.go and to whichever mandatory load boundary admits metasystem/metasystem.conf; include those artifacts and fixtures in part one.

Evidence: metasystem/internal/config/resolve.go reads only the requested configuration key and treats every other key as irrelevant. metasystem/internal/config/validate.go recognizes the old key only through GoalNormJobMinutesKey and has no general unknown-key rejection. Removing that identifier therefore turns the stale setting into an ignored setting, contrary to Revision 2's promised refusal.

## STR2-ROUND-ACCOUNTING-05 (high)

Claim: The tier-derived critic boundary needs a chain-root snapshot and a defined consumed-round counter in metasystem/internal/dispatch/critique.go and metasystem/internal/dispatch/finding_register.go. Count completed and failed critic attempts, exclude cancelled attempts, and evaluate rigor only for unresolved entries. The records contain enough status and round data to derive this count, but findingRegisterRound is not that count and a mutable current goal tier is not a stable boundary.

Evidence: A cancelled record advances findingRegisterRound specifically to unblock folding while the code says it consumes no cap. readCritiqueCapState nevertheless uses findingRegisterRound as the boundary number. It also searches every register entry for severe or unproven rigor after merely establishing that some unresolved identifier exists, so a resolved severe entry can affect a remaining bounded set. Revision 2 does not say when the tier or raised review budget is frozen onto the chain.

## STR2-RULING-CONFLICT-06 (high)

Claim: A human ruling is required before implementing review budgets above three. Amend either metasystem/plans/severity-tiered-rigor-design.md or metasystem/memory/rulings.md so one binding rule says whether a fourth critic round can ever be dispatched.

Evidence: Revision 2 permits goal set-budget --review-rounds to raise a chain above its tier default. Human ruling R-42-m0, recorded on the same date after the two cited rulings, says every loop stops at three and a fourth round is never dispatched. An implementer cannot preserve both outcomes.

## STR2-ARTIFACT-MEMBERSHIP-07 (critical)

Claim: Artifact existence is not a mechanical test of materiality. Amend metasystem/internal/critique/model.go and the generated schemas in metasystem/internal/returnschema/returnschema.go so an artifact must belong to the reviewed subject's changed-path set or a declared build output. Define a canonical representation for renames and restrict the literal NEW form to declared outputs; otherwise an unrelated existing path or an invented path that is never built can keep the loop alive.

Evidence: Revision 2 accepts any repository path resolving in the reviewed tree and any literal NEW path. Neither condition proves that the finding changes what will be built. An old side of a rename fails existence while the new side passes, and the design provides no reviewed-diff membership or planned-output membership rule.

## STR2-DEMOTION-TRANSITION-08 (high)

Claim: Artifact demotion must not enter metasystem/internal/dispatch/finding_register.go as ordinary material:false. Introduce a distinct normalized-demotion outcome that ignores a new invalid finding but preserves any pre-existing unresolved entry with the same identifier.

Evidence: foldCritiqueFindings marks an existing register entry resolved whenever the current return carries material:false. Under Revision 2, a critic can therefore re-emit an open finding with an empty or invalid artifact, have it demoted, and resolve the earlier finding. Omitting a synthetic finding is safe only if demotion is not treated as withdrawal or resolution.

## STR2-CLOSE-STATE-MACHINE-09 (high)

Claim: The close state machine is incomplete for mixed registers. Define an atomic transition table in metasystem/internal/dispatch/finding_register.go and metasystem/internal/dispatch/close.go: all unresolved bounded entries may defer; any open or disputed severe or unproven entry refuses without partially deferring others; no unresolved entries close early. Also add an explicit resolution reason for out-of-scope closure, because that surviving rule cannot be enforced from the current status vocabulary.

Evidence: The existing engine treats both open and disputed as unresolved. Revision 2 says disputed bounded entries defer but only “open” severe or unproven entries block, leaving disputed severe and unproven entries unspecified. Current status is only open, resolved, or disputed and records no out-of-scope reason, while Revision 2 requires scope closure to depend on rigor class.

## STR2-CLOSE-PERSISTENCE-10 (critical)

Claim: The two close exits require a new review-obligation data model and a recoverable cross-store transaction. Amend metasystem/internal/goal/file.go, metasystem/internal/goal/obligation.go, metasystem/internal/goal/verbs.go, metasystem/internal/counselor/sources.go, and the accepted-risk writer for metasystem/records/counselor/accepted-risk-register.jsonl; add all of them to part two. Define identifiers, human authority evidence, idempotency, and crash recovery across the critic register, goal ledger, and accepted-risk register.

Evidence: GoalFile carries one pointer to a governed recurring-process obligation, not a list of finding identifier, artifact, and test triples. That object requires authority, effects, assumptions, triggers, and a claim-revision binding, and clearClaimBinding removes it during lifecycle transitions including conclusion. The accepted-risk register requires a different strict schema with structured facts, citations, and review links, and the reviewed code exposes a reader but no close-verb writer. Revision 2 supplies no mapping or commit order for these three mutations.

## STR2-CRITIC-UNION-11 (critical)

Claim: Reading the register instead of the last return reopens critic shopping unless same-tree union lands in the same build. Amend metasystem/internal/validate/conformance.go and the close command in metasystem/cmd/metasystem/dispatch_verbs.go to join every critic root reviewing the implementation tree, or designate and enforce exactly one authoritative chain. The close selector must identify a chain, not only a goal.

Evidence: mergeCritique currently enumerates all eligible code-critic roots but returns success on the first passing one, even if another root fails. Multiple design and code critic roots can share one goal, so job critique-register-close --goal does not identify which register is closed. Revision 2 explicitly postpones the union and exact-tree certificate despite changing the zero-material source to per-chain registers.

## STR2-TIER1-PROTECTED-PATHS-12 (critical)

Claim: The Tier 1 landing rule is too broad and contradicts the orthogonal hazard rule. Amend metasystem/internal/landing/observe.go, metasystem/scripts/agents/path-classes.txt, and metasystem/internal/dispatch/admission.go so a small diff cannot bypass protected authorization, proof, dispatch, landing, or schema owners merely because all are classed as behavior. Preserve a DESTRUCTIVE-REACH hazard on any tier instead of forcing every Tier 1 job to MECHANICAL.

Evidence: The path manifest classifies all of metasystem/internal/ and metasystem/scripts/ as behavior, including the machinery that grants authority and verifies proof. Three files and forty changed lines can alter such a law. The MECHANICAL hazard class requires neither independent critique nor live proof, while DESTRUCTIVE-REACH requires both. Revision 2 first says hazard is independent and adds live proof to any tier, then says a Tier 1 implementer is MECHANICAL.

## STR2-TIER1-EVIDENCE-13 (high)

Claim: Part three lacks an evidence seam for its gate stamp and a complete diff metric. Amend metasystem/internal/landing/observe.go, metasystem/cmd/metasystem/landing_verbs.go, and metasystem/scripts/agents/commit.sh so structured test receipts are bound to the candidate tree before observation; add the latter two artifacts to the build list. Define changed lines and refuse binary patches, mode-only changes, and rename or copy shapes that otherwise appear to have zero text lines.

Evidence: ObserveParams contains the candidate tree, chain, direct-fix class, revert, goal, and actor, but no message or test evidence. metasystem/scripts/agents/commit.sh invokes landing observation before the commit is created and does not pass message content. Revision 2 lists only metasystem/scripts/agents/land.sh for the shell seam and leaves “changed lines” undefined for non-textual Git changes. A self-asserted message stamp also does not prove which tests ran against which tree.

## STR2-RESERVATION-RECOMMENDATION-14 (medium)

Claim: The reserved-minute recommendation is sound only if 120 minutes becomes an enforced maximum, which it is not today. Amend metasystem/internal/config/budget.go and metasystem/internal/dispatch/cap.go so each temporary tier pool is at least attempt limit multiplied by the maximum dispatch cap admitted for that tier's roles and runtime-model pairs, and reject explicit caps above that maximum or require a new approved budget. Hard-coding attempts multiplied by 120 can still stall a lawful ladder.

Evidence: ResolveCap uses 120 only as its final fallback. An explicit requested cap or role, runtime, model, or general configuration key may supply any positive integer. Therefore the proposed 360, 720, and 1200 reserved-minute defaults cover all attempts only under an unstated 120-minute ceiling.

# Constraints

Wall-clock budget: 40 minutes. Read-only outside the design file. The
design stays implementation-ready: every amended point names the file
and the function or table it changes, and the build lists (parts one to
three) are updated so the implementer has zero judgment calls.

# Expected Return

Per the implementer schema: the one-file boundary, and for each finding
the one line naming where revision 3 answers it.

# Gap Rule

stop and report a gap; never fill it silently.
