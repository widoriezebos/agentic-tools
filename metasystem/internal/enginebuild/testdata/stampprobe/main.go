// Command stampprobe prints the engine's derived build stamp so the
// enginebuild tests can build real binaries with and without -trimpath.
package main

import (
	"fmt"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
)

func main() { fmt.Println(supervise.BuildStamp) }
