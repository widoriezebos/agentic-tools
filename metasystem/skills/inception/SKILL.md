---
name: inception
description: Guide a human through the birth of an app under the metasystem — vision, criteria, the minimal guardrail net, covenant v1, doctrine v1, and the steel-thread backlog — for a fresh repository or as a retrofit into a mature one. Use on the coordinator seat, outside every mission, after adoption (fresh) or reconciliation (mature). Do not use during an active mission, and never to edit an existing covenant — that is the warden's lane and the human tier.
---

# Inception

Inception is the moment the iterative game becomes playable: the human's
intent enters the covenant, the covenant's net gets its first strands,
and the first goal opens. You run the interview; the human rules. You
have no authority here beyond shaping — every artifact is displayed and
confirmed before it is written, and the human performs the one act that
is theirs by construction.

Two variants share this flow. A FRESH repository arrives through
adoption with a Goal-free ledger, no covenant, and a placeholder
`docs/project-rules.md`. A RETROFIT arrives through reconciliation into
a mature app that already owns tests, benchmarks, gates, and documents —
there the interview HARVESTS instead of authoring: covenant rows point
at what exists, by reference, never by rewrite. The retrofit is the
dominant real case; treat the fresh path as its simplification.

## Preconditions — refuse before you begin

1. The repository is adopted (fresh) or reconciled (mature) at a known
   template SHA and structurally healthy — the adoption's own audit
   passed. Full validation with zero placeholders is step 8's exit
   bar, not the entry bar: a fresh adoption arrives WITH placeholders,
   and filling them is this interview's work.
2. No mission is active — checked observably, not assumed: every
   `artifacts/agents/missions/*/state.json` is terminal or absent, and
   this session carries no mission stamp. A covenant created inside a
   mission escalates to the human tier by the wall's own law; a
   coordinator-side write DURING someone else's mission is outside the
   wall's sight, which is why this check is yours. Repeat it
   immediately before step 8 — the world can change during a long
   interview.
3. No `covenant.json` exists. Inception births a covenant; it never
   edits one. An existing covenant routes to the net-review lanes, not
   here.
4. The human is present. This is a conversation, not a batch job.

## Step 1 — Harvest (retrofit) or survey (fresh), read-only

Before asking the human anything, learn the terrain so the interview is
concrete. For a retrofit, inventory: specifications and design
documents, test suites and their entrypoints, benchmark harnesses,
thresholds and cost controls, golden sets, CI gates, and any current
run evidence. Record WHERE each lives and HOW it is invoked. Touch
nothing, run nothing paid, and hold the inventory lightly — existing
machinery informs the interview; it must never redefine the human's
vision. For a fresh repository, survey what adoption installed and note
the placeholder surfaces the interview will fill.

## Step 2 — Vision, in English

Ask, in plain language, and write down verbatim answers: who the app is
for, what outcome it exists to produce, where its boundaries are, and
what it deliberately is NOT. This becomes the identity's ground and the
doctrine's opening. Do not translate into schema yet — vision first,
structure later.

## Step 3 — Criteria with stable IDs

Turn the vision into functional and non-functional criteria, each with
a stable ID the covenant and the backlog will both cite, each stating
what OBSERVABLE success means. Non-functional criteria (latency, cost,
dependency posture, determinism) are criteria like any other — they get
IDs and eventual proofs, not adjectives.

Then ELICIT the high-level design before writing a word of doctrine:
ask the human, in English, about the system's boundaries and
components, how data flows between them, the tradeoffs already decided,
and the patterns chosen and refused — you scaffold what the human
says, never what you would have designed. Only then write
`docs/app-doctrine.md` v1 for a fresh app — the architecture intent,
the patterns chosen and refused, the conventions delegates must honor. For a retrofit, the doctrine INDEXES
the existing canonical design documents by path rather than duplicating
a word of them. Doctrine v1 is one scaffold; its evolution loop is the
app-doctrine goal's, not yours.

## Step 4 — The evidence table, before any net is authored

This is the interview's spine, and it outlives the interview. For every
criterion, one row:

| criterion | proof id | exact command or selector | evidence source | status |

Status is exactly one of: `observed` (you or the human ran it against
the CURRENT tree during this interview and saw the result),
`referenced-not-run` (it exists and is invocable, but was not exercised
now — the honest retrofit default for anything paid or slow), or
`planned-floating` (no executable proof exists yet). Never launder a
floating row into a guarantee: floating rows become backlog goals in
step 6, and the row says so.

Persist the table as `docs/covenant-evidence.md` — app-owned, under the
same never-overwritten law as the covenant. It is the record a future
net review re-derives against the tree; give it stable IDs and exact
commands so a machine can.

Total coverage is the table's law: every confirmed criterion maps to
a covenant requirement row or to an EXPLICIT deferred gap the human
accepted by name — never silently dropped. Display the wired-versus-
floating counts whenever you show the table.

## Step 4b — The configuration, interviewed

`metasystem.conf` is the other half of adoption's finish line, and it
is interview material, not defaults: ask which runtimes serve this
app, which models at which tiers, the role assignments where they
differ, and the durable evidence root where run evidence must
survive. Fill the configuration from the answers and carry it into
step 7's confirmation set — a fresh adoption cannot pass final
validation without it.

## Step 5 — The minimal net, from the table only

Author the proposed `covenant.json` (schema v1: identity, requirements,
battery, budgets, guards, guardrails — the reader refuses anything
else) with these disciplines:

- Every requirement row cites a criterion ID in `ref` and names a proof
  from the table in `proof`. A proof name that appears nowhere in the
  table is an invented proof; do not write it.
- The battery is the ONE command that earns green. Define what green
  means BEFORE observing the current score — a threshold chosen after
  peeking is laundering. For a retrofit, preserve the accepted
  threshold by reference; weakening it is a human-tier governance
  change and you must say so out loud.
- The battery must actually measure: it emits `metric=<id>=<value>` and
  exits 0 whenever measurement ran (non-zero means the battery gate could not
  measure, never that the work is unfinished). An existing suite
  harvested by reference gets a thin wrapper to this grammar. TRACE
  the reach: name which proofs from the table the battery's command
  actually exercises — a dynamic but irrelevant metric satisfies every
  schema and guards nothing. Where a cheap, safe witness exists,
  record one: a known-bad input or variation whose score visibly
  moves. Full mutation adequacy is the counselor's later work, but
  the reach trace is yours now.
- A budget or guard earns its row only whole: an owner, a unit, the
  collecting command or source, a baseline, the bound or floor, and
  the exact `metric=<id>=<value>` line it emits. Anything less is
  OMITTED and becomes a goal. Empty `budgets` and `guards` lists are
  lawful and more honest than invented measurement.
- Guardrails are derived per path, with a displayed reason each, and
  the derivation must CLOSE over the net's own dependencies: proof
  commands, their wrappers, threshold sources, specs and goldens, the
  doctrine, and the evidence memory. Every considered-and-omitted
  path gets its reason displayed too. Never guard ordinary product
  source wholesale, and never list the goal ledger —
  the goal machinery custodies itself. Entries are canonical
  repository-relative paths, comma-free; the covenant custodies itself
  by construction, so `covenant.json` never needs listing.
- Retrofit references must resolve to repository-owned, versioned
  commands or named external evidence. An absolute path into a sibling
  checkout, a live secret, or an unpinned asset is a GAP to record, not
  silently valid content.

## Step 6 — The steel-thread backlog, from the gaps

Derive the first goals: the steel thread itself (the thinnest
end-to-end slice that makes the battery's metric real), plus one goal
per floating proof and per omitted budget or guard. Each goal passes
the intake checklist as written in `docs/backlog-mechanism.md`: intent
says what done looks like in one line, the piece is independently
deployable, the appetite is agreed with the human and recorded as the
`Appetite:` token, and the goal CITES the covenant requirement and
proof IDs it will make real. A goal that closes no criterion fails
intake here, in conversation, where it is cheap.

## Step 7 — Display, then confirm, once

Show the human everything as it will be written: the covenant, the
doctrine, the filled `docs/project-rules.md` facts, the filled
`metasystem.conf`, the evidence table with its wired-versus-floating
counts, the guardrail list with its per-path (and per-omission)
reasons, and the exact goal-opening commands. One confirmation for the set; amendments loop
back to the step that owns them.

## Step 8 — Finalize, outside the mission

On the human's confirmation: write the confirmed set — the app-owned
artifacts (`covenant.json`, `docs/app-doctrine.md`,
`docs/covenant-evidence.md`, the `docs/project-rules.md` facts) and
the filled `metasystem.conf` — then prove the shape mechanically:

    bin/metasystem covenant validate

Its success line says "shape valid; adequacy not established" — repeat
that honesty to the human, because it is the truth: the parser proves
the rows parse, never that the proofs guard the intent. Then run
`scripts/validate-metasystem.sh`; it must pass with zero placeholders.
Exercise only safe, already-approved checks — anything paid or slow
stays `referenced-not-run` with its row saying so.

## Step 9 — The human opens the goals

Goal origin derives from who runs the command: the human's asks carry
the human's authority gates only when the human's own hand opens them.
Generate the exact `goal open` command(s) — id, intent with its cited
IDs, next step with its `Appetite:` token — and hand them over for the
human to run. Zero transcription: the command is complete as displayed.
The first open clears the adopted repo's declared Goal-free by the
machinery's own design.

## Step 10 — Declare playable, honestly

The game is playable when: the metasystem validates; the battery
actually measures and emits its metric WITH its reach traced (which
proofs it exercises, and the witness where one was safe to record) — even if the score is below
threshold today, because a thin steel thread SHOULD start below
threshold; and the first ready goal is open. A dummy battery printing a
constant green is not playable, it is a hollow covenant wearing a valid
shape. Close by telling the human what the first mission's contract
must carry: `covenant.path=covenant.json`, a `gate.command`,
`gate.threshold.<metric>`, and `gate.direction` equal to the battery,
and `wall.guardrails` equal to the covenant's net — the preflight gate
refuses anything less, in both directions.
