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

func (r *scriptedVerificationReader) ReadStart(int64) (Exact, Liveness, error) {
	if r.startRead >= len(r.starts) {
		return Exact{}, Unknown, errors.New("unexpected start read")
	}
	read := r.starts[r.startRead]
	r.startRead++
	return read.exact, read.state, read.err
}

func (r *scriptedVerificationReader) ReadArgv(int64) ([]string, bool) {
	r.argvRead++
	return r.argv, r.argvKnown
}

func verificationShape(name string, token int64) Exact {
	switch name {
	case "darwin-microseconds":
		return Exact{Pid: 41, StartedAt: time.UnixMicro(100_000_000 + token)}
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
					want: VerificationDead, wantPresence: Dead, wantStartRead: 1,
				},
				{
					name: "first read unknown", starts: []verificationStartRead{{state: Unknown, err: errors.New("permission denied")}},
					want: VerificationIndeterminate, wantPresence: Unknown, wantStartRead: 1,
				},
				{
					name: "second read unknown", starts: []verificationStartRead{{exact: stable, state: Alive}, {state: Unknown, err: errors.New("permission denied")}},
					argv: []string{"adapter", "--instance-tag", "job-tag"}, argvKnown: true,
					want: VerificationIndeterminate, wantPresence: Unknown, wantStartRead: 2, wantArgvRead: 1,
				},
				{
					name: "second read gone", starts: []verificationStartRead{{exact: stable, state: Alive}, {state: Dead}},
					argv: []string{"adapter", "--instance-tag", "job-tag"}, argvKnown: true,
					want: VerificationDead, wantPresence: Dead, wantStartRead: 2, wantArgvRead: 1,
				},
				{
					name: "start changed during argv read", starts: []verificationStartRead{{exact: stable, state: Alive}, {exact: changed, state: Alive}},
					argv: []string{"adapter", "--instance-tag", "job-tag"}, argvKnown: true,
					want: VerificationIndeterminate, wantPresence: Alive, wantStartRead: 2, wantArgvRead: 1,
				},
				{
					name: "argv unreadable with stable live identity", starts: []verificationStartRead{{exact: stable, state: Alive}, {exact: stable, state: Alive}},
					argvKnown: false,
					want:      VerificationIndeterminate, wantPresence: Alive, wantStartRead: 2, wantArgvRead: 1,
				},
				{
					name: "tag absent from its position", starts: []verificationStartRead{{exact: stable, state: Alive}, {exact: stable, state: Alive}},
					argv: []string{"rg", "job-tag"}, argvKnown: true,
					want: VerificationNotOurs, wantPresence: Alive, wantStartRead: 2, wantArgvRead: 1,
				},
				{
					name: "stable identity and positioned tag", starts: []verificationStartRead{{exact: stable, state: Alive}, {exact: stable, state: Alive}},
					argv: []string{"adapter", "--instance-tag", "job-tag"}, argvKnown: true,
					want: VerificationVerified, wantPresence: Alive, wantStartRead: 2, wantArgvRead: 1,
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

func TestExactIdentityRejectsSameSecondPidReuse(t *testing.T) {
	darwinLive := Exact{Pid: 41, StartedAt: time.UnixMicro(100_000_002)}
	darwinRecorded := Ref{Pid: 41, StartedAtSec: 100, StartedAtUnixMicro: 100_000_001}
	if comparison := Compare(darwinLive, darwinRecorded); comparison.Matches || comparison.Mode != CompareDarwinMicroseconds {
		t.Fatalf("darwin comparison = %+v, want a microsecond-exact rejection", comparison)
	}

	linuxLive := Exact{Pid: 41, StartedAt: time.Unix(100, 0), StartTicks: 7002, BootID: "boot-a"}
	linuxRecorded := Ref{Pid: 41, StartedAtSec: 100, StartTicks: 7001, BootID: "boot-a"}
	if comparison := Compare(linuxLive, linuxRecorded); comparison.Matches || comparison.Mode != CompareLinuxTicksBootID {
		t.Fatalf("linux comparison = %+v, want a ticks-and-boot-id rejection", comparison)
	}
}

func TestLegacySecondsComparisonIsLabeled(t *testing.T) {
	live := Exact{Pid: 41, StartedAt: time.UnixMicro(100_900_000)}
	comparison := Compare(live, Ref{Pid: 41, StartedAtSec: 100})
	if !comparison.Matches || comparison.Mode != CompareLegacySeconds {
		t.Fatalf("legacy comparison = %+v", comparison)
	}
}
