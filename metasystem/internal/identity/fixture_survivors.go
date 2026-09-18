package identity

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

const FixtureOwnerEnv = "METASYSTEM_FIXTURE_OWNER"
const FixtureAttemptEnv = "METASYSTEM_FIXTURE_ATTEMPT"
const fixtureOwnerPrefix = FixtureOwnerEnv + "="

type FixtureCarrier string

const FixtureCarrierArgvWord FixtureCarrier = "argv-word"
const FixtureCarrierEnvironment FixtureCarrier = "environment"
const FixtureCarrierRecord FixtureCarrier = "record"

type FixtureSurvivorClass string

const (
	FixtureSurvivorCertain    FixtureSurvivorClass = "fixture-survivor"
	FixtureSurvivorUnreadable FixtureSurvivorClass = "fixture-survivor?"
	FixtureSurvivorUnowned    FixtureSurvivorClass = "unowned-in-cache"
)

type FixtureSurvivor struct {
	Class     FixtureSurvivorClass
	Ref       Ref
	Pgid      int64
	Ppid      int64
	Started   time.Time
	Exe       string
	ExeKnown  bool
	Argv      []string
	ArgvKnown bool
	Key       FixtureKey
	Carrier   FixtureCarrier
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

// FixtureProcessScope carries process-table facts that are outside an exact identity probe.
type FixtureProcessScope struct {
	Pgid, Sid  int64
	Ppid       int64
	Signalable bool
}

var fixtureSurvivorScope = func(pid int64) FixtureProcessScope {
	pgid, pgErr := unix.Getpgid(int(pid))
	sid, sidErr := unix.Getsid(int(pid))
	signalable := unix.Kill(int(pid), 0) == nil && pgErr == nil && sidErr == nil
	return FixtureProcessScope{Pgid: int64(pgid), Sid: int64(sid), Signalable: signalable}
}

// FixtureSurvivorSelection chooses one scan contract. Its zero value scans the whole process table.
type FixtureSurvivorSelection struct {
	Key   *FixtureKey
	Owner *Ref
}

// FixtureSurvivors returns processes for key, including unreadable processes only when their process group or session ties them to a certain result.
func FixtureSurvivors(key FixtureKey) ([]FixtureSurvivor, error) {
	pids, err := survivorPids()
	if err != nil {
		return nil, fmt.Errorf("identity: enumerate fixture processes: %w", err)
	}
	return ScanFixtureSurvivors(pids, fixtureSurvivorProber, fixtureSurvivorScope, FixtureSurvivorSelection{Key: &key})
}

// FixtureSurvivorsOfDeadOwner returns processes for owner's fixtures, including unreadable processes under go-tmp or tied to a certain result by process group or session.
func FixtureSurvivorsOfDeadOwner(prober Prober, owner Ref) ([]FixtureSurvivor, error) {
	pids, err := survivorPids()
	if err != nil {
		return nil, fmt.Errorf("identity: enumerate fixture processes: %w", err)
	}
	return ScanFixtureSurvivors(pids, prober, fixtureSurvivorScope, FixtureSurvivorSelection{Owner: &owner})
}

type fixtureObservation struct {
	exact   Exact
	scope   FixtureProcessScope
	key     FixtureKey
	carrier FixtureCarrier
}

// ScanFixtureSurvivors classifies fixture processes from exactly the supplied source.
func ScanFixtureSurvivors(pids []int64, prober Prober, scope func(int64) FixtureProcessScope, selection FixtureSurvivorSelection) ([]FixtureSurvivor, error) {
	if prober == nil || scope == nil || selection.Key != nil && selection.Owner != nil {
		return nil, fmt.Errorf("identity: invalid fixture survivor scan")
	}
	var wantedKey, wantedOwner string
	var err error
	if selection.Key != nil {
		wantedKey, err = EncodeKey(*selection.Key)
	}
	if selection.Owner != nil {
		wantedOwner, err = EncodeRef(*selection.Owner)
		if err == nil {
			switch fixtureOwnerLiveness(prober, *selection.Owner) {
			case Alive:
				return nil, fmt.Errorf("identity: fixture owner %s is alive", wantedOwner)
			case Unknown:
				return nil, fmt.Errorf("identity: fixture owner %s cannot be proved dead", wantedOwner)
			}
		}
	}
	if err != nil {
		return nil, fmt.Errorf("identity: fixture owner is not exactly inspectable")
	}
	matches := func(key FixtureKey) bool {
		if selection.Key != nil {
			encoded, encodeErr := EncodeKey(key)
			return encodeErr == nil && encoded == wantedKey
		}
		if selection.Owner != nil {
			encoded, encodeErr := EncodeRef(key.Owner)
			return encodeErr == nil && encoded == wantedOwner
		}
		return true
	}
	wholeTable := selection.Key == nil && selection.Owner == nil
	var certain, unreadable, unowned []fixtureObservation
	for _, pid := range pids {
		exact, state, probeErr := prober.Probe(pid)
		if probeErr != nil || state != Alive || !exact.Ref().NativeExact() || isFixtureCustodian(exact) {
			continue
		}
		observation := fixtureObservation{exact: exact, scope: scope(pid)}
		if key, carrier, ok := FixtureTag(exact); ok {
			if !matches(key) {
				continue
			}
			observation.key, observation.carrier = key, carrier
			if wholeTable {
				switch fixtureOwnerLiveness(prober, key.Owner) {
				case Alive:
					continue
				case Unknown:
					unreadable = append(unreadable, observation)
					continue
				}
			}
			certain = append(certain, observation)
			continue
		}
		if exact.ArgvKnown && containsFixtureTagWord(exact.Argv) || exact.EnvironKnown && containsFixtureTagWord(exact.Environ) {
			continue
		}
		if exact.ExeKnown {
			if key, ok := fixtureOwnershipRecord(exact.Exe); ok {
				if !matches(key) {
					continue
				}
				observation.key, observation.carrier = key, FixtureCarrierRecord
				if wholeTable {
					switch fixtureOwnerLiveness(prober, key.Owner) {
					case Alive:
						continue
					case Unknown:
						unreadable = append(unreadable, observation)
						continue
					}
				}
				certain = append(certain, observation)
				continue
			}
			if _, underGoTmp := goTmpRoot(exact.Exe); wholeTable && underGoTmp && exact.ArgvKnown && exact.EnvironKnown {
				unowned = append(unowned, observation)
				continue
			}
		}
		if !exact.ArgvKnown && !exact.EnvironKnown && observation.scope.Signalable {
			unreadable = append(unreadable, observation)
		}
	}
	var result []FixtureSurvivor
	for _, observation := range certain {
		result = append(result, observation.survivor(FixtureSurvivorCertain))
	}
	for _, observation := range unreadable {
		_, underGoTmp := goTmpRoot(observation.exact.Exe)
		if observation.key.Owner.Pid != 0 || selection.Key == nil && observation.exact.ExeKnown && underGoTmp || ledByFixtureSurvivor(observation, certain) {
			result = append(result, observation.survivor(FixtureSurvivorUnreadable))
		}
	}
	for _, observation := range unowned {
		result = append(result, observation.survivor(FixtureSurvivorUnowned))
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Ref.Pid < result[j].Ref.Pid })
	return result, nil
}

func fixtureOwnerLiveness(prober Prober, owner Ref) Liveness {
	exact, state, _ := prober.Probe(owner.Pid)
	if state != Alive {
		return state
	}
	comparison := Compare(exact, owner)
	if comparison.Mode == CompareInvalid {
		return Unknown
	}
	if !comparison.Matches || exact.Zombie {
		return Dead
	}
	return Alive
}

func containsFixtureTagWord(words []string) bool {
	for _, word := range words {
		if strings.HasPrefix(word, fixtureOwnerPrefix) {
			return true
		}
	}
	return false
}

func (observation fixtureObservation) survivor(class FixtureSurvivorClass) FixtureSurvivor {
	return FixtureSurvivor{
		Class: class, Ref: observation.exact.Ref(), Pgid: observation.scope.Pgid, Ppid: observation.scope.Ppid,
		Started: observation.exact.StartedAt,
		Exe:     observation.exact.Exe, ExeKnown: observation.exact.ExeKnown,
		Argv: observation.exact.Argv, ArgvKnown: observation.exact.ArgvKnown,
		Key: observation.key, Carrier: observation.carrier,
	}
}

func ledByFixtureSurvivor(observation fixtureObservation, certain []fixtureObservation) bool {
	for _, candidate := range certain {
		if SameIdentity(observation.exact, candidate.key.Owner) {
			continue
		}
		if observation.scope.Pgid > 1 && observation.scope.Pgid == candidate.exact.Pid ||
			observation.scope.Sid > 1 && observation.scope.Sid == candidate.exact.Pid {
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
	for {
		path := filepath.Join(directory, "fixture-owner")
		info, err := os.Lstat(path)
		if err == nil {
			if !info.Mode().IsRegular() || info.Size() > 4096 {
				return FixtureKey{}, false
			}
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return FixtureKey{}, false
			}
			key, parseErr := ParseKey(strings.TrimSpace(string(data)))
			topLevelTest, _, _ := strings.Cut(key.Test, "/")
			// Top-level test names are Go identifiers, so makeTempDir removes no other symbols.
			return key, parseErr == nil && strings.HasPrefix(filepath.Base(directory), strings.ToValidUTF8(topLevelTest[:min(len(topLevelTest), 64)], ""))
		}
		if !os.IsNotExist(err) {
			return FixtureKey{}, false
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return FixtureKey{}, false
		}
		directory = parent
	}
}
