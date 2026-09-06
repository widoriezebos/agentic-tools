# hook-enrollment-per-checkout

- State: approved
- Intent: Guard enrollment is per-use, not per-machine: ensureGuardEnrolled installs and probe-verifies the pre-commit hook chain only when a goal verb runs (cmd/metasystem/goal.go:139, goalsync_mutations.go:27), so a checkout that only commits - the paper seat, .git/hooks empty - has every commit fence inert; two-bars r3 (TB-R1-02) separately notes adopt.sh skips installation when a hook exists. DONE means: every enrolled machine's checkout has the guard chain present and probe-verified at a boundary that does not depend on ledger usage, and an uncovered checkout is loudly visible.
- Origin: main
- Next step: INTENT: move guard-chain assurance to a per-checkout boundary (metasystem up's preflight, adoption, or steward health - the probe machinery exists and is reusable). CONSTRAINTS: reuse ensureGuardEnrolled's probe-and-propagate proof verbatim (exit-42 propagation, no forged acks); the shared-hooksPath refusal stays. FREEDOMS: which boundary hosts the check, and whether an uncovered checkout heals automatically or only alarms. TEST SHAPE: a checkout with empty hooks gains a verified chain at the chosen boundary; a hook chain that swallows the probe exit refuses enrollment and surfaces as unhealthy.
- OpenedAt: 2026-09-01T13:20:54Z
- Revision: 3
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=240 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T06:53:37Z revision=3 opid=A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f authority=proven digest=7049224358a419b7c685d69ab415ee397ad66fd204bd820ad71e966466b6f5c7

History:
- 2026-09-01T13:20:54Z R8B8A0WRNGXFJ4X8V88HCKBEG5-m0-c5dbf036 open actor=human:Wido targets=hook-enrollment-per-checkout
- 2026-09-01T20:26:51Z TFZG2ETZMMM9YKWH2BX5JB06GG-m0b-6638932d set-budget actor=m0b+main-1788250419-3170380-8a1fb3 targets=hook-enrollment-per-checkout
- 2026-09-06T06:53:37Z A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f approve actor=human:Wido targets=hook-enrollment-per-checkout reason=sweep
Integrity: sha256=1a893c2a84836f980ed3f84c5e9a379b7a2c9703e069989c08b037fa7bdcd44d
