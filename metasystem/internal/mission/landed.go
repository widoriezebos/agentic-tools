package mission

import (
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/boundedexec"
)

// The Landed Returns derivation (plans/patience-orphan-usage.md): the
// delegate rounds whose return landed on disk but which no concluded turn's
// host-authored record has acted on. The list is a pure function of the tree
// and the turn log, derived fresh at every prompt assembly and once more at
// the completion conclude — there is no recorded surfacing state, so nothing
// can go stale, no scan can race a late return, and old missions need no
// migration. A round retires only by the host's own recorded action: its
// jobId in a concluded turn's certified entries, or an accepted dispatch
// claim naming a later round of its chain. Chain closure never excludes a
// row: the runner closes chains at every park, and the park that orphans a
// return must not also hide it.

// landedRowMax bounds the section. When 20 or fewer chains qualify every
// qualifying row is emitted; when more qualify, exactly 19 data rows (the
// first 19 in sort order) plus one final overflow summary row are emitted.
const landedRowMax = 20

// LandedReturns derives the Landed Returns rows for a mission: at most one
// row per mission-owned chain — the latest landed round the host has neither
// certified nor superseded — each row three fields:
// chain-root, round-or-marker, return-path-or-none. The markers are
// `invalid` (the return exists but fails the role checker), `unreadable`
// (the chain's artifacts cannot be read), and `overflow` (the bound row,
// whose second field is the count of chains left unlisted). Rows sort by
// (chain root, round), both ascending. Derivation is tolerant end to end —
// what cannot be read is skipped or marked, never a hard failure — because
// silence is the failure mode this list exists to close.
func LandedReturns(repo, missionID string, turnLog []any) [][]string {
	records := readLandedJobRecords(filepath.Join(repo, "artifacts", "agents", "jobs"))
	reserved := landedReservedJobIDs(repo, missionID)
	certified, dispatched := hostActedRecords(turnLog)

	// Group the readable records into chains; a record whose parent walk
	// breaks belongs to no chain and neither lists nor blocks one.
	chains := map[string][]string{}
	rootByID := map[string]string{}
	for _, id := range sortedRecordIDs(records) {
		root, ok := chainRootOf(records, id)
		if !ok {
			continue
		}
		rootByID[id] = root
		chains[root] = append(chains[root], id)
	}

	// The successor-dispatch boundary: the highest round any concluded
	// turn's accepted dispatch claim names, per chain.
	claimedRound := map[string]int64{}
	for id := range dispatched {
		record := records[id]
		root, ok := rootByID[id]
		if record == nil || !ok {
			continue
		}
		if round, ok := intValue(record["round"]); ok && round > claimedRound[root] {
			claimedRound[root] = round
		}
	}

	type candidate struct {
		row   []string
		jobID string // empty for marker rows that need no validation
	}
	var candidates []candidate
	roots := make([]string, 0, len(chains))
	for root := range chains {
		roots = append(roots, root)
	}
	sort.Strings(roots)
	for _, root := range roots {
		members := chains[root]
		if !chainOwnedByMission(records, members, missionID, reserved) {
			continue
		}
		bestID := ""
		var bestRound int64
		unreadable := false
		for _, id := range members {
			round, ok := intValue(records[id]["round"])
			if !ok || round < 1 {
				continue
			}
			if certified[id] || claimedRound[root] > round {
				// The host's own recorded action retires the round.
				continue
			}
			returnPath := filepath.Join(repo, "artifacts", "agents", root,
				"rounds", strconv.FormatInt(round, 10), "return.json")
			if _, err := os.Stat(returnPath); err != nil {
				if os.IsNotExist(err) {
					continue // never landed
				}
				unreadable = true
				break
			}
			if bestID == "" || round > bestRound {
				bestID, bestRound = id, round
			}
		}
		switch {
		case unreadable:
			candidates = append(candidates, candidate{row: []string{root, "unreadable", "none"}})
		case bestID != "":
			rel := path.Join("artifacts", "agents", root, "rounds",
				strconv.FormatInt(bestRound, 10), "return.json")
			candidates = append(candidates,
				candidate{row: []string{root, strconv.FormatInt(bestRound, 10), rel}, jobID: bestID})
		}
	}

	if len(candidates) > landedRowMax {
		remaining := len(candidates) - (landedRowMax - 1)
		candidates = append(candidates[:landedRowMax-1:landedRowMax-1], candidate{
			row: []string{"overflow", strconv.Itoa(remaining), "none"},
		})
	}
	// Validation runs last and only over emitted rows: a failed check turns
	// the round field into the invalid marker without changing membership,
	// order, or the bound.
	rows := make([][]string, 0, len(candidates))
	for _, item := range candidates {
		if item.jobID != "" && !landedRoundValid(repo, item.jobID) {
			item.row[1] = "invalid"
		}
		rows = append(rows, item.row)
	}
	return rows
}

// landedRoundValid runs the shipped job-mode return checker through the
// repository's own script, so return-schema authority stays in one place. A
// checker that cannot run proves nothing, and an unproven return lists as
// invalid rather than as ready.
func landedRoundValid(repo, jobID string) bool {
	cmd := exec.Command(filepath.Join(repo, "scripts", "assert-return-complete.sh"), "--job", jobID)
	cmd.Dir = repo
	// Bounded: a wedged checker must not freeze prompt assembly; a
	// checker that ran out of its bound proved nothing, so the return
	// lists as invalid.
	limit := boundedexec.Timeout(filepath.Join(repo, "metasystem.conf"), boundedexec.Local)
	return boundedexec.Run(cmd, limit, "return checker for job "+jobID) == nil
}

// readLandedJobRecords reads every readable job record by its id. Unreadable
// records are skipped: they can prove neither membership nor a landed round.
func readLandedJobRecords(jobsDir string) map[string]map[string]any {
	paths, _ := filepath.Glob(filepath.Join(jobsDir, "*.json"))
	sort.Strings(paths)
	records := map[string]map[string]any{}
	for _, recordPath := range paths {
		record, err := readJSONObjectFile(recordPath)
		if err != nil {
			continue
		}
		id, _ := record["jobId"].(string)
		if id == "" {
			id = strings.TrimSuffix(filepath.Base(recordPath), ".json")
		}
		if _, exists := records[id]; !exists {
			records[id] = record
		}
	}
	return records
}

func sortedRecordIDs(records map[string]map[string]any) []string {
	ids := make([]string, 0, len(records))
	for id := range records {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// landedReservedJobIDs reads the mission's fence reservations, whose keys
// also stamp an unstamped setup husk as mission-owned (the durable boundary).
func landedReservedJobIDs(repo, missionID string) map[string]bool {
	reserved := map[string]bool{}
	fences, err := readJSONObjectFile(filepath.Join(missionDir(repo, missionID), "fences.json"))
	if err != nil {
		return reserved
	}
	reservations, _ := fences["reservations"].(map[string]any)
	for job := range reservations {
		reserved[job] = true
	}
	return reserved
}

// chainRootOf walks a record's parentJob lineage to its root among the
// readable records. A broken, cyclic, or non-string ancestry has no root.
func chainRootOf(records map[string]map[string]any, id string) (string, bool) {
	seen := map[string]bool{}
	current := id
	for {
		if seen[current] {
			return "", false
		}
		seen[current] = true
		record := records[current]
		if record == nil {
			return "", false
		}
		parent, present := record["parentJob"]
		if !present || parent == nil {
			return current, true
		}
		name, ok := parent.(string)
		if !ok || name == "" {
			return "", false
		}
		current = name
	}
}

// chainOwnedByMission reports whether every chain member belongs to the
// mission: stamped for it, or an unstamped husk its fence reservations name.
func chainOwnedByMission(records map[string]map[string]any, members []string, missionID string, reserved map[string]bool) bool {
	for _, id := range members {
		stamped, _ := records[id]["mission"].(string)
		if stamped == missionID || (stamped == "" && reserved[id]) {
			continue
		}
		return false
	}
	return true
}

// hostActedRecords collects the two host-authored "I acted" records from the
// turn log: every concluded turn's certified jobIds and its accepted
// dispatched jobIds. Faulted and failed turns carry neither, so only real
// conclusions retire a row.
func hostActedRecords(turnLog []any) (certified, dispatched map[string]bool) {
	certified, dispatched = map[string]bool{}, map[string]bool{}
	for _, raw := range turnLog {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if list, ok := entry["certified"].([]any); ok {
			for _, item := range list {
				if value, ok := item.(map[string]any); ok {
					if id, _ := value["jobId"].(string); id != "" {
						certified[id] = true
					}
				}
			}
		}
		if list, ok := entry["accepted"].([]any); ok {
			for _, item := range list {
				claim, ok := item.(map[string]any)
				if !ok {
					continue
				}
				if kind, _ := claim["kind"].(string); kind != "dispatched" {
					continue
				}
				value, _ := claim["value"].(map[string]any)
				if id, _ := value["jobId"].(string); id != "" {
					dispatched[id] = true
				}
			}
		}
	}
	return certified, dispatched
}
