// Package adapterfile is the file half of the external agent-adapter trust
// boundary (design verbs-object-action 3.5, VOA-28): where an external
// adapter lives, which names it may take, which configuration key names it,
// and whether its file is safe to execute. It is a dependency leaf so the
// configuration validator and the runtime registry apply one rule.
package adapterfile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"syscall"
)

// Dir is the installation-relative adapter directory.
const Dir = "adapters"

// UseValue is the only value of adapters.<name>.use that lets an executable
// run: a new runtime, or an override of a built-in of the same name.
const UseValue = "external"

// UseKey is the configuration key that names an external adapter (or an
// override of a built-in): adapters.<name>.use=external.
func UseKey(name string) string { return "adapters." + name + ".use" }

var nameRE = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)

// ValidName reports whether a name fits the runtime-name grammar.
func ValidName(name string) bool { return nameRE.MatchString(name) }

// Path is the adapter executable of a name under an installation root.
func Path(root, name string) string { return filepath.Join(root, Dir, name) }

// ErrAbsent is a name with no executable in the adapter directory.
var ErrAbsent = errors.New("no adapter executable")

// Check reports why the adapter file of a name may not run, with the fix a
// person applies: an empty reason is a trusted file. ErrAbsent is a missing
// or non-executable file.
func Check(root, name string) (reason string, err error) {
	path := Path(root, name)
	info, statErr := os.Stat(path)
	if statErr != nil || !info.Mode().IsRegular() || info.Mode()&0o111 == 0 {
		return "", ErrAbsent
	}
	if !ValidName(name) {
		return fmt.Sprintf("the name %q is not a runtime name (lowercase letters, digits and dashes, starting with a letter); rename the file", name), nil
	}
	return Unsafe(path, info), nil
}

// Unsafe names why an adapter file is not trusted, and the fix: writable by
// its group or by everyone, or not owned by the installation's user.
func Unsafe(path string, info os.FileInfo) string {
	if info.Mode()&0o022 != 0 {
		return fmt.Sprintf("the adapter file is group- or world-writable, so someone other than you could change what runs; fix it with: chmod go-w %s", path)
	}
	if stat, ok := info.Sys().(*syscall.Stat_t); ok && int(stat.Uid) != os.Getuid() {
		return fmt.Sprintf("the adapter file is not owned by the installation's user (uid %d); fix it with: chown %d %s", stat.Uid, os.Getuid(), path)
	}
	return ""
}

// Unnamed is the refusal of an executable the configuration does not name.
func Unnamed(name string) string {
	return fmt.Sprintf("the configuration does not name the external adapter %s, so it does not run\nadd %s=%s to metasystem.conf or metasystem.conf.local", name, UseKey(name), UseValue)
}
