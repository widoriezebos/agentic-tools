package dispatch

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

// ValidateMission proves a mission has a live, matching lease before any job
// may join it: the lease must be the canonical file for the mission id, name
// exactly the expected fields, and its holder must pass a process identity
// proof — alive, in the recorded process group, with the recorded instance
// tag on its command line. Restricted sandboxes that hide the process table
// may substitute a trusted fixture, but only when the fake runtime is the
// sole configured runtime.
func ValidateMission(root, mission, leasePath string) error {
	if !validJobID.MatchString(mission) {
		return fmt.Errorf("invalid mission id")
	}
	rootResolved := resolvePath(root)
	expected := resolvePath(filepath.Join(rootResolved, "artifacts", "agents", "missions", mission, "lease.json"))
	if resolvePath(leasePath) != expected {
		return fmt.Errorf("mission lease path is ambiguous or non-canonical")
	}
	lease, err := readPlainObject(expected)
	if err != nil {
		return fmt.Errorf("mission has no readable live lease: %v", err)
	}
	required := map[string]bool{
		"missionId": true, "pid": true, "pgid": true,
		"instanceTag": true, "startedAt": true, "renewedAt": true,
	}
	if len(lease) != len(required) {
		return fmt.Errorf("mission lease has an invalid shape or identity")
	}
	for key := range required {
		if _, present := lease[key]; !present {
			return fmt.Errorf("mission lease has an invalid shape or identity")
		}
	}
	if asString(lease["missionId"]) != mission {
		return fmt.Errorf("mission lease has an invalid shape or identity")
	}
	pid, pidOK := numInt(lease["pid"])
	pgid, pgidOK := numInt(lease["pgid"])
	tag := asString(lease["instanceTag"])
	if !pidOK || !pgidOK || tag == "" {
		return fmt.Errorf("mission lease has invalid ownership fields")
	}
	if err := unix.Kill(int(pid), 0); err != nil {
		return fmt.Errorf("mission lease holder is not alive")
	}
	actualPgid, err := unix.Getpgid(int(pid))
	if err != nil {
		return fmt.Errorf("mission lease holder is not alive")
	}
	command, err := processCommand(pid)
	if err != nil {
		// The trusted fixture substitutes for a hidden process table, but only
		// in a checkout whose sole runtime is fake — it can never weaken a real
		// runtime's identity proof.
		command, err = fixtureCommand(rootResolved, pid, int64(actualPgid))
		if err != nil {
			return err
		}
	}
	if !strings.Contains(command, tag) || int64(actualPgid) != pgid {
		return fmt.Errorf("mission lease holder failed process identity proof")
	}
	return nil
}

// processCommand reads a pid's full command line from the process table.
func processCommand(pid int64) (string, error) {
	out, err := exec.Command("ps", "-p", strconv.FormatInt(pid, 10), "-o", "command=").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// fixtureCommand resolves a pid's command line from the fake-runtime identity
// fixture, verifying the fixture agrees with the kernel about the process
// group.
func fixtureCommand(root string, pid, actualPgid int64) (string, error) {
	fixturePath := os.Getenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE")
	configured := ""
	conf, err := os.ReadFile(filepath.Join(root, "metasystem.conf"))
	if err != nil {
		return "", fmt.Errorf("mission process command line could not be verified")
	}
	for _, raw := range strings.Split(string(conf), "\n") {
		if strings.HasPrefix(raw, "metasystem.runtimes=") {
			configured = strings.TrimSpace(strings.SplitN(raw, "=", 2)[1])
		}
	}
	if configured != "fake" || fixturePath == "" {
		return "", fmt.Errorf("mission process command line could not be verified")
	}
	data, err := os.ReadFile(fixturePath)
	if err != nil {
		return "", fmt.Errorf("fake mission process identity fixture is invalid")
	}
	var fixture map[string]map[string]any
	if json.Unmarshal(data, &fixture) != nil {
		return "", fmt.Errorf("fake mission process identity fixture is invalid")
	}
	identity, present := fixture[strconv.FormatInt(pid, 10)]
	if !present {
		return "", fmt.Errorf("fake mission process identity fixture is invalid")
	}
	command, commandOK := identity["command"].(string)
	fixturePgid, pgidOK := numInt(identity["pgid"])
	if !commandOK || !pgidOK || fixturePgid != actualPgid {
		return "", fmt.Errorf("fake mission process identity fixture is invalid")
	}
	return command, nil
}
