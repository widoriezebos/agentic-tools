# Fleet-join design critique — round 1 (Sol)

Chain: design revision 1 -> critic fleet-join-crit1 (codex {'effective': 'gpt-5.6-sol', 'requested': 'gpt-5.6-sol'}, reviewed commit 5db6c42147350488b201e7c7e6565f20c144e369), 2026-09-02. 9 material findings. Full return: artifacts/agents/fleet-join-crit1/rounds/1/return.json.

## FJB-R1-001-HUMAN-REARM-DOES-NOT-REENROLL — high, material=True

CLAIM: The human-terminal rejoin path is not idempotent after an engine rebuild. The design says that an existing seat reaches step 5 with the same three cases and that “arm replaces the live runner and mints the next generation.” That is false for the no-word human case: plain steward arm calls Arm with replace=false, and a live runner returns “already armed” before either the enrollment identity or generation is updated. The subsequent up therefore still reports enrollment drift. Only temporary arm or steward restart replaces a live runner.

EVIDENCE: metasystem/plans/fleet-join-bootstrap-design.md lines 243-253 specify plain steward arm for a human and claim replacement. metasystem/internal/steward/runner.go lines 147-169 show Arm uses replace=false while ArmTemporary and Restart use replace=true; lines 403-420 return early for a live runner when replace is false and mint the identity only afterward. The design’s own corrected remedy at metasystem/plans/fleet-join-bootstrap-design.md line 417 already says restart is needed when a runner is live.

## FJB-R1-002-HUMAN-UP-CANNOT-FINISH-JOIN — high, material=True

CLAIM: The one-shot path introduces an undocumented second human stop after successful enrollment. Step 6 invokes up without process identity flags while explicitly acknowledging that a human shell requires the process identifier and start-time pair. Such a run fails at session identity rather than reaching green up, contradicting the goal’s completion rule that the only accepted non-green stop is where a human word is required. The composing-script precedent already demonstrates how to obtain and pass the pair.

EVIDENCE: metasystem/plans/fleet-join-bootstrap-design.md lines 255-263 require bin/metasystem up --repo . without identity flags and acknowledge the human-shell requirement. metasystem/internal/up/up.go lines 163-186 require runtime-signature ancestry when the pair is absent and lines 428-431 return the explicit-pair remedy. metasystem/scripts/agents/second-session.sh lines 37-42 obtains proc started-at for the shell and passes --pid and --start-time. metasystem/plans/goals/fleet-join-bootstrap.md line 4 permits only green up or the human-word stop.

## FJB-R1-003-RESOLVED-VALIDATION-DIVERGES-FROM-RESOLUTION — high, material=True

CLAIM: The proposed resolved validator does not implement the repository’s resolution contract. The design says “Overlay every .local key onto values (last writer wins, matching resolve.go)” and ignores environment sources. Actual resolution gives the environment higher precedence than the local overlay and refuses duplicate keys within the local file. Implementing the design literally can therefore validate green while the running system either selects different values or refuses the same configuration.

EVIDENCE: metasystem/plans/fleet-join-bootstrap-design.md lines 345-369 define resolved behavior, especially last-writer-wins at lines 355-357. metasystem/internal/config/resolve.go lines 42-49 and 67-95 define environment then local precedence. ConfLookup at lines 121-147 returns an error for duplicates rather than selecting the last value. These differences affect role runtimes and models that dispatch actually resolves through this path.

## FJB-R1-004-MACHINE-NICKNAME-GRAMMAR-UNCHECKED — medium, material=True

CLAIM: Machine enrollment lacks the nickname grammar required by later ledger operations. The script sets any nonempty --machine value directly in git configuration, and ResolveMachine only trims it. A nickname containing whitespace passes join orientation but later renders extra whitespace-delimited ledger fields; the reserved nickname “-” can claim but can never be used as a pin target. The design supplies neither validation nor a refusal for these cases.

EVIDENCE: metasystem/plans/fleet-join-bootstrap-design.md lines 172-175 and 209-215 define only empty, existing, and different-name handling. metasystem/internal/goal/actor.go lines 21-27 accepts every nonempty trimmed value. History parsing at metasystem/internal/goal/file.go lines 851-914 treats whitespace-separated extras as unknown fields. metasystem/internal/goal/verbs.go lines 1898-1912 defines the existing machine-nickname grammar as one whitespace-free word and reserves “-”.

## FJB-R1-005-BUILD-STAMP-READER-UNSPECIFIED — medium, material=True

CLAIM: The engine freshness gate is left to implementer invention. The design names “metasystem version” or “the equivalent stamp reader the implementer finds,” but no metasystem version command exists. Choosing a stale artifact, always rebuilding, or parsing Go build metadata produces materially different binary replacement and enrollment-drift behavior. The exact existing go version -m mechanism should be selected by the design if it is the intended contract.

EVIDENCE: metasystem/plans/fleet-join-bootstrap-design.md lines 186-191 make stamp equality the build-versus-skip decision. metasystem/scripts/agents/go-build.sh lines 38-64 embed BuildStamp. A repository search finds no top-level metasystem version command; metasystem/scripts/validate-metasystem.sh line 594 is the existing reader and parses go version -m output.

## FJB-R1-006-FIXTURE-CONFIGURATION-FAILS-BEFORE-JOIN — high, material=True

CLAIM: The fixture’s fresh-clone configuration cannot reach the join sequence. It appends metasystem.runtimes=fake to a committed file that already has metasystem.runtimes, which the validator rejects as a duplicate. It also supplies only evidence-root, notification, machine, and human-word flags, leaving every copied model placeholder unresolved. Consequently green-join stops at configuration validation, and the no-machine scenario can stop on roster placeholders before reaching the machine refusal it claims to assert.

EVIDENCE: metasystem/plans/fleet-join-bootstrap-design.md lines 454-459 define the append and lines 463-480 define the flags and assertions. metasystem/metasystem.conf line 5 already defines metasystem.runtimes. metasystem/internal/config/validate.go lines 43-59 reject duplicate committed keys. The template specified at design lines 297-318 contains unresolved model placeholders, while step 2 at lines 193-207 substitutes only supplied flags and names only evidence-root substitution.

## FJB-R1-007-FAKE-BED-CANNOT-PROVE-ENROLLMENT — high, material=True

CLAIM: Even after repairing its configuration, the fake-runtime fixture cannot prove enrollment or green up as designed. The steward owner deliberately excludes fake-runtime repositories before minting identity.json, so temporary steward arm reports “not armed” without creating the identity that scenarios 1 and 6 require. The following internal step-6 up then sees no enrollment and cannot be rescued by the later explicit-pair up invocation. Running outside the delegate sandbox does not change this code-level exclusion.

EVIDENCE: metasystem/plans/fleet-join-bootstrap-design.md lines 463-491 require identity.json, armed up, and generation 2. metasystem/internal/steward/runner.go lines 362-387 classifies metasystem.runtimes=fake as excluded and returns before notification, identity minting, or runner launch; identity minting begins only at lines 411-423. Step 6 still runs inside join at design lines 255-263, before the fixture’s separately described explicit-pair up command.

## FJB-R1-008-CORRECTION-ROUND-EXCEEDS-RESERVATION — medium, material=True

CLAIM: The correction-round reservation plan is arithmetically inadmissible. Two default-cap launches already consume all 240 reserved minutes. Launching the correction at cap 60 afterward consumes 300 minutes, not a three-launch fit. The same defect affects slice 1’s suggestion that a later revision can simply use cap 60 after its two 120-minute launches. To preserve correction capacity, earlier launches must be capped lower before they reserve, or the goal limit must be raised.

EVIDENCE: metasystem/plans/fleet-join-bootstrap-design.md lines 505-510 correctly state cumulative cap accounting, but lines 529-533 and 535-542 then propose a later cap-60 launch after two cap-120 launches. metasystem/internal/dispatch/budget.go lines 342-369 adds every launch cap, including terminal launches, and metasystem/internal/dispatch/admission.go lines 155-163 refuses a proposed cap larger than the remaining allowance.

## FJB-R1-009-HOST-REFUSAL-MISNAMES-EFFECTIVE-REMOTE — medium, material=True

CLAIM: The script-owned host refusal is false for a configured non-default remote and incomplete for another declared precondition. Step 0 checks the effective goal.sync-remote but hardcodes “no remote named origin”; when configuration names another missing remote, the message directs the operator to add an irrelevant origin. The same step declares git on PATH as a precondition but specifies no refusal or next command for missing git despite claiming every missing precondition has exact text.

EVIDENCE: metasystem/plans/fleet-join-bootstrap-design.md lines 156-184 promise exact refusals, read the configured remote, and then hardcode origin. metasystem/internal/goal/txn.go lines 46-60 show goal.sync-remote can select any configured name. The Go-toolchain and remote failures have specified text, but the declared git prerequisite has none.

## Critic-declared gaps

- No repository record describes what the m0 and m0b guest clones actually hand-fixed. The design’s historical claim that their repair was substantially goal fetch remains unverified and must reopen if primary seat evidence contradicts it.

- The brief states that origin carries leftover m0 accepted and materialized-base references. Network access was not used to query origin, so their remote existence is treated as supplied context. Static search does prove that current production code neither fetches nor reads a machine-specific metasystem reference namespace.

- The read-only mandate prohibited Go execution. All control-flow conclusions are source-proven, but no proposed fixture or fresh-clone command was executed.

- No completed local design-critic record supports the design’s borrowed critique duration estimate. The design labels this limitation correctly; no stronger estimate was inferred.

- The brief attributes the existing-owner rule partly to architecture.md, but that map states only the Go-versus-shell boundary. The explicit existing-owner questions are in project-adaptation.md and take-a-step-back/SKILL.md; the owner decision was assessed against those supplied authorities.
