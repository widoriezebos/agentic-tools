package mission

import (
	"encoding/json"
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/contract"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// A mission host-turn prompt is assembled deterministically from the frozen
// authority (the shipped orchestrator preamble and the signed contract) plus
// the mission's live control-plane data (ledger tail, open asks, stream goals,
// the prior turn awaiting reconciliation, and the landed delegate returns the
// host has not yet acted on). The same inputs always produce the same bytes:
// fields are collapsed to single safe lines, data blocks are framed so quoted
// delegate output can never be mistaken for authority, and sections are
// joined in a fixed order. This file owns that assembly.

// promptClassificationRe matches a ledger cycle's single classification line,
// capturing the verdict, the candidate sha, the observed measurement, and the
// optional new-best marker (absent on lines older binaries wrote).
var promptClassificationRe = regexp.MustCompile(
	`(?m)^- Classification:[ \t]*([a-z-]+); candidate-sha=([^;\n]+); observed=(.*?)(?:; best=(yes|no))?$`)

// promptDataMarkers frames every data block. When a field's own text contains
// a frame marker it is defanged so the framing stays unambiguous.
var promptDataMarkers = [...][2]string{
	{"<<<DATA>>>", "< < <DATA> > >"},
	{"<<<END>>>", "< < <END> > >"},
}

// promptOneLine collapses any value to one safe line for a data field: null and
// empty become "none", booleans become yes/no, and newlines, tabs, and frame
// markers are neutralized.
func promptOneLine(value any) string {
	var text string
	switch v := value.(type) {
	case nil:
		return "none"
	case bool:
		if v {
			text = "yes"
		} else {
			text = "no"
		}
	case string:
		text = v
	case json.Number:
		text = v.String()
	default:
		text = fmt.Sprint(v)
	}
	text = strings.NewReplacer("\r", " ", "\n", " ", "\t", " ").Replace(text)
	for _, marker := range promptDataMarkers {
		text = strings.ReplaceAll(text, marker[0], marker[1])
	}
	if text == "" {
		return "none"
	}
	return text
}

// promptAuthoredValues extracts the contract's single authored mission block as
// key=value pairs.
func promptAuthoredValues(contractText string) (map[string]string, error) {
	blocks := contract.AuthoredBlocks(contractText)
	if len(blocks) != 1 {
		return nil, fmt.Errorf("mission contract does not contain exactly one authored mission block")
	}
	values, err := contract.ParseAuthoredValues(blocks[0][1], "mission contract")
	if err != nil {
		return nil, fmt.Errorf("mission contract key/value grammar is invalid: %v", err)
	}
	return values, nil
}

// promptLedgerRecords returns the last maximum adjudicated cycles as
// [cycle, classification, candidate-sha, observed] rows, with the new-best
// marker as a fifth field on lines that carry it.
func promptLedgerRecords(ledgerPath string, maximum int) ([][]string, error) {
	data, err := os.ReadFile(ledgerPath)
	if err != nil {
		return nil, fmt.Errorf("mission ledger is unreadable: %s: %v", ledgerPath, err)
	}
	text := string(data)
	headings := headingRe.FindAllStringSubmatchIndex(text, -1)
	var records [][]string
	for i, loc := range headings {
		end := len(text)
		if i+1 < len(headings) {
			end = headings[i+1][0]
		}
		block := text[loc[1]:end]
		number := text[loc[2]:loc[3]]
		matches := promptClassificationRe.FindAllStringSubmatch(block, -1)
		if len(matches) != 1 {
			return nil, fmt.Errorf("mission ledger cycle %s lacks one parseable classification", number)
		}
		m := matches[0]
		record := []string{
			number,
			m[1],
			promptOneLine(strings.TrimSpace(m[2])),
			promptOneLine(strings.TrimSpace(m[3])),
		}
		if m[4] != "" {
			record = append(record, m[4])
		}
		records = append(records, record)
	}
	if maximum < len(records) {
		records = records[len(records)-maximum:]
	}
	return records, nil
}

// promptAskRecords returns the unanswered asks as [askId, streamId, reasonClass,
// question] rows, ordered by ask id.
func promptAskRecords(dir string) ([][]string, error) {
	if _, err := os.Stat(dir); err != nil {
		return nil, nil
	}
	paths, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	// Closure is DERIVED from the successor's existence, not only the
	// predecessor's marker: a crash between the
	// successor write and the marker write must not resurrect the stale
	// question.
	superseded := map[string]bool{}
	for _, path := range paths {
		ask, err := readJSONObjectFile(path)
		if err != nil {
			continue
		}
		if named, _ := ask["supersedes"].(string); named != "" {
			superseded[named] = true
		}
	}
	var records [][]string
	for _, path := range paths {
		ask, err := readJSONObjectFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("mission ask is missing: %s", path)
			}
			return nil, fmt.Errorf("mission ask is unreadable: %s: %v", path, err)
		}
		if ask["answeredAt"] != nil {
			continue
		}
		// A superseded ask is closed by its successor: showing
		// it beside the live question would present a stale duplicate.
		if ask["supersededBy"] != nil {
			continue
		}
		askID, ok := ask["askId"].(string)
		if !ok {
			return nil, fmt.Errorf("mission ask has no askId: %s", path)
		}
		if superseded[askID] {
			continue
		}
		records = append(records, []string{
			promptOneLine(askID),
			promptOneLine(ask["streamId"]),
			promptOneLine(ask["reasonClass"]),
			promptOneLine(ask["question"]),
		})
	}
	sort.SliceStable(records, func(i, j int) bool { return records[i][0] < records[j][0] })
	return records, nil
}

// promptAnswerRecords renders every standing human ruling: the
// asks each stream's answeredAsk names, as [askId, streamId, answeredAt,
// question, answer] rows ordered by ask id. The answer is THE thing the
// next turn must act on; a state that names a missing or unanswered ask
// is an assembly error, never a silent omission.
func promptAnswerRecords(dir string, state map[string]any) ([][]string, error) {
	streams, ok := state["streams"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("mission state streams are unreadable")
	}
	seen := map[string]bool{}
	var records [][]string
	for _, raw := range streams {
		stream, _ := raw.(map[string]any)
		if stream == nil {
			continue
		}
		askID, _ := stream["answeredAsk"].(string)
		if askID == "" || seen[askID] {
			continue
		}
		seen[askID] = true
		ask, err := readJSONObjectFile(filepath.Join(dir, askID+".json"))
		if err != nil {
			return nil, fmt.Errorf("mission stream names answered ask %s but it is unreadable: %v", askID, err)
		}
		if ask["answeredAt"] == nil {
			return nil, fmt.Errorf("mission stream names answered ask %s but it carries no answer", askID)
		}
		records = append(records, []string{
			promptOneLine(askID),
			promptOneLine(ask["streamId"]),
			promptOneLine(ask["answeredAt"]),
			promptOneLine(ask["question"]),
			promptOneLine(ask["answer"]),
		})
	}
	sort.SliceStable(records, func(i, j int) bool { return records[i][0] < records[j][0] })
	return records, nil
}

// promptStreamRecords returns each stream as an [id, state, goal, reason,
// answeredAsk] row, ordered by stream id.
func promptStreamRecords(state map[string]any) ([][]string, error) {
	streams, ok := state["streams"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("mission state streams are unreadable")
	}
	ids := make([]string, 0, len(streams))
	for id := range streams {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var records [][]string
	for _, id := range ids {
		stream, ok := streams[id].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("mission stream is unreadable: %s", id)
		}
		records = append(records, []string{
			promptOneLine(id),
			promptOneLine(stream["state"]),
			promptOneLine(stream["goal"]),
			promptOneLine(stream["reason"]),
			promptOneLine(stream["answeredAsk"]),
		})
	}
	return records, nil
}

// promptReconciliationRecords returns the single prior turn awaiting
// reconciliation — the most recent turn other than this one whose outcome was
// neither a success nor a clean return — or nothing when reconciliation is not
// required.
func promptReconciliationRecords(state map[string]any, turnID string, required bool) ([][]string, error) {
	if !required {
		return nil, nil
	}
	turnLog, ok := state["turnLog"].([]any)
	if !ok {
		return nil, fmt.Errorf("mission state turn log is unreadable")
	}
	for i := len(turnLog) - 1; i >= 0; i-- {
		item, ok := turnLog[i].(map[string]any)
		if !ok {
			continue
		}
		if id, _ := item["turnId"].(string); id == turnID {
			continue
		}
		outcome := item["outcome"]
		if outcome == nil {
			continue
		}
		if s, ok := outcome.(string); ok && (s == "completed" || s == "return-ok") {
			continue
		}
		return [][]string{{
			promptOneLine(item["turnId"]),
			promptOneLine(outcome),
			promptOneLine(item["detail"]),
		}}, nil
	}
	return nil, fmt.Errorf("turn record requires reconciliation but no prior non-zero outcome exists")
}

// promptDataSection frames a heading and its rows so the data can never be read
// as authority: tab-joined fields between explicit start and end markers, with a
// "(none)" placeholder when there are no rows.
func promptDataSection(heading string, records [][]string) string {
	content := make([]string, 0, len(records))
	for _, record := range records {
		fields := make([]string, len(record))
		for i, field := range record {
			fields[i] = promptOneLine(field)
		}
		content = append(content, strings.Join(fields, "\t"))
	}
	if len(content) == 0 {
		content = []string{"(none)"}
	}
	lines := make([]string, 0, len(content)+3)
	lines = append(lines, heading, "<<<DATA>>>")
	lines = append(lines, content...)
	lines = append(lines, "<<<END>>>")
	return strings.Join(lines, "\n")
}

// promptContractInt reads a required integer from the authored contract values.
func promptContractInt(values map[string]string, key string) (int, bool) {
	raw, ok := values[key]
	if !ok {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, false
	}
	return n, true
}

func promptMax0(x int) int {
	if x < 0 {
		return 0
	}
	return x
}

// promptFenceHeadroom reports how many cycles and jobs the mission may
// still spend against its signed fences, the concurrency headroom, and the
// LIVE delegate roster. The concurrency line and roster exist because a
// host without them is structurally blind to the one fence a parallel
// dispatch can trip — and a host that cannot see the fence it tripped
// misdiagnoses the refusal and records it as a false ledger fact.
func promptFenceHeadroom(repo string, values map[string]string, dir string) (string, error) {
	cycleLimit, ok1 := promptContractInt(values, "fence.cycles")
	jobLimit, ok2 := promptContractInt(values, "fence.jobs")
	concurrencyLimit, ok3 := promptContractInt(values, "fence.concurrency")
	if !ok1 || !ok2 || !ok3 {
		return "", fmt.Errorf("mission contract fence limits are unreadable")
	}
	cycles, jobs := 0, 0
	var running []string
	fencesPath := filepath.Join(dir, "fences.json")
	if _, err := os.Stat(fencesPath); err == nil {
		fences, err := readJSONObjectFile(fencesPath)
		if err != nil {
			return "", fmt.Errorf("mission fence counters are unreadable: %s: %v", fencesPath, err)
		}
		if raw, present := fences["cycles"]; present {
			if _, isBool := raw.(bool); isBool {
				return "", fmt.Errorf("mission fence counters have an invalid shape")
			}
			n, ok := intValue(raw)
			if !ok {
				return "", fmt.Errorf("mission fence counters have an invalid shape")
			}
			cycles = int(n)
		}
		if raw, present := fences["reservations"]; present {
			reservations, ok := raw.(map[string]any)
			if !ok {
				return "", fmt.Errorf("mission fence counters have an invalid shape")
			}
			jobs = len(reservations)
			for job := range reservations {
				if !terminalJobStatus[jobStatus(repo, job)] {
					running = append(running, job)
				}
			}
			sort.Strings(running)
		}
	}
	line := fmt.Sprintf("cycles=%d,jobs=%d,concurrency=%d/%d",
		promptMax0(cycleLimit-cycles), promptMax0(jobLimit-jobs),
		promptMax0(concurrencyLimit-len(running)), concurrencyLimit)
	if len(running) > 0 {
		line += "; live delegates: " + strings.Join(running, ", ")
	}
	return line, nil
}

// promptConfLookup finds key in a metasystem configuration file, reporting a
// duplicate as an error. When optional is set a missing file is not an error.
func promptConfLookup(path, key string, optional bool) (value string, found bool, err error) {
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		if optional {
			return "", false, nil
		}
		return "", false, fmt.Errorf("cannot read metasystem configuration: %s: %v", path, readErr)
	}
	var matches []string
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
			continue
		}
		candidate, val, _ := strings.Cut(line, "=")
		if strings.TrimSpace(candidate) == key {
			matches = append(matches, strings.TrimSpace(val))
		}
	}
	if len(matches) > 1 {
		return "", false, fmt.Errorf("duplicate metasystem configuration key: %s", key)
	}
	if len(matches) == 1 {
		return matches[0], true, nil
	}
	return "", false, nil
}

// promptConfigValue resolves a configuration key against the repository: a
// per-key environment override wins, then the uncommitted local override, then
// the committed settings, then the given default.
func promptConfigValue(repo, key, def string) (string, error) {
	envName := "METASYSTEM_" + strings.NewReplacer(".", "_", "-", "_").Replace(strings.ToUpper(key))
	if v, ok := os.LookupEnv(envName); ok {
		return v, nil
	}
	confPath := filepath.Join(repo, "metasystem.conf")
	if v, found, err := promptConfLookup(confPath+".local", key, true); err != nil {
		return "", err
	} else if found {
		return v, nil
	}
	if v, found, err := promptConfLookup(confPath, key, false); err != nil {
		return "", err
	} else if found {
		return v, nil
	}
	return def, nil
}

// promptTurnInt reads an integer turn field, rejecting booleans and fractional
// numbers.
func promptTurnInt(v any) (int64, bool) {
	if _, isBool := v.(bool); isBool {
		return 0, false
	}
	return intValue(v)
}

// AssemblePrompt writes one unattended mission host-turn prompt for the given
// turn to output, assembled from the frozen authority and the mission's live
// control-plane data. The bytes are a deterministic function of the inputs.
func AssemblePrompt(repo, mission, turnID, output string) error {
	if !idRe.MatchString(mission) || !idRe.MatchString(turnID) {
		return fmt.Errorf("mission and turn ids must match the lowercase metasystem id grammar")
	}
	dir := missionDir(repo, mission)
	turnPath := filepath.Join(dir, "turns", turnID, "turn.json")
	if _, err := os.Stat(turnPath); err != nil {
		return fmt.Errorf("missing turn record: %s", turnPath)
	}
	turn, err := readJSONObjectFile(turnPath)
	if err != nil {
		return fmt.Errorf("turn record is unreadable: %s: %v", turnPath, err)
	}
	state, err := readStateDoc(filepath.Join(dir, "state.json"))
	if err != nil {
		return err
	}
	if id, _ := turn["missionId"].(string); id != mission {
		return fmt.Errorf("turn record identity does not match --mission and --turn")
	}
	if id, _ := turn["turnId"].(string); id != turnID {
		return fmt.Errorf("turn record identity does not match --mission and --turn")
	}
	if id, _ := state["missionId"].(string); id != mission {
		return fmt.Errorf("mission state identity does not match --mission")
	}

	cycle, ok := promptTurnInt(turn["cycle"])
	if !ok {
		return fmt.Errorf("turn record field is invalid: cycle")
	}
	runtime, ok := turn["runtime"].(string)
	if !ok {
		return fmt.Errorf("turn record field is invalid: runtime")
	}
	model, ok := turn["model"].(string)
	if !ok {
		return fmt.Errorf("turn record field is invalid: model")
	}
	reconciliation, ok := turn["reconciliation"].(bool)
	if !ok {
		return fmt.Errorf("turn record field is invalid: reconciliation")
	}
	if cycle < 1 {
		return fmt.Errorf("turn record cycle must be positive")
	}
	if hostSession := turn["hostSession"]; hostSession != nil {
		if s, ok := hostSession.(string); !ok || s == "" {
			return fmt.Errorf("turn record hostSession must be a non-empty string or null")
		}
	}

	contractPath := filepath.Join(repo, "plans", fmt.Sprintf("mission-%s.contract.md", mission))
	contractData, err := os.ReadFile(contractPath)
	if err != nil {
		return fmt.Errorf("prompt authority artifact is unreadable: %v", err)
	}
	preambleData, err := os.ReadFile(filepath.Join(repo, "scripts", "agents", "roles", "orchestrator.md"))
	if err != nil {
		return fmt.Errorf("prompt authority artifact is unreadable: %v", err)
	}
	instructionData, err := os.ReadFile(filepath.Join(repo, "scripts", "agents", "templates", "host-turn-instruction.md"))
	if err != nil {
		return fmt.Errorf("prompt authority artifact is unreadable: %v", err)
	}
	contractText := string(contractData)
	values, err := promptAuthoredValues(contractText)
	if err != nil {
		return err
	}

	tailText, err := promptConfigValue(repo, "mission.ledger-tail-cycles", "5")
	if err != nil {
		return err
	}
	maximumText, err := promptConfigValue(repo, "mission.max-prompt-kb", "256")
	if err != nil {
		return err
	}
	tail, tailErr := strconv.Atoi(tailText)
	if !positiveIntRe.MatchString(tailText) || tailErr != nil || tail < 1 || tail > 50 {
		return fmt.Errorf("mission.ledger-tail-cycles must be an integer from 1 through 50")
	}
	if !positiveIntRe.MatchString(maximumText) {
		return fmt.Errorf("mission.max-prompt-kb must be a positive integer")
	}
	maximumKB, err := strconv.Atoi(maximumText)
	if err != nil {
		return fmt.Errorf("mission.max-prompt-kb must be a positive integer")
	}

	headroom, err := promptFenceHeadroom(repo, values, dir)
	if err != nil {
		return err
	}
	ledgerRecords, err := promptLedgerRecords(filepath.Join(dir, "ledger.md"), tail)
	if err != nil {
		return err
	}
	askRecords, err := promptAskRecords(filepath.Join(dir, "asks"))
	if err != nil {
		return err
	}
	answerRecords, err := promptAnswerRecords(filepath.Join(dir, "asks"), state)
	if err != nil {
		return err
	}
	streamRecords, err := promptStreamRecords(state)
	if err != nil {
		return err
	}
	reconRecords, err := promptReconciliationRecords(state, turnID, reconciliation)
	if err != nil {
		return err
	}
	// The Landed Returns list derives fresh from the tree and the turn log
	// at every assembly (plans/patience-orphan-usage.md): landed work is
	// inherited through the prompt, never through recorded surfacing state.
	turnLog, _ := state["turnLog"].([]any)
	landedRecords := LandedReturns(repo, mission, turnLog)

	reconYesNo := "no"
	if reconciliation {
		reconYesNo = "yes"
	}
	cycleText := strconv.FormatInt(cycle, 10)
	headers := strings.Join([]string{
		"Mission-Id: " + mission,
		"Turn-Id: " + turnID,
		"Cycle: " + cycleText,
		"Host-Session: " + promptOneLine(turn["hostSession"]),
		"Runtime: " + runtime,
		"Model: " + model,
		"Reconciliation: " + reconYesNo,
	}, "\n")
	thisTurn := strings.NewReplacer(
		"<cycle-number>", cycleText,
		"<fence-headroom>", headroom,
		"<yes | no>", reconYesNo,
	).Replace(string(instructionData))
	thisTurn = strings.TrimRight(thisTurn, "\n")
	// Patience breaches project into This Turn from the ledger's final cycle
	// block (plans/patience-satellite-4.md): the prompt is a pure function of
	// the ledger plus current chain-closed flags — restart-deterministic, no
	// runner memory. Detail lines whose chain root is now closed are dropped;
	// the overflow and excluded lines name no chains and are exempt.
	patienceLines := patiencePromptLines(
		filepath.Join(dir, "ledger.md"),
		filepath.Join(repo, "artifacts", "agents", "jobs"))
	if len(patienceLines) > 0 {
		thisTurn += "\n\n" + strings.Join(patienceLines, "\n")
	}

	blocks := []struct {
		name    string
		content string
	}{
		{"machine header", headers},
		{"orchestrator preamble", strings.TrimRight(string(preambleData), "\n")},
		{"## Mission Contract", "## Mission Contract\n" + strings.TrimRight(contractText, "\n")},
	}
	// The serving-goal orientation line: one optional block between the
	// mission intent and the streams, read through the goal parser. A
	// missing, absent, or degraded ledger produces NO line — prompt
	// assembly never degrades and never blocks on goal state.
	// Runner-side and runtime-neutral: every host of every runtime gets
	// the same line the same way.
	// Best-effort freshness first: a validated fetch may advance the
	// accepted ledger, and an offline or refused fetch keeps the stale
	// read under the same never-degrade rule — the prompt never blocks
	// on goal state.
	if endpoint, endpointErr := goal.ResolveEndpoint(repo); endpointErr == nil {
		_, _ = goal.Project(endpoint, true, time.Now())
	}
	if goalId, goalIntent, ok := (&goal.Store{Root: repo}).ServingProjection(); ok {
		block := "## Serving goal\n" + goalId + " — " + goalIntent
		blocks = append(blocks, struct {
			name    string
			content string
		}{"## Serving goal", block})
	}
	blocks = append(blocks, []struct {
		name    string
		content string
	}{
		{"## Ledger Tail", promptDataSection("## Ledger Tail", ledgerRecords)},
		{"## Human Answers", promptDataSection("## Human Answers", answerRecords)},
		{"## Open Asks", promptDataSection("## Open Asks", askRecords)},
		{"## Streams", promptDataSection("## Streams", streamRecords)},
		{"## Reconciliation", promptDataSection("## Reconciliation", reconRecords)},
		{"## Landed Returns", promptDataSection("## Landed Returns", landedRecords)},
		{"## This Turn", "## This Turn\n" + thisTurn},
	}...)
	parts := make([]string, len(blocks))
	for i, block := range blocks {
		parts[i] = block.content
	}
	prompt := []byte(strings.Join(parts, "\n\n") + "\n")

	maximum := maximumKB * 1024
	if len(prompt) > maximum {
		widest := blocks[0]
		for _, block := range blocks[1:] {
			if len(block.content) > len(widest.content) {
				widest = block
			}
		}
		return fmt.Errorf(
			"assembled prompt exceeds mission.max-prompt-kb (%s KiB); oversized block: %s",
			maximumText, widest.name)
	}
	return atomicWriteText(output, string(prompt))
}
