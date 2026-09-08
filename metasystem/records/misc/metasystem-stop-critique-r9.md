# metasystem stop: sixth code critique (chain stopverb-build1, round 25)

Critic stopverb-crit10 (code-critic, Opus 5) on reviewed tree 68e82f80bf4275ee7729868b0a3d136d28a5be45. Five material findings: two high, three medium. This was dispatched as the read that closes the chain, so it read the whole slice rather than the fold alone, and three of its five findings are in the code the last two folds added.

## SVC10-01 - high - material=True

CLAIM: The round-25 run-and-suite fold mislabels any monitored run whose record became terminal before the run step reached it. The line helper appends the words "concluded <status> (suite stopped)" whenever the stop outcome carries a terminal status, on the ordinary path as well as the suite path, and the ordinary path then appends its own verdict on top with no separator. Nothing links the phrase to the record of which suites were actually stopped. The result is a line outside the report grammar and a false claim about what stop did. It is reachable in ordinary operation, not only in a race: the repository watcher concludes every non-terminal run on each of its passes and is not stopped until step six.

EVIDENCE: Proven by execution. In a scratch copy of the module the critic created a valid run record, concluded it through the store's own failed-launch path standing in for the watcher, left the cached inventory record saying the run was running under wrapped custody, left the suite-stop join empty, and called the run family's Stop. The composed line was "run r1 running wrapped: concluded launch-failed (suite stopped)already gone" with an empty suite-stop join. Code: internal/stoptransition/families.go, where runLine appends the suite phrase on any terminal outcome status and the run family's Stop appends its own verdict after it; internal/run/stop.go, where stopHeld returns already-gone with the record's terminal status; cmd/metasystem/supervise_component.go, whose run pass concludes non-terminal runs on every watcher pass. The phrase appears zero times in the round-24 diff and twice in the round-25 diff.

## SVC10-02 - medium - material=True

CLAIM: The line printed for a wrapped run that stop ended with a signal always says the run concluded ended-unknown, even when the code concluded it red or green from the run's own exit sidecar. The stop outcome carries the status the record had before the stop, and the line builder hardcodes the words. When a wrapped run's workload has already written its sidecar and only the wrapper group is alive, the conclusion path preserves that evidence and terminalises the record red or green; the report then tells the person the opposite, so stop's report disagrees with the durable record it just wrote.

EVIDENCE: internal/run/stop.go: outcomeFor captures the record's pre-stop status, and concludeStoppedHeld lets the assessed verdict stand when the record is draining or carries sidecar evidence. internal/stoptransition/families.go: the run family's Stop composes the successful wrapped case as the literal text "concluded ended-unknown (" plus the stop reason, and never re-reads the record after the conclusion.

## SVC10-03 - high - material=True

CLAIM: The new arm verb can open the stop fence with no caller classification at all. When the caller passes a temporary human word together with a review date, the block that calls the human-terminal classifier is skipped, and the transition then opens the fence and starts every ring. The word is validated for shape only, not authenticated against anything, so any process that can run the engine binary in a stopped checkout can clear a human stop by inventing a word and a date. The long form guards this: steward arm refuses under a closed fence before it reaches its own temporary-word branch. The new verb removes that guard for the one command whose purpose is to open the fence.

EVIDENCE: cmd/metasystem/process_verbs.go, function runProcessArm: the classifier call sits inside a branch taken only when the temporary word is empty, and the transition's Arm follows unconditionally. internal/stoptransition/transition.go: Arm acquires the lock, opens the fence and then runs the arming function. internal/humanauthority/authority.go: the temporary word pair is checked for a non-blank word and a parsable date and nothing else. cmd/metasystem/steward_verbs.go: the steward arm verb calls its stopped-checkout refusal before the temporary-word branch, so the long form is protected where the new verb is not.

## SVC10-04 - medium - material=True

CLAIM: The round-25 fix that made a supervision shutdown bookkeeping failure visible has two exits that drop it again. The failures are held on the family and attached to exactly one anchored component outcome. Two earlier returns in the same function, taken when the anchored component has no matching outcome in the shutdown report, return before the attachment and without setting the flag that records the failures as reported. On those paths a failure to append the escalated registry row or to release the owner lock is printed nowhere, saved in no survivor entry and cannot influence the exit code, which is the defect the fifth code read raised and this fold was written to close. Section 17.5 states the rule as absolute, whatever the source of the error.

EVIDENCE: internal/stoptransition/families.go, the supervision family's Stop: when the outcome lookup misses, the function either returns the stored shutdown error or returns a not-stopped line, in both cases before the loop that turns the held failures into auxiliary outcomes and before the reported flag is set. A later pass finds no live owner once it is gone, so nothing restores them. internal/supervise/arming.go records a failure and returns a non-nil error for both the registry-append and lock-release cases. The existing test covers only the path where the anchored outcome is present.

## SVC10-05 - medium - material=True

CLAIM: The up path does not describe the phase of the stop it reports, although section 18.3 requires it to. Under any closed fence, up reports the component as stopped with the detail carrying the time of the change alone; only the remedy varies with the phase. Health does what 18.3 asks; up does not. A person whose stop left survivors behind, or whose session hook runs up for them, is told the checkout is stopped and given no hint that the last stop never finished or how many things it could not end.

EVIDENCE: internal/up/up.go, function Run: the stopped result is built with the detail "since " plus the record's change time for every phase, and only the remedy comes from the phase-sensitive command helper. internal/steward/health.go selects its prefix from the phase. internal/up/stopfence_test.go asserts the detail is exactly "since now" for the incomplete and unfinished phases, so the deviation is pinned by a test rather than accidental.

## Coordinator's reading (m1b, 2026-09-08)

All five accepted; round 26 folds them under decisions D108 to D112.
Nothing here is a design question, so no design round follows.

Three of the five are in code the last two folds added, and two of those
three are regressions of the very rule the fold was written to satisfy:
SVC10-01 is the run-and-suite fix printing a false line, and SVC10-04 is
the bookkeeping-visibility fix keeping two exits that drop the failure.
That is the strongest argument in this chain for the rule the fourth and
fifth reads both left behind: a fold that adds a promise adds its
scenario in the same round, or the next read finds the promise unkept.

SVC10-03 is the one that touches Wido's own requirement rather than the
design's internal consistency. He asked that only a human arm clear a
stop; a temporary word plus a date cleared it. Relayed authority has
been retired in fact since the authority horizon passed on 2026-09-06
(ruling R-82-m1b), which is why no bed caught this, and the retirement
is what makes the fix small: the classifier gate becomes unconditional,
exactly as the steward long form already has it.
