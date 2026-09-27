package delegation

// recordCAS is record_cas: a compare-and-swap through the internal entry
// (or directly under a stop batch's cancellation authority), a lifecycle
// event for a genuine pending or running transition, and the one-shot
// patch removed.
func (s *session) recordCAS(job, expect, target, patch string) (string, error) {
	var observed string
	var err error
	if s.stopCancelAuthorized != "" {
		observed, err = s.l.ports.Records.CAS(job, expect, target, patch)
		if observed != "" {
			s.println(observed)
		}
		if err != nil {
			err = s.verbFailure(err)
		}
	} else {
		observed, err = s.casEntry(job, expect, target, patch)
	}
	if err == nil && expect != target && (target == "pending" || target == "running") {
		s.emit("job-"+target, map[string]string{
			"jobId": job, "missionId": fieldOr(s.recordPath(job), "mission"),
			"summary": "transition " + expect + " -> " + target,
		})
	}
	removeQuietly(patch)
	return observed, err
}

// casEntry is the __record-cas callback as a call: the record-writer
// authority for the job, then the owner's compare-and-swap. The lost
// compare's observation is printed as the verb printed it.
func (s *session) casEntry(job, expect, target, patch string) (string, error) {
	if err := s.internalAuthority(AuthorityRecordWriter, job); err != nil {
		return "", err
	}
	observed, err := s.l.ports.Records.CAS(job, expect, target, patch)
	if observed != "" {
		s.println(observed)
	}
	if err != nil {
		return observed, s.verbFailure(err)
	}
	return observed, nil
}
