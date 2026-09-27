package adapter

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
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

var rfc3339UTC = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]+)?Z$`)

// WaitDeliveryAccepted is every runtime adapter's wait-delivery answer: a
// complete request (wait id, nonce, session, and an RFC 3339 UTC deadline)
// is held by the runtime's blocking foreground command.
func WaitDeliveryAccepted(waitID, nonce, deadline, session string) bool {
	return waitID != "" && nonce != "" && session != "" && rfc3339UTC.MatchString(deadline)
}

// DeliverWait asks a runtime's adapter whether its session can hold the
// foreground command. The answer is deliberately a one-word protocol so
// provider output can never become wait evidence.
func DeliverWait(ctx context.Context, runtime string, request WaitDeliveryRequest) (string, error) {
	return DeliverWaitAt(ctx, "", runtime, request)
}

// DeliverWaitAt is DeliverWait for an installation root (the root an
// external adapter's answer will come from, U6c).
func DeliverWaitAt(_ context.Context, _, runtime string, request WaitDeliveryRequest) (string, error) {
	if runtime == "" || request.WaitID == "" || request.Nonce == "" || request.Deadline.IsZero() || request.Session == "" {
		return "", fmt.Errorf("wait delivery requires an adapter, wait identifier, nonce, deadline, and session")
	}
	deadline := request.Deadline.UTC().Format(time.RFC3339Nano)
	if !WaitDeliveryAccepted(request.WaitID, request.Nonce, deadline, request.Session) {
		return "", ErrWaitDeliveryDeclined
	}
	if declaration, ok := runtimes.Lookup(runtime); !ok || !declaration.HasAdapter {
		return "", fmt.Errorf("wait delivery adapter %s is unavailable", runtime)
	}
	return "blocking", nil
}
