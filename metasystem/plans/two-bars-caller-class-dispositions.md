# Dispositions: two-bars caller-class design critique

Design: plans/two-bars-caller-class-design.md (revision 1, job
implementer-178d269e0852ac7a8e897657). Critic chain: two-bars-cc-crit-3
(gpt-5.6-sol, xhigh, DESIGN-BEARING). Adjudicated by the dispatch
delegate m1b+main-1788333346-60696-6a3256. Materiality criterion as in
the critique brief. Round budget on the critic chain: three; failsafe
round 3.

## Round 1 — 3 material findings, verdictMaterialCount=3

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| TBCC-R1-LAWFUL-DELEGATE-COMMIT-PATH | accepted | Verified in the tree: scripts/agents/pre-commit-guard.sh:50-58 refuses every non-HUMAN raw commit without the wrapper token; internal/adapter/codex.go:59-64 and :165-171 grant the worktree's git metadata precisely so a worktree implementer's commit works ("every worktree codex implementer's commit died read-only"); dispatch.sh:1374-1402 builds the object quarantine for delegate git writes; docs/concepts.md:104-106 states raw agent commits are refused and wrapped ones carry identity. A worker committing in its dispatched worktree is therefore an anticipated flow that must go through the wrapper, and the design's global DELEGATE refusal plus its "use plain git" fallback (design :451-453) is the dead end the finding names. The design's own reject condition (section 10) fires. | Revision 2 adds a worker path: a DELEGATE whose ancestry maps to a dispatched job and whose repository root is that job's own worktree commits lawfully, ungated by the landing bar (worktree commits never land; landings ride --chain from the main checkout) and stamped with the job's identity; a DELEGATE anywhere else stays refused. Fold brief plans/two-bars-caller-class-fold2-brief.md. |
| TBCC-R1-FIXTURE-STUB-CONTRACT | accepted | Verified: static-reproof-fixtures.sh:227-242 and :407-413 implement `json get` only for the landing-observation fields (provenance, verdictTrailer, code, mode); :226 and :406 make `lease commit-token` a no-op, so "no token file was created" cannot discriminate; internal/behaviorsurface/consumer_wiring_test.go:66-76 implements no json get at all. Section 7 promises the stubs gain answers but does not specify the field extraction the new wrapper needs, so an implementer would have to invent the bed's contract. | Revision 2 specifies the stub contract: `json get` behaviour for path, claimEpoch, mainId, lineage, message and code, keyed on the commit-authority answer; the default verdict rule that keeps every existing `__lease-held` leg meaningful; a commit-token stub that writes a marker so the token assertion discriminates; and the consumer-wiring stub's answer. |
| TBCC-R1-NEGATIVE-BRANCH-PROOFS | accepted | Verified against the design's own text: refusals specified at :126 (empty or unknown path), :143-156 (token/verdict mismatch), :291-298 (missing lineage) have no leg in :323-384; the legacy legs are configured so token and default verdict agree, so none of the three consistency refusals is exercised. A wrapper that defaulted malformed output to human, accepted a cross-path re-entry, or restored the "+human" stamp would pass every listed test. | Revision 2 adds three named shell legs: empty-or-unknown path refuses with the named message; an accepted verdict with empty lineage refuses with the named message; an agent verdict while re-entering as `human` refuses with the mismatch message. |

Round 1 closes on three accepted findings; revision 2 goes back to the
same critic chain as round 2.
