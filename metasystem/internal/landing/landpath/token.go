package landpath

import (
	"path/filepath"
)

// WrapperToken is the live commit wrapper token the pre-commit guard
// verifies (internal/validate.WrapperToken): the wrapper's pid and kernel
// start second, a fresh 32-hex-character nonce, and the creation time. Its
// JSON form is the one `lease commit-token` wrote for the shell wrapper, so
// a guard at any base verifies a token this path mints.
type WrapperToken struct {
	WrapperPid          int64  `json:"wrapperPid"`
	WrapperPidStartedAt int64  `json:"wrapperPidStartedAt"`
	Nonce               string `json:"nonce"`
	CreatedAt           string `json:"createdAt"`
}

// TokenPath is the one per-checkout wrapper token path.
func TokenPath(root string) string {
	return filepath.Join(root, "artifacts", "agents", "mains", "worktree-commit-token.json")
}
