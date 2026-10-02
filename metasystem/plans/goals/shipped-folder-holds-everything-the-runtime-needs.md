# shipped-folder-holds-everything-the-runtime-needs

- State: queued
- Risk: severity=2 novelty=1 exposure=3 accumulation=2 basis="An adopted repo breaks at runtime when a dependency is missing (severity 2); moving files and adding an audit (novelty 1); every adopter (exposure 3); every agent turn reads these files (accumulation 2)"
- Tier: 2
- Intent: Everything the MetaSystem runtime and its participating agents need lives in the shipped installation folder, so an adopted repository works (Wido 2026-10-02: 'we really need to move everything required by metasystem runtime and agents participating in that into the folder that is deployed/shipped. Otherwise the metasystem will break in an adopted repo'). Outside it there is only project state, which the runtime locates through its state root, never by a guessed relative path. Agent-facing texts (AGENTS.md, wow.md, skills, brief templates, hook and verb messages) cite installation files unambiguously from any working directory, and an audit refuses a reference to a runtime dependency outside the shipped set.
- Origin: main
- Next step: Inventory every reference from engine code and agent-facing texts that resolves outside the shipped set or ambiguously; classify each as shipped, project state or misplaced; then design the moves and the audit
- OpenedAt: 2026-10-02T14:15:33Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-10-02T14:15:33Z 57S6AMPSN96W61BQX4VJRFA2PC-m1e-9c612d71 open actor=human:Wido targets=shipped-folder-holds-everything-the-runtime-needs
Integrity: sha256=93332f1509aa6e3951366d2dfd19b35a322476b403c2535c0b8580fe2f236707
