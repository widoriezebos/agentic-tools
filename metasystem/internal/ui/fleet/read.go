package fleet

// The host facts this page reads without git, and the presence copy it reads
// with it. Each one is a separate function so the composition above can be
// driven by a test from fakes, and so a reader that fails says which reader
// failed rather than emptying the page.

import (
	"encoding/json"
	"os"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

// ReadHealth reads the steward's last recorded verdict for this seat.
//
// It is the LAST RECORDED verdict and is carried as such: the instant travels
// with it, and a file that cannot be read or parsed is named rather than read
// as a seat that is not armed. An absent file is no verdict, which is not an
// error: a checkout that has never been armed has never written one.
func ReadHealth(checkout string) *Health {
	data, err := os.ReadFile(steward.HealthRecordPath(checkout))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return &Health{Problem: "the steward's health record could not be read: " + err.Error()}
	}
	var record struct {
		Verdict steward.HealthVerdict `json:"verdict"`
	}
	if err := json.Unmarshal(data, &record); err != nil {
		return &Health{Problem: "the steward's health record is malformed: " + err.Error()}
	}
	verdict := record.Verdict
	health := &Health{State: verdict.Aggregate, Roles: make([]Role, 0, len(verdict.Roles))}
	if !verdict.ObservedAt.IsZero() {
		health.ObservedAt = verdict.ObservedAt.UTC().Format(time.RFC3339)
	}
	if verdict.Aggregate == "" {
		return &Health{Problem: "the steward's health record names no aggregate verdict"}
	}
	for _, role := range verdict.Roles {
		health.Roles = append(health.Roles, Role{
			Role: string(role.Role), Status: string(role.Status), Reason: role.Reason,
		})
	}
	return health
}

// ReadPublication reads this machine's own publishing state, telling an
// absent one from an unreadable one the way the seat package already does.
func ReadPublication(checkout string) (*seat.PublicationState, string) {
	state, present, err := seat.LoadPublicationState(checkout)
	if err != nil {
		return nil, err.Error()
	}
	if !present {
		return nil, ""
	}
	held := state
	return &held, ""
}

// ReadStandings reads the tick's standings file, which is the only frozen
// `since` this interface may name. A file it cannot read names no standing,
// which leaves every `since` unknown rather than inventing one.
func ReadStandings(checkout string) map[string]seat.Observation {
	state, _, err := seat.LoadStandings(checkout)
	if err != nil {
		return map[string]seat.Observation{}
	}
	return state.Machines
}

// This seat's own jobs are read by the caller now, once, because two things
// are composed from them — the newest chain this seat's fact line says, and
// every job in flight its row opens to — and two reads a moment apart could
// disagree. seat.ReadJobs and seat.NewestChain are what it calls; a wrapper
// here would only hide which of them failed.

// ReadPresence reads one namespace of the presence copy and says where it
// came from.
//
// The interface reads its own namespace once its own fetch has succeeded, and
// the tick's canonical copy until then, because a page that showed nothing
// until the first minute had passed would be a page that looked broken. In
// LocalMode nothing was ever fetched and the publishing refs are read in
// place.
func ReadPresence(transport seat.Git, ownFetchSucceeded bool) (seat.Copy, string, string) {
	namespace, source := NamespaceFor(transport.Local, ownFetchSucceeded)
	read, err := transport.Read(namespace)
	if err != nil {
		return seat.Copy{Records: map[string]seat.Record{}, Malformed: map[string]string{}},
			source, err.Error()
	}
	return read, source, ""
}

// NamespaceFor is which namespace a page reads and what the page calls it.
//
// It is its own function because it is the whole of the fallback rule and
// nothing about it needs a repository: in LocalMode the publishing refs are
// read in place, before the first success of this server's own fetch the
// tick's canonical copy stands in, and after it the interface's own namespace
// is the answer.
func NamespaceFor(local, ownFetchSucceeded bool) (namespace, source string) {
	switch {
	case local:
		return seat.UINamespace, SourceLocal
	case !ownFetchSucceeded:
		return seat.TickNamespace, SourceTick
	default:
		return seat.UINamespace, SourceInterface
	}
}
