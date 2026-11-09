package goal

import "strings"

// GoalRecordsOnly reports whether every changed path is the incident register
// or a goal page under the goals root or a concluded goal's record, still present and
// read without problems by the ledger's own parsers. after maps each changed
// installation-relative path to its new bytes.
func GoalRecordsOnly(after map[string][]byte) bool {
	for name, data := range after {
		if name == trunkRedPath {
			if _, problems := ParseTrunkRed(data); len(problems) != 0 {
				return false
			}
			continue
		}
		if !GoalRecordPath(name) {
			return false
		}
		if _, problems := ParseFile(data); len(problems) != 0 {
			return false
		}
	}
	return len(after) != 0
}

// GoalRecordPath reports whether name is the incident register or a goal page
// under the goals root or a concluded goal's record, the only paths GoalRecordsOnly accepts.
func GoalRecordPath(name string) bool {
	archive, recorded := strings.CutPrefix(name, recordsGoalsPrefix)
	return name == trunkRedPath || strings.HasPrefix(name, goalsPrefix) && strings.HasSuffix(name, ".md") ||
		recorded && strings.HasSuffix(archive, ".md") && !strings.Contains(archive, "/")
}

// ConcludedIn reports whether a deleted live goal keeps its identity in a record
// added or changed in the same move. It reads only the new record.
func ConcludedIn(deleted string, after map[string][]byte) bool {
	id, live := strings.CutPrefix(deleted, goalsPrefix)
	id, page := strings.CutSuffix(id, ".md")
	if !live || !page || id == "" || strings.Contains(id, "/") || deleted == trunkRedPath {
		return false
	}
	data, exists := after[recordsGoalsPrefix+id+".md"]
	if !exists {
		return false
	}
	f, problems := ParseFile(data)
	return len(problems) == 0 && f.Id == id
}
