# lane-one-state U6 (D5): one describer — the words, the intent verb and what a person may do come from one table over the state record

Working Mode: Implement. Goal lane-one-state, design `plans/designs/lane-one-state.md` (read it whole). Builds on U1-U5 (branch build/los1) and on the lane-recovery stack's wording rule (lane-recovery U4: the audit over the producers of status lines, questions and refusals). One commit; never push. Production code ≤ 250 lines (deleted producer code is free). No test may read ../../testing.json or plans/goals; no "witness" or "certification" tests.

## Build
1. `internal/landing/state/describe.go`: ONE table `Words` keyed by (section, word) → {headline phrase, the intent verb a person would use (`landing resume`, `landing give-back`, `landing land-anyway`, `landing deploy`, `landing stop`, `landing prove`, `landing run --goals`, or none), what a person may do now, what the agent does next}; `Describe(r Record) Description` renders from the record only: the status headline ("proving (233 of 234 units, 52 min); the agent waits for the keeper"), the health facts, the steward's rows, each question's first line, each remedy. Every since in the record prints as a local time (R: local time for people) and an observation prints "seen since 21:02 (the batch record carries no running time)".
2. Status, health, the steward's report, the questions (question.go) and the remedy mapper of lane-recovery U4 call `Describe` and delete their own phrase producers; the lane-recovery U4 audit (the wording rule) becomes one walk over the `Words` table: every row names a verb that exists (`intent coverage` list) or `none`, every phrase is a sentence a person can act on, and no producer outside describe.go builds a headline (an AST test over internal/landing and cmd/metasystem for string literals containing the state words running|paused|stopped|proving|resolving|held|returned|landed outside describe.go and its table fails with file:line; allow-list empty).
3. `landing health --words` prints the table (section, word, phrase, verb) so a person can read what every state means and what to do, without the code.

## Tests
Table completeness: every (section, word) the state package can hold has a row (a test enumerates the package's word constants and fails on a missing row). Golden renders for the lane's states of 2026-10-12 (idle; proving; a red proof with incidents; paused by the person with a reason; a barren hold; a stale merge; an engine incident) assert the headline, the verb and the person's option. The producer-free audit. Existing status/health/steward/question tests keep their expectations where the words did not change; where a phrase changed the test's expectation changes with it (listed in the commit message).

## Checks
`go vet ./internal/landing/state ./internal/landing/plain ./internal/landing/lane ./cmd/metasystem`; `go test -p 4 -count=1 -timeout 30m ./internal/landing/state ./internal/landing/plain ./internal/landing/lane ./internal/layering ./internal/testenv`; `go test -count=1 -run 'TestAudit|TestLanding|TestIntentPublicCoverage' ./cmd/metasystem`.

## Do not
No new state words. No producer survives outside describe.go for a headline, a question's first line or a remedy. The landing-agent skill is updated only where it quotes a phrase that changed (keep it under 3000 words).
