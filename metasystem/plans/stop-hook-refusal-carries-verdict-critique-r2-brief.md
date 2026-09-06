Working Mode: implement
Orchestrator Identity: m1c (lineage main-1788680061-17829-64951c, dispatch delegate under goal stop-hook-refusal-carries-verdict, tier 3, hazard DESIGN-BEARING, code critique round two of chain shr-build1)
Date: 2026-09-06

# Review brief: stop-hook-refusal-carries-verdict, round two

Round budget: three focused rounds for the goal's tier-3 box; this is
round two, after one fold. The orchestrator adjudicates every finding; you edit nothing.

Threat model: one seat and its delegates on one machine, no adversaries.
In scope: a Stop hook that blocks without the verdict display, blocks
twice for the same open work or the same failure cause, lets a turn end
on an all-clear it cannot vouch for, loses the HEALTH line from the
system message, replaces a worker's published answer with the deadline
sentence, breaks on the stock Mac bash 3.2, or weakens a fixture
assertion beyond the two order tests the brief names. Out of scope:
hostile inputs, the turn verdict verb's decision logic, and the
steward.

Scope: the computed diff of chain shr-build1 after follow-up round three
(job shr-build1-r3) against its base commit 2f8fbc51. Round three folded
your round-one findings per
metasystem/plans/stop-hook-refusal-carries-verdict-fold-r3-brief.md
(landed 857d3648): F-1 (the first-occurrence failed-stop block now
carries the extras before the check-in tail in its system message), F-2
(the verb comment), F-3 (the fourth holder-state name); F-4 and F-5 were
noted and changed nothing. The orchestrator's dispositions are in
metasystem/records/misc/stop-hook-refusal-carries-verdict-critique-r1-dispositions.md. Round one
implemented metasystem/plans/stop-hook-refusal-carries-verdict-brief.md
(landed 2f8fbc51); round two folded four findings the orchestrator made
by running the scenario to its end seat-side, per
metasystem/plans/stop-hook-refusal-carries-verdict-fold-r2-brief.md
(landed a7c1c6ac). Both bind. The computed diff for the whole
chain after round two is
metasystem/artifacts/agents/shr-build1/rounds/3/diff.patch and its
reviewed tree is 485474af31a3eff1c4244ec1375950569bf4bedb; carry that
hash into your return exactly.

# Goal

Say whether the change ships a defect, violates its brief, or damages
what certifies it. Apply the materiality criterion verbatim:

> Would the change ship a defect, violate its brief, or damage what certifies it?

# What to attack

0. The fold: F-1's first-occurrence branch passes the extras, a newline
   only when extras exist, then the check-in tail as the system
   message, and nothing else in compose_failed_stop moved; F-2's
   comment; F-3's fourth name. Confirm rounds one and two are otherwise
   byte-identical to what you reviewed.

1. The reason order in metasystem/internal/report/stopblock.go: detail
   first, one blank line, guidance; guidance alone for an empty detail;
   `StopRefusal` inherits it; the two named tests in
   metasystem/internal/report/stopblock_test.go pin exactly the new
   law and nothing else in that file loosened.
2. The Stop body's order in metasystem/scripts/agents/supervision-hook.sh:
   the verdict runs before any block except the advisor exit; the
   failed-stop path composes display, blank line, failure sentence,
   with the check-in tail (HEALTH line) as the system message; the
   repeated-cause case defers to the verdict; the degraded path is
   unchanged; the evidence trail writes (one dated line per emitted
   response) are where they were. Trace a Stop that both fails arming
   and finds open work through its first and second firing, and a Stop
   whose verdict verb fails while a stop failure is recorded.
3. The deadline parent in the same file: a complete published response
   (block with string reason and a system message containing "HEALTH ",
   or a system-message-only allow) is emitted verbatim at the deadline;
   the deadline sentence appears only when no such response exists;
   the worker is still stopped and the temporary directory cleaned in
   both cases; a partial write (truncated JSON) is not mistaken for a
   published answer.
4. The fixture changes in metasystem/scripts/agents/supervision-fixtures.sh:
   the pid line is the exec form (a plain command substitution reports a
   transient pid on bash 3.2); the goal-open call for the fixture goal
   carries no tier flag and is otherwise byte-identical; the isolation
   loop skips exactly the three named holder-state files by name and
   still fails any other pidless file; no other assertion moved and no
   other bash-4 construct entered the scripts directory.
5. The fail-closed marker: in metasystem/internal/goal/turnverdict.go
   the Verdict gains one additive field, failClosed with omitempty, set
   only by the fail-closed constructor; the new test
   TestFailClosedVerdictCarriesItsMarker in
   metasystem/internal/goal/turnverdict_test.go pins it and no existing
   test changed; the hook reads the field and renders a fail-closed
   verdict through its fixed "turn-verdict unavailable:" message with
   the "stop verdict unavailable" trail line, and treats an absent field
   as false. Check that a readable verdict over a missing ledger (ledger
   status "degraded" but not fail-closed) still composes normally.
6. Conformance: every changed path is inside the union of the two
   briefs' May-touch lists; no other test file changed; nothing under
   plans.

# Evidence you may run

If your runtime gives you a shell, from the reviewed worktree root (the
metasystem directory):

- `go test -count=1 ./internal/report`
- `go test -count=1 -run 'TestFailClosedVerdictCarriesItsMarker' ./internal/goal`
- `bash -n ./scripts/agents/supervision-hook.sh` and `bash -n ./scripts/agents/supervision-fixtures.sh`
- `grep -n 'BASHPID' ./scripts/agents/supervision-fixtures.sh`
- `bash --version` (to confirm which bash the checks ran under)

The supervision fixture suite is not yours to run; the orchestrator ran
it seat-side on the reviewed worktree under the stock bash 3.2 (all
seven scenarios green) and runs it again on the landed tree.
If you have no shell, say so in gaps and review by reading.

# Expected Return

The code-critic role's version-3 JSON with every required property,
the reviewed tree hash carried exactly, findings only, each with a
stable id, severity, a boolean material value, a plain-English claim
and evidence marked ran, read or inferred; one rigor row per material
finding. A round with zero material findings is the closure candidate;
say so plainly and list what you checked.

# Gap Rule

stop and report a gap; never fill it silently.
