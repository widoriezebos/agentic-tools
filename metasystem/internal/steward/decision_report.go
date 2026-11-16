package steward

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/shellquote"
)

func decisionsDeliveredPath(home string) string {
	return filepath.Join(filepath.Dir(humanauthority.DecisionLogPath(home)), "decisions-delivered.json")
}

func readDeliveredDecisions(home string) (map[string]bool, error) {
	delivered := map[string]bool{}
	data, err := os.ReadFile(decisionsDeliveredPath(home))
	if os.IsNotExist(err) {
		return delivered, nil
	}
	if err == nil {
		err = json.Unmarshal(data, &delivered)
	}
	if delivered == nil {
		delivered = map[string]bool{}
	}
	return delivered, err
}

// DeliverDecisions posts grant decisions from completed local days and records
// their ids only after delivery succeeds. Late appends remain eligible.
func DeliverDecisions(home string, loc *time.Location, now time.Time, post func(string) error) (int, error) {
	if loc == nil {
		loc = time.Local
	}
	decisions, _, err := humanauthority.ReadDecisions(home)
	if err != nil {
		return 0, err
	}
	delivered, err := readDeliveredDecisions(home)
	if err != nil {
		return 0, fmt.Errorf("read delivered decisions: %w", err)
	}
	today := now.In(loc).Format("2006-01-02")
	decisions = slices.DeleteFunc(decisions, func(d humanauthority.Decision) bool {
		return d.Grant == "" || delivered[d.ID] || d.At.In(loc).Format("2006-01-02") >= today
	})
	if len(decisions) == 0 {
		return 0, nil
	}
	// Several seats share the host log; serialize delivery and its marker.
	held, err := lock.File(decisionsDeliveredPath(home)+".lock", 0o600, lock.Exclusive)
	if err != nil {
		return 0, err
	}
	defer held.Release()
	delivered, err = readDeliveredDecisions(home)
	if err != nil {
		return 0, fmt.Errorf("read delivered decisions: %w", err)
	}
	decisions = slices.DeleteFunc(decisions, func(d humanauthority.Decision) bool { return delivered[d.ID] })
	if len(decisions) == 0 {
		return 0, nil
	}
	slices.SortStableFunc(decisions, func(a, b humanauthority.Decision) int { return a.At.Compare(b.At) })
	var lines []string
	day := ""
	for _, d := range decisions {
		at := d.At.In(loc)
		if next := at.Format("2006-01-02"); next != day {
			day = next
			lines = append(lines, day)
		}
		act := make([]string, len(d.Argv))
		for i, word := range d.Argv {
			act[i] = shellquote.Word(word)
		}
		overrule, _, _ := humanauthority.OverruleLine(d)
		line := fmt.Sprintf("%s %s: %s -> %s", at.Format("15:04"), d.Class, strings.Join(act, " "), d.Outcome)
		if overrule != "" {
			line += "; overrule: " + overrule
		}
		lines = append(lines, line)
	}
	if err := post(strings.Join(lines, "\n")); err != nil {
		return 0, err
	}
	for _, d := range decisions {
		delivered[d.ID] = true
	}
	data, err := json.Marshal(delivered)
	if err == nil {
		_, err = atomicfile.WriteFile(decisionsDeliveredPath(home), append(data, '\n'), 0o600, home)
	}
	return len(decisions), err
}
