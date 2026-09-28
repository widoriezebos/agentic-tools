package validate

// The prose half of the agent-protocol-fixtures section of the retired
// validate-metasystem.sh (verbs-object-action U7b): the shipped dispatch
// templates, role preambles and plans keep their contract, and the turn-prompt
// and critique checkers name each drift. The checks read the shipped
// installation at ../.. and write only to temp dirs.

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func shippedFile(t *testing.T, parts ...string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(append([]string{"..", ".."}, parts...)...))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// The brief carries only orchestrator-authored headers; dispatch owns job
// identity, role, runtime, model and round. A follow-up restates one finding,
// its disposition and the unchanged return contract.
func TestShippedDispatchTemplatesKeepTheirHeaders(t *testing.T) {
	t.Parallel()
	brief := shippedFile(t, "internal", "protocol", "templates", "brief.md")
	for _, header := range []string{"Working Mode:", "Mission Stream:", "Orchestrator Identity:", "Date:"} {
		if !regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(header)).MatchString(brief) {
			t.Errorf("brief template is missing authored header: %s", header)
		}
	}
	if regexp.MustCompile(`(?m)^(Job-Id|Role|Runtime|Model|Round):`).MatchString(brief) {
		t.Error("brief template contains a dispatch-assigned header")
	}
	followUp := shippedFile(t, "internal", "protocol", "templates", "follow-up.md")
	for pattern, message := range map[string]string{
		`(?m)^Finding Id:`:                  "follow-up template does not restate one finding",
		`(?m)^Disposition:`:                 "follow-up template does not restate the disposition",
		`(?m)^# Unchanged Return Contract$`: "follow-up template can lose the original return contract",
	} {
		if !regexp.MustCompile(pattern).MatchString(followUp) {
			t.Error(message)
		}
	}
}

// The host-turn instruction is parameterized by exactly the cycle, the fence
// headroom and the reconciliation flag, in that order, and never by runtime.
// The scan crosses line boundaries the way the placeholder grammar does.
func TestShippedHostTurnInstructionParameters(t *testing.T) {
	t.Parallel()
	body := shippedFile(t, "internal", "protocol", "templates", "host-turn-instruction.md")
	var parameters []string
	start := -1
	for index, char := range body {
		switch {
		case char == '<':
			start = index
		case char == '>' && start >= 0 && index-start > 1:
			parameters = append(parameters, strings.ReplaceAll(body[start+1:index], "\n", `\n`))
			start = -1
		case char == '>':
			start = -1
		}
	}
	if got := strings.Join(parameters, "|"); got != "cycle-number|fence-headroom|yes | no" {
		t.Errorf("host-turn instruction parameters drifted from cycle, fence headroom, reconciliation: %q", parameters)
	}
	if strings.Contains(body, "Runtime:") {
		t.Error("host-turn instruction is parameterized by runtime")
	}
}

// countQuoteBlocks counts the quote blocks from one source whose body
// carries the marker: as a line suffix when the marker names a full heading
// line, anywhere in a line otherwise.
func countQuoteBlocks(preamble, source, marker string, endsLine bool) int {
	count, inBlock, hit, blockSource := 0, false, false, ""
	opener := regexp.MustCompile(`^<!-- quote source="([^"]+)" -->$`)
	for _, line := range strings.Split(preamble, "\n") {
		switch {
		case !inBlock && opener.MatchString(line):
			blockSource, inBlock, hit = opener.FindStringSubmatch(line)[1], true, false
		case inBlock && line == "<!-- /quote -->":
			if blockSource == source && hit {
				count++
			}
			inBlock = false
		case inBlock:
			if endsLine && strings.HasSuffix(line, marker) || !endsLine && strings.Contains(line, marker) {
				hit = true
			}
		}
	}
	return count
}

func copyShippedRoles(t *testing.T) string {
	t.Helper()
	source := filepath.Join("..", "..", "internal", "protocol", "roles")
	destination := filepath.Join(t.TempDir(), "roles")
	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return os.MkdirAll(filepath.Join(destination, rel), 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(destination, rel), data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return destination
}

// Quote markers name their canonical source and the content bytes are
// compared, never a second prose copy. Each mandated (source, marker) pair
// appears in exactly one orchestrator block, so deleting it cannot go
// undetected; a drifted binding criterion is refused naming its source.
func TestShippedPreambleQuotesHoldAndNameEachDrift(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..")
	roles := filepath.Join(root, "internal", "protocol", "roles")
	if violations := PreambleQuotes(root, roles); len(violations) != 0 {
		t.Fatalf("shipped preamble quotes drifted: %v", violations)
	}
	orchestrator := shippedFile(t, "internal", "protocol", "roles", "orchestrator.md")
	for _, mandated := range []struct {
		source, marker string
		endsLine       bool
	}{
		{"AGENTS.md", "## Completion", true},
		{"docs/orchestration.md", "## Delegation Contract", true},
		{"docs/orchestration.md", "### Working without the human", true},
		{"docs/collaboration.md", "## Review Guide in Reports", true},
		{"docs/collaboration.md", "## Escalation Shape", true},
		{"docs/project-rules.md", "These require explicit in-task approval", false},
	} {
		switch count := countQuoteBlocks(orchestrator, mandated.source, mandated.marker, mandated.endsLine); {
		case count < 1:
			t.Errorf("orchestrator preamble lacks mandated quote block: %s %s", mandated.source, mandated.marker)
		case count > 1:
			t.Errorf("orchestrator required-block deletion would go undetected: %s %s appears in %d blocks", mandated.source, mandated.marker, count)
		}
	}

	// Drift the design-critique block by its markers, not its wording, so a
	// lawful rewording of source and quote together cannot empty the fixture.
	designRoles := copyShippedRoles(t)
	designPath := filepath.Join(designRoles, "design-critic.md")
	lines := strings.Split(shippedFile(t, "internal", "protocol", "roles", "design-critic.md"), "\n")
	inBlock, drifted := false, false
	for index, line := range lines {
		switch {
		case line == `<!-- quote source="skills/design-critique/SKILL.md" -->`:
			inBlock = true
		case inBlock && line == "<!-- /quote -->":
			inBlock = false
		case inBlock && !drifted && strings.HasPrefix(line, "> ") && len(line) > 2:
			lines[index], drifted = line+" (drifted)", true
		}
	}
	if !drifted {
		t.Fatal("design-critique quote drift fixture changed nothing")
	}
	if err := os.WriteFile(designPath, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, drift := range []struct {
		roles, want string
	}{
		{designRoles, "quote drifted from skills/design-critique/SKILL.md"},
		{driftedCopy(t, "code-critic.md", "ship a defect", "ship no defect"), "quote drifted from skills/code-critique/SKILL.md"},
		{driftedCopy(t, "orchestrator.md", "The human's absence narrows", "The human's presence narrows"), "quote drifted from docs/orchestration.md"},
	} {
		if violations := PreambleQuotes(root, drift.roles); !strings.Contains(strings.Join(violations, "\n"), drift.want) {
			t.Errorf("preamble quote checker did not refuse a drifted criterion naming %q: %v", drift.want, violations)
		}
	}
}

func driftedCopy(t *testing.T, file, from, to string) string {
	t.Helper()
	roles := copyShippedRoles(t)
	path := filepath.Join(roles, file)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), from) {
		t.Fatalf("%s no longer carries %q; the drift fixture would test nothing", file, from)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(string(data), from, to, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	return roles
}

// A hand-authored host-turn prompt around the shipped orchestrator preamble
// passes, and each single mutation is refused by its named check family. The
// positive prompt deliberately shares no assembly logic with the checker.
func TestShippedPreambleTurnPromptAcceptsAndNamesEachDrift(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..")
	turnDir := filepath.Join(t.TempDir(), "turn-3")
	writeFile(t, filepath.Join(turnDir, "turn.json"), `{"missionId":"fixture-mission","turnId":"turn-3","cycle":3,"runtime":"fake","model":"fake-model","hostSession":null,"reconciliation":false,"startedAt":"2026-08-04T12:00:00Z","pid":1234,"outcome":null}`)
	good := strings.Join([]string{
		"Mission-Id: fixture-mission", "Turn-Id: turn-3", "Cycle: 3", "Host-Session: none",
		"Runtime: fake", "Model: fake-model", "Reconciliation: no", "",
	}, "\n") + "\n" + shippedFile(t, "internal", "protocol", "roles", "orchestrator.md") + "\n" + `## Mission Contract
Signed fixture mission contract.

## Ledger Tail
<<<DATA>>>
1	contract-improved	aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa	metric=1
2	unresolved	bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb	metric=1
<<<END>>>

## Human Answers
<<<DATA>>>
ask-0	stream-a	2026-08-18T00:00:00Z	May the fixture proceed?	Yes, option A, this mission only.
<<<END>>>

## Open Asks
<<<DATA>>>
ask-1	stream-a	reserved-decision	Approve the named API contract?
<<<END>>>

## Streams
<<<DATA>>>
stream-a	active	Make the fixture gate pass	none	ask-0
stream-b	parked-reserved	Publish the fixture	Awaiting approval	none
<<<END>>>

## Reconciliation
<<<DATA>>>
(none)
<<<END>>>

## Landed Returns
<<<DATA>>>
chain-a	2	artifacts/agents/chain-a/rounds/2/return.json
chain-b	invalid	artifacts/agents/chain-b/rounds/1/return.json
chain-c	unreadable	none
<<<END>>>

## This Turn
Cycle: 3
Fence headroom: cycles=2,jobs=3
Reconciliation: no

Advance active streams by designing, dispatching, reviewing, and certifying. When Reconciliation is ` + "`yes`" + `, reconcile the prior turn before starting new work. End this turn when work is dispatched and reviewed; never wait inside the turn.
`
	dir := t.TempDir()
	goodPath := filepath.Join(dir, "good.md")
	writeFile(t, goodPath, good)
	if violation := TurnPrompt(root, goodPath, turnDir); violation != nil {
		t.Fatalf("hand-authored prompt around the shipped preamble refused: [%s] %s", violation.Check, violation.Message)
	}
	for _, mutation := range []struct {
		name, check string
		pairs       []string
	}{
		{"missing-header", "headers", []string{"Model: fake-model\n", ""}},
		{"turn-mismatch", "identity", []string{"Turn-Id: turn-3\n", "Turn-Id: turn-other\n"}},
		{"mission-mismatch", "identity", []string{"Mission-Id: fixture-mission\n", "Mission-Id: other-mission\n"}},
		{"altered-preamble", "preamble", []string{"You are the orchestrator for an unattended mission.", "You are an orchestrator for an unattended mission."}},
		{"headings-out-of-order", "headings", []string{"## Open Asks", "## TEMP", "## Streams", "## Open Asks", "## TEMP", "## Streams"}},
		{"unfenced-data", "fencing", []string{"## Open Asks\n<<<DATA>>>\nask-1\tstream-a\treserved-decision\tApprove the named API contract?\n<<<END>>>",
			"## Open Asks\nask-1\tstream-a\treserved-decision\tApprove the named API contract?"}},
		{"malformed-record", "records", []string{"ask-1\tstream-a\treserved-decision\tApprove the named API contract?", "ask-1\tstream-a\treserved-decision"}},
	} {
		mutated := good
		for index := 0; index < len(mutation.pairs); index += 2 {
			mutated = strings.Replace(mutated, mutation.pairs[index], mutation.pairs[index+1], 1)
		}
		if mutated == good {
			t.Fatalf("turn-prompt mutation did not change the fixture: %s", mutation.name)
		}
		path := filepath.Join(dir, mutation.name+".md")
		writeFile(t, path, mutated)
		violation := TurnPrompt(root, path, turnDir)
		if violation == nil || violation.Check != mutation.check {
			t.Errorf("turn prompt checker did not name the %s check for %s: %+v", mutation.check, mutation.name, violation)
		}
	}
}

// Critique closure joins the canonical return's findings against the one
// dispositions table; each join invariant is refused by name.
func TestCritiqueClosedNamesEachJoinInvariant(t *testing.T) {
	t.Parallel()
	material := map[string]any{"id": "F-1", "severity": "high", "material": true, "claim": "contract gap", "evidence": "read design"}
	nonmaterial := map[string]any{"id": "F-2", "severity": "low", "material": false, "claim": "wording issue", "evidence": "read design"}
	second := map[string]any{"id": "F-3", "severity": "medium", "material": true, "claim": "incorrect premise", "evidence": "checked implementation"}
	dir := t.TempDir()
	writeReturn := func(name string, findings []any, drop bool) string {
		value := map[string]any{"jobId": "fixture-job", "round": 1, "runtime": "fake", "sessionId": "session-1", "mode": "design",
			"reviewedCommit": strings.Repeat("0", 40)}
		count := 0
		for _, finding := range findings {
			if finding.(map[string]any)["material"] == true {
				count++
			}
		}
		value["verdictMaterialCount"] = count
		if !drop {
			value["findings"] = findings
		}
		data, _ := json.Marshal(value)
		path := filepath.Join(dir, name+".json")
		writeFile(t, path, string(data))
		return path
	}
	writeTable := func(name, separator string, rows ...string) string {
		var table strings.Builder
		table.WriteString("| Finding id | Disposition | Reasoning and evidence | Amendment |\n" + separator + "\n")
		for _, row := range rows {
			table.WriteString("| " + strings.ReplaceAll(row, "\t", " | ") + " |\n")
		}
		path := filepath.Join(dir, name+".md")
		writeFile(t, path, table.String())
		return path
	}
	separator := "| --- | --- | --- | --- |"
	joinable := writeReturn("joinable", []any{material, nonmaterial, second}, false)
	allDisposed := writeTable("all-disposed", separator,
		"F-1\taccepted\tdesign amended\tsection 3", "F-2\tnoted\tdoes not change implementation\tnone", "F-3\trefuted\timplementation disproves the claim\tnone")
	if violations := CritiqueClosed(joinable, allDisposed); len(violations) != 0 {
		t.Fatalf("closed critique refused: %v", violations)
	}
	for _, open := range []struct {
		name, findings, dispositions string
		want                         []string
	}{
		{"open-material", joinable, writeTable("open-material", separator,
			"F-2\tnoted\tdoes not change implementation\tnone", "F-3\trefuted\timplementation disproves the claim\tnone"),
			[]string{"finding id 'F-1' has no disposition row"}},
		{"noted-on-material", joinable, writeTable("noted-on-material", separator,
			"F-1\tnoted\tincorrect disposition\tnone", "F-2\tnoted\tdoes not change implementation\tnone", "F-3\trefuted\timplementation disproves the claim\tnone"),
			[]string{"material finding id 'F-1' cannot use disposition 'noted'"}},
		{"missing-nonmaterial-disposition", joinable, writeTable("missing-nonmaterial", separator,
			"F-1\taccepted\tdesign amended\tsection 3", "F-3\trefuted\timplementation disproves the claim\tnone"),
			[]string{"finding id 'F-2' has no disposition row"}},
		{"duplicate-id", writeReturn("duplicate-id", []any{material, material, nonmaterial, second}, false), writeTable("duplicate-id", separator,
			"F-1\taccepted\tfirst row\tsection 3", "F-1\trefuted\tsecond row\tnone", "F-2\tnoted\tdoes not change implementation\tnone", "F-3\trefuted\timplementation disproves the claim\tnone"),
			[]string{"duplicate finding id: 'F-1'", "duplicate disposition id: 'F-1'"}},
		{"unknown-disposition", joinable, writeTable("unknown-disposition", separator,
			"F-1\tdismissed\tnot a protocol value\tnone", "F-2\tnoted\tdoes not change implementation\tnone", "F-3\trefuted\timplementation disproves the claim\tnone"),
			[]string{"disposition for finding id 'F-1' has unknown value 'dismissed'"}},
		{"unknown-finding-id", joinable, writeTable("unknown-finding-id", separator,
			"F-1\taccepted\tdesign amended\tsection 3", "F-2\tnoted\tdoes not change implementation\tnone", "F-3\trefuted\timplementation disproves the claim\tnone", "F-404\trefuted\tno matching finding\tnone"),
			[]string{"disposition names unknown finding id: 'F-404'"}},
		{"unjoinable-missing-findings", writeReturn("missing-findings", nil, true), allDisposed, []string{"$.findings array is missing"}},
		{"unjoinable-malformed-table", joinable, writeTable("malformed-table", "| --- | --- | --- |"), []string{"malformed dispositions table: invalid separator row"}},
	} {
		violations := strings.Join(CritiqueClosed(open.findings, open.dispositions), "\n")
		if violations == "" {
			t.Errorf("critique checker accepted the negative %s fixture", open.name)
		}
		for _, want := range open.want {
			if !strings.Contains(violations, want) {
				t.Errorf("critique checker did not name the %s violation %q:\n%s", open.name, want, violations)
			}
		}
	}
}

// The shipped plans prescribe no retired term.
func TestShippedPlansPrescribeNoRetiredTerm(t *testing.T) {
	t.Parallel()
	retired, violations, err := PlanConsistency(filepath.Join("..", "..", "plans"))
	if err != nil || len(violations) != 0 {
		t.Fatalf("plan consistency over the shipped plans: retired=%d violations=%v err=%v", retired, violations, err)
	}
}
