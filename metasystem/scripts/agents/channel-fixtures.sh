#!/usr/bin/env bash
set -euo pipefail

source_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd -P)
ms=${METASYSTEM_BIN:-$source_root/metasystem/bin/metasystem}
[[ -x "$ms" ]] || { echo "channel fixtures: bin/metasystem is not built" >&2; exit 1; }
bed=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-channel.XXXXXX")
server_pid=
cleanup() {
  local status=$?
  trap - EXIT HUP INT TERM
  if [[ -n "$server_pid" ]] && ps -p "$server_pid" -o command= 2>/dev/null | grep -F -- "$bed/fake" >/dev/null; then
    kill -TERM "$server_pid" 2>/dev/null || true
    wait "$server_pid" 2>/dev/null || true
  fi
  [[ $status -ne 0 ]] || rm -rf "$bed"
  return "$status"
}
trap cleanup EXIT HUP INT TERM

git init -q --bare "$bed/origin.git"
git -C "$source_root" push -q "$bed/origin.git" HEAD:refs/heads/main
git clone -q "$bed/origin.git" "$bed/export"
repo="$bed/export/metasystem"
fake_dir="$bed/fake"
mkdir -p "$fake_dir"
"$ms" channel fake serve --dir "$fake_dir" >"$bed/fake.log" 2>&1 &
server_pid=$!
deadline=$((SECONDS + 30))
while [[ ! -s "$fake_dir/base-url" ]]; do
  if (( SECONDS >= deadline )); then
    echo "channel fixtures: fake did not publish base-url within 30 seconds" >&2
    exit 1
  fi
  sleep 0.05
done

git -C "$repo" config metasystem.goal.machine fixture-machine
git -C "$repo" config goal.sync-remote origin
git -C "$repo" config goal.sync-branch refs/heads/main
git -C "$repo" update-ref refs/metasystem/goals/accepted HEAD
secret='JBSWY3DPEHPK3PXP'
cat >>"$repo/metasystem.conf.local" <<CONF
channel.destination.fleet.adapter=fake
channel.destination.fleet.fake.dir=$fake_dir
channel.human.slack.user-id=UWIDO
channel.human.totp-secret=$secret
channel.status.interval-minutes=240
CONF
export METASYSTEM_OWNER_LINEAGE=fixture-lineage
fixture_start=$("$ms" proc started-at --pid "$$")
"$ms" lease announce --root "$repo" --session channel-fixture \
  --pid "$$" --start "$fixture_start" --tag channel-fixture \
  --runtime fake --owner-lineage fixture-lineage >/dev/null

"$ms" channel status --root "$repo" --post >"$bed/status.out"
grep -q 'status ' "$bed/status.out"
grep -q '"method":"chat.postMessage"' "$fake_dir/journal.jsonl"

"$ms" goal open --root "$repo" --id channel-fixture --intent 'Prove the fleet channel fixture.' --next 'Ask for authority.' --tier 3 >/dev/null
qid=$("$ms" channel ask --root "$repo" --goal channel-fixture --kind budget-above-norm \
  --fact 'Approve a one-hour fixture budget.' --option 'approve: continue the fixture' \
  --recommend approve --wants 'goal=channel-fixture minutes=60 reviewRounds=3 goalRevision=3')
root_ts=$(python3 - "$fake_dir/journal.jsonl" <<'PY'
import json, sys
n = sum(1 for line in open(sys.argv[1]) if json.loads(line)['method'] == 'chat.postMessage')
print(f"{1000000+n}.000000")
PY
)
printf '{"thread_ts":"%s","user":"UWIDO","text":"approve"}\n' "$root_ts" >>"$fake_dir/replies.jsonl"
"$ms" channel poll --root "$repo" >/dev/null
grep -q 'not recorded: no code' "$fake_dir/journal.jsonl"

code=$("$ms" channel fake code --secret "$secret")
printf '{"thread_ts":"%s","user":"UWIDO","text":"approve %s"}\n' "$root_ts" "$code" >>"$fake_dir/replies.jsonl"
"$ms" channel poll --root "$repo" >"$bed/poll.out"
tip=$(git --git-dir "$bed/origin.git" rev-parse refs/heads/main)
history=$(git --git-dir "$bed/origin.git" show "$tip:metasystem/plans/goals/channel-fixture.md")
grep -q 'answer actor=human:wido' <<<"$history"
grep -q 'recorded as your word' "$fake_dir/journal.jsonl"
opid=$(sed -n 's/^- [^ ]* \([^ ]*\) answer actor=human:wido.*/\1/p' <<<"$history")
[[ -n "$opid" ]]
"$ms" goal claim --root "$repo" --id channel-fixture --approved-ref "$opid" \
	--elapsed-limit 1h --attempt-limit 1 --reserved-job-minutes-limit 60 --active-job-limit 1 \
	--review-round-limit 3 >/dev/null
[[ $("$ms" channel wait --root "$repo" --question "$qid" --timeout 1) == approve ]]

"$ms" goal done --root "$repo" --id channel-fixture --conclude 'Slack fixture passed.' >/dev/null
rm -f "$repo/artifacts/agents/channel/fleet/cursor.json"
cat >>"$repo/metasystem.conf.local" <<CONF
channel.destination.fleet.fake.face=telegram
channel.destination.fleet.telegram.api-base=$(<"$fake_dir/base-url")
channel.human.telegram.user-id=7001
CONF
export METASYSTEM_CHANNEL_DESTINATION_FLEET_TELEGRAM_BOT_TOKEN=fake-telegram-token
"$ms" channel status --root "$repo" --post >"$bed/telegram-status.out"
grep -q '"method":"sendMessage"' "$fake_dir/journal.jsonl"

"$ms" goal open --root "$repo" --id channel-telegram-fixture \
  --intent 'Prove the Telegram fleet channel fixture.' --next 'Ask for authority.' --tier 3 >/dev/null
telegram_qid=$("$ms" channel ask --root "$repo" --goal channel-telegram-fixture --kind budget-above-norm \
  --fact 'Approve a one-hour Telegram fixture budget.' --option 'approve: continue the fixture' \
  --recommend approve --wants 'goal=channel-telegram-fixture minutes=60 reviewRounds=3 goalRevision=3')
telegram_root=$(python3 - "$repo/artifacts/agents/channel/questions/$telegram_qid.json" <<'PY'
import json, sys
print(json.load(open(sys.argv[1]))['thread']['id'])
PY
)
printf '{"face":"telegram","reply_to":%s,"user":7001,"text":"approve"}\n' "$telegram_root" >>"$fake_dir/replies.jsonl"
"$ms" channel telegram peek --root "$repo" >"$bed/telegram-peek.out"
grep -q '^chat=1000 user=7001 text=approve$' "$bed/telegram-peek.out"
"$ms" channel poll --root "$repo" >/dev/null
grep -q 'not recorded: no code' "$fake_dir/journal.jsonl"
telegram_receipt=$(python3 - "$repo/artifacts/agents/channel/questions/$telegram_qid.json" <<'PY'
import json, sys
print(json.load(open(sys.argv[1]))['rejected'][-1]['postRef']['id'])
PY
)
telegram_code=$("$ms" channel fake code --secret "$secret" --at "$(( $(date +%s) + 30 ))")
printf '{"face":"telegram","reply_to":%s,"user":7001,"text":"approve %s"}\n' "$telegram_receipt" "$telegram_code" >>"$fake_dir/replies.jsonl"
"$ms" channel poll --root "$repo" >"$bed/telegram-poll.out"
telegram_tip=$(git --git-dir "$bed/origin.git" rev-parse refs/heads/main)
telegram_history=$(git --git-dir "$bed/origin.git" show "$telegram_tip:metasystem/plans/goals/channel-telegram-fixture.md")
grep -q 'answer actor=human:wido' <<<"$telegram_history"
grep -q 'recorded as your word on channel telegram fixture' "$fake_dir/journal.jsonl"
telegram_opid=$(sed -n 's/^- [^ ]* \([^ ]*\) answer actor=human:wido.*/\1/p' <<<"$telegram_history")
[[ -n "$telegram_opid" ]]
"$ms" goal claim --root "$repo" --id channel-telegram-fixture --approved-ref "$telegram_opid" \
	--elapsed-limit 1h --attempt-limit 1 --reserved-job-minutes-limit 60 --active-job-limit 1 \
	--review-round-limit 3 >/dev/null
[[ $("$ms" channel wait --root "$repo" --question "$telegram_qid" --timeout 1) == approve ]]

echo "channel fixtures: PASSED"
