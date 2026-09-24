package steward

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNotifyCommandUsesPlatformForHumanEnrollments(t *testing.T) {
	for _, enrollment := range []string{EnrollmentHumanTerminal, EnrollmentTemporaryWord} {
		t.Run(enrollment, func(t *testing.T) {
			f := absentNotifyFixture(t, 1)
			f.darwin()
			human := f.root
			notifyIdentity(t, human, enrollment)
			if command, ok := notifyCommandWithDependencies(human, f.deps); !ok || command != "" {
				t.Fatalf("%s enrollment did not choose the platform notifier: command=%q ok=%v", enrollment, command, ok)
			}
		})
	}

	f := absentNotifyFixture(t, 1)
	f.darwin()
	fixture := f.root
	notifyIdentity(t, fixture, EnrollmentFixture)
	if command, ok := notifyCommandWithDependencies(fixture, f.deps); !ok || command != "" {
		t.Fatalf("fixture enrollment did not choose its local delivery channel: command=%q ok=%v", command, ok)
	}
	t.Run("empty command follows fixture fallback", func(t *testing.T) {
		f := newNotifyFixture(t, notifyRead{command: " "})
		notifyIdentity(t, f.root, EnrollmentFixture)
		if command, ok := notifyCommandWithDependencies(f.root, f.deps); !ok || command != "" {
			t.Fatalf("blank command did not choose fixture log: command=%q ok=%v", command, ok)
		}
	})
	t.Run("read error without fallback is unavailable", func(t *testing.T) {
		f := newNotifyFixture(t, notifyRead{err: errors.New("config unreadable")}, notifyRead{err: errors.New("config unreadable")})
		if command, ok := notifyCommandWithDependencies(f.root, f.deps); ok || command != "" {
			t.Fatalf("read error invented a channel: command=%q ok=%v", command, ok)
		}
		if err := f.deliver(f.root, "message"); err == nil {
			t.Fatal("read error without fallback claimed delivery")
		}
	})
}

func TestHumanDeliveryUsesRepositoryRootInPlatformTitle(t *testing.T) {
	f := absentNotifyFixture(t, 1)
	f.darwin()
	root := f.root
	notifyIdentity(t, root, EnrollmentHumanTerminal)
	if err := f.deliver(root, "worker dead"); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(f.invocation, " ")
	if !strings.Contains(joined, "osascript -e") || !strings.Contains(joined, "metasystem steward - "+canonicalPath(root)) {
		t.Fatalf("platform notification did not name its repository root: %q", joined)
	}
	wantScript := fmt.Sprintf("display notification %q with title %q", "worker dead", "metasystem steward - "+canonicalPath(root))
	if len(f.invocation) != 3 || f.invocation[0] != "osascript" || f.invocation[1] != "-e" || f.invocation[2] != wantScript {
		t.Fatalf("platform notifier invocation changed: got=%q want script=%q", f.invocation, wantScript)
	}
}

func TestFixtureDeliveryAppendsTimestampedLocalLog(t *testing.T) {
	f := absentNotifyFixture(t, 1)
	f.darwin()
	root := f.root
	notifyIdentity(t, root, EnrollmentFixture)
	if err := f.deliver(root, "HEALTH unhealthy"); err != nil {
		t.Fatal(err)
	}
	if len(f.invocation) != 0 {
		t.Fatalf("fixture delivery invoked the platform notifier: %v", f.invocation)
	}
	data, err := os.ReadFile(filepath.Join(root, "artifacts", "agents", "steward", "notifications.log"))
	if err != nil {
		t.Fatal(err)
	}
	fields := strings.Fields(string(data))
	if len(fields) < 3 || fields[1] != "HEALTH" || fields[2] != "unhealthy" {
		t.Fatalf("fixture notification log omitted its message: %q", data)
	}
	stamp, err := time.Parse(time.RFC3339, fields[0])
	if err != nil || stamp.Location() != time.UTC {
		t.Fatalf("fixture notification log did not start with a UTC timestamp: %q %v", fields[0], err)
	}
}

func TestConfiguredCommandWinsForEveryEnrollment(t *testing.T) {
	for _, enrollment := range []string{EnrollmentHumanTerminal, EnrollmentTemporaryWord, EnrollmentFixture} {
		t.Run(enrollment, func(t *testing.T) {
			f := configuredNotifyFixture(t, 2, "default")
			f.deps.platform = "darwin"
			root, sink := f.root, f.sink
			notifyIdentity(t, root, enrollment)
			command, ok := notifyCommandWithDependencies(root, f.deps)
			if !ok || command == "" {
				t.Fatalf("configured command was not resolved for %s", enrollment)
			}
			if err := f.deliver(root, enrollment); err != nil {
				t.Fatal(err)
			}
			if data, err := os.ReadFile(sink); err != nil || !strings.Contains(string(data), enrollment) {
				t.Fatalf("configured command did not receive %s: %q %v", enrollment, data, err)
			}
		})
	}
}

func TestMissingIdentityDeliversAsHuman(t *testing.T) {
	f := absentNotifyFixture(t, 1)
	f.darwin()
	root := f.root
	if err := f.deliver(root, "legacy installation"); err != nil {
		t.Fatal(err)
	}
	if len(f.invocation) == 0 || f.invocation[0] != "osascript" {
		t.Fatalf("missing identity did not retain platform delivery: %v", f.invocation)
	}
}

func TestDeliveryMeansTheCommandSucceeded(t *testing.T) {
	f := configuredNotifyFixture(t, 1, "default")
	root, sink := f.root, f.sink
	if err := f.deliver(root, "worker dead; reviving fix-it"); err != nil {
		t.Fatalf("a zero exit is a delivery: %v", err)
	}
	data, err := os.ReadFile(sink)
	if err != nil || !strings.Contains(string(data), "worker dead") {
		t.Fatalf("the message must reach the channel: %q %v", data, err)
	}
}

func TestFailedChannelIsNotADelivery(t *testing.T) {
	f := configuredNotifyFixture(t, 1, "exit 1")
	if err := f.deliver(f.root, "anything"); err == nil {
		t.Fatal("a failing notifier must not claim delivery")
	}
}

func TestPendingQueueDrainsOnDeliveryAndHoldsOnFailure(t *testing.T) {
	f := newNotifyFixture(t, notifyRead{command: "default"}, notifyRead{command: "default"}, notifyRead{command: "exit 1"})
	root, sink := f.root, f.sink
	for _, n := range []string{"a", "b"} {
		if err := QueueNotification(root, PendingNotification{Nonce: n, Message: "msg-" + n}); err != nil {
			t.Fatal(err)
		}
	}
	delivered, err := f.pending()
	if err != nil || delivered != 2 {
		t.Fatalf("both must deliver: %d %v", delivered, err)
	}
	if pending, _ := PendingNotifications(root); len(pending) != 0 {
		t.Fatalf("delivered messages leave the queue: %v", pending)
	}
	data, _ := os.ReadFile(sink)
	if !strings.Contains(string(data), "msg-a") || !strings.Contains(string(data), "msg-b") {
		t.Fatalf("both messages must reach the channel: %q", data)
	}
	if strings.Index(string(data), "msg-a") >= strings.Index(string(data), "msg-b") {
		t.Fatalf("queued notifications must reach the channel in order: %q", data)
	}

	// Break the channel: the queue holds and reports.
	if err := QueueNotification(root, PendingNotification{Nonce: "c", Message: "msg-c"}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pending(); err == nil {
		t.Fatal("a down channel must surface, not silently drop")
	}
	if pending, _ := PendingNotifications(root); len(pending) != 1 {
		t.Fatalf("undelivered messages stay queued: %v", pending)
	}
}

func TestPendingQueueRetiresAnObsoleteAlertBeforeRevival(t *testing.T) {
	f := newNotifyFixture(t)
	root, sink := f.root, f.sink
	nonce := "verdict-" + string(VerdictStalledDead)
	if err := QueueNotification(root, PendingNotification{Nonce: nonce, Message: "steward: stalled-dead — reviving"}); err != nil {
		t.Fatal(err)
	}
	delivered, err := f.pending()
	if err != nil || delivered != 0 {
		t.Fatalf("the old alert-before-heal intent must retire without delivery: %d %v", delivered, err)
	}
	if _, err := os.Stat(sink); !os.IsNotExist(err) {
		t.Fatalf("the obsolete recovery alert reached the notifier: %v", err)
	}
	if pending, err := PendingNotifications(root); err != nil || len(pending) != 0 {
		t.Fatalf("the obsolete recovery alert remained pending: %v %v", pending, err)
	}
}

func TestHealthAlertEpisodeDeduplicatesSubmissionAndRecordsAcknowledgment(t *testing.T) {
	f := configuredNotifyFixture(t, 1, "default")
	root, sink := f.root, f.sink
	first := evidenceDigest("runner stale")
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	unhealthy := HealthVerdict{Aggregate: "unhealthy", FindingDigest: first,
		Roles: []RoleVerdict{{Role: RoleStewardRunner, Status: HealthDead, Reason: "runner stale"}}}
	episode, err := f.alert(unhealthy, "HEALTH unhealthy — runner stale", now)
	if err != nil {
		t.Fatal(err)
	}
	if episode.TransportResult != TransportPending || len(episode.Attempts) != 0 {
		t.Fatalf("the first failure must open silent history before escalation: %+v", episode)
	}
	if _, err := os.Stat(sink); !os.IsNotExist(err) {
		t.Fatalf("a recoverable first failure must not notify the human: %v", err)
	}
	unhealthy.ShouldAlert = true
	episode, err = f.alert(unhealthy, "HEALTH unhealthy — runner stale", now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if episode.TransportResult != TransportSubmitted || len(episode.Attempts) != 1 {
		t.Fatalf("notifier zero records one transport submission: %+v", episode)
	}
	repeated, err := f.alert(unhealthy, "a later rendering", now.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if repeated.EpisodeID != episode.EpisodeID || len(repeated.Attempts) != 1 {
		t.Fatalf("the same digest stays one episode and one notification: first=%+v repeated=%+v", episode, repeated)
	}
	data, err := os.ReadFile(sink)
	if err != nil || strings.Count(strings.TrimSpace(string(data)), "\n") != 0 {
		t.Fatalf("one episode submits exactly one desktop notification: %q %v", data, err)
	}

	invoker := AlertInvoker{Pid: 9001, PidStartedAt: 77, UID: 501, ArgvDigest: evidenceDigest("human shell")}
	acknowledged, err := AcknowledgeAlert(root, episode.EpisodeID, invoker, now.Add(3*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if !acknowledged.Acknowledged || acknowledged.AcknowledgedBy == nil || acknowledged.AcknowledgedBy.Pid != invoker.Pid {
		t.Fatalf("acknowledgment must retain the observed invoker identity: %+v", acknowledged)
	}

	healthy := HealthVerdict{Aggregate: "healthy", FindingDigest: evidenceDigest(""), Roles: []RoleVerdict{{Role: RoleStewardRunner, Status: HealthAlive}}}
	if _, err := f.alert(healthy, "HEALTH healthy", now.Add(4*time.Minute)); err != nil {
		t.Fatal(err)
	}
	episodes, err := AlertEpisodes(root)
	if err != nil || len(episodes) != 1 || !episodes[0].Resolved || !episodes[0].Cleared || episodes[0].ClearedAt.IsZero() {
		t.Fatalf("a healthy verdict resolves and clears without deleting the episode: %+v %v", episodes, err)
	}
}

func TestPendingSubmissionJournalIsReusedAfterRecovery(t *testing.T) {
	f := configuredNotifyFixture(t, 1, "default")
	root, sink := f.root, f.sink
	now := time.Date(2026, 8, 28, 12, 30, 0, 0, time.UTC)
	digest := evidenceDigest("hook has no lawful remedy")
	episode := AlertEpisode{
		Schema: 1, EpisodeID: "alert-" + digest[:16] + "-1", Digest: digest,
		Message: "HEALTH unhealthy — hook failed", OpenedAt: now,
		Attempts:        []AlertAttempt{{Sequence: 1, AttemptedAt: now, Result: TransportPending}},
		TransportResult: TransportPending,
	}
	if err := os.MkdirAll(alertDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := saveAlertEpisode(root, episode); err != nil {
		t.Fatal(err)
	}
	health := HealthVerdict{Aggregate: "unhealthy", FindingDigest: digest, ShouldAlert: true,
		Roles: []RoleVerdict{{Role: RoleHookFreshness, Status: HealthDead, FailureEscalation: NoLawfulRemedy}}}
	recovered, err := f.alert(health, episode.Message, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if recovered.TransportResult != TransportSubmitted || len(recovered.Attempts) != 1 || recovered.Attempts[0].Sequence != 1 {
		t.Fatalf("recovery must finish the journaled submission instead of creating another: %+v", recovered)
	}
	if data, err := os.ReadFile(sink); err != nil || strings.Count(strings.TrimSpace(string(data)), "\n") != 0 {
		t.Fatalf("the recovered attempt submits once in this non-crash execution: %q %v", data, err)
	}
}

func TestFailedAlertSubmissionRetriesWithoutDeletingTheEpisode(t *testing.T) {
	f := newNotifyFixture(t, notifyRead{command: "exit 1"}, notifyRead{command: "default"})
	root := f.root
	now := time.Date(2026, 8, 28, 13, 0, 0, 0, time.UTC)
	health := HealthVerdict{Aggregate: "unhealthy", FindingDigest: evidenceDigest("watcher dead"), ShouldAlert: true,
		Roles: []RoleVerdict{{Role: RoleRepoWatcher, Status: HealthDead, Reason: "watcher dead"}}}
	failed, err := f.alert(health, "HEALTH unhealthy — watcher dead", now)
	if err != nil {
		t.Fatal(err)
	}
	if failed.TransportResult != TransportFailed || len(failed.Attempts) != 1 || failed.Acknowledged {
		t.Fatalf("a failed transport must leave its unacknowledged episode retryable: %+v", failed)
	}
	retried, err := f.alert(health, "HEALTH unhealthy — watcher dead", now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if retried.EpisodeID != failed.EpisodeID || retried.TransportResult != TransportSubmitted || len(retried.Attempts) != 2 {
		t.Fatalf("retry must retain the episode and append its transport result: failed=%+v retried=%+v", failed, retried)
	}
	episodes, err := AlertEpisodes(root)
	if err != nil || len(episodes) != 1 || episodes[0].Acknowledged {
		t.Fatalf("retry must never delete or invent acknowledgment for the episode: %+v %v", episodes, err)
	}
}

func TestLegacyHeldHealthNotificationMigratesIntoAnEpisode(t *testing.T) {
	f := newNotifyFixture(t)
	root := f.root
	now := time.Date(2026, 8, 29, 10, 0, 0, 0, time.UTC)
	digest := evidenceDigest("legacy watcher finding")
	if err := QueueNotification(root, PendingNotification{
		Nonce: digest, Message: "HEALTH unhealthy — legacy watcher finding", DeliveryOwner: legacyHealthDeliveryOwner,
	}); err != nil {
		t.Fatal(err)
	}
	if err := QueueNotification(root, PendingNotification{
		Nonce: "unrelated", Message: "ordinary notification", DeliveryOwner: "another-owner",
	}); err != nil {
		t.Fatal(err)
	}
	health := HealthVerdict{Aggregate: "unhealthy", FindingDigest: digest}
	episode, err := f.alert(health, "HEALTH unhealthy — legacy watcher finding", now)
	if err != nil {
		t.Fatal(err)
	}
	if episode.Digest != digest || episode.Message != "HEALTH unhealthy — legacy watcher finding" || episode.TransportResult != TransportPending {
		t.Fatalf("legacy notification did not become silent episode history: %+v", episode)
	}
	pending, err := PendingNotifications(root)
	if err != nil || len(pending) != 1 || pending[0].Nonce != "unrelated" {
		t.Fatalf("migration retired the wrong pending records: pending=%+v err=%v", pending, err)
	}
	second, err := f.alert(health, "later rendering", now.Add(time.Minute))
	if err != nil || second.EpisodeID != episode.EpisodeID {
		t.Fatalf("migration duplicated an open episode: first=%+v second=%+v err=%v", episode, second, err)
	}
}

func TestClearedFindingRecurrenceOpensANewEpisode(t *testing.T) {
	f := newNotifyFixture(t)
	root := f.root
	now := time.Date(2026, 8, 29, 11, 0, 0, 0, time.UTC)
	digest := evidenceDigest("runner stale")
	unhealthy := HealthVerdict{Aggregate: "unhealthy", FindingDigest: digest}
	first, err := f.alert(unhealthy, "runner stale", now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.alert(HealthVerdict{Aggregate: "healthy"}, "healthy", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	second, err := f.alert(unhealthy, "runner stale again", now.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if first.EpisodeID == second.EpisodeID || !strings.HasSuffix(second.EpisodeID, "-2") {
		t.Fatalf("recurrence reused cleared history: first=%+v second=%+v", first, second)
	}
	episodes, err := AlertEpisodes(root)
	if err != nil || len(episodes) != 2 || !episodes[0].Cleared || episodes[1].Cleared {
		t.Fatalf("recurrence history is incomplete: episodes=%+v err=%v", episodes, err)
	}
}

func TestNewFindingResolvesEarlierOpenEpisodeWithoutClearingIt(t *testing.T) {
	f := newNotifyFixture(t)
	root := f.root
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	firstDigest := evidenceDigest("watcher stale")
	secondDigest := evidenceDigest("runner stale")
	if _, err := f.alert(HealthVerdict{Aggregate: "unhealthy", FindingDigest: firstDigest}, "watcher stale", now); err != nil {
		t.Fatal(err)
	}
	if _, err := f.alert(HealthVerdict{Aggregate: "unhealthy", FindingDigest: secondDigest}, "runner stale", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	episodes, err := AlertEpisodes(root)
	if err != nil || len(episodes) != 2 || !episodes[0].Resolved || episodes[0].Cleared || episodes[1].Resolved {
		t.Fatalf("new finding did not resolve only its predecessor: episodes=%+v err=%v", episodes, err)
	}
}

func TestAlertEpisodeBoundariesRejectInvalidEvidenceAndInvoker(t *testing.T) {
	f := newNotifyFixture(t)
	root := f.root
	now := time.Now()
	if _, err := f.alert(HealthVerdict{Aggregate: "unhealthy", FindingDigest: "not-a-digest"}, "message", now); err == nil {
		t.Fatal("invalid finding digest opened an episode")
	}
	if _, err := f.alert(HealthVerdict{Aggregate: "unhealthy", FindingDigest: evidenceDigest("x")}, " ", now); err == nil {
		t.Fatal("blank finding message opened an episode")
	}
	if _, err := AcknowledgeAlert(root, "../escape", AlertInvoker{Pid: 1, PidStartedAt: 1}, now); err == nil {
		t.Fatal("invalid episode id was acknowledged")
	}
	if _, err := AcknowledgeAlert(root, "alert-valid-1", AlertInvoker{}, now); err == nil {
		t.Fatal("acknowledgment without an observed invoker was accepted")
	}
}

func TestAlertEpisodeLoaderRejectsMalformedAndIncompleteRecords(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(alertDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(alertDir(root), "alert-bad-1.json")
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := AlertEpisodes(root); err == nil || !strings.Contains(err.Error(), "malformed") {
		t.Fatalf("malformed episode was accepted: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"schema":1,"episodeId":"alert-bad-1"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := AlertEpisodes(root); err == nil || !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("incomplete episode was accepted: %v", err)
	}
}

func TestSeatIdleIncidentIsAStatusEpisodeNotAHumanAlarm(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	incident := SeatIdleIncident{
		SessionID: "seat-session", MainID: "main-1", GoalID: "next-goal",
		BacklogDigest: evidenceDigest("unchanged backlog"), Refusal: 3,
		StopHookActive: true, ClaimActor: "machine+lineage", ClaimMade: true,
		ClaimDetail: "claimed by machine+lineage", IntentID: "intent-1",
		IntentPrepared: true, IntentDetail: "prepared steward continuation intent intent-1",
	}
	episode, err := RecordSeatIdleIncident(root, incident, now)
	if err != nil {
		t.Fatal(err)
	}
	if episode.Owner != seatIdleAlertOwner || episode.SeatIdle == nil ||
		!episode.SeatIdle.StopHookActive || !episode.SeatIdle.IntentPrepared {
		t.Fatalf("the alert episode did not retain the refusal handoff: %+v", episode)
	}
	replayed, err := RecordSeatIdleIncident(root, incident, now.Add(time.Minute))
	if err != nil || replayed.EpisodeID != episode.EpisodeID {
		t.Fatalf("the unchanged backlog opened more than one incident: %+v %v", replayed, err)
	}
	incident.Refusal = 4
	incident.ClaimDetail = "updated refusal detail"
	updated, err := RecordSeatIdleIncident(root, incident, now.Add(2*time.Minute))
	if err != nil || updated.EpisodeID != episode.EpisodeID || updated.SeatIdle == nil ||
		updated.SeatIdle.Refusal != 4 || updated.SeatIdle.ClaimDetail != "updated refusal detail" ||
		!strings.Contains(updated.Message, "refusal 4") {
		t.Fatalf("the active seat-idle episode did not absorb the repeated refusal details: %+v %v", updated, err)
	}
	if pending, pendingErr := PendingNotifications(root); pendingErr != nil || len(pending) != 0 {
		t.Fatalf("the status incident also raised a human alarm: %+v %v", pending, pendingErr)
	}
}
