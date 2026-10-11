package rehearsal

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// liveChildren includes detached descendants: each child inherits the private
// registry identity, executable or working directory after its parent exits.
func (s *Scratch) liveChildren() ([]identity.Exact, error) {
	if s.Registry == "" {
		return nil, nil
	}
	pids, err := s.processes.Pids()
	if err != nil {
		return nil, err
	}
	marker := "METASYSTEM_SUPERVISION_REGISTRY_HOME=" + filepath.Dir(s.Registry)
	var children []identity.Exact
	for _, pid := range pids {
		exact, state, err := (identity.KernelProber{}).Probe(pid)
		if err != nil {
			return nil, err
		}
		if state != identity.Alive || exact.Zombie || exact.Exiting {
			continue
		}
		cwd, known := identity.ProcessCwd(pid)
		inside := func(path string) bool {
			return path == s.Scratch || strings.HasPrefix(path, s.Scratch+string(filepath.Separator))
		}
		owned := slices.Contains(exact.Environ, marker) || known && inside(cwd) || exact.ExeKnown && inside(exact.Exe)
		for _, word := range exact.Argv {
			owned = owned || inside(word)
		}
		if owned {
			children = append(children, exact)
		}
	}
	return children, nil
}

func (s *Scratch) waitChildren(deadline time.Time) error {
	for {
		children, err := s.liveChildren()
		if err != nil {
			return fmt.Errorf("cannot establish rehearsal children ended: %w", err)
		}
		if len(children) == 0 {
			return nil
		}
		if !time.Now().Before(deadline) {
			return fmt.Errorf("rehearsal children still alive at cleanup deadline: %v", children[0].Ref())
		}
		time.Sleep(20 * time.Millisecond)
	}
}
