# Sol's read of the g1-s62 build

Produced 2026-09-27 by Codex on `gpt-6-sol`, read-only, against `git diff ui-development...ui/g1-s62` at `5f774dcd9` (six commits, rebased twice: onto `86bbe90b5` after g1-s63 and onto `a7943a658` after g1-s64, whose tenth catalogue row it folded into the public-name shape) and the design's revision 1 with its dispositions, under R-124: one fix round, then landing. Verbatim; there was nothing material, so the slice landed as built, with the one-word count correction the read deferred folded at the landing (the design's Built section).

---

## Findings

No material findings. The implementation meets D1–D5 under the brief's WORK and SAFE test.

- **D1:** `metasystem/internal/ui/uitools/propose.go:496` accepts public goal actions and flag spellings, refuses unsupported inputs by name, parses `risk` with `goal.ParseRiskRecord`, and persists route ids. `metasystem/internal/ui/partner/proposals.go:210` composes edit labels with `goal.ApplyLabelDelta` at admission. The reported catalogue, refusal, bounds, risk, label, and usage-line tests hold the behaviors they name.
- **D2:** `metasystem/cmd/metasystem/ui_propose_test.go:76` reads the public descriptors and checks object, action, audience, hidden marks, and each admitted flag's own spelling. Its completeness test covers public goal actions; the existing act-table join (`metasystem/internal/ui/httpd/proposals_test.go`) covers routes and body fields from both sides. These tests hold their stated joins.
- **D3–D4:** The word map (`metasystem/internal/ui/web/_app/src/partner/proposing.ts:230`), renamed controls, help terms, and inspected screenshots use the new words, including Abandon. Persisted route ids still resolve through the map. The word-map and vocabulary-drift tests assert the new values. Both Partner skill copies match; the deposit refusal names `propose`.
- **D5 and the g1-s64 fold:** The changed-source diff stays within the slice. Abandon retains its reason/successor mapping, named `waive`/`also` refusals, dependent read, card arguments, and stale-state guard. The transitional catalogue identifiers have no remaining references. The reported abandon tests retain those assertions.

## Departures adjudicated

- **P1 #1–2 — Accepted:** `Button`→`Word` and separate public `Action`/stored `Route` express the two names D1 and D3 require.
- **P1 #3–4 — Accepted:** Label deltas travel to admission for composition; making `propose` a `Readers` method obtains usage lines from the command catalogue.
- **P1 #5–6 — Accepted:** Exported refusal lists let the descriptor join read the same lists; `successor` became an admitted Abandon flag in the final fold.
- **P1 #7 — Accepted:** Naming every label under `unlabel` clears the list through the public delta grammar; the admission test checks the resulting empty list.
- **P1 #8–10 — Accepted:** Updated comments and walkthrough input use the public words while the frame retains its route id; "at least one of them" describes the field requirement without duplicating its list.
- **First rebase — Accepted:** The `edgesNamedBy` conflict keeps its behavior and shares the same name parsing.
- **Second rebase — Accepted:** The ten-row catalogue, `Body()` join, Go and frontend word maps, admission tests, abandon read and guard, and rebuilt bundle preserve the reported g1-s64 substance.
- **`Open goal`→`Open` — Accepted:** The proposal word is Open; the new-goal button and navigation menu retain "Open goal."

## Deferred and non-material

- Both Partner skill copies still say **"nine"** although the tool offers ten actions. The tool names all ten, including Abandon, so this does not prevent the slice from working or staying safe. Change that word and its test assertion in a later tidy-up.
- The bulk title says "Pause *n* goals" where D3 specified "Pause *n* selected." The count and action remain clear; no WORK or SAFE failure follows.
- I inspected representative desktop and mobile screenshots, ran the specified source-word search and `git diff --check`, and confirmed the worktree is clean. The builder reports the final Go, frontend, gate, and bundle checks passing. My focused Go rerun could not start because the read-only sandbox denied creation of Go's temporary build directory.

VERDICT: 0 material findings; land as built
