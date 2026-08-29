package janitor

import (
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
)

const tag = "metasystem-supervision-owner-repo-123-watcher-2-456"

func observed(pid int64, startSec int64, argv ...string) identity.Exact {
	return identity.Exact{Pid: pid, StartedAt: time.Unix(startSec, 0), Argv: argv}
}

// Proof "Kill authority" (SLC-R5-010, SLC-R5-011) and "Triple kill
// proof" (SLC-R8-006), row by row.
func TestKillable(t *testing.T) {
	recorded := &registry.ProcessRef{Pid: 41, PidStartedAt: 100}
	watcherArgv := []string{"bash", "/repo/scripts/watch-background-jobs.sh", "--census", "--instance-tag", tag}
	cases := []struct {
		name     string
		observed identity.Exact
		recorded *registry.ProcessRef
		want     bool
	}{
		{
			name:     "real leaked component IS killable",
			observed: observed(41, 100, watcherArgv...),
			recorded: recorded,
			want:     true,
		},
		{
			name: "same-second pid reuse with WRONG argv is not",
			observed: observed(41, 100,
				"vim", "notes-about-"+tag+".md"),
			recorded: recorded,
			want:     false,
		},
		{
			name:     "recycled pid, different start second",
			observed: observed(41, 101, watcherArgv...),
			recorded: recorded,
			want:     false,
		},
		{
			// A shell QUOTING the tag matches no shape
			// because the tag is not the tag-flag's value.
			name: "tag mention in a grep is never killable",
			observed: observed(41, 100,
				"grep", "-rn", tag, "/repo"),
			recorded: recorded,
			want:     false,
		},
		{
			name:     "unreadable argv means report, not kill",
			observed: identity.Exact{Pid: 41, StartedAt: time.Unix(100, 0)},
			recorded: recorded,
			want:     false,
		},
		{
			// An establishment orphan has no recorded identity; the Go owner
			// shape with the claim's tag is proof.
			name: "signature-only owner proof",
			observed: observed(77, 999,
				"/repo/bin/metasystem", "supervise", "owner", "--repo", "/repo", "--tag", tag),
			recorded: nil,
			want:     true,
		},
		{
			name: "flag=value spelling matches",
			observed: observed(41, 100,
				"/repo/scripts/watch-background-jobs.sh", "--instance-tag="+tag),
			recorded: recorded,
			want:     true,
		},
		{
			name: "the go owner verb is a known shape",
			observed: observed(41, 100,
				"/usr/local/bin/metasystem", "supervise", "owner", "--tag", tag),
			recorded: recorded,
			want:     true,
		},
		{
			name: "wrong tag in the tag position",
			observed: observed(41, 100,
				"/repo/scripts/watch-background-jobs.sh", "--instance-tag", "some-other-tag"),
			recorded: recorded,
			want:     false,
		},
	}
	for _, row := range cases {
		t.Run(row.name, func(t *testing.T) {
			_, got := Killable(row.observed, row.recorded, DefaultShapes(), []string{tag})
			if got != row.want {
				t.Fatalf("killable=%v, want %v", got, row.want)
			}
		})
	}
}

func TestMatchShapeRequiresAllIncludes(t *testing.T) {
	// "dispatch.sh" alone must not match the reaper shape without its
	// "reap" subcommand — dispatch runs many verbs that are not
	// supervision components.
	argv := []string{"bash", "/repo/scripts/agents/dispatch.sh", "dispatch", "--instance-tag", tag}
	if _, ok := MatchShape(DefaultShapes(), argv, tag); ok {
		t.Fatal("a non-reap dispatch verb matched the reaper shape")
	}
}
