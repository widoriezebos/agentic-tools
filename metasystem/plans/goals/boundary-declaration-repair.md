# boundary-declaration-repair

- State: approved
- Priority: 3
- Sequence: 41
- Intent: A round's malformed boundary declaration (a path missing the metasystem/ project prefix) poisons the chain's cumulative union permanently: conformance validates every declared string from every round, and no later round can retract or correct an earlier round's strings - the chain gsb-joint hit this 2026-09-01 (round 1 declared 'metasystem.conf' and 'scripts/agents/goal-cli-fixtures.sh' unprefixed; round 2 redeclared everything correctly; conformance still refused on round 1's strings). The seat normalized the two prefixes in the round-1 record as a disclosed formatting repair (meaning preserved, the diff is the independent proof) - that should never be necessary or possible.
- Origin: main
- Next step: Appetite: 2h. Either conformance validates the union of MEANINGS (a path that normalizes into the project counts, or a later round's correct redeclaration of the same file supersedes an earlier malformed string), or a declaration-repair verb exists with its own audit trail. The seat hand-edit of 2026-09-01 is the anti-pattern to make unnecessary.
- OpenedAt: 2026-09-01T01:07:51Z
- Revision: 4
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=240 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T06:53:37Z revision=3 opid=A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f authority=proven digest=bb4a60e318702b00dd651aa5130b248c28f7577886bc73f819ef7ee63a9ddce9

History:
- 2026-09-01T01:07:51Z MMPHNSF0MD5J9SVJ1VT2KQAKRS-m2-bc1be9cb open actor=m2+mac-coordinator targets=boundary-declaration-repair
- 2026-09-01T20:26:18Z NC33XJJ2TXQPEKP24QHFQ24E53-m0b-6638932d set-budget actor=m0b+main-1788250419-3170380-8a1fb3 targets=boundary-declaration-repair
- 2026-09-06T06:53:37Z A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f approve actor=human:Wido targets=boundary-declaration-repair reason=sweep
- 2026-09-08T16:01:31Z 375ET9NRHSD3W8CWTBTA2QKMTF-m1-7cd0bd60 set-priority actor=human:Wido targets=boundary-declaration-repair reason=priority-order subject=boundary-declaration-repair from=unranked to=3:41 requested-sequence=41
Integrity: sha256=375d84e251336c1079fc7258a1ed3be1b695570a2630e291386e2a28873d3093
