Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal journey-chapter-fixes-2026-09-04)
Date: 2026-09-04

# Goal

Write the journey chapter that concludes the day's run of fixes on m2 (goals stop-hook-wedge-on-enrollment-drift, up-kills-runner, finding-register-id-collision-across-chains, path-class-fixture-ripgrep, dispatch-engine-script-skew-silent-exit with its carry member, channel-status-concise, and fixture-suite-drift-after-approval-gate if the orchestrator's facts below say it landed): one chapter appended at the END of metasystem/docs/journey.md (never anchor an edit on existing prose), in the file's voice and under the standard in metasystem/docs/backlog-mechanism.md, section "Concluding a goal": for a casual reader who has never seen this repository, no acronyms, identifiers, decision numbers or commit hashes in the prose, every borrowed word explained at first use, concrete first, meaning second, readable aloud, and concise: no restatement, no closing summary.

# What the chapter tells (the facts; use none of these labels verbatim)

On the fourth of September one machine, freshly enrolled the evening before, spent a night and a morning on the bugs the previous days had surfaced, each one small and each one a lesson about the machinery itself.

The first was a hook that would not let the assistant stop. The harness asks a small script at every turn end whether stopping is safe; when the script could not prove it within its deadline, because the engine's enrollment had drifted or a human-only remedy was pending, it refused, and the harness re-prompted the assistant forever, flooding the human's chat with hundreds of identical lines. The fix made the hook answer once and defer to the human when the remedy is not the assistant's to apply.

The second: the command that brings the system up killed the very runner it was meant to keep alive, because it treated the runner's own process as a stranger. Fixed by teaching it whose child the runner is.

The third: two unrelated reviews that each numbered a finding "one" collided in the shared register, and a correction round on one chain was blocked by the other's classification. Findings are now scoped to what a review reviewed: the same implementer chain or the same reviewed tree; unrelated reviews never union, while two critics of one subject with conflicting classes still refuse, naming both.

The fourth: a test searched with a tool outside the declared inventory; on a machine without it the search died and the test reported a false reader of deleted tables. The search now uses the ordinary tool, skips binaries, and a search that fails for any reason other than "no match" is reported as a broken search rather than passing.

The fifth: after a pull, scripts newer than the built engine read a field the engine did not emit, and the dispatcher died with a bare exit status and no message; three dispatches were refused with no reason until a trace found the line. It happened again that morning, mid-landing, when another machine's work arrived. The engine now reports its build stamp, the dispatcher refuses loudly, naming both commits and the remedy, when the engine predates a checkout whose engine or scripts changed, and a missing field is named. This fix lost its own budget on the way: two dispatches refused before any agent ran still consumed attempts, the box closed with the work finished on a preserved branch, and the human answered one question on the fleet channel to grant one more attempt; the ledger refused a second relayed approval on the same goal, so the fix landed through a member goal, the engine's own suggested split.

The sixth: the status message every machine posts to the fleet channel dumped the whole queued backlog four times a day, and the human said he only wanted what needs his judgement, what was delivered, and what comes next. The post now has those three parts, twelve lines at most, and is silent when nothing changed. Its first version listed every unapproved item as a decision, the same dump relabeled, and was corrected the same morning.

Along the way the machine learned that an implementer names files from where it stands, not from the repository root, and that a return in the wrong path form loses the round even when the work is right; that a six-digit code sent over the channel is checked against the moment the machine reads it rather than the moment the human wrote it, so an answer can fail for being read late; and that a question's expected reply is a token to repeat verbatim, not a free-form yes.

# Bounds

Append the chapter only; touch nothing else in the file and no other file. Every path in your return (diffBoundary, files) is relative to the repository root, so it starts with `metasystem/`.
