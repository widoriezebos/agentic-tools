// Package seat carries seat presence: one record per machine, published on
// one git ref per machine by the steward tick and read by every other seat.
//
// The record exists to feed liveness — the fleet panel's presence column and
// the flagging of claims whose holder has gone silent. It authorizes nothing.
// Every field is an identifier, a number, a hash or a time; there is no free
// text, so a record can carry neither a secret nor a seat's words.
//
// The design is plans/seat-mutual-awareness-design.md, revision 7.
package seat

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// RecordSchema is the schema this engine writes. A reader accepts this
// version or any later one and ignores the keys it does not know, so that a
// schema addition never reads as a dead machine.
const RecordSchema = 1

// NoLease is the armedLineage of a runner armed while no session held the
// checkout. The literal is named, never guessed.
const NoLease = "no-lease"

// UnknownEngine stands in for an enrolled identity minted before the engine
// build was stamped on it. The record must still name something.
const UnknownEngine = "unknown"

// machineNickname is the charset a nickname must match to publish presence:
// it is the last segment of a git ref and the tail of a notification's file
// name, so a slash cannot appear in it.
var machineNickname = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// ValidateMachineName refuses a nickname that cannot be a ref segment or a
// plain file name. The engine's own ValidateMachineNickname refuses only
// whitespace (internal/goal/actor.go:34), which is not enough here.
func ValidateMachineName(name string) error {
	if !machineNickname.MatchString(name) || name == "." || strings.Contains(name, "..") {
		return fmt.Errorf("SEAT_MACHINE_NICKNAME_INVALID: the machine nickname %q is not one word of [A-Za-z0-9._-] and cannot be a presence ref segment", name)
	}
	return nil
}

// Chain is the newest non-terminal delegate job on the publishing machine —
// the panel's "running" column. Only that machine knows it.
type Chain struct {
	Root      string  `json:"root"`
	Job       string  `json:"job"`
	Role      string  `json:"role"`
	Round     int     `json:"round"`
	Goal      string  `json:"goal"`
	StartedAt *string `json:"startedAt"`
}

// Record is the published presence of one machine at one tick.
type Record struct {
	PresenceSchema int    `json:"presenceSchema"`
	Machine        string `json:"machine"`
	RepoIdentity   string `json:"repoIdentity"`
	Generation     int    `json:"generation"`
	Engine         string `json:"engine"`
	ArmedLineage   string `json:"armedLineage"`
	TickSeconds    int    `json:"tickSeconds"`
	Chain          *Chain `json:"chain"`
	TickAt         string `json:"tickAt"`
}

// Encode renders the record as the single file a presence commit carries.
func (r Record) Encode() ([]byte, error) {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// At returns the record's own tick time. A record that parsed always has one.
func (r Record) At() time.Time {
	parsed, err := time.Parse(time.RFC3339, r.TickAt)
	if err != nil {
		return time.Time{}
	}
	return parsed.UTC()
}

// rawRecord distinguishes an absent key from a zero value and a wrong type
// from a missing one, which the reader's rule requires.
type rawRecord struct {
	PresenceSchema *int    `json:"presenceSchema"`
	Machine        *string `json:"machine"`
	RepoIdentity   *string `json:"repoIdentity"`
	Generation     *int    `json:"generation"`
	Engine         *string `json:"engine"`
	ArmedLineage   *string `json:"armedLineage"`
	TickSeconds    *int    `json:"tickSeconds"`
	// Chain is read raw, because a pointer would make a present null
	// indistinguishable from an absent key, and the reader must refuse the
	// one while accepting the other.
	Chain  json.RawMessage `json:"chain"`
	TickAt *string         `json:"tickAt"`
}

// ParseRecord validates the keys this engine knows and ignores the rest. It
// refuses only a missing or lower schema, a missing required key, a wrong
// type or a malformed time; a refused record reads as standing unknown with
// the reason "malformed presence", never as reachable.
func ParseRecord(data []byte) (Record, error) {
	var raw rawRecord
	if err := json.Unmarshal(data, &raw); err != nil {
		return Record{}, fmt.Errorf("SEAT_PRESENCE_MALFORMED: the presence record is not the object this engine reads: %v", err)
	}
	malformed := func(format string, args ...any) (Record, error) {
		return Record{}, fmt.Errorf("SEAT_PRESENCE_MALFORMED: "+format, args...)
	}
	if raw.PresenceSchema == nil {
		return malformed("the presence record has no presenceSchema")
	}
	if *raw.PresenceSchema < RecordSchema {
		return malformed("the presence record has schema %d, below %d", *raw.PresenceSchema, RecordSchema)
	}
	required := []struct {
		key   string
		value *string
	}{
		{"machine", raw.Machine},
		{"repoIdentity", raw.RepoIdentity},
		{"engine", raw.Engine},
		{"armedLineage", raw.ArmedLineage},
		{"tickAt", raw.TickAt},
	}
	for _, field := range required {
		if field.value == nil {
			return malformed("the presence record has no %s", field.key)
		}
		if strings.TrimSpace(*field.value) == "" {
			return malformed("the presence record has an empty %s", field.key)
		}
	}
	if raw.Generation == nil {
		return malformed("the presence record has no generation")
	}
	if *raw.Generation < 1 {
		return malformed("the presence record has generation %d, which no armed runner publishes", *raw.Generation)
	}
	if raw.TickSeconds == nil {
		return malformed("the presence record has no tickSeconds")
	}
	if *raw.TickSeconds < 1 {
		return malformed("the presence record has tickSeconds %d, which is not a cadence", *raw.TickSeconds)
	}
	if raw.Chain == nil {
		return malformed("the presence record has no chain")
	}
	if err := ValidateMachineName(*raw.Machine); err != nil {
		return malformed("the presence record names %q, which is not a publishable machine nickname", *raw.Machine)
	}
	tickAt, err := parsePresenceTime(*raw.TickAt)
	if err != nil {
		return malformed("the presence record has tickAt %q: %v", *raw.TickAt, err)
	}
	record := Record{
		PresenceSchema: *raw.PresenceSchema,
		Machine:        *raw.Machine,
		RepoIdentity:   *raw.RepoIdentity,
		Generation:     *raw.Generation,
		Engine:         *raw.Engine,
		ArmedLineage:   *raw.ArmedLineage,
		TickSeconds:    *raw.TickSeconds,
		TickAt:         tickAt.Format(time.RFC3339),
	}
	if string(raw.Chain) != "null" {
		chain, err := parseChain(raw.Chain)
		if err != nil {
			return malformed("%v", err)
		}
		record.Chain = chain
	}
	return record, nil
}

// rawChain tells an absent chain field from a present empty one, exactly as
// rawRecord does for the record: a chain of empty fields is not a chain, and
// a reader that accepted it would report a machine as running nothing in
// particular rather than saying the record is malformed.
type rawChain struct {
	Root  *string `json:"root"`
	Job   *string `json:"job"`
	Role  *string `json:"role"`
	Round *int    `json:"round"`
	Goal  *string `json:"goal"`
	// StartedAt is read raw for the same reason the record reads its chain
	// raw: a pointer would make a present null look like an absent key.
	StartedAt json.RawMessage `json:"startedAt"`
}

// parseChain validates the fields this engine knows and ignores the rest.
func parseChain(data json.RawMessage) (*Chain, error) {
	var raw rawChain
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("the presence record's chain is neither an object nor null: %v", err)
	}
	named := []struct {
		key   string
		value *string
	}{{"root", raw.Root}, {"job", raw.Job}, {"role", raw.Role}}
	for _, field := range named {
		if field.value == nil {
			return nil, fmt.Errorf("the presence record's chain has no %s", field.key)
		}
		if strings.TrimSpace(*field.value) == "" {
			return nil, fmt.Errorf("the presence record's chain has an empty %s", field.key)
		}
	}
	if raw.Round == nil {
		return nil, fmt.Errorf("the presence record's chain has no round")
	}
	if *raw.Round < 0 {
		return nil, fmt.Errorf("the presence record's chain has round %d", *raw.Round)
	}
	if raw.Goal == nil {
		return nil, fmt.Errorf("the presence record's chain has no goal")
	}
	chain := Chain{Root: *raw.Root, Job: *raw.Job, Role: *raw.Role, Round: *raw.Round, Goal: *raw.Goal}
	if raw.StartedAt == nil {
		return nil, fmt.Errorf("the presence record's chain has no startedAt")
	}
	if string(raw.StartedAt) != "null" {
		var started string
		if err := json.Unmarshal(raw.StartedAt, &started); err != nil {
			return nil, fmt.Errorf("the presence record's chain has a startedAt that is neither a time nor null")
		}
		if _, err := parsePresenceTime(started); err != nil {
			return nil, fmt.Errorf("the presence record's chain has startedAt %q: %v", started, err)
		}
		chain.StartedAt = &started
	}
	return &chain, nil
}

// parsePresenceTime holds tickAt to RFC 3339 UTC at second precision, so a
// reader's age arithmetic never depends on a zone or a fraction.
func parsePresenceTime(value string) (time.Time, error) {
	if !strings.HasSuffix(value, "Z") {
		return time.Time{}, fmt.Errorf("a presence time is RFC 3339 UTC and ends in Z")
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("a presence time is RFC 3339: %v", err)
	}
	if !parsed.Equal(parsed.Truncate(time.Second)) {
		return time.Time{}, fmt.Errorf("a presence time carries whole seconds")
	}
	return parsed.UTC(), nil
}

// FormatTime renders a tick time the way every record carries it.
func FormatTime(at time.Time) string { return at.UTC().Truncate(time.Second).Format(time.RFC3339) }
