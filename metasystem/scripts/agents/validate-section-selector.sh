#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "Usage: scripts/agents/validate-section-selector.sh <catalog|list|twice|run SECTION_ID>" >&2
}

sections() {
  cat <<'EOF'
engine-delivery-contract	engine delivery contract
static-placeholder-scan	static adopted-repository placeholder scan
covenant-evidence-pre-rebuild	covenant evidence before rebuild
go-engine-gate	Go engine gate
supervision-go-fixtures	Go supervision fixtures
gate-fence-fixtures	gate fence fixtures
covenant-evidence-post-rebuild	covenant evidence after rebuild
suite-host-prerequisites	suite host prerequisites
metasystem-audit	metasystem audit
gate-fail-open-tripwire	Go gate fail-open tripwire
witness-gate-fixtures	witness gate fixtures
suite-progress-fixtures	suite progress and watchdog fixtures
land-fixtures	landing chain fixtures
static-contract-audits	static contract audits
supervision-and-census-fixtures	supervision and census fixtures
supervisor-fingerprint-heal-harness	supervisor fingerprint heal harness
mission-fixtures	mission fixtures
shell-and-dependency-audits	shell syntax and dependency audits
conformance-fixtures	conformance fixtures
goal-cli-fixtures	goal command fixtures
telemetry-census-fixtures	telemetry census fixtures
return-schema-fixtures	return schema fixtures
config-identity-fixtures	configuration identity fixtures
authority-regression-fixtures	authority regression fixtures
pre-commit-guard-fixtures	pre-commit guard fixtures
static-reproof-fixtures	static reproof fixtures
project-extra-suites	project-declared extra suites
record-protocol-fixtures	record protocol fixtures
evidence-segment-fixtures	evidence segment fixtures
second-session-fixtures	second-session fixtures
lease-succession-fixtures	lease succession fixtures
flight-recorder-fixtures	flight recorder fixtures
acp-fixtures	Agent Client Protocol fixtures
delegate-caps-fixtures	delegate cap fixtures
adapter-deadline-fixtures	adapter deadline fixtures
enumeration-mode-fixtures	enumeration mode fixtures
runtime-contract-audits	runtime contract audits
agent-protocol-fixtures	agent protocol fixtures
dispatcher-adapter-and-mission-runner-fixtures	dispatcher, adapter, and mission-runner fixtures
workflow-tooling-fixtures	workflow tooling fixtures
adoption-fixtures	adoption fixtures
gate-run-freeze-fixtures	gate-run freeze fixtures
watch-background-jobs-fixtures	background-job watcher fixtures
EOF
}

selected_sections() {
  local root template_mode=0 section_id section_name
  root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
  [[ "${root##*/}" == metasystem && -f "${root%/*}/development/metasystem-design.md" ]] \
    && template_mode=1
  while IFS=$'\t' read -r section_id section_name; do
    if (( ! template_mode )); then
      case "$section_id" in
        witness-gate-fixtures | suite-progress-fixtures | land-fixtures | adoption-fixtures | gate-run-freeze-fixtures)
          continue
          ;;
      esac
    fi
    printf '%s\t%s\n' "$section_id" "$section_name"
  done < <(sections)
}

twice_consulted_sections() {
  # Empty since the continue-and-collect conversion unified the
  # doubled call sites: every section runs exactly once. Add a
  # section id per line here if a double consult ever returns.
  :
}

case ${1:-} in
  catalog)
    [[ $# -eq 1 ]] || { usage; exit 2; }
    sections
    ;;
  list)
    [[ $# -eq 1 ]] || { usage; exit 2; }
    selected_sections
    ;;
  twice)
    [[ $# -eq 1 ]] || { usage; exit 2; }
    twice_consulted_sections
    ;;
  run)
    [[ $# -eq 2 ]] || { usage; exit 2; }
    section_id=$2
    section_known=0
    while IFS=$'\t' read -r known_id _; do
      if [[ "$section_id" == "$known_id" ]]; then
        section_known=1
      fi
    done < <(sections)
    (( section_known )) \
      || { echo "unknown validation section: $section_id" >&2; exit 2; }
    root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
    stage_results_out=${METASYSTEM_ENUMERATION_STAGE_RESULTS_OUT:-}
    if [[ -n "$stage_results_out" ]]; then
      [[ "$stage_results_out" == /* ]] \
        || { echo "enumeration stage-results path must be absolute" >&2; exit 2; }
      METASYSTEM_VALIDATION_STAGE_RESULTS_OUT=$stage_results_out \
      METASYSTEM_VALIDATION_STAGE_RESULTS_WRITER=1 \
      METASYSTEM_ENUMERATION_ENGINE_DEPENDENCY=${METASYSTEM_ENUMERATION_ENGINE_DEPENDENCY:-unproven} \
      METASYSTEM_ENUMERATION_DRIVER=1 \
        bash "$root/scripts/validate-metasystem.sh" --enumeration-section "$section_id"
    else
      METASYSTEM_ENUMERATION_ENGINE_DEPENDENCY=${METASYSTEM_ENUMERATION_ENGINE_DEPENDENCY:-unproven} \
      METASYSTEM_ENUMERATION_DRIVER=1 \
        bash "$root/scripts/validate-metasystem.sh" --enumeration-section "$section_id"
    fi
    ;;
  -h|--help)
    usage
    ;;
  *)
    usage
    exit 2
    ;;
esac
