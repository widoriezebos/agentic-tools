package identity

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

const (
	FixtureCustodianEnv        = "METASYSTEM_FIXTURE_CUSTODIAN"
	FixtureCustodianOwnerEnv   = "METASYSTEM_FIXTURE_CUSTODIAN_OWNER"
	FixtureCustodianLogEnv     = "METASYSTEM_FIXTURE_CUSTODIAN_LOG"
	FixtureCustodianChainEnv   = "METASYSTEM_FIXTURE_CUSTODIAN_CHAIN"
	FixtureCustodianRecordsEnv = "METASYSTEM_FIXTURE_CUSTODIAN_RECORDS"
	FixtureCustodianLeashEnv   = "METASYSTEM_FIXTURE_CUSTODIAN_LEASH"
	RunOwnerEnv                = "METASYSTEM_RUN_OWNER"
	custodianPoll              = 250 * time.Millisecond
	custodianBound             = 5 * time.Second
	custodianHaltMargin        = time.Second
)

// ExportRunOwner preserves an existing run owner or appends the caller's exact identity.
func ExportRunOwner(env []string) ([]string, error) {
	for _, entry := range env {
		if name, _, _ := strings.Cut(entry, "="); name == RunOwnerEnv {
			return env, nil
		}
	}
	exact, state, err := (KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != Alive {
		return nil, fmt.Errorf("identity: prove run owner: state=%s err=%v", state, err)
	}
	value, err := EncodeRef(exact.Ref())
	if err != nil {
		return nil, err
	}
	return append(append([]string(nil), env...), RunOwnerEnv+"="+value), nil
}

// ResolveRunOwner returns the exact ancestor chain that the custodian must watch.
func ResolveRunOwner(owner Ref, value string, set bool) ([]Ref, error) {
	return resolveRunOwner(KernelProber{}, ParentPid, owner, value, set)
}

func resolveRunOwner(prober Prober, parent func(int64) (int64, bool), owner Ref, value string, set bool) ([]Ref, error) {
	var required Ref
	if set {
		var err error
		required, err = ParseRef(value)
		if err != nil {
			return nil, err
		}
		exact, state, err := prober.Probe(required.Pid)
		if err != nil || state != Alive || !SameIdentity(exact, required) {
			return nil, fmt.Errorf("identity: required run owner is not alive at its exact identity")
		}
	}
	pid, known := parent(owner.Pid)
	var chain []Ref
	for known && pid > 0 {
		exact, state, err := prober.Probe(pid)
		if err != nil || state != Alive || !exact.Ref().NativeExact() {
			return nil, fmt.Errorf("identity: cannot prove ancestor %d: state=%s err=%v", pid, state, err)
		}
		chain = append(chain, exact.Ref())
		if set && SameIdentity(exact, required) {
			return chain, nil
		}
		if !set && (!exact.ArgvKnown || len(exact.Argv) < 2 || exact.Argv[0] != "go" && !strings.HasSuffix(exact.Argv[0], "/go") || exact.Argv[1] != "test" && exact.Argv[1] != "tool") {
			return chain, nil
		}
		pid, known = parent(pid)
	}
	if set {
		return nil, fmt.Errorf("identity: required run owner is not an ancestor with a readable parent chain")
	}
	return chain, nil
}

type custodianRuntime struct {
	prober     Prober
	self       Ref
	chain      []Ref
	sender     SignalFunc
	scan       func(Prober, Ref) ([]FixtureSurvivor, error)
	poll       time.Duration
	bound      time.Duration
	halt       func(int)
	records    string
	leash      func() (bool, error)
	closeWatch func() error
}

// RunCustodian watches the owner and its launcher chain, kills the owner after launcher loss, and reaps its attributed children.
func RunCustodian(owner Ref, watch io.Reader, ready io.WriteCloser, log io.Writer) error {
	if ready != nil {
		defer ready.Close()
	}
	if _, err := EncodeRef(owner); err != nil || watch == nil || log == nil {
		return fmt.Errorf("identity: fixture custodian has invalid inputs")
	}
	prober := KernelProber{}
	exact, state, err := prober.Probe(int64(os.Getpid()))
	if err != nil || state != Alive || !exact.Ref().NativeExact() {
		return fmt.Errorf("identity: fixture custodian cannot prove its own identity")
	}
	signal.Ignore(syscall.SIGTERM, syscall.SIGHUP)
	chain, err := custodianChain(owner)
	if err != nil {
		return err
	}
	if ready != nil {
		_, _ = io.WriteString(ready, "ready\n")
		_ = ready.Close()
	}
	var closeWatch func() error
	if closer, ok := watch.(io.Closer); ok {
		closeWatch = closer.Close
	}
	return runCustodian(owner, watch, log, custodianRuntime{
		prober: prober, self: exact.Ref(), chain: chain,
		scan: FixtureSurvivorsOfDeadOwner, poll: custodianPoll, bound: custodianBound, halt: os.Exit,
		records: os.Getenv(FixtureCustodianRecordsEnv), closeWatch: closeWatch,
		leash: fixtureCustodianLeash(os.Getenv(FixtureCustodianLeashEnv)),
	})
}

func custodianChain(owner Ref) ([]Ref, error) {
	if encoded, set := os.LookupEnv(FixtureCustodianChainEnv); set && os.Getenv(FixtureCustodianEnv) == "1" {
		parts := strings.FieldsFunc(encoded, func(r rune) bool { return r == '|' })
		chain := make([]Ref, len(parts))
		var err error
		for index, part := range parts {
			chain[index], err = ParseRef(part)
			if err != nil {
				return nil, fmt.Errorf("identity: invalid custodian chain: %w", err)
			}
		}
		return chain, nil
	}
	value, set := os.LookupEnv(RunOwnerEnv)
	return ResolveRunOwner(owner, value, set)
}

func runCustodian(owner Ref, watch io.Reader, log io.Writer, runtime custodianRuntime) error {
	pipeClosed := make(chan struct{}, 1)
	go func() {
		_, _ = io.Copy(io.Discard, watch)
		pipeClosed <- struct{}{}
	}()
	poll := time.NewTicker(runtime.poll)
	defer poll.Stop()
	var watchEOF <-chan struct{} = pipeClosed
	for {
		select {
		case <-watchEOF:
			watchEOF = nil
		case <-poll.C:
		}
		stop := armCustodianHalt(log, runtime)
		ownerState := AliveRef(runtime.prober, owner)
		var deadMember Ref
		if ownerState == Alive {
			for _, member := range runtime.chain {
				if AliveRef(runtime.prober, member) == Dead {
					deadMember = member
					break
				}
			}
		}
		stop()
		if deadMember.Pid != 0 {
			return reapLostLauncher(owner, deadMember, log, runtime)
		}
		switch ownerState {
		case Dead:
			return reapDeadOwner(owner, log, runtime)
		case Unknown:
			if watchEOF != nil {
				continue
			}
			if dead, err := proveOwnerDead(owner, log, runtime); err != nil {
				return err
			} else if dead {
				return reapDeadOwner(owner, log, runtime)
			}
		}
	}
}

func proveOwnerDead(owner Ref, log io.Writer, runtime custodianRuntime) (bool, error) {
	stop := armCustodianHalt(log, runtime)
	defer stop()
	deadline := time.Now().Add(runtime.bound)
	for {
		if state := AliveRef(runtime.prober, owner); state != Unknown {
			return state == Dead, nil
		}
		if !time.Now().Before(deadline) {
			return false, fmt.Errorf("identity: fixture custodian could not prove owner dead within %s", runtime.bound)
		}
		time.Sleep(runtime.poll)
	}
}

func reapLostLauncher(owner, member Ref, log io.Writer, runtime custodianRuntime) error {
	stop := armCustodianHalt(log, runtime)
	err := SignalExact(runtime.prober, owner, syscall.SIGKILL, runtime.sender)
	memberValue, _ := EncodeRef(member)
	fmt.Fprintf(log, "fixture-custodian action=kill-owner dead-launcher=%s result=%v\n", memberValue, err)
	deadline := time.Now().Add(runtime.bound)
	for AliveRef(runtime.prober, owner) != Dead {
		if !time.Now().Before(deadline) {
			stop()
			return fmt.Errorf("identity: fixture custodian owner survived launcher loss for %s", runtime.bound)
		}
		time.Sleep(runtime.poll)
	}
	stop()
	return reapDeadOwner(owner, log, runtime)
}

func armCustodianHalt(log io.Writer, runtime custodianRuntime) func() {
	if runtime.halt == nil {
		return func() {}
	}
	after := runtime.bound + custodianHaltMargin
	timer := time.AfterFunc(after, func() {
		fmt.Fprintf(log, "fixture-custodian action=halt after=%s\n", after)
		runtime.halt(2)
	})
	return func() { timer.Stop() }
}

func reapDeadOwner(owner Ref, log io.Writer, runtime custodianRuntime) error {
	if AliveRef(runtime.prober, owner) != Dead {
		return fmt.Errorf("identity: fixture custodian owner %d is not proved dead", owner.Pid)
	}
	recorded, err := fixtureCustodianRecords(runtime.records, log)
	if err != nil {
		return err
	}
	waitFixtureCustodianLeash(recorded, log, runtime)
	for _, ref := range recorded {
		signalErr := SignalExact(runtime.prober, ref, syscall.SIGKILL, runtime.sender)
		fmt.Fprintf(log, "fixture-custodian action=kill pid=%d carrier=record result=%v\n", ref.Pid, signalErr)
	}
	quietWindow := time.Second + runtime.poll
	if halfBound := runtime.bound / 2; quietWindow > halfBound {
		quietWindow = halfBound
	}
	var quietSince time.Time
	var progressSince time.Time
	previousRemaining := -1
	scanUnavailable := false
	scan := runtime.scan
	if scan == nil {
		scan = FixtureSurvivorsOfDeadOwner
	}
	for {
		stop := armCustodianHalt(log, runtime)
		pruneFixtureCustodianRecords(recorded, runtime.prober)
		survivors, err := scan(runtime.prober, owner)
		actionable := 0
		if err == nil {
			for _, survivor := range survivors {
				if survivor.Class != FixtureSurvivorCertain || sameExactRef(survivor.Ref, runtime.self) {
					continue
				}
				actionable++
				signalErr := SignalExact(runtime.prober, survivor.Ref, syscall.SIGKILL, runtime.sender)
				fmt.Fprintf(log, "fixture-custodian action=kill pid=%d carrier=%s result=%v\n", survivor.Ref.Pid, survivor.Carrier, signalErr)
			}
		} else if !scanUnavailable {
			fmt.Fprintf(log, "fixture-custodian scan=unavailable error=%v\n", err)
			scanUnavailable = true
		}
		stop()
		remaining := actionable + len(recorded)
		passFinished := time.Now()
		if previousRemaining < 0 || remaining < previousRemaining {
			progressSince = passFinished
		}
		previousRemaining = remaining
		if remaining == 0 {
			if quietSince.IsZero() {
				quietSince = passFinished
			}
			if passFinished.Sub(quietSince) >= quietWindow {
				if runtime.records != "" {
					if removeErr := os.Remove(runtime.records); removeErr != nil && !os.IsNotExist(removeErr) {
						return fmt.Errorf("identity: remove fixture custodian records: %w", removeErr)
					}
				}
				fmt.Fprint(log, FixtureCustodianCompletionLine(owner))
				return nil
			}
		} else {
			quietSince = time.Time{}
		}
		if remaining > 0 && passFinished.Sub(progressSince) >= runtime.bound {
			return fmt.Errorf("identity: fixture custodian cleanup exceeded %s: %v", runtime.bound, err)
		}
		time.Sleep(runtime.poll)
	}
}

func waitFixtureCustodianLeash(recorded map[string]Ref, log io.Writer, runtime custodianRuntime) {
	if runtime.leash == nil {
		return
	}
	if runtime.closeWatch != nil {
		if err := runtime.closeWatch(); err != nil {
			fmt.Fprintf(log, "fixture-custodian leash=unavailable error=close-watch: %v\n", err)
			return
		}
	}
	progressSince := time.Now()
	previousRemaining := len(recorded)
	for {
		for _, ref := range releasedFixtureCustodianRecords(recorded, runtime.prober) {
			fmt.Fprintf(log, "fixture-custodian action=release pid=%d carrier=record\n", ref.Pid)
		}
		checkedAt := time.Now()
		if len(recorded) < previousRemaining {
			progressSince = checkedAt
		}
		previousRemaining = len(recorded)
		held, err := runtime.leash()
		if err != nil {
			fmt.Fprintf(log, "fixture-custodian leash=unavailable error=%v\n", err)
			return
		}
		if !held {
			return
		}
		remaining := runtime.bound - checkedAt.Sub(progressSince)
		if remaining <= 0 {
			return
		}
		pause := runtime.poll
		if remaining < pause {
			pause = remaining
		}
		time.Sleep(pause)
	}
}

func pruneFixtureCustodianRecords(recorded map[string]Ref, prober Prober) {
	releasedFixtureCustodianRecords(recorded, prober)
}

func releasedFixtureCustodianRecords(recorded map[string]Ref, prober Prober) []Ref {
	var released []Ref
	for encoded, ref := range recorded {
		exact, state, err := prober.Probe(ref.Pid)
		if err == nil && (state == Dead || state == Alive && (!SameIdentity(exact, ref) || exact.Zombie)) {
			delete(recorded, encoded)
			released = append(released, ref)
		}
	}
	return released
}

func fixtureCustodianLeash(path string) func() (bool, error) {
	if path == "" {
		return nil
	}
	return func() (bool, error) {
		descriptor, err := unix.Open(path, unix.O_WRONLY|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
		if errors.Is(err, unix.ENXIO) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if err := unix.Close(descriptor); err != nil {
			return false, err
		}
		return true, nil
	}
}

func fixtureCustodianRecords(path string, log io.Writer) (map[string]Ref, error) {
	live := make(map[string]Ref)
	if path == "" {
		return live, nil
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("identity: read fixture custodian records: %w", err)
	}
	lastNewline := strings.LastIndexByte(string(contents), '\n')
	if lastNewline < 0 {
		return live, nil
	}
	released := make(map[string]bool)
	for index, line := range strings.Split(string(contents[:lastNewline]), "\n") {
		if len(line) < 2 || line[0] != '+' && line[0] != '-' {
			fmt.Fprintf(log, "fixture-custodian error=malformed-record line=%d\n", index+1)
			continue
		}
		ref, parseErr := ParseRef(line[1:])
		encoded, encodeErr := EncodeRef(ref)
		if parseErr != nil || encodeErr != nil {
			fmt.Fprintf(log, "fixture-custodian error=malformed-record line=%d\n", index+1)
			continue
		}
		if line[0] == '+' {
			live[encoded] = ref
		} else {
			released[encoded] = true
		}
	}
	for encoded := range released {
		delete(live, encoded)
	}
	return live, nil
}

// FixtureCustodianCompletionLine returns the complete line written after an owner has been reaped.
func FixtureCustodianCompletionLine(owner Ref) string {
	ownerValue, _ := EncodeRef(owner)
	return fmt.Sprintf("fixture-custodian owner=%s action=complete\n", ownerValue)
}

func sameExactRef(left, right Ref) bool {
	return left.Pid == right.Pid && left.Mode() == right.Mode() && left.StartedAtUnixMicro == right.StartedAtUnixMicro &&
		left.StartTicks == right.StartTicks && left.BootID == right.BootID
}
