# Engine re-arm design critique — round 1

Chain: revision 1 (landed a4c5947d5, sha256 8cd403ed6dea2b3a20a02047429413ca57ab216d9df01e5560d8253689819b11) -> critic engine-rebuild-rearm-critique-r1-20260906 (claude, design-critic, fresh context), 2026-09-06. EIGHT material findings (five high, three medium) and one non-material label inconsistency: revision 2 is required before any build. Register root for closure: engine-rebuild-rearm-critique-r1-20260906.

## ERAR-R1-01-STRAY-CRON — high, material=True

CLAIM: The sole automatic eligibility condition admits the stray cron caller that the stated threat model explicitly intends to reject. The design asserts that “the stray cron job of the header runs some other binary at some other path,” but neither code nor policy imposes that restriction. metasystem/cmd/metasystem/up.go derives Binary from os.Executable, so a stray cron invoking the rebuilt engine at metasystem/bin/metasystem satisfies C13's path test. The design deliberately re-arms before session identity is resolved, so the later failure to recognize the cron cannot prevent the mint. Eligibility needs a recognized-session or equivalent caller proof before re-arm; canonical pathname equality alone does not preserve the stated accident-proofing boundary.

EVIDENCE: metasystem/internal/steward/identity.go:3-8 says the pin prevents a stray cron job from matching the record. metasystem/plans/engine-rebuild-rearm-design.md:85-87 assumes without support that such a cron necessarily uses another path, while lines 311-316 require re-arm before session identity. The current ordering in metasystem/internal/up/up.go:418-428 confirms enrollment work precedes resolveSessionIdentity.

## ERAR-R1-02-LOCKED-ORDER — high, material=True

CLAIM: The proposed arm ordering performs destructive live-runner handling before eligibility and skip are decided. Consequently, a stranger that should refuse can stop the enrolled runner, and the second concurrent re-arm can stop the newly installed runner before returning already-current. This directly contradicts the design's claims that C14 leaves the installation untouched and that Skip “does not stop the live runner.” The locked decision must precede every stop/no-op decision, not merely the mint.

EVIDENCE: metasystem/plans/engine-rebuild-rearm-design.md:182-184 explicitly places the record read and decide callback after “live-runner handling.” The machine path always supplies replace=true at lines 190-193. Existing metasystem/internal/steward/runner.go:410-417 stops a live runner before line 418 reads the record. The contradictory skip contract is at design lines 204-207, and fixtures F3 and F4 require one live runner after the skipped contender.

## ERAR-R1-03-PROVENANCE — high, material=True

CLAIM: C13 is broader than the governing re-arm order because it automatically enrolls unstamped or unproven builds, and EngineBuild is not tied to the bytes digested under the lock. The design expressly permits BuildStamp dev and postpones provenance eligibility to a future feature, although R-37-m3 requires engines built from landed commits and requires every re-arm to record the consumed commit. A second stage-and-rename between the initial invocation and the locked digest can also make the running process's BuildStamp describe engine B while the record enrolls engine C. The design needs a present-tense eligibility rule that rejects an absent, dev, or otherwise unverified build stamp and reads provenance from the same pinned bytes it enrolls.

EVIDENCE: metasystem/memory/rulings.md:64 says re-arms are “bounded by construction to engines built from commits that arrived through the landing gates” and that every re-arm records the consumed commit. metasystem/internal/supervise/disk.go:135-139 defaults BuildStamp to dev. The design postpones provenance checking at metasystem/plans/engine-rebuild-rearm-design.md:200-218 and admits the violation at lines 469-475. The arm lock does not coordinate metasystem/scripts/agents/go-build.sh:45-63, which replaces the executable path atomically.

## ERAR-R1-04-LEGACY-MIGRATION — medium, material=True

CLAIM: The legacy rule knowingly reports an existing machine-minted enrollment as human-witnessed, but the promised cleanup is neither a build prerequisite nor reliable through steward arm. The current generation 9 record has no new stamp, and the design admits that it was machine-minted. Moreover, ordinary Arm returns already armed before minting whenever a runner is live, so the design's statement that the next steward arm clears machine state is false. The rollout needs an explicit migration or mandatory human restart/temporary arm before health may label this record human-witnessed.

EVIDENCE: metasystem/artifacts/agents/steward/identity.json:1-9 shows generation 9 minted on 2026-09-05 with no MintedBy or witnessed fields. metasystem/plans/engine-rebuild-rearm-design.md:266-268 classifies every such record as human-witnessed, while lines 476-479 admit the known machine-minted exception. The claimed clearing behavior at lines 259-263 conflicts with metasystem/internal/steward/runner.go:410-413, where Arm with replace=false returns without minting beside a live runner.

## ERAR-R1-05-VISIBILITY — high, material=True

CLAIM: Decision 3 does not make a re-arm never silent end to end. A successful re-arm followed by session, announcement, lease, supervision, or runner failure flows through helpers that construct a new Result without carrying ReArmed; both hook modes then inspect only the aggregate last line. Even when up succeeds, Stop can exit through emit_failed_stop before up_notice joins extras, or the four-second parent can terminate the worker after the mint and emit a deadline response that contains no notice. F5 tests only immediate stub success plus normal verdict paths, so it cannot prove the absolute visibility claim.

EVIDENCE: The proposed aggregate field is described at metasystem/plans/engine-rebuild-rearm-design.md:301-310, but existing metasystem/internal/up/up.go:108-113 constructs failure Results without inherited fields and lines 428-483 contain several post-enrollment failures. metasystem/scripts/agents/supervision-hook.sh:126-220 replaces timed-out worker output; lines 530-533 and 585-588 emit before extras are assembled at lines 616-619. F5 at design line 424 covers only an immediate exit-zero stub and normal allow/block verdicts.

## ERAR-R1-06-PARTIAL-MINT-API — high, material=True

CLAIM: Decision 4's partial-success contract cannot be implemented from the proposed arm signature and omits two post-mint failure stages. arm still returns only string and error, so ReArmRebuiltEngine cannot reliably distinguish an error before MintIdentity from an error after it without a racy record reread. After minting, reopening and snapshot preparation can also fail before launch, yet the design defines Minted=true only as “the mint landed and the launch failed.” The design must specify a typed stage/outcome returned while the lock is held for every post-mint error; choosing no rollback is otherwise not observably implementable.

EVIDENCE: The proposed signature at metasystem/plans/engine-rebuild-rearm-design.md:174-176 returns only (string, error), while ReArmOutcome and Minted semantics are required at lines 230-233 and 389-399. Existing metasystem/internal/steward/runner.go:424-441 has three distinct post-mint operations that can fail: OpenEnrolledBinary, PrepareForExecution, and launchRunner.

## ERAR-R1-07-FIXTURE-HARNESS — medium, material=True

CLAIM: The named proof cannot run as specified without a different executable integration harness or additional production behavior. F6 requires a real engine whose steward run exits when BuildStamp is die, but the shipped command never reads BuildStamp. An in-process internal/up fixture also executes the Go test binary, so supervise.BuildStamp cannot prove engine B's commit merely because Options.Binary names B. Finally, the ordinary gate installs its executable after internal tests, and the cited steward test supplies cleanup but not the promised process-owning group and isolated supervision registry. The design must name an exec-driven integration or shell proof, its cleanup ownership, and a legitimate failure injection seam.

EVIDENCE: F1 and F6 are specified at metasystem/plans/engine-rebuild-rearm-design.md:420 and 425; the unsupported die mechanism is explicit in F6. metasystem/cmd/metasystem/steward_verbs.go:470-493 runs the steward loop without consulting BuildStamp. metasystem/scripts/agents/go-gate.sh:528 runs internal tests and only lines 549-556 install the executable in an ordinary run. metasystem/internal/steward/runner_test.go:156-200 uses t.Cleanup with Disarm but creates neither a process group nor METASYSTEM_SUPERVISION_REGISTRY_HOME.

## ERAR-R1-08-C16-OUTCOME — medium, material=True

CLAIM: C16 is not dispositioned according to the actual up outcome paths. The table defines every REFUSE as an accepted-engine ENROLLMENT_DRIFT result, but drift returned by EnrolledBinary.Command during supervision-owner or steward-runner launch is propagated as a component failure, not through enrollmentDrift. The design must either route typed command-time drift to the declared refusal outcome or state and test the distinct outcome mapping.

EVIDENCE: metasystem/plans/engine-rebuild-rearm-design.md:116-120 defines REFUSE as the existing ENROLLMENT_DRIFT response and line 139 assigns Command drift to REFUSE. metasystem/internal/steward/identity.go:228-235 returns ErrEnrollmentDrift from Command. metasystem/internal/up/up.go:354-365 maps a Command error during supervision to failure, while lines 480-483 map a steward launch error to steward-runner failure.

## ERAR-R1-09-TABLE-LABEL — low, material=False

CLAIM: The statement that the table has exactly one RE-ARM row is textually false: C13 and C18 are both labelled RE-ARM. C18 is conditional on C13 and does not create an independent automatic cause, so this is a count/label inconsistency and would not change a competent implementation.

EVIDENCE: metasystem/plans/engine-rebuild-rearm-design.md:136 labels C13 RE-ARM, line 141 labels C18 “RE-ARM (when C13 holds),” and line 143 nevertheless says C13 is the only RE-ARM row.

## Critic-declared gaps (verbatim)
