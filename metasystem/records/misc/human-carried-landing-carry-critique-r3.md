# Critique register: human-carried-landing-carry design, read 3

Job hcl-crit3-20260911 (codex, gpt-5.6-sol, design-critic) reviewed revision 5 at local scaffold 316c99b2d (sha256 138f6bb7311dec3c648fb49124234ef7255d9ff25441605b37a25995d4403a04). Material findings: 4 of 4. Projected verbatim from artifacts/agents/hcl-crit3-20260911/rounds/1/return.json.

## HCL-C-33 (critical)

**Claim.** HCL-C-33, the reservation-row finding, is reopened in HCL-COUNTER-08 and HCL-TRANSACTION-06. The landed-but-unrecorded debt arm can never recognize its subject: it scans only an “open word,” while the exact Carry trailer it seeks already makes that word consumed, and open words are defined as unconsumed and unexpired. After seat A pushes and its reservation is abandoned or expires before the carried row is written, seat B sees neither an open reservation nor an obligation, excludes A's consumed word from the trailer scan, and can reserve and stack a second carried landing. If this stands, the third debt predicate must scan every proven carry word that has a reachable Carry trailer but no carried row regardless of consumption or expiry, and the two-seat fixture must cover both abandonment and expiry after the push.

**Evidence.** metasystem/plans/human-carried-landing-carry-design.md:599-615 makes a Carry trailer consumption and then restricts incomplete-record debt to another open word. metasystem/plans/human-carried-landing-carry-design.md:1019-1035 repeats that the trailer consumes the word. metasystem/plans/human-carried-landing-carry-design.md:1498-1501 defines an open word as unconsumed and unexpired. metasystem/plans/human-carried-landing-carry-design.md:1525-1534 and 1566-1578 remove an abandoned or expired reservation from debt and rely on the contradictory trailer arm. The proposed HCL-33-TRAILER-WITHOUT-ROW-IS-DEBT fixture at metasystem/plans/human-carried-landing-carry-design.md:930-935 would therefore fail under the written predicate.

## HCL-C-25 (high)

**Claim.** HCL-C-25, the full-payload replay finding, is reopened in HCL-TRANSACTION-06. The replay comparison names fourteen reason fields but omits the goal target. The journal defines targets as part of complete normalized intent, the carried row stores the goal in targets, and the counselor record derives its goal from that row. A fresh-operation replay with the same approved carry reference but a different target can therefore be classified AlreadyApplied and terminalize an intent that does not equal the durable row. If this stands, replay must compare the intent's unique goal target with the existing row's unique target before returning AlreadyApplied, and the mismatch table needs a goal-target case.

**Evidence.** metasystem/internal/goal/journal.go:57-64 defines Targets as part of complete normalized command intent. metasystem/plans/human-carried-landing-carry-design.md:974-979 stores the goal in the carried intent's Targets. metasystem/plans/human-carried-landing-carry-design.md:1091-1109 searches for a row on any live or done goal but compares only fourteen fields, none of them the target. metasystem/plans/human-carried-landing-carry-design.md:1433-1460 puts the goal in the row's targets and in the counselor line. The fixture table at metasystem/plans/human-carried-landing-carry-design.md:1209-1216 likewise covers only the fourteen reason fields.

## HCL-C-03 (high)

**Claim.** HCL-C-03, the base-judge blindness finding, is reopened in HCL-LANDING-05. The purported owner fence omits metasystem/internal/governance/** even though that package owns the authority-outcome constants and recorded-authority validation imported by the goal ledger parser. A candidate can change that authority wire or its validation while a failed live engine falls back to a base judge that cannot see the change; step 11 would allow the fallback instead of asking. If this stands, the fence and its table fixture must include metasystem/internal/governance/** at minimum and must cover every compiled package that owns a ledger authority wire read by the base judge.

**Evidence.** metasystem/plans/human-carried-landing-carry-design.md:193-205 places the new HUMAN_AUTHORITY_PROVEN outcome in metasystem/internal/governance/types.go. The fence list at metasystem/plans/human-carried-landing-carry-design.md:618-642 omits that directory. At HEAD, metasystem/internal/goal/file.go:13-20 imports governance, and metasystem/internal/goal/file.go:1436-1440 delegates non-channel recorded-authority validation through it. metasystem/internal/governance/types.go:91-158 owns the accepted outcomes and their recorded validation.

## HCL-C-34 (high)

**Claim.** HCL-C-34, the test-execution finding, is reopened in Contract ownership before the first canary. The design puts all new cmd/metasystem carry fixtures in carry-landing-standard, but its explicit obligation mapping adds that group only to proof-and-landing. The new human-authority-and-channel surface owns channel_verbs.go, dispatch-goal-mission owns the goalsync files, and testing-policy owns main.go; a focused selection for those owners therefore need not execute the command-package carry tests. This contradicts the stated function-name inclusion fixture and leaves implementers to choose different mappings. If this stands, split the command tests by owning surface or attach carry-landing-standard and its obligation to every surface owning a command file containing those fixtures, then fixture each exact owner path.

**Evidence.** metasystem/plans/human-carried-landing-carry-design.md:1823-1830 assigns channel command files to human-authority-and-channel, and metasystem/plans/human-carried-landing-carry-design.md:1843-1846 assigns goalsync files to dispatch-goal-mission. metasystem/plans/human-carried-landing-carry-design.md:1876-1893 puts every cmd/metasystem fixture in carry-landing-standard but maps the landing obligation only to proof-and-landing. HCL-34 at metasystem/plans/human-carried-landing-carry-design.md:1910-1921 requires every TestHCL function in the package to be selected. At HEAD, metasystem/testing.json:5-7 confirms the command files have different surface owners, while metasystem/internal/testpolicy/select.go:162-176 selects groups only from affected surfaces and their critical-obligation providers.

## Gaps the critic named

- No fixture bed, test, build, or live proof was run because the review brief explicitly prohibited beds and requested read-only design critique.
- The runtime is read-only, so metasystem/records/misc/human-carried-landing-carry-critique-r3.md was not created; this structured return is the register for coordinator projection.
- The provider tool catalog and independent context isolation remain unobserved, so the critique is advisory about runtime independence.
- The carried implementation does not exist at HEAD. The interleaving, replay, and selection outcomes are design-level inferences grounded in the existing transaction, journal, authority, and testing-contract code.
- The design leaves how metasystem/records/counselor/*.jsonl reaches origin untraced. The closing code read should trace that existing append path and verify the carried line reaches its intended consumer; this was retained as a code-read note because revision 5 explicitly reuses the existing counselor mechanism rather than defining a new publication contract.
- The hand-maintained shell refusal register remains expressly incomplete. The closing code read should compare every new exit-3 shell site with the resulting ShellRows, but this does not change the carried-landing control flow designed here.

## Coordinator dispositions (m1d, 2026-09-11, binding on fold 3 = revision 6, the last fold; no fourth design read, the four are named checks of the implementation's closing read)

All four accepted, in the critic's own words:
- HCL-C-33 (critical): the third debt predicate scans every proven carry word with a reachable `Carry:` trailer and no `carried` row, regardless of consumption or expiry; the two-seat fixture covers abandonment and expiry between the push and the row.
- HCL-C-25: replay compares the intent's goal target with the row's target before AlreadyApplied; the mismatch table gains the target case.
- HCL-C-03: the owner fence includes internal/governance/** and every compiled package that owns a ledger authority wire the base judge reads.
- HCL-C-34: the command-package carry fixtures are split by owning surface, or `carry-landing-standard` and its obligation are attached to every surface that owns a command file holding them; the selection fixture asserts execution for each owner.
