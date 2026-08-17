#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────────────────────
# agent-workbench — shell helpers around a single Lima VM
#
# Install:
#   echo 'source /Users/wido/LocalStorage/GitHub/agent-workbench/workbench.sh' >> ~/.zshrc
#
# The VM mounts the whole GitHub root at /workspace, so "switching project"
# is just running the agent from a different host directory — the helpers map
# your host cwd to the matching guest path automatically. No remount, no restart.
# ─────────────────────────────────────────────────────────────────────────────

WORKBENCH_VM="${WORKBENCH_VM:-agent-workbench}"
WORKBENCH_HOST_ROOT="${WORKBENCH_HOST_ROOT:-/Users/wido/LocalStorage/GitHub}"
WORKBENCH_GUEST_ROOT="${WORKBENCH_GUEST_ROOT:-/workspace}"
WORKBENCH_SELF="${BASH_SOURCE[0]:-$0}"
WORKBENCH_DIR="$(cd "$(dirname "$WORKBENCH_SELF")" && pwd)"
WORKBENCH_TEMPLATE="${WORKBENCH_TEMPLATE:-$WORKBENCH_DIR/workbench.yaml}"

# ── internals ────────────────────────────────────────────────────────────────

_wb_exists() { limactl list -q 2>/dev/null | grep -qx "$WORKBENCH_VM"; }

_wb_running() {
  limactl list --format '{{.Name}} {{.Status}}' 2>/dev/null \
    | grep -qx "$WORKBENCH_VM Running"
}

# Map a host path under the mount root to its path inside the VM.
_wb_guest_path() {
  local d
  d="$(cd "${1:-$PWD}" 2>/dev/null && pwd -P)" || return 1
  case "$d" in
    "$WORKBENCH_HOST_ROOT")   echo "$WORKBENCH_GUEST_ROOT" ;;
    "$WORKBENCH_HOST_ROOT"/*) echo "$WORKBENCH_GUEST_ROOT/${d#"$WORKBENCH_HOST_ROOT"/}" ;;
    *) return 1 ;;
  esac
}

# Ensure the VM is up, then resolve the guest working directory.
_wb_prepare() {
  if ! _wb_exists; then
    echo "VM '$WORKBENCH_VM' does not exist. Run: workbench build" >&2
    return 1
  fi
  if ! _wb_running; then
    echo "Starting $WORKBENCH_VM..." >&2
    limactl start "$WORKBENCH_VM" >/dev/null || return 1
  fi
  if ! _wb_guest_path "$PWD" 2>/dev/null; then
    echo "Not inside $WORKBENCH_HOST_ROOT — the VM cannot see $PWD." >&2
    echo "cd into a repo under the mount root first." >&2
    return 1
  fi
}

# Run a command in the VM, in the guest directory matching the host cwd.
_wb_in() {
  local wd
  wd="$(_wb_prepare)" || return 1
  limactl shell --workdir "$wd" "$WORKBENCH_VM" -- "$@"
}

# ── commands ─────────────────────────────────────────────────────────────────

_wb_build() {
  if _wb_exists; then
    echo "VM '$WORKBENCH_VM' already exists. Use 'workbench rebuild' to recreate it." >&2
    return 1
  fi
  echo "Creating $WORKBENCH_VM from $WORKBENCH_TEMPLATE ..."
  limactl create --name="$WORKBENCH_VM" --tty=false "$@" "$WORKBENCH_TEMPLATE" || return 1
  limactl start "$WORKBENCH_VM" || return 1
  echo
  echo "Ready. Agents must authenticate inside the VM on first use — see 'workbench help'."
}

_wb_rebuild() {
  if _wb_exists; then
    printf "Destroy and recreate '%s'? Everything inside the VM is lost (your repos are on the host and unaffected). [y/N] " "$WORKBENCH_VM"
    local reply; read -r reply
    [[ "$reply" =~ ^[Yy]$ ]] || { echo "Aborted."; return 0; }
    limactl stop "$WORKBENCH_VM" >/dev/null 2>&1
    limactl delete "$WORKBENCH_VM" --force >/dev/null 2>&1
  fi
  _wb_build "$@"
}

# Block outbound internet, keeping host<->VM traffic (mounts, port forwards) alive.
# NOTE: a guardrail, not a wall — the guest user has passwordless sudo and can
# undo this. It reliably stops accidental network access, not a hostile process.
_wb_offline() {
  _wb_running || { echo "VM is not running." >&2; return 1; }
  limactl shell "$WORKBENCH_VM" -- sudo sh -c '
    iptables -F OUTPUT
    iptables -A OUTPUT -o lo -j ACCEPT
    iptables -A OUTPUT -d 10.0.0.0/8     -j ACCEPT
    iptables -A OUTPUT -d 172.16.0.0/12  -j ACCEPT
    iptables -A OUTPUT -d 192.168.0.0/16 -j ACCEPT
    iptables -P OUTPUT DROP'
  echo "Offline: outbound internet blocked (host/VM traffic still works)."
}

_wb_online() {
  _wb_running || { echo "VM is not running." >&2; return 1; }
  limactl shell "$WORKBENCH_VM" -- sudo sh -c 'iptables -P OUTPUT ACCEPT; iptables -F OUTPUT'
  echo "Online: outbound internet restored."
}

_wb_readonly() {
  _wb_running || { echo "VM is not running." >&2; return 1; }
  limactl shell "$WORKBENCH_VM" -- sudo mount -o remount,ro "$WORKBENCH_GUEST_ROOT" \
    && echo "$WORKBENCH_GUEST_ROOT is now read-only."
}

_wb_readwrite() {
  _wb_running || { echo "VM is not running." >&2; return 1; }
  limactl shell "$WORKBENCH_VM" -- sudo mount -o remount,rw "$WORKBENCH_GUEST_ROOT" \
    && echo "$WORKBENCH_GUEST_ROOT is now read-write."
}

# Protect git history across every repo while leaving working files writable.
_wb_git_ro() {
  _wb_running || { echo "VM is not running." >&2; return 1; }
  limactl shell "$WORKBENCH_VM" -- sudo bash -c '
    for g in '"$WORKBENCH_GUEST_ROOT"'/*/.git; do
      [ -d "$g" ] || continue
      mountpoint -q "$g" && continue
      mount --bind "$g" "$g" && mount -o remount,ro,bind "$g"
    done'
  echo "All .git directories under $WORKBENCH_GUEST_ROOT are read-only."
}

_wb_git_rw() {
  _wb_running || { echo "VM is not running." >&2; return 1; }
  limactl shell "$WORKBENCH_VM" -- sudo bash -c '
    for g in '"$WORKBENCH_GUEST_ROOT"'/*/.git; do
      mountpoint -q "$g" && umount "$g"
    done' 2>/dev/null
  echo "git history is writable again."
}

_wb_status() {
  if ! _wb_exists; then echo "VM '$WORKBENCH_VM' does not exist."; return 0; fi
  limactl list "$WORKBENCH_VM"
  echo
  echo "Mount root (host):  $WORKBENCH_HOST_ROOT"
  echo "Mount point (VM):   $WORKBENCH_GUEST_ROOT"
  local wd
  if wd="$(_wb_guest_path "$PWD" 2>/dev/null)"; then
    echo "Current dir maps to: $wd"
  else
    echo "Current dir is OUTSIDE the mount — not visible to the VM."
  fi
  if _wb_running; then
    echo
    limactl shell "$WORKBENCH_VM" -- sh -c '
      printf "Network: "; [ "$(sudo iptables -S OUTPUT 2>/dev/null | head -1)" = "-P OUTPUT DROP" ] && echo "OFFLINE" || echo "online"
      printf "Mount:   "; findmnt -no OPTIONS '"$WORKBENCH_GUEST_ROOT"' 2>/dev/null | cut -d, -f1' 2>/dev/null
  fi
}

_wb_help() {
  cat <<'EOF'
workbench — one Lima VM, whole GitHub root mounted at /workspace

  Lifecycle
    workbench build [limactl-args]   Create and start the VM
    workbench rebuild                Destroy and recreate it (prompts)
    workbench start | stop           Start / stop the VM
    workbench status                 VM state, mount, network, current mapping
    workbench rm                     Delete the VM (repos on the host are untouched)

  Working
    workbench shell                  zsh in the VM, in the dir matching your cwd
    workbench run <cmd...>           One-off command in the VM, same dir mapping
    workbench claude [args...]       Claude Code  (--dangerously-skip-permissions)
    workbench codex  [args...]       Codex CLI    (--full-auto)
    workbench devin  [args...]       Devin CLI

  Guardrails (per-session; reset on VM restart)
    workbench offline | online       Block / restore outbound internet
    workbench readonly | readwrite   Remount /workspace ro / rw
    workbench git-ro  | git-rw       Freeze / unfreeze all .git directories

  Build with your orchestration framework preinstalled:
    workbench build --param FRAMEWORK_REPO=git@github.com:you/orch.git \
                    --param FRAMEWORK_INSTALL='./install.sh'

  First-run auth: agents authenticate inside the VM, independently of the host.
  Lima forwards guest ports to host 127.0.0.1, so OAuth callbacks to localhost
  work — run the login, open the printed URL in your Mac browser, done.
  Or skip the browser entirely by exporting ANTHROPIC_API_KEY / OPENAI_API_KEY
  in ~/.workbench-env inside the VM.
EOF
}

# ── dispatch ─────────────────────────────────────────────────────────────────

workbench() {
  local cmd="${1:-help}"; shift 2>/dev/null || true
  case "$cmd" in
    build)      _wb_build "$@" ;;
    rebuild)    _wb_rebuild "$@" ;;
    start)      limactl start "$WORKBENCH_VM" ;;
    stop)       limactl stop "$WORKBENCH_VM" ;;
    rm|delete)  limactl stop "$WORKBENCH_VM" >/dev/null 2>&1; limactl delete "$WORKBENCH_VM" --force ;;
    status)     _wb_status ;;
    shell)      local wd; wd="$(_wb_prepare)" || return 1; limactl shell --workdir "$wd" "$WORKBENCH_VM" ;;
    run)        _wb_in "$@" ;;
    claude)     _wb_in claude --dangerously-skip-permissions "$@" ;;
    codex)      _wb_in codex --full-auto "$@" ;;
    devin)      _wb_in devin "$@" ;;
    offline)    _wb_offline ;;
    online)     _wb_online ;;
    readonly)   _wb_readonly ;;
    readwrite)  _wb_readwrite ;;
    git-ro)     _wb_git_ro ;;
    git-rw)     _wb_git_rw ;;
    help|-h|--help) _wb_help ;;
    *)          echo "Unknown command: $cmd" >&2; _wb_help; return 1 ;;
  esac
}
