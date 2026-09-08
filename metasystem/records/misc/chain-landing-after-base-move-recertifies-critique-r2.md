# Chain landing after a base move: closing build critique (chain clbm-build2, round 1)

Critic clbm-crit2 (code-critic, Opus 5) on reviewed tree 419c81107e56145776f4d6f69773b2b6334943a7. Three material findings, none high, and two notes. The question the brief put first was whether anything unreviewed can reach main through the new path. The answer is no, and the critic says so inside two of the three findings without being asked to soften them.

## CLC-01 - medium - material=True

CLAIM: The recertified merge-conformance path skips a check every ordinary merge performs: that each declared runtime instruction file carries the behavior class in the path-class manifest. mergeStage loads the manifest and refuses otherwise; its new sibling mergeRecertified never loads the manifest at all. The consequence is not that unreviewed code reaches main, because the chain's content is still bound to the mechanically merged tree. It is that a landing taken through the proof path passes a version of the merge gate missing one of its checks, so a misclassified instruction file would stop an ordinary merge and not a recertified one.

EVIDENCE: internal/validate/conformance.go lines 460-468 load the manifest and fail the merge unless each declared runtime instruction file resolves to the behavior class. mergeRecertified at lines 512-545 goes from proof verification straight to the worktree snapshot comparisons, then the critic check and the authorization, and never calls the manifest loader. The waiver branch is absent too. The critic read both functions end to end and reports this as the only check present in one and missing from the other.

## CLC-02 - medium - material=True

CLAIM: The design named one canary as the arbiter of the overlap decision and required each overlap case to show both offending base ranges. The canary does not test that: its assertion checks that the two range starts are not negative, and a range start can never be negative, because the field is an int whose zero value is zero and every value reaching it comes from an unsigned parse of ASCII digits. So an implementation that raised the typed overlap refusal while leaving both ranges empty would satisfy the test. The production code does populate them correctly, so nothing is broken at run time; what is missing is the proof.

EVIDENCE: internal/gittree/disjoint_merge_test.go lines 86-89 fail only if the error does not convert, its kind is not overlap, its path is not the fixture file, or either range start is below zero. HunkRange declares Start as an int and parseCount builds every value with an unsigned 31-bit parse, so both range clauses are always false. The refusal site at lines 557-560 does set both ranges from the offending hunks.

## CLC-03 - low - material=True

CLAIM: On the recertified path, when the candidate receipt check fails, scripts/agents/land.sh picks between two refusal codes by reading a variable that is not always set. The only assignment sits inside a block entered when the chain name matches a lowercase-letters-digits-hyphens pattern, so any other chain name leaves it undefined, and under set -u the read aborts bash with an unbound-variable message before the park verb runs. The result is an ending a reader cannot tell from a crash: no parked line, no park-failed line, no durable park record, which is exactly what the headless failure contract exists to prevent. It needs a malformed chain identifier and such a landing would be refused later on other grounds, so nothing passes; it removes the durable evidence on one failure path.

EVIDENCE: land.sh line 513 reads the gate-width variable with no default inside the branch that runs when the receipt check returns nonzero; the only assignment is at line 176 inside the pattern-guarded block. The critic reproduced the shell behaviour under set -uo pipefail: bash printed an unbound-variable message and exited 1 at the read. The fix is to read the variable with an empty default.

## CLC-04 - low - material=False

CLAIM: The design says blob comparisons run outside attribute-bearing worktrees with no repository-local diff configuration loaded, and the implementation creates its comparison directory with the standard temporary-directory call, which honours TMPDIR and so does not guarantee the directory sits outside a Git repository.

## CLC-05 - low - material=False

CLAIM: The design says the park write's ten-second ceiling is shortened by any remaining enclosing ceiling, and the park takes a plain ten-second budget with no channel through which an enclosing limit could shorten it. No current caller passes such a limit, so the clause has no effect today.

## Coordinator's reading (m1b, 2026-09-08)

All three material findings accepted; one fold round closes them. Both
notes are recorded and not actioned, and CLC-04 is worth a sentence
because it is the more interesting of the two: an operator whose TMPDIR
sits inside a Git repository could give the comparison directory
attributes the design wanted excluded. The critic graded it non-material
because it found no behaviour that changes as a result, and the fold
brief asks for the cheap defensive fix rather than leaving it, since it
is one call.

The brief's first question was whether anything unreviewed can reach
main through this path. Two of the three findings answer it unprompted:
CLC-01 states that the chain's content is still bound to the merged
tree, and CLC-03 states that nothing passes because such a landing is
refused later on other grounds. Neither weakens the guarantee that
every line in a landed candidate was either reviewed or came from main.

CLC-02 is the fourth canary in this program that could not fail for the
right reason, and it is the sharpest instance yet, because the vacuity
is arithmetic rather than a matter of judgment: the assertion asks
whether an unsigned-parsed integer is negative. The first three were
caught by reads. So was this one. That is four for four, which is
reassuring about the reads and damning about the writing, and it is why
the fold brief states the rule as a check rather than as advice: an
assertion whose clauses can never be false is not a test, and the way to
know is to make the production code wrong on purpose and watch the test
notice.

This is fold cycle one on the build. Six attempts of ten are spent; the
fold is the seventh and the closing read the eighth.
