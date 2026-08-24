package main

// The composition root's driver wiring (acp-adapter-seam slice two):
// the native ACP driver registers here, per runtime, with the
// adapter-owned dialect arriving as data — the import direction is
// the root importing both packages, never acp importing adapter. A
// runtime gains a native driver with one line the day its
// ExpectedACP capabilities land in the runtimes registry.

import (
	"fmt"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/acp"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/adapter"
)

func init() {
	acp.RegisterNative("devin", func(tools string) (string, error) {
		dialect, err := adapter.ACPDialectFor("devin")
		if err != nil {
			return "", err
		}
		mode := dialect.ModeForTools[tools]
		if mode == "" {
			return "", fmt.Errorf("devin acp dialect maps no mode for tools grade %q", tools)
		}
		return mode, nil
	})
}
