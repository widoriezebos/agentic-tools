# go-groups-carry-their-target-as-the-test-timeout

- State: approved
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="severity 2: a correct candidate is refused at the gate by machine load, nothing granted or destroyed; novelty 1: one flag derived from a field the contract already carries; exposure 2: every landing whose plan selects a full-coverage group on a loaded seat; accumulation 1: each refusal costs one receipt run"
- Tier: 2
- Intent: The receipt's go adapter (internal/proofrun/test_go.go) runs a unit group as go test -json -count=1 <packages> without a -timeout flag, so every group sits on Go's default ten-minute limit whatever its declared targetMs. Seen 2026-09-10 11:11Z on m1 in a landing receipt: goal-full-coverage (targetMs 1800000) failed with 'panic: test timed out after 10m0s' at 600.466 s while every test that ran passed; the same package took 523 to 680 s in this seat's gates today, so on this Mac the group is a coin flip under load, and a candidate that touches internal/goal is refused by luck. Done means: a go unit group passes -timeout derived from its targetMs (never below Go's default), the group record names the timeout it ran with, and a fixture proves a group with targetMs above ten minutes keeps running past 600 s.
- Origin: main
- Next step: m1c's lane (proof engine): in test_go.go build the argv with -timeout <targetMs as a Go duration> when targetMs exceeds the default, record it on the group, one unit test; Sol builds, Opus critiques.
- OpenedAt: 2026-09-10T11:24:31Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-10T11:25:06Z revision=2 opid=4AH1P389PPBSZ6TWT43EP59GZ2-m1-c6925449 authority=proven digest=b2ee827f724e159ba6098d3c7e74370e8fdfe48608d85b50e42e94b9772c7f76

History:
- 2026-09-10T11:24:31Z J2NF02RE4HDBASGVHV14Z9H0MQ-m1-1701c13c open actor=m1+main-1788940932-18533-7fa6c2 targets=go-groups-carry-their-target-as-the-test-timeout
- 2026-09-10T11:25:06Z 4AH1P389PPBSZ6TWT43EP59GZ2-m1-c6925449 approve actor=human:Wido targets=go-groups-carry-their-target-as-the-test-timeout
Integrity: sha256=ace9e8c0c2915d6a32ce42fd0f996b66275f29e7fc2ab02bdb6b3ba3f7c0045e
