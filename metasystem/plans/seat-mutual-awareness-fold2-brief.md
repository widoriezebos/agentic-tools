Working Mode: design
Orchestrator Identity: m1d+main-1788683763-71870-f7f607 (dispatch delegate under goal seat-mutual-awareness)
Date: 2026-09-06

# Goal

Revision 2 of the design for goal seat-mutual-awareness. Revision 1
(metasystem/plans/seat-mutual-awareness-design.md, commit 6948392b) went
to one design review (job sma-crit1-20260906); its register is
metasystem/records/misc/seat-mutual-awareness-critique-r1.md: ten
material findings, every one grounded in landed code, every one
accepted by the seat. Answer all ten in the design itself and append a
dispositions section at the end of the design file, one table with one
row per finding (finding id, disposition, reasoning and evidence, what
changed in the design), the way
metasystem/plans/fleet-channel-gateway-design.md closes its rounds.
Your revision-1 brief
(metasystem/plans/seat-mutual-awareness-design-brief.md) still binds;
Wido's inbound rule and the no-authority-for-seats rule are not up for
redesign. Where a finding forces a choice, make it in the design with
its reason; do not push it back to the seat.

# Workspace

The delegate worktree the dispatcher created for this job. Read
anything; write exactly one file, the existing design under
metasystem/plans (revision 2 in place, the revision-1 text rewritten
where a finding changes it, not appended as a delta).

# The ten findings and the constraint on each answer

- SMA-C-01 (seat words reach the human through the Stop hook's
  digest). Constraint: no seat-authored words on any surface that
  reaches the human (the digest, the health line, the status post, the
  stop message). The digest may name the asker, the goal in words, the
  age and the question id and the verb that shows the words; a seat
  reads the words through a read verb (name it). State it as a
  structural fact with the consumer list, and add the human-visible
  surfaces to the refusal and residual lists.
- SMA-C-02 (an old checkout can land a malformed seat record before
  the fence exists). Constraint: name the fleet-wide order: what lands
  first (the path-class row, the guard regexp, the validator) and what
  the first writer requires of every machine before it writes; say how
  an upgraded validator treats a record an old engine landed (refuse
  the tip, or tolerate and name), and which of the two Wido's law of
  first-commit-wins implies.
- SMA-C-03 (a refused presence publish marks the role unknown, and two
  unknowns alert the human in twenty minutes). Constraint: decide
  whether presence publish failure belongs in the seat-questions role
  at all; if it does, say which counter may alert and which age
  governs; if not, where it goes (the tick's own component record is
  the candidate).
- SMA-C-04 (seat publications cannot be recovered from the journal).
  Constraint: every seat verb's intent carries what recovery needs to
  rebuild the record (the whole record body for ask; qid, outcome, text
  and reason for answer and close; machine and engine plus the composed
  body for presence), recovery gains named cases for the three verbs,
  and the outcomes are named.
- SMA-C-05 (no fleet membership; an idle machine and a nonexistent one
  look the same). Constraint: name one durable membership source and
  who writes it (a record a machine writes when its steward is armed or
  when it joins, or the enrollment the fleet-join design already
  proposes; read metasystem/plans/fleet-join-bootstrap-design.md before
  inventing one), define `unreachable` (known, no presence within the
  staleness window) apart from `unknown` (no membership), and say what
  `seat ask` does with each.
- SMA-C-06 (timeouts and late answers). Constraint: the ask carries a
  durable deadline (derived from ifSilent or given), the outcomes after
  it are named (an answer after the deadline is `late`, recorded on the
  record and never binding; the asker's wait past the deadline exits
  with a named status), a missing timeout has a stated default, and a
  target that never runs a steward yields exactly one outcome for the
  asker.
- SMA-C-07 (the validator is not total). Constraint: the refusal table
  enumerates every state and field combination the schema allows and
  refuses the rest by name, including target machine existence.
- SMA-C-08 (load, history growth, legacy transport). Constraint: state
  the accepted load bound in commits per day at the fleet's size, the
  rotation or compaction policy (or who owns it, with the goal named),
  and what the legacy transport mirror does with presence commits.
- SMA-C-09 (no dependency-safe seam returns the lineage to the steward
  package; the lease package imports steward). Constraint: name the
  owner of the read seam without a package cycle, and decide whether
  presence reports the arming lineage or the current lease holder's,
  with the reason.
- SMA-C-10 (the proof matrix cannot certify the highest-risk behavior;
  the estimate understates it). Constraint: add the named scenarios
  (mixed-version fetch and write, recovery for every seat verb, absent
  and old stewards, unreachable versus unknown target, timeout then late
  answer, persistent-unknown alert, the digest path, commit churn at
  fleet scale, legacy transport, the lineage source) to the fixture and
  unit-test lists, and restate the estimate honestly against the box;
  the seat asks Wido for the raise on your number.

Ground every changed claim in file-and-line evidence from the worktree;
keep the self-grade current.

# Constraints

Wall-clock budget: 45 minutes. Do not edit anything but the design
file.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly the one design file.

# Gap Rule

stop and report a gap; never fill it silently.
