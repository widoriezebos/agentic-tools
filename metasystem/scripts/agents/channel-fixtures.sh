#!/usr/bin/env bash
set -euo pipefail

source_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd -P)
source "$source_root/metasystem/scripts/agents/fixture-budget.sh"
ms=${METASYSTEM_BIN:-$source_root/metasystem/bin/metasystem}
[[ -x "$ms" ]] || { echo "channel fixtures: bin/metasystem is not built" >&2; exit 1; }
fixture_review_by_date() {
  local review_by
  review_by=$(date -u -v+1d +%Y-%m-%d 2>/dev/null) \
    || review_by=$(date -u -d '+1 day' +%Y-%m-%d 2>/dev/null) \
    || { echo "channel fixtures: this host's date cannot compute tomorrow in UTC" >&2; return 1; }
  printf '%s\n' "$review_by"
}
fixture_review_by=$(fixture_review_by_date)
bed=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-channel.XXXXXX")
server_pid=
fixture_owner_started=0
cleanup() {
  local status=$?
  trap - EXIT HUP INT TERM
  if [[ -n "$server_pid" ]] && ps -p "$server_pid" -o command= 2>/dev/null | grep -F -- "$bed/fake" >/dev/null; then
    kill -TERM "$server_pid" 2>/dev/null || true
    wait "$server_pid" 2>/dev/null || true
  fi
  if (( fixture_owner_started )) && ! harness_fixture_reap; then
    status=1
  fi
  [[ $status -ne 0 ]] || rm -rf "$bed"
  return "$status"
}
trap cleanup EXIT HUP INT TERM

# This fixture predates the scenario bed, but its fake server needs the same
# authenticated owner/leash boundary and private mutable state. Go caches are
# deliberately not relocated: they remain under their concurrency-safe owner.
mkdir -p "$bed/home" "$bed/registry" "$bed/proof-admission" \
  "$bed/queue" "$bed/endpoints" "$bed/executables" "$bed/tmp"
export HOME=$bed/home
export TMPDIR=$bed/tmp
export METASYSTEM_SUPERVISION_REGISTRY_HOME=$bed/registry
export METASYSTEM_PROOF_ADMISSION_TEST_DIR=$bed/proof-admission
export METASYSTEM_FIXTURE_NAMESPACE=$bed
export METASYSTEM_FIXTURE_QUEUE_ROOT=$bed/queue
export METASYSTEM_FIXTURE_ENDPOINT_ROOT=$bed/endpoints
export METASYSTEM_FIXTURE_EXECUTABLE_ROOT=$bed/executables
harness_fixture_owner "$source_root/metasystem"
harness_fixture_budget_init "$source_root/metasystem"
fixture_owner_started=1

assert_wait_answer() { # provider label, question id, answer token, stdout path, stderr path, optional wait arguments
	local provider=$1 question=$2 expected_answer=$3 stdout_path=$4 stderr_path=$5
	local wait_status wait_output wait_answer
	shift 5
	set +e
	"$ms" channel wait --root "$repo" --question "$question" --timeout 1 "$@" \
		>"$stdout_path" 2>"$stderr_path"
	wait_status=$?
	set -e
	wait_output=$(<"$stdout_path")
	wait_answer=$(tail -n 1 "$stdout_path")
	if [[ "$wait_status" -eq 0 && "$wait_answer" == "$expected_answer" ]]; then
		return 0
	fi
	printf 'channel fixtures: %s wait exit=%s stdout=%q expected-answer=%q\n' \
		"$provider" "$wait_status" "$wait_output" "$expected_answer" >&2
	cat "$stderr_path" >&2
	tail -n 40 "$fake_dir/journal.jsonl" >&2
	return 1
}

git init -q --bare "$bed/origin.git"
git -C "$source_root" push -q "$bed/origin.git" HEAD:refs/heads/main
git clone -q "$bed/origin.git" "$bed/export"
repo="$bed/export/metasystem"
fake_dir="$bed/fake"
conf_edit "$repo/metasystem.conf" replace-line-first \
  '^metasystem[.]runtimes=.*$' 'metasystem.runtimes=fake'
mkdir -p "$fake_dir"
fake_ready_fifo=$bed/fake.ready
mkfifo "$fake_ready_fifo"
harness_fixture_key channel-fake-server
METASYSTEM_FIXTURE_OWNER="$harness_fixture_key_value" \
  "$ms" channel fake serve --dir "$fake_dir" --ready-fd 3 \
    3>"$fake_ready_fifo" >"$bed/fake.log" 2>&1 &
server_pid=$!
harness_fixture_record_pid "$server_pid"
fake_ready_address=
if ! IFS= read -r fake_ready_address <"$fake_ready_fifo"; then
  if wait "$server_pid"; then fake_server_status=0; else fake_server_status=$?; fi
  server_pid=
  echo "channel fixtures: fake exited before readiness (status $fake_server_status); log follows:" >&2
  cat "$bed/fake.log" >&2
  exit 1
fi
rm -f "$fake_ready_fifo"
[[ -n "$fake_ready_address" && "$fake_ready_address" == "$(<"$fake_dir/base-url")" ]] || {
  echo "channel fixtures: fake readiness did not match its published base-url" >&2
  cat "$bed/fake.log" >&2
  exit 1
}

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
channel.poll-timeout-sec=120
CONF
export METASYSTEM_OWNER_LINEAGE=fixture-lineage
channel_clock_epoch=1893456000
export METASYSTEM_GOAL_NOW=2030-01-01T00:00:00Z
fixture_start=$("$ms" proc started-at --pid "$$")
"$ms" lease announce --root "$repo" --session channel-fixture \
  --pid "$$" --start "$fixture_start" --tag channel-fixture \
  --runtime fake --owner-lineage fixture-lineage >/dev/null

"$ms" channel status --root "$repo" --post >"$bed/status.out"
grep -q 'status ' "$bed/status.out"
grep -q '"method":"chat.postMessage"' "$fake_dir/journal.jsonl"

"$ms" goal open --root "$repo" --id channel-fixture --origin human \
  --intent 'Prove the fleet channel fixture.' --next 'Ask for authority.' \
  --risk severity=3,novelty=1,exposure=1,accumulation=1 \
  --basis 'This established, isolated fixture has low novelty, exposure, and accumulation, but an incorrect channel answer could authorize a one-hour budget without the human.' >/dev/null
qid=$("$ms" channel ask --root "$repo" --goal channel-fixture --kind budget-above-norm \
  --fact 'Approve a one-hour fixture budget.' --option 'approve: continue the fixture' \
  --elapsed-limit 1h --attempt-limit 1 --reserved-job-minutes-limit 60 \
  --active-job-limit 1 --review-round-limit 3 \
  --recommend approve --wants 'goal=channel-fixture minutes=60 reviewRounds=3 goalRevision=3')
root_ts=$(python3 - "$repo/artifacts/agents/channel/questions/$qid.json" <<'PY'
import json, sys
print(json.load(open(sys.argv[1]))['thread']['id'])
PY
)
answer_token=$(python3 - "$repo/artifacts/agents/channel/questions/$qid.json" <<'PY'
import json, sys
print(json.load(open(sys.argv[1]))['wants'])
PY
)
printf '{"thread_ts":"%s","user":"UWIDO","text":"approve","ts":"%s.000001"}\n' \
  "$root_ts" "$channel_clock_epoch" >>"$fake_dir/replies.jsonl"
"$ms" channel poll --root "$repo" >/dev/null
grep -q 'not recorded: no code' "$fake_dir/journal.jsonl"

slack_sent_at=$((channel_clock_epoch + 30))
export METASYSTEM_GOAL_NOW=2030-01-01T00:00:30Z
code=$("$ms" channel fake code --secret "$secret" --at "$slack_sent_at")
printf '{"thread_ts":"%s","user":"UWIDO","text":"%s %s","ts":"%s.000001"}\n' \
  "$root_ts" "$answer_token" "$code" "$slack_sent_at" >>"$fake_dir/replies.jsonl"
"$ms" channel poll --root "$repo" >"$bed/poll.out"
tip=$(git --git-dir "$bed/origin.git" rev-parse refs/heads/main)
history=$(git --git-dir "$bed/origin.git" show "$tip:metasystem/plans/goals/channel-fixture.md")
if ! grep -q 'answer actor=human:wido.*authorityOutcome=AUTHENTICATED_CHANNEL_WORD' <<<"$history" ||
   ! grep -q 'approve actor=human:UWIDO.*authorityOutcome=VERIFIED_CHANNEL_ANSWER' <<<"$history"; then
	printf '%s\n' "$history" >&2
	echo "channel fixtures: Slack history did not record the authenticated answer and verified channel approval" >&2
	exit 1
fi
if ! grep -Fq 'recorded: channel-fixture box raised to 1h, 1 attempts, 60 reserved minutes, 1 active job, 3 review rounds' "$fake_dir/journal.jsonl"; then
	cat "$fake_dir/journal.jsonl" >&2
	echo "channel fixtures: Slack journal did not contain the budget box receipt" >&2
	exit 1
fi
opid=$(sed -n 's/^- [^ ]* \([^ ]*\) answer actor=human:wido.*/\1/p' <<<"$history")
[[ -n "$opid" ]]
"$ms" goal approve --root "$repo" --id channel-fixture --by Wido \
	--elapsed-limit 1h --attempt-limit 1 --reserved-job-minutes-limit 60 --active-job-limit 1 \
	--review-round-limit 3 --fixture-human-authority >/dev/null
"$ms" goal claim --root "$repo" --id channel-fixture >/dev/null
if ! assert_wait_answer Slack "$qid" "$answer_token" \
  "$bed/slack-repeat-wait.out" "$bed/slack-repeat-wait.err"; then
	exit 1
fi

"$ms" goal done --root "$repo" --id channel-fixture --by Wido --conclude 'Slack fixture passed.' >/dev/null
rm -f "$repo/artifacts/agents/channel/fleet/cursor.json"
cat >>"$repo/metasystem.conf.local" <<CONF
channel.destination.fleet.fake.face=telegram
channel.destination.fleet.telegram.api-base=$(<"$fake_dir/base-url")
channel.human.telegram.user-id=7001
CONF
export METASYSTEM_CHANNEL_DESTINATION_FLEET_TELEGRAM_BOT_TOKEN=fake-telegram-token
telegram_question_at=$((channel_clock_epoch + 60))
export METASYSTEM_GOAL_NOW=2030-01-01T00:01:00Z
"$ms" channel status --root "$repo" --post >"$bed/telegram-status.out"
grep -q '"method":"sendMessage"' "$fake_dir/journal.jsonl"

"$ms" goal open --root "$repo" --id channel-telegram-fixture --origin human \
  --intent 'Prove the Telegram fleet channel fixture.' --next 'Ask for authority.' \
  --risk severity=3,novelty=1,exposure=1,accumulation=1 \
  --basis 'This established, isolated fixture has low novelty, exposure, and accumulation, but an incorrect Telegram answer could authorize a one-hour budget without the human.' >/dev/null
telegram_qid=$("$ms" channel ask --root "$repo" --goal channel-telegram-fixture --kind budget-above-norm \
  --fact 'Approve a one-hour Telegram fixture budget.' --option 'approve: continue the fixture' \
  --elapsed-limit 1h --attempt-limit 1 --reserved-job-minutes-limit 60 \
  --active-job-limit 1 --review-round-limit 3 \
  --recommend approve --wants 'goal=channel-telegram-fixture minutes=60 reviewRounds=3 goalRevision=3')
telegram_root=$(python3 - "$repo/artifacts/agents/channel/questions/$telegram_qid.json" <<'PY'
import json, sys
print(json.load(open(sys.argv[1]))['thread']['id'])
PY
)
telegram_answer_token=$(python3 - "$repo/artifacts/agents/channel/questions/$telegram_qid.json" <<'PY'
import json, sys
print(json.load(open(sys.argv[1]))['wants'])
PY
)
printf '{"face":"telegram","reply_to":%s,"user":7001,"date":%s,"text":"approve"}\n' \
  "$telegram_root" "$telegram_question_at" >>"$fake_dir/replies.jsonl"
"$ms" channel telegram peek --root "$repo" >"$bed/telegram-peek.out"
grep -q '^chat=1000 user=7001 text=approve$' "$bed/telegram-peek.out"
"$ms" channel poll --root "$repo" >/dev/null
grep -q 'not recorded: no code' "$fake_dir/journal.jsonl"
telegram_receipt=$(python3 - "$repo/artifacts/agents/channel/questions/$telegram_qid.json" <<'PY'
import json, sys
print(json.load(open(sys.argv[1]))['rejected'][-1]['postRef']['id'])
PY
)
telegram_sent_at=$((channel_clock_epoch + 90))
export METASYSTEM_GOAL_NOW=2030-01-01T00:01:30Z
telegram_code=$("$ms" channel fake code --secret "$secret" --at "$telegram_sent_at")
printf '{"face":"telegram","reply_to":%s,"user":7001,"date":%s,"text":"%s %s"}\n' \
  "$telegram_receipt" "$telegram_sent_at" "$telegram_answer_token" "$telegram_code" >>"$fake_dir/replies.jsonl"
if ! assert_wait_answer Telegram "$telegram_qid" "$telegram_answer_token" \
  "$bed/telegram-wait.out" "$bed/telegram-wait.err" --poll-seconds 1; then
	exit 1
fi
telegram_tip=$(git --git-dir "$bed/origin.git" rev-parse refs/heads/main)
telegram_history=$(git --git-dir "$bed/origin.git" show "$telegram_tip:metasystem/plans/goals/channel-telegram-fixture.md")
if ! grep -q 'answer actor=human:wido.*authorityOutcome=AUTHENTICATED_CHANNEL_WORD' <<<"$telegram_history" ||
   ! grep -q 'approve actor=human:7001.*authorityOutcome=VERIFIED_CHANNEL_ANSWER' <<<"$telegram_history"; then
	printf '%s\n' "$telegram_history" >&2
	echo "channel fixtures: Telegram history did not record the authenticated answer and verified channel approval" >&2
	exit 1
fi
if ! grep -Fq 'recorded: channel-telegram-fixture box raised to 1h, 1 attempts, 60 reserved minutes, 1 active job, 3 review rounds' "$fake_dir/journal.jsonl"; then
	cat "$fake_dir/journal.jsonl" >&2
	echo "channel fixtures: Telegram journal did not contain the budget box receipt" >&2
	exit 1
fi
telegram_opid=$(sed -n 's/^- [^ ]* \([^ ]*\) answer actor=human:wido.*/\1/p' <<<"$telegram_history")
[[ -n "$telegram_opid" ]]
"$ms" goal approve --root "$repo" --id channel-telegram-fixture --by Wido \
	--elapsed-limit 1h --attempt-limit 1 --reserved-job-minutes-limit 60 --active-job-limit 1 \
	--review-round-limit 3 --fixture-human-authority >/dev/null
"$ms" goal claim --root "$repo" --id channel-telegram-fixture >/dev/null
if ! assert_wait_answer Telegram "$telegram_qid" "$telegram_answer_token" \
  "$bed/telegram-repeat-wait.out" "$bed/telegram-repeat-wait.err"; then
	exit 1
fi

echo "channel fixtures: PASSED"
