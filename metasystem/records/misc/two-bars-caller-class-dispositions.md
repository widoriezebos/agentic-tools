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

## Fold to revision 2 (job implementer-178d269e0852ac7a8e897657-r2, Fable lane)

All three round-1 findings folded in one pass (design sections 1-4, 6,
7, 10; changelog in the header). The round reported three gaps; the
orchestrator's answers:

| Gap | Answer |
| --- | --- |
| F13, the pre-commit guard's token geometry: on a guard-enrolled machine a worker's WRAPPED worktree commit is refused because the guard reads the token under the main root while the wrapper mints it under the worktree (pre-commit-guard.sh:26, :58 versus commit.sh:6, :98). | DECIDED: resolution (a) — the guard derives its root from the committing repository's own geometry (toplevel plus prefix, the wrapper's geometry at commit.sh:141-142), never from its install location; the wrapper's token placement is unchanged, so the design's reject condition (c) does not fire. This is a pre-existing defect (today's worker commits take the human path and mint the token under the worktree, and meet the same guard), so it rides as its own slice on two-bars-for-changes, "guard-geometry", sequenced right after this one, not folded into this design. Recorded on the goal. |
| The Devin ACP server carries no instance tag in argv (devin.sh:338), so the custody join on (pid, start) is primary and the tag a cross-check. | ACCEPTED as the design's decision; the tag stays optional. |
| Two decisions left to the critic to confirm: `--push` refused on the worker path; the trailer names the custody-joined running job (a follow-up's -rN id). | ACCEPTED by the orchestrator; the round-2 brief asks the critic to confirm them. |

Revision 2 goes to the same critic chain as round 2 of three.

Round 2's dispositions are in plans/two-bars-caller-class-dispositions-r2.md (one table per file for the mechanical join).
