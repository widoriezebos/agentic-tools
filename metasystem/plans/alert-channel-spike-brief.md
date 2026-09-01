Working Mode: design
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal alert-escalation-channel)
Date: 2026-09-01

# Goal

DESIGN-EVIDENCE SPIKE, not product code: in your worktree only, prototype the
four disputed mechanisms of alert design revision 10 and report which design
rules survive contact with executable reality. Nothing you write lands; your
RETURN is the product, feeding revision 11 as evidence. The four open
findings (records/misc/alert-channel-critique-r10.md, in your worktree)
are your test targets:

1. Birth token (critical): inspect the shipped job-record writers — is there
   a field that is ALREADY mandatory and immutable for every record
   incarnation (creation timestamp? first-write inode identity? record path
   birth)? Prototype the pin key with the best candidate and write a test
   that reuses a job identifier across two incarnations; report which
   candidate makes the reuse test pass and what contract change (if any)
   the design must demand.
2. Read-set bound: prototype the per-tick scan against a store seeded with
   10,000 episodes and 1,000 job records; measure what the revision-10
   contract actually opens; report the numbers and the smallest index or
   cursor mechanism that makes the tick O(live work) instead of O(history).
3. Stop clear transition: prototype the filename-only stop scan plus the
   clear transition; demonstrate the regression the critic describes and
   the smallest rule that fixes it.
4. Remedy preconditions: execute (dry) the advertised commands against a
   chain-level failed fixture; report which of the four table rows
   advertises a command whose preconditions refuse.

# Constraints

Wall-clock budget: 45 minutes. Go code in the worktree with tests, run
them; report per finding: SURVIVES / NEEDS RULE <stated> / REFUTED. Do not
polish; this is evidence, not product.

# Expected Return

Version-2 implementer JSON; evidence entries are your test runs; whatWasDone
maps each finding id to its verdict and the exact design rule it implies.

# Gap Rule

stop and report a gap; never fill it silently.
