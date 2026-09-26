# Fable critique r1: Complete tasks through intent

- Kind: critique report, round 1
- Design: metasystem/plans/designs/intent-workflows.md
- Design SHA256: 4382ffaea494dad8ae5c39eb76198bd922573a89946f017ac623facd47462bd9
- Source baseline read: c8ecd4bb62706c02331f12a4de4ae0025b634568 (HEAD of this worktree)
- Resolver: `project design-of --goal verbs-match-intent` listed the new design as draft and the old as superseded
- Critic: Claude Fable 5.1 (claude-fable-5-1). Author: root Codex.
- Tool calls used: 27 of 36. Every source line cited below was read in this session.

## Verdict

MATERIAL findings: 6 (IW-C1 to IW-C6). Notes: 7. The design is the right shape
and the follow-on scope is not a silent deferral (see criterion 6). The material
items are contract gaps an implementer would fill two different ways, or fill
wrongly, when building step 1.

## Criterion answers

1. Ordinary journey hides mechanics? Mostly yes. Exceptions: the journey starts
   with an unmentioned claim (C1), and the revise/close split for accepted
   material findings is unfixed (C4).
2. Selection non-guessing, retry identity unambiguous? Selection rule names
   "relevant" without defining it (C2). Retry identity is sound for lost
   responses and concurrency, wrong for a failed round (C3).
3. Empty closure, partial publication, changed subject sound? Empty closure and
   partial publication are sound in intent; the close owner's own refusal
   classes are unmapped (C6). Changed subject is sound.
4. Prior capabilities retained? Nearly all. `ready` maps to an ambiguous flag
   (C5). `claim` and `enroll` are not in the retained list (N1).
5. One accurate discovery source, remedies without dead ends? Discovery: yes,
   descriptor table exists (intent.go:42-55) and needs group/visibility fields
   added; the root-help test at intent_test.go:176 is explicitly superseded.
   Remedies: close-owner refusals still end in "resolve what the close owner
   names above" (C6).
6. Smallest robust implementation, owners fixed? Owners are real and correctly
   anchored (checked: UnitRunner AdvancePrepared and named lock, finish() state,
   commitReview, criticClosure, close_chain, CritiqueClosed, landing unread
   path, ask/answer paths, help renderers, Partner catalogue). Follow-on scope
   (lines 27-28) is declared required before the request is complete, and the
   deferred list (29-33) contains nothing the user asked for. No silent deferral.

## Material findings

### IW-C1 The ordinary build journey starts with an unmentioned mandatory claim

Design: lines 97-106, 127, 172-176. Source: intent_worktree.go:123,187-189
("goal/%s is prepared only under this session's claim"); intent_work.go:301-310.
Scenario: an agent reads root help, runs `metasystem build G --brief F --check C`
on an approved, unclaimed goal. Build refuses because the session holds no claim.
The design's canonical grammar, its retained-capability list (174-176) and its
public language section never name `claim`, so the refusal's remedy is not a
public step of the design and the implementer either leaves the raw refusal or
invents a route. Test 1: yes, step 1 differs. Test 2: it does not work for the
first ordinary caller.
Correction (smallest): state that `build G` acquires the claim through the
existing claim owner when the goal is ready and unclaimed, under that owner's
quota, elapsed-fence and authority rules, and otherwise refuses naming the
holder and the public commands `claim G` and `goals --ready`. Keep `claim` as an
advertised agent command in `help work`.

### IW-C2 "Exactly one relevant work item" is undefined

Design: lines 110-112. Source: named entries are keyed per (worktree, goal,
unit) with no goal listing (unit_named.go:100-125; UnitRunner public API has no
list); the design adds a read API, so the rule is decided here.
Scenario: `main` is built, reviewed and read; author starts `build G --work fix`.
Then `review G`. Under reading A (relevant = every named work of the goal) the
call is ambiguous forever after a second item, and every later bare command
must name work. Under reading B (relevant = stage-appropriate) `review G`
resolves to `fix`. Both are non-guessing; they produce different CLIs, and
IW-2 is the CRITICAL obligation. Land already has its own rule (all unread).
Correction: define relevance per command from the run record: build selects
work with no run or an identical retained request; review selects work whose
newest round has a build result and no collected read; revise selects work with
a collected read or a failed newest round; wait selects running work; status
lists all. Work that has landed or belongs to a concluded goal is never
relevant. Ambiguity output stays as designed.

### IW-C3 Revision retry identity captures a terminally failed round

Design: lines 128-135 ("A retry rejoins that recorded round even after
completion"; same brief for a later revision "requires the new subject").
Source: unit_run.go:744-750 finish() sets awaiting-judgement for every outcome,
so the owner today admits another follow-up after a failed round
(admitFollowUp, 941-950).
Scenario: `revise G --brief F --dispositions D` launches; the builder crashes or
the round ends failed with no new result. The author repeats the exact command.
Identity (goal/work, subject, brief bytes, disposition digest) is unchanged
because no new subject exists, so the call rejoins the failed round and returns
its failure. There is no public way to run the same revision again without
editing the brief. Test 1: implementer builds the rejoin without an outcome
check. Test 2: not robust under the crash case the brief names.
Correction: the retained identity records the round outcome. Rejoin applies
while the round is running or when it produced a result. A round that ended
without a result releases the identity; the same request starts a new round
under the same named lock (so concurrent calls still launch once) and the
result says "retrying revision round N, which failed: cause".

### IW-C4 Accepted material findings: review --dispositions versus revise is unfixed

Design: lines 144-149 and 153-157. Source: CritiqueClosed accepts `accepted`
on a material finding and refuses only `noted` (critiqueclosed.go:41-43,
246-248); landing refuses a unit with "no clean read" and points at review
(intent_delivery.go:1377-1380); fold validates against the return of the
unclosed chain (886-903); close_chain writes chainClosed (dispatch.sh:3030-3044).
Scenario: critic returns one material finding. Author writes `accepted` and
runs `review G --dispositions D`. As written the design closes, collects and
publishes. Whether that read is "clean" for land, and whether `revise` may
still fold a chain that is already closed, is not stated. One implementer
closes and land later refuses with no public remedy; another refuses the close
and demands revise. Both are step-1 behaviour.
Correction: fix the rule: a disposition set with any accepted material finding
is a revision request; `review G --dispositions` refuses without effect and
prints `revise G --brief FILE --dispositions FILE`; sets with zero accepted or
unrefuted material findings close, collect and publish. `revise` on an
already-closed review refuses naming the current subject. Say in help that
acceptance means "will fix", refutation carries evidence, out-of-scope cites
the brief.

### IW-C5 `land G --prepare-only` conflates the ledger handover with the proof

Design: lines 162-166. Source: `ready` is the goal act land-ready: "The claim
leaves the one-claim quota and its elapsed fence until it lands. It is the
claim holder's own act" (intent_planning.go:163-170, 1084-1085). Land's
"prepared" is the proof stage with test receipt (intent_delivery.go:1318-1345).
Scenario: an agent finished a goal and must release its claim quota while a
landing seat lands later. It reads `land G --prepare-only`. Reading A: runs the
full proof without push (expensive, needs the landing checkout). Reading B:
performs the land-ready handover only. The name "prepare" is land's proof
vocabulary, so reading A is the natural build, and the cheap ledger act
disappears behind a suite run.
Correction: define the flag as exactly the existing land-ready act (read
collection plus claim handover, no proof, no push), or rename it
`--handover`. State that ordinary `land G` performs the handover implicitly
before proof when the caller holds the claim.

### IW-C6 Close-owner refusal classes are not mapped to public remedies

Design: lines 142-151 (auto-close inside review), 242-249 (translate at the
typed boundary; every raw-command return audited). Source: close_chain runs
`brain fence close` (dispatch.sh:2976-2982), `lease_entry_check` requiring a
holder for the caller pid (371-378, 2994), `close-check` after mirroring
(3020-3025), and `internal_authority record-writer` for critique close (3439).
The existing close intent maps only the fence (1091-1093) and otherwise says
"resolve what the close owner names above, then run the same close again"
(1100), the vague dead end the design bans.
Scenario: `review G` on a zero-finding read runs from a terminal that is not
the claim holder, or while the brain fence holds, or a mirror fails
close-check. The read is collected-but-unclosed. The implementer inherits the
existing text, so the user gets a bash `die` message with no public act.
Correction: enumerate the four refusal classes and their public remedy in the
review result: fence -> outcome in-progress with `status` and the same command;
holder -> name the holding session and the public command from it;
close-check/mirror -> `repair` target or an honest outside dependency;
record-writer authority -> the authority refusal as today. Report the read as
"collected, closure pending" and never as accepted.

## Notes (not material)

- N1 `claim`, `enroll` and `fleet` are existing both/human commands absent from
  the retained-capability list (174-176). `status --machines` covers fleet;
  add claim and enroll explicitly so the visibility pass cannot drop them.
- N2 The descriptor struct (intent.go:42-55) has no group or visibility field;
  the implementer adds both. Consistent with "one existing table".
- N3 `answer Q [TEXT]`: for channel questions TEXT is never accepted
  (intent_process.go:106-110 today). Help must say TEXT applies to mission
  questions only, or the optional argument reads as a trap.
- N4 Disposition template (line 146): the validator's vocabulary is accepted,
  refuted, noted, out-of-scope, with evidence rules for refuted and
  out-of-scope. The template should carry those words and the evidence rule.
- N5 `intent.review.tool-calls=48` names a config key; the config owner's
  masking/provenance rules apply. Fine as written.
- N6 Line 275 supersedes the root-help completeness test. Correct; keep the
  per-command `help NAME` and `--help` assertions at intent_test.go:181-195.
- N7 Estimates (about 5500 lines) are plausible given four owners; unchecked.

## Unexamined

intent_worktree.go worktree preparation beyond the claim check; channel
ReplyInstructions and delivery retry; mission engine Answer/resume and
restore/accept-workspace; doctor/recover/settings owners (intent_process.go:958,
intent_planning.go:1645); split/group input formats; landing replay and
`--exception` carry-proof rules in intent_delivery.go:1166-1300; Partner
renderer beyond kit.go:150-160; evidence mirroring under close-check.
