package steward

// The lane-silent signal (design blocked-agent-asks-the-human, Decision 4):
// the host landing lane has queued work, no proof runs, it is not paused,
// no lane question is open, and nothing has moved for laneSilentAfter.
// Progress is the newest of a proof start (running.json), a proof end
// (results.jsonl), a push (pushes.jsonl), a return (queue.jsonl) or a lane
// question opened. The steward that keeps the lane reads it; each silent
// interval is one episode, posted once to the channel.

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/narratordigest"
)

// laneSilentAfter is how long the lane may be without progress while work
// waits; the lane is checked sooner than seats because it blocks them all.
const laneSilentAfter = 20 * time.Minute

const laneSilentAlertOwner = "lane-silent"

// LaneSilence is one reading of the host landing lane.
type LaneSilence struct {
	// Known is false when the lane could not be read: standing episodes are
	// left as they are, and none opens.
	Known  bool
	Silent bool
	// Since is when the silence began: the newest progress, and never
	// before the oldest queued hand-in (the silence is counted while work
	// waits).
	Since  time.Time
	Queued int
}

// hostLaneSilence reads this host's lane for the steward of self.
func hostLaneSilence(self string, now time.Time) LaneSilence {
	home, err := board.Home()
	if err != nil {
		return LaneSilence{}
	}
	return readLaneSilenceWithDesignChecks(home, self, now, narratordigest.Append, os.Stderr)
}

func readLaneSilenceWithDesignChecks(home, self string, now time.Time, appendDigest func(string, []narratordigest.Entry, time.Time) error, output io.Writer) LaneSilence {
	carryLaneDesignChecks(home, self, now, appendDigest, output)
	return readLaneSilence(home, self, now)
}

// carryLaneDesignChecks offers the lane's warnings on every tick. The digest
// keeps exact retries once, so a checkout reset needs no separate cursor.
func carryLaneDesignChecks(home, self string, now time.Time, appendDigest func(string, []narratordigest.Entry, time.Time) error, output io.Writer) {
	record, registered, err := lane.Read(home)
	if err != nil || !registered || !lane.OwnsLane(self, record) {
		return
	}
	checks, err := plain.DesignChecks(record.Install)
	var entries []narratordigest.Entry
	for _, check := range checks {
		if check.Goal != "" && check.Commit != "" && check.Reason != "" {
			entries = append(entries, narratordigest.Entry{Kind: "lowlight", Text: check.Reason, SourceType: "design-gate-landing", SourceID: check.Goal + "@" + check.Commit})
		}
	}
	if err == nil {
		err = appendDigest(self, entries, now)
	}
	if err != nil {
		fmt.Fprintf(output, "warning: the lane's design warnings could not reach the narrator (%s); the tick goes on\nnothing to do: the next tick retries\n", strings.Join(strings.Fields(err.Error()), " "))
	}
}

// readLaneSilence reads the lane registered under home, for the steward of
// self: only the steward that keeps the lane reads it silent.
func readLaneSilence(home, self string, now time.Time) LaneSilence {
	record, registered, err := lane.Read(home)
	if err != nil {
		return LaneSilence{}
	}
	if !registered || !lane.OwnsLane(self, record) {
		return LaneSilence{Known: true}
	}
	layout, err := record.Layout()
	if err != nil {
		return LaneSilence{}
	}
	install, checkout := string(layout.Install), string(layout.Checkout)
	if _, paused := lane.ReadPause(home); paused {
		return LaneSilence{Known: true}
	}
	var progress time.Time
	advance := func(stamp string) {
		if at, err := time.Parse(time.RFC3339, stamp); err == nil && at.After(progress) {
			progress = at
		}
	}
	running, recorded, alive, err := plain.ReadRunning(install, plain.ProveSeams{})
	if err != nil {
		return LaneSilence{}
	}
	if recorded && alive {
		return LaneSilence{Known: true}
	}
	if recorded {
		advance(running.Since)
	}
	pending, err := plain.Pending(install, checkout)
	if err != nil {
		return LaneSilence{}
	}
	if len(pending) == 0 {
		return LaneSilence{Known: true}
	}
	open, opened, err := laneQuestions(install)
	if err != nil {
		return LaneSilence{}
	}
	if open {
		return LaneSilence{Known: true}
	}
	if !opened.IsZero() && opened.After(progress) {
		progress = opened
	}
	if last, ok, err := plain.LastResult(install); err != nil {
		return LaneSilence{}
	} else if ok {
		advance(last.At)
	}
	if last, ok, err := plain.LastPush(install); err != nil {
		return LaneSilence{}
	} else if ok {
		advance(last.At)
	}
	entries, err := plain.Entries(install)
	if err != nil {
		return LaneSilence{}
	}
	for _, entry := range entries {
		if entry.State == plain.StateReturned {
			advance(entry.ReturnedAt)
		}
	}
	var queuedAt time.Time
	for _, entry := range pending {
		if at, err := time.Parse(time.RFC3339, entry.At); err == nil && (queuedAt.IsZero() || at.Before(queuedAt)) {
			queuedAt = at
		}
	}
	if queuedAt.After(progress) {
		progress = queuedAt
	}
	return LaneSilence{Known: true, Silent: !now.Before(progress.Add(laneSilentAfter)), Since: progress.UTC(), Queued: len(pending)}
}

// laneQuestions reads the lane installation's question records for the
// questions about the lane (`question ask --about lane`): whether one is
// open, and when the newest was opened. The records are read as they are;
// a record that does not decode is skipped.
func laneQuestions(install string) (open bool, newest time.Time, err error) {
	paths, err := filepath.Glob(filepath.Join(install, "artifacts", "agents", "channel", "questions", "*.json"))
	if err != nil {
		return false, time.Time{}, err
	}
	for _, path := range paths {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			continue
		}
		var record struct {
			About    string    `json:"about"`
			State    string    `json:"state"`
			OpenedAt time.Time `json:"openedAt"`
		}
		if json.Unmarshal(data, &record) != nil || record.About != "lane" {
			continue
		}
		if record.State == "open" {
			open = true
		}
		if record.OpenedAt.After(newest) {
			newest = record.OpenedAt
		}
	}
	return open, newest, nil
}

func laneSilentNotice(reading LaneSilence, now time.Time) string {
	minutes := int(now.Sub(reading.Since) / time.Minute)
	work := "1 hand-in waits"
	if reading.Queued != 1 {
		work = fmt.Sprintf("%d hand-ins wait", reading.Queued)
	}
	return fmt.Sprintf("The landing lane has not moved for %d minutes while %s.\nmetasystem landing status", minutes, work)
}

// updateLaneSilentEpisodes opens one episode per silent interval and posts
// it once to the channel; an interval that ended (progress, nothing
// queued, a proof, a pause or a lane question) clears its episode. With no
// channel configured nothing opens.
func updateLaneSilentEpisodes(repoRoot string, reading LaneSilence, now time.Time) error {
	if !reading.Known {
		return nil
	}
	digest := ""
	if reading.Silent {
		digest = signalDigest(laneSilentAlertOwner, reading.Since.Format(time.RFC3339))
	} else if _, err := os.Stat(alertDir(repoRoot)); os.IsNotExist(err) {
		// Nothing silent and no episode store: nothing to open or clear.
		return nil
	}
	held, err := lockAlerts(repoRoot, lock.Exclusive)
	if err != nil {
		return err
	}
	defer unlockAlerts(held)
	episodes, err := loadAlertEpisodesUnlocked(repoRoot)
	if err != nil {
		return err
	}
	var current *AlertEpisode
	for index := range episodes {
		episode := &episodes[index]
		if episode.Owner != laneSilentAlertOwner {
			continue
		}
		if episode.Digest == digest {
			current = episode
			continue
		}
		if episode.Cleared {
			continue
		}
		episode.Resolved, episode.ResolvedAt = true, now.UTC()
		episode.Cleared, episode.ClearedAt = true, now.UTC()
		if err := saveAlertEpisode(repoRoot, *episode); err != nil {
			return err
		}
	}
	if !reading.Silent {
		return nil
	}
	channel := loadSignalChannel(repoRoot)
	if !channel.configured {
		return nil
	}
	if current == nil {
		episodes = append(episodes, AlertEpisode{
			Schema: 1, EpisodeID: nextEpisodeID(digest, episodes), Digest: digest,
			Owner: laneSilentAlertOwner, ScopeID: "landing-lane", Message: laneSilentNotice(reading, now),
			OpenedAt: now.UTC(), Attempts: []AlertAttempt{}, TransportResult: TransportPending,
		})
		current = &episodes[len(episodes)-1]
		if err := saveAlertEpisode(repoRoot, *current); err != nil {
			return err
		}
	}
	if current.Cleared || current.TransportResult == TransportSubmitted {
		return nil
	}
	return submitEpisode(repoRoot, current, now, channel.transport(current.Message))
}
