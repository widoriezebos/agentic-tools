package missionrunner

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/contract"
)

// Contract reading and the pure per-turn context decisions: which prior
// session a turn resumes, what the last measured metrics were, and whether a
// contract's fences are reached.

var (
	authoredBlockRe = regexp.MustCompile("(?ms)^```mission[ \t]*\n(.*?)^```[ \t]*$")
	sealBlockRe     = regexp.MustCompile("(?ms)^```mission-seal[ \t]*\n(.*?)^```[ \t]*$")
)

// parseContractText splits a contract into its one authored block and one
// generated seal block and parses both key=value grammars.
func parseContractText(text string) (authored, seal map[string]string, err error) {
	authoredBlocks := authoredBlockRe.FindAllStringSubmatch(text, -1)
	sealBlocks := sealBlockRe.FindAllStringSubmatch(text, -1)
	if len(authoredBlocks) != 1 || len(sealBlocks) != 1 {
		return nil, nil, failf(3, "mission contract lacks one authored block and one generated seal")
	}
	if authored, err = contractKeyValues(authoredBlocks[0][1]); err != nil {
		return nil, nil, err
	}
	if seal, err = contractKeyValues(sealBlocks[0][1]); err != nil {
		return nil, nil, err
	}
	return authored, seal, nil
}

// contractKeyValues parses one fenced block's key=value lines under the
// contract owner's ONE grammar (review mission-contract-4).
func contractKeyValues(block string) (map[string]string, error) {
	values, err := contract.ParseAuthoredValues(block, "mission contract")
	if err != nil {
		return nil, failf(3, "mission contract key/value grammar is invalid: %v", err)
	}
	return values, nil
}

// parseContract reads the mission contract — the pinned approved snapshot by
// default, the authored file in plans/ otherwise — and returns its text and
// both parsed blocks. The approved snapshot must still match the sha the
// fence counters pinned at start, or the mission is running against bytes
// nobody approved.
func (e *Engine) parseContract(approved bool) (string, map[string]string, map[string]string, error) {
	path := e.contractPath()
	if approved {
		path = e.approvedContractPath()
	}
	return e.parseContractAt(path, approved)
}

// parseContractAt parses a contract at an explicit path — the launch
// gate reads the PREFLIGHT-VERIFIED snapshot this way, so its values
// come from the bytes preflight verified, never a reread of the mutable
// authored file.
func (e *Engine) parseContractAt(path string, approved bool) (string, map[string]string, map[string]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", nil, nil, failf(3, "mission contract is unreadable: %s: %v", path, err)
	}
	if !utf8.Valid(raw) {
		return "", nil, nil, failf(3, "mission contract is not UTF-8: %s", path)
	}
	if approved {
		fences, err := readDocLabeled(e.fencesPath(), "mission fence counters", 3)
		if err != nil {
			return "", nil, nil, err
		}
		expected, ok := fences["approvedContractSha256"].(string)
		sum := sha256.Sum256(raw)
		if !ok || hex.EncodeToString(sum[:]) != expected {
			return "", nil, nil, failf(3, "approved mission contract snapshot does not match approvedContractSha256")
		}
	}
	text := string(raw)
	authored, seal, err := parseContractText(text)
	if err != nil {
		return "", nil, nil, err
	}
	return text, authored, seal, nil
}

// fencesPath is the mission's fence-counter file.
func (e *Engine) fencesPath() string {
	return filepath.Join(e.missionDir(), "fences.json")
}

// PriorContext reads the turn log's tail into the next turn's launch context:
// the host session to resume (nil after an unresumable turn), whether the
// turn is a reconciliation (the last turn did not complete), and how many
// consecutive turns have failed. Unresumable turns never count, and neither
// does a turn recorded as feeding no host-failure blame (feedsBreaker false —
// a no-witness session mismatch convicts nobody): both are skipped without
// resetting the count.
func PriorContext(turnLog []any) (hostSession any, reconciliation bool, failures int) {
	if len(turnLog) == 0 {
		return nil, false, 0
	}
	last, _ := turnLog[len(turnLog)-1].(map[string]any)
	if session, ok := last["sessionId"].(string); ok {
		hostSession = session
	}
	outcome := last["outcome"]
	reconciliation = outcome != "completed" && outcome != "return-ok"
	for index := len(turnLog) - 1; index >= 0; index-- {
		entry, _ := turnLog[index].(map[string]any)
		if entry["feedsBreaker"] == false {
			continue
		}
		switch entry["outcome"] {
		case "completed", "return-ok":
			return sessionAfter(outcome, hostSession), reconciliation, failures
		case "unresumable":
			continue
		default:
			failures++
		}
	}
	return sessionAfter(outcome, hostSession), reconciliation, failures
}

// sessionAfter drops the session when the last turn declared it unresumable.
func sessionAfter(lastOutcome, session any) any {
	if lastOutcome == "unresumable" {
		return nil
	}
	return session
}

// PreviousMetrics finds the per-metric values a regression is judged against:
// the most recent turn whose measurement carries every declared metric. Nil
// lets the gate measure against the sealed baseline.
func PreviousMetrics(turnLog []any, names []string) map[string]string {
	for index := len(turnLog) - 1; index >= 0; index-- {
		entry, _ := turnLog[index].(map[string]any)
		measurement, ok := entry["measurement"].(map[string]any)
		if !ok {
			continue
		}
		metrics, ok := measurement["metrics"].(map[string]any)
		if !ok {
			continue
		}
		previous := map[string]string{}
		complete := true
		for _, name := range names {
			value, present := metrics[name]
			if !present {
				complete = false
				break
			}
			previous[name] = valueString(value)
		}
		if complete {
			return previous
		}
	}
	return nil
}

// gateMetricNames lists a contract's declared gate metrics in stable order.
func gateMetricNames(values map[string]string) []string {
	names := []string{}
	for key := range values {
		if strings.HasPrefix(key, "gate.threshold.") {
			names = append(names, strings.TrimPrefix(key, "gate.threshold."))
		}
	}
	sort.Strings(names)
	return names
}

// fenceReachedAt decides whether any of a contract's fences is reached: wall
// clock, spent cycles, total job reservations, or concurrently active jobs.
// jobStatus maps a reserved job id to its recorded status; a reservation with
// no readable record counts as active, because losing sight of a job must
// never relax a fence.
func fenceReachedAt(fences map[string]any, values map[string]string, jobStatus map[string]string, now time.Time) (bool, []string, error) {
	startedAt, _ := fences["startedAt"].(string)
	started, err := time.Parse(time.RFC3339, startedAt)
	if err != nil {
		return false, nil, failf(3, "mission fence counters carry an invalid startedAt: %v", err)
	}
	elapsedHours := now.Sub(started).Seconds() / 3600
	wallClock, err := strconv.ParseFloat(values["fence.wall-clock-hours"], 64)
	if err != nil {
		return false, nil, failf(3, "mission contract fence.wall-clock-hours is not numeric")
	}
	cycles, ok := jsonInt(fences["cycles"])
	if !ok {
		return false, nil, failf(3, "mission fence counters carry an invalid cycle count")
	}
	limits := map[string]int{}
	for _, name := range []string{"fence.cycles", "fence.jobs", "fence.concurrency"} {
		limit, err := intFromString(values[name])
		if err != nil {
			return false, nil, failf(3, "mission contract %s is not an integer", name)
		}
		limits[name] = limit
	}
	reservations, _ := fences["reservations"].(map[string]any)
	active := 0
	for job := range reservations {
		if !terminalJobStatuses[jobStatus[job]] {
			active++
		}
	}
	var names []string
	if elapsedHours >= wallClock {
		names = append(names, "fence.wall-clock-hours")
	}
	if cycles >= int64(limits["fence.cycles"]) {
		names = append(names, "fence.cycles")
	}
	if len(reservations) >= limits["fence.jobs"] {
		names = append(names, "fence.jobs")
	}
	if active >= limits["fence.concurrency"] {
		names = append(names, "fence.concurrency")
	}
	return len(names) > 0, names, nil
}

// missionJobStatuses maps each of the mission's job records (by file stem) to
// its recorded status, for the fence concurrency reading.
func missionJobStatuses(root, mission string) map[string]string {
	statuses := map[string]string{}
	for _, record := range missionJobs(root, mission) {
		status, _ := record.doc["status"].(string)
		statuses[jobRecordStem(record)] = status
	}
	return statuses
}

// turnCapFromDoc reads a turn record's cap as a wall-clock allowance.
func turnCapFromDoc(doc map[string]any) (time.Duration, error) {
	capMin, ok := jsonInt(doc["turnCapMin"])
	if !ok {
		return 0, failf(3, "turn record turnCapMin is invalid")
	}
	return time.Duration(capMin) * time.Minute, nil
}
