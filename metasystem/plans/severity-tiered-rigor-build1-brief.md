Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal severity-tiered-rigor)
Date: 2026-09-03

# Goal

Build PART ONE of the tiering machinery, "the tier at intake", from
metasystem/plans/severity-tiered-rigor-design.md revision 3 (the section
"Revision 3 (2026-09-03): the review folded" and its "Build list,
revision 3"; revision 2's points 1 to 3 as amended there). The design's
closing review (metasystem/records/misc/severity-tiered-rigor-critique-r3.md)
left eight material findings; by Wido's stop criterion (R-55-m1, in
metasystem/memory/rulings.md) the design loop ended and the agreed
parts build now, with the four findings that name part-one files
carried below as BINDING TEST OBLIGATIONS: each becomes at least one
fixture or Go test that proves the property the critic demanded, and
the implementation is shaped so that test passes. Where a finding
contradicts revision 3's text, the finding wins and you say so in the
return. Zero judgment calls beyond that; where the design and the
findings are both silent, stop and report the gap.

# Workspace

The delegate worktree the dispatcher created for this job. Build on
main as it is (the path-class manifest's second part and the second
landing bar are landed).

# The change (part one)

Per the build list, revision 3, part one: the `Tier` field, its digest
and edit rules (01); `goal classify-sweep` and the `TierLaw` marker
(02); the five-member budget tuple, the token quadruple, box assignment
and `--review-round-limit` (03); the retired-key tombstone (04);
`goalTier` on chain roots, the refusals by tier and the
hazard-under-tier-1 refusal (12). Files named there plus the ones the
findings below add (metasystem.conf tier boxes, internal/goal/root.go,
internal/dispatch/stop.go, record.go, cap.go, cmd/metasystem/main.go and
dispatch_verbs.go).

# The two decisions that are Wido's

Revision 3's section "STR2-RULING-CONFLICT-06 and
STR2-RESERVATION-RECOMMENDATION-14: Wido's" states them. Build the
rest either way: implement the `Validate` bound and the box minutes
with the design's RECOMMENDED option as the configured default, in
metasystem.conf, so his one-word answer is a config change, not a
rebuild. Name the two keys in the return.

# Binding test obligations (the closing review's findings on part one, verbatim)

## STR3-MIGRATION-BOOTSTRAP-01 (critical)

Claim: The classification migration cannot bootstrap the current approved ledger. Revision 3 changes ApprovalDigest before classify-sweep reads existing tierless approvals, so ParseFile and ValidateApprovalRecord reject their legacy digests before the sweep can rebind them. It also tells Edit to refuse tier changes on approved, claimed, or parked goals while telling the sweep to perform those exact edits, and it never converts a legacy four-member budget's temporary tier-3 round count to the selected tier. The named test obligation must cover a current approved and claimed goal through preview, confirmation, tier-specific tuple normalization, and post-marker validation. This changes metasystem/internal/goal/file.go, metasystem/internal/goal/approval.go, metasystem/internal/goal/verbs.go, and metasystem/internal/goal/root.go.

Evidence: Revision 3 lines 281-306 impose the conflicting digest and edit rules; lines 316-319 assign tier-3 rounds while a legacy goal remains tierless. The repository currently contains five approved tierless goal records, and file.go lines 452-492 validate their stored digest during parsing.

## STR3-TIER-SNAPSHOT-PLUMBING-02 (critical)

Claim: The immutable goalTier snapshot is wired through the wrong reader. Active fresh and follow-up dispatch use job goal-binding and ResolveGoalBinding, not ResolveGoalRevision; the design also omits the command flags and RecordSetup equality needed to prevent the pending husk and final record from disagreeing. The named test obligation must prove that fresh and follow-up roots receive one claimed-revision tier and that setup refuses a mismatched tier. This changes metasystem/scripts/agents/dispatch.sh, metasystem/internal/dispatch/stop.go, metasystem/internal/dispatch/build.go, metasystem/internal/dispatch/record.go, and metasystem/cmd/metasystem/dispatch_verbs.go.

Evidence: dispatch.sh lines 1328-1334 and 1774-1780 read job goal-binding. stop.go lines 31-63 own that binding. dispatch_verbs.go lines 838-855 expose ResolveGoalRevision only as a separate scalar command, contrary to Revision 3 lines 288-292.

## STR3-MISSION-CAP-BYPASS-07 (high)

Claim: Neither reserved-minute option covers mission-dispatched jobs, so one word from Wido does not yet close the reservation decision. Mission dispatch gets its cap from the signed mission fence and bypasses ResolveCap; option “ceiling” therefore does not enforce 120 minutes there, while option “computed” walks cap.min configuration keys and also omits fence.job-cap-min. A lawful mission cap above the box can still stall the goal ladder. The named test obligation must dispatch a goal-bound mission above 120 minutes and prove either a global refusal or an approved goal-budget expansion. This changes metasystem/scripts/agents/dispatch.sh, metasystem/internal/dispatch/cap.go, and metasystem/internal/mission/fence.go.

Evidence: dispatch.sh lines 1565-1574 route mission caps through mission_fence and only non-mission caps through ResolveCap. Revision 3 lines 481-488 makes ResolveCap or cap.min keys the sole mechanism for its two choices.

## STR3-BUILD-LIST-COVERAGE-08 (high)

Claim: The replacement build lists do not name every required file and function. Unlisted mandatory artifacts include metasystem/metasystem.conf for the tier boxes; metasystem/internal/goal/root.go for RootRecord, parseRootField, and RenderRoot of TierLaw; metasystem/internal/dispatch/stop.go for GoalBinding; metasystem/internal/dispatch/record.go for RecordSetup and dedicated metadata protection; metasystem/internal/dispatch/cap.go for the recommended cap option; metasystem/cmd/metasystem/main.go for every new verb; and metasystem/cmd/metasystem/dispatch_verbs.go in part one for goalTier plumbing. Part three names the directory metasystem/internal/pathclass rather than the files and Parse or Manifest functions it requires. This changes the Revision 3 build lists in metasystem/plans/severity-tiered-rigor-design.md and the named implementation artifacts.

Evidence: Revision 3 lines 490-518 omit those artifacts despite earlier sections explicitly introducing TierLaw, new goal, job, and landing verbs, ResolveCap changes, and a new pathclass row kind. Current main.go is the closed command router, root.go owns the root grammar, record.go owns setup equality and protected metadata, and pathclass/pathclass.go owns Manifest parsing.

# Findings that bind parts two and three (for your awareness; do not build them)
- STR3-DESIGN-OUTPUTS-CONTRACT-03: The design-chain subject set is not mechanically bound to the reviewed design. The required --outputs file has no byte grammar, path normalization, parser owner, digest, or rule proving its contents came from the design's build list; a call
- STR3-DEFERRED-OBLIGATION-WIRE-04: The deferred-obligation wire and inputs remain undefined. ReviewObligation uses whitespace-separated artifact=<ref> and test=<text> even though NEW artifact references and ordinary test descriptions contain spaces; the close command supplie
- STR3-ACCEPTED-RISK-TRANSITION-05: There is no complete transition that accepts a severe or unproven finding as risk. The close table says such an unresolved entry always refuses, while AcceptedRiskDecision has no command surface, finding-selection contract, or named coordin
- STR3-TIER1-RECEIPT-PROOF-06: The Tier 1 test receipt is only labelled with a tree; it is not proof that the command ran against that tree. A caller can supply any existing candidate SHA while executing the command in a different working-tree or index state, and observe

# Gate

`cd metasystem && go build ./... && go test ./internal/goal/... ./internal/goalbudget/... ./internal/config/... ./internal/dispatch/... -count=1` green;
`bash metasystem/scripts/agents/goal-cli-fixtures.sh` green; gofmt clean on
every Go file touched. Do not run scripts/agents/path-class-fixtures.sh
(ripgrep is absent on this host).

# Constraints

Wall-clock budget: 90 minutes. DESIGN-BEARING reach. Non-goals: parts
two, three and four. Declare the boundary as exactly the files you
touch, with the metasystem/ prefix.

# Expected Return

Per the implementer schema: the boundary; for each of the four
obligations the test that proves it; the two config keys; the gate
commands and their observed results.

# Gap Rule

stop and report a gap; never fill it silently.
