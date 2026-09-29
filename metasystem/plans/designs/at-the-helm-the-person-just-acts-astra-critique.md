# Astra round 1 (task-mumsddti-olzuyw, commit 5b7d6ac4b), verbatim

One material finding: the checkout comparison rejects an active helm in a submodule’s primary worktree. The ordinary checkout path appears sound by source inspection.

**HA-01 — Medium severity; material: yes**

**Claim.** D-scope assumes `helm.Seat.Checkout` always names the primary worktree. For a submodule, it can name the Git metadata directory instead.

**Evidence, read at the reviewed commit.** [Design:32](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/plans/designs/at-the-helm-the-person-just-acts.md:32) requires the resolved `--show-toplevel` path to equal `Seat.Checkout`. [helm.go:68](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/helm/helm.go:68) initializes `Checkout` to the common directory and substitutes its parent only when its basename is `.git`. [helm.go:82](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/helm/helm.go:82) follows a submodule’s `gitdir:` pointer without recovering its worktree path.

**Concrete failure, inferred from that control flow.** Consider a submodule at `/project/component`, with `.git` pointing to `/project/.git/modules/component`, and a helm held for that submodule. `Active(workTree)` reads its signature successfully, but `Seat.Checkout` is `/project/.git/modules/component`. Git reports `/project/component` as the toplevel. Resolving symlinks cannot make those directories equal. A headless commit on main therefore still reaches the wrapper refusal at [precommit.go:78](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/landing/landpath/precommit.go:78). This fails the first commit in the submodule layout explicitly included in the brief’s threat model.

**Change to the design.** Define primary-worktree identity using Git’s administrative directories rather than assuming the common directory’s parent is the checkout. A bounded approach is to require the resolved committing Git directory to equal its resolved common directory, with that common directory matching the seat whose helm was read. Ordinary linked worktrees have a distinct administrative directory and remain excluded. Add a submodule-primary fixture alongside the existing primary and linked-worktree cases; this need not redesign `internal/helm`.

**Test 1:** DIFFERENT — changes the admission predicate and its fixtures.  
**Test 2:** WORKS/SAFE without it: **no** — the first main-branch commit in this declared layout remains refused.

**Deferred and non-material**

**HA-02 — Low severity; material: no**

**Claim and evidence.** Proof item 3’s “no yield” needs a fixture qualification. [Design:49](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/plans/designs/at-the-helm-the-person-just-acts.md:49) requests ledger and `.orig` refusals with no yield, while D-yield preserves the existing order: the wrapper branch precedes both damage checks in [precommit.go:78](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/landing/landpath/precommit.go:78).

**Concrete consequence.** With `DELEGATE` or `UNTRUSTED` on main and no wrapper token, the wrapper records its yield before the ledger or `.orig` check refuses the commit. The default classifier-unavailable fixture can correctly produce no yield.

**Change to the design.** Clarify that “no yield” uses a fixture that never encounters the wrapper refusal. Preserve the specified guard ordering; a yield records a gate yielding, not a completed commit.

**Test 1:** SAME — the production behavior is already unambiguous.  
**Test 2:** WORKS/SAFE without it: **yes** — both damage checks still refuse.

Slices 2 and 3, richer attribution, and additional recovery machinery remain deferred; none is necessary for the ordinary checkout’s commit-and-push path.

**What I verified holds**

All conclusions below are from reading, not execution.

- **The slice is explicit and small.** It changes two refusal branches, production wiring, help text, and tests. The classifier still runs; its unavailable-classification observation remains intact. The enrollment probe exits before these changes.
- **Ordinary checkout separation is sound.** The entry obtains the actual working directory in [precommit_entry.go:21](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/cmd/metasystem/precommit_entry.go:21). Reading the helm from that directory avoids confusing the installation root with the committing repository. Canonicalizing both comparison operands handles the stated `/tmp` symlink case. Ordinary linked worktrees resolve to their primary seat but fail the proposed worktree comparison.
- **The inspected dispatch path supports the safety premise.** Writable permission requests automatically select worktrees at [dispatch_phase.go:321](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/delegation/dispatch_phase.go:321); permission expansion rejects writes without a worktree and writes outside its workspace at [envelope.go:74](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/dispatch/envelope.go:74). Shared-checkout dispatch exists, but this path does not grant it product-write permission.
- **Existing protections remain.** The design leaves ledger and `.orig` refusals reachable after a wrapper yield. Without eligible helm authority, `wrapperFenced` retains its configured sync-branch, ledger-branch, and seat-checkout behavior.
- **Malformed and old signatures follow existing contracts.** `Active` deliberately treats unreadable signatures as active; existing tests also retain old signatures across restarts. `RecordYield` supplies attribution and process identity and swallows append failures, so recording failure cannot refuse a commit. Consequently, the proposed word “recorded” is not a durability guarantee.
- **The GUI path needs no new terminal interaction.** The inspected hook directly invokes the engine. The proposed no-terminal integration test exercises the relevant classifier behavior. The inspected `.git/hooks` contains no active pre-push hook.
- **No refusal-register anchor needs moving.** Searches for `precommit.go` and pre-commit variants under `internal/refusal` returned no matches. The cited register entries concern other landing files.

Proposed receipt, unwritten: `Independent helm slice-1 design critique at 5b7d6ac4: one material checkout-identity finding; source inspection only.`

**VERDICT: 1 material finding (fail test 2): HA-01**

Commit reviewed: `5b7d6ac4ba710a0bb778e954396d96ee7456e16e`, branch `ui-development`.

Limitations: no tests, builds, commits, or GitHub Desktop actions were run. Failure traces are inferred from committed source. The hook was inspected as a local, unversioned file. No files were changed; the prohibited configuration and artifact directories were not read.

Codex session ID: 01a0ed9e-9deb-7bb2-a339-cd0d6cbfcd9d
Resume in Codex: codex resume 01a0ed9e-9deb-7bb2-a339-cd0d6cbfcd9d


# Astra round 2, the failsafe (task-mumslsjy-v11waj, commit 4da766b7b), verbatim

**HA-01 is FIXED:** the revised predicate admits a submodule’s primary worktree. HA-02’s test wording is also reconciled. One new material finding remains: Git steering can make a linked worktree pass the primary-checkout test.

**HA-03 — High severity; material: yes**

**Claim.** Equality between the effective Git directory and common directory does not prove that the hook runs in that repository’s primary checkout. Both answers honor Git’s environment; the helm reader independently follows the checkout’s on-disk `.git` entry.

**Evidence at the reviewed commit.**

- [Design:32](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/plans/designs/at-the-helm-the-person-just-acts.md:32) requires the two Git answers to match and asserts that `helm.Active(workTree)` necessarily reads the same seat.
- [helm.go:49](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/helm/helm.go:49) resolves the seat through the filesystem. Its deliberate independence from Git steering is covered by [helm_test.go:119](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/helm/helm_test.go:119).
- [landing_path.go:42](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/cmd/metasystem/landing_path.go:42) runs Git with the inherited environment.
- [precommit_entry.go:21](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/cmd/metasystem/precommit_entry.go:21) supplies the hook’s working directory to the guard.

**Concrete failure.** Let `P` be a primary checkout holding the helm and `W` its linked worktree. A headless caller commits with `GIT_DIR=P/.git` and `GIT_WORK_TREE=W`. The hook runs in `W`; `helm.Active(W)` finds P’s active signature. Both Git queries return `P/.git`, so revision 2 grants the yield even though the caller operates in the excluded linked worktree. On main without a wrapper token, today’s guard refuses.

This directory behavior was **verified with read-only Git commands** against the existing `agentic-tools-ui` primary checkout and `agentic-tools-dot` linked worktree:

| Invocation | Git directory | Common directory | Worktree |
|---|---|---|---|
| Ordinary linked worktree | `P/.git/worktrees/agentic-tools-dot` | `P/.git` | `W` |
| Steered as above | `P/.git` | `P/.git` | `W` |

The same mismatch can borrow a held checkout’s signature while targeting another clone’s Git directory. Commit admission is inferred from the proposed predicate; no commit was attempted.

**Change to the design.** Bind the predicate to the actual checkout’s `.git` entry. Require its resolved administrative directory to equal both effective Git directory answers. For example, resolve the explicit `<workTree>/.git` path with `git rev-parse --resolve-git-dir`, then canonicalize and compare. Read-only probes confirmed that this explicit-path resolution still identifies the linked worktree’s administrative directory under the steering above.

Add fixtures for the steered linked-worktree case and a held-checkout/foreign-repository mismatch. Both must retain today’s guard decision and produce no helm yield. Ordinary primary and submodule cases must continue to pass.

**Test 1:** DIFFERENT — changes D-scope’s predicate and fixtures.  
**Test 2:** WORKS/SAFE without it: **no** — an excluded linked-worktree invocation gains admission, and authority can cross repository boundaries.

**Deferred and non-material**

No additional non-material findings are reported. Slices 2 and 3 and broader robustness work remain deferred.

**What I verified holds**

- HA-01’s original submodule failure is removed: eligibility no longer depends on `Seat.Checkout`.
- Ordinary primary and linked-worktree Git directory relationships match the revised design; symlink canonicalization remains specified.
- HA-02 now correctly distinguishes classifier-unavailable fixtures from a wrapper yield followed by a damage-check refusal.
- A no-worktree invocation fails `--show-toplevel` before reaching the proposed predicate; the fold does not change that existing guard path.
- The relevant implementation files are unchanged between the two reviewed commits.

Proposed receipt, unwritten: `Helm design round 2: HA-01 fixed; HA-03 identifies Git-steering admission outside the primary checkout; read-only directory probes and source inspection.`

Reviewed commit: `4da766b7b5558e78a4747c8418e48676b43448b3`, branch `ui-development`. Limitations: no implementation tests, builds, hook execution, or commits; submodule admission and proposed guard outcomes were assessed from source and design. No files were changed or prohibited configuration/artifact contents read.

**VERDICT: 1 material finding (fail test 2): HA-03**

Codex session ID: 01a0ed9e-9deb-7bb2-a339-cd0d6cbfcd9d
Resume in Codex: codex resume 01a0ed9e-9deb-7bb2-a339-cd0d6cbfcd9d


# Astra scoped confirmation read (task-mumsrjfe-n5hcr4, commit befa262cb), verbatim

**HA-03 is FIXED (CONFIRMED)** at `befa262cbfb9d408c5f3edfdbb97ef4cb6251b28`.

[D-scope](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/plans/designs/at-the-helm-the-person-just-acts.md:32) now binds both effective Git directory answers to the worktree’s on-disk `.git` entry. Any resolution failure retains today’s guard behavior.

Read-only Git probes confirmed:

- Primary checkout: all three directories match.
- Ordinary linked worktree: its administrative directory differs from the common directory.
- Steered linked worktree and foreign-repository cases: the on-disk entry differs from the effective directories.

Submodule admission remains correct by inspection: its `.git` entry resolves to its Git directory, also its common directory. This case was not executed.

Test legs 1–3 and §7 agree with the revised predicate. I found no fold-introduced first-use WORK or SAFE failure. No implementation tests or commits were run; no files were changed.

**CONFIRMATION: CONFIRMED**

Codex session ID: 01a0ed9e-9deb-7bb2-a339-cd0d6cbfcd9d
Resume in Codex: codex resume 01a0ed9e-9deb-7bb2-a339-cd0d6cbfcd9d

