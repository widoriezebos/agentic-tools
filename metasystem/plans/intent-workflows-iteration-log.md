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

## Fifth checkpoint and independent read-owner implementation

Continuation5 completed12:26:45 CEST, exit0; cumulative460 model/473 tool calls,
81 new tools,82 turns, actual15m57.3s. Real dispatcher diagnostic dispatch-b passes
capped/lost critic followup in the same chain. Authentic auto-claim and foreign
claim refusal, real mission repair and file-wait/resume proofs now exist. Parent
administration/carry/event-wait/publication proof remains incomplete; planning not
started. Broad owner check failed three steward tests without installed engine;
the suspected environment cause was not baseline-proved. Broad intent run failed
two expectation regressions, fixed and focused-rerun0; no full rerun claimed.

Main pulled/merged through8db599e25 (new UI design correction). Private read-owner
seed f2d5da5fe541cd5d41aef30d64475d928f83c7d6 preserves unfinished product and
root design, tree9e820e6e0e8d1c7117608da6da71f7aba1613a34; not certification.
Independent Opus launch20260926t102804-0d4f267ead implements only read sequencing
and diagnostic request owner in sibling intent-read-owner checkout. First start
attempt raced unfinished worktree creation and refused before launch; after the
creation exited0 the successful launch above began. Main continuation6 prioritizes
planning and avoids that owner's exclusive files. Root will compute/integrate
its full patch before combined code critique and final testing.

## Parallel proof task and matrix structure check

Independent Opus launch20260926t103219-f91b1050ad, provider session
8c3d9fd1-eb05-4277-9e76-96d5ebd02821, works only on positive administration/carry/
event-wait tests in sibling intent-authority-proof checkout, same private seed.
Brief is retained under artifacts/agents/intent-positive-proof-brief.md; no product
source or shared test registry edits authorized there. Root will import computed
changes and preserve every actual failing boundary. Main continuation6 and read
owner launch continue independently; read owner provider session is
ecc9b3c2-d835-452f-bfec-d1953f2ab644.

Root ran obligation validators before completion. Parent IW1..7 and planning
IP1..4 correctly refuse completion while rows remain missing. Manual table had a
real schema defect: abbreviated columns made its four rows invisible to the
validator. Root restored the canonical ten-column table with explicit code targets
and next actions, preserving all behavior/obligations. Revalidation recognizes
IM1..4 and correctly blocks completion; exact exit/output retained in
artifacts/agents/intent-workflows-verification/design-gate-before-completion.json.
This is a record structure correction, not a runtime pass or a new design round.
Goal Next step published; live main now e3a86b67e, a goal-only change pending next
idle integration into the implementation checkout.


## Sixth checkpoint: real owner proof found delivery gaps

Main continuation6 completed12:49:42 CEST, exit0; cumulative538 model/554 tool
calls,81 new tools,82 invocation turns, actual19m44.8s. Builder estimate135calls
was not its measured count. Authoring, retained drafts, conditional publication,
startup recovery, lifecycle and design critique adapters now exist. Whole launch,
project and dispatch packages pass; broad intent/design/help exits0, followed by
a focused check after final input validation. Actual dispatcher design follow-up,
positive lifecycle, proof producers and manual work remain open. Root read full
return, merged main e3a86b67e and launched continuation7 as20260926t105203-033e78b3f4.

Independent positive-proof builder returned seven real-owner passing tests and a
real failing landing-wait observer: branch landing succeeds but its actual message
is not recognized by the observer. No assertion was weakened. Continuation2 is
20260926t105012-f0acae5912, adding actual question waiting in its same isolated
scope. Full carried transport still unproved. Source audit independently found
that the current exceptional adapter neither stages a candidate nor passes paths
or --staged-only. Root drafted intent-carried-delivery-design.md using a delta
between projected trees to preserve coordination files; Fable review is
20260926t105423-7b79807e56. This is required IW-4 completion, not optional expansion.

Read-owner first computed patch was exactly four files, tree
4388b5bbee0a4891f3bd931fac30b3e990ca3315, SHA256
93f5c563fc3bf545d28d6aeeff0ccef64d40486b68d078d215df92ed2e25de77.
Source audit found mixed compaction could falsely report green; original Opus
fixed it and tested the failing mutation, plus integrated actual startup recovery.
Correction1 launch20260926t104918-a3b719920f completed12:52:10 CEST: full launch0,
26.3s. A report filename collision remains; correction2 runs as
20260926t105527-7f2362031e. Final computed import and independent Sol critique are
still pending. Read-owner design_request.go is a read-only primary dependency,
not part of its returned patch. No product is certified by these checkpoints.


## Integrated independent read and owner-test changes

Root computed and imported read-owner final four-file patch after both source
corrections, tree1ab2d3fc26f67253fb7461e55972719921fe4173, patch SHA256
08ea63aa28e15429d84e36595eef453971f2c149eaf24540d2be04444713a9ad.
The copied design_request.go dependency matched primary byte-for-byte and was
excluded. Full launch previously passed; final naming/display focused tests and
mutation check pass. Main builder's integration marker names actual API and fresh
public-surface audit. Import is not independent Sol certification.

Positive tests imported as exactly two new files, tree
16b7d52c56c520c077fbab9d113eb0ea558e02f3, SHA256
b0e894b500876e38892c12d15f03b5af1293b535b3023e17135540239cd47d7c.
Return2 reports8 actual owner positives (including controlled pending question
wait -> authenticated channel answer -> public continuation), and2 preserved
failing reproducers: landing-event observation and actual carried land.sh staging.
Main builder reproduced the first and reports a focused fix; final combined proof
is pending. Carried transport still fails and remains required.

Carried Fable r1 completed13:02:11 CEST, report fully read,4 declared material
findings plus IC-C5 minor. Root joined all5 and folded fetch preconditions, PRIMARY
mutex/root, top-level apply and public replacement remedy. Duplicate patch storage
was deleted; the minimal old base/workspace identity remains because CarryWord has
no code endpoint, and diff(new main, old approved tree) could undo newer product.
Final Fable r2 is20260926t110543-be619ff15f, same session
3e48acf8-042b-4abb-b5df-c0909ff54a1f. No carried product extension authorized yet.
Live main pull still up to date at e3a86b67e; user-owned logs preserved.


## Carried design accepted for implementation, not certified

Fable final r2 completed13:11:42 CEST with2 material findings and1 minor. Full
report and IC-C6..8 dispositions retained. Root confirmed receipt.Add's actual
field schema, explicit File requirement and ObserveReceiptLine ownership, plus
Advance's own mutex/no-clobber behavior. The composition was rewritten in one
pass: fetch first, distinguish remote movement from lagging local main, stage the
product plus a truthful existing-owner receipt, then unchanged carried transaction.
Fable called missing receipt a contract-shape blocker; root adjudicated its exact
existing-owner correction as implementation-ready under the user's bypass. No
third prose round or runtime approval is claimed; actual fixtures and Sol review
remain required. Parent IW-4 now links these obligations.

Independent carried builder20260926t111623-8e55e2946e runs in sibling
intent-carried checkout from private seed05881caa339964f9162decd5fc6be2c59d920548,
treebd8c0ad04a831ba9cf53c691b0979a73273159ca. It reads the primary ACCEPTED design
inputs (its seed holds the older draft), owns only exception adapter/staging and
carried-only tests, and avoids shared delivery/testing registry files. Main Opus7
continues manual/public workflow implementation. Root prepared a thin external
Go coverage harness and unchanged prior selector; build passed against current
source, but no acceptance tests have run. Both must be repinned/rebuilt against
final immutable candidate before actual verification.

## Main refreshed, completion work split by owner

Live main pulled9d6002b09 (Partner/UI plus goal record). Main builder7 completed
13:22:13 CEST,30m10s,162 new tool calls (716 cumulative),1 compaction, peak966377.
Return7 fully read. Actual landing-event observation now passes its real Git owner
fixture; short waits retain usable capture time, owner JSON refusals keep their
reason, and read selection distinguishes publication. Diagnostic public review is
connected. Real planning follow-up and full manual submission remained missing;
these are not completed by that checkpoint.

Root committed capture corrections/explicit completion briefs e411b6cd8, then
merged latest main as5cd534d8d9e01c0f9d0d02bc10cc0ff2e1b8df90. Task-only product
backup stashc2a5f70529b8c26238551d90965e9d239f254e8a was applied cleanly and retained.
Live user receipt/narrator changes were untouched. Private planning seed
0ab9824964c2a2211c55690e904152615eac8999, tree4602a66714d833434d4565b1821fe34d17c63a29,
captures this unreviewed combined state; it is not a delivery commit.

Original Opus8 runs20260926t113051-48f0551eb5, manual submission/shared surface.
Independent planning completion runs20260926t113116-ea8923ded0 in its sibling
checkout, accepted actual dispatch/author/public-close obligations. Carried return1
20260926t111623-8e55e2946e completed13:32:12 CEST,76 tools,15m49s (its guessed40min
was not actual). It reaches real staged product/receipt but proof is still missing.
Same-context continuation2 is20260926t113421-2d30b3f2a1. The source recipe confirms
public test can prove the staged index; real transaction/recovery fixtures remain.

Read-owner correction4 was imported as delta1ab2d3 ->440972dcca508255cf71be4f9f13923717f67cae,
SHA256a963b3a36032013c07ce905b7051dd70984126a29ea84b5889aed98a7c359fca.
It excludes the explicitly named in-repo brief from implicit diagnostic capture,
keeps supplied patches and other changes, and preserves copied-index mtime. A
read-only independent real-Git reproduction proves a fresh private-index timestamp
can silently omit a same-size edit. Original Opus added its deterministic adapter
regression in continuation5, delta440972 ->2678096eed92dd25b7b9156f17830002747ce771,
SHA256aa653fa52aa84e72675690954d751c8646738f616949e29d692672b35889689e.
Mutation fails5/5; fixed test passes5/5; relevant capture suite0,6.0s. Root's own
three imported brief/capture adapter checks pass0,0.963s in the combined checkout.
No full launch rerun or final certification claimed. Final-symlink brief alias is
noted for later under R124; no ordinary source/index corruption was found.

Fresh source audit found first-use brief preparation, qualified goal/question
continuations, task discovery/disambiguation and archived-goal recovery still
expose internals or dead-end. Root's final-surface audit specifies the minimal
public corrections within existing capabilities. A separate serial audit found
nine actual environment-mutating tests and one eligible parallel test; truthful
exemptions/parallel change remain for integration. Coverage floors stay unchanged.
Independent Sol brief is prepared; no Sol read or final acceptance has run yet.

## Real owner journeys and recovery corrections

Main8 completed13:58:03 CEST (20260926t113051-48f0551eb5). Manual submission now
uses the actual commit/range/read owners and shared checkout lock; focused checks
pass, but actual endpoint delivery and several failure recoveries remain. Root
independently reproduced a real failure: cleanup after a refused same-checkout
submission resets files that the caller had already staged. The external real-Git
overlay fails with lost staging; main9 must preserve the original index state and
report uncertain/failed cleanup truthfully. Explicit patches from inside the goal
checkout are supported input, and changed briefs must bind the actual read owner.
Main9 (20260926t120520-b255ad63c1) runs in the same original Opus context.

Planning checkpoint20260926t113116-ea8923ded0 completed14:01:18 CEST. Actual
dispatch and supervisor journeys exposed and corrected invalid review mode names,
misreported dispatch refusal, absolute/relative subject mismatch, adopted design
path rejection, lost design-chain lookup, repeated-read redispatch, round decoding,
relative default-store custody, and a missing output path in the author prompt.
Real public whole-close, refreshed round-two subject, lost-response replay,
second-checkout publication and process cancellation pass. Required path-scoped
fresh-root refusal, failed retry and foreign-round/revision replay remain open.
Root computed and imported exclusive tree83eac8372570dae0212ac52a13e475aca2f6adfc,
patch SHA256861b20090c4cace23830fed80c9ddab027c72bc218681a3b5e64400d11c69a6c.
Shared CLI seams are a separate patch for main9; no concurrent shared-file edit.
Planning continuation20260926t121453-b81d774e4d completes those accepted obligations
and moves newly introduced fixture logic into Go, preserving actual owner proof.

Carried checkpoint20260926t113421-2d30b3f2a1 completed14:00:17 CEST. The real
unchanged land.sh transaction now commits/pushes the candidate with provenance
and consumes its word. Same-word recovery after failed push, retained old subject,
fetched consumption and linked-checkout/outside-prefix preservation pass. Explicit
replacement remains red because the existing candidate scratch commit runs actual
delivery hooks. Root adjudicated scratch-only hook isolation and exact-candidate
new-plan acknowledgment in the accepted carried design. Public test recovery and
channel-word adoption still require actual proof. Root computed/imported exclusive
tree2db595cce6e328c440bcdd4e4464b327b0b3228a, patch SHA256
9bdb36f599b98a069e6bbbba238e29773c7b0705b3bf7753b16ba896bb258cd6; preserved the
independent parallel correction in the other authority test. Continuation
20260926t121454-c3b1921a3e runs in the original carried Opus context.

Live main pulled8ca54c6f4 on26September, including private interface-store bounds;
the primary implementation still sits on merge dd1233e66 until main9 returns, so
merging avoids racing its shared testing.json/metasystem.conf writes. Live user
receipt/narrator rows are preserved. No product certification, final coverage or
Sol review is claimed by these checkpoints; all remain completion requirements.
