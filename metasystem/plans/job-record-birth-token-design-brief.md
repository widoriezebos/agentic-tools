Working Mode: design
Orchestrator Identity: m1+main-1788333680-2840-7f79f4 (dispatch delegate under goal job-record-birth-token)
Date: 2026-09-02

# Goal

Author the design note for goal job-record-birth-token (read
metasystem/plans/goals/job-record-birth-token.md first: every job record
incarnation carries a mandatory, immutable, machine-minted birth token,
minted under the record lock at create, ignoring any caller-supplied
value, joining the immutable fields). The rule is already implied by the
executable spike in metasystem/records/misc/alert-channel-spike-verdicts.md
(2026-09-01): no shipped field qualifies, because createdAt is neither
mandatory nor immutable through the real writers, startedAt and claimEpoch
are optional and caller-supplied, and inode identity changes on every
atomic rewrite. Two landed designs now wait on this token: the alert
channel's retention pin (metasystem/plans/alert-channel-design.md) and
failed-job-attention's dedup digest (metasystem/plans/failed-job-attention-design.md,
revision 3, section on the reused-identity fold), which blocks its build
on this goal. Write exactly one NEW file named
job-record-birth-token-design.md in the metasystem plans directory. Every
claim about the tree cites file and line.

# Workspace

The delegate worktree the dispatcher created for this job. Read anything;
write only that one new design file.

# What the design must settle

1. THE FIELD. Its JSON name (both waiting designs read it by the name this
   goal lands; choose it and state it once), its value shape (a timestamp
   plus a nonce generation, since a second-precision timestamp alone
   collides on same-second identifier reuse; say the exact width and
   encoding), and where it is minted: the create path in
   metasystem/internal/dispatch/record.go (read RecordCreate around lines
   222-272 and the immutable-fields map around 60-75) under the record
   lock, ignoring any caller-supplied value.
2. IMMUTABILITY. The field joins immutableFields; specify what RecordCAS
   and every other writer (record-setup, record-cas, the patch verbs
   under metasystem/cmd/metasystem adapter *, the reap and protocol-error
   paths) do when a write carries a different or missing value: refuse
   with a typed error, never rewrite. Enumerate every writer with file
   and line.
3. PRE-CONTRACT RECORDS. Records born before this lands have no token.
   Specify the rule: the field stays absent forever on such records
   (never back-filled, so no digest that hashes it ever changes), and
   every reader treats absence as "pre-contract", never as an error.
   Verify this against failed-job-attention's pre-contract digest rule so
   the two designs agree word for word.
4. EVERY INCARNATION-COMPARISON CALLER. The goal record says "every
   incarnation-comparison caller runs": find every place in the tree that
   decides whether two job records are the same incarnation (the alert
   channel design's pin, the reap facts in
   metasystem/internal/dispatch/reapfacts.go, chain and follow-up
   resolution in metasystem/scripts/agents/dispatch.sh and its Go owners,
   the steward's failed-job candidacy) and state for each whether it
   switches to the token, stays as it is, or is out of scope, with the
   reason.
5. FIXTURES. Deterministic tests, no sleeps: same-second identifier reuse
   yields two distinct tokens; a caller-supplied token is ignored at
   create; a CAS carrying a changed token is refused; a pre-contract
   record reads as absent and never gains a token; the record schema
   fixtures (metasystem/scripts/agents/record-protocol-fixtures.sh,
   return-schema-fixtures.sh) still pass or are updated by name.
6. SIZE. This is a small mechanical item (4h box); state the build's
   estimate against precedent and the one slice's diff boundary.

Self-grade per the house rule: confidence, weakest claim, reject condition.

# Constraints

Wall-clock budget: 30 minutes. Design only; no builds, no benchmarks
(R-31). Edit nothing but the design file.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly the one design file
named under Goal.

# Gap Rule

stop and report a gap; never fill it silently.
