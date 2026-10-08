# review-chain-stops-and-records

- State: done
- Risk: severity=3 novelty=2 exposure=3 accumulation=3 basis="The review chain gates every unit of every seat (severity 3, exposure 3); designed against the accepted mechanisms (novelty 2); several units over the review owners (accumulation 3)"
- Tier: 3
- Intent: The review chain records a stop by the stop rule, does not re-run green units, records what the stop needs, and runs design critique to convergence
- Origin: main
- Next step: Design critique to convergence, then build by hand with delegates
- Concluded: Review chain stops and records integrated at 4c0a5aadc: the unit stop record and asks, build outcomes and failed steps, the tree reservation, the round result, read publication, and the declared check (frozen check, test run --unit-run, builder and runner on one executor, publication replay, rebase carry subject-commit check). Integration gate: vet, internal, cmd and static ok after four fix-forwards; one load-fragile test (TestIntentLandWholeOwnerGitAdapter) passed 4 of 4 reruns and is recorded for the follow-up. Cheap-check preview, assertion audit and next-round declarations moved to process-changes-cover-declarations-and-interventions
- OpenedAt: 2026-10-07T04:46:02Z
- Revision: 3
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-07T04:46:09Z revision=2 opid=3CHFE53MGTPSH4EPQE5G5QFVK6-m1e-718ba0eb authority=proven digest=64679b31d155c2e6805493007ac70cc222af08e53841d19d4e396d341e8fd2cf episode=2

History:
- 2026-10-07T04:46:02Z VH1W99ET1X11VCDK54R4AWJ0EK-m1e-718ba0eb open actor=human:Wido targets=review-chain-stops-and-records
- 2026-10-07T04:46:09Z 3CHFE53MGTPSH4EPQE5G5QFVK6-m1e-718ba0eb approve actor=human:Wido targets=review-chain-stops-and-records
- 2026-10-08T14:20:42Z S21TEFBBK8M1WG0YWQ45QJH1XA-m1e-718ba0eb done actor=human:Wido targets=review-chain-stops-and-records
Integrity: sha256=28d3064d5f40be20f8ab9f249ca74d4d7d0a710a16917213ac36d62bffe2f2e0
