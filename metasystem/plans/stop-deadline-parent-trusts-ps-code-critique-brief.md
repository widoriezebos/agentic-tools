Working Mode: implement
Orchestrator Identity: m1d+main-1788683763-71870-f7f607 (dispatch delegate under goal stop-deadline-parent-trusts-ps)
Date: 2026-09-06

# Review brief: the Stop deadline parent's liveness test (chain sdp-build1-20260906)

FINDING IDS: chain-unique, SDP-01, SDP-02, ... never F-n.

Round budget: 1 focused round, then at most one correction and its
re-review (tier 3). R-60-m1's rule: material only if it changes what
gets built and names the artifact.

Threat model: a hung worker that the parent still never refuses (any
path that leaves the deadline loop while the worker lives); a worker
signalled without its command line verified (the kill path widening);
a zombie or a reaped pid that kill -0 misreads, leaving the parent
looping to the deadline where today it would return the verdict at
once; the ps-shim scenario passing for a reason other than the new
test (the shim not on the parent's PATH, the assertion satisfied by the
old path); a suite that can hang (no bound on the new scenario). Out:
the hook's verdict wording; taste.

Scope: the computed diff of the implementer job under review.
Contract: metasystem/plans/stop-deadline-parent-trusts-ps-build-brief.md
and the goal record metasystem/plans/goals/stop-deadline-parent-trusts-ps.md.

# Mandate

1. deadline_running in metasystem/scripts/agents/supervision-hook.sh
   no longer reads ps; both loops that wait on the worker use it; an
   empty or failing ps answer cannot end the deadline early.
2. Every kill of the worker or the resolver still sits behind the
   command-line check; no new signal path.
3. The verdict path is unchanged when the worker finishes normally: the
   parent leaves the loop as soon as the worker exits (name what proves
   it; a zombie must not hold the loop to the deadline).
4. The new scenario in metasystem/scripts/agents/supervision-hook-fixtures.sh
   proves the expiry under a ps that prints nothing, within sixty
   seconds, with its own bound; and the existing deadline scenario still
   passes.
5. Nothing outside the two named files changed.

If nothing material remains, say so; that closes the chain and the fix
lands.

# Constraints

Wall-clock budget: 30 minutes. Return per the code-critic schema with
the reviewedTree from validate conformance --stage review for job
sdp-build1-20260906.

# Gap Rule

stop and report a gap; never fill it silently.
