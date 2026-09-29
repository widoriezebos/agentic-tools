// Package progress classifies a delegate job's declared product roots when
// its launch record is sealed: each root's launch-time standing says whether
// its files may later count as the job's own progress.
package progress

import (
	"fmt"
	"path/filepath"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
)

const (
	LaunchModeWorktree       = "worktree"
	LaunchModeSharedCheckout = "shared-checkout"

	StandingLiveness        = "liveness"
	StandingAttributionOnly = "attribution-only"

	ReasonContainedAtLaunch       = "contained-at-launch"
	ReasonOutsideWorktreeAtLaunch = "outside-worktree-at-launch"
	ReasonExcludedAtLaunch        = "excluded-at-launch"
	ReasonSharedCheckout          = "shared-checkout"
)

// ProductRootScope is the immutable launch-time standing of one declared
// product root. Path remains the declared path, never its resolved target.
type ProductRootScope struct {
	Path     string `json:"path"`
	Standing string `json:"standing"`
	Reason   string `json:"reason"`
}

// CaptureProductRootScopes fixes each root's launch-time standing. A root in
// a worktree is independent: one outside or excluded root cannot demote a
// contained sibling. Shared checkouts make every root attribution-only.
func CaptureProductRootScopes(launchMode, workspace string, roots []string) ([]ProductRootScope, error) {
	if launchMode != LaunchModeWorktree && launchMode != LaunchModeSharedCheckout {
		return nil, fmt.Errorf("progress launch mode must be worktree or shared-checkout")
	}
	if workspace == "" {
		return nil, fmt.Errorf("progress launch requires the workspace root")
	}
	workspace = realpath.Resolve(workspace)
	scopes := make([]ProductRootScope, 0, len(roots))
	for _, root := range roots {
		if root == "" || !filepath.IsAbs(root) {
			return nil, fmt.Errorf("progress product roots must be non-empty absolute paths")
		}
		scope := ProductRootScope{Path: root}
		if launchMode == LaunchModeSharedCheckout {
			scope.Standing = StandingAttributionOnly
			scope.Reason = ReasonSharedCheckout
			scopes = append(scopes, scope)
			continue
		}
		resolved := realpath.Resolve(root)
		switch {
		case !realpath.Within(resolved, workspace):
			scope.Standing = StandingAttributionOnly
			scope.Reason = ReasonOutsideWorktreeAtLaunch
		case excludedProductPath(workspace, resolved):
			scope.Standing = StandingAttributionOnly
			scope.Reason = ReasonExcludedAtLaunch
		default:
			scope.Standing = StandingLiveness
			scope.Reason = ReasonContainedAtLaunch
		}
		scopes = append(scopes, scope)
	}
	return scopes, nil
}

func excludedProductPath(workspace, path string) bool {
	for _, excluded := range []string{
		realpath.Resolve(filepath.Join(workspace, ".git")),
		realpath.Resolve(filepath.Join(workspace, "artifacts", "agents")),
	} {
		if realpath.Within(path, excluded) {
			return true
		}
	}
	return false
}
