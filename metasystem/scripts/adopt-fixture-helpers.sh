#!/usr/bin/env bash
# Shared assertion bodies for ordinary adoption and its scoped comparison.

fill_harness_conf() { # config path, absolute evidence root
  # Point evidence at the harness sandbox and give every rostered
  # role a fixture model, so a nested validation never resolves a
  # real model or writes evidence outside the fixture tree. Tier 1
  # then names exactly the fixture models (sorted, deduplicated) and
  # every deeper tier empties.
  local path=$1 evidence=$2 line key value runtime joined out=""
  local model_key='^(role\.[a-z0-9-]+|mode\.[a-z0-9-]+\.role\.[a-z0-9-]+)\.model\.([a-z0-9-]+)$'
  local models=()
  while IFS= read -r line || [[ -n "$line" ]]; do
    if [[ "$line" != *=* ]]; then out+="$line"$'\n'; continue; fi
    key=${line%%=*}
    value=${line#*=}
    if [[ "$key" == evidence.root ]]; then
      value=$evidence
    elif [[ "$key" == dispatch.cap-min ]]; then
      # The target's own delivery validation reserves this many minutes; its
      # watchdog stops the suite at that absolute deadline.
      value=40
    elif [[ "$key" =~ $model_key ]]; then
      runtime=${BASH_REMATCH[2]}
      value="fixture-$runtime-model"
      models+=("$runtime:$value")
    elif [[ "$key" == model.tier.1 ]]; then
      value=__MODELS__
    elif [[ "$key" == model.tier.* ]]; then
      value=""
    fi
    out+="$key=$value"$'\n'
  done <"$path"
  joined=""
  if [[ ${#models[@]} -gt 0 ]]; then
    joined=$(printf '%s\n' "${models[@]}" | LC_ALL=C sort -u | paste -sd, -)
  fi
  printf '%s' "${out//model.tier.1=__MODELS__/model.tier.1=$joined}" >"$path"
}

fill_harness_testing_contract() { # source contract, adopted contract
  # Adoption correctly installs an explicitly incomplete contract. This
  # fixture then performs the application's ordinary reviewed tailoring:
  # the adopted engine source is at the application root rather than below a
  # metasystem/ prefix. Keep every group and assertion from the source
  # contract; only translate that physical prefix and working directory.
  local source=$1 target=$2
  sed -e 's#"metasystem/#"#g' -e 's#"cwd":"metasystem"#"cwd":"."#g' \
    "$source" >"$target.new"
  mv "$target.new" "$target"
}

copy_tree_without_artifacts() { # source root, destination
  # Only for copies whose source is the live metasystem root: artifacts/ is
  # runtime state, not shipped content, and copying it races
  # with any job writing lock directories, and an adoption fixture has no use
  # for it. Excluding it makes the suite safe to run while work is in flight.
  local from=$1 to=$2 entry
  mkdir -p "$to"
  (cd "$from" && for entry in * .[!.]*; do
    [[ -e "$entry" ]] || continue
    [[ "$entry" == artifacts || "$entry" == .git || "$entry" == metasystem.conf.local ]] && continue
    cp -R "$entry" "$to/"
  done)
}

prepare_filled_target_covenant() {
  local tgt=$1
  # The covenant evidence gate rides the same green validate (counselor
  # slice one): a fully valid covenant + canonical table + present deps
  # must pass, and the run must PROVE the gate fired, not skipped.
  printf '#!/usr/bin/env bash\nset -euo pipefail\nexec go run ./src\n' >"$tgt/gate.sh"
  mkdir -p "$tgt/src"
  cat >"$tgt/src/app.go" <<'APP'
package main

import "fmt"

func main() {
	fmt.Println("hello")
	fmt.Println("metric=greets=0")
}
APP
  cat >"$tgt/covenant.json" <<'COVENANT'
{
  "schemaVersion": 1,
  "identity": {"name": "adopt-bed", "entryPoint": "bash gate.sh", "sourcePaths": ["src/"]},
  "requirements": [
    {"id": "1", "ref": "criterion 1: the app greets by name", "proof": "greets"}
  ],
  "battery": {"command": "bash gate.sh", "metric": "greets", "direction": "max", "threshold": ">=1"},
  "budgets": [],
  "guards": [],
  "guardrails": ["gate.sh", "docs/covenant-evidence.md"]
}
COVENANT
  cat >"$tgt/docs/covenant-evidence.md" <<'EVIDENCE'
# Covenant evidence — adopt-bed

| criterion id | criterion | proof id | kind | exact command | repo deps | evidence source | status |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | The app greets by name | greets | repo | bash gate.sh | gate.sh,src/app.go | gate.sh runs the entrypoint | observed |

Wired: 1. Floating: 0.
EVIDENCE
}

assert_filled_target_delivery() {
  local tgt=$1
  local diagnostic_command=(bash "$tgt/scripts/validate-metasystem.sh" --delivery-contract)
  if [[ "${2:-}" == shared-testing ]]; then
    # The first filled delivery below still consumes the whole contract.
    # Fault injections then drive the same covenant owner directly: a
    # deliberately changed table cannot reuse the green source's proof.
    diagnostic_command=("$tgt/bin/metasystem" covenant evidence --root "$tgt")
    "$tgt/bin/metasystem" test verify --root "$tgt" --tree "$(git -C "$tgt" write-tree)" \
      >"$tmp/filled-testing-prerequisite.log" 2>&1 \
      || { cat "$tmp/filled-testing-prerequisite.log" >&2; return 1; }
  fi
  # Capture, never discard: the receipt-stats flake's nested firings kept
  # dying invisibly behind this redirect (2026-08-14, evidence 51987/94210).
  # Digest equality before the nested run: the staged payload must be the
  # exact content the outer witness gate proved (D33) — when a witness is
  # armed, the check-only probe IS that comparison.
  if [[ -n "${METASYSTEM_GATE_WITNESS:-}" ]]; then
    ( cd "$tgt" && METASYSTEM_GATE_WITNESS_CONSUMER_SCOPE=DELIVERY \
        bash scripts/agents/go-gate.sh --witness-check-only >/dev/null ) \
      || { echo "adopt: staged payload digest does not match the witness-gated tree" >&2; exit 1; }
  fi
  bash "$tgt/scripts/validate-metasystem.sh" --delivery-contract >"$tmp/adopt-filled.out" 2>&1 || {
    echo "adopt: filled target failed validation" >&2
    grep -n -A 12 'SECTION RED' "$tmp/adopt-filled.out" >&2 || true
    tail -20 "$tmp/adopt-filled.out" >&2
    exit 1
  }
  # D17's whole point, asserted: with the source shipped, the adopted
  # target's own validation rebuilds and gates the engine.
  grep -Fq 'metasystem delivery contract validated' "$tmp/adopt-filled.out" \
    || { echo "adopt: filled target did not end on the contract verdict" >&2; exit 1; }
  if [[ -n "${METASYSTEM_GATE_WITNESS:-}" ]]; then
    grep -Fq 'outer witness' "$tmp/adopt-filled.out" \
      || { echo "adopt: filled target did not accept the outer witness" >&2; exit 1; }
  fi
  grep -Fq 'go gate: PASSED' "$tmp/adopt-filled.out" \
    || { echo "adopt: filled target did not run the go gate" >&2; exit 1; }
  grep -Fq 'covenant evidence gate passed' "$tmp/adopt-filled.out" \
    || { echo "adopt: the covenant evidence gate did not fire in the green run" >&2; exit 1; }
  # The red half: remove exactly the one criterion/proof pair the
  # covenant cites and the same validation must refuse, NAMING the
  # missing pair — then the table restores byte-for-byte so the later
  # fixtures keep judging the green bed.
  cp "$tgt/docs/covenant-evidence.md" "$tmp/evidence-green-reference.md"
  sed 's/| greets |/| salutes |/' "$tgt/docs/covenant-evidence.md" >"$tgt/docs/covenant-evidence.md.new"
  mv "$tgt/docs/covenant-evidence.md.new" "$tgt/docs/covenant-evidence.md"
  if "${diagnostic_command[@]}" >"$tmp/evidence-red.out" 2>&1; then
    echo "adopt: validation passed while the covenant cited a proof the table does not record" >&2
    exit 1
  fi
  grep -Fq 'bound to proof greets in the covenant but records proof salutes' "$tmp/evidence-red.out" \
    || { echo "adopt: the evidence refusal did not name the missing pair" >&2; tail -5 "$tmp/evidence-red.out" >&2; exit 1; }
  cp "$tmp/evidence-green-reference.md" "$tgt/docs/covenant-evidence.md"
  # A malformed entry at the covenant's home must REFUSE, never read
  # as absent: a dangling symlink is the shape only no-follow presence
  # detection sees (directory and FIFO refusals are the engine's,
  # pinned by its unit tests on the same branch).
  mv "$tgt/covenant.json" "$tmp/covenant-green-reference.json"
  ln -s covenant-that-does-not-exist.json "$tgt/covenant.json"
  if "${diagnostic_command[@]}" >"$tmp/evidence-symlink.out" 2>&1; then
    echo "adopt: validation passed while the covenant home held a dangling symlink" >&2
    exit 1
  fi
  grep -q "symlink" "$tmp/evidence-symlink.out" \
    || { echo "adopt: the symlinked covenant refusal did not name the symlink" >&2; tail -5 "$tmp/evidence-symlink.out" >&2; exit 1; }
  rm "$tgt/covenant.json"
  mv "$tmp/covenant-green-reference.json" "$tgt/covenant.json"

}

assert_copied_registration_positive() {
  local srcrepo=$1 copied=$2 copied_setup projection
  copied_setup=$(
    "$copied/bin/metasystem" runtime setup --repo "$copied" \
      --runtimes claude,codex --copy-skills --check
  ) || { echo "adopt: filled copied registrations failed shared setup check" >&2; exit 1; }
  grep -Fq 'TEST_CONTRACT_READY' <<<"$copied_setup" \
    || { echo "adopt: copied registration setup passed without a ready testing contract" >&2; exit 1; }
  for projection in ENGINE PAYLOAD; do
    "$copied/bin/metasystem" behavior-surface digest --root "$srcrepo" \
      --projection "$projection" --endpoint copied-registration \
      >"$tmp/copied-source-$projection.json"
    "$copied/bin/metasystem" behavior-surface digest --root "$copied" \
      --projection "$projection" --endpoint copied-registration \
      >"$tmp/copied-target-$projection.json"
    cmp -s "$tmp/copied-source-$projection.json" "$tmp/copied-target-$projection.json" \
      || { echo "adopt: copied target changed non-tailored $projection bytes" >&2; exit 1; }
  done
  METASYSTEM_ENUMERATION_ENGINE_DEPENDENCY=ready \
    bash "$copied/scripts/agents/validate-section-selector.sh" run runtime-contract-audits \
    >"$tmp/nested-copied-skills.log" 2>&1 \
    || { echo "adopt: copied-skills runtime contract audit failed" >&2; tail -20 "$tmp/nested-copied-skills.log" >&2; exit 1; }
}

assert_copied_registration_drift() {
  local copied=$1
  echo drift >>"$copied/.claude/skills/verify/SKILL.md"
  if METASYSTEM_ENUMERATION_ENGINE_DEPENDENCY=ready \
      bash "$copied/scripts/agents/validate-section-selector.sh" run runtime-contract-audits \
      >"$tmp/copied-claude-drift.out" 2>&1; then
    echo "adopt: validation missed a drifted claude skill copy" >&2
    exit 1
  fi
  grep -Fq 'registered skill copy has drifted from its source: .claude/skills/verify vs skills/verify' \
    "$tmp/copied-claude-drift.out" \
    || { echo "adopt: drifted claude skill copy refusal did not name its registration" >&2; exit 1; }
  cp "$copied/skills/verify/SKILL.md" "$copied/.claude/skills/verify/SKILL.md"
  echo drift >>"$copied/.agents/skills/verify/SKILL.md"
  if METASYSTEM_ENUMERATION_ENGINE_DEPENDENCY=ready \
      bash "$copied/scripts/agents/validate-section-selector.sh" run runtime-contract-audits \
      >"$tmp/copied-codex-drift.out" 2>&1; then
    echo "adopt: validation missed a drifted codex skill copy" >&2
    exit 1
  fi
  grep -Fq 'registered skill copy has drifted from its source: .agents/skills/verify vs skills/verify' \
    "$tmp/copied-codex-drift.out" \
    || { echo "adopt: drifted codex skill copy refusal did not name its registration" >&2; exit 1; }
  cp "$copied/skills/verify/SKILL.md" "$copied/.agents/skills/verify/SKILL.md"
}

assert_filled_target_mutations() {
  local tgt=$1 missing_module_banner
  # A positive delivery gate establishes engine readiness before each mutation;
  # the negative calls below target registration auditing only.
  echo drift >>"$tgt/.claude/agents/verify.md"
  if METASYSTEM_ENUMERATION_ENGINE_DEPENDENCY=ready \
      bash "$tgt/scripts/agents/validate-section-selector.sh" run runtime-contract-audits \
      >"$tmp/profile-drift.out" 2>&1; then
    echo "adopt: validation missed a drifted claude profile" >&2
    exit 1
  fi
  grep -q 'profile drifted' "$tmp/profile-drift.out" \
    || { echo "adopt: profile-drift failure did not name the profile" >&2; exit 1; }
  cp "$tgt/skills/verify/agents/claude-profile.md" "$tgt/.claude/agents/verify.md"

  # The D17 fail-open, closed and asserted (D33): a source-delivery target
  # whose go.mod vanished must FAIL — never read as "no engine expected".
  mv "$tgt/go.mod" "$tgt/go.mod.hidden"
  mkdir -p "$tmp/gomod-gone-progress"
  # The already-built target engine establishes actual worker custody even
  # when the source module needed by the ordinary bootstrap is absent.
  missing_module_banner=$("$tgt/bin/metasystem" proof-run banner \
    --suite adoption-missing-module --root "$tgt" \
    --progress "$tgt/artifacts/agents/supervision/suite-progress.jsonl" \
    --log "$tmp/gomod-gone-progress.log")
  if "$tgt/bin/metasystem" proof-run launch --suite adoption-missing-module \
      --root "$tgt" --control-root "$tgt" --goal adoption-goal --cap-min 1 \
      --conf "$tgt/metasystem.conf" \
      --progress "$tgt/artifacts/agents/supervision/suite-progress.jsonl" \
      --banner "$missing_module_banner" \
      --log "$tmp/gomod-gone-progress.log" --tmp "$tmp/gomod-gone-progress" -- \
      env METASYSTEM_SUITE_PROGRESS_ACTIVE=1 \
        METASYSTEM_SUITE_PROGRESS_SUITE=validate-metasystem \
        METASYSTEM_SUITE_PROGRESS_ROOT="$tgt" \
        METASYSTEM_SUITE_PROGRESS_DEPTH=0 \
        METASYSTEM_SUITE_PROGRESS_TMP="$tmp/gomod-gone-progress" \
        METASYSTEM_SUITE_PROGRESS_LOG="$tmp/gomod-gone-progress.log" \
        bash "$tgt/scripts/validate-metasystem.sh" --delivery-contract \
      >"$tmp/gomod-gone.out" 2>&1; then
    echo "adopt: a source-delivery target without go.mod validated green" >&2
    exit 1
  fi
  grep -Fq 'engine source did not ship' "$tmp/gomod-gone.out" \
    || { echo "adopt: the missing-go.mod refusal did not name the delivery" >&2; tail -5 "$tmp/gomod-gone.out" >&2; exit 1; }
  mv "$tgt/go.mod.hidden" "$tgt/go.mod"

}

assert_copied_registration_orphan() {
  local copied=$1
  rm -rf "$copied/skills/verify"
  if METASYSTEM_ENUMERATION_ENGINE_DEPENDENCY=ready \
      bash "$copied/scripts/agents/validate-section-selector.sh" run runtime-contract-audits \
      >"$tmp/orphan.out" 2>&1; then
    echo "adopt: validation missed an orphaned copy of a pruned skill" >&2
    exit 1
  fi
  grep -q "orphaned" "$tmp/orphan.out" || {
    echo "adopt: pruned-skill failure did not name the orphaned copy" >&2
    exit 1
  }

}
