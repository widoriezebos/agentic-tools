package goal

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// SessionStopProcessRef is the process birth identity of the attended human
// who minted a stop. A marker stops being valid as soon as that process is no
// longer alive at the recorded identity.
type SessionStopProcessRef struct {
	Pid           int64  `json:"pid"`
	PidStartedAt  int64  `json:"pidStartedAt"`
	PidStartTicks int64  `json:"pidStartTicks,omitempty"`
	BootID        string `json:"bootId,omitempty"`
}

// SessionStop is one holder-bound, expiring authorization. AuthorizationId
// is retained in the consumed registry before a quiet stop is returned or its
// session ends, so restoring the marker bytes cannot replay the authorization.
type SessionStop struct {
	SchemaVersion    int                   `json:"schemaVersion"`
	AuthorizationId  string                `json:"authorizationId"`
	SessionId        string                `json:"sessionId"`
	HolderMainId     string                `json:"holderMainId"`
	ClaimEpoch       int64                 `json:"claimEpoch"`
	By               string                `json:"by"`
	WrittenAt        string                `json:"writtenAt"`
	ExpiresAt        string                `json:"expiresAt"`
	Human            SessionStopProcessRef `json:"human"`
	HumanAuthority   string                `json:"humanAuthorityProof"`
	SessionLifecycle string                `json:"sessionLifecycle"`
}

type consumedSessionStop struct {
	ConsumedAt   string `json:"consumedAt"`
	SessionId    string `json:"sessionId"`
	HolderMainId string `json:"holderMainId"`
	ClaimEpoch   int64  `json:"claimEpoch"`
}

type sessionStopRegistry struct {
	SchemaVersion int                            `json:"schemaVersion"`
	Consumed      map[string]consumedSessionStop `json:"consumed"`
}

type sessionStopLease struct {
	HolderMainId  string `json:"holderMainId"`
	Pid           int64  `json:"pid"`
	PidStartedAt  int64  `json:"pidStartedAt"`
	PidStartTicks int64  `json:"pidStartTicks,omitempty"`
	BootID        string `json:"bootId,omitempty"`
	ClaimEpoch    int64  `json:"claimEpoch"`
}

type sessionStopAnnouncement struct {
	SessionId     string          `json:"sessionId"`
	MainId        string          `json:"mainId"`
	Pid           int64           `json:"pid"`
	PidStartedAt  int64           `json:"pidStartedAt"`
	PidStartTicks int64           `json:"pidStartTicks,omitempty"`
	BootID        string          `json:"bootId,omitempty"`
	Runtime       string          `json:"runtime"`
	InstanceTag   string          `json:"instanceTag"`
	CommandHash   string          `json:"commandHash"`
	AnnouncedAt   string          `json:"announcedAt"`
	Pgid          int64           `json:"pgid"`
	OwnerLineage  string          `json:"ownerLineage,omitempty"`
	Provenance    json.RawMessage `json:"identityProvenance,omitempty"`
}

var sessionStopID = regexp.MustCompile(`^[0-9a-f]{32}$`)
var sessionStopDigest = regexp.MustCompile(`^[0-9a-f]{64}$`)
var sessionStopAnnouncementName = regexp.MustCompile(`-[1-9][0-9]*\.json$`)

func sessionStopPath(root, sessionId string) string {
	return filepath.Join(root, "artifacts", "agents", "session-stops", NormalizeSession(sessionId)+".json")
}

func sessionStopRegistryPath(root string) string {
	return filepath.Join(root, "artifacts", "agents", "session-stops", "consumed.json")
}

func sessionStopLeasePath(root string) string {
	return filepath.Join(root, "artifacts", "agents", "mains", "worktree-lease.json")
}

func validateSessionStopBy(by string) (string, error) {
	by = strings.TrimSpace(by)
	if by == "" {
		return "", fmt.Errorf("session stop requires a non-blank human name")
	}
	if len(by) > 200 || strings.IndexFunc(by, unicode.IsControl) >= 0 {
		return "", fmt.Errorf("session stop human name must be at most 200 bytes and contain no control characters")
	}
	return by, nil
}

func processRef(ref SessionStopProcessRef) identity.Ref {
	return identity.Ref{
		Pid: ref.Pid, StartedAtSec: ref.PidStartedAt,
		StartTicks: ref.PidStartTicks, BootID: ref.BootID,
	}
}

func validateSessionStop(marker *SessionStop) error {
	if marker.SchemaVersion != 3 || !sessionStopID.MatchString(marker.AuthorizationId) {
		return fmt.Errorf("session stop marker has an invalid schema or authorization id")
	}
	if marker.SessionId == "" || marker.SessionId != NormalizeSession(marker.SessionId) ||
		!safeSession.MatchString(marker.HolderMainId) || marker.ClaimEpoch < 1 {
		return fmt.Errorf("session stop marker has invalid holder coordinates")
	}
	by, err := validateSessionStopBy(marker.By)
	if err != nil {
		return err
	}
	marker.By = by
	written, err := parseISO(marker.WrittenAt)
	if err != nil {
		return fmt.Errorf("session stop marker has an invalid written time")
	}
	expires, err := parseISO(marker.ExpiresAt)
	if err != nil || !expires.After(written) {
		return fmt.Errorf("session stop marker has an invalid expiry")
	}
	if marker.Human.Pid < 1 || marker.Human.PidStartedAt < 1 || processRef(marker.Human).Mode() == identity.CompareInvalid {
		return fmt.Errorf("session stop marker has an invalid attended-human process identity")
	}
	if !sessionStopDigest.MatchString(marker.HumanAuthority) || !sessionStopDigest.MatchString(marker.SessionLifecycle) {
		return fmt.Errorf("session stop marker lacks its human-classification proof or session lifecycle binding")
	}
	return nil
}

func newSessionStopID() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

func sessionStopHumanRef(ref humanauthority.ProcessRef) SessionStopProcessRef {
	return SessionStopProcessRef{
		Pid: ref.PID, PidStartedAt: ref.PIDStartedAt,
		PidStartTicks: ref.StartTicks, BootID: ref.BootID,
	}
}

func sessionStopHumanAuthorityToken(proof humanauthority.Proof) (string, error) {
	data, err := json.Marshal(proof)
	if err != nil {
		return "", err
	}
	return sha256Hex(data), nil
}

func sessionStopLifecycleToken(announcement sessionStopAnnouncement) (string, error) {
	data, err := json.Marshal(announcement)
	if err != nil {
		return "", err
	}
	return sha256Hex(data), nil
}

// currentSessionLifecycle detects a replaced or retired announcement. It does
// not authenticate a logical session end when the same announcement bytes are
// deliberately retained and replayed with a marker; durable goal-ledger
// authentication owns that deliberate-forgery boundary.
func (s *Store) currentSessionLifecycle(sessionId, mainId string, lease sessionStopLease) (string, error) {
	paths, err := filepath.Glob(filepath.Join(s.Root, "artifacts", "agents", "mains", "*.json"))
	if err != nil {
		return "", err
	}
	var matched *sessionStopAnnouncement
	for _, path := range paths {
		if !sessionStopAnnouncementName.MatchString(filepath.Base(path)) {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("session announcement %s is unreadable: %w", filepath.Base(path), err)
		}
		var announcement sessionStopAnnouncement
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&announcement); err != nil {
			return "", fmt.Errorf("session announcement %s is malformed: %w", filepath.Base(path), err)
		}
		if err := decoder.Decode(&struct{}{}); err != io.EOF {
			return "", fmt.Errorf("session announcement %s has trailing content", filepath.Base(path))
		}
		if announcement.SessionId != sessionId || announcement.MainId != mainId {
			continue
		}
		if matched != nil {
			return "", fmt.Errorf("session lifecycle is ambiguous: more than one announcement matches holder %s session %s", mainId, sessionId)
		}
		copy := announcement
		matched = &copy
	}
	if matched == nil {
		return "", fmt.Errorf("session lifecycle is absent for holder %s session %s", mainId, sessionId)
	}
	if matched.Pid != lease.Pid || matched.PidStartedAt != lease.PidStartedAt ||
		matched.PidStartTicks != lease.PidStartTicks || matched.BootID != lease.BootID ||
		matched.Runtime == "" || matched.InstanceTag == "" || matched.CommandHash == "" || matched.AnnouncedAt == "" {
		return "", fmt.Errorf("session lifecycle does not match the current holder process")
	}
	return sessionStopLifecycleToken(*matched)
}

// WriteSessionStop validates an enrolled-terminal ancestry proof before it
// persists an authorization. This in-process boundary does not authenticate
// the caller that obtained the proof: code deliberately supplying another
// process's valid proof is the same trust class as code forging marker bytes,
// and belongs to durable goal-ledger authentication rather than this gate.
func (s *Store) WriteSessionStop(marker SessionStop, proof humanauthority.Proof) (SessionStop, error) {
	resolvedRoot, err := ResolveStateRoot(s.Root)
	if err != nil {
		return SessionStop{}, err
	}
	s.Root = resolvedRoot
	if strings.TrimSpace(marker.SessionId) == "" {
		return SessionStop{}, fmt.Errorf("session stop requires an announced session id")
	}
	marker.SessionId = NormalizeSession(marker.SessionId)
	if !proof.ValidFor(s.Root) {
		return SessionStop{}, fmt.Errorf("session stop requires a fresh enrolled-terminal human-classification proof")
	}
	if _, err := parseISO(marker.WrittenAt); err != nil || proof.CheckedAt.UTC().Format("2006-01-02T15:04:05Z07:00") != marker.WrittenAt {
		return SessionStop{}, fmt.Errorf("session stop human-classification proof does not match the authorization time")
	}
	marker.Human = sessionStopHumanRef(proof.InvokerRef)
	marker.HumanAuthority, err = sessionStopHumanAuthorityToken(proof)
	if err != nil {
		return SessionStop{}, err
	}
	if marker.AuthorizationId == "" {
		id, err := newSessionStopID()
		if err != nil {
			return SessionStop{}, err
		}
		marker.AuthorizationId = id
	}
	_, err = s.withLock(func() (Result, error) {
		lease, err := s.currentSessionStopLease()
		if err != nil {
			return Result{}, fmt.Errorf("session stop holder lease cannot be proved: %w", err)
		}
		if lease.HolderMainId != marker.HolderMainId || lease.ClaimEpoch != marker.ClaimEpoch {
			return Result{}, fmt.Errorf("session stop coordinates do not match the current holder lease")
		}
		marker.SessionLifecycle, err = s.currentSessionLifecycle(marker.SessionId, marker.HolderMainId, lease)
		if err != nil {
			return Result{}, err
		}
		if err := validateSessionStop(&marker); err != nil {
			return Result{}, err
		}
		data, err := json.MarshalIndent(marker, "", "  ")
		if err != nil {
			return Result{}, err
		}
		durable, err := atomicfile.WriteText(sessionStopPath(s.Root, marker.SessionId), string(append(data, '\n')), s.Root)
		if err != nil {
			return Result{}, err
		}
		if !durable {
			return Result{}, fmt.Errorf("session stop marker was published with durability unknown")
		}
		return Result{}, nil
	})
	if err != nil {
		return SessionStop{}, err
	}
	return marker, nil
}

func decodeSessionStop(data []byte) (SessionStop, error) {
	var marker SessionStop
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&marker); err != nil {
		return SessionStop{}, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return SessionStop{}, fmt.Errorf("trailing content")
	}
	if err := validateSessionStop(&marker); err != nil {
		return SessionStop{}, err
	}
	return marker, nil
}

func (s *Store) readSessionStopRegistry() (sessionStopRegistry, error) {
	data, err := os.ReadFile(sessionStopRegistryPath(s.Root))
	if os.IsNotExist(err) {
		return sessionStopRegistry{SchemaVersion: 1, Consumed: map[string]consumedSessionStop{}}, nil
	}
	if err != nil {
		return sessionStopRegistry{}, err
	}
	var registry sessionStopRegistry
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&registry); err != nil {
		return sessionStopRegistry{}, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF || registry.SchemaVersion != 1 || registry.Consumed == nil {
		return sessionStopRegistry{}, fmt.Errorf("consumed registry has an invalid schema")
	}
	return registry, nil
}

func (s *Store) saveSessionStopRegistry(registry sessionStopRegistry) error {
	data, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		return err
	}
	durable, err := atomicfile.WriteText(sessionStopRegistryPath(s.Root), string(append(data, '\n')), s.Root)
	if err != nil {
		return err
	}
	if !durable {
		return fmt.Errorf("session stop consumed registry durability is unknown")
	}
	return nil
}

func removeSessionStop(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("session stop marker could not be removed: %w", err)
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return fmt.Errorf("session stop marker removal durability is unknown: %w", err)
	}
	defer dir.Close()
	if err := dir.Sync(); err != nil {
		return fmt.Errorf("session stop marker removal durability is unknown: %w", err)
	}
	return nil
}

// EndSessionStop spends and removes an unused authorization before the
// session announcement is retired. Keeping the durable spent record separate
// from announcement retirement makes a failed retirement unable to preserve
// stop authority for a resumed lifecycle.
func (s *Store) EndSessionStop(sessionId string) error {
	resolvedRoot, err := ResolveStateRoot(s.Root)
	if err != nil {
		return err
	}
	s.Root = resolvedRoot
	sessionId = NormalizeSession(sessionId)
	_, err = s.withLock(func() (Result, error) {
		path := sessionStopPath(s.Root, sessionId)
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			return Result{}, nil
		}
		if err != nil {
			return Result{}, fmt.Errorf("session stop marker unreadable: %w", err)
		}
		marker, err := decodeSessionStop(data)
		if err != nil {
			return Result{}, fmt.Errorf("session stop marker malformed: %w", err)
		}
		if marker.SessionId != sessionId {
			return Result{}, fmt.Errorf("session stop marker does not match the ending session")
		}
		registry, err := s.readSessionStopRegistry()
		if err != nil {
			return Result{}, fmt.Errorf("session stop consumed registry unreadable: %w", err)
		}
		if _, spent := registry.Consumed[marker.AuthorizationId]; !spent {
			registry.Consumed[marker.AuthorizationId] = consumedSessionStop{
				ConsumedAt: s.nowISO(), SessionId: marker.SessionId,
				HolderMainId: marker.HolderMainId, ClaimEpoch: marker.ClaimEpoch,
			}
			if err := s.saveSessionStopRegistry(registry); err != nil {
				return Result{}, err
			}
		}
		if err := removeSessionStop(path); err != nil {
			return Result{}, err
		}
		return Result{}, nil
	})
	return err
}

func (s *Store) currentSessionStopLease() (sessionStopLease, error) {
	data, err := os.ReadFile(sessionStopLeasePath(s.Root))
	if err != nil {
		return sessionStopLease{}, err
	}
	var lease sessionStopLease
	if err := json.Unmarshal(data, &lease); err != nil || lease.HolderMainId == "" || lease.ClaimEpoch < 1 ||
		lease.Pid < 1 || lease.PidStartedAt < 1 {
		return sessionStopLease{}, fmt.Errorf("checkout lease is malformed")
	}
	return lease, nil
}

// inspectSessionStop checks every freshness coordinate using repository-local
// state only. The detail explains an invalid or replayed marker without
// converting uncertainty into permission.
func (s *Store) inspectSessionStop(sessionId, mainId string) (SessionStop, bool, string, error) {
	sessionId = NormalizeSession(sessionId)
	path := sessionStopPath(s.Root, sessionId)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return SessionStop{}, false, "", nil
	}
	if err != nil {
		return SessionStop{}, false, "", fmt.Errorf("session stop marker unreadable: %w", err)
	}
	marker, err := decodeSessionStop(data)
	if err != nil {
		return SessionStop{}, false, "", fmt.Errorf("session stop marker malformed: %w", err)
	}
	registry, err := s.readSessionStopRegistry()
	if err != nil {
		return SessionStop{}, false, "", fmt.Errorf("session stop consumed registry unreadable: %w", err)
	}
	if _, spent := registry.Consumed[marker.AuthorizationId]; spent {
		return marker, false, "SESSION STOP authorization was already consumed; copied marker bytes cannot replay it", nil
	}
	if marker.SessionId != sessionId || mainId == "" || marker.HolderMainId != mainId {
		return marker, false, "SESSION STOP authorization does not match this holder session", nil
	}
	lease, err := s.currentSessionStopLease()
	if err != nil {
		return SessionStop{}, false, "", fmt.Errorf("session stop holder lease cannot be proved: %w", err)
	}
	if lease.HolderMainId != marker.HolderMainId || lease.ClaimEpoch != marker.ClaimEpoch {
		return marker, false, "SESSION STOP authorization belongs to an earlier holder or lease epoch", nil
	}
	leaseRef := identity.Ref{Pid: lease.Pid, StartedAtSec: lease.PidStartedAt, StartTicks: lease.PidStartTicks, BootID: lease.BootID}
	if identity.AliveRef(s.prober(), leaseRef) != identity.Alive {
		return marker, false, "SESSION STOP holder process is not provably live", nil
	}
	expires, _ := parseISO(marker.ExpiresAt)
	if !s.now().Before(expires) {
		return marker, false, "SESSION STOP authorization expired before this stop", nil
	}
	if identity.AliveRef(s.prober(), processRef(marker.Human)) != identity.Alive {
		return marker, false, "SESSION STOP attending human is no longer present", nil
	}
	lifecycle, err := s.currentSessionLifecycle(marker.SessionId, marker.HolderMainId, lease)
	if err != nil {
		return marker, false, "SESSION STOP session lifecycle is no longer active: " + err.Error(), nil
	}
	if lifecycle != marker.SessionLifecycle {
		return marker, false, "SESSION STOP authorization belongs to an earlier session lifecycle", nil
	}
	return marker, true, "", nil
}

// consumeSessionStop validates the authorization again and durably records its
// single use. It is called while TurnVerdict holds the goal lock. Once the
// registry commit succeeds, marker-file cleanup cannot revoke the authorized
// stop because restored marker bytes remain blocked by the registry.
func (s *Store) consumeSessionStop(sessionId, mainId string) (SessionStop, bool, string, error) {
	marker, authorized, detail, err := s.inspectSessionStop(sessionId, mainId)
	if err != nil || !authorized {
		return marker, authorized, detail, err
	}
	registry, err := s.readSessionStopRegistry()
	if err != nil {
		return SessionStop{}, false, "", fmt.Errorf("session stop consumed registry unreadable: %w", err)
	}
	if _, spent := registry.Consumed[marker.AuthorizationId]; spent {
		return marker, false, "SESSION STOP authorization was already consumed; copied marker bytes cannot replay it", nil
	}
	registry.Consumed[marker.AuthorizationId] = consumedSessionStop{
		ConsumedAt: s.nowISO(), SessionId: marker.SessionId,
		HolderMainId: marker.HolderMainId, ClaimEpoch: marker.ClaimEpoch,
	}
	if err := s.saveSessionStopRegistry(registry); err != nil {
		return SessionStop{}, false, "", err
	}
	if err := removeSessionStop(sessionStopPath(s.Root, marker.SessionId)); err != nil {
		return marker, true, "SESSION STOP authorization was consumed, but its marker file could not be removed; copied bytes remain blocked: " + err.Error(), nil
	}
	return marker, true, "", nil
}
