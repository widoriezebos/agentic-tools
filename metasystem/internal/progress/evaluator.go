// Package progress owns the filesystem signals that can show a delegate job
// made progress. It classifies product roots when the launch record is sealed
// and rechecks their standing whenever a caller asks for fresh evidence.
package progress

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	LaunchModeWorktree       = "worktree"
	LaunchModeSharedCheckout = "shared-checkout"

	StandingLiveness        = "liveness"
	StandingAttributionOnly = "attribution-only"

	ReasonContainedAtLaunch         = "contained-at-launch"
	ReasonOutsideWorktreeAtLaunch   = "outside-worktree-at-launch"
	ReasonExcludedAtLaunch          = "excluded-at-launch"
	ReasonSharedCheckout            = "shared-checkout"
	ReasonResolutionOutsideWorktree = "resolution-outside-worktree"
	ReasonResolutionExcluded        = "resolution-excluded"

	RootStatusScanned    = "scanned"
	RootStatusEmpty      = "empty"
	RootStatusMissing    = "missing"
	RootStatusUnreadable = "unreadable"
	RootStatusDemoted    = "demoted"
	RootStatusNotScanned = "not-scanned"
)

// ProductRootScope is the immutable launch-time standing of one declared
// product root. Path remains the canonical declared path so a later scan can
// resolve it again instead of trusting an old target.
type ProductRootScope struct {
	Path     string `json:"path"`
	Standing string `json:"standing"`
	Reason   string `json:"reason"`
}

// Anomaly reports a surprising filesystem transition without turning the
// observation into progress.
type Anomaly struct {
	Label  string `json:"label"`
	Detail string `json:"detail"`
}

// OutputEvidence describes the child output stream at one observation. The
// returned HighWater is monotonic and is the value the caller should retain.
type OutputEvidence struct {
	Path         string     `json:"path"`
	Status       string     `json:"status"`
	ObservedSize int64      `json:"observedSize"`
	HighWater    int64      `json:"highWater"`
	Grew         bool       `json:"grew"`
	EventAt      *time.Time `json:"eventAt,omitempty"`
	Anomalies    []Anomaly  `json:"anomalies"`
}

// ProductRootEvidence carries both the launch standing and the binding
// standing after resolving containment and exclusions for this scan.
type ProductRootEvidence struct {
	Path           string     `json:"path"`
	ResolvedPath   string     `json:"resolvedPath,omitempty"`
	LaunchStanding string     `json:"launchStanding"`
	ScanStanding   string     `json:"scanStanding"`
	Reason         string     `json:"reason"`
	Status         string     `json:"status"`
	EventAt        *time.Time `json:"eventAt,omitempty"`
	Detail         string     `json:"detail,omitempty"`
}

// ProductEvidence is the per-root scan plus the newest file event time from
// roots that retained liveness standing. Directory mtimes are never events.
type ProductEvidence struct {
	Roots   []ProductRootEvidence `json:"roots"`
	EventAt *time.Time            `json:"eventAt,omitempty"`
}

// Evidence is one progress observation. It contains facts only; the sweep
// that consumes it owns age thresholds, persistence, and final verdicts.
type Evidence struct {
	Output   OutputEvidence  `json:"output"`
	Products ProductEvidence `json:"products"`
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
	workspace = resolvePath(workspace)
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
		resolved := resolvePath(root)
		switch {
		case !pathWithin(resolved, workspace):
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

// Evaluate observes the immutable child output stream and the record's
// launch-classified product roots. The caller supplies and retains the output
// high-water; a smaller file never lowers it.
func Evaluate(record map[string]any, outputHighWater int64) (Evidence, error) {
	if outputHighWater < 0 {
		return Evidence{}, fmt.Errorf("progress output high-water cannot be negative")
	}
	outputStream, _ := record["outputStream"].(string)
	workspace, _ := record["workspaceRoot"].(string)
	launchMode, _ := record["launchMode"].(string)
	if outputStream == "" || workspace == "" {
		return Evidence{}, fmt.Errorf("progress record requires outputStream and workspaceRoot")
	}
	if launchMode != LaunchModeWorktree && launchMode != LaunchModeSharedCheckout {
		return Evidence{}, fmt.Errorf("progress record launchMode must be worktree or shared-checkout")
	}
	scopes, err := productRootScopes(record["productRootScopes"])
	if err != nil {
		return Evidence{}, err
	}

	evidence := Evidence{
		Output: observeOutput(outputStream, outputHighWater),
		Products: ProductEvidence{
			Roots: make([]ProductRootEvidence, 0, len(scopes)),
		},
	}
	workspace = resolvePath(workspace)
	for _, scope := range scopes {
		root := observeProductRoot(scope, launchMode, workspace)
		evidence.Products.Roots = append(evidence.Products.Roots, root)
		if root.EventAt != nil && (evidence.Products.EventAt == nil || root.EventAt.After(*evidence.Products.EventAt)) {
			eventAt := *root.EventAt
			evidence.Products.EventAt = &eventAt
		}
	}
	return evidence, nil
}

func observeOutput(path string, highWater int64) OutputEvidence {
	result := OutputEvidence{
		Path: path, Status: "observed", HighWater: highWater,
		Anomalies: []Anomaly{},
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			result.Status = "missing"
		} else {
			result.Status = "unreadable"
			result.Anomalies = append(result.Anomalies, Anomaly{Label: "output-unreadable", Detail: err.Error()})
		}
		return result
	}
	if !info.Mode().IsRegular() {
		result.Status = "invalid"
		result.Anomalies = append(result.Anomalies, Anomaly{Label: "output-not-regular", Detail: path})
		return result
	}
	result.ObservedSize = info.Size()
	switch {
	case info.Size() > highWater:
		result.Grew = true
		result.HighWater = info.Size()
		eventAt := info.ModTime()
		result.EventAt = &eventAt
	case info.Size() < highWater:
		result.Anomalies = append(result.Anomalies, Anomaly{
			Label:  "output-truncated",
			Detail: fmt.Sprintf("observed size %d is below retained high-water %d", info.Size(), highWater),
		})
	}
	return result
}

func observeProductRoot(scope ProductRootScope, launchMode, workspace string) ProductRootEvidence {
	result := ProductRootEvidence{
		Path: scope.Path, LaunchStanding: scope.Standing,
		ScanStanding: StandingAttributionOnly, Reason: scope.Reason,
		Status: RootStatusNotScanned,
	}
	if launchMode == LaunchModeSharedCheckout {
		result.Reason = ReasonSharedCheckout
		return result
	}
	if scope.Standing != StandingLiveness {
		return result
	}
	resolved := resolvePath(scope.Path)
	result.ResolvedPath = resolved
	switch {
	case !pathWithin(resolved, workspace):
		result.Reason = ReasonResolutionOutsideWorktree
		result.Status = RootStatusDemoted
		return result
	case excludedProductPath(workspace, resolved):
		result.Reason = ReasonResolutionExcluded
		result.Status = RootStatusDemoted
		return result
	}
	result.ScanStanding = StandingLiveness
	result.Reason = ReasonContainedAtLaunch
	eventAt, status, detail := newestProductFileEvent(resolved, workspace)
	result.EventAt = eventAt
	result.Status = status
	result.Detail = detail
	return result
}

func newestProductFileEvent(root, workspace string) (*time.Time, string, string) {
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, RootStatusMissing, err.Error()
		}
		return nil, RootStatusUnreadable, err.Error()
	}
	if info.Mode().IsRegular() {
		eventAt := info.ModTime()
		return &eventAt, RootStatusScanned, ""
	}
	if !info.IsDir() {
		return nil, RootStatusUnreadable, "product root is neither a regular file nor a directory"
	}
	var newest *time.Time
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path != root && excludedProductPath(workspace, resolvePath(path)) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		entryInfo, infoErr := entry.Info()
		if infoErr != nil {
			return infoErr
		}
		if !entryInfo.Mode().IsRegular() {
			return nil
		}
		modified := entryInfo.ModTime()
		if newest == nil || modified.After(*newest) {
			newest = &modified
		}
		return nil
	})
	if err != nil {
		return nil, RootStatusUnreadable, err.Error()
	}
	if newest == nil {
		return nil, RootStatusEmpty, ""
	}
	return newest, RootStatusScanned, ""
}

func productRootScopes(value any) ([]ProductRootScope, error) {
	if value == nil {
		return nil, fmt.Errorf("progress record requires productRootScopes")
	}
	if typed, ok := value.([]ProductRootScope); ok {
		return append([]ProductRootScope(nil), typed...), nil
	}
	raw, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("progress record productRootScopes must be an array")
	}
	scopes := make([]ProductRootScope, 0, len(raw))
	for index, item := range raw {
		object, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("progress record productRootScopes[%d] must be an object", index)
		}
		path, _ := object["path"].(string)
		standing, _ := object["standing"].(string)
		reason, _ := object["reason"].(string)
		if path == "" || (standing != StandingLiveness && standing != StandingAttributionOnly) || reason == "" {
			return nil, fmt.Errorf("progress record productRootScopes[%d] is malformed", index)
		}
		scopes = append(scopes, ProductRootScope{Path: path, Standing: standing, Reason: reason})
	}
	return scopes, nil
}

// resolvePath follows the deepest existing ancestor and preserves a missing
// suffix. Calling it again later observes an intermediate symlink that did not
// exist when the launch was recorded.
func resolvePath(path string) string {
	if absolute, err := filepath.Abs(path); err == nil {
		path = absolute
	}
	suffix := ""
	current := path
	for {
		if resolved, err := filepath.EvalSymlinks(current); err == nil {
			return filepath.Join(resolved, suffix)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return filepath.Clean(path)
		}
		suffix = filepath.Join(filepath.Base(current), suffix)
		current = parent
	}
}

func pathWithin(path, root string) bool {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, "../"))
}

func excludedProductPath(workspace, path string) bool {
	for _, excluded := range []string{
		resolvePath(filepath.Join(workspace, ".git")),
		resolvePath(filepath.Join(workspace, "artifacts", "agents")),
	} {
		if pathWithin(path, excluded) {
			return true
		}
	}
	return false
}
