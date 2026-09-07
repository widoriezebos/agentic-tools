Working Mode: design
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal metasystem-stop-verb)
Date: 2026-09-08

# Design round 3: one conflict your section 18 leaves open (goal metasystem-stop-verb)

You wrote section 18 and folded its critique. The build implemented it
and the acceptance is green except one scenario, which exposes a
conflict between section 18.1 and section 9's refusal table that the
build cannot resolve without choosing for you.

# The evidence, from the orchestrator's run

The scenario corrupts a job record belonging to another machine, then
runs `metasystem arm`. Arm never reaches its survivor probe. The
human-terminal gate classifies the caller first, that classifier reads
job records, and the corrupt record makes classification refuse. Arm
prints:

    metasystem arm: the caller's ancestry could not be read: caller
    classification refused: job record corrupt or unidentified:
    arm-refuses-remote.json.
    at an agent-free terminal, run: metasystem arm --repo <checkout>

The scenario judges that refusal unactionable and fails.

Both sections are being obeyed at once, which is the problem:

- Section 9's table has a row for classification failure: the sentence
  "the caller's ancestry could not be read: <error>" with the terminal
  form as its second line. That is exactly what printed.
- Section 18.1 says a refusal must name a way forward, must never fall
  back to the arm command when repeating arm cannot succeed, and must
  name the file and reason when a file failed. Standing at a terminal
  does not fix a corrupt record, so the printed second line cannot
  succeed.

# What to decide

Amend section 18 to settle what any of the three verbs says when the
caller classifier fails for a reason that is NOT about the caller: a
record it reads is corrupt, unreadable or unidentifiable. Say what the
person sees, which command clears it, and whether the classifier's own
failure should distinguish "I cannot read you" from "I cannot read
something else". Keep section 9's row for genuine ancestry failures if
that is your judgment, and say so.

# Constraints, binding

- No new record, verb, barrier or pass. This is a wording and routing
  decision, not a mechanism.
- Do not enlarge the slice or touch the deferred scenarios.
- Touch only the design page.
- This is the LAST design round: after it the decision goes into a build
  brief and anything further is carried as a build finding.

# Constraints

Wall-clock budget: 30 minutes. Return per the schema. Gap rule: stop and
report a gap; never fill it silently.
