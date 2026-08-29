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

# The v3 critic grammar is exercised through the same materializer and return
# validator used by adapters and dispatch. JSON construction and inspection go
# through the engine too, so these legs do not maintain a second parser.
cat >"$fixture/critic-v3-base.json" <<'JSON'
{"schemaVersion":3,"claimed":{"sessionId":null,"model":null},
 "jobId":"fixture-critic","round":1,"runtime":"fake","sessionId":"fake-session",
 "model":{"requested":"fixture-model","effective":"fixture-model"},
 "evidence":[],"gaps":[],"mode":"critique","reviewedCommit":"abc1234",
 "findings":[],"verdictMaterialCount":0,"rigor":[]}
JSON

safe_facts='{"local":true,"recoverable":true,"proofBoundaryCrossed":false,"authorityBoundaryCrossed":false,"secretsBoundaryCrossed":false,"irreversibleDataBoundaryCrossed":false,"externalSideEffectBoundaryCrossed":false}'

write_v3_return() { # output, findings JSON, material count, rigor JSON
  local output=$1 findings=$2 count=$3 rigor=$4
  cp "$fixture/critic-v3-base.json" "$output"
  json_replace_field "$output" findings "$findings"
  "$ms" json set --file "$output" --int verdictMaterialCount="$count"
  json_replace_field "$output" rigor "$rigor"
}

expect_v3_pass() { # leg, findings JSON, material count, rigor JSON
  local leg=$1 findings=$2 count=$3 rigor=$4 candidate
  candidate="$fixture/$leg.json"
  write_v3_return "$candidate" "$findings" "$count" "$rigor"
  "$ms" validate return-complete --root "$root" --role design-critic --file "$candidate"
}

expect_v3_refusal() { # leg, findings JSON, material count, rigor JSON, diagnostic
  local leg=$1 findings=$2 count=$3 rigor=$4 diagnostic=$5 candidate errors
  candidate="$fixture/$leg.json"
  errors="$fixture/$leg.err"
  write_v3_return "$candidate" "$findings" "$count" "$rigor"
  if "$ms" validate return-complete --root "$root" --role design-critic \
      --file "$candidate" >"$fixture/$leg.out" 2>"$errors"; then
    echo "$leg: invalid version-3 critic return passed" >&2
    exit 1
  fi
  grep -Fq "$diagnostic" "$errors" \
    || { echo "$leg: refusal did not name $diagnostic" >&2; cat "$errors" >&2; exit 1; }
}

finding_f1='[{"id":"F1","severity":"high","material":true,"claim":"fixture claim","evidence":"fixture evidence"}]'
bounded_row='[{"findingId":"F1","rigorClass":"bounded","facts":'"$safe_facts"',"reopeningTrigger":"reopen if the finding recurs"}]'

expect_v3_pass zero-material-empty-rigor '[]' 0 '[]'

lawful_findings='[{"id":"F1","severity":"high","material":true,"claim":"bounded","evidence":"read"},{"id":"F2","severity":"critical","material":true,"claim":"severe","evidence":"read"},{"id":"F3","severity":"medium","material":true,"claim":"unproven","evidence":"read"}]'
lawful_rigor='[{"findingId":"F1","rigorClass":"bounded","facts":'"$safe_facts"',"reopeningTrigger":"reopen if it recurs"},{"findingId":"F2","rigorClass":"severe","facts":'"$safe_facts"',"reopeningTrigger":"reopen until the invariant is proved"},{"findingId":"F3","rigorClass":"unproven","facts":'"$safe_facts"',"reopeningTrigger":"reopen when classification evidence exists"}]'
expect_v3_pass lawful-bounded-severe-unproven "$lawful_findings" 3 "$lawful_rigor"

expect_v3_refusal missing-rigor-row "$finding_f1" 1 '[]' \
  'missing a classification row for material finding "F1"'
expect_v3_refusal extra-rigor-row '[]' 0 "$bounded_row" \
  'which is not a material finding'
malformed_facts='{"local":true,"recoverable":true,"proofBoundaryCrossed":false,"authorityBoundaryCrossed":false,"secretsBoundaryCrossed":false,"irreversibleDataBoundaryCrossed":false}'
malformed_row='[{"findingId":"F1","rigorClass":"bounded","facts":'"$malformed_facts"',"reopeningTrigger":"reopen if it recurs"}]'
expect_v3_refusal malformed-facts "$finding_f1" 1 "$malformed_row" \
  '$.rigor[0].facts.externalSideEffectBoundaryCrossed is required'

empty_id='[{"id":"","severity":"high","material":true,"claim":"empty id","evidence":"read"}]'
expect_v3_refusal empty-finding-id "$empty_id" 1 '[]' \
  '$.findings[0].id must be a non-empty string without surrounding whitespace'
whitespace_id='[{"id":" F1 ","severity":"high","material":true,"claim":"spaced id","evidence":"read"}]'
expect_v3_refusal whitespace-finding-id "$whitespace_id" 1 '[]' \
  '$.findings[0].id must be a non-empty string without surrounding whitespace'
duplicate_ids='[{"id":"F1","severity":"high","material":true,"claim":"first","evidence":"read"},{"id":"F1","severity":"low","material":true,"claim":"second","evidence":"read"}]'
expect_v3_refusal duplicate-finding-id "$duplicate_ids" 2 "$bounded_row" \
  'duplicates finding identifier "F1"'

# The classifier has no standalone command surface. These focused package
# invocations execute its production normalization owner and keep each
# validation-order rule independently visible in this fixture suite.
(
  cd "$root"
  go test ./internal/critique -run '^TestNormalizeRecurrenceToUnproven$' -count=1
  go test ./internal/critique -run '^TestNormalizeDangerousFactsToSevere$' -count=1
  go test ./internal/critique -run '^TestNormalizeUnknownDangerousClassStaysUnproven$' -count=1
)

# Version 1 and version 2 are frozen byte contracts, represented by their
# engine-computed schema digests.
"$ms" schema materialize --root "$root" --role code-critic --version 1 \
  --output "$fixture/code-critic-v1.schema.json"
"$ms" schema materialize --root "$root" --role code-critic --version 2 \
  --output "$fixture/code-critic-v2.schema.json"
[[ "$("$ms" util sha256 --file "$fixture/code-critic-v1.schema.json")" == \
   fe4ec2d623507feed6a5dbbdf6e4040ced855348d111f79e43ced4129a96943c ]] \
  || { echo "version-1 critic schema bytes changed" >&2; exit 1; }
[[ "$("$ms" util sha256 --file "$fixture/code-critic-v2.schema.json")" == \
   6161117b74d84c34941d0181030b99108869421b2044b8bf80b539ee26e33056 ]] \
  || { echo "version-2 critic schema bytes changed" >&2; exit 1; }

cat >"$fixture/fake-critic-record.json" <<'JSON'
{"jobId":"fake-critic-v3","round":1,"role":"code-critic","sessionId":"fake-session",
 "requestedModel":"fake-model","effectiveModel":"fake-model"}
JSON
printf 'Working Mode: critique\n' >"$fixture/fake-critic-prompt.md"
"$ms" adapter fake-return --record "$fixture/fake-critic-record.json" \
  --prompt "$fixture/fake-critic-prompt.md" --output "$fixture/fake-critic-return.json"
"$ms" validate return-complete --root "$root" --role code-critic \
  --file "$fixture/fake-critic-return.json"
[[ "$("$ms" json get --file "$fixture/fake-critic-return.json" --field schemaVersion)" == 3 ]] \
  || { echo "fake critic did not speak return schema version 3" >&2; exit 1; }
[[ "$("$ms" json get --file "$fixture/fake-critic-return.json" --field rigor)" == '[]' ]] \
  || { echo "zero-finding fake critic did not emit empty rigor" >&2; exit 1; }

echo "return schema version 1, version 2, and critic version 3 fixtures passed"
