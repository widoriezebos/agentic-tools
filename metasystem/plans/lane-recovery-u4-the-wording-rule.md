# lane-recovery-along-intent U4 (D6): every status headline, question and remedy names an intent verb; the mechanism words move to health

Working Mode: Implement. Goal lane-recovery-along-intent, design `plans/designs/lane-recovery-along-intent.md` (D6). Worktree = this checkout (branch build/lri4, on top of U1-U3 of this goal: resume, give-back, land-anyway, deploy exist). At most 250 production lines. Never commit, rebase or push; never touch memory/receipts.log, plans/, testing.json, go.mod or any metasystem.conf*. Tests never read a live ledger or install: fixtures only.

## Build
1. Every remedy a person reads from the lane — the status headline (status.go, fixHeadline, the barren-hold reason in lane/agent.go, the stop lines in stop.go), every lane question's first line and options (policy_question.go, the keeper's question facts, question.go `LaneQuestionHeadline`), every refusal's `next` (intent_landing*.go) — names exactly one of the six intent verbs as the thing to do: `metasystem landing resume`, `landing give-back G --reason TEXT`, `landing land-anyway G --reason TEXT`, `landing deploy`, `landing stop`, `landing start`; where a wait is the right thing, the line says "the lane proves main (M min); nothing to do". The mechanism words — barren, recorded selection, fingerprint, sealed window, admission, exception code, lane-return loop, attempt ids — leave these texts and stay in `landing health` and `--json`.
2. The audit test `TestLaneRemediesNameIntentVerbs` (internal/landing/plain) walks every headline, question and remedy string the lane can produce through the fixtures of the existing tests (collect them from one table of producers) and fails on any mechanism word (a fixed list) or any `metasystem landing <verb>` outside the six and `prove --trunk` for the steward's own line.
3. `landing status` text: the first line under the headline is "to do: <intent verb>" when a person's act would help, else absent.

## Tests
the audit test itself with a fixture per producer; a mutation that re-introduces "recorded selection" in the barren-hold reason → red; a mutation that names `landing run --goals` as a remedy → red; the status "to do:" line present for held/stopped/question states and absent while proving.

## Checks
as U1's brief. Report: every producer string changed (before → after), the production line count, every check's exit, the mutations.
