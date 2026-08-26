#!/usr/bin/env bash
# IL-28 fixtures: the landing boundary re-proves the static checks via
# go-gate.sh --fast before any commit concludes. Leg 1 proves the SHAPE
# (the re-proof sits in commit.sh between the wrapper token and the
# commit, with no environment escape). Legs 2 and 3 prove the BEHAVIOR
# on a stubbed boundary: a red fast gate refuses the commit and names
# the re-proof; a green fast gate lets the commit conclude.
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)

# Leg 1: shape. The re-proof invokes the fast gate before the commit and
# carries no environment switch that could outlive an edit loop.
wrapper="$root/scripts/agents/commit.sh"
tail_body=$(awk '/IL-28 static re-proof/{flag=1} flag' "$wrapper")
[[ -n "$tail_body" ]] \
  || { echo "static re-proof fixture: commit.sh lost its IL-28 stanza" >&2; exit 1; }
grep -Fq 'go-gate.sh" --fast' <<<"$tail_body" \
  || { echo "static re-proof fixture: the boundary does not invoke go-gate.sh --fast" >&2; exit 1; }
grep -Fq -- '--fast --proof-out' "$wrapper" \
  || { echo "static re-proof fixture: the boundary's gate call lost its side-effect-free --proof-out" >&2; exit 1; }
gate_line=$(grep -n 'go-gate.sh" --fast' "$wrapper" | head -1 | cut -d: -f1)
commit_line=$(grep -n 'git -C "$root" commit --trailer' "$wrapper" | head -1 | cut -d: -f1)
[[ -n "$gate_line" && -n "$commit_line" && "$gate_line" -lt "$commit_line" ]] \
  || { echo "static re-proof fixture: the re-proof does not precede the commit" >&2; exit 1; }
escape_scan() {
  grep -Eq 'METASYSTEM[A-Z_]*(SKIP|FAST|REPROOF)' "$1"
}
if escape_scan "$wrapper"; then
  echo "static re-proof fixture: an environment escape appeared in the wrapper" >&2
  exit 1
fi

# Legs 2 and 3: behavior, on a stubbed boundary. The copied wrapper
# resolves its engine and gate inside the fixture root, so a stub engine
# answers the lease verbs and a stub gate flips the verdict.
tmp=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-static-reproof.XXXXXX")
trap 'rm -rf "$tmp"' EXIT
fixture_root="$tmp/metasystem"
mkdir -p "$fixture_root/scripts/agents" "$fixture_root/bin" "$fixture_root/artifacts/agents/mains"
cp "$wrapper" "$fixture_root/scripts/agents/commit.sh"
# The push leg mirrors transport through the sync script, so the bed
# carries it — a wrapper dependency absent from the bed reads as a
# wrapper defect and turns this fixture red for the wrong reason.
cp "$root/scripts/agents/sync-transport.sh" "$fixture_root/scripts/agents/sync-transport.sh"
# A permissive audit stub: the audit legs belong to the real suite; these
# legs prove the gate coupling.
cat >"$fixture_root/scripts/audit-metasystem.sh" <<'SH'
#!/usr/bin/env bash
exit 0
SH
chmod +x "$fixture_root/scripts/audit-metasystem.sh"
cat >"$fixture_root/bin/metasystem" <<'SH'
#!/usr/bin/env bash
case "$1 $2" in
  "lease require-holder") echo '{}' ;;
  "proc started-at") echo 1 ;;
  "util token-hex") echo cafecafecafecafecafecafecafecafe ;;
  "lease commit-token") : ;;
  "behavior-surface select")
    while IFS= read -r -d '' path; do
      case "$path" in artifacts/*|bin/*|plans/goals/*|plans/goals.md|plans/goals-accepted.json|plans/receipts.log|metasystem.conf.local) ;;
        *) printf '%s\0' "$path" ;;
      esac
    done ;;
  *) : ;;
esac
exit 0
SH
chmod +x "$fixture_root/bin/metasystem"
cat >"$fixture_root/scripts/agents/proof-engine.sh" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
[[ -z "${STATIC_REPROOF_POLICY_ENGINE_MARKER:-}" ]] || : >"$STATIC_REPROOF_POLICY_ENGINE_MARKER"
case "$1 $2" in
  "behavior-surface select")
    while IFS= read -r -d '' path; do
      case "$path" in artifacts/*|bin/*|plans/goals/*|plans/goals.md|plans/goals-accepted.json|plans/receipts.log|metasystem.conf.local) ;;
        *) printf '%s\0' "$path" ;;
      esac
    done ;;
  "gate weight-add") echo "proof-engine weight refusal" >&2; exit 1 ;;
  *) : ;;
esac
SH
chmod +x "$fixture_root/scripts/agents/proof-engine.sh"
export STATIC_REPROOF_POLICY_ENGINE_MARKER="$tmp/proof-engine-used"

git -C "$fixture_root" init -q -b main
# Beds enroll a fixture nickname: publishing surfaces refuse without
# one, and a bed must exercise the stamp, not the refusal.
git -C "$fixture_root" config metasystem.goal.machine fixture-machine
# Repo-local identity: the wrapper's own `git commit` (leg 3) must not
# depend on ambient user configuration on a clean runner (IL28-R1-2).
git -C "$fixture_root" config user.name fixture
git -C "$fixture_root" config user.email fixture@example.invalid
printf 'seed\n' >"$fixture_root/README"
# The whole fixture scaffold is TRACKED at the seed: the boundary's own
# input closure refuses untracked files under scripts/, and the scaffold
# must not trip the very check it exists to prove.
printf 'bin/\nartifacts/\n' >"$fixture_root/.gitignore"
git -C "$fixture_root" add -A
git -C "$fixture_root" commit -qm seed
printf 'change\n' >"$fixture_root/README"
git -C "$fixture_root" add README

# Leg 2: a red fast gate refuses the commit and names the re-proof. The
# stub is STAGED each time it changes: the boundary's own input closure
# (untracked and diverged gate inputs refuse) would otherwise fire first.
cat >"$fixture_root/scripts/agents/go-gate.sh" <<'SH'
#!/usr/bin/env bash
exit 1
SH
chmod +x "$fixture_root/scripts/agents/go-gate.sh"
git -C "$fixture_root" add scripts/agents/go-gate.sh
set +e
refusal=$("$fixture_root/scripts/agents/commit.sh" __lease-held human -m "must refuse" 2>&1)
status=$?
set -e
[[ $status -ne 0 ]] \
  || { echo "static re-proof fixture: a red fast gate did not refuse the commit" >&2; exit 1; }
grep -Fq "static re-proof failed" <<<"$refusal" \
  || { echo "static re-proof fixture: the refusal did not name the re-proof: $refusal" >&2; exit 1; }
git -C "$fixture_root" diff --cached --quiet && {
  echo "static re-proof fixture: the refused commit concluded anyway" >&2; exit 1; }

# Leg 3: a green fast gate lets the commit conclude.
cat >"$fixture_root/scripts/agents/go-gate.sh" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
proof_out=
while (($#)); do
  case "$1" in
    --proof-out) proof_out=$2; shift 2 ;;
    *) shift ;;
  esac
done
[[ -n "$proof_out" ]]
cp "$(cd "$(dirname "$0")" && pwd -P)/proof-engine.sh" "$proof_out"
chmod +x "$proof_out"
SH
chmod +x "$fixture_root/scripts/agents/go-gate.sh"
git -C "$fixture_root" add scripts/agents/go-gate.sh
"$fixture_root/scripts/agents/commit.sh" __lease-held human -q -m "concludes green" \
  || { echo "static re-proof fixture: a green fast gate blocked the commit" >&2; exit 1; }
[[ "$(git -C "$fixture_root" log --format=%s -1)" == "concludes green" ]] \
  || { echo "static re-proof fixture: the green-gate commit did not land" >&2; exit 1; }
[[ -f "$STATIC_REPROOF_POLICY_ENGINE_MARKER" ]] \
  || { echo "static re-proof fixture: the proof-built policy engine branch was not exercised" >&2; exit 1; }

# Weight bookkeeping is NON-FATAL by proof, not by inspection. The
# proof-built engine refuses the weight verb; the live stub below is stale and
# cannot replace the prospective policy owner.
cat >"$fixture_root/bin/metasystem" <<'SH'
#!/usr/bin/env bash
case "$1 $2" in
  "lease require-holder") echo '{}' ;;
  "proc started-at") echo 1 ;;
  "util token-hex") echo cafecafecafecafecafecafecafecafe ;;
  "lease commit-token") : ;;
  "behavior-surface select")
    while IFS= read -r -d '' path; do
      case "$path" in artifacts/*|bin/*|plans/goals/*|plans/goals.md|plans/goals-accepted.json|plans/receipts.log|metasystem.conf.local) ;;
        *) printf '%s\0' "$path" ;;
      esac
    done ;;
  "gate weight-add") echo "stub weight refusal" >&2; exit 1 ;;
  *) : ;;
esac
exit 0
SH
chmod +x "$fixture_root/bin/metasystem"
printf 'weight-refused\n' >>"$fixture_root/README"
git -C "$fixture_root" add README
"$fixture_root/scripts/agents/commit.sh" __lease-held human -q -m "concludes despite weight refusal" \
  || { echo "static re-proof fixture: a weight bookkeeping failure refused a lawful landing" >&2; exit 1; }
[[ "$(git -C "$fixture_root" log --format=%s -1)" == "concludes despite weight refusal" ]] \
  || { echo "static re-proof fixture: the weight-refusal commit did not land" >&2; exit 1; }

# Leg 4 (IL28-R1-1): a gate input differing between index and worktree
# refuses — the proof must bind the bytes the commit records, and a
# staged-red/worktree-repaired divergence is exactly what it cannot bind.
mkdir -p "$fixture_root/internal/red"
printf 'package red\n' >"$fixture_root/internal/red/red.go"
git -C "$fixture_root" add internal/red/red.go
printf 'package repaired\n' >"$fixture_root/internal/red/red.go"
set +e
diverged=$("$fixture_root/scripts/agents/commit.sh" __lease-held human -m "must refuse divergence" 2>&1)
status=$?
set -e
[[ $status -ne 0 ]] \
  || { echo "static re-proof fixture: a diverged gate input did not refuse the commit" >&2; exit 1; }
grep -Fq "not what the commit would record" <<<"$diverged" \
  || { echo "static re-proof fixture: the divergence refusal did not name the remedy: $diverged" >&2; exit 1; }
git -C "$fixture_root" add internal/red/red.go
"$fixture_root/scripts/agents/commit.sh" __lease-held human -q -m "converged concludes" \
  || { echo "static re-proof fixture: a converged gate input blocked the commit" >&2; exit 1; }

# Leg 5 (IL28-R1-3, mutation): an environment guard placed BEFORE the
# IL-28 marker must still be caught by the escape scan — the scan reads
# the whole wrapper, never just the stanza.
mutated="$tmp/mutated-commit.sh"
{ head -5 "$wrapper"; echo '[[ -n "${METASYSTEM_SKIP_REPROOF:-}" ]] && exec git commit "$@"'; tail -n +6 "$wrapper"; } >"$mutated"
if ! escape_scan "$mutated"; then
  echo "static re-proof fixture: the escape scan missed a pre-marker environment guard" >&2
  exit 1
fi

# Leg 6 (IL28-R2-1): an UNTRACKED gate input refuses — the gate would
# build it while the commit omits it.
printf 'package stray\n' >"$fixture_root/internal/red/stray.go"
printf 'tick\n' >>"$fixture_root/README"
git -C "$fixture_root" add README
set +e
stray=$("$fixture_root/scripts/agents/commit.sh" __lease-held human -m "must refuse stray" 2>&1)
status=$?
set -e
[[ $status -ne 0 ]] \
  || { echo "static re-proof fixture: an untracked gate input did not refuse the commit" >&2; exit 1; }
grep -Fq "stray.go" <<<"$stray" \
  || { echo "static re-proof fixture: the untracked refusal did not name the file: $stray" >&2; exit 1; }
rm "$fixture_root/internal/red/stray.go"

# Leg 7 (IL28-R2-2/R3-2/R4-1/R4-5, postcondition form): content
# selection in ANY spelling lands a tree the proof never judged; the
# boundary detects the tree mismatch after the fact, rolls the commit
# back softly, and refuses. Two files staged, a positional pathspec
# commits one — the wrapper must undo it and leave the stage intact.
printf 'alpha\n' >"$fixture_root/internal/red/alpha.txt"
printf 'beta\n' >"$fixture_root/internal/red/beta.txt"
git -C "$fixture_root" add internal/red/alpha.txt internal/red/beta.txt
before_head=$(git -C "$fixture_root" rev-parse HEAD)
staged_tree=$(git -C "$fixture_root" write-tree)
set +e
selected=$("$fixture_root/scripts/agents/commit.sh" __lease-held human -m "must roll back" internal/red/alpha.txt 2>&1)
status=$?
set -e
[[ $status -ne 0 ]] \
  || { echo "static re-proof fixture: a pathspec commit was not refused" >&2; exit 1; }
grep -Fq "never judged" <<<"$selected" \
  || { echo "static re-proof fixture: the postcondition refusal did not explain itself: $selected" >&2; exit 1; }
[[ "$(git -C "$fixture_root" rev-parse HEAD)" == "$before_head" ]] \
  || { echo "static re-proof fixture: the pathspec commit was not rolled back" >&2; exit 1; }
# The rollback preserves the EXACT proved index (IL28-R5-7): the same
# write-tree, not merely "some cached change".
[[ "$(git -C "$fixture_root" write-tree)" == "$staged_tree" ]] \
  || { echo "static re-proof fixture: the rollback did not preserve the proved index tree" >&2; exit 1; }
engine_before=$(shasum -a 256 "$fixture_root/bin/metasystem" | cut -d' ' -f1)
"$fixture_root/scripts/agents/commit.sh" __lease-held human -q -m "plain message concludes" \
  || { echo "static re-proof fixture: a plain -m commit was blocked" >&2; exit 1; }
[[ "$(git -C "$fixture_root" rev-parse 'HEAD^{tree}')" == "$staged_tree" ]] \
  || { echo "static re-proof fixture: the concluding commit did not record the proved tree" >&2; exit 1; }
# The boundary never touches the supervised engine (the armed-checkout
# fingerprint incident): bin/metasystem is byte-identical after a landing.
[[ "$(shasum -a 256 "$fixture_root/bin/metasystem" | cut -d' ' -f1)" == "$engine_before" ]] \
  || { echo "static re-proof fixture: the landing boundary rewrote bin/metasystem" >&2; exit 1; }

# Leg 8b (landing-tooling-fixes): --push lands on BOTH declared
# remotes or fails by name — the rule agents remembered around the
# tooling is now the tooling's.
origin_bare=$(mktemp -d)/origin.git
transport_bare=$(mktemp -d)/transport.git
git init -q --bare -b main "$origin_bare"
git init -q --bare -b main "$transport_bare"
git -C "$fixture_root" remote add origin "$origin_bare"
git -C "$fixture_root" remote add transport "$transport_bare"
printf 'landed\n' >"$fixture_root/internal/red/landed.txt"
git -C "$fixture_root" add internal/red/landed.txt
"$fixture_root/scripts/agents/commit.sh" __lease-held human --push -q -m "push lands both remotes" \
  || { echo "static re-proof fixture: --push refused a lawful landing" >&2; exit 1; }
pushed_head=$(git -C "$fixture_root" rev-parse HEAD)
[[ "$(git -C "$origin_bare" rev-parse main)" == "$pushed_head" ]] \
  || { echo "static re-proof fixture: --push did not land origin" >&2; exit 1; }
[[ "$(git -C "$transport_bare" rev-parse main)" == "$pushed_head" ]] \
  || { echo "static re-proof fixture: --push did not land transport" >&2; exit 1; }
# The mirror's own refusals: an explicitly EMPTY branch and a
# newline-bearing name both die with the named refusal before git
# runs — an empty argument must never silently become the default.
sync="$fixture_root/scripts/agents/sync-transport.sh"
for bad in "" $'main\nevil'; do
  sync_rc=0
  sync_out=$(cd "$fixture_root" && bash "$sync" "$bad" 2>&1) || sync_rc=$?
  [[ $sync_rc == 2 && "$sync_out" == *"is not a plain branch"* ]] \
    || { echo "static re-proof fixture: the mirror accepted an unlawful branch name (rc=$sync_rc)" >&2; exit 1; }
done
# A TAG shadowing the branch name plus a stale tracking ref must not
# select what lands. Advance origin ALONE, then leave the tag and
# transport at the prior head and force the tracking ref one commit
# further back. A no-op, the tag, and the stale ref are therefore all
# distinguishable from origin's true branch head.
printf 'origin advances\n' >"$fixture_root/internal/red/origin-advances.txt"
git -C "$fixture_root" add internal/red/origin-advances.txt
git -C "$fixture_root" commit -qm "origin advances beyond its mirrors"
true_head=$(git -C "$fixture_root" rev-parse HEAD)
git -C "$fixture_root" push -q origin refs/heads/main:refs/heads/main
git -C "$origin_bare" update-ref refs/tags/main "$pushed_head"
stale=$(git -C "$fixture_root" rev-parse "$pushed_head^")
git -C "$fixture_root" update-ref refs/remotes/origin/main "$stale"
[[ "$true_head" != "$pushed_head" && "$stale" != "$pushed_head" \
  && "$(git -C "$origin_bare" rev-parse refs/heads/main)" == "$true_head" \
  && "$(git -C "$origin_bare" rev-parse refs/tags/main)" == "$pushed_head" \
  && "$(git -C "$transport_bare" rev-parse refs/heads/main)" == "$pushed_head" ]] \
  || { echo "static re-proof fixture: the tag-plus-stale-ref bed is not discriminating" >&2; exit 1; }
# No argument exercises the mirror's sole lawful default while the
# discriminating bed proves which main ref it transports.
( cd "$fixture_root" && bash "$sync" >/dev/null ) \
  || { echo "static re-proof fixture: the mirror refused a lawful sync" >&2; exit 1; }
[[ "$(git -C "$fixture_root" rev-parse refs/remotes/origin/main)" == "$true_head" ]] \
  || { echo "static re-proof fixture: the mirror did not fetch origin's true branch head" >&2; exit 1; }
[[ "$(git -C "$transport_bare" rev-parse refs/heads/main)" == "$true_head" ]] \
  || { echo "static re-proof fixture: the mirror pushed something other than origin's branch head" >&2; exit 1; }
git -C "$fixture_root" remote remove origin
git -C "$fixture_root" remote remove transport


# Leg 8 (IL28-R3-1/R4-2): a red audit refuses the commit by its OWN
# exact message. The red shim is STAGED so the closure passes and the
# audit actually executes — an unstaged red shim would trip the
# divergence refusal instead and falsely certify this leg (IL28-R6-5).
printf 'gamma\n' >"$fixture_root/internal/red/alpha.txt"
git -C "$fixture_root" add internal/red/alpha.txt
cat >"$fixture_root/scripts/audit-metasystem.sh" <<'SH'
#!/usr/bin/env bash
exit 1
SH
git -C "$fixture_root" add scripts/audit-metasystem.sh
set +e
audited=$("$fixture_root/scripts/agents/commit.sh" __lease-held human -m "must refuse audit" 2>&1)
status=$?
set -e
[[ $status -ne 0 ]] \
  || { echo "static re-proof fixture: a red audit did not refuse the commit" >&2; exit 1; }
grep -Fq "the static re-proof failed (audit-metasystem.sh)" <<<"$audited" \
  || { echo "static re-proof fixture: the audit refusal did not carry the audit's own message: $audited" >&2; exit 1; }
cat >"$fixture_root/scripts/audit-metasystem.sh" <<'SH'
#!/usr/bin/env bash
exit 0
SH
git -C "$fixture_root" add scripts/audit-metasystem.sh

# Leg 9 (IL28-R3-3): an IGNORED gate input refuses — the toolchain would
# consume it while the commit omits it forever.
printf 'internal/red/generated.go\n' >>"$fixture_root/.gitignore"
git -C "$fixture_root" add .gitignore
printf 'package red\n' >"$fixture_root/internal/red/generated.go"
set +e
shadowed=$("$fixture_root/scripts/agents/commit.sh" __lease-held human -m "must refuse ignored" 2>&1)
status=$?
set -e
[[ $status -ne 0 ]] \
  || { echo "static re-proof fixture: an ignored gate input did not refuse the commit" >&2; exit 1; }
grep -Fq "generated.go" <<<"$shadowed" \
  || { echo "static re-proof fixture: the ignored refusal did not name the file: $shadowed" >&2; exit 1; }
rm "$fixture_root/internal/red/generated.go"
"$fixture_root/scripts/agents/commit.sh" __lease-held human -q -m "audit and stage converge" \
  || { echo "static re-proof fixture: the converged tail commit was blocked" >&2; exit 1; }

# Leg 10: every landing names the machine it came from — the wrapper
# stamps the Machine trailer (hostname plus lineage), so provenance
# never depends on what an author typed.
stamped=$(git -C "$fixture_root" log -1 --format=%B)
machine_line=$(grep '^Machine: ' <<<"$stamped" | head -1)
[[ "$machine_line" == "Machine: fixture-machine+"* ]] \
  || { echo "static re-proof fixture: the landing's Machine trailer is not the enrolled nickname: ${machine_line:-absent}" >&2; exit 1; }

# Leg 11: replacing a whole projected directory with one staged symlink must
# refuse just like a symlink at a critical leaf. Git records only the target
# text while the proof can follow an external directory tree.
directory_target=$tmp/projected-directory-target
subdirectory_target=$tmp/projected-subdirectory-target
mkdir "$directory_target" "$subdirectory_target"
printf 'external proof input\n' >"$directory_target/project-rules.md"
printf 'package gaterun\n' >"$subdirectory_target/weight.go"
ln -s "$directory_target" "$fixture_root/docs"
ln -s "$subdirectory_target" "$fixture_root/internal/gaterun"
git -C "$fixture_root" add docs internal/gaterun
set +e
directory_symlink=$($fixture_root/scripts/agents/commit.sh __lease-held human -m "must refuse directory symlink" 2>&1)
status=$?
set -e
[[ $status -ne 0 ]] \
  || { echo "static re-proof fixture: a staged directory-level symlink did not refuse the commit" >&2; exit 1; }
grep -Fq "docs" <<<"$directory_symlink" \
  || { echo "static re-proof fixture: the directory symlink refusal did not name the path: $directory_symlink" >&2; exit 1; }
grep -Fq "internal/gaterun" <<<"$directory_symlink" \
  || { echo "static re-proof fixture: the projected subdirectory symlink refusal did not name the path: $directory_symlink" >&2; exit 1; }

echo "static re-proof fixtures: PASSED"
