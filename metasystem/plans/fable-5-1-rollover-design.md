# Fable 5.1 rollover — design

Goal: `plans/goals/fable-5-1-model-rollover.md`. Wido's order, verbatim
(2026-09-02): "Fable 5.1 is released, make sure we use that model for
Fable models going forwards". New model id `claude-fable-5-1` (CLI probe
2026-09-02 returned canonicalModel `claude-fable-5-1`). File:line
citations are relative to the metasystem root, traced on this tree
(branch agent/f51-design-6 at 07e90824, 2026-09-02).

> Provenance: authored whole by the Fable design delegate (job
> f51-design-6, claude-fable-5-1, fresh context), brief
> `plans/fable-5-1-rollover-design-brief.md`.

## 0. The brief's premise is stale — read this first

The brief says `metasystem.conf:6` reads `claude-fable-5`. It does not.
Commit d081ef07 (Wido, m1, 2026-09-02 09:15 +0200, "The Fable lane moves
to five point one", landed directly under R-39-m2) already changed line 6
to `runtime.claude.maximal-models=claude-fable-5-1` and DROPPED the old
id. Its message states that the machine-local rosters on all four
machines follow the same key. m0b's own `metasystem.conf.local` line 37
carries the identical single-id override (not the dual value the goal
record describes). Every decision below is made against that tree, not
against the brief's picture of it.

## 1. The tracked line

**Decision: the tracked value stays exactly `runtime.claude.maximal-models=claude-fable-5-1`. This goal makes no further change to `metasystem.conf`.**

Why not the brief's `claude-fable-5-1,claude-fable-5`:

- Re-adding the old id would partly reverse a landing made by Wido's own
  hand on his own word. That is his decision, not the design lane's; if
  he wants it, it is one token and a word, see the reject condition.
- Dual admission is a DRAIN mechanism, and the evidence that licenses
  the drain (an unclosed chain whose critic ran on the old id) lives in
  one seat's jobs directory. The seat can act on it alone: the `.local`
  overlay wins over the tracked line for this key
  (`internal/config/resolve.go:87-95`; proven by
  `internal/dispatch/composition_test.go:269-284`). A seat that finds
  such a chain writes `runtime.claude.maximal-models=claude-fable-5-1,claude-fable-5`
  into its own `metasystem.conf.local` and removes the old token when
  the drain check in §5 is clean. Nothing tracked moves for that.
- Both values validate: the rules are non-empty tokens after trim and no
  duplicates (`internal/config/validate.go:140-151`; the gate re-applies
  the same two rules at `internal/dispatch/hazard.go:123-137`).
  `claude-fable-5-1` alone passes; `claude-fable-5-1,claude-fable-5`
  passes; `claude-fable-5-1,` and any repeated token fail.

Dropping the old id later: nothing remains to drop from the tracked
file. The old id survives only in historical records (receipts,
rulings, past design provenance lines, which record what actually ran
and are not rewritten) and as arbitrary fixture strings (§3).
Keeping those is harmless and correct.

## 2. Where the lane models live

`internal/config/resolve.go` has no model default anywhere: `Get`
resolves flag, environment, `.local` (mode-scoped then base), tracked
mode-scoped, tracked base, then the caller's explicit default
(`resolve.go:50-119`). The tracked conf pins no Fable id: its role model
keys are the literal placeholders `<model>` (`metasystem.conf:47,56`).
So the only tracked line that ever named the model was line 6, and it
has landed. Confirmed.

**Operator note, every seat (m0, m1, m2, m3; m0b already matches at
`.local` lines 11, 15, 25, 35-37), in `metasystem.conf.local`:**

```text
role.default.model.claude=claude-fable-5-1
role.code-critic.model.claude=claude-fable-5-1
mode.design.role.implementer.model.claude=claude-fable-5-1
cap.min.code-critic.claude.claude-fable-5-1=<same minutes as the seat's existing claude-fable-5 row>
cap.min.implementer.claude.claude-fable-5-1=<same minutes as the seat's existing claude-fable-5 row>
```

Key shape `cap.min.<role>.<runtime>.<model>` per
`internal/dispatch/cap.go:40-41`; `claude-fable-5-1` is already canonical
(lowercase, hyphens only, `internal/config/model.go:15-18`), so the key
validates as written. Old `claude-fable-5` cap rows may stay or go: a cap
row is consulted only for the model actually dispatched. If the seat
carries a local `runtime.claude.maximal-models`, it shadows the tracked
line and must itself list `claude-fable-5-1`.

## 3. Tests and fixtures — verdict per file

| File:line | Verdict | Reason |
| --- | --- | --- |
| `internal/dispatch/composition_test.go:256` and `:262` | **MUST CHANGE** to `claude-fable-5-1` | `compositionRepoRoot` (`composition_test.go:13-20`) is `../..`, the REAL metasystem root. The test composes a DESIGN-BEARING claude packet for `claude-fable-5` and expects acceptance, so it asserts against the committed conf. It fails on this tree today: `go test ./internal/dispatch/ -run TestHazardConfigurationAcceptsConfiguredMaximalModel` → "runtime claude has no executable maximal-effort mapping for destructiveReach DESIGN-BEARING" (ran 2026-09-02, worktree without `.local`). |
| `internal/dispatch/composition_test.go:275, 278` | leave | Temp root with its own conf and `.local`; arbitrary string. |
| `internal/dispatch/composition_test.go:441, 447` | leave | Temp repo from `closeReadyHazardChain` → `mirrorFixture` (`decisions_test.go:730-747`, `t.TempDir()`); writes its own conf. |
| `internal/dispatch/decisions_test.go:449, 476, 488` | leave | `t.TempDir()` root with its own conf; arbitrary string. Passes. |
| `internal/dispatch/claim_test.go:75` | leave | `t.TempDir()` root with its own conf; arbitrary string. Passes. |
| `internal/config/validate_test.go:44, 98, 103, 104` | leave | `validConf` constant in a temp repo; the string is the subject of the empty-token and duplicate cases. Passes. |
| `cmd/metasystem/delegate_reroute_test.go:485, 497` | leave | JSON fixture for the modelUsage collapse; any key works. Passes. |

Build rule for the one change: replace the two string literals
`"claude-fable-5"` at lines 256 and 262 with `"claude-fable-5-1"`;
nothing else in that function moves. Accepted non-goal: the test stays
coupled to the developer's committed conf and any `.local`; that is
pre-existing and out of this goal's scope.

## 4. The ruling row

Append to `memory/rulings.md` after the last row (R-45-m0b, line 89);
R-25-m1 is not edited (append-only register):

```text
| R-46-m0b | 2026-09-02 | THE FABLE LANE MOVES TO CLAUDE FABLE 5.1 (Wido, verbatim: "Fable 5.1 is released, make sure we use that model for Fable models going forwards"): every R-25-m1 Fable lane (design authoring, implementation critique) runs on model id claude-fable-5-1 from this date; the tracked runtime.claude.maximal-models admits claude-fable-5-1 alone (landed by Wido's own hand in d081ef07 under R-39-m2); each seat carries the three machine-local lane keys and its cap rows on the new id; claude-fable-5 is retired from dispatch and survives only in historical records and as arbitrary fixture strings. R-25-m1's lane STRUCTURE is unchanged; only the model id moves. A seat that still holds an unclosed hazard-governed chain whose independent critique ran on claude-fable-5 may admit both ids in its own metasystem.conf.local until that chain closes | Given 2026-09-02 morning; CLI acceptance probed the same day (canonicalModel claude-fable-5-1 on Claude Code 2.1.258; the id refuses on CLIs older than 2.1.251). Goal fable-5-1-model-rollover carries the tracked remainder: the composition_test fixture and this row | Wido | |
```

## 5. Live-round safety

The maximal gate fires at two points, and the second is retroactive:

1. Dispatch admission: `ClaimLaunch` → `ValidateRuntimeHazardConfiguration`
   → `runtimeProvesMaximalExecution` (`hazard.go:91-107, 109-138`;
   refusal shape at `claim_test.go:73-89`).
2. Chain closure: `validateIndependentCritiqueReference` re-reads the
   conf AT CLOSE TIME and checks the critic's `requestedModel` against
   it (`hazard.go:293-296`). `composition_test.go:439-462` shows a chain
   whose critic ran on an admitted id being refused with
   `REFUSED-R22-M1-RULING-O-INDEPENDENT-CRITIQUE` after the list changed.

What this goal changes cannot refuse anything in flight: the admission
value is already `claude-fable-5-1` on the tracked line (since d081ef07)
and on m0b's `.local`, and the remaining edits are a fixture string and
a ruling row. What guarantees safety per seat is that the seat's
EFFECTIVE list contains the model of every not-yet-closed chain's
critic. Drain check, run in the seat's `artifacts/agents/jobs/`: no
record with a non-terminal status on `claude-fable-5`, and no root
record with `chainClosed` false whose `independentCritiqueJobRef` names
a job with `requestedModel` `claude-fable-5`. m0b passes it today: zero
non-terminal jobs; the one old-id critic
(`code-critic-1d7716a3e5e141468637ff63`, ended 2026-09-01T19:40:36Z)
reviews a root whose `chainClosed` is true; the newest old-id job of any
role ended 2026-09-02T05:57:58Z, completed. m0, m1, m2, m3 are
unobservable from this worktree; the fleet has been running on the
new-only tracked value since 07:15Z today, so a seat still dispatching
the old id without a local override would already be refused, and the
§1 local dual value is its repair.

## 6. Build plan (Sol lane, zero judgment calls)

1. `internal/dispatch/composition_test.go`: lines 256 and 262, the two
   `"claude-fable-5"` literals become `"claude-fable-5-1"`.
2. `memory/rulings.md`: append the §4 row verbatim after line 89.
3. `metasystem.conf`: no change.
4. Gate: `go test ./internal/dispatch/ -run TestHazardConfiguration -count=1`
   green from a checkout with no `metasystem.conf.local`, then the
   ordinary landing gate.
5. Dispatcher, not implementer: revise the goal record, which still
   describes a dual local value and a pending tracked change.

## 7. Self-grade

- Confidence: high. Every mechanism claim is file:line on this tree and
  the one test verdict that matters was run, not inferred.
- Weakest claim: fleet-wide safety for m0, m1, m2 and m3 rests on the
  statement in Wido's commit message and on the local-overlay rule, not
  on observing those machines' rosters or jobs directories.
- Reject condition: reject §1 if Wido wants the tracked file itself to
  carry `claude-fable-5-1,claude-fable-5` (his call; it reverses his own
  drop), or if any seat reports an unclosed old-id chain and no local
  override. Then the tracked line takes the dual value, the drop
  becomes a later ruling licensed by the §5 drain check on every seat,
  and §3 is unchanged either way.
