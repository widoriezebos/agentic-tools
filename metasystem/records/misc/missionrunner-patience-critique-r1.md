# Missionrunner patience design critique — round 1 (Sol)

Chain: design implementer-45c39b11f8b164818040e7d6 (Fable lane) ->
critic design-critic-3a18d1fa751735d994a611b7 (codex gpt-5.6-sol,
xhigh, fresh context), 2026-09-02. Nine material findings — every one
an executable question about process-group observation semantics
(uncertainty-as-death, natural-exit laundering, fingerprint stalls on
healthy kills, global census churn as false progress, SIGKILL
unproven by error, identity-unsafe resolver joins, leader-only
cleanup, unreachable ceiling diagnostics, stays-red without
deterministic proof). Per the day's proven pattern, an executable
spike answers them before revision 2 folds.

## DC-PAT-001-UNKNOWN-IS-DOWN — high, material=True

CLAIM: The proposed group observation turns uncertainty into death, so it cannot support the nil-return ownership discriminator. The design says an identity.AllPids failure produces a nil substantive-process list and calls that conservative, but its down predicate treats every empty or nil list as down. Per-process Getpgid failures, process probe errors, and an Alive process with unreadable argv are also omitted from the list and can produce the same false down result. A nil terminateGroup return caused by ownership refusal can therefore be accepted immediately as groupWentDown or died-late during th

EVIDENCE: metasystem/plans/missionrunner-patience-design.md section 3.1 defines down as group absent or zero substantive process identifiers. metasystem/internal/missionrunner/proc.go:85-100 instead returns true on identity.AllPids failure to preserve uncertainty. metasystem/internal/identity/identity_linux.go:62-84 returns Alive with ArgvKnown false when ar

## DC-PAT-002-NATURAL-EXIT-LAUNDERS-REFUSAL — high, material=True

CLAIM: A persistent ownership-proof refusal is not guaranteed to stay red because the fixture expires naturally at the same duration as the new stall limit. The retry loop invokes terminateGroup again only after a 30-second unchanged observation, while the unsignalled fixture runs sleep for 30 seconds. Because each loop checks down before checking the stall clock, natural expiry can return groupWentDown and pass the kill-through test even though terminateGroup refused ownership on every invocation that occurred. The compressed leak classifier has the same laundering path through died-late.

EVIDENCE: metasystem/internal/missionrunner/winddown_test.go:21-25 creates a group whose work naturally ends after 30 seconds. metasystem/plans/missionrunner-patience-design.md sections 3.0, 3.2, and 3.3 set observationStall to 30 seconds, check down first, and retry only after groupStalled. The shipped test at metasystem/internal/missionrunner/winddown_test

## DC-PAT-003-GROUP-SIGNAL-IS-STILL-A-TIMEOUT — high, material=True

CLAIM: The group-membership fingerprint is not a progress signal for the ordinary healthy kill path; it leaves a fixed 30-second speed assertion. A fixed process group normally retains the same members until they disappear, so a delayed but correctly delivered kill can show no shrinking set before final death. The observer will label that healthy-but-slow interval stalled after 30 seconds. Calling the limit a progress failsafe does not change that its normal input has no intermediate progress to reset it.

EVIDENCE: metasystem/plans/missionrunner-patience-design.md sections 3.1-3.2 fingerprint only the alive flag and substantive process identifiers, and return groupStalled after 30 seconds without a fingerprint change. metasystem/internal/missionrunner/winddown_test.go:21-25 creates a stable bash-and-sleep group rather than an actor with a monotonic attempt ma

## DC-PAT-004-GLOBAL-CENSUS-CHURN-LOOKS-LIKE-PROGRESS — high, material=True

CLAIM: The census fingerprint can advance while the target group is completely wedged because it contains unrelated system-wide uncertainty. ScanTaggedProcesses enumerates every process and records any identity-indeterminate process even when its tag cannot be determined. Process churn elsewhere on the loaded machine can therefore keep changing the indeterminate rows and reset the target group's stall clock. The 300-second ceiling eventually makes this red, but the alleged progress signal is unrelated to target progress and combines with natural fixture expiry to permit a false died-late pass.

EVIDENCE: metasystem/internal/census/tagged.go:165-182 enumerates the whole process table, and metasystem/internal/census/tagged.go:219-223 appends an indeterminate row without establishing tag membership. metasystem/plans/missionrunner-patience-design.md sections 3.1 and 3.4 fingerprint every indeterminate row and reset the stall clock on any fingerprint ch

## DC-PAT-005-ERROR-DOES-NOT-PROVE-SIGKILL — high, material=True

CLAIM: A non-nil terminateGroup result does not prove that ownership was established and SIGKILL was sent, contrary to both new classifiers. After SIGTERM, terminateGroup can return an invalid scale or heartbeat-interval error before reaching SIGKILL. The proposed tests may forgive such an error if the TERM-responsive group dies or the fixture expires naturally, changing a configuration failure into died-late success. The classifier table collapses these shipped outcomes into the single label kill-through error and therefore asserts the wrong outcome mapping.

EVIDENCE: metasystem/internal/missionrunner/host.go:120-154 sends SIGTERM, then calls ScaledWaitAtLeast and Interval, either of which can error before the SIGKILL at line 151. metasystem/internal/missionrunner/missionrunner.go:75-118 documents and implements those configuration errors. metasystem/plans/missionrunner-patience-design.md sections 3.3-3.4 state

## DC-PAT-006-RESOLVER-LOSES-PROCESS-IDENTITY — high, material=True

CLAIM: The zero-process-group-identifier census completion rule is not an identity-safe proof. It joins an old indeterminate process identifier to a later Getpgid result without proving that the process identifier still names the same process. Exit and process-identifier reuse between the scan and resolver call can grant target-group custody based on two different processes. This contradicts the design's assertion that the resolver is proof completion rather than proof weakening.

EVIDENCE: metasystem/plans/missionrunner-patience-design.md section 3.4 resolves an Indeterminate row containing only PID and zero PGID by a later Getpgid call. metasystem/internal/census/tagged.go:56-61 shows that IndeterminateProcess carries no exact identity. By contrast, metasystem/internal/census/tagged.go:187-217 performs a second start-identity read a

## DC-PAT-007-CLEANUP-DOES-NOT-PROVE-GROUP-EXIT — high, material=True

CLAIM: The cleanup block is not the promised group cleanup handshake. Its channel proves only that the bash leader was reaped, not that the process group left the table, and the SIGKILL result is discarded. If reap does not finish in 30 seconds, cleanup logs and returns green, allowing the goroutine and group to outlive the test indefinitely. This does not close the stated cross-package interference path and is weaker than the cited steward precedent.

EVIDENCE: metasystem/plans/missionrunner-patience-design.md section 3.7 closes reaped after cmd.Wait, ignores group SIGKILL errors, and uses t.Logf on timeout. Its failure-anatomy table says cleanup progress requires both reap completion and the group leaving the table, but no group observation appears in the cleanup block. Commit 65c36111, the cited precede

## DC-PAT-008-CEILING-REASON-CANNOT-REACH-CALLER — medium, material=True

CLAIM: The classifier cannot produce the ceiling diagnostic required by its own contract. Both a no-change stall and a changing-until-ceiling condition return the same leaked classification and census value; the specified return type carries no reason. The caller's unchanged default log consequently cannot know when to print the required oscillation wording. An implementer must either violate the fixed signature or omit the promised evidence.

EVIDENCE: metasystem/plans/missionrunner-patience-design.md section 3.4 specifies only liveWindDownClassification and TaggedProcessCensus as return values, while steps 4 and 5 both return leaked and step 5 requires special oscillation wording. Section 3.5 prescribes an unchanged default log with no reason parameter, and table row 5 asserts only classificatio

## DC-PAT-009-STAYS-RED-HAS-NO-DETERMINISTIC-PROOF — high, material=True

CLAIM: The proof profile never deterministically exercises the central stays-red invariant. The table test fakes only group and census observations; it does not fake terminateGroup, force three consecutive ownership refusals, or assert that natural expiry and observation uncertainty cannot bypass attempt exhaustion. Repetition under ambient load cannot prove a branch that the environment may never produce, so an implementation that launders every persistent refusal can still satisfy all specified verification commands.

EVIDENCE: metasystem/plans/missionrunner-patience-design.md section 3.6 lists classifier rows but no terminateGroup actor sequence. Section 5 relies on repeated live process tests and treats a refusal-exhaustion message as an incidental finding rather than a required fixture outcome. The direct call at metasystem/internal/missionrunner/winddown_test.go:105 h

## Critic-declared gaps (verbatim)

- The exact historical failure artifact named by metasystem/plans/goals/missionrunner-terminate-flake.md was absent, so the critique could not compare the design's classifier claims with that run's actual stderr, timing, or process state.
- Live focused verification was blocked by the read-only runtime because Go could not create a temporary build directory. Findings are grounded in the checked-in control flow, but no new execution evidence was obtained.
