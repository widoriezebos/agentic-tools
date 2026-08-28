Verdict: commit `6bd1f2b` does not close all authorization holes. Two laundering paths and the authorization-to-lock race remain.

1. **STRUCTURAL — HIGH — A crafted source can still elevate a delegate to `MAIN`.**

   `goalCaller` trusts any source classification of `MAIN` and then ignores the target’s `DELEGATE` result ([goal.go:68](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/cmd/metasystem/goal.go:68), [goal.go:76](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/cmd/metasystem/goal.go:76)). Classification checks each ancestor’s announcement before its runtime signature ([classify.go:290](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/lease/classify.go:290), [classify.go:293](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/lease/classify.go:293)).

   Therefore a caller-controlled root can copy the live main ancestor’s announcement while omitting the nearer delegate’s adapter signature. Announcements are not root-bound or secret-backed; authentication only compares process identifier, start time, and deterministic command hash ([classify.go:129](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/lease/classify.go:129), [classify.go:140](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/lease/classify.go:140)). The resulting source class is `MAIN`, which genesis admits ([authority.go:38](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/authority/authority.go:38)).

   Missing and empty source roots correctly fall back to the target for an ordinary delegate; a crafted root retaining an announcement does not.

2. **STRUCTURAL — HIGH — Adapter-supervisor laundering remains open.**

   Every non-`MAIN` source result—including a correct `ADAPTER-SUPERVISOR` result—is discarded in favor of the target classification ([goal.go:64](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/cmd/metasystem/goal.go:64), [goal.go:76](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/cmd/metasystem/goal.go:76)). Adapter-supervisor identity depends on root-local job custody records ([classify.go:221](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/lease/classify.go:221), [classify.go:301](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/lease/classify.go:301)).

   Adoption does copy all adapter scripts before reconciliation ([adopt.sh:247](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:247), [adopt.sh:289](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:289)), but the virgin target has no source job-custody records, and adapter-supervisor scripts are deliberately excluded from runtime signatures—for example Codex at [codex.sh:192](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/codex.sh:192) and [codex.sh:197](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/codex.sh:197).

   Thus an adapter-supervisor without another runtime-CLI ancestor can fall through to target `HUMAN` ([classify.go:307](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/lease/classify.go:307)), which is admitted unconditionally ([authority.go:29](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/authority/authority.go:29)). The requested “any value” property therefore fails for adapter-supervisors too.

3. **STRUCTURAL — HIGH — The authorization/state race remains open when a baseline appears.**

   Genesis versus holder-only authorization is decided before locking, using `os.Stat` ([goal.go:53](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/cmd/metasystem/goal.go:53), [goal.go:55](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/cmd/metasystem/goal.go:55)). `Reconcile` later locks and reads current state ([goalverbs.go:552](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/goal/goalverbs.go:552), [goalverbs.go:556](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/goal/goalverbs.go:556)).

   The new holder/`HasGoals` guard runs only when `state.base == nil` ([goalverbs.go:572](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/goal/goalverbs.go:572), [goalverbs.go:590](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/goal/goalverbs.go:590)). If a baseline appears while the caller waits for the lock, that branch is skipped. The caller previously admitted as non-holder genesis can then reach ledger restoration, malformed-baseline recovery, or normal replay writes without another holder check ([goalverbs.go:568](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/goal/goalverbs.go:568), [goalverbs.go:609](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/goal/goalverbs.go:609), [goalverbs.go:621](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/goal/goalverbs.go:621)).

   Ledger population alone is protected when the baseline remains absent, because parsing, `HasGoals`, and writing are inside the lock. Baseline appearance is not.

4. **MECHANICAL-GRAIN — LOW — The fixtures still do not certify the `MAIN` path, but this is recorded.**

   No test calls `goalCaller`; the added tests inject already-distilled `Caller` values directly and exercise only sequential baseline deletion ([goalverbs_test.go:471](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/goal/goalverbs_test.go:471), [goalverbs_test.go:499](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/goal/goalverbs_test.go:499)).

   The adopt fixture excludes source `artifacts/`, removes the announcements needed for `MAIN`, and asserts only successful reconciliation ([adopt-fixtures.sh:69](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt-fixtures.sh:69), [adopt-fixtures.sh:266](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt-fixtures.sh:266)). `validate-kit.sh` passes the live root, so an announced-main invocation can exercise `MAIN`, but the fixture does not assert that classification ([validate-kit.sh:261](/Users/wido/LocalStorage/GitHub/agentic-tools/benchmark/validate-kit.sh:261)). A green run can still use `HUMAN`.

   This limitation is explicitly recorded as follow-up F4/F5 rather than silently claimed as proof ([delegated-decisions.md:2460](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/docs/reviews/2026-08-13-delegated-decisions.md:2460)).

The remaining requested properties do hold by code reading:

- A genuine terminal human passes; an announced main passes when adoption supplies its authentic source root.
- The store receives the target holder bit, not the source bit ([goal.go:89](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/cmd/metasystem/goal.go:89)).
- `HasGoals` covers Current, Queued, Parked, and Done ([goal.go:86](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/goal/goal.go:86)); a parse-legal ledger without those goals must carry the Goal-free declaration.
- Sequential deleted-baseline recovery still permits the target holder, while goal-free genesis permits a non-holder main.

Evidence: source and tests read; `git diff --check` passed. Go and integration suites were not run because the provided environment is read-only and cannot create their temporary/cache/state files. No files were modified.

Proposed receipt: `type=review|outcome=shipped|skills=none|verify=skipped|corrections=0|stop_loss=no|delegate=none|note=Read-only review of 6bd1f2b found two genesis laundering paths and one authorization-to-lock race; HasGoals and no-over-refusal checks otherwise hold; fixture HUMAN-fallback limitation is recorded`

REVISE — structural findings remain
GEN3-DONE
