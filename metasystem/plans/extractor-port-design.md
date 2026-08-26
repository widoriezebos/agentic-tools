# The kit's ruler moves into the engine (os-dependency-reduction slice two)

Working Mode: design

STATUS: PARKED 2026-08-26 after the single critique round
(design-critic-20260826t040859z-a81a, NINE material incl. the
critical EPC-01: the python-removal premise is false for an
extractor-only port — python3 pervades the kit: pairs.py,
compare.py, system-fingerprint.py, provision and staging blocks —
and the true cost is a multi-slice arc with Go/Python JSON
byte-equality and a general schema validator underneath). The
decision (port the whole kit vs declare python3 a kit-scoped
dependency) is Wido's, drafted for his return; the ratchet slice
proceeds either way and pins whichever ruling lands.

Owner: m2 coordinator brief. Appetite 3h within the claimed item.
Critique loop: ONE design round (the port is a translation with an
oracle, not an invention — the calibrated light shape), failsafe at
round 2. Codex builds; fresh certification; fast tests.

## Intent

benchmark/extractor.py (~1300 lines) and its schema-validation
helpers become a Go engine verb family (`metasystem benchmark
extract ...`), removing python3 from the kit's dependency list.
extract.sh keeps its CLI surface and becomes a thin exec of the
engine verb — no caller changes anywhere (run-cohort.sh, fixtures,
the VM flows).

## The oracle law (the whole verification story)

The Python extractor stays in the tree during this slice and IS the
oracle: for every golden mission-evidence bed in
benchmark/extractor-fixtures.sh (the valid bed plus all thirteen
variant beds), the Go verb's scorecard must be BYTE-IDENTICAL to
the Python one (canonical JSON key order pinned to match Python's
json.dumps conventions, or both normalized through one canonical
serializer before compare — the design decision the critique
verifies). The fixture suite runs both, diffs, and refuses on any
divergence; ONLY after a full green oracle run does a follow-up
slice delete the .py files and flip extract.sh. Two-step removal:
port-with-oracle first, deletion second — the deletion is trivial
once the oracle proves equality, and the kit is never without a
working ruler.

## Shape

- internal/benchmark/ (new engine package): the extractor's gates
  (transport health), mechanicalBehaviorMetrics (delegationFloor,
  fenceEconomy, tracking...), evidence-set checklist (incl. the
  transport-aware acp transcript rule), schema validation against
  benchmark/schemas/evidence/ (the engine's existing JSON-schema
  machinery), identity/fences assembly, and the scorecard document
  writer.
- cmd verb `benchmark extract --evidence <root> --spec <ref> --out
  <json>` mirroring extract.sh's exact argument surface.
- The kit-side schema files stay where they are (the kit remains
  the ruler's data home; the engine reads them via the spec path —
  no schema relocation in this slice).
- model-equivalence.json, aliases, pairs resolution: pairs.py is
  NOT in this slice (it serves registration/resolution and is
  entangled with versions.lock law); the extract verb receives the
  RESOLVED spec exactly as extract.sh receives it today. pairs.py
  is slice 2b or rides the ratchet slice — recorded, not smuggled.

## Constraints

- Zero behavior drift: the oracle law is the constraint.
- No changes to run-cohort.sh beyond none (extract.sh's surface is
  preserved).
- The Python files are DELETED only in the follow-up commit after
  a green oracle run on this host AND the VM (both platforms have
  graded live evidence to re-extract as an extra oracle bed: the
  bm-2dc scorecards must re-derive identically).
- go test fixtures for the Go side mirror the Python fixture
  variants (table-driven over the same beds).

## Acceptance (fast tests)

1. Oracle equality across all fixture beds (valid + 13 variants),
   host-side.
2. Oracle equality on the real bm-2dc rep scorecards in the VM
   (re-extract both reps with both rulers, diff).
3. extract.sh thin-wrapper produces the identical file and exit
   codes for the usage-error paths.
4. bash -n; go vet/staticcheck; the extractor-fixtures suite green
   with the dual-run diff added.
5. After the deletion follow-up: zero python3 references in
   benchmark/ scripts (the ratchet slice then pins it).
