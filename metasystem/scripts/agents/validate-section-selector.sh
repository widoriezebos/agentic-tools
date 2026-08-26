#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "Usage: scripts/agents/validate-section-selector.sh <list|run SECTION_ID>" >&2
}

sections() {
  cat <<'EOF'
engine-delivery-contract	engine delivery contract
covenant-evidence-pre-rebuild	covenant evidence before rebuild
go-engine-gate	Go engine gate
supervision-go-fixtures	Go supervision fixtures
gate-fence-fixtures	gate fence fixtures
covenant-evidence-post-rebuild	covenant evidence after rebuild
suite-host-prerequisites	suite host prerequisites
metasystem-audit	metasystem audit
gate-fail-open-tripwire	Go gate fail-open tripwire
witness-gate-fixtures	witness gate fixtures
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

case ${1:-} in
  list)
    [[ $# -eq 1 ]] || { usage; exit 2; }
    sections
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
    METASYSTEM_ENUMERATION_DRIVER=1 \
      bash "$root/scripts/validate-metasystem.sh" --enumeration-section "$section_id"
    ;;
  -h|--help)
    usage
    ;;
  *)
    usage
    exit 2
    ;;
esac
