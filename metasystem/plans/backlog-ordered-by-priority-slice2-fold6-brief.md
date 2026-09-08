Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal backlog-ordered-by-priority)
Date: 2026-09-08

# Fold brief: round 7 of chain bolnext-build1, two sentences of guidance

The fourth read (job bolnext-crit4) returned one material finding,
medium, and two notes. Everything else in round 6 is confirmed good and
verified by the orchestrator outside the sandbox: the goal-command bed
exits 0, the three canaries pass, the fast gate exits 0, and the goal,
channel, steward and command packages all pass in full.

This round changes documentation only. Write no Go.

## D1 - BOQ-01 - the seat must be told what to do when it is refused on authority

The frontier now applies every gate the claim verb applies to a goal
record. But the claim verb's first act, before it examines any goal, is
an unconditional refusal on a checkout declared the fleet brain or whose
brain declaration is unreadable, and the frontier never consults that
state. So on such a checkout `goal next --machine <nick> --fetch`
returns a ready goal and `goal claim` refuses it, every time.

That was harmless when seats read records. It is not harmless now,
because this chain rewrote the contract into a standing instruction to
take work through the verb whenever the seat is free.

The design already specifies the remedy, in the same paragraph as the
rest of the seat guidance: stop a retry cycle on a transport, authority
or quota error and report that cause. Two of that paragraph's three
requirements are in both seat-guidance documents. This one is in
neither.

Add it, in each document's own voice, so a seat refused on authority
stops rather than fetching and selecting again, and reports the cause it
was given. Quote or paraphrase the design's own wording rather than
inventing a rule.

Do not change the frontier to consult brain state. That would be a
behaviour change the page does not ask for, and the page's chosen
remedy is the guidance sentence.

## Not in this round

BOQ-02 and BOQ-03 are recorded and not actioned. BOQ-02 is a correction
to the orchestrator's own brief rather than a defect in your work: an
over-norm goal was admitted to the ready list before this chain, so
narrowing the frontier also narrowed the idle refusal. Leave it.
BOQ-03 is the re-rank attention event, recorded twice already.

## Verification

- `bash -n` is not applicable; this is prose. Instead, quote in your
  return the exact sentence you added to each document, so the
  orchestrator can compare it against the design paragraph.
- Confirm in the return that no Go file changed, and that the two other
  documents which describe the turn-end read remain consistent.
- `scripts/agents/go-gate.sh --fast`, once, because the refusal register
  and the word audit run there and both read documentation.

Your return must carry its own job id.

Gap rule: stop and report a gap; never fill it silently. If the design
paragraph does not actually say what this brief claims it says, stop and
quote what it does say.
