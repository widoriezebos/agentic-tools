# Intent workflow iteration log

Goal: verbs-match-intent. User authorization: Wido, 26 September 2026, design,
Fable critique, implementation, and repeated usability improvement; machinery
bypass and ignored stale stop hook persist. This record describes work, not policy.

## Baseline and first design

- Shared checkout: agentic-tools-m1e; preserve its uncommitted receipt/narrator logs.
- Isolated worktree: sibling agentic-tools-intent-workflows-20260926.
- Branch: codex/intent-workflows-20260926; source baseline c8ecd4bb62706c02331f12a4de4ae0025b634568.
- User requested pulling main; both checkouts pulled it. Incoming Partner sitting
  design does not change the CLI owners; its human authority boundaries are retained.
- Observed old executable: 527 help lines, no-argument exit 2. Observed first
  redesign executable: 72 help lines, 48 public commands, no-argument exit 2.
- Source audit found required manual review close/collect, non-idempotent repeated
  revision, internal question/recovery commands, and doubled Partner command names.
- Capability map: 48 current commands plus 13 existing advanced outcomes.
- Draft checkpoint: 98a632446d910b84308b983594322b370fe47346.
- Draft SHA256: 4382ffaea494dad8ae5c39eb76198bd922573a89946f017ac623facd47462bd9.
- Fable critique: launch 20260926t081353-83a8177af2, round 1, running at this entry.
- Design resolver and project check pass. No product implementation or runtime
  change is claimed by this draft checkpoint.

## Additional root questions for the design fold

1. Revision retries after completion must bind a stable prior work version; simply
   recomputing a key from the now-current result can accidentally authorize another
   round. Reusing identical text for a genuine later correction needs an explicit
   public version/decision, not a hidden timestamp or opaque chain id.
2. Compatibility-only flags such as fixture authority and internal session bindings
   must remain parseable for their existing callers while absent from public help
   and suggestions. Public agent recovery must establish its own session through
   the existing owner, never ask an agent to copy another session identity.
3. Review dispositions refer to an exact subject and findings, not just a mutable
   pathname. A retry cannot close a different newly current review with old decisions.
4. Keep ordinary reviewer/author decisions independent even while mechanical close
   and collection disappear from the caller's task sequence.

## Release completion test

All seven design obligations must have concrete owner tests and applicable runtime
proof. Run the complete public human/agent journeys again after implementation;
record each observed gap and its correction. A hidden help entry, renamed wrapper,
or retained internal escape hatch alone does not satisfy the contract. Preserve
an already verified candidate while making later improvements; do not repeat broad
checks for unchanged evidence inputs. Final judgment requires no unresolved material
findings and no demonstrated journey that still needs internal knowledge.

## Design accepted for implementation

Both checkouts pulled main again at 1226c71bf; incoming Partner changes merged
cleanly. Fable reviewed twice on one provider session: material findings 6 then 2.
Root folded all eight, refined C7 to retain authentic unresolved findings before
collection, and closed the final two bounded corrections as named fixture
obligations. Independent code critique remains mandatory. Main contract R126
now also records the user's explicit requirement that internal structure never
be required to complete a task. Product implementation is next.

## Implementation and verification preparation

- Accepted design checkpoint: b5d7b6819. Opus launch
  20260926t084154-3eae3d3163, claude-opus-5-5, running in the isolated worktree.
- Goal next step updated through the real owner; published tip
  e23da6aeb30837f7314f07ea12732b1b6c235ae1. Shared main pulled that record.
- Baseline CLI observations retained under ignored
  artifacts/agents/intent-workflows-verification/baseline.json. The first release
  prints 72 root-help lines and exits 2 on no arguments; --help exits 0. Build,
  review, wait and close command help was captured separately with exact exits.
- Independent source audit grounded every administration choice in the build
  brief, distinguishing human attribution, human caller classification and
  enrolled proof. It also identified targeted channel retry/withdraw locking,
  retained mission answer before resume, and carried landing token consumption.
  These are required implementation details, not claims of Fable certification.

Next verification consumes the actual candidate, with focused owner checks first:

1. Rebuild the CLI. Run no arguments, all root aliases, every focused topic and
   public command help, unknown and conflicting inputs. Observe exit codes,
   ordinary suggestions, hidden compatibility flags and no service effects.
2. Drive fresh goal/work flows through the connected real-owner fixtures:
   automatic lawful claim, multiple work selection, review/decision/revision,
   failed correction replay, superseded decisions, failed critic retry and land.
   Count actual launches/publications; inspecting printed commands alone is weak.
3. Drive human acts with and without the required authority, goal-event waits,
   question retry/withdraw races, exact mission answer/resume repetition, each
   administration choice, queue-only and carried landing interruption recovery.
4. Exercise the Partner's real catalogue consumer and compare it to CLI public
   discovery. All 48 original capabilities and 13 advanced outcomes need an
   implemented public route; retaining an old alias does not pass this check.
5. Sol reviews the computed complete implementation diff and named final Fable
   fixtures before certification. Root dispositions findings and resumes Opus
   for material corrections, then rechecks affected behavior.
6. At integration, pull current main, select required tests from the actual
   candidate/risk, complete every required group and all coverage floors, and
   reuse only matching successful proof. Record evidence and limits, then repeat
   fresh human/agent journeys and correct any justified remaining usability gap.

## Open questions for the first completed usability pass

Source-grounded, not yet judgments on the unfinished build:

- AGENTS.md:25 and docs/orchestration.md:228 currently require an internal
  delegate command for a rostered design author; orchestration:245 also sends a
  later critic round to internal delegate. The current public design covers
  review of a written design, but not obviously authoring through the roster or
  following up that design review. After step 1 works, drive these actual agent
  intentions and determine the smallest public route. A hidden internal escape
  cannot count as completion. If no lawful existing public route exists, this
  is a concrete requirement failure for a focused design amendment and Fable
  critique, not permission to publish an incomplete capability claim.
- Existing wait also supports proof attempts and filesystem observations
  through legacy selectors. Determine which are actual caller intentions and
  ensure their advanced public discovery is clear, without exposing registration
  or process bookkeeping as required steps.
- AGENTS.md's public work guidance must change with the completed workflow;
  the builder does not own AGENTS.md, so root folds those proven commands.

## First build checkpoint and next usability iteration

Opus launch 20260926t084154-3eae3d3163 completed with exit 0, provider session
124b0e5a-c6b5-42d0-835b-87b46082ce1e, 90 cumulative model calls, 96 cumulative tool calls and 97 invocation turns.
Authoritative start/end: 08:41:54.379064Z to 09:00:02.972678Z (18m 8.6s).
Its return incorrectly claims 150 calls/40 minutes reached and gives guessed
local times. Root's initial continuation brief approximation of 23 minutes was
also inaccurate; the retained launch timestamps above govern. This is a coherent
partial checkpoint, not budget exhaustion or product completion. Reported provider
cost was USD 7.0584966, not an estimate. The first focused owner tests and CLI
help/refusal checks passed; full delivery and operations remain incomplete.

Root separately drove the checkpoint binary from a disposable directory: bare,
--help and -h exit 0 with 44 lines; focused help succeeds; typo/bare wait/invalid
revision/conflicting queue-only flags exit 2. help design exits 2. Exact binary
hash and outputs are retained in ignored artifacts/agents/intent-workflows-
verification/checkpoint-1/result.json. These prove discovery only.

Continuation 2 (same Opus session) is launch 20260926t090311-bf1bba8367,
prioritizing actual review/close/collect/publication, bound decisions, failed
review retry and claim/selection completion before remaining operations. No
product changes have been committed or independently certified.

The source audit confirmed an additional required planning task: design authoring
through its own roster and continuing a bounded design critique lack complete
public routes. Proof/file waits exist but disappear from discovery. The first
runtime help still exposes read units and certified job chain. Root authored a
focused second usability design, designs/intent-planning-continuity.md (ID
01M3EFDSFTKWEMSDCP1BB7TDGQ), SHA256
495456bf4e7175abef81392d37e9a2e066b0808e48d4d2270b578d195a09e7d7.
Fable critique launch 20260926t091003-5291dcd738 is running, round 1 of a declared
maximum 2 for this new requirement gap. Parent C1-C8 stay closed on their mandatory
fixtures; this is not another round to polish their prose. Project check passes
147 records and the resolver finds the new draft. Root will disposition this
critique before authorizing its additional implementation.

## Second build checkpoint and continued work

Continuation 2, launch 20260926t090311-bf1bba8367, returned exit 0 with
187 cumulative model calls, 195 cumulative tool calls and 100 invocation turns. Its partial return is retained at
artifacts/agents/intent-workflows-build-return-2.md. Actual complete-review
composition, bound decisions, a bounded retry admission extension and automatic
claim were built. The return explicitly distinguishes real whole-close proof from
mocked dispatch/wait connections; several final fixtures and all major operations
still remain. Root has not certified or committed product changes.

Live main was pulled again and merged into the isolated implementation branch
through 31f26c53e; only two goal-record commits had moved since the prior merge.
User-owned live receipt/narrator changes remain unstaged and preserved.
Continuation 3, launch 20260926t092330-64b6eb51ce in the same Opus session,
now tackles the remaining human/administration/question routes and final parent
proof holes. It explicitly requires actual stopped-process proof and the real
retry shell branch, not a successful mocked callback alone.

Planning Fable round 1 reported three material contract corrections. All are
adjudicated in intent-planning-dispositions.md: author admission without build
claim/worktree, atomic stranded-start recovery, public production of proof wait
references. Root refined the recovery to use Supervise's existing locked claim
before any child, rejecting an age-only inference of process death. Fable final
round 2 is launch 20260926t092513-fbee9d93d4, same provider session. Planning
remains draft until that report is adjudicated. Parent C1-C8 stay closed on their
mandatory fixtures.

## Planning design accepted for implementation

Final planning Fable report read in full. Round 2 has two material findings,
IP-C4 (completed-follow-up replay must read the operation's actual child) and
IP-C5 (public capped-critic retry and design-path fresh-root guard). Both were
accepted into the existing-owner design and named TestDesignCritiqueReplayAndCap
cases. Trajectory is 3 to 2; no third prose round. Root's source audit additionally
clarified that claim-free authoring does not imply claim-free critique: existing
dispatch requires claimed tier-3 goal and stop authority, so review uses ordinary
lawful claim admission while retaining accounting and actor/epoch checks.

The follow-on design is accepted for implementation on these mandatory fixture
obligations, alongside the parent's obligations and independent Sol code review.
No code certification or runtime success is inferred from design acceptance.
Continuation-4 brief is prepared, estimated total combined patch 9000 lines
including tests, with the task-only size override authorized by Wido's machinery
bypass. It will resume the same Opus context after continuation 3 finishes.
Goal Next step now records both accepted designs, partial implementation, required
proof and the outstanding pull/push/integration. Separate evidence-reuse work
remains separate. Project record check passes 147 records; diff whitespace check
passes. Live user-owned logs remain preserved.

## Third checkpoint; counting correction grounded in the measurement owner

Opus continuation 3 completed at 11:40:34 CEST, launch
20260926t092330-64b6eb51ce, exit 0. Administration, question routing/locked retry
and withdrawal, accepted-risk lookup, project record reads and initial death-proof
admission are implemented. Return 3 clearly leaves exceptional landing, process
targets, typed health remedies and several real owner/positive-authority fixtures
unfinished. Root accepts channel:Q as the explicit ambiguity choice alongside M/Q.
Root separately drove the intermediate CLI; checkpoint-3/result.json binds its
binary hash and twelve actual commands/exits. Root still has 45 lines, design
help is absent and start lacks ui/machine: expected incomplete work, not proof of
finished discovery. No product commit or code certification exists yet.

Measurement precision: internal/launch/claude.go:measureClaudeTranscript scans
the WHOLE resumed provider session. calls counts unique model response IDs;
toolCalls counts tool_use blocks. Both are CUMULATIVE; turns comes from this
invocation's result. Earlier prose calling calls per-launch tool calls was wrong.
The three checkpoint snapshots are model calls 90/187/280 and tool calls
96/195/288, so new tool calls were 96/99/93. Actual duration: first 18m8.6s,
second 18m30.0s, third 17m3.7s. Agent approximations are not authoritative. No
internal budget violation is inferred from a cumulative counter. Historical
immutable briefs remain as issued; this correction governs their count claims.

Main merged through 6a97b8afc (goal record only). Continuation 4 is running as
20260926t094150-d14fba30a4, same Opus session, prioritizing unfinished parent
paths then the accepted planning work. All original fixture obligations remain.

## Manual work capability audit

Root source audit confirmed that arbitrary manual changes lack a complete public
feedback/submission route. Existing branch commit/read owners already support
manual delivery without a paid builder. Root authored the focused draft
`designs/intent-manual-review.md`, SHA256 d5c739fce4904f2cc9abfcdbf82a93c50f52268104de16aebd2f43143b1d4525.
Fable round 1 is running as 20260926t095707-28b7fc9e0f; maximum two rounds
for this separate concrete gap, no new product work authorized until disposition.
Investigation follows take-a-step-back with a bounded ledger. Diagnostic feedback
never becomes delivery certification.

Live main pulled through 5799bd3c6, including the new Partner/UI sitting work.
The isolated branch will merge it after Opus continuation 4 finishes; product
changes remain uncommitted. User-owned live logs are preserved.

## Fourth checkpoint and latest main integration

Continuation 4 completed at 12:06:41 CEST, exit0; 380 cumulative model calls,
392 cumulative tool calls (104 new), 105 invocation turns, actual24m50.7s. Nine
root starting points, UI/machine targets, exception route, public wait continuation
and branch read-state selection are implemented. Root read its full appendix:
real dispatch retry now reaches the register, which rejects capped critics; this
is an unfinished owner defect. No full dispatcher pass or positive authority
certification is claimed. The builder left run4 in progress; its result must be
read and fixture custody preserved.

Main merged through ee33be672, including 5799bd3c6 UI sitting changes; product
reapplied cleanly. Backup stash08d5a9c3a5bddaa939c685816d3d670289693ecd retained.
Root found the existing authentic positive-claim seam and a remaining check-remedy
loop; both are concrete required corrections in continuation5, not new policy.
Manual-review Fable final round runs as20260926t100811-91db57f15c; draft remains
unapproved for product work. Root deleted duplicate registry/patch-core shortcut
and recorded all round1 dispositions before that final critique.

## Manual design accepted; independent discovery checkpoint

Final manual Fable report read in full:3->2 material trajectory, closed at round2
on nested capture and staging-refusal fixtures. Root also corrected token-as-lock
and adoption replay semantics with existing owner primitives, recording IM-C1..7.
Manual build brief is ready for the same Opus context after current work. It does
not replace IW/IP obligations. No product/runtime certification is claimed.

Root drove checkpoint4 binary from a disposable directory: bare root exits0 in24
lines with nine starting points; focused help succeeds, malformed wait proof/file/
resume refuses2. Design help still2 (unbuilt). Binary hash, all commands and exits
are in artifacts/agents/intent-workflows-verification/checkpoint-4/result.json.
Continuation5 is running as20260926t101048-ddde2f31b7. Current main already merged.
