# Delegate delivery: every channel a return can arrive by, and the guards on each

Working Mode: design

Owner: main session (delegate), under Wido's 2026-08-15 morning
rulings (design-critique-implement before anything else; everything
Go is better suited for goes in Go). Status: CONVERGED at r8 by the
stop criterion (critiques at plans/delegate-delivery-critique-r{1..8}.md,
decision recorded as D64): the architecture has been stable since
r5, and rounds 6-8 each corrected only the width of ONE row's
predicate — r8's remedy, like r7's, is fully determined by shipped
code. The final formulation deliberately stops paraphrasing:
candidatesPresent replicates the shipped per-channel presence bar BY
REFERENCE (stdout: any non-empty bytes; named file: non-empty valid
JSON — the exact gate devin.sh applies today), and the four fixture
legs in the proof obligations are the enforcement, not the prose.
IMPLEMENTED — both phases — under the full gates: phase 1 in
checkpoints A-F (2e035c4..6432948, gated green on the VM), phase 2
at 412b596..a2be65b (host walk, resumable rejection seam, FinishTurn
accepted-reply). The D64 entry and its addenda in the decisions doc
carry the disposition trail; the live devin path's proof remains the
arm's selftest and its next rep.

## The problem, with two cohorts of evidence

A Devin delegate's reply is whatever `devin -p` prints. Two cohorts
proved swe-1-7 does not reliably use that channel: it finishes work
by WRITING FILES. Under graded permissions the final write was
confirmation-blocked and the session died undelivered (D57); under
dangerous mode the write landed — a schema-perfect return in /tmp —
and stdout was still empty (D62). One bias, two failure shapes, five
jobs' work lost while sitting in evidence we held. D62's named
return path is a request the model can ignore; this design owns the
full mechanism. The same bias is LIVE on the mission-host path
(hosts/devin.sh reads stdout only) — r1 wrongly excluded it; r2
scopes it in.

## Shape: one engine verb owns everything but the processes

`adapter devin-collect` (internal/adapter, new) owns candidate
enumeration, per-candidate CANONICAL validation, selection,
attempt boundaries, provenance, and the verdict. devin.sh keeps:
spawn, wait, custody, and — on the verdict's say-so — one repair
CLI invocation whose outputs feed a second devin-collect call.
hosts/devin.sh integrates the same verb for its turn result
(rungs 1-3 only; host retries belong to the mission runner's
existing priorFailures machinery, never to a delegate-style repair).

Inputs per call: --job, --round-dir, --workspace DIR, --stdout
FILE, --named FILE, --transcript FILE, --schema FILE, --record FILE,
--attempt initial|repair. Output: a JSON FACTS document on stdout —
`{"delivered": bool, "channel": "stdout|named-file|transcript|none",
"reply": "<accepted snapshot path>", "candidatesPresent": bool,
"watermarkValid": bool, "reason": "..."}` — candidatesPresent
replicates the SHIPPED per-channel presence bar by reference, not by
paraphrase (rounds 7 and 8 each caught a paraphrase drifting):
stdout counts when non-empty regardless of content, the named file
counts when non-empty valid JSON of any kind — exactly the bars the
current adapter applies before its handshake refusal — and no
canonical validation runs. The four presence shapes are pinned as
fixture legs; the code being replicated is the authority when prose
and code disagree — plus the side effects below. The collector
reports COLLECTION FACTS ONLY (r3 finding 5): it has no CLI-status
or session inputs and emits no repair recommendation; adjudication
alone composes eligibility from CLI status + collection facts +
session correlation. Exit codes: 0 delivered, 3 nothing qualified,
1 mechanical failure (the walk itself impossible).

## Selection is validity-aware (r1 finding 2)

A candidate is accepted only if it passes the ONE canonical
validator — the same schema-and-job-identity check
`validate return-complete` applies, invoked in-process by the
collect verb (both live in the engine; no second validator is
born, the existing one gains a library entry point). Walk order:
stdout, named file, transcript. A candidate that fails validation
falls through — junk on stdout no longer shadows a valid named
file, and a torn named file no longer shadows a valid transcript
return. The verdict's `reason` names every rejected candidate and
why (evidence, not silence).

## Candidate snapshots (r1 finding 10)

Each candidate is read ONCE into a bounded in-memory snapshot
(ceiling: 1 MiB per candidate — a role return is kilobytes; the
ceiling is generous and named in the contract). Validation runs on
the snapshot; the accepted snapshot is written to
`<round>/reply-accepted.json` and THAT path is what the downstream
pipeline consumes — no second read of a mutable file, no
check-then-copy race. Named files are per-attempt:
`devin-return.json` for the initial attempt,
`devin-return.repair-1.json` for the repair (the prompt for each
attempt names its own path); both are preserved, the launch step
refuses to start if its attempt's file already exists (a stale file
is evidence of a previous crashed attempt, surfaced not clobbered).

## Transcript mining, authenticated (r1 findings 3, 4, 8)

The threat model, stated: a delegate already owns its return
channel — mining selects among the delegate's OWN writes and can
grant nothing stdout could not. The guards exist to stop the OTHER
failures: poisoned repository content or an early draft being
selected over final intent, and stale pre-repair material being
credited to a repair.

- **The designation rule (final intent, not convenience)**: a write
  qualifies ONLY if its target path's BASENAME equals the attempt's
  named return file (devin-return.json / devin-return.repair-1.json,
  any directory). The model using the exact name we gave it,
  wherever it put the file, is the designation of final intent;
  drafts under invented names never qualify.
- **The success oracle is the filesystem, not the transcript** (r3
  finding 1: stored ATIF tool results carry no success field, only
  free-form text — parsing English is not a predicate): a qualifying
  write DELIVERS only if the target file EXISTS at collect time
  (relative paths resolved against --workspace) and its content's
  sha256 equals the transcript argument's. A denied, rolled-back, or
  since-deleted write demonstrably did not persist and falls to the
  repair rung. Stated residuals, accepted: a model that skips stdout
  AND invents its own filename, or whose named-basename write no
  longer exists on disk, is served by the repair rung alone.
- **Attempt watermark, fail-closed**: the initial collect records
  the step count in `<round>/collect-watermark` ONLY when the
  bounded read was complete. A repair collect mines steps after the
  watermark; with no valid watermark (over-bounds or truncated
  initial read) the transcript rung is DISABLED for the repair —
  never a guessed boundary that could credit pre-repair material.
- **Per-candidate normalization BEFORE canonical validation** (r2
  finding 6): each candidate snapshot runs the existing normalizer
  extraction (fences, wrapper results, claimed-identity
  reconciliation — the pinned behaviors stay) and THEN the canonical
  validator; nothing supported today is deleted.
- **Write-tool calls only**, last qualifying match in-window.
- **Bounds are lifecycle-wide, through ONE snapshot** (r2 finding
  3, r3 findings 3+8): a new LEAF package `internal/atif` owns the
  bounded transcript read (8 MiB ceiling) and step iteration —
  internal/adapter already imports internal/usage, so the reader
  cannot live in adapter without a cycle; atif imports neither. The
  FIRST consumer of an attempt materializes
  `<round>/transcript.attempt-<n>.snapshot` via atif (copy-once,
  bounded); usage extraction, settlement, and collection ALL read
  that immutable snapshot path, and provenance hashes it — identity
  and mining are decided over the same bytes by construction, not by
  hoping three reads raced nothing. Per-candidate the 1 MiB snapshot
  ceiling applies. An over-ceiling transcript is a NAMED degraded
  terminal (transcript-oversize), never identity disagreement, never
  repair-eligible, never a wedge.
- **No re-mining downstream**: the accepted snapshot is handed
  forward as a file on EVERY channel (stdout and named-file
  included), so the normalizer's brace-scanning never runs on a raw
  transcript in this path at all.

## The repair rung, inside the existing state machine (r1 findings 5, 6, 7)

r1 was wrong twice here and r2 corrects both: the repair prompt is
authored in Go (adjudicate.go), and empty-vs-malformed must not
become parallel shell flows. The empty-delivery repair therefore
extends the ENGINE's adjudication state machine:

- `adjudicate.go` stays PURE (r2 finding 1): it gains the
  empty-delivery case as a RECOMMENDATION — repair-eligible when
  (CLI exit 0) AND (collect verdict is nothing-qualified, never
  mechanical) AND (a session correlated) — and authors the
  delivery-repair prompt exactly like the malformed prompt today.
  Record mutation never enters adjudication.
- **The claim is a new dispatch-owned conditional operation**:
  `job repair-claim` compare-and-swaps requiring `status == running
  && returnRepairs absent-or-0` atomically under the record lock —
  ABSENT MEANS ZERO by the verb's contract (r3 finding 6: neither
  record builder initializes the field, and they should not have
  to). The exit taxonomy separates the refusals: exit 3 =
  already-claimed (the repair is spent; the turn adjudicates
  empty-reply as a delegate outcome), exit 1 = mechanical
  (unreadable record, lock, authority) — a HARNESS failure that
  terminates degraded and never masquerades as delegate emptiness.
  The shell invokes the claim BETWEEN adjudication's recommendation
  and the paid launch. The current record-after-provider-call
  ordering with its ignored CAS failure is the defect this
  replaces. Authority: record-writer mode, the same matrix path
  every record mutation rides.
- **The repair interface separates provider exit from delivery**:
  the repair invocation reports its CLI exit; delivery is judged
  solely by the post-repair devin-collect walk (stdout, repair
  named file, transcript-after-watermark). The current
  `runtime_repair_turn` nonempty-stdout requirement is removed for
  devin — a file-only repair delivery is a success, which is the
  entire point of the rung.
- Repair usage, session settlement, cumulative-usage recomputation,
  and terminal CAS all stay where they live (the existing repair
  path); this design adds an entry condition and a collect-based
  delivery judgment, not a second flow.

## Precedence: two outcome tables (r1 finding 9, r2 findings 4, 7)

The INITIAL attempt (settlement runs FIRST, before CLI status is
inspected — today's order, kept deliberately and named: identity
adjudication precedes everything, so a nonzero call WITH settlement
disagreement terminates as session_identity_disagreement, not as
provider failure):

| Settlement | CLI exit | Collect facts | Outcome |
| --- | --- | --- | --- |
| disagreement/unreadable | any | never consulted | session_identity_disagreement, as today |
| transcript over ceiling | any | never consulted | transcript-oversize, a NAMED degraded terminal — never identity disagreement, never empty-reply, never repair |
| certified | non-zero | never consulted | provider failure, as today — no channel promotes output from a failed call |
| certified | 0, NO session correlated | candidatesPresent scan ONLY (the shipped gate's non-empty-parseable bar; no canonical validation — r5 finding 1, r7's width pin) | candidatesPresent=true → handshake_missing_session_id, today's gate for a parseable reply that arrived uncorrelated; candidatesPresent=false (nothing, torn, or non-persisted) → empty-reply adjudication, today's PINNED outcome (r6+r7: neither reclassification direction is permitted) |
| certified | 0 + session correlated | delivered | normal return pipeline on the accepted snapshot |
| certified | 0 + session correlated | nothing-qualified + claim exit 0 | the REPAIR attempt below |
| certified | 0 + session correlated | nothing-qualified + claim exit 3 | empty-reply adjudication, as today |
| certified | 0 | nothing-qualified + claim exit 1 | degraded harness terminal (the claim's mechanical taxonomy) |
| certified | 0 | mechanical | degraded terminal naming the harness failure — NEVER a paid repair |

An oversized or unreadable SINGLE candidate is not a mechanical
verdict: it falls through to the next rung with its rejection named
in provenance; mechanical is reserved for failures that make the
walk itself impossible.

The REPAIR attempt. Explicit precedence, matching the real state
machine (r5 finding 2 corrected r4's order): repair USAGE
EXTRACTION first, then the COMBINED REPAIR RESULT (CLI exit and
delivery together), and settlement LAST — only a repair that
otherwise succeeded reaches identity settlement, so a nonzero,
empty, or malformed repair is protocol-error today even when the
transcript disagrees, and stays so:

| Repair usage | Repair CLI exit + collect | Repair settlement | Outcome |
| --- | --- | --- | --- |
| extracted (always attempted; oversize snapshot → usage unavailable, noted) | non-zero | never reached | protocol-error, as today |
| extracted | 0 + nothing-qualified or mechanical | never reached | protocol-error, as today — a repair that did not deliver is a protocol violation, never a second empty-reply loop |
| extracted | 0 + delivered | disagreement/unreadable | session_identity_disagreement, as the repaired-session settlement maps today |
| extracted | 0 + delivered | transcript over ceiling | transcript-oversize degraded terminal |
| extracted | 0 + delivered | certified | normal pipeline on the accepted snapshot |

## Provenance, bound to bytes (r1 finding 11)

`<round>/reply-source.json` (written by the collect verb, only
after canonical validation): attempt, channel, sha256 of the
accepted snapshot, and the rejected list — plus, when the channel is
`transcript`, the full mining audit: selected step id, tool-call id,
target path, recorded tool outcome, the watermark in force, and the
sha256 of the bounded transcript snapshot the decision was made
over. With cumulative transcripts the artifact must prove not just
WHICH bytes delivered but WHY that event qualified and that it was
in-window; anything less is unauditable.

## Host scope: phase 2 of this design, host-shaped (r1 finding 11, r2 finding 2)

The host cannot reuse the job-shaped collector: a host return
validates against the TURN contract, and host.FinishTurn fails today
whenever raw.out is empty. The host half is an explicit SECOND PHASE
— committed scope, landing right after phase 1 under the same
priority ruling — and its interface is specified NOW so phase 2
implements rather than guesses (r3 finding 4, r4 finding 2): a
SEPARATE verb, `host devin-collect`, with --turn-record, --turn-id,
--workspace, --stdout, --named, --transcript, --round-dir. Its
pre-envelope check is EVERYTHING decidable before the envelope
exists: schema shape, turnId, missionId, cycle, and runtime/model
against the turn record. Session identity alone cannot move earlier
(the runner reads the observed session from the completed envelope),
so the walk is RESUMABLE: when the runner's post-envelope validation
rejects the selected candidate on session identity, it re-invokes
the collector with --reject <sha256-of-rejected-candidate> and the
walk continues from the next channel — a wrong-identity stdout can
delay but never destroy a valid named-file result (bounded: at most
one pass over the three channels, each candidate judged once).
FinishTurn gains an --accepted-reply input replacing its raw.out
read when collection delivered. Same snapshot/bounds/provenance
machinery via internal/atif; provenance records every rejected
candidate including post-envelope rejections. No host repair rung in
either phase: the runner's turn-retry machinery owns host failures.
Phase 1 ships no half-wired host path.

## Runtime scope

Devin-only mechanism; Claude and Codex have CLI-enforced structured
output, and the failure class cannot occur there. The generic form
(adapter-declared delivery channels) belongs to ACP (item 18); the
collect verb's interface is deliberately transport-shaped so item
18 can re-house it over typed events.

## What this deliberately does not do

No ACP client (item 18). No workspace sweeps. No streaming. No
claude/codex changes. No second repair. No new validator.

## Blast radius

Phase 1 (delegates):
- internal/atif (NEW LEAF): the bounded transcript reader, step
  iteration, and the per-attempt immutable snapshot — imported by
  adapter and usage without cycles.
- internal/adapter: devin-collect (walk, per-candidate
  normalization, snapshots, watermark, provenance; facts only —
  no repair recommendation), settlement through the atif snapshot,
  the return-complete library entry point, adjudicate.go's
  empty-delivery recommendation.
- internal/dispatch: the `job repair-claim` conditional CAS.
- cmd/metasystem: devin-collect and repair-claim verb rows.
- internal/usage: transcript reads through the bounded reader.
- scripts/agents/adapters/devin.sh: collect integration,
  per-attempt named paths, repair exit reported separately;
  runtime-common.sh: the devin repair-shape exception.
- Fixtures: devin selftest ladder legs.

Phase 2 (hosts): scripts/agents/hosts/devin.sh, internal/host
(FinishTurn's accepted-snapshot input), internal/missionrunner (the
turn-contract validator wiring), a host-path selftest leg.

## Proof obligations

- Selection: junk stdout + valid named file → named file; torn
  named file + valid transcript → transcript; wrong-job
  schema-valid stdout + valid named file → named file; every
  rejection named in provenance.
- Mining: last-match within attempt window; watermark excludes
  pre-repair steps (fixture uses a cumulative two-attempt
  transcript); denied/failed writes with intact arguments do not
  deliver unless canonically valid AND in-window; over-bounds
  transcript → degraded reason, not a wedge and not "no return".
- Repair: durable claim CASed before launch (crash between claim
  and launch → no second repair, fixture kills the harness there);
  file-only repair delivery succeeds; provider nonzero exit on
  repair → failure regardless of files; one repair ever.
- Precedence: every row of the outcome table as a fixture leg,
  including nonzero-exit-with-valid-named-file (must NOT deliver)
  and unreadable-transcript-with-valid-stdout (must fail
  settlement).
- Snapshots: mutation of the named file after collect does not
  change what downstream reads; per-attempt files both preserved;
  pre-existing attempt file refuses launch.
- Provenance: sha256 matches accepted bytes on every path; kit
  extractor tolerates the new files (additive).
- Host: the host-turn collect recovers a file-delivered host result
  end to end in the selftest; a wrong-identity stdout candidate is
  rejected post-envelope and the walk RESUMES to deliver the valid
  named file (r4 finding 2's exact scenario).
- Initial no-session split (r5 finding 1 + r6 + r7): with no
  correlated session and a PARSEABLE candidate, the turn refuses
  handshake_missing_session_id with no validation artifacts; with
  nothing, a torn file, or a non-persisted attempt, empty-reply
  adjudication exactly as pinned today — all four shapes as fixture
  legs (parseable stdout; parseable named file; torn named file;
  empty everything).
- Repair precedence: a nonzero repair with a disagreeing transcript
  is protocol-error (settlement never reached); a DELIVERED repair
  with a disagreeing transcript is session_identity_disagreement;
  repair usage is extracted in every row (r5 finding 2).
- Regression, corrected (r3 finding 2): the D62 frozen transcript
  — whose write predates the named-path instruction and targets an
  invented filename — must NOT deliver via mining (the designation
  rule's negative case); its shape is served by the repair rung. The
  POSITIVE mining regression is synthetic: a named-basename write to
  a foreign directory (e.g. /tmp/devin-return.json) with the file
  present and digest-matching recovers via rung 3; the same write
  with the file absent falls to repair (the success-oracle case).
