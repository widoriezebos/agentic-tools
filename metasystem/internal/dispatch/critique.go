package dispatch

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// The critique-exhaustion rule: a critic chain that still has material
// findings open at a round divisible by three has exhausted its budget. The
// successor follow-up must enumerate every open finding id, the exhaustion is
// recorded once on the chain root, and a second exhaustion is refused
// outright — at that point waiting on the human is the only remedy.

const secondExhaustionRefused = "a second critique exhaustion is refused outright; waiting on the human is the only remedy"

// critiqueState is the record table one exhaustion decision reads: every
// parseable job record whose file name matches its own job id.
type critiqueState struct {
	agents  string
	records map[string]map[string]any
}

func loadCritiqueState(repoRoot string) critiqueState {
	agents := filepath.Join(repoRoot, "artifacts", "agents")
	state := critiqueState{agents: agents, records: map[string]map[string]any{}}
	paths, _ := filepath.Glob(filepath.Join(agents, "jobs", "*.json"))
	for _, path := range paths {
		record, err := readObject(path)
		if err != nil {
			continue
		}
		stem := strings.TrimSuffix(filepath.Base(path), ".json")
		if asString(record["jobId"]) == stem {
			state.records[stem] = record
		}
	}
	return state
}

// chainRoot resolves a job's lineage root within the loaded table, or ""
// when the walk leaves the table, cycles, or hits a malformed parent.
func (s critiqueState) chainRoot(job string) string {
	seen := map[string]bool{}
	for {
		record, present := s.records[job]
		if !present || seen[job] {
			return ""
		}
		seen[job] = true
		parent, hasParent := record["parentJob"]
		if !hasParent || parent == nil {
			return job
		}
		next, ok := parent.(string)
		if !ok {
			return ""
		}
		job = next
	}
}

// latestMember returns the chain's highest-round record among members with an
// integer round, or nil when the chain has none.
func (s critiqueState) latestMember(chain string) map[string]any {
	var ids []string
	for id := range s.records {
		if s.chainRoot(id) == chain {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	var best map[string]any
	bestRound := int64(0)
	for _, id := range ids {
		round, ok := numInt(s.records[id]["round"])
		if !ok {
			continue
		}
		if best == nil || round > bestRound {
			best, bestRound = s.records[id], round
		}
	}
	return best
}

// openMaterialIDs reads a completed critique round's return and lists its
// open material finding ids. The JOB RECORD owns round identity — a
// delegate-returned round is data and cannot decide whether the budget has
// elapsed. A round that failed with a protocol error, or has not completed,
// has no open findings to enumerate.
func (s critiqueState) openMaterialIDs(record map[string]any, chain string) (ids []string, round int64, err error) {
	round, ok := numInt(record["round"])
	if !ok || round < 1 {
		return nil, 0, fmt.Errorf("job record '%v' has an invalid round number", record["jobId"])
	}
	if asString(record["status"]) == "failed" && asString(record["error"]) == "protocol_error" {
		return nil, round, nil
	}
	if asString(record["status"]) != "completed" {
		return nil, round, nil
	}
	returnPath := filepath.Join(s.agents, chain, "rounds", fmt.Sprint(round), "return.json")
	result, err := readObject(returnPath)
	if err != nil {
		return nil, 0, fmt.Errorf("critique return for job '%v' is unreadable: %v", record["jobId"], err)
	}
	findings, ok := result["findings"].([]any)
	if !ok {
		return nil, 0, fmt.Errorf("critique return for job '%v' has no findings array", record["jobId"])
	}
	seen := map[string]bool{}
	for _, item := range findings {
		finding, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if material, ok := finding["material"].(bool); !ok || !material {
			continue
		}
		id := asString(finding["id"])
		if id != "" && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids, round, nil
}

// exhaustions reads a chain root's recorded exhaustions, refusing a record
// that already carries more than one.
func exhaustions(record map[string]any) ([]map[string]any, error) {
	value, present := record["critiqueExhaustions"]
	if !present {
		return nil, nil
	}
	list, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("critiqueExhaustions is malformed; waiting on the human is the only remedy")
	}
	if len(list) > 1 {
		return nil, errors.New(secondExhaustionRefused)
	}
	var entries []map[string]any
	for _, item := range list {
		entry, ok := item.(map[string]any)
		if !ok {
			return nil, errors.New(secondExhaustionRefused)
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// requireEnumeration refuses a successor message that does not name every
// open finding id as a standalone token.
func requireEnumeration(message string, openIDs []string) error {
	var missing []string
	for _, id := range openIDs {
		pattern := `(?:^|[^A-Za-z0-9_-])` + regexp.QuoteMeta(id) + `(?:$|[^A-Za-z0-9_-])`
		if !regexp.MustCompile(pattern).MatchString(message) {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("critique budget exhausted; the implementer or design successor follow-up "+
			"must enumerate every open finding identifier: %s", strings.Join(missing, ", "))
	}
	return nil
}

// exhaustionAction is one chain-root update the caller must record before
// the follow-up proceeds.
type exhaustionAction struct {
	jobID string
	entry map[string]any
}

// CritiqueExhaustionAction decides what a follow-up dispatch must do about
// critique exhaustion: nothing ("none"), or record the exhaustion entries in
// the written manifest ("record"). Any other outcome is a refusal explaining
// why the follow-up may not proceed.
func CritiqueExhaustionAction(repoRoot, rootJob, role, latestPath, messagePath, successor, outputPath string) (string, error) {
	state := loadCritiqueState(repoRoot)
	messageBytes, err := os.ReadFile(messagePath)
	if err != nil {
		return "", fmt.Errorf("critique exhaustion successor message is unreadable: %v", err)
	}
	message := string(messageBytes)

	latest, err := readObject(latestPath)
	if err != nil {
		return "", fmt.Errorf("latest follow-up job record is unreadable: %v", err)
	}
	if asString(latest["status"]) == "failed" && asString(latest["error"]) == "protocol_error" {
		// Protocol recovery deliberately does not read the missing or
		// malformed return that caused the protocol error.
		return "none", nil
	}

	loadRoot := func(chain, description string) (map[string]any, error) {
		if record, present := state.records[chain]; present {
			return record, nil
		}
		record, err := readObject(filepath.Join(state.agents, "jobs", chain+".json"))
		if err != nil {
			return nil, fmt.Errorf("%s is unreadable: %v", description, err)
		}
		return record, nil
	}

	newEntry := func(round int64, openIDs []string) map[string]any {
		ids := make([]any, len(openIDs))
		for i, id := range openIDs {
			ids[i] = id
		}
		return map[string]any{
			"round":          round,
			"openFindingIds": ids,
			"successorJobId": successor,
		}
	}

	var actions []exhaustionAction
	switch role {
	case "design-critic":
		openIDs, round, err := state.openMaterialIDs(latest, rootJob)
		if err != nil {
			return "", err
		}
		if len(openIDs) == 0 || round%3 != 0 {
			return "none", nil
		}
		current, err := loadRoot(rootJob, "critique root record")
		if err != nil {
			return "", err
		}
		previous, err := exhaustions(current)
		if err != nil {
			return "", err
		}
		if len(previous) > 0 {
			if roundOf(previous[0]) == round && asString(previous[0]["successorJobId"]) == successor {
				return "none", nil
			}
			return "", errors.New(secondExhaustionRefused)
		}
		if err := requireEnumeration(message, openIDs); err != nil {
			return "", err
		}
		actions = append(actions, exhaustionAction{rootJob, newEntry(round, openIDs)})

	case "code-critic":
		openIDs, round, err := state.openMaterialIDs(latest, rootJob)
		if err != nil {
			return "", err
		}
		if len(openIDs) == 0 || round%3 != 0 {
			return "none", nil
		}
		current, err := loadRoot(rootJob, "critique root record")
		if err != nil {
			return "", err
		}
		previous, err := exhaustions(current)
		if err != nil {
			return "", err
		}
		if len(previous) == 0 {
			return "", fmt.Errorf("code critique budget exhausted; dispatch an implementer follow-up that enumerates "+
				"every open finding identifier before continuing the code-critic chain: %s", strings.Join(openIDs, ", "))
		}
		if roundOf(previous[0]) != round {
			return "", errors.New(secondExhaustionRefused)
		}

	case "implementer":
		implementationIDs := map[string]bool{}
		for id := range state.records {
			if state.chainRoot(id) == rootJob {
				implementationIDs[id] = true
			}
		}
		var criticIDs []string
		for id, record := range state.records {
			if asString(record["role"]) == "code-critic" && record["parentJob"] == nil &&
				implementationIDs[asString(record["reviews"])] {
				criticIDs = append(criticIDs, id)
			}
		}
		sort.Strings(criticIDs)
		for _, criticID := range criticIDs {
			criticLatest := state.latestMember(criticID)
			if criticLatest == nil {
				continue
			}
			openIDs, round, err := state.openMaterialIDs(criticLatest, criticID)
			if err != nil {
				return "", err
			}
			if len(openIDs) == 0 || round%3 != 0 {
				continue
			}
			previous, err := exhaustions(state.records[criticID])
			if err != nil {
				return "", err
			}
			if len(previous) > 0 {
				if roundOf(previous[0]) == round {
					continue
				}
				return "", errors.New(secondExhaustionRefused)
			}
			if err := requireEnumeration(message, openIDs); err != nil {
				return "", err
			}
			actions = append(actions, exhaustionAction{criticID, newEntry(round, openIDs)})
		}

	default:
		return "", fmt.Errorf("critique exhaustion has no rule for role %s", role)
	}

	if len(actions) == 0 {
		return "none", nil
	}
	manifest := make([]any, len(actions))
	for i, action := range actions {
		manifest[i] = map[string]any{
			"jobId":               action.jobID,
			"critiqueExhaustions": []any{action.entry},
		}
	}
	if err := writeCompactJSON(outputPath, map[string]any{"records": manifest}); err != nil {
		return "", err
	}
	return "record", nil
}

// roundOf reads an exhaustion entry's round, or -1 when it has none.
func roundOf(entry map[string]any) int64 {
	if round, ok := numInt(entry["round"]); ok {
		return round
	}
	return -1
}

// ExhaustionPatches materializes one record patch per manifest entry and
// lists "jobId<TAB>patchPath" lines for the caller to apply through the
// record CAS.
func ExhaustionPatches(manifestPath, dir string) ([]string, error) {
	manifest, err := readObject(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("exhaustion manifest is unreadable: %v", err)
	}
	items, ok := manifest["records"].([]any)
	if !ok {
		return nil, fmt.Errorf("exhaustion manifest has no records array")
	}
	var lines []string
	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("exhaustion manifest entry is not an object")
		}
		jobID := asString(item["jobId"])
		entries, present := item["critiqueExhaustions"]
		if jobID == "" || !present {
			return nil, fmt.Errorf("exhaustion manifest entry is missing jobId or critiqueExhaustions")
		}
		patch, err := os.CreateTemp(dir, "exhaustion-record.*")
		if err != nil {
			return nil, err
		}
		name := patch.Name()
		patch.Close()
		if err := writeCompactJSON(name, map[string]any{"critiqueExhaustions": entries}); err != nil {
			return nil, err
		}
		lines = append(lines, jobID+"\t"+name)
	}
	return lines, nil
}
