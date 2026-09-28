# Go surface consolidation — round 3 dispositions

Critic: design-critic-20260812t070347z-86d2 (codex, gpt-5.6-sol).
5 findings, 4 material. All material folded; the non-material count
drift fixed too.

| Finding | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| GSC-R1-014 | accepted | The >=3 floor missed two-verb invariant sequences, and shell re-entry (__record-create) hides pairs from a literal grep. | Census covers any fixed ordering of two or more invocations, same- or cross-family, with re-entry router branches in the corpus. |
| GSC-R1-015 | accepted | Coarsen-or-document without a criterion lets two implementers both claim conformance with opposite results. | Decision by rule: coarsen when the ordering is purely a decision ordering (no custody interleaving, no load-bearing intermediate state); keep-and-document otherwise. The three known hits are worked through in the doc: reservation keeps, reap coarsens, the pre-commit authority pair coarsens in step 4. |
| GSC-R1-016 | accepted | Step 1 could not activate aliases whose target families do not exist until steps 2-4; the reap-facts row was prose, not a mapping. | Step 1 lands the mechanism and the full inert table (tested row-for-row against the appendix); entries activate with their target family's commit; reap-facts gets an executable alias that retires inside step 3. |
| GSC-R1-017 | accepted | The three consumers legitimately differ in action authority (kill-capable vs no-kill vs record-only); a flat three-consumer wiring could orphan a live process or weaken budget enforcement. | The verdict owner's contract takes the caller's authority class and returns the verdict plus required actions split into must-perform and must-defer; the status-to-verdict mapping is stated once; fixtures assert each consumer class end-to-end. |
| GSC-R1-018 | noted (non-material) | Arithmetic drift: 27 appendix rows vs "26 verbs" and "dispatch (23)". | Counts corrected. |
