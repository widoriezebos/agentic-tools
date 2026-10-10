# Brief: review-chain-stops-and-records, unit read-publication

Working Mode: Implement
Goal 2. build-outcomes, tree-boundaries and round-result (all parts and fixes) are committed on this branch. Spec, and ONLY this text: plans/designs/review-chain-stops-and-records.md Decision 5 "Publish and carry the same read on both routes" (all four paragraphs) and the read-publication rows of the Units and public-verb tables, plus the one item moved here from tree-boundaries (design page 03:50): manual and commit-form read publication goes through the tree reservation like the unit form (tree/unit/run order, fresh comparison under the tree lock, locks released during the push).

Build: the canonical read in both publication bundles, carried by the existing carry owner with carriedFrom (new identity and subject; original source and model/engine provenance kept; no new model examination, no extra round); changed work is unread, lost or corrupt predecessor evidence is unknown, never relabelled clean; range handling per read (on the branch; replayed equivalently and re-keyed; landed under a proven equivalent commit and omitted from the active range with evidence retained; changed and needing a read), never matched by filename or unit label alone; `work rebase` resolves current remote main before ancestry and reports the actual behind count when it does nothing; `work commit G --work U` exposes the existing commit owner for hand changes with the unit trailer and the same subject and gate path; a person's `goal done ... --reason` prints and records the impact of closing the remaining review chains before it takes effect, then closes them after the goal act succeeds (never as clean reads), idempotent and discoverable if interrupted; an agent's conclusion cannot bypass transferred work.
Size: at most 250 production lines. If it will not fit, build the largest usable first part within 250 (carry with carriedFrom and range handling first) and report the rest.
Public-verb test (from the design): correct an earlier unit through `work review`; a unit-read and a by-commit critic read on unchanged later units get new subjects with carriedFrom, unchanged source evidence and no new model call; a landed equivalent leaves the active range; changed work requests its own read; `work commit G --work U` records the trailer; a person's `goal done` prints and records its impact before conclusion and chain closure. Mutations: relabel changed work clean; match by filename; close chains as clean reads.


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (Wido 10-07: "be Scrooge where it comes to testing. ONLY WHEN ABSOLUTELY NEEDED")
`go build ./...` and `go vet` on the packages you changed; then ONLY the tests you added or changed, by name (`-run '^(TestA|TestB)$'`), and each one's mutation (break the code, see it red, restore). No package-wide runs, no broad selections, no whole suite: the full suite runs once, when the goal lands. If you change a message or skill text, also run `-run 'TestAudit|TestInstruction'` once. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
