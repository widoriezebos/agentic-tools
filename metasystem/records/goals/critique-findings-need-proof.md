# critique-findings-need-proof

- State: done
- Priority: 1
- Sequence: 1
- Risk: severity=1 novelty=2 exposure=2 accumulation=2 basis="Text change to the critique skills and brief template that every seat follows; no code; reversible."
- Tier: 3
- Intent: Critique stays out of rabbit holes: a code finding counts only with a concrete scenario inside the declared threat model and a failing test; a fix that needs a new mechanism cuts scope instead; a loop whose findings don't fall, or land in the last fix, stops and steps back; the real thing runs before a second review. Every real finding is still fixed. Why: the lane redesign went 10 design rounds and ~15 fix rounds on hypotheticals without ever running. Pro: real defects still fixed, imagined ones don't drive work. Con: a true but hard-to-test finding waits on the follow-up list.
- Origin: human
- Next step: Design page, Astra critique via design review, then edit skills/code-critique, skills/design-critique and the review brief template
- Concluded: Landed 0c4880c65: skills/code-critique, skills/design-critique, review-brief template; Astra agreed at round 2
- OpenedAt: 2026-10-01T06:26:01Z
- Revision: 6
- Pinned: m1e
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:wido at=2026-10-01T06:26:07Z revision=2 opid=0EBBPHRSHSWHCHBSC2XRG9RDW1-m1e-b6a4eb0a authority=proven digest=99453b82af22bdce62f1464159af3686ba55394cf5b86eb603abac3955a7ca60 episode=2

History:
- 2026-10-01T06:26:01Z V9F862C5AKA0XW5EMG3RHF96KM-m1e-b6a4eb0a open actor=human:wido targets=critique-findings-need-proof reason=TierOverride: derived=2 set=3 why=Wido asked for an Astra design critique, which exists only at tier 3
- 2026-10-01T06:26:07Z 0EBBPHRSHSWHCHBSC2XRG9RDW1-m1e-b6a4eb0a approve actor=human:wido targets=critique-findings-need-proof
- 2026-10-01T06:26:13Z 8PFWTKV66HSP90320AT3YC8BFN-m1e-b6a4eb0a set-priority actor=human:wido targets=ask-what-happened-follow-ups,builder-proves-each-rule-by-mutation,busy-seat-shells-are-ended,critique-findings-need-proof,cross-cutting-change-inventories-its-readers,design-rounds-read-less-and-repeat-less,evidence-and-build-output-have-retention-and-stay-unindexed,fleet-doctor-repairs-what-stops-other-seats,host-setup-from-scratch,landing-lane-runtime-redesign,one-steward-per-checkout-on-its-own-root,receipt-writer-follows-the-worktree-it-runs-in,reviewers-check-the-rulings,round-proof-feeds-the-next-brief,seat-successor-continues-a-handoff-without-a-human,seats-spend-tokens-in-bounded-sessions,spend-fence-reports-tokens-per-model-and-cause,steward-acts-on-behaviour-patterns,stop-hook-never-forces-an-empty-turn,system-start-launches-your-agent,terminal-enrollment-per-computer,testing-surfaces-declare-their-mirror reason=priority-order subject=critique-findings-need-proof from=unranked to=1:1 requested-sequence=1
- 2026-10-01T06:26:20Z FA111AQ0MZTGCQT6YNRNCYFH9R-m1e-b6a4eb0a set-pin actor=human:wido targets=critique-findings-need-proof
- 2026-10-01T06:28:04Z W08MYB6XTVTMFBB8JHQBX7B2AE-m1e-f456f182 claim actor=m1e+main-1790454088-93948-21671b targets=critique-findings-need-proof
- 2026-10-01T06:35:08Z RSWAE8KACJEH5AFBYA9YKEKZZM-m1e-f456f182 done actor=human:wido targets=ask-what-happened-follow-ups,builder-proves-each-rule-by-mutation,busy-seat-shells-are-ended,critique-findings-need-proof,cross-cutting-change-inventories-its-readers,design-rounds-read-less-and-repeat-less,evidence-and-build-output-have-retention-and-stay-unindexed,fleet-doctor-repairs-what-stops-other-seats,host-setup-from-scratch,landing-lane-runtime-redesign,one-steward-per-checkout-on-its-own-root,receipt-writer-follows-the-worktree-it-runs-in,reviewers-check-the-rulings,round-proof-feeds-the-next-brief,seat-successor-continues-a-handoff-without-a-human,seats-spend-tokens-in-bounded-sessions,spend-fence-reports-tokens-per-model-and-cause,steward-acts-on-behaviour-patterns,stop-hook-never-forces-an-empty-turn,system-start-launches-your-agent,terminal-enrollment-per-computer,testing-surfaces-declare-their-mirror
Integrity: sha256=6a40e2bf656f68deb75dbfdc2593204effe7643eb61830dff131ce2259f07f35
