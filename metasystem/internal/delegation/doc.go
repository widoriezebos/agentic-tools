// Package delegation is the delegate lifecycle: the dispatch composition
// package of the object-action verb redesign (plans/designs/verbs-object-action.md,
// section 6.3, finding VOA-07; units U6-seam and U6b).
//
// internal/lease and internal/steward import internal/dispatch, so the
// lifecycle cannot live in internal/dispatch. It lives here, above its
// owners: this package imports dispatch, lease, steward, adapter and goal,
// and nothing under internal/ imports it; only cmd/ does. Rule R9 holds that
// line; its witness is internal/layering.
//
// Lifecycle.Run takes the retired scripts/agents/dispatch.sh argv grammar
// (dispatch, follow-up, watch, status, cancel, close, reap, and the "__"
// callbacks the runtime adapters still call) and runs it in process: the
// callbacks are function calls, the checkout execution guard and the event
// emitter are owner calls, and every refusal keeps the script's exit code,
// message and typed outcome. The lifecycle reaches its owners only through
// the injected operation interfaces of ports.go. NewOwnerPorts wires them to
// the real owners; the fake subpackage supplies recording doubles for tests.
package delegation
