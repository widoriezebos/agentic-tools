Working Mode: implement
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal up-kills-runner-before-first-tick)
Date: 2026-09-04

# Review brief: the steward-runner fix (chain ukr-build1)

FINDING IDS: chain-unique, UKR-01, ... never F-n.

Round budget: 1 focused round, then at most one correction and its
re-review (tier 2). R-60-m1's rule: material only if it changes what
gets built and names the artifact.

Threat model: a dead runner no longer replaced (the watchdog's reason
to exist); a stuck attempt kept forever; the patience bound unbounded
or unvalidated; `up` declaring a runner verified that is not alive;
the batched ledger read returning different bytes than the per-file
read for any goal file (renames, symlinks, empty files); the goal
package's tests weakened. Out: the steward's tick cost beyond the
ledger read; taste.

Scope: the computed diff of the implementer job under review.
Contract: metasystem/plans/up-kills-runner-build-brief.md and the goal
record metasystem/plans/goals/up-kills-runner-before-first-tick.md.

# Mandate

1. Alive-and-attempting runners survive `up` and the watcher cycle;
   dead runners and attempts past the patience bound are replaced;
   the fixtures prove all three.
2. `steward.tick-patience-sec` is validated and the default derives
   from the last measured tick as the brief says.
3. ReadCommitGoals returns byte-identical maps to the old
   implementation for the current ledger (name the test that proves
   it) and the goal package's test time fell (the return's numbers).
4. Nothing outside the named owners changed.

If nothing material remains, say so; that closes the chain and the fix
lands.

# Constraints

Wall-clock budget: 30 minutes. Return per the code-critic schema with
the reviewedTree from validate conformance --stage review for job
ukr-build1.

# Gap Rule

stop and report a gap; never fill it silently.
