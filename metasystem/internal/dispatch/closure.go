package dispatch

import "github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"

const closureField = "closure"

type Closure = readsubject.Closure

func ReadClosure(root map[string]any) (Closure, bool, error) {
	return readsubject.ReadClosure(root)
}
