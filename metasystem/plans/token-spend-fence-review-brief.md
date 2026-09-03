Working Mode: design
Orchestrator Identity: m1b+main-1788333346-60696-6a3256 (dispatch delegate under goal token-spend-fence)
Date: 2026-09-03

# Goal

The one review of metasystem/plans/token-spend-fence-design.md (landed,
in your workspace), the design for the token spend fence in alert
mode: spend in tokens and money per goal, machine and day from the
runtimes' usage records, ceilings in metasystem.conf, one health line,
an alert on crossing, nothing refused. Tier 3 under R-54-m1: this is
the one review before the fold, then a closing review; the ladder has
no other rounds.

# Inputs

The design was authored by job fence-design-1 (Claude Fable 5.1,
design mode) against main df799e42. Its six reported gaps and the
orchestrator's answers, so you do not re-raise them as findings:

- Seat-to-goal attribution is not measurable today; seat spend goes to
  goal "seat" with an explicit unattributed line, and claim-interval
  attribution is fold obligation O-1. ACCEPTED as the honest bound.
- No price rows are invented; the roster's prices are entered in
  metasystem.conf from the providers' lists (fold obligation: the seat
  proposes the rows with their source, Wido's word lands them).
  ACCEPTED.
- The day ceiling is per machine-day; a fleet roll-up needs a shared
  spend ledger and is later work. ACCEPTED.
- Codex-runtime seats print "unmeasured". ACCEPTED.
- The claude adapter drops cache_creation tokens; adapters are a
  non-goal here and the seat tokens that follow-up separately.
  ACCEPTED.
- 299 lines, one deliberately long health-line example. ACCEPTED.

# Review brief

Round budget (R-60-m1, review depth is a risk-based budget): ONE
round now, one closing review after the fold. A finding is material
only if it changes what gets built AND names the artifact it changes
(a file, a key, a line format, a test). At the budget, the agreed parts
build and any disputed point becomes a named test obligation, never a
raise for another round.

Threat model: accidents in measurement — spend counted twice or not at
all, money invented for an unpriced model, an unavailable usage folded
into a total as zero, a seat's spend silently absent, a ceiling that
never alerts or alerts every tick, a step-1 change that refuses
something. Adversarial records are OUT of scope.

Scope: the reader over job records and round usage files, the
seat-spend line, the price table and its keys, the ceiling keys and
their defaults, the health line, the alert episode, the fixture bed.
OUT: step 2's refusal; the reserved-minute pool; adapter usage writers.

Attack in particular: (1) the mapping of a job to (goal, machine, day)
and the follow-up rounds of one chain; (2) double counting between the
job record's usage object and the round usage files; (3) the honesty
rule for unavailable usage; (4) native cost versus derived cost when
both exist; (5) whether the defaults follow from the stated arithmetic;
(6) the alert's once-per-crossing rule; (7) that the named tests
discriminate each of these.

Return format: the design-critic schema; stable identifiers
TSF-R1-<name>; each finding marked material only if it meets the rule
above and names the artifact; a clean verdict is
`verdictMaterialCount: 0` with observations recorded.

# Constraints

Wall-clock budget: 20 minutes. Do not rewrite the design.

# Gap Rule

stop and report a gap; never fill it silently.
