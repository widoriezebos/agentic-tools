// Package enginebuild owns the grammar of source-linked engine build stamps.
package enginebuild

// DevelopmentStamp is the stamp of a build whose compiled engine inputs differ
// from the commit it names.
func DevelopmentStamp(commit string) string {
	return "dev-" + commit + "-dirty"
}
