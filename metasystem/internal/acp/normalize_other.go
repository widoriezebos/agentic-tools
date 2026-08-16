//go:build !darwin

package acp

// platformCanonical is the identity on case-sensitive filesystems:
// the on-disk spelling IS the given spelling.
func platformCanonical(path string) string { return path }
