package hostsetup

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// The template repository exposes every adoptable host's contract and
// lifecycle configuration: setup's check mode validates all of the root's
// registrations against the registry without writing (ported from the
// retired validate-metasystem.sh agent-protocol section). Repository
// discovery is the checked-out tree itself; no Git runs.
func TestShippedRepositoryHostRegistrationsAreConfigurationReady(t *testing.T) {
	t.Parallel()
	repository, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if resolved, resolveErr := filepath.EvalSymlinks(repository); resolveErr == nil {
		repository = resolved
	}
	resolver := stateroot.NewResolver(func(string) (string, error) { return repository, nil },
		func() (string, error) {
			return "", errors.New("host registration check must not consult the executing engine")
		})
	result, err := SetupWithResolver(Options{RepositoryPath: repository, Check: true}, resolver.ResolveLayout)
	if err != nil {
		t.Fatalf("the template repository's host registrations are not configuration-ready: %v", err)
	}
	if len(result.Changed) != 0 {
		t.Fatalf("check mode reported changes: %v", result.Changed)
	}
	if len(result.Runtimes) == 0 {
		t.Fatal("check mode validated no host runtime")
	}
}
