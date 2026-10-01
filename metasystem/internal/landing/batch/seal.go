package batch

import (
	"os"
	"path/filepath"
)

// ModuleRoot returns the nested MetaSystem module when a checkout contains one.
func ModuleRoot(checkout string) string {
	nested := filepath.Join(checkout, "metasystem")
	if info, err := os.Stat(filepath.Join(nested, "go.mod")); err == nil && !info.IsDir() {
		return nested
	}
	return checkout
}
