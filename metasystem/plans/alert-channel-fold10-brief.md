Working Mode: design
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal alert-escalation-channel)
Date: 2026-09-01

# Goal

Revision 10 of metasystem/plans/alert-channel-design.md: fold all five
material findings of records/misc/alert-channel-critique-r9.md (in your
worktree; full return durable at
artifacts/agents/design-critic-a27506cb4736a12e5dcfc31c/rounds/1/return.json).

# The mandate, by id

- AC9-JOB-ID-ABA-001 (critical): close the job-identifier-reuse hole. The
  pin key and the episode digest must carry a birth-unique component (the
  job record's creation timestamp or equivalent birth token the record
  already persists), so a reused identifier can never satisfy another
  incarnation's pin or dedup row. Extend the interleaving table with the
  reuse row and prove it.
- AC9-RETENTION-DIGEST-ADDRESSING-001: define the episode-addressing
  operation the collector calls — one stated primitive (exists-by-digest)
  with its exact key derivation, so the four interleaving rows rest on an
  operation the store actually specifies.
- AC9-SCAN-BOUNDEDNESS-001: bound the read set for real — an explicit
  contract: what the scan opens per tick (the enumerated live/terminal sets
  it may touch, with their size owners), never every episode file; if an
  index or cursor file is required, specify it as slice-1 mechanism.
- AC9-STOP-SUPPRESSION-MERGE-001: make the pre-send recheck and the
  suppression rule one non-contradictory operation — state which check
  cancels the send, at what point, holding what.
- AC9-ANSWER-FOLLOWUP-ACTION-001: the delegate producer advertises the
  resume command only for cases where resume is the true remedy; enumerate
  the failure cases and their advertised actions (resume, follow-up
  dispatch, or none-with-reason).

Then the consistency pass over changed rules and touched sections, named
pairs in the status line; self-grade; the reject condition stays a third
implementer gap-stop.

# Constraints

Wall-clock budget: 35 minutes. No changes beyond the five folds and the
pass. Wido's standing words untouchable.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly
metasystem/plans/alert-channel-design.md.

# Gap Rule

stop and report a gap; never fill it silently.
