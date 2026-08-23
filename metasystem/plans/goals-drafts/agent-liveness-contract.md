# Draft: a liveness contract for runtime-launched agents

Status: draft — from the silent agent death of 2026-08-22. Needs an
appetite and promotion.

What happened: a comment-sweep agent launched through the runtime's
own delegation tool went silent after completing its file edits —
no error surfaced, no completion event arrived, and the runtime
offered no liveness signal beyond its transcript file, which does
not grow incrementally. The coordinator detected the stall only by
absence (no package-file modifications where work belonged), probed
it, and armed a timed verdict. The probe doubled as the recovery
lever: messaging a stopped agent resumes it from its transcript,
and the agent returned, verified, and reported in full.

The gap: metasystem dispatches get supervision — job records, the
census, the reaper, exit honesty. Agents launched through the
runtime's delegation tool get none of that; their only watcher is
the coordinator's attention, which is exactly the single point the
steward exists to remove.

Scope to design:
1. A heartbeat any coordinator-launched agent must produce (a file
   touch under artifacts on every tool round is enough), so silence
   becomes measurable by the steward, not just by a curious human.
2. Steward coverage: the tick flags a launched agent whose
   heartbeat is older than a named ceiling, same covenant as
   delegated work.
3. Probe-resume as the standard first intervention: a silence past
   the ceiling earns a message before a kill, because resume is
   cheap and loses nothing.
4. The delegation brief rule that made this stall cheap, written
   down as doctrine: work lands on disk incrementally; the final
   act is a report, never the only copy of the result; verification
   runs in bounded chunks, never one unbounded call.

## SEQUENCING (coordinator, 2026-08-23)

This draft WAITS FOR acp-adapter-seam deliberately. Harness-launched
agents cannot be instrumented from outside — the runtime does not run
our code between their tool rounds — so any bespoke heartbeat here
would be coordinator-side bookkeeping around an opaque process. Once
the ACP-shaped seam lands, agent sessions carry typed events and
liveness falls out of the session stream nearly for free. Building
bespoke liveness first would be spend the seam obsoletes within
weeks. Interim protection is the recorded playbook: work-product
mtimes, a resume-capable probe, and a timed verdict watch — exercised
five times tonight, recovered every stall. Promote this draft only
if the seam slips materially.