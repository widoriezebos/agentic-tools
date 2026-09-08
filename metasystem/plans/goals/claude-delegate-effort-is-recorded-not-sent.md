# claude-delegate-effort-is-recorded-not-sent

- State: queued
- Priority: 3
- Sequence: 48
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="severity 2: a hazard class records a critique effort the runtime never received, so a review can run below the effort the record claims; nothing unsafe is permitted; novelty 1: one settings key the adapter already writes a file for; exposure 2: every claude-runtime delegate on every host; accumulation 1: fixed"
- Tier: 2
- Intent: The dispatcher records xhigh reasoning effort for the builder and the independent critique of every DESIGN-BEARING and DESTRUCTIVE-REACH class (internal/dispatch/hazard.go, mirrored in scripts/agents/role-packets.json and stamped into each round composition.json), and admits the claude runtime only because runtime.claude.maximal-models in metasystem.conf lists the model. The claude adapter (scripts/agents/adapters/claude.sh, internal/adapter/claude.go) sends no effort at all: the command line carries model, schema and a per-job settings file with no effortLevel key, so the delegate runs at whatever the operator saved in ~/.claude/settings.json. Found 2026-09-06 when Wido changed his Claude Code default for Fable from xhigh to high: every Fable delegate silently followed, while the metasystem kept recording xhigh. The record and the execution disagree, and the operator has no metasystem knob for the effort his delegates run at. DONE means the claude adapter writes the effort the composition records into the per-job settings file (effortLevel), the launch refuses when the runtime cannot honour it instead of proving it by a model list, the round record shows the effort that ran, and Wido sets delegate effort in metasystem.conf per role or hazard class rather than through his terminal default; a fixture proves the settings file carries the effort and the record matches it.
- Origin: main
- Next step: Unclaimed; awaits approval and Wido word on the wanted effort per role (his 2026-09-06 change: Fable high). Small build: one Sol round, one Fable code critique.
- OpenedAt: 2026-09-06T19:52:23Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-06T19:52:23Z TVM1W4FSAWWA7W3Z14MHEEXV10-m1d-62183579 open actor=m1d+main-1788683763-71870-f7f607 targets=claude-delegate-effort-is-recorded-not-sent
- 2026-09-08T16:01:55Z SGS5X4MNTTBJ3JQ6Z9MYN1MGZ4-m1-7cd0bd60 set-priority actor=human:Wido targets=claude-delegate-effort-is-recorded-not-sent reason=priority-order subject=claude-delegate-effort-is-recorded-not-sent from=unranked to=3:48 requested-sequence=48
Integrity: sha256=1a55b168816b3029cba95c237d487a807684fce1bc821c6a1447d310e28af0b1
