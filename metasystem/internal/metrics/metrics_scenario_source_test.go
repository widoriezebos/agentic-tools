package metrics

import (
	"fmt"
	"strings"
)

const (
	sourceLegacy   = "1111111111111111111111111111111111111111"
	sourceMove     = "2222222222222222222222222222222222222222"
	sourceCurrent  = "3333333333333333333333333333333333333333"
	sourceInvalid  = "4444444444444444444444444444444444444444"
	sourceNoBudget = "5555555555555555555555555555555555555555"
	sourceAnchor   = "6666666666666666666666666666666666666666"
	sourceMetrics  = "7777777777777777777777777777777777777777"
	sourceUnknown  = "8888888888888888888888888888888888888888"
	sourceShared   = "9999999999999999999999999999999999999999"
	sourceSharedG2 = "abababababababababababababababababababab"
)

// The records below are raw Git stdout. Their callers supply the historical
// parents, authors, paths, line counts, and receipt additions under test.
func appendRawCommit(s sourceSnapshot, sha, parent, email, at string, numstats ...string) sourceSnapshot {
	s.mainLog += "\x1e" + sha + "\x1f" + parent + "\x1f" + email + "\x1f" + at + "\n\n" + strings.Join(numstats, "\n") + "\n"
	s.mainTip = sha
	return s
}

func appendRawReceiptPatch(s sourceSnapshot, sha, path string, before int, created bool, added ...string) sourceSnapshot {
	patch := "\x1e" + sha + "\n\ndiff --git a/" + path + " b/" + path + "\n"
	if created {
		patch += "new file mode 100644\nindex 0000000..1111111\n--- /dev/null\n"
	} else {
		patch += "index 1111111..2222222 100644\n--- a/" + path + "\n"
	}
	patch += "+++ b/" + path + "\n"
	if len(added) == 1 {
		patch += fmt.Sprintf("@@ -%d,0 +%d @@\n", before, before+1)
	} else {
		patch += fmt.Sprintf("@@ -%d,0 +%d,%d @@\n", before, before+1, len(added))
	}
	for _, line := range added {
		patch += "+" + line + "\n"
	}
	if path == "metasystem/plans/receipts.log" {
		s.legacyPatch += patch
	} else {
		s.currentPatch += patch
	}
	return s
}

func withCommittedGoal(s sourceSnapshot, path string, content []byte) sourceSnapshot {
	files := make(map[string][]byte, len(s.goalFiles)+1)
	for name, data := range s.goalFiles {
		files[name] = append([]byte(nil), data...)
	}
	files[path] = append([]byte(nil), content...)
	s.goalFiles = files
	return s
}
