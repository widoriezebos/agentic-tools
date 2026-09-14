# every-round-gets-an-independent-read

- State: approved
- Priority: 1
- Sequence: 54
- Risk: severity=2 novelty=1 exposure=3 accumulation=3 basis="severity 2: a builder that writes a test accepting its own bug passes its own verification and the defect reaches the bed or trunk; novelty 1: the independent read exists as a role, only its timing changes; exposure 3: every build round; accumulation 3: undetected rounds compound into the nineteen-round member"
- Tier: 2
- Intent: Delivery-efficiency phase D. Model strength goes where judgment pays: after every build round, before its beds run, an independent reader with a fresh context checks the round against its brief and attacks the implementation, and the next brief carries its findings. Why: on 2026-09-14 one independent read of a build found eleven material defects in a single pass, including a literal newline that would have broken every provider launch and a new stop allowance the build had quietly given one runtime; the same day a builder's own test accepted the malformed object it should have refused, and only a later bed caught it. A reader after each round catches 'the test accepts the bug' before a bed does. DONE: the dispatcher runs the code-critique role on each round's diff with the round's brief, the read is recorded on the findings record round-proof-feeds-the-next-brief composes from, a round whose read is missing or unread cannot be proved, and a fixture proves a planted test-accepts-the-bug defect is named by the read before any bed runs. The reader is a different model from the builder, as the lanes already require.
- Origin: human
- Next step: Design in the verification-loop program page: where in the round the read sits, what it receives, where its findings land, and its cost bound. Then build the dispatcher step and the record.
- OpenedAt: 2026-09-14T19:26:27Z
- Revision: 3
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-14T19:26:33Z revision=2 opid=WVWK15E0BEREQNTGCZB1Y3A8TJ-m1e-c6925449 authority=proven digest=fbccbfe124b5ea5361872b044e3cd2a584b139ef4892bc25d87277c2e9c5405d

History:
- 2026-09-14T19:26:27Z EETJ3AM9DM9T9453GF5XRRSS51-m1e-c6925449 open actor=human:Wido targets=every-round-gets-an-independent-read
- 2026-09-14T19:26:33Z WVWK15E0BEREQNTGCZB1Y3A8TJ-m1e-c6925449 approve actor=human:Wido targets=every-round-gets-an-independent-read
- 2026-09-14T19:26:39Z N7Q4B8NR6MVB2HFS6N6TTB6W37-m1e-c6925449 set-priority actor=human:Wido targets=every-round-gets-an-independent-read reason=priority-order subject=every-round-gets-an-independent-read from=unranked to=1:54 requested-sequence=append
Integrity: sha256=4dd645048cdaaf56bc3ace75adfffedf7bca7ea4af7e219f532bcaa289d2ac62
