# Removing the relayed-word path: the finding that parked it

Goal `no-code-carries-the-relayed-word-path`, opened 2026-09-20 on Wido's
instruction: "Remove the temporary word functionality. I need clean
code." Parked the same day, on this branch, with the work below unstarted
on purpose.

## What we thought we were removing

R-82-m1b (2026-09-07) retired the relayed-word path by leaving
`TemporaryGoalAuthorityHorizon` at `2026-09-06` and not renewing it. On
the goal verbs that retirement is real: `validateTemporaryGoalAuthority`
in `internal/humanauthority/authority.go` requires
`checkedDate <= reviewDate <= horizon`, and with the horizon in the past
that window is empty, so every `--temporary-human-word` call on a goal
verb refuses. The brief for the removal said so, and told the builder to
stop and report instead of deleting if it found a caller that could still
reach the path.

## What is actually still reachable

`steward arm` does not go through the goal validator. It calls
`humanauthority.ValidateTemporaryWordPair`
(`internal/humanauthority/authority.go:286`), which checks two things:
the word is not whitespace, and `--review-by` parses as a date. It never
looks at the horizon. The horizon check lives in
`validateTemporaryGoalAuthority` at `:302`, a different function with
different callers.

Then `cmd/metasystem/steward_verbs.go:676-689` branches on the word:

    if *temporaryWord == "" {
        fixtureEnrollment, authorized = requireHumanTerminal(...)
        if !authorized { return 1 }
    } else {
        // prints a warning
    }

So `steward arm --temporary-human-word "<any three words>" --review-by
<any parseable future date>` skips `requireHumanTerminal` entirely and
goes on to `steward.ArmTemporary`
(`internal/steward/runner.go`), which also does not check the horizon.
The shorter `metasystem arm` in `cmd/metasystem/process_verbs.go` accepts
the same pair, though that route crosses the human-terminal boundary
first.

This is not leftover dead code. It is a live way to arm a steward without
the agent-free terminal, available to anything that can run the binary,
and the 2026-09-06 horizon does not close it. The retirement swept the
goal verbs and left the enrolment verb open.

The Codex build job (launch `20260920t073213-35418ad015`) found this,
stopped before editing anything, and asked. The worktree stayed clean.
The read was then verified independently against the three sites above.

## What the removal has to do when it resumes

Delete the writing and the accepting; keep the reading.

- `steward arm` keeps one path: `requireHumanTerminal`. The word branch,
  `steward.ArmTemporary` and `ValidateTemporaryWordPair` go with it.
  `steward.Arm` and `steward.ArmFixture` stay.
- The flag registrations go from `cmd/metasystem/goalsync_mutations.go`
  (`:209`, `:1057`), `cmd/metasystem/steward_verbs.go` (`:657`, `:658`)
  and `cmd/metasystem/process_verbs.go`, along with the remedy and the
  re-run site in `cmd/metasystem/goal_refusal.go` (`:30`, `:101`).
- The horizon and its ruling constant go from
  `internal/governance/types.go:103`, and the "horizon has passed"
  message from `internal/goal/file.go:302`.
- The identity carry goes: the `temporaryHumanWord` field
  (`internal/steward/identity.go:66`), the rebuild carry and the status
  suffix (`internal/steward/runner.go:344`, `:858`).
- `scripts/agents/goal-cli-fixtures.sh` stops passing the flag.
- `memory/rulings.md` marks R-32-m1 swept under R-82-m1b, so no row
  points at deleted code.

Keep the authenticated-channel authority. It is the replacement, not part
of this cut: `AuthorityOutcomeAuthenticatedChannelWord`,
`AuthorityOutcomeVerifiedChannelAnswer` and both paths in
`internal/channel/poll.go`. The power-of-attorney verbs and their
`--expires`, `--tiers` and `--verbs` flags are a different mechanism and
stay.

## The one thing that can break the ledger

Landed goal records already carry the relayed-word fields, and the record
grammar is closed: a parser that forgets a field which landed records
contain refuses the whole ledger, not just the line that carries it. That
cost two refusals on 2026-09-20 alone, `goal migrate` over a `- Risk:`
line and `goal open` over `Approved: unknown key "episode"`. So the
record readers keep accepting the fields after nothing writes them.
`internal/goal/file.go:1267-1278` shows the tolerant shape to preserve.

Identity JSON is the easier half, because `encoding/json` ignores unknown
keys, but check for `DisallowUnknownFields` before assuming that. On this
machine `artifacts/agents/steward/identity.json` carries no
`temporaryHumanWord` key, so there is nothing to migrate here.

## State

Goal parked. Branch `rwr1` holds this note; the full build brief, with
the impact rules block, is at
`hact-20260912/m1c-dm-0917/brief-relayed-word-removal.md`. Nothing was
deleted, so the reachable `steward arm` path described above is still
live on `main` today.
