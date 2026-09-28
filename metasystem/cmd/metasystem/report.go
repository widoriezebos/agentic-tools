package main

import (
	"fmt"
	"os"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/hooks"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/report"
)

func runReportStopStatus(args []string) int {
	flags := newFlagSet("report stop-status")
	id := flags.String("id", "", "exact short Stop report alias or legacy full id")
	root := pathFlag(flags, "root", "", "explicit metasystem installation")
	if flags.Parse(args) != nil {
		return 2
	}
	if *id == "" || len(flags.Args()) != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem session status --id ID [--root INSTALLATION]")
		return 2
	}
	if err := report.ValidateStopStatusID(*id); err != nil {
		fmt.Fprintln(os.Stderr, "report stop-status:", err)
		return 2
	}
	data, _, err := report.ReadStopStatus(*root, *id)
	if err != nil {
		fmt.Fprintln(os.Stderr, "report stop-status:", err)
		return 1
	}
	if _, err := os.Stdout.Write(data); err != nil {
		fmt.Fprintln(os.Stderr, "report stop-status:", err)
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
