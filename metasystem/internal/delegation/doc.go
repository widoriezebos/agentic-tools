// Package delegation is the dispatch composition package of the
// object-action verb redesign (plans/designs/verbs-object-action.md, section
// 6.3, finding VOA-07, unit U6-seam).
//
// internal/lease and internal/steward import internal/dispatch, so the
// delegate lifecycle that scripts/agents/dispatch.sh orchestrates today cannot
// be ported into internal/dispatch. It lives here, above its owners: this
// package imports dispatch, lease, steward, adapter and goal, and none of
// those (nor any other package under internal/) imports it. Rule R9 holds that
// line; its witness is internal/layering.
//
// The lifecycle reaches its owners only through the injected operation
// interfaces of ports.go. NewOwnerPorts wires them to the real owners; the
// fake subpackage supplies recording doubles for tests. The lifecycle itself
// is a skeleton whose phases mirror dispatch.sh's commands (dispatch,
// follow-up, watch, status, cancel, close, reap). Every phase refuses with
// ErrNotPorted until U6b fills it; U6b ports dispatch.sh, its
// checkout-execution-guard.sh and emit-event.sh into this package and turns
// dispatch.sh's "__" callbacks into calls.
package delegation
