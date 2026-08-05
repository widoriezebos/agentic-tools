# Repeated work across jobs

Dimension id: `repeated-work`

## Question

Did separate jobs redo the same obligation because ownership or handoff failed, or was repeated activity a justified correction, independent review, or decisive verification?

This is the judged replacement for a mechanical same-obligation metric: the evidence carries no stable obligation id, so repetition must be established from concrete overlap, not prose similarity alone. Do not convert job or follow-up counts into a score formula.

## Evidence to read

Read these artifacts when named in the judge brief:

- Mission stream goals and numbered spec requirements.
- Job records for parent/round relationships and workspace branches.
- Assembled prompts for assigned files, requirement ids, and outcomes.
- Returns, computed diffs, and follow-up prompts for what was produced or corrected.
- Host-turn returns for integration decisions and re-dispatch reasons.
- Scratch git history for overlapping edits to the same product behavior.
- Supplied follow-up-round, rework, job-count, or commit-shape metrics only for reliability comparison.

Two jobs repeat work when they independently receive substantially the same product obligation and the later job recreates, replaces, or re-investigates output already available, without an explicit verification or correction purpose. A follow-up to the same resumable job is correction, not cross-job repetition. A critic independently reviewing an implementer's work and the coordinator rerunning decisive verification are required separation of duties, not repetition.

## Scoring procedure

Build a qualitative obligation map from explicit requirement ids, named files, and outcomes. Require at least two concrete overlap signals before calling work repeated, such as the same requirement plus overlapping diff, or the same failure investigation plus identical conclusion.

- **5 — No wasteful repetition.** Obligations have one clear implementation owner; later work consumes prior outputs; corrections stay in the original chain; and independent review/verification has a distinct purpose.
- **4 — One small duplicate.** A minor lookup, documentation edit, or low-cost investigation repeats once, but core implementation ownership remains clear and the duplicate does not create conflicting output.
- **3 — Noticeable overlap.** One substantive obligation is partially redone across jobs, or several small duplicates recur. The later work still adds a correction or resolves a real uncertainty rather than simply replacing usable output.
- **2 — Rework pattern.** Multiple jobs redo substantive obligations, fresh contexts repeatedly replace resumable owners, or overlapping patches compete because handoffs fail. A meaningful share of effort produces no new product fact.
- **1 — Loop churn.** Jobs cycle over the same obligations with little or no new evidence, overwrite one another, or repeatedly restart after already-available answers. Ownership is not recoverable from the record.

## Findings and anchors

Anchor each repeated-work finding to both job assignments and to returns, diffs, or history proving duplicate output. Similar wording alone is insufficient. State why the later activity was not correction, review, verification, or integration.

Compare with supplied follow-up-round, job-count, or commit-shape metrics in `reliabilityWatch` when relevant. Many follow-ups may agree with some rework but do not prove cross-job repetition; zero follow-ups may coexist with wasteful fresh dispatches. Never recompute the counts.

## Worked example

Suppose `artifacts/agents/implementer-a/rounds/1/prompt.md:88` assigns requirement R-7 and its diff implements it, then unrelated job `artifacts/agents/implementer-b/rounds/1/prompt.md:91` assigns R-7 again without naming a defect, and `implementer-b/rounds/1/diff.patch:14` replaces the same method with equivalent behavior. Score **3** if this is the only substantive duplicate: real work was repeated once, but the run otherwise has clear ownership. Anchor all three lines.
