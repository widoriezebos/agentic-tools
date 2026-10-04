// Package roots gives the fixture the engine's two root types.
package roots

import "path/filepath"

// Installation is the deployed tree, where run state lives.
type Installation string

// State is the tree the application evolves.
type State string

func (i Installation) Path(segments ...string) string {
	return filepath.Join(append([]string{string(i)}, segments...)...)
}

func (s State) Path(segments ...string) string {
	return filepath.Join(append([]string{string(s)}, segments...)...)
}
