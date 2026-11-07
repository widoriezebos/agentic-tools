package plain

import (
	"errors"
	"time"
)

// LaunchRecord belongs to the record of the act requesting the process.
type LaunchRecord struct {
	Person *ActProvenance `json:"person,omitempty"`
	Reason string         `json:"reason,omitempty"`
	Impact string         `json:"impact,omitempty"`

	Owner   string `json:"owner,omitempty"`
	Attempt string `json:"attempt,omitempty"`
	Job     string `json:"job,omitempty"`
	What    string `json:"what"`
	At      string `json:"at"`
	By      string `json:"by"`
	Error   string `json:"error,omitempty"`
}

// LaunchResolution joins an existing job or records the request before starting one, under the lane lock.
func LaunchResolution(install string, fix *Fix, now time.Time, request LaunchRecord, existing func() (string, bool, error), start func() (string, error)) error {
	return withLock(install, func() error {
		current, err := FixForAttempt(install, fix.Attempt)
		if err != nil {
			return err
		}
		if current != nil && current.Job != "" {
			*fix = *current
			return nil
		}
		job, found, err := existing()
		if err != nil {
			return err
		}
		if found {
			if current != nil {
				*fix = *current
			} else {
				fix.Launch = nil
			}
			if fix.Launch == nil {
				fix.Launch = &LaunchRecord{What: "resolution"}
			}
			if fix.Launch.Attempt == "" {
				fix.Launch.Attempt = fix.Attempt
			}
			if fix.Launch.Job == "" {
				fix.Launch.Job = job
			}
			fix.Job = job
			return WriteFix(install, fix)
		}
		jobID := ""
		if fix.Launch != nil {
			jobID = fix.Launch.Job
		}
		at := now.UTC().Format(time.RFC3339Nano)
		if fix.Launch != nil && fix.Launch.At != "" {
			at = fix.Launch.At
		}
		request.Attempt, request.Job, request.What, request.At = fix.Attempt, jobID, "resolution", at
		fix.Launch = &request
		fix.At = fix.Launch.At
		if err := WriteFix(install, fix); err != nil {
			return err
		}
		job, err = start()
		if err != nil {
			fix.Launch.Error = err.Error()
		} else {
			fix.Job = job
		}
		return errors.Join(err, WriteFix(install, fix))
	})
}

func launchBy(person *ActProvenance) string {
	if person != nil {
		return person.Person
	}
	return "landing"
}
