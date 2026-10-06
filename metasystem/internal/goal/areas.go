package goal

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

// AreaSnapshot binds declared edits to the accepted design's exact bytes.
type AreaSnapshot struct {
	Areas    []string `json:"areas,omitempty"`
	Source   string   `json:"areas-source,omitempty"`
	Known    bool     `json:"areas-known,omitempty"`
	Warnings []string `json:"areas-warnings,omitempty"`
}

// AreaOverlap reports patterns and their literal prefixes, never a guessed file.
type AreaOverlap struct{ Left, Right, LeftPrefix, RightPrefix string }

func AreasOverlap(left, right []string) (*AreaOverlap, error) {
	a, err := launch.NormalizeAreas(left)
	if err != nil {
		return nil, err
	}
	b, err := launch.NormalizeAreas(right)
	if err != nil {
		return nil, err
	}
	prefix := func(s string) string {
		if s == "." {
			return ""
		}
		if i := strings.IndexAny(s, "*?["); i >= 0 {
			return s[:i]
		}
		return s
	}
	for _, x := range a {
		for _, y := range b {
			p, q := prefix(x), prefix(y)
			if strings.HasPrefix(p, q) || strings.HasPrefix(q, p) {
				return &AreaOverlap{x, y, p, q}, nil
			}
		}
	}
	return nil, nil
}

// ClaimAreaEntry is the newest queue state, observed against local main.
type ClaimAreaEntry struct {
	Goal, State string
	Snapshot    AreaSnapshot
}

type ClaimAreaReaders struct {
	Design func(goalID, ledgerTip string) AreaSnapshot
	Queue  func() ([]ClaimAreaEntry, []string)
}

const ClaimAreasCode = "GOAL_CLAIM_AREAS"

// DesignAreasAt reads accepted designs at the immutable fetched ledger tip.
// Both design homes use repository paths, as unit admission does.
func DesignAreasAt(endpoint Endpoint, tip, id string) AreaSnapshot {
	prefixes := []string{"plans/designs/"}
	if endpoint.Repository != nil {
		prefixes = append(prefixes, "../plans/designs/", "metasystem/plans/designs/")
	} else if prefix, err := gitIn(endpoint.Root, "rev-parse", "--show-prefix"); err == nil && strings.TrimSpace(prefix) != "" {
		prefixes = append(prefixes, strings.Repeat("../", strings.Count(strings.TrimSpace(prefix), "/"))+"plans/designs/")
	}
	files, err := readCommitFiles(endpoint, tip, prefixes...)
	if err != nil {
		return AreaSnapshot{Warnings: []string{"areas unknown for goal " + id + ": " + err.Error()}}
	}
	var result AreaSnapshot
	paths := make([]string, 0, len(files))
	for name := range files {
		paths = append(paths, name)
	}
	slices.Sort(paths)
	for _, name := range paths {
		text := string(files[name])
		head := map[string]string{}
		for _, line := range strings.Split(text, "\n") {
			if len(head) > 0 && !strings.HasPrefix(line, "- ") {
				break
			}
			if strings.HasPrefix(line, "- ") {
				key, value, ok := strings.Cut(strings.TrimPrefix(line, "- "), ":")
				if ok {
					head[key] = strings.TrimSpace(value)
				}
			}
		}
		if head["Kind"] != "design" || head["Status"] != "accepted" || !slices.Contains(strings.Fields(strings.ReplaceAll(head["Goals"], ",", " ")), id) {
			continue
		}
		areas, known, err := launch.DeclaredAreas(text)
		source := fmt.Sprintf("%s@%x", head["Id"], sha256.Sum256(files[name]))
		if err != nil || !known || head["Id"] == "" {
			return AreaSnapshot{Source: source, Warnings: []string{"areas unknown for goal " + id + " in design " + name}}
		}
		result.Known = true
		result.Areas = append(result.Areas, areas...)
		if result.Source != "" {
			result.Source += ";"
		}
		result.Source += source
	}
	result.Areas, _ = launch.NormalizeAreas(result.Areas)
	if !result.Known {
		result.Warnings = []string{"areas unknown for goal " + id + ": no accepted design declares areas"}
	}
	return result
}

func snapshotFor(readers ClaimAreaReaders, endpoint Endpoint, tip, id string) AreaSnapshot {
	if readers.Design != nil {
		return readers.Design(id, tip)
	}
	return DesignAreasAt(endpoint, tip, id)
}

// ClaimAreas is the one admission predicate for selection and publication.
// Unknown declarations warn; another known overlap still refuses admission.
func ClaimAreas(tree *TreeGoals, endpoint Endpoint, tip, id string, candidate AreaSnapshot, readers ClaimAreaReaders) ([]string, error) {
	warnings := slices.Clone(candidate.Warnings)
	if _, err := launch.NormalizeAreas(candidate.Areas); err != nil {
		candidate.Known = false
		warnings = append(warnings, "areas unknown for goal "+id+": "+err.Error())
	}
	if !candidate.Known && len(warnings) == 0 {
		warnings = append(warnings, "areas unknown for goal "+id)
	}
	queue := []ClaimAreaEntry{}
	if readers.Queue != nil {
		var unknown []string
		queue, unknown = readers.Queue()
		warnings = append(warnings, unknown...)
	}
	landed := map[string]bool{}
	for _, entry := range queue {
		if entry.State == "landed" {
			landed[entry.Goal] = true
		}
	}
	compare := func(other string, snapshot AreaSnapshot) error {
		if err := validateAreaSnapshot(snapshot); err != nil {
			snapshot.Known = false
			warnings = append(warnings, "areas unknown for goal "+other+": "+err.Error())
		}
		if !snapshot.Known {
			warnings = append(warnings, "areas unknown for goal "+other)
			return nil
		}
		if !candidate.Known {
			return nil
		}
		overlap, err := AreasOverlap(candidate.Areas, snapshot.Areas)
		if err != nil {
			warnings = append(warnings, "areas unknown for goal "+other+": "+err.Error())
			return nil
		}
		if overlap != nil {
			return coded(ClaimAreasCode, fmt.Errorf("goal %s overlaps goal %s: declared areas %q and %q overlap (literal prefixes %q and %q)\nrun: metasystem goal claim  (after goal %s lands or is dropped)", id, other, overlap.Left, overlap.Right, overlap.LeftPrefix, overlap.RightPrefix, other))
		}
		return nil
	}
	for _, other := range sortedGoalIds(tree.Live) {
		f := tree.Live[other]
		if other == id || f.State != StateClaimed || f.Claimed == nil || landed[other] {
			continue
		}
		snapshot := f.Claimed.AreaSnapshot
		if snapshot.Source == "" && snapshot.Areas == nil && len(snapshot.Warnings) == 0 {
			snapshot = DesignAreasAt(endpoint, tip, other)
		}
		if err := compare(other, snapshot); err != nil {
			return warnings, err
		}
	}
	for _, entry := range queue {
		if entry.Goal == id || entry.State != "waiting" || landed[entry.Goal] {
			continue
		}
		f := tree.Live[entry.Goal]
		if f == nil || f.State == StateAbandoned || f.State == StateDone {
			continue
		}
		if err := compare(entry.Goal, entry.Snapshot); err != nil {
			return warnings, err
		}
	}
	return warnings, nil
}

func admitClaimAreas(t *TreeGoals, r VerbRequest, tip, id string) (AreaSnapshot, error) {
	snapshot := snapshotFor(r.ClaimAreaReaders, r.Endpoint, tip, id)
	warnings, err := ClaimAreas(t, r.Endpoint, tip, id, snapshot, r.ClaimAreaReaders)
	snapshot.Warnings = warnings
	return snapshot, err
}

// areaBindingChanged asks the publication owner to rebuild with fresh design bytes.
type areaBindingChanged struct{ Goal, Pinned, Current string }

func (changed areaBindingChanged) Error() string {
	return fmt.Sprintf("the accepted design for goal %s changed (bound to %q, read %q); rereading its areas", changed.Goal, changed.Pinned, changed.Current)
}

func validateClaimAreas(r VerbRequest, commit string, ids []string) error {
	if err := validateCommitFor(r.Endpoint, commit); err != nil {
		return err
	}
	if r.ClaimAreaReaders.Design == nil {
		return nil
	}
	tree, err := loadTreeFor(r.Endpoint, commit)
	if err != nil {
		return err
	}
	for _, id := range ids {
		f := tree.Live[id]
		if f == nil || f.Claimed == nil {
			continue
		}
		current := r.ClaimAreaReaders.Design(id, commit)
		pinned := f.Claimed.AreaSnapshot
		if current.Source != pinned.Source || current.Known != pinned.Known || !slices.Equal(current.Areas, pinned.Areas) {
			return areaBindingChanged{Goal: id, Pinned: pinned.Source, Current: current.Source}
		}
	}
	return nil
}

func validateAreaSnapshot(snapshot AreaSnapshot) error {
	if !snapshot.Known {
		return nil
	}
	areas, err := launch.NormalizeAreas(snapshot.Areas)
	if err != nil {
		return err
	}
	if !slices.Equal(areas, snapshot.Areas) {
		return fmt.Errorf("claim areas must be normalized")
	}
	if snapshot.Source == "" {
		return fmt.Errorf("known claim areas need the accepted design and its content hash")
	}
	for _, source := range strings.Split(snapshot.Source, ";") {
		at := strings.LastIndex(source, "@")
		if at < 1 {
			return fmt.Errorf("claim areas have no accepted design and content hash")
		}
		digest, err := hex.DecodeString(source[at+1:])
		if err != nil || len(digest) != sha256.Size {
			return fmt.Errorf("claim areas have an invalid content hash for their accepted design")
		}
	}
	return nil
}
