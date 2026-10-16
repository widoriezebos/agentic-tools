package plain

import (
	"errors"
	"time"
)

// LaunchRecord belongs to the record of the act requesting the process.
type LaunchRecord struct {
	Owner   string `json:"owner,omitempty"`
	Attempt string `json:"attempt,omitempty"`
	Job     string `json:"job,omitempty"`
	What    string `json:"what"`
	At      string `json:"at"`
	By      string `json:"by"`
	Error   string `json:"error,omitempty"`
}

// LaunchResolution keeps the request before launching and the job only after success.
func LaunchResolution(install string, fix *Fix, now time.Time, by string, start func() (string, error)) error {
	return withLock(install, func() error {
		jobID := ""
		if fix.Launch != nil {
			jobID = fix.Launch.Job
		}
		at := now.UTC().Format(time.RFC3339Nano)
		if fix.Launch != nil && fix.Launch.At != "" {
			at = fix.Launch.At
		}
		fix.Launch = &LaunchRecord{Attempt: fix.Attempt, Job: jobID, What: "resolution", At: at, By: by}
		fix.At = fix.Launch.At
		if err := WriteFix(install, fix); err != nil {
			return err
		}
		job, err := start()
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
