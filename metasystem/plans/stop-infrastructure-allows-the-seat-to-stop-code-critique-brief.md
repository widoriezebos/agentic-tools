Working Mode: implement
Orchestrator Identity: m1b (lineage main-1789191336-90295-e4b24b, dispatch delegate under goal stop-infrastructure-allows-the-seat-to-stop)
Date: 2026-09-12

# Review brief: independent read of the member build (chain implementer-b18d717d6d38a443b0cb339f, round 4)

FINDING IDS: chain-unique, SIC-01, SIC-02, ... never F-n.

Why this review exists: chain completion under Ruling O requires a
distinct independent-critique job. Rounds 2 to 4 of the chain were read
three times by the Opus lane through the harness (five, six, then one
material finding; all folded by round 4 except the last, an unset-variable
guard on the hook's deadline path, which the seat folds itself under
R-103 with three bed legs converted to the new contract). This job reads
round 4's tree in a fresh session and is the record.

Round budget: 1 focused round. A finding is material only if it changes
what gets built and names the artifact.

Specification: metasystem/plans/stop-infrastructure-allows-the-seat-to-stop-design.md
(sections 2, 3 and 5). Goal record:
metasystem/plans/goals/stop-infrastructure-allows-the-seat-to-stop.md (five
DONE clauses). The round's diff is the job's computed diff, eleven files:
report.go and up_test.go under cmd/metasystem; sessionstop.go,
turnverdict.go, turnverdict_idle_test.go and turnverdict_test.go under
internal/goal; stopblock.go and stopblock_test.go under internal/report;
supervision-hook.sh and supervision-hook-fixtures.sh under scripts/agents;
claude-code-hooks.json under scripts/enforcement.

# Mandate

1. Every decided block survives every persistence failure (verdict state,
   status write, session-stop marker read and consume, lock acquisition);
   no infrastructure condition ever spends an idle count or invents an
   authorization; the idle-with-backlog refusal, counter, digest and
   three-refusals handoff are unchanged.
2. The infrastructure class allows on every occurrence with the notice
   and one hook-log line per condition in every outcome; the seat class
   keeps today's first-occurrence block; the stop-refusal record schema is
   unchanged.
3. The hook emits exactly one JSON response on every path; the launcher
   fallback is valid JSON and never a block; a verdict from an engine
   older than this landing is still read (tolerant defaults).
4. The tests pin behavior through the real entry points and would fail
   against the base tree.
5. Nothing outside the eleven files; nothing the design excludes.

If nothing material remains, say so; that closes the chain.

# Gap Rule

stop and report a gap; never fill it silently.
