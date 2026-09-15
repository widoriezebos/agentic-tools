VERDICT: land

Closing read (Opus) of coordinator-context-stays-under-budget, follow-up unit C1b, amendment 8c.15. Worktree /Users/wido/LocalStorage/GitHub/agentic-tools-m1e/.claude/worktrees/ccb-c1b at HEAD 25922904 with eight uncommitted files. Every probe and mutant ran in two private copies built from git archive of HEAD plus the eight changed files, each sha256-checked against the live file. Material findings: 0. Non-material findings: 5.

### F-1. Severity low. Material no.

Claim: the recorded-identity term is witnessed only for the pid, so weakening the whole `identity.Ref` comparison to a pid comparison survives every named test.

Evidence: mutant X4 changes handoff_capture.go:1060 from `binding.Predecessor == authority.Ref` to `binding.Predecessor.Pid == authority.Ref.Pid`. `TestCancelHandoffRefusesAnotherSession` passed (exit 0). The B9 witness varies only `Ref.Pid` (handoff_capture_test.go:962), exactly as the page's row B9 specifies, so start seconds, ticks and boot id are unwitnessed. D15-7's claim that a later session reusing a session id cannot match rests on the whole comparison, which the code does make, and the tag term also separates instances. Not material: the code conforms to D15-3, and removing the term outright (B9) is killed. An optional strengthening would add one row to handoff_capture_test.go.

### F-2. Severity low. Material no.

Claim: D15-2's rule that a person who omits `--by` is judged only by identity has no verb-level witness.

Evidence: mutant X9 changes context_verbs.go:173 from `if bySupplied {` to `if bySupplied || caller.Class == lease.ClassHuman {`. `TestContextVerifyAndCancel`, `TestContextHandoffCancelByHuman` and `TestContextVerbUsage` all passed. On the unmutated tree, a probe in a private copy showed a HUMAN caller without `--by` exits 9 with `HANDOFF_NOT_HOLDER`, so the code is right. Under the mutant, the steward's name check (D15-4 step 6) still refuses the empty name, so no cancellation becomes possible. The rule is witnessed at the steward by B14's zero-caller and untrusted rows, and no row asks for a verb-level witness. Not material.

### F-3. Severity low. Material no.

Claim: the new `HANDOFF_OTHER_SESSION` row is out of alphabetical order, while D15-10 places both rows in the alphabetical steward block.

Evidence: register.go:53 holds `HANDOFF_OTHER_SESSION` and register.go:54 holds `HANDOFF_OTHER_PENDING`. `sort -c` over the `HANDOFF_` codes reports disorder at `HANDOFF_OTHER_PENDING`, and every other row in the block is in order. No test or reader depends on row order. Not material.

### F-4. Severity low. Material no.

Claim: the diff adds one unrelated blank line to an existing test.

Evidence: handoff_capture_test.go:700 is a new empty line before the closing brace of `TestHandoffAcceptsOnlyTheActiveContinuation`, a test the brief does not name. gofmt keeps it, and it counts one of the 398 lines. It has no behaviour or proof effect. Not material.

### F-5. Severity low. Material no.

Claim: the builder's mutation table cites failure lines from earlier test revisions, and its B7 mutant is killed by the recording call rather than the cancellation.

Evidence: the builder lists B1 at context_verbs_test.go:879, B3 at :790, B8 to B12 at handoff_capture_test.go:972, B13 at :989, B16 at :847 and A4e at :889. On the final tree the same mutants fail at :902, :811, :977, :994, :849 and :891. The builder's B7 mutant removes `activeDelegateCaller` inside `admitHandoffCaller`, which `Handoff` shares, so its observed `HANDOFF_UNOBSERVABLE runtime=devin` comes from recording with the supplied runtime `devin`. My cancel-side-only mutant B7c kills the `delegate` subtest at the cancellation (handoff_capture_test.go:944, `HANDOFF_OTHER_SESSION ... caller=DELEGATE`), so the cancel-side rule is witnessed. Every kill reproduces on the final tree (table below). Not material.

### Mutation table

Every mutant was applied alone in a private copy, and the applier refused any pattern that did not match exactly once. The named test ran with `-count=1 -timeout 40m` under GOCACHE=/tmp/opus-c1b-gocache and GOTMPDIR=/tmp/opus-c1b-tmp, with METASYSTEM_BIN unset. The file was then restored, and `git diff --quiet` confirmed the restore. The unmutated baseline selection was green: steward ok 55.3s, refusal ok 0.67s, cmd/metasystem ok 8.36s.

| Rule | Mutant, alone | Test run | Result | Failing case and observed line |
| --- | --- | --- | --- | --- |
| A19 | register.go:47 `HANDOFF_HUMAN_UNPROVEN` Shape Identity to Question | TestHCL03HandoffCancelRows | killed | register_test.go:85 row HANDOFF_HUMAN_UNPROVEN shape:question, want shape="identity" |
| R1 | handoff_capture.go:1109 empty-session guard to `if true` | TestCancelHandoffAdmitsAProvenHumanAct | killed | empty session with process generation, :843 session=e3b0c442... want session=none |
| R2 | handoff_capture.go:1112 `boot := ""` | TestCancelHandoffAdmitsAProvenHumanAct | killed | empty session with process generation, :843 human-boot=none want boot-fixture |
| R3 | handoff_capture.go:1117 ticks printed as 0 | TestCancelHandoffAdmitsAProvenHumanAct | killed | empty session with process generation, :843 human-ticks=0 want 5 |
| R4 | handoff_capture.go:1050 empty-class substitution to `if false` | TestCancelHandoffRefusesAnUnprovenHumanAct | killed | empty caller class, :923 caller= want caller=none |
| A4e | handoff_capture.go:1137 not-exist branch returns a plain error when Human is set and its proof is valid for stateRoot | TestCancelHandoffRefusesAnUnprovenHumanAct | killed | only consumed nonce with a proven act, :891 consumed cancellation=handoff 6400000000000001 is not live |
| A4e (F-12 form) | same site, plain error when Human is set and Caller.Class is HUMAN | TestCancelHandoffRefusesAnUnprovenHumanAct | killed | only consumed nonce with a proven act, :891 mutant: human act on a nonce that is not live |
| B1 | context_verbs.go:159 whole `--by` usage predicate to false | TestContextVerbUsage | killed | context_verbs_test.go:902 args --root R --by Wido = code 1 (no machine nickname) |
| B1 (term) | :159 drop `!cancelSupplied` | TestContextVerbUsage | killed | :902 args --root R --by Wido = code 1 |
| B1 (term) | :159 drop the TrimSpace blank test | TestContextVerbUsage | killed | :902 args --cancel N --by= = code 1 |
| B2 | context_verbs.go:177 `os.Getppid()` to `os.Getpid()` | TestContextHandoffCancelByHuman | killed | attended human, :765 human proof arguments pid=57149 |
| B3 | context_verbs.go:176 class gate to `if bySupplied` | TestContextHandoffCancelByHuman | killed | agent never reaches the prover, :811 agent reached the human prover |
| B4 | context_verbs.go:177 proof discarded | TestContextHandoffCancelByHuman | killed | attended human :771 code 9 human=unobserved; refused proof also failed |
| B5 | session_stop.go:25 back to a zero proof | TestProveSessionStopHumanKeepsThePopulatedProof plus both session stop command tests | killed | session_stop_test.go:196 proof outcome="" err=ANCESTRY_UNREADABLE; both command tests stayed green |
| B6 | handoff_capture.go:1071 binding-match branch to `if false` | TestCancelHandoffAdmitsTheRecordingSession | killed | main and delegate, :944 HANDOFF_OTHER_SESSION ... caller=MAIN |
| B7 | handoff_capture.go:1067 cancel side only: authority is the supplied caller, admission skipped | TestCancelHandoffAdmitsTheRecordingSession | killed | only delegate, :944 HANDOFF_OTHER_SESSION ... caller=DELEGATE |
| B8 | handoff_capture.go:1058 session term to true | TestCancelHandoffRefusesAnotherSession | killed | only session, :977 refusal=<nil> |
| B9 | handoff_capture.go:1060 Predecessor term to true | TestCancelHandoffRefusesAnotherSession | killed | only pid, :977 refusal=<nil> |
| B10 | handoff_capture.go:1061 tag term to true | TestCancelHandoffRefusesAnotherSession | killed | only tag, :977 refusal=<nil> |
| B11 | handoff_capture.go:1059 main-id term to true | TestCancelHandoffRefusesAnotherSession | killed | only main id, :977 refusal=<nil> |
| B12 | handoff_capture.go:1057 runtime term to true | TestCancelHandoffRefusesAnotherSession | killed | only runtime, :977 refusal=<nil> |
| B13 | handoff_capture.go:1062 job term to true | TestCancelHandoffRefusesAnotherSession | killed | only job, :994 refusal=<nil> want ... caller=DELEGATE |
| B14 | handoff_capture.go:1067 admission error wrapped | TestCancelHandoffPassesAdmissionRefusalsThrough | killed | all six rows, first :1017 refusal=cancel caller: HANDOFF_NOT_HOLDER |
| B14 | handoff_capture.go:1067 admission error relabelled HANDOFF_NOT_HOLDER | TestCancelHandoffPassesAdmissionRefusalsThrough | killed | unobservable and plain error, :1017 want HANDOFF_UNOBSERVABLE runtime=devin |
| B15 | handoff_capture.go:1135 zero seat caller judged before the live read | TestVerifyHandoffStateUsesExactLifecycleRecord | killed | consumed remains verifiable..., :1310 consumed handoff cancellation=HANDOFF_NOT_HOLDER |
| B16 | handoff_capture.go:1066 nil branch returns the seat reason before admission | TestCancelHandoffAdmitsAProvenHumanAct plus TestCancelHandoffPassesAdmissionRefusalsThrough | killed | dead recorder :849 refusal=<nil> want HANDOFF_NOT_HOLDER; all six admission rows also failed |
| B17 | context_verbs.go:185 zero canceller passed | TestContextVerifyAndCancel | killed | context_verbs_test.go:701 cancel = code 9 stderr HANDOFF_NOT_HOLDER |
| B17 (fail closed) | context_verbs.go:168-171 classification error ignored | TestContextVerifyAndCancel | killed | :719 unclassified cancel = code 9 stderr HANDOFF_NOT_HOLDER |
| B18 | register.go:53 override without `--root <installation>` | TestHCL03HandoffCancelRows | killed | register_test.go:85 row HANDOFF_OTHER_SESSION Override:metasystem context handoff --cancel <nonce> ... |
| B18 | register.go:53 Commands 1 to 2 | TestHCL03HandoffCancelRows | killed | register_test.go:85 row HANDOFF_OTHER_SESSION |
| B18 | register.go:53 Shape Agent to Question | TestHCL03HandoffCancelRows | killed | register_test.go:85 row HANDOFF_OTHER_SESSION Shape:question |
| B19 | register.go:53 Site 1074 to 1073 | TestHCL03HandoffCaptureSitesAreEmissionLines | killed | register_test.go:103 row HANDOFF_OTHER_SESSION site "handoff_capture.go:1073" is not an emission line |
| extra | context_verbs.go:182 ClaimEpoch not copied into the act | TestContextHandoffCancelByHuman | killed | attended human, :771 code 1 supplied holder coordinates do not match |
| extra | handoff_capture.go:1058 NormalizeSession dropped from the session term | TestCancelHandoffAdmitsTheRecordingSession plus TestCancelHandoffRefusesAnotherSession | killed | main and delegate, :944 HANDOFF_OTHER_SESSION |
| extra X4 | handoff_capture.go:1060 whole Ref compared by pid only | TestCancelHandoffRefusesAnotherSession | survived | F-1 |
| extra X9 | context_verbs.go:173 human act attached to a HUMAN caller without `--by` | TestContextVerifyAndCancel, TestContextHandoffCancelByHuman, TestContextVerbUsage | survived | F-2 |

### Checks with no finding

Conformance. All eight changed files are inside the Boundary, and there are no untracked files. Numstat is 385 additions plus 13 deletions, 398 lines, under the 400 ceiling. There is no diff under internal/goal, internal/lease, internal/humanauthority, cmd/metasystem/main.go, scripts, docs or testing.json. The human leg's production lines in `handoffCancelReason` are unchanged. The only human-leg test additions are the dead recorder subtest and the three carried cases, and the two F-2 subtests are identical to the tested code in codex-ccb-c1a-result-2.md. context_verbs.go names no `classifySessionStopCaller`, `classifySessionStopView` or `sessionStopNow`, and it classifies once per cancel through `contextHandoffCaller`. session_stop.go changes one line, the `ProveTerminal` error return, so `runSessionStop` is unchanged. `admitHandoffCaller` and the `HANDOFF_HUMAN_UNPROVEN` text are unchanged. The existing rows of `TestContextVerbUsage` keep their assertion, and `TestContextVerifyAndCancel` only gains legs. `TestVerifyHandoffStateUsesExactLifecycleRecord` changes only its canceller, to the zero caller, as B15 requires. gofmt reports nothing on the eight files.

Cutover. `CancelHandoff` has one non-test caller, context_verbs.go:185, and it always passes the classified caller. The nil branch now runs admission and the binding match, so a zero canceller is refused `HANDOFF_NOT_HOLDER`. The command therefore cannot cancel a live handoff unless the caller is admitted as the recording session or passes the human act. The same commit removes the unauthenticated path: B16 kills its restoration, as do the six admission rows. The remaining cancel paths are the supersession cancel inside `Handoff`, after capture admission, and the engine's `CancelIntent` calls in revive.go. D15-11 leaves both alone. No fixture script cancels a handoff (supervision-hook-fixtures.sh:1000 only records). Admission now runs under the arbitration lock in `CancelHandoff`, as it already did in `Handoff` (handoff_capture.go:929 before :592), so lock ordering does not change.

Dead recorder. A person can still cancel a dead recorder's handoff on this tree. B16 (steward) and B2 (verb) pass. A verb-level probe in a private copy ran the whole shape through the command. An untrusted caller was refused `HANDOFF_NOT_HOLDER` with exit 9. A HUMAN caller with `--by Wido` and an empty holder session then exited 0, and the tombstone read `cancelled: cancelled by human by=Wido holder=main-context epoch=7 session=none human-pid=10039 human-started=1789438032 human-ticks=0 human-boot=none`.

Register. With the Site field removed, the `HANDOFF_OTHER_SESSION` row equals the D15-10 row byte for byte (scripted comparison): Owner internal/steward, Shape Agent, the full one-command override, Commands 1. The test's expected override is the same string. All 13 steward rows whose Site is in handoff_capture.go name a line carrying their code literal, checked by script and by `TestHCL03HandoffCaptureSitesAreEmissionLines`. `TestHCL03EveryCodeRowed` and `TestHCL03PendingRowsNamed` pass.

Not run here. I did not run `go build ./...`, `go vet`, the race suite or `scripts/agents/go-gate.sh --fast`. The builder's result reports that the race suite could not complete in its sandbox and the fast gate was not run, so both remain with the seat.

Live worktree. At the start and at the end, HEAD was 25922904, the status listed the same eight modified files, and the sha256 of each file and of the full `git diff HEAD` were identical (diff of the two hash records empty).
