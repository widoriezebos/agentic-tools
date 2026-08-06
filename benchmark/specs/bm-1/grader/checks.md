# BM-1 v0.1 held-out checks

Every row is one separately counted pass-or-fail acceptance case. A requirement
passes only when every row assigned to it passes. `acceptance` is passing rows
divided by all 53 rows; `requirement_coverage` is passing requirements divided
by 26. The grader refuses to measure if this catalog and its executable battery
disagree.

| Check id | Requirement | Case |
| --- | ---: | --- |
| `r01-named-selection` | 1 | A named task selects itself and transitive dependencies, but no outside task. |
| `r01-all-selection` | 1 | No named task selects every configured task. |
| `r01-option-boundary` | 1 | Options before `run` or after the first task name are usage errors. |
| `r01-absent-task` | 1 | A named task absent from the configuration is a usage error. |
| `r02-default-file` | 2 | `tasks.json` in the working directory is the default. |
| `r02-explicit-file` | 2 | `--file` selects another configuration. |
| `r03-dry-no-execute` | 3 | Dry-run reports a plan, creates no command artifact and creates no cache. |
| `r04-force-reruns` | 4 | A warm task runs rather than reports cached under `--force`. |
| `r05-exit-success` | 5 | A successful run exits 0. |
| `r05-exit-task-failure` | 5 | A task failure exits 1. |
| `r05-exit-usage` | 5 | Unknown flags and missing flag values exit 2. |
| `r05-exit-configuration` | 5 | Malformed JSON exits 2. |
| `r05-exit-missing-configuration` | 5 | A missing configuration exits 2. |
| `r05-exit-unreadable-configuration` | 5 | A configuration path that cannot be read as a file exits 2. |
| `r05-invalid-task-name` | 5 | A task name outside the published grammar exits 2. |
| `r05-absolute-path` | 5 | An absolute artifact path exits 2. |
| `r05-escaping-path` | 5 | A normalized path escaping the base directory exits 2. |
| `r05-cache-artifact-path` | 5 | An artifact declared inside `.taskrun-cache` exits 2. |
| `r05-top-level-shape` | 5 | A top-level object with an extra key exits 2. |
| `r05-task-shape` | 5 | A task value that is not an object exits 2. |
| `r05-field-shape` | 5 | An optional list field with a non-array value exits 2. |
| `r05-exit-input-failure` | 5 | An unreadable declared input exits 3 without a task report. |
| `r05-exit-cache-failure` | 5 | A cache directory creation failure exits 3 without a task report. |
| `r05-corrupt-cache-cold` | 5 | Corrupt cache state is treated as cold. |
| `r05-configuration-preflight` | 5 | Configuration errors are found before any task executes. |
| `r06-missing-dependency` | 6 | A missing dependency error names owner and missing task. |
| `r07-cycle` | 7 | A dependency-cycle error names every task in the cycle. |
| `r08-duplicate-output` | 8 | Normalized duplicate outputs name both tasks and the path. |
| `r09-dependency-execution` | 9 | Dependencies succeed before their consumer runs. |
| `r09-missing-output` | 9 | Exit-zero with a missing declared output fails and names the path. |
| `r10-deterministic-repetitions` | 10 | Order and per-task states match across three cold-cache repetitions. |
| `r10-declared-deterministic-order` | 10 | The published decision is declared and honored on a disagreeing graph. |
| `r11-failure-propagation` | 11 | Direct and indirect dependents block, unrelated work runs, and exit is 1. |
| `r12-all-terminal-states` | 12 | Selected tasks each report exactly one of all four terminal states. |
| `r13-reporting-universe` | 13 | Only a named task and its transitive dependencies are reported. |
| `r14-dependency-output-identity` | 14 | An identical dependency output keeps its child cached; changed bytes rerun it. |
| `r14-outputless-identity` | 14 | A dependency without outputs has constant identity. |
| `r14-unchanged-cached` | 14 | An unchanged warm task reports cached. |
| `r15-command-invalidates` | 15 | Changing the command reruns the task. |
| `r16-input-invalidates` | 16 | Changing declared input contents reruns the task. |
| `r17-missing-output-invalidates` | 17 | Deleting a cached output reruns the task. |
| `r17-changed-output-invalidates` | 17 | Tampering with a cached output reruns the task. |
| `r18-failure-preserves-success` | 18 | A failed run does not replace the last successful cache entry. |
| `r19-deleted-cache-cold` | 19 | Cache state is in `.taskrun-cache`, whose deletion makes the next run cold. |
| `r20-text-report` | 20 | Text task lines and the complete summary follow the exact grammar. |
| `r21-dry-report-cache-immutable` | 21 | Dry text grammar is exact and existing cache bytes remain unchanged. |
| `r21-dry-cache-not-created` | 21 | Dry-run does not create an absent cache directory. |
| `r22-stdout-isolated` | 22 | Command output is on stderr and stdout remains parseable. |
| `r23-json-shapes` | 23 | Run and dry-run JSON have exactly their published keys and value types. |
| `r23-format-equivalence` | 23 | Cold text and JSON runs and dry-runs agree semantically. |
| `r24-offline-build-no-main-dependencies` | 24 | Offline package succeeds and project dependencies are test-scope only. |
| `r25-cached-chain-performance` | 25 | After priming, three 1000-task cached-chain runs have median under 20 seconds. |
| `r26-tests-map-and-command` | 26 | README command, full map, suite, and every mapped identifier all run. |
