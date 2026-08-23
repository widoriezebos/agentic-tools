#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
ms="${METASYSTEM_BIN:-$root/bin/metasystem}"
[[ -x "$ms" ]] || { echo "return schema fixtures: binary absent; run the go gate first" >&2; exit 1; }
fixture=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-return-schema.XXXXXX")
trap 'rm -rf "$fixture"' EXIT

# Atomically replace one top-level field of a JSON object file, leaving
# every other field exactly as the file's parser sees it. `json set`
# covers string and integer fields; this covers null and the object and
# array fields it cannot spell. The whole file and the field are rendered
# by the same engine encoder, so the needle below is byte-exact; a failed
# splice or an unparseable result refuses instead of writing. (Copied from
# supervision-fixtures.sh, extended two ways: string-valued fields print
# bare, so those retry with the quoted spelling; and a read-back proves
# the edit landed on the top-level field, not a nested lookalike.)
json_replace_field() { # file, top-level field, replacement JSON value
  local file=$1 field=$2 new=$3 compact old out canonical staged
  compact=$("$ms" json get --value "{\"root\":$(cat "$file")}" --field root) \
    || { echo "json_replace_field: $file did not parse" >&2; return 1; }
  old=$("$ms" json get --file "$file" --field "$field") \
    || { echo "json_replace_field: $file has no $field" >&2; return 1; }
  out=${compact/"\"$field\":$old"/"\"$field\":$new"}
  if [[ "$out" == "$compact" ]]; then
    out=${compact/"\"$field\":\"$old\""/"\"$field\":$new"}
  fi
  "$ms" util json-validate --value "$out" \
    || { echo "json_replace_field: editing $field left $file unparseable" >&2; return 1; }
  canonical=$("$ms" json get --value "{\"root\":$new}" --field root) \
    || { echo "json_replace_field: replacement for $field is not JSON: $new" >&2; return 1; }
  [[ "$("$ms" json get --value "$out" --field "$field")" == "$canonical" ]] \
    || { echo "json_replace_field: could not locate $field in $file" >&2; return 1; }
  staged=$(mktemp "$(dirname "$file")/.replace.XXXXXX") || return 1
  printf '%s\n' "$out" >"$staged"
  mv "$staged" "$file"
}

# Atomically add one top-level field a JSON object file does not have yet.
# `json set` spells only string and integer values; this inserts the null,
# object, and array values it cannot. The file is re-rendered by the engine
# encoder first so the splice point is exact, and a read-back proves the
# new field landed before the stage replaces the file.
json_insert_field() { # file, new top-level field, JSON value
  local file=$1 field=$2 new=$3 compact out canonical staged
  compact=$("$ms" json get --value "{\"root\":$(cat "$file")}" --field root) \
    || { echo "json_insert_field: $file did not parse" >&2; return 1; }
  if "$ms" json get --file "$file" --field "$field" >/dev/null 2>&1; then
    echo "json_insert_field: $file already has $field" >&2; return 1
  fi
  if [[ "$compact" == "{}" ]]; then
    out="{\"$field\":$new}"
  else
    out="{\"$field\":$new,${compact#\{}"
  fi
  "$ms" util json-validate --value "$out" \
    || { echo "json_insert_field: inserting $field left $file unparseable" >&2; return 1; }
  canonical=$("$ms" json get --value "{\"root\":$new}" --field root) \
    || { echo "json_insert_field: value for $field is not JSON: $new" >&2; return 1; }
  [[ "$("$ms" json get --value "$out" --field "$field")" == "$canonical" ]] \
    || { echo "json_insert_field: $field did not land in $file" >&2; return 1; }
  staged=$(mktemp "$(dirname "$file")/.insert.XXXXXX") || return 1
  printf '%s\n' "$out" >"$staged"
  mv "$staged" "$file"
}

# Atomically remove one top-level field that must exist: the callers model
# a return that HAD the field, so a silent no-op would leave the fixture
# proving nothing.
json_remove_field() { # file, top-level field the file must have
  local file=$1 field=$2 staged
  "$ms" json get --file "$file" --field "$field" >/dev/null \
    || { echo "json_remove_field: $file has no $field" >&2; return 1; }
  staged=$(mktemp "$(dirname "$file")/.remove.XXXXXX") || return 1
  "$ms" json strip --file "$file" --key "$field" >"$staged" \
    || { rm -f "$staged"; return 1; }
  mv "$staged" "$file"
}

# The v1 return every leg starts from, and the record whose observed model
# normalization must prefer over the agent's claim.
cat >"$fixture/v1.json" <<'JSON'
{"jobId":"fixture-job","round":1,"runtime":"fake","sessionId":null,
 "model":{"requested":"requested-model","effective":null},
 "evidence":[],"gaps":[],"mode":"implement","riskiestPart":"fixture",
 "diffBoundary":[],"whatWasDone":"fixture"}
JSON
printf '{"effectiveModel":"observed-model"}\n' >"$fixture/record.json"
# The v2 candidate extends the same v1 shape with a claimed session and model.
cp "$fixture/v1.json" "$fixture/candidate.json"
"$ms" json set --file "$fixture/candidate.json" \
  --int schemaVersion=2 --field sessionId=claimed-session
json_replace_field "$fixture/candidate.json" model \
  '{"requested":"requested-model","effective":"claimed-model"}'
json_insert_field "$fixture/candidate.json" claimed '{"model":"earlier-claim"}'
cp "$fixture/candidate.json" "$fixture/missing-version.json"
json_remove_field "$fixture/missing-version.json" schemaVersion
cp "$fixture/candidate.json" "$fixture/extra.json"
"$ms" json set --file "$fixture/extra.json" --field undeclared=refuse

# A schema is only proven by the provider that enforces it, and this family is
# handed straight to structured output. Two defects shipped because nothing
# checked the shape: an object without `required` and a bare `const` without a
# `type`, each of which failed EVERY codex delegate dispatch before the model
# produced a token. The rules are cheap to state, so they are checked here
# instead of being rediscovered live.
# The structured-output linter moved to the generator's own package
# (TestMaterializedSchemasObeyStructuredOutputRules, internal/returnschema,
# under the go gate — script-fixtures-002/D37): generator invariants of
# Go-owned code belong where they survive fixture retirement. This file
# keeps its thin normalize_return and assert-return-complete legs.

"$root/scripts/assert-return-complete.sh" --role implementer --file "$fixture/v1.json"

# Exercise the adapter's real normalization owner, not a fixture reimplementation.
source "$root/scripts/agents/adapters/runtime-common.sh"
record=$fixture/record.json
round_dir=$fixture
session_id=observed-session
normalize_return "$fixture/candidate.json"
[[ "$("$ms" json get --file "$fixture/return.json" --field schemaVersion)" == 2 ]] \
  || { echo "normalized return lost its schema version" >&2; cat "$fixture/return.json" >&2; exit 1; }
[[ "$("$ms" json get --file "$fixture/return.json" --field sessionId)" == observed-session ]] \
  || { echo "normalized return did not adopt the observed session" >&2; cat "$fixture/return.json" >&2; exit 1; }
[[ "$("$ms" json get --file "$fixture/return.json" --field model.effective)" == observed-model ]] \
  || { echo "normalized return did not adopt the record's observed model" >&2; cat "$fixture/return.json" >&2; exit 1; }
[[ "$("$ms" json get --file "$fixture/return.json" --field claimed)" == \
   "$("$ms" json get --value '{"root":{"sessionId":"claimed-session","model":"claimed-model"}}' --field root)" ]] \
  || { echo "normalized return did not preserve both claims" >&2; cat "$fixture/return.json" >&2; exit 1; }
"$root/scripts/assert-return-complete.sh" --role implementer --file "$fixture/return.json"

# A claim on ONE member and agreement on the other still carries both keys.
# OpenAI structured output rejects an object schema that leaves any property
# out of `required`, and that rejection failed every codex delegate dispatch
# before the model produced a token, so the shape is checked here rather than
# discovered live again.
cp "$fixture/candidate.json" "$fixture/one-claim.json"
"$ms" json set --file "$fixture/one-claim.json" --field sessionId=observed-session
json_replace_field "$fixture/one-claim.json" model \
  '{"requested":"requested-model","effective":"claimed-model"}'
json_remove_field "$fixture/one-claim.json" claimed
normalize_return "$fixture/one-claim.json"
[[ "$("$ms" json get --file "$fixture/return.json" --field claimed)" == \
   "$("$ms" json get --value '{"root":{"sessionId":null,"model":"claimed-model"}}' --field root)" ]] \
  || { echo "a claim on one member did not keep both claimed keys" >&2; cat "$fixture/return.json" >&2; exit 1; }
"$root/scripts/assert-return-complete.sh" --role implementer --file "$fixture/return.json"

if "$root/scripts/assert-return-complete.sh" --role implementer --file "$fixture/missing-version.json" >/dev/null 2>&1; then
  echo "version-2-shaped return without schemaVersion passed the frozen v1 schema" >&2
  exit 1
fi
if "$root/scripts/assert-return-complete.sh" --role implementer --file "$fixture/extra.json" >/dev/null 2>&1; then
  echo "version-2 return with an undeclared property passed" >&2
  exit 1
fi

echo "return schema version 1 compatibility and version 2 normalization fixtures passed"
