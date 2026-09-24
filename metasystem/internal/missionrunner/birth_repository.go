package missionrunner

import (
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
)

// birthRepositoryEffects owns the repository observations and anchor effects
// used while an Engine publishes its initial mission state.
type birthRepositoryEffects interface {
	CaptureAdmissionOrigins(root, missionID string) (map[string]any, error)
	AnchorInitial(root, missionID, tree string) error
	DropAnchors(root, missionID string) error
}

type defaultBirthRepositoryEffects struct{}

func (defaultBirthRepositoryEffects) CaptureAdmissionOrigins(root, missionID string) (map[string]any, error) {
	return mission.CaptureAdmissionOrigins(root, missionID)
}

func (defaultBirthRepositoryEffects) AnchorInitial(root, missionID, tree string) error {
	return (gittree.Workspace{Dir: root}).Anchor(missionID, tree)
}

func (defaultBirthRepositoryEffects) DropAnchors(root, missionID string) error {
	return (gittree.Workspace{Dir: root}).DropAnchors(missionID)
}

func (e *Engine) birthRepository() birthRepositoryEffects {
	if e.birthEffects != nil {
		return e.birthEffects
	}
	return defaultBirthRepositoryEffects{}
}
