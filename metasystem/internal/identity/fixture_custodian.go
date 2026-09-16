package identity

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

const (
	FixtureCustodianEnv      = "METASYSTEM_FIXTURE_CUSTODIAN"
	FixtureCustodianOwnerEnv = "METASYSTEM_FIXTURE_CUSTODIAN_OWNER"
	FixtureCustodianLogEnv   = "METASYSTEM_FIXTURE_CUSTODIAN_LOG"
	FixtureCustodianStartEnv = "METASYSTEM_FIXTURE_CUSTODIAN_START"
	FixtureCustodianChainEnv = "METASYSTEM_FIXTURE_CUSTODIAN_CHAIN"
	RunOwnerEnv              = "METASYSTEM_RUN_OWNER"
	custodianPoll            = 250 * time.Millisecond
	custodianBound           = 5 * time.Second
	custodianHaltMargin      = time.Second
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
	prober Prober
	self   Ref
	chain  []Ref
	sender SignalFunc
	poll   time.Duration
	bound  time.Duration
	halt   func(int)
}

// RunCustodian watches the owner and its launcher chain, kills the owner after launcher loss, and reaps its attributed children.
func RunCustodian(owner Ref, watch io.Reader, log io.Writer) error {
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
	return runCustodian(owner, watch, log, custodianRuntime{
		prober: prober, self: exact.Ref(), chain: chain,
		poll: custodianPoll, bound: custodianBound, halt: os.Exit,
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
		stop := armCustodianHalt(runtime)
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
			if dead, err := proveOwnerDead(owner, runtime); err != nil {
				return err
			} else if dead {
				return reapDeadOwner(owner, log, runtime)
			}
		}
	}
}

func proveOwnerDead(owner Ref, runtime custodianRuntime) (bool, error) {
	stop := armCustodianHalt(runtime)
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
	stop := armCustodianHalt(runtime)
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

func armCustodianHalt(runtime custodianRuntime) func() {
	if runtime.halt == nil {
		return func() {}
	}
	timer := time.AfterFunc(runtime.bound+custodianHaltMargin, func() { runtime.halt(2) })
	return func() { timer.Stop() }
}

func reapDeadOwner(owner Ref, log io.Writer, runtime custodianRuntime) error {
	stop := armCustodianHalt(runtime)
	defer stop()
	deadline := time.Now().Add(runtime.bound)
	quietWindow := time.Second + runtime.poll
	if halfBound := runtime.bound / 2; quietWindow > halfBound {
		quietWindow = halfBound
	}
	ownerValue, _ := EncodeRef(owner)
	var quietSince time.Time
	for {
		survivors, err := FixtureSurvivorsOfDeadOwner(runtime.prober, owner)
		if err == nil {
			actionable := 0
			for _, survivor := range survivors {
				if survivor.Class != FixtureSurvivorCertain || sameExactRef(survivor.Ref, runtime.self) {
					continue
				}
				actionable++
				signalErr := SignalExact(runtime.prober, survivor.Ref, syscall.SIGKILL, runtime.sender)
				fmt.Fprintf(log, "fixture-custodian action=kill pid=%d carrier=%s result=%v\n", survivor.Ref.Pid, survivor.Carrier, signalErr)
			}
			if actionable == 0 {
				if quietSince.IsZero() {
					quietSince = time.Now()
				}
				if time.Since(quietSince) >= quietWindow {
					fmt.Fprintf(log, "fixture-custodian owner=%s action=complete\n", ownerValue)
					return nil
				}
			} else {
				quietSince = time.Time{}
			}
		} else {
			quietSince = time.Time{}
		}
		if !time.Now().Before(deadline) {
			return fmt.Errorf("identity: fixture custodian cleanup exceeded %s: %v", runtime.bound, err)
		}
		time.Sleep(runtime.poll)
	}
}
func sameExactRef(left, right Ref) bool {
	return left.Pid == right.Pid && left.Mode() == right.Mode() && left.StartedAtUnixMicro == right.StartedAtUnixMicro &&
		left.StartTicks == right.StartTicks && left.BootID == right.BootID
}
