Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal metasystem-stop-verb)
Date: 2026-09-08

# Round 23: two scenarios round 22 broke (chain stopverb-build1)

Round 22 built section 18 and its remote-obligation behaviour is
demonstrably right: the orchestrator's run shows arm refusing with
"cannot establish whether job <id> on machine <machine> is terminal
because its record <path> ..." and the stop printing that as its one NOT
STOPPED line with "stop incomplete ... 1 not stopped". That is section
18.1 and 18.2 working.

Two scenarios in the supervision bed now fail, both against section 18's
own text rather than because of it. The packages you were asked to run
stay green.

# Facts, from the orchestrator's runs

- stop-fence: the arming path's stopped line reads
  `component=stopped outcome=standing detail="the metasystem is stopped
  for <checkout> since <time>..."` where the scenario expects
  `detail="since <changedAt>"`. Section 2's reader table specifies that
  line's shape, and section 18 does not change it: 18 governs the
  refusal renderer, the report and status, not the arming path's
  component line.
- arm-refuses-survivor: it fails on its own assertion, "stop did not
  retain one missing remote obligation". Section 18.1 is explicit that
  neither repeating stop nor repeating arm replaces terminal evidence,
  so a missing remote record's obligation is RETAINED across a repeated
  stop. The durable record printed in the evidence does carry the entry,
  so code and scenario disagree about where or when that retention is
  observed.

# Decisions (the orchestrator's; decided, not open)

D98. Restore the arming path's stopped component line to the shape
section 2 specifies. If you believe section 18 requires the fuller
sentence there, STOP and report it as a gap rather than choosing: the
design has had two independent reads and the coordinator will not decide
it in a brief.

D99. Make the implementation and the arm-refuses-survivor scenario agree
with section 18.1 as written, and say in your return which of the two
was wrong. The rule is that the obligation is retained until terminal
evidence exists, not until a stop is repeated.

D100. Nothing else changes. No new record, verb, barrier or pass.

# Verification

The smallest run that can fail on this change: `scripts/agents/go-gate.sh
--fast`; `go test -count=1 -timeout 40m ./internal/stoptransition/
./internal/up/ ./cmd/metasystem/`; and name the supervision scenarios you
expect green. The orchestrator runs that bed. Your return names every
unbuilt slice-1 scenario as a gap, as your last three did.

# Constraints

Wall-clock budget: 60 minutes. Return per the implementer schema with
the cumulative diff boundary listed. Gap rule: stop and report a gap;
never fill it silently.
