package pattern

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

// trunkCommit is one commit a test writes to origin's main: its committer
// time, its Goal-Transaction (empty for hand work) and the one path it
// changes.
type trunkCommit struct {
	At   time.Time
	Opid string
	Path string
}

func lineageHash(lineage string) string {
	sum := sha256.Sum256([]byte(lineage))
	return hex.EncodeToString(sum[:4])
}

func opid(n int, machine, lineage string) string {
	return fmt.Sprintf("%026d-%s-%s", n, machine, lineageHash(lineage))
}

func (b *bed) git(dir string, args ...string) string {
	b.t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		b.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// origin makes a bare origin for the landing checkout, reached as
// "origin" the way a real landing checkout reaches GitHub.
func (b *bed) origin() string {
	b.t.Helper()
	bare := filepath.Join(filepath.Dir(b.top), "origin.git")
	if out, err := exec.Command("git", "init", "-q", "--bare", "-b", "main", bare).CombinedOutput(); err != nil {
		b.t.Fatalf("git init --bare: %v %s", err, out)
	}
	b.git(b.top, "remote", "add", "origin", bare)
	return bare
}

// push appends the commits to origin's main with git fast-import: each at
// its own committer time, with its trailer, changing its one path.
func (b *bed) push(bare string, commits []trunkCommit) {
	b.t.Helper()
	var stream bytes.Buffer
	_, err := exec.Command("git", "-C", bare, "rev-parse", "--verify", "-q", "refs/heads/main").Output()
	existing := err == nil
	for index, commit := range commits {
		message := fmt.Sprintf("commit %d\n", index)
		if commit.Opid != "" {
			message += "\nGoal-Transaction: " + commit.Opid + "\n"
		}
		content := fmt.Sprintf("%s %d\n", commit.At.Format(time.RFC3339Nano), index)
		fmt.Fprintf(&stream, "commit refs/heads/main\ncommitter Fixture <fixture@example.invalid> %d +0000\ndata %d\n%s", commit.At.Unix(), len(message), message)
		if index == 0 && existing {
			stream.WriteString("from refs/heads/main^0\n")
		}
		fmt.Fprintf(&stream, "M 100644 inline %s\ndata %d\n%s\n", commit.Path, len(content), content)
	}
	command := exec.Command("git", "-C", bare, "fast-import", "--quiet")
	command.Stdin = &stream
	if out, err := command.CombinedOutput(); err != nil {
		b.t.Fatalf("fast-import: %v\n%s", err, out)
	}
}

func (b *bed) churn() []steward.AlertEpisode {
	var churn []steward.AlertEpisode
	for _, episode := range b.episodes() {
		if episode.Owner == steward.PatternOwner(Churn) {
			churn = append(churn, episode)
		}
	}
	return churn
}

func openOf(episodes []steward.AlertEpisode) []steward.AlertEpisode {
	var open []steward.AlertEpisode
	for _, episode := range episodes {
		if !episode.Cleared {
			open = append(open, episode)
		}
	}
	return open
}

// The day's 24 lane commits of trunk-red.json, observed in two snapshots
// (18:16 and 18:40 CEST): the first sees four and reports nothing; the
// second sees the rest late, judges windows by committer time, and opens
// one episode whose burst is the sixth write, 16:17:03Z, attributed to the
// landing lane by the old owner's lineage hash (the replay's only use of
// it), with one notification.
func TestReplay20260930Churn(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.pass.LaneLineages = []string{LaneLineage, "landing-m1l"}
	bare := b.origin()
	data, err := os.ReadFile("testdata/replay-churn-20260930.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Commits []struct {
			CommittedAt     time.Time `json:"committedAt"`
			GoalTransaction string    `json:"goalTransaction"`
			Path            string    `json:"path"`
		} `json:"commits"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil || len(fixture.Commits) != 24 {
		t.Fatalf("fixture: %v, %d commits", err, len(fixture.Commits))
	}
	var early, late []trunkCommit
	first := time.Date(2026, 9, 30, 16, 16, 0, 0, time.UTC)
	for _, commit := range fixture.Commits {
		c := trunkCommit{At: commit.CommittedAt, Opid: commit.GoalTransaction, Path: commit.Path}
		if c.At.Before(first) {
			early = append(early, c)
		} else {
			late = append(late, c)
		}
	}
	b.push(bare, early)
	b.cycle(first)
	if churn := b.churn(); len(churn) != 0 || len(early) != 4 {
		t.Fatalf("four writes by 18:16 CEST reported: %d early, %+v", len(early), churn)
	}
	b.push(bare, late)
	b.cycle(time.Date(2026, 9, 30, 16, 40, 0, 0, time.UTC))
	churn := b.churn()
	if len(churn) != 1 || len(b.sent) != 1 {
		t.Fatalf("episodes %+v, notifications %q; want one of each", churn, b.sent)
	}
	episode := churn[0]
	if episode.ScopeID != "landing-2b626e27:metasystem/plans/goals/trunk-red.json" ||
		!strings.HasPrefix(b.sent[0], "The landing lane wrote the same record to main 6 times in 3 minutes") {
		t.Fatalf("episode %s says %q", episode.ScopeID, b.sent[0])
	}
	if episode.Digest != openingDigestFor(Churn, episode.ScopeID, time.Date(2026, 9, 30, 16, 17, 3, 0, time.UTC)) {
		t.Fatalf("the episode's burst is not the sixth write at 16:17:03Z")
	}
	if len(episode.Evidence) != 20 || episode.Evidence[0].At != "2026-09-30T16:14:18Z" || episode.Evidence[19].At != "2026-09-30T16:25:29Z" {
		t.Fatalf("evidence (first and latest of 24, capped at 20): %+v", episode.Evidence)
	}
	if _, err := os.Stat(filepath.Join(b.top, ".git", "FETCH_HEAD")); !os.IsNotExist(err) {
		t.Fatalf("the fetch wrote FETCH_HEAD: %v", err)
	}
	if b.git(b.top, "for-each-ref", "--format=%(refname)") != TrunkRef {
		t.Fatalf("the fetch touched a ref other than %s: %q", TrunkRef, b.git(b.top, "for-each-ref", "--format=%(refname)"))
	}
}

func openingDigestFor(pattern, work string, since time.Time) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{steward.PatternOwner(pattern), work, since.UTC().Format(time.RFC3339Nano)}, "\n")))
	return hex.EncodeToString(sum[:])
}

// Seven days of main like the calibration sample (seats write their goal
// files at most four times in ten minutes; the lane once writes
// trunk-red.json 21 times in ten minutes), observed four times a day: one
// episode, the lane's, which clears once main is quiet.
func TestSevenDayMainOneEpisode(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	bare := b.origin()
	start := time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC)
	var commits []trunkCommit
	n := 0
	for day := 0; day < 7; day++ {
		for hour := 8; hour < 20; hour += 2 {
			for _, seat := range []string{"m1e", "ui"} {
				for write := 0; write < 4; write++ {
					n++
					commits = append(commits, trunkCommit{At: start.Add(time.Duration(day*24+hour)*time.Hour + time.Duration(write*2)*time.Minute),
						Opid: opid(n, seat, "main-"+seat), Path: fmt.Sprintf("metasystem/plans/goals/goal-%s-%d.json", seat, day)})
				}
			}
		}
	}
	burst := start.Add(6*24*time.Hour + 16*time.Hour + 14*time.Minute)
	for write := 0; write < 21; write++ {
		n++
		commits = append(commits, trunkCommit{At: burst.Add(time.Duration(write*25) * time.Second), Opid: opid(n, "landing", LaneLineage), Path: "metasystem/plans/goals/trunk-red.json"})
	}
	sortCommits(commits)
	for cycle := start.Add(6 * time.Hour); cycle.Before(start.Add(7*24*time.Hour + 12*time.Hour)); cycle = cycle.Add(6 * time.Hour) {
		var due []trunkCommit
		for len(commits) > 0 && !commits[0].At.After(cycle) {
			due, commits = append(due, commits[0]), commits[1:]
		}
		if len(due) > 0 {
			b.push(bare, due)
		}
		b.cycle(cycle)
	}
	churn := b.churn()
	if len(churn) != 1 || len(b.sent) != 1 || churn[0].ScopeID != "landing-"+lineageHash(LaneLineage)+":metasystem/plans/goals/trunk-red.json" {
		t.Fatalf("seven days of main: %+v, notifications %q", churn, b.sent)
	}
	if len(openOf(churn)) != 0 {
		t.Fatalf("the lane's episode did not clear once main was quiet: %+v", churn)
	}
}

func sortCommits(commits []trunkCommit) {
	for i := 1; i < len(commits); i++ {
		for j := i; j > 0 && commits[j].At.Before(commits[j-1].At); j-- {
			commits[j], commits[j-1] = commits[j-1], commits[j]
		}
	}
}

// A commit without a Goal-Transaction trailer is hand work and never
// counted, however often it writes one record.
func TestHandCommitsNeverCount(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	bare := b.origin()
	at := time.Date(2026, 9, 30, 16, 0, 0, 0, time.UTC)
	var commits []trunkCommit
	for write := 0; write < 10; write++ {
		commits = append(commits, trunkCommit{At: at.Add(time.Duration(write*10) * time.Second), Path: "metasystem/records/narrator-digest.log"})
	}
	commits = append(commits, trunkCommit{At: at.Add(2 * time.Minute), Opid: opid(1, "m1e", "main-m1e"), Path: "metasystem/records/narrator-digest.log"})
	b.push(bare, commits)
	b.cycle(at.Add(5 * time.Minute))
	if churn := b.churn(); len(churn) != 0 || len(b.sent) != 0 {
		t.Fatalf("hand commits counted: %+v", churn)
	}
}

// A fetch older than max-gap is stale: the open episode neither clears nor
// grows, and says its signal cannot be read; a fresh fetch clears it after
// a quiet window.
func TestStaleFetchIsUnknown(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	bare := b.origin()
	at := time.Date(2026, 9, 30, 16, 0, 0, 0, time.UTC)
	var commits []trunkCommit
	for write := 0; write < 6; write++ {
		commits = append(commits, trunkCommit{At: at.Add(time.Duration(write*20) * time.Second), Opid: opid(write, "landing", LaneLineage), Path: "metasystem/plans/goals/trunk-red.json"})
	}
	b.push(bare, commits)
	b.cycle(at.Add(5 * time.Minute))
	if open := openOf(b.churn()); len(open) != 1 {
		t.Fatalf("no churn episode: %+v", b.churn())
	}
	moved := bare + ".moved"
	if err := os.Rename(bare, moved); err != nil {
		t.Fatal(err)
	}
	// Within max-gap of the last fetch the old ref is still usable, but a
	// failed fetch never observes a quiet window.
	b.cycle(at.Add(15 * time.Minute))
	for _, minute := range []int{25, 35, 45, 55} {
		b.cycle(at.Add(time.Duration(minute) * time.Minute))
	}
	open := openOf(b.churn())
	if len(open) != 1 || open[0].Standing != steward.StandingUnreadable || open[0].CleanCount != 0 {
		t.Fatalf("a stale fetch moved the episode: %+v", b.churn())
	}
	if err := os.Rename(moved, bare); err != nil {
		t.Fatal(err)
	}
	b.cycle(at.Add(65 * time.Minute))
	b.cycle(at.Add(75 * time.Minute))
	if open := openOf(b.churn()); len(open) != 0 {
		t.Fatalf("two fresh quiet windows did not clear the episode: %+v", open)
	}

	// Staleness is the fetch's age, not the burst's: past max-gap, a burst
	// still inside the window reads Unknown, never a finding from a stale ref.
	c := newBed(t)
	c.setting("steward.pattern.max-gap-sec=120")
	cbare := c.origin()
	c.push(cbare, commits)
	c.cycle(at.Add(110 * time.Second))
	if err := os.Rename(cbare, cbare+".moved"); err != nil {
		t.Fatal(err)
	}
	c.cycle(at.Add(4 * time.Minute))
	if open := openOf(c.churn()); len(open) != 1 || open[0].Standing != steward.StandingUnreadable {
		t.Fatalf("a fetch older than max-gap was read: %+v", c.churn())
	}
}

// Attribution is by the opid's lineage hash alone: the lane's stable
// identity is the landing lane; the old owner's hash, a machine named
// landing, or a name that starts like the lane are each a machine.
func TestLaneAttributionByLineageHash(t *testing.T) {
	t.Parallel()
	if lineageHash(LaneLineage) != "106adb03" || lineageHash("landing-m1l") != "2b626e27" {
		t.Fatalf("hash8 of the lane identities: %s %s", lineageHash(LaneLineage), lineageHash("landing-m1l"))
	}
	b := newBed(t)
	bare := b.origin()
	at := time.Date(2026, 9, 30, 16, 0, 0, 0, time.UTC)
	writers := []struct{ machine, lineage, path string }{
		{"m1e", LaneLineage, "a.json"},          // the lane's identity, whatever the machine
		{"landing", "landing-m1l", "b.json"},    // the deleted owner: a machine outside the replay
		{"landing", "landing-lane-2", "c.json"}, // a name that starts like the lane
		{"landing-lane", "main-seat", "d.json"}, // a machine named like the lane
	}
	var commits []trunkCommit
	for index, writer := range writers {
		for write := 0; write < 6; write++ {
			commits = append(commits, trunkCommit{At: at.Add(time.Duration(index*6+write) * 10 * time.Second), Opid: opid(index*10+write, writer.machine, writer.lineage), Path: writer.path})
		}
	}
	sortCommits(commits)
	b.push(bare, commits)
	b.cycle(at.Add(8 * time.Minute))
	said := map[string]string{}
	for _, episode := range b.churn() {
		line, _, _ := strings.Cut(episode.Message, "\n")
		said[episode.ScopeID[strings.LastIndex(episode.ScopeID, ":")+1:]] = line
	}
	for path, want := range map[string]string{
		"a.json": "The landing lane wrote", "b.json": "Machine landing wrote", "c.json": "Machine landing wrote", "d.json": "Machine landing-lane wrote",
	} {
		if !strings.HasPrefix(said[path], want) {
			t.Errorf("%s: %q; want %q", path, said[path], want)
		}
	}
}
