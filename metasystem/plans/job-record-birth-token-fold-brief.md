Working Mode: design
Orchestrator Identity: m1+main-1788333680-2840-7f79f4 (dispatch delegate under goal job-record-birth-token)
Date: 2026-09-02

# Goal

Revision 2 of metasystem/plans/job-record-birth-token-design.md (revision
1, landed; edit it in place, bump the revision line): fold the six
material findings of metasystem/records/misc/job-record-birth-token-critique-r1.md
by id. Every closure is a design change verified against the tree, never
a softened claim; the design's own reject condition (a writer that drops
the token) fired on two findings, so the writer and caller enumerations
are rebuilt, not patched.

# Workspace

The delegate worktree the dispatcher created for this job. Edit exactly one
existing file, the design; nothing else.

# Direction per finding

- BIRTH-R1-FAKE-WRITER-001: the fake host simulator writes tokenless
  completed records after landing and can erase a token; "pre-contract by
  construction" is not a closure. Either the fake writer mints through
  the same create path (so its records carry tokens like every other
  post-contract record) or it is confined to fixture beds and the design
  proves no production reader ever sees its records; choose, cite the
  fake's file and line, and add the fixture.
- BIRTH-R1-JSON-SET-BYPASS-002: the shipped metasystem json set verb can
  rewrite any record field, including the token, and can add one to a
  pre-contract record. Enumerate it as a writer; decide whether it refuses
  the field (a typed refusal for the immutable set) or is declared out of
  the immutability contract with the reason; the no-backfill statement
  must be true for every repository verb or say which verb is exempt.
- BIRTH-R1-CALLER-ENUMERATION-003: metasystem/internal/metrics/data.go
  (around lines 485-542) groups records by job id across local and
  durable copies and would collapse or overwrite lawful reused
  identifiers; search again for every consumer that keys on job id
  across incarnations (metrics, evidence mirroring, the flight recorder,
  the janitor) and give each its disposition; correct the eight-file
  boundary.
- BIRTH-R1-ALERT-FALLBACKS-004: the coordinator amendment to
  metasystem/plans/alert-channel-design.md must name every clause that
  makes a legacy timestamp fallback normative, not only the three; list
  them all with line numbers.
- BIRTH-R1-RANDOM-FIXTURE-005: the same-second reuse fixture must use the
  injected entropy the design promises, with two distinct injected values,
  so the inequality is deterministic; keep a separate note that real
  entropy is used in production.
- BIRTH-R1-REPAIR-FIXTURE-006: the pre-contract fixture's operation order
  never reaches RepairClaim because RecordProtocolError sets status to
  failed first (metasystem/internal/dispatch/record.go around 453-463);
  reorder the fixture so each writer under test actually executes, and
  say what each asserts.

Fold record: add a section mapping each finding id to its fold. Self-grade
per the house rule.

# Constraints

Wall-clock budget: 30 minutes. Design only; edit nothing but the design
file. R-31: no benchmarks.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly the one design file.

# Gap Rule

stop and report a gap; never fill it silently.
