package proofrun

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

const scriptFailureLogTailBytes int64 = 256 * 1024

func scriptFailureReason(logPath string, exit int) string {
	names, err := scriptFailedScenarioNames(logPath)
	if err != nil || len(names) == 0 {
		return fmt.Sprintf("process exit %d with no failed-scenarios block in the log", exit)
	}
	listed := names
	more := ""
	if len(listed) > 5 {
		listed = listed[:5]
		more = fmt.Sprintf(" and %d more", len(names)-len(listed))
	}
	return fmt.Sprintf("failed scenarios: %s%s (process exit %d)", strings.Join(listed, ", "), more, exit)
}

func scriptFailedScenarioNames(logPath string) ([]string, error) {
	file, err := os.Open(logPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	start := info.Size() - scriptFailureLogTailBytes
	if start < 0 {
		start = 0
	}
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		return nil, err
	}
	data, err := io.ReadAll(io.LimitReader(file, scriptFailureLogTailBytes))
	if err != nil {
		return nil, err
	}
	return parseScriptFailedScenarios(string(data)), nil
}

func parseScriptFailedScenarios(logTail string) []string {
	var bed string
	var names, completed []string
	for _, line := range strings.Split(logTail, "\n") {
		if bed == "" {
			const prefix, suffix = "=== ", " failed scenarios ==="
			if strings.HasPrefix(line, prefix) && strings.HasSuffix(line, suffix) {
				candidate := strings.TrimSuffix(strings.TrimPrefix(line, prefix), suffix)
				if candidate != "" && !strings.HasPrefix(candidate, "end ") {
					bed, names = candidate, nil
				}
			}
			continue
		}
		if line == "=== end "+bed+" failed scenarios ===" {
			completed = append([]string(nil), names...)
			bed, names = "", nil
			continue
		}
		if name, ok := scriptFailedScenarioName(line); ok {
			names = append(names, name)
		}
	}
	return completed
}

func scriptFailedScenarioName(line string) (string, bool) {
	const prefix, rcMarker = "- ", " (rc="
	if !strings.HasPrefix(line, prefix) || !strings.HasSuffix(line, ")") {
		return "", false
	}
	marker := strings.LastIndex(line, rcMarker)
	if marker <= len(prefix) {
		return "", false
	}
	rc, err := strconv.Atoi(line[marker+len(rcMarker) : len(line)-1])
	if err != nil || rc < 1 || rc > 255 {
		return "", false
	}
	return line[len(prefix):marker], true
}
