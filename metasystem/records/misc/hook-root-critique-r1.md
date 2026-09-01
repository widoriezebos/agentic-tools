# Hook-root design critique — round 1 (Sol)

Chain: design implementer-ce272ab63b6ba568322f1e01 (Fable lane) ->
critic design-critic-d72e332214149decc5d43c85 (codex gpt-5.6-sol,
xhigh, fresh context), 2026-09-02. Five material findings; revision 2
folds each by id.

## SHR-ROOT-01 — high, material=True

CLAIM: The proposed metasystem.conf marker is not an authoritative or unique world identifier, so its precedence can recreate the wrong-root defect or silently mask a world the shipped engine still recognizes. The design says, “A directory is a metasystem root exactly when it contains a regular file named metasystem.conf” and makes an ancestor win before the descendant probe at metasystem/plans/supervision-hook-root-design.md:49 and metasystem/plans/supervision-hook-root-design.md:117. A wrapper carrying its own metasystem.conf therefore wins for a hook fired from the wrapper or a sibling subtree, even when the hook executable and actual template world are under wrapper/metasystem. Conversely, a template installation with scripts/agents and only metasystem.conf.local is accepted by the shipped installation and layered-configuration mechanisms but is silently rejected by this resolver. The arbitrary-renamed-configuration case has no shipped support and is not part of this finding. The design must use or explicitly reconcile the shipped root authority and define collision behavior instead of treating one generic filename as proof of governance.

EVIDENCE: Checked by reading metasystem/plans/supervision-hook-root-design.md:49-51, metasystem/plans/supervision-hook-root-design.md:99-119, metasystem/internal/stateroot/stateroot.go:97-108, metasystem/internal/stateroot/stateroot.go:137-162, and metasystem/internal/config/resolve.go:24-27 and 71-95. The shipped hook currently derives repo at metasystem/scripts/agents/supervision-hook.sh:65-66, so the proposed rule would directly determine all subsequent shell consumers.

## SHR-WORKTREE-01 — high, material=True

CLAIM: The design does not establish which world a hook inside a linked delegate worktree should report, and its unsupported assertion that the worktree-local world is “correct” can produce a false turn verdict. The design says the worktree's vendored metasystem is “the nearest governing world, which is correct” at metasystem/plans/supervision-hook-root-design.md:87-93 and repeats that outcome in its case table. Real worktrees contain their tracked plans but not the primary world's ignored artifacts/agents/jobs state; turn reporting reads both collections beneath one supplied root. While a delegate is active, its pending or running job record is in the primary world, so worktree-local reporting can see open plans and zero active jobs. The contract must choose between worktree-local reporting, primary-world reporting, or suppressing delegate hooks, and pin the chosen outcome with an actual linked-worktree fixture.

EVIDENCE: Ran git worktree inspection against an existing delegate worktree: it was its own Git toplevel, had metasystem/metasystem.conf and metasystem/plans, had no metasystem/artifacts/agents/jobs, and had a corresponding job record in the primary metasystem/artifacts/agents/jobs directory. Read metasystem/internal/report/openwork.go:23-28, 72-100, and 220-237, which derive open-work results from exactly those root-relative plans and job records. The proposed fixtures at metasystem/plans/supervision-hook-root-design.md:167-222 create nested and flat repositories but never create a linked Git worktree.

## SHR-EXIT-01 — high, material=True

CLAIM: The benign-exit contract is false for new normalization failures under the shipped shell's strict mode. The design prescribes candidate=$(cd "$cwd" && pwd -P) and equivalent physical-path normalization, then promises, “The resolver introduces no new nonzero exits and no new output channels.” With set -euo pipefail at metasystem/scripts/agents/supervision-hook.sh:2, a current working directory that disappears or becomes inaccessible between Git discovery and normalization exits 1 and emits a cd diagnostic unless every normalization failure is explicitly mapped to silent exit 0. The design and fixture obligations must specify that mapping for each new cd/pwd operation.

EVIDENCE: Read metasystem/plans/supervision-hook-root-design.md:99-115 and 151-165 and metasystem/scripts/agents/supervision-hook.sh:1-2. Ran the proposed assignment shape under set -euo pipefail with an unavailable directory; it exited 1, printed the cd error, and skipped the following command.

## SHR-FIXTURE-01 — medium, material=True

CLAIM: The two nested fixture cases cannot both assert the requested block unless their session identity is specified, but the design states that requirement only for the flat deep-current-working-directory variant. Cases 1 and 2 at metasystem/plans/supervision-hook-root-design.md:189-198 use the same open-work sentinel and both require decision:block. The shipped block-once behavior suppresses the second identical turn for one session. An implementer may reuse one payload, as the existing fixture does, and get a failing second assertion or weaken it. The nested cases need distinct session identifiers or an explicit state reset as a named fixture obligation.

EVIDENCE: Read metasystem/scripts/agents/supervision-fixtures.sh:1544-1555, where replaying one payload blocks only on the first call. Read metasystem/plans/supervision-hook-root-design.md:205-212, where the design correctly calls for a distinct session only in the flat deep-current-working-directory case, leaving the same requirement unstated for nested cases 1 and 2.

## SHR-CONSUMER-01 — high, material=True

CLAIM: The consumer enumeration is textually complete but materially wrong about the up consumer, invalidating the claimed single-root blast analysis. The design says every consumer inherits repo and claims that up will arm at the newly resolved root and return ENROLLMENT_DRIFT for the existing operator-layout fixture. In shipped code, --repo is first reduced to a Git scope, but runUp then derives the state root from --metasystem-root through stateroot.RootForInstallation and overwrites options.Root. Thus changing the shell repo value does not select up's state world: a true template installation was already rooted beneath metasystem, while an adopted-style nested fixture still resolves state to its containing Git toplevel. The consumer matrix and fixture expectations must trace that independent owner or the hook can remain split across two roots.

EVIDENCE: The incorrect claims appear at metasystem/plans/supervision-hook-root-design.md:38-43, 224-245, and 256-273. Shipped behavior is explicit at metasystem/cmd/metasystem/up.go:104-113 and 139-151 and metasystem/internal/stateroot/stateroot.go:97-108. The existing nested operator fixture at metasystem/scripts/agents/supervision-fixtures.sh:586-614 omits the template-mode development marker and enrolls state at the outer scope, so the design's claimed enrollment-drift explanation does not match that fixture.

## Critic-declared gaps (verbatim)

- The live m2 and m3 machines were not accessible from this worktree, so their fleet-state observations were not independently rechecked. None of the five findings depends on those observations.
- A concurrent or background process appended three lines to metasystem/records/narrator-digest.log after the review began. The cause was not established; the file was not reverted or otherwise touched by this critique. The reviewed design and script remained at commit ceca2541db39cd2261356aaf0acec095961cc0ac.
