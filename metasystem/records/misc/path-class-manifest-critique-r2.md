# Path-class manifest design critique — round 2, closing (Sol)

Chain: design revision 2 -> critic path-class-crit2 (codex {'effective': 'gpt-5.6-sol', 'requested': 'gpt-5.6-sol'}, reviewed commit bd14936f2a839009cf6a03b23909f81b714d513c), 2026-09-03. 5 material findings under the stop criterion; this is the closing round, so every material finding becomes a test obligation for the build. Full return: artifacts/agents/path-class-crit2/rounds/1/return.json.

## PCM-R2-001-ADOPTED-OWNER-FIRST — high, material=True

CLAIM: PCM-R2-001, adopted ownership must precede manifest matching: TestAdoptedModeAnswersOutside in metasystem/internal/pathclass/pathclass_test.go and an adopted leg of TestPathClassVerbAnswersFromManifest in metasystem/scripts/agents/path-class-fixtures.sh must place application-owned docs/application.md beneath an install:docs/ behavior row and still assert outside. The resolver in metasystem/internal/pathclass/pathclass.go must consult application ownership before longest-prefix matching. Otherwise the manifest classifies an application's path, contrary to Wido's stated boundary.

EVIDENCE: Metasystem/plans/path-class-manifest-design.md:73-78 says only an adopted key with no matching row becomes outside and routes every path inside the installation through the install namespace; :81-84 defines install:docs/ as behavior. Metasystem/internal/stateroot/owner_test.go:98-126 proves that docs/application.md is application-owned when an adopted repository and installation share one root. The broad manifest prefix therefore captures an application path even though the design claims adopted application paths keep today's rules.

## PCM-R2-002-EXISTING-RECORD-APPEND — high, material=True

CLAIM: PCM-R2-002, immediate record-not-owned promotion is sound only after lawful existing-record appends pass: the existing-record-append leg of TestObserveRecordSemantics in metasystem/internal/landing/observe_test.go must create records/misc/fx-analysis.md at the base, append bytes under a goal held by the actor, and assert pass while replacement or deletion asserts register-carriage-not-append-only and a missing owner asserts record-not-owned. Metasystem/scripts/agents/landing-promotion.json may promote record-not-owned in slice 2 only with that leg green. A false refusal stops the wrapper before commit and forces the seat's ordinary record through a reviewed chain.

EVIDENCE: Metasystem/plans/path-class-manifest-design.md:199 says every existing records/ file other than two named registers is “new file only” and any modification returns record-not-owned. Metasystem/memory/rulings.md:92 instead says the record trees use “new-file-or-append only,” and metasystem/records/README.md:3-6 calls the records tree append-only rather than immutable. The design's own fact F7 at :27-30 acknowledges two existing record modifications; git show confirms both were pure appends. The zero-hit justification at :216-218 covers only frozen plans and cross-seat handoffs, not this wider use of record-not-owned.

## PCM-R2-003-GOAL-ITEM-FINAL-MESSAGE — high, material=True

CLAIM: PCM-R2-003, Goal-Item must be verified on the final commit message, not only exact-case input strings: TestCommitWrapperStampsGoalItemTrailer in metasystem/scripts/agents/path-class-fixtures.sh must prove that a lowercase goal-item input is refused, a commit-msg hook that injects or changes Goal-Item causes a soft rollback with HEAD unchanged, and a successful commit contains exactly one byte-exact Goal-Item value. Metasystem/scripts/agents/commit.sh must implement that final-message postcondition.

EVIDENCE: Metasystem/plans/path-class-manifest-design.md:153-158 promises a wrapper-owned field but scans only lines matching the case-sensitive ^Goal-Item:. Git interpret-trailers accepted lowercase goal-item: victim and then produced both that trailer and Goal-Item: fx. Metasystem/scripts/agents/commit.sh:363-375 invokes Git and verifies only the resulting tree, so prepare-commit-msg or commit-msg can alter the message after the proposed scan. The repository's parallel wrapper design documents this exact hook mutation at metasystem/plans/two-bars-caller-class-design.md:379-386.

## PCM-R2-004-SLICE-ONE-HANDOFF-SEAM — high, material=True

CLAIM: PCM-R2-004, slice 1 must preserve handoff carriage across the slice boundary: TestSliceOneRetainsHandoffCarriage in metasystem/internal/landing/observe_test.go must assert that a new plans/handoff-fixture-1.md still passes register carriage immediately after the two old list files are deleted and before slice 2 ownership logic exists. Metasystem/internal/landing/observe.go must retain that current exception during slice 1.

EVIDENCE: Metasystem/scripts/agents/register-carriage-paths.txt:4 currently allows plans/handoff-*.md. Metasystem/plans/path-class-manifest-design.md:281-283 replaces carriage eligibility in slice 1 with only “one of the three append-only files,” so the handoff pattern falls into register-carriage-path-refused until slice 2. Metasystem/AGENTS.md:35 requires an unfinished multi-session stream to update its handoff note, making this seam a repeatable operational refusal rather than a theoretical transition state.

## PCM-R2-005-FLOOR-PRECEDENCE — medium, material=True

CLAIM: PCM-R2-005, the evaluator must pin floor-before-ownership precedence: TestObserveFloorPrecedesGoalOwnershipValidation in metasystem/internal/landing/observe_test.go must combine a behavior path with an invalid or foreign Goal and assert direct-fix-floor-refused; a record-only change with the same foreign Goal must assert goal-item-not-held. Metasystem/internal/landing/observe.go must follow that ordering.

EVIDENCE: Metasystem/plans/path-class-manifest-design.md:115-117 says floor precedence remains set-wide, and :132-134 repeats “floor first.” Lines 142-145 contradict this by requiring a set Goal to be validated “before any path rule” and returning goal-item-not-held. An implementer can therefore produce two different documented verdict codes for the same mixed landing.
