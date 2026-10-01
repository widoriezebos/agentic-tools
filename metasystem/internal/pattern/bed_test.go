package pattern

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

// bed is one host with a landing lane: a nested landing checkout (the
// checkout root holds .git; its metasystem/ installation is the lane
// steward's repoRoot), the host home the lane record lives under, and one
// seat. Every reader is the production one.
type bed struct {
	t                     *testing.T
	home, top, repo, seat string
	sent                  []string
	pass                  Pass
}

var bedStart = time.Date(2026, 9, 30, 9, 0, 0, 0, time.UTC)

func newBed(t *testing.T) *bed {
	t.Helper()
	base := t.TempDir()
	b := &bed{t: t, home: filepath.Join(base, "home", ".metasystem"), top: filepath.Join(base, "landing"), seat: filepath.Join(base, "seat")}
	b.repo = filepath.Join(b.top, "metasystem")
	for _, dir := range []string{b.repo, filepath.Join(b.seat, ".git")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(b.repo, "metasystem.conf"), []byte("metasystem.template=true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "init", "-q", b.top).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	layout, err := lane.NewLayout(b.top)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := lane.Register(b.home, layout, "Wido", bedStart); err != nil {
		t.Fatal(err)
	}
	b.pass = Pass{Home: func() (string, error) { return b.home, nil },
		Deliver: func(_, message string) error { b.sent = append(b.sent, message); return nil }}
	return b
}

// setting writes one override into the lane installation's metasystem.conf.
func (b *bed) setting(line string) {
	b.t.Helper()
	path := filepath.Join(b.repo, "metasystem.conf")
	data, err := os.ReadFile(path)
	if err != nil {
		b.t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, []byte(line+"\n")...), 0o644); err != nil {
		b.t.Fatal(err)
	}
}

func (b *bed) writeRaw(path string, data []byte) {
	b.t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		b.t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		b.t.Fatal(err)
	}
}

func (b *bed) cycle(now time.Time) {
	b.t.Helper()
	if err := b.pass.Run(b.repo, now); err != nil {
		b.t.Fatalf("pattern pass at %s: %v", now.Format(time.RFC3339), err)
	}
}

func (b *bed) episodes() []steward.AlertEpisode {
	b.t.Helper()
	all, err := steward.AlertEpisodes(b.repo)
	if err != nil {
		b.t.Fatal(err)
	}
	var mine []steward.AlertEpisode
	for _, episode := range all {
		if steward.IsPatternOwner(episode.Owner) {
			mine = append(mine, episode)
		}
	}
	return mine
}

func (b *bed) open() []steward.AlertEpisode {
	var open []steward.AlertEpisode
	for _, episode := range b.episodes() {
		if !episode.Cleared {
			open = append(open, episode)
		}
	}
	return open
}

func (b *bed) takeHelm(root string) {
	b.t.Helper()
	if _, err := helm.Write(root, helm.Record{By: "Wido", At: bedStart.Format(time.RFC3339), Reason: "fixture"}); err != nil {
		b.t.Fatal(err)
	}
}

func (b *bed) returnHelm(root string) {
	b.t.Helper()
	if _, err := helm.Remove(root); err != nil {
		b.t.Fatal(err)
	}
}

var bedInvoker = steward.AlertInvoker{Pid: 4242, PidStartedAt: 77, UID: 501}
