package dispatch

import (
	"fmt"
	"os"
	"time"
)

type CensusWaitObservation struct {
	Fresh      bool
	Generation int64
	ScanSeq    int64
	Interval   time.Duration
}

type CensusWaitCursor struct {
	PostGeneration  int64
	PostScanSeq     int64
	Marker          int64
	Initialized     bool
	CompletedPasses int
	Regressions     int
}

type CensusWaitOptions struct {
	VerdictPath         string
	StatePath           string
	ArmHint             string
	RepoHint            string
	ExpectedFingerprint string
	PostGeneration      int64
	PostScanSeq         int64
	AttemptBudget       int
	MaxRegressions      int
	PollInterval        time.Duration
	Observe             func(CensusWaitMeasurement)
}

// CensusWaitMeasurement counts completed passes observed after this caller
// actually blocked. A successful freshness lookup is not a blocking pass.
type CensusWaitMeasurement struct {
	StartedAt            string `json:"startedAt"`
	EndedAt              string `json:"endedAt"`
	DurationNS           int64  `json:"durationNs"`
	Polls                int    `json:"polls"`
	BlockingPasses       int    `json:"blockingPasses"`
	Regressions          int    `json:"regressions"`
	ObservationsComplete bool   `json:"observationsComplete"`
}

func readCensusWaitObservation(path string) (CensusWaitObservation, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return CensusWaitObservation{}, err
	}
	decoded, err := decodeJSONValue(data)
	if err != nil {
		return CensusWaitObservation{}, err
	}
	value, ok := decoded.(map[string]any)
	if !ok {
		return CensusWaitObservation{}, fmt.Errorf("census wait observed a non-object verdict")
	}
	generation, generationOK := numInt(value["generation"])
	sequence, sequenceOK := numInt(value["scanSeq"])
	interval, intervalOK := numInt(value["intervalSec"])
	if !generationOK || generation < 1 || !sequenceOK || sequence < 0 || !intervalOK || interval < 1 {
		return CensusWaitObservation{}, fmt.Errorf("census wait observed invalid generation, scanSeq, or intervalSec")
	}
	return CensusWaitObservation{Generation: generation, ScanSeq: sequence, Interval: time.Duration(interval) * time.Second}, nil
}

// ObserveCensusWait owns the distinction between ordinary freshness and an
// explicit post-event wait. Ordinary callers return on the first production-
// fresh observation. A transition witness additionally requires a completed
// scan after the caller's captured generation/sequence.
func ObserveCensusWait(cursor CensusWaitCursor, observation CensusWaitObservation, attemptBudget, maxRegressions int) (CensusWaitCursor, bool, error) {
	if attemptBudget < 1 || maxRegressions < 0 || observation.Generation < 1 || observation.ScanSeq < 0 {
		return cursor, false, fmt.Errorf("census wait inputs are invalid")
	}
	if !cursor.Initialized {
		cursor.Marker = observation.ScanSeq
		cursor.Initialized = true
	}
	if observation.ScanSeq < cursor.Marker {
		cursor.Regressions++
		cursor.Marker = observation.ScanSeq
		if cursor.Regressions > maxRegressions {
			return cursor, false, fmt.Errorf("census scanSeq regressed %d times", cursor.Regressions)
		}
	} else if observation.ScanSeq > cursor.Marker {
		cursor.CompletedPasses += int(observation.ScanSeq - cursor.Marker)
		cursor.Marker = observation.ScanSeq
	}
	if observation.Fresh {
		if cursor.PostGeneration == 0 || observation.Generation != cursor.PostGeneration || observation.ScanSeq > cursor.PostScanSeq {
			return cursor, true, nil
		}
	}
	if cursor.CompletedPasses >= attemptBudget {
		return cursor, false, fmt.Errorf("census remained unsatisfied after %d completed passes", cursor.CompletedPasses)
	}
	return cursor, false, nil
}

func WaitForCensus(options CensusWaitOptions) error {
	if options.VerdictPath == "" || options.StatePath == "" || options.AttemptBudget < 1 || options.MaxRegressions < 0 || options.PollInterval <= 0 ||
		(options.PostGeneration == 0 && options.PostScanSeq != 0) {
		return fmt.Errorf("census wait options are incomplete")
	}
	cursor := CensusWaitCursor{PostGeneration: options.PostGeneration, PostScanSeq: options.PostScanSeq}
	started := time.Now()
	polls := 0
	observationsComplete := true
	blockingPasses := 0
	lastGeneration, lastSequence := int64(0), int64(0)
	defer func() {
		if options.Observe != nil {
			ended := time.Now()
			options.Observe(CensusWaitMeasurement{StartedAt: started.UTC().Format(time.RFC3339Nano), EndedAt: ended.UTC().Format(time.RFC3339Nano),
				DurationNS: ended.Sub(started).Nanoseconds(), Polls: polls, BlockingPasses: blockingPasses, Regressions: cursor.Regressions,
				ObservationsComplete: observationsComplete})
		}
	}()
	lastAdvance := time.Now()
	lastMarker := int64(-1)
	for {
		fresh := CensusFresh(options.VerdictPath, options.StatePath, options.ArmHint, options.RepoHint, options.ExpectedFingerprint, time.Now()) == nil
		observation, readErr := readCensusWaitObservation(options.VerdictPath)
		if readErr == nil {
			if polls > 0 && lastGeneration != 0 {
				if observation.Generation != lastGeneration {
					blockingPasses++
				} else if observation.ScanSeq > lastSequence {
					blockingPasses += int(observation.ScanSeq - lastSequence)
				}
			}
			lastGeneration, lastSequence = observation.Generation, observation.ScanSeq
			observation.Fresh = fresh
			if observation.ScanSeq != lastMarker {
				lastMarker = observation.ScanSeq
				lastAdvance = time.Now()
			}
			var done bool
			var decisionErr error
			cursor, done, decisionErr = ObserveCensusWait(cursor, observation, options.AttemptBudget, options.MaxRegressions)
			if done {
				return nil
			}
			if decisionErr != nil {
				return decisionErr
			}
			silence := 30 * observation.Interval
			if silence < time.Minute {
				silence = time.Minute
			}
			if time.Since(lastAdvance) >= silence {
				return fmt.Errorf("no completed census pass for %s (scanSeq stuck at %d)", silence, lastMarker)
			}
		} else {
			observationsComplete = false
			if time.Since(lastAdvance) >= time.Minute {
				return fmt.Errorf("no readable completed census pass for 1m: %w", readErr)
			}
		}
		polls++
		time.Sleep(options.PollInterval)
	}
}
