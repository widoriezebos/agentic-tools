package main

import (
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

type processRefProber struct {
	exact identity.Exact
	state identity.Liveness
	err   error
}

func (p processRefProber) Probe(int64) (identity.Exact, identity.Liveness, error) {
	return p.exact, p.state, p.err
}
