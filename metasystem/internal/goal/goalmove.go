package goal

import "strings"

// GoalRecordsOnly reports whether every changed path is the incident register
// or a goal page under the goals root, still present after the change and
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
// under the goals root, the only paths GoalRecordsOnly accepts.
func GoalRecordPath(name string) bool {
	return name == trunkRedPath || strings.HasPrefix(name, goalsPrefix) && strings.HasSuffix(name, ".md")
}
