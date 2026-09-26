# Fable critique r2: Complete tasks through intent (final design round)

- Kind: critique report, round 2 of 2, provider session d55cdac2-eac6-4f23-874c-e5e0bbd137d8
- Design: metasystem/plans/designs/intent-workflows.md (370 lines, modified in the worktree, uncommitted)
- Design SHA256: 4cd91701097ea040dec208d87e21095dce6a10cd7712dcc38049ac22d917815c
- Worktree HEAD: 98a632446d910b84308b983594322b370fe47346; source owners read at that tree
- Resolver: `project design-of --goal verbs-match-intent` lists the record as draft, old design superseded
- Also read: intent-workflows-design-dispositions.md, intent-workflows-capabilities.md (48 commands, 13 outcomes)
- Critic: Claude Fable 5.1 (claude-fable-5-1). Tool calls used: 16 of 36. Every line cited was read this session.

## Verdict

MATERIAL findings: 2 (new IW-C7, IW-C8). All six round-1 corrections are
confirmed as folded; IW-C7 is a residual inside the IW-C3 fold. Seven notes,
five of them exact fixture obligations. Unexamined scope is listed at the end.

## Criterion answers

1. Ordinary journey hides mechanics? Yes. Claim acquisition, stage-based
   selection, bound dispositions and in-operation closure are now stated.
2. Selection and retry identity unambiguous? Selection yes (lines 119-129).
   Retry identity yes for lost responses and concurrency; the disposition
   binding on the failed-attempt remedy is ambiguous (C7).
3. Empty closure, partial publication, changed subject sound? Yes (167-192).
4. Prior capabilities retained? All 48 commands are mapped; one wait
   capability has no public route (C8).
5. One discovery source and public remedies? Yes; closure refusal classes are
   mapped (193-203) and no bash command is printed.
6. Smallest robust implementation, owners fixed? Yes. Follow-on scope stays
   required (27-28); deferred list (30-33) contains nothing the user asked for.

## Round-1 corrections confirmed

- IW-C1 (lines 112-117). `claim [G]` and `release [G] --reason` exist as
  agent commands (intent_planning.go:131-155); build acquires through that
  owner, never take-over. Confirmed.
- IW-C2 (119-129). Per-command eligible sets, zero and multiple behaviour,
  no timestamp rule. Confirmed. Fixture obligation in N11.
- IW-C3 (145-160). `--after N` is a public attempt, present even for a failed
  round; rejoin before inference; replay never spends. Confirmed as the
  retry/rerun separation. Residual: C7.
- IW-C4 (173-180). Accepted unresolved material findings route to revise
  before closure; only owner-recorded resolutions complete. Confirmed.
- IW-C5 (234-239). land-ready is a claim state, not proof: goal/file.go:99,
  360; validate.go:455 (one landing slot per machine). The ordinary landing
  region (intent_delivery.go:1166-1400) makes no handover call, so "existing
  route-specific handover order" is accurate. Confirmed. See N14.
- IW-C6 (193-216). dispatch.sh:2478-2490 verified: follow-up admits
  completed, protocol_error and after-cap rounds; process-lost and cancelled
  require a fresh dispatch. The design states this honestly and bounds the
  extension. Confirmed. Fixture obligation in N13.
- Fixture and session-binding flags parseable but undiscoverable (84-86).
  Confirmed.

## Material findings

### IW-C7 The disposition binding refuses, or hollows out, the design's own failed-attempt remedy

Design: 156-158 ("A failed attempt still has N: its remedy is the same brief
with `--after N`"), 182-186 (file carries "a machine-written binding to goal,
work attempt, immutable reviewed subject and return digest"; "a stale file
cannot resolve a different review"). Source: the fold identity today binds
review, round, subject, brief digest, return digest and dispositions digest
(intent_delivery.go:938-953); the fold message, which is the builder's
follow-up input, embeds the findings and the dispositions (958-973); a
follow-up round's "previous" inputs are the read outputs of the newest round
only (unit_run.go:190-191).
Scenario: attempt 1 is reviewed; the generated file D is bound to attempt 1.
`revise --brief F --dispositions D` creates attempt 2, which fails without a
result. The result prints the remedy `revise --after 2 --brief F
--dispositions D`. D's bound attempt is 1 and the request says 2. Implementer
A validates binding attempt against `--after` and refuses the printed remedy
as stale. Implementer B accepts `--after 2 --brief F` without D; the new
round's previous inputs are the failed round's empty read outputs, so the
findings and decisions the correction exists for are absent from the
builder's input. Test 1: step 1 differs. Test 2: the retry route either dead
ends or silently loses its subject.
Correction (smallest): the binding names the reviewed attempt, not the
attempt being corrected. Define "current review" as the newest collected read
of that work. `--after N` accepts a file bound to the current review whenever
N is at or after the reviewed attempt and no later collected read exists; the
new round's input is the same bound findings-plus-dispositions document. A
file bound to an older review refuses naming the current one. Say this in
`help revise` and in the failed-attempt result.

### IW-C8 The goal-event wait has no public route

Design: 106 and 119 (`wait G [--work NAME]` "selects running work"); 34-36
(a hidden alias does not count as preserved). Capability map row 52 maps
`wait` to work and question waits only. Source: intent_work.go:188 advertises
`wait goal G [--event landing|human-act] [--after TIP]` with `--verb` for
human-act waits.
Scenario: an agent runs `land G --queue-only` and must wait for the landing
seat, or a script waits for a human act before continuing. `wait G` finds no
running work and returns the "completed or waiting" result. The landing and
human-act waits survive only as the unadvertised `wait goal G` spelling.
Test 1: the implementer builds `wait G` as unit-only and either drops the
event flags or leaves them reachable by internal knowledge. Test 2: an
existing meaningful capability is lost from the public surface.
Correction (smallest): advertise `wait G --for landing|human-act [--verb V]
[--since TIP]` in `help work` (or keep `--event`; keep `--after TIP` as a
compatibility alias, since `--after` now means an attempt in revise). Bare
`wait G` with nothing running and a queued landing suggests `wait G --for
landing`. Add the row to the capability map.

## Notes and fixture obligations (not material)

- N8 `show G` and `status G` both present a goal (258-259). Fix the split in
  help: show is the goal record, status is live work. Wording only.
- N9 `--after` is a ledger tip in wait and an attempt number in revise. Rename
  wait's flag `--since` with `--after` as alias. Naming only (see C8).
- N10 Line 206 cites branch/read.go; the file is internal/goal/branch/read.go.
  Citation only.
- N11 Fixture (IW-2, TestNamedWorkSelection): "collected current read" is not
  UnitRunner state today. UnitReview binds record, round, head and result
  (unit_review.go:42-56); collection lives with the read owner. The read API
  must retain the collected-read binding through the existing ReviewSubject
  retain callback or consult the read owner. Fixture: one work item whose read
  is collected and published, one collected but unpublished, one built and
  unread; `review G` and `land G` select correctly in all three.
- N12 Fixture (IW-2, TestRevisionRequestReplay): crash between recording the
  after-N identity and admitting the launch. Retry resumes that round and the
  manager admits exactly one launch; a concurrent identical call joins it.
- N13 Fixture (IW-3): `review G --retry N` refused while the failed critic's
  process is live; admitted after the sweep marks it stopped; the new
  examination carries the original chain's round cap; repeating retry N after
  the retry itself fails rejoins and offers the new number.
- N14 Fixture (IW-4, TestIntentQueueOnly): a second `land G --queue-only` on
  the same machine returns the owner's one-landing-slot refusal
  (validate.go:455) with `land` of the earlier goal as the public remedy.

## Unexamined (not certified)

`repair goals` choices (goal recover, reconcile, migrate, accept-rewritten
owners) and their exact human inputs; `repair mission M` restore and
accept-workspace; channel ReplyInstructions, question authentication and
delivery retry; mission engine Answer and resume; exceptional landing carry
proof, token and one-exception rules; `settings coordinator` and
`compatibility` owners; the Partner renderer; worktree preparation beyond the
claim check; split and group input formats. Obligation: the design says these
flags "describe the actual human choice" (292-293) but lists none. Before the
human/operations unit is built, its brief enumerates each choice with its
owner verb and the exact input, and TestIntentRepairAuthority exercises every
choice once with and once without actor proof.
