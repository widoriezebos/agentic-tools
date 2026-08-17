#!/usr/bin/env bash
# vm-sync.sh — seed / re-sync selected host dotfiles into the Lima VM.
#
# One-way: host -> guest. Never copies guest state back.
# Idempotent: safe to run as often as you like.
#
#   ./vm-sync.sh              sync the config allowlist
#   ./vm-sync.sh --start      start the VM if stopped, then sync
#   ./vm-sync.sh --creds      also inject tokens (see CREDENTIALS below)
#   ./vm-sync.sh --dry-run    show what would change, copy nothing
#   ./vm-sync.sh --list       print the allowlist and exit
#
# CREDENTIALS
#   macOS keeps Claude Code and gh secrets in the Keychain, not in files, so
#   copying dotfiles cannot carry them. --creds pulls them out via each tool's
#   own CLI and writes them into ~/.vm-secrets.env (mode 600) in the guest.
#   That puts live tokens at rest inside the VM -- opt in deliberately.
set -euo pipefail

INSTANCE="${VM_INSTANCE:-metasystem-debian}"

# --- allowlist ---------------------------------------------------------------
# "host-path-relative-to-$HOME : guest-path-relative-to-$HOME"
# Add lines here; that is the whole configuration surface.
ALLOWLIST=(
  ".gitconfig:.gitconfig"
  ".ssh/known_hosts:.ssh/known_hosts"
  ".claude/settings.json:.claude/settings.json"
)

# Deliberately NOT synced, and why:
#   .claude.json           82 KB of macOS-side project history and paths
#   .ssh/config            may carry macOS-only directives (UseKeychain)
#   .ssh/id_*              private keys; agents in the guest could read them.
#                          Use a scoped deploy key or PAT instead -- and NOT
#                          ssh.forwardAgent, which lets the guest authenticate
#                          as you anywhere for as long as the session is open.
#   .config/gh/hosts.yml   ACTIVELY BREAKS gh in the guest: it carries no token
#                          (the host keeps that in the macOS Keychain), and gh
#                          then tries to migrate it, looks for a freedesktop
#                          secret service that does not exist headless, and
#                          refuses to run at all. Authenticate the guest
#                          separately with `gh auth login` (device flow) or
#                          GH_TOKEN.
# -----------------------------------------------------------------------------

START=0; CREDS=0; DRY=0
for arg in "$@"; do
  case "$arg" in
    --start)   START=1 ;;
    --creds)   CREDS=1 ;;
    --dry-run) DRY=1 ;;
    --list)    printf '%s\n' "${ALLOWLIST[@]}" | sed 's/^/  /'; exit 0 ;;
    -h|--help) sed -n '2,20p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "unknown flag: $arg" >&2; exit 2 ;;
  esac
done

log() { printf '%s\n' "$*"; }

# --- ensure the VM is up -----------------------------------------------------
status=$(limactl list "$INSTANCE" --format '{{.Status}}' 2>/dev/null || true)
if [ "$status" != "Running" ]; then
  if [ "$START" = 1 ]; then
    log "==> starting $INSTANCE"
    limactl start "$INSTANCE" --tty=false >/dev/null
  else
    echo "$INSTANCE is not running (status: ${status:-absent}). Use --start." >&2
    exit 1
  fi
fi

SSH_CONFIG="$HOME/.lima/$INSTANCE/ssh.config"
SSH_HOST="lima-$INSTANCE"
[ -f "$SSH_CONFIG" ] || { echo "missing $SSH_CONFIG" >&2; exit 1; }
RSH="ssh -F $SSH_CONFIG"

guest() { ssh -F "$SSH_CONFIG" "$SSH_HOST" "$@"; }
GUEST_HOME=$(guest 'echo $HOME')

# --- sync the allowlist ------------------------------------------------------
log "==> syncing into $INSTANCE:$GUEST_HOME"
rsync_flags=(-a --no-perms --itemize-changes --human-readable)
[ "$DRY" = 1 ] && rsync_flags+=(--dry-run)

copied=0 missing=0
for entry in "${ALLOWLIST[@]}"; do
  src="$HOME/${entry%%:*}"
  dst="${entry##*:}"
  if [ ! -e "$src" ]; then
    log "    skip (absent on host): ~/${entry%%:*}"
    missing=$((missing + 1))
    continue
  fi
  # create the parent directory in the guest before copying into it
  parent=$(dirname "$dst")
  if [ "$parent" != "." ] && [ "$DRY" = 0 ]; then
    guest "mkdir -p ~/$parent"
  fi
  rc=0
  out=$(rsync "${rsync_flags[@]}" -e "$RSH" "$src" "$SSH_HOST:$GUEST_HOME/$dst" 2>&1) || rc=$?
  if [ "$rc" -ne 0 ]; then
    if [ "$DRY" = 1 ]; then
      log "    ~/$dst  (would create ~/$parent/ first)"
    else
      log "    FAILED ~/$dst"; printf '%s\n' "$out" | sed 's/^/        /'
    fi
  else
    log "    ~/$dst"
    [ -n "$out" ] && printf '%s\n' "$out" | sed 's/^/        /'
  fi
  copied=$((copied + 1))
done

# --- guest-side fixups -------------------------------------------------------
if [ "$DRY" = 0 ]; then
  guest 'bash -s' <<'GUESTFIX'
set -e
# macOS credential helper does not exist on Linux; drop it if it came along.
if git config --global --get credential.helper 2>/dev/null | grep -q osxkeychain; then
  git config --global --unset-all credential.helper
  echo "    fixup: removed osxkeychain credential.helper"
fi
# rsync runs with --no-perms (macOS openrsync has no --chmod), so modes are
# set here. Because rsync ignores modes, this never triggers a re-transfer.
[ -d ~/.ssh ] && chmod 700 ~/.ssh
[ -f ~/.ssh/known_hosts ] && chmod 600 ~/.ssh/known_hosts
exit 0
GUESTFIX
fi

# --- optional credential injection -------------------------------------------
if [ "$CREDS" = 1 ] && [ "$DRY" = 0 ]; then
  log "==> injecting tokens into $INSTANCE:~/.vm-secrets.env (mode 600)"
  secrets=""
  if command -v gh >/dev/null 2>&1 && tok=$(gh auth token 2>/dev/null) && [ -n "$tok" ]; then
    secrets+="export GH_TOKEN='$tok'"$'\n'
    log "    GH_TOKEN            (from host Keychain via gh)"
  else
    log "    GH_TOKEN            unavailable (run 'gh auth login' on the host)"
  fi
  if [ -n "${CLAUDE_CODE_OAUTH_TOKEN:-}" ]; then
    secrets+="export CLAUDE_CODE_OAUTH_TOKEN='$CLAUDE_CODE_OAUTH_TOKEN'"$'\n'
    log "    CLAUDE_CODE_OAUTH_TOKEN (from host environment)"
  else
    log "    CLAUDE_CODE_OAUTH_TOKEN unavailable — run 'claude setup-token' on the"
    log "                            host, export it, then re-run with --creds"
  fi
  if [ -n "$secrets" ]; then
    printf '%s' "$secrets" | guest "cat > ~/.vm-secrets.env && chmod 600 ~/.vm-secrets.env"
    guest 'grep -q vm-secrets.env ~/.bashrc || echo "[ -f ~/.vm-secrets.env ] && . ~/.vm-secrets.env" >> ~/.bashrc'
  fi
fi

log "==> done: $copied synced, $missing skipped$([ "$DRY" = 1 ] && echo ' (dry run)')"
