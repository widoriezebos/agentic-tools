package validate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/turn"
)

// Violation is one named check failure: the check family the failure
// belongs to and the human message describing it.
type Violation struct {
	Check   string
	Message string
}

// The machine header keys, in the order the assembler must emit them.
var turnHeaderKeys = []string{
	"Mission-Id", "Turn-Id", "Cycle", "Host-Session", "Runtime", "Model", "Reconciliation",
}

// The eight required section headings, in order. Human Answers sits at
// the top of the dynamic sections (issue #11): a human ruling is THE
// thing the next turn must act on, never inferable from an id.
var turnHeadings = []string{
	"## Mission Contract", "## Ledger Tail", "## Human Answers", "## Open Asks",
	"## Streams", "## Reconciliation", "## Landed Returns", "## This Turn",
}

// servingGoalHeading is the one OPTIONAL section:
// a single orientation line between the contract and the ledger tail.
// Goal absence produces no section and never blocks assembly.
const servingGoalHeading = "## Serving goal"

var servingGoalLine = regexp.MustCompile(`^.{1,64} — .{1,160}$`)

var (
	turnIDRe    = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	turnShaRe   = regexp.MustCompile(`^[0-9a-f]{40,64}$`)
	turnCycleRe = regexp.MustCompile(`^[1-9][0-9]*$`)
)

var turnClassifications = map[string]bool{
	"contract-improved": true, "falsified-continue": true, "falsified-dead-end": true,
	"no-progress": true, "unresolved": true, "invalid-run": true,
}

var turnStreamStates = map[string]bool{
	"active": true, "parked-reserved": true, "parked-stop-loss": true, "done": true,
}

// TurnPrompt validates an assembled unattended host-turn prompt against
// its canonical turn record and the shipped orchestrator preamble: LF
// framing, the ordered machine header, the byte-exact preamble, section
// fencing, and the tab-separated Ledger/Asks/Streams/Reconciliation/
// Landed Returns records. It returns the first violation found, or nil
// on a pass.
func TurnPrompt(root, promptPath, turnDir string) *Violation {
	preamblePath := filepath.Join(root, "scripts", "agents", "roles", "orchestrator.md")
	turnRecordPath := filepath.Join(turnDir, "turn.json")

	prompt, err := os.ReadFile(promptPath)
	if err != nil {
		return &Violation{"framing", fmt.Sprintf("prompt could not be read: %s: %v", promptPath, err)}
	}
	preamble, err := os.ReadFile(preamblePath)
	if err != nil {
		return &Violation{"preamble", fmt.Sprintf("shipped preamble could not be read: %s: %v", preamblePath, err)}
	}
	if bytes.Contains(prompt, []byte("\r")) {
		return &Violation{"framing", "prompt must use LF line endings"}
	}
	if !bytes.HasSuffix(prompt, []byte("\n")) {
		return &Violation{"framing", "prompt must end with an LF"}
	}

	headerEnd := bytes.Index(prompt, []byte("\n\n"))
	if headerEnd < 0 {
		return &Violation{"headers", "machine header is not followed by one blank line"}
	}
	if !utf8.Valid(prompt[:headerEnd]) {
		return &Violation{"headers", "machine header is not UTF-8"}
	}
	header := map[string]string{}
	var headerOrder []string
	for _, line := range strings.Split(string(prompt[:headerEnd]), "\n") {
		key, value, found := strings.Cut(line, ": ")
		if !found {
			return &Violation{"headers", fmt.Sprintf("malformed machine header line: %q", line)}
		}
		if _, duplicate := header[key]; duplicate {
			return &Violation{"headers", fmt.Sprintf("machine header repeats %s", key)}
		}
		header[key] = value
		headerOrder = append(headerOrder, key)
	}
	for _, key := range turnHeaderKeys {
		if header[key] == "" {
			return &Violation{"headers", fmt.Sprintf("header key %s is missing or empty", key)}
		}
	}
	if !equalStrings(headerOrder, turnHeaderKeys) {
		return &Violation{"headers", "machine header keys are not in their declared order"}
	}

	recordBytes, exists, err := readFileIfExists(turnRecordPath)
	if !exists {
		return &Violation{"turn-record", fmt.Sprintf("turn record does not exist: %s", turnRecordPath)}
	}
	if err != nil {
		return &Violation{"turn-record", fmt.Sprintf("turn record is unreadable: %s: %v", turnRecordPath, err)}
	}
	var parsed any
	if err := json.Unmarshal(recordBytes, &parsed); err != nil {
		return &Violation{"turn-record", fmt.Sprintf("turn record is unreadable: %s: %v", turnRecordPath, err)}
	}
	record, ok := parsed.(map[string]any)
	if !ok {
		return &Violation{"turn-record", "turn.json must contain a JSON object"}
	}
	identityFields := map[string]string{}
	for _, field := range []string{"missionId", "turnId"} {
		value, ok := record[field].(string)
		if !ok || value == "" {
			return &Violation{"turn-record", fmt.Sprintf("turn.json field %s must be a non-empty string", field)}
		}
		identityFields[field] = value
	}
	if header["Mission-Id"] != identityFields["missionId"] {
		return &Violation{"identity", "Mission-Id does not equal turn.json missionId"}
	}
	if header["Turn-Id"] != identityFields["turnId"] {
		return &Violation{"identity", "Turn-Id does not equal turn.json turnId"}
	}

	preambleStart := headerEnd + 2
	preambleEnd := preambleStart + len(preamble)
	if preambleEnd > len(prompt) || !bytes.Equal(prompt[preambleStart:preambleEnd], preamble) {
		return &Violation{"preamble", "assembled preamble bytes differ from scripts/agents/roles/orchestrator.md"}
	}
	if preambleEnd >= len(prompt) || prompt[preambleEnd] != '\n' {
		return &Violation{"preamble", "shipped preamble is not followed by exactly one blank line"}
	}

	sectionBytes := prompt[preambleEnd+1:]
	if !utf8.Valid(sectionBytes) {
		return &Violation{"framing", "prompt sections are not UTF-8"}
	}
	lines := strings.Split(string(sectionBytes), "\n")
	if lines[len(lines)-1] != "" {
		return &Violation{"framing", "prompt sections must end with an LF"}
	}

	type headingPosition struct {
		index   int
		heading string
	}
	var positions []headingPosition
	insideData := false
	for index, line := range lines[:len(lines)-1] {
		switch {
		case line == "<<<DATA>>>":
			if insideData {
				return &Violation{"fencing", "data fences may not nest"}
			}
			insideData = true
		case line == "<<<END>>>":
			if !insideData {
				return &Violation{"fencing", "data end marker has no matching start marker"}
			}
			insideData = false
		default:
			if !insideData && isTurnHeading(line) {
				positions = append(positions, headingPosition{index, line})
			}
		}
	}
	if insideData {
		return &Violation{"fencing", "data start marker has no matching end marker"}
	}
	var found []string
	for _, position := range positions {
		found = append(found, position.heading)
	}
	// The serving-goal block is OPTIONAL at exactly one position:
	// immediately after the mission contract. The
	// assembler and this grammar move together by design.
	withGoal := make([]string, 0, len(turnHeadings)+1)
	withGoal = append(withGoal, turnHeadings[0], servingGoalHeading)
	withGoal = append(withGoal, turnHeadings[1:]...)
	if !equalStrings(found, turnHeadings) && !equalStrings(found, withGoal) {
		return &Violation{"headings", "the required headings are missing, duplicated, or out of order (the optional ## Serving goal sits only between the contract and the ledger tail)"}
	}

	sections := map[string][]string{}
	for index, position := range positions {
		end := len(lines) - 1
		last := index+1 >= len(positions)
		if !last {
			end = positions[index+1].index
		}
		body := lines[position.index+1 : end]
		if !last {
			// Every section but the last is separated from the next
			// block by exactly one blank line.
			if len(body) == 0 || body[len(body)-1] != "" ||
				(len(body) > 1 && body[len(body)-2] == "") {
				return &Violation{"framing", fmt.Sprintf(
					"%s is not separated from the next block by exactly one blank line", position.heading)}
			}
			body = body[:len(body)-1]
		}
		sections[position.heading] = body
	}

	if body, present := sections[servingGoalHeading]; present {
		wellFormed := len(body) == 1 && servingGoalLine.MatchString(body[0])
		if !wellFormed {
			return &Violation{"serving-goal", "## Serving goal holds exactly one '<id> — <intent>' line"}
		}
	}

	// Ledger Tail records carry four fields, or five when the measurement
	// line recorded its new-best marker — both vintages are legal during the
	// best-token migration (docs/design/stop-loss-core.md).
	ledger, violation := turnDataRecordsRange(sections, "## Ledger Tail", 4, 5)
	if violation != nil {
		return violation
	}
	var cycles []string
	for number, record := range ledger {
		cycle, classification, candidateSha, observed := record[0], record[1], record[2], record[3]
		if !turnCycleRe.MatchString(cycle) {
			return &Violation{"records", fmt.Sprintf("Ledger Tail record %d cycle must be a positive integer", number+1)}
		}
		if !turnClassifications[classification] {
			return &Violation{"records", fmt.Sprintf("Ledger Tail record %d classification is unknown", number+1)}
		}
		if candidateSha != "none" && !turnShaRe.MatchString(candidateSha) {
			return &Violation{"records", fmt.Sprintf("Ledger Tail record %d candidateSha must be a resolved git SHA or none", number+1)}
		}
		if observed == "(none)" {
			return &Violation{"records", fmt.Sprintf("Ledger Tail record %d uses (none) instead of literal none", number+1)}
		}
		if len(record) == 5 && record[4] != "yes" && record[4] != "no" {
			return &Violation{"records", fmt.Sprintf("Ledger Tail record %d best marker must be yes or no", number+1)}
		}
		cycles = append(cycles, cycle)
	}
	for index := 1; index < len(cycles); index++ {
		if !cycleLess(cycles[index-1], cycles[index]) {
			return &Violation{"records", "Ledger Tail records must be unique and ordered oldest to newest"}
		}
	}

	answers, violation := turnDataRecords(sections, "## Human Answers", 5)
	if violation != nil {
		return violation
	}
	var answerIds []string
	for number, record := range answers {
		askID, streamID, answeredAt, question, answer := record[0], record[1], record[2], record[3], record[4]
		if !turnIDRe.MatchString(askID) {
			return &Violation{"records", fmt.Sprintf("Human Answers record %d askId is invalid", number+1)}
		}
		if streamID != "none" && !turnIDRe.MatchString(streamID) {
			return &Violation{"records", fmt.Sprintf("Human Answers record %d streamId must be an id or none", number+1)}
		}
		if answeredAt == "none" || answeredAt == "" {
			return &Violation{"records", fmt.Sprintf("Human Answers record %d carries no answeredAt", number+1)}
		}
		if question == "(none)" || answer == "(none)" {
			return &Violation{"records", fmt.Sprintf("Human Answers record %d uses (none) instead of literal none", number+1)}
		}
		if answer == "none" || answer == "" {
			return &Violation{"records", fmt.Sprintf("Human Answers record %d carries no answer", number+1)}
		}
		answerIds = append(answerIds, askID)
	}
	if !strictlyIncreasing(answerIds) {
		return &Violation{"records", "Human Answers records must have unique ask ids in sorted order"}
	}

	asks, violation := turnDataRecords(sections, "## Open Asks", 4)
	if violation != nil {
		return violation
	}
	var askIds []string
	for number, record := range asks {
		askID, streamID, reasonClass, question := record[0], record[1], record[2], record[3]
		if !turnIDRe.MatchString(askID) {
			return &Violation{"records", fmt.Sprintf("Open Asks record %d askId is invalid", number+1)}
		}
		if streamID != "none" && !turnIDRe.MatchString(streamID) {
			return &Violation{"records", fmt.Sprintf("Open Asks record %d streamId must be an id or none", number+1)}
		}
		if !turn.PromptMayCarry(reasonClass) {
			return &Violation{"records", fmt.Sprintf("Open Asks record %d reasonClass is unknown", number+1)}
		}
		if question == "(none)" {
			return &Violation{"records", fmt.Sprintf("Open Asks record %d uses (none) instead of literal none", number+1)}
		}
		askIds = append(askIds, askID)
	}
	if !strictlyIncreasing(askIds) {
		return &Violation{"records", "Open Asks records must have unique ask ids in sorted order"}
	}

	streams, violation := turnDataRecords(sections, "## Streams", 5)
	if violation != nil {
		return violation
	}
	var streamIds []string
	for number, record := range streams {
		streamID, state, goal, reason := record[0], record[1], record[2], record[3]
		if answeredAsk := record[4]; answeredAsk != "none" && !turnIDRe.MatchString(answeredAsk) {
			return &Violation{"records", fmt.Sprintf("Streams record %d answeredAsk must be an id or none", number+1)}
		}
		if !turnIDRe.MatchString(streamID) {
			return &Violation{"records", fmt.Sprintf("Streams record %d streamId is invalid", number+1)}
		}
		if !turnStreamStates[state] {
			return &Violation{"records", fmt.Sprintf("Streams record %d state is unknown", number+1)}
		}
		if goal == "(none)" || reason == "(none)" {
			return &Violation{"records", fmt.Sprintf("Streams record %d uses (none) instead of literal none", number+1)}
		}
		streamIds = append(streamIds, streamID)
	}
	if !strictlyIncreasing(streamIds) {
		return &Violation{"records", "Streams records must have unique stream ids in sorted order"}
	}

	reconciliation, violation := turnDataRecords(sections, "## Reconciliation", 3)
	if violation != nil {
		return violation
	}
	for number, record := range reconciliation {
		turnID, outcome, detail := record[0], record[1], record[2]
		if !turnIDRe.MatchString(turnID) {
			return &Violation{"records", fmt.Sprintf("Reconciliation record %d turnId is invalid", number+1)}
		}
		if outcome == "(none)" || detail == "(none)" {
			return &Violation{"records", fmt.Sprintf("Reconciliation record %d uses (none) instead of literal none", number+1)}
		}
	}

	// Landed Returns rows are chain-root, round-or-marker, path-or-none: the
	// generic three-field grammar, under which the invalid, unreadable, and
	// overflow marker rows are syntax-compatible by construction.
	if _, violation := turnDataRecords(sections, "## Landed Returns", 3); violation != nil {
		return violation
	}

	return nil
}

func isTurnHeading(line string) bool {
	if line == servingGoalHeading {
		return true
	}
	for _, heading := range turnHeadings {
		if line == heading {
			return true
		}
	}
	return false
}

// turnDataRecords extracts a section's fenced, tab-separated records:
// the body must be exactly one <<<DATA>>>...<<<END>>> fence holding
// either the literal (none) or records of fieldCount non-empty fields.
func turnDataRecords(sections map[string][]string, heading string, fieldCount int) ([][]string, *Violation) {
	return turnDataRecordsRange(sections, heading, fieldCount, fieldCount)
}

// turnDataRecordsRange is turnDataRecords for a section whose records may
// carry a bounded range of field counts (a named grammar migration).
func turnDataRecordsRange(sections map[string][]string, heading string, minFields, maxFields int) ([][]string, *Violation) {
	body := sections[heading]
	if len(body) < 3 || body[0] != "<<<DATA>>>" || body[len(body)-1] != "<<<END>>>" {
		return nil, &Violation{"fencing", fmt.Sprintf("%s is not fenced with the fixed data markers", heading)}
	}
	content := body[1 : len(body)-1]
	if len(content) == 0 {
		return nil, &Violation{"fencing", fmt.Sprintf("%s has an empty data fence; use (none)", heading)}
	}
	if len(content) == 1 && content[0] == "(none)" {
		return nil, nil
	}
	for _, line := range content {
		if line == "(none)" {
			return nil, &Violation{"records", fmt.Sprintf("%s mixes (none) with records", heading)}
		}
	}
	var records [][]string
	for number, line := range content {
		fields := strings.Split(line, "\t")
		valid := len(fields) >= minFields && len(fields) <= maxFields
		for _, field := range fields {
			if field == "" {
				valid = false
			}
		}
		if !valid {
			want := strconv.Itoa(minFields)
			if maxFields != minFields {
				want = fmt.Sprintf("%d to %d", minFields, maxFields)
			}
			return nil, &Violation{"records", fmt.Sprintf(
				"%s record %d must contain %s non-empty tab-separated fields", heading, number+1, want)}
		}
		records = append(records, fields)
	}
	return records, nil
}

// cycleLess compares two positive decimal integers of unbounded size:
// with no leading zeros, the longer string is the larger number, and
// equal lengths compare lexicographically.
func cycleLess(left, right string) bool {
	if len(left) != len(right) {
		return len(left) < len(right)
	}
	return left < right
}

func strictlyIncreasing(values []string) bool {
	for index := 1; index < len(values); index++ {
		if values[index] <= values[index-1] {
			return false
		}
	}
	return true
}
