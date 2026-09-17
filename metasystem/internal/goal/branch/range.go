package branch

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"unicode"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
)

type Kind string

const (
	Unit Kind = "unit"
	Plan Kind = "plan"
	Read Kind = "read"
)

type Class string

const (
	ClassExcluded  Class = "excluded"
	ClassRead      Class = "read"
	ClassReadProse Class = "read-prose"
	ClassPlan      Class = "plan"
	ClassUnit      Class = "unit"
)

type KindInfo struct {
	Kind           Kind
	Unit, CommitID string
}
type Commit struct {
	ID           string
	Kind         Kind
	Unit, Digest string
}

const RangeCode = "GOAL_BRANCH_RANGE"

type RangeError struct{ Code, Commit, Reason string }

func (e *RangeError) Error() string {
	return fmt.Sprintf("%s: commit %s: %s", e.Code, e.Commit, e.Reason)
}
func refuse(commit, reason string) error { return &RangeError{RangeCode, commit, reason} }

func splitGoalUnit(value string) (string, string, bool) {
	goal, unit, ok := strings.Cut(value, "/")
	return goal, unit, ok && goal != "" && unit != "" && !strings.Contains(unit, "/") && strings.IndexFunc(unit, unicode.IsSpace) < 0
}

func KindOf(repo, commit, goalID string) (KindInfo, error) {
	out, err := gitOutput(repo, "show", "-s", "--format=%(trailers:only,unfold=true)", commit)
	if err != nil {
		return KindInfo{}, err
	}
	var trailers [][2]string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		key, value, ok := strings.Cut(line, ":")
		if ok && (key == "Goal-Unit" || key == "Goal-Plan" || key == "Goal-Read") {
			trailers = append(trailers, [2]string{key, strings.TrimSpace(value)})
		}
	}
	if len(trailers) != 1 {
		return KindInfo{}, refuse(commit, fmt.Sprintf("expected exactly one kind trailer, found %d", len(trailers)))
	}
	key, value := trailers[0][0], trailers[0][1]
	if key == "Goal-Plan" {
		if value != goalID {
			return KindInfo{}, refuse(commit, fmt.Sprintf("Goal-Plan names goal %q, not %q", value, goalID))
		}
		return KindInfo{Kind: Plan}, nil
	}
	fields := strings.Fields(value)
	if key == "Goal-Read" && len(fields) != 2 {
		return KindInfo{}, refuse(commit, "Goal-Read is malformed")
	}
	first := value
	if key == "Goal-Read" {
		first = fields[0]
	}
	goal, unit, ok := splitGoalUnit(first)
	if !ok {
		return KindInfo{}, refuse(commit, key+" is malformed")
	}
	if goal != goalID {
		return KindInfo{}, refuse(commit, fmt.Sprintf("%s names goal %q, not %q", key, goal, goalID))
	}
	if key == "Goal-Unit" {
		return KindInfo{Kind: Unit, Unit: unit}, nil
	}
	if !hex40(fields[1]) {
		return KindInfo{}, refuse(commit, "Goal-Read commit id is not full 40-hex")
	}
	return KindInfo{Kind: Read, Unit: unit, CommitID: fields[1]}, nil
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
	if strings.HasPrefix(path, "metasystem/records/misc/") {
		return ClassReadProse
	}
	if path == "metasystem/plans" || strings.HasPrefix(path, "metasystem/plans/") || path == "metasystem/records" || strings.HasPrefix(path, "metasystem/records/") {
		return ClassPlan
	}
	return ClassUnit
}

func ValidateRange(repo, endpointTip, tip, goalID string) ([]Commit, error) {
	if _, err := gitOutput(repo, "merge-base", "--is-ancestor", endpointTip, tip); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 1 {
			return nil, refuse(tip, "tip does not descend from the endpoint")
		}
		return nil, err
	}
	out, err := gitOutput(repo, "rev-list", "--first-parent", "--reverse", "--parents", endpointTip+".."+tip)
	if err != nil {
		return nil, err
	}
	commits := []Commit{}
	for _, line := range strings.FieldsFunc(strings.TrimSpace(string(out)), func(r rune) bool { return r == '\n' || r == '\r' }) {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		id := fields[0]
		if len(fields) != 2 {
			return nil, refuse(id, fmt.Sprintf("commit has %d parents; exactly one is required", len(fields)-1))
		}
		kind, err := KindOf(repo, id, goalID)
		if err != nil {
			return nil, err
		}
		entries, err := RawEntries(repo, id)
		if err != nil {
			return nil, err
		}
		reads, prose := 0, 0
		for _, entry := range entries {
			class := PathClass(entry.Path)
			allowed := kind.Kind == Unit && class == ClassUnit || kind.Kind == Plan && class == ClassPlan
			if kind.Kind == Read {
				switch class {
				case ClassRead:
					g, c, _ := readPath(entry.Path)
					reads++
					allowed = reads == 1 && g == goalID && c == kind.CommitID
				case ClassReadProse:
					prose++
					allowed = prose <= 1
				}
			}
			if !allowed {
				return nil, refuse(id, fmt.Sprintf("path %s has class %s, which kind %s does not allow", entry.Path, class, kind.Kind))
			}
		}
		if kind.Kind == Read && reads != 1 {
			return nil, refuse(id, fmt.Sprintf("kind read requires exactly one attestation path, found %d", reads))
		}
		item := Commit{ID: id, Kind: kind.Kind, Unit: kind.Unit}
		if kind.Kind == Unit {
			item.Digest, err = UnitDigest(repo, id)
			if err != nil {
				return nil, err
			}
		}
		commits = append(commits, item)
	}
	return commits, nil
}
