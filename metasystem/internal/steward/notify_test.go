package steward

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// notifyRepo is a git repository whose notify-command appends to a
// sink file — a fully observable delivery channel.
func notifyRepo(t *testing.T, command string) (string, string) {
	t.Helper()
	root := t.TempDir()
	if out, err := exec.Command("git", "-C", root, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}
	sink := filepath.Join(t.TempDir(), "delivered.log")
	if command == "" {
		command = `printf '%s\n' "$STEWARD_MESSAGE" >> ` + sink
	}
	if out, err := exec.Command("git", "-C", root, "config", "metasystem.steward.notify-command", command).CombinedOutput(); err != nil {
		t.Fatalf("config: %v\n%s", err, out)
	}
	return root, sink
}

func TestDeliveryMeansTheCommandSucceeded(t *testing.T) {
	root, sink := notifyRepo(t, "")
	if err := Deliver(root, "worker dead; reviving fix-it"); err != nil {
		t.Fatalf("a zero exit is a delivery: %v", err)
	}
	data, err := os.ReadFile(sink)
	if err != nil || !strings.Contains(string(data), "worker dead") {
		t.Fatalf("the message must reach the channel: %q %v", data, err)
	}
}

func TestFailedChannelIsNotADelivery(t *testing.T) {
	root, _ := notifyRepo(t, "exit 1")
	if err := Deliver(root, "anything"); err == nil {
		t.Fatal("a failing notifier must not claim delivery")
	}
}

func TestPendingQueueDrainsOnDeliveryAndHoldsOnFailure(t *testing.T) {
	root, sink := notifyRepo(t, "")
	for _, n := range []string{"a", "b"} {
		if err := QueueNotification(root, PendingNotification{Nonce: n, Message: "msg-" + n}); err != nil {
			t.Fatal(err)
		}
	}
	delivered, err := DeliverPending(root)
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

	// Break the channel: the queue holds and reports.
	if out, err := exec.Command("git", "-C", root, "config", "metasystem.steward.notify-command", "exit 1").CombinedOutput(); err != nil {
		t.Fatalf("reconfig: %v\n%s", err, out)
	}
	if err := QueueNotification(root, PendingNotification{Nonce: "c", Message: "msg-c"}); err != nil {
		t.Fatal(err)
	}
	if _, err := DeliverPending(root); err == nil {
		t.Fatal("a down channel must surface, not silently drop")
	}
	if pending, _ := PendingNotifications(root); len(pending) != 1 {
		t.Fatalf("undelivered messages stay queued: %v", pending)
	}
}

func TestPendingQueueRetiresAnObsoleteAlertBeforeRevival(t *testing.T) {
	root, sink := notifyRepo(t, "")
	nonce := "verdict-" + string(VerdictStalledDead)
	if err := QueueNotification(root, PendingNotification{Nonce: nonce, Message: "steward: stalled-dead — reviving"}); err != nil {
		t.Fatal(err)
	}
	delivered, err := DeliverPending(root)
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
	root, sink := notifyRepo(t, "")
	first := evidenceDigest("runner stale")
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	unhealthy := HealthVerdict{Aggregate: "unhealthy", FindingDigest: first,
		Roles: []RoleVerdict{{Role: RoleStewardRunner, Status: HealthDead, Reason: "runner stale"}}}
	episode, err := UpdateAlertEpisodes(root, unhealthy, "HEALTH unhealthy — runner stale", now)
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
	episode, err = UpdateAlertEpisodes(root, unhealthy, "HEALTH unhealthy — runner stale", now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if episode.TransportResult != TransportSubmitted || len(episode.Attempts) != 1 {
		t.Fatalf("notifier zero records one transport submission: %+v", episode)
	}
	repeated, err := UpdateAlertEpisodes(root, unhealthy, "a later rendering", now.Add(2*time.Minute))
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
	if _, err := UpdateAlertEpisodes(root, healthy, "HEALTH healthy", now.Add(4*time.Minute)); err != nil {
		t.Fatal(err)
	}
	episodes, err := AlertEpisodes(root)
	if err != nil || len(episodes) != 1 || !episodes[0].Resolved || !episodes[0].Cleared || episodes[0].ClearedAt.IsZero() {
		t.Fatalf("a healthy verdict resolves and clears without deleting the episode: %+v %v", episodes, err)
	}
}

func TestPendingSubmissionJournalIsReusedAfterRecovery(t *testing.T) {
	root, sink := notifyRepo(t, "")
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
	recovered, err := UpdateAlertEpisodes(root, health, episode.Message, now.Add(time.Minute))
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
	root, sink := notifyRepo(t, "exit 1")
	now := time.Date(2026, 8, 28, 13, 0, 0, 0, time.UTC)
	health := HealthVerdict{Aggregate: "unhealthy", FindingDigest: evidenceDigest("watcher dead"), ShouldAlert: true,
		Roles: []RoleVerdict{{Role: RoleRepoWatcher, Status: HealthDead, Reason: "watcher dead"}}}
	failed, err := UpdateAlertEpisodes(root, health, "HEALTH unhealthy — watcher dead", now)
	if err != nil {
		t.Fatal(err)
	}
	if failed.TransportResult != TransportFailed || len(failed.Attempts) != 1 || failed.Acknowledged {
		t.Fatalf("a failed transport must leave its unacknowledged episode retryable: %+v", failed)
	}
	command := `printf '%s\n' "$STEWARD_MESSAGE" >> ` + sink
	if out, err := exec.Command("git", "-C", root, "config", "metasystem.steward.notify-command", command).CombinedOutput(); err != nil {
		t.Fatalf("reconfigure notifier: %v\n%s", err, out)
	}
	retried, err := UpdateAlertEpisodes(root, health, "HEALTH unhealthy — watcher dead", now.Add(time.Minute))
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
	root, _ := notifyRepo(t, "true")
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
	episode, err := UpdateAlertEpisodes(root, health, "HEALTH unhealthy — legacy watcher finding", now)
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
	second, err := UpdateAlertEpisodes(root, health, "later rendering", now.Add(time.Minute))
	if err != nil || second.EpisodeID != episode.EpisodeID {
		t.Fatalf("migration duplicated an open episode: first=%+v second=%+v err=%v", episode, second, err)
	}
}

func TestClearedFindingRecurrenceOpensANewEpisode(t *testing.T) {
	root, _ := notifyRepo(t, "true")
	now := time.Date(2026, 8, 29, 11, 0, 0, 0, time.UTC)
	digest := evidenceDigest("runner stale")
	unhealthy := HealthVerdict{Aggregate: "unhealthy", FindingDigest: digest}
	first, err := UpdateAlertEpisodes(root, unhealthy, "runner stale", now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := UpdateAlertEpisodes(root, HealthVerdict{Aggregate: "healthy"}, "healthy", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	second, err := UpdateAlertEpisodes(root, unhealthy, "runner stale again", now.Add(2*time.Minute))
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
	root, _ := notifyRepo(t, "true")
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	firstDigest := evidenceDigest("watcher stale")
	secondDigest := evidenceDigest("runner stale")
	if _, err := UpdateAlertEpisodes(root, HealthVerdict{Aggregate: "unhealthy", FindingDigest: firstDigest}, "watcher stale", now); err != nil {
		t.Fatal(err)
	}
	if _, err := UpdateAlertEpisodes(root, HealthVerdict{Aggregate: "unhealthy", FindingDigest: secondDigest}, "runner stale", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	episodes, err := AlertEpisodes(root)
	if err != nil || len(episodes) != 2 || !episodes[0].Resolved || episodes[0].Cleared || episodes[1].Resolved {
		t.Fatalf("new finding did not resolve only its predecessor: episodes=%+v err=%v", episodes, err)
	}
}

func TestAlertEpisodeBoundariesRejectInvalidEvidenceAndInvoker(t *testing.T) {
	root, _ := notifyRepo(t, "true")
	now := time.Now()
	if _, err := UpdateAlertEpisodes(root, HealthVerdict{Aggregate: "unhealthy", FindingDigest: "not-a-digest"}, "message", now); err == nil {
		t.Fatal("invalid finding digest opened an episode")
	}
	if _, err := UpdateAlertEpisodes(root, HealthVerdict{Aggregate: "unhealthy", FindingDigest: evidenceDigest("x")}, " ", now); err == nil {
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
