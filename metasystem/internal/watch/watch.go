// Package watch owns the zero-write projection of persisted metasystem
// verdicts. It reads producer records as they stand; it never refreshes a
// producer, probes a process, takes a creating lock, or writes reader state.
package watch

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

// AggregateVerdict is the read surface's total result. Attention outranks
// unknown because a known persisted failure must not be hidden by a second
// unreadable store.
type AggregateVerdict string

const (
	AggregateHealthy   AggregateVerdict = "HEALTHY"
	AggregateAttention AggregateVerdict = "ATTENTION"
	AggregateUnknown   AggregateVerdict = "UNKNOWN"
)

// TrackedClass is the closed set of persisted producer classes in the watch
// snapshot. Keeping empty classes in the output makes absence explicit.
type TrackedClass string

const (
	ClassJobs            TrackedClass = "jobs"
	ClassCompletedRounds TrackedClass = "completed-rounds"
	ClassCensus          TrackedClass = "census"
	ClassHealth          TrackedClass = "health"
	ClassDelivery        TrackedClass = "delivery"
	ClassAlerts          TrackedClass = "alerts"
	ClassIntents         TrackedClass = "intents"
	ClassBreachRoutes    TrackedClass = "breach-routes"
)

var trackedClassOrder = []TrackedClass{
	ClassJobs,
	ClassCompletedRounds,
	ClassCensus,
	ClassHealth,
	ClassDelivery,
	ClassAlerts,
	ClassIntents,
	ClassBreachRoutes,
}

// SectionVerdict describes only the producer store, not its domain-specific
// item verdicts. Ordinary absent stores are empty; health is the fail-safe
// exception and degrades to a typed dead freshness item.
type SectionVerdict string

const (
	SectionEmpty    SectionVerdict = "EMPTY"
	SectionReadable SectionVerdict = "READABLE"
	SectionDegraded SectionVerdict = "DEGRADED"
)

// ProducerVerdict preserves the vocabulary written by the source producer.
// The read surface does not translate job, census, health, alert, intent, or
// breach states into a new shared domain vocabulary.
type ProducerVerdict string

const (
	VerdictUnreadable         ProducerVerdict = "UNREADABLE"
	VerdictUnknownConsumption ProducerVerdict = "UNKNOWN-CONSUMPTION"
)

// GoalFieldState preserves the distinction the job wire format makes between
// an explicit no-goal null and corrupt or legacy shapes.
type GoalFieldState string

const (
	GoalFieldBound   GoalFieldState = "BOUND"
	GoalFieldNull    GoalFieldState = "NULL"
	GoalFieldEmpty   GoalFieldState = "EMPTY_STRING"
	GoalFieldAbsent  GoalFieldState = "ABSENT"
	GoalFieldInvalid GoalFieldState = "INVALID"
)

// Item is one persisted producer verdict. Optional fields carry typed join
// coordinates without copying whole source documents into the public output.
type Item struct {
	Kind        string          `json:"kind"`
	ID          string          `json:"id"`
	Verdict     ProducerVerdict `json:"verdict"`
	Evidence    string          `json:"evidence"`
	Role        string          `json:"role,omitempty"`
	Stage       string          `json:"stage,omitempty"`
	GoalID      string          `json:"goalId,omitempty"`
	GoalField   GoalFieldState  `json:"goalField,omitempty"`
	Problem     string          `json:"problem,omitempty"`
	ObservedAt  string          `json:"observedAt,omitempty"`
	PendingJobs []string        `json:"pendingJobs,omitempty"`
}

// Section is one member of the closed class enumeration. Items is always a
// JSON array, including for an absent producer store.
type Section struct {
	Class   TrackedClass   `json:"class"`
	Store   string         `json:"store"`
	Verdict SectionVerdict `json:"verdict"`
	Items   []Item         `json:"items"`
}

// Snapshot is the typed output of one read. Empty means no producer had a
// persisted item; it is independent of whether producer directories exist.
type Snapshot struct {
	SchemaVersion int              `json:"schemaVersion"`
	Aggregate     AggregateVerdict `json:"aggregate"`
	Empty         bool             `json:"empty"`
	Sections      []Section        `json:"sections"`
}

type healthRecordLens struct {
	Verdict steward.HealthVerdict `json:"verdict"`
}

// Read joins the persisted producer records under root without changing any
// producer or reader-owned state.
func Read(root string) Snapshot {
	return readAt(root, time.Now().UTC())
}

func readAt(root string, now time.Time) Snapshot {
	if root == "" {
		root = "."
	}
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		sections := emptySections()
		problem := "checkout root is not a directory"
		if err != nil {
			problem = err.Error()
		}
		sections[0] = degrade(sections[0], root, problem)
		return summarize(sections)
	}
	health, healthState, healthProblem := readHealth(root, now)
	sections := []Section{
		readJobs(root),
		readCompletedRounds(root),
		readCensus(root),
		healthSection(health, healthState, healthProblem),
		readDelivery(root, health, healthState, healthProblem),
		readAlerts(root),
		readIntents(root),
		readBreachRoutes(root),
	}
	return summarize(sections)
}

func summarize(sections []Section) Snapshot {
	snapshot := Snapshot{SchemaVersion: 1, Aggregate: AggregateHealthy, Empty: true, Sections: sections}
	unknown := false
	attention := false
	for _, section := range sections {
		if len(section.Items) > 0 {
			snapshot.Empty = false
		}
		if section.Verdict == SectionDegraded {
			unknown = true
		}
		for _, item := range section.Items {
			if item.Verdict == VerdictUnreadable {
				unknown = true
			}
			if itemNeedsUnknown(section.Class, item) {
				unknown = true
			}
			if itemNeedsAttention(section.Class, item) {
				attention = true
			}
		}
	}
	if attention {
		snapshot.Aggregate = AggregateAttention
	} else if unknown {
		snapshot.Aggregate = AggregateUnknown
	}
	return snapshot
}

func emptySections() []Section {
	return []Section{
		newSection(ClassJobs, "artifacts/agents/jobs"),
		newSection(ClassCompletedRounds, "artifacts/agents/jobs + memory/receipts.log"),
		newSection(ClassCensus, "artifacts/agents/supervision/last-census.json"),
		newSection(ClassHealth, "artifacts/agents/steward/health.json"),
		newSection(ClassDelivery, "artifacts/agents/steward/health.json + artifacts/agents/steward/pending"),
		newSection(ClassAlerts, "artifacts/agents/steward/alerts"),
		newSection(ClassIntents, "artifacts/agents/steward/{intents,consumed,cancelled}"),
		newSection(ClassBreachRoutes, "artifacts/agents/goal-stops"),
	}
}

func itemNeedsUnknown(class TrackedClass, item Item) bool {
	switch class {
	case ClassJobs:
		return item.GoalField == GoalFieldAbsent || item.GoalField == GoalFieldEmpty || item.GoalField == GoalFieldInvalid
	case ClassCompletedRounds:
		return item.Verdict == VerdictUnknownConsumption
	case ClassHealth:
		return item.Verdict == "unknown"
	case ClassDelivery:
		return item.Verdict == "unknown"
	default:
		return false
	}
}

// ExitCode mirrors the health surface: known attention is one; unknown is two.
func (s Snapshot) ExitCode() int {
	switch s.Aggregate {
	case AggregateHealthy:
		return 0
	case AggregateAttention:
		return 1
	default:
		return 2
	}
}

func itemNeedsAttention(class TrackedClass, item Item) bool {
	switch class {
	case ClassJobs:
		// Goal-bound terminal outcomes need the delivery producer's recovery
		// join. Explicit no-goal failures have no such owner and remain visible.
		return item.GoalField == GoalFieldNull &&
			(item.Verdict == "failed" || item.Verdict == "timeout")
	case ClassCensus:
		return item.Verdict != "SUCCESS" && item.Verdict != VerdictUnreadable
	case ClassHealth:
		return item.Kind == "health-freshness" && item.Verdict == "dead" ||
			item.Kind == "health-aggregate" && item.Verdict == "unhealthy"
	case ClassDelivery:
		return item.Verdict == "dead" || item.Verdict == "PENDING"
	case ClassAlerts:
		return item.Stage == "ACTIVE" && item.Verdict != "TRANSPORT_SUBMITTED"
	case ClassBreachRoutes:
		return item.Verdict == "OPEN" || item.Verdict == "INDETERMINATE"
	default:
		return false
	}
}

func readJobs(root string) Section {
	store := "artifacts/agents/jobs"
	section := newSection(ClassJobs, store)
	paths, problem := jsonFiles(root, store)
	if problem != "" {
		return degrade(section, store, problem)
	}
	for _, path := range paths {
		rel := relativeEvidence(root, path)
		data, err := os.ReadFile(path)
		if err != nil {
			section.Items = append(section.Items, unreadableItem("job", fileID(path), rel, err))
			section.Verdict = SectionDegraded
			continue
		}
		var record map[string]json.RawMessage
		if err := json.Unmarshal(data, &record); err != nil || record == nil {
			if err == nil {
				err = fmt.Errorf("record is not a JSON object")
			}
			section.Items = append(section.Items, unreadableItem("job", fileID(path), rel, err))
			section.Verdict = SectionDegraded
			continue
		}
		id, idOK := rawString(record["jobId"])
		status, statusOK := rawString(record["status"])
		if !idOK || id == "" || id != fileID(path) || !knownJobStatus(status) || !statusOK {
			section.Items = append(section.Items, unreadableItem("job", fileID(path), rel, fmt.Errorf("job identity or status is incomplete")))
			section.Verdict = SectionDegraded
			continue
		}
		goalID, goalState := goalField(record)
		item := Item{Kind: "job", ID: id, Verdict: ProducerVerdict(status), Evidence: rel, GoalField: goalState}
		if goalState == GoalFieldBound {
			item.GoalID = goalID
		}
		if goalState == GoalFieldAbsent || goalState == GoalFieldEmpty || goalState == GoalFieldInvalid {
			item.Problem = "goalId has a corrupt or legacy-unknown wire shape"
			section.Verdict = SectionDegraded
		}
		section.Items = append(section.Items, item)
	}
	finishSection(&section)
	return section
}

// readCompletedRounds exposes the only mechanically provable risk window:
// a completed goal-bound round whose terminal timestamp is later than that
// goal's newest landing receipt. Job records persist no return-consumption
// marker, so every such item remains UNKNOWN-CONSUMPTION instead of guessing.
func readCompletedRounds(root string) Section {
	store := "artifacts/agents/jobs + memory/receipts.log"
	section := newSection(ClassCompletedRounds, store)
	receipts, receiptState, receiptProblem := readLandingReceiptTimes(root)
	if receiptState == SectionDegraded {
		return degrade(section, "memory/receipts.log", receiptProblem)
	}
	paths, problem := jsonFiles(root, "artifacts/agents/jobs")
	if problem != "" {
		return degrade(section, "artifacts/agents/jobs", problem)
	}
	for _, path := range paths {
		rel := relativeEvidence(root, path)
		data, err := os.ReadFile(path)
		if err != nil {
			section.Items = append(section.Items, unreadableItem("completed-round-consumption", fileID(path), rel, err))
			section.Verdict = SectionDegraded
			continue
		}
		var record map[string]json.RawMessage
		if err := json.Unmarshal(data, &record); err != nil || record == nil {
			if err == nil {
				err = fmt.Errorf("record is not a JSON object")
			}
			section.Items = append(section.Items, unreadableItem("completed-round-consumption", fileID(path), rel, err))
			section.Verdict = SectionDegraded
			continue
		}
		id, idOK := rawString(record["jobId"])
		status, statusOK := rawString(record["status"])
		if !idOK || id == "" || id != fileID(path) || !statusOK || !knownJobStatus(status) {
			section.Items = append(section.Items, unreadableItem("completed-round-consumption", fileID(path), rel, fmt.Errorf("job identity or status is incomplete")))
			section.Verdict = SectionDegraded
			continue
		}
		if status != "completed" {
			continue
		}
		role, roleOK := rawString(record["role"])
		if !roleOK || role == "" {
			section.Items = append(section.Items, unreadableItem("completed-round-consumption", id, rel, fmt.Errorf("completed delegate record has no role")))
			section.Verdict = SectionDegraded
			continue
		}
		goalID, goalState := goalField(record)
		if goalState != GoalFieldBound {
			continue
		}
		receiptAt, hasReceipt := receipts[goalID]
		if !hasReceipt {
			continue
		}
		var round int64
		if err := json.Unmarshal(record["round"], &round); err != nil || round < 1 {
			section.Items = append(section.Items, unreadableItem("completed-round-consumption", id, rel, fmt.Errorf("completed delegate record has no positive round")))
			section.Verdict = SectionDegraded
			continue
		}
		endedText, endedOK := rawString(record["endedAt"])
		endedAt, endedErr := time.Parse(time.RFC3339, endedText)
		if !endedOK || endedErr != nil {
			section.Items = append(section.Items, unreadableItem("completed-round-consumption", id, rel, fmt.Errorf("completed job endedAt is not an RFC 3339 time")))
			section.Verdict = SectionDegraded
			continue
		}
		if !endedAt.After(receiptAt) {
			continue
		}
		section.Items = append(section.Items, Item{
			Kind:       "completed-round-consumption",
			ID:         id,
			Verdict:    VerdictUnknownConsumption,
			Evidence:   rel,
			Role:       role,
			GoalID:     goalID,
			GoalField:  GoalFieldBound,
			ObservedAt: endedAt.UTC().Format(time.RFC3339),
			Problem: fmt.Sprintf("job records persist no return-consumption marker; completion postdates goal %s landing receipt at %s",
				goalID, receiptAt.UTC().Format(time.RFC3339)),
		})
	}
	finishSection(&section)
	return section
}

func readLandingReceiptTimes(root string) (map[string]time.Time, SectionVerdict, string) {
	store := "memory/receipts.log"
	data, state, problem := readSingleton(root, store)
	if state != SectionReadable {
		return map[string]time.Time{}, state, problem
	}
	latest := map[string]time.Time{}
	for lineNumber, line := range strings.Split(string(data), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Split(line, "|")
		goalID := ""
		for _, field := range fields[1:] {
			if strings.HasPrefix(field, "goal=") {
				goalID = strings.TrimPrefix(field, "goal=")
				break
			}
		}
		if goalID == "" {
			continue
		}
		epoch, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			return nil, SectionDegraded, fmt.Sprintf("landing receipt line %d has a non-numeric epoch", lineNumber+1)
		}
		stamp := time.Unix(epoch, 0).UTC()
		if stamp.After(latest[goalID]) {
			latest[goalID] = stamp
		}
	}
	return latest, SectionReadable, ""
}

func knownJobStatus(status string) bool {
	switch status {
	case "pending-setup", "pending", "running", "completed", "failed", "cancelled", "timeout":
		return true
	default:
		return false
	}
}

func goalField(record map[string]json.RawMessage) (string, GoalFieldState) {
	raw, present := record["goalId"]
	if !present {
		return "", GoalFieldAbsent
	}
	if strings.TrimSpace(string(raw)) == "null" {
		return "", GoalFieldNull
	}
	value, ok := rawString(raw)
	if !ok {
		return "", GoalFieldInvalid
	}
	if value == "" {
		return "", GoalFieldEmpty
	}
	return value, GoalFieldBound
}

func rawString(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 {
		return "", false
	}
	var value string
	if json.Unmarshal(raw, &value) != nil {
		return "", false
	}
	return value, true
}

func readCensus(root string) Section {
	store := "artifacts/agents/supervision/last-census.json"
	section := newSection(ClassCensus, store)
	data, state, problem := readSingleton(root, store)
	if state == SectionEmpty {
		return section
	}
	if state == SectionDegraded {
		return degrade(section, store, problem)
	}
	var verdict census.Verdict
	if err := json.Unmarshal(data, &verdict); err != nil || verdict.SchemaVersion != 2 || verdict.CompletedAt == "" ||
		(verdict.Verdict != "SUCCESS" && verdict.Verdict != "CENSUS-FAILED") {
		if err == nil {
			err = fmt.Errorf("census verdict is incomplete")
		}
		return degrade(section, store, err.Error())
	}
	section.Items = append(section.Items, Item{Kind: "census", ID: "last-census", Verdict: ProducerVerdict(verdict.Verdict), Evidence: store, ObservedAt: verdict.CompletedAt})
	finishSection(&section)
	return section
}

func readHealth(root string, now time.Time) (steward.HealthVerdict, SectionVerdict, string) {
	store := "artifacts/agents/steward/health.json"
	data, state, problem := readSingleton(root, store)
	if state == SectionEmpty {
		return steward.HealthVerdict{}, SectionDegraded, fmt.Sprintf("health record %s is absent; age=unknown", store)
	}
	if state == SectionDegraded {
		return steward.HealthVerdict{}, SectionDegraded, fmt.Sprintf("health record %s is unreadable; age=unknown: %s", store, problem)
	}
	var record healthRecordLens
	if err := json.Unmarshal(data, &record); err != nil {
		return steward.HealthVerdict{}, SectionDegraded, fmt.Sprintf("health record %s is unreadable; age=unknown: %v", store, err)
	}
	if record.Verdict.Schema != 1 || record.Verdict.ObservedAt.IsZero() || record.Verdict.Observation < 1 ||
		!knownHealthAggregate(record.Verdict.Aggregate) || record.Verdict.Roles == nil {
		return steward.HealthVerdict{}, SectionDegraded, fmt.Sprintf("health record %s is unreadable; age=unknown: health verdict is incomplete", store)
	}
	for _, role := range record.Verdict.Roles {
		if role.Role == "" || !knownHealthStatus(string(role.Status)) {
			return steward.HealthVerdict{}, SectionDegraded, fmt.Sprintf("health record %s is unreadable; age=unknown: health verdict contains an invalid role", store)
		}
	}
	if record.Verdict.ObservedAt.After(now) {
		return record.Verdict, SectionDegraded, fmt.Sprintf("health record %s is unreadable; age=clock-regressed", store)
	}
	rawAge := now.Sub(record.Verdict.ObservedAt)
	age := rawAge.Round(time.Second)
	window := time.Duration(2*steward.TickSeconds(root)) * time.Second
	if rawAge >= window {
		return record.Verdict, SectionDegraded, fmt.Sprintf("health record %s is stale; age=%s exceeds freshness window %s", store, age, window)
	}
	return record.Verdict, SectionReadable, ""
}

func knownHealthAggregate(value string) bool {
	return value == "healthy" || value == "unhealthy" || value == "unknown"
}

func knownHealthStatus(value string) bool {
	return value == "alive" || value == "dead" || value == "unknown"
}

func healthSection(verdict steward.HealthVerdict, state SectionVerdict, problem string) Section {
	store := "artifacts/agents/steward/health.json"
	section := newSection(ClassHealth, store)
	if state == SectionDegraded {
		section.Verdict = SectionDegraded
		item := Item{Kind: "health-freshness", ID: "health-record", Verdict: "dead", Evidence: store, Problem: problem}
		if !verdict.ObservedAt.IsZero() {
			item.ObservedAt = verdict.ObservedAt.UTC().Format(time.RFC3339)
		}
		section.Items = append(section.Items, item)
		if verdict.Schema != 1 {
			return section
		}
	} else {
		section.Items = append(section.Items, Item{Kind: "health-freshness", ID: "health-record", Verdict: "alive", Evidence: store, ObservedAt: verdict.ObservedAt.UTC().Format(time.RFC3339)})
	}
	section.Items = append(section.Items, Item{Kind: "health-aggregate", ID: "health", Verdict: ProducerVerdict(verdict.Aggregate), Evidence: store, ObservedAt: verdict.ObservedAt.UTC().Format(time.RFC3339)})
	for _, role := range verdict.Roles {
		section.Items = append(section.Items, Item{Kind: "health-role", ID: string(role.Role), Role: string(role.Role), Verdict: ProducerVerdict(role.Status), Evidence: store})
	}
	finishSection(&section)
	return section
}

func readDelivery(root string, health steward.HealthVerdict, healthState SectionVerdict, healthProblem string) Section {
	store := "artifacts/agents/steward/health.json + artifacts/agents/steward/pending"
	section := newSection(ClassDelivery, store)
	if healthState == SectionDegraded {
		section = degrade(section, "artifacts/agents/steward/health.json", healthProblem)
	}
	if healthState == SectionReadable || health.Schema == 1 {
		found := false
		for _, role := range health.Roles {
			if role.Role == steward.RoleClaimedGoalDelivery {
				section.Items = append(section.Items, Item{Kind: "claimed-goal-delivery", ID: string(role.Role), Role: string(role.Role), Verdict: ProducerVerdict(role.Status), Evidence: "artifacts/agents/steward/health.json"})
				found = true
				break
			}
		}
		if !found {
			section = degrade(section, "artifacts/agents/steward/health.json", "persisted health verdict omits claimed-goal-delivery")
		}
	}
	pendingStore := "artifacts/agents/steward/pending"
	paths, problem := jsonFiles(root, pendingStore)
	if problem != "" {
		section = degrade(section, pendingStore, problem)
	} else {
		for _, path := range paths {
			rel := relativeEvidence(root, path)
			data, err := os.ReadFile(path)
			var notification steward.PendingNotification
			if err == nil {
				err = json.Unmarshal(data, &notification)
			}
			if err != nil || notification.Nonce == "" || notification.Nonce != fileID(path) || notification.Message == "" {
				if err == nil {
					err = fmt.Errorf("pending notification is incomplete")
				}
				section.Items = append(section.Items, unreadableItem("pending-notification", fileID(path), rel, err))
				section.Verdict = SectionDegraded
				continue
			}
			section.Items = append(section.Items, Item{Kind: "pending-notification", ID: notification.Nonce, Verdict: "PENDING", Evidence: rel})
		}
	}
	finishSection(&section)
	return section
}

func readAlerts(root string) Section {
	store := "artifacts/agents/steward/alerts"
	section := newSection(ClassAlerts, store)
	paths, problem := jsonFiles(root, store)
	if problem != "" {
		return degrade(section, store, problem)
	}
	for _, path := range paths {
		rel := relativeEvidence(root, path)
		data, err := os.ReadFile(path)
		var episode steward.AlertEpisode
		if err == nil {
			err = json.Unmarshal(data, &episode)
		}
		if err != nil || episode.Schema != 1 || episode.EpisodeID == "" || episode.EpisodeID != fileID(path) ||
			len(episode.Digest) != 64 || episode.Message == "" || episode.OpenedAt.IsZero() || episode.Attempts == nil ||
			!knownAlertTransport(string(episode.TransportResult)) {
			if err == nil {
				err = fmt.Errorf("alert episode is incomplete")
			}
			section.Items = append(section.Items, unreadableItem("alert-episode", fileID(path), rel, err))
			section.Verdict = SectionDegraded
			continue
		}
		stage := "ACTIVE"
		if episode.Cleared {
			stage = "CLEARED"
		} else if episode.Resolved {
			stage = "RESOLVED"
		}
		section.Items = append(section.Items, Item{Kind: "alert-episode", ID: episode.EpisodeID, Verdict: ProducerVerdict(episode.TransportResult), Evidence: rel, Stage: stage, ObservedAt: episode.OpenedAt.UTC().Format("2006-01-02T15:04:05Z")})
	}
	finishSection(&section)
	return section
}

func knownAlertTransport(value string) bool {
	return value == "PENDING" || value == "TRANSPORT_SUBMITTED" || value == "TRANSPORT_FAILED"
}

func readIntents(root string) Section {
	store := "artifacts/agents/steward/{intents,consumed,cancelled}"
	section := newSection(ClassIntents, store)
	for _, source := range []struct {
		dir   string
		stage string
	}{
		{"artifacts/agents/steward/intents", "LIVE"},
		{"artifacts/agents/steward/consumed", "CONSUMED"},
		{"artifacts/agents/steward/cancelled", "CANCELLED"},
	} {
		paths, problem := jsonFiles(root, source.dir)
		if problem != "" {
			section = degrade(section, source.dir, problem)
			continue
		}
		for _, path := range paths {
			rel := relativeEvidence(root, path)
			data, err := os.ReadFile(path)
			var intent steward.Intent
			if err == nil {
				err = json.Unmarshal(data, &intent)
			}
			if err != nil || intent.Nonce == "" || intent.Nonce != fileID(path) || intent.JobId == "" {
				if err == nil {
					err = fmt.Errorf("steward intent is incomplete")
				}
				section.Items = append(section.Items, unreadableItem("steward-intent", fileID(path), rel, err))
				section.Verdict = SectionDegraded
				continue
			}
			verdict := source.stage
			if source.stage == "CONSUMED" && intent.LaunchStamped {
				verdict = "LAUNCHED"
			}
			if intent.ReapedAt != "" {
				verdict = "REAPED"
				if intent.Outcome != "" {
					verdict = intent.Outcome
				}
			}
			section.Items = append(section.Items, Item{Kind: "steward-intent", ID: intent.Nonce, Verdict: ProducerVerdict(verdict), Evidence: rel, Stage: source.stage, GoalID: intent.Goal})
		}
	}
	sort.Slice(section.Items, func(i, j int) bool {
		if section.Items[i].ID == section.Items[j].ID {
			return section.Items[i].Stage < section.Items[j].Stage
		}
		return section.Items[i].ID < section.Items[j].ID
	})
	finishSection(&section)
	return section
}

func readBreachRoutes(root string) Section {
	store := "artifacts/agents/goal-stops"
	section := newSection(ClassBreachRoutes, store)
	paths, problem := jsonFiles(root, store)
	if problem != "" {
		return degrade(section, store, problem)
	}
	for _, path := range paths {
		rel := relativeEvidence(root, path)
		id := fileID(path)
		batch, err := goal.ReadStopBatch(root, id)
		if err != nil {
			section.Items = append(section.Items, unreadableItem("breach-stop", id, rel, err))
			section.Verdict = SectionDegraded
			continue
		}
		section.Items = append(section.Items, Item{Kind: "breach-stop", ID: batch.StopID, Verdict: ProducerVerdict(batch.State), Evidence: rel, GoalID: batch.GoalID, PendingJobs: batch.Pending, ObservedAt: batch.UpdatedAt})
	}
	finishSection(&section)
	return section
}

func newSection(class TrackedClass, store string) Section {
	return Section{Class: class, Store: store, Verdict: SectionEmpty, Items: []Item{}}
}

func finishSection(section *Section) {
	if section.Verdict != SectionDegraded {
		if len(section.Items) == 0 {
			section.Verdict = SectionEmpty
		} else {
			section.Verdict = SectionReadable
		}
	}
}

func degrade(section Section, evidence, problem string) Section {
	section.Verdict = SectionDegraded
	section.Items = append(section.Items, unreadableItem("store", string(section.Class), evidence, fmt.Errorf("%s", problem)))
	return section
}

func unreadableItem(kind, id, evidence string, err error) Item {
	problem := "unreadable persisted evidence"
	if err != nil {
		problem = err.Error()
	}
	return Item{Kind: kind, ID: id, Verdict: VerdictUnreadable, Evidence: evidence, Problem: problem}
}

func jsonFiles(root, relativeDir string) ([]string, string) {
	dir := filepath.Join(root, filepath.FromSlash(relativeDir))
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return []string{}, ""
	}
	if err != nil {
		return nil, err.Error()
	}
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, fmt.Sprintf("inspect %s: %v", entry.Name(), err)
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Sprintf("%s is not a regular file", entry.Name())
		}
		paths = append(paths, filepath.Join(dir, entry.Name()))
	}
	sort.Strings(paths)
	return paths, ""
}

func readSingleton(root, relativePath string) ([]byte, SectionVerdict, string) {
	path := filepath.Join(root, filepath.FromSlash(relativePath))
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil, SectionEmpty, ""
	}
	if err != nil {
		return nil, SectionDegraded, err.Error()
	}
	if !info.Mode().IsRegular() {
		return nil, SectionDegraded, "persisted evidence is not a regular file"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, SectionDegraded, err.Error()
	}
	return data, SectionReadable, ""
}

func fileID(path string) string {
	return strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
}

func relativeEvidence(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}
