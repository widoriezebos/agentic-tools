package dispatch

import (
	"fmt"
	"strings"
)

// CritiqueTransferClose closes a source only after its caller publishes the
// destination completion obligations. Transferred evidence never becomes a
// clean read.
func CritiqueTransferClose(repoRoot, rootJob string, findingIDs []string, stopRef string) error {
	if strings.TrimSpace(stopRef) == "" || len(findingIDs) == 0 {
		return fmt.Errorf("transfer needs findings and its recorded stop")
	}
	_, err := withFindingRegisterLock(repoRoot, func() (string, error) {
		err := withRecordLock(repoRoot, rootJob, func(path string) error {
			root, err := readObject(path)
			if err != nil {
				return err
			}
			register, present, err := critiqueFindingRegister(root)
			if err != nil {
				return err
			}
			if !present {
				return fmt.Errorf("critique %s has no finding register", rootJob)
			}
			wanted := map[string]bool{}
			for _, id := range findingIDs {
				wanted[id] = true
			}
			for i := range register {
				f := &register[i]
				if !wanted[f.FindingID] {
					continue
				}
				delete(wanted, f.FindingID)
				if f.Status == "transferred" {
					if f.TransferStop != stopRef {
						return fmt.Errorf("finding %s already belongs to another transfer", f.FindingID)
					}
					continue
				}
				if f.Status != "open" && f.Status != "disputed" {
					return fmt.Errorf("finding %s is already decided", f.FindingID)
				}
				f.Status, f.Resolution, f.TransferStop = "transferred", "transferred", stopRef
			}
			if len(wanted) != 0 {
				return fmt.Errorf("transfer names findings absent from critique %s", rootJob)
			}
			root[findingRegisterField] = encodeFindingRegister(register)
			if len(openRegisterFindingIDs(register)) == 0 {
				root["chainClosed"], root["chainCloseReason"] = true, "transferred"
				delete(root, closureField)
			}
			return writeRecord(path, root)
		})
		return "", err
	})
	return err
}
