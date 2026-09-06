# bed-child-death-reported-as-pass

- State: claimed
- Risk: severity=3 novelty=1 exposure=3 accumulation=2 basis="severity 3: the supervision bed reports a scenario as passed after its child died, so a landing's real-runner proof can be hollow - it was, for the engine re-arm landing ea8c3ead7; novelty 1: one helper declaration and one trap discipline, both in shell that exists; exposure 3: every bed run on every macOS host (stock bash 3.2 is the only bash here) and every landing that cites the bed; accumulation 2: every vacuous pass compounds into trust in proofs that never ran"
- Tier: 3
- Intent: On this host's only bash (3.2.57) the supervision bed reports scenarios as passed after their child process died. Two facts, both verified 2026-09-06 on main at 805912fc3. (1) install_rearm_engine in scripts/agents/supervision-fixtures.sh declares 'local built=$1 enrolled=$2 staged=$enrolled.replacement' in one statement; bash 3.2 expands $enrolled before that statement assigns it, so under set -u the child dies with 'line 659: enrolled: unbound variable' and the engine at the enrolled path is never replaced - found by m1c. (2) The bed still printed 'scenario passed' for rearm-rebuild, rearm-launch-fails and rearm-provenance, because the child's EXIT trap (cleanup, line 575) runs after the death and on bash 3.2 the script's exit status then becomes the trap's last command status, 0: a probe script with the same shape exits 1 without a trap and 0 with one. So every child that dies by an expansion error under set -u is reported passed; deaths by exit 1 or a failed command still propagate (rearm-provenance's redirection failure was reported red earlier the same day). Consequence: the landing receipt of engine-rebuild-rearms-itself cited three bed scenarios that never exercised a replaced engine; the Go tests with the fake runner stand, the real-runner proof does not. The bed must fail a scenario whose child died, and the helper must assign in the order bash 3.2 evaluates.
- Origin: main
- Next step: Chain bcd-build1 (codex, DESIGN-BEARING, started 12:3xZ) builds from plans/bed-child-death-reported-as-pass-brief.md (cb649601). Reproduced on m1c before briefing: the re-arm scenario child run directly the way the bed runs it dies at line 662 (enrolled: unbound variable) and exits 0; a plain probe with the same helper and trap shape exits 1, so the swallow is specific to the bed's cleanup body. Three parts: the helper assigns staged after enrolled; cleanup ends with exit "$status" (parent cleanup checked too); a first scenario bed-death-self-test whose child dies on purpose and passes only when reported nonzero. Then: one Fable critique, close, land, run the bed seat-side under stock bash and read the three re-arm logs for the receipt on rearm-remedies-and-adoption-notes.
- OpenedAt: 2026-09-06T11:28:08Z
- Revision: 6
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T12:16:37Z revision=3 opid=0F5B2YJ9DP5VE6V56AKCCV44TM-m1-7cd0bd60 authority=proven digest=b248439b971de9038fcd85937521762667019bf8d5297ed6f15f463c50c6a20f
- Sliced: machine=m1c lineage=main-1788680061-17829-64951c revision=4 at=2026-09-06T12:26:49Z
- Claimed: machine=m1c lineage=main-1788680061-17829-64951c at=2026-09-06T12:22:31Z revision=4 accountingRevision=4
- StopCapability: generation=4 revision=4 machine=m1c claimEpoch=1 fenceEpoch=0

History:
- 2026-09-06T11:28:08Z YY893AYMBJC9Y753VAX0CRMSPF-m1-a4f8999f open actor=m1+main-1788594343-3833-fb64b9 targets=bed-child-death-reported-as-pass
- 2026-09-06T11:28:40Z 33QFFMG3C7PGED62XAWT1EH8XE-m1-a4f8999f edit actor=m1+main-1788594343-3833-fb64b9 targets=bed-child-death-reported-as-pass
- 2026-09-06T12:16:37Z 0F5B2YJ9DP5VE6V56AKCCV44TM-m1-7cd0bd60 approve actor=human:Wido targets=bed-child-death-reported-as-pass
- 2026-09-06T12:22:31Z 6TNT5HN2RHBWWM75H7X1MJB1XY-m1c-7cd0bd60 claim actor=m1c+main-1788680061-17829-64951c targets=bed-child-death-reported-as-pass
- 2026-09-06T12:26:49Z PAGS32SDVATPKJZJKYFBERBHY6-m1c-7cd0bd60 slice-start actor=m1c+main-1788680061-17829-64951c targets=bed-child-death-reported-as-pass
- 2026-09-06T12:27:02Z JT0YWJD40VMGXS39N4P59E5NTV-m1c-7cd0bd60 edit actor=m1c+main-1788680061-17829-64951c targets=bed-child-death-reported-as-pass
Integrity: sha256=d2c051015ffea04ceda735b68acfbe8d821d503cba7f2bfef34f94949001f701
