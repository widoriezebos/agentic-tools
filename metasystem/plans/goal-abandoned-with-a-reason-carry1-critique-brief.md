Working Mode: design
Orchestrator Identity: m1b (lineage main-1789191336-90295-e4b24b, dispatch delegate under goal goal-abandoned-with-a-reason)
Date: 2026-09-13

# Review brief: the carry of chain gawr-build1 onto today's trunk

FINDING IDS: GAWR-C1-01 onward.

Why this review exists: chain gawr-build1 (slices 1 and 2 of the goal:
abandon with a reason, the engine floor, reopen from abandoned and the
abandoned carry) closed its round 5 clean after three Opus reads on
2026-09-10 and never landed. Round 6 carried it from 938dbe7c onto trunk
f389cf1d, which had changed five of its files. The specification is
metasystem/plans/goal-abandoned-with-a-reason-design.md (revision 4).
The dispatcher gives you the round's computed diff against trunk.

Codex reported these joins:
- metasystem/cmd/metasystem/goal.go: trunk's bounded summary, history
  opt-in and JSON form kept, with the abandoned count, archive bucket and
  show lookup added.
- metasystem/cmd/metasystem/goalsync_mutations.go: trunk's landing carry
  and park/unpark proof routing kept, abandoned reopen inserted before
  ordinary reopen, and the abandoned carry routed only when its `--to`
  flag is present, because trunk later added its own `goal carry` for
  landings. Trunk's `Carry` stays the landing API; the certified API is
  now `CarryAbandoned`.
- metasystem/internal/goal/file.go: trunk's Landing, Episode, park
  blocker, authority generation and landing-carry fields combined with
  Abandoned, the stop identifier and the carried successor; a landing
  carry is not read as an abandoned carry.
- metasystem/internal/goal/root.go: trunk's power-of-attorney duplicate
  validation and the engine-floor validation both kept.
- metasystem/scripts/agents/goal-cli-fixtures.sh: every trunk scenario and
  the abandoned scenario kept, the listing checks moved to trunk's
  bounded summary and `--json`.

Round budget: 1 focused round. A finding is material only if the code or
tests must change before this lands, and it names the artifact.

# Mandate

1. Does each join keep both trunk's behaviour and the certified behaviour,
   with nothing lost from either side and nothing added beyond them?
2. The shared `goal carry` name: can a landing carry ever route to the
   abandoned carry or the reverse, and can a history row of one be read as
   the other?
3. Did trunk's changes since 938dbe7c invalidate any assumption of the
   certified change outside the five conflicted files (for example the
   goal record's authority fields or the human-verb proof routing)?
4. Nothing else.

# Expected Return

The code-critic return schema, findings sorted by materiality, each with
file:line evidence and its rigor row. Every path in your return starts
with `metasystem/`.

# Gap Rule

stop and report a gap; never fill it silently.
