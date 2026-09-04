Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal channel-fixture-answer-history-drift)
Date: 2026-09-04

# Follow-up: the reply goes to the wrong thread

Seat-side your assertions now fail loudly and show the real cause: the goal history has no answer event at all, and both polls report one message received and dispositioned. The fixture posts a status message (`channel status --post`) before it asks its question, then takes `root_ts` from the journal's first entry, so the human's reply carries the status post's thread reference, not the question's. Since the status-post binding landed (4bdaa5ec), a reply in the status thread is judged against the status token ("start <goal>") and answered "not recorded: wrong token", so the question never sees it. Fix the fixture: take the question's thread reference from the question record the ask verb wrote (the `channel show` verb or the question file under the fixture's channel questions directory carries `thread`), not from the journal's first post, for both the Slack and the Telegram legs; keep your history assertions. `bash -n` on the script. Every path in your return is relative to the repository root (starting with `metasystem/`). The orchestrator reruns the suite seat-side.
