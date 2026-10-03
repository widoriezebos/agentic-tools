package receipt

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// AddToInstallation appends one task receipt to the receipt ledger owned by
// the installation at root, exactly as `metasystem receipt add` run from that
// installation's own engine does: the ledger is the installation's state root
// (not the calling executable's), unset optional fields take the verb's
// defaults, and a refusal returns the verb's diagnostic. Advisory lines (a
// retro-due note) come back in the result's Err for the caller to relay.
func AddToInstallation(root string, opts Options) (Result, error) {
	installation, err := stateroot.ParseInstallation(root)
	if err != nil {
		return Result{}, fmt.Errorf("receipt: %w", err)
	}
	appRoot, err := stateroot.RootForInstallation(installation)
	if err != nil {
		return Result{}, fmt.Errorf("receipt: %w", err)
	}
	relative, err := stateroot.RelativeRoot(stateroot.Receipts)
	if err != nil {
		return Result{}, fmt.Errorf("receipt: %w", err)
	}
	opts.Root = root
	opts.File = appRoot.Path(filepath.FromSlash(relative), "receipts.log")
	defaults := map[*string]string{&opts.Skills: "none", &opts.Verify: "skipped", &opts.Corrections: "0", &opts.StopLoss: "no"}
	for field, value := range defaults {
		if *field == "" {
			*field = value
		}
	}
	result := Add(opts)
	if result.Code != 0 {
		return result, fmt.Errorf("receipt refused: %s", strings.Join(result.Err, "; "))
	}
	return result, nil
}
