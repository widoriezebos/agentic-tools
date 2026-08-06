# Evidence index

One line per delegate chain, newest first: what it was for and where its full
evidence lives. The durable store sits outside the repository on purpose (a
day of transcripts weighs hundreds of megabytes); `evidence/` at the
repository root is a gitignored symlink into it for browsing, and this index
is the tracked memory of what exists. Maintained when evidence-gc collects.

- `investigator-20260804t123608z-ac09` — Triage every retained watch-list finding in `plans/agent-orchestration-watchlist.md` so implementation can resume. The pile is currently one undiffere
- `implementer-20260805t221100z-c169` — Build `scripts/agents/hosts/codex.sh`, the mission host adapter for the codex runtime, so a mission's coordinator turns can run on a GPT-5 model. Toda
- `implementer-20260805t211546z-677b` — Build `benchmark/provision.sh`, which turns a benchmark spec into a repository a mission can actually start in. Mission Zero's first real preflight fo
- `implementer-20260805t211542z-5b5b` — Build the judged layer of the benchmark: the rubrics the kit owns, and the `behavior-judge` role that reads them. The mechanical extractor is a separa
- `implementer-20260805t211220z-bc7e` — Build the scorecard extractor: `benchmark/extract.sh <run-evidence-root> --spec <spec-dir> --out <scorecard.json>`. It turns one benchmark run's evide
- `implementer-20260805t210929z-2639` — Build the scorecard extractor: `benchmark/extract.sh <run-evidence-root> --spec <spec-dir> --out <scorecard.json>`. It turns one benchmark run's evide
- `implementer-20260805t210214z-8bc7` — Build `scripts/agents/hosts/codex.sh`, the mission host adapter for the codex runtime, so a mission's coordinator turns can run on a GPT-5 model. Toda
- `implementer-20260805t205155z-aaaa` — Build `benchmark/provision.sh`, which turns a benchmark spec into a repository a mission can actually start in. Mission Zero's first real preflight fo
- `implementer-20260805t205059z-263e` — Make delegate commits attributable to the delegate that authored them. Today Mission Zero's host fixed a test and the commit was authored with the mac
- `implementer-20260805t205056z-9a64` — Build `benchmark/provision.sh`, which turns a benchmark spec into a repository a mission can actually start in. Mission Zero's first real preflight fo
- `implementer-20260805t194101z-a102` — Build the v0.1 grading baseline for benchmark case BM-1, and finish its seed. The design closed under critique today (five rounds, 41 material finding
- `implementer-20260805t063758z-3648` — Finish item 20b, the minimal mission runner. The implementation already exists and is preserved on a branch; a previous job wrote it but hit its time 
- `implementer-20260804t205954z-5650` — Build the minimal mission runner: the component that drives an unattended mission forward without a human providing continuation. This is the harness'
- `implementer-20260804t131716z-231f` — Ship the orchestrator's own prompt artifacts, section 6.2a of `plans/agent-orchestration-design.md`. These are the shipped instructions a host turn re
- `design-critic-20260805t120744z-4a60` — Adversarial critique of `benchmark/specs/bm-1/spec.md` and `benchmark/specs/bm-1/manifest.json`, the first benchmark case. Findings only.
- `design-critic-20260805t093318z-b195` — Adversarial design critique of `plans/benchmark-spec-bm1-design.md`, the first benchmark spec for this harness: a task runner the harness builds from 
- `design-critic-20260805t092455z-404b` — Adversarial design critique of `plans/benchmark-spec-bm1-design.md`, the first benchmark spec for this harness. Findings only.
- `design-critic-20260804t102631z-67c9` — Adversarial design critique of `plans/harness-benchmark-design.md`, the harness benchmark design: a coordinator and sub-agents build software to a fix
