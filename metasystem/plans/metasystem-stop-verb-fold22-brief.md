Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal metasystem-stop-verb)
Date: 2026-09-08

# Round 24: build the classifier decision (chain stopverb-build1)

Round 23 fixed the arming line, and the last failing scenario then
failed differently: with a corrupt job record, arm never reaches its
survivor probe because the caller classifier reads job records and
refuses, so the person was told their ancestry could not be read and
sent to a terminal, which cannot fix a file.

That was a design conflict, not your error, and the design lane has
settled it. The design page on main now distinguishes a classification
failure caused by DATA the classifier reads from a genuine ancestry
failure: the first names the full path, the precise reason and the
repair prerequisite, and sends the person back to the command they ran;
the second keeps the terminal remedy of section 9. Status keeps no
caller gate. No mechanism is added and no durable state changes on a
refusal.

Read the amended section 18 whole before you start. Implement it as
written; if you believe it is underdetermined, STOP and report a gap
rather than choosing.

# Decisions (the orchestrator's; decided, not open)

D101. Implement the amended section 18's classifier rule across all
three verbs and their shared gate, and make the arm-refuses-survivor
scenario assert it.

D102. No new record, verb, barrier or pass; no change to any deferral.

D103. Your return names every unbuilt slice-1 scenario as a gap, as
your last four did.

# Verification

The smallest run that can fail on this: `scripts/agents/go-gate.sh
--fast`; `go test -count=1 -timeout 40m ./internal/stoptransition/
./internal/up/ ./cmd/metasystem/`; and name the supervision scenarios
you expect green, arm-refuses-survivor included. The orchestrator runs
that bed.

# Constraints

Wall-clock budget: 60 minutes. Return per the implementer schema with
the cumulative diff boundary listed. Gap rule: stop and report a gap;
never fill it silently.
