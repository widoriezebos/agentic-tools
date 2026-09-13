package adapter

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// WaitDeliveryRequest is the complete adapter contract for holding one
// foreground wait in an already-established runtime session.
type WaitDeliveryRequest struct {
	WaitID   string
	Nonce    string
	Deadline time.Time
	Session  string
}

var ErrWaitDeliveryDeclined = errors.New("the runtime declined blocking wait delivery")

// DeliverWait asks an installed adapter whether its session can hold the
// foreground command. The adapter response is deliberately a one-word
// protocol so provider output can never become wait evidence.
func DeliverWait(ctx context.Context, adapterPath string, request WaitDeliveryRequest) (string, error) {
	if adapterPath == "" || request.WaitID == "" || request.Nonce == "" || request.Deadline.IsZero() || request.Session == "" {
		return "", fmt.Errorf("wait delivery requires an adapter, wait identifier, nonce, deadline, and session")
	}
	command := exec.CommandContext(ctx, adapterPath, "wait-delivery",
		"--wait-id", request.WaitID,
		"--nonce", request.Nonce,
		"--deadline", request.Deadline.UTC().Format(time.RFC3339Nano),
		"--session", request.Session,
	)
	output, err := command.Output()
	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) && exitError.ExitCode() == 2 {
			return "", ErrWaitDeliveryDeclined
		}
		if ctx.Err() != nil {
			return "", fmt.Errorf("wait delivery did not finish before its deadline: %w", ctx.Err())
		}
		return "", fmt.Errorf("wait delivery adapter failed: %w", err)
	}
	if string(output) != "blocking\n" {
		return "", fmt.Errorf("wait delivery adapter returned %q instead of one blocking line", strings.TrimSpace(string(output)))
	}
	return "blocking", nil
}
