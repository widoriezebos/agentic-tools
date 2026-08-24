# Honest emulator streams and boundary-ask events (acp-adapter-seam slice three)

Working Mode: design

Owner: m2 session under the acp-adapter-seam claim (slices one and
two landed 358f970 / 99ab15a). Appetite 6h, Wido-ratified
2026-08-24. Critique loop failsafe, declared at open: round 3 —
land with residue there. Round 1
(design-critic-20260824t220232z-aa03): 8 material, all folded in
this v2; dispositions at the end.

## The step being implemented

`emulator upgrades — structured event streams and turn-boundary
asks for the CLI-runtime ports, honestly declared` (the goal step).
Slice one built the seam's vocabulary; slice two shipped the one
complete native driver. This slice upgrades what the CLI runtimes'
read-side PORTS honestly serve: a normalized post-hoc event
projection per runtime, a real stream for claude (whose probe
declaration currently outruns its adapter), and the turn-boundary
ask candidates surfaced AS EVENTS — projection of what exists, not
invention of what does not.

## What the runtimes actually have (recon, 2026-08-25)

codex: a true native JSONL stream on disk already (`codex exec
--json` → round events.jsonl, live). claude: probe declares
nativeEvents true and lists stream-json among available transports,
but the dispatch argv runs blocking `--output-format json`; the
round events.jsonl carries exactly two hook/adapter lines. devin
legacy: post-hoc ATIF transcript (usage/settle territory) plus one
synthetic session-correlated line; devin acp rounds carry
acp-launched + session-correlated synthetic lines beside the native
journal. fake: hand-printed deterministic lines. Asks: every CLI
runtime structurally suppresses mid-turn asks; the only ask-shaped
turn data is the ORCHESTRATOR-SCHEMA return's askCandidates
({streamId, reasonClass, question, supersedes} — no ids, no
options; ids are allocated later by adjudication).

## Shape: two port fields become one, plus one adapter move

### 1. The Events port — post-hoc, normalized, per-runtime

`delegate.Ports` gains ONE field:

    // Events projects the round's native event artifacts into the
    // seam's Event vocabulary, post hoc. Nil = the runtime records
    // no projectable events; callers refuse by name. The
    // projection is bounded (8 MiB per artifact, the ATIF
    // precedent, exported constant; over-ceiling is an ERROR,
    // never a silent truncation), order is the artifact's own line
    // order, params are the line's RAW BYTES verbatim
    // (json.RawMessage, no map round-trip), and skipped
    // unparseable lines are counted and surfaced as one final
    // <runtime>/lines-skipped event whose params are EXACTLY
    // {"skipped":<count>} (the slice-two loss-visibility
    // precedent; a distinct key from the driver's "dropped"
    // because the meanings differ — malformed input vs overflow).
    Events func(roundDir string) ([]Event, error)

A PORT by slice-one law: it interprets native artifacts, keyed by
runtime alone. The devin registration follows the COLLECT precedent
explicitly: the port projects whatever synthetic lines the round
holds — legacy's session-correlated and acp's acp-launched alike —
because the bytes mean the same however the session was driven; the
native LIVE stream stays driver territory, and nothing here spans
into the journal. (Round 1's F5 contradiction is resolved by this
paragraph: no reservation sentence survives.)

Kind law, per runtime, keyed to the fields the artifacts REALLY
carry (F4):
- codex: `codex/<type>` (thread.started, turn.completed, ...);
  absent type → `codex/unlabeled`.
- claude: `claude/<type>` with `.<subtype>` appended when a
  subtype field is present (`claude/system.init`,
  `claude/result.success`, `claude/assistant`); absent type →
  `claude/unlabeled`.
- devin: `devin/<type>` (session-correlated, acp-launched); absent
  → `devin/unlabeled`.
- fake: first present of `type`, then `event` (fake.sh prints event
  strings; topLevel is a boolean and never a kind) →
  `fake/<value>`; neither present → `fake/unlabeled`.
- Every projected kind is `<runtime>/<value>` — visibly distinct
  from the native driver's `update/` and `driver/` namespaces so
  post-hoc replay can never impersonate a live wire.
- Seq: see the SEQ LAW below — one statement only (the projection
  ordinal), stated once so no second rule can contradict it.

### 2. Boundary-ask candidates are a HOST-side twin (F1, S3-R2-004)

askCandidates exist ONLY in the orchestrator schema — a host-turn
return concern, owned by mission adjudication; no delegate v2
return carries them. So the delegate Events port never touches
return documents (the layering smell is gone), and the goal's
"turn-boundary asks" lands where asks actually live: `Ports` gains
the host twin

    // HostBoundaryAskEvents projects the accepted host return's
    // askCandidates as events: kind
    // `<runtime>/boundary-ask-candidate`, params = the candidate
    // object verbatim, seq = 1-based candidate ordinal. Missing or
    // candidate-less return → empty, nil error; malformed → error,
    // never invention. No delegate.Ask synthesis: ids are
    // adjudication's to allocate, options do not exist, and
    // Ask/Answer stays driver-territory.
    HostBoundaryAskEvents func(acceptedReturnPath string) ([]Event, error)

registered for the host-launcher runtimes via ONE shared projection
in the adapter package (the accepted-return document is
runtime-neutral). Answering stays exactly where it lives (mission
adjudication and follow-up turns); a future slice may wire the
seam's Ask/Answer to that machinery with its own design.

### 3. Claude joins the streamers — without touching the host (F2, F3)

- `BuildClaudeCommand` gains an EXPLICIT output-mode parameter. The
  DISPATCH adapter passes stream-json — WITH the mandatory
  `--verbose` companion flag (the installed CLI refuses print-mode
  stream-json without it; S3-R2-001) — while the HOST launcher
  keeps json and is untouched byte-for-byte (host changes stay a
  non-goal; the parameter makes the divergence typed instead of
  accidental, and proof 7 pins the host argv against a recorded
  golden).
- SINGLE-WRITER FILES (F3): the stream goes to its own artifact,
  `claude-stream.jsonl` — the CLI process is that file's only
  writer, ever. events.jsonl keeps its exact two appended lines
  (session-init from the hook; the result line appended by the
  adapter AFTER deriving claude-result.json) and its append
  discipline — so the dead-round recoverer and every other consumer
  keep their exact inputs.
- ARTIFACT SELECTION, not concatenation (S3-R2-002): the claude
  projection uses claude-stream.jsonl as the SOLE event artifact
  when it exists (streamed rounds) and events.jsonl otherwise
  (legacy blocking rounds) — never both, so the final result event
  can never appear twice and no cross-file ordering exists to get
  wrong. The recoverer additionally learns the stream file as a
  second artifact to walk when events.jsonl yields nothing (a dead
  streamed round has no appended result line; its partial stream is
  the only usage evidence) — one added path in claude's registered
  recoverer, proof-pinned.
- SEQ LAW (S3-R2-003): Event.Seq for an emulator projection is the
  1-based ordinal IN THE PROJECTED LIST — one monotonic counter
  over the selected artifact's lines. The artifact chosen is the
  order authority; provenance is carried by Kind alone. (The native
  driver's wire-seq semantics are driver-territory and unchanged;
  the two meanings never meet because the namespaces never mix.)
- `claude-result.json` is DERIVED at turn end from the stream's
  final result-typed event, by the adapter, and every existing
  consumer (ClaudeUsage, ResultField, dead-round recovery, collect)
  reads exactly the file it reads today. A stream that ends without
  a result-typed event takes the existing missing-result failure
  path — same classification, no document invented from partial
  events.

## Honesty pins (F8)

The probe snapshot has no "selected transport" — it stores an
availability array plus the nativeEvents boolean. The pin is the
IMPLICATION — declared nativeEvents must match what the dispatch
argv actually does — enforced exactly as far as a joint exists
(S3-R2-005, S3-R3-002): builder-side, claude's and codex's Go
builders get static assertions (claude stream+verbose, codex
--json); devin's inline shell argv gets the dispatch-fixture grep
against its declared false. DECLARATION-SIDE population coverage is
RESIDUE: the runtimes registry has no emulator-events data field
today, so no conformance loop can join "what the probe declares"
to "what the builder does" for runtimes not named above — the
future shape is one pure-data field in the runtimes registry
(the ACPCapabilities precedent) joined in runtime conformance;
recorded on the goal at landing, not smuggled into this slice.
The recon's found lie — claude declared true, built blocking —
still becomes unrepresentable at the builder joint.

## Proof obligations

1. Per-runtime Events projection: golden round dirs lifted from
   real fixture shapes → exact Event lists (kinds, seqs, raw-byte
   params); malformed tail → lines-skipped event with exact count;
   over-8MiB artifact → error; empty/missing artifacts → empty
   list, nil error.
2. Boundary-ask events: return with candidates → one event per
   candidate, verbatim params, seq continuing; no return / no
   candidates → nothing; malformed return → error.
3. Claude stream derivation (F6, honestly scoped): the mission
   fixtures' fake claude CLI gains stream-json emission of the SAME
   scripted conversation it emits blocking today; the derivation
   from the streamed fixture yields a claude-result.json
   byte-identical to the blocking fixture's document, and
   ClaudeUsage + ResultField + the dead-round recoverer read
   identically over both. The fake is the executable CLI contract
   (the repo's standing idiom); first real dispatch remains the
   live validation, noted in the delivery contract.
4. Missing-final-result stream → the existing missing-result
   failure, same message class.
5. Ports law, BOTH new fields (S3-R3-005): Events and
   HostBoundaryAskEvents each get their explicit merge branch in
   RegisterPorts (the merge is manual, not reflective), each
   double-registration panics, each nil-field refusal names its
   field, and the slice-one differential fixtures extend to both —
   a registration that forgets a branch fails the fixture, not the
   field silently.
6. The nativeEvents→argv implication (Honesty pins above): the
   conformance-suite check over every Go-builder runtime, plus the
   shell-fixture assertion for devin; claude's stream argv must
   also carry --verbose (the CLI's stream-json precondition).
7. Host argv golden (S3-R2-006): the host-mode builder output is
   compared against the recorded pre-slice argv exactly — json
   output, no --verbose, no stream flag — extending
   claudecommand_test's host case to a full-vector comparison.
8. Streamed-round recovery: a dead streamed round (stream file with
   usage-bearing lines, no appended result line) recovers usage
   through the extended recoverer; a dead blocking round recovers
   exactly as today.
9. THE SHELL JOINT (S3-R3-001): the dispatch fixtures run the REAL
   claude.sh dispatch flow against the fake CLI in stream mode —
   the launch redirection (the one changed line), the NUL-token
   command readback, the session-signal polling, and wait_for_cli
   all survive; claude-stream.jsonl is written by the CLI process
   alone; claude-result.json is derived; the round completes
   delivered. Derivation-function unit tests (proof 3) do not
   substitute for this leg.

## Non-goals

- No emulator Drivers; ports stay ports.
- No delegate.Ask synthesis and no Answer plumbing (future slice).
- No devin transcript→event invention; the journal stays the
  native driver's.
- No host launcher changes (the output-mode parameter defaults the
  host to today's json).
- No live tailing API for emulator events.

## Round-1 disposition (design-critic-20260824t220232z-aa03)

- F1 folded: BoundaryAsks port dropped; candidates project as
  events, verbatim, ids/options never invented.
- F2 folded: explicit output-mode parameter; host path
  byte-untouched.
- F3 folded: claude-stream.jsonl single-writer artifact;
  events.jsonl untouched; three-source projection order documented.
- F4 folded: kind law rewritten against the artifacts' real fields,
  per runtime, with unlabeled fallbacks.
- F5 folded: devin Events adopts the collect precedent (projects
  both transports' synthetic lines); the contradictory reservation
  removed.
- F6 folded: fixture-contract equivalence via the fake CLI's dual
  modes; real-CLI byte claims withdrawn; live validation noted for
  first dispatch.
- F7 folded: 8 MiB ceiling (exported, ATIF precedent),
  over-ceiling errors, raw-byte params, counted skips surfaced as
  the lines-skipped event.
- F8 folded: the pin is the nativeEvents→argv implication, tested
  at the builder; "selected transport" language withdrawn.

## Round-2 disposition (design-critic-20260824t221003z-2ffb)

- S3-R2-001 folded: --verbose rides stream-json in the dispatch
  argv; proof 6 asserts it.
- S3-R2-002 folded: artifact SELECTION replaces concatenation
  (stream file sole authority when present; events.jsonl for legacy
  rounds); result-line append order documented; the recoverer
  learns the stream file as a fallback artifact (proof 8).
- S3-R2-003 folded: Seq = projection ordinal, one counter, order
  authority named; native wire-seq semantics untouched.
- S3-R2-004 folded: boundary candidates move to
  HostBoundaryAskEvents (askCandidates are orchestrator-schema
  only); the delegate Events port no longer reads returns.
- S3-R2-005 folded: the implication joins runtime conformance for
  Go-builder runtimes (future-proof by population), shell-fixture
  half for devin, weakness named.
- S3-R2-006 folded: proof 7 pins the host argv against the recorded
  golden, full vector.

## Round-3 disposition (design-critic-20260824t222021z-655f, failsafe round)

- S3-R3-001 folded: proof 9 exercises the real claude.sh dispatch
  flow against the stream-mode fake CLI — the changed shell joint
  is a named fixture leg, not a derivation unit's shadow.
- S3-R3-002 folded as scoping + RESIDUE: builder-side assertions
  land now; declaration-side population coverage awaits a runtimes
  registry data field (future slice, recorded on the goal).
- S3-R3-003 folded: the seq contradiction removed — one law, the
  projection ordinal, stated once.
- S3-R3-004 folded: lines-skipped params pinned to
  {"skipped":<count>}.
- S3-R3-005 folded: proof 5 names both fields, both branches, both
  panics, both fixture extensions.

LANDED AT THE FAILSAFE: rounds ran 8 → 6 → 5 material findings, all
folded; one residue recorded (the declaration-side conformance
joint). Implementation begins.
