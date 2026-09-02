# Never-idle analysis critique — round 1 (Sol)

Chain: analysis (plans/never-idle-analysis.md) -> critic never-idle-crit1 (codex {'effective': 'gpt-5.6-sol', 'requested': 'gpt-5.6-sol'}, reviewed commit e368e7859553be41b6f8e5d289954cc0addab292), 2026-09-02. 9 material findings. Full return: artifacts/agents/never-idle-crit1/rounds/1/return.json.

## NIA-R1-HOOK-CANNOT-OWN-IRONCLAD-GUARANTEE — critical, material=True

CLAIM: The analysis assigns the iron-clad prevention guarantee to a mechanism that cannot provide it. It says, “If all six bound goals landed as designed, part (1) would hold on a Claude seat whose hook is loaded” and leaves turn-verdict-hardening as owner of parts (1) and (4). Claude Code overrides a Stop hook after eight consecutive blocks without progress, so the gate is only a bounded first line; runtime-neutral steward re-engagement is the actual floor. Implementing the proposed ownership and ordering would preserve a false invariant and build the hook side before the only mechanism capable of recovering after the harness overrides it.

EVIDENCE: The disagreeing analysis text is at metasystem/plans/never-idle-analysis.md:502-505 and 589-603. Metasystem/records/misc/idle-terminal-critique.md:3-15 records the eight-block architectural wall, and metasystem/plans/goals/never-idle-ironclad.md:6 adopts the steward-first consequence. The current official Claude Code hooks guide independently documents the eight-block override.

## NIA-R1-STOP-PATHS-AND-FIXTURES-OMIT-HARNESS-ESCAPES — critical, material=True

CLAIM: The twelve-path map and its proof arc omit four material escape paths introduced or exposed by the declared Claude landing: the harness block cap, failure of the launcher before the repaired script starts, a non-empty but malformed or truncated response, and replay of an honest unused human-stop marker across a failed or absent session retirement. None is equivalent to P2's failure after the script begins or P5's runtime timeout. The proposed fixtures consequently cannot prove fail-closed behavior on every hook failure path or that the recorded stop word is the only quiet exit.

EVIDENCE: Metasystem/plans/never-idle-analysis.md:52-73 claims the stop-path enumeration, and lines 563-581 claim specimen-replaying fixtures. Metasystem/records/misc/idle-terminal-critique.md:11-33 supplies grounded counterexamples for all four omitted paths. Row D's hook-budget-verdict-hang fixture at metasystem/plans/never-idle-analysis.md:574 does not replay launcher failure, partial output, the eight-block override, or marker replay; it also cannot honestly be called the observed m1 cause because the analysis itself says P4 versus P5 is undecidable at lines 447-452.

## NIA-R1-NUDGE-AND-RELAUNCH-ALREADY-HAVE-AN-OWNER — high, material=True

CLAIM: The hole map wrongly declares seat nudge and seat relaunch unowned and proposes two new goals for work already assigned to idle-every-runtime-enforcement. That goal explicitly owns active re-engagement or restart of an idle backlogged seat on every runtime, gated by marking mode and Law 2. Following the analysis would create competing decision owners for the same acting behavior.

EVIDENCE: Metasystem/plans/never-idle-analysis.md:542-555 labels nudge and relaunch unowned, and lines 610-611 propose seat-channel-and-nudge and seat-relaunch goals. Metasystem/plans/goals/idle-every-runtime-enforcement.md:4-6 explicitly assigns the steward both active re-engagement and restart. The analysis should identify missing contracts or slices inside that owner, not create duplicate owners.

## NIA-R1-GATE-DISPOSITION-CONTRADICTS-WIDOS-FORK — high, material=True

CLAIM: The proposed collision disposition is the opposite of Wido's recorded fork and rests on a false current-state claim. The analysis says idle-with-backlog-alarm's gate and marker “should yield” to turn-verdict-hardening and says both goals are claimed. Wido instead chose to land the alarm goal's Claude gate now; turn-verdict-hardening is queued and explicitly released. An implementer following the analysis would edit the wrong owner or rebuild the Claude gate after it lands.

EVIDENCE: The rejected disposition is at metasystem/plans/never-idle-analysis.md:594-603. Metasystem/plans/goals/idle-every-runtime-enforcement.md:4 records “Wido chose fork (a): land the claude turn-exit gate now”; metasystem/plans/goals/never-idle-ironclad.md:6 says not to rebuild that landed piece. Metasystem/plans/goals/idle-with-backlog-alarm.md:3 and 11 show that goal claimed, while metasystem/plans/goals/turn-verdict-hardening.md:3 and 18-19 show hardening queued after release.

## NIA-R1-SLICE-RESERVATIONS-EXCEED-OR-OMIT-THE-CAP — high, material=True

CLAIM: The split's “at most 240 reserved minutes” assertion is arithmetically false for three rows and unsupported for six others. Rows C, D, and J each combine two hardening slices that the source design budgets as separate 240-minute implementation-and-correction chains, permitting 480 reserved minutes per analysis row. Rows B, E, F, G, H, and I state no implementer, critic, correction, and re-critique reservation at all. These rows cannot be dispatched with the promised correction round intact.

EVIDENCE: Metasystem/plans/never-idle-analysis.md:563-565 promises every slice is at most 240 reserved minutes; lines 573-574 and 580 combine 1a+1b, 2a+2b, and 4a+4b. Metasystem/plans/turn-verdict-hardening-design.md:1197-1205 defines one 240-minute chain per slice, and lines 1248-1254 list each member as a separate capped slice. The analysis provides no equivalent chain arithmetic for its six new rows.

## NIA-R1-SPLIT-DEPENDENCIES-ARE-OUT-OF-ORDER — high, material=True

CLAIM: The dependency table is not topological. Freshness row K depends only on C even though hardening slice 3 follows 2b in row D; human-stop row J depends on C and D even though hardening slices 4a and 4b follow freshness slice 3. Rows G and I require watch-verb marking-mode and Law 2 promotion but omit that dependency from the table, and row H consumes seat-mutual-awareness's authenticated second channel without depending on its delivery. Executing the stated order can therefore build consumers before their prerequisites.

EVIDENCE: The faulty dependencies appear at metasystem/plans/never-idle-analysis.md:577-581, while the prose acknowledges the omitted watch and second-channel relationships at lines 583-609. Metasystem/plans/turn-verdict-hardening-design.md:1250-1254 requires 2b after 2a, freshness after 2b, 4a after freshness, and 4b after 4a. Metasystem/plans/goals/watch-verb.md:6 says acting classes still require marking-mode trials and Law 2 records; metasystem/plans/goals/seat-mutual-awareness.md:6 shows its authenticated inbound channel is not landed.

## NIA-R1-STRANDED-MESSAGE-FIXTURE-MISSES-INGRESS — high, material=True

CLAIM: The proposed stranded-message fixture starts downstream of the specimen's failure and can pass while the original defect remains. The specimen is an instruction that never leaves the human-facing input box; row G tests a steward write to a declared pipe and row H tests an already-written order that receives no acknowledgment. Neither exercises ingestion from the remote-control input box into the primary channel, nor proves recovery when that ingress silently accepts but does not send.

EVIDENCE: P10 at metasystem/plans/never-idle-analysis.md:71 defines messages that “sit unsent in a seat's input box.” The fixtures at lines 577-578 begin with the steward producing or writing an order. The analysis also admits at lines 470-482 that no independent record of the event exists. A downstream pipe fixture is useful, but it is not a replay of this specimen and cannot certify the claimed channel guarantee.

## NIA-R1-STEWARD-HUMAN-STOP-LIFECYCLE-IS-UNDEFINED — high, material=True

CLAIM: The steward-side human-stop lifecycle is undefined. Row J consumes a single-use Stop marker from a recurring idle verdict, but does not state how long the human's stop remains effective, what event resumes work, or how later steward ticks and dead-session relaunches avoid immediately undoing the human's direction after the marker is spent. The hardening design intentionally makes the marker authorize one Stop only. An implementer must therefore guess between a one-tick exemption, indefinite suspension, reminting, or a separate durable stopped state.

EVIDENCE: Metasystem/plans/never-idle-analysis.md:580 adds consumption from the steward path without a duration or resume contract. Metasystem/plans/turn-verdict-hardening-design.md:1030-1033 says the Stop decision is the only consumption boundary, and its fixture at line 1254 requires the next Stop to block again. Metasystem/records/misc/idle-terminal-critique.md:29-33 additionally proves that failed session retirement can replay an honest unused marker, so simply sharing that marker with the steward does not establish guarantee part (4).

## NIA-R1-ACTIVE-STEWARD-LACKS-SEAT-SCOPED-ADMISSION — high, material=True

CLAIM: The analysis misses seat-scoped admission and arbitration for its acting steward. It defines claimable work through goal.Next, which is machine-scoped, then proposes nudging or relaunching a particular seat from unclaimed backlog. Its fixtures contain only one seat. With two seats on one machine, the plan does not determine which seat is eligible, whether both are nudged, how machine quota is respected, or how a relaunch avoids duplicating a live sibling. This is an unowned hole at the exact authority boundary where the steward begins acting.

EVIDENCE: Metasystem/plans/never-idle-analysis.md:54-57 adopts goal.Next; metasystem/internal/goal/project.go:81-123 computes its verdict for a machine, not a machine-and-lineage seat. Analysis rows E, G, and I at lines 575 and 577-579 use one live or fake seat and state no arbitration rule. The adjacent hardening design recognizes the same topology by requiring two-seat fixtures at metasystem/plans/turn-verdict-hardening-design.md:1249. Without an equivalent steward contract, the new active owner can act on the wrong seat or more than once.

## Critic-declared gaps

- The correction note says the Claude gate is landed or landing, but at reviewed commit e368e7859553be41b6f8e5d289954cc0addab292 main contains no implementation commit and lacks metasystem/internal/goal/sessionstop.go, metasystem/cmd/metasystem/session_stop.go, and metasystem/internal/goal/turnverdict_idle_test.go. The landing branch or uncommitted implementation was unavailable, so code-specific terminal-critique paths were checked against its landed record and official runtime documentation, not against the missing implementation bytes.

- Other-seat records are only partly independently verifiable. Metasystem/records/misc/idle-rootcause-critique.md, idle-loss-2026-08-31.md, idle-loss-2026-09-01.md, and seat-stop-analysis.md exist on main and support their relayed narratives. The m0 raw alert artifact named by the root-cause critique is absent, the two m0b stop narratives have no independent incident record, and the remote-control stranded-message specimen has no record beyond the goal and analysis brief.

- The files prove that m1 recorded no Stop attempt after 2026-08-30 and no verdict after 2026-08-27, but they do not distinguish an unloaded hook from a hook killed before its first durable write. The analysis correctly leaves P4 versus P5 undecidable.

- The files prove m1's idle intervals but not that a permission classifier or decision-ask caused them. Those causal labels remain goal-reported specimens rather than independently replayable records.

- Per the user's read-only instruction, no Go tests or executable fixtures were run. Fixture conclusions are from reading their specified inputs and assertions.
