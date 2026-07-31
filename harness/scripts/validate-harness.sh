#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$root"

scripts/audit-harness.sh .

# Validate every skill present, including project-added and moved optional
# skills, so this script holds in adopted repositories as well as the template.
# A skill directory without a SKILL.md is invisible to the find, so check for
# hollow directories explicitly first.
for dir in skills optional-skills; do
  [[ -d "$dir" ]] || continue
  for d in "$dir"/*/; do
    [[ -d "$d" ]] || continue
    [[ -f "${d}SKILL.md" ]] || { echo "skill directory without SKILL.md: ${d%/}" >&2; exit 1; }
  done
  while IFS= read -r skill_md; do
    scripts/validate-skill.sh "$(dirname "$skill_md")"
  done < <(find "$dir" -name SKILL.md | sort)
done

# Core assets are required everywhere. The full six-skill set with every
# per-runtime profile is required only in the template repository (marked by
# meta/harness-design.md): adopted repositories may prune unused skills, and
# each skill present is still validated by the loop above. Profile files are
# not demanded in adopted mode, because project-added skills never had them
# and core skills may drop them after runtime registration.
template_mode=0
[[ -f meta/harness-design.md ]] && template_mode=1

for link in \
  docs/project-rules.md \
  docs/orchestration.md \
  docs/collaboration.md \
  docs/design/design-principles.md \
  docs/design/design-obligation-gate.md \
  docs/examples/design-obligation-matrix.md \
  docs/examples/step-back-ledger.md \
  .gitattributes \
  plans/instruction-ledger.md \
  scripts/refactor-baseline.sh \
  scripts/frontier.sh \
  scripts/receipt.sh \
  scripts/adopt.sh \
  scripts/enforcement/github-actions-harness.yml \
  scripts/enforcement/claude-code-hooks.json \
  scripts/assert-stop-loss.sh \
  docs/project-adaptation.md \
  docs/harness-reconciliation.md \
  docs/working-modes.md \
  docs/working-with-agents.md \
  plans/README.md; do
  [[ -e "$link" ]] || { echo "missing routed asset: $link" >&2; exit 1; }
done

if (( template_mode )); then
  for link in \
    skills/take-a-step-back/SKILL.md \
    skills/take-a-step-back/agents/claude-profile.md \
    skills/take-a-step-back/agents/devin/AGENT.md \
    skills/take-a-step-back/agents/openai.yaml \
    skills/design-critique/SKILL.md \
    skills/design-critique/agents/claude-profile.md \
    skills/design-critique/agents/devin/AGENT.md \
    skills/design-critique/agents/openai.yaml \
    skills/verify/SKILL.md \
    skills/verify/agents/claude-profile.md \
    skills/verify/agents/devin/AGENT.md \
    skills/verify/agents/openai.yaml \
    skills/refactor/SKILL.md \
    skills/refactor/agents/claude-profile.md \
    skills/refactor/agents/devin/AGENT.md \
    skills/refactor/agents/openai.yaml \
    skills/improve/SKILL.md \
    skills/improve/agents/claude-profile.md \
    skills/improve/agents/devin/AGENT.md \
    skills/improve/agents/openai.yaml \
    skills/retro/SKILL.md \
    skills/retro/agents/claude-profile.md \
    skills/retro/agents/devin/AGENT.md \
    skills/retro/agents/openai.yaml; do
    [[ -e "$link" ]] || { echo "missing template skill asset: $link" >&2; exit 1; }
  done
fi

# Registered skills must track their canonical source under skills/: copies
# must not drift, orphaned copies of a pruned skill must not linger, and a
# symlink to a pruned skill is dangling.
for regroot in .claude/skills .agents/skills; do
  [[ -d "$regroot" ]] || continue
  for reg in "$regroot"/*; do
    [[ -e "$reg" || -L "$reg" ]] || continue
    name=$(basename "$reg")
    if [[ -L "$reg" ]]; then
      [[ -e "$reg" ]] || { echo "registered skill link is dangling: $reg" >&2; exit 1; }
      continue
    fi
    [[ -d "$reg" ]] || continue
    [[ -d "skills/$name" ]] || { echo "orphaned registered skill copy: $reg has no skills/$name source" >&2; exit 1; }
    if ! diff -rq "$reg" "skills/$name" >/dev/null 2>&1; then
      echo "registered skill copy has drifted from its source: $reg vs skills/$name" >&2
      exit 1
    fi
  done
done

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

# The shipped Stop hook must stay rooted and surface via JSON output: hooks
# run in the session's cwd, receipt.sh resolves its ledger from there, and a
# non-blocking exit code shows only a first-line hook-error notice.
hooks_json=scripts/enforcement/claude-code-hooks.json
grep -Fq 'cd \"$CLAUDE_PROJECT_DIR\"' "$hooks_json" || { echo "stop hook is not rooted at CLAUDE_PROJECT_DIR" >&2; exit 1; }
grep -Fq 'systemMessage' "$hooks_json" || { echo "stop hook does not surface a systemMessage when a retro is due" >&2; exit 1; }
if grep -Fq '|| true' "$hooks_json"; then
  echo "stop hook masks the retro-due exit code with || true" >&2
  exit 1
fi
if command -v python3 >/dev/null; then
  hook_cmd=$(python3 -c "import json; print(json.load(open('$hooks_json'))['hooks']['Stop'][0]['hooks'][0]['command'])")
  hookrepo="$tmp/hookrepo"
  mkdir -p "$hookrepo/scripts" "$hookrepo/plans"
  cp scripts/receipt.sh "$hookrepo/scripts/"
  printf '1|1970-01-01T00:00:01Z|RECEIPT|type=implement|outcome=shipped|skills=none|verify=clean|corrections=0|stop_loss=no|note=aged\n' >"$hookrepo/plans/receipts.log"
  out=$(cd "$tmp" && CLAUDE_PROJECT_DIR="$hookrepo" bash -c "$hook_cmd")
  grep -q systemMessage <<<"$out" || { echo "stop hook stayed silent on a due retro" >&2; exit 1; }
  printf '%s|%s|RETRO|note=fixture\n' "$(date -u +%s)" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >>"$hookrepo/plans/receipts.log"
  out=$(cd "$tmp" && CLAUDE_PROJECT_DIR="$hookrepo" bash -c "$hook_cmd")
  [[ -z "$out" ]] || { echo "stop hook emitted output when no retro is due" >&2; exit 1; }
  printf 'garbage\n' >"$hookrepo/plans/receipts.log"
  out=$(cd "$tmp" && CLAUDE_PROJECT_DIR="$hookrepo" bash -c "$hook_cmd")
  grep -q "errored" <<<"$out" || { echo "stop hook hid a failing receipt check" >&2; exit 1; }
  if grep -q "retro due" <<<"$out"; then
    echo "stop hook misreported a check error as a due retro" >&2
    exit 1
  fi
  out=$(cd "$tmp" && CLAUDE_PROJECT_DIR="$tmp/definitely-missing" bash -c "$hook_cmd")
  grep -q "project directory" <<<"$out" || { echo "stop hook stayed silent on an unresolvable project directory" >&2; exit 1; }
fi

# The debug-java preflight is optional: absent in adopted repositories that
# excluded the skill, moved into skills/ in JVM repositories that enabled it.
for preflight in optional-skills/debug-java/scripts/preflight.sh skills/debug-java/scripts/preflight.sh; do
  if [[ -x "$preflight" ]]; then
    touch "$tmp/source" "$tmp/artifact"
    "$preflight" --source "$tmp/source" --artifact "$tmp/artifact" >/dev/null
    touch -t 202001010000 "$tmp/artifact"
    if "$preflight" --source "$tmp/source" --artifact "$tmp/artifact" >/dev/null 2>&1; then
      echo "debug preflight accepted a stale artifact" >&2
      exit 1
    fi
    break
  fi
done

cat >"$tmp/good.md" <<'EOF'
| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| OBL-1 | HIGH | Requirement | Behavior | `owner.py` | `owner.py` | `test_owner.py` | Not applicable: pure derivation | DONE | None |
EOF
scripts/assert-design-obligation-gate.sh --runtime-required --file "$tmp/good.md" >/dev/null

# Proof cells on critical/high rows must be concrete: a DONE row whose proof
# is vague prose must fail, or a declared status can outrun its evidence.
sed 's/| `test_owner.py` |/| covered somewhere |/' "$tmp/good.md" >"$tmp/vague.md"
if scripts/assert-design-obligation-gate.sh --file "$tmp/vague.md" >/dev/null 2>&1; then
  echo "obligation gate accepted a DONE row with a vague proof cell" >&2
  exit 1
fi
# Keyword-carrying prose is still prose: promises of future proof, and owners
# without a code-shaped token, must fail.
cat >"$tmp/keyword.md" <<'EOF'
| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| OBL-1 | CRITICAL | Requirement | Behavior | someone will own this | we should test this later | needs testing | manual test pending | DONE | None |
EOF
if scripts/assert-design-obligation-gate.sh --runtime-required --file "$tmp/keyword.md" >/dev/null 2>&1; then
  echo "obligation gate accepted keyword prose as proof and a prose owner" >&2
  exit 1
fi
sed 's/| Not applicable: pure derivation |/| Not applicable |/' "$tmp/good.md" >"$tmp/bare-na.md"
if scripts/assert-design-obligation-gate.sh --file "$tmp/bare-na.md" >/dev/null 2>&1; then
  echo "obligation gate accepted a bare Not applicable without a reason" >&2
  exit 1
fi
sed 's/| Not applicable: pure derivation |/| Not applicable: |/' "$tmp/good.md" >"$tmp/empty-na.md"
if scripts/assert-design-obligation-gate.sh --file "$tmp/empty-na.md" >/dev/null 2>&1; then
  echo "obligation gate accepted an empty-delimiter Not applicable" >&2
  exit 1
fi
sed 's/| `owner.py` | `owner.py` |/| `owner.py` | pyproject.toml |/' "$tmp/good.md" >"$tmp/toml.md"
scripts/assert-design-obligation-gate.sh --runtime-required --file "$tmp/toml.md" >/dev/null || {
  echo "obligation gate rejected an unbackticked config-file proof path" >&2
  exit 1
}
sed 's/| `owner.py` | `owner.py` |/| `owner.py` | module.mjs |/' "$tmp/good.md" >"$tmp/mjs.md"
scripts/assert-design-obligation-gate.sh --runtime-required --file "$tmp/mjs.md" >/dev/null || {
  echo "obligation gate rejected an unbackticked filename outside the old whitelist" >&2
  exit 1
}
sed 's/| `owner.py` | `owner.py` |/| `owner.py` | compare e.g. the results |/' "$tmp/good.md" >"$tmp/eg.md"
if scripts/assert-design-obligation-gate.sh --file "$tmp/eg.md" >/dev/null 2>&1; then
  echo "obligation gate mistook abbreviation prose for a filename" >&2
  exit 1
fi
# Matrices shown inside fenced code blocks are documentation, not declarations.
{ printf '```markdown\n'; cat "$tmp/good.md"; printf '```\n'; } >"$tmp/fenced.md"
if scripts/assert-design-obligation-gate.sh --file "$tmp/fenced.md" >/dev/null 2>&1; then
  echo "obligation gate read a matrix out of a fenced code block" >&2
  exit 1
fi

sed 's/| DONE |/| MISSING |/' "$tmp/good.md" >"$tmp/bad.md"
if scripts/assert-design-obligation-gate.sh --file "$tmp/bad.md" >/dev/null 2>&1; then
  echo "obligation gate accepted a missing high obligation" >&2
  exit 1
fi

sed 's/| HIGH |/| MEDIUM |/; s/| DONE |/| PARTIAL |/' "$tmp/good.md" >"$tmp/medium.md"
scripts/assert-design-obligation-gate.sh --runtime-required --file "$tmp/medium.md" >/dev/null || {
  echo "obligation gate rejected a valid medium-only matrix" >&2
  exit 1
}

scripts/assert-design-obligation-gate.sh --runtime-required --file docs/examples/design-obligation-matrix.md >/dev/null 2>&1 && {
  echo "example matrix with READY_FOR_RUNTIME passed --runtime-required; negative fixture broken" >&2
  exit 1
}
scripts/assert-design-obligation-gate.sh --file docs/examples/design-obligation-matrix.md >/dev/null

repo="$tmp/baseline-repo"
git init -q "$repo"
git -C "$repo" -c user.name=harness -c user.email=harness@example.invalid commit --allow-empty -qm base
(cd "$repo" && "$root/scripts/refactor-baseline.sh" record --gate "declared acceptance gate" >/dev/null)
(cd "$repo" && "$root/scripts/refactor-baseline.sh" check >/dev/null) || {
  echo "refactor baseline check blocked on the baseline file's own dirt right after record" >&2
  exit 1
}
git -C "$repo" add plans/refactor-baseline
git -C "$repo" -c user.name=harness -c user.email=harness@example.invalid commit -qm baseline
(cd "$repo" && "$root/scripts/refactor-baseline.sh" check >/dev/null)
echo dirty >"$repo/dirty.txt"
if (cd "$repo" && "$root/scripts/refactor-baseline.sh" check >/dev/null 2>&1); then
  echo "refactor baseline check accepted a dirty worktree" >&2
  exit 1
fi
rm "$repo/dirty.txt"
if (cd "$repo" && "$root/scripts/refactor-baseline.sh" check --max-commits 0 >/dev/null 2>&1); then
  echo "refactor baseline check ignored the commit-count backstop" >&2
  exit 1
fi
# Custom and absolute --file paths normalize to the repository root; paths
# outside the repository are rejected because git cannot see their dirt.
(cd "$repo" && "$root/scripts/refactor-baseline.sh" record --gate "declared acceptance gate" --file plans/custom-baseline >/dev/null)
(cd "$repo" && "$root/scripts/refactor-baseline.sh" check --file plans/custom-baseline >/dev/null) || {
  echo "refactor baseline check blocked a custom relative --file right after record" >&2
  exit 1
}
git -C "$repo" add plans/custom-baseline
git -C "$repo" -c user.name=harness -c user.email=harness@example.invalid commit -qm custom-baseline
(cd "$repo" && "$root/scripts/refactor-baseline.sh" record --gate "declared acceptance gate" --file "$repo/plans/abs-baseline" >/dev/null)
(cd "$repo" && "$root/scripts/refactor-baseline.sh" check --file "$repo/plans/abs-baseline" >/dev/null) || {
  echo "refactor baseline check blocked an in-repository absolute --file right after record" >&2
  exit 1
}
git -C "$repo" add plans/abs-baseline
git -C "$repo" -c user.name=harness -c user.email=harness@example.invalid commit -qm abs-baseline
(cd "$repo" && "$root/scripts/refactor-baseline.sh" record --gate "declared acceptance gate" --file "plans/bäseline" >/dev/null)
(cd "$repo" && "$root/scripts/refactor-baseline.sh" check --file "plans/bäseline" >/dev/null) || {
  echo "refactor baseline check blocked a non-ASCII --file right after record (quotePath)" >&2
  exit 1
}
git -C "$repo" add "plans/bäseline"
git -C "$repo" -c user.name=harness -c user.email=harness@example.invalid commit -qm nonascii-baseline
(cd "$repo" && "$root/scripts/refactor-baseline.sh" record --gate "declared acceptance gate" --file "plans/my baseline" >/dev/null)
(cd "$repo" && "$root/scripts/refactor-baseline.sh" check --file "plans/my baseline" >/dev/null) || {
  echo "refactor baseline check blocked a space-containing --file right after record (C-quoting)" >&2
  exit 1
}
git -C "$repo" add "plans/my baseline"
git -C "$repo" -c user.name=harness -c user.email=harness@example.invalid commit -qm space-baseline
if (cd "$repo" && "$root/scripts/refactor-baseline.sh" record --gate "declared acceptance gate" --file "$tmp/outside-baseline" >/dev/null 2>&1); then
  echo "refactor baseline accepted a --file outside the repository" >&2
  exit 1
fi

(cd "$repo" && "$root/scripts/frontier.sh" record --score 80 --min-delta 1 --eval "declared eval" >/dev/null)
if (cd "$repo" && "$root/scripts/frontier.sh" challenge --score 79 >/dev/null 2>&1); then
  echo "frontier challenge accepted a score below the frontier" >&2
  exit 1
fi
if (cd "$repo" && "$root/scripts/frontier.sh" challenge --score 80.5 >/dev/null 2>&1); then
  echo "frontier challenge forgot the stored noise floor" >&2
  exit 1
fi
(cd "$repo" && "$root/scripts/frontier.sh" challenge --score 80.5 --min-delta 0 >/dev/null)
(cd "$repo" && "$root/scripts/frontier.sh" challenge --score 82 >/dev/null)
git -C "$repo" add plans/frontier
git -C "$repo" -c user.name=harness -c user.email=harness@example.invalid commit -qm frontier
if (cd "$repo" && "$root/scripts/frontier.sh" record --score 75 --eval "declared eval" >/dev/null 2>&1); then
  echo "frontier record accepted a regression without --force" >&2
  exit 1
fi
printf 'sha=x\nrecorded_epoch=1\nscore=80\nmin_delta=1\nmax_age_minutes=60\neval=declared\nartifact=\n' >"$tmp/frontier-old"
if scripts/frontier.sh challenge --score 99 --file "$tmp/frontier-old" >/dev/null 2>&1; then
  echo "frontier challenge compared against an expired frontier" >&2
  exit 1
fi
printf 'sha=x\nrecorded_epoch=1\nscore=80\nmin_delta=1\nmax_age_minutes=\neval=declared\nartifact=\n' >"$tmp/frontier-nowindow"
scripts/frontier.sh challenge --score 99 --file "$tmp/frontier-nowindow" >/dev/null
scripts/frontier.sh status --file "$tmp/frontier-nowindow" | grep -qx 'direction=max' || {
  echo "frontier status hid the effective direction of a legacy file" >&2
  exit 1
}
printf 'sha=x\nrecorded_epoch=1\nscore=80\nmin_delta=1\ndirection=sideways\nmax_age_minutes=\neval=declared\nartifact=\n' >"$tmp/frontier-malformed"
if scripts/frontier.sh challenge --score 99 --file "$tmp/frontier-malformed" >/dev/null 2>&1; then
  echo "frontier challenge accepted a malformed persisted direction" >&2
  exit 1
fi
printf 'sha=x\nrecorded_epoch=1\nscore=80\nmin_delta=1\ndirection=\nmax_age_minutes=\neval=declared\nartifact=\n' >"$tmp/frontier-emptydir"
if scripts/frontier.sh challenge --score 99 --file "$tmp/frontier-emptydir" >/dev/null 2>&1; then
  echo "frontier challenge accepted an empty persisted direction" >&2
  exit 1
fi

# Lower-is-better frontiers: persisted direction, force-gated changes, and a
# challenge that only ever uses the stored direction.
(cd "$repo" && "$root/scripts/frontier.sh" record --score 80 --min-delta 1 --direction min --eval "declared eval" --file plans/frontier-min >/dev/null)
git -C "$repo" add plans/frontier-min
git -C "$repo" -c user.name=harness -c user.email=harness@example.invalid commit -qm frontier-min
if (cd "$repo" && "$root/scripts/frontier.sh" challenge --score 79.5 --file plans/frontier-min >/dev/null 2>&1); then
  echo "min-direction challenge accepted a within-noise improvement" >&2
  exit 1
fi
(cd "$repo" && "$root/scripts/frontier.sh" challenge --score 78 --file plans/frontier-min >/dev/null)
(cd "$repo" && HARNESS_FRONTIER_DIRECTION=max "$root/scripts/frontier.sh" challenge --score 78 --file plans/frontier-min >/dev/null) || {
  echo "challenge honored an environment direction instead of the persisted one" >&2
  exit 1
}
if (cd "$repo" && "$root/scripts/frontier.sh" record --score 85 --eval "declared eval" --file plans/frontier-min >/dev/null 2>&1); then
  echo "min-direction record accepted a regression without --force" >&2
  exit 1
fi
if (cd "$repo" && "$root/scripts/frontier.sh" record --score 99 --direction max --eval "declared eval" --file plans/frontier-min >/dev/null 2>&1); then
  echo "frontier record accepted a direction change without --force" >&2
  exit 1
fi
if (cd "$repo" && "$root/scripts/frontier.sh" challenge --score 1 --direction min --file plans/frontier-min >/dev/null 2>&1); then
  echo "frontier challenge accepted a direction flag" >&2
  exit 1
fi

scripts/assert-stop-loss.sh --file docs/examples/step-back-ledger.md >/dev/null
printf '### Cycle C1\n- Classification: no-progress\n### Cycle C2\n- Classification: no-progress\n' >"$tmp/stuck.md"
if scripts/assert-stop-loss.sh --file "$tmp/stuck.md" >/dev/null 2>&1; then
  echo "stop-loss check allowed a third cycle after two no-progress results" >&2
  exit 1
fi
printf -- '- Cycle budget: 2\n### Cycle C1\n- Classification: contract-improved\n### Cycle C2\n- Classification: falsified-continue\n' >"$tmp/spent.md"
if scripts/assert-stop-loss.sh --file "$tmp/spent.md" >/dev/null 2>&1; then
  echo "stop-loss check ignored an exhausted cycle budget" >&2
  exit 1
fi
printf '### Cycle C1\n- Classification: falsified-dead-end\n' >"$tmp/deadend.md"
if scripts/assert-stop-loss.sh --file "$tmp/deadend.md" >/dev/null 2>&1; then
  echo "stop-loss check allowed cycles after a dead end" >&2
  exit 1
fi
printf -- '- No-gain budget: 3\n### Cycle E1\n- Classification: unresolved\n### Cycle E2\n- Classification: falsified-continue\n### Cycle E3\n- Classification: unresolved\n' >"$tmp/nogain.md"
if scripts/assert-stop-loss.sh --file "$tmp/nogain.md" >/dev/null 2>&1; then
  echo "stop-loss check ignored an exhausted no-gain budget over a mixed trailing sequence" >&2
  exit 1
fi
printf -- '- No-gain budget: 3\n### Cycle E1\n- Classification: unresolved\n### Cycle E2\n- Classification: contract-improved\n### Cycle E3\n- Classification: unresolved\n### Cycle E4\n- Classification: falsified-continue\n' >"$tmp/nogain-reset.md"
scripts/assert-stop-loss.sh --file "$tmp/nogain-reset.md" >/dev/null || {
  echo "stop-loss check failed to reset the no-gain count on a contract-improved cycle" >&2
  exit 1
}
printf '### Cycle E1\n- Classification: unresolved\n### Cycle E2\n- Classification: unresolved\n### Cycle E3\n- Classification: unresolved\n' >"$tmp/nogain-optout.md"
scripts/assert-stop-loss.sh --file "$tmp/nogain-optout.md" >/dev/null || {
  echo "stop-loss check blocked unresolved cycles without a declared no-gain budget" >&2
  exit 1
}
printf -- '- No-gain budget: 3\n### Cycle E1\n- Classification: unresolved\n### Cycle E2\n### Cycle E3\n- Classification: falsified-continue\n' >"$tmp/nogain-unclassified.md"
if scripts/assert-stop-loss.sh --file "$tmp/nogain-unclassified.md" >/dev/null 2>&1; then
  echo "stop-loss no-gain count let an unclassified cycle vanish from the tail" >&2
  exit 1
fi
printf -- '- No-gain budget: 3\n### Cycle E1\n- Classification: unresolved\n### Cycle E2\n- Classification: not-contract-improved\n### Cycle E3\n- Classification: falsified-continue\n' >"$tmp/nogain-fakegain.md"
if scripts/assert-stop-loss.sh --file "$tmp/nogain-fakegain.md" >/dev/null 2>&1; then
  echo "stop-loss no-gain count reset on a classification merely containing contract-improved" >&2
  exit 1
fi

rfile="$tmp/receipts.log"
scripts/receipt.sh add --type implement --outcome shipped --file "$rfile" >/dev/null
scripts/receipt.sh check --file "$rfile" >/dev/null
scripts/receipt.sh add --type review --outcome reworked --corrections 1 --file "$rfile" >/dev/null
if scripts/receipt.sh check --max-receipts 1 --file "$rfile" >/dev/null 2>&1; then
  echo "receipt check ignored the receipt-count backstop" >&2
  exit 1
fi
scripts/receipt.sh retro "fixture retro" --file "$rfile" >/dev/null
scripts/receipt.sh check --max-receipts 1 --file "$rfile" >/dev/null
if scripts/receipt.sh add --type bogus --outcome shipped --file "$rfile" >/dev/null 2>&1; then
  echo "receipt add accepted an invalid type" >&2
  exit 1
fi
printf '1|1970-01-01T00:00:01Z|RETRO|note=aged\n' >"$tmp/receipts-aged.log"
scripts/receipt.sh check --max-age-days 0 --file "$tmp/receipts-aged.log" >/dev/null || {
  echo "receipt check demanded a retro over an empty period" >&2
  exit 1
}
scripts/receipt.sh add --type improve --outcome shipped --verify caught --file "$rfile" >/dev/null
scripts/receipt.sh stats --file "$rfile" | grep -q '^receipts=1$' || { echo "receipt stats miscounted the post-retro period" >&2; exit 1; }
scripts/receipt.sh stats --file "$rfile" | grep -q '^type_improve=1$' || { echo "receipt stats missed the improve type" >&2; exit 1; }
scripts/receipt.sh stats --all --file "$rfile" | grep -q '^receipts=3$' || { echo "receipt stats --all miscounted" >&2; exit 1; }

# Every free-text field is sanitized by one shared path: CRLF through the
# note, the skills list, and the retro summary must each stay one log line.
crlf_fixture=$(printf 'a\r\nb')
rfile_crlf="$tmp/receipts-crlf.log"
scripts/receipt.sh add --type implement --outcome shipped --note "$crlf_fixture" --file "$rfile_crlf" >/dev/null 2>&1
[[ $(wc -l <"$rfile_crlf" | tr -d ' ') == 1 ]] || { echo "a CRLF note corrupted the receipt log" >&2; exit 1; }
scripts/receipt.sh add --type implement --outcome shipped --skills "$crlf_fixture" --file "$rfile_crlf" >/dev/null 2>&1
[[ $(wc -l <"$rfile_crlf" | tr -d ' ') == 2 ]] || { echo "a CRLF skills list corrupted the receipt log" >&2; exit 1; }
scripts/receipt.sh retro "$crlf_fixture" --file "$rfile_crlf" >/dev/null 2>&1
[[ $(wc -l <"$rfile_crlf" | tr -d ' ') == 3 ]] || { echo "a CRLF retro summary corrupted the receipt log" >&2; exit 1; }

# Adopted-mode contract: a copy without the template marker validates with a
# skill pruned, and a present-but-broken skill still fails. Template mode
# only, so the nested run (which lacks meta/) cannot recurse.
if (( template_mode )); then
  adopted="$tmp/adopted"
  mkdir -p "$adopted"
  cp -R "$root/." "$adopted"
  rm -rf "$adopted/meta" "$adopted/skills/improve" "$adopted/plans/receipts.log" "$adopted/.claude"
  sed 's/<[^>]*>/filled/g' "$adopted/docs/project-rules.md" >"$adopted/docs/project-rules.md.new"
  mv "$adopted/docs/project-rules.md.new" "$adopted/docs/project-rules.md"
  bash "$adopted/scripts/validate-harness.sh" >/dev/null 2>&1 || {
    echo "adopted-mode validation failed for a copy with one skill pruned" >&2
    exit 1
  }
  mkdir "$adopted/skills/hollow"
  if bash "$adopted/scripts/validate-harness.sh" >/dev/null 2>&1; then
    echo "adopted-mode validation accepted a skill directory without SKILL.md" >&2
    exit 1
  fi
  rmdir "$adopted/skills/hollow"
  grep -v '^name:' "$adopted/skills/verify/SKILL.md" >"$adopted/skills/verify/SKILL.md.new"
  mv "$adopted/skills/verify/SKILL.md.new" "$adopted/skills/verify/SKILL.md"
  if bash "$adopted/scripts/validate-harness.sh" >/dev/null 2>&1; then
    echo "adopted-mode validation accepted a present skill with broken frontmatter" >&2
    exit 1
  fi
fi

# adopt.sh self-test, template mode only. The source is a committed snapshot
# of the current working tree: a git clone would exercise committed HEAD, not
# the implementation under review.
if (( template_mode )); then
  srcrepo="$tmp/adopt-src"
  mkdir -p "$srcrepo"
  cp -R "$root/." "$srcrepo"
  echo 'ignored-fixture.txt' >>"$srcrepo/.gitignore"
  echo junk >"$srcrepo/ignored-fixture.txt"
  git init -q "$srcrepo"
  git -C "$srcrepo" add -A
  git -C "$srcrepo" -c user.name=harness -c user.email=harness@example.invalid commit -qm snapshot
  adopt="$srcrepo/scripts/adopt.sh"
  src_sha=$(git -C "$srcrepo" rev-parse HEAD)

  tgt="$tmp/adopt-default"
  mkdir -p "$tgt"
  printf 'project readme\n' >"$tgt/README.md"
  bash "$adopt" "$tgt" >/dev/null
  [[ -f "$tgt/.github/workflows/harness.yml" ]] || { echo "adopt: CI workflow not installed" >&2; exit 1; }
  [[ -L "$tgt/.claude/skills/verify" ]] || { echo "adopt: claude skill symlink missing" >&2; exit 1; }
  [[ -f "$tgt/.claude/agents/verify.md" ]] || { echo "adopt: claude agent profile missing" >&2; exit 1; }
  grep -q systemMessage "$tgt/.claude/settings.json" || { echo "adopt: settings.json lacks the shipped hook" >&2; exit 1; }
  [[ ! -e "$tgt/optional-skills" ]] || { echo "adopt: unselected optional skills were copied" >&2; exit 1; }
  [[ "$(cat "$tgt/README.md")" == "project readme" ]] || { echo "adopt: the project's own README was touched" >&2; exit 1; }
  [[ ! -e "$tgt/ignored-fixture.txt" ]] || { echo "adopt: ignored source content entered the payload" >&2; exit 1; }
  [[ "$(ls "$tgt/plans" | sort | tr '\n' ' ')" == "README.md instruction-ledger.md known-issues.md " ]] \
    || { echo "adopt: plans/ payload carries more than the standing ledgers" >&2; exit 1; }
  [[ -d "$tgt/artifacts" ]] || { echo "adopt: artifacts directory missing" >&2; exit 1; }
  grep -qxF 'artifacts/' "$tgt/.gitignore" || { echo "adopt: artifacts/ not gitignored" >&2; exit 1; }
  grep -q "$src_sha" "$tgt/docs/project-rules.md" || { echo "adopt: template SHA not recorded" >&2; exit 1; }
  if grep -q '<template sha>' "$tgt/docs/project-rules.md"; then
    echo "adopt: template SHA placeholder left unreplaced" >&2
    exit 1
  fi
  snap="$tmp/adopt-snap"
  mkdir -p "$snap"
  cp -R "$tgt/." "$snap"
  bash "$adopt" "$tgt" >/dev/null
  diff -r "$snap" "$tgt" >/dev/null || { echo "adopt: second run changed an adopted target" >&2; exit 1; }
  if bash "$tgt/scripts/validate-harness.sh" >/dev/null 2>&1; then
    echo "adopt: target validated with unreplaced placeholders" >&2
    exit 1
  fi
  sed 's/<[^>]*>/filled/g' "$tgt/docs/project-rules.md" >"$tgt/docs/project-rules.md.new"
  mv "$tgt/docs/project-rules.md.new" "$tgt/docs/project-rules.md"
  bash "$tgt/scripts/validate-harness.sh" >/dev/null 2>&1 || { echo "adopt: filled target failed validation" >&2; exit 1; }

  bash "$adopt" "$tmp/adopt-devin" --runtimes devin >/dev/null
  [[ -f "$tmp/adopt-devin/.devin/agents/verify/AGENT.md" ]] || { echo "adopt: devin profile missing" >&2; exit 1; }
  [[ ! -e "$tmp/adopt-devin/.claude" ]] || { echo "adopt: devin-only target got .claude state" >&2; exit 1; }
  bash "$adopt" "$tmp/adopt-codex" --runtimes codex >/dev/null
  [[ -L "$tmp/adopt-codex/.agents/skills/verify" ]] || { echo "adopt: codex skill registration missing" >&2; exit 1; }
  bash "$adopt" "$tmp/adopt-none" --runtimes none >/dev/null
  [[ ! -e "$tmp/adopt-none/.claude" && ! -e "$tmp/adopt-none/.devin" && ! -e "$tmp/adopt-none/.agents" ]] \
    || { echo "adopt: --runtimes none still registered a runtime" >&2; exit 1; }
  [[ -f "$tmp/adopt-none/.github/workflows/harness.yml" ]] || { echo "adopt: CI workflow skipped for --runtimes none" >&2; exit 1; }
  bash "$adopt" "$tmp/adopt-java" --enable debug-java >/dev/null
  [[ -f "$tmp/adopt-java/skills/debug-java/SKILL.md" ]] || { echo "adopt: --enable did not move the optional skill" >&2; exit 1; }
  bash "$adopt" "$tmp/adopt-copy" --runtimes claude,codex --copy-skills >/dev/null
  [[ -d "$tmp/adopt-copy/.claude/skills/verify" && ! -L "$tmp/adopt-copy/.claude/skills/verify" ]] \
    || { echo "adopt: --copy-skills did not copy" >&2; exit 1; }
  [[ -d "$tmp/adopt-copy/.agents/skills/verify" && ! -L "$tmp/adopt-copy/.agents/skills/verify" ]] \
    || { echo "adopt: --copy-skills did not copy the codex registration" >&2; exit 1; }
  sed 's/<[^>]*>/filled/g' "$tmp/adopt-copy/docs/project-rules.md" >"$tmp/adopt-copy/docs/project-rules.md.new"
  mv "$tmp/adopt-copy/docs/project-rules.md.new" "$tmp/adopt-copy/docs/project-rules.md"
  bash "$tmp/adopt-copy/scripts/validate-harness.sh" >/dev/null 2>&1 || { echo "adopt: copied-skills target failed validation" >&2; exit 1; }
  echo drift >>"$tmp/adopt-copy/.claude/skills/verify/SKILL.md"
  if bash "$tmp/adopt-copy/scripts/validate-harness.sh" >/dev/null 2>&1; then
    echo "adopt: validation missed a drifted claude skill copy" >&2
    exit 1
  fi
  cp "$tmp/adopt-copy/skills/verify/SKILL.md" "$tmp/adopt-copy/.claude/skills/verify/SKILL.md"
  echo drift >>"$tmp/adopt-copy/.agents/skills/verify/SKILL.md"
  if bash "$tmp/adopt-copy/scripts/validate-harness.sh" >/dev/null 2>&1; then
    echo "adopt: validation missed a drifted codex skill copy" >&2
    exit 1
  fi
  cp "$tmp/adopt-copy/skills/verify/SKILL.md" "$tmp/adopt-copy/.agents/skills/verify/SKILL.md"
  rm -rf "$tmp/adopt-copy/skills/verify"
  if bash "$tmp/adopt-copy/scripts/validate-harness.sh" >"$tmp/orphan.out" 2>&1; then
    echo "adopt: validation missed an orphaned copy of a pruned skill" >&2
    exit 1
  fi
  grep -q "orphaned" "$tmp/orphan.out" || {
    echo "adopt: pruned-skill failure did not name the orphaned copy" >&2
    exit 1
  }

  mkdir -p "$tmp/adopt-foreign"
  touch "$tmp/adopt-foreign/.cursorrules"
  if bash "$adopt" "$tmp/adopt-foreign" >/dev/null 2>&1; then
    echo "adopt: accepted a target with a foreign instruction asset" >&2
    exit 1
  fi
  mkdir -p "$tmp/adopt-collide/docs"
  echo different >"$tmp/adopt-collide/docs/collaboration.md"
  if bash "$adopt" "$tmp/adopt-collide" >/dev/null 2>"$tmp/collide.err"; then
    echo "adopt: overwrote or skipped a colliding payload path" >&2
    exit 1
  fi
  grep -q 'docs/collaboration.md' "$tmp/collide.err" || {
    echo "adopt: collision refusal did not name the colliding path" >&2
    exit 1
  }
  if bash "$adopt" "$tmp/adopt-bogus" --runtimes codez >/dev/null 2>&1; then
    echo "adopt: accepted an unknown runtime name" >&2
    exit 1
  fi
  [[ ! -e "$tmp/adopt-bogus/wow.md" ]] || {
    echo "adopt: a rejected runtime name still mutated the target" >&2
    exit 1
  }
  if bash "$adopt" "$tmp/adopt-nonemix" --runtimes none,claude >/dev/null 2>&1; then
    echo "adopt: accepted the contradictory none-plus-runtime form" >&2
    exit 1
  fi
  [[ ! -e "$tmp/adopt-nonemix/wow.md" ]] || {
    echo "adopt: a rejected runtime combination still mutated the target" >&2
    exit 1
  }
  bash "$adopt" "$tmp/adopt-partial" >/dev/null
  rm "$tmp/adopt-partial/.github/workflows/harness.yml"
  if bash "$adopt" "$tmp/adopt-partial" >/dev/null 2>&1; then
    echo "adopt: rerun over an incomplete installation reported success" >&2
    exit 1
  fi
  bash "$adopt" "$tmp/adopt-partial2" >/dev/null
  rm "$tmp/adopt-partial2/AGENTS.md"
  if bash "$adopt" "$tmp/adopt-partial2" >/dev/null 2>&1; then
    echo "adopt: rerun over a structurally broken installation reported success" >&2
    exit 1
  fi
  echo dirty >>"$srcrepo/wow.md"
  if bash "$adopt" "$tmp/adopt-dirty" >/dev/null 2>&1; then
    echo "adopt: ran from a dirty template worktree" >&2
    exit 1
  fi
  git -C "$srcrepo" checkout -q -- wow.md
  rm -rf "$tgt/skills/take-a-step-back"
  if bash "$tgt/scripts/validate-harness.sh" >"$tmp/dangling.out" 2>&1; then
    echo "adopt: validation missed a dangling registered skill link" >&2
    exit 1
  fi
  grep -q "dangling" "$tmp/dangling.out" || {
    echo "adopt: pruned-skill failure did not name the dangling link" >&2
    exit 1
  }
fi

echo "harness validation passed"
