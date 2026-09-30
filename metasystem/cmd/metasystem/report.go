package main

import (
	"fmt"
	"io"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/hooks"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/report"
)

func runReportStopStatus(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("session status", stdout, stderr)
	id := flags.String("id", "", "exact short Stop report alias or legacy full id")
	root := pathFlag(flags, "root", "", "explicit metasystem installation")
	if flags.Parse(args) != nil {
		return 2
	}
	if *id == "" || len(flags.Args()) != 0 {
		fmt.Fprintln(stderr, "session status needs the id from a Stop line, so nothing was read")
		fmt.Fprintln(stderr, "run: metasystem session status --id <the id on the Stop line>")
		return 2
	}
	if err := report.ValidateStopStatusID(*id); err != nil {
		fmt.Fprintf(stderr, "%s is not a Stop report id (those are lowercase hexadecimal); nothing was read\n", *id)
		fmt.Fprintln(stderr, "run: metasystem session status --id <the id on the Stop line>")
		return 2
	}
	data, _, err := report.ReadStopStatus(*root, *id)
	if err != nil {
		fmt.Fprintf(stderr, "Stop report %s could not be read: %v\n", *id, err)
		fmt.Fprintln(stderr, "run: metasystem system check  (names what is wrong here)")
		return 1
	}
	if _, err := stdout.Write(data); err != nil {
		fmt.Fprintf(stderr, "Stop report %s could not be written out: %v\n", *id, err)
		return 1
	}
	return 0
}

// renderStopBlock renders one Stop refusal: an unrecorded block (bounded idle
// or the open-work form) with an optional system message, or, when the
// request names a record, the recorded refusal that owns the occurrence
// count. An open-work root first marks its open-work lines durably seen.
func renderStopBlock(request hooks.StopBlockRequest, now time.Time) (map[string]any, int, error) {
	class := request.Class
	if class == "" {
		class = string(report.StopClassSeatActionable)
	}
	if class != string(report.StopClassInfrastructure) && class != string(report.StopClassSeatActionable) {
		return nil, 2, fmt.Errorf("report stop-block: the class must be infrastructure or seat-actionable")
	}
	systemMessage := request.SystemMessage
	if request.OpenWorkRoot != "" {
		if warning := report.OpenWorkSeenWarning(request.OpenWorkRoot); warning != "" {
			if systemMessage != "" {
				systemMessage += "\n"
			}
			systemMessage += warning
		}
	}
	if request.ArmingResult != "" {
		if systemMessage != "" {
			systemMessage += "\n"
		}
		systemMessage += request.ArmingResult
	}
	systemMessage = report.BoundSystemMessage(systemMessage)
	if request.RefusalRecord != "" || request.Session != "" || request.Cause != "" || request.Remedy != "" {
		if request.RefusalRecord == "" || request.Session == "" || request.Cause == "" || request.Remedy == "" {
			return nil, 2, fmt.Errorf("report stop-block: the refusal record, session, cause, and remedy must be provided together")
		}
		block, err := report.StopRefusal(request.RefusalRecord, request.Session, request.Cause, request.Remedy, request.Detail, systemMessage, report.StopClass(class), now)
		if err != nil {
			return nil, 1, fmt.Errorf("report stop-block: %v", err)
		}
		return block, 0, nil
	}
	var block map[string]any
	if request.BoundedIdle {
		block = report.BoundedIdleStopBlock(request.Detail)
	} else {
		block = report.StopBlock(request.Detail)
	}
	if systemMessage != "" {
		block["systemMessage"] = systemMessage
	}
	return block, 0, nil
}
