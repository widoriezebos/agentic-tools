package delegation

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// Ported from the supervision bed's census gate (verb redesign U7b part 3),
// retargeted at the delegate lifecycle when dispatch.sh's
// require_fresh_census moved into it (batch 2): an absent verdict refuses
// before any engine gate runs, naming the absence and the re-arm command.
func TestSupCCensusGateAbsentVerdictRefuses(t *testing.T) {
	t.Parallel()
	const repo = "/fixture/census-gate-repo"
	var stderr bytes.Buffer
	root := t.TempDir()
	s := &session{
		l:         &Lifecycle{ports: Ports{Git: ownerGit{}}},
		ctx:       context.Background(),
		stderr:    &stderr,
		root:      root,
		agents:    filepath.Join(root, "artifacts", "agents"),
		repoScope: repo,
	}
	err := s.requireFreshCensus()
	var exit *Exit
	if !errors.As(err, &exit) || exit.Code != 1 {
		t.Fatalf("absent census did not refuse with exit 1: %v\n%s", err, stderr.String())
	}
	want := "dispatch refused: census verdict is absent; run metasystem system start --repo " + repo
	if !strings.Contains(stderr.String(), want) {
		t.Fatalf("absent census refusal = %q, want %q", stderr.String(), want)
	}
}
