package steward

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
)

// AlertTransportResult is what the notifier can prove. A zero exit submits a
// message to the transport; it does not prove delivery to a person.
type AlertTransportResult string

const (
	TransportPending   AlertTransportResult = "PENDING"
	TransportSubmitted AlertTransportResult = "TRANSPORT_SUBMITTED"
	TransportFailed    AlertTransportResult = "TRANSPORT_FAILED"
)

// AlertAttempt is one durable attempt to submit an episode to the notifier.
type AlertAttempt struct {
	Sequence    int                  `json:"sequence"`
	AttemptedAt time.Time            `json:"attemptedAt"`
	CompletedAt time.Time            `json:"completedAt,omitempty"`
	Result      AlertTransportResult `json:"result"`
	Problem     string               `json:"problem,omitempty"`
}

// AlertInvoker records what can be observed about the process that invoked an
// acknowledgment. It makes no claim about human authority.
type AlertInvoker struct {
	Pid           int64  `json:"pid"`
	PidStartedAt  int64  `json:"pidStartedAt"`
	PidStartTicks int64  `json:"pidStartTicks,omitempty"`
	BootID        string `json:"bootId,omitempty"`
	UID           int    `json:"uid"`
	ArgvDigest    string `json:"argvDigest,omitempty"`
}

// AlertEpisode is one finding's durable notification and acknowledgment
// lifecycle. Cleared episodes remain evidence and a recurrence opens a new id.
type AlertEpisode struct {
	Schema          int                  `json:"schema"`
	EpisodeID       string               `json:"episodeId"`
	Digest          string               `json:"digest"`
	Owner           string               `json:"owner,omitempty"`
	ScopeID         string               `json:"scopeId,omitempty"`
	Ceiling         string               `json:"ceiling,omitempty"`
	Multiple        int                  `json:"multiple,omitempty"`
	Message         string               `json:"message"`
	OpenedAt        time.Time            `json:"openedAt"`
	Attempts        []AlertAttempt       `json:"attempts"`
	TransportResult AlertTransportResult `json:"transportResult"`
	Acknowledged    bool                 `json:"acknowledged"`
	AcknowledgedAt  time.Time            `json:"acknowledgedAt,omitempty"`
	AcknowledgedBy  *AlertInvoker        `json:"acknowledgedBy,omitempty"`
	Resolved        bool                 `json:"resolved"`
	ResolvedAt      time.Time            `json:"resolvedAt,omitempty"`
	Cleared         bool                 `json:"cleared"`
	ClearedAt       time.Time            `json:"clearedAt,omitempty"`
}

func alertDir(repoRoot string) string {
	return filepath.Join(repoRoot, "artifacts", "agents", "steward", "alerts")
}

func alertLockPath(repoRoot string) string {
	return filepath.Join(repoRoot, "artifacts", "agents", "steward", "alerts.flock")
}

func lockAlerts(repoRoot string, operation int) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(alertLockPath(repoRoot)), 0o755); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(alertLockPath(repoRoot), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(file.Fd()), operation); err != nil {
		file.Close()
		return nil, err
	}
	return file, nil
}

func unlockAlerts(file *os.File) {
	_ = unix.Flock(int(file.Fd()), unix.LOCK_UN)
	_ = file.Close()
}

func alertPath(repoRoot, episodeID string) string {
	return filepath.Join(alertDir(repoRoot), episodeID+".json")
}

func validEpisodeID(value string) bool {
	if value == "" || len(value) > 96 {
		return false
	}
	for _, r := range value {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
			return false
		}
	}
	return true
}

func loadAlertEpisode(path string) (AlertEpisode, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return AlertEpisode{}, err
	}
	var episode AlertEpisode
	if err := json.Unmarshal(data, &episode); err != nil {
		return AlertEpisode{}, fmt.Errorf("alert episode %s is malformed: %w", filepath.Base(path), err)
	}
	if episode.Schema != 1 || !validEpisodeID(episode.EpisodeID) || !validEvidenceDigest(episode.Digest) ||
		episode.Message == "" || episode.OpenedAt.IsZero() || episode.Attempts == nil || episode.TransportResult == "" {
		return AlertEpisode{}, fmt.Errorf("alert episode %s is incomplete", filepath.Base(path))
	}
	if episode.Owner == string(RoleSpendFence) &&
		(episode.ScopeID == "" || (episode.Ceiling != "tokens" && episode.Ceiling != "money") || episode.Multiple < 1) {
		return AlertEpisode{}, fmt.Errorf("alert episode %s has incomplete spend identity", filepath.Base(path))
	}
	return episode, nil
}

func loadAlertEpisodesUnlocked(repoRoot string) ([]AlertEpisode, error) {
	entries, err := os.ReadDir(alertDir(repoRoot))
	if err != nil {
		if os.IsNotExist(err) {
			return []AlertEpisode{}, nil
		}
		return nil, err
	}
	var paths []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			paths = append(paths, filepath.Join(alertDir(repoRoot), entry.Name()))
		}
	}
	sort.Strings(paths)
	episodes := make([]AlertEpisode, 0, len(paths))
	for _, path := range paths {
		episode, err := loadAlertEpisode(path)
		if err != nil {
			return nil, err
		}
		episodes = append(episodes, episode)
	}
	return episodes, nil
}

func saveAlertEpisode(repoRoot string, episode AlertEpisode) error {
	data, err := json.MarshalIndent(episode, "", "  ")
	if err != nil {
		return err
	}
	durable, err := atomicfile.WriteText(alertPath(repoRoot, episode.EpisodeID), string(append(data, '\n')), repoRoot)
	if err != nil {
		return err
	}
	if !durable {
		return fmt.Errorf("alert episode %s was published with durability unknown", episode.EpisodeID)
	}
	return nil
}

// AlertEpisodes returns every retained episode in stable id order.
func AlertEpisodes(repoRoot string) ([]AlertEpisode, error) {
	lock, err := lockAlerts(repoRoot, unix.LOCK_SH)
	if err != nil {
		return nil, err
	}
	defer unlockAlerts(lock)
	return loadAlertEpisodesUnlocked(repoRoot)
}

func nextEpisodeID(digest string, episodes []AlertEpisode) string {
	prefix := digest[:16]
	for sequence := 1; ; sequence++ {
		candidate := fmt.Sprintf("alert-%s-%d", prefix, sequence)
		found := false
		for _, episode := range episodes {
			if episode.EpisodeID == candidate {
				found = true
				break
			}
		}
		if !found {
			return candidate
		}
	}
}

func migrateHeldHealthNotifications(repoRoot string, episodes *[]AlertEpisode, now time.Time) error {
	pending, err := PendingNotifications(repoRoot)
	if err != nil {
		return err
	}
	for _, notification := range pending {
		if notification.DeliveryOwner != legacyHealthDeliveryOwner || !validEvidenceDigest(notification.Nonce) {
			continue
		}
		found := false
		for _, episode := range *episodes {
			if episode.Owner == "" && episode.Digest == notification.Nonce && !episode.Cleared {
				found = true
				break
			}
		}
		if !found {
			episode := AlertEpisode{
				Schema: 1, EpisodeID: nextEpisodeID(notification.Nonce, *episodes), Digest: notification.Nonce,
				Message: notification.Message, OpenedAt: now.UTC(), Attempts: []AlertAttempt{}, TransportResult: TransportPending,
			}
			if err := saveAlertEpisode(repoRoot, episode); err != nil {
				return err
			}
			*episodes = append(*episodes, episode)
		}
		if err := MarkDelivered(repoRoot, notification.Nonce); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

// UpdateAlertEpisodes joins one health verdict to the durable episode store.
// It opens silent history on the first failure and submits only at the alert
// boundary; every healthy verdict clears retained episodes without deleting
// them.
func UpdateAlertEpisodes(repoRoot string, health HealthVerdict, message string, now time.Time) (AlertEpisode, error) {
	lock, err := lockAlerts(repoRoot, unix.LOCK_EX)
	if err != nil {
		return AlertEpisode{}, err
	}
	episodes, err := loadAlertEpisodesUnlocked(repoRoot)
	if err != nil {
		unlockAlerts(lock)
		return AlertEpisode{}, err
	}
	if err := migrateHeldHealthNotifications(repoRoot, &episodes, now); err != nil {
		unlockAlerts(lock)
		return AlertEpisode{}, err
	}

	if health.Aggregate == "healthy" {
		for index := range episodes {
			if episodes[index].Owner != "" {
				continue
			}
			changed := false
			if !episodes[index].Resolved {
				episodes[index].Resolved = true
				episodes[index].ResolvedAt = now.UTC()
				changed = true
			}
			if !episodes[index].Cleared {
				episodes[index].Cleared = true
				episodes[index].ClearedAt = now.UTC()
				changed = true
			}
			if changed {
				if err := saveAlertEpisode(repoRoot, episodes[index]); err != nil {
					unlockAlerts(lock)
					return AlertEpisode{}, err
				}
			}
		}
		unlockAlerts(lock)
		return AlertEpisode{}, nil
	}

	for index := range episodes {
		if episodes[index].Owner != "" {
			continue
		}
		if !episodes[index].Cleared && episodes[index].Digest != health.FindingDigest && !episodes[index].Resolved {
			episodes[index].Resolved = true
			episodes[index].ResolvedAt = now.UTC()
			if err := saveAlertEpisode(repoRoot, episodes[index]); err != nil {
				unlockAlerts(lock)
				return AlertEpisode{}, err
			}
		}
	}
	if !validEvidenceDigest(health.FindingDigest) || strings.TrimSpace(message) == "" {
		unlockAlerts(lock)
		return AlertEpisode{}, fmt.Errorf("a nonhealthy health verdict needs a finding digest and message")
	}

	var episode AlertEpisode
	for _, candidate := range episodes {
		if candidate.Owner == "" && candidate.Digest == health.FindingDigest && !candidate.Cleared {
			episode = candidate
			break
		}
	}
	if episode.EpisodeID == "" {
		episode = AlertEpisode{
			Schema: 1, EpisodeID: nextEpisodeID(health.FindingDigest, episodes), Digest: health.FindingDigest,
			Message: message, OpenedAt: now.UTC(), Attempts: []AlertAttempt{}, TransportResult: TransportPending,
		}
		episodes = append(episodes, episode)
		if err := saveAlertEpisode(repoRoot, episode); err != nil {
			unlockAlerts(lock)
			return AlertEpisode{}, err
		}
	}
	if episode.Resolved {
		episode.Resolved = false
		episode.ResolvedAt = time.Time{}
	}
	if !health.ShouldAlert {
		if err := saveAlertEpisode(repoRoot, episode); err != nil {
			unlockAlerts(lock)
			return AlertEpisode{}, err
		}
		unlockAlerts(lock)
		return episode, nil
	}
	if episode.TransportResult == TransportSubmitted {
		if err := saveAlertEpisode(repoRoot, episode); err != nil {
			unlockAlerts(lock)
			return AlertEpisode{}, err
		}
		unlockAlerts(lock)
		return episode, nil
	}
	if err := submitEpisode(repoRoot, &episode, now); err != nil {
		unlockAlerts(lock)
		return AlertEpisode{}, err
	}
	unlockAlerts(lock)
	return episode, nil
}

func submitEpisode(repoRoot string, episode *AlertEpisode, now time.Time) error {
	var attempt AlertAttempt
	if len(episode.Attempts) > 0 && episode.Attempts[len(episode.Attempts)-1].Result == TransportPending {
		// A pending journal entry means a process may have stopped after the
		// notifier side effect. Recovery reuses that exact attempt: at-least-once
		// delivery may repeat across the crash gap, but it never invents a second
		// submission for the episode.
		attempt = episode.Attempts[len(episode.Attempts)-1]
	} else {
		attempt = AlertAttempt{Sequence: len(episode.Attempts) + 1, AttemptedAt: now.UTC(), Result: TransportPending}
		episode.Attempts = append(episode.Attempts, attempt)
		episode.TransportResult = TransportPending
		if err := saveAlertEpisode(repoRoot, *episode); err != nil {
			return err
		}
	}

	transportErr := Deliver(repoRoot, episode.Message)
	completedAt := time.Now().UTC()
	if len(episode.Attempts) < attempt.Sequence || episode.Attempts[attempt.Sequence-1].Result != TransportPending {
		return fmt.Errorf("alert episode %s transport attempt changed before completion", episode.EpisodeID)
	}
	episode.Attempts[attempt.Sequence-1].CompletedAt = completedAt
	if transportErr == nil {
		episode.Attempts[attempt.Sequence-1].Result = TransportSubmitted
		episode.TransportResult = TransportSubmitted
	} else {
		episode.Attempts[attempt.Sequence-1].Result = TransportFailed
		episode.Attempts[attempt.Sequence-1].Problem = transportErr.Error()
		episode.TransportResult = TransportFailed
	}
	if err := saveAlertEpisode(repoRoot, *episode); err != nil {
		return err
	}
	return nil
}

// UpdateSpendEpisodes joins one valid spend observation to the durable alert
// store. Unknown observations are no-ops because absence was not proven.
func UpdateSpendEpisodes(repoRoot string, observation SpendObservation, now time.Time) error {
	if !observation.Valid {
		return nil
	}
	lock, err := lockAlerts(repoRoot, unix.LOCK_EX)
	if err != nil {
		return err
	}
	defer unlockAlerts(lock)
	episodes, err := loadAlertEpisodesUnlocked(repoRoot)
	if err != nil {
		return err
	}
	current := map[string]int{}
	for _, crossing := range observation.Crossings {
		key := crossing.ScopeID + "\x00" + crossing.Ceiling
		if crossing.Multiple > current[key] {
			current[key] = crossing.Multiple
		}
	}
	for index := range episodes {
		episode := &episodes[index]
		if episode.Owner != string(RoleSpendFence) || episode.Cleared {
			continue
		}
		if current[episode.ScopeID+"\x00"+episode.Ceiling] >= episode.Multiple {
			continue
		}
		episode.Resolved = true
		episode.ResolvedAt = now.UTC()
		episode.Cleared = true
		episode.ClearedAt = now.UTC()
		if err := saveAlertEpisode(repoRoot, *episode); err != nil {
			return err
		}
	}
	for _, crossing := range observation.Crossings {
		digest := spendCrossingDigest(crossing)
		var episode *AlertEpisode
		for index := range episodes {
			candidate := &episodes[index]
			if candidate.Owner == string(RoleSpendFence) && candidate.Digest == digest &&
				candidate.ScopeID == crossing.ScopeID && candidate.Ceiling == crossing.Ceiling &&
				candidate.Multiple == crossing.Multiple && !candidate.Cleared {
				episode = candidate
				break
			}
		}
		if episode == nil {
			created := AlertEpisode{
				Schema: 1, EpisodeID: nextEpisodeID(digest, episodes), Digest: digest,
				Owner: string(RoleSpendFence), ScopeID: crossing.ScopeID, Ceiling: crossing.Ceiling, Multiple: crossing.Multiple,
				Message: spendCrossingMessage(crossing), OpenedAt: now.UTC(), Attempts: []AlertAttempt{}, TransportResult: TransportPending,
			}
			episodes = append(episodes, created)
			episode = &episodes[len(episodes)-1]
			if err := saveAlertEpisode(repoRoot, *episode); err != nil {
				return err
			}
		}
		if episode.TransportResult == TransportSubmitted {
			continue
		}
		if err := submitEpisode(repoRoot, episode, now); err != nil {
			return err
		}
	}
	return nil
}

func spendCrossingDigest(crossing SpendCrossing) string {
	key := fmt.Sprintf("spend-fence\n%s.%sx%d", crossing.ScopeID, crossing.Ceiling, crossing.Multiple)
	digest := sha256.Sum256([]byte(key))
	return hex.EncodeToString(digest[:])
}

func spendCrossingMessage(crossing SpendCrossing) string {
	spendValue := fmt.Sprintf("%.0f", crossing.Spend)
	limitValue := fmt.Sprintf("%.0f", crossing.Limit)
	if crossing.Ceiling == "money" {
		spendValue = fmt.Sprintf("%.2f", crossing.Spend)
		limitValue = fmt.Sprintf("%.2f", crossing.Limit)
	}
	ledger := filepath.ToSlash(filepath.Join("artifacts", "agents", "steward", "spend", crossing.Day+".json"))
	return fmt.Sprintf("SPEND CROSSED %s.%sx%d machine=%s spend=%s ceiling=%s ledger=%s raise: spend.ceiling.%s.%s in metasystem.conf on Wido's recorded word (R-60-m1); alert mode refuses nothing",
		crossing.ScopeID, crossing.Ceiling, crossing.Multiple, crossing.Machine, spendValue, limitValue, ledger, crossing.Scope, crossing.Ceiling)
}

// AcknowledgeAlert records acknowledgment without clearing an episode or
// changing any goal, refusal, or authority state.
func AcknowledgeAlert(repoRoot, episodeID string, invoker AlertInvoker, now time.Time) (AlertEpisode, error) {
	if !validEpisodeID(episodeID) {
		return AlertEpisode{}, fmt.Errorf("alert episode id is invalid")
	}
	if invoker.Pid < 1 || invoker.PidStartedAt < 1 {
		return AlertEpisode{}, fmt.Errorf("alert acknowledgment needs an observed invoker identity")
	}
	lock, err := lockAlerts(repoRoot, unix.LOCK_EX)
	if err != nil {
		return AlertEpisode{}, err
	}
	defer unlockAlerts(lock)
	episode, err := loadAlertEpisode(alertPath(repoRoot, episodeID))
	if err != nil {
		return AlertEpisode{}, err
	}
	if episode.Acknowledged {
		return episode, nil
	}
	episode.Acknowledged = true
	episode.AcknowledgedAt = now.UTC()
	episode.AcknowledgedBy = &invoker
	if err := saveAlertEpisode(repoRoot, episode); err != nil {
		return AlertEpisode{}, err
	}
	return episode, nil
}
