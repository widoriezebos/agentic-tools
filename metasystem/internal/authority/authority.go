// Package authority applies the control-plane authority matrix to one
// classified caller: given a write mode and a caller's classification, it
// decides whether the write is permitted, returning a refusal error
// naming why when it is not.
package authority

import "fmt"

// ValidMode reports whether name is a control-plane write mode the
// matrix understands. A function, not an exported map: the mode set is
// closed and importers must not be able to mutate it.
func ValidMode(name string) bool {
	switch name {
	case "holder-only", "record-writer", "adapter-writer", "supervision-only", "genesis":
		return true
	}
	return false
}

// Authorize reports whether a caller with the given classification may perform
// a control-plane write in the given mode. job is the job a record-mutating
// call names, matched against an adapter supervisor's custody. A nil return
// means permitted; a non-nil error is the refusal.
func Authorize(mode string, classification map[string]any, job string) error {
	class, _ := classification["class"].(string)
	holder, _ := classification["holder"].(bool)

	switch {
	case class == "HUMAN":
		return nil
	case class == "MAIN" && holder:
		if mode == "supervision-only" {
			return fmt.Errorf("standing reap requires authenticated supervision custody")
		}
		return nil
	}

	if mode == "genesis" {
		// Genesis baselines a ledger in a root with no accepted
		// baseline. The human and the holder are admitted above.
		// Anyone else — a main without the lease, a delegate, a
		// helper — is admitted only when the ledger it would baseline
		// is adoption-shaped: goal-free, on a checkout whose history
		// carries no ledger (the verb layer computes the flag from the
		// root; the store re-judges it under its lock). That is the
		// whole of what adoption states, and it is what lets every
		// provisioning caller — a terminal, an announced session, a
		// session whose announcement lapsed, a fixture under agent
		// ancestry, the kit gate in a delegate sandbox — seed a new
		// control plane, while nobody but the holder puts intent into
		// one that exists.
		if shaped, _ := classification["adoptionShaped"].(bool); shaped {
			return nil
		}
		return fmt.Errorf("genesis admits a non-holder only for a goal-free ledger on a checkout whose history carries none")
	}

	if mode == "holder-only" {
		// The steward's one admission: launching the unattended
		// continuation job its consumed authorization names. A dead
		// worker holds no lease, and the steward's authority extends
		// to exactly this job — every other holder-only write refuses.
		if class == "STEWARD" {
			if stewardJob, _ := classification["stewardJob"].(string); stewardJob != "" && job != "" && stewardJob == job {
				return nil
			}
			return fmt.Errorf("the steward may launch only the continuation job its consumed authorization names")
		}
		return fmt.Errorf("control-plane write requires the authenticated lease holder")
	}

	switch class {
	case "SUPERVISION":
		if mode != "record-writer" && mode != "supervision-only" {
			return fmt.Errorf("supervision may write only its state, census, and standing-reaper transitions")
		}
		return nil
	case "ADAPTER-SUPERVISOR":
		if mode != "record-writer" && mode != "adapter-writer" {
			return fmt.Errorf("adapter supervisor is outside this control-plane authority")
		}
		jobID, _ := classification["jobId"].(string)
		if job == "" || jobID != job {
			return fmt.Errorf("adapter supervisor may mutate only the job named by its custody record")
		}
		return nil
	}
	return fmt.Errorf("control-plane write refused for caller class %v", classification["class"])
}
