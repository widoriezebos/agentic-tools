# Delegate delivery: every channel a return can arrive by, and the guards on each

Working Mode: design

Owner: main session (delegate), under Wido's 2026-08-15 morning
rulings (design-critique-implement before anything else; everything
Go is better suited for goes in Go). Status: r3 — folds the eight r2
findings (critique at plans/delegate-delivery-critique-r2.md; r1 at
-critique-r1.md); awaiting r3 critique.

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

Inputs per call: --job, --round-dir, --stdout FILE, --named FILE,
--transcript FILE, --schema FILE, --record FILE, --attempt
initial|repair. Output: a JSON verdict on stdout —
`{"delivered": bool, "channel": "stdout|named-file|transcript|none",
"reply": "<path of the accepted snapshot>", "repairEligible": bool,
"reason": "..."}` — plus the side effects below. Exit codes: 0
delivered, 3 nothing qualified, 1 mechanical failure (unreadable
inputs beyond bounds).

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
  any directory) AND its recorded tool outcome is success. The model
  using the exact name we gave it, wherever it put the file, is the
  designation of final intent; drafts under invented names (r2's
  evidence: a denied validation draft that was schema-valid) never
  qualify. Stated residual, accepted: a model that both skips stdout
  AND invents its own filename is served by the repair rung alone.
  Denied writes no longer deliver — under D61's dangerous mode
  writes are not denied, so this costs the recovery path nothing.
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
- **Bounds are lifecycle-wide** (r2 finding 3): one shared bounded
  transcript reader in internal/adapter (8 MiB ceiling) becomes the
  ONLY way usage extraction, settlement, and collection read a
  transcript; per-candidate the 1 MiB snapshot ceiling applies. An
  over-ceiling transcript is a NAMED degraded terminal
  (transcript-oversize), distinct from identity disagreement and
  never repair-eligible — a loud harness verdict, not a wedge and
  not an empty reply.
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
  `job repair-claim` compare-and-swaps requiring
  `status == running && returnRepairs == 0` atomically under the
  record lock (the existing record-cas compares status only and
  cannot express this — named as why the verb is new). The shell
  invokes it BETWEEN the recommendation and the paid launch; a
  failed claim is terminal for repair (already claimed = never a
  second paid repair, across crash or re-entry). The current
  record-after-provider-call ordering with its ignored CAS failure
  is the defect this replaces. Authority: record-writer mode, the
  same matrix path every record mutation rides.
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

The INITIAL attempt:

| CLI exit | Settlement | Collect verdict | Outcome |
| --- | --- | --- | --- |
| non-zero | any | never consulted | provider failure, as today — no channel promotes output from a failed call |
| 0 | disagreement | never consulted | session_identity_disagreement, as today: identity outranks delivery |
| 0 | transcript over ceiling | never consulted | transcript-oversize, a NAMED degraded terminal — never identity disagreement, never empty-reply, never repair |
| 0 | certified | delivered | normal return pipeline on the accepted snapshot |
| 0 | certified | nothing-qualified + session correlated | repair recommendation → `job repair-claim` → the REPAIR table |
| 0 | certified | nothing-qualified, no session or claim refused | empty-reply adjudication, as today |
| 0 | certified | mechanical | degraded terminal naming the harness failure (unreadable schema/record, snapshot or provenance write failure) — NEVER a paid repair, exactly the taxonomy the adjudication tests already pin for unreadable schemas |

An oversized or unreadable SINGLE candidate is not a mechanical
verdict: it falls through to the next rung with its rejection named
in provenance; mechanical is reserved for failures that make the
walk itself impossible.

The REPAIR attempt (aligned with the existing after-repair
protocol-error taxonomy, not the initial table):

| Repair CLI exit | Post-repair collect | Outcome |
| --- | --- | --- |
| non-zero | never consulted | protocol-error, as the after-repair path maps today |
| 0 | delivered | repaired-session settlement, then the normal pipeline (settlement order unchanged from today's repair path) |
| 0 | nothing-qualified or mechanical | protocol-error, as today — a repair that did not deliver is a protocol violation, never a second empty-reply loop |

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

The host cannot reuse the job-shaped collector as r2 sketched: a
host return validates against the TURN contract (turnId, missionId,
cycle, the announced-or-observed session rule), and host.FinishTurn
fails today whenever raw.out is empty. The host half is therefore an
explicit SECOND PHASE of this same design — committed scope, its own
proof legs, landing right after phase 1 under the same priority
ruling: hosts/devin.sh gains the named return path; the collect walk
gains a host mode whose validator is the turn contract (internal/
host + internal/missionrunner join the blast radius honestly);
FinishTurn accepts the accepted-snapshot input. No host repair rung
in either phase: the runner's turn-retry machinery owns host
failures. Phase 1 does not ship a half-wired host path — hosts stay
exactly as today until phase 2 lands whole.

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
- internal/adapter: devin-collect (walk, per-candidate
  normalization, snapshots, bounds, watermark, provenance), the
  shared bounded transcript reader adopted by usage extraction and
  settlement, the return-complete library entry point,
  adjudicate.go's empty-delivery recommendation.
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
  end to end in the selftest.
- Regression: the D62 frozen transcript replays to a rung-3
  recovery with full validation.
