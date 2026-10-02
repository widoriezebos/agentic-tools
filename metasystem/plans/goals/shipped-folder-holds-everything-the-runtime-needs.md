# shipped-folder-holds-everything-the-runtime-needs

- State: approved
- Priority: 1
- Sequence: 6
- Risk: severity=2 novelty=1 exposure=3 accumulation=2 basis="An adopted repo breaks at runtime when a dependency is missing (severity 2); moving files and adding an audit (novelty 1); every adopter (exposure 3); every agent turn reads these files (accumulation 2)"
- Tier: 2
- Intent: Everything the MetaSystem runtime and its participating agents need lives in the shipped installation folder, so an adopted repository works (Wido 2026-10-02: 'we really need to move everything required by metasystem runtime and agents participating in that into the folder that is deployed/shipped. Otherwise the metasystem will break in an adopted repo'). Outside it there is only project state, which the runtime locates through its state root, never by a guessed relative path. Agent-facing texts (AGENTS.md, wow.md, skills, brief templates, hook and verb messages) cite installation files unambiguously from any working directory, and an audit refuses a reference to a runtime dependency outside the shipped set.
- Origin: main
- Next step: Inventory every reference from engine code and agent-facing texts that resolves outside the shipped set or ambiguously; classify each as shipped, project state or misplaced; then design the moves and the audit
- OpenedAt: 2026-10-02T14:15:33Z
- Revision: 5
- Pinned: m1e
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-02T14:15:40Z revision=2 opid=7T6AP97VFEYF1VTADH9SQPA2EJ-m1e-9c612d71 authority=proven digest=7daeea5c97b701d823c82669080281a567641cf9b160cb8240701e1914d1d562 episode=2

History:
- 2026-10-02T14:15:33Z 57S6AMPSN96W61BQX4VJRFA2PC-m1e-9c612d71 open actor=human:Wido targets=shipped-folder-holds-everything-the-runtime-needs
- 2026-10-02T14:15:40Z 7T6AP97VFEYF1VTADH9SQPA2EJ-m1e-9c612d71 approve actor=human:Wido targets=shipped-folder-holds-everything-the-runtime-needs
- 2026-10-02T14:15:47Z 4KGRZ0BZE0WFYC107QNDX6A3TE-m1e-9c612d71 set-pin actor=human:Wido targets=shipped-folder-holds-everything-the-runtime-needs
- 2026-10-02T14:15:53Z TRH5GDM36YYMTV93K14G425KTH-m1e-9c612d71 set-priority actor=human:Wido targets=builder-proves-each-rule-by-mutation,busy-seat-shells-are-ended,cross-cutting-change-inventories-its-readers,design-rounds-read-less-and-repeat-less,evidence-and-build-output-have-retention-and-stay-unindexed,fleet-doctor-repairs-what-stops-other-seats,one-steward-per-checkout-on-its-own-root,receipt-writer-follows-the-worktree-it-runs-in,round-proof-feeds-the-next-brief,seat-successor-continues-a-handoff-without-a-human,seats-spend-tokens-in-bounded-sessions,shipped-folder-holds-everything-the-runtime-needs,spend-fence-reports-tokens-per-model-and-cause,stop-hook-never-forces-an-empty-turn,testing-surfaces-declare-their-mirror reason=priority-order subject=shipped-folder-holds-everything-the-runtime-needs from=unranked to=1:5 requested-sequence=5
- 2026-10-02T14:27:37Z 9C5K9E6BZE82WXNRWF4YMENF2S-m1e-9c612d71 set-priority actor=human:Wido targets=builder-proves-each-rule-by-mutation,busy-seat-shells-are-ended,cross-cutting-change-inventories-its-readers,design-rounds-read-less-and-repeat-less,evidence-and-build-output-have-retention-and-stay-unindexed,fleet-doctor-repairs-what-stops-other-seats,one-folder-deployed-and-evolved,one-steward-per-checkout-on-its-own-root,receipt-writer-follows-the-worktree-it-runs-in,round-proof-feeds-the-next-brief,seat-successor-continues-a-handoff-without-a-human,seats-spend-tokens-in-bounded-sessions,shipped-folder-holds-everything-the-runtime-needs,spend-fence-reports-tokens-per-model-and-cause,stop-hook-never-forces-an-empty-turn,testing-surfaces-declare-their-mirror reason=priority-order subject=one-folder-deployed-and-evolved from=1:5 to=1:6 requested-sequence=5
Integrity: sha256=8f921697016b392527838e9ac56cec753829b2138d823472c6f621b60a67e63c
