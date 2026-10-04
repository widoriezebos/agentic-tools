// Package crossing builds run-state paths from both roots: each State-fed
// function crosses, and its Installation-fed twin does not.
package crossing

import (
	"path/filepath"

	"example.test/fixture/internal/roots"
)

func runDir(root string) string { return filepath.Join(root, "artifacts") }

func middle(dir string) string { return runDir(dir) }

func DirectState(state roots.State) string { return state.Path("artifacts", "runs") }

func DirectInstallation(installation roots.Installation) string {
	return installation.Path("artifacts", "runs")
}

func ParamChain(state roots.State) string { return middle(state.Path()) }

func ParamChainFromInstallation(installation roots.Installation) string {
	return middle(installation.Path())
}

type store struct{ root string }

func (s store) runs() string { return filepath.Join(s.root, "artifacts") }

func FieldFlow(state roots.State) string { return store{root: state.Path()}.runs() }

func FieldFlowFromInstallation(installation roots.Installation) string {
	return store{root: installation.Path()}.runs()
}

type owners struct{ launch func(root string) string }

func FuncField(state roots.State) string {
	o := owners{launch: runDir}
	return o.launch(state.Path())
}

func FuncFieldFromInstallation(installation roots.Installation) string {
	o := owners{launch: runDir}
	return o.launch(installation.Path())
}

// Named carries the state root by name alone.
func Named() string {
	stateRoot := "/srv/state"
	return runDir(stateRoot)
}
