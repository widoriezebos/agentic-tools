package branch

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
)

type Kind string

const (
	Unit Kind = "unit"
	Plan Kind = "plan"
	Read Kind = "read"
	Drop Kind = "drop"
)

type Class string

const (
	ClassExcluded    Class = "excluded"
	ClassRead        Class = "read"
	ClassReadClosure Class = "read-closure"
	ClassReadProse   Class = "read-prose"
	ClassPlan        Class = "plan"
	ClassUnit        Class = "unit"
)

type KindInfo struct {
	Whole    bool
	Kind     Kind
	Units    []string
	Unit     string
	CommitID string

	Operation string
}
type Commit struct {
	Whole  bool
	ID     string
	Kind   Kind
	Units  []string
	Unit   string
	Digest string
}

const RangeCode = "GOAL_BRANCH_RANGE"

// commitWord names a goal branch commit of kind in a person's words.
func commitWord(kind Kind) string {
	switch kind {
	case Unit:
		return "build"
	case Read:
		return "review commit"
	}
	return string(kind) + " commit"
}

// RangeError is a goal branch that breaks its shape at one commit. Remedy
// is its line 2: the command that shows the goal's work, where the goal is
// known ("Messages a Person Reads"). Trailer, when set, is the kind trailer
// the commit lacks; only a verb that knows the commit is the goal branch's
// tip, checked out in the goal worktree, may offer the amend that adds it.
type RangeError struct {
	Code, Commit, Reason, Remedy string
	Trailer                      string
}

func (e *RangeError) Error() string {
	if e.Remedy == "" {
		return fmt.Sprintf("commit %s: %s", e.Commit, e.Reason)
	}
	return fmt.Sprintf("commit %s: %s\n%s", e.Commit, e.Reason, e.Remedy)
}

// RefusalCode is the refusal's code.
func (e *RangeError) RefusalCode() string { return e.Code }

// rangeRefusal is a range refusal on goalID's branch; its remedy shows the
// goal's work, or checks this installation when the goal is not known.
func rangeRefusal(goalID, commit, reason string) error {
	remedy := "run: metasystem system check"
	if goalID != "" {
		remedy = "run: metasystem work status " + goalID
	}
	return &RangeError{Code: RangeCode, Commit: commit, Reason: reason, Remedy: remedy}
}

// missingKindRefusal is a commit on goalID's branch with no Goal-Unit,
// Goal-Plan or Goal-Read trailer: its words name the trailer a build needs,
// with the goal filled in (the unit is the person's).
func missingKindRefusal(goalID, commit string) error {
	trailer := "Goal-Unit: " + goalID + "/UNIT"
	refusal := rangeRefusal(goalID, commit, "it doesn't say which goal and unit it builds (no trailer "+trailer+")").(*RangeError)
	refusal.Trailer = trailer
	return refusal
}

func parseUnits(value string) ([]string, bool) {
	parts := strings.Split(value, "+")
	seen := map[string]bool{}
	for _, unit := range parts {
		if unit == "" || strings.Contains(unit, "/") || strings.IndexFunc(unit, unicode.IsSpace) >= 0 || seen[unit] {
			return nil, false
		}
		seen[unit] = true
	}
	return parts, true
}

func splitGoalUnits(value string) (string, []string, bool) {
	goal, list, ok := strings.Cut(value, "/")
	units, valid := parseUnits(list)
	return goal, units, ok && goal != "" && valid
}

func unitList(units []string) string { return strings.Join(units, "+") }

func sameUnits(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for index := range a {
		if a[index] != b[index] {
			return false
		}
	}
	return true
}

func KindOf(repo, commit, goalID string) (KindInfo, error) {
	return kindOfWithGit(repo, commit, goalID, gitOutput)
}

func kindOfWithGit(repo, commit, goalID string, gitRead func(string, ...string) ([]byte, error)) (KindInfo, error) {
	out, err := gitRead(repo, "show", "-s", "--format=%(trailers:only,unfold=true)", commit)
	if err != nil {
		return KindInfo{}, err
	}
	var trailers [][2]string
	whole := false
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		key, value, ok := strings.Cut(line, ":")
		if ok && key == "Goal-Whole" && strings.TrimSpace(value) == goalID {
			whole = true
		}
		if ok && (key == "Goal-Unit" || key == "Goal-Plan" || key == "Goal-Read" || key == "Goal-Drop") {
			trailers = append(trailers, [2]string{key, strings.TrimSpace(value)})
		}
	}
	if len(trailers) == 0 && goalID != "" {
		return KindInfo{}, missingKindRefusal(goalID, commit)
	}
	if len(trailers) != 1 {
		return KindInfo{}, rangeRefusal(goalID, commit, fmt.Sprintf("it should say whether it is a build, a plan or a review, and it says so %d times", len(trailers)))
	}
	key, value := trailers[0][0], trailers[0][1]
	if key == "Goal-Plan" {
		if value != goalID {
			return KindInfo{}, rangeRefusal(goalID, commit, fmt.Sprintf("it is a plan of goal %q, not of %q", value, goalID))
		}
		return KindInfo{Kind: Plan}, nil
	}
	fields := strings.Fields(value)
	if (key == "Goal-Read" || key == "Goal-Drop") && len(fields) != 2 {
		return KindInfo{}, rangeRefusal(goalID, commit, "its review line is damaged")
	}
	first := value
	if key == "Goal-Read" || key == "Goal-Drop" {
		first = fields[0]
	}
	goal, units, ok := splitGoalUnits(first)
	if !ok {
		return KindInfo{}, rangeRefusal(goalID, commit, "its "+key+" line is damaged")
	}
	if goal != goalID {
		return KindInfo{}, rangeRefusal(goalID, commit, fmt.Sprintf("it belongs to goal %q, not to %q (%s)", goal, goalID, key))
	}
	if key == "Goal-Unit" {
		return KindInfo{Kind: Unit, Units: units, Unit: unitList(units), Whole: whole}, nil
	}
	if key == "Goal-Drop" {
		if len(units) != 1 || whole || !validName(fields[1]) {
			return KindInfo{}, rangeRefusal(goalID, commit, "a drop names one unit and takes no whole option")
		}
		return KindInfo{Kind: Drop, Units: units, Unit: units[0], Operation: fields[1]}, nil
	}
	if !hex40(fields[1]) {
		return KindInfo{}, rangeRefusal(goalID, commit, "its review line names the reviewed commit by a short id")
	}
	return KindInfo{Kind: Read, Units: units, Unit: unitList(units), CommitID: fields[1]}, nil
}

func hex40(value string) bool {
	if len(value) != 40 {
		return false
	}
	for _, c := range value {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return false
		}
	}
	return true
}

func readPath(path string) (goal, commit string, ok bool) {
	rest, ok := strings.CutPrefix(path, "metasystem/records/reads/")
	if !ok {
		return "", "", false
	}
	goal, file, ok := strings.Cut(rest, "/")
	commit, ok2 := strings.CutSuffix(file, ".json")
	return goal, commit, ok && ok2 && goal != "" && !strings.Contains(file, "/") && hex40(commit)
}

func readClosurePath(path string) (goal, commit string, ok bool) {
	rest, ok := strings.CutPrefix(path, "metasystem/records/reads/")
	if !ok {
		return "", "", false
	}
	goal, file, ok := strings.Cut(rest, "/")
	commit, ok2 := strings.CutSuffix(file, ".closure.json")
	return goal, commit, ok && ok2 && goal != "" && !strings.Contains(file, "/") && hex40(commit)
}

func PathClass(path string) Class {
	for _, excluded := range landing.WorkspaceExclusions() {
		name := "metasystem/" + strings.TrimSuffix(excluded, "/")
		if path == name || strings.HasPrefix(path, name+"/") {
			return ClassExcluded
		}
	}
	if _, _, ok := readPath(path); ok {
		return ClassRead
	}
	if _, _, ok := readClosurePath(path); ok {
		return ClassReadClosure
	}
	if path == "metasystem/records/reads" || strings.HasPrefix(path, "metasystem/records/reads/") {
		return ClassExcluded
	}
	if strings.HasPrefix(path, "metasystem/records/misc/") {
		return ClassReadProse
	}
	if path == "metasystem/plans" || strings.HasPrefix(path, "metasystem/plans/") || path == "metasystem/records" || strings.HasPrefix(path, "metasystem/records/") {
		return ClassPlan
	}
	return ClassUnit
}

func ValidateRange(repo, endpointTip, tip, goalID string) ([]Commit, error) {
	return validateRangeWithGit(repo, endpointTip, tip, goalID, gitOutput)
}

func ValidateRangeWithGit(repo, endpointTip, tip, goalID string, gitRead func(string, ...string) ([]byte, error)) ([]Commit, error) {
	return validateRangeWithGit(repo, endpointTip, tip, goalID, gitRead)
}

func KindOfWithRaw(repo, commit, goalID string, read func(string, ...string) ([]byte, error)) (KindInfo, error) {
	return kindOfWithGit(repo, commit, goalID, read)
}

func validateRangeWithGit(repo, endpointTip, tip, goalID string, gitRead func(string, ...string) ([]byte, error)) ([]Commit, error) {
	baseOut, err := gitRead(repo, "merge-base", endpointTip, tip)
	if err != nil || strings.TrimSpace(string(baseOut)) == "" {
		return nil, rangeRefusal(goalID, tip, "the goal branch shares no history with main")
	}
	base := strings.TrimSpace(string(baseOut))
	out, err := gitRead(repo, "rev-list", "--first-parent", "--reverse", "--parents", base+".."+tip)
	if err != nil {
		return nil, err
	}
	commits := []Commit{}
	unitCommits := map[string]KindInfo{}
	unitLists := map[string]bool{}
	seenUnits := map[string]string{}
	emptyUnits := map[string]string{}
	for _, line := range strings.FieldsFunc(strings.TrimSpace(string(out)), func(r rune) bool { return r == '\n' || r == '\r' }) {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		id := fields[0]
		if len(fields) != 2 {
			return nil, rangeRefusal(goalID, id, fmt.Sprintf("it has %d parents, and a goal branch commit has one", len(fields)-1))
		}
		kind, err := kindOfWithGit(repo, id, goalID, gitRead)
		if err != nil {
			return nil, err
		}
		entries, err := rawEntriesWithGitParsed(repo, id, gitRead)
		if err != nil {
			return nil, err
		}
		reads, closures, prose := 0, 0, 0
		if kind.Kind == Unit {
			for _, unit := range kind.Units {
				if earlier := seenUnits[unit]; earlier != "" {
					return nil, rangeRefusal(goalID, id, fmt.Sprintf("build %s was already made by commit %s", unit, earlier))
				}
				seenUnits[unit] = id
			}
			if len(entries) == 0 {
				emptyUnits[kind.Unit] = id
				continue
			}
		}
		for _, entry := range entries {
			class := PathClass(entry.Path)
			allowed := (kind.Kind == Unit || kind.Kind == Drop) && class == ClassUnit ||
				kind.Kind == Plan && (class == ClassPlan || entry.Path == landingRecordPath(goalID))
			if kind.Kind == Read {
				switch class {
				case ClassRead:
					g, c, _ := readPath(entry.Path)
					reads++
					allowed = reads == 1 && g == goalID && c == kind.CommitID
				case ClassReadClosure:
					g, c, _ := readClosurePath(entry.Path)
					closures++
					allowed = closures == 1 && g == goalID && c == kind.CommitID
				case ClassReadProse:
					prose++
					allowed = prose <= 1
				}
			}
			if !allowed {
				return nil, rangeRefusal(goalID, id, fmt.Sprintf("a %s may not change %s (a %s file)", commitWord(kind.Kind), entry.Path, class))
			}
		}
		if kind.Kind == Read && reads != 1 {
			return nil, rangeRefusal(goalID, id, fmt.Sprintf("a review commit holds one review record, and this one holds %d", reads))
		}
		if kind.Kind == Unit {
			unitCommits[id] = kind
			unitLists[kind.Unit] = true
		}
		if kind.Kind == Read {
			subject, inRange := unitCommits[kind.CommitID]
			present := inRange
			if !inRange {
				subject, err = kindOfWithGit(repo, kind.CommitID, goalID, gitRead)
				present = err == nil && subject.Kind == Unit
			}
			reviewed := present && sameUnits(subject.Units, kind.Units)
			earlier := reviewed && unitLists[kind.Unit]
			// A read may carry the review of a build main already holds.
			if reviewed && !inRange && !earlier {
				landed, err := readUnitLanded(repo, endpointTip, goalID, kind, gitRead)
				if err != nil {
					return nil, err
				}
				if landed {
					delete(emptyUnits, kind.Unit)
					continue
				}
			}
			if !earlier {
				return nil, rangeRefusal(goalID, id, "the review names no earlier build of the same work")
			}
		}
		item := Commit{ID: id, Kind: kind.Kind, Units: append([]string(nil), kind.Units...), Unit: kind.Unit, Whole: kind.Whole}
		if kind.Kind == Unit {
			item.Digest, err = unitDigestWithGit(repo, id, gitRead)
			if err != nil {
				return nil, err
			}
		}
		commits = append(commits, item)
	}
	for _, id := range emptyUnits {
		return nil, rangeRefusal(goalID, id, "the build changes no file and has no retained read proving its change landed")
	}
	return commits, nil
}

// readUnitLanded proves that a read's original change is on main. Unit names
// narrow the search; only ancestry or a verified content digest proves it.
func readUnitLanded(repo, endpoint, goalID string, kind KindInfo, gitRead func(string, ...string) ([]byte, error)) (bool, error) {
	if _, err := gitRead(repo, "merge-base", "--is-ancestor", kind.CommitID, endpoint); err == nil {
		return true, nil
	}
	out, err := gitRead(repo, "log", "--first-parent", "--format=%H", "--fixed-strings", "--grep=Goal-Unit: "+goalID+"/"+kind.Unit, endpoint)
	if err != nil {
		return false, err
	}
	if len(strings.Fields(string(out))) == 0 {
		return false, nil
	}
	digest, err := unitDigestWithGit(repo, kind.CommitID, gitRead)
	if err != nil {
		return false, err
	}
	for _, commit := range strings.Fields(string(out)) {
		verified, err := verifyLandedWithGit(repo, commit, gitRead)
		if err == nil && verified.Goal == goalID && verified.Units == kind.Unit && verified.Actual == digest {
			return true, nil
		}
	}
	return false, nil
}
