package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginedeploy"
)

// runDeploy is this repository's deploy adapter, as deploy.json names it:
// the deploy runner calls it in the clean tree of the commit an operation is
// about, with the request on standard input and the response on standard
// output. The engine is built by the build action's --out branch; the
// build's own output goes to standard error, the deploy's log, because
// standard output carries the response alone.
func runDeploy(ctx context.Context, args []string, root string, d deps) int {
	home, err := board.HomeWith(func(name string) (string, bool) {
		value := d.getenv(name)
		return value, value != ""
	})
	if err != nil {
		return enginedeploy.Failed(d.stdout, fmt.Errorf("the home directory can't be found: %w", err))
	}
	operation := ""
	if len(args) == 1 {
		operation = args[0]
	}
	engines := enginedeploy.Engines{Home: home, Log: d.stderr, Build: func(out string) error {
		quiet := d
		quiet.stdout = d.stderr
		if runBuild(ctx, []string{"--out", out}, root, quiet) != 0 {
			return errors.New("go-build failed; the deploy's log holds its output")
		}
		return nil
	}}
	return engines.Serve(operation, d.stdin, d.stdout)
}
