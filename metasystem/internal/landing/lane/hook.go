package lane

// The pre-push hook an earlier engine installed in the lane checkout is
// gone (simple lane): landing set and landing unset remove one they find,
// so the checkout pushes as any other.

import (
	"bytes"
	"os"
	"path/filepath"
)

// hookMarker is the line that made a pre-push hook the lane's own.
const hookMarker = "# the landing lane's pre-push hook, installed by landing set"

// removeHook removes the lane's earlier pre-push hook from checkout; a hook
// that is not the lane's, or a checkout that is gone, is left as it is.
func removeHook(checkout string) error {
	if checkout == "" || gone(checkout) {
		return nil
	}
	hooks, err := laneGit(checkout, nil, "rev-parse", "--path-format=absolute", "--git-path", "hooks")
	if err != nil {
		return nil
	}
	path := filepath.Join(hooks, "pre-push")
	existing, err := os.ReadFile(path)
	if err != nil || !bytes.Contains(existing, []byte(hookMarker)) {
		return nil
	}
	return removeIfPresent(path)
}
