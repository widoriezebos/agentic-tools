Working Mode: implementation
Orchestrator Identity: root Codex, main-1790272787-5030-13dde3
Date: 2026-09-26
Changed-line allocation: 6500

| Unit | Changed lines |
| --- | --- |
| intent-workflows | 6500 |

# Goal
Implement the accepted complete intent-workflow design so people and agents can
finish the retained tasks and their ordinary recovery using public intentions,
without learning or calling the machinery. This is a coherent public-boundary
redesign spanning the existing owners, not a help-only patch. Root will run a
fresh usability pass after this implementation and request justified corrections.

# Workspace
/Users/wido/LocalStorage/GitHub/agentic-tools-intent-workflows-20260926
Branch codex/intent-workflows-20260926. No mission is active. Source baseline
1226c71bf (merged at 795db0d18) plus the coordinator's tracked design.
Leave product changes uncommitted. Never switch branch, reset, clean, stash,
pull, push, change goals or claims, or edit the receipt/narrator logs. The root
owns integration and records. Never read/copy metasystem/metasystem.conf.local.
Never start real supervision, missions, fleet machines, channels, paid subagents,
or external services to test. Use existing dependency seams and isolated fixtures.

Existing explicit human machinery bypass applies to this work. It permits this
rostered Opus build launch outside the obsolete goal/stop machinery. Ignore stale
stop hooks and unrelated ready goals. It does not authorize falsified evidence,
weakened authority checks, erased test failures or unasked external communication.
Do not redispatch yourself or a critic. Root authors design and runs independent
Sol review; you implement and run focused checks.

# Inputs
The complete specification is metasystem/plans/designs/intent-workflows.md, with
metasystem/plans/intent-workflows-design-dispositions.md and the final Fable report.
The final two design findings are closed as named fixture obligations; implement
those exact scenarios and read the root dispositions, including its correction of
Fable's collected-only assumption for current findings. Read those fully. The capability audit is
metasystem/plans/intent-workflows-capabilities.md; all its meaningful outcomes
are required by the end of this implementation, not saved by hidden aliases.
The old superseded plans/designs/verbs-match-intent.md documents baseline custody
and implementation proof; it is not the authority for old discovery/manual steps.

Primary allowed product owners:
- metasystem/cmd/metasystem/intent*.go, main.go, ui_describe.go, corresponding/new tests;
- metasystem/internal/launch/unit*.go and tests for selection/revision request identity;
- metasystem/internal/goal/branch/read.go and tests for bounded failed examination retry;
- metasystem/internal/dispatch Go policy and tests; scripts/agents/dispatch.sh only
  the necessary existing follow-up plumbing, no new shell implementation logic;
- metasystem/internal/ui/uitools command catalogue projection and tests;
- metasystem/internal/config and metasystem/metasystem.conf for the reader allowance,
  settings discovery/masking through the existing owner;
- existing narrow health/channel/mission/project adapters when typed public outcomes
  require an additive owner API. Preserve policy, storage and authority owners.
- metasystem/testing.json to include real new tests in the appropriate existing groups;
- public workflow docs and skill examples whose actual commands change. Do not edit
  the design, rulings, AGENTS contract or root-owned plans; report needed corrections.

The whole change estimate is not a cap. Work in coherent stages with their tests:
discovery/projection, complete delivery, human/questions/operations, integration.
Do not add a workflow database, generic repair coordinator, broad error-text regex
translator, automatic semantic judgment, another role or new dependency. Extend
existing owners and typed boundaries. No source comment may narrate review history.

# Acceptance
The seven design obligations are the contract, and the capability audit is the
preservation checklist. Complete each canonical route, its help, JSON/text outcome
and actual owner invocation. No advertised stub or silently ignored flag. Unknown
inputs refuse before effects; truthful partial states retain the public recovery.
Zero-findings review closes lawfully; decisions bind exact subject/return; accepted
unresolved material findings require revision; failed revision and failed review
retry are explicit, bounded and idempotent. Private compatibility stays callable.

Required fixture details include every named final-critique fixture in the design.
Do not conflate the reviewed attempt in a decision file with --after N. Goal-event
waits remain public through --for landing|human-act and --since TIP.

Root specifically authorizes updating assertions that demanded all 48 commands in
root help, exposed internal catalogue in Partner, no-arg exit 2, or manual close/
collect steps. Replace them with the new behavior AND retain every substantive
capability, exact subject, independent review, authority and concurrency assertion.
Never skip/delete a failing test to get green, lower coverage or change policy.
Behavior tests stub Git and use per-instance dependencies; use actual Git only in
existing narrowly justified adapter integration fixtures, not fresh broad fake Git
implementations. Prefer extending actual connected-owner fixtures to testing only
that a wrapper printed an expected command.

# Verification and budget
Maximum work before reporting a checkpoint: 150 tool calls or 40 minutes. If that
bound arrives with useful incomplete work, leave it intact and return precise
remaining work, test results and a resumable checkpoint. This is not permission
to omit a requirement or call partial implementation complete. No enforced token
budget is invented. Do not wait more than 60 seconds in one command; launch long
checks to files and collect their real exit codes. Use METASYSTEM_TESTING_WORKERS=9
for suite runners and preserve inherited METASYSTEM_TEST_WORKERS and all caches.

Run gofmt. Run tests directly affected by each changed owner with actual -run names;
repeat process/concurrency fixtures three times. Build the CLI and drive at least
the help/invalid-input surface from the new binary. Broader compile/vet and selected
acceptance proof belong to root after the completed diff; do not run the entire
repository suite or coverage ratchet repeatedly. Capture each command, actual exit,
and the behavior observed. A red baseline is still red and must be reported.

# Return
Write a factual checkpoint/return to
metasystem/artifacts/agents/intent-workflows-build-return.md (ignored runtime artifact).
State completed and remaining obligations, exact changed files, implementation
choices, focused checks with real exit status, actual runtime observations, risks,
any requested design decision, and the total diff size. Return that path and a
short outcome. Do not claim Fable/Sol approval or product completeness on your own.

# Gap rule
If the written design leaves a material authority, capability, retry or workflow
choice unresolved, name the smallest concrete gap and stop that dependent part.
Continue independent agreed work. Ordinary local implementation choices are yours;
do not turn every helper signature or formatting choice into a design escalation.

# Source-grounded administration choices

This appendix fixes public grammar; map to existing owners, preserving their actual
checks. Do not invent enrolled-proof requirements where an owner currently uses
human attribution/classification. Positive and negative authority fixtures prove
what each owner actually requires, and partial outcomes stay partial.

| Public choice | Existing owner and exact input mapping | Required semantics |
| --- | --- | --- |
| `repair goals` | goalsync_verbs.go runGoalRecover, selected root | Recover that endpoint's entire journal; leave live owners alone; never claim goal-local scope. |
| `repair goals --accept-edits --by NAME` | goalsync_mutations.go reconcile, root/by and existing actor context | Reconcile exact current local edits against their base; no invented digest flag. Sensitive edits still require owner proof. Expose a read-only preview via check goals before this act. |
| `repair goals --refresh` | reconcile --refresh-only | Repair the published-view crash tail without interpreting manual edits as new authority. |
| `repair goals --upgrade --source-digest SHA256 --by NAME [--amendments FILE] [--identity ULID] [--sync-mode remote/local]` | goalsync_verbs.go migrate; amendments maps manifest, remote default | Exact reviewed legacy bytes; identity retains an existing value on replay. Help/preview must explain and emit source digest and complete amendment format from the owner. No migration runs in real user state during this build. |
| `repair goals --accept-remote-history --by NAME` | goalsync_verbs.go repair --accept-remote | Validate same ledger identity and accept fetched current history locally; no push and no invented expected-tip guarantee. Owner classification is stricter at a declared coordinator; preserve it. |
| `settings coordinator [--declare/--withdraw --by NAME]` | brain.go show/declare/withdraw | Default read-only. Agent-free human caller for mutation, declaration quiescence and unique-machine rules retained; absent is an honest absent state, not crash. |
| `settings compatibility [--minimum-engine SHA40 --by NAME]` | read existing floor / goalsync_mutations.go engine-floor --commit | Mutation requires enrolled-human proof and synced goals. This records the human's assertion of compatibility; never claim machines were probed. |
| `repair mission M --problem N --confirm-restored TREE --by NAME --reason TEXT` | missionrunner_verbs.go resolve-taint --taint N --restore TREE | N positive; TREE 40-64 hex; human caller. Confirms files already match a recorded safe tree; never claim automatic file restoration. Explain any required external restoration to the named safe version; ledger-domain damage cannot use this route. |
| `repair mission M --problem N --accept-workspace --waive CLAIM... --by NAME --reason TEXT` | resolve-taint --adopt --waives CLAIM... | Compute exact observed workspace via owner, record explicit waived attribution claims; never auto-waive or accept a supplied adoption tree. Resolve all outstanding problems before resume. |

All mutually exclusive choices refuse together before effects. Normal help hides
fixture/lineage/session binding options but compatibility parsing remains. Public
operation adapters preserve exact owner outcomes, including authorities they do
not possess; they never promote a named human to a proven one.

Additional retained choices:

- Exceptional landing: `land G --exception CODE --reason TEXT --by NAME
  [--expires 2h] [--replace-exception ID] [--transfer] [--upgrade-goals]` maps
  runGoalCarryLanding's past/why/by/expires/supersede/transfer/raise-format.
  Compute the exact whole-project candidate through the existing landing owner;
  do not ask callers to construct a private tree. Retain existing enrolled-human
  proof, positive lifetime <=4h, transfer-requires-replacement, exact one-exception
  scope and format decision. Complete through the existing land.sh --carried
  transaction, including its proof token, reservation, consumption and interruption
  recovery. Repeating the same public request must first rejoin the matching
  existing exception/landing, never mint another approval. A changed candidate
  asks for the explicit replacement decision. `--using-exception ID` is the public
  advanced route for an already recorded local or authenticated-channel exception.
  The existing requirement to land from main is a named checkout choice, not a
  prompt to run a shell protocol. Permit only minimal land.sh plumbing changes
  if an existing Go policy seam needs a public typed outcome.
- `split G --plan FILE`: use the existing closed input format and include a full
  two-member example in help: heading `# split G`, then `## member first` and
  `## member second`, each with one-line `- Intent: ...` and `- Next step: ...`;
  optional `- BlockedBy: ...` and `- Labels: ...` comma lists. Parent concludes,
  resulting related goals retain exact owner ratification; human-origin parent
  requires enrolled-human ratification. `group G GROUP`/`ungroup G` preserve arc
  membership behavior but explain a group of related goals, including released
  group claims when ungrouping.
- Upgrade amendments use internal/goal/manifest.go's existing format, documented
  as a user-facing amendment file, not a private schema to find: headers
  MIGRATION_EPOCH (RFC3339) and REVIEWED_SOURCE_SHA256, then `### add-goal: ID`
  with intent/origin/next (optional blockedby/arc), or `### amend-goal: ID` with
  next/blockedby/arc/state. Parked amendment also requires parked-by, parked-at
  (timestamp or EPOCH) and parked-because. Closed grammar and reviewed digest
  remain enforced. A bare upgrade requires no amendment file.
- `ask --withdraw Q --reason TEXT` maps channel.Close question/because; a
  missing/answered question preserves that owner's outcome. `ask --retry Q`
  validates and selects the existing question before reattempting delivery;
  never sends a new question. Existing poll drains the shared channel, so say
  what it actually does, or use a narrow existing-owner targeted retry seam;
  do not silently promise isolated Q-only effects while retrying every message.
- `answer Q [TEXT]`: channel owner ReplyInstructions is the authenticated reply
  location; local TEXT cannot become channel proof. Mission answer maps exact
  mission/ask/answer inputs and resumes via LaunchAtGeneration only when lawful.
  Ambiguous Q resolves neither owner; explicit mission M/Q resolves it.

Final owner audit clarifications (required for questions/exception stages):

- Choose targeted question retry inside the existing channel owner. Poll today
  retries all open threadless questions and advances answers; Close currently
  lacks the shared poll lock. Add the smallest owner operation that uses the
  same lock and preserves concurrent withdrawal. Do not implement retry by
  globally polling while claiming only Q was retried. Existing authenticated
  user/code/freshness/replay checks remain unchanged.
- Mission Answer has no enrolled-proof gate and does not launch resume. Keep
  that authority contract. A repeated combined answer/resume must recognize
  the exact already-recorded answer and complete resume, while conflicting
  repeated text refuses. Preserve reset-reason and budget-exhaustion rules.
- Exceptional landing only admits remote mode and the existing exact code or
  one group:X exception. Noncarryable candidate/record-ownership refusals stay
  noncarryable. Run the existing land.sh carried continuation once authorization
  exists; an existing authenticated channel answer also qualifies. Root's
  public grammar intentionally computes the tree rather than exposing a private
  tree-construction protocol. Fresh request matching must use the existing carry
  records and exact subject, not a second authorization store.
