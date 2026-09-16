package identity

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/sys/unix"
)

const FixtureOwnerEnv = "METASYSTEM_FIXTURE_OWNER"
const fixtureOwnerPrefix = FixtureOwnerEnv + "="

type FixtureCarrier string

const FixtureCarrierArgvWord FixtureCarrier = "argv-word"
const FixtureCarrierEnvironment FixtureCarrier = "environment"

type FixtureSurvivorClass string

const (
	FixtureSurvivorCertain    FixtureSurvivorClass = "fixture-survivor"
	FixtureSurvivorUnreadable FixtureSurvivorClass = "fixture-survivor?"
	FixtureSurvivorUnowned    FixtureSurvivorClass = "unowned-in-cache"
)

type FixtureSurvivor struct {
	Class   FixtureSurvivorClass
	Ref     Ref
	Pgid    int64
	Exe     string
	Argv    []string
	Key     FixtureKey
	Carrier FixtureCarrier
}

func FixtureTag(exact Exact) (FixtureKey, FixtureCarrier, bool) {
	carriers := []FixtureCarrier{FixtureCarrierArgvWord, FixtureCarrierEnvironment}
	for index, words := range [][]string{exact.Argv, exact.Environ} {
		if index == 0 && !exact.ArgvKnown || index == 1 && !exact.EnvironKnown {
			continue
		}
		for _, word := range words {
			if !strings.HasPrefix(word, fixtureOwnerPrefix) {
				continue
			}
			key, err := ParseKey(strings.TrimPrefix(word, fixtureOwnerPrefix))
			if err == nil {
				return key, carriers[index], true
			}
		}
	}
	return FixtureKey{}, "", false
}

var fixtureSurvivorProber Prober = KernelProber{}

type fixtureProcessScope struct {
	pgid, sid  int64
	signalable bool
}

var fixtureSurvivorScope = func(pid int64) fixtureProcessScope {
	pgid, pgErr := unix.Getpgid(int(pid))
	sid, sidErr := unix.Getsid(int(pid))
	signalable := unix.Kill(int(pid), 0) == nil && pgErr == nil && sidErr == nil
	return fixtureProcessScope{int64(pgid), int64(sid), signalable}
}

func FixtureSurvivors(key FixtureKey) ([]FixtureSurvivor, error) {
	wanted, err := EncodeKey(key)
	if err != nil {
		return nil, err
	}
	return scanFixtureSurvivors(fixtureSurvivorProber, func(candidate FixtureKey) bool {
		encoded, encodeErr := EncodeKey(candidate)
		return encodeErr == nil && encoded == wanted
	})
}

func FixtureSurvivorsOfDeadOwner(prober Prober, owner Ref) ([]FixtureSurvivor, error) {
	wanted, err := EncodeRef(owner)
	if prober == nil || err != nil {
		return nil, fmt.Errorf("identity: fixture owner is not exactly inspectable")
	}
	switch AliveRef(prober, owner) {
	case Alive:
		return nil, fmt.Errorf("identity: fixture owner %d is alive", owner.Pid)
	case Unknown:
		return nil, fmt.Errorf("identity: fixture owner %d cannot be proved dead", owner.Pid)
	}
	return scanFixtureSurvivors(prober, func(candidate FixtureKey) bool {
		encoded, encodeErr := EncodeRef(candidate.Owner)
		return encodeErr == nil && encoded == wanted
	})
}

type fixtureObservation struct {
	exact   Exact
	scope   fixtureProcessScope
	key     FixtureKey
	carrier FixtureCarrier
}

func scanFixtureSurvivors(prober Prober, matches func(FixtureKey) bool) ([]FixtureSurvivor, error) {
	pids, err := survivorPids()
	if err != nil {
		return nil, fmt.Errorf("identity: enumerate fixture processes: %w", err)
	}
	var certain, unreadable []fixtureObservation
	for _, pid := range pids {
		exact, state, probeErr := prober.Probe(pid)
		if probeErr != nil || state != Alive || !exact.Ref().NativeExact() {
			continue
		}
		observation := fixtureObservation{exact: exact, scope: fixtureSurvivorScope(pid)}
		if key, carrier, ok := FixtureTag(exact); ok {
			if matches(key) {
				observation.key, observation.carrier = key, carrier
				certain = append(certain, observation)
			}
			continue
		}
		if exact.ExeKnown {
			if key, ok := fixtureOwnershipRecord(exact.Exe); ok && matches(key) {
				observation.key = key
				certain = append(certain, observation)
				continue
			}
		}
		if !exact.ArgvKnown && !exact.EnvironKnown && observation.scope.signalable {
			unreadable = append(unreadable, observation)
		}
	}
	var result []FixtureSurvivor
	for _, observation := range certain {
		result = append(result, observation.survivor(FixtureSurvivorCertain))
	}
	for _, observation := range unreadable {
		_, underGoTmp := goTmpRoot(observation.exact.Exe)
		if observation.exact.ExeKnown && underGoTmp || sharesFixtureScope(observation, certain) {
			result = append(result, observation.survivor(FixtureSurvivorUnreadable))
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Ref.Pid < result[j].Ref.Pid })
	return result, nil
}

func (observation fixtureObservation) survivor(class FixtureSurvivorClass) FixtureSurvivor {
	return FixtureSurvivor{
		Class: class, Ref: observation.exact.Ref(), Pgid: observation.scope.pgid,
		Exe: observation.exact.Exe, Argv: observation.exact.Argv, Key: observation.key, Carrier: observation.carrier,
	}
}

func sharesFixtureScope(observation fixtureObservation, certain []fixtureObservation) bool {
	for _, candidate := range certain {
		if SameIdentity(observation.exact, candidate.key.Owner) {
			continue
		}
		if observation.scope.pgid > 1 && observation.scope.pgid == candidate.scope.pgid ||
			observation.scope.sid > 1 && observation.scope.sid == candidate.scope.sid {
			return true
		}
	}
	return false
}

func goTmpRoot(executable string) (string, bool) {
	cacheRoot := filepath.Dir(filepath.Clean(executable))
	for filepath.Base(cacheRoot) != "go-tmp" {
		parent := filepath.Dir(cacheRoot)
		if parent == cacheRoot {
			return "", false
		}
		cacheRoot = parent
	}
	return cacheRoot, true
}

func fixtureOwnershipRecord(executable string) (FixtureKey, bool) {
	directory := filepath.Dir(filepath.Clean(executable))
	cacheRoot, ok := goTmpRoot(executable)
	if !ok {
		return FixtureKey{}, false
	}
	for directory != cacheRoot {
		data, err := os.ReadFile(filepath.Join(directory, "fixture-owner"))
		if err == nil {
			key, parseErr := ParseKey(strings.TrimSpace(string(data)))
			return key, parseErr == nil
		}
		if !os.IsNotExist(err) {
			return FixtureKey{}, false
		}
		directory = filepath.Dir(directory)
	}
	return FixtureKey{}, false
}
