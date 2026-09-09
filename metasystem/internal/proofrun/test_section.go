package proofrun

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

func parseSectionResult(path, selected string) (status string, exit int, blocked []NativeTestIdentity, complete bool) {
	file, err := os.Open(path)
	if err != nil {
		return "invalid", 0, nil, false
	}
	defer file.Close()
	count := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), "\t")
		if len(fields) < 4 || fields[0] != "section" || fields[1] != selected {
			continue
		}
		count++
		status = fields[2]
		parsedExit, parseErr := strconv.Atoi(fields[3])
		if parseErr != nil || parsedExit < 0 || parsedExit > 255 {
			return "invalid", 0, blocked, false
		}
		exit = parsedExit
		if status == "gated" {
			reason := "prerequisite-failed"
			if len(fields) > 4 && fields[4] != "" {
				reason = strings.Join(fields[4:], "\t")
			}
			blocked = append(blocked, NativeTestIdentity{Name: selected, Status: "blocked", Reason: reason})
		}
	}
	if count != 1 {
		return "invalid", 0, blocked, false
	}
	switch status {
	case "pass":
		if exit != 0 {
			return "invalid", exit, blocked, false
		}
		return "passed", exit, blocked, true
	case "fail":
		if exit == 0 {
			return "invalid", exit, blocked, false
		}
		return "failed", exit, blocked, true
	case "gated":
		return "unavailable", exit, blocked, false
	default:
		return "invalid", 0, blocked, false
	}
}
