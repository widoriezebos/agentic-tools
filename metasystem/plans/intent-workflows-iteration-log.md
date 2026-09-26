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
124b0e5a-c6b5-42d0-835b-87b46082ce1e, 90 measured tool calls and 97 turns.
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
187 measured calls and 100 turns. Its partial return is retained at
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
