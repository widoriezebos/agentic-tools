// Package roots names the two directories every MetaSystem path starts from,
// as two types the compiler keeps apart.
//
// The installation root holds what MetaSystem deploys: the engine binary,
// metasystem.conf, skills, docs and hooks. The state root holds what the
// application evolves: the goal ledger, plans, records and memory registers.
// A path joined onto the wrong root reads or writes the other owner's files,
// so neither type converts to the other, and a root becomes a plain path only
// through Path.
//
// The package imports nothing from the engine, so the configuration reader and
// the state-root resolver can both name these types. It resolves nothing: the
// resolver in internal/stateroot produces every root value.
package roots

import "path/filepath"

// Installation is the directory of one MetaSystem installation, the one that
// holds metasystem.conf.
type Installation string

// State is the directory beneath which one application's project state lives.
type State string

// Path returns the installation root, or a path beneath it when segments are
// given, as the plain string a filesystem, Git or child-process call takes.
func (i Installation) Path(segments ...string) string { return join(string(i), segments) }

// Path returns the state root, or a path beneath it when segments are given,
// as the plain string a filesystem, Git or child-process call takes.
func (s State) Path(segments ...string) string { return join(string(s), segments) }

func join(root string, segments []string) string {
	if len(segments) == 0 {
		return root
	}
	return filepath.Join(append([]string{root}, segments...)...)
}
