package audit

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func acceptedHookStartSource() string {
	return fmt.Sprintf(`#!/usr/bin/env bash
runtime=${1-}
event=${2-}
if [[ "$runtime" == claude && "$event" == tool ]]; then
  [[ -n ${METASYSTEM_HOOK_DELEGATE_JOB:-} ]] && exit 0
  [[ -r cache ]] || exit 0
  exec engine adapter claude-tool-gate --root installation
fi
start_terminal_complete=false
start_arming_status=0
start_wait_recovery_status=0
start_deferred_identity_read=false
start_brain_failure=
start_holder_context_notice='holder notice'
start_last_resort='fallback'
start_notice_json() {
  local key=$1
  case "$key" in
    %s)
      start_notice='{"systemMessage":"notice"}' ;;
    *) return 1 ;;
  esac
}
start_emergency() {
  builtin trap - EXIT HUP INT TERM
  start_terminal_complete=true
  builtin exit 0
}
start_json_escape() {
  start_escaped=$1
}
start_append_notice_text() {
  : "$1" "$2"
}
start_finish() {
  builtin trap - EXIT HUP INT TERM
  if [[ "$start_deferred_identity_read" != true && "${identity_pid:-}" =~ ^[1-9][0-9]*$ ]]; then :; fi
  case "${1-}" in
    notice)
      if [[ ( "$key" == brain-boot || "$key" == brain-timeout ) && -n "$start_notices" ]]; then
        start_json_escape "$start_notices"
        start_append_notice_text "$response" "$start_escaped"
      fi
      ;;
    intentional)
      local key=$2
      case "$key" in
        %s)
          : ;;
        *) start_emergency ;;
      esac
      ;;
  esac
  start_terminal_complete=true
  builtin exit 0
}
start_finish_prepared() {
  if [[ "$start_brain_failure" == brain-boot ]]; then
    start_finish notice brain-boot
  elif [[ "$start_brain_failure" == brain-timeout ]]; then
    start_finish notice brain-timeout
  elif [[ "$start_context_kind" == channel ]]; then
    start_finish intentional context-ready
  elif [[ "$start_context_kind" == screen ]]; then
    start_finish intentional screen-context-ready
  elif [[ -n "$start_notices" ]]; then
    start_finish intentional notices-ready
  else
    start_finish intentional healthy-no-context
  fi
}
start_defer_brain_failure() {
  case ${1-} in
    brain-boot)
      start_brain_failure=brain-boot
      ;;
    brain-timeout)
      start_brain_failure=brain-timeout
      ;;
    *) start_finish notice unexpected-termination ;;
  esac
}
start_capture_defer_brain_failure() {
  case ${1-} in
    brain-boot) start_defer_brain_failure brain-boot ;;
    brain-timeout) start_defer_brain_failure brain-timeout ;;
    *) return 1 ;;
  esac
}
start_capture() {

  local failure_key=$1 destination=$2 policy=$3 captured= capture_status=0
  shift 3
  if [[ "$policy" == arming || "$policy" == wait-recovery ]]; then
    captured=$(builtin trap - EXIT HUP INT TERM; "$@" 2>&1) && capture_status=0 || capture_status=$?
  elif captured=$(builtin trap - EXIT HUP INT TERM; "$@"); then
    capture_status=0
  else
    capture_status=$?
  fi
  case "$policy" in
    required)
      if start_capture_defer_brain_failure "$failure_key"; then return 0; fi
      ;;
    arming)
      start_arming_status=$capture_status
      ;;
    wait-recovery)
      start_wait_recovery_status=$capture_status
      ;;
    identity-required-nonempty)
      start_deferred_identity_read=true
      ;;
  esac
}
collect_start_notice() {
  : "$1"
}
start_prepare_brain() {
  :
}
start_exit_trap() {
  start_finish notice unexpected-termination
}
start_signal() {
  start_finish notice interrupted
}
start_main() {
  start_capture engine-missing value required tool verb
  start_capture holder-read parent_view identity-required-nonempty tool verb
  start_prepare_brain
  if [[ "$start_deferred_identity_read" == true ]]; then
    collect_start_notice "$start_holder_context_notice"
  else
    start_capture arming up_output arming start_up "$runtime" "$session" "$identity_pid" "$identity_started"
    up_aggregate=${up_output##*$'\n'}
    if (( start_arming_status != 0 )); then
      collect_start_notice "Metasystem supervision arming failed: $up_aggregate"
      if [[ "$up_aggregate" == *' re-armed='* ]]; then
        collect_start_notice "Metasystem re-armed the rebuilt engine: $up_aggregate"
      fi
    elif [[ "$up_aggregate" == *' re-armed='* ]]; then
      collect_start_notice "Metasystem re-armed the rebuilt engine: $up_aggregate"
    fi
  fi
  start_capture wait-recovery waiting_lines wait-recovery "$ms" session start --root "$repo" --session "$session"
  if (( start_wait_recovery_status == 0 )); then
    :
  elif (( start_wait_recovery_status != 64 )); then
    waiting_lines=${waiting_lines//$'\n'/'; '}
    collect_start_notice "Metasystem could not read durable wait recovery rows for this session: $waiting_lines"
  fi
  start_finish_prepared
  start_finish notice engine-missing
}
if [[ "$event" == start ]]; then
  builtin trap 'start_exit_trap "$?"' EXIT
  builtin trap 'start_signal 129' HUP
  builtin trap 'start_signal 130' INT
  builtin trap 'start_signal 143' TERM
fi
set -euo pipefail
if [[ "$event" == start ]]; then
  start_main
  start_finish notice unexpected-termination
fi
# SessionStart cannot pass this dispatcher.
exit 0
`, strings.Join(allStartNoticeKeys(), "|"), strings.Join(allStartIntentionalKeys(), "|"))
}

func completeHookStartFixtureLabels() string {
	var lines []string
	for _, key := range allStartOutcomeKeys() {
		if key == "engine-skew" {
			lines = append(lines, "# hook-start-case: engine-skew-start outcome=engine-skew")
		} else {
			lines = append(lines, "# hook-start-case: "+key)
		}
	}
	lines = append(lines, "TestHookStartDeclaredOutcomeMatrixOnBash32 TestHookStartContextOutcomeShapesOnBash32 TestHookStartIntentionalFullPathFixturesOnBash32 TestHookStartFailureBranchFixturesOnBash32 TestEngineSkewStartFixtureOnBash32")
	return strings.Join(lines, "\n")
}

func completeHookStartFixtureAssertions() string {
	lines := []string{
		"func TestHookStartDeclaredOutcomeMatrixOnBash32() {",
		"func TestHookStartContextOutcomeShapesOnBash32() {",
		"func TestHookStartIntentionalFullPathFixturesOnBash32() {",
		"func TestHookStartForgedDelegateHintRefusesOnBash32() {",
		"func TestHookStartBrainBootFailureKeepsWaitLineOnBash32() {",
		"func TestHookStartBrainTimeoutKeepsWaitLineOnBash32() {",
		`"HOOK_START_WAIT_MATCHED=1"`,
		`mode: "context-arming-failure"`,
		`mode: "context-holder-read"`,
		`mode: "context-process-identity"`,
		`strings.Contains(traceText, "\nup ") || !strings.Contains(traceText, "\nsession start ")`,
		"assertFullPathContextObject(t, stdout)",
		"if status != statuses[key] || stdout != notices[key] || stderr != expected {}",
	}
	for _, key := range allStartNoticeKeys() {
		lines = append(lines, fmt.Sprintf("  %q: expected,", key))
	}
	for _, key := range allStartIntentionalKeys() {
		lines = append(lines, fmt.Sprintf("  {%q, expected},", key))
	}
	return strings.Join(lines, "\n")
}

func requireHookStartFinding(t *testing.T, source, fixtures, want string) {
	t.Helper()
	findings := auditHookStartSource("scripts/agents/supervision-hook.sh", source, fixtures, completeHookStartFixtureAssertions())
	for _, finding := range findings {
		if strings.Contains(finding.Invariant+" "+finding.Detail, want) {
			return
		}
	}
	t.Fatalf("missing finding containing %q: %#v", want, findings)
}

func TestAuditHookStartExitsAcceptsOwnedBoundary(t *testing.T) {
	findings := auditHookStartSource("scripts/agents/supervision-hook.sh", acceptedHookStartSource(), completeHookStartFixtureLabels(), completeHookStartFixtureAssertions())
	if len(findings) != 0 {
		t.Fatalf("accepted boundary findings: %#v", findings)
	}
}

func TestAuditHookStartExitsAcceptsProductionHook(t *testing.T) {
	findings, err := AuditHookStartExits(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("production boundary findings: %#v", findings)
	}
}

func TestAuditHookStartExitsFailsClosedWhenProofInputIsMissingOrUnreadable(t *testing.T) {
	inputs := []string{
		filepath.Join("scripts", "agents", "supervision-hook.sh"),
		filepath.Join("scripts", "agents", "supervision-hook-fixtures.sh"),
		filepath.Join("internal", "audit", "hookstartexits_test.go"),
	}
	for _, target := range inputs {
		for _, shape := range []string{"missing", "directory"} {
			t.Run(filepath.Base(target)+"/"+shape, func(t *testing.T) {
				root := t.TempDir()
				for _, input := range inputs {
					path := filepath.Join(root, input)
					if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
						t.Fatal(err)
					}
					if input != target {
						if err := os.WriteFile(path, []byte("fixture\n"), 0o644); err != nil {
							t.Fatal(err)
						}
					}
				}
				if shape == "directory" {
					if err := os.Mkdir(filepath.Join(root, target), 0o755); err != nil {
						t.Fatal(err)
					}
				}
				if _, err := AuditHookStartExits(root); err == nil || !strings.Contains(err.Error(), filepath.ToSlash(target)) {
					t.Fatalf("%s %s input did not fail closed by path: %v", shape, target, err)
				}
			})
		}
	}
}

func TestAuditHookStartExitsRejectsMutationBypasses(t *testing.T) {
	base := acceptedHookStartSource()
	fixtures := completeHookStartFixtureLabels()
	tests := []struct {
		name     string
		mutate   func(string) string
		fixtures string
		want     string
	}{
		{"bare exit zero", func(s string) string {
			return strings.Replace(s, "  start_finish notice engine-missing", "  exit 0\n  start_finish notice engine-missing", 1)
		}, fixtures, "start exit ownership"},
		{"tool branch reaches start", func(s string) string {
			return strings.Replace(s, `if [[ "$runtime" == claude && "$event" == tool ]]; then`, `if [[ "$runtime" == claude || "$event" == tool ]]; then`, 1)
		}, fixtures, "start exit ownership"},
		{"bare exit nonzero", func(s string) string {
			return strings.Replace(s, "  start_finish notice engine-missing", "  'exit' 1\n  start_finish notice engine-missing", 1)
		}, fixtures, "start exit ownership"},
		{"prefixed exit", func(s string) string {
			return strings.Replace(s, "  start_finish notice engine-missing", "  command exit 1\n  start_finish notice engine-missing", 1)
		}, fixtures, "start exit ownership"},
		{"escaped exit", func(s string) string {
			return strings.Replace(s, "  start_finish notice engine-missing", "  e\\xit 1\n  start_finish notice engine-missing", 1)
		}, fixtures, "start exit ownership"},
		{"trap replacement", func(s string) string {
			return strings.Replace(s, "  start_finish notice engine-missing", "  trap - EXIT\n  start_finish notice engine-missing", 1)
		}, fixtures, "start trap ownership"},
		{"capture trap replacement", func(s string) string {
			return strings.Replace(s, `  elif captured=$(builtin trap - EXIT HUP INT TERM; "$@"); then`, `  elif captured=$(builtin trap - EXIT HUP INT TERM; "$@"); then
  trap 'bypass' EXIT`, 1)
		}, fixtures, "start trap ownership"},
		{"ignored error", func(s string) string {
			return strings.Replace(s, "  start_finish notice engine-missing", "  false || true\n  start_finish notice engine-missing", 1)
		}, fixtures, "start error consumption"},
		{"dynamic dispatch", func(s string) string {
			return strings.Replace(s, "  start_finish notice engine-missing", "  $runner exit\n  start_finish notice engine-missing", 1)
		}, fixtures, "start dynamic bypass"},
		{"common helper dynamic dispatch", func(s string) string {
			s = strings.Replace(s, "start_main() {", "common_helper() {\n  $runner exit\n}\nstart_main() {", 1)
			return strings.Replace(s, "  start_finish notice engine-missing", "  common_helper\n  start_finish notice engine-missing", 1)
		}, fixtures, "start dynamic bypass"},
		{"unmapped failing command", func(s string) string {
			return strings.Replace(s, "  start_finish notice engine-missing", "  false\n  start_finish notice engine-missing", 1)
		}, fixtures, "outside start_capture"},
		{"direct stdout", func(s string) string {
			return strings.Replace(s, "  start_finish notice engine-missing", "  printf leak\n  start_finish notice engine-missing", 1)
		}, fixtures, "writes directly"},
		{"fallible helper unconsumed", func(s string) string {
			s = strings.Replace(s, "start_main() {", "fallible_helper() {\n  return 1\n}\nstart_main() {", 1)
			return strings.Replace(s, "  start_finish notice engine-missing", "  fallible_helper\n  start_finish notice engine-missing", 1)
		}, fixtures, "fallible helper is invoked directly"},
		{"reachable common helper exit", func(s string) string {
			s = strings.Replace(s, "start_main() {", "common_helper() {\n  exit 9\n}\nstart_main() {", 1)
			return strings.Replace(s, "  start_finish notice engine-missing", "  common_helper\n  start_finish notice engine-missing", 1)
		}, fixtures, "start exit ownership"},
		{"captured common helper exit", func(s string) string {
			s = strings.Replace(s, "start_main() {", "captured_helper() {\n  exit 9\n}\nstart_main() {", 1)
			return strings.Replace(s, "start_capture engine-missing value required tool verb", "start_capture engine-missing value required captured_helper", 1)
		}, fixtures, "start exit ownership"},
		{"exec replacement", func(s string) string {
			return strings.Replace(s, "  start_finish notice engine-missing", "  exec tool\n  start_finish notice engine-missing", 1)
		}, fixtures, "start dynamic bypass"},
		{"eval", func(s string) string {
			return strings.Replace(s, "  start_finish notice engine-missing", "  eval payload\n  start_finish notice engine-missing", 1)
		}, fixtures, "start dynamic bypass"},
		{"source", func(s string) string {
			return strings.Replace(s, "  start_finish notice engine-missing", "  source helper.sh\n  start_finish notice engine-missing", 1)
		}, fixtures, "start dynamic bypass"},
		{"redirected finalizer", func(s string) string {
			return strings.Replace(s, "  start_finish notice engine-missing", "  start_finish notice engine-missing >/dev/null", 1)
		}, fixtures, "start output channel"},
		{"piped finalizer", func(s string) string {
			return strings.Replace(s, "  start_finish notice engine-missing", "  start_finish notice engine-missing | tool", 1)
		}, fixtures, "start finalizer context"},
		{"conditional finalizer", func(s string) string {
			return strings.Replace(s, "  start_finish notice engine-missing", "  if start_finish notice engine-missing; then :; fi", 1)
		}, fixtures, "start finalizer context"},
		{"piped capture", func(s string) string {
			return strings.Replace(s, "start_capture engine-missing value required tool verb", "start_capture engine-missing value required tool verb | tool", 1)
		}, fixtures, "start operation context"},
		{"undeclared capture policy", func(s string) string {
			return strings.Replace(s, "start_capture engine-missing value required tool verb", "start_capture engine-missing value invented tool verb", 1)
		}, fixtures, "status policy must be a declared literal"},
		{"dynamic capture policy", func(s string) string {
			return strings.Replace(s, "start_capture engine-missing value required tool verb", "start_capture engine-missing value $policy tool verb", 1)
		}, fixtures, "status policy must be a declared literal"},
		{"lossy shell string capture", func(s string) string {
			return strings.Replace(s, "start_capture engine-missing value required tool verb", "start_capture engine-missing value required tool json get --shell-safe", 1)
		}, fixtures, "retain trailing newlines"},
		{"undeclared notice", func(s string) string { return strings.Replace(s, "notice engine-missing", "notice invented", 1) }, fixtures, "declared literal"},
		{"dynamic outcome", func(s string) string { return strings.Replace(s, "notice engine-missing", "notice $key", 1) }, fixtures, "declared literal"},
		{"undeclared signal outcome", func(s string) string {
			return strings.Replace(s, "start_signal() {", "start_signal() {\n  start_finish notice invented", 1)
		}, fixtures, "declared literal"},
		{"inconsistent notice object", func(s string) string {
			return strings.Replace(s, `start_notice='{"systemMessage":"notice"}'`, `start_notice='{}'`, 1)
		}, fixtures, "sole systemMessage object"},
		{"early terminal seal", func(s string) string {
			return strings.Replace(s, "  start_finish notice engine-missing", "  start_terminal_complete=true\n  start_finish notice engine-missing", 1)
		}, fixtures, "terminal state"},
		{"forged publication state", func(s string) string {
			return strings.Replace(s, "  start_finish notice engine-missing", "  start_published=true\n  start_finish notice engine-missing", 1)
		}, fixtures, "publication state"},
		{"forged arming state", func(s string) string {
			return strings.Replace(s, "  start_finish notice engine-missing", "  start_arming_started=true\n  start_finish notice engine-missing", 1)
		}, fixtures, "start arming state"},
		{"forged context shape", func(s string) string {
			return strings.Replace(s, "  start_finish notice engine-missing", "  start_context_shape_ready=true\n  start_finish notice engine-missing", 1)
		}, fixtures, "start context shape"},
		{"forged re-arm observation", func(s string) string {
			return strings.Replace(s, "  start_finish notice engine-missing", "  start_arming_rearmed=true\n  start_finish notice engine-missing", 1)
		}, fixtures, "start re-arm observation"},
		{"return from main", func(s string) string {
			return strings.Replace(s, "  start_finish notice engine-missing", "  return 0", 1)
		}, fixtures, "fall-through"},
		{"conditional dispatch", func(s string) string {
			return strings.Replace(s, "  start_main\n  start_finish notice unexpected-termination", "  if start_main; then :; fi\n  start_finish notice unexpected-termination", 1)
		}, fixtures, "must run plainly"},
		{"subshell dispatch", func(s string) string {
			return strings.Replace(s, "  start_main\n  start_finish notice unexpected-termination", "  (start_main)\n  start_finish notice unexpected-termination", 1)
		}, fixtures, "must run plainly"},
		{"missing fixture", func(s string) string { return s }, strings.Replace(fixtures, "# hook-start-case: engine-skew-start outcome=engine-skew\n", "", 1), `"engine-skew" has no discriminating`},
		{"unlaunched fixtures", func(s string) string { return s }, strings.Replace(fixtures, "TestHookStartFailureBranchFixturesOnBash32", "removed-launcher", 1), "not launched through the Go assertion matrix"},
		{"unclassifiable syntax", func(s string) string {
			return strings.Replace(s, "  start_finish notice engine-missing", "  value='unterminated\n  start_finish notice engine-missing", 1)
		}, fixtures, "start unclassifiable syntax"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requireHookStartFinding(t, test.mutate(base), test.fixtures, test.want)
		})
	}
}

func TestAuditHookStartExitsRejectsPostContextFailureBypasses(t *testing.T) {
	base := readProductionHook(t)
	fixtures := completeHookStartFixtureLabels()
	tests := []struct {
		name   string
		mutate func(string) string
		want   string
	}{
		{"forged arming status", func(s string) string {
			return strings.Replace(s, "start_main() {", "start_main() {\n  start_arming_status=0", 1)
		}, "only start_capture may record the checked arming status"},
		{"forged wait-recovery status", func(s string) string {
			return strings.Replace(s, "start_main() {", "start_main() {\n  start_wait_recovery_status=0", 1)
		}, "only start_capture may record the checked wait-recovery status"},
		{"arming stderr dropped", func(s string) string {
			return strings.Replace(s,
				`captured=$(builtin trap - EXIT HUP INT TERM; "$@" 2>&1) && capture_status=0 || capture_status=$?`,
				`captured=$(builtin trap - EXIT HUP INT TERM; "$@") && capture_status=0 || capture_status=$?`, 1)
		}, "post-context arming and wait-recovery failures must become notices"},
		{"arming failure swallowed", func(s string) string {
			return strings.Replace(s,
				`collect_start_notice "Metasystem supervision arming failed: $up_aggregate"`,
				`: "$up_aggregate"`, 1)
		}, "post-context arming and wait-recovery failures must become notices"},
		{"arming failure skips wait recovery", func(s string) string {
			return strings.Replace(s,
				`      collect_start_notice "Metasystem supervision arming failed: $up_aggregate"`,
				"      collect_start_notice \"Metasystem supervision arming failed: $up_aggregate\"\n      start_finish_prepared", 1)
		}, "post-context arming and wait-recovery failures must become notices"},
		{"re-arm evidence swallowed", func(s string) string {
			return strings.Replace(s,
				`collect_start_notice "Metasystem re-armed the rebuilt engine: $up_aggregate"`,
				`: "$up_aggregate"`, 1)
		}, "post-context arming and wait-recovery failures must become notices"},
		{"wait-recovery failure swallowed", func(s string) string {
			return strings.Replace(s,
				`collect_start_notice "Metasystem could not read durable wait recovery rows for this session: $waiting_lines"`,
				`: "$waiting_lines"`, 1)
		}, "post-context arming and wait-recovery failures must become notices"},
		{"brain failure wait recovery removed", func(s string) string {
			return strings.Replace(s,
				`start_capture wait-recovery waiting_lines wait-recovery "$ms" session start --root "$repo" --session "$session"`,
				`: "$waiting_lines"`, 1)
		}, "brain boot failure must become a notice and continue to holder-matched wait recovery"},
		{"brain failure finalized early", func(s string) string {
			return strings.Replace(s, "      start_defer_brain_failure brain-boot\n      return 0", "      start_finish notice brain-boot\n      return 0", 1)
		}, "brain boot failure must become a notice and continue to holder-matched wait recovery"},
		{"prepared context publisher bypassed", func(s string) string {
			return strings.Replace(s, "    start_finish intentional context-ready", "    start_finish intentional healthy-no-context", 1)
		}, "post-context arming and wait-recovery failures must become notices"},
		{"arming policy reused", func(s string) string {
			return strings.Replace(s, "start_capture payload-read payload_probe required", "start_capture payload-read payload_probe arming", 1)
		}, "arming is reserved for start_main"},
		{"wait-recovery policy reused", func(s string) string {
			return strings.Replace(s, "start_capture payload-read payload_probe required", "start_capture payload-read payload_probe wait-recovery", 1)
		}, "wait-recovery is reserved for start_main"},
		{"forged identity-read result", func(s string) string {
			return strings.Replace(s, "start_finish() {", "start_finish() {\n  start_deferred_identity_read=true", 1)
		}, "only start_capture and start_main may record a checked identity-read failure"},
		{"cleared identity-read result", func(s string) string {
			return strings.Replace(s, `  if [[ "$start_deferred_identity_read" == true ]]; then`, "  start_deferred_identity_read=false\n  if [[ \"$start_deferred_identity_read\" == true ]]; then", 1)
		}, "may clear identity-read failure only while preserving a failed process lookup"},
		{"identity-read context swallowed", func(s string) string {
			return strings.Replace(s, `collect_start_notice "$start_holder_context_notice"`, `: "$start_holder_context_notice"`, 1)
		}, "identity-read failure must skip arming but preserve holder-checked wait recovery"},
		{"identity-read wait recovery skipped", func(s string) string {
			return strings.Replace(s, `    collect_start_notice "$start_holder_context_notice"`, "    collect_start_notice \"$start_holder_context_notice\"\n    start_finish_prepared", 1)
		}, "identity-read failure must skip arming but preserve holder-checked wait recovery"},
		{"identity deferral policy reused", func(s string) string {
			return strings.Replace(s, "start_capture payload-read payload_probe required", "start_capture payload-read payload_probe identity-required", 1)
		}, "identity deferral is reserved for start_main"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requireHookStartFinding(t, test.mutate(base), fixtures, test.want)
		})
	}
}

func TestAuditHookStartExitsRejectsMissingFullPathContextShapeProof(t *testing.T) {
	for _, test := range []struct {
		name, required string
	}{
		{"arming failure context case", `mode: "context-arming-failure"`},
		{"process identity recovery case", `mode: "context-process-identity"`},
		{"forged delegate refusal case", "func TestHookStartForgedDelegateHintRefusesOnBash32"},
		{"session start trace assertion", `strings.Contains(traceText, "\nup ") || !strings.Contains(traceText, "\nsession start ")`},
		{"brain boot wait recovery case", "func TestHookStartBrainBootFailureKeepsWaitLineOnBash32"},
		{"brain timeout wait recovery case", "func TestHookStartBrainTimeoutKeepsWaitLineOnBash32"},
		{"brain matched-holder wait row", `"HOOK_START_WAIT_MATCHED=1"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			assertions := strings.Replace(completeHookStartFixtureAssertions(), test.required, "", 1)
			findings := auditHookStartSource("scripts/agents/supervision-hook.sh", acceptedHookStartSource(), completeHookStartFixtureLabels(), assertions)
			for _, finding := range findings {
				if finding.Invariant == "start fixture coverage" && finding.Detail == "declared outcome matrix must execute and assert status, stdout, and stderr" {
					return
				}
			}
			t.Fatalf("missing fixture-coverage finding after deleting %q: %#v", test.required, findings)
		})
	}
	if t.Failed() {
		return
	}
	findings := auditHookStartSource("scripts/agents/supervision-hook.sh", acceptedHookStartSource(), completeHookStartFixtureLabels(), completeHookStartFixtureAssertions())
	for _, finding := range findings {
		if finding.Invariant == "start fixture coverage" {
			t.Fatalf("complete full-path proof was rejected: %#v", findings)
		}
	}
}

func TestAuditHookStartExitsRejectsFixtureLabelWithoutTerminalAssertion(t *testing.T) {
	assertions := strings.Replace(completeHookStartFixtureAssertions(), `  "engine-missing": expected,`+"\n", "", 1)
	findings := auditHookStartSource("hook.sh", acceptedHookStartSource(), completeHookStartFixtureLabels(), assertions)
	for _, finding := range findings {
		if strings.Contains(finding.Detail, `"engine-missing" has a label but no executable terminal assertion`) {
			return
		}
	}
	t.Fatalf("label-only fixture was accepted: %#v", findings)
}

func TestAuditHookStartExitsAcceptsStopOnlyExit(t *testing.T) {
	source := strings.Replace(acceptedHookStartSource(), "start_main() {", "stop_only_helper() {\n  exit 9\n}\nstart_main() {", 1)
	findings := auditHookStartSource("hook.sh", source, completeHookStartFixtureLabels(), completeHookStartFixtureAssertions())
	if len(findings) != 0 {
		t.Fatalf("Stop-only helper changed the start audit: %#v", findings)
	}
}

func TestAuditHookStartExitsReportsLinesAndIgnoresInertExitText(t *testing.T) {
	source := strings.Replace(acceptedHookStartSource(), "  start_finish notice engine-missing", "  printf '%s' 'exit is prose'\n  # exit 9\n  exit 7", 1)
	findings := auditHookStartSource("hook.sh", source, completeHookStartFixtureLabels(), completeHookStartFixtureAssertions())
	for _, finding := range findings {
		if finding.Invariant == "start exit ownership" {
			if finding.Line == 0 || !strings.Contains(finding.String(), fmt.Sprintf("hook.sh:%d", finding.Line)) {
				t.Fatalf("finding omitted physical line: %+v", finding)
			}
			return
		}
	}
	t.Fatalf("executable exit was not reported: %#v", findings)
}

const hookStartFallback = `{"systemMessage":"Metasystem SessionStart could not produce its response: role context delivery is unconfirmed; a declared brain may be uninstructed. Rebuild bin/metasystem with scripts/agents/go-build.sh, then start a new session."}`
const hookStartInterrupted = `{"systemMessage":"Metasystem SessionStart could not finish because it was interrupted: this session received no role context; if this checkout is a declared brain it is uninstructed. Restore the runtime session. Then start a new session."}`

func TestHookStartDeclaredOutcomeMatrixOnBash32(t *testing.T) {
	template := func(cause, remedy string) string {
		return fmt.Sprintf(`{"systemMessage":"Metasystem SessionStart could not %s: this session received no role context; if this checkout is a declared brain it is uninstructed. %s Then start a new session."}`, cause, remedy)
	}
	notices := map[string]string{
		"engine-missing":          `{"systemMessage":"Metasystem engine missing: this session received no role context; if this checkout is a declared brain it is uninstructed until the engine is rebuilt: run scripts/agents/go-build.sh, then start a new session"}`,
		"engine-skew":             `{"systemMessage":"Metasystem engine does not answer path state-root: this session received no role context; if this checkout is a declared brain it is uninstructed. Rebuild bin/metasystem with scripts/agents/go-build.sh, then start a new session."}`,
		"installation-directory":  template("locate its installation directory", "Restore access to the installed hook and its parent directories."),
		"checkout-identification": template("identify the checkout and its primary installation", "Restore Git and access to the checkout and its primary metasystem installation."),
		"resolved-directory":      template("open the resolved installation directory", "Restore access to the installation directory returned by the engine."),
		"installation-validation": template("validate the metasystem installation", "Restore a complete, readable metasystem installation and rebuild bin/metasystem with scripts/agents/go-build.sh."),
		"payload-storage":         template("stage its input", "Restore writable temporary storage and free space."),
		"boot-storage":            template("stage brain context", "Restore writable temporary storage and free space."),
		"start-preparation":       template("prepare the session identity and context", "Restore the installed shell tools and rebuild bin/metasystem with scripts/agents/go-build.sh."),
		"response-rendering":      hookStartFallback,
		"payload-read":            template("read its session input", "Repair the SessionStart hook input and rebuild bin/metasystem with scripts/agents/go-build.sh."),
		"pending-read":            template("read pending steward incidents", "Restore access to the steward incident records and repair unreadable records."),
		"holder-read":             template("read checkout holder identity", "Restore readable checkout custody records and restart the owning runtime."),
		"invocation-invalid":      template("accept the runtime invocation", "Repair the installed hook registration."),
		"runtime-unregistered":    template("find the runtime in its registry", "Repair the installed hook registration."),
		"runtime-registry":        template("read the runtime registry", "Rebuild bin/metasystem with scripts/agents/go-build.sh and restore the installed runtime declarations."),
		"context-contract":        template("read the runtime context contract", "Rebuild bin/metasystem with scripts/agents/go-build.sh and restore the installed runtime declarations."),
		"custody-unreadable":      template("authenticate delegate custody", "Restore the recorded delegate custody and restart through its launcher."),
		"process-identity":        template("identify the owning runtime process", "Restart through the installed runtime launcher."),
		"brain-boot":              `{"systemMessage":"Metasystem brain boot failed: this session received no role context; if this checkout is a declared brain it is uninstructed. Run metasystem brain boot --root <checkout> --repo <checkout> by hand and rebuild if it fails. Then start a new session."}`,
		"brain-timeout":           `{"systemMessage":"Metasystem brain boot failed (timeout): this session received no role context; if this checkout is a declared brain it is uninstructed. Run metasystem brain boot --root <checkout> --repo <checkout> by hand and rebuild if it fails. Then start a new session."}`,
		"arming":                  `{"systemMessage":"Metasystem supervision arming failed: this session received no role context; if this checkout is a declared brain it is uninstructed. Repair supervision from the owning installation and run metasystem up there. Then start a new session."}`,
		"wait-recovery":           template("read durable wait recovery rows", "Repair supervision from the owning installation and run metasystem up there."),
		"temporary-cleanup":       template("remove its temporary files", "Restore temporary-directory access and remove leftover metasystem hook temporary files."),
		"interrupted":             hookStartInterrupted,
		"unexpected-termination":  hookStartFallback,
	}
	statuses := map[string]int{"invocation-invalid": 2, "runtime-unregistered": 2, "custody-unreadable": 1, "interrupted": 143}
	sourceLines := strings.Split(readProductionHook(t), "\n")
	boundary := lineContaining(sourceLines, hookStartBoundary)
	catalog, findings := parseStartOutcomeCatalog("hook.sh", sourceLines[:boundary], shellFunctionRanges(sourceLines[:boundary]))
	if len(findings) != 0 || strings.Join(catalog.keys, "\n") != strings.Join(sortedStrings(allStartOutcomeKeys()), "\n") {
		t.Fatalf("runtime outcome matrix and production catalog differ: keys=%v findings=%v", catalog.keys, findings)
	}
	for _, key := range allStartNoticeKeys() {
		t.Run(key, func(t *testing.T) {
			hook := mutatedStartHook(t, "  start_finish notice "+key+"\n")
			before, err := os.ReadFile(hook)
			if err != nil {
				t.Fatal(err)
			}
			stdout, stderr, status := runHookStart(t, hook, nil, false)
			if status != statuses[key] || stdout != notices[key]+"\n" || stderr != "" {
				t.Fatalf("outcome = status %d stdout %q stderr %q", status, stdout, stderr)
			}
			after, err := os.ReadFile(hook)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("outcome changed its staged durable input: %v", err)
			}
		})
	}

	intentional := []struct {
		key, setup, stdout, stderr string
	}{
		{"authenticated-delegate", "", "", "Metasystem SessionStart intentionally skipped: authenticated delegate; its launcher owns context.\n"},
		{"foreign-runtime", "", "", "Metasystem SessionStart intentionally skipped: another runtime owns this process.\n"},
		{"context-ready", "  start_context_kind=channel\n  start_context_field=hookSpecificOutput.additionalContext\n  start_context_event=SessionStart\n  start_context_payload='role packet'\n  start_context_shape_ready=true\n  start_prepared_context='{\"hookSpecificOutput\":{\"additionalContext\":\"role packet\",\"hookEventName\":\"SessionStart\"}}'\n", `{"hookSpecificOutput":{"additionalContext":"role packet","hookEventName":"SessionStart"}}` + "\n", ""},
		{"screen-context-ready", "  start_context_kind=screen\n  start_notices='screen context'\n", `{"systemMessage":"screen context"}` + "\n", ""},
		{"notices-ready", "  start_notices='notice text'\n", `{"systemMessage":"notice text"}` + "\n", ""},
		{"healthy-no-context", "", "{}\n", ""},
	}
	for _, test := range intentional {
		t.Run(test.key, func(t *testing.T) {
			hook := mutatedStartHook(t, test.setup+"  start_finish intentional "+test.key+"\n")
			stdout, stderr, status := runHookStart(t, hook, nil, false)
			if status != 0 || stdout != test.stdout || stderr != test.stderr {
				t.Fatalf("outcome = status %d stdout %q stderr %q", status, stdout, stderr)
			}
		})
	}
}

func TestHookStartContextOutcomeShapesOnBash32(t *testing.T) {
	tests := []struct {
		name, setup, expected string
	}{
		{
			name: "context-ready",
			setup: `  start_context_kind=channel
  start_context_field=hookSpecificOutput.additionalContext
  start_context_event=SessionStart
  start_context_payload='role packet'
  start_prepared_context='{"hookSpecificOutput":{"additionalContext":"role packet","hookEventName":"SessionStart"}}'
  start_context_shape_ready=true
`,
			expected: `{"hookSpecificOutput":{"additionalContext":"role packet","hookEventName":"SessionStart"}}` + "\n",
		},
		{
			name: "screen-context-ready",
			setup: `  start_context_kind=screen
  start_notices='this runtime has no session-context channel; the packet reached the screen only
role packet'
`,
			expected: `{"systemMessage":"this runtime has no session-context channel; the packet reached the screen only\nrole packet"}` + "\n",
		},
		{
			name: "context-ready-with-notice",
			setup: `  start_context_kind=channel
  start_context_field=hookSpecificOutput.additionalContext
  start_context_event=SessionStart
  start_context_payload='role packet'
  start_prepared_context='{"hookSpecificOutput":{"additionalContext":"role packet","hookEventName":"SessionStart"}}'
  start_context_shape_ready=true
  start_notices='pending notice'
`,
			expected: `{"systemMessage":"pending notice","hookSpecificOutput":{"additionalContext":"role packet","hookEventName":"SessionStart"}}` + "\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			outcome := strings.TrimSuffix(test.name, "-with-notice")
			hook := mutatedStartHook(t, test.setup+"  start_finish intentional "+outcome+"\n")
			stdout, stderr, status := runHookStart(t, hook, nil, false)
			if status != 0 || stdout != test.expected || stderr != "" || strings.Count(stdout, "\n") != 1 {
				t.Fatalf("context outcome = status %d stdout %q stderr %q", status, stdout, stderr)
			}
		})
	}

	t.Run("context-ready-without-event", func(t *testing.T) {
		hook := mutatedStartHook(t, `  start_context_kind=channel
  start_context_field=hookSpecificOutput.additionalContext
  start_context_payload='role packet'
  start_prepared_context='{"hookSpecificOutput":{"additionalContext":"role packet"}}'
  start_finish intentional context-ready
`)
		stdout, stderr, status := runHookStart(t, hook, nil, false)
		if status != 0 || stdout != hookStartFallback+"\n" || stderr != "" {
			t.Fatalf("eventless context outcome = status %d stdout %q stderr %q", status, stdout, stderr)
		}
	})
}

func TestHookStartBuiltinJSONEncoderOnBash32(t *testing.T) {
	setup := `  start_notices=$'quote" slash\\ newline\n tab\t carriage\r back\b form\f utf8 h\xc3\xa9llo'
  start_finish intentional notices-ready
`
	hook := mutatedStartHook(t, setup)
	stdout, stderr, status := runHookStart(t, hook, nil, false)
	message := "quote\" slash\\ newline\n tab\t carriage\r back\b form\f utf8 héllo"
	encoded, err := json.Marshal(map[string]string{"systemMessage": message})
	if err != nil {
		t.Fatal(err)
	}
	if status != 0 || stdout != string(encoded)+"\n" || stderr != "" {
		t.Fatalf("builtin JSON encoding = status %d stdout %q stderr %q", status, stdout, stderr)
	}
}

func sortedStrings(values []string) []string {
	result := append([]string(nil), values...)
	sort.Strings(result)
	return result
}

func TestEngineSkewStartFixtureOnBash32(t *testing.T) {
	variants := []struct {
		name, mode, source string
		declared           bool
		stderr             string
	}{
		{"startup-declared-refusal", "refusal", "startup", true, "metasystem path: unknown verb \"state-root\"\n"},
		{"compact-declared-refusal", "refusal", "compact", true, "metasystem path: unknown verb \"state-root\"\n"},
		{"startup-undeclared-refusal", "refusal", "startup", false, "metasystem path: unknown verb \"state-root\"\n"},
		{"compact-undeclared-refusal", "refusal", "compact", false, "metasystem path: unknown verb \"state-root\"\n"},
		{"empty-success", "empty", "startup", false, ""},
		{"lf-success", "lf", "startup", false, ""},
		{"cr-success", "cr", "startup", false, ""},
	}
	for _, test := range variants {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			hook := filepath.Join(root, "scripts", "agents", "supervision-hook.sh")
			writeExecutable(t, hook, readProductionHook(t))
			trace := filepath.Join(t.TempDir(), "trace")
			engine := `#!/usr/bin/env bash
printf '%s\n' "$*" >>"${HOOK_START_TRACE:?}"
if [[ ${1-} == runtime && ${2-} == list ]]; then
  printf '%s\n' claude
  exit 0
fi
if [[ ${1-} == path && ${2-} == state-root ]]; then
  case "${HOOK_START_ENGINE_MODE:?}" in
    refusal) printf '%s\n' 'metasystem path: unknown verb "state-root"' >&2; exit 2 ;;
    empty) exit 0 ;;
    lf) printf '%s\n%s\n' "$3" extra; exit 0 ;;
    cr) printf '%s\r\n' "$3"; exit 0 ;;
  esac
fi
exit 91
`
			// The installation must carry its own executable even though the
			// deliberately older engine runs the turn through the supported
			// override. This distinguishes provenance validation from execution.
			writeExecutable(t, filepath.Join(root, "bin", "metasystem"), "#!/usr/bin/env bash\nexit 92\n")
			override := filepath.Join(t.TempDir(), "metasystem")
			writeExecutable(t, override, engine)
			if test.declared {
				brainPath := filepath.Join(root, "artifacts", "agents", "brain.json")
				if err := os.MkdirAll(filepath.Dir(brainPath), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(brainPath, []byte("{\"declared\":true}\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			command := exec.Command("git", "init", "-q", "-b", "main", root)
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("git init: %v: %s", err, output)
			}
			before := snapshotFixtureTree(t, root)
			payload := fmt.Sprintf("{\"session_id\":\"engine-skew-%s\",\"source\":%q}\n", test.name, test.source)
			stdout, stderr, status := runHookStartCommand(t, hook, "claude", payload, []string{
				"HOOK_START_TRACE=" + trace,
				"HOOK_START_ENGINE_MODE=" + test.mode,
				"METASYSTEM_BIN=" + override,
			}, false)
			if status != 0 || stdout != `{"systemMessage":"Metasystem engine does not answer path state-root: this session received no role context; if this checkout is a declared brain it is uninstructed. Rebuild bin/metasystem with scripts/agents/go-build.sh, then start a new session."}`+"\n" || stderr != test.stderr {
				t.Fatalf("engine skew = status %d stdout %q stderr %q", status, stdout, stderr)
			}
			traceBytes, err := os.ReadFile(trace)
			if err != nil {
				t.Fatal(err)
			}
			physicalRoot, err := filepath.EvalSymlinks(root)
			if err != nil {
				t.Fatal(err)
			}
			wantTrace := "runtime list\npath state-root " + physicalRoot + "\n"
			if string(traceBytes) != wantTrace {
				t.Fatalf("resolver trace = %q, want %q", traceBytes, wantTrace)
			}
			if after := snapshotFixtureTree(t, root); after != before {
				t.Fatalf("engine skew changed durable state\nbefore:\n%s\nafter:\n%s", before, after)
			}
		})
	}
}

func TestHookStartIntentionalFullPathFixturesOnBash32(t *testing.T) {
	tests := []struct {
		mode, stdout, stderr  string
		context, acknowledged bool
		skipsArming           bool
	}{
		{mode: "healthy", stdout: "{}\n"},
		{mode: "notices", stdout: `{"systemMessage":"Steward incidents pending: incident"}` + "\n"},
		{mode: "context", stdout: `{"hookSpecificOutput":{"additionalContext":"role packet","hookEventName":"SessionStart"}}` + "\n", context: true, acknowledged: true},
		{mode: "context-trailing-newline", stdout: `{"hookSpecificOutput":{"additionalContext":"role packet\n","hookEventName":"SessionStart"}}` + "\n", context: true, acknowledged: true},
		{mode: "context-arming-failure", stdout: `{"systemMessage":"Metasystem supervision arming failed: up outcome=ENROLLMENT_DRIFT component=accepted-engine remedy=\"restart fixture\"\nWAIT RECOVERY fixture row","hookSpecificOutput":{"additionalContext":"role packet","hookEventName":"SessionStart"}}` + "\n", context: true, acknowledged: true},
		{mode: "context-arming-rearmed", stdout: `{"systemMessage":"Metasystem supervision arming failed: up outcome=failed re-armed=\"generation=9 previous=8\" component=steward-runner\nMetasystem re-armed the rebuilt engine: up outcome=failed re-armed=\"generation=9 previous=8\" component=steward-runner","hookSpecificOutput":{"additionalContext":"role packet","hookEventName":"SessionStart"}}` + "\n", context: true, acknowledged: true},
		{mode: "context-pending", stdout: `{"systemMessage":"Steward incidents pending: incident","hookSpecificOutput":{"additionalContext":"role packet","hookEventName":"SessionStart"}}` + "\n", context: true, acknowledged: true},
		{mode: "context-holder-read", stdout: `{"systemMessage":"Metasystem supervision could not identify the immediate claude agent process; arming was refused.","hookSpecificOutput":{"additionalContext":"role packet","hookEventName":"SessionStart"}}` + "\n", context: true, acknowledged: true, skipsArming: true},
		{mode: "context-process-identity", stdout: `{"systemMessage":"Metasystem supervision could not identify the immediate claude agent process; arming was refused.\nWAIT RECOVERY fixture row","hookSpecificOutput":{"additionalContext":"role packet","hookEventName":"SessionStart"}}` + "\n", context: true, acknowledged: true, skipsArming: true},
		{mode: "context-wait-failure", stdout: `{"systemMessage":"Metasystem could not read durable wait recovery rows for this session: first failure; second failure","hookSpecificOutput":{"additionalContext":"role packet","hookEventName":"SessionStart"}}` + "\n", context: true, acknowledged: true},
		{mode: "context-wait-rows", stdout: `{"systemMessage":"WAIT RECOVERY fixture row","hookSpecificOutput":{"additionalContext":"role packet","hookEventName":"SessionStart"}}` + "\n", context: true, acknowledged: true},
		{mode: "context-digest", stdout: `{"hookSpecificOutput":{"additionalContext":"role packet\nNARRATOR DIGEST warning","hookEventName":"SessionStart"}}` + "\n", context: true, acknowledged: true},
		{mode: "context-corrupt", stdout: `{"hookSpecificOutput":{"additionalContext":"BRAIN SEAT warning\nrole packet","hookEventName":"SessionStart"}}` + "\n", context: true},
		{mode: "screen", stdout: `{"systemMessage":"this runtime has no session-context channel; the packet reached the screen only\nrole packet"}` + "\n"},
		{mode: "screen-holder-read", stdout: `{"systemMessage":"this runtime has no session-context channel; the packet reached the screen only\nrole packet\nMetasystem supervision could not identify the immediate claude agent process; arming was refused."}` + "\n", acknowledged: true, skipsArming: true},
		{mode: "screen-arming-failure", stdout: `{"systemMessage":"this runtime has no session-context channel; the packet reached the screen only\nrole packet\nMetasystem supervision arming failed: up outcome=failed component=accepted-engine"}` + "\n"},
		{mode: "rearm-success", stdout: `{"systemMessage":"Metasystem re-armed the rebuilt engine: up outcome=armed authority=writer re-armed=\"generation=9 previous=8\""}` + "\n"},
		{mode: "delegate", stderr: "Metasystem SessionStart intentionally skipped: authenticated delegate; its launcher owns context.\n"},
		{mode: "foreign", stderr: "Metasystem SessionStart intentionally skipped: another runtime owns this process.\n"},
	}
	for _, test := range tests {
		t.Run(test.mode, func(t *testing.T) {
			root := t.TempDir()
			hook := filepath.Join(root, "scripts", "agents", "supervision-hook.sh")
			writeExecutable(t, hook, readProductionHook(t))
			trace := filepath.Join(t.TempDir(), "trace")
			writeExecutable(t, filepath.Join(root, "bin", "metasystem"), fullPathFixtureEngine())
			command := exec.Command("git", "init", "-q", "-b", "main", root)
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("git init: %v: %s", err, output)
			}
			before := snapshotFixtureTree(t, root)
			stdout, stderr, status := runHookStartCommand(t, hook, "claude", `{"session_id":"full-path","source":"startup"}`+"\n", []string{
				"HOOK_START_TRACE=" + trace,
				"HOOK_START_FULL_MODE=" + test.mode,
			}, false)
			if status != 0 || stdout != test.stdout || stderr != test.stderr {
				t.Fatalf("full path = status %d stdout %q stderr %q", status, stdout, stderr)
			}
			traceBytes, err := os.ReadFile(trace)
			if err != nil {
				t.Fatal(err)
			}
			traceText := string(traceBytes)
			if test.mode == "delegate" {
				if strings.Contains(traceText, " up ") || strings.Contains(traceText, "brain boot") {
					t.Fatalf("delegate skip continued into preparation: %s", traceText)
				}
			} else if test.mode == "foreign" {
				if strings.Contains(traceText, "brain boot") || strings.Contains(traceText, " up ") {
					t.Fatalf("foreign skip continued into role preparation: %s", traceText)
				}
			} else if test.skipsArming {
				if strings.Contains(traceText, "\nup ") || !strings.Contains(traceText, "\nsession start ") {
					t.Fatalf("identity failure did not skip arming and preserve wait recovery: %s", traceText)
				}
			} else {
				if !strings.Contains(traceText, "\nup ") || !strings.Contains(traceText, "\nsession start ") {
					t.Fatalf("successful completion skipped arming or wait recovery: %s", traceText)
				}
			}
			if test.acknowledged && !strings.Contains(traceText, "\nbrain start-delivered ") {
				t.Fatalf("published declared context was not acknowledged: %s", traceText)
			}
			if test.context {
				assertFullPathContextObject(t, stdout)
			}
			if after := snapshotFixtureTree(t, root); after != before {
				t.Fatalf("fake-engine full path changed durable state\nbefore:\n%s\nafter:\n%s", before, after)
			}
		})
	}
}

func TestHookStartForgedDelegateHintRefusesOnBash32(t *testing.T) {
	root := t.TempDir()
	hook := filepath.Join(root, "scripts", "agents", "supervision-hook.sh")
	writeExecutable(t, hook, readProductionHook(t))
	trace := filepath.Join(t.TempDir(), "trace")
	writeExecutable(t, filepath.Join(root, "bin", "metasystem"), fullPathFixtureEngine())
	command := exec.Command("git", "init", "-q", "-b", "main", root)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	before := snapshotFixtureTree(t, root)
	stdout, stderr, status := runHookStartCommand(t, hook, "claude", "{}\n", []string{
		"HOOK_START_TRACE=" + trace,
		"HOOK_START_FULL_MODE=healthy",
		"METASYSTEM_HOOK_DELEGATE_STATE_ROOT=" + root,
		"METASYSTEM_HOOK_DELEGATE_INSTALLATION_ROOT=" + root,
		"METASYSTEM_HOOK_DELEGATE_JOB=job-forged",
	}, false)
	wantStdout, wantStatus := directNoticeOutcome(t, "custody-unreadable")
	if status != wantStatus || stdout != wantStdout || stderr != "" {
		t.Fatalf("forged delegate hint = status %d stdout %q stderr %q; want status %d stdout %q", status, stdout, stderr, wantStatus, wantStdout)
	}
	traceBytes, err := os.ReadFile(trace)
	if err != nil {
		t.Fatal(err)
	}
	traceText := string(traceBytes)
	if strings.Count(traceText, "lease hook-delegate ") != 1 || strings.Contains(traceText, "\nruntime list") ||
		strings.Contains(traceText, "\nup ") || strings.Contains(traceText, "\nsession start ") {
		t.Fatalf("forged delegate hint continued after custody refusal: %s", traceText)
	}
	if after := snapshotFixtureTree(t, root); after != before {
		t.Fatalf("forged delegate hint changed durable state\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func assertFullPathContextObject(t *testing.T, stdout string) {
	t.Helper()
	var object map[string]json.RawMessage
	if err := json.Unmarshal([]byte(strings.TrimSuffix(stdout, "\n")), &object); err != nil {
		t.Fatalf("full start path did not publish one JSON object: %v: %q", err, stdout)
	}
	var context struct {
		AdditionalContext string `json:"additionalContext"`
		HookEventName     string `json:"hookEventName"`
	}
	if err := json.Unmarshal(object["hookSpecificOutput"], &context); err != nil {
		t.Fatalf("full start path omitted hookSpecificOutput: %v: %q", err, stdout)
	}
	if context.HookEventName != "SessionStart" || context.AdditionalContext == "" {
		t.Fatalf("full start path context shape = event %q context %q", context.HookEventName, context.AdditionalContext)
	}
}

func TestHookStartFailureBranchFixturesOnBash32(t *testing.T) {
	tests := []struct {
		key, mode, runtime, mutation string
		gitRoot, engine              bool
		environment                  []string
	}{
		{"engine-missing", "healthy", "claude", "", true, false, nil},
		{"installation-directory", "healthy", "claude", `  source_parent=/metasystem-fixture-directory-does-not-exist
`, true, true, nil},
		{"checkout-identification", "healthy", "claude", "", false, true, nil},
		{"resolved-directory", "resolved-directory", "claude", "", true, true, nil},
		{"installation-validation", "installation-validation", "claude", "", true, true, nil},
		{"payload-storage", "healthy", "claude", "replace-payload-storage", true, true, nil},
		{"boot-storage", "healthy", "claude", "replace-boot-storage", true, true, nil},
		{"start-preparation", "start-preparation", "claude", "", true, true, nil},
		{"response-rendering", "response-rendering", "claude", "", true, true, nil},
		{"payload-read", "payload-read", "claude", "", true, true, nil},
		{"pending-read", "pending-read", "claude", "", true, true, nil},
		{"holder-read", "holder-read", "claude", "", true, true, nil},
		{"invocation-invalid", "healthy", "Bad", "", true, true, nil},
		{"runtime-unregistered", "runtime-unregistered", "claude", "", true, true, nil},
		{"runtime-registry", "runtime-registry", "claude", "", true, true, nil},
		{"context-contract", "context-contract", "claude", "", true, true, nil},
		{"custody-unreadable", "healthy", "claude", "", true, true, []string{"METASYSTEM_HOOK_DELEGATE_STATE_ROOT=incomplete"}},
		{"process-identity", "process-identity", "claude", "", true, true, nil},
		{"brain-boot", "brain-boot", "claude", "", true, true, nil},
		{"brain-timeout", "brain-timeout", "claude", "", true, true, []string{"METASYSTEM_BRAIN_BOOT_DEADLINE_MS=1"}},
		{"arming", "arming", "claude", "", true, true, nil},
		{"arming", "arming-rearmed", "claude", "", true, true, nil},
		{"wait-recovery", "wait-recovery", "claude", "", true, true, nil},
		{"temporary-cleanup", "payload-read", "claude", "replace-payload-cleanup", true, true, nil},
	}
	for _, test := range tests {
		t.Run(test.key, func(t *testing.T) {
			root := t.TempDir()
			source := readProductionHook(t)
			switch test.mutation {
			case "replace-payload-storage":
				source = strings.Replace(source,
					`start_capture payload-storage start_payload required-nonempty command mktemp "${TMPDIR:-/tmp}/metasystem-supervision-hook.XXXXXX"`,
					`start_capture payload-storage start_payload required-nonempty false`, 1)
			case "replace-boot-storage":
				source = strings.Replace(source,
					`start_capture boot-storage start_boot_dir required-nonempty command mktemp -d "${TMPDIR:-/tmp}/metasystem-brain-boot-hook.XXXXXX"`,
					`start_capture boot-storage start_boot_dir required-nonempty false`, 1)
			case "replace-payload-cleanup":
				source = strings.Replace(source, `if ! command rm -f "$start_payload" 2>/dev/null; then`, `if ! false; then`, 1)
			default:
				if test.mutation != "" {
					source = strings.Replace(source, "  start_capture installation-directory script_dir", test.mutation+"  start_capture installation-directory script_dir", 1)
				}
			}
			hook := filepath.Join(root, "scripts", "agents", "supervision-hook.sh")
			writeExecutable(t, hook, source)
			trace := filepath.Join(t.TempDir(), "trace")
			enginePath := filepath.Join(root, "bin", "metasystem")
			if test.engine {
				writeExecutable(t, enginePath, fullPathFixtureEngine())
			}
			if test.gitRoot {
				command := exec.Command("git", "init", "-q", "-b", "main", root)
				if output, err := command.CombinedOutput(); err != nil {
					t.Fatalf("git init: %v: %s", err, output)
				}
			}
			before := snapshotFixtureTree(t, root)
			environment := append([]string{
				"HOOK_START_TRACE=" + trace,
				"HOOK_START_FULL_MODE=" + test.mode,
			}, test.environment...)
			if test.key == "brain-timeout" {
				shimDir := t.TempDir()
				writeExecutable(t, filepath.Join(shimDir, "ps"), "#!/bin/bash\nprintf '%s\\n' \"${HOOK_START_PS_COMMAND:?}\"\n")
				environment = append(environment, "PATH="+shimDir+":"+os.Getenv("PATH"), "HOOK_START_PS_COMMAND="+enginePath)
			}
			stdout, stderr, status := runHookStartCommand(t, hook, test.runtime, "{}\n", environment, false)
			wantStdout, wantStatus := directNoticeOutcome(t, test.key)
			if test.key == "arming" && test.mode == "arming" {
				wantStdout = `{"systemMessage":"Metasystem supervision arming failed: up outcome=failed component=accepted-engine remedy=\"restart fixture\""}` + "\n"
			}
			if test.key == "arming" && test.mode == "arming-rearmed" {
				wantStdout = `{"systemMessage":"Metasystem supervision arming failed: up outcome=failed re-armed=\"generation=9 previous=8\" component=steward-runner\nMetasystem re-armed the rebuilt engine: up outcome=failed re-armed=\"generation=9 previous=8\" component=steward-runner"}` + "\n"
			}
			if test.key == "wait-recovery" {
				wantStdout = `{"systemMessage":"Metasystem could not read durable wait recovery rows for this session: first failure; second failure"}` + "\n"
			}
			if test.key == "holder-read" || test.key == "process-identity" {
				wantStdout = `{"systemMessage":"Metasystem supervision could not identify the immediate claude agent process; arming was refused."}` + "\n"
			}
			if test.key == "brain-boot" || test.key == "brain-timeout" {
				wantStdout = appendSystemMessage(wantStdout, "Supervision may have been partly initialized.")
			}
			if test.key == "temporary-cleanup" {
				wantStdout, wantStatus = directNoticeOutcome(t, "payload-read")
				wantStdout = appendSystemMessage(wantStdout, "Metasystem SessionStart also could not remove its temporary files. Restore temporary-directory access and remove leftover metasystem hook temporary files.")
			}
			if status != wantStatus || stdout != wantStdout || stderr != "" {
				t.Fatalf("branch = status %d stdout %q stderr %q; want status %d stdout %q", status, stdout, stderr, wantStatus, wantStdout)
			}
			if test.key == "brain-boot" || test.key == "brain-timeout" {
				traceBytes, err := os.ReadFile(trace)
				if err != nil || !strings.Contains(string(traceBytes), "\nup ") || !strings.Contains(string(traceBytes), "\nsession start ") {
					t.Fatalf("brain failure skipped supervision arming or holder-matched wait recovery: %v: %s", err, traceBytes)
				}
			}
			if after := snapshotFixtureTree(t, root); after != before {
				t.Fatalf("failure branch changed durable state\nbefore:\n%s\nafter:\n%s", before, after)
			}
		})
	}
}

func TestHookStartBrainBootFailureKeepsWaitLineOnBash32(t *testing.T) {
	assertHookStartBrainFailureKeepsWaitLineOnBash32(t, "brain-boot", nil)
}

func TestHookStartBrainTimeoutKeepsWaitLineOnBash32(t *testing.T) {
	assertHookStartBrainFailureKeepsWaitLineOnBash32(t, "brain-timeout", []string{"METASYSTEM_BRAIN_BOOT_DEADLINE_MS=1"})
}

func assertHookStartBrainFailureKeepsWaitLineOnBash32(t *testing.T, mode string, environment []string) {
	t.Helper()
	root := t.TempDir()
	hook := filepath.Join(root, "scripts", "agents", "supervision-hook.sh")
	writeExecutable(t, hook, readProductionHook(t))
	trace := filepath.Join(t.TempDir(), "trace")
	enginePath := filepath.Join(root, "bin", "metasystem")
	writeExecutable(t, enginePath, fullPathFixtureEngine())
	command := exec.Command("git", "init", "-q", "-b", "main", root)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	before := snapshotFixtureTree(t, root)
	environment = append([]string{
		"HOOK_START_TRACE=" + trace,
		"HOOK_START_FULL_MODE=" + mode,
		"HOOK_START_WAIT_MATCHED=1",
	}, environment...)
	if mode == "brain-timeout" {
		shimDir := t.TempDir()
		writeExecutable(t, filepath.Join(shimDir, "ps"), "#!/bin/bash\nprintf '%s\\n' \"${HOOK_START_PS_COMMAND:?}\"\n")
		environment = append(environment, "PATH="+shimDir+":"+os.Getenv("PATH"), "HOOK_START_PS_COMMAND="+enginePath)
	}
	stdout, stderr, status := runHookStartCommand(t, hook, "claude", `{"session_id":"brain-wait","source":"startup"}`+"\n", environment, false)
	wantStdout, wantStatus := directNoticeOutcome(t, mode)
	wantStdout = appendSystemMessage(wantStdout, "WAIT RECOVERY fixture row")
	wantStdout = appendSystemMessage(wantStdout, "Supervision may have been partly initialized.")
	if status != wantStatus || stdout != wantStdout || stderr != "" {
		t.Fatalf("brain failure = status %d stdout %q stderr %q; want status %d stdout %q", status, stdout, stderr, wantStatus, wantStdout)
	}
	traceBytes, err := os.ReadFile(trace)
	if err != nil {
		t.Fatal(err)
	}
	traceText := string(traceBytes)
	brainAt := strings.Index(traceText, "brain boot ")
	upAt := strings.Index(traceText, "\nup ")
	waitAt := strings.Index(traceText, "\nsession start ")
	if brainAt < 0 || upAt < brainAt || waitAt < upAt {
		t.Fatalf("brain failure did not reach arming and holder-matched wait recovery in order: %s", traceText)
	}
	if after := snapshotFixtureTree(t, root); after != before {
		t.Fatalf("brain-failure wait fixture changed fake durable state\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestHookStartPostPreparationFixturesOnBash32(t *testing.T) {
	bookkeeping := "Metasystem SessionStart published its response but could not finish delivery bookkeeping. Repair supervision from the owning installation, then start a new session; context may repeat.\n"
	tests := []struct {
		name, mode, mutation, stdout, stderr string
		status                               int
	}{
		{
			name: "acknowledgment-failure", mode: "acknowledgment-failure",
			stdout: `{"hookSpecificOutput":{"additionalContext":"role packet","hookEventName":"SessionStart"}}` + "\n",
			stderr: bookkeeping, status: 1,
		},
		{
			name: "cleanup-after-publication", mode: "healthy", mutation: "cleanup",
			stdout: "{}\n", stderr: bookkeeping, status: 1,
		},
		{
			name: "signal-after-publication", mode: "signal-after-publication",
			stdout: `{"hookSpecificOutput":{"additionalContext":"role packet","hookEventName":"SessionStart"}}` + "\n",
			stderr: bookkeeping, status: 1,
		},
		{
			name: "signal-after-arming", mode: "healthy", mutation: "signal",
			stdout: appendSystemMessage(hookStartInterrupted+"\n", "Supervision may have been partly initialized."), status: 143,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			source := readProductionHook(t)
			switch test.mutation {
			case "cleanup":
				source = strings.Replace(source, `if ! command rm -f "$start_payload" 2>/dev/null; then`, `if ! false; then`, 1)
			case "signal":
				source = strings.Replace(source,
					`  start_capture wait-recovery waiting_lines wait-recovery "$ms" session start --root "$repo" --session "$session"`,
					"  builtin kill -TERM \"$$\"\n  start_capture wait-recovery waiting_lines wait-recovery \"$ms\" session start --root \"$repo\" --session \"$session\"", 1)
			}
			hook := filepath.Join(root, "scripts", "agents", "supervision-hook.sh")
			writeExecutable(t, hook, source)
			writeExecutable(t, filepath.Join(root, "bin", "metasystem"), fullPathFixtureEngine())
			command := exec.Command("git", "init", "-q", "-b", "main", root)
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("git init: %v: %s", err, output)
			}
			before := snapshotFixtureTree(t, root)
			trace := filepath.Join(t.TempDir(), "trace")
			stdout, stderr, status := runHookStartCommand(t, hook, "claude", "{}\n", []string{
				"HOOK_START_TRACE=" + trace,
				"HOOK_START_FULL_MODE=" + test.mode,
			}, false)
			if status != test.status || stdout != test.stdout || stderr != test.stderr {
				t.Fatalf("post-preparation = status %d stdout %q stderr %q", status, stdout, stderr)
			}
			if after := snapshotFixtureTree(t, root); after != before {
				t.Fatalf("post-preparation fixture changed fake durable state\nbefore:\n%s\nafter:\n%s", before, after)
			}
		})
	}
}

func directNoticeOutcome(t *testing.T, key string) (string, int) {
	t.Helper()
	hook := mutatedStartHook(t, "  start_finish notice "+key+"\n")
	stdout, stderr, status := runHookStart(t, hook, nil, false)
	if stderr != "" {
		t.Fatalf("direct %s notice wrote stderr: %q", key, stderr)
	}
	return stdout, status
}

func appendSystemMessage(object, line string) string {
	object = strings.TrimSuffix(object, "\n")
	object = strings.TrimSuffix(object, `"}`)
	return object + `\n` + line + `"}` + "\n"
}

func fullPathFixtureEngine() string {
	return `#!/usr/bin/env bash
printf '%s\n' "$*" >>"${HOOK_START_TRACE:?}"
mode=${HOOK_START_FULL_MODE:?}
if [[ ${1-} == runtime && ${2-} == list ]]; then
  [[ $mode != runtime-registry ]] || exit 7
  if [[ $mode == runtime-unregistered ]]; then printf '%s\n' fake; else printf '%s\n' claude; fi
  exit 0
fi
if [[ ${1-} == path && ${2-} == state-root ]]; then
  [[ $mode != installation-validation ]] || exit 1
  if [[ $mode == resolved-directory ]]; then printf '%s\n' "$3/disappeared"; else printf '%s\n' "$3"; fi
  exit 0
fi
if [[ ${1-} == lease && ${2-} == hook-delegate ]]; then
  if [[ $mode == delegate ]]; then printf '%s\n' '{"delegate":true,"jobId":"fixture","matchedPid":1,"comparisonMode":"legacy-seconds"}'; exit 0; fi
  exit 3
fi
if [[ ${1-} == proc && ${2-} == find-ancestor ]]; then
  [[ $mode != process-identity && $mode != context-process-identity ]] || exit 7
  [[ $mode != holder-read && $mode != context-holder-read && $mode != screen-holder-read ]] || exit 0
  printf '%s\n' '{"identity":true}'
  exit 0
fi
if [[ ${1-} == lease && ${2-} == classify ]]; then exit 7; fi
if [[ ${1-} == runtime && ${2-} == start-context ]]; then
  [[ $mode != context-contract ]] || { printf '%s\n' malformed; exit 0; }
  case "$mode" in
    context|context-*|response-rendering|acknowledgment-failure|signal-after-publication)
      printf '%s\n' 'field=hookSpecificOutput.additionalContext event=SessionStart bytes=2048 sources=startup,compact'
      exit 0 ;;
    *) exit 1 ;;
  esac
fi
if [[ ${1-} == steward && ${2-} == pending ]]; then
  [[ $mode != pending-read ]] || exit 7
  [[ $mode != notices && $mode != context-pending ]] || printf '%s\n' incident
  exit 0
fi
if [[ ${1-} == brain && ${2-} == boot ]]; then
  [[ $mode != brain-boot ]] || exit 7
  if [[ $mode == brain-timeout ]]; then sleep 10; exit 0; fi
  case "$mode" in
    context-trailing-newline)
      printf '%s\n' '{"declared":true,"state":"declared","payload":"role packet\n","bytes":12,"sections":{"asks":"complete","held":"complete","fleet":"complete","digest":"complete"},"digestEmitted":false,"digestCursor":0,"digestPrefixSha256":"","declarationSha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}' ;;
    context-digest)
      printf '%s\n' '{"declared":true,"state":"declared","payload":"role packet\nNARRATOR DIGEST warning","bytes":35,"sections":{"asks":"complete","held":"complete","fleet":"complete","digest":"complete"},"digestEmitted":true,"digestCursor":7,"digestPrefixSha256":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","declarationSha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}' ;;
    context-corrupt)
      printf '%s\n' '{"declared":true,"state":"corrupt","payload":"BRAIN SEAT warning\nrole packet","bytes":30,"sections":{"asks":"error","held":"skipped","fleet":"skipped","digest":"skipped"},"digestEmitted":false,"digestCursor":0,"digestPrefixSha256":"","declarationSha256":""}' ;;
    context|context-*|screen|screen-holder-read|screen-arming-failure|response-rendering|acknowledgment-failure|signal-after-publication)
      printf '%s\n' '{"declared":true,"state":"declared","payload":"role packet","bytes":11,"sections":{"asks":"complete","held":"complete","fleet":"complete","digest":"complete"},"digestEmitted":false,"digestCursor":0,"digestPrefixSha256":"","declarationSha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}' ;;
    *) printf '%s\n' '{"declared":false}' ;;
  esac
  exit 0
fi
if [[ ${1-} == json && ${2-} == strip ]]; then
  [[ $mode != payload-read ]] || exit 7
  printf '%s\n' '{}'
  exit 0
fi
if [[ ${1-} == json && ${2-} == object ]]; then
  [[ $mode != response-rendering ]] || exit 7
  pair=$3; key=${pair%%=*}; value=${pair#*=}
  if [[ $mode == context-trailing-newline && $key == additionalContext ]]; then
    printf '%s\n' '{"additionalContext":"role packet\n"}'
  elif [[ $mode == context-digest && $key == additionalContext ]]; then
    printf '%s\n' '{"additionalContext":"role packet\nNARRATOR DIGEST warning"}'
  elif [[ $mode == context-corrupt && $key == additionalContext ]]; then
    printf '%s\n' '{"additionalContext":"BRAIN SEAT warning\nrole packet"}'
  else
    printf '{"%s":"%s"}\n' "$key" "$value"
  fi
  exit 0
fi
if [[ ${1-} == json && ${2-} == get ]]; then
  field=
  previous=
  for argument in "$@"; do
    if [[ $previous == --field ]]; then field=$argument; break; fi
    previous=$argument
  done
  case "$field" in
    session_id) if [[ $mode == start-preparation ]]; then printf 'valid\n'; fi ;;
    runtime) if [[ $mode == foreign ]]; then printf fake; else printf claude; fi ;;
    pid|pidStartedAt) printf 1 ;;
    declared) case "$mode" in context|context-*|screen|screen-holder-read|screen-arming-failure|response-rendering|acknowledgment-failure|signal-after-publication) printf true ;; *) printf false ;; esac ;;
    state) if [[ $mode == context-corrupt ]]; then printf corrupt; else printf declared; fi ;;
    bytes) if [[ $mode == context-trailing-newline ]]; then printf 12; elif [[ $mode == context-digest ]]; then printf 35; elif [[ $mode == context-corrupt ]]; then printf 30; else printf 11; fi ;;
    digestEmitted) if [[ $mode == context-digest ]]; then printf true; else printf false; fi ;;
    digestCursor) if [[ $mode == context-digest ]]; then printf 7; else printf 0; fi ;;
    digestPrefixSha256) if [[ $mode == context-digest ]]; then printf bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb; else exit 3; fi ;;
    declarationSha256) if [[ $mode == context-corrupt ]]; then exit 3; else printf aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa; fi ;;
    payload|hookSpecificOutput.additionalContext)
      if [[ $mode == context-trailing-newline ]]; then
        printf 'role packet\n'
      elif [[ $mode == context-digest ]]; then
        printf 'role packet\nNARRATOR DIGEST warning'
      elif [[ $mode == context-corrupt ]]; then
        printf 'BRAIN SEAT warning\nrole packet'
      else
        printf 'role packet'
      fi ;;
    hookSpecificOutput.hookEventName) printf SessionStart ;;
    sections.asks|sections.held|sections.fleet|sections.digest) printf complete ;;
    *) exit 1 ;;
  esac
  exit 0
fi
if [[ ${1-} == util && ${2-} == sha256 ]]; then exit 7; fi
if [[ ${1-} == up ]]; then
  if [[ $mode == rearm-success ]]; then
    printf '%s\n' 'up outcome=armed authority=writer re-armed="generation=9 previous=8"'
    exit 0
  fi
  if [[ $mode == arming-rearmed ]]; then
    printf '%s\n' 'up outcome=failed re-armed="generation=9 previous=8" component=steward-runner'
    exit 7
  fi
  if [[ $mode == context-arming-rearmed ]]; then
    printf '%s\n' 'up outcome=failed re-armed="generation=9 previous=8" component=steward-runner'
    exit 7
  fi
  if [[ $mode == arming ]]; then
    printf '%s\n' 'up outcome=failed component=accepted-engine remedy="restart fixture"'
    exit 7
  fi
  if [[ $mode == context-arming-failure ]]; then
    printf '%s\n' 'up outcome=ENROLLMENT_DRIFT component=accepted-engine remedy="restart fixture"'
    exit 7
  fi
  if [[ $mode == screen-arming-failure ]]; then
    printf '%s\n' 'up outcome=failed component=accepted-engine' >&2
    exit 7
  fi
  exit 0
fi
if [[ ${1-} == session && ${2-} == start ]]; then
  if [[ $mode == wait-recovery || $mode == context-wait-failure ]]; then
    printf '%s\n' 'first failure' >&2
    printf '%s\n' 'second failure' >&2
    exit 7
  fi
  if [[ $mode == context-wait-rows || $mode == context-process-identity || $mode == context-arming-failure ||
        ( ${HOOK_START_WAIT_MATCHED:-0} == 1 && ( $mode == brain-boot || $mode == brain-timeout ) ) ]]; then
    printf '%s\n' 'WAIT RECOVERY fixture row'
    exit 0
  fi
  exit 64
fi
if [[ ${1-} == brain && ${2-} == start-delivered ]]; then
  [[ $mode != acknowledgment-failure ]] || exit 7
  if [[ $mode == signal-after-publication ]]; then kill -TERM "$PPID"; sleep 0.1; fi
  exit 0
fi
exit 91
`
}

func TestHookStartExitTrapReportsUnexpectedTerminationOnBash32(t *testing.T) {
	for _, test := range []struct {
		name     string
		injected string
	}{
		{"exit zero", "  exit 0\n"},
		{"exit nonzero", "  exit 7\n"},
		{"return", "  return 0\n"},
		{"failed command", "  false\n"},
		{"failed substitution", "  failed=$(false)\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			hook := mutatedStartHook(t, test.injected)
			before := snapshotHookInstallation(t, hook)
			stdout, _, status := runHookStart(t, hook, nil, false)
			if status != 0 || stdout != hookStartFallback+"\n" {
				t.Fatalf("unexpected termination = status %d stdout %q", status, stdout)
			}
			if after := snapshotHookInstallation(t, hook); after != before {
				t.Fatalf("unexpected termination changed durable state\nbefore:\n%s\nafter:\n%s", before, after)
			}
		})
	}
}

func TestHookStartSignalsUseOwnedStatusesOnBash32(t *testing.T) {
	for _, signal := range []struct {
		name   string
		value  string
		status int
	}{{"HUP", "HUP", 129}, {"INT", "INT", 130}, {"TERM", "TERM", 143}} {
		t.Run(signal.name, func(t *testing.T) {
			hook := mutatedStartHook(t, "  kill -"+signal.value+" $$\n")
			before := snapshotHookInstallation(t, hook)
			stdout, _, status := runHookStart(t, hook, nil, false)
			if status != signal.status || stdout != hookStartInterrupted+"\n" {
				t.Fatalf("signal = status %d stdout %q", status, stdout)
			}
			if after := snapshotHookInstallation(t, hook); after != before {
				t.Fatalf("signal changed durable state\nbefore:\n%s\nafter:\n%s", before, after)
			}
		})
	}
	t.Run("TERM while finalizing", func(t *testing.T) {
		source := readProductionHook(t)
		needle := "  builtin trap 'start_defer_signal 143' TERM\n"
		source = strings.Replace(source, needle, needle+"  builtin kill -TERM \"$$\"\n", 1)
		hook := mutatedStartHookSource(t, source, "  start_finish intentional healthy-no-context\n")
		before := snapshotHookInstallation(t, hook)
		stdout, stderr, status := runHookStart(t, hook, nil, false)
		if status != 143 || stdout != hookStartInterrupted+"\n" || stderr != "" {
			t.Fatalf("deferred signal = status %d stdout %q stderr %q", status, stdout, stderr)
		}
		if after := snapshotHookInstallation(t, hook); after != before {
			t.Fatalf("deferred signal changed durable state\nbefore:\n%s\nafter:\n%s", before, after)
		}
	})
}

func TestHookStartLastResortUsesStderrWhenStdoutIsClosed(t *testing.T) {
	hook := mutatedStartHook(t, "  start_finish notice response-rendering\n")
	before := snapshotHookInstallation(t, hook)
	stdout, stderr, status := runHookStart(t, hook, nil, true)
	if status != 74 || stdout != "" || stderr != hookStartFallback+"\n" {
		t.Fatalf("closed stdout = status %d stdout %q stderr %q", status, stdout, stderr)
	}
	if after := snapshotHookInstallation(t, hook); after != before {
		t.Fatalf("closed stdout changed durable state\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestHookStartBash32ArrayCanaryCoversEmptyAndPopulatedValues(t *testing.T) {
	script := `set -u
empty=()
populated=(alpha beta)
for value in "${empty[@]+"${empty[@]}"}"; do printf 'empty=%s\n' "$value"; done
for value in "${populated[@]+"${populated[@]}"}"; do printf 'populated=%s\n' "$value"; done
`
	command := exec.Command("/bin/bash", "-c", script)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("Bash 3.2 array canary failed: %v: %s", err, output)
	}
	if string(output) != "populated=alpha\npopulated=beta\n" {
		t.Fatalf("Bash 3.2 array output = %q", output)
	}
}

func mutatedStartHook(t *testing.T, injected string) string {
	t.Helper()
	return mutatedStartHookSource(t, readProductionHook(t), injected)
}

func mutatedStartHookSource(t *testing.T, source, injected string) string {
	t.Helper()
	needle := "start_main() {\n"
	if !strings.Contains(source, needle) {
		t.Fatal("production hook has no start_main")
	}
	source = strings.Replace(source, needle, needle+injected, 1)
	root := t.TempDir()
	hook := filepath.Join(root, "scripts", "agents", "supervision-hook.sh")
	writeExecutable(t, hook, source)
	return hook
}

func readProductionHook(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "scripts", "agents", "supervision-hook.sh"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func writeExecutable(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
}

func runHookStart(t *testing.T, hook string, extraEnv []string, stdoutClosed bool) (string, string, int) {
	return runHookStartCommand(t, hook, "fake", "{}\n", extraEnv, stdoutClosed)
}

func runHookStartCommand(t *testing.T, hook, runtime, input string, extraEnv []string, stdoutClosed bool) (string, string, int) {
	t.Helper()
	var command *exec.Cmd
	if stdoutClosed {
		command = exec.Command("/bin/bash", "-c", `exec 1>&-; exec /bin/bash "$1" "$2" start`, "hook-start-test", hook, runtime)
	} else {
		command = exec.Command("/bin/bash", hook, runtime, "start")
	}
	command.Stdin = strings.NewReader(input)
	command.Env = withoutEnvironment(os.Environ(), "METASYSTEM_BIN")
	command.Env = append(command.Env, extraEnv...)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	status := 0
	if err != nil {
		var exitError *exec.ExitError
		if !strings.Contains(err.Error(), "exit status") || !errors.As(err, &exitError) {
			t.Fatalf("run hook: %v", err)
		}
		status = exitError.ExitCode()
	}
	return stdout.String(), stderr.String(), status
}

func snapshotFixtureTree(t *testing.T, root string) string {
	t.Helper()
	var snapshot strings.Builder
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil || relative == "." || info.IsDir() {
			return err
		}
		if relative == filepath.Join("artifacts", "agents", "context", "engine-path") {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		fmt.Fprintf(&snapshot, "%s %s %x\n", relative, info.Mode(), content)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot.String()
}

func snapshotHookInstallation(t *testing.T, hook string) string {
	t.Helper()
	return snapshotFixtureTree(t, filepath.Clean(filepath.Join(filepath.Dir(hook), "..", "..")))
}

func withoutEnvironment(environment []string, name string) []string {
	prefix := name + "="
	filtered := environment[:0]
	for _, entry := range environment {
		if !strings.HasPrefix(entry, prefix) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func allStartNoticeKeys() []string {
	return []string{
		"engine-missing", "engine-skew", "installation-directory", "checkout-identification",
		"resolved-directory", "installation-validation", "payload-storage", "boot-storage",
		"start-preparation", "response-rendering", "payload-read", "pending-read", "holder-read",
		"invocation-invalid", "runtime-unregistered", "runtime-registry", "context-contract",
		"custody-unreadable", "process-identity", "brain-boot", "brain-timeout", "arming",
		"wait-recovery", "temporary-cleanup", "interrupted", "unexpected-termination",
	}
}

func allStartOutcomeKeys() []string {
	keys := append([]string(nil), allStartNoticeKeys()...)
	return append(keys, allStartIntentionalKeys()...)
}

func allStartIntentionalKeys() []string {
	return []string{"authenticated-delegate", "foreign-runtime", "context-ready", "screen-context-ready", "notices-ready", "healthy-no-context"}
}
