# Current verb owners, 25 September 2026

Source read at `410a71c91`; root checked the material source excerpts after
two independent read-only inventories. These are current owners, not new
design decisions. Historical failures are in
`plans/verbs-match-intent-evidence-20260925.md`.

| Fact | Source |
| --- | --- |
| Root help expands almost every engine family; the public registration has name, summary, handler only | `metasystem/cmd/metasystem/main.go:20`, `:887` |
| Goal shared parser admits flags into help before checking applicability | `metasystem/cmd/metasystem/goalsync_mutations.go:1090` |
| `done` help names legacy `then`/`and-none`; synced execution needs id/conclude | `metasystem/cmd/metasystem/main.go:586`, `goalsync_mutations.go:1932` |
| Existing complete layout resolution handles repository descendants and installation variants | `metasystem/internal/stateroot/stateroot.go:158` |
| Existing `pathFlag` only absolutizes | `metasystem/cmd/metasystem/helpers.go:35` |
| Budget owns state-sensitive approve/update/stopped resume and norm/keep parsing | `metasystem/cmd/metasystem/goalsync_mutations.go:1532` |
| Actor name defaults only after matching proof of the enrolled human | `metasystem/cmd/metasystem/goalsync_mutations.go:1485` |
| Agent lineage is still supplied explicitly or by environment; human lineage derives from proof | `metasystem/cmd/metasystem/goalsync_mutations.go:666` |
| Direct resume still requires the complete tuple; authenticated channel resume has a distinct validation path | `metasystem/cmd/metasystem/goalsync_mutations.go:3176` |
| Claim epoch preservation is already implemented | `metasystem/internal/goal/verbs.go:1673` |
| Human process startup uses arm; ordinary up expects agent ancestry | `metasystem/cmd/metasystem/process_verbs.go:184`, `metasystem/internal/up/up.go:215` |
| Stop checkout, stop one session and cancel one job are distinct effects | `metasystem/cmd/metasystem/process_verbs.go:110`, `session_stop.go:39`, `delegate.go:27` |
| Mission answer is an actual transition; channel answers use authenticated polling | `metasystem/internal/missionrunner/answer.go:17`, `metasystem/internal/channel/poll.go:347` |
| Risk decision owns ledger, counselor/register update and proof; those can partially succeed | `metasystem/cmd/metasystem/goalsync_mutations.go:1252` |
| Current request identity already supports exact dispatch retry | `metasystem/internal/dispatch/operation.go:10` |
| Build/proof/read is one existing resumable unit owner, ending at judgement | `metasystem/internal/launch/unit_run.go:104`, `:330`, `:696` |
| Unit input is already a JSON plan; callers currently have to assemble it | `metasystem/internal/launch/unit_plan.go:14`, `:108` |
| Standalone launches retain process results, not certified-chain evidence | `metasystem/internal/launch/record.go:65` |
| Branch review freezes the subject and preserves uncertain dispatch | `metasystem/internal/goal/branch/read.go:330` |
| Batch join already owns transport preparation, proof admission, handover and owner wake | `metasystem/cmd/metasystem/landing_batch_join.go:128` |
| Hand branch landing separately owns prep and push and requires exact test evidence | `metasystem/cmd/metasystem/goal_branch.go:297`, `:523` |
| Full chain close mirrors terminal jobs, advances/closes review, verifies and closes | `metasystem/scripts/agents/dispatch.sh:2975` |
| Register advancement is already idempotent | `metasystem/internal/dispatch/finding_register.go:69` |
| CLI/Partner catalogue currently reads the engine family registry | `metasystem/cmd/metasystem/ui_describe.go:86` |

Observed in this conversation: `goal list --root . --json --done` reported
the legacy world; selecting `--root metasystem` fetched the actual synced
ledger. `goal edit` initially refused missing lineage; repeating the same
intended edit with the current stop report's proven session lineage succeeded
at goal tip `b7add43fef7dd896a854b957225ad1b234bbb93f`. The action changed the
goal's intent and next step, not its authority record. Current conversation
authorization is recorded honestly rather than fabricating terminal proof.
