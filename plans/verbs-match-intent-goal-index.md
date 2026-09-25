# Verb-related goals, 25 September 2026

Canonical ledger inventory read with `goal list --root metasystem --json --done --fetch`; the design
requirement map decides what this release absorbs. Done means the ledger
state, not an independent claim that every historical requirement is still
satisfied by current code. Broad substring hits such as “verbatim” are excluded.

| Goal | Ledger state | Recorded intent (abridged) |
| --- | --- | --- |
| verbs-match-intent | queued; implementation authorized in this conversation | Full intuitive, powerful and forgiving command redesign around human/agent intent, with complete workflows and existing transaction owners. Live intent and next step refreshed 25 September. Complete implementation is reviewed, verified and integrated locally. All final correction checks passed; six baseline coverage-policy failures remain disclosed. The goal stays formally queued; no approval or conclusion was fabricated. See the design and verification record. |
| critique-launch-verb-enforces-the-round-cap | queued | The critique launch verb enforces the two-round cap |
| hp-enrollment-before-migration | queued | A checkout can enroll a terminal before its backlog is migrated, and the fleet-cutoff publication waits until the backlog exists, so migrate can take the enrolled grade and the bootstrap needs no exception |
| hp-resume-takes-its-budget-from-the-ledger | queued | goal resume reads the standing approved budget from the ledger instead of demanding the five-member tuple retyped on the command line; a human who wants a different budget uses set-budget, and a mismatch refusal prints the standing values |
| land-verb-pruning | queued | The land verb and the script surface pruning: landing internalizes into the engine per Ruling N, and the disposition ledger's remaining verdicts execute (7 INTERNALIZE, 8 THIN-SHIM, remaining deletes) - program L15 |
| land-verb-writes-the-receipt | queued | The land verb writes the receipt |
| repo-flag-resolves-one-root | queued | The --repo flag means the installation for health and the steward verbs but the git checkout for up and the goal family |
| up-is-the-human-start-command | queued | A human runs metasystem up from the repository root to authorize and bring up the MetaSystem, including refreshing an enrolled rebuilt engine, with automatic context detection and no process-identity flags; the current agent-only startup operation has a dis... |
| watch-verb | queued | The watch verb: one command shows everything running with liveness verdicts and ACTS on stalls within Ruling L (heal-first escalation) - folds never-idle enforcement and Ruling M (delegated work is watched); the steward seam's operator surface (program L14) |
| codex-jobs-run-through-a-metasystem-verb | done | Codex companion brokers leak: the openai-codex plugin 1.0.6 keeps one detached broker per working directory, shuts it only through its SessionEnd hook for the session's own cwd, and runs each job in a detached worker with no completion callback, so every wo... |
| delegate-launchers-become-go-verbs | done | Wido 2026-09-16 18:25 CEST: "Land bash now, immediately after (i.e |
| every-verb-resolves-repo-to-an-absolute-root | done | A relative --repo must not change what a verb reports |
| goal-done-flags-do-not-match-the-verb-table | done | The goal verb table and the goal verbs disagree about how a goal concludes, and the mismatch costs a caller several refused attempts before it can record anything |
| human-carried-landing-verb | done | DUPLICATE, opened by m1b on 2026-09-10 before finding the sibling goal human-carried-landing-carry (approved 2026-09-04 on Wido's ruling as the home of the carry verb, the carried landing and the commit critic, points 02 and 04-09 of plans/human-carried-lan... |
| human-goal-verbs-forgiving | done | Wido's word 2026-09-06, after three refused attempts to raise one goal's budget at his terminal: 'this command is too complex, not intuitive and maybe not forgiving enough' |
| ledger-attention-clears-after-journaled-verbs | abandoned | On m1b the steward's ledger-attention health stays dead for over 90 minutes, reporting 'the shared ledger moved to <tip> .. |
| metasystem-stop-verb | done | Wido's word 2026-09-06: 'For a human it should be a simple command to stop the meta system |
| repair-accept-remote-verb | done | The goal machinery's own refusal advertises a verb that does not exist: a non-descending canonical tip makes goal fetch say 'repair --accept-remote is the deliberate path', but no CLI verb wires internal/goal.RepairAcceptRemote (implemented, tested, journal... |
| verb-ergonomics | done | The goal-verb ergonomics from the ease review: goal show, pretty list, lineage refusal on mutations, park/unpark --arc cascades, declare-free wiring, root+world in read answers |
| wait-verb-returns-on-recorded-events | done | Member 1 of coordinator-wakes-on-events-not-polls (its design page plans/coordinator-wakes-on-events-not-polls-design.md, sections 2, 5 and 6) |
| role-names-say-what-the-role-does | done | Role names describe where a role runs instead of what it does, and the same name means two different jobs |
| goal-landing-needs-a-human-word | queued | A land-ready goal lands on main only after a human approves that landing, naming the branch commit the word covers; today that step does not exist |
| human-carried-landing | parked | A verified human may fast-track a landing under close scrutiny and the machine never refuses that human |

The umbrella is **verbs-match-intent**. Read the current complete design in
`plans/designs/verbs-match-intent.md`. `watch-verb` includes new autonomous
intervention policy beyond this command-boundary redesign; do not accidentally
claim that policy work done by exposing the existing read surface.
`land-verb-writes-the-receipt` is a distinct atomic receipt requirement: the
public land workflow must call the current receipt owner and preserve that
sibling if its stronger atomic guarantee still requires owner work.
