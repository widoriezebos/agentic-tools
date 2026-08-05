# Delegation discipline

Dimension id: `delegation-discipline`

## Question

Was delegation used for honest, substantive work with clear ownership, or as ceremony that satisfied a job-count floor while the coordinator retained the real implementation?

This dimension judges task partitioning and work substance. It does not reward the highest possible number of jobs, and it does not punish coordinator-owned design, adjudication, decisive verification, or certification.

## Evidence to read

Read these artifacts when named in the judge brief:

- Mission streams, benchmark spec requirements, and the coordinator's host-turn prompts and returns.
- Every implementer job record and assembled prompt, including its assigned outcome and workspace.
- Implementer returns and computed `diff.patch` files for what each job actually produced.
- Follow-up chains for whether a delegate retained ownership of its correction.
- Scratch-repository git history and product-path changes where the brief supplies them.
- The supplied delegation floor, delegated-share result, and job counts, if present. These are comparison inputs; never recompute them.

Classify each implementer assignment as substantive when it owns a coherent product obligation whose successful output could be reviewed independently. A trivial typo, redundant inspection, test-only restatement, empty patch, or already-completed task is not substantive merely because it produced a terminal job. Verification and critique are valuable, but dispatching only those does not satisfy an implementer-work claim.

## Scoring procedure

Inspect all implementer jobs and enough coordinator turns to identify who performed the corresponding product work. Distinguish legitimate integration edits from the coordinator silently rebuilding the delegated unit.

- **5 — Honest ownership.** Each stream delegates at least one coherent implementation obligation; briefs have clean boundaries; delegates produce substantive reviewable work; corrections return to the same owner when possible; and coordinator work stays within coordination, integration, adjudication, and certification.
- **4 — Sound with a small imbalance.** Delegation is clearly substantive, but one assignment is undersized, one integration step absorbs more implementation than expected, or ownership is slightly fragmented without becoming ceremonial.
- **3 — Mixed discipline.** At least one meaningful unit is delegated, but another job is ceremonial, duplicative, or poorly partitioned, or the coordinator performs a material share of an obligation nominally delegated. The run still gains real work from delegation.
- **2 — Mostly ceremony.** The required job exists but owns a marginal task while the coordinator builds most of the product, or jobs repeatedly return empty/redundant work because ownership was not real.
- **1 — Delegation claim false.** No implementer does substantive product work, jobs are dispatched only after the work is complete, or the coordinator uses delegates as labels for work it performed itself.

## Findings and anchors

Anchor a ceremonial-delegation finding to the job's assignment line and to the return, diff, host return, or history line showing the actual work split. Do not infer authorship from prose alone when a computed diff or history artifact is supplied. A small task is not automatically ceremonial; explain why it lacked independent product value.

Record reliability-watch entries for supplied delegation-floor, delegated-share, job-count, or commit-shape metrics that overlap this judgment. Agreement concerns direction, not equal numeric scales. Never calculate a share from blame or commits.

## Worked example

Suppose `artifacts/agents/implementer-a/rounds/1/prompt.md:82` assigns only a README typo, its `diff.patch:6` changes that one line, and `artifacts/agents/missions/run/turns/t1/return.json:27` states the coordinator implemented all numbered functional requirements. Score **2** if the typo job is the only implementer dispatch: a real patch occurred, but delegation was ceremony relative to the product. Anchor all three artifacts and record disagreement if the supplied mechanical floor merely reports `implementer-jobs=1` as met.
