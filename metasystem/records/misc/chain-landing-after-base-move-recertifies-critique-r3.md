# Chain landing after a base move: closing read of the final round (chain clbm-build2, round 2)

Critic clbm-crit3 (code-critic, Opus 5) on reviewed tree ba85f6ff703086b1cc62880532e950cab8010e18. ZERO material findings, three notes. This is the read that closes the chain.

## The question this chain existed to answer

The brief put it first: can anything unreviewed reach main through the new path? CLD-03 answers it directly and is worth quoting in substance rather than summarising. The one route that exists is the pre-existing extra-path lane, and this change inherits it without widening it. A candidate may carry files outside the certified path list; those go to the same register-carriage and path-class checks every chain landing already uses, and when they fail the verdict is an observation-mode refusal, exactly as on the ordinary chain path today. The canary deliberately asserts that observation-mode outcome so the recertified path matches rather than diverges. Every path inside the certified list is bound entry for entry, including mode, blob and absence, so the lane cannot alter a certified file.

So the guarantee that survives the change is the one the design set out to keep: every line in a landed candidate was either reviewed or came from main.

## CLD-01 - low - material=False

CLAIM: The refactor that turned the review-boundary check into one shared helper re-reads and re-parses every job record once per job record, so its cost grows as the square of the number of job records. On this repository's 99 records one call takes 624 milliseconds against 6 for a single pass. It computes the same answer, so nothing is decided differently; ordinary review, recertification preparation and every recertified landing observation each pay the growing cost. Preparation runs under a five-minute ceiling, so the cost sits inside a bounded budget. The fix, if ever actioned, is to hoist the single record-set load out of the loop in internal/validate/conformance.go.

## CLD-02 - low - material=False

CLAIM: On the recertified path, land.sh decides whether to record a durable park after a failed commit step by scanning that step's output for a refusal-code token. Every refusal the design enumerates does emit one, so the stated contract holds. Outside it are failures the design does not enumerate, such as a Git hook rejecting the commit or the wrapper's own rollback when the recorded tree differs from the proved tree; those exit through the ordinary failed-step path, which prints the failing step and a nonzero exit but writes no park record. A reader can still tell that ending from a stall by the output, but not from a durable artefact.

## CLD-03 - low - material=False

CLAIM: See above. Recorded as the answer to the brief's first question rather than as a defect.

## Gaps the critic declared

It could not examine whether the caller-classification machinery behind the park verb is itself sound, because the park verb passes its own parent process id to the classifier. It did not run the receipt-bound battery or the dispatch and goal-CLI beds, and makes no claim about them. It did not run the process-group ownership probe, which its sandbox cannot observe. It ran the two landing canaries and the merge proof against an export of the reviewed tree, and made its mutation experiments in a separate scratch copy that it restored and verified. It also noticed the narrator digest log modified in the working tree, disclaimed writing it, and correctly excluded it from the reviewed tree.

## Coordinator's reading (m1b, 2026-09-08)

Zero material. The chain closes and lands.

All three notes are recorded and not actioned, and two deserve a
sentence because they will outlive this landing.

CLD-01 is a real cost that grows: quadratic record parsing, 624
milliseconds today at 99 job records. It decides nothing differently, so
it is not a defect, but the fleet direction is more nodes producing more
job records, and the fix named in the note is one hoist out of a loop.
It belongs on the queue rather than in this chain, where it would be an
unreviewed change on a path the read just cleared.

CLD-02 is the honest residue of the park contract: the durable park
record covers the failures the design enumerated, and a Git hook
rejection or a wrapper rollback still exits through the ordinary
failed-step path with output but no artefact. That is strictly better
than before this change, when no failure produced a park record at all,
and it is the kind of thing a headless node's operator would want closed
eventually.

The read also did what the brief asked about this chain's unusual
construction: it looked for the marks of the hand carry after the cap
and found none, and it confirmed the seed marker appears nowhere in the
reviewed tree.

Counts for the record: two design cycles, one build round capped and
continued, one fold, three reads. Eight attempts of ten. No cycle was
opened on the coordinator's own authority beyond the planned ladder.
