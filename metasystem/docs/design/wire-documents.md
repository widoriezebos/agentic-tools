# Wire documents: the on-disk JSON contract

The durable record of what go-production-grade Phase 5 built and the rules
that keep it true. The plan and its dispositions are deleted at close-out;
this is the part the next change to any record format must know.

## The shape

Every JSON document the metasystem persists — job records, turn records,
mission state, fence counters, host results, adapter records — is read and
written through `internal/wiredoc`:

- **`wiredoc.Decode`** is the one accepted-input grammar: `UseNumber`
  (literal number spellings preserved end to end), duplicate keys
  last-wins, trailing bytes after the top-level value tolerated, top-level
  non-objects refused. This grammar is FROZEN — it is what every reader
  historically accepted, pinned by fixture tests
  (`grammar_freeze_test.go` in dispatch, `TestTurnStateGrammarFrozen` in
  missionrunner, `TestDecodeGrammar` in wiredoc). Narrowing it is a
  behavior change with a migration story, never a cleanup.
- **`wiredoc.Doc`** is a lossless envelope: unknown keys, unknown nested
  structure, and null-vs-absent distinctions survive every
  read-modify-write. Typed views (`dispatch.JobRecord`,
  `missionrunner.TurnRecord`) are READ LENSES over the shared map — a
  lens, never a filter. The permissive paths (RecordCAS's arbitrary
  patches) operate on the raw map via `FromRaw` and are not narrowed by
  anything typed.
- **Structs never marshal onto the wire.** `encoding/json` emits struct
  fields in declaration order; the canonical form is sorted. Writers
  render maps through the envelope.

## The two dialects — do not unify them

- **Unescaped canon** (`Doc.Render`): two-space indent, sorted keys, HTML
  left intact, trailing newline. Spoken by dispatch job records, host
  results, adapter records.
- **Escaped** (`Doc.RenderEscaped`): the `MarshalIndent` form — HTML
  escaped to `<`-style sequences. Spoken by missionrunner's turn
  and state documents.

The split is historical fact, discovered by corpus capture, and every
existing record on every machine embodies it. Unifying the dialects would
change bytes under files that are hashed, diffed, and read by the shell.
If unification is ever wanted, it is a migration with a version story.

## The rule for changing any of this

1. **Corpus first**: capture the current writer's bytes as committed
   goldens (`testdata/corpus/<family>/`, `-capture-corpus` to re-record)
   BEFORE touching the writer. The corpus is the oracle; the old code
   need not survive to arbitrate.
2. **Grammar second**: pin what the current reader accepts as fixture
   verdicts before touching the reader. A fixture the new reader rejects
   is a stop.
3. **Equivalence always**: a converted writer diffs `bytes.Equal` against
   the corpus in its package's tests, on every run, forever.

## Durability (the companion contract, owned by internal/atomicfile)

Replacement writes have two outcomes — pre-publication failure (nothing
published, plain error) and committed-with-doubt (`(false, nil)`: the
rename landed, the directory sync did not; never reported as failure).
Appends (registry, receipts) have a third — VISIBLE BUT UNCOMMITTED: bytes
may be in the file but the append failed its barrier, the caller claims no
success, and the reader-side torn-tail contract makes it survivable.
Ephemeral liveness signals (heartbeats) use `WriteVolatile` — atomic
visibility, no barriers — because durability nobody reads taxes the
hottest write path (F_FULLFSYNC on darwin destabilized timing-scaled
fixtures when it was tried).
