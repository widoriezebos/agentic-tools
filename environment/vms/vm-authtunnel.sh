#!/usr/bin/env bash
# vm-authtunnel.sh — auto-forward a guest OAuth callback port to the host.
#
# Port forwarding is default-deny in this VM, so browser-based logins that use a
# localhost callback (claude auth login, codex login) cannot complete: the
# browser runs on the host, the listener is in the guest. This watches for a
# callback listener appearing in the guest and opens an `ssh -L` tunnel for it,
# which bypasses Lima's portForwards entirely.
#
#   ./vm-authtunnel.sh          watch until a callback appears, tunnel it, wait
#   ./vm-authtunnel.sh 1455     tunnel a known fixed port immediately (codex)
set -euo pipefail
INSTANCE="${VM_INSTANCE:-metasystem-debian}"
SSH_CONFIG="$HOME/.lima/$INSTANCE/ssh.config"
SSH_HOST="lima-$INSTANCE"
# Owners whose listeners are real login callbacks. containerd/buildkit/sshd all
# listen too, and tunnelling those is pointless noise.
OWNERS='claude|node|codex|devin'

tunnel() {
  local p="$1"
  pgrep -f "L $p:127.0.0.1:$p" >/dev/null 2>&1 && { echo "  already tunnelled: $p"; return; }
  nohup ssh -F "$SSH_CONFIG" -N -L "$p:127.0.0.1:$p" "$SSH_HOST" >/dev/null 2>&1 &
  sleep 2
  echo "  tunnel open: host 127.0.0.1:$p -> guest 127.0.0.1:$p"
  echo "  now complete the login in your browser."
}

if [ $# -ge 1 ]; then tunnel "$1"; exit 0; fi

echo "watching $INSTANCE for a login callback listener (Ctrl-C to stop)..."
while true; do
  port="$(ssh -F "$SSH_CONFIG" "$SSH_HOST" \
      "ss -ltnpH 2>/dev/null | grep -E '\"($OWNERS)\"' | awk '{print \$4}' | sed 's/.*://' | head -1" 2>/dev/null || true)"
  if [ -n "${port:-}" ]; then tunnel "$port"; echo "  (leaving tunnel up; Ctrl-C when done)"; wait; fi
  sleep 2
done
