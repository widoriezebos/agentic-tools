package delegation_test

// Ported scenarios of dispatch-fixtures.sh lines 1863-2441 (dispatch
// cluster a, second part). Git is stubbed in every test here; the
// scenarios whose subject reads Git live in port_p3_integration_test.go.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/protocol"
)

// newAssetBed is a stubbed-Git bed with the shipped roles, permission
// presets and role packets installed and the fake-runtime configuration
// (plus extra lines) in place: enough for a dispatch refusal that precedes
// every Git read to run its real admissions.
func newAssetBed(t *testing.T, extraConf string) *bed {
	t.Helper()
	b := newBed(t)
	if err := os.WriteFile(filepath.Join(b.root, "metasystem.conf"),
		[]byte(dispatchBedConfig+extraConf+"evidence.root="+filepath.Join(b.root, "evidence")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var packets struct {
		Roles map[string]struct {
			Sources []struct {
				Path string `json:"path"`
			} `json:"sources"`
		} `json:"roles"`
	}
	if err := json.Unmarshal(protocol.RolePackets(), &packets); err != nil {
		t.Fatalf("role packets: %v", err)
	}
	for _, role := range packets.Roles {
		for _, source := range role.Sources {
			if !protocol.IsReference(source.Path) && !exists(filepath.Join(b.root, source.Path)) {
				b.installAsset(source.Path)
			}
		}
	}
	return b
}

// requireNothingPublished asserts a refused dispatch left no record, job
// log, payload, heartbeat or worktree for job.
func requireNothingPublished(t *testing.T, b *bed, job string) {
	t.Helper()
	for _, relative := range []string{
		"artifacts/agents/jobs/" + job + ".json", "artifacts/agents/jobs/" + job + ".log",
		"artifacts/agents/" + job, "artifacts/agents/hb/" + job, "artifacts/agents/hb/" + job + ".start",
		"artifacts/agents/worktrees/" + job,
	} {
		if exists(filepath.Join(b.root, relative)) {
			t.Fatalf("the refused dispatch of %s published %s", job, relative)
		}
	}
}

// Lines 2300-2320: a forbidden packet source is a pure preflight. It
// refuses with the typed REFUSED-CONTEXT-SOURCE before the guard, the
// lease, any Git (so no worktree, agent branch or alternates line), and
// before any reservation exists; the worktree request changes nothing.
func TestDispatchRefusesAForbiddenPacketSourceBeforeAnyState(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		argv []string
	}{
		{"delegate-forbidden-source", []string{"--role", "design-critic", "--outputs", "outputs.txt", "--design", "plans/designs/subject.md"}},
		{"delegate-forbidden-worktree", []string{"--role", "implementer", "--worktree"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b := newAssetBed(t, "")
			brief := b.brief("brief.md", "design", "Review.")
			argv := append([]string{"dispatch"}, tc.argv...)
			argv = append(argv, "--brief", brief, "--job-id", tc.name, "--source", "docs/project-rules.md", "--destructive-reach", "MECHANICAL")
			result := b.run(argv...)
			requireExit(t, result, 9, b.stderr.String())
			if !strings.Contains(string(result.Stdout), `"outcome":"REFUSED-CONTEXT-SOURCE"`) {
				t.Fatalf("stdout %q", result.Stdout)
			}
			if outcome := outcomeOf(t, result); outcome["outcome"] != "REFUSED-CONTEXT-SOURCE" {
				t.Fatalf("typed outcome %v", outcome)
			}
			requireNothingPublished(t, b, tc.name)
			if len(b.doubles.Git.Calls) != 0 || len(b.calls("guard.")) != 0 || len(b.calls("lease.")) != 0 {
				t.Fatalf("the source preflight reached Git, the guard or the lease: %v %v", b.doubles.Git.Calls, b.doubles.Log.Calls())
			}
		})
	}
}

// Lines 2326-2331: the dispatch selection refusals that precede the
// reservation, each naming its cause, none leaving a record.
func TestDispatchSelectionRefusalsNameTheirCause(t *testing.T) {
	t.Parallel()
	b := newAssetBed(t, "model.tier.1=fake:fake-model\nmodel.tier.2=fake:fake-premium\n")
	brief := b.brief("brief.md", "design", "Review.")
	b.writeFile("outputs.txt", "plans/designs/subject.md\n")
	base := []string{"dispatch", "--role", "design-critic", "--outputs", filepath.Join(b.root, "outputs.txt"),
		"--design", "plans/designs/subject.md", "--brief", brief, "--destructive-reach", "MECHANICAL"}
	for _, tc := range []struct {
		name  string
		extra []string
		code  int
		want  string
	}{
		{"malformed-job-id", []string{"--job-id", "Bad_Id"}, 2, "invalid job id: Bad_Id"},
		{"contradictory-mode", []string{"--mode", "verify"}, 1, "contradicts the brief's Working Mode"},
		{"unregistered-override", []string{"--runtime", "ghost"}, 1, "outside metasystem.runtimes"},
		{"main-override", []string{"--runtime", "main"}, 1, "assigned to main"},
		{"costlier-unmapped", []string{"--model", "absent-from-tier"}, 1, "(cost unranked"},
		{"ranked-costlier", []string{"--model", "fake-premium"}, 1, "higher (tier 1 -> tier 2)"},
	} {
		result := b.run(append(append([]string{}, base...), tc.extra...)...)
		if result.ExitCode != tc.code || !strings.Contains(b.stderr.String(), tc.want) {
			t.Fatalf("%s: exit %d stderr %q; want %d with %q", tc.name, result.ExitCode, b.stderr.String(), tc.code, tc.want)
		}
		entries, _ := os.ReadDir(filepath.Join(b.root, "artifacts", "agents", "jobs"))
		if len(entries) != 0 {
			t.Fatalf("%s left job state: %v", tc.name, entries)
		}
	}
}
