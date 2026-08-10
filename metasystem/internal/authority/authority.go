// Package authority applies the control-plane authority matrix to one
// classified caller: given a write mode and a caller's classification, it
// decides whether the write is permitted, returning a typed refusal naming
// why when it is not.
package authority

import "fmt"

// Modes are the control-plane write modes the matrix understands.
var Modes = map[string]bool{
	"holder-only":      true,
	"record-writer":    true,
	"adapter-writer":   true,
	"supervision-only": true,
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

	if mode == "holder-only" {
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
