Working Mode: design
Orchestrator Identity: m1+main-1788333680-2840-7f79f4 (dispatch delegate under goal job-record-birth-token)
Date: 2026-09-02

# Goal

Round-1 critique of metasystem/plans/job-record-birth-token-design.md
(revision 1, landed, in your worktree), the design note for goal
job-record-birth-token (read metasystem/plans/goals/job-record-birth-token.md
first, and metasystem/records/misc/alert-channel-spike-verdicts.md for the
spike that proved the token necessary). Two landed designs read the field
this note names: metasystem/plans/failed-job-attention-design.md (revision
3, the dedup digest) and metasystem/plans/alert-channel-design.md (the
retention pin). The note declares no gaps; its self-grade names the writer
enumeration as its weakest claim and its reject condition.

# Your mandate

1. ATTACK THE WRITER ENUMERATION (section 3), the design's own reject
   condition: find every path that writes a job record under
   artifacts/agents/jobs, including any that replaces a record wholesale
   from caller bytes or by rename, in packages the note did not open
   (search Go for the jobs directory path and the record lifecycle
   functions in metasystem/internal and metasystem/cmd, and shell for
   redirections into the jobs directory in metasystem/scripts). The note
   found the steward's rename-based chain close
   (metasystem/internal/steward/reap.go around 143-169) only by reading;
   name any sibling it missed and whether it would drop the token.
2. VERIFY THE THREE CREATE PATHS and the minting: the legacy create and
   indexed create in metasystem/internal/dispatch/record.go and the
   claim-launch reservation write in metasystem/internal/dispatch/claim.go
   around line 522; that the mint happens under the record lock in each;
   that the setup handshake (record-setup) carries the husk's value
   forward and refuses a differing source; and that the immutable-fields
   map refusal reuses the existing typed error.
3. VERIFY THE FIELD SHAPE: the UTC-second plus 32-hex grammar and its
   pinned vector; whether 16 random bytes are drawn from a source every
   supported platform provides; whether same-second reuse on the same
   identifier is collision-free in the fixtures as written.
4. VERIFY THE PRE-CONTRACT RULE word for word against the
   failed-job-attention design's digest rule, and judge the contradiction
   the note flags in the alert channel design's fallback clauses (section
   10): is it real, and what must the channel design change?
5. ATTACK THE CALLER TABLE (section 5): every incarnation-comparison
   caller with its switch, stay or out-of-scope disposition, including
   the chain resolution's disclosed residual after a collected root is
   reused; name any caller missing.
6. ATTACK THE FIXTURES AND THE SIZE (sections 6 and 7): deterministic, no
   sleeps, the record-protocol shell fixture update named correctly, the
   eight-file boundary complete for the build, and the four-hour box
   credible.
7. NEW FINDINGS only if material and grounded.

Findings quote the disagreeing text or code. Your sandbox is read-only:
verify by reading, do not run go. Zero material findings is an
acceptable, closing answer if the reading supports it.

# Constraints

Wall-clock budget: 40 minutes. Return per the design-critic schema.

# Gap Rule

stop and report a gap; never fill it silently.
