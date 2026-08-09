package registry

import (
	"fmt"
	"time"
)

// REG-2: events and schema. The validator is per-event-type; there is
// no field that is sometimes present under one meaning and sometimes
// another. An event not named here is a bug, not a feature.

// Claim event names (REG-2). Claim events carry ownerTag.
const (
	EventArming     = "arming"
	EventArmed      = "armed"
	EventRelaunched = "relaunched"
	EventLaunched   = "launched"
	EventExited     = "exited"
	EventReaped     = "reaped"
	EventSwept      = "swept"
)

// Custody event names (REG-2). Custody events carry custodyId.
const (
	EventCustody         = "custody"
	EventCustodyReleased = "custody-released"
)

// Exit reasons (REG-2 `exited`, D-1). `shutdown` requires the
// shutdown-intent channel; a signal without intent is `terminated`.
var ExitReasons = map[string]bool{
	"purpose-gone":         true,
	"superseded":           true,
	"giving-up":            true,
	"establishment-failed": true,
	"shutdown":             true,
	"terminated":           true,
}

// Reap reasons (REG-2 `reaped`). `shutdown-escalated` is appended by
// the --shutdown caller after escalating past the owner-stop wait
// (SLC-R9-004, SLC-R10-002): an owner killed mid-teardown cannot speak.
var ReapReasons = map[string]bool{
	"checkout-gone":        true,
	"custodian-dead":       true,
	"owner-dead":           true,
	"establishment-orphan": true,
	"shutdown-escalated":   true,
}

// Components a `launched` record may name.
var Components = map[string]bool{"watcher": true, "reaper": true}

// Record is one validated registry event. Numeric fields are int64
// because pids and epoch seconds arrive as JSON numbers.
type Record struct {
	Event        string
	CheckoutPath string
	At           time.Time

	// Claim fields.
	OwnerTag  string
	CustodyID string // on arming/armed when custodied (D-3); on custody events always

	// armed
	OwnerPid          int64
	OwnerPidStartedAt int64
	Generation        int64

	// relaunched
	WatcherTag     string
	ReaperTag      string
	RetiredThrough int64

	// launched
	Component    string
	Pid          int64
	PidStartedAt int64

	// exited / reaped
	Reason           string
	Diagnosis        string
	TeardownComplete bool
	SweepPending     bool
	Killed           []string

	// custody
	CustodianPid          int64
	CustodianPidStartedAt int64
	Note                  string
}

// ParseRecord validates one framed object against REG-2 and returns
// its typed form. A torn marker parses to a Record with Event torn and
// nothing else (it carries no claim state).
func ParseRecord(raw map[string]any) (*Record, error) {
	version, ok := number(raw["schemaVersion"])
	if !ok || version != 1 {
		return nil, fmt.Errorf("registry record without schemaVersion 1: %v", raw["schemaVersion"])
	}
	event, _ := raw["event"].(string)
	record := &Record{Event: event}

	// Common fields (REG-1/REG-2). The torn marker's checkoutPath is
	// empty by definition; every other record must name its checkout.
	record.CheckoutPath, _ = raw["checkoutPath"].(string)
	atText, _ := raw["at"].(string)
	at, err := time.Parse(time.RFC3339, atText)
	if err != nil && event != TornEvent {
		return nil, fmt.Errorf("record %s: unparseable at %q", event, atText)
	}
	record.At = at
	if event == TornEvent {
		return record, nil
	}
	if record.CheckoutPath == "" {
		return nil, fmt.Errorf("record %s: missing checkoutPath", event)
	}

	switch event {
	case EventArming, EventArmed, EventRelaunched, EventLaunched, EventExited, EventReaped, EventSwept:
		record.OwnerTag, _ = raw["ownerTag"].(string)
		if record.OwnerTag == "" {
			return nil, fmt.Errorf("claim record %s: missing ownerTag", event)
		}
	case EventCustody, EventCustodyReleased:
		record.CustodyID, _ = raw["custodyId"].(string)
		if record.CustodyID == "" {
			return nil, fmt.Errorf("custody record %s: missing custodyId", event)
		}
	default:
		return nil, fmt.Errorf("unregistered registry event %q", event)
	}

	switch event {
	case EventArming:
		record.CustodyID, _ = raw["custodyId"].(string) // optional binding (D-3)
	case EventArmed:
		record.CustodyID, _ = raw["custodyId"].(string)
		if record.OwnerPid, ok = number(raw["ownerPid"]); !ok || record.OwnerPid < 1 {
			return nil, fmt.Errorf("armed %s: invalid ownerPid", record.OwnerTag)
		}
		if record.OwnerPidStartedAt, ok = number(raw["ownerPidStartedAt"]); !ok || record.OwnerPidStartedAt < 1 {
			return nil, fmt.Errorf("armed %s: invalid ownerPidStartedAt", record.OwnerTag)
		}
		if record.Generation, ok = number(raw["generation"]); !ok || record.Generation < 0 {
			return nil, fmt.Errorf("armed %s: invalid generation", record.OwnerTag)
		}
	case EventRelaunched:
		if record.Generation, ok = number(raw["generation"]); !ok || record.Generation < 1 {
			return nil, fmt.Errorf("relaunched %s: invalid generation", record.OwnerTag)
		}
		record.WatcherTag, _ = raw["watcherTag"].(string)
		record.ReaperTag, _ = raw["reaperTag"].(string)
		if record.WatcherTag == "" || record.ReaperTag == "" {
			return nil, fmt.Errorf("relaunched %s: missing component tags", record.OwnerTag)
		}
		if record.RetiredThrough, ok = number(raw["retiredThrough"]); !ok || record.RetiredThrough < 0 {
			return nil, fmt.Errorf("relaunched %s: invalid retiredThrough", record.OwnerTag)
		}
	case EventLaunched:
		if record.Generation, ok = number(raw["generation"]); !ok || record.Generation < 1 {
			return nil, fmt.Errorf("launched %s: invalid generation", record.OwnerTag)
		}
		record.Component, _ = raw["component"].(string)
		if !Components[record.Component] {
			return nil, fmt.Errorf("launched %s: invalid component %q", record.OwnerTag, record.Component)
		}
		if record.Pid, ok = number(raw["pid"]); !ok || record.Pid < 1 {
			return nil, fmt.Errorf("launched %s: invalid pid", record.OwnerTag)
		}
		if record.PidStartedAt, ok = number(raw["pidStartedAt"]); !ok || record.PidStartedAt < 1 {
			return nil, fmt.Errorf("launched %s: invalid pidStartedAt", record.OwnerTag)
		}
	case EventExited:
		record.Reason, _ = raw["reason"].(string)
		if !ExitReasons[record.Reason] {
			return nil, fmt.Errorf("exited %s: invalid reason %q", record.OwnerTag, record.Reason)
		}
		record.Diagnosis, _ = raw["diagnosis"].(string)
		complete, ok := raw["teardownComplete"].(bool)
		if !ok {
			return nil, fmt.Errorf("exited %s: missing teardownComplete", record.OwnerTag)
		}
		record.TeardownComplete = complete
	case EventReaped:
		record.Reason, _ = raw["reason"].(string)
		if !ReapReasons[record.Reason] {
			return nil, fmt.Errorf("reaped %s: invalid reason %q", record.OwnerTag, record.Reason)
		}
		pending, ok := raw["sweepPending"].(bool)
		if !ok {
			return nil, fmt.Errorf("reaped %s: missing sweepPending", record.OwnerTag)
		}
		record.SweepPending = pending
		if killed, ok := raw["killed"].([]any); ok {
			for _, item := range killed {
				if text, ok := item.(string); ok {
					record.Killed = append(record.Killed, text)
				}
			}
		}
	case EventCustody:
		if record.CustodianPid, ok = number(raw["custodianPid"]); !ok || record.CustodianPid < 1 {
			return nil, fmt.Errorf("custody %s: invalid custodianPid", record.CustodyID)
		}
		if record.CustodianPidStartedAt, ok = number(raw["custodianPidStartedAt"]); !ok || record.CustodianPidStartedAt < 1 {
			return nil, fmt.Errorf("custody %s: invalid custodianPidStartedAt", record.CustodyID)
		}
		record.Note, _ = raw["note"].(string)
	}
	return record, nil
}

func number(value any) (int64, bool) {
	switch v := value.(type) {
	case float64:
		if v != float64(int64(v)) {
			return 0, false
		}
		return int64(v), true
	case int64:
		return v, true
	case int:
		return int64(v), true
	}
	return 0, false
}
