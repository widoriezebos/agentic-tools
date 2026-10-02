# one-folder-deployed-and-evolved

- State: approved
- Priority: 1
- Sequence: 4
- Risk: severity=3 novelty=2 exposure=3 accumulation=2 basis="Moves where every adopted project's state lives and what an upgrade replaces; a wrong boundary loses project state on upgrade (severity 3); a new layout rule and a new upgrade boundary (novelty 2); every adopter and this repository (exposure 3); every agent turn reads these paths (accumulation 2)"
- Tier: 3
- Intent: An adopted repository has one MetaSystem folder, metasystem/, that separates what MetaSystem deploys (replaced only by an upgrade) from what the application evolves as it is built with MetaSystem (plans, designs, decisions, records, testing contract, committed config), so adoption works and newcomers see one folder (Wido 2026-10-02). Spec: plans/designs/one-folder-deployed-and-evolved.md; tier 3; Astra critique under the stop rule before any build.
- Origin: main
- Next step: Astra critique of the design (tier 3: threat model and rabbit-hole risks named up front), then build step 1
- OpenedAt: 2026-10-02T14:27:17Z
- Revision: 5
- Pinned: m1e
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-02T14:27:24Z revision=2 opid=J4DDF6HKHD2PQVRBVK0E4BZR2X-m1e-9c612d71 authority=proven digest=427db9e3d13e3794fe6adfc0bb4315aa91e8df721ed56128aeb42752da9b759f episode=2

History:
- 2026-10-02T14:27:17Z 835YY2SYJ5EB97ZYCSK1FZJDB5-m1e-9c612d71 open actor=human:Wido targets=one-folder-deployed-and-evolved
- 2026-10-02T14:27:24Z J4DDF6HKHD2PQVRBVK0E4BZR2X-m1e-9c612d71 approve actor=human:Wido targets=one-folder-deployed-and-evolved
- 2026-10-02T14:27:30Z X97YDE3PK7ZETEXDH7G1XZMACF-m1e-9c612d71 set-pin actor=human:Wido targets=one-folder-deployed-and-evolved
- 2026-10-02T14:27:37Z 9C5K9E6BZE82WXNRWF4YMENF2S-m1e-9c612d71 set-priority actor=human:Wido targets=builder-proves-each-rule-by-mutation,busy-seat-shells-are-ended,cross-cutting-change-inventories-its-readers,design-rounds-read-less-and-repeat-less,evidence-and-build-output-have-retention-and-stay-unindexed,fleet-doctor-repairs-what-stops-other-seats,one-folder-deployed-and-evolved,one-steward-per-checkout-on-its-own-root,receipt-writer-follows-the-worktree-it-runs-in,round-proof-feeds-the-next-brief,seat-successor-continues-a-handoff-without-a-human,seats-spend-tokens-in-bounded-sessions,shipped-folder-holds-everything-the-runtime-needs,spend-fence-reports-tokens-per-model-and-cause,stop-hook-never-forces-an-empty-turn,testing-surfaces-declare-their-mirror reason=priority-order subject=one-folder-deployed-and-evolved from=unranked to=1:5 requested-sequence=5
- 2026-10-02T17:03:29Z Y8AT6TM53TN357Y4RPN3095FV9-m1e-f456f182 done actor=human:Wido targets=blocked-agent-asks-the-human,builder-proves-each-rule-by-mutation,busy-seat-shells-are-ended,cross-cutting-change-inventories-its-readers,design-rounds-read-less-and-repeat-less,evidence-and-build-output-have-retention-and-stay-unindexed,fleet-doctor-repairs-what-stops-other-seats,one-folder-deployed-and-evolved,one-steward-per-checkout-on-its-own-root,receipt-writer-follows-the-worktree-it-runs-in,round-proof-feeds-the-next-brief,seat-successor-continues-a-handoff-without-a-human,seat-works-without-a-person,seats-spend-tokens-in-bounded-sessions,spend-fence-reports-tokens-per-model-and-cause,stop-hook-never-forces-an-empty-turn,switch-on-trial,terminal-enrollment-per-computer,testing-surfaces-declare-their-mirror reason=priority-order from=1:5 to=1:4
Integrity: sha256=46b41726a62c6ca661834b6752d402e7aa10b477c51088e6bcd33756129de8e5
