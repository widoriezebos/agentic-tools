package main

import (
	"os"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

func TestTestRunWorkerEnvironmentCarriesTheRunOwner(t *testing.T) {
	environment, err := testingWorkerEnvironment([]string{"PATH=/fixture/bin"})
	if err != nil {
		t.Fatal(err)
	}
	var value string
	for _, entry := range environment {
		if name, candidate, ok := strings.Cut(entry, "="); ok && name == identity.RunOwnerEnv {
			value = candidate
		}
	}
	ref, err := identity.ParseRef(value)
	if err != nil || ref.Pid != int64(os.Getpid()) || identity.AliveRef(identity.KernelProber{}, ref) != identity.Alive {
		t.Fatalf("test run worker owner = %q, ref=%+v, err=%v", value, ref, err)
	}
}
