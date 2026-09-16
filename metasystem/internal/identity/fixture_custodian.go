package identity

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	FixtureCustodianEnv      = "METASYSTEM_FIXTURE_CUSTODIAN"
	FixtureCustodianOwnerEnv = "METASYSTEM_FIXTURE_CUSTODIAN_OWNER"
	FixtureCustodianLogEnv   = "METASYSTEM_FIXTURE_CUSTODIAN_LOG"
	FixtureCustodianStartEnv = "METASYSTEM_FIXTURE_CUSTODIAN_START"
	custodianPoll            = 250 * time.Millisecond
	custodianBound           = 5 * time.Second
	custodianHaltMargin      = time.Second
)

type custodianRuntime struct {
	prober Prober
	self   Ref
	sender SignalFunc
	poll   time.Duration
	bound  time.Duration
	halt   func(int)
}

// RunCustodian watches owner's pipe and reaps only children attributed to its exact dead identity.
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
	return runCustodian(owner, watch, log, custodianRuntime{
		prober: prober, self: exact.Ref(),
		poll: custodianPoll, bound: custodianBound, halt: os.Exit,
	})
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
		switch AliveRef(runtime.prober, owner) {
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

func reapDeadOwner(owner Ref, log io.Writer, runtime custodianRuntime) error {
	if runtime.halt != nil {
		hardHalt := time.AfterFunc(runtime.bound+custodianHaltMargin, func() { runtime.halt(2) })
		defer hardHalt.Stop()
	}
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
