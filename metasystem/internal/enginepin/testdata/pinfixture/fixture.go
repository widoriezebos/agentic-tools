// Package pinfixture supplies native executable pin bytes and exact process
// observations without starting processes or depending on a live installation.
package pinfixture

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginebuild"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

func Pin(t testing.TB, root string, generation int, source string) (string, string) {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, []byte(enginebuild.StampRecord(source))...)
	sum := sha256.Sum256(data)
	dir := filepath.Join(root, "artifacts", "agents", "steward", "engine-pins")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, fmt.Sprintf("generation-%d-%x", generation, sum))
	if err := testexec.WriteFile(path, data, 0o500); err != nil {
		t.Fatal(err)
	}
	return path, fmt.Sprintf("sha256:%x", sum)
}

func Process(pid int64, executable string) identity.Exact {
	return identity.Exact{Pid: pid, StartedAt: time.Unix(1700000000, pid*1000), Exe: executable, ExeKnown: true}
}

type Prober map[int64]identity.Exact

func (p Prober) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	if exact, ok := p[pid]; ok {
		return exact, identity.Alive, nil
	}
	return identity.Exact{}, identity.Dead, nil
}
