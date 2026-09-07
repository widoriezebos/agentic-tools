Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal metasystem-stop-verb)
Date: 2026-09-07

# Round 21: fold the fourth read, and simplify what keeps breaking (chain stopverb-build1)

The fourth read found four material items, one CRITICAL, and proved two
of them by running probe tests rather than by reading. All four live in
the late-arrivals and final-sweep machinery, which has now produced the
worst finding of two consecutive reads. Dispositions are in
records/misc/metasystem-stop-critique-r7.md; the design's new section 17
decides each and simplifies that machinery rather than patching it. Read
section 17 first; it wins over earlier sections.

The critical one, in the critic's own probe: a family whose live item
first appears in the third post-fence inventory yields three
inventories, ZERO stops, exit 0, and a report reading "nothing is
running" followed by "stopped <checkout>", while that item is alive and
owned throughout. That is the one thing this verb may never do.

# Decisions (the orchestrator's; decided, not open)

D88. Per 17.1: delete the key-prefix filter in the final sweep. Every
item still live goes through its family's stop path, exactly as in the
late-arrivals passes; the untracked family already reports its own
processes without signalling them, so no caller needs a filter. Assert
what the critic proved: an item first seen in the final inventory is
stopped, reported, and cannot coexist with a "stopped" footer or exit 0.

D89. Per 17.2: a survivor entry for an unreadable family carries the
file that failed and the parse error; the stop line and arm's refusal
both print them; arm says it cannot probe that survivor rather than
implying it is alive; and the entry clears when a stop reads that family
successfully. Test the three-cycle case the critic ran: stop, arm, stop,
arm, with the family readable again on the last.

D90. Per 17.3: the repair never lowers the generation. Establish the
highest generation visible in any durable source, write one above it,
and say the counter was recovered rather than read. Test a truncated
record.

D91. Per 17.4: every iteration of the late-arrivals loop runs the family
stop path, the last included; nothing is reported or recorded as a
survivor of a pass that never signalled it.

D92. Per 17.5: an observer or hook error after the fence closes becomes
a printed line, never a returned error. Third variant of one fault;
close the class.

D93. Nothing else. The deferred scenarios stay deferred and your return
names them as gaps, as round 20's correctly did.

# Verification

The smallest run that can fail on this change, reported at evidence
level ran: `scripts/agents/go-gate.sh --fast`; `go test -count=1
-timeout 40m ./internal/stoptransition/ ./internal/stopfence/
./internal/up/ ./internal/supervise/ ./cmd/metasystem/`; and name the
supervision-bed scenarios you expect to remain green. The orchestrator
runs that bed outside your sandbox. Do not run the whole matrix or all
five beds: the change is confined to the transition and the fence.

# Constraints

Wall-clock budget: 120 minutes. Return per the implementer schema with
the cumulative diff boundary listed. Gap rule: stop and report a gap;
never fill it silently.
