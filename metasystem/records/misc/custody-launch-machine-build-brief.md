# custody-launch-machine — implementation-first build brief (RULING 7)

Authorized: Wido, 2026-08-27 (implementation-first exit at the
satellite's first exhaustion, per D81). Timebox: ~10h of codex
passes under MANDATORY code-critique. Sources of truth, in order:
this brief; plans/custody-launch-machine-design.md v3 (frozen);
the round-3 verdict
artifacts/agents/critiques/custody-launch-machine/r3-output.md
(the 11 open findings); the parent map
plans/delegate-job-liveness-design.md (rulings 1-7);
plans/delegate-job-liveness-facts.md (anchors).

THE DISCIPLINE: failing-first fixtures. Every open finding
S1R3-01..11 becomes a NAMED fixture written to FAIL before its
resolving code lands; the implementation resolves each shape
question IN CODE; the code-critique chain (fresh, three-round
budget) plus the fixtures are the arbiter prose review stopped
being. A shape choice the implementer makes is RECORDED in the
final report with its alternatives — the critic attacks the choice,
not its absence.

## Build order (dependency on truth)

1. IDENTITY CORE (Go): platform-exact identity representation
   (darwin microseconds / linux ticks+bootID) persisted in primary
   ownership + custody entries + proc alive + census join (design
   D-C inherited); the ordered R-C verification table INCLUDING the
   first-read-GONE row (S1R3-08); MatchShape adapter shapes + the
   kill-path substring retirement (S1R2-03).
   Fixtures: recycled-pid per platform; every table row; rg-tag
   rejection incl. as group leader.
2. THE CLAIM VERB (Go, `job claim-launch`): total outcome set; the
   v1 fingerprint with a PINNED WIRE ENCODING and golden vectors
   (S1R3-11 — length-prefixed field framing, explicit null/empty,
   UTF-8, the field order as listed in R-B; publish the golden
   digests as fixtures); waiter bound PINNED (S1R3-09): 40 retries
   × 15s (10 minutes aligned to AbandonedSetupGrace), attempt
   counting from the first read; the advisory deadline is
   WAKE-ONLY (S1R3-06) — reconciliation FAILS a reservation only on
   creator abandonment (creator-written liveness breadcrumb absent
   past its own bound) or complete-census proven absence, never on
   deadline + momentary absence.
   Fixtures: every outcome; forward and backward clock steps.
3. SESSION OCCUPANCY (Go + records): the per-session index with a
   crash-safe protocol (S1R3-03): a single PER-SESSION lock file
   owns index+record publication ordering — creation: index
   entry FIRST (busy), then record (a crash leaves busy-without-
   record, healed by a bounded per-session recovery that re-reads
   only that session's records); terminal: record first, index
   second (a crash leaves stale-busy, healed the same way).
   Self-healing NEVER scans the registry under the cap lock
   (S1R3-04): the fallback scan runs OFF-lock, then revalidates
   under the lock with a bounded generation check (index
   generation counter).
   Fixtures: both crash orders; concurrent distinct-opid claimants;
   the slow-registry case holding acquisition under 10s.
4. GROUP CUSTODY (Go + shell): custody closure over EVERY custody
   entry (S1R3-01): death examines the recorded pgid AND every
   custody-added process identity (cross-group members counted);
   the kill path winds down each custody entry's group, not only
   the primary pgid. Pre-fork marker (shell): written before fork
   NAMING pid+start identity of the supervisor and intended pgid;
   removal ordered AFTER custody-add's record write (crash leaves
   marker+custody — reconciled by marker sweep against the
   record); the recycled-pgid-with-standing-marker hole (S1R3-02)
   closes because the marker names the SUPERVISOR's identity: a
   dead supervisor + marker + no tagged survivor = the marker's
   fork can never have happened after supervisor death was proven
   → bounded by the supervisor's own recorded identity, the
   marker expires with the supervisor, not with wall time.
   Adoption census (S1R3-05): the adoption scanner uses a result
   shape that PRESERVES unknown observations (per-process
   indeterminate list); complete-census absence requires zero
   unknowns.
   Fixtures: supervisor-death-before-fork with pgid reuse;
   marker/custody crash orders; cross-group custody death and
   kill; unreadable-argv survivor blocks absence.
5. PRODUCT/PROGRESS SIGNALS (Go + shell): outputStream recording;
   the events-file-only rule; watermark never-reset law recorded
   for S2; containment RE-RESOLVED AT EVERY SCAN (S1R3-07 —
   launch-time containment is necessary, scan-time containment is
   the binding check; a root whose resolution drifted outside or
   into the exclusion set demotes at that scan, labeled);
   demotion granularity PINNED (S1R3-10): PER-ROOT — an outside
   root demotes itself only; the mixed-root fixture proves one
   contained fresh root still evidences liveness.
   Fixtures: artificial-log-touch regression; supervisor-append
   exclusion; mixed roots; symlink drift mid-job.
6. CALL-SITE MIGRATION (shell): both existing reservation call
   sites adopt the lock order; critique-round.sh routes through
   the custodial channel (coordinated with the severity cutover
   task — build ONCE); the nonce-tag lands in instanceTag
   everywhere the tag is minted.
   Fixtures: dispatch + follow-up honor the order; a custodial
   critique round writes a real job record.

RULES: plain-English application-language comments (never finding
ids in code); bash 3.2; no python; decisions in Go, composition in
shell; no commits — working-tree bytes for the coordinator; the
execution guard and existing cap/kill owners are never bypassed.
VERIFY per pass: gofmt -l (empty), go vet ./..., focused go test
over touched packages -count=1, the new fixture suites, and
scripts/agents/dispatch-fixtures.sh when dispatch.sh changes.
