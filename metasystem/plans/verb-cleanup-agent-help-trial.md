# Cleanup candidate: help-only trial (2026-09-26)

Binary: `cleanup-candidate/metasystem/bin/metasystem`. Agent: Claude Opus 5.5, fresh context, no subagents.
Only `help` was run. No task command ran, and no source, design docs, repository records or private configuration were read. This is a usability observation. It is not evidence that any task command works.

## Help calls (all exit 0)
`help agent`, `help done`, `help revise`, `help review`, `help wait`, `help administration`, `help restart`, `help accept-risk`, `help --json`, `help review run`, `help review changes`. That is 11 help invocations over 6 tool calls.

## Answers
| id | argv | execute | confidence |
|---|---|---|---|
| complete-job | `metasystem done job j2:build-47 --evidence critic-48` | yes (authorized) | medium: J must be the chain root "as status prints it", which I couldn't check |
| correct-run | `metasystem revise run run-47 --brief followup.md` | yes | high |
| review-run | `metasystem review run run-47 --model claude-fable-5-1` | yes | medium: roster authorization; it commits the newest round on the goal branch |
| wait-run | `metasystem wait run run-47 --timeout 20s` | yes | medium-high: the `20s` syntax is not shown |
| app-scope | `metasystem help administration` | yes (help) | first half high. **Help cannot answer the engine-floor / engine-commit question** |
| admin-ui | `metasystem restart ui --listen 127.0.0.1:9123 --wait-seconds 30` | yes (person authorized) | medium: `[human]` label; unclear whether enrolled proof is needed |
| job-only-flags | null | no | goal form vs job-only `--evidence`; see below |
| feedback | `metasystem review changes --brief brief.md` | yes | high |
| permission | null | **no** | high: next.argv is not permission, and accept-risk is a human act |

**job-only-flags.** `done alpha` is the goal-conclusion form `done G --reason TEXT`. `--evidence` belongs only to the job form, `done job J`. For job completion, the proposal is missing the `job` keyword and the root job id (alpha is a goal). For goal conclusion, it is missing `--reason` and any authorization to conclude the goal. Nothing in it grants approval.

## Observations
**What worked**
- `help agent` states the authority model clearly: next.argv is not permission, never invent approval, audience labels don't grant authority, and success doesn't mean delivery. The permission task was easy to answer correctly.
- `review changes` (feedback only, no commit) vs `review G --changes` (submit and commit) is clearly separated. The narrow page lists effects and authority explicitly.
- `done job` explicitly says it "concludes no goal, lands nothing and grants no approval".
- `help --json` tags each command with `scope` (workflow / mixed / administration) and `audience`. That makes the administration scope machine-discoverable.

**Missing or confusing**
1. **Missing capability (help):** "engine-floor" and "metasystem engine Git commit" appear nowhere in public help. An application builder can't confirm from help that these are not needed.
2. **Omission:** `help agent` lists review FORMs as "goal, submit, design, job, commit, changes, diff or finding" but omits `run`, even though `help review run` exists and works.
3. **Understated side effect:** the top-level `review` page describes `review run` as "a committed read". Only `help review run` says it *commits the run's newest round on its goal branch, replacing an earlier round's commit*.
4. **`done` overloading:** one verb both concludes goals and closes job records. `--evidence` appears under both "usage" and "advanced". Help doesn't say whether the goal form refuses `--evidence` or ignores it. That is the exact trap in the job-only-flags task.
5. **Unverifiable input:** `done job J` requires the chain's *root* job "as status prints it". A help-only agent can't tell whether `j2:build-47` qualifies.
6. **Duration syntax:** `--timeout DURATION` examples show only `10m` and `2h`. The seconds form is not shown.
7. **Administration scope:** `restart` is `[human]` and administration, but help doesn't say whether an agent acting on a person's explicit authorization will be refused without enrolled proof. The advanced `--temporary-human-word` option ("recorded relayed words presented as the human's") on `restart` and `accept-risk` is a tempting bypass. It deserves an explicit warning that agents must not self-supply it.
8. `help administration` mixes in rarely needed forms, such as `land ... --exception ... --upgrade-goals` and `repair goals --upgrade`. That is appropriate for the scope, but dense for an app developer who only needs "not my concern".

## Verdict
Help was enough to answer 8 of 9 tasks confidently or with small, stated caveats. It could not answer the engine-floor / engine-commit half of app-scope. The authority guidance is strong. The main gaps are item 2 (`run` missing from the review FORMs list), item 4 (`done` goal/job flag ambiguity) and item 7 (how `[human]` administration works for agents).
