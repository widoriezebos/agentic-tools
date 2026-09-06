# brain seat design critique — round 2 (revision 2)

Chain: revision 2 (landed 44efc781e as records/misc/brain-seat-design-revision-2.md, sha256 ae0a1e5f7438eee1d4544dc8e4175f1fada7d8ce4781a40bd65) -> critic brain-crit2-20260906 (design-critic, read-only runtime; the return named the job id and validated). Reviewed commit 44efc781e473658fec51a4896e755d573159c700. 10 material findings. The coordinator carried the return here verbatim because the critic wrote no register file.

## BRAIN-R2-01 — high, material=True

CLAIM: Round-one finding BRAIN-R1-01, which requires one brain per fleet, was renamed rather than folded. Revision two explicitly permits a second host to declare another brain and offers only later visibility. The promised visibility also has no automatic producer or cadence, and the host pointer has no lock or compare-and-swap contract for concurrent declarations. Revision two is not ready for the build.

EVIDENCE: metasystem/records/misc/brain-seat-design-revision-2.md lines 162-175 describe an unsynchronized host-local pointer, while lines 192-198 concede that cross-host uniqueness is not fenced. Its sequential fixture at lines 503-504 cannot expose a declaration race. The accepted round-one disposition in metasystem/records/misc/brain-seat-critique-r1.md lines 76-81 promised publication at boot and every turn end, but revision two instead refers to the status post's own cadence. metasystem/cmd/metasystem/channel_verbs.go lines 47-90 show that status publication happens only when someone invokes channel status with --post.

## BRAIN-R2-02 — high, material=True

CLAIM: The proposed fleet identity is neither stable nor safe to publish. A raw Git remote URL can differ between hosts for the same ledger, change when credentials or transport spelling change, and contain embedded credentials. Treating that string as identity can corrupt a valid declaration after rotation, miss duplicate brains, and disclose a secret in model context and the shared channel.

EVIDENCE: metasystem/records/misc/brain-seat-design-revision-2.md lines 125-141 define fleet identity as a remote URL with only a trailing slash and .git removed; lines 194-198 and 246-247 emit it. metasystem/internal/goal/txn.go lines 46-60 stores only the configured remote name and imposes no URL form or credential prohibition. The repository already has a transport-independent ledger identity reader in metasystem/internal/goal/actor.go lines 35-54, disproving the design's premise that raw URL equality is the only available fleet identity.

## BRAIN-R2-03 — high, material=True

CLAIM: A human can designate a checkout that still owns claims or running jobs, immediately stranding its node work. The declaration deliberately disables landing and dispatch but has no quiescence precondition, so conversion itself can make fleet work wait on repair or reassignment.

EVIDENCE: metasystem/records/misc/brain-seat-design-revision-2.md lines 166-184 list declaration refusals only for declaration and pointer state and acknowledge that declaration halts the node's dispatch and landing. Lines 407-408 merely report existing claims after designation. The brain-absent-node-proceeds fixture at lines 506-506 uses a separate clone and therefore cannot detect an in-flight job or claim stranded in the checkout being converted.

## BRAIN-R2-04 — critical, material=True

CLAIM: The boot-failure fallback has no safe source of designation once the engine or brain boot command is unavailable. An unconditional shell fallback would inject the brain role into ordinary nodes; no fallback leaves a real brain uninstructed. Revision two also contradicts itself by requiring the hook to parse the packet and decide this fallback while later claiming that the hook only relays engine decisions.

EVIDENCE: metasystem/records/misc/brain-seat-design-revision-2.md lines 299-304 tell the hook to treat any failed or malformed boot output like a corrupt brain declaration, but do not name a declaration read that remains available in that failure. Lines 469-470 say the hook decides nothing. The current early branch in metasystem/scripts/agents/supervision-hook.sh lines 350-357 exits with no SessionStart output when the engine binary is absent. The fixture at design line 495 tests only a declared bed with a replaceable boot subcommand, not an absent engine or an undeclared checkout under the same failure.

## BRAIN-R2-05 — high, material=True

CLAIM: Round-one finding BRAIN-R1-05 remains open because the five-second boot deadline is cooperative, not a bound. A single ledger projection, file read, or group of fewer than 200 large files can exceed both the designed deadline and the runtime's fifteen-second hook timeout before any buffered packet reaches the session.

EVIDENCE: metasystem/records/misc/brain-seat-design-revision-2.md lines 238-242 promise a five-second deadline, but lines 299-300 check time only between sections and every 200 enumeration entries. The packet is emitted only after brain boot returns. metasystem/scripts/enforcement/claude-code-hooks.json lines 4-12 gives the entire SessionStart command fifteen seconds. The deadline fixture at design line 494 uses a one-millisecond limit with ordinary local inputs and does not provide a stalled or slow reader, so it cannot prove the wall-clock property.

## BRAIN-R2-06 — high, material=True

CLAIM: Round-one finding BRAIN-R1-04 is still open at the mandatory header. Declaration fields and the packet path have no maximum length, yet the header is never cut. The brain boot command also accepts any byte bound, including the 300-byte fixture, without a minimum or a shorter bounded fallback. An implementer must still choose between exceeding the declared bound and dropping mandatory text.

EVIDENCE: metasystem/records/misc/brain-seat-design-revision-2.md lines 124-141 validate declaration strings only as non-empty. Lines 244-259 always include the complete nickname, fleet URL, declarer, date, and path. The bound fixture at line 493 demands a 300-byte response but does not constrain those inputs or state what happens when even the header plus DOES NOT FIT diagnostic exceeds 300 bytes. Long asks were capped, but the unbounded mandatory fields that generate the same contradiction were not.

## BRAIN-R2-07 — medium, material=True

CLAIM: The standing instruction is not designed across the complete Claude SessionStart lifecycle. Claude emits SessionStart after compaction, but the shipped matcher and revision-two fixture omit that source, so a long-lived brain can continue after context compaction without a specified reinjection of its standing role.

EVIDENCE: metasystem/scripts/enforcement/claude-code-hooks.json line 6 matches only startup, resume, and clear. The official Claude Code hooks reference names compact as a SessionStart source. Revision two changes payload composition but never changes the matcher, and its fixtures invoke the hook directly without a compact-source case. Because additional context is conversation content rather than a persistent instruction file, preservation across compaction cannot be silently assumed.

## BRAIN-R2-08 — critical, material=True

CLAIM: The dispatch fence leaves kill-capable and record-mutating dispatcher acts available to the brain. In particular, the brain can run metasystem delegate --cancel, and can directly run dispatch.sh close or reap; none enters the two fenced launch functions. Cancellation can terminate a node process, directly violating the fixed no-delegate and never-a-bottleneck rules.

EVIDENCE: metasystem/records/misc/brain-seat-design-revision-2.md lines 319-330 fences only the direct legacy launch guard plus dispatch_job and follow_up, exempting only breach-stop. metasystem/cmd/metasystem/delegate.go lines 224-229 maps --cancel to the shell's cancel command. metasystem/scripts/agents/dispatch.sh lines 2884-2906 routes cancel, close, and reap independently, while lines 2494-2513 and 2646-2716 show cancellation marking records and winding down process groups. The fixture at design line 496 tests only fresh dispatch through the two launch grammars.

## BRAIN-R2-09 — high, material=True

CLAIM: The claimed universal human-authority seam is not universal, and classification failure is unspecified at the protected boundary. discharge-review-obligation deliberately passes no human name through syncReq, while syncReq currently constructs a human actor before classification and ignores classifier errors. Implementing only the designed check can therefore miss the promised brain-specific refusal or fail open under an unreadable caller classification.

EVIDENCE: metasystem/records/misc/brain-seat-design-revision-2.md lines 345-360 says every listed verb inherits one declaration-keyed refusal, and lines 378-379 specifically claim discharge-review-obligation is covered. In metasystem/cmd/metasystem/goalsync_mutations.go lines 200-215, that command calls syncReq with an empty human name and assigns Actor.Human later only for a positively classified human. Lines 59-76 construct Actor.Human before lease.ClassifyVerb and return the request even when classification errors. The fixture at design line 499 omits discharge-review-obligation and any classifier-error case.

## BRAIN-R2-10 — high, material=True

CLAIM: Round-one proof finding BRAIN-R1-08 remains open. The matrix does not prove corrupt declarations at each guarded act, the newly exposed cancel, close, and reap routes, caller-classification failure, hard deadline enforcement, compact-session reinjection, or fleet-wide uniqueness. Its status assertion is assigned to a fixture script that the governed validation driver does not run. Today's absent brain command would make the new scenarios red, but that coarse failure does not discriminate these designed omissions after implementation.

EVIDENCE: The corrupt fixtures in metasystem/records/misc/brain-seat-design-revision-2.md lines 492 and 502 cover boot, show, and the Stop verdict, while the delegate, landing, claim, and human-word fixtures use only a valid declaration. Lines 496-500 omit the dispatcher and authority cases named above; line 494 does not stall an input; no compact case exists; and line 503 uses one registry home rather than two hosts. Line 505 assigns brain-status-line to channel-fixtures.sh, but searches of metasystem/scripts/validate-metasystem.sh and metasystem/scripts/agents/validate-section-selector.sh find no invocation or section for that script. Only the proposed new brain-fixtures.sh is explicitly promised registration at lines 508-509.

## Reopening triggers the critic named

- BRAIN-R2-01: Reopen if the design still permits two declarations for one fleet, lacks serialization for concurrent declarations, or relies on a status line without a named automatic producer and cadence.
- BRAIN-R2-02: Reopen if raw or transport-dependent remote URLs still determine fleet identity or can reach model context, diagnostics, or a shared status post.
- BRAIN-R2-03: Reopen if declaration can succeed while the target checkout has claims, live jobs, a mission, or other custody that its new fences would strand.
- BRAIN-R2-04: Reopen if any absent, failed, timed-out, or malformed engine path can either start a declared brain without instruction or inject the brain role into an undeclared node.
- BRAIN-R2-05: Reopen if the boot deadline remains cooperative, any individual input operation can exceed it, or the fixture does not exercise a genuinely stalled reader.
- BRAIN-R2-06: Reopen if any mandatory payload component remains unbounded or if an accepted --bytes value can be smaller than the mandatory fallback without a defined refusal or shorter representation.
- BRAIN-R2-07: Reopen if compaction can occur without reloading the brain context or if the runtime matcher and fixture still omit the compact SessionStart source.
- BRAIN-R2-08: Reopen if a declared brain can still cancel, close, reap, or otherwise mutate dispatcher custody outside the explicitly justified breach-stop exception.
- BRAIN-R2-09: Reopen if any command can establish or attempt a human actor outside the declaration-keyed refusal, if caller-classification errors do not fail closed, or if discharge-review-obligation lacks the promised brain-specific remedy.
- BRAIN-R2-10: Reopen if any named fence, failure state, runtime lifecycle, uniqueness boundary, or no-bottleneck contract still lacks a fixture that discriminates that exact behavior and runs in a named governed suite.

## Gaps the critic named

- No live Claude seat was opened, so actual model consumption remains unobserved. The current official documentation supports the designed additionalContext channel, but the deployed Claude version is not pinned.
- The new brain command family and brain fixture script do not exist at the reviewed commit, so future passing behavior could not be executed; the critique examined whether the proposed contracts and fixtures discriminate the required behavior.
- The launcher exposed no session identifier and classified this broad-read run as advisory; session isolation and independent-context provenance remain unobserved.
- The working tree contains unrelated changes, but the reviewed design, round-one critique, and role packet match commit 44efc781e473658fec51a4896e755d573159c700.
- The brief supplied the outputs-manifest digest 791babd225bfe696839accc42c909a65c372d61a32bcf8b725cd131ca70c1e63 but did not name the manifest path, so that digest could not be recomputed.

## Coordinator disposition (m1, 2026-09-07)

All ten are accepted. Revision 3 folds them and is the LAST design round
for this page (two prose budgets are spent; the build then proceeds
behind the fixtures with a code review, per the implementation-first
ruling). Reads per finding:

- BRAIN-R2-01 (uniqueness): the host pointer is written under the lock
  package with compare-and-swap semantics so two concurrent declarations
  cannot both win; the visibility line is produced at brain boot and at
  every brain turn end (the status post is the carrier, the brain is the
  producer); cross-host uniqueness stays a human rule stated as such.
- BRAIN-R2-02 (fleet identity): the identity is the transport-independent
  ledger identity the goal package already reads (internal/goal/actor.go),
  never a remote URL; nothing with credentials is ever emitted.
- BRAIN-R2-03 (quiescence): brain declare refuses while the checkout
  holds a claim or a non-terminal job and prints the release or cancel
  command a node or the human runs first.
- BRAIN-R2-04 (boot without the engine): the hook never decides the
  role. When the engine or the boot verb is unavailable, the hook prints
  one notice naming that failure and injects nothing; a declared brain
  is then visibly uninstructed and the remedy is the rebuild. The page
  drops the contradictory shell fallback.
- BRAIN-R2-05 (hard deadline): the packet's standing-instruction section
  is emitted by a fast path before any other input is read; the rest of
  the composition runs in a bounded child killed at the deadline, and
  the payload says what was cut.
- BRAIN-R2-06 (header bound): declaration fields are capped at declare
  time; the configured byte bound has a minimum below which the engine
  refuses it; the header therefore always fits.
- BRAIN-R2-07 (compaction): the runtime's session-start matcher gains
  the compact source and the payload is re-injected then.
- BRAIN-R2-08 (cancel, close, reap): fenced like launch; breach-stop
  stays the one exemption.
- BRAIN-R2-09 (authority seam): classification failure fails closed at
  the seam; discharge-review-obligation gets the same check by
  construction.
- BRAIN-R2-10 (fixtures): corrupt declaration at every guarded act; the
  cancel, close and reap routes; classification failure; a stalled input
  against the hard deadline; the compact source; two registry homes; and
  the status line joins a script the validation driver runs.

Next: revision 3, then the build brief.
