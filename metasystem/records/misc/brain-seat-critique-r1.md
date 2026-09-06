# brain seat design critique — round 1 (revision 1)

Chain: revision 1 (landed 0d44db3cc as records/misc/brain-seat-design.md, sha256 9933693e8451c42bb4c5a8370ecd16390aad0adad763361dee9f44f5a7023b21) -> critic brain-crit1-20260906 (design-critic, read-only runtime; the return named the job id and validated). Reviewed commit 0d44db3cc94dbee53b9778b61d598dd7f0659877. 8 material findings. The coordinator carried the return here verbatim because the critic wrote no register file.

## BRAIN-R1-01 — high, material=True

CLAIM: The designation does not enforce one brain per fleet. It enforces one pointer per supervision-registry host without recording a fleet identity, so two hosts can declare brains for the same fleet, while two unrelated fleets on one host are incorrectly prevented from each having a brain.

EVIDENCE: metasystem/records/misc/brain-seat-design.md lines 118-122 explicitly concede that the pointer is only a per-host mechanism and that duplicate status posts are merely visibility. The record at lines 73-78 contains machine, declarer, and time but no fleet identity. The fixture at lines 326-327 tests two checkouts under one registry home only; it tests neither two hosts in one fleet nor two fleets on one host. The claimed status post also has no producer, cadence, or record contract elsewhere in the design.

## BRAIN-R1-02 — high, material=True

CLAIM: The declaration reader has no malformed or unreadable-state contract. An implementer can reasonably classify a truncated or invalid brain file as undeclared, which would simultaneously disable boot instruction loading, dispatch and landing fences, claim fencing, and the brain-specific Stop behavior.

EVIDENCE: metasystem/records/misc/brain-seat-design.md lines 80-84 define only exit zero for a valid record and exit three for absence. Lines 193-198 describe brain.Fence as returning refusal text or nothing, with no error result or fail-closed mapping. The declaration lives in mutable ignored state at metasystem/artifacts/agents/brain.json. Missing packet behavior is specified at lines 164-167, but malformed declaration behavior and an atomic-publication requirement are not, and no listed fixture corrupts or makes the declaration unreadable.

## BRAIN-R1-03 — critical, material=True

CLAIM: The goal-authority fence is designed around a false centralization premise, leaving the brain able to perform authority-bearing acts. Goal resume can reauthorize execution, goal set-obligation can mint a governing ruling, goal steal can take a node's claim, and goal set-pin can pin approved work to the brain machine so every node is refused. Several further human-labelled routes also bypass the proposed helper.

EVIDENCE: metasystem/records/misc/brain-seat-design.md lines 48-54 claim every human-only goal verb passes through proveGoalHumanAuthority, and lines 222-230 put the brain refusal there. In current code, metasystem/cmd/metasystem/goalsync_mutations.go lines 802-850 and 1044-1115 implement independent temporary-word and authenticated-channel proof paths for resume and set-obligation. Lines 32-76 turn any --by string into Actor.Human, while lines 1175-1177 and 1252-1254 route steal and set-pin without proveGoalHumanAuthority. metasystem/internal/goal/verbs.go lines 1658-1715 allow a named human actor to displace claims, and lines 2296-2379 allow the named actor to install a pin that the claim path enforces. metasystem/cmd/metasystem/goalsync_verbs.go lines 414-448 provide another independent human-authorized repair route. An implementer adding the refusal only where designed would leave these acts operational, violating never approves, never mints a ruling, and never becomes a bottleneck.

## BRAIN-R1-04 — high, material=True

CLAIM: The boot payload bound is both above Claude's inline context limit and internally unenforceable. A valid designed payload can be externalized to a file instead of entering model context, while mandatory uncut fields can exceed even the design's own 32,768-byte ceiling.

EVIDENCE: metasystem/records/misc/brain-seat-design.md lines 183-189 allow 32,768 bytes and forbid cutting the packet, asks, and held claims. The [official Claude Code hooks reference](https://code.claude.com/docs/en/hooks) says output strings and additionalContext are capped at 10,000 characters; larger values are replaced in context by a short preview and file path. Therefore a permitted ASCII payload between 10,001 and 32,768 characters does not satisfy verbatim unprompted loading. Separately, metasystem/internal/channel/question.go lines 41-75 and 162-193 store Wants without a field bound or single-line rule; its 1,600-rune limit applies only to provider rendering. Forty such asks can exceed 32,768 bytes, and the packet itself has no maximum. The design forces an implementer to violate either the total bound or the never-cut rule.

## BRAIN-R1-05 — high, material=True

CLAIM: Boot has no fail-soft input-error contract or computation-time budget, so ordinary local state failures or a large fleet can consume the entire 15-second SessionStart hook and omit the standing instruction. The missing packet is handled, but failures in every other input are left for the implementer to guess.

EVIDENCE: metasystem/records/misc/brain-seat-design.md lines 147-189 specify successful boot output and only the missing-packet case. They do not map malformed digest cursor, malformed goal ledger, malformed channel question, unreadable job record, absent or malformed census, or an exceeded deadline to a payload. metasystem/internal/narratordigest/digest.go lines 201-249 returns errors for malformed cursor and prefix mismatch. metasystem/internal/channel/question.go lines 119-134 aborts question enumeration on one read or JSON error. metasystem/scripts/enforcement/claude-code-hooks.json lines 4-12 caps the whole start hook at 15 seconds, while the designed fleet and question scans have output caps but no read-count, byte, or elapsed-time budget. A boot command that returns an error is not assigned a hook response, so one implementation can silently start an uninstructed brain.

## BRAIN-R1-06 — medium, material=True

CLAIM: The turn-end design does not unambiguously make the brain's asks and drafts visible. Its literal instruction places the brain summary where the existing goal clause runs, which means Busy, Open, WaitingOnHuman, or Unreadable suppresses that summary; those inputs do not otherwise include channel questions or draft goals. Draft ownership across restarted lineages is also undefined.

EVIDENCE: metasystem/records/misc/brain-seat-design.md lines 248-257 say the goal clause is replaced while every other ladder branch remains unchanged, then promise asks and drafts are reported. In metasystem/internal/goal/turnverdict.go lines 655-739, the goal clause is only the default branch after Busy, Open, WaitingOnHuman, and Unreadable. metasystem/internal/report/scan.go lines 29-100 scans jobs, gates, missions, runs, and plans, not questions or queued draft goals. The listed brain-stop-not-idle fixture at metasystem/records/misc/brain-seat-design.md line 325 seeds only approved unclaimed goals and checks only that the display contains BRAIN SEAT. Finally, the declaration identifies only a machine, while metasystem/internal/goal/verbs.go lines 171-186 records a non-human opening actor as machine+lineage; the design does not say whether a later brain lineage owns the earlier draft. Implementers can therefore choose materially different display placement and ownership rules.

## BRAIN-R1-07 — medium, material=True

CLAIM: A direct forbidden dispatch.sh command never reaches the proposed brain refusal. It exits through the existing legacy-grammar guard first, producing neither the BRAIN_REFUSED typed outcome nor the complete node command promised by the fence contract.

EVIDENCE: metasystem/records/misc/brain-seat-design.md lines 202-210 place the brain check inside dispatch_job and follow_up and call that the path every launch passes. metasystem/scripts/agents/dispatch.sh lines 28-33 exit direct dispatch, follow-up, cancel, or default invocations lacking METASYSTEM_DELEGATE_INTERNAL before the router calls either function. The existing detail merely says to use metasystem delegate. The brain-delegate-refuses fixture at design line 322 invokes only the Go metasystem delegate command, so it cannot detect this direct-command outcome mismatch even though the standing packet explicitly forbids scripts/agents/dispatch.sh.

## BRAIN-R1-08 — high, material=True

CLAIM: The fixture matrix would allow omissions at several explicitly designed seams to pass. It does not prove direct commit-wrapper landing refusal, claim or open-with-claim refusal, the independent authority routes, fleet-scoped uniqueness, malformed-state behavior, provider-safe payload delivery, or ask and draft visibility in the Stop ladder.

EVIDENCE: The table at metasystem/records/misc/brain-seat-design.md lines 317-328 tests land.sh but not commit.sh with --chain or --direct-fix; tests temporary-word approve but not resume, set-obligation, steal, set-pin, classify-sweep, or repair; contains no goal claim or goal open --claim case; tests only one registry home for uniqueness; bounds digest and job counts but not long mandatory asks, a growing packet, or Claude's 10,000-character delivery boundary; and checks only the BRAIN SEAT substring without seeding an ask, draft, prior lineage, or competing ladder branch. The source search found no fixture names for these cases. Because these omissions correspond to named code seams and change what the proof asserts, an implementer could omit required control flow and still pass every listed fixture.

## Reopening triggers the critic named

- BRAIN-R1-01: Reopen if the design still permits a second declaration for the same fleet on another host, or still prevents distinct fleets on one host from each having a declaration.
- BRAIN-R1-02: Reopen if any malformed, unreadable, partially written, or wrong-schema declaration can map to undeclared or lacks a fixture proving fail-closed fences and safe boot behavior.
- BRAIN-R1-03: Reopen if any agent-reachable goal route can establish a human actor, consume relayed or channel authority, resume execution, displace custody, or install a self-pin without the declaration-keyed brain refusal.
- BRAIN-R1-04: Reopen if the payload may exceed the target runtime's inline context limit, or if any mandatory field can exceed the engine's total bound without a specified truncation or reference rule.
- BRAIN-R1-05: Reopen if any local input error, unbounded enumeration, or SessionStart deadline exhaustion can still finish without placing at least the bounded standing packet and an honest diagnostic into model context.
- BRAIN-R1-06: Reopen if another ladder branch can suppress ask or draft visibility, or if a draft opened by an earlier brain lineage on the same machine is not counted by a named and tested ownership rule.
- BRAIN-R1-07: Reopen if a direct dispatch.sh launch attempt from a declared checkout can still exit before the brain-specific typed refusal and complete node remedy are produced.
- BRAIN-R1-08: Reopen if any named fence, failure state, uniqueness boundary, provider bound, cursor isolation rule, or turn-display contract lacks a fixture that fails on today's code and passes only with that behavior present.

## Gaps the critic named

- No live Claude session was opened. Delivery through hookSpecificOutput.additionalContext was checked against the current official Claude Code hooks reference, while the design itself defers live consumption proof to a one-time human check.
- The target Claude Code version is not pinned in the design. If the deployed version has a different output limit or SessionStart schema, that version-specific behavior was not available in the repository.
- The new brain command family does not exist at the reviewed commit, so future fixture pass results cannot be run. The critique instead checked whether each proposed fixture would fail today's absent feature and discriminate the designed seams.
- The launcher exposed no session identifier and labels this broad-read critique advisory; session and independent-context isolation are therefore unobserved.

## Coordinator disposition (m1, 2026-09-06)

All eight are accepted; each changes what gets built, so revision 2
follows. Reads per finding:

- BRAIN-R1-01 (one brain per fleet): the declaration record gains the
  fleet identity (the transport remote the roster names), the per-host
  fence stays, and cross-host uniqueness becomes a visible status post
  with a named producer (the brain's boot and every turn end) and a
  cadence. A second brain in one fleet is a status line every seat can
  read, not a fence; the page says so in one sentence.
- BRAIN-R1-02 (malformed declaration): fail closed. An unreadable or
  invalid record is neither declared nor undeclared: the fence refuses
  every act it guards with the remedy (brain show, brain withdraw), boot
  carries the packet plus one line naming the corruption, and the record
  is published atomically. Fixture corrupts the record.
- BRAIN-R1-03 (authority routes): the fence moves to the one seam every
  human-labelled goal verb shares, the actor resolution of --by into a
  human actor, and covers resume, set-obligation, steal, set-pin,
  classify-sweep and repair by construction; the page lists every verb
  the seam reaches and states that a verb added later inherits the fence.
- BRAIN-R1-04 (payload bound): the runtime registry declares the context
  channel's byte bound beside its field (claude: 10,000 characters); the
  packet must fit whole or the payload carries the packet's path and its
  standing-instruction section only; asks and claims are single-line
  summaries with a per-line cap and a count of the rest.
- BRAIN-R1-05 (fail-soft inputs, time budget): each input maps to a
  payload line on error (digest, ledger, questions, jobs, census) and
  the composition runs under a deadline inside the hook's 15 seconds,
  packet first; the packet is never the part that is cut.
- BRAIN-R1-06 (turn end): the brain summary line is emitted from the
  verdict's top, before the ladder branches, so Busy, Open,
  WaitingOnHuman and Unreadable never suppress it; the scan gains open
  questions and queued drafts; drafts are owned by machine, not lineage.
- BRAIN-R1-07 (dispatch.sh guard): the legacy-grammar guard prints the
  brain refusal and the node command when the checkout is declared, the
  same text the engine's fence renders.
- BRAIN-R1-08 (fixtures): the matrix gains commit.sh --chain and
  --direct-fix refusals, goal claim and goal open --claim refusals, one
  fixture per authority route named in 03, the corrupted declaration,
  the over-bound packet and the long ask, and a Stop fixture seeded with
  an ask, a draft and a competing ladder branch.

Next: revision 2 folds these eight. Stop criterion for the design loop:
when a critique's findings stop changing the build; the page then goes
to the build behind its fixtures.
