# bed-child-death-reported-as-pass

- State: claimed
- Risk: severity=3 novelty=1 exposure=3 accumulation=2 basis="severity 3: the supervision bed reports a scenario as passed after its child died, so a landing's real-runner proof can be hollow - it was, for the engine re-arm landing ea8c3ead7; novelty 1: one helper declaration and one trap discipline, both in shell that exists; exposure 3: every bed run on every macOS host (stock bash 3.2 is the only bash here) and every landing that cites the bed; accumulation 2: every vacuous pass compounds into trust in proofs that never ran"
- Tier: 3
- Intent: On this host's only bash (3.2.57) the supervision bed reports scenarios as passed after their child process died. Two facts, both verified 2026-09-06 on main at 805912fc3. (1) install_rearm_engine in scripts/agents/supervision-fixtures.sh declares 'local built=$1 enrolled=$2 staged=$enrolled.replacement' in one statement; bash 3.2 expands $enrolled before that statement assigns it, so under set -u the child dies with 'line 659: enrolled: unbound variable' and the engine at the enrolled path is never replaced - found by m1c. (2) The bed still printed 'scenario passed' for rearm-rebuild, rearm-launch-fails and rearm-provenance, because the child's EXIT trap (cleanup, line 575) runs after the death and on bash 3.2 the script's exit status then becomes the trap's last command status, 0: a probe script with the same shape exits 1 without a trap and 0 with one. So every child that dies by an expansion error under set -u is reported passed; deaths by exit 1 or a failed command still propagate (rearm-provenance's redirection failure was reported red earlier the same day). Consequence: the landing receipt of engine-rebuild-rearms-itself cited three bed scenarios that never exercised a replaced engine; the Go tests with the fake runner stand, the real-runner proof does not. The bed must fail a scenario whose child died, and the helper must assign in the order bash 3.2 evaluates.
- Origin: main
- Next step: MECHANICAL, tier-1 lane or one small chain, ANY FREE SEAT, AHEAD OF OTHER WORK (m1c offers to take it once its chain shr-build1 lands, within the hour): (a) split the helper's declaration (assign staged on its own line after enrolled) and audit supervision-fixtures.sh and the other fixture scripts for the same 'local a=$1 b=$a' shape; BASHPID is NOT in scope - m1c's chain shr-build1 already replaces the stop-hook-monitor line with stop_main_pid=$(exec sh -c 'echo $PPID') (a plain substitution reports a transient pid on 3.2); (b) the child's cleanup trap captures $? first and ends with exit "$status" so a death keeps its status - and a bed self-test proves it: a scenario that dies on an unbound variable under the trap is reported failed. The vacuous class is exactly that shape: m1c observed that a scenario's own assertion failures (exit 1) propagate as rc=1, so ordinary failures are not hidden; (c) receipt: the bed on this host with rearm-rebuild, rearm-launch-fails and rearm-provenance genuinely green (the log shows no unbound line and the identity assertions ran), cited on rearm-remedies-and-adoption-notes as the real-runner proof the re-arm landing lacked. Until it lands, no bed pass is evidence unless the scenario's log is read.
- OpenedAt: 2026-09-06T11:28:08Z
- Revision: 5
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
Integrity: sha256=9cfe2dc3aebf1595d927e146c11b56d58aae2abecbc5c538b1c8eb253315246d
