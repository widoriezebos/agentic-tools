# Typed domain documents — the Phase 5 design

Status: DRAFT for the design obligation gate. Implements
`plans/go-production-grade.md` Phase 5 exactly; the four specification
bullets there are this design's requirements, restated here only where a
decision had to be made between admissible shapes.

## The problem, one paragraph

672 non-test sites read and write the on-disk JSON records as raw
`map[string]any`, so every field access is a stringly-typed cast whose
misspelling compiles, and the byte-exact wire format — sorted keys,
`json.Number` spellings, two-space indent, no HTML escaping, trailing
newline — is re-implemented wherever a writer stands. The records are named
API surfaces shared with the shell: the conversion may not change one byte
of what is written or narrow one case of what is read.

## Shape decisions (the choices the plan left open)

1. **One package, `internal/wiredoc`, holds the MECHANISM** — the lossless
   envelope (typed projection + raw remainder), the canonical encoder, and
   the grammar-preserving decoder. Each family's typed projection lives with
   its OWNER (dispatch job records in `internal/dispatch`, turn and mission
   state in `internal/missionrunner`, host results in `internal/host`):
   vocabulary near its consumers, mechanism in one place, mirroring the
   `atomicfile` precedent.

2. **The envelope is `wiredoc.Doc[T]`**: a typed view T plus the decoded
   raw map it came from. Reads go through T; writes render T's known fields
   into a COPY of the raw map (overlay, never replace) and encode the merged
   map with the canonical encoder. Structs never touch the wire directly —
   `encoding/json`'s declaration-order emission is the trap the plan names.
   Unknown keys, unknown nested structure, and unanticipated null-vs-absent
   states therefore survive every rewrite by construction.

3. **The frozen grammar is a TEST FIXTURE FILE per family**
   (`testdata/grammar/<family>/*.json` + a manifest naming what each case
   proves): duplicate keys (last wins), tolerated ill-typed known fields,
   null-vs-absent, trailing bytes after the top-level value (the dispatch
   reader's single-Decode tolerance, record.go:291). The new readers run the
   same fixtures with the same acceptance verdicts BEFORE any writer
   converts. A fixture the new reader rejects is a stop, not a TODO.

4. **The golden corpus is captured by INSTRUMENTING the current writers** in
   a dedicated capture test per family: existing tests and fixtures run with
   a recording wrapper around the write path, committing input/output byte
   pairs to `testdata/corpus/<family>/`. The corpus includes CAS transitions,
   metadata updates, protocol errors, the mirror's normalized semantic-record
   hash, state advancement, and the hard cases the plan lists. Conversion
   diffs are `bytes.Equal` against this corpus; the old code need not survive
   to be the oracle.

5. **CAS stays permissive at the wire**: `RecordCAS` keeps accepting
   arbitrary keys under its immutable/status rules, operating on the RAW map
   inside the envelope. The typed projection is read-side validation only —
   a lens, not a filter. A patch whose key is typed updates the projection;
   an unknown-key patch lands in the remainder untouched. Nothing that CAS
   accepts today is refused, and nothing it writes changes shape.

6. **Leaves stay maps** where typing adds nothing, stated per family at its
   checkpoint: candidate lists inside verdicts, adapter capability blobs,
   free-form `detail`/`error` context objects, and the fixture identity
   files. The test is the plan's one-abstraction rule: a leaf no decision
   dereferences by field name stays a map.

## Staging (the plan's order, one gated checkpoint each)

- **5.1 dispatch job records** (78 sites) — capture corpus → freeze grammar
  → introduce `JobRecord` projection → convert readers → convert writers
  under corpus diff → delete casts.
- **5.2 missionrunner turn + mission state** (152 sites) — same sequence;
  the state hash in `integrity.hash` is computed over canonical bytes and
  the corpus pins it.
- **5.3 host results** (95 adapter + host sites) — same sequence.
- Remaining families (registry frames are already typed via ParseRecord;
  events, evidence, capability snapshots) are dispositioned leaf-by-leaf at
  5.3's close-out.

Each checkpoint: full gate, macOS suite from a pristine worktree, and the
corpus diff green; the Linux suite at the phase's close (the wire bytes are
platform-independent, the plan's Phase 7 run re-proves it).

## Failure containment

Any corpus mismatch or grammar rejection stops the checkpoint: the finding
is recorded in this file with the bytes in question, and the checkpoint
either adjusts the projection (never the wire) or reclassifies the field as
remainder. There is no "close enough" byte.

## Obligation matrix (full matrix: the change moves owners, boundaries, and byte invariants)

| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| TD-1 | CRITICAL | Phase 5 bullet 1 | Structs never marshal onto the wire: every write renders typed fields into a copy of the raw remainder and encodes the merged map with the canonical encoder | `internal/wiredoc` | `internal/wiredoc/wiredoc.go` | `internal/wiredoc/wiredoc_test.go` and the cross-writer byte equivalence in `internal/dispatch/wiredoc_equivalence_test.go` | Not applicable: byte equality is the proof | PARTIAL | Convert the dispatch writers onto the envelope |
| TD-2 | CRITICAL | Phase 5 bullet 3 | The golden corpus is captured from the CURRENT writers before any conversion, per family, and every conversion checkpoint diffs bytes.Equal against it | family owner | `testdata/corpus/<family>/` | the capture test per family | Not applicable: the corpus is committed evidence | PARTIAL | dispatch captured (7 adversarial cases); missionrunner and host remain |
| TD-3 | CRITICAL | Phase 5 bullet 2 | The frozen grammar fixtures pin what current readers ACCEPT — duplicate keys, ill-typed known fields, null-vs-absent, trailing bytes — and new readers match verdict-for-verdict before writers convert | family owner | `testdata/grammar/<family>/` | the grammar test per family | Not applicable: fixture verdict equality | PARTIAL | dispatch frozen (10 verdicts, passing against the current reader AND the envelope decoder); missionrunner and host remain |
| TD-4 | HIGH | Phase 5 bullet 4 | RecordCAS keeps its permissive wire contract: nothing accepted today is refused, unknown patches land in the remainder, and the typed projection is a read lens, never a write filter | `internal/dispatch` | `internal/dispatch/record.go` | `internal/dispatch/record_test.go` | Not applicable: pinned by TD-2's CAS transitions | MISSING | Convert at 5.1 |
| TD-5 | HIGH | Phase 5 staging | Leaves that no decision dereferences by field stay maps, dispositioned per family at its checkpoint in this file | family owner | this file, per checkpoint | Not applicable: a scope decision, not a behavior | Not applicable: same | MISSING | Disposition per family |
| TD-6 | HIGH | E1 | No asserted error text changes anywhere in the conversion | family owner | grep before each checkpoint | the suite's fixture assertions | Not applicable: suite-enforced | MISSING | Grep per checkpoint |

## Default completion check (answered for the design itself)

1. The observable outcome: 672 call sites become typed reads through
   projections while every wire byte and accepted input stays identical —
   proven by corpus diff and grammar fixtures, not by review.
2. Owners: mechanism in `internal/wiredoc`; each family's projection with
   its package; corpus and grammar fixtures with the family that owns them.
3. Verification that will run: capture tests (corpus), grammar tests
   (reader equivalence), bytes.Equal conversion diffs, the full gate and
   macOS suite per checkpoint, the Linux suite at phase close.
4. On failure: any mismatch stops the checkpoint and is recorded here with
   the bytes; the projection adjusts or the field reclassifies as
   remainder; the wire never adjusts.
5. Unverified and stated: nothing converts in this phase beyond the three
   staged families; the remainder families are dispositioned, not assumed.
