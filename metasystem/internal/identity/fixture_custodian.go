package identity

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"
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
	FixtureCustodianPollEnv    = "METASYSTEM_FIXTURE_CUSTODIAN_POLL"
	FixtureCustodianBoundEnv   = "METASYSTEM_FIXTURE_CUSTODIAN_BOUND"
	RunOwnerEnv                = "METASYSTEM_RUN_OWNER"
	custodianPoll              = 250 * time.Millisecond
	custodianBound             = 5 * time.Second
	custodianHaltMargin        = time.Second
	custodianSettledScans      = 2
	custodianLeashBoundFactor  = 10
)

var errFixtureCustodianOwnerNotAlive = errors.New("fixture custodian owner is not alive")

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
	prober      Prober
	self        Ref
	chain       []Ref
	sender      SignalFunc
	scan        func(Prober, Ref) ([]FixtureSurvivor, error)
	descendants func(Prober, Ref) ([]Ref, error)
	poll        time.Duration
	bound       time.Duration
	haltMargin  time.Duration
	halt        func(int)
	clock       custodianClock
	records     string
	leash       func() (bool, error)
	closeWatch  func() error
	observed    map[string]Ref
}

type custodianScanConvergence struct {
	unchanged int
}

func (convergence *custodianScanConvergence) reset() {
	convergence.unchanged = 0
}

func (convergence *custodianScanConvergence) settled(changed bool) bool {
	if changed {
		convergence.reset()
		return false
	}
	convergence.unchanged++
	return convergence.unchanged >= custodianSettledScans
}

func fixtureCustodianLeashBound(bound time.Duration) time.Duration {
	return custodianLeashBoundFactor * bound
}

type custodianTimer interface {
	Stop() bool
}

type custodianClock interface {
	Now() time.Time
	After(time.Duration) <-chan time.Time
	AfterFunc(time.Duration, func()) custodianTimer
	Sleep(time.Duration)
}

type systemCustodianClock struct{}

func (systemCustodianClock) Now() time.Time { return time.Now() }
func (systemCustodianClock) After(duration time.Duration) <-chan time.Time {
	return time.After(duration)
}
func (systemCustodianClock) AfterFunc(duration time.Duration, action func()) custodianTimer {
	return time.AfterFunc(duration, action)
}
func (systemCustodianClock) Sleep(duration time.Duration) { time.Sleep(duration) }

// RunCustodian watches the owner and its launcher chain, kills the owner after launcher loss, and reaps its attributed children.
func RunCustodian(owner Ref, watch io.Reader, ready io.WriteCloser, log io.Writer) error {
	if ready != nil {
		defer ready.Close()
	}
	if _, err := EncodeRef(owner); err != nil || watch == nil || log == nil {
		return fmt.Errorf("identity: fixture custodian has invalid inputs")
	}
	poll, bound, err := custodianTiming(os.LookupEnv)
	if err != nil {
		return err
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
		scan: FixtureSurvivorsOfDeadOwner, poll: poll, bound: bound, haltMargin: custodianHaltMargin,
		descendants: func(prober Prober, owner Ref) ([]Ref, error) {
			return fixtureDescendants(prober, owner, AllPids, ParentPid)
		},
		halt: os.Exit, clock: systemCustodianClock{},
		records: os.Getenv(FixtureCustodianRecordsEnv), closeWatch: closeWatch,
		leash: fixtureCustodianLeash(os.Getenv(FixtureCustodianLeashEnv)),
	})
}

func custodianTiming(lookup func(string) (string, bool)) (time.Duration, time.Duration, error) {
	read := func(name string, fallback time.Duration) (time.Duration, error) {
		value, present := lookup(name)
		if !present {
			return fallback, nil
		}
		duration, err := time.ParseDuration(value)
		if err != nil || duration <= 0 {
			return 0, fmt.Errorf("identity: %s must be a positive duration, got %q", name, value)
		}
		return duration, nil
	}
	poll, err := read(FixtureCustodianPollEnv, custodianPoll)
	if err != nil {
		return 0, 0, err
	}
	bound, err := read(FixtureCustodianBoundEnv, custodianBound)
	if err != nil {
		return 0, 0, err
	}
	return poll, bound, nil
}

func (runtime custodianRuntime) timing() (custodianClock, time.Duration) {
	clock := runtime.clock
	if clock == nil {
		clock = systemCustodianClock{}
	}
	margin := runtime.haltMargin
	if margin == 0 {
		margin = custodianHaltMargin
	}
	return clock, margin
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
	if runtime.observed == nil {
		runtime.observed = make(map[string]Ref)
	}
	fmt.Fprint(log, FixtureCustodianWatchLine(owner))
	pipeClosed := make(chan struct{}, 1)
	go func() {
		_, _ = io.Copy(io.Discard, watch)
		pipeClosed <- struct{}{}
	}()
	clock, _ := runtime.timing()
	var watchEOF <-chan struct{} = pipeClosed
	observationUnavailable := false
	for {
		select {
		case <-watchEOF:
			watchEOF = nil
		case <-clock.After(runtime.poll):
		}
		stop := armCustodianHalt(log, runtime)
		var lostMember Ref
		for _, member := range runtime.chain {
			exact, state, _ := runtime.prober.Probe(member.Pid)
			if state == Dead || state == Alive && (!SameIdentity(exact, member) || exact.Zombie || exact.Exiting) {
				lostMember = member
				break
			}
		}
		ownerState := AliveRef(runtime.prober, owner)
		if ownerState == Alive {
			if lostMember.Pid == 0 && runtime.descendants != nil {
				descendants, err := runtime.descendants(runtime.prober, owner)
				if err != nil {
					if errors.Is(err, errFixtureCustodianOwnerNotAlive) {
						observationUnavailable = false
					} else if !observationUnavailable {
						fmt.Fprintf(log, "fixture-custodian observation=unavailable error=%v\n", err)
						observationUnavailable = true
					}
				} else {
					observationUnavailable = false
					for _, ref := range descendants {
						if sameExactRef(ref, runtime.self) {
							continue
						}
						identity, encodeErr := EncodeRef(ref)
						if encodeErr != nil {
							continue
						}
						if _, known := runtime.observed[identity]; known {
							continue
						}
						runtime.observed[identity] = ref
						fmt.Fprintf(log, "fixture-custodian action=observe pid=%d carrier=descendant identity=%s\n", ref.Pid, identity)
					}
				}
			}
		}
		stop()
		if lostMember.Pid != 0 {
			return reapLostLauncher(owner, lostMember, log, runtime)
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

func fixtureDescendants(
	prober Prober,
	owner Ref,
	allPids func() ([]int64, error),
	parentPid func(int64) (int64, bool),
) ([]Ref, error) {
	if prober == nil || allPids == nil || parentPid == nil {
		return nil, fmt.Errorf("identity: fixture custodian owner %d is not proved alive for descendant observation", owner.Pid)
	}
	if AliveRef(prober, owner) != Alive {
		return nil, fmt.Errorf("identity: fixture custodian owner %d is not alive before descendant observation: %w", owner.Pid, errFixtureCustodianOwnerNotAlive)
	}
	pids, err := allPids()
	if err != nil {
		return nil, err
	}
	parents := make(map[int64]int64, len(pids))
	for _, pid := range pids {
		if parent, known := parentPid(pid); known {
			parents[pid] = parent
		}
	}
	var descendants []Ref
	for _, pid := range pids {
		if pid == owner.Pid || !descendsFrom(pid, owner.Pid, parents) {
			continue
		}
		before, state, probeErr := prober.Probe(pid)
		if probeErr != nil || state != Alive || before.Pid != pid || !before.Ref().NativeExact() ||
			isFixtureCustodian(before) || !descendsFromFacts(pid, owner.Pid, parentPid) {
			continue
		}
		after, state, probeErr := prober.Probe(pid)
		if probeErr == nil && state == Alive && SameIdentity(after, before.Ref()) &&
			!isFixtureCustodian(after) && descendsFromFacts(pid, owner.Pid, parentPid) {
			descendants = append(descendants, after.Ref())
		}
	}
	sort.Slice(descendants, func(i, j int) bool { return descendants[i].Pid < descendants[j].Pid })
	if AliveRef(prober, owner) != Alive {
		return nil, fmt.Errorf("identity: fixture custodian owner %d is not alive after descendant observation: %w", owner.Pid, errFixtureCustodianOwnerNotAlive)
	}
	return descendants, nil
}

func isFixtureCustodian(exact Exact) bool {
	if !exact.EnvironKnown {
		return false
	}
	for _, entry := range exact.Environ {
		if entry == FixtureCustodianEnv+"=1" {
			return true
		}
	}
	return false
}

func descendsFromFacts(pid, owner int64, parentPid func(int64) (int64, bool)) bool {
	seen := make(map[int64]struct{})
	for pid > 0 {
		if _, duplicate := seen[pid]; duplicate {
			return false
		}
		seen[pid] = struct{}{}
		parent, known := parentPid(pid)
		if !known || parent <= 0 {
			return false
		}
		if parent == owner {
			return true
		}
		pid = parent
	}
	return false
}

func descendsFrom(pid, owner int64, parents map[int64]int64) bool {
	seen := make(map[int64]struct{})
	for pid > 0 {
		if _, duplicate := seen[pid]; duplicate {
			return false
		}
		seen[pid] = struct{}{}
		parent, known := parents[pid]
		if !known || parent <= 0 {
			return false
		}
		if parent == owner {
			return true
		}
		pid = parent
	}
	return false
}

func proveOwnerDead(owner Ref, log io.Writer, runtime custodianRuntime) (bool, error) {
	stop := armCustodianHalt(log, runtime)
	defer stop()
	clock, _ := runtime.timing()
	deadline := clock.Now().Add(runtime.bound)
	for {
		if state := AliveRef(runtime.prober, owner); state != Unknown {
			return state == Dead, nil
		}
		if !clock.Now().Before(deadline) {
			return false, fmt.Errorf("identity: fixture custodian could not prove owner dead within %s", runtime.bound)
		}
		clock.Sleep(runtime.poll)
	}
}

func reapLostLauncher(owner, member Ref, log io.Writer, runtime custodianRuntime) error {
	stop := armCustodianHalt(log, runtime)
	clock, _ := runtime.timing()
	err := SignalExact(runtime.prober, owner, syscall.SIGKILL, runtime.sender)
	memberValue, _ := EncodeRef(member)
	fmt.Fprintf(log, "fixture-custodian action=kill-owner dead-launcher=%s result=%v\n", memberValue, err)
	deadline := clock.Now().Add(runtime.bound)
	for AliveRef(runtime.prober, owner) != Dead {
		if !clock.Now().Before(deadline) {
			stop()
			return fmt.Errorf("identity: fixture custodian owner survived launcher loss for %s", runtime.bound)
		}
		clock.Sleep(runtime.poll)
	}
	stop()
	return reapDeadOwner(owner, log, runtime)
}

func armCustodianHalt(log io.Writer, runtime custodianRuntime) func() {
	if runtime.halt == nil {
		return func() {}
	}
	clock, margin := runtime.timing()
	after := runtime.bound + margin
	timer := clock.AfterFunc(after, func() {
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
	released, signaled := waitFixtureCustodianLeash(recorded, log, runtime)
	tracked := make(map[string]Ref, len(recorded)+len(runtime.observed))
	for identity, ref := range signaled {
		tracked[identity] = ref
	}
	for identity, ref := range recorded {
		if _, exited := released[identity]; exited {
			continue
		}
		if _, killed := signaled[identity]; killed {
			continue
		}
		signalErr := SignalExact(runtime.prober, ref, syscall.SIGKILL, runtime.sender)
		fmt.Fprintf(log, "fixture-custodian action=kill pid=%d carrier=record result=%v\n", ref.Pid, signalErr)
		tracked[identity] = ref
	}
	for identity, ref := range runtime.observed {
		if _, exited := released[identity]; exited {
			continue
		}
		if _, recorded := tracked[identity]; recorded {
			continue
		}
		exact, state, probeErr := runtime.prober.Probe(ref.Pid)
		if fixtureCustodianIdentityReleased(exact, state, probeErr, ref) {
			released[identity] = ref
			fmt.Fprintf(log, "fixture-custodian action=release pid=%d carrier=descendant\n", ref.Pid)
			continue
		}
		if probeErr == nil && state == Alive && SameIdentity(exact, ref) && !exact.Zombie {
			signalErr := SignalExact(runtime.prober, ref, syscall.SIGKILL, runtime.sender)
			fmt.Fprintf(log, "fixture-custodian action=kill pid=%d carrier=descendant result=%v\n", ref.Pid, signalErr)
		}
		tracked[identity] = ref
	}
	clock, _ := runtime.timing()
	progressSince := clock.Now()
	lowestRemaining := -1
	var convergence custodianScanConvergence
	seen := make(map[string]struct{})
	scanUnavailable := false
	scanCount := 0
	scan := runtime.scan
	if scan == nil {
		scan = FixtureSurvivorsOfDeadOwner
	}
	for {
		scanCount++
		stop := armCustodianHalt(log, runtime)
		pruneFixtureCustodianRecords(tracked, runtime.prober)
		survivors, err := scan(runtime.prober, owner)
		actionable := 0
		newDescendants := 0
		if err == nil {
			for _, survivor := range survivors {
				if survivor.Class != FixtureSurvivorCertain || sameExactRef(survivor.Ref, runtime.self) {
					continue
				}
				identity, encodeErr := EncodeRef(survivor.Ref)
				if encodeErr == nil {
					if _, exited := released[identity]; exited {
						continue
					}
				}
				actionable++
				if encodeErr == nil {
					if _, known := seen[identity]; !known {
						seen[identity] = struct{}{}
						newDescendants++
					}
				}
				signalErr := SignalExact(runtime.prober, survivor.Ref, syscall.SIGKILL, runtime.sender)
				fmt.Fprintf(log, "fixture-custodian action=kill pid=%d carrier=%s result=%v\n", survivor.Ref.Pid, survivor.Carrier, signalErr)
			}
		} else if !scanUnavailable {
			fmt.Fprintf(log, "fixture-custodian scan=unavailable error=%v\n", err)
			scanUnavailable = true
		}
		stop()
		remaining := actionable + len(tracked)
		passFinished := clock.Now()
		if lowestRemaining < 0 || remaining < lowestRemaining {
			progressSince = passFinished
			lowestRemaining = remaining
		}
		settled := convergence.settled(newDescendants > 0)
		if remaining == 0 && settled {
			if runtime.records != "" {
				if removeErr := os.Remove(runtime.records); removeErr != nil && !os.IsNotExist(removeErr) {
					return fmt.Errorf("identity: remove fixture custodian records: %w", removeErr)
				}
			}
			fmt.Fprint(log, FixtureCustodianCompletionLine(owner))
			return nil
		}
		if remaining > 0 && passFinished.Sub(progressSince) >= runtime.bound {
			return fmt.Errorf("identity: fixture custodian cleanup exceeded %s after %d scans: remaining=%d new=%d lowest=%d scan-error=%v",
				runtime.bound, scanCount, remaining, newDescendants, lowestRemaining, err)
		}
		clock.Sleep(runtime.poll)
	}
}

func waitFixtureCustodianLeash(recorded map[string]Ref, log io.Writer, runtime custodianRuntime) (map[string]Ref, map[string]Ref) {
	released := make(map[string]Ref)
	signaled := make(map[string]Ref)
	if runtime.leash == nil {
		return released, signaled
	}
	if runtime.closeWatch != nil {
		if err := runtime.closeWatch(); err != nil {
			fmt.Fprintf(log, "fixture-custodian leash=unavailable error=close-watch: %v\n", err)
			return released, signaled
		}
	}
	clock, _ := runtime.timing()
	progressSince := clock.Now()
	var emptyConvergence custodianScanConvergence
	var runningConvergence custodianScanConvergence
	var previousLive map[string]Exact
	var previousRunning map[string]Exact
	bound := fixtureCustodianLeashBound(runtime.bound)
	for {
		stop := armCustodianHalt(log, runtime)
		inactive := make(map[string]Ref, len(released)+len(signaled))
		for identity, ref := range released {
			inactive[identity] = ref
		}
		for identity, ref := range signaled {
			inactive[identity] = ref
		}
		justReleased, live := probeFixtureCustodianRecords(recorded, inactive, runtime.prober)
		for identity, ref := range justReleased {
			released[identity] = ref
			fmt.Fprintf(log, "fixture-custodian action=release pid=%d carrier=record\n", ref.Pid)
		}
		checkedAt := clock.Now()
		changed := !sameFixtureCustodianRecordSet(previousLive, live)
		if changed {
			progressSince = checkedAt
		}
		previousLive = live
		running := fixtureCustodianRunningRecords(live)
		runningChanged := !sameFixtureCustodianRecordSet(previousRunning, running)
		previousRunning = running
		held, err := runtime.leash()
		stop()
		if err != nil {
			fmt.Fprintf(log, "fixture-custodian leash=unavailable error=%v\n", err)
			return released, signaled
		}
		if held {
			emptyConvergence.reset()
			runningConvergence.reset()
		} else if len(running) > 0 {
			emptyConvergence.reset()
			if runningConvergence.settled(runningChanged) {
				for identity := range running {
					ref := recorded[identity]
					signalErr := SignalExact(runtime.prober, ref, syscall.SIGKILL, runtime.sender)
					fmt.Fprintf(log, "fixture-custodian action=kill pid=%d carrier=record result=%v\n", ref.Pid, signalErr)
					signaled[identity] = ref
				}
				if len(running) == len(live) {
					return released, signaled
				}
			}
		} else {
			runningConvergence.reset()
			if len(live) > 0 {
				emptyConvergence.reset()
			} else if emptyConvergence.settled(changed) {
				return released, signaled
			}
		}
		remaining := bound - checkedAt.Sub(progressSince)
		if remaining <= 0 {
			return released, signaled
		}
		pause := runtime.poll
		if remaining < pause {
			pause = remaining
		}
		clock.Sleep(pause)
	}
}

func pruneFixtureCustodianRecords(recorded map[string]Ref, prober Prober) {
	released, _ := probeFixtureCustodianRecords(recorded, nil, prober)
	for identity := range released {
		delete(recorded, identity)
	}
}

func probeFixtureCustodianRecords(recorded, alreadyReleased map[string]Ref, prober Prober) (map[string]Ref, map[string]Exact) {
	released := make(map[string]Ref)
	live := make(map[string]Exact)
	for encoded, ref := range recorded {
		if _, known := alreadyReleased[encoded]; known {
			continue
		}
		exact, state, err := prober.Probe(ref.Pid)
		if fixtureCustodianIdentityReleased(exact, state, err, ref) {
			released[encoded] = ref
		} else if err == nil && state == Alive && SameIdentity(exact, ref) && !exact.Zombie {
			live[encoded] = exact
		}
	}
	return released, live
}

func fixtureCustodianIdentityReleased(exact Exact, state Liveness, err error, ref Ref) bool {
	return err == nil && (state == Dead || state == Alive && (!SameIdentity(exact, ref) || exact.Zombie))
}

func fixtureCustodianRunningRecords(live map[string]Exact) map[string]Exact {
	running := make(map[string]Exact)
	for identity, exact := range live {
		if !exact.Exiting {
			running[identity] = exact
		}
	}
	return running
}

func sameFixtureCustodianRecordSet(left, right map[string]Exact) bool {
	if len(left) != len(right) {
		return false
	}
	for encoded := range left {
		if _, present := right[encoded]; !present {
			return false
		}
	}
	return true
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

// FixtureCustodianWatchLine returns the first line written after the custodian enters its owner-watch loop.
func FixtureCustodianWatchLine(owner Ref) string {
	ownerValue, _ := EncodeRef(owner)
	return fmt.Sprintf("fixture-custodian owner=%s action=watch\n", ownerValue)
}

func sameExactRef(left, right Ref) bool {
	return left.Pid == right.Pid && left.Mode() == right.Mode() && left.StartedAtUnixMicro == right.StartedAtUnixMicro &&
		left.StartTicks == right.StartTicks && left.BootID == right.BootID
}
