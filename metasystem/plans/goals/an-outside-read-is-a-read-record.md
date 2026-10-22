# an-outside-read-is-a-read-record

- State: approved
- Risk: severity=2 novelty=1 exposure=1 accumulation=1 basis="adds a way to record a read; a wrong record would let an unread unit land, so it must name the unit commit and the reader file exactly"
- Tier: 2
- Intent: A read made outside the machinery (an Opus subagent's report written as a reader record under records/misc) can be recorded as the unit's clean read with work review --commit <sha> --goal G --reader-record PATH, so a hand-built goal lands through the lane with its real reads on main instead of reads waived.
- Origin: human
- Next step: Design the --reader-record option on work review --commit (cmd/metasystem/goal_branch.go:251, internal/goal/branch/read.go:715; the reader-record source exists at internal/goal/branch/attest.go:338-355), ~40-60 production lines; confirm a pane reservation passes the claim check; Astra under the rare-case rule.
- OpenedAt: 2026-10-22T10:09:16Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-22T10:09:29Z revision=2 opid=DJJCXJRKFBZHF7NGBMH3JKQ8P5-m1e-4a1ba1c4 authority=proven digest=9e0caf300ee159e3691edc7200d32963714066ef268f712ab606ae4b8009506d episode=2

History:
- 2026-10-22T10:09:16Z 4P8WZ3JJ6EW0016D61TG519WY3-m1e-4a1ba1c4 open actor=human:Wido targets=an-outside-read-is-a-read-record
- 2026-10-22T10:09:29Z DJJCXJRKFBZHF7NGBMH3JKQ8P5-m1e-4a1ba1c4 approve actor=human:Wido targets=an-outside-read-is-a-read-record
Integrity: sha256=f0a7647f6800460a3571e340a82d9628993eeb8f840759310c2e624e7102177c
