Working Mode: design
Orchestrator Identity: main-1788759014-39092-24fa5c
Date: 2026-09-07

# Goal

Critique the design in `metasystem/plans/host-runtime-setup-design.md` for
the user-approved host-runtime-setup goal. Claude Code must keep working;
Devin and Codex must discover the contract/skills and operate their own
lifecycle hooks in this nested checkout and adopted root installations.

# Workspace and inputs

Read the repository at its supplied workspace. No edits. Read the design,
`metasystem/AGENTS.md`, `metasystem/wow.md`, the design-critique skill, and
the implementation files cited by the design. The design's official-source
facts were fetched by the coordinator; there is no need for network access.
The current runtime registry and registration rows are authoritative.

# Constraints

Round budget: at most three rounds on this chain; failsafe at round 2.
Stop at the first round with no material findings. Bounded findings that
can be fully expressed by fixtures become named implementation obligations.
Aim for a focused review within 15 minutes; the job's enforced reservation
is the configured 120-minute ceiling, not a target duration.

Threat model: trusted repository operators; accidental loss of settings,
wrong runtime identity, duplicate lifecycle execution, wrong paths,
interrupted setup, and false readiness reports. Preserve runtime trust and
permissions. No hostile-root defense or general installer/updater rewrite.

Scope: host entry points, configuration merge, skill registration, hook
selection, and proof of compatibility. Out: unattended re-engagement,
spend accounting, terminal approval policy, private-memory migration,
arbitrary future runtime dialects, and model roster changes. Prefer a
simpler design when it satisfies the same observable contract.

# Expected return and acceptance

Return the current design-critic role schema the dispatcher supplies, with
stable finding ids beginning HOST-D1-, evidence levels, and rigor rows.
Findings only, sorted by severity. Name the concrete artifact/test each
material finding would change. Do not grade word choice or demand a
general shell parser when a narrow emitted-command contract suffices.

> Would an implementer working from this design build something DIFFERENT, or WRONG, because of this finding?

Every path in the return is relative to the repository root, beginning with
`metasystem/`. Evidence commands must be individually replayable. No full
validation or process-owning fixtures are required in the reviewer sandbox.

# Gap rule

Stop and report a genuine missing input; never fill it silently.
