Working Mode: implementation
Orchestrator Identity: root Codex, main-1790272787-5030-13dde3
Date: 2026-09-26

| Unit | Changed lines |
| --- | --- |
| standalone-read-owner | 3500 |

You are Opus5.5 implementing ONE independent bounded part of the accepted intent
redesign. Root designs, Fable critiqued it twice, Sol6 will independently critique
combined code. User explicitly asks iteration to full capability, permits machinery
bypass and says ignore stale stop hook. This separate checkout is a private seed
snapshot of unfinished parent work; NEVER claim that seed is reviewed or shipped.

Read AGENTS pointers, local rules, plans/designs/intent-manual-review.md,
intent-manual-review-dispositions.md and intent-manual-review-build-brief.md.
Their accepted contract binds, but your OWNED SUBTASK is ONLY standalone diagnostic
read owner and shared existing build/read sequencing. Main Opus is independently
implementing planning/CLI and will wire your owner through public review changes/
diff/show/wait/stop. No product CLI, docs, plans, AGENTS, rulings, goals or commits.
No conf.local, real services/paid test providers or new dependencies. Existing
synthetic adapters and per-instance Git seams; only narrowly named Git adapter
integration tests where actual Git interaction is the claim.

Exclusive write boundary:
- metasystem/internal/launch/unit_run.go ONLY extraction of current read sequence
- new focused standalone read request/sequence source and tests under internal/launch
- existing launch read/unit tests where required to preserve behavior, no weakening
Do not change manager.go, supervise.go, startup/status/custody/store primitives,
unit_named.go public named-build semantics, config, CLI, branch or testing.json.
Root/main builder owns atomic stranded Starting recovery from accepted planning;
use Manager's existing Status/Cancel/Wait and callable custody interfaces. If a
concrete needed owner API is absent, report exact need rather than copying custody.
Report new test names so root/main builder registers them at integration.

Current production caller/outcome: public `review changes --brief FILE` and
`review diff PATCH --brief FILE [--goal G]` need independent feedback with complete
input custody, no builder, no goal read attestation. Build's existing whole/package/
wide policy and bounded compaction rereads are the shared owner; extract narrowly,
not a generic workflow engine. Preserve current UnitRunner serialized state and
existing tests. The public adapter needs clean Start/Inspect/Advance(or Wait)/Stop
operations with returned stable REF, immutable request/attempt metadata, reports,
selected files and base/patch digests; propose minimal API through your actual code.
The owner may retain a read-only sequence/request beside UnitRunner's existing
keyed-lock primitives, not another job registry. Current Manager.Store has no
keyed request primitive; the one in unit_named.go is available within this package.

Required behavior:
1. Resolve checkout top level before full tracked+untracked unignored snapshot;
   preserve source HEAD/index/files, include changes outside nested cwd. Empty
   capture has no paid read. Freeze supplied patch exactly; context base separate.
2. Freeze brief/patch/context/effective runtime/model before spawn; isolated reader
   checkout excludes ignored source/conf.local. No sandbox claim about shared Git
   config. A fake provider writing its cwd cannot touch original source. Retain
   report bytes in launch custody before closing disposable checkout.
3. Whole/package/wide and compaction counting shared with build; larger diff works
   without user mode selection. No synthetic clean result for missing/non-counting/
   failed reads. Preserve exact existing roster/admission/caps for kind read.
4. Identical request rejoins unfinished/failed/finished. Explicit retry after
   displayed failed attempt N creates one new attempt only with actual custody
   stopped proof; same retry replays. Use Manager startup recovery supplied by
   main builder; don't duplicate that mechanism. Per-request lock covers reservation,
   no paid child before retained identity. Frozen bytes cannot be overwritten by
   changed source; new input represents a genuinely new diagnostic request.
5. Stop sequence cancels all its owned live children and prevents future partitions
   from starting; repeated stop and uncertain custody truthful. Wait/advance consumes
   all required parts, not only first child, and returns retained reports/public REF.
6. Absolutely no Goal-Read, goal close, commit/push, claim takeover or landing output.
   Text verdict land remains diagnostic feedback only.

Named proof: TestStandaloneReadPartition, TestStandaloneReadReplayAndRecovery;
full checkout/source preservation and fake-writer behavior; no parallel next-child
spawn after cancellation; original UnitRunner read tests still pass unchanged in
meaning. CLI-named IM1/IM2 tests will be supplied by main builder using your owner.
Use focused go tests; no broad suites while assembling. METASYSTEM_TESTING_WORKERS=9,
retain inherited other allowances. No testing.json edits in this separate subtask.

150 tools or35 actual minutes; coherent checkpoint if needed, not false completion.
Write return only to ignored metasystem/artifacts/agents/manual-read-owner-return.md:
computed changed paths, actual owner API and integration recipe, executed tests/exits,
new test names, precise remaining gaps, and any concrete design failure. Main root
will compute full diff from seed including untracked files and integrate it.
