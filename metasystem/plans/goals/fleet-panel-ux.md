# fleet-panel-ux

- State: approved
- Priority: 1
- Sequence: 1
- Risk: severity=1 novelty=2 exposure=2 accumulation=2 basis="Presentation of existing facts plus three thin endpoints for existing verbs (severity 1); a new layout (novelty 2); the person's main view (exposure 2); read many times a day (accumulation 2)"
- Tier: 2
- Intent: The fleet panel answers, at a glance and on a phone, what the person wants there (Wido 2026-10-02: 'the UX of the bottom half of the fleet panel is a mess... first think what I want to be able to do in that screen, use cases, and then design an elegant, intuitive UX and build it'): one verdict, what needs him with its one action, what each seat is doing and how far (the fleet table he likes, its Running column made a true Doing column), the landing lane as waiting/proving/landed today/came back in plain words with Pause/Resume/Land now buttons, technical details behind a disclosure. Spec: plans/designs/fleet-panel-ux.md.
- Origin: main
- Next step: Step 1 LANDED on main as bb738c1f6 (2026-10-03 06:18, hand-landed by m1e in Wido's word). Next: step 2 of the accepted design: (1) a seat whose progress card stalls goes into Needs you (Sol F-2, risk accepted ad54ad134); (2) the prover records a reason for a red proof so Needs you says why (F-1); (3) the proof-log endpoint; (4) Pause/Resume/Stop carrying the signed-in session; plus Talk and Forget when their goals land.
- OpenedAt: 2026-10-02T21:18:23Z
- Revision: 16
- Pinned: ui
- Budget: elapsedLimit=6h attemptLimit=9 reservedJobMinutesLimit=1080 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 1
- NormApproval: approvedRef=HF2WNFAAJ1HZAB4K3005BV0KGG-m1e-718ba0eb minutes=1080 reviewRounds=20 goalRevision=8
- Approved: by=human:Wido at=2026-10-03T03:12:42Z revision=9 opid=HF2WNFAAJ1HZAB4K3005BV0KGG-m1e-718ba0eb authority=proven digest=9bee65a93add12b95e7652e77deb806bb66f5e96716bcd7a8b8b442adc4e787d episode=9
- Sliced: machine=ui lineage=steward-seat revision=6 at=2026-10-03T00:05:30Z
- AcceptedRisk: finding=F-2 chain=code-critic-1eb6d6383985e27d682ddcc4 by=Wido opid=86BBAFQKGYCDB233EASW6NQQKQ-ui-31a738e9
- Episode: machine=ui lineage=steward-seat accountingRevision=9 episodeAt=2026-10-02T22:59:40Z episodeRevision=6 idleSeconds=5 released=2026-10-03T04:31:54Z

History:
- 2026-10-02T21:18:23Z EM5JJFDXWZ6592RRGHJA018JB8-m1e-9c612d71 open actor=human:Wido targets=fleet-panel-ux
- 2026-10-02T21:18:30Z X6N0HG00E1RJPSE293PPD72655-m1e-9c612d71 approve actor=human:Wido targets=fleet-panel-ux
- 2026-10-02T21:18:36Z EAK62KYS3P3HFT193DMFGPQS51-m1e-9c612d71 set-pin actor=human:Wido targets=fleet-panel-ux
- 2026-10-02T21:18:43Z MGWN9QEKSDG62Q0ZBFCY6EKEHZ-m1e-9c612d71 set-priority actor=human:Wido targets=builder-proves-each-rule-by-mutation,busy-seat-shells-are-ended,critique-stops-on-convergence,cross-cutting-change-inventories-its-readers,evidence-and-build-output-have-retention-and-stay-unindexed,fleet-doctor-repairs-what-stops-other-seats,fleet-forgets-an-unreachable-seat,fleet-panel-ux,one-folder-deployed-and-evolved,one-steward-per-checkout-on-its-own-root,receipt-writer-follows-the-worktree-it-runs-in,rosters-are-configuration-items,round-proof-feeds-the-next-brief,seat-path-lands-without-help,seat-works-without-a-person,spend-fence-reports-tokens-per-model-and-cause,stop-hook-never-forces-an-empty-turn,switch-on-trial,terminal-enrollment-per-computer,testing-surfaces-declare-their-mirror,ui-connects-to-a-running-agent,work-review-starts-its-critic reason=priority-order subject=fleet-panel-ux from=unranked to=1:2 requested-sequence=2
- 2026-10-02T21:23:24Z 87SK1HNSS72691NDAZ1HW4NHXK-m1e-9c612d71 done actor=human:Wido targets=builder-proves-each-rule-by-mutation,busy-seat-shells-are-ended,critique-stops-on-convergence,cross-cutting-change-inventories-its-readers,evidence-and-build-output-have-retention-and-stay-unindexed,fleet-doctor-repairs-what-stops-other-seats,fleet-forgets-an-unreachable-seat,fleet-panel-ux,one-folder-deployed-and-evolved,one-steward-per-checkout-on-its-own-root,receipt-writer-follows-the-worktree-it-runs-in,rosters-are-configuration-items,round-proof-feeds-the-next-brief,seat-claim-takes-the-next-ready-goal,seat-path-lands-without-help,seat-works-without-a-person,spend-fence-reports-tokens-per-model-and-cause,stop-hook-never-forces-an-empty-turn,switch-on-trial,terminal-enrollment-per-computer,testing-surfaces-declare-their-mirror,ui-connects-to-a-running-agent,work-review-starts-its-critic reason=priority-order from=1:2 to=1:1
- 2026-10-02T22:59:40Z ZYVWY8ZE320CWWJD8QTFAXSSV0-ui-71c5cb39 claim actor=ui+steward-seat targets=fleet-panel-ux
- 2026-10-03T00:05:30Z 10FY1H08D1ST48GE3ZRDEM1X77-ui-71c5cb39 slice-start actor=ui+steward-seat targets=fleet-panel-ux
- 2026-10-03T03:07:14Z 6S7M7NJE1CAQV0TXJ6ATEZ3S1E-ui-71c5cb39 ask actor=ui+steward-seat targets=fleet-panel-ux
- 2026-10-03T03:12:42Z HF2WNFAAJ1HZAB4K3005BV0KGG-m1e-718ba0eb set-budget actor=human:Wido targets=fleet-panel-ux displaced=ui+steward-seat@2026-10-02T22:59:40Z
- 2026-10-03T03:30:40Z 6VXGFMDNPESRGPYNDRKBFMHGDA-ui-71c5cb39 release actor=ui+steward-seat targets=fleet-panel-ux reason=re-claim after the budget raise rebound the claim to m1e's checkout; the critic dispatch was refused as bound under another claim
- 2026-10-03T03:30:45Z EHMZ83NVXTAT5MEJHVDN3CVCKQ-ui-71c5cb39 claim actor=ui+steward-seat targets=fleet-panel-ux
- 2026-10-03T03:42:47Z PDE259AWBC9VKPX1SYQB52N1NP-ui-71c5cb39 ask actor=ui+steward-seat targets=fleet-panel-ux
- 2026-10-03T03:43:11Z 86BBAFQKGYCDB233EASW6NQQKQ-ui-31a738e9 accept-risk actor=human:Wido targets=fleet-panel-ux reason=Accepted by m1e in Wido word: the accepted design puts stuck seats in Needs you with Stop in step 2; step 1 is read-only plus Land now. Fix in step 2.
- 2026-10-03T03:43:27Z XDJ5NR037DY9MQ3NMD61TDZ952-m1e-718ba0eb edit actor=human:Wido targets=fleet-panel-ux displaced=ui+steward-seat@2026-10-03T03:30:45Z
- 2026-10-03T04:22:05Z ZJ5MWTM6G5J5M45TG2AQQTWSVX-m1e-718ba0eb edit actor=human:Wido targets=fleet-panel-ux displaced=ui+steward-seat@2026-10-03T03:30:45Z
- 2026-10-03T04:31:54Z YM5ZEZFBBG48YHE1424W0RGRH2-ui-71c5cb39 release actor=ui+steward-seat targets=fleet-panel-ux reason=re-claim: the critic dispatch was refused as not the authenticated lease holder after a goal edit from another checkout
Integrity: sha256=95c2e370b90307174123a308f78a52bda5c76236481f254c09a1605ec5403e1c
