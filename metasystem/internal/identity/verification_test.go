package identity

import (
	"errors"
	"testing"
	"time"
)

type verificationStartRead struct {
	exact Exact
	state Liveness
	err   error
}

type scriptedVerificationReader struct {
	starts    []verificationStartRead
	argv      []string
	argvKnown bool
	startRead int
	argvRead  int
}

func (r *scriptedVerificationReader) Probe(int64) (Exact, Liveness, error) {
	if r.startRead >= len(r.starts) {
		return Exact{}, Unknown, errors.New("unexpected start read")
	}
	read := r.starts[r.startRead]
	r.startRead++
	// Ported: argv rides the probe itself; the second read carries the
	// scripted argv exactly as the kernel prober attaches it.
	exact := read.exact
	exact.Argv = r.argv
	exact.ArgvKnown = r.argvKnown
	r.argvRead++
	return exact, read.state, read.err
}

func verificationShape(name string, token int64) Exact {
	switch name {
	case "darwin-microseconds":
		// Ported: main records darwin seconds; distinct tokens differ
		// by whole seconds.
		return Exact{Pid: 41, StartedAt: time.Unix(100_000+token, 0)}
	case "linux-ticks-boot-id":
		return Exact{Pid: 41, StartedAt: time.Unix(100, 0), StartTicks: 7000 + token, BootID: "boot-a"}
	default:
		panic("unknown verification shape")
	}
}

func tagInPosition(argv []string) bool {
	for i, word := range argv {
		if word == "--instance-tag" && i+1 < len(argv) && argv[i+1] == "job-tag" {
			return true
		}
	}
	return false
}

func TestOrderedVerificationTable(t *testing.T) {
	for _, platform := range []string{"darwin-microseconds", "linux-ticks-boot-id"} {
		platform := platform
		t.Run(platform, func(t *testing.T) {
			stable := verificationShape(platform, 1)
			changed := verificationShape(platform, 2)
			rows := []struct {
				name          string
				starts        []verificationStartRead
				argv          []string
				argvKnown     bool
				want          VerificationOutcome
				wantPresence  Liveness
				wantStartRead int
				wantArgvRead  int
			}{
				{
					name: "first read gone stops the sandwich", starts: []verificationStartRead{{state: Dead}},
					want: VerificationDead, wantPresence: Dead, wantStartRead: 1, wantArgvRead: 1,
				},
				{
					name: "first read unknown", starts: []verificationStartRead{{state: Unknown, err: errors.New("permission denied")}},
					want: VerificationIndeterminate, wantPresence: Unknown, wantStartRead: 1, wantArgvRead: 1,
				},
				{
					name: "second read unknown", starts: []verificationStartRead{{exact: stable, state: Alive}, {state: Unknown, err: errors.New("permission denied")}},
					argv: []string{"adapter", "--instance-tag", "job-tag"}, argvKnown: true,
					want: VerificationIndeterminate, wantPresence: Unknown, wantStartRead: 2, wantArgvRead: 2,
				},
				{
					name: "second read gone", starts: []verificationStartRead{{exact: stable, state: Alive}, {state: Dead}},
					argv: []string{"adapter", "--instance-tag", "job-tag"}, argvKnown: true,
					want: VerificationDead, wantPresence: Dead, wantStartRead: 2, wantArgvRead: 2,
				},
				{
					name: "start changed during argv read", starts: []verificationStartRead{{exact: stable, state: Alive}, {exact: changed, state: Alive}},
					argv: []string{"adapter", "--instance-tag", "job-tag"}, argvKnown: true,
					want: VerificationIndeterminate, wantPresence: Alive, wantStartRead: 2, wantArgvRead: 2,
				},
				{
					name: "argv unreadable with stable live identity", starts: []verificationStartRead{{exact: stable, state: Alive}, {exact: stable, state: Alive}},
					argvKnown: false,
					want:      VerificationIndeterminate, wantPresence: Alive, wantStartRead: 2, wantArgvRead: 2,
				},
				{
					name: "tag absent from its position", starts: []verificationStartRead{{exact: stable, state: Alive}, {exact: stable, state: Alive}},
					argv: []string{"rg", "job-tag"}, argvKnown: true,
					want: VerificationNotOurs, wantPresence: Alive, wantStartRead: 2, wantArgvRead: 2,
				},
				{
					name: "stable identity and positioned tag", starts: []verificationStartRead{{exact: stable, state: Alive}, {exact: stable, state: Alive}},
					argv: []string{"adapter", "--instance-tag", "job-tag"}, argvKnown: true,
					want: VerificationVerified, wantPresence: Alive, wantStartRead: 2, wantArgvRead: 2,
				},
			}
			for _, row := range rows {
				t.Run(row.name, func(t *testing.T) {
					reader := &scriptedVerificationReader{starts: row.starts, argv: row.argv, argvKnown: row.argvKnown}
					got := VerifyProcess(reader, 41, tagInPosition)
					if got.Outcome != row.want || got.Presence != row.wantPresence {
						t.Fatalf("outcome=%s presence=%s, want %s/%s", got.Outcome, got.Presence, row.want, row.wantPresence)
					}
					if reader.startRead != row.wantStartRead || reader.argvRead != row.wantArgvRead {
						t.Fatalf("reads=start:%d argv:%d, want start:%d argv:%d", reader.startRead, reader.argvRead, row.wantStartRead, row.wantArgvRead)
					}
					if row.name == "argv unreadable with stable live identity" && got.Identity.ArgvKnown {
						t.Fatal("an unreadable argv must remain an ordinary live observation with ArgvKnown=false")
					}
				})
			}
		})
	}
}

func TestExactIdentityComparisonPortedShapes(t *testing.T) {
	// Ported: main has no microsecond mode — a darwin same-second pid
	// reuse is indistinguishable BY DESIGN (SameIdentity matches), and
	// the sub-second rejection the wip proved has no shape to prove
	// here. The linux pair still rejects exactly.
	linuxLive := Exact{Pid: 41, StartedAt: time.Unix(100, 0), StartTicks: 7002, BootID: "boot-a"}
	linuxRecorded := Ref{Pid: 41, StartedAtSec: 100, StartTicks: 7001, BootID: "boot-a"}
	if SameIdentity(linuxLive, linuxRecorded) {
		t.Fatal("a linux ticks mismatch must reject")
	}
	darwinLive := Exact{Pid: 41, StartedAt: time.Unix(100, 900_000_000)}
	if !SameIdentity(darwinLive, Ref{Pid: 41, StartedAtSec: 100}) {
		t.Fatal("the whole-second comparison is darwin's identity rule")
	}
}
