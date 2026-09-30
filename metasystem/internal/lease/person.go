package lease

import (
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
)

// ClassifyPersonAt is ClassifyAt for the checks that ask whether a person
// runs the verb. A caller ClassifyAt does not see as HUMAN is a person here
// when a live general power of attorney admits it (humanauthority.AtAttorney,
// asked for the checkout, then for the metasystem state root): the answer is
// HUMAN with the grant. ClassifyAt itself never changes, so the grantee's own
// claim, landing and lease renewal still see MAIN.
func ClassifyPersonAt(root, metasystemRoot string, caller int64, now time.Time) (Classification, error) {
	return classifyPersonAt(root, metasystemRoot, caller, now, ClassifyAt, humanauthority.AtAttorney)
}

func classifyPersonAt(root, metasystemRoot string, caller int64, now time.Time,
	classify func(string, string, int64) (Classification, error),
	admit func(string, int64, time.Time) (humanauthority.HelmGrant, bool)) (Classification, error) {
	classification, err := classify(root, metasystemRoot, caller)
	if err != nil || classification.Class == ClassHuman || admit == nil || now.IsZero() {
		return classification, err
	}
	roots := []string{root}
	if filepath.Clean(metasystemRoot) != filepath.Clean(root) {
		roots = append(roots, metasystemRoot)
	}
	for _, candidate := range roots {
		if grant, admitted := admit(candidate, caller, now); admitted && grant.Grant != "" {
			return Classification{Class: ClassHuman, MainId: classification.MainId, Pid: classification.Pid, Attorney: &grant}, nil
		}
	}
	return classification, nil
}
