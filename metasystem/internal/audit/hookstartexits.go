package audit

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const hookStartBoundary = "# SessionStart cannot pass this dispatcher."

// HookStartExitFinding names a start-reachable construct that can bypass the
// SessionStart outcome owner or an outcome that has no executed fixture.
type HookStartExitFinding struct {
	Path      string
	Line      int
	Invariant string
	Detail    string
}

func (finding HookStartExitFinding) String() string {
	return fmt.Sprintf("%s: %s:%d: %s", finding.Invariant, finding.Path, finding.Line, finding.Detail)
}

type shellFunctionRange struct {
	name       string
	start, end int
}

var functionStartPattern = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)\(\) \{`)
var startFinishCallPattern = regexp.MustCompile(`\bstart_finish[[:space:]]+(notice|intentional)[[:space:]]+([^[:space:];|&]+)`)
var startCaptureCallPattern = regexp.MustCompile(`^[[:space:]]*start_capture[[:space:]]+([^[:space:]]+)[[:space:]]+[^[:space:]]+[[:space:]]+([^[:space:]]+)`)
var startCaptureArgvPattern = regexp.MustCompile(`^[[:space:]]*start_capture[[:space:]]+[^[:space:]]+[[:space:]]+[^[:space:]]+[[:space:]]+[^[:space:]]+[[:space:]]+(?:command[[:space:]]+)?([A-Za-z_][A-Za-z0-9_]*)`)
var startFixturePattern = regexp.MustCompile(`^[[:space:]]*#[[:space:]]*hook-start-case:[[:space:]]*([a-z0-9-]+)(?:[[:space:]]+outcome=([a-z0-9-]+))?[[:space:]]*$`)
var startCatalogKeyPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// AuditHookStartExits audits the complete start-reachable prefix of the hook.
// The explicit dispatcher is the cut: SessionStart must terminate inside it,
// so later Stop/end/receipt code is unreachable and remains under its existing
// contracts.
func AuditHookStartExits(root string) ([]HookStartExitFinding, error) {
	hookPath := filepath.Join(root, "scripts", "agents", "supervision-hook.sh")
	data, err := os.ReadFile(hookPath)
	if err != nil {
		return nil, fmt.Errorf("hook start exit audit could not read %s: %w", hookPath, err)
	}
	fixturePath := filepath.Join(root, "scripts", "agents", "supervision-hook-fixtures.sh")
	fixtureData, err := os.ReadFile(fixturePath)
	if err != nil {
		return nil, fmt.Errorf("hook start exit audit could not read %s: %w", fixturePath, err)
	}
	assertionPath := filepath.Join(root, "internal", "audit", "hookstartexits_test.go")
	assertionData, err := os.ReadFile(assertionPath)
	if err != nil {
		return nil, fmt.Errorf("hook start exit audit could not read %s: %w", assertionPath, err)
	}
	return auditHookStartSource(filepath.ToSlash(filepath.Join("scripts", "agents", "supervision-hook.sh")), string(data), string(fixtureData), string(assertionData)), nil
}

func auditHookStartSource(path, source, fixtures, fixtureAssertions string) []HookStartExitFinding {
	lines := strings.Split(source, "\n")
	boundaryLine := lineContaining(lines, hookStartBoundary)
	if boundaryLine == 0 {
		return []HookStartExitFinding{{path, 1, "start dispatcher", "exclusive non-start boundary is missing"}}
	}
	prefixLines := lines[:boundaryLine]
	prefix := strings.Join(prefixLines, "\n")
	ranges := shellFunctionRanges(prefixLines)
	reachable := startReachableFunctions(prefixLines, ranges)
	functions := map[string]bool{}
	fallibleFunctions := map[string]bool{}
	for _, functionRange := range ranges {
		functions[functionRange.name] = true
		for _, line := range prefixLines[functionRange.start-1 : functionRange.end] {
			trimmed := strings.TrimSpace(line)
			if regexp.MustCompile(`^return[[:space:]]+[1-9][0-9]*([[:space:];]|$)`).MatchString(trimmed) || trimmed == "false" {
				fallibleFunctions[functionRange.name] = true
			}
		}
	}
	ownerAt := func(line int) string {
		for _, candidate := range ranges {
			if line >= candidate.start && line <= candidate.end {
				return candidate.name
			}
		}
		return ""
	}
	var findings []HookStartExitFinding
	add := func(line int, invariant, detail string) {
		findings = append(findings, HookStartExitFinding{path, line, invariant, detail})
	}
	catalog, catalogFindings := parseStartOutcomeCatalog(path, prefixLines, ranges)
	findings = append(findings, catalogFindings...)
	commands, scanIssues := shellScan(prefix)
	for _, issue := range scanIssues {
		add(issue.Line, "start unclassifiable syntax", issue.Detail)
	}

	lastResort := lineContaining(prefixLines, "start_last_resort=")
	exitTrap := lineContaining(prefixLines, `builtin trap 'start_exit_trap "$?"' EXIT`)
	hupTrap, intTrap, termTrap := exitTrap+1, exitTrap+2, exitTrap+3
	entrySignalTraps := exitTrap > 0 && termTrap <= len(prefixLines) &&
		strings.TrimSpace(prefixLines[hupTrap-1]) == `builtin trap 'start_signal 129' HUP` &&
		strings.TrimSpace(prefixLines[intTrap-1]) == `builtin trap 'start_signal 130' INT` &&
		strings.TrimSpace(prefixLines[termTrap-1]) == `builtin trap 'start_signal 143' TERM`
	strictMode := lineContaining(prefixLines, "set -euo pipefail")
	dispatch := lastLineContaining(prefixLines, `if [[ "$event" == start ]]; then`)
	if lastResort == 0 || exitTrap == 0 || !entrySignalTraps || strictMode == 0 || dispatch == 0 || !(lastResort < exitTrap && termTrap < strictMode && strictMode < dispatch) {
		add(1, "start initialization", "last-resort bytes and EXIT trap must be initialized before strict mode and dispatch")
	}
	armingStatusInitialization := lineContaining(prefixLines, "start_arming_status=0")
	waitStatusInitialization := lineContaining(prefixLines, "start_wait_recovery_status=0")
	armingStatusAssignments := countTrimmedLine(prefixLines, "start_arming_status=$capture_status")
	waitStatusAssignments := countTrimmedLine(prefixLines, "start_wait_recovery_status=$capture_status")
	if armingStatusInitialization == 0 || waitStatusInitialization == 0 || armingStatusAssignments != 1 || waitStatusAssignments != 1 {
		add(1, "start post-context status", "arming and wait-recovery status must each be initialized and recorded exactly once by start_capture")
	}
	postContextCapture := "if [[ \"$policy\" == arming || \"$policy\" == wait-recovery ]]; then\n" +
		"    captured=$(builtin trap - EXIT HUP INT TERM; \"$@\" 2>&1) && capture_status=0 || capture_status=$?"
	armingCapture := `start_capture arming up_output arming start_up "$runtime" "$session" "$identity_pid" "$identity_started"`
	waitCapture := `start_capture wait-recovery waiting_lines wait-recovery "$ms" session start --root "$repo" --session "$session"`
	armingFailureHandling := "if (( start_arming_status != 0 )); then\n" +
		"      collect_start_notice \"Metasystem supervision arming failed: $up_aggregate\"\n" +
		"      if [[ \"$up_aggregate\" == *' re-armed='* ]]; then\n" +
		"        collect_start_notice \"Metasystem re-armed the rebuilt engine: $up_aggregate\"\n" +
		"      fi\n" +
		"    elif [[ \"$up_aggregate\" == *' re-armed='* ]]; then\n" +
		"      collect_start_notice \"Metasystem re-armed the rebuilt engine: $up_aggregate\"\n" +
		"    fi"
	waitFailureHandling := "elif (( start_wait_recovery_status != 64 )); then\n" +
		"    waiting_lines=${waiting_lines//$'\\n'/'; '}\n" +
		"    collect_start_notice \"Metasystem could not read durable wait recovery rows for this session: $waiting_lines\""
	preparedPublisher := "start_finish_prepared() {\n" +
		"  if [[ \"$start_brain_failure\" == brain-boot ]]; then\n" +
		"    start_finish notice brain-boot\n" +
		"  elif [[ \"$start_brain_failure\" == brain-timeout ]]; then\n" +
		"    start_finish notice brain-timeout\n" +
		"  elif [[ \"$start_context_kind\" == channel ]]; then\n" +
		"    start_finish intentional context-ready\n" +
		"  elif [[ \"$start_context_kind\" == screen ]]; then\n" +
		"    start_finish intentional screen-context-ready\n" +
		"  elif [[ -n \"$start_notices\" ]]; then\n" +
		"    start_finish intentional notices-ready\n" +
		"  else\n" +
		"    start_finish intentional healthy-no-context\n" +
		"  fi\n" +
		"}"
	preparedCallAt := strings.LastIndex(prefix, "\n  start_finish_prepared\n")
	waitCaptureAt := strings.Index(prefix, waitCapture)
	if !strings.Contains(prefix, postContextCapture) || countTrimmedLine(prefixLines, armingCapture) != 1 ||
		countTrimmedLine(prefixLines, waitCapture) != 1 || !strings.Contains(prefix, armingFailureHandling) ||
		!strings.Contains(prefix, waitFailureHandling) || !strings.Contains(prefix, preparedPublisher) ||
		countTrimmedLine(prefixLines, "start_finish_prepared") != 1 || preparedCallAt < waitCaptureAt {
		add(dispatch, "start post-context status", "post-context arming and wait-recovery failures must become notices before prepared-context publication")
	}
	identityStateInitialization := lineContaining(prefixLines, "start_deferred_identity_read=false")
	identityFailureHandling := "if [[ \"$start_deferred_identity_read\" == true ]]; then\n" +
		"    collect_start_notice \"$start_holder_context_notice\"\n" +
		"  else\n" +
		"    " + armingCapture
	identityFailureAt := strings.Index(prefix, identityFailureHandling)
	armingCaptureAt := strings.Index(prefix, armingCapture)
	if identityStateInitialization == 0 || identityFailureAt < 0 || armingCaptureAt < 0 || identityFailureAt > armingCaptureAt ||
		waitCaptureAt < armingCaptureAt || preparedCallAt < waitCaptureAt {
		add(1, "start deferred identity result", "identity-read failure must skip arming but preserve holder-checked wait recovery before publishing prepared context")
	}
	brainFailureInitialization := lineContaining(prefixLines, "start_brain_failure=")
	brainPrepareAt := strings.Index(prefix, "\n  start_prepare_brain\n")
	brainFailureDeferral := "start_defer_brain_failure() {\n" +
		"  case ${1-} in\n" +
		"    brain-boot)\n" +
		"      start_brain_failure=brain-boot"
	brainTimeoutDeferral := "brain-timeout)\n" +
		"      start_brain_failure=brain-timeout"
	captureDeferral := `if start_capture_defer_brain_failure "$failure_key"; then return 0; fi`
	brainNoticeRelay := "if [[ ( \"$key\" == brain-boot || \"$key\" == brain-timeout ) && -n \"$start_notices\" ]]; then\n" +
		"        start_json_escape \"$start_notices\"\n" +
		"        start_append_notice_text \"$response\" \"$start_escaped\""
	if brainFailureInitialization == 0 || brainPrepareAt < 0 || waitCaptureAt < brainPrepareAt || preparedCallAt < waitCaptureAt ||
		!strings.Contains(prefix, brainFailureDeferral) || !strings.Contains(prefix, brainTimeoutDeferral) ||
		!strings.Contains(prefix, captureDeferral) || !strings.Contains(prefix, brainNoticeRelay) ||
		countTrimmedLine(prefixLines, "start_finish notice brain-boot") != 1 ||
		countTrimmedLine(prefixLines, "start_finish notice brain-timeout") != 1 {
		add(1, "start brain wait recovery", "brain boot failure must become a notice and continue to holder-matched wait recovery before prepared publication")
	}
	wantDispatcher := "if [[ \"$event\" == start ]]; then\n  start_main\n  start_finish notice unexpected-termination\nfi\n" + hookStartBoundary
	if !strings.Contains(prefix+"\n"+hookStartBoundary, wantDispatcher) {
		add(dispatch, "start dispatcher", "start_main must run plainly and be followed by the unexpected-termination finalizer")
	}

	for _, command := range commands {
		owner := ownerAt(command.Line)
		if owner != "" && !reachable[owner] {
			continue
		}
		word := filepath.Base(command.Word)
		switch word {
		case "exit":
			if owner != "start_finish" && owner != "start_emergency" {
				add(command.Line, "start exit ownership", "start-reachable exit is outside start_finish")
			}
		case "exec", "eval", "source", ".":
			add(command.Line, "start dynamic bypass", "dynamic parent-shell replacement or sourcing is forbidden")
		case "trap":
			line := strings.TrimSpace(prefixLines[command.Line-1])
			captureTrap := owner == "start_capture" && (strings.Contains(line, `captured=$(builtin trap - EXIT HUP INT TERM; "$@")`) ||
				strings.Contains(line, `captured=$(builtin trap - EXIT HUP INT TERM; "$@" 2>&1)`))
			ownerClear := (owner == "start_finish" || owner == "start_emergency") && line == "builtin trap - EXIT HUP INT TERM"
			ownerSignal := owner == "start_finish" && (line == "builtin trap 'start_defer_signal 129' HUP" ||
				line == "builtin trap 'start_defer_signal 130' INT" || line == "builtin trap 'start_defer_signal 143' TERM" ||
				line == "builtin trap 'start_signal 129' HUP" || line == "builtin trap 'start_signal 130' INT" ||
				line == "builtin trap 'start_signal 143' TERM")
			entryTrap := command.Line == exitTrap || command.Line == exitTrap+1 || command.Line == exitTrap+2 || command.Line == exitTrap+3
			if !captureTrap && !ownerClear && !ownerSignal && !entryTrap {
				add(command.Line, "start trap ownership", "start-reachable code replaces an outcome trap")
			}
		}
	}

	allowedIgnoredOwners := map[string]bool{"start_cleanup": true, "start_emergency": true, "start_finish": true, "start_preserve_boot_stderr": true}
	startCapturePolicies := map[string]bool{
		"required": true, "required-nonempty": true, "shell-required": true, "shell-required-nonempty": true,
		"arming": true, "one-line": true, "state-root": true, "json-optional-field": true,
		"delegate-custody": true, "context-channel": true, "wait-recovery": true, "allow-empty": true,
		"identity-optional": true, "identity-required": true, "identity-required-nonempty": true, "identity-shell-required": true,
	}
	for index, line := range prefixLines {
		lineNumber := index + 1
		owner := ownerAt(lineNumber)
		if owner != "" && !reachable[owner] {
			continue
		}
		trimmed := strings.TrimSpace(line)
		previous := ""
		if index > 0 {
			previous = strings.TrimSpace(prefixLines[index-1])
		}
		continued := strings.HasSuffix(previous, `\`) || strings.HasSuffix(previous, "&&") || strings.HasSuffix(previous, "||")
		if owner == "start_main" && !continued {
			if word := startMainLeadingCommand(trimmed); word != "" {
				if word == "printf" {
					add(lineNumber, "start output channel", "start_main writes directly instead of publishing through start_finish")
				}
				if fallibleFunctions[word] {
					add(lineNumber, "start operation mapping", "fallible helper is invoked directly instead of through start_capture")
				} else if !functions[word] && !startMainBuiltin(word) {
					add(lineNumber, "start operation mapping", "external or fallible command is outside start_capture")
				}
			}
		}
		if (strings.Contains(line, "|| true") || strings.Contains(line, "|| :")) && !allowedIgnoredOwners[owner] {
			add(lineNumber, "start error consumption", "ignored status is outside the outcome owner or cleanup")
		}
		if owner == "start_main" && (trimmed == "return" || strings.HasPrefix(trimmed, "return ")) {
			add(lineNumber, "start fall-through", "start_main may not return")
		}
		if strings.Contains(line, "start_terminal_complete=") && lineNumber != lineContaining(prefixLines, "start_terminal_complete=false") && owner != "start_finish" && owner != "start_emergency" {
			add(lineNumber, "start terminal state", "only start_finish may seal completion")
		}
		if strings.Contains(line, "start_published=") && lineNumber != lineContaining(prefixLines, "start_published=false") && owner != "start_finish" {
			add(lineNumber, "start publication state", "only start_finish may record successful output publication")
		}
		if strings.Contains(line, "start_arming_started=") && lineNumber != lineContaining(prefixLines, "start_arming_started=false") && owner != "start_capture" {
			add(lineNumber, "start arming state", "only start_capture may mark arming immediately before up")
		}
		if strings.Contains(line, "start_context_shape_ready=") && lineNumber != lineContaining(prefixLines, "start_context_shape_ready=false") && owner != "start_prepare_context_object" {
			add(lineNumber, "start context shape", "only the context renderer may validate a publishable context object")
		}
		if strings.Contains(line, "start_arming_rearmed=") && lineNumber != lineContaining(prefixLines, "start_arming_rearmed=false") && owner != "start_capture" {
			add(lineNumber, "start re-arm observation", "only start_capture may record the checked arming result")
		}
		if strings.Contains(line, "start_arming_status=") && lineNumber != armingStatusInitialization && owner != "start_capture" {
			add(lineNumber, "start post-context status", "only start_capture may record the checked arming status")
		}
		if strings.Contains(line, "start_wait_recovery_status=") && lineNumber != waitStatusInitialization && owner != "start_capture" {
			add(lineNumber, "start post-context status", "only start_capture may record the checked wait-recovery status")
		}
		if strings.Contains(line, "start_deferred_identity_read=") && lineNumber != identityStateInitialization && owner != "start_capture" && owner != "start_main" {
			add(lineNumber, "start deferred identity result", "only start_capture and start_main may record a checked identity-read failure")
		}
		if strings.Contains(line, "start_brain_failure=") && lineNumber != brainFailureInitialization && owner != "start_defer_brain_failure" {
			add(lineNumber, "start brain wait recovery", "only the deferred brain-failure owner may record a brain outcome that continues to wait recovery")
		}
		if trimmed == "start_deferred_identity_read=false" && lineNumber != identityStateInitialization &&
			(owner != "start_main" || previous != "process_identity_failed=$start_deferred_identity_read") {
			add(lineNumber, "start deferred identity result", "start_main may clear identity-read failure only while preserving a failed process lookup across holder fallback")
		}
		if finishAt := strings.Index(line, "start_finish"); finishAt >= 0 && strings.ContainsAny(line[finishAt:], "><") && owner != "start_finish" {
			add(lineNumber, "start output channel", "caller redirects finalizer output")
		}
		if finishAt := strings.Index(line, "start_finish"); finishAt >= 0 && owner != "start_finish" && owner != "start_capture" && owner != "start_exit_trap" && owner != "start_signal" {
			before := strings.TrimSpace(line[:finishAt])
			after := line[finishAt+len("start_finish"):]
			if before == "if" || before == "!" || strings.HasPrefix(before, "if ") || strings.HasPrefix(before, "! ") || strings.Contains(before, "$(") || strings.ContainsAny(after, "|&`") {
				add(lineNumber, "start finalizer context", "start_finish is conditional, substituted, backgrounded, or piped")
			}
		}
		if captureAt := strings.Index(line, "start_capture"); captureAt >= 0 && owner != "start_capture" {
			after := line[captureAt+len("start_capture"):]
			if strings.ContainsAny(after, "|&`") {
				add(lineNumber, "start operation context", "start_capture is backgrounded, substituted, or piped")
			}
		}
		if match := startFinishCallPattern.FindStringSubmatch(line); match != nil && owner != "start_finish" && owner != "start_capture" {
			key := strings.Trim(match[2], `"'`)
			if strings.HasPrefix(key, "$") || !catalog.contains(match[1], key) {
				add(lineNumber, "start declared outcome", "start_finish outcome key must be a declared literal")
			}
		}
		if match := startCaptureCallPattern.FindStringSubmatch(line); match != nil {
			key := strings.Trim(match[1], `"'`)
			if strings.HasPrefix(key, "$") || !catalog.contains("notice", key) {
				add(lineNumber, "start operation mapping", "start_capture failure key must be a declared notice literal")
			}
			policy := strings.Trim(match[2], `"'`)
			if strings.HasPrefix(policy, "$") || !startCapturePolicies[policy] {
				add(lineNumber, "start operation mapping", "start_capture status policy must be a declared literal")
			}
			if policy == "arming" && (owner != "start_main" || key != "arming" || !strings.Contains(line, " start_up ")) {
				add(lineNumber, "start operation mapping", "arming is reserved for start_main's checked post-context arming call")
			}
			if policy == "wait-recovery" && (owner != "start_main" || key != "wait-recovery" || !strings.Contains(line, ` "$ms" session start `)) {
				add(lineNumber, "start operation mapping", "wait-recovery is reserved for start_main's checked post-context session-start call")
			}
			if strings.HasPrefix(policy, "identity-") && (owner != "start_main" || (key != "holder-read" && key != "process-identity")) {
				add(lineNumber, "start operation mapping", "identity deferral is reserved for start_main's checked process-identity and holder-read calls")
			}
		}
		if strings.Contains(line, "--shell-safe") && !strings.Contains(line+"\n"+previous, "start_json_value_with_sentinel") {
			add(lineNumber, "start operation mapping", "shell-safe output must retain trailing newlines through the sentinel boundary")
		}
		if variable := startLeadingCommandVariable(trimmed); variable != "" && !continued && !knownStartCommandVariable(owner, variable) {
			add(lineNumber, "start dynamic bypass", "dynamic command dispatch is outside the checked operation boundary")
		}
	}

	fixtureKeys := map[string]bool{}
	if !strings.Contains(fixtures, "TestHookStartDeclaredOutcomeMatrixOnBash32") ||
		!strings.Contains(fixtures, "TestHookStartFailureBranchFixturesOnBash32") ||
		!strings.Contains(fixtures, "TestEngineSkewStartFixtureOnBash32") ||
		!strings.Contains(fixtures, "TestHookStartContextOutcomeShapesOnBash32") ||
		!strings.Contains(fixtures, "TestHookStartIntentionalFullPathFixturesOnBash32") {
		add(1, "start fixture coverage", "hook start labels are not launched through the Go assertion matrix")
	}
	for _, line := range strings.Split(fixtures, "\n") {
		if match := startFixturePattern.FindStringSubmatch(line); match != nil {
			key := match[1]
			if match[2] != "" {
				key = match[2]
			}
			if fixtureKeys[key] {
				add(1, "start fixture coverage", fmt.Sprintf("catalog outcome %q has duplicate hook-start cases", key))
			}
			fixtureKeys[key] = true
		}
	}
	for _, key := range catalog.keys {
		if !fixtureKeys[key] {
			add(lineContaining(prefixLines, key+")"), "start fixture coverage", fmt.Sprintf("catalog outcome %q has no discriminating hook-start-case", key))
		}
	}
	for key := range fixtureKeys {
		if !catalog.notice[key] && !catalog.intentional[key] {
			add(1, "start fixture coverage", fmt.Sprintf("hook-start case names undeclared outcome %q", key))
		}
	}
	if !declaresTestFunc(fixtureAssertions, "TestHookStartDeclaredOutcomeMatrixOnBash32") ||
		!declaresTestFunc(fixtureAssertions, "TestHookStartContextOutcomeShapesOnBash32") ||
		!declaresTestFunc(fixtureAssertions, "TestHookStartIntentionalFullPathFixturesOnBash32") ||
		!declaresTestFunc(fixtureAssertions, "TestHookStartForgedDelegateHintRefusesOnBash32") ||
		!declaresTestFunc(fixtureAssertions, "TestHookStartBrainBootFailureKeepsWaitLineOnBash32") ||
		!declaresTestFunc(fixtureAssertions, "TestHookStartBrainTimeoutKeepsWaitLineOnBash32") ||
		!strings.Contains(fixtureAssertions, `"HOOK_START_WAIT_MATCHED=1"`) ||
		!strings.Contains(fixtureAssertions, `mode: "context-arming-failure"`) ||
		!strings.Contains(fixtureAssertions, `mode: "context-holder-read"`) ||
		!strings.Contains(fixtureAssertions, `mode: "context-process-identity"`) ||
		!strings.Contains(fixtureAssertions, `strings.Contains(traceText, "\nup ") || !strings.Contains(traceText, "\nsession start ")`) ||
		!strings.Contains(fixtureAssertions, "assertFullPathContextObject(t, stdout)") ||
		!strings.Contains(fixtureAssertions, "status != statuses[key]") ||
		!strings.Contains(fixtureAssertions, "stdout != notices[key]") ||
		!strings.Contains(fixtureAssertions, "stderr !=") {
		add(1, "start fixture coverage", "declared outcome matrix must execute and assert status, stdout, and stderr")
	}
	for _, key := range catalog.keys {
		mapEntry := regexp.MustCompile(`(?m)^[[:space:]]*"` + regexp.QuoteMeta(key) + `":[[:space:]]`).MatchString(fixtureAssertions)
		tableEntry := regexp.MustCompile(`(?m)^[[:space:]]*\{"` + regexp.QuoteMeta(key) + `",`).MatchString(fixtureAssertions)
		if !mapEntry && !tableEntry {
			add(lineContaining(prefixLines, key+")"), "start fixture coverage", fmt.Sprintf("catalog outcome %q has a label but no executable terminal assertion", key))
		}
	}

	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Line != findings[j].Line {
			return findings[i].Line < findings[j].Line
		}
		return findings[i].Invariant+findings[i].Detail < findings[j].Invariant+findings[j].Detail
	})
	return findings
}

func knownStartCommandVariable(owner, word string) bool {
	switch word {
	case "$ms", "${ms}":
		return owner == "start_prepare_brain" || owner == "start_finish" || owner == "start_up" || owner == "start_hash_text"
	case "$@", "${@}":
		return owner == "start_capture" || owner == "start_json_value_with_sentinel"
	default:
		return false
	}
}

var startLeadingCommandVariablePattern = regexp.MustCompile(`^"?(\$(?:[A-Za-z_][A-Za-z0-9_]*|@)|\$\{(?:[A-Za-z_][A-Za-z0-9_]*|@)\})"?(?:[[:space:]]|$)`)

func startLeadingCommandVariable(line string) string {
	match := startLeadingCommandVariablePattern.FindStringSubmatch(line)
	if match == nil {
		return ""
	}
	return match[1]
}

func startMainBuiltin(word string) bool {
	switch word {
	case "start_main", "local", "[[", "[", ":", "case", "esac", "if", "then", "elif", "else", "fi", "for", "do", "done", "while", "builtin", "return":
		return true
	default:
		return false
	}
}

var startLeadingCommandPattern = regexp.MustCompile(`^(?:builtin[[:space:]]+|command[[:space:]]+)?([A-Za-z_][A-Za-z0-9_-]*)(?:[[:space:]]|$)`)

func startMainLeadingCommand(line string) string {
	match := startLeadingCommandPattern.FindStringSubmatch(line)
	if match == nil {
		return ""
	}
	return match[1]
}

func startReachableFunctions(lines []string, ranges []shellFunctionRange) map[string]bool {
	byName := make(map[string]shellFunctionRange, len(ranges))
	for _, functionRange := range ranges {
		byName[functionRange.name] = functionRange
	}
	reachable := map[string]bool{}
	queue := []string{"start_main", "start_finish", "start_emergency", "start_exit_trap", "start_signal"}
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		if reachable[name] {
			continue
		}
		functionRange, ok := byName[name]
		if !ok {
			continue
		}
		reachable[name] = true
		body := strings.Join(lines[functionRange.start-1:functionRange.end], "\n")
		for _, command := range shellCommands(body) {
			called := filepath.Base(command.Word)
			if _, exists := byName[called]; exists && !reachable[called] {
				queue = append(queue, called)
			}
		}
		for _, line := range lines[functionRange.start-1 : functionRange.end] {
			if match := startCaptureArgvPattern.FindStringSubmatch(line); match != nil {
				if _, exists := byName[match[1]]; exists && !reachable[match[1]] {
					queue = append(queue, match[1])
				}
			}
		}
	}
	return reachable
}

type startOutcomeCatalog struct {
	notice      map[string]bool
	intentional map[string]bool
	keys        []string
}

func (catalog startOutcomeCatalog) contains(family, key string) bool {
	if family == "notice" {
		return catalog.notice[key]
	}
	return family == "intentional" && catalog.intentional[key]
}

func parseStartOutcomeCatalog(path string, lines []string, ranges []shellFunctionRange) (startOutcomeCatalog, []HookStartExitFinding) {
	catalog := startOutcomeCatalog{notice: map[string]bool{}, intentional: map[string]bool{}}
	var findings []HookStartExitFinding
	add := func(line int, detail string) {
		findings = append(findings, HookStartExitFinding{path, line, "start outcome catalog", detail})
	}
	rangeFor := func(name string) (shellFunctionRange, bool) {
		for _, candidate := range ranges {
			if candidate.name == name {
				return candidate, true
			}
		}
		return shellFunctionRange{}, false
	}
	readCase := func(function, marker string, destination map[string]bool) {
		functionRange, ok := rangeFor(function)
		if !ok {
			add(1, fmt.Sprintf("%s is missing", function))
			return
		}
		inside := false
		for index := functionRange.start - 1; index < functionRange.end; index++ {
			trimmed := strings.TrimSpace(lines[index])
			if !inside {
				if trimmed == marker {
					inside = true
				}
				continue
			}
			if trimmed == "esac" {
				break
			}
			if !strings.HasSuffix(trimmed, ")") || strings.ContainsAny(strings.TrimSuffix(trimmed, ")"), " \t") {
				continue
			}
			patterns := strings.TrimSuffix(trimmed, ")")
			if patterns == "*" {
				continue
			}
			for _, key := range strings.Split(patterns, "|") {
				if !startCatalogKeyPattern.MatchString(key) {
					add(index+1, fmt.Sprintf("catalog key %q is not a literal", key))
					continue
				}
				if destination[key] {
					add(index+1, fmt.Sprintf("catalog key %q is duplicated", key))
				}
				destination[key] = true
			}
		}
		if !inside || len(destination) == 0 {
			add(functionRange.start, fmt.Sprintf("%s has no machine-readable %s", function, marker))
		}
	}
	readCase("start_notice_json", `case "$key" in`, catalog.notice)
	readCase("start_finish", `case "$key" in`, catalog.intentional)
	if functionRange, ok := rangeFor("start_notice_json"); ok {
		currentLine := 0
		currentKeys := ""
		currentBody := ""
		validate := func() {
			if currentKeys == "" || currentKeys == "*" {
				return
			}
			if !strings.Contains(currentBody, `start_notice='{"systemMessage":"`) && !strings.Contains(currentBody, "start_notice=$start_last_resort") {
				add(currentLine, fmt.Sprintf("notice %q does not select a sole systemMessage object", currentKeys))
			}
		}
		for index := functionRange.start - 1; index < functionRange.end; index++ {
			trimmed := strings.TrimSpace(lines[index])
			if strings.HasSuffix(trimmed, ")") && !strings.ContainsAny(strings.TrimSuffix(trimmed, ")"), " \t") {
				validate()
				currentLine = index + 1
				currentKeys = strings.TrimSuffix(trimmed, ")")
				currentBody = ""
				continue
			}
			if currentKeys != "" {
				currentBody += trimmed + "\n"
			}
		}
		validate()
	}
	for key := range catalog.notice {
		catalog.keys = append(catalog.keys, key)
	}
	for key := range catalog.intentional {
		if catalog.notice[key] {
			add(1, fmt.Sprintf("catalog key %q occurs in both families", key))
		}
		catalog.keys = append(catalog.keys, key)
	}
	sort.Strings(catalog.keys)
	return catalog, findings
}

func shellFunctionRanges(lines []string) []shellFunctionRange {
	var ranges []shellFunctionRange
	for index, line := range lines {
		match := functionStartPattern.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		for end := index + 1; end < len(lines); end++ {
			if lines[end] == "}" {
				ranges = append(ranges, shellFunctionRange{match[1], index + 1, end + 1})
				break
			}
		}
	}
	return ranges
}

func lineContaining(lines []string, text string) int {
	for index, line := range lines {
		if strings.Contains(line, text) {
			return index + 1
		}
	}
	return 0
}

func countTrimmedLine(lines []string, text string) int {
	count := 0
	for _, line := range lines {
		if strings.TrimSpace(line) == text {
			count++
		}
	}
	return count
}

func lastLineContaining(lines []string, text string) int {
	for index := len(lines) - 1; index >= 0; index-- {
		if strings.Contains(lines[index], text) {
			return index + 1
		}
	}
	return 0
}

// declaresTestFunc reports whether source declares the named test function at
// the start of a line, so a quoted copy of the name elsewhere does not count.
func declaresTestFunc(source, name string) bool {
	return regexp.MustCompile(`(?m)^func ` + regexp.QuoteMeta(name) + `\(`).MatchString(source)
}
