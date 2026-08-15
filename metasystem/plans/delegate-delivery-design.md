# Delegate delivery: every channel a return can arrive by, and the guards on each

Working Mode: design

Owner: main session (delegate), under Wido's 2026-08-15 morning
rulings (design-critique-implement before anything else; everything
Go is better suited for goes in Go). Status: r2 — folds all eleven
r1 findings (critique preserved at
plans/delegate-delivery-critique-r1.md) and the Go-boundary ruling;
awaiting r2 critique.

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

- **Attempt watermark**: the initial collect records the
  transcript's step count in `<round>/collect-watermark`. A repair
  collect mines only steps AFTER the watermark. Exports are
  cumulative (verified: a round-2 export contains round 1
  byte-for-byte), so the watermark is the attempt boundary the
  transcript itself lacks.
- **Full canonical validation** of the mined content (not jobId
  alone — jobId is prompt-disclosed and forgeable by content the
  session merely read).
- **Write-tool calls only**, last qualifying match wins within the
  attempt window; exec arguments are never mined.
- **Bounds**: the verb refuses transcripts beyond 8 MiB and stops
  after the first 4096 steps (both far above every observed export,
  named so growth is a loud failure, not a wedge); per-candidate
  the 1 MiB snapshot ceiling applies. A delegate cannot wedge the
  collect step with a giant write argument.
- **No re-mining downstream**: because the accepted snapshot is
  handed forward as a file, the existing normalizer's own
  transcript-brace-scanning never runs on a transcript the collect
  verb already mined (the shell passes reply-accepted.json, not the
  transcript, when the channel was `transcript`).

## The repair rung, inside the existing state machine (r1 findings 5, 6, 7)

r1 was wrong twice here and r2 corrects both: the repair prompt is
authored in Go (adjudicate.go), and empty-vs-malformed must not
become parallel shell flows. The empty-delivery repair therefore
extends the ENGINE's adjudication state machine:

- `adjudicate.go` gains the empty-delivery case: verdict
  repair-eligible when (CLI exit 0) AND (collect found nothing) AND
  (a session correlated) AND (the durable repair claim below
  succeeds). It authors the delivery-repair prompt (naming the
  repair attempt's return path and the print instruction) exactly
  like it authors the malformed-repair prompt today.
- **The one-shot is claimed durably BEFORE launch**: the
  returnRepairs record patch is compare-and-swapped first; a failed
  CAS is terminal (already claimed = never a second paid repair,
  even across a crash or re-entry). The current
  record-after-provider-call ordering with its ignored CAS failure
  is named as the defect this fixes.
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

## Precedence: the outcome table (r1 finding 9)

| CLI exit | Settlement (transcript vs session) | Collect result | Outcome |
| --- | --- | --- | --- |
| non-zero | any | never consulted | provider failure, exactly as today — no channel promotes output from a failed call |
| 0 | disagreement/unreadable transcript | never consulted | session_identity_disagreement, as today: identity certainty outranks delivery (a reply we cannot attribute is not a reply) |
| 0 | certified | delivered (any channel) | normal return pipeline on the accepted snapshot |
| 0 | certified | none + repair-eligible | one repair, then this table re-applies with attempt=repair (no second repair row exists) |
| 0 | certified | none + not eligible | empty-reply adjudication, as today |

Mining exit 3 (nothing qualified) falls through inside the collect
walk; exit 1 (mechanical: over-bounds, unreadable) surfaces as a
degraded reason and the turn adjudicates as if the channel were
absent — never silently as "no return existed".

## Provenance, bound to bytes (r1 finding 11)

`<round>/reply-source.json` (written by the collect verb, only
after canonical validation): `{"attempt": "initial|repair",
"channel": "...", "sha256": "<of the accepted snapshot>",
"rejected": [{"channel": ..., "reason": ...}]}`. The single-token
file from r1 is dropped — a bare `repair` token erased the
channel-within-repair and bound to nothing.

## Host scope (r1 finding 11)

hosts/devin.sh gains the named return path in its prompt (the
host-turn variant of devin-prompt already exists; it gains the same
--return-file) and the collect walk (rungs 1-3) for its result. No
host repair rung: the runner's turn-retry machinery owns host
failures. The host integration is part of THIS change, not future
work — the bias is live there today.

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

- internal/adapter: devin-collect (walk, snapshots, bounds,
  watermark, provenance), the return-complete library entry point,
  adjudicate.go's empty-delivery case and durable repair claim.
- cmd/metasystem: the devin-collect verb row.
- scripts/agents/adapters/devin.sh: collect-verb integration,
  per-attempt named paths, repair invocation reporting exit
  separately; runtime-common.sh: the devin repair-shape exception.
- scripts/agents/hosts/devin.sh: named path + collect rungs 1-3.
- Fixtures: devin selftest ladder legs; a host-path leg.

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
