Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal metasystem-stop-verb)
Date: 2026-09-07

# Round 22: build section 18 (chain stopverb-build1)

The fifth code read found three material failures in what a person is
told. This time the coordinator did not decide them: a design round on
another model wrote section 18 of
metasystem/plans/metasystem-stop-verb-design.md, a design critic on a
third model found three material holes in it, and the design round
folded those. Section 18 as landed is the decision, in six parts. Read
it whole before you touch anything; it is cumulative on sections 1 to 17
and wins where it contradicts them.

Your job is to implement it exactly, not to reinterpret it. Where you
believe section 18 is wrong or underdetermined, STOP and report it as a
gap rather than choosing: it has had two independent reads and a third
choice made silently in code would undo that.

# Decisions

D94. Implement section 18 in full, all six parts: the unprobeable
survivor's named way forward and the explicit remote-job cases, one
final line per thing under its identity rule, status distinguishing a
closed fence from a completed stop, and the three amendments the design
critique forced, covering missing evidence about a remote job, the
publication-failure case for the equality invariant, and status's
behaviour when a family cannot be read.

D95. No new record, verb, barrier or pass. Section 18 states that it
introduces none; if implementing it appears to require one, that is a
gap to report, not a thing to add.

D96. The acceptance surface stays as it is: no scenario renamed or
removed. Proofs go in the beds section 10 already assigns and in package
tests beside the existing ones.

D97. Your return names every unbuilt slice-1 scenario as a gap, as
rounds 20 and 21 correctly did.

# Verification

The smallest run that can fail on this change, reported at evidence
level ran: `scripts/agents/go-gate.sh --fast`; `go test -count=1
-timeout 40m ./internal/stoptransition/ ./internal/stopfence/
./internal/up/ ./internal/supervise/ ./cmd/metasystem/`; and name the
supervision-bed scenarios you expect to stay green. The orchestrator
runs the beds outside your sandbox. Do not run the whole matrix or all
five beds.

# Constraints

Wall-clock budget: 120 minutes. Return per the implementer schema with
the cumulative diff boundary listed. Gap rule: stop and report a gap;
never fill it silently.
