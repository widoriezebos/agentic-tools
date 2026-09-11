# Critique register: human-carried-landing-carry design, read 1

Job hcl-crit1-20260911 (codex, gpt-5.6-sol, design-critic) reviewed revision 3 of plans/human-carried-landing-carry-design.md at local scaffold e8bdd13f4 (sha256 eaf9ffd4d831317f1dab1aa90d45313a9d2a74aab3916d4d6c920bc16a2a9698), read at 976cb50bb. Material findings: 19 of 19. The critic's runtime is read-only; the register is projected verbatim from artifacts/agents/hcl-crit1-20260911/rounds/1/return.json.

## HCL-C-21 (critical)

**Claim.** Section HCL-LANDING-05: the refusal-code branch can land two failures under a word naming only one. When the ordinary verdict code matches the word, step 12 ignores every missing or failing testing group.

**Evidence.** metasystem/plans/human-carried-landing-carry-design.md:267-282 defines M, then requires M to be exact only for group words. A code word naming missing-declaration therefore lands even when M also contains goal-full-coverage. metasystem/artifacts/agents/hcl-context/design-brief.md:154-156 requires exactly one named refusal or group, never a blanket. What changes if this stands: a code hit must require M to be empty, or the design must specify another exact one-defect relation and add a two-failures-one-name fixture.

## HCL-C-22 (high)

**Claim.** Section HCL-LANDING-05: the unneeded outcome consumes a verified human word and lands without asking even though the named refusal did not occur.

**Evidence.** metasystem/plans/human-carried-landing-carry-design.md:278-281 consumes the word as unneeded with no obligation. The settled rule at metasystem/plans/human-carried-landing-design.md:56-64 says a word that names something that no longer exists is a question back to the human, not silent consumption. What changes if this stands: unneeded must ask for a fresh instruction or proceed as an ordinary landing without consuming the carry word; its fixture must assert that distinction.

## HCL-C-03 (high)

**Claim.** Section HCL-LANDING-05, HCL-C-03: the fallback evaluator has neither a defined live-failure input nor an honest policy boundary.

**Evidence.** metasystem/plans/human-carried-landing-carry-design.md:272-278 requires the base classifier to know that the live judge failed, but metasystem/internal/landing/observe.go:43-55 has no such parameter and the design lists only Carried and ProjectTree additions. metasystem/scripts/agents/commit.sh:477-490 also parses decisions with the live engine even after fallback. More fundamentally, metasystem/scripts/agents/commit.sh:308-317 currently makes the candidate-built engine the policy owner; the proposed base binary predates candidate policy or schema changes yet may call its answer complete. What changes if this stands: specify an authenticated parameter carrying live-failure state, parse fallback output without depending on the failed engine, and refuse fallback interpretation of candidate changes to policy or wire formats the base does not understand.

## HCL-C-04 (high)

**Claim.** Section HCL-LANDING-05, HCL-C-04: carry-forward mutates the leased checkout outside the lease's run-held critical section.

**Evidence.** metasystem/plans/human-carried-landing-carry-design.md:356-369 prescribes update-ref, rebase, and reset before commit. metasystem/scripts/agents/land.sh:537-590 invokes commit.sh only after staging, while metasystem/scripts/agents/commit.sh:31-54 is where run-held begins; metasystem/internal/lease/verbs.go:462-481 holds the lock only around that child. The temporary plumbing commit intentionally skips hooks, while the eventual commit still uses the normal hook path; hooks are not the defect. What changes if this stands: the entire carried mutation sequence must run under one verified lease epoch, with a fixture that attempts a concurrent lease mutation during carry-forward.

## HCL-C-19 (high)

**Claim.** Section HCL-LANDING-05, HCL-C-19: recovery from a local carried commit deletes the commit and then instructs the wrapper to push it.

**Evidence.** metasystem/plans/human-carried-landing-carry-design.md:364-367 defines carry-forward step 2 as rebase followed by reset --soft to origin/main, explicitly removing the carried commit. Lines 428-430 invoke that step after an origin move and then continue at push step 4. What changes if this stands: the recovery branch must either retain and push the rebased commit or reset, restamp a new commit, and repeat all tree and trailer postconditions before pushing.

## HCL-C-18 (high)

**Claim.** Section HCL-TRANSACTION-06, HCL-C-18: the proposed goal recover extension cannot discover either crash case it claims to close.

**Evidence.** metasystem/plans/human-carried-landing-carry-design.md:438-459 adds only a requestForEntry case for an existing carried journal intent, then claims recovery can turn an origin trailer or an expired word into a carried row. metasystem/internal/goal/recover.go:33-64 walks journal entries only. A crash after the code push but before goal carried starts, and an expired word for which goal carried never started, create no carried journal entry. What changes if this stands: record recoverable carried intent before the push, or specify a separate fresh-ledger scan that discovers words and reconstructs exact safe intent from their rows and trailers.

## HCL-C-23 (high)

**Claim.** Section HCL-TRANSACTION-06: carry-status defines a superseded state but gives land.sh no branch for it.

**Evidence.** metasystem/plans/human-carried-landing-carry-design.md:421-425 includes superseded:<opid>; lines 428-436 branch on none, local, origin, ledger, expired, and missing only. An implementer must guess whether superseded is success, an ask, or a fresh landing. What changes if this stands: specify a terminal no-landing outcome for superseded words and test that no commit, carried row, or transport side effect is produced.

## HCL-C-24 (high)

**Claim.** Section HCL-TRANSACTION-06: origin consumption is clock-dependent and can miss an already-landed commit, permitting replay.

**Evidence.** metasystem/plans/human-carried-landing-carry-design.md:250-255 and 417-419 search reachable commits with git log --since=<row At>. Git's since filter uses commit timestamps; a cross-seat clock skew or explicit backdated committer timestamp can place the exact Carry trailer before the row timestamp even though the commit is reachable. What changes if this stands: scan the bounded reachable ancestry by the globally unique exact trailer without a wall-clock cutoff, or use an ancestry anchor whose correctness does not depend on clocks.

## HCL-C-25 (high)

**Claim.** Section HCL-TRANSACTION-06: ApprovedRef idempotence accepts a conflicting carried payload as success.

**Evidence.** metasystem/plans/human-carried-landing-carry-design.md:438-445 returns AlreadyApplied as soon as any carried row has the same ApprovedRef, independent of the new operation identifier; it does not compare commit, tree, workspace, past, battery, judge, or outcome. metasystem/internal/goal/txn.go:728-743 turns AlreadyApplied into confirmed success. What changes if this stands: an idempotent replay must compare every immutable field and refuse a mismatched payload; add a negative fixture, not only the same-arguments replay fixture.

## HCL-C-26 (high)

**Claim.** Section HCL-TRANSACTION-06 and HCL-COUNTER-08: a crash after the carried ledger row but before the counselor append permanently loses the required carried-landings receipt.

**Evidence.** metasystem/plans/human-carried-landing-carry-design.md:404-415 calls goal carried one transaction step, while lines 543-548 place a separate local counselor append inside its command layer. The ledger branch at lines 433-435 runs only transport and never replays goal carried. Current command-layer precedent at metasystem/cmd/metasystem/goalsync_mutations.go:341-362 publishes the goal first and appends counselor state afterward. What changes if this stands: carry-status must report the counselor postcondition and the ledger branch must idempotently repair it, or the append must be represented by durable pre-push intent.

## HCL-C-02 (high)

**Claim.** Section HCL-IDENTITY-02, HCL-C-02: authorityGeneration does not perform the claimed join to a real proof, and generation zero is indistinguishable from a production row fabricated with fixture semantics.

**Evidence.** metasystem/plans/human-carried-landing-carry-design.md:119-139 says generation zero means a fixture root and that the row joins to enrollment and proof files, but carried classification at lines 237-245 checks only the row outcome and token. metasystem/internal/humanauthority/authority.go:123-145 binds fixture authority to a root only in the live in-process proof; that fact is absent from the rendered row. metasystem/internal/humanauthority/authority.go:700-740 describes the proof file as post-act audit evidence, and the existing command pattern writes it after publication. What changes if this stands: production validation must reject generation zero without fixture-root authorization, and the design must specify how missing enrollment or proof audit evidence is detected and repaired, or stop claiming readers can make that join.

## HCL-C-27 (high)

**Claim.** Section HCL-IDENTITY-02: publishing the new authority outcome has no rolling-compatibility rule, so an old engine cannot read the fleet ledger after the first carry word.

**Evidence.** metasystem/internal/goal/file.go:1436-1441 sends every non-channel outcome to the temporary-authority validator; metasystem/internal/governance/types.go:134-140 rejects anything except TEMPORARY_HUMAN_WORD. Revision 3 adds HUMAN_AUTHORITY_PROVEN but specifies no format version, capability fence, or fleet arm-before-write transition. What changes if this stands: land a backward-readable parser or a capability-gated fleet rollout before enabling goal carry, and fixture an old engine fetching a ledger containing the new row.

## HCL-C-28 (critical)

**Claim.** Section HCL-COUNTER-08: the landing's cap and debt checks read a stale accepted ledger even after land.sh fetches origin/main.

**Evidence.** metasystem/plans/human-carried-landing-carry-design.md:231-233 names goal.Project and lines 594-600 assume the landing can read every live goal after the code fetch. metasystem/internal/goal/project.go:48-66 with fetchFirst false reads refs/metasystem/goals/accepted; metasystem/internal/goal/verbs.go:68-77 uses exactly that offline read. metasystem/scripts/agents/land.sh:432-437 fetches only refs/remotes/origin/main and does not advance the accepted ledger ref. What changes if this stands: the landing must obtain one fresh validated goal-ledger tip and use it consistently for word, cap, and debt checks, with a peer-debt fixture.

## HCL-C-29 (high)

**Claim.** Section HCL-COUNTER-08: a carry word is not bound to the seat that later uses it, making per-seat cap attribution ambiguous and bypassable.

**Evidence.** metasystem/plans/human-carried-landing-carry-design.md:223-224 supplies the current actor, while lines 579-589 attribute the word to the machine segment of its operation identifier. Nothing requires those machines to match. After a claim transfer, seat B can use a word created or polled on seat A; implementers can count either A or B, and one choice evades B's cap. What changes if this stands: require the landing actor's machine to equal the word's seat, or define a human-authorized transfer that atomically moves cap ownership.

## HCL-C-30 (high)

**Claim.** Section HCL-COUNTER-08: --supersede has no decidable target preconditions or retry-time race rule.

**Evidence.** metasystem/plans/human-carried-landing-carry-design.md:579-588 says the new row writes supersedes=<opid>, but does not require the target to be one of the current seat's blocking open, unexpired, unconsumed words, nor require that condition to be rechecked on each ledger transaction rebuild. An arbitrary, expired, already-consumed, or concurrently landing word can therefore be marked superseded. What changes if this stands: define exact same-seat/open/unconsumed preconditions inside the mutation callback and specify the outcome if the target changes before publication.

## HCL-C-07 (medium)

**Claim.** Section HCL-OBLIGATION-07, HCL-C-07: skipping the critic finding read leaves the accepted-risk counselor record without required semantic fields.

**Evidence.** metasystem/plans/human-carried-landing-carry-design.md:503-515 skips CritiqueRegisterDecisionFinding and specifies only a commit citation for AppendCarriedAcceptedRisk. Today metasystem/cmd/metasystem/goalsync_mutations.go:326-350 obtains rigor class, title, claim, and evidence from that finding, and metasystem/internal/counselor/register.go:26-46 requires a title, reason, evidence facts, class, and review link. What changes if this stands: define the complete carried accepted-risk record schema and deterministic sources for every field, plus its idempotent replay behavior.

## HCL-C-11 (high)

**Claim.** Section HCL-C-11 and build slices: Slice 2a activates a carried override that Slice 2b does not yet implement, and its no-pending test cannot detect that mismatch.

**Evidence.** metasystem/plans/human-carried-landing-carry-design.md:674-680 says the test checks only carry and carried in the goal verb table. metasystem/internal/refusal/register_test.go:248-283 confirms collectGoalVerbs reads only that table. Yet metasystem/plans/human-carried-landing-carry-design.md:820-827 places the register flip and goal verbs in Slice 2a and land.sh/commit.sh in Slice 2b. Slice 2a can therefore pass while every Override says land.sh --carried and that command does not exist. What changes if this stands: keep the internals inactive and Pending set until 2b, then test the actual goal, landing, land.sh, and commit.sh entry points. As scoped, 2a is not independently buildable as a truthful Sol-round landing; only the already-deferred documentation can remain a follow-up without weakening the final goal.

## HCL-C-31 (high)

**Claim.** Sections Fixtures and Files: five changed package families are unowned by the testing contract, so the proposed proof plan will not select their package-specific tests.

**Evidence.** metasystem/plans/human-carried-landing-carry-design.md:802-810 defers checking counselor, channel, refusal, humanauthority, and governance. metasystem/testing.json:4-22 has no surface for counselor, channel, humanauthority, or governance and owns only metasystem/internal/refusal/register.go, not register_test.go. metasystem/internal/testpolicy/select.go:119-190 sends such paths to the generic unknown fallback. What changes if this stands: testing.json must gain reviewed ownership and appropriate groups for all five package families before the first implementation slice.

## HCL-C-32 (medium)

**Claim.** Section Fixtures of this revision: the page asserts that every named fixture already exists and passes, but the new fixtures are absent and at least the transaction-order oracle conflicts with its carried-forward definition.

**Evidence.** metasystem/plans/human-carried-landing-carry-design.md:722-752 says each named fixture is one existing passing test. A repository search found none of the new HCL identifiers, TestHCL03NoPendingAfterSlice2, or new carried fixture legs outside the plans. The old HCL-06-ORDER condition at metasystem/plans/human-carried-landing-design.md:263-266 asserts four writes, while revision 3 defines six at metasystem/plans/human-carried-landing-carry-design.md:402-415 and merely calls HCL-06-ORDER an old fixture. What changes if this stands: list every future test or bed leg with an exact oracle, update HCL-06-ORDER to the six-step transaction and its sub-write crash points, and stop describing unimplemented fixtures as passing evidence.

## Gaps the critic named

- No fixture bed or test suite was run because the review brief explicitly prohibited running beds.
- The runtime is read-only, so metasystem/records/misc/human-carried-landing-carry-critique-r1.md was not created; this JSON is the returned register for coordinator projection.
- The launcher reported the provider tool catalog as unobserved, so this critique relies on repository evidence and is advisory about runtime isolation.

## Coordinator dispositions (m1d, 2026-09-11, binding on fold 1 = revision 4)

All nineteen accepted; none refuted. The fold answers them in the page. Narrowings:
- HCL-C-21: a code hit requires M empty; a group hit requires O to pass or be one of the receipt-refusal codes and M to equal {G}; one word, one defect, with the two-failures-one-name fixture.
- HCL-C-22: `unneeded` asks (exit 3) instead of consuming; the ordinary landing is the human's next command.
- HCL-C-03: the base judge is limited to what the base can read: it refuses (asks) when the candidate changes policy or wire formats it does not know; the live-failure state travels as an authenticated parameter; the fallback's output is parsed without the failed engine.
- HCL-C-04, -19: the whole carried sequence runs under one lease epoch; recovery keeps and pushes the rebased carried commit after re-checking the tree and trailer postconditions.
- HCL-C-18, -26: durable pre-push intent (a `carrying` journal entry written before the push, holding everything `goal carried` and the counselor append need) so recover.go's walk finds it; `landing carry-status` reports the counselor postcondition and the ledger branch repairs it.
- HCL-C-23: `superseded` is a terminal no-landing outcome with its negative fixture.
- HCL-C-24: consumption by an ancestry scan for the exact trailer, bounded by the row's opid/ancestor anchor, never by a clock.
- HCL-C-25: replay compares every immutable field and refuses a mismatch.
- HCL-C-02, -27: production readers reject generation zero outside a fixture-authorized root; the new outcome ships behind a ledger format version or an arm-before-write fence, with the old-engine fixture.
- HCL-C-28: the landing reads one fresh validated ledger tip for word, cap and debt.
- HCL-C-29, -30: the word binds its seat and the landing actor must match it (or a human transfer moves it); `--supersede` has exact same-seat/open/unconsumed preconditions rechecked in the mutation.
- HCL-C-07: the carried accepted-risk record's schema and field sources named in full.
- HCL-C-11: one chain, not two slices: the register flip and the verbs land with the wrapper; internals stay inactive and Pending until then.
- HCL-C-31: testing.json ownership for counselor, channel, refusal (tests), humanauthority and governance is part of this chain.
- HCL-C-32: fixtures are named as future tests with oracles; HCL-06-ORDER is the six-step transaction.
